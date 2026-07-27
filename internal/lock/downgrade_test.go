// downgrade_test.go pins the two properties that decide whether a DOWNGRADED
// lock store — one whose "version" field was edited backwards — stays visible.
//
// They were the shared enabler under two separate laundering paths: the version
// field is the trigger for both on-load migrations, and every command that wrote
// the store for any reason of its own re-stamped it, so a refusal a reviewer had
// just been shown was not reproducible by the time they looked.
package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// onDiskVersion reads the "version" field straight out of the file, which is
// the only thing that matters here: the in-memory value is what LoadStore
// decided, and the question is what the next reader will see.
func onDiskVersion(t *testing.T, path string) float64 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	v, ok := doc["version"].(float64)
	if !ok {
		t.Fatalf("store has no numeric version field: %v", doc)
	}
	return v
}

// A store carrying LEDGER RECORDS has provably never been a schema-0 store: the
// ledger key did not exist before schema 2. Setting "version": 0 on one and
// letting MigrateLegacyStore re-arm every dependency baseline from CURRENT
// content is a complete drift launder in one edited number — LoadStore drops the
// hashes for version < 1, which defeats the "already has baselines" guard, and
// the diskVersion guard is defeated by the same edit.
//
// Reproduced end to end before the fix: a sanctioned dependency change correctly
// flipped a dependent to review_pending; deleting that line by hand (the content
// hash cannot see it) and setting the version to 0 made plain `dossierx check`
// exit 0 and re-baseline the dependency to the DRIFTED content, after which
// `check --validate` was green and review_pending never returned.
func TestMigrateLegacyStoreRefusesADowngradedStoreCarryingLedgerRecords(t *testing.T) {
	silenceAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")

	dep := model.Claim{ID: "widget.contract.overview", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "the drifted body"}
	dependent := model.Claim{ID: "widget.internals.fields", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	// A store that says schema 0 (so both existing guards are satisfied: no
	// hashes, diskVersion 0) while carrying a ledger record, which no genuine
	// schema-0 store can.
	downgraded := `{
  "version": 0,
  "hashes": {},
  "locked_at": {},
  "ledger": {
    "widget.internals.fields": {
      "subject": "claim",
      "hash": "whatever-was-approved",
      "at": "2026-07-20T09:00:00Z",
      "actor": "human",
      "reason": "approved"
    }
  }
}`
	if err := os.WriteFile(path, []byte(downgraded), 0o644); err != nil {
		t.Fatalf("write downgraded store: %v", err)
	}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if changed := MigrateLegacyStore(store, []model.Claim{dep, dependent}); changed {
		t.Fatalf("a store carrying ledger records must never be re-baselined as a legacy one")
	}
	if len(store.Hashes) != 0 {
		t.Fatalf("expected the baselines left untouched, got %v", store.Hashes)
	}
	if store.Rebaselined() != nil {
		t.Fatalf("expected no re-baselined ids, got %v", store.Rebaselined())
	}
}

// The same refusal on the OTHER piece of evidence: a comment digest store beside
// the lock store. It is a file this build creates the moment a project becomes
// ledger-covered, so a genuine v0.2.x project has never had one.
func TestMigrateLegacyStoreRefusesADowngradeBesideADigestStore(t *testing.T) {
	silenceAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if err := os.WriteFile(path, []byte(`{"version":0,"hashes":{},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write downgraded store: %v", err)
	}
	digests, err := digest.LoadStore(digest.StorePathBeside(path))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	if err := digests.Save(); err != nil {
		t.Fatalf("save digest store: %v", err)
	}

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "v2"}
	dependent := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if changed := MigrateLegacyStore(store, []model.Claim{dep, dependent}); changed {
		t.Fatalf("a store beside a comment digest store has been through a ledger-aware build; it must not re-baseline")
	}
}

// An HONEST legacy store still migrates — the guard above must not cost the
// upgrade path it was added next to — and it now announces which claims it
// re-baselined, because re-arming an integrity baseline from current content is
// not a silent bookkeeping step.
func TestMigrateLegacyStoreStillReArmsAnHonestLegacyStoreAndSaysSo(t *testing.T) {
	buf := captureAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	if err := os.WriteFile(path, []byte(`{"hashes":{"widget.contract.dep":"legacy"},"locked_at":{}}`), 0o644); err != nil {
		t.Fatalf("write legacy store: %v", err)
	}

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "v2"}
	dependent := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if changed := MigrateLegacyStore(store, []model.Claim{dep, dependent}); !changed {
		t.Fatalf("an honest legacy store must still be re-armed")
	}
	if got := store.Rebaselined(); len(got) != 1 || got[0] != dependent.ID {
		t.Fatalf("expected the re-baselined claim reported, got %v", got)
	}
	if !strings.Contains(buf.String(), dependent.ID) {
		t.Fatalf("the re-arm must announce which claims it re-baselined, got %q", buf.String())
	}
}

// THE RE-STAMP. Version used to be a write-time constant, so ANY command that
// saved the store for its own reasons — `claim unlock` on an unrelated claim was
// the reproduction — wrote the current version over a downgraded one. The
// lock-ledger-downgraded refusal held for exactly one run: `check --validate`
// reported it, the next ordinary write erased the evidence, and a reviewer
// running check after the report saw a clean project.
//
// Version is now the version the store has EARNED. A store whose adoption is
// REFUSED keeps its downgraded version on disk, and the finding stays
// reproducible until a human restores the file.
func TestADowngradedVersionSurvivesAnOrdinaryWrite(t *testing.T) {
	silenceAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")

	// A pristine, ledger-covered project with one downgraded number and nothing
	// else touched.
	downgraded := `{
  "version": 1,
  "hashes": {},
  "locked_at": {},
  "ledger": {
    "widget.contract.main": {
      "subject": "claim",
      "hash": "approved-hash",
      "at": "2026-07-20T09:00:00Z",
      "actor": "human",
      "reason": "approved"
    }
  }
}`
	if err := os.WriteFile(path, []byte(downgraded), 0o644); err != nil {
		t.Fatalf("write downgraded store: %v", err)
	}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if !store.LedgerDowngraded(digestStorePresentBeside(path)) {
		t.Fatalf("fixture precondition: this store must read as downgraded")
	}

	// The ordinary write: an unrelated `claim unlock`, which releases a record
	// and Saves. Nothing about this command has any opinion about the version.
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked}
	Unlock(claim, store, Approval{Actor: "human", Reason: "unrelated work"})
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got := onDiskVersion(t, path); got != 1 {
		t.Fatalf("an ordinary write re-stamped the version to %v; the downgrade must survive so the refusal is reproducible", got)
	}

	// And the gate still says so on the next run.
	reopened, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reopen): %v", err)
	}
	if !reopened.LedgerDowngraded(digestStorePresentBeside(path)) {
		t.Fatalf("the downgrade must still be reported after an ordinary write")
	}
}

// The composition of the two fixes: a downgraded SCHEMA-0 store must have BOTH
// migrations bail and therefore write back version 0. If either one ran, it
// would stamp the current version and take the evidence with it.
func TestADowngradedSchemaZeroStoreWritesBackVersionZero(t *testing.T) {
	silenceAnnouncements(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	downgraded := `{
  "version": 0,
  "hashes": {},
  "locked_at": {},
  "ledger": {
    "widget.contract.main": {
      "subject": "claim",
      "hash": "approved-hash",
      "at": "2026-07-20T09:00:00Z",
      "actor": "human",
      "reason": "approved"
    }
  }
}`
	if err := os.WriteFile(path, []byte(downgraded), 0o644); err != nil {
		t.Fatalf("write downgraded store: %v", err)
	}

	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	// The record count BEFORE: this fixture's store legitimately carries one
	// already (that is what makes it a downgrade rather than a pre-ledger
	// store), so the assertion is that an ordinary command does not ADD to it.
	recordsBefore := len(store.Ledger)
	changed := PrepareStore(store, []model.Claim{claim})
	if changed {
		t.Fatalf("a downgraded store must migrate nothing, got changed=%v", changed)
	}
	if len(store.Ledger) != recordsBefore {
		t.Fatalf("an ordinary command adopted into a downgraded store: %d record(s) before, %d after", recordsBefore, len(store.Ledger))
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := onDiskVersion(t, path); got != 0 {
		t.Fatalf("expected the refused store written back at version 0, got %v", got)
	}
}

// THE EMPTIED LEDGER. The downgrade evidence used to be "the ledger map is
// non-empty", which reads the RECORDS and not the KEY — so emptying the map in
// the same edit that lowers the version satisfied both existing guards:
//
//	"version": 2 -> 1, "ledger": { … } -> {}   and the store looks pre-ledger
//
// The ledger key did not exist before schema 2, so its PRESENCE is the same
// proof its contents were: a store that has never been through a ledger-aware
// build cannot carry the key at all, empty or not. Reading the key rather than
// its size costs nothing and closes the cheaper of the two edits — one that also
// leaves a smaller diff than deleting the key outright.
func TestAnEmptiedLedgerKeyIsStillEvidenceOfADowngrade(t *testing.T) {
	silenceAnnouncements(t)

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{},"ledger":{}}`), 0o644); err != nil {
		t.Fatalf("write downgraded store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	// No digest store beside it (the attacker deletes that too), so the ledger
	// key is the only evidence left — which is exactly what is being asserted.
	if !store.LedgerDowngraded(digestStorePresentBeside(path)) {
		t.Fatalf("a store carrying the ledger key cannot predate the ledger")
	}

	tampered := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "quietly rewritten"}
	if _, err := AdoptProject(store, []model.Claim{tampered}); !errors.Is(err, ErrAdoptionRefused) {
		t.Fatalf("an emptied ledger must not re-arm grandfathering, even through the explicit migration: err = %v", err)
	}
	if !hasRule(Audit([]model.Claim{tampered}, store, nil), RuleLockLedgerDowngraded) {
		t.Fatalf("expected %s", RuleLockLedgerDowngraded)
	}
	// The claims under it must still be reported as unrecorded: the downgrade is
	// the cause, "no standing approval" is what it cost, and a reader needs both.
	if !hasRule(Audit([]model.Claim{tampered}, store, nil), RuleLockLedgerMissing) {
		t.Fatalf("expected %s alongside the downgrade", RuleLockLedgerMissing)
	}
}

// The honest side of the same read: a genuine v0.2.x store has NO ledger key at
// all, and must still be grandfathered. This is the assertion that keeps the
// guard above from being a different outage — an upgrading project that could
// not adopt would fail check with lock-ledger-missing on every locked claim it
// had done nothing wrong to earn.
func TestAStoreWithNoLedgerKeyStillGrandfathers(t *testing.T) {
	silenceAnnouncements(t)

	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.main":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.LedgerDowngraded(digestStorePresentBeside(path)) {
		t.Fatalf("a genuine pre-ledger store must not read as downgraded")
	}
	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}
	adoption, err := AdoptProject(store, []model.Claim{locked})
	if err != nil || len(adoption.Claims) != 1 {
		t.Fatalf("a genuine pre-ledger project must still be adoptable by the explicit migration, got (%v, %v)", adoption.Claims, err)
	}
}
