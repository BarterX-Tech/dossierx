// flagstore.go persists the pending claim-says/now-does/reason data "dossierx
// flag" records for a locked claim, so a later "dossierx reaudit <id>" — the
// existing, unchanged confirm-before-write flow — can read it back and
// produce a real diff instead of ProposeDiff's dependency-diff stub
// placeholder.
//
// This is a second, independent trigger source for the exact same
// lifecycle transition internal/lock.DetectStale already drives
// (locked -> locked+review_pending): DetectStale flips ReviewPending when a
// dependency's content changes underneath a claim; "dossierx claim flag" flips the
// very same field when an agent asserts the claim itself is now wrong,
// based on something it just observed (e.g. its own code change) rather
// than a tracked mirrors/rests_on edge. Both these paths converge on the same
// boolean field and the same confirm-before-write "dossierx reaudit --confirm"
// gate.
//
// A THIRD trigger — an unresolved comment thread on a locked claim — flips the
// very same review_pending field but deliberately does NOT route through
// reaudit: a thread is discussion, not a proposed content edit to
// diff-and-confirm, so it is cleared by resolving/deleting the thread (via
// internal/comments), never by "reaudit --confirm" (which refuses a
// comment-only review_pending claim with exit 2 — nothing to propose). There is
// still exactly one lifecycle state (locked+review_pending) and one
// ReviewPending-shaped field, now with three independent triggers (drift, flag,
// open thread) and three matching clearers — see internal/lock's package doc and
// internal/comments.PendingTriggers/Recompute.
package reaudit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PendingFlag is one claim's outstanding "dossierx claim flag" trigger: the agent's
// own account of what the claim currently (wrongly) asserts, what is
// actually true now, and why. It stays parked in FlagStore until a
// confirmed "dossierx reaudit <id> --confirm" consumes it (see
// cmd/dossierx/main.go's newReauditCmd), at which point the CLI deletes this
// entry — a flag is a one-shot trigger, not a durable record. (AuditNotes,
// appended to the claim itself by Apply, is the durable trail a flag
// eventually leaves behind.)
type PendingFlag struct {
	ClaimSays string `json:"claim_says"`
	NowDoes   string `json:"now_does"`
	Reason    string `json:"reason"`
	FlaggedAt string `json:"flagged_at"`
}

// FlagStore is the on-disk (JSON) table of every locked claim's pending
// "dossierx claim flag" trigger, keyed by claim id. Like internal/lock.Store, it is a
// single project-wide shared file (not one per module or per claim), so
// callers writing to it are expected to serialize concurrent access via
// internal/lock.AcquireFileLock, the same way "dossierx lock"/"dossierx reaudit
// --confirm" already do for internal/lock.Store — see cmd/dossierx/flag.go and
// cmd/dossierx/main.go's newReauditCmd for that usage.
type FlagStore struct {
	// Version is the on-disk schema version, written on every Save and
	// required by DecodeFlagStore. It exists so the store can be told apart
	// from unrelated JSON at the generic name flag-store.json (internal/check
	// decodes index blobs to decide whether a commit carries dossierx
	// content); a store written before the field existed is read by
	// LoadFlagStore through a one-time lenient path DecodeFlagStore does not
	// offer, and the next Save stamps it.
	Version int                    `json:"version"`
	Flags   map[string]PendingFlag `json:"flags"`

	path string
}

// FlagStoreSchemaVersion is the only version FlagStore.Save writes and
// DecodeFlagStore accepts.
const FlagStoreSchemaVersion = 1

// LoadFlagStore reads the flag store from path. A missing file is not an
// error — it is the common, expected case for any claim or project that has
// never used "dossierx claim flag" — and is treated as an empty, freshly-initialized
// store, mirroring internal/lock.LoadStore's same "missing file is a fresh
// store, not a failure" contract.
func LoadFlagStore(path string) (*FlagStore, error) {
	s := &FlagStore{Version: FlagStoreSchemaVersion, Flags: map[string]PendingFlag{}, path: path}

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reaudit: read flag store %s: %w", path, err)
	}
	// The one-time lenient path: a store written before "version" existed
	// has no such key, and this loader accepts it (and Save then stamps it).
	// DecodeFlagStore, which answers "is this blob our store?", does not.
	if err := decodeFlagStore(s, raw, false); err != nil {
		return nil, fmt.Errorf("reaudit: parse flag store %s: %w", path, err)
	}
	s.Version = FlagStoreSchemaVersion
	s.path = path
	return s, nil
}

// DecodeFlagStore decodes a flag store from bytes already in hand — the git
// index's copy, under "check --staged" — STRICTLY: unknown keys are refused,
// "version" must be present and equal FlagStoreSchemaVersion, and the "flags"
// key must be present. `{}` and unrelated JSON are refused, which is what lets
// internal/check tell this store from any other file named flag-store.json.
// The returned store has no path and cannot be saved.
func DecodeFlagStore(raw []byte) (*FlagStore, error) {
	s := &FlagStore{Flags: map[string]PendingFlag{}}
	if err := decodeFlagStore(s, raw, true); err != nil {
		return nil, fmt.Errorf("reaudit: decode flag store: %w", err)
	}
	return s, nil
}

// decodeFlagStore is the one parser behind LoadFlagStore and DecodeFlagStore.
// strict requires the version and flags keys and refuses unknown keys; the
// lenient mode ignores an unknown key, as every release before this one did,
// so a key a later release adds does not make the older binary's check fail
// (internal/lock and internal/digest draw the same line).
func decodeFlagStore(s *FlagStore, raw []byte, strict bool) error {
	var onDisk struct {
		Version *int                    `json:"version"`
		Flags   *map[string]PendingFlag `json:"flags"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(&onDisk); err != nil {
		return err
	}
	if strict {
		if onDisk.Version == nil {
			return fmt.Errorf("no version field")
		}
		if *onDisk.Version != FlagStoreSchemaVersion {
			return fmt.Errorf("version %d is not one this engine writes", *onDisk.Version)
		}
		if onDisk.Flags == nil {
			return fmt.Errorf("no flags field")
		}
	}
	if onDisk.Version != nil {
		s.Version = *onDisk.Version
	}
	if onDisk.Flags != nil && *onDisk.Flags != nil {
		s.Flags = *onDisk.Flags
	}
	return nil
}

// Save writes the flag store back to its path as JSON, atomically (temp
// file in the same directory, then rename), so a concurrent reader outside
// AcquireFileLock's critical section never observes a partially-written
// file — the same reasoning as internal/lock.Store.Save.
func (s *FlagStore) Save() error {
	s.Version = FlagStoreSchemaVersion
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("reaudit: marshal flag store: %w", err)
	}
	// The store lives under build/ledger/, which a fresh project does not
	// have until the first write creates it.
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("reaudit: create flag store dir for %s: %w", s.path, err)
	}
	if err := atomicWriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("reaudit: write flag store %s: %w", s.path, err)
	}
	return nil
}

// atomicWriteFile writes data to path without ever leaving a reader able to
// observe a partially-written file: it writes to a temp file created in
// path's own directory (so the later rename stays on one filesystem, which
// is what makes it atomic) and then renames it over path. Duplicated here
// rather than imported from internal/lock (which has its own copy),
// mirroring internal/buildorder/store.go's same "keep this package's
// dependency footprint limited" precedent for the identical helper.
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
