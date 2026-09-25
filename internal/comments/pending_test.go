// pending_test.go covers PendingTriggers/Recompute — the "single authority the
// comment ops, reaudit, and the check/serve reconciler all consult so the three
// triggers can never diverge" — against the one thing that made that sentence
// untrue: this package used to carry its own hand-copied dependencyIDs, so
// widening lock's drift set left comments' copy behind.
package comments

import (
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func restsOnPair(t *testing.T) (hub, child model.Claim, store *lock.Store) {
	t.Helper()
	hub = model.Claim{ID: "widget.contract.hub", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "hub v1"}
	child = model.Claim{
		ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusLocked,
		Body: "child", RestsOn: model.RestsOnIDs(hub.ID),
	}
	var err error
	store, err = lock.LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("lock.LoadStore: %v", err)
	}
	lock.RefreshBaseline(child, []model.Claim{hub, child}, store)
	return hub, child, store
}

func TestPendingTriggers_DependencyEditIsDrift(t *testing.T) {
	hub, child, store := restsOnPair(t)

	if drift, flag, open := PendingTriggers(child, []model.Claim{hub, child}, store, nil); drift || flag || open != 0 {
		t.Fatalf("a freshly baselined claim has no trigger; got drift=%v flag=%v open=%d", drift, flag, open)
	}

	hub.Body = "hub v2"
	drift, _, _ := PendingTriggers(child, []model.Claim{hub, child}, store, nil)
	if !drift {
		t.Fatalf("editing the rests_on target must report drift=true")
	}
	if !Recompute(child, []model.Claim{hub, child}, store, nil) {
		t.Fatalf("Recompute must agree with PendingTriggers")
	}
}

func TestPendingTriggers_AgreesWithDetectStale(t *testing.T) {
	hub := model.Claim{ID: "widget.contract.hub", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "hub v1"}
	rested := model.Claim{ID: "widget.contract.rested", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "rested"}

	cases := []struct {
		name  string
		child model.Claim
		edit  func(claims []model.Claim)
	}{
		{
			name:  "rests_on hub",
			child: model.Claim{ID: "c1", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(hub.ID)},
			edit:  func(claims []model.Claim) { claims[0].Body = "hub v2" },
		},
		{
			name:  "rests_on rested",
			child: model.Claim{ID: "c3", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(rested.ID)},
			edit:  func(claims []model.Claim) { claims[1].Body = "rested v2" },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := []model.Claim{hub, rested, tc.child}
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

func TestPendingTriggers_RestsOnNoneIsNotADependency(t *testing.T) {
	claim := model.Claim{
		ID: "widget.contract.ungoverned", Facet: "contract", Module: "widget", Status: model.StatusLocked,
		Body: "ungoverned", RestsOn: model.RestsNone("deliberately rests on nothing"),
	}
	store, err := lock.LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("lock.LoadStore: %v", err)
	}
	lock.RefreshBaseline(claim, []model.Claim{claim}, store)

	if _, known := store.Baseline(claim.ID, "none"); known {
		t.Fatalf("rests_on none must create no baseline; store has %v", store.Hashes)
	}
	if drift, _, _ := PendingTriggers(claim, []model.Claim{claim}, store, nil); drift {
		t.Fatalf("rests_on none must never report drift")
	}
}
