package lock

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// v0721Store is a lock store in the shape v0.7.21 writes: schema 3, a claim
// ledger record with its approved content, a dependency baseline and receipt,
// and the constitution's record — no briefs map.
const v0721Store = `{
  "version": 3,
  "policy_version": 1,
  "hashes": {"w.contract.b": {"w.contract.a": "hash-a"}},
  "receipts": {"w.contract.b": {"w.contract.a": {"hash": "hash-a", "content": {"ID": "w.contract.a", "Summary": "A."}}}},
  "locked_at": {"w.contract.b": "2026-09-22T00:00:00Z"},
  "ledger": {
    "w.contract.b": {"subject": "claim", "hash": "h-b", "at": "2026-09-22T00:00:00Z", "actor": "a", "reason": "r",
      "content": {"ID": "w.contract.b", "Summary": "B."}}
  },
  "constitution": {"hash": "roof", "reason": "roof", "locked_at": "2026-09-22T00:00:00Z"}
}`

// TestAV0721StoreLoadsAndEarnsTheBriefsSchemaOnlyWhenABriefIsRecorded pins the
// version-3 read path and the schema-4 bump (NIT-205). A v0.7.21 store decodes
// strictly and leniently with every claim record intact; an ordinary save of
// it stays version 3 (a project that never locks a brief keeps its store as
// it was); recording a brief moves it to 4 and changes nothing claim-side.
//
// Regression it catches: a bump that stamped 4 on every save (every claim-side
// store and test that reads "version": 3 would move for a project with no
// briefs), or a v3 decode that dropped the ledger.
func TestAV0721StoreLoadsAndEarnsTheBriefsSchemaOnlyWhenABriefIsRecorded(t *testing.T) {
	if _, err := DecodeStore([]byte(v0721Store)); err != nil {
		t.Fatalf("DecodeStore on a v0.7.21 store: %v", err)
	}
	path := filepath.Join(t.TempDir(), "lock-store.json")
	if err := os.WriteFile(path, []byte(v0721Store), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore on a v0.7.21 store: %v", err)
	}
	before := claimSide(t, s)
	if s.Version != 3 || s.Briefs != nil {
		t.Fatalf("a v0.7.21 store must load at version 3 with no briefs, got version %d briefs %v", s.Version, s.Briefs)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"version": 3`) || strings.Contains(string(raw), `"briefs"`) {
		t.Fatalf("a save that records no brief must keep the store at version 3 with no briefs key:\n%s", raw)
	}

	RecordBriefApproval(s, "flow.overview", BriefRecord{Path: "briefs/flow/overview.md", Hash: "bh", Baselines: map[string]string{"w.contract.a": "hash-a"}})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.OnDiskVersion() != briefsSchemaVersion {
		t.Fatalf("recording a brief must earn version %d, got %d", briefsSchemaVersion, reloaded.OnDiskVersion())
	}
	if got := claimSide(t, reloaded); got != before {
		t.Fatalf("recording a brief changed the claim side of the store:\nbefore %s\nafter  %s", before, got)
	}
	if raw, err = os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeStore(raw); err != nil {
		t.Fatalf("the version-4 store this engine wrote must decode strictly: %v", err)
	}
	if rec, ok := reloaded.BriefRecordFor("flow.overview"); !ok || rec.Hash != "bh" {
		t.Fatalf("the brief record did not survive the round trip: %+v %v", rec, ok)
	}
}

// claimSide is every claim-side field of a store, serialized, for comparison.
func claimSide(t *testing.T, s *Store) string {
	t.Helper()
	raw, err := json.Marshal(struct {
		H any
		R any
		L any
		G any
		C any
	}{s.Hashes, s.Receipts, s.LockedAt, s.Ledger, s.Constitution})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestDecodeStoreRefusesABriefsMapBelowVersionFour pins the strict decoder's
// half: a briefs map exists only at version 4, so a version-3 store carrying
// one was not written by this engine, and an unknown version is refused.
func TestDecodeStoreRefusesABriefsMapBelowVersionFour(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		ok   bool
	}{
		{"v3 without briefs", `{"version":3}`, true},
		{"v4 with briefs", `{"version":4,"briefs":{"a.b":{"hash":"h"}}}`, true},
		{"v3 with briefs", `{"version":3,"briefs":{"a.b":{"hash":"h"}}}`, false},
		{"v5", `{"version":5}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeStore([]byte(tc.raw))
			if (err == nil) != tc.ok {
				t.Fatalf("DecodeStore(%s) error = %v, want ok=%v", tc.raw, err, tc.ok)
			}
		})
	}
}

// TestBriefRecordsNeverReachAClaimRule is the approval contract at the store:
// a brief record — standing, released, or naming a claim as a baseline — moves
// no claim hash, no claim baseline and no Audit finding. Regression: a brief
// record routed into Ledger (its natural-looking home) would make Audit report
// the brief id as an abandoned claim record.
func TestBriefRecordsNeverReachAClaimRule(t *testing.T) {
	claim := model.Claim{ID: "w.contract.a", Module: "w", Facet: "contract", Status: model.StatusDraft, Summary: "A.", Body: "a"}
	s := emptyStore(filepath.Join(t.TempDir(), "lock-store.json"))
	hashBefore, lockedBefore := ContentHash(claim), LockedClaimHash(claim)
	auditBefore := Audit([]model.Claim{claim}, s, nil)

	RecordBriefApproval(s, "flow.overview", BriefRecord{Hash: "bh", Baselines: map[string]string{claim.ID: "stale"}, Receipts: map[string]model.Claim{claim.ID: claim}})
	RecordBriefApproval(s, "flow.other", BriefRecord{Hash: "bh2"})
	ReleaseBriefApproval(s, "flow.other", Approval{Actor: "a", Reason: "r"})

	if ContentHash(claim) != hashBefore || LockedClaimHash(claim) != lockedBefore {
		t.Fatal("a brief record moved a claim hash")
	}
	if len(s.Hashes) != 0 || len(s.Ledger) != 0 || len(s.LockedAt) != 0 {
		t.Fatalf("a brief record reached the claim baselines or ledger: %+v %+v %+v", s.Hashes, s.Ledger, s.LockedAt)
	}
	if got := Audit([]model.Claim{claim}, s, nil); !reflect.DeepEqual(got, auditBefore) {
		t.Fatalf("a brief record changed the claim audit: %+v, was %+v", got, auditBefore)
	}
}

// TestAStoreFromANewerBinaryIsRefusedNotRead pins F5 and F14 on both readers:
// a store whose version is above every version this binary knows is refused
// with ErrStoreTooNew — by LoadStore as well as DecodeStore, since reading it
// leniently and saving is how an older binary drops what it does not know —
// and a version-3 store carrying a briefs map is refused by both readers alike.
func TestAStoreFromANewerBinaryIsRefusedNotRead(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, raw string
		tooNew    bool
	}{
		{"version 9", `{"version":9,"future_key":{}}`, true},
		{"version 5", `{"version":5}`, true},
		{"version 3 with briefs", `{"version":3,"briefs":{"a.b":{"hash":"h"}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".json")
			if err := os.WriteFile(path, []byte(tc.raw), 0o644); err != nil {
				t.Fatal(err)
			}
			_, lenient := LoadStore(path)
			_, strict := DecodeStore([]byte(tc.raw))
			if lenient == nil || strict == nil {
				t.Fatalf("both readers must refuse: LoadStore %v, DecodeStore %v", lenient, strict)
			}
			if tc.tooNew != errors.Is(lenient, ErrStoreTooNew) || tc.tooNew != errors.Is(strict, ErrStoreTooNew) {
				t.Fatalf("ErrStoreTooNew = %v / %v, want %v", errors.Is(lenient, ErrStoreTooNew), errors.Is(strict, ErrStoreTooNew), tc.tooNew)
			}
		})
	}
}
