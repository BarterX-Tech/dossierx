// images.go carries the ONE value this package contributes to the image
// feature: the URL a claim's images are addressed by.
//
// WHY THE URL IS BUILT HERE. internal/render/markdown decides WHETHER a src may
// be rendered and what its canonical path is; it deliberately knows nothing
// about routes. internal/serve decides what to answer for a URL. Between them
// somebody has to say what the URL IS, and it has to be one statement, because
// the emitter and the server agreeing about it by coincidence is exactly the
// class of bug the co-location rule exists to remove. That statement is
// AssetRoutePrefix plus ClaimAssetURLPrefix, and internal/serve imports them
// rather than spelling the route a second time.
//
// WHY THE CLAIM ID KEYS IT. An image lives beside its claim file
// (filepath.Dir(SourcePath) + "/assets/"), so the URL has to name a claim.
// SourcePath itself is an absolute filesystem path — putting it in a URL would
// leak the author's home directory into every page and make the same corpus
// render differently on two machines — and it is not available to a template
// anyway. The id is the claim's identity everywhere else in the engine, it is
// short, it is what a reader would type, and internal/serve can resolve it back
// to a directory from the claims it has already loaded.
package components

import (
	"html/template"

	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// AssetRoutePrefix is the fixed first path segment of every claim-image URL and
// the route internal/serve mounts. It is a constant rather than a setting for
// the same reason "assets" is: a second way to spell it is a second thing a
// reviewer would have to check.
const AssetRoutePrefix = "/claim-assets/"

// claimMarkdown renders one of a claim's own prose fields — Body, or one entry
// of Steps — with the two claim-scoped constructs enabled for that claim:
// images, and the "[n]" citation markers that address its own sources.
//
// It is bound into funcMap as "claimMarkdown" and is the ONLY binding in this
// package that permits either. It takes the whole claim rather than just the
// text because both capabilities are properties of the claim — where its
// assets live, which refs its footer answers to, and whether it can be
// addressed at all — not of the string.
//
// A claim ClaimAssetURLPrefix refuses gets the zero AssetPrefix, and a claim
// with no sources (or an id ClaimSourceAnchorPrefix refuses) gets the zero
// Citations; markdown.RenderClaimBody renders the corresponding construct as
// escaped literal text under either — the same degradation a refused src gets
// and the same one a comment body gets. There is no branch here that could
// produce a third outcome.
func claimMarkdown(c model.Claim, text string) template.HTML {
	prefix, _ := ClaimAssetURLPrefix(c)
	return markdown.RenderClaimBody(text, prefix, claimCitations(c))
}

// ClaimAssetURLPrefix returns the URL prefix under which claim c's images are
// served, and whether c can carry images at all.
//
// THE ID IS UNTRUSTED BYTES HERE, and that is not a hypothetical. "dossierx
// serve" does not run the lint suite — it loads, builds and renders (see
// internal/serve's disarmUngatedMockups for the same argument applied to
// raw_html) — so the id-shape rule that would normally hold an id to
// module.facet.slug has not run by the time this is called. An id is therefore
// held to a closed character set here rather than assumed to have passed one:
// it must be a non-empty run of [A-Za-z0-9._-] not beginning with a ".".
//
// That set is exactly what makes the result ONE URL PATH SEGMENT with no
// encoding in it. No "/" to split the segment, no "%" to decode into one, no
// "?" or "#" to end the path early, no ".." to traverse with, no space or
// control byte to normalize away. So the emitted src, the allowlist key and the
// request path are the same bytes at every step, and neither side has to decode
// anything to compare them.
//
// A refused id loses images. It does not get a mangled URL, an escaped one, or
// a shared fallback route — the capability is simply absent, which is the same
// answer every other refusal in this feature gives.
func ClaimAssetURLPrefix(c model.Claim) (markdown.AssetPrefix, bool) {
	if !routableClaimID(c.ID) {
		return "", false
	}
	return markdown.AssetPrefix(AssetRoutePrefix + c.ID + "/"), true
}

// ClaimImageSurfaces returns the claim's own prose fields that THIS PACKAGE'S
// partial for c.Layout actually routes through claimMarkdown — that is, the
// exact texts an <img> can come out of for this claim, in the order the partial
// renders them.
//
// IT EXISTS BECAUSE THE PERMISSION IS PER-PARTIAL AND THE CLAIM IS NOT. Body and
// Steps are the two image-permitting FIELDS (gate 0 amendment A3), but no claim
// has both of them rendered:
//
//	card, table, list, banner, mockup   Body
//	steps                              Body, then each step
//	tree                               nothing at all
//
// tree.html emits {{.Body}} raw inside a <pre> with no markdown call anywhere in
// it, and every layout except steps ignores .Steps completely (dossierx check
// calls that second case out in as many words: "the data will be silently
// dropped"). So "this claim carries a Body" is not the same statement as "this
// claim's Body can produce an image", and internal/serve's allowlist — whose
// entire safety argument is that a path it answers for is a path the renderer
// would have emitted — has to ask the second question, not the first.
//
// Asking it HERE rather than in internal/serve is the point: the answer is a
// property of the partials, which live in this package, so the table above sits
// next to the templates it describes and this package's own test derives the
// ground truth by EXECUTING each partial and comparing.
//
// An unknown layout gets nothing. internal/render treats one as a hard error
// (see renderClaims), so a claim that reaches here with a layout this package
// has no partial for renders no page at all, let alone an image.
//
// KNOWN CEILING: a project that overrides a partial via
// viewer.template_overrides can change which fields it renders, and this table
// does not read the override. The two directions are not symmetric. An override
// that renders MORE (a tree.html calling claimMarkdown) emits an <img> the
// allowlist does not carry, so the reviewer sees a broken image — visible, and
// the fail-closed direction. An override that renders LESS leaves an entry for a
// co-located, claim-referenced assets/ file that nothing displays, which is the
// same exposure an unedited claim already has. Neither reaches a file outside
// the claim's own assets/ directory.
func ClaimImageSurfaces(c model.Claim) []string {
	switch c.Layout {
	case model.LayoutCard, model.LayoutTable, model.LayoutList,
		model.LayoutBanner, model.LayoutMockup:
		return []string{c.Body}
	case model.LayoutSteps:
		out := make([]string, 0, 1+len(c.Steps))
		out = append(out, c.Body)
		return append(out, c.Steps...)
	default:
		// model.LayoutTree, and any layout with no partial.
		return nil
	}
}

// routableClaimID reports whether id can be one URL path segment verbatim.
func routableClaimID(id string) bool {
	if id == "" || id[0] == '.' || len(id) > maxRoutableIDBytes {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// maxRoutableIDBytes bounds the id segment for the same reason
// markdown.ImageSrc bounds a src: every downstream consumer works on a value
// whose size is known before it sees it.
const maxRoutableIDBytes = 256
