// install_hook_scope_test.go pins the machine-wide disclosure the hook
// installer makes when core.hooksPath did not come from the repository it is
// being run against.
//
// A user with a global core.hooksPath (`git config --global core.hooksPath
// ~/.githooks` — an ordinary single-machine setup) who says yes to "add a
// pre-commit check" is having a hook written that fires on every commit, in
// every repository, on their whole machine. The installer used to write it
// and say nothing beyond naming the setting; a maintainer's ruling of 11 Aug
// 2026 on a v0.5.2 gate finding requires the scope to be said out loud. These
// tests hold the two halves of that ruling in place:
//
//   - a setting whose origin is NOT this repository's own config produces the
//     "EVERY git repository on this machine" disclosure, naming the config
//     file it came from;
//   - a setting that IS this repository's own produces no such disclosure —
//     including when the installer is pointed at a SUBDIRECTORY via --repo,
//     which is the exact shape that once false-positived when the origin was
//     anchored on $PWD instead of the work-tree top;
//   - the config file the disclosure names is named by its RAW path, not by
//     git's C-quoted rendering of it, because the path is the one fact that
//     lets the reader verify or undo the setting, and the quoted rendering
//     exists nowhere on their disk.
//
// The environment is hermetic the same way hook_hostile_paths_test.go's is:
// GIT_CONFIG_NOSYSTEM plus an explicit GIT_CONFIG_GLOBAL, so the machine's own
// configuration can neither trigger nor suppress the disclosure under test.
package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// machineWideSentence is the load-bearing phrase of the disclosure. The test
// matches this one phrase rather than the whole paragraph so that rewording
// around it stays free, while the fact being disclosed — the hook's reach —
// cannot be dropped without a red test.
const machineWideSentence = "EVERY git\n      repository on this machine"

// runInstallerWithGlobal runs the installer with --yes in repo, under a global
// git config at globalConfig, and returns combined output and exit code.
func runInstallerWithGlobal(t *testing.T, repo, globalConfig string, args ...string) (output string, code int) {
	t.Helper()
	shell := hostileShell(t)
	cmd := exec.Command(shell, append([]string{hostileInstaller(t)}, args...)...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+globalConfig,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("could not run the installer in %s: %v", repo, err)
		}
		code = exitErr.ExitCode()
	}
	return string(out), code
}

// newScopeRepo builds a bare-bones git repository (no dossierx project — the
// installer writes the hook regardless, and these tests are about what it
// SAYS, not what the hook later gates).
func newScopeRepo(t *testing.T, globalConfig string) string {
	t.Helper()
	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-q", ".")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+globalConfig)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return repo
}

func TestInstallHookDisclosesAMachineWideHooksPath(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "everywhere-hooks")
	globalConfig := filepath.Join(t.TempDir(), "gitconfig")
	// hooksPath is written with forward slashes because the value crosses
	// git's own config parser, and on Windows the test's backslashed temp path
	// would be an escaping question this test is not about.
	if err := os.WriteFile(globalConfig,
		[]byte("[core]\n\thooksPath = "+filepath.ToSlash(hooksDir)+"\n"), 0o644); err != nil {
		t.Fatalf("write the global gitconfig: %v", err)
	}
	repo := newScopeRepo(t, globalConfig)

	out, code := runInstallerWithGlobal(t, repo, globalConfig, "--yes")
	if code != 0 {
		t.Fatalf("the installer exited %d under a global core.hooksPath:\n%s", code, out)
	}
	if !strings.Contains(out, machineWideSentence) {
		t.Errorf("a hook was just installed into a GLOBAL core.hooksPath directory and the installer never said it runs machine-wide.\nWanted the disclosure containing %q.\nGot:\n%s",
			machineWideSentence, out)
	}
	if !strings.Contains(out, filepath.ToSlash(globalConfig)) && !strings.Contains(out, globalConfig) {
		t.Errorf("the disclosure does not name the config file the setting came from (%s), which is the one fact that lets the reader verify or undo it:\n%s", globalConfig, out)
	}
	// The write itself must still have happened — the disclosure accompanies
	// the install, it does not veto it: a global hooks path is a legitimate
	// setup and refusing it would repoint nothing and help nobody.
	if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); err != nil {
		t.Errorf("the hook was not written into the configured global hooks directory: %v", err)
	}
}

func TestInstallHookStaysQuietOverThisRepositorysOwnHooksPath(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "gitconfig-empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write the empty global gitconfig: %v", err)
	}
	repo := newScopeRepo(t, empty)
	if err := os.MkdirAll(filepath.Join(repo, "team-hooks"), 0o755); err != nil {
		t.Fatalf("mkdir team-hooks: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	cmd := exec.Command("git", "config", "core.hooksPath", "team-hooks")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+empty)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config core.hooksPath: %v\n%s", err, out)
	}

	// Once from the repository root, and once pointed at a SUBDIRECTORY with
	// --repo: the second is the invocation that false-positived when origin
	// paths were anchored on $PWD, because `git config --show-origin` prints
	// `file:.git/config` relative to the work-tree top from everywhere.
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "from the root", args: []string{"--yes"}},
		{name: "via --repo sub", args: []string{"--repo", filepath.Join(repo, "sub"), "--yes"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := runInstallerWithGlobal(t, repo, empty, tc.args...)
			if code != 0 {
				t.Fatalf("the installer exited %d under a repo-local core.hooksPath:\n%s", code, out)
			}
			if strings.Contains(out, machineWideSentence) {
				t.Errorf("a repository-scoped core.hooksPath was announced as machine-wide, which is the false positive pointing the other way — crying wolf here is how the real disclosure gets ignored:\n%s", out)
			}
			// The note that names the setting must still print: staying quiet
			// about SCOPE is not staying quiet about the setting.
			if !strings.Contains(out, "core.hooksPath is set to") {
				t.Errorf("the core.hooksPath note itself disappeared; the scope classification was meant to extend it, not replace it:\n%s", out)
			}
		})
	}
}

// cQuotedOriginCase is one entry of the C-quoting corpus below. The defect it
// exists for is PATH-SHAPED, not platform-shaped: `git config --show-origin`
// C-quotes an origin containing a double quote or a backslash UNCONDITIONALLY
// (quote.c's quote_c_style — the same layer that broke the hook body's
// discovery and moved it to `ls-files -z`, see install-git-hook.sh's v8
// header). Windows merely guarantees the trigger, because there the origin is
// an absolute native path and the platform's own separator IS the backslash.
//
// The per-OS split is a DECLARATION, exactly as hook_hostile_paths_test.go's
// table declares its: `"` and `\` are illegal in Windows FILE NAMES
// (CreateFile refuses them), so the hostile-directory case is declared for
// POSIX only — never probed or discovered at run time — and Windows declares
// the case it alone can supply, the separator that appears in every absolute
// path the OS produces. Each case ends by proving its trigger bytes are
// actually in the path, and the parent test's last assertion is that the
// executed set equals the declared set, so a case that could not start fails
// the run rather than shrinking coverage silently. There is no t.Skip here.
type cQuotedOriginCase struct {
	name string
	// dir is the hostile directory component the global config is placed
	// under; empty means the platform's own separator supplies the trigger.
	dir string
	// onWindows declares which OS the case belongs to: true means it runs
	// ONLY on Windows, false means it runs everywhere else.
	onWindows bool
}

var cQuotedOriginCases = []cQuotedOriginCase{
	// The two bytes git C-quotes unconditionally that a POSIX filesystem will
	// store. A tab would trigger the same quoting but adds no third quoting
	// layer this test could tell apart from these two.
	{name: "quote and backslash in the directory", dir: `we"ird\dir`, onWindows: false},
	// No hostile component at all: t.TempDir on Windows is already a
	// backslashed absolute path, which is the exact shape both Windows CI
	// legs failed on ("C:\\Users\\RUNNER~1\\..." in the disclosure).
	{name: "the platform separator itself", dir: "", onWindows: true},
}

// TestInstallHookDisclosureNamesTheOriginRaw pins the third half-ruling of
// the file header: when git would C-quote the origin, the disclosure still
// names the config file by the path that exists on disk. Before the fix the
// installer read the origin from `git config --show-origin`'s default output
// and printed the quoted rendering — backslashes doubled, wrapping quotes
// baked in — which Contains() below correctly refuses to recognise as the
// real path (the doubling means the raw path is not a substring of its own
// quoted form, in either slash direction).
func TestInstallHookDisclosureNamesTheOriginRaw(t *testing.T) {
	declared := make([]string, 0, len(cQuotedOriginCases))
	executed := make(map[string]bool, len(cQuotedOriginCases))
	for _, tc := range cQuotedOriginCases {
		if (runtime.GOOS == "windows") != tc.onWindows {
			// Excluded BY DECLARATION: the hostile directory cannot exist on
			// Windows, and the separator case is vacuous anywhere else — a
			// POSIX temp path carries no backslash, so it would assert the
			// disclosure survives quoting git never applies.
			continue
		}
		declared = append(declared, tc.name)
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			executed[tc.name] = true

			configDir := t.TempDir()
			if tc.dir != "" {
				configDir = filepath.Join(configDir, tc.dir)
				if err := os.MkdirAll(configDir, 0o755); err != nil {
					t.Fatalf("could not create the hostile config directory %q: %v (if an OS refuses this name the case belongs in the table's declarations, not in a runtime probe here)", configDir, err)
				}
			}
			globalConfig := filepath.Join(configDir, "gitconfig")

			// The probe that keeps the corpus honest: the trigger bytes must
			// actually be in the path git will print as the origin, or every
			// assertion below "passes" over an origin git had no reason to
			// quote. Windows only guarantees the backslash; the POSIX case
			// declared both bytes and must carry both.
			if !strings.Contains(globalConfig, `\`) {
				t.Fatalf("the global config path %q carries no backslash, so git will not C-quote its origin and this case can detect nothing", globalConfig)
			}
			if tc.dir != "" && !strings.Contains(globalConfig, `"`) {
				t.Fatalf("the global config path %q lost the double quote the case declared; it would be exercising a weaker trigger than it reports", globalConfig)
			}

			hooksDir := filepath.Join(t.TempDir(), "everywhere-hooks")
			// Forward slashes for the VALUE, as in the disclosure test above:
			// the value crosses git's config parser, and escaping it is that
			// parser's question, not this test's. The config FILE's own path
			// crosses no parser — it travels raw in GIT_CONFIG_GLOBAL.
			if err := os.WriteFile(globalConfig,
				[]byte("[core]\n\thooksPath = "+filepath.ToSlash(hooksDir)+"\n"), 0o644); err != nil {
				t.Fatalf("write the global gitconfig: %v", err)
			}
			repo := newScopeRepo(t, globalConfig)

			out, code := runInstallerWithGlobal(t, repo, globalConfig, "--yes")
			if code != 0 {
				t.Fatalf("the installer exited %d under a global core.hooksPath at a C-quoting-triggering path:\n%s", code, out)
			}
			// The setting is global, so the disclosure itself must fire; a
			// C-quoted origin fails the is-it-this-repository comparison in
			// the machine-wide direction, so this half passing while the next
			// fails is exactly the defect's signature.
			if !strings.Contains(out, machineWideSentence) {
				t.Errorf("no machine-wide disclosure printed over a global core.hooksPath.\nWanted the disclosure containing %q.\nGot:\n%s", machineWideSentence, out)
			}
			if !strings.Contains(out, globalConfig) && !strings.Contains(out, filepath.ToSlash(globalConfig)) {
				t.Errorf("the disclosure does not name the config file by its real path (%s) — git C-quotes an origin containing a quote or a backslash, and the quoted rendering names a file that exists nowhere on the reader's disk, so they can neither verify nor undo the setting:\n%s", globalConfig, out)
			}
			// The write must still have happened: the disclosure accompanies
			// the install, it does not veto it.
			if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); err != nil {
				t.Errorf("the hook was not written into the configured global hooks directory: %v", err)
			}
		})
	}

	got := make([]string, 0, len(executed))
	for name := range executed {
		got = append(got, name)
	}
	sort.Strings(got)
	want := append([]string(nil), declared...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the executed case set does not equal the declared case set for %s.\n  declared: %v\n  executed: %v\nEvery declared case must run: a case that could not start is missing coverage, not a pass", runtime.GOOS, want, got)
	}
	if len(want) == 0 {
		t.Fatalf("zero cases are declared for %s, so this test asserted nothing at all; the table must declare a case for every OS — the defect is path-shaped, and every OS has a path shape that triggers it", runtime.GOOS)
	}
}
