package lock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
)

func buildDirConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// TestStoreSaveCreatesTheLedgerDirectory: the lock store lives at
// build/ledger/lock-store.json and its first Save from an empty project
// creates the directory (and, with it, the comment digest beside it).
func TestStoreSaveCreatesTheLedgerDirectory(t *testing.T) {
	cfg := buildDirConfig(t)
	path := cfg.LockStorePath()
	if want := filepath.Join(cfg.Dir(), "build", "ledger", "lock-store.json"); path != want {
		t.Fatalf("LockStorePath = %q, want %q", path, want)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save from an empty project: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the store was not written under build/ledger/: %v", err)
	}
}

// TestAcquireFileLockCreatesTheSentinelDirectory: the first lock on a fresh
// project must not fail on the missing build/ledger/, and a failure names
// the sentinel path itself (cmd/dossierx classifies write_conflict by it).
func TestAcquireFileLockCreatesTheSentinelDirectory(t *testing.T) {
	cfg := buildDirConfig(t)
	base := ClaimsSentinelPath(cfg)
	if want := filepath.Join(cfg.Dir(), "build", "ledger", "claims"); base != want {
		t.Fatalf("ClaimsSentinelPath = %q, want %q", base, want)
	}
	release, err := AcquireClaimsLock(cfg)
	if err != nil {
		t.Fatalf("first lock on a fresh project: %v", err)
	}
	if _, err := os.Stat(base + ".lock"); err != nil {
		t.Fatalf("sentinel not created under build/ledger/: %v", err)
	}
	release()
	if _, err := os.Stat(base + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("sentinel not released (stat err=%v)", err)
	}

	// The read-only project directory: MkdirAll is now the first failing step,
	// and its error must still contain the sentinel path.
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	ro := buildDirConfig(t)
	if err := os.Chmod(ro.Dir(), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro.Dir(), 0o755) }) //nolint:errcheck // best-effort restore
	_, err = AcquireClaimsLock(ro)
	if err == nil {
		t.Fatal("expected the sentinel to be refused on a read-only project directory")
	}
	if !strings.Contains(err.Error(), ClaimsSentinelPath(ro)+".lock") {
		t.Fatalf("the failure must name the sentinel path, got: %v", err)
	}
}
