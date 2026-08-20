// track_test.go covers the YAML round-trip of model.Claim's tracks field and
// the owns/cites role semantics — above all the default, since an omitted
// role is legal and means "cites", and the whole owns-is-exclusive invariant
// rests on that default being the SAFE half of the pair rather than the
// assertive one.
package model

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const tracksDoc = `id: widget.contract.a
facet: contract
module: widget
status: draft
governed_by:
  type: none
  reason: fixture
tracks:
  - id: checkout
    role: owns
  - id: refunds
    role: cites
  - id: reporting
`

func TestClaim_Tracks_RoundTrip(t *testing.T) {
	c := decodeClaim(t, tracksDoc)

	if len(c.Tracks) != 3 {
		t.Fatalf("len(Tracks) = %d, want 3", len(c.Tracks))
	}
	if c.Tracks[0].ID != "checkout" || !c.Tracks[0].Owns() {
		t.Errorf("Tracks[0] = %+v, want checkout/owns", c.Tracks[0])
	}
	if c.Tracks[1].ID != "refunds" || c.Tracks[1].Owns() {
		t.Errorf("Tracks[1] = %+v, want refunds/cites", c.Tracks[1])
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reloaded := decodeClaim(t, string(out))
	if len(reloaded.Tracks) != len(c.Tracks) {
		t.Fatalf("Tracks did not round-trip: got %d entries, want %d\n%s", len(reloaded.Tracks), len(c.Tracks), out)
	}
	for i := range c.Tracks {
		if reloaded.Tracks[i] != c.Tracks[i] {
			t.Errorf("Tracks[%d] did not round-trip:\n got %+v\nwant %+v", i, reloaded.Tracks[i], c.Tracks[i])
		}
	}
}

// TestTrackRef_OmittedRoleMeansCites pins the default. Citing adds a
// reference; owning makes an exclusivity assertion that track-multi-owner
// enforces. Defaulting the other way would mean a membership someone typed in
// a hurry silently claimed a feature's single ownership slot.
func TestTrackRef_OmittedRoleMeansCites(t *testing.T) {
	c := decodeClaim(t, tracksDoc)

	bare := c.Tracks[2]
	if bare.ID != "reporting" {
		t.Fatalf("Tracks[2].ID = %q, want %q", bare.ID, "reporting")
	}
	if bare.Role != "" {
		t.Errorf("Tracks[2].Role = %q, want the unset zero value to decode as-is", bare.Role)
	}
	if got := bare.EffectiveRole(); got != TrackRoleCites {
		t.Errorf("EffectiveRole() on a membership with no role = %q, want %q", got, TrackRoleCites)
	}
	if bare.Owns() {
		t.Error("a membership with no role answered Owns()=true; an omitted role must never claim ownership")
	}

	// The unset role must also stay unset on disk, so a round-trip does not
	// quietly promote every bare membership into an explicit one and rewrite
	// files that were fine.
	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "role: \"\"") {
		t.Errorf("an unset role was written out explicitly; omitempty must drop it:\n%s", out)
	}
}

// TestClaim_Tracks_OptionalAndOmitted is the compatibility half of the pair —
// see the equivalent in source_test.go for why the absent key, rather than an
// empty list, is what the lock ledger's compatibility gate depends on.
func TestClaim_Tracks_OptionalAndOmitted(t *testing.T) {
	c := decodeClaim(t, "id: widget.contract.a\nfacet: contract\nstatus: draft\ngoverned_by:\n  type: none\n  reason: fixture\n")
	if c.Tracks != nil {
		t.Fatalf("Tracks = %+v, want nil when omitted", c.Tracks)
	}
	if got := c.OwnedTrackID(); got != "" {
		t.Errorf("OwnedTrackID() on a claim with no tracks = %q, want empty", got)
	}
	if c.InTrack("checkout") {
		t.Error("InTrack reported membership on a claim that declares no tracks")
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "tracks:") {
		t.Fatalf("expected omitempty to drop an unset tracks field, got:\n%s", out)
	}
}

func TestClaim_OwnedTrackIDAndInTrack(t *testing.T) {
	c := decodeClaim(t, tracksDoc)

	if got := c.OwnedTrackID(); got != "checkout" {
		t.Errorf("OwnedTrackID() = %q, want %q", got, "checkout")
	}
	for _, id := range []string{"checkout", "refunds", "reporting"} {
		if !c.InTrack(id) {
			t.Errorf("InTrack(%q) = false, want true", id)
		}
	}
	if c.InTrack("nonexistent") {
		t.Error("InTrack reported membership in a track the claim does not name")
	}
}

// TestOwnedTrackID_IsDeterministicUnderMultipleOwners pins the documented
// behaviour for a claim that violates the one-owner invariant. The lint
// (track-multi-owner) is what REPORTS the defect; this method must not also
// become a second, quieter opinion about it. Returning the first owned track
// keeps every rendering and counting path deterministic while the real
// finding travels to the human.
func TestOwnedTrackID_IsDeterministicUnderMultipleOwners(t *testing.T) {
	c := Claim{Tracks: []TrackRef{
		{ID: "checkout", Role: TrackRoleOwns},
		{ID: "refunds", Role: TrackRoleOwns},
	}}
	for i := 0; i < 8; i++ {
		if got := c.OwnedTrackID(); got != "checkout" {
			t.Fatalf("OwnedTrackID() = %q on iteration %d, want the first owned track %q every time", got, i, "checkout")
		}
	}
}

// TestTrackRoleValues pins the two enum values as the strings they appear as
// on disk. They are a closed vocabulary track-shape validates against.
func TestTrackRoleValues(t *testing.T) {
	if TrackRoleOwns != "owns" {
		t.Errorf("TrackRoleOwns = %q, want %q", TrackRoleOwns, "owns")
	}
	if TrackRoleCites != "cites" {
		t.Errorf("TrackRoleCites = %q, want %q", TrackRoleCites, "cites")
	}
}

// TestTrackRef_UnknownRoleIsNotOwnership checks the role predicate fails
// closed: a typo like "own" or "owner" must not be treated as ownership. The
// track-shape lint reports the real problem; until then the safe reading is
// the non-exclusive one.
func TestTrackRef_UnknownRoleIsNotOwnership(t *testing.T) {
	for _, bad := range []TrackRole{"own", "owner", "OWNS", "maintains"} {
		r := TrackRef{ID: "checkout", Role: bad}
		if r.Owns() {
			t.Errorf("TrackRef with role %q answered Owns()=true; only the exact value %q is ownership", bad, TrackRoleOwns)
		}
	}
}
