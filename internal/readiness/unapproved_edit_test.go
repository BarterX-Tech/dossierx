package readiness

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// releasedStore returns a store holding one RELEASED claim record for each
// supplied claim, signed against that claim exactly as it stands — i.e. the
// state right after an honest unlock, before anybody has edited anything.
func releasedStore(claims ...model.Claim) *lock.Store {
	s := standingStore(claims...)
	for _, c := range claims {
		r := s.Ledger[c.ID]
		r.ReleasedAt = "2026-09-04T00:00:00Z"
		r.ReleasedBy = "maintainer"
		r.ReleasedReason = "fixing the wording"
		s.Ledger[c.ID] = r
	}
	return s
}

func draftOf(c model.Claim) model.Claim {
	c.Status = model.StatusDraft
	c.ReviewPending = false
	return c
}

// The gap this cause closes: before it, a claim that was unlocked, rewritten,
// and depended on by nothing produced no readiness record anywhere, so the
// viewer's banner could count it while the Issues screen had no row to list.
func TestUnapprovedEditIsOwnCauseWithNoDependents(t *testing.T) {
	approved := lockedClaim("fixture.contract.alone")
	store := releasedStore(approved)

	edited := draftOf(approved)
	edited.Body = "a different assertion entirely"

	got := Compute([]model.Claim{edited}, store, nil)[edited.ID]
	if !hasCause(got, CauseUnapprovedEdit, edited.ID) {
		t.Fatalf("an unlocked-and-rewritten claim must own an unapproved_edit cause; got causes %+v", got.ReviewCauses)
	}
	if !got.ReviewPending {
		t.Fatal("a claim with an active cause must report review_pending")
	}
}

func TestUnapprovedEditSilentUntilTheTextActuallyMoves(t *testing.T) {
	approved := lockedClaim("fixture.contract.untouched")
	store := releasedStore(approved)

	// Unlocked and not yet edited: the approval is released, the text is the
	// approved text. There is nothing for a reviewer to look at, and asserting
	// a change here would put a row on the Issues screen for every claim
	// anybody has open.
	got := Compute([]model.Claim{draftOf(approved)}, store, nil)[approved.ID]
	if hasCause(got, CauseUnapprovedEdit, approved.ID) {
		t.Fatalf("an unlocked but unedited claim must own no unapproved_edit cause; got %+v", got.ReviewCauses)
	}
}

// A draft claim that was NEVER approved is the other half of the word DRAFT,
// and it must stay distinguishable: no record means nothing was approved,
// which means nothing has moved away from an approval.
func TestUnapprovedEditNeverFiresOnAClaimThatWasNeverApproved(t *testing.T) {
	never := draftOf(lockedClaim("fixture.contract.new"))
	store := standingStore() // no records at all

	got := Compute([]model.Claim{never}, store, nil)[never.ID]
	if hasCause(got, CauseUnapprovedEdit, never.ID) {
		t.Fatalf("a claim with no ledger record must own no unapproved_edit cause; got %+v", got.ReviewCauses)
	}
}

// An UNRELEASED record on a draft claim is the lock-ledger-orphan tamper
// finding — somebody flipped locked -> draft by hand. Reporting it here as an
// ordinary edit would offer the softer of the two available readings of the
// same bytes.
func TestUnapprovedEditDefersToTheOrphanTamperFinding(t *testing.T) {
	approved := lockedClaim("fixture.contract.orphan")
	store := standingStore(approved) // standing, never released

	edited := draftOf(approved)
	edited.Body = "hand-edited after a hand-flipped status"

	got := Compute([]model.Claim{edited}, store, nil)[edited.ID]
	if hasCause(got, CauseUnapprovedEdit, edited.ID) {
		t.Fatalf("a draft holding an UNRELEASED record must not read as an ordinary edit; got %+v", got.ReviewCauses)
	}
}

// A record minted before LockedClaimHash was recorded carries no hash. It
// cannot say the text moved, and silence is the honest answer.
func TestUnapprovedEditSilentWhenTheRecordCarriesNoHash(t *testing.T) {
	approved := lockedClaim("fixture.contract.hashless")
	store := releasedStore(approved)
	r := store.Ledger[approved.ID]
	r.Hash = ""
	store.Ledger[approved.ID] = r

	edited := draftOf(approved)
	edited.Body = "rewritten"

	got := Compute([]model.Claim{edited}, store, nil)[edited.ID]
	if hasCause(got, CauseUnapprovedEdit, edited.ID) {
		t.Fatalf("a hashless record cannot assert that content moved; got %+v", got.ReviewCauses)
	}
}

// The cause reaches dependents the same way every other claim-owned cause
// does — as an inherited upstream_dependency_review retaining the source kind
// — rather than through any new propagation path of its own.
func TestUnapprovedEditPropagatesAsOrdinaryUpstreamReview(t *testing.T) {
	dep := lockedClaim("fixture.contract.dep")
	store := releasedStore(dep)

	edited := draftOf(dep)
	edited.Body = "rewritten after unlock"

	dependent := lockedClaim("fixture.contract.dependent", dep.ID)
	store.Ledger[dependent.ID] = lock.LedgerRecord{
		Subject: lock.SubjectClaim, Hash: lock.LockedClaimHash(dependent),
		At: "2026-09-03T00:00:00Z", Reason: "fixture approval",
	}
	recordBaseline(store, dependent.ID, dep)

	got := Compute([]model.Claim{edited, dependent}, store, nil)[dependent.ID]
	var found bool
	for _, cause := range got.ReviewCauses {
		if cause.Kind == CauseUpstreamDependencyReview && cause.SourceKind == CauseUnapprovedEdit {
			found = true
			if cause.Direct {
				t.Fatal("an inherited cause must not claim to be direct")
			}
		}
	}
	if !found {
		t.Fatalf("the dependent must inherit the edit as an upstream review cause; got %+v", got.ReviewCauses)
	}
}

// Readiness is a read-only calculation (see the package doc). The new cause
// reads the store; it must not write to it.
func TestUnapprovedEditLeavesTheStoreUntouched(t *testing.T) {
	approved := lockedClaim("fixture.contract.readonly")
	store := releasedStore(approved)
	before := store.Ledger[approved.ID]

	edited := draftOf(approved)
	edited.Body = "rewritten"
	Compute([]model.Claim{edited}, store, nil)

	if store.Ledger[approved.ID] != before {
		t.Fatalf("Compute mutated the ledger record: %+v -> %+v", before, store.Ledger[approved.ID])
	}
}
