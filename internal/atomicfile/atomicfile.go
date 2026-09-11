// Package atomicfile replaces generated files through a sibling temporary file.
// If replacement succeeds, concurrent readers see a complete old or new file;
// platform-specific rename refusal is returned without truncating the old file.
package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Write creates path's parent directory, writes a sibling temporary file, and
// atomically renames it over path.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return os.Rename(tmpPath, path)
	}
	var renameErr error
	for i := 0; i < 5; i++ {
		renameErr = os.Rename(tmpPath, path)
		if renameErr == nil {
			return nil
		}
		if i < 4 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return renameErr
}
