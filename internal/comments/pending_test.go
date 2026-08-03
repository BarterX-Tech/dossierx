// pending_test.go covers PendingTriggers/Recompute — the "single authority the
// comment ops, reaudit, and the check/serve reconciler all consult so the three
// triggers can never diverge" — against the one thing that made that sentence
// untrue: this package used to carry its own hand-copied dependencyIDs, so
// widening lock's drift set left comments' copy behind.
//
// The consequence was not a missing test, it was a self-contradicting engine:
// lock.DetectStale wrote review_pending: true to the claim file while
// PendingTriggers reported no trigger at all, so `check` announced "1 claim(s)
// review_pending with no active trigger", `claim show` said review_trigger:
// none, and any comment op overwrote the flag away via Recompute. The tests
// below are written against lock.BaselineDependencyIDs by construction: there
// is no second list here to keep in step.
package comments

import (
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// governedPair returns a locked doctrine hub and a locked claim governed by it
// through governed_by.type ONLY — no mirrors, no rests_on naming the hub — plus
// a store already baselined at the hub's current content.
func governedPair(t *testing.T) (hub, child model.Claim, store *lock.Store) {
	t.Helper()
	hub = model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusLocked, Body: "doctrine v1"}
	child = model.Claim{
		ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusLocked,
		Body: "child", Governed: model.Governed{Type: hub.ID},
	}
	var err error
	store, err = lock.LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("lock.LoadStore: %v", err)
	}
	lock.RefreshBaseline(child, []model.Claim{hub, child}, store)
	return hub, child, store
}

func TestPendingTriggers_GovernorEditIsDrift(t *testing.T) {
	hub, child, store := governedPair(t)

	// Baselined and unchanged: no trigger at all.
	if drift, flag, open := PendingTriggers(child, []model.Claim{hub, child}, store, nil); drift || flag || open != 0 {
		t.Fatalf("a freshly baselined claim has no trigger; got drift=%v flag=%v open=%d", drift, flag, open)
	}

	// The governor's comparable content changes.
	hub.Body = "doctrine v2"
	drift, _, _ := PendingTriggers(child, []model.Claim{hub, child}, store, nil)
	if !drift {
		t.Fatalf("editing the governing claim must report drift=true — this is the trigger lock.DetectStale acted on when it wrote review_pending")
	}
	if !Recompute(child, []model.Claim{hub, child}, store, nil) {
		t.Fatalf("Recompute must agree with PendingTriggers; a comment op that disagrees erases the drift silently")
	}
}

// TestPendingTriggers_AgreesWithDetectStale is the invariant this package's own
// doc claims and the reason the hand-copy was deleted rather than widened: for
// the same claims and the same store, comments and lock must reach the same
// verdict. It is asserted over every edge type at once.
func TestPendingTriggers_AgreesWithDetectStale(t *testing.T) {
	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusLocked, Body: "doctrine v1"}
	mirrored := model.Claim{ID: "widget.contract.mirrored", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "mirrored"}
	rested := model.Claim{ID: "widget.contract.rested", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "rested"}

	cases := []struct {
		name  string
		child model.Claim
		edit  func(claims []model.Claim)
	}{
		{
			name:  "governed_by",
			child: model.Claim{ID: "c1", Facet: "contract", Module: "widget", Status: model.StatusLocked, Governed: model.Governed{Type: hub.ID}},
			edit:  func(claims []model.Claim) { claims[0].Body = "doctrine v2" },
		},
		{
			name:  "mirrors",
			child: model.Claim{ID: "c2", Facet: "contract", Module: "widget", Status: model.StatusLocked, Mirrors: []string{mirrored.ID}},
			edit:  func(claims []model.Claim) { claims[1].Body = "mirrored v2" },
		},
		{
			name:  "rests_on",
			child: model.Claim{ID: "c3", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{rested.ID}},
			edit:  func(claims []model.Claim) { claims[2].Body = "rested v2" },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := []model.Claim{hub, mirrored, rested, tc.child}
			store, err := lock.LoadStore(filepath.Join(t.TempDir(), "store.json"))
			if err != nil {
				t.Fatalf("lock.LoadStore: %v", err)
			}
			lock.RefreshBaseline(tc.child, claims, store)

			tc.edit(claims)
			drift, _, _ := PendingTriggers(tc.child, claims, store, nil)
			stale := false
			for _, c := range lock.DetectStale(claims, store) {
				if c.ID == tc.child.ID && c.ReviewPending {
					stale = true
				}
			}
			if drift != stale {
				t.Fatalf("comments.PendingTriggers drift=%v but lock.DetectStale review_pending=%v — the two must never diverge", drift, stale)
			}
			if !drift {
				t.Fatalf("expected the %s edit to be drift", tc.name)
			}
		})
	}
}

// TestPendingTriggers_GovernedByNoneIsNotADependency: "none" is the sentinel for
// "deliberately ungoverned". It names no claim, so it can never drift.
func TestPendingTriggers_GovernedByNoneIsNotADependency(t *testing.T) {
	claim := model.Claim{
		ID: "widget.contract.ungoverned", Facet: "contract", Module: "widget", Status: model.StatusLocked,
		Body: "ungoverned", Governed: model.Governed{Type: "none", Reason: "deliberately ungoverned"},
	}
	store, err := lock.LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("lock.LoadStore: %v", err)
	}
	lock.RefreshBaseline(claim, []model.Claim{claim}, store)

	if _, known := store.Baseline(claim.ID, "none"); known {
		t.Fatalf("governed_by.type: none must create no baseline; store has %v", store.Hashes)
	}
	if drift, _, _ := PendingTriggers(claim, []model.Claim{claim}, store, nil); drift {
		t.Fatalf("governed_by.type: none must never report drift")
	}
}
