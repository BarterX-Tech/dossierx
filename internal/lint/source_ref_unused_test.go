package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestSourceRefUnused(t *testing.T) {
	second := wellFormedExternal()
	second.Ref = 2
	second.Title = "Vendor burst ceiling note"

	cases := []struct {
		name      string
		claim     model.Claim
		wantCount int
	}{
		{
			name: "passing: the only source is cited",
			claim: model.Claim{
				ID:      "widget.internals.fields",
				Body:    "The ceiling is 100/min [1].",
				Sources: []model.Source{wellFormedExternal()},
			},
			wantCount: 0,
		},
		{
			name: "passing: both sources are cited",
			claim: model.Claim{
				ID:      "widget.internals.fields",
				Body:    "The ceiling is 100/min [1] and the burst is 500 [2].",
				Sources: []model.Source{wellFormedExternal(), second},
			},
			wantCount: 0,
		},
		{
			name: "failing: a declared source nothing cites",
			claim: model.Claim{
				ID:      "widget.internals.fields",
				Body:    "The ceiling is 100/min [1].",
				Sources: []model.Source{wellFormedExternal(), second},
			},
			wantCount: 1,
		},
		{
			name: "failing: a claim with sources and no body cites nothing at all",
			claim: model.Claim{
				ID:      "widget.internals.fields",
				Sources: []model.Source{wellFormedExternal(), second},
			},
			wantCount: 2,
		},
		{
			name: "failing: a marker that only appears inside code does not count as a citation",
			claim: model.Claim{
				ID:      "widget.internals.fields",
				Body:    "Read `argv[1]` for the shape.",
				Sources: []model.Source{wellFormedExternal()},
			},
			wantCount: 1,
		},
		{
			name: "failing: a ref shared by two entries is reported once, not twice",
			claim: model.Claim{
				ID:      "widget.internals.fields",
				Body:    "No markers here.",
				Sources: []model.Source{wellFormedExternal(), wellFormedExternal()},
			},
			wantCount: 1,
		},
	}

	l := sourceRefUnusedLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check([]model.Claim{tc.claim}, nil)
			if len(findings) != tc.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantCount, findings)
			}
			for _, f := range findings {
				if f.Severity != SeverityWarning {
					t.Errorf("Severity = %q, want warning — an uncited source is clutter, not falsehood", f.Severity)
				}
			}
		})
	}
}
