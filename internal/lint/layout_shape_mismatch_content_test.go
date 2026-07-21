package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestLayoutShapeMismatch_ContentRequired covers the "Claims & IDs" edge
// cases around a claim's body/rows content: a claim with neither is
// invalid (nothing would ever render for it), while a claim with both is
// perfectly fine (body is illustrative prose, rows is separately
// lint-checked structured data). Kept in its own file/test function so it
// doesn't collide with concurrent edits to TestLayoutShapeMismatch's own
// case table.
func TestLayoutShapeMismatch_ContentRequired(t *testing.T) {
	cases := []struct {
		name    string
		claim   model.Claim
		wantErr bool
	}{
		{
			name: "failing: no body, no rows, no steps at all",
			claim: model.Claim{
				ID: "widget.contract.empty", Layout: model.LayoutCard,
			},
			wantErr: true,
		},
		{
			// The Layout=="" skip in this lint only applies to the
			// layout-vs-shape checks; "has no content at all" must still
			// fire even when layout hasn't been inferred yet.
			name:    "failing: no content and unset layout",
			claim:   model.Claim{ID: "widget.contract.empty-unset-layout"},
			wantErr: true,
		},
		{
			name: "passing: table layout with both body and rows",
			claim: model.Claim{
				ID: "widget.internals.both", Layout: model.LayoutTable,
				Body: "context for the table below",
				Rows: []model.Row{{"field": "id"}},
			},
			wantErr: false,
		},
	}

	l := layoutShapeMismatchLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check([]model.Claim{tc.claim}, nil)
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
