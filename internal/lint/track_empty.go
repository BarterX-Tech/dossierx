// Lint track-empty reports a track declared in the project config that no
// claim references in any role.
//
// A declared track is a promise that the viewer will render a page answering
// "what does the user get here?". With no members the page is real, linked
// from the sidebar, and blank — which reads to anyone outside the team as
// "this feature has no claims behind it" rather than the truth, which is
// almost always one of two much more ordinary things: the track was renamed
// and the memberships still name the old id, or it was declared in advance of
// the work and forgotten. Both are cheap to fix the day they happen and
// expensive to notice later, which is exactly the shape of thing a lint is
// for.
//
// It is a WARNING, not an error, for the same reason track-unowned is:
// declaring the track before writing its claims is a legitimate way to work,
// and a project-wide error would make that ordinary sequence unbuildable —
// see internal/lint/roll_up.go for what that failure mode costs in practice.
// It also means an empty track never blocks `claim lock`: track membership
// gates nothing about a claim's lifecycle, by design.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, trackEmptyLint{})
}

type trackEmptyLint struct{}

// Name returns this lint's rule name.
func (trackEmptyLint) Name() string { return "track-empty" }

// Check reports every declared track with zero members. With a nil cfg, or a
// project that declares no tracks, there is nothing declared to be empty and
// the rule is a no-op.
func (trackEmptyLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	if cfg == nil {
		return nil
	}
	var findings []Finding
	for _, t := range cfg.Tracks {
		if trackHasAnyMember(claims, t.ID) {
			continue
		}
		findings = append(findings, Finding{
			LintName: "track-empty",
			// This finding is scoped to a TRACK, not a claim: the defect is
			// the absence of any claim, so there is no claim to attach it
			// to. Every other lint in this package fills ClaimID with the id
			// of the offending claim, and the reporting surfaces
			// (text/JSON/viewer) treat the field as an opaque label, so the
			// track id goes here and the message says "track" in its first
			// word to keep a reader from hunting for a claim by that name.
			ClaimID:  t.ID,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("track %q is declared in the project config but no claim declares membership in it; add a claim with tracks: - id: %s, or remove the declaration", t.ID, t.ID),
		})
	}
	return findings
}

// trackHasAnyMember reports whether any claim names id in its membership
// list, in either role. It reads model.Claim.InTrack rather than comparing
// ids inline so that the definition of "is a member" lives in exactly one
// place alongside the type it describes.
func trackHasAnyMember(claims []model.Claim, id string) bool {
	for _, c := range claims {
		if c.InTrack(id) {
			return true
		}
	}
	return false
}
