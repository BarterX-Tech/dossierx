// Package releasebaseline resolves the stable release immediately preceding
// the version documented at the head of a repository.
package releasebaseline

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Result struct {
	CurrentVersion  string
	CurrentCommit   string
	BaselineTag     string
	BaselineVersion string
	BaselineCommit  string
}

type Options struct {
	RepoDir     string
	Changelog   string
	OverrideTag string
	EventRef    string
	EventCommit string
}

var semverPattern = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)
var changelogPattern = regexp.MustCompile(`^## \[([0-9]+\.[0-9]+\.[0-9]+)\]`)

type version struct{ major, minor, patch string }

func parseVersion(tag string) (version, bool) {
	m := semverPattern.FindStringSubmatch(tag)
	if m == nil || !canonicalNumber(m[1]) || !canonicalNumber(m[2]) || !canonicalNumber(m[3]) {
		return version{}, false
	}
	return version{m[1], m[2], m[3]}, true
}

func canonicalNumber(s string) bool { return s == "0" || !strings.HasPrefix(s, "0") }

func (v version) String() string { return v.major + "." + v.minor + "." + v.patch }

func compare(a, b version) int {
	for _, pair := range [][2]string{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if len(pair[0]) != len(pair[1]) {
			if len(pair[0]) < len(pair[1]) {
				return -1
			}
			return 1
		}
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

func Resolve(opts Options) (Result, error) {
	if opts.RepoDir == "" {
		opts.RepoDir = "."
	}
	if opts.Changelog == "" {
		opts.Changelog = filepath.Join(opts.RepoDir, "CHANGELOG.md")
	}
	if shallow, err := git(opts.RepoDir, "rev-parse", "--is-shallow-repository"); err != nil {
		return Result{}, fmt.Errorf("check repository history: %w", err)
	} else if strings.TrimSpace(shallow) == "true" {
		return Result{}, errors.New("repository is shallow; fetch full history and tags before resolving a release baseline")
	}
	currentVersion, err := currentFromChangelog(opts.Changelog)
	if err != nil {
		return Result{}, err
	}
	currentCommit, err := git(opts.RepoDir, "rev-parse", "HEAD")
	if err != nil {
		return Result{}, fmt.Errorf("resolve current commit: %w", err)
	}
	currentCommit = strings.TrimSpace(currentCommit)
	if opts.EventRef != "" {
		want := "refs/tags/v" + currentVersion.String()
		if opts.EventRef != want {
			return Result{}, fmt.Errorf("release event ref %q is not current stable tag %q", opts.EventRef, want)
		}
		eventCommit, err := git(opts.RepoDir, "rev-parse", opts.EventRef+"^{commit}")
		if err != nil || strings.TrimSpace(eventCommit) != currentCommit {
			return Result{}, fmt.Errorf("release tag %q does not point at HEAD", want)
		}
		if opts.EventCommit != "" && strings.TrimSpace(opts.EventCommit) != currentCommit {
			return Result{}, fmt.Errorf("event commit %q does not match HEAD %q", opts.EventCommit, currentCommit)
		}
	}

	tags, err := git(opts.RepoDir, "for-each-ref", "--format=%(refname:strip=2)", "refs/tags")
	if err != nil {
		return Result{}, fmt.Errorf("enumerate release tags: %w", err)
	}
	type candidate struct {
		tag    string
		v      version
		commit string
	}
	var candidates []candidate
	for _, tag := range nonemptyLines(tags) {
		v, ok := parseVersion(tag)
		if !ok {
			continue
		}
		commit, err := git(opts.RepoDir, "rev-parse", tag+"^{commit}")
		if err != nil {
			return Result{}, fmt.Errorf("resolve stable tag %q: %w", tag, err)
		}
		commit = strings.TrimSpace(commit)
		if compare(v, currentVersion) >= 0 || commit == currentCommit {
			continue
		}
		if err := gitAncestor(opts.RepoDir, commit, currentCommit); err != nil {
			if errors.Is(err, errNotAncestor) {
				continue
			}
			return Result{}, fmt.Errorf("check ancestry for %q: %w", tag, err)
		}
		candidates = append(candidates, candidate{tag, v, commit})
	}
	if len(candidates) == 0 {
		return Result{}, fmt.Errorf("no reachable stable release below current %s", currentVersion.String())
	}
	sort.Slice(candidates, func(i, j int) bool { return compare(candidates[i].v, candidates[j].v) > 0 })
	base := candidates[0]
	if opts.OverrideTag != "" {
		if opts.OverrideTag != base.tag {
			return Result{}, fmt.Errorf("%s=%q is not automatic immediate predecessor %q", "DOSSIERX_PREV_RELEASE_TAG", opts.OverrideTag, base.tag)
		}
	}
	return Result{CurrentVersion: currentVersion.String(), CurrentCommit: currentCommit, BaselineTag: base.tag, BaselineVersion: base.v.String(), BaselineCommit: base.commit}, nil
}

func currentFromChangelog(path string) (version, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return version{}, fmt.Errorf("read changelog %s: %w", path, err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		m := changelogPattern.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if m == nil {
			continue
		}
		parts := strings.Split(m[1], ".")
		v := version{parts[0], parts[1], parts[2]}
		if !canonicalNumber(v.major) || !canonicalNumber(v.minor) || !canonicalNumber(v.patch) {
			return version{}, fmt.Errorf("changelog release heading %q is not canonical semver", m[1])
		}
		return v, nil
	}
	return version{}, errors.New("changelog has no stable release heading")
}

var errNotAncestor = errors.New("not ancestor")

func gitAncestor(dir, ancestor, descendant string) error {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, descendant)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return errNotAncestor
		}
		return err
	}
	return nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

func nonemptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}
