package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// The brief noun's payloads and rule set are registered from here rather than
// named in surface_test.go, which is copied into older trees that have neither
// (see surfacePayloadTypes).
func init() {
	registerSurfacePayloadType("briefListData", briefListData{})
	registerSurfacePayloadType("briefShowData", briefShowData{})
	registerSurfaceBriefRules(briefs.RuleNames())
}

// writeBriefFile writes one file under root/briefs.
func writeBriefFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, "briefs", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// decodeData re-reads an envelope's data into a typed payload.
func decodeData(t *testing.T, env cliout.Envelope, into any) {
	t.Helper()
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("data does not decode as %T: %v\n%s", into, err, raw)
	}
}

const widgetFlowBrief = "---\nsummary: How the widget flow reads end to end.\nstatus: locked\nrests_on:\n  - widget.contract.overview\n---\n# Widget flow\n\nOne paragraph.\n"

// TestBriefListAndShow is the noun's envelope contract at the CLI boundary:
// list names every brief with its path, summary, status and (empty) review
// state; --review-pending lists none today while total still counts every
// brief; show answers by path or by id with the content and its digest; and an
// argument that names no brief is brief_not_found at exit 2.
func TestBriefListAndShow(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	writeBriefFile(t, root, "widget/flow.md", widgetFlowBrief)
	writeBriefFile(t, root, "widget/order-plan.md", "---\nsummary: The order plan.\n---\nNo heading.\n")

	env, _, err := execCLIJSON(t, "--config", cfgPath, "brief", "list")
	if err != nil || !env.OK {
		t.Fatalf("brief list: %v %+v", err, env)
	}
	var list briefListData
	decodeData(t, env, &list)
	if list.Count != 2 || list.Total != 2 || len(list.Briefs) != 2 {
		t.Fatalf("brief list = %+v", list)
	}
	flow := list.Briefs[0]
	if flow.Path != "briefs/widget/flow.md" || flow.ID != "widget.flow" || flow.Title != "Widget flow" ||
		flow.Summary != "How the widget flow reads end to end." || flow.Status != "locked" || flow.ReviewPending {
		t.Fatalf("unexpected entry: %+v", flow)
	}
	if list.Briefs[1].Title != "Order Plan" || list.Briefs[1].Status != "draft" {
		t.Fatalf("unexpected fallback title or default status: %+v", list.Briefs[1])
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "brief", "list", "--review-pending")
	if err != nil || !env.OK {
		t.Fatalf("brief list --review-pending: %v %+v", err, env)
	}
	decodeData(t, env, &list)
	if list.Count != 0 || list.Total != 2 || !list.ReviewPendingOnly || len(list.Briefs) != 0 {
		t.Fatalf("nothing can be review-pending before briefs have a lock store; got %+v", list)
	}

	var byPath, byID briefShowData
	env, _, err = execCLIJSON(t, "--config", cfgPath, "brief", "show", "briefs/widget/flow.md")
	if err != nil || !env.OK {
		t.Fatalf("brief show by path: %v %+v", err, env)
	}
	decodeData(t, env, &byPath)
	env, _, err = execCLIJSON(t, "--config", cfgPath, "brief", "show", "widget.flow")
	if err != nil || !env.OK {
		t.Fatalf("brief show by id: %v %+v", err, env)
	}
	decodeData(t, env, &byID)
	if byPath.Content != widgetFlowBrief || byPath.Status != "locked" || len(byPath.Digest) != 64 ||
		strings.Join(byPath.RestsOn, ",") != "widget.contract.overview" {
		t.Fatalf("unexpected show payload: %+v", byPath)
	}
	if byID.Digest != byPath.Digest || byID.Path != byPath.Path {
		t.Fatal("show by id and by path must answer with the same brief")
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "brief", "show", "briefs/widget/nope.md")
	if env.OK || env.Error == nil || env.Error.Code != cliout.CodeBriefNotFound {
		t.Fatalf("expected brief_not_found, got %+v", env)
	}
	if got := exitStatusFor(err); got != 2 {
		t.Fatalf("brief_not_found must exit 2, got %d", got)
	}
}

// TestBriefListAndShowReportWhatTheyCouldNotRead pins the envelope when the
// tree cannot be fully read: the command still answers (ok, as manifest show
// answers with its findings), every readable brief is listed, and the
// unreadable entry is in data.findings — so neither leaf says "this project
// holds no briefs" about a tree it could not look into. `brief show` of a brief
// behind the unreadable entry is brief_not_found carrying the findings.
func TestBriefListAndShowReportWhatTheyCouldNotRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced: an unreadable directory is still readable on Windows and to root")
	}
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	writeBriefFile(t, root, "widget/flow.md", widgetFlowBrief)
	writeBriefFile(t, root, "secret/plan.md", "---\nsummary: Behind a locked door.\n---\nText.\n")
	secret := filepath.Join(root, "briefs", "secret")
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(secret, 0o755) }) //nolint:errcheck // best-effort restore for TempDir cleanup

	env, _, err := execCLIJSON(t, "--config", cfgPath, "brief", "list")
	if err != nil || !env.OK {
		t.Fatalf("brief list: %v %+v", err, env)
	}
	var list briefListData
	decodeData(t, env, &list)
	if list.Total != 1 || list.Briefs[0].Path != "briefs/widget/flow.md" {
		t.Fatalf("the readable folder's brief must still be listed, got %+v", list)
	}
	if len(list.Findings) != 1 || list.Findings[0].Lint != briefs.RuleShape || list.Findings[0].ClaimID != "briefs/secret/" {
		t.Fatalf("the unreadable folder must be a finding in data.findings, got %+v", list.Findings)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "brief", "show", "briefs/secret/plan.md")
	if env.OK || env.Error == nil || env.Error.Code != cliout.CodeBriefNotFound || !strings.Contains(env.Error.Hint, "finding") {
		t.Fatalf("a brief behind an unreadable folder must be brief_not_found naming the findings, got %+v", env)
	}
	if raw, _ := json.Marshal(env.Error.Details); !strings.Contains(string(raw), "briefs/secret/") {
		t.Fatalf("brief_not_found must carry the tree findings in details, got %s", raw)
	}

	// The only folder unreadable: nothing to list, and the text must not call
	// that a project with no briefs.
	if err := os.RemoveAll(filepath.Join(root, "briefs", "widget")); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := execCLI(t, "--config", cfgPath, "brief", "list")
	if err != nil {
		t.Fatalf("brief list (text): %v", err)
	}
	if strings.Contains(stdout, "holds no briefs") || !strings.Contains(stdout, "briefs/secret/") {
		t.Fatalf("an unreadable tree must not read as a project with no briefs:\n%s", stdout)
	}
}

// TestBriefFindingsFailCheckAndNeverGateAClaim pins both halves of the
// coupling rule at the CLI: a brief over its cap fails `check --validate` with
// lint_failed and a brief-word-cap finding naming the brief's path, AND the
// same project's `claim lock --dry-run` is not blocked by it — brief state never
// affects claim locking.
func TestBriefFindingsFailCheckAndNeverGateAClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	writeProjectConfigFile(t, cfgPath, string(cfg)+"max_brief_words: 2\n")
	writeBriefFile(t, root, "widget/flow.md", widgetFlowBrief)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err == nil || env.OK || env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("an over-cap brief must fail check with lint_failed, got %+v", env)
	}
	var data checkData
	decodeData(t, env, &data)
	found := false
	for _, f := range data.LintFindings {
		if f.Lint == briefs.RuleWordCap && f.ClaimID == "briefs/widget/flow.md" && f.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a brief-word-cap finding on the brief's path, got %+v", data.LintFindings)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved", "--dry-run")
	if err != nil || !env.OK {
		t.Fatalf("claim lock --dry-run: %v %+v", err, env)
	}
	var preview policyLockPreviewData
	decodeData(t, env, &preview)
	if preview.Blocked {
		t.Fatalf("a brief finding must not block a claim lock, got %+v", preview)
	}
}
