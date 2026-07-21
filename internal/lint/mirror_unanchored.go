// mirror_unanchored.go implements the "mirror-unanchored" lint: every id a
// claim names in its mirrors[] edge must resolve to another claim in the
// same claim set. A mirrors edge is a deterministic-equality edge (see
// FORMAT.md), so an edge with no anchor on the other end can never be
// verified for equality at all — that's a distinct, more specific failure
// than the generic "dangling" lint (which covers all edge kinds), so it
// gets its own named rule and message.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, MirrorUnanchored{})
}

// MirrorUnanchored is the "mirror-unanchored" lint.
type MirrorUnanchored struct{}

// Name returns this lint's rule name.
func (MirrorUnanchored) Name() string { return "mirror-unanchored" }

// Check flags every mirrors[] entry that does not resolve to a claim id
// present in claims.
func (MirrorUnanchored) Check(claims []model.Claim, _ *config.Config) []Finding {
	byID := indexByID(claims)

	var findings []Finding
	for _, c := range claims {
		for _, target := range c.Mirrors {
			if _, ok := byID[target]; !ok {
				findings = append(findings, Finding{
					LintName: "mirror-unanchored",
					ClaimID:  c.ID,
					Message:  fmt.Sprintf("mirrors unanchored id %q: no such claim", target),
				})
			}
		}
	}
	return findings
}

// indexByID is a small shared helper for the mirror-* lints: it builds a
// lookup of claim id -> claim over claims, keyed by the last-seen claim for
// a given id (duplicate ids are a different lint's problem).
func indexByID(claims []model.Claim) map[string]model.Claim {
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}
	return byID
}
