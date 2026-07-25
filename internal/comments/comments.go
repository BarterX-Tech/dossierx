package comments

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

// Structured, matchable (errors.Is) failure sentinels. Callers — the CLI and,
// in a later phase, the serve HTTP handlers — map these to exit codes / status
// codes (e.g. thread_not_found -> 404) rather than string-sniffing. Every op
// resolves its target BY ID: an unknown thread/reply id yields ErrThreadNotFound
// / ErrReplyNotFound, never a positional fallback that mutates a neighbour.
var (
	// ErrClaimNotFound: no claim with the given id exists in the project.
	ErrClaimNotFound = errors.New("comments: claim not found")
	// ErrThreadNotFound: no comment thread with the given id on the claim.
	ErrThreadNotFound = errors.New("comments: comment thread not found")
	// ErrReplyNotFound: no reply with the given id under the thread.
	ErrReplyNotFound = errors.New("comments: reply not found")
	// ErrEmptyBody: a comment/reply body was empty or whitespace-only.
	ErrEmptyBody = errors.New("comments: comment body must not be empty")
	// ErrUnsafeBody: a comment/reply body with real content that cannot be stored
	// and read back byte-exact through YAML. yaml.v3 v3.0.1 emits certain
	// leading-whitespace bodies as a block scalar it then cannot re-parse (a bare
	// leading newline, a leading blank/whitespace-only line) or re-parses lossily
	// (a first CONTENT line indented by a tab or spaces, e.g. "\tcode\nmore" or
	// "    code\n    more"), so — stored verbatim — it would brick the whole claims
	// dir on the next load. It is refused here, at the shared input boundary both
	// the CLI and the serve handlers cross, by an ACTUAL round-trip probe
	// (loader.CommentBodyRoundTrips) that matches the loader's save-time guard
	// (loader.ErrClaimNotRoundTrippable) by construction — the guard remains the
	// systemic backstop under it. The message is actionable: start the body with a
	// non-whitespace character.
	ErrUnsafeBody = errors.New("comments: comment body cannot be safely stored as YAML: start it with a non-whitespace character — remove any leading blank line and de-indent the first line — then retry")
	// ErrInvalidActor: --as was neither "human" nor "agent".
	ErrInvalidActor = errors.New(`comments: actor must be "human" or "agent"`)
	// ErrRightsDenied: advisory rights — an agent may not resolve/reopen/edit/
	// delete a human-opened thread or a human-authored reply.
	ErrRightsDenied = errors.New("comments: advisory rights denied for this actor")
	// ErrThreadResolved: the thread is resolved (cannot reply to it, and
	// resolving an already-resolved thread is a no-op error).
	ErrThreadResolved = errors.New("comments: comment thread is resolved")
	// ErrThreadOpen: the thread is already open (reopen is a no-op error).
	ErrThreadOpen = errors.New("comments: comment thread is already open")
	// ErrBannerClaim: banner-layout claims are decorative and cannot carry
	// comment threads (they never render an edges/comment panel).
	ErrBannerClaim = errors.New("comments: banner claims cannot carry comment threads")
)

// nowFunc is the ops' clock, overridable in tests so created/resolved/reopened
// timestamps can be asserted deterministically.
var nowFunc = time.Now

// mutateInterlude is a test seam invoked inside mutate's claims-sentinel
// critical section, after the claim's pre-mutation file token is captured and
// just before the optimistic-concurrency-guarded save, so a test can
// deterministically simulate an out-of-band edit landing in the load->save
// window and assert the CAS guard refuses to clobber it. It is a no-op in
// production, mirroring nowFunc.
var mutateInterlude = func() {}

func nowRFC3339() string { return nowFunc().UTC().Format(time.RFC3339) }

// Deps is the dependency bundle every comment op runs against — the shared
// surface for the CLI verbs and the serve handlers, so both go through exactly
// one implementation and one locking discipline.
//
// Claims is the caller's already-loaded snapshot; the read-only List reads it
// directly. Every MUTATING op deliberately IGNORES it and re-reads the claims
// fresh from disk inside the claims-sentinel critical section, because a
// whole-file save over a stale snapshot would silently erase a concurrent
// writer's change. The lock- and flag-store review_pending inputs are likewise
// re-read fresh inside that same sentinel — supply them as LockStorePath /
// FlagStorePath (see below), which the production wiring does — so a concurrent
// `dossierx flag` committed under the sentinel is honored, not orphaned. An op
// still takes exactly one lock (the claims sentinel), and because the two
// stores' own sentinels are never taken here the claims -> lock-store ->
// flag-store ordering hazard cannot arise by construction.
type Deps struct {
	Cfg       *config.Config
	Claims    []model.Claim
	LockStore *lock.Store
	FlagStore *reaudit.FlagStore

	// LockStorePath and FlagStorePath, when set, are the on-disk lock- and
	// flag-store files a mutating op RE-READS FRESH inside the claims sentinel
	// (right before it recomputes review_pending), rather than trusting the
	// LockStore/FlagStore snapshots above. The production wiring (the CLI and
	// serve Deps builders) sets these paths: a snapshot loaded BEFORE the
	// sentinel can miss a `dossierx flag` that committed concurrently — the
	// claims sentinel serializes the two writers and flag persists its
	// flag-store entry while holding it — so a fresh read here honors that flag
	// instead of clobbering review_pending to false and orphaning it. A caller
	// that leaves a path empty keeps the corresponding snapshot (unit tests that
	// inject synthetic drift/flag state in memory).
	LockStorePath string
	FlagStorePath string
}

// Add opens a new comment thread on claimID and returns the updated claim and
// the minted thread id. Banner-layout claims are refused. On a LOCKED claim
// the new open thread sets review_pending; on a draft it does not (a draft
// never carries review_pending).
func (d *Deps) Add(claimID string, actor model.CommentRole, body string) (model.Claim, string, error) {
	if err := validateActor(actor); err != nil {
		return model.Claim{}, "", err
	}
	if err := validateBody(body); err != nil {
		return model.Claim{}, "", err
	}
	var tid string
	c, err := d.mutate(claimID, func(c *model.Claim) error {
		if c.Layout == model.LayoutBanner {
			return fmt.Errorf("comments: claim %q: %w", claimID, ErrBannerClaim)
		}
		used, err := backfillIDs(c)
		if err != nil {
			return err
		}
		tid, err = mintUniqueID(threadIDPrefix, used)
		if err != nil {
			return err
		}
		c.Comments = append(c.Comments, model.Comment{
			ID:      tid,
			Status:  model.CommentStatusOpen,
			Author:  actor,
			Created: nowRFC3339(),
			Body:    body,
			Edited:  false,
		})
		return nil
	})
	if err != nil {
		return model.Claim{}, "", err
	}
	return c, tid, nil
}

// Reply adds a follow-up to an OPEN thread and returns the updated claim and
// the minted reply id. Replying to a resolved thread is refused. Any actor may
// reply to any open thread (rights gate only resolve/reopen/edit/delete).
func (d *Deps) Reply(claimID, threadID string, actor model.CommentRole, body string) (model.Claim, string, error) {
	if err := validateActor(actor); err != nil {
		return model.Claim{}, "", err
	}
	if err := validateBody(body); err != nil {
		return model.Claim{}, "", err
	}
	var rid string
	c, err := d.mutate(claimID, func(c *model.Claim) error {
		used, err := backfillIDs(c)
		if err != nil {
			return err
		}
		th, ok := findThread(c, threadID)
		if !ok {
			return threadNotFound(claimID, threadID)
		}
		if th.Status != model.CommentStatusOpen {
			return fmt.Errorf("comments: thread %q on claim %q: %w", threadID, claimID, ErrThreadResolved)
		}
		rid, err = mintUniqueID(replyIDPrefix, used)
		if err != nil {
			return err
		}
		th.Replies = append(th.Replies, model.Reply{
			ID:      rid,
			Author:  actor,
			Created: nowRFC3339(),
			Body:    body,
			Edited:  false,
		})
		return nil
	})
	if err != nil {
		return model.Claim{}, "", err
	}
	return c, rid, nil
}

// Resolve marks an open thread resolved (recording the actor and time). Rights:
// a human-opened thread only by a human; an agent-opened one by either.
// Resolving the last open thread clears review_pending iff no drift and no flag
// still stand; otherwise it is retained.
func (d *Deps) Resolve(claimID, threadID string, actor model.CommentRole) (model.Claim, error) {
	if err := validateActor(actor); err != nil {
		return model.Claim{}, err
	}
	return d.mutate(claimID, func(c *model.Claim) error {
		if _, err := backfillIDs(c); err != nil {
			return err
		}
		th, ok := findThread(c, threadID)
		if !ok {
			return threadNotFound(claimID, threadID)
		}
		if !canAct(actor, th.Author) {
			return rightsDenied(claimID, threadID)
		}
		if th.Status != model.CommentStatusOpen {
			return fmt.Errorf("comments: thread %q on claim %q: %w", threadID, claimID, ErrThreadResolved)
		}
		th.Status = model.CommentStatusResolved
		th.ResolvedBy = actor
		th.ResolvedAt = nowRFC3339()
		return nil
	})
}

// Reopen returns a resolved thread to open (recording the actor and time), and
// re-sets review_pending on a locked claim. Same advisory rights as Resolve.
func (d *Deps) Reopen(claimID, threadID string, actor model.CommentRole) (model.Claim, error) {
	if err := validateActor(actor); err != nil {
		return model.Claim{}, err
	}
	return d.mutate(claimID, func(c *model.Claim) error {
		if _, err := backfillIDs(c); err != nil {
			return err
		}
		th, ok := findThread(c, threadID)
		if !ok {
			return threadNotFound(claimID, threadID)
		}
		if !canAct(actor, th.Author) {
			return rightsDenied(claimID, threadID)
		}
		if th.Status != model.CommentStatusResolved {
			return fmt.Errorf("comments: thread %q on claim %q: %w", threadID, claimID, ErrThreadOpen)
		}
		th.Status = model.CommentStatusOpen
		th.ReopenedBy = actor
		th.ReopenedAt = nowRFC3339()
		return nil
	})
}

// Edit replaces the body of a thread root (replyID == "") or a specific reply,
// marking it edited. Rights key off the edited message's own author.
func (d *Deps) Edit(claimID, threadID, replyID string, actor model.CommentRole, body string) (model.Claim, error) {
	if err := validateActor(actor); err != nil {
		return model.Claim{}, err
	}
	if err := validateBody(body); err != nil {
		return model.Claim{}, err
	}
	return d.mutate(claimID, func(c *model.Claim) error {
		if _, err := backfillIDs(c); err != nil {
			return err
		}
		th, ok := findThread(c, threadID)
		if !ok {
			return threadNotFound(claimID, threadID)
		}
		if replyID == "" {
			if !canAct(actor, th.Author) {
				return rightsDenied(claimID, threadID)
			}
			th.Body = body
			th.Edited = true
		} else {
			rp, ok := findReply(th, replyID)
			if !ok {
				return replyNotFound(claimID, threadID, replyID)
			}
			if !canAct(actor, rp.Author) {
				return rightsDenied(claimID, threadID)
			}
			rp.Body = body
			rp.Edited = true
		}
		return nil
	})
}

// Delete removes a whole thread (replyID == "") or a single reply. Rights key
// off the removed message's own author. Deleting the last open thread clears
// review_pending under the same iff-no-other-trigger rule as Resolve.
func (d *Deps) Delete(claimID, threadID, replyID string, actor model.CommentRole) (model.Claim, error) {
	if err := validateActor(actor); err != nil {
		return model.Claim{}, err
	}
	return d.mutate(claimID, func(c *model.Claim) error {
		if _, err := backfillIDs(c); err != nil {
			return err
		}
		ti := threadIndex(c, threadID)
		if ti < 0 {
			return threadNotFound(claimID, threadID)
		}
		if replyID == "" {
			if !canAct(actor, c.Comments[ti].Author) {
				return rightsDenied(claimID, threadID)
			}
			c.Comments = append(c.Comments[:ti], c.Comments[ti+1:]...)
		} else {
			ri := replyIndex(&c.Comments[ti], replyID)
			if ri < 0 {
				return replyNotFound(claimID, threadID, replyID)
			}
			if !canAct(actor, c.Comments[ti].Replies[ri].Author) {
				return rightsDenied(claimID, threadID)
			}
			c.Comments[ti].Replies = append(c.Comments[ti].Replies[:ri], c.Comments[ti].Replies[ri+1:]...)
		}
		return nil
	})
}

// List returns the comment threads on claimID from the caller's loaded
// snapshot (Deps.Claims). openOnly filters to threads still status: open. It is
// read-only — no lock, no write. An unknown claim id is ErrClaimNotFound.
func (d *Deps) List(claimID string, openOnly bool) ([]model.Comment, error) {
	c, ok := findClaim(d.Claims, claimID)
	if !ok {
		return nil, fmt.Errorf("comments: claim %q: %w", claimID, ErrClaimNotFound)
	}
	var out []model.Comment
	for _, cm := range c.Comments {
		if openOnly && cm.Status != model.CommentStatusOpen {
			continue
		}
		out = append(out, cm)
	}
	return out, nil
}

// mutate is the shared load->mutate->save skeleton for every mutating op. It
// takes the claims sentinel, re-reads the claims FRESH inside it (never trusts
// Deps.Claims for a write), locates the target claim by id, captures the claim
// file's pre-mutation token, runs fn (which mutates *c in place), recomputes
// review_pending against store state re-read fresh inside the sentinel, and
// writes exactly that one claim back with an optimistic-concurrency guard
// (SaveClaimIfUnchanged) so an out-of-band edit in the load->save window is
// refused (ErrClaimFileChanged) rather than clobbered — releasing the lock on
// the way out. If fn returns an error, nothing is written, so an unknown-id /
// rights / validation failure never touches a neighbouring claim or message.
func (d *Deps) mutate(claimID string, fn func(c *model.Claim) error) (model.Claim, error) {
	release, err := lock.AcquireClaimsLock(d.Cfg)
	if err != nil {
		return model.Claim{}, err
	}
	defer release()

	claims, err := loader.LoadClaims(d.Cfg.ClaimsDir)
	if err != nil {
		return model.Claim{}, err
	}
	idx := claimIndex(claims, claimID)
	if idx < 0 {
		return model.Claim{}, fmt.Errorf("comments: claim %q: %w", claimID, ErrClaimNotFound)
	}

	// Snapshot the claim file's pre-mutation bytes now — inside the sentinel,
	// right after load — so the SaveClaimIfUnchanged below refuses to clobber an
	// out-of-band edit (a text editor, a sentinel-less writer) that slips into
	// this load->save window, surfacing loader.ErrClaimFileChanged (the wired
	// 409 claim_file_changed) instead. This mirrors flag/reaudit/lock, which all
	// write via CaptureClaimFileToken+SaveClaimIfUnchanged.
	token, err := loader.CaptureClaimFileToken(claims[idx].SourcePath)
	if err != nil {
		return model.Claim{}, err
	}

	if err := fn(&claims[idx]); err != nil {
		return model.Claim{}, err
	}

	// Recompute review_pending against store state RE-READ FRESH inside this
	// claims sentinel (see reviewStores): a snapshot taken before the sentinel
	// could miss a `dossierx flag` that committed concurrently and orphan it
	// with review_pending:false.
	ls, fs, err := d.reviewStores()
	if err != nil {
		return model.Claim{}, err
	}
	d.refreshReviewPending(claims, &claims[idx], ls, fs)

	mutateInterlude()

	if err := loader.SaveClaimIfUnchanged(claims[idx], token); err != nil {
		return model.Claim{}, err
	}
	return claims[idx], nil
}

// refreshReviewPending recomputes review_pending for a LOCKED claim from the
// single pending predicate after a mutation, against the ls/lock- and fs/flag-
// store snapshots mutate re-read fresh inside the claims sentinel. A draft
// claim's review_pending is left untouched (it is false and stays false — never
// set on a draft — and not writing it preserves on-disk byte-identity for
// uncommented drafts).
func (d *Deps) refreshReviewPending(claims []model.Claim, c *model.Claim, ls *lock.Store, fs *reaudit.FlagStore) {
	if c.Status != model.StatusLocked {
		return
	}
	c.ReviewPending = Recompute(*c, claims, ls, fs)
}

// reviewStores returns the lock- and flag-store snapshots review_pending is
// recomputed against. When the caller supplied a path (the CLI/serve production
// wiring), the store is RE-READ FRESH from disk here — inside mutate's claims
// sentinel — so a `dossierx flag` (or lock/reaudit) that committed concurrently
// is honored rather than missed by a snapshot taken before the sentinel. When
// only an in-memory store and no path is supplied (unit tests injecting
// synthetic drift/flag state), that snapshot is used as-is.
func (d *Deps) reviewStores() (*lock.Store, *reaudit.FlagStore, error) {
	ls := d.LockStore
	if d.LockStorePath != "" {
		loaded, err := lock.LoadStore(d.LockStorePath)
		if err != nil {
			return nil, nil, fmt.Errorf("comments: re-read lock store: %w", err)
		}
		ls = loaded
	}
	fs := d.FlagStore
	if d.FlagStorePath != "" {
		loaded, err := reaudit.LoadFlagStore(d.FlagStorePath)
		if err != nil {
			return nil, nil, fmt.Errorf("comments: re-read flag store: %w", err)
		}
		fs = loaded
	}
	return ls, fs, nil
}

func validateActor(a model.CommentRole) error {
	if a != model.CommentRoleHuman && a != model.CommentRoleAgent {
		return fmt.Errorf("comments: %q: %w", a, ErrInvalidActor)
	}
	return nil
}

// validateBody is the shared input-boundary check both the CLI verbs and the
// serve handlers cross before any mutating op. An empty/whitespace-only body is
// ErrEmptyBody; a body with real content that would not survive storage is
// ErrUnsafeBody. The unsafe check is an ACTUAL marshal + strict-decode round-trip
// probe (loader.CommentBodyRoundTrips), NOT a hand-rolled leading-whitespace
// heuristic, so it rejects EXACTLY the bodies the loader's save-time guard
// (loader.ErrClaimNotRoundTrippable) would refuse — matching it by construction.
// That closes the two gaps the old heuristic had: it now rejects a first CONTENT
// line indented by a tab or spaces (which the heuristic missed, so the op ran and
// the guard leaked a raw round-trip error), and it now ACCEPTS bodies that
// round-trip cleanly despite a whitespace-only or CR/NBSP/NEL-led first line
// (which the heuristic false-rejected).
func validateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return ErrEmptyBody
	}
	if !loader.CommentBodyRoundTrips(body) {
		return ErrUnsafeBody
	}
	return nil
}

// canAct is the advisory-rights rule: a human may act on anything; an agent may
// act only on agent-authored messages. Equivalently: a human-opened thread /
// human-authored reply is actionable only by a human.
func canAct(actor, author model.CommentRole) bool {
	return actor == model.CommentRoleHuman || author == model.CommentRoleAgent
}

func claimIndex(claims []model.Claim, id string) int {
	for i := range claims {
		if claims[i].ID == id {
			return i
		}
	}
	return -1
}

func findThread(c *model.Claim, threadID string) (*model.Comment, bool) {
	i := threadIndex(c, threadID)
	if i < 0 {
		return nil, false
	}
	return &c.Comments[i], true
}

func threadIndex(c *model.Claim, threadID string) int {
	for i := range c.Comments {
		if c.Comments[i].ID == threadID {
			return i
		}
	}
	return -1
}

func findReply(th *model.Comment, replyID string) (*model.Reply, bool) {
	i := replyIndex(th, replyID)
	if i < 0 {
		return nil, false
	}
	return &th.Replies[i], true
}

func replyIndex(th *model.Comment, replyID string) int {
	for i := range th.Replies {
		if th.Replies[i].ID == replyID {
			return i
		}
	}
	return -1
}

func threadNotFound(claimID, threadID string) error {
	return fmt.Errorf("comments: thread %q on claim %q: %w", threadID, claimID, ErrThreadNotFound)
}

func replyNotFound(claimID, threadID, replyID string) error {
	return fmt.Errorf("comments: reply %q in thread %q on claim %q: %w", replyID, threadID, claimID, ErrReplyNotFound)
}

func rightsDenied(claimID, threadID string) error {
	return fmt.Errorf("comments: thread %q on claim %q: %w", threadID, claimID, ErrRightsDenied)
}
