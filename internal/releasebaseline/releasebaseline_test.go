package releasebaseline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveChoosesGreatestReachableStablePredecessor(t *testing.T) {
	dir := tempRepo(t)
	writeRepoFile(t, dir, "CHANGELOG.md", "## [0.7.8] - 2026-09-06\n")
	commitRepo(t, dir, "current")
	gitRepo(t, dir, "tag", "v0.7.7")
	writeRepoFile(t, dir, "extra", "later")
	commitRepo(t, dir, "later")
	gitRepo(t, dir, "tag", "v0.8.0")
	gitRepo(t, dir, "tag", "nightly")
	got, err := Resolve(Options{RepoDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if got.BaselineTag != "v0.7.7" || got.BaselineVersion != "0.7.7" || got.CurrentVersion != "0.7.8" {
		t.Fatalf("result = %+v", got)
	}
	if _, err := Resolve(Options{RepoDir: dir, OverrideTag: "v0.7.7"}); err != nil {
		t.Fatalf("exact predecessor override rejected: %v", err)
	}
}

func TestResolveRejectsStaleOrFutureOverride(t *testing.T) {
	dir := tempRepo(t)
	writeRepoFile(t, dir, "CHANGELOG.md", "## [0.7.8] - 2026-09-06\n")
	commitRepo(t, dir, "baseline")
	gitRepo(t, dir, "tag", "v0.7.7")
	writeRepoFile(t, dir, "current", "yes")
	commitRepo(t, dir, "current")
	gitRepo(t, dir, "tag", "v0.8.0")
	for _, override := range []string{"v0.7.0", "v0.8.0", "v0.7.7-nope"} {
		if _, err := Resolve(Options{RepoDir: dir, OverrideTag: override}); err == nil {
			t.Fatalf("override %q unexpectedly accepted", override)
		}
	}
}

func TestCompareVersionsUsesNumericOrder(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{{"0.7.9", "0.7.10", -1}, {"1.0.0", "0.99.99", 1}, {"0.7.7", "0.7.7", 0}}
	for _, tc := range cases {
		a, _ := parseVersion("v" + tc.a)
		b, _ := parseVersion("v" + tc.b)
		if got := compare(a, b); got != tc.want {
			t.Errorf("compare(%s,%s)=%d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestResolveRejectsMalformedHeadingAndShallowRepo(t *testing.T) {
	dir := tempRepo(t)
	writeRepoFile(t, dir, "CHANGELOG.md", "## [not-semver] - 2026-09-06\n")
	commitRepo(t, dir, "bad heading")
	if _, err := Resolve(Options{RepoDir: dir}); err == nil {
		t.Fatal("malformed current heading unexpectedly accepted")
	}

	parent := tempRepo(t)
	writeRepoFile(t, parent, "CHANGELOG.md", "## [0.7.8] - 2026-09-06\n")
	commitRepo(t, parent, "current")
	gitRepo(t, parent, "tag", "v0.7.7")
	clone := filepath.Join(t.TempDir(), "shallow")
	cmd := exec.Command("git", "clone", "--depth", "1", "file://"+parent, clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone shallow: %v\n%s", err, out)
	}
	if _, err := Resolve(Options{RepoDir: clone}); err == nil {
		t.Fatal("shallow repository unexpectedly accepted")
	}
}

func TestResolveRejectsNonCommitStableTag(t *testing.T) {
	dir := tempRepo(t)
	writeRepoFile(t, dir, "CHANGELOG.md", "## [0.7.8] - 2026-09-06\n")
	commitRepo(t, dir, "current")
	gitRepo(t, dir, "tag", "v0.7.7")
	blob := strings.TrimSpace(gitRepo(t, dir, "hash-object", "-w", "CHANGELOG.md"))
	gitRepo(t, dir, "tag", "v0.7.6", blob)
	if _, err := Resolve(Options{RepoDir: dir}); err == nil {
		t.Fatal("stable tag pointing at a blob unexpectedly accepted")
	}
}

func TestResolveExcludesCurrentTag(t *testing.T) {
	dir := tempRepo(t)
	writeRepoFile(t, dir, "CHANGELOG.md", "## [0.7.8] - 2026-09-06\n")
	commitRepo(t, dir, "baseline")
	gitRepo(t, dir, "tag", "v0.7.7")
	writeRepoFile(t, dir, "current", "current")
	commitRepo(t, dir, "current")
	gitRepo(t, dir, "tag", "v0.7.8")
	got, err := Resolve(Options{RepoDir: dir, EventRef: "refs/tags/v0.7.8"})
	if err != nil || got.BaselineTag != "v0.7.7" {
		t.Fatalf("tag-first result=%+v err=%v", got, err)
	}
}

func tempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRepo(t, dir, "init", "-q")
	gitRepo(t, dir, "config", "user.email", "test@example.invalid")
	gitRepo(t, dir, "config", "user.name", "test")
	return dir
}

func writeRepoFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitRepo(t *testing.T, dir, message string) {
	gitRepo(t, dir, "add", ".")
	gitRepo(t, dir, "commit", "-qm", message)
}

func gitRepo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
