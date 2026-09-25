// module_claim_cap.go implements the "module-claim-cap" lint: a module
// may not hold more claims than project.config.yaml's
// max_claims_per_module (default 10 when the field is omitted).
//
// The ceiling is project-wide on purpose. A pack is one module; 10 cards
// keeps that pack small. A corpus that already has fatter modules raises
// the one config number rather than hiding a second override per module.
// The rule counts every loaded claim in the module — draft and locked —
// because a draft already occupies pack budget.
//
// It is an ERROR. check refuses the project, and claim lock refuses any
// candidate in an over-cap module because each of those claims carries
// a finding (findingAffects keys off ClaimID). Recovery is to split or
// retire claims, or to raise max_claims_per_module.
package lint

import (
	"fmt"
	"sort"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, moduleClaimCapLint{})
}

type moduleClaimCapLint struct{}

// Name returns this lint's rule name.
func (moduleClaimCapLint) Name() string { return "module-claim-cap" }

// Check reports every claim in a module whose loaded claim count is
// above the project cap. With a nil cfg the default cap still applies.
func (moduleClaimCapLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	limit := cfg.ClaimsPerModuleLimit()
	type group struct {
		n   int
		ids []string
	}
	byModule := map[string]*group{}
	for _, c := range claims {
		mod := c.Module
		// A project claim belongs to no module (NIT-25), so it never counts
		// toward one, even if a malformed file names a module.
		if mod == "" || c.IsProjectClaim() {
			continue
		}
		g, ok := byModule[mod]
		if !ok {
			g = &group{}
			byModule[mod] = g
		}
		g.n++
		g.ids = append(g.ids, c.ID)
	}

	modules := make([]string, 0, len(byModule))
	for mod := range byModule {
		modules = append(modules, mod)
	}
	sort.Strings(modules)

	var findings []Finding
	for _, mod := range modules {
		g := byModule[mod]
		if g.n <= limit {
			continue
		}
		sort.Strings(g.ids)
		for _, id := range g.ids {
			findings = append(findings, Finding{
				LintName: "module-claim-cap",
				ClaimID:  id,
				Severity: SeverityError,
				Message:  fmt.Sprintf("module %q has %d claims, over the project cap of %d; split the module or retire claims; raising max_claims_per_module in project.config.yaml is the human's call, only on their explicit approval", mod, g.n, limit),
			})
		}
	}
	return findings
}
