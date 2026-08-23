// sources.go renders the evidence half of the shared claim footer: the
// "sources:" row, one entry per model.Source, each one the anchor a "[n]"
// citation marker in the body lands on.
//
// WHY PROVENANCE IS FOOTER METADATA AND NOT BODY PROSE. A source is a fact
// ABOUT the claim, in the same family as governed_by and migrated_from — it is
// authored in the claim's YAML, signed by the lock hash, and checked by the
// source-* lints. Writing it into the body instead would put it beyond every
// one of those: nothing could tell a citation from a sentence, so nothing
// could report a dead one. That is the whole reason model.Source exists (see
// its doc comment), and this row is the reading end of it.
//
// THE ANCHOR IS THE CONTRACT BETWEEN TWO PACKAGES. internal/render/markdown
// emits "#<prefix><ref>" for a resolved marker; this file emits
// id="<prefix><ref>" on the row that answers it. Both spellings come from
// ClaimSourceAnchorPrefix and there is no second place either is built, which
// is what makes the two agreeing a property of the code rather than of a
// reviewer noticing.
package components

import (
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
	"github.com/BarterX-Tech/dossierx/internal/urlsafe"
)

// sourceAnchorInfix separates a claim's id from a source's ref in the anchor
// id the two halves of the citation feature share.
//
// A KNOWN, DELIBERATE AMBIGUITY: a project that named a claim
// "mod.facet.thing-source-1" would collide with source 1 of the claim
// "mod.facet.thing". Nothing here can rule that out, because a claim id is
// author input and this is not a namespace. The consequence is bounded — two
// elements with one id, so a deep link resolves to whichever the document has
// first — and the alternative, a separator no id may contain, does not exist:
// the id character set is exactly [A-Za-z0-9._-] and every one of those is
// already legal inside a slug.
const sourceAnchorInfix = "-source-"

// sourceHashPrefixLen is how much of an internal source's sha256 the footer
// shows. It is a RECOGNITION aid, never a verification one: the check that
// matters is source-internal-drift's, which reads the whole hash. Twelve hex
// characters is enough for a human comparing two rows on one page and short
// enough that nobody mistakes it for something they are meant to re-derive.
const sourceHashPrefixLen = 12

// ClaimSourceAnchorPrefix returns the id prefix a claim's source rows are
// addressed by — the value both the citation markers in its body and the
// id= attributes on its footer rows are built from — and whether c can carry
// citation anchors at all.
//
// THE ID IS UNTRUSTED BYTES HERE, exactly as it is in ClaimAssetURLPrefix, and
// for a reason one step narrower: the anchor has to be the SAME STRING read as
// an HTML id attribute and as a URL fragment. A closed character set is what
// makes those two readings identical bytes with nothing to decode on either
// side — no "#" to end the fragment early, no space or control byte a parser
// normalizes away, no quote to close the attribute.
//
// A refused id loses its citation LINKS, not its sources: the rows still
// render, with the reader's evidence intact and their "[n]" markers left as
// the literal text they were before this feature. That is the same degradation
// every other refusal in this family gives, and it is the right way round —
// the evidence is the point, the hyperlink is the convenience.
func ClaimSourceAnchorPrefix(c model.Claim) (string, bool) {
	if !routableClaimID(c.ID) {
		return "", false
	}
	return c.ID + sourceAnchorInfix, true
}

// ClaimSourceAnchorID returns the anchor id for one of c's sources, or "" when
// c's id cannot carry one (see ClaimSourceAnchorPrefix). It is exported so
// internal/render can name the very ids it has to strip off a duplicated copy
// of a claim without re-deriving the scheme.
func ClaimSourceAnchorID(c model.Claim, ref int) string {
	prefix, ok := ClaimSourceAnchorPrefix(c)
	if !ok {
		return ""
	}
	return prefix + strconv.Itoa(ref)
}

// claimCitations builds the body renderer's citation capability for c: which
// refs its "[n]" markers may resolve, and the anchor they resolve to.
//
// A claim with no sources — every claim in every project that has not adopted
// the feature — gets the zero value, under which markdown renders its body
// byte-identically to how it did before citations existed. That is the whole
// zero-cost contract, and it is one branch rather than a policy anyone has to
// remember.
func claimCitations(c model.Claim) markdown.Citations {
	prefix, ok := ClaimSourceAnchorPrefix(c)
	if !ok || len(c.Sources) == 0 {
		return markdown.Citations{}
	}
	refs := make([]int, 0, len(c.Sources))
	for _, s := range c.Sources {
		refs = append(refs, s.Ref)
	}
	return markdown.NewCitations(prefix, refs)
}

// writeSourcesRow writes the footer's "sources:" <li> and the nested list of
// one entry per source, in the order the author declared them.
//
// DECLARED ORDER, NEVER SORTED BY REF. Refs are author-assigned precisely so
// that reordering the list does not renumber the prose (see model.Source.Ref),
// which means the list's order is itself authored information — and a render
// that re-sorted it would quietly disagree with the YAML a reviewer is reading
// beside the page.
//
// Every interpolation point is hand-escaped, for the same reason the rest of
// EdgesHTMLWithLinks is: a FuncMap-returned template.HTML bypasses
// html/template's automatic escaping, so nothing downstream will escape these.
func writeSourcesRow(b *strings.Builder, c model.Claim) {
	b.WriteString(`<li class="claim-sources">sources:<ul class="claim-source-list">`)
	for _, s := range c.Sources {
		b.WriteString(`<li class="claim-source"`)
		if id := ClaimSourceAnchorID(c, s.Ref); id != "" {
			b.WriteString(` id="`)
			b.WriteString(html.EscapeString(id))
			b.WriteString(`"`)
		}
		b.WriteString(`><span class="claim-source-ref">[`)
		b.WriteString(strconv.Itoa(s.Ref))
		b.WriteString(`]</span> `)

		if s.IsInternal() {
			writeInternalSource(b, s)
		} else {
			// EXTERNAL IS THE FALLBACK, not a third branch. Kind is author
			// input and source-shape is what refuses an unknown one; a claim
			// that reaches render carrying "kind: aricle" must still show its
			// evidence, and the external shape — a title and whatever anchor
			// it can honestly make of the URL — is the one that degrades to
			// something readable when the anchoring fields are absent.
			writeExternalSource(b, s)
		}

		writeSourceNote(b, "supports", s.Supports)
		writeSourceNote(b, "does_not_support", s.DoesNotSupport)
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul></li>`)
}

// writeExternalSource writes a URL-anchored source: its title as a link, then
// the host it points at and the date it was read.
//
// THE HREF PASSES THE SAME GATE AN AUTHORED MARKDOWN LINK DOES —
// urlsafe.IsAllowedHref, the one definition this repository has — and a URL it
// refuses produces NO ANCHOR AT ALL rather than an escaped one. The title
// renders as plain text and the refused URL rides along as escaped literal
// text beside it, so the reader still sees exactly what the author wrote and
// can see that it is not a link. That is the same refusal-not-repair rule the
// image src gate states at length in markdown_images.go.
func writeExternalSource(b *strings.Builder, s model.Source) {
	title := html.EscapeString(s.Title)
	if title == "" {
		// A source with no title is a lint finding (source-shape), not a
		// reason to render an empty anchor nobody can click.
		title = html.EscapeString(s.URL)
	}
	if s.URL != "" && urlsafe.IsAllowedHref(s.URL) {
		b.WriteString(`<a class="claim-source-title" href="`)
		b.WriteString(html.EscapeString(s.URL))
		b.WriteString(`">`)
		b.WriteString(title)
		b.WriteString(`</a>`)
	} else {
		b.WriteString(`<span class="claim-source-title">`)
		b.WriteString(title)
		b.WriteString(`</span>`)
		if s.URL != "" {
			b.WriteString(` <code class="claim-source-anchor">`)
			b.WriteString(html.EscapeString(s.URL))
			b.WriteString(`</code>`)
		}
	}

	var meta []string
	if host := sourceHost(s.URL); host != "" {
		meta = append(meta, host)
	}
	if s.AccessedOn != "" {
		meta = append(meta, "accessed "+s.AccessedOn)
	}
	if len(meta) > 0 {
		b.WriteString(` <span class="claim-source-meta">`)
		b.WriteString(html.EscapeString(strings.Join(meta, claimRefModuleSep)))
		b.WriteString(`</span>`)
	}
}

// writeInternalSource writes a repository-anchored source: its title, the file
// (and record) it names, and a recognizable prefix of the content hash that
// pins it.
//
// The path renders inside <code>, matching the .claim-implemented-in row above it,
// because both are the same kind of thing to a reader — a filename they can
// paste into an editor — and rendering one as prose and the other as code
// would suggest a difference that is not there.
func writeInternalSource(b *strings.Builder, s model.Source) {
	title := html.EscapeString(s.Title)
	if title == "" {
		title = html.EscapeString(s.Path)
	}
	b.WriteString(`<span class="claim-source-title">`)
	b.WriteString(title)
	b.WriteString(`</span>`)

	if s.Path != "" {
		b.WriteString(` <code class="claim-source-anchor">`)
		b.WriteString(html.EscapeString(s.Path))
		if s.RecordID != "" {
			b.WriteString(`#`)
			b.WriteString(html.EscapeString(s.RecordID))
		}
		b.WriteString(`</code>`)
	}
	if s.SHA256 != "" {
		b.WriteString(` <span class="claim-source-hash" title="`)
		b.WriteString(html.EscapeString(s.SHA256))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(shortHash(s.SHA256)))
		b.WriteString(`</span>`)
	}
}

// writeSourceNote writes one of the two authored boundary lines under a source
// entry, or nothing when the author left it unset.
//
// The text is hand-written prose that routinely names a claim id, a path or a
// quoted phrase, so it goes through the same INLINE-ceiling renderer every
// other prose field in this footer uses (markdown.RenderInline: code spans and
// links, no block constructs) rather than a bare html.EscapeString — the
// identical treatment governed.reason gets, and for the identical reason.
//
// THE THREE-LINE CLAMP AND WHY THE BUTTON SHIPS HIDDEN. A supports/
// does_not_support line is the one field in this footer with no natural length
// — it is an author quoting or paraphrasing a source — and a handful of long
// ones turn a claim's evidence into a wall. The note is therefore clamped to
// three lines with a show more/show less control, which is emitted here and
// starts `hidden`.
//
// Hidden is the load-bearing word. Whether prose exceeds three lines is a fact
// about the RENDERED BOX, not about the string: it depends on the reader's
// viewport, their font and their zoom, and this function knows none of the
// three. So the clamp is applied by the viewer's script, which can measure, and
// the button is revealed only on the notes that actually overflow — a one-line
// note keeps the bare look it has now.
//
// The direction of the degradation is deliberate and is the reason the clamp is
// not simply a static CSS rule. With no script — a printed page, a text
// browser, a reader who blocks it — this markup shows the WHOLE note and no
// control, because a truncation whose "show more" cannot run would hide a
// citation's stated limit behind a button that does nothing. Evidence is the
// point; the clamp is the convenience.
//
// The control carries NO id, and must not gain one: a claim is rendered a
// second time inside any track that owns it, and render.stripDuplicateClaimIDs
// removes only the ids it knows how to enumerate. Its handle on the note it
// governs is DOM position, which survives being copied.
func writeSourceNote(b *strings.Builder, label, text string) {
	if text == "" {
		return
	}
	b.WriteString(`<div class="claim-source-note"><div class="claim-source-note-body"><span class="claim-source-note-label">`)
	b.WriteString(label)
	b.WriteString(`:</span> `)
	b.WriteString(string(markdown.RenderInline(text)))
	b.WriteString(`</div><button class="claim-source-note-toggle" type="button" aria-expanded="false" data-collapse-label="`)
	b.WriteString(html.EscapeString(sourceNoteCollapseLabel))
	b.WriteString(`" hidden>`)
	b.WriteString(html.EscapeString(sourceNoteExpandLabel))
	b.WriteString(`</button></div>`)
}

// The two states of the note control. Both are written HERE, and the expanded
// one rides the button as a data attribute rather than being restated in the
// viewer's script, so the wording has one home and the two halves cannot come
// to disagree about it in a later edit.
const (
	sourceNoteExpandLabel   = "show more"
	sourceNoteCollapseLabel = "show less"
)

// sourceHost returns the host an external source's URL names, or "" when the
// URL does not parse or carries no host.
//
// It is shown because the DOMAIN is what a reader judges a citation by at a
// glance — whether the sentence rests on a vendor's own reference or on a
// forum post — and the full URL is usually too long to read in a footer. It is
// display only: nothing is fetched, nothing is resolved, and the href the
// reader actually follows is the authored URL, never this.
func sourceHost(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}

// shortHash returns the leading sourceHashPrefixLen characters of a hash, or
// the whole thing when it is shorter. A truncated hash is never compared
// against anything — see sourceHashPrefixLen.
func shortHash(sum string) string {
	if len(sum) <= sourceHashPrefixLen {
		return sum
	}
	return sum[:sourceHashPrefixLen]
}
