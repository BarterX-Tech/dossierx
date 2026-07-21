package lint

import (
	"testing"

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
