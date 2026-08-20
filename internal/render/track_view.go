// track_view.go builds the viewer's track pages: one sidebar entry and one
// content section per track the project config declares.
//
// A TRACK SECTION IS A MODULE SECTION IN EVERY WAY THE VIEWER CHROME CAN SEE,
// and that is the design rather than an accident of reuse. shell.html's
// show/hide machinery is keyed on two classes — .module-section for the thing a
// sidebar tab reveals, .claim-group for the thing inside it that a deep link
// resolves to — and every lookup map, every hash rule and every fragment-swap
// re-init is derived from those two by query. A track section carries both, so
// it inherits deep linking, tab state, the hash contract and the SSE re-render
// with no second implementation and nothing new to keep in step. What makes it
// a TRACK is what it holds, not how it is switched to.
//
// THE COMPLETION PILL REPORTS AND NEVER GATES. Nothing in this file is read by
// the lock ledger, by a lint, or by the release driver; the number exists so a
// reader can see how far a feature has got without opening every claim in it.
// A track that says "incomplete" blocks nothing, and a track that says
// "complete" authorizes nothing — the gate is the gate, and a viewer badge is
// not a second one. That separation is why the count is computed here, at
// render time, from the claims as they are, rather than stored anywhere.
package render

import (
	"fmt"
	"html/template"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// trackSectionIDPrefix namespaces a track's element ids away from every module
// slug, so a project that declares a track and a module with the same name gets
// two sections rather than one id claimed twice. It is part of the hash
// contract — "#track-<slug>" is what a shared link carries — so it may not
// change without changing every link anyone has already sent.
const trackSectionIDPrefix = "track-"

// trackClaimsIDSuffix names the .claim-group inside a track section. A track
// has exactly one, because a track has no facets: the suffix exists only so the
// inner group's id cannot collide with its own parent section's.
const trackClaimsIDSuffix = "-claims"

// TrackSection is one declared track as shell.html renders it: a sidebar tab,
// a header stating what the track is and how far it has got, the claims it owns
// rendered whole, and the claims it merely cites rendered as pointers.
type TrackSection struct {
	// ID is the track section's element id and hash target
	// ("track-<slug of the track id>"), and ClaimsID is the .claim-group
	// nested inside it. Both are slugified rather than used raw because a
	// track id is author input and an element id is not.
	ID       string
	ClaimsID string

	// Title and Summary are config.Track's own fields, rendered as authored.
	// Summary is empty for a track that declares none, and shell.html then
	// renders no summary element at all.
	Title   string
	Summary string

	// StateClass is one of the existing .pill modifiers (ps/pv/pw) and
	// StateLabel the text inside it — see trackCompletion for the mapping and
	// for why "complete" and "no claim needs re-review" are two questions.
	StateClass string
	StateLabel string

	// OwnedClaims are the track's own claims, already rendered through the
	// same per-layout partials a module section uses, with their duplicate
	// element ids stripped (see stripDuplicateClaimIDs): the canonical,
	// id-bearing copy of every claim stays in its module's facet, because a
	// claim's module is still what guarantees it.
	OwnedClaims []template.HTML

	// CitedHTML is the compact reference list for claims this track cites,
	// or "" when it cites none. NEVER a rendered body — see components/
	// tracks.go for why a second copy of a claim's prose is the exact defect
	// this tool exists to catch.
	CitedHTML template.HTML

	// Empty is true when the track has neither owned nor cited claims. A
	// declared track with nothing in it is a real and useful state — it is
	// what a track looks like the day someone declares it — so it renders a
	// page saying so rather than vanishing from the nav.
	Empty bool
}

// buildTrackSections turns cfg.Tracks into the sections shell.html renders, in
// DECLARATION ORDER.
//
// Declaration order, never sorted: the order tracks appear in project config is
// authored information (a reader put the feature they care about first), and it
// is the same order config.TrackIDs and the CLI's track leaves use. Sorting
// here would make the viewer disagree with both.
//
// A project that declares no tracks gets nil, and shell.html then emits not one
// byte of track markup — the zero-cost contract this feature is held to, and
// the thing tests/fixture_staleness_test.go would turn red over.
func buildTrackSections(cat *catalog.Catalog, cfg *config.Config, renderedByID map[string]template.HTML) []TrackSection {
	if cfg == nil || len(cfg.Tracks) == 0 || cat == nil {
		return nil
	}

	out := make([]TrackSection, 0, len(cfg.Tracks))
	for _, t := range cfg.Tracks {
		owned, cited := partitionTrackClaims(cat, t.ID)

		section := TrackSection{
			ID:      trackSectionIDPrefix + slugify(t.ID),
			Title:   t.Title,
			Summary: t.Summary,
			Empty:   len(owned) == 0 && len(cited) == 0,
		}
		section.ClaimsID = section.ID + trackClaimsIDSuffix
		if section.Title == "" {
			// A track with no title is a config-validation finding, not a
			// reason to render a nameless tab nobody can identify. The id is
			// the one name that is always there.
			section.Title = t.ID
		}
		section.StateClass, section.StateLabel = trackCompletion(owned, cited)

		for _, c := range owned {
			section.OwnedClaims = append(section.OwnedClaims,
				stripDuplicateClaimIDs(renderedByID[c.ID], c))
		}

		rows := make([]components.TrackCitedClaim, 0, len(cited))
		for _, c := range cited {
			rows = append(rows, components.TrackCitedClaim{
				ID:            c.ID,
				Status:        c.Status,
				ReviewPending: c.ReviewPending,
			})
		}
		section.CitedHTML = components.TrackCitedListHTML(rows)

		out = append(out, section)
	}
	return out
}

// partitionTrackClaims splits the claims belonging to one track into the ones
// it owns and the ones it cites, each in the same order newGroup puts a
// facet's claims in (model.OrderClaims: explicit Order first, then the stable
// catalog order behind it).
//
// A claim that declares the SAME track twice, once in each role, counts as
// owned and is not also listed as cited — the ownership assertion is the
// stronger of the two, and listing the claim in both halves of its own track
// page would read as two claims. track-multi-owner is what reports the
// authoring mistake; this is only what keeps the page from compounding it.
func partitionTrackClaims(cat *catalog.Catalog, trackID string) (owned, cited []model.Claim) {
	for _, c := range cat.Claims {
		if !c.InTrack(trackID) {
			continue
		}
		if c.OwnedTrackID() == trackID {
			owned = append(owned, c)
			continue
		}
		cited = append(cited, c)
	}
	return orderClaims(owned), orderClaims(cited)
}

// trackCompletion turns a track's membership into the pill shell.html renders
// in its header: the CSS class and the text.
//
// COMPLETE MEANS EVERY CLAIM IS LOCKED — owned and cited alike. Citing is not
// a lesser membership for this purpose: a feature whose trigger is signed off
// but whose retry contract is still a draft is not finished, and a count that
// only looked at owned claims would say it was.
//
// A LOCKED CLAIM FLAGGED review_pending STILL COUNTS AS LOCKED, and the pill
// says so separately rather than by silently lowering the number. Those are two
// different questions — "has a human signed this off" and "has something it
// depends on moved since" — and folding the second into the first would make
// one number answer both and be checkable against neither. The warn class is
// what carries the second answer, so a track reading "complete" while something
// needs re-reading is never a green badge over a yellow fact.
//
// A track with no claims at all reads "empty", not "complete · 0 / 0 locked".
// A ratio over nothing is true and useless, and a green pill on a track nobody
// has written yet is the one reading this number must not produce.
func trackCompletion(owned, cited []model.Claim) (class, label string) {
	total := len(owned) + len(cited)
	if total == 0 {
		return "pv", "empty · no claims yet"
	}

	locked, pending := 0, 0
	count := func(claims []model.Claim) {
		for _, c := range claims {
			if c.Status == model.StatusLocked {
				locked++
				if c.ReviewPending {
					pending++
				}
			}
		}
	}
	count(owned)
	count(cited)

	state := "incomplete"
	class = "pv"
	if locked == total {
		state = "complete"
		class = "ps"
	}
	label = fmt.Sprintf("%s · %d / %d locked", state, locked, total)
	if pending > 0 {
		class = "pw"
		label += fmt.Sprintf(" · %d review_pending", pending)
	}
	return class, label
}
