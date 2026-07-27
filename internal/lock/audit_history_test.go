package lock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// historyCommit builds a ledger-covered project on disk — a lock store at the
// ledger schema with a comment digest store beside it — and returns both stores
// loaded, so the tests below can play "parent commit" and "commit under audit"
// against each other the way internal/check's staged gate does.
func historyCommit(t *testing.T) (*Store, *digest.Store) {
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
// locked_at stamp, the dependency baselines, and the ledger record.
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

// TestAuditAgainstParentCatchesTheThreeKeyLockErasure is B4: deleting a claim's
// ledger record, its locked_at stamp and its own dependency baselines in ONE
// edit takes it out of every rule Audit has, because engineLocked reads exactly
// the two keys that were deleted alongside the record. The claim becomes an
// ordinary, freely-editable draft, and re-locking it afterwards writes a record
// indistinguishable from a first approval.
//
// The first half of this test asserts the gap in Audit deliberately, so the
// evidence for why AuditAgainstParent has to exist is executable rather than
// only argued in a comment.
func TestAuditAgainstParentCatchesTheThreeKeyLockErasure(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep"}
	victim := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget",
		Status: model.StatusLocked, Body: "the approved body", RestsOn: []string{dep.ID},
	}

	parent, parentDigests := historyCommit(t)
	honestLock(t, parent, dep)
	honestLock(t, parent, victim, dep)

	if findings := Audit([]model.Claim{dep, victim}, parent, parentDigests); len(findings) != 0 {
		t.Fatalf("the parent commit is honest and must be silent, got %+v", findings)
	}

	// THE ATTACK, in the commit under audit: three keys out of the lock store,
	// one line of YAML, and the body rewritten.
	commit, commitDigests := historyCommit(t)
	honestLock(t, commit, dep)
	tampered := victim
	tampered.Status = model.StatusDraft
	tampered.Body = "quietly rewritten"
	claims := []model.Claim{dep, tampered}

	// The gap, asserted: from this commit's directory alone there is nothing to
	// see, and the write path agrees — Lock would happily record a fresh approval
	// over the rewritten body.
	if findings := Audit(claims, commit, commitDigests); len(findings) != 0 {
		t.Fatalf("Audit is documented as unable to see this from one snapshot; if it now can, %s and this test are both out of date. Got %+v", RuleLockLedgerDeleted, findings)
	}
	if commit.LedgerRecordDeleted(tampered) {
		t.Fatalf("engineLocked is documented as blind to a three-key erasure; if it is not, this test is out of date")
	}

	// The parent commit still has every one of those keys.
	findings := AuditAgainstParent(claims, commit, parent, commitDigests, parentDigests)
	if !hasRule(findings, RuleLockLedgerDeleted) {
		t.Fatalf("expected %s for a claim whose whole lock history was deleted, got %+v", RuleLockLedgerDeleted, findings)
	}
	for _, f := range findings {
		if f.ClaimID == dep.ID {
			t.Fatalf("the untouched dependency must not be reported, got %+v", f)
		}
	}
}

// TestAuditAgainstParentCatchesTheDeletedClaimAndRecord: deleting a LOCKED
// claim's file is lock-ledger-abandoned — but that rule reads the record, so
// deleting the record in the same commit removes the rule's own evidence.
// Neither of Audit's directions can start from something that is gone.
func TestAuditAgainstParentCatchesTheDeletedClaimAndRecord(t *testing.T) {
	victim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}

	parent, parentDigests := historyCommit(t)
	honestLock(t, parent, victim)

	commit, commitDigests := historyCommit(t)

	if findings := Audit(nil, commit, commitDigests); len(findings) != 0 {
		t.Fatalf("Audit cannot see a claim and its record deleted together; got %+v", findings)
	}
	// It is reported as lock-ledger-abandoned: the claim's departure is the
	// finding, and that rule's recovery (restore, or unlock on the record and
	// delete again) is exactly the right one.
	if !hasRule(AuditAgainstParent(nil, commit, parent, commitDigests, parentDigests), RuleLockLedgerAbandoned) {
		t.Fatalf("expected %s when a locked claim and its whole approval history leave in one commit", RuleLockLedgerAbandoned)
	}
}

// TestAuditAgainstParentCatchesTheErasedCommentThread is B6: a human's open
// thread on a DRAFT claim, erased from the YAML together with the claim's key in
// the comment digest store.
//
// Erasing the block ALONE is not silent, and the first half asserts that too —
// the surviving entry recorded threads, the claim now hashes to the empty
// digest, and comment-ledger-drift fires. It is erasing BOTH that clears the
// board, and the lock the thread was blocking then succeeds.
func TestAuditAgainstParentCatchesTheErasedCommentThread(t *testing.T) {
	reviewed := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "this is not what we agreed",
		}},
	}

	parent, parentDigests := historyCommit(t)
	parentDigests.Record(reviewed)

	erased := reviewed
	erased.Comments = nil
	claims := []model.Claim{erased}

	// Half the attack: the block is gone, the entry is not. Already reported.
	commit, commitDigests := historyCommit(t)
	commitDigests.Record(reviewed)
	if !hasRule(Audit(claims, commit, commitDigests), RuleCommentLedgerDrift) {
		t.Fatalf("erasing the block alone must stay reported as %s", RuleCommentLedgerDrift)
	}

	// The whole attack: the entry goes too, and the claim reads exactly like one
	// nobody has ever commented on.
	commitDigests.Forget(reviewed.ID)
	if findings := Audit(claims, commit, commitDigests); len(findings) != 0 {
		t.Fatalf("Audit is documented as unable to see this from one snapshot; got %+v", findings)
	}
	if !hasRule(AuditAgainstParent(claims, commit, parent, commitDigests, parentDigests), RuleCommentLedgerDrift) {
		t.Fatalf("expected %s: the parent commit's entry is what proves the threads existed", RuleCommentLedgerDrift)
	}
}

// TestAuditAgainstParentIsSilentOnLegitimateWork is the half that matters most.
// A gate that fires on correct state is the outage the implicit grandfathering
// existed to prevent, and these are the sequences an honest project actually
// runs.
func TestAuditAgainstParentIsSilentOnLegitimateWork(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "approved"}

	t.Run("unlock, fix, lock", func(t *testing.T) {
		parent, parentDigests := historyCommit(t)
		honestLock(t, parent, claim)

		// The unlock commit: the record is RELEASED and kept.
		commit, commitDigests := historyCommit(t)
		honestLock(t, commit, claim)
		unlocked := Unlock(claim, commit, Approval{Actor: "alice", Reason: "needs a fix"})
		if f := AuditAgainstParent([]model.Claim{unlocked}, commit, parent, commitDigests, parentDigests); len(f) != 0 {
			t.Fatalf("an honest unlock must be silent, got %+v", f)
		}

		// The re-lock commit, from the unlock commit as parent.
		relocked := unlocked
		relocked.Status = model.StatusLocked
		relocked.Body = "approved, with the fix"
		after, afterDigests := historyCommit(t)
		honestLock(t, after, relocked)
		if f := AuditAgainstParent([]model.Claim{relocked}, after, commit, afterDigests, commitDigests); len(f) != 0 {
			t.Fatalf("an honest re-lock must be silent, got %+v", f)
		}
	})

	t.Run("unlock then delete the claim", func(t *testing.T) {
		parent, parentDigests := historyCommit(t)
		honestLock(t, parent, claim)
		Unlock(claim, parent, Approval{Actor: "alice", Reason: "dropping this"})

		// The claim file is gone; unlock kept the released record, as documented.
		commit, commitDigests := historyCommit(t)
		honestLock(t, commit, claim)
		Unlock(claim, commit, Approval{Actor: "alice", Reason: "dropping this"})
		if f := AuditAgainstParent(nil, commit, parent, commitDigests, parentDigests); len(f) != 0 {
			t.Fatalf("the documented restore -> unlock -> delete flow must be silent, got %+v", f)
		}
	})

	t.Run("the engine deletes the last thread", func(t *testing.T) {
		reviewed := claim
		reviewed.Comments = []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusResolved, Author: model.CommentRoleAgent,
			Created: "2026-07-24T10:00:00Z", Body: "typo, fixed",
		}}
		parent, parentDigests := historyCommit(t)
		honestLock(t, parent, reviewed)
		parentDigests.Record(reviewed)

		// digest.Store.Record REWRITES the entry to the empty digest; only Forget
		// removes one, and only for a claim that has left the project.
		emptied := reviewed
		emptied.Comments = nil
		commit, commitDigests := historyCommit(t)
		honestLock(t, commit, reviewed)
		commitDigests.Record(emptied)
		if f := AuditAgainstParent([]model.Claim{emptied}, commit, parent, commitDigests, parentDigests); len(f) != 0 {
			t.Fatalf("deleting a thread through the engine must be silent, got %+v", f)
		}
	})

	t.Run("the one-time migration", func(t *testing.T) {
		// A pre-ledger parent: locked_at, no records, no digest store.
		dir := t.TempDir()
		path := filepath.Join(dir, "store.json")
		if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.main":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
			t.Fatalf("write v1 store: %v", err)
		}
		parent, err := LoadStore(path)
		if err != nil {
			t.Fatalf("LoadStore: %v", err)
		}
		parentDigests, err := digest.LoadStore(digest.StorePathBeside(path))
		if err != nil {
			t.Fatalf("digest.LoadStore: %v", err)
		}

		// The migration commit: evidence only grows.
		commit, commitDigests := historyCommit(t)
		honestLock(t, commit, claim)
		if f := AuditAgainstParent([]model.Claim{claim}, commit, parent, commitDigests, parentDigests); len(f) != 0 {
			t.Fatalf("`migrate --adopt` only ever ADDS evidence and must be silent, got %+v", f)
		}
	})

	t.Run("no parent to compare against", func(t *testing.T) {
		commit, commitDigests := historyCommit(t)
		if f := AuditAgainstParent([]model.Claim{claim}, commit, nil, commitDigests, nil); f != nil {
			t.Fatalf("an initial commit or a shallow clone has no parent: missing evidence is never a finding, got %+v", f)
		}
		// A parent whose store file does not exist is the same case.
		empty, _ := LoadStore(filepath.Join(t.TempDir(), "store.json"))
		if f := AuditAgainstParent([]model.Claim{claim}, commit, empty, commitDigests, nil); f != nil {
			t.Fatalf("a parent with no lock store is not evidence of anything, got %+v", f)
		}
	})

	t.Run("a whole store absent on this side is said once elsewhere", func(t *testing.T) {
		parent, parentDigests := historyCommit(t)
		honestLock(t, parent, claim)
		parentDigests.Record(model.Claim{ID: claim.ID, Comments: []model.Comment{{ID: "c-aaaaaa", Body: "x"}}})

		// No lock store and no digest store in this commit: lock-ledger-absent and
		// comment-digest-absent are the project-scoped causes, said once each.
		absent, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
		if err != nil {
			t.Fatalf("LoadStore: %v", err)
		}
		absentDigests, err := digest.LoadStore(digest.StorePathBeside(absent.path))
		if err != nil {
			t.Fatalf("digest.LoadStore: %v", err)
		}
		if f := AuditAgainstParent([]model.Claim{claim}, absent, parent, absentDigests, parentDigests); len(f) != 0 {
			t.Fatalf("a whole-file absence must not be repeated per claim, got %+v", f)
		}
	})
}
