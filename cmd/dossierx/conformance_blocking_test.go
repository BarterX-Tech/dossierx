package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/conformance"
)

const blockingCompareClaim = `id: widget.contract.state
facet: contract
module: widget
status: draft
layout: card
body: neutral conformance gate fixture
governed_by:
  type: none
  reason: fixture
embodiment:
  mode: compare
  checks:
    - id: state
      adapter: neutral/v1
      target: widget://state
      expectation:
        shape: set
        value: [ready]
`

func blockingCLIProject(t *testing.T, blocking bool, claim, observations string) (root, cfgPath string) {
	t.Helper()
	policy := ""
	if blocking {
		policy = "true"
	}
	return blockingCLIProjectWithPolicy(t, policy, claim, observations)
}

func blockingCLIProjectWithPolicy(t *testing.T, policy, claim, observations string) (root, cfgPath string) {
	t.Helper()
	root = t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	blockingLine := ""
	if policy != "" {
		blockingLine = "  blocking: " + policy + "\n"
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	cfg := "schema_version: 1\nfacets: [contract]\nmodules: [widget]\nclaims_dir: claims\nconformance:\n  observations: observations.json\n" + blockingLine
	for path, data := range map[string]string{
		cfgPath: cfg,
		filepath.Join(root, "claims", "state.yaml"): claim,
		filepath.Join(root, "observations.json"):    observations,
	} {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, cfgPath
}

func stageBlockingCLIProject(t *testing.T, root string) {
	t.Helper()
	stagedGit(t, root, "init", "-q", "-b", "main")
	stagedGit(t, root, "add", "--", ".")
}

func TestConformanceBlockingOmittedAndExplicitFalseAreReportOnlyForEveryStateAndCheckMode(t *testing.T) {
	noneClaim := strings.Replace(blockingCompareClaim, "mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n", "mode: none\n  reason: neutral documentation-only fixture\n", 1)
	states := []struct {
		name         string
		claim        string
		observations string
	}{
		{name: "matched", claim: blockingCompareClaim, observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]}]}`},
		{name: "owed", claim: blockingCompareClaim, observations: `{"format_version":1,"observations":[]}`},
		{name: "mismatch", claim: blockingCompareClaim, observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`},
		{name: "uncheckable", claim: blockingCompareClaim, observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","error":{"code":"adapter_failed","message":"neutral fixture failure"}}]}`},
		{name: "declared-none", claim: noneClaim, observations: `{"format_version":1,"observations":[]}`},
	}
	for _, policy := range []struct {
		name  string
		value string
	}{
		{name: "omitted", value: ""},
		{name: "explicit-false", value: "false"},
	} {
		for _, state := range states {
			t.Run(policy.name+"/"+state.name, func(t *testing.T) {
				root, cfgPath := blockingCLIProjectWithPolicy(t, policy.value, state.claim, state.observations)
				stageBlockingCLIProject(t, root)
				for _, mode := range []struct {
					name string
					args []string
				}{
					{name: "plain", args: []string{"--config", cfgPath, "check"}},
					{name: "validate", args: []string{"--config", cfgPath, "check", "--validate"}},
					{name: "staged", args: []string{"--config", cfgPath, "check", "--staged"}},
				} {
					env, stderr, err := execCLIJSON(t, mode.args...)
					if err != nil || !env.OK {
						t.Fatalf("%s report-only check failed: env=%+v stderr=%s err=%v", mode.name, env, stderr, err)
					}
					var data checkData
					envData(t, env, &data)
					if data.Conformance == nil || data.ConformanceBlockingEnabled || data.ConformanceBlockingChecks != 0 {
						t.Fatalf("%s report-only data = %+v", mode.name, data)
					}
				}
			})
		}
	}
}

func TestConformanceBlockingStatesRefuseEveryCheckModeAndPreservePlainEvidence(t *testing.T) {
	states := []struct {
		name         string
		observations string
		want         conformance.State
	}{
		{name: "owed", observations: `{"format_version":1,"observations":[]}`, want: conformance.StateOwed},
		{name: "mismatch", observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`, want: conformance.StateMismatch},
		{name: "uncheckable", observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","error":{"code":"adapter_failed","message":"neutral fixture failure"}}]}`, want: conformance.StateUncheckable},
	}
	for _, state := range states {
		for _, mode := range []struct {
			name string
			args func(string) []string
		}{
			{name: "plain", args: func(cfg string) []string { return []string{"--config", cfg, "check"} }},
			{name: "validate", args: func(cfg string) []string { return []string{"--config", cfg, "check", "--validate"} }},
			{name: "staged", args: func(cfg string) []string { return []string{"--config", cfg, "check", "--staged"} }},
		} {
			t.Run(state.name+"/"+mode.name, func(t *testing.T) {
				root, cfgPath := blockingCLIProject(t, true, blockingCompareClaim, state.observations)
				stageBlockingCLIProject(t, root)
				before := snapshotTree(t, root)
				env, stderr, err := execCLIJSON(t, mode.args(cfgPath)...)
				if err == nil || env.OK || env.Error == nil || env.Error.Code != "conformance_failed" || env.StoppedAt != "conformance" {
					t.Fatalf("gate verdict: env=%+v stderr=%s err=%v", env, stderr, err)
				}
				var data checkData
				envData(t, env, &data)
				if data.Conformance == nil || !data.ConformanceBlockingEnabled || data.ConformanceBlockingChecks != 1 || len(data.Conformance.Results) != 1 || len(data.Conformance.Results[0].Checks) != 1 || data.Conformance.Results[0].Checks[0].State != state.want {
					t.Fatalf("gate data = %+v", data)
				}
				if mode.name != "plain" {
					after := snapshotTree(t, root)
					if len(after) != len(before) {
						t.Fatalf("read-only gate changed file count: before=%d after=%d", len(before), len(after))
					}
					for path, content := range before {
						if after[path] != content {
							t.Fatalf("read-only gate changed %s", path)
						}
					}
					return
				}

				statusRaw, statusErr := os.ReadFile(filepath.Join(root, "build", "conformance", "status.json"))
				catalogRaw, catalogErr := os.ReadFile(filepath.Join(root, "build", "catalog", "catalog.json"))
				viewerRaw, viewerErr := os.ReadFile(filepath.Join(root, "build", "viewer", "index.html"))
				if statusErr != nil || catalogErr != nil || viewerErr != nil {
					t.Fatalf("plain gate did not leave agreeing evidence: status=%v catalog=%v viewer=%v", statusErr, catalogErr, viewerErr)
				}
				var report conformance.Report
				if err := json.Unmarshal(statusRaw, &report); err != nil || report.Results[0].Checks[0].State != state.want {
					t.Fatalf("status evidence = %+v err=%v", report, err)
				}
				for name, raw := range map[string][]byte{"catalog": catalogRaw, "viewer": viewerRaw} {
					if !strings.Contains(string(raw), string(state.want)) {
						t.Fatalf("%s does not expose %q", name, state.want)
					}
				}
			})
		}
	}
}

func TestConformanceBlockingTextNamesTheGateAfterWrittenArtifactsAndNeverPrintsOK(t *testing.T) {
	_, cfgPath := blockingCLIProject(t, true, blockingCompareClaim,
		`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`)
	out, stderr, err := execCLI(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatalf("blocking check unexpectedly succeeded: stdout=%s stderr=%s", out, stderr)
	}
	for _, want := range []string{"catalog: wrote ", "conformance: wrote ", "render: wrote ", "[error] conformance: 1 blocking compare check(s) (0 owed, 1 mismatch, 0 uncheckable)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("blocking output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "check: OK") {
		t.Fatalf("blocking output printed success:\n%s", out)
	}
}

func TestConformanceBlockingAllowsMatchedAndDeclaredNone(t *testing.T) {
	for _, tc := range []struct {
		name         string
		claim        string
		observations string
	}{
		{name: "matched", claim: blockingCompareClaim, observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]}]}`},
		{name: "declared-none", claim: strings.Replace(blockingCompareClaim, "mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n", "mode: none\n  reason: neutral documentation-only fixture\n", 1), observations: `{"format_version":1,"observations":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, cfgPath := blockingCLIProject(t, true, tc.claim, tc.observations)
			stageBlockingCLIProject(t, root)
			for _, args := range [][]string{
				{"--config", cfgPath, "check"},
				{"--config", cfgPath, "check", "--validate"},
				{"--config", cfgPath, "check", "--staged"},
			} {
				env, stderr, err := execCLIJSON(t, args...)
				if err != nil || !env.OK {
					t.Fatalf("args=%v env=%+v stderr=%s err=%v", args, env, stderr, err)
				}
				var data checkData
				envData(t, env, &data)
				if !data.ConformanceBlockingEnabled || data.ConformanceBlockingChecks != 0 {
					t.Fatalf("args=%v data=%+v", args, data)
				}
			}
		})
	}
}

func TestConformanceBlockingCountsChecksNotClaims(t *testing.T) {
	claim := strings.Replace(blockingCompareClaim, "        value: [ready]\n", "        value: [ready]\n    - id: policy\n      adapter: neutral/v1\n      target: widget://policy\n      expectation:\n        shape: scalar\n        value: enabled\n", 1)
	root, cfgPath := blockingCLIProject(t, true, claim,
		`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]},{"adapter":"neutral/v1","target":"widget://policy","shape":"scalar","value":"disabled"}]}`)
	stageBlockingCLIProject(t, root)
	for _, args := range [][]string{
		{"--config", cfgPath, "check"},
		{"--config", cfgPath, "check", "--validate"},
		{"--config", cfgPath, "check", "--staged"},
	} {
		env, stderr, err := execCLIJSON(t, args...)
		if err == nil || env.Error == nil || env.Error.Code != "conformance_failed" || env.StoppedAt != "conformance" {
			t.Fatalf("args=%v env=%+v stderr=%s err=%v", args, env, stderr, err)
		}
		var data checkData
		envData(t, env, &data)
		if data.Conformance == nil || data.Conformance.Summary.Checks != 2 || data.Conformance.Summary.Matched != 1 || data.Conformance.Summary.Mismatch != 1 || data.ConformanceBlockingChecks != 1 {
			t.Fatalf("args=%v mixed check counts = %+v", args, data)
		}
	}
}

func TestConformanceBlockingPreservesLintAndThemePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(string) string
		wantCode string
		wantStop string
	}{
		{name: "lint", mutate: func(claim string) string {
			return strings.Replace(claim, "governed_by:\n", "rests_on:\n  - widget.contract.missing\ngoverned_by:\n", 1)
		}, wantCode: "lint_failed", wantStop: "lint"},
		{name: "theme", mutate: func(claim string) string { return claim }, wantCode: "invalid_config", wantStop: "render"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, cfgPath := blockingCLIProject(t, true, tc.mutate(blockingCompareClaim),
				`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`)
			if tc.name == "theme" {
				raw, err := os.ReadFile(cfgPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cfgPath, append(raw, []byte("viewer:\n  theme:\n    preset: does-not-exist\n")...), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			stageBlockingCLIProject(t, root)
			for _, args := range [][]string{
				{"--config", cfgPath, "check"},
				{"--config", cfgPath, "check", "--validate"},
				{"--config", cfgPath, "check", "--staged"},
			} {
				env, stderr, err := execCLIJSON(t, args...)
				if err == nil || env.Error == nil || string(env.Error.Code) != tc.wantCode || env.StoppedAt != tc.wantStop {
					t.Fatalf("args=%v env=%+v stderr=%s err=%v", args, env, stderr, err)
				}
				if strings.Contains(env.Error.Hint, "observations") || strings.Contains(env.Error.Hint, "blocking compare") {
					t.Fatalf("args=%v earlier %s failure received conformance recovery hint %q", args, tc.name, env.Error.Hint)
				}
			}
		})
	}
}

func TestConformanceBlockingDoesNotMaskPriorImplinkScanFailure(t *testing.T) {
	root, cfgPath := blockingCLIProject(t, true, blockingCompareClaim,
		`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`)
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, append(raw, []byte("source_dirs: [src]\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "impl.go"), []byte("package fixture\n\n// dossierx-claim: widget.contract.state\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.Error == nil || env.Error.Code != "implink_refused" || env.StoppedAt != "scan" {
		t.Fatalf("scan precedence: env=%+v stderr=%s err=%v", env, stderr, err)
	}
	if strings.Contains(env.Error.Hint, "observations") || strings.Contains(env.Error.Hint, "blocking compare") {
		t.Fatalf("scan failure received conformance recovery hint %q", env.Error.Hint)
	}
	var data checkData
	envData(t, env, &data)
	if data.Conformance == nil || !data.ConformanceBlockingEnabled || data.ConformanceBlockingChecks != 1 || len(data.ScanErrors) != 1 {
		t.Fatalf("scan result lost secondary conformance evidence: %+v", data)
	}
}

func TestConformanceBlockingDoesNotMaskProjectionFailureInAnyCheckMode(t *testing.T) {
	root, cfgPath := blockingCLIProject(t, true, blockingCompareClaim,
		`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`)
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
	stageBlockingCLIProject(t, root)

	for _, args := range [][]string{
		{"--config", cfgPath, "check"},
		{"--config", cfgPath, "check", "--validate"},
		{"--config", cfgPath, "check", "--staged"},
	} {
		env, stderr, err := execCLIJSON(t, args...)
		if err == nil || env.Error == nil || env.Error.Code != "write_failed" || env.StoppedAt != "render" {
			t.Fatalf("args=%v projection precedence: env=%+v stderr=%s err=%v", args, env, stderr, err)
		}
		if !strings.Contains(env.Error.Hint, "viewer projection") || strings.Contains(env.Error.Hint, "observations") || strings.Contains(env.Error.Hint, "blocking compare") {
			t.Fatalf("args=%v projection recovery hint = %q", args, env.Error.Hint)
		}
		var data checkData
		envData(t, env, &data)
		if data.Conformance == nil || !data.ConformanceBlockingEnabled || data.ConformanceBlockingChecks != 1 || data.RenderError == "" {
			t.Fatalf("args=%v projection result lost secondary conformance evidence: %+v", args, data)
		}
	}
}

func TestConformanceBlockingDoesNotMaskLedgerIntegrityInAnyCheckMode(t *testing.T) {
	for _, mode := range []struct {
		name string
		args func(string) []string
	}{
		{name: "plain", args: func(cfg string) []string { return []string{"--config", cfg, "check"} }},
		{name: "validate", args: func(cfg string) []string { return []string{"--config", cfg, "check", "--validate"} }},
		{name: "staged", args: func(cfg string) []string { return []string{"--config", cfg, "check", "--staged"} }},
	} {
		t.Run(mode.name, func(t *testing.T) {
			root, cfgPath := blockingCLIProject(t, true, blockingCompareClaim,
				`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`)
			lockLifecycle(t, cfgPath, "widget.contract.state")
			claimPath := filepath.Join(root, "claims", "state.yaml")
			tamper(t, claimPath, "neutral conformance gate fixture", "neutral tampered fixture")
			stageBlockingCLIProject(t, root)

			env, stderr, err := execCLIJSON(t, mode.args(cfgPath)...)
			if err == nil || env.OK || env.Error == nil || env.Error.Code != "integrity_failed" || env.StoppedAt != "ledger" {
				t.Fatalf("integrity precedence: env=%+v stderr=%s err=%v", env, stderr, err)
			}
			var data checkData
			envData(t, env, &data)
			if data.Conformance == nil || !data.ConformanceBlockingEnabled || data.ConformanceBlockingChecks != 1 || len(data.LedgerFindings) == 0 {
				t.Fatalf("coexisting conformance/integrity data = %+v", data)
			}
			if env.Error.Hint == "" || strings.Contains(env.Error.Hint, "observations") || strings.Contains(env.Error.Hint, "blocking compare") {
				t.Fatalf("ledger recovery hint lost authority: %q", env.Error.Hint)
			}
		})
	}
}
