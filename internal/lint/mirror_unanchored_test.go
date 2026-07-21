package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestMirrorUnanchored(t *testing.T) {
	cases := []struct {
		name       string
		claims     []model.Claim
		wantClaims []string // ClaimIDs expected to have a finding, in order
	}{
		{
			name: "passing: mirror target exists",
			claims: []model.Claim{
				{ID: "a.contract.one", Mirrors: []string{"b.contract.one"}},
				{ID: "b.contract.one"},
			},
			wantClaims: nil,
		},
		{
			name: "passing: no mirrors at all",
			claims: []model.Claim{
				{ID: "a.contract.one"},
			},
			wantClaims: nil,
		},
		{
			name: "failing: mirror target missing",
			claims: []model.Claim{
				{ID: "a.contract.one", Mirrors: []string{"nope.contract.one"}},
			},
			wantClaims: []string{"a.contract.one"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MirrorUnanchored{}.Check(tc.claims, nil)
			gotIDs := findingClaimIDs(got)
			assertStringSlicesEqual(t, gotIDs, tc.wantClaims)
		})
	}
}

// findingClaimIDs and assertStringSlicesEqual are small shared test
// helpers used across the lint package's table-driven tests.
func findingClaimIDs(findings []Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	ids := make([]string, len(findings))
	for i, f := range findings {
		ids[i] = f.ClaimID
	}
	return ids
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
