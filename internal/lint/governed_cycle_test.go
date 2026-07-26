package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestGovernedCycleLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			// The terminating case: every chain bottoms out at type: none.
			name: "passing: governed_by chain ends at none",
			claims: []model.Claim{
				{ID: "widget.doctrine.root", Governed: model.Governed{Type: "none", Reason: "the root doctrine claim"}},
				{ID: "widget.contract.overview", Governed: model.Governed{Type: "widget.doctrine.root"}},
				{ID: "widget.internals.detail", Governed: model.Governed{Type: "widget.contract.overview"}},
			},
			wantFindings: 0,
		},
		{
			// A governed_by naming an id nothing defines is
			// validated-on-missing's (and dangling's) finding, not a cycle.
			name: "passing: unresolved governed_by target is another lint's concern",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Governed: model.Governed{Type: "widget.doctrine.ghost"}},
			},
			wantFindings: 0,
		},
		{
			// The hole this rule closes: mutual governance passed the entire
			// registry before it existed.
			name: "failing: two claims govern each other",
			claims: []model.Claim{
				{ID: "widget.contract.a", Governed: model.Governed{Type: "widget.contract.b"}},
				{ID: "widget.contract.b", Governed: model.Governed{Type: "widget.contract.a"}},
			},
			wantFindings: 2,
		},
		{
			name: "failing: three-claim governed_by cycle",
			claims: []model.Claim{
				{ID: "widget.a.one", Governed: model.Governed{Type: "widget.b.two"}},
				{ID: "widget.b.two", Governed: model.Governed{Type: "widget.c.three"}},
				{ID: "widget.c.three", Governed: model.Governed{Type: "widget.a.one"}},
			},
			wantFindings: 3,
		},
		{
			name: "failing: claim governed by itself is a degenerate one-claim cycle",
			claims: []model.Claim{
				{ID: "widget.contract.self", Governed: model.Governed{Type: "widget.contract.self"}},
			},
			wantFindings: 1,
		},
		{
			// A rests_on cycle must not leak into this graph, and vice versa:
			// the two walks are over different edges entirely.
			name: "passing: a rests_on cycle is not a governed_by cycle",
			claims: []model.Claim{
				{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b"}, Governed: model.Governed{Type: "none", Reason: "x"}},
				{ID: "widget.contract.b", RestsOn: []string{"widget.contract.a"}, Governed: model.Governed{Type: "none", Reason: "x"}},
			},
			wantFindings: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := GovernedCycleLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "governed-cycle" {
					t.Errorf("finding %+v: LintName = %q, want %q", f, f.LintName, "governed-cycle")
				}
				if f.Severity != SeverityError {
					t.Errorf("finding %+v: Severity = %q, want %q", f, f.Severity, SeverityError)
				}
				// The message must name the graph it is about: a bare
				// "cycle detected" would be indistinguishable from the
				// rests_on lint's finding on the same claim.
				if !strings.Contains(f.Message, "governed_by cycle detected") {
					t.Errorf("message %q does not identify the governed_by graph", f.Message)
				}
				if strings.Contains(f.Message, "rests_on") {
					t.Errorf("message %q mentions rests_on; the two cycle lints must not be confusable", f.Message)
				}
			}
		})
	}
}

// TestGovernedCycleLintIsRegistered guards the coverage meta-gate's premise:
// the rule has to be in Registry for RunAll (and therefore "dossierx lint")
// to run it at all.
func TestGovernedCycleLintIsRegistered(t *testing.T) {
	for _, l := range Registry {
		if l.Name() == "governed-cycle" {
			return
		}
	}
	t.Fatal("governed-cycle lint is not registered in the lint Registry")
}
