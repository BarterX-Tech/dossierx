package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestValidatedOnMissing(t *testing.T) {
	cases := []struct {
		name    string
		claims  []model.Claim
		wantErr bool
	}{
		{
			name: "passing: governed_by type none",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Governed: model.Governed{Type: "none", Reason: "fixture"}},
			},
			wantErr: false,
		},
		{
			name: "passing: governed_by references existing doctrine claim",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Governed: model.Governed{Type: "widget.doctrine.hub"}},
				{ID: "widget.doctrine.hub"},
			},
			wantErr: false,
		},
		{
			name: "failing: governed_by references missing claim",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Governed: model.Governed{Type: "widget.doctrine.missing"}},
			},
			wantErr: true,
		},
	}

	l := validatedOnMissingLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check(tc.claims, nil)
			if got := len(findings) > 0; got != tc.wantErr {
				t.Fatalf("findings = %+v, wantErr=%v", findings, tc.wantErr)
			}
			for _, f := range findings {
				if f.Severity != SeverityError {
					t.Errorf("Severity = %q, want error", f.Severity)
				}
			}
		})
	}
}
