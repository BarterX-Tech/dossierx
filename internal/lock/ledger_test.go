package lock

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// captureAnnouncements redirects the grandfathering notice for one test and
// returns the buffer it lands in.
func captureAnnouncements(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := ledgerAnnounceWriter
	ledgerAnnounceWriter = &buf
	t.Cleanup(func() { ledgerAnnounceWriter = orig })
	return &buf
}

// silenceAnnouncements suppresses the notice for tests that are not asserting
// on it, so a passing test run stays readable.
func silenceAnnouncements(t *testing.T) {
	t.Helper()
	orig := ledgerAnnounceWriter
	ledgerAnnounceWriter = io.Discard
	t.Cleanup(func() { ledgerAnnounceWriter = orig })
}

// freezeClock pins nowFunc so recorded timestamps can be compared exactly
// rather than raced.
func freezeClock(t *testing.T, at string) {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		t.Fatalf("freezeClock: %v", err)
	}
	orig := nowFunc
	nowFunc = func() time.Time { return parsed }
	t.Cleanup(func() { nowFunc = orig })
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	return s
}

// TestLockWritesTheLedgerRecord is the primary write hook: a successful lock
// puts the approved content, the time, the actor and the human's own words on
// the record. Without this, a legitimately locked claim is indistinguishable
// from a hand-flipped one and the gate would refuse it.
func TestLockWritesTheLedgerRecord(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes
	freezeClock(t, "2026-07-27T10:00:00Z")

	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "the approved body"}
	store := newStore(t)

	locked, err := Lock(claim, []model.Claim{claim}, testConfig(), store, Approval{Actor: "alice", Reason: "yes, ship it"})
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}

	rec, ok := store.Record(claim.ID)
	if !ok {
		t.Fatalf("expected a ledger record for the locked claim")
	}
	if rec.Subject != SubjectClaim {
		t.Errorf("subject = %q, want %q", rec.Subject, SubjectClaim)
	}
	if rec.Hash != LockedClaimHash(locked) {
		t.Errorf("recorded hash does not match the locked claim's content hash")
	}
	if rec.At != "2026-07-27T10:00:00Z" {
		t.Errorf("at = %q, want the frozen clock", rec.At)
	}
	if rec.Actor != "alice" || rec.Reason != "yes, ship it" {
		t.Errorf("actor/reason = %q/%q, want the supplied approval", rec.Actor, rec.Reason)
	}
	if rec.Grandfathered {
		t.Errorf("a real lock must never be marked grandfathered")
	}
	if rec.Released() {
		t.Errorf("a fresh lock must not be released")
	}
}

// TestRefusedLockWritesNoLedgerRecord: the ledger records APPROVALS, and a
// refused lock is not one. A record written before the gates ran would say a
// human approved content that never locked.
func TestRefusedLockWritesNoLedgerRecord(t *testing.T) {
	withRegistry(t, failingLint{})

	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	store := newStore(t)

	if _, err := Lock(claim, []model.Claim{claim}, testConfig(), store, testApproval()); err == nil {
		t.Fatalf("precondition: expected the lint gate to refuse this lock")
	}
	if _, ok := store.Record(claim.ID); ok {
		t.Fatalf("a refused lock must not leave a ledger record behind")
	}
}

// TestUnlockReleasesTheRecordButKeepsIt is the "keep the record alive across
// Unlock" requirement. Deleting it would destroy the only evidence the claim
// was ever locked — which lock-ledger-orphan needs to tell an honest unlock
// from a hand-flip, and which comment-drift detection needs to survive the
// window right after an unlock, when supervision is weakest.
func TestUnlockReleasesTheRecordButKeepsIt(t *testing.T) {
	withRegistry(t)
	freezeClock(t, "2026-07-27T10:00:00Z")

	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "body"}
	store := newStore(t)
	locked, err := Lock(claim, []model.Claim{claim}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"})
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	lockedHash, _ := store.Record(claim.ID)

	freezeClock(t, "2026-07-27T11:00:00Z")
	drafted := Unlock(locked, store, Approval{Actor: "bob", Reason: "needs a fix"})
	if drafted.Status != model.StatusDraft {
		t.Fatalf("expected unlock to return a draft claim, got %q", drafted.Status)
	}

	rec, ok := store.Record(claim.ID)
	if !ok {
		t.Fatalf("unlock must RELEASE the ledger record, not delete it")
	}
	if !rec.Released() {
		t.Fatalf("expected the record marked released")
	}
	if rec.ReleasedAt != "2026-07-27T11:00:00Z" || rec.ReleasedBy != "bob" || rec.ReleasedReason != "needs a fix" {
		t.Errorf("release stamp = %q/%q/%q, want the unlock's own approval", rec.ReleasedAt, rec.ReleasedBy, rec.ReleasedReason)
	}
	if rec.Hash != lockedHash.Hash || rec.Reason != "approved" {
		t.Errorf("releasing must not overwrite what was originally approved")
	}
}

// TestRelockClearsThePriorRelease: re-locking a released claim is a NEW
// approval. Leaving the old release stamped on the record would make it read
// "this claim is allowed to be draft" while it is locked — which is precisely
// the state lock-ledger-orphan exists to catch, inverted.
func TestRelockClearsThePriorRelease(t *testing.T) {
	withRegistry(t)

	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "v1"}
	store := newStore(t)
	locked, err := Lock(claim, []model.Claim{claim}, testConfig(), store, testApproval())
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	drafted := Unlock(locked, store, testApproval())
	drafted.Body = "v2"

	relocked, err := Lock(drafted, []model.Claim{drafted}, testConfig(), store, Approval{Actor: "alice", Reason: "approved v2"})
	if err != nil {
		t.Fatalf("re-Lock: %v", err)
	}
	rec, _ := store.Record(claim.ID)
	if rec.Released() {
		t.Fatalf("re-locking must clear the prior release")
	}
	if rec.Hash != LockedClaimHash(relocked) || rec.Reason != "approved v2" {
		t.Fatalf("re-locking must record the NEW content and the NEW approval")
	}
}

// TestReleaseApprovalOnAnUnknownClaimIsHarmless: unlock is the recovery escape
// hatch and must always work, including on a claim the ledger has never heard
// of (which the gate has already reported as lock-ledger-missing).
func TestReleaseApprovalOnAnUnknownClaimIsHarmless(t *testing.T) {
	store := newStore(t)
	if ReleaseApproval(store, "widget.contract.ghost", testApproval()) {
		t.Fatalf("expected ReleaseApproval to report false for a claim with no record")
	}
	if _, ok := store.Record("widget.contract.ghost"); ok {
		t.Fatalf("ReleaseApproval must not invent a record for a claim that never had one")
	}
}

// TestLedgerRoundTripsThroughTheStoreFile pins the on-disk shape: the ledger is
// a committed, tracked artifact that CI reads, so it has to survive save/load
// exactly, and the store has to be stamped at the ledger schema version.
func TestLedgerRoundTripsThroughTheStoreFile(t *testing.T) {
	withRegistry(t)
	freezeClock(t, "2026-07-27T10:00:00Z")

	path := filepath.Join(t.TempDir(), "store.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "body"}
	locked, err := Lock(claim, []model.Claim{claim}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"})
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	RecordBuildOrderApproval(store, "widget", "artifact-signature", Approval{Actor: "alice", Reason: "order approved"})
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reload): %v", err)
	}
	if reloaded.OnDiskVersion() != ledgerSchemaVersion {
		t.Errorf("on-disk version = %d, want %d", reloaded.OnDiskVersion(), ledgerSchemaVersion)
	}
	if !reloaded.FileExists() {
		t.Errorf("expected FileExists true for a store that was just saved")
	}
	rec, ok := reloaded.Record(claim.ID)
	if !ok || rec.Hash != LockedClaimHash(locked) || rec.Reason != "approved" {
		t.Errorf("claim record did not round-trip: %+v", rec)
	}
	bo, ok := reloaded.Record(BuildOrderLedgerKey("widget"))
	if !ok || bo.Subject != SubjectBuildOrder || bo.Hash != "artifact-signature" {
		t.Errorf("build-order record did not round-trip: %+v", bo)
	}
}

// TestAdoptLedgerGrandfathersAnOlderStore is upgrade day for a project that has
// been locking claims for months: the store exists at an older schema version,
// so its locks were made by a build that had no ledger to write to. Every
// locked claim is adopted once, marked grandfathered, and announced.
func TestAdoptLedgerGrandfathersAnOlderStore(t *testing.T) {
	announced := captureAnnouncements(t)
	freezeClock(t, "2026-07-27T10:00:00Z")

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{"widget.contract.main":{"widget.contract.dep":"h"}},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}
	draft := model.Claim{ID: "widget.contract.draft", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	claims := []model.Claim{locked, draft}

	adopted := AdoptLedger(store, claims)
	if len(adopted) != 1 || adopted[0] != locked.ID {
		t.Fatalf("adopted = %v, want just the locked claim (a draft has nothing to grandfather)", adopted)
	}
	rec, ok := store.Record(locked.ID)
	if !ok {
		t.Fatalf("expected an adopted record")
	}
	if !rec.Grandfathered {
		t.Errorf("an adopted record MUST be marked grandfathered: its hash is what was on disk, not what anyone approved")
	}
	if rec.Hash != LockedClaimHash(locked) {
		t.Errorf("adoption must record the claim's content as found")
	}
	if _, ok := store.Record(draft.ID); ok {
		t.Errorf("a draft claim must not be adopted — it holds no approval to record")
	}

	notice := announced.String()
	for _, want := range []string{"grandfathered", "NOT content anyone approved", locked.ID} {
		if !strings.Contains(notice, want) {
			t.Errorf("adoption notice does not mention %q; it must be loud and honest about what it did NOT establish.\ngot:\n%s", want, notice)
		}
	}
}

// TestAdoptLedgerRefusesWhenTheStoreFileIsAbsent is the security core of
// grandfathering. "No ledger, so adopt everything" would make deleting the
// store the universal bypass: rm the file, run any command, and every tampered
// claim in the project is blessed as approved. Absence never adopts.
func TestAdoptLedgerRefusesWhenTheStoreFileIsAbsent(t *testing.T) {
	silenceAnnouncements(t)

	store := newStore(t) // path points at a file that does not exist
	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}

	if adopted := AdoptLedger(store, []model.Claim{locked}); len(adopted) != 0 {
		t.Fatalf("adopted %v from an ABSENT ledger: deleting the ledger must never re-bless a project", adopted)
	}
	if _, ok := store.Record(locked.ID); ok {
		t.Fatalf("no record may be created when the ledger file is absent")
	}

	// And the gate must say so, rather than the absence passing silently.
	findings := Audit([]model.Claim{locked}, store, nil)
	if !hasRule(findings, RuleLockLedgerAbsent) || !hasRule(findings, RuleLockLedgerMissing) {
		t.Fatalf("expected the missing ledger reported by the gate, got %+v", findings)
	}
}

// TestAdoptLedgerNeverRunsOnACurrentStore: after the upgrade, a locked claim
// WITHOUT a record is a finding, not an invitation. If adoption keyed on "the
// ledger has no entry for this claim", hand-flipping a claim to locked would
// grandfather itself on the next command.
func TestAdoptLedgerNeverRunsOnACurrentStore(t *testing.T) {
	silenceAnnouncements(t)

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"hashes":{},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write current store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	handFlipped := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}

	if adopted := AdoptLedger(store, []model.Claim{handFlipped}); len(adopted) != 0 {
		t.Fatalf("adopted %v from a CURRENT store: a hand-flipped claim must never grandfather itself", adopted)
	}
	if !hasRule(Audit([]model.Claim{handFlipped}, store, nil), RuleLockLedgerMissing) {
		t.Fatalf("expected the hand-flipped claim reported as lock-ledger-missing")
	}
}

// TestAdoptLedgerIsIdempotent: adoption is a one-time event. A second run must
// not re-stamp records (which would move their timestamps and, worse, re-adopt
// content edited since the first adoption).
func TestAdoptLedgerIsIdempotent(t *testing.T) {
	silenceAnnouncements(t)
	freezeClock(t, "2026-07-27T10:00:00Z")

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "v1"}
	claims := []model.Claim{locked}

	if adopted := AdoptLedger(store, claims); len(adopted) != 1 {
		t.Fatalf("first adoption should adopt the locked claim, got %v", adopted)
	}
	first, _ := store.Record(locked.ID)

	// Content changes, and adoption runs again: the second run must do nothing,
	// so the tampered content is NOT adopted over the record it would break.
	claims[0].Body = "v2 — edited by hand after adoption"
	if adopted := AdoptLedger(store, claims); len(adopted) != 0 {
		t.Fatalf("second adoption must be a no-op, got %v", adopted)
	}
	if again, _ := store.Record(locked.ID); again.Hash != first.Hash {
		t.Fatalf("a second adoption re-recorded the claim's content — a hand edit would launder itself through the upgrade path")
	}
}

// TestLoadStoreKeepsVersion1NestedBaselines is the schema-bump regression the
// audit called out ahead of time. The old code dropped the baselines whenever
// the on-disk version was below the CURRENT one; that read as correct while
// there was one migration, and became a silent drift hole the moment the
// version was bumped for the ledger. Dropping a v1 store's per-dependent
// baselines takes drift detection down for every existing project AND hands
// MigrateLegacyStore an empty map, which it re-arms from current content —
// adopting whatever drift already happened as the new baseline.
func TestLoadStoreKeepsVersion1NestedBaselines(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v2 (drifted since main was locked)"}
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	path := filepath.Join(t.TempDir(), "store.json")
	v1 := `{
  "version": 1,
  "hashes": { "widget.contract.main": { "widget.contract.dep": "hash-recorded-when-main-was-locked" } },
  "locked_at": { "widget.contract.main": "2026-01-01T00:00:00Z" }
}`
	if err := os.WriteFile(path, []byte(v1), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if h, ok := store.Baseline(main.ID, dep.ID); !ok || h != "hash-recorded-when-main-was-locked" {
		t.Fatalf("a version-1 store's nested baselines must survive the version-2 upgrade, got %q/%v", h, ok)
	}

	claims := []model.Claim{main, dep}
	if MigrateLegacyStore(store, claims) {
		t.Fatalf("MigrateLegacyStore must not re-arm a store that already has real baselines")
	}

	// The drift that happened before the upgrade must still be detected.
	var flagged bool
	for _, c := range DetectStale(claims, store) {
		if c.ID == main.ID {
			flagged = c.ReviewPending
		}
	}
	if !flagged {
		t.Fatalf("pre-upgrade dependency drift went undetected: the v2 upgrade silently re-baselined it")
	}
}

// TestMigrateLegacyStoreDoesNotReArmACurrentStore closes the other half of the
// same hole. A store at the per-dependent schema with an EMPTY baselines map is
// not "legacy hashes were dropped" — most sharply, it is what a claim
// hand-flipped to status: locked after the store was written looks like.
// Re-arming there would hand the hand-flip a clean dependency baseline before
// the ledger gate ever got to report it.
func TestMigrateLegacyStoreDoesNotReArmACurrentStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":2,"hashes":{},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write current store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep"}
	handFlipped := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	if MigrateLegacyStore(store, []model.Claim{handFlipped, dep}) {
		t.Fatalf("MigrateLegacyStore must not re-arm a store already at the per-dependent schema")
	}
	if _, ok := store.Baseline(handFlipped.ID, dep.ID); ok {
		t.Fatalf("a hand-flipped locked claim must not be handed a fresh dependency baseline")
	}
}

// TestPrepareStoreRunsBothMigrationsInOrder pins the single entry point every
// writing command uses. Running only one of the two — which is exactly what
// happens when a new call site copies the older idiom — leaves either drift
// detection or the ledger unarmed, and neither failure is visible at the time.
func TestPrepareStoreRunsBothMigrationsInOrder(t *testing.T) {
	silenceAnnouncements(t)

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v1"}
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}
	claims := []model.Claim{main, dep}

	path := filepath.Join(t.TempDir(), "store.json")
	// A pre-versioning (schema 0) store: flat hashes, no ledger. Both
	// migrations have work to do.
	if err := os.WriteFile(path, []byte(`{"hashes":{"widget.contract.dep":"stale-flat-hash"},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write legacy store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	changed, adopted := PrepareStore(store, claims)
	if !changed {
		t.Fatalf("expected PrepareStore to report the store changed, so the caller saves it")
	}
	if len(adopted) != 2 {
		t.Fatalf("expected both locked claims grandfathered, got %v", adopted)
	}
	if _, ok := store.Baseline(main.ID, dep.ID); !ok {
		t.Fatalf("expected the legacy per-dependent baseline re-armed alongside the ledger adoption")
	}

	// And the whole thing is a no-op on the next run.
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reload): %v", err)
	}
	if changed, adopted := PrepareStore(reloaded, claims); changed || len(adopted) != 0 {
		t.Fatalf("PrepareStore must be a no-op on an already-migrated store, got changed=%v adopted=%v", changed, adopted)
	}
}

// TestPrepareStoreDoesNotCreateAStoreForAFreshProject: a project with nothing
// locked must not have a lock store conjured into existence by a read-mostly
// command. The store file's EXISTENCE is evidence the ledger reads (an absent
// file with locked claims is the deleted-ledger case), so creating it as a side
// effect would erode the signal.
func TestPrepareStoreDoesNotCreateAStoreForAFreshProject(t *testing.T) {
	silenceAnnouncements(t)

	store := newStore(t)
	draft := model.Claim{ID: "widget.contract.draft", Facet: "contract", Module: "widget", Status: model.StatusDraft}

	if changed, adopted := PrepareStore(store, []model.Claim{draft}); changed || len(adopted) != 0 {
		t.Fatalf("expected no work for a fresh project, got changed=%v adopted=%v", changed, adopted)
	}
}

// TestDefaultActorPrefersTheExplicitOverride pins the resolution order. CI and
// any wrapper set DOSSIERX_ACTOR; the fallbacks are a convenience, and
// "unknown" is an honest answer rather than a guess, since the ledger's
// integrity rests on the hash and the reason, not on the actor.
func TestDefaultActorPrefersTheExplicitOverride(t *testing.T) {
	t.Setenv("DOSSIERX_ACTOR", "ci-runner")
	t.Setenv("USER", "someone-else")
	if got := DefaultActor(); got != "ci-runner" {
		t.Fatalf("DefaultActor() = %q, want the DOSSIERX_ACTOR override", got)
	}

	t.Setenv("DOSSIERX_ACTOR", "")
	t.Setenv("USER", "")
	t.Setenv("USERNAME", "")
	if got := DefaultActor(); got != "unknown" {
		t.Fatalf("DefaultActor() = %q, want %q when nothing in the environment says", got, "unknown")
	}
}

// hasRule reports whether findings contain the named rule.
func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

// The migration bridge for the COMMENT DIGEST store.
//
// internal/check reports a ledger-covered project with no digest store as
// comment-digest-absent, which is what stops "delete the file and the
// comment-ledger-drift finding disappears" from being free. That rule keys on
// the LOCK store's version — the digest store's absence cannot be evidence about
// itself — so the two have to cross the line together: the same PrepareStore
// that stamps a pre-ledger store must also create the digest store, adopting the
// threads the project already has. Otherwise every upgrading project with a
// comment thread fails check the moment it upgrades.
func TestPrepareStoreCreatesTheCommentDigestStoreOnUpgrade(t *testing.T) {
	silenceAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	commented := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "please clarify",
		}},
	}
	claims := []model.Claim{commented}

	PrepareStore(store, claims)

	digestPath := digest.StorePathBeside(path)
	digests, err := digest.LoadStore(digestPath)
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	if !digests.FileExists() {
		t.Fatalf("expected the upgrade to create %s, so the project is covered from the moment it becomes ledger-covered", digestPath)
	}
	recorded, ok := digests.Digest(commented.ID)
	if !ok || recorded != digest.CommentsDigest(commented) {
		t.Fatalf("expected the existing thread adopted as found, got %q (present=%v)", recorded, ok)
	}
}

// An EXISTING digest store is never touched by the upgrade. Adoption records
// whatever is on disk as the truth, so re-running it over a store that already
// has entries would re-bless a hand-edited comment block — the exact laundering
// the digest exists to prevent.
func TestPrepareStoreNeverOverwritesAnExistingDigestStore(t *testing.T) {
	silenceAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	// A digest store recorded BEFORE someone edited the thread out of the YAML.
	withThread := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "please clarify",
		}},
	}
	digests, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	digests.Record(withThread)
	if err := digests.Save(); err != nil {
		t.Fatalf("save digest store: %v", err)
	}

	tampered := withThread
	tampered.Comments = nil
	PrepareStore(store, []model.Claim{tampered})

	reloaded, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("reload digest store: %v", err)
	}
	if recorded, _ := reloaded.Digest(withThread.ID); recorded != digest.CommentsDigest(withThread) {
		t.Fatalf("the upgrade re-recorded the tampered comment block as the truth")
	}
}

// The OTHER door into ledger-covered. A fresh project never runs a migration —
// its first "claim lock" creates the lock store outright — so
// TestPrepareStoreCreatesTheCommentDigestStoreOnUpgrade's guarantee did not
// reach it, and it ended up ledger-covered with no digest store. That state is
// byte-for-byte the state of a project whose digest store was DELETED, and that
// ambiguity is what forces check's comment-digest-absent rule to demand a
// surviving thread before it will fire. Save closes it: creating the lock store
// creates the digest store in the same breath.
func TestSaveCreatesTheCommentDigestStoreForAFreshProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.FileExists() {
		t.Fatalf("precondition: a fresh project has no lock store yet")
	}

	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	digests, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	if !digests.FileExists() {
		t.Fatalf("expected the first Save to create %s, so a fresh project crosses both lines at once", digest.StorePathBeside(path))
	}
}

// ...and it is created EMPTY, never adopted. At first creation no claim has been
// through the comment engine, so a comments: block present at this instant was
// hand-written; recording it would bless it as the truth. Unknown — never
// blessed, never accused — is the same default AdoptLedger takes for an absent
// lock ledger.
func TestSaveCreatesAnEmptyDigestStoreAndAdoptsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	handWritten := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "hand-written, never through the engine",
		}},
	}
	RecordApproval(store, handWritten, Approval{Actor: "test", Reason: "fixture"})
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	digests, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	if _, ok := digests.Digest(handWritten.ID); ok {
		t.Fatalf("expected the fresh digest store to adopt nothing, but %s was recorded", handWritten.ID)
	}
}

// Save must never RESTORE a digest store that was deleted from an
// already-covered project: that would make the next lock silently undo the
// deletion and clear the finding that named it, which is the laundering the
// digest store exists to prevent. The guard is Store.fileExists, read at load
// time, so an existing lock store never triggers creation.
func TestSaveDoesNotRecreateADeletedDigestStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save (first): %v", err)
	}
	digestPath := digest.StorePathBeside(path)
	if err := os.Remove(digestPath); err != nil {
		t.Fatalf("remove digest store: %v", err)
	}

	// Re-open the way a later command does: the lock store is on disk now, so
	// this project is already covered and is NOT crossing any line.
	reopened, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reopen): %v", err)
	}
	if err := reopened.Save(); err != nil {
		t.Fatalf("Save (second): %v", err)
	}

	if _, err := os.Stat(digestPath); !os.IsNotExist(err) {
		t.Fatalf("expected the deletion to stand so check can report it, but the digest store was re-created (%v)", err)
	}
}
