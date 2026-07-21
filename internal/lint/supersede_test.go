package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestSupersede(t *testing.T) {
	cases := []struct {
		name       string
		claims     []model.Claim
		wantClaims []string
	}{
		{
			name: "passing: migrated_from names a claim that no longer exists (completed migration)",
			claims: []model.Claim{
				{ID: "a.contract.one", MigratedFrom: "legacy.contract.one"},
			},
			wantClaims: nil,
		},
		{
			name: "passing: no migrated_from set",
			claims: []model.Claim{
				{ID: "a.contract.one"},
			},
			wantClaims: nil,
		},
		{
			name: "passing: migrated_from is self-referential (not this lint's concern)",
			claims: []model.Claim{
				{ID: "a.contract.one", MigratedFrom: "a.contract.one"},
			},
			wantClaims: nil,
		},
		{
			name: "failing: migrated_from still resolves to a live claim",
			claims: []model.Claim{
				{ID: "a.contract.one", MigratedFrom: "b.contract.one"},
				{ID: "b.contract.one"},
			},
			wantClaims: []string{"a.contract.one"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Supersede{}.Check(tc.claims, nil)
			gotIDs := findingClaimIDs(got)
			assertStringSlicesEqual(t, gotIDs, tc.wantClaims)
		})
	}
}
