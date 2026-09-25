package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestRestsOnRequiredLint pins the shape rule (NIT-24): every claim — module
// or project — carries either a list of claim ids or {none: true, reason}.
// Nothing at all and none without a reason are the two refusals, each with
// its own message.
func TestRestsOnRequiredLint(t *testing.T) {
	cases := []struct {
		name    string
		claim   model.Claim
		wantN   int
		message string
	}{
		{
			name:  "a list of targets is complete",
			claim: model.Claim{ID: "widget.contract.a", RestsOn: model.RestsOnIDs("widget.contract.b")},
		},
		{
			name:  "none with a reason is complete",
			claim: model.Claim{ID: "widget.contract.a", RestsOn: model.RestsNone("the roof above it is the constitution")},
		},
		{
			name:  "a project claim follows the same rule: none with a reason",
			claim: model.Claim{ID: "project.scope", Scope: model.ScopeProject, RestsOn: model.RestsNone("nothing above the roof")},
		},
		{
			name:  "a project claim with targets is complete",
			claim: model.Claim{ID: "project.retention", Scope: model.ScopeProject, RestsOn: model.RestsOnIDs("widget.contract.a")},
		},
		{
			name:    "rests_on absent",
			claim:   model.Claim{ID: "widget.contract.a"},
			wantN:   1,
			message: "rests_on is required",
		},
		{
			name:    "a project claim with rests_on absent",
			claim:   model.Claim{ID: "project.scope", Scope: model.ScopeProject},
			wantN:   1,
			message: "rests_on is required",
		},
		{
			name:    "none without a reason",
			claim:   model.Claim{ID: "widget.contract.a", RestsOn: model.RestsOn{None: true}},
			wantN:   1,
			message: "rests_on.reason is required when rests_on.none is true",
		},
		{
			name:    "none with a blank reason",
			claim:   model.Claim{ID: "widget.contract.a", RestsOn: model.RestsOn{None: true, Reason: "   "}},
			wantN:   1,
			message: "rests_on.reason is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := RestsOnRequiredLint{}.Check([]model.Claim{tc.claim}, nil)
			if len(findings) != tc.wantN {
				t.Fatalf("got %d finding(s), want %d: %+v", len(findings), tc.wantN, findings)
			}
			for _, f := range findings {
				if f.LintName != "rests-on-required" || f.ClaimID != tc.claim.ID {
					t.Fatalf("finding names %s on %s", f.LintName, f.ClaimID)
				}
				if !strings.Contains(f.Message, tc.message) {
					t.Fatalf("message %q lacks %q", f.Message, tc.message)
				}
			}
		})
	}
}
