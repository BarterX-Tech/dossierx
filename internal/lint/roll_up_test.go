package lint

import (
	"strings"
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
				// WARNING, not error: a project-wide error-severity roll-up
				// deadlocked every module in the ordinary locked-banner +
				// two-drafts shape (see this lint's file comment). The refusal
				// lives in cmd/dossierx's lock gate, scoped to the banner.
				if f.Severity != SeverityWarning {
					t.Errorf("Severity = %q, want warning", f.Severity)
				}
				// Both ends are named: the banner the finding hangs off, and the
				// sibling actually holding it open — the one the caller acts on.
				if !strings.Contains(f.Message, "widget.status.banner") || !strings.Contains(f.Message, "widget.contract.overview") {
					t.Errorf("message must name the banner AND the blocking sibling, got %q", f.Message)
				}
			}
		})
	}
}
