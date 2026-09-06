package readiness

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"sort"
	"strings"
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

// This is repeated-read determinism, not a singleton/batch lock-policy proof.
func TestComputeRepeatedEvaluationStable(t *testing.T) {
	c := lockedClaim("fixture.parity.c")
	a := lockedClaim("fixture.parity.a", c.ID)
	b := lockedClaim("fixture.parity.b", a.ID)
	claims := []model.Claim{b, a, c}
	s := standingStore(claims...)
	recordBaseline(s, a.ID, c)
	recordBaseline(s, b.ID, a)
	want := Compute(claims, s, nil)
	for repeat := 0; repeat < 3; repeat++ {
		if got := Compute(claims, s, nil); !reflect.DeepEqual(want, got) {
			t.Fatalf("readiness changed on repeated evaluation %d: want=%+v got=%+v", repeat, want, got)
		}
	}
}

func TestAuditEmptyFlagPropagation(t *testing.T) {
	a := lockedClaim("audit.a")
	b := lockedClaim("audit.b", a.ID)
	s := standingStore(a, b)
	recordBaseline(s, b.ID, a)
	got := Compute([]model.Claim{a, b}, s, &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{a.ID: {Reason: ""}}})
	if !got[a.ID].ReviewPending {
		t.Fatal("fixture must have local flag cause")
	}
	if !got[b.ID].ReviewPending || got[b.ID].Ready {
		t.Fatalf("active flag lost upstream: a=%+v b=%+v", got[a.ID], got[b.ID])
	}
}

func TestAuditDraftDriftConsistency(t *testing.T) {
	c := lockedClaim("audit.c")
	a := lockedClaim("audit.a", c.ID)
	b := lockedClaim("audit.b", a.ID)
	s := standingStore(a, b, c)
	recordBaseline(s, a.ID, c)
	a.Status = model.StatusDraft
	recordBaseline(s, b.ID, a)
	c.Body += " changed"
	s.Ledger[c.ID] = lock.LedgerRecord{Subject: lock.SubjectClaim, Hash: lock.LockedClaimHash(c)}
	r := s.Ledger[a.ID]
	r.ReleasedAt = "2026-09-06T00:00:00Z"
	s.Ledger[a.ID] = r
	got := Compute([]model.Claim{a, b, c}, s, nil)
	for _, cause := range got[b.ID].ReviewCauses {
		if cause.SourceKind == CauseDirectDependencyChange && cause.DependencyID == c.ID {
			t.Fatalf("inherited drift from draft owner absent locally: a=%+v b=%+v", got[a.ID], got[b.ID])
		}
	}
}

func TestIndependentSourceIdentityCollision(t *testing.T) {
	a := lockedClaim("a")
	a.Comments = []model.Comment{{ID: "t1", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Body: "review"}}
	b := lockedClaim("b", "a")
	s := standingStore(a, b)
	recordBaseline(s, "b", a)
	flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{"a": {Reason: "t1"}}}
	gotB := Compute([]model.Claim{a, b}, s, flags)["b"]
	kinds := map[CauseKind]bool{}
	for _, c := range gotB.Causes {
		kinds[c.SourceKind] = true
	}
	if !kinds[CauseOwnFlag] || !kinds[CauseOwnThread] {
		t.Fatalf("independent thread and flag must both survive; got %+v", gotB.Causes)
	}
}

func TestAuditShortestSCCWitness(t *testing.T) {
	x := lockedClaim("x", "a", "z")
	a := lockedClaim("a", "b")
	b := lockedClaim("b", "z")
	z := lockedClaim("z", "a", "z")
	claims := []model.Claim{x, a, b, z}
	got := Compute(claims, standingStore(claims...), nil)
	found := false
	for _, c := range got[x.ID].DependencyConditions {
		if c.Kind == ConditionDependencyCycle {
			found = true
			if len(c.Path) != 3 {
				t.Fatalf("shortest cycle witness is [x z z] (len 3), got %v (len %d)", c.Path, len(c.Path))
			}
			if !reflect.DeepEqual(c.Path, Path{"x", "z", "z"}) {
				t.Fatalf("expected witness [x z z], got %v", c.Path)
			}
		}
	}
	if !found {
		t.Fatal("expected cycle condition was not found")
	}
}

func auditIdentities(a Assessment) []string {
	out := []string{}
	set := map[string]bool{}
	for _, c := range a.Causes {
		owner := c.Path[len(c.Path)-1]
		if c.DependencyID != "" {
			owner = c.Path[len(c.Path)-2]
		}
		set[fmt.Sprintf("cause|%s|%s|%s", owner, c.SourceKind, c.DependencyID)] = true
	}
	for _, c := range a.Conditions {
		owner := ""
		if c.Kind == ConditionUnknownHistoricalBaseline {
			owner = c.Path[len(c.Path)-2]
		}
		set[fmt.Sprintf("condition|%s|%s|%s", owner, c.Kind, c.DependencyID)] = true
	}
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestIndependentDifferentialDAG(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		claims := []model.Claim{}
		flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{}}
		for i := 0; i < 6; i++ {
			c := lockedClaim(fmt.Sprintf("n%d", i))
			if rng.Intn(3) == 0 {
				c.Status = model.StatusDraft
			}
			if rng.Intn(5) == 0 {
				c.Comments = []model.Comment{{ID: fmt.Sprintf("thread%d", i), Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Body: "review"}}
			}
			if rng.Intn(3) == 0 {
				reason := ""
				if rng.Intn(2) == 0 {
					reason = "flag"
				}
				flags.Flags[c.ID] = reaudit.PendingFlag{Reason: reason}
			}
			for j := i + 1; j < 6; j++ {
				if rng.Intn(2) == 0 {
					c.RestsOn = append(c.RestsOn, fmt.Sprintf("n%d", j))
				}
			}
			if rng.Intn(4) == 0 {
				c.Governed.Type = "n5"
			}
			claims = append(claims, c)
		}
		s := standingStore(claims...)
		if seed%2 == 0 {
			s.PolicyVersion = lock.PolicyLegacy
		}
		for _, c := range claims {
			for _, dep := range lock.BaselineDependencyIDs(c) {
				if rng.Intn(5) == 0 {
					continue
				}
				for _, d := range claims {
					if dep == d.ID {
						recordBaseline(s, c.ID, d)
					}
				}
				if rng.Intn(3) == 0 {
					s.Hashes[c.ID][dep] = "stale"
				}
			}
		}
		old := oracleCompute(claims, s, flags)
		got := Compute(claims, s, flags)
		for id, a := range got {
			b := old[id]
			if a.LocalApproved != b.LocalApproved || a.DependencyReady != b.DependencyReady || a.ReviewPending != b.ReviewPending || a.Ready != b.Ready || !reflect.DeepEqual(auditIdentities(a), auditIdentities(b)) {
				t.Fatalf("seed=%d id=%s candidate=%+v oracle=%+v", seed, id, a, b)
			}
		}
	}
	t.Log("200 six-node DAGs, seeds 0..199; pinned d115399 oracle; four truth values and normalized independent identities")
}

func auditLex(a, b []string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func TestIndependentCycleOracle(t *testing.T) {
	testIndependentCycleOracle(t, false)
}

func TestIndependentLifecycleCycleOracle(t *testing.T) {
	testIndependentCycleOracle(t, true)
}

func testIndependentCycleOracle(t *testing.T, lifecycle bool) {
	for seed := int64(0); seed < 400; seed++ {
		rng := rand.New(rand.NewSource(seed))
		n := 5
		claims := make([]model.Claim, n)
		reach := make([][]bool, n)
		for i := 0; i < n; i++ {
			reach[i] = make([]bool, n)
			claims[i] = lockedClaim(fmt.Sprintf("n%d", i))
			for j := 0; j < n; j++ {
				if rng.Intn(4) == 0 {
					claims[i].RestsOn = append(claims[i].RestsOn, fmt.Sprintf("n%d", j))
					reach[i][j] = true
				}
			}
		}
		// Independent lifecycle predicate: do not reuse production eligibility.
		consumable := func(c model.Claim) bool {
			status := strings.ToLower(strings.TrimSpace(string(c.Status)))
			return status == "locked" || status == "draft" || status == ""
		}
		if lifecycle {
			statuses := []model.Status{model.StatusLocked, model.StatusDraft, "retired", "unreadable", ""}
			for i := range claims {
				claims[i].Status = statuses[rng.Intn(len(statuses))]
			}
			for i := range claims {
				for j := range claims {
					reach[i][j] = reach[i][j] && consumable(claims[i]) && consumable(claims[j])
				}
			}
		}
		for k := 0; k < n; k++ {
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					reach[i][j] = reach[i][j] || (reach[i][k] && reach[k][j])
				}
			}
		}
		comp := map[string]string{}
		for i := 0; i < n; i++ {
			if !reach[i][i] {
				continue
			}
			for j := 0; j < n; j++ {
				if reach[i][j] && reach[j][i] {
					comp[claims[i].ID] = claims[j].ID
					break
				}
			}
		}
		got := Compute(claims, standingStore(claims...), nil)
		byID := map[string]model.Claim{}
		for _, c := range claims {
			byID[c.ID] = c
		}
		permuted := append([]model.Claim(nil), claims...)
		for i := range permuted {
			permuted[i].RestsOn = append([]string(nil), permuted[i].RestsOn...)
			slices.Reverse(permuted[i].RestsOn)
		}
		slices.Reverse(permuted)
		if !reflect.DeepEqual(got, Compute(permuted, standingStore(permuted...), nil)) {
			t.Fatalf("seed=%d lifecycle=%v reordered claims/edges changed assessment", seed, lifecycle)
		}

		for _, root := range claims {
			// Independently collect terminal invalid prerequisites, including a
			// return to an invalid root. Cycle-only equality cannot detect a lost
			// blocker after the invalid root is removed from cycle membership.
			if lifecycle {
				wantInvalid := map[string]bool{}
				seen := map[string]bool{}
				var visit func(string)
				visit = func(id string) {
					if seen[id] {
						return
					}
					seen[id] = true
					for _, dep := range byID[id].RestsOn {
						if !consumable(byID[dep]) {
							wantInvalid[dep] = true
						} else {
							visit(dep)
						}
					}
				}
				visit(root.ID)
				actualInvalid := map[string]bool{}
				for _, cond := range got[root.ID].Conditions {
					if cond.Kind == ConditionRetiredDependency || cond.Kind == ConditionUnreadableDependency {
						actualInvalid[cond.DependencyID] = true
					}
				}
				if !reflect.DeepEqual(wantInvalid, actualInvalid) {
					t.Fatalf("seed=%d root=%s invalid blockers: want=%v got=%v", seed, root.ID, wantInvalid, actualInvalid)
				}
				if len(wantInvalid) > 0 && got[root.ID].DependencyReady {
					t.Fatalf("seed=%d root=%s invalid prerequisites must block readiness", seed, root.ID)
				}
			}

			best := map[string]Path{}
			var walk func(Path)
			walk = func(p Path) {
				for _, next := range byID[p[len(p)-1]].RestsOn {
					if !consumable(byID[next]) {
						continue
					}
					q := append(append(Path(nil), p...), next)
					repeated := false
					for _, prev := range p {
						if next == prev {
							repeated = true
							break
						}
					}
					if repeated {
						k := comp[next]
						b := best[k]
						if b == nil || len(q) < len(b) || (len(q) == len(b) && auditLex(q, b)) {
							best[k] = q
						}
					} else {
						walk(q)
					}
				}
			}
			walk(Path{root.ID})
			actual := map[string]Path{}
			for _, c := range got[root.ID].Conditions {
				if c.Kind == ConditionDependencyCycle {
					component, exists := comp[c.DependencyID]
					if !exists {
						t.Fatalf("seed=%d root=%s cycle has no eligible component: %+v", seed, root.ID, c)
					}
					if _, duplicate := actual[component]; duplicate {
						t.Fatalf("seed=%d root=%s duplicate component: %+v", seed, root.ID, c)
					}
					actual[comp[c.DependencyID]] = c.Path
				}
			}
			if !reflect.DeepEqual(best, actual) {
				t.Fatalf("seed=%d root=%s expected=%v actual=%v", seed, root.ID, best, actual)
			}
		}
	}
	t.Log("400 five-node directed graphs, seeds 0..399; bounded exhaustive simple-walk oracle checks existence, all edges, global length and lexicographic tie-break per SCC")
}

func TestPermutationInvariance(t *testing.T) {
	nodes := []string{"n0", "n1", "n2", "n3", "n4"}
	for seed := 0; seed < 10; seed++ {
		var claims []model.Claim
		flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{}}
		for i, id := range nodes {
			var deps []string
			for j := i + 1; j < len(nodes); j++ {
				if (seed+i+j)%2 == 0 {
					deps = append(deps, nodes[j])
				}
			}
			c := lockedClaim(id, deps...)
			if (seed+i)%3 == 0 {
				c.Status = model.StatusDraft
			}
			if (seed+i)%4 == 0 {
				flags.Flags[id] = reaudit.PendingFlag{Reason: fmt.Sprintf("flag%d", seed)}
			}
			claims = append(claims, c)
		}
		s := standingStore(claims...)
		res1 := Compute(claims, s, flags)
		var reversed []model.Claim
		for i := len(claims) - 1; i >= 0; i-- {
			reversed = append(reversed, claims[i])
		}
		res2 := Compute(reversed, s, flags)
		if !reflect.DeepEqual(res1, res2) {
			t.Fatalf("seed %d: Compute is not invariant to claim ordering", seed)
		}
	}
}

func TestAuditInvalidBoundaryCutoff(t *testing.T) {
	for _, status := range []model.Status{"retired", "unreadable"} {
		t.Run(string(status), func(t *testing.T) {
			c := lockedClaim("c")
			b := lockedClaim("b", "c")
			b.Status = status
			a := lockedClaim("a", "b")
			s := standingStore(a, b, c)
			recordBaseline(s, "a", b)
			recordBaseline(s, "b", c)
			flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{"c": {Reason: "needs review"}}}
			claims := []model.Claim{a, b, c}

			old := oracleCompute(claims, s, flags)["a"]
			got := Compute(claims, s, flags)["a"]

			if old.ReviewPending {
				t.Fatal("baseline oracle must stop before c")
			}
			if got.ReviewPending {
				t.Fatalf("review traversed non-consumable boundary: baseline=%+v candidate=%+v", old, got)
			}
			if got.DependencyReady {
				t.Fatalf("a should not have dependency ready when prerequisite is %s", status)
			}
			if got.Ready {
				t.Fatalf("a should not be ready when prerequisite is %s", status)
			}

			// Ensure ConditionRetiredDependency or ConditionUnreadableDependency is present on a
			var foundCondition bool
			for _, cond := range got.Conditions {
				if (status == "retired" && cond.Kind == ConditionRetiredDependency) ||
					(status == "unreadable" && cond.Kind == ConditionUnreadableDependency) {
					foundCondition = true
					if !reflect.DeepEqual(cond.Path, Path{"a", "b"}) {
						t.Fatalf("expected condition path [a, b], got %v", cond.Path)
					}
				}
			}
			if !foundCondition {
				t.Fatalf("expected condition for %s dependency on a", status)
			}

			// Ensure c's review cause is NOT inherited by a
			for _, cause := range got.Causes {
				if cause.DependencyID == "c" {
					t.Fatalf("a must not inherit review cause from c behind %s prerequisite: %+v", status, cause)
				}
			}
		})
	}
}

func TestAuditInvalidBoundaryAlternateRoute(t *testing.T) {
	for _, status := range []model.Status{"retired", "unreadable"} {
		t.Run(string(status), func(t *testing.T) {
			// Diamond graph:
			// a -> b (retired/unreadable) -> c (flagged)
			// a -> d (locked, valid)      -> c (flagged)
			c := lockedClaim("c")
			b := lockedClaim("b", "c")
			b.Status = status
			d := lockedClaim("d", "c")
			a := lockedClaim("a", "b", "d")
			s := standingStore(a, b, c, d)
			recordBaseline(s, "a", b)
			recordBaseline(s, "a", d)
			recordBaseline(s, "b", c)
			recordBaseline(s, "d", c)
			flags := &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{"c": {Reason: "needs review"}}}
			claims := []model.Claim{a, b, c, d}

			got := Compute(claims, s, flags)["a"]
			old := oracleCompute(claims, s, flags)["a"]

			if !got.ReviewPending {
				t.Fatalf("a should have review_pending=true because c is reachable via valid route d: %+v", got)
			}
			if !old.ReviewPending {
				t.Fatalf("oracle should also have review_pending=true via valid route d: %+v", old)
			}
			if got.DependencyReady {
				t.Fatalf("a should not have dependency ready because b is %s", status)
			}

			// Verify the inherited review cause from c uses path [a, d, c] (not through b)
			var foundCause bool
			expectedPath := Path{"a", "d", "c"}
			for _, cause := range got.Causes {
				if cause.Kind == CauseUpstreamDependencyReview && reflect.DeepEqual(cause.Path, expectedPath) {
					foundCause = true
					if cause.SourceKind != CauseOwnFlag {
						t.Fatalf("expected SourceKind own_flag, got %v", cause.SourceKind)
					}
				}
			}
			if !foundCause {
				t.Fatalf("expected inherited review cause from c on path [a, d, c]: %+v", got.Causes)
			}
		})
	}
}

func TestAuditShortestDAGWitnesses400(t *testing.T) {
	for seed := int64(1000); seed < 1400; seed++ {
		rng := rand.New(rand.NewSource(seed))
		var claims []model.Claim
		byID := map[string]model.Claim{}
		for i := 0; i < 7; i++ {
			c := model.Claim{ID: fmt.Sprintf("n%d", i), Status: model.StatusDraft, Body: "draft"}
			for j := i + 1; j < 9; j++ {
				if rng.Intn(3) == 0 {
					c.RestsOn = append(c.RestsOn, fmt.Sprintf("n%d", j))
				}
			}
			rng.Shuffle(len(c.RestsOn), func(a, b int) { c.RestsOn[a], c.RestsOn[b] = c.RestsOn[b], c.RestsOn[a] })
			claims = append(claims, c)
			byID[c.ID] = c
		}
		got := Compute(claims, nil, nil)
		for _, root := range claims {
			expected := map[string]Path{}
			var walk func(Path)
			walk = func(p Path) {
				for _, d := range byID[p[len(p)-1]].RestsOn {
					q := append(append(Path(nil), p...), d)
					old := expected[d]
					if old == nil || len(q) < len(old) || (len(q) == len(old) && auditLex(q, old)) {
						expected[d] = q
					}
					if _, ok := byID[d]; ok {
						walk(q)
					}
				}
			}
			walk(Path{root.ID})
			actual := map[string]Path{}
			for _, c := range got[root.ID].Conditions {
				actual[c.DependencyID] = c.Path
			}
			if !reflect.DeepEqual(expected, actual) {
				t.Fatalf("seed=%d root=%s expected=%v got=%v", seed, root.ID, expected, actual)
			}
		}
		rng.Shuffle(len(claims), func(a, b int) { claims[a], claims[b] = claims[b], claims[a] })
		for i := range claims {
			rng.Shuffle(len(claims[i].RestsOn), func(a, b int) {
				claims[i].RestsOn[a], claims[i].RestsOn[b] = claims[i].RestsOn[b], claims[i].RestsOn[a]
			})
		}
		if !reflect.DeepEqual(got, Compute(claims, nil, nil)) {
			t.Fatalf("seed %d: reordered edges/claims changed witnesses", seed)
		}
	}
}
