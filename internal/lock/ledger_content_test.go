package lock

import (
	"encoding/json"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// RecordApproval must store the TEXT behind the hash, or the viewer has
// nothing to show a reviewer when the wording later moves.
func TestRecordApprovalKeepsTheApprovedText(t *testing.T) {
	store := &Store{}
	claim := model.Claim{ID: "fixture.contract.a", Status: model.StatusLocked, Body: "The approved wording."}
	RecordApproval(store, claim, Approval{Actor: "approver", Reason: "reviewed"})

	got, ok := store.ApprovedContent(claim.ID)
	if !ok {
		t.Fatal("an approval must retain the claim it approved")
	}
	if got.Body != claim.Body {
		t.Fatalf("retained body is %q, want %q", got.Body, claim.Body)
	}
	if LockedClaimHash(got) != store.Ledger[claim.ID].Hash {
		t.Fatal("the retained content must hash to the hash recorded beside it")
	}
}

// Unlock releases the record; it must not drop the text. The released window
// is precisely when the approved wording is needed, because that is when the
// current wording starts to differ from it.
func TestReleaseApprovalKeepsTheApprovedText(t *testing.T) {
	store := &Store{}
	claim := model.Claim{ID: "fixture.contract.b", Status: model.StatusLocked, Body: "Approved."}
	RecordApproval(store, claim, Approval{Actor: "approver", Reason: "reviewed"})
	ReleaseApproval(store, claim.ID, Approval{Actor: "maintainer", Reason: "rewording"})

	got, ok := store.ApprovedContent(claim.ID)
	if !ok || got.Body != "Approved." {
		t.Fatalf("release must keep the approved text, got %q ok=%v", got.Body, ok)
	}
}

// A store written before this field existed decodes with no content. The
// second return is what keeps that distinguishable from "an empty claim was
// approved" — a consumer that lost the distinction would render the whole
// claim as newly added.
func TestApprovedContentReportsALegacyRecordAsUnretained(t *testing.T) {
	var store Store
	legacy := []byte(`{"version":2,"hashes":{},"locked_at":{},"ledger":{"fixture.contract.c":{"subject":"claim","hash":"abc","at":"2026-01-01T00:00:00Z","actor":"a","reason":"r"}}}`)
	if err := json.Unmarshal(legacy, &store); err != nil {
		t.Fatalf("decode legacy store: %v", err)
	}
	if _, ok := store.ApprovedContent("fixture.contract.c"); ok {
		t.Fatal("a record with no content must report the content as not retained")
	}
	if _, ok := store.ApprovedContent("fixture.contract.absent"); ok {
		t.Fatal("an absent record must report the content as not retained")
	}
}

// A leftover v0.7.20 "build-order" row is not a claim approval.
// ApprovedContent filters on Subject rather than on the key's shape.
func TestApprovedContentIgnoresNonClaimSubjects(t *testing.T) {
	store := &Store{Ledger: map[string]LedgerRecord{
		"build-order:fixture": {Subject: "build-order", Hash: "abc", Content: &model.Claim{ID: "build-order:fixture"}},
	}}
	if _, ok := store.ApprovedContent("build-order:fixture"); ok {
		t.Fatal("a non-claim record must never answer as claim content")
	}
}

func TestSignedFieldsDifferingNamesOnlySignedFields(t *testing.T) {
	a := model.Claim{ID: "fixture.contract.d", Status: model.StatusLocked, Body: "one"}
	b := a
	b.Body = "two"
	b.RestsOn = model.RestsOnIDs("fixture.contract.other")
	// status, review_pending and comments are the three fields
	// LockedClaimHash does not sign; a difference in them is not a reason the
	// hash moved and must not be reported as one.
	b.Status = model.StatusDraft
	b.ReviewPending = true

	got := SignedFieldsDiffering(a, b)
	want := map[string]bool{"body": true, "rests_on": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want exactly %v", got, want)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("%q is not a signed field difference; got %v", name, got)
		}
	}
	if len(SignedFieldsDiffering(a, a)) != 0 {
		t.Fatal("identical claims differ in no field")
	}
}
