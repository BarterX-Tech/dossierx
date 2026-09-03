package readiness

import (
	"reflect"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

func lockedClaim(id string, rests ...string) model.Claim {
	return model.Claim{ID: id, Facet: "contract", Module: "fixture", Status: model.StatusLocked, Body: id + " body", RestsOn: rests}
}

func standingStore(claims ...model.Claim) *lock.Store {
	s := &lock.Store{
		PolicyVersion: lock.PolicyLocalApprovalV1,
		Hashes:        map[string]map[string]string{},
		Ledger:        map[string]lock.LedgerRecord{},
	}
	for _, c := range claims {
		s.Ledger[c.ID] = lock.LedgerRecord{Subject: lock.SubjectClaim, Hash: lock.LockedClaimHash(c), At: "2026-09-03T00:00:00Z", Reason: "fixture approval"}
	}
	return s
}

func recordBaseline(s *lock.Store, dependent string, dependency model.Claim) {
	if s.Hashes[dependent] == nil {
		s.Hashes[dependent] = map[string]string{}
	}
	s.Hashes[dependent][dependency.ID] = lock.ContentHash(dependency)
}

func hasPath[T interface{ getPath() Path }](values []T, want ...string) bool {
	for _, value := range values {
		if reflect.DeepEqual(value.getPath(), Path(want)) {
			return true
		}
	}
	return false
}

func (c DependencyCondition) getPath() Path { return c.Path }
func (c Cause) getPath() Path               { return c.Path }

func hasCause(a Assessment, kind CauseKind, want ...string) bool {
	for _, cause := range a.ReviewCauses {
		if cause.Kind == kind && hasPath([]Cause{cause}, want...) {
			return true
		}
	}
	return false
}

func hasCondition(a Assessment, kind ConditionKind, want ...string) bool {
	for _, condition := range a.DependencyConditions {
		if condition.Kind == kind && hasPath([]DependencyCondition{condition}, want...) {
			return true
		}
	}
	return false
}

func TestComputeStandingApprovalAndDraftDependency(t *testing.T) {
	a := model.Claim{ID: "fixture.contract.a", Facet: "contract", Module: "fixture", Status: model.StatusDraft, Body: "draft boundary"}
	b := lockedClaim("fixture.contract.b", a.ID)
	s := standingStore(b)
	recordBaseline(s, b.ID, a)

	got := Compute([]model.Claim{a, b}, s, nil)
	if !got[b.ID].LocalApproved || !got[b.ID].LocallyApproved {
		t.Fatalf("B should remain locally approved against a reviewed draft dependency: %+v", got[b.ID])
	}
	if got[b.ID].DependencyReady || got[b.ID].Ready {
		t.Fatalf("B must not be dependency-ready while A is draft: %+v", got[b.ID])
	}
	if !hasCondition(got[b.ID], ConditionDependencyUnapproved, b.ID, a.ID) {
		t.Fatalf("B must expose the direct unapproved-dependency path: %+v", got[b.ID].DependencyConditions)
	}
}

func TestComputeNestedCausesAndSelectivePathClearing(t *testing.T) {
	c := lockedClaim("fixture.contract.c")
	a := lockedClaim("fixture.contract.a", c.ID)
	d := lockedClaim("fixture.contract.d", c.ID)
	b := lockedClaim("fixture.contract.b", a.ID, d.ID)
	claims := []model.Claim{b, a, d, c}
	s := standingStore(claims...)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, d.ID, c)
	recordBaseline(s, b.ID, a)
	recordBaseline(s, b.ID, d)

	changedC := c
	changedC.Body = "changed foundation"
	claims[3] = changedC
	// C has been explicitly reapproved after its content change. This keeps
	// the test focused on the two dependent baselines rather than C's own
	// approval-integrity cause.
	s.Ledger[c.ID] = lock.LedgerRecord{Subject: lock.SubjectClaim, Hash: lock.LockedClaimHash(changedC), At: "2026-09-03T00:01:00Z", Reason: "reapproved foundation"}
	got := Compute(claims, s, nil)
	if !hasCause(got[a.ID], CauseDirectDependencyChange, a.ID, c.ID) || !hasCause(got[d.ID], CauseDirectDependencyChange, d.ID, c.ID) {
		t.Fatalf("both direct consumers need independent drift causes: A=%+v D=%+v", got[a.ID].ReviewCauses, got[d.ID].ReviewCauses)
	}
	if !hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, a.ID, c.ID) || !hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, d.ID, c.ID) {
		t.Fatalf("B must retain both nested paths: %+v", got[b.ID].ReviewCauses)
	}

	// Confirming A's unchanged boundary refreshes only A's C baseline. D's
	// independent path remains active and must still keep B pending.
	recordBaseline(s, a.ID, changedC)
	got = Compute(claims, s, nil)
	if hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, a.ID, c.ID) {
		t.Fatalf("confirming A must clear only A's inherited path: %+v", got[b.ID].ReviewCauses)
	}
	if !hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, d.ID, c.ID) || !got[b.ID].ReviewPending {
		t.Fatalf("D's independent path must remain: %+v", got[b.ID])
	}
}

func TestComputeOwnCausesPropagateAndUnchangedUnlockIsCondition(t *testing.T) {
	c := lockedClaim("fixture.contract.c")
	a := lockedClaim("fixture.contract.a", c.ID)
	a.Comments = []model.Comment{{ID: "c-thread", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Body: "review boundary"}}
	b := lockedClaim("fixture.contract.b", a.ID)
	s := standingStore(a, b)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, b.ID, a)
	flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{a.ID: {Reason: "boundary needs review"}}}
	got := Compute([]model.Claim{b, a, c}, s, flags)
	if !hasCause(got[a.ID], CauseOwnThread, a.ID) || !hasCause(got[a.ID], CauseOwnFlag, a.ID) {
		t.Fatalf("A must retain independent direct thread and flag causes: %+v", got[a.ID].ReviewCauses)
	}
	if !hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, a.ID) || !got[b.ID].ReviewPending {
		t.Fatalf("B must inherit A's own flag: %+v", got[b.ID])
	}
	if !hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, a.ID) {
		t.Fatalf("B must also retain A's open-thread path: %+v", got[b.ID])
	}

	// Unlock C without changing its comparable content. This creates a
	// readiness condition through A and B, never a semantic drift cause.
	unlockedC := c
	unlockedC.Status = model.StatusDraft
	got = Compute([]model.Claim{b, a, unlockedC}, s, nil)
	if !hasCondition(got[a.ID], ConditionDependencyUnapproved, a.ID, c.ID) || !hasCondition(got[b.ID], ConditionDependencyUnapproved, b.ID, a.ID, c.ID) {
		t.Fatalf("unchanged unlock must propagate unapproved status: A=%+v B=%+v", got[a.ID].DependencyConditions, got[b.ID].DependencyConditions)
	}
	if hasCause(got[b.ID], CauseDirectDependencyChange, b.ID, a.ID) || hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, a.ID, c.ID) {
		t.Fatalf("unchanged unlock must not fabricate drift: %+v", got[b.ID].ReviewCauses)
	}
}

func TestComputeInvalidNestedApprovalPreservesEveryPathNode(t *testing.T) {
	c := lockedClaim("fixture.contract.c")
	a := lockedClaim("fixture.contract.a", c.ID)
	b := lockedClaim("fixture.contract.b", a.ID)
	s := standingStore(a, b, c)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, b.ID, a)
	changedC := c
	changedC.Body = "tampered foundation"

	got := Compute([]model.Claim{b, a, changedC}, s, nil)
	if !hasCause(got[a.ID], CauseDirectDependencyChange, a.ID, c.ID) {
		t.Fatalf("A must report the direct dependency drift path: %+v", got[a.ID].ReviewCauses)
	}
	if !hasCause(got[a.ID], CauseUpstreamDependencyReview, a.ID, c.ID) {
		t.Fatalf("A must also retain C's independent approval-integrity path: %+v", got[a.ID].ReviewCauses)
	}
	if !hasCondition(got[b.ID], ConditionDependencyUnapproved, b.ID, a.ID) {
		t.Fatalf("B must report its immediate invalid prerequisite: %+v", got[b.ID].DependencyConditions)
	}
	if !hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, a.ID, c.ID) {
		t.Fatalf("B must preserve B->A->C for inherited invalid approval: %+v", got[b.ID].ReviewCauses)
	}
}

func TestComputeIntegrityDriftPropagatesAndStaleBitCannotMakeReady(t *testing.T) {
	a := lockedClaim("fixture.contract.a")
	b := lockedClaim("fixture.contract.b", a.ID)
	s := standingStore(a, b)
	recordBaseline(s, b.ID, a)
	changedA := a
	changedA.Body = "tampered after approval"
	changedA.ReviewPending = false
	got := Compute([]model.Claim{b, changedA}, s, nil)
	if got[a.ID].LocalApproved || got[a.ID].Ready || !hasCause(got[a.ID], CauseApprovalContentDrift, a.ID) {
		t.Fatalf("ledger content drift must invalidate A's local approval: %+v", got[a.ID])
	}
	if got[b.ID].Ready || !hasCondition(got[b.ID], ConditionDependencyUnapproved, b.ID, a.ID) || !hasCause(got[b.ID], CauseUpstreamDependencyReview, b.ID, a.ID) {
		t.Fatalf("A's invalid approval must propagate non-readiness to B: %+v", got[b.ID])
	}

	// A stale persisted bit alone cannot create a review cause or a false
	// negative/positive readiness result; the live ledger and dependencies are
	// the authority.
	clean := a
	clean.ReviewPending = true
	cleanStore := standingStore(clean)
	cleanGot := Compute([]model.Claim{clean}, cleanStore, nil)[clean.ID]
	if !cleanGot.LocalApproved || cleanGot.ReviewPending || !cleanGot.Ready {
		t.Fatalf("stale persisted review_pending must not affect live readiness: %+v", cleanGot)
	}
}

func TestComputeMissingRetiredCycleGovernedAndLegacyHistory(t *testing.T) {
	missing := lockedClaim("fixture.contract.missing-user", "fixture.contract.does-not-exist")
	retiredDep := lockedClaim("fixture.contract.retired")
	retiredDep.Status = model.Status("retired")
	retired := lockedClaim("fixture.contract.retired-user", retiredDep.ID)
	cycleA := lockedClaim("fixture.contract.cycle-a", "fixture.contract.cycle-b")
	cycleB := lockedClaim("fixture.contract.cycle-b", cycleA.ID)
	governor := model.Claim{ID: "fixture.doctrine.rule", Facet: "doctrine", Status: model.StatusDraft, Body: "draft doctrine"}
	governed := lockedClaim("fixture.contract.governed")
	governed.Governed.Type = governor.ID
	legacyA := lockedClaim("fixture.contract.legacy-a")
	legacyB := lockedClaim("fixture.contract.legacy-b", legacyA.ID)

	s := standingStore(missing, retired, cycleA, cycleB, governed, legacyA, legacyB)
	recordBaseline(s, governed.ID, governor)
	// An old policy store keeps the approval record but has no attributable
	// dependency baseline. Readiness must remain explicitly unknown.
	s.PolicyVersion = lock.PolicyLegacy
	claims := []model.Claim{missing, retired, retiredDep, cycleA, cycleB, governor, governed, legacyA, legacyB}
	got := Compute(claims, s, nil)
	if !hasCondition(got[missing.ID], ConditionMissingDependency, missing.ID, "fixture.contract.does-not-exist") {
		t.Fatalf("missing required dependency must be visible: %+v", got[missing.ID].DependencyConditions)
	}
	if !hasCondition(got[retired.ID], ConditionRetiredDependency, retired.ID, retiredDep.ID) {
		t.Fatalf("retired required dependency must be visible: %+v", got[retired.ID].DependencyConditions)
	}
	if !hasCondition(got[cycleA.ID], ConditionDependencyCycle, cycleA.ID, cycleB.ID, cycleA.ID) {
		t.Fatalf("cycle must be cut and reported with its path: %+v", got[cycleA.ID].DependencyConditions)
	}
	if !hasCondition(got[cycleB.ID], ConditionDependencyCycle, cycleB.ID, cycleA.ID, cycleB.ID) {
		t.Fatalf("reverse cycle assessment must also terminate and report its path: %+v", got[cycleB.ID].DependencyConditions)
	}
	if !got[governed.ID].DependencyReady || hasCondition(got[governed.ID], ConditionDependencyUnapproved, governed.ID, governor.ID) {
		t.Fatalf("governed_by must remain outside approval prerequisites: %+v", got[governed.ID])
	}
	if got[legacyB.ID].DependencyReady || !hasCondition(got[legacyB.ID], ConditionUnknownHistoricalBaseline, legacyB.ID, legacyA.ID) {
		t.Fatalf("legacy missing baseline must remain unknown: %+v", got[legacyB.ID])
	}
}
