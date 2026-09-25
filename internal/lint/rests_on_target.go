package lint

import (
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, RestsOnTargetLint{})
}

// RestsOnTargetLint enforces the target rule on every rests_on id (NIT-24,
// NIT-25): a claim may rest on project.<slug>, on any module's *.contract.*,
// and on its OWN module's *.internals.* — never on a foreign module's
// internals, which are exactly the facts that module reserves the right to
// change without anyone else's review. A project claim has no module, so for
// it every *.internals.* target is foreign.
//
// Whether the id exists at all is dangling's question, not this one's, so an
// unknown target is skipped here rather than reported twice.
type RestsOnTargetLint struct{}

func (RestsOnTargetLint) Name() string { return "rests-on-target" }

func (RestsOnTargetLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	known := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		known[c.ID] = c
	}
	var findings []Finding
	for _, c := range claims {
		for _, target := range c.RestsOn.IDs {
			dep, ok := known[target]
			if !ok {
				continue // dangling reports unknown claim ids
			}
			if !internalsTarget(dep) || sameModule(c, dep) {
				continue
			}
			msg := "rests_on must not name a foreign module's internals (" + target + ")"
			if c.IsProjectClaim() {
				msg = "a project claim's rests_on may name project.* ids and any module's *.contract.* claims, never *.internals.* (" + target + ")"
			}
			findings = append(findings, Finding{LintName: "rests-on-target", ClaimID: c.ID, Message: msg})
		}
	}
	return findings
}

func internalsTarget(c model.Claim) bool {
	return strings.EqualFold(c.Facet, "internals")
}

func sameModule(a, b model.Claim) bool {
	if a.IsProjectClaim() || b.IsProjectClaim() {
		return false
	}
	return a.Module != "" && a.Module == b.Module
}
