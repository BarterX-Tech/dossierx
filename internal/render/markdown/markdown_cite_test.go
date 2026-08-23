package markdown

import (
	"strings"
	"testing"
)

// markdown_cite_test.go pins the citation marker's recognition rules — the
// SAME list internal/lint's markdown_scan.go mirrors. A change here that is
// not made there means a claim renders one way and lints another, which is the
// failure mode that file's doc comment exists to warn about.

// citeTestAnchor is the anchor prefix these tests build capabilities with. It
// is a fixed literal rather than the production shape so the assertions record
// this package's contract — "the prefix is prepended to the decimal ref and to
// nothing else" — independently of how components spells a claim's anchor.
const citeTestAnchor = "widget.contract.demo-source-"

func citeTestPolicy(refs ...int) Citations {
	return NewCitations(citeTestAnchor, refs)
}

// TestRenderClaimBody_CitationMarkerRules is the recognition table. Each case
// states the body, the refs the claim declares, and the exact HTML expected —
// so a rule change shows up as a diff on the rendered bytes, not on a
// paraphrase of them.
func TestRenderClaimBody_CitationMarkerRules(t *testing.T) {
	link := func(n string) string {
		return `<a class="claim-cite" href="#` + citeTestAnchor + n + `">[` + n + `]</a>`
	}

	cases := []struct {
		name string
		body string
		refs []int
		want string
	}{
		{
			name: "resolved ref becomes an anchor",
			body: "The retry budget is three [1].",
			refs: []int{1},
			want: "<p>The retry budget is three " + link("1") + ".</p>",
		},
		{
			name: "multi digit ref",
			body: "see [12]",
			refs: []int{12},
			want: "<p>see " + link("12") + "</p>",
		},
		{
			name: "two markers in one sentence",
			body: "both [1] and [2]",
			refs: []int{1, 2},
			want: "<p>both " + link("1") + " and " + link("2") + "</p>",
		},
		{
			name: "a claim with no sources renders every marker literally",
			body: "The retry budget is three [1].",
			refs: nil,
			want: "<p>The retry budget is three [1].</p>",
		},
		{
			name: "an undefined ref stays literal — source-ref-undefined reports it",
			body: "see [7]",
			refs: []int{1, 2},
			want: "<p>see [7]</p>",
		},
		{
			name: "an array index is not a citation",
			body: "read array[0] first",
			refs: []int{1},
			want: "<p>read array[0] first</p>",
		},
		{
			name: "a leading zero is not a marker: one citation, one spelling",
			body: "see [01]",
			refs: []int{1},
			want: "<p>see [01]</p>",
		},
		{
			name: "no space inside the brackets",
			body: "see [ 1 ]",
			refs: []int{1},
			want: "<p>see [ 1 ]</p>",
		},
		{
			name: "no sign inside the brackets",
			body: "see [+1]",
			refs: []int{1},
			want: "<p>see [+1]</p>",
		},
		{
			name: "an unclosed bracket run is literal",
			body: "see [1 and [2",
			refs: []int{1, 2},
			want: "<p>see [1 and [2</p>",
		},
		{
			name: "the link grammar wins over the marker",
			body: "see [1](notes/a.md)",
			refs: []int{1},
			want: `<p>see <a href="notes/a.md">1</a></p>`,
		},
		{
			// The link grammar wins even when it loses: a complete link whose
			// scheme urlsafe refuses is inert literal text, and the marker does
			// NOT get a second chance at the bytes it left behind.
			name: "a refused link is literal, not a marker",
			body: "see [1](javascript:x)",
			refs: []int{1},
			want: "<p>see [1](javascript:x)</p>",
		},
		{
			// An image run is consumed whole by "!" before "[" is reached, so
			// the marker never sees it either.
			name: "an image run is not a marker",
			body: "see ![1](assets/a.png)",
			refs: []int{1},
			want: "<p>see ![1](assets/a.png)</p>",
		},
		{
			name: "a code span's interior is not prose",
			body: "the token `[1]` is literal",
			refs: []int{1},
			want: "<p>the token <code>[1]</code> is literal</p>",
		},
		{
			name: "an escaped bracket cannot open a marker",
			body: `see \[1]`,
			refs: []int{1},
			want: "<p>see [1]</p>",
		},
		{
			name: "markers resolve inside a list item",
			body: "- first [1]",
			refs: []int{1},
			want: "<ul><li>first " + link("1") + "</li></ul>",
		},
		{
			name: "markers resolve inside a blockquote",
			body: "> quoted [1]",
			refs: []int{1},
			want: "<blockquote><p>quoted " + link("1") + "</p></blockquote>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(RenderClaimBody(tc.body, "", citeTestPolicy(tc.refs...)))
			if got != tc.want {
				t.Errorf("RenderClaimBody(%q) =\n  %q\nwant\n  %q", tc.body, got, tc.want)
			}
		})
	}
}

// TestRenderClaimBody_MarkerNeverRendersInsideAFence is separate from the table
// because a fence is the one surface whose exemption is STRUCTURAL: fenced
// content never reaches the inline pass at all, so there is nothing in the
// citation scanner that could grant or refuse it. Pinning it here is what would
// catch a future change that started running fence content through the inline
// pass "for syntax highlighting".
func TestRenderClaimBody_MarkerNeverRendersInsideAFence(t *testing.T) {
	body := "```\nsee [1]\n```"
	got := string(RenderClaimBody(body, "", citeTestPolicy(1)))
	if strings.Contains(got, "claim-cite") {
		t.Fatalf("a fenced [1] rendered as a citation anchor: %q", got)
	}
	if !strings.Contains(got, "[1]") {
		t.Fatalf("the fenced text lost its literal marker: %q", got)
	}
}

// TestRenderClaimBody_CitationsAreClaimBodyOnly holds the surface boundary that
// makes this construct safe to add: the two entry points a comment body and a
// table cell reach have no way to be told about citations at all, so a "[1]" in
// reviewer-authored text is the literal text it has always been no matter what
// the claim beside it declares.
func TestRenderClaimBody_CitationsAreClaimBodyOnly(t *testing.T) {
	const body = "see [1]"
	if got := string(Render(body)); got != "<p>see [1]</p>" {
		t.Errorf("Render(%q) = %q, want the marker literal on a comment surface", body, got)
	}
	if got := string(RenderInline(body)); got != "see [1]" {
		t.Errorf("RenderInline(%q) = %q, want the marker literal on the inline surface", body, got)
	}
}

// TestNewCitations_ZeroValueOnAnEmptyCapability pins the inversion the whole
// zero-cost contract rests on: "no refs" and "no anchor" are both the absence,
// not two half-enabled states.
func TestNewCitations_ZeroValueOnAnEmptyCapability(t *testing.T) {
	const body = "see [1]"
	for _, tc := range []struct {
		name  string
		cites Citations
	}{
		{"zero value", Citations{}},
		{"no refs", NewCitations(citeTestAnchor, nil)},
		{"no anchor", NewCitations("", []int{1})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(RenderClaimBody(body, "", tc.cites)); got != "<p>see [1]</p>" {
				t.Errorf("RenderClaimBody with %s capability = %q, want the marker literal", tc.name, got)
			}
		})
	}
}

// TestNewCitations_AnchorIsEscapedOnce holds the escaping boundary at the one
// place this construct could leak: the anchor is claim-derived rather than
// prose, so nothing else in the pass would escape it.
func TestNewCitations_AnchorIsEscapedOnce(t *testing.T) {
	got := string(RenderClaimBody("see [1]", "", NewCitations(`a"><script>x`, []int{1})))
	if strings.Contains(got, "<script>") {
		t.Fatalf("an anchor prefix escaped its attribute: %q", got)
	}
	if !strings.Contains(got, "&#34;&gt;&lt;script&gt;") {
		t.Fatalf("expected the anchor prefix escaped in attribute context, got: %q", got)
	}
}

// TestRenderClaimBody_MarkerDigitCeiling pins the bound that keeps the scan
// constant-time per bracket and keeps the parse away from overflow.
func TestRenderClaimBody_MarkerDigitCeiling(t *testing.T) {
	tooLong := strings.Repeat("1", maxCiteDigits+1)
	got := string(RenderClaimBody("see ["+tooLong+"]", "", citeTestPolicy(1)))
	if strings.Contains(got, "claim-cite") {
		t.Fatalf("a %d-digit run was recognized as a marker: %q", maxCiteDigits+1, got)
	}
}
