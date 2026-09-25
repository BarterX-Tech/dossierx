package readiness

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

// Project claims (NIT-25) are graph nodes like any other: id project.<slug>,
// scope: project, no module and no facet. Readiness is id-driven, so nothing
// here special-cases them — which is exactly what these tests pin, shape by
// shape from the proof matrix: a single edge to a draft project claim, a chain
// through an unchanged intermediate, a cycle that passes through a project
// claim (with and without tails, and beside an independent obstacle), and a
// rests_on: {none: true} project claim as a leaf. Every case is also run
// through the pinned oracle, and the inputs are checked byte for byte for
// mutation.

func projectClaim(id string, status model.Status, rests ...string) model.Claim {
	c := model.Claim{ID: id, Scope: model.ScopeProject, Status: status, Body: id + " body"}
	if len(rests) == 0 {
		c.RestsOn = model.RestsNone("the roof above it is the constitution")
	} else {
		c.RestsOn = model.RestsOnIDs(rests...)
	}
	return c
}

func emptyFlags() *reaudit.FlagStore {
	return &reaudit.FlagStore{Flags: map[string]reaudit.PendingFlag{}}
}

// assertAgreesWithOracle compares the four truth values and the normalized
// independent identities of every assessment against the pinned oracle, the
// same comparison TestIndependentDifferentialDAG makes over generated graphs.
func assertAgreesWithOracle(t *testing.T, claims []model.Claim, s *lock.Store, got map[string]Assessment) {
	t.Helper()
	old := oracleCompute(claims, s, emptyFlags())
	for id, a := range got {
		b := old[id]
		if a.LocalApproved != b.LocalApproved || a.DependencyReady != b.DependencyReady || a.ReviewPending != b.ReviewPending || a.Ready != b.Ready || !reflect.DeepEqual(auditIdentities(a), auditIdentities(b)) {
			t.Fatalf("candidate disagrees with the oracle on %s:\ncandidate=%+v\noracle=%+v", id, a, b)
		}
	}
}

// assertNoMutation runs Compute and fails if the claims or the store changed
// byte for byte.
func assertNoMutation(t *testing.T, claims []model.Claim, s *lock.Store) map[string]Assessment {
	t.Helper()
	before, err := json.Marshal(struct {
		Claims []model.Claim
		Store  *lock.Store
	}{claims, s})
	if err != nil {
		t.Fatal(err)
	}
	got := Compute(claims, s, emptyFlags())
	after, err := json.Marshal(struct {
		Claims []model.Claim
		Store  *lock.Store
	}{claims, s})
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("Compute mutated its inputs:\nbefore=%s\nafter=%s", before, after)
	}
	return got
}

// isRestsOnEdge reports whether from -> to is a declared rests_on edge.
func isRestsOnEdge(claims []model.Claim, from, to string) bool {
	for _, c := range claims {
		if c.ID != from {
			continue
		}
		for _, id := range c.RestsOn.IDs {
			if id == to {
				return true
			}
		}
	}
	return false
}

// assertClosedCycleWitness checks a cycle witness against the input: every
// hop is a real rests_on edge and the path closes on a node it visited.
func assertClosedCycleWitness(t *testing.T, claims []model.Claim, a Assessment) Path {
	t.Helper()
	for _, cond := range a.DependencyConditions {
		if cond.Kind != ConditionDependencyCycle {
			continue
		}
		p := cond.Path
		if len(p) < 2 {
			t.Fatalf("%s: cycle witness too short: %v", a.ClaimID, p)
		}
		for i := 0; i+1 < len(p); i++ {
			if !isRestsOnEdge(claims, p[i], p[i+1]) {
				t.Fatalf("%s: cycle witness %v walks a non-edge %s -> %s", a.ClaimID, p, p[i], p[i+1])
			}
		}
		last := p[len(p)-1]
		closed := false
		for _, n := range p[:len(p)-1] {
			if n == last {
				closed = true
			}
		}
		if !closed {
			t.Fatalf("%s: cycle witness %v does not close on a node it visited", a.ClaimID, p)
		}
		return p
	}
	t.Fatalf("%s: no dependency_cycle condition: %+v", a.ClaimID, a.DependencyConditions)
	return nil
}

func TestProjectClaimDraftKeepsDependentModuleClaimUnready(t *testing.T) {
	scope := projectClaim("project.scope", model.StatusDraft)
	overview := lockedClaim("widget.contract.overview", scope.ID)
	// A project claim resting on the module claim: the chain shape, so the
	// obstacle must propagate through the unchanged intermediate.
	retention := projectClaim("project.retention", model.StatusLocked, overview.ID)
	claims := []model.Claim{retention, scope, overview}
	s := standingStore(overview, retention)
	recordBaseline(s, overview.ID, scope)
	recordBaseline(s, retention.ID, overview)

	got := assertNoMutation(t, claims, s)
	assertAgreesWithOracle(t, claims, s, got)

	ov := got[overview.ID]
	if !ov.LocalApproved || ov.DependencyReady || ov.Ready || ov.ReviewPending {
		t.Fatalf("a module claim locked against a readable draft project claim is locally approved and not ready: %+v", ov)
	}
	if !hasCondition(ov, ConditionDependencyUnapproved, overview.ID, scope.ID) {
		t.Fatalf("the direct witness [overview scope] is missing: %+v", ov.DependencyConditions)
	}
	rt := got[retention.ID]
	if !rt.LocalApproved || rt.DependencyReady || rt.Ready {
		t.Fatalf("the project claim above the module claim inherits the obstacle: %+v", rt)
	}
	if !hasCondition(rt, ConditionDependencyUnapproved, retention.ID, overview.ID, scope.ID) {
		t.Fatalf("the chain witness [retention overview scope] is missing: %+v", rt.DependencyConditions)
	}
	sc := got[scope.ID]
	if sc.LocalApproved || !sc.DependencyReady || sc.Ready || len(sc.DependencyConditions) != 0 {
		t.Fatalf("a draft project claim resting on nothing is dependency-ready and not locally approved: %+v", sc)
	}

	// Scope is a store fact, not a graph fact: the same graph with the scope
	// field cleared, approved under its own ledger (the approval hash covers
	// every persisted field, scope included), assesses identically.
	unscoped := make([]model.Claim, len(claims))
	for i, c := range claims {
		c.Scope = ""
		unscoped[i] = c
	}
	sUnscoped := standingStore(unscoped[0], unscoped[2])
	recordBaseline(sUnscoped, unscoped[2].ID, unscoped[1])
	recordBaseline(sUnscoped, unscoped[0].ID, unscoped[2])
	if again := Compute(unscoped, sUnscoped, emptyFlags()); !reflect.DeepEqual(again, got) {
		t.Fatalf("readiness depended on the scope field:\nscoped=%+v\nunscoped=%+v", got, again)
	}
	// Input order does not matter.
	if again := Compute([]model.Claim{overview, retention, scope}, s, emptyFlags()); !reflect.DeepEqual(again, got) {
		t.Fatalf("readiness depended on input order")
	}
}

func TestProjectClaimApprovedClearsDependentsAndNoneIsALeaf(t *testing.T) {
	scope := projectClaim("project.scope", model.StatusLocked)
	overview := lockedClaim("widget.contract.overview", scope.ID)
	retention := projectClaim("project.retention", model.StatusLocked, overview.ID)
	claims := []model.Claim{scope, overview, retention}
	s := standingStore(scope, overview, retention)
	recordBaseline(s, overview.ID, scope)
	recordBaseline(s, retention.ID, overview)

	got := assertNoMutation(t, claims, s)
	assertAgreesWithOracle(t, claims, s, got)
	for _, id := range []string{scope.ID, overview.ID, retention.ID} {
		a := got[id]
		if !a.LocalApproved || !a.DependencyReady || !a.Ready || a.ReviewPending || len(a.DependencyConditions) != 0 || len(a.ReviewCauses) != 0 {
			t.Fatalf("%s must be ready with no conditions or causes once the project claim is approved: %+v", id, a)
		}
	}

	// A drift on the project claim's baseline propagates: the direct
	// dependent turns review-pending on its own cause, the claim above it on
	// the inherited one, and the leaf itself stays ready.
	s.Hashes[overview.ID][scope.ID] = "stale"
	drifted := Compute(claims, s, emptyFlags())
	assertAgreesWithOracle(t, claims, s, drifted)
	if a := drifted[scope.ID]; !a.Ready || a.ReviewPending {
		t.Fatalf("the rewritten project claim is itself ready: %+v", a)
	}
	if a := drifted[overview.ID]; !a.ReviewPending || a.Ready || !hasCause(a, CauseDirectDependencyChange, overview.ID, scope.ID) {
		t.Fatalf("the direct dependent must carry direct_dependency_change [overview scope]: %+v", a)
	}
	if a := drifted[retention.ID]; !a.ReviewPending || a.Ready || !hasCause(a, CauseUpstreamDependencyReview, retention.ID, overview.ID, scope.ID) {
		t.Fatalf("the claim above the unchanged intermediate must carry upstream_dependency_review [retention overview scope]: %+v", a)
	}
}

func TestProjectClaimCycleTerminatesWithValidWitnesses(t *testing.T) {
	// A two-node cycle through a project claim: the module claim rests on
	// the project claim and the project claim rests on the module's contract
	// claim, which the target lint allows.
	scope := projectClaim("project.scope", model.StatusLocked, "widget.contract.overview")
	overview := lockedClaim("widget.contract.overview", scope.ID)
	claims := []model.Claim{scope, overview}
	s := standingStore(scope, overview)
	recordBaseline(s, overview.ID, scope)
	recordBaseline(s, scope.ID, overview)

	got := assertNoMutation(t, claims, s)
	assertAgreesWithOracle(t, claims, s, got)
	for _, id := range []string{scope.ID, overview.ID} {
		if a := got[id]; a.DependencyReady || a.Ready {
			t.Fatalf("%s must stay unready inside a cycle: %+v", id, a)
		}
	}
	if p := assertClosedCycleWitness(t, claims, got[overview.ID]); !reflect.DeepEqual(p, Path{overview.ID, scope.ID, overview.ID}) {
		t.Fatalf("overview's cycle witness = %v, want [overview scope overview]", p)
	}
	if p := assertClosedCycleWitness(t, claims, got[scope.ID]); !reflect.DeepEqual(p, Path{scope.ID, overview.ID, scope.ID}) {
		t.Fatalf("scope's cycle witness = %v, want [scope overview scope]", p)
	}

	// An incoming tail: a second project claim resting on the module claim
	// reaches the cycle through a real edge and reports a closed witness with
	// that prefix.
	tail := projectClaim("project.retention", model.StatusLocked, overview.ID)
	withTail := []model.Claim{tail, scope, overview}
	sTail := standingStore(tail, scope, overview)
	recordBaseline(sTail, tail.ID, overview)
	recordBaseline(sTail, overview.ID, scope)
	recordBaseline(sTail, scope.ID, overview)
	gotTail := assertNoMutation(t, withTail, sTail)
	assertAgreesWithOracle(t, withTail, sTail, gotTail)
	if a := gotTail[tail.ID]; a.DependencyReady || a.Ready {
		t.Fatalf("a claim whose prerequisite sits in a cycle is not ready: %+v", a)
	}
	if p := assertClosedCycleWitness(t, withTail, gotTail[tail.ID]); !reflect.DeepEqual(p, Path{tail.ID, overview.ID, scope.ID, overview.ID}) {
		t.Fatalf("tail's cycle witness = %v, want [retention overview scope overview]", p)
	}

	// An outgoing tail: the project claim in the cycle also rests on a draft
	// leaf. The cycle must not hide that independent obstacle from the
	// module claim, and the draft is reported through real edges.
	draft := projectClaim("project.policy", model.StatusDraft)
	scopeWithDraft := scope
	scopeWithDraft.RestsOn = model.RestsOnIDs(overview.ID, draft.ID)
	withDraft := []model.Claim{scopeWithDraft, overview, draft}
	sDraft := standingStore(scopeWithDraft, overview)
	recordBaseline(sDraft, overview.ID, scopeWithDraft)
	recordBaseline(sDraft, scopeWithDraft.ID, overview)
	recordBaseline(sDraft, scopeWithDraft.ID, draft)
	gotDraft := assertNoMutation(t, withDraft, sDraft)
	assertAgreesWithOracle(t, withDraft, sDraft, gotDraft)
	assertClosedCycleWitness(t, withDraft, gotDraft[overview.ID])
	if !hasCondition(gotDraft[overview.ID], ConditionDependencyUnapproved, overview.ID, scopeWithDraft.ID, draft.ID) {
		t.Fatalf("the cycle hid the draft leaf behind it: %+v", gotDraft[overview.ID].DependencyConditions)
	}
	if a := gotDraft[draft.ID]; a.LocalApproved || !a.DependencyReady || a.Ready {
		t.Fatalf("the draft leaf is dependency-ready and not approved: %+v", a)
	}
}

func TestProjectClaimMissingAndSelfCycle(t *testing.T) {
	// A module claim resting on a project claim nothing declares.
	orphaned := lockedClaim("widget.contract.overview", "project.ghost")
	// A project claim resting on itself.
	self := projectClaim("project.self", model.StatusLocked, "project.self")
	claims := []model.Claim{orphaned, self}
	s := standingStore(orphaned, self)
	recordBaseline(s, self.ID, self)

	got := assertNoMutation(t, claims, s)
	assertAgreesWithOracle(t, claims, s, got)
	if a := got[orphaned.ID]; a.DependencyReady || a.Ready || !hasCondition(a, ConditionMissingDependency, orphaned.ID, "project.ghost") {
		t.Fatalf("a missing project claim must be a visible missing-dependency obstacle: %+v", a)
	}
	if a := got[self.ID]; a.DependencyReady || a.Ready {
		t.Fatalf("a self-cycle through a project claim must not be ready: %+v", a)
	}
	if p := assertClosedCycleWitness(t, claims, got[self.ID]); !reflect.DeepEqual(p, Path{self.ID, self.ID}) {
		t.Fatalf("self-cycle witness = %v, want [self self]", p)
	}
}
