// rows_shape.go implements the "rows-shape" lint. Per model.Row's doc
// comment, rows is structured data and every row on a given claim is
// expected to share a consistent set of columns (keys) — a table claim
// with rows of differing shape can't render a sane header, and usually
// means a typo'd or missing column on one row rather than intentionally
// heterogeneous data.
//
// It additionally checks that every cell value is an authored string.
// table.html renders cells as-is, so a bare number (1.0 → "1") is silent
// numeric corruption and a list/map renders as Go-native junk; forcing
// authors to quote such values is the correct, loud behavior. This value
// check runs for every table claim, including single-row tables, and skips
// the engine-private row-order sentinel model stashes in decoded rows.
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, RowsShape{})
}

// RowsShape is the "rows-shape" lint.
type RowsShape struct{}

// Name returns this lint's rule name.
func (RowsShape) Name() string { return "rows-shape" }

// Check flags any claim whose rows[] entries do not all share the same set
// of column keys as the claim's first row, plus any cell whose value is not
// an authored string.
func (RowsShape) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		// Column-set agreement: needs at least two rows to disagree.
		if len(c.Rows) >= 2 {
			want := sortedKeys(c.Rows[0])
			wantJoined := strings.Join(want, ",")
			for i := 1; i < len(c.Rows); i++ {
				got := sortedKeys(c.Rows[i])
				if strings.Join(got, ",") != wantJoined {
					findings = append(findings, Finding{
						LintName: "rows-shape",
						ClaimID:  c.ID,
						Severity: SeverityError,
						Message: fmt.Sprintf("row %d columns [%s] do not match row 0 columns [%s]",
							i, strings.Join(got, ", "), strings.Join(want, ", ")),
					})
				}
			}
		}

		// Every cell must be an authored string, on every row of every
		// table (single-row tables included).
		for i, row := range c.Rows {
			for _, col := range rowRealColumns(row) {
				if _, ok := row[col].(string); !ok {
					findings = append(findings, Finding{
						LintName: "rows-shape",
						ClaimID:  c.ID,
						Severity: SeverityError,
						Message: fmt.Sprintf("row %d column %q value %v (%T) is not a string; quote it in the YAML",
							i, col, row[col], row[col]),
					})
				}
			}
		}
	}
	return findings
}

// rowOrderSentinel mirrors model's engine-private row-order key (a leading
// NUL byte): YAML-decoded rows stash their authored column order under it as
// a []string. model.RowColumns already excludes it, but the map-key
// fallback below skips it explicitly so the non-string-cell check never
// mistakes engine bookkeeping for a bad cell.
const rowOrderSentinel = "\x00order"

// rowRealColumns returns a row's real (author-facing) column names. For a
// row decoded from YAML it uses the captured column order; for a row built
// directly in Go (which carries no captured order) it falls back to the
// row's own keys, skipping the engine-private order sentinel.
func rowRealColumns(r model.Row) []string {
	if cols := model.RowColumns(r); cols != nil {
		return cols
	}
	cols := make([]string, 0, len(r))
	for k := range r {
		if k == rowOrderSentinel {
			continue
		}
		cols = append(cols, k)
	}
	sort.Strings(cols)
	return cols
}

func sortedKeys(r model.Row) []string {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
