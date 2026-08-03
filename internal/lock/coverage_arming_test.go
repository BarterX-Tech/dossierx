package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// downgradeStoreVersion rewrites the store file's "version" to 1, leaving
// everything else — the ledger key and its records included — exactly as it was.
// That is the cheaper half of the edit LedgerDowngraded is built to detect: the
// store now CLAIMS to predate the ledger while still carrying one.
func downgradeStoreVersion(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var onDisk map[string]json.RawMessage
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	onDisk["version"] = json.RawMessage(`1`)
	out, err := json.Marshal(onDisk)
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
}

// Every comment-digest guard used to be armed by LedgerCovered, which reads the
// lock store's own "version" field — so the same hand edit the gate already
// reports as lock-ledger-downgraded ALSO switched the guards off, and the
// laundering path they exist to close re-opened underneath the finding. The
// downgrade is reported once, project-scoped; the refusals have to hold
// everywhere. See Store.LedgerEstablished.
//
// Without that change this fails at the Lock assertion: commentDigestUnrecorded
// asks LedgerCovered, gets false from the edited version field, and the lock
// succeeds — writing an entry that certifies the forged comment block.
func TestTheDigestGatesStayArmedOnADowngradedStore(t *testing.T) {
	withRegistry(t)

	storePath := filepath.Join(t.TempDir(), "store.json")
	store, err := LoadStore(storePath)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	// A real lock takes the project across into ledger coverage and creates the
	// comment digest store beside the lock store.
	other := model.Claim{ID: "widget.contract.other", Facet: "contract", Module: "widget", Body: "unrelated"}
	if _, err := Lock(other, []model.Claim{other}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
		t.Fatalf("seed Lock: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	downgradeStoreVersion(t, storePath)
	store, err = LoadStore(storePath)
	if err != nil {
		t.Fatalf("LoadStore (reopen): %v", err)
	}
	if store.LedgerCovered() {
		t.Fatalf("fixture precondition: the downgraded store must no longer read as covered")
	}
	if !store.LedgerDowngraded(true) {
		t.Fatalf("fixture precondition: the store must read as downgraded")
	}
	if !store.LedgerEstablished(true) {
		t.Fatalf("a downgraded store has provably been through a ledger-aware build")
	}

	// The claim under attack: a human's objection forged to resolved, with its
	// key dropped from the digest store so nothing compares the block.
	forged := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body",
		Comments: []model.Comment{{
			ID: "c-aaa111", Status: model.CommentStatusResolved, Author: model.CommentRoleHuman,
			Created: "2026-07-27T10:00:00Z", Body: "this is wrong, please fix",
		}},
	}

	if !store.CommentDigestUnrecorded(forged) {
		t.Fatalf("the deleted-digest-key state must still be recognised on a downgraded store")
	}
	_, lockErr := Lock(forged, []model.Claim{forged}, testConfig(), store, Approval{Actor: "mallory", Reason: "the thread reads resolved"})
	if !errors.Is(lockErr, ErrCommentDigestUnrecorded) {
		t.Fatalf("locking must stay refused on a downgraded store; got %v", lockErr)
	}
	after, err := digest.LoadStore(digest.StorePathBeside(storePath))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	if _, known := after.Digest(forged.ID); known {
		t.Fatalf("the refused lock still recorded a digest for the forged comment block")
	}
}

// The other side of the same predicate: an HONEST un-migrated v0.2.x project is
// pre-ledger and NOT downgraded, so nothing above arms against it. It must keep
// locking exactly as before — the migration is what it owes, and that refusal is
// raised elsewhere with its own recovery.
func TestLedgerEstablishedIsSilentOnAnHonestPreLedgerProject(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(storePath, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.old":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}
	store, err := LoadStore(storePath)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.LedgerEstablished(false) {
		t.Fatalf("a genuine v0.2.x project has never been through a ledger-aware build; treating it as one would refuse every comment op in it")
	}
	if !store.PreLedgerUnadopted(false) {
		t.Fatalf("fixture precondition: the honest pre-ledger project must still be offered the crossing")
	}
}
