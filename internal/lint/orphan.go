// Lint orphan flags a claim with no edges at all in either direction: it
// neither rests_on anything, nor is it the target of any other
// claim's rests_on. This is deliberately a WARNING, not an error —
// an isolated claim (e.g. a standalone glossary entry) can be entirely
// intentional, but is worth surfacing since the far more common case is a
// claim that was meant to be wired into the graph and wasn't.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, orphanLint{})
}

type orphanLint struct{}

func (orphanLint) Name() string { return "orphan" }

func (orphanLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	hasIncoming := make(map[string]bool, len(claims))
	for _, c := range claims {
		for _, dep := range c.RestsOn.IDs {
			hasIncoming[dep] = true
		}
	}

	var findings []Finding
	for _, c := range claims {
		hasOutgoing := len(c.RestsOn.IDs) > 0
		if hasOutgoing || hasIncoming[c.ID] {
			continue
		}
		findings = append(findings, Finding{
			LintName: "orphan",
			ClaimID:  c.ID,
			Message:  "claim has no rests_on edges in either direction",
			Severity: SeverityWarning,
		})
	}
	return findings
}
