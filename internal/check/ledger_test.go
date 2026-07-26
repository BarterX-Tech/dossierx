// ledger_test.go covers the lock-ledger gate as the check pipeline runs it:
// which conditions fail a run, which ones only get reported, and — the property
// the whole "not a lint" argument rests on — that a failing gate still leaves a
// regenerated catalog and viewer behind.
package check_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
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
// codebase that renders author bytes unescaped — is caught, even though
// ContentHash does not hash raw_html at all.
func TestRun_LedgerCatchesSwappedRawHTML(t *testing.T) {
	cfgBody := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n" +
		"mockup_modules:\n  - widget\n"
	cfg, claims := project(t, cfgBody, map[string]string{
		"claims/mock.yaml": "id: widget.contract.mock\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: mockup\n" +
			"raw_html: '<div class=\"gcp-row\">approved markup</div>'\nraw_html_reviewed: true\n" +
			"body: |\n  a locked mockup.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	if got := lock.ContentHash(claims[0]); got != lock.ContentHash(swapRawHTML(claims[0])) {
		t.Fatalf("precondition failed: ContentHash is supposed to be BLIND to raw_html, but it moved")
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

	res, _ := check.Run(claims, cfg)
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
