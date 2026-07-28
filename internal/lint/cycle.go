// cycle.go implements the "cycle" lint: rests_on is a semantic-consequence
// edge (this claim depends on the target remaining true), so the rests_on
// graph must be a DAG. Every claim that participates in a rests_on cycle is
// reported.
//
// The graph walk itself lives in this file but is deliberately shared:
// governed_cycle.go runs the same traversal over the governed_by graph (see
// findEdgeCycles below), because governed_by is a real directed edge too and
// must likewise terminate. Keeping one traversal means a fix to the walk —
// like the recursion removal documented on findEdgeCycles — lands for every
// graph the engine checks, not just this one.
package lint

import (
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, CycleLint{})
}

// CycleLint reports claims that participate in a rests_on cycle.
type CycleLint struct{}

func (CycleLint) Name() string { return "cycle" }

const (
	unvisited = 0
	visiting  = 1
	visited   = 2
)

func (l CycleLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	// Severity is left unset (rather than SeverityError) purely to preserve
	// this lint's original Finding value byte-for-byte: RunAll normalizes an
	// empty Severity to SeverityError at its single choke point, so every
	// consumer already sees "error" — see lint.go's normalization comment.
	return findEdgeCycles(claims, "cycle", "rests_on cycle detected: ", "", func(c model.Claim) []string {
		return c.RestsOn
	})
}

// findEdgeCycles walks the directed graph whose nodes are claims and whose
// out-edges from a claim are edgesOf(claim), and returns one finding per
// claim that participates in a cycle. Edges pointing at ids no claim defines
// are skipped (that is the dangling lint's concern, not a cycle). Each
// finding's message is messagePrefix followed by the cycle path, rendered as
// "a -> b -> c -> a"; severity is written onto every finding verbatim, so
// passing "" leaves it for RunAll to normalize.
//
// The walk is an iterative depth-first search over an explicit frame stack
// rather than the obvious recursive closure, and the difference matters. The
// recursion depth here is the length of the longest edge chain in the
// project — authored data, with no engine-imposed bound. A pathologically
// deep chain (a generator, a bulk migration, a machine-authored corpus)
// therefore overflowed the goroutine stack and killed the process with an
// unrecoverable panic: the worst possible failure mode for a rule whose
// entire job is to report a malformed graph. A heap-allocated stack turns
// that same input into a finding.
//
// The frame's next cursor reproduces the recursive version's visit order
// exactly — children in slice order, depth-first, backtrack on exhaustion —
// so the findings this returns (their order, their cycle paths, and their
// messages) are identical to what the recursive implementation produced.
func findEdgeCycles(claims []model.Claim, lintName, messagePrefix string, severity Severity, edgesOf func(model.Claim) []string) []Finding {
	edges := make(map[string][]string, len(claims))
	byID := make(map[string]bool, len(claims))
	for _, c := range claims {
		byID[c.ID] = true
		edges[c.ID] = edgesOf(c)
	}

	state := make(map[string]int, len(claims))
	reported := make(map[string]bool, len(claims))
	var findings []Finding

	// frame is one node's position in the depth-first walk: the node's id and
	// the index of the next out-edge to follow from it. The frames' ids, in
	// stack order, ARE the current DFS path — which is what a back edge's
	// cycle gets read off of below, exactly as the recursive version read it
	// off a separate path slice.
	type frame struct {
		id   string
		next int
	}

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
				// Every out-edge explored. This is the recursive version's
				// return: shrink the path and mark the node fully visited.
				state[stack[top].id] = visited
				stack = stack[:top]
				continue
			}

			next := out[stack[top].next]
			stack[top].next++

			if !byID[next] {
				continue // dangling lint's concern
			}
			switch state[next] {
			case unvisited:
				state[next] = visiting
				stack = append(stack, frame{id: next})
			case visiting:
				// Back edge to next: stack[idx:] (inclusive) is the cycle.
				idx := -1
				for i := range stack {
					if stack[i].id == next {
						idx = i
						break
					}
				}
				if idx >= 0 {
					cycle := make([]string, 0, len(stack)-idx)
					for _, f := range stack[idx:] {
						cycle = append(cycle, f.id)
					}
					for _, member := range cycle {
						if reported[member] {
							continue
						}
						reported[member] = true
						findings = append(findings, Finding{
							LintName: lintName,
							ClaimID:  member,
							Message:  messagePrefix + strings.Join(append(cycle, next), " -> "),
							Severity: severity,
						})
					}
				}
			}
			// state == visited: already fully explored, no cycle through it.
		}
	}
	return findings
}
