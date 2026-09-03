// cli_uxaudit_test.go exercises the exit-code-sensitive half of the CLI/UX
// audit fixes (DX-AUD-19..21) against the real built binary as a subprocess
// (via this package's run helper), because exit codes are only observable
// through an actual process exit — the in-process half lives in
// cmd/dossierx/cli_uxaudit_test.go:
//
//   - DX-AUD-20: lock/unlock/flag on a nonexistent claim exit 2 (the
//     documented "not found / not in the right state" family), matching
//     deps/reaudit; flag on a not-locked claim exits 2 too.
//   - DX-AUD-21: build-order/implink status on an unknown --module exit
//     non-zero instead of silently succeeding; a known-but-unused module
//     still succeeds.
//   - DX-AUD-19: `version` and `--version` work with no project config and
//     print the binary's version.
package tests

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// DX-AUD-20: lock/unlock/flag map "claim not found" to exit 2, and flag
// maps "not locked" (wrong state) to exit 2, mirroring deps/reaudit.
// ---------------------------------------------------------------------

func TestUnknownClaimIDExitsTwo(t *testing.T) {
	root := t.TempDir()
	writeFixtureProject(t, root, "widget")

	cases := [][]string{
		{"claim", "lock", "widget.contract.ghost", "--reason", "audit fixture"},
		{"claim", "unlock", "widget.contract.ghost", "--reason", "audit fixture"},
		{"claim", "flag", "widget.contract.ghost", "--claim-says", "a", "--now-does", "b", "--reason", "c"},
	}
	for _, args := range cases {
		_, stderr, code := reviewedRun(t, root, args...)
		if code != 2 {
			t.Fatalf("expected exit 2 for %v on an unknown claim, got %d (stderr: %s)", args, code, stderr)
		}
	}
}

func TestFlagNotLockedExitsTwo(t *testing.T) {
	root := t.TempDir()
	// writeFixtureProject writes one *draft* claim.
	writeFixtureProject(t, root, "widget")

	_, stderr, code := run(t, root, "claim", "flag", "widget.contract.overview",
		"--claim-says", "a", "--now-does", "b", "--reason", "c")
	if code != 2 {
		t.Fatalf("expected exit 2 flagging a not-locked (wrong-state) claim, got %d (stderr: %s)", code, stderr)
	}
}

// ---------------------------------------------------------------------
// DX-AUD-21: an unknown --module exits non-zero; a known-but-unused module
// still exits 0 with its normal report.
//
// "implink status" was the second command in this pair until v0.3.0 absorbed
// it into "claim show". "claim list --module" inherits the guarantee: it is the
// surviving command where a typo'd module would otherwise produce an empty,
// success-looking answer.
// ---------------------------------------------------------------------

func TestStatusUnknownModuleExitsNonZero(t *testing.T) {
	root := t.TempDir()
	writeFixtureProject(t, root, "widget")

	cases := [][]string{
		{"build-order", "status", "--module", "nope"},
		{"claim", "list", "--module", "nope"},
	}
	for _, args := range cases {
		_, stderr, code := run(t, root, args...)
		if code == 0 {
			t.Fatalf("expected a non-zero exit for %v with an unknown module, got 0 (stderr: %s)", args, stderr)
		}
		if !strings.Contains(stderr, "unknown module") {
			t.Fatalf("expected an unknown-module error for %v, got stderr: %s", args, stderr)
		}
	}
}

func TestStatusKnownButUnusedModuleExitsZero(t *testing.T) {
	root := t.TempDir()
	writeFixtureProject(t, root, "widget")

	out, stderr, code := run(t, root, "build-order", "status", "--module", "widget")
	if code != 0 {
		t.Fatalf("expected exit 0 for a known-but-unproposed module, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(out, "not proposed yet") {
		t.Fatalf("expected 'not proposed yet' for a known module, got: %s", out)
	}

	out2, stderr2, code2 := run(t, root, "claim", "list", "--module", "widget")
	if code2 != 0 {
		t.Fatalf("expected exit 0 for a known module, got %d (stderr: %s)", code2, stderr2)
	}
	if !strings.Contains(out2, "widget.contract.overview") {
		t.Fatalf("expected the known module's claim listed, got: %s", out2)
	}
}

// ---------------------------------------------------------------------
// DX-AUD-19: version reporting works with no project config on disk.
// ---------------------------------------------------------------------

func TestVersionReportingWorksWithoutConfig(t *testing.T) {
	// An isolated dir with no project.config.yaml anywhere above it.
	isolated := t.TempDir()

	out, stderr, code := run(t, isolated, "version")
	if code != 0 {
		t.Fatalf("expected `version` to exit 0 with no config, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(out, "dossierx") {
		t.Fatalf("expected `version` to name the binary, got: %s", out)
	}

	fout, fstderr, fcode := run(t, isolated, "--version")
	if fcode != 0 {
		t.Fatalf("expected `--version` to exit 0 with no config, got %d (stderr: %s)", fcode, fstderr)
	}
	if !strings.Contains(fout+fstderr, "version") {
		t.Fatalf("expected `--version` to print a version line, got stdout=%q stderr=%q", fout, fstderr)
	}
}
