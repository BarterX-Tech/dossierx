package approvaledit

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func approvedThenReleased(c model.Claim) *lock.Store {
	s := &lock.Store{PolicyVersion: lock.PolicyLocalApprovalV1, Ledger: map[string]lock.LedgerRecord{}}
	content := c
	s.Ledger[c.ID] = lock.LedgerRecord{
		Subject: lock.SubjectClaim, Hash: lock.LockedClaimHash(c),
		At: "2026-09-03T00:00:00Z", Actor: "approver", Reason: "reviewed with the team",
		ReleasedAt: "2026-09-04T00:00:00Z", ReleasedBy: "maintainer", ReleasedReason: "tightening the wording",
		Content: &content,
	}
	return s
}

func locked(id, body string) model.Claim {
	return model.Claim{ID: id, Module: "fixture", Facet: "contract", Status: model.StatusLocked, Body: body}
}

func draft(c model.Claim, body string) model.Claim {
	c.Status = model.StatusDraft
	c.Body = body
	return c
}

func TestComputeReportsTheWordingThatMoved(t *testing.T) {
	approved := locked("fixture.contract.a", "A withdrawal is derived.\n\nIt is never entered by hand.")
	store := approvedThenReleased(approved)
	edited := draft(approved, "A withdrawal is derived.\n\nIt is entered by an operator.")

	changes := Compute([]model.Claim{edited}, store)
	change, ok := changes[edited.ID]
	if !ok {
		t.Fatal("an unlocked-and-rewritten claim must produce a change")
	}
	if !change.ContentRetained {
		t.Fatal("a record carrying the approved claim must report the content as retained")
	}
	if !change.BodyChanged() {
		t.Fatalf("the body moved but no hunk says so: %+v", change.Hunks)
	}
	if change.ChangedPassages != 1 {
		t.Fatalf("one passage moved, so the header must say one: got %d", change.ChangedPassages)
	}
	// Every passage of both versions must be on the page somewhere: the
	// unchanged ones once, the changed one twice (old and new).
	var removed, added, equal []string
	for _, h := range change.Hunks {
		switch h.Op {
		case "remove":
			removed = append(removed, h.HTML)
		case "add":
			added = append(added, h.HTML)
		default:
			equal = append(equal, h.HTML)
		}
	}
	// The changed passage is on the page twice, once per side, with the words
	// that moved marked inside each. Asserting on the marks rather than on a
	// contiguous sentence is deliberate: a passage with a mark in the middle
	// of it no longer contains that sentence as one string, and a test that
	// wanted it to would be a test against the feature.
	if len(removed) != 1 || !strings.Contains(removed[0], `claim-edit-word--removed">never</span>`) {
		t.Fatalf("the approved wording must be shown as its own passage, marked: %v", removed)
	}
	if len(added) != 1 || !strings.Contains(added[0], `claim-edit-word--added">an`) {
		t.Fatalf("the current wording must be shown as its own passage, marked: %v", added)
	}
	if !strings.Contains(removed[0], "It is ") || !strings.Contains(added[0], "entered by") {
		t.Fatalf("each side must still carry the words around its marks: %v / %v", removed, added)
	}
	if len(equal) != 1 || !strings.Contains(equal[0], "is derived") {
		t.Fatalf("the untouched passage must be shown once, unmarked: %v", equal)
	}
	if change.ApprovedReason != "reviewed with the team" || change.ReleasedReason != "tightening the wording" {
		t.Fatalf("the approving and releasing words must ride along: %+v", change)
	}
}

// The panel must never assert a change and then show nothing. When the hash
// moved because structure moved, the change names the fields instead.
func TestComputeNamesNonBodyFieldsWhenTheWordingHeld(t *testing.T) {
	approved := locked("fixture.contract.b", "Unchanged wording.")
	store := approvedThenReleased(approved)
	edited := draft(approved, approved.Body)
	edited.RestsOn = model.RestsOnIDs("fixture.contract.other")

	change, ok := Compute([]model.Claim{edited}, store)[edited.ID]
	if !ok {
		t.Fatal("a claim whose structure moved after release must still produce a change")
	}
	if change.BodyChanged() {
		t.Fatalf("the wording did not move; no hunk may say it did: %+v", change.Hunks)
	}
	if len(change.OtherFields) != 1 || change.OtherFields[0] != "rests_on" {
		t.Fatalf("the moved field must be named, got %v", change.OtherFields)
	}
}

// A record minted before LedgerRecord.Content existed proves the text moved
// and cannot say what it moved from. Reporting that honestly is the whole
// point of the second return on Store.ApprovedContent.
func TestComputeSaysSoWhenTheApprovedTextWasNotRetained(t *testing.T) {
	approved := locked("fixture.contract.c", "Original wording.")
	store := approvedThenReleased(approved)
	r := store.Ledger[approved.ID]
	r.Content = nil
	store.Ledger[approved.ID] = r

	change, ok := Compute([]model.Claim{draft(approved, "New wording.")}, store)[approved.ID]
	if !ok {
		t.Fatal("a legacy record must still report that the claim was edited")
	}
	if change.ContentRetained {
		t.Fatal("a record with no content must not claim the content was retained")
	}
	if len(change.Hunks) != 0 {
		t.Fatalf("with nothing to diff against there must be no hunks, got %+v", change.Hunks)
	}
}

// Compute and readiness.CauseUnapprovedEdit must agree on which claims are in
// this state. These are the four cases where a naive predicate would disagree.
func TestComputeSilentOnEveryStateThatIsNotAnUnapprovedEdit(t *testing.T) {
	approved := locked("fixture.contract.d", "Body.")

	standing := approvedThenReleased(approved)
	r := standing.Ledger[approved.ID]
	r.ReleasedAt, r.ReleasedBy, r.ReleasedReason = "", "", ""
	standing.Ledger[approved.ID] = r

	cases := []struct {
		name   string
		claims []model.Claim
		store  *lock.Store
	}{
		{"still locked", []model.Claim{approved}, approvedThenReleased(approved)},
		{"released and unedited", []model.Claim{draft(approved, approved.Body)}, approvedThenReleased(approved)},
		{"never approved", []model.Claim{draft(approved, "anything")}, &lock.Store{Ledger: map[string]lock.LedgerRecord{}}},
		{"unreleased record (tamper)", []model.Claim{draft(approved, "edited")}, standing},
		{"nil store", []model.Claim{draft(approved, "edited")}, nil},
	}
	for _, tc := range cases {
		if got := Compute(tc.claims, tc.store); len(got) != 0 {
			t.Fatalf("%s: expected no change, got %+v", tc.name, got)
		}
	}
}

func TestComputeDoesNotMutateItsInputs(t *testing.T) {
	approved := locked("fixture.contract.e", "Before.")
	store := approvedThenReleased(approved)
	record := store.Ledger[approved.ID]
	edited := draft(approved, "After.")
	claimCopy := edited

	Compute([]model.Claim{edited}, store)

	if edited.Body != claimCopy.Body || edited.Status != claimCopy.Status {
		t.Fatal("Compute mutated the claim it was given")
	}
	got := store.Ledger[approved.ID]
	if got.Hash != record.Hash || got.ReleasedAt != record.ReleasedAt || got.Content != record.Content {
		t.Fatal("Compute mutated the ledger record")
	}
}

// The refinement, stated as a claim about what a reviewer sees: a one-word
// change must not render as a rewritten passage merely because the author's
// editor re-wrapped it.
func TestComputeMarksTheWordThatMovedNotThePassage(t *testing.T) {
	approved := locked("fixture.contract.wrapped",
		"The field is outward, not internal. A claim in the support authority already depends on it.\n"+
			"An observer revision changing is enumerated there among the material changes that revoke a\n"+
			"support verdict.")
	store := approvedThenReleased(approved)

	// One word changed AND the passage re-wrapped afterwards.
	edited := draft(approved,
		"The field is outward, not private. A claim in the support authority already depends on\n"+
			"it. An observer revision changing is enumerated there among the material changes that\n"+
			"revoke a support verdict.")

	change, ok := Compute([]model.Claim{edited}, store)[edited.ID]
	if !ok {
		t.Fatal("the edit must produce a change")
	}
	if change.ChangedPassages != 1 {
		t.Fatalf("one passage moved, got %d", change.ChangedPassages)
	}
	var changed []string
	for _, h := range change.Hunks {
		changed = append(changed, h.Changed...)
	}
	if len(changed) != 2 || changed[0] != "internal." || changed[1] != "private." {
		t.Fatalf("one word moved, so exactly one word is marked on each side; got %v", changed)
	}
	// And the whole passage is still there on both sides, as prose.
	for _, h := range change.Hunks {
		if strings.Contains(h.HTML, "**") {
			t.Fatalf("a passage must render as prose, not source: %q", h.HTML)
		}
		if !strings.Contains(h.HTML, "support verdict") {
			t.Fatalf("a marked passage must still carry the sentence around the change: %q", h.HTML)
		}
	}
}
