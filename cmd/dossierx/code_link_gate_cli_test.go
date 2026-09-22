// code_link_gate_cli_test.go pins the code-link gate at the CLI surface
// (issue #78): with source_dirs set, a plain `dossierx check` on a locked,
// code-producing claim nobody tagged exits 1 with error.code unlinked_claims
// and stopped_at links, names the claim in data.code_links AND on the
// terminal, and still writes the viewer; `check --validate` on the same
// project stays green but says scanned:false, gated:false with the same
// unlinked claim listed, so a read-only green can never pass for a linked one.
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

type codeLinksEnvelopeData struct {
	ViewerPath string `json:"viewer_path"`
	CodeLinks  *struct {
		Scanned bool `json:"scanned"`
		Gated   bool `json:"gated"`
		Modules []struct {
			Module   string   `json:"module"`
			Linked   int      `json:"linked"`
			Unlinked []string `json:"unlinked"`
			Partial  []struct {
				ClaimID string `json:"claim_id"`
				Covered int    `json:"covered"`
				Total   int    `json:"total"`
				Missing []int  `json:"missing"`
			} `json:"partial"`
		} `json:"modules"`
	} `json:"code_links"`
}

func decodeCodeLinksData(t *testing.T, env cliout.Envelope) codeLinksEnvelopeData {
	t.Helper()
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-marshal data: %v", err)
	}
	var out codeLinksEnvelopeData
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode data: %v\n%s", err, raw)
	}
	return out
}

// codeLinkGateProject writes a project with source_dirs: [src], one locked
// behavior claim and one source file with no tag. The ledger is armed so the
// only gate that can fire is the code-link one.
func codeLinkGateProject(t *testing.T) (cfgPath, srcPath string) {
	t.Helper()
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\nsource_dirs:\n  - src\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeLockedFixtureClaim(t, claimsDir, "widget.contract.main", "widget", "the widget retries twice")
	armLedgerFixture(t, cfgPath)
	srcPath = filepath.Join(root, "src", "widget.go")
	if err := os.WriteFile(srcPath, []byte("package widget\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return cfgPath, srcPath
}

func TestCLI_Check_UnlinkedClaim_RefusesAfterWritingTheViewer(t *testing.T) {
	cfgPath, srcPath := codeLinkGateProject(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.OK || env.Error == nil {
		t.Fatalf("expected check to refuse on the unlinked claim, got err=%v env=%+v", err, env)
	}
	if env.Error.Code != cliout.CodeUnlinkedClaims {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, cliout.CodeUnlinkedClaims)
	}
	if env.StoppedAt != "links" {
		t.Fatalf("stopped_at = %q, want links", env.StoppedAt)
	}
	if cliout.ExitCode(env.Error.Code) != 1 {
		t.Fatalf("unlinked_claims must exit 1 (the check-failure family), got %d", cliout.ExitCode(env.Error.Code))
	}
	if !strings.Contains(env.Error.Hint, "data.code_links") || !strings.Contains(env.Error.Hint, "dossierx-step:") || !strings.Contains(env.Error.Hint, "build_role") {
		t.Fatalf("the hint must point at data.code_links and name both recoveries (tag, or re-role), got %q", env.Error.Hint)
	}
	data := decodeCodeLinksData(t, env)
	if data.ViewerPath == "" {
		t.Fatalf("the viewer must have been written before the refusal, got %+v", data)
	}
	if _, statErr := os.Stat(data.ViewerPath); statErr != nil {
		t.Fatalf("viewer_path %q does not exist: %v", data.ViewerPath, statErr)
	}
	if data.CodeLinks == nil || !data.CodeLinks.Scanned || !data.CodeLinks.Gated {
		t.Fatalf("expected code_links scanned:true gated:true, got %+v", data.CodeLinks)
	}
	if len(data.CodeLinks.Modules) != 1 || len(data.CodeLinks.Modules[0].Unlinked) != 1 || data.CodeLinks.Modules[0].Unlinked[0] != "widget.contract.main" {
		t.Fatalf("expected widget.contract.main listed as unlinked, got %+v", data.CodeLinks.Modules)
	}
	if data.CodeLinks.Modules[0].Partial == nil {
		t.Fatalf("partial must be an array, never null, got %+v", data.CodeLinks.Modules[0])
	}

	// The terminal names the claim too, and never prints "check: OK".
	out, _, err := execCLI(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatalf("expected a text-mode refusal as well")
	}
	if !strings.Contains(out, "[links] widget: widget.contract.main: no code link") || !strings.Contains(out, "code links: 1 claim(s) not linked") {
		t.Fatalf("expected the refusal to name the claim on the terminal, got:\n%s", out)
	}
	if strings.Contains(out, "check: OK") {
		t.Fatalf("a refused check must not print check: OK:\n%s", out)
	}
	// The viewer itself says so, on the claim's card.
	viewer, readErr := os.ReadFile(data.ViewerPath)
	if readErr != nil {
		t.Fatalf("read viewer: %v", readErr)
	}
	if !strings.Contains(string(viewer), "not linked to code") {
		t.Fatalf("the regenerated viewer must show the unlinked row on the claim card")
	}

	// Tag the file: the same check goes green, and the row is gone.
	if err := os.WriteFile(srcPath, []byte("package widget\n\n// dossierx-claim: widget.contract.main\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatalf("tag source: %v", err)
	}
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("expected the tagged project to pass, got err=%v env=%+v", err, env)
	}
	data = decodeCodeLinksData(t, env)
	if data.CodeLinks == nil || !data.CodeLinks.Gated || len(data.CodeLinks.Modules[0].Unlinked) != 0 || data.CodeLinks.Modules[0].Linked != 1 {
		t.Fatalf("expected a gated, complete report after tagging, got %+v", data.CodeLinks)
	}
	viewer, readErr = os.ReadFile(data.ViewerPath)
	if readErr != nil {
		t.Fatalf("read viewer after tagging: %v", readErr)
	}
	if strings.Contains(string(viewer), "not linked to code") {
		t.Fatalf("the unlinked row must disappear once the claim is tagged")
	}
}

func TestCLI_CheckValidate_ReportsUnlinkedWithoutGating(t *testing.T) {
	cfgPath, _ := codeLinkGateProject(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil || !env.OK {
		t.Fatalf("--validate never refuses on links, got err=%v env=%+v", err, env)
	}
	data := decodeCodeLinksData(t, env)
	if data.CodeLinks == nil || data.CodeLinks.Scanned || data.CodeLinks.Gated {
		t.Fatalf("expected code_links scanned:false gated:false on --validate, got %+v", data.CodeLinks)
	}
	if len(data.CodeLinks.Modules) != 1 || len(data.CodeLinks.Modules[0].Unlinked) != 1 {
		t.Fatalf("the unlinked claim must still be named on --validate, got %+v", data.CodeLinks.Modules)
	}
}

func TestCLI_Check_LinksNone_IsExcludedFromTheGate(t *testing.T) {
	cfgPath, _ := codeLinkGateProject(t)
	claimsDir := filepath.Join(filepath.Dir(cfgPath), "claims")
	src := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\nbuild_role: behavior\n" +
		"body: |\n  a refusal with no declaration\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"links:\n  mode: none\n  reason: no implementing declaration\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "widget_contract_main.yaml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	armLedgerFixture(t, cfgPath)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("links.mode none must pass the gate, got err=%v env=%+v", err, env)
	}
	data := decodeCodeLinksData(t, env)
	if data.CodeLinks == nil || !data.CodeLinks.Gated || len(data.CodeLinks.Modules[0].Unlinked) != 0 {
		t.Fatalf("expected gated complete, got %+v", data.CodeLinks)
	}
	viewer, readErr := os.ReadFile(data.ViewerPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(viewer), "implemented in: none") || !strings.Contains(string(viewer), "no implementing declaration") {
		t.Fatalf("card must show links none + reason:\n%s", viewer)
	}
	if strings.Contains(string(viewer), "not linked to code") {
		t.Fatalf("links.none must not show the unlinked pill")
	}
}

func TestCLI_Check_ProcessStep_ViaClaimLinkAndYAML(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\nsource_dirs:\n  - src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "id: widget.contract.walk\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: steps\nbuild_role: verification\n" +
		"body: walkthrough\nsteps:\n  - freeze the subject\n  - run the suite\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"steps_owned_by:\n  1: process\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "walk.yaml"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "widget.go"), []byte("package widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	armLedgerFixture(t, cfgPath)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeUnlinkedClaims {
		t.Fatalf("step 2 still missing: err=%v env=%+v", err, env)
	}

	setEnv, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "link",
		"--module", "widget", "--claim", "widget.contract.walk", "--step", "2", "--process", "operator run record")
	if err != nil || !setEnv.OK {
		t.Fatalf("claim link --process: err=%v env=%+v", err, setEnv)
	}

	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("YAML process + claim link --process should complete the claim, got err=%v env=%+v", err, env)
	}
	viewer, readErr := os.ReadFile(decodeCodeLinksData(t, env).ViewerPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(viewer), "process-owned") || !strings.Contains(string(viewer), "operator run record") {
		t.Fatalf("card must show process-owned steps:\n%s", viewer)
	}
}

func TestCLI_Check_SourceRootsSibling_ScansAndRecordsCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	parent := t.TempDir()
	app := filepath.Join(parent, "app")
	corpus := filepath.Join(parent, "corpus")
	if err := os.MkdirAll(filepath.Join(app, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(corpus, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "src", "widget.go"), []byte("package widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = app
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "fixture@example.invalid")
	run("config", "user.name", "fixture")
	run("add", "-A")
	run("commit", "-qm", "fixture")
	sha := strings.TrimSpace(run("rev-parse", "HEAD"))

	cfgPath := filepath.Join(corpus, "project.config.yaml")
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n" +
		"source_roots:\n  - path: ../app\n    repo: example/app\n    ref: " + sha + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	writeLockedFixtureClaim(t, filepath.Join(corpus, "claims"), "widget.contract.main", "widget", "the widget retries twice")
	armLedgerFixture(t, cfgPath)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeUnlinkedClaims {
		t.Fatalf("source_roots without a tag must gate, err=%v env=%+v", err, env)
	}

	if err := os.WriteFile(filepath.Join(app, "src", "widget.go"), []byte("// dossierx-claim: widget.contract.main\npackage widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("tagged sibling source should link, err=%v env=%+v", err, env)
	}
	raw, err := os.ReadFile(filepath.Join(corpus, "build", "code-links", "widget.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), sha) || !strings.Contains(string(raw), "example/app") || !strings.Contains(string(raw), "../app/src/widget.go") {
		t.Fatalf("code-links artifact must record repo, commit and sibling path:\n%s", raw)
	}
}
