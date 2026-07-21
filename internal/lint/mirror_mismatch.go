// mirror_mismatch.go implements the "mirror-mismatch" lint: the
// deterministic-equality guarantee behind a mirrors[] edge (see
// FORMAT.md's "Edge types" section) — the target claim's comparable
// content must match the source claim's exactly, or this lint fails.
//
// "Comparable content" is deliberately narrower than the whole claim: id,
// facet, module, status, and edges (mirrors/rests_on/governed_by) are
// expected to differ between two mirrored claims (that's the whole point
// of them being two claims), so equality is checked over the rendered
// payload only — Layout, Body, Rows, and Steps.
package lint

import (
	"fmt"
	"reflect"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, MirrorMismatch{})
}

// MirrorMismatch is the "mirror-mismatch" lint.
type MirrorMismatch struct{}

// Name returns this lint's rule name.
func (MirrorMismatch) Name() string { return "mirror-mismatch" }

// Check flags every mirrors[] edge whose target resolves to a claim (an
// unresolved target is mirror-unanchored's problem, not this lint's) whose
// comparable content differs from the source claim's.
func (MirrorMismatch) Check(claims []model.Claim, _ *config.Config) []Finding {
	byID := indexByID(claims)

	var findings []Finding
	for _, c := range claims {
		for _, target := range c.Mirrors {
			other, ok := byID[target]
			if !ok {
				continue // mirror-unanchored's concern.
			}
			if !comparableContentEqual(c, other) {
				findings = append(findings, Finding{
					LintName: "mirror-mismatch",
					ClaimID:  c.ID,
					Message:  fmt.Sprintf("mirrors %q but comparable content differs", target),
				})
			}
		}
	}
	return findings
}

// comparableContentEqual reports whether a and b's rendered payload —
// Layout, Body, Rows, and Steps — is exactly equal.
func comparableContentEqual(a, b model.Claim) bool {
	if a.Layout != b.Layout {
		return false
	}
	if a.Body != b.Body {
		return false
	}
	if !reflect.DeepEqual(a.Rows, b.Rows) {
		return false
	}
	if !reflect.DeepEqual(a.Steps, b.Steps) {
		return false
	}
	return true
}
