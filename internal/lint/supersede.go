// supersede.go implements the "supersede" lint. migrated_from is
// provenance: a claim recording that it replaced an earlier one. If that
// earlier id still resolves to a live claim in the same claim set, the
// migration was never completed — the predecessor claim should have been
// removed (or the successor's migrated_from note is stale/wrong), and
// either way the claim set now asserts two claims for the same fact. This
// lint only fires when migrated_from happens to match another claim's id
// in the current set; a migrated_from note describing a claim that no
// longer exists anywhere (the normal, completed-migration case) is not
// flagged.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, Supersede{})
}

// Supersede is the "supersede" lint.
type Supersede struct{}

// Name returns this lint's rule name.
func (Supersede) Name() string { return "supersede" }

// Check flags any claim whose migrated_from names another claim id that is
// still present in claims.
func (Supersede) Check(claims []model.Claim, _ *config.Config) []Finding {
	byID := indexByID(claims)

	var findings []Finding
	for _, c := range claims {
		if c.MigratedFrom == "" {
			continue
		}
		if c.MigratedFrom == c.ID {
			continue // can't supersede itself; not this lint's concern.
		}
		if _, ok := byID[c.MigratedFrom]; ok {
			findings = append(findings, Finding{
				LintName: "supersede",
				ClaimID:  c.ID,
				Message:  fmt.Sprintf("migrated_from %q still exists as a live claim", c.MigratedFrom),
			})
		}
	}
	return findings
}
