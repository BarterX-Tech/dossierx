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
//   - Unordered ("-"/"*") and ordered ("1.", "2.", ...) lists, with exactly
//     one level of nesting (an indented "-"/"*" or "N." item folded under
//     the immediately preceding top-level item) — new in this package, and
//     matches the real corpora's maximum depth (confirmed by scanning every
//     body in the corpus this package was first built against; nothing
//     goes deeper).
//
// Explicitly out of scope: bold/italic, headings, links, blockquotes,
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
//   - A same-line span like `` ```x``` `` is technically matched too (the
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

// renderInline runs the inline pass over one paragraph's or list item's raw
// text: single backtick pairs become <code>escaped</code>, and everything
// else is HTML-escaped plain text. It is never called on fence content.
//
// Backtick pairing is a simple greedy left-to-right scan: the first
// backtick looks for the next backtick anywhere later in the same text: if
// found, everything between becomes one <code> span and the scan resumes
// after the closing backtick; if not found (an odd/unclosed backtick), that
// backtick and everything after it in this text is treated as literal,
// unescaped-for-code-purposes plain text (still HTML-escaped) rather than
// silently dropped — the same "no match => fall through as-is" philosophy
// bodyFence already uses for an unclosed triple-backtick fence.
func renderInline(text string) string {
	var b strings.Builder
	i := 0
	for i < len(text) {
		idx := strings.IndexByte(text[i:], '`')
		if idx == -1 {
			b.WriteString(html.EscapeString(text[i:]))
			break
		}
		start := i + idx
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
	}
	return b.String()
}

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
