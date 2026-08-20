package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestSourceRefUndefined(t *testing.T) {
	one := []model.Source{wellFormedExternal()} // ref 1

	cases := []struct {
		name      string
		claim     model.Claim
		wantCount int
	}{
		{
			name:      "passing: every marker resolves",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "The ceiling is 100/min [1].", Sources: one},
			wantCount: 0,
		},
		{
			name:      "passing: a claim with no sources is not reading markers at all",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "The first element is argv[0] and the second is argv[1]."},
			wantCount: 0,
		},
		{
			name:      "passing: an index inside a code fence is not a marker",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "See [1].\n\n```\nargv[9]\n```\n", Sources: one},
			wantCount: 0,
		},
		{
			name:      "passing: an index inside an inline code span is not a marker",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "Read `argv[9]` after [1].", Sources: one},
			wantCount: 0,
		},
		{
			name:      "failing: a marker with no matching ref",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "The ceiling is 100/min [1], the burst is 500 [2].", Sources: one},
			wantCount: 1,
		},
		{
			name:      "failing: the same undefined marker twice is one finding",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "Per [2], and again per [2].", Sources: one},
			wantCount: 1,
		},
		{
			name:      "failing: two distinct undefined markers are two findings",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "Per [2] and [3].", Sources: one},
			wantCount: 2,
		},
		{
			name:      "failing: a marker spanning a soft line break still resolves as one token",
			claim:     model.Claim{ID: "widget.internals.fields", Body: "The ceiling is\n100/min [4].", Sources: one},
			wantCount: 1,
		},
	}

	l := sourceRefUndefinedLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check([]model.Claim{tc.claim}, nil)
			if len(findings) != tc.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantCount, findings)
			}
			for _, f := range findings {
				if f.Severity != SeverityError {
					t.Errorf("Severity = %q, want error", f.Severity)
				}
			}
		})
	}
}

// TestCitedSourceRefs pins the marker grammar itself, which is implemented
// twice (here and in the renderer) and can therefore drift. Each case below is
// a rule a change to either implementation has to keep.
func TestCitedSourceRefs(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []int
	}{
		{"plain marker", "As documented [1].", []int{1}},
		{"multi-digit", "As documented [42].", []int{42}},
		{"several, first-appearance order", "[3] then [1] then [3].", []int{3, 1}},
		{"no digits is not a marker", "See [the docs].", nil},
		{"mixed content is not a marker", "See [1a].", nil},
		{"spaces are not allowed", "See [ 1 ].", nil},
		{"a range is not a marker", "See [1-3].", nil},
		{"a list is not a marker", "See [1,2].", nil},
		{"fenced code is excluded", "```\nargv[1]\n```", nil},
		{"inline code is excluded", "Read `argv[1]`.", nil},
		{"a longer backtick run closes only on its own length", "``a `b` argv[1]``", nil},
		{"an unmatched backtick leaves the rest visible", "` argv[1]", []int{1}},
		{"a marker after a closed span is still seen", "`x` then [7].", []int{7}},
		{"empty body", "", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := citedSourceRefs(tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("citedSourceRefs(%q) = %v, want %v", tc.body, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("citedSourceRefs(%q) = %v, want %v", tc.body, got, tc.want)
				}
			}
		})
	}
}
