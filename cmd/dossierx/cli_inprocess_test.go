// cli_inprocess_test.go exercises the actual command wiring in this
// package — the newXxxCmd() constructors and their RunE closures — by
// building a real *cobra.Command tree via newRootCmd() and executing it
// in-process against a throwaway fixture project, the same way a real
// invocation of the "dossierx" binary would.
//
// This is deliberately narrower than tests/ (see that package's own doc
// comment): tests/ execs the built binary as a subprocess and is the
// source of truth for end-to-end CLI behavior, including exit codes,
// because two RunE branches here (newDepsCmd and newReauditCmd's
// "not found" / "not locked+review_pending" cases) call os.Exit(2)
// directly rather than returning an error — calling those in-process would
// kill this test binary, so only tests/'s subprocess model can exercise
// them. What belongs here instead is coverage of the command wiring itself
// (each subcommand's happy path, in-process, cheap, no subprocess build
// step) — including "dossierx coverage", which tests/ does not exercise at
// all.
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// execCLI builds a fresh root command (so no state leaks between calls via
// the newXxxCmd() closures) and runs it in-process with args, returning
// combined stdout/stderr and the error Execute() returned (nil on
// success). Every subcommand here reaches its work through --config, never
// a process chdir, so tests can run in parallel-safe temp dirs.
func execCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
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
// lint / catalog / render / check
// ---------------------------------------------------------------------

func TestCLI_LintCatalogRenderCheck(t *testing.T) {
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

	// The single fixture claim carries no mirrors/rests_on edges, which
	// trips the warning-severity "orphan" lint — expected and non-fatal
	// (lint only fails the command on an error-severity finding).
	if out, _, err := execCLI(t, "--config", cfgPath, "lint"); err != nil {
		t.Fatalf("lint: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "0 error(s)") {
		t.Fatalf("expected a warning-only (non-failing) lint run, got: %s", out)
	}

	if out, _, err := execCLI(t, "--config", cfgPath, "lint", "--json"); err != nil {
		t.Fatalf("lint --json: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "\"orphan\"") {
		t.Fatalf("expected the orphan warning finding in JSON output, got: %s", out)
	}

	if out, _, err := execCLI(t, "--config", cfgPath, "catalog"); err != nil {
		t.Fatalf("catalog: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "wrote") {
		t.Fatalf("expected catalog to report where it wrote, got: %s", out)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".catalog.json")); statErr != nil {
		t.Fatalf("expected .catalog.json to exist: %v", statErr)
	}

	if out, _, err := execCLI(t, "--config", cfgPath, "render"); err != nil {
		t.Fatalf("render: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "wrote") {
		t.Fatalf("expected render to report where it wrote, got: %s", out)
	}
	if _, statErr := os.Stat(filepath.Join(root, "viewer", "index.html")); statErr != nil {
		t.Fatalf("expected viewer/index.html to exist: %v", statErr)
	}

	if out, _, err := execCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v (out: %s)", err, out)
	} else if !strings.Contains(out, "check: OK") {
		t.Fatalf("expected check: OK, got: %s", out)
	} else if !strings.Contains(out, `orientation notes: module "widget": 1 (1 in overview)`) {
		t.Fatalf("expected orientation notes report line, got: %s", out)
	}
}

func TestCLI_LintFailurePropagatesAsError(t *testing.T) {
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

	out, _, err := execCLI(t, "--config", cfgPath, "lint")
	if err == nil {
		t.Fatalf("expected lint to fail on a dangling rests_on target, got no error (out: %s)", out)
	}
	if !strings.Contains(out, "error(s)") {
		t.Fatalf("expected findings summary in stdout, got: %s", out)
	}

	if _, _, err := execCLI(t, "--config", cfgPath, "check"); err == nil {
		t.Fatalf("expected check to fail too, since it lints first")
	}
}

// ---------------------------------------------------------------------
// deps
// ---------------------------------------------------------------------

func TestCLI_DepsFoundClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	out, _, err := execCLI(t, "--config", cfgPath, "deps", "widget.contract.overview")
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if !strings.Contains(out, "widget.contract.overview") || !strings.Contains(out, "governed_by") {
		t.Fatalf("expected deps output to describe the claim, got: %s", out)
	}
	if !strings.Contains(out, "incoming mirrors") || !strings.Contains(out, "incoming rests_on") {
		t.Fatalf("expected deps to report incoming edges, got: %s", out)
	}
}

// Deps on an unknown id, and reaudit on a not-pending claim, both call
// os.Exit(2) directly (see this file's package doc comment) — that path
// is intentionally left to tests/'s subprocess-based CLI tests
// (tests/cli_test.go's TestNestedConfigNearestWins and
// tests/lock_lifecycle_test.go's TestLockLifecycle_ReauditRefusedWhenNotPending),
// never exercised in-process here.

// ---------------------------------------------------------------------
// coverage
// ---------------------------------------------------------------------

func TestCLI_CoverageCommand(t *testing.T) {
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

	out, _, err := execCLI(t, "--config", cfgPath, "coverage")
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if !strings.Contains(out, "1/2 claim(s) carry migrated_from (50.0%)") {
		t.Fatalf("expected a 1/2 (50.0%%) coverage report, got: %s", out)
	}
}

func TestCLI_CoverageCommandEmptyClaimsDir(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	out, _, err := execCLI(t, "--config", cfgPath, "coverage")
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if !strings.Contains(out, "0/0 claim(s) carry migrated_from (0.0%)") {
		t.Fatalf("expected the zero-total case to report 0.0%% without dividing by zero, got: %s", out)
	}
}

// ---------------------------------------------------------------------
// stale
// ---------------------------------------------------------------------

func TestCLI_StaleNothingLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	out, _, err := execCLI(t, "--config", cfgPath, "stale")
	if err != nil {
		t.Fatalf("stale: %v", err)
	}
	if !strings.Contains(out, "nothing locked") {
		t.Fatalf("expected 'nothing locked', got: %s", out)
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

	if out, _, err := execCLI(t, "--config", cfgPath, "lock", "widget.contract.dep"); err != nil {
		t.Fatalf("lock dep: %v (out: %s)", err, out)
	}
	if out, _, err := execCLI(t, "--config", cfgPath, "lock", "widget.contract.main"); err != nil {
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

	staleOut, _, err := execCLI(t, "--config", cfgPath, "stale")
	if err != nil {
		t.Fatalf("stale: %v", err)
	}
	if !strings.Contains(staleOut, "widget.contract.main") {
		t.Fatalf("expected stale to list widget.contract.main, got: %s", staleOut)
	}

	// Reject (no --confirm): propose-only, claim untouched.
	rejectOut, _, err := execCLI(t, "--config", cfgPath, "reaudit", "widget.contract.main")
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
	confirmOut, _, err := execCLI(t, "--config", cfgPath, "reaudit", "widget.contract.main", "--confirm")
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
	unlockOut, _, err := execCLI(t, "--config", cfgPath, "unlock", "widget.contract.main")
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
