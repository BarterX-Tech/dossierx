// Package lock implements the claim lock lifecycle described in
// FORMAT.md:
//
//	draft -> locked            via Lock (human-initiated; refused on any
//	                           error-severity lint finding, on hub gating, OR
//	                           on an unresolved comment thread)
//	locked -> locked+pending   on ANY of three independent triggers — a
//	                           dependency's content hash drifts (DetectStale),
//	                           a "dossierx flag" records a spec mismatch, or a
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
// claim a locked claim depends on (via Mirrors or RestsOn) has changed
// underneath it. This is load-bearing: dossierx check's staleness detection
// and dossierx lock's baseline-recording both go through it, so its file
// format is considered part of the engine's on-disk contract, not an
// implementation detail — and is versioned (see storeSchemaVersion) so a
// format change like the per-dependent re-keying stays migratable.
package lock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
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
// whose words. A store at version < 2 predates the ledger, so its already-locked
// claims are grandfathered in once — see AdoptLedger for why that trigger is
// "the file exists at an older version" and never "the ledger is empty".
const (
	storeSchemaVersion      = 2
	nestedHashSchemaVersion = 1
	ledgerSchemaVersion     = 2
)

// Store is the on-disk (JSON) record of dependency content hashes as of each
// locked claim's most recent lock or confirmed reaudit.
type Store struct {
	// Version is the store's on-disk schema version (see storeSchemaVersion).
	// It is written on every Save and read on every Load so a format change
	// like the per-dependent re-keying stays migratable rather than a silent,
	// unversioned break.
	Version int `json:"version"`

	// Hashes records dependency baselines keyed PER DEPENDENT:
	// Hashes[dependentID][depID] is ContentHash(dep) as observed the last
	// time dependentID itself was locked or reaudited-and-confirmed. Keying
	// per-dependent (rather than by dependency id alone) is load-bearing: two
	// locked claims that share a dependency each keep their OWN baseline for
	// it, so locking/reauditing one never overwrites the other's baseline and
	// masks real drift the other should have flipped review_pending on.
	Hashes map[string]map[string]string `json:"hashes"`

	// LockedAt maps a locked claim's own ID -> the RFC3339Nano timestamp of
	// its most recent "dossierx lock" or confirmed "dossierx reaudit". Per
	// FORMAT.md, a confirmed reaudit refreshes this alongside the
	// dependency content hash, even when the proposal was a no-change
	// confirmation.
	LockedAt map[string]string `json:"locked_at"`

	// Ledger is the lock ledger (schema version 2 and later): one
	// LedgerRecord per locked artifact, keyed by claim id — or by
	// BuildOrderLedgerKey(module) for a locked build order. See ledger.go's
	// file doc for what it is for, and audit.go for the rules read off it.
	// It is `omitempty` so a project with nothing locked keeps a store file
	// shaped exactly as the pre-ledger one.
	Ledger map[string]LedgerRecord `json:"ledger,omitempty"`

	path string

	// diskVersion is the schema version this store was DECODED FROM, as
	// opposed to Version, which LoadStore always sets to the current version
	// so Save writes a current-shaped file. Every migration keys on
	// diskVersion: after a load, Version no longer says anything about what
	// was on disk.
	diskVersion int

	// fileExists records whether the store file was actually there. It is
	// load-bearing for grandfathering, which must distinguish "an older store
	// that never had a ledger" (adopt, once, loudly) from "no store at all
	// while locked claims exist" (never adopt — see AdoptLedger).
	fileExists bool
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
	s := &Store{
		Version:  storeSchemaVersion,
		Hashes:   map[string]map[string]string{},
		LockedAt: map[string]string{},
		path:     path,
	}

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock: read store %s: %w", path, err)
	}

	// Decode in two phases so a legacy flat "hashes" (map[depID]hash) can
	// never make json.Unmarshal fail against the new nested
	// map[dependentID]map[depID]hash shape: capture "hashes" as raw bytes
	// first, decide by schema version whether to keep or drop it, and only
	// then decode the ones we keep.
	var onDisk struct {
		Version  int                     `json:"version"`
		Hashes   json.RawMessage         `json:"hashes"`
		LockedAt map[string]string       `json:"locked_at"`
		Ledger   map[string]LedgerRecord `json:"ledger"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		return nil, fmt.Errorf("lock: parse store %s: %w", path, err)
	}
	if onDisk.LockedAt != nil {
		s.LockedAt = onDisk.LockedAt
	}
	if onDisk.Ledger != nil {
		s.Ledger = onDisk.Ledger
	}
	// Remember what was actually on disk (see Store.diskVersion/fileExists):
	// Version has already been set to the CURRENT version above, so from here
	// on it is the write format, not a fact about this file.
	s.fileExists = true
	s.diskVersion = onDisk.Version

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
// content hash of each dependency it rests on, using the exact dependencyIDs
// set DetectStale compares so baselines and staleness checks always agree. It
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
// migrate-and-Save sequence, the same as any other load-mutate-save on the
// shared store file.
func MigrateLegacyStore(s *Store, claims []model.Claim) (changed bool) {
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

	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		for _, dep := range dependencyIDs(c) {
			depClaim, ok := findByID(claims, dep)
			if !ok {
				continue
			}
			s.recordBaseline(c.ID, dep, ContentHash(depClaim))
			changed = true
		}
	}

	if changed {
		s.Version = storeSchemaVersion
	}
	return changed
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
	if err := atomicWriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("lock: write store %s: %w", s.path, err)
	}
	return nil
}

// atomicWriteFile writes data to path without ever leaving a reader able to
// observe a partially-written file: it writes to a temp file created in
// path's own directory (so the later rename stays on one filesystem, which
// is what makes it atomic) and then renames it over path.
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

// ContentHash returns a deterministic hash of the parts of a claim that
// matter for staleness detection: everything a dependent claim's
// correctness could be affected by (its comparable content), but not
// engine-managed bookkeeping like Status or ReviewPending themselves.
func ContentHash(c model.Claim) string {
	// A minimal, explicit field list rather than hashing the whole struct:
	// this keeps ReviewPending/Status changes from ever being mistaken for
	// content changes, which would create a feedback loop.
	h := sha256.New()
	fmt.Fprintf(h, "id=%s\nfacet=%s\nmodule=%s\nlayout=%s\nbody=%s\n", c.ID, c.Facet, c.Module, c.Layout, c.Body)
	for _, r := range c.Rows {
		fmt.Fprintf(h, "row=%v\n", r)
	}
	for _, s := range c.Steps {
		fmt.Fprintf(h, "step=%s\n", s)
	}
	for _, m := range c.Mirrors {
		fmt.Fprintf(h, "mirrors=%s\n", m)
	}
	for _, r := range c.RestsOn {
		fmt.Fprintf(h, "rests_on=%s\n", r)
	}
	fmt.Fprintf(h, "governed=%s/%s\n", c.Governed.Type, c.Governed.Reason)
	return hex.EncodeToString(h.Sum(nil))
}

// Lock transitions claim from draft to locked. It is refused (with a
// non-nil error) if running the full lint suite against claims produces
// any error-severity finding — matching "dossierx lint"/"dossierx check"'s own
// pass/fail semantics (see reportLintFindings in cmd/dossierx/main.go), where
// warning-severity findings (e.g. "orphan") are reported but never fail
// the command. A claim with only warning-level findings against it is
// therefore still lockable. On success it records claim's current content
// hash as the new baseline for every claim it depends on, and returns the
// updated claim (Status=locked, ReviewPending=false).
//
// Beyond the lint gate, Lock has two further, candidate-scoped refusal paths:
// hub gating (checkHubGating — a dependency in the doctrine facet must itself
// be locked first) and the comment gate (a claim carrying an unresolved
// comment thread cannot lock; the refusal names the open thread ids). The
// comment gate is the real enforcement — the comments-unresolved lint is only
// a non-blocking warning — and it reads only THIS claim's own threads, so an
// unrelated claim's open thread never blocks locking a thread-free one.
//
// The lint suite runs against claims with claim's own entry replaced by
// its about-to-be-locked (Status=locked) form, not against claim's
// still-draft entry in claims. Lints that key off a claim's own Status —
// rest-on-locked (a locked claim's rests_on targets must themselves be
// locked) and roll-up (a locked banner's module-mates must themselves be
// locked) — describe a property of the claim once it is locked; checking
// against its pre-lock draft status would let a claim that rests_on a
// still-draft dependency lock successfully, silently defeating the
// lint's entire purpose.
//
// ap is the approval this lock executes — the human's own --reason words and
// the account that ran the command — and it is a REQUIRED PARAMETER rather than
// something the caller records afterwards on purpose: a lock that writes the
// claim but forgets the ledger record is indistinguishable, from then on, from
// a hand-flipped status, and the gate would refuse the honest lock. Putting it
// in the signature makes forgetting it a compile error.
func Lock(claim model.Claim, claims []model.Claim, cfg *config.Config, store *Store, ap Approval) (model.Claim, error) {
	lintClaims := withLockedCandidate(claims, claim)
	findings := lint.RunAll(lintClaims, cfg)
	errCount := 0
	for _, f := range findings {
		if f.Severity != lint.SeverityWarning {
			errCount++
		}
	}
	if errCount > 0 {
		return claim, fmt.Errorf("lock: refused, %d error-level lint finding(s) outstanding", errCount)
	}

	if err := checkHubGating(claim, claims, cfg); err != nil {
		return claim, err
	}

	// Third refusal path (after the lint gate and hub gating): a claim cannot
	// lock while it carries an unresolved comment thread. THIS is the lock
	// gate; the comments-unresolved lint is only a non-blocking warning (a
	// project-wide error-lint would freeze all locking and take render/check
	// down with it). It is candidate-scoped — it inspects only THIS claim's own
	// threads via the pure model predicate — so an unrelated locked claim's
	// open thread never blocks locking a different, thread-free claim.
	//
	// The refusal deliberately does NOT name a command to run. Resolving a
	// thread is the human's act — it IS the approval this lock is waiting on —
	// and "comment resolve" was removed from the CLI in v0.3.0 precisely so an
	// agent cannot clear its own gate. Naming the viewer instead tells the
	// caller who has to act, which is the actual blocker.
	if open := claim.OpenThreadIDs(); len(open) > 0 {
		return claim, fmt.Errorf("lock: refused, claim %q has %d unresolved comment thread(s) %v — the human resolves them in the viewer (\"dossierx serve\"); an agent may reply but never resolve", claim.ID, len(open), open)
	}

	claim.Status = model.StatusLocked
	claim.ReviewPending = false

	for _, dep := range dependencyIDs(claim) {
		if depClaim, ok := findByID(claims, dep); ok {
			store.recordBaseline(claim.ID, dep, ContentHash(depClaim))
		}
	}
	if store.LockedAt == nil {
		store.LockedAt = map[string]string{}
	}
	store.LockedAt[claim.ID] = nowFunc().UTC().Format(time.RFC3339Nano)

	// The ledger record: what this human approved, in bytes. Recorded from the
	// already-flipped claim, which is safe in either order — LockedClaimHash
	// does not sign Status (see lockedClaimHashExcluded).
	RecordApproval(store, claim, ap)

	return claim, nil
}

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
		for _, dep := range dependencyIDs(c) {
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
	for _, dep := range dependencyIDs(claim) {
		if depClaim, ok := findByID(claims, dep); ok {
			store.recordBaseline(claim.ID, dep, ContentHash(depClaim))
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

func dependencyIDs(c model.Claim) []string {
	ids := make([]string, 0, len(c.Mirrors)+len(c.RestsOn))
	ids = append(ids, c.Mirrors...)
	ids = append(ids, c.RestsOn...)
	return ids
}

// withLockedCandidate returns a copy of claims with the entry matching
// candidate.ID replaced by candidate as it will look once locked
// (Status=locked, ReviewPending=false) — or with candidate appended if its
// id isn't present in claims at all. claims itself is never mutated.
func withLockedCandidate(claims []model.Claim, candidate model.Claim) []model.Claim {
	candidate.Status = model.StatusLocked
	candidate.ReviewPending = false

	out := make([]model.Claim, len(claims))
	copy(out, claims)
	for i, c := range out {
		if c.ID == candidate.ID {
			out[i] = candidate
			return out
		}
	}
	return append(out, candidate)
}

func findByID(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}

// checkHubGating implements: if doctrine_facet is configured, a claim
// naming that facet as a dependency cannot be locked until the hub
// (doctrine) claim itself is locked. If doctrine_facet is unset this
// check is skipped entirely, not run as a vacuous pass.
func checkHubGating(claim model.Claim, claims []model.Claim, cfg *config.Config) error {
	if cfg == nil || !cfg.HubGatingEnabled() {
		return nil
	}
	for _, dep := range dependencyIDs(claim) {
		depClaim, ok := findByID(claims, dep)
		if !ok {
			continue
		}
		if depClaim.Facet == cfg.DoctrineFacet && depClaim.Status != model.StatusLocked {
			return fmt.Errorf("lock: refused, dependency %q is in doctrine facet %q and is not yet locked", dep, cfg.DoctrineFacet)
		}
	}
	return nil
}
