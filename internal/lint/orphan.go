// Lint orphan flags a claim with no edges at all in either direction: it
// neither mirrors/rests_on anything, nor is it the target of any other
// claim's mirrors/rests_on. This is deliberately a WARNING, not an error —
// an isolated claim (e.g. a standalone glossary entry) can be entirely
// intentional, but is worth surfacing since the far more common case is a
// claim that was meant to be wired into the graph and wasn't.
//
// governed_by is not counted as an edge here: type: none with a reason is
// the normal, expected state for a claim with no doctrine backing, so
// treating every such claim as "has an edge" would make this lint nearly
// useless; the governed-required lint is what enforces the reason itself.
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
		for _, dep := range append(append([]string{}, c.Mirrors...), c.RestsOn...) {
			hasIncoming[dep] = true
		}
	}

	var findings []Finding
	for _, c := range claims {
		hasOutgoing := len(c.Mirrors) > 0 || len(c.RestsOn) > 0
		if hasOutgoing || hasIncoming[c.ID] {
			continue
		}
		findings = append(findings, Finding{
			LintName: "orphan",
			ClaimID:  c.ID,
			Message:  "claim has no mirrors/rests_on edges in either direction",
			Severity: SeverityWarning,
		})
	}
	return findings
}
