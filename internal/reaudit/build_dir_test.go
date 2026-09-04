package reaudit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFlagStoreSaveCreatesTheLedgerDirectory: the flag store's first Save from
// an empty project creates build/ledger/ and stamps the version.
func TestFlagStoreSaveCreatesTheLedgerDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build", "ledger", "flag-store.json")
	s, err := LoadFlagStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Flags["widget.contract.one"] = PendingFlag{ClaimSays: "a", NowDoes: "b", Reason: "c", FlaggedAt: "2026-01-01T00:00:00Z"}
	if err := s.Save(); err != nil {
		t.Fatalf("Save from an empty project: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the store was not written under build/ledger/: %v", err)
	}
	if _, err := DecodeFlagStore(raw); err != nil {
		t.Fatalf("what Save wrote must decode strictly: %v\n%s", err, raw)
	}
}

// TestDecodeFlagStoreIsStrictAndLoadIsLenientOnce: DecodeFlagStore refuses
// {}, unrelated JSON and a store without version/flags; LoadFlagStore still
// reads a pre-version store and the next Save stamps it.
func TestDecodeFlagStoreIsStrictAndLoadIsLenientOnce(t *testing.T) {
	for _, bad := range []string{`{}`, `{"unrelated": true}`, `{"flags":{}}`, `{"version":1}`, `{"version":2,"flags":{}}`} {
		if _, err := DecodeFlagStore([]byte(bad)); err == nil {
			t.Fatalf("DecodeFlagStore(%s) must refuse", bad)
		}
	}
	if _, err := DecodeFlagStore([]byte(`{"version":1,"flags":{}}`)); err != nil {
		t.Fatalf("a store this engine writes must decode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "flag-store.json")
	if err := os.WriteFile(path, []byte(`{"flags":{"x":{"claim_says":"a","now_does":"b","reason":"c","flagged_at":"t"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadFlagStore(path)
	if err != nil {
		t.Fatalf("LoadFlagStore must read a pre-version store: %v", err)
	}
	if _, ok := s.Flags["x"]; !ok {
		t.Fatal("the pre-version store's flags were dropped")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeFlagStore(raw); err != nil {
		t.Fatalf("after Save the store must carry the version: %v\n%s", err, raw)
	}
}
