package comments

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// THE WEDGE. The write path saves the claim first and refreshes the digest
// second (Record explains why that order is the only safe one), so a crash in
// between leaves a digest that LAGS the file — and so does an ordinary commit
// that carries the claim file without .dossierx-comment-digest.json, which
// reproduces the same state for every teammate who pulls.
//
// From there EVERY comment op on that claim is refused, because checkCommentDigest
// runs before fn inside the same mutate. The doc comment used to call that "a
// false positive a human can see and clear by re-running the gate after any
// comment op", which is false by construction, and the refusal's own advice
// ("restore the claim file from version control") throws away the comment the
// human actually wrote. There was no other verb.
//
// This test reproduces the lag by restoring the PRE-op digest store, asserts
// that the wedge is real, and then asserts that the recovery clears it without
// touching the thread.
func TestReauditDigest_ClearsALaggingDigestWithoutLosingTheThread(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})

	// One honest thread, so the store covers this claim.
	tid := p.addThread(model.CommentRoleHuman, "please clarify this")

	before, err := os.ReadFile(digest.StorePath(p.cfg))
	if err != nil {
		t.Fatalf("read digest store: %v", err)
	}

	// A second honest op, then the crash: the claim file keeps the reply and the
	// digest store is rolled back to what it said before it.
	if _, _, err := p.deps().Reply(claimA, tid, model.CommentRoleAgent, "here is the clarification"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if err := os.WriteFile(digest.StorePath(p.cfg), before, 0o644); err != nil {
		t.Fatalf("restore pre-op digest store: %v", err)
	}

	// The wedge: every mutating op is refused, including the one an agent would
	// reach for next.
	if _, _, err := p.deps().Add(claimA, model.CommentRoleAgent, "another note"); !errors.Is(err, ErrCommentDigestDrift) {
		t.Fatalf("expected the lagging digest to refuse the next comment op, got %v", err)
	} else if !strings.Contains(err.Error(), digest.StoreFileName) {
		// ...and the refusal must name the recovery a reader can actually run.
		// It used to name `dossierx comment reaudit`, which this recovery IS but
		// which no CLI verb reaches, so a wedged reader got `unknown command`.
		// digest_refusal_test.go owns that property in full; this line only
		// keeps the wedge and its message tied together in one place.
		t.Fatalf("the refusal must name the store to restore, got %q", err.Error())
	}

	// The recovery. It requires the human's words, exactly as claim unlock does.
	if _, err := p.deps().ReauditDigest(claimA, model.CommentRoleHuman, "  "); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("expected a blank reason to be refused, got %v", err)
	}
	claim, err := p.deps().ReauditDigest(claimA, model.CommentRoleHuman, "the reply on disk is the one I wrote")
	if err != nil {
		t.Fatalf("ReauditDigest: %v", err)
	}

	// The thread — root and reply — is intact: this writes no claim file.
	if len(claim.Comments) != 1 || len(claim.Comments[0].Replies) != 1 {
		t.Fatalf("the recovery must not touch the claim, got %+v", claim.Comments)
	}

	// The gate is green again, by the same predicate lock.Audit uses.
	on := p.claimOnDisk(claimA)
	recorded, known := p.digestStore().Digest(claimA)
	if !known || recorded != digest.CommentsDigest(on) {
		t.Fatalf("expected the digest to match the claim on disk after the recovery")
	}

	// And ordinary comment ops work again.
	if _, _, err := p.deps().Add(claimA, model.CommentRoleAgent, "another note"); err != nil {
		t.Fatalf("expected comment ops to work after the recovery, got %v", err)
	}
}

// The recovery is the one operation that can make an integrity finding
// disappear, so it leaves evidence in the tracked file that it happened: who
// asked for it, when, in whose words, and which value it adopted.
func TestReauditDigest_RecordsWhoAuthorisedIt(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	p.addThread(model.CommentRoleHuman, "please clarify this")

	if _, err := p.deps().ReauditDigest(claimA, model.CommentRoleHuman, "I wrote that thread by hand and it is correct"); err != nil {
		t.Fatalf("ReauditDigest: %v", err)
	}

	store := p.digestStore()
	entries := store.Reaudits[claimA]
	if len(entries) != 1 {
		t.Fatalf("expected exactly one recorded re-adoption, got %+v", store.Reaudits)
	}
	e := entries[0]
	if e.Reason != "I wrote that thread by hand and it is correct" {
		t.Fatalf("the human's words must be on the record, got %q", e.Reason)
	}
	if e.Actor != string(model.CommentRoleHuman) || e.At == "" {
		t.Fatalf("expected actor and timestamp recorded, got %+v", e)
	}
	if recorded, _ := store.Digest(claimA); e.Digest != recorded {
		t.Fatalf("the record must name the value it adopted, got %q vs %q", e.Digest, recorded)
	}
}

// An unknown claim id is refused with the shared sentinel, so the CLI classifies
// it the same way every other comment verb's unknown id is classified.
func TestReauditDigest_UnknownClaimIsRefused(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	if _, err := p.deps().ReauditDigest("widget.contract.nope", model.CommentRoleHuman, "because"); !errors.Is(err, ErrClaimNotFound) {
		t.Fatalf("expected ErrClaimNotFound, got %v", err)
	}
}
