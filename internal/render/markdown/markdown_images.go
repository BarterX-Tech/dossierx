// markdown_images.go holds the one construct in this package that is not
// available everywhere the renderer is: the markdown image, "![alt](src)".
//
// THE CAPABILITY IS OPT-IN, AND THAT IS THE WHOLE DESIGN (gate 0).
//
// Images render in a CLAIM body and never in a comment body. The obvious way to
// build that is a boolean parameter every caller passes, and it is the wrong way
// round: a caller that forgets the parameter gets the permissive answer, and
// there are four markdown.Render call sites across two packages — the shared
// component funcMap, comments.html, and internal/serve's commentToDTO twice —
// of which three are comment paths. So the default is INVERTED instead:
//
//	Render        renders no images. "![alt](src)" is escaped literal text.
//	RenderInline  renders no images, and cannot be told to. It is the
//	              layout:table cell entry point, where amendment A3's
//	              per-surface table says images never render.
//	RenderClaimBody is the ONLY image-permitting entry point.
//
// Every existing caller keeps calling Render and is correct by changing nothing.
// The failure mode of forgetting to opt in is a missing diagram in a claim,
// which the human reviewing the page sees immediately; the failure mode of
// forgetting to opt OUT would be the capability leaking into reviewer-authored
// comment text, which nobody would notice. Those are not symmetric, and this is
// which way round the asymmetry is spent.
//
// THE SRC GATE IS DEFINED BY CONSTRUCTION, not by a blocklist (amendment A4). A
// src is accepted only if, after entity-decoding, it spells a relative path
// under this claim's own "assets/" directory out of a closed character set and
// ends in one of six extensions. Everything else — every scheme, every authority
// prefix (including the backslash spellings a browser normalizes to "/"), every
// root-relative path, every ".." segment, every query or fragment, every byte
// outside the closed set — is refused, and a refused src renders as ESCAPED
// LITERAL TEXT rather than as a broken <img> or as the anchor the "!" used to
// leave behind.
//
// IT REFUSES; IT NEVER REWRITES. The control-and-space strip that the LEGALITY
// half needs (a tab smuggled into a scheme name is invisible to a browser) is
// applied to the legality question only. It is deliberately NOT applied ahead of
// the shape rule, because a strip that runs before a shape test cannot fail it —
// it deletes the evidence, and "assets/team photo.png" stops being a refusal and
// becomes a DIFFERENT, legal, possibly existing filename that the tag would then
// name and the route would then serve. See ImageSrc's normalisation guard.
//
// WHAT IS EMITTED IS WHAT WAS VALIDATED. ImageSrc returns the CANONICAL path,
// and that one string is what the tag carries, what ClaimBodyImages reports and
// what internal/serve's image route builds its allowlist key and its filesystem
// path from. There is no second normalization step anywhere downstream that
// could disagree with this one — which is the property that makes the route's
// allowlist a statement about the rendered page rather than about the disk.
package markdown

import (
	"html"
	"html/template"
	"path"
	"strings"
)

// AssetPrefix is the URL prefix an accepted image src is rewritten onto: what
// the browser must ask for to be served that claim's file. internal/render/
// components builds it per claim and internal/serve answers it; this package
// treats it as opaque bytes and escapes it in attribute context like any other.
//
// THE ZERO VALUE IS NOT "SERVE IT RELATIVE" — it is "this claim has no image
// capability", and RenderClaimBody renders every image as literal text under it.
// That matters because the prefix is derived from the claim's identity, and a
// claim whose id cannot be put in a URL safely must lose images rather than
// acquire an ambiguous one.
//
// It is a named type rather than a bare string so it cannot be swapped with the
// body argument at a call site.
type AssetPrefix string

// AssetDirName is the fixed directory a claim's images must live in, relative
// to the directory holding the claim file. It is deliberately not configurable:
// a per-project asset root would be a second way to answer a question the
// co-location rule exists to have exactly one answer to, and the human
// reviewing a diagram would then have to read the config to know whether it is
// in scope. internal/lint's asset-scope rule states the same constant for the
// same reason.
const AssetDirName = "assets"

// maxAssetSrcBytes bounds an accepted src. A path longer than this is not a
// diagram reference, and the bound keeps every downstream consumer — the
// allowlist key, the filesystem join, the emitted attribute — working on a
// value whose size is known before any of them sees it.
const maxAssetSrcBytes = 512

// assetExtensions is the fixed extension allowlist from gate 0. It is the same
// six internal/lint's asset-scope rule enforces.
var assetExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".svg":  true,
}

// RenderClaimBody converts a CLAIM's body markdown into safe HTML with images
// enabled. It is Render plus exactly one construct, and it is the only entry
// point in this package that has it.
//
// assets is where this claim's images are served from; a zero AssetPrefix
// renders every image as literal text, exactly as Render does. See this file's
// doc comment for why the capability is spelled as a separate entry point
// rather than as a parameter on Render.
func RenderClaimBody(body string, assets AssetPrefix) template.HTML {
	var b strings.Builder
	renderBlocks(&b, strings.Split(body, "\n"), true, imagePolicy{
		enabled: assets != "",
		prefix:  string(assets),
	})
	return template.HTML(b.String())
}

// ClaimBodyImages returns the canonical asset paths RenderClaimBody would emit
// for body, in document order, relative to the directory holding the claim
// file ("assets/diagram.png"). A refused src, an image inside a fenced example
// and an image on any surface the renderer would not render one on are all
// absent, because this runs the SAME block and inline passes the renderer runs
// and records what they accept.
//
// It exists for internal/serve's image route, whose allowlist has to be a
// statement about what the rendered page actually references. Deriving that
// from a second, simpler scanner would be a second set of rules that could
// disagree with the first; deriving it from this one cannot.
//
// The rendered HTML is discarded. That is not free, but the caller caches the
// result across requests and the alternative — a route that walks the
// filesystem instead — is the thing this design exists to avoid.
func ClaimBodyImages(body string) []string {
	var refs []string
	var b strings.Builder
	renderBlocks(&b, strings.Split(body, "\n"), true, imagePolicy{enabled: true, refs: &refs})
	return refs
}

// imagePolicy is the image capability, threaded through the block and inline
// passes as an ordinary value.
//
// THE ZERO VALUE RENDERS NO IMAGES. Every function that takes one takes it
// explicitly, so a new block construct cannot acquire the capability by
// forgetting to mention it — it acquires the zero value, which is the refusal.
type imagePolicy struct {
	// enabled is the opt-in. Nothing below is consulted when it is false.
	enabled bool
	// prefix is the AssetPrefix an accepted path is rewritten onto.
	prefix string
	// refs, when non-nil, collects every accepted path in document order.
	// It is a pointer because the policy is copied by value down the whole
	// call tree and every copy must append to the same slice.
	refs *[]string
}

// accept applies the whole gate to one authored src and returns the URL to
// emit. A false second result is the refusal the caller renders as literal
// text; there is no third answer and in particular no "emit it anyway".
func (p imagePolicy) accept(src string) (string, bool) {
	if !p.enabled {
		return "", false
	}
	rel, ok := ImageSrc(src)
	if !ok {
		return "", false
	}
	if p.refs != nil {
		*p.refs = append(*p.refs, rel)
	}
	return p.prefix + rel, true
}

// imgHTML builds one image tag. The tag, the class value and both attribute
// delimiters are fixed literals in this package; src has passed ImageSrc and is
// escaped here in attribute context, and alt is raw author bytes escaped in the
// same context and parsed as nothing at all.
//
// ALT IS NEVER RUN THROUGH THE INLINE PASS. It is attribute content, and the
// inline pass is a document-context producer that emits tags — putting its
// output in an attribute is precisely the mistake markdown.go's escaping
// boundary is written to make impossible. Amendment A3's per-surface permission
// table says the same thing from the other end: "image alt — never".
func imgHTML(src, alt string) string {
	return `<img class="md-img" src="` + html.EscapeString(src) +
		`" alt="` + html.EscapeString(alt) + `">`
}

// --- the src gate ----------------------------------------------------------

// ImageSrc is the whole image-src gate: it reports whether raw may be rendered
// as an image at all and, when it may, returns the CANONICAL path relative to
// the directory holding the claim file.
//
// It is three rules, and all three must pass:
//
//  1. LEGALITY (ImageSrcLegal): a relative reference with no scheme, no
//     authority prefix, no leading "/", no ".." segment, no "#" and no "?",
//     tested after entity-decoding and control-byte stripping.
//  2. CO-LOCATION: the path lies under this claim's own "assets/" directory.
//     Not a sibling facet's, not a shared top-level pool — the claim's own.
//     Nested subdirectories under assets/ are in scope.
//  3. SHAPE AND EXTENSION: every segment is drawn from the closed set
//     [A-Za-z0-9._-] and does not begin with ".", and the final segment's
//     extension is one of the six — tested on the entity-decoded bytes AS
//     AUTHORED, with nothing stripped out of them first.
//
// Rule 3 is doing more work than it looks like it is. It is what makes the
// returned string safe to put in an attribute, safe to use as a map key, safe
// to join onto a filesystem path and safe to compare against a URL path
// byte-for-byte — no percent-encoding to decode, no quote to close an
// attribute, no separator that means one thing to a browser and another to the
// operating system. The cost is a documented ceiling: an asset filename may not
// contain a space or any other character outside that set, and neither may the
// src as written — "![a]( assets/x.png )" is refused, not trimmed.
//
// IT REFUSES; IT NEVER REWRITES. That sentence is the whole reason rule 3 runs
// on the ENTITY-DECODED BYTES AND NOTHING ELSE. The control-and-space strip that
// ImageSrcLegal needs (a scheme name with a tab in it is still a scheme to a
// browser) must not run ahead of the shape rule here, because a strip applied
// before a shape test cannot fail: it deletes the evidence. "assets/team
// photo.png" would not be refused, it would BECOME "assets/teamphoto.png" — a
// different, legal, quite possibly existing file, which the tag would then name,
// the allowlist would index and the route would serve, with nothing anywhere
// reporting that the author's reference had been replaced. The explicit
// normalisation guard below therefore treats a strippable byte as a REFUSAL
// rather than as something to clean up, and assetSegment's closed set states the
// same rule a second time on the bytes that survive it.
func ImageSrc(raw string) (rel string, ok bool) {
	if !ImageSrcLegal(raw) {
		return "", false
	}
	// Entity-decoded, and NOT stripped: "&#32;" is a space the author wrote in
	// a second spelling, and it is refused on exactly the same terms as one
	// typed directly.
	s := html.UnescapeString(raw)
	if s == "" || len(s) > maxAssetSrcBytes {
		return "", false
	}
	// THE NORMALISATION GUARD. Anything the strip would have removed makes the
	// src unspellable, so the src is refused rather than silently repaired.
	if stripCtrlAndSpace(s) != s {
		return "", false
	}
	segs := strings.Split(s, "/")
	out := make([]string, 0, len(segs))
	for _, seg := range segs {
		if seg == "." {
			// "./assets/x.png" and "assets/./x.png" are the same reference
			// as the canonical spelling, so they normalize to it.
			continue
		}
		if !assetSegment(seg) {
			return "", false
		}
		out = append(out, seg)
	}
	// At least "assets" plus a file, and the first segment must be the fixed
	// asset directory: "assets/" alone is a directory, not an image.
	if len(out) < 2 || out[0] != AssetDirName {
		return "", false
	}
	if !assetExtensions[strings.ToLower(path.Ext(out[len(out)-1]))] {
		return "", false
	}
	return strings.Join(out, "/"), true
}

// assetSegment reports whether seg is one legal path segment: non-empty, drawn
// entirely from [A-Za-z0-9._-], and not beginning with a "." (which rules out
// "..", every dotfile, and every hidden directory in one clause).
//
// It runs on the bytes AS AUTHORED (entity-decoded only). A space, a tab, a NUL
// and a DEL are all outside the set and all make the segment illegal — see
// ImageSrc's normalisation guard for why they must reach here at all rather than
// being cleaned away upstream.
func assetSegment(seg string) bool {
	if seg == "" || seg[0] == '.' {
		return false
	}
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// ImageSrcLegal is amendment A4's gate, DEFINED BY CONSTRUCTION rather than by
// negation: a legal image src is a relative path with no scheme, no authority
// prefix formed from two of "/" or "\", no leading "/", no ".." segment, no "#"
// and no "?", tested after entity-decoding and stripping every byte <= 0x20 and
// 0x7f.
//
// It is legality, NOT co-location: "diagram.png" is legal and is still refused
// by ImageSrc, because it is not under assets/. The split is the one
// internal/lint's two content rules divide a bad src along — an off-origin src
// is markdown-sanity's finding alone, since it is not a path and has no
// directory to be in or out of, while a ".." traversal is also asset-scope's,
// because that is the canonical co-location mistake and asset-scope's message is
// the one that explains where images must live.
//
// This function and ImageSrcOffOrigin are exported so there is ONE callable
// definition of the rule rather than two that can drift. internal/lint carries
// a private mirror of them today (markdown_scan.go's mdImageSrcLegal, and
// raw_html_scope.go's weaker mockupAbsoluteURLPattern, which recognizes only
// "//" as an authority prefix and would classify "/\evil.example/p.png" as
// relative); converging both onto these is a cross-file change this phase
// records rather than makes.
func ImageSrcLegal(raw string) bool {
	if ImageSrcOffOrigin(raw) {
		return false
	}
	s := stripCtrlAndSpace(html.UnescapeString(raw))
	for _, seg := range strings.FieldsFunc(s, isSlashRune) {
		if seg == ".." {
			return false
		}
	}
	return true
}

// ImageSrcOffOrigin is ImageSrcLegal MINUS its ".." clause: it answers only
// "could this reference leave the origin, or is it not a path at all" — a
// scheme, an authority prefix, a root-relative leading slash, a query or a
// fragment.
//
// THE BACKSLASH CLAUSE IS LOAD-BEARING and is the half a regex over "//" gets
// wrong: browsers normalize "\" to "/" in the authority position of a URL under
// a special (http/https) scheme, so "/\host", "\\host" and "\/host" are exactly
// as off-origin as "//host". One test covers all four spellings plus the plain
// root-relative "/foo", because the first byte of each is one of the two slash
// bytes.
//
// The entity-decode and the control-byte strip are what stop the two cheap
// evasions: "&#47;&#47;host" is "//host" to a browser, and a tab smuggled into
// the middle of a scheme name is invisible to one. Both are decided here,
// before any byte is read as structure.
func ImageSrcOffOrigin(raw string) bool {
	s := stripCtrlAndSpace(html.UnescapeString(raw))
	if s == "" {
		return true
	}
	if strings.ContainsAny(s, "#?") {
		return true
	}
	if _, hasScheme := schemeOf(s); hasScheme {
		return true
	}
	return isSlashByte(s[0])
}

// isSlashRune is isSlashByte for strings.FieldsFunc. Both separators are ASCII,
// so no multi-byte rune can be one.
func isSlashRune(r rune) bool { return r == '/' || r == '\\' }
