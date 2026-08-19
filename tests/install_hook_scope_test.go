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
//     anchored on $PWD instead of the work-tree top.
//
// The environment is hermetic the same way hook_hostile_paths_test.go's is:
// GIT_CONFIG_NOSYSTEM plus an explicit GIT_CONFIG_GLOBAL, so the machine's own
// configuration can neither trigger nor suppress the disclosure under test.
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
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
func runInstallerWithGlobal(t *testing.T, repo, globalConfig string, args ...string) (string, int) {
	t.Helper()
	shell := hostileShell(t)
	cmd := exec.Command(shell, append([]string{hostileInstaller(t)}, args...)...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+globalConfig,
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
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
