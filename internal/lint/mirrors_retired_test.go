package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestMirrorsRetired(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: no mirrors field",
			claims: []model.Claim{
				{ID: "a.contract.one", RestsOn: []string{"a.contract.two"}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: leftover mirrors list",
			claims: []model.Claim{
				{ID: "a.contract.one", Mirrors: []string{"a.contract.two"}},
			},
			wantFindings: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MirrorsRetired{}.Check(tc.claims, nil)
			if len(got) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tc.wantFindings, got)
			}
		})
	}
}
