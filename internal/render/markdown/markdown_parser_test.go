package markdown

import (
	"strings"
	"testing"
)

// This file is P1's contract: the three parser-core behaviours frozen at
// gate 1 — fence recognition inside the line scanner (amendment A1),
// backslash escapes resolved inside the inline scan (A7), and
// backtick-run-matched code spans — plus the fall-through rule for every
// construct P1 touches (A24) and the L3 regression matrix that guards the
// list-marker/emphasis-delimiter boundary P2 will build on.

// --- (1) fences meet the scanner in document order -----------------------

// TestRender_FenceInsideOrderedListItem is amendment A1's named failure.
// Before P1 this rendered as two <ol>s with the numbering restarted and the
// code ejected to top level; after P1 it is one list of two items with the
// code block inside the first item.
func TestRender_FenceInsideOrderedListItem(t *testing.T) {
	body := "1. Install it:\n   ```\n   npm install dossierx\n   ```\n2. Then run it."
	got := string(Render(body))
	want := "<ol><li>Install it:<pre><code>npm install dossierx\n</code></pre></li><li>Then run it.</li></ol>"
	if got != want {
		t.Errorf("fence under an ordered-list item:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

func TestRender_FenceInsideUnorderedListItem(t *testing.T) {
	body := "- Install it:\n  ```\n  npm install dossierx\n  ```\n- Then run it."
	got := string(Render(body))
	want := "<ul><li>Install it:<pre><code>npm install dossierx\n</code></pre></li><li>Then run it.</li></ul>"
	if got != want {
		t.Errorf("fence under an unordered-list item:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

// TestRender_FenceInsideNestedListItem pins that the fence attaches to the
// deepest OPEN item, not always the top-level one.
func TestRender_FenceInsideNestedListItem(t *testing.T) {
	body := "- Top\n  - Nested\n    ```\n    code\n    ```\n- Second top"
	got := string(Render(body))
	want := "<ul><li>Top<ul><li>Nested<pre><code>code\n</code></pre></li></ul></li><li>Second top</li></ul>"
	if got != want {
		t.Errorf("fence under a nested item:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

// TestRender_FenceContinuationAfterFenceStaysInSameList pins that the fence
// no longer resets the scanner's block state: the item text written AFTER a
// fence still folds into the same item, and numbering never restarts.
func TestRender_FenceDoesNotSplitOrRenumber(t *testing.T) {
	body := "1. One\n   ```\n   a\n   ```\n2. Two\n   ```\n   b\n   ```\n3. Three"
	got := string(Render(body))
	if strings.Count(got, "<ol>") != 1 {
		t.Errorf("expected exactly one <ol>, got %d:\n%s", strings.Count(got, "<ol>"), got)
	}
	if strings.Count(got, "<li>") != 3 {
		t.Errorf("expected 3 items, got %d:\n%s", strings.Count(got, "<li>"), got)
	}
	if strings.Count(got, "<pre><code>") != 2 {
		t.Errorf("expected 2 fences, got %d:\n%s", strings.Count(got, "<pre><code>"), got)
	}
	assertTagBalance(t, got)
}

// TestRender_ProseAfterFenceStaysAfterIt: an item's content is an ordered
// block sequence, so prose written after a fence renders after it instead of
// folding backwards in front of it.
func TestRender_ProseAfterFenceStaysAfterIt(t *testing.T) {
	body := "1. Install it:\n   ```\n   npm i\n   ```\n   Then check the version."
	got := string(Render(body))
	want := "<ol><li>Install it:<pre><code>npm i\n</code></pre>Then check the version.</li></ol>"
	if got != want {
		t.Errorf("prose after a fence:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

// TestRender_FenceAtColumnZeroClosesAnOpenList is the documented other half
// of the container rule: an UNindented fence is a top-level block, so it
// closes any open list exactly as it did before P1.
func TestRender_FenceAtColumnZeroClosesAnOpenList(t *testing.T) {
	body := "- item one\n```\ncode here\n```\n- item two"
	got := string(Render(body))
	want := "<ul><li>item one</li></ul><pre><code>code here\n</code></pre><ul><li>item two</li></ul>"
	if got != want {
		t.Errorf("column-zero fence:\n got: %s\nwant: %s", got, want)
	}
}

// --- (1b) the loose-list rule: a blank line does not end a list ----------
//
// A1's repair originally only covered a fence written DIRECTLY under an item
// line. The shape essentially all real documentation uses — item text, a
// blank line, then the indented fence — still split the list, ejected the
// code to top level and restarted the numbering, because renderBlocks
// treated a blank line as an unconditional flush of the open list. These
// pin the fixed rule and, just as importantly, the two cases that must still
// end a list.

// TestRender_BlankLineBeforeIndentedFenceKeepsOrderedList is the named
// failure: one <ol>, two items, the <pre> inside item one, numbering never
// restarting.
func TestRender_BlankLineBeforeIndentedFenceKeepsOrderedList(t *testing.T) {
	body := "1. Install it:\n\n   ```sh\n   go install ./...\n   ```\n\n2. Then run it.\n"
	got := string(Render(body))
	want := "<ol><li><p>Install it:</p><pre><code>go install ./...\n</code></pre></li><li><p>Then run it.</p></li></ol>"
	if got != want {
		t.Errorf("blank line before an indented fence:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

func TestRender_BlankLineBeforeIndentedFenceKeepsUnorderedList(t *testing.T) {
	body := "- Install it:\n\n  ```sh\n  go install ./...\n  ```\n\n- Then run it.\n"
	got := string(Render(body))
	want := "<ul><li><p>Install it:</p><pre><code>go install ./...\n</code></pre></li><li><p>Then run it.</p></li></ul>"
	if got != want {
		t.Errorf("blank line before an indented fence:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

// TestRender_BlankLineBeforeNestedItemAndItsFence: the rule holds at depth —
// a blank line before a nested item, and another before a fence indented
// under that nested item, and the fence still lands in the nested item.
func TestRender_BlankLineBeforeNestedItemAndItsFence(t *testing.T) {
	body := "- Top\n\n  - Nested\n\n    ```\n    code\n    ```\n\n- Second top\n"
	got := string(Render(body))
	want := "<ul><li><p>Top</p><ul><li><p>Nested</p><pre><code>code\n</code></pre></li></ul></li><li><p>Second top</p></li></ul>"
	if got != want {
		t.Errorf("blank lines around a nested item's fence:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

// TestRender_BlankLineNeverRenumbers is the numbering half stated on its own,
// across several blank-separated items each carrying a fence.
func TestRender_BlankLineNeverRenumbers(t *testing.T) {
	body := "1. One\n\n   ```\n   a\n   ```\n\n2. Two\n\n   ```\n   b\n   ```\n\n3. Three\n"
	got := string(Render(body))
	if n := strings.Count(got, "<ol>"); n != 1 {
		t.Errorf("expected exactly one <ol>, got %d:\n%s", n, got)
	}
	if n := strings.Count(got, "<li>"); n != 3 {
		t.Errorf("expected 3 items, got %d:\n%s", n, got)
	}
	if n := strings.Count(got, "<pre><code>"); n != 2 {
		t.Errorf("expected 2 fences, got %d:\n%s", n, got)
	}
	assertTagBalance(t, got)
}

// TestRender_BlankLineBetweenPlainItemsKeepsOneList: a blank line between two
// plain items is an ordinary loose list, not two lists.
func TestRender_BlankLineBetweenPlainItemsKeepsOneList(t *testing.T) {
	cases := []struct{ in, want string }{
		{"- a\n\n- b\n", "<ul><li><p>a</p></li><li><p>b</p></li></ul>"},
		{"1. a\n\n2. b\n", "<ol><li><p>a</p></li><li><p>b</p></li></ol>"},
	}
	for _, tc := range cases {
		if got := string(Render(tc.in)); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRender_BlankLineThenIndentedProseContinuesItem: an indented non-marker
// line after a blank line is item content, folded into the open item's prose
// run exactly as an unblanked continuation line is.
func TestRender_BlankLineThenIndentedProseContinuesItem(t *testing.T) {
	body := "1. one\n\n   continued prose\n\n2. two\n"
	got := string(Render(body))
	want := "<ol><li><p>one continued prose</p></li><li><p>two</p></li></ol>"
	if got != want {
		t.Errorf("blank line before a continuation:\n got: %s\nwant: %s", got, want)
	}
}

// TestRender_BlankLineThenDedentedProseReturnsToTopItem: a line indented back
// out of a nested item, but still into the top item's content, belongs to the
// TOP item — and the list stays open either way.
func TestRender_BlankLineThenDedentedProseReturnsToTopItem(t *testing.T) {
	body := "1. Step one\n   - sub a\n\n   More about step one.\n\n2. Step two\n"
	got := string(Render(body))
	want := "<ol><li><p>Step one</p><ul><li>sub a</li></ul><p>More about step one.</p></li><li><p>Step two</p></li></ol>"
	if got != want {
		t.Errorf("dedented continuation after a blank line:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

// TestRender_BlankLineThenColumnZeroProseStillEndsList is the non-regression
// half: a blank line followed by a NON-indented, NON-marker line still ends
// the list, exactly as before.
func TestRender_BlankLineThenColumnZeroProseStillEndsList(t *testing.T) {
	body := "- item one\n\nplain prose\n"
	got := string(Render(body))
	want := "<ul><li>item one</li></ul><p>plain prose</p>"
	if got != want {
		t.Errorf("blank line then column-zero prose:\n got: %s\nwant: %s", got, want)
	}
}

// TestRender_BlankLineThenColumnZeroFenceIsStillTopLevel is the other
// non-regression half, and P1's stated rule: an UNindented fence is a
// top-level block whether or not a blank line precedes it, so it still closes
// the list.
func TestRender_BlankLineThenColumnZeroFenceIsStillTopLevel(t *testing.T) {
	body := "- item one\n\n```\ncode here\n```\n\n- item two\n"
	got := string(Render(body))
	want := "<ul><li>item one</li></ul><pre><code>code here\n</code></pre><ul><li>item two</li></ul>"
	if got != want {
		t.Errorf("blank line then column-zero fence:\n got: %s\nwant: %s", got, want)
	}
}

// TestRender_FenceOpenerIsLineAnchored: the pre-P1 bodyFence regex matched
// the literal substring ``` anywhere in the body, so a mid-line run opened a
// block-level fence and swallowed the surrounding prose. A fence opener is
// now a LINE that begins (after optional indentation) with the run.
func TestRender_FenceOpenerIsLineAnchored(t *testing.T) {
	got := string(Render("Use ```x``` inline."))
	want := "<p>Use <code>x</code> inline.</p>"
	if got != want {
		t.Errorf("mid-line backtick run must not open a block fence:\n got: %s\nwant: %s", got, want)
	}
}

// TestRender_FenceInfoStringWithBacktickIsNotAFence keeps the line-anchored
// rule honest for a one-line ```x``` written at column zero: an opening
// fence's info string may not contain a backtick, so this stays inline.
func TestRender_FenceInfoStringWithBacktickIsNotAFence(t *testing.T) {
	got := string(Render("```x``` and more"))
	if strings.Contains(got, "<pre>") {
		t.Errorf("a backtick in the info string must not open a block fence: %s", got)
	}
	if !strings.Contains(got, "<code>x</code>") {
		t.Errorf("expected an inline code span: %s", got)
	}
}

// TestRender_EscapedFenceMarkerDoesNotOpenAFence is A1's escape-blindness
// half: a backslash before the run means the line no longer BEGINS with a
// backtick run, so the fence cannot open. Structural, not a special case.
func TestRender_EscapedFenceMarkerDoesNotOpenAFence(t *testing.T) {
	body := "\\```\nnot code\n\\```"
	got := string(Render(body))
	if strings.Contains(got, "<pre>") {
		t.Errorf("an escaped fence marker must not open a fence: %s", got)
	}
	if !strings.Contains(got, "not code") {
		t.Errorf("content dropped: %s", got)
	}
}

// TestRender_UnclosedFenceInsideListItemFallsThrough: an unclosed fence is
// never a fence — the opening line falls through to ordinary handling for
// the container it sits in, and no content is dropped.
func TestRender_UnclosedFenceInsideListItemFallsThrough(t *testing.T) {
	body := "1. One:\n   ```\n   never closed"
	got := string(Render(body))
	if strings.Contains(got, "<pre>") {
		t.Errorf("unclosed fence must not produce a <pre>: %s", got)
	}
	if !strings.Contains(got, "never closed") {
		t.Errorf("unclosed-fence content dropped: %s", got)
	}
	assertTagBalance(t, got)
}

// TestRender_FenceContentIsNeverInlineProcessed pins the escaping boundary
// on the fence side: content is raw source, HTML-escaped once, with no
// escape resolution and no code-span parsing.
func TestRender_FenceContentIsNeverInlineProcessed(t *testing.T) {
	body := "```\na\\b `c` \\* [d](http://x)\n```"
	got := string(Render(body))
	want := "<pre><code>a\\b `c` \\* [d](http://x)\n</code></pre>"
	if got != want {
		t.Errorf("fence content must be verbatim:\n got: %s\nwant: %s", got, want)
	}
}

// TestRender_FenceIndentStrippedToOpenerIndent pins the de-indent rule for a
// fence inside a container.
func TestRender_FenceIndentStrippedToOpenerIndent(t *testing.T) {
	body := "- x:\n  ```\n  a\n    b\n  ```"
	got := string(Render(body))
	if !strings.Contains(got, "<pre><code>a\n  b\n</code></pre>") {
		t.Errorf("opener indent should be stripped, deeper indent kept: %s", got)
	}
}

// --- (2) backslash escapes ----------------------------------------------

// TestRenderInline_EscapableSet walks the whole closed set from the spec.
func TestRenderInline_EscapableSet(t *testing.T) {
	cases := []struct{ in, want string }{
		{`\\`, `\`},
		{"\\`", "`"},
		{`\*`, `*`},
		{`\_`, `_`},
		{`\~`, `~`},
		{`\[`, `[`},
		{`\]`, `]`},
		{`\(`, `(`},
		{`\)`, `)`},
		{`\>`, `&gt;`},
		{`\#`, `#`},
		{`\-`, `-`},
		{`\.`, `.`},
		{`\!`, `!`},
		{`\|`, `|`},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_NonEscapableFallThrough: anything outside the closed set
// leaves the backslash as a literal and the scan resumes at the next byte.
// < and & are deliberately OUTSIDE the set (A7), so they still escape.
func TestRenderInline_NonEscapableFallThrough(t *testing.T) {
	cases := []struct{ in, want string }{
		{`\<`, `\&lt;`},
		{`\&`, `\&amp;`},
		{`\a`, `\a`},
		{`\ `, `\ `},
		{`\"`, `\&#34;`},
		{`\`, `\`},    // dangling backslash at end of input
		{`a\`, `a\`},  // dangling backslash after text
		{`\\\`, `\\`}, // escaped backslash, then a dangling one
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_EscapeDefusesConstructOpeners: an escaped byte can never
// open, close or delimit a construct.
func TestRenderInline_EscapeDefusesConstructOpeners(t *testing.T) {
	cases := []struct{ in, want string }{
		{"\\`not code`", "`not code`"},
		{`\[a](http://x)`, `[a](http://x)`},
		{`\*\*bold\*\*`, `**bold**`},
		{"``a\\`b``", "<code>a\\`b</code>"}, // no escape processing inside a span
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_NoEscapeProcessingInsideCodeSpan is the spec's named
// silent-corruption case: `a\b` must not lose a character.
func TestRenderInline_NoEscapeProcessingInsideCodeSpan(t *testing.T) {
	if got := string(RenderInline("`a\\b`")); got != `<code>a\b</code>` {
		t.Errorf("escape processed inside a code span: got %q", got)
	}
	if got := string(RenderInline("`\\*x\\*`")); got != `<code>\*x\*</code>` {
		t.Errorf("escape processed inside a code span: got %q", got)
	}
}

// TestRender_EscapeDefusesBlockMarkers: the block scanner matches markers on
// the raw line, so a leading backslash means the marker regex cannot match
// and the backslash is consumed by the inline pass.
func TestRender_EscapeDefusesBlockMarkers(t *testing.T) {
	cases := []struct{ in, want string }{
		{`\- not a list`, `<p>- not a list</p>`},
		{`\* not a list`, `<p>* not a list</p>`},
		{`1\. not a list`, `<p>1. not a list</p>`},
		{`\> not a quote`, `<p>&gt; not a quote</p>`},
		{`\# not a heading`, `<p># not a heading</p>`},
	}
	for _, tc := range cases {
		if got := string(Render(tc.in)); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_EscapedOutputNeverReEntersTheParser: the byte produced by
// an escape is written straight to the output builder, so it can never be
// re-scanned. A body that escapes a delimiter next to a live one proves the
// escaped copy did not participate in matching.
func TestRenderInline_EscapedOutputNeverReEntersTheParser(t *testing.T) {
	// The escaped backtick must not pair with the real one that follows.
	if got := string(RenderInline("\\``x`")); got != "`<code>x</code>" {
		t.Errorf("escaped backtick participated in span matching: got %q", got)
	}
	// An escaped "[" must not open a link with the later "](...)".
	if got := string(RenderInline(`\[a](http://x)`)); strings.Contains(got, "<a ") {
		t.Errorf("escaped bracket opened a link: got %q", got)
	}
}

// --- (3) backtick-run code spans ----------------------------------------

func TestRenderInline_BacktickRuns(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"single unchanged", "`a`", "<code>a</code>"},
		{"double holds a literal backtick", "``a`b``", "<code>a`b</code>"},
		{"triple", "```x```", "<code>x</code>"},
		{"run length must match exactly", "`` a ``` b ``", "<code> a ``` b </code>"},
		{"unclosed single falls through", "`a", "`a"},
		{"unclosed double falls through", "``a`", "``a`"},
		{"unclosed run resumes scanning", "``a [d](http://x)", "``a <a href=\"http://x\">d</a>"},
		{"empty span", "``` ```", "<code> </code>"},
		{"escaping still applies inside", "`<b>&`", "<code>&lt;b&gt;&amp;</code>"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := string(RenderInline(tc.in)); got != tc.want {
				t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// --- L3 regression matrix (guards for P2, which P1 must not pre-empt) ----

// TestRender_L3DelimiterMatrix pins that P1 changes nothing about the
// list-marker / emphasis-delimiter boundary: unorderedMarker's required
// whitespace is what stops **bold** parsing as a list, and no emphasis is
// implemented yet.
func TestRender_L3DelimiterMatrix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"**b**", "<p>**b**</p>"},
		{"* item", "<ul><li>item</li></ul>"},
		{"*italic* rest", "<p>*italic* rest</p>"},
		{"- *italic* item", "<ul><li>*italic* item</li></ul>"},
		{"*not a list*item", "<p>*not a list*item</p>"},
		{"governed_by and rests_on", "<p>governed_by and rests_on</p>"},
		{"_leading, trailing_", "<p>_leading, trailing_</p>"},
	}
	for _, tc := range cases {
		if got := string(Render(tc.in)); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- Render / RenderInline parity ---------------------------------------

// TestRenderInlineParity: both entry points run the SAME inline pass, so
// every construct P1 owns behaves identically in a paragraph and in a table
// cell. Render adds only the block wrapper.
func TestRenderInlineParity(t *testing.T) {
	for _, in := range []string{
		`\*x\*`,
		"``a`b``",
		"`a\\b`",
		`\<script>`,
		"[d](http://x)",
		"[d](javascript:alert(1))",
	} {
		inline := string(RenderInline(in))
		block := string(Render(in))
		if block != "<p>"+inline+"</p>" {
			t.Errorf("parity broken for %q:\n Render:       %s\n RenderInline: %s", in, block, inline)
		}
	}
}

// TestRender_NoUnescapedAuthorBytes is the escaping-boundary smoke test for
// the two new constructs: every hostile byte still comes out escaped.
func TestRender_NoUnescapedAuthorBytes(t *testing.T) {
	bodies := []string{
		`\<script>alert(1)</script>`,
		"``<script>``",
		"```\n<script>\n```",
		"- x:\n  ```\n  <script>\n  ```",
		`\&\<\>`,
	}
	for _, body := range bodies {
		out := string(Render(body))
		if strings.Contains(out, "<script>") {
			t.Errorf("unescaped <script> leaked from %q: %s", body, out)
		}
	}
}
