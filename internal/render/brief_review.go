package render

import (
	"html/template"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/approvaledit"
	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// brief_review.go is a locked brief whose rests_on claim moved since the
// brief's baseline (NIT-200, Paper B3). The page draws one banner per
// changed claim, each with that claim's wording then and now, using
// NIT-199's passage wrap (BriefDiff.ChangesHTML) over the claim renderer.
//
// NOTHING HERE DECIDES A STATE. Whether a brief is review-pending, which
// claims moved, their wording at the baseline and now, and when the current
// wording was approved, are briefs.Evaluate's answers (ReviewPending,
// ChangedClaims). This file only draws what it is handed.

// BriefReviewView is the review-pending page: one stacked banner per
// changed rests_on claim, and the "changed <date>" labels the Rests on
// rows read.
type BriefReviewView struct {
	// Trigger is Review.ReviewPendingTrigger (dependency_drift or
	// rests_on_missing). The banner's rule is brief-dependency-drift unless
	// a listed claim is gone, in which that banner names
	// brief-rests-on-missing.
	Trigger string
	Claims  []BriefChangedClaimView
	// ChangedAt is claim id → the short UTC date on that claim's Rests on
	// row ("changed 18 Sep"), or "changed" when the engine has no clock.
	ChangedAt map[string]string
}

// BriefChangedClaimView is one changed rests_on claim's banner.
type BriefChangedClaimView struct {
	ID      string
	Missing bool
	// Href is the claim card's section id, the claim's own id.
	Href string
	// ChangedAt is the current wording's approval time, RFC 3339, or "".
	ChangedAt string
	Long      string
	Short     string
	// Retained is false when neither the brief's receipt nor any other
	// snapshot the store keeps holds the baseline wording. The banner then
	// says "earlier wording not available" and still links the claim.
	Retained bool
	// Diff is the redline of the claim's wording (summary, body, steps)
	// through approvaledit.Passages and markdown.Render, wrapped the way
	// BriefDiff.ChangesHTML wraps a brief. Empty when there is nothing to
	// compare (unavailable, or a gone claim with no retained baseline).
	Diff template.HTML
}

// briefReviewView builds the review-pending banners, or nil when the brief
// is not pending. A draft never reaches here: Evaluate never sets
// ReviewPending on one.
func briefReviewView(r briefs.Review) *BriefReviewView {
	if !r.ReviewPending {
		return nil
	}
	v := &BriefReviewView{
		Trigger:   r.ReviewPendingTrigger,
		ChangedAt: map[string]string{},
	}
	for _, c := range r.ChangedClaims {
		item := BriefChangedClaimView{
			ID:        c.ID,
			Missing:   c.Missing,
			Href:      c.ID,
			ChangedAt: c.ChangedAt,
			Retained:  c.Baseline != nil,
		}
		if t, err := time.Parse(time.RFC3339Nano, c.ChangedAt); err == nil {
			t = t.UTC()
			item.Long, item.Short = t.Format("2 Jan 2006"), t.Format("2 Jan")
		}
		if item.Retained && c.Current != nil {
			before, after := wordingSource(c.Baseline), wordingSource(c.Current)
			hunks, _ := approvaledit.Passages(endPassage(before), endPassage(after), func(s string) string {
				return string(markdown.Render(s))
			})
			item.Diff = BriefDiff{Hunks: hunks}.ChangesHTML()
		} else if item.Retained {
			item.Diff = template.HTML(string(markdown.Render(wordingSource(c.Baseline))))
		}
		if item.Short != "" {
			v.ChangedAt[c.ID] = "changed " + item.Short
		} else {
			v.ChangedAt[c.ID] = "changed"
		}
		v.Claims = append(v.Claims, item)
	}
	return v
}

// wordingSource is the claim markdown the redline compares: the summary, the
// body and the steps, each a passage of its own when present. The claim
// renderer treats "#" as text, so a claim's title line stays a title line.
func wordingSource(w *briefs.Wording) string {
	if w == nil {
		return ""
	}
	var parts []string
	if s := strings.TrimSpace(w.Summary); s != "" {
		parts = append(parts, s)
	}
	if b := strings.TrimSpace(w.Body); b != "" {
		parts = append(parts, b)
	}
	if len(w.Steps) > 0 {
		var b strings.Builder
		for i, s := range w.Steps {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(s)
		}
		if s := strings.TrimSpace(b.String()); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

// reviewPillState is the lock_state BriefLockPillHTML reads: "review" for a
// standing locked brief that is pending and not also edited (edited outranks
// it, as briefMark does).
func reviewPillState(r briefs.Review) string {
	if r.LockState == briefs.LockLocked && r.ReviewPending {
		return "review"
	}
	return string(r.LockState)
}
