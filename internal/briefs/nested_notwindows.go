//go:build !windows

package briefs

import "os"

// searchable stands in for git's access(path, X_OK) on a git directory's
// objects/ and refs/: the path exists (followed through a symlink) and carries
// an execute (search) bit. That is the answer access gives for every ordinary
// shape — a directory git init made, a directory stripped of its x bits, a
// plain file with or without one — and it is read from the mode bits because
// the engine makes no raw system calls (tests/portability_test.go). What it
// does not reproduce is access's per-caller answer: an x bit that is set for
// another user only, or a caller running as root.
func searchable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().Perm()&0o111 != 0
}
