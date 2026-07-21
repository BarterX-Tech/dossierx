// rest_on_locked.go implements the "rest-on-locked" lint: a locked claim
// asserts stability, so it may not rest_on a claim that has not itself
// passed review. If a locked claim's rests_on target exists but is still
// draft, that is a lint failure. Targets that don't exist at all are the
// "dangling" lint's concern, not this one.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, RestOnLockedLint{})
}

// RestOnLockedLint reports locked claims whose rests_on edge targets a
// claim that is not (yet) locked.
type RestOnLockedLint struct{}

func (RestOnLockedLint) Name() string { return "rest-on-locked" }

func (RestOnLockedLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}

	var findings []Finding
	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		for _, target := range c.RestsOn {
			dep, ok := byID[target]
			if !ok {
				continue // dangling lint's concern
			}
			if dep.Status != model.StatusLocked {
				findings = append(findings, Finding{
					LintName: "rest-on-locked",
					ClaimID:  c.ID,
					Message:  "locked claim rests_on " + target + " which is not locked",
				})
			}
		}
	}
	return findings
}
