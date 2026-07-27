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
	_, err := Status(path, nil, nil)
	if err == nil || !errors.Is(err, ErrNotProposed) {
		t.Fatalf("expected an ErrNotProposed-wrapping error, got: %v", err)
	}
}

func TestLock_MissingFile_Refuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".build-order.widget.json")
	_, err := Lock(path, nil, nil)
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
	st, err := Status(path, claims, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Locked || st.Stale {
		t.Fatalf("expected proposed-only artifact to be locked=false stale=false, got %+v", st)
	}

	// lock
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	locked, err := Lock(path, claims, nil)
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
	st2, err := Status(path, claims, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st2.Stale {
		t.Fatalf("expected freshly-locked artifact to be non-stale, got stale_claim_ids=%v", st2.StaleIDs)
	}

	// Lock again with nothing changed: refused.
	if _, err := Lock(path, claims, nil); err == nil {
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

	st3, err := Status(path, mutated, nil)
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
	if _, err := Lock(path, mutated, nil); err == nil {
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
	relocked, err := Lock(path, mutated, nil)
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Simulate one covered claim being deleted entirely (e.g. its file
	// removed from disk) — it's simply absent from the claim set Status
	// is given now, not just changed.
	remaining := []model.Claim{claims[0]}

	st, err := Status(path, remaining, nil)
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// A brand-new claim is locked into the same (fully-covered) module.
	augmented := append(append([]model.Claim{}, claims...),
		mc("widget.contract.api", "widget", model.BuildRoleAPI, "widget.contract.behavior"))

	st, err := Status(path, augmented, nil)
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	st, err := Status(path, claims, nil)
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
	if _, err := Lock(path, claims, nil); err != nil {
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

	st, err := Status(path, mutated, nil)
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// The excluded (out-of-scope) claim is deleted entirely from the claim set.
	remaining := []model.Claim{claims[0]}

	st, err := Status(path, remaining, nil)
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

// TestRecomputeStale_ExcludedClaimBuildRoleChangedToInPhase_IsStale is the
// GAP-3c regression test, symmetric with the covered build_role-change case:
// recomputeStale's a.Excluded loop only flagged an excluded claim's DELETION,
// and the addition loop folds a.Excluded ids into the "already covered" set —
// so an out-of-scope claim later promoted to an in-phase build_role (e.g.
// out-of-scope -> schema) was examined by NOTHING and left stale:false, even
// though a fresh propose would now place it in a phase, a silently different
// order. Promoting an excluded claim into a build phase must be surfaced as
// staleness, mirroring Propose's own out-of-scope-vs-phase classification.
func TestRecomputeStale_ExcludedClaimBuildRoleChangedToInPhase_IsStale(t *testing.T) {
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Promote the formerly out-of-scope claim into an in-phase build_role
	// (out-of-scope -> schema). A fresh propose would now place it in the
	// schema phase, so the frozen artifact must report stale.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.future" {
			mutated[i].BuildRole = model.BuildRoleSchema
		}
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after promoting an excluded claim into a build phase, got %+v", st)
	}
	found := false
	for _, id := range st.StaleIDs {
		if id == "widget.contract.future" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stale_claim_ids to include the promoted claim %q, got %v", "widget.contract.future", st.StaleIDs)
	}
}

// TestRecomputeStale_ExcludedClaimEditedButStillOutOfScope_NoFalsePositive
// guards the promotion check above: an excluded claim whose body is edited but
// whose build_role stays out-of-scope is still not placed in any phase, so a
// fresh propose would compute the same order. It must NOT be flagged stale.
func TestRecomputeStale_ExcludedClaimEditedButStillOutOfScope_NoFalsePositive(t *testing.T) {
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Edit the excluded claim's body but keep it out-of-scope.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.future" {
			mutated[i].Body = "future scope notes revised, still deferred"
		}
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Stale {
		t.Fatalf("expected an excluded claim that stayed out-of-scope to keep the artifact non-stale, got stale_claim_ids=%v", st.StaleIDs)
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Mutate a covered claim's body so the artifact goes stale.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.schema" {
			mutated[i].Body = "schema definition changed after lock"
		}
	}

	_, err = Lock(path, mutated, nil)
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
	locked, err := Lock(path, claims, nil)
	if err != nil {
		t.Fatalf("Lock must accept an orientation-note claim carrying build_role, got: %v", err)
	}
	if !locked.Locked {
		t.Fatalf("expected locked=true after locking an orientation-note-bearing module")
	}
}

// TestRecomputeStale_LockedAllOutOfScope_PromotedClaim_IsStale is the DEFECT-1
// regression test (a): a module locked with ONLY out-of-scope claims has an
// empty ClaimIDs(), so Lock snapshots an empty (omitempty-dropped) Hashes map
// even though the artifact IS locked. recomputeStale used to early-return "not
// stale" on len(a.Hashes)==0, which skipped every drift check for such a
// module forever — so promoting one of its out-of-scope claims into a build
// phase (a silently different order a fresh propose would honor) went
// unnoticed. Staleness must run for any LOCKED artifact regardless of Hashes.
func TestRecomputeStale_LockedAllOutOfScope_PromotedClaim_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.futureA", "widget", model.BuildRoleOutOfScope),
		mc("widget.contract.futureB", "widget", model.BuildRoleOutOfScope),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if len(a.Phases) != 0 {
		t.Fatalf("expected an all-out-of-scope module to have no phases, got %+v", a.Phases)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	locked, err := Lock(path, claims, nil)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if !locked.Locked {
		t.Fatalf("expected the all-out-of-scope module to lock")
	}
	if len(locked.Hashes) != 0 {
		t.Fatalf("expected an empty hash snapshot for an all-out-of-scope module, got %v", locked.Hashes)
	}

	// Promote one out-of-scope claim into an in-phase build_role.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.futureA" {
			mutated[i].BuildRole = model.BuildRoleSchema
		}
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after promoting an out-of-scope claim in a locked all-out-of-scope module, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.futureA") < 0 {
		t.Fatalf("expected stale_claim_ids to include the promoted claim %q, got %v", "widget.contract.futureA", st.StaleIDs)
	}
}

// TestRecomputeStale_LockedAllOutOfScope_ExcludedDeleted_IsStale is the
// DEFECT-1 regression test (b): deleting an excluded claim from a locked
// all-out-of-scope module (empty Hashes) must surface as staleness. The old
// len(a.Hashes)==0 early-return swallowed it, keeping a phantom id in the
// excluded count for a claim that's gone.
func TestRecomputeStale_LockedAllOutOfScope_ExcludedDeleted_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.futureA", "widget", model.BuildRoleOutOfScope),
		mc("widget.contract.futureB", "widget", model.BuildRoleOutOfScope),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Delete one of the excluded claims entirely.
	remaining := []model.Claim{claims[0]}

	st, err := Status(path, remaining, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true when an excluded claim is deleted from a locked all-out-of-scope module, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.futureB") < 0 {
		t.Fatalf("expected stale_claim_ids to include the deleted excluded claim %q, got %v", "widget.contract.futureB", st.StaleIDs)
	}
}

// TestRecomputeStale_LockedAllOutOfScope_FirstInPhaseAdded_IsStale is the
// DEFECT-1 regression test (c): locking the FIRST in-phase claim into a module
// that was locked as all-out-of-scope (empty Hashes) must surface as
// staleness. The old early-return skipped the addition loop, so the frozen
// artifact silently omitted the new claim from the (empty) order forever.
func TestRecomputeStale_LockedAllOutOfScope_FirstInPhaseAdded_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.futureA", "widget", model.BuildRoleOutOfScope),
		mc("widget.contract.futureB", "widget", model.BuildRoleOutOfScope),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// A brand-new in-phase claim is locked into the (all-out-of-scope) module.
	augmented := append(append([]model.Claim{}, claims...),
		mc("widget.contract.schema", "widget", model.BuildRoleSchema))

	st, err := Status(path, augmented, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true when the first in-phase claim is added to a locked all-out-of-scope module, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.schema") < 0 {
		t.Fatalf("expected stale_claim_ids to include the newly added claim %q, got %v", "widget.contract.schema", st.StaleIDs)
	}
}

// TestRecomputeStale_LockedAllOutOfScope_Unchanged_NoFalsePositive guards the
// DEFECT-1 fix: running staleness for a LOCKED all-out-of-scope module must not
// false-positive when nothing has changed.
func TestRecomputeStale_LockedAllOutOfScope_Unchanged_NoFalsePositive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.futureA", "widget", model.BuildRoleOutOfScope),
		mc("widget.contract.futureB", "widget", model.BuildRoleOutOfScope),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	st, err := Status(path, claims, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Stale {
		t.Fatalf("expected an unchanged locked all-out-of-scope module to stay non-stale, got stale_claim_ids=%v", st.StaleIDs)
	}
}

// TestRecomputeStale_ExcludedClaimEditedToEmptyRole_IsStale is the DEFECT-2
// regression test (empty role): the excluded loop used to flag stale only when
// isKnownPhase(currentRole) was true, but Propose classifies a claim as
// excluded IFF build_role == out-of-scope. An excluded claim edited to an empty
// build_role is therefore no longer excluded (a fresh propose would ERROR on
// it), yet the old loop left it stale:false — an asymmetric, silent divergence
// from the covered path. It must flag stale.
func TestRecomputeStale_ExcludedClaimEditedToEmptyRole_IsStale(t *testing.T) {
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Edit the excluded claim's build_role to empty (no longer excluded by
	// Propose, which would now error on it). Covered claim left untouched so a
	// non-empty Hashes proves this is the excluded-loop classification, not the
	// DEFECT-1 guard, that surfaces the staleness.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.future" {
			mutated[i].BuildRole = ""
		}
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after editing an excluded claim's build_role to empty, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.future") < 0 {
		t.Fatalf("expected stale_claim_ids to include the reclassified claim %q, got %v", "widget.contract.future", st.StaleIDs)
	}
}

// TestRecomputeStale_ExcludedClaimEditedToInvalidRole_IsStale is the DEFECT-2
// regression test (invalid/typo role): symmetric with the empty-role case, an
// excluded claim edited to an invalid build_role is no longer classified
// excluded by Propose (which would error on it), so it must flag stale rather
// than silently staying stale:false.
func TestRecomputeStale_ExcludedClaimEditedToInvalidRole_IsStale(t *testing.T) {
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Edit the excluded claim's build_role to an invalid/typo value.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.future" {
			mutated[i].BuildRole = model.BuildRole("scheema")
		}
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after editing an excluded claim's build_role to an invalid value, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.future") < 0 {
		t.Fatalf("expected stale_claim_ids to include the reclassified claim %q, got %v", "widget.contract.future", st.StaleIDs)
	}
}

// TestRecomputeStale_UnlockedProposedArtifact_NeverStale guards the DEFECT-1
// invariant from the other side: an UNLOCKED (proposed-only) artifact is never
// stale — staleness is a locked-artifact concept — even if a covered claim's
// body has since changed. Keying the early-return on !a.Locked (not on
// len(a.Hashes)) must not start flagging unlocked artifacts.
func TestRecomputeStale_UnlockedProposedArtifact_NeverStale(t *testing.T) {
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

	// Mutate a covered claim's body, but never Lock the artifact.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.schema" {
			mutated[i].Body = "changed before ever locking"
		}
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Locked {
		t.Fatalf("test setup error: artifact must be unlocked")
	}
	if st.Stale {
		t.Fatalf("expected an unlocked proposed-only artifact to stay non-stale, got stale_claim_ids=%v", st.StaleIDs)
	}
}

// TestRecomputeStale_CoveredClaimOrderEdited_IsStale is the primary regression
// test for this fix: Propose sequences each phase via stableDisplayOrder, which
// reads each claim's Order field — but Order is in NEITHER lock.ContentHash NOR
// any per-input check recomputeStale previously ran. So editing ONLY the order:
// of a covered claim silently changed the within-phase sequence a fresh propose
// would compute while status reported stale:false. Flipping two same-phase
// claims' relative order via Order must now be surfaced as staleness by the
// structural re-derivation.
func TestRecomputeStale_CoveredClaimOrderEdited_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	// Two independent (no rests_on between them) behavior claims: both land in
	// the same phase's layer 0, so their relative order is decided purely by
	// stableDisplayOrder (Order field, then incoming order).
	claims := []model.Claim{
		mc("widget.contract.first", "widget", model.BuildRoleBehavior),
		mc("widget.contract.second", "widget", model.BuildRoleBehavior),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if got := idsOf(onlyPhase(a, model.BuildRoleBehavior)); len(got) != 2 || got[0] != "widget.contract.first" {
		t.Fatalf("test setup: expected the initial order [first, second], got %v", got)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Edit ONLY the order: of the second claim so a fresh propose would now
	// sequence it FIRST (a set Order sorts ahead of an unordered claim).
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.second" {
			mutated[i].Order = 1
		}
	}
	// Guard: an order: edit must not alter lock.ContentHash (which excludes
	// Order), so a passing test proves the structural re-derivation — not the
	// content-hash check — is what surfaces the staleness.
	if lock.ContentHash(claims[1]) != lock.ContentHash(mutated[1]) {
		t.Fatalf("test setup error: an order: edit must not alter lock.ContentHash")
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after editing a covered claim's order:, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.second") < 0 {
		t.Fatalf("expected stale_claim_ids to include the reordered claim %q, got %v", "widget.contract.second", st.StaleIDs)
	}
}

// TestRecomputeStale_CoveredClaimSourceFileRenamed_IsStale is the second
// regression test for this fix: Propose records each claim's source file into
// ClaimEntry.File (displayPath of model.Claim.SourcePath) — but SourcePath is
// in neither lock.ContentHash nor any prior per-input check. So renaming a
// covered claim's source file (id and body unchanged) silently changed the File
// a fresh propose would record while status reported stale:false. A source-file
// rename must now be surfaced as staleness by the structural re-derivation.
func TestRecomputeStale_CoveredClaimSourceFileRenamed_IsStale(t *testing.T) {
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
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Rename ONLY the source file of a covered claim (id and body unchanged).
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.schema" {
			mutated[i].SourcePath = "widget.contract.renamed.yaml"
		}
	}
	// Guard: a source-file rename must not alter lock.ContentHash (which
	// excludes SourcePath), so a passing test proves the ClaimEntry.File diff —
	// not the content-hash check — surfaces the staleness.
	if lock.ContentHash(claims[0]) != lock.ContentHash(mutated[0]) {
		t.Fatalf("test setup error: a source-file rename must not alter lock.ContentHash")
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after renaming a covered claim's source file, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.schema") < 0 {
		t.Fatalf("expected stale_claim_ids to include the renamed claim %q, got %v", "widget.contract.schema", st.StaleIDs)
	}
}

// TestRecomputeStale_CoveredClaimRestsOnReordered_IsStale keeps the content-hash
// path honest: reordering a claim's rests_on list (same target set, different
// order) does NOT change its layeredTopoSort placement (deps are set-based), so
// the structural re-derivation alone would miss it — but lock.ContentHash hashes
// rests_on in order, so the retained content-hash check still surfaces it. This
// guards against the structural diff being mistaken for a full replacement of
// the content-hash check.
func TestRecomputeStale_CoveredClaimRestsOnReordered_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.a", "widget", model.BuildRoleBehavior),
		mc("widget.contract.b", "widget", model.BuildRoleBehavior),
		mc("widget.contract.c", "widget", model.BuildRoleBehavior, "widget.contract.a", "widget.contract.b"),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	if _, err := Lock(path, claims, nil); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Reverse c's rests_on order (same set): topo placement is unchanged, but
	// the ordered content hash differs.
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.c" {
			mutated[i].RestsOn = []string{"widget.contract.b", "widget.contract.a"}
		}
	}

	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after reordering a covered claim's rests_on, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.c") < 0 {
		t.Fatalf("expected stale_claim_ids to include the rests_on-reordered claim %q, got %v", "widget.contract.c", st.StaleIDs)
	}
}

// ---------------------------------------------------------------------
// The hand-edit gate (ErrHandEdited)
// ---------------------------------------------------------------------

// Reversing the phase blocks between propose and lock used to be frozen
// verbatim: recomputeStale early-returns on an UNLOCKED artifact, and an
// unlocked artifact is the only input Lock accepts, so the structural
// re-derivation never ran on what Lock was actually about to sign. The order an
// implementing agent follows — and the ledger record taken over it — were both
// the attacker's.
func TestLock_RefusesAReversedPhaseSequence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.orient", "widget", model.BuildRoleOrientation),
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
	}

	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if len(a.Phases) != 3 {
		t.Fatalf("precondition: expected three phase blocks, got %d", len(a.Phases))
	}
	// The hand edit: build behavior before schema before orientation.
	for i, j := 0, len(a.Phases)-1; i < j; i, j = i+1, j-1 {
		a.Phases[i], a.Phases[j] = a.Phases[j], a.Phases[i]
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	locked, err := Lock(path, claims, nil)
	if !errors.Is(err, ErrHandEdited) {
		t.Fatalf("expected Lock to refuse a hand-reordered artifact with ErrHandEdited, got %v", err)
	}
	if locked != nil {
		t.Fatalf("a refused lock must return no artifact, got %+v", locked)
	}

	// And nothing was written: the artifact on disk is still unlocked, so the
	// refusal cannot be laundered by reading it back.
	onDisk, err := LoadArtifact(path)
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if onDisk.Locked {
		t.Fatalf("a refused lock must leave the artifact unlocked on disk")
	}
}

// The same gate, on the other hand-editable field a reader would never notice:
// ClaimEntry.File is where the viewer and an implementing agent are told to look
// for the claim's source.
func TestLock_RefusesARepointedClaimFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{mc("widget.contract.schema", "widget", model.BuildRoleSchema)}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	a.Phases[0].Claims[0].File = "/etc/passwd"
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	if _, err := Lock(path, claims, nil); !errors.Is(err, ErrHandEdited) {
		t.Fatalf("expected ErrHandEdited for a repointed ClaimEntry.File, got %v", err)
	}
}

// Adding an id to `excluded` by hand takes a claim out of the build sequence
// entirely — the quietest possible edit, since the phases still read correctly.
func TestLock_RefusesASmuggledExclusion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	a.Excluded = append(a.Excluded, "widget.contract.behavior")
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	if _, err := Lock(path, claims, nil); !errors.Is(err, ErrHandEdited) {
		t.Fatalf("expected ErrHandEdited for a hand-added exclusion, got %v", err)
	}
}

// The gate must be SILENT on the honest path — propose, then lock, unedited —
// or it would refuse every legitimate build order in every project.
func TestLock_AcceptsAFreshlyProposedArtifact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".build-order.widget.json")

	claims := []model.Claim{
		mc("widget.contract.orient", "widget", model.BuildRoleOrientation),
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
		mc("widget.contract.spare", "widget", model.BuildRoleOutOfScope),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	locked, err := Lock(path, claims, nil)
	if err != nil {
		t.Fatalf("an unedited, freshly-proposed artifact must lock: %v", err)
	}
	if !locked.Locked || locked.Stale {
		t.Fatalf("expected locked=true stale=false, got %+v", locked)
	}
}
