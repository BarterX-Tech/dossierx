//go:build windows

package lock

import (
	"errors"
	"io/fs"
	"syscall"
)

// ERROR_SHARING_VIOLATION is 32 in the Windows API. syscall.Errno is only the
// portable error-code type here; no syscall is invoked.
const windowsErrorSharingViolation = syscall.Errno(32)

// lockOpenIsTransient reports whether a failed O_CREATE|O_EXCL open of the lock
// sentinel names a state that clears on its own, and so is a reason to keep
// polling rather than to give up.
//
// On Windows there is exactly one: DELETE-PENDING. A file whose unlink has
// returned but whose last handle has not yet closed keeps its directory entry,
// and every open of that path fails with ERROR_ACCESS_DENIED — which the Go
// runtime maps to fs.ErrPermission — until the entry finally goes. Since the
// releasing process removes the lock file and the waiting process is polling
// for it, that window is precisely the moment the waiter is trying to open.
//
// This deliberately does not distinguish delete-pending from a genuine
// permission failure, because Windows does not give the caller anything to
// distinguish them WITH: both arrive as the same error code. The cost of
// treating a real permission error as transient is bounded — one
// lockAcquireTimeout of polling, and then a refusal that says in as many words
// that every attempt failed this way and a permission problem is the likely
// cause. The cost of the other mistake is the one already paid: the contended
// case, which is the only case the lock exists for, failing outright.
func lockOpenIsTransient(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}

func lockReadIsTransient(err error) bool {
	// ERROR_SHARING_VIOLATION is distinct from ERROR_ACCESS_DENIED, so the
	// fs.ErrPermission check alone does not recognize this transient race.
	return errors.Is(err, fs.ErrPermission) || errors.Is(err, windowsErrorSharingViolation)
}
