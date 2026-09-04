package check

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/layout"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// The three non-verdicts Gitignored can return as reason. When the check ran
// and gave a verdict, reason is "".
const (
	GitignoreNotAWorkTree    = "not a work tree"
	GitignoreOutsideWorkTree = "outside the work tree"
	GitignoreGitNotAvailable = "git not available"
	gitignoreVerb            = "gitignore check"
)

// ErrGitUnavailable is wrapped into Gitignored's error when the project is
// inside a git work tree and git could not answer — not on PATH, a bare or
// corrupt repository, or a `check-ignore` exit other than 0 or 1. It is never a
// clean verdict: the approval-recording verbs refuse on it, and the read-only
// check modes report it as data.gitignore_check.
var ErrGitUnavailable = errors.New("git could not answer whether the build directory is ignored")

// gitignoreTarget is one path the engine will write, with its kind (the
// finding's harm clause is per kind) and the noun the sentence uses for it.
type gitignoreTarget struct {
	abs    string
	what   string
	kind   layout.Kind
	module string
}

// gitignoreTargets lists the checked paths in the order the findings are
// reported: the three ledger stores, then each module's two artifacts, then
// the build directory's own .gitignore LAST — the approval verbs surface the
// first finding as the store_gitignored refusal's body, so under the ordinary
// bare `build/` pattern that body is the lock ledger's, and build/.gitignore,
// whose harm is the mildest (check rewrites it from the default on the next
// clone), never leads.
func gitignoreTargets(cfg *config.Config) []gitignoreTarget {
	targets := []gitignoreTarget{
		{cfg.LockStorePath(), "the lock ledger", layout.KindLockStore, ""},
		{cfg.CommentDigestPath(), "the comment digest", layout.KindCommentDigest, ""},
		{cfg.FlagStorePath(), "the flag store", layout.KindFlagStore, ""},
	}
	for _, m := range cfg.Modules {
		targets = append(targets,
			gitignoreTarget{cfg.BuildOrderPath(m), fmt.Sprintf("module %q's build order", m), layout.KindBuildOrder, m},
			gitignoreTarget{cfg.CodeLinksPath(m), fmt.Sprintf("module %q's code links", m), layout.KindCodeLinks, m},
		)
	}
	return append(targets, gitignoreTarget{cfg.BuildGitignorePath(), "the build directory's own .gitignore", layout.KindBuildGitignore, ""})
}

// Gitignored decides, from FILE paths and never directories, whether any path
// the engine writes under the build directory would be ignored by git.
//
// It returns one finding per ignored path that is NOT in the index (the harm:
// the approval never reaches the repository), one warning per ignored path
// that IS in the index (force-added, or committed before the pattern; the
// ledger does reach collaborators, but nothing will stage the next new
// artifact), and a reason when the check could not give a verdict:
//
//   - GitignoreNotAWorkTree: no .git directory or file above cfg.Dir(). Decided
//     without git, so a machine with no git binary gets this answer too.
//   - GitignoreOutsideWorkTree: the build directory resolves above the
//     repository's top level; no finding, and one warning saying so.
//   - GitignoreGitNotAvailable, WITH a non-nil error wrapping ErrGitUnavailable:
//     a .git exists but git is not on PATH, the runner refused the directory
//     (a bare or corrupt repository), or `check-ignore` exited with a status
//     other than 0 or 1. Never a clean verdict; what a caller does with it is
//     decided per verb (the approval verbs refuse, the read-only check modes
//     report the reason and exit 0 — see cmd/dossierx's runCheckStaged doc).
//
// Three git spawns at most, independent of module count: `check-ignore
// --no-index -z --stdin` for the VERDICT (each echoed path is ignored; exit 0
// means at least one was, exit 1 none), `check-ignore -v` for the pattern and
// line (never for the verdict — with -v a path matched only by a negation is
// echoed too), and `ls-files` over the ignored set to split tracked from
// untracked. --no-index is deliberate: plain check-ignore consults the index
// and reports a directory as not ignored the moment one tracked file sits
// under it, which the prescribed migration's first `git mv` produces, and a
// directory-level check would then go green while every sibling stayed
// ignored. check.Status sits behind GET /api/status, so the cost is bounded
// on purpose and the verdict is never cached (a .gitignore edit while serve
// runs is outside the claims watcher's view).
func Gitignored(cfg *config.Config) (findings []lock.Finding, warnings []string, reason string, err error) {
	if cfg == nil {
		return nil, nil, GitignoreNotAWorkTree, nil
	}
	if !layout.InWorkTree(cfg.Dir()) {
		return nil, nil, GitignoreNotAWorkTree, nil
	}
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		return nil, nil, GitignoreGitNotAvailable, gitUnavailable(cfg, lookErr.Error())
	}
	g, runnerErr := newGitRunner(cfg.Dir())
	if runnerErr != nil {
		// The git-free walk found a .git, so "no index" is not the exit-0
		// escape hatch here: a bare or corrupt repository is one git cannot
		// answer for, and an approval recorded over it reaches nobody.
		return nil, nil, GitignoreGitNotAvailable, gitUnavailable(cfg, runnerErr.Error())
	}
	g.verb = gitignoreVerb

	targets := gitignoreTargets(cfg)
	specs := make([]string, 0, len(targets))
	byspec := make(map[string]gitignoreTarget, len(targets))
	for _, t := range targets {
		spec, specErr := g.spec(t.abs)
		if specErr != nil {
			if errors.Is(specErr, errOutsideWorkTree) {
				return nil, []string{layout.OutsideWorkTreeWarning(cfg.BuildDirPath())}, GitignoreOutsideWorkTree, nil
			}
			return nil, nil, GitignoreGitNotAvailable, gitUnavailable(cfg, specErr.Error())
		}
		specs = append(specs, spec)
		byspec[spec] = t
	}

	out, code, runErr := g.runStatus(strings.Join(specs, "\x00")+"\x00", "check-ignore", "--no-index", "-z", "--stdin", "--")
	if runErr != nil {
		return nil, nil, GitignoreGitNotAvailable, gitUnavailable(cfg, runErr.Error())
	}
	var ignored []string
	switch code {
	case 0:
		for _, p := range splitZ(out) {
			if _, known := byspec[p]; known {
				ignored = append(ignored, p)
			}
		}
	case 1:
		return nil, nil, "", nil
	default:
		return nil, nil, GitignoreGitNotAvailable, gitUnavailable(cfg, fmt.Sprintf("git check-ignore exited %d: %s", code, strings.TrimSpace(string(out))))
	}
	if len(ignored) == 0 {
		return nil, nil, "", nil
	}
	// Keep the engine's own order (the ledger three first, then per module,
	// then build/.gitignore), not the order git echoed.
	orderedIgnored := make([]string, 0, len(ignored))
	isIgnored := map[string]bool{}
	for _, p := range ignored {
		isIgnored[p] = true
	}
	for _, s := range specs {
		if isIgnored[s] {
			orderedIgnored = append(orderedIgnored, s)
		}
	}

	// The pattern and line, for the message only.
	vout, vcode, verr := g.runStatus(strings.Join(orderedIgnored, "\x00")+"\x00", "check-ignore", "-v", "--no-index", "-z", "--stdin", "--")
	if verr != nil || (vcode != 0 && vcode != 1) {
		var cause string
		if verr != nil {
			cause = verr.Error()
		} else {
			cause = fmt.Sprintf("git check-ignore -v exited %d: %s", vcode, strings.TrimSpace(string(vout)))
		}
		return nil, nil, GitignoreGitNotAvailable, gitUnavailable(cfg, cause)
	}
	type match struct {
		source  string
		line    int
		pattern string
	}
	matches := map[string]match{}
	fields := splitZ(vout)
	for i := 0; i+3 < len(fields); i += 4 {
		// A line number git could not print as an integer is reported as 0
		// rather than failing the verdict: the number is for the message only.
		line := 0
		if n, err := strconv.Atoi(fields[i+1]); err == nil {
			line = n
		}
		matches[fields[i+3]] = match{source: fields[i], line: line, pattern: fields[i+2]}
	}

	// Which of the ignored paths the index holds — this one reads the index
	// on purpose (see the doc comment).
	lsOut, lsCode, lsErr := g.runStatus("", append([]string{"ls-files", "-z", "--"}, orderedIgnored...)...)
	if lsErr != nil || lsCode != 0 {
		var cause string
		if lsErr != nil {
			cause = lsErr.Error()
		} else {
			cause = fmt.Sprintf("git ls-files exited %d: %s", lsCode, strings.TrimSpace(string(lsOut)))
		}
		return nil, nil, GitignoreGitNotAvailable, gitUnavailable(cfg, cause)
	}
	tracked := map[string]bool{}
	for _, p := range splitZ(lsOut) {
		tracked[p] = true
	}

	buildRel := projectRelative(cfg.Dir(), cfg.BuildDirPath())
	// git names the source .gitignore relative to the top level it resolved
	// (through symlinks: /private/var on macOS), while cfg.Dir() is the path
	// as configured; both sides are resolved before any arithmetic so a temp
	// directory reached through a symlink does not print as ../../../private.
	projectDir := resolved(cfg.Dir())
	repoDir := resolved(g.dir)
	for _, spec := range orderedIgnored {
		t := byspec[spec]
		m := matches[spec]
		// git names a .gitignore inside the repository relative to the top
		// level, and a core.excludesFile by the ABSOLUTE path it was
		// configured with. Joining an absolute path onto the top level would
		// land it inside the repository — printing a file that does not exist
		// and, worse, passing the "sits in the project" test below, which
		// would offer the eight-line block for a machine-wide file.
		sourceAbs := m.source
		if sourceAbs != "" && !filepath.IsAbs(sourceAbs) {
			sourceAbs = filepath.Join(g.dir, filepath.FromSlash(sourceAbs))
		}
		sourceAbs = resolved(sourceAbs)
		ip := layout.IgnoredPath{
			Path:     projectRelative(cfg.Dir(), t.abs),
			Kind:     t.kind,
			Module:   t.module,
			What:     t.what,
			Source:   projectRelative(projectDir, sourceAbs),
			Line:     m.line,
			Pattern:  m.pattern,
			BuildRel: buildRel,
		}
		sourceDir := filepath.Dir(sourceAbs)
		switch {
		case m.source == "":
			ip.Source = ".gitignore"
		case !dirContains(repoDir, sourceDir):
			// Not a file in this repository: name it exactly as git did, and
			// offer the negation for the PROJECT's own .gitignore, which
			// outranks the machine-wide file.
			ip.OutsideRepository = true
			ip.Source = filepath.ToSlash(m.source)
			ip.ProjectNegation = fmt.Sprintf("!%s/", buildRel)
		case !dirContains(projectDir, sourceDir):
			ip.AboveProject = true
			rel, relErr := filepath.Rel(sourceDir, resolved(cfg.BuildDirPath()))
			if relErr == nil {
				ip.ProjectNegation = fmt.Sprintf("!%s/", filepath.ToSlash(rel))
			}
		}
		if tracked[spec] {
			warnings = append(warnings, layout.IgnoredButTrackedWarning(ip))
			continue
		}
		findings = append(findings, lock.Finding{
			Rule:    RuleStoreGitignored,
			Message: layout.StoreGitignoredMessage(ip),
		})
	}
	return findings, warnings, "", nil
}

// gitUnavailable is the ErrGitUnavailable-wrapping error with the verbatim
// refusal text the approval verbs print.
func gitUnavailable(cfg *config.Config, cause string) error {
	return fmt.Errorf("%w: %s", ErrGitUnavailable, layout.GitUnavailableMessage(projectRelative(cfg.Dir(), cfg.LockStorePath()), cause))
}

// projectRelative expresses abs as a slash-separated path relative to the
// project directory — the namespace a reader opens files in.
func projectRelative(projectDir, abs string) string {
	rel, err := filepath.Rel(projectDir, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// dirContains reports whether child is dir or sits under it.
func dirContains(dir, child string) bool {
	rel, err := filepath.Rel(dir, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// resolved is p with symlinks evaluated where the path exists, and p itself
// otherwise (a build directory that does not exist yet still has to be
// expressed relative to something).
func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	// Resolve the longest existing prefix so a not-yet-created leaf under a
	// symlinked temp root still lands beside its resolved siblings.
	dir, base := filepath.Split(filepath.Clean(p))
	if dir == "" || dir == p {
		return p
	}
	return filepath.Join(resolved(filepath.Clean(dir)), base)
}

// withGitignoreFindings puts the guard's project-scoped findings AHEAD of the
// ledger gate's, in a fresh slice: the guard's verdict is about where the
// ledger lives, which a reader needs before any finding about its content.
func withGitignoreFindings(gitignore, ledger []lock.Finding) []lock.Finding {
	out := make([]lock.Finding, 0, len(gitignore)+len(ledger))
	out = append(out, gitignore...)
	return append(out, ledger...)
}
