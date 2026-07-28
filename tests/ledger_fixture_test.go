// ledger_fixture_test.go holds the one helper the v0.3.0 lock-ledger gate makes
// every pre-ledger fixture in this suite need.
//
// Before the ledger, "status: locked" in a hand-written YAML file was a complete
// description of a locked claim, and many fixtures here say it that way because
// reaching the state through the CLI would have turned each of them into a
// lifecycle test. The gate now — correctly, and this IS the release — reads a
// locked claim with no approval record as tampering: a status flipped by hand
// walks straight past the lint gate, hub gating and the unresolved-comment gate
// as though all three had passed.
//
// armLedger says out loud what those fixtures always meant: these claims are
// legitimately locked. It is called explicitly at each site rather than folded
// into a shared project writer, so a fixture that is SUPPOSED to trip the gate
// does so by simply not calling it, and no future fixture is blessed by accident.
//
// It is also the right call after a fixture EDITS a locked claim in place to
// stand in for a change that real use makes through unlock -> edit -> lock:
// without it the gate reports the honest simulation as content drift, which is
// the gate working rather than the gate being wrong.
package tests

import (
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// armLedger records a lock-ledger approval for every locked claim in the
// project rooted at root (which must contain project.config.yaml), exactly as
// "dossierx claim lock" would have.
func armLedger(t *testing.T, root string) {
	t.Helper()
	cfg, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("arm ledger: load config under %s: %v", root, err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		// A fixture whose claims do not parse is testing the LOAD failure
		// itself; there is nothing to approve, so leave it untouched.
		return
	}
	path := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(path)
	if err != nil {
		t.Fatalf("arm ledger: load store %s: %v", path, err)
	}
	armed := false
	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		lock.RecordApproval(store, c, lock.Approval{Actor: "fixture", Reason: "fixture approval"})
		armed = true
	}
	if !armed {
		// Nothing locked: leave the project exactly as it was. Creating a store
		// file for a project that has never locked anything would change what
		// the very next command sees on disk, for no benefit.
		return
	}
	if err := store.Save(); err != nil {
		t.Fatalf("arm ledger: save store: %v", err)
	}
}
