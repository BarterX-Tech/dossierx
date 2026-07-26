package comments

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

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
	const commentedB = `id: widget.contract.b
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
	p := newProject(t, map[string]string{"a.yaml": draftAYAML, "b.yaml": commentedB})

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
