// legacy_migration_cli_test.go covers DEFERRED-1: on the first command after
// an upgrade, the legacy lock-store migration must RE-ARM per-dependent hash
// baselines for a project's already-locked claims, so dependency-drift
// detection (DX-AUD-09) is restored immediately — with no manual re-lock and
// no spurious review_pending — rather than staying down until each locked
// claim is hand-relocked. Driven end to end through execCLI, mirroring
// lifecycle_audit_cli_test.go's style.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI_Check_MigratesLegacyStore_ReArmsDriftDetection writes an
// already-upgraded project (two claims locked directly in YAML, main rests_on
// dep) alongside a legacy (pre-versioning) flat-format store, then runs
// "dossierx check". The first check must migrate the store WITHOUT spuriously
// flipping main to review_pending; a dependency edited AFTER that migration
// must then flip main on the next check — the go-forward drift detection the
// legacy drop had left disarmed.
func TestCLI_Check_MigratesLegacyStore_ReArmsDriftDetection(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// An already-upgraded project: two claims locked directly in YAML, exactly
	// as they would look after "dossierx lock" in a prior release.
	depPath := filepath.Join(claimsDir, "dep.yaml")
	dep := "id: widget.contract.dep\nfacet: contract\nmodule: widget\nstatus: locked\n" +
		"body: |\n  dependency claim, v1.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(depPath, []byte(dep), 0o644); err != nil {
		t.Fatalf("write dep: %v", err)
	}
	mainPath := filepath.Join(claimsDir, "main.yaml")
	mainClaim := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: locked\n" +
		"body: |\n  the main body.\n" +
		"rests_on:\n  - widget.contract.dep\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(mainPath, []byte(mainClaim), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	// A legacy (pre-versioning) store: no "version" field, flat map[depID]hash
	// whose value no longer matches dep's current content. LoadStore drops
	// these un-attributable flat hashes; the migration must then re-arm main's
	// baseline from dep's CURRENT content.
	storeFile := filepath.Join(root, ".dossierx-lock-store.json")
	legacy := `{
  "hashes": {
    "widget.contract.dep": "stale-legacy-hash"
  },
  "locked_at": {
    "widget.contract.main": "2020-01-01T00:00:00Z",
    "widget.contract.dep": "2020-01-01T00:00:00Z"
  }
}`
	if err := os.WriteFile(storeFile, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy store: %v", err)
	}

	// THE MIGRATION COMES FIRST, and for this fixture that ordering is the
	// point rather than a formality.
	//
	// A schema-0 store predates BOTH on-load migrations: the per-dependent
	// baseline re-arm and the lock ledger. Adoption stamps the current schema
	// version, and lock.MigrateLegacyStore refuses a store already at schema 1
	// or later — so if the two ran in the other order this project's
	// dependency-drift detection would be disarmed permanently, with no command
	// left that could restore it. migrate.go's migrateLegacyBaselines is what
	// keeps the pair together now that adoption no longer rides along with an
	// ordinary command.
	if _, _, err := execCLI(t, "--config", cfgPath, "migrate", "--adopt"); err != nil {
		t.Fatalf("migrate --adopt (post-upgrade): %v", err)
	}

	// First check after the migration: must NOT spuriously flip main to
	// review_pending (its re-armed baseline equals dep's current content).
	if _, _, err := execCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check (post-upgrade): %v", err)
	}
	if raw, err := os.ReadFile(mainPath); err != nil {
		t.Fatalf("read main after first check: %v", err)
	} else if strings.Contains(string(raw), "review_pending: true") {
		t.Fatalf("expected NO spurious review_pending immediately after legacy migration, got:\n%s", raw)
	}

	// The store must have been re-armed and persisted: stamped current schema
	// version, carrying a nested per-dependent baseline keyed under main.
	//
	// Version 2 is the lock-ledger schema, stamped by the migration above,
	// which also grandfathered both already-locked claims into the ledger.
	// Asserting the ledger's presence here as well is what keeps the two
	// migrations pinned TOGETHER: they no longer share an entry point, and a
	// migrate that adopted without re-arming — or a check that re-armed without
	// being able to adopt — would still pass every assertion above.
	storeRaw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store after migration: %v", err)
	}
	if !strings.Contains(string(storeRaw), `"version": 2`) {
		t.Fatalf("expected the migrated store stamped version 2, got:\n%s", storeRaw)
	}
	if !strings.Contains(string(storeRaw), "widget.contract.main") {
		t.Fatalf("expected the migrated store to carry a per-dependent baseline for main, got:\n%s", storeRaw)
	}
	if !strings.Contains(string(storeRaw), `"grandfathered": true`) {
		t.Fatalf("expected the legacy store's already-locked claims grandfathered into the lock ledger, got:\n%s", storeRaw)
	}

	// Now drift the dependency AFTER migration: the next check must flip main to
	// review_pending — proving go-forward drift detection is armed. Before the
	// fix, main had no baseline at all (the legacy drop discarded it and check
	// never re-armed), so this drift went silently undetected.
	drifted := strings.Replace(dep, "dependency claim, v1.", "dependency claim, v2.", 1)
	if err := os.WriteFile(depPath, []byte(drifted), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	// dep is locked, and the rewrite above stands in for the real approval path
	// (unlock -> edit -> lock), which re-records the approved content. Without
	// the re-record the ledger gate correctly reports the in-place edit as
	// tampering, and this test would be measuring that instead of drift re-arming.
	armLedgerFixture(t, cfgPath)
	if _, _, err := execCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check (post-drift): %v", err)
	}
	if raw, err := os.ReadFile(mainPath); err != nil {
		t.Fatalf("read main after drift check: %v", err)
	} else if !strings.Contains(string(raw), "review_pending: true") {
		t.Fatalf("expected main flagged review_pending by post-migration dependency drift, got:\n%s", raw)
	}
}
