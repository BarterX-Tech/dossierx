// This file covers edge cases beyond the six audited categories that a
// thorough engineer would still expect the engine to handle correctly,
// found while auditing the existing test suite for completeness:
//
//  1. concurrent "docs lock" invocations on different claims, sharing one
//     project-wide lock store, must not lose each other's store updates
//     (a real lost-update race found and fixed in internal/lock; see
//     internal/lock/filelock.go and filelock_test.go for the unit-level
//     proof).
//  2. a very long claim id (well beyond any "normal" length) must not be
//     truncated, mis-hashed, or otherwise mishandled anywhere in the
//     lint/catalog/render/lock pipeline.
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------
// Concurrent "docs lock" invocations must not lose store updates.
// ---------------------------------------------------------------------

func TestConcurrentLocksDoNotLoseStoreUpdates(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - concmod\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	const n = 6
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id := "concmod.contract.c" + strconv.Itoa(i)
		ids[i] = id
		claim := "id: " + id + "\n" +
			"facet: contract\nmodule: concmod\nstatus: draft\nlayout: card\n" +
			"body: |\n  concurrently-locked claim number " + strconv.Itoa(i) + ".\n" +
			"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
		if err := os.WriteFile(filepath.Join(claimsDir, "c"+strconv.Itoa(i)+".yaml"), []byte(claim), 0o644); err != nil {
			t.Fatalf("write claim %s: %v", id, err)
		}
	}

	var wg sync.WaitGroup
	codes := make([]int, n)
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, stderr, code := run(t, root, "lock", id)
			codes[i] = code
			if code != 0 {
				t.Errorf("concurrent lock of %s failed: exit %d, stderr: %s", id, code, stderr)
			}
		}(i, id)
	}
	wg.Wait()

	// Every claim file must independently say status: locked (each claim
	// has its own file, so this part was never at risk — the shared
	// resource under test is the store file, checked below).
	for i, id := range ids {
		raw, err := os.ReadFile(filepath.Join(claimsDir, "c"+strconv.Itoa(i)+".yaml"))
		if err != nil {
			t.Fatalf("read claim %s: %v", id, err)
		}
		if !strings.Contains(string(raw), "status: locked") {
			t.Errorf("expected %s to be locked on disk, got:\n%s", id, raw)
		}
	}

	// The shared lock store's LockedAt map must carry an entry for every
	// one of the n concurrently-locked claims — this is exactly what a
	// load-modify-save race would silently drop (whichever store.Save()
	// happened last would win, discarding every earlier concurrent
	// writer's LockedAt entry).
	storeRaw, err := os.ReadFile(filepath.Join(root, ".docs-lock-store.json"))
	if err != nil {
		t.Fatalf("read lock store: %v", err)
	}
	var store struct {
		LockedAt map[string]string `json:"locked_at"`
	}
	if err := json.Unmarshal(storeRaw, &store); err != nil {
		t.Fatalf("parse lock store: %v\nraw: %s", err, storeRaw)
	}
	for _, id := range ids {
		if _, ok := store.LockedAt[id]; !ok {
			t.Errorf("expected lock store's locked_at to carry an entry for %s, but it was lost (store: %+v)", id, store.LockedAt)
		}
	}
}

// ---------------------------------------------------------------------
// A very long claim id must not be truncated, mis-hashed, or otherwise
// mishandled anywhere in the lint/catalog/render/lock/deps pipeline.
// ---------------------------------------------------------------------

func TestVeryLongClaimIDHandledEndToEnd(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - longidmod\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	// A single kebab-case slug segment ~500 characters long: still valid
	// per id-shape's grammar (no length cap is spec'd), just unusually
	// long.
	var slugBuilder strings.Builder
	for i := 0; i < 100; i++ {
		if i > 0 {
			slugBuilder.WriteByte('-')
		}
		slugBuilder.WriteString("seg" + strconv.Itoa(i))
	}
	slug := slugBuilder.String()
	id := "longidmod.contract." + slug
	if len(id) < 400 {
		t.Fatalf("test setup bug: generated id is only %d chars, expected several hundred", len(id))
	}

	claim := "id: " + id + "\n" +
		"facet: contract\nmodule: longidmod\nstatus: draft\nlayout: card\n" +
		"body: |\n  claim with a very long id.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "long.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	if stdout, stderr, code := run(t, root, "check"); code != 0 {
		t.Fatalf("check failed for a very long claim id: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// deps must find it by its full, exact (untruncated) id.
	depsOut, depsErr, depsCode := run(t, root, "deps", id)
	if depsCode != 0 {
		t.Fatalf("deps failed for the long id: exit %d\nstdout: %s\nstderr: %s", depsCode, depsOut, depsErr)
	}
	if !strings.Contains(depsOut, id) {
		t.Fatalf("expected deps output to echo the full long id, got: %s", depsOut)
	}

	// The catalog must carry the claim under its full id, not a truncated
	// or hashed form.
	catalogRaw, err := os.ReadFile(filepath.Join(root, ".catalog.json"))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	if !strings.Contains(string(catalogRaw), id) {
		t.Fatalf("expected .catalog.json to contain the full long id, got a file that does not:\n(len=%d)", len(catalogRaw))
	}

	// The rendered viewer must also carry the full id, and lock must
	// succeed against it (round-tripping SaveClaim/LoadClaims/ContentHash
	// on a very long id, all keyed by exact string equality).
	viewerRaw, err := os.ReadFile(filepath.Join(root, "viewer", "index.html"))
	if err != nil {
		t.Fatalf("read rendered viewer: %v", err)
	}
	if !strings.Contains(string(viewerRaw), id) {
		t.Fatalf("expected rendered viewer to contain the full long id")
	}

	if stdout, stderr, code := run(t, root, "lock", id); code != 0 {
		t.Fatalf("lock failed for the long id: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	raw, err := os.ReadFile(filepath.Join(claimsDir, "long.yaml"))
	if err != nil {
		t.Fatalf("read claim after lock: %v", err)
	}
	if !strings.Contains(string(raw), "status: locked") {
		t.Fatalf("expected the long-id claim to be locked on disk, got:\n%s", raw)
	}
}
