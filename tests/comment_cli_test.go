// comment_cli_test.go exercises the "dossierx comment" verb group end to end
// via the built binary (reusing cli_test.go's run/binPath and
// lock_lifecycle_test.go's llWriteConfig/llWriteClaim/llClaimSpec/llReadFile
// helpers), plus the lock gate, the buildorder/check integration, and the
// reaudit comment-only refusal that all consume the same open-thread state.
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// firstToken returns the first whitespace-delimited token in s that begins
// with prefix (trailing ";," stripped) — used to pull the minted "c-"/"r-" id
// out of a comment verb's echo line.
func firstToken(t *testing.T, s, prefix string) string {
	t.Helper()
	for _, f := range strings.Fields(s) {
		f = strings.TrimRight(f, ";,")
		if strings.HasPrefix(f, prefix) {
			return f
		}
	}
	t.Fatalf("no token with prefix %q in output: %s", prefix, s)
	return ""
}

// writeBannerClaim writes a banner-layout claim (llWriteClaim can't set a
// layout), used to prove comment add is refused on banner claims.
func writeBannerClaim(t *testing.T, root, id, module string) string {
	t.Helper()
	body := "id: " + id + "\n" +
		"facet: contract\nmodule: " + module + "\nstatus: draft\nlayout: banner\n" +
		"body: |\n  Section divider.\n" +
		"governed_by:\n  type: none\n  reason: fixture banner claim\n"
	path := filepath.Join(root, "claims", lastSegment(id)+".yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write banner claim: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------
// Full loop: add -> lock refused (names id) -> reply -> resolve -> lock ok.
// ---------------------------------------------------------------------

func TestComment_AddLockRefusedResolveLockSucceeds(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.overview", facet: "contract", module: "widget", status: "draft", body: "a claim under review."})

	addOut, addErr, addCode := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.overview", "--as", "human", "--body", "please clarify the retry policy")
	if addCode != 0 {
		t.Fatalf("comment add: exit %d\nstdout: %s\nstderr: %s", addCode, addOut, addErr)
	}
	tid := firstToken(t, addOut, "c-")

	// A draft claim never carries review_pending, even with an open thread.
	if strings.Contains(llReadFile(t, claimPath), "review_pending: true") {
		t.Fatalf("did not expect review_pending on a DRAFT claim, got:\n%s", llReadFile(t, claimPath))
	}

	// Lock refused while the thread is open, and the refusal names the id.
	lockOut, lockErr, lockCode := run(t, root, "--config", cfgPath, "lock", "widget.contract.overview")
	if lockCode == 0 {
		t.Fatalf("expected lock refused while an open thread exists, got exit 0\nstdout: %s", lockOut)
	}
	if !strings.Contains(lockOut+lockErr, tid) {
		t.Fatalf("expected the lock refusal to name the open thread id %q, got stdout: %s stderr: %s", tid, lockOut, lockErr)
	}
	if !strings.Contains(llReadFile(t, claimPath), "status: draft") {
		t.Fatalf("expected the claim to stay draft after a refused lock")
	}

	// A reply (any actor) keeps the thread open, so lock stays refused.
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "reply", "widget.contract.overview", tid, "--as", "agent", "--body", "addressed, please confirm"); code != 0 {
		t.Fatalf("comment reply: exit %d, stderr: %s", code, stderr)
	}
	if _, _, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.overview"); code == 0 {
		t.Fatalf("expected lock still refused while the thread remains open after a reply")
	}

	// Resolve, then lock succeeds.
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "resolve", "widget.contract.overview", tid, "--as", "human"); code != 0 {
		t.Fatalf("comment resolve: exit %d, stderr: %s", code, stderr)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.overview"); code != 0 {
		t.Fatalf("expected lock to succeed once the thread is resolved, stderr: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, claimPath), "status: locked") {
		t.Fatalf("expected the claim locked on disk after resolving the thread")
	}
}

// ---------------------------------------------------------------------
// Candidate-scoped gate: lock B succeeds while unrelated locked A has an
// open thread.
// ---------------------------------------------------------------------

func TestComment_LockBSucceedsWhileLockedAHasOpenThread(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	aPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.a", facet: "contract", module: "widget", status: "draft", body: "claim A."})
	bPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.b", facet: "contract", module: "widget", status: "draft", body: "claim B."})

	if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.a"); code != 0 {
		t.Fatalf("lock A: %s", stderr)
	}
	// Open a thread on the already-locked A: A is now review_pending with an
	// open thread.
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.a", "--as", "human", "--body", "revisit A"); code != 0 {
		t.Fatalf("comment add on A: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, aPath), "review_pending: true") {
		t.Fatalf("expected A review_pending after commenting on the locked claim")
	}

	// Locking the unrelated, thread-free B must still succeed.
	if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.b"); code != 0 {
		t.Fatalf("expected lock of B to succeed while A has an open thread, stderr: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, bPath), "status: locked") {
		t.Fatalf("expected B locked on disk")
	}
	// A is untouched: still locked, still review_pending.
	if !strings.Contains(llReadFile(t, aPath), "review_pending: true") {
		t.Fatalf("expected A still review_pending (untouched by locking B)")
	}
}

// ---------------------------------------------------------------------
// Locked -> comment -> review_pending -> stale lists it -> resolve -> cleared.
// ---------------------------------------------------------------------

func TestComment_OnLockedSetsReviewPendingClearedOnResolve(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "a lockable claim."})

	if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.main"); code != 0 {
		t.Fatalf("lock: %s", stderr)
	}

	addOut, _, addCode := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "revisit this")
	if addCode != 0 {
		t.Fatalf("comment add on locked claim: %s", addOut)
	}
	tid := firstToken(t, addOut, "c-")
	if !strings.Contains(llReadFile(t, claimPath), "review_pending: true") {
		t.Fatalf("expected review_pending set after commenting on a locked claim, got:\n%s", llReadFile(t, claimPath))
	}

	staleOut, _, staleCode := run(t, root, "--config", cfgPath, "stale")
	if staleCode != 0 {
		t.Fatalf("stale exited %d", staleCode)
	}
	if !strings.Contains(staleOut, "widget.contract.main") {
		t.Fatalf("expected stale to list the commented locked claim, got: %s", staleOut)
	}

	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "resolve", "widget.contract.main", tid, "--as", "human"); code != 0 {
		t.Fatalf("comment resolve: %s", stderr)
	}
	after := llReadFile(t, claimPath)
	if strings.Contains(after, "review_pending: true") {
		t.Fatalf("expected review_pending cleared after resolving the last open thread, got:\n%s", after)
	}
	if !strings.Contains(after, "status: locked") {
		t.Fatalf("expected the claim to stay locked, got:\n%s", after)
	}
}

// ---------------------------------------------------------------------
// reaudit on a comment-only review_pending claim -> exit 2, claim file
// BYTE-IDENTICAL (no audit_notes growth, no lock/hash mutation), no lock leak.
// ---------------------------------------------------------------------

func TestComment_ReauditCommentOnlyRefusedExit2ByteIdentical(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "a lockable claim."})

	if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.main"); code != 0 {
		t.Fatalf("lock: %s", stderr)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "agent", "--body", "is this still true?"); code != 0 {
		t.Fatalf("comment add: %s", stderr)
	}

	before := llReadFile(t, claimPath)
	if !strings.Contains(before, "review_pending: true") {
		t.Fatalf("precondition: expected the claim review_pending from its open thread")
	}

	out, stderr, code := run(t, root, "--config", cfgPath, "reaudit", "widget.contract.main")
	if code != 2 {
		t.Fatalf("expected reaudit on a comment-only review_pending claim to exit 2, got %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}

	after := llReadFile(t, claimPath)
	if after != before {
		t.Fatalf("expected the claim file byte-identical after a refused comment-only reaudit\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if strings.Contains(after, "audit_notes:") {
		t.Fatalf("expected NO audit note appended by a refused reaudit, got:\n%s", after)
	}

	// No lock leak: a follow-up check (which takes the claims + lock-store
	// sentinels) must run cleanly, proving reaudit's deferred releases ran
	// despite the exit-2 refusal.
	if _, stderr, code := run(t, root, "--config", cfgPath, "check"); code != 0 {
		t.Fatalf("expected check to succeed after a refused reaudit (no leaked lock), stderr: %s", stderr)
	}
}

// ---------------------------------------------------------------------
// drift + comment: reaudit --confirm applies the drift half but review_pending
// STAYS true (the open thread), the claim is still stale; resolving finally
// clears it.
// ---------------------------------------------------------------------

func TestComment_DriftPlusComment_ReauditRetainsReviewPending(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	depPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.dep", facet: "contract", module: "widget", status: "draft", body: "dependency, v1."})
	mainPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "main resting on dep.", restsOn: []string{"widget.contract.dep"}})

	for _, id := range []string{"widget.contract.dep", "widget.contract.main"} {
		if _, stderr, code := run(t, root, "--config", cfgPath, "lock", id); code != 0 {
			t.Fatalf("lock %s: %s", id, stderr)
		}
	}

	// Drift the dependency, then check flips main to review_pending.
	dep := strings.Replace(llReadFile(t, depPath), "dependency, v1.", "dependency, v2.", 1)
	if err := os.WriteFile(depPath, []byte(dep), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "check"); code != 0 {
		t.Fatalf("check: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, mainPath), "review_pending: true") {
		t.Fatalf("expected main review_pending from the drifted dependency")
	}

	// Now ALSO open a comment thread on main: two independent triggers.
	addOut, _, addCode := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "double-check this against the new dep")
	if addCode != 0 {
		t.Fatalf("comment add: %s", addOut)
	}
	tid := firstToken(t, addOut, "c-")

	// reaudit --confirm applies the drift half (audit note) but leaves
	// review_pending TRUE because the open thread still stands.
	confirmOut, stderr, confirmCode := run(t, root, "--config", cfgPath, "reaudit", "widget.contract.main", "--confirm")
	if confirmCode != 0 {
		t.Fatalf("reaudit --confirm on a drift+comment claim should exit 0, got %d\nstdout: %s\nstderr: %s", confirmCode, confirmOut, stderr)
	}
	if !strings.Contains(confirmOut, "retained") {
		t.Fatalf("expected the confirm output to report review_pending retained, got: %s", confirmOut)
	}
	afterConfirm := llReadFile(t, mainPath)
	if !strings.Contains(afterConfirm, "review_pending: true") {
		t.Fatalf("expected review_pending RETAINED after confirming a drift+comment reaudit, got:\n%s", afterConfirm)
	}
	if !strings.Contains(afterConfirm, "audit_notes:") {
		t.Fatalf("expected the drift half applied (audit note appended), got:\n%s", afterConfirm)
	}
	if !strings.Contains(afterConfirm, "status: locked") {
		t.Fatalf("expected the claim still locked, got:\n%s", afterConfirm)
	}

	staleOut, _, _ := run(t, root, "--config", cfgPath, "stale")
	if !strings.Contains(staleOut, "widget.contract.main") {
		t.Fatalf("expected main still stale (open thread), got: %s", staleOut)
	}

	// Resolving the thread now clears review_pending (drift already confirmed).
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "resolve", "widget.contract.main", tid, "--as", "human"); code != 0 {
		t.Fatalf("comment resolve: %s", stderr)
	}
	if strings.Contains(llReadFile(t, mainPath), "review_pending: true") {
		t.Fatalf("expected review_pending finally cleared once the thread is resolved and drift already confirmed")
	}
}

// ---------------------------------------------------------------------
// check on a locked+open-thread project: exit 0, still writes catalog + viewer,
// prints the comment next-step and the open-comments summary, NOT a reaudit hint.
// ---------------------------------------------------------------------

func TestComment_CheckOnLockedOpenThread(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "a lockable claim."})

	if _, stderr, code := run(t, root, "--config", cfgPath, "lock", "widget.contract.main"); code != 0 {
		t.Fatalf("lock: %s", stderr)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "revisit"); code != 0 {
		t.Fatalf("comment add: %s", stderr)
	}

	out, stderr, code := run(t, root, "--config", cfgPath, "check")
	if code != 0 {
		t.Fatalf("expected check to exit 0 on a locked+open-thread project, got %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".catalog.json")); err != nil {
		t.Fatalf("expected .catalog.json written by check: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "viewer", "index.html")); err != nil {
		t.Fatalf("expected viewer/index.html written by check: %v", err)
	}
	if !strings.Contains(out, `open comments: module "widget": 1`) {
		t.Fatalf("expected an open-comments summary line, got: %s", out)
	}
	if !strings.Contains(out, "dossierx comment resolve") {
		t.Fatalf("expected the comment-resolve next step, got: %s", out)
	}
	if strings.Contains(out, "dossierx reaudit") {
		t.Fatalf("did NOT expect a reaudit hint for a comment-only pending claim, got: %s", out)
	}
}

// ---------------------------------------------------------------------
// comment list: pinned greppable line format, and --json shape.
// ---------------------------------------------------------------------

func TestComment_ListFormatAndJSON(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "a claim."})

	addOut, _, addCode := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "clarify the retry policy")
	if addCode != 0 {
		t.Fatalf("comment add: %s", addOut)
	}
	tid := firstToken(t, addOut, "c-")

	// Pinned line format: "<tid> <status> <author> <created> replies=<N>: <body>"
	listOut, _, listCode := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.main")
	if listCode != 0 {
		t.Fatalf("comment list exited %d", listCode)
	}
	line := strings.TrimSpace(listOut)
	fields := strings.Fields(line)
	if len(fields) < 6 {
		t.Fatalf("expected the pinned list line to have >=6 fields, got %q", line)
	}
	if fields[0] != tid {
		t.Fatalf("expected field 0 to be the thread id %q, got %q (line: %q)", tid, fields[0], line)
	}
	if fields[1] != "open" {
		t.Fatalf("expected field 1 to be the status \"open\", got %q (line: %q)", fields[1], line)
	}
	if fields[2] != "human" {
		t.Fatalf("expected field 2 to be the author \"human\", got %q (line: %q)", fields[2], line)
	}
	if !strings.Contains(line, "replies=0:") {
		t.Fatalf("expected \"replies=0:\" in the line, got %q", line)
	}
	if !strings.Contains(line, "clarify the retry policy") {
		t.Fatalf("expected the body in the line, got %q", line)
	}

	// --json: stdout is a pure JSON array; the human hint goes to stderr.
	jsonOut, jsonErr, jsonCode := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.main", "--json")
	if jsonCode != 0 {
		t.Fatalf("comment list --json exited %d", jsonCode)
	}
	var threads []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &threads); err != nil {
		t.Fatalf("comment list --json stdout is not a JSON array: %v\nstdout: %s", err, jsonOut)
	}
	if len(threads) != 1 {
		t.Fatalf("expected exactly one thread in --json output, got %d: %v", len(threads), threads)
	}
	if strings.Contains(jsonOut, "comment list:") {
		t.Fatalf("expected stdout to be pure JSON (hint belongs on stderr), got: %s", jsonOut)
	}
	if !strings.Contains(jsonErr, "comment list:") {
		t.Fatalf("expected the --json hint on stderr, got: %s", jsonErr)
	}

	// --open filters to unresolved threads only.
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "resolve", "widget.contract.main", tid, "--as", "human"); code != 0 {
		t.Fatalf("comment resolve: %s", stderr)
	}
	openOut, _, _ := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.main", "--open")
	if strings.Contains(openOut, tid) {
		t.Fatalf("expected --open to hide the now-resolved thread, got: %s", openOut)
	}
}

// ---------------------------------------------------------------------
// reopen / edit / delete wiring, including the --reply target flag.
// ---------------------------------------------------------------------

func TestComment_ReopenEditDeleteWiring(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "a claim."})

	addOut, _, addCode := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "original")
	if addCode != 0 {
		t.Fatalf("comment add: %s", addOut)
	}
	tid := firstToken(t, addOut, "c-")

	// Resolve then reopen (both human, matching the human-opened thread's rights).
	if _, e, c := run(t, root, "--config", cfgPath, "comment", "resolve", "widget.contract.main", tid, "--as", "human"); c != 0 {
		t.Fatalf("resolve: %s", e)
	}
	if _, e, c := run(t, root, "--config", cfgPath, "comment", "reopen", "widget.contract.main", tid, "--as", "human"); c != 0 {
		t.Fatalf("reopen: %s", e)
	}

	// Edit the thread root body.
	if _, e, c := run(t, root, "--config", cfgPath, "comment", "edit", "widget.contract.main", tid, "--as", "human", "--body", "edited-root-body"); c != 0 {
		t.Fatalf("edit root: %s", e)
	}
	if listOut, _, _ := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.main"); !strings.Contains(listOut, "edited-root-body") {
		t.Fatalf("expected the edited body in list, got: %s", listOut)
	}

	// Reply, edit the reply with --reply, delete the reply with --reply.
	replyOut, _, replyCode := run(t, root, "--config", cfgPath, "comment", "reply", "widget.contract.main", tid, "--as", "agent", "--body", "a reply")
	if replyCode != 0 {
		t.Fatalf("comment reply: %s", replyOut)
	}
	rid := firstToken(t, replyOut, "r-")
	if _, e, c := run(t, root, "--config", cfgPath, "comment", "edit", "widget.contract.main", tid, "--as", "agent", "--body", "edited reply", "--reply", rid); c != 0 {
		t.Fatalf("edit reply: %s", e)
	}
	if _, e, c := run(t, root, "--config", cfgPath, "comment", "delete", "widget.contract.main", tid, "--as", "agent", "--reply", rid); c != 0 {
		t.Fatalf("delete reply: %s", e)
	}

	// Delete the whole thread -> list is empty.
	if _, e, c := run(t, root, "--config", cfgPath, "comment", "delete", "widget.contract.main", tid, "--as", "human"); c != 0 {
		t.Fatalf("delete thread: %s", e)
	}
	if finalList, _, _ := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.main"); strings.Contains(finalList, tid) {
		t.Fatalf("expected no threads after deleting, got: %s", finalList)
	}
}

// ---------------------------------------------------------------------
// comment add is refused on a banner-layout claim.
// ---------------------------------------------------------------------

func TestComment_AddRefusedOnBannerClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	bannerPath := writeBannerClaim(t, root, "widget.contract.divider", "widget")

	before := llReadFile(t, bannerPath)
	out, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.divider", "--as", "human", "--body", "no threads here")
	if code == 0 {
		t.Fatalf("expected comment add on a banner claim to be refused, got exit 0 (stdout: %s)", out)
	}
	if !strings.Contains(stderr+out, "banner") {
		t.Fatalf("expected the refusal to mention banner, got stdout: %s stderr: %s", out, stderr)
	}
	if llReadFile(t, bannerPath) != before {
		t.Fatalf("expected the banner claim file untouched after a refused comment add")
	}
}
