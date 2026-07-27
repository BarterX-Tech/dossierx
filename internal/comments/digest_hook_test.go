package comments

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// commentedBYAML is a claim whose comment thread was written before this
// project had a digest store — the fixture both the adoption test and the
// no-re-adoption test key off.
const commentedBYAML = `id: widget.contract.b
facet: contract
module: widget
status: draft
body: claim b
comments:
  - id: c-aaa111
    status: open
    author: human
    created: "2026-07-27T10:00:00Z"
    body: a thread written before the digest store existed
    edited: false
governed_by:
  type: none
  reason: fixture
`

// digestStore reloads the project's comment digest store from disk.
func (p *project) digestStore() *digest.Store {
	p.t.Helper()
	s, err := digest.LoadStore(digest.StorePath(p.cfg))
	if err != nil {
		p.t.Fatalf("digest.LoadStore: %v", err)
	}
	return s
}

// claimOnDisk reloads one claim from the project's claims dir.
func (p *project) claimOnDisk(id string) model.Claim {
	p.t.Helper()
	claims, err := loader.LoadClaims(p.claimsDir)
	if err != nil {
		p.t.Fatalf("LoadClaims: %v", err)
	}
	for _, c := range claims {
		if c.ID == id {
			return c
		}
	}
	p.t.Fatalf("claim %q not found on disk", id)
	return model.Claim{}
}

// TestEveryCommentOpRefreshesTheDigest walks the whole comment lifecycle
// through the ops and checks the digest tracks the file after each one. mutate
// is the single choke point every comment write in the product goes through —
// the CLI verbs and the serve HTTP handlers both — so one hook there covers all
// of them, and this test is what proves the hook is actually on the shared path
// rather than on one caller's.
func TestEveryCommentOpRefreshesTheDigest(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})

	assertTracked := func(after string) {
		t.Helper()
		on := p.claimOnDisk(claimA)
		recorded, ok := p.digestStore().Digest(claimA)
		if !ok {
			t.Fatalf("after %s: no digest recorded for %s", after, claimA)
		}
		if recorded != digest.CommentsDigest(on) {
			t.Fatalf("after %s: the recorded digest does not match the claim on disk", after)
		}
	}

	_, tid, err := p.deps().Add(claimA, model.CommentRoleAgent, "first thread")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertTracked("add")

	if _, _, err := p.deps().Reply(claimA, tid, model.CommentRoleHuman, "a reply"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	assertTracked("reply")

	if _, err := p.deps().Edit(claimA, tid, "", model.CommentRoleAgent, "first thread, reworded"); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	assertTracked("edit")

	if _, err := p.deps().Resolve(claimA, tid, model.CommentRoleAgent); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	assertTracked("resolve")

	if _, err := p.deps().Reopen(claimA, tid, model.CommentRoleAgent); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	assertTracked("reopen")

	if _, err := p.deps().Delete(claimA, tid, "", model.CommentRoleAgent); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	assertTracked("delete")

	// And the post-delete digest is the EMPTY-comments digest, recorded rather
	// than dropped: that is what makes a hand-added thread detectable on a
	// claim whose last thread was legitimately removed.
	if recorded, _ := p.digestStore().Digest(claimA); recorded != digest.CommentsDigest(p.claimOnDisk(claimA)) {
		t.Fatalf("the empty comment block must be recorded, not forgotten")
	}
}

// TestHandDeletingAThreadIsDetected is the rule this whole store exists for.
// A claim carrying an unresolved thread cannot lock — that is the comment gate
// in lock.Lock — so deleting the thread straight out of the YAML is how a claim
// gets locked with a review still open. The claim here is therefore a DRAFT: it
// is the state the bypass is performed from. Before the digest, nothing in the
// engine could tell it had happened.
func TestHandDeletingAThreadIsDetected(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})

	if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, "this is wrong, please fix"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	on := p.claimOnDisk(claimA)
	if len(on.OpenThreadIDs()) != 1 {
		t.Fatalf("precondition: expected one open thread on disk")
	}

	// The gate is quiet while the file matches the record.
	if findings := lock.Audit([]model.Claim{on}, nil, p.digestStore()); len(findings) != 0 {
		t.Fatalf("expected no findings before tampering, got %+v", findings)
	}

	// Someone opens the YAML and removes the thread.
	raw, err := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if err := os.WriteFile(filepath.Join(p.claimsDir, "a.yaml"), []byte(draftAYAML), 0o644); err != nil {
		t.Fatalf("rewrite claim: %v", err)
	}
	if string(raw) == draftAYAML {
		t.Fatalf("precondition: the comment op should have changed the file")
	}

	tampered := p.claimOnDisk(claimA)
	if len(tampered.Comments) != 0 {
		t.Fatalf("precondition: expected the hand edit to have removed the thread")
	}
	findings := lock.Audit([]model.Claim{tampered}, nil, p.digestStore())
	var found bool
	for _, f := range findings {
		if f.Rule == lock.RuleCommentLedgerDrift && f.ClaimID == claimA {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %s after an unresolved thread was deleted by hand, got %+v", lock.RuleCommentLedgerDrift, findings)
	}
}

// TestTheDigestNeverTouchesTheLockStore is the guarantee this package split
// exists to keep true: "dossierx serve" has no write authority over the lock
// store. Serve's comment handlers run the very ops driven here, so if the
// digest lived in the lock store, serving a browser would make the server a
// lock-store writer and the guarantee would be false the day it shipped.
func TestTheDigestNeverTouchesTheLockStore(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	lockStorePath := filepath.Join(p.root, ".dossierx-lock-store.json")

	if _, _, err := p.deps().Add(claimA, model.CommentRoleAgent, "a thread"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := os.Stat(lockStorePath); !os.IsNotExist(err) {
		t.Fatalf("a comment op created or wrote the lock store file (err=%v); comment writes must never reach it", err)
	}
	if _, err := os.Stat(digest.StorePath(p.cfg)); err != nil {
		t.Fatalf("expected the comment op to write its own digest store: %v", err)
	}
}

// TestDigestAdoptsThePreExistingThreadsOnFirstWrite: a project upgrading into
// this feature has comments already in its YAML. Recording only the claim being
// mutated would leave every OTHER commented claim uncovered until someone
// happened to comment on it, which in practice means forever.
func TestDigestAdoptsThePreExistingThreadsOnFirstWrite(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML, "b.yaml": commentedBYAML})

	// The first comment op anywhere in the project creates the store.
	if _, _, err := p.deps().Add(claimA, model.CommentRoleAgent, "a thread on a"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	recorded, ok := p.digestStore().Digest(claimB)
	if !ok {
		t.Fatalf("expected claim b's pre-existing threads adopted when the digest store was created")
	}
	if recorded != digest.CommentsDigest(p.claimOnDisk(claimB)) {
		t.Fatalf("adopted digest does not match claim b on disk")
	}
}

// TestCommentWriteRefusesALaunderedDigest is the gate on the OTHER side of the
// digest hook.
//
// Every comment op ends by re-recording the digest from whatever the file now
// says (recordCommentDigest). Without a check first, that refresh is a
// laundering machine: hand-delete an unresolved thread straight out of the YAML
// — which is how a claim gets past the lock gate with a review still open — then
// run ANY comment op on that claim, and the comment-ledger-drift finding that
// had named the edit is overwritten by a digest of the edited block. The
// integrity record would agree with the tampered file from then on, permanently,
// and the deleted review would be unrecoverable.
//
// So a mutating op on a claim whose stored block disagrees with its recorded
// digest is refused, and — the half that makes the refusal worth anything —
// nothing is written when it is.
func TestCommentWriteRefusesALaunderedDigest(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	p.addThread(model.CommentRoleHuman, "this contradicts the API facet")

	// The hand edit: strip the comments block out of the file entirely.
	before := p.readAYAML()
	stripped := before[:bytes.Index(before, []byte("comments:"))]
	if err := os.WriteFile(filepath.Join(p.claimsDir, "a.yaml"), stripped, 0o644); err != nil {
		t.Fatalf("hand-edit the claim: %v", err)
	}
	tamperedOnDisk := p.readAYAML()

	if _, _, err := p.deps().Add(claimA, model.CommentRoleAgent, "a write that would re-bless the edit"); !errors.Is(err, ErrCommentDigestDrift) {
		t.Fatalf("expected ErrCommentDigestDrift on a hand-edited comment block, got %v", err)
	}

	// Nothing written: not the claim, and — the point — not the digest.
	if got := p.readAYAML(); !bytes.Equal(got, tamperedOnDisk) {
		t.Fatalf("a refused comment write must not touch the claim file")
	}
	on := p.claimOnDisk(claimA)
	recorded, ok := p.digestStore().Digest(claimA)
	if !ok {
		t.Fatalf("the recorded digest disappeared")
	}
	if recorded == digest.CommentsDigest(on) {
		t.Fatalf("the refused write re-recorded the tampered block as the truth")
	}
}

// A claim the digest store has never seen is NOT drift. Unknown and drifted are
// different states, and conflating them would make the first comment on any new
// claim impossible — as well as every comment in a project that has not yet run
// a comment op.
func TestCommentWriteAllowsAClaimTheDigestHasNeverSeen(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, "the very first comment in this project"); err != nil {
		t.Fatalf("an uncovered claim must accept its first comment, got %v", err)
	}
	// And a second op on the now-covered, untampered claim still works.
	if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, "a second thread"); err != nil {
		t.Fatalf("a covered, untampered claim must keep accepting writes, got %v", err)
	}
}

// countThreadBodies counts how many times body appears as a thread/reply body in
// claimA's file on disk — the assertion that matters when a caller retries a
// failing op: the op must have written the comment either once or not at all,
// never once per attempt.
func (p *project) countThreadBodies(body string) int {
	p.t.Helper()
	n := 0
	for _, c := range p.claimOnDisk(claimA).Comments {
		if c.Body == body {
			n++
		}
		for _, r := range c.Replies {
			if r.Body == body {
				n++
			}
		}
	}
	return n
}

// TestUnreadableDigestStoreRefusesBeforeWriting is the duplication bug.
//
// The digest was refreshed AFTER the claim was saved, and every failure of that
// refresh was returned as a hard error — so a store truncated by a partial
// checkout, a merge conflict marker or a full disk produced "ok: false" for an
// op that HAD written the comment. The documented response to a failure is to
// retry, and each retry appended the same thread again while still reporting
// failure: the human's review thread filled with duplicates, and the comment
// ledger drifted from the file, which is the very condition the store exists to
// detect.
//
// The refusal now happens before anything is written, so two identical attempts
// leave zero copies on disk.
func TestUnreadableDigestStoreRefusesBeforeWriting(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	// One legitimate op, so the store exists and is covering the claim.
	p.addThread(model.CommentRoleHuman, "first")

	if err := os.WriteFile(digest.StorePath(p.cfg), []byte("{ truncated"), 0o644); err != nil {
		t.Fatalf("truncate digest store: %v", err)
	}

	const body = "second"
	for attempt := 1; attempt <= 2; attempt++ {
		_, _, err := p.deps().Add(claimA, model.CommentRoleHuman, body)
		if !errors.Is(err, ErrCommentDigestUnavailable) {
			t.Fatalf("attempt %d: expected ErrCommentDigestUnavailable, got %v", attempt, err)
		}
		if got := p.countThreadBodies(body); got != 0 {
			t.Fatalf("attempt %d: a refused comment op wrote the comment anyway (%d copies on disk)", attempt, got)
		}
	}
}

// The same property for a HELD sentinel — the other way the store becomes
// unavailable, and the one a crashed process leaves behind.
//
// It runs ONE attempt, not the two the tests above run, purely for wall-clock
// reasons: AcquireFileLock retries for ten seconds before giving up (a stale
// lock file must not wedge a project forever), and the property under test —
// refused, with nothing written — does not need a second ten-second wait to be
// demonstrated. Skipped under -short for the same reason.
func TestHeldDigestSentinelRefusesBeforeWriting(t *testing.T) {
	if testing.Short() {
		t.Skip("AcquireFileLock's timeout makes this a ten-second test")
	}
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	p.addThread(model.CommentRoleHuman, "first")

	lockPath := digest.StorePath(p.cfg) + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("hold the digest sentinel: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(lockPath) }) //nolint:errcheck // best-effort cleanup of the test's own sentinel

	const body = "blocked"
	if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, body); !errors.Is(err, ErrCommentDigestUnavailable) {
		t.Fatalf("expected ErrCommentDigestUnavailable while the digest sentinel is held, got %v", err)
	}
	if got := p.countThreadBodies(body); got != 0 {
		t.Fatalf("a refused comment op wrote the comment anyway (%d copies on disk)", got)
	}
}

// A read-only project directory — the full-disk / read-only-checkout shape.
//
// The sentinel that refuses here is the CLAIMS one, which lives in the same
// directory and so cannot be taken either; the assertion is deliberately about
// the PROPERTY rather than about which gate produced it, because that property
// is the whole fix: a refused comment op leaves nothing behind, so a caller that
// retries does not accumulate copies. (The digest store's own writability is
// probed by digest.Store.CheckWritable, unit-tested in internal/digest, for the
// case where the store's directory is the one that cannot take a write.)
func TestReadOnlyProjectDirWritesNothingOnEitherAttempt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not gate file creation on Windows")
	}
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	p.addThread(model.CommentRoleHuman, "first")

	if err := os.Chmod(p.root, 0o555); err != nil {
		t.Fatalf("make the project dir read-only: %v", err)
	}
	t.Cleanup(func() { os.Chmod(p.root, 0o755) }) //nolint:errcheck // best-effort restore so TempDir cleanup works

	const body = "on a read-only project dir"
	for attempt := 1; attempt <= 2; attempt++ {
		if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, body); err == nil {
			t.Fatalf("attempt %d: expected the op to be refused", attempt)
		}
		if got := p.countThreadBodies(body); got != 0 {
			t.Fatalf("attempt %d: a refused comment op wrote the comment anyway (%d copies on disk)", attempt, got)
		}
	}
}

// A comment op in a LEDGER-COVERED project must not re-adopt a missing digest
// store.
//
// The adoption branch exists for projects upgrading INTO the digest feature: the
// store is absent, so every claim's current block is recorded at once. In a
// project that has ALREADY been through a ledger-aware build, an absent store is
// not an upgrade — it is a deleted file, and internal/check reports it as
// comment-digest-absent. Adopting there would record a hand-edited comment block
// as the truth and clear the finding that named it: rm the store, run any comment
// op anywhere in the project, and the deleted review thread is blessed forever.
func TestNoReAdoptionInALedgerCoveredProject(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML, "b.yaml": commentedBYAML})

	// Make the project ledger-covered the way any locking build does: a lock
	// store on disk at the current schema.
	lockStorePath := filepath.Join(p.root, ".dossierx-lock-store.json")
	store, err := lock.LoadStore(lockStorePath)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("save lock store: %v", err)
	}
	covered, err := lock.LoadStore(lockStorePath)
	if err != nil {
		t.Fatalf("reload lock store: %v", err)
	}
	if !covered.LedgerCovered() {
		t.Fatalf("precondition: expected the project to read as ledger-covered")
	}
	p.store = covered

	// The first comment op in the project, with no digest store on disk.
	if _, _, err := p.deps().Add(claimA, model.CommentRoleAgent, "a thread on a"); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, ok := p.digestStore().Digest(claimB); ok {
		t.Fatalf("claim b's comment block was adopted in a ledger-covered project: a deleted digest store must not be re-blessed by an ordinary write")
	}
	if _, ok := p.digestStore().Digest(claimA); !ok {
		t.Fatalf("the claim actually written must still be recorded")
	}
}
