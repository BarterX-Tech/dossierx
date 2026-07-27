package lock

import (
	"bytes"
	"encoding/json"
	"errors"
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

	adoption, err := AdoptProject(store, claims)
	if err != nil {
		t.Fatalf("AdoptProject: %v", err)
	}
	adopted := adoption.Claims
	if len(adopted) != 1 || adopted[0] != locked.ID {
		t.Fatalf("adopted = %v, want just the locked claim (a draft has nothing to grandfather)", adopted)
	}
	// The migration writes the comment digest store in the same act, so the
	// project crosses BOTH lines at once: a lock store at the ledger schema with
	// no digest store beside it is the shape check reports as
	// comment-digest-absent, and a migration that produced it would hand the
	// human a finding with a version-control recovery for a file that never
	// existed.
	if _, err := os.Stat(digest.StorePathBeside(path)); err != nil {
		t.Fatalf("adoption must write the comment digest store beside the ledger: %v", err)
	}
	if len(adoption.CommentDigests) != 2 {
		t.Errorf("adoption must record a digest for EVERY claim (including drafts, whose empty digest is what makes a later hand-added thread detectable), got %v", adoption.CommentDigests)
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

	adoption, err := AdoptProject(store, []model.Claim{locked})
	if !errors.Is(err, ErrAdoptionRefused) {
		t.Fatalf("AdoptProject on an ABSENT ledger = (%v, %v), want ErrAdoptionRefused: deleting the ledger must never re-bless a project, and the migration command must not become the deletion's second half", adoption, err)
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

	if _, err := AdoptProject(store, []model.Claim{handFlipped}); !errors.Is(err, ErrAdoptionNotRequired) {
		t.Fatalf("AdoptProject on a CURRENT store = %v, want ErrAdoptionNotRequired: a hand-flipped claim must never grandfather itself, and re-running the migration must not be a way to re-adopt a covered project", err)
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

	adoption, err := AdoptProject(store, claims)
	if err != nil || len(adoption.Claims) != 1 {
		t.Fatalf("first adoption should adopt the locked claim, got (%v, %v)", adoption.Claims, err)
	}
	first, _ := store.Record(locked.ID)

	// Content changes, and the migration is run again: the second run must
	// REFUSE, so the tampered content is NOT adopted over the record it would
	// break. The store is at the ledger schema now — in memory and on disk, since
	// AdoptProject saves — so a second migration is exactly the "re-adopt a
	// covered project" the refusal exists for.
	claims[0].Body = "v2 — edited by hand after adoption"
	if _, err := AdoptProject(store, claims); !errors.Is(err, ErrAdoptionNotRequired) {
		t.Fatalf("second adoption must be refused, got %v", err)
	}
	if again, _ := store.Record(locked.ID); again.Hash != first.Hash {
		t.Fatalf("a second adoption re-recorded the claim's content — a hand edit would launder itself through the upgrade path")
	}
}

// writeDowngradedStore rewrites the store file at path the way a hand editor
// would: the schema version put back to a pre-ledger number and the whole ledger
// key removed. Everything else is left exactly as the engine wrote it, because
// that is the point — the attack is one number and one deletion inside the file
// the gate audits.
func writeDowngradedStore(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	if _, ok := doc["ledger"]; !ok {
		t.Fatalf("precondition: the store being downgraded should have a ledger to delete; got %s", raw)
	}
	doc["version"] = nestedHashSchemaVersion
	delete(doc, "ledger")
	edited, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal downgraded store: %v", err)
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("write downgraded store: %v", err)
	}
}

// TestDowngradingTheStoreCannotReArmGrandfathering is the whole attack, end to
// end, in the order it was reproduced against a real binary:
//
//	lock a claim                        -> record written, gate silent
//	edit the locked claim's body        -> lock-content-drift, correctly
//	"version": 2 -> 1, delete "ledger"  -> grandfathering fires AGAIN and adopts
//	                                       the EDITED body as approved; ok:true
//
// The gate's trigger for adoption lived in the file the gate audits, so one
// number in a text editor re-armed the entire approval path and every finding
// against the project disappeared. An audit that can be switched off from inside
// the thing being audited is worse than no audit, because it reports a pass.
//
// Both halves are asserted here, because closing only one leaves the hole open:
// the WRITE path must adopt nothing, and the READ-ONLY path (which grandfathers
// in memory for honest pre-ledger projects — see PreLedgerExempt) must not
// extend that exemption to a store whose pre-ledger claim is contradicted.
func TestDowngradingTheStoreCannotReArmGrandfathering(t *testing.T) {
	withRegistry(t) // empty registry: the lint gate always passes
	announced := captureAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "the approved body"}
	locked, err := Lock(claim, []model.Claim{claim}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"})
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	// The first save is what makes this project ledger-covered, and it creates
	// the comment digest store beside the lock store at the same instant — the
	// sibling evidence a downgrade cannot forge away without deleting a second
	// tracked file.
	if _, statErr := os.Stat(digest.StorePathBeside(path)); statErr != nil {
		t.Fatalf("precondition: the first lock must create the comment digest store: %v", statErr)
	}

	tampered := locked
	tampered.Body = "quietly rewritten after approval"
	claims := []model.Claim{tampered}

	writeDowngradedStore(t, path)

	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if reloaded.OnDiskVersion() >= ledgerSchemaVersion || len(reloaded.Ledger) != 0 {
		t.Fatalf("precondition: the downgraded store should read as pre-ledger with no records; got version %d, %d record(s)", reloaded.OnDiskVersion(), len(reloaded.Ledger))
	}

	// The WRITE path, which is now two paths. An ordinary command adopts
	// NOTHING at all any more, whatever the store's version says...
	changed := PrepareStore(reloaded, claims)
	// PrepareStore cannot report an adoption any more — it has no such return —
	// so the assertion is against the STORE, which is the thing an adoption
	// would have to change.
	if len(reloaded.Ledger) != 0 {
		t.Fatalf("an ordinary command wrote ledger records %+v: nothing is grandfathered implicitly any more", reloaded.Ledger)
	}
	if _, ok := reloaded.Record(tampered.ID); ok {
		t.Fatalf("a downgraded store must not gain a record for the tampered claim")
	}
	if changed {
		t.Errorf("a refused adoption must not rewrite the store: stamping the current version over the downgrade destroys the evidence that an edit happened")
	}

	// ...and the EXPLICIT migration, the one path that does adopt, refuses this
	// store and says so as loudly as an adoption would.
	if _, err := AdoptProject(reloaded, claims); !errors.Is(err, ErrAdoptionRefused) {
		t.Fatalf("the migration must refuse a downgraded store, got %v", err)
	}
	if _, ok := reloaded.Record(tampered.ID); ok {
		t.Fatalf("a refused migration must not record the tampered claim")
	}
	if notice := announced.String(); !strings.Contains(notice, "predates the lock ledger") {
		t.Errorf("the refusal must be announced as loudly as an adoption; got:\n%s", notice)
	}

	// The READ-ONLY path.
	digests, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	findings := Audit(claims, reloaded, digests)
	if !hasRule(findings, RuleLockLedgerDowngraded) {
		t.Fatalf("expected %s, got %+v", RuleLockLedgerDowngraded, findings)
	}
	if !hasRule(findings, RuleLockLedgerMissing) {
		t.Fatalf("the tampered claim's lost approval must still be reported alongside the cause, got %+v", findings)
	}
}

// A store that keeps its ledger records but lowers its version is the lazier
// half of the same edit, and it needs no sibling file to detect: the ledger key
// did not exist before schema 2, so a store that predates the ledger cannot be
// carrying records.
func TestAStoreThatPredatesTheLedgerCannotCarryLedgerRecords(t *testing.T) {
	silenceAnnouncements(t)

	path := filepath.Join(t.TempDir(), "store.json")
	raw := `{"version":1,"hashes":{},"locked_at":{},"ledger":{"widget.contract.other":{"subject":"claim","hash":"stale","at":"2026-01-01T00:00:00Z","actor":"alice","reason":"approved"}}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}

	if _, err := AdoptProject(store, []model.Claim{locked}); !errors.Is(err, ErrAdoptionRefused) {
		t.Fatalf("AdoptProject = %v, want ErrAdoptionRefused: a store carrying ledger records has been through a ledger-aware build, whatever its version says", err)
	}
	// No digest store exists in this temp dir, so the ledger map is the only
	// evidence — which is exactly what is being asserted.
	if !hasRule(Audit([]model.Claim{locked}, store, nil), RuleLockLedgerDowngraded) {
		t.Fatalf("expected %s", RuleLockLedgerDowngraded)
	}
}

// The honest side of the same predicate: a genuine v0.2.x project (an older
// store, no comment digest store, no ledger records) still grandfathers exactly
// as it always did. This is the assertion that makes the two above safe to
// ship — a guard that also refused honest upgrades would just be a different
// outage.
func TestAdoptLedgerStillGrandfathersAGenuinePreLedgerProject(t *testing.T) {
	silenceAnnouncements(t)
	freezeClock(t, "2026-07-27T10:00:00Z")

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.main":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}

	adoption, err := AdoptProject(store, []model.Claim{locked})
	if err != nil || len(adoption.Claims) != 1 {
		t.Fatalf("a genuine pre-ledger project must still be adoptable by the explicit migration, got (%v, %v)", adoption.Claims, err)
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

// TestPrepareStoreMigratesBaselinesButNeverAdopts is the write-path half of
// ADOPTION FAILS CLOSED, and it is the inversion of what this test used to
// assert. PrepareStore ran the ledger grandfathering as its second migration, so
// EVERY writing command — every `dossierx check`, every claim command — adopted
// any project that presented the pre-ledger shape. That made adoption something
// an attacker could trigger with two hand edits, which is what makes an adoption
// worthless as evidence.
//
// So: the baseline migration still runs (it is not an approval, and a project
// that upgrades must not lose drift detection), the ledger is untouched, and the
// project is left for the explicit migration to adopt — while the gate says so.
//
// The schema version it stamps is the one it EARNED. Stamping the current
// version here would take this schema-0 store to the LEDGER schema with no
// record in it, and every locked claim in it would read as covered-but-deleted.
func TestPrepareStoreMigratesBaselinesButNeverAdopts(t *testing.T) {
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

	changed := PrepareStore(store, claims)
	if !changed {
		t.Fatalf("expected PrepareStore to report the store changed, so the caller saves it")
	}
	if len(store.Ledger) != 0 {
		t.Fatalf("an ordinary command grandfathered %+v: adoption must only ever happen through the explicit migration", store.Ledger)
	}
	if _, ok := store.Baseline(main.ID, dep.ID); !ok {
		t.Fatalf("expected the legacy per-dependent baseline re-armed: an upgrading project must not lose drift detection while it waits to be adopted")
	}
	if store.Version >= ledgerSchemaVersion {
		t.Fatalf("the baseline migration stamped the LEDGER schema (%d) onto a store with no ledger in it: every locked claim would then read as covered-but-unrecorded, and the migration would never be offered", store.Version)
	}

	// And the gate says what is left to do, once, by name.
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reload): %v", err)
	}
	if changed := PrepareStore(reloaded, claims); changed {
		t.Fatalf("PrepareStore must be a no-op on an already-baselined store, got changed=%v", changed)
	}
	if len(reloaded.Ledger) != 0 {
		t.Fatalf("a second ordinary command adopted %+v", reloaded.Ledger)
	}
	if !hasRule(Audit(claims, reloaded, nil), RuleLockLedgerAdoptionRequired) {
		t.Fatalf("expected %s: a project nothing adopted must be told what to run, not left passing", RuleLockLedgerAdoptionRequired)
	}

	// The explicit migration is what finishes the job.
	adoption, err := AdoptProject(reloaded, claims)
	if err != nil || len(adoption.Claims) != 2 {
		t.Fatalf("expected both locked claims grandfathered by the migration, got (%v, %v)", adoption.Claims, err)
	}
	if findings := Audit(claims, reloaded, nil); hasRule(findings, RuleLockLedgerAdoptionRequired) {
		t.Fatalf("the migration must clear the refusal it names, got %+v", findings)
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

	if changed := PrepareStore(store, []model.Claim{draft}); changed {
		t.Fatalf("expected no work for a fresh project, got changed=%v", changed)
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

// The migration bridge for the COMMENT DIGEST store, now owned by the explicit
// migration — and the assertion that an ordinary command does NOT build half of
// it.
//
// internal/check reports a ledger-covered project with no digest store as
// comment-digest-absent, which is what stops "delete the file and the
// comment-ledger-drift finding disappears" from being free. That rule keys on
// the LOCK store's version — the digest store's absence cannot be evidence about
// itself — so the two have to cross the line TOGETHER, in one act.
//
// While adoption was implicit they did: PrepareStore stamped the ledger schema
// and created the digest store in the same run. With adoption explicit, an
// ordinary command that still created the digest store would leave a v0.2.x
// project carrying a digest store beside a version-1 lock store — exactly the
// contradiction LedgerDowngraded is built to detect — and the next `check` would
// report lock-ledger-downgraded against a project whose only fault was being
// un-migrated. So the sweep leaves a pre-ledger project alone, and AdoptProject
// writes both files at once.
func TestAdoptProjectCreatesTheCommentDigestStoreOnUpgrade(t *testing.T) {
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
	digestPath := digest.StorePathBeside(path)

	// An ordinary command first: it must leave the un-migrated project exactly
	// as it found it, digest store included.
	PrepareStore(store, claims)
	if _, err := os.Stat(digestPath); err == nil {
		t.Fatalf("an ordinary command created the digest store beside an un-migrated lock store: the next run would read that pair as a DOWNGRADE and accuse a project that did nothing")
	}

	if _, err := AdoptProject(store, claims); err != nil {
		t.Fatalf("AdoptProject: %v", err)
	}

	digests, err := digest.LoadStore(digestPath)
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	if !digests.FileExists() {
		t.Fatalf("expected the migration to create %s, so the project crosses both lines in one act", digestPath)
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

// ...and an APPROVAL records the approved claim's comment digest beside it, in
// the same act that records the approval.
//
// This inverts what this test used to assert (that a fresh store adopts
// NOTHING, leaving a hand-written block unknown), and the inversion is the
// point. "Unknown" sounded conservative and was not: a claim holding a standing
// approval with no digest entry is indistinguishable from a claim whose entry
// was deleted, so `{"digests":{}}` erased an unresolved human review with no
// finding at all while deleting the whole FILE was caught. Coverage that is
// established by the approval path is coverage the tampered file does not get to
// decide — see RecordApproval and internal/check's comment-digest-missing.
//
// What is given up is stated plainly: a comments block hand-written before the
// claim's first lock is adopted as-found rather than left unknown. Lock refuses a
// claim with an UNRESOLVED thread, so what can be adopted here is a resolved
// history nobody can now attest to — the same caveat grandfathering carries, in
// exchange for the emptied-map launder being reported.
func TestApprovalRecordsTheClaimsCommentDigest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	approved := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusResolved, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "reviewed and resolved before the lock",
			ResolvedBy: model.CommentRoleHuman, ResolvedAt: "2026-07-24T11:00:00Z",
		}},
	}
	RecordApproval(store, approved, Approval{Actor: "test", Reason: "fixture"})
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	digests, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	recorded, ok := digests.Digest(approved.ID)
	if !ok {
		t.Fatalf("expected the approval to record %s's comment digest, so an emptied digest map is a finding", approved.ID)
	}
	if want := digest.CommentsDigest(approved); recorded != want {
		t.Fatalf("recorded digest %q, want the approved claim's own %q", recorded, want)
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

// coveredProject builds the ordinary state every rule in this file is about: a
// lock store on disk at the ledger schema, one locked claim with a STANDING
// approval, and the comment digest store the approval wrote beside it. It
// returns the reopened store — reopened because fileExists/diskVersion are read
// at load time, and "already covered" is precisely what the second open sees.
func coveredProject(t *testing.T, c model.Claim) (model.Claim, *Store) {
	t.Helper()
	c.Status = model.StatusLocked

	path := filepath.Join(t.TempDir(), "store.json")
	seed, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	RecordApproval(seed, c, Approval{Actor: "alice", Reason: "approved"})
	if err := seed.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reopen): %v", err)
	}
	if !store.LedgerCovered() {
		t.Fatalf("fixture is not a ledger-covered project: version %d, exists %v", store.OnDiskVersion(), store.FileExists())
	}
	return c, store
}

func digestFor(t *testing.T, store *Store, id string) (string, bool) {
	t.Helper()
	digests, err := digest.LoadStore(digest.StorePathBeside(store.path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	return digests.Digest(id)
}

// TestSweepDoesNotAdoptIntoACoveredProjectWhoseDigestStoreWasDeleted is the
// laundering path plain `dossierx check` carried. The sweep called digest.Adopt
// with no guard at all, so the digest store was re-derived from whatever the
// claim files said NOW:
//
//	edit a locked claim's comments by hand  -> comment-ledger-drift, correctly
//	rm .dossierx-comment-digest.json        -> comment-digest-absent, correctly
//	dossierx check                          -> ONE ordinary run adopts the edited
//	                                           block as the truth: ok:true, both
//	                                           findings gone, permanently green
//
// A human's open objection on a LOCKED claim is erased and the commit then
// passes the hook and CI. Absence must never adopt in a covered project — the
// same rule AdoptLedger has always applied to the ledger itself (an absent lock
// store is indistinguishable from a deleted one, so `rm` must not be a bypass),
// and the same one internal/comments' recordCommentDigest applies at its own
// adoption point.
func TestSweepDoesNotAdoptIntoACoveredProjectWhoseDigestStoreWasDeleted(t *testing.T) {
	silenceAnnouncements(t)

	objection := model.Comment{
		ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
		Created: "2026-07-24T10:00:00Z", Body: "this contradicts the schema claim",
	}
	locked, store := coveredProject(t, model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget",
		Body: "approved body", Comments: []model.Comment{objection},
	})

	// The tamper: the objection edited straight out of the YAML, and the digest
	// store that would have caught it deleted in the same commit.
	tampered := locked
	tampered.Comments = nil
	digestPath := digest.StorePathBeside(store.path)
	if err := os.Remove(digestPath); err != nil {
		t.Fatalf("remove digest store: %v", err)
	}

	PrepareStore(store, []model.Claim{tampered})

	if recorded, known := digestFor(t, store, tampered.ID); known {
		t.Fatalf("the sweep adopted the tampered comment block as the truth (%q): a deleted digest store must never re-derive coverage from the claim files", recorded)
	}
	if _, err := os.Stat(digestPath); !os.IsNotExist(err) {
		t.Fatalf("expected the deletion to stand so check can report comment-digest-absent, got %v", err)
	}
}

// The same launder with the digest store left in place and ONE entry deleted —
// the variant that is cheaper to hide in a review diff than removing the file,
// and that the file-scoped guard above does not see. A claim under a STANDING
// approval had its digest recorded at the moment of that approval
// (RecordApproval), so a missing entry there is never "a claim the store has
// not met yet": it is comment-digest-missing, a finding, and adopting would
// clear it.
func TestSweepDoesNotAdoptAClaimHoldingAStandingApproval(t *testing.T) {
	silenceAnnouncements(t)

	locked, store := coveredProject(t, model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget",
		Body: "approved body",
		Comments: []model.Comment{{
			ID: "c-aaaaaa", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-24T10:00:00Z", Body: "this contradicts the schema claim",
		}},
	})

	digests, err := digest.LoadStore(digest.StorePathBeside(store.path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	digests.Forget(locked.ID)
	if err := digests.Save(); err != nil {
		t.Fatalf("save digest store: %v", err)
	}

	tampered := locked
	tampered.Comments = nil
	PrepareStore(store, []model.Claim{tampered})

	if recorded, known := digestFor(t, store, tampered.ID); known {
		t.Fatalf("the sweep adopted a claim under a standing approval (%q), clearing the finding that named the emptied entry", recorded)
	}
}

// The sweep's PURPOSE has to survive its guards, or the fix is just a different
// hole: a claim the store has genuinely never met — authored after this project
// became covered, still draft, no ledger record, NO THREADS — must still be
// adopted, and the adoption must be REPORTED so no coverage decision is silent.
//
// Adopting it at its EMPTY digest is what makes a thread hand-added to it
// afterwards report as drift, which is the whole point of covering a claim
// nobody has commented on yet.
func TestSweepStillAdoptsAnUnknownThreadlessClaimAndReportsIt(t *testing.T) {
	silenceAnnouncements(t)

	locked, store := coveredProject(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})
	fresh := model.Claim{ID: "widget.contract.new", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "written yesterday"}

	PrepareStore(store, []model.Claim{locked, fresh})

	recorded, known := digestFor(t, store, fresh.ID)
	if !known || recorded != digest.CommentsDigest(fresh) {
		t.Fatalf("a claim the digest store has never seen must still be adopted, got %q (present=%v)", recorded, known)
	}
	adopted := store.CommentDigestsAdopted()
	if len(adopted) != 1 || adopted[0] != fresh.ID {
		t.Fatalf("the adoption must be reported so a caller can warn about it, got %v", adopted)
	}
}

// THE ONE-DELETED-KEY LAUNDER, from both ends. This is the second critical the
// round-6 review reproduced: in a fully covered project with the digest store
// present and readable (so comment-digest-absent cannot fire), forge a human's
// open thread to `status: resolved` and delete that ONE claim's key from the
// "digests" map. comment-ledger-drift compares against a RECORDED digest and a
// claim with no entry is "unknown", so the finding was gone; the next ordinary
// command re-adopted the forged block; and the claim then locked, defeating the
// gate the whole review loop rests on (a human's open objection blocks the lock).
//
// Two things have to hold to close it, and this test asserts both: the sweep must
// not re-adopt a claim that CARRIES THREADS, and the gate must report the claim
// as uncovered. The threads themselves are the trigger because they are evidence
// the tamper cannot remove without giving up what it came for — comments are
// engine-managed, and the one path that writes a thread records the digest in the
// same act.
func TestASingleDeletedDigestKeyIsNeitherReAdoptedNorSilent(t *testing.T) {
	silenceAnnouncements(t)

	locked, store := coveredProject(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})
	reviewed := model.Claim{
		ID: "widget.contract.reviewed", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "under review",
		Comments: []model.Comment{{
			ID: "c-bbbbbb", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-25T10:00:00Z", Body: "I do not agree with this yet",
		}},
	}
	claims := []model.Claim{locked, reviewed}

	// The engine's own record of the OPEN thread, as a comment op would leave it.
	digests, err := digest.LoadStore(digest.StorePathBeside(store.path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	digests.Record(reviewed)
	if err := digests.Save(); err != nil {
		t.Fatalf("save digest store: %v", err)
	}

	// The forgery, and the one deleted key that used to hide it.
	forged := reviewed
	forged.Comments = []model.Comment{{
		ID: "c-bbbbbb", Status: model.CommentStatusResolved, Author: model.CommentRoleHuman,
		Created: "2026-07-25T10:00:00Z", Body: "I do not agree with this yet",
		ResolvedBy: model.CommentRoleHuman, ResolvedAt: "2026-07-26T10:00:00Z",
	}}
	claims[1] = forged

	digests.Forget(forged.ID)
	if err := digests.Save(); err != nil {
		t.Fatalf("save digest store: %v", err)
	}

	// An ordinary writing command must NOT put the forged block back on the
	// record. That re-adoption is what made the launder permanent.
	PrepareStore(store, claims)
	if recorded, known := digestFor(t, store, forged.ID); known {
		t.Fatalf("the sweep re-adopted a claim that carries threads (%q): a forged thread is blessed by an ordinary command nobody would look twice at", recorded)
	}

	// And the gate must say the claim is uncovered, rather than reading it as
	// unknown and staying silent.
	reloaded, err := digest.LoadStore(digest.StorePathBeside(store.path))
	if err != nil {
		t.Fatalf("reload digest store: %v", err)
	}
	findings := Audit(claims, store, reloaded)
	if !hasRule(findings, RuleCommentDigestUnrecorded) {
		t.Fatalf("expected %s for a claim carrying threads with no entry, got %+v", RuleCommentDigestUnrecorded, findings)
	}
	for _, f := range findings {
		if f.Rule == RuleCommentDigestUnrecorded && f.ClaimID != forged.ID {
			t.Errorf("the finding must name the claim whose entry went missing, got %q", f.ClaimID)
		}
	}
	// The claim that still HAS its entry is not accused of anything.
	if len(findings) != 1 {
		t.Fatalf("only the uncovered claim may be reported, got %+v", findings)
	}
}

// TestLockRefusesUntilTheProjectIsAdopted is the WRITE-path half of "adoption
// fails closed", and it is the guard that keeps the store's version field
// honest.
//
// A pre-ledger store must not acquire its first ledger record from an ordinary
// lock. If it did, the file would carry ledger records at a pre-ledger version —
// which Store.LedgerDowngraded reads, correctly by its own rules, as a hand
// edit — and the project would be accused of tampering from then on for having
// done nothing but lock a claim. Stamping the schema instead would be worse: the
// store would become covered while every other locked claim in it still had no
// record, turning an honest upgrade into N lock-ledger-deleted findings.
//
// The refusal costs one command, and the test's second half is the promise that
// matters more than the refusal: unlock -> fix -> lock, the sanctioned path
// everything else points at, works normally the moment the migration has run.
func TestLockRefusesUntilTheProjectIsAdopted(t *testing.T) {
	withRegistry(t)
	silenceAnnouncements(t)

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.old":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write v0.2.x store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	old := model.Claim{ID: "widget.contract.old", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "locked months ago"}
	fresh := model.Claim{ID: "widget.contract.new", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "written today"}
	claims := []model.Claim{old, fresh}

	if _, err := Lock(fresh, claims, testConfig(), store, testApproval()); !errors.Is(err, ErrAdoptionRequired) {
		t.Fatalf("Lock on an un-migrated project = %v, want ErrAdoptionRequired", err)
	}
	if len(store.Ledger) != 0 {
		t.Fatalf("a refused lock must leave the store exactly as it found it, got %+v", store.Ledger)
	}

	// The one command that clears it.
	if _, err := AdoptProject(store, claims); err != nil {
		t.Fatalf("AdoptProject: %v", err)
	}

	locked, err := Lock(fresh, claims, testConfig(), store, Approval{Actor: "alice", Reason: "approved"})
	if err != nil {
		t.Fatalf("after the migration, locking must work normally: %v", err)
	}

	// unlock -> fix -> lock, unchanged.
	drafted := Unlock(locked, store, Approval{Actor: "alice", Reason: "needs a fix"})
	drafted.Body = "fixed"
	if _, err := Lock(drafted, []model.Claim{old, drafted}, testConfig(), store, Approval{Actor: "alice", Reason: "approved the fix"}); err != nil {
		t.Fatalf("unlock -> fix -> lock must never be blocked: %v", err)
	}
}

// UNLOCK IS NEVER GATED, including before the migration. It is the recovery
// escape hatch every other refusal in this package points at, and a project that
// could not get a claim out of locked would have no way to fix the thing it is
// being refused for.
func TestUnlockStillWorksOnAnUnmigratedProject(t *testing.T) {
	silenceAnnouncements(t)

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.old":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write v0.2.x store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	old := model.Claim{ID: "widget.contract.old", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "locked months ago"}

	if drafted := Unlock(old, store, Approval{Actor: "alice", Reason: "needs a fix"}); drafted.Status != model.StatusDraft {
		t.Fatalf("unlock must always work, got status %q", drafted.Status)
	}
}
