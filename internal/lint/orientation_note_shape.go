// orientation_note_shape.go implements the "orientation-note-shape" lint:
// structural rules for model.Claim.Kind/EffectiveKind that don't depend on
// any other claim (unlike orientation_note_order.go, which is about
// position relative to sibling claims). See model.Kind's doc comment for
// what an orientation-note claim is for.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, OrientationNoteShapeLint{})
}

// OrientationNoteShapeLint reports:
//  1. a non-empty Kind that isn't one of model's two defined values — same
//     typo-protection reasoning as BuildRoleRequiredLint's own enum check.
//  2. an overview-facet claim that explicitly sets kind: fact, directly
//     contradicting what living under that reserved facet already means
//     (EffectiveKind would report orientation-note regardless, so this
//     catches the author's stated intent disagreeing with reality, not a
//     rendering bug).
//  3. any EffectiveKind() == KindOrientationNote claim missing Module (the
//     per-module grouping internal/render's overview-injection and this
//     lint's own sibling, orientation-note-order, both key off) or not
//     laid out as layout: banner (the one component whose existing CSS
//     already gives orientation notes their distinct warn-orange
//     treatment — see FORMAT.md's viewer section).
type OrientationNoteShapeLint struct{}

func (OrientationNoteShapeLint) Name() string { return "orientation-note-shape" }

func (OrientationNoteShapeLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.Kind != "" && c.Kind != model.KindFact && c.Kind != model.KindOrientationNote {
			findings = append(findings, Finding{
				LintName: "orientation-note-shape",
				ClaimID:  c.ID,
				Message:  "invalid kind " + string(c.Kind),
			})
			continue
		}

		if c.Facet == config.ReservedOverviewFacet && c.Kind == model.KindFact {
			findings = append(findings, Finding{
				LintName: "orientation-note-shape",
				ClaimID:  c.ID,
				Message:  "kind: fact contradicts living under the reserved overview facet, which is always orientation-note",
			})
			continue
		}

		if c.EffectiveKind() != model.KindOrientationNote {
			continue
		}
		if c.Module == "" {
			findings = append(findings, Finding{
				LintName: "orientation-note-shape",
				ClaimID:  c.ID,
				Message:  "orientation-note claims must set module",
			})
		}
		if c.Layout != model.LayoutBanner {
			findings = append(findings, Finding{
				LintName: "orientation-note-shape",
				ClaimID:  c.ID,
				Message:  "orientation-note claims must set layout: banner",
			})
		}
	}
	return findings
}
