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

// TestProjectAuditLoopSkillIsWiredButNotEmbedded keeps the maintainer audit
// workflow reachable without shipping it through `dossierx skills export`.
func TestProjectAuditLoopSkillIsWiredButNotEmbedded(t *testing.T) {
	root := repoRoot(t)
	canonical := filepath.Join(root, ".agents", "skills", "audit-loop", "SKILL.md")
	raw, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("read canonical audit-loop skill: %v", err)
	}
	for _, required := range []string{
		"two consecutive audit rounds find zero regressions",
		"A round cap is a cost guard, not a success condition",
		"It does not grant permission to commit, push, merge, publish, or release",
	} {
		if !strings.Contains(string(raw), required) {
			t.Errorf("audit-loop skill lost required contract: %q", required)
		}
	}

	workflow := filepath.Join(root, ".agents", "workflows", "audit-fix-loop.md")
	workflowRaw, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("read canonical audit-loop workflow: %v", err)
	}
	if !strings.Contains(string(workflowRaw), "Missing auditor output") {
		t.Fatal("audit-loop workflow no longer treats missing coverage as non-green")
	}

	if body := readRepoFile(t, "AGENTS.md"); !strings.Contains(body, ".agents/skills/audit-loop/SKILL.md") {
		t.Error("AGENTS.md does not point to the project audit-loop skill")
	}

	if _, err := fs.Stat(dxskills.FS, "audit-loop"); err == nil {
		t.Fatal("maintainer-only audit-loop skill entered the embedded consumer bundle")
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("inspect embedded consumer skills: %v", err)
	}
}
