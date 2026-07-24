package lint

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRowsShape(t *testing.T) {
	cases := []struct {
		name       string
		claims     []model.Claim
		wantClaims []string
	}{
		{
			name: "passing: all rows share the same columns",
			claims: []model.Claim{
				{
					ID: "a.internals.fields",
					Rows: []model.Row{
						{"field": "id", "type": "string"},
						{"field": "created_at", "type": "timestamp"},
					},
				},
			},
			wantClaims: nil,
		},
		{
			name: "passing: fewer than two rows can't disagree",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{{"field": "id"}}},
			},
			wantClaims: nil,
		},
		{
			name: "failing: a later row is missing a column present on row 0",
			claims: []model.Claim{
				{
					ID: "a.internals.fields",
					Rows: []model.Row{
						{"field": "id", "type": "string"},
						{"field": "created_at"},
					},
				},
			},
			wantClaims: []string{"a.internals.fields"},
		},
		{
			name: "failing: a later row has an extra column not on row 0",
			claims: []model.Claim{
				{
					ID: "a.internals.fields",
					Rows: []model.Row{
						{"field": "id"},
						{"field": "created_at", "type": "timestamp"},
					},
				},
			},
			wantClaims: []string{"a.internals.fields"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RowsShape{}.Check(tc.claims, nil)
			gotIDs := findingClaimIDs(got)
			assertStringSlicesEqual(t, gotIDs, tc.wantClaims)
		})
	}
}

// TestRowsShape_NonStringCells covers the value-shape half of rows-shape:
// every cell must be an authored string. A bare number like 1.0 renders as
// "1" (silent numeric corruption) and lists/maps render as Go-native junk,
// so any non-string cell is an error naming claim+row+column. This runs for
// every table claim, including single-row tables (there is no len<2 gate on
// the value check), and must never false-flag the engine-private row-order
// sentinel carried by YAML-decoded rows.
func TestRowsShape_NonStringCells(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: single-row table with only string cells",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{{"field": "id", "type": "string"}}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: an int cell",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{{"field": "id", "count": 1}}},
			},
			wantFindings: 1,
		},
		{
			name: "failing: a float cell",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{{"field": "id", "count": 1.0}}},
			},
			wantFindings: 1,
		},
		{
			name: "failing: a bool cell",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{{"field": "id", "nullable": true}}},
			},
			wantFindings: 1,
		},
		{
			name: "failing: a list cell",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{{"field": "id", "tags": []any{"a", "b"}}}},
			},
			wantFindings: 1,
		},
		{
			name: "failing: a map cell",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{{"field": "id", "meta": map[string]any{"k": "v"}}}},
			},
			wantFindings: 1,
		},
		{
			name: "failing: a non-string cell in a later row of a multi-row table",
			claims: []model.Claim{
				{ID: "a.internals.fields", Rows: []model.Row{
					{"field": "id", "count": "1"},
					{"field": "created_at", "count": 2},
				}},
			},
			wantFindings: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RowsShape{}.Check(tc.claims, nil)
			if len(got) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(got), tc.wantFindings, got)
			}
			for _, f := range got {
				if f.LintName != "rows-shape" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
				if f.Severity != SeverityError {
					t.Fatalf("expected SeverityError for a non-string cell, got %q", f.Severity)
				}
			}
		})
	}
}

// TestRowsShape_IgnoresDecodedRowOrderSentinel makes sure the value check
// does not flag the engine-private row-order key that model's YAML decode
// stashes in every decoded row (it holds a []string, not a string). Rows
// decoded from YAML carry that sentinel; a value check that iterated raw map
// keys would wrongly report it as a non-string cell.
func TestRowsShape_IgnoresDecodedRowOrderSentinel(t *testing.T) {
	var c model.Claim
	if err := yaml.Unmarshal([]byte(
		"id: a.internals.fields\nrows:\n  - field: id\n    type: string\n  - field: created_at\n    type: timestamp\n"), &c); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	// Sanity: the decoded rows really do carry captured column order.
	if model.RowColumns(c.Rows[0]) == nil {
		t.Fatal("fixture rows should carry captured column order after YAML decode")
	}
	got := RowsShape{}.Check([]model.Claim{c}, nil)
	if len(got) != 0 {
		t.Fatalf("expected no findings for an all-string decoded table, got %+v", got)
	}
}
