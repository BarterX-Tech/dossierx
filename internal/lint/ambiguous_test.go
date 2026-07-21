package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestAmbiguousLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: all ids unique",
			claims: []model.Claim{
				{ID: "widget.contract.overview"},
				{ID: "widget.internals.fields"},
			},
			wantFindings: 0,
		},
		{
			name: "failing: duplicate id reported for both claims",
			claims: []model.Claim{
				{ID: "widget.contract.overview"},
				{ID: "widget.contract.overview"},
				{ID: "widget.internals.fields"},
			},
			wantFindings: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := AmbiguousLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
		})
	}
}
