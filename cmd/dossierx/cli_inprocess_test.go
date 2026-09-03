// cli_inprocess_test.go exercises the actual command wiring in this
// package — the newXxxCmd() constructors and their RunE closures — by
// building a real *cobra.Command tree via newRootCmd() and executing it
// in-process against a throwaway fixture project, the same way a real
// invocation of the "dossierx" binary would.
//
// This is deliberately narrower than tests/ (see that package's own doc
// comment): tests/ execs the built binary as a subprocess and is the source of
// truth for end-to-end CLI behavior, including exit codes, which cannot be
// asserted in-process (a RunE that called os.Exit would kill this test binary;
// since v0.3.0 none do, but the exit STATUS a returned error maps to is still
// only observable from a real process). What belongs here instead is coverage
// of the command wiring itself — each subcommand's happy path, in-process,
// cheap, no subprocess build step.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// execCLI builds a fresh root command (so no state leaks between calls via
// the newXxxCmd() closures) and runs it in-process with args, returning
// combined stdout/stderr and the error runCLI returned (nil on
// success). Every subcommand here reaches its work through --config, never
// a process chdir, so tests can run in parallel-safe temp dirs.
//
// It goes through runCLI rather than root.Execute() so these tests see exactly
// what main() sees, including the "Error: <msg>" line runCLI now prints in
// place of cobra's (root.SilenceErrors is set unconditionally — see
// output.go's runCLI).
//
// --format text is PREPENDED to every invocation, which is what pins this whole
// suite — check_parity_test.go's ten byte-for-byte goldens above all — to the
// human surface it was written against. v0.3.0 makes JSON the default; the
// prose is now the opt-in, and these fixtures assert the prose. Because
// pflag takes the LAST occurrence of a repeated flag, a test that wants the
// machine surface simply passes "--format", "json" in its own args and wins.
func execCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"--format", formatText}, args...))
	err = runCLI(root)
	return outBuf.String(), errBuf.String(), err
}

// execCLIJSON is execCLI's machine-surface twin: it runs the SAME command tree
// with the v0.3.0 default format and decodes the single envelope the command
// printed to stdout. Tests that assert the contract use this; tests that assert
// the prose use execCLI.
func execCLIJSON(t *testing.T, args ...string) (env cliout.Envelope, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"--format", formatJSON}, args...))
	err = runCLI(root)
	if decodeErr := json.Unmarshal(outBuf.Bytes(), &env); decodeErr != nil {
		t.Fatalf("stdout is not a single JSON envelope (%v):\n%s\nstderr:\n%s", decodeErr, outBuf.String(), errBuf.String())
	}
	return env, errBuf.String(), err
}

// icWriteFixtureProject writes a minimal valid project.config.yaml plus one
// draft claim under root, mirroring tests/cli_test.go's
// writeFixtureProject so the two suites stay easy to cross-reference.
func icWriteFixtureProject(t *testing.T, root, module string) (cfgPath, claimPath string) {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}

	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - " + module + "\nclaims_dir: claims\n"
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	claimPath = filepath.Join(claimsDir, "overview.yaml")
	claim := "id: " + module + ".contract.overview\n" +
		"facet: contract\nmodule: " + module + "\nstatus: draft\nlayout: card\n" +
		"body: |\n  fixture claim for in-process CLI tests.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	return cfgPath, claimPath
}

// ---------------------------------------------------------------------
// check, in both its writing and read-only forms
//
// The four separate lint/catalog/render/check invocations this test used to
// make are one command now: v0.3.0 deleted the three stage verbs, so what is
// asserted here is that the ONE surviving command still produces every artifact
// each of them used to, and that --validate produces none of them.
// ---------------------------------------------------------------------

func TestCLI_CheckWritesArtifactsAndValidateWritesNone(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	// An orientation-note claim (via the reserved "overview" facet, which
	// implies kind: orientation-note without saying so explicitly — see
	// model.Claim.EffectiveKind) so the "check" assertion below can cover the
	// orientation-notes non-blocking report line (computed in internal/check,
	// formatted by formatCheckResult), not just "check: OK" itself.
	overviewClaim := "id: widget.overview.router\n" +
		"facet: overview\nmodule: widget\nstatus: draft\nlayout: banner\n" +
		"body: |\n  fixture orientation-note claim for in-process CLI tests.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "overview-router.yaml"), []byte(overviewClaim), 0o644); err != nil {
		t.Fatalf("write overview claim: %v", err)
	}

	// --validate first, on a clean tree: it must report the lint verdict and
	// leave the two artifacts ABSENT. The fixture claims carry no
	// mirrors/rests_on edges, which trips the warning-severity "orphan" lint —
	// expected and non-fatal (only an error-severity finding fails a run).
	out, _, err := execCLI(t, "--config", cfgPath, "check", "--validate")
	if err != nil {
		t.Fatalf("check --validate: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "0 error(s)") {
		t.Fatalf("expected a warning-only (non-failing) validate run, got: %s", out)
	}
	if !strings.Contains(out, "read-only") {
		t.Fatalf("expected --validate to say it wrote nothing, got: %s", out)
	}
	if strings.Contains(out, "wrote") {
		t.Fatalf("--validate must not report writing anything, got: %s", out)
	}
	for _, artifact := range []string{filepath.Join(root, "build", "catalog", "catalog.json"), filepath.Join(root, "build", "viewer", "index.html")} {
		if _, statErr := os.Stat(artifact); statErr == nil {
			t.Fatalf("check --validate wrote %s; it must be read-only", artifact)
		}
	}

	// Now the writing form: same lint verdict, plus both artifacts and the
	// non-blocking orientation-notes report.
	out, _, err = execCLI(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "check: OK") {
		t.Fatalf("expected check: OK, got: %s", out)
	}
	if !strings.Contains(out, `orientation notes: module "widget": 1 (1 in overview)`) {
		t.Fatalf("expected orientation notes report line, got: %s", out)
	}
	if _, statErr := os.Stat(filepath.Join(root, "build", "catalog", "catalog.json")); statErr != nil {
		t.Fatalf("expected build/catalog/catalog.json to exist after check: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(root, "build", "viewer", "index.html")); statErr != nil {
		t.Fatalf("expected build/viewer/index.html to exist after check: %v", statErr)
	}
}

func TestCLI_LintFailureFailsBothCheckForms(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// A claim resting on a target that does not exist anywhere: a real
	// error-severity lint finding.
	broken := "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\n" +
		"body: |\n  broken fixture.\n" +
		"rests_on:\n  - widget.contract.does-not-exist\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "broken.yaml"), []byte(broken), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	out, _, err := execCLI(t, "--config", cfgPath, "check", "--validate")
	if err == nil {
		t.Fatalf("expected --validate to fail on a dangling rests_on target, got no error (out: %s)", out)
	}
	if !strings.Contains(out, "error(s)") {
		t.Fatalf("expected findings summary in stdout, got: %s", out)
	}
	if strings.Contains(out, "check --validate: OK") {
		t.Fatalf("a failing validate must not print its OK line, got: %s", out)
	}

	// The writing form fails on the same finding, at the same step — the point
	// being that --validate is the SAME gate, not a laxer one.
	if _, _, err := execCLI(t, "--config", cfgPath, "check"); err == nil {
		t.Fatalf("expected check to fail too, since it lints first")
	}
}

// ---------------------------------------------------------------------
// claim show — successor to "deps" (and to "implink status")
// ---------------------------------------------------------------------

func TestCLI_ClaimShowReportsBothEdgeDirections(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	out, _, err := execCLI(t, "--config", cfgPath, "claim", "show", "widget.contract.overview")
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	if !strings.Contains(out, "widget.contract.overview") || !strings.Contains(out, "governed_by") {
		t.Fatalf("expected claim show to describe the claim, got: %s", out)
	}
	if !strings.Contains(out, "incoming mirrors") || !strings.Contains(out, "incoming rests_on") {
		t.Fatalf("expected claim show to report incoming edges, got: %s", out)
	}
	// The two things "deps" never said, and the reason claim show replaced it.
	if !strings.Contains(out, "implemented in") {
		t.Fatalf("expected claim show to report implementation links, got: %s", out)
	}
	if !strings.Contains(out, "next actions") {
		t.Fatalf("expected claim show to report the legal next actions, got: %s", out)
	}
}

// The exit STATUS of "claim show" on an unknown id (2, the not-found family)
// is asserted in tests/, which execs a real process — see this file's package
// doc comment.

// ---------------------------------------------------------------------
// claim list --migrated — successor to "coverage"
//
// "coverage" printed a ratio and nothing else. The replacement prints the same
// ratio AND names the claims in it, which is what a caller actually wanted the
// ratio for.
// ---------------------------------------------------------------------

func TestCLI_ClaimListMigratedReportsTheRatioAndTheClaims(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	migrated := "id: widget.contract.migrated\nfacet: contract\nmodule: widget\nstatus: draft\n" +
		"body: |\n  migrated fixture.\nmigrated_from: docs/tabs/widget.html\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	fresh := "id: widget.contract.fresh\nfacet: contract\nmodule: widget\nstatus: draft\n" +
		"body: |\n  new fixture, never migrated.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "migrated.yaml"), []byte(migrated), 0o644); err != nil {
		t.Fatalf("write migrated claim: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claimsDir, "fresh.yaml"), []byte(fresh), 0o644); err != nil {
		t.Fatalf("write fresh claim: %v", err)
	}

	out, _, err := execCLI(t, "--config", cfgPath, "claim", "list", "--migrated")
	if err != nil {
		t.Fatalf("claim list --migrated: %v", err)
	}
	if !strings.Contains(out, "claim list: 1 of 2 claim(s) (50.0%)") {
		t.Fatalf("expected a 1-of-2 (50.0%%) summary, got: %s", out)
	}
	if !strings.Contains(out, "widget.contract.migrated") {
		t.Fatalf("expected the migrated claim to be NAMED, not just counted, got: %s", out)
	}
	if strings.Contains(out, "widget.contract.fresh") {
		t.Fatalf("--migrated must exclude claims with no migrated_from, got: %s", out)
	}
}

func TestCLI_ClaimListMigratedEmptyClaimsDir(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, _, err := execCLI(t, "--config", cfgPath, "claim", "list", "--migrated")
	if err != nil {
		t.Fatalf("claim list --migrated: %v", err)
	}
	if !strings.Contains(out, "claim list: 0 of 0 claim(s) (0.0%)") {
		t.Fatalf("expected the zero-total case to report 0.0%% without dividing by zero, got: %s", out)
	}
}

// ---------------------------------------------------------------------
// claim list --review-pending — successor to "stale"
// ---------------------------------------------------------------------

func TestCLI_ClaimListReviewPendingWhenNothingIsLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	out, _, err := execCLI(t, "--config", cfgPath, "claim", "list", "--review-pending")
	if err != nil {
		t.Fatalf("claim list --review-pending: %v", err)
	}
	// "stale" had a bespoke "nothing locked" message; the filter reports the
	// same fact in the one shape every claim list uses — an empty result set
	// out of a non-empty project.
	if !strings.Contains(out, "claim list: 0 of 1 claim(s)") {
		t.Fatalf("expected an empty review-pending result over one claim, got: %s", out)
	}
}

// ---------------------------------------------------------------------
// lock / unlock / check / reaudit, end to end: lock two claims, edit the
// dependency so "check" flips the dependent to review_pending, "stale"
// lists it, "reaudit" (reject then confirm) clears it, "unlock" reverts a
// claim back to draft. One flow, since each step needs the prior step's
// on-disk state.
// ---------------------------------------------------------------------

func TestCLI_LockCheckStaleReauditUnlockFlow(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	depPath := filepath.Join(claimsDir, "dep.yaml")
	dep := "id: widget.contract.dep\nfacet: contract\nmodule: widget\nstatus: draft\n" +
		"body: |\n  dependency claim, v1.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(depPath, []byte(dep), 0o644); err != nil {
		t.Fatalf("write dep: %v", err)
	}

	mainPath := filepath.Join(claimsDir, "main.yaml")
	mainClaim := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: draft\n" +
		"body: |\n  main claim resting on the dependency.\n" +
		"rests_on:\n  - widget.contract.dep\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(mainPath, []byte(mainClaim), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	if out, _, err := execCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.dep", "--reason", "test fixture"); err != nil {
		t.Fatalf("lock dep: %v (out: %s)", err, out)
	}
	if out, _, err := execCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); err != nil {
		t.Fatalf("lock main: %v (out: %s)", err, out)
	}

	depOnDisk, err := os.ReadFile(depPath)
	if err != nil {
		t.Fatalf("read dep: %v", err)
	}
	changed := strings.Replace(string(depOnDisk), "dependency claim, v1.", "dependency claim, v2.", 1)
	if err := os.WriteFile(depPath, []byte(changed), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	// dep is LOCKED: rewriting its file in place is what the ledger gate
	// refuses. The real path is unlock -> edit -> lock, which records the new
	// content; re-record it so this test stays about the DEPENDENT's drift.
	armLedgerFixture(t, cfgPath)

	if out, _, err := execCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v (out: %s)", err, out)
	}
	mainAfterCheck, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if !strings.Contains(string(mainAfterCheck), "review_pending: true") {
		t.Fatalf("expected main to be flagged review_pending after check, got:\n%s", mainAfterCheck)
	}

	staleOut, _, err := execCLI(t, "--config", cfgPath, "claim", "list", "--review-pending")
	if err != nil {
		t.Fatalf("stale: %v", err)
	}
	if !strings.Contains(staleOut, "widget.contract.main") {
		t.Fatalf("expected stale to list widget.contract.main, got: %s", staleOut)
	}

	// Reject (no --confirm): propose-only, claim untouched.
	rejectOut, _, err := execCLI(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main")
	if err != nil {
		t.Fatalf("reaudit (propose-only): %v", err)
	}
	if !strings.Contains(rejectOut, "not applied") {
		t.Fatalf("expected propose-only reaudit to say not applied, got: %s", rejectOut)
	}
	mainAfterReject, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if string(mainAfterReject) != string(mainAfterCheck) {
		t.Fatalf("expected main untouched by a propose-only reaudit")
	}

	// Confirm: review_pending clears, claim stays locked, gains an audit note.
	confirmOut, _, err := execCLI(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main", "--confirm", "--reason", "test fixture")
	if err != nil {
		t.Fatalf("reaudit --confirm: %v (out: %s)", err, confirmOut)
	}
	mainAfterConfirm, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if strings.Contains(string(mainAfterConfirm), "review_pending: true") {
		t.Fatalf("expected review_pending cleared after confirmed reaudit, got:\n%s", mainAfterConfirm)
	}
	if !strings.Contains(string(mainAfterConfirm), "status: locked") {
		t.Fatalf("expected main to remain locked, got:\n%s", mainAfterConfirm)
	}
	if !strings.Contains(string(mainAfterConfirm), "audit_notes:") {
		t.Fatalf("expected an audit note appended, got:\n%s", mainAfterConfirm)
	}

	// unlock: main reverts to draft.
	unlockOut, _, err := execCLI(t, "--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "test fixture")
	if err != nil {
		t.Fatalf("unlock: %v (out: %s)", err, unlockOut)
	}
	mainAfterUnlock, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if !strings.Contains(string(mainAfterUnlock), "status: draft") {
		t.Fatalf("expected main to be draft after unlock, got:\n%s", mainAfterUnlock)
	}
}
