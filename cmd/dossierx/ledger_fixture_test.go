// ledger_fixture_test.go holds the one helper the v0.3.0 lock-ledger gate makes
// every pre-ledger fixture need.
//
// Before the ledger, "status: locked" in a hand-written YAML file was a complete
// description of a locked claim, and dozens of fixtures in this suite say it
// that way because reaching the state through the CLI would have turned each of
// them into a lifecycle test. The gate now — correctly, and this is the point of
// the release — reads a locked claim with no approval record as tampering: a
// status flipped by hand walks straight past the lint gate, hub gating and the
// unresolved-comment gate as though all three had passed.
//
// armLedgerFixture says out loud what those fixtures always meant: these claims
// are legitimately locked. It is deliberately explicit at each call site rather
// than folded into a shared project writer, so that a fixture which is SUPPOSED
// to trip the gate does so by simply not calling it, and no future fixture is
// blessed by accident.
package main

import (
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// armLedgerFixture records a lock-ledger approval for every locked claim in the
// project at cfgPath, exactly as "dossierx claim lock" would have.
//
// It is also the right call after a fixture EDITS a locked claim in place to
// simulate a change that real use makes through unlock -> edit -> lock: without
// it the gate reports the honest simulation as content drift, which is the gate
// working, not the gate being wrong.
func armLedgerFixture(t *testing.T, cfgPath string) {
	t.Helper()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("arm ledger: load config %s: %v", cfgPath, err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		// A fixture whose claims do not parse is testing the LOAD failure
		// itself; there is nothing to approve and nothing this helper can
		// usefully say about it. Returning quietly keeps the helper safe to
		// call from a shared project writer.
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
