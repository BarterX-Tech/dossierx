package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestBodyEdgeHint(t *testing.T) {
	cfg := testConfig(t)

	cases := []struct {
		name    string
		claims  []model.Claim
		wantErr bool
	}{
		{
			name: "passing: prose mention is declared as rests_on",
			claims: []model.Claim{
				{
					ID:      "widget.internals.fields",
					Body:    "Builds on widget.contract.overview for the base shape.",
					RestsOn: []string{"widget.contract.overview"},
				},
				{ID: "widget.contract.overview"},
			},
			wantErr: false,
		},
		{
			name: "passing: prose mentions a claim that doesn't exist",
			claims: []model.Claim{
				{ID: "widget.internals.fields", Body: "See widget.contract.nonexistent for context."},
			},
			wantErr: false,
		},
		{
			name: "passing: mention is only inside a code fence (code-orphan's territory)",
			claims: []model.Claim{
				{ID: "widget.internals.fields", Body: "```\nwidget.contract.overview\n```"},
				{ID: "widget.contract.overview"},
			},
			wantErr: false,
		},
		{
			name: "failing: prose mentions a real claim with no declared edge",
			claims: []model.Claim{
				{ID: "widget.internals.fields", Body: "See widget.contract.overview for the base shape."},
				{ID: "widget.contract.overview"},
			},
			wantErr: true,
		},
	}

	l := bodyEdgeHintLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check(tc.claims, cfg)
			if got := len(findings) > 0; got != tc.wantErr {
				t.Fatalf("findings = %+v, wantErr=%v", findings, tc.wantErr)
			}
			for _, f := range findings {
				if f.Severity != SeverityWarning {
					t.Errorf("Severity = %q, want warning", f.Severity)
				}
			}
		})
	}
}
