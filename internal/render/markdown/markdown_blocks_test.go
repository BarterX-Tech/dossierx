package markdown

import (
	"strings"
	"testing"
)

// This file pins the phase-B block constructs at the edges the golden fixtures
// under testdata/markdown-cases do not reach: the REJECTIONS (what is
// deliberately not a heading, not a rule, not a task item, not a quote), the
// escape interactions, and the two places where a construct's ceiling had to be
// chosen rather than inherited. Every rule that has a normal-shaped example
// lives in a fixture instead; these are the corners.

func renderCases(t *testing.T, name string, cases []struct{ in, want string }) {
	t.Helper()
	for _, tc := range cases {
		got := string(Render(tc.in))
		if got != tc.want {
			t.Errorf("%s: Render(%q)\n got: %s\nwant: %s", name, tc.in, got, tc.want)
		}
		assertTagBalance(t, got)
	}
}

// --- thematic breaks ------------------------------------------------------

// TestRender_ThematicBreakRejections pins the one-spelling rule from the
// outside: everything that LOOKS like a rule in some other Markdown dialect
// but is not one here.
func TestRender_ThematicBreakRejections(t *testing.T) {
	renderCases(t, "hr rejections", []struct{ in, want string }{
		{"--\n", "<p>--</p>"},                                             // two dashes is not a rule
		{"- - -\n", "<ul><li>- -</li></ul>"},                              // spaced dashes are a list item
		{"---x\n", "<p>---x</p>"},                                         // a rule line holds dashes only
		{"x---\n", "<p>x---</p>"},                                         // ...and starts at the line start
		{"___\n", "<p>___</p>"},                                           // underscores are not a rule
		{"\\---\n", "<p>---</p>"},                                         // an escape defuses the marker
		{"Prose\n  ---\n", "<p>Prose ---</p>"},                            // indented: not column zero
		{"> Quoted\n---\n", "<blockquote><p>Quoted</p></blockquote><hr>"}, // ends the quote
	})
}

// --- ATX headings ---------------------------------------------------------

// TestRender_HeadingRejections pins the reserved levels and the marker
// grammar. Levels 1 and 2 are viewer chrome: they must come out VISIBLE, so an
// author can see the heading did not happen, and so markdown-sanity has
// something to report.
func TestRender_HeadingRejections(t *testing.T) {
	renderCases(t, "heading rejections", []struct{ in, want string }{
		{"# One\n", "<p># One</p>"},
		{"## Two\n", "<p>## Two</p>"},
		{"####### Seven\n", "<p>####### Seven</p>"},
		{"###x\n", "<p>###x</p>"},                       // a hash run needs a space
		{"\\### x\n", "<p>### x</p>"},                   // an escape defuses the marker
		{"  ### x\n", "<p>### x</p>"},                   // indented: not column zero
		{"a\n  ### x\n", "<p>a ### x</p>"},              // ...and folds into the paragraph
		{"- a\n  ### x\n", "<ul><li>a ### x</li></ul>"}, // container matrix: literal in an item
	})
}

// TestRender_HeadingClosingSequenceEdges pins the CommonMark ATX closing rule
// at the two shapes that are easy to get backwards.
func TestRender_HeadingClosingSequenceEdges(t *testing.T) {
	renderCases(t, "heading closing sequence", []struct{ in, want string }{
		{"### ###\n", "<h3></h3>"},     // all hashes: an empty heading
		{"### a ####\n", "<h3>a</h3>"}, // a longer closing run is still a closing run
		{"### a#\n", "<h3>a#</h3>"},    // no space before it: part of the word
		{"###\n", "<h3></h3>"},         // a bare marker is an empty heading
	})
}

// TestRender_HeadingEscapesAuthorBytes is the escaping boundary at the newest
// emission point: heading text is author-controlled and reaches the output
// only through renderInline.
func TestRender_HeadingEscapesAuthorBytes(t *testing.T) {
	got := string(Render("### <script>alert(1)</script> & \"q\"\n"))
	if strings.Contains(got, "<script>") {
		t.Fatalf("unescaped markup leaked into a heading: %s", got)
	}
	want := "<h3>&lt;script&gt;alert(1)&lt;/script&gt; &amp; &#34;q&#34;</h3>"
	if got != want {
		t.Errorf("heading escaping:\n got: %s\nwant: %s", got, want)
	}
}

// --- blockquotes ----------------------------------------------------------

// TestRender_BlockquoteDepthGuard pins the one-level cap as the STRUCTURAL
// guarantee it is: the recursion runs with quote recognition off, so no depth
// of ">" can ever produce a second <blockquote>.
func TestRender_BlockquoteDepthGuard(t *testing.T) {
	for _, body := range []string{
		"> > x\n",
		">> x\n",
		"> > > x\n",
		">>>> x\n",
		"> a\n> > b\n> > > c\n",
	} {
		got := string(Render(body))
		if n := strings.Count(got, "<blockquote>"); n != 1 {
			t.Errorf("Render(%q) produced %d blockquotes, want exactly 1:\n%s", body, n, got)
		}
		assertTagBalance(t, got)
	}
}

func TestRender_BlockquoteEdges(t *testing.T) {
	renderCases(t, "blockquote edges", []struct{ in, want string }{
		// The prefix is ">" plus ONE optional space, so ">x" is a quote too.
		{">x\n", "<blockquote><p>x</p></blockquote>"},
		// A quote interrupts an open paragraph.
		{"Prose\n> quoted\n", "<p>Prose</p><blockquote><p>quoted</p></blockquote>"},
		// No lazy continuation: the quote ends at the first unprefixed line.
		{"> a\nb\n", "<blockquote><p>a</p></blockquote><p>b</p>"},
		// A lone ">" is a paragraph break inside the quote, not a terminator.
		{"> a\n>\n> b\n", "<blockquote><p>a</p><p>b</p></blockquote>"},
		// A quote closes an open list first.
		{"- a\n> q\n", "<ul><li>a</li></ul><blockquote><p>q</p></blockquote>"},
		// Container matrix: indented under an item, ">" is item prose.
		{"- a\n  > not a quote\n", "<ul><li>a &gt; not a quote</li></ul>"},
		// An escape defuses the marker at column zero.
		{"\\> not a quote\n", "<p>&gt; not a quote</p>"},
	})
}

func TestRender_BlockquoteEscapesAuthorBytes(t *testing.T) {
	got := string(Render("> <script>alert(1)</script>\n"))
	if strings.Contains(got, "<script>") {
		t.Fatalf("unescaped markup leaked into a blockquote: %s", got)
	}
	if want := "<blockquote><p>&lt;script&gt;alert(1)&lt;/script&gt;</p></blockquote>"; got != want {
		t.Errorf("blockquote escaping:\n got: %s\nwant: %s", got, want)
	}
}

// --- list nesting ---------------------------------------------------------

// TestRender_ListIndentIsMeasuredInColumns is the tab-width pin. leadingSpaces
// counts a tab as one BYTE, which is right for slicing a line and wrong for
// every indent comparison; indentWidth counts it as four COLUMNS. A tab and
// four spaces must therefore produce identical structure.
func TestRender_ListIndentIsMeasuredInColumns(t *testing.T) {
	tabbed := string(Render("- a\n\t- b\n- c\n"))
	spaced := string(Render("- a\n    - b\n- c\n"))
	if tabbed != spaced {
		t.Errorf("a tab must measure four columns:\n tab: %s\nspace: %s", tabbed, spaced)
	}
	if want := "<ul><li>a<ul><li>b</li></ul></li><li>c</li></ul>"; tabbed != want {
		t.Errorf("tab-indented nesting:\n got: %s\nwant: %s", tabbed, want)
	}
	// The same for a continuation line.
	if got, want := string(Render("- x\n\tcontinued\n")), "<ul><li>x continued</li></ul>"; got != want {
		t.Errorf("tab-indented continuation:\n got: %s\nwant: %s", got, want)
	}
}

// TestRender_ListNestingIsUnbounded walks the depth stack past any fixed cap:
// N levels in must be N <ul>s out, for N well past the old limit of one.
func TestRender_ListNestingIsUnbounded(t *testing.T) {
	for _, depth := range []int{1, 2, 3, 8, 40} {
		var body strings.Builder
		for d := 0; d < depth; d++ {
			body.WriteString(strings.Repeat(" ", 2*d))
			body.WriteString("- a\n")
		}
		got := string(Render(body.String()))
		if n := strings.Count(got, "<ul>"); n != depth {
			t.Errorf("depth %d: got %d <ul>, want %d:\n%s", depth, n, depth, got)
		}
		if n := strings.Count(got, "<li>"); n != depth {
			t.Errorf("depth %d: got %d <li>, want %d:\n%s", depth, n, depth, got)
		}
		assertTagBalance(t, got)
	}
}

// TestRender_IndentedMarkerWithNoOpenListIsProse pins the one indentation case
// that is NOT a list, unchanged from before the depth stack: a marker indented
// under a paragraph, with no list open for it to join, stays prose. Otherwise
// any indented dash inside a wrapped sentence would silently become a list.
func TestRender_IndentedMarkerWithNoOpenListIsProse(t *testing.T) {
	renderCases(t, "indented marker, no list", []struct{ in, want string }{
		{"  - a\n", "<p>- a</p>"},
		{"Prose\n  - a\n", "<p>Prose - a</p>"},
	})
}

// --- task items -----------------------------------------------------------

func TestRender_TaskItemRejections(t *testing.T) {
	renderCases(t, "task rejections", []struct{ in, want string }{
		// Pinned ceiling: the checkbox is an UNORDERED-item construct only.
		{"1. [ ] x\n", "<ol><li>[ ] x</li></ol>"},
		// The bracket contents must be exactly one space, x or X.
		{"- [y] x\n", "<ul><li>[y] x</li></ul>"},
		{"- [] x\n", "<ul><li>[] x</li></ul>"},
		{"- [ x] y\n", "<ul><li>[ x] y</li></ul>"},
		// ...and must be followed by whitespace or end the item.
		{"- [x]y\n", "<ul><li>[x]y</li></ul>"},
		{"- [ ]\n", `<ul><li class="task"><input type="checkbox" disabled> </li></ul>`},
		// An escape defuses it with no special case.
		{"- \\[x] escaped\n", "<ul><li>[x] escaped</li></ul>"},
		// Upper-case X is checked, per GFM.
		{"- [X] done\n", `<ul><li class="task"><input type="checkbox" disabled checked> done</li></ul>`},
	})
}

func TestRender_TaskItemEscapesAuthorBytes(t *testing.T) {
	got := string(Render("- [x] <img onerror=alert(1)>\n"))
	if strings.Contains(got, "<img") {
		t.Fatalf("unescaped markup leaked into a task item: %s", got)
	}
	want := `<ul><li class="task"><input type="checkbox" disabled checked> &lt;img onerror=alert(1)&gt;</li></ul>`
	if got != want {
		t.Errorf("task item escaping:\n got: %s\nwant: %s", got, want)
	}
}

// --- ol start -------------------------------------------------------------

func TestRender_OrderedListStart(t *testing.T) {
	renderCases(t, "ol start", []struct{ in, want string }{
		// Only the FIRST item's number decides; the rest are ignored, exactly
		// as CommonMark specifies.
		{"1. a\n2. b\n", "<ol><li>a</li><li>b</li></ol>"},
		{"1. a\n7. b\n", "<ol><li>a</li><li>b</li></ol>"},
		{"5. a\n6. b\n", `<ol start="5"><li>a</li><li>b</li></ol>`},
		{"0. a\n", `<ol start="0"><li>a</li></ol>`},
		{"007. a\n", `<ol start="7"><li>a</li></ol>`},
		// A number too large to represent emits no attribute rather than a
		// truncated or negative one: start is a rendering nicety, and a
		// wrong number would be worse than none.
		{"99999999999999999999. a\n", "<ol><li>a</li></ol>"},
	})
}

// --- hard line breaks -----------------------------------------------------

// TestRender_SoftBreakStillJoinsWithASpace is the non-regression half of the
// hard-break work: an ordinary wrapped line is still one space, and a code
// span still spans a SOFT break. Only a HARD break bounds a span.
func TestRender_SoftBreakStillJoinsWithASpace(t *testing.T) {
	renderCases(t, "soft breaks", []struct{ in, want string }{
		{"a\nb\n", "<p>a b</p>"},
		{"a `co\nde` b\n", "<p>a <code>co de</code> b</p>"},
		{"- a\n  b\n", "<ul><li>a b</li></ul>"},
	})
}

func TestRender_HardBreakEdges(t *testing.T) {
	renderCases(t, "hard breaks", []struct{ in, want string }{
		// Escapes resolve first: an even run of trailing backslashes is not
		// a break, an odd one is.
		{"a\\\nb\n", "<p>a<br>b</p>"},
		{"a\\\\\nb\n", "<p>a\\ b</p>"},
		{"a\\\\\\\nb\n", "<p>a\\<br>b</p>"},
		// One trailing space is not a break; two are.
		{"a \nb\n", "<p>a b</p>"},
		{"a  \nb\n", "<p>a<br>b</p>"},
		// A break marker on the last line of a block emits nothing. The
		// backslash stays visible (the dangling-backslash finding); the
		// trailing spaces were never visible to begin with.
		{"a\\\n", "<p>a\\</p>"},
		{"a  \n", "<p>a</p>"},
		{"- a\\\n", "<ul><li>a\\</li></ul>"},
		// A break at the end of a paragraph that is followed by ANOTHER
		// paragraph is still a block end.
		{"a\\\n\nb\n", "<p>a\\</p><p>b</p>"},
	})
}

// TestRender_CodeSpanDoesNotSpanAHardBreak pins the one construct the
// hard-break amendment explicitly bounds: an opening backtick run whose only
// closer is on the far side of a break does not close, so both runs stay
// literal and the <br> is emitted between them.
func TestRender_CodeSpanDoesNotSpanAHardBreak(t *testing.T) {
	got := string(Render("a `co\\\nde` b\n"))
	if strings.Contains(got, "<code>") {
		t.Errorf("a code span must not span a hard break:\n%s", got)
	}
	if want := "<p>a `co<br>de` b</p>"; got != want {
		t.Errorf("code span across a hard break:\n got: %s\nwant: %s", got, want)
	}

	// The consequence of the package's existing fall-through rule, pinned so
	// it is on the record rather than a surprise: a run that cannot close is
	// literal and the scan RESUMES AFTER IT, so a later run — including the
	// one that was just rejected as a closer — can still open a span of its
	// own. That span is wholly on one side of the break, so the rule above
	// still holds; only the pairing moves, exactly as it does for any
	// unbalanced backtick run without a break involved.
	got = string(Render("a `co\\\nde` b `x` c\n"))
	if want := "<p>a `co<br>de<code> b </code>x` c</p>"; got != want {
		t.Errorf("unbalanced runs either side of a break:\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "<br>") && strings.Contains(got, "<code>") {
		// Structural check of the same claim: no <br> may fall inside a span.
		open := strings.Index(got, "<code>")
		closeAt := strings.Index(got, "</code>")
		if br := strings.Index(got, "<br>"); open < br && br < closeAt {
			t.Errorf("a <br> landed inside a code span:\n%s", got)
		}
	}
}

// TestRender_LinkTextAcrossAHardBreakLosesTheBreak pins the documented
// ceiling, so it is a decision on the record rather than a surprise: the
// amendment bounds code spans at a break and says nothing about links, and
// parseLink's grammar takes its text verbatim — so a link written ACROSS a
// break renders as one anchor and the break is not emitted. Nothing is
// dropped or mis-escaped; only the <br> is absent.
func TestRender_LinkTextAcrossAHardBreakLosesTheBreak(t *testing.T) {
	got := string(Render("[a\\\nb](#x)\n"))
	if want := `<p><a href="#x">a b</a></p>`; got != want {
		t.Errorf("link across a hard break:\n got: %s\nwant: %s", got, want)
	}
}

// --- the inline-only surface ----------------------------------------------

// TestRenderInline_EmitsNoBlockMarkup is the table-cell half of the container
// matrix, asserted from the outside: RenderInline is the <td> entry point, and
// no phase-B construct may appear through it — no <hr>, no heading, no
// <blockquote>, no list, no checkbox, and no <br>, since a cell is one line.
func TestRenderInline_EmitsNoBlockMarkup(t *testing.T) {
	for _, in := range []string{
		"---",
		"### heading",
		"> quoted",
		"- [x] task",
		"1. item",
		"a\\",
		"a  ",
	} {
		got := string(RenderInline(in))
		for _, forbidden := range []string{"<hr", "<h3", "<blockquote", "<ul", "<ol", "<li", "<input", "<br", "<p>"} {
			if strings.Contains(got, forbidden) {
				t.Errorf("RenderInline(%q) emitted %s — a table cell is inline-only: %s", in, forbidden, got)
			}
		}
	}
}

// --- looseness ------------------------------------------------------------

// TestRender_LoosenessWrapsProseOnly pins what the <p> wrapper does and does
// NOT cover in a loose item: prose runs are wrapped, a nested list and a
// fenced code block are not, because CommonMark wraps paragraphs and those are
// not paragraphs.
func TestRender_LoosenessWrapsProseOnly(t *testing.T) {
	got := string(Render("- a\n\n  text\n\n  ```\n  code\n  ```\n\n  - n\n"))
	want := "<ul><li><p>a text</p><pre><code>code\n</code></pre><ul><li>n</li></ul></li></ul>"
	if got != want {
		t.Errorf("loose item block wrapping:\n got: %s\nwant: %s", got, want)
	}
	assertTagBalance(t, got)
}

// TestRender_LoosenessIsPerList pins the property that gives the release its
// spaced-list fix without over-reaching: a blank line makes the list it falls
// in loose, and leaves a tight nested list tight.
func TestRender_LoosenessIsPerList(t *testing.T) {
	renderCases(t, "looseness scope", []struct{ in, want string }{
		// Blank inside the NESTED list: only the nested list goes loose.
		{"- a\n  - x\n\n  - y\n",
			"<ul><li>a<ul><li><p>x</p></li><li><p>y</p></li></ul></li></ul>"},
		// A trailing blank line before the list ends is not "inside" it.
		{"- a\n- b\n\nprose\n", "<ul><li>a</li><li>b</li></ul><p>prose</p>"},
	})
}
