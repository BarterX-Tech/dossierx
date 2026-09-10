package tests

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
	info, err := os.Stat(runner)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("local-client runner is not executable")
	}

	if _, err := fs.Stat(dxskills.FS, "dossierx-local-client"); err == nil {
		t.Fatal("maintainer-only local-client skill entered the embedded consumer bundle")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}
