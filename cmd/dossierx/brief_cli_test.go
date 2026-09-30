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

// TestBriefShowNotFoundNamesTheConfiguredBriefsDir pins that brief_not_found's
// recovery hint spells the path shape the way brief list prints it — briefs_dir
// relative to the config file, here docs/briefs — not a hard-coded briefs/,
// which names a directory this project does not read.
func TestBriefShowNotFoundNamesTheConfiguredBriefsDir(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	writeProjectConfigFile(t, cfgPath, string(cfg)+"briefs_dir: docs/briefs\n")
	writeBriefFile(t, filepath.Join(root, "docs"), "widget/flow.md", widgetFlowBrief)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "brief", "show", "widget.nope")
	if env.OK || env.Error == nil || env.Error.Code != cliout.CodeBriefNotFound || exitStatusFor(err) != 2 {
		t.Fatalf("expected brief_not_found at exit 2, got %+v", env)
	}
	if !strings.Contains(env.Error.Hint, "(docs/briefs/<folder>/<slug>.md)") {
		t.Fatalf("the hint must name the configured briefs_dir, got %q", env.Error.Hint)
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
	if env.OK || env.Error == nil || env.Error.Code != cliout.CodeBriefNotFound || !strings.Contains(env.Error.Hint, "finding") || exitStatusFor(err) != 2 {
		t.Fatalf("a brief behind an unreadable folder must be brief_not_found (exit 2) naming the findings, got %+v", env)
	}
	raw, err := json.Marshal(env.Error.Details)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "briefs/secret/") {
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
//
// The negative control has to be one the lock evaluator WOULD act on if brief
// findings reached it. Its scoping rule (lock.findingAffects) holds a finding
// against a candidate when the finding's claim_id is the candidate or its
// message names it; a brief finding's claim_id is a path, so a word-cap finding
// alone would pass even if routed in. The second brief's rests_on names an id
// that CONTAINS the candidate's id, so its brief-rests-on-unknown ERROR names
// the candidate in its message and would block the lock were brief findings
// ever handed to claim lock; and no brief-* rule may appear anywhere in the
// preview (routed-in findings on other paths would land in unrelated_findings).
func TestBriefFindingsFailCheckAndNeverGateAClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	writeProjectConfigFile(t, cfgPath, string(cfg)+"max_brief_words: 2\n")
	writeBriefFile(t, root, "widget/flow.md", widgetFlowBrief)
	writeBriefFile(t, root, "widget/retired.md", "---\nsummary: Rests on a retired id.\nrests_on:\n  - widget.contract.overview-retired\n---\nOk.\n")

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err == nil || env.OK || env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("an over-cap brief must fail check with lint_failed, got %+v", env)
	}
	var data checkData
	decodeData(t, env, &data)
	wordCap, namesCandidate := false, false
	for _, f := range data.LintFindings {
		if f.Lint == briefs.RuleWordCap && f.ClaimID == "briefs/widget/flow.md" && f.Severity == "error" {
			wordCap = true
		}
		if f.Lint == briefs.RuleRestsOnUnknown && f.ClaimID == "briefs/widget/retired.md" && f.Severity == "error" &&
			strings.Contains(f.Message, "widget.contract.overview") {
			namesCandidate = true
		}
	}
	if !wordCap || !namesCandidate {
		t.Fatalf("expected a brief-word-cap error and a brief-rests-on-unknown error naming the candidate, got %+v", data.LintFindings)
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
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"brief-`) {
		t.Fatalf("no brief rule may reach any claim lock preview field, got %s", raw)
	}
}

// TestCheck_BriefImagesOverTheViewerBoundAreNamed is NIT-197's F2 at the CLI
// boundary. The default caps allow more image bytes than the 64 MiB viewer
// holds, so a lint-clean project can reach the bound through its brief images
// alone. The refusal must say so — the images, their total and the bound in
// the message, and shrinking them in the hint — instead of the generic "reduce
// projected viewer content", and nothing is written. The image is a sparse
// file: its size is what discovery reads and the render charges, so the test
// needs no 65 MB of disk.
func TestCheck_BriefImagesOverTheViewerBoundAreNamed(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("max_brief_image_bytes: 70000000\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	writeBriefFile(t, root, "widget/flow.md", "---\nsummary: The flow.\n---\n# Flow\n\n![Flow](flow.png)\n")
	img := filepath.Join(root, "briefs", "widget", "flow.png")
	if err := os.WriteFile(img, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	const size = 65 << 20
	if err := os.Truncate(img, size); err != nil {
		t.Fatal(err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatal("check must fail when brief images exceed the viewer bound")
	}
	if env.OK || env.Error == nil || env.Error.Code != cliout.CodeConformanceCapacityExceeded {
		t.Fatalf("expected conformance_capacity_exceeded, got %+v", env.Error)
	}
	for _, want := range []string{"brief image", "68157440", "67108864", "briefs/widget/flow.png", "shrink or remove brief images"} {
		if !strings.Contains(env.Error.Message, want) {
			t.Errorf("message %q does not name %q", env.Error.Message, want)
		}
	}
	if !strings.Contains(env.Error.Hint, "shrink or remove brief images") || strings.Contains(env.Error.Hint, "facet duplication") {
		t.Errorf("hint must name the images as the recovery: %q", env.Error.Hint)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "viewer", "index.html")); err == nil {
		t.Error("a refused render must write no viewer")
	}
}
