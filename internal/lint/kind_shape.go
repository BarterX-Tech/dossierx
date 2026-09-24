// kind_shape.go implements the "kind-shape" lint: the only legal Kind is
// fact (or omitted, which EffectiveKind maps to fact). Any other value,
// including the retired orientation-note string, is refused.
package lint

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, KindShapeLint{})
}

// KindShapeLint reports a non-empty Kind that is not KindFact.
type KindShapeLint struct{}

func (KindShapeLint) Name() string { return "kind-shape" }

func (KindShapeLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.Kind == "" || c.Kind == model.KindFact {
			continue
		}
		findings = append(findings, Finding{
			LintName: "kind-shape",
			ClaimID:  c.ID,
			Message:  "invalid kind " + string(c.Kind) + "; the only legal value is fact (or omit the field)",
		})
	}
	return findings
}
