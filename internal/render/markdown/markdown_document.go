// markdown_document.go is DOCUMENT MODE (NIT-204): the renderer as a brief
// uses it.
//
// A brief — briefs/<folder>/<slug>.md, see internal/briefs — is prose beside the
// claims, and it differs from a claim body in exactly two ways that reach this
// package:
//
//   - IT OWNS ITS PAGE. A claim card sits inside viewer chrome that already
//     spends h1 and h2, which is why every claim entry point renders "#" and
//     "##" as literal text (see atxHeading). A brief is the whole page it is
//     shown on, so its title is its h1 and its sections are its h2s.
//   - ITS IMAGES SIT BESIDE IT. A claim's images live under its own assets/
//     directory; a brief's live in its own folder, next to the .md, because
//     the folder is one level deep and holds nothing else (internal/briefs
//     refuses anything else). DocumentImageSrc is that folder's gate.
//
// Everything else is the claim-body ceiling unchanged — the same block scan,
// the same inline pass, the same escaping boundary. Citations are OFF: a brief
// has no sources block for a "[n]" to address, so every marker stays the
// literal text it is in a comment.
//
// Both differences are fields on bodyPolicy whose zero value is the refusal, so
// Render, RenderClaimBody and RenderInline cannot acquire either one by a
// forgotten argument. RenderDocument is the only entry point that sets them.
package markdown

import (
	"html"
	"html/template"
	"path"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/urlsafe"
)

// RenderDocument converts a brief's body (the markdown after its frontmatter)
// into safe HTML in document mode: headings at every level from 1 to 6, and
// images named by DocumentImageSrc rewritten onto assets.
//
// A zero AssetPrefix renders every image as literal text, exactly as it does
// for a claim — the capability exists only once a caller has somewhere to
// serve the file from.
func RenderDocument(body string, assets AssetPrefix) template.HTML {
	var b strings.Builder
	renderBlocks(&b, strings.Split(body, "\n"), true, bodyPolicy{
		img: imagePolicy{
			enabled: assets != "",
			sibling: true,
			prefix:  string(assets),
		},
		document: true,
	})
	return template.HTML(b.String())
}

// DocumentImages returns the canonical file names RenderDocument would emit
// for body, in document order, duplicates included. It is ClaimBodyImages for
// the document gate, and it exists for the same reason: internal/briefs has to
// know which images a brief references — to refuse an image nothing
// references, and to count a brief's images against its cap — and deriving
// that from a second, simpler scanner would be a second set of rules that
// could disagree with what the page shows. An image inside a fenced example, or
// one whose src the gate refuses, is absent here because it is absent there.
func DocumentImages(body string) []string {
	var refs []string
	var b strings.Builder
	renderBlocks(&b, strings.Split(body, "\n"), true, bodyPolicy{
		img:      imagePolicy{enabled: true, sibling: true, refs: &refs},
		document: true,
	})
	return refs
}

// DocumentRefusedImages returns every image src in body that DocumentImageSrc
// refuses, as authored, in document order: "PIC.svg", "./pic.svg",
// "../x.svg", "https://...". RenderDocument writes each as the literal text of
// its "![alt](src)", so internal/briefs raises it as a finding rather than let
// an image the author meant to show read as nothing wrong. It runs the same
// block and inline passes, so an image in a fenced example is absent here as it
// is absent from the page.
func DocumentRefusedImages(body string) []string {
	var refused []string
	var b strings.Builder
	renderBlocks(&b, strings.Split(body, "\n"), true, bodyPolicy{
		img:      imagePolicy{enabled: true, sibling: true, refused: &refused},
		document: true,
	})
	return refused
}

// DocumentTitle returns the raw text of the first level-1 heading RenderDocument
// would emit for body, and false when it emits none. "Raw" means as authored,
// before the inline pass: the title is a label (a list row, a payload field),
// never HTML. A "# x" inside a fenced example is not a heading and so is never
// the title — the same block scan answers both questions.
func DocumentTitle(body string) (string, bool) {
	var title string
	var b strings.Builder
	renderBlocks(&b, strings.Split(body, "\n"), true, bodyPolicy{
		document: true,
		title:    &title,
	})
	return title, title != ""
}

// DocumentText returns the text a reader of the rendered document reads: every
// word RenderDocument puts on the page, and none of the markdown it took to put
// it there. Image references, link targets, emphasis delimiters and fence
// markers are syntax and are gone; heading, paragraph, list, table and code
// text all remain. Each tag the renderer emits becomes one space, so two blocks
// never run their words together, and entities are decoded back to the
// characters the reader sees.
//
// It is the meter internal/briefs counts a brief's words with. Counting the
// raw markdown instead charged a brief three "words" for every
// "![Flow](flow.png)" and one for every "**" pair, which is a cap measuring the
// author's punctuation rather than the reader's time.
func DocumentText(body string) string {
	// A placeholder prefix turns every accepted image into a tag, which the
	// strip below then removes whole — alt text included, since alt is
	// attribute content and not text on the page.
	rendered := string(RenderDocument(body, "i/"))
	var b strings.Builder
	inTag := false
	for i := 0; i < len(rendered); i++ {
		c := rendered[i]
		switch {
		case c == '<':
			inTag = true
			b.WriteByte(' ')
		case c == '>' && inTag:
			inTag = false
		case !inTag:
			b.WriteByte(c)
		}
	}
	return html.UnescapeString(b.String())
}

// maxDocumentImageBytes bounds an accepted document src, for the reason
// maxAssetSrcBytes bounds a claim's: every consumer downstream works on a value
// whose size is known.
const maxDocumentImageBytes = 128

// DocumentImageSrc is the whole document image-src gate: it reports whether raw
// may be rendered as a brief's image and, when it may, returns the canonical
// file name.
//
// It is ImageSrc's three rules with co-location moved from "under assets/" to
// "beside the brief":
//
//  1. LEGALITY (urlsafe.IsRelativePath), tested after entity-decoding exactly
//     as ImageSrc tests it.
//  2. CO-LOCATION: the src is ONE path segment — a file in the brief's own
//     folder. No directory, no "./", no "..": a brief folder is one level deep
//     and a deeper path is refused by internal/briefs anyway.
//  3. SHAPE AND EXTENSION: the name is a stem drawn from [a-z0-9-] followed by
//     one of the six image extensions, lowercase. That is the brief folder's
//     own naming rule (internal/briefs refuses any other file name), so an
//     accepted src always names a file the folder is allowed to hold.
//
// IT REFUSES; IT NEVER REWRITES, for ImageSrc's reason: the normalisation guard
// below treats a strippable byte as a refusal rather than cleaning it away.
func DocumentImageSrc(raw string) (name string, ok bool) {
	if !urlsafe.IsRelativePath(raw) {
		return "", false
	}
	s := html.UnescapeString(raw)
	if s == "" || len(s) > maxDocumentImageBytes {
		return "", false
	}
	if urlsafe.StripCtrlAndSpace(s) != s {
		return "", false
	}
	if strings.Contains(s, "/") {
		return "", false
	}
	ext := path.Ext(s)
	if !assetExtensions[ext] || ext != strings.ToLower(ext) {
		return "", false
	}
	if !DocumentNameStem(strings.TrimSuffix(s, ext)) {
		return "", false
	}
	return s, true
}

// DocumentNameStem reports whether stem is a legal brief-tree name: non-empty
// and drawn entirely from [a-z0-9-]. It is exported because internal/briefs
// applies the same rule to folder names, brief file names and image file names,
// and one rule spelled once cannot disagree with itself about which image a
// brief is allowed to reference.
func DocumentNameStem(stem string) bool {
	if stem == "" {
		return false
	}
	for i := 0; i < len(stem); i++ {
		c := stem[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
		default:
			return false
		}
	}
	return true
}

// IsDocumentImageExt reports whether ext (with its dot) is one of the six image
// extensions a brief folder may hold, lowercase — the same set the claim gate
// accepts, spelled once in assetExtensions.
func IsDocumentImageExt(ext string) bool {
	return assetExtensions[ext] && ext == strings.ToLower(ext)
}
