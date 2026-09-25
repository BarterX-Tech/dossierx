// dangling.go implements the "dangling" lint: every id a claim's rests_on
// names must resolve to a claim that actually exists in the claim set. A
// reference to an id with no matching claim is a dangling edge.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, DanglingLint{})
}

// DanglingLint reports rests_on edges that point at an id with no
// corresponding claim.
type DanglingLint struct{}

func (DanglingLint) Name() string { return "dangling" }

func (DanglingLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	known := make(map[string]bool, len(claims))
	for _, c := range claims {
		known[c.ID] = true
	}

	var findings []Finding
	for _, c := range claims {
		for _, target := range c.RestsOn.IDs {
			if !known[target] {
				findings = append(findings, Finding{
					LintName: "dangling",
					ClaimID:  c.ID,
					Message:  "rests_on references unknown claim id " + target,
				})
			}
		}
	}
	return findings
}
