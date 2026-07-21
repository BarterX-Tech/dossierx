// Lint roll-up checks the one place a claim's status makes a claim about
// other claims: layout: banner. banner.html renders no edges footer at all
// (internal/render/components/banner.html) — it's meant to read as a
// standalone status callout for its whole module, mirroring the
// "draft · N/M locked" roll-up convention this project already uses for
// its own docs tabs (a roll-up only shows locked once every card under it
// is locked). So: a banner claim's Status must not be "locked" while any
// other claim sharing its Module is still "draft" — that would present a
// module as fully reviewed when it isn't. A banner with no module-mates at
// all has nothing to roll up and is never flagged.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, rollUpLint{})
}

type rollUpLint struct{}

func (rollUpLint) Name() string { return "roll-up" }

func (rollUpLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, banner := range claims {
		if banner.Layout != model.LayoutBanner || banner.Status != model.StatusLocked {
			continue
		}
		for _, sibling := range claims {
			if sibling.ID == banner.ID || sibling.Module != banner.Module {
				continue
			}
			if sibling.Status != model.StatusLocked {
				findings = append(findings, Finding{
					LintName: "roll-up",
					ClaimID:  banner.ID,
					Message:  fmt.Sprintf("banner claim is locked but module %q still has a draft claim (%q); roll-up must stay unlocked until every claim in the module is locked", banner.Module, sibling.ID),
					Severity: SeverityError,
				})
				break
			}
		}
	}
	return findings
}
