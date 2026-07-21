package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestBuildRoleRequiredLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: draft with no build_role, locked with a valid one",
			claims: []model.Claim{
				{ID: "widget.contract.draft-claim", Status: model.StatusDraft, BuildRole: ""},
				{ID: "widget.contract.locked-claim", Status: model.StatusLocked, BuildRole: model.BuildRoleBehavior},
				{ID: "widget.contract.locked-out-of-scope", Status: model.StatusLocked, BuildRole: model.BuildRoleOutOfScope},
			},
			wantFindings: 0,
		},
		{
			name: "failing: locked claim missing build_role, in a module that has adopted build_role elsewhere",
			claims: []model.Claim{
				{ID: "widget.contract.locked-no-role", Module: "widget", Status: model.StatusLocked, BuildRole: ""},
				{ID: "widget.contract.other-locked", Module: "widget", Status: model.StatusLocked, BuildRole: model.BuildRoleBehavior},
			},
			wantFindings: 1,
		},
		{
			name: "passing: locked claim missing build_role, in a module that has never adopted build_role at all (zero-behavior-change for non-adopting projects)",
			claims: []model.Claim{
				{ID: "legacy.contract.locked-no-role", Module: "legacy", Status: model.StatusLocked, BuildRole: ""},
				{ID: "legacy.contract.also-locked", Module: "legacy", Status: model.StatusLocked, BuildRole: ""},
			},
			wantFindings: 0,
		},
		{
			name: "failing: invalid build_role value, draft or locked",
			claims: []model.Claim{
				{ID: "widget.contract.draft-bad-role", Status: model.StatusDraft, BuildRole: model.BuildRole("not-a-real-phase")},
				{ID: "widget.contract.locked-bad-role", Status: model.StatusLocked, BuildRole: model.BuildRole("not-a-real-phase")},
			},
			wantFindings: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := BuildRoleRequiredLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "build-role-required-for-locked" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
			}
		})
	}
}
