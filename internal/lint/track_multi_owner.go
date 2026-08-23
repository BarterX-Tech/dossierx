// Lint track-multi-owner reports a claim that declares `role: owns` for more
// than one track.
//
// The invariant is one owner per axis: a claim has exactly one module, and at
// most one track it owns. model.TrackRole's doc comment is the argument for
// it; this file is its enforcement. Two owners means no owner — a claim that
// is the acceptance statement for two features is the acceptance statement
// for neither, because locking it, reviewing it or reading it answers "is
// this feature done?" for both at once and the two answers are not the same
// answer. The failure is not that the claim is over-committed; it is that the
// question the track axis exists to answer stops having a single addressee.
//
// This is an error rather than a warning because there is no legitimate
// mid-development state it describes. A claim that genuinely covers two
// features is two claims, and splitting it is the fix — whereas
// track-unowned's condition (an owner not written YET) is ordinary work in
// progress and stays a warning.
//
// Ownership is counted over DISTINCT track ids so that a claim naming one
// track twice, both times as owner, reports the duplicate (track-shape) and
// not a second, misleading defect here. Blank ids are likewise skipped:
// track-shape owns them.
package lint

import (
	"fmt"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, trackMultiOwnerLint{})
}

type trackMultiOwnerLint struct{}

// Name returns this lint's rule name.
func (trackMultiOwnerLint) Name() string { return "track-multi-owner" }

// Check reports each claim owning two or more distinct tracks, naming every
// track it claims so the author can see the whole conflict rather than
// discovering it one track at a time.
func (trackMultiOwnerLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		var owned []string
		seen := make(map[string]bool, len(c.Tracks))
		for _, ref := range c.Tracks {
			id := strings.TrimSpace(ref.ID)
			if id == "" || !ref.Owns() || seen[id] {
				continue
			}
			seen[id] = true
			owned = append(owned, id)
		}
		if len(owned) < 2 {
			continue
		}
		findings = append(findings, Finding{
			LintName: "track-multi-owner",
			ClaimID:  c.ID,
			Severity: SeverityError,
			Message:  fmt.Sprintf("claim owns %d tracks (%s); a claim owns at most one track — split it, or demote all but one membership to role: cites", len(owned), strings.Join(owned, ", ")),
		})
	}
	return findings
}
