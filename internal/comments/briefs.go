package comments

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/BarterX-Tech/dossierx/internal/atomicfile"
	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// briefs.go is comment threads on BRIEFS (NIT-205): a thread anchors on a
// brief's path (or its <folder>.<slug> id) the way it anchors on a claim id.
//
// These are NEW methods beside the claim-keyed ones, whose signatures are
// unchanged (internal/serve's handlers compile untouched; NIT-198 owns the
// routes). Each op runs the same thread operation as its claim twin (addThreadOp
// and friends), so ids, the advisory rights rule (an agent acts only on
// agent-authored messages; the human resolves) and thread states are judged by
// one implementation.
//
// Where they differ is storage, and only there:
//
//   - The threads live in the brief's own frontmatter, under `comments:`,
//     written by briefs.SetComments, which touches no other byte of the file
//     and re-reads its own output. The brief's lock hash does not sign them.
//   - Coverage is the comment digest store's `briefs` map, never its claim map,
//     so no claim rule mistakes a brief for a deleted claim.
//   - A brief has no review_pending to recompute from a thread: an open thread
//     on a brief refuses `brief lock` and `brief reaudit` (comment_open) and is
//     counted by check, and that is all it does.
//
// LOCKING. Every op takes the claims sentinel — the one lock every authored
// file write in this engine takes, and the one `brief lock` / `brief unlock`
// take before rewriting a brief's status line — then the digest store's own
// lock as a leaf, exactly the order the claim ops use. Inside it the brief is
// read fresh, and the bytes are compared again just before the write, so an
// editor's save that lands in between is refused (ErrBriefFileChanged), never
// overwritten.

// ErrBriefNotFound: the ref names no brief.
var ErrBriefNotFound = errors.New("comments: brief not found")

// ErrBriefFileChanged: the brief's file changed between the read and the write.
var ErrBriefFileChanged = errors.New("comments: the brief's file changed while the comment was being written; nothing was written, retry")

// BriefAdd opens a thread on the brief ref names and returns the brief as
// written and the minted thread id.
func (d *Deps) BriefAdd(ref string, actor model.CommentRole, body string) (briefs.Brief, string, error) {
	if err := validateActor(actor); err != nil {
		return briefs.Brief{}, "", err
	}
	if err := validateBody(body); err != nil {
		return briefs.Brief{}, "", err
	}
	var tid string
	b, err := d.mutateBrief(ref, func(path string) func(*model.Claim) error {
		return addThreadOp(subjectBrief(path), actor, body, &tid)
	})
	if err != nil {
		return briefs.Brief{}, "", err
	}
	return b, tid, nil
}

// BriefReply adds a reply to an open thread on the brief.
func (d *Deps) BriefReply(ref, threadID string, actor model.CommentRole, body string) (briefs.Brief, string, error) {
	if err := validateActor(actor); err != nil {
		return briefs.Brief{}, "", err
	}
	if err := validateBody(body); err != nil {
		return briefs.Brief{}, "", err
	}
	var rid string
	b, err := d.mutateBrief(ref, func(path string) func(*model.Claim) error {
		return replyOp(subjectBrief(path), threadID, actor, body, &rid)
	})
	if err != nil {
		return briefs.Brief{}, "", err
	}
	return b, rid, nil
}

// BriefResolve resolves a thread on the brief (the advisory rights rule applies).
func (d *Deps) BriefResolve(ref, threadID string, actor model.CommentRole) (briefs.Brief, error) {
	if err := validateActor(actor); err != nil {
		return briefs.Brief{}, err
	}
	return d.mutateBrief(ref, func(path string) func(*model.Claim) error {
		return resolveOp(subjectBrief(path), threadID, actor)
	})
}

// BriefReopen reopens a resolved thread on the brief.
func (d *Deps) BriefReopen(ref, threadID string, actor model.CommentRole) (briefs.Brief, error) {
	if err := validateActor(actor); err != nil {
		return briefs.Brief{}, err
	}
	return d.mutateBrief(ref, func(path string) func(*model.Claim) error {
		return reopenOp(subjectBrief(path), threadID, actor)
	})
}

// BriefEdit replaces a thread root's body (replyID "") or a reply's.
func (d *Deps) BriefEdit(ref, threadID, replyID string, actor model.CommentRole, body string) (briefs.Brief, error) {
	if err := validateActor(actor); err != nil {
		return briefs.Brief{}, err
	}
	if err := validateBody(body); err != nil {
		return briefs.Brief{}, err
	}
	return d.mutateBrief(ref, func(path string) func(*model.Claim) error {
		return editOp(subjectBrief(path), threadID, replyID, actor, body)
	})
}

// BriefDelete removes a thread (replyID "") or a reply.
func (d *Deps) BriefDelete(ref, threadID, replyID string, actor model.CommentRole) (briefs.Brief, error) {
	if err := validateActor(actor); err != nil {
		return briefs.Brief{}, err
	}
	return d.mutateBrief(ref, func(path string) func(*model.Claim) error {
		return deleteOp(subjectBrief(path), threadID, replyID, actor)
	})
}

// BriefList returns the threads on a brief, read from the working tree (a read
// takes no lock, as List takes none).
func (d *Deps) BriefList(ref string, openOnly bool) (briefs.Brief, []model.Comment, error) {
	b, err := findBrief(briefs.Load(d.Cfg), ref)
	if err != nil {
		return briefs.Brief{}, nil, err
	}
	var out []model.Comment
	for _, cm := range b.Comments {
		if openOnly && cm.Status != model.CommentStatusOpen {
			continue
		}
		out = append(out, cm)
	}
	return b, out, nil
}

func findBrief(set *briefs.Set, ref string) (briefs.Brief, error) {
	b, ok := set.Lookup(ref)
	if !ok {
		return briefs.Brief{}, fmt.Errorf("comments: brief %q: %w", ref, ErrBriefNotFound)
	}
	return b, nil
}

// mutateBrief is mutate for a brief: the claims sentinel, a fresh read, the
// digest integrity gate, the op on the brief's threads, the rewrite of its
// comments block, and the digest record, in that order.
func (d *Deps) mutateBrief(ref string, op func(path string) func(*model.Claim) error) (briefs.Brief, error) {
	release, err := lock.AcquireClaimsLock(d.Cfg)
	if err != nil {
		return briefs.Brief{}, err
	}
	defer release()

	b, err := findBrief(briefs.Load(d.Cfg), ref)
	if err != nil {
		return briefs.Brief{}, err
	}
	file := briefs.FilePath(d.Cfg, b)
	info, err := os.Lstat(file)
	if err != nil {
		return briefs.Brief{}, fmt.Errorf("comments: stat %s: %w", b.Path, err)
	}
	if !info.Mode().IsRegular() {
		// Load refuses a link, and one that appeared since is refused here:
		// a write through it would land outside the briefs tree.
		return briefs.Brief{}, fmt.Errorf("comments: %s is not a regular file; nothing was written: %w", b.Path, ErrBriefFileChanged)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return briefs.Brief{}, fmt.Errorf("comments: read %s: %w", b.Path, err)
	}
	// The set was loaded from these same bytes a moment ago; parse the bytes
	// just read so the threads operated on are exactly the ones on disk now.
	current, ok := briefs.FromFiles(d.Cfg, []briefs.File{{Rel: path.Join(b.Folder, b.Slug+".md"), Size: int64(len(raw)), Regular: true, Data: raw}}).Lookup(b.ID)
	if !ok {
		return briefs.Brief{}, fmt.Errorf("comments: brief %q: %w", ref, ErrBriefNotFound)
	}
	b.Comments = current.Comments

	digests, releaseDigests, err := d.openCommentDigest()
	if err != nil {
		return briefs.Brief{}, err
	}
	defer releaseDigests()

	ls, _, err := d.reviewStores()
	if err != nil {
		return briefs.Brief{}, err
	}
	ledgerCovered := ls.LedgerEstablished(digests.FileExists())
	preLedger := ls.PreLedger()
	if err := checkBriefDigest(digests, b, ledgerCovered); err != nil {
		return briefs.Brief{}, err
	}

	carrier := model.Claim{ID: b.ID, Comments: b.Comments}
	if err := op(b.Path)(&carrier); err != nil {
		return briefs.Brief{}, err
	}
	written, err := briefs.SetComments(raw, carrier.Comments)
	if err != nil {
		return briefs.Brief{}, fmt.Errorf("comments: %s: %w", b.Path, err)
	}
	mutateInterlude()
	if again, err := os.ReadFile(file); err != nil || !bytes.Equal(again, raw) {
		return briefs.Brief{}, fmt.Errorf("%w (%s)", ErrBriefFileChanged, b.Path)
	}
	if err := atomicfile.Write(file, written, info.Mode().Perm()); err != nil {
		return briefs.Brief{}, fmt.Errorf("comments: write %s: %w", b.Path, err)
	}
	b.Comments = carrier.Comments

	// Coverage, as for a claim: a pre-ledger project with no digest store is
	// not given one by a comment; a project with neither a store nor ledger
	// coverage adopts every claim's block in the act that creates the store,
	// so no claim thread reads as unrecorded once it is covered.
	if preLedger && !digests.FileExists() {
		return b, nil
	}
	if !digests.FileExists() && !ledgerCovered {
		if claims, loadErr := loader.LoadAll(d.Cfg); loadErr == nil {
			digest.Adopt(digests, claims)
		}
	}
	digests.RecordBrief(b.ID, b.Comments)
	if err := digests.Save(); err != nil {
		return briefs.Brief{}, fmt.Errorf("comments: THE COMMENT WAS SAVED to %s, but the comment digest could not be written (%w) — do NOT retry the comment op, which would write it a second time; fix the store's directory and run any comment op on the brief to refresh the digest", b.Path, err)
	}
	return b, nil
}

// checkBriefDigest is checkCommentDigest for a brief's threads.
func checkBriefDigest(store *digest.Store, b briefs.Brief, ledgerCovered bool) error {
	recorded, known := store.BriefDigest(b.ID)
	if !known {
		if ledgerCovered && len(b.Comments) > 0 {
			return fmt.Errorf("%w: brief %s carries %d comment thread(s) but has NO entry in %s, in a project covered by the lock ledger; the only code path that writes a brief's thread records its digest in the same act. This write is refused rather than recording the block as the truth: restore %s (or the brief's comments block, if that is what was forged) from version control", ErrCommentDigestDrift, b.Path, len(b.Comments), config.CommentDigestDisplayPath, config.CommentDigestDisplayPath)
		}
		return nil
	}
	if recorded == digest.BriefCommentsDigest(b.ID, b.Comments) {
		return nil
	}
	return fmt.Errorf("%w: brief %s's comments block does not match the digest recorded at the last comment operation, so this write is refused rather than re-recording the edited block as the truth. Restore whichever side is wrong from version control — %s, or %s — and if you cannot tell, restore both from the same commit", ErrCommentDigestDrift, b.Path, b.Path, config.CommentDigestDisplayPath)
}
