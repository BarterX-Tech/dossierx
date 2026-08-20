package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// trackCfg builds a config declaring the given track ids, with a title each
// (config.Validate requires one, and these tests read the same registry the
// CLI would hand the lint).
func trackCfg(ids ...string) *config.Config {
	cfg := &config.Config{}
	for _, id := range ids {
		cfg.Tracks = append(cfg.Tracks, config.Track{ID: id, Title: id})
	}
	return cfg
}

func TestTrackUnknownLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		cfg          *config.Config
		wantFindings int
	}{
		{
			name:         "passing: no claims at all",
			cfg:          trackCfg("checkout"),
			wantFindings: 0,
		},
		{
			name: "passing: every membership names a declared track",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout", Role: model.TrackRoleOwns}}},
				{ID: "widget.contract.b", Tracks: []model.TrackRef{{ID: "refunds"}}},
			},
			cfg:          trackCfg("checkout", "refunds"),
			wantFindings: 0,
		},
		{
			name: "failing: a near-miss spelling is a different track",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "check-out"}}},
			},
			cfg:          trackCfg("checkout"),
			wantFindings: 1,
		},
		{
			name: "failing: membership with no registry declared at all",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout"}}},
			},
			cfg:          trackCfg(),
			wantFindings: 1,
		},
		{
			name: "passing: a blank id belongs to track-shape, not here",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "  "}}},
			},
			cfg:          trackCfg("checkout"),
			wantFindings: 0,
		},
		{
			name: "passing: a nil config reports nothing rather than everything",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout"}}},
			},
			cfg:          nil,
			wantFindings: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := trackUnknownLint{}.Check(tc.claims, tc.cfg)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "track-unknown" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
				if f.Severity != SeverityError {
					t.Fatalf("expected SeverityError, got %q", f.Severity)
				}
				if f.ClaimID != "widget.contract.a" {
					t.Fatalf("finding should be scoped to the offending claim, got %q", f.ClaimID)
				}
			}
		})
	}
}

func TestTrackUnknownLint_Registered(t *testing.T) {
	if !lintRegistered("track-unknown") {
		t.Fatal("track-unknown lint is not registered in the lint Registry")
	}
}
