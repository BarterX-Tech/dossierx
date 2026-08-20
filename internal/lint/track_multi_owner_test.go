package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestTrackMultiOwnerLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name:         "passing: no claims at all",
			claims:       nil,
			wantFindings: 0,
		},
		{
			name: "passing: one owned track and any number of citations",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{
					{ID: "checkout", Role: model.TrackRoleOwns},
					{ID: "refunds"},
					{ID: "search", Role: model.TrackRoleCites},
				}},
			},
			wantFindings: 0,
		},
		{
			name: "passing: two claims may each own a different track",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout", Role: model.TrackRoleOwns}}},
				{ID: "widget.contract.b", Tracks: []model.TrackRef{{ID: "refunds", Role: model.TrackRoleOwns}}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: one claim owning two tracks",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{
					{ID: "checkout", Role: model.TrackRoleOwns},
					{ID: "refunds", Role: model.TrackRoleOwns},
				}},
			},
			wantFindings: 1,
		},
		{
			name: "passing: the same track owned twice is a duplicate, not two owners",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{
					{ID: "checkout", Role: model.TrackRoleOwns},
					{ID: "checkout", Role: model.TrackRoleOwns},
				}},
			},
			wantFindings: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := trackMultiOwnerLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "track-multi-owner" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
				if f.Severity != SeverityError {
					t.Fatalf("expected SeverityError, got %q", f.Severity)
				}
				if f.ClaimID != "widget.contract.a" {
					t.Fatalf("finding should be scoped to the owning claim, got %q", f.ClaimID)
				}
			}
		})
	}
}

func TestTrackMultiOwnerLint_Registered(t *testing.T) {
	if !lintRegistered("track-multi-owner") {
		t.Fatal("track-multi-owner lint is not registered in the lint Registry")
	}
}
