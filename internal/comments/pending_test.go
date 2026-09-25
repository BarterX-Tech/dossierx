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
	stale := false
	for _, c := range lock.DetectStale([]model.Claim{hub, child}, store) {
		if c.ID == child.ID && c.ReviewPending {
			stale = true
		}
	}
	if !stale {
		t.Fatalf("comments.PendingTriggers reported drift but lock.DetectStale did not — the two must never diverge")
	}
}
