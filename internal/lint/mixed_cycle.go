// mixed_cycle.go implements the "mixed-cycle" lint: a loop that alternates
// edge kinds -- "A rests_on B, B governed_by A" -- is a real cycle that
// neither existing cycle rule can see.
//
// cycle.go's findEdgeCycles is one traversal walked once per call with a
// SINGLE edgesOf function: CycleLint passes c.RestsOn only and
// GovernedCycleLint passes governedByEdges only. Neither takes the union, so
// the mixed loop has no rests_on back edge for the first walk to find and no
// governed_by forward edge for the second to follow. It resolved on both ends,
// satisfied every other rule, and passed the entire registry.
//
// This rule walks the UNION graph with the edge kind carried on every hop, and
// reports a cycle only when that cycle's hops include at least one rests_on
// AND at least one governed_by. That restriction is load-bearing, not
// fastidiousness: tests/lint_fixtures_test.go's testLintFixtureFiresExactlyOneRule
// requires each coverage fixture to trip its own rule and nothing else, so a
// rule that also fired on the "cycle" and "governed-cycle" fixtures would
// break both of them. The restriction is also the honest reading of the
// defect: a pure rests_on loop is already reported, with a message naming the
// dependency graph, and a second finding on the same claims saying "mixed"
// would be false.
//
// findEdgeCycles is deliberately NOT modified to carry edge kinds. Its exact
// finding order and message text are what "cycle" and "governed-cycle" emit
// today, and both are asserted on; a shared walk that grew a third caller's
// requirements would put those at risk for no gain, since the union graph
// needs a different frame anyway (each frame has to remember which edge kind
// entered it).
//
// mirrors is not in the union. It is reciprocal by design -- mirror_reciprocal.go
// requires the pair -- so every mirrored pair would be a "cycle" and the rule
// would fire on correct corpora.
//
// Severity is error, matching cycle and governed-cycle. There is no migration
// document: a corpus containing this shape was always malformed, the engine
// simply could not see it.
package lint

import (
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, MixedCycleLint{})
}

// MixedCycleLint reports claims that participate in a cycle whose hops use
// both rests_on and governed_by.
type MixedCycleLint struct{}

// Name returns this lint's rule name.
func (MixedCycleLint) Name() string { return "mixed-cycle" }

const (
	edgeRestsOn    = "rests_on"
	edgeGovernedBy = "governed_by"
)

// mixedEdge is one out-edge of the union graph: where it points and which
// relation it came from. The kind is what the whole rule turns on, and it is
// also what the message prints, so a reader repairing the loop can see which
// hop to cut without opening every claim in the path.
type mixedEdge struct {
	to   string
	kind string
}

// Check walks the union of the rests_on and governed_by graphs and reports
// every claim caught in a cycle that uses both kinds.
//
// The walk is an iterative depth-first search over an explicit frame stack,
// for the reason findEdgeCycles' doc comment gives at length: recursion depth
// here is the length of the longest authored edge chain, with no
// engine-imposed bound, and a deep chain would kill the process with an
// unrecoverable panic -- the worst possible failure mode for a rule whose job
// is to report a malformed graph.
//
// Like every back-edge cycle detector (findEdgeCycles included), this reports
// the cycles its depth-first path finds, not the complete set of simple cycles
// in the graph, which is exponential. A corpus can therefore in principle hold
// a second mixed loop through claims an earlier branch already retired. That
// is acceptable for the same reason it is acceptable in "cycle": the first
// finding fails the build, and the repair is re-run.
func (MixedCycleLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	byID := make(map[string]bool, len(claims))
	for _, c := range claims {
		byID[c.ID] = true
	}

	edges := make(map[string][]mixedEdge, len(claims))
	for _, c := range claims {
		var out []mixedEdge
		for _, target := range c.RestsOn {
			if byID[target] {
				out = append(out, mixedEdge{to: target, kind: edgeRestsOn})
			}
		}
		// governedByEdges (governed_cycle.go) is the single definition of
		// "governed_by is a real edge here": nothing for the "none" sentinel,
		// nothing for an unset type. Reusing it means the two rules can never
		// disagree about what an edge is.
		for _, target := range governedByEdges(c) {
			if byID[target] {
				out = append(out, mixedEdge{to: target, kind: edgeGovernedBy})
			}
		}
		edges[c.ID] = out
	}

	// frame is one node's position in the walk: its id, the kind of edge that
	// entered it (empty at a root), and the index of the next out-edge to
	// follow. The frames' ids in stack order ARE the current path, and their
	// via kinds are that path's hops -- which is what makes a back edge's
	// cycle readable as an alternating sequence rather than a list of names.
	type frame struct {
		id   string
		via  string
		next int
	}

	state := make(map[string]int, len(claims))
	reported := make(map[string]bool, len(claims))
	var findings []Finding

	for _, root := range claims {
		if state[root.ID] != unvisited {
			continue
		}
		state[root.ID] = visiting
		stack := []frame{{id: root.ID}}

		for len(stack) > 0 {
			top := len(stack) - 1
			out := edges[stack[top].id]

			if stack[top].next >= len(out) {
				state[stack[top].id] = visited
				stack = stack[:top]
				continue
			}

			e := out[stack[top].next]
			stack[top].next++

			switch state[e.to] {
			case unvisited:
				state[e.to] = visiting
				stack = append(stack, frame{id: e.to, via: e.kind})
			case visiting:
				idx := -1
				for i := range stack {
					if stack[i].id == e.to {
						idx = i
						break
					}
				}
				if idx < 0 {
					continue
				}
				members := make([]string, 0, len(stack)-idx)
				hops := make([]string, 0, len(stack)-idx)
				for i := idx; i < len(stack); i++ {
					members = append(members, stack[i].id)
					if i > idx {
						hops = append(hops, stack[i].via)
					}
				}
				// The closing hop is the back edge itself. members and hops
				// are now the same length: hop i is the edge leaving
				// members[i].
				hops = append(hops, e.kind)

				if !hasBothKinds(hops) {
					// A pure rests_on loop is "cycle"'s finding and a pure
					// governed_by loop is "governed-cycle"'s. Deliberately no
					// `reported` marking here: a claim that sits on a pure
					// cycle may sit on a mixed one too, and must still be
					// reportable when that one is found.
					continue
				}

				path := renderMixedCyclePath(members, hops)
				for _, member := range members {
					if reported[member] {
						continue
					}
					reported[member] = true
					findings = append(findings, Finding{
						LintName: "mixed-cycle",
						ClaimID:  member,
						Message:  "mixed rests_on/governed_by cycle detected: " + path,
						Severity: SeverityError,
					})
				}
			}
			// state == visited: fully explored, no cycle through it.
		}
	}
	return findings
}

// hasBothKinds reports whether a cycle's hops carry at least one rests_on and
// at least one governed_by. This single predicate is the entire difference
// between this rule and the two that came before it.
func hasBothKinds(hops []string) bool {
	var sawRests, sawGoverned bool
	for _, k := range hops {
		switch k {
		case edgeRestsOn:
			sawRests = true
		case edgeGovernedBy:
			sawGoverned = true
		}
	}
	return sawRests && sawGoverned
}

// renderMixedCyclePath renders the loop as
// "a -(rests_on)-> b -(governed_by)-> a", closing back on the first member.
// members and hops are the same length; hops[i] is the edge leaving
// members[i].
func renderMixedCyclePath(members, hops []string) string {
	var b strings.Builder
	for i, id := range members {
		b.WriteString(id)
		b.WriteString(" -(")
		b.WriteString(hops[i])
		b.WriteString(")-> ")
	}
	b.WriteString(members[0])
	return b.String()
}
