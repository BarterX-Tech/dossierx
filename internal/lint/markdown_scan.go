// markdown_scan.go holds the markdown scanner the two content lints share —
// markdown-sanity (markdown_sanity.go) and asset-scope (asset_scope.go). Like
// helpers.go it implements no Lint itself.
//
// WHY THIS IS A SECOND SCANNER AND NOT THE RENDERER'S.
// internal/render/markdown exports exactly two functions, Render and
// RenderInline, and both return finished template.HTML. There is no
// diagnostics API: nothing in that package can tell a caller "this fence never
// closed" or "this src was refused" — the renderer's whole contract is that
// malformed input degrades silently to literal text, which is precisely the
// silence these lints exist to break. So the recognizers below are a
// deliberate, documented MIRROR of that package's rules rather than a call
// into it, and every one of them names the rule it mirrors. Two consequences
// follow and neither is hidden:
//
//   - The mirror can drift. Anything that changes a recognition rule in
//     markdown.go (the escapable set, the fence opener's shape, the list
//     indentation rule, the scheme allowlist) must change the twin here. The
//     right long-term fix is a diagnostics entry point on the markdown package
//     — see this file's note in the release's cross-file change list.
//   - The mirror is deliberately COARSER than the renderer in the two places
//     where fidelity would cost more than it buys: a blockquote's interior is
//     re-scanned recursively (faithful) but a table cell is not re-scanned as
//     a container at all (it is inline-only in the renderer too, so there is
//     nothing to lose), and the block scan does not build a document tree —
//     it only needs to know where each inline run starts and ends.
//
// Everything here is prefixed md* so it can never collide with the lint
// package's other helpers.
package lint

import (
	"sort"
	"strings"
)

// --- the inline text unit -------------------------------------------------

// mdSeg is one source line's contribution to a block's inline text: the RAW
// line (the only place a trailing-space or trailing-backslash hard break still
// exists), the line's marker-stripped text, and its 1-based source line
// number. It mirrors markdown.segment, which exists for the same reason.
type mdSeg struct {
	raw  string
	text string
	line int
}

// mdRun is one block's prose — a paragraph, a list item's prose, a heading's
// text — as the sequence of source lines that fold into it. The inline pass
// runs over a run's JOINED text exactly once, the way the renderer's does, so
// a construct that spans a soft line break is seen to span it here too.
type mdRun struct {
	segs []mdSeg
}

// joined reproduces markdown.joinSegments: one space per line break, the
// backslash that spelled a hard break consumed, and the ascending byte offsets
// of the separator spaces that are hard breaks. renderInline caps a code span
// at the next such offset, so the lint has to know them or it would call a
// span closed that the renderer renders literally.
func (r mdRun) joined() (text string, breaks []int) {
	if len(r.segs) == 1 {
		return r.segs[0].text, nil
	}
	var b strings.Builder
	for i, s := range r.segs {
		t := s.text
		last := i == len(r.segs)-1
		brk := mdHardBreak(s.raw)
		if brk == mdBrkSlash && !last && strings.HasSuffix(t, `\`) {
			t = t[:len(t)-1]
		}
		b.WriteString(t)
		if last {
			break
		}
		if brk != mdBrkNone {
			breaks = append(breaks, b.Len())
		}
		b.WriteByte(' ')
	}
	return b.String(), breaks
}

// firstLine is the 1-based source line the run opens on, so an inline finding
// can point at a place in the file rather than at the whole field.
func (r mdRun) firstLine() int {
	if len(r.segs) == 0 {
		return 0
	}
	return r.segs[0].line
}

// danglingLine returns the source line of a trailing backslash that has
// nothing to break to — a hard-break marker on the run's LAST line — or 0.
// This is markdown.go's own definition of the dangling-backslash finding (see
// its hard-break rule): the two-space spelling emits nothing and leaves no
// trace, but the backslash spelling stays visible as a literal character the
// author did not mean to write.
func (r mdRun) danglingLine() int {
	if len(r.segs) == 0 {
		return 0
	}
	last := r.segs[len(r.segs)-1]
	if mdHardBreak(last.raw) == mdBrkSlash {
		return last.line
	}
	return 0
}

type mdBreakKind uint8

const (
	mdBrkNone mdBreakKind = iota
	mdBrkSpaces
	mdBrkSlash
)

// mdHardBreak mirrors markdown.hardBreakOf: two or more trailing spaces, or an
// ODD run of trailing backslashes (escapes resolve first, so "x\\" ends in an
// escaped literal backslash and is not a break).
func mdHardBreak(line string) mdBreakKind {
	if mdTrailingRun(line, ' ') >= 2 {
		return mdBrkSpaces
	}
	if mdTrailingRun(line, '\\')%2 == 1 {
		return mdBrkSlash
	}
	return mdBrkNone
}

func mdTrailingRun(s string, c byte) int {
	n := 0
	for n < len(s) && s[len(s)-1-n] == c {
		n++
	}
	return n
}

// --- the block scan -------------------------------------------------------

// mdIndentIssue is one list marker whose indentation matched no open level and
// therefore snapped down to the nearest enclosing one instead of nesting.
type mdIndentIssue struct {
	line  int
	width int
}

// mdTableIssue is one malformed pipe table, with the reason spelled out.
type mdTableIssue struct {
	line   int
	reason string
}

// mdScan is everything the block pass learned about one markdown source: the
// block-level defects, and the inline runs the inline pass still has to walk.
type mdScan struct {
	unclosedFences   []int
	reservedHeadings []int
	indentIssues     []mdIndentIssue
	tableIssues      []mdTableIssue
	danglingSlashes  []int
	runs             []mdRun
}

// mdScanBlocks walks one markdown source the way markdown.renderBlocks walks
// it — fences and containers met in document order — and records the
// block-level defects plus the prose runs the inline pass needs.
//
// lineBase is the 1-based source line the first element of lines sits on, so a
// blockquote's recursive scan reports line numbers in the OUTER document's
// coordinates. allowQuote is markdown.renderBlocks' one-level blockquote cap,
// mirrored exactly.
func mdScanBlocks(lines []string, lineBase int, allowQuote bool) *mdScan {
	s := &mdScan{}
	closers := mdCloserRuns(lines)

	var stack []mdLevel
	var cur []mdSeg

	flush := func() {
		if len(cur) == 0 {
			return
		}
		run := mdRun{segs: cur}
		if ln := run.danglingLine(); ln != 0 {
			s.danglingSlashes = append(s.danglingSlashes, ln)
		}
		s.runs = append(s.runs, run)
		cur = nil
	}
	// startRun ends whatever prose was open and starts a new one. A list item
	// is its own block, so its prose never folds backwards into the previous
	// item's.
	startRun := func(seg mdSeg) {
		flush()
		cur = []mdSeg{seg}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		lineNo := lineBase + i
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			// A blank line ends a paragraph or an item's prose run but only
			// ARMS the end of a list (markdown.go's loose-list rule), so the
			// level stack survives it.
			flush()
			continue
		}

		// Fences first, so an open container survives one and no construct
		// inside a fence is ever recognized.
		if runLen, ok := mdFenceOpener(line); ok {
			if closeIdx, closed := mdScanFence(lines, closers, i, runLen); closed {
				flush()
				i = closeIdx
				continue
			}
			// An unclosed fence is not a fence: markdown.go falls the opening
			// line through to ordinary handling. Report it and do the same.
			s.unclosedFences = append(s.unclosedFences, lineNo)
		}

		w := mdIndentWidth(line)

		// Column-zero-only constructs. This guard IS the container matrix's
		// list-item row: indented under an open item, a quote marker, rule or
		// heading is ordinary item prose.
		if w == 0 {
			if allowQuote && line[0] == '>' {
				flush()
				stack = nil
				end := i
				var inner []string
				for end < len(lines) {
					rest, ok := mdQuotePrefix(lines[end])
					if !ok {
						break
					}
					inner = append(inner, rest)
					end++
				}
				s.merge(mdScanBlocks(inner, lineNo, false))
				i = end - 1
				continue
			}
			if mdThematicBreak(trimmed) {
				flush()
				stack = nil
				continue
			}
			if level, text, ok := mdATXHeading(trimmed); ok {
				flush()
				stack = nil
				if level <= 2 {
					s.reservedHeadings = append(s.reservedHeadings, lineNo)
					// Levels 1 and 2 are a REJECTION in markdown.go: the line
					// renders as an ordinary paragraph, hashes and all, so its
					// whole trimmed text is the inline run.
					startRun(mdSeg{raw: line, text: trimmed, line: lineNo})
					flush()
					continue
				}
				startRun(mdSeg{raw: line, text: text, line: lineNo})
				flush()
				continue
			}
			if end, ok := s.scanTable(lines, lineBase, i); ok {
				flush()
				stack = nil
				i = end
				continue
			}
		}

		ordered, itemText, isMarker := mdListMarker(trimmed)
		if isMarker && (len(stack) > 0 || w == 0) {
			cc := w + len(trimmed) - len(itemText)
			switch {
			case len(stack) == 0:
				stack = append(stack, mdLevel{ordered: ordered, markerCol: w, contentCol: cc})
			case w >= stack[len(stack)-1].contentCol:
				stack = append(stack, mdLevel{ordered: ordered, markerCol: w, contentCol: cc})
			default:
				for len(stack) > 1 && w < stack[len(stack)-1].markerCol {
					stack = stack[:len(stack)-1]
				}
				top := &stack[len(stack)-1]
				if w != top.markerCol {
					// THE UNRESOLVABLE INDENT. markdown.go's rule is that an
					// indent matching no level's marker column "snaps DOWN to
					// the nearest enclosing one rather than inventing a level:
					// an indent is always resolvable, and nothing is ever
					// dropped". Nothing is dropped, but the nesting the author
					// drew is not the nesting they get, and only a lint can
					// say so.
					s.indentIssues = append(s.indentIssues, mdIndentIssue{line: lineNo, width: w})
				}
				*top = mdLevel{ordered: ordered, markerCol: w, contentCol: cc}
			}
			// A task-list checkbox is a marker, not prose; strip it so "[ ]"
			// is never mistaken for an unclosed link.
			itemText = mdStripTaskMarker(ordered, itemText)
			startRun(mdSeg{raw: line, text: itemText, line: lineNo})
			continue
		}

		if len(stack) > 0 && w > 0 {
			// Continuation line: it folds into an open item. Which item is
			// markdown.go's concern; for diagnostics only the prose matters.
			cur = append(cur, mdSeg{raw: line, text: trimmed, line: lineNo})
			continue
		}

		// Plain paragraph line. A non-indented, non-marker line is never a
		// list continuation, so any open list ends here.
		if len(stack) > 0 {
			flush()
			stack = nil
		}
		cur = append(cur, mdSeg{raw: line, text: trimmed, line: lineNo})
	}
	flush()
	return s
}

// merge folds a nested scan (a blockquote's interior) into its parent.
func (s *mdScan) merge(in *mdScan) {
	s.unclosedFences = append(s.unclosedFences, in.unclosedFences...)
	s.reservedHeadings = append(s.reservedHeadings, in.reservedHeadings...)
	s.indentIssues = append(s.indentIssues, in.indentIssues...)
	s.tableIssues = append(s.tableIssues, in.tableIssues...)
	s.danglingSlashes = append(s.danglingSlashes, in.danglingSlashes...)
	s.runs = append(s.runs, in.runs...)
}

// mdLevel is one open list level: the two columns every indent decision is
// made against, mirroring markdown.openLevel.
type mdLevel struct {
	ordered    bool
	markerCol  int
	contentCol int
}

// --- fences ---------------------------------------------------------------

// mdFenceOpener mirrors markdown.fenceOpener: a line whose leading whitespace
// is followed by three or more backticks with no further backtick in the info
// string. Line-anchoring is what makes a leading backslash defuse a fence and
// what keeps "use ```x``` inline" an inline span.
func mdFenceOpener(line string) (runLen int, ok bool) {
	indent := mdLeadingSpaces(line)
	runLen = mdBacktickRun(line, indent)
	if runLen < 3 {
		return 0, false
	}
	if strings.IndexByte(line[indent+runLen:], '`') != -1 {
		return 0, false
	}
	return runLen, true
}

// mdFenceCloses mirrors markdown.fenceCloses.
func mdFenceCloses(line string, openLen int) bool {
	n := mdLeadingSpaces(line)
	run := mdBacktickRun(line, n)
	if run < openLen {
		return false
	}
	return strings.TrimSpace(line[n+run:]) == ""
}

// mdCloserRuns mirrors markdown.closerRuns: result[j] is the longest backtick
// run that can close a fence at or after line j. It is what keeps an opener
// that never closes an O(1) answer instead of a walk to the end of the
// document — the same quadratic the renderer's own cost sweep bounds, reachable
// here through the same reviewer-authored bodies.
func mdCloserRuns(lines []string) []int {
	suffixMax := make([]int, len(lines)+1)
	for j := len(lines) - 1; j >= 0; j-- {
		best := suffixMax[j+1]
		n := mdLeadingSpaces(lines[j])
		if run := mdBacktickRun(lines[j], n); run > best && strings.TrimSpace(lines[j][n+run:]) == "" {
			best = run
		}
		suffixMax[j] = best
	}
	return suffixMax
}

// mdScanFence reports the closing line of the fence opened at lines[start], or
// closed=false when it never closes.
func mdScanFence(lines []string, closers []int, start, openLen int) (closeIdx int, closed bool) {
	if closers[start+1] < openLen {
		return start, false
	}
	for j := start + 1; j < len(lines); j++ {
		if mdFenceCloses(lines[j], openLen) {
			return j, true
		}
	}
	return start, false
}

// --- column-zero constructs ------------------------------------------------

// mdQuotePrefix mirrors markdown.blockquotePrefix.
func mdQuotePrefix(line string) (rest string, ok bool) {
	if line == "" || line[0] != '>' {
		return "", false
	}
	rest = line[1:]
	if rest != "" && rest[0] == ' ' {
		rest = rest[1:]
	}
	return rest, true
}

// mdThematicBreak mirrors markdown.thematicBreak: three or more dashes and
// nothing else, the subset's ONE spelling of a rule.
func mdThematicBreak(trimmed string) bool {
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

// mdATXHeading mirrors markdown.atxHeading's RECOGNITION, but not its
// rejection: it returns the level for a hash run of 1 to 6 so the caller can
// tell an accepted heading (3 to 6) from a reserved one (1 and 2). A run of
// seven or more is not a heading in CommonMark either and is not reported —
// markdown-sanity's finding set names "#" and "##" only.
func mdATXHeading(trimmed string) (level int, text string, ok bool) {
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}
	if n < len(trimmed) && trimmed[n] != ' ' && trimmed[n] != '\t' {
		return 0, "", false
	}
	return n, strings.TrimSpace(trimmed[n:]), true
}

// --- list markers ----------------------------------------------------------

// mdListMarker mirrors markdown.listMarker against a line's trimmed content.
// It is hand-rolled rather than regexp-based so it can never disagree with the
// renderer about what "\s+" matched.
func mdListMarker(trimmed string) (ordered bool, itemText string, ok bool) {
	if trimmed == "" {
		return false, "", false
	}
	if c := trimmed[0]; c == '-' || c == '*' {
		rest := trimmed[1:]
		if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			return false, "", false
		}
		return false, strings.TrimLeft(rest, " \t"), true
	}
	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(trimmed) || trimmed[i] != '.' {
		return false, "", false
	}
	rest := trimmed[i+1:]
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return false, "", false
	}
	return true, strings.TrimLeft(rest, " \t"), true
}

// mdStripTaskMarker removes a GFM checkbox from an UNORDERED item's content,
// mirroring markdown.taskMarker (ordered items have no checkbox in this
// subset).
func mdStripTaskMarker(ordered bool, itemText string) string {
	if ordered || len(itemText) < 3 || itemText[0] != '[' || itemText[2] != ']' {
		return itemText
	}
	switch itemText[1] {
	case ' ', 'x', 'X':
	default:
		return itemText
	}
	rest := itemText[3:]
	if rest == "" {
		return ""
	}
	if rest[0] != ' ' && rest[0] != '\t' {
		return itemText
	}
	return strings.TrimLeft(rest, " \t")
}

// --- pipe tables -----------------------------------------------------------
//
// GFM pipe tables are a v0.3.1 construct (amendment A9). The lint recognizes
// them where the renderer does: at indent column 0, a pipe-bearing line
// immediately followed by an alignment row.
//
// ONE DELIBERATE NARROWING, called out because it is a departure from A9's
// literal wording. A9 says "a pipe-bearing line whose next line is not a valid
// alignment row is an ordinary paragraph, reported by markdown-sanity as a
// malformed pipe table". Read literally that fires on every prose sentence
// containing a pipe — "pass a | to the shell", any BNF alternation, any
// truth-table sentence — which is a warning on correct, intentional prose and
// would train authors to ignore the rule. So the finding fires only when the
// author demonstrably ATTEMPTED a table: the following line must itself be
// alignment-row shaped AND carry an unescaped pipe of its own. The requirement
// that the alignment row carry a pipe is what keeps a paragraph followed by a
// "---" thematic break from being read as a one-column table.
//
// Within a recognized table the ragged-row half of A9 is enforced literally: a
// body row whose cell count differs from the header's is reported, because the
// renderer pads or truncates it and the author sees data silently move.

// scanTable recognizes a pipe table starting at lines[start], recording any
// malformation, and returns the index of its last line.
func (s *mdScan) scanTable(lines []string, lineBase, start int) (end int, ok bool) {
	if start+1 >= len(lines) {
		return start, false
	}
	header := strings.TrimSpace(lines[start])
	if !mdHasUnescapedPipe(header) {
		return start, false
	}
	delim := strings.TrimSpace(lines[start+1])
	if !mdHasUnescapedPipe(delim) {
		return start, false
	}
	delimCells, isDelim := mdAlignmentRow(delim)
	if !isDelim {
		return start, false
	}
	headerCells := len(mdSplitRow(header))
	if delimCells != headerCells {
		s.tableIssues = append(s.tableIssues, mdTableIssue{
			line:   lineBase + start + 1,
			reason: "the alignment row has a different number of cells than the header row, so the lines are not a table at all and render as an ordinary paragraph",
		})
		return start, false
	}
	end = start + 1
	for j := start + 2; j < len(lines); j++ {
		row := strings.TrimSpace(lines[j])
		if row == "" || !mdHasUnescapedPipe(row) {
			break
		}
		if n := len(mdSplitRow(row)); n != headerCells {
			s.tableIssues = append(s.tableIssues, mdTableIssue{
				line:   lineBase + j,
				reason: "this body row's cell count does not match the header row's, so it is padded or truncated rather than rendered ragged",
			})
		}
		end = j
	}
	return end, true
}

// mdSplitRow splits a table row on unescaped pipes, per amendment A8: "\|" is
// a literal pipe and does not split, "\\|" is an escaped backslash followed by
// a real boundary, and a code span does NOT protect a pipe. Outer pipes are
// optional, so a leading/trailing empty cell is dropped.
func mdSplitRow(s string) []string {
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) && (s[i+1] == '|' || s[i+1] == '\\') {
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if c == '|' {
			cells = append(cells, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	cells = append(cells, cur.String())
	if len(cells) > 1 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if len(cells) > 1 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// mdHasUnescapedPipe reports whether s carries a pipe that would split a row.
func mdHasUnescapedPipe(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == '|' {
			return true
		}
	}
	return false
}

// mdAlignmentRow reports whether s is a GFM alignment row (each cell being
// dashes with an optional leading and/or trailing colon) and how many cells it
// has.
func mdAlignmentRow(s string) (cells int, ok bool) {
	parts := mdSplitRow(s)
	if len(parts) == 0 {
		return 0, false
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, ":")
		p = strings.TrimSuffix(p, ":")
		if p == "" {
			return 0, false
		}
		for i := 0; i < len(p); i++ {
			if p[i] != '-' {
				return 0, false
			}
		}
	}
	return len(parts), true
}

// --- the inline scan -------------------------------------------------------

// mdImageRef is one ![alt](src) the inline pass found, with its src taken
// verbatim (parseLink's grammar takes it to the first ")").
type mdImageRef struct {
	alt string
	src string
}

// mdInline is what one run's inline pass learned.
type mdInline struct {
	// unclosedSpans holds the backtick-run lengths that never found a closing
	// run of exactly the same length.
	unclosedSpans []int
	// unbalanced holds the delimiter characters whose open/close runs did not
	// pair, in '*' '_' '~' order.
	unbalanced []byte
	// links holds every complete [text](url) url, verbatim.
	links []string
	// images holds every complete ![alt](src).
	images []mdImageRef
}

// mdScanInline is the mirror of markdown.renderInline: ONE left-to-right scan
// that recognizes backslash escapes, backtick code spans, images and links,
// with everything else ordinary text. It never re-enters itself — an escaped
// byte is stepped over, never re-examined — so the same "escapes never re-enter
// the parser" guarantee holds here.
//
// breaks are the hard-break offsets from mdRun.joined; a code span may not
// span one, exactly as in the renderer.
//
// Two constructs the renderer does not implement in this file's sibling
// package are still recognized here, on purpose:
//
//   - IMAGES. "![" is treated as an image opener. Until P7 lands the renderer
//     reads the same bytes as a literal "!" plus a link, so an author who
//     writes an image today gets neither an image nor a warning. Recognizing
//     it here is what makes the rejected-src and asset-scope gates real on the
//     day images render rather than one release later.
//   - EMPHASIS/STRIKE DELIMITERS. Bold, italic and strikethrough are an
//     explicit non-goal of the renderer's subset, so "*bold*" renders with its
//     asterisks visible. The finding exists to say that out loud rather than
//     let an author ship markup that silently is not markup.
func mdScanInline(text string, breaks []int) mdInline {
	var in mdInline
	// runs collects the delimiter runs that can UNAMBIGUOUSLY open or close;
	// see mdDelimRuns for why ambiguous ones are excluded.
	var delims []mdDelimRun

	x := newMDInlineIndex(text)

	i := 0
	for i < len(text) {
		switch c := text[i]; c {
		case '\\':
			if i+1 < len(text) && mdIsEscapable(text[i+1]) {
				i += 2
			} else {
				i++
			}

		case '`':
			runLen := mdBacktickRun(text, i)
			contentStart := i + runLen
			limit := mdNextBreak(breaks, contentStart, len(text))
			closeStart, ok := x.findBacktickRun(contentStart, runLen)
			if ok && closeStart >= limit {
				ok = false
			}
			if !ok {
				in.unclosedSpans = append(in.unclosedSpans, runLen)
				i = contentStart
				continue
			}
			i = closeStart + runLen

		case '!':
			if i+1 < len(text) && text[i+1] == '[' {
				if matchLen, alt, src, ok := x.parseLinkAt(text, i+1); ok {
					in.images = append(in.images, mdImageRef{alt: alt, src: src})
					i += 1 + matchLen
					continue
				}
			}
			i++

		case '[':
			if matchLen, _, url, ok := x.parseLinkAt(text, i); ok {
				in.links = append(in.links, url)
				i += matchLen
				continue
			}
			i++

		case '*', '_', '~':
			runLen := 1
			for i+runLen < len(text) && text[i+runLen] == c {
				runLen++
			}
			if d, ok := mdDelimRunAt(text, i, runLen, c); ok {
				delims = append(delims, d)
			}
			i += runLen

		default:
			i++
		}
	}

	in.unbalanced = mdUnbalancedDelims(delims)
	return in
}

// mdDelimRun is one emphasis/strike delimiter run that can unambiguously open
// or close.
type mdDelimRun struct {
	char  byte
	opens bool
}

// mdDelimRunAt classifies a delimiter run, returning ok=false for a run that
// is not a delimiter at all or that is AMBIGUOUS (both left- and
// right-flanking).
//
// The ambiguity exclusion is what makes this finding usable on a real claim
// corpus rather than a noise generator. CommonMark's flanking rules alone
// would call the "*" in "count(*)" both an opener and a closer, and the "_" in
// an identifier a delimiter; a claim corpus is full of both. So:
//
//   - a "_" run sitting between two alphanumerics is intraword and is not a
//     delimiter at all (CommonMark's own intraword-underscore rule) — this is
//     what keeps "governed_by" and "rests_on", the two most common tokens in
//     any DossierX body, silent;
//   - a run that is both left- and right-flanking ("2*3", "count(*)") is
//     skipped, because a reader cannot tell which way the author meant it and
//     neither can the lint;
//   - a "~" run must be at least two characters, so "~/some/path" is a path
//     and not an unclosed strikethrough.
//
// What remains is exactly the unambiguous case: a run with whitespace (or a
// text edge) on one side and content on the other, which can only be an opener
// or only be a closer.
func mdDelimRunAt(text string, start, runLen int, c byte) (mdDelimRun, bool) {
	if c == '~' && runLen < 2 {
		return mdDelimRun{}, false
	}
	var prev, next byte = ' ', ' '
	if start > 0 {
		prev = text[start-1]
	}
	end := start + runLen
	if end < len(text) {
		next = text[end]
	}
	if c == '_' && mdIsAlnum(prev) && mdIsAlnum(next) {
		return mdDelimRun{}, false
	}
	prevSpace := mdIsSpaceByte(prev)
	nextSpace := mdIsSpaceByte(next)
	leftFlanking := !nextSpace
	rightFlanking := !prevSpace
	if leftFlanking == rightFlanking {
		// Neither (" * ") or both ("2*3"): not an unambiguous delimiter.
		return mdDelimRun{}, false
	}
	return mdDelimRun{char: c, opens: leftFlanking}, true
}

// mdUnbalancedDelims pairs the runs left to right and returns, in a stable
// '*' '_' '~' order, every character that had a leftover opener or a closer
// with nothing open.
func mdUnbalancedDelims(runs []mdDelimRun) []byte {
	open := map[byte]int{}
	bad := map[byte]bool{}
	for _, r := range runs {
		if r.opens {
			open[r.char]++
			continue
		}
		if open[r.char] == 0 {
			bad[r.char] = true
			continue
		}
		open[r.char]--
	}
	for ch, n := range open {
		if n > 0 {
			bad[ch] = true
		}
	}
	var out []byte
	for _, ch := range []byte{'*', '_', '~'} {
		if bad[ch] {
			out = append(out, ch)
		}
	}
	return out
}

// mdInlineIndex is the lint's copy of the two cost structures the renderer
// carries (markdown.backtickIndex and markdown.linkIndex), for exactly the
// same reason and against exactly the same input.
//
// Both of the searches this scan makes — "the next backtick run of length n"
// and parseLink's "the next ] then the next )" — consume nothing when they
// FAIL, so the scan advances one byte and the next opener repeats the same
// walk over the same remainder: O(N^2) in the length of one block. The
// renderer had precisely these two quadratics, they were reachable from a
// reviewer-authored comment body capped at 1 MiB, and they are now bounded and
// guarded by a growth sweep. This lint reads the SAME bodies — including
// comment bodies, through mdImagesIn — so a mirror without the bound would
// simply move the cost from the render path to the lint path and leave "dossierx
// check" as the slow surface instead.
//
// Unlike the renderer's, this index is built EAGERLY rather than at the first
// failure. The renderer indexes lazily because it must not add an allocation
// to the ordinary render path; a lint has no such constraint, and building
// unconditionally means there is no threshold anywhere for an input to sit
// just underneath.
type mdInlineIndex struct {
	backticksByLen map[int][]int
	closeBrackets  []int
	closeParens    []int
}

// newMDInlineIndex records every backtick RUN (by length), every "]" and every
// ")" in text, in ascending order.
//
// Runs are measured the way the scan measures them — maximal, taken left to
// right — so a position recorded here is always a position the walking search
// would itself have considered. Escapes are deliberately NOT accounted for,
// matching markdown.backtickIndex/linkIndex, which are also built over raw
// text: the lint has to agree with the renderer about which delimiter closes
// what, including in the corners where an escaped byte was indexed.
func newMDInlineIndex(text string) *mdInlineIndex {
	x := &mdInlineIndex{backticksByLen: make(map[int][]int)}
	for i := 0; i < len(text); {
		switch text[i] {
		case '`':
			run := mdBacktickRun(text, i)
			x.backticksByLen[run] = append(x.backticksByLen[run], i)
			i += run
			continue
		case ']':
			x.closeBrackets = append(x.closeBrackets, i)
		case ')':
			x.closeParens = append(x.closeParens, i)
		}
		i++
	}
	return x
}

// findBacktickRun answers markdown.findBacktickRun by lookup: the start of the
// first run of EXACTLY n backticks at or after from.
func (x *mdInlineIndex) findBacktickRun(from, n int) (int, bool) {
	return mdNextAt(x.backticksByLen[n], from)
}

// parseLinkAt answers markdown.parseLink(text[at:]) by lookup: same match
// length, same verbatim link text, same verbatim url, same ok. It keeps the
// v0.3.1 grammar ceiling exactly — the FIRST "]" and the FIRST ")", no
// nesting, no escaped delimiters inside a link — because a lint that parsed a
// wider grammar would report on links the renderer never sees.
func (x *mdInlineIndex) parseLinkAt(text string, at int) (matchLen int, linkText, url string, ok bool) {
	if at >= len(text) || text[at] != '[' {
		return 0, "", "", false
	}
	closeBr, found := mdNextAt(x.closeBrackets, at+1)
	if !found || closeBr+1 >= len(text) || text[closeBr+1] != '(' {
		return 0, "", "", false
	}
	linkText = text[at+1 : closeBr]
	urlStart := closeBr + 2 // skip "]("
	closeParen, found := mdNextAt(x.closeParens, urlStart)
	if !found {
		return 0, "", "", false
	}
	url = text[urlStart:closeParen]
	return closeParen + 1 - at, linkText, url, true
}

// mdNextAt returns the first offset in the ascending slice pos that is at or
// after from.
func mdNextAt(pos []int, from int) (int, bool) {
	k := sort.Search(len(pos), func(i int) bool { return pos[i] >= from })
	if k == len(pos) {
		return 0, false
	}
	return pos[k], true
}

func mdNextBreak(breaks []int, from, def int) int {
	if len(breaks) == 0 {
		return def
	}
	k := sort.Search(len(breaks), func(i int) bool { return breaks[i] >= from })
	if k == len(breaks) {
		return def
	}
	return breaks[k]
}

// mdImagesIn returns every ![alt](src) in one markdown source, in document
// order, skipping fence content (a fenced example is source code, not an
// image reference). It is the entry point asset-scope uses: that lint cares
// only about where an image points, never about the prose around it.
func mdImagesIn(source string) []mdImageRef {
	var out []mdImageRef
	scan := mdScanBlocks(strings.Split(source, "\n"), 1, true)
	for _, run := range scan.runs {
		text, breaks := run.joined()
		out = append(out, mdScanInline(text, breaks).images...)
	}
	return out
}

// --- url gates -------------------------------------------------------------
//
// THERE ARE NONE IN THIS FILE ANY MORE, AND THAT IS THE POINT. This file used
// to carry mdAllowedScheme, mdImageSrcLegal, mdImageSrcOffOrigin,
// mdIsNetworkPath, mdSchemeOf and mdStripCtrlAndSpaceBytes — a hand-copied
// mirror of internal/render/markdown's URL rules, added because that package
// exports no diagnostics API (see this file's header) and a lint cannot ask it
// what it refused.
//
// The mirror argument holds for the SCANNER, whose job is to see what the
// renderer sees. It never held for the URL GATES, which are not recognition at
// all but a security decision that has exactly one correct answer, and which
// four files in this repo had four private copies of — one of which, the mockup
// <img> gate in raw_html_scope.go, had drifted into a live off-origin hole.
// Those six functions are gone; both lints now call internal/urlsafe, the same
// leaf package the renderer calls, so this half of the mirror cannot drift by
// construction rather than by vigilance.
//
// markdown-sanity uses urlsafe.IsAllowedHref for a link href and
// urlsafe.IsRelativePath for an image src; asset-scope uses
// urlsafe.IsOffOrigin. The last split is deliberate and is why urlsafe exports
// the off-origin question separately from the relative-path one: a
// "../other-facet/assets/x.png" is refused by the renderer AND is the canonical
// co-location mistake, so asset-scope reports it too rather than deferring
// (deliberate co-firing, on the package's existing cycle/self-edge and
// dangling/mirror-unanchored precedent), whereas an OFF-ORIGIN src is not a
// path at all and has no directory to be in or out of, so asset-scope stays
// silent on it and markdown-sanity alone reports it.

// --- byte helpers ----------------------------------------------------------

// mdIsEscapable mirrors markdown.isEscapable's CLOSED 15-character set:
//
//	\ ` * _ ~ [ ] ( ) > # - . ! |
func mdIsEscapable(c byte) bool {
	switch c {
	case '\\', '`', '*', '_', '~', '[', ']', '(', ')', '>', '#', '-', '.', '!', '|':
		return true
	}
	return false
}

func mdBacktickRun(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// mdLeadingSpaces counts BYTES of leading whitespace — a slicing offset, never
// an indent measurement (mirrors markdown.leadingSpaces).
func mdLeadingSpaces(s string) int {
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') {
		n++
	}
	return n
}

// mdIndentWidth measures leading whitespace in COLUMNS, a tab advancing to the
// next multiple of four (mirrors markdown.indentWidth). Every list decision is
// made in these units and never in bytes.
func mdIndentWidth(s string) int {
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

// mdIsAlpha/mdIsDigit/mdIsAlnum classify a byte for the INLINE scan's
// intraword-underscore rule only. They were also the scheme grammar's character
// classes until the URL gates moved to internal/urlsafe; urlsafe carries its own
// copies now, deliberately, because a byte class shared across a package
// boundary would make the scheme grammar look like something a scanner change
// could edit.
func mdIsAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func mdIsDigit(c byte) bool { return c >= '0' && c <= '9' }
func mdIsAlnum(c byte) bool { return mdIsAlpha(c) || mdIsDigit(c) }
func mdIsSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}
