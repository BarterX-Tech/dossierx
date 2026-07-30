// Package urlsafe is the ONE definition of "could this URL leave the page's
// origin" in this repository.
//
// WHY IT IS A LEAF PACKAGE. It imports nothing from this module — only "html"
// and "strings" from the standard library — so every boundary that has to
// decide what a URL means can import it without creating a cycle. Before it
// existed the same decision was spelled four times, in four files, in two
// packages:
//
//	internal/lint/raw_html_scope.go        a regexp over "^\s*(scheme:|//)"
//	internal/lint/markdown_scan.go         mdImageSrcOffOrigin / mdIsNetworkPath
//	internal/render/markdown/markdown.go   isNetworkPath / schemeOf
//	internal/render/markdown/markdown_images.go  ImageSrcOffOrigin
//
// Three of those agreed. The fourth — the regexp guarding the <img src> of a
// human-REVIEWED layout: mockup claim, the one place this repo emits author
// markup unescaped — recognised "//host" but not the backslash spellings of the
// same authority, so "\\host", "/\host" and "\/host" all read as relative paths
// and lint clean. A browser normalises "\" to "/" in the authority position of
// an http/https URL, so all three load off-origin. Four copies is why: the rule
// was strengthened in one of them and the other three had no reason to notice.
//
// The functions below are therefore deliberately the STRONGEST of the four, not
// their average, and there is nowhere left for a fifth copy to be born.
//
// # The two normalisations, and why only one of them decodes entities
//
// StripCtrlAndSpace drops every byte a browser drops before it resolves a URL
// (code point <= 0x20, plus DEL). Every gate here applies it, because a gate
// that skips it decides about a string the browser will never see:
// "ht\ttp://host" is a relative path to a naive reader and an absolute URL to
// Chrome.
//
// Entity decoding is applied by IsOffOrigin and IsRelativePath and deliberately
// NOT by SchemeOf, IsNetworkPath or IsAllowedHref. The split follows the escaping
// of the surface each one guards:
//
//   - IsOffOrigin guards values that reach the browser as MARKUP — the src of an
//     <img> inside a mockup claim's raw_html, which is emitted verbatim. There
//     "&#47;&#47;host" IS "//host" by the time it is resolved, so the gate must
//     decode before it decides.
//   - IsAllowedHref guards a markdown anchor's href, which is emitted through
//     html.EscapeString. There "&#106;avascript:" reaches the browser as the
//     literal text "&amp;#106;avascript:" and is never a scheme, so decoding
//     first would only make the gate refuse strings that are already inert.
//
// Adding a decode to the anchor gate would not close a hole; it would change a
// verified-clean allowlist for no gain. Keeping it off is a decision, not an
// omission.
package urlsafe

import (
	"html"
	"strings"
)

// StripCtrlAndSpace removes every ASCII control byte and space (code point
// <= 0x20, plus DEL 0x7f) from anywhere in s.
//
// It is exported because callers outside this package need to ask the same
// question this package asks internally — internal/render/markdown's ImageSrc
// refuses any src whose bytes the strip would have CHANGED, which is only
// answerable against the identical strip. Two strips that dropped different
// byte sets would put that guard and this package's gates into disagreement,
// which is the whole failure mode this package exists to end.
func StripCtrlAndSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if c := s[i]; c > 0x20 && c != 0x7f {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// SchemeOf extracts url's lower-cased URI scheme (the part before the first
// ':', e.g. "http" or "mailto"), returning ok=false when url carries no scheme
// at all (a relative path, a fragment, a network-path reference).
//
// Control bytes and spaces are stripped from ANYWHERE in url first, so
// "  JavaScript:", "java\tscript:" and "java\nscript:" all normalise to the
// scheme "javascript" — the evasion a leading-anchored regexp over the raw
// bytes misses. The scheme is then read per RFC 3986's grammar
// (ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":"); any other byte before a
// ':' means there is no scheme.
func SchemeOf(url string) (scheme string, ok bool) {
	stripped := StripCtrlAndSpace(url)
	for i := 0; i < len(stripped); i++ {
		c := stripped[i]
		if c == ':' {
			if i == 0 {
				return "", false
			}
			return strings.ToLower(stripped[:i]), true
		}
		if isSchemeAlpha(c) {
			continue
		}
		if i > 0 && (isDigit(c) || c == '+' || c == '-' || c == '.') {
			continue
		}
		// A non-scheme byte before any ':': url is relative, not schemed.
		return "", false
	}
	return "", false
}

// IsNetworkPath reports whether url is a protocol-relative (RFC 3986
// "network-path", "//host...") reference: scheme-less, yet not same-origin — it
// resolves against the page's own scheme to an arbitrary host.
//
// THE BACKSLASH CLAUSE IS THE FIX THIS PACKAGE EXISTS FOR. A browser normalises
// "\" to "/" in the authority position of a URL under a special (http/https)
// scheme, so "\\host", "/\host" and "\/host" are exactly as off-origin as
// "//host". A check that tests for the literal two bytes "//" blocks one
// spelling of four.
func IsNetworkPath(url string) bool {
	stripped := StripCtrlAndSpace(url)
	return len(stripped) >= 2 && isSlashByte(stripped[0]) && isSlashByte(stripped[1])
}

// IsOffOrigin is THE gate: it reports whether raw could reference anything other
// than a same-origin relative path.
//
// It is defined BY CONSTRUCTION rather than by a denylist of known-bad prefixes,
// because a denylist is only ever as good as the last spelling someone thought
// of. raw is off-origin when, after entity-decoding and stripping every byte
// <= 0x20 and 0x7f, it is:
//
//   - empty (it references nothing, so it is not a relative path either);
//   - carrying a "#" or a "?" anywhere (a fragment or query is not a plain path
//     reference, and neither belongs in the asset and mockup-image positions
//     this gate guards);
//   - carrying ANY explicit scheme — http:, https:, javascript:, data:,
//     vbscript:, or one nobody has heard of. The gate never asks WHICH scheme;
//     see IsAllowedHref for the one place that question is asked;
//   - beginning with "/" or "\" — one test that covers the root-relative "/foo"
//     and all four authority spellings "//", "/\", "\\", "\/" at once, since the
//     first byte of every one of them is one of the two slash bytes.
//
// Everything else is a relative reference and is not off-origin. "assets/x.png",
// "./x.png", "../x.png" and "x.png" all pass; whether they are ALLOWED is a
// further question their callers answer (see IsRelativePath for traversal, and
// internal/render/markdown's ImageSrc for co-location, shape and extension).
func IsOffOrigin(raw string) bool {
	s := StripCtrlAndSpace(html.UnescapeString(raw))
	if s == "" {
		return true
	}
	if strings.ContainsAny(s, "#?") {
		return true
	}
	if _, hasScheme := SchemeOf(s); hasScheme {
		return true
	}
	return isSlashByte(s[0])
}

// IsRelativePath is IsOffOrigin's complement PLUS a traversal clause: raw spells
// a relative path that stays on this origin AND contains no ".." segment.
//
// The ".." test splits segments on BOTH slash bytes, for the same reason
// IsNetworkPath reads both: "a\..\x.png" is "a/../x.png" to a browser, so a
// splitter that only knew "/" would see the single segment "a\..\x.png" and find
// no traversal in it.
//
// It is legality, not co-location: "x.png" is a legal relative path and may
// still be refused by a caller that requires it to live somewhere specific.
func IsRelativePath(raw string) bool {
	if IsOffOrigin(raw) {
		return false
	}
	s := StripCtrlAndSpace(html.UnescapeString(raw))
	for _, seg := range strings.FieldsFunc(s, isSlashRune) {
		if seg == ".." {
			return false
		}
	}
	return true
}

// IsAllowedHref reports whether url may be emitted as a live anchor's href.
// Only http, https, mailto and scheme-less same-origin references (a relative
// path or a "#fragment") are permitted; every other scheme — notably
// javascript:, data: and vbscript: — is refused, as is a scheme-less
// network-path reference in any of its four slash spellings.
//
// It is the ONE place in this package that asks which scheme a URL has, and it
// is deliberately narrower than "not off-origin": a "#fragment" and a "?query"
// are legitimate in an anchor and are not in an image src, so IsOffOrigin
// refuses both and this does not. The two gates are different questions about
// the same string, which is why they are two functions rather than one with a
// flag.
//
// It does NOT entity-decode. See this package's doc comment: the anchor href it
// guards is emitted through html.EscapeString, so an entity in it is never
// decoded by the browser and can never become a scheme.
func IsAllowedHref(url string) bool {
	scheme, ok := SchemeOf(url)
	if !ok {
		// No scheme: a relative path or #fragment is allowed — but NOT a
		// protocol-relative "//host" network-path, which carries no scheme yet
		// resolves against the page's own scheme to an arbitrary off-origin
		// host.
		return !IsNetworkPath(url)
	}
	switch scheme {
	case "http", "https", "mailto":
		return true
	default:
		return false
	}
}

// --- byte helpers ----------------------------------------------------------

// isSlashByte treats "\" as "/" — see IsNetworkPath for why that is not a
// convenience but the rule.
func isSlashByte(c byte) bool { return c == '/' || c == '\\' }

// isSlashRune is isSlashByte for strings.FieldsFunc. Both separators are ASCII,
// so no multi-byte rune can be one.
func isSlashRune(r rune) bool { return r == '/' || r == '\\' }

func isSchemeAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c byte) bool       { return c >= '0' && c <= '9' }
