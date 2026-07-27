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
	changed, adopted := PrepareStore(store, []model.Claim{claim})
	if changed || len(adopted) != 0 {
		t.Fatalf("a downgraded store must migrate nothing, got changed=%v adopted=%v", changed, adopted)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := onDiskVersion(t, path); got != 0 {
		t.Fatalf("expected the refused store written back at version 0, got %v", got)
	}
}
