package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

func init() {
	registerSurfacePayloadType("briefLockData", briefLockData{})
	registerSurfacePayloadType("briefUnlockData", briefUnlockData{})
	registerSurfacePayloadType("briefReauditData", briefReauditData{})
}

const draftWidgetFlow = "---\nsummary: How the widget flow reads end to end.\nrests_on:\n  - widget.contract.overview\n---\n# Widget flow\n\nOne paragraph.\n"

// briefLockProject is a fixture project with one draft brief resting on the
// fixture claim, and the paths a test reads back.
func briefLockProject(t *testing.T) (cfgPath, claimPath, briefPath, storeFile string) {
	t.Helper()
	root := t.TempDir()
	cfgPath, claimPath = icWriteFixtureProject(t, root, "widget")
	writeBriefFile(t, root, "widget/flow.md", draftWidgetFlow)
	return cfgPath, claimPath, filepath.Join(root, "briefs", "widget", "flow.md"), filepath.Join(root, "build", "ledger", "lock-store.json")
}

func mustOK(t *testing.T, args ...string) cliout.Envelope {
	t.Helper()
	env, stderr, err := execCLIJSON(t, args...)
	if err != nil || !env.OK {
		t.Fatalf("%v: %v %+v\n%s", args, err, env.Error, stderr)
	}
	return env
}

func mustCode(t *testing.T, code cliout.Code, args ...string) cliout.Envelope {
	t.Helper()
	env, _, err := execCLIJSON(t, args...)
	if err == nil || env.OK || env.Error == nil || env.Error.Code != code {
		t.Fatalf("%v: want %s, got %+v", args, code, env)
	}
	return env
}

func validateFindings(t *testing.T, cfgPath string) (lint []lintFindingData, ledger []lock.Finding) {
	t.Helper()
	env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if env.Data == nil {
		t.Fatalf("check --validate returned no data: %v %+v", err, env.Error)
	}
	var data checkData
	decodeData(t, env, &data)
	return data.LintFindings, data.LedgerFindings
}

// TestBriefLockLifecycle drives the write side end to end through the CLI: a
// lock records the brief's hash, approved text and one baseline (and moves the
// store to schema 4) and flips the file's status; a second lock is
// already_locked; a moved rests_on claim makes the brief review_pending in
// brief list --review-pending, brief show and check --validate (a warning);
// reaudit previews the claim's wording then and now, and --confirm clears it;
// unlock returns the brief to draft and stamps the release on the kept record.
func TestBriefLockLifecycle(t *testing.T) {
	cfgPath, claimPath, briefPath, storeFile := briefLockProject(t)
	const flow = "briefs/widget/flow.md"

	env := mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "the human approved the flow")
	var locked briefLockData
	decodeData(t, env, &locked)
	if locked.Relocked || len(locked.Baselines) != 1 || locked.Baselines["widget.contract.overview"] == "" {
		t.Fatalf("brief lock payload = %+v", locked)
	}
	raw := mustRead(t, briefPath)
	if !strings.Contains(string(raw), "status: locked\n") || !strings.HasSuffix(string(raw), "# Widget flow\n\nOne paragraph.\n") {
		t.Fatalf("the brief file after lock:\n%s", raw)
	}
	store, err := lock.LoadStore(storeFile)
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := store.BriefRecordFor("widget.flow")
	if !ok || rec.Hash != locked.Hash || rec.Reason != "the human approved the flow" || rec.Approved.Markdown != "# Widget flow\n\nOne paragraph.\n" || store.OnDiskVersion() != 4 {
		t.Fatalf("store record = %+v (version %d)", rec, store.OnDiskVersion())
	}
	if lint, ledger := validateFindings(t, cfgPath); len(ledger) != 0 || hasBriefRule(lint) {
		t.Fatalf("an approved brief must be clean: lint %+v ledger %+v", lint, ledger)
	}
	mustCode(t, cliout.CodeAlreadyLocked, "--config", cfgPath, "brief", "lock", flow, "--reason", "again")

	claim := mustRead(t, claimPath)
	moved := strings.Replace(string(claim), "body: |\n", "body: |\n  rewritten.\n", 1)
	if err := os.WriteFile(claimPath, []byte(moved), 0o644); err != nil {
		t.Fatal(err)
	}
	var list briefListData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "list", "--review-pending"), &list)
	if list.Count != 1 || !list.Briefs[0].ReviewPending || list.Briefs[0].ReviewPendingTrigger != briefs.TriggerDependencyDrift || list.Briefs[0].LockState != "locked" {
		t.Fatalf("brief list --review-pending = %+v", list)
	}
	lint, ledger := validateFindings(t, cfgPath)
	if len(ledger) != 0 || !hasFinding(lint, briefs.RuleDependencyDrift, flow, "warning") {
		t.Fatalf("a moved claim must be a brief-dependency-drift warning and nothing else: lint %+v ledger %+v", lint, ledger)
	}
	var preview briefReauditData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "reaudit", flow), &preview)
	if !preview.ReviewPending || len(preview.ChangedClaims) != 1 || preview.ChangedClaims[0].Baseline == nil || preview.ChangedClaims[0].Current == nil ||
		preview.ChangedClaims[0].Baseline.Body == preview.ChangedClaims[0].Current.Body {
		t.Fatalf("reaudit preview = %+v", preview)
	}
	mustOK(t, "--config", cfgPath, "brief", "reaudit", flow, "--confirm", "--reason", "the rewrite does not change the flow")
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "list", "--review-pending"), &list)
	if list.Count != 0 {
		t.Fatalf("a confirmed reaudit must clear review_pending, got %+v", list)
	}
	if lint, ledger := validateFindings(t, cfgPath); len(ledger) != 0 || hasBriefRule(lint) {
		t.Fatalf("after the reaudit the project must be clean: lint %+v ledger %+v", lint, ledger)
	}

	mustOK(t, "--config", cfgPath, "brief", "unlock", flow, "--reason", "rework")
	raw = mustRead(t, briefPath)
	if !strings.Contains(string(raw), "status: draft\n") {
		t.Fatalf("unlock must set status: draft:\n%s", raw)
	}
	if store, err = lock.LoadStore(storeFile); err != nil {
		t.Fatal(err)
	}
	if rec, ok := store.BriefRecordFor("widget.flow"); !ok || !rec.Released() || rec.ReleasedReason != "rework" || len(rec.Reaudits) != 1 {
		t.Fatalf("unlock must keep the record and stamp its release: %+v", rec)
	}
	mustCode(t, cliout.CodeNotLocked, "--config", cfgPath, "brief", "unlock", flow, "--reason", "again")
}

// mustRead is os.ReadFile that fails the test instead of returning an error.
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func hasFinding(fs []lintFindingData, rule, path, severity string) bool {
	for _, f := range fs {
		if f.Lint == rule && f.ClaimID == path && f.Severity == severity {
			return true
		}
	}
	return false
}

func hasBriefRule(fs []lintFindingData) bool {
	for _, f := range fs {
		if strings.HasPrefix(f.Lint, "brief-") {
			return true
		}
	}
	return false
}

// TestBriefWriteDryRunsWriteNothing pins the dry-run rule for every brief
// write: lock, unlock, reaudit --confirm and a comment add on a brief, each
// previewed with --dry-run, leave every file in the project byte-identical and
// create none.
func TestBriefWriteDryRunsWriteNothing(t *testing.T) {
	cfgPath, _, _, _ := briefLockProject(t)
	root := filepath.Dir(cfgPath)
	const flow = "briefs/widget/flow.md"
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	before := snapshotTree(t, root)
	for _, args := range [][]string{
		{"brief", "lock", flow, "--dry-run", "--reason", "x"},
		{"brief", "unlock", flow, "--dry-run", "--reason", "x"},
		{"brief", "reaudit", flow, "--dry-run", "--confirm", "--reason", "x"},
		{"brief", "reaudit", flow},
		{"comment", "add", flow, "--dry-run", "--as", "human", "--body", "why?"},
	} {
		mustOK(t, append([]string{"--config", cfgPath}, args...)...)
		if after := snapshotTree(t, root); !reflect.DeepEqual(after, before) {
			t.Fatalf("%v wrote to the project", args)
		}
	}
}

// TestAnOpenThreadRefusesBriefLockAndReaudit pins comment_open on both verbs,
// and the advisory route out of it: an agent replies, the human resolves, and
// only then does the lock go through. The thread is also in comment inbox,
// keyed by the brief's path.
func TestAnOpenThreadRefusesBriefLockAndReaudit(t *testing.T) {
	cfgPath, claimPath, _, _ := briefLockProject(t)
	const flow = "briefs/widget/flow.md"
	var added commentWriteData
	decodeData(t, mustOK(t, "--config", cfgPath, "comment", "add", flow, "--as", "human", "--body", "is the flow still right?"), &added)
	mustCode(t, cliout.CodeCommentOpen, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")

	var inbox commentInboxData
	decodeData(t, mustOK(t, "--config", cfgPath, "comment", "inbox"), &inbox)
	if inbox.Count != 1 || inbox.Threads[0].ClaimID != flow || inbox.Threads[0].Kind != "brief" || inbox.Threads[0].AgentCanResolve {
		t.Fatalf("comment inbox must carry the brief's thread by path, not resolvable by the agent: %+v", inbox)
	}
	mustOK(t, "--config", cfgPath, "comment", "reply", flow, added.ThreadID, "--as", "agent", "--body", "yes")

	// The human resolves (serve's route, NIT-198); the op is the one Deps runs.
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	deps, err := mutatingCommentDeps(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.BriefResolve(flow, added.ThreadID, "human"); err != nil {
		t.Fatal(err)
	}
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")

	// Reopened on a locked, review-pending brief: reaudit --confirm refuses.
	if _, err := deps.BriefReopen(flow, added.ThreadID, "human"); err != nil {
		t.Fatal(err)
	}
	claim := mustRead(t, claimPath)
	if err := os.WriteFile(claimPath, []byte(strings.Replace(string(claim), "body: |\n", "body: |\n  rewritten.\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCode(t, cliout.CodeCommentOpen, "--config", cfgPath, "brief", "reaudit", flow, "--confirm", "--reason", "fine")
}

// TestBriefStateNeverGatesAClaim is the approval contract at the CLI: with a
// brief edited since approval (brief-content-drift), another typed locked by
// hand (brief-unrecorded) and a third review-pending on the claim itself, the
// claim still locks, carries no review_pending, and its lock preview names no
// brief rule.
func TestBriefStateNeverGatesAClaim(t *testing.T) {
	cfgPath, claimPath, briefPath, _ := briefLockProject(t)
	root := filepath.Dir(cfgPath)
	const flow = "briefs/widget/flow.md"
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	raw := mustRead(t, briefPath)
	if err := os.WriteFile(briefPath, []byte(strings.Replace(string(raw), "One paragraph.", "One paragraph, edited.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBriefFile(t, root, "widget/typed.md", "---\nsummary: Typed locked by hand.\nstatus: locked\n---\nText.\n")
	_, ledger := validateFindings(t, cfgPath)
	if len(ledger) != 2 {
		t.Fatalf("fixture: expected brief-content-drift and brief-unrecorded, got %+v", ledger)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved", "--dry-run")
	if err != nil || !env.OK {
		t.Fatalf("claim lock --dry-run: %v %+v", err, env)
	}
	var preview policyLockPreviewData
	decodeData(t, env, &preview)
	rawPreview, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Blocked || strings.Contains(string(rawPreview), `"brief-`) {
		t.Fatalf("brief state must not reach a claim lock: %s", rawPreview)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	claim := mustRead(t, claimPath)
	if strings.Contains(string(claim), "review_pending") {
		t.Fatalf("a brief must never set review_pending on a claim:\n%s", claim)
	}
}

// TestBriefLockRefusesALinkedBriefAndWritesNothing: a brief file that is a
// symlink is refused by discovery (brief-shape), so brief lock cannot find it
// (brief_not_found) and neither the link's target nor the lock store is
// written.
func TestBriefLockRefusesALinkedBriefAndWritesNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires POSIX symlinks")
	}
	cfgPath, _, _, storeFile := briefLockProject(t)
	root := filepath.Dir(cfgPath)
	target := filepath.Join(root, "outside.md")
	if err := os.WriteFile(target, []byte(draftWidgetFlow), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "briefs", "widget", "linked.md")); err != nil {
		t.Fatal(err)
	}
	storeBefore := mustRead(t, storeFile)
	mustCode(t, cliout.CodeBriefNotFound, "--config", cfgPath, "brief", "lock", "briefs/widget/linked.md", "--reason", "approved")
	if got := mustRead(t, target); string(got) != draftWidgetFlow {
		t.Fatal("the link's target was written")
	}
	if got := mustRead(t, storeFile); string(got) != string(storeBefore) {
		t.Fatal("the lock store was written for a refused lock")
	}
}

// TestBriefListAnswersWhenTheStoreCannotBeRead: an unreadable lock store does
// not take brief list down — it answers, says in a warning that the store
// could not be read, and reports a locked brief as unrecorded (no evidence it
// was approved), the same fail-closed reading check gives.
func TestBriefListAnswersWhenTheStoreCannotBeRead(t *testing.T) {
	cfgPath, _, _, storeFile := briefLockProject(t)
	mustOK(t, "--config", cfgPath, "brief", "lock", "briefs/widget/flow.md", "--reason", "approved")
	if err := os.WriteFile(storeFile, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := mustOK(t, "--config", cfgPath, "brief", "list")
	var list briefListData
	decodeData(t, env, &list)
	if len(env.Warnings) == 0 || !strings.Contains(strings.Join(env.Warnings, " "), "lock store could not be read") || list.Briefs[0].LockState != "unrecorded" {
		t.Fatalf("an unreadable store must be a warning and the locked brief unrecorded, got warnings %v entries %+v", env.Warnings, list.Briefs)
	}
}

// TestBriefLockAndReauditRefuseWhatTheyCannotSign pins the refusals FORMAT.md
// names beyond comment_open: brief lock refuses a brief with an error finding
// (lint_failed: an unknown rests_on id, and a frontmatter defect); brief
// reaudit --confirm refuses a
// brief edited since its approval (integrity_failed), because a reaudit
// accepts moved claims and does not approve the brief's own edit; and locking
// that edited brief again is how its edit is approved (relocked).
func TestBriefLockAndReauditRefuseWhatTheyCannotSign(t *testing.T) {
	cfgPath, claimPath, briefPath, _ := briefLockProject(t)
	const flow = "briefs/widget/flow.md"
	if err := os.WriteFile(briefPath, []byte(strings.Replace(draftWidgetFlow, "widget.contract.overview", "widget.contract.ghost", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCode(t, cliout.CodeLintFailed, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	if err := os.WriteFile(briefPath, []byte(strings.Replace(draftWidgetFlow, "---\n# Widget", "owner: me\n---\n# Widget", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCode(t, cliout.CodeLintFailed, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")

	if err := os.WriteFile(briefPath, []byte(draftWidgetFlow), 0o644); err != nil {
		t.Fatal(err)
	}
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	raw := mustRead(t, briefPath)
	if err := os.WriteFile(briefPath, []byte(strings.Replace(string(raw), "One paragraph.", "One paragraph, edited.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	claim := mustRead(t, claimPath)
	if err := os.WriteFile(claimPath, []byte(strings.Replace(string(claim), "body: |\n", "body: |\n  rewritten.\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCode(t, cliout.CodeIntegrityFailed, "--config", cfgPath, "brief", "reaudit", flow, "--confirm", "--reason", "fine")

	var relock briefLockData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "the edit is approved"), &relock)
	if !relock.Relocked {
		t.Fatalf("locking an edited brief is a re-lock, got %+v", relock)
	}
	if lint, ledger := validateFindings(t, cfgPath); len(ledger) != 0 || hasBriefRule(lint) {
		t.Fatalf("a re-lock signs the edit and re-baselines the claims: lint %+v ledger %+v", lint, ledger)
	}
}
