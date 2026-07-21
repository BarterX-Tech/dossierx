// rows_shape.go implements the "rows-shape" lint. Per model.Row's doc
// comment, rows is structured data and every row on a given claim is
// expected to share a consistent set of columns (keys) — a table claim
// with rows of differing shape can't render a sane header, and usually
// means a typo'd or missing column on one row rather than intentionally
// heterogeneous data.
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
// of column keys as the claim's first row.
func (RowsShape) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if len(c.Rows) < 2 {
			continue
		}
		want := sortedKeys(c.Rows[0])
		wantJoined := strings.Join(want, ",")
		for i := 1; i < len(c.Rows); i++ {
			got := sortedKeys(c.Rows[i])
			if strings.Join(got, ",") != wantJoined {
				findings = append(findings, Finding{
					LintName: "rows-shape",
					ClaimID:  c.ID,
					Message: fmt.Sprintf("row %d columns [%s] do not match row 0 columns [%s]",
						i, strings.Join(got, ", "), strings.Join(want, ", ")),
				})
			}
		}
	}
	return findings
}

func sortedKeys(r model.Row) []string {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
