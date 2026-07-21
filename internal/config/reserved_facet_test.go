package config

import "testing"

// TestReservedOverviewFacet_MatchesModelCopy guards against the two
// independent copies of this constant (config.ReservedOverviewFacet and
// model's unexported reservedOverviewFacet — see Claim.EffectiveKind's doc
// comment for why model can't import config directly) drifting apart.
func TestReservedOverviewFacet_MatchesModelCopy(t *testing.T) {
	if ReservedOverviewFacet != "overview" {
		t.Fatalf("ReservedOverviewFacet = %q, want %q — internal/model's reservedOverviewFacet copy must be updated to match", ReservedOverviewFacet, "overview")
	}
}
