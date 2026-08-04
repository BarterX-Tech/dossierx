// ledger_test.go covers the lock-ledger gate as the check pipeline runs it:
// which conditions fail a run, which ones only get reported, and — the property
// the whole "not a lint" argument rests on — that a failing gate still leaves a
// regenerated catalog and viewer behind.
package check_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// armLedger records a lock-ledger approval for every LOCKED claim, exactly as
// "dossierx claim lock" would have, and writes the store to the path check
// reads it from. It is the fixture equivalent of "a human approved these".
func armLedger(t *testing.T, cfg *config.Config, claims []model.Claim) {
	t.Helper()
	path := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(path)
	if err != nil {
		t.Fatalf("arm ledger: load store: %v", err)
	}
	armed := false
	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		lock.RecordApproval(store, c, lock.Approval{Actor: "fixture", Reason: "fixture approval"})
		armed = true
	}
	if !armed {
		// Nothing to approve. Writing an empty store anyway would be wrong: a
		// project with no locked claims and no store file is the ordinary state
		// of a project that has never locked anything, and the gate must pass
		// on it without any fixture help at all.
		return
	}
	if err := store.Save(); err != nil {
		t.Fatalf("arm ledger: save store: %v", err)
	}
}

// armDigests records the comment digest for every claim, as the engine does on
// its first comment write. Without it the comment rules are inert (unknown, not
// drifted), which is the correct default and is asserted separately below.
func armDigests(t *testing.T, cfg *config.Config, claims []model.Claim) {
	t.Helper()
	store, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		t.Fatalf("arm digests: load: %v", err)
	}
	digest.Adopt(store, claims)
	if err := store.Save(); err != nil {
		t.Fatalf("arm digests: save: %v", err)
	}
}

// rulesOf collapses findings to their rule names, for assertions that care
// about which rules fired rather than about the exact prose.
func rulesOf(findings []lock.Finding) []string {
	names := make([]string, 0, len(findings))
	for _, f := range findings {
		names = append(names, f.Rule)
	}
	return names
}

func hasRule(findings []lock.Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// A locked claim with no approval record fails the run — AND the catalog and
// the viewer are on disk when it does.
//
// That second half is the entire reason these rules are a gate rather than a
// lint. Registered as a lint they would fail at step 1, before either write,
// so one tampered YAML file would take a project's documentation offline for
// every reader. Here the reader still gets a current viewer to go look at the
// disputed claim in; what the project loses is the exit status.
func TestRun_LedgerMissingFailsButStillRegeneratesTheViewer(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	// Undo the fixture's arming: this test is the hand-flipped case.
	if err := os.Remove(filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")); err != nil {
		t.Fatalf("remove store: %v", err)
	}

	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected the ledger gate to fail the run, got nil (findings=%v)", rulesOf(res.LedgerFindings))
	}
	if err.Error() != "ledger: 2 integrity finding(s)" {
		t.Fatalf("ledger error text drift: %q", err.Error())
	}
	if res.OK {
		t.Fatalf("expected OK=false when the gate fires")
	}
	if !hasRule(res.LedgerFindings, lock.RuleLockLedgerMissing) || !hasRule(res.LedgerFindings, lock.RuleLockLedgerAbsent) {
		t.Fatalf("expected lock-ledger-missing AND lock-ledger-absent, got %v", rulesOf(res.LedgerFindings))
	}

	if res.CatalogPath == "" || res.RenderPath == "" {
		t.Fatalf("the gate must run LAST: expected catalog+render paths recorded, got %q / %q", res.CatalogPath, res.RenderPath)
	}
	for _, p := range []string{res.CatalogPath, res.RenderPath} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("a ledger failure must not take the viewer offline: %s missing (%v)", p, statErr)
		}
	}
}

// Editing a locked claim's body after it was approved is lock-content-drift.
func TestRun_LedgerContentDrift(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})

	// The claims slice is what Run judges; mutate the in-memory copy, which is
	// exactly what a hand-edited file would have produced at load time.
	claims[0].Body = "a locked claim, quietly rewritten.\n"

	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected the gate to catch the edit, got nil")
	}
	if !hasRule(res.LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("expected lock-content-drift, got %v", rulesOf(res.LedgerFindings))
	}
}

// The headline finding of the audit, asserted through the pipeline: swapping
// raw_html on a locked, reviewed, allowlisted mockup — the only path in this
// codebase that renders author bytes unescaped — is caught as lock-content
// drift.
//
// WHAT CATCHES IT is LockedClaimHash, which signs every persisted field, and
// that is independent of whatever ContentHash happens to cover: the ledger
// would report this swap even if no other hash in the engine could see it.
// This test is not about ContentHash. The comparison below is only a
// PRECONDITION proving the fixture's swap is a real edit to a hashed field. It
// used to assert the opposite — ContentHash was blind to raw_html, which was
// the most convenient way to say "nothing else would catch this" — but v0.4.1
// put raw_html in ContentHash's allowlist (raw_html became legal on any layout,
// so a dependent can now rest on a claim that carries one, and an edited
// payload has to mark it stale). So the precondition is now that the hash DOES
// move; the assertion that matters, the ledger finding, is unchanged.
func TestRun_LedgerCatchesSwappedRawHTML(t *testing.T) {
	cfgBody := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n" +
		"mockup_modules:\n  - widget\n"
	cfg, claims := project(t, cfgBody, map[string]string{
		"claims/mock.yaml": "id: widget.contract.mock\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: mockup\n" +
			"raw_html: '<div class=\"gcp-row\">approved markup</div>'\nraw_html_reviewed: true\n" +
			"body: |\n  a locked mockup.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	if got := lock.ContentHash(claims[0]); got == lock.ContentHash(swapRawHTML(claims[0])) {
		t.Fatalf("precondition failed: since v0.4.1 ContentHash covers raw_html, but it did not move (got %s for both)", got)
	}

	claims[0] = swapRawHTML(claims[0])
	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected a swapped raw_html payload to be caught, got nil")
	}
	if !hasRule(res.LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("expected lock-content-drift for the swapped raw_html, got %v", rulesOf(res.LedgerFindings))
	}
}

func swapRawHTML(c model.Claim) model.Claim {
	c.RawHTML = `<div class="gcp-row">substituted markup</div>`
	return c
}

// A claim flipped back to draft while still holding an unreleased record is
// lock-ledger-orphan: the cheapest way to dodge review is to stop being locked.
func TestRun_LedgerOrphan(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	claims[0].Status = model.StatusDraft

	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected lock-ledger-orphan to fail the run, got nil")
	}
	if !hasRule(res.LedgerFindings, lock.RuleLockLedgerOrphan) {
		t.Fatalf("expected lock-ledger-orphan, got %v", rulesOf(res.LedgerFindings))
	}
}

// A comment thread deleted out of the YAML is comment-ledger-drift — and it is
// caught on a DRAFT claim, because an unlocked claim's review history matters
// just as much.
func TestRun_CommentLedgerDrift(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/draft.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n",
	})
	armDigests(t, cfg, claims)

	claims[0].Comments = nil // the thread edited away by hand
	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected comment-ledger-drift to fail the run, got nil")
	}
	if !hasRule(res.LedgerFindings, lock.RuleCommentLedgerDrift) {
		t.Fatalf("expected comment-ledger-drift, got %v", rulesOf(res.LedgerFindings))
	}
}

// With NO digest store at all, comment drift is UNKNOWN, never reported. An
// integrity check that manufactures findings out of missing evidence is one
// people learn to ignore, and every project predating the digest store would
// otherwise light up on upgrade day.
func TestRun_NoDigestStoreMeansUnknownNotDrifted(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/draft.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n",
	})
	// Undo the fixture's arming: this test is about a project that has NEVER had
	// a digest store. (The claim here is a DRAFT, so nothing armed the lock
	// ledger either — which is what keeps comment-digest-absent out of it too:
	// that rule fires only once a project is ledger-covered.)
	if err := os.Remove(digest.StorePath(cfg)); err != nil {
		t.Fatalf("remove digest store: %v", err)
	}
	claims[0].Comments = nil

	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("expected no findings without a digest store, got %v (%v)", err, rulesOf(res.LedgerFindings))
	}
}

// A ledger that exists but does not parse must be LOUDER than one that is
// missing, never quieter: the gate names the unreadable store and then fails
// closed, reporting every locked claim as unapproved.
func TestRun_UnreadableLedgerFailsClosed(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	if err := os.WriteFile(filepath.Join(cfg.Dir(), ".dossierx-lock-store.json"), []byte("{ truncated"), 0o644); err != nil {
		t.Fatalf("corrupt store: %v", err)
	}

	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected a corrupt ledger to fail the run, got nil")
	}
	if !hasRule(res.LedgerFindings, check.RuleLedgerUnreadable) {
		t.Fatalf("expected lock-ledger-unreadable, got %v", rulesOf(res.LedgerFindings))
	}
	if !hasRule(res.LedgerFindings, lock.RuleLockLedgerMissing) {
		t.Fatalf("a gate that cannot read its evidence must fail closed: expected every locked claim reported unapproved, got %v", rulesOf(res.LedgerFindings))
	}
}

// A corrupt COMMENT DIGEST store must not accuse the locked claims of anything.
// Its blast radius is the comment rules and nothing else — folding the two
// store failures together would file a false report about content approval,
// and a gate that files false reports gets bypassed.
func TestRun_UnreadableDigestStoreDoesNotAccuseLockedClaims(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	if err := os.WriteFile(filepath.Join(cfg.Dir(), ".dossierx-comment-digest.json"), []byte("{ truncated"), 0o644); err != nil {
		t.Fatalf("corrupt digest store: %v", err)
	}

	res, _ := check.Run(claims, cfg) //nolint:errcheck // the run is expected to fail; Result is what this asserts on
	if !hasRule(res.LedgerFindings, check.RuleLedgerUnreadable) {
		t.Fatalf("expected lock-ledger-unreadable for the digest store, got %v", rulesOf(res.LedgerFindings))
	}
	for _, f := range res.LedgerFindings {
		if f.Rule == lock.RuleLockLedgerMissing || f.Rule == lock.RuleLockContentDrift {
			t.Fatalf("a corrupt digest store must not accuse a locked claim's CONTENT: got %s on %s", f.Rule, f.ClaimID)
		}
	}
	if !strings.Contains(res.LedgerFindings[0].Message, "comment digest store") {
		t.Fatalf("the finding must name WHICH store failed: %q", res.LedgerFindings[0].Message)
	}
}

// Status REPORTS the gate; it does not refuse. That asymmetry is what lets one
// function serve both "dossierx serve"'s status strip — which has to keep
// rendering a disputed project so a human can go read the disputed claim — and
// the enforcing read-only CLI paths, which read the same field and decide.
func TestStatus_ReportsLedgerFindingsWithoutFailing(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	claims[0].Body = "quietly rewritten.\n"

	res := check.Status(claims, cfg)
	if !hasRule(res.LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("expected Status to REPORT the drift, got %v", rulesOf(res.LedgerFindings))
	}
	if !res.OK {
		t.Fatalf("expected Status's OK to stay lint-driven so the status strip keeps rendering")
	}
	if len(res.NextSteps) == 0 && len(res.OpenComments) == 0 {
		// Not an assertion about content, only that the reporting tail ran at
		// all — the gate must not have short-circuited it.
		t.Logf("no reporting produced for this fixture; acceptable, recorded for context")
	}
}

// Status must not write. It is the seam both --validate and --staged drive, and
// both promise it.
func TestStatus_LedgerGateIsReadOnly(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	before := treeSnapshot(t, cfg.Dir())
	_ = check.Status(claims, cfg)
	after := treeSnapshot(t, cfg.Dir())
	if before != after {
		t.Fatalf("check.Status wrote to the project tree:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// treeSnapshot renders every file under root as "relpath size" lines, sorted —
// enough to catch a created, deleted or resized file.
func treeSnapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		b.WriteString(rel)
		b.WriteString(" ")
		b.WriteString(strings.TrimSpace(strings.Join([]string{itoa(info.Size())}, "")))
		b.WriteString("\n")
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return b.String()
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// A lint error must not SILENCE the ledger gate.
//
// The two partitions answer different questions and have different recoveries —
// a lint finding is fixed by editing the claim, a ledger finding by unlock ->
// fix -> lock or by restoring the ledger — so an agent told "lint_failed,
// data.ledger_findings: []" concludes the integrity gate ran and passed. It had
// not run at all: the fail-fast returned before it. The commit in this test is
// exactly the shape that made it matter — one hand-edited locked claim and one
// unrelated typo'd rests_on in a still-draft claim, staged together.
func TestLintErrorStillReportsTheLedgerGate(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
		"claims/draft.yaml":  draftClaim("widget.contract.draft") + "rests_on:\n  - widget.contract.nope\n",
	})

	// The hand edit to the LOCKED claim, after the fixture armed its approval.
	for i := range claims {
		if claims[i].ID == "widget.contract.locked" {
			claims[i].Body = "a locked claim, quietly rewritten.\n"
		}
	}

	for _, tc := range []struct {
		name string
		res  check.Result
	}{
		{"Status", check.Status(claims, cfg)},
		{"Run", runResult(t, claims, cfg)},
	} {
		if len(tc.res.LintErrors) == 0 {
			t.Fatalf("%s: precondition: expected the dangling rests_on to be an error-severity lint finding", tc.name)
		}
		if !hasRule(tc.res.LedgerFindings, lock.RuleLockContentDrift) {
			t.Fatalf("%s: a lint error suppressed the ledger gate: ledger_findings=%v", tc.name, rulesOf(tc.res.LedgerFindings))
		}
	}
}

// runResult drives check.Run and returns its Result, ignoring the (expected)
// fail-fast error — the assertion above is about what the Result CARRIES when
// the run stops early, not about the error.
func runResult(t *testing.T, claims []model.Claim, cfg *config.Config) check.Result {
	t.Helper()
	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected Run to stop at the lint step")
	}
	return res
}

// Deleting the digest store used to launder a comment tamper permanently.
//
// The sequence was: hand-delete an unresolved thread (comment-ledger-drift fires,
// correctly), then delete .dossierx-comment-digest.json. Every claim goes back to
// "unknown", which is never "drifted", so the finding vanished before any command
// ran — and the unresolved review the human had left was gone with it, no gate,
// no trace but the deleted file in the diff. This is the same guard the lock
// ledger has always had as lock-ledger-absent.
func TestRun_DeletingTheDigestStoreIsReported(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
		"claims/commented.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n",
	})
	// The fixture armed both stores, so this project IS ledger-covered — the
	// state every project is in after one run of a ledger-aware build.
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatalf("precondition: expected a clean run before the deletion, got %v", err)
	}

	if err := os.Remove(digest.StorePath(cfg)); err != nil {
		t.Fatalf("remove digest store: %v", err)
	}

	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected the deleted digest store to fail the run, got nil")
	}
	if !hasRule(res.LedgerFindings, check.RuleCommentDigestAbsent) {
		t.Fatalf("expected %s, got %v", check.RuleCommentDigestAbsent, rulesOf(res.LedgerFindings))
	}
}

// downgradeLockStore rewrites cfg's lock store the way v0.2.x left it — the
// schema version back at the pre-ledger number, no ledger key — and optionally
// removes the comment digest store beside it.
//
// The two flavours are the whole point of the fixture. WITHOUT the digest store
// this is exactly the shape of a genuine project upgrading from v0.2.x (that
// file did not exist before v0.3.0). WITH it, the same bytes are a project that
// has demonstrably already been through a ledger-aware build and had its store
// hand-edited back. The gate has to pass the first and refuse the second, and
// the only thing separating them is the sibling file.
func downgradeLockStore(t *testing.T, cfg *config.Config, keepDigestStore bool) {
	t.Helper()
	path := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse lock store: %v", err)
	}
	doc["version"] = 1
	delete(doc, "ledger")
	edited, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal downgraded store: %v", err)
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("write downgraded store: %v", err)
	}
	if !keepDigestStore {
		if err := os.Remove(digest.StorePath(cfg)); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove digest store: %v", err)
		}
	}
}

// A pre-ledger (v0.2.x) project is refused ONCE, by name, and told which command
// clears it — it is not accused per-claim, and it is not waved through.
//
// This test asserted the opposite until v0.3.0, and the reason it changed is the
// whole of the fail-closed decision. Grandfathering used to happen by itself: the
// read-only gate exempted a store whose own version field said it predated the
// ledger, and the next writing command adopted whatever was on disk. Review
// established that NO evidence inside the project directory can tell an honest
// v0.2.x store from a downgraded one — locked_at shipped in v0.2.0, the version
// field lives in the file being audited, and deleting the comment digest store
// alongside the ledger reproduces the honest shape byte for byte. So the
// exemption was, in the same code path, the bypass.
//
// What replaces it is one PROJECT-SCOPED finding rather than an accusation
// against each locked claim. That distinction is the part worth keeping: the old
// failure mode was `lock-ledger-missing` fired once per locked claim with
// recovery text telling the human to set their claims back to draft and re-lock
// them — a gate firing on correct state with destructive advice attached, at the
// exact moment a project upgrades. lock-ledger-pre-ledger says the true thing
// instead ("this project's locks predate the lock ledger") and names the ordered
// crossing that clears it, which is why the next-step assertion below is as
// load-bearing as the finding assertion.
func TestStatus_PreLedgerProjectIsRefusedOnceByNameNotAccusedPerClaim(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	lockBuildOrder(t, cfg, claims, "widget")
	downgradeLockStore(t, cfg, false)

	res := check.Status(claims, cfg)
	if got := rulesOf(res.LedgerFindings); len(got) != 1 || got[0] != lock.RuleLockLedgerPreLedger {
		t.Fatalf("a pre-ledger project must be refused exactly once, by %s, and never accused per-claim; got %v",
			lock.RuleLockLedgerPreLedger, got)
	}

	// The finding fails the gate; the next step is what tells the reader how to
	// clear it. Naming the crossing is the assertion — an agent that is refused
	// without being told to unlock loops on a gate it cannot pass.
	found := false
	for _, h := range res.NextSteps {
		if strings.Contains(h, "predate the lock ledger") && strings.Contains(h, "unlock every locked claim") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the crossing must be reported as a next step naming unlock, got %#v", res.NextSteps)
	}
}

// THE STATE THE CLAIMS-ONLY EMITTER CANNOT SEE: a pre-ledger project holding a
// LOCKED BUILD ORDER and ZERO locked claims.
//
// It is reachable, and it was silent. `claim unlock` never touches the
// build-order artifact and internal/buildorder never clears Locked on unlock, so
// lock a module, lock its order, then unlock every claim. In that state
// lock.Audit's claims-only term is zero and buildOrderGate suppresses
// build-order-ledger-missing under the pre-ledger exemption — while BOTH write
// paths refuse with pre_ledger_unadopted. A refusal with no finding naming it,
// and no recovery text reachable from `check`, is exactly what the project-scoped
// rule exists to prevent.
//
// So: exactly ONE lock-ledger-pre-ledger (not one per module, and not two from
// the two emitters), and still zero build-order-ledger-missing.
func TestStatus_PreLedgerProjectWithOnlyALockedBuildOrderIsStillReported(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	lockBuildOrder(t, cfg, claims, "widget")
	downgradeLockStore(t, cfg, false)

	// Unlock every claim, leaving the LOCKED artifact in place — the reachable
	// state described above.
	for i := range claims {
		claims[i].Status = model.StatusDraft
	}

	got := rulesOf(check.Status(claims, cfg).LedgerFindings)
	preLedger, missing := 0, 0
	for _, r := range got {
		switch r {
		case lock.RuleLockLedgerPreLedger:
			preLedger++
		case check.RuleBuildOrderLedgerMissing:
			missing++
		}
	}
	if preLedger != 1 {
		t.Fatalf("expected exactly one %s so check and the write path agree, got %d in %v", lock.RuleLockLedgerPreLedger, preLedger, got)
	}
	if missing != 0 {
		t.Fatalf("the pre-ledger exemption still covers the build order itself; got %d %s in %v", missing, check.RuleBuildOrderLedgerMissing, got)
	}
}

// The same bytes, with the sibling file that proves this project has already
// been through a ledger-aware build, are a downgrade — and the read-only path
// must not extend the pre-ledger exemption to them. Otherwise the fix for the
// honest project would hand the attacker a quieter version of the same bypass:
// edit one number, and check --staged reports nothing at all.
func TestStatus_DowngradedLockStoreIsRefusedNotGrandfathered(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	lockBuildOrder(t, cfg, claims, "widget")
	downgradeLockStore(t, cfg, true)

	res := check.Status(claims, cfg)
	for _, want := range []string{lock.RuleLockLedgerDowngraded, lock.RuleLockLedgerMissing, check.RuleBuildOrderLedgerMissing} {
		if !hasRule(res.LedgerFindings, want) {
			t.Fatalf("expected %s, got %v", want, rulesOf(res.LedgerFindings))
		}
	}
}

// The TOTAL launder: delete a claim's only comment thread AND the digest store
// in one commit.
//
// This is the case the rule used to miss, and it missed it for the worst
// possible reason — its own trigger was computed from the tampered state. The
// rule only fired when some claim still carried a thread, so removing the LAST
// one removed the evidence that any had ever existed, and the deleted store went
// unreported. Deleting more had to stop buying more silence: the partial launder
// was caught and the total one was free.
func TestRun_DeletingTheOnlyThreadAndTheDigestStoreIsStillReported(t *testing.T) {
	const commented = "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"comments:\n" +
		"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n"

	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml":    lockedClaim("widget.contract.locked"),
		"claims/commented.yaml": commented,
	})
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatalf("precondition: expected a clean run before the launder, got %v", err)
	}

	// The launder, in one commit: the thread goes out of the YAML, and the file
	// that would have reported the edit goes with it. Nothing else changes.
	if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "commented.yaml"), []byte(draftClaim("widget.contract.one")), 0o644); err != nil {
		t.Fatalf("delete the thread: %v", err)
	}
	if err := os.Remove(digest.StorePath(cfg)); err != nil {
		t.Fatalf("remove digest store: %v", err)
	}
	laundered, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("reload claims: %v", err)
	}
	for _, c := range laundered {
		if len(c.Comments) > 0 {
			t.Fatalf("precondition: the laundered project must carry no comment threads at all; %s still has %d", c.ID, len(c.Comments))
		}
	}

	res, err := check.Run(laundered, cfg)
	if err == nil {
		t.Fatalf("the total launder must not be quieter than the partial one, got a clean run")
	}
	if !hasRule(res.LedgerFindings, check.RuleCommentDigestAbsent) {
		t.Fatalf("expected %s, got %v", check.RuleCommentDigestAbsent, rulesOf(res.LedgerFindings))
	}
}

// The one qualifier that keeps the rule off correct state: a project that
// predates the ledger (its lock store is still at the older schema, or there is
// no lock store because nothing has ever been locked) is mid-upgrade, not
// tampered with.
func TestRun_DigestStoreAbsenceIsSilentWithoutLedgerCoverage(t *testing.T) {
	t.Run("not yet ledger-covered", func(t *testing.T) {
		cfg, claims := project(t, baseConfig, map[string]string{
			"claims/commented.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
				"body: |\n  a draft claim.\n" +
				"governed_by:\n  type: none\n  reason: fixture\n" +
				"comments:\n" +
				"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n",
		})
		// Nothing is locked, so there is no lock store: the shape of a project
		// that has not yet run a ledger-aware build.
		if err := os.Remove(digest.StorePath(cfg)); err != nil {
			t.Fatalf("remove digest store: %v", err)
		}
		res, err := check.Run(claims, cfg)
		if err != nil {
			t.Fatalf("a project that is not ledger-covered must not be accused: %v (%v)", err, rulesOf(res.LedgerFindings))
		}
	})
}
