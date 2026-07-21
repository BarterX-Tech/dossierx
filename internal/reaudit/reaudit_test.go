package reaudit

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestReauditRefusedWhenNotPending(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Status: model.StatusDraft, Body: "dep"}

	cases := []struct {
		name  string
		claim model.Claim
	}{
		{"draft claim", model.Claim{ID: "widget.contract.main", Status: model.StatusDraft, ReviewPending: false}},
		{"locked but not pending", model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: false}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ProposeDiff(tc.claim, dep)
			if err == nil {
				t.Fatalf("expected ProposeDiff to refuse a claim that is not locked+review_pending")
			}
		})
	}
}

func TestProposeDiffAllowedWhenLockedAndPending(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true, Body: "current body"}
	dep := model.Claim{ID: "widget.contract.dep", Status: model.StatusDraft, Body: "dep"}

	diff, err := ProposeDiff(claim, dep)
	if err != nil {
		t.Fatalf("expected ProposeDiff to succeed on a locked+review_pending claim, got: %v", err)
	}
	if diff.ClaimID != claim.ID {
		t.Fatalf("expected diff for claim %q, got %q", claim.ID, diff.ClaimID)
	}
}

func TestConfirmAppliesAndClearsFlag(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "dep v2"}
	claim := model.Claim{
		ID:            "widget.contract.main",
		Facet:         "contract",
		Module:        "widget",
		Status:        model.StatusLocked,
		ReviewPending: true,
		RestsOn:       []string{dep.ID},
		Body:          "Widget supports old behavior.",
	}
	claims := []model.Claim{claim, dep}
	store := &lock.Store{Hashes: map[string]string{dep.ID: "stale-hash"}}

	diff := Diff{
		ClaimID:  claim.ID,
		NoChange: false,
		Body:     `Widget supports <mark style="background:#f7c2c2;text-decoration:line-through">old</mark><mark style="background:#b7ebb0">new</mark> behavior.`,
		Note:     "dependency wording changed",
	}

	applied, err := Apply(claim, diff)
	if err != nil {
		t.Fatalf("Apply: unexpected error: %v", err)
	}
	if strings.Contains(applied.Body, "<mark") {
		t.Fatalf("expected all <mark> markup stripped, got body: %q", applied.Body)
	}
	if applied.Body != "Widget supports new behavior." {
		t.Fatalf("unexpected applied body: %q", applied.Body)
	}
	// Apply itself must not touch lock state; that's internal/lock's job,
	// invoked by the CLI right after a successful confirmed Apply.
	if !applied.ReviewPending {
		t.Fatalf("expected Apply to leave review_pending untouched (still true)")
	}
	if applied.Status != model.StatusLocked {
		t.Fatalf("expected Apply to leave status untouched (locked)")
	}

	cleared := lock.ClearReviewPending(applied, claims, store)
	if cleared.ReviewPending {
		t.Fatalf("expected review_pending cleared after confirmed reaudit")
	}
	if cleared.Status != model.StatusLocked {
		t.Fatalf("expected status to remain locked, got %q", cleared.Status)
	}
	if store.Hashes[dep.ID] != lock.ContentHash(dep) {
		t.Fatalf("expected dependency baseline hash refreshed after confirmed reaudit")
	}
}

func TestApplyNoChangeLeavesBodyPlain(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true, Body: "unchanged body"}
	diff := Diff{ClaimID: claim.ID, NoChange: true, Body: claim.Body, Note: "no change needed"}

	applied, err := Apply(claim, diff)
	if err != nil {
		t.Fatalf("Apply: unexpected error: %v", err)
	}
	if applied.Body != "unchanged body" {
		t.Fatalf("expected body unchanged, got %q", applied.Body)
	}
}

func TestApplyRejectsMismatchedClaimID(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true}
	diff := Diff{ClaimID: "widget.contract.other", Body: "irrelevant"}

	if _, err := Apply(claim, diff); err == nil {
		t.Fatalf("expected Apply to reject a diff for a different claim ID")
	}
}

func TestRejectLeavesClaimUntouched(t *testing.T) {
	// Simulates a human declining to confirm: the caller obtains a Diff via
	// ProposeDiff (or hand-authors one) but never calls Apply / ClearReviewPending.
	// The claim on disk must be completely unaffected.
	dep := model.Claim{ID: "widget.contract.dep", Status: model.StatusDraft, Body: "dep v2"}
	original := model.Claim{
		ID:            "widget.contract.main",
		Status:        model.StatusLocked,
		ReviewPending: true,
		Body:          "Widget supports old behavior.",
	}

	_, err := ProposeDiff(original, dep)
	if err != nil {
		t.Fatalf("ProposeDiff: unexpected error: %v", err)
	}

	// No Apply call happens (reject path). original must be unchanged.
	if original.Status != model.StatusLocked || !original.ReviewPending || original.Body != "Widget supports old behavior." {
		t.Fatalf("expected claim to be untouched on reject, got: %+v", original)
	}
}

func TestStripMarkupHandlesMultipleSpansAndStrayTags(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true}
	diff := Diff{
		ClaimID: claim.ID,
		Body: `<mark style="background:#f7c2c2;text-decoration:line-through">remove me</mark>keep` +
			`<mark style="background:#b7ebb0">add me</mark> and ` +
			`<mark style="background:#f7c2c2;text-decoration:line-through">also remove</mark>` +
			`<mark style="background:#b7ebb0">also add</mark>.`,
	}

	applied, err := Apply(claim, diff)
	if err != nil {
		t.Fatalf("Apply: unexpected error: %v", err)
	}
	want := "keepadd me and also add."
	if applied.Body != want {
		t.Fatalf("unexpected stripped body: got %q, want %q", applied.Body, want)
	}
}
