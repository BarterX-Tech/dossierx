package tests

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dxskills "github.com/BarterX-Tech/dossierx/skills"
)

// This checks packaging, not whether an agent follows the convergence rules.
func TestProjectAuditLoopSkillIsWiredButNotEmbedded(t *testing.T) {
	root := repoRoot(t)
	canonical := filepath.Join(root, ".agents", "skills", "audit-loop", "SKILL.md")
	raw, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	checkMaintainerSkillMetadata(t, canonical, raw)
	for _, rel := range []string{"AGENTS.md", ".agents/workflows/audit-fix-loop.md"} {
		from := filepath.Join(root, filepath.FromSlash(rel))
		body, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, link := range graphSkillMarkdownLink.FindAllStringSubmatch(string(body), -1) {
			if filepath.Clean(filepath.Join(filepath.Dir(from), filepath.FromSlash(link[1]))) == canonical {
				found = true
			}
		}
		if !found {
			t.Errorf("%s does not route to the canonical audit-loop skill", rel)
		}
	}
	if _, err := fs.Stat(dxskills.FS, "audit-loop"); err == nil {
		t.Fatal("maintainer-only audit-loop skill entered the embedded consumer bundle")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

// Copy only the reusable directory, then follow its local Markdown references.
// A dependency on the repository's sibling workflow directory must fail here.
func TestProjectAuditLoopSkillCanBeCopiedAlone(t *testing.T) {
	checkSkillCanBeCopiedAlone(t, "audit-loop", true)
}

func TestProjectLearningSkillsCanBeCopiedAlone(t *testing.T) {
	for _, name := range []string{"learning-to-skill", "skill-gap-review"} {
		t.Run(name, func(t *testing.T) { checkSkillCanBeCopiedAlone(t, name, name == "learning-to-skill") })
	}
}

func checkSkillCanBeCopiedAlone(t *testing.T, name string, requireReferences bool) {
	t.Helper()
	source := filepath.Join(repoRoot(t), ".agents", "skills", name)
	target := filepath.Join(t.TempDir(), "another project", name)
	if err := os.CopyFS(target, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	references := 0
	err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, link := range graphSkillMarkdownLink.FindAllStringSubmatch(string(raw), -1) {
			local := strings.SplitN(link[1], "#", 2)[0]
			if local == "" || strings.Contains(local, "://") {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(local)))
			rel, err := filepath.Rel(target, resolved)
			if err != nil || filepath.IsAbs(local) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Errorf("reference escapes portable skill: %s -> %s", path, local)
				continue
			}
			body, err := os.ReadFile(resolved)
			if err != nil || strings.TrimSpace(string(body)) == "" {
				t.Errorf("unreadable portable reference %s: %v", local, err)
			}
			references++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if requireReferences && references == 0 {
		t.Fatal("portable workflow references were not exercised")
	}
}
