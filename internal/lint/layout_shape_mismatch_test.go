package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestLayoutShapeMismatch(t *testing.T) {
	cases := []struct {
		name    string
		claim   model.Claim
		wantErr bool
	}{
		{
			name: "passing: table layout with rows",
			claim: model.Claim{
				ID: "widget.internals.fields", Layout: model.LayoutTable,
				Rows: []model.Row{{"field": "id"}},
			},
			wantErr: false,
		},
		{
			name: "passing: steps layout with steps",
			claim: model.Claim{
				ID: "widget.contract.setup", Layout: model.LayoutSteps,
				Steps: []string{"do a thing"},
			},
			wantErr: false,
		},
		{
			name: "passing: card layout, no rows/steps",
			claim: model.Claim{
				ID: "widget.contract.overview", Layout: model.LayoutCard, Body: "hi",
			},
			wantErr: false,
		},
		{
			name:    "passing: unset layout never mismatches",
			claim:   model.Claim{ID: "widget.contract.unset", Rows: []model.Row{{"field": "id"}}},
			wantErr: false,
		},
		{
			name: "failing: table layout with no rows",
			claim: model.Claim{
				ID: "widget.internals.empty-table", Layout: model.LayoutTable,
			},
			wantErr: true,
		},
		{
			// rows: [] on disk decodes to a non-nil, zero-length slice —
			// distinct from an omitted rows key (nil). An explicit empty
			// array is valid, intentional data (table.html renders an
			// explicit "No rows." state), so it must NOT be flagged the
			// same way as the "no rows at all" case above.
			name: "passing: table layout with explicit empty rows array",
			claim: model.Claim{
				ID: "widget.internals.explicit-empty-table", Layout: model.LayoutTable,
				Rows: []model.Row{},
			},
			wantErr: false,
		},
		{
			name: "failing: steps layout with no steps",
			claim: model.Claim{
				ID: "widget.contract.empty-steps", Layout: model.LayoutSteps,
			},
			wantErr: true,
		},
		{
			name: "failing: card layout with rows silently dropped",
			claim: model.Claim{
				ID: "widget.contract.card-with-rows", Layout: model.LayoutCard,
				Rows: []model.Row{{"field": "id"}},
			},
			wantErr: true,
		},
		{
			name: "failing: list layout with steps silently dropped",
			claim: model.Claim{
				ID: "widget.contract.list-with-steps", Layout: model.LayoutList,
				Steps: []string{"one"},
			},
			wantErr: true,
		},
		{
			name: "failing: layout value outside the six allowed types is a lint error, not a render panic",
			claim: model.Claim{
				ID: "widget.contract.bogus-layout", Layout: model.Layout("carousel"), Body: "hi",
			},
			wantErr: true,
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
