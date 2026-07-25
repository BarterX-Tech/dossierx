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

	// First check after upgrade: migrates the store, but must NOT spuriously
	// flip main to review_pending (its re-armed baseline equals dep's current
	// content).
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
	storeRaw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store after migration: %v", err)
	}
	if !strings.Contains(string(storeRaw), `"version": 1`) {
		t.Fatalf("expected the migrated store stamped version 1, got:\n%s", storeRaw)
	}
	if !strings.Contains(string(storeRaw), "widget.contract.main") {
		t.Fatalf("expected the migrated store to carry a per-dependent baseline for main, got:\n%s", storeRaw)
	}

	// Now drift the dependency AFTER migration: the next check must flip main to
	// review_pending — proving go-forward drift detection is armed. Before the
	// fix, main had no baseline at all (the legacy drop discarded it and check
	// never re-armed), so this drift went silently undetected.
	drifted := strings.Replace(dep, "dependency claim, v1.", "dependency claim, v2.", 1)
	if err := os.WriteFile(depPath, []byte(drifted), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	if _, _, err := execCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check (post-drift): %v", err)
	}
	if raw, err := os.ReadFile(mainPath); err != nil {
		t.Fatalf("read main after drift check: %v", err)
	} else if !strings.Contains(string(raw), "review_pending: true") {
		t.Fatalf("expected main flagged review_pending by post-migration dependency drift, got:\n%s", raw)
	}
}
