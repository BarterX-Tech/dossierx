package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestStatusShapeLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: draft and locked are the only legal values",
			claims: []model.Claim{
				{ID: "widget.contract.a", Status: model.StatusDraft},
				{ID: "widget.contract.b", Status: model.StatusLocked},
			},
			wantFindings: 0,
		},
		{
			name: "failing: an out-of-enum status value",
			claims: []model.Claim{
				{ID: "widget.contract.a", Status: model.Status("published")},
			},
			wantFindings: 1,
		},
		{
			name: "failing: an empty/missing status",
			claims: []model.Claim{
				{ID: "widget.contract.a", Status: ""},
			},
			wantFindings: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := statusShapeLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "status-shape" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
				if f.Severity != SeverityError {
					t.Fatalf("expected SeverityError, got %q", f.Severity)
				}
			}
		})
	}
}

// TestStatusShapeLint_Registered ensures the lint is wired into the suite
// (its init() appended it to Registry) so RunAll actually runs it.
func TestStatusShapeLint_Registered(t *testing.T) {
	found := false
	for _, l := range Registry {
		if l.Name() == "status-shape" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("status-shape lint is not registered in the lint Registry")
	}
}
