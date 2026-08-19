// WHAT THIS FILE ESTABLISHES, AND WHY IT IS GO AND NOT MORE SHELL.
//
// scripts/hook-smoke-test.sh proves the pre-commit gate's behaviour over
// twenty-one fixtures, and every one of them lives at a tame path: ASCII, no
// quoting metacharacters, one accented "café" as the single non-ASCII case.
// The three defects that motivated this file were all found by READING the
// installer, never by running it, because nothing ever ran it anywhere a path
// looked like a real user's machine: a Windows user named O'Brien, a mount
// with a space in it, a directory a colleague really did name with a `$`.
//
// It is a Go test rather than a 22nd shell case for an accounting reason the
// shell cannot satisfy: the `hooks` CI job runs a script, and a script emits
// no countable per-test record — tests/ci_run_evidence_test.go says in its
// header that a smoke test which degenerates into asserting nothing still
// reaches a release as a green conclusion. This file runs inside the root
// `go test -race -json ./...` step on all three platforms of the `test`
// matrix, so every case here is a named entry in the account the release
// gate's evidence stage counts, and a case that silently stopped executing is
// a missing entry rather than a quieter shade of green.
//
// THE FIXTURE SHAPE, and why the hostile name appears at TWO levels. Each
// case builds a repository whose PARENT directory carries the hostile name,
// and places the project under a directory of the same hostile name INSIDE
// the repository. The two levels detect different defect classes and neither
// substitutes for the other:
//
//   - the parent directory puts the hostile bytes into every ABSOLUTE path
//     the installer computes and prints — the hooks directory, the target,
//     the remediation lines a user is told to run. That is what breaks when
//     printed shell lines interpolate a path into quoting that cannot carry
//     it (the replay test below is the detector for that).
//   - the in-repo directory puts the hostile bytes into the TRACKED path of
//     project.config.yaml, which is what the hook's discovery reads back out
//     of `git ls-files`. git C-quotes output paths — and core.quotepath=false
//     lifts that only for bytes above ASCII; a `"`, a `\` or a control
//     character is quoted UNCONDITIONALLY (see quote.c's quote_c_style),
//     so the hook's discovery hands `dossierx --config` a C-quoted string
//     that names no file, gets config_not_found for a config it is holding,
//     and takes the refusal branch. Every commit, on every branch, until
//     somebody uninstalls the gate. The `quote` case is the direct detector.
//
// WHAT THE CASE TABLE DECLARES, AND HOW EXCLUSION WORKS. The per-OS split is
// a DECLARATION in the table, not a discovery at run time: `"`, `\` and a
// literal tab are illegal in Windows file names (CreateFile refuses them), so
// those cases are declared POSIX-only, and everything else runs everywhere.
// The last assertion of each test is that the set of cases that EXECUTED
// equals the set the table declares for this OS. A case that could not run —
// a filesystem that transliterated the name, a shell that is missing, a
// narrowed `-run` selector that deselected a subtest — therefore fails the
// parent test rather than shrinking coverage silently, which is CLAUDE.md's
// "a skip is a failure" applied to a corpus. There is no t.Skip in this file.
//
// WHAT IS ASSERTED PER CASE, all on exit codes and file bytes, never on
// message text (prose is rewordable; bytes and exit codes are the contract):
//
//  1. the installed hook is byte-identical to `--print-hook` output;
//  2. core.hooksPath is unchanged across the install (bytes AND exit code
//     of `git config --get`, so "unset before, unset after" is checked as
//     exactly that rather than as two empty strings that happen to agree);
//  3. an honest commit exits 0 — the half that fails when the gate starts
//     refusing everything, which is precisely what defect (3) above does;
//  4. a commit carrying a hand-edited LOCKED claim exits non-zero — the
//     half that fails when the gate silently skips the project it cannot
//     name, so neither half alone means anything;
//  5. `git status --porcelain` is byte-identical before and after the
//     refusal: the hook reads the index and must never dirty the tree it
//     judges.
//
// WHAT THIS FILE CANNOT PROMISE. It runs the installer under the sh this
// machine has, against paths this machine's filesystem accepts. It cannot
// prove anything about WSL's bash launcher (that is
// scripts/install-git-hook.Tests.ps1's subject, with its own stated limits),
// and it cannot prove behaviour under path encodings the platform refuses to
// store — those cases are excluded by declaration, visibly, in the table.
package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// hostileClaimID mirrors scripts/hook-smoke-test.sh's CLAIM_ID so a reader can
// line the two suites up; the value itself only has to be a valid claim id.
const hostileClaimID = "widget.contract.overview"

// hostilePathCase is one entry of the corpus. `dir` is the directory name the
// fixture actually creates — twice, per the header — and `onWindows` is the
// DECLARATION that the name is storable there. Exclusion happens only through
// that field: nothing below probes, retries, or discovers its way around a
// case.
type hostilePathCase struct {
	name      string
	dir       string
	onWindows bool
}

// The corpus. Each entry names the shell or git behaviour it exists to
// collide with; "harmless-looking" entries (space, apostrophe) are here
// because they are the ones real user directories actually contain.
var hostilePathCases = []hostilePathCase{
	// An apostrophe terminates a single-quoted shell string. C:\Users\O'Brien
	// is an ordinary Windows account; a printed remediation line that
	// single-quotes a path naively dies on it, which is why the fixed
	// installer has to escape it rather than merely switch quote styles.
	{name: "apostrophe", dir: "O'Brien", onWindows: true},
	// `$` expands inside double quotes. A printed line carrying "$hooks_dir"
	// already expanded is fine; one carrying a path that CONTAINS `$` inside
	// double quotes re-expands when the user runs it.
	{name: "dollar", dir: "pa$th", onWindows: true},
	// A backtick opens command substitution inside double quotes — the other
	// re-expansion channel, and the nastier one because the substitution can
	// swallow the rest of the printed block looking for its closing mate.
	{name: "backtick", dir: "back`tick", onWindows: true},
	// Whitespace is the boring one that catches unquoted expansions.
	{name: "space", dir: "two words", onWindows: true},
	// The one class core.quotepath=false genuinely does fix; kept in the
	// corpus so the fix for the classes it does NOT fix cannot regress this
	// one. Matches hook-smoke-test.sh case 16.
	{name: "nonascii", dir: "café", onWindows: true},
	// The three git C-quotes UNCONDITIONALLY, quotepath notwithstanding.
	// All three are illegal in Windows file names, so they are declared
	// POSIX-only — by this table, not by any runtime probe.
	{name: "quote", dir: `qu"ote`, onWindows: false},
	{name: "backslash", dir: `back\slash`, onWindows: false},
	{name: "tab", dir: "tab\tchar", onWindows: false},
}

// runHostilePathCases drives `body` over every case declared for this OS and
// then makes the equality assertion the header promises: executed == declared.
//
// Execution is recorded at subtest ENTRY, so a case that fails still counts as
// executed — the equality is about coverage, not about verdicts; the verdicts
// are the subtests' own. What the equality catches is a case that never STARTED:
// a `-run` selector that deselected a subtest, a harness change that reordered
// the table into oblivion, a future edit that filters the slice. Each of those
// currently reads as a green parent over fewer cases, and this assertion is
// what makes it read as the coverage loss it is. It is deliberately the LAST
// assertion, after every subtest has run.
func runHostilePathCases(t *testing.T, body func(t *testing.T, hostile string)) {
	t.Helper()
	declared := make([]string, 0, len(hostilePathCases))
	executed := make(map[string]bool, len(hostilePathCases))
	for _, c := range hostilePathCases {
		if runtime.GOOS == "windows" && !c.onWindows {
			// Excluded BY DECLARATION. The name cannot exist on this
			// filesystem, and a case that "ran" by storing a mangled
			// substitute would be proving something about the substitute.
			continue
		}
		declared = append(declared, c.name)
		c := c
		t.Run(c.name, func(t *testing.T) {
			executed[c.name] = true
			body(t, c.dir)
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
		t.Fatalf("the executed case set does not equal the declared case set for %s.\n  declared: %v\n  executed: %v\nEvery declared case must run: a case that could not start is missing coverage, not a pass, and if a -run selector narrowed the subtests then the narrowed run is the thing this failure is reporting",
			runtime.GOOS, want, got)
	}
	if len(want) == 0 {
		t.Fatalf("zero cases are declared for %s, so this test asserted nothing at all; the table above must declare at least the portable cases on every OS", runtime.GOOS)
	}
}

// hostileShell resolves the shell that runs the POSIX installer: sh everywhere,
// and Git for Windows' bash on Windows (git executes hooks through its own
// bundled sh there, so bash is the honest stand-in for "the sh git will use").
//
// The System32 check below is this file's one nod to the WSL trap the
// PowerShell wrapper guards against: C:\Windows\System32\bash.exe is WSL's
// LAUNCHER, and a "bash" that resolves there would run this suite inside a
// Linux VM against Linux paths — every case would then be exercising the wrong
// operating system while reporting on this one. That is "could not run", and
// it fails rather than proceeding.
func hostileShell(t *testing.T) string {
	t.Helper()
	name := "sh"
	if runtime.GOOS == "windows" {
		name = "bash"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("no %s on PATH (%v); the installer under test is a POSIX script and there is nothing to run it with. That is a failure of this check, not a reason to skip it", name, err)
	}
	if runtime.GOOS == "windows" {
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.Contains(lower, "/windows/system32/") || strings.Contains(lower, "/windows/sysnative/") {
			t.Fatalf("bash on PATH resolves to %s, which is WSL's launcher, not a Windows bash; running the installer through it would test a Linux filesystem while reporting on this one. Install Git for Windows or reorder PATH", path)
		}
	}
	return path
}

// hostileEnv is the hermetic environment every git, installer and hook
// invocation below runs under. GIT_CONFIG_NOSYSTEM plus an empty global config
// make the fixture immune to the machine's own configuration — a developer's
// global core.hooksPath would otherwise redirect the install and fail the
// .git/hooks assertions for a reason that has nothing to do with the corpus.
// DOSSIERX_BIN points the hook at the binary TestMain built; forward slashes
// because the value is read by git's bundled sh on Windows, where a
// backslashed path inside "$bin" is a quoting question this test refuses to
// have an opinion on.
func hostileEnv(t *testing.T) []string {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "empty-gitconfig")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty git config: %v", err)
	}
	return append(os.Environ(),
		"DOSSIERX_BIN="+filepath.ToSlash(binPath),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+empty,
	)
}

// hostileExec runs one command with no shell in between — the hostile bytes
// travel in argv and Dir, where no quoting layer exists to get wrong — and
// returns stdout, stderr and the exit code. Any failure that is not the child
// exiting non-zero (binary missing, dir unenterable) is a t.Fatal: it means
// the check could not run.
func hostileExec(t *testing.T, dir string, env []string, name string, args ...string) (stdout, stderr []byte, code int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("could not run %s %v in %s: %v", name, args, dir, err)
		}
		code = exitErr.ExitCode()
	}
	return out.Bytes(), errb.Bytes(), code
}

// hostileInstaller is the path of the script under test, from the checkout —
// the corpus tests the installer as it will ship, not a copy.
func hostileInstaller(t *testing.T) string {
	t.Helper()
	p := filepath.Join(moduleRoot(t), "scripts", "install-git-hook.sh")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("install-git-hook.sh not found at %s: %v", p, err)
	}
	return p
}

// hostilePrintHook captures `--print-hook` output once: the canonical hook
// bytes every installed file below is compared against. Byte-for-byte is the
// installer's own idempotence contract (it decides "already current" by cmp),
// so it is the right notion of "installed correctly" here too.
func hostilePrintHook(t *testing.T, shell string, env []string) []byte {
	t.Helper()
	out, errOut, code := hostileExec(t, moduleRoot(t), env, shell, hostileInstaller(t), "--print-hook")
	if code != 0 || len(out) == 0 {
		t.Fatalf("--print-hook exited %d with %d bytes of output; without the canonical hook body nothing below can be compared against anything.\nstderr:\n%s", code, len(out), errOut)
	}
	return out
}

// newHostileRepo builds the two-level fixture the header describes and leaves
// it fully staged: <tmp>/<hostile>/repo/<hostile>/{project.config.yaml,claims/…}
// with the claim LOCKED — the state a real project is in when someone is about
// to tamper with it — and `git add -A` already run.
//
// It ends with the probe that keeps the corpus honest: the TRACKED config path
// must actually carry the hostile bytes. A filesystem or git layer that
// transliterated the name away would leave a tame path, and every assertion
// after that would "pass" while proving nothing — so that is a Fatal, said out
// loud, never a quiet green. (Exception: the non-ASCII case only requires SOME
// byte above ASCII to survive, because macOS may legitimately renormalize the
// exact bytes while preserving the property the case is about.)
func newHostileRepo(t *testing.T, hostile string, env []string) string {
	t.Helper()
	parent := filepath.Join(t.TempDir(), hostile)
	repo := filepath.Join(parent, "repo")
	project := filepath.Join(repo, hostile)
	if err := os.MkdirAll(filepath.Join(project, "claims"), 0o755); err != nil {
		t.Fatalf("could not create the hostile fixture directories under %s: %v (if the OS refuses this name the case belongs in the table's exclusions, declared, not discovered here)", parent, err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(project, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	mustRun := func(dir, what string, name string, args ...string) {
		t.Helper()
		out, errOut, code := hostileExec(t, dir, env, name, args...)
		if code != 0 {
			t.Fatalf("fixture setup failed at %q (exit %d)\nstdout:\n%s\nstderr:\n%s", what, code, out, errOut)
		}
	}
	mustRun(repo, "git init", "git", "init", "-q", ".")
	mustRun(repo, "git config user.email", "git", "config", "user.email", "hostile-paths@example.invalid")
	mustRun(repo, "git config user.name", "git", "config", "user.name", "hostile path corpus")
	mustRun(repo, "git config commit.gpgsign", "git", "config", "commit.gpgsign", "false")
	mustRun(project, "claim new", binPath, "--format", "text", "claim", "new", hostileClaimID,
		"--body", "the widget answers within 200ms.",
		"--governed-reason", "hostile-path fixture, not backed by any doctrine claim")
	mustRun(project, "check", binPath, "--format", "text", "check")
	mustRun(project, "claim lock", binPath, "--format", "text", "claim", "lock", hostileClaimID,
		"--reason", "approved for the hostile-path corpus")
	mustRun(repo, "git add -A", "git", "add", "-A")

	// The probe. -z in the TEST is fine — Go reads raw bytes; the constraint
	// that keeps -z out of naive shell loops (no dependable `read -d ''` in
	// git's bundled sh) binds the hook, not this file.
	lsOut, lsErr, code := hostileExec(t, repo, env, "git", "ls-files", "-z")
	if code != 0 {
		t.Fatalf("git ls-files -z failed (exit %d): %s", code, lsErr)
	}
	var configDir string
	found := 0
	for _, entry := range strings.Split(string(lsOut), "\x00") {
		if strings.HasSuffix(entry, "/project.config.yaml") {
			configDir = strings.TrimSuffix(entry, "/project.config.yaml")
			found++
		}
	}
	if found != 1 {
		t.Fatalf("expected exactly one tracked */project.config.yaml, found %d; the fixture is not the shape it claims to be.\nls-files -z:\n%q", found, lsOut)
	}
	if hostile == "café" {
		ascii := true
		for i := 0; i < len(configDir); i++ {
			if configDir[i] > 0x7f {
				ascii = false
				break
			}
		}
		if ascii {
			t.Fatalf("this platform transliterated the non-ASCII directory name (git tracked %q); the case cannot prove anything here, and saying so beats a green over a tame path", configDir)
		}
	} else if configDir != hostile {
		t.Fatalf("git tracked the project under %q, not the declared hostile name %q; the case would be exercising a different corpus entry than it reports", configDir, hostile)
	}
	return repo
}

// hostileTamper is the hand edit the gate exists to catch: the locked claim's
// body changed on disk with no unlock and no ledger record.
func hostileTamper(t *testing.T, projectDir string) {
	t.Helper()
	claim := filepath.Join(projectDir, "claims", hostileClaimID+".yaml")
	raw, err := os.ReadFile(claim)
	if err != nil {
		t.Fatalf("read the locked claim: %v", err)
	}
	edited := bytes.Replace(raw, []byte("200ms"), []byte("900ms"), 1)
	if bytes.Equal(edited, raw) {
		t.Fatalf("the tamper did not change the claim body; the fixture no longer contains the text this test edits")
	}
	if err := os.WriteFile(claim, edited, 0o644); err != nil {
		t.Fatalf("write the tampered claim: %v", err)
	}
}

// gitHooksPathState reads core.hooksPath as (bytes, exit code). Both halves
// matter: "unset" is exit 1 with no output, and collapsing that to an empty
// string would let "unset before, set-to-empty after" read as unchanged.
func gitHooksPathState(t *testing.T, repo string, env []string) ([]byte, int) {
	t.Helper()
	out, _, code := hostileExec(t, repo, env, "git", "config", "--get", "core.hooksPath")
	return out, code
}

// TestHookGatesEveryHostilePath is the corpus itself: install, honest commit,
// tamper, refusal, no side effects — under every declared hostile name.
func TestHookGatesEveryHostilePath(t *testing.T) {
	shell := hostileShell(t)
	env := hostileEnv(t)
	installer := hostileInstaller(t)
	wantHook := hostilePrintHook(t, shell, env)

	runHostilePathCases(t, func(t *testing.T, hostile string) {
		repo := newHostileRepo(t, hostile, env)
		project := filepath.Join(repo, hostile)

		hpBefore, hpBeforeCode := gitHooksPathState(t, repo, env)

		out, errOut, code := hostileExec(t, repo, env, shell, installer, "--yes")
		if code != 0 {
			t.Fatalf("the installer exited %d in a repository under this path.\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
		}

		// 1 · the installed file is byte-identical to --print-hook.
		hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
		got, err := os.ReadFile(hook)
		if err != nil {
			t.Fatalf("no hook at %s after a zero-exit install: %v\ninstaller stdout:\n%s\nstderr:\n%s", hook, err, out, errOut)
		}
		if !bytes.Equal(got, wantHook) {
			t.Fatalf("the installed hook (%d bytes) is not byte-identical to --print-hook output (%d bytes); the installer's own idempotence comparison would now report its fresh install as outdated", len(got), len(wantHook))
		}

		// 2 · core.hooksPath untouched, as bytes AND exit code.
		hpAfter, hpAfterCode := gitHooksPathState(t, repo, env)
		if hpBeforeCode != hpAfterCode || !bytes.Equal(hpBefore, hpAfter) {
			t.Fatalf("core.hooksPath changed across the install: before (exit %d) %q, after (exit %d) %q — the installer must only ever READ that setting", hpBeforeCode, hpBefore, hpAfterCode, hpAfter)
		}

		// 3 · honest work commits. This is the half that goes red when the
		// gate starts refusing everything — the exact shape of the C-quoting
		// defect, where a discovered config that git printed C-quoted names
		// no file and the hook refuses a commit touching no claim at all.
		out, errOut, code = hostileExec(t, repo, env, "git", "commit", "-q", "-m", "claims")
		if code != 0 {
			t.Fatalf("the gate REFUSED AN HONEST COMMIT (exit %d) in a repository under this path — the hook is almost certainly feeding dossierx a path quoted for display rather than the path itself.\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
		}

		// 4 · the gate is still ON. Without this half, a hook that reached
		// "pass" by silently skipping the project it could not name would
		// satisfy everything above.
		hostileTamper(t, project)
		if out, errOut, code = hostileExec(t, repo, env, "git", "add", "-A"); code != 0 {
			t.Fatalf("git add -A after the tamper exited %d:\n%s\n%s", code, out, errOut)
		}
		statusBefore, errOut, code := hostileExec(t, repo, env, "git", "status", "--porcelain")
		if code != 0 {
			t.Fatalf("git status --porcelain exited %d: %s", code, errOut)
		}
		out, errOut, code = hostileExec(t, repo, env, "git", "commit", "-q", "-m", "sneak an edit past review")
		if code == 0 {
			t.Fatalf("a commit carrying a hand-edited LOCKED claim SUCCEEDED under this path — the gate did not fire.\nstdout:\n%s\nstderr:\n%s", out, errOut)
		}

		// 5 · the refusal left no fingerprints on the tree it judged.
		statusAfter, errOut, code := hostileExec(t, repo, env, "git", "status", "--porcelain")
		if code != 0 {
			t.Fatalf("git status --porcelain after the refusal exited %d: %s", code, errOut)
		}
		if !bytes.Equal(statusBefore, statusAfter) {
			t.Fatalf("the refused commit changed the working state.\nbefore:\n%s\nafter:\n%s", statusBefore, statusAfter)
		}
	})
}

// extractChainItLines pulls the two runnable commands and the one hook line out
// of the foreign-hook refusal, VERBATIM — continuation backslashes intact —
// because the thing under test is the text a user would paste, not this file's
// idea of what that text meant.
//
// The selection is structural, not textual: a command line is recognised by
// the program it invokes (the installer, then chmod), and the chain line by
// being the one line the block tells the user to put in their own hook. If the
// block is reworded so that nothing matches, this fails loudly — the
// remediation this test replays no longer exists in a form it can find, and
// that is a finding about the installer's output contract, not a parsing
// inconvenience to code around.
func extractChainItLines(t *testing.T, refusal []byte) (script, chainLine string) {
	t.Helper()
	text := strings.ReplaceAll(string(refusal), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var cmd []string
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(trimmed, "scripts/install-git-hook.sh "):
			cur := trimmed
			cmd = append(cmd, cur)
			for strings.HasSuffix(cur, "\\") && i+1 < len(lines) {
				i++
				cur = strings.TrimRight(lines[i], " \t")
				cmd = append(cmd, cur)
			}
		case len(cmd) > 0 && strings.HasPrefix(trimmed, "chmod "):
			cmd = append(cmd, trimmed)
		case strings.HasSuffix(trimmed, "|| exit 1") && strings.Contains(trimmed, "dossierx-pre-commit"):
			chainLine = trimmed
		}
	}
	if len(cmd) < 2 || chainLine == "" {
		t.Fatalf("could not extract the chain-it remediation from the refusal (found %d command line(s), chain line %q).\nThe refusal was:\n%s", len(cmd), chainLine, refusal)
	}
	return strings.Join(cmd, "\n"), chainLine
}

// TestPrintedChainItRemediationExecutesUnderHostilePaths replays what the
// installer PRINTS. The foreign-hook refusal tells the user to run two lines
// and add a third to their own hook; this test captures those exact lines from
// a repository at a hostile path, executes them in a fresh shell, and asserts
// they work and have the stated effect — the generic detector for printed
// lines whose quoting cannot carry the path they interpolate.
//
// The installer is COPIED to scripts/install-git-hook.sh inside the fixture
// first, because that is the relative path the printed line names: the
// instructions assume the reader's repository holds the script where this one
// does, and replaying them anywhere else would test a different sentence.
func TestPrintedChainItRemediationExecutesUnderHostilePaths(t *testing.T) {
	shell := hostileShell(t)
	env := hostileEnv(t)
	installerSrc, err := os.ReadFile(hostileInstaller(t))
	if err != nil {
		t.Fatalf("read the installer: %v", err)
	}
	wantHook := hostilePrintHook(t, shell, env)

	runHostilePathCases(t, func(t *testing.T, hostile string) {
		repo := newHostileRepo(t, hostile, env)
		project := filepath.Join(repo, hostile)

		if err := os.MkdirAll(filepath.Join(repo, "scripts"), 0o755); err != nil {
			t.Fatalf("mkdir scripts: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo, "scripts", "install-git-hook.sh"), installerSrc, 0o755); err != nil {
			t.Fatalf("copy the installer into the fixture: %v", err)
		}

		// The pre-existing hook the user wants to keep. Its observable effect
		// is a marker file under .git — outside the worktree, so the
		// no-side-effects comparison below cannot be confused by it — and
		// hooks run from the top of the worktree, so the relative path needs
		// no quoting at all.
		hooksDir := filepath.Join(repo, ".git", "hooks")
		if err := os.MkdirAll(hooksDir, 0o755); err != nil {
			t.Fatalf("mkdir hooks dir: %v", err)
		}
		foreign := []byte("#!/bin/sh\n: > .git/foreign-hook-ran\n")
		foreignHook := filepath.Join(hooksDir, "pre-commit")
		if err := os.WriteFile(foreignHook, foreign, 0o755); err != nil {
			t.Fatalf("plant the foreign hook: %v", err)
		}
		marker := filepath.Join(repo, ".git", "foreign-hook-ran")

		// The refusal that carries the instructions. Non-zero by contract —
		// and the foreign hook must be byte-identical afterwards, because a
		// refusal that edited what it refused to touch is not a refusal.
		out, errOut, code := hostileExec(t, repo, env, shell, filepath.Join("scripts", "install-git-hook.sh"), "--yes")
		if code == 0 {
			t.Fatalf("the installer replaced a foreign hook without --force.\nstdout:\n%s\nstderr:\n%s", out, errOut)
		}
		afterRefusal, err := os.ReadFile(foreignHook)
		if err != nil || !bytes.Equal(afterRefusal, foreign) {
			t.Fatalf("the foreign hook changed across a refusal (read err %v)", err)
		}

		script, chainLine := extractChainItLines(t, errOut)

		// THE REPLAY. A fresh shell, -e so the first failing line is the
		// verdict, cwd at the repository root exactly as the instructions
		// assume. Everything hostile is inside the script TEXT — this test
		// adds no quoting of its own, which is the point.
		out, errOut, code = hostileExec(t, repo, env, shell, "-e", "-c", script)
		if code != 0 {
			t.Fatalf("the remediation lines the installer PRINTED do not run (exit %d) when the path they interpolate is hostile.\nThe lines were:\n%s\nstdout:\n%s\nstderr:\n%s", code, script, out, errOut)
		}

		// The stated effect, in bytes: our hook body, at the promised path.
		chained, err := os.ReadFile(filepath.Join(hooksDir, "dossierx-pre-commit"))
		if err != nil {
			t.Fatalf("the printed lines exited 0 but produced no %s — they wrote SOMEWHERE, which under a hostile path is worse than failing: %v\nThe lines were:\n%s", filepath.Join(hooksDir, "dossierx-pre-commit"), err, script)
		}
		if !bytes.Equal(chained, wantHook) {
			t.Fatalf("the chained hook is not byte-identical to --print-hook output (%d vs %d bytes)", len(chained), len(wantHook))
		}

		// The third printed line, appended to the user's own hook verbatim.
		if err := os.WriteFile(foreignHook, append(append([]byte{}, foreign...), []byte(chainLine+"\n")...), 0o755); err != nil {
			t.Fatalf("chain the gate from the foreign hook: %v", err)
		}

		// Honest work still commits, and the foreign half still ran.
		if out, errOut, code = hostileExec(t, repo, env, "git", "add", "-A"); code != 0 {
			t.Fatalf("git add -A exited %d:\n%s\n%s", code, out, errOut)
		}
		out, errOut, code = hostileExec(t, repo, env, "git", "commit", "-q", "-m", "claims plus the chained gate")
		if code != 0 {
			t.Fatalf("an honest commit was refused through the chained hook (exit %d).\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("the foreign hook's own work did not happen on the honest commit (%v); chaining replaced the user's hook instead of extending it", err)
		}
		if err := os.Remove(marker); err != nil {
			t.Fatalf("reset the foreign-hook marker: %v", err)
		}

		// And the gate is armed THROUGH the chain: tamper, refuse, no
		// side effects, foreign half still ran first.
		hostileTamper(t, project)
		if out, errOut, code = hostileExec(t, repo, env, "git", "add", "-A"); code != 0 {
			t.Fatalf("git add -A after the tamper exited %d:\n%s\n%s", code, out, errOut)
		}
		statusBefore, errOut, code := hostileExec(t, repo, env, "git", "status", "--porcelain")
		if code != 0 {
			t.Fatalf("git status --porcelain exited %d: %s", code, errOut)
		}
		out, errOut, code = hostileExec(t, repo, env, "git", "commit", "-q", "-m", "sneak an edit past review")
		if code == 0 {
			t.Fatalf("a tampered locked claim was committed through the chained hook.\nstdout:\n%s\nstderr:\n%s", out, errOut)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("the refusal fired before the foreign hook ran (%v); the printed chain line was supposed to ADD the gate, wherever the user put it, not pre-empt the hook it was added to", err)
		}
		statusAfter, errOut, code := hostileExec(t, repo, env, "git", "status", "--porcelain")
		if code != 0 {
			t.Fatalf("git status --porcelain after the refusal exited %d: %s", code, errOut)
		}
		if !bytes.Equal(statusBefore, statusAfter) {
			t.Fatalf("the refused commit changed the working state.\nbefore:\n%s\nafter:\n%s", statusBefore, statusAfter)
		}
	})
}
