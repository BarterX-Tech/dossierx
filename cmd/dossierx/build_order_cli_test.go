// build_order_cli_test.go exercises "dossierx build-order propose|status|lock"
// in-process (see cli_inprocess_test.go's package doc comment for why
// in-process is the right model here: none of these RunE closures call
// os.Exit directly, unlike newDepsCmd/newReauditCmd, so there is nothing
// forcing a subprocess model for this command group). Covers: the
// completeness-gate refusal, a full propose -> status -> lock happy path
// against a synthetic fully-locked fixture spanning 3 phases, and stale
// detection after mutating a covered claim post-lock.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// boWriteConfig writes a minimal project.config.yaml (facets: [contract],
// modules: [module]) plus an empty claims/ dir under root.
func boWriteConfig(t *testing.T, root, module string) string {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - " + module + "\nclaims_dir: claims\n"
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	return cfgPath
}

// boWriteClaim writes one claim YAML file under root/claims.
func boWriteClaim(t *testing.T, root, filename, id, module, status, buildRole string, restsOn []string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("id: " + id + "\n")
	b.WriteString("facet: contract\nmodule: " + module + "\nstatus: " + status + "\n")
	if buildRole != "" {
		b.WriteString("build_role: " + buildRole + "\n")
	}
	b.WriteString("body: |\n  fixture claim for build-order CLI tests.\n")
	if len(restsOn) > 0 {
		b.WriteString("rests_on:\n")
		for _, r := range restsOn {
			b.WriteString("  - " + r + "\n")
		}
	}
	b.WriteString("governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n")

	path := filepath.Join(root, "claims", filename)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write claim %s: %v", filename, err)
	}
	return path
}

// ---------------------------------------------------------------------
// completeness-gate refusal
// ---------------------------------------------------------------------

func TestCLI_BuildOrderPropose_RefusedWhenNotFullyLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)
	boWriteClaim(t, root, "behavior.yaml", "widget.contract.behavior", "widget", "draft", "behavior", nil)

	out, _, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil {
		t.Fatalf("expected propose to refuse a module that isn't fully locked (out: %s)", out)
	}
	if !strings.Contains(err.Error(), "widget.contract.behavior") {
		t.Fatalf("expected the non-locked claim id named in the error, got: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(root, ".build-order.widget.json")); statErr == nil {
		t.Fatalf("expected no artifact file to be written on a refused propose")
	}
}

func TestCLI_BuildOrderPropose_RequiresModuleFlag(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "propose"); err == nil {
		t.Fatalf("expected propose without --module to fail")
	}
	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "status"); err == nil {
		t.Fatalf("expected status without --module to fail")
	}
	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock"); err == nil {
		t.Fatalf("expected lock without --module to fail")
	}
}

func TestCLI_BuildOrderStatus_NotProposedYet(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)

	out, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "not proposed yet") {
		t.Fatalf("expected a friendly not-proposed-yet message, got: %s", out)
	}
}

func TestCLI_BuildOrderLock_RefusedWithoutPropose(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	if out, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget"); err == nil {
		t.Fatalf("expected lock to refuse when nothing has been proposed (out: %s)", out)
	}
}

// ---------------------------------------------------------------------
// full propose -> status -> lock happy path, spanning 3 phases, plus
// stale detection after mutating a covered claim post-lock.
// ---------------------------------------------------------------------

func TestCLI_BuildOrderFullLifecycle_ProposeStatusLockStale(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	// A small dependency graph spanning 3 phases: schema -> behavior -> api,
	// plus an out-of-scope claim that must be excluded, not placed.
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)
	boWriteClaim(t, root, "behavior.yaml", "widget.contract.behavior", "widget", "locked", "behavior", []string{"widget.contract.schema"})
	boWriteClaim(t, root, "api.yaml", "widget.contract.api", "widget", "locked", "api", []string{"widget.contract.behavior"})
	boWriteClaim(t, root, "future.yaml", "widget.contract.future", "widget", "locked", "out-of-scope", nil)

	// propose
	proposeOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err != nil {
		t.Fatalf("propose: %v (out: %s)", err, proposeOut)
	}
	if !strings.Contains(proposeOut, "schema") || !strings.Contains(proposeOut, "behavior") || !strings.Contains(proposeOut, "api") {
		t.Fatalf("expected propose summary to mention all 3 phases, got: %s", proposeOut)
	}
	if !strings.Contains(proposeOut, "excluded") {
		t.Fatalf("expected propose summary to show the excluded count, got: %s", proposeOut)
	}
	artifactPath := filepath.Join(root, ".build-order.widget.json")
	if _, statErr := os.Stat(artifactPath); statErr != nil {
		t.Fatalf("expected build-order artifact file to exist: %v", statErr)
	}

	// status: proposed, not locked, coverage shows 3 of 4 covered (1 excluded).
	statusOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status: %v (out: %s)", err, statusOut)
	}
	if !strings.Contains(statusOut, "locked:   false") {
		t.Fatalf("expected locked: false before locking, got: %s", statusOut)
	}
	if !strings.Contains(statusOut, "3 of 4 claim(s) covered (1 excluded as out-of-scope)") {
		t.Fatalf("expected coverage to report 3 of 4 covered, 1 excluded, got: %s", statusOut)
	}

	// lock
	lockOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget")
	if err != nil {
		t.Fatalf("lock: %v (out: %s)", err, lockOut)
	}
	if !strings.Contains(lockOut, "locked at") {
		t.Fatalf("expected a locked-at confirmation, got: %s", lockOut)
	}

	// Re-locking immediately (unchanged) is refused: nothing to relock.
	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget"); err == nil {
		t.Fatalf("expected relocking an unchanged, already-locked artifact to be refused")
	}

	// status after lock: locked true, stale false.
	statusAfterLock, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status after lock: %v", err)
	}
	if !strings.Contains(statusAfterLock, "locked:   true") {
		t.Fatalf("expected locked: true after lock, got: %s", statusAfterLock)
	}
	if !strings.Contains(statusAfterLock, "stale:    false") {
		t.Fatalf("expected stale: false immediately after lock, got: %s", statusAfterLock)
	}

	// Mutate the schema claim's body on disk (simulating a post-lock edit)
	// and confirm status now reports staleness naming that claim.
	schemaPath := filepath.Join(root, "claims", "schema.yaml")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema claim: %v", err)
	}
	mutated := strings.Replace(string(raw), "fixture claim for build-order CLI tests.", "fixture claim, mutated after lock.", 1)
	if err := os.WriteFile(schemaPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("rewrite schema claim: %v", err)
	}

	statusAfterMutate, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status after mutate: %v", err)
	}
	if !strings.Contains(statusAfterMutate, "stale:    true") {
		t.Fatalf("expected stale: true after mutating a covered claim, got: %s", statusAfterMutate)
	}
	if !strings.Contains(statusAfterMutate, "widget.contract.schema") {
		t.Fatalf("expected the mutated claim id named in stale output, got: %s", statusAfterMutate)
	}

	// Relock is now allowed (stale) and clears staleness.
	relockOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget")
	if err != nil {
		t.Fatalf("expected relock to succeed once stale, got: %v (out: %s)", err, relockOut)
	}
	statusAfterRelock, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status after relock: %v", err)
	}
	if !strings.Contains(statusAfterRelock, "stale:    false") {
		t.Fatalf("expected stale: false after relock, got: %s", statusAfterRelock)
	}
}
