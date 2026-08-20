// Package markdown is the engine's shared body renderer. It replaces the
// three ad-hoc, inconsistent string transforms that used to live inline in
// render/components/components.go (renderBody's regex-only fence matcher,
// bodyLines' naive newline split, and steps.html's raw unescaped dump) with
// three exported entry points over the same construct set, differing only in
// which surface they serve and, for two of them, whether images render:
//
//   - Render(body string) template.HTML — the block ceiling with images
//     OFF. Used by every surface that must never render an image: comment
//     bodies (root and every reply), on every render path that reaches one.
//   - RenderClaimBody(body string, assets AssetPrefix, cites Citations)
//     template.HTML — the same block ceiling with images ON and with "[n]"
//     citation markers resolving against the claim's own sources. This is what
//     every claim component (card, table, steps, banner, list, mockup)
//     actually binds for a claim's own body and steps entries, via the
//     "claimMarkdown" template func in render/components; see
//     markdown_images.go for the whole argument for why the capabilities are a
//     separate entry point rather than parameters on Render, and
//     markdown_cite.go for the citation rules internal/lint mirrors.
//   - RenderInline(text string) template.HTML — the narrower inline-only
//     ceiling, images always OFF, used by a layout:table claim's own rows
//     cells.
//
// A GFM pipe-table cell embedded inside a body rendered by RenderClaimBody
// is a fourth case worth naming here because it is easy to get wrong by
// analogy with RenderInline: it goes through the same inline-only renderer,
// but it inherits the CLAIM-BODY capabilities of the body around it — images
// and citation markers both — rather than always having them off. See
// markdown_tables.go's writeTable/writeTableRow. That is the right way round:
// a cell is part of the claim's own prose, so a "[1]" in one cites the same
// source a "[1]" in the paragraph above it does.
//
// This is deliberately not a general Markdown parser. It implements exactly
// the subset real claim corpora (and FORMAT.md's generic "body: markdown
// string" contract) actually need:
//
//   - Paragraphs — a run of non-blank lines separated by a blank line.
//   - Fenced code blocks — a ```-delimited block, recognized by the line
//     scanner in document order (see renderBlocks and fenceOpener), becoming a
//     real <pre><code>escaped-content</code></pre>. The opening line's info
//     string contributes class="language-x" when its first word is a plain
//     identifier and NOTHING when it is not (see infoLanguage). A fence
//     indented under an open list item renders INSIDE that item — including
//     across a blank line, which is how nearly all real documentation writes
//     one.
//   - Backslash escapes — a closed escapable set, resolved inside the inline
//     scan (see isEscapable and renderInline).
//   - Inline code spans — a backtick run matched against a closing run of
//     equal length, so a literal backtick can appear inside inline code.
//   - Inline links — [text](url) becomes <a href="url">text</a>. The url is
//     held to a scheme allowlist: only http, https, mailto, and scheme-less
//     relative-path or #fragment hrefs are emitted as anchors. A javascript:,
//     data:, vbscript: (or any other) scheme — AND a scheme-less
//     protocol-relative "//host" network-path, which resolves off-origin — is
//     neutralized to escaped literal text with no anchor. The url is
//     HTML-escaped in attribute context; the link TEXT runs the full inline
//     pass, so "[**text**](url)" composes. See renderInline and allowedScheme
//     for the parsing/allowlist boundary.
//   - Emphasis — **bold** becomes <strong>, *italic* and _italic_ become <em>,
//     and ~~strike~~ becomes <del>, on strict CommonMark left/right-flanking
//     delimiter rules (see markdown_inline.go's flanking rule, which is where
//     the whole "_" argument lives: an intraword underscore can neither open
//     nor close, so no snake_case corpus token can ever italicise — and where
//     the THREE classes of "_" run that rule does still let through are stated
//     in full, including the punctuation-flanked one that can both open and
//     close). Strikethrough is exactly two tildes; one or three or more is
//     literal.
//   - Autolinks — an angle-bracket <scheme:...> and a bare http/https URL both
//     become <a>, both through allowedScheme unchanged. "<" is a construct
//     opener for the angle form and for nothing else: anything that is not a
//     complete autolink with an accepted scheme is escaped, which is what keeps
//     raw inline HTML a non-goal. No bare-email autolinking.
//   - Unordered ("-"/"*") and ordered ("1.", "2.", ...) lists, nested to
//     UNBOUNDED depth via an indent-keyed depth stack (see renderBlocks' list
//     indentation rule), with GFM task items ("- [ ]" / "- [x]") rendering a
//     disabled checkbox, CommonMark looseness (a blank line inside a list
//     makes the whole list loose and every item's prose is <p>-wrapped) and
//     an <ol start="n"> whenever an ordered list's first item is not 1.
//   - Thematic breaks — a line of three or more dashes and nothing else is
//     ALWAYS an <hr>. There is no setext heading in this subset, so "text"
//     followed by "---" is a paragraph and then a rule, never an <h2>.
//   - ATX headings at levels 3 to 6 only. "#" and "##" are reserved for the
//     viewer's own chrome and render as literal text (markdown-sanity reports
//     them), as does a run of seven or more.
//   - GFM pipe tables — a header row, a REQUIRED delimiter row that sets each
//     column's alignment, and zero or more body rows, becoming a real
//     <table class="md-table"> whose cells carry a fixed-literal alignment
//     class. Outer pipes are optional, a short row renders SHORT and a long one
//     is truncated, and "\|" is a literal pipe rather than a cell boundary. Row
//     splitting happens BEFORE inline parsing, so a code span does not protect
//     a pipe — see markdown_tables.go, which carries the whole grammar and the
//     placement rule. A WELL-FORMED TABLE IS ALWAYS A TABLE: there is no size,
//     shape or alignment at which one degrades to prose, and the only thing
//     that renders as prose is a candidate with no valid delimiter row. A row
//     emits exactly the cells it has, which is what keeps a table's output
//     linear in its input without any bound having to be enforced.
//   - Blockquotes — one level deep. The "> " prefix is stripped and the
//     interior recurses into the same block scanner with blockquote
//     recognition OFF, so lists, headings, rules, task items and fenced code
//     inside a quote come free while a second ">" stays literal text.
//   - Hard line breaks — a trailing backslash or two trailing spaces becomes
//     a <br>. Both spellings are captured before the line is trimmed and
//     carried as an offset into the joined block text, so the inline pass
//     still runs ONCE per paragraph and the emphasis/strikethrough construct
//     above spans a break for free (see joinSegments).
//   - Images — "![alt](src)" — are the one construct in this list NOT
//     available through every entry point: see this file's own doc comment
//     above and markdown_images.go's file doc for the whole argument. Where
//     permitted, src is accepted only if, after entity-decoding, it names a
//     relative path under the rendering claim's own "assets/" directory
//     drawn from a closed character set and ending in one of six extensions
//     (.png .jpg .jpeg .gif .webp .svg); anything else — including every
//     scheme, every authority prefix (both slash spellings), every ".."
//     segment — renders the whole "![alt](src)" as escaped literal text
//     rather than as a broken <img>. alt is raw author bytes, HTML-escaped
//     and never itself run through the inline pass (see imgHTML).
//
// THE CONTAINER MATRIX (gate 0, amendment A2) is implemented exactly as
// stated and no cell is filled by implementation choice:
//
//	blockquote interior : paragraphs, lists, task items, headings, thematic
//	                      breaks, fenced code and pipe tables are all legal;
//	                      a nested quote is literal text.
//	list-item interior  : nested lists, task items and fenced code are legal;
//	                      every other block construct is literal text (a
//	                      heading, rule, quote marker or pipe-table row
//	                      indented under an item is item prose, not a block).
//	table cell          : inline only — renderInline, no block construct at
//	                      all, and no <br>.
//
// Explicitly out of scope: multi-level blockquotes, setext headings,
// indented (non-fenced) code blocks, reference-style links, footnotes, and
// raw inline HTML. Images are IN scope — see this list's own bullet above —
// but only through the two entry points that carry the capability.
// FORMAT.md's body field ("markdown string") makes no promise
// beyond "markdown", so trimming to this subset does not contradict the
// format spec's contract — it is a documented ceiling, not a silent gap.
//
// Pure stdlib (strings/html/html/template) — no new dependency.
package markdown

import (
	"html"
	"html/template"
	"regexp"
	"sort"
	"strconv"
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
// IMAGES ARE NOT PART OF THIS ENTRY POINT AND THAT IS DELIBERATE. Render's
// meaning is "no images": "![alt](src)" falls through as escaped literal text,
// so the shared component funcMap, comments.html and internal/serve's
// commentToDTO — the three comment-bearing call sites — are correct without
// being edited. markdown_images.go's RenderClaimBody is the opt-in, and its doc
// comment carries the whole argument for which way round the default sits.
func Render(body string) template.HTML {
	var b strings.Builder
	renderBlocks(&b, strings.Split(body, "\n"), true, bodyPolicy{})
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
// links, no emphasis and no autolinking of a URL that happens to sit inside
// one. The info string is not content: its first word becomes the code
// element's language class when it is a plain identifier (see infoLanguage),
// and the rest of it is dropped exactly as the whole of it used to be.

// fenceOpener reports whether line opens a fenced code block, and if so its
// leading-whitespace width, the length of its backtick run, and its trimmed
// info string (see infoLanguage, which turns that into a language class).
func fenceOpener(line string) (indent, runLen int, info string, ok bool) {
	indent = leadingSpaces(line)
	runLen = backtickRun(line, indent)
	if runLen < 3 {
		return 0, 0, "", false
	}
	// CommonMark: a backtick fence's info string may not contain a
	// backtick. Without this, "```x``` and more" would open a block fence
	// on a line that is plainly an inline span.
	if strings.IndexByte(line[indent+runLen:], '`') != -1 {
		return 0, 0, "", false
	}
	return indent, runLen, strings.TrimSpace(line[indent+runLen:]), true
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
//
// info is the opening line's info string. Its first word becomes
// class="language-x" when it is a plain identifier, and NOTHING when it is not
// (see infoLanguage): the class value is author bytes in an attribute, so the
// rule there is rejection first and escaping second, not escaping alone. A
// fence with no info string, or with one that is refused, emits exactly the
// bytes it emitted before this phase.
func preBlock(content, info string) string {
	open := "<pre><code>"
	if lang := infoLanguage(info); lang != "" {
		open = `<pre><code class="language-` + html.EscapeString(lang) + `">`
	}
	return open + html.EscapeString(content) + "</code></pre>"
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

// --- thematic breaks ------------------------------------------------------

// thematicBreak reports whether a line's TRIMMED content is a horizontal rule.
//
// ONE SPELLING ONLY: three or more dashes and nothing else. "***" and "___"
// are not rules here — they stay literal text (and are left free for the
// emphasis delimiters this package's inline pass may grow), so there is
// exactly one way to write a rule and exactly one thing "---" can mean.
//
// That single meaning is the point. This subset has NO setext heading, so a
// "---" line under a paragraph is unconditionally a rule and never retitles
// the paragraph above it as an <h2> — which is what makes "#" and "##" being
// reserved for viewer chrome (see atxHeading) actually hold: there is no back
// door that produces one.
//
// A rule is recognized at indent column 0 only. Indented under an open list
// item it is item prose, per the container matrix.
func thematicBreak(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] != '-' {
			return false
		}
	}
	return true
}

// --- ATX headings ---------------------------------------------------------

// atxHeading recognizes "### text" through "###### text" — levels 3 to 6 and
// nothing else — returning the level and the heading's inline text.
//
// LEVELS 1 AND 2 ARE RESERVED for the viewer's own chrome, so "# x" and "## x"
// return ok=false and fall through to ordinary paragraph handling: the hashes
// stay visible as literal text rather than being silently swallowed, and
// markdown-sanity reports them as "reserved heading level, use ### or deeper".
// A run of seven or more hashes is likewise not a heading (CommonMark caps at
// six). This is deliberately a REJECTION, not a silent acceptance at some other
// level — an author who writes "# Title" must see that it did not become a
// heading.
//
// A heading is recognized at indent column 0 only (indented under an open list
// item it is item prose, per the container matrix), it interrupts an open
// paragraph without a blank line before it, and its text runs the full inline
// pass — code spans, escapes and links today, plus whatever else renderInline
// grows, since it is the same single scan.
func atxHeading(trimmed string) (level int, text string, ok bool) {
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	if n == 0 {
		return 0, "", false
	}
	// A hash run must be followed by a space (or be the whole line) to be a
	// heading marker at all: "###x" is a word, not a heading.
	if n < len(trimmed) && trimmed[n] != ' ' && trimmed[n] != '\t' {
		return 0, "", false
	}
	if n < 3 || n > 6 {
		return 0, "", false
	}
	return n, trimClosingHashes(strings.TrimSpace(trimmed[n:])), true
}

// trimClosingHashes removes a CommonMark ATX closing sequence: a trailing run
// of "#" that is either the entire remaining content or preceded by a space.
// "### foo ###" is "foo"; "### foo#" keeps its hash, because that run is part
// of the word.
func trimClosingHashes(s string) string {
	end := len(s)
	for end > 0 && s[end-1] == '#' {
		end--
	}
	if end == len(s) {
		return s
	}
	if end == 0 {
		return ""
	}
	if s[end-1] == ' ' || s[end-1] == '\t' {
		return strings.TrimRight(s[:end], " \t")
	}
	return s
}

// --- blockquotes ----------------------------------------------------------
//
// THE BLOCKQUOTE RULE, in full (amendment A26).
//
// A quote opens on a line whose first byte is ">" at indent column 0. Every
// following line whose first byte is ">" belongs to the same quote; the quote
// ends at the first line that does not start with one. There is NO lazy
// continuation — an unprefixed line after a quoted line is a new top-level
// paragraph, not more quote.
//
// The prefix stripped from each line is ">" plus ONE optional space, and
// nothing else: trailing whitespace is deliberately left alone, because it is
// what a two-space hard break is made of and stripping it here would silently
// delete a break inside a quote.
//
// The interior then recurses into renderBlocks WITH BLOCKQUOTE RECOGNITION
// OFF. That single flag is the whole one-level cap, and it is a structural
// guarantee rather than a depth counter: inside a quote no line can ever open
// another quote, so "> > x" is one quote containing the literal text "> x",
// and ">> x" — whose second ">" is not preceded by a space and so survives the
// one-space strip — is the same thing. Recursion into the same scanner is also
// what makes the container matrix's blockquote row true for free: paragraphs,
// lists, task items, headings, thematic breaks and fenced code all work inside
// a quote because they are the same code paths, and any block construct a
// later phase adds to renderBlocks works inside a quote on the day it lands.
//
// A line that is only ">" (or ">" plus spaces) strips to a blank line, which
// is a paragraph break INSIDE the quote, not a terminator.

// blockquotePrefix strips a leading ">" plus one optional space from line,
// reporting ok=false when line does not open/continue a quote.
func blockquotePrefix(line string) (rest string, ok bool) {
	if line == "" || line[0] != '>' {
		return "", false
	}
	rest = line[1:]
	if rest != "" && rest[0] == ' ' {
		rest = rest[1:]
	}
	return rest, true
}

// --- hard line breaks -----------------------------------------------------
//
// THE HARD-BREAK RULE, in full (gate 0's hard-break amendment).
//
// Two spellings, both captured from the RAW line before it is trimmed — which
// is the only place the two-space form still exists, since strings.TrimSpace
// is what a paragraph line meets next:
//
//   - two or more trailing spaces, and
//   - a single unescaped trailing backslash.
//
// ESCAPES RESOLVE FIRST, so the backslash form is an ODD-length run of
// trailing backslashes: "x\\" ends in an escaped backslash and is not a break,
// "x\" and "x\\\" are. This is the same 15-char escapable set the inline scan
// uses, decided at the same place, so the two can never disagree.
//
// A break marker on the LAST line of a paragraph, item or quote emits nothing
// — there is nothing to break to. It is markdown-sanity's dangling-backslash
// finding, and the backslash itself stays visible as literal text (the inline
// scan's dangling-backslash rule), rather than being silently eaten.
//
// THE INLINE UNIT IS STILL THE WHOLE BLOCK. A break is NOT rendered by running
// the inline pass per line and gluing <br> between the results — that would
// make every future emphasis/strikethrough delimiter unable to span a break.
// Instead joinSegments joins the block's lines exactly as before, with one
// space per line, and returns the byte offsets of the spaces that are hard
// breaks; renderInline emits <br> at those offsets instead of the space. The
// inline scan therefore still runs ONCE per block over one string, so any
// construct that spans a soft line break spans a hard one too.
//
// One construct deliberately does not: a code span may not span a hard break
// (renderInline caps its closing search at the next break offset), so an
// opening backtick run before a break whose only closer is after it stays
// literal.

// breakKind records how one source line spelled its hard line break.
type breakKind uint8

const (
	brkNone   breakKind = iota // no hard break: an ordinary soft line break
	brkSpaces                  // two or more trailing spaces
	brkSlash                   // a single unescaped trailing backslash
)

// segment is one source line's contribution to a paragraph or a list item's
// prose run: the line's trimmed text plus how the line ended. Blocks accumulate
// segments and join them once (see joinSegments); nothing concatenates
// incrementally, which is what keeps the accumulator linear (see addText).
type segment struct {
	text string
	brk  breakKind
}

// newSegment captures line's hard-break spelling BEFORE trimming it — the
// entire reason this function exists rather than a bare strings.TrimSpace at
// each call site.
func newSegment(line string) segment {
	return segment{text: strings.TrimSpace(line), brk: hardBreakOf(line)}
}

// hardBreakOf reports how line spells a hard break, or brkNone.
func hardBreakOf(line string) breakKind {
	if trailingRun(line, ' ') >= 2 {
		return brkSpaces
	}
	// Odd run of trailing backslashes: escapes resolve first, so "\\" is an
	// escaped literal backslash and only a leftover single "\" is a break.
	if trailingRun(line, '\\')%2 == 1 {
		return brkSlash
	}
	return brkNone
}

// trailingRun counts the run of c at the end of s.
func trailingRun(s string, c byte) int {
	n := 0
	for n < len(s) && s[len(s)-1-n] == c {
		n++
	}
	return n
}

// joinSegments joins a block's prose segments into the single string the inline
// pass runs over, and returns the ascending byte offsets of the hard breaks
// inside it.
//
// The join is byte-identical to the pre-P4 one for text with no hard break:
// one space per soft line break, which is exactly what strings.Join(segs, " ")
// produced. A HARD break is that same separator space, recorded in the returned
// offsets; renderInline writes <br> in its place. Carrying the break as an
// offset onto a real separator byte — rather than as an in-band sentinel
// character — is what keeps the escaping boundary intact (no byte is invented
// that an author could also have typed, and none is destroyed) and what stops
// tokens on either side of a break from merging into one: there is always a
// space between them, so a backtick run, a bracket or an emphasis delimiter can
// never straddle a break offset.
//
// A break on the final segment produces no offset: there is nothing to break
// to, so it emits nothing. Its backslash is left in the text, where the inline
// scan renders it as the literal dangling backslash it is.
func joinSegments(segs []segment) (joined string, breaks []int) {
	if len(segs) == 1 {
		return segs[0].text, nil
	}
	var sb strings.Builder
	for i, s := range segs {
		text := s.text
		last := i == len(segs)-1
		if s.brk == brkSlash && !last {
			// Consume the backslash that spelled the break; the two-space
			// spelling was already consumed by TrimSpace.
			text = text[:len(text)-1]
		}
		sb.WriteString(text)
		if last {
			break
		}
		if s.brk != brkNone {
			breaks = append(breaks, sb.Len())
		}
		sb.WriteByte(' ')
	}
	return sb.String(), breaks
}

// nextBreak returns the first break offset at or after from, or def when there
// is none. It is how renderInline bounds a code span to one line.
func nextBreak(breaks []int, from, def int) int {
	if len(breaks) == 0 {
		return def
	}
	k := sort.Search(len(breaks), func(i int) bool { return breaks[i] >= from })
	if k == len(breaks) {
		return def
	}
	return breaks[k]
}

// orderedMarker matches an "N. rest of line" list marker (e.g. "14. Context
// immutability: ..."); group 1 is the number, which becomes the list's start
// attribute when it is not 1, and group 2 is the item's own text with the
// marker stripped. It is matched against a line's trimmed content regardless
// of the line's actual indentation — indentation decides only WHICH open level
// the match belongs to (see renderBlocks' list indentation rule).
var orderedMarker = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)

// unorderedMarker matches a "- rest of line" or "* rest of line" list marker;
// group 1 is the item's own text with the marker stripped. Same
// indentation-independent matching note as orderedMarker.
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
// difference. It is the threshold every indent decision is made against: a
// marker at or past it opens a deeper level, and a continuation line at or
// past it belongs to that item.
func contentCol(indent int, trimmed, itemText string) int {
	return indent + len(trimmed) - len(itemText)
}

// taskMarker recognizes a GFM task-list checkbox at the head of an unordered
// item's content: "[ ]", "[x]" or "[X]" followed by whitespace (or by nothing,
// for an empty item). It returns the checkbox state and the item's remaining
// content.
//
// UNORDERED ITEMS ONLY. "1. [ ] x" is an ordinary numbered item whose text
// happens to start with a bracket — a numbered checklist is a shape nothing in
// the corpus writes, and inventing one would put an <input> inside an <ol>
// where the number and the box compete for the same gutter. That is a pinned
// ceiling, not an oversight.
//
// The marker is defused by the ordinary escape mechanism with no special case:
// "- \[x] done" no longer begins with "[", so it is not a task item, and the
// inline scan then resolves "\[" to a literal bracket.
func taskMarker(itemText string) (state taskState, content string, ok bool) {
	if len(itemText) < 3 || itemText[0] != '[' || itemText[2] != ']' {
		return taskNone, "", false
	}
	switch itemText[1] {
	case ' ':
		state = taskUnchecked
	case 'x', 'X':
		state = taskChecked
	default:
		return taskNone, "", false
	}
	rest := itemText[3:]
	if rest == "" {
		return state, "", true
	}
	if rest[0] != ' ' && rest[0] != '\t' {
		return taskNone, "", false
	}
	return state, strings.TrimLeft(rest, " \t"), true
}

// taskState is a list item's checkbox: absent, unchecked or checked. The
// rendered checkbox is always DISABLED — the viewer is a reading surface, and
// a live checkbox would offer a state change that has nowhere to be stored.
type taskState uint8

const (
	taskNone taskState = iota
	taskUnchecked
	taskChecked
)

// listNode is one unordered or ordered list block: a flat sequence of items,
// each of which may itself contain further lists to unbounded depth (see the
// depth stack in renderBlocks).
//
// start is the number the first item was written with. It is emitted as
// <ol start="n"> whenever it is not 1, which is the fix for the release's
// longest-standing internal bug: orderedMarker has always captured the number
// and writeList has always thrown it away, so EVERY ordered list restarted at
// 1 — including the second half of one that a top-level fence split in two.
//
// loose is CommonMark looseness, and it is a property of the LIST, not of an
// item: if a blank line falls inside the list and the list continues past it,
// every item in that list wraps each of its prose runs in <p>. Marking the
// list rather than the item is what stops a spaced list from rendering half its
// items with a paragraph and half without.
type listNode struct {
	ordered bool
	start   int
	loose   bool
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
// textIdx is a cursor onto whichever prose run in blocks is currently open, so
// continuation lines know where to fold; it is invalidated (textIdx = -1)
// whenever a different block type opens. There is no longer a nested-list
// cursor: renderBlocks' depth stack holds the open list at every level, so an
// item does not need to remember one.
type itemNode struct {
	blocks  []itemBlock
	textIdx int
	task    taskState
}

func newItem(seg segment) *itemNode {
	it := &itemNode{textIdx: -1}
	it.addText(seg)
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
func (it *itemNode) addText(s segment) {
	if it.textIdx >= 0 {
		blk := &it.blocks[it.textIdx]
		blk.segs = append(blk.segs, s)
		return
	}
	it.blocks = append(it.blocks, itemBlock{kind: blockText, segs: []segment{s}})
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
// line folded in by addText — not the joined string. They are joined exactly
// once, at render time in writeList (see joinSegments); see addText for why the
// join cannot happen incrementally.
type itemBlock struct {
	kind itemBlockKind
	segs []segment
	html string
	list *listNode
}

// renderBlocks scans one container's lines — a whole body, or the interior of
// a blockquote — and writes paragraphs, thematic breaks, headings,
// blockquotes, fenced code blocks and lists to b. Every non-blank line is
// either a fence opener (see the fence rule above), a blockquote line, a
// thematic break, an ATX heading, a list marker, a continuation of an open
// list item, or plain paragraph prose.
//
// allowQuote is the one-level blockquote cap: Render passes true, and the
// recursion a blockquote makes into this same function passes false, so no
// line inside a quote can open another (see the blockquote rule above). It is
// the ONLY container flag — everything else about a container's interior is
// decided by the container matrix, which this function implements by WHERE it
// tests each construct, not by a per-construct switch:
//
//   - thematic break, ATX heading, blockquote and pipe table are tested only
//     at indent column 0, so under an open list item they are ordinary item
//     prose. That is the matrix's list-item row ("other block constructs
//     literal text") with no extra machinery, and it is why "  ### x" inside
//     an item renders as the literal text "### x" and why an indented
//     "  | a | b |" row is item prose rather than a table.
//   - a fence and a list marker are tested at any indent, which is the
//     matrix's "nested lists, task items and fenced code are legal inside an
//     item".
//   - a table cell never reaches this function at all: renderInline is the
//     table-cell entry point, so a cell is inline-only by construction.
//
// The pipe table is tested LAST of the column-zero constructs — after the list
// marker and continuation branches — so a line that already opens or continues
// a list item is never re-read as a header row. See markdown_tables.go.
//
// Block-switch rule: encountering a different block type (a marker line while
// a paragraph is accumulating, a plain paragraph line while a list is open, a
// heading/rule/quote at column 0, or a top-level fence) flushes whatever was
// open before starting the new one — this is what makes list boundaries and
// paragraph boundaries deterministic without a lookahead. The one construct
// that does NOT flush is an indented fence under an open list item: it is item
// content, so the list stays open across it.
//
// THE LIST INDENTATION RULE (amendment A24's restored dedent half). Open list
// levels live on a DEPTH STACK keyed by indent width, replacing the old
// one-level cap. Each level remembers two columns: its marker column (where
// its bullet sits) and its content column (where its text starts, past the
// marker — see contentCol). A line at indent width w is resolved against them:
//
//   - a MARKER at w >= the innermost open level's content column OPENS a new,
//     deeper level nested inside that level's item. That is the only thing
//     that opens a level, and it is why "- a" / "  - b" nests (2 >= 2) while
//     "- a" / " - b" does not (1 < 2).
//   - a MARKER at a smaller w CLOSES levels until w is no longer inside one —
//     it pops while w < the innermost marker column — and then joins the level
//     it lands on as a sibling item. A w that matches no level's marker column
//     exactly snaps DOWN to the nearest enclosing one rather than inventing a
//     level: an indent is always resolvable, and nothing is ever dropped.
//   - a CONTINUATION line (indented, no marker) pops while w < the innermost
//     CONTENT column and then folds into that level's item. So a line that
//     dedents out of a nested item continues its parent, and a line indented
//     past everything continues the innermost item.
//   - INDENT WIDTH IS COLUMNS, NOT BYTES (see indentWidth): a tab is four
//     columns. leadingSpaces — a byte count — is still what the fence
//     machinery uses to slice a line, and the two must not be confused.
//
// THE LOOSE-LIST RULE, in two halves.
//
// CONTINUATION (already shipped): a blank line always closes an open
// PARAGRAPH, but it does not by itself close an open LIST. It only records
// that a blank line was seen (pendingBlank); the next non-blank line decides,
// by its indent, whether the list was really over:
//
//   - indented to at least the OUTERMOST item's content column, the line is
//     list content and the list continues — the indentation rule above then
//     says which level it lands in. This is what makes the universal
//     documentation shape work: an item, a blank line, then an indented fence,
//     then a blank line, then the next item.
//   - a list marker at any indent: the list continues with another item,
//     exactly as it would have without the blank line.
//   - anything else — a non-indented, non-marker line, which includes a
//     COLUMN-ZERO FENCE, heading or rule — ends the list, as it did before.
//
// LOOSENESS (new, CommonMark): when a blank line is crossed that way, the list
// it was crossed IN becomes loose, and every item of that list then wraps each
// of its prose runs in <p> (see writeList). Which list is "the one it was
// crossed in" is exactly the list the following line lands in — except for a
// line that opens a DEEPER level, where the blank separated two blocks inside
// the PARENT item, so it is the parent's list that goes loose. A nested list
// with no blank line of its own stays tight inside a loose parent, which is
// the CommonMark shape and the reason looseness is tracked per list rather
// than per document.
// pol is the claim-body capability set, carried unchanged into every container
// this function opens — a blockquote interior, a list item, a table cell — and
// into the inline pass. ITS ZERO VALUE GRANTS NOTHING, so a block construct
// added later that forgets to pass it along refuses images and citation markers
// rather than granting them. See bodyPolicy.
func renderBlocks(b *strings.Builder, lines []string, allowQuote bool, pol bodyPolicy) {
	closers := closerRuns(lines)

	// stack is the open list levels, outermost first. stack[0].list is the
	// root list currently being accumulated; an empty stack means no list is
	// open.
	var stack []openLevel
	var paraSegs []segment
	// pendingBlank: a blank line was seen with a list open, and the next
	// non-blank line has yet to say whether that ended the list.
	pendingBlank := false

	flushParagraph := func() {
		if len(paraSegs) == 0 {
			return
		}
		text, breaks := joinSegments(paraSegs)
		b.WriteString("<p>")
		b.WriteString(renderInline(text, breaks, pol))
		b.WriteString("</p>")
		paraSegs = nil
	}
	flushList := func() {
		pendingBlank = false
		if len(stack) == 0 {
			return
		}
		writeList(b, stack[0].list, pol)
		stack = stack[:0]
	}
	// snapToContent pops levels whose content column a line at width w does
	// not reach, and returns the level the line therefore belongs to. It is
	// the shared resolution step for continuation lines and for an indented
	// fence, so both land in the same item for the same indent.
	snapToContent := func(w int) *openLevel {
		for len(stack) > 1 && w < stack[len(stack)-1].contentCol {
			stack = stack[:len(stack)-1]
		}
		return &stack[len(stack)-1]
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushParagraph()
			// A blank line ends a paragraph but only ARMS the end of a
			// list; see the loose-list rule above.
			if len(stack) > 0 {
				pendingBlank = true
			}
			continue
		}

		// byteIndent slices the line (the fence machinery's unit); w measures
		// it in columns (the list machinery's unit). They are never
		// interchangeable, but both are zero for exactly the same lines.
		byteIndent := leadingSpaces(line)
		w := indentWidth(line)

		// Resolve a pending blank line before anything else looks at the
		// open containers, so every branch below sees the list state this
		// line actually belongs to. looseArm carries the answer forward:
		// whichever branch consumes this line marks its list loose.
		looseArm := false
		if pendingBlank {
			pendingBlank = false
			switch {
			case w >= stack[0].contentCol, isListMarker(trimmed):
				looseArm = true
			default:
				flushList()
			}
		}

		// Fences are met in document order, so an open container survives
		// one. An unclosed fence is not a fence: it falls through to the
		// ordinary line handling below.
		if fIndent, runLen, info, ok := fenceOpener(line); ok {
			if content, closeIdx, closed := scanFence(lines, closers, i, fIndent, runLen); closed {
				if byteIndent > 0 && len(stack) > 0 {
					// Indented under an open item: the block is that
					// item's content, and the list stays open.
					lv := snapToContent(w)
					if looseArm {
						lv.list.loose = true
					}
					lv.item.addBlock(itemBlock{kind: blockHTML, html: preBlock(content, info)})
				} else {
					flushParagraph()
					flushList()
					b.WriteString(preBlock(content, info))
				}
				i = closeIdx
				continue
			}
		}

		// Column-zero-only constructs: the container matrix's list-item row
		// is enforced by this guard and nothing else.
		if w == 0 {
			if allowQuote && line != "" && line[0] == '>' {
				flushParagraph()
				flushList()
				end := i
				var inner []string
				for end < len(lines) {
					rest, ok := blockquotePrefix(lines[end])
					if !ok {
						break
					}
					inner = append(inner, rest)
					end++
				}
				b.WriteString("<blockquote>")
				renderBlocks(b, inner, false, pol)
				b.WriteString("</blockquote>")
				i = end - 1
				continue
			}
			if thematicBreak(trimmed) {
				flushParagraph()
				flushList()
				b.WriteString("<hr>")
				continue
			}
			if level, text, ok := atxHeading(trimmed); ok {
				flushParagraph()
				flushList()
				tag := "h" + strconv.Itoa(level)
				b.WriteString("<" + tag + ">")
				b.WriteString(renderInline(text, nil, pol))
				b.WriteString("</" + tag + ">")
				continue
			}
		}

		// List markers are matched against the line's TRIMMED content at any
		// indent; the indentation rule above then decides which level the
		// match belongs to. A marker with no list open and a non-zero indent
		// is NOT a list — it is indented prose, exactly as before, so a
		// stray indented bullet under a paragraph keeps folding into it.
		ordered, num, itemText, isMarker := listMarker(trimmed)
		if isMarker && (len(stack) > 0 || w == 0) {
			flushParagraph()
			cc := contentCol(w, trimmed, itemText)
			task := taskNone
			if !ordered {
				if st, content, ok := taskMarker(itemText); ok {
					task, itemText = st, content
				}
			}
			it := newItem(segment{text: itemText, brk: hardBreakOf(line)})
			it.task = task
			switch {
			case len(stack) == 0:
				lst := &listNode{ordered: ordered, start: num}
				lst.items = append(lst.items, it)
				stack = append(stack, openLevel{list: lst, item: it, markerCol: w, contentCol: cc})

			case w >= stack[len(stack)-1].contentCol:
				// Opens a level: the new list is a block of the item that
				// encloses it, in document order with that item's prose.
				parent := &stack[len(stack)-1]
				if looseArm {
					parent.list.loose = true
				}
				lst := &listNode{ordered: ordered, start: num}
				lst.items = append(lst.items, it)
				parent.item.addBlock(itemBlock{kind: blockList, list: lst})
				stack = append(stack, openLevel{list: lst, item: it, markerCol: w, contentCol: cc})

			default:
				for len(stack) > 1 && w < stack[len(stack)-1].markerCol {
					stack = stack[:len(stack)-1]
				}
				lv := &stack[len(stack)-1]
				if lv.list.ordered != ordered {
					// A flavour change ends this level's list and opens a
					// sibling of the other flavour in its place — a "-" item
					// can never join an <ol>.
					lst := &listNode{ordered: ordered, start: num}
					lst.items = append(lst.items, it)
					if len(stack) == 1 {
						writeList(b, lv.list, pol)
						stack = append(stack[:0], openLevel{list: lst, item: it, markerCol: w, contentCol: cc})
					} else {
						parent := &stack[len(stack)-2]
						parent.item.addBlock(itemBlock{kind: blockList, list: lst})
						*lv = openLevel{list: lst, item: it, markerCol: w, contentCol: cc}
					}
				} else {
					if looseArm {
						lv.list.loose = true
					}
					lv.list.items = append(lv.list.items, it)
					lv.item, lv.markerCol, lv.contentCol = it, w, cc
				}
			}
			continue
		}

		if len(stack) > 0 && w > 0 {
			// Continuation line: fold into whichever open item this indent
			// resolves to, joined with a single space (a soft line break) or
			// a <br> (a hard one), matching the "wrapped bullet" fix this
			// package exists to make. If a fence or a nested list intervened,
			// addText starts a new prose run after it rather than folding
			// backwards in front of it.
			lv := snapToContent(w)
			if looseArm {
				lv.list.loose = true
			}
			lv.item.addText(newSegment(line))
			continue
		}

		// A GFM pipe table, at indent column 0 only — the same guard hr,
		// headings and blockquotes use, and the whole of why a table is legal
		// inside a blockquote (via this function's own recursion) and is
		// literal item prose inside a list item. It is tested here, AFTER the
		// list-marker and continuation branches, so a line that already opens
		// or continues a list item is never re-read as a header row.
		//
		// A table interrupts an open paragraph and ends any open list, exactly
		// as the plain-paragraph branch below would have. See markdown_tables.go
		// for the grammar. There is no refusal branch here and there must not
		// be one: a candidate with a valid delimiter row IS a table, at every
		// size and every shape, and the only thing that renders as prose is a
		// candidate tableAt did not accept — which consumes just its own line.
		if w == 0 {
			if aligns, end, ok := tableAt(lines, i); ok {
				flushParagraph()
				flushList()
				writeTable(b, lines, i, end, aligns, pol)
				i = end
				continue
			}
		}

		// Plain paragraph line: any open list ends here (a non-indented,
		// non-marker line is never a list continuation), and an indented line
		// with no open list to fold into is ordinary prose rather than a
		// dropped line.
		flushList()
		paraSegs = append(paraSegs, newSegment(line))
	}

	flushParagraph()
	flushList()
}

// openLevel is one open list level on renderBlocks' depth stack: the list
// being accumulated at that depth, its currently open item, and the two
// columns every indent decision is made against (see the list indentation
// rule).
type openLevel struct {
	list       *listNode
	item       *itemNode
	markerCol  int
	contentCol int
}

// listMarker matches either flavour of list marker against a line's trimmed
// content, returning the ordered flag, the ordered marker's NUMBER (1 for an
// unordered item, and 1 for a number too large to be represented, which then
// simply emits no start attribute), the item's own text and whether anything
// matched.
func listMarker(trimmed string) (ordered bool, num int, itemText string, ok bool) {
	if m := orderedMarker.FindStringSubmatch(trimmed); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			n = 1
		}
		return true, n, m[2], true
	}
	if m := unorderedMarker.FindStringSubmatch(trimmed); m != nil {
		return false, 1, m[1], true
	}
	return false, 0, "", false
}

// writeList renders a listNode as a real <ul>/<ol>. An item is its blocks in
// document order: prose runs (through renderInline), fenced code blocks
// (already rendered and escaped by preBlock — never re-escaped, never
// re-parsed) and nested lists, recursively, to whatever depth renderBlocks'
// depth stack reached.
//
// THE EMITTED MARKUP, frozen here so the stylesheet can be written against it:
//
//	<ul> / <ol> / <ol start="n">   n only when the first item is not 1
//	<li>…</li>                     an ordinary item
//	<li class="task"><input type="checkbox" disabled[ checked]> …</li>
//	<li>…<p>…</p>…</li>            prose runs of a LOOSE list's items
//
// The checkbox is the first thing inside the <li>, before any <p>, so one CSS
// rule reaches it in a tight list and a loose one alike. Every attribute here
// is a fixed literal except start's number, which is re-emitted from a parsed
// int and so can only ever be digits — the escaping boundary is unchanged.
func writeList(b *strings.Builder, l *listNode, pol bodyPolicy) {
	tag := "ul"
	if l.ordered {
		tag = "ol"
	}
	if l.ordered && l.start != 1 {
		b.WriteString("<ol start=\"" + strconv.Itoa(l.start) + "\">")
	} else {
		b.WriteString("<" + tag + ">")
	}
	for _, it := range l.items {
		if it.task == taskNone {
			b.WriteString("<li>")
		} else {
			b.WriteString(`<li class="task"><input type="checkbox" disabled`)
			if it.task == taskChecked {
				b.WriteString(" checked")
			}
			b.WriteString("> ")
		}
		for _, blk := range it.blocks {
			switch blk.kind {
			case blockText:
				// The one place a prose run's segments are joined: a single
				// space per soft line break and a <br> per hard one, exactly
				// as addText's old incremental concatenation produced for the
				// soft case.
				text, breaks := joinSegments(blk.segs)
				inline := renderInline(text, breaks, pol)
				switch {
				case !l.loose:
					b.WriteString(inline)
				case inline != "":
					b.WriteString("<p>" + inline + "</p>")
				}
			case blockHTML:
				b.WriteString(blk.html)
			case blockList:
				writeList(b, blk.list, pol)
			}
		}
		b.WriteString("</li>")
	}
	b.WriteString("</" + tag + ">")
}

// RenderInline is the exported entry point for the inline pass — backslash
// escapes, backtick-run code spans, [text](url) links, emphasis, strikethrough
// and both autolink forms, with everything else HTML-escaped. It is used by
// render/components' table-cell helper so a
// <td> renders the same inline markdown subset a card/list/steps body does,
// minus the block-level paragraph/list/fence handling Render layers on top (a
// table cell wants inline content, not a <p>-wrapped block).
//
// TWO UNRELATED THINGS ARE CALLED A TABLE CELL and both land here. A
// "layout: table" claim's structured Rows[].cell is one, via components; a GFM
// pipe table's cell inside a markdown BODY is the other, via
// markdown_tables.go. They share this entry point precisely because they want
// the identical answer — inline markdown, no block layer — and they share
// nothing else: the claim layout keeps its own rows-shape lint and column
// ordering, and a pipe table has neither.
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
// A table cell is the one context with no block layer at all, so it also has
// no hard line break: a cell is single-line by construction, RenderInline is
// called with no break offsets, and a trailing backslash in a cell is the
// literal dangling backslash the inline scan already renders.
func RenderInline(text string) template.HTML {
	return template.HTML(renderInline(text, nil, bodyPolicy{}))
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

// renderInline runs the inline pass over one paragraph's, heading's, list
// item's or table cell's raw text. It is never called on fence content.
//
// The scan itself lives in markdown_inline.go, whose doc comment carries the
// full construct set and the precedence rule; this function is the block
// layer's entry point into it, and the one place the default context (not
// inside a link's text) is chosen.
//
// THE ESCAPING BOUNDARY, which every construct in that file keeps:
//
//   - every emitted tag and attribute delimiter is a fixed literal in this
//     package; and
//   - every author byte reaches the output through html.EscapeString.
//
// BACKSLASH ESCAPES (amendment A7) are resolved inside the scan — never as a
// pre-pass over the segment. On "\\" the scanner inspects the next byte: if it
// is in the escapable set that byte is written straight to the output and the
// scan resumes two bytes on; otherwise the backslash itself is an ordinary
// character and the scan resumes one byte on. A trailing "\\" at end of text is
// a literal backslash.
//
// "Escapes never re-enter the parser" is therefore structural, not a
// convention: the escaped byte is written to the output buffer, which is never
// read back, and the scanner only ever reads from text at an index already
// advanced past it. There is no buffer an escape result could be re-scanned
// from, so an escaped byte can never open, close or delimit any construct —
// including the emphasis and autolink constructs added at phase A.
//
// Escapes do NOT process inside a code span: a span's content is sliced
// verbatim out of text and escaped once, so "`a\b`" keeps its backslash instead
// of silently losing a character. They likewise do not process inside a fence,
// which never reaches this function at all, nor inside a bare URL or an angle
// autolink, both of which are consumed verbatim. Inside a link's URL they are
// also not resolved — parseLink's grammar takes it verbatim to the first ")"
// (a documented v0.3.1 ceiling). A link's TEXT, by contrast, now runs the full
// inline pass, so an escape inside it resolves exactly as it would anywhere
// else.
//
// TWO THINGS ABOUT LINK TEXT ARE STILL NOT IDENTICAL to top-level text, and
// both are deliberate. Neither autolink form fires inside it, because an anchor
// may not contain an anchor. And because the pass runs on a SUBSTRING, the
// recursive call is told which characters bracket that substring — the link's
// own "[" and "]" — so that a delimiter run at either end of the text flanks
// against those brackets, which is what the source actually has there and what
// CommonMark resolves against. Without that it would flank against a
// start-of-text that is not there. See inlineCtx.
//
// At block level the escape is what stops a marker from being a marker: the
// block scanner matches "-", "*", "N." and a ``` run against the RAW line, so a
// leading backslash means the line no longer starts with the marker and the
// backslash is then consumed here. Same mechanism, no special case.
//
// HARD LINE BREAKS reach the scan as BREAKS: the ascending byte offsets, in
// text, of the separator spaces the block layer decided were hard line breaks
// (see joinSegments). At such an offset the scan emits a fixed-literal <br> and
// skips the space. Carrying them out of band rather than as an in-band sentinel
// is what keeps the escaping boundary exact — there is no byte in text that the
// scan treats as anything other than an author byte, so no author input can
// forge a break — and it is why the inline unit is still the whole block:
// emphasis and strikethrough span a hard break because a break is just a byte
// in the same string.
//
// The single exception is a CODE SPAN, which may not span a hard break: the
// closing-run search is capped at the next break offset, so an opening run
// whose only closer is on the far side of a break is emitted as literal text.
// The cap is exact rather than approximate because a break offset is always a
// separator SPACE, so no backtick run can straddle one.
func renderInline(text string, breaks []int, pol bodyPolicy) string {
	return renderInlineCtx(text, breaks, inlineCtx{pol: pol})
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

// THE URL GATES LIVE IN internal/urlsafe AND NOT HERE. allowedScheme,
// isNetworkPath, schemeOf, stripCtrlAndSpace and isSlashByte used to be defined
// at this spot. They were correct — and they were also one of FOUR private
// copies of the same rule in this repository, the weakest of which (the mockup
// <img src> regexp in internal/lint/raw_html_scope.go) recognised "//host" but
// not "\\host", "/\host" or "\/host" and so let a reviewed mockup claim carry an
// off-origin image. Correct copies do not stop the drift; having one definition
// does.
//
// The renderer therefore calls urlsafe.IsAllowedHref for an anchor href,
// urlsafe.SchemeOf where the autolink scanner needs to know a scheme exists, and
// urlsafe.StripCtrlAndSpace / urlsafe.IsRelativePath in markdown_images.go.
// urlsafe imports nothing from this module, so both this package and
// internal/lint can depend on it without a cycle — which is the property that
// made a shared gate possible at all.
//
// isSchemeAlpha and isSchemeDigit stay here. They are not the scheme GATE; they
// are the autolink scanner's candidate-word character classes (see
// markdown_inline.go's autolinkWord, which deliberately also accepts "_" and is
// wider than the grammar), and the gate is applied to what they find, never
// derived from it.

func isSchemeAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isSchemeDigit(c byte) bool { return c >= '0' && c <= '9' }

// leadingSpaces counts the BYTES in a line's leading space/tab run. It is a
// slicing offset, not an indent measurement: fenceOpener, fenceCloses,
// closerRuns and stripIndent all index into the line with it, so a tab must
// count as the one byte it occupies.
//
// Use indentWidth, never this, for any question about how deeply a line is
// indented.
func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') {
		n++
	}
	return n
}

// indentWidth measures a line's leading whitespace in COLUMNS: a space is one
// column and A TAB ADVANCES TO THE NEXT MULTIPLE OF FOUR, the tab width this
// package pins.
//
// Pinning it is a fix, not a formality. Every list decision is made by
// comparing indents, and leadingSpaces — the only measure this package used to
// have — counts a tab as ONE, so a tab-indented sub-bullet under a "- " item
// measured 1 against a content column of 2 and silently failed to nest, while
// the same document written with four spaces nested correctly. Columns are
// also what a reader sees, so the rendered nesting now matches the source's
// visual nesting for every mixture of tabs and spaces.
func indentWidth(s string) int {
	col := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ':
			col++
		case '\t':
			col += 4 - col%4
		default:
			return col
		}
	}
	return col
}
