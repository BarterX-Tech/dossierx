// comment_digest_coverage_test.go pins the two rules that read the comment
// digest store's CONTENT rather than its existence.
//
// Both exist because the store was guarded against being DELETED and against
// nothing else, and both of the cheaper edits — emptying the map, and renaming
// the claim out from under its entry — were reproduced as complete launders of
// an unresolved human review.
package check_test

import (
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

// commentedLockedClaim is a locked claim carrying a resolved review thread — a
// claim that has been through the comment engine, so the digest store covers it.
func commentedLockedClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"body: |\n  a locked claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"comments:\n" +
		"  - id: c-8136dd\n    status: resolved\n    author: human\n" +
		"    created: \"2026-07-26T10:00:00Z\"\n    body: I do not agree with this yet.\n    edited: false\n" +
		"    resolved_by: human\n    resolved_at: \"2026-07-26T11:00:00Z\"\n"
}

// writeDigestStore replaces the digest store with exactly raw — the hand edit
// under test, not something the engine would ever write.
func writeDigestStore(t *testing.T, cfg *config.Config, raw string) {
	t.Helper()
	if err := os.WriteFile(digest.StorePath(cfg), []byte(raw), 0o644); err != nil {
		t.Fatalf("overwrite digest store: %v", err)
	}
}

// THE EMPTIED MAP. Deleting the digest store is caught (comment-digest-absent);
// overwriting it with {"version":1,"digests":{}} was not caught by anything, and
// it is strictly cheaper to hide in a review diff than the `rm` the rule does
// see. On the same tampered claim the measured asymmetry was: delete the file ->
// ok:false ['comment-digest-absent']; empty the map -> ok:true, [].
//
// The trigger is coverage, not file presence, and it is built only out of the
// LEDGER RECORD — which this edit does not touch.
func TestCommentDigest_EmptiedMapIsReportedPerApprovedClaim(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": commentedLockedClaim("widget.internals.fields"),
	})

	// Precondition: the honest project is silent.
	if res := check.Status(claims, cfg); len(res.LedgerFindings) != 0 {
		t.Fatalf("fixture precondition: an honest project must be silent, got %v", rulesOf(res.LedgerFindings))
	}

	writeDigestStore(t, cfg, `{"version":1,"digests":{}}`)

	res := check.Status(claims, cfg)
	if !hasRule(res.LedgerFindings, check.RuleCommentDigestMissing) {
		t.Fatalf("emptying the digest map must be reported, got %v", rulesOf(res.LedgerFindings))
	}
	// And it must name the claim, so the report says which approval is now
	// covering nothing rather than only that something is wrong.
	named := false
	for _, f := range res.LedgerFindings {
		if f.Rule == check.RuleCommentDigestMissing && f.ClaimID == "widget.internals.fields" {
			named = true
			if !strings.Contains(f.Message, digest.StoreFileName) {
				t.Fatalf("the finding must name the file to restore, got %q", f.Message)
			}
		}
	}
	if !named {
		t.Fatalf("expected the finding to be attributed to the claim, got %v", res.LedgerFindings)
	}
}

// The rule must stay off the states that are innocent, or it is a rule people
// switch off: a DRAFT claim has no approval to be covering anything, and a
// RELEASED record describes a claim that is allowed to be outside the approval
// path.
func TestCommentDigest_MissingEntryIsSilentForDraftsAndReleasedRecords(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/draft.yaml":  draftClaim("widget.contract.draft"),
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})

	// Release the locked claim's record the way `claim unlock` does, and empty
	// the digest store: neither claim may be reported.
	storePath := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	lock.ReleaseApproval(store, "widget.contract.locked", lock.Approval{Actor: "fixture", Reason: "unlocked"})
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	writeDigestStore(t, cfg, `{"version":1,"digests":{}}`)

	// The claim is draft on disk too — a released record with a locked claim is
	// its own finding (lock-ledger-released), which is not what this asserts.
	locked := filepath.Join(cfg.ClaimsDir, "locked.yaml")
	raw, err := os.ReadFile(locked)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if err := os.WriteFile(locked, []byte(strings.Replace(string(raw), "status: locked", "status: draft", 1)), 0o644); err != nil {
		t.Fatalf("rewrite claim: %v", err)
	}
	claims := reload(t, cfg)

	if res := check.Status(claims, cfg); hasRule(res.LedgerFindings, check.RuleCommentDigestMissing) {
		t.Fatalf("neither a draft nor a released record may be reported as uncovered, got %v", rulesOf(res.LedgerFindings))
	}
}

// THE RENAME LAUNDER. Deleting a claim's comments block alone is reported
// (comment-ledger-drift). Deleting it AND changing the claim's id in the same
// edit took every rule that starts from a claim out of play at once — the store
// knows an id that no longer exists, and the id that exists is one the store has
// never seen. The old entry survives the edit, because it is not reachable from
// the file the tamper rewrote, and that is what this rule reads.
func TestCommentDigest_RenamingACoveredClaimIsAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/thread.yaml": commentedLockedClaim("widget.internals.fields"),
	})
	if res := check.Status(claims, cfg); len(res.LedgerFindings) != 0 {
		t.Fatalf("fixture precondition: an honest project must be silent, got %v", rulesOf(res.LedgerFindings))
	}

	// The launder, in one edit: drop the thread and rename the claim. (The lock
	// ledger's own rules are sidestepped the same way, which is why this test
	// asserts on the digest rule specifically.)
	path := filepath.Join(cfg.ClaimsDir, "thread.yaml")
	renamed := "id: widget.internals.gamma\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a locked claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(path, []byte(renamed), 0o644); err != nil {
		t.Fatalf("rewrite claim: %v", err)
	}
	claims = reload(t, cfg)

	res := check.Status(claims, cfg)
	if !hasRule(res.LedgerFindings, check.RuleCommentDigestAbandoned) {
		t.Fatalf("a covered claim renamed out from under its digest entry must be reported, got %v", rulesOf(res.LedgerFindings))
	}
}

// The abandoned sweep must not fire on a departure nobody has to answer for: a
// claim whose entry recorded NO threads erased no review history when it went.
// Without this the rule would fire on every deleted draft in every project.
func TestCommentDigest_AbandonedIsSilentForAThreadlessEntry(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": draftClaim("widget.contract.a"),
		"claims/b.yaml": draftClaim("widget.contract.b"),
	})
	// Cover both claims the way the coverage sweep does, then delete one.
	store, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	digest.Adopt(store, claims)
	if err := store.Save(); err != nil {
		t.Fatalf("save digest store: %v", err)
	}
	if err := os.Remove(filepath.Join(cfg.ClaimsDir, "b.yaml")); err != nil {
		t.Fatalf("remove claim: %v", err)
	}
	claims = reload(t, cfg)

	if res := check.Status(claims, cfg); hasRule(res.LedgerFindings, check.RuleCommentDigestAbandoned) {
		t.Fatalf("a claim that recorded no threads must go quietly, got %v", rulesOf(res.LedgerFindings))
	}
}

// The COVERAGE SWEEP: a claim authored after the project's first lock used to be
// covered by nothing, forever, because the digest store existed and only a
// non-existent store was ever adopted into. A hand-written "resolved by human"
// thread on such a claim passed the gate and rendered as genuine review.
//
// PrepareStore — which every writing command runs — now extends coverage to
// every claim that has none.
func TestCommentDigest_SweepCoversAClaimAuthoredAfterTheFirstLock(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/first.yaml": lockedClaim("widget.contract.first"),
	})

	// A second claim, authored later, never through the comment engine.
	second := "id: widget.contract.second\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  authored after the project's first lock.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "second.yaml"), []byte(second), 0o644); err != nil {
		t.Fatalf("write second claim: %v", err)
	}
	claims := reload(t, cfg)

	storePath := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	lock.PrepareStore(store, claims)

	digests, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	if _, ok := digests.Digest("widget.contract.second"); !ok {
		t.Fatalf("expected the sweep to cover a claim authored after the first lock; entries=%v", digests.Digests)
	}
}

// reload re-reads the claims after a fixture edits one on disk.
func reload(t *testing.T, cfg *config.Config) []model.Claim {
	t.Helper()
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("reload claims: %v", err)
	}
	return claims
}
