// This file covers the "Config" edge-case category end-to-end at the CLI
// level (package-level cases for the same rules live in
// internal/config/config_test.go). It reuses the binPath/run/TestMain
// scaffolding from cli_test.go in this same package.
//
// Row -> test mapping (see table in the task report):
//  1. missing config file entirely           -> TestExplicitConfigMissingExitsTwoNamingPath
//  6. claims_dir relative path resolution    -> covered at unit level (config_test.go);
//     TestClaimsDirResolvedAgainstConfigDirAtCLILevel proves it end-to-end too.
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// Row 1: config file missing entirely -> CLI exits 2 naming the exact
// path it looked for.
// ---------------------------------------------------------------------

func TestExplicitConfigMissingExitsTwoNamingPath(t *testing.T) {
	isolated := t.TempDir()
	missing := filepath.Join(isolated, "does-not-exist.config.yaml")

	stdout, stderr, code := run(t, isolated, "--config", missing, "lint")

	if code != 2 {
		t.Fatalf("expected exit code 2 for missing --config file, got %d (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	if !strings.Contains(stderr, missing) {
		t.Fatalf("expected stderr to name the exact missing path %q, got: %q", missing, stderr)
	}
}

func TestDefaultSearchNoConfigExitsTwo(t *testing.T) {
	// No --config passed and no project.config.yaml anywhere above this
	// isolated temp dir: upward search exhausts and the CLI must still
	// use the "missing config" exit code (2), not the generic failure
	// code (1) used for e.g. lint/render failures.
	isolated := t.TempDir()

	stdout, stderr, code := run(t, isolated, "lint")
	if code != 2 {
		t.Fatalf("expected exit code 2 when no project.config.yaml is found anywhere, got %d (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
}

// ---------------------------------------------------------------------
// Row 6 (CLI-level confirmation): claims_dir given as a relative path is
// resolved against the config file's own directory, never the process's
// cwd — proven here by invoking the CLI from a *different* cwd than the
// config file's directory and confirming claims from claims_dir still
// load.
// ---------------------------------------------------------------------

func TestClaimsDirResolvedAgainstConfigDirAtCLILevel(t *testing.T) {
	projectDir := t.TempDir()
	claimsDir := filepath.Join(projectDir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgBody := "schema_version: 1\n" +
		"facets:\n  - contract\n" +
		"modules:\n  - relmod\n" +
		"claims_dir: claims\n" // relative, deliberately
	cfgPath := filepath.Join(projectDir, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	claim := "id: relmod.contract.only-claim\n" +
		"facet: contract\n" +
		"module: relmod\n" +
		"status: draft\n" +
		"governed_by:\n  type: none\n  reason: test fixture\n" +
		"body: exercises relative claims_dir resolution\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "only.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run from an entirely unrelated cwd, passing --config explicitly, so
	// if claims_dir were (wrongly) resolved against cwd instead of the
	// config file's directory, "deps" would fail to find the claim.
	elsewhere := t.TempDir()
	stdout, stderr, code := run(t, elsewhere, "--config", cfgPath, "deps", "relmod.contract.only-claim")
	if code != 0 {
		t.Fatalf("expected deps to find the claim via config-relative claims_dir, got exit %d (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "relmod.contract.only-claim") {
		t.Fatalf("expected deps output to mention the claim id, got: %q", stdout)
	}
}
