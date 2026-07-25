// status_shape.go implements the "status-shape" lint: a claim's Status must
// be exactly one of the two lifecycle values model.Status defines —
// "draft" or "locked" — and must not be empty. This mirrors the
// enum-legality checks build-role-required-for-locked and
// layout-shape-mismatch already perform for BuildRole and Layout: an
// out-of-enum status (a typo, or a value like "published" borrowed from
// some other system) would otherwise flow past load-time and silently
// misclassify a claim in every status-aware code path (the lock gate,
// build-order's completeness check, the viewer's draft/locked styling).
//
// Per this codebase's convention, enum-legality lives in lint rather than
// in the model or loader, so this is enforced here rather than by rejecting
// the value at decode time. This lint is only about the Status enum itself;
// review_pending is a separate engine-managed boolean field and is not a
// status value, so it is deliberately not touched here.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, statusShapeLint{})
}

type statusShapeLint struct{}

// Name returns this lint's rule name.
func (statusShapeLint) Name() string { return "status-shape" }

// validStatuses is the fixed set of lifecycle values model.Status defines.
// It is checked here (rather than delegating to some validation method on
// model.Status) because Finding.Message needs to name the offending value,
// and lint is where every other enum-legality check in this codebase lives.
var validStatuses = map[model.Status]bool{
	model.StatusDraft:  true,
	model.StatusLocked: true,
}

func (statusShapeLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.Status == "" {
			findings = append(findings, Finding{
				LintName: "status-shape",
				ClaimID:  c.ID,
				Severity: SeverityError,
				Message:  "status is required and must be one of: draft, locked",
			})
			continue
		}
		if !validStatuses[c.Status] {
			findings = append(findings, Finding{
				LintName: "status-shape",
				ClaimID:  c.ID,
				Severity: SeverityError,
				Message:  fmt.Sprintf("invalid status %q; must be one of: draft, locked", string(c.Status)),
			})
		}
	}
	return findings
}
