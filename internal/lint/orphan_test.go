package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestOrphan(t *testing.T) {
	cases := []struct {
		name    string
		claims  []model.Claim
		wantIDs []string // claim IDs expected to be flagged
	}{
		{
			name: "passing: outgoing edge",
			claims: []model.Claim{
				{ID: "widget.internals.fields", RestsOn: []string{"widget.contract.overview"}},
				{ID: "widget.contract.overview"},
			},
			wantIDs: nil,
		},
		{
			name: "passing: incoming edge only",
			claims: []model.Claim{
				{ID: "widget.contract.overview"},
				{ID: "widget.internals.fields", RestsOn: []string{"widget.contract.overview"}},
			},
			wantIDs: nil,
		},
		{
			name: "failing: fully isolated claim",
			claims: []model.Claim{
				{ID: "widget.contract.overview", RestsOn: []string{"widget.internals.fields"}},
				{ID: "widget.internals.fields"},
				{ID: "widget.internals.lonely"},
			},
			wantIDs: []string{"widget.internals.lonely"},
		},
	}

	l := orphanLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check(tc.claims, nil)
			gotIDs := make([]string, 0, len(findings))
			for _, f := range findings {
				if f.Severity != SeverityWarning {
					t.Errorf("Severity = %q, want warning", f.Severity)
				}
				gotIDs = append(gotIDs, f.ClaimID)
			}
			if !equalSets(gotIDs, tc.wantIDs) {
				t.Fatalf("flagged IDs = %v, want %v", gotIDs, tc.wantIDs)
			}
		})
	}
}

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	am := map[string]bool{}
	for _, x := range a {
		am[x] = true
	}
	for _, x := range b {
		if !am[x] {
			return false
		}
	}
	return true
}
