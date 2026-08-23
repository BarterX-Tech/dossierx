package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestTrackUnownedLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		cfg          *config.Config
		wantFindings int
		wantClaimID  string
	}{
		{
			name:         "passing: a nil config",
			claims:       []model.Claim{{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout"}}}},
			cfg:          nil,
			wantFindings: 0,
		},
		{
			name:         "passing: an empty track is track-empty's finding, not this one",
			claims:       []model.Claim{{ID: "widget.contract.a"}},
			cfg:          trackCfg("checkout"),
			wantFindings: 0,
		},
		{
			name: "passing: one owner among many citations",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout"}}},
				{ID: "widget.contract.b", Tracks: []model.TrackRef{{ID: "checkout", Role: model.TrackRoleCites}}},
				{ID: "widget.contract.c", Tracks: []model.TrackRef{{ID: "checkout", Role: model.TrackRoleOwns}}},
			},
			cfg:          trackCfg("checkout"),
			wantFindings: 0,
		},
		{
			name: "failing: citations with nobody owning the track",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout"}}},
				{ID: "widget.contract.b", Tracks: []model.TrackRef{{ID: "checkout", Role: model.TrackRoleCites}}},
			},
			cfg:          trackCfg("checkout"),
			wantFindings: 1,
			wantClaimID:  "checkout",
		},
		{
			name: "passing: a multi-owner claim still owns every track it named",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{
					{ID: "checkout", Role: model.TrackRoleOwns},
					{ID: "refunds", Role: model.TrackRoleOwns},
				}},
			},
			cfg:          trackCfg("checkout", "refunds"),
			wantFindings: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := trackUnownedLint{}.Check(tc.claims, tc.cfg)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "track-unowned" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
				if f.Severity != SeverityWarning {
					t.Fatalf("expected SeverityWarning, got %q", f.Severity)
				}
				// Track-scoped, like track-empty: ClaimID carries the track
				// id because the defect is that no claim did something.
				if f.ClaimID != tc.wantClaimID {
					t.Fatalf("expected ClaimID to carry the track id %q, got %q", tc.wantClaimID, f.ClaimID)
				}
			}
		})
	}
}

func TestTrackUnownedLint_Registered(t *testing.T) {
	if !lintRegistered("track-unowned") {
		t.Fatal("track-unowned lint is not registered in the lint Registry")
	}
}
