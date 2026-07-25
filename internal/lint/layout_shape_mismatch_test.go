package lint

import (
	"strings"
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
			// A layout: mockup claim's renderable content lives in RawHTML,
			// not Body/Rows/Steps — that is its documented primary use (a
			// body-less markup blob). The no-content check must count RawHTML
			// as content, or such a claim wrongly fails lint and can never
			// lock.
			name: "passing: mockup layout with only raw_html is content",
			claim: model.Claim{
				ID: "widget.internals.console-mockup", Layout: model.LayoutMockup,
				RawHTML: `<div class="gcp-row">mock</div>`,
			},
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
			name: "failing: layout value outside the allowed set is a lint error, not a render panic",
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

// TestLayoutShapeMismatch_InvalidLayoutMessageListsAllLayouts guards
// DX-AUD-22: the invalid-layout message used to hardcode "six allowed
// layouts (card, table, list, steps, tree, banner)", silently omitting the
// valid seventh layout (mockup). The message must be built from the same
// fixed-order layout list that backs the validity check, so it can never
// drift, and must include every renderable layout — mockup included.
func TestLayoutShapeMismatch_InvalidLayoutMessageListsAllLayouts(t *testing.T) {
	findings := layoutShapeMismatchLint{}.Check([]model.Claim{
		{ID: "widget.contract.bogus", Layout: model.Layout("carousel"), Body: "hi"},
	}, nil)
	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding for an unrenderable layout, got: %+v", findings)
	}
	msg := findings[0].Message
	if !strings.Contains(msg, "mockup") {
		t.Fatalf("invalid-layout message must list mockup, got: %q", msg)
	}
	for _, l := range []model.Layout{
		model.LayoutCard, model.LayoutTable, model.LayoutList,
		model.LayoutSteps, model.LayoutTree, model.LayoutBanner, model.LayoutMockup,
	} {
		if !strings.Contains(msg, string(l)) {
			t.Errorf("invalid-layout message must list %q, got: %q", l, msg)
		}
	}
	// The stale hardcoded count word must be gone.
	if strings.Contains(msg, "six") {
		t.Errorf("invalid-layout message must not hardcode a count word, got: %q", msg)
	}
}
