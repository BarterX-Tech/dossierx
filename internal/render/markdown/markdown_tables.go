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
//	  - A body row with fewer cells than the header is PADDED with empty cells;
//	    one with more has the extra cells DROPPED. A row is never emitted ragged.
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
// COST: PADDING IS THE ONLY UNBOUNDED PART OF THIS CONSTRUCT, and it is bounded
// here. Every other construct in this package emits output linear in its input;
// padding does not. A header of N columns costs N bytes ("|" per column) and a
// body row costs as little as ONE byte ("|"), yet each such row emits N cells —
// so N columns and M rows amplify N+M input bytes into N*M cells. At the 1 MiB
// untrusted comment-body cap that is ~10^11 cells from one stored comment,
// re-paid on every GET /api/comments. The bound is stated as an invariant
// rather than a tuned constant: A TABLE MAY NOT EMIT MORE CELLS THAN ITS OWN
// SOURCE HAS BYTES. Real tables carry several bytes per cell and are unaffected
// (the tightest legal spelling, "|||", is 1.5 bytes per cell); only genuine
// raggedness at scale trips it. A table that would trip it is not a table: the
// whole candidate block renders as ordinary paragraphs with its source bytes
// unchanged, which is the same degradation the spec gives every other malformed
// table.
//
// That refusal CONSUMES the block it refused, and must. Falling through line by
// line would make the delimiter row the next candidate header, and a document
// of alternating wide and narrow rows would re-walk the same extent once per
// line — the same quadratic shape as the four cost defects this file's package
// has already had. tableAt therefore reports the extent it examined even when
// it refuses, and renderBlocks advances past all of it.
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

// tableVerdict is what tableAt decided about a candidate at some line.
type tableVerdict uint8

const (
	// notATable: no valid delimiter row of matching arity. The candidate LINE
	// falls through to ordinary handling; nothing else is consumed, so a
	// heading or a rule on the next line is still a heading or a rule.
	notATable tableVerdict = iota
	// tableTooSparse: a real table whose padding would emit more cells than its
	// source has bytes. The whole EXTENT renders as ordinary paragraph prose
	// and is consumed (see this file's cost note).
	tableTooSparse
	// isATable: emit it.
	isATable
)

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
	var cells []string
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

// tableAt examines the table candidate beginning at lines[start] and returns
// the column alignments, the index of the candidate's LAST line, and what to do
// with it.
//
// end is meaningful for every verdict: on isATable it is the table's last row,
// on tableTooSparse it is the last line of the block that must be consumed as
// prose, and on notATable it is start itself (only that one line falls
// through). Reporting the extent even on a refusal is what keeps the scan
// linear — see this file's cost note.
//
// Cost: each line is examined at most twice, once as a candidate header and
// once as a candidate delimiter row, and the extent walk runs only after a
// delimiter row has validated — after which the extent is consumed whatever the
// verdict. Nothing here re-walks ground the scanner does not then advance past.
func tableAt(lines []string, start int) (aligns []cellAlign, end int, v tableVerdict) {
	if !hasUnescapedPipe(lines[start]) || start+1 >= len(lines) {
		return nil, start, notATable
	}
	aligns, ok := delimiterAligns(lines[start+1])
	if !ok {
		return nil, start, notATable
	}
	cols := len(splitRow(lines[start]))
	if cols != len(aligns) {
		return nil, start, notATable
	}

	// The extent: every following line up to the first blank one, the first
	// with no unescaped "|", or the end of the container.
	end = start + 1
	rows := 0
	srcBytes := len(lines[start]) + len(lines[start+1])
	for j := start + 2; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "" || !hasUnescapedPipe(lines[j]) {
			break
		}
		end, rows = j, rows+1
		srcBytes += len(lines[j])
	}

	// Every row emits exactly cols cells, header included; the delimiter row
	// emits none. A table that would emit more cells than its source has bytes
	// is padding-amplified rather than tabular.
	if cols*(1+rows) > srcBytes {
		return nil, end, tableTooSparse
	}
	return aligns, end, isATable
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

// writeTableRow emits exactly one cell per COLUMN, never one per parsed cell:
// a short row is padded with empty cells and a long one has its extra cells
// dropped, so no row is ever ragged and a dropped cell never reaches the inline
// pass at all.
//
// A cell runs renderInline with no break offsets — a cell is single-line by
// construction, so there is nothing for a hard break to break to and a trailing
// backslash stays the literal dangling backslash the inline scan renders.
func writeTableRow(b *strings.Builder, cells []string, aligns []cellAlign, header bool) {
	open, close := "<td", "</td>"
	if header {
		open, close = "<th", "</th>"
	}
	b.WriteString("<tr>")
	for i, a := range aligns {
		b.WriteString(open)
		b.WriteString(alignClass(a))
		b.WriteString(">")
		if i < len(cells) {
			b.WriteString(renderInline(cells[i], nil))
		}
		b.WriteString(close)
	}
	b.WriteString("</tr>")
}
