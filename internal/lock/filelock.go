package lock

import (
	"fmt"
	"os"
	"path/filepath"
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
// to another's. Without this, two concurrent "dossierx lock <id>" runs (on two
// different claims, sharing one project-wide store file) each load the same
// on-disk snapshot, mutate their own in-memory copy, and whichever Save
// happens last silently discards the other's LockedAt/Hashes entries — a
// classic lost-update race, reproducible by running "dossierx lock" on several
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
// "ALREADY EXISTS" IS NOT THE ONLY WAY A CONTENDED LOCK FAILS, and the other
// way is why this loop does not simply test os.IsExist.
//
// On Windows, deleting a file does not remove its directory entry the moment
// the unlink returns. The entry survives in a DELETE-PENDING state until the
// last handle to it closes, and while it is in that state every open of that
// path fails with ERROR_ACCESS_DENIED — not ERROR_FILE_EXISTS. So the instant
// another process releases the lock is precisely an instant when this open
// reports "Access is denied."
//
// That is a transient state which clears on its own within one poll, and it is
// exactly what this retry loop exists for. Reading it as a hard failure turned
// the contended case — the ONLY case the lock is for — into an immediate error,
// and it did so intermittently, which is the shape that survives review. It
// failed a windows-latest CI leg on this very release, in
// TestConcurrentClaimWritersNeverCorruptClaimFiles, having passed the two runs
// before it.
//
// POSIX never reaches it: unlink is atomic there, so a lock path is either
// present or gone and EACCES on it is a real permission problem that must fail
// fast rather than spin for ten seconds. lockOpenIsTransient is therefore
// per-platform, and on every platform but Windows it is a constant false.
func AcquireFileLock(storePath string) (release func(), err error) {
	lockPath := storePath + ".lock"
	// The sentinel sits beside its store under build/ledger/, which a fresh
	// project does not have until the first write creates it — and the first
	// write is exactly the one this lock guards. A failure here is reported
	// with lockPath in the message, never only the directory: cmd/dossierx's
	// comment path classifies a sentinel failure as write_conflict by finding
	// the sentinel's path in the error text (see claimsSentinelContention),
	// and on a read-only project directory this MkdirAll is now the first
	// step that fails.
	if mkErr := os.MkdirAll(filepath.Dir(lockPath), 0o755); mkErr != nil {
		return nil, fmt.Errorf("lock: acquire file lock %s: %w", lockPath, mkErr)
	}
	deadline := time.Now().Add(lockAcquireTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		contended := os.IsExist(err) || lockOpenIsTransient(err)
		if !contended {
			return nil, fmt.Errorf("lock: acquire file lock %s: %w", lockPath, err)
		}
		if time.Now().After(deadline) {
			// The two contended endings are reported apart. A timeout over
			// "already exists" is a holder that never let go; a timeout over the
			// transient error is a path that stayed unopenable for the whole
			// window, which is not delete-pending behaviour and is much more
			// likely a permission or antivirus problem wearing its clothes.
			// Collapsing them would send a reader hunting for a process that was
			// never there.
			if !os.IsExist(err) {
				return nil, fmt.Errorf("lock: timed out waiting for file lock %s, and every attempt failed with %w rather than \"already exists\". That is not a lock another process is holding — check the directory's permissions, and any scanner that may be holding the path open", lockPath, err)
			}
			return nil, fmt.Errorf("lock: timed out waiting for file lock %s (another docs process may be holding it; remove the file manually if it was left behind by a crash)", lockPath)
		}
		time.Sleep(lockPollInterval)
	}
}
