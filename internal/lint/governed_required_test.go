package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestGovernedRequiredLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: none-with-reason and doctrine-id both fine",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Governed: model.Governed{Type: "none", Reason: "fixture claim, not backed by any real doctrine"}},
				{ID: "widget.internals.fields", Governed: model.Governed{Type: "widget.doctrine.hub"}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: missing type, and none without reason",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Governed: model.Governed{}},
				{ID: "widget.internals.fields", Governed: model.Governed{Type: "none", Reason: "  "}},
			},
			wantFindings: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := GovernedRequiredLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
		})
	}
}
