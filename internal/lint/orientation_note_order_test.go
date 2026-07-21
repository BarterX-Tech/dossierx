package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestOrientationNoteOrder(t *testing.T) {
	note := func(id, module, facet string, order int) model.Claim {
		return model.Claim{ID: id, Module: module, Facet: facet, Kind: model.KindOrientationNote, Layout: model.LayoutBanner, Section: "s", Order: order}
	}
	fact := func(id, module, facet string, order int) model.Claim {
		return model.Claim{ID: id, Module: module, Facet: facet, Section: "s", Order: order}
	}

	cases := []struct {
		name    string
		claims  []model.Claim
		wantIDs []string
	}{
		{
			name: "passing: notes strictly before facts",
			claims: []model.Claim{
				note("w.contract.note-a", "w", "contract", 1),
				note("w.contract.note-b", "w", "contract", 2),
				fact("w.contract.rule", "w", "contract", 3),
			},
			wantIDs: nil,
		},
		{
			name: "passing: overview-facet notes never compared against anything (no siblings in that facet)",
			claims: []model.Claim{
				{ID: "w.overview.router", Module: "w", Facet: "overview", Layout: model.LayoutBanner},
			},
			wantIDs: nil,
		},
		{
			name: "passing: separate facets don't interfere",
			claims: []model.Claim{
				note("w.contract.note", "w", "contract", 1),
				fact("w.internals.rule", "w", "internals", 1),
			},
			wantIDs: nil,
		},
		{
			name: "failing: a fact claim sorts ahead of an orientation-note in the same facet",
			claims: []model.Claim{
				fact("w.contract.rule", "w", "contract", 1),
				note("w.contract.note", "w", "contract", 2),
			},
			wantIDs: []string{"w.contract.note"},
		},
	}

	l := OrientationNoteOrderLint{}
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
