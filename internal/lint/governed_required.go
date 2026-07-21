// governed_required.go implements the "governed-required" lint: every
// claim must carry a governed_by.type, and when that type is the "none"
// sentinel (deliberately not backed by any doctrine claim), governed_by.
// reason is required and must be non-blank, per FORMAT.md.
package lint

import (
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, GovernedRequiredLint{})
}

// GovernedRequiredLint reports claims missing governed_by.type, and claims
// whose governed_by.type is "none" but whose governed_by.reason is blank.
type GovernedRequiredLint struct{}

func (GovernedRequiredLint) Name() string { return "governed-required" }

func (GovernedRequiredLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if strings.TrimSpace(c.Governed.Type) == "" {
			findings = append(findings, Finding{
				LintName: "governed-required",
				ClaimID:  c.ID,
				Message:  "governed_by.type is required (either \"none\" or a doctrine claim id)",
			})
			continue
		}
		if c.Governed.Type == "none" && strings.TrimSpace(c.Governed.Reason) == "" {
			findings = append(findings, Finding{
				LintName: "governed-required",
				ClaimID:  c.ID,
				Message:  "governed_by.reason is required when governed_by.type is \"none\"",
			})
		}
	}
	return findings
}
