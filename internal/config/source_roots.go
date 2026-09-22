package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SourceRoot is one named source tree the code-link scan may walk.
// Path is relative to the config file (and may climb out of the project
// with ".."). Repo names the remote the tree belongs to. Ref is the
// commit, tag or branch the project pins; load resolves it to HEAD and
// refuses when the checkout is on a different commit.
type SourceRoot struct {
	Path string `yaml:"path"`
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`

	abs    string
	commit string
}

// AbsPath is Path resolved against the config directory.
func (r SourceRoot) AbsPath() string { return r.abs }

// Commit is the SHA of the checkout at load time (rev-parse HEAD).
func (r SourceRoot) Commit() string { return r.commit }

// ScansSource reports whether this project opted into tag scanning and
// the code-link gate: it named at least one in-tree source_dirs entry or
// one source_roots tree.
func (c *Config) ScansSource() bool {
	return c != nil && (len(c.SourceDirs) > 0 || len(c.SourceRoots) > 0)
}

// ScanWalkDirs returns the unique absolute directories Scan should walk:
// every source_dirs entry plus every source_roots path.
func (c *Config) ScanWalkDirs() []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]bool, len(c.SourceDirs)+len(c.SourceRoots))
	var out []string
	add := func(dir string) {
		dir = filepath.Clean(dir)
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		out = append(out, dir)
	}
	for _, sd := range c.SourceDirs {
		add(sd)
	}
	for _, r := range c.SourceRoots {
		add(r.abs)
	}
	return out
}

// SourceRootFor returns the first declared source root that contains
// absFile, or nil when the file is not under any source_roots path.
func (c *Config) SourceRootFor(absFile string) *SourceRoot {
	if c == nil {
		return nil
	}
	absFile = filepath.Clean(absFile)
	for i := range c.SourceRoots {
		if pathContains(c.SourceRoots[i].abs, absFile) {
			return &c.SourceRoots[i]
		}
	}
	return nil
}

// ResolveLinkFile applies Set's path contract: file must be relative, must
// exist, and must resolve either inside the project or under a declared
// source_roots path. recorded is the slash-separated path relative to the
// project directory (it may begin with "../" when the file lives in a
// sibling root).
func (c *Config) ResolveLinkFile(file string) (abs, recorded string, root *SourceRoot, err error) {
	if c == nil {
		return "", "", nil, fmt.Errorf("config: cfg must not be nil")
	}
	file = strings.TrimSpace(file)
	if file == "" {
		return "", "", nil, fmt.Errorf("file must not be empty")
	}
	if filepath.IsAbs(file) {
		return "", "", nil, fmt.Errorf("file %q must be a project-relative path, not absolute", file)
	}
	absDir, err := filepath.Abs(c.Dir())
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve project dir %q: %w", c.Dir(), err)
	}
	absFile, err := filepath.Abs(filepath.Join(absDir, file))
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve file %q: %w", file, err)
	}
	rel, err := filepath.Rel(absDir, absFile)
	if err != nil {
		return "", "", nil, fmt.Errorf("file %q must resolve relative to the project directory", file)
	}
	escapes := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
	if escapes {
		root = c.SourceRootFor(absFile)
		if root == nil {
			return "", "", nil, fmt.Errorf("file %q must resolve to a path inside the project directory or a declared source_roots path, not escape it via \"..\"", file)
		}
	}
	return absFile, filepath.ToSlash(rel), root, nil
}

func (c *Config) resolveSourceRoots(dir string) error {
	seenPath := make(map[string]int, len(c.SourceRoots))
	seenRepo := make(map[string]int, len(c.SourceRoots))
	for i := range c.SourceRoots {
		r := &c.SourceRoots[i]
		if strings.TrimSpace(r.Path) == "" {
			return fmt.Errorf("source_roots[%d].path is empty", i)
		}
		if strings.TrimSpace(r.Repo) == "" {
			return fmt.Errorf("source_roots[%d].repo is empty", i)
		}
		if strings.TrimSpace(r.Ref) == "" {
			return fmt.Errorf("source_roots[%d].ref is empty", i)
		}
		r.Path = filepath.ToSlash(filepath.Clean(r.Path))
		r.Repo = strings.TrimSpace(r.Repo)
		r.Ref = strings.TrimSpace(r.Ref)
		abs := r.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, filepath.FromSlash(r.Path))
		}
		abs = filepath.Clean(abs)
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("source_roots[%d].path %q: %w", i, r.Path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("source_roots[%d].path %q is not a directory", i, r.Path)
		}
		if prev, ok := seenPath[abs]; ok {
			return fmt.Errorf("source_roots[%d].path %q is the same directory as source_roots[%d]", i, r.Path, prev)
		}
		seenPath[abs] = i
		if prev, ok := seenRepo[r.Repo]; ok {
			return fmt.Errorf("source_roots[%d].repo %q is already used by source_roots[%d]", i, r.Repo, prev)
		}
		seenRepo[r.Repo] = i
		commit, err := resolvePinnedCommit(abs, r.Ref)
		if err != nil {
			return fmt.Errorf("source_roots[%d]: %w", i, err)
		}
		r.abs = abs
		r.commit = commit
	}
	return nil
}

func refuseEscapingSourceDirs(projectDir string, dirs []string) error {
	projectDir = filepath.Clean(projectDir)
	for _, sd := range dirs {
		clean := filepath.Clean(sd)
		if !pathContains(projectDir, clean) {
			return fmt.Errorf("source_dirs %q escapes the project directory; name a sibling checkout in source_roots: [{path, repo, ref}]", sd)
		}
	}
	return nil
}

func resolvePinnedCommit(repoDir, ref string) (string, error) {
	head, err := gitRevParse(repoDir, "HEAD")
	if err != nil {
		return "", fmt.Errorf("path %q is not a git checkout (repo and ref require one): %w", repoDir, err)
	}
	pinned, err := gitRevParse(repoDir, ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("ref %q does not resolve in %q: %w", ref, repoDir, err)
	}
	if head != pinned {
		return "", fmt.Errorf("checkout HEAD %s does not match ref %q (%s)", head, ref, pinned)
	}
	return head, nil
}

func gitRevParse(repoDir, rev string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "rev-parse", "--verify", rev)
	cmd.Env = isolatedGitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git rev-parse %s: %s", rev, detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func isolatedGitEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+4)
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_DIR=") || strings.HasPrefix(e, "GIT_WORK_TREE=") ||
			strings.HasPrefix(e, "GIT_CONFIG_GLOBAL=") || strings.HasPrefix(e, "GIT_CONFIG_SYSTEM=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	return out
}
