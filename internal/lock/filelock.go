package lock

import (
	"fmt"
	"os"
	"time"
)

// lockAcquireTimeout and lockPollInterval are overridable in tests so the
// timeout path can be exercised without a real multi-second sleep.
var (
	lockAcquireTimeout = 10 * time.Second
	lockPollInterval   = 20 * time.Millisecond
)

// AcquireFileLock creates an exclusive, cross-process advisory lock next to
// the Store at storePath (storePath+".lock"), so concurrent CLI invocations
// that each load-mutate-save the same Store never lose one process's update
// to another's. Without this, two concurrent "docs lock <id>" runs (on two
// different claims, sharing one project-wide store file) each load the same
// on-disk snapshot, mutate their own in-memory copy, and whichever Save
// happens last silently discards the other's LockedAt/Hashes entries — a
// classic lost-update race, reproducible by running "docs lock" on several
// claims in parallel against a fresh project.
//
// This is a plain, portable mechanism (os.O_EXCL on a sentinel file — no
// OS-specific syscalls, preserving the engine's cross-platform portability
// bar), not a general-purpose distributed lock: it only serializes
// same-machine callers targeting the same store path, which is exactly the
// engine's actual concurrency model (a single project's claims_dir, worked
// on by one or more local CLI invocations).
//
// It retries with backoff up to a fixed timeout before giving up — a stale
// lock file left behind by a killed process would otherwise wedge every
// future invocation forever. On success it returns a release func the
// caller must call (typically via defer) to remove the lock file; release
// is always non-nil when err is nil.
func AcquireFileLock(storePath string) (release func(), err error) {
	lockPath := storePath + ".lock"
	deadline := time.Now().Add(lockAcquireTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("lock: acquire file lock %s: %w", lockPath, err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock: timed out waiting for file lock %s (another docs process may be holding it; remove the file manually if it was left behind by a crash)", lockPath)
		}
		time.Sleep(lockPollInterval)
	}
}
