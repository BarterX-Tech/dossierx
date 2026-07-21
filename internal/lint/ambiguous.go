// ambiguous.go implements the "ambiguous" lint: a claim id must be unique
// across the whole claim set. Two or more claims sharing an id makes any
// edge reference to that id ambiguous (it is unclear which claim's content
// is authoritative), so every claim sharing a duplicated id is reported.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, AmbiguousLint{})
}

// AmbiguousLint reports claims whose id is shared by another claim.
type AmbiguousLint struct{}

func (AmbiguousLint) Name() string { return "ambiguous" }

func (AmbiguousLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	counts := make(map[string]int, len(claims))
	for _, c := range claims {
		counts[c.ID]++
	}

	var findings []Finding
	for _, c := range claims {
		if counts[c.ID] > 1 {
			findings = append(findings, Finding{
				LintName: "ambiguous",
				ClaimID:  c.ID,
				Message:  "id is used by more than one claim, making references to it ambiguous",
			})
		}
	}
	return findings
}
