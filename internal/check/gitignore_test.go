package check_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/layout"
)

// gitignoreProject writes a two-module project (no claims are needed: the
// guard reads the config's module list and git, nothing else) and returns
// its config.
func gitignoreProject(t *testing.T, root string) *config.Config {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(root, config.FileName), "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - panel\nclaims_dir: claims\n")
	cfg, err := config.LoadConfig(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

// gitignoreRepo is gitignoreProject inside a fresh git repository whose
// .gitignore holds ignore.
func gitignoreRepo(t *testing.T, ignore string) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := gitignoreProject(t, root)
	gitRepo(t, root)
	writeFixtureFile(t, filepath.Join(root, ".gitignore"), ignore)
	return cfg
}

// TestGitignored_BareBuildPatternIsOneFindingPerPath: `.gitignore` = "build/"
// yields one finding per checked path — the three ledger stores, both
// artifacts of each module, and build/.gitignore LAST — each naming the
// pattern and its line, in the engine's order. The order is load-bearing: the
// approval verbs print findings[0] as the store_gitignored refusal, so the
// first finding must be the lock ledger's, whose harm clause is the plan's
// verbatim one. Each finding's harm clause is its own kind's — the sentence
// true of the lock ledger (check fails with lock-ledger-absent) is false of a
// build order, the flag store and build/.gitignore, and a refutable harm is
// the report that teaches people to bypass the gate.
func TestGitignored_BareBuildPatternIsOneFindingPerPath(t *testing.T) {
	cfg := gitignoreRepo(t, "build/\n")
	findings, warnings, reason, err := check.Gitignored(cfg)
	if err != nil || reason != "" {
		t.Fatalf("expected a verdict, got reason %q err %v", reason, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("nothing is tracked, so nothing may be a warning: %v", warnings)
	}
	want := []string{
		"build/ledger/lock-store.json",
		"build/ledger/comment-digest.json",
		"build/ledger/flag-store.json",
		"build/build-order/widget.json",
		"build/code-links/widget.json",
		"build/build-order/panel.json",
		"build/code-links/panel.json",
		"build/.gitignore",
	}
	// The harm clause each kind must carry, and the negation caveat derived
	// from the reported path rather than hard-coded to the ledger.
	harm := map[string][]string{
		"build/ledger/lock-store.json":     {"so the lock ledger never reaches the repository", "check fails there with lock-ledger-absent", `so "!build/ledger/" alone does nothing`},
		"build/ledger/comment-digest.json": {"so the comment digest never reaches the repository", "comment-digest-absent", `so "!build/ledger/" alone does nothing`},
		"build/ledger/flag-store.json":     {"so the flag store never reaches the repository", "claim reaudit there finds no pending flag", `so "!build/ledger/" alone does nothing`},
		"build/build-order/widget.json":    {`so module "widget"'s build order never reaches the repository`, `build-order-ledger-abandoned`, `module "widget"`, `so "!build/build-order/" alone does nothing`},
		"build/code-links/widget.json":     {`so module "widget"'s code links never reaches the repository`, "prints no code-link status", `so "!build/code-links/" alone does nothing`},
		"build/build-order/panel.json":     {`module "panel"`, `build-order-ledger-abandoned`},
		"build/code-links/panel.json":      {`module "panel"`, "prints no code-link status"},
		"build/.gitignore":                 {"so the build directory's own .gitignore never reaches the repository", "rewrites the file from the default", `so "!build/.gitignore" alone does nothing`},
	}
	var got []string
	for _, f := range findings {
		if f.Rule != check.RuleStoreGitignored || f.ClaimID != "" {
			t.Fatalf("finding must be project-scoped store-gitignored, got %+v", f)
		}
		p := strings.SplitN(f.Message, " ", 2)[0]
		got = append(got, p)
		if !strings.Contains(f.Message, `pattern "build/" at .gitignore:1`) {
			t.Fatalf("finding must name the pattern and line: %s", f.Message)
		}
		for _, clause := range harm[p] {
			if !strings.Contains(f.Message, clause) {
				t.Fatalf("finding for %s must state its own kind's harm (missing %q): %s", p, clause, f.Message)
			}
		}
		if p != "build/ledger/lock-store.json" && strings.Contains(f.Message, "check fails there with lock-ledger-absent") {
			t.Fatalf("finding for %s states the lock ledger's harm, which is false of it: %s", p, f.Message)
		}
		// The block is printed indented by two spaces, line for line.
		for _, line := range strings.Split(layout.RecommendedGitignore, "\n") {
			if !strings.Contains(f.Message, "\n  "+line+"\n") && !strings.HasSuffix(f.Message, "\n  "+line) {
				t.Fatalf("finding must print the replacement block verbatim (missing %q): %s", line, f.Message)
			}
		}
		if !strings.Contains(f.Message, "set build_dir in project.config.yaml") {
			t.Fatalf("finding must name build_dir as the second recovery: %s", f.Message)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("findings for paths %v, want %v", got, want)
	}
}

// TestGitignored_ForceAddedLedgerIsAWarningAndItsSiblingsStayFindings pins
// file-level, --no-index checking: with build/ ignored and the lock store
// force-added, the finding STILL fires for every sibling, while the tracked
// store itself is a warning naming the pattern and build_dir.
func TestGitignored_ForceAddedLedgerIsAWarningAndItsSiblingsStayFindings(t *testing.T) {
	cfg := gitignoreRepo(t, "build/\n")
	if err := os.MkdirAll(filepath.Dir(cfg.LockStorePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, cfg.LockStorePath(), `{"version":2,"hashes":{},"locked_at":{},"ledger":{}}`)
	git(t, cfg.Dir(), "add", "-f", config.LockStoreDisplayPath)

	findings, warnings, reason, err := check.Gitignored(cfg)
	if err != nil || reason != "" {
		t.Fatalf("expected a verdict, got reason %q err %v", reason, err)
	}
	var paths []string
	for _, f := range findings {
		paths = append(paths, strings.SplitN(f.Message, " ", 2)[0])
	}
	for _, sibling := range []string{"build/ledger/flag-store.json", "build/ledger/comment-digest.json", "build/build-order/widget.json", "build/build-order/panel.json", "build/code-links/widget.json"} {
		if !containsStr(paths, sibling) {
			t.Fatalf("a directory-level check goes green here; %s must still be a finding, got %v", sibling, paths)
		}
	}
	if containsStr(paths, "build/ledger/lock-store.json") {
		t.Fatalf("the force-added ledger reaches every collaborator and must not be a finding: %v", paths)
	}
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "build/ledger/lock-store.json is in the repository but matched by .gitignore pattern \"build/\" (.gitignore:1)") || !strings.Contains(warnings[0], "build_dir") {
		t.Fatalf("expected one ignored-but-tracked warning naming the pattern and build_dir, got %v", warnings)
	}
}

// TestGitignored_RecommendedBlockPassesWithNoBuildDirOnDisk: the exact
// replacement block, in a repo where build/ does not exist yet (the reference
// client never creates build/code-links/), reports every path not-ignored.
func TestGitignored_RecommendedBlockPassesWithNoBuildDirOnDisk(t *testing.T) {
	cfg := gitignoreRepo(t, layout.RecommendedGitignore+"\n")
	if _, err := os.Stat(cfg.BuildDirPath()); !os.IsNotExist(err) {
		t.Fatalf("precondition: build/ must not exist on disk (stat err=%v)", err)
	}
	findings, warnings, reason, err := check.Gitignored(cfg)
	if err != nil || reason != "" || len(findings) != 0 || len(warnings) != 0 {
		t.Fatalf("the recommended block must pass cleanly: findings=%v warnings=%v reason=%q err=%v", findings, warnings, reason, err)
	}
}

// TestGitignored_NegatedOnlyMatchIsNotReported: a path matched only by a
// negation is not ignored, and -v alone would have said otherwise.
func TestGitignored_NegatedOnlyMatchIsNotReported(t *testing.T) {
	cfg := gitignoreRepo(t, "!build/ledger/lock-store.json\n")
	findings, warnings, reason, err := check.Gitignored(cfg)
	if err != nil || reason != "" || len(findings) != 0 || len(warnings) != 0 {
		t.Fatalf("a negation-only match must not be reported: findings=%v warnings=%v reason=%q err=%v", findings, warnings, reason, err)
	}
}

// TestGitignored_OutsideAWorkTreeIsAReasonEvenWithoutGit: no .git above the
// project (the test walks up and fatals if it finds one) is "not a work tree"
// — decided without git, so an empty PATH changes nothing.
func TestGitignored_OutsideAWorkTreeIsAReasonEvenWithoutGit(t *testing.T) {
	root := t.TempDir()
	if layout.InWorkTree(root) {
		t.Fatalf("%s sits inside a git work tree; this row cannot be judged here", root)
	}
	cfg := gitignoreProject(t, root)
	t.Setenv("PATH", t.TempDir())
	findings, warnings, reason, err := check.Gitignored(cfg)
	if err != nil || reason != check.GitignoreNotAWorkTree || len(findings) != 0 || len(warnings) != 0 {
		t.Fatalf("expected reason %q with no findings, got findings=%v warnings=%v reason=%q err=%v", check.GitignoreNotAWorkTree, findings, warnings, reason, err)
	}
}

// TestGitignored_GitOffPathInsideAWorkTreeIsAnError: a .git exists but git
// cannot be found — never a clean verdict.
func TestGitignored_GitOffPathInsideAWorkTreeIsAnError(t *testing.T) {
	cfg := gitignoreRepo(t, "build/\n")
	t.Setenv("PATH", t.TempDir())
	findings, _, reason, err := check.Gitignored(cfg)
	if err == nil || !errors.Is(err, check.ErrGitUnavailable) || reason != check.GitignoreGitNotAvailable {
		t.Fatalf("expected the git-unavailable error, got findings=%v reason=%q err=%v", findings, reason, err)
	}
	if !strings.Contains(err.Error(), "cannot tell whether build/ledger/lock-store.json is ignored") {
		t.Fatalf("the error must carry the verbatim refusal text: %v", err)
	}
}

// TestGitignored_BareRepositoryIsAnError pins the newGitRunner arm: a .git
// that is a BARE repository makes rev-parse say "false" before any
// check-ignore runs, and that is not the exit-0 escape hatch here.
func TestGitignored_BareRepositoryIsAnError(t *testing.T) {
	root := t.TempDir()
	cfg := gitignoreProject(t, root)
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is required for this row: %v", err)
	}
	git(t, root, "init", "-q", "--bare", filepath.Join(root, ".git"))
	if !layout.InWorkTree(root) {
		t.Fatalf("precondition: the git-free walk must find the bare .git")
	}
	findings, _, reason, err := check.Gitignored(cfg)
	if err == nil || !errors.Is(err, check.ErrGitUnavailable) || reason != check.GitignoreGitNotAvailable || len(findings) != 0 {
		t.Fatalf("a bare repository must be the git-unavailable error, never a clean verdict: findings=%v reason=%q err=%v", findings, reason, err)
	}
}

// TestGitignored_CheckIgnoreExit128IsAnError: a stub git first on PATH that
// exits 128 on check-ignore and passes everything else through must be
// reported, not read as "not ignored" — the row that pins runStatus.
//
// The stub is THIS TEST BINARY copied to <tmp>/git (git.exe on windows) and
// re-executed with stubGitEnv set, which TestMain turns into stubGitMain. Not a
// shell script named `git`: the windows CI leg resolves executables by PATHEXT,
// so an extensionless script is never found there, the real git answers, the
// verdict is clean and this row fails for a reason that has nothing to do with
// runStatus. One binary, one code path, the same on every platform.
func TestGitignored_CheckIgnoreExit128IsAnError(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("git is required for this row: %v", err)
	}
	cfg := gitignoreRepo(t, "build/\n")
	stubDir := t.TempDir()
	stubName := "git"
	if runtime.GOOS == "windows" {
		stubName = "git.exe"
	}
	copyExecutable(t, filepath.Join(stubDir, stubName))
	t.Setenv(stubGitEnv, realGit)
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got, err := exec.LookPath("git"); err != nil || got != filepath.Join(stubDir, stubName) {
		t.Fatalf("precondition: PATH must resolve git to the stub, got %q, %v", got, err)
	}
	findings, _, reason, err := check.Gitignored(cfg)
	if err == nil || !errors.Is(err, check.ErrGitUnavailable) || reason != check.GitignoreGitNotAvailable || len(findings) != 0 {
		t.Fatalf("a check-ignore that exits 128 must be the git-unavailable error, never a verdict: findings=%v reason=%q err=%v", findings, reason, err)
	}
	if !strings.Contains(err.Error(), "exited 128") {
		t.Fatalf("the error must carry git's exit status: %v", err)
	}
}

// TestGitignored_BuildDirAboveTheTopLevelIsAReason: a build_dir resolving
// above the repository's top level passes with a reason and one warning.
func TestGitignored_BuildDirAboveTheTopLevelIsAReason(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(repo, config.FileName), "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\nbuild_dir: ../out\n")
	cfg, err := config.LoadConfig(filepath.Join(repo, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	gitRepo(t, repo)
	writeFixtureFile(t, filepath.Join(repo, ".gitignore"), "build/\n")
	findings, warnings, reason, err := check.Gitignored(cfg)
	if err != nil || reason != check.GitignoreOutsideWorkTree || len(findings) != 0 {
		t.Fatalf("expected reason %q with no findings, got findings=%v reason=%q err=%v", check.GitignoreOutsideWorkTree, findings, reason, err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "build_dir resolves outside the repository") {
		t.Fatalf("expected the outside-the-repository warning, got %v", warnings)
	}
}

// TestGitignored_MonorepoPatternAboveTheProjectNamesAProjectNegation: the
// project at docs/dx under a ROOT `build/` — pasting the block into the root
// .gitignore would un-ignore every other subproject's build output, so the
// finding leads with build_dir and offers the one negation for THIS project.
func TestGitignored_MonorepoPatternAboveTheProjectNamesAProjectNegation(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "docs", "dx")
	cfg := gitignoreProject(t, project)
	gitRepo(t, root)
	writeFixtureFile(t, filepath.Join(root, ".gitignore"), "build/\n")
	findings, _, reason, err := check.Gitignored(cfg)
	if err != nil || reason != "" || len(findings) == 0 {
		t.Fatalf("expected findings, got findings=%v reason=%q err=%v", findings, reason, err)
	}
	msg := findings[0].Message
	if !strings.Contains(msg, "!docs/dx/build/") {
		t.Fatalf("the finding must name the per-project negation: %s", msg)
	}
	if strings.Contains(msg, "!build/code-links/*") {
		t.Fatalf("the finding must NOT print the eight-line block for a pattern above the project: %s", msg)
	}
	if !strings.Contains(msg, "../../.gitignore:1") {
		t.Fatalf("the finding must name the pattern source as seen from the project directory: %s", msg)
	}
}

// TestGitignored_GlobalExcludesFileIsNamedAsGitReportedIt: a `build/` pattern
// in a core.excludesFile. git names that source by its ABSOLUTE path; joining
// it onto the top level would print a file that does not exist and, because
// the mangled path lands inside the project, offer the eight-line block for a
// machine-wide file — pasted there, it un-ignores every other project's build
// output on the machine. The finding must name the file git named, say it is
// machine-wide, and offer the project's own "!build/" negation instead.
func TestGitignored_GlobalExcludesFileIsNamedAsGitReportedIt(t *testing.T) {
	root := t.TempDir()
	cfg := gitignoreProject(t, root)
	gitRepo(t, root)
	excludes := filepath.Join(t.TempDir(), "globalignore")
	writeFixtureFile(t, excludes, "build/\n")
	git(t, root, "config", "core.excludesFile", excludes)
	findings, _, reason, err := check.Gitignored(cfg)
	if err != nil || reason != "" || len(findings) == 0 {
		t.Fatalf("expected findings, got findings=%v reason=%q err=%v", findings, reason, err)
	}
	msg := findings[0].Message
	if !strings.HasPrefix(msg, "build/ledger/lock-store.json is ignored") {
		t.Fatalf("the first finding must be the lock ledger's: %s", msg)
	}
	// The source printed between "at " and ":1)" must be the file git named,
	// openable as printed (absolute), not a path relocated into the project.
	_, after, ok := strings.Cut(msg, " at ")
	if !ok {
		t.Fatalf("finding must name the pattern source: %s", msg)
	}
	source, _, ok := strings.Cut(after, ":1)")
	if !ok {
		t.Fatalf("finding must name the pattern's line: %s", msg)
	}
	if !filepath.IsAbs(source) {
		t.Fatalf("a core.excludesFile source must be printed absolute, as git reported it, got %q: %s", source, msg)
	}
	if _, statErr := os.Stat(source); statErr != nil {
		t.Fatalf("the source the finding names must exist as printed (%v): %s", statErr, msg)
	}
	if filepath.Base(source) != "globalignore" {
		t.Fatalf("the finding must name the excludes file, got %q: %s", source, msg)
	}
	if !strings.Contains(msg, "core.excludesFile") {
		t.Fatalf("the finding must say the pattern is machine-wide: %s", msg)
	}
	if !strings.Contains(msg, "\n\n  !build/\n\n") {
		t.Fatalf("the finding must offer the project's own !build/ negation: %s", msg)
	}
	if strings.Contains(msg, "!build/code-links/*") {
		t.Fatalf("the finding must NOT print the eight-line block for a machine-wide file: %s", msg)
	}
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// stubGitEnv, when set, makes a re-execution of this test binary behave as the
// stub git that TestGitignored_CheckIgnoreExit128IsAnError puts first on PATH:
// its value is the path of the real git every other invocation is passed to.
const stubGitEnv = "DOSSIERX_TEST_STUB_GIT"

// TestMain routes the stub re-execution before the testing flags are parsed
// (git's arguments are not test flags) and runs the package's tests otherwise.
func TestMain(m *testing.M) {
	if realGit := os.Getenv(stubGitEnv); realGit != "" {
		os.Exit(stubGitMain(realGit, os.Args[1:]))
	}
	os.Exit(m.Run())
}

// stubGitMain exits 128 with git's own work-tree message on any check-ignore
// invocation and hands every other invocation to the real git, stdio and exit
// status intact, so rev-parse answers as it would and only runStatus's exit-128
// arm is exercised.
func stubGitMain(realGit string, args []string) int {
	for _, a := range args {
		if a == "check-ignore" {
			fmt.Fprintln(os.Stderr, "fatal: this operation must be run in a work tree")
			return 128
		}
	}
	cmd := exec.Command(realGit, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// copyExecutable copies the running test binary to dst, executable.
func copyExecutable(t *testing.T, dst string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	in, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}
