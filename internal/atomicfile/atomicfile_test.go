package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesCompleteFileAndLeavesNoTemporarySibling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "artifact.json")
	old := bytes.Repeat([]byte("old"), 1024)
	newData := bytes.Repeat([]byte("new"), 2048)
	if err := Write(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, newData, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newData) {
		t.Fatalf("replacement has %d bytes, want %d", len(got), len(newData))
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), filepath.Base(path)+".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary siblings remain: %v", matches)
	}
}

func TestWriteFailedReplacementPreservesDestinationCleansTempAndCanRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "previous")
	if err := os.WriteFile(marker, []byte("still here"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("replacement"), 0o644); err == nil {
		t.Fatal("expected replacement of directory by file to fail")
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != "still here" {
		t.Fatalf("failed replacement damaged destination: %q err=%v", got, err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "artifact.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary siblings remain after failure: %v", matches)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("replacement"), 0o644); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "replacement" {
		t.Fatalf("retry output = %q err=%v", got, err)
	}
}
