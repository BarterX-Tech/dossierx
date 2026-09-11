package tests

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dxskills "github.com/BarterX-Tech/dossierx/skills"
)

func TestProjectLocalClientSkillIsWiredButNotEmbedded(t *testing.T) {
	root := repoRoot(t)
	canonical := filepath.Join(root, ".agents", "skills", "dossierx-local-client", "SKILL.md")
	raw, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	checkMaintainerSkillMetadata(t, canonical, raw)

	agentsPath := filepath.Join(root, "AGENTS.md")
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, link := range graphSkillMarkdownLink.FindAllStringSubmatch(string(agents), -1) {
		if filepath.Clean(filepath.Join(filepath.Dir(agentsPath), filepath.FromSlash(link[1]))) == canonical {
			found = true
		}
	}
	if !found {
		t.Fatal("AGENTS.md does not route to the canonical local-client skill")
	}

	runner := filepath.Join(filepath.Dir(canonical), "scripts", "run-local-client.sh")
	if _, err := os.Stat(runner); err != nil {
		t.Fatal(err)
	}
	relRunner, err := filepath.Rel(root, runner)
	if err != nil {
		t.Fatal(err)
	}
	mode, err := exec.Command("git", "-C", root, "ls-files", "--stage", "--", filepath.ToSlash(relRunner)).Output()
	if err != nil {
		t.Fatalf("read local-client runner mode from git index: %v", err)
	}
	fields := strings.Fields(string(mode))
	if len(fields) == 0 || fields[0] != "100755" {
		t.Fatalf("local-client runner git mode = %q, want 100755", strings.TrimSpace(string(mode)))
	}

	if _, err := fs.Stat(dxskills.FS, "dossierx-local-client"); err == nil {
		t.Fatal("maintainer-only local-client skill entered the embedded consumer bundle")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}
