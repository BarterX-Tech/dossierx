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

func nowRFC3339() string { return nowFunc().UTC().Format(time.RFC3339) }

// Deps is the dependency bundle every comment op runs against — the shared
// surface for the CLI verbs and the serve handlers, so both go through exactly
// one implementation and one locking discipline.
//
// Claims is the caller's already-loaded snapshot; the read-only List reads it
// directly. Every MUTATING op deliberately IGNORES it and re-reads the claims
// fresh from disk inside the claims-sentinel critical section, because a
// whole-file loader.SaveClaim over a stale snapshot would silently erase a
// concurrent writer's change. LockStore and FlagStore are read WITHOUT their
// own sentinels (a stale read at worst leaves review_pending set, which the
// next reconcile corrects) so an op takes exactly one lock — the claims
// sentinel — and the claims -> lock-store -> flag-store ordering hazard cannot
// arise by construction.
type Deps struct {
	Cfg       *config.Config
	Claims    []model.Claim
	LockStore *lock.Store
	FlagStore *reaudit.FlagStore
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
	c, err := d.mutate(claimID, func(claims []model.Claim, c *model.Claim) error {
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
		d.refreshReviewPending(claims, c)
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
	c, err := d.mutate(claimID, func(claims []model.Claim, c *model.Claim) error {
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
		d.refreshReviewPending(claims, c)
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
	return d.mutate(claimID, func(claims []model.Claim, c *model.Claim) error {
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
		d.refreshReviewPending(claims, c)
		return nil
	})
}

// Reopen returns a resolved thread to open (recording the actor and time), and
// re-sets review_pending on a locked claim. Same advisory rights as Resolve.
func (d *Deps) Reopen(claimID, threadID string, actor model.CommentRole) (model.Claim, error) {
	if err := validateActor(actor); err != nil {
		return model.Claim{}, err
	}
	return d.mutate(claimID, func(claims []model.Claim, c *model.Claim) error {
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
		d.refreshReviewPending(claims, c)
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
	return d.mutate(claimID, func(claims []model.Claim, c *model.Claim) error {
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
		d.refreshReviewPending(claims, c)
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
	return d.mutate(claimID, func(claims []model.Claim, c *model.Claim) error {
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
		d.refreshReviewPending(claims, c)
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
// Deps.Claims for a write), locates the target claim by id, runs fn (which
// mutates *c in place and may set review_pending), and writes exactly that one
// claim back — releasing the lock on the way out. If fn returns an error,
// nothing is written, so an unknown-id / rights / validation failure never
// touches a neighbouring claim or message.
func (d *Deps) mutate(claimID string, fn func(claims []model.Claim, c *model.Claim) error) (model.Claim, error) {
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
	if err := fn(claims, &claims[idx]); err != nil {
		return model.Claim{}, err
	}
	if err := loader.SaveClaim(claims[idx]); err != nil {
		return model.Claim{}, err
	}
	return claims[idx], nil
}

// refreshReviewPending recomputes review_pending for a LOCKED claim from the
// single pending predicate after a mutation. A draft claim's review_pending is
// left untouched (it is false and stays false — never set on a draft — and not
// writing it preserves on-disk byte-identity for uncommented drafts).
func (d *Deps) refreshReviewPending(claims []model.Claim, c *model.Claim) {
	if c.Status != model.StatusLocked {
		return
	}
	c.ReviewPending = Recompute(*c, claims, d.LockStore, d.FlagStore)
}

func validateActor(a model.CommentRole) error {
	if a != model.CommentRoleHuman && a != model.CommentRoleAgent {
		return fmt.Errorf("comments: %q: %w", a, ErrInvalidActor)
	}
	return nil
}

func validateBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return ErrEmptyBody
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
