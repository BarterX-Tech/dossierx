// Package approvaledit answers, for one claim, the question the lock ledger
// could previously only answer yes or no to: what has changed since this was
// approved.
//
// The state it describes is the ordinary one. A claim is locked; someone needs
// to change it; they unlock it, rewrite it, and lock it again. Between the
// unlock and the re-lock the new wording simply replaced the old in the file,
// and a reviewer opening the viewer saw the new text with nothing to compare
// it against — the approved wording existed only in git, which the viewer
// cannot read and which does not know which commit carried the approval.
//
// This package is a projection, not a gate. It reads claims and the lock
// store, mutates neither, and decides nothing about whether anything may be
// locked. Readiness owns the question of whether the edit is a review cause
// (readiness.CauseUnapprovedEdit); this owns only the presentation of what
// moved. Keeping those apart is why an empty result here can never suppress a
// cause there.
package approvaledit

import (
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/textdiff"
)

// Change is what moved in one claim since the approval that was released.
type Change struct {
	// ClaimID is the claim this describes.
	ClaimID string `json:"claim_id"`

	// ApprovedAt/By/Reason are the released ledger record's approval fields:
	// when the text now being replaced was approved, by whom, and on whose
	// words. Reason is the one a reviewer actually reads — it is what the
	// approver said they were approving, next to what has since been written
	// instead.
	ApprovedAt     string `json:"approved_at,omitempty"`
	ApprovedBy     string `json:"approved_by,omitempty"`
	ApprovedReason string `json:"approved_reason,omitempty"`

	// ReleasedAt/By/Reason are the unlock that released it.
	ReleasedAt     string `json:"released_at,omitempty"`
	ReleasedBy     string `json:"released_by,omitempty"`
	ReleasedReason string `json:"released_reason,omitempty"`

	// ContentRetained is false when the ledger record predates
	// LedgerRecord.Content — the record proves the text moved and does not
	// carry the text it moved from. Hunks is then empty and a renderer must
	// say the approved wording is not retained, NOT that nothing changed and
	// NOT that everything is new. This is the one field a consumer may not
	// skip reading.
	ContentRetained bool `json:"content_retained"`

	// Hunks is the passage-by-passage difference from the approved body to
	// the current body, already rendered. Empty with ContentRetained true
	// means the body itself did not move and something in OtherFields did.
	Hunks []Hunk `json:"hunks,omitempty"`

	// ChangedPassages counts the passages that moved — the number the panel's
	// own header states ("1 passage differs from the approval of 12 Sep").
	// A removal and the addition that replaces it are ONE passage that
	// differs, not two, because that is what a reader counts when they look
	// at the page.
	ChangedPassages int `json:"changed_passages,omitempty"`

	// OtherFields names the persisted, signed fields other than the body that
	// differ from the approval, by their on-disk names. It is what keeps the
	// panel honest when only structure moved: a claim whose rests_on changed
	// and whose wording did not has an empty Hunks and a populated
	// OtherFields, and the renderer has something true to say.
	OtherFields []string `json:"other_fields,omitempty"`

	// FieldChanges is those same fields, diffed. OtherFields says WHICH
	// moved and this says WHAT moved in each, so a claim whose prose never
	// changed still shows a reader the change they are being asked about
	// rather than only its name. The two are always about the same set, in
	// the same order; OtherFields stays because a consumer that only wants
	// the names should not have to walk the diffs to get them.
	FieldChanges []FieldChange `json:"field_changes,omitempty"`
}

// Hunk is one passage of the diff, rendered.
//
// It carries HTML and not source because the panel shows PROSE: a reader
// looking at a claim sees its markdown rendered, and a diff of the same claim
// showing `**like this**` puts them in a second document written in a
// notation nobody asked them to read. The rendering happens here, in Go,
// because internal/render/markdown is this engine's only markdown renderer
// and the viewer has none — shipping source to the browser would mean
// growing a second one that could disagree with the first.
type Hunk struct {
	// Op is "equal", "remove" or "add".
	Op string `json:"op"`

	// HTML is the passage rendered, with the words that moved wrapped in
	// marks when this passage was paired with one on the other side. It is
	// produced by internal/render/markdown, which is the escaping boundary
	// for every other claim body in the viewer, so it is exactly as trusted
	// as the body it sits beside — and no more.
	HTML string `json:"html"`

	// Changed is the words marked inside HTML, in reading order, so a screen
	// reader can be told what moved instead of having the passage read to it
	// twice.
	Changed []string `json:"changed,omitempty"`
}

// BodyChanged reports whether any passage moved, as opposed to the single
// equal hunk an unchanged body produces.
func (c Change) BodyChanged() bool { return c.ChangedPassages > 0 }

// Compute returns one Change per claim that was approved, released by an
// honest unlock, and has since been edited away from that approval.
//
// The predicate is lock.EditedSinceApproval — the same call readiness makes
// to raise CauseUnapprovedEdit, so a claim this returns nothing for is a claim
// the Issues screen lists no unapproved edit for, and vice versa. See that
// function for why the question has one implementation and not three.
//
// Cost is one LockedClaimHash and, for the claims that qualify, one bounded
// line diff (see textdiff.MaxLines) per claim: O(V) hashes and O(V) diffs of
// claim-sized bodies, with no dependency traversal and no path-dependent term.
// It never mutates claims or the store.
func Compute(claims []model.Claim, store *lock.Store) map[string]Change {
	if store == nil {
		return nil
	}
	out := make(map[string]Change)
	for _, c := range claims {
		record, ok := lock.EditedSinceApproval(c, store)
		if !ok {
			continue
		}
		change := Change{
			ClaimID:        c.ID,
			ApprovedAt:     record.At,
			ApprovedBy:     record.Actor,
			ApprovedReason: record.Reason,
			ReleasedAt:     record.ReleasedAt,
			ReleasedBy:     record.ReleasedBy,
			ReleasedReason: record.ReleasedReason,
		}
		if approved, retained := store.ApprovedContent(c.ID); retained {
			change.ContentRetained = true
			// Passages, then the words inside them, then rendered. See
			// internal/textdiff's package comment for why the unit is a
			// passage and not a line, and render.go for why the marking has
			// to happen before the markdown renderer runs rather than after.
			for _, h := range textdiff.MarkWords(textdiff.Blocks(approved.Body, c.Body)) {
				rendered, changed := renderHunk(h)
				if strings.TrimSpace(rendered) == "" {
					// A passage that is nothing but blank lines carries the
					// spacing between two others and renders to nothing. It
					// round-trips, which is why textdiff keeps it, and it
					// would draw an empty block, which is why this drops it.
					continue
				}
				change.Hunks = append(change.Hunks, Hunk{Op: string(h.Op), HTML: rendered, Changed: changed})
				if h.Op == textdiff.OpRemove {
					change.ChangedPassages++
				}
			}
			// An addition with no removal facing it is a passage that
			// differs too — counted here rather than in the loop so a
			// remove/add pair stays ONE passage that differs.
			for i, h := range change.Hunks {
				if h.Op == "add" && (i == 0 || change.Hunks[i-1].Op != "remove") {
					change.ChangedPassages++
				}
			}
			for _, name := range lock.SignedFieldsDiffering(approved, c) {
				if name == "body" {
					continue
				}
				change.OtherFields = append(change.OtherFields, name)
			}
			sort.Strings(change.OtherFields)
			change.FieldChanges = fieldChanges(approved, c, change.OtherFields)
		}
		out[c.ID] = change
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
