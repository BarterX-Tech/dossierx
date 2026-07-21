package reaudit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestProposeFlagDiff_RefusedWhenNotPending(t *testing.T) {
	flag := PendingFlag{ClaimSays: "old", NowDoes: "new", Reason: "code changed"}
	cases := []struct {
		name  string
		claim model.Claim
	}{
		{"draft claim", model.Claim{ID: "widget.contract.main", Status: model.StatusDraft}},
		{"locked but not pending", model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ProposeFlagDiff(tc.claim, flag); err == nil {
				t.Fatalf("expected ProposeFlagDiff to refuse a claim that is not locked+review_pending")
			}
		})
	}
}

func TestProposeFlagDiff_RendersClaimSaysAsRedAndNowDoesAsGreen(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true, Body: "Widget supports old behavior."}
	flag := PendingFlag{ClaimSays: "old behavior", NowDoes: "new behavior", Reason: "implementation changed"}

	diff, err := ProposeFlagDiff(claim, flag)
	if err != nil {
		t.Fatalf("ProposeFlagDiff: %v", err)
	}
	if diff.NoChange {
		t.Fatalf("expected NoChange=false for a real flag-sourced proposal")
	}
	if !strings.Contains(diff.Body, `background:#f7c2c2;text-decoration:line-through">old behavior</mark>`) {
		t.Fatalf("expected claim-says wrapped in the red removal span, got: %s", diff.Body)
	}
	if !strings.Contains(diff.Body, `background:#b7ebb0">new behavior</mark>`) {
		t.Fatalf("expected now-does wrapped in the green addition span, got: %s", diff.Body)
	}
	if !strings.Contains(diff.Note, "implementation changed") {
		t.Fatalf("expected the flag's reason surfaced in Note, got: %s", diff.Note)
	}
}

func TestProposeFlagDiff_ThenApply_ReplacesBodyWithNowDoes(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true, Body: "Widget supports old behavior."}
	flag := PendingFlag{ClaimSays: "old behavior", NowDoes: "new behavior only", Reason: "rewritten"}

	diff, err := ProposeFlagDiff(claim, flag)
	if err != nil {
		t.Fatalf("ProposeFlagDiff: %v", err)
	}
	applied, err := Apply(claim, diff)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Body != "new behavior only" {
		t.Fatalf("expected confirmed flag-sourced reaudit to replace the whole body with now-does, got %q", applied.Body)
	}
	if len(applied.AuditNotes) != 1 || !strings.Contains(applied.AuditNotes[0], "flagged: rewritten") {
		t.Fatalf("expected the flag's reason recorded in AuditNotes, got: %v", applied.AuditNotes)
	}
}

// ---------------------------------------------------------------------
// FlagStore
// ---------------------------------------------------------------------

func TestFlagStore_LoadMissingFile_IsEmptyNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".docs-flag-store.json")
	store, err := LoadFlagStore(path)
	if err != nil {
		t.Fatalf("LoadFlagStore: %v", err)
	}
	if len(store.Flags) != 0 {
		t.Fatalf("expected an empty store for a missing file, got: %v", store.Flags)
	}
}

func TestFlagStore_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".docs-flag-store.json")
	store, err := LoadFlagStore(path)
	if err != nil {
		t.Fatalf("LoadFlagStore: %v", err)
	}
	store.Flags["widget.contract.main"] = PendingFlag{
		ClaimSays: "old", NowDoes: "new", Reason: "why", FlaggedAt: "2026-01-01T00:00:00Z",
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := LoadFlagStore(path)
	if err != nil {
		t.Fatalf("LoadFlagStore (reload): %v", err)
	}
	got, ok := reloaded.Flags["widget.contract.main"]
	if !ok {
		t.Fatalf("expected the saved flag to round-trip")
	}
	if got.ClaimSays != "old" || got.NowDoes != "new" || got.Reason != "why" {
		t.Fatalf("unexpected round-tripped flag: %+v", got)
	}
}

func TestFlagStore_DeleteThenSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".docs-flag-store.json")
	store, _ := LoadFlagStore(path)
	store.Flags["widget.contract.main"] = PendingFlag{ClaimSays: "a", NowDoes: "b", Reason: "c"}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	delete(store.Flags, "widget.contract.main")
	if err := store.Save(); err != nil {
		t.Fatalf("Save (after delete): %v", err)
	}

	reloaded, err := LoadFlagStore(path)
	if err != nil {
		t.Fatalf("LoadFlagStore: %v", err)
	}
	if len(reloaded.Flags) != 0 {
		t.Fatalf("expected the flag removed after a confirmed reaudit, got: %v", reloaded.Flags)
	}
}
