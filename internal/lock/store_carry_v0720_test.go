package lock

import (
	"reflect"
	"testing"
)

// v0.7.20 embedded approved claims with rests_on as a plain list. A store
// written by that release must still decode, or every approval in an
// upgraded project vanishes and every write refuses.
func TestDecodeStoreReadsV0720RestsOnLists(t *testing.T) {
	raw := []byte(`{
  "version": 3,
  "policy_version": 1,
  "receipts": {"w.contract.c": {"w.contract.b": {"hash": "h1",
    "content": {"ID": "w.contract.b", "RestsOn": ["w.contract.a"], "GovernedBy": {"Type": "none"}}}}},
  "ledger": {
    "w.contract.b": {"subject": "claim", "hash": "h2", "at": "2026-09-22T00:00:00Z", "actor": "a", "reason": "r",
      "content": {"ID": "w.contract.b", "RestsOn": ["w.contract.a"]}},
    "w.contract.a": {"subject": "claim", "hash": "h3", "at": "2026-09-22T00:00:00Z", "actor": "a", "reason": "r",
      "content": {"ID": "w.contract.a", "RestsOn": null}}
  }
}`)
	s, err := DecodeStore(raw)
	if err != nil {
		t.Fatalf("DecodeStore on a v0.7.20 store: %v", err)
	}
	want := []string{"w.contract.a"}
	if got := s.Ledger["w.contract.b"].Content.RestsOn.IDs; !reflect.DeepEqual(got, want) {
		t.Fatalf("ledger content rests_on = %v, want %v", got, want)
	}
	if got := s.Receipts["w.contract.c"]["w.contract.b"].Content.RestsOn.IDs; !reflect.DeepEqual(got, want) {
		t.Fatalf("receipt content rests_on = %v, want %v", got, want)
	}
	if !s.Ledger["w.contract.a"].Content.RestsOn.IsZero() {
		t.Fatalf("null rests_on should decode as undeclared, got %+v", s.Ledger["w.contract.a"].Content.RestsOn)
	}
}
