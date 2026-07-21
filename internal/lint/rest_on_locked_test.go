package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRestOnLockedLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: locked claim only rests_on other locked claims",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Status: model.StatusLocked},
				{ID: "widget.internals.fields", Status: model.StatusLocked, RestsOn: []string{"widget.contract.overview"}},
			},
			wantFindings: 0,
		},
		{
			name: "passing: draft claim may rest on a draft claim",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Status: model.StatusDraft},
				{ID: "widget.internals.fields", Status: model.StatusDraft, RestsOn: []string{"widget.contract.overview"}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: locked claim rests_on a draft claim",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Status: model.StatusDraft},
				{ID: "widget.internals.fields", Status: model.StatusLocked, RestsOn: []string{"widget.contract.overview"}},
			},
			wantFindings: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := RestOnLockedLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
		})
	}
}
