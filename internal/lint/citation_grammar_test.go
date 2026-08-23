package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// TestCitationGrammarMatchesTheRenderer is the tripwire for the one hazard
// this feature has that no single package can see: the "[n]" citation marker
// is recognized by TWO independent scanners — this package's, which decides
// what the lints report, and internal/render/markdown's, which decides what a
// reader actually sees — and nothing but agreement between them makes either
// one meaningful.
//
// The drift is silent in both directions and neither is loud enough to notice
// by hand:
//
//   - If this scanner is WIDER on syntax, a body citing "[01]" registers ref 1
//     as cited, source-ref-unused stays quiet, and the viewer renders literal
//     text. The reader gets a source nothing points at and no finding says so.
//   - If it is NARROWER, the viewer links a marker this side never saw, so
//     source-ref-undefined cannot report a citation that resolves to nothing.
//
// The test drives the REAL renderer rather than re-describing its rules, so a
// change to markdown_cite.go's grammar fails here rather than being mirrored
// by hand and hoped about.
//
// The one intended asymmetry is resolution, not syntax: the renderer treats an
// unresolvable ref as literal text because it has no way to report anything,
// while this side must collect it so source-ref-undefined can name it. Every
// case below therefore declares the ref it uses, which holds resolution
// constant and leaves syntax as the only variable.
func TestCitationGrammarMatchesTheRenderer(t *testing.T) {
	cases := []struct {
		name string
		body string
		ref  int
	}{
		{"plain marker", "the vendor documents no such property [1]", 1},
		{"multi digit", "see [42] for the full table", 42},
		{"adjacent markers", "both pages agree [1][2]", 1},
		{"start of body", "[1] is the whole basis for this", 1},
		{"leading zero is not a marker", "the array index [01] is not a citation", 1},
		{"long leading zero run", "not a citation [0000000001] at all", 1},
		{"zero never resolves", "argv[0] is the program name", 1},
		{"inside a code span", "call `rows[1]` to read it", 1},
		{"space inside brackets", "not a marker [ 1 ]", 1},
		{"comma list is not a marker", "not a range [1,2]", 1},
		{"dash range is not a marker", "not a range [1-2]", 1},
		{"link syntax wins", "see [1](https://example.invalid/a) instead", 1},
		{"image syntax wins", "see ![1](https://example.invalid/a.png) instead", 1},
		{"nested brackets", "not a marker [[1]]", 1},
		{"unclosed bracket", "not a marker [1 and on", 1},
		{"letters inside", "not a marker [a1]", 1},
		{"marker at end of body", "the whole basis for this [1]", 1},
		{"marker inside a fenced block", "```\nrows[1]\n```\n", 1},
		{"double digit with trailing text", "see [12]th", 12},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lintSaw := false
			for _, got := range citedSourceRefs(tc.body) {
				if got == tc.ref {
					lintSaw = true
				}
			}

			cites := markdown.NewCitations("claim-source-", []int{tc.ref})
			rendered := string(markdown.RenderClaimBody(tc.body, markdown.AssetPrefix(""), cites))
			rendererLinked := strings.Contains(rendered, `class="claim-cite"`)

			if lintSaw != rendererLinked {
				t.Errorf("the two citation scanners disagree about %q\n"+
					"  lint collected ref %d: %v\n"+
					"  renderer emitted a citation link: %v\n"+
					"  rendered: %s\n\n"+
					"These must agree on SYNTAX. The lint reports what the reader sees, so a marker one side\n"+
					"recognizes and the other does not is a finding about text that renders differently, or a\n"+
					"link nothing checks. Fix internal/lint/helpers.go's citationRef or\n"+
					"internal/render/markdown/markdown_cite.go's match — in the same commit.",
					tc.body, tc.ref, lintSaw, rendererLinked, rendered)
			}
		})
	}
}

// TestCitationGrammarFencedBlocksAreNotProse pins the exclusion separately,
// because a fenced block never reaches the renderer's inline pass at all
// (there is no marker to compare against) and only this side can be asked
// about it directly. Claim bodies are full of "argv[0]" and "rows[2]" inside
// code, and a citation lint that read those as citations would report an
// undefined ref on the most ordinary technical prose in the corpus.
func TestCitationGrammarFencedBlocksAreNotProse(t *testing.T) {
	body := "the real citation is [1]\n\n```go\nfmt.Println(rows[2], rows[3])\n```\n"
	got := citedSourceRefs(body)
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("citedSourceRefs(%q) = %v, want [1]: bracketed digits inside a fenced block are code, not citations", body, got)
	}
}
