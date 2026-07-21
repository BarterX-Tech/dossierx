// orientation_note_order.go implements the "orientation-note-order" lint:
// the actual "an agent/reviewer reads orientation notes before anything
// else" guarantee. It reuses model.OrderClaims — the exact function
// internal/render's viewer uses to decide a facet's claim order — grouped
// per (module, facet), and asserts every orientation-note claim in that
// order appears strictly before every non-orientation-note claim. Reusing
// model.OrderClaims (rather than reimplementing "what does 'first' mean"
// here) is deliberate: a lint that agreed with the viewer today but
// silently stopped agreeing after some future render.go change would be
// far worse than no lint at all.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, OrientationNoteOrderLint{})
}

// OrientationNoteOrderLint reports every non-orientation-note claim that
// model.OrderClaims places ahead of at least one orientation-note claim
// within the same (module, facet) group.
type OrientationNoteOrderLint struct{}

func (OrientationNoteOrderLint) Name() string { return "orientation-note-order" }

func (OrientationNoteOrderLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	type key struct{ module, facet string }
	byGroup := map[key][]model.Claim{}
	var order []key
	for _, c := range claims {
		k := key{c.Module, c.Facet}
		if _, seen := byGroup[k]; !seen {
			order = append(order, k)
		}
		byGroup[k] = append(byGroup[k], c)
	}

	var findings []Finding
	for _, k := range order {
		ordered := model.OrderClaims(byGroup[k])
		seenFact := false
		for _, c := range ordered {
			if c.EffectiveKind() != model.KindOrientationNote {
				seenFact = true
				continue
			}
			if seenFact {
				findings = append(findings, Finding{
					LintName: "orientation-note-order",
					ClaimID:  c.ID,
					Message:  "orientation-note claim must render before every non-orientation claim in its facet, but at least one non-orientation claim precedes it",
				})
			}
		}
	}
	return findings
}
