package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestSelfEdgeLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
		wantContains string
	}{
		{
			name: "passing: every edge points at another claim",
			claims: []model.Claim{
				{ID: "widget.contract.doctrine"},
				{
					ID:      "widget.contract.overview",
					RestsOn: []string{"widget.contract.doctrine"},
				},
				{ID: "widget.internals.overview", RestsOn: []string{"widget.contract.overview"}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: rests_on names its own id",
			claims: []model.Claim{
				{ID: "widget.contract.self", RestsOn: []string{"widget.contract.self"}},
			},
			wantFindings: 1,
			wantContains: "rests_on names this claim's own id",
		},
		{
			// One finding per edge kind, not per occurrence.
			name: "failing: a duplicated self reference in one list is still one finding",
			claims: []model.Claim{
				{ID: "widget.contract.self", RestsOn: []string{"widget.contract.self", "widget.contract.self"}},
			},
			wantFindings: 1,
		},
		{
			// An empty id belongs to id-shape and may not be reported here.
			name: "passing: an edgeless claim and an id-less claim are not self-edges",
			claims: []model.Claim{
				{ID: "widget.contract.grounded"},
				{ID: ""},
			},
			wantFindings: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := SelfEdgeLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "self-edge" {
					t.Errorf("finding %+v: LintName = %q, want %q", f, f.LintName, "self-edge")
				}
				if f.Severity != SeverityError {
					t.Errorf("finding %+v: Severity = %q, want %q", f, f.Severity, SeverityError)
				}
			}
			if tc.wantContains != "" && !strings.Contains(findings[0].Message, tc.wantContains) {
				t.Errorf("message %q does not mention %q", findings[0].Message, tc.wantContains)
			}
		})
	}
}

// TestSelfEdgeLintIsRegistered guards the coverage meta-gate's premise: the
// rule has to be in Registry for RunAll (and therefore "dossierx lint") to
// run it at all.
func TestSelfEdgeLintIsRegistered(t *testing.T) {
	for _, l := range Registry {
		if l.Name() == "self-edge" {
			return
		}
	}
	t.Fatal("self-edge lint is not registered in the lint Registry")
}
