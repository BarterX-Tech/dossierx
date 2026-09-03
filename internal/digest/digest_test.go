package digest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func claimWithThread() model.Claim {
	return model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked,
		Comments: []model.Comment{{
			ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-27T10:00:00Z", Body: "is this still true?",
			Replies: []model.Reply{{ID: "r-abc123", Author: model.CommentRoleAgent, Created: "2026-07-27T10:05:00Z", Body: "checking now"}},
		}},
	}
}

func loadTempStore(t *testing.T) (store *Store, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "digest.json")
	s, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	return s, path
}

// TestCommentsDigestSeesEveryPartOfAThread walks the ways a comment block can
// be edited out of band. Every one of them has to move the digest: a review
// history that can be quietly rewritten is not a review history.
func TestCommentsDigestSeesEveryPartOfAThread(t *testing.T) {
	base := claimWithThread()
	baseDigest := CommentsDigest(base)

	edits := map[string]func(*model.Claim){
		"thread deleted":        func(c *model.Claim) { c.Comments = nil },
		"thread body rewritten": func(c *model.Claim) { c.Comments[0].Body = "actually this is fine" },
		"thread resolved":       func(c *model.Claim) { c.Comments[0].Status = model.CommentStatusResolved },
		"author swapped":        func(c *model.Claim) { c.Comments[0].Author = model.CommentRoleAgent },
		"created backdated":     func(c *model.Claim) { c.Comments[0].Created = "2020-01-01T00:00:00Z" },
		"edited flag cleared":   func(c *model.Claim) { c.Comments[0].Edited = true },
		"resolver forged":       func(c *model.Claim) { c.Comments[0].ResolvedBy = model.CommentRoleHuman },
		"reply deleted":         func(c *model.Claim) { c.Comments[0].Replies = nil },
		"reply rewritten":       func(c *model.Claim) { c.Comments[0].Replies[0].Body = "all good" },
		"thread added": func(c *model.Claim) {
			c.Comments = append(c.Comments, model.Comment{ID: "c-def456", Status: model.CommentStatusResolved, Author: model.CommentRoleAgent, Created: "2026-07-27T11:00:00Z", Body: "second"})
		},
	}

	for name, edit := range edits {
		mutated := claimWithThread()
		edit(&mutated)
		if CommentsDigest(mutated) == baseDigest {
			t.Errorf("%s left the comment digest unchanged", name)
		}
	}
}

// TestCommentsDigestSeesThreadOrder: the threads are a list, and reordering
// them is an edit a reviewer reading the diff would see, so the digest sees it
// too.
func TestCommentsDigestSeesThreadOrder(t *testing.T) {
	first := model.Comment{ID: "c-aaa111", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-27T10:00:00Z", Body: "one"}
	second := model.Comment{ID: "c-bbb222", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-27T11:00:00Z", Body: "two"}

	ab := model.Claim{ID: "widget.contract.main", Comments: []model.Comment{first, second}}
	ba := model.Claim{ID: "widget.contract.main", Comments: []model.Comment{second, first}}

	if CommentsDigest(ab) == CommentsDigest(ba) {
		t.Fatalf("reordering threads must change the digest")
	}
}

// TestCommentsDigestResistsBodySmuggling proves the length-prefixed body
// encoding does its job. Without it, a body containing the field separator
// could be crafted to reproduce another thread's byte layout — letting an edit
// hash to what was recorded.
func TestCommentsDigestResistsBodySmuggling(t *testing.T) {
	honest := model.Claim{ID: "widget.contract.main", Comments: []model.Comment{
		{ID: "c-aaa111", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-27T10:00:00Z", Body: "one"},
	}}
	forged := model.Claim{ID: "widget.contract.main", Comments: []model.Comment{
		{ID: "c-aaa111", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-27T10:00:00Z",
			Body: "one\nr:c-aaa111|human|2026-07-27T10:00:00Z|false"},
	}}

	if CommentsDigest(honest) == CommentsDigest(forged) {
		t.Fatalf("a crafted body imitated another field's encoding: the digest is ambiguous")
	}
}

// TestEmptyCommentsAreRecordedNotAbsent is why Record always writes an entry.
// If "no threads" fell back to "no entry", every claim would become uncovered
// the moment its comments were emptied — which is exactly the state someone
// deleting an unresolved thread wants to leave it in.
func TestEmptyCommentsAreRecordedNotAbsent(t *testing.T) {
	store, _ := loadTempStore(t)
	empty := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget"}
	store.Record(empty)

	recorded, ok := store.Digest(empty.ID)
	if !ok {
		t.Fatalf("a claim with no comments must still be recorded")
	}
	if recorded == "" {
		t.Fatalf("the empty comment block must hash to a real value, not the empty string")
	}

	forged := empty
	forged.Comments = []model.Comment{{ID: "c-forged", Status: model.CommentStatusResolved, Author: model.CommentRoleAgent, Created: "2026-07-27T11:00:00Z", Body: "looks fine"}}
	if CommentsDigest(forged) == recorded {
		t.Fatalf("a hand-added thread must not match the recorded empty digest")
	}
}

// TestStoreRoundTripsThroughItsOwnFile pins the on-disk shape, and — the point
// of this package existing at all — that the file is the digest store's own,
// not the lock store's. Sharing the lock store would make "dossierx serve" a
// lock-store writer, since every comment write goes through internal/comments,
// and the release's guarantee that it has no such authority would be false.
func TestStoreRoundTripsThroughItsOwnFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StoreFileName)
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.FileExists() {
		t.Fatalf("a store loaded from a missing file must report FileExists false")
	}

	c := claimWithThread()
	store.Record(c)
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "build", "ledger", "lock-store.json")); !os.IsNotExist(err) {
		t.Fatalf("the comment digest must never be written into the lock store file")
	}

	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reload): %v", err)
	}
	if !reloaded.FileExists() {
		t.Fatalf("expected FileExists true after Save")
	}
	if reloaded.Version != StoreSchemaVersion {
		t.Fatalf("version = %d, want %d", reloaded.Version, StoreSchemaVersion)
	}
	if got, ok := reloaded.Digest(c.ID); !ok || got != CommentsDigest(c) {
		t.Fatalf("digest did not round-trip: %q/%v", got, ok)
	}
}

// TestAdoptCoversClaimsTheStoreHasNeverSeen: a project upgrading into this
// feature must get coverage for the threads it already has, not only for claims
// someone happens to comment on afterwards.
func TestAdoptCoversClaimsTheStoreHasNeverSeen(t *testing.T) {
	store, _ := loadTempStore(t)
	commented := claimWithThread()
	other := model.Claim{ID: "widget.contract.other", Facet: "contract", Module: "widget"}

	adopted := Adopt(store, []model.Claim{commented, other})
	if len(adopted) != 2 {
		t.Fatalf("adopted = %v, want both claims", adopted)
	}
	if got, ok := store.Digest(commented.ID); !ok || got != CommentsDigest(commented) {
		t.Fatalf("adoption must record the claim's comments as found")
	}
}

// TestAdoptNeverOverwritesAnExistingDigest: adoption fills gaps, it does not
// re-bless. Overwriting would turn any command that touches the store into a
// laundering path for a comment block edited since the last real write.
func TestAdoptNeverOverwritesAnExistingDigest(t *testing.T) {
	store, _ := loadTempStore(t)
	c := claimWithThread()
	store.Record(c)
	before, _ := store.Digest(c.ID)

	tampered := c
	tampered.Comments = nil
	if adopted := Adopt(store, []model.Claim{tampered}); len(adopted) != 0 {
		t.Fatalf("adopted %v over an existing digest", adopted)
	}
	if after, _ := store.Digest(c.ID); after != before {
		t.Fatalf("adoption overwrote a recorded digest with tampered content")
	}
}

// TestStorePathIsResolvedAgainstTheConfigDir, not the process cwd — the same
// convention claims_dir, the catalog and the lock store follow — and outside
// claims_dir, so the file is never itself decoded as a claim.
func TestStorePathIsResolvedAgainstTheConfigDir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := filepath.Join(dir, "build", "ledger", StoreFileName)
	if got := StorePath(cfg); got != want || got != cfg.CommentDigestPath() {
		t.Fatalf("StorePath = %q, want %q (cfg.CommentDigestPath = %q)", got, want, cfg.CommentDigestPath())
	}
	// StorePathBeside derives the digest's path from the lock store's DIRECTORY
	// (internal/lock's PrepareStore has only a *lock.Store in hand), so the two
	// must be siblings under build/ledger/ by construction. A wrong sibling
	// reads as "no digest store" — LoadStore treats a missing file as fresh —
	// which is a silent pass, so this equality is pinned by name.
	if got := StorePathBeside(cfg.LockStorePath()); got != cfg.CommentDigestPath() {
		t.Fatalf("StorePathBeside(LockStorePath) = %q, want CommentDigestPath %q", got, cfg.CommentDigestPath())
	}
}

// CheckWritable is the probe internal/comments runs BEFORE it saves a comment,
// so that an unwritable store is a clean refusal rather than "the comment was
// written and the op reported failure" — the combination a retrying agent turns
// into duplicate threads.
func TestCheckWritableDetectsAnUnwritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission bits do not gate file creation on Windows")
	}
	dir := t.TempDir()
	store, err := LoadStore(filepath.Join(dir, StoreFileName))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := store.CheckWritable(); err != nil {
		t.Fatalf("a writable directory must probe clean: %v", err)
	}

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) }) //nolint:errcheck // best-effort restore so TempDir cleanup works

	if err := store.CheckWritable(); err == nil {
		t.Fatalf("expected CheckWritable to refuse an unwritable directory")
	}
	// And the probe leaves nothing behind on the way through.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("the probe left files behind: %v", entries)
	}
}

// TestLoadStoreToleratesAnUnknownKeyAndDecodeStoreRefusesIt pins the split
// between the two entry points. LoadStore is the on-disk path every verb takes:
// a key a LATER release adds to the store must not make check on this binary
// fail with lock-ledger-unreadable and prescribe restoring a file that is not
// corrupt. DecodeStore answers "is this blob OUR store?" for the index's copy
// under check --staged, and there an unknown key is a no.
func TestLoadStoreToleratesAnUnknownKeyAndDecodeStoreRefusesIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StoreFileName)
	raw := []byte(`{"version":1,"digests":{"widget.contract.main":"sha256:abc"},"future_key":"x"}`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore must tolerate an unknown key, as every release before did: %v", err)
	}
	if got, ok := store.Digest("widget.contract.main"); !ok || got != "sha256:abc" {
		t.Fatalf("the known keys must still load beside the unknown one: %q/%v", got, ok)
	}
	if _, err := DecodeStore(raw); err == nil {
		t.Fatalf("DecodeStore must refuse an unknown key")
	}
	if _, err := DecodeStore([]byte(`{"version":1,"digests":{}}`)); err != nil {
		t.Fatalf("DecodeStore must accept the exact on-disk shape: %v", err)
	}
}
