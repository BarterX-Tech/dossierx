package main

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// A requested set is not the same object as its dependency closure: B's
// closure can include A, while a reviewer of {A,B} made a different approval
// decision than a reviewer of {B}. Keep that distinction at the token seam.
func TestPolicySnapshotBindsCanonicalRequestedSet(t *testing.T) {
	a := model.Claim{ID: "fixture.contract.a", Status: model.StatusDraft, Body: "A"}
	b := model.Claim{ID: "fixture.contract.b", Status: model.StatusDraft, Body: "B", RestsOn: []string{a.ID}}
	claims := []model.Claim{a, b}
	group := policySnapshot(claims, []string{b.ID, a.ID})
	single := policySnapshot(claims, []string{b.ID})
	if group == single {
		t.Fatal("a group token must not authorize its dependency consumer alone")
	}
	if got := proposalMismatch(group, []string{b.ID}); got != "wrong_request" {
		t.Fatalf("group token submitted for singleton = %q, want wrong_request", got)
	}
	if got := proposalMismatch(single, []string{b.ID}); got != "stale" {
		t.Fatalf("matching request token with a content mismatch check = %q, want stale", got)
	}
}
