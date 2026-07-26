package lock

import (
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// lockedWithRecord returns a locked claim and a store holding its (honest)
// ledger record — the clean baseline every rule below deviates from.
func lockedWithRecord(t *testing.T, c model.Claim) (model.Claim, *Store) {
	t.Helper()
	c.Status = model.StatusLocked
	store := newStore(t)
	RecordApproval(store, c, Approval{Actor: "alice", Reason: "approved"})
	return c, store
}

// TestAuditIsSilentOnAnHonestProject: a gate that fires on correct state is a
// gate people turn off. Every rule below must stay quiet here.
func TestAuditIsSilentOnAnHonestProject(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})
	draft := model.Claim{ID: "widget.contract.draft", Facet: "contract", Module: "widget", Status: model.StatusDraft}

	if findings := Audit([]model.Claim{locked, draft}, store, nil); len(findings) != 0 {
		t.Fatalf("expected no findings for an honest project, got %+v", findings)
	}
}

// TestAuditCatchesAHandFlippedStatus is the first row of the audit's table:
// editing "status: draft" to "status: locked" walked past the lint gate, hub
// gating and the unresolved-comment gate as though all three had passed, and
// nothing in the engine noticed.
func TestAuditCatchesAHandFlippedStatus(t *testing.T) {
	store := newStore(t)
	handFlipped := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}

	findings := Audit([]model.Claim{handFlipped}, store, nil)
	if !hasRule(findings, RuleLockLedgerMissing) {
		t.Fatalf("expected %s, got %+v", RuleLockLedgerMissing, findings)
	}
}

// TestAuditCatchesEditedLockedContent covers the audit's second, third and
// fourth rows at once — an edited body, a swapped raw_html payload, and a
// flipped build_role/section/order/emphasis. ContentHash sees none of these
// unless a dependent happens to exist; LockedClaimHash sees all of them.
func TestAuditCatchesEditedLockedContent(t *testing.T) {
	tamper := map[string]func(*model.Claim){
		"body edited":       func(c *model.Claim) { c.Body = "quietly rewritten" },
		"raw_html swapped":  func(c *model.Claim) { c.RawHTML = `<img src=x onerror=alert(1)>` },
		"raw_html reviewed": func(c *model.Claim) { c.RawHTMLReviewed = false },
		"build_role":        func(c *model.Claim) { c.BuildRole = model.BuildRoleOutOfScope },
		"section":           func(c *model.Claim) { c.Section = "moved elsewhere" },
		"order":             func(c *model.Claim) { c.Order = 42 },
		"emphasis":          func(c *model.Claim) { c.Emphasis = true },
	}

	for name, mutate := range tamper {
		t.Run(name, func(t *testing.T) {
			approved := model.Claim{
				ID: "widget.contract.mockup", Facet: "contract", Module: "widget",
				Layout: model.LayoutMockup, Body: "approved body",
				RawHTML: `<div>approved markup</div>`, RawHTMLReviewed: true,
				BuildRole: model.BuildRoleSchema, Section: "1 - orientation", Order: 1,
			}
			locked, store := lockedWithRecord(t, approved)

			mutate(&locked)
			findings := Audit([]model.Claim{locked}, store, nil)
			if !hasRule(findings, RuleLockContentDrift) {
				t.Fatalf("expected %s after %s, got %+v", RuleLockContentDrift, name, findings)
			}
		})
	}
}

// TestAuditIgnoresTheThreeEngineManagedFields is the deny-list, end to end. A
// comment op, a review_pending reconcile, and (on its own) a status field must
// never read as tampering — otherwise the gate would fire on every routine run
// of "dossierx check" and be worthless within a day.
func TestAuditIgnoresTheThreeEngineManagedFields(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})

	locked.ReviewPending = true
	locked.Comments = append(locked.Comments, model.Comment{
		ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
		Created: "2026-07-27T10:00:00Z", Body: "is this still true?",
	})

	if findings := Audit([]model.Claim{locked}, store, nil); len(findings) != 0 {
		t.Fatalf("a comment and a review_pending flip are ordinary engine writes, not tampering; got %+v", findings)
	}
}

// TestAuditCatchesTheOrphanedLockRecord is the audit's last row: flipping
// locked -> draft by hand to dodge review. A draft claim is edited freely and
// can be re-locked later, and before the ledger there was nothing at all to
// notice it had ever been locked. Releasing on a REAL unlock is what keeps this
// rule from firing on honest work.
func TestAuditCatchesTheOrphanedLockRecord(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "body"})

	handFlipped := locked
	handFlipped.Status = model.StatusDraft
	if !hasRule(Audit([]model.Claim{handFlipped}, store, nil), RuleLockLedgerOrphan) {
		t.Fatalf("expected %s for a draft claim still holding an active record", RuleLockLedgerOrphan)
	}

	// The same claim, unlocked properly, is silent.
	properly := Unlock(locked, store, Approval{Actor: "alice", Reason: "needs a fix"})
	if findings := Audit([]model.Claim{properly}, store, nil); len(findings) != 0 {
		t.Fatalf("an honest unlock must not trip the orphan rule, got %+v", findings)
	}
}

// TestAuditCatchesHandEditedComments: deleting an unresolved thread straight out
// of the YAML is how a claim gets past the lock gate with a review still open.
func TestAuditCatchesHandEditedComments(t *testing.T) {
	claim := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Comments: []model.Comment{{
			ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-27T10:00:00Z", Body: "this is wrong, please fix",
		}},
	}
	digests, err := digest.LoadStore(filepath.Join(t.TempDir(), "digest.json"))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	digests.Record(claim)

	if findings := Audit([]model.Claim{claim}, nil, digests); len(findings) != 0 {
		t.Fatalf("expected silence while the comment block matches its digest, got %+v", findings)
	}

	// The open thread is deleted by hand.
	tampered := claim
	tampered.Comments = nil
	if !hasRule(Audit([]model.Claim{tampered}, nil, digests), RuleCommentLedgerDrift) {
		t.Fatalf("expected %s after an unresolved thread was deleted from the YAML", RuleCommentLedgerDrift)
	}

	// So is a thread ADDED by hand to a claim whose comments were legitimately
	// emptied — which is why the digest records the empty state rather than
	// dropping the entry.
	emptied := claim
	emptied.Comments = nil
	digests.Record(emptied)
	forged := claim
	forged.Comments = []model.Comment{{ID: "c-forged", Status: model.CommentStatusResolved, Author: model.CommentRoleAgent, Created: "2026-07-27T11:00:00Z", Body: "looks fine to me"}}
	if !hasRule(Audit([]model.Claim{forged}, nil, digests), RuleCommentLedgerDrift) {
		t.Fatalf("expected %s after a thread was hand-added to a claim with an empty recorded digest", RuleCommentLedgerDrift)
	}
}

// TestAuditTreatsUnknownAsUnknown: a claim the digest store has never seen
// reports nothing. An integrity check must never manufacture a finding out of
// missing evidence — the same distinction Store.Baseline draws for dependency
// hashes.
func TestAuditTreatsUnknownAsUnknown(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Comments: []model.Comment{{ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-27T10:00:00Z", Body: "hello"}}}
	digests, err := digest.LoadStore(filepath.Join(t.TempDir(), "digest.json"))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}

	if findings := Audit([]model.Claim{claim}, nil, digests); len(findings) != 0 {
		t.Fatalf("a claim with no recorded digest is uncovered, not drifted; got %+v", findings)
	}
	if findings := Audit([]model.Claim{claim}, nil, nil); len(findings) != 0 {
		t.Fatalf("a nil digest store must disable the comment rules, not fire them; got %+v", findings)
	}
}

// TestAuditDoesNotReadABuildOrderRecordAsAClaim: the ledger holds two subject
// kinds in one map. If the claim rules matched on the key alone, a build-order
// record would have to be parsed out by shape — and a subject added later would
// silently start being audited as a claim.
func TestAuditDoesNotReadABuildOrderRecordAsAClaim(t *testing.T) {
	store := newStore(t)
	RecordBuildOrderApproval(store, "widget", "artifact-signature", Approval{Actor: "alice", Reason: "order approved"})

	// A claim whose id collides with the build-order key would still be
	// reported: the record it finds is not a claim record.
	collide := model.Claim{ID: BuildOrderLedgerKey("widget"), Facet: "contract", Module: "widget", Status: model.StatusLocked}
	if !hasRule(Audit([]model.Claim{collide}, store, nil), RuleLockLedgerMissing) {
		t.Fatalf("a build-order record must never satisfy a claim's approval requirement")
	}
}

// TestAuditOrderIsDeterministic: the hook's output and a CI log get diffed
// against each other, so the same state must always print the same way.
func TestAuditOrderIsDeterministic(t *testing.T) {
	store := newStore(t)
	claims := []model.Claim{
		{ID: "widget.contract.c", Facet: "contract", Module: "widget", Status: model.StatusLocked},
		{ID: "widget.contract.a", Facet: "contract", Module: "widget", Status: model.StatusLocked},
		{ID: "widget.contract.b", Facet: "contract", Module: "widget", Status: model.StatusLocked},
	}

	first := Audit(claims, store, nil)
	for i := 0; i < 20; i++ {
		again := Audit(claims, store, nil)
		if len(again) != len(first) {
			t.Fatalf("finding count varies between runs")
		}
		for j := range again {
			if again[j] != first[j] {
				t.Fatalf("finding %d varies between runs: %+v vs %+v", j, again[j], first[j])
			}
		}
	}
	// Project-scoped findings sort first (empty claim id), then by claim id.
	if first[0].Rule != RuleLockLedgerAbsent {
		t.Fatalf("expected the project-scoped finding first, got %+v", first[0])
	}
	if first[1].ClaimID != "widget.contract.a" || first[3].ClaimID != "widget.contract.c" {
		t.Fatalf("expected per-claim findings sorted by id, got %+v", first)
	}
}
