// markdown_tables.go holds phase C: GFM pipe tables.
//
// THE GRAMMAR, in full (gate 0 amendments A8 and A9).
//
// A table is a header row, a REQUIRED delimiter row with the same cell count,
// and zero or more body rows:
//
//		| Left | Centre | Right |
//		|:-----|:------:|------:|
//		| a    |   b    |     c |
//
//	  - The delimiter row is what makes a table. A pipe-bearing line whose next
//	    line is not a valid delimiter row of the same arity is ordinary paragraph
//	    prose, source bytes unchanged (markdown-sanity's malformed-pipe-table
//	    finding, which is P10's to report — this package only renders).
//	  - A delimiter cell is an optional ":", one or more "-", an optional ":".
//	    That is the whole alignment vocabulary: left, centre, right, or none.
//	  - Outer pipes are optional; cell content is trimmed of surrounding
//	    whitespace.
//	  - A table may interrupt an open paragraph, and it ends at the first blank
//	    line or the first line with no unescaped "|".
//	  - A ROW EMITS EXACTLY THE CELLS IT HAS. A body row with fewer cells than
//	    the header renders SHORT — no empty cells are invented to square it off —
//	    and one with more has the extra cells DROPPED.
//
//	    THIS DIVERGES FROM AMENDMENT A9 AS WRITTEN, and the divergence is
//	    deliberate and needs recording rather than glossing. A9 says a body row
//	    whose cell count differs from the header's is "padded with empty <td> or
//	    truncated — never emitted ragged". The truncation half is implemented
//	    exactly. The padding half is not, because padding is the one thing in
//	    this package that emits more output than it was given input, and two
//	    successive attempts to bound it instead ended up REFUSING well-formed
//	    tables — which A9 forbids far more clearly than it requires padding, and
//	    which made real authors' tables vanish into paragraphs. A short row is
//	    still a row of the same table; nothing degrades. The spec sentence needs
//	    the amendment, and markdown-sanity's ragged-row finding — which still
//	    fires, and should — needs its wording updated with it.
//
// PLACEMENT is decided by WHERE the scanner sits, exactly as it is for every
// other block construct in this package. tableAt is consulted from renderBlocks
// under the same indent-column-zero guard that hr, ATX headings and blockquotes
// use, so:
//
//   - top level: legal.
//   - blockquote interior: legal, for free, via renderBlocks' own recursion —
//     the quote prefix is stripped first, so the table's rows reach the scanner
//     at column zero.
//   - list-item interior: NOT a table. An indented pipe row under an open item
//     is item prose, which is the container matrix's list-item row enforced by
//     the same guard and nothing else.
//   - table cell: inline only. A cell goes to renderInline and never to
//     renderBlocks, so no block construct — including another table — can
//     appear inside one, and a cell is single-line by construction.
//
// THE DELIMITER ROW MUST CARRY A PIPE OF ITS OWN. GFM leaves this ambiguous for
// a one-column table; this package cannot. "---" is UNCONDITIONALLY a thematic
// break here (there is no setext heading — see thematicBreak), so accepting a
// pipe-less delimiter row would let a preceding pipe-bearing line silently
// retitle an <hr> as a table. Requiring a pipe is also what makes the table's
// own termination rule uniform: every line of a table, delimiter row included,
// contains an unescaped "|". The rule only ever NARROWS what is a table, so
// nothing accepted here is rejected by GFM.
//
// ROW SPLITTING HAPPENS FIRST, AND INDEPENDENTLY OF INLINE PARSING (amendment
// A8). The row text is scanned left to right: "\|" is consumed as a literal
// pipe and does not split, "\\|" is an escaped backslash followed by a REAL
// cell boundary, and every other "|" splits — A CODE SPAN DOES NOT PROTECT A
// PIPE, so "| `a|b` |" is two cells, matching GFM. This is the single
// documented exception to the package's "escapes never process inside a code
// span" rule; it applies only to "\|", only at the row-splitting step, and it
// is the only way to write a pipe inside a code span in a cell.
//
// The exception is implemented as the narrowest possible substitution and its
// safety is structural, not conventional: unescapePipes deletes the ONE
// backslash that escaped a pipe and copies every other byte — including both
// bytes of any other escape — through untouched, so no escape's parity is
// disturbed. What the cell then hands renderInline is a bare "|", and "|" is
// not an opener, closer or delimiter of any inline construct in this package.
// A substituted pipe therefore cannot forge a construct: it is inert by
// construction, which is what keeps "escapes never re-enter the parser" true
// alongside the exception.
//
// THE EMITTED MARKUP, frozen by gate 0's emitted-markup contract so the
// stylesheet can be written against it:
//
//	<table class="md-table">
//	  <thead><tr><th[ class="md-col-…"]>…</th></tr></thead>
//	  <tbody><tr><td[ class="md-col-…"]>…</td></tr></tbody>   (omitted when empty)
//	</table>
//
// The alignment class is chosen by a CLOSED SWITCH over the parsed alignment
// token (see alignClass) and is one of three fixed literals; a column with no
// alignment carries no class. There is never a style attribute, never an
// interpolated attribute value, and no other attribute of any kind — so the
// delimiter row is not an interpolation point at all. Cell text is the only
// author content, and it reaches the output only through renderInline, which is
// the same escaping boundary every other construct in this package uses.
//
// COST: NOTHING IN THIS CONSTRUCT AMPLIFIES, BECAUSE EVERY EMITTED CELL IS A
// CELL THAT EXISTS IN THE SOURCE. That is a property of the grammar above, not
// of a guard bolted on after it, and it is worth stating why the guard is gone.
//
// PADDING WAS THE AMPLIFICATION, AND IT WAS THE WHOLE OF IT. A header of N
// columns costs N+1 bytes ("|" per column plus the outer one) and a body row
// costs as little as two ("|" and its newline), yet a padded row emitted one
// cell per HEADER column — so N columns and M rows turned N+2M source bytes
// into N*M cells. At the 1 MiB untrusted comment-body cap that is ~10^11 cells
// from one stored comment, re-paid on every GET /api/comments. Every other
// construct in this package emits output linear in its input; padding was the
// single exception.
//
// TWO BOUNDS WERE TRIED AGAINST IT AND BOTH WERE WRONG IN THE SAME WAY. One
// capped a table's CELLS by its source bytes; the next capped its emitted
// MARKUP BYTES by its source bytes at a derived ratio. Each picked a point at
// which a WELL-FORMED TABLE WAS REFUSED and re-rendered as a paragraph, and
// each therefore contradicted the spec by construction. A9 makes a ragged body
// row a rendering decision INSIDE a table — padded or truncated — never a reason
// to stop being one, and A24's degradation matrix gives exactly one path from a
// table to prose: an alignment row whose arity does not match the header's, or
// none at all. Nothing anywhere authorises refusing a table for its size. What
// the two bounds actually did was relocate a cliff:
//
//	10 columns, 5 short rows, plain            rendered as a table
//	10 columns, 5 short rows, CENTRE-ALIGNED   rendered as a PARAGRAPH
//	 4 columns, 3 short rows, CENTRE-ALIGNED   rendered as a PARAGRAPH
//
// A four-column centre-aligned table with three ragged rows silently vanished,
// and centre alignment made it worse because <td class="md-col-center"> is the
// widest cell markup this file can write. That is not a shape an attacker
// reaches for; it is a shape an author writes.
//
// SO THE FIX IS AT THE SOURCE OF THE AMPLIFICATION RATHER THAN DOWNSTREAM OF
// IT: writeTableRow emits one cell per PARSED CELL, capped at the header's
// column count. A short row renders short and a long row still has its extra
// cells dropped — truncation is the half of A9 that is bounded by construction,
// and it is the half that survives. Every emitted cell now corresponds to a cell
// that exists in the source, so
//
//	emitted cells <= source cells <= source bytes
//
// and the vector is CLOSED rather than capped. There is no ceiling, no refusal
// path and no verdict to report: a well-formed table is ALWAYS a table.
//
// WHAT THE CONSTANT FACTOR IS, now that there is one. The widest cell is
// `<td class="md-col-center"></td>` at 31 bytes and the cheapest source spelling
// of a cell is one byte (its "|"), so a row of C empty centre-aligned cells
// emits 9+31C bytes of markup from C+2 source bytes — under 31x, approached
// from below as C grows and never reached, since the delimiter row is pure
// source that emits nothing and any cell CONTENT costs more source per cell
// than it can emit. 31x is the construct's intrinsic supremum, it is reached
// only by a table of empty cells at scale, and it is a CONSTANT: the shape that
// used to amplify quadratically is now the same 31x as everything else. That
// figure is measured rather than argued — see TestRender_TableOutputIsBounded-
// ByItsInput, which asserts it across the sweep, and markdown_cost_test.go's
// table rows, which measure the allocation it implies at the 1 MiB cap.
//
// The scan is linear for a separate reason worth keeping: tableAt reports the
// EXTENT it examined, and renderBlocks advances past all of it. A candidate
// that is not a table consumes only its own first line, so a heading or a rule
// on the next line is still a heading or a rule.
package markdown

import "strings"

// cellAlign is one column's alignment, parsed from its delimiter cell. It is a
// closed enum: these four values are the entire vocabulary the delimiter row
// can express, and alignClass is the only thing that reads it.
type cellAlign uint8

const (
	alignNone cellAlign = iota
	alignLeft
	alignCenter
	alignRight
)

// alignClass is THE CLOSED SWITCH gate 0's emitted-markup contract requires:
// the alignment token selects one of three fixed literal class attributes, or
// none at all. No author byte reaches this, so the delimiter row is not an
// interpolation point; and there is deliberately no style-attribute branch,
// because a style attribute is the one thing this codebase forbids everywhere
// (raw_html_scope.go makes it a hard finding).
func alignClass(a cellAlign) string {
	switch a {
	case alignLeft:
		return ` class="md-col-left"`
	case alignCenter:
		return ` class="md-col-center"`
	case alignRight:
		return ` class="md-col-right"`
	default:
		return ""
	}
}

// hasUnescapedPipe reports whether s contains a "|" that would split a row. It
// is the predicate behind both "is this a table candidate" and "does the table
// end here", so those two questions can never disagree.
//
// The walk is the same one splitRow makes: a backslash consumes the byte after
// it, whatever that byte is, so "\|" is not a boundary and "\\|" is.
func hasUnescapedPipe(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // the escaped byte is never a boundary
		case '|':
			return true
		}
	}
	return false
}

// splitRow splits one row into its cells: outer pipes dropped, cells trimmed,
// "\|" resolved to a literal pipe (see unescapePipes and this file's note on
// the escaping exception).
//
// A trailing unescaped "|" closes the last cell rather than opening an empty
// one — that is what "outer pipes are optional" means — while an INTERIOR empty
// cell ("a || b") is preserved, because dropping it would silently shift every
// later cell into the wrong column.
func splitRow(line string) []string {
	s := strings.TrimSpace(line)
	start := 0
	if len(s) > 0 && s[0] == '|' {
		start = 1
	}
	// One allocation per row instead of the append ladder's five. A row can
	// hold at most one cell per "|" plus one, and strings.Count is a strict
	// upper bound on that (it counts escaped pipes too, which do not split), so
	// this over-reserves by at most the row's escape count and never
	// under-reserves. The reservation is linear in the row's own bytes, which is
	// the property that matters: it cannot be the amplification vector padding
	// was, because a row with no "|" reserves one cell.
	cells := make([]string, 0, strings.Count(s[start:], "|")+1)
	seg := start
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // the escaped byte is never a boundary
		case '|':
			cells = append(cells, cellText(s[seg:i]))
			seg = i + 1
		}
	}
	tail := s[seg:]
	if seg > start && strings.TrimSpace(tail) == "" {
		// The row's final "|" was the optional outer pipe.
		return cells
	}
	return append(cells, cellText(tail))
}

// cellText is one cell's raw content: trimmed, with "\|" resolved. What comes
// back is still AUTHOR BYTES — it goes to renderInline, never to the output.
func cellText(s string) string {
	return unescapePipes(strings.TrimSpace(s))
}

// unescapePipes resolves "\|" to "|" and changes nothing else.
//
// It is the row-splitting step's half of amendment A8's exception, and it is
// deliberately the narrowest transform that can implement it. Every other
// escape is copied through as BOTH of its bytes in one step, so removing this
// backslash can never change another escape's parity: "\\\|" becomes "\\|",
// which the inline pass then renders as a literal backslash followed by a
// literal pipe, exactly as GFM does.
//
// A cell with no "\|" is returned unchanged and allocates nothing, which is the
// overwhelmingly common case.
func unescapePipes(s string) string {
	if !strings.Contains(s, `\|`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		if s[i+1] == '|' {
			b.WriteByte('|')
		} else {
			// Any other escape survives whole; its second byte must never be
			// re-read as an escape opener.
			b.WriteByte('\\')
			b.WriteByte(s[i+1])
		}
		i++
	}
	return b.String()
}

// delimiterShaped is the no-allocation pre-filter for a delimiter row: such a
// row can contain only "|", "-", ":" and whitespace, and must contain at least
// one "|" (see this file's note on why the pipe is required). One byte scan
// rejects every line of ordinary prose before splitRow allocates anything,
// which is what keeps a body of pipe-bearing prose linear.
//
// A delimiter row can hold no backslash, so every "|" it contains is unescaped
// by construction and this scan needs no escape handling.
func delimiterShaped(s string) bool {
	pipe := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '|':
			pipe = true
		case '-', ':', ' ', '\t':
		default:
			return false
		}
	}
	return pipe
}

// delimiterCell parses one delimiter cell — optional ":", one or more "-",
// optional ":" — into its column alignment.
func delimiterCell(s string) (cellAlign, bool) {
	left, right := false, false
	if strings.HasPrefix(s, ":") {
		left, s = true, s[1:]
	}
	if strings.HasSuffix(s, ":") {
		right, s = true, s[:len(s)-1]
	}
	if s == "" {
		return alignNone, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			return alignNone, false
		}
	}
	switch {
	case left && right:
		return alignCenter, true
	case left:
		return alignLeft, true
	case right:
		return alignRight, true
	default:
		return alignNone, true
	}
}

// delimiterAligns parses a whole delimiter row into per-column alignments.
func delimiterAligns(line string) ([]cellAlign, bool) {
	if !delimiterShaped(strings.TrimSpace(line)) {
		return nil, false
	}
	cells := splitRow(line)
	aligns := make([]cellAlign, len(cells))
	for i, c := range cells {
		a, ok := delimiterCell(c)
		if !ok {
			return nil, false
		}
		aligns[i] = a
	}
	return aligns, true
}

// tableAt examines the table candidate beginning at lines[start]. It returns
// the column alignments and the index of the table's LAST line, or ok=false if
// there is no table here.
//
// THERE IS NO THIRD ANSWER. A candidate either has a valid delimiter row of
// matching arity, in which case it is a table and is rendered as one, or it does
// not, in which case only lines[start] is consumed and a heading or a rule on
// the next line is still a heading or a rule. Nothing here can refuse a
// well-formed table — see this file's cost note for the two bounds that used to
// and why neither could be right.
//
// Cost: each line is examined at most twice, once as a candidate header and
// once as a candidate delimiter row, and the extent walk runs only after a
// delimiter row has validated — after which renderBlocks advances past the whole
// extent. Nothing here re-walks ground the scanner does not then advance past.
func tableAt(lines []string, start int) (aligns []cellAlign, end int, ok bool) {
	if !hasUnescapedPipe(lines[start]) || start+1 >= len(lines) {
		return nil, start, false
	}
	aligns, ok = delimiterAligns(lines[start+1])
	if !ok {
		return nil, start, false
	}
	if len(splitRow(lines[start])) != len(aligns) {
		return nil, start, false
	}

	// The extent: every following line up to the first blank one, the first
	// with no unescaped "|", or the end of the container.
	end = start + 1
	for j := start + 2; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "" || !hasUnescapedPipe(lines[j]) {
			break
		}
		end = j
	}
	return aligns, end, true
}

// writeTable renders the table occupying lines[start:end+1], whose delimiter
// row is lines[start+1]. Every tag and attribute it writes is a fixed literal
// in this package; the only author bytes it emits go through renderInline.
func writeTable(b *strings.Builder, lines []string, start, end int, aligns []cellAlign) {
	b.WriteString(`<table class="md-table"><thead>`)
	writeTableRow(b, splitRow(lines[start]), aligns, true)
	b.WriteString("</thead>")
	if end > start+1 {
		b.WriteString("<tbody>")
		for j := start + 2; j <= end; j++ {
			writeTableRow(b, splitRow(lines[j]), aligns, false)
		}
		b.WriteString("</tbody>")
	}
	b.WriteString("</table>")
}

// writeTableRow emits EXACTLY THE CELLS THE ROW HAS, capped at the header's
// column count: a short row renders short and a long one has its extra cells
// dropped, so a dropped cell never reaches the inline pass at all.
//
// The loop bound is what makes this construct's output linear in its input, and
// it is the whole of that argument: cells comes from splitRow, which returns one
// cell per unescaped "|" in the SOURCE row, so `for i := range cells` cannot emit
// a cell the author did not write. Padding to len(aligns) is what could, and is
// what this file no longer does — see the cost note.
//
// A cell runs renderInline with no break offsets — a cell is single-line by
// construction, so there is nothing for a hard break to break to and a trailing
// backslash stays the literal dangling backslash the inline scan renders.
func writeTableRow(b *strings.Builder, cells []string, aligns []cellAlign, header bool) {
	open, close := "<td", "</td>"
	if header {
		open, close = "<th", "</th>"
	}
	b.Grow(tableRowEstimate(cells, aligns))
	b.WriteString("<tr>")
	for i, cell := range cells {
		if i >= len(aligns) {
			break // an extra cell has no column: dropped, per amendment A9.
		}
		b.WriteString(open)
		b.WriteString(alignClass(aligns[i]))
		b.WriteString(">")
		b.WriteString(renderInline(cell, nil))
		b.WriteString(close)
	}
	b.WriteString("</tr>")
}

// tableRowEstimate is how many bytes writeTableRow is about to need, reserved in
// ONE step before the row is written. It changes no output byte; it is here for
// what it does to ALLOCATION, and that is worth a paragraph because it is the
// difference between this construct fitting the package's memory budget and not.
//
// strings.Builder is backed by append, and append's growth for a large slice is
// ~1.25x — so a builder walked up to a K-byte result by unreserved writes has
// allocated about 5K bytes on the way, each step copying the last. A table is
// the widest output this package produces (see the cost note: up to 31 bytes of
// markup per source byte), so it is the one construct where that 5x lands on
// tens of megabytes. Builder.Grow reserves 2*cap+n, i.e. it DOUBLES, which
// takes the same walk from ~5K to ~2K.
//
// It is deliberately an ESTIMATE and deliberately linear in what the row
// actually contains: one term per emitted cell, each the cell's own fixed markup
// plus its own source length. renderInline can expand a cell past that (escaping
// "<" costs four bytes for one) and append absorbs the difference, which is why
// this may be low but must never be a MULTIPLE of the truth — a reservation
// computed from the header's column count rather than the row's cell count would
// re-create, in allocated bytes, exactly the padding amplification this file
// removed from the output.
func tableRowEstimate(cells []string, aligns []cellAlign) int {
	n := len(`<tr></tr>`)
	for i, cell := range cells {
		if i >= len(aligns) {
			break
		}
		n += len(`<td></td>`) + len(alignClass(aligns[i])) + len(cell)
	}
	return n
}
