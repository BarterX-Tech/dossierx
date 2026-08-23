package model

// TrackRole is a claim's relationship to one track. It is a CLOSED enum of
// exactly two values, and that pair is the whole reason a track is not a tag.
//
// A corpus partitioned by module answers "who guarantees this?" — exactly one
// module per claim, which is the right shape for writing and reviewing
// contracts. It cannot answer "what does the user get, and is it finished?",
// because a user-facing feature is assembled from claims spread across many
// modules and a module serves many features. That relationship is
// many-to-many; the module axis allows one.
//
// A track is the second axis. The invariant that keeps it from degenerating
// into free-form tagging is that every claim still has exactly ONE owner on
// EACH axis: one module, and at most one track for which its role is
// TrackRoleOwns. Everything else is TrackRoleCites — a reference, never a
// copy. Two owners means no owner, which is precisely the property that makes
// review and locking meaningful, so it is enforced (track-multi-owner) rather
// than merely documented.
type TrackRole string

const (
	// TrackRoleOwns marks the claim as the track's own statement: the
	// feature-level trigger, failure behaviour, or acceptance criterion that
	// belongs to no single module.
	//
	// Owning is what gives such sentences a home INSIDE the corpus. Without
	// it they end up in a generated document outside the tool — text that
	// cannot be locked, reviewed, commented on, or linted, and that is a
	// second copy of the truth by construction. A claim may own at most one
	// track. It still has an ordinary module, like every other claim.
	TrackRoleOwns TrackRole = "owns"

	// TrackRoleCites marks the claim as participating in the track without
	// belonging to it: the track's document references the claim, and the
	// claim's own module keeps guaranteeing it. A claim may cite any number
	// of tracks.
	//
	// This is the default when a membership omits its role, because it is the
	// safe half of the pair: citing adds a reference, owning makes an
	// exclusivity assertion, and an assertion that strong should be typed out
	// rather than fallen into.
	TrackRoleCites TrackRole = "cites"
)

// TrackRef is one claim's membership in one track.
//
// Membership is deliberately NOT modelled as an edge alongside RestsOn,
// Mirrors and Governed. Those are semantic dependencies between claims, and
// they carry cycle lints (cycle, governed-cycle, mixed-cycle) that would
// report nonsense over a membership graph: a track is a set, and a set has no
// direction to run in a circle. See internal/lint/mixed_cycle.go's doc comment
// for the rule this follows — a new edge kind must state whether it joins the
// union walk, and this one deliberately does not.
type TrackRef struct {
	// ID is the track this claim belongs to. It must name a track the
	// project config declares; an unknown id is a lint error
	// (track-unknown), never a track that springs into existence because
	// someone typed it. That registry is the second thing separating a
	// track from a tag.
	ID string `yaml:"id"`

	// Role is owns or cites. Unset means cites — read it via EffectiveRole
	// rather than directly, everywhere except lint's own explicit-value
	// checks, exactly as Claim.Kind is read via Claim.EffectiveKind.
	Role TrackRole `yaml:"role,omitempty"`
}

// EffectiveRole returns r's real role, mapping the unset zero value to
// TrackRoleCites. See TrackRoleCites for why citing is the default.
func (r TrackRef) EffectiveRole() TrackRole {
	if r.Role == "" {
		return TrackRoleCites
	}
	return r.Role
}

// Owns reports whether this membership claims ownership of its track.
func (r TrackRef) Owns() bool { return r.EffectiveRole() == TrackRoleOwns }

// OwnedTrackID returns the id of the single track c owns, or "" when c owns
// none. When a claim declares ownership of more than one track — which
// track-multi-owner refuses — the FIRST is returned, so callers that render or
// count are deterministic while the lint reports the defect.
func (c Claim) OwnedTrackID() string {
	for _, t := range c.Tracks {
		if t.Owns() {
			return t.ID
		}
	}
	return ""
}

// InTrack reports whether c declares membership in the track named id, in
// either role.
func (c Claim) InTrack(id string) bool {
	for _, t := range c.Tracks {
		if t.ID == id {
			return true
		}
	}
	return false
}
