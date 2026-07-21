// Lint code-orphan looks inside a claim's Body for fenced ```code blocks```
// (the parts of Body meant to be read literally, e.g. sample YAML/config
// snippets) and flags any claim-id-shaped token found there (module.facet
// segments matching the project's own config, per extractCandidateIDs)
// that doesn't resolve to a real claim. A code sample that references
// "widget.contract.overview" is implicitly asserting that claim exists;
// if it doesn't (typo, or the claim was renamed/removed), the sample is
// now an orphaned reference and the lint catches it — unlike prose
// mentions, which body-edge-hint handles separately and with a softer,
// hint-only severity.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, codeOrphanLint{})
}

type codeOrphanLint struct{}

func (codeOrphanLint) Name() string { return "code-orphan" }

func (codeOrphanLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.Body == "" {
			continue
		}
		fenced, _ := splitFencedAndProse(c.Body)
		if fenced == "" {
			continue
		}
		for _, tok := range dedupeStrings(extractCandidateIDs(fenced, cfg)) {
			if _, ok := claimByID(claims, tok); ok {
				continue
			}
			findings = append(findings, Finding{
				LintName: "code-orphan",
				ClaimID:  c.ID,
				Message:  fmt.Sprintf("code block references claim %q, which does not exist", tok),
				Severity: SeverityError,
			})
		}
	}
	return findings
}
