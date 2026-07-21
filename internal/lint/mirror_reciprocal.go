// mirror_reciprocal.go implements the "mirror-reciprocal" lint: a mirrors[]
// edge asserts that two claims are content-identical, which is inherently
// a symmetric relationship — so if claim A declares mirrors: [B], claim B
// is expected to declare mirrors: [A] right back. A one-directional
// mirrors edge usually means the reverse declaration was simply forgotten,
// which this lint catches independently of whether the content actually
// matches (that's mirror-mismatch's job).
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, MirrorReciprocal{})
}

// MirrorReciprocal is the "mirror-reciprocal" lint.
type MirrorReciprocal struct{}

// Name returns this lint's rule name.
func (MirrorReciprocal) Name() string { return "mirror-reciprocal" }

// Check flags every mirrors[] edge whose target claim exists (an
// unresolved target is mirror-unanchored's concern) but does not declare a
// mirrors[] edge back to the source claim.
func (MirrorReciprocal) Check(claims []model.Claim, _ *config.Config) []Finding {
	byID := indexByID(claims)

	var findings []Finding
	for _, c := range claims {
		for _, target := range c.Mirrors {
			other, ok := byID[target]
			if !ok {
				continue // mirror-unanchored's concern.
			}
			if !containsString(other.Mirrors, c.ID) {
				findings = append(findings, Finding{
					LintName: "mirror-reciprocal",
					ClaimID:  c.ID,
					Message:  fmt.Sprintf("mirrors %q but %q does not mirror back", target, target),
				})
			}
		}
	}
	return findings
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
