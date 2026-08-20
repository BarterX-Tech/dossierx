// Lint track-unknown reports a claim that declares membership in a track the
// project config does not declare.
//
// This rule is what makes the config registry (config.Config.Tracks) mean
// something rather than decorate something. A vocabulary that creates itself
// on first use cannot catch a typo: "checkout" and "check-out" both look like
// working memberships, both render, and both produce a track page — so one
// feature quietly becomes two, each with half the claims and, very likely,
// neither with an owner. Nothing in the corpus is wrong enough to notice, and
// the reader who trusts the track page reads a partial answer as a whole one.
// The module axis has had this protection from the start (a claim's module
// must be declared); a second axis without it would be strictly weaker than
// the first while looking just as authoritative.
//
// Requiring the declaration also forces the moment where a human writes the
// track's Title, which is the only place the corpus says what the track is
// FOR in words a reader outside the team can use.
//
// Blank ids are skipped here on purpose: track-shape already reports them,
// and a blank id is not a wrong name, it is an absent one. One defect, one
// finding.
package lint

import (
	"fmt"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, trackUnknownLint{})
}

type trackUnknownLint struct{}

// Name returns this lint's rule name.
func (trackUnknownLint) Name() string { return "track-unknown" }

// Check reports every membership naming an id absent from cfg.Tracks.
//
// With a nil cfg there is no registry to check against, and this rule reports
// nothing: "no config was supplied" is not evidence that a track is
// undeclared, and a rule that fired on absence would flag every membership in
// every caller that lints claims without a project (package-level unit tests,
// most obviously) while proving nothing about the corpus.
func (trackUnknownLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	if cfg == nil {
		return nil
	}
	var findings []Finding
	for _, c := range claims {
		for _, ref := range c.Tracks {
			id := strings.TrimSpace(ref.ID)
			if id == "" {
				continue
			}
			if cfg.HasTrack(id) {
				continue
			}
			// The message names the declared vocabulary, because the reader's
			// next action is almost always to correct a spelling against it
			// rather than to add a track.
			findings = append(findings, Finding{
				LintName: "track-unknown",
				ClaimID:  c.ID,
				Severity: SeverityError,
				Message:  fmt.Sprintf("tracks names undeclared track %q; declare it under tracks: in the project config or use one of: %s", id, describeTrackVocabulary(cfg)),
			})
		}
	}
	return findings
}

// describeTrackVocabulary renders the project's declared track ids for a
// finding message, with an explicit phrase for the empty case — "use one of:
// (none declared)" tells the author the registry itself is missing, which is
// a different fix from a misspelling and would otherwise print as a bare,
// puzzling empty list.
func describeTrackVocabulary(cfg *config.Config) string {
	ids := cfg.TrackIDs()
	if len(ids) == 0 {
		return "(none declared)"
	}
	return strings.Join(ids, ", ")
}
