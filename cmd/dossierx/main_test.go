// Unit tests for cmd/dossierx's own wiring — as opposed to tests/cli_test.go,
// which execs the built binary as a subprocess (and so never counts toward
// this package's own "go test ./... -cover" number), these tests call the
// package's unexported helpers directly: config discovery (resolveConfigPath),
// the small pure helpers (containsStr, pickChangedDependency), lint-finding
// reporting (reportLintFindings), and the store/catalog/render path helpers.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ---------------------------------------------------------------------
// resolveConfigPath
// ---------------------------------------------------------------------

func TestResolveConfigPathExplicitFlagWins(t *testing.T) {
	old := configPath
	defer func() { configPath = old }()

	configPath = "/some/explicit/path.yaml"
	got, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	if got != configPath {
		t.Fatalf("expected explicit --config value %q, got %q", configPath, got)
	}
}

func TestResolveConfigPathWalksUpward(t *testing.T) {
	old := configPath
	configPath = ""
	defer func() { configPath = old }()

	root := t.TempDir()
	cfgFile := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgFile, []byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("write fixture config: %v", err)
	}

	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWD); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(deep); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	// Resolve symlinks on both sides: on macOS, t.TempDir() lives under
	// /var, which is itself a symlink to /private/var, so a naive
	// filepath.Abs comparison spuriously fails even when resolveConfigPath
	// found the right file.
	gotAbs, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(got): %v", err)
	}
	wantAbs, err := filepath.EvalSymlinks(cfgFile)
	if err != nil {
		t.Fatalf("EvalSymlinks(want): %v", err)
	}
	if gotAbs != wantAbs {
		t.Fatalf("expected walk-upward to find %q, got %q", wantAbs, gotAbs)
	}
}

func TestResolveConfigPathNotFound(t *testing.T) {
	old := configPath
	configPath = ""
	defer func() { configPath = old }()

	isolated := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWD); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	}()
	if err := os.Chdir(isolated); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, err = resolveConfigPath()
	if err == nil {
		t.Fatalf("expected an error when no project.config.yaml exists anywhere upward")
	}
	if !strings.Contains(err.Error(), "--config") {
		t.Fatalf("expected error to suggest --config, got: %v", err)
	}
}

// ---------------------------------------------------------------------
// containsStr
// ---------------------------------------------------------------------

func TestContainsStr(t *testing.T) {
	ss := []string{"a", "b", "c"}
	if !containsStr(ss, "b") {
		t.Fatalf("expected containsStr to find %q in %v", "b", ss)
	}
	if containsStr(ss, "z") {
		t.Fatalf("expected containsStr to not find %q in %v", "z", ss)
	}
	if containsStr(nil, "a") {
		t.Fatalf("expected containsStr(nil, ...) to be false")
	}
}

// ---------------------------------------------------------------------
// pickChangedDependency
// ---------------------------------------------------------------------

func TestPickChangedDependencyPrefersStaleHash(t *testing.T) {
	fresh := model.Claim{ID: "m.contract.fresh", Body: "fresh"}
	stale := model.Claim{ID: "m.contract.stale", Body: "stale, changed since lock"}
	claim := model.Claim{ID: "m.contract.main", RestsOn: []string{"m.contract.fresh", "m.contract.stale"}}
	claims := []model.Claim{claim, fresh, stale}

	store := &lock.Store{Hashes: map[string]map[string]string{
		claim.ID: {
			"m.contract.fresh": lock.ContentHash(fresh),
			"m.contract.stale": "stale-hash-that-no-longer-matches",
		},
	}}

	got := pickChangedDependency(claim, claims, store)
	if got.ID != stale.ID {
		t.Fatalf("expected the claim with the mismatched stored hash (%q), got %q", stale.ID, got.ID)
	}
}

func TestPickChangedDependencyFallsBackToFirstDep(t *testing.T) {
	dep := model.Claim{ID: "m.contract.dep"}
	claim := model.Claim{ID: "m.contract.main", Mirrors: []string{"m.contract.dep"}}
	claims := []model.Claim{claim, dep}

	store := &lock.Store{Hashes: map[string]map[string]string{}}

	got := pickChangedDependency(claim, claims, store)
	if got.ID != dep.ID {
		t.Fatalf("expected fallback to first declared dependency %q, got %q", dep.ID, got.ID)
	}
}

func TestPickChangedDependencyNoDepsReturnsZeroValue(t *testing.T) {
	claim := model.Claim{ID: "m.contract.lonely"}
	store := &lock.Store{Hashes: map[string]map[string]string{}}

	got := pickChangedDependency(claim, []model.Claim{claim}, store)
	if got.ID != "" {
		t.Fatalf("expected zero-value Claim for a claim with no dependencies, got %+v", got)
	}
}

// ---------------------------------------------------------------------
// reportLintFindings
// ---------------------------------------------------------------------

func TestReportLintFindingsEmptyIsClean(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := reportLintFindings(cmd, nil, false); err != nil {
		t.Fatalf("expected no error for zero findings, got: %v", err)
	}
	if !strings.Contains(buf.String(), "0 findings") {
		t.Fatalf("expected 0-findings message, got: %q", buf.String())
	}
}

func TestReportLintFindingsWarningOnlyDoesNotFail(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	findings := []lint.Finding{
		{LintName: "orphan", ClaimID: "m.contract.x", Message: "no edges", Severity: lint.SeverityWarning},
	}
	if err := reportLintFindings(cmd, findings, false); err != nil {
		t.Fatalf("expected warning-only findings to not fail the command, got: %v", err)
	}
	if !strings.Contains(buf.String(), "orphan") {
		t.Fatalf("expected finding text in output, got: %q", buf.String())
	}
}

func TestReportLintFindingsErrorSeverityFails(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	findings := []lint.Finding{
		{LintName: "dangling", ClaimID: "m.contract.x", Message: "missing target", Severity: lint.SeverityError},
	}
	err := reportLintFindings(cmd, findings, false)
	if err == nil {
		t.Fatalf("expected an error-severity finding to fail the command")
	}
	if !strings.Contains(buf.String(), "1 error(s)") {
		t.Fatalf("expected error count in output, got: %q", buf.String())
	}
}

func TestReportLintFindingsJSON(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	findings := []lint.Finding{
		{LintName: "dangling", ClaimID: "m.contract.x", Message: "missing target", Severity: lint.SeverityError},
	}
	err := reportLintFindings(cmd, findings, true)
	if err == nil {
		t.Fatalf("expected an error-severity finding to fail the command even with --json")
	}
	var decoded []lint.Finding
	if jsonErr := json.Unmarshal(buf.Bytes(), &decoded); jsonErr != nil {
		t.Fatalf("expected valid JSON output, got %v: %q", jsonErr, buf.String())
	}
	if len(decoded) != 1 || decoded[0].LintName != "dangling" {
		t.Fatalf("expected decoded JSON to round-trip the finding, got: %+v", decoded)
	}
}

// ---------------------------------------------------------------------
// storePath / catalogPath / renderOutPath: resolved against cfg.Dir(),
// never the process cwd (same convention as claims_dir).
// ---------------------------------------------------------------------

func TestPathHelpersResolveAgainstConfigDir(t *testing.T) {
	root := t.TempDir()
	cfgFile := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgFile, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - m\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write fixture config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}

	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if got, want := storePath(cfg), filepath.Join(root, ".dossierx-lock-store.json"); got != want {
		t.Fatalf("storePath: got %q, want %q", got, want)
	}
	if got, want := catalogPath(cfg), filepath.Join(root, ".catalog.json"); got != want {
		t.Fatalf("catalogPath: got %q, want %q", got, want)
	}
	if got, want := renderOutPath(cfg), filepath.Join(root, "viewer", "index.html"); got != want {
		t.Fatalf("renderOutPath: got %q, want %q", got, want)
	}
}
