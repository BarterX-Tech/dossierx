// summary_oversize.go implements the "summary-oversize" lint: a well-shaped
// summary still cannot exceed max_claim_summary_chars (default 200), counted
// as Unicode code points via utf8.RuneCountInString. Missing or malformed
// summaries are summary-required's job; this rule only measures a present
// string. It is an ERROR on every status. The engine refuses rather than
// truncating.
package lint

import (
	"fmt"
	"unicode/utf8"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, summaryOversizeLint{})
}

type summaryOversizeLint struct{}

// Name returns this lint's rule name.
func (summaryOversizeLint) Name() string { return "summary-oversize" }

func (summaryOversizeLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	limit := cfg.ClaimSummaryCharLimit()
	var findings []Finding
	for _, c := range claims {
		if model.SummaryIsMissing(c.Summary) {
			continue
		}
		n := utf8.RuneCountInString(c.Summary)
		if n <= limit {
			continue
		}
		findings = append(findings, Finding{
			LintName: "summary-oversize",
			ClaimID:  c.ID,
			Severity: SeverityError,
			Message:  fmt.Sprintf("summary is %d characters, over the project cap of %d; shorten it or split the claim; raising max_claim_summary_chars in project.config.yaml is the human's call, only on their explicit approval", n, limit),
		})
	}
	return findings
}
