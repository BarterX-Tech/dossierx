package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestMirrorReciprocal(t *testing.T) {
	cases := []struct {
		name       string
		claims     []model.Claim
		wantClaims []string
	}{
		{
			name: "passing: mirror is reciprocated",
			claims: []model.Claim{
				{ID: "a.contract.one", Mirrors: []string{"b.contract.one"}},
				{ID: "b.contract.one", Mirrors: []string{"a.contract.one"}},
			},
			wantClaims: nil,
		},
		{
			name: "passing: unresolved mirror target is skipped (mirror-unanchored's concern)",
			claims: []model.Claim{
				{ID: "a.contract.one", Mirrors: []string{"nope.contract.one"}},
			},
			wantClaims: nil,
		},
		{
			name: "failing: mirror not reciprocated",
			claims: []model.Claim{
				{ID: "a.contract.one", Mirrors: []string{"b.contract.one"}},
				{ID: "b.contract.one"},
			},
			wantClaims: []string{"a.contract.one"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MirrorReciprocal{}.Check(tc.claims, nil)
			gotIDs := findingClaimIDs(got)
			assertStringSlicesEqual(t, gotIDs, tc.wantClaims)
		})
	}
}
