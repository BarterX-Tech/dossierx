// Lint shared-context-budget keeps the shared half of every
// `manifest show --isolation` view — the constitution text plus the project
// claims index — inside manifest.SharedBudgetBytes (NIT-7 Q2).
//
// Every module's isolation view carries that text, so it is enforced once,
// here, and never by refusing a module's view. The finding lands on the
// project claim whose index line pushes the shared context over the budget,
// in index (id) order, so check refuses the project and claim lock refuses
// that claim. When the constitution text alone is over, the finding is
// project-wide (claim id ""). Recovery is to shorten project claim
// summaries, retire project claims, or trim the constitution.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// SharedContextBudgetLintName is the finding's LintName.
const SharedContextBudgetLintName = "shared-context-budget"

func init() {
	Registry = append(Registry, sharedContextBudgetLint{})
}

type sharedContextBudgetLint struct{}

func (sharedContextBudgetLint) Name() string { return SharedContextBudgetLintName }

func (sharedContextBudgetLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	if cfg == nil {
		return nil
	}
	sc := manifest.BuildSharedContext(claims, cfg)
	over, err := manifest.CheckSharedBudget(sc)
	if err != nil || over == nil {
		return nil
	}
	who := fmt.Sprintf("project claim %s's index line pushes it over", over.ClaimID)
	if over.ClaimID == "" {
		who = "the constitution text alone is over"
	}
	return []Finding{{
		LintName: SharedContextBudgetLintName,
		ClaimID:  over.ClaimID,
		Severity: SeverityError,
		Message: fmt.Sprintf("shared isolation context (constitution text + %d project claim summaries) is %d bytes, over its %d-byte budget; %s. "+
			"Every module's manifest show --isolation carries this text: shorten project claim summaries, retire project claims, or trim the constitution",
			len(sc.ProjectClaims), over.Bytes, manifest.SharedBudgetBytes, who),
	}}
}
