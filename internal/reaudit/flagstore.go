// flagstore.go persists the pending claim-says/now-does/reason data "dossierx
// flag" records for a locked claim, so a later "dossierx reaudit <id>" — the
// existing, unchanged confirm-before-write flow — can read it back and
// produce a real diff instead of ProposeDiff's dependency-diff stub
// placeholder.
//
// This is a second, independent trigger source for the exact same
// lifecycle transition internal/lock.DetectStale already drives
// (locked -> locked+review_pending): DetectStale flips ReviewPending when a
// dependency's content changes underneath a claim; "dossierx flag" flips the
// very same field when an agent asserts the claim itself is now wrong,
// based on something it just observed (e.g. its own code change) rather
// than a tracked mirrors/rests_on edge. Both paths converge on the same
// boolean field and the same confirm-before-write "dossierx reaudit --confirm"
// gate — there is deliberately no second lifecycle state, and no second
// ReviewPending-shaped flag.
package reaudit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PendingFlag is one claim's outstanding "dossierx flag" trigger: the agent's
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
// "dossierx flag" trigger, keyed by claim id. Like internal/lock.Store, it is a
// single project-wide shared file (not one per module or per claim), so
// callers writing to it are expected to serialize concurrent access via
// internal/lock.AcquireFileLock, the same way "dossierx lock"/"dossierx reaudit
// --confirm" already do for internal/lock.Store — see cmd/dossierx/flag.go and
// cmd/dossierx/main.go's newReauditCmd for that usage.
type FlagStore struct {
	Flags map[string]PendingFlag `json:"flags"`

	path string
}

// LoadFlagStore reads the flag store from path. A missing file is not an
// error — it is the common, expected case for any claim or project that has
// never used "dossierx flag" — and is treated as an empty, freshly-initialized
// store, mirroring internal/lock.LoadStore's same "missing file is a fresh
// store, not a failure" contract.
func LoadFlagStore(path string) (*FlagStore, error) {
	s := &FlagStore{Flags: map[string]PendingFlag{}, path: path}

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reaudit: read flag store %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("reaudit: parse flag store %s: %w", path, err)
	}
	if s.Flags == nil {
		s.Flags = map[string]PendingFlag{}
	}
	s.path = path
	return s, nil
}

// Save writes the flag store back to its path as JSON, atomically (temp
// file in the same directory, then rename), so a concurrent reader outside
// AcquireFileLock's critical section never observes a partially-written
// file — the same reasoning as internal/lock.Store.Save.
func (s *FlagStore) Save() error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("reaudit: marshal flag store: %w", err)
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
