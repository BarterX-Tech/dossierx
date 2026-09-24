package lint

import (
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, RestsOnRequiredLint{})
}

// RestsOnRequiredLint makes rests_on required (NIT-24): every claim carries
// either a list of claim ids or the stated absence {none: true, reason}. A
// claim with neither says nothing about what it builds on; a NONE without a
// reason says "nothing" without saying why; and both at once contradict each
// other. All three are error severity — the shape is the contract.
type RestsOnRequiredLint struct{}

func (RestsOnRequiredLint) Name() string { return "rests-on-required" }

func (RestsOnRequiredLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.RestsOn.Empty() {
			findings = append(findings, Finding{
				LintName: "rests-on-required",
				ClaimID:  c.ID,
				Message:  "rests_on is required: a list of claim ids, or {none: true, reason: ...}",
			})
			continue
		}
		if c.RestsOn.None && strings.TrimSpace(c.RestsOn.Reason) == "" {
			findings = append(findings, Finding{
				LintName: "rests-on-required",
				ClaimID:  c.ID,
				Message:  "rests_on.reason is required when rests_on.none is true",
			})
		}
		if c.RestsOn.None && len(c.RestsOn.IDs) > 0 {
			findings = append(findings, Finding{
				LintName: "rests-on-required",
				ClaimID:  c.ID,
				Message:  "rests_on cannot be none: true and name targets at the same time",
			})
		}
	}
	return findings
}
