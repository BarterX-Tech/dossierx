// Lint track-unowned reports a track that has citing claims but no claim
// owning it.
//
// An unowned track renders as a page of references with nothing at its head:
// every module's contribution is listed, and the feature-level sentence — the
// trigger, the failure behaviour, the acceptance criterion that belongs to no
// single module — is missing. That sentence is the reason the track axis
// exists (see model.TrackRoleOwns); a track without it is a folder, and a
// reader asking "is this finished, and what does finished mean?" gets a list
// of parts as an answer.
//
// WHY THIS IS A WARNING AND NOT AN ERROR.
//
// Writing the citing claims first and the owning claim last is not a mistake;
// it is how the work usually goes. The modules ship their pieces, and the
// feature-level statement is written once there is a feature to state. An
// error here would make that ordinary sequence fail `check`, the pre-commit
// hook and CI from the moment the first claim joins a track until the day
// someone writes its owner — and, because internal/lock.Lock lints the
// whole project, it would refuse `claim lock` on entirely unrelated claims
// for the duration. roll-up already paid that bill once; the lesson recorded
// there is that a project-wide error is the wrong instrument for a state that
// permitted, ordinary work creates.
//
// The narrower reading matters too: this rule reports a track whose OWNING
// STATEMENT has not been written. It says nothing about whether the track is
// complete, and deliberately so. Track completion is never a `check` error and
// never gates `claim lock` — a half-built feature is a normal state of the
// world, and a corpus that refused to describe one would be describing
// something else.
//
// An empty track is track-empty's finding, not this one: reporting both for a
// track nobody has touched would be two findings for one absence.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, trackUnownedLint{})
}

type trackUnownedLint struct{}

// Name returns this lint's rule name.
func (trackUnownedLint) Name() string { return "track-unowned" }

// Check reports every declared track that has at least one member and no
// owner. With a nil cfg, or a project declaring no tracks, it is a no-op.
func (trackUnownedLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	if cfg == nil {
		return nil
	}
	var findings []Finding
	for _, t := range cfg.Tracks {
		if !trackHasAnyMember(claims, t.ID) {
			continue
		}
		if trackHasOwner(claims, t.ID) {
			continue
		}
		findings = append(findings, Finding{
			LintName: "track-unowned",
			// Scoped to a TRACK rather than a claim, for the same reason
			// track-empty is: the defect is that no claim does something, so
			// no single claim can carry the finding. The track id goes in
			// ClaimID and the message leads with the word "track".
			ClaimID:  t.ID,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("track %q has citing claims but no owning claim; add role: owns to the claim that states what this track delivers", t.ID),
		})
	}
	return findings
}

// trackHasOwner reports whether any claim declares ownership of id.
//
// It scans memberships directly rather than going through
// model.Claim.OwnedTrackID, which returns only the FIRST owned track: a claim
// that (wrongly) owns two tracks would otherwise look like a non-owner of the
// second, and this rule would report a track as unowned on top of the
// multi-owner error already raised. One defect, one finding —
// track-multi-owner reports it, and this rule stays quiet.
func trackHasOwner(claims []model.Claim, id string) bool {
	for _, c := range claims {
		for _, ref := range c.Tracks {
			if ref.ID == id && ref.Owns() {
				return true
			}
		}
	}
	return false
}
