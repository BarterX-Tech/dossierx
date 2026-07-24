// Package lock implements the claim lock lifecycle described in
// FORMAT.md:
//
//	draft -> locked            via Lock (human-initiated, refused on lint failure)
//	locked -> locked+pending   via DetectStale, when a dependency's content hash changes
//	locked+pending -> locked   only via a confirmed internal/reaudit apply (ClearReviewPending)
//
// A locked claim's Status never reverts to draft on its own; review_pending
// is the only automatic transition, and it is one-directional until a
// human confirms a reaudit.
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

// storeSchemaVersion is the on-disk schema version of the lock hash store.
//
// Version 1 introduced PER-DEPENDENT hash baselines: Hashes became
// map[dependentID]map[depID]hash, so each locked claim records its own
// snapshot of every dependency it rests on. A store file carrying no
// "version" field (Version == 0 after decode) predates that change and holds
// the legacy flat map[depID]hash; LoadStore migrates it — see LoadStore.
const storeSchemaVersion = 1

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

	path string
}

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
		Version  int               `json:"version"`
		Hashes   json.RawMessage   `json:"hashes"`
		LockedAt map[string]string `json:"locked_at"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		return nil, fmt.Errorf("lock: parse store %s: %w", path, err)
	}
	if onDisk.LockedAt != nil {
		s.LockedAt = onDisk.LockedAt
	}

	// Legacy (pre-versioning) store: drop its flat hashes, keep LockedAt, and
	// present it to callers as an already-migrated current-version store.
	if onDisk.Version < storeSchemaVersion {
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
// The lint suite runs against claims with claim's own entry replaced by
// its about-to-be-locked (Status=locked) form, not against claim's
// still-draft entry in claims. Lints that key off a claim's own Status —
// rest-on-locked (a locked claim's rests_on targets must themselves be
// locked) and roll-up (a locked banner's module-mates must themselves be
// locked) — describe a property of the claim once it is locked; checking
// against its pre-lock draft status would let a claim that rests_on a
// still-draft dependency lock successfully, silently defeating the
// lint's entire purpose.
func Lock(claim model.Claim, claims []model.Claim, cfg *config.Config, store *Store) (model.Claim, error) {
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

	return claim, nil
}

// Unlock transitions claim back to draft. This is always human-initiated
// and always allowed (no lint gate) — a project may need to unlock a
// locked claim precisely to fix the thing lint is complaining about.
func Unlock(claim model.Claim) model.Claim {
	claim.Status = model.StatusDraft
	claim.ReviewPending = false
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

// ClearReviewPending is called only by a confirmed internal/reaudit apply.
// It refreshes claim's dependency hashes in store and clears
// ReviewPending; Status remains locked throughout.
func ClearReviewPending(claim model.Claim, claims []model.Claim, store *Store) model.Claim {
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
