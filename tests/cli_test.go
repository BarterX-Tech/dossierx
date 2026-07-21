// Package tests holds CLI-integration-style tests that exercise the built
// "dossierx" binary end-to-end via exec.Command, covering the "CLI / runtime"
// edge-case category:
//
//  1. invocation from a subdirectory walks upward for project.config.yaml,
//     like git finds .git
//  2. no config found anywhere up the tree produces a clear error
//     suggesting --config <path>
//  3. a nested project.config.yaml (a sub-tree with its own config) means
//     the nearest one wins, never merged with an ancestor's
//  4. an unwritable .catalog.json target directory produces a clear
//     permissions error, not a silent no-op
//  5. lint failures in a CI-style invocation always exit non-zero, with
//     --json producing machine-readable output
package tests

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// exeSuffix returns ".exe" on Windows and "" everywhere else. A binary
// built via `go build -o <path>` with no extension is still a valid,
// readable executable file on Windows, but os/exec's CreateProcess-backed
// lookup requires the .exe suffix to actually launch it — omitting this
// suffix is what makes exec.Command(binPath, ...) fail with "executable
// file not found in %PATH%" on Windows even though the file exists.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// binPath is set once by TestMain to the path of a "dossierx" binary built from
// this module's cmd/dossierx, so every test in this package execs the real CLI
// rather than "go run" (faster, and avoids "go run" mixing its own output
// into stderr).
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "docs-cli-test-bin-")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "dossierx"+exeSuffix())
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		panic(err)
	}
	build := exec.Command("go", "build", "-o", binPath, "./cmd/dossierx")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		panic("building docs binary for CLI tests: " + err.Error() + "\n" + string(out))
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// run execs the built docs binary with args, in dir, and returns combined
// stdout, stderr, and the process exit code (0 if it exited cleanly).
func run(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("running docs binary: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// writeFixtureProject writes a minimal, valid project.config.yaml plus one
// draft claim under root, using module/facet names unique to each test so
// fixtures never collide with the shared testdata/fixture-basic.
func writeFixtureProject(t *testing.T, root, module string) {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}

	cfg := "schema_version: 1\n" +
		"facets:\n  - contract\nmodules:\n  - " + module + "\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	claim := "id: " + module + ".contract.overview\n" +
		"facet: contract\nmodule: " + module + "\nstatus: draft\nlayout: card\n" +
		"body: |\n  fixture claim for CLI tests.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "overview.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
}

// ---------------------------------------------------------------------
// 1. invocation from a subdirectory walks upward for project.config.yaml
// ---------------------------------------------------------------------

func TestSubdirectoryInvocationWalksUpwardForConfig(t *testing.T) {
	root := t.TempDir()
	writeFixtureProject(t, root, "widgetsub")

	// A deep subdirectory with nothing of its own in it.
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep subdir: %v", err)
	}

	stdout, stderr, code := run(t, deep, "lint")
	if code != 0 {
		t.Fatalf("lint from subdirectory: expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	// A lone fixture claim with no edges trips the (warning-severity)
	// "orphan" lint, which is fine — the point of this test is that the
	// config was found at all (exit 0, no --config error), not that the
	// fixture is edge-complete.
	if !strings.Contains(stdout, "0 error(s)") {
		t.Fatalf("expected zero error-severity findings, got stdout: %s", stdout)
	}

	// Prove this is really walk-up-and-find behavior, not some other path
	// resolution: from a directory with NO ancestor config, the same
	// command must fail to find one.
	isolated := t.TempDir()
	_, stderr2, code2 := run(t, isolated, "lint")
	if code2 == 0 {
		t.Fatalf("lint from a directory with no ancestor config unexpectedly succeeded")
	}
	if !strings.Contains(stderr2, "project.config.yaml") {
		t.Fatalf("expected error mentioning project.config.yaml, got stderr: %s", stderr2)
	}
}

// ---------------------------------------------------------------------
// 2. no config found anywhere up the tree -> clear error suggesting
//    --config <path>
// ---------------------------------------------------------------------

func TestNoConfigFoundSuggestsConfigFlag(t *testing.T) {
	// An isolated temp directory guaranteed to have no project.config.yaml
	// anywhere above it (t.TempDir() lives under the OS temp root, well
	// outside this repo's tree).
	isolated := t.TempDir()

	stdout, stderr, code := run(t, isolated, "lint")
	if code == 0 {
		t.Fatalf("expected non-zero exit with no config anywhere, got 0 (stdout: %s)", stdout)
	}
	if !strings.Contains(stderr, "--config") {
		t.Fatalf("expected error to suggest --config <path>, got stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "project.config.yaml") {
		t.Fatalf("expected error to name project.config.yaml, got stderr: %q", stderr)
	}
}

// ---------------------------------------------------------------------
// 3. nested project.config.yaml -> nearest one wins, not merged with an
//    ancestor's
// ---------------------------------------------------------------------

func TestNestedConfigNearestWins(t *testing.T) {
	outer := t.TempDir()
	writeFixtureProject(t, outer, "outermod")

	// A nested sub-tree with its own, independent project.config.yaml and
	// claim set, using a distinct module name so we can tell which config
	// the CLI actually picked up.
	inner := filepath.Join(outer, "nested")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir inner: %v", err)
	}
	writeFixtureProject(t, inner, "innermod")

	// Run from a directory below the inner config; deps should resolve
	// against the inner claim set only (innermod), never the outer one.
	workDir := filepath.Join(inner, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}

	stdout, stderr, code := run(t, workDir, "deps", "innermod.contract.overview")
	if code != 0 {
		t.Fatalf("deps for inner claim: expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "innermod.contract.overview") {
		t.Fatalf("expected deps output to reference innermod claim, got: %s", stdout)
	}

	// The outer config's claim must NOT be visible: if the two configs were
	// merged (or the outer one won instead), this claim id would resolve.
	_, _, code2 := run(t, workDir, "deps", "outermod.contract.overview")
	if code2 != 2 {
		t.Fatalf("expected deps for outer-only claim id to exit 2 (not found) when nested config is nearest, got %d", code2)
	}
}

// ---------------------------------------------------------------------
// 4. .catalog.json target directory not writable -> clear permissions
//    error, not a silent no-op
// ---------------------------------------------------------------------

func TestCatalogUnwritableTargetDirFailsLoudly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows's FILE_ATTRIBUTE_READONLY on a directory doesn't block creating files inside it, unlike POSIX write-permission bits")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root: permission bits are not enforced, skipping")
	}

	root := t.TempDir()
	writeFixtureProject(t, root, "readonlymod")

	catalogPath := filepath.Join(root, ".catalog.json")

	// Make the config's own directory (where .catalog.json is written)
	// read-only, so the write must fail instead of silently doing nothing.
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("chmod root read-only: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(root, 0o755); err != nil {
			t.Logf("restore permissions: %v", err)
		}
	})

	stdout, stderr, code := run(t, root, "catalog")
	if code == 0 {
		t.Fatalf("expected non-zero exit writing catalog into an unwritable directory, got 0 (stdout: %s)", stdout)
	}
	combined := stdout + stderr
	if !strings.Contains(strings.ToLower(combined), "permission") {
		t.Fatalf("expected a clear permissions error, got stdout: %q stderr: %q", stdout, stderr)
	}

	if _, err := os.Stat(catalogPath); err == nil {
		t.Fatalf(".catalog.json was written despite the unwritable directory (silent no-op or partial write)")
	}
}

// ---------------------------------------------------------------------
// 5. lint failures in a CI-style invocation -> always non-zero exit;
//    --json flag for machine-readable output
// ---------------------------------------------------------------------

func TestLintFailureExitsNonZeroAndSupportsJSON(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - brokenmod\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	// A claim with a dangling rests_on reference: guaranteed error-severity
	// "dangling" lint finding.
	claim := "id: brokenmod.contract.overview\n" +
		"facet: contract\nmodule: brokenmod\nstatus: draft\nlayout: card\n" +
		"body: |\n  broken fixture claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n" +
		"rests_on:\n  - brokenmod.contract.does-not-exist\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "overview.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	stdout, stderr, code := run(t, root, "lint")
	if code == 0 {
		t.Fatalf("expected non-zero exit for a claim with a dangling reference, got 0 (stdout: %s)", stdout)
	}
	if !strings.Contains(stdout, "dangling") {
		t.Fatalf("expected text lint output to mention the dangling finding, got: %s", stdout)
	}
	_ = stderr

	jsonOut, jsonErr, jsonCode := run(t, root, "lint", "--json")
	if jsonCode == 0 {
		t.Fatalf("expected non-zero exit with --json too, got 0")
	}
	var findings []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &findings); err != nil {
		t.Fatalf("lint --json output is not valid JSON: %v\noutput: %s\nstderr: %s", err, jsonOut, jsonErr)
	}
	if len(findings) == 0 {
		t.Fatalf("expected at least one finding in --json output, got none")
	}
	foundDangling := false
	for _, f := range findings {
		if f["LintName"] == "dangling" || f["lint_name"] == "dangling" {
			foundDangling = true
		}
	}
	if !foundDangling {
		t.Fatalf("expected a dangling-lint finding in --json output, got: %v", findings)
	}
}
