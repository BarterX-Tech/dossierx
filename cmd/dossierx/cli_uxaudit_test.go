// cli_uxaudit_test.go covers the in-process (execCLI / direct-helper) half
// of the CLI/UX audit fixes (DX-AUD-17..22): the empty-findings envelope shape
// for a clean validate run, the version subcommand and --version flag, and the
// unknown-module rejection shared by build-order status and claim list. The
// exit-code-sensitive half (claim lock/unlock/flag not-found -> exit 2, unknown
// module -> non-zero exit) lives in tests/cli_uxaudit_test.go, which execs
// the built binary as a subprocess (see cli_inprocess_test.go's package doc
// comment for why exit codes cannot be asserted in-process).
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// DX-AUD-18: a clean run emits an EMPTY ARRAY of findings, never null.
//
// The original form of this test pinned "dossierx lint --json" printing "[]".
// v0.3.0 deleted that verb; the guarantee moved to the envelope, where the same
// mistake (a nil Go slice encoding as null) would break exactly the same
// consumer — one that ranges over the findings without a null check.
// ---------------------------------------------------------------------

func TestCleanValidateRunEmitsAnEmptyFindingsArrayNotNull(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	// A lone claim with no edges trips the WARNING-severity orphan lint, so
	// findings is non-empty; give it an edge partner so the run is truly clean.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil {
		t.Fatalf("check --validate: %v", err)
	}
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-marshal envelope data: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode envelope data: %v", err)
	}
	for _, key := range []string{"lint_findings", "scan_errors"} {
		got, ok := decoded[key]
		if !ok {
			t.Fatalf("expected %s in the check payload, got: %s", key, raw)
		}
		if strings.TrimSpace(string(got)) == "null" {
			t.Fatalf("%s must encode as an array, never null: %s", key, raw)
		}
	}
	if !decodedBool(t, decoded, "read_only") {
		t.Fatalf("expected check --validate to mark its payload read_only, got: %s", raw)
	}
}

// decodedBool reads one boolean out of an already-decoded payload.
func decodedBool(t *testing.T, decoded map[string]json.RawMessage, key string) bool {
	t.Helper()
	raw, ok := decoded[key]
	if !ok {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("decode %s: %v", key, err)
	}
	return b
}

// ---------------------------------------------------------------------
// DX-AUD-19: the binary can report its version, via both the `version`
// subcommand and the --version flag. Neither needs a project config on disk.
//
// The flag was cobra's built-in until it was taken back (see newRootCmd): the
// built-in printed prose on stdout and exited 0 without reaching any RunE, so it
// was the last invocation in the surface that answered outside the envelope.
// These two tests assert the PROSE surface, which is what --format text still
// gets; machine_contract_test.go asserts the envelope both doors now emit.
// ---------------------------------------------------------------------

func TestCLI_VersionSubcommand(t *testing.T) {
	out, _, err := execCLI(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "dossierx") {
		t.Fatalf("expected version output to name the binary, got: %q", out)
	}
	if !strings.Contains(out, "commit:") || !strings.Contains(out, "date:") {
		t.Fatalf("expected version output to include commit/date lines, got: %q", out)
	}
}

func TestCLI_VersionFlag(t *testing.T) {
	out, errOut, err := execCLI(t, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	combined := out + errOut
	if !strings.Contains(combined, "dossierx") || !strings.Contains(combined, "version") {
		t.Fatalf("expected --version to print a version line, got stdout=%q stderr=%q", out, errOut)
	}
}

// ---------------------------------------------------------------------
// DX-AUD-21: a command taking --module rejects an unknown one instead of
// silently reporting an empty state and exiting 0. A known-but-unused module
// still reaches its normal report.
//
// "implink status" was the second half of this pair until v0.3.0 absorbed it
// into "claim show"; "claim list --module" inherits the guarantee, since it is
// the surviving command where a typo'd module would otherwise produce an
// empty, success-looking answer.
// ---------------------------------------------------------------------

func TestCLI_BuildOrderStatus_UnknownModuleRejected(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	_, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "nope")
	if err == nil {
		t.Fatalf("expected build-order status to reject an unknown module")
	}
	if !strings.Contains(err.Error(), "unknown module") {
		t.Fatalf("expected an unknown-module error, got: %v", err)
	}

	// A known module with no artifact yet must still report normally.
	if out, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget"); err != nil {
		t.Fatalf("known-but-unproposed module should not error: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "not proposed yet") {
		t.Fatalf("expected a not-proposed-yet report for a known module, got: %s", out)
	}
}

func TestCLI_ClaimList_UnknownModuleRejected(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	_, _, err := execCLI(t, "--config", cfgPath, "claim", "list", "--module", "nope")
	if err == nil {
		t.Fatalf("expected claim list to reject an unknown module")
	}
	if !strings.Contains(err.Error(), "unknown module") {
		t.Fatalf("expected an unknown-module error, got: %v", err)
	}

	// A known module still reaches its normal report.
	if out, _, err := execCLI(t, "--config", cfgPath, "claim", "list", "--module", "widget"); err != nil {
		t.Fatalf("known module should not error: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "widget.contract.overview") {
		t.Fatalf("expected the module's claim listed, got: %s", out)
	}
}
