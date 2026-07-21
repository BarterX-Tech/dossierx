package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestOrientationNoteShape(t *testing.T) {
	cases := []struct {
		name    string
		claims  []model.Claim
		wantIDs []string
	}{
		{
			name: "passing: explicit orientation-note with module + banner layout",
			claims: []model.Claim{
				{ID: "w.contract.how-to-read", Module: "w", Facet: "contract", Kind: model.KindOrientationNote, Layout: model.LayoutBanner},
			},
			wantIDs: nil,
		},
		{
			name: "passing: overview facet, kind unset, banner layout, module set",
			claims: []model.Claim{
				{ID: "w.overview.router", Module: "w", Facet: "overview", Layout: model.LayoutBanner},
			},
			wantIDs: nil,
		},
		{
			name: "passing: ordinary fact claim untouched",
			claims: []model.Claim{
				{ID: "w.contract.rule", Module: "w", Facet: "contract", Layout: model.LayoutCard},
			},
			wantIDs: nil,
		},
		{
			name: "failing: invalid kind value",
			claims: []model.Claim{
				{ID: "w.contract.bad", Module: "w", Facet: "contract", Kind: model.Kind("nonsense"), Layout: model.LayoutBanner},
			},
			wantIDs: []string{"w.contract.bad"},
		},
		{
			name: "failing: overview facet explicitly claims kind: fact",
			claims: []model.Claim{
				{ID: "w.overview.contradiction", Module: "w", Facet: "overview", Kind: model.KindFact, Layout: model.LayoutBanner},
			},
			wantIDs: []string{"w.overview.contradiction"},
		},
		{
			name: "failing: orientation-note with no module",
			claims: []model.Claim{
				{ID: "w.contract.no-module", Facet: "contract", Kind: model.KindOrientationNote, Layout: model.LayoutBanner},
			},
			wantIDs: []string{"w.contract.no-module"},
		},
		{
			name: "failing: orientation-note not laid out as banner",
			claims: []model.Claim{
				{ID: "w.contract.wrong-layout", Module: "w", Facet: "contract", Kind: model.KindOrientationNote, Layout: model.LayoutCard},
			},
			wantIDs: []string{"w.contract.wrong-layout"},
		},
	}

	l := OrientationNoteShapeLint{}
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
