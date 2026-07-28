package tests

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckExitCode_Parity pins "dossierx check"'s real process exit code and
// stderr across the Phase-4a extraction (the pipeline moved into
// internal/check.Run, formatted back to the terminal by the command). The
// in-process golden test (cmd/dossierx/check_parity_test.go) covers stdout/
// stderr byte-for-byte; this one execs the built binary so the actual exit
// CODE is asserted, not just "an error was returned":
//
//   - a clean project exits 0;
//   - a lint error exits 1 (the generic-failure family), NOT 2 (the
//     not-found / wrong-state family) — check failures must never be mistaken
//     for a missing claim/config.
func TestCheckExitCode_Parity(t *testing.T) {
	writeClaim := func(t *testing.T, root, name, body string) {
		t.Helper()
		claimsDir := filepath.Join(root, "claims")
		if err := os.MkdirAll(claimsDir, 0o755); err != nil {
			t.Fatalf("mkdir claims: %v", err)
		}
		if err := os.WriteFile(filepath.Join(claimsDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	writeConfig := func(t *testing.T, root string) {
		t.Helper()
		cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"
		if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	t.Run("clean project exits 0", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root)
		writeClaim(t, root, "locked.yaml",
			"id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n"+
				"body: |\n  a locked claim.\n"+
				"governed_by:\n  type: none\n  reason: fixture\n")
		// "Clean" now includes the lock-ledger gate: a hand-written
		// "status: locked" with no approval record is exactly what it refuses,
		// so record the approval this fixture always implied.
		armLedger(t, root)
		stdout, stderr, code := run(t, root, "check")
		if code != 0 {
			t.Fatalf("expected exit 0 on a clean check, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
	})

	t.Run("lint error exits 1 not 2", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root)
		writeClaim(t, root, "broken.yaml",
			"id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
				"body: |\n  broken fixture.\n"+
				"rests_on:\n  - widget.contract.does-not-exist\n"+
				"governed_by:\n  type: none\n  reason: fixture\n")
		stdout, stderr, code := run(t, root, "check")
		if code != 1 {
			t.Fatalf("expected exit 1 on a lint-error check, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if stderr != "Error: check: lint: 1 error-level finding(s)\n" {
			t.Fatalf("check stderr drift: %q", stderr)
		}
		wantStdout := "[error] dangling: widget.contract.broken: rests_on references unknown claim id widget.contract.does-not-exist\n" +
			"lint: 1 finding(s), 1 error(s)\n"
		if stdout != wantStdout {
			t.Fatalf("check stdout drift:\n--- got ----\n%q\n--- want ---\n%q", stdout, wantStdout)
		}
	})
}
