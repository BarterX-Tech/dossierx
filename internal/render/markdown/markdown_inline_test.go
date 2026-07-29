package markdown

import (
	"html"
	"strings"
	"testing"
)

// This file is phase A's test surface: the four inline constructs (emphasis,
// strikethrough, fence info string, autolinks) plus the nesting matrix that is
// their acceptance test.
//
// Every case is written against the RULE, not against the implementation: the
// flanking cases in particular name the corpus tokens the rule exists to
// protect (governed_by, rests_on, schema_version) rather than abstract
// examples, because "does not italicise" is the whole reason "_" is in scope.

// --- 1. bold -------------------------------------------------------------

func TestRenderInline_Bold(t *testing.T) {
	cases := []struct{ in, want string }{
		{"**bold**", "<strong>bold</strong>"},
		{"a **b** c", "a <strong>b</strong> c"},
		{"**a** and **b**", "<strong>a</strong> and <strong>b</strong>"},
		{"__bold__", "<strong>bold</strong>"},
		{"**unclosed", "**unclosed"},
		{`\*\*not bold\*\*`, "**not bold**"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRender_BoldAtColumnZeroIsNotAList is the invariant the plan asks to be
// made deliberate rather than accidental: unorderedMarker is ^[-*]\s+, so the
// REQUIRED WHITESPACE after the marker is the only thing standing between
// "**bold**" at the head of a line and a one-item unordered list.
func TestRender_BoldAtColumnZeroIsNotAList(t *testing.T) {
	cases := []struct{ in, want string }{
		{"**bold**", "<p><strong>bold</strong></p>"},
		{"**bold** and more", "<p><strong>bold</strong> and more</p>"},
		{"*italic* rest", "<p><em>italic</em> rest</p>"},
		{"*not a list*item", "<p><em>not a list</em>item</p>"},
		{"* item", "<ul><li>item</li></ul>"},
		{"*   item", "<ul><li>item</li></ul>"},
		{"- **bold** item", "<ul><li><strong>bold</strong> item</li></ul>"},
	}
	for _, tc := range cases {
		if got := string(Render(tc.in)); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestUnorderedMarkerRequiresWhitespace pins the regex itself, so a future
// edit that drops the \s+ fails here with the reason attached rather than
// silently turning every bold run at column zero into a bullet.
func TestUnorderedMarkerRequiresWhitespace(t *testing.T) {
	for _, s := range []string{"**bold**", "*italic*", "*x", "-x", "**", "*"} {
		if unorderedMarker.MatchString(s) {
			t.Errorf("unorderedMarker matched %q — the required whitespace after the marker is what keeps emphasis and list markers apart", s)
		}
	}
	for _, s := range []string{"* x", "- x", "*\tx", "*   x"} {
		if !unorderedMarker.MatchString(s) {
			t.Errorf("unorderedMarker did not match %q", s)
		}
	}
}

// --- 2. italic and the flanking rule -------------------------------------

func TestRenderInline_Italic(t *testing.T) {
	cases := []struct{ in, want string }{
		{"*italic*", "<em>italic</em>"},
		{"_italic_", "<em>italic</em>"},
		{"a *b* c", "a <em>b</em> c"},
		{"a _b_ c", "a <em>b</em> c"},
		{"*unclosed", "*unclosed"},
		{`\*not italic\*`, "*not italic*"},
		{`\_not italic\_`, "_not italic_"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_IntrawordUnderscoreNeverEmphasises is the corpus-protection
// test. Every token here was found by the gate-0 corpus scan; every one of
// them is INTRAWORD, and CommonMark's flanking rule makes an intraword "_"
// incapable of opening OR closing emphasis. If this test ever fails, the
// entire argument for admitting "_" to the construct set has failed with it.
func TestRenderInline_IntrawordUnderscoreNeverEmphasises(t *testing.T) {
	corpus := []string{
		"rests_on", "governed_by", "claims_dir", "schema_version", "build_role",
		"raw_html", "migrated_from", "validated_at", "depended_by",
		"snake_case_word", "a_b_c_d", "governed_by and rests_on and claims_dir",
		"schema_version=3 and build_role=api",
		"one governed_by, two rests_on, three raw_html.",
	}
	for _, in := range corpus {
		got := string(RenderInline(in))
		if strings.Contains(got, "<em>") || strings.Contains(got, "<strong>") {
			t.Errorf("intraword underscore emphasised in %q: %s", in, got)
		}
		if got != in {
			t.Errorf("RenderInline(%q) = %q, want the input unchanged", in, got)
		}
	}
}

// TestRenderInline_UnderscoreWordBoundaryExposure covers classes 1 and 2 of the
// residual exposure named in markdown_inline.go's flanking comment: a
// word-boundary underscore that can only open, and one that can only close.
// Alone each is literal, because it has no partner; the two together in one
// block legitimately emphasise, which is CommonMark's answer and the documented
// cost of admitting "_".
//
// A BACKSLASH ESCAPE AND A CODE SPAN ARE THE COVER, and they are the only cover.
// This comment used to name markdown-sanity's unmatched-run finding as a
// backstop; it is not one, and TestRenderInline_UnderscorePairsAreNotUnmatched
// is why. Every case in this file that emphasises PAIRS, and an unmatched-run
// finding by definition reports runs that do not.
func TestRenderInline_UnderscoreWordBoundaryExposure(t *testing.T) {
	cases := []struct{ in, want string }{
		// Unpaired: literal.
		{"_leading", "_leading"},
		{"trailing_.", "trailing_."},
		{"foo_bar_ baz", "foo_bar_ baz"},
		{"a _leading identifier", "a _leading identifier"},
		// Escaped: literal, by the ordinary mechanism.
		{`\_leading and trailing\_.`, "_leading and trailing_."},
		// A code span defuses the whole run at once.
		{"`_leading and trailing_`", "<code>_leading and trailing_</code>"},
		// Legitimately paired: emphasis, as CommonMark says.
		{"_leading and trailing_.", "<em>leading and trailing</em>."},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_UnderscorePunctuationFlankedEmphasises is CLASS 3, the class
// the earlier statement of the residual exposure left out entirely.
//
// The claim it corrects was: "the residual exposure is exactly word-boundary
// underscores, and nothing else — a leading _ident and a trailing ident_;
// neither does anything alone." Both halves were wrong. delimFlags gives "_"
// canOpen = left && (!right || beforePunct) and canClose = right && (!left ||
// afterPunct); a run with ASCII PUNCTUATION ON BOTH SIDES is both left- and
// right-flanking AND has beforePunct and afterPunct both true, so BOTH escape
// clauses fire and the run can open AND close. It is neither leading nor
// trailing. And a single token can carry a class-1 run and a class-2 run at
// once, so it needs no partner: "__init__" emphasises on its own.
//
// Every expectation here is what CommonMark's reference implementation
// produces. The test exists so the CEILING is pinned, not because the rule is
// wrong: a reader deciding whether "_" was safe to admit is entitled to see the
// real shapes it reaches, including the one that occurs in this repository's own
// .goreleaser.yaml.
func TestRenderInline_UnderscorePunctuationFlankedEmphasises(t *testing.T) {
	cases := []struct {
		why  string
		in   string
		want string
	}{
		{
			why:  "the shape in this repo's .goreleaser.yaml: braces are punctuation on both sides",
			in:   "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}",
			want: "{{ .ProjectName }}<em>{{ .Os }}</em>{{ .Arch }}",
		},
		{why: "dashes", in: "-_- -_-", want: "-<em>- -</em>-"},
		{why: "slashes, as in a path fragment", in: "p/_/q/_/r", want: "p/<em>/q/</em>/r"},
		{why: "a leading underscore inside a path", in: "see dir/_private/ and name_/x",
			want: "see dir/<em>private/ and name</em>/x"},
		{why: "quotes, which are escaped as author bytes either way", in: `"_" and "_"`,
			want: "&#34;<em>&#34; and &#34;</em>&#34;"},
		{why: "pipes", in: "|_|_|", want: "|<em>|</em>|"},
		{why: "equals signs", in: "a=_=b=_=c", want: "a=<em>=b=</em>=c"},
		{why: "parentheses", in: "x)_(y)_(z", want: "x)<em>(y)</em>(z"},
		// One token, no partner needed: a class-1 run and a class-2 run in the
		// same word, which is exactly what a dunder name is.
		{why: "a dunder name emphasises alone", in: "__init__", want: "<strong>init</strong>"},
		{why: "two dunder names in one line", in: "__all__ and __main__",
			want: "<strong>all</strong> and <strong>main</strong>"},
		// The cover, on the same shapes.
		{why: "a code span defuses class 3", in: "`{{ .A }}_{{ .B }}_{{ .C }}`",
			want: "<code>{{ .A }}_{{ .B }}_{{ .C }}</code>"},
		{why: "a backslash escape defuses class 3", in: `a}\_{b}\_{c`, want: "a}_{b}_{c"},
		{why: "a backslash escape defuses a dunder name", in: `\_\_init\_\_`, want: "__init__"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

// TestRenderInline_UnderscoreClassThreeNeedsPunctuationOnBothSides is the
// narrowing that keeps the corpus argument intact after class 3 is admitted.
//
// Class 3 requires ASCII punctuation on BOTH sides of the run. A snake_case
// identifier by definition has alphanumerics there, so no amount of class-3
// exposure can reach one — which is why
// TestRenderInline_IntrawordUnderscoreNeverEmphasises still holds and why the
// nine corpus tokens are still safe. This test states that boundary as a
// property of delimFlags rather than as a claim about a token list: a "_" run
// can open or close ONLY IF at least one side is not alphanumeric.
func TestRenderInline_UnderscoreClassThreeNeedsPunctuationOnBothSides(t *testing.T) {
	alnum := "aZ0"
	other := " \t.-/{}|=+#\"')(_"
	for _, b := range alnum + other {
		for _, a := range alnum + other {
			text := string(b) + "_" + string(a)
			f := flankingOf(text, 1, 1, inlineCtx{})
			canOpen, canClose := delimFlags('_', f)
			bothAlnum := strings.ContainsRune(alnum, b) && strings.ContainsRune(alnum, a)
			if bothAlnum && (canOpen || canClose) {
				t.Errorf("intraword %q: canOpen=%v canClose=%v, want both false — "+
					"this is the property every snake_case corpus token relies on",
					text, canOpen, canClose)
			}
			bothPunct := strings.ContainsRune(other, b) && strings.ContainsRune(other, a) &&
				b != ' ' && b != '\t' && a != ' ' && a != '\t'
			if bothPunct && !(canOpen && canClose) {
				t.Errorf("punctuation-flanked %q: canOpen=%v canClose=%v, want both true — "+
					"this is class 3 of the residual exposure", text, canOpen, canClose)
			}
		}
	}
}

// TestDelimiterFlanking pins the flanking predicate itself, one run at a time,
// so a regression names the rule rather than an output string.
func TestDelimiterFlanking(t *testing.T) {
	cases := []struct {
		name             string
		text             string
		start, n         int
		ch               byte
		canOpen, canClos bool
	}{
		{"star at start before word", "*a*", 0, 1, '*', true, false},
		{"star at end after word", "*a*", 2, 1, '*', false, true},
		{"star intraword", "a*b", 1, 1, '*', true, true},
		{"star surrounded by spaces", "a * b", 2, 1, '*', false, false},
		{"underscore intraword", "a_b", 1, 1, '_', false, false},
		{"underscore after space before word", " _a", 1, 1, '_', true, false},
		{"underscore after word before space", "a_ ", 1, 1, '_', false, true},
		{"underscore after word before punct", "a_.", 1, 1, '_', false, true},
		{"underscore after punct before word", "._a", 1, 1, '_', true, false},
		// CLASS 3. Punctuation on BOTH sides makes the run left- AND
		// right-flanking, and makes beforePunct and afterPunct both true, so
		// both of the "_" escape clauses fire and the run can open AND close.
		// The eleven rows this table used to have contained the two near-misses
		// above ("._a", open only; "a_.", close only) and nothing that pinned
		// the case where both fire — which is how the claim that the exposure
		// was "exactly word-boundary underscores" survived.
		{"underscore between punctuation (braces)", "}_{", 1, 1, '_', true, true},
		{"underscore between punctuation (slashes)", "/_/", 1, 1, '_', true, true},
		{"underscore between punctuation (dashes)", "-_-", 1, 1, '_', true, true},
		{"underscore between punctuation (quotes)", `"_"`, 1, 1, '_', true, true},
		{"double underscore between punctuation", "}__{", 1, 2, '_', true, true},
		// A dunder name is a class-1 run and a class-2 run in one token, which
		// is why it emphasises with no partner elsewhere in the block.
		{"dunder opener at start of token", "__init__", 0, 2, '_', true, false},
		{"dunder closer at end of token", "__init__", 6, 2, '_', false, true},
		// "*" has no extra clause, so punctuation on both sides is nothing
		// special for it — it is the same both-flanking case as intraword.
		{"star between punctuation", "}*{", 1, 1, '*', true, true},
		{"tilde at start before word", "~~a~~", 0, 2, '~', true, false},
		{"tilde at end after word", "~~a~~", 3, 2, '~', false, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := flankingOf(tc.text, tc.start, tc.n, inlineCtx{})
			gotOpen, gotClose := delimFlags(tc.ch, f)
			if gotOpen != tc.canOpen || gotClose != tc.canClos {
				t.Errorf("delimFlags(%q of %q at %d) = (open=%v, close=%v), want (open=%v, close=%v)",
					tc.text[tc.start:tc.start+tc.n], tc.text, tc.start,
					gotOpen, gotClose, tc.canOpen, tc.canClos)
			}
		})
	}
}

// TestDelimiterFlanking_NeighbourIsACharacterNotAByte pins the classification of
// a NON-ASCII neighbour, which is the half of the flanking rule a byte-wise
// implementation gets wrong in both directions at once.
//
// The rule reads one character on each side. Classifying every byte >= 0x80 as
// a word character — which is what this file used to do, under a comment
// calling it "the safe direction" — makes canOpen unconditionally true next to
// a non-ASCII dash or quotation mark and takes canClose away next to a
// non-ASCII space. Neither is conservative; both are simply wrong. Every row
// here is a character CommonMark classifies as Zs or as P/S, sitting next to a
// run that must therefore not do what the byte rule let it do.
func TestDelimiterFlanking_NeighbourIsACharacterNotAByte(t *testing.T) {
	cases := []struct {
		name             string
		text             string
		start, n         int
		ch               byte
		canOpen, canClos bool
	}{
		// Category Zs: whitespace, so the run cannot open across it. Every
		// non-ASCII character in this table that is not a letter is written as
		// a \u escape, because an invisible byte in an expected value is
		// exactly how a flanking bug hides.
		{"star before no-break space", "*\u00a0a", 0, 1, '*', false, false},
		{"star after no-break space", "a\u00a0*b", 3, 1, '*', true, false},
		{"star before en quad", "*\u2000a", 0, 1, '*', false, false},
		{"underscore before no-break space", "_\u00a0a", 0, 1, '_', false, false},
		// Category Pd / Pi / Pf / Po: punctuation, exactly like ASCII "-" or ".".
		{"star between em dash and word", "\u2014*b", 3, 1, '*', true, false},
		{"star between word and em dash", "b*\u2014", 1, 1, '*', false, true},
		{"tilde between em dash and word", "\u2014~~b", 3, 2, '~', true, false},
		{"star between left and right curly quotes", "\u201c*\u201d", 3, 1, '*', true, true},
		// Category Sc: a currency sign is punctuation under CommonMark's
		// definition, which is spec example 354's whole point: the run AFTER a
		// currency sign is left-flanking and NOT right-flanking, so it cannot
		// close what the run before it opened and the emphasis never forms.
		{"star between pound sign and word", "\u00a3*b", 2, 1, '*', true, false},
		{"star between word and pound sign", "b*\u00a3", 1, 1, '*', false, true},
		// A letter is a word character whatever its script.
		{"star between cyrillic letters", "а*б", 2, 1, '*', true, true},
		// An invalid UTF-8 byte is a word character: stated divergence (a).
		{"star after an invalid byte", "\xff*b", 1, 1, '*', true, true},
		{"underscore after an invalid byte", "\xff_b", 1, 1, '_', false, false},
		// A NUL is a neighbour, not the ABSENCE of one: stated divergence (b).
		// If it were read as absence it would count as whitespace, and the
		// underscore row below would come out (true, false) instead.
		{"star after a NUL byte", "\x00*b", 1, 1, '*', true, true},
		{"underscore after a NUL byte", "\x00_b", 1, 1, '_', false, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := flankingOf(tc.text, tc.start, tc.n, inlineCtx{})
			gotOpen, gotClose := delimFlags(tc.ch, f)
			if gotOpen != tc.canOpen || gotClose != tc.canClos {
				t.Errorf("delimFlags(%q of %q at %d) = (open=%v, close=%v), want (open=%v, close=%v)",
					tc.text[tc.start:tc.start+tc.n], tc.text, tc.start,
					gotOpen, gotClose, tc.canOpen, tc.canClos)
			}
		})
	}
}

// TestDelimiterFlanking_EdgesStandInForTheSurroundingText pins inlineCtx's
// edges at the level of the predicate itself: the same four bytes must flank
// differently depending on what the caller says is outside them, because for a
// link's text something always is.
func TestDelimiterFlanking_EdgesStandInForTheSurroundingText(t *testing.T) {
	cases := []struct {
		name             string
		text             string
		start, n         int
		ch               byte
		ctx              inlineCtx
		canOpen, canClos bool
	}{
		// No edges: the ends of the text are whitespace, which is what a
		// top-level block's text actually has there.
		{"underscore run at the start of a bare text", "__.", 0, 2, '_',
			inlineCtx{}, true, false},
		{"underscore run at the end of a bare text", ".__", 1, 2, '_',
			inlineCtx{}, false, true},
		// Bracketed: "[" and "]" are ASCII punctuation, so both runs are both
		// left- and right-flanking AND punctuation-flanked, which is class 3 —
		// they can open AND close. That is the difference the rule of three
		// then arbitrates, and it is why "[__._](url)" stays literal.
		{"underscore run at the start of a link's text", "__.", 0, 2, '_',
			inlineCtx{inLink: true, leftEdge: '[', rightEdge: ']'}, true, true},
		{"underscore run at the end of a link's text", ".__", 1, 2, '_',
			inlineCtx{inLink: true, leftEdge: '[', rightEdge: ']'}, true, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := flankingOf(tc.text, tc.start, tc.n, tc.ctx)
			gotOpen, gotClose := delimFlags(tc.ch, f)
			if gotOpen != tc.canOpen || gotClose != tc.canClos {
				t.Errorf("delimFlags(%q of %q at %d, edges %q/%q) = (open=%v, close=%v), want (open=%v, close=%v)",
					tc.text[tc.start:tc.start+tc.n], tc.text, tc.start,
					tc.ctx.leftEdge, tc.ctx.rightEdge,
					gotOpen, gotClose, tc.canOpen, tc.canClos)
			}
		})
	}
}

// TestRenderInline_NonASCIINeighboursDoNotEmphasise is the same defect seen
// from the output end, on the exact strings the byte rule mis-rendered.
func TestRenderInline_NonASCIINeighboursDoNotEmphasise(t *testing.T) {
	cases := []struct{ why, in, want string }{
		{"an em dash is punctuation, so the closing run cannot close",
			"*\u2014*bravo.", "*\u2014*bravo."},
		{"the same, spelled with tildes", "~~\u2014~~bravo.", "~~\u2014~~bravo."},
		{"an en dash behaves like an em dash", "*\u2013*bravo.", "*\u2013*bravo."},
		{"a right single quote is punctuation", "*\u2019*bravo.", "*\u2019*bravo."},
		{"an ellipsis is punctuation", "*\u2026*bravo.", "*\u2026*bravo."},
		{"a no-break space is whitespace, so the opener cannot open",
			"*\u00a0a\u00a0*", "*\u00a0a\u00a0*"},
		{"a no-break space stops '_' the same way", "_\u00a0a\u00a0_", "_\u00a0a\u00a0_"},
		{"spec example 354: a currency sign is punctuation",
			"*\u00a3*bravo.", "*\u00a3*bravo."},
		{"a letter is a word character in any script, so this one does emphasise",
			"*аб*", "<em>аб</em>"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

// TestRenderInline_LinkTextFlanksAgainstItsBrackets pins the other half of the
// same "one character on each side" problem: a link's text is rendered by a
// recursive call on a SUBSTRING, so without inlineCtx's edges the run at either
// end of it would flank against start-of-text and end-of-text \u2014 whitespace \u2014
// instead of against the "[" and "]" that actually bracket it in the source.
//
// The mechanism is visible with no link involved at all: RenderInline("__._")
// and RenderInline("[__._]") are the same four bytes with the same neighbours
// in the second case as in the link case, and they must not disagree about
// whether the run at the start can open.
func TestRenderInline_LinkTextFlanksAgainstItsBrackets(t *testing.T) {
	cases := []struct{ why, in, want string }{
		{"the opening run has '[' before it, which is punctuation, and '.' after it",
			"[__._](https://ok.test/)", `<a href="https://ok.test/">__._</a>`},
		{"the mirror case at the closing end",
			"[_.__](https://ok.test/)", `<a href="https://ok.test/">_.__</a>`},
		{"the same bytes with no link: the brackets are visible either way",
			"[__._]", "[__._]"},
		{"the acceptance case still composes",
			"[**text**](https://ok.test/)", `<a href="https://ok.test/"><strong>text</strong></a>`},
		{"and so does emphasis that pairs strictly inside the text",
			"[a *b* c](https://ok.test/)", `<a href="https://ok.test/">a <em>b</em> c</a>`},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

// --- 3. strikethrough -----------------------------------------------------

func TestRenderInline_Strikethrough(t *testing.T) {
	cases := []struct{ in, want string }{
		{"~~gone~~", "<del>gone</del>"},
		{"a ~~b~~ c", "a <del>b</del> c"},
		{"~single~", "~single~"},
		{"~~~triple~~~", "~~~triple~~~"},
		{"~~unclosed", "~~unclosed"},
		{`\~\~not struck\~\~`, "~~not struck~~"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- 4. fence info string -> language class -------------------------------

func TestRender_FenceInfoStringBecomesLanguageClass(t *testing.T) {
	cases := []struct{ in, want string }{
		{"```json\n{}\n```\n", `<pre><code class="language-json">{}` + "\n</code></pre>"},
		{"```go\nx\n```\n", `<pre><code class="language-go">x` + "\n</code></pre>"},
		{"```c++\nx\n```\n", `<pre><code class="language-c++">x` + "\n</code></pre>"},
		// Only the first word is the language; the rest of the info string is
		// dropped exactly as the whole of it used to be.
		{"```js title=a.js\nx\n```\n", `<pre><code class="language-js">x` + "\n</code></pre>"},
		// No info string: no class at all, byte-identical to before.
		{"```\nx\n```\n", "<pre><code>x\n</code></pre>"},
		{"```   \nx\n```\n", "<pre><code>x\n</code></pre>"},
		// Not a plain identifier: REJECTED, never emitted raw.
		{"```<script>\nx\n```\n", "<pre><code>x\n</code></pre>"},
		{"```\"onload=alert(1)\nx\n```\n", "<pre><code>x\n</code></pre>"},
		{"```1st\nx\n```\n", "<pre><code>x\n</code></pre>"},
		{"```a b\"c\nx\n```\n", `<pre><code class="language-a">x` + "\n</code></pre>"},
	}
	for _, tc := range cases {
		if got := string(Render(tc.in)); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestFenceInfoLanguage_RejectsEverythingButAnIdentifier is the attribute-safety
// half: the class value is author bytes in an attribute, so anything that is
// not a plain identifier must be REJECTED rather than escaped-and-emitted.
func TestFenceInfoLanguage_RejectsEverythingButAnIdentifier(t *testing.T) {
	for _, bad := range []string{
		`"`, `" onload=x`, `<script>`, `a"b`, `a'b`, `a>b`, `a&b`, `a/b`, `a\b`,
		"1", "-a", "+a", ".a", "", "   ", strings.Repeat("a", 33),
	} {
		if got := infoLanguage(bad); got != "" {
			t.Errorf("infoLanguage(%q) = %q, want \"\" (rejected)", bad, got)
		}
	}
	for _, good := range []struct{ in, want string }{
		{"json", "json"}, {"Go", "Go"}, {"c++", "c++"}, {"objective-c", "objective-c"},
		{"asp.net", "asp.net"}, {"x_y", "x_y"}, {"h1", "h1"}, {"js title=a", "js"},
		// Only the FIRST word is considered, so a hostile tail is dropped
		// rather than making the whole info string unusable.
		{"a b", "a"}, {"a\tb\"c", "a"},
	} {
		if got := infoLanguage(good.in); got != good.want {
			t.Errorf("infoLanguage(%q) = %q, want %q", good.in, got, good.want)
		}
	}
}

// --- 5a. angle-bracket autolinks -----------------------------------------

func TestRenderInline_AngleAutolink(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<https://example.test/p>", `<a href="https://example.test/p">https://example.test/p</a>`},
		{"<http://example.test>", `<a href="http://example.test">http://example.test</a>`},
		{"<mailto:a@b.test>", `<a href="mailto:a@b.test">mailto:a@b.test</a>`},
		{"see <https://x.test> now", `see <a href="https://x.test">https://x.test</a> now`},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_AngleIsNeverAnHTMLOpener is the invariant that keeps "raw
// inline HTML" a non-goal now that "<" is a construct opener. Anything that is
// not a complete autolink with an ALLOWED scheme comes out escaped.
func TestRenderInline_AngleIsNeverAnHTMLOpener(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		{`<img src=x onerror=alert(1)>`, "&lt;img src=x onerror=alert(1)&gt;"},
		{"<b>x</b>", "&lt;b&gt;x&lt;/b&gt;"},
		{"a < b", "a &lt; b"},
		{"<https://x.test", "&lt;https://x.test"},
		{"<>", "&lt;&gt;"},
		// A complete autolink whose scheme is REJECTED: the whole run,
		// brackets included, as inert escaped text.
		{"<javascript:alert(1)>", "&lt;javascript:alert(1)&gt;"},
		{"<data:text/html,x>", "&lt;data:text/html,x&gt;"},
		{"<vbscript:x>", "&lt;vbscript:x&gt;"},
		// Scheme-less: not an autolink at all (an autolink needs a scheme).
		{"<//evil.test/p>", "&lt;//evil.test/p&gt;"},
		{"<#frag>", "&lt;#frag&gt;"},
		{"<relative/path>", "&lt;relative/path&gt;"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- 5b. bare-URL autolinks ----------------------------------------------

func TestRenderInline_BareURLAutolink(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://example.test/p", `<a href="https://example.test/p">https://example.test/p</a>`},
		{"see https://example.test/p now", `see <a href="https://example.test/p">https://example.test/p</a> now`},
		{"HTTPS://EXAMPLE.TEST", `<a href="HTTPS://EXAMPLE.TEST">HTTPS://EXAMPLE.TEST</a>`},
		{"http://example.test", `<a href="http://example.test">http://example.test</a>`},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderInline_BareURLTermination(t *testing.T) {
	cases := []struct{ in, want string }{
		// Trailing sentence punctuation is not part of the href.
		{"see https://x.test/p.", `see <a href="https://x.test/p">https://x.test/p</a>.`},
		{"see https://x.test/p, and", `see <a href="https://x.test/p">https://x.test/p</a>, and`},
		{"see https://x.test/p; y", `see <a href="https://x.test/p">https://x.test/p</a>; y`},
		{"see https://x.test/p: y", `see <a href="https://x.test/p">https://x.test/p</a>: y`},
		{"see https://x.test/p!", `see <a href="https://x.test/p">https://x.test/p</a>!`},
		{"see https://x.test/p?", `see <a href="https://x.test/p">https://x.test/p</a>?`},
		{"see https://x.test/p?a=b", `see <a href="https://x.test/p?a=b">https://x.test/p?a=b</a>`},
		// A ")" that closes an enclosing parenthetical is not part of the href.
		{"(see https://x.test/p)", `(see <a href="https://x.test/p">https://x.test/p</a>)`},
		{"(see https://x.test/p).", `(see <a href="https://x.test/p">https://x.test/p</a>).`},
		// A ")" that the URL itself opened IS part of the href.
		{"https://x.test/wiki/A_(b)", `<a href="https://x.test/wiki/A_(b)">https://x.test/wiki/A_(b)</a>`},
		{"(https://x.test/wiki/A_(b))", `(<a href="https://x.test/wiki/A_(b)">https://x.test/wiki/A_(b)</a>)`},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderInline_BareURLDoesNotFire pins every place bare-URL recognition
// must stay off. The detector is a literal http://|https:// at a word
// boundary \u2014 allowedScheme is a second gate and is NEVER the detector, because
// it returns true for almost every prose token.
func TestRenderInline_BareURLDoesNotFire(t *testing.T) {
	cases := []struct{ in, want string }{
		// Not at a word boundary.
		{"ahttps://x.test", "ahttps://x.test"},
		{"x=https://x.test", "x=https://x.test"},
		// Wrong scheme, no www., no bare email.
		{"ftp://x.test", "ftp://x.test"},
		{"www.example.test", "www.example.test"},
		{"a@b.test", "a@b.test"},
		{"mailto:a@b.test", "mailto:a@b.test"},
		{"javascript:alert(1)", "javascript:alert(1)"},
		// Scheme with no authority at all.
		{"https://", "https://"},
		// Inside a code span.
		{"`https://x.test`", "<code>https://x.test</code>"},
		{"``https://x.test``", "<code>https://x.test</code>"},
		// Inside an already-consumed link: the text stays literal and the url
		// is the anchor's href, not a second anchor.
		{"[https://x.test](https://x.test)", `<a href="https://x.test">https://x.test</a>`},
		{"[a](https://x.test)", `<a href="https://x.test">a</a>`},
		// Escaped prose that merely looks like a scheme.
		{"the string claims_dir", "the string claims_dir"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRender_BareURLNotInAFence: a fence never reaches the inline pass at all,
// so a URL inside one stays source text.
func TestRender_BareURLNotInAFence(t *testing.T) {
	out := string(Render("```\nhttps://x.test\n```\n"))
	if strings.Contains(out, "<a href") {
		t.Errorf("bare URL autolinked inside a fence: %s", out)
	}
}

// TestRenderInline_BareURLWinsOverDelimiters: every emphasis, strike and
// underscore delimiter inside a consumed URL is literal. A URL is one token.
func TestRenderInline_BareURLWinsOverDelimiters(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://x.test/v1/_status_", `<a href="https://x.test/v1/_status_">https://x.test/v1/_status_</a>`},
		{"https://x.test/a*b*c", `<a href="https://x.test/a*b*c">https://x.test/a*b*c</a>`},
		{"https://x.test/a~~b~~c", `<a href="https://x.test/a~~b~~c">https://x.test/a~~b~~c</a>`},
	}
	for _, tc := range cases {
		got := string(RenderInline(tc.in))
		if got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- nesting: the acceptance test ----------------------------------------

func TestRenderInline_Nesting(t *testing.T) {
	cases := []struct{ in, want string }{
		// Code spans win over everything \u2014 this was already true and stays true.
		{"**bold `code`**", "<strong>bold <code>code</code></strong>"},
		{"` **not bold** `", "<code> **not bold** </code>"},
		{"`*a*`", "<code>*a*</code>"},
		{"`~~a~~`", "<code>~~a~~</code>"},
		{"**`a`**", "<strong><code>a</code></strong>"},
		// Emphasis composes with itself and with strike.
		{"*__a__*", "<em><strong>a</strong></em>"},
		{"**_a_**", "<strong><em>a</em></strong>"},
		{"***a***", "<em><strong>a</strong></em>"},
		{"~~**a**~~", "<del><strong>a</strong></del>"},
		{"**~~a~~**", "<strong><del>a</del></strong>"},
		// Link text runs the full inline pass.
		{"[**text**](https://x.test/y)", `<a href="https://x.test/y"><strong>text</strong></a>`},
		{"[`code`](https://x.test/y)", `<a href="https://x.test/y"><code>code</code></a>`},
		{"**[text](https://x.test/y)**", `<strong><a href="https://x.test/y">text</a></strong>`},
		// A rejected scheme is still inert literal text, delimiters and all.
		{"[**t**](javascript:alert(1))", "[**t**](javascript:alert(1))"},
	}
	for _, tc := range cases {
		if got := string(RenderInline(tc.in)); got != tc.want {
			t.Errorf("RenderInline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRender_EmphasisSpansAHardBreak: the inline unit is the whole block, so a
// construct that spans a soft line break spans a hard one too. A code span is
// the documented exception and must stay one.
func TestRender_EmphasisSpansAHardBreak(t *testing.T) {
	cases := []struct{ in, want string }{
		{"**a  \nb**", "<p><strong>a<br>b</strong></p>"},
		{"**a\\\nb**", "<p><strong>a<br>b</strong></p>"},
		{"~~a  \nb~~", "<p><del>a<br>b</del></p>"},
		{"*a\nb*", "<p><em>a b</em></p>"},
		// A code span may NOT span a hard break: the run stays literal.
		{"`a  \nb`", "<p>`a<br>b`</p>"},
	}
	for _, tc := range cases {
		if got := string(Render(tc.in)); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRender_EmphasisInEveryContainer: the inline pass is one function, so a
// construct behaves identically in a paragraph, a heading, a list item, a
// blockquote and a table cell.
func TestRender_EmphasisInEveryContainer(t *testing.T) {
	cases := []struct{ in, want string }{
		{"### a **b** c", "<h3>a <strong>b</strong> c</h3>"},
		{"- *italic* item", "<ul><li><em>italic</em> item</li></ul>"},
		{"1. ~~gone~~", "<ol><li><del>gone</del></li></ol>"},
		{"> a **b**", "<blockquote><p>a <strong>b</strong></p></blockquote>"},
		{"- [ ] **todo**", `<ul><li class="task"><input type="checkbox" disabled> <strong>todo</strong></li></ul>`},
		{"### <https://x.test>", `<h3><a href="https://x.test">https://x.test</a></h3>`},
		{"> see https://x.test", `<blockquote><p>see <a href="https://x.test">https://x.test</a></p></blockquote>`},
	}
	for _, tc := range cases {
		if got := string(Render(tc.in)); got != tc.want {
			t.Errorf("Render(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- the escaping boundary ------------------------------------------------

// TestRenderInline_EscapingBoundaryHolds is the smoke test for the four new
// constructs: no author byte reaches the output unescaped, by any route.
func TestRenderInline_EscapingBoundaryHolds(t *testing.T) {
	hostile := []string{
		"<script>alert(1)</script>",
		"**<script>**",
		"*<script>*",
		"~~<script>~~",
		"<https://x.test/\"><script>alert(1)</script>",
		"https://x.test/\"><script>",
		"[**<script>**](https://x.test)",
		"<https://x.test/&amp;>",
		"**a & b**",
		"_a < b_",
		"~~a > b~~",
		"<javascript:alert(1)>",
		"< script>",
		"<<<<<<<<",
		"<a href=\"x\">",
	}
	for _, in := range hostile {
		got := string(RenderInline(in))
		if strings.Contains(got, "<script") || strings.Contains(got, "onerror") {
			t.Errorf("unescaped markup leaked from %q: %s", in, got)
		}
		// The only tags this package may emit.
		for _, frag := range []string{"<a href=\"x\">", "<img", "<b>", "<div"} {
			if strings.Contains(got, frag) {
				t.Errorf("unexpected tag %q emitted from %q: %s", frag, in, got)
			}
		}
	}
}

// TestEscapedDelimMatchesEscapeString pins the one place the write-out does not
// call html.EscapeString on the spot. A delimiter run's leftover characters are
// written from a table computed at package init, so that a run of a million
// unconsumed "*" costs no allocation per character; this test is what keeps that
// table equal to what html.EscapeString would have produced, which is the whole
// reason it is allowed to exist.
func TestEscapedDelimMatchesEscapeString(t *testing.T) {
	for _, c := range []byte{'*', '_', '~'} {
		want := html.EscapeString(string(rune(c)))
		if got := escapedDelim[c]; got != want {
			t.Errorf("escapedDelim[%q] = %q, want html.EscapeString's %q", c, got, want)
		}
	}
	// Every other slot is empty, so a future delimiter character added to the
	// scan without a table entry emits nothing and fails loudly in the golden
	// files rather than silently emitting a raw byte.
	for c := 0; c < len(escapedDelim); c++ {
		if c == '*' || c == '_' || c == '~' {
			continue
		}
		if escapedDelim[c] != "" {
			t.Errorf("escapedDelim[%d] = %q, want empty", c, escapedDelim[c])
		}
	}
}

// TestRenderInline_UnderscorePairsAreNotUnmatched is the negative result that
// corrects what the flanking comment used to claim. It asserts a property of
// THIS package only \u2014 that every underscore shape which emphasises has EVEN
// PARITY, i.e. its delimiter runs pair \u2014 because an unmatched-run finding, by
// construction, reports only runs that do NOT pair. So markdown-sanity cannot be
// the backstop for any of them, and the flanking comment no longer says it is.
//
// The lint itself lives in internal/lint and phase A may not touch it; the gap
// is reported in cross_file_needs. This test is what stops the claim from
// quietly coming back.
func TestRenderInline_UnderscorePairsAreNotUnmatched(t *testing.T) {
	for _, in := range []string{
		"_leading and trailing_",
		"{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}",
		"__init__",
		"-_- -_-",
		"p/_/q/_/r",
	} {
		out := string(RenderInline(in))
		if !strings.Contains(out, "<em>") && !strings.Contains(out, "<strong>") {
			t.Fatalf("RenderInline(%q) = %q \u2014 this test only means anything for shapes that DO emphasise", in, out)
		}
		if n := strings.Count(in, "_") % 2; n != 0 {
			t.Errorf("RenderInline(%q) emphasises with an odd number of underscores; "+
				"the point of this test is that every emphasising shape has even parity, "+
				"so an unmatched-run finding can never report one", in)
		}
	}
}

// TestRenderInline_EmphasisHrefsGoThroughAllowedScheme closes the loop from
// both autolink forms to the security boundary: nothing becomes a live anchor
// that allowedScheme would refuse.
func TestRenderInline_EmphasisHrefsGoThroughAllowedScheme(t *testing.T) {
	for _, url := range []string{
		"javascript:alert(1)", "data:text/html,x", "vbscript:x", "  JavaScript:x",
		"java\tscript:alert(1)", "//evil.test/p", "\\\\evil.test/p",
	} {
		for _, in := range []string{"<" + url + ">", "**<" + url + ">**", "[t](" + url + ")"} {
			got := string(RenderInline(in))
			if strings.Contains(got, "<a href") {
				t.Errorf("rejected scheme became an anchor: RenderInline(%q) = %s", in, got)
			}
		}
	}
}

// --- parity ---------------------------------------------------------------

// TestRenderInlineParity_PhaseA: Render and RenderInline run the same inline
// pass, so every phase-A construct behaves identically in a paragraph and in a
// table cell. Render adds only the block wrapper.
func TestRenderInlineParity_PhaseA(t *testing.T) {
	for _, in := range []string{
		"**b**", "*i*", "_i_", "~~s~~", "*__a__*", "**`a`**",
		"<https://x.test>", "<javascript:x>", "see https://x.test/p.",
		"governed_by", "[**t**](https://x.test)",
	} {
		inline := string(RenderInline(in))
		block := string(Render(in))
		if block != "<p>"+inline+"</p>" {
			t.Errorf("parity broken for %q:\n Render:       %s\n RenderInline: %s", in, block, inline)
		}
	}
}
