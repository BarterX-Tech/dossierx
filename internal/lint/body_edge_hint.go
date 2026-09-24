// Lint body-edge-hint scans a claim's prose (Body with any fenced code
// blocks stripped out — code-orphan owns those) for mentions of another
// real claim's id that isn't declared through this claim's rests_on
// edges. Prose that says "see
// widget.contract.overview" but never lists widget.contract.overview under
// rests_on is a strong hint the edge was meant to be
// declared and was simply forgotten. This is a WARNING, not an error: the
// prose reference might be deliberately loose (a passing mention, not a
// dependency), so it's a nudge for a human to check, not a hard failure.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, bodyEdgeHintLint{})
}

type bodyEdgeHintLint struct{}

func (bodyEdgeHintLint) Name() string { return "body-edge-hint" }

func (bodyEdgeHintLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.Body == "" {
			continue
		}
		_, prose := splitFencedAndProse(c.Body)
		if prose == "" {
			continue
		}

		declared := make(map[string]bool, len(c.RestsOn))
		for _, id := range c.RestsOn {
			declared[id] = true
		}

		for _, tok := range dedupeStrings(extractCandidateIDs(prose, cfg)) {
			if tok == c.ID || declared[tok] {
				continue
			}
			if _, ok := claimByID(claims, tok); !ok {
				continue // not a real claim; out of scope for this lint
			}
			findings = append(findings, Finding{
				LintName: "body-edge-hint",
				ClaimID:  c.ID,
				Message:  fmt.Sprintf("body mentions claim %q but does not declare it as a rests_on edge", tok),
				Severity: SeverityWarning,
			})
		}
	}
	return findings
}
