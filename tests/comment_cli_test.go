// comment_cli_test.go exercises the "dossierx comment" verb group end to end
// via the built binary (reusing cli_test.go's run/binPath and
// lock_lifecycle_test.go's llWriteConfig/llWriteClaim/llClaimSpec/llReadFile
// helpers), plus the lock gate, the buildorder/check integration, and the
// reaudit comment-only refusal that all consume the same open-thread state.
//
// v0.3.0 removed edit, delete, resolve and reopen from the CLI (they live in
// the viewer, where the rights holder is). They are still the same
// internal/comments operations, still reachable over serve's HTTP API, and
// several invariants here — above all "resolving the last open thread clears
// review_pending and unblocks the lock gate" — are about the OPERATION, not
// about argv. Those are driven through internal/comments directly, via
// resolveThread below; the CLI's refusal to offer them is itself pinned by
// TestComment_RetiredVerbsAreGoneFromTheCLI.
package tests

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// commentDeps builds the internal/comments Deps for a fixture project, wired
// exactly the way cmd/dossierx's mutatingCommentDeps and internal/serve both
// wire it: store PATHS, not snapshots, so each op re-reads them fresh inside
// the claims sentinel.
func commentDeps(t *testing.T, cfgPath string) *comments.Deps {
	t.Helper()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config %s: %v", cfgPath, err)
	}
	return &comments.Deps{
		Cfg:           cfg,
		LockStorePath: filepath.Join(cfg.Dir(), "build", "ledger", "lock-store.json"),
		FlagStorePath: filepath.Join(cfg.Dir(), "build", "ledger", "flag-store.json"),
	}
}

// resolveThread performs the human's Resolve — the single most load-bearing
// action in this release's review loop, since that click is what clears
// review_pending and lets a claim lock. There is no CLI verb for it any more,
// so it goes through the same package call the viewer's API handler makes.
func resolveThread(t *testing.T, cfgPath, claimID, threadID string) {
	t.Helper()
	if _, err := commentDeps(t, cfgPath).Resolve(claimID, threadID, model.CommentRoleHuman); err != nil {
		t.Fatalf("resolve thread %s on %s: %v", threadID, claimID, err)
	}
}

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
	lockOut, lockErr, lockCode := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "test fixture")
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
	if _, _, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "test fixture"); code == 0 {
		t.Fatalf("expected lock still refused while the thread remains open after a reply")
	}

	// Resolve (the human's viewer click), then lock succeeds.
	resolveThread(t, cfgPath, "widget.contract.overview", tid)
	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "test fixture"); code != 0 {
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

	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.a", "--reason", "test fixture"); code != 0 {
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
	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.b", "--reason", "test fixture"); code != 0 {
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

	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
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

	staleOut, _, staleCode := run(t, root, "--config", cfgPath, "claim", "list", "--review-pending")
	if staleCode != 0 {
		t.Fatalf("stale exited %d", staleCode)
	}
	if !strings.Contains(staleOut, "widget.contract.main") {
		t.Fatalf("expected stale to list the commented locked claim, got: %s", staleOut)
	}

	resolveThread(t, cfgPath, "widget.contract.main", tid)
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

	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock: %s", stderr)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "agent", "--body", "is this still true?"); code != 0 {
		t.Fatalf("comment add: %s", stderr)
	}

	before := llReadFile(t, claimPath)
	if !strings.Contains(before, "review_pending: true") {
		t.Fatalf("precondition: expected the claim review_pending from its open thread")
	}

	out, stderr, code := run(t, root, "--config", cfgPath, "claim", "reaudit", "widget.contract.main")
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
		if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", id, "--reason", "test fixture"); code != 0 {
			t.Fatalf("lock %s: %s", id, stderr)
		}
	}

	// Drift the dependency, then check flips main to review_pending.
	dep := strings.Replace(llReadFile(t, depPath), "dependency, v1.", "dependency, v2.", 1)
	if err := os.WriteFile(depPath, []byte(dep), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	// The in-place rewrite of a LOCKED claim above stands in for the real
	// approval path (unlock -> edit -> lock), which records the new content on
	// the ledger. Re-record it, so the lock-ledger gate sees an approved edit
	// rather than tampering and this test keeps measuring dependency drift.
	armLedger(t, root)
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
	confirmOut, stderr, confirmCode := run(t, root, "--config", cfgPath, "claim", "reaudit", "widget.contract.main", "--confirm", "--reason", "test fixture")
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

	staleOut, _, _ := run(t, root, "--config", cfgPath, "claim", "list", "--review-pending")
	if !strings.Contains(staleOut, "widget.contract.main") {
		t.Fatalf("expected main still stale (open thread), got: %s", staleOut)
	}

	// Resolving the thread now clears review_pending (drift already confirmed).
	resolveThread(t, cfgPath, "widget.contract.main", tid)
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

	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock: %s", stderr)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "revisit"); code != 0 {
		t.Fatalf("comment add: %s", stderr)
	}

	out, stderr, code := run(t, root, "--config", cfgPath, "check")
	if code != 0 {
		t.Fatalf("expected check to exit 0 on a locked+open-thread project, got %d\nstdout: %s\nstderr: %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "catalog", "catalog.json")); err != nil {
		t.Fatalf("expected build/catalog/catalog.json written by check: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "viewer", "index.html")); err != nil {
		t.Fatalf("expected build/viewer/index.html written by check: %v", err)
	}
	if !strings.Contains(out, `open comments: module "widget": 1`) {
		t.Fatalf("expected an open-comments summary line, got: %s", out)
	}
	// The hint must point at the HUMAN's surface, not at a resolve command.
	// "comment resolve" left the CLI in v0.3.0 — resolving a thread is the
	// approval itself, so the agent is not the rights holder and check must not
	// hand it an invocation. Asserting the viewer is named (and that no resolve
	// invocation is) is the positive form of that rule.
	if !strings.Contains(out, "the human resolves them in the viewer") {
		t.Fatalf("expected the open-thread next step to point at the viewer, got: %s", out)
	}
	if strings.Contains(out, "dossierx comment resolve") {
		t.Fatalf("check advised the retired \"comment resolve\" verb, which the CLI no longer has: %s", out)
	}
	// Matched WITHOUT the "dossierx " prefix on purpose: the verb moved under
	// the claim noun, so a check for "dossierx reaudit" would silently stop
	// catching the hint it exists to forbid.
	if strings.Contains(out, "reaudit") {
		t.Fatalf("did NOT expect a reaudit hint for a comment-only pending claim, got: %s", out)
	}
}

// ---------------------------------------------------------------------
// comment list: pinned greppable line format, and the envelope shape.
// ---------------------------------------------------------------------

func TestComment_ListFormatAndEnvelope(t *testing.T) {
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

	// The machine surface. Before v0.3.0 this command had its own "--json"
	// flag emitting a BARE ARRAY on stdout with a prose hint on stderr — a
	// second, differently-shaped JSON surface on one binary. It is one
	// envelope now, like every other command, and stdout carries nothing else.
	jsonOut, jsonErr, jsonCode := run(t, root, "--config", cfgPath, "--format", "json", "comment", "list", "widget.contract.main")
	if jsonCode != 0 {
		t.Fatalf("comment list --format json exited %d (stderr: %s)", jsonCode, jsonErr)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			ClaimID string           `json:"claim_id"`
			Count   int              `json:"count"`
			Threads []map[string]any `json:"threads"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &env); err != nil {
		t.Fatalf("comment list stdout is not a single envelope: %v\nstdout: %s", err, jsonOut)
	}
	if !env.OK || env.Data.ClaimID != "widget.contract.main" {
		t.Fatalf("envelope drift: %s", jsonOut)
	}
	if env.Data.Count != 1 || len(env.Data.Threads) != 1 {
		t.Fatalf("expected exactly one thread in the envelope, got %s", jsonOut)
	}
	if strings.Contains(jsonOut, "comment list:") {
		t.Fatalf("expected stdout to be the envelope alone, with no prose mixed in: %s", jsonOut)
	}

	// --open filters to unresolved threads only.
	resolveThread(t, cfgPath, "widget.contract.main", tid)
	openOut, _, _ := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.main", "--open")
	if strings.Contains(openOut, tid) {
		t.Fatalf("expected --open to hide the now-resolved thread, got: %s", openOut)
	}
}

// ---------------------------------------------------------------------
// The four retired verbs: gone from the CLI, unchanged in the product.
//
// This test used to drive resolve/reopen/edit/delete through argv. v0.3.0
// removed all four from the CLI, so what has to be proved is now two things at
// once, and BOTH matter: the CLI really does refuse them (a half-removed verb
// that still works from the terminal would make the whole "review history the
// agent cannot rewrite" claim false), and the operations themselves — the ones
// the viewer drives over serve's HTTP API — still work exactly as before.
// ---------------------------------------------------------------------

func TestComment_RetiredVerbsAreGoneFromTheCLI(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "a claim."})

	// The group's own help IS the surface an agent discovers: cobra lists every
	// subcommand it has, so a verb absent from this listing is a verb the binary
	// does not offer. (Asserting on an invocation instead would be weaker — a
	// parent command with no Run prints help and exits 0 for an unrecognized
	// subcommand, which looks like success.)
	help, _, code := run(t, root, "--config", cfgPath, "comment", "--help")
	if code != 0 {
		t.Fatalf("comment --help exited %d", code)
	}
	for _, verb := range []string{"resolve", "reopen", "edit", "delete"} {
		if strings.Contains(help, "\n  "+verb+" ") {
			t.Fatalf("comment %s must not be offered by the CLI any more, help says:\n%s", verb, help)
		}
	}
	for _, verb := range []string{"inbox", "list", "add", "reply"} {
		if !strings.Contains(help, "\n  "+verb+" ") {
			t.Fatalf("comment %s must still be offered, help says:\n%s", verb, help)
		}
	}
}

func TestComment_RetiredVerbsStillWorkThroughTheirPackage(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "a claim."})

	addOut, _, addCode := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "original")
	if addCode != 0 {
		t.Fatalf("comment add: %s", addOut)
	}
	tid := firstToken(t, addOut, "c-")

	deps := commentDeps(t, cfgPath)

	// Resolve then reopen (both human, matching the human-opened thread's rights).
	if _, err := deps.Resolve("widget.contract.main", tid, model.CommentRoleHuman); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := deps.Reopen("widget.contract.main", tid, model.CommentRoleHuman); err != nil {
		t.Fatalf("reopen: %v", err)
	}

	// Edit the thread root body. The observable result is asserted through the
	// surviving CLI verb, so this still proves the write reached disk.
	if _, err := deps.Edit("widget.contract.main", tid, "", model.CommentRoleHuman, "edited-root-body"); err != nil {
		t.Fatalf("edit root: %v", err)
	}
	if listOut, _, _ := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.main"); !strings.Contains(listOut, "edited-root-body") {
		t.Fatalf("expected the edited body in list, got: %s", listOut)
	}

	// Reply (still a CLI verb), then edit and delete that reply by id.
	replyOut, _, replyCode := run(t, root, "--config", cfgPath, "comment", "reply", "widget.contract.main", tid, "--as", "agent", "--body", "a reply")
	if replyCode != 0 {
		t.Fatalf("comment reply: %s", replyOut)
	}
	rid := firstToken(t, replyOut, "r-")
	if _, err := deps.Edit("widget.contract.main", tid, rid, model.CommentRoleAgent, "edited reply"); err != nil {
		t.Fatalf("edit reply: %v", err)
	}
	if _, err := deps.Delete("widget.contract.main", tid, rid, model.CommentRoleAgent); err != nil {
		t.Fatalf("delete reply: %v", err)
	}

	// Delete the whole thread -> list is empty.
	if _, err := deps.Delete("widget.contract.main", tid, "", model.CommentRoleHuman); err != nil {
		t.Fatalf("delete thread: %v", err)
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

// ---------------------------------------------------------------------
// A store-bricking body whose FIRST line is real CONTENT that begins with a TAB
// ("\tcode\nmore") — the class the old leading-whitespace heuristic MISSED —
// must be refused at the CLI with the
// FRIENDLY, actionable ErrUnsafeBody guidance (exit non-zero), NEVER the cryptic
// internal "did not round-trip byte-exact" text, and must never brick the store.
// A body the old heuristic FALSE-REJECTED but that round-trips fine (" \ncontent")
// must now be ACCEPTED.
// ---------------------------------------------------------------------

func TestComment_UnsafeBody_FriendlyErrorNotCryptic(t *testing.T) {
	unsafe := []struct {
		name string
		body string
	}{
		{"tab-led-content-line", "\tcode line\nmore"},
		{"tab-led-multiline", "\tone\n\ttwo"},
	}
	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
			claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.a", facet: "contract", module: "widget", status: "draft", body: "claim A."})
			before := llReadFile(t, claimPath)

			out, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.a", "--as", "human", "--body", tc.body)
			if code == 0 {
				t.Fatalf("expected a non-zero exit for an unsafe body, got 0 (stdout: %s)", out)
			}
			msg := stderr + out
			// FRIENDLY + actionable: the guidance names how to fix it. It stopped
			// saying "de-indent" in v0.4.0, because a space-indented first line
			// now stores fine and only a tab-led one does not — sending the caller
			// to de-indent would point them at something that is not broken.
			if !strings.Contains(msg, "not a blank line or a tab") || !strings.Contains(msg, "stored as YAML") {
				t.Fatalf("expected the friendly unsafe-body guidance, got stdout: %s stderr: %s", out, stderr)
			}
			// NEVER the cryptic internal round-trip text.
			for _, cryptic := range []string{"round-trip", "byte-exact", "store-bricking", "block scalar", "did not re-parse"} {
				if strings.Contains(msg, cryptic) {
					t.Fatalf("unsafe-body CLI error leaked the internal detail %q: stdout: %s stderr: %s", cryptic, out, stderr)
				}
			}
			// Never bricked: the claim file is byte-unchanged and the dir still lists.
			if llReadFile(t, claimPath) != before {
				t.Fatalf("the claim file changed after a refused unsafe-body add")
			}
			if _, listErr, listCode := run(t, root, "--config", cfgPath, "comment", "list", "widget.contract.a"); listCode != 0 {
				t.Fatalf("claims dir unusable (possibly bricked) after a refused unsafe-body add: %s", listErr)
			}
		})
	}

	// A body the OLD heuristic false-rejected but that round-trips fine is now
	// accepted end to end.
	t.Run("false-reject-now-accepted", func(t *testing.T) {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		llWriteClaim(t, root, llClaimSpec{id: "widget.contract.a", facet: "contract", module: "widget", status: "draft", body: "claim A."})

		out, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.a", "--as", "human", "--body", " \ncontent below a blank first line")
		if code != 0 {
			t.Fatalf("expected a body that round-trips (\" \\ncontent\") to be ACCEPTED, got exit %d: stdout: %s stderr: %s", code, out, stderr)
		}
	})
}

// writePoisonStoredBodyClaim hand-authors a claim whose STORED prose body is a
// tab-led first content line — a shape yaml.v3 v3.0.1 emits as a block scalar it
// cannot re-parse, so the WHOLE claim will not round-trip on the next save. The
// double-quoted flow scalar on disk loads cleanly (so `dossierx check`/list pass,
// exactly the state a user reaches by hand-editing a claim YAML), yet ANY comment
// op that re-saves the claim trips the loader's store-bricking guard. It carries
// one open thread (c-poison1) and one resolved thread (c-poison2) so every
// mutating op has a valid target, and the poison lives in the CLAIM body (not
// a thread body) so even a whole-thread delete still re-saves the bad bytes.
func writePoisonStoredBodyClaim(t *testing.T, root, id, module string) string {
	t.Helper()
	body := "id: " + id + "\n" +
		"facet: contract\nmodule: " + module + "\nstatus: draft\nlayout: card\n" +
		"body: \"\\tstored prose yaml cannot round-trip\\nsecond line\"\n" +
		"governed_by:\n  type: none\n  reason: fixture poison claim\n" +
		"comments:\n" +
		"  - id: c-poison1\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: open thread on a poison claim\n    edited: false\n" +
		"  - id: c-poison2\n    status: resolved\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: resolved thread on a poison claim\n    edited: false\n    resolved_by: human\n    resolved_at: \"2026-07-24T11:00:00Z\"\n"
	path := filepath.Join(root, "claims", lastSegment(id)+".yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write poison claim: %v", err)
	}
	return path
}

// A claim whose PRE-EXISTING stored body will not round-trip (a hand-edited
// tab-led body, a state `dossierx check` passes clean) must NOT print the
// supplied-body de-indent guidance — the caller's supplied body, if any, is fine.
// Every mutating CLI verb must instead print a DISTINCT, claim-SCOPED message
// that names the offending claim, exit non-zero, and NEVER leak the raw internal
// yaml / round-trip / store-bricking text. This pins EM-02 (the blanket
// ErrUnsafeBody misattribution).
//
// The four retired verbs (resolve/reopen/edit/delete) are covered by the
// companion below, which asserts the same distinction at the layer they still
// live at — the sentinel their surface now translates is the same one, and
// internal/serve's writeOpError is what translates it.
func TestComment_StoredBodyNotRoundTrippable_ClaimScopedError(t *testing.T) {
	const claimID = "widget.contract.poison"
	verbs := []struct {
		name string
		args []string
	}{
		{"add", []string{"comment", "add", claimID, "--as", "human", "--body", "a perfectly fine new thread"}},
		{"reply", []string{"comment", "reply", claimID, "c-poison1", "--as", "human", "--body", "a perfectly fine reply"}},
	}
	for _, v := range verbs {
		t.Run(v.name, func(t *testing.T) {
			root := t.TempDir()
			cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
			claimPath := writePoisonStoredBodyClaim(t, root, claimID, "widget")
			before := llReadFile(t, claimPath)

			// Sanity: the poison file loads cleanly (list works) — the exact state a
			// user reaches by hand-editing a claim YAML; the failure is at SAVE time.
			if _, e, c := run(t, root, "--config", cfgPath, "comment", "list", claimID); c != 0 {
				t.Fatalf("precondition: poison claim should LOAD clean (list), got exit %d: %s", c, e)
			}

			args := append([]string{"--config", cfgPath}, v.args...)
			out, stderr, code := run(t, root, args...)
			msg := stderr + out
			if code == 0 {
				t.Fatalf("%s: expected a non-zero exit on a non-round-trippable stored body, got 0 (stdout: %s)", v.name, out)
			}
			// DISTINCT, claim-SCOPED message that NAMES the claim and points at its
			// STORED body — not the supplied body.
			if !strings.Contains(msg, claimID) {
				t.Fatalf("%s: error must name the offending claim %q, got stdout: %s stderr: %s", v.name, claimID, out, stderr)
			}
			if !strings.Contains(msg, "stored body") {
				t.Fatalf("%s: error must point at the STORED body, got stdout: %s stderr: %s", v.name, out, stderr)
			}
			// NOT the supplied-body de-indent guidance (the caller's body is fine here).
			if strings.Contains(msg, "de-indent") {
				t.Fatalf("%s: must not print the supplied-body de-indent guidance for a stored-body failure, got: %s", v.name, msg)
			}
			// NEVER the raw internal loader/yaml/round-trip text.
			for _, cryptic := range []string{"round-trip", "byte-exact", "store-bricking", "block scalar", "did not re-parse", "loader:", "yaml:"} {
				if strings.Contains(msg, cryptic) {
					t.Fatalf("%s: leaked internal detail %q: stdout: %s stderr: %s", v.name, cryptic, out, stderr)
				}
			}
			// Never bricked: the file is byte-unchanged and the dir still lists.
			if llReadFile(t, claimPath) != before {
				t.Fatalf("%s: the poison claim file changed after a refused op", v.name)
			}
			if _, e, c := run(t, root, "--config", cfgPath, "comment", "list", claimID); c != 0 {
				t.Fatalf("%s: claims dir unusable after a refused op: %s", v.name, e)
			}
		})
	}
}

// TestComment_StoredBodyNotRoundTrippable_RetiredVerbsRaiseTheSameSentinel is
// the other half of the one above.
//
// The CLI-friendly translation of "this claim's STORED bytes cannot be
// re-serialized" is per-surface: cmd/dossierx's friendlyCommentBodyErr does it
// for add/reply, internal/serve's writeOpError does it for the four verbs the
// viewer owns. What both translations rest on is that the OPERATION raises
// loader.ErrClaimNotRoundTrippable and NOT comments.ErrUnsafeBody — confusing
// the two is what would put "de-indent your input" in front of a caller whose
// input was perfect. That distinction is asserted here, at the source.
func TestComment_StoredBodyNotRoundTrippable_RetiredVerbsRaiseTheSameSentinel(t *testing.T) {
	const claimID = "widget.contract.poison"
	ops := []struct {
		name string
		call func(*comments.Deps) error
	}{
		{"resolve", func(d *comments.Deps) error {
			_, err := d.Resolve(claimID, "c-poison1", model.CommentRoleHuman)
			return err
		}},
		{"reopen", func(d *comments.Deps) error {
			_, err := d.Reopen(claimID, "c-poison2", model.CommentRoleHuman)
			return err
		}},
		{"edit", func(d *comments.Deps) error {
			_, err := d.Edit(claimID, "c-poison1", "", model.CommentRoleHuman, "a perfectly fine edit")
			return err
		}},
		{"delete", func(d *comments.Deps) error {
			_, err := d.Delete(claimID, "c-poison1", "", model.CommentRoleHuman)
			return err
		}},
	}
	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			root := t.TempDir()
			cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
			claimPath := writePoisonStoredBodyClaim(t, root, claimID, "widget")
			before := llReadFile(t, claimPath)

			err := op.call(commentDeps(t, cfgPath))
			if err == nil {
				t.Fatalf("%s: expected a refusal on a non-round-trippable stored body", op.name)
			}
			if !errors.Is(err, loader.ErrClaimNotRoundTrippable) {
				t.Fatalf("%s: expected ErrClaimNotRoundTrippable, got: %v", op.name, err)
			}
			// The caller's input is fine, so this must NOT be the supplied-body
			// sentinel — that is exactly the misattribution EM-02 fixed.
			if errors.Is(err, comments.ErrUnsafeBody) {
				t.Fatalf("%s: a STORED-body failure must not surface as ErrUnsafeBody, got: %v", op.name, err)
			}
			// Never bricked.
			if llReadFile(t, claimPath) != before {
				t.Fatalf("%s: the poison claim file changed after a refused op", op.name)
			}
		})
	}
}
