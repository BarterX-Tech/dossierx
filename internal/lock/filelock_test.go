package lock

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireFileLock_MutualExclusion(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "store.json")

	release1, err := AcquireFileLock(storePath)
	if err != nil {
		t.Fatalf("first AcquireFileLock: %v", err)
	}

	// Speed the second attempt's retry loop up so the test doesn't wait
	// out the real default timeout.
	origTimeout, origPoll := lockAcquireTimeout, lockPollInterval
	lockAcquireTimeout = 150 * time.Millisecond
	lockPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { lockAcquireTimeout, lockPollInterval = origTimeout, origPoll })

	if _, err := AcquireFileLock(storePath); err == nil {
		t.Fatal("expected second AcquireFileLock to fail/time out while the first holder still holds the lock, got nil error")
	}

	release1()

	// Once released, a new acquire must succeed.
	release2, err := AcquireFileLock(storePath)
	if err != nil {
		t.Fatalf("AcquireFileLock after release: %v", err)
	}
	release2()
}

// TestAcquireFileLock_SerializesConcurrentGoroutines proves the lock
// actually forces mutual exclusion under real concurrency (not just two
// sequential calls): n goroutines each acquire the lock, then while
// holding it, increment a shared counter, sleep briefly, and re-read the
// counter — if the lock ever let two goroutines hold it at once, a
// goroutine's re-read could observe a value incremented by someone else
// mid-critical-section, not just its own increment. The counter itself is
// accessed with sync/atomic (not a bare int) purely so this test is
// itself race-detector clean; the property under test is the filesystem
// lock's real-world mutual exclusion, not Go's memory model.
func TestAcquireFileLock_SerializesConcurrentGoroutines(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "store.json")

	const n = 8
	var counter atomic.Int64
	var wg sync.WaitGroup
	violations := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := AcquireFileLock(storePath)
			if err != nil {
				violations <- "acquire failed: " + err.Error()
				return
			}
			defer release()

			mine := counter.Add(1)
			time.Sleep(2 * time.Millisecond)
			if got := counter.Load(); got != mine {
				violations <- "counter mutated by another holder while lock was held"
			}
		}()
	}
	wg.Wait()
	close(violations)

	for v := range violations {
		t.Error(v)
	}
	if got := counter.Load(); got != n {
		t.Errorf("expected counter == %d after all goroutines held the lock exactly once, got %d", n, got)
	}
}

// TestLockOpenIsTransientMatchesThisPlatformsUnlinkSemantics pins the one
// classification AcquireFileLock's retry loop turns on, per platform, because
// getting it wrong is invisible in the uncontended case and intermittent in the
// contended one.
//
// It is written against the platform rather than against a fixed expectation on
// purpose: the correct answer genuinely differs, and a test that asserted one
// answer everywhere would have to be wrong on one side.
func TestLockOpenIsTransientMatchesThisPlatformsUnlinkSemantics(t *testing.T) {
	permission := &fs.PathError{Op: "open", Path: "store.json.lock", Err: fs.ErrPermission}
	exists := &fs.PathError{Op: "open", Path: "store.json.lock", Err: fs.ErrExist}
	missing := &fs.PathError{Op: "open", Path: "store.json.lock", Err: fs.ErrNotExist}

	// On Windows a permission error is the DELETE-PENDING window and must be
	// waited out; everywhere else unlink is atomic, there is no such window, and
	// the same error is a real permission problem that must fail fast.
	wantPermissionTransient := runtime.GOOS == "windows"
	if got := lockOpenIsTransient(permission); got != wantPermissionTransient {
		t.Errorf("lockOpenIsTransient(permission) = %v on %s, want %v.\n"+
			"On Windows this must be true or the contended case — the only case the lock exists for — fails outright the moment another process releases the lock.\n"+
			"Everywhere else it must be false or a genuinely unwritable directory spins for the whole acquire timeout and then reports a vaguer error than the OS already gave.",
			got, runtime.GOOS, wantPermissionTransient)
	}

	// "Already exists" is the OTHER contended branch, handled by os.IsExist. If
	// this ever answered true for it the two would collapse, and the timeout
	// message would stop being able to tell a holder that never let go from a
	// path that was never openable.
	if lockOpenIsTransient(exists) {
		t.Error("lockOpenIsTransient(exists) = true; the already-exists case is os.IsExist's, and conflating the two makes AcquireFileLock's two timeout messages indistinguishable")
	}

	// Nothing else is transient on any platform. A missing file is not even a
	// failure mode of O_CREATE|O_EXCL, so answering true here would mean the
	// classifier is not reading the error at all.
	if lockOpenIsTransient(missing) {
		t.Error("lockOpenIsTransient(not-exist) = true; that is not a state O_CREATE|O_EXCL can report, so a true here means the classifier ignores its argument")
	}
}

// TestAcquireFileLockReportsTheTwoContendedTimeoutsApart holds the lock and
// times a waiter out, which is the "already exists" ending. The message has to
// name a holder, because that is the recovery: find the other process, or delete
// a file a crash left behind.
//
// The other ending — every attempt failing with a transient error for the whole
// window — cannot be provoked portably, since only Windows produces it and only
// under a race. It is covered by the classifier test above rather than left
// unstated.
func TestAcquireFileLockReportsTheTwoContendedTimeoutsApart(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "store.json")

	release, err := AcquireFileLock(storePath)
	if err != nil {
		t.Fatalf("acquire the lock to hold it: %v", err)
	}
	defer release()

	origTimeout, origPoll := lockAcquireTimeout, lockPollInterval
	lockAcquireTimeout = 80 * time.Millisecond
	lockPollInterval = 10 * time.Millisecond
	t.Cleanup(func() { lockAcquireTimeout, lockPollInterval = origTimeout, origPoll })

	_, err = AcquireFileLock(storePath)
	if err == nil {
		t.Fatal("a second acquire against a held lock returned no error")
	}
	if !strings.Contains(err.Error(), "another docs process may be holding it") {
		t.Errorf("the held-lock timeout must name a holder and the manual recovery; got: %v", err)
	}
	if strings.Contains(err.Error(), "check the directory's permissions") {
		t.Errorf("the held-lock timeout reported the permission ending, which sends the reader after a problem that is not there; got: %v", err)
	}
}
