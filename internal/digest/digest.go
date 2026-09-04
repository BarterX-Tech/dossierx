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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// nowFunc is this package's clock, overridable in tests so a recorded
// re-adoption's timestamp can be asserted deterministically instead of racing
// real time — the same pattern internal/lock.nowFunc follows.
var nowFunc = time.Now

// StoreSchemaVersion is the on-disk schema version of the comment digest
// store. Version 1 is its first shipped shape.
const StoreSchemaVersion = 1

// digestVersion is the version of the DIGEST ALGORITHM below, mixed in as a
// domain separator. Bumping it invalidates every recorded digest (every claim
// would read as drifted), so a bump must come with a StoreSchemaVersion bump
// and a re-adoption — same contract as internal/lock's LockedClaimHash.
const digestVersion = 1

// StoreFileName is the digest store's BASE NAME, an alias of
// config.CommentDigestFileName. The file sits next to the lock store under the
// project's build directory (config.Config.CommentDigestPath(),
// build/ledger/comment-digest.json by default), never cwd, and outside
// claims_dir so it is never itself decoded as a claim.
//
// It is exported because internal/check's index scan has to RECOGNISE the file
// by base name. A finding that NAMES the file prints
// config.CommentDigestDisplayPath instead: the bare base name is two
// directories away from the file, and the gate may be reading a store
// materialized out of the git index into a temp directory, so the honest thing
// to print is the project-relative display form, not the path it read.
const StoreFileName = config.CommentDigestFileName

// StorePath returns the digest store's path for cfg. Callers that need to
// serialize on it acquire lock.AcquireFileLock(StorePath(cfg)) — INSIDE the
// claims sentinel; see this package's doc comment.
func StorePath(cfg *config.Config) string {
	return cfg.CommentDigestPath()
}

// StorePathBeside returns the digest store's path for a project identified by
// its LOCK STORE's path — the two files are siblings under the build
// directory's ledger/ subdirectory, by construction (config.Config.LockStorePath
// and CommentDigestPath join the same directory), and internal/digest's tests
// pin StorePathBeside(cfg.LockStorePath()) == cfg.CommentDigestPath() so a
// wrong sibling — which digest.LoadStore would read as "no digest store", a
// silent pass — fails by name.
//
// It exists for exactly one caller: internal/lock's PrepareStore, which
// grandfathers a pre-ledger project and must create the digest store at the same
// moment, and which is handed a *lock.Store rather than a *config.Config. Taking
// the path from the store it already has beats widening a signature that four
// commands call, and the two paths cannot drift apart because both are derived
// from the same directory.
func StorePathBeside(lockStorePath string) string {
	return filepath.Join(filepath.Dir(lockStorePath), StoreFileName)
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

	// Reaudits records every human-authorised RE-ADOPTION of a claim's comment
	// block: who asked for it, when, and in whose words (see Store.Reaudit).
	//
	// It is on the record, in the tracked file, for the reason unlock stamps
	// ReleasedAt/By/Reason on a ledger record instead of deleting it: the one
	// operation that legitimately makes a drift finding go away must leave
	// evidence that it happened, or it is indistinguishable from the tampering
	// it clears. It is append-only and `omitempty`, so a project that has never
	// needed the recovery keeps a store shaped exactly as before.
	Reaudits map[string][]Reaudit `json:"reaudits,omitempty"`

	path string

	// fileExists records whether the store file was present at load, so a
	// caller can adopt on first creation (see Adopt).
	fileExists bool
}

// Reaudit is one human-authorised re-adoption of a claim's comment block into
// the digest store — the recovery for a digest that LAGS its claim file, which a
// crash between the claim save and the digest refresh (or a commit carrying one
// file and not the other) leaves behind. See Store.Reaudit.
type Reaudit struct {
	// At is the RFC3339Nano UTC time the re-adoption was recorded.
	At string `json:"at"`
	// Actor is who the machine says ran the command. Provenance, not identity,
	// exactly as lock.LedgerRecord.Actor is.
	Actor string `json:"actor"`
	// Reason is the human's own words for why the block on disk is the block to
	// trust. It is the only part a machine cannot generate for itself, which is
	// why the command that writes it requires one.
	Reason string `json:"reason"`
	// Digest is the value adopted, so a reader of the diff can see WHICH block
	// this re-adoption blessed rather than only that one happened.
	Digest string `json:"digest"`
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
	if err := decodeInto(s, raw, false); err != nil {
		return nil, fmt.Errorf("digest: parse store %s: %w", path, err)
	}
	return s, nil
}

// DecodeStore decodes a digest store from bytes already in hand — the git
// index's copy, under "check --staged" — STRICTLY: an unknown key is refused
// and "version" must be StoreSchemaVersion, so a caller asking "is this blob
// OUR store?" gets a no for `{}` and for unrelated JSON that happens to sit at
// the generic name comment-digest.json. LoadStore and DecodeStore share one
// parser (decodeInto) and only THIS entry point refuses unknown keys: LoadStore
// keeps json.Unmarshal's tolerance, as every release before this one had, so a
// key a later release adds to the store does not make check on the older
// binary fail with lock-ledger-unreadable and prescribe restoring a file that
// is not corrupt — the same split internal/lock draws between its two entry
// points. The returned store has no path and cannot be saved.
func DecodeStore(raw []byte) (*Store, error) {
	var probe struct {
		Version *int `json:"version"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&probe); err != nil {
		return nil, fmt.Errorf("digest: decode store: %w", err)
	}
	if probe.Version == nil {
		return nil, fmt.Errorf("digest: decode store: no version field")
	}
	if *probe.Version != StoreSchemaVersion {
		return nil, fmt.Errorf("digest: decode store: version %d is not one this engine writes", *probe.Version)
	}
	s := &Store{Version: StoreSchemaVersion, Digests: map[string]string{}}
	if err := decodeInto(s, raw, true); err != nil {
		return nil, fmt.Errorf("digest: decode store: %w", err)
	}
	return s, nil
}

// decodeInto is the one parser behind LoadStore and DecodeStore. strict
// refuses unknown keys (the on-disk shape is exactly what Save writes; the
// index's copy under check --staged is judged on that); lenient ignores them.
func decodeInto(s *Store, raw []byte, strict bool) error {
	var onDisk struct {
		Version  int                  `json:"version"`
		Digests  map[string]string    `json:"digests"`
		Reaudits map[string][]Reaudit `json:"reaudits"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(&onDisk); err != nil {
		return err
	}
	if onDisk.Digests != nil {
		s.Digests = onDisk.Digests
	}
	s.Reaudits = onDisk.Reaudits
	s.fileExists = true
	return nil
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
	// The store lives under build/ledger/, which a fresh project does not
	// have until the first write creates it.
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("digest: create store dir for %s: %w", s.path, err)
	}
	if err := atomicWriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("digest: write store %s: %w", s.path, err)
	}
	return nil
}

// CheckWritable reports whether Save could persist this store, WITHOUT writing
// it — by creating and removing a probe file exactly where Save's temp file
// would go (see atomicWriteFile: same directory, so the rename stays on one
// filesystem).
//
// It exists so a caller can find out that the store is unwritable BEFORE it
// commits the change the store is supposed to record. internal/comments is that
// caller, and the ordering matters more there than anywhere else: its write path
// saves the claim first and refreshes the digest second (Record explains why
// that order is the only safe one), so a store it cannot write turns into "the
// comment is on disk AND the op reported failure" — and an agent's ordinary
// response to a failure is to retry, appending the same thread again on every
// attempt. Probing first turns that into a clean refusal that wrote nothing.
//
// It is a probe, not a guarantee: the directory can become unwritable between
// the probe and the Save, and on Windows a rename over a read-only FILE can fail
// even when the directory is writable. It removes the common, persistent causes
// — a read-only project directory, a full disk, a directory that does not exist
// — not the racing ones.
func (s *Store) CheckWritable() error {
	dir := filepath.Dir(s.path)
	probe, err := os.CreateTemp(dir, filepath.Base(s.path)+".probe-*")
	if err != nil {
		return fmt.Errorf("digest: the comment digest store %s cannot be written: %w", s.path, err)
	}
	name := probe.Name()
	probe.Close()          //nolint:errcheck // the probe's content is never read
	return os.Remove(name) //nolint:gocritic // removing our own probe is the last step
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

// Reaudit re-adopts c's CURRENT comment block as the recorded truth, and
// records who authorised it and why (see Store.Reaudits).
//
// It is the ONLY sanctioned way a comment-ledger-drift finding is cleared
// without restoring the claim file, and it exists because the state that
// produces that finding is not always tampering. The engine saves the claim
// first and refreshes the digest second (see Record for why that order is the
// only safe one), so a crash in between — or a commit that carries the claim
// file and not the digest store, which reproduces the same state for every
// teammate who pulls — leaves a digest that LAGS the file. The refusal that
// follows is total: every comment op on that claim is refused, because the
// integrity check runs before the mutation, so the advice the engine used to
// give ("restore the claim file from version control") discarded the comment the
// human had actually written and there was no other way out.
//
// It is deliberately per-claim and reason-carrying rather than a blanket
// re-adopt. Blessing a comment block is exactly what an attacker wants an
// integrity tool to do, so the act is narrowed to one named claim and leaves the
// human's words in the tracked file beside the value it adopted.
func (s *Store) Reaudit(c model.Claim, actor, reason string) {
	s.Record(c)
	if s.Reaudits == nil {
		s.Reaudits = map[string][]Reaudit{}
	}
	s.Reaudits[c.ID] = append(s.Reaudits[c.ID], Reaudit{
		At:     nowFunc().UTC().Format(time.RFC3339Nano),
		Actor:  actor,
		Reason: reason,
		Digest: s.Digests[c.ID],
	})
}

// Forget drops a claim's digest entry — for a claim that no longer exists. It
// is not called on delete-a-thread (that is an ordinary Record of the new,
// smaller comment block).
//
// Its ONE caller is lock.SweepCommentDigests, and only for an entry whose
// departure is accounted for: a claim that recorded NO threads (deleting it
// erases no review history) or one whose lock-ledger record was released by an
// honest unlock. Everything else is left in place on purpose — an orphaned entry
// that recorded real threads is the evidence internal/check's
// comment-digest-abandoned rule is built from, and it is evidence precisely
// because a rename cannot reach it.
func (s *Store) Forget(claimID string) {
	delete(s.Digests, claimID)
}

// EmptyCommentsDigest is CommentsDigest of claimID with NO comment threads —
// the value a covered claim's entry holds when its comment block is empty.
//
// It exists so a caller can ask "did this entry ever record any review
// history?" without holding the claim it describes, which is exactly the
// question about a claim that is no longer in the project (see Store.Forget and
// lock.AbandonedCommentDigests). It is derived from CommentsDigest rather than
// spelled out, so the two can never disagree about what "no threads" hashes to.
func EmptyCommentsDigest(claimID string) string {
	return CommentsDigest(model.Claim{ID: claimID})
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
