// Package gitrepo is the engine's one read-only git reader.
//
// It exists because two different questions in this codebase have to be asked
// of the same repository, and asking them through two runners would mean
// maintaining the same hard-won fixes twice:
//
//   - internal/check asks about the INDEX and the ignore rules — what would a
//     commit carry, and is a store the engine writes ignored?
//   - internal/approvalrecovery asks about HISTORY — which past revision of a
//     claim file hashes to the approval the ledger signed?
//
// The commands differ; the way git must be located, anchored and interrogated
// does not. In particular the top-level re-anchoring in NewRunner is a repair
// for a real hole, described in full there: pathspecs computed relative to the
// config's own directory read an ordinary monorepo layout as "outside the
// repository" and answered the gate with silence. A second runner that
// recomputed that arithmetic would reopen it in the new caller while the old
// one stayed fixed, and nothing would report the difference.
//
// Everything here is read-only. No command this package issues can modify the
// index, the work tree or the object store, and callers cannot pass one
// through: the argv is assembled here and git is never given a shell.
package gitrepo

import (
	"errors"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// ErrOutsideWorkTree is Spec's "this path is not in the repository" answer.
// Callers decide what that means for them: for a claims directory it is
// "nothing a commit could carry", for a store it is "absent from the index",
// which is a finding rather than a pass.
var ErrOutsideWorkTree = errors.New("path is outside the git work tree")

// Runner runs git with a fixed working directory. Every command it issues is
// read-only; nothing here can modify the index, the worktree, or the object
// store.
type Runner struct {
	bin string

	// dir is git's working directory, and it is the repository's TOP LEVEL —
	// not the project's own directory. Everything git reports is therefore
	// repository-relative, which is the same namespace the index itself uses,
	// so a pathspec can name any tracked file in the repository regardless of
	// where the config happens to sit.
	dir string

	// base is the directory the callers' absolute paths are expressed relative
	// to (the config's own directory), and prefix is that directory as a
	// slash-separated path relative to dir — "" when the project sits at the top
	// level. Together they are all Spec needs to translate a project path into
	// a repository path.
	//
	// prefix comes from git ("rev-parse --show-prefix") rather than from
	// comparing strings, and that is deliberate: on macOS a temp directory is
	// reached through /var while git resolves it to /private/var, so any
	// arithmetic over the two absolute paths would disagree with git about a
	// path both of them can open.
	base   string
	prefix string

	// verb is the prefix every git failure is reported under — "check --staged"
	// for the index gate, "gitignore check" for Gitignored, "claim
	// recover-approved-content" for the history reader — so a git failure
	// during one command is never reported as another's.
	verb string
}

// NewRunner locates git, confirms dir is inside a work tree, and re-anchors
// itself at that work tree's TOP LEVEL.
//
// unavailable is the sentinel a missing git and a directory outside any work
// tree are both wrapped in. It is a parameter rather than a package-level
// error because the two callers answer that condition differently and pin
// different messages for it: internal/check treats it as ErrNoIndex, its
// deliberate exit-0 escape hatch for "there is nothing here to evaluate",
// while a history reader treats it as a refusal. Letting each supply its own
// sentinel keeps both behaviours and both message sets exactly as they are.
//
// THE RE-ANCHORING IS THE FIX FOR A HOLE, not tidiness. Pathspecs used to be
// computed relative to the config's own directory, so an ordinary monorepo
// layout — docs/project.config.yaml with `claims_dir: ../claims` — produced the
// spec "../claims", which read as "outside the repository" and answered with
// the escape hatch. The claims were not outside anything; git resolves them
// from the top level without complaint. The gate the pre-commit hook and CI
// both run therefore evaluated NOTHING, silently, on a layout that every other
// command in the product handles, and an out-of-band edit to a locked claim
// committed clean while `check --validate` on the same tree reported
// lock-content-drift.
func NewRunner(dir string, unavailable error) (*Runner, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: git is not installed or not on PATH", unavailable)
	}
	g := &Runner{bin: bin, dir: dir, base: dir}
	out, err := g.Run("rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return nil, fmt.Errorf("%w: %s is not inside a git work tree", unavailable, dir)
	}

	// One invocation, two answers, in the order they are asked for: the top
	// level, then this directory's path within it. Asking git for both keeps
	// the two consistent with each other and with the index, which is what
	// makes them safe to do path arithmetic with.
	out, err = g.Run("rev-parse", "--show-toplevel", "--show-prefix")
	if err != nil {
		return nil, fmt.Errorf("%w: git would not name the work tree containing %s: %w", unavailable, dir, err)
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) == "" {
		return nil, fmt.Errorf("%w: git would not name the top level of the work tree containing %s", unavailable, dir)
	}
	g.dir = lines[0]
	// --show-prefix is slash-terminated and empty at the top level.
	g.prefix = strings.Trim(lines[1], "/")
	return g, nil
}

// SetVerb names the command git failures are reported under. It returns the
// runner so a caller can set it where the runner is built.
func (g *Runner) SetVerb(verb string) *Runner {
	g.verb = verb
	return g
}

// Dir is the repository top level the runner is anchored at.
func (g *Runner) Dir() string { return g.dir }

// Spec expresses target — an absolute path, or one relative to the project
// directory — as a git pathspec relative to the REPOSITORY TOP LEVEL, which is
// both what the runner's commands are issued from and the namespace git reports
// paths in.
//
// It fails with ErrOutsideWorkTree only when the result climbs above the top
// level, i.e. when the path really is outside the repository. "Outside the
// config file's directory" is not that, and treating it as if it were is the
// defect NewRunner's comment describes.
func (g *Runner) Spec(target string) (string, error) {
	rel, err := filepath.Rel(g.base, target)
	if err != nil {
		return "", err
	}
	// path.Join cleans, so a "../" in rel is consumed by prefix when there is
	// prefix left to consume and survives when there is not.
	p := path.Join(g.prefix, filepath.ToSlash(rel))
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", fmt.Errorf("%w: %s is not under %s", ErrOutsideWorkTree, target, g.dir)
	}
	if p == "" {
		p = "."
	}
	return p, nil
}

// Run executes git with the runner's directory as cwd and returns stdout.
//
// Two -c overrides are not optional. core.quotepath=false and the -z flags the
// callers pass keep non-ASCII paths raw instead of C-quoted, and
// diff.relative=false pins whether "git diff" reports paths relative to cwd or
// to the repository root — a user config setting that would otherwise silently
// change which paths this code matches against ls-files' output. A gate whose
// answer depends on the auditee's git config is not a gate.
func (g *Runner) Run(args ...string) ([]byte, error) {
	return g.RunWithStdin("", args...)
}

// RunWithStdin is Run with input fed to git's stdin. Only cat-file --batch
// needs it, and it needs it because the alternative — one "git show" per object
// — is what made "always read from the index" look expensive enough to shortcut
// in the first place.
//
// Both are thin wrappers over RunStatus that treat EVERY non-zero exit as an
// error, which is right for every command that does not use the exit status as
// an answer.
func (g *Runner) RunWithStdin(stdin string, args ...string) ([]byte, error) {
	out, code, err := g.RunStatus(stdin, args...)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("%s: git %s: %s", g.verbOrDefault(), strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (g *Runner) verbOrDefault() string {
	if g.verb == "" {
		return "check --staged"
	}
	return g.verb
}

// RunStatus executes git and CARRIES ITS EXIT STATUS. err is non-nil only for a
// spawn failure (the binary vanished, fork failed); otherwise code is the
// process's exit status and out is stdout on success or stderr's text on a
// non-zero exit. It exists because `git check-ignore` ANSWERS through its
// status — 0 (at least one path ignored), 1 (none), 128 (fatal) — and a
// caller of Run could only write `if err != nil { /* not ignored */ }`, which
// reports "not ignored" for exit 1 and for exit 128 alike: a broken git passing
// the gate, the skip that reads as a pass.
func (g *Runner) RunStatus(stdin string, args ...string) (out []byte, code int, err error) {
	full := append([]string{"-c", "core.quotepath=false", "-c", "diff.relative=false"}, args...)
	cmd := exec.Command(g.bin, full...) //nolint:gosec // fixed binary, fixed argv; no shell involved
	cmd.Dir = g.dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err = cmd.Output()
	if err == nil {
		return out, 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return []byte(msg), ee.ExitCode(), nil
	}
	return nil, -1, fmt.Errorf("%s: git %s: %w", g.verbOrDefault(), strings.Join(args, " "), err)
}
