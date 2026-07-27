package comments

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// depsAgainst builds the production wiring the CLI and serve use: a Deps whose
// LockStorePath points at a REAL lock store on disk, so mutate re-reads it and
// asks it the coverage questions. project.deps() supplies an in-memory store
// with no path, which cannot answer them.
func depsAgainst(t *testing.T, p *project, storePath string) *Deps {
	t.Helper()
	claims, err := loader.LoadClaims(p.claimsDir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("lock.LoadStore: %v", err)
	}
	return &Deps{Cfg: p.cfg, Claims: claims, LockStore: store, LockStorePath: storePath, FlagStore: p.flags}
}

// TestACommentOpLeavesAnUnMigratedProjectMigratable.
//
// The comment digest store is a SIBLING FILE that only exists once a project has
// been through a ledger-aware build — which is exactly why lock.LedgerDowngraded
// reads its presence as proof that a store claiming "version 1" is lying. The
// comment write path created it unconditionally, so ONE ordinary `comment add`
// on an honest, un-migrated v0.2.x project manufactured that contradiction
// against itself:
//
//	dossierx comment add …    succeeds, and creates .dossierx-comment-digest.json
//	dossierx check            lock-ledger-downgraded, from now on
//	dossierx migrate --adopt  REFUSED: "restore the lock store from version control"
//
// The project is accused of tampering, its one-time upgrade path is closed, and
// the recovery named is for a file nobody touched. That is the outage the whole
// fail-closed design exists to avoid handing an honest project on upgrade day.
//
// Without the guard in recordCommentDigest this fails at the first assertion
// (the digest store is created) and again at the last (AdoptProject refuses).
func TestACommentOpLeavesAnUnMigratedProjectMigratable(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML, "b.yaml": preLedgerLockedBYAML})
	storePath := filepath.Join(p.root, ".dossierx-lock-store.json")
	if err := os.WriteFile(storePath, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.b":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write v1 store: %v", err)
	}

	if _, _, err := depsAgainst(t, p, storePath).Add(claimA, model.CommentRoleHuman, "is this still true?"); err != nil {
		t.Fatalf("a comment op on an un-migrated project must still work: %v", err)
	}

	digestPath := digest.StorePathBeside(storePath)
	if _, err := os.Stat(digestPath); !os.IsNotExist(err) {
		t.Fatalf("the comment op created %s beside a version-1 lock store: the next `check` reads that pair as a DOWNGRADE and accuses a project that did nothing (%v)", digestPath, err)
	}

	// The comment itself is on disk — the op is not weakened, only the sibling
	// file it had no business creating.
	threads, err := depsAgainst(t, p, storePath).List(claimA, false)
	if err != nil || len(threads) != 1 {
		t.Fatalf("expected the thread to have been written; got %d thread(s), err %v", len(threads), err)
	}

	// And the one-time migration still works, which is the whole point.
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("lock.LoadStore: %v", err)
	}
	if !store.AdoptionRequired(false) {
		t.Fatalf("the project must still be offered the one-time migration")
	}
	claims, err := loader.LoadClaims(p.claimsDir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	adoption, err := lock.AdoptProject(store, claims)
	if err != nil {
		t.Fatalf("`migrate --adopt` must still work after a comment op: %v", err)
	}
	// The migration is what records the comment blocks, in one act — including
	// the thread this test just wrote.
	digests, err := digest.LoadStore(digestPath)
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	if _, known := digests.Digest(claimA); !known {
		t.Fatalf("the migration must record every claim's comment block; adopted %v", adoption.CommentDigests)
	}
}

// TestACommentWriteStaysRefusedOnADowngradedStore.
//
// mutate armed checkCommentDigest's threads-without-an-entry gate on
// lock.Store.LedgerCovered, which reads the lock store's own "version" field. So
// the same edit the gate already reports as lock-ledger-downgraded switched the
// refusal off, and the laundering path it exists to close re-opened underneath
// the finding: with the entry dropped and the block forged, an ordinary
// `comment reply` was accepted and RE-RECORDED the forged block as the truth.
//
// Without the LedgerEstablished change in mutate this fails at the Reply
// assertion (the reply is accepted) and again at the digest assertion (the
// forged block is recorded).
func TestACommentWriteStaysRefusedOnADowngradedStore(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	storePath := filepath.Join(p.root, ".dossierx-lock-store.json")
	digestPath := digest.StorePathBeside(storePath)

	// A covered project: a lock store at the ledger schema with a record in it.
	if err := os.WriteFile(storePath, []byte(`{"version":2,"hashes":{},"locked_at":{},"ledger":{"widget.contract.z":{"subject":"claim","hash":"h","at":"2026-01-01T00:00:00Z","actor":"alice","reason":"approved"}}}`), 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}

	// A human's thread, written by the engine, which records the digest with it.
	tid := ""
	var err error
	if _, tid, err = depsAgainst(t, p, storePath).Add(claimA, model.CommentRoleHuman, "this is not what we agreed"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	digests, err := digest.LoadStore(digestPath)
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	if _, known := digests.Digest(claimA); !known {
		t.Fatalf("fixture precondition: the engine's own write must have recorded the digest")
	}

	// THE ATTACK: drop the claim's key from the digest store, and downgrade the
	// lock store's version so the guard that would notice reads as disarmed. The
	// ledger key stays, so the project is still provably ledger-aware.
	digests.Forget(claimA)
	if err := digests.Save(); err != nil {
		t.Fatalf("digest Save: %v", err)
	}
	downgradeLockStore(t, storePath)

	_, _, replyErr := depsAgainst(t, p, storePath).Reply(claimA, tid, model.CommentRoleAgent, "agreed, changing it")
	if !errors.Is(replyErr, ErrCommentDigestDrift) {
		t.Fatalf("a comment write to a claim whose digest entry was removed must stay refused on a downgraded store; got %v", replyErr)
	}
	after, err := digest.LoadStore(digestPath)
	if err != nil {
		t.Fatalf("digest.LoadStore (reopen): %v", err)
	}
	if _, known := after.Digest(claimA); known {
		t.Fatalf("the refused write still re-recorded the claim's comment block, which is the launder the refusal exists to stop")
	}
}

// downgradeLockStore rewrites the store file's "version" to 1 and leaves
// everything else alone — the ledger key included, which is what still proves
// the project has been through a ledger-aware build.
func downgradeLockStore(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	out := bytesReplaceOnce(raw, []byte(`"version": 2`), []byte(`"version": 1`))
	out = bytesReplaceOnce(out, []byte(`"version":2`), []byte(`"version":1`))
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
	store, err := lock.LoadStore(path)
	if err != nil {
		t.Fatalf("lock.LoadStore: %v", err)
	}
	if !store.LedgerDowngraded(true) {
		t.Fatalf("fixture precondition: the store must read as downgraded after the edit")
	}
}

func bytesReplaceOnce(in, old, new []byte) []byte {
	idx := indexOf(in, old)
	if idx < 0 {
		return in
	}
	out := make([]byte, 0, len(in)-len(old)+len(new))
	out = append(out, in[:idx]...)
	out = append(out, new...)
	return append(out, in[idx+len(old):]...)
}

func indexOf(haystack, needle []byte) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

const preLedgerLockedBYAML = `id: widget.contract.b
facet: contract
module: widget
status: locked
build_role: schema
body: claim b
governed_by:
  type: none
  reason: fixture
`
