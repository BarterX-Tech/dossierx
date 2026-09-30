package main

import (
	"encoding/json"
	"os"
	"os/exec"
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
	// A brief's first lock has no earlier record: every baseline is read
	// fresh, nothing is carried (an empty list, never null), and it is not a
	// re-lock.
	if locked.Relocked || locked.Carried == nil || len(locked.Carried) != 0 || len(locked.Baselines) != 1 || locked.Baselines["widget.contract.overview"] == "" {
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
// that edited brief again is how its edit is approved (relocked) — which keeps
// the claim change pending for the reaudit it now allows.
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
	// The re-lock signed the brief's edit and nothing else: the claim that moved
	// is still a dependency-drift warning, which the now-allowed reaudit clears.
	if lint, ledger := validateFindings(t, cfgPath); len(ledger) != 0 || !hasFinding(lint, briefs.RuleDependencyDrift, flow, "warning") {
		t.Fatalf("a re-lock must sign the edit and keep the claim change pending: lint %+v ledger %+v", lint, ledger)
	}
	mustOK(t, "--config", cfgPath, "brief", "reaudit", flow, "--confirm", "--reason", "the claim change is fine")
	if lint, ledger := validateFindings(t, cfgPath); len(ledger) != 0 || hasBriefRule(lint) {
		t.Fatalf("after the reaudit the project must be clean: lint %+v ledger %+v", lint, ledger)
	}
}

// TestARelockKeepsAReviewPendingBriefPending pins F1: a brief review-pending
// because a claim it rests on moved, then edited and re-locked, is STILL
// review_pending — the re-lock approves the brief's words and carries the
// baseline forward (carried_baselines names it) — and brief reaudit, allowed on
// the re-locked brief, is what shows and clears the claim change. A claim newly
// added to rests_on in the same edit is baselined as it reads now.
func TestARelockKeepsAReviewPendingBriefPending(t *testing.T) {
	cfgPath, claimPath, briefPath, _ := briefLockProject(t)
	root := filepath.Dir(cfgPath)
	const flow = "briefs/widget/flow.md"
	extra := "id: widget.contract.extra\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: A second claim.\nbody: |\n  extra.\nrests_on:\n  - widget.contract.overview\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "extra.yaml"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	claim := mustRead(t, claimPath)
	if err := os.WriteFile(claimPath, []byte(strings.Replace(string(claim), "body: |\n", "body: |\n  rewritten.\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(mustRead(t, briefPath)), "  - widget.contract.overview\n", "  - widget.contract.overview\n  - widget.contract.extra\n", 1)
	edited = strings.Replace(edited, "One paragraph.", "One paragraph, with a typo fixed.", 1)
	if err := os.WriteFile(briefPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	var relock briefLockData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "the typo fix is fine"), &relock)
	if !relock.Relocked || strings.Join(relock.Carried, ",") != "widget.contract.overview" || relock.Baselines["widget.contract.extra"] == "" {
		t.Fatalf("a re-lock must carry the standing baseline and baseline the new claim: %+v", relock)
	}
	var list briefListData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "list", "--review-pending"), &list)
	if list.Count != 1 || list.Briefs[0].ReviewPendingTrigger != briefs.TriggerDependencyDrift {
		t.Fatalf("the re-locked brief must stay review_pending on the moved claim, got %+v", list)
	}
	mustOK(t, "--config", cfgPath, "brief", "reaudit", flow, "--confirm", "--reason", "the claim change is fine")
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "list", "--review-pending"), &list)
	if list.Count != 0 {
		t.Fatalf("brief reaudit --confirm must clear it, got %+v", list)
	}
}

// TestUnlockEditLockKeepsAReviewPendingBriefPending pins the NIT-193 audit's
// follow-up to F1: a brief review-pending on a moved claim that is unlocked,
// edited and locked again is STILL review_pending. brief unlock releases the
// approval, but the lock after it carries the released record's baseline and
// receipt for every rests_on claim still listed (the dry run and the payload
// name it in carried_baselines, and the lock reads as relocked), baselines a
// claim newly listed as it reads now, and drops one no longer listed. The
// moved claim's diff is then shown by brief reaudit, whose --confirm clears it;
// check --staged, judging the committed index, agrees with --validate.
func TestUnlockEditLockKeepsAReviewPendingBriefPending(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git not on PATH: the --staged half of this test cannot run (a skip is a failure): %v", err)
	}
	cfgPath, claimPath, briefPath, storeFile := briefLockProject(t)
	root := filepath.Dir(cfgPath)
	const flow = "briefs/widget/flow.md"
	const overview, extra, third = "widget.contract.overview", "widget.contract.extra", "widget.contract.third"
	for name, id := range map[string]string{"extra.yaml": extra, "third.yaml": third} {
		src := "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: Another claim.\nbody: |\n  " + name + "\nrests_on:\n  - " + overview + "\n"
		if err := os.WriteFile(filepath.Join(root, "claims", name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(briefPath, []byte(strings.Replace(draftWidgetFlow, "  - "+overview+"\n", "  - "+overview+"\n  - "+extra+"\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	var first briefLockData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved"), &first)

	claim := mustRead(t, claimPath)
	if err := os.WriteFile(claimPath, []byte(strings.Replace(string(claim), "body: |\n", "body: |\n  rewritten.\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	mustOK(t, "--config", cfgPath, "brief", "unlock", flow, "--reason", "rework")
	edited := strings.Replace(string(mustRead(t, briefPath)), "  - "+extra+"\n", "  - "+third+"\n", 1)
	edited = strings.Replace(edited, "One paragraph.", "One paragraph, reworked.", 1)
	if err := os.WriteFile(briefPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	var dr cliout.DryRun
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--dry-run", "--reason", "x"), &dr)
	got, err := json.Marshal(dr.Proposed["carried_baselines"])
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `["`+overview+`"]` {
		t.Fatalf("the dry run must name the baseline carried over the released record, got %s", got)
	}
	var relock briefLockData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "the rework is approved"), &relock)
	if !relock.Relocked || strings.Join(relock.Carried, ",") != overview || len(relock.Baselines) != 2 ||
		relock.Baselines[overview] != first.Baselines[overview] || relock.Baselines[third] == "" {
		t.Fatalf("a lock after unlock must carry the released baseline, baseline the new claim and drop the removed one: %+v (first %+v)", relock, first)
	}
	store, err := lock.LoadStore(storeFile)
	if err != nil {
		t.Fatal(err)
	}
	rec, _ := store.BriefRecordFor("widget.flow")
	if _, ok := rec.Receipts[extra]; ok || rec.Receipts[third].ID != third || rec.Receipts[overview].Body == "" || rec.Released() {
		t.Fatalf("the new record's receipts: %+v", rec)
	}

	var list briefListData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "list", "--review-pending"), &list)
	if list.Count != 1 || list.Briefs[0].ReviewPendingTrigger != briefs.TriggerDependencyDrift || list.Briefs[0].LockState != "locked" {
		t.Fatalf("the brief must stay review_pending on the moved claim after unlock, edit and lock, got %+v", list)
	}
	stagedGit(t, root, "init", "-q", "-b", "main")
	stagedGit(t, root, "add", "-A")
	for _, mode := range []string{"--validate", "--staged"} {
		env, stderr, err := execCLIJSON(t, "--config", cfgPath, "check", mode)
		if err != nil || !env.OK {
			t.Fatalf("check %s: a warning must not fail it: %v %+v\n%s", mode, err, env.Error, stderr)
		}
		var data checkData
		decodeData(t, env, &data)
		if len(data.LedgerFindings) != 0 || !hasFinding(data.LintFindings, briefs.RuleDependencyDrift, flow, "warning") {
			t.Fatalf("check %s must report the carried review as brief-dependency-drift and nothing in the ledger: lint %+v ledger %+v", mode, data.LintFindings, data.LedgerFindings)
		}
	}

	var preview briefReauditData
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "reaudit", flow), &preview)
	if !preview.ReviewPending || len(preview.ChangedClaims) != 1 || preview.ChangedClaims[0].ID != overview ||
		preview.ChangedClaims[0].Baseline == nil || preview.ChangedClaims[0].Current == nil ||
		preview.ChangedClaims[0].Baseline.Body == preview.ChangedClaims[0].Current.Body {
		t.Fatalf("the reaudit preview must show the moved claim's wording then and now: %+v", preview)
	}
	mustOK(t, "--config", cfgPath, "brief", "reaudit", flow, "--confirm", "--reason", "the claim change is fine")
	decodeData(t, mustOK(t, "--config", cfgPath, "brief", "list", "--review-pending"), &list)
	if list.Count != 0 {
		t.Fatalf("brief reaudit --confirm must clear it, got %+v", list)
	}
}

// TestBriefCommandsRefuseAStoreFromANewerBinary pins F5 at the CLI: the error
// code is store_too_new whatever code the call site wraps a store load in, and
// nothing is written.
func TestBriefCommandsRefuseAStoreFromANewerBinary(t *testing.T) {
	cfgPath, _, briefPath, storeFile := briefLockProject(t)
	if err := os.WriteFile(storeFile, []byte(`{"version":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	before := string(mustRead(t, briefPath))
	env := mustCode(t, cliout.CodeStoreTooNew, "--config", cfgPath, "brief", "lock", "briefs/widget/flow.md", "--reason", "approved")
	if !strings.Contains(env.Error.Message, "newer dossierx") || string(mustRead(t, briefPath)) != before || string(mustRead(t, storeFile)) != `{"version":9}` {
		t.Fatalf("a newer store must refuse before anything is written: %+v", env.Error)
	}
}

// TestAFailedUnlockLandsInTheLoudState pins F6: brief unlock releases the
// record before it rewrites the file, so when the file cannot be written the
// project is left with status: locked on a released record — brief-unrecorded,
// reported — never a draft on a standing approval.
func TestAFailedUnlockLandsInTheLoudState(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced: a read-only directory is still writable on Windows and to root")
	}
	cfgPath, _, briefPath, _ := briefLockProject(t)
	const flow = "briefs/widget/flow.md"
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	dir := filepath.Dir(briefPath)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) }) //nolint:errcheck // best-effort restore for TempDir cleanup
	mustCode(t, cliout.CodeWriteFailed, "--config", cfgPath, "brief", "unlock", flow, "--reason", "rework")
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ledger := validateFindings(t, cfgPath); len(ledger) != 1 || ledger[0].Rule != briefs.RuleUnrecorded {
		t.Fatalf("a failed unlock must leave brief-unrecorded and nothing else, got %+v", ledger)
	}
}

// TestCheckCountsOpenBriefThreads pins F9: check reports a brief's open threads
// in data.open_brief_comments by path, and as an "open comments: brief" line.
func TestCheckCountsOpenBriefThreads(t *testing.T) {
	cfgPath, _, _, _ := briefLockProject(t)
	const flow = "briefs/widget/flow.md"
	mustOK(t, "--config", cfgPath, "comment", "add", flow, "--as", "human", "--body", "why?")
	var data checkData
	decodeData(t, mustOK(t, "--config", cfgPath, "check", "--validate"), &data)
	if data.OpenBriefComments[flow] != 1 {
		t.Fatalf("open_brief_comments = %+v", data.OpenBriefComments)
	}
	stdout, _, err := execCLI(t, "--config", cfgPath, "--format", "text", "check", "--validate")
	if err != nil || !strings.Contains(stdout, `open comments: brief "briefs/widget/flow.md": 1`) {
		t.Fatalf("text form: %v\n%s", err, stdout)
	}
}

// TestCommentErrorsNameTheBrief pins F11: a thread op on a brief names the
// brief, not a claim; and a comment verb given a brief's id (which the verbs
// do not take) answers claim_not_found with a hint naming the brief's path.
func TestCommentErrorsNameTheBrief(t *testing.T) {
	cfgPath, _, _, _ := briefLockProject(t)
	env := mustCode(t, cliout.CodeThreadNotFound, "--config", cfgPath, "comment", "reply", "briefs/widget/flow.md", "c-000000", "--as", "agent", "--body", "hi")
	if !strings.Contains(env.Error.Message, "on brief briefs/widget/flow.md") || strings.Contains(env.Error.Message, "claim") {
		t.Fatalf("a brief thread error must name the brief: %q", env.Error.Message)
	}
	for _, args := range [][]string{
		{"comment", "list", "widget.flow"},
		{"comment", "add", "widget.flow", "--as", "agent", "--body", "hi"},
	} {
		env = mustCode(t, cliout.CodeClaimNotFound, append([]string{"--config", cfgPath}, args...)...)
		if !strings.Contains(env.Error.Hint, "briefs/widget/flow.md") {
			t.Fatalf("%v: the hint must name the brief's path, got %q", args, env.Error.Hint)
		}
	}
}

// TestReauditPreviewShowsTheBodyAndIgnoresOpenThreads pins F12 and one of F13:
// the text preview of a body-only claim change prints the removed and added
// body lines, and an open thread on the brief does not refuse the preview (only
// --confirm is the human's yes it waits for).
func TestReauditPreviewShowsTheBodyAndIgnoresOpenThreads(t *testing.T) {
	cfgPath, claimPath, _, _ := briefLockProject(t)
	const flow = "briefs/widget/flow.md"
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	claim := string(mustRead(t, claimPath))
	if err := os.WriteFile(claimPath, []byte(strings.Replace(claim, "fixture claim for in-process CLI tests.", "a rewritten body line.", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	mustOK(t, "--config", cfgPath, "comment", "add", flow, "--as", "human", "--body", "why?")
	stdout, _, err := execCLI(t, "--config", cfgPath, "--format", "text", "brief", "reaudit", flow)
	if err != nil || !strings.Contains(stdout, "- body: fixture claim for in-process CLI tests.") || !strings.Contains(stdout, "+ body: a rewritten body line.") {
		t.Fatalf("the preview must print the body diff and not be refused by the open thread: %v\n%s", err, stdout)
	}
}

// TestBriefWritesRefuseAFileThatMovedUnderThem pins F15 and one of F13: the
// bytes a lock hashes must be the bytes it rewrites (briefBytesAsLoaded refuses
// a file that changed since discovery), and the rewrite itself refuses a file
// that changed since it was read, leaving the editor's bytes in place.
func TestBriefWritesRefuseAFileThatMovedUnderThem(t *testing.T) {
	cfgPath, _, briefPath, _ := briefLockProject(t)
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := briefs.Load(cfg).Lookup("widget.flow")
	if !ok {
		t.Fatal("fixture brief not found")
	}
	saved := draftWidgetFlow + "An editor's save.\n"
	if err := os.WriteFile(briefPath, []byte(saved), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := briefBytesAsLoaded(cfg, b); err == nil {
		t.Fatal("a file that changed since discovery must be refused before it is hashed")
	}
	if err := rewriteBriefFile(cfg, b, []byte(draftWidgetFlow), []byte("rewritten")); err == nil {
		t.Fatal("a file that changed since it was read must not be rewritten")
	}
	if string(mustRead(t, briefPath)) != saved {
		t.Fatal("the editor's save was overwritten")
	}
}

// TestImagesInTheLock pins F8 through the lock path: brief lock records the
// sha256 of the image the brief references (so the project is clean right
// after the lock), a swapped image under the same name is then
// brief-content-drift naming it, and the human's re-lock signs the new bytes.
func TestImagesInTheLock(t *testing.T) {
	cfgPath, _, briefPath, _ := briefLockProject(t)
	root := filepath.Dir(cfgPath)
	const flow = "briefs/widget/flow.md"
	if err := os.WriteFile(briefPath, []byte(strings.Replace(draftWidgetFlow, "One paragraph.", "One paragraph. ![diagram](diagram.svg)", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeBriefFile(t, root, "widget/diagram.svg", "<svg>approved</svg>")
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "approved")
	if lint, ledger := validateFindings(t, cfgPath); len(ledger) != 0 || hasBriefRule(lint) {
		t.Fatalf("a brief locked with its image must be clean: lint %+v ledger %+v", lint, ledger)
	}
	writeBriefFile(t, root, "widget/diagram.svg", "<svg>swapped</svg>")
	_, ledger := validateFindings(t, cfgPath)
	if len(ledger) != 1 || ledger[0].Rule != briefs.RuleContentDrift || !strings.Contains(ledger[0].Message, `"diagram.svg"`) {
		t.Fatalf("a swapped image must be brief-content-drift naming it, got %+v", ledger)
	}
	mustOK(t, "--config", cfgPath, "brief", "lock", flow, "--reason", "the new diagram is approved")
	if _, ledger := validateFindings(t, cfgPath); len(ledger) != 0 {
		t.Fatalf("the re-lock must sign the new image, got %+v", ledger)
	}
}
