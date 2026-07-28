// self_edge.go implements the "self-edge" lint: no claim may name its own id
// in any of its three edge kinds (rests_on, mirrors, governed_by).
//
// This rule exists because two of the three self-edges were silently legal,
// and the third was caught only as a side effect:
//
//   - rests_on: [self] was already reported, but incidentally — as the
//     degenerate one-node case of the "cycle" lint, with a cycle message that
//     describes the shape of the graph rather than the mistake.
//   - mirrors: [self] passed everything. mirror-reciprocal asks whether the
//     target mirrors the source back, and when the target IS the source the
//     answer is trivially yes; mirror-mismatch compares the two claims'
//     comparable content, and a claim's content always equals its own. So a
//     self-mirror read as a perfectly formed equality edge while asserting
//     nothing about any other claim.
//   - governed_by: self resolved cleanly — the claim exists, so dangling and
//     validated-on-missing are both satisfied — and asserted that the claim's
//     authority rests on the claim itself, which is exactly the circularity
//     governed_by exists to force an author to make explicit.
//
// All three are the same authoring mistake (nearly always a copy-pasted id),
// so they get one rule, one name, and a per-edge-kind message. Error
// severity: unlike an orphan claim, a self-edge is never a defensible
// modeling choice — an edge is a statement about another claim, and these
// statements have no other claim in them.
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
		// A claim with no id at all is id-shape's problem; without this
		// guard an empty id would "match" an empty governed_by.type and
		// report a self-edge that the author cannot act on.
		if c.ID == "" {
			continue
		}

		// One finding per edge KIND, not per occurrence: an edge list that
		// names the claim's own id twice is still one thing to delete, and
		// duplicate entries within a single list are not this rule's
		// subject.
		if containsString(c.RestsOn, c.ID) {
			findings = append(findings, Finding{
				LintName: "self-edge",
				ClaimID:  c.ID,
				Message:  "rests_on names this claim's own id: a claim cannot rest on itself",
				Severity: SeverityError,
			})
		}
		if containsString(c.Mirrors, c.ID) {
			findings = append(findings, Finding{
				LintName: "self-edge",
				ClaimID:  c.ID,
				Message:  "mirrors names this claim's own id: a claim cannot mirror itself",
				Severity: SeverityError,
			})
		}
		if c.Governed.Type == c.ID {
			findings = append(findings, Finding{
				LintName: "self-edge",
				ClaimID:  c.ID,
				Message:  "governed_by names this claim's own id: a claim cannot govern itself (use type: none with a reason, or name a doctrine claim)",
				Severity: SeverityError,
			})
		}
	}
	return findings
}
