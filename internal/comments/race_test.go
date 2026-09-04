package comments

import (
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

// These pin the flag-orphan race fix: a mutating op must re-read the flag store
// (and lock store) FRESH inside the claims sentinel, so a `dossierx flag` that
// committed concurrently — after the op's Deps was built but serialized by the
// claims sentinel — is honored by the review_pending recompute rather than
// clobbered to false and orphaned.

func (p *project) lockStorePath() string {
	return filepath.Join(p.root, "build", "ledger", "lock-store.json")
}

func (p *project) flagStorePath() string {
	return filepath.Join(p.root, "build", "ledger", "flag-store.json")
}

// depsFresh builds a Deps wired the way the CLI and serve do it: the two review
// stores are supplied as PATHS (re-read fresh inside the claims sentinel), not
// as in-memory snapshots, so a store change committed after this Deps is built
// is still seen at recompute time.
func (p *project) depsFresh() *Deps {
	return &Deps{Cfg: p.cfg, LockStorePath: p.lockStorePath(), FlagStorePath: p.flagStorePath()}
}

// writeFlagOnDisk persists a pending flag for id to the on-disk flag store,
// simulating a `dossierx flag` that has committed its flag-store entry.
func (p *project) writeFlagOnDisk(id string) {
	p.t.Helper()
	fs, err := reaudit.LoadFlagStore(p.flagStorePath())
	if err != nil {
		p.t.Fatalf("LoadFlagStore: %v", err)
	}
	fs.Flags[id] = reaudit.PendingFlag{ClaimSays: "x", NowDoes: "y", Reason: "z", FlaggedAt: "2026-07-24T00:00:00Z"}
	if err := fs.Save(); err != nil {
		p.t.Fatalf("flag store Save: %v", err)
	}
}

// TestPending_ResolveReReadsFlagStoreFreshInsideSentinel is the flag-orphan
// race, made deterministic: a flag is committed to the on-disk flag store AFTER
// the resolve Deps is built but BEFORE the resolve runs. With the fresh re-read,
// resolving the last open thread must RETAIN review_pending (the flag stands);
// a pre-sentinel snapshot would miss the flag and wrongly clear it.
func TestPending_ResolveReReadsFlagStoreFreshInsideSentinel(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})

	_, tid, err := p.deps().Add(claimA, model.CommentRoleAgent, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !p.reload(claimA).ReviewPending {
		t.Fatalf("precondition: an open thread on a locked claim sets review_pending")
	}

	// Build the resolve Deps NOW (production-style: store paths, no snapshots).
	d := p.depsFresh()

	// A concurrent `dossierx flag` commits its flag-store entry after d was built.
	p.writeFlagOnDisk(claimA)

	c, err := d.Resolve(claimA, tid, model.CommentRoleAgent)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !c.ReviewPending {
		t.Fatalf("resolving the last thread cleared review_pending while a concurrently-committed flag stands: the flag store was read from a pre-sentinel snapshot, not re-read fresh inside the claims sentinel")
	}
	if !p.reload(claimA).ReviewPending {
		t.Fatalf("retained review_pending must persist to disk")
	}
}

// TestPending_DeleteReReadsFlagStoreFreshInsideSentinel is the same race on the
// other clearer: deleting the last open thread must likewise honor a
// concurrently-committed flag and retain review_pending.
func TestPending_DeleteReReadsFlagStoreFreshInsideSentinel(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})

	_, tid, err := p.deps().Add(claimA, model.CommentRoleAgent, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	d := p.depsFresh()
	p.writeFlagOnDisk(claimA)

	c, err := d.Delete(claimA, tid, "", model.CommentRoleAgent)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !c.ReviewPending {
		t.Fatalf("deleting the last thread cleared review_pending while a concurrently-committed flag stands: flag store not re-read fresh inside the claims sentinel")
	}
}

// TestPending_FreshReadStillClearsWhenNoFlag guards against over-correction: with
// the fresh re-read wired but NO flag on disk, resolving the last thread must
// still clear review_pending (the fresh read sees an empty flag store).
func TestPending_FreshReadStillClearsWhenNoFlag(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": lockedAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleAgent, "please look")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	c, err := p.depsFresh().Resolve(claimA, tid, model.CommentRoleAgent)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if c.ReviewPending {
		t.Fatalf("resolving the last thread with no drift and no flag must clear review_pending even with the fresh re-read wired")
	}
}
