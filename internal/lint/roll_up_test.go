package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRollUp(t *testing.T) {
	cases := []struct {
		name    string
		claims  []model.Claim
		wantErr bool
	}{
		{
			name: "passing: banner locked, all module-mates locked",
			claims: []model.Claim{
				{ID: "widget.status.banner", Module: "widget", Layout: model.LayoutBanner, Status: model.StatusLocked},
				{ID: "widget.contract.overview", Module: "widget", Status: model.StatusLocked},
			},
			wantErr: false,
		},
		{
			name: "passing: banner still draft while module-mates are draft",
			claims: []model.Claim{
				{ID: "widget.status.banner", Module: "widget", Layout: model.LayoutBanner, Status: model.StatusDraft},
				{ID: "widget.contract.overview", Module: "widget", Status: model.StatusDraft},
			},
			wantErr: false,
		},
		{
			name: "passing: banner with no module-mates",
			claims: []model.Claim{
				{ID: "widget.status.banner", Module: "widget", Layout: model.LayoutBanner, Status: model.StatusLocked},
			},
			wantErr: false,
		},
		{
			name: "failing: banner locked but a module-mate is still draft",
			claims: []model.Claim{
				{ID: "widget.status.banner", Module: "widget", Layout: model.LayoutBanner, Status: model.StatusLocked},
				{ID: "widget.contract.overview", Module: "widget", Status: model.StatusDraft},
			},
			wantErr: true,
		},
	}

	l := rollUpLint{}
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
