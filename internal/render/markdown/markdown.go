// Package markdown is the engine's single shared body renderer: one
// entry point, Render(body string) template.HTML, used identically by
// every claim component (card, table, steps, banner, list) instead of the
// three ad-hoc, inconsistent string transforms that used to live inline in
// render/components/components.go (renderBody's regex-only fence matcher,
// bodyLines' naive newline split, and steps.html's raw unescaped dump).
//
// This is deliberately not a general Markdown parser. It implements exactly
// the subset real claim corpora (and FORMAT.md's generic "body: markdown
// string" contract) actually need:
//
//   - Paragraphs — a run of non-blank lines separated by a blank line.
//   - Fenced code blocks — unchanged behavior from renderBody/bodyFence
//     (ported verbatim below, not rewritten): a ```-delimited span, with an
//     optional dropped info string on the opening line, becomes a real
//     <pre><code>escaped-content</code></pre>.
//   - Inline code spans — single backtick pairs become <code>escaped</code>,
//     new in this package.
//   - Inline links — [text](url) becomes <a href="url">text</a>, new in this
//     package. The url is held to a scheme allowlist: only http, https,
//     mailto, and scheme-less (relative-path or #fragment) hrefs are emitted
//     as anchors; a javascript:, data:, vbscript: (or any other) scheme is
//     neutralized to escaped literal text with no anchor. Both the url (in
//     attribute context) and the link text are HTML-escaped. See renderInline
//     and allowedScheme for the parsing/allowlist boundary.
//   - Unordered ("-"/"*") and ordered ("1.", "2.", ...) lists, with exactly
//     one level of nesting (an indented "-"/"*" or "N." item folded under
//     the immediately preceding top-level item) — new in this package, and
//     matches the real corpora's maximum depth (confirmed by scanning every
//     body in the corpus this package was first built against; nothing
//     goes deeper).
//
// Explicitly out of scope: bold/italic, headings, blockquotes,
// tables-in-markdown, more than one level of list nesting. None of these
// appear anywhere in the corpus this engine currently renders, and
// FORMAT.md's body field ("markdown string") makes no promise beyond
// "markdown", so trimming to this subset does not contradict the format
// spec's contract — it is a documented ceiling, not a silent gap.
//
// Pure stdlib (strings/html/html/template) — no new dependency.
package markdown

import (
	"html"
	"html/template"
	"regexp"
	"strings"
)

// bodyFence mirrors internal/render/components/components.go's bodyFence
// exactly (ported, not rewritten, per the round-3 plan's explicit
// instruction to reuse the existing-correct fence logic): it matches a
// fenced ```...``` code block, across lines, with an optional info string
// on the opening fence line (e.g. ```json — dropped, never rendered).
//
// Two known, deliberate scope notes carried over unchanged from the
// original:
//   - A same-line span like “ ```x``` “ is technically matched too (the
//     regex doesn't require a newline before the closing fence), but the
//     corpus never writes fences that way — every real fence is a
//     multi-line block, opening and closing markers each on their own
//     line — so this is an inherited, harmless generality, not a feature
//     this package exercises or relies on.
//   - An unclosed fence (no later "```") simply fails to match at all; the
//     text — including the literal opening ``` marker — falls through to
//     ordinary block/paragraph handling below, exactly like the original
//     renderBody's "no match => escaped as-is" behavior.
var bodyFence = regexp.MustCompile("(?s)```(?:[^\n]*\n)?(.*?)```")

// Render converts a claim's free-form Body markdown subset into safe HTML.
// It is the single shared body-rendering entry point every component that
// shows free text should route through, so fenced examples, inline code,
// and lists render identically everywhere instead of only in whichever
// component happened to get the last hand fix.
//
// Two-pass shape: bodyFence (the block pass' fence half) is applied once
// over the whole raw body first, exactly as renderBody did, splitting the
// body into fence spans (rendered verbatim, escaped, as <pre><code>) and
// the text between them. Each non-fence text segment is then handed to
// renderBlocks, a line scanner that further splits it into paragraphs and
// lists and runs the inline pass (backtick-span conversion) only over that
// paragraph/list-item text — never over fence content, which stays
// untouched raw-escaped source, matching the "no inline code parsing inside
// a code block" rule.
func Render(body string) template.HTML {
	var b strings.Builder
	last := 0
	for _, loc := range bodyFence.FindAllStringSubmatchIndex(body, -1) {
		start, end := loc[0], loc[1]
		codeStart, codeEnd := loc[2], loc[3]

		renderBlocks(&b, body[last:start])

		b.WriteString("<pre><code>")
		b.WriteString(html.EscapeString(body[codeStart:codeEnd]))
		b.WriteString("</code></pre>")

		last = end
	}
	renderBlocks(&b, body[last:])

	return template.HTML(b.String())
}

// orderedMarker matches a top-level-shaped "N. rest of line" list marker
// (e.g. "14. Context immutability: ..."); group 2 is the item's own text
// with the marker stripped. It is matched against a line's trimmed content
// regardless of the line's actual indentation — indentation only decides
// whether a matching line starts a new top-level item or a nested item
// under the current top-level item (see renderBlocks).
var orderedMarker = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)

// unorderedMarker matches a top-level-shaped "- rest of line" or "* rest of
// line" list marker; group 1 is the item's own text with the marker
// stripped. Same indentation-independent matching note as orderedMarker.
var unorderedMarker = regexp.MustCompile(`^[-*]\s+(.*)$`)

// listNode is one unordered or ordered list block: a flat sequence of
// items, each of which may carry exactly one nested list (one level of
// nesting only — a nested item's own nested field is never populated).
type listNode struct {
	ordered bool
	items   []*itemNode
}

// itemNode is one list item's accumulated raw (pre-inline-processing) text
// — continuation lines are folded into this via string concatenation as
// they're encountered — plus an optional one-level-deep nested list.
type itemNode struct {
	text   string
	nested *listNode
}

// renderBlocks scans a fence-free text segment and writes paragraphs and
// lists to b. It never sees fence markers — those are already extracted by
// Render's bodyFence pass — so every non-blank line here is either a list
// marker (top-level or, when indented under a currently open top-level
// item, nested), a continuation of the currently open item (indented, no
// marker, with a list open), or plain paragraph prose.
//
// Block-switch rule: encountering a different block type (a marker line
// while a paragraph is accumulating, a plain paragraph line while a list is
// open, or a blank line) flushes whatever was open before starting the new
// one — this is what makes list boundaries and paragraph boundaries
// deterministic without a lookahead.
func renderBlocks(b *strings.Builder, text string) {
	lines := strings.Split(text, "\n")

	var curList *listNode
	var curTop *itemNode
	var curNested *itemNode
	var paraLines []string

	flushParagraph := func() {
		if len(paraLines) == 0 {
			return
		}
		b.WriteString("<p>")
		b.WriteString(renderInline(strings.Join(paraLines, " ")))
		b.WriteString("</p>")
		paraLines = nil
	}
	flushList := func() {
		if curList == nil {
			return
		}
		writeList(b, curList)
		curList, curTop, curNested = nil, nil, nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushParagraph()
			flushList()
			continue
		}

		indent := leadingSpaces(line)

		if indent == 0 {
			if m := orderedMarker.FindStringSubmatch(trimmed); m != nil {
				flushParagraph()
				if curList == nil || !curList.ordered {
					flushList()
					curList = &listNode{ordered: true}
				}
				curTop = &itemNode{text: m[2]}
				curList.items = append(curList.items, curTop)
				curNested = nil
				continue
			}
			if m := unorderedMarker.FindStringSubmatch(trimmed); m != nil {
				flushParagraph()
				if curList == nil || curList.ordered {
					flushList()
					curList = &listNode{ordered: false}
				}
				curTop = &itemNode{text: m[1]}
				curList.items = append(curList.items, curTop)
				curNested = nil
				continue
			}
			// Plain paragraph line: any open list ends here (a
			// non-indented, non-marker line is never a list
			// continuation).
			flushList()
			paraLines = append(paraLines, trimmed)
			continue
		}

		// indent > 0.
		if curList != nil && curTop != nil {
			if m := orderedMarker.FindStringSubmatch(trimmed); m != nil {
				if curTop.nested == nil || !curTop.nested.ordered {
					curTop.nested = &listNode{ordered: true}
				}
				curNested = &itemNode{text: m[2]}
				curTop.nested.items = append(curTop.nested.items, curNested)
				continue
			}
			if m := unorderedMarker.FindStringSubmatch(trimmed); m != nil {
				if curTop.nested == nil || curTop.nested.ordered {
					curTop.nested = &listNode{ordered: false}
				}
				curNested = &itemNode{text: m[1]}
				curTop.nested.items = append(curTop.nested.items, curNested)
				continue
			}
			// Continuation line: fold into whichever item is
			// currently deepest-open, joined with a single space
			// (soft line break), matching the "wrapped bullet"
			// fix this package exists to make.
			if curNested != nil {
				curNested.text += " " + trimmed
			} else {
				curTop.text += " " + trimmed
			}
			continue
		}

		// Indented line with no open list item to fold into: treat
		// as ordinary paragraph prose rather than dropping it.
		flushList()
		paraLines = append(paraLines, trimmed)
	}

	flushParagraph()
	flushList()
}

// writeList renders a listNode as a real <ul>/<ol>, recursing at most once
// (into an item's optional nested list) since itemNode.nested is never
// itself populated on a nested item.
func writeList(b *strings.Builder, l *listNode) {
	tag := "ul"
	if l.ordered {
		tag = "ol"
	}
	b.WriteString("<" + tag + ">")
	for _, it := range l.items {
		b.WriteString("<li>")
		b.WriteString(renderInline(it.text))
		if it.nested != nil {
			writeList(b, it.nested)
		}
		b.WriteString("</li>")
	}
	b.WriteString("</" + tag + ">")
}

// RenderInline is the exported entry point for the inline pass — backtick
// code spans and [text](url) links, with everything else HTML-escaped. It is
// used by render/components' table-cell helper so a <td> renders the same
// inline markdown subset a card/list/steps body does, minus the block-level
// paragraph/list/fence handling Render layers on top (a table cell wants
// inline content, not a <p>-wrapped block).
//
// Its result is trusted template.HTML — html/template's auto-escaping is
// bypassed for it — which is safe precisely because renderInline is the
// escaping boundary: every emitted anchor's url has passed allowedScheme, and
// both the url (in attribute context) and the link text are HTML-escaped, so
// re-marking the string as template.HTML reintroduces no un-escaped,
// attacker-controlled content.
func RenderInline(text string) template.HTML {
	return template.HTML(renderInline(text))
}

// renderInline runs the inline pass over one paragraph's or list item's raw
// text in a single left-to-right scan recognizing two inline constructs —
// backtick code spans and [text](url) links — and HTML-escaping everything
// else. It is never called on fence content.
//
// The scan always advances to whichever construct opener (a backtick or a
// "[") comes first, so the two compose predictably: a link written INSIDE a
// code span (e.g. `[x](y)`) stays literal because the code span opens first
// and consumes it verbatim, and a "[" or "]" inside a code span is never
// mistaken for a link.
//
// Backtick pairing is a simple greedy scan: the first backtick looks for the
// next backtick later in the same text; if found, everything between becomes
// one <code> span and the scan resumes after the closing backtick; if not
// found (an odd/unclosed backtick), that backtick and everything after it is
// treated as literal (still HTML-escaped) rather than silently dropped — the
// same "no match => fall through as-is" philosophy bodyFence already uses for
// an unclosed triple-backtick fence.
//
// Link parsing is likewise fall-through-safe: a "[" that resolves to a
// complete [text](url) (see parseLink) is emitted as <a href="url">text</a>
// only when allowedScheme accepts the url; a rejected scheme
// (javascript:/data:/vbscript:/anything not http|https|mailto|relative)
// renders the whole match as escaped literal text with no anchor, and a "["
// that never completes a link is a literal character with the scan resuming
// just after it.
func renderInline(text string) string {
	var b strings.Builder
	i := 0
	for i < len(text) {
		bt := strings.IndexByte(text[i:], '`')
		br := strings.IndexByte(text[i:], '[')
		if bt == -1 && br == -1 {
			b.WriteString(html.EscapeString(text[i:]))
			break
		}

		// Advance to whichever construct opener comes first.
		if bt != -1 && (br == -1 || bt < br) {
			start := i + bt
			b.WriteString(html.EscapeString(text[i:start]))

			rel := strings.IndexByte(text[start+1:], '`')
			if rel == -1 {
				// Unclosed: this backtick and the remainder of the
				// text is literal.
				b.WriteString(html.EscapeString(text[start:]))
				break
			}
			end := start + 1 + rel
			b.WriteString("<code>")
			b.WriteString(html.EscapeString(text[start+1 : end]))
			b.WriteString("</code>")
			i = end + 1
			continue
		}

		// A "[" opener comes first: try to parse a [text](url) link.
		start := i + br
		b.WriteString(html.EscapeString(text[i:start]))

		matchLen, linkText, url, ok := parseLink(text[start:])
		if !ok {
			// Not a complete link: "[" is a literal char; resume the
			// scan just after it so any later construct still matches.
			b.WriteString(html.EscapeString(text[start : start+1]))
			i = start + 1
			continue
		}
		if allowedScheme(url) {
			b.WriteString(`<a href="`)
			b.WriteString(html.EscapeString(url))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(linkText))
			b.WriteString("</a>")
		} else {
			// Rejected scheme: emit the whole "[text](url)" match as
			// inert escaped literal text — never an anchor.
			b.WriteString(html.EscapeString(text[start : start+matchLen]))
		}
		i = start + matchLen
	}
	return b.String()
}

// parseLink attempts to match a [text](url) link anchored at the very start
// of s (s[0] must be '['). On success it returns the byte length of the whole
// "[text](url)" match, the raw (un-escaped) link text and url, and ok=true.
// On any structural failure — no closing ']', no '(' immediately after ']',
// or no closing ')' — it returns ok=false so the caller can treat the '[' as
// a literal character. Link text and url are taken verbatim up to the first
// ']' and first ')' respectively: this deliberately small grammar does not
// support nested brackets/parens or escaped delimiters, matching the rest of
// this package's subset.
func parseLink(s string) (matchLen int, linkText, url string, ok bool) {
	// s[0] == '['.
	rest := s[1:]
	closeBr := strings.IndexByte(rest, ']')
	if closeBr == -1 || closeBr+1 >= len(rest) || rest[closeBr+1] != '(' {
		return 0, "", "", false
	}
	linkText = rest[:closeBr]
	urlStart := closeBr + 2 // skip "]("
	closeParen := strings.IndexByte(rest[urlStart:], ')')
	if closeParen == -1 {
		return 0, "", "", false
	}
	url = rest[urlStart : urlStart+closeParen]
	// "[" + rest up to and including ")".
	matchLen = 1 + urlStart + closeParen + 1
	return matchLen, linkText, url, true
}

// allowedScheme reports whether url may be emitted as an anchor's href. Only
// http, https, mailto, and scheme-less (relative-path or #fragment) urls are
// permitted; every other scheme — notably javascript, data, and vbscript — is
// rejected so a rejected link renders as inert escaped text instead of an
// active anchor. This is the allowlist half of renderInline's escaping
// boundary and is deliberately evasion-resistant (see schemeOf).
func allowedScheme(url string) bool {
	scheme, ok := schemeOf(url)
	if !ok {
		// No scheme: a relative path or #fragment — always allowed.
		return true
	}
	switch scheme {
	case "http", "https", "mailto":
		return true
	default:
		return false
	}
}

// schemeOf extracts url's lower-cased URI scheme (the part before the first
// ':', e.g. "http" or "mailto"), returning ok=false when url has no scheme
// (a relative path or fragment).
//
// Before reading the scheme it removes every ASCII control byte and space
// (code point <= 0x20, plus DEL 0x7f) from anywhere in url, so an attacker
// cannot smuggle a javascript: scheme past allowedScheme with leading/
// trailing whitespace or embedded tabs/newlines/control chars (e.g.
// "  JavaScript:", "java\tscript:", "java\nscript:" all normalize to
// "javascript:"). The scheme is then read per RFC 3986's grammar
// (ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":") — anything that reaches a
// non-scheme byte before a ':' (e.g. "/", "#", "?") has no scheme.
func schemeOf(url string) (scheme string, ok bool) {
	var s strings.Builder
	s.Grow(len(url))
	for i := 0; i < len(url); i++ {
		if c := url[i]; c > 0x20 && c != 0x7f {
			s.WriteByte(c)
		}
	}
	stripped := s.String()

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
		if i > 0 && (isSchemeDigit(c) || c == '+' || c == '-' || c == '.') {
			continue
		}
		// A non-scheme byte before any ':': url is relative, not schemed.
		return "", false
	}
	return "", false
}

func isSchemeAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isSchemeDigit(c byte) bool { return c >= '0' && c <= '9' }

// leadingSpaces counts a line's leading space/tab run, used only to decide
// top-level vs. nested list-marker/continuation handling in renderBlocks —
// the exact indent width otherwise never matters (unlike a general Markdown
// parser, this package doesn't compute list levels from indent width, only
// from "indented at all, under a currently open top-level item, or not").
func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') {
		n++
	}
	return n
}
