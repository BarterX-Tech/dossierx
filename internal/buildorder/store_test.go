package buildorder

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/lock"
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

	// A stale artifact is NOT bare-relockable (FIX-13): a bare relock would
	// freeze the OLD phase order (Lock never recomputes Phases) while
	// silently clearing staleness. Lock refuses and directs a re-propose.
	if _, err := Lock(path, mutated); err == nil {
		t.Fatalf("expected Lock to refuse a stale artifact and direct a re-propose")
	}

	// Re-propose regenerates the order against the mutated claim set, then
	// lock succeeds and clears staleness — the SKILL's re-propose-then-lock
	// flow.
	fresh, err := Propose(mutated, nil, "widget")
	if err != nil {
		t.Fatalf("re-Propose: %v", err)
	}
	if err := WriteArtifact(fresh, path); err != nil {
		t.Fatalf("WriteArtifact after re-propose: %v", err)
	}
	relocked, err := Lock(path, mutated)
	if err != nil {
		t.Fatalf("expected lock to succeed after re-propose, got: %v", err)
	}
	if relocked.Stale {
		t.Fatalf("expected staleness cleared after re-propose + lock")
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

// TestRecomputeStale_AddedClaim_IsStale is the FIX-12 regression test:
// recomputeStale used to compare only the artifact's frozen ClaimIDs()
// against their hashes (handling deletion + content change) but never noticed
// a NEW claim locked into an already-covered module. The artifact would keep
// reporting stale:false while silently omitting that claim from the order.
// Coverage that grows must be surfaced as staleness, symmetric with the
// deletion case, using a.ClaimIDs() UNION a.Excluded as the "already covered"
// set so legitimately out-of-scope claims never false-positive.
func TestRecomputeStale_AddedClaim_IsStale(t *testing.T) {
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

	// A brand-new claim is locked into the same (fully-covered) module.
	augmented := append(append([]model.Claim{}, claims...),
		mc("widget.contract.api", "widget", model.BuildRoleAPI, "widget.contract.behavior"))

	st, err := Status(path, augmented)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true when a new claim is locked into a covered module, got %+v", st)
	}
	found := false
	for _, id := range st.StaleIDs {
		if id == "widget.contract.api" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stale_claim_ids to include the newly added claim %q, got %v", "widget.contract.api", st.StaleIDs)
	}
}

// TestRecomputeStale_AddedOutOfScopeClaim_NoFalsePositive guards the Excluded
// union in FIX-12: an out-of-scope claim that was already recorded as
// Excluded at propose time must NOT be flagged stale on a later re-check just
// because it isn't in ClaimIDs() (it never is — Excluded claims are never
// placed in a phase).
func TestRecomputeStale_AddedOutOfScopeClaim_NoFalsePositive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.future", "widget", model.BuildRoleOutOfScope),
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

	st, err := Status(path, claims)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Stale {
		t.Fatalf("expected an unchanged artifact with an excluded claim to stay non-stale, got stale_claim_ids=%v", st.StaleIDs)
	}
}

// TestRecomputeStale_CoveredClaimBuildRoleChanged_IsStale is the GAP-3a
// regression test: a covered claim's PHASE is derived from its build_role,
// but recomputeStale previously compared only lock.ContentHash, which
// deliberately EXCLUDES build_role. So changing a covered locked claim's
// build_role (e.g. schema -> orientation) silently changed what a fresh
// propose would order while the artifact still reported stale:false — the
// same silently-wrong-order class as the deletion/addition gaps. A build_role
// change on a covered claim must be surfaced as staleness even though the
// content hash is unchanged.
func TestRecomputeStale_CoveredClaimBuildRoleChanged_IsStale(t *testing.T) {
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

	// Change one covered claim's build_role only (schema -> orientation).
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.schema" {
			mutated[i].BuildRole = model.BuildRoleOrientation
		}
	}
	// Guard: the mutation is build_role-only, so lock.ContentHash (which
	// excludes build_role) is unchanged — a passing test therefore proves the
	// phase check, not the content-hash check, is what surfaces the staleness.
	if lock.ContentHash(claims[0]) != lock.ContentHash(mutated[0]) {
		t.Fatalf("test setup error: a build_role change must not alter lock.ContentHash")
	}

	st, err := Status(path, mutated)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after changing a covered claim's build_role, got %+v", st)
	}
	found := false
	for _, id := range st.StaleIDs {
		if id == "widget.contract.schema" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stale_claim_ids to include the build_role-changed claim %q, got %v", "widget.contract.schema", st.StaleIDs)
	}
}

// TestRecomputeStale_ExcludedClaimDeleted_IsStale is the GAP-3b regression
// test: recomputeStale's deletion loop only iterated the artifact's in-phase
// coverage (a.ClaimIDs()) and never a.Excluded, so deleting a claim recorded
// as out-of-scope left stale:false while a.Excluded still listed the gone id
// (a phantom "N excluded" count). Deleting an excluded claim must be surfaced
// as staleness, symmetric with the covered-deletion case.
func TestRecomputeStale_ExcludedClaimDeleted_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.future", "widget", model.BuildRoleOutOfScope),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if len(a.Excluded) != 1 || a.Excluded[0] != "widget.contract.future" {
		t.Fatalf("expected the out-of-scope claim recorded as excluded, got %v", a.Excluded)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	if _, err := Lock(path, claims); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// The excluded (out-of-scope) claim is deleted entirely from the claim set.
	remaining := []model.Claim{claims[0]}

	st, err := Status(path, remaining)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true when an excluded claim has been deleted, got %+v", st)
	}
	found := false
	for _, id := range st.StaleIDs {
		if id == "widget.contract.future" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stale_claim_ids to include the deleted excluded claim %q, got %v", "widget.contract.future", st.StaleIDs)
	}
}

// TestLock_RefusesStaleArtifact is the FIX-13 regression test: a bare relock
// of a stale artifact used to succeed, freezing the OLD phase order (Lock
// never recomputes Phases) while clearing staleness. Lock must instead refuse
// a stale artifact, leave the file untouched, and direct the user to
// re-propose first.
func TestLock_RefusesStaleArtifact(t *testing.T) {
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

	// Mutate a covered claim's body so the artifact goes stale.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.schema" {
			mutated[i].Body = "schema definition changed after lock"
		}
	}

	_, err = Lock(path, mutated)
	if err == nil {
		t.Fatalf("expected Lock to refuse a stale artifact")
	}
	if !strings.Contains(err.Error(), "propose") {
		t.Fatalf("expected the refusal to direct a re-propose, got: %v", err)
	}

	// The on-disk artifact must be left exactly as the original lock wrote it.
	after, err := LoadArtifact(path)
	if err != nil {
		t.Fatalf("LoadArtifact after refused relock: %v", err)
	}
	if !after.Locked {
		t.Fatalf("expected the artifact to remain locked after a refused relock")
	}
	if after.Stale {
		t.Fatalf("the persisted artifact must not have been rewritten with stale=true by a refused Lock")
	}
}

// TestLifecycle_OrientationNoteClaim_ProposesAndLocks is the FIX-14
// regression test (the code half of the SKILL correction): a
// kind:orientation-note claim that carries a build_role DOES participate in
// Build Order — Propose accepts it, places it in the orientation phase, and
// Lock freezes it cleanly. The SKILL previously (falsely) claimed such claims
// were invisible to Build Order and never carried a build_role.
func TestLifecycle_OrientationNoteClaim_ProposesAndLocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	orient := mc("widget.contract.readme", "widget", model.BuildRoleOrientation)
	orient.Kind = model.KindOrientationNote
	orient.Layout = model.LayoutBanner
	claims := []model.Claim{
		orient,
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
	}

	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose must accept a kind:orientation-note claim carrying build_role, got: %v", err)
	}
	ids := idsOf(onlyPhase(a, model.BuildRoleOrientation))
	if len(ids) != 1 || ids[0] != "widget.contract.readme" {
		t.Fatalf("expected the orientation-note claim placed in the orientation phase, got %v", ids)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	locked, err := Lock(path, claims)
	if err != nil {
		t.Fatalf("Lock must accept an orientation-note claim carrying build_role, got: %v", err)
	}
	if !locked.Locked {
		t.Fatalf("expected locked=true after locking an orientation-note-bearing module")
	}
}
