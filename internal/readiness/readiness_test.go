package readiness

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

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
	// A remains locally approved here; C's invalid standing approval is the
	// blocking input, so B must retain the complete transitive condition path.
	if !hasCondition(got[b.ID], ConditionDependencyUnapproved, b.ID, a.ID, c.ID) {
		t.Fatalf("B must preserve B->A->C for the transitive invalid prerequisite: %+v", got[b.ID].DependencyConditions)
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

func TestComputeReconvergingDiamondDefeatsTerminalAndFirstHopIdentities(t *testing.T) {
	// Shape: X -> (E, F) -> B -> (A, D) -> C
	// There are 4 distinct routes from X to C:
	//   X -> E -> B -> A -> C
	//   X -> E -> B -> D -> C
	//   X -> F -> B -> A -> C
	//   X -> F -> B -> D -> C
	// C changes content and is reapproved.
	// A and D have direct drift on (A, C) and (D, C).
	// Under terminal-only deduplication, X would merge (A, C) and (D, C) into one cause for C,
	// which would break independent clearing.
	// Under first-hop-only deduplication, X would split by E vs F and lose the A vs D identity.
	// Under bounded owner identity (Owner, SourceKind, DependencyID), X has exactly 2 causes:
	//   one for (A, C) and one for (D, C).
	c := lockedClaim("fixture.reconverge.c")
	a := lockedClaim("fixture.reconverge.a", c.ID)
	d := lockedClaim("fixture.reconverge.d", c.ID)
	b := lockedClaim("fixture.reconverge.b", a.ID, d.ID)
	e := lockedClaim("fixture.reconverge.e", b.ID)
	f := lockedClaim("fixture.reconverge.f", b.ID)
	x := lockedClaim("fixture.reconverge.x", e.ID, f.ID)

	claims := []model.Claim{x, e, f, b, a, d, c}
	s := standingStore(claims...)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, d.ID, c)
	recordBaseline(s, b.ID, a)
	recordBaseline(s, b.ID, d)
	recordBaseline(s, e.ID, b)
	recordBaseline(s, f.ID, b)
	recordBaseline(s, x.ID, e)
	recordBaseline(s, x.ID, f)

	// Drift C and reapprove it
	changedC := c
	changedC.Body = "changed foundation C"
	claims[6] = changedC
	s.Ledger[c.ID] = lock.LedgerRecord{Subject: lock.SubjectClaim, Hash: lock.LockedClaimHash(changedC), At: "2026-09-06T00:00:00Z", Reason: "reapproved C"}

	got := Compute(claims, s, nil)
	xCauses := got[x.ID].ReviewCauses

	if len(xCauses) != 2 {
		t.Fatalf("X must have exactly 2 review causes (one for A->C drift, one for D->C drift), got %d: %+v", len(xCauses), xCauses)
	}

	hasAC := false
	hasDC := false
	for _, cause := range xCauses {
		if cause.DependencyID == c.ID {
			// Check whether it names A or D in the path
			if hasPath([]Cause{cause}, x.ID, e.ID, b.ID, a.ID, c.ID) {
				hasAC = true
			}
			if hasPath([]Cause{cause}, x.ID, e.ID, b.ID, d.ID, c.ID) {
				hasDC = true
			}
		}
	}
	if !hasAC || !hasDC {
		t.Fatalf("X must have both independent causes with representative shortest paths (hasAC=%v, hasDC=%v): %+v", hasAC, hasDC, xCauses)
	}

	// Now reapprove A against changedC (updating baseline for A -> C).
	recordBaseline(s, a.ID, changedC)
	got = Compute(claims, s, nil)
	xCausesAfter := got[x.ID].ReviewCauses

	if len(xCausesAfter) != 1 {
		t.Fatalf("clearing A->C baseline must leave exactly 1 cause on X, got %d: %+v", len(xCausesAfter), xCausesAfter)
	}
	if !hasPath([]Cause{xCausesAfter[0]}, x.ID, e.ID, b.ID, d.ID, c.ID) {
		t.Fatalf("D->C cause must remain on X: %+v", xCausesAfter)
	}
	if !got[x.ID].ReviewPending {
		t.Fatalf("X must remain review pending while D->C is unreviewed: %+v", got[x.ID])
	}
}

func TestComputeMultipleRoutesToOneSourceSingleDerivedRecord(t *testing.T) {
	// X -> (N1, N2, N3) -> Z
	// Z is unapproved draft. There are 3 routes from X to Z.
	// Bounded readiness must emit exactly ONE ConditionDependencyUnapproved for Z on X.
	// If the active witness route (X -> N1 -> Z) is removed, X still has Z unapproved via N2.
	z := model.Claim{ID: "fixture.routes.z", Status: model.StatusDraft, Body: "draft Z"}
	n1 := lockedClaim("fixture.routes.n1", z.ID)
	n2 := lockedClaim("fixture.routes.n2", z.ID)
	n3 := lockedClaim("fixture.routes.n3", z.ID)
	x := lockedClaim("fixture.routes.x", n1.ID, n2.ID, n3.ID)

	claims := []model.Claim{x, n1, n2, n3, z}
	s := standingStore(x, n1, n2, n3)
	recordBaseline(s, n1.ID, z)
	recordBaseline(s, n2.ID, z)
	recordBaseline(s, n3.ID, z)
	recordBaseline(s, x.ID, n1)
	recordBaseline(s, x.ID, n2)
	recordBaseline(s, x.ID, n3)

	got := Compute(claims, s, nil)
	xConds := got[x.ID].DependencyConditions

	if len(xConds) != 1 {
		t.Fatalf("X must have exactly 1 condition for unapproved Z, got %d: %+v", len(xConds), xConds)
	}
	if xConds[0].Kind != ConditionDependencyUnapproved || xConds[0].DependencyID != z.ID {
		t.Fatalf("expected ConditionDependencyUnapproved for Z, got: %+v", xConds[0])
	}
	// By lexicographical tie-break, n1 < n2 < n3, so witness path is X -> n1 -> z
	if !reflect.DeepEqual(xConds[0].Path, Path{x.ID, n1.ID, z.ID}) {
		t.Fatalf("expected shortest tie-break path [x, n1, z], got: %v", xConds[0].Path)
	}

	// Remove edge x -> n1 by updating x.RestsOn to only [n2, n3]
	x2 := x
	x2.RestsOn = []string{n2.ID, n3.ID}
	claims2 := []model.Claim{x2, n1, n2, n3, z}
	s2 := standingStore(x2, n1, n2, n3)
	recordBaseline(s2, n2.ID, z)
	recordBaseline(s2, n3.ID, z)
	recordBaseline(s2, x2.ID, n2)
	recordBaseline(s2, x2.ID, n3)

	got2 := Compute(claims2, s2, nil)
	xConds2 := got2[x2.ID].DependencyConditions

	if len(xConds2) != 1 {
		t.Fatalf("after removing route via n1, X must still have 1 condition for Z, got %d: %+v", len(xConds2), xConds2)
	}
	// Witness path must switch deterministically to [x, n2, z]
	if !reflect.DeepEqual(xConds2[0].Path, Path{x2.ID, n2.ID, z.ID}) {
		t.Fatalf("expected witness path to switch to [x, n2, z], got: %v", xConds2[0].Path)
	}
}

func TestComputeMultiNodeCycleAndSelfCycleValidClosedWitnesses(t *testing.T) {
	// 1. Self-cycle: A -> A
	selfA := lockedClaim("fixture.cycle.self", "fixture.cycle.self")
	sSelf := standingStore(selfA)
	gotSelf := Compute([]model.Claim{selfA}, sSelf, nil)
	if !hasCondition(gotSelf[selfA.ID], ConditionDependencyCycle, selfA.ID, selfA.ID) {
		t.Fatalf("self-cycle must produce [self, self]: %+v", gotSelf[selfA.ID].DependencyConditions)
	}

	// 2. 3-node cycle: A -> B -> C -> A
	// Must produce a valid closed cycle without duplicate non-edges.
	cA := lockedClaim("fixture.cycle.3.a", "fixture.cycle.3.b")
	cB := lockedClaim("fixture.cycle.3.b", "fixture.cycle.3.c")
	cC := lockedClaim("fixture.cycle.3.c", cA.ID)

	s3 := standingStore(cA, cB, cC)
	got3 := Compute([]model.Claim{cA, cB, cC}, s3, nil)

	// Check A: [A, B, C, A]
	if !hasCondition(got3[cA.ID], ConditionDependencyCycle, cA.ID, cB.ID, cC.ID, cA.ID) {
		t.Fatalf("3-node cycle from A must be [A, B, C, A]: %+v", got3[cA.ID].DependencyConditions)
	}
	// Check B: [B, C, A, B]
	if !hasCondition(got3[cB.ID], ConditionDependencyCycle, cB.ID, cC.ID, cA.ID, cB.ID) {
		t.Fatalf("3-node cycle from B must be [B, C, A, B]: %+v", got3[cB.ID].DependencyConditions)
	}
	// Check C: [C, A, B, C]
	if !hasCondition(got3[cC.ID], ConditionDependencyCycle, cC.ID, cA.ID, cB.ID, cC.ID) {
		t.Fatalf("3-node cycle from C must be [C, A, B, C]: %+v", got3[cC.ID].DependencyConditions)
	}

	// 3. Incoming tail: X -> A -> B -> C -> A
	tailX := lockedClaim("fixture.cycle.3.tail", cA.ID)
	sTail := standingStore(tailX, cA, cB, cC)
	recordBaseline(sTail, tailX.ID, cA)
	gotTail := Compute([]model.Claim{tailX, cA, cB, cC}, sTail, nil)
	if !hasCondition(gotTail[tailX.ID], ConditionDependencyCycle, tailX.ID, cA.ID, cB.ID, cC.ID, cA.ID) {
		t.Fatalf("incoming tail to cycle must produce [X, A, B, C, A]: %+v", gotTail[tailX.ID].DependencyConditions)
	}

	// 4. Outgoing tail: cycle A -> B -> C -> A, and C also rests on unapproved D.
	// Cycles must not hide independent obstacles!
	unapprovedD := model.Claim{ID: "fixture.cycle.3.d", Status: model.StatusDraft, Body: "draft D"}
	cCWithD := cC
	cCWithD.RestsOn = []string{cA.ID, unapprovedD.ID}
	sOutgoing := standingStore(cA, cB, cCWithD)
	gotOutgoing := Compute([]model.Claim{cA, cB, cCWithD, unapprovedD}, sOutgoing, nil)

	// A must report BOTH the cycle condition AND the unapproved D condition!
	if !hasCondition(gotOutgoing[cA.ID], ConditionDependencyCycle, cA.ID, cB.ID, cCWithD.ID, cA.ID) {
		t.Fatalf("A must report cycle condition: %+v", gotOutgoing[cA.ID].DependencyConditions)
	}
	if !hasCondition(gotOutgoing[cA.ID], ConditionDependencyUnapproved, cA.ID, cB.ID, cCWithD.ID, unapprovedD.ID) {
		t.Fatalf("A must also report unapproved D condition through the cycle: %+v", gotOutgoing[cA.ID].DependencyConditions)
	}
}

func TestComputeByteForByteInputImmutability(t *testing.T) {
	c := lockedClaim("fixture.mut.c")
	a := lockedClaim("fixture.mut.a", c.ID)
	a.Comments = []model.Comment{{ID: "t1", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Body: "thread"}}
	b := lockedClaim("fixture.mut.b", a.ID)

	claims := []model.Claim{b, a, c}
	s := standingStore(claims...)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, b.ID, a)
	flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{a.ID: {Reason: "audit flag"}}}

	claimsBefore, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	storeBefore, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	flagsBefore, err := json.Marshal(flags)
	if err != nil {
		t.Fatal(err)
	}

	// Run Compute
	_ = Compute(claims, s, flags)

	claimsAfter, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	storeAfter, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	flagsAfter, err := json.Marshal(flags)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(claimsBefore, claimsAfter) {
		t.Fatalf("Compute mutated input claims!\nBefore: %s\nAfter:  %s", claimsBefore, claimsAfter)
	}
	if !reflect.DeepEqual(storeBefore, storeAfter) {
		t.Fatalf("Compute mutated input store!\nBefore: %s\nAfter:  %s", storeBefore, storeAfter)
	}
	if !reflect.DeepEqual(flagsBefore, flagsAfter) {
		t.Fatalf("Compute mutated input flags!\nBefore: %s\nAfter:  %s", flagsBefore, flagsAfter)
	}
}

func TestComputeLayeredDenseDAGScaleBounds(t *testing.T) {
	// Build a dense layered DAG:
	// 8 layers, 5 nodes per layer = 40 claims.
	// Each node in layer L rests on all 5 nodes in layer L+1.
	// Total routes from any layer 0 node to leaf layer 7: 5^7 = 78,125 distinct routes!
	// All claims are draft, which is the exact precondition for Issue #62.
	const layers = 8
	const width = 5

	var allClaims []model.Claim
	var roots []string

	for l := 0; l < layers; l++ {
		var nextLayerIDs []string
		if l < layers-1 {
			for w := 0; w < width; w++ {
				nextLayerIDs = append(nextLayerIDs, fmt.Sprintf("claim.l%d.w%d", l+1, w))
			}
		}
		for w := 0; w < width; w++ {
			id := fmt.Sprintf("claim.l%d.w%d", l, w)
			claim := model.Claim{
				ID:      id,
				Facet:   "contract",
				Module:  "dense",
				Status:  model.StatusDraft,
				Body:    fmt.Sprintf("Dense claim layer %d node %d", l, w),
				RestsOn: nextLayerIDs,
			}
			allClaims = append(allClaims, claim)
			if l == 0 {
				roots = append(roots, id)
			}
		}
	}

	start := time.Now()
	assessments := Compute(allClaims, nil, nil)
	elapsed := time.Since(start)

	// Runtime bound: 40 claims must finish well under 100ms
	if elapsed > 200*time.Millisecond {
		t.Fatalf("dense DAG traversal took too long: %v (expected < 200ms)", elapsed)
	}

	// For each root:
	// There are 7 layers below it, each with 5 nodes = 35 reachable dependencies.
	// Number of DependencyCondition records must be EXACTLY 35, NOT 78,125!
	for _, rootID := range roots {
		a, ok := assessments[rootID]
		if !ok {
			t.Fatalf("missing assessment for root %s", rootID)
		}
		condCount := len(a.DependencyConditions)
		if condCount != 35 {
			t.Fatalf("root %s produced %d conditions, want exactly 35 distinct dependency conditions (Issue 62 route explosion)", rootID, condCount)
		}
		if a.DependencyReady || a.Ready {
			t.Fatalf("root %s cannot be dependency ready with all draft dependencies", rootID)
		}
	}

	// Serialized size bound:
	// Marshaling the entire assessment map for all 40 claims must be bounded (< 500 KB).
	serialized, err := json.Marshal(assessments)
	if err != nil {
		t.Fatalf("failed to marshal assessments: %v", err)
	}
	if len(serialized) > 500*1024 {
		t.Fatalf("serialized assessments size %d bytes exceeded 500 KB budget", len(serialized))
	}

	// Allocations bound:
	allocs := testing.AllocsPerRun(10, func() {
		_ = Compute(allClaims, nil, nil)
	})
	// Across all 40 claims on dense graph, allocations must be strictly bounded (< 20,000 total allocs)
	if allocs > 20000 {
		t.Fatalf("dense DAG allocations per run %.0f exceeded budget 20000", allocs)
	}

	t.Logf("Dense DAG (40 claims, 78,125 paths/root): %d conditions/root, %d bytes total, elapsed %v, allocs/run %.0f",
		len(assessments[roots[0]].DependencyConditions), len(serialized), elapsed, allocs)
}

func TestComputeSingletonAndBatchEvaluatorParity(t *testing.T) {
	// Verify that singleton assessment and batch assessment yield identical results.
	c := lockedClaim("fixture.parity.c")
	a := lockedClaim("fixture.parity.a", c.ID)
	b := lockedClaim("fixture.parity.b", a.ID)
	claims := []model.Claim{b, a, c}
	s := standingStore(claims...)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, b.ID, a)

	// Batch compute all 3 claims
	batchResult := Compute(claims, s, nil)

	// Individual singleton computes (providing full claim corpus for dependency lookups)
	bSingle := Compute(claims, s, nil)[b.ID]
	aSingle := Compute(claims, s, nil)[a.ID]
	cSingle := Compute(claims, s, nil)[c.ID]

	if !reflect.DeepEqual(batchResult[b.ID], bSingle) {
		t.Fatalf("B batch and singleton assessments differ:\nBatch: %+v\nSingle: %+v", batchResult[b.ID], bSingle)
	}
	if !reflect.DeepEqual(batchResult[a.ID], aSingle) {
		t.Fatalf("A batch and singleton assessments differ:\nBatch: %+v\nSingle: %+v", batchResult[a.ID], aSingle)
	}
	if !reflect.DeepEqual(batchResult[c.ID], cSingle) {
		t.Fatalf("C batch and singleton assessments differ:\nBatch: %+v\nSingle: %+v", batchResult[c.ID], cSingle)
	}
}

func TestComputeDifferentialComparison(t *testing.T) {
	// A small acyclic diamond graph comparing expected semantics.
	// B -> (A, D) -> C
	// Known historical defects (such as the cyclePath duplicate non-edge on 3-node cycles)
	// are explicitly named exceptions; on acyclic graphs, the set of reachable causes
	// and conditions matches the bounded fact identities.
	c := lockedClaim("fixture.diff.c")
	a := lockedClaim("fixture.diff.a", c.ID)
	d := lockedClaim("fixture.diff.d", c.ID)
	b := lockedClaim("fixture.diff.b", a.ID, d.ID)

	claims := []model.Claim{b, a, d, c}
	s := standingStore(claims...)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, d.ID, c)
	recordBaseline(s, b.ID, a)
	recordBaseline(s, b.ID, d)

	got := Compute(claims, s, nil)

	// All are locked and clean: Ready must be true
	for _, id := range []string{b.ID, a.ID, d.ID, c.ID} {
		if !got[id].Ready || !got[id].LocalApproved || !got[id].DependencyReady || got[id].ReviewPending {
			t.Fatalf("clean graph claim %s must be Ready: %+v", id, got[id])
		}
	}
}
