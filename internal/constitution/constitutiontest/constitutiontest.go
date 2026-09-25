// Package constitutiontest arms a fixture project's constitution the way
// `dossierx constitution lock` does on a fresh project, so check and serve
// tests share one definition of an "armed" fixture instead of each keeping a
// copy that drifts.
package constitutiontest

import (
	"fmt"
	"os"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// Roof is the one-entry constitution written when the fixture has none.
const Roof = "status: locked\ninvariants:\n  - slug: one-roof\n    title: One roof\n    body: This fixture has one lockable constitution above every module.\n"

// Arm writes Roof if cfg's project has no constitution, marks a draft roof
// locked, records the constitution lock in the lock store, and on a fresh
// project takes the comment threads already on disk into digest coverage, as
// the real command's first ledger write does. A nil cfg (a fixture whose
// config is refused on purpose) has no roof to lock and is a no-op.
func Arm(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	path := cfg.ConstitutionPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(Roof), 0o644); err != nil {
			return fmt.Errorf("arm constitution: write: %w", err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("arm constitution: read: %w", err)
	}
	locked, err := constitution.LockedBytes(raw, path)
	if err != nil {
		return fmt.Errorf("arm constitution: lock: %w", err)
	}
	if err := os.WriteFile(path, locked, 0o644); err != nil {
		return fmt.Errorf("arm constitution: rewrite: %w", err)
	}
	f, err := constitution.Parse(locked, path)
	if err != nil {
		return fmt.Errorf("arm constitution: parse: %w", err)
	}
	store, err := lock.LoadStore(cfg.LockStorePath())
	if err != nil {
		return fmt.Errorf("arm constitution: load store: %w", err)
	}
	lock.LockConstitution(store, f, "fixture roof", time.Now())
	if !store.LedgerCovered() && !store.PreLedger() {
		if claims, loadErr := loader.LoadAll(cfg); loadErr == nil {
			lock.SweepCommentDigests(store, claims, false)
		}
	}
	if err := store.Save(); err != nil {
		return fmt.Errorf("arm constitution: save store: %w", err)
	}
	return nil
}
