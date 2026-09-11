package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func lifecycleClaim(id, embodiment string) string {
	return fmt.Sprintf("id: %s\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: lifecycle fixture\ngoverned_by:\n  type: none\n  reason: fixture\n%s", id, embodiment)
}

func loadLifecycleClaim(t *testing.T, cfgPath, id string) model.Claim {
	t.Helper()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatal(err)
	}
	claim, ok := loader.FindByID(claims, id)
	if !ok {
		t.Fatalf("claim %s not found", id)
	}
	return claim
}

func lifecycleRecord(t *testing.T, cfgPath, id string) lock.LedgerRecord {
	t.Helper()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	store, err := lock.LoadStore(cfg.LockStorePath())
	if err != nil {
		t.Fatal(err)
	}
	record, ok := store.Record(id)
	if !ok || record.Released() {
		t.Fatalf("record for %s = %+v, want standing", id, record)
	}
	return record
}

func writeLifecycleClaim(t *testing.T, root, name, id, embodiment string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "claims", name+".yaml"), []byte(lifecycleClaim(id, embodiment)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lockLifecycle(t *testing.T, cfgPath string, ids ...string) {
	t.Helper()
	args := []string{"--config", cfgPath, "claim", "lock"}
	args = append(args, ids...)
	args = append(args, "--reason", "reviewed lifecycle transition")
	env, stderr, err := execReviewedCLIJSON(t, args...)
	if err != nil || !env.OK {
		t.Fatalf("lock %v: env=%+v stderr=%s err=%v", ids, env, stderr, err)
	}
}

func unlockLifecycle(t *testing.T, cfgPath, id string) {
	t.Helper()
	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "claim", "unlock", id, "--reason", "edit embodiment fixture")
	if err != nil || !env.OK {
		t.Fatalf("unlock %s: env=%+v stderr=%s err=%v", id, env, stderr, err)
	}
}

func TestConformanceRealSingletonAndBatchLockLifecycle(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	cfgText := "schema_version: 1\nfacets: [contract]\nmodules: [widget]\nclaims_dir: claims\nconformance:\n  observations: observations.json\n"
	if err := os.WriteFile(cfgPath, []byte(cfgText), 0o644); err != nil {
		t.Fatal(err)
	}
	ids := []string{"widget.contract.one", "widget.contract.two", "widget.contract.three"}
	for i, id := range ids {
		writeLifecycleClaim(t, root, []string{"one", "two", "three"}[i], id, "")
	}
	observations := `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://a","shape":"set","value":["ready"]},{"adapter":"neutral/v1","target":"widget://b","shape":"set","value":["ready"]},{"adapter":"neutral/v1","target":"widget://two","shape":"set","value":["ready"]}]}`
	if err := os.WriteFile(filepath.Join(root, "observations.json"), []byte(observations), 0o644); err != nil {
		t.Fatal(err)
	}

	lockLifecycle(t, cfgPath, ids[0])
	omittedHash := lifecycleRecord(t, cfgPath, ids[0]).Hash
	lockLifecycle(t, cfgPath, ids[1], ids[2])
	for _, id := range ids {
		_ = lifecycleRecord(t, cfgPath, id)
	}

	compareA := "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://a\n      expectation:\n        shape: set\n        value: [ready]\n"
	unlockLifecycle(t, cfgPath, ids[0])
	writeLifecycleClaim(t, root, "one", ids[0], compareA)
	lockLifecycle(t, cfgPath, ids[0])
	claimA := loadLifecycleClaim(t, cfgPath, ids[0])
	recordA := lifecycleRecord(t, cfgPath, ids[0])
	if recordA.Hash == omittedHash || recordA.Hash != lock.LockedClaimHash(claimA) {
		t.Fatalf("adding embodiment was not signed: omitted=%s added=%s", omittedHash, recordA.Hash)
	}

	compareB := "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://b\n      expectation:\n        shape: set\n        value: [ready]\n"
	unlockLifecycle(t, cfgPath, ids[0])
	writeLifecycleClaim(t, root, "one", ids[0], compareB)
	lockLifecycle(t, cfgPath, ids[0])
	claimB := loadLifecycleClaim(t, cfgPath, ids[0])
	recordB := lifecycleRecord(t, cfgPath, ids[0])
	if recordB.Hash == recordA.Hash || lock.ContentHash(claimB) != lock.ContentHash(claimA) {
		t.Fatal("address edit must move only the exact locked hash")
	}

	compareContent := "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://b\n      expectation:\n        shape: set\n        value: [waiting]\n"
	unlockLifecycle(t, cfgPath, ids[0])
	writeLifecycleClaim(t, root, "one", ids[0], compareContent)
	lockLifecycle(t, cfgPath, ids[0])
	claimContent := loadLifecycleClaim(t, cfgPath, ids[0])
	if lifecycleRecord(t, cfgPath, ids[0]).Hash == recordB.Hash || lock.ContentHash(claimContent) == lock.ContentHash(claimB) {
		t.Fatal("expected-member edit must move both hashes")
	}

	unlockLifecycle(t, cfgPath, ids[0])
	writeLifecycleClaim(t, root, "one", ids[0], "")
	lockLifecycle(t, cfgPath, ids[0])
	if got := lifecycleRecord(t, cfgPath, ids[0]).Hash; got != omittedHash {
		t.Fatalf("removing embodiment did not restore omitted locked hash: got %s want %s", got, omittedHash)
	}

	// Exercise the same add/edit/remove transitions through the real grouped
	// lock path. Unlock remains deliberately singleton in the public surface.
	for _, id := range ids[1:] {
		unlockLifecycle(t, cfgPath, id)
	}
	writeLifecycleClaim(t, root, "two", ids[1], "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://two\n      expectation:\n        shape: set\n        value: [ready]\n")
	writeLifecycleClaim(t, root, "three", ids[2], "embodiment:\n  mode: none\n  reason: generated documentation only\n")
	lockLifecycle(t, cfgPath, ids[1], ids[2])
	batchAdded := []string{lifecycleRecord(t, cfgPath, ids[1]).Hash, lifecycleRecord(t, cfgPath, ids[2]).Hash}
	for _, id := range ids[1:] {
		unlockLifecycle(t, cfgPath, id)
	}
	writeLifecycleClaim(t, root, "two", ids[1], "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://two\n      expectation:\n        shape: set\n        value: [waiting]\n")
	writeLifecycleClaim(t, root, "three", ids[2], "")
	lockLifecycle(t, cfgPath, ids[1], ids[2])
	if lifecycleRecord(t, cfgPath, ids[1]).Hash == batchAdded[0] || lifecycleRecord(t, cfgPath, ids[2]).Hash == batchAdded[1] {
		t.Fatal("batch content edit/removal did not refresh exact approvals")
	}

	// Observation-only state changes cannot move the ledger or the existing
	// readiness axis after either singleton or batch approval paths.
	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("matched check: env=%+v stderr=%s err=%v", env, stderr, err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	ledgerBefore, err := os.ReadFile(cfg.LockStorePath())
	if err != nil {
		t.Fatal(err)
	}
	catalogBefore, err := os.ReadFile(cfg.CatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "observations.json"), []byte(`{"format_version":1,"observations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	env, stderr, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("owed check: env=%+v stderr=%s err=%v", env, stderr, err)
	}
	ledgerAfter, err := os.ReadFile(cfg.LockStorePath())
	if err != nil {
		t.Fatal(err)
	}
	catalogAfter, err := os.ReadFile(cfg.CatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ledgerBefore, ledgerAfter) {
		t.Fatal("observation change mutated the real lock ledger")
	}
	if !reflect.DeepEqual(catalogReadinessByID(t, catalogBefore), catalogReadinessByID(t, catalogAfter)) {
		t.Fatal("observation change mutated existing readiness")
	}
	for _, id := range ids {
		_ = lifecycleRecord(t, cfgPath, id)
	}
}

func catalogReadinessByID(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var doc struct {
		Claims []struct {
			ID        string          `json:"id"`
			Readiness json.RawMessage `json:"readiness"`
		} `json:"claims"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]json.RawMessage, len(doc.Claims))
	for _, claim := range doc.Claims {
		out[claim.ID] = claim.Readiness
	}
	return out
}
