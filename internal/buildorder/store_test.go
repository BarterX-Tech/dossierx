package buildorder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	path := filepath.Join(t.TempDir(), "build", "build-order", "widget.json")
	_, err := LoadArtifact(path)
	if err == nil || !errors.Is(err, ErrNotProposed) {
		t.Fatalf("expected an ErrNotProposed-wrapping error, got: %v", err)
	}
}

func TestStatus_MissingFile_WrapsErrNotProposed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build", "build-order", "widget.json")
	_, err := Status(path, nil, nil)
	if err == nil || !errors.Is(err, ErrNotProposed) {
		t.Fatalf("expected an ErrNotProposed-wrapping error, got: %v", err)
	}
}

func TestLock_MissingFile_Refuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build", "build-order", "widget.json")
	_, err := Lock(path, nil, nil)
	if err == nil || !errors.Is(err, ErrNotProposed) {
		t.Fatalf("expected Lock to refuse against a not-yet-proposed artifact, got: %v", err)
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	// Issue #58: the frozen artifact is its own baseline. No content-hash
	// snapshot is written, so nothing can later compare claim CONTENT against
	// it.
	if len(locked.Hashes) != 0 {
		t.Fatalf("expected no content-hash snapshot on a locked artifact, got %v", locked.Hashes)
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

	// A prose edit to a covered claim (the issue #58 case): its body changes,
	// nothing the order is derived from does. The order must NOT go stale — a
	// fresh propose would produce the identical artifact, and reporting stale
	// here is what cost a human approval per reworded sentence.
	prose := make([]model.Claim, len(claims))
	copy(prose, claims)
	for i, c := range prose {
		if c.ID == "widget.contract.schema" {
			c.Body = "schema definition reworded after lock"
			prose[i] = c
		}
	}
	stProse, err := Status(path, prose, nil)
	if err != nil {
		t.Fatalf("Status after prose edit: %v", err)
	}
	if stProse.Stale || len(stProse.StaleIDs) != 0 {
		t.Fatalf("a prose edit must not make the order stale (issue #58), got stale=%v stale_claim_ids=%v", stProse.Stale, stProse.StaleIDs)
	}
	if _, err := Lock(path, prose, nil); err == nil || errors.Is(err, ErrStale) {
		t.Fatalf("after a prose edit Lock must refuse as already-locked-and-current, not as stale; got: %v", err)
	}

	// Move a derivation input: the behavior claim's rests_on edge is
	// redirected. Status must now report stale, naming exactly the claim
	// whose input moved — and nothing else.
	mutated := make([]model.Claim, len(claims))
	copy(mutated, claims)
	for i, c := range mutated {
		if c.ID == "widget.contract.behavior" {
			c.RestsOn = []string{"widget.contract.orient"}
			mutated[i] = c
		}
	}

	st3, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status after mutation: %v", err)
	}
	if !st3.Stale {
		t.Fatalf("expected stale=true after moving a covered claim's rests_on")
	}
	if len(st3.StaleIDs) != 1 || st3.StaleIDs[0] != "widget.contract.behavior" {
		t.Fatalf("expected stale_claim_ids=[widget.contract.behavior], got %v", st3.StaleIDs)
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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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

	// Move a covered claim's build_role so the artifact goes stale (a body
	// edit would not — see TestRecomputeStale_ProseEdit_NotStale).
	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.schema" {
			mutated[i].BuildRole = model.BuildRoleOrientation
		}
	}

	_, err = Lock(path, mutated, nil)
	if err == nil {
		t.Fatalf("expected Lock to refuse a stale artifact")
	}
	if !errors.Is(err, ErrStale) {
		t.Fatalf("expected the refusal to wrap ErrStale, got: %v", err)
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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	// Propose, which would now error on it). The covered claim is left
	// untouched so the excluded-loop classification, not a covered-claim
	// check, is what surfaces the staleness.
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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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

// TestRecomputeStale_CoveredClaimRestsOnReordered_IsStale pins that the
// artifact's rests_on ECHO is part of what a fresh propose must reproduce:
// reordering a claim's rests_on list (same target set, different order) does NOT
// change its layeredTopoSort placement (deps are set-based), but the artifact
// records rests_on verbatim and the viewer and "build-order show" draw the edges
// from that record, so a fresh propose would write a different artifact. Before
// issue #58 this was caught only as a side effect of the content-hash check;
// now the per-entry rests_on comparison and the structural signature both carry
// it, so removing the content hash did not lose it.
func TestRecomputeStale_CoveredClaimRestsOnReordered_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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
	path := filepath.Join(dir, "build", "build-order", "widget.json")

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

// ---------------------------------------------------------------------
// Issue #58: staleness keys on the derivation inputs, never on claim content
// ---------------------------------------------------------------------

// proposeAndLock is the shared fixture step for the issue #58 cases: propose
// module's order from claims, write it to path, and lock it under a fixed
// clock, failing the test on any refusal.
func proposeAndLock(t *testing.T, path string, claims []model.Claim, module string) *Artifact {
	t.Helper()
	a, err := Propose(claims, nil, module)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	locked, err := Lock(path, claims, nil)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	return locked
}

// TestRecomputeStale_ProseEdit_NotStale is the issue #58 regression test. A
// locked order used to report stale after ANY content change to a covered claim
// (the check compared a per-claim lock.ContentHash snapshot), and the documented
// recovery — re-propose, then re-lock — releases the standing approval in the
// ledger. So every reworded sentence in a still-being-audited spec cost a human
// approval that decided nothing.
//
// Body, steps and section are edited on TWO covered claims and one excluded
// claim; none of it is a derivation input, so the order must stay stale:false
// with an empty stale_claim_ids — and the on-disk artifact must be untouched,
// because Status is a read.
func TestRecomputeStale_ProseEdit_NotStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "widget.json")

	claims := []model.Claim{
		mc("widget.contract.orient", "widget", model.BuildRoleOrientation),
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
		mc("widget.contract.api", "widget", model.BuildRoleAPI, "widget.contract.behavior"),
		mc("widget.contract.future", "widget", model.BuildRoleOutOfScope),
	}
	proposeAndLock(t, path, claims, "widget")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	edited := append([]model.Claim{}, claims...)
	for i := range edited {
		switch edited[i].ID {
		case "widget.contract.schema":
			edited[i].Body = "the schema, reworded during audit"
			edited[i].Section = "Data shapes"
		case "widget.contract.api":
			edited[i].Steps = []string{"call it", "check the response"}
		case "widget.contract.future":
			edited[i].Body = "still deferred, note tightened"
		}
	}
	// Guard: these edits DO move the content hash the old check compared, or
	// this test would assert nothing about issue #58.
	for i := range claims {
		if claims[i].ID != "widget.contract.orient" && claims[i].ID != "widget.contract.behavior" &&
			lock.ContentHash(claims[i]) == lock.ContentHash(edited[i]) {
			t.Fatalf("test setup error: the prose edit to %s did not change lock.ContentHash", claims[i].ID)
		}
	}

	st, err := Status(path, edited, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Stale || len(st.StaleIDs) != 0 {
		t.Fatalf("issue #58: prose edits must not make the order stale, got stale=%v stale_claim_ids=%v", st.Stale, st.StaleIDs)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact after Status: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Status must not rewrite the artifact")
	}

	// And the count a caller shows ("N claim(s) moved") counts ONLY claims
	// whose derivation inputs moved: add one real move on top of the prose
	// edits and exactly that one id must be named.
	moved := append([]model.Claim{}, edited...)
	for i := range moved {
		if moved[i].ID == "widget.contract.orient" {
			moved[i].BuildRole = model.BuildRoleVerification
		}
	}
	st, err = Status(path, moved, nil)
	if err != nil {
		t.Fatalf("Status after a real move: %v", err)
	}
	if !st.Stale || len(st.StaleIDs) != 1 || st.StaleIDs[0] != "widget.contract.orient" {
		t.Fatalf("expected exactly the moved claim named, got stale=%v stale_claim_ids=%v", st.Stale, st.StaleIDs)
	}
}

// TestRecomputeStale_CoveredClaimRestsOnRetargeted_IsStale: a rests_on edit that
// actually reorders a phase. b rested on a (layers: [a c] [b]); after the edit b
// rests on c (layers: [a c] [b] still — but the recorded edge differs) and c is
// moved to rest on b, which flips the two: a fresh propose now yields [a b] [c].
func TestRecomputeStale_CoveredClaimRestsOnRetargeted_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "widget.json")

	claims := []model.Claim{
		mc("widget.contract.a", "widget", model.BuildRoleBehavior),
		mc("widget.contract.b", "widget", model.BuildRoleBehavior, "widget.contract.a"),
		mc("widget.contract.c", "widget", model.BuildRoleBehavior),
	}
	proposeAndLock(t, path, claims, "widget")

	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.c" {
			mutated[i].RestsOn = []string{"widget.contract.b"}
		}
	}
	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale {
		t.Fatalf("expected stale=true after retargeting a covered claim's rests_on, got %+v", st)
	}
	if indexOf(st.StaleIDs, "widget.contract.c") < 0 {
		t.Fatalf("expected the retargeted claim named, got %v", st.StaleIDs)
	}
	if indexOf(st.StaleIDs, "widget.contract.a") >= 0 {
		t.Fatalf("a's inputs and placement did not move; it must not be named, got %v", st.StaleIDs)
	}
	if _, err := Lock(path, mutated, nil); !errors.Is(err, ErrStale) {
		t.Fatalf("expected Lock to refuse with ErrStale, got: %v", err)
	}
}

// TestRecomputeStale_CoveredClaimReclassifiedOutOfScope_IsStale is the
// exclusion-set change in the covered direction (the excluded->in-phase
// direction is TestRecomputeStale_ExcludedClaimBuildRoleChangedToInPhase_IsStale):
// a placed claim whose build_role becomes out-of-scope leaves the sequence and
// joins the excluded set, so a fresh propose differs in both.
func TestRecomputeStale_CoveredClaimReclassifiedOutOfScope_IsStale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior),
	}
	proposeAndLock(t, path, claims, "widget")

	mutated := append([]model.Claim{}, claims...)
	for i := range mutated {
		if mutated[i].ID == "widget.contract.behavior" {
			mutated[i].BuildRole = model.BuildRoleOutOfScope
		}
	}
	st, err := Status(path, mutated, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Stale || len(st.StaleIDs) != 1 || st.StaleIDs[0] != "widget.contract.behavior" {
		t.Fatalf("expected exactly the reclassified claim named, got stale=%v stale_claim_ids=%v", st.Stale, st.StaleIDs)
	}
}

// TestRecomputeStale_LegacyArtifactWithHashes_JudgedByRederivation pins the
// upgrade rule for an artifact written by a release that still snapshotted
// lock.ContentHash into `hashes`. The map is loaded (so the artifact re-marshals
// byte-identically for its ledger signature) and otherwise ignored: the order
// is judged by re-deriving and comparing, exactly like a new one. The stored
// hashes here deliberately match NOTHING — the old check would have flagged
// every covered claim — and the verdict must still be stale:false, because a
// fresh propose reproduces the artifact. A real move must still be caught.
func TestRecomputeStale_LegacyArtifactWithHashes_JudgedByRederivation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
		mc("widget.contract.future", "widget", model.BuildRoleOutOfScope),
	}
	legacy := `{
  "module": "widget",
  "locked": true,
  "locked_at": "2026-01-01T00:00:00Z",
  "stale": false,
  "excluded": ["widget.contract.future"],
  "phases": [
    {"phase": "schema", "claims": [{"id": "widget.contract.schema", "file": "widget.contract.schema.yaml"}]},
    {"phase": "behavior", "claims": [{"id": "widget.contract.behavior", "file": "widget.contract.behavior.yaml", "rests_on": ["widget.contract.schema"]}]}
  ],
  "hashes": {
    "widget.contract.schema": "0000000000000000000000000000000000000000000000000000000000000000",
    "widget.contract.behavior": "0000000000000000000000000000000000000000000000000000000000000000"
  }
}
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy artifact: %v", err)
	}
	// Guard: the stored hashes really do disagree with the claims' content,
	// so the old rule would have said stale.
	for _, c := range claims[:2] {
		if lock.ContentHash(c) == strings.Repeat("0", 64) {
			t.Fatalf("test setup error: the legacy hash accidentally matches %s", c.ID)
		}
	}

	st, err := Status(path, claims, nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Stale || len(st.StaleIDs) != 0 {
		t.Fatalf("a legacy artifact whose order a fresh propose reproduces must be stale=false, got stale=%v stale_claim_ids=%v", st.Stale, st.StaleIDs)
	}
	if len(st.Hashes) != 2 {
		t.Fatalf("the legacy hashes map must survive the load untouched (its ledger signature covers it), got %v", st.Hashes)
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"hashes":{`) {
		t.Fatalf("a loaded legacy artifact must re-marshal with its hashes map, got %s", raw)
	}

	// A real move on the same legacy artifact is still caught.
	moved := append([]model.Claim{}, claims...)
	for i := range moved {
		if moved[i].ID == "widget.contract.behavior" {
			moved[i].RestsOn = nil
		}
	}
	st, err = Status(path, moved, nil)
	if err != nil {
		t.Fatalf("Status after move: %v", err)
	}
	if !st.Stale || indexOf(st.StaleIDs, "widget.contract.behavior") < 0 {
		t.Fatalf("expected the moved claim named on a legacy artifact, got stale=%v stale_claim_ids=%v", st.Stale, st.StaleIDs)
	}
}

// TestLock_RefusesASplicedRestsOn: the hand-edit gate now sees the rests_on
// echo. Dropping an edge from the artifact between propose and lock used to
// pass structuralDivergence (which compared phase/position/file only) and be
// signed into the ledger; the viewer then drew an order missing a dependency
// the claims still declare.
func TestLock_RefusesASplicedRestsOn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "widget.json")

	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	for pi := range a.Phases {
		for ci := range a.Phases[pi].Claims {
			a.Phases[pi].Claims[ci].RestsOn = nil
		}
	}
	if err := WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	fixedNow(t, time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC))
	_, err = Lock(path, claims, nil)
	if !errors.Is(err, ErrHandEdited) {
		t.Fatalf("expected Lock to refuse a rests_on splice as hand-edited, got: %v", err)
	}
	if !strings.Contains(err.Error(), "rests_on") {
		t.Fatalf("expected the refusal to name rests_on, got: %v", err)
	}
}

// ---------------------------------------------------------------------
// Graph-safety scale evidence (.agents/skills/dossierx-graph-safety)
// ---------------------------------------------------------------------

// scaleClaims builds one module's worst-case shapes for the staleness
// re-derivation: a deep behavior chain of depth claims (each rests_on the
// previous one) and a layered dense DAG of layers x width behavior claims where
// every claim rests_on EVERY claim of the previous layer — exponentially many
// routes, O(layers * width^2) edges — plus one other-module claim every chain
// claim also rests_on (a cross-module edge, informational only). Ids are
// returned in source order so stableDisplayOrder's tiebreak is deterministic.
func scaleClaims(depth, layers, width int) (claims []model.Claim, edges int) {
	claims = append(claims, mc("other.contract.x", "other", model.BuildRoleSchema))
	prev := ""
	for i := 0; i < depth; i++ {
		id := fmt.Sprintf("scale.contract.chain%04d", i)
		deps := []string{"other.contract.x"}
		edges++
		if prev != "" {
			deps = append(deps, prev)
			edges++
		}
		claims = append(claims, mc(id, "scale", model.BuildRoleBehavior, deps...))
		prev = id
	}
	var prevLayer []string
	for l := 0; l < layers; l++ {
		var layer []string
		for w := 0; w < width; w++ {
			id := fmt.Sprintf("scale.contract.dag%dx%d", l, w)
			claims = append(claims, mc(id, "scale", model.BuildRoleBehavior, prevLayer...))
			edges += len(prevLayer)
			layer = append(layer, id)
		}
		prevLayer = layer
	}
	return claims, edges
}

// TestRecomputeStale_Scale_BoundedByClaimsAndEdges is the adversarial scale
// evidence for the issue #58 rule. The re-derivation is layeredTopoSort over
// the module (Kahn's algorithm: O(V + E) work, one visit per claim and one
// decrement per edge, never a walk over routes) plus one signature per placed
// claim of O(len(id) + sum of its rests_on ids) bytes, so on a layered dense
// DAG with exponentially many routes the work, the allocations and the artifact
// bytes must all follow V + E, not the route count. The budgets below are
// asserted as structural allocation counts (deterministic on a given toolchain;
// wall time is logged, not asserted) and as serialized artifact bytes.
//
// It also proves the two semantic ends of the rule at scale: a prose edit on
// EVERY covered claim leaves the order stale:false with no ids, and one
// rests_on edit that CLOSES A CYCLE (so a fresh propose errors) terminates and
// names only the edited claim rather than the whole coverage.
func TestRecomputeStale_Scale_BoundedByClaimsAndEdges(t *testing.T) {
	for _, size := range []struct{ depth, layers, width int }{
		{32, 4, 16},
		{64, 6, 24},
		{128, 8, 32},
	} {
		t.Run(fmt.Sprintf("chain%d_dag%dx%d", size.depth, size.layers, size.width), func(t *testing.T) {
			scaleCase(t, size.depth, size.layers, size.width)
		})
	}
}

func scaleCase(t *testing.T, depth, layers, width int) {
	t.Helper()
	claims, edges := scaleClaims(depth, layers, width)
	moduleClaims := 0
	for _, c := range claims {
		if c.Module == "scale" {
			moduleClaims++
		}
	}
	if moduleClaims != depth+layers*width {
		t.Fatalf("fixture: expected %d module claims, got %d", depth+layers*width, moduleClaims)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "build", "build-order", "scale.json")
	locked := proposeAndLock(t, path, claims, "scale")
	raw, err := json.Marshal(locked)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Output bytes: every placed claim carries its id, file and rests_on echo,
	// so the artifact is O(sum over claims of (id + file + sum rests_on ids))
	// bytes. Ids here are <= 32 bytes; the budget is 96 bytes per claim (id,
	// file, JSON scaffolding) plus 40 per edge (quoted id, comma, indentation).
	if byteBudget := 96*moduleClaims + 40*edges; len(raw) > byteBudget {
		t.Fatalf("artifact bytes %d exceed the V+E budget %d (V=%d, E=%d)", len(raw), byteBudget, moduleClaims, edges)
	}

	// Prose edit on EVERY covered claim: not stale, nothing named.
	prose := append([]model.Claim{}, claims...)
	for i := range prose {
		prose[i].Body = "reworded " + prose[i].ID
	}
	start := time.Now()
	st, err := Status(path, prose, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Stale || len(st.StaleIDs) != 0 {
		t.Fatalf("prose edits on every claim must not make the order stale, got stale=%v, %d ids", st.Stale, len(st.StaleIDs))
	}

	// Allocations of the re-check itself (load excluded): measured per run and
	// bounded by V + E. The constant is generous headroom over the measured
	// value on go1.26 so toolchain drift does not flake it, but it is linear in
	// V + E by construction — a route-enumerating regression on this DAG would
	// allocate orders of magnitude more.
	loaded, err := LoadArtifact(path)
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	allocs := testing.AllocsPerRun(5, func() { recomputeStale(loaded, prose, nil) })
	if allocBudget := float64(40*moduleClaims + 4*edges); allocs > allocBudget {
		t.Fatalf("recomputeStale allocated %.0f times, over the V+E budget %.0f (V=%d, E=%d)", allocs, allocBudget, moduleClaims, edges)
	}
	t.Logf("scale: V=%d module claims, E=%d edges, artifact=%d bytes, Status=%s, recomputeStale allocs/run=%.0f",
		moduleClaims, edges, len(raw), elapsed, allocs)

	// One edge that closes a cycle in the chain: a fresh propose ERRORS, the
	// structural comparison cannot run, and the per-input rests_on check names
	// exactly the edited claim. Terminates (layeredTopoSort's cycle guard) and
	// does not flag the whole coverage.
	cyclic := append([]model.Claim{}, prose...)
	first := "scale.contract.chain0000"
	last := fmt.Sprintf("scale.contract.chain%04d", depth-1)
	for i := range cyclic {
		if cyclic[i].ID == first {
			cyclic[i].RestsOn = append([]string{"other.contract.x"}, last)
		}
	}
	st, err = Status(path, cyclic, nil)
	if err != nil {
		t.Fatalf("Status on a cyclic edit: %v", err)
	}
	if !st.Stale || len(st.StaleIDs) != 1 || st.StaleIDs[0] != first {
		t.Fatalf("a cycle-closing rests_on edit must name exactly the edited claim, got stale=%v ids=%v", st.Stale, st.StaleIDs)
	}
}
