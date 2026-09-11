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
)

func TestProjectReleaseSafetySkillIsWiredButNotEmbedded(t *testing.T) {
	root := repoRoot(t)
	canonical := filepath.Join(root, ".agents", "skills", "dossierx-release-safety", "SKILL.md")
	raw, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	checkMaintainerSkillMetadata(t, canonical, raw)

	for _, rel := range []string{
		"AGENTS.md",
		"docs/MAINTAINER_SKILLS.md",
		".claude/skills/dossierx-release-safety/SKILL.md",
	} {
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
			t.Errorf("%s does not route to the canonical release-safety skill", rel)
		}
	}

	if _, err := fs.Stat(dxskills.FS, "dossierx-release-safety"); err == nil {
		t.Fatal("maintainer-only release-safety skill entered the embedded consumer bundle")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestReleaseProcedureNamesFinalCandidateGates(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "docs", "RELEASING.md"))
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	pins := regexp.MustCompile(`(?m)^\s+version: (v\d+\.\d+\.\d+)\s*$`).FindAllStringSubmatch(string(workflow), -1)
	if len(pins) != 2 || pins[0][1] != pins[1][1] {
		t.Fatalf("CI must expose one matching golangci-lint version for the root and viewer modules, got %v", pins)
	}
	for _, required := range []string{
		"golangci-lint@" + pins[0][1],
		"DOSSIERX_PREV_RELEASE_TAG=vPREVIOUS go test -race -json ./...",
		"TestRenderedOutputAcrossReleases",
		"Re-run every tree- and history-dependent pre-tag check on the merge",
		"Restore stamp-only viewer diffs; do not commit them.",
		"Exactly one Release publisher owns the tag.",
	} {
		if !strings.Contains(string(body), required) {
			t.Errorf("docs/RELEASING.md is missing release safeguard %q", required)
		}
	}
}
