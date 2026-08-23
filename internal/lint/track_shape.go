// Lint track-shape checks that a claim's `tracks:` list is structurally
// usable before any other track rule tries to reason about it: every
// membership names a non-blank track id, carries a role drawn from
// model.TrackRole's closed pair (or omits it, which means cites), and names
// each track at most once.
//
// These three are grouped into one rule because they are the same failure at
// three angles — a membership entry that does not identify a single track in
// a single role. None of them is a judgement call, and each one silently
// breaks a DIFFERENT downstream reader if it slips through:
//
//   - A blank id joins no track, but it is not nothing: it renders as a
//     membership the author believes exists. The claim is missing from the
//     track page and present in the claim's own `tracks:` block, which is the
//     worst possible pair of appearances.
//   - An out-of-enum role is read by model.TrackRef.Owns as "not owns", so
//     `role: owner` or `role: Owns` degrades silently into a citation. The
//     author asserted ownership, the corpus records a reference, and
//     track-unowned then reports the track as having no acceptance statement
//     while the author is looking at the word "owner" in their own file.
//     That is why the check is on the literal value and not on
//     EffectiveRole, which by design cannot tell a typo from an omission.
//   - The same track named twice on one claim double-counts the claim in
//     every per-track count, and if the two entries disagree about role it is
//     genuinely undecidable which one the author meant. Refusing is the only
//     honest answer; picking one would be inventing intent.
//
// The rule is deliberately about the DECLARATION and not about the track it
// names: whether the id corresponds to a track the project declares is
// track-unknown's question. Splitting them that way keeps a typo'd id from
// producing two findings for one mistake, and keeps this rule meaningful for
// a project that has not yet written its config registry.
package lint

import (
	"fmt"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, trackShapeLint{})
}

type trackShapeLint struct{}

// Name returns this lint's rule name.
func (trackShapeLint) Name() string { return "track-shape" }

// validTrackRoles is the closed set of roles a membership may spell out.
// The empty string is legal too and is handled separately below, because it
// means "cites" (see model.TrackRoleCites) rather than "no role" — listing
// it here would make the two indistinguishable in the message text.
var validTrackRoles = map[model.TrackRole]bool{
	model.TrackRoleOwns:  true,
	model.TrackRoleCites: true,
}

// Check reports blank ids, out-of-enum roles, and repeated track ids within a
// single claim's membership list. A claim with no `tracks:` at all is
// untouched, so this rule costs nothing for a project that never adopts the
// second axis.
func (trackShapeLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		// seen is scoped to ONE claim: two different claims naming the same
		// track is the entire point of a track, and only a repeat within a
		// single list is a defect.
		seen := make(map[string]bool, len(c.Tracks))
		for i, ref := range c.Tracks {
			id := strings.TrimSpace(ref.ID)
			if id == "" {
				findings = append(findings, Finding{
					LintName: "track-shape",
					ClaimID:  c.ID,
					Severity: SeverityError,
					Message:  fmt.Sprintf("tracks[%d] has an empty id; every membership must name the track it joins", i),
				})
				// No id means nothing to check for duplication or to report
				// a role against — a second finding here would describe the
				// same broken entry twice.
				continue
			}
			if ref.Role != "" && !validTrackRoles[ref.Role] {
				findings = append(findings, Finding{
					LintName: "track-shape",
					ClaimID:  c.ID,
					Severity: SeverityError,
					Message:  fmt.Sprintf("tracks[%d] (%q) has invalid role %q; must be one of: owns, cites (omit it to mean cites)", i, id, string(ref.Role)),
				})
			}
			if seen[id] {
				findings = append(findings, Finding{
					LintName: "track-shape",
					ClaimID:  c.ID,
					Severity: SeverityError,
					Message:  fmt.Sprintf("tracks[%d] repeats track %q; a claim declares its membership in a track exactly once", i, id),
				})
				continue
			}
			seen[id] = true
		}
	}
	return findings
}
