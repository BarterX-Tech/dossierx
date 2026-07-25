// cli_uxaudit_test.go covers the in-process (execCLI / direct-helper) half
// of the CLI/UX audit fixes (DX-AUD-17..22): the JSON-array coercion for a
// clean lint run, the version subcommand and --version flag, and the
// unknown-module rejection shared by build-order and implink status. The
// exit-code-sensitive half (lock/unlock/flag not-found -> exit 2, unknown
// module -> non-zero exit) lives in tests/cli_uxaudit_test.go, which execs
// the built binary as a subprocess (see cli_inprocess_test.go's package doc
// comment for why exit codes cannot be asserted in-process).
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------
// DX-AUD-18: a clean lint --json run emits [] (a JSON array), never null.
// ---------------------------------------------------------------------

func TestReportLintFindings_EmptyJSONIsArrayNotNull(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := reportLintFindings(cmd, nil, true); err != nil {
		t.Fatalf("expected no error for zero findings, got: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "[]" {
		t.Fatalf("expected a clean lint --json run to print [], got: %q", got)
	}
}

// ---------------------------------------------------------------------
// DX-AUD-19: the binary can report its version, via both the `version`
// subcommand and cobra's built-in --version flag. Neither needs a project
// config on disk.
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
// DX-AUD-21: `build-order status` / `implink status` reject an unknown
// --module instead of silently reporting an empty state and exiting 0. A
// known-but-unused module still reaches its normal report.
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

func TestCLI_ImplinkStatus_UnknownModuleRejected(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	_, _, err := execCLI(t, "--config", cfgPath, "implink", "status", "--module", "nope")
	if err == nil {
		t.Fatalf("expected implink status to reject an unknown module")
	}
	if !strings.Contains(err.Error(), "unknown module") {
		t.Fatalf("expected an unknown-module error, got: %v", err)
	}

	// A known module with nothing linked must still report normally.
	if out, _, err := execCLI(t, "--config", cfgPath, "implink", "status", "--module", "widget"); err != nil {
		t.Fatalf("known-but-unlinked module should not error: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "nothing linked yet") {
		t.Fatalf("expected a nothing-linked-yet report for a known module, got: %s", out)
	}
}
