package tests

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	dxskills "github.com/BarterX-Tech/dossierx/skills"
	"gopkg.in/yaml.v3"
)

const projectGraphSkill = ".agents/skills/dossierx-graph-safety/SKILL.md"

// These tests prove that checked-in entry points and references remain readable
// in a relocated checkout without symlinks. They do not prove that a particular
// agent runtime automatically discovers or follows the skill.
func TestProjectGraphSafetySkillIsWiredButNotEmbedded(t *testing.T) {
	checkGraphSkillEntryPoints(t, repoRoot(t))
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), ".agents", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := fs.Stat(dxskills.FS, entry.Name()); err == nil {
			t.Errorf("maintainer-only skill %s entered the embedded consumer bundle", entry.Name())
		} else if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("inspect embedded consumer skill %s: %v", entry.Name(), err)
		}
	}
}

func TestProjectGraphSafetySkillWorksWithoutSymlinksAfterRelocation(t *testing.T) {
	source := repoRoot(t)
	// A different parent and spaces in the path expose links tied to the current
	// directory or to a maintainer's checkout. Only ordinary files are copied.
	relocated := filepath.Join(t.TempDir(), "relocated checkout", "dossierx")
	for _, rel := range []string{
		"AGENTS.md", "CLAUDE.md", "docs/RELEASING.md",
		"docs/MAINTAINER_SKILLS.md", "docs/lessons",
		".agents",
		".claude/skills/dossierx-graph-safety",
	} {
		err := filepath.WalkDir(filepath.Join(source, filepath.FromSlash(rel)), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				t.Fatalf("project skill routing requires a symlink: %s", path)
			}
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			target := filepath.Join(relocated, relative)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, body, 0o644)
		})
		if err != nil {
			t.Fatalf("copy skill entry point %s: %v", rel, err)
		}
	}
	checkGraphSkillEntryPoints(t, relocated)
}

// Markdown links resolve relative to their containing document. Root-level
// documents also use inline-code repository paths; accept those as references.
var graphSkillMarkdownLink = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
var graphSkillCodePath = regexp.MustCompile("`([^`\\n]+\\.md)`")

func checkGraphSkillEntryPoints(t *testing.T, root string) {
	t.Helper()
	for _, route := range []struct{ from, to string }{
		{"CLAUDE.md", "AGENTS.md"},
		{"AGENTS.md", projectGraphSkill},
		{"AGENTS.md", ".agents/skills/learning-to-skill/SKILL.md"},
		{"AGENTS.md", ".agents/skills/skill-gap-review/SKILL.md"},
		{"AGENTS.md", "docs/MAINTAINER_SKILLS.md"},
		{"docs/RELEASING.md", projectGraphSkill},
		{".claude/skills/dossierx-graph-safety/SKILL.md", projectGraphSkill},
	} {
		from := filepath.Join(root, filepath.FromSlash(route.from))
		want := filepath.Join(root, filepath.FromSlash(route.to))
		body, err := os.ReadFile(from)
		if err != nil {
			t.Fatalf("read project skill entry point %s: %v", route.from, err)
		}
		if filepath.Base(from) == "SKILL.md" {
			checkMaintainerSkillMetadata(t, from, body)
		}
		found := false
		for _, link := range graphSkillMarkdownLink.FindAllStringSubmatch(string(body), -1) {
			if filepath.Clean(filepath.Join(filepath.Dir(from), filepath.FromSlash(link[1]))) == want {
				found = true
			}
		}
		for _, codePath := range graphSkillCodePath.FindAllStringSubmatch(string(body), -1) {
			if filepath.Clean(filepath.Join(root, filepath.FromSlash(codePath[1]))) == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no resolvable reference to %s", route.from, route.to)
		}
		if body, err := os.ReadFile(want); err != nil || len(strings.TrimSpace(string(body))) == 0 {
			t.Errorf("%s references an unreadable or empty %s: %v", route.from, route.to, err)
		}
	}

	// Follow every local Markdown reference from the canonical skill library so a
	// missing or incorrectly relocated supporting document cannot pass silently.
	skillDir := filepath.Join(root, ".agents", "skills")
	err := filepath.WalkDir(skillDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if entry.Name() == "SKILL.md" {
			checkMaintainerSkillMetadata(t, path, body)
		}
		for _, link := range graphSkillMarkdownLink.FindAllStringSubmatch(string(body), -1) {
			target := strings.SplitN(link[1], "#", 2)[0]
			if target == "" || strings.Contains(target, "://") {
				continue
			}
			if filepath.IsAbs(target) {
				t.Errorf("%s uses a machine-specific absolute reference: %s", path, target)
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)))
			relative, err := filepath.Rel(root, resolved)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				t.Errorf("%s references a file outside the checkout: %s", path, target)
				continue
			}
			if raw, err := os.ReadFile(resolved); err != nil || len(strings.TrimSpace(string(raw))) == 0 {
				t.Errorf("%s has an unreadable or empty reference %s: %v", path, target, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read canonical graph-safety references: %v", err)
	}
}

// Validate the portable format rather than pinning the wording of instructions.
func checkMaintainerSkillMetadata(t *testing.T, path string, body []byte) {
	t.Helper()
	parts := strings.SplitN(strings.ReplaceAll(string(body), "\r\n", "\n"), "---\n", 3)
	if len(parts) != 3 || parts[0] != "" {
		t.Errorf("%s lacks YAML frontmatter", path)
		return
	}
	var metadata struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(parts[1]), &metadata); err != nil {
		t.Errorf("%s has invalid skill metadata: %v", path, err)
		return
	}
	if metadata.Name != filepath.Base(filepath.Dir(path)) || len(metadata.Name) > 64 || !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(metadata.Name) {
		t.Errorf("%s has an invalid or mismatched skill name %q", path, metadata.Name)
	}
	if strings.TrimSpace(metadata.Description) == "" || len(metadata.Description) > 1024 {
		t.Errorf("%s needs a nonempty description of at most 1024 bytes", path)
	}
	if strings.TrimSpace(parts[2]) == "" {
		t.Errorf("%s has no skill instructions", path)
	}
}
