// Lint validated-on-missing checks the one edge dangling doesn't cover:
// governed_by. When governed_by.type is a doctrine claim id (i.e. not the
// literal "none"), that id must resolve to a real claim — a claim can't be
// "validated on" a doctrine claim that doesn't exist. type: none is exempt
// by definition (governed-required is what checks it carries a reason).
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, validatedOnMissingLint{})
}

type validatedOnMissingLint struct{}

func (validatedOnMissingLint) Name() string { return "validated-on-missing" }

func (validatedOnMissingLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		t := c.Governed.Type
		if t == "" || t == string(model.GovernedNone) {
			continue
		}
		if _, ok := claimByID(claims, t); !ok {
			findings = append(findings, Finding{
				LintName: "validated-on-missing",
				ClaimID:  c.ID,
				Message:  fmt.Sprintf("governed_by references claim %q, which does not exist", t),
				Severity: SeverityError,
			})
		}
	}
	return findings
}
