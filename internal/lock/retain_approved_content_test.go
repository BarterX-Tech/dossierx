package lock

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// legacyStore is a ledger record written before Content existed: a hash, an
// approval, and a release, with no wording behind any of them. It is the state
// approved-content recovery exists for.
func legacyStore(t *testing.T, id, hash string) *Store {
	t.Helper()
	var store Store
	raw := `{"version":2,"hashes":{},"locked_at":{},"ledger":{"` + id + `":{` +
		`"subject":"claim","hash":"` + hash + `","at":"2026-01-01T00:00:00Z",` +
		`"actor":"approver","reason":"reviewed",` +
		`"released_at":"2026-02-01T00:00:00Z","released_by":"maintainer","released_reason":"rewording"}}}`
	if err := json.Unmarshal([]byte(raw), &store); err != nil {
		t.Fatalf("decode legacy store: %v", err)
	}
	return &store
}

// The whole safety argument for recovery is this test. RetainApprovedContent
// must accept only content that hashes to the hash the record ALREADY signed,
// so no caller — whatever it read, from git or anywhere else — can put
// different words behind an existing approval.
func TestRetainApprovedContentRefusesContentThatDoesNotHashToTheApproval(t *testing.T) {
	approved := model.Claim{ID: "fixture.contract.a", Status: model.StatusLocked, Body: "The approved wording."}
	store := legacyStore(t, approved.ID, LockedClaimHash(approved))

	tampered := approved
	tampered.Body = "Words nobody approved."
	stored, err := store.RetainApprovedContent(approved.ID, tampered)
	if stored {
		t.Fatal("content that does not hash to the approval must never be stored")
	}
	if !errors.Is(err, ErrContentMismatch) {
		t.Fatalf("refusal must be ErrContentMismatch, got %v", err)
	}
	if store.Ledger[approved.ID].Content != nil {
		t.Fatal("a refused retain must leave the record without content")
	}

	// And the honest one is accepted, so the refusal above is a real
	// discrimination rather than a function that refuses everything.
	stored, err = store.RetainApprovedContent(approved.ID, approved)
	if err != nil || !stored {
		t.Fatalf("content that hashes to the approval must be stored: stored=%v err=%v", stored, err)
	}
	got, ok := store.ApprovedContent(approved.ID)
	if !ok || got.Body != approved.Body {
		t.Fatalf("retained body is %q ok=%v, want %q", got.Body, ok, approved.Body)
	}
}

// Retaining must change the WORDING on the record and nothing else. If it
// moved a hash, a timestamp, an actor or a reason it would be rewriting the
// approval rather than completing it, and the record would no longer describe
// the event that actually happened.
func TestRetainApprovedContentChangesNothingButTheContent(t *testing.T) {
	approved := model.Claim{ID: "fixture.contract.b", Status: model.StatusLocked, Body: "Approved."}
	store := legacyStore(t, approved.ID, LockedClaimHash(approved))
	before := store.Ledger[approved.ID]

	if _, err := store.RetainApprovedContent(approved.ID, approved); err != nil {
		t.Fatalf("retain: %v", err)
	}
	after := store.Ledger[approved.ID]

	// Blank the one field that is allowed to move and require the rest to be
	// identical. Naming the survivors instead would silently pass a field
	// added to LedgerRecord tomorrow.
	after.Content = nil
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("retain moved something other than the content:\n before %+v\n after  %+v", before, after)
	}
}

// A record that already carries its approved wording — because it was written
// by an engine that kept it — must never be overwritten by a recovered
// historical revision. The engine's own record is the better evidence.
func TestRetainApprovedContentNeverReplacesRetainedWording(t *testing.T) {
	approved := model.Claim{ID: "fixture.contract.c", Status: model.StatusLocked, Body: "Kept at lock time."}
	store := &Store{}
	RecordApproval(store, approved, Approval{Actor: "approver", Reason: "reviewed"})
	ReleaseApproval(store, approved.ID, Approval{Actor: "maintainer", Reason: "rewording"})

	stored, err := store.RetainApprovedContent(approved.ID, approved)
	if err != nil {
		t.Fatalf("retain over a retained record must not error, got %v", err)
	}
	if stored {
		t.Fatal("a record that already carries its wording must report nothing stored")
	}
	got, _ := store.ApprovedContent(approved.ID)
	if got.Body != "Kept at lock time." {
		t.Fatalf("retained wording was replaced: %q", got.Body)
	}
}

// A record with no hash cannot certify anything, so it cannot be completed
// either. Storing content against it would create the one thing the design
// refuses: wording on the record that nothing signed.
func TestRetainApprovedContentRefusesARecordWithNoHash(t *testing.T) {
	claim := model.Claim{ID: "fixture.contract.d", Status: model.StatusLocked, Body: "Anything."}
	store := legacyStore(t, claim.ID, "")
	if _, err := store.RetainApprovedContent(claim.ID, claim); !errors.Is(err, ErrContentMismatch) {
		t.Fatalf("a record with no hash must refuse with ErrContentMismatch, got %v", err)
	}
	if _, err := store.RetainApprovedContent("fixture.contract.absent", claim); err == nil {
		t.Fatal("a claim with no ledger record must refuse")
	}
}

// EditedSinceApproval is the one predicate readiness, approvaledit and
// approvalrecovery all consult. Its conditions are pinned here, together,
// because the cost of any one of them drifting is a claim that appears on one
// surface and not the other.
func TestEditedSinceApprovalConditions(t *testing.T) {
	approved := model.Claim{ID: "fixture.contract.e", Status: model.StatusLocked, Body: "Approved."}
	edited := approved
	edited.Status = model.StatusDraft
	edited.Body = "Rewritten."

	t.Run("released and edited reports true", func(t *testing.T) {
		store := legacyStore(t, approved.ID, LockedClaimHash(approved))
		if _, ok := EditedSinceApproval(edited, store); !ok {
			t.Fatal("a released approval whose text has moved is an unapproved edit")
		}
	})

	t.Run("unchanged text reports false", func(t *testing.T) {
		store := legacyStore(t, approved.ID, LockedClaimHash(approved))
		unchanged := approved
		unchanged.Status = model.StatusDraft
		// A draft of the same words hashes differently only if status is
		// signed, and it is not — see lockedClaimHashExcluded.
		if _, ok := EditedSinceApproval(unchanged, store); ok {
			t.Fatal("a claim whose text still matches its approval is not an unapproved edit")
		}
	})

	t.Run("still locked reports false", func(t *testing.T) {
		store := legacyStore(t, approved.ID, LockedClaimHash(approved))
		locked := edited
		locked.Status = model.StatusLocked
		if _, ok := EditedSinceApproval(locked, store); ok {
			t.Fatal("only a draft can be edited away from its approval; unlock is the only path to draft")
		}
	})

	t.Run("a record that was never released reports false", func(t *testing.T) {
		store := &Store{}
		RecordApproval(store, approved, Approval{Actor: "approver", Reason: "reviewed"})
		if _, ok := EditedSinceApproval(edited, store); ok {
			t.Fatal("an unreleased record is tampering (lock-ledger-orphan), not an honest unlock; the integrity gate owns it")
		}
	})

	t.Run("no store reports false", func(t *testing.T) {
		if _, ok := EditedSinceApproval(edited, nil); ok {
			t.Fatal("no store means no approval to have been edited away from")
		}
	})
}
