package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestTrackShapeLint(t *testing.T) {
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
			name: "passing: a claim with no tracks is untouched",
			claims: []model.Claim{
				{ID: "widget.contract.a"},
			},
			wantFindings: 0,
		},
		{
			name: "passing: both spelled roles and an omitted one",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{
					{ID: "checkout", Role: model.TrackRoleOwns},
					{ID: "refunds", Role: model.TrackRoleCites},
					{ID: "search"},
				}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: blank id",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "   "}}},
			},
			wantFindings: 1,
		},
		{
			name: "failing: a role outside the closed pair",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout", Role: model.TrackRole("owner")}}},
			},
			wantFindings: 1,
		},
		{
			name: "failing: the same track twice on one claim",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{
					{ID: "checkout", Role: model.TrackRoleOwns},
					{ID: "checkout"},
				}},
			},
			wantFindings: 1,
		},
		{
			name: "passing: two different claims may name the same track",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: "checkout", Role: model.TrackRoleOwns}}},
				{ID: "widget.contract.b", Tracks: []model.TrackRef{{ID: "checkout"}}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: a blank id reports once, not once per angle",
			claims: []model.Claim{
				{ID: "widget.contract.a", Tracks: []model.TrackRef{
					{ID: "", Role: model.TrackRole("owner")},
					{ID: ""},
				}},
			},
			wantFindings: 2, // one per blank entry; no role or duplicate finding on top
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := trackShapeLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			for _, f := range findings {
				if f.LintName != "track-shape" {
					t.Fatalf("unexpected LintName %q", f.LintName)
				}
				if f.Severity != SeverityError {
					t.Fatalf("expected SeverityError, got %q", f.Severity)
				}
				if f.ClaimID == "" {
					t.Fatalf("finding has no ClaimID: %+v", f)
				}
			}
		})
	}
}

// TestTrackShapeLint_DoesNotMutate guards the Lint contract's "must not
// mutate claims" clause for the one rule that reads every field of every
// membership.
func TestTrackShapeLint_DoesNotMutate(t *testing.T) {
	claims := []model.Claim{
		{ID: "widget.contract.a", Tracks: []model.TrackRef{{ID: " checkout ", Role: model.TrackRole("owner")}}},
	}
	trackShapeLint{}.Check(claims, nil)
	if got := claims[0].Tracks[0].ID; got != " checkout " {
		t.Fatalf("Check rewrote the membership id to %q", got)
	}
	if got := claims[0].Tracks[0].Role; got != model.TrackRole("owner") {
		t.Fatalf("Check rewrote the membership role to %q", got)
	}
}

func TestTrackShapeLint_Registered(t *testing.T) {
	if !lintRegistered("track-shape") {
		t.Fatal("track-shape lint is not registered in the lint Registry")
	}
}

// lintRegistered reports whether a rule name is present in Registry. Shared
// by the five track rules' registration tests.
func lintRegistered(name string) bool {
	for _, l := range Registry {
		if l.Name() == name {
			return true
		}
	}
	return false
}
