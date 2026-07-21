// cycle.go implements the "cycle" lint: rests_on is a semantic-consequence
// edge (this claim depends on the target remaining true), so the rests_on
// graph must be a DAG. Every claim that participates in a rests_on cycle is
// reported.
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
	edges := make(map[string][]string, len(claims))
	byID := make(map[string]bool, len(claims))
	for _, c := range claims {
		byID[c.ID] = true
		edges[c.ID] = c.RestsOn
	}

	state := make(map[string]int, len(claims))
	var findings []Finding
	reported := make(map[string]bool, len(claims))

	var path []string
	var visit func(id string)
	visit = func(id string) {
		state[id] = visiting
		path = append(path, id)
		for _, next := range edges[id] {
			if !byID[next] {
				continue // dangling lint's concern
			}
			switch state[next] {
			case unvisited:
				visit(next)
			case visiting:
				// Back edge to next: path[idx:] (inclusive) is the cycle.
				idx := -1
				for i, p := range path {
					if p == next {
						idx = i
						break
					}
				}
				if idx >= 0 {
					cycle := append([]string{}, path[idx:]...)
					for _, member := range cycle {
						if reported[member] {
							continue
						}
						reported[member] = true
						findings = append(findings, Finding{
							LintName: "cycle",
							ClaimID:  member,
							Message:  "rests_on cycle detected: " + strings.Join(append(cycle, next), " -> "),
						})
					}
				}
			}
			// state == visited: already fully explored, no cycle through it.
		}
		path = path[:len(path)-1]
		state[id] = visited
	}

	for _, c := range claims {
		if state[c.ID] == unvisited {
			visit(c.ID)
		}
	}
	return findings
}
