// Package digest maintains the COMMENT DIGEST STORE: a per-claim hash of a
// claim's engine-managed comment threads, refreshed on every legitimate comment
// operation, so that a comment block edited out of band — most sharply, a
// thread deleted straight out of the YAML to make an unresolved review
// disappear — is detectable rather than silent.
//
// It lives in its OWN store file, deliberately NOT in internal/lock's store,
// and that is the whole reason this package exists rather than a few more
// fields on lock.Store. The release makes a hard guarantee that "dossierx serve"
// has no write authority over the lock store — true today, and worth keeping
// true: internal/comments only ever calls lock.LoadStore, never Save, so the
// browser-facing server can read the lock state and cannot change it. But every
// comment write DOES go through internal/comments, including the ones serve
// makes. Putting the comment digest in the lock store would therefore make
// serve a lock-store writer and falsify the guarantee the moment it shipped.
// Two files, two authorities: the lock ledger is written only by the approval
// path, the comment digest is written by the comment path.
//
// LOCK ORDERING. Every write to this store happens INSIDE the project-wide
// claims sentinel that internal/comments' mutate already holds — comments ->
// digest, never the reverse, and never digest alone. That is why this package
// exposes StorePath and leaves acquisition to the caller (which already imports
// internal/lock for exactly that): a second sentinel taken in a second order
// somewhere else is how AB-BA deadlocks get introduced, and the one-line rule
// "the digest sentinel is only ever taken inside the claims sentinel" is
// enforceable by reading the two call sites.
//
// This package imports only config and model, so internal/lock can import it
// for the ledger gate without a cycle (lock -> digest; comments -> lock,
// digest).
package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// StoreSchemaVersion is the on-disk schema version of the comment digest
// store. Version 1 is its first shipped shape.
const StoreSchemaVersion = 1

// digestVersion is the version of the DIGEST ALGORITHM below, mixed in as a
// domain separator. Bumping it invalidates every recorded digest (every claim
// would read as drifted), so a bump must come with a StoreSchemaVersion bump
// and a re-adoption — same contract as internal/lock's LockedClaimHash.
const digestVersion = 1

// storeFileName is the digest store's filename. It sits next to the lock store
// under the config file's own directory (never cwd), the same convention
// claims_dir, .catalog.json and the lock store follow, and outside claims_dir
// so it is never itself decoded as a claim.
const storeFileName = ".dossierx-comment-digest.json"

// StorePath returns the digest store's path for cfg. Callers that need to
// serialize on it acquire lock.AcquireFileLock(StorePath(cfg)) — INSIDE the
// claims sentinel; see this package's doc comment.
func StorePath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), storeFileName)
}

// Store is the on-disk record of each claim's comment-block digest as of the
// last comment operation the engine itself performed.
type Store struct {
	// Version is the on-disk schema version, written on every Save and read
	// on every load, so a format change stays migratable rather than a silent
	// unversioned break.
	Version int `json:"version"`

	// Digests maps a claim id -> CommentsDigest(claim) as of the engine's last
	// comment write to it. A claim with NO entry is simply unknown to the
	// store and reports no drift; a claim WITH an entry is covered, including
	// the "no threads at all" state — see CommentsDigest for why recording the
	// empty case is what makes a hand-added thread detectable.
	Digests map[string]string `json:"digests"`

	path string

	// fileExists records whether the store file was present at load, so a
	// caller can adopt on first creation (see Adopt).
	fileExists bool
}

// FileExists reports whether the store file was present when this store was
// loaded. See Adopt.
func (s *Store) FileExists() bool { return s.fileExists }

// LoadStore reads the digest store from path. A missing file is not an error:
// it is an empty, freshly-initialized store, which is the normal state of a
// project that has never had a comment written through the engine.
func LoadStore(path string) (*Store, error) {
	s := &Store{
		Version: StoreSchemaVersion,
		Digests: map[string]string{},
		path:    path,
	}

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("digest: read store %s: %w", path, err)
	}

	var onDisk struct {
		Version int               `json:"version"`
		Digests map[string]string `json:"digests"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		return nil, fmt.Errorf("digest: parse store %s: %w", path, err)
	}
	if onDisk.Digests != nil {
		s.Digests = onDisk.Digests
	}
	s.fileExists = true
	return s, nil
}

// Save writes the store back to its path as JSON, atomically (temp file in the
// same directory, then rename) for the same reason internal/lock.Store.Save
// does: a reader outside the writer's critical section must only ever observe
// the old complete file or the new complete file, never a truncated one.
func (s *Store) Save() error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("digest: marshal store: %w", err)
	}
	if err := atomicWriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("digest: write store %s: %w", s.path, err)
	}
	return nil
}

// Digest returns the recorded comment digest for claimID and whether one
// exists. No entry means the store has never seen this claim; the gate treats
// that as "not covered", never as "drifted".
func (s *Store) Digest(claimID string) (string, bool) {
	if s == nil || s.Digests == nil {
		return "", false
	}
	d, ok := s.Digests[claimID]
	return d, ok
}

// Record refreshes c's digest to c's CURRENT comment block. It is called at the
// single choke point every comment write goes through (internal/comments'
// mutate), for the claim that was just written, AFTER the claim file itself was
// saved.
//
// The ordering is chosen so the failure mode is loud rather than silent. If the
// process dies between the claim save and this refresh, the recorded digest
// LAGS the file and the gate reports comment-ledger-drift — a false positive a
// human can see and clear. Refreshing first would invert that: a crash would
// leave a digest describing comments the file does not have, and the next
// hand-edit back to that state would pass. An integrity check may cry wolf; it
// may not be quietly wrong.
func (s *Store) Record(c model.Claim) {
	if s.Digests == nil {
		s.Digests = map[string]string{}
	}
	s.Digests[c.ID] = CommentsDigest(c)
}

// Forget drops a claim's digest entry — for a claim that no longer exists. It
// is not called on delete-a-thread (that is an ordinary Record of the new,
// smaller comment block).
func (s *Store) Forget(claimID string) {
	delete(s.Digests, claimID)
}

// Adopt records a digest for every claim that does not already have one, and
// reports the ids it adopted (sorted). It is how a project that upgrades into
// this feature gets coverage for comments written before the store existed:
// without it, a claim's existing threads would be unprotected until the next
// time someone happened to comment on it.
//
// Adoption is honest about what it establishes, which is only "these were the
// threads on disk at adoption time" — the same caveat internal/lock's
// grandfathering carries. It is deliberately WEAKER than the lock ledger's rule
// (which refuses to adopt when its file is absent, so that deleting the ledger
// cannot re-bless a project): a deleted digest store re-adopts. That asymmetry
// is chosen, not overlooked. The lock ledger guards the trust boundary — locked
// content, including the only unescaped-HTML render path in the engine — where
// a bypass is worth closing at the cost of a hard error. A comment digest
// guards a review-workflow gate, where the same hard error would mean a project
// whose digest file was never created (any project that has not yet run a
// comment op) could not run the gate at all.
func Adopt(s *Store, claims []model.Claim) (adopted []string) {
	for _, c := range claims {
		if _, ok := s.Digest(c.ID); ok {
			continue
		}
		s.Record(c)
		adopted = append(adopted, c.ID)
	}
	sort.Strings(adopted)
	return adopted
}

// CommentsDigest returns a deterministic hash of c's entire comment block:
// every thread, every reply, every lifecycle field, in declaration order.
//
// Declaration order is part of the digest because it is part of the file: the
// threads are a list, and reordering them is an edit a reviewer reading the
// diff would see, so the digest should see it too.
//
// A claim with NO comments hashes to a well-defined value rather than to the
// empty string, and that is load-bearing. Recording the empty state is what
// makes "hand-add a thread to a claim whose last thread was legitimately
// deleted" detectable: if absence were encoded as "no entry", every claim would
// fall back to uncovered the moment its comments were emptied — which is
// precisely the state an attacker wants to leave a claim in.
func CommentsDigest(c model.Claim) string {
	h := sha256.New()
	fmt.Fprintf(h, "dossierx-comment-digest/v%d\nclaim=%s\nthreads=%d\n", digestVersion, c.ID, len(c.Comments))
	for _, t := range c.Comments {
		fmt.Fprintf(h, "t:%s|%s|%s|%s|%t|%s|%s|%s|%s|replies=%d\n",
			t.ID, t.Status, t.Author, t.Created, t.Edited,
			t.ResolvedBy, t.ResolvedAt, t.ReopenedBy, t.ReopenedAt, len(t.Replies))
		// Bodies are length-prefixed and hashed separately from the metadata
		// line so a body containing the "|" separator (or a newline) cannot be
		// crafted to imitate a different thread's field layout.
		fmt.Fprintf(h, "tb%d:%s\n", len(t.Body), t.Body)
		for _, r := range t.Replies {
			fmt.Fprintf(h, "r:%s|%s|%s|%t\n", r.ID, r.Author, r.Created, r.Edited)
			fmt.Fprintf(h, "rb%d:%s\n", len(r.Body), r.Body)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// atomicWriteFile writes data to path without ever leaving a reader able to
// observe a partially-written file: it writes to a temp file created in path's
// own directory (so the rename stays on one filesystem, which is what makes it
// atomic) and then renames it over path. Duplicated here rather than imported
// from internal/lock or internal/loader — both of which already have their own
// copy, independently — to keep this package importable BY internal/lock
// without a cycle, which is the entire reason it is its own package.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
