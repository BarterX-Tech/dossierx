package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestMirrorMismatch(t *testing.T) {
	cases := []struct {
		name       string
		claims     []model.Claim
		wantClaims []string
	}{
		{
			name: "passing: mirrored claims have identical content",
			claims: []model.Claim{
				{ID: "a.contract.one", Layout: model.LayoutCard, Body: "same text", Mirrors: []string{"b.contract.one"}},
				{ID: "b.contract.one", Layout: model.LayoutCard, Body: "same text", Mirrors: []string{"a.contract.one"}},
			},
			wantClaims: nil,
		},
		{
			name: "passing: unresolved mirror target is skipped (mirror-unanchored's concern)",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "text", Mirrors: []string{"nope.contract.one"}},
			},
			wantClaims: nil,
		},
		{
			name: "failing: mirrored claims have divergent body",
			claims: []model.Claim{
				{ID: "a.contract.one", Layout: model.LayoutCard, Body: "text A", Mirrors: []string{"b.contract.one"}},
				{ID: "b.contract.one", Layout: model.LayoutCard, Body: "text B", Mirrors: []string{"a.contract.one"}},
			},
			wantClaims: []string{"a.contract.one", "b.contract.one"},
		},
		{
			name: "failing: mirrored claims have divergent layout",
			claims: []model.Claim{
				{ID: "a.contract.one", Layout: model.LayoutCard, Body: "same", Mirrors: []string{"b.contract.one"}},
				{ID: "b.contract.one", Layout: model.LayoutBanner, Body: "same", Mirrors: []string{"a.contract.one"}},
			},
			wantClaims: []string{"a.contract.one", "b.contract.one"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MirrorMismatch{}.Check(tc.claims, nil)
			gotIDs := findingClaimIDs(got)
			assertStringSlicesEqual(t, gotIDs, tc.wantClaims)
		})
	}
}
