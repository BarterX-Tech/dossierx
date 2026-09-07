//go:build windows

package lock

import (
	"errors"
	"io/fs"
	"testing"
)

func TestLockReadIsTransientRecognizesWindowsSharingViolation(t *testing.T) {
	for _, err := range []error{
		windowsErrorSharingViolation,
		&fs.PathError{Op: "open", Path: "lock-store.json", Err: windowsErrorSharingViolation},
		errors.Join(errors.New("read store"), windowsErrorSharingViolation),
	} {
		if !lockReadIsTransient(err) {
			t.Fatalf("lockReadIsTransient(%v) = false, want true", err)
		}
	}
}

func TestLockReadIsTransientDoesNotBroadenToOtherWindowsErrors(t *testing.T) {
	// ERROR_LOCK_VIOLATION (33) was not observed here and is not classified by
	// this repair. Unknown filesystem failures must still return immediately.
	if lockReadIsTransient(windowsErrorSharingViolation + 1) {
		t.Fatal("unobserved Windows error was classified as transient")
	}
}
