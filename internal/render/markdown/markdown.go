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
//   - Fenced code blocks — a ```-delimited block, recognized by the line
//     scanner in document order (see renderBlocks and fenceOpener), with an
//     optional dropped info string on the opening line, becoming a real
//     <pre><code>escaped-content</code></pre>. A fence indented under an open
//     list item renders INSIDE that item — including across a blank line,
//     which is how nearly all real documentation writes one.
//   - Backslash escapes — a closed escapable set, resolved inside the inline
//     scan (see isEscapable and renderInline).
//   - Inline code spans — a backtick run matched against a closing run of
//     equal length, so a literal backtick can appear inside inline code.
//   - Inline links — [text](url) becomes <a href="url">text</a>, new in this
//     package. The url is held to a scheme allowlist: only http, https,
//     mailto, and scheme-less relative-path or #fragment hrefs are emitted as
//     anchors. A javascript:, data:, vbscript: (or any other) scheme — AND a
//     scheme-less protocol-relative "//host" network-path, which resolves
//     off-origin — is neutralized to escaped literal text with no anchor. Both
//     the url (in attribute context) and the link text are HTML-escaped. See
//     renderInline and allowedScheme for the parsing/allowlist boundary.
//   - Unordered ("-"/"*") and ordered ("1.", "2.", ...) lists, with exactly
//     one level of nesting (an indented "-"/"*" or "N." item folded under
//     the immediately preceding top-level item) — new in this package, and
//     matches the real corpora's maximum depth (confirmed by scanning every
//     body in the corpus this package was first built against; nothing
//     goes deeper). Lists are LOOSE: a blank line between items, or between
//     an item and its indented content, does not end the list — only a
//     non-indented, non-marker line does (see renderBlocks' loose-list
//     rule).
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
	"sort"
	"strings"
)

// Render converts a claim's free-form Body markdown subset into safe HTML.
// It is the single shared body-rendering entry point every component that
// shows free text should route through, so fenced examples, inline code,
// and lists render identically everywhere instead of only in whichever
// component happened to get the last hand fix.
//
// One pass: the whole body goes to renderBlocks, a single line scanner that
// meets fences, paragraphs and lists in document order and runs the inline
// pass (escapes, code spans, links) only over paragraph and list-item text —
// never over fence content, which stays untouched raw-escaped source,
// matching the "no inline parsing inside a code block" rule.
//
// This replaces the pre-P1 two-pass shape, in which a whole-body regex
// (bodyFence) ripped every ``` span out of the raw body first and called
// renderBlocks separately on each leftover segment. Because renderBlocks
// holds its list state in locals, no open container could survive a fence:
// a fenced block under an ordered-list item split the list in two, restarted
// the numbering at 1 and ejected the code to top level. Recognizing fences
// inside the scanner is what makes containers survive them.
func Render(body string) template.HTML {
	var b strings.Builder
	renderBlocks(&b, body)
	return template.HTML(b.String())
}

// --- fenced code blocks ---------------------------------------------------
//
// THE FENCE RULE, in full (P1 / amendment A1).
//
// Opening: a fence opens only on a LINE that, after its optional leading
// whitespace run, begins with three or more backticks whose trailing info
// string contains no further backtick. Line-anchoring is the whole fix for
// the pre-P1 pass' escape-blindness: a line beginning with a backslash no
// longer begins with a backtick run, so a backslash defuses a fence marker
// for free, and a mid-line run such as "Use ```x``` inline" is an inline code
// span, not a block that swallows the prose around it. The pre-P1 regex
// matched the literal substring ``` ANYWHERE in the body, so both of those
// were block fences.
//
// Indentation allowance and placement: the opening line's indent decides
// which container the block belongs to. Indented (indent > 0) with a list
// item open, the fence renders INSIDE the deepest open item and the list
// carries on across it — the A1 repair. Otherwise the fence is a top-level
// block and, exactly as before P1, it closes any open paragraph or list
// first. There is no maximum opening indent, because this package has no
// indented-code-block construct for a deeper indent to mean instead.
//
// A blank line between the item and the fence changes none of this: it is
// the fence's own INDENT, never its distance from the item line, that says
// which container it belongs to. See renderBlocks' loose-list rule for how
// the list survives the blank line so there is still an open item for an
// indented fence to land in.
//
// Closing: the first later line whose optional leading whitespace is
// followed by a backtick run at least as long as the opener's and then only
// whitespace. Content lines have up to the opener's indent width of leading
// whitespace stripped, so a fence written inside a container is not rendered
// with its container's indentation baked in.
//
// Fall-through: an unclosed fence is not a fence at all. The opening line
// falls through to ordinary handling for whatever container it sits in
// (paragraph prose, or a continuation line of an open item), the scan
// resumes at the next line, and no content is dropped.
//
// Content: fence content is raw source bytes, HTML-escaped exactly once and
// never handed to renderInline — no escape resolution, no code spans, no
// links. The info string is dropped (P9 turns it into a language class).

// fenceOpener reports whether line opens a fenced code block, and if so its
// leading-whitespace width and the length of its backtick run.
func fenceOpener(line string) (indent, runLen int, ok bool) {
	indent = leadingSpaces(line)
	runLen = backtickRun(line, indent)
	if runLen < 3 {
		return 0, 0, false
	}
	// CommonMark: a backtick fence's info string may not contain a
	// backtick. Without this, "```x``` and more" would open a block fence
	// on a line that is plainly an inline span.
	if strings.IndexByte(line[indent+runLen:], '`') != -1 {
		return 0, 0, false
	}
	return indent, runLen, true
}

// fenceCloses reports whether line closes a fence opened with a run of
// openLen backticks.
func fenceCloses(line string, openLen int) bool {
	n := leadingSpaces(line)
	run := backtickRun(line, n)
	if run < openLen {
		return false
	}
	return strings.TrimSpace(line[n+run:]) == ""
}

// closerRuns precomputes the fence-closing capability of every line as a
// SUFFIX MAXIMUM: result[j] is the longest backtick run that can close a
// fence anywhere at or after line j (0 when no line from j on can close
// one). A line contributes its own run only when that run is followed by
// nothing but whitespace — i.e. exactly when fenceCloses would accept it —
// so for any opener length L, "some line at or after j closes an L-fence"
// is precisely result[j] >= L.
//
// This is what keeps fence scanning linear. scanFence used to walk EVERY
// remaining line for EVERY opener that never closes, allocating a fresh
// content slice each time: quadratic in both time and allocation, and
// reachable from reviewer-authored comment bodies (capped at 1 MiB by
// internal/serve) via markdown.Render, where it cost minutes of CPU on a
// single request while failing no test. With this table an opener that
// cannot close is rejected in O(1), and an opener that can close walks only
// the lines the scanner then consumes, so the whole document is one linear
// pass. It changes no rendered output: the early rejection fires exactly in
// the cases the old walk would have reported closed=false.
func closerRuns(lines []string) []int {
	// One extra slot so scanFence can index start+1 for the last line.
	suffixMax := make([]int, len(lines)+1)
	for j := len(lines) - 1; j >= 0; j-- {
		best := suffixMax[j+1]
		n := leadingSpaces(lines[j])
		if run := backtickRun(lines[j], n); run > best && strings.TrimSpace(lines[j][n+run:]) == "" {
			best = run
		}
		suffixMax[j] = best
	}
	return suffixMax
}

// scanFence consumes the fence opened at lines[start]. On success it returns
// the block's raw content (each line de-indented by up to indent, joined with
// "\n" and terminated by a trailing "\n" when non-empty), the index of the
// closing line, and closed=true. When no closing line exists it returns
// closed=false and the caller falls the opening line through as ordinary
// text.
//
// closers is the suffix-maximum table from closerRuns: consulting it first
// is what makes "this opener never closes" an O(1) answer instead of a walk
// to the end of the document (see closerRuns).
func scanFence(lines []string, closers []int, start, indent, openLen int) (content string, closeIdx int, closed bool) {
	if closers[start+1] < openLen {
		return "", start, false
	}
	var body []string
	for j := start + 1; j < len(lines); j++ {
		if fenceCloses(lines[j], openLen) {
			if len(body) == 0 {
				return "", j, true
			}
			return strings.Join(body, "\n") + "\n", j, true
		}
		body = append(body, stripIndent(lines[j], indent))
	}
	return "", start, false
}

// preBlock renders one fenced code block. Every author byte passes
// html.EscapeString and both tags are fixed literals — the same escaping
// boundary renderInline maintains.
func preBlock(content string) string {
	return "<pre><code>" + html.EscapeString(content) + "</code></pre>"
}

// backtickRun counts the run of consecutive backticks in s starting at i.
func backtickRun(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// stripIndent removes up to n leading space/tab bytes from line.
func stripIndent(line string, n int) string {
	i := 0
	for i < n && i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[i:]
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

// isListMarker reports whether a line's TRIMMED content opens a list item of
// either flavour. Only the loose-list rule (see renderBlocks) needs the
// question asked without also consuming the match.
func isListMarker(trimmed string) bool {
	return orderedMarker.MatchString(trimmed) || unorderedMarker.MatchString(trimmed)
}

// contentCol returns the column an item's content starts at: the item line's
// own indent plus the width of its marker and the whitespace run after it.
// trimmed is the whole trimmed marker line and itemText is the regex group
// holding what follows the marker, so the marker's width is just the
// difference. It is the threshold the loose-list rule compares a
// post-blank-line indent against.
func contentCol(indent int, trimmed, itemText string) int {
	return indent + len(trimmed) - len(itemText)
}

// listNode is one unordered or ordered list block: a flat sequence of
// items, each of which may carry exactly one nested list (one level of
// nesting only — a nested item's own nested field is never populated).
type listNode struct {
	ordered bool
	items   []*itemNode
}

// itemNode is one list item's content as an ORDERED sequence of blocks.
//
// Before P1 an item was "one string of accumulated text, plus at most one
// nested list", because nothing block-level could live inside an item. A
// fence can now (amendment A1), and an item that held its prose in a single
// field would render prose written AFTER a fence in front of it. Keeping the
// blocks in document order is what makes "a fence can no longer swallow or
// split the structure around it" true inside an item as well as around it.
//
// nested and textIdx are cursors onto whichever nested list / prose run in
// blocks is currently open, so continuation lines still know where to fold;
// both are invalidated (textIdx = -1) whenever a different block type opens.
type itemNode struct {
	blocks  []itemBlock
	nested  *listNode
	textIdx int
}

func newItem(text string) *itemNode {
	it := &itemNode{textIdx: -1}
	it.addText(text)
	return it
}

// addText folds a line of prose into the item's open prose run, joining with
// a single space (a soft line break), or starts a new run if the previous
// block was a fence or a nested list.
//
// The run is APPENDED TO, never concatenated. This used to be
//
//	it.blocks[it.textIdx].text += " " + s
//
// which is quadratic in both time and allocation: Go's string += builds a
// fresh string and copies the whole accumulation on every continuation line,
// so K lines under one item cost O(K^2). That cost was latent while a blank
// line unconditionally flushed the list — continuation lines went down the
// paragraph path, which has always accumulated into a slice. The loose-list
// rule made this accumulator reachable for the first time (an indented line
// after a blank line now legitimately continues the item), which turned a
// dormant O(K^2) into a live one on the untrusted comment-body surface: 1 MiB
// of "- x" followed by indented lines cost seconds of CPU and tens of
// gigabytes of allocation per render, re-paid on every GET /api/comments
// because handleListComments re-renders every stored comment.
//
// Appending segments and joining once at flush (see writeList) is linear and
// byte-identical: strings.Join(segs, " ") produces exactly the string the
// repeated `+= " " + s` produced, for any number of segments including one.
func (it *itemNode) addText(s string) {
	if it.textIdx >= 0 {
		blk := &it.blocks[it.textIdx]
		blk.segs = append(blk.segs, s)
		return
	}
	it.blocks = append(it.blocks, itemBlock{kind: blockText, segs: []string{s}})
	it.textIdx = len(it.blocks) - 1
}

// addBlock appends a non-prose block and closes the open prose run.
func (it *itemNode) addBlock(blk itemBlock) {
	it.blocks = append(it.blocks, blk)
	it.textIdx = -1
}

// itemBlockKind discriminates the three things that can appear inside a list
// item. Exactly one of itemBlock's payload fields is meaningful per kind.
type itemBlockKind uint8

const (
	blockText itemBlockKind = iota // raw prose, still to run through renderInline
	blockHTML                      // an already-rendered, already-escaped fenced code block
	blockList                      // a nested list, still being accumulated
)

// itemBlock's blockText payload is the prose run's SEGMENTS — one per source
// line folded in by addText — not the joined string. They are joined with a
// single space exactly once, at render time in writeList; see addText for why
// the join cannot happen incrementally.
type itemBlock struct {
	kind itemBlockKind
	segs []string
	html string
	list *listNode
}

// renderBlocks scans a whole body (or, once P4 lands, a container's
// interior) and writes paragraphs, fenced code blocks and lists to b. Every
// non-blank line is either a fence opener (see the fence rule above), a list
// marker (top-level or, when indented under a currently open top-level item,
// nested), a continuation of the currently open item (indented, no marker,
// with a list open), or plain paragraph prose.
//
// Block-switch rule: encountering a different block type (a marker line
// while a paragraph is accumulating, a plain paragraph line while a list is
// open, or a top-level fence) flushes whatever was open before starting the
// new one — this is what makes list boundaries and paragraph boundaries
// deterministic without a lookahead. The one construct that does NOT flush is
// an indented fence under an open list item: it is item content, so the list
// stays open across it.
//
// THE LOOSE-LIST RULE. A blank line is the exception to the block-switch
// rule: it always closes an open PARAGRAPH, but it does not by itself close
// an open LIST. It only records that a blank line was seen (pendingBlank);
// the next non-blank line decides, by its indent, whether the list was
// really over:
//
//   - indented to at least the deepest open item's content column — the
//     column its own text starts at, past its marker — the line is that
//     item's content and the list continues. This is what makes the
//     universal documentation shape work: an item, a blank line, then an
//     indented fence, then a blank line, then the next item.
//   - indented to at least the TOP-level item's content column but less than
//     the nested one's: the line dedented back out of the nested item, so
//     the nested cursor is dropped and the line continues the top-level
//     item.
//   - a list marker at any indent: the list continues with another item,
//     exactly as it would have without the blank line.
//   - anything else — a non-indented, non-marker line, which includes a
//     COLUMN-ZERO FENCE — ends the list, as it did before. A column-zero
//     fence is a top-level block by the fence rule above, blank line or not.
//
// Before this, a blank line was an unconditional flush, so an item followed
// by a blank line and an indented fence split the list in two, ejected the
// code to top level and restarted the numbering — the very failure the
// fence-in-the-scanner work exists to fix, still reachable through the one
// way almost everyone actually writes such a list.
func renderBlocks(b *strings.Builder, text string) {
	lines := strings.Split(text, "\n")
	closers := closerRuns(lines)

	var curList *listNode
	var curTop *itemNode
	var curNested *itemNode
	// Content columns of the two open items (see contentCol), used only by
	// the loose-list rule to judge a line that follows a blank line.
	var curTopCol, curNestedCol int
	var paraLines []string
	// pendingBlank: a blank line was seen with a list open, and the next
	// non-blank line has yet to say whether that ended the list.
	pendingBlank := false

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
		pendingBlank = false
		if curList == nil {
			return
		}
		writeList(b, curList)
		curList, curTop, curNested = nil, nil, nil
		curTopCol, curNestedCol = 0, 0
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushParagraph()
			// A blank line ends a paragraph but only ARMS the end of a
			// list; see the loose-list rule above.
			if curList != nil {
				pendingBlank = true
			}
			continue
		}

		indent := leadingSpaces(line)

		// Resolve a pending blank line before anything else looks at the
		// open containers, so every branch below sees the list state this
		// line actually belongs to.
		if pendingBlank {
			pendingBlank = false
			switch {
			case curNested != nil && indent >= curNestedCol:
				// Continues the nested item.
			case curTop != nil && indent >= curTopCol:
				// Continues the top-level item: the line dedented out
				// of any nested item.
				curNested = nil
			case isListMarker(trimmed):
				// Another item of the same list.
			default:
				flushList()
			}
		}

		// Fences are met in document order, so an open container survives
		// one. An unclosed fence is not a fence: it falls through to the
		// ordinary line handling below.
		if fIndent, runLen, ok := fenceOpener(line); ok {
			if content, closeIdx, closed := scanFence(lines, closers, i, fIndent, runLen); closed {
				if indent > 0 && curList != nil && curTop != nil {
					// Indented under an open item: the block is that
					// item's content, and the list stays open.
					target := curTop
					if curNested != nil {
						target = curNested
					}
					target.addBlock(itemBlock{kind: blockHTML, html: preBlock(content)})
				} else {
					flushParagraph()
					flushList()
					b.WriteString(preBlock(content))
				}
				i = closeIdx
				continue
			}
		}

		if indent == 0 {
			if m := orderedMarker.FindStringSubmatch(trimmed); m != nil {
				flushParagraph()
				if curList == nil || !curList.ordered {
					flushList()
					curList = &listNode{ordered: true}
				}
				curTop = newItem(m[2])
				curTopCol = contentCol(indent, trimmed, m[2])
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
				curTop = newItem(m[1])
				curTopCol = contentCol(indent, trimmed, m[1])
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
					curTop.addBlock(itemBlock{kind: blockList, list: curTop.nested})
				}
				curNested = newItem(m[2])
				curNestedCol = contentCol(indent, trimmed, m[2])
				curTop.nested.items = append(curTop.nested.items, curNested)
				continue
			}
			if m := unorderedMarker.FindStringSubmatch(trimmed); m != nil {
				if curTop.nested == nil || curTop.nested.ordered {
					curTop.nested = &listNode{ordered: false}
					curTop.addBlock(itemBlock{kind: blockList, list: curTop.nested})
				}
				curNested = newItem(m[1])
				curNestedCol = contentCol(indent, trimmed, m[1])
				curTop.nested.items = append(curTop.nested.items, curNested)
				continue
			}
			// Continuation line: fold into whichever item is
			// currently deepest-open, joined with a single space
			// (soft line break), matching the "wrapped bullet"
			// fix this package exists to make. If a fence or a
			// nested list intervened, addText starts a new prose
			// run after it rather than folding backwards in front
			// of it.
			if curNested != nil {
				curNested.addText(trimmed)
			} else {
				curTop.addText(trimmed)
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

// writeList renders a listNode as a real <ul>/<ol>. An item is its blocks in
// document order: prose runs (through renderInline), fenced code blocks
// (already rendered and escaped by preBlock — never re-escaped, never
// re-parsed) and nested lists. List nesting itself is still capped at one
// level: a nested item never opens a list of its own, because renderBlocks
// only ever attaches a nested list to the current TOP-level item. (P5
// replaces that cap with a depth stack; the recursion here is already
// shape-agnostic.)
func writeList(b *strings.Builder, l *listNode) {
	tag := "ul"
	if l.ordered {
		tag = "ol"
	}
	b.WriteString("<" + tag + ">")
	for _, it := range l.items {
		b.WriteString("<li>")
		for _, blk := range it.blocks {
			switch blk.kind {
			case blockText:
				// The one place a prose run's segments are joined: a
				// single space per soft line break, exactly as addText's
				// old incremental concatenation produced.
				b.WriteString(renderInline(strings.Join(blk.segs, " ")))
			case blockHTML:
				b.WriteString(blk.html)
			case blockList:
				writeList(b, blk.list)
			}
		}
		b.WriteString("</li>")
	}
	b.WriteString("</" + tag + ">")
}

// RenderInline is the exported entry point for the inline pass — backslash
// escapes, backtick-run code spans and [text](url) links, with everything
// else HTML-escaped. It is used by render/components' table-cell helper so a
// <td> renders the same inline markdown subset a card/list/steps body does,
// minus the block-level paragraph/list/fence handling Render layers on top (a
// table cell wants inline content, not a <p>-wrapped block).
//
// Render and RenderInline stay in parity by construction: both run the SAME
// renderInline, so a construct behaves identically in a paragraph, a list
// item and a table cell, and Render adds only the block wrapper.
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

// isEscapable reports whether c is in the CLOSED backslash-escapable set
// frozen at gate 1:
//
//	\ ` * _ ~ [ ] ( ) > # - . ! |
//
// The set is closed on purpose. < and & are deliberately OUTSIDE it, so "\<"
// is a literal backslash followed by "&lt;" — an escape can never be a route
// to an unescaped angle bracket or ampersand.
func isEscapable(c byte) bool {
	switch c {
	case '\\', '`', '*', '_', '~', '[', ']', '(', ')', '>', '#', '-', '.', '!', '|':
		return true
	}
	return false
}

// renderInline runs the inline pass over one paragraph's or list item's raw
// text in a single left-to-right scan recognizing three inline constructs —
// backslash escapes, backtick code spans and [text](url) links — and
// HTML-escaping everything else. It is never called on fence content.
//
// THE ESCAPING BOUNDARY. renderInline's output is trusted as template.HTML
// (see RenderInline), which is safe for exactly two structural reasons, and
// every construct added here must keep both true:
//
//   - every emitted tag and attribute delimiter is a fixed literal in this
//     file; and
//   - every author byte reaches the output through html.EscapeString.
//
// BACKSLASH ESCAPES (amendment A7) are resolved HERE, inside this scan —
// never as a pre-pass over the segment. On "\" the scanner inspects the next
// byte: if it is in the escapable set that byte is written straight to the
// output builder through html.EscapeString and the scan resumes two bytes
// on; otherwise the backslash itself is written through html.EscapeString
// and the scan resumes one byte on. A trailing "\" at end of text is a
// literal backslash.
//
// "Escapes never re-enter the parser" is therefore structural, not a
// convention: the escaped byte is appended to b, and the scanner only ever
// reads from text at an index already advanced past it. There is no buffer
// an escape result could be re-scanned from, so an escaped byte can never
// open, close or delimit any construct — present or future.
//
// Escapes do NOT process inside a code span: a span's content is sliced
// verbatim out of text and escaped once, so "`a\b`" keeps its backslash
// instead of silently losing a character. They likewise do not process
// inside a fence, which never reaches this function at all. Inside a link's
// text or url they are also not resolved — parseLink's grammar takes both
// verbatim to the first "]" / ")" (a documented v0.3.1 ceiling, unchanged by
// P1).
//
// At block level the escape is what stops a marker from being a marker: the
// block scanner matches "-", "*", "N." and a ``` run against the RAW line,
// so a leading backslash means the line no longer starts with the marker and
// the backslash is then consumed here. Same mechanism, no special case.
//
// CODE SPANS match by run length: an opening run of N backticks is closed by
// the next run of EXACTLY N backticks, so a literal backtick can appear
// inside inline code — a double-backtick span holding a-backtick-b renders
// as <code>a`b</code>. If no run of exactly N follows, the opening run alone
// is emitted as escaped literal text and the scan resumes immediately after
// it — the remainder of the text is still parsed, rather than being
// abandoned as literal wholesale, which is what the pre-P1 single-backtick
// pairing did.
//
// LINKS are fall-through-safe as before: a "[" that resolves to a complete
// [text](url) (see parseLink) is emitted as <a href="url">text</a> only when
// allowedScheme accepts the url; a rejected scheme
// (javascript:/data:/vbscript:/anything not http|https|mailto|relative)
// renders the whole match as escaped literal text with no anchor, and a "["
// that never completes a link is a literal character with the scan resuming
// just after it.
//
// The three constructs compose by position: whichever opener the scan meets
// first wins. A link written inside a code span stays literal because the
// span opened first and consumed it verbatim; an escaped opener is consumed
// as a plain byte before it can open anything.
func renderInline(text string) string {
	var b strings.Builder
	// plain is the start of the pending run of ordinary bytes; it is
	// flushed through html.EscapeString before every construct and once at
	// the end, so no author byte can reach b by any other route.
	plain := 0
	// spans is nil until a code-span search fails; from then on it answers
	// the remaining searches (see backtickIndex). Purely a cost structure —
	// it returns what findBacktickRun would have returned.
	var spans *backtickIndex
	// links is the same idea for link delimiters: nil until a parseLink
	// attempt fails, and from then on it answers the remaining attempts (see
	// linkIndex). Also purely a cost structure — it returns what parseLink
	// would have returned, byte for byte, accept for accept.
	var links *linkIndex
	flushPlain := func(end int) {
		if end > plain {
			b.WriteString(html.EscapeString(text[plain:end]))
		}
	}

	i := 0
	for i < len(text) {
		c := text[i]
		if c != '\\' && c != '`' && c != '[' {
			i++
			continue
		}
		flushPlain(i)

		switch c {
		case '\\':
			if i+1 < len(text) && isEscapable(text[i+1]) {
				b.WriteString(html.EscapeString(text[i+1 : i+2]))
				i += 2
			} else {
				// Not an escape (or a dangling backslash at end of
				// text): the backslash is a literal character and the
				// scan resumes at the next byte.
				b.WriteString(html.EscapeString(text[i : i+1]))
				i++
			}

		case '`':
			runLen := backtickRun(text, i)
			contentStart := i + runLen
			var closeStart int
			var ok bool
			if spans != nil {
				closeStart, ok = spans.find(contentStart, runLen)
			} else if closeStart, ok = findBacktickRun(text, contentStart, runLen); !ok {
				// This search just walked every remaining run without
				// consuming any of them. Index them once so no later
				// opener repeats the walk (see backtickIndex).
				spans = newBacktickIndex(text, contentStart)
			}
			if !ok {
				// Unclosed run: the run itself is literal, and the scan
				// resumes after it so later constructs still match.
				b.WriteString(html.EscapeString(text[i:contentStart]))
				i = contentStart
				break
			}
			b.WriteString("<code>")
			b.WriteString(html.EscapeString(text[contentStart:closeStart]))
			b.WriteString("</code>")
			i = closeStart + runLen

		case '[':
			var matchLen int
			var linkText, url string
			var ok bool
			if links != nil {
				matchLen, linkText, url, ok = links.parseLinkAt(text, i)
			} else if matchLen, linkText, url, ok = parseLink(text[i:]); !ok {
				// This attempt failed, which means it consumed nothing:
				// the scan will advance one byte and the next "[" would
				// repeat both of parseLink's searches over the same
				// remainder. Index the delimiters once so it doesn't
				// (see linkIndex).
				//
				// Indexing on ANY failure, not only an expensive one, is
				// deliberate. "Was that scan expensive?" has no cheap
				// honest answer: a failure can walk the whole remainder
				// and still return a valid index rather than -1 — N
				// copies of "[" followed by a single "]x" is quadratic
				// with every IndexByte succeeding. The price of the
				// simple rule is one extra linear pass over a body whose
				// brackets fail cheaply; the price of a clever one is a
				// threshold an attacker gets to sit just underneath.
				links = newLinkIndex(text, i)
			}
			if !ok {
				// Not a complete link: "[" is a literal char; resume the
				// scan just after it so any later construct still matches.
				b.WriteString(html.EscapeString(text[i : i+1]))
				i++
				break
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
				b.WriteString(html.EscapeString(text[i : i+matchLen]))
			}
			i += matchLen
		}
		plain = i
	}
	flushPlain(len(text))
	return b.String()
}

// findBacktickRun returns the start index of the first run of EXACTLY n
// backticks in s at or after from. Runs of a different length are skipped
// whole, which is what makes a double-backtick span a single span with a literal
// backtick rather than two adjacent spans.
func findBacktickRun(s string, from, n int) (int, bool) {
	for i := from; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		run := backtickRun(s, i)
		if run == n {
			return i, true
		}
		i += run
	}
	return 0, false
}

// backtickIndex records, per run length, the ascending start offsets of every
// backtick run in one inline text from the offset it was built at onwards. It
// answers exactly what findBacktickRun answers, by lookup instead of by
// walking the text.
//
// It exists only for cost. A findBacktickRun search that SUCCEEDS is already
// amortized linear, because renderInline then consumes everything the search
// walked. A search that FAILS consumes nothing, so a paragraph whose runs are
// all of different lengths — runs of 1, then 2, then 3 backticks, none ever
// closing another — made every span opener re-walk the whole remainder:
// superlinear, ~43ms for 700 runs in one paragraph, and
// reachable from the same reviewer-authored 1 MiB comment body as the fence
// scan. Output was always correct; only cost exploded.
//
// The repair is to index at the FIRST failure and not before. That first
// failed walk has already visited every remaining run, so recording them
// costs one more pass over ground already covered, and every later opener
// gets a binary search instead of another walk. Text whose code spans all
// close — the overwhelmingly common case — never builds an index at all, so
// the ordinary path keeps its current cost and allocation profile exactly.
type backtickIndex struct {
	byLen map[int][]int
}

// newBacktickIndex records every backtick run in s at or after from. Runs are
// measured the way renderInline's scan measures them — whole runs, taken left
// to right — and from is always a run BOUNDARY (every construct in the scan
// advances to the end of the run it consumed), so an indexed position is
// always a position findBacktickRun would itself have considered.
func newBacktickIndex(s string, from int) *backtickIndex {
	x := &backtickIndex{byLen: make(map[int][]int)}
	for i := from; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		run := backtickRun(s, i)
		x.byLen[run] = append(x.byLen[run], i)
		i += run
	}
	return x
}

// find returns the start index of the first run of EXACTLY n backticks at or
// after from — the same answer findBacktickRun computes for the same
// arguments, and the reason this is a drop-in replacement for it.
func (x *backtickIndex) find(from, n int) (int, bool) {
	pos := x.byLen[n]
	k := sort.Search(len(pos), func(i int) bool { return pos[i] >= from })
	if k == len(pos) {
		return 0, false
	}
	return pos[k], true
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

// linkIndex records the ascending offsets of every "]" and every ")" in one
// inline text from the offset it was built at onwards. It answers exactly what
// parseLink answers, by lookup instead of by re-scanning the text.
//
// It exists only for cost, and it is the SAME repair backtickIndex is, applied
// to the other rescanning search in this scan. parseLink's two searches —
// strings.IndexByte for "]" and then for ")" — each run over the whole
// remaining text, and a FAILED parseLink consumes nothing: the caller advances
// one byte and the next "[" repeats both searches over the same remainder. A
// text of N brackets that never complete a link therefore cost O(N^2):
// measured at ~4x per doubling, and seconds of CPU on a 1 MiB
// reviewer-authored comment body for inputs as trivial as N copies of "[",
// "[](" or "[a](". A parseLink that SUCCEEDS was never the problem — the
// caller advances past the whole match, so a successful search is amortized by
// the bytes it consumes.
//
// This changes no rendered output and, critically, no accept/reject decision:
// parseLinkAt computes the same matchLen, the same verbatim linkText and the
// same verbatim url as parseLink for the same position, so the url handed to
// allowedScheme/schemeOf — the security boundary — is byte-identical. The
// v0.3.1 link grammar ceiling is likewise untouched: still the FIRST "]" and
// the FIRST ")", still no nested brackets, still no escaped delimiters inside
// a link. Only the cost of finding those delimiters changes.
//
// As with backtickIndex, the index is built at the FIRST failure and not
// before, so ordinary prose — whose links complete — never allocates one.
type linkIndex struct {
	closeBrackets []int
	closeParens   []int
}

// newLinkIndex records every "]" and ")" in s at or after from. from is always
// the offset of a "[" the scan has reached, and the scan only ever moves
// forward, so every position any later lookup can ask about is at or after it.
func newLinkIndex(s string, from int) *linkIndex {
	x := &linkIndex{}
	for i := from; i < len(s); i++ {
		switch s[i] {
		case ']':
			x.closeBrackets = append(x.closeBrackets, i)
		case ')':
			x.closeParens = append(x.closeParens, i)
		}
	}
	return x
}

// nextAt returns the first offset in the ascending slice pos that is at or
// after from — the answer strings.IndexByte would give, as an absolute offset.
func nextAt(pos []int, from int) (int, bool) {
	k := sort.Search(len(pos), func(i int) bool { return pos[i] >= from })
	if k == len(pos) {
		return 0, false
	}
	return pos[k], true
}

// parseLinkAt is parseLink(s[at:]) computed by lookup: same match length, same
// verbatim link text, same verbatim url, same ok. s[at] must be '['. It is a
// drop-in for parseLink and is held to that by differential test.
func (x *linkIndex) parseLinkAt(s string, at int) (matchLen int, linkText, url string, ok bool) {
	// s[at] == '['. Absolute offsets throughout; parseLink's are relative to
	// rest := s[at+1:], which is the only difference between the two.
	closeBr, found := nextAt(x.closeBrackets, at+1)
	if !found || closeBr+1 >= len(s) || s[closeBr+1] != '(' {
		return 0, "", "", false
	}
	linkText = s[at+1 : closeBr]
	urlStart := closeBr + 2 // skip "]("
	closeParen, found := nextAt(x.closeParens, urlStart)
	if !found {
		return 0, "", "", false
	}
	url = s[urlStart:closeParen]
	// "[" through ")" inclusive.
	matchLen = closeParen + 1 - at
	return matchLen, linkText, url, true
}

// allowedScheme reports whether url may be emitted as an anchor's href. Only
// http, https, mailto, and scheme-less relative-path / #fragment urls are
// permitted; every other scheme — notably javascript, data, and vbscript — is
// rejected, AS IS a scheme-less protocol-relative "//host" network-path (see
// isNetworkPath), so a rejected link renders as inert escaped text instead of
// an active off-origin anchor. This is the allowlist half of renderInline's
// escaping boundary and is deliberately evasion-resistant (see schemeOf).
func allowedScheme(url string) bool {
	scheme, ok := schemeOf(url)
	if !ok {
		// No scheme: a relative path or #fragment is allowed — but NOT a
		// protocol-relative "//host" network-path, which carries no scheme yet
		// resolves against the page's own scheme to an arbitrary off-origin
		// host, outside the documented scheme-less (relative-path / #fragment)
		// scope.
		return !isNetworkPath(url)
	}
	switch scheme {
	case "http", "https", "mailto":
		return true
	default:
		return false
	}
}

// isNetworkPath reports whether url is a protocol-relative (RFC 3986
// "network-path", "//host...") reference: scheme-less, yet NOT a same-origin
// relative reference — it points at an arbitrary host under the page's own
// scheme, so it must never be emitted as a live anchor. Control bytes and
// spaces are stripped first (the same evasion resistance schemeOf applies before
// reading a scheme), and a backslash counts as a slash because browsers
// normalize "\" to "/" in the authority position of a URL under a special
// (http/https) scheme — so "/\host", "\\host", and "\/host" are just as
// off-origin as "//host".
func isNetworkPath(url string) bool {
	var b strings.Builder
	b.Grow(len(url))
	for i := 0; i < len(url); i++ {
		if c := url[i]; c > 0x20 && c != 0x7f {
			b.WriteByte(c)
		}
	}
	stripped := b.String()
	return len(stripped) >= 2 && isSlashByte(stripped[0]) && isSlashByte(stripped[1])
}

func isSlashByte(c byte) bool { return c == '/' || c == '\\' }

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
