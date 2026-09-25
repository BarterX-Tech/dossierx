// body_oversize.go implements the "body-oversize" lint: the sum of body +
// steps + rows cells cannot exceed max_claim_body_chars (default 2000),
// counted as Unicode code points. raw_html is exempt and has its own
// ceiling later. A body-only cap is dodged by moving prose into
// steps/rows, so those three surfaces share one budget. It is an ERROR on
// every status. The engine refuses rather than truncating.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, bodyOversizeLint{})
}

type bodyOversizeLint struct{}

// Name returns this lint's rule name.
func (bodyOversizeLint) Name() string { return "body-oversize" }

func (bodyOversizeLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	limit := cfg.ClaimBodyCharLimit()
	var findings []Finding
	for _, c := range claims {
		n := model.ClaimProseChars(c)
		if n <= limit {
			continue
		}
		findings = append(findings, Finding{
			LintName: "body-oversize",
			ClaimID:  c.ID,
			Severity: SeverityError,
			Message:  fmt.Sprintf("body+steps+rows is %d characters, over the project cap of %d; shorten the prose or raise max_claim_body_chars in project.config.yaml", n, limit),
		})
	}
	return findings
}
