// Package tests: Phase 0 claim-file write discipline, end to end via the built
// "dossierx" binary (reusing cli_test.go's TestMain/run/binPath and
// lock_lifecycle_test.go's llReadFile, all in this same package).
//
// Phase 0 introduces one project-wide "claims sentinel" —
// lock.AcquireFileLock(cfg.Dir()/.dossierx-claims), whose real lock file is
// cfg.Dir()/.dossierx-claims.lock — that EVERY claim-file writer
// (lock/unlock/check/flag/reaudit) must take, FIRST, before its own lock-store
// or flag-store sentinel, then re-read claims inside. loader.SaveClaim rewrites
// a claim's whole file, so without this a writer holding a pre-mutation
// snapshot would silently erase a concurrent writer's change.
//
// The genuinely dangerous race is cross-process TOCTOU that the -race detector
// cannot observe (it lives in separate processes, not shared memory), so these
// tests assert on real on-disk outcomes: (1) each writer actually blocks on the
// claims sentinel (hold it, prove no progress, release, prove completion — the
// hold is brief so completion lands well within AcquireFileLock's 10s timeout);
// (2) a storm of concurrent writers neither deadlocks under the new global
// acquisition order (claims -> lock-store -> flag-store) nor corrupts any claim
// file.
package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// asyncResult is one finished background CLI invocation.
type asyncResult struct {
	stdout, stderr string
	code           int
}

// runAsync execs the built dossierx binary in dir without blocking the caller,
// delivering its result on the returned channel. Unlike run() it never touches
// *testing.T (a goroutine may not call t.Fatalf), so the caller does the
// asserting on the main goroutine. A binary that fails to even start reports
// code -1.
func runAsync(dir string, args ...string) <-chan asyncResult {
	ch := make(chan asyncResult, 1)
	go func() {
		// --format text for the same reason run() prepends it: this suite
		// asserts prose and exit codes, not the v0.3.0 envelope.
		cmd := exec.Command(binPath, append([]string{"--format", "text"}, args...)...)
		cmd.Dir = dir
		var out, errb strings.Builder
		cmd.Stdout = &out
		cmd.Stderr = &errb
		err := cmd.Run()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		ch <- asyncResult{out.String(), errb.String(), code}
	}()
	return ch
}

// assertGatedThenCompletes holds the project-wide claims sentinel, launches the
// writer, and requires that while the sentinel is held the writer makes NO
// progress — it neither exits nor changes claimPath (when non-empty) — then
// releases the sentinel and requires the writer to finish with wantCode. If the
// writer did not take the claims sentinel it would run to completion while we
// held it, which is exactly what the first select catches.
func assertGatedThenCompletes(t *testing.T, root string, args []string, wantCode int, claimPath string) {
	t.Helper()

	var before string
	if claimPath != "" {
		before = llReadFile(t, claimPath)
	}

	sentinel := filepath.Join(root, ".dossierx-claims.lock")
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatalf("hold claims sentinel: %v", err)
	}

	ch := runAsync(root, args...)

	// While the sentinel is held the writer must be blocked. 500ms is far
	// under the 10s AcquireFileLock timeout, so releasing next still lets it
	// finish promptly.
	select {
	case r := <-ch:
		t.Fatalf("%v completed while the claims sentinel was held (exit %d) — it did not take the sentinel\nstdout: %s\nstderr: %s", args, r.code, r.stdout, r.stderr)
	case <-time.After(500 * time.Millisecond):
	}
	if claimPath != "" {
		if now := llReadFile(t, claimPath); now != before {
			t.Fatalf("%v changed its claim file while the claims sentinel was held — it did not take the sentinel\nbefore:\n%s\nafter:\n%s", args, before, now)
		}
	}

	if err := os.Remove(sentinel); err != nil {
		t.Fatalf("release claims sentinel: %v", err)
	}
	select {
	case r := <-ch:
		if r.code != wantCode {
			t.Fatalf("%v after the sentinel was released: exit %d, want %d\nstdout: %s\nstderr: %s", args, r.code, wantCode, r.stdout, r.stderr)
		}
	case <-time.After(9 * time.Second):
		t.Fatalf("%v did not complete within 9s after the claims sentinel was released — possible deadlock", args)
	}
}

// TestClaimsSentinelGatesEveryClaimWriter proves each retrofitted command takes
// the project-wide claims sentinel before touching a claim file.
func TestClaimsSentinelGatesEveryClaimWriter(t *testing.T) {
	claimPathOf := func(root string) string { return filepath.Join(root, "claims", "overview.yaml") }

	t.Run("claim lock", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureProject(t, root, "widlock")
		id := "widlock.contract.overview"
		assertGatedThenCompletes(t, root, []string{"claim", "lock", id, "--reason", "test fixture"}, 0, claimPathOf(root))
		if got := llReadFile(t, claimPathOf(root)); !strings.Contains(got, "status: locked") {
			t.Fatalf("expected %s locked after release, got:\n%s", id, got)
		}
	})

	t.Run("claim unlock", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureProject(t, root, "widunlock")
		id := "widunlock.contract.overview"
		if _, stderr, code := run(t, root, "claim", "lock", id, "--reason", "test fixture"); code != 0 {
			t.Fatalf("setup lock: exit %d: %s", code, stderr)
		}
		assertGatedThenCompletes(t, root, []string{"claim", "unlock", id, "--reason", "test fixture"}, 0, claimPathOf(root))
		if got := llReadFile(t, claimPathOf(root)); !strings.Contains(got, "status: draft") {
			t.Fatalf("expected %s draft after release, got:\n%s", id, got)
		}
	})

	t.Run("claim flag", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureProject(t, root, "widflag")
		id := "widflag.contract.overview"
		if _, stderr, code := run(t, root, "claim", "lock", id, "--reason", "test fixture"); code != 0 {
			t.Fatalf("setup lock: exit %d: %s", code, stderr)
		}
		assertGatedThenCompletes(t, root, []string{"claim", "flag", id, "--claim-says", "old", "--now-does", "new", "--reason", "because"}, 0, claimPathOf(root))
		if got := llReadFile(t, claimPathOf(root)); !strings.Contains(got, "review_pending: true") {
			t.Fatalf("expected %s review_pending after release, got:\n%s", id, got)
		}
	})

	t.Run("reaudit_confirm", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureProject(t, root, "widreaudit")
		id := "widreaudit.contract.overview"
		if _, stderr, code := run(t, root, "claim", "lock", id, "--reason", "test fixture"); code != 0 {
			t.Fatalf("setup lock: exit %d: %s", code, stderr)
		}
		if _, stderr, code := run(t, root, "claim", "flag", id, "--claim-says", "old", "--now-does", "the new truth", "--reason", "because"); code != 0 {
			t.Fatalf("setup flag: exit %d: %s", code, stderr)
		}
		assertGatedThenCompletes(t, root, []string{"claim", "reaudit", id, "--confirm", "--reason", "test fixture"}, 0, claimPathOf(root))
		if got := llReadFile(t, claimPathOf(root)); strings.Contains(got, "review_pending: true") {
			t.Fatalf("expected %s review_pending cleared after release, got:\n%s", id, got)
		}
	})

	t.Run("check", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureProject(t, root, "widcheck")
		// "check" takes the claims sentinel in its review_pending reconcile
		// phase; with no dependency drift it writes no claim file, so gating is
		// proven purely by "blocked while held, completes after release".
		assertGatedThenCompletes(t, root, []string{"check"}, 0, "")
	})

	// Negative control: a read-only command must NOT take the claims sentinel.
	// If it completes promptly while the sentinel is held, that confirms the
	// blocking seen above is specifically the writers taking the sentinel —
	// not some unrelated stall that would make every subtest pass vacuously.
	t.Run("deps_read_is_not_gated", func(t *testing.T) {
		root := t.TempDir()
		writeFixtureProject(t, root, "widdeps")
		id := "widdeps.contract.overview"
		sentinel := filepath.Join(root, ".dossierx-claims.lock")
		if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
			t.Fatalf("hold claims sentinel: %v", err)
		}
		defer os.Remove(sentinel)

		ch := runAsync(root, "claim", "show", id)
		select {
		case r := <-ch:
			if r.code != 0 {
				t.Fatalf("read-only deps exit %d while sentinel held\nstderr: %s", r.code, r.stderr)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("read-only deps blocked on the claims sentinel — a reader must never take it")
		}
	})
}

// TestConcurrentClaimWritersNeverCorruptClaimFiles hammers the claims sentinel
// from many processes at once — distinct-claim locks in parallel plus a single
// "hot" claim repeatedly locked/unlocked — and requires that the new global
// acquisition order neither deadlocks nor leaves any claim file corrupt, the
// lock store missing an entry, or the sentinel lock file leaked.
func TestConcurrentClaimWritersNeverCorruptClaimFiles(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - cwmod\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	writeClaim := func(name, id string) {
		claim := "id: " + id + "\n" +
			"facet: contract\nmodule: cwmod\nstatus: draft\nlayout: card\n" +
			"body: |\n  concurrent-writer fixture claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
		if err := os.WriteFile(filepath.Join(claimsDir, name+".yaml"), []byte(claim), 0o644); err != nil {
			t.Fatalf("write claim %s: %v", id, err)
		}
	}

	const n = 5
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = "cwmod.contract.c" + strconv.Itoa(i)
		writeClaim("c"+strconv.Itoa(i), ids[i])
	}
	hotID := "cwmod.contract.hot"
	writeClaim("hot", hotID)

	var wg sync.WaitGroup

	// Distinct-claim locks, all in parallel (each writes its own file).
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, stderr, code := run(t, root, "claim", "lock", id, "--reason", "test fixture"); code != 0 {
				t.Errorf("concurrent lock %s: exit %d: %s", id, code, stderr)
			}
		}(id)
	}

	// Several writers pounding ONE shared claim file with lock<->unlock cycles.
	// lock and unlock are each idempotent on an already-in-that-state claim, so
	// every op must exit 0; the claims sentinel is what keeps their whole-file
	// rewrites from interleaving.
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 2; k++ {
				if _, stderr, code := run(t, root, "claim", "lock", hotID, "--reason", "test fixture"); code != 0 {
					t.Errorf("concurrent lock %s: exit %d: %s", hotID, code, stderr)
					return
				}
				if _, stderr, code := run(t, root, "claim", "unlock", hotID, "--reason", "test fixture"); code != 0 {
					t.Errorf("concurrent unlock %s: exit %d: %s", hotID, code, stderr)
					return
				}
			}
		}()
	}
	wg.Wait()

	// Every distinct claim must be intact and locked on disk.
	for i, id := range ids {
		raw := llReadFile(t, filepath.Join(claimsDir, "c"+strconv.Itoa(i)+".yaml"))
		if !strings.Contains(raw, "status: locked") {
			t.Errorf("expected %s locked on disk after the storm, got:\n%s", id, raw)
		}
	}
	// The shared lock store must carry every distinct claim — the lost-update
	// invariant TestConcurrentLocksDoNotLoseStoreUpdates guards, re-checked
	// under the added claims-sentinel contention.
	storeRaw := llReadFile(t, filepath.Join(root, ".dossierx-lock-store.json"))
	for _, id := range ids {
		if !strings.Contains(storeRaw, id) {
			t.Errorf("expected lock store to carry %s, but it was lost:\n%s", id, storeRaw)
		}
	}
	// The hot claim must have survived as valid, parseable YAML in a legal end
	// state (a final "check" both proves it parses and confirms no deadlock).
	if _, stderr, code := run(t, root, "check"); code != 0 {
		t.Fatalf("final check after the storm failed (a corrupt claim file would surface here): exit %d: %s", code, stderr)
	}
	hotRaw := llReadFile(t, filepath.Join(claimsDir, "hot.yaml"))
	if !strings.Contains(hotRaw, "status: locked") && !strings.Contains(hotRaw, "status: draft") {
		t.Fatalf("hot claim ended in neither a locked nor a draft state — corrupt/truncated:\n%s", hotRaw)
	}
	// The sentinel lock file is created and removed per invocation; none may
	// have leaked (a leak would wedge the next writer for the 10s timeout).
	if _, err := os.Stat(filepath.Join(root, ".dossierx-claims.lock")); !os.IsNotExist(err) {
		t.Fatalf("claims sentinel lock file leaked after the storm (stat err=%v)", err)
	}
}
