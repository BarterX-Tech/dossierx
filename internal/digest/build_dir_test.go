package digest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
)

// TestStoreSaveCreatesTheLedgerDirectory: the digest store lives at
// build/ledger/comment-digest.json and its first Save from an empty project
// creates the directory.
func TestStoreSaveCreatesTheLedgerDirectory(t *testing.T) {
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
	path := StorePath(cfg)
	if want := filepath.Join(dir, "build", "ledger", "comment-digest.json"); path != want {
		t.Fatalf("StorePath = %q, want %q", path, want)
	}
	s, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save from an empty project: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the store was not written under build/ledger/: %v", err)
	}
}

// TestDecodeStoreIsStrict: DecodeStore refuses {} and unrelated JSON, which is
// what lets check --staged tell this store from any other comment-digest.json.
func TestDecodeStoreIsStrict(t *testing.T) {
	for _, bad := range []string{`{}`, `{"unrelated": true}`, `{"version": 7, "digests": {}}`, `not json`} {
		if _, err := DecodeStore([]byte(bad)); err == nil {
			t.Fatalf("DecodeStore(%s) must refuse", bad)
		}
	}
	good := `{"version":1,"digests":{"a":"b"}}`
	s, err := DecodeStore([]byte(good))
	if err != nil || s.Digests["a"] != "b" {
		t.Fatalf("DecodeStore(%s) = %+v, %v", good, s, err)
	}
}
