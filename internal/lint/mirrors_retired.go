// mirrors_retired.go implements the "mirrors-retired" lint: the mirrors
// edge kind is gone (NIT-17). The field remains on model.Claim so a
// leftover key is not an unknown-field parse error and so LockedClaimHash
// stays byte-identical for claims that never declared it. A non-empty list
// is an authoring error: delete the key; the engine does not walk it.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, MirrorsRetired{})
}

// MirrorsRetired is the "mirrors-retired" lint.
type MirrorsRetired struct{}

// Name returns this lint's rule name.
func (MirrorsRetired) Name() string { return "mirrors-retired" }

// Check flags every claim that still declares a mirrors edge.
func (MirrorsRetired) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if len(c.Mirrors) == 0 {
			continue
		}
		findings = append(findings, Finding{
			LintName: "mirrors-retired",
			ClaimID:  c.ID,
			Message:  "mirrors is retired: remove the mirrors: list; the engine does not walk it",
			Severity: SeverityError,
		})
	}
	return findings
}
