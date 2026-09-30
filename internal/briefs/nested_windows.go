//go:build windows

package briefs

import "os"

// searchable is git's access(path, X_OK) on a git directory's objects/ and
// refs/. Git for Windows drops X_OK before calling the C runtime's access, so
// there it asks only whether the path exists; so does this.
func searchable(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
