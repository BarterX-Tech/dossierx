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
				{ID: "widget.contract.overview", RestsOn: model.RestsNone("root")},
				{ID: "widget.internals.fields", RestsOn: model.RestsOnIDs("widget.contract.overview")},
			},
			wantFindings: 0,
		},
		{
			name: "failing: rests_on dangles",
			claims: []model.Claim{
				{ID: "widget.internals.fields", RestsOn: model.RestsOnIDs("widget.contract.missing")},
			},
			wantFindings: 1,
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
