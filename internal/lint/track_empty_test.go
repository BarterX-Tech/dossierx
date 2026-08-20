package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestTrackEmptyLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		cfg          *config.Config
		wantFindings int
		wantClaimID  string
	}{
		{
			name:         "passing: a project declaring no tracks",
			claims:       []model.Claim{{ID: "widget.contract.a"}},
			cfg:          trackCfg(),
			wantFindings: 0,
		},
		{
			name:         "passing: a nil config",
			claims:       []model.Claim{{ID: "widget.contract.a"}},
			cfg:          nil,
			wantFindings: 0,
		},
		{
			name: "passing: a citation is enough to be non-empty",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout"}}},
			},
			cfg:          trackCfg("checkout"),
			wantFindings: 0,
		},
		{
			name:         "failing: a declared track nobody joined",
			claims:       []model.Claim{{ID: "widget.contract.a"}},
			cfg:          trackCfg("checkout"),
			wantFindings: 1,
			wantClaimID:  "checkout",
		},
		{
			name: "failing: memberships that all name some other track",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "refunds", Role: model.TrackRoleOwns}}},
			},
			cfg:          trackCfg("checkout", "refunds"),
			wantFindings: 1,
			wantClaimID:  "checkout",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := trackEmptyLint{}.Check(tc.claims, tc.cfg)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "track-empty" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
				if f.Severity != SeverityWarning {
					t.Fatalf("expected SeverityWarning, got %q", f.Severity)
				}
				// The finding is track-scoped: ClaimID carries the track id
				// because no claim exists to carry it.
				if f.ClaimID != tc.wantClaimID {
					t.Fatalf("expected ClaimID to carry the track id %q, got %q", tc.wantClaimID, f.ClaimID)
				}
			}
		})
	}
}

func TestTrackEmptyLint_Registered(t *testing.T) {
	if !lintRegistered("track-empty") {
		t.Fatal("track-empty lint is not registered in the lint Registry")
	}
}
