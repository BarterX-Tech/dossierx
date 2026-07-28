// audit_boundary_test.go pins THE BOUNDARY OF THIS GATE at the rules' own level:
// per-claim coordinated changes audit.go documents as UNDETECTED (the disowned
// claim and the erased review), and the single-tree refusals that still stand on
// either side of them. The cases here are INSTANCES of audit.go's principle — an
// in-repo ledger cannot attest anything against the person who can write it —
// not an inventory of it; do not read the set as complete, and do not put a
// count on it.
//
// It is the successor to audit_history_test.go, which was deleted. That file
// exercised AuditAgainstParent, and it kept PASSING after the parent-commit
// comparison was removed from the product — asserting detections that no longer
// happened anywhere in the binary. A green test for a control that is not wired
// is worse than no test: the next reader takes the gap for covered.
//
// So the assertions here are inverted on purpose. Where the old file proved "the
// history gate catches this", these prove "NOTHING catches this, and here is
// exactly what that looks like", beside "either half alone is STILL a refusal",
// which is the part that has to keep working. If a future change closes one of
// these gaps, the corresponding test FAILS — and that failure is the signal to
// come back and rewrite audit.go's boundary note, not to weaken the test.
package lock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// boundaryProject builds a ledger-covered project on disk — a lock store at the
// ledger schema with a comment digest store beside it — and returns both stores
// loaded. Coverage is what arms lock-ledger-deleted and
// comment-digest-unrecorded, so it is the state every case below needs.
func boundaryProject(t *testing.T) (*Store, *digest.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"hashes":{},"locked_at":{},"ledger":{}}`), 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	digests, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	if err := digests.Save(); err != nil {
		t.Fatalf("digest Save: %v", err)
	}
	digests, err = digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("digest reload: %v", err)
	}
	return store, digests
}

// honestLock records everything a real Lock leaves behind for claim c: the
// locked_at stamp, the dependency baselines, and the ledger record. It is the
// "all three keys" state the disowned-claim case has to erase.
func honestLock(t *testing.T, store *Store, c model.Claim, deps ...model.Claim) {
	t.Helper()
	if store.LockedAt == nil {
		store.LockedAt = map[string]string{}
	}
	store.LockedAt[c.ID] = "2026-07-01T00:00:00Z"
	for _, dep := range deps {
		store.recordBaseline(c.ID, dep.ID, ContentHash(dep))
	}
	RecordApproval(store, c, Approval{Actor: "alice", Reason: "approved"})
}

// TestAuditBoundary_TheDisownedClaimIsNotDetected is the DISOWNED CLAIM, one of
// boundary note, made executable.
//
// Deleting a claim's ledger record, its locked_at stamp and its own dependency
// baselines in ONE edit takes it out of every rule Audit has, because
// engineLocked reads exactly the two keys that were deleted alongside the record.
// The claim becomes an ordinary, freely-editable draft, and re-locking it
// afterwards writes a record indistinguishable from a first approval.
//
// Nothing in this build reports it. The forge does: three keys out of a tracked
// JSON file, plus a status flip and a rewritten body, in one diff.
func TestAuditBoundary_TheDisownedClaimIsNotDetected(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep"}
	victim := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget",
		Status: model.StatusLocked, Body: "the approved body", RestsOn: []string{dep.ID},
	}

	honest, honestDigests := boundaryProject(t)
	honestLock(t, honest, dep)
	honestLock(t, honest, victim, dep)
	if findings := Audit([]model.Claim{dep, victim}, honest, honestDigests); len(findings) != 0 {
		t.Fatalf("the honest project must be silent, got %+v", findings)
	}

	// THE WHOLE SHAPE: three keys out of the lock store, one line of YAML, and
	// the body rewritten. The dependency is left alone, so the store is otherwise
	// a perfectly ordinary covered store.
	tampered := victim
	tampered.Status = model.StatusDraft
	tampered.Body = "quietly rewritten"
	claims := []model.Claim{dep, tampered}

	store, digests := boundaryProject(t)
	honestLock(t, store, dep)

	if findings := Audit(claims, store, digests); len(findings) != 0 {
		t.Fatalf("audit.go documents the disowned claim as UNDETECTED. It is now detected as %+v — which is good news, but audit.go's boundary note (RuleLockLedgerDeleted's \"WHAT IT DOES NOT CLOSE\", and THE BOUNDARY OF THIS GATE) still says nothing sees it. Update the prose in the same change as the rule", findings)
	}
	// The write path agrees it is blind: Lock would happily record a fresh
	// approval over the rewritten body.
	if store.LedgerRecordDeleted(tampered) {
		t.Fatalf("engineLocked is documented as blind to a three-key erasure; if it is not, audit.go's boundary note is out of date")
	}
}

// TestAuditBoundary_EitherHalfOfTheDisownedClaimIsStillRefused is the half that
// has to keep working, and the reason the disowned claim is a CONJUNCTION rather than a
// hole: an attacker who forgets one key is refused from this one tree.
func TestAuditBoundary_EitherHalfOfTheDisownedClaimIsStillRefused(t *testing.T) {
	victim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "the approved body"}

	t.Run("the record deleted, locked_at left behind", func(t *testing.T) {
		store, digests := boundaryProject(t)
		honestLock(t, store, victim)
		delete(store.Ledger, victim.ID)

		flipped := victim
		flipped.Status = model.StatusDraft
		flipped.Body = "quietly rewritten"
		if !hasRule(Audit([]model.Claim{flipped}, store, digests), RuleLockLedgerDeleted) {
			t.Fatalf("deleting the record while locked_at survives must stay reported as %s", RuleLockLedgerDeleted)
		}
	})

	t.Run("the keys deleted, the status left locked", func(t *testing.T) {
		// Erasing every trace of the lock but forgetting to flip the YAML leaves
		// a locked claim with no record at all, which is lock-ledger-missing.
		store, digests := boundaryProject(t)
		if !hasRule(Audit([]model.Claim{victim}, store, digests), RuleLockLedgerMissing) {
			t.Fatalf("a locked claim with no evidence anywhere must stay reported as %s", RuleLockLedgerMissing)
		}
	})
}

// TestAuditBoundary_TheErasedReviewIsNotDetected is the ERASED REVIEW, one of
// boundary note: a human's OPEN thread on a DRAFT claim, erased from the YAML
// together with the claim's key in the comment digest store.
//
// An open thread is what BLOCKS `claim lock`, so the erasure buys the lock, and
// the claim then locks cleanly over a review that was deleted. Nothing in this
// build reports it.
//
// It is confined to draft claims because check.RuleCommentDigestMissing keys on
// a STANDING lock-ledger record with no digest entry: a locked claim has one and
// is reported (comment-digest-missing), a draft claim has none and is never
// asked. Not lock-content-drift — `comments` is excluded from the locked-claim
// hash by lockedClaimHashExcluded, so no comments edit can ever produce it.
func TestAuditBoundary_TheErasedReviewIsNotDetected(t *testing.T) {
	reviewed := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "this is not what we agreed",
		}},
	}

	erased := reviewed
	erased.Comments = nil
	claims := []model.Claim{erased}

	store, digests := boundaryProject(t)
	digests.Record(reviewed)
	digests.Forget(reviewed.ID)

	if findings := Audit(claims, store, digests); len(findings) != 0 {
		t.Fatalf("audit.go documents the erased review as UNDETECTED. It is now detected as %+v — update RuleCommentDigestUnrecorded's \"THE ONE THING IT CANNOT SEE\" and THE BOUNDARY OF THIS GATE in the same change", findings)
	}
}

// TestAuditBoundary_EitherHalfOfTheErasedReviewIsStillRefused: the erased review is a
// conjunction too. Erase the block alone and the surviving entry disagrees with
// it; drop the entry alone and the threads have nothing recorded beside them.
func TestAuditBoundary_EitherHalfOfTheErasedReviewIsStillRefused(t *testing.T) {
	reviewed := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "this is not what we agreed",
		}},
	}

	t.Run("the block erased, the entry left behind", func(t *testing.T) {
		store, digests := boundaryProject(t)
		digests.Record(reviewed)

		erased := reviewed
		erased.Comments = nil
		if !hasRule(Audit([]model.Claim{erased}, store, digests), RuleCommentLedgerDrift) {
			t.Fatalf("erasing the block alone must stay reported as %s", RuleCommentLedgerDrift)
		}
	})

	t.Run("the entry dropped, the threads left behind", func(t *testing.T) {
		store, digests := boundaryProject(t)
		digests.Record(reviewed)
		digests.Forget(reviewed.ID)

		if !hasRule(Audit([]model.Claim{reviewed}, store, digests), RuleCommentDigestUnrecorded) {
			t.Fatalf("dropping the entry alone must stay reported as %s", RuleCommentDigestUnrecorded)
		}
	})
}
