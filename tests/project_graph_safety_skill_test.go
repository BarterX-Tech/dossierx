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

// TestProjectGraphSafetySkillIsWiredButNotEmbedded keeps the maintainer-only
// graph gate reachable from both agent harnesses without shipping it in the
// consumer bundle installed by `dossierx skills export`.
func TestProjectGraphSafetySkillIsWiredButNotEmbedded(t *testing.T) {
	root := repoRoot(t)
	canonical := filepath.Join(root, ".agents", "skills", "dossierx-graph-safety", "SKILL.md")
	if raw, err := os.ReadFile(canonical); err != nil {
		t.Fatalf("read canonical graph-safety skill: %v", err)
	} else if !strings.Contains(string(raw), "If any bound depends on the number of possible paths, stop") {
		t.Fatal("graph-safety skill lost its path-growth refusal")
	}

	claudeLink := filepath.Join(root, ".claude", "skills", "dossierx-graph-safety")
	info, err := os.Lstat(claudeLink)
	if err != nil {
		t.Fatalf("inspect Claude graph-safety skill link: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink to the canonical project skill", claudeLink)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(claudeLink, "SKILL.md"))
	if err != nil {
		t.Fatalf("resolve Claude graph-safety skill link: %v", err)
	}
	canonicalResolved, err := filepath.EvalSymlinks(canonical)
	if err != nil {
		t.Fatalf("resolve canonical graph-safety skill: %v", err)
	}
	if resolved != canonicalResolved {
		t.Fatalf("Claude graph-safety skill resolves to %s, want %s", resolved, canonicalResolved)
	}

	for _, check := range []struct {
		path string
		want string
	}{
		{"AGENTS.md", ".agents/skills/dossierx-graph-safety/SKILL.md"},
		{"CLAUDE.md", ".claude/skills/dossierx-graph-safety/SKILL.md"},
		{"docs/RELEASING.md", ".agents/skills/dossierx-graph-safety/SKILL.md"},
	} {
		if body := readRepoFile(t, check.path); !strings.Contains(body, check.want) {
			t.Errorf("%s does not point to the project graph-safety skill", check.path)
		}
	}

	if _, err := fs.Stat(dxskills.FS, "dossierx-graph-safety"); err == nil {
		t.Fatal("maintainer-only graph-safety skill entered the embedded consumer bundle")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("inspect embedded consumer skills: %v", err)
	}
}
