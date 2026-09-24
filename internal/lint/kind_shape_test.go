package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestKindShape(t *testing.T) {
	cases := []struct {
		name    string
		claims  []model.Claim
		wantIDs []string
	}{
		{
			name: "passing: omitted kind",
			claims: []model.Claim{
				{ID: "w.contract.rule", Module: "w", Facet: "contract", Layout: model.LayoutCard},
			},
			wantIDs: nil,
		},
		{
			name: "passing: explicit fact",
			claims: []model.Claim{
				{ID: "w.contract.rule", Module: "w", Facet: "contract", Kind: model.KindFact, Layout: model.LayoutCard},
			},
			wantIDs: nil,
		},
		{
			name: "failing: retired orientation-note",
			claims: []model.Claim{
				{ID: "w.contract.note", Module: "w", Facet: "contract", Kind: model.Kind("orientation-note"), Layout: model.LayoutBanner},
			},
			wantIDs: []string{"w.contract.note"},
		},
		{
			name: "failing: unknown kind",
			claims: []model.Claim{
				{ID: "w.contract.bad", Module: "w", Facet: "contract", Kind: model.Kind("nonsense"), Layout: model.LayoutCard},
			},
			wantIDs: []string{"w.contract.bad"},
		},
	}

	l := KindShapeLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check(tc.claims, nil)
			gotIDs := make([]string, 0, len(findings))
			for _, f := range findings {
				gotIDs = append(gotIDs, f.ClaimID)
			}
			if !equalSets(gotIDs, tc.wantIDs) {
				t.Fatalf("flagged IDs = %v, want %v (findings: %#v)", gotIDs, tc.wantIDs, findings)
			}
		})
	}
}
