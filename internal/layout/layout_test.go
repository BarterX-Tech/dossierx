package layout_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/layout"
)

// legacyProject writes a project.config.yaml and a claims/ directory under a
// fresh temp dir, plus each legacy root file named in files (path -> content),
// and returns the loaded config. The temp dir sits under the OS temp root,
// which the no-git rows require to have no .git above it.
func legacyProject(t *testing.T, files map[string]string, extraConfig string) *config.Config {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - panel\nclaims_dir: claims\n" + extraConfig
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return loaded
}

// requireNoGitAbove fails the test when a .git exists at dir or above it: the
// rows that assert the git-free block would otherwise pass vacuously.
func requireNoGitAbove(t *testing.T, dir string) {
	t.Helper()
	if layout.InWorkTree(dir) {
		t.Fatalf("%s sits inside a git work tree; the no-git rows cannot be judged here", dir)
	}
}

// recoveryLines returns the indented block lines of a layout_legacy error.
func recoveryLines(t *testing.T, err error) []string {
	t.Helper()
	if err == nil {
		t.Fatal("expected a layout_legacy refusal, got nil")
	}
	coded := cliout.As(err)
	if coded == nil || coded.Code != cliout.CodeLayoutLegacy {
		t.Fatalf("expected error.code layout_legacy, got %v", err)
	}
	var lines []string
	for _, l := range strings.Split(coded.Message, "\n") {
		if strings.HasPrefix(l, "  ") {
			lines = append(lines, strings.TrimPrefix(l, "  "))
		}
	}
	return lines
}

// shPath is the POSIX shell the tracked-ness rows run the printed block in,
// resolved ONCE at package init, before any row empties PATH on purpose. It is
// looked up rather than spelled /bin/sh because the windows CI leg has no
// /bin/sh: there `sh` is Git for Windows' sh.exe, the same resolution
// tests/readme_setup_replay_test.go relies on. A host with no sh at all fails
// every row that runs the block, which is the point: running the block IS the
// check, and a row that could not run it has checked nothing.
var shPath, shErr = exec.LookPath("sh")

func runBlock(t *testing.T, dir string, lines []string, path string) (string, error) {
	t.Helper()
	if shErr != nil {
		t.Fatalf("no sh on PATH at package init, so the printed block cannot be run: %v", shErr)
	}
	script := "set -e\n" + strings.Join(lines, "\n") + "\n"
	cmd := exec.Command(shPath, "-c", script) // an absolute path: one row empties PATH on purpose
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + path, "HOME=" + dir, "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// coreutilsPath is the PATH the git-free rows run the block under: first (an
// empty temp dir) and then only the directories the block's mkdir, mv and rm
// are found in — /bin and /usr/bin on a POSIX host, sh.exe's own directory on
// windows (Git for Windows' usr/bin, where its coreutils live). Joined with the
// platform's list separator, never a literal colon.
func coreutilsPath(t *testing.T, first string) string {
	t.Helper()
	if shErr != nil {
		t.Fatalf("no sh on PATH at package init, so the printed block cannot be run: %v", shErr)
	}
	dirs := []string{first}
	if runtime.GOOS == "windows" {
		dirs = append(dirs, filepath.Dir(shPath))
	} else {
		dirs = append(dirs, "/bin", "/usr/bin")
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "fixture@example.invalid")
	gitIn(t, dir, "config", "user.name", "fixture")
}

// TestLayoutRefusesEachLegacyKindAlone pins that each of the seven kinds, on
// its own, refuses; that a fresh project proceeds; and that the one line each
// kind prints is the one (d) prescribes.
func TestLayoutRefusesEachLegacyKindAlone(t *testing.T) {
	requireNoGitAbove(t, t.TempDir())
	cases := []struct {
		file string
		want string
	}{
		{".dossierx-lock-store.json", "mv .dossierx-lock-store.json build/ledger/lock-store.json"},
		{".dossierx-comment-digest.json", "mv .dossierx-comment-digest.json build/ledger/comment-digest.json"},
		{".dossierx-flag-store.json", "mv .dossierx-flag-store.json build/ledger/flag-store.json"},
		{".build-order.widget.json", "mv .build-order.widget.json build/build-order/widget.json"},
		{".implementation.widget.json", "mv .implementation.widget.json build/code-links/widget.json"},
		{".catalog.json", "rm -f .catalog.json"},
		{"viewer/index.html", "rm -f viewer/index.html"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			cfg := legacyProject(t, map[string]string{tc.file: "{}"}, "")
			lines := recoveryLines(t, layout.Refuse(cfg, false))
			if !containsLine(lines, tc.want) {
				t.Fatalf("recovery for %s must contain %q, got %v", tc.file, tc.want, lines)
			}
			moves, err := layout.LegacyFiles(cfg)
			if err != nil || len(moves) != 1 || moves[0].From != tc.file {
				t.Fatalf("LegacyFiles = %+v, %v; want exactly %s", moves, err, tc.file)
			}
		})
	}
	t.Run("a fresh project proceeds", func(t *testing.T) {
		cfg := legacyProject(t, nil, "")
		if err := layout.Refuse(cfg, false); err != nil {
			t.Fatalf("a project with no legacy file must not be refused: %v", err)
		}
	})
}

// TestLayoutRecoveryOrderAndDetails pins the order of the printed block — the
// one sentence in the plan that decides it — and that the JSON details carry
// one {from, to, tracked} entry per printed move line, in the same order.
func TestLayoutRecoveryOrderAndDetails(t *testing.T) {
	requireNoGitAbove(t, t.TempDir())
	cfg := legacyProject(t, map[string]string{
		"viewer/index.html":             "<html>",
		".catalog.json":                 "{}",
		".implementation.panel.json":    "{}",
		".implementation.widget.json":   "{}",
		".build-order.widget.json":      "{}",
		".build-order.panel.json":       "{}",
		".dossierx-flag-store.json":     "{}",
		".dossierx-comment-digest.json": "{}",
		".dossierx-lock-store.json":     "{}",
	}, "")
	err := layout.Refuse(cfg, false)
	lines := recoveryLines(t, err)
	want := []string{
		"mkdir -p build/ledger build/build-order build/code-links",
		"mv .dossierx-lock-store.json build/ledger/lock-store.json",
		"mv .dossierx-comment-digest.json build/ledger/comment-digest.json",
		"mv .dossierx-flag-store.json build/ledger/flag-store.json",
		"mv .build-order.panel.json build/build-order/panel.json",
		"mv .build-order.widget.json build/build-order/widget.json",
		"mv .implementation.panel.json build/code-links/panel.json",
		"mv .implementation.widget.json build/code-links/widget.json",
		"rm -f .catalog.json",
		"rm -f viewer/index.html",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("block order:\n got %q\nwant %q", lines, want)
	}
	details, ok := cliout.As(err).Details.(map[string]any)
	if !ok {
		t.Fatalf("details = %#v, want a map with moves", cliout.As(err).Details)
	}
	moves, ok := details["moves"].([]layout.Move)
	if !ok || len(moves) != len(want)-1 {
		t.Fatalf("details.moves = %#v, want %d entries", details["moves"], len(want)-1)
	}
	for i, m := range moves {
		if !strings.Contains(want[i+1], m.From) || (!m.Regenerated && !strings.Contains(want[i+1], m.To)) {
			t.Fatalf("details.moves[%d] = %+v does not match printed line %q", i, m, want[i+1])
		}
		if m.Tracked {
			t.Fatalf("outside a work tree every move must be Tracked: false, got %+v", m)
		}
	}
	// The no-git form carries no git command and no "stage the moves" clause.
	msg := cliout.As(err).Message
	if strings.Contains(msg, "git") {
		t.Fatalf("outside a work tree the refusal must contain no git command:\n%s", msg)
	}
	if !strings.Contains(msg, "Move each one so its approvals stay beside its claims:") {
		t.Fatalf("the no-git sentence is missing:\n%s", msg)
	}
}

// TestLayoutBothPresentRefuses pins the half-done move: old and new both on
// disk is still a refusal, and it names both files.
func TestLayoutBothPresentRefuses(t *testing.T) {
	cfg := legacyProject(t, map[string]string{
		".dossierx-lock-store.json":    "{}",
		"build/ledger/lock-store.json": "{}",
	}, "")
	err := layout.Refuse(cfg, false)
	coded := cliout.As(err)
	if coded == nil || coded.Code != cliout.CodeLayoutLegacy {
		t.Fatalf("expected layout_legacy, got %v", err)
	}
	if !strings.Contains(coded.Message, "both build/ledger/lock-store.json and .dossierx-lock-store.json exist") {
		t.Fatalf("the half-done move must name both copies:\n%s", coded.Message)
	}
}

// TestLayoutViewerLineNamesOnlyTheFile: an authored viewer/template/ directory
// (viewer.template_overrides) must survive the recovery, so the viewer line
// names the one file and never the directory.
func TestLayoutViewerLineNamesOnlyTheFile(t *testing.T) {
	cfg := legacyProject(t, map[string]string{
		"viewer/index.html":         "<html>",
		"viewer/template/card.html": "<div>authored</div>",
	}, "viewer:\n  template_overrides: viewer/template\n")
	err := layout.Refuse(cfg, false)
	msg := cliout.As(err).Message
	lines := recoveryLines(t, err)
	if !containsLine(lines, "rm -f viewer/index.html") {
		t.Fatalf("expected the viewer line to name viewer/index.html, got %v", lines)
	}
	if strings.Contains(msg, "rm -r") || strings.Contains(msg, "viewer/template") {
		t.Fatalf("the recovery must not remove the viewer directory or name the authored partials:\n%s", msg)
	}
}

// TestLayoutHintWhenBuildDirIgnored: the appended hint and the replacement
// block, when the caller reports the build directory is ignored as well.
func TestLayoutHintWhenBuildDirIgnored(t *testing.T) {
	cfg := legacyProject(t, map[string]string{".dossierx-lock-store.json": "{}"}, "")
	msg := cliout.As(layout.Refuse(cfg, true)).Message
	if !strings.Contains(msg, layout.BuildDirIgnoredHint("build")) {
		t.Fatalf("missing the build/-is-ignored hint:\n%s", msg)
	}
	if !strings.Contains(msg, "!build/code-links/*") {
		t.Fatalf("the hint must be followed by the RecommendedGitignore block:\n%s", msg)
	}
	if !strings.Contains(cliout.As(layout.Refuse(cfg, false)).Message, "Then run: dossierx check --validate") ||
		strings.Contains(cliout.As(layout.Refuse(cfg, false)).Message, "is ignored by .gitignore") {
		t.Fatalf("without the verdict the hint must be absent")
	}
}

// The tracked-ness rows: each EXECUTES the printed block under `set -e` and
// requires exit 0 — running the block is the check.

func TestLayoutBlockRunsWhenEveryFileIsTracked(t *testing.T) {
	cfg := legacyProject(t, map[string]string{
		".dossierx-lock-store.json":     `{"version":2,"hashes":{},"locked_at":{},"ledger":{}}`,
		".dossierx-comment-digest.json": `{"version":1,"digests":{}}`,
		".build-order.widget.json":      "{}",
		".catalog.json":                 "{}",
		"viewer/index.html":             "<html>",
	}, "")
	root := cfg.Dir()
	gitInit(t, root)
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "baseline")

	moves, err := layout.LegacyFiles(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range moves {
		if !m.Tracked {
			t.Fatalf("every file is committed, so every move must be Tracked: %+v", m)
		}
	}
	lines := recoveryLines(t, layout.Refuse(cfg, false))
	if lines[0] != "mkdir -p build/ledger build/build-order" {
		t.Fatalf("mkdir line = %q", lines[0])
	}
	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "mv ") {
			t.Fatalf("a tracked file must move with git mv, got %q", l)
		}
	}
	if out, err := runBlock(t, root, lines, os.Getenv("PATH")); err != nil {
		t.Fatalf("the printed block failed under set -e: %v\n%s", err, out)
	}
	status := gitIn(t, root, "status", "--porcelain")
	for _, want := range []string{
		"R  .dossierx-lock-store.json -> build/ledger/lock-store.json",
		"R  .dossierx-comment-digest.json -> build/ledger/comment-digest.json",
		"R  .build-order.widget.json -> build/build-order/widget.json",
	} {
		if !strings.Contains(status, want) {
			t.Fatalf("expected %q in git status, got:\n%s", want, status)
		}
	}
	if err := layout.Refuse(cfg, false); err != nil {
		t.Fatalf("after the block ran the project must load: %v", err)
	}
}

func TestLayoutBlockMixesGitMvAndMvPerFile(t *testing.T) {
	cfg := legacyProject(t, map[string]string{
		".dossierx-lock-store.json":     `{"version":2,"hashes":{},"locked_at":{},"ledger":{}}`,
		".dossierx-comment-digest.json": `{"version":1,"digests":{}}`,
	}, "")
	root := cfg.Dir()
	gitInit(t, root)
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "baseline")
	// The ordinary state: a proposed-but-unlocked build order and a flag
	// store, both untracked.
	for _, f := range []string{".build-order.widget.json", ".dossierx-flag-store.json"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lines := recoveryLines(t, layout.Refuse(cfg, false))
	want := []string{
		"mkdir -p build/ledger build/build-order",
		"git mv .dossierx-lock-store.json build/ledger/lock-store.json",
		"git mv .dossierx-comment-digest.json build/ledger/comment-digest.json",
		"mv .dossierx-flag-store.json build/ledger/flag-store.json",
		"mv .build-order.widget.json build/build-order/widget.json",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("block:\n got %q\nwant %q", lines, want)
	}
	if out, err := runBlock(t, root, lines, os.Getenv("PATH")); err != nil {
		t.Fatalf("the mixed block failed under set -e: %v\n%s", err, out)
	}
	status := gitIn(t, root, "status", "--porcelain")
	renamed, untracked := 0, 0
	for _, l := range strings.Split(status, "\n") {
		switch {
		case strings.HasPrefix(l, "R ") && strings.Contains(l, "build/"):
			renamed++
		case strings.HasPrefix(l, "?? build/"):
			untracked++
		}
	}
	if renamed != 2 || untracked != 2 {
		t.Fatalf("expected two R lines and two ?? lines under build/, got %d and %d:\n%s", renamed, untracked, status)
	}
}

func TestLayoutBlockHasNoGitOutsideAWorkTree(t *testing.T) {
	cfg := legacyProject(t, map[string]string{
		".dossierx-lock-store.json": "{}",
		".build-order.widget.json":  "{}",
		".catalog.json":             "{}",
		"viewer/index.html":         "<html>",
	}, "")
	requireNoGitAbove(t, cfg.Dir())
	err := layout.Refuse(cfg, false)
	lines := recoveryLines(t, err)
	for _, l := range append(lines, cliout.As(err).Message) {
		if strings.Contains(l, "git") {
			t.Fatalf("outside a work tree the block must contain no git token: %q", l)
		}
	}
	empty := t.TempDir()
	if out, err := runBlock(t, cfg.Dir(), lines, coreutilsPath(t, empty)); err != nil {
		t.Fatalf("the git-free block failed: %v\n%s", err, out)
	}
	if err := layout.Refuse(cfg, false); err != nil {
		t.Fatalf("after the block ran the project must load: %v", err)
	}
}

func TestLayoutBlockHasNoGitWhenGitIsOffPath(t *testing.T) {
	cfg := legacyProject(t, map[string]string{
		".dossierx-lock-store.json": "{}",
		".build-order.widget.json":  "{}",
		".catalog.json":             "{}",
	}, "")
	root := cfg.Dir()
	gitInit(t, root)
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-qm", "baseline")
	t.Setenv("PATH", t.TempDir())
	moves, err := layout.LegacyFiles(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range moves {
		if m.Tracked || m.InWorkTree {
			t.Fatalf("with git off PATH every move must be Tracked: false, got %+v", m)
		}
	}
	lines := recoveryLines(t, layout.Refuse(cfg, false))
	for _, l := range lines {
		if strings.Contains(l, "git") {
			t.Fatalf("with git off PATH the block must contain no git token: %q", l)
		}
	}
	if out, err := runBlock(t, root, lines, coreutilsPath(t, t.TempDir())); err != nil {
		t.Fatalf("the git-free block failed: %v\n%s", err, out)
	}
}

// TestRecoveryTextNamesGitBashOnlyOnWindows: the shell hint is printed on
// windows and nowhere else, and it is prose, never one of the indented block
// lines a reader pastes.
func TestRecoveryTextNamesGitBashOnlyOnWindows(t *testing.T) {
	cfg := legacyProject(t, map[string]string{".dossierx-lock-store.json": "{}"}, "")
	err := layout.Refuse(cfg, false)
	lines := recoveryLines(t, err)
	msg := cliout.As(err).Message
	if got, want := strings.Contains(msg, layout.WindowsShellHint), runtime.GOOS == "windows"; got != want {
		t.Fatalf("on %s the refusal must name Git Bash: %v, got %v in:\n%s", runtime.GOOS, want, got, msg)
	}
	for _, l := range lines {
		if strings.Contains(l, "Git Bash") {
			t.Fatalf("the shell hint must not be a pasteable line: %q", l)
		}
	}
}

// TestEnsureBuildGitignoreWritesOnce pins the content in (c) and that an
// existing file is never rewritten.
func TestEnsureBuildGitignoreWritesOnce(t *testing.T) {
	cfg := legacyProject(t, nil, "")
	if err := layout.EnsureBuildGitignore(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cfg.BuildGitignorePath())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != layout.BuildGitignoreContent {
		t.Fatalf("build/.gitignore = %q, want %q", got, layout.BuildGitignoreContent)
	}
	for _, line := range []string{"catalog/", "viewer/", "*.lock", "*.tmp-*", "*.probe-*"} {
		if !strings.Contains(string(got), line+"\n") {
			t.Fatalf("build/.gitignore lacks %q", line)
		}
	}
	if err := os.WriteFile(cfg.BuildGitignorePath(), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := layout.EnsureBuildGitignore(cfg); err != nil {
		t.Fatal(err)
	}
	got, readErr := os.ReadFile(cfg.BuildGitignorePath())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "edited\n" {
		t.Fatalf("an existing build/.gitignore must never be rewritten, got %q", got)
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}
