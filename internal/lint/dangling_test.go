package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestDanglingLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: all edges resolve",
			claims: []model.Claim{
				{
					ID:       "widget.contract.overview",
					RestsOn:  nil,
					Mirrors:  nil,
					Governed: model.Governed{Type: "none", Reason: "fixture"},
				},
				{
					ID:       "widget.internals.fields",
					RestsOn:  []string{"widget.contract.overview"},
					Mirrors:  []string{"widget.contract.overview"},
					Governed: model.Governed{Type: "widget.contract.overview"},
				},
			},
			wantFindings: 0,
		},
		{
			name: "failing: mirrors, rests_on, and governed_by all dangle",
			claims: []model.Claim{
				{
					ID:       "widget.internals.fields",
					RestsOn:  []string{"widget.contract.missing"},
					Mirrors:  []string{"widget.contract.also-missing"},
					Governed: model.Governed{Type: "widget.doctrine.ghost"},
				},
			},
			wantFindings: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := DanglingLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
		})
	}
}
