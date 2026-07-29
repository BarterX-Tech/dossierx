package markdown

import (
	"strings"
	"testing"
)

// markdown_tables_test.go is phase C's unit suite: the GFM pipe table grammar,
// its placement in the container matrix, its ONE degradation path — a candidate
// with no valid delimiter row is prose, and that is the only one there is — and
// the one deliberate exception to the escaping rules (see the "\|" tests below).
//
// The golden corpus under testdata/markdown-cases carries the same behaviors as
// whole-document fixtures; these are the focused cases, including the ones whose
// input is too large or too adversarial to read as a fixture.

// --- the grammar ----------------------------------------------------------

func TestRender_TableRequiresADelimiterRow(t *testing.T) {
	// The delimiter row is what makes a pipe-bearing line a table. Without one
	// the lines are ordinary paragraph prose, source bytes unchanged.
	for _, body := range []string{
		"| a | b |\n| c | d |",
		"| a | b |",
		"a | b\nc | d",
	} {
		out := string(Render(body))
		if strings.Contains(out, "<table") {
			t.Errorf("body %q became a table with no delimiter row:\n%s", body, out)
		}
		if !strings.HasPrefix(out, "<p>") {
			t.Errorf("body %q did not render as a paragraph:\n%s", body, out)
		}
	}
}

func TestRender_TableSimple(t *testing.T) {
	body := "| a | b |\n| --- | --- |\n| 1 | 2 |"
	want := `<table class="md-table"><thead><tr><th>a</th><th>b</th></tr></thead>` +
		`<tbody><tr><td>1</td><td>2</td></tr></tbody></table>`
	if got := string(Render(body)); got != want {
		t.Errorf("simple table\nwant: %s\ngot:  %s", want, got)
	}
}

func TestRender_TableAlignmentClasses(t *testing.T) {
	// A closed switch over the parsed alignment token: a fixed literal class,
	// never a style attribute and never an interpolated value. A column with no
	// alignment carries no class at all.
	body := "| l | c | r | n |\n| :--- | :-: | ---: | --- |\n| 1 | 2 | 3 | 4 |"
	out := string(Render(body))
	for _, want := range []string{
		`<th class="md-col-left">l</th>`,
		`<th class="md-col-center">c</th>`,
		`<th class="md-col-right">r</th>`,
		`<th>n</th>`,
		`<td class="md-col-left">1</td>`,
		`<td class="md-col-center">2</td>`,
		`<td class="md-col-right">3</td>`,
		`<td>4</td>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("alignment: want substring %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "style") {
		t.Errorf("alignment emitted a style attribute (forbidden):\n%s", out)
	}
}

func TestRender_TableDelimiterCellRejections(t *testing.T) {
	// A delimiter cell is: optional ":", one or more "-", optional ":". Anything
	// else and the block is not a table at all.
	for _, delim := range []string{
		"| :: | --- |",    // no dashes
		"| :  | --- |",    // a bare colon
		"| -x- | --- |",   // a non-dash byte
		"| --- | *** |",   // the other rule spelling is not a delimiter
		"| ---- - | -- |", // an interior space splits the dash run
		"|    | --- |",    // an empty cell
	} {
		body := "| a | b |\n" + delim + "\n| 1 | 2 |"
		if out := string(Render(body)); strings.Contains(out, "<table") {
			t.Errorf("delimiter %q was accepted:\n%s", delim, out)
		}
	}
}

func TestRender_TableDelimiterArityMustMatchHeader(t *testing.T) {
	for _, body := range []string{
		"| a | b |\n| --- |\n| 1 | 2 |",
		"| a |\n| --- | --- |\n| 1 |",
	} {
		if out := string(Render(body)); strings.Contains(out, "<table") {
			t.Errorf("arity mismatch was accepted for %q:\n%s", body, out)
		}
	}
}

func TestRender_TableDelimiterRowNeedsAPipe(t *testing.T) {
	// "---" is unconditionally a thematic break in this package (there is no
	// setext heading), and a preceding pipe-bearing line must not reinterpret
	// it. A delimiter row therefore has to carry a pipe of its own, exactly as
	// every other row of a table does.
	out := string(Render("| a |\n---\n"))
	if strings.Contains(out, "<table") {
		t.Errorf("a pipe-less delimiter row was accepted:\n%s", out)
	}
	if !strings.Contains(out, "<hr>") {
		t.Errorf("the thematic break was lost:\n%s", out)
	}
}

func TestRender_TableOuterPipesAreOptional(t *testing.T) {
	bodies := []string{
		"| a | b |\n| --- | --- |\n| 1 | 2 |",
		"a | b\n--- | ---\n1 | 2",
		"| a | b\n| --- | ---\n| 1 | 2",
		"a | b |\n--- | --- |\n1 | 2 |",
	}
	want := string(Render(bodies[0]))
	for _, body := range bodies[1:] {
		if got := string(Render(body)); got != want {
			t.Errorf("outer pipes changed the result for %q\nwant: %s\ngot:  %s", body, want, got)
		}
	}
}

func TestRender_TableCellsAreTrimmed(t *testing.T) {
	out := string(Render("|    a    |\tb\t|\n| --- | --- |\n|  1  |  2  |"))
	if !strings.Contains(out, "<th>a</th><th>b</th>") {
		t.Errorf("header cells not trimmed:\n%s", out)
	}
	if !strings.Contains(out, "<td>1</td><td>2</td>") {
		t.Errorf("body cells not trimmed:\n%s", out)
	}
}

func TestRender_TableShortRowRendersShort(t *testing.T) {
	// A row emits exactly the cells it has. No empty cells are invented to
	// square it off against the header, because inventing them is the one thing
	// in this construct that emitted more output than it was given input — see
	// markdown_tables.go's cost note. The row is still a row of the same table.
	out := string(Render("| a | b | c |\n| --- | --- | --- |\n| 1 |"))
	if !strings.Contains(out, "<tr><td>1</td></tr>") {
		t.Errorf("short row did not render with exactly its own cells:\n%s", out)
	}
	if !strings.Contains(out, "<table") {
		t.Errorf("a ragged row must not cost the table its tablehood:\n%s", out)
	}
}

func TestRender_TableExtraCellsAreDropped(t *testing.T) {
	out := string(Render("| a | b |\n| --- | --- |\n| 1 | 2 | 3 | 4 |"))
	if !strings.Contains(out, "<tr><td>1</td><td>2</td></tr>") {
		t.Errorf("extra cells were not dropped:\n%s", out)
	}
	if strings.Contains(out, ">3<") || strings.Contains(out, ">4<") {
		t.Errorf("a dropped cell reached the output:\n%s", out)
	}
}

func TestRender_TableEmptyCellsSurvive(t *testing.T) {
	out := string(Render("| a | b | c |\n| --- | --- | --- |\n| 1 |  | 3 |"))
	if !strings.Contains(out, "<tr><td>1</td><td></td><td>3</td></tr>") {
		t.Errorf("an interior empty cell was lost:\n%s", out)
	}
}

func TestRender_TableHeaderOnly(t *testing.T) {
	out := string(Render("| a | b |\n| --- | --- |"))
	want := `<table class="md-table"><thead><tr><th>a</th><th>b</th></tr></thead></table>`
	if out != want {
		t.Errorf("header-only table\nwant: %s\ngot:  %s", want, out)
	}
}

// --- boundaries -----------------------------------------------------------

func TestRender_TableEndsAtABlankLine(t *testing.T) {
	out := string(Render("| a |\n| --- |\n| 1 |\n\nafter | pipe"))
	if !strings.Contains(out, "</table><p>after | pipe</p>") {
		t.Errorf("table did not end at the blank line:\n%s", out)
	}
}

func TestRender_TableEndsAtTheFirstPipelessLine(t *testing.T) {
	out := string(Render("| a |\n| --- |\n| 1 |\nplain prose\n| 2 |"))
	if !strings.Contains(out, "<td>1</td>") {
		t.Errorf("body row lost:\n%s", out)
	}
	if strings.Contains(out, "<td>2</td>") {
		t.Errorf("the table ran past a pipe-less line:\n%s", out)
	}
	if !strings.Contains(out, "</table><p>plain prose | 2 |</p>") {
		t.Errorf("the tail did not become prose:\n%s", out)
	}
}

func TestRender_TableInterruptsAParagraph(t *testing.T) {
	out := string(Render("prose\n| a |\n| --- |\n| 1 |"))
	if !strings.HasPrefix(out, "<p>prose</p><table") {
		t.Errorf("table did not interrupt the open paragraph:\n%s", out)
	}
}

// --- placement (the container matrix) -------------------------------------

func TestRender_TableInBlockquote(t *testing.T) {
	out := string(Render("> | a | b |\n> | --- | --- |\n> | 1 | 2 |"))
	want := `<blockquote><table class="md-table"><thead><tr><th>a</th><th>b</th></tr></thead>` +
		`<tbody><tr><td>1</td><td>2</td></tr></tbody></table></blockquote>`
	if out != want {
		t.Errorf("table in blockquote\nwant: %s\ngot:  %s", want, out)
	}
}

func TestRender_TableInListItemIsLiteralText(t *testing.T) {
	out := string(Render("- item\n  | a | b |\n  | --- | --- |\n  | 1 | 2 |"))
	if strings.Contains(out, "<table") {
		t.Errorf("a table was recognized inside a list item:\n%s", out)
	}
	if !strings.Contains(out, "| a | b |") {
		t.Errorf("the pipe rows were not kept as literal item prose:\n%s", out)
	}
}

func TestRender_TableIsColumnZeroOnly(t *testing.T) {
	// Same guard as hr, heading and blockquote: an indented candidate with no
	// open list is ordinary prose, not a table.
	out := string(Render("  | a | b |\n  | --- | --- |\n  | 1 | 2 |"))
	if strings.Contains(out, "<table") {
		t.Errorf("an indented table was recognized:\n%s", out)
	}
}

// --- row splitting vs. the inline pass ------------------------------------

func TestRender_TableEscapedPipeDoesNotSplit(t *testing.T) {
	out := string(Render(`| a \| b | c |` + "\n| --- | --- |\n" + `| 1 \| 2 | 3 |`))
	if !strings.Contains(out, "<th>a | b</th><th>c</th>") {
		t.Errorf("escaped pipe split the header:\n%s", out)
	}
	if !strings.Contains(out, "<td>1 | 2</td><td>3</td>") {
		t.Errorf("escaped pipe split a body row:\n%s", out)
	}
}

func TestRender_TableDoubleBackslashIsARealBoundary(t *testing.T) {
	// "\\|" is an escaped backslash followed by a REAL cell boundary.
	out := string(Render(`| a \\| b |` + "\n| --- | --- |"))
	if !strings.Contains(out, `<th>a \</th><th>b</th>`) {
		t.Errorf(`"\\|" did not split, or lost its backslash:`+"\n%s", out)
	}
}

func TestRender_TableCodeSpanDoesNotProtectAPipe(t *testing.T) {
	// GFM, and gate 0 amendment A8: row splitting happens FIRST and
	// independently of inline parsing, so a code span does not protect a pipe.
	out := string(Render("| `a|b` | c |\n| --- | --- | --- |"))
	if strings.Contains(out, "<code>a|b</code>") {
		t.Errorf("a pipe inside a code span failed to split the row:\n%s", out)
	}
}

func TestRender_TableEscapedPipeSurvivesIntoACodeSpan(t *testing.T) {
	// The single documented exception to "escapes never process inside a code
	// span": "\|" is resolved at the row-splitting step, so it is the only way
	// to write a pipe inside a code span in a table cell.
	out := string(Render("| `a\\|b` |\n| --- |"))
	if !strings.Contains(out, "<code>a|b</code>") {
		t.Errorf(`"\|" did not reach the code span as a literal pipe:`+"\n%s", out)
	}
	if strings.Contains(out, `a\|b`) {
		t.Errorf("the escaping backslash leaked into the code span:\n%s", out)
	}
}

// --- cell content ---------------------------------------------------------

func TestRender_TableCellRunsTheInlinePass(t *testing.T) {
	body := "| **b** | `c` | [t](https://x.test/) | ~~s~~ | <https://y.test/> |\n" +
		"| --- | --- | --- | --- | --- |"
	out := string(Render(body))
	for _, want := range []string{
		"<strong>b</strong>",
		"<code>c</code>",
		`<a href="https://x.test/">t</a>`,
		"<del>s</del>",
		`<a href="https://y.test/">https://y.test/</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("cell inline pass: want %q in:\n%s", want, out)
		}
	}
}

func TestRender_TableCellHasNoBlockConstructs(t *testing.T) {
	body := "| - a | ### h | --- | ``` | > q | 1. n |\n| --- | --- | --- | --- | --- | --- |"
	out := string(Render(body))
	for _, forbidden := range []string{"<ul", "<ol", "<li", "<h3", "<hr", "<pre", "<blockquote"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("block construct %q rendered inside a cell:\n%s", forbidden, out)
		}
	}
	if !strings.Contains(out, "<th>- a</th>") {
		t.Errorf("a list marker did not stay literal in a cell:\n%s", out)
	}
}

func TestRender_TableCellTrailingBackslashIsLiteral(t *testing.T) {
	// A cell is single-line by construction, so the hard-break spelling has
	// nothing to break to: the backslash stays visible.
	out := string(Render(`| a\ | b |` + "\n| --- | --- |"))
	if strings.Contains(out, "<br>") {
		t.Errorf("a cell emitted a hard break:\n%s", out)
	}
	if !strings.Contains(out, `<th>a\</th>`) {
		t.Errorf("the dangling backslash was eaten:\n%s", out)
	}
}

func TestRender_TableCellsEscapeAuthorBytes(t *testing.T) {
	body := `| <script>alert(1)</script> | a & b | "q" | [x](javascript:alert(1)) |` + "\n" +
		"| --- | --- | --- | --- |"
	out := string(Render(body))
	if strings.Contains(out, "<script") {
		t.Errorf("a script tag survived a table cell:\n%s", out)
	}
	for _, want := range []string{"&lt;script&gt;", "a &amp; b", "&#34;q&#34;"} {
		if !strings.Contains(out, want) {
			t.Errorf("escaping: want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "href=\"javascript:") {
		t.Errorf("a rejected scheme became an anchor:\n%s", out)
	}
}

// --- output is bounded by input -------------------------------------------

// TestRender_NoWellFormedTableIsEverRefused IS THE POINT OF THIS CHANGE, and it
// is the test the two bounds it replaces could not have passed.
//
// Both of those bounds picked a ratio at which a WELL-FORMED table stopped
// being a table and re-rendered as a paragraph. Nothing in the spec authorises
// that: A9 makes a ragged body row a rendering decision INSIDE a table, never a
// reason to stop being one, and A24's degradation matrix gives a table exactly
// one path to prose — an alignment row whose arity does not match the header's,
// or none at all. So every such rule contradicted the spec by construction and
// merely relocated a cliff that real authors fall off. Measured against the byte
// ceiling, immediately before this change:
//
//	10 columns, 5 short rows, plain            rendered as a table
//	10 columns, 5 short rows, CENTRE-ALIGNED   rendered as a PARAGRAPH
//	 4 columns, 3 short rows, CENTRE-ALIGNED   rendered as a PARAGRAPH
//
// A four-column centre-aligned table with three ragged rows silently vanished.
// So the property is asserted as a UNIVERSAL over the whole two-dimensional
// space rather than at a handful of sampled shapes: every column count 1..40
// against every row count 1..40, plain and centre-aligned, with the raggedest
// body rows there are. Sampling is what let the previous rounds each claim a
// universal that did not hold; a sweep cannot make that mistake, because the
// cliff is somewhere in the interior and the interior is what is swept.
func TestRender_NoWellFormedTableIsEverRefused(t *testing.T) {
	for _, delim := range []struct {
		name string
		cell string
	}{
		{"plain", "-"},
		// Centre alignment is swept because it is the WORST case, not because
		// it is exotic: `<td class="md-col-center"></td>` is the widest cell
		// markup this package can write, so a ratio bound bit here first. Both
		// of the vanishing shapes above were centred.
		{"centred", ":-:"},
	} {
		for cols := 1; cols <= 40; cols++ {
			header := "|" + strings.Repeat("h|", cols)
			drow := "|" + strings.Repeat(delim.cell+"|", cols)
			for rows := 1; rows <= 40; rows++ {
				body := header + "\n" + drow + "\n" + strings.Repeat("|x|\n", rows)
				if out := string(Render(body)); !strings.Contains(out, "<table") {
					t.Fatalf("%s, %d columns x %d ragged one-cell rows was refused "+
						"and rendered as prose:\n%.200s", delim.name, cols, rows, out)
				}
			}
		}
	}
}

// TestRender_TableWithContentInEveryCellIsNeverRefused is the same universal
// restricted to DENSE tables — every cell present, every cell non-empty — and
// it is now trivially true, since nothing is refused at all. It is kept because
// the previous round asserted exactly this and it was the claim that failed:
// the bound it guarded could not refuse a dense table, but the whole family of
// ragged ones it could refuse went unswept. Keeping the narrow case next to the
// universal above records that the narrow case was never the hard one.
func TestRender_TableWithContentInEveryCellIsNeverRefused(t *testing.T) {
	for _, cols := range []int{1, 2, 3, 5, 10, 40, 200, 1000} {
		for _, rows := range []int{0, 1, 5, 20, 100, 500} {
			header := "|" + strings.Repeat("a|", cols)
			delim := "|" + strings.Repeat(":-:|", cols)
			row := "|" + strings.Repeat("1|", cols)
			body := header + "\n" + delim + "\n" + strings.Repeat(row+"\n", rows)
			if out := string(Render(body)); !strings.Contains(out, "<table") {
				t.Errorf("%d columns x %d rows, every cell centred and non-empty, was refused",
					cols, rows)
			}
		}
	}
}

func TestRender_OrdinaryTablesAreNeverRefused(t *testing.T) {
	// The shapes a human actually writes, including the exact ones the two
	// previous bounds refused.
	for name, body := range map[string]string{
		"the shape the cell bound refused": "|a|b|c|d|e|f|g|h|i|j|\n|-|-|-|-|-|-|-|-|-|-|\n" +
			strings.Repeat("|1|\n", 5),
		"the shape the byte ceiling refused": "|a|b|c|d|e|f|g|h|i|j|\n" +
			strings.Repeat("|:-:", 10) + "|\n" + strings.Repeat("|1|\n", 5),
		"four centred columns, three ragged rows": "|a|b|c|d|\n|:-:|:-:|:-:|:-:|\n" +
			strings.Repeat("|x|\n", 3),
		"eight columns, five short rows": "|" + strings.Repeat("h|", 8) + "\n|" +
			strings.Repeat("-|", 8) + "\n" + strings.Repeat("|x|\n", 5),
		"an ordinary forty-row table": "| Header 1 | Header 2 |\n| -------- | -------- |\n" +
			strings.Repeat("| value a  | value b  |\n", 40),
		"a thirty-column documentation table": "|" + strings.Repeat(" col |", 30) + "\n|" +
			strings.Repeat(" --- |", 30) + "\n" + strings.Repeat("|"+strings.Repeat(" v |", 30)+"\n", 10),
		"a sparse table written with spaces": "| a | b | c | d | e | f | g | h | i | j |\n" +
			strings.Repeat("|:-:", 10) + "|\n" +
			strings.Repeat("| 1 |   | 2 |   | 3 |   | 4 |   |   |   |\n", 30),
		"every cell empty but spaced": "| a | b | c |\n| - | - | - |\n" +
			strings.Repeat("|   |   |   |\n", 30),
		"every cell empty and unspaced": "|a|b|c|\n|:-:|:-:|:-:|\n" +
			strings.Repeat("||||\n", 30),
		"right-aligned numbers": "| item | qty |\n| ---- | ---:|\n" +
			strings.Repeat("| a | 1 |\n", 50),
		// The two shapes the cost sweep used to pin as REFUSED. They are here
		// so the change of verdict is stated in the unit suite too, not only in
		// a sweep whose wantTable flags moved.
		"the maximum-amplification shape": "|" + strings.Repeat("|", 4000) + "\n|" +
			strings.Repeat("-|", 4000) + "\n" + strings.Repeat("|\n", 200),
		"four hundred centred columns, five one-cell rows": strings.Repeat("|", 401) +
			strings.Repeat(" ", 395) + "\n|" + strings.Repeat(":-:|", 400) + "\n" +
			strings.Repeat("|\n", 5),
	} {
		if out := string(Render(body)); !strings.Contains(out, "<table") {
			t.Errorf("%s was refused:\n%.200s", name, out)
		}
	}
}

// widestCellBytes is the widest cell writeTable can emit, spelled through the
// very alignClass the renderer calls so it cannot drift from the markup. It is
// the numerator of the construct's intrinsic amplification ratio; the
// denominator is ONE source byte, the "|" that is the cheapest spelling of a
// cell there is.
var widestCellBytes = len("<td") + len(alignClass(alignCenter)) + len(">") + len("</td>")

// TestRender_TableOutputIsBoundedByItsInput measures the property that replaced
// the ceiling. There is no longer a rule that refuses a table, so the bound is
// no longer enforced anywhere — it is a CONSEQUENCE of writeTableRow emitting
// one cell per parsed cell, and this is what checks the consequence actually
// holds.
//
// THE ARGUMENT, in full. splitRow returns one cell per unescaped "|" in the
// source row, so a row of C emitted cells costs at least C source bytes and
// emits at most 9 bytes of <tr></tr> plus widestCellBytes per cell. Cell CONTENT
// only ever moves the ratio down: content costs source bytes of its own, and the
// widest expansion the inline pass applies to a single byte is html.EscapeString
// turning `"` into `&#34;`, five bytes from one, against the 31 that same cell's
// markup already costs. The delimiter row is pure source that emits nothing. So
// widestCellBytes is a strict supremum, approached only by a table of empty
// centre-aligned cells as its column count grows, and never reached.
//
// EVERY SHAPE IN THE SWEEP IS NOW A TABLE, which is itself the assertion that
// matters most here. The previous version of this test pinned three of its eight
// shapes as REFUSED, so for those three it measured the cost of rendering a
// paragraph while reading as though it measured a table. wantTable is kept as an
// explicit field rather than dropped, and every value is true, so that
// reintroducing any refusal path fails this test loudly instead of quietly
// turning a row back into a paragraph measurement.
func TestRender_TableOutputIsBoundedByItsInput(t *testing.T) {
	maxRatio := float64(widestCellBytes)
	for _, tc := range []struct {
		name      string
		wantTable bool
		gen       func(int) string
	}{
		{"table-dense-empty-cells", true, denseEmptyCellTable},
		{"table-many-tables-at-width", true, tableManyMediumTables},
		{"table-rows-many", true, func(n int) string { return tableOf(3, "| a | b | c |\n", n) }},
		{"table-ragged-rows", true, func(n int) string { return tableOf(4, "| x |\n", n) }},
		{"table-columns-many", true, wideTable},
		{"table-wide-header-short-rows", true, paddedTableAtCellBound},
		{"table-max-amplification", true, maxAmplificationTable},
		{"table-ragged-rows-one-cell", true, func(n int) string { return tableOf(24, "|\n", n) }},
	} {
		for _, size := range []int{16 << 10, 128 << 10} {
			body := tc.gen(size)
			out := string(Render(body))
			ratio := float64(len(out)) / float64(len(body))
			t.Logf("%s at %d bytes: %d out, %.2fx (table=%v)",
				tc.name, len(body), len(out), ratio, strings.Contains(out, "<table"))
			if got := strings.Contains(out, "<table"); got != tc.wantTable {
				t.Errorf("%s at %d bytes: rendered a table = %v, want %v — this shape is "+
					"measuring something other than what it was added to measure",
					tc.name, len(body), got, tc.wantTable)
			}
			if ratio > maxRatio {
				t.Errorf("%s at %d bytes: output %d bytes, ratio %.2fx exceeds the "+
					"widest-cell supremum of %.0fx — a table is emitting cells its "+
					"source does not contain",
					tc.name, len(body), len(out), ratio, maxRatio)
			}
		}
	}
}

// --- unit-level -----------------------------------------------------------

func TestSplitRow(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"| a | b |", []string{"a", "b"}},
		{"a | b", []string{"a", "b"}},
		{"| a | b", []string{"a", "b"}},
		{"a | b |", []string{"a", "b"}},
		{"||a||", []string{"", "a", ""}},
		{"|", []string{""}},
		{"a", []string{"a"}},
		{`a \| b | c`, []string{"a | b", "c"}},
		// Only the pipe escape is resolved here. "\\" is left for the inline
		// pass, which renders it as one literal backslash — splitRow must not
		// do that job twice.
		{`a \\| b`, []string{`a \\`, "b"}},
		{`a \\\| b`, []string{`a \\| b`}},
		// A code span does not protect a pipe (amendment A8), and the split
		// cells are then trimmed like any others.
		{"`a|b` | c", []string{"`a", "b`", "c"}},
	} {
		got := splitRow(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitRow(%q) = %q, want %q", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitRow(%q) = %q, want %q", tc.in, got, tc.want)
				break
			}
		}
	}
}

func TestDelimiterCellAlignment(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want cellAlign
		ok   bool
	}{
		{"---", alignNone, true},
		{"-", alignNone, true},
		{":---", alignLeft, true},
		{"---:", alignRight, true},
		{":---:", alignCenter, true},
		{":-:", alignCenter, true},
		{"", alignNone, false},
		{":", alignNone, false},
		{"::", alignNone, false},
		{"- -", alignNone, false},
		{"-x-", alignNone, false},
	} {
		got, ok := delimiterCell(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("delimiterCell(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
