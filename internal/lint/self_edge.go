// self_edge.go implements the "self-edge" lint: no claim may name its own id
// in rests_on.
//
// This rule exists because a self-edge is an authoring mistake (nearly always
// a copy-pasted id) that the "cycle" lint only catches as a side effect:
// rests_on: [self] is the degenerate one-node case of that lint, with a cycle
// message that describes the graph rather than the mistake. Error severity:
// unlike an orphan claim, a self-edge is never a defensible modeling choice —
// an edge is a statement about another claim, and this statement has no
// other claim in it.
//
// A rests_on self-edge therefore fires both this rule and "cycle", by design:
// the two say different true things about it (a self-reference, and a cycle
// in the dependency graph), and suppressing either would mean one of them
// lying about what it checks.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, SelfEdgeLint{})
}

// SelfEdgeLint reports claims that reference their own id from any edge.
type SelfEdgeLint struct{}

// Name returns this lint's rule name.
func (SelfEdgeLint) Name() string { return "self-edge" }

// Check reports at most one finding per claim per edge kind.
func (SelfEdgeLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		// A claim with no id at all is id-shape's problem, not a self-edge
		// the author could act on.
		if c.ID == "" {
			continue
		}

		// One finding per edge KIND, not per occurrence: an edge list that
		// names the claim's own id twice is still one thing to delete, and
		// duplicate entries within a single list are not this rule's
		// subject.
		if contains(c.RestsOn, c.ID) {
			findings = append(findings, Finding{
				LintName: "self-edge",
				ClaimID:  c.ID,
				Message:  "rests_on names this claim's own id: a claim cannot rest on itself",
				Severity: SeverityError,
			})
		}
	}
	return findings
}
