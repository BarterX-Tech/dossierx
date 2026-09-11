package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/spf13/cobra"
)

func conformanceCLIProject(t *testing.T) (root, cfgPath string) {
	t.Helper()
	root = t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	cfg := "schema_version: 1\nfacets: [contract]\nmodules: [widget]\nclaims_dir: claims\nconformance:\n  observations: observations.json\n"
	claim := "id: widget.contract.state\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: state fixture\ngoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [blocked, ready]\n"
	observation := `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready","paused"]}]}`
	for path, data := range map[string]string{cfgPath: cfg, filepath.Join(root, "claims", "state.yaml"): claim, filepath.Join(root, "observations.json"): observation} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, cfgPath
}

func conformanceCapacityCLIProject(t *testing.T) (root, cfgPath string) {
	t.Helper()
	root = t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.Mkdir(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	cfg := "schema_version: 1\nfacets: [contract]\nmodules: [widget]\nclaims_dir: claims\nconformance:\n  observations: observations.json\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 64; i++ {
		claim := fmt.Sprintf("id: widget.contract.c%03d\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: capacity fixture\ngoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://shared\n      expectation:\n        shape: set\n        value: [expected]\n", i)
		if err := os.WriteFile(filepath.Join(claimsDir, fmt.Sprintf("c%03d.yaml", i)), []byte(claim), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// One shared observation is repeated in every claim projection. Its encoded
	// string bytes alone cross the output limit, while the input stays bounded.
	member := strings.Repeat("x", (1<<20)+(4<<10))
	observation := `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://shared","shape":"set","value":["` + member + `"]}]}`
	if err := os.WriteFile(filepath.Join(root, "observations.json"), []byte(observation), 0o644); err != nil {
		t.Fatal(err)
	}
	stagedGit(t, root, "init", "-q")
	stagedGit(t, root, "add", "--", ".")
	return root, cfgPath
}

func TestCLIConformanceValidateAndWritingCheck(t *testing.T) {
	root, cfgPath := conformanceCLIProject(t)
	before := snapshotTree(t, root)
	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil {
		t.Fatalf("check --validate: %v stderr=%s", err, stderr)
	}
	var validated checkData
	envData(t, env, &validated)
	if !validated.ReadOnly || validated.Conformance == nil || validated.Conformance.Results[0].Checks[0].State != conformance.StateMismatch || validated.ConformancePath != "" {
		t.Fatalf("validate data = %+v", validated)
	}
	after := snapshotTree(t, root)
	if len(after) != len(before) {
		t.Fatalf("validate changed file count: before=%d after=%d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("validate changed %s", path)
		}
	}

	env, stderr, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check: %v stderr=%s", err, stderr)
	}
	var checked checkData
	envData(t, env, &checked)
	if checked.ReadOnly || checked.Conformance == nil || checked.ConformancePath == "" {
		t.Fatalf("check data = %+v", checked)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "conformance", "status.json")); err != nil {
		t.Fatalf("status artifact: %v", err)
	}
}

func TestCLIConformanceWriteFailureReportsItsOwnPhase(t *testing.T) {
	root, cfgPath := conformanceCLIProject(t)
	statusPath := filepath.Join(root, "build", "conformance", "status.json")
	if err := os.MkdirAll(statusPath, 0o755); err != nil {
		t.Fatal(err)
	}
	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.OK {
		t.Fatalf("check unexpectedly succeeded: env=%+v stderr=%s", env, stderr)
	}
	if env.StoppedAt != "conformance" || env.Error == nil || env.Error.Code != "write_failed" {
		t.Fatalf("failure envelope = %+v, want write_failed at conformance", env)
	}
	var data checkData
	envData(t, env, &data)
	if data.CatalogPath != "" || data.ConformancePath != "" || data.ViewerPath != "" {
		t.Fatalf("failure paths = catalog %q conformance %q viewer %q", data.CatalogPath, data.ConformancePath, data.ViewerPath)
	}
}

func TestCLIConformanceCapacityRefusalIsConsistentAndNeverPrintsOK(t *testing.T) {
	root, cfgPath := conformanceCapacityCLIProject(t)
	before := snapshotTree(t, root)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "check", args: []string{"--config", cfgPath, "check"}},
		{name: "validate", args: []string{"--config", cfgPath, "check", "--validate"}},
		{name: "staged", args: []string{"--config", cfgPath, "check", "--staged"}},
	} {
		t.Run(tc.name+"-json", func(t *testing.T) {
			env, stderr, err := execCLIJSON(t, tc.args...)
			if err == nil || env.OK || env.Error == nil {
				t.Fatalf("unexpected success: env=%+v stderr=%s", env, stderr)
			}
			if env.Error.Code != "conformance_capacity_exceeded" || env.StoppedAt != "conformance" {
				t.Fatalf("envelope=%+v", env)
			}
			if !strings.Contains(env.Error.Hint, "no generated artifact was replaced") {
				t.Fatalf("hint=%q", env.Error.Hint)
			}
		})
	}
	out, _, err := execCLI(t, "--config", cfgPath, "check", "--staged")
	if err == nil || strings.Contains(out, "check --staged: OK") || !strings.Contains(out, "[error] conformance") {
		t.Fatalf("staged text=%q err=%v", out, err)
	}
	after := snapshotTree(t, root)
	for path, content := range before {
		if after[path] != content {
			t.Fatalf("capacity refusal changed %s", path)
		}
	}
}

func TestCLIConformanceInvalidThemeStopsAtRenderInEveryMode(t *testing.T) {
	root, cfgPath := conformanceCLIProject(t)
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("viewer:\n  theme:\n    preset: does-not-exist\n")...)
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	stagedGit(t, root, "init", "-q")
	stagedGit(t, root, "add", "--", ".")
	for _, args := range [][]string{
		{"--config", cfgPath, "check"},
		{"--config", cfgPath, "check", "--validate"},
		{"--config", cfgPath, "check", "--staged"},
	} {
		env, stderr, err := execCLIJSON(t, args...)
		if err == nil || env.Error == nil || env.Error.Code != "invalid_config" || env.StoppedAt != "render" {
			t.Fatalf("args=%v env=%+v stderr=%s err=%v", args, env, stderr, err)
		}
	}
}

func TestCLIConformanceCapacityPrecedesInvalidThemeInEveryMode(t *testing.T) {
	root, cfgPath := conformanceCapacityCLIProject(t)
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("viewer:\n  theme:\n    preset: does-not-exist\n")...)
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	stagedGit(t, root, "add", "--", ".")
	for _, args := range [][]string{
		{"--config", cfgPath, "check"},
		{"--config", cfgPath, "check", "--validate"},
		{"--config", cfgPath, "check", "--staged"},
	} {
		env, stderr, err := execCLIJSON(t, args...)
		if err == nil || env.Error == nil || env.Error.Code != "conformance_capacity_exceeded" || env.StoppedAt != "conformance" {
			t.Fatalf("args=%v env=%+v stderr=%s err=%v", args, env, stderr, err)
		}
		var data checkData
		envData(t, env, &data)
		if data.ThemeError != "" || data.FailurePhase != "conformance" {
			t.Fatalf("args=%v reported secondary theme fault: %+v", args, data)
		}
	}
}

func TestCLILintPrecedesConformanceCapacityInEveryMode(t *testing.T) {
	root, cfgPath := conformanceCapacityCLIProject(t)
	claimPath := filepath.Join(root, "claims", "c000.yaml")
	raw, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "governed_by:\n", "rests_on:\n  - widget.contract.missing\ngoverned_by:\n", 1))
	if err := os.WriteFile(claimPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	stagedGit(t, root, "add", "--", ".")

	for _, args := range [][]string{
		{"--config", cfgPath, "check"},
		{"--config", cfgPath, "check", "--validate"},
		{"--config", cfgPath, "check", "--staged"},
	} {
		env, stderr, err := execCLIJSON(t, args...)
		if err == nil || env.Error == nil || env.Error.Code != "lint_failed" || env.StoppedAt != "lint" {
			t.Fatalf("args=%v env=%+v stderr=%s err=%v", args, env, stderr, err)
		}
	}
}

func TestCLIProjectionErrorsKeepRenderDomainInEveryMode(t *testing.T) {
	root, cfgPath := conformanceCLIProject(t)
	if err := os.Mkdir(filepath.Join(root, "overrides"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "overrides", "shell.html"), []byte("{{end}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, append(raw, []byte("viewer:\n  template_overrides: overrides\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	stagedGit(t, root, "init", "-q")
	stagedGit(t, root, "add", "--", ".")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "check", args: []string{"--config", cfgPath, "check"}},
		{name: "validate", args: []string{"--config", cfgPath, "check", "--validate"}},
		{name: "staged", args: []string{"--config", cfgPath, "check", "--staged"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, stderr, err := execCLIJSON(t, tc.args...)
			if err == nil || env.Error == nil || env.Error.Code != "write_failed" || env.StoppedAt != "render" {
				t.Fatalf("env=%+v stderr=%s err=%v", env, stderr, err)
			}
			var data checkData
			envData(t, env, &data)
			if data.RenderError == "" || data.ConformanceError != "" || data.FailurePhase != "render" {
				t.Fatalf("projection domains = %+v", data)
			}
			if !strings.Contains(env.Error.Hint, "viewer projection") || strings.Contains(env.Error.Hint, "conformance input") {
				t.Fatalf("misleading recovery hint = %q", env.Error.Hint)
			}
		})
	}
	out, _, err := execCLI(t, "--config", cfgPath, "check", "--validate")
	if err == nil || !strings.Contains(out, "[error] render:") || strings.Contains(out, "[error] conformance:") {
		t.Fatalf("text=%q err=%v", out, err)
	}
}

func TestPlainCatalogCapacityHasCatalogMessageAndRecovery(t *testing.T) {
	res := check.Result{
		CatalogError:                "conformance capacity exceeded: catalog output is 67108865 bytes; maximum is 67108864",
		ConformanceCapacityExceeded: true,
		ConformanceFailurePhase:     "catalog",
	}
	if got := projectionError(res); got != res.CatalogError {
		t.Fatalf("projectionError=%q", got)
	}
	if got := checkFailureCode(res, "catalog"); got != cliout.CodeConformanceCapacityExceeded {
		t.Fatalf("code=%q", got)
	}
	if hint := projectionRecoveryHint(res); !strings.Contains(hint, "catalog, readiness, or conformance volume") || !strings.Contains(hint, "no generated artifact was replaced") {
		t.Fatalf("hint=%q", hint)
	}
	data := newCheckData(res)
	if data.CatalogError == "" || data.ConformanceError != "" || data.FailurePhase != "catalog" {
		t.Fatalf("machine data=%+v", data)
	}
	var out strings.Builder
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	reportProjectionError(cmd, res)
	if !strings.Contains(out.String(), "[error] catalog:") || strings.Contains(out.String(), "[error] conformance:") {
		t.Fatalf("text=%q", out.String())
	}
}
