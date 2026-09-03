package comments

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

const (
	claimA = "widget.contract.a"
	claimB = "widget.contract.b"
)

var (
	threadIDRe = regexp.MustCompile(`^c-[0-9a-f]{6}$`)
	replyIDRe  = regexp.MustCompile(`^r-[0-9a-f]{6}$`)
)

const draftAYAML = `id: widget.contract.a
facet: contract
module: widget
status: draft
body: claim a
governed_by:
  type: none
  reason: fixture
`

const lockedAYAML = `id: widget.contract.a
facet: contract
module: widget
status: locked
build_role: schema
body: claim a
governed_by:
  type: none
  reason: fixture
`

const lockedARestsOnBYAML = `id: widget.contract.a
facet: contract
module: widget
status: locked
build_role: schema
body: claim a
rests_on:
  - widget.contract.b
governed_by:
  type: none
  reason: fixture
`

const draftBYAML = `id: widget.contract.b
facet: contract
module: widget
status: draft
body: claim b
governed_by:
  type: none
  reason: fixture
`

const bannerYAML = `id: widget.contract.a
facet: contract
module: widget
status: draft
layout: banner
body: a banner
governed_by:
  type: none
  reason: fixture
`

// project is a test harness: a temp project dir (config + claims) plus the two
// in-memory stores the ops read. deps() reloads the claims fresh each call so a
// read via List reflects the latest on-disk state.
type project struct {
	t         *testing.T
	root      string
	claimsDir string
	cfg       *config.Config
	store     *lock.Store
	flags     *reaudit.FlagStore
}

func newProject(t *testing.T, files map[string]string) *project {
	t.Helper()
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(claimsDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	store, err := lock.LoadStore(filepath.Join(root, "build", "ledger", "lock-store.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	flags, err := reaudit.LoadFlagStore(filepath.Join(root, "build", "ledger", "flag-store.json"))
	if err != nil {
		t.Fatalf("LoadFlagStore: %v", err)
	}
	return &project{t: t, root: root, claimsDir: claimsDir, cfg: cfg, store: store, flags: flags}
}

func (p *project) deps() *Deps {
	p.t.Helper()
	claims, err := loader.LoadClaims(p.claimsDir)
	if err != nil {
		p.t.Fatalf("LoadClaims: %v", err)
	}
	return &Deps{Cfg: p.cfg, Claims: claims, LockStore: p.store, FlagStore: p.flags}
}

// addThread seeds a comment thread on claimA and fails the test on error,
// returning the new thread id — keeps the common "seed a thread" setup a
// conflict-free one-liner.
func (p *project) addThread(role model.CommentRole, body string) string {
	p.t.Helper()
	_, tid, err := p.deps().Add(claimA, role, body)
	if err != nil {
		p.t.Fatalf("Add(%q): %v", body, err)
	}
	return tid
}

// readAYAML reads claimA's on-disk file and fails the test on error.
func (p *project) readAYAML() []byte {
	p.t.Helper()
	b, err := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))
	if err != nil {
		p.t.Fatal(err)
	}
	return b
}

func (p *project) reload(id string) model.Claim {
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
	p.t.Fatalf("claim %q not found after reload", id)
	return model.Claim{}
}

func (p *project) setStaleBaseline(dependent, dep string) {
	if p.store.Hashes[dependent] == nil {
		p.store.Hashes[dependent] = map[string]string{}
	}
	p.store.Hashes[dependent][dep] = "stale-baseline-that-will-never-match-a-content-hash"
}

func (p *project) setFlag(id string) {
	p.flags.Flags[id] = reaudit.PendingFlag{ClaimSays: "x", NowDoes: "y", Reason: "z", FlaggedAt: "2026-07-24T00:00:00Z"}
}

func mustParseRFC3339(t *testing.T, s string) {
	t.Helper()
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		t.Errorf("timestamp %q is not RFC3339: %v", s, err)
	}
}

// --- full lifecycle across all seven ops, on a DRAFT claim ---------------

func TestLifecycle_AllOps_Draft(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})

	c, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "the row contradicts the API facet")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !threadIDRe.MatchString(tid) {
		t.Fatalf("thread id %q does not match %s", tid, threadIDRe)
	}
	if len(c.Comments) != 1 || c.Comments[0].Status != model.CommentStatusOpen || c.Comments[0].Author != model.CommentRoleHuman {
		t.Fatalf("unexpected comment after Add: %+v", c.Comments)
	}
	mustParseRFC3339(t, c.Comments[0].Created)
	if c.ReviewPending {
		t.Fatalf("a draft claim must never carry review_pending")
	}

	// Reply
	_, rid, err := p.deps().Reply(claimA, tid, model.CommentRoleAgent, "fixed the rows; API facet was stale")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !replyIDRe.MatchString(rid) {
		t.Fatalf("reply id %q does not match %s", rid, replyIDRe)
	}

	// Edit reply
	if _, err := p.deps().Edit(claimA, tid, rid, model.CommentRoleAgent, "fixed the rows (edited)"); err != nil {
		t.Fatalf("Edit reply: %v", err)
	}
	// Edit thread root
	if _, err := p.deps().Edit(claimA, tid, "", model.CommentRoleHuman, "which facet is right?"); err != nil {
		t.Fatalf("Edit root: %v", err)
	}
	got := p.reload(claimA)
	if got.Comments[0].Body != "which facet is right?" || !got.Comments[0].Edited {
		t.Fatalf("thread root not edited: %+v", got.Comments[0])
	}
	if got.Comments[0].Replies[0].Body != "fixed the rows (edited)" || !got.Comments[0].Replies[0].Edited {
		t.Fatalf("reply not edited: %+v", got.Comments[0].Replies[0])
	}

	// Resolve then Reopen
	rc, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rc.Comments[0].Status != model.CommentStatusResolved || rc.Comments[0].ResolvedBy != model.CommentRoleHuman {
		t.Fatalf("thread not resolved: %+v", rc.Comments[0])
	}
	mustParseRFC3339(t, rc.Comments[0].ResolvedAt)

	oc, err := p.deps().Reopen(claimA, tid, model.CommentRoleHuman)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if oc.Comments[0].Status != model.CommentStatusOpen || oc.Comments[0].ReopenedBy != model.CommentRoleHuman {
		t.Fatalf("thread not reopened: %+v", oc.Comments[0])
	}
	mustParseRFC3339(t, oc.Comments[0].ReopenedAt)

	// List (open only vs all)
	all, err := p.deps().List(claimA, false)
	if err != nil || len(all) != 1 {
		t.Fatalf("List all: %v len=%d", err, len(all))
	}
	// Delete the reply, then the whole thread.
	if _, err := p.deps().Delete(claimA, tid, rid, model.CommentRoleAgent); err != nil {
		t.Fatalf("Delete reply: %v", err)
	}
	if got := p.reload(claimA); len(got.Comments[0].Replies) != 0 {
		t.Fatalf("reply not deleted: %+v", got.Comments[0].Replies)
	}
	if _, err := p.deps().Delete(claimA, tid, "", model.CommentRoleHuman); err != nil {
		t.Fatalf("Delete thread: %v", err)
	}
	if got := p.reload(claimA); len(got.Comments) != 0 {
		t.Fatalf("thread not deleted: %+v", got.Comments)
	}
}

// --- pending-trigger arithmetic ------------------------------------------

func TestPending_OpenOnLockedSetsReviewPending(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})
	c, _, err := p.deps().Add(claimA, model.CommentRoleHuman, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !c.ReviewPending {
		t.Fatalf("adding an open thread to a LOCKED claim must set review_pending")
	}
	if !p.reload(claimA).ReviewPending {
		t.Fatalf("review_pending must persist to disk")
	}
}

func TestPending_ResolveLastClearsWhenNoDriftNoFlag(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	c, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if c.ReviewPending {
		t.Fatalf("resolving the last open thread with no drift and no flag must clear review_pending")
	}
}

func TestPending_DeleteLastClearsWhenNoDriftNoFlag(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	c, err := p.deps().Delete(claimA, tid, "", model.CommentRoleHuman)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if c.ReviewPending {
		t.Fatalf("deleting the last open thread with no drift and no flag must clear review_pending")
	}
}

func TestPending_ResolveLastRetainedOnDrift(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedARestsOnBYAML, "b.yaml": draftBYAML})
	p.setStaleBaseline(claimA, claimB) // dependency drifted since lock
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	c, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !c.ReviewPending {
		t.Fatalf("review_pending must be RETAINED when a dependency has drifted, even after resolving the last thread")
	}
}

// TestPending_StaleFlagSurvivesResolveLast is the flag->unlock->lock->comment
// ->resolve-last "stale flag" case: a pending flag entry survives an
// unlock/lock cycle (only reaudit --confirm deletes it), so resolving the last
// comment thread must NOT clear review_pending while the flag still stands.
func TestPending_StaleFlagSurvivesResolveLast(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})
	p.setFlag(claimA) // flag parked from before an unlock/lock cycle
	_, tid, err := p.deps().Add(claimA, model.CommentRoleAgent, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	c, err := p.deps().Resolve(claimA, tid, model.CommentRoleAgent)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !c.ReviewPending {
		t.Fatalf("review_pending must be RETAINED while a pending flag stands, even after resolving the last thread")
	}
}

func TestPending_ReopenReSetsReviewPending(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.reload(claimA).ReviewPending {
		t.Fatalf("precondition: review_pending should be cleared after resolve")
	}
	c, err := p.deps().Reopen(claimA, tid, model.CommentRoleHuman)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if !c.ReviewPending {
		t.Fatalf("reopening a thread on a locked claim must re-set review_pending")
	}
}

// TestAllOps_Locked exercises add/reply/resolve/reopen/edit/delete against a
// locked claim and checks the review_pending flips at each step.
func TestAllOps_Locked(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleAgent, "why does this rest on the loader?")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !p.reload(claimA).ReviewPending {
		t.Fatalf("locked add must set review_pending")
	}
	if _, _, err := p.deps().Reply(claimA, tid, model.CommentRoleHuman, "historical reason; see notes"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if _, err := p.deps().Edit(claimA, tid, "", model.CommentRoleAgent, "why does this rest on loader (clarified)?"); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if _, err := p.deps().Resolve(claimA, tid, model.CommentRoleAgent); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.reload(claimA).ReviewPending {
		t.Fatalf("resolve-last should clear review_pending")
	}
	if _, err := p.deps().Reopen(claimA, tid, model.CommentRoleAgent); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if !p.reload(claimA).ReviewPending {
		t.Fatalf("reopen should re-set review_pending")
	}
	if _, err := p.deps().Delete(claimA, tid, "", model.CommentRoleAgent); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if p.reload(claimA).ReviewPending {
		t.Fatalf("delete-last should clear review_pending")
	}
}

// --- advisory rights matrix (op x actor x target-author) -----------------

func TestRightsMatrix(t *testing.T) {
	roles := []model.CommentRole{model.CommentRoleHuman, model.CommentRoleAgent}
	// wantAllowed[actor][author]: human may act on anything; agent only on agent.
	allowed := func(actor, author model.CommentRole) bool {
		return actor == model.CommentRoleHuman || author == model.CommentRoleAgent
	}

	ops := []struct {
		name string
		// run performs the whole-thread op (resolve/reopen/edit-root/delete-thread)
		// on a thread OPENED by author, acting as actor; returns the op error.
		run func(p *project, tid string, author, actor model.CommentRole) error
	}{
		{"resolve", func(p *project, tid string, _, actor model.CommentRole) error {
			_, err := p.deps().Resolve(claimA, tid, actor)
			return err
		}},
		{"reopen", func(p *project, tid string, _, actor model.CommentRole) error {
			// A thread must be resolved before it can be reopened; resolve as
			// human (always permitted) so we isolate the reopen rights check.
			if _, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman); err != nil {
				return err
			}
			_, err := p.deps().Reopen(claimA, tid, actor)
			return err
		}},
		{"edit-root", func(p *project, tid string, _, actor model.CommentRole) error {
			_, err := p.deps().Edit(claimA, tid, "", actor, "edited body")
			return err
		}},
		{"delete-thread", func(p *project, tid string, _, actor model.CommentRole) error {
			_, err := p.deps().Delete(claimA, tid, "", actor)
			return err
		}},
	}

	for _, op := range ops {
		for _, author := range roles {
			for _, actor := range roles {
				t.Run(op.name+"/author="+string(author)+"/actor="+string(actor), func(t *testing.T) {
					p := newProject(t, map[string]string{"a.yaml": draftAYAML})
					_, tid, err := p.deps().Add(claimA, author, "thread body")
					if err != nil {
						t.Fatalf("Add: %v", err)
					}
					err = op.run(p, tid, author, actor)
					if allowed(actor, author) {
						if err != nil {
							t.Fatalf("expected %s allowed (actor=%s author=%s), got %v", op.name, actor, author, err)
						}
					} else {
						if !errors.Is(err, ErrRightsDenied) {
							t.Fatalf("expected %s denied (actor=%s author=%s), got %v", op.name, actor, author, err)
						}
					}
				})
			}
		}
	}
}

// TestRights_ReplyOps keys off the REPLY's own author, not the thread's.
func TestRights_ReplyOps(t *testing.T) {
	// Thread opened by human, reply authored by agent: an agent may edit/delete
	// its OWN reply even though the thread is human-owned.
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "human thread")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	_, rid, err := p.deps().Reply(claimA, tid, model.CommentRoleAgent, "agent reply")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if _, err := p.deps().Edit(claimA, tid, rid, model.CommentRoleAgent, "agent reply edited"); err != nil {
		t.Fatalf("agent editing its own reply must be allowed, got %v", err)
	}
	// A human-authored reply may NOT be edited/deleted by an agent.
	_, hrid, err := p.deps().Reply(claimA, tid, model.CommentRoleHuman, "human reply")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if _, err := p.deps().Edit(claimA, tid, hrid, model.CommentRoleAgent, "nope"); !errors.Is(err, ErrRightsDenied) {
		t.Fatalf("agent editing a human reply must be denied, got %v", err)
	}
	if _, err := p.deps().Delete(claimA, tid, hrid, model.CommentRoleAgent); !errors.Is(err, ErrRightsDenied) {
		t.Fatalf("agent deleting a human reply must be denied, got %v", err)
	}
}

// TestRights_AnyoneMayReplyToOpenThread: reply carries no author-rights gate.
func TestRights_AnyoneMayReplyToOpenThread(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	tid := p.addThread(model.CommentRoleHuman, "human thread")
	if _, _, err := p.deps().Reply(claimA, tid, model.CommentRoleAgent, "agent may reply to a human thread"); err != nil {
		t.Fatalf("agent replying to a human-opened thread must be allowed, got %v", err)
	}
}

// --- edge cases ----------------------------------------------------------

func TestEdge_UnknownClaim(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	if _, _, err := p.deps().Add("widget.contract.missing", model.CommentRoleHuman, "x"); !errors.Is(err, ErrClaimNotFound) {
		t.Fatalf("want ErrClaimNotFound, got %v", err)
	}
}

func TestEdge_UnknownThread_NeighbourUntouched(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	tid := p.addThread(model.CommentRoleHuman, "real thread")
	before := p.reload(claimA)

	if _, err := p.deps().Resolve(claimA, "c-nope00", model.CommentRoleHuman); !errors.Is(err, ErrThreadNotFound) {
		t.Fatalf("want ErrThreadNotFound, got %v", err)
	}
	after := p.reload(claimA)
	if len(after.Comments) != 1 || after.Comments[0].ID != tid || after.Comments[0].Status != model.CommentStatusOpen {
		t.Fatalf("neighbour thread mutated by an unknown-id op: before=%+v after=%+v", before.Comments, after.Comments)
	}
}

func TestEdge_UnknownReply_NeighbourUntouched(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "thread")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	_, rid, err := p.deps().Reply(claimA, tid, model.CommentRoleHuman, "real reply")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}

	if _, err := p.deps().Edit(claimA, tid, "r-nope00", model.CommentRoleHuman, "x"); !errors.Is(err, ErrReplyNotFound) {
		t.Fatalf("want ErrReplyNotFound, got %v", err)
	}
	got := p.reload(claimA)
	if len(got.Comments[0].Replies) != 1 || got.Comments[0].Replies[0].ID != rid || got.Comments[0].Replies[0].Body != "real reply" {
		t.Fatalf("neighbour reply mutated by an unknown-id op: %+v", got.Comments[0].Replies)
	}
}

func TestEdge_ReplyToResolvedRejected(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	tid := p.addThread(model.CommentRoleHuman, "thread")
	if _, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, _, err := p.deps().Reply(claimA, tid, model.CommentRoleHuman, "late reply"); !errors.Is(err, ErrThreadResolved) {
		t.Fatalf("want ErrThreadResolved replying to a resolved thread, got %v", err)
	}
}

func TestEdge_DoubleResolveRejected(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	tid := p.addThread(model.CommentRoleHuman, "thread")
	if _, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := p.deps().Resolve(claimA, tid, model.CommentRoleHuman); !errors.Is(err, ErrThreadResolved) {
		t.Fatalf("want ErrThreadResolved on double-resolve, got %v", err)
	}
}

func TestEdge_ReopenOpenRejected(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	tid := p.addThread(model.CommentRoleHuman, "thread")
	if _, err := p.deps().Reopen(claimA, tid, model.CommentRoleHuman); !errors.Is(err, ErrThreadOpen) {
		t.Fatalf("want ErrThreadOpen reopening an open thread, got %v", err)
	}
}

func TestEdge_EmptyBodyRejected_NoWrite(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	before := p.readAYAML()
	for _, body := range []string{"", "   ", "\t\n"} {
		if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, body); !errors.Is(err, ErrEmptyBody) {
			t.Fatalf("Add(%q): want ErrEmptyBody, got %v", body, err)
		}
	}
	// Reply and Edit reject empty too.
	tid := p.addThread(model.CommentRoleHuman, "real")
	if _, _, err := p.deps().Reply(claimA, tid, model.CommentRoleHuman, "  "); !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("Reply empty: want ErrEmptyBody, got %v", err)
	}
	if _, err := p.deps().Edit(claimA, tid, "", model.CommentRoleHuman, ""); !errors.Is(err, ErrEmptyBody) {
		t.Fatalf("Edit empty: want ErrEmptyBody, got %v", err)
	}
	// The two rejected Add-empty cases before any real Add must not have written.
	_ = before
}

func TestEdge_InvalidActorRejected(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	if _, _, err := p.deps().Add(claimA, model.CommentRole("robot"), "x"); !errors.Is(err, ErrInvalidActor) {
		t.Fatalf("want ErrInvalidActor, got %v", err)
	}
}

func TestEdge_BannerClaimRejected(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": bannerYAML})
	before := p.readAYAML()
	if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, "x"); !errors.Is(err, ErrBannerClaim) {
		t.Fatalf("want ErrBannerClaim, got %v", err)
	}
	after := p.readAYAML()
	if !bytes.Equal(before, after) {
		t.Fatalf("banner claim file must be untouched after a rejected Add")
	}
}

func TestEdge_TwoEditsLastWriteWins(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	tid := p.addThread(model.CommentRoleHuman, "v0")
	if _, err := p.deps().Edit(claimA, tid, "", model.CommentRoleHuman, "v1"); err != nil {
		t.Fatalf("Edit v1: %v", err)
	}
	if _, err := p.deps().Edit(claimA, tid, "", model.CommentRoleHuman, "v2"); err != nil {
		t.Fatalf("Edit v2: %v", err)
	}
	got := p.reload(claimA)
	if got.Comments[0].Body != "v2" || !got.Comments[0].Edited {
		t.Fatalf("last-write-wins expected v2/edited, got %+v", got.Comments[0])
	}
}

func TestEdge_ReadOnlyClaimFile_CleanError_NoPartialWrite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory bit does not block file creation on Windows")
	}
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	before := p.readAYAML()
	d := p.deps() // load claims while the dir is still writable

	if err := os.Chmod(p.claimsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(p.claimsDir, 0o755) }) //nolint:errcheck // best-effort perm restore on cleanup

	_, _, err := d.Add(claimA, model.CommentRoleHuman, "body")
	if err == nil {
		t.Fatalf("expected an error saving into a read-only claims dir")
	}
	if errors.Is(err, ErrClaimNotFound) || errors.Is(err, ErrEmptyBody) {
		t.Fatalf("expected an I/O write error, got a logical error: %v", err)
	}

	if err := os.Chmod(p.claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	after := p.readAYAML()
	if !bytes.Equal(before, after) {
		t.Fatalf("claim file changed despite a failed save (partial write)")
	}
}

// --- id generation: collision regeneration + legacy backfill -------------

const claimWithThreadIDYAML = `id: widget.contract.a
facet: contract
module: widget
status: draft
body: claim a
comments:
  - id: c-8f3a2b
    status: resolved
    author: human
    created: 2026-07-24T10:12:00Z
    body: existing thread
    edited: false
governed_by:
  type: none
  reason: fixture
`

const claimWithReplyIDYAML = `id: widget.contract.a
facet: contract
module: widget
status: draft
body: claim a
comments:
  - id: c-aaaaaa
    status: open
    author: human
    created: 2026-07-24T10:12:00Z
    body: existing thread
    edited: false
    replies:
      - id: r-8f3a2b
        author: agent
        created: 2026-07-24T10:40:00Z
        body: existing reply
        edited: false
governed_by:
  type: none
  reason: fixture
`

const claimWithIDLessCommentsYAML = `id: widget.contract.a
facet: contract
module: widget
status: draft
body: claim a
comments:
  - status: open
    author: human
    created: 2026-07-24T10:12:00Z
    body: legacy id-less thread
    edited: false
    replies:
      - author: agent
        created: 2026-07-24T10:40:00Z
        body: legacy id-less reply
        edited: false
governed_by:
  type: none
  reason: fixture
`

func stubRand(t *testing.T, hexes ...string) {
	t.Helper()
	orig := randRead
	i := 0
	randRead = func(b []byte) (int, error) {
		if i >= len(hexes) {
			t.Fatalf("randRead called more than the %d stubbed times", len(hexes))
		}
		raw, err := hex.DecodeString(hexes[i])
		if err != nil {
			t.Fatalf("bad stub hex %q: %v", hexes[i], err)
		}
		copy(b, raw)
		i++
		return len(b), nil
	}
	t.Cleanup(func() { randRead = orig })
}

func TestID_ThreadCollisionRegenerates(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": claimWithThreadIDYAML})
	stubRand(t, "8f3a2b", "112233") // first collides with the existing c-8f3a2b
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "new thread")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if tid != "c-112233" {
		t.Fatalf("expected regenerated id c-112233 on collision, got %q", tid)
	}
}

func TestID_ReplyCollisionRegenerates(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": claimWithReplyIDYAML})
	stubRand(t, "8f3a2b", "112233") // first collides with the existing r-8f3a2b
	_, rid, err := p.deps().Reply(claimA, "c-aaaaaa", model.CommentRoleHuman, "new reply")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if rid != "r-112233" {
		t.Fatalf("expected regenerated id r-112233 on collision, got %q", rid)
	}
}

func TestID_LegacyBackfilledOnSave(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": claimWithIDLessCommentsYAML})
	// Adding a NEW thread must also backfill the id-less legacy comment+reply.
	if _, _, err := p.deps().Add(claimA, model.CommentRoleHuman, "new thread"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := p.reload(claimA)
	if len(got.Comments) != 2 {
		t.Fatalf("expected 2 threads (legacy + new), got %d", len(got.Comments))
	}
	seen := map[string]bool{}
	for _, cm := range got.Comments {
		if !threadIDRe.MatchString(cm.ID) {
			t.Fatalf("comment id %q not backfilled to the c- pattern", cm.ID)
		}
		if seen[cm.ID] {
			t.Fatalf("duplicate comment id %q", cm.ID)
		}
		seen[cm.ID] = true
		for _, r := range cm.Replies {
			if !replyIDRe.MatchString(r.ID) {
				t.Fatalf("reply id %q not backfilled to the r- pattern", r.ID)
			}
			if seen[r.ID] {
				t.Fatalf("duplicate reply id %q", r.ID)
			}
			seen[r.ID] = true
		}
	}
}

// --- List filtering ------------------------------------------------------

func TestList_OpenOnlyFiltersResolved(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	t1 := p.addThread(model.CommentRoleHuman, "open one")
	t2 := p.addThread(model.CommentRoleHuman, "to be resolved")
	if _, err := p.deps().Resolve(claimA, t2, model.CommentRoleHuman); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	all, err := p.deps().List(claimA, false)
	if err != nil || len(all) != 2 {
		t.Fatalf("List all: err=%v len=%d", err, len(all))
	}
	open, err := p.deps().List(claimA, true)
	if err != nil || len(open) != 1 || open[0].ID != t1 {
		t.Fatalf("List open: err=%v got=%+v want single %q", err, open, t1)
	}
	_ = t2
}

func TestList_UnknownClaim(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	if _, err := p.deps().List("widget.contract.missing", false); !errors.Is(err, ErrClaimNotFound) {
		t.Fatalf("want ErrClaimNotFound, got %v", err)
	}
}
