// Package lock implements the claim lock lifecycle described in
// FORMAT.md:
//
//	draft -> locked            via Lock (human-initiated; refused on any
//	                           error-severity lint finding OR on an unresolved
//	                           comment thread)
//	locked -> locked+pending   on ANY of three independent triggers — a
//	                           dependency's content hash drifts (DetectStale),
//	                           a "dossierx claim flag" records a spec mismatch, or a
//	                           comment thread is opened on the locked claim
//	locked+pending -> locked   once EVERY trigger is gone, via ANY of three
//	                           clearers — a human-confirmed "dossierx reaudit
//	                           --confirm" (drift/flag), "dossierx unlock" then
//	                           re-lock, or resolving/deleting the last open
//	                           comment thread while no drift or flag still stands
//
// review_pending is set automatically but never cleared automatically: every
// clearer above is either human-initiated (unlock) or gated on a human-confirmed
// reaudit / an explicit comment resolution, so a locked claim's Status never
// reverts to draft on its own. The three-trigger recomputation itself lives in
// internal/comments (PendingTriggers/Recompute) so drift, flag, and open-thread
// state can never diverge across the lock gate, the reaudit path, and check.
//
// Store is a small JSON-file-backed table of dependency content hashes,
// keyed per-dependent (Hashes[dependentID][depID]), used to detect when a
// claim a locked claim depends on (see BaselineDependencyIDs) has changed
// underneath it. This is load-bearing: dossierx check's staleness detection
// and dossierx lock's baseline-recording both go through it, so its file
// format is considered part of the engine's on-disk contract, not an
// implementation detail — and is versioned (see storeSchemaVersion) so a
// format change like the per-dependent re-keying stays migratable.
package lock

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/atomicfile"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// nowFunc is the store's clock, overridable in tests so lock-timestamp
// refreshes can be asserted deterministically instead of racing real time.
var nowFunc = time.Now

// storeSchemaVersion is the on-disk schema version of the lock hash store, and
// nestedHashSchemaVersion / ledgerSchemaVersion are the two versions at which
// its two migratable changes landed. They are separate constants on purpose:
// the old code compared the on-disk version against "the current version" to
// decide whether to DROP the baselines, which was correct only while there was
// exactly one migration. Bumping the current version with that comparison still
// in place would have silently thrown away every v1 store's per-dependent
// baselines — re-opening the DX-AUD-09 drift hole for every existing project on
// upgrade day, and then handing MigrateLegacyStore an empty Hashes map, which
// it would re-arm from CURRENT content, blessing any drift that happened before
// the upgrade as the new baseline. Each migration therefore keys on the version
// that introduced ITS shape, never on "current".
//
// Version 1 introduced PER-DEPENDENT hash baselines: Hashes became
// map[dependentID]map[depID]hash, so each locked claim records its own
// snapshot of every dependency it rests on. A store file carrying no
// "version" field (Version == 0 after decode) predates that change and holds
// the legacy flat map[depID]hash; LoadStore migrates it — see LoadStore.
//
// Version 2 introduced the LOCK LEDGER (Store.Ledger — see ledger.go): a record
// per locked artifact of the content that was approved, when, by whom, and on
// whose words. A store at version < 2 predates the ledger, and nothing here is
// grandfathered: such a store crosses to version 2 only through
// CrossPreLedger, which refuses while the project still holds a locked artifact
// — see it for why the trigger is "the file exists at an older version" and
// never "the ledger is empty".
const (
	storeSchemaVersion      = 3
	nestedHashSchemaVersion = 1
	ledgerSchemaVersion     = 2
	policySchemaVersion     = 3
)

// PolicyVersion records which approval semantics a store was written under.
// Local approval v1 is the only policy this engine runs. The legacy policy 0
// ("every rests_on target must already be locked") was retired in v0.7.21:
// a store that records it, or predates the field, loads as v1 and is stamped
// v1 on its next write (see decodeStore). Every approval such a store holds
// was granted under the stricter legacy rule, so none of them is reinterpreted.
type PolicyVersion int

const PolicyLocalApprovalV1 PolicyVersion = 1

// retiredPolicyMigrationReason is stamped on a store the first time a write
// carries it off the retired policy 0, so the adoption shows in the diff a
// reviewer reads rather than happening silently.
const retiredPolicyMigrationReason = "lock policy 0 retired in v0.7.21; this store now records local approval v1"

// DependencyReceipt is the exact readable dependency boundary a local approval
// reviewed. Hash is a comparison aid only; Content keeps the reviewed text
// recoverable when the working draft later changes or disappears.
type DependencyReceipt struct {
	Hash    string      `json:"hash"`
	Content model.Claim `json:"content"`
}

// StoreFileName is the lock store's BASE NAME, an alias of
// config.LockStoreFileName. The file lives under the project's build directory
// at config.Config.LockStorePath() (build/ledger/lock-store.json by default),
// never cwd, beside the comment digest store.//
// It is exported for the same reason digest.StoreFileName is: internal/check's
// index scan has to RECOGNISE the file by base name. It is the name, not the
// path, and not the display form either — a message that names the file
// prints config.LockStoreDisplayPath, because the bare base name is two
// directories away from the file a reader has to open.
const StoreFileName = config.LockStoreFileName

// Store is the on-disk (JSON) record of dependency content hashes as of each
// locked claim's most recent lock or confirmed reaudit.
type Store struct {
	// Version is the store's on-disk schema version (see storeSchemaVersion).
	// It is written on every Save and read on every Load so a format change
	// like the per-dependent re-keying stays migratable rather than a silent,
	// unversioned break.
	Version int `json:"version"`

	// PolicyVersion is deliberately separate from Version. Version describes the
	// JSON shape; this field records a human-visible policy migration and must
	// never be inferred from the current binary.
	PolicyVersion         PolicyVersion `json:"policy_version,omitempty"`
	PolicyMigratedAt      string        `json:"policy_migrated_at,omitempty"`
	PolicyMigrationReason string        `json:"policy_migration_reason,omitempty"`

	// Hashes records dependency baselines keyed PER DEPENDENT:
	// Hashes[dependentID][depID] is ContentHash(dep) as observed the last
	// time dependentID itself was locked or reaudited-and-confirmed. Keying
	// per-dependent (rather than by dependency id alone) is load-bearing: two
	// locked claims that share a dependency each keep their OWN baseline for
	// it, so locking/reauditing one never overwrites the other's baseline and
	// masks real drift the other should have flipped review_pending on.
	Hashes map[string]map[string]string `json:"hashes"`

	// Receipts preserve each reviewed dependency boundary for local approvals.
	// They are keyed the same way as Hashes: dependent then dependency id.
	Receipts map[string]map[string]DependencyReceipt `json:"receipts,omitempty"`

	// LockedAt maps a locked claim's own ID -> the RFC3339Nano timestamp of
	// its most recent "dossierx lock" or confirmed "dossierx reaudit". Per
	// FORMAT.md, a confirmed reaudit refreshes this alongside the
	// dependency content hash, even when the proposal was a no-change
	// confirmation.
	LockedAt map[string]string `json:"locked_at"`

	// Ledger is the lock ledger (schema version 2 and later): one
	// LedgerRecord per locked claim, keyed by claim id. See ledger.go's
	// file doc for what it is for, and audit.go for the rules read off it.
	// It is `omitempty` so a project with nothing locked keeps a store file
	// shaped exactly as the pre-ledger one.
	Ledger map[string]LedgerRecord `json:"ledger,omitempty"`

	// Constitution is the roof's lock record (NIT-6): the content hash
	// `constitution lock` approved, the human's reason and the time. It is
	// what check and every claim-lock path compare the file against
	// (constitution.Evaluate); nil means the constitution was never locked.
	// It is a sibling of Ledger rather than a record inside it because the
	// constitution is not a claim: none of the ledger's per-claim rules
	// (orphan, abandoned, released) mean anything for it.
	Constitution *constitution.LockRecord `json:"constitution,omitempty"`

	path string

	// diskVersion is the schema version this store was DECODED FROM, as
	// opposed to Version, which LoadStore always sets to the current version
	// so Save writes a current-shaped file. Every migration keys on
	// diskVersion: after a load, Version no longer says anything about what
	// was on disk.
	diskVersion int

	// fileExists records whether the store file was actually there. It is
	// load-bearing for the pre-ledger predicates, which must distinguish "an
	// older store that never had a ledger" (crossable, once the project holds
	// nothing locked) from "no store at all while locked claims exist" (never
	// crossable — see Store.PreLedger).
	fileExists bool

	// ledgerKeyOnDisk records whether the decoded file carried a "ledger" KEY,
	// as opposed to carrying ledger RECORDS. The two differ by exactly one
	// attacker edit — emptying the map instead of deleting the key — and the key
	// alone is already conclusive: it did not exist before schema 2, so a store
	// that claims to predate the ledger cannot honestly have one. See
	// Store.LedgerDowngraded.
	ledgerKeyOnDisk bool

	// rebaselined is the claim ids MigrateLegacyStore re-armed dependency
	// baselines for on this load — read through Rebaselined() so a caller can
	// name them in its envelope. See MigrateLegacyStore.
	rebaselined []string

	// commentDigestsAdopted is the claim ids SweepCommentDigests took COMMENT
	// DIGEST coverage of on this run — read through CommentDigestsAdopted() for
	// the same reason rebaselined is read through Rebaselined(): adoption
	// records "whatever the file said just now" as the truth, and a caller that
	// cannot name what it adopted cannot warn about it. It rides on the store
	// rather than in PrepareStore's return so the four commands that already
	// call PrepareStore keep compiling and can surface it when they are ready.
	commentDigestsAdopted []string
}

// OnDiskVersion returns the schema version this store was decoded from, or 0
// for a store whose file did not exist. Exported for the ledger gate, which
// must tell "this project predates the ledger" from "someone deleted the
// ledger".
func (s *Store) OnDiskVersion() int { return s.diskVersion }

// FileExists reports whether the store file existed when this store was loaded.
// See OnDiskVersion.
func (s *Store) FileExists() bool { return s.fileExists }

// Baseline returns the recorded content hash of dependency depID as of the
// last time dependent dependentID was locked or reaudited-and-confirmed, and
// whether such a baseline exists. Baselines are keyed per-dependent (see
// Store.Hashes' doc comment).
func (s *Store) Baseline(dependentID, depID string) (string, bool) {
	deps, ok := s.Hashes[dependentID]
	if !ok {
		return "", false
	}
	h, ok := deps[depID]
	return h, ok
}

// recordBaseline records dep's content hash under the dependent claim's own
// id, allocating the per-dependent sub-map on first use.
func (s *Store) recordBaseline(dependentID, depID, hash string) {
	if s.Hashes == nil {
		s.Hashes = map[string]map[string]string{}
	}
	if s.Hashes[dependentID] == nil {
		s.Hashes[dependentID] = map[string]string{}
	}
	s.Hashes[dependentID][depID] = hash
}

// recordReceipt keeps the complete readable dependency boundary alongside its
// comparable hash. It is written only as part of an approval/explicit
// reapproval snapshot; status and review bookkeeping remain excluded from the
// comparison hash and never manufacture semantic proof.
func (s *Store) recordReceipt(dependentID string, dep model.Claim) {
	if s.Receipts == nil {
		s.Receipts = map[string]map[string]DependencyReceipt{}
	}
	if s.Receipts[dependentID] == nil {
		s.Receipts[dependentID] = map[string]DependencyReceipt{}
	}
	s.Receipts[dependentID][dep.ID] = DependencyReceipt{
		Hash:    ContentHash(dep),
		Content: dep,
	}
}

// Receipt returns the preserved reviewed boundary for dependentID's dependency.
func (s *Store) Receipt(dependentID, depID string) (DependencyReceipt, bool) {
	if s == nil || s.Receipts == nil {
		return DependencyReceipt{}, false
	}
	r, ok := s.Receipts[dependentID][depID]
	return r, ok
}

// LoadStore reads the hash store from path. A missing file is not an
// error: it is treated as an empty, freshly-initialized store (the common
// case for a project's first "dossierx lock").
//
// On-load migration: a store predating per-dependent baselines carries no
// "version" field (so its decoded Version is 0) and a legacy flat
// map[depID]hash under "hashes". Those flat baselines cannot be attributed
// to a specific dependent — the whole reason DX-AUD-09 was a bug — so
// re-keying them per-dependent would mean fabricating baselines, which could
// SUPPRESS real drift (a dependent whose dependency actually changed since
// its own lock would be handed a fresh-looking baseline it never recorded).
// The safe migration is therefore to DROP the legacy flat hashes entirely
// and let every locked claim re-baseline on its next lock/reaudit. Until
// then DetectStale simply finds no baseline for those dependents and reports
// no drift — never a crash, never a spurious review_pending.
//
// That drop is scoped to schema 0 by comparing against nestedHashSchemaVersion,
// NOT against the current version. This matters more than it looks: while there
// was one migration the two were the same number, and comparing against
// "current" read as correct. It is not. A store at version 1 holds perfectly
// good NESTED baselines, and dropping them on the version-2 upgrade would take
// dependency-drift detection down for every existing project — and then hand
// MigrateLegacyStore an empty Hashes map, which it re-arms from CURRENT
// content, silently adopting as the new baseline whatever drift had already
// happened. That is the exact silent drift hole this comparison exists to
// prevent, so it must stay keyed on the version that changed the SHAPE.
func LoadStore(path string) (*Store, error) {
	raw, err := readStoreBytes(path)
	if os.IsNotExist(err) {
		return emptyStore(path), nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock: read store %s: %w", path, err)
	}
	s, err := decodeStore(raw, path)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func readStoreBytes(path string) ([]byte, error) {
	return readStoreWithRetry(path, os.ReadFile, time.Now, time.Sleep, lockReadIsTransient)
}

func readStoreWithRetry(path string, read func(string) ([]byte, error), now func() time.Time, sleep func(time.Duration), transient func(error) bool) ([]byte, error) {
	deadline := now().Add(lockAcquireTimeout)
	for {
		raw, err := read(path)
		if err == nil {
			return raw, nil
		}
		if os.IsNotExist(err) || !transient(err) {
			return nil, err
		}
		if !now().Before(deadline) {
			return nil, err
		}
		sleep(lockPollInterval)
	}
}

// emptyStore is the empty, freshly-initialised store a missing file loads as.
func emptyStore(path string) *Store {
	return &Store{
		Version:       storeSchemaVersion,
		PolicyVersion: PolicyLocalApprovalV1,
		Hashes:        map[string]map[string]string{},
		Receipts:      map[string]map[string]DependencyReceipt{},
		LockedAt:      map[string]string{},
		path:          path}
}

// DecodeStore decodes a lock store from bytes already in hand — the git index's
// copy, under "check --staged" — STRICTLY: an unknown top-level key is refused,
// and the "version" must be one this package has ever written (the nested-hash
// schema or the ledger schema; a pre-ledger store is still dossierx content).
// Strictness is the point: LoadStore's decoder accepts `{}` and any JSON with
// a "version" key, and once the store's base name is the generic
// lock-store.json a caller asking "is this blob OUR store?" needs a decoder
// that can say no. There is one parser (decodeStore) behind both entry points.
func DecodeStore(raw []byte) (*Store, error) {
	var probe map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&probe); err != nil {
		return nil, fmt.Errorf("lock: decode store: %w", err)
	}
	for key := range probe {
		switch key {
		case "version", "policy_version", "policy_migrated_at", "policy_migration_reason", "hashes", "receipts", "locked_at", "ledger", "constitution":
		default:
			return nil, fmt.Errorf("lock: decode store: unknown key %q", key)
		}
	}
	versionRaw, ok := probe["version"]
	if !ok {
		return nil, fmt.Errorf("lock: decode store: no version field")
	}
	var version int
	if err := json.Unmarshal(versionRaw, &version); err != nil {
		return nil, fmt.Errorf("lock: decode store: version: %w", err)
	}
	if version != nestedHashSchemaVersion && version != storeSchemaVersion {
		return nil, fmt.Errorf("lock: decode store: version %d is not one this engine writes", version)
	}
	return decodeStore(raw, "")
}

// decodeStore is the one parser behind LoadStore and DecodeStore; path is the
// file the bytes came from, for messages, or "" for an in-memory blob.
func decodeStore(raw []byte, path string) (*Store, error) {
	s := emptyStore(path)

	// Decode in two phases so a legacy flat "hashes" (map[depID]hash) can
	// never make json.Unmarshal fail against the new nested
	// map[dependentID]map[depID]hash shape: capture "hashes" as raw bytes
	// first, decide by schema version whether to keep or drop it, and only
	// then decode the ones we keep.
	//
	// "ledger" is captured as raw bytes for the same reason "hashes" is, plus one
	// of its own: whether the KEY was there at all is evidence, and a decoded map
	// cannot answer that — `"ledger": {}` and no ledger key both decode to an
	// empty map. See Store.ledgerKeyOnDisk.
	var onDisk struct {
		Version               int               `json:"version"`
		PolicyVersion         PolicyVersion     `json:"policy_version"`
		PolicyMigratedAt      string            `json:"policy_migrated_at"`
		PolicyMigrationReason string            `json:"policy_migration_reason"`
		Hashes                json.RawMessage   `json:"hashes"`
		Receipts              json.RawMessage   `json:"receipts"`
		LockedAt              map[string]string `json:"locked_at"`
		Ledger                json.RawMessage   `json:"ledger"`
		// Constitution is the roof's record (NIT-6). It is read at every
		// schema version: the record is a sibling of the ledger, not part of
		// it, so a pre-ledger store that carries one still knows its roof.
		Constitution *constitution.LockRecord `json:"constitution"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		return nil, fmt.Errorf("lock: parse store %s: %w", path, err)
	}
	if onDisk.LockedAt != nil {
		s.LockedAt = onDisk.LockedAt
	}
	s.Constitution = onDisk.Constitution
	if len(onDisk.Ledger) > 0 {
		s.ledgerKeyOnDisk = true
		var ledger map[string]LedgerRecord
		if err := json.Unmarshal(onDisk.Ledger, &ledger); err != nil {
			return nil, fmt.Errorf("lock: parse store %s ledger: %w", path, err)
		}
		if ledger != nil {
			s.Ledger = ledger
		}
	}
	// Remember what was actually on disk (see Store.diskVersion/fileExists).
	//
	// Version is set from the FILE, not to the current constant, and that is the
	// difference between a refusal that reproduces and one that evaporates. It
	// used to be a write-time constant: every Save re-stamped the current version
	// no matter what had been loaded, so the very next command that wrote the
	// store for any reason of its own — `claim unlock`, a lock, a check that
	// migrated something — silently repaired a downgraded version field. A
	// reviewer who ran `check --validate` after reading a lock-ledger-downgraded
	// report saw a clean project, and the two migrations that key on the version
	// got a fresh chance to fire on the next edit. PrepareStore's stated
	// invariant ("leaving the file exactly as found keeps the refusal
	// reproducible") was unreachable, because PrepareStore does not own the only
	// write.
	//
	// So Version is the version this store has EARNED: whatever the file said,
	// until a migration that actually ran raises it. MigrateLegacyStore sets it
	// on its success path and CrossPreLedger sets it on its own, so an honest
	// upgrade still stamps forward exactly once — and a store whose crossing was
	// REFUSED keeps its downgraded version on disk, where the gate keeps
	// reporting it until a human restores the file.
	s.fileExists = true
	s.diskVersion = onDisk.Version
	s.Version = onDisk.Version
	// Policy 0 is retired. A store that records it (or predates the field,
	// which omitempty wrote as absent) carries over to v1; the migration
	// fields say so on the next write, in the file a reviewer diffs.
	s.PolicyVersion = onDisk.PolicyVersion
	s.PolicyMigratedAt = onDisk.PolicyMigratedAt
	s.PolicyMigrationReason = onDisk.PolicyMigrationReason
	if s.PolicyVersion < PolicyLocalApprovalV1 {
		s.PolicyVersion = PolicyLocalApprovalV1
		s.PolicyMigratedAt = nowFunc().UTC().Format(time.RFC3339Nano)
		s.PolicyMigrationReason = retiredPolicyMigrationReason
	}

	// Legacy (pre-versioning, schema 0) store: drop its flat hashes, keep
	// LockedAt, and present it to callers as an already-migrated
	// current-version store. Scoped to the version that changed the hashes'
	// SHAPE — see this function's doc comment for why comparing against the
	// current version instead would be a silent drift hole.
	if onDisk.Version < nestedHashSchemaVersion {
		return s, nil
	}

	if len(onDisk.Hashes) > 0 {
		var nested map[string]map[string]string
		if err := json.Unmarshal(onDisk.Hashes, &nested); err != nil {
			return nil, fmt.Errorf("lock: parse store %s hashes: %w", path, err)
		}
		if nested != nil {
			s.Hashes = nested
		}
	}
	if len(onDisk.Receipts) > 0 {
		var receipts map[string]map[string]DependencyReceipt
		if err := json.Unmarshal(onDisk.Receipts, &receipts); err != nil {
			return nil, fmt.Errorf("lock: parse store %s receipts: %w", path, err)
		}
		if receipts != nil {
			s.Receipts = receipts
		}
	}
	return s, nil
}

// MigrateLegacyStore re-arms per-dependent hash baselines for a store that
// predates them, so an existing project's already-locked claims regain
// dependency-drift detection immediately on upgrade — with no manual re-lock
// and no spurious review_pending.
//
// LoadStore migrates a pre-versioning store by DROPPING its un-attributable
// flat hashes (see LoadStore's doc), which leaves every already-locked claim
// with no baseline: DX-AUD-09's drift safety net is down for exactly the
// claims that already exist, until each is manually re-locked or reaudited.
// MigrateLegacyStore closes that gap. When the store carries no baselines at
// all — Hashes empty, the state a legacy drop leaves behind (and equally a
// brand-new store) — it records, for every currently-LOCKED claim, the CURRENT
// content hash of each dependency it rests on, using the exact
// BaselineDependencyIDs set DetectStale compares so baselines and staleness
// checks always agree. It
// also stamps the store to the current schema version.
//
// Baselining against CURRENT content (rather than fabricating a historical
// baseline, which is impossible: the dropped flat hashes could not be
// attributed to any specific dependent) is what makes this safe:
//   - DetectStale immediately after migration reports NO drift (current ==
//     baseline), so no claim is spuriously flipped to review_pending;
//   - any dependency edited AFTER migration flips its dependent to
//     review_pending on the next DetectStale, exactly as a fresh lock would;
//   - a claim already review_pending before the upgrade stays so — that flag
//     lives in the claim's YAML, not the store, and DetectStale only ever sets
//     the flag, never clears it (this function never touches claims at all).
//
// It is idempotent: once baselines are present (Hashes non-empty) it does
// nothing and returns false, so re-running any command that calls it is a
// no-op. It reports whether it changed the store so callers can skip a
// needless Save.
//
// The caller must hold this store's file lock (AcquireFileLock) around the
// MigrateLegacyStore-and-Save sequence, the same as any other load-mutate-save
// on the shared store file.
func MigrateLegacyStore(s *Store, claims []model.Claim) (changed bool) {
	rebaselined := map[string]bool{}

	// Any recorded baseline means this store is already at (or past) the
	// per-dependent schema — a current-version store, or one an earlier call
	// already re-armed. Re-arming then would clobber real, differing baselines
	// with current content and mask genuine drift, so bail out.
	if len(s.Hashes) > 0 {
		return false
	}

	// A store already AT the per-dependent schema is never re-armed, even when
	// its baselines map is empty. The emptiness of a schema-0 store's map means
	// "LoadStore dropped un-attributable legacy hashes", which is the one
	// situation baselining from current content is the safe answer to. The
	// emptiness of a schema-1-or-later store means something else entirely —
	// most sharply, a claim hand-flipped to status: locked after the store was
	// written — and re-arming THAT would bless whatever its dependencies happen
	// to say right now as an approved baseline. The hand-flip itself is caught
	// by the ledger gate (lock-ledger-missing); this guard makes sure the drift
	// machinery does not quietly hand it a clean bill of health first.
	if s.diskVersion >= nestedHashSchemaVersion {
		return false
	}

	// THIRD guard, and the one that makes the two above worth having: a store
	// that says it predates per-dependent baselines, in a project that proves it
	// does not, is not migrated at all.
	//
	// The version field lives inside the audited file, so without this the
	// re-arm was re-armable with a text editor — and re-arming is not a neutral
	// act, it RECORDS CURRENT CONTENT AS THE APPROVED BASELINE. The reproduction
	// was three keystrokes wide: take a project where a sanctioned dependency
	// edit has correctly flipped a dependent to review_pending, delete the
	// `review_pending: true` line by hand (LockedClaimHash excludes it, so
	// lock-content-drift cannot see it), set `"version": 0`, and run plain
	// `dossierx check`. LoadStore drops the hashes for version < 1 — which
	// defeats the `len(s.Hashes) > 0` guard — the diskVersion guard is defeated
	// by the same edit, and this function then re-baselined every dependency
	// from the DRIFTED content. review_pending never came back and check went
	// green. LedgerDowngraded is the evidence the edit does not control (see it
	// for both halves and for what it does not close); a store carrying ledger
	// records, or sitting beside a comment digest store, has provably never been
	// a schema-0 store.
	if s.LedgerDowngraded(digestStorePresentBeside(s.path)) {
		return false
	}

	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		for _, dep := range BaselineDependencyIDs(c) {
			depClaim, ok := findByID(claims, dep)
			if !ok {
				continue
			}
			s.recordBaseline(c.ID, dep, ContentHash(depClaim))
			changed = true
			rebaselined[c.ID] = true
		}
	}

	if changed {
		// It stamps the version IT earned — the per-dependent baseline schema —
		// and not the current one, which is what it used to write while the two
		// were the same number.
		//
		// They are not the same number any more, and the difference is now
		// load-bearing: stamping storeSchemaVersion here would take a v0.1.x
		// project (schema 0) straight to the LEDGER schema without a single ledger
		// record in it. Every locked claim in that project would then read as
		// covered-but-unrecorded — lock-ledger-deleted, "restore the store from
		// version control" — for a project whose only fault was being two versions
		// behind, and the crossing that should have run
		// (RuleLockLedgerPreLedger -> CrossPreLedger) would never be offered,
		// because the store would no longer claim to predate the ledger. While
		// grandfathering was implicit this could not happen: it ran in the same
		// PrepareStore and stamped the records in the same breath. The two
		// migrations now cross their lines separately, and each one may only claim
		// the schema it actually performed.
		s.Version = nestedHashSchemaVersion
		s.rebaselined = sortedKeys(rebaselined)
		// A migration that rewrites integrity baselines announces itself: it is a
		// one-time event that re-arms what "unchanged since approval" MEANS for every
		// claim named, and a run that does it silently is a run whose ok:true a
		// human cannot interpret. The ids are also kept on the store
		// (Store.Rebaselined) so a caller can put them in its envelope rather
		// than making a consumer read two streams.
		announceRebaseline(s.rebaselined)
	}
	return changed
}

// Rebaselined returns the claim ids whose dependency baselines MigrateLegacyStore
// re-armed from current content on this load, sorted; empty when no migration
// ran. It is how a command surfaces the re-arm in its machine envelope — see
// MigrateLegacyStore for why a silent re-baseline is not acceptable.
func (s *Store) Rebaselined() []string {
	if s == nil {
		return nil
	}
	return s.rebaselined
}

// CommentDigestsAdopted returns the claim ids whose comment blocks
// SweepCommentDigests took coverage of on this run, sorted; empty when nothing
// was adopted.
//
// It exists because adoption must never be SILENT. What an adoption establishes
// is only "these were the threads on disk just now" — never that anybody
// approved them — so the run that adopts is exactly the run a human should look
// at, and a command that reports ok:true with no findings on that run has not
// told the whole truth. Same contract as Rebaselined: the ids are on the store
// so the caller can put them in its machine envelope rather than making a
// consumer read a second stream.
func (s *Store) CommentDigestsAdopted() []string {
	if s == nil {
		return nil
	}
	return s.commentDigestsAdopted
}

// announceRebaseline writes the legacy-baseline re-arm notice. It goes to the
// package writer rather than one threaded through every caller because the
// migration is reached from five commands, and a notice each caller has to
// remember to print is one that a caller will eventually forget.
func announceRebaseline(ids []string) {
	if ledgerAnnounceWriter == nil || len(ids) == 0 {
		return
	}
	fmt.Fprintf(ledgerAnnounceWriter,
		"dossierx: lock store migrated — dependency baselines re-armed for %d already-locked claim(s)\n"+
			"  from the content on disk just now. Drift that happened BEFORE this run is adopted as the\n"+
			"  new baseline and will not be reported; drift after it will be. Review these claims:\n",
		len(ids))
	for _, id := range ids {
		fmt.Fprintf(ledgerAnnounceWriter, "    %s\n", id)
	}
}

// sortedKeys returns set's keys in sorted order — determinism for an
// announcement and an envelope that are both diffed between runs.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Save writes the store back to its path as JSON.
//
// The write is atomic (temp file in the same directory, then rename) rather
// than a direct os.WriteFile, for the same reason as loader.SaveClaim: a
// reader of the store outside AcquireFileLock's critical section (e.g. a
// future read-only "dossierx status") could otherwise observe a truncated file
// mid-write. Rename is atomic within one directory/filesystem, so any
// concurrent reader only ever sees the old complete file or the new
// complete file.
func (s *Store) Save() error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("lock: marshal store: %w", err)
	}
	// The store lives under build/ledger/, which does not exist in a fresh
	// project until the first write creates it.
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("lock: create store dir for %s: %w", s.path, err)
	}
	if err := atomicWriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("lock: write store %s: %w", s.path, err)
	}
	// A store being written for the FIRST time is a project crossing into
	// ledger-covered, and it must acquire its comment digest store at the same
	// instant. CrossPreLedger already does this for the project that CROSSES
	// (a pre-ledger store being stamped); this is the other door into the same
	// room, and it was open. A fresh project that reaches ledger-covered through
	// its first "claim lock" never crosses at all, so it ended up ledger-covered
	// with no digest store — which is
	// indistinguishable on disk from a project whose digest store was DELETED,
	// and that ambiguity is the whole reason check's comment-digest-absent rule
	// still has to require a surviving thread before it will fire.
	//
	// The gate is s.fileExists, read from LOAD time, and it is what keeps this
	// from becoming the laundering path itself: a project whose lock store is
	// already on disk never reaches here, so deleting the digest store from a
	// covered project is never quietly undone by the next lock.
	if !s.fileExists {
		ensureCommentDigestStore(s.path)
	}
	return nil
}

// ensureCommentDigestStore creates an EMPTY comment digest store beside the lock
// store at path, if there is not one there already.
//
// Empty, never adopted: at first creation no claim has been through the comment
// engine, so there is nothing legitimate to record. A hand-written comments:
// block present at this moment stays UNKNOWN to the digest rules — never
// blessed, never accused — which is the same conservative default Store.PreLedger
// takes for an absent lock ledger, and the opposite of what adopting would do.
//
// Best-effort by design: a project that cannot write this file is not one whose
// lock should fail. The cost of it not being written is that the project looks
// like one that predates this behaviour, which is the state everything here
// already tolerates.
func ensureCommentDigestStore(lockStorePath string) {
	createCommentDigestStore(lockStorePath) //nolint:errcheck // best-effort: see this function's doc comment
}

// createCommentDigestStore is ensureCommentDigestStore with its errors RETURNED
// rather than swallowed, so there is one creator of this file rather than two.
//
// CrossPreLedger needs the failing form and cannot take the best-effort one:
// crossing a project onto the ledger without the digest store beside it leaves
// it stamped as covered with no comment evidence at all, which internal/check
// reports as comment-digest-absent with a version-control recovery aimed at a
// file that never existed. A skipped write here is the QUIET failure, so it is
// returned; ensureCommentDigestStore's callers are the ones for whom a project
// that cannot write the file is not a project whose lock should fail.
//
// It takes the digest store's own file lock as a LEAF — nothing else is held
// while acquiring it — which is what lets it run from a path holding only the
// lock-store sentinel. See CrossPreLedger's LOCKING paragraph.
func createCommentDigestStore(lockStorePath string) error {
	if lockStorePath == "" {
		return nil
	}
	path := digest.StorePathBeside(lockStorePath)

	release, err := AcquireFileLock(path)
	if err != nil {
		return fmt.Errorf("lock: create comment digest store: %w", err)
	}
	defer release()

	store, err := digest.LoadStore(path)
	if err != nil {
		return fmt.Errorf("lock: create comment digest store: %w", err)
	}
	if store.FileExists() {
		return nil
	}
	if err := store.Save(); err != nil {
		return fmt.Errorf("lock: create comment digest store: %w", err)
	}
	return nil
}

// atomicWriteFile writes data to path without ever leaving a reader able to
// observe a partially-written file: it writes to a temp file created in
// path's own directory (so the later rename stays on one filesystem, which
// is what makes it atomic) and then renames it over path.
//
// The rename is delegated to atomicfile.Write so Windows gets the same
// bounded retry as loader.SaveClaim. A concurrent reader (dry-run / another
// lock process loading the store) can hold lock-store.json open; Windows then
// refuses MoveFileEx with a sharing violation. A single os.Rename turned that
// into "store write failed and recovery incomplete" on windows-latest Go
// stable in TestConcurrentClaimWritersNeverCorruptClaimFiles (CI run
// 36166155789). POSIX rename stays one-shot.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return atomicfile.Write(path, data, perm)
}

// ContentHash returns a deterministic hash of the parts of a claim that
// matter for staleness detection: everything a dependent claim's
// correctness could be affected by (its comparable content), but not
// engine-managed bookkeeping like Status or ReviewPending themselves.
func ContentHash(c model.Claim) string {
	// A minimal, explicit field list rather than hashing the whole struct:
	// this keeps ReviewPending/Status changes from ever being mistaken for
	// content changes, which would create a feedback loop. The list is also a
	// compatibility surface — every existing project's recorded baselines were
	// computed from it — so a field is only ever added the way raw_html is
	// added below: gated so that a claim that does not carry it keeps hashing
	// byte-identically to before.
	legacy := sha256.New()
	fmt.Fprintf(legacy, "id=%s\nfacet=%s\nmodule=%s\nlayout=%s\nbody=%s\n", c.ID, c.Facet, c.Module, c.Layout, c.Body)
	// summary is in the allowlist, but ONLY WHEN NON-EMPTY, same gating as
	// raw_html: an omitted field must keep hashing byte-identically to the
	// claim that never had the key. Editing a present summary is still an
	// unapproved content change (NIT-8). Missing summaries fail
	// summary-required; they are not grandfathered by this gate.
	for _, r := range c.Rows {
		fmt.Fprintf(legacy, "row=%v\n", r)
	}
	for _, s := range c.Steps {
		fmt.Fprintf(legacy, "step=%s\n", s)
	}
	// RESTS ON NONE (NIT-24) writes one line of its own; a target list writes
	// the per-id lines it always did, so no existing baseline moves.
	if c.RestsOn.None {
		fmt.Fprintf(legacy, "rests_on=none/%s\n", c.RestsOn.Reason)
	}
	for _, r := range c.RestsOn.IDs {
		fmt.Fprintf(legacy, "rests_on=%s\n", r)
	}
	// The retired governed_by edge used to write a "governed=" line here,
	// and the retired mirrors key a "mirrors=" line before rests_on. Both
	// are gone without a placeholder (NIT-29), so every ContentHash — and
	// with it every recorded dependency baseline — moved on that release.
	// That is the "no migration tooling" decision: a corpus that carries
	// baselines from before it re-locks by hand.

	// raw_html is in the allowlist, but ONLY WHEN NON-EMPTY. The conditional
	// is the whole point of this stanza and must not be "simplified" into an
	// unconditional Fprintf.
	//
	// Why it belongs at all: raw_html used to be legal only on layout: mockup
	// illustrations, which by and large had no inbound rests_on edges, so a
	// dependent could not be misled by an edit it could not see. v0.4.1 made
	// raw_html an ATTACHMENT legal on ANY layout — including a rule-bearing
	// claim other claims rest_on. Without this, editing that attachment would
	// change what the claim shows a reader while leaving every dependent
	// unflagged, which is exactly the staleness this hash exists to catch.
	//
	// Why it must stay conditional: this hash is the baseline recorded in
	// Store.Hashes for every locked claim's dependencies, so any change to the
	// bytes fed in re-hashes that claim and flips its dependents to
	// review_pending. Appending an unconditional "raw_html=\n" line would
	// therefore move the hash of EVERY claim in EVERY consuming project, and
	// the first run after upgrading would mark the entire graph stale —
	// migration-shaped churn out of a patch release. Gated on non-empty, a
	// claim without raw_html feeds byte-identical input to the one it fed
	// before and nothing churns; only the raw_html-bearing claims re-hash, and
	// only once. TestContentHash_RawHTMLIsHashedOnlyWhenPresent pins both
	// halves of that against a hash constant captured before the change.
	if c.RawHTML != "" {
		fmt.Fprintf(legacy, "raw_html=%s\n", c.RawHTML)
	}
	if c.Summary != "" {
		fmt.Fprintf(legacy, "summary=%s\n", c.Summary)
	}
	legacyDigest := legacy.Sum(nil)

	// The nil branch returns the completed historical digest literally. That is
	// the compatibility contract for every claim written before embodiment
	// existed and for every current claim that does not opt in.
	if c.Embodiment == nil {
		return hex.EncodeToString(legacyDigest)
	}

	// An opted-in claim moves into a separate, versioned domain. Hashing the
	// completed historical digest (rather than appending to its authored byte
	// stream) makes the boundary unforgeable by raw_html or any other legacy
	// string field. The remainder is a closed, length-framed canonical encoding
	// of semantic embodiment content. Adapter and Target stay excluded: they are
	// opaque observation addresses, not meaning a dependent relies upon. Check
	// IDs remain included because they are stable authored semantic identity.
	h := sha256.New()
	h.Write([]byte("dossierx/content-hash/embodiment/v1\x00")) //nolint:errcheck // hash.Hash.Write cannot fail
	h.Write(legacyDigest)                                      //nolint:errcheck // hash.Hash.Write cannot fail
	writeContentHashString(h, string(c.Embodiment.Mode))
	switch c.Embodiment.Mode {
	case model.EmbodimentModeCompare:
		h.Write([]byte{1}) //nolint:errcheck // compare arm
		checks := append([]model.EmbodimentCheck(nil), c.Embodiment.Checks...)
		sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
		writeContentHashUint64(h, uint64(len(checks)))
		for _, check := range checks {
			writeContentHashString(h, check.ID)
			if check.Expectation == nil {
				h.Write([]byte{0}) //nolint:errcheck // invalid-but-closed nil expectation
				continue
			}
			h.Write([]byte{1}) //nolint:errcheck // expectation present
			writeContentHashString(h, string(check.Expectation.Shape))
			switch check.Expectation.Shape {
			case model.ExpectationShapeSet:
				h.Write([]byte{1}) //nolint:errcheck // set arm
				members, ok := check.Expectation.Value.([]string)
				if !ok {
					h.Write([]byte{0}) //nolint:errcheck // invalid-but-closed wrong value type
					continue
				}
				h.Write([]byte{1}) //nolint:errcheck // correctly typed set
				members = append([]string(nil), members...)
				sort.Strings(members)
				writeContentHashUint64(h, uint64(len(members)))
				for _, member := range members {
					writeContentHashString(h, member)
				}
			case model.ExpectationShapeScalar:
				h.Write([]byte{2}) //nolint:errcheck // scalar arm
				value, ok := check.Expectation.Value.(string)
				if !ok {
					h.Write([]byte{0}) //nolint:errcheck // invalid-but-closed wrong value type
					continue
				}
				h.Write([]byte{1}) //nolint:errcheck // correctly typed scalar
				writeContentHashString(h, value)
			default:
				h.Write([]byte{0}) //nolint:errcheck // invalid-but-closed unknown shape
			}
		}
	case model.EmbodimentModeNone:
		h.Write([]byte{2}) //nolint:errcheck // none arm
		writeContentHashString(h, c.Embodiment.Reason)
	default:
		h.Write([]byte{0}) //nolint:errcheck // invalid-but-closed unknown arm
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeContentHashString(w hash.Hash, value string) {
	writeContentHashUint64(w, uint64(len(value)))
	w.Write([]byte(value)) //nolint:errcheck // hash.Hash.Write cannot fail
}

func writeContentHashUint64(w hash.Hash, value uint64) {
	var framed [8]byte
	binary.BigEndian.PutUint64(framed[:], value)
	w.Write(framed[:]) //nolint:errcheck // hash.Hash.Write cannot fail
}

// ErrPreLedgerUnadopted is the refusal every approval-recording path makes on a
// project whose lock store predates the lock ledger while the project still
// holds locked artifacts (see Store.PreLedgerUnadopted and CrossPreLedger).
// It is a sentinel because the CLI classifies it into a machine-readable
// error code, and the recovery — an ordered sequence of
// ordinary commands — has to be reachable from the envelope rather than only
// from the prose.
var ErrPreLedgerUnadopted = errors.New("lock: this project's lock store predates the lock ledger and still holds locked artifacts")

// Unlock transitions claim back to draft. This is always human-initiated
// and always allowed (no lint gate) — a project may need to unlock a
// locked claim precisely to fix the thing lint is complaining about.
//
// It RELEASES the claim's ledger record rather than deleting it, and takes the
// store and the approval to do so. The store parameter is what makes
// lock-ledger-orphan a precise rule instead of a heuristic: a draft claim
// holding an UNRELEASED ledger record was flipped out of locked by something
// that was not this function. A nil store is tolerated (in-memory callers with
// no ledger); a claim with no record at all releases nothing, which is
// deliberate — unlock is the recovery escape hatch and must never fail.
func Unlock(claim model.Claim, store *Store, ap Approval) model.Claim {
	claim.Status = model.StatusDraft
	claim.ReviewPending = false
	ReleaseApproval(store, claim.ID, ap)
	return claim
}

// DetectStale re-checks every locked claim's dependencies against store
// and flips ReviewPending to true for any locked claim whose dependency
// content has drifted since the last lock/reaudit. It never changes
// Status; a claim already flagged stays flagged until a confirmed
// reaudit clears it. Returns the (possibly) updated claims slice.
func DetectStale(claims []model.Claim, store *Store) []model.Claim {
	out := make([]model.Claim, len(claims))
	copy(out, claims)

	for i, c := range out {
		if c.Status != model.StatusLocked {
			continue
		}
		for _, dep := range BaselineDependencyIDs(c) {
			depClaim, ok := findByID(claims, dep)
			if !ok {
				continue
			}
			if stored, known := store.Baseline(c.ID, dep); known && stored != ContentHash(depClaim) {
				out[i].ReviewPending = true
				break
			}
		}
	}
	return out
}

// RefreshBaseline re-records claim's dependency content hashes in store and
// refreshes its LockedAt stamp — the "re-snapshot the dependencies I rest on
// as of now" half of a confirmed reaudit. It deliberately does NOT touch
// claim.ReviewPending: whether the claim is still review_pending AFTER a
// re-baseline is a whole-claim verdict the caller computes from every trigger
// (see internal/comments.Recompute), because a re-baseline only clears the
// DRIFT trigger — a claim can still carry an independent open-comment-thread
// trigger that a dependency re-baseline must not silently clear.
//
// Do NOT call RefreshBaseline (or ClearReviewPending) from internal/comments'
// comment ops: re-baselining dependency hashes is only ever correct after a
// human-confirmed reaudit has reviewed the drifted dependency, never as a side
// effect of resolving a comment thread (which must leave the drift baseline
// exactly as it was so genuine dependency drift stays detected).
func RefreshBaseline(claim model.Claim, claims []model.Claim, store *Store) {
	for _, dep := range BaselineDependencyIDs(claim) {
		if depClaim, ok := findByID(claims, dep); ok {
			store.recordBaseline(claim.ID, dep, ContentHash(depClaim))
			store.recordReceipt(claim.ID, depClaim)
		}
	}
	if store.LockedAt == nil {
		store.LockedAt = map[string]string{}
	}
	store.LockedAt[claim.ID] = nowFunc().UTC().Format(time.RFC3339Nano)
}

// ClearReviewPending re-baselines claim's dependency hashes (via
// RefreshBaseline) and unconditionally clears ReviewPending, returning the
// updated claim; Status remains locked throughout. It is the simple primitive
// for a caller that KNOWS a re-baseline fully clears the claim's pending state
// (no other trigger stands). The reaudit CLI no longer calls it directly — it
// uses RefreshBaseline and then recomputes ReviewPending from all three
// triggers, so a claim that still has an open comment thread stays
// review_pending even after its drifted dependency is confirmed. Like
// RefreshBaseline, it must not be called from internal/comments.
func ClearReviewPending(claim model.Claim, claims []model.Claim, store *Store) model.Claim {
	RefreshBaseline(claim, claims, store)
	claim.ReviewPending = false
	return claim
}

// LockConstitution records the roof's approval on the store (NIT-6): the
// file's content hash, the human's reason and the time. It writes nothing —
// the caller flips the file's status line and saves both, under the store
// sentinel, exactly as a claim lock does. Re-recording is legal and is the
// whole point of the edited-after-lock state: a file whose hash moved is
// re-locked by a fresh record, never by editing the old one.
func LockConstitution(store *Store, f *constitution.File, reason string, now time.Time) constitution.LockRecord {
	rec := constitution.LockRecord{
		Hash:     constitution.Hash(f),
		Reason:   reason,
		LockedAt: now.UTC().Format(time.RFC3339Nano),
	}
	store.Constitution = &rec
	return rec
}

// BaselineDependencyIDs is the dependency set whose CONTENT a locked claim is
// baselined against: its rests_on targets, each recorded once.
//
// Lock refusal walks claim.RestsOn.IDs directly (internal/lock/policy.go).
// This helper feeds a drift baseline. The retired governed_by edge was the
// one input that sat in this set and not the refusal walk (NIT-29); keeping
// the exported helper means a later drift-only edge kind is added here, not
// to the gate. RESTS ON NONE contributes nothing: a stated absence is not
// an edge and has no content to baseline (NIT-24).
//
// Exported because internal/comments and cmd/dossierx used to keep hand-copied
// duplicates of this list; they call this now, so the three cannot diverge.
func BaselineDependencyIDs(c model.Claim) []string {
	return dedupeStable(c.RestsOn.IDs)
}

// dedupeStable drops repeated ids while preserving first-seen order. It is
// required, not incidental: a claim may list X under rests_on twice, and a
// store that recorded X twice (or in a map-iteration order) would make the
// baseline table depend on which entry was walked first.
func dedupeStable(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func findByID(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}
