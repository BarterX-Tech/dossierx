// governed_cycle.go implements the "governed-cycle" lint: the governed_by
// graph must terminate.
//
// governed_by.type is either the "none" sentinel or the id of the doctrine
// claim that backs this claim's authority — and in the second case it is a
// real directed edge in the claim graph, not a free-text note: dangling.go
// resolves it, validated-on-missing.go requires it to point at a live claim,
// and hub-gating refuses to lock a claim whose doctrine claim is still draft.
// What nothing checked was whether following that edge ever gets anywhere.
// "A is governed by B, B is governed by A" resolved on both ends, satisfied
// every other rule, and passed the entire registry — while asserting that
// each claim's authority rests on the other's, which is to say on nothing.
// Authority has to bottom out, at "none" with a reason or at a doctrine claim
// that is itself grounded.
//
// This is a separate rule from "cycle" rather than a second edge kind folded
// into it, and its message names governed_by explicitly, because the two
// graphs mean different things — dependency versus authority — and the repair
// differs accordingly. A reader must never have to guess which graph a
// "cycle detected" finding is talking about.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, GovernedCycleLint{})
}

// GovernedCycleLint reports claims that participate in a governed_by cycle.
type GovernedCycleLint struct{}

// Name returns this lint's rule name.
func (GovernedCycleLint) Name() string { return "governed-cycle" }

// Check walks the governed_by graph with the same traversal the rests_on
// "cycle" lint uses (findEdgeCycles, in cycle.go) and reports every claim
// caught in a cycle.
func (GovernedCycleLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	return findEdgeCycles(claims, "governed-cycle", "governed_by cycle detected: ", SeverityError, governedByEdges)
}

// governedByEdges is the governed_by view of a claim as graph out-edges: at
// most one, and none at all for the "none" sentinel (which is precisely the
// grounded, terminating case this lint wants every chain to reach) or for an
// unset type (governed-required's concern).
func governedByEdges(c model.Claim) []string {
	t := c.Governed.Type
	if t == "" || t == string(model.GovernedNone) {
		return nil
	}
	return []string{t}
}
