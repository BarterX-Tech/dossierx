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
				{ID: "widget.contract.doctrine", Governed: model.Governed{Type: "none", Reason: "root"}},
				{
					ID:       "widget.contract.overview",
					RestsOn:  []string{"widget.contract.doctrine"},
					Mirrors:  []string{"widget.internals.overview"},
					Governed: model.Governed{Type: "widget.contract.doctrine"},
				},
				{ID: "widget.internals.overview", Mirrors: []string{"widget.contract.overview"}},
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
			// The case the whole rule exists for: mirror-reciprocal and
			// mirror-mismatch are both trivially satisfied by a self-mirror,
			// so nothing reported this before.
			name: "failing: mirrors names its own id",
			claims: []model.Claim{
				{ID: "widget.contract.self", Mirrors: []string{"widget.contract.self"}},
			},
			wantFindings: 1,
			wantContains: "mirrors names this claim's own id",
		},
		{
			// Likewise: governed_by: self resolves, so dangling and
			// validated-on-missing both pass it.
			name: "failing: governed_by names its own id",
			claims: []model.Claim{
				{ID: "widget.contract.self", Governed: model.Governed{Type: "widget.contract.self"}},
			},
			wantFindings: 1,
			wantContains: "governed_by names this claim's own id",
		},
		{
			name: "failing: all three edge kinds self-reference at once",
			claims: []model.Claim{
				{
					ID:       "widget.contract.self",
					RestsOn:  []string{"widget.contract.self"},
					Mirrors:  []string{"widget.contract.self"},
					Governed: model.Governed{Type: "widget.contract.self"},
				},
			},
			wantFindings: 3,
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
			// governed_by: none is the grounded case, not a self-edge, and an
			// empty id belongs to id-shape — neither may be reported here.
			name: "passing: governed_by none and an id-less claim are not self-edges",
			claims: []model.Claim{
				{ID: "widget.contract.grounded", Governed: model.Governed{Type: "none", Reason: "no doctrine"}},
				{ID: "", Governed: model.Governed{Type: ""}},
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
