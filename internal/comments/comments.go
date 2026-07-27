package comments

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
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
	// ErrCommentDigestDrift: the claim's STORED comments block does not match
	// the digest recorded at the engine's last comment write to it, so someone
	// edited the block out of band. Every mutating op refuses rather than
	// writing.
	//
	// This exists because the write path's LAST act is to re-record the digest
	// from whatever the file now says. Without a check first, that refresh
	// launders the tamper: hand-delete an unresolved thread (which is how a
	// claim gets past the lock gate with a review still open), then run any
	// comment op on that claim, and the `comment-ledger-drift` finding that had
	// named the edit is overwritten by a digest of the edited block. The
	// integrity record would then agree with the tampered file forever. An
	// integrity record any ordinary write can launder is not a record.
	//
	// The recovery is version control, not a retry: nothing the engine can
	// compute recovers the threads that were deleted.
	ErrCommentDigestDrift = errors.New("comments: the claim's comment threads were changed outside dossierx")
	// ErrCommentDigestUnavailable: the comment digest store could not be opened
	// for writing — it is there but does not decode, its sentinel is held by
	// another process, or its directory cannot be written.
	//
	// It is raised by the PRE-FLIGHT, before fn runs and before anything is
	// saved, and that is the entire point of it. The write path's last act is to
	// refresh the digest, so a store that fails at THAT point leaves the comment
	// on disk and the op reporting failure. An agent's documented response to a
	// failure is to retry, and each retry appended another identical thread to
	// the claim while still reporting failure — filling a human's review thread
	// with duplicates and manufacturing the exact comment-ledger drift the store
	// exists to detect. Refusing up front makes the failure atomic: nothing is
	// written, so a retry is safe and idempotent (it fails the same way until the
	// store is restored).
	ErrCommentDigestUnavailable = errors.New("comments: the comment digest store could not be opened for writing, so this write was refused before anything was changed: restore .dossierx-comment-digest.json from version control (or remove a stale .dossierx-comment-digest.json.lock left by a crash) and retry")
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

	// Open the comment digest store — take its sentinel, decode it, and prove it
	// can be written — BEFORE fn runs and before anything is saved.
	//
	// The store used to be opened at the foot of this function, after the claim
	// was already on disk, and every failure there was returned as a hard error.
	// That combination is the worst one available: the comment was written AND
	// the op reported failure, so a retrying caller appended the same thread
	// again on every attempt while still being told it had not worked. Doing the
	// whole open here makes the refusal atomic — nothing is written, and a retry
	// is safe.
	//
	// It also removes a real disagreement. checkCommentDigest deliberately
	// treated an unreadable store as "not covered" while recordCommentDigest
	// treated the identical failure as fatal; both now read the ONE store this
	// opens, so there is only one answer about it.
	//
	// The sentinel is held across fn and the claim save, which is wider than the
	// old momentary hold and costs nothing: every acquisition of the digest
	// sentinel in the product happens INSIDE this claims sentinel (see
	// internal/digest's package comment), which we hold, so nothing else can be
	// waiting on it. The claims -> digest ordering is unchanged, and digest is
	// still never taken alone.
	digests, releaseDigests, err := d.openCommentDigest()
	if err != nil {
		return model.Claim{}, err
	}
	defer releaseDigests()

	// The integrity gate, evaluated against the PRE-mutation block and BEFORE fn
	// runs, so a refusal writes nothing at all. recordCommentDigest at the foot
	// of this function re-records the digest unconditionally; if that ran over a
	// comment block someone had hand-edited, the write would adopt the tampered
	// block as the new truth and erase the comment-ledger-drift finding that
	// named it. Checking here is what makes the digest a record rather than a
	// rubber stamp.
	if err := checkCommentDigest(digests, claims[idx]); err != nil {
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

	// Taken from the SAME freshly-read lock store, because it answers a question
	// about this project's history rather than about this op: has a ledger-aware
	// build already run here? It decides whether an absent digest store may be
	// adopted wholesale (an upgrade) or must not be (a deletion) — see
	// recordCommentDigest.
	ledgerCovered := ls.LedgerCovered()

	mutateInterlude()

	if err := loader.SaveClaimIfUnchanged(claims[idx], token); err != nil {
		return model.Claim{}, err
	}

	// Refresh this claim's comment digest — the integrity record that makes a
	// hand-edited comment block detectable (internal/digest). This is THE hook
	// point: every comment write in the product, from the CLI and from serve
	// alike, funnels through this one function, so one call here covers all of
	// them and there is no second path to keep in step.
	if err := d.recordCommentDigest(digests, claims, claims[idx], ledgerCovered); err != nil {
		return model.Claim{}, err
	}
	return claims[idx], nil
}

// openCommentDigest takes the digest store's sentinel, loads the store, and
// proves it can be written — returning the store and the sentinel's release.
//
// All three failures collapse to ErrCommentDigestUnavailable, because to the
// caller they are one condition with one recovery ("restore the store, then
// retry") and, far more importantly, because they are all raised at a point
// where NOTHING has been written. See ErrCommentDigestUnavailable for what the
// old late-and-fatal ordering cost.
//
// It runs INSIDE mutate's claims sentinel and takes the digest store's own
// sentinel underneath it — claims -> digest, always that way round and never
// digest alone — so it adds no new lock-ordering hazard to the existing claims
// -> lock-store -> flag-store discipline.
func (d *Deps) openCommentDigest() (*digest.Store, func(), error) {
	path := digest.StorePath(d.Cfg)

	release, err := lock.AcquireFileLock(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%w (%v)", ErrCommentDigestUnavailable, err)
	}

	store, err := digest.LoadStore(path)
	if err != nil {
		release()
		return nil, nil, fmt.Errorf("%w (%v)", ErrCommentDigestUnavailable, err)
	}
	if err := store.CheckWritable(); err != nil {
		release()
		return nil, nil, fmt.Errorf("%w (%v)", ErrCommentDigestUnavailable, err)
	}
	return store, release, nil
}

// checkCommentDigest refuses a comment write to a claim whose stored comment
// block no longer matches the digest the engine recorded at its last write.
//
// The predicate is deliberately the same one lock.Audit's comment-ledger-drift
// rule uses, including its "unknown is not drifted" half: a claim with NO
// recorded digest is simply not covered yet (a project that has never run a
// comment op, or a claim created since the last one), and refusing there would
// make the first comment on every new claim impossible. Only a RECORDED digest
// that DISAGREES is tampering.
//
// It reads the store openCommentDigest already loaded under the digest
// sentinel, rather than loading its own copy. That is not just deduplication:
// the two used to disagree about the same file — this one swallowed a load
// failure as "not covered" while the recording half treated it as fatal — and
// the disagreement is what let a write proceed against a store that could not
// then be updated. One open, one answer.
//
// A store that could not be read never reaches here at all: openCommentDigest
// refuses first, before anything is written (ErrCommentDigestUnavailable).
func checkCommentDigest(store *digest.Store, c model.Claim) error {
	recorded, known := store.Digest(c.ID)
	if !known || recorded == digest.CommentsDigest(c) {
		return nil
	}
	return fmt.Errorf("%w: claim %q's comments block does not match the digest recorded at the last comment operation, so this write is refused rather than re-recording the edited block as the truth. Comments are engine-managed: restore the claim file from version control, then retry", ErrCommentDigestDrift, c.ID)
}

// recordCommentDigest refreshes the digest store's entry for the claim just
// written, into the store openCommentDigest opened (and whose sentinel mutate
// still holds).
//
// It runs AFTER the claim file is saved, not before, for two reasons that pull
// in the same direction. First, SaveClaimIfUnchanged can legitimately REFUSE
// (an out-of-band edit landed in the load->save window): recording the digest
// first would leave a digest describing a mutation that never happened, so an
// honest concurrency refusal would be reported forever after as tampering.
// Second, if this process dies between the save and this refresh, the recorded
// digest LAGS the file and the gate reports comment-ledger-drift — a false
// positive a human can see and clear by re-running the gate after any comment
// op. Both failure modes are loud; neither is silently wrong, which is the only
// ordering property an integrity record has to have.
//
// On a store file that does not exist yet, every claim's current comment block
// is adopted at once (digest.Adopt), so a project upgrading into this feature
// gets coverage for the threads it already has instead of only for claims
// someone happens to comment on afterwards.
//
// The one failure left here is Save itself, and it is returned to the caller
// even though the comment is already on disk, because the alternative is to
// swallow the one signal that the integrity record has stopped tracking
// reality. It is now an exceptional path rather than the ordinary one:
// openCommentDigest has already proved the store loads and its directory takes
// a write, so reaching this means the world changed underneath a probe taken
// moments earlier. The message says plainly that the comment WAS saved, because
// the wrong response to it is a retry — that is what appends the thread twice.
func (d *Deps) recordCommentDigest(store *digest.Store, claims []model.Claim, c model.Claim, ledgerCovered bool) error {
	// Adoption on first creation is what gives a project upgrading INTO this
	// feature coverage for the threads it already has. It is gated on the project
	// NOT already being ledger-covered, because in a covered project "the digest
	// store is not there" is not an upgrade — it is a deleted file, which
	// internal/check reports as comment-digest-absent, and adopting there would
	// re-record a hand-edited comment block as the truth and clear the very
	// finding that named it. Same rule the lock ledger has always applied to
	// itself (AdoptLedger: absence never adopts), for the same reason: an
	// adoption an attacker can trigger is not an adoption.
	//
	// The cost is honest and bounded: in a covered project whose store is gone,
	// this write covers only the claim it touched, and every other claim stays
	// UNKNOWN — never blessed, never accused.
	if !store.FileExists() && !ledgerCovered {
		digest.Adopt(store, claims)
	}
	store.Record(c)
	if err := store.Save(); err != nil {
		return fmt.Errorf("comments: THE COMMENT WAS SAVED, but the comment digest could not be written (%w) — do NOT retry the comment op, which would write it a second time; fix the store's directory and run any comment op to refresh the digest", err)
	}
	return nil
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
