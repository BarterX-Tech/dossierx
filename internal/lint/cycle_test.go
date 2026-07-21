package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestCycleLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: acyclic rests_on chain",
			claims: []model.Claim{
				{ID: "widget.contract.overview"},
				{ID: "widget.internals.fields", RestsOn: []string{"widget.contract.overview"}},
				{ID: "widget.internals.detail", RestsOn: []string{"widget.internals.fields"}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: three-claim rests_on cycle",
			claims: []model.Claim{
				{ID: "widget.a.one", RestsOn: []string{"widget.b.two"}},
				{ID: "widget.b.two", RestsOn: []string{"widget.c.three"}},
				{ID: "widget.c.three", RestsOn: []string{"widget.a.one"}},
			},
			wantFindings: 3,
		},
		{
			name: "failing: claim rests_on itself is a degenerate one-claim cycle",
			claims: []model.Claim{
				{ID: "widget.contract.self", RestsOn: []string{"widget.contract.self"}},
			},
			wantFindings: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := CycleLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
		})
	}
}
