package markdown

import (
	"strings"
	"testing"
)

// This file is the conformance gate for emphasis.
//
// The flanking rule that decides whether a "*" or a "_" emphasises is
// CommonMark's, and the argument for admitting "_" at all — that no intraword
// underscore can emphasise, so no snake_case identifier in the corpus can — is
// only as good as this package's fidelity to that rule. Hand-written tables
// test the cases their author thought of; the spec's own examples test the ones
// nobody thought of, which is where a flanking bug actually lives.
//
// commonMarkEmphasisSpec (markdown_commonmark_data_test.go) holds all 132
// examples of the spec's "Emphasis and strong emphasis" section verbatim. This
// file runs them through Render and pins the result, including the handful this
// renderer deliberately does not match.

// specToRendererSpelling rewrites the spec's expected HTML into this renderer's
// spelling. There are exactly two differences, both structural, both older than
// emphasis, and neither one about emphasis:
//
//	BLOCK SEPARATOR — the spec writes "\n" between two block elements; this
//	renderer concatenates them with nothing in between.
//	SOFT LINE BREAK — the spec keeps the source newline inside a paragraph;
//	this renderer joins a block's lines with a single space before the inline
//	pass ever runs.
//	QUOTE SPELLING — the reference implementation writes a double quote in text
//	as "&quot;"; html.EscapeString, which is the only route author bytes take to
//	the output here, writes "&#34;". Same character, same escaping boundary,
//	different spelling of the same entity. It is the only entity that appears
//	anywhere in this section's expected output.
//
// A "\n" sitting between ">" and "<" is a block separator and is dropped; every
// other "\n" is a soft break and becomes a space. Over the 132 examples that
// rule is unambiguous: the only newlines matching ">\n<" are the paragraph
// boundaries of example 354, and every soft break in the section has a
// non-"<" byte after it or a non-">" byte before it.
//
// Nothing else is normalised, and every rewrite here is applied to the SPEC
// side only — this renderer's output is compared byte for byte as it comes. In
// particular no whitespace is collapsed and no tag is rewritten, because both
// would hide exactly the kind of defect this table exists to catch.
func specToRendererSpelling(h string) string {
	h = strings.TrimSuffix(h, "\n")
	h = strings.ReplaceAll(h, "&quot;", "&#34;")
	var b strings.Builder
	b.Grow(len(h))
	for i := 0; i < len(h); i++ {
		if h[i] != '\n' {
			b.WriteByte(h[i])
			continue
		}
		if i > 0 && h[i-1] == '>' && i+1 < len(h) && h[i+1] == '<' {
			continue
		}
		b.WriteByte(' ')
	}
	return b.String()
}

// ceilingCase is one example this renderer knowingly does not match: what it
// produces instead, and why that is a stated ceiling rather than a bug.
type ceilingCase struct {
	got string
	why string
}

// commonMarkEmphasisCeiling is the complete list of examples in the section
// where this renderer's output differs from the reference implementation's.
//
// It is short on purpose and every entry names a construct this package does
// not implement AT ALL, not an emphasis disagreement. If an entry ever needs to
// be added for a case where both sides render only emphasis, that is a defect
// in the flanking rule and not a new ceiling.
var commonMarkEmphasisCeiling = map[int]ceilingCase{
	475: {
		got: `<p><em>&lt;img src=&#34;foo&#34; title=&#34;</em>&#34;/&gt;</p>`,
		why: "raw inline HTML is a non-goal. The spec parses the img tag as an HTML inline, whose bytes are then not available to the delimiter scan; this renderer escapes the tag as ordinary text, so the '*' inside the title attribute is an ordinary delimiter and pairs with the one before the tag.",
	},
	476: {
		got: `<p><strong>&lt;a href=&#34;</strong>&#34;&gt;</p>`,
		why: "raw inline HTML is a non-goal, as in example 475: the '**' inside the href is ordinary text here, so it closes the run before the tag.",
	},
	477: {
		got: `<p><strong>&lt;a href=&#34;</strong>&#34;&gt;</p>`,
		why: "raw inline HTML is a non-goal, as in example 475. The '__' spelling of example 476, and it pairs for the same reason: both runs sit between ASCII punctuation, which is class 3 of the underscore exposure documented in markdown_inline.go.",
	},
}

// TestCommonMark_EmphasisSection runs every example of the spec's emphasis
// section through Render.
//
// A failure here is one of three things, in order of likelihood: the flanking
// rule is wrong, the pairing pass is wrong, or a construct outside emphasis
// changed under it. It is never "the spec is wrong" and it is never fixed by
// copying the got value into the table — commonMarkEmphasisCeiling is the only
// place a divergence may be recorded, and only with a reason that names a
// construct this package does not implement.
func TestCommonMark_EmphasisSection(t *testing.T) {
	if got, want := len(commonMarkEmphasisSpec), 132; got != want {
		t.Fatalf("emphasis section has %d examples, want %d: the table was edited or regenerated from a different spec", got, want)
	}
	seen := make(map[int]bool, len(commonMarkEmphasisSpec))
	for _, ex := range commonMarkEmphasisSpec {
		seen[ex.n] = true
		got := string(Render(ex.md))
		want := specToRendererSpelling(ex.html)
		if c, ok := commonMarkEmphasisCeiling[ex.n]; ok {
			if got == want {
				t.Errorf("example %d now MATCHES the spec but is still recorded as a ceiling; delete its entry from commonMarkEmphasisCeiling\n  markdown: %q", ex.n, ex.md)
				continue
			}
			if got != c.got {
				t.Errorf("example %d: recorded divergence changed\n  markdown: %q\n  spec:     %q\n  recorded: %q\n  got:      %q\n  ceiling:  %s", ex.n, ex.md, want, c.got, got, c.why)
			}
			continue
		}
		if got != want {
			t.Errorf("example %d\n  markdown: %q\n  want:     %q\n  got:      %q", ex.n, ex.md, want, got)
		}
	}
	for n := range commonMarkEmphasisCeiling {
		if !seen[n] {
			t.Errorf("commonMarkEmphasisCeiling records example %d, which is not in the emphasis section", n)
		}
	}
}
