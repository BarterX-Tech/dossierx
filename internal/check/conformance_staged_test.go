package check_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/loader"
)

const stagedEmbodiment = "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n"

func stagedConformanceFixture(t *testing.T, observationName string) (cfg *config.Config, observationPath string) {
	t.Helper()
	cfg, _ = project(t, baseConfig+"conformance:\n  observations: "+observationName+"\n", map[string]string{
		"claims/state.yaml": draftClaim("widget.contract.state") + stagedEmbodiment,
		observationName:     `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]}]}`,
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")
	return cfg, filepath.Join(cfg.Dir(), observationName)
}

func TestStatusStagedUsesIndexObservationNotWorktree(t *testing.T) {
	cfg, observationPath := stagedConformanceFixture(t, "observations.json")
	if err := os.WriteFile(observationPath, []byte(`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := check.StatusStaged(sp, cfg).Conformance.Results[0].Checks[0].State; got != conformance.StateMatched {
		t.Fatalf("unstaged worktree observation affected index verdict: %s", got)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := check.Status(claims, cfg).Conformance.Results[0].Checks[0].State; got != conformance.StateMismatch {
		t.Fatalf("worktree verdict = %s", got)
	}
	git(t, cfg.Dir(), "add", "observations.json")
	sp, err = check.Staged(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := check.StatusStaged(sp, cfg).Conformance.Results[0].Checks[0].State; got != conformance.StateMismatch {
		t.Fatalf("staged observation verdict = %s", got)
	}
}

func TestStatusStagedObservationLiteralAndSizeBound(t *testing.T) {
	cfg, observationPath := stagedConformanceFixture(t, "observations[1].json")
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := check.StatusStaged(sp, cfg).Conformance.Results[0].Checks[0].State; got != conformance.StateMatched {
		t.Fatalf("literal bracket path verdict = %s", got)
	}
	if err := os.WriteFile(observationPath, bytes.Repeat([]byte{'x'}, conformance.MaxInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, cfg.Dir(), "add", "--", "observations[1].json")
	sp, err = check.Staged(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result := check.StatusStaged(sp, cfg).Conformance.Results[0]
	if result.Checks[0].State != conformance.StateUncheckable || result.Checks[0].Reason != "observation input is invalid" {
		t.Fatalf("oversized staged result = %+v", result)
	}
}

func TestStatusStagedRejectsNonRegularObservation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix index semantics")
	}
	cfg, observationPath := stagedConformanceFixture(t, "observations.json")
	if err := os.Remove(observationPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.json", observationPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Dir(), "target.json"), []byte(`{"format_version":1,"observations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, cfg.Dir(), "add", "-A")
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result := check.StatusStaged(sp, cfg).Conformance.Results[0]; result.Checks[0].State != conformance.StateUncheckable {
		t.Fatalf("symlink observation result = %+v", result)
	}
}

func TestStatusStagedRejectsObservationPhysicallyInsideSymlinkedBuildDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix permissions")
	}
	cfg, _ := stagedConformanceFixture(t, "evidence/observations.json")
	if err := os.RemoveAll(cfg.BuildDirPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cfg.Dir(), "evidence"), cfg.BuildDirPath()); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(cfg.Dir(), "project.config.yaml")
	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configRaw = bytes.Replace(configRaw, []byte("claims_dir: claims\n"), []byte("claims_dir: claims\nbuild_dir: safe-build\n"), 1)
	if err := os.WriteFile(configPath, configRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	worktreeCfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}

	sp, err := check.Staged(worktreeCfg)
	if err != nil {
		t.Fatal(err)
	}
	result := check.StatusStaged(sp, worktreeCfg).Conformance.Results[0]
	if result.Checks[0].State != conformance.StateUncheckable || result.Checks[0].Reason != "observation input is unavailable" {
		t.Fatalf("physically contained staged observation result = %+v", result)
	}
}
