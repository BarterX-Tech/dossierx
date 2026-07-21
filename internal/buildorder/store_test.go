package buildorder

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func fixedNow(t *testing.T, ts time.Time) {
	t.Helper()
	old := nowFunc
	nowFunc = func() time.Time { return ts }
	t.Cleanup(func() { nowFunc = old })
}

func TestLoadArtifact_MissingFile_WrapsErrNotProposed(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".build-order.widget.json")
	_, err := LoadArtifact(path)
	if err == nil || !errors.Is(err, ErrNotProposed) {
		t.Fatalf("expected an ErrNotProposed-wrapping error, got: %v", err)
	}
}

func TestStatus_MissingFile_WrapsErrNotProposed(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".build-order.widget.json")
	_, err := Status(path, nil)
	if err == nil || !errors.Is(err, ErrNotProposed) {
		t.Fatalf("expected an ErrNotProposed-wrapping error, got: %v", err)
	}
}

func TestLock_MissingFile_Refuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".build-order.widget.json")
	_, err := Lock(path, nil)
	if err == nil || !errors.Is(err, ErrNotProposed) {
		t.Fatalf("expected Lock to refuse against a not-yet-proposed artifact, got: %v", err)
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	got, err := LoadArtifact(path)
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if got.Module != "widget" || got.Locked {
		t.Fatalf("expected round-tripped artifact module=widget locked=false, got %+v", got)
	}
	if len(got.Phases) != 1 || got.Phases[0].Phase != "schema" {
		t.Fatalf("expected the schema phase to round-trip, got %+v", got.Phases)
	}
}

// TestFullLifecycle_ProposeStatusLockThenStale is the end-to-end propose ->
// status -> lock -> mutate -> status-is-stale flow described in the task
// (a synthetic, in-memory claim set spanning 3+ phases; no filesystem
// claims, just the artifact file).
func TestFullLifecycle_ProposeStatusLockThenStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.orient", "widget", model.BuildRoleOrientation),
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
	}

	// propose
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	// status: proposed, not locked, not stale (no baseline yet).
	st, err := Status(path, claims)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Locked || st.Stale {
		t.Fatalf("expected proposed-only artifact to be locked=false stale=false, got %+v", st)
	}

	// lock
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	locked, err := Lock(path, claims)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if !locked.Locked {
		t.Fatalf("expected Locked=true after Lock")
	}
	if locked.LockedAt != "2026-07-19T12:00:00Z" {
		t.Fatalf("expected LockedAt stamped to the fixed clock, got %q", locked.LockedAt)
	}
	if len(locked.Hashes) != 3 {
		t.Fatalf("expected a hash snapshot for all 3 covered claims, got %d", len(locked.Hashes))
	}

	// status immediately after lock: not stale.
	st2, err := Status(path, claims)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st2.Stale {
		t.Fatalf("expected freshly-locked artifact to be non-stale, got stale_claim_ids=%v", st2.StaleIDs)
	}

	// Lock again with nothing changed: refused.
	if _, err := Lock(path, claims); err == nil {
		t.Fatalf("expected Lock to refuse re-locking an already-locked, non-stale artifact")
	}

	// Mutate a covered claim's body: status must now report stale, naming
	// the changed claim id.
	mutated := make([]model.Claim, len(claims))
	copy(mutated, claims)
	for i, c := range mutated {
		if c.ID == "widget.contract.schema" {
			c.Body = "schema definition changed after lock"
			mutated[i] = c
		}
	}

	st3, err := Status(path, mutated)
	if err != nil {
		t.Fatalf("Status after mutation: %v", err)
	}
	if !st3.Stale {
		t.Fatalf("expected stale=true after mutating a covered claim")
	}
	if len(st3.StaleIDs) != 1 || st3.StaleIDs[0] != "widget.contract.schema" {
		t.Fatalf("expected stale_claim_ids=[widget.contract.schema], got %v", st3.StaleIDs)
	}

	// Relock is now allowed (stale, even though already locked) and clears
	// staleness again.
	relocked, err := Lock(path, mutated)
	if err != nil {
		t.Fatalf("expected Lock to allow relocking a stale artifact, got: %v", err)
	}
	if relocked.Stale {
		t.Fatalf("expected staleness cleared after a successful relock")
	}
}

// TestRecomputeStale_CoveredClaimDeleted_IsStale is a regression test: an
// earlier version of recomputeStale treated a covered claim's outright
// disappearance (id present in the artifact, absent from the current
// claim set) as "nothing to hash-compare" and silently left it out of
// StaleIDs, so a locked artifact whose own claim had been deleted still
// reported stale:false. Deleting a covered claim must be surfaced as
// staleness, not swallowed.
func TestRecomputeStale_CoveredClaimDeleted_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
	}

	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	if _, err := Lock(path, claims); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Simulate one covered claim being deleted entirely (e.g. its file
	// removed from disk) — it's simply absent from the claim set Status
	// is given now, not just changed.
	remaining := []model.Claim{claims[0]}

	st, err := Status(path, remaining)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true when a covered claim has been deleted, got %+v", st)
	}
	found := false
	for _, id := range st.StaleIDs {
		if id == "widget.contract.behavior" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stale_claim_ids to include the deleted claim %q, got %v", "widget.contract.behavior", st.StaleIDs)
	}
}
