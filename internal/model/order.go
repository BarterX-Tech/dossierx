// Package model's OrderClaims sorts claims for viewer display within a
// single nav group (one module+facet). It is the single source of truth
// both internal/render (the viewer) and internal/lint (the
// orientation-note-order lint, which must agree with the viewer on "what
// counts as first") consume — see that lint's doc comment for why this
// used to live only in internal/render and was extracted here.
//
// Ordering is two-level. Claims are first split into runs: a maximal
// sequence of consecutive claims (in incoming order) that all share the
// same Section value. Within one run, a claim opts into an explicit
// position via its Order field (1-based; 0/unset means "no explicit
// order"): Order-set claims sort first, ascending by Order, ahead of every
// unordered claim in that same run, which keeps its incoming relative
// order (sort.SliceStable) as a stable, deterministic secondary key. Runs
// themselves are never reordered or merged with a same-Section-valued run
// elsewhere in the list.
package model

import "sort"

// OrderClaims returns claims in viewer display order. It never mutates its
// input.
func OrderClaims(claims []Claim) []Claim {
	var runs [][]Claim
	prevSection := ""
	for i, c := range claims {
		if i == 0 || c.Section != prevSection {
			runs = append(runs, nil)
		}
		runs[len(runs)-1] = append(runs[len(runs)-1], c)
		prevSection = c.Section
	}

	out := make([]Claim, 0, len(claims))
	for _, run := range runs {
		sort.SliceStable(run, func(i, j int) bool {
			oi, oj := run[i].Order, run[j].Order
			if oi == oj {
				return false
			}
			if oi == 0 {
				return false
			}
			if oj == 0 {
				return true
			}
			return oi < oj
		})
		out = append(out, run...)
	}
	return out
}
