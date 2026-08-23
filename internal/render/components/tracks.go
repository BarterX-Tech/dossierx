// tracks.go renders the one thing a track page shows that no other surface
// does: the compact reference row for a claim the track CITES.
//
// WHY A CITED CLAIM IS A ROW AND NEVER A CARD. A track is the second axis over
// a corpus already partitioned by module (see model.TrackRole), so most of the
// claims a track needs are claims some module already guarantees. Rendering
// their bodies here would put a second copy of every one of those sentences on
// the page — and a second copy that no lock, no lint and no reviewer can tell
// from the first is precisely the drift this whole tool exists to catch. The
// row is therefore a pointer and carries no prose at all: the claim's label,
// where it lives, and what state it is in.
//
// It reuses writeClaimRef rather than building its own anchor so a track page
// labels a claim exactly as an edges footer does — the same three elision
// tiers, the same data-claim-id and title hooks, the same styling — and so a
// future change to how a claim is named lands on both at once.
package components

import (
	"html"
	"html/template"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TrackCitedClaim is one cited claim as a track row needs it: its id, and
// enough of its state to draw a pill. It is deliberately not a model.Claim —
// the row renders no body, so holding one would invite a later change to start
// rendering it.
type TrackCitedClaim struct {
	ID            string
	Status        model.Status
	ReviewPending bool
}

// TrackCitedListHTML renders one track's cited claims as a list of compact
// reference rows, in the order given.
//
// EVERY ROW CARRIES A PILL, including a healthy locked one — which is the
// deliberate difference from targetPillHTML, whose actionable-only gate keeps
// an edges footer quiet. A track page is read to answer "is this finished?",
// and a page that showed a pill only on the unfinished claims would answer it
// by absence: the reader would have to know that nothing means locked. The
// completion count in the track's own header is derived from these same
// states, so the pills are what let a reader check that number instead of
// trusting it.
//
// The reader's context is empty on purpose. A track page belongs to no module
// and no facet, so nothing in a target's "Widget · Contract ›" prefix is
// redundant with where the reader is standing — every row keeps its full
// prefix, which is the whole point of a page that gathers claims from
// everywhere.
//
// Values are hand-escaped, because a FuncMap-returned template.HTML bypasses
// html/template's automatic escaping and nothing downstream will escape them.
func TrackCitedListHTML(claims []TrackCitedClaim) template.HTML {
	if len(claims) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<ul class="track-cites">`)
	for _, c := range claims {
		b.WriteString(`<li class="track-cite">`)
		// nil statuses: this row draws its own pill below, and letting
		// writeClaimRef draw a second one from a catalog lookup would print
		// two pills on exactly the claims that most need one to be read.
		writeClaimRef(&b, c.ID, "", "", nil)
		b.WriteString(` <span class="pill `)
		b.WriteString(pillClass(c.Status, c.ReviewPending))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(TrackClaimStateLabel(c.Status, c.ReviewPending)))
		b.WriteString(`</span></li>`)
	}
	b.WriteString(`</ul>`)
	return template.HTML(b.String())
}

// TrackClaimStateLabel is the word one claim's pill shows on a track page:
// its status, except that a locked claim flagged for re-review says so
// instead.
//
// "review_pending" replaces "locked" rather than joining it because the pill
// has room for one word and that is the word that changes what the reader
// should do. The completion count still counts such a claim as locked — see
// internal/render's trackCompletion for why those two answers differ on
// purpose.
func TrackClaimStateLabel(status model.Status, reviewPending bool) string {
	if status == model.StatusLocked && reviewPending {
		return "review_pending"
	}
	return string(status)
}
