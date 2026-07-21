package lock

import (
	"path/filepath"
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
