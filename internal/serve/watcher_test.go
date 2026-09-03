package serve

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestIgnoredClaimFile(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"claim.yaml", false},
		{"claim.yml", false},
		{"claim.YAML", false},   // extension match is case-insensitive
		{".hidden.yaml", false}, // dot-FILES are still claims; only dot-DIRS are skipped (at the walk level)
		{"notes.md", true},
		{"README", true},
		{"claim.yaml.tmp-123456", true}, // the atomic writer's scratch pattern
		{"claim.yml.tmp-9", true},
		{"a.tmp-b.yaml", true}, // matches the *.tmp-* exclusion
	}
	for _, c := range cases {
		if got := ignoredClaimFile(c.name); got != c.want {
			t.Errorf("ignoredClaimFile(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsInsideDir(t *testing.T) {
	sep := string(filepath.Separator)
	root := filepath.Join(sep+"proj", "claims")
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(root, "a.yaml"), true},
		{filepath.Join(root, "sub", "deep", "b.yaml"), true},
		{root, true}, // the dir itself counts as inside
		{filepath.Join(sep+"proj", "build", "viewer", "index.html"), false},
		{filepath.Join(sep+"proj", "build", "catalog", "catalog.json"), false},
		{sep + "proj", false}, // the parent is outside
		{filepath.Join(sep+"other", "x"), false},
	}
	for _, c := range cases {
		if got := isInsideDir(root, c.path); got != c.want {
			t.Errorf("isInsideDir(%q, %q) = %v, want %v", root, c.path, got, c.want)
		}
	}
}

// A relevant claim create/modify/delete anywhere in the tree (including a nested
// dir) changes the fingerprint; nothing else does.
func TestScanFingerprint_DetectsRelevantChanges(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yaml"), "one")
	writeTestFile(t, filepath.Join(dir, "sub", "b.yaml"), "two")

	fp1, err := scanFingerprint(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(fp1) != 2 {
		t.Fatalf("fingerprint has %d entries, want 2", len(fp1))
	}

	writeTestFile(t, filepath.Join(dir, "sub", "b.yaml"), "two-and-more")
	fp2, err := scanFingerprint(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if fingerprintsEqual(fp1, fp2) {
		t.Fatal("fingerprint unchanged after a nested-dir claim modify")
	}

	writeTestFile(t, filepath.Join(dir, "c.yaml"), "three")
	fp3, err := scanFingerprint(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if fingerprintsEqual(fp2, fp3) {
		t.Fatal("fingerprint unchanged after adding a claim")
	}

	if err := os.Remove(filepath.Join(dir, "a.yaml")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	fp4, err := scanFingerprint(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if fingerprintsEqual(fp3, fp4) {
		t.Fatal("fingerprint unchanged after deleting a claim")
	}
}

// Temp files, dot-directories, and non-YAML files are invisible to the
// fingerprint, so none of them can ever raise a spurious "changed".
func TestScanFingerprint_IgnoresTmpDotDirAndNonYAML(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yaml"), "one")
	base, err := scanFingerprint(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	writeTestFile(t, filepath.Join(dir, "a.yaml.tmp-000123"), "scratch")
	fp, err := scanFingerprint(dir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !fingerprintsEqual(base, fp) {
		t.Fatal("a *.tmp-* scratch file changed the fingerprint")
	}
	writeTestFile(t, filepath.Join(dir, ".git", "HEAD.yaml"), "ref: main")
	if fp, err := scanFingerprint(dir); err != nil {
		t.Fatalf("scan: %v", err)
	} else if !fingerprintsEqual(base, fp) {
		t.Fatal("a file under a dot-directory changed the fingerprint")
	}
	writeTestFile(t, filepath.Join(dir, "notes.md"), "text")
	if fp, err := scanFingerprint(dir); err != nil {
		t.Fatalf("scan: %v", err)
	} else if !fingerprintsEqual(base, fp) {
		t.Fatal("a non-YAML file changed the fingerprint")
	}
}

// newTestWatcher runs a watcher on dir at a short cadence and returns a pointer
// to its debounced-change counter plus a cancel. The baseline is captured
// synchronously here (as Serve does), so a change after this call is seen.
func newTestWatcher(t *testing.T, dir string) (*int32, context.CancelFunc) {
	t.Helper()
	baseline, err := scanFingerprint(dir)
	if err != nil {
		t.Fatalf("baseline scan: %v", err)
	}
	var count int32
	w := newWatcher(dir, 10*time.Millisecond, 40*time.Millisecond, func() {
		atomic.AddInt32(&count, 1)
	})
	ctx, cancel := context.WithCancel(context.Background())
	go w.run(ctx, baseline)
	return &count, cancel
}

// A single claim modify fires exactly one debounced change, then stays quiet:
// one delta -> one debounce -> one onChange, and later polls of the stable file
// re-arm nothing.
func TestWatcher_SingleChangeFiresOnce(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yaml"), "one")
	count, cancel := newTestWatcher(t, dir)
	defer cancel()

	writeTestFile(t, filepath.Join(dir, "a.yaml"), "one-changed")
	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("single change produced %d notifications, want exactly 1", got)
	}
}

// An external write of a brand-new claim in a NEW nested subdir is detected.
func TestWatcher_ExternalNestedWriteFires(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yaml"), "one")
	count, cancel := newTestWatcher(t, dir)
	defer cancel()

	writeTestFile(t, filepath.Join(dir, "nested", "deep", "b.yaml"), "two")
	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("a nested-dir claim write produced %d notifications, want 1", got)
	}
}

// A temp file appearing and then vanishing (the atomic-writer pattern) fires
// nothing at all.
func TestWatcher_TmpFileDoesNotFire(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yaml"), "one")
	count, cancel := newTestWatcher(t, dir)
	defer cancel()

	tmp := filepath.Join(dir, "a.yaml.tmp-424242")
	writeTestFile(t, tmp, "scratch")
	time.Sleep(120 * time.Millisecond)
	if err := os.Remove(tmp); err != nil {
		t.Fatalf("remove tmp: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if got := atomic.LoadInt32(count); got != 0 {
		t.Fatalf("a *.tmp-* file appearing/vanishing produced %d notifications, want 0", got)
	}
}

// Two separate changes, spaced well beyond the debounce window, fire twice —
// the watcher keeps working after the first notification.
//
// Each write changes the file's SIZE as well as its content, deliberately. The
// fingerprint is (modNano, size), and a filesystem whose modification-time
// granularity is coarser than the gap between two writes reports both with the
// same stamp — on Windows that granularity is tens of milliseconds, so an
// earlier version of this test ("one" -> "two", both 3 bytes, written
// microseconds after the baseline scan) saw no delta and failed there while
// passing on macOS and Linux. That is a real property of a zero-dependency
// mtime poll and is documented on scanFingerprint; this test is about the
// watcher firing repeatedly, so it does not also try to assert its way around
// timestamp resolution.
func TestWatcher_SeparateChangesFireEachTime(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.yaml"), "one")
	count, cancel := newTestWatcher(t, dir)
	defer cancel()

	writeTestFile(t, filepath.Join(dir, "a.yaml"), "two-changed")
	time.Sleep(250 * time.Millisecond)
	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("after first change: %d notifications, want 1", got)
	}
	writeTestFile(t, filepath.Join(dir, "a.yaml"), "three-changed-again")
	time.Sleep(250 * time.Millisecond)
	if got := atomic.LoadInt32(count); got != 2 {
		t.Fatalf("after second change: total %d notifications, want 2", got)
	}
}
