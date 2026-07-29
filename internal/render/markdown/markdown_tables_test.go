package markdown

import (
	"strings"
	"testing"
)

// markdown_tables_test.go is phase C's unit suite: the GFM pipe table grammar,
// its placement in the container matrix, its two degradation paths, and the one
// deliberate exception to the escaping rules (see the "\|" tests below).
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

func TestRender_TableShortRowIsPadded(t *testing.T) {
	out := string(Render("| a | b | c |\n| --- | --- | --- |\n| 1 |"))
	if !strings.Contains(out, "<tr><td>1</td><td></td><td></td></tr>") {
		t.Errorf("short row not padded to the header's arity:\n%s", out)
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

// --- the sparse-padding refusal -------------------------------------------

func TestRender_SparsePaddingIsRefused(t *testing.T) {
	// Padding is the one part of this construct whose OUTPUT is not bounded by
	// its input: a header of N columns and M one-byte rows emits N*M cells from
	// N+M bytes. The bound is that a table may not emit more cells than its own
	// source has bytes; a table that would is not a table, and the whole
	// candidate block renders as ordinary paragraphs with its source bytes
	// unchanged.
	body := "|a|b|c|d|e|\n|-|-|-|-|-|\n" + strings.Repeat("|\n", 6)
	out := string(Render(body))
	if strings.Contains(out, "<table") {
		t.Errorf("a sparse table was emitted:\n%s", out)
	}
	if !strings.HasPrefix(out, "<p>|a|b|c|d|e|") {
		t.Errorf("the refused block did not fall through as prose:\n%s", out)
	}
}

func TestRender_SparsePaddingRefusalConsumesTheWholeBlock(t *testing.T) {
	// The refusal must CONSUME the block it refused. Falling through line by
	// line would let the delimiter row become the next candidate header, and a
	// document of alternating wide and narrow rows would then re-walk the same
	// extent once per line — quadratic.
	body := "|a|b|c|d|e|\n|-|-|-|-|-|\n" + strings.Repeat("|\n", 6) + "\ntail"
	out := string(Render(body))
	if n := strings.Count(out, "<p>"); n != 2 {
		t.Errorf("refused block did not render as exactly one paragraph plus the tail (got %d):\n%s", n, out)
	}
}

func TestRender_TableOutputIsBoundedByItsInput(t *testing.T) {
	// The bound, measured rather than argued. Every other construct in this
	// package emits output linear in its input; padding is the one that does
	// not, so this asserts the property directly on the two shapes that reach
	// the maximum ratio — including the one whose UNBOUNDED cell count would be
	// bytes^2/12 (~800 GB from a 1 MiB comment body).
	//
	// The ceiling is generous on purpose: it is a DoS bound, not a golden. The
	// point is that the ratio is a CONSTANT, which no quadratic can satisfy.
	const maxRatio = 12
	for _, sh := range []costShape{
		{name: "table-max-amplification", gen: maxAmplificationTable},
		{name: "table-ragged-rows-padded", gen: func(n int) string { return tableOf(4, "| x |\n", n) }},
		{name: "table-ragged-rows-refused", gen: func(n int) string { return tableOf(24, "|\n", n) }},
	} {
		for _, size := range []int{16 << 10, 128 << 10} {
			body := sh.gen(size)
			out := string(Render(body))
			if ratio := float64(len(out)) / float64(len(body)); ratio > maxRatio {
				t.Errorf("%s at %d bytes: output %d bytes, ratio %.1fx exceeds %dx",
					sh.name, len(body), len(out), ratio, maxRatio)
			}
		}
	}
}

func TestRender_DenseTableIsStillATable(t *testing.T) {
	// The bound must not refuse an ordinary table. A real one carries far more
	// than one byte per emitted cell.
	body := "| Header 1 | Header 2 |\n| -------- | -------- |\n" +
		strings.Repeat("| value a  | value b  |\n", 40)
	if out := string(Render(body)); !strings.Contains(out, "<table") {
		t.Errorf("an ordinary 40-row table was refused:\n%s", out[:min(len(out), 200)])
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
