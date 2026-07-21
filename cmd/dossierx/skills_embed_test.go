// skills_embed_test.go covers "dossierx skills export <dir>" (see
// skills_embed.go): it runs the command in-process via execCLI (defined in
// cli_inprocess_test.go) and asserts every embedded SKILL.md lands on
// disk under the target directory with the expected frontmatter "name:"
// value.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_SkillsExport_WritesAllSkillFiles(t *testing.T) {
	targetDir := t.TempDir()

	stdout, stderr, err := execCLI(t, "skills", "export", targetDir)
	if err != nil {
		t.Fatalf("skills export: unexpected error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	wantNames := map[string]string{
		filepath.Join(targetDir, "dossierx-claims", "SKILL.md"):      "name: dossierx-claims",
		filepath.Join(targetDir, "dossierx-build-order", "SKILL.md"): "name: dossierx-build-order",
		filepath.Join(targetDir, "dossierx-code-links", "SKILL.md"):  "name: dossierx-code-links",
	}

	for path, wantFrontmatter := range wantNames {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected exported file %s to exist: %v", path, err)
		}
		if !strings.Contains(string(data), wantFrontmatter) {
			t.Fatalf("expected %s to contain frontmatter %q, got:\n%s", path, wantFrontmatter, string(data))
		}
	}

	if !strings.Contains(stdout, "wrote 3 file(s)") {
		t.Fatalf("expected stdout to report 3 file(s) written, got:\n%s", stdout)
	}
}

func TestCLI_SkillsExport_OverwritesExistingFiles(t *testing.T) {
	targetDir := t.TempDir()
	stalePath := filepath.Join(targetDir, "dossierx-claims", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if _, _, err := execCLI(t, "skills", "export", targetDir); err != nil {
		t.Fatalf("skills export: unexpected error: %v", err)
	}

	data, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if strings.Contains(string(data), "stale content") {
		t.Fatalf("expected stale content to be overwritten, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "name: dossierx-claims") {
		t.Fatalf("expected overwritten file to contain fresh frontmatter, got:\n%s", string(data))
	}
}

func TestCLI_SkillsExport_RequiresTargetDirArg(t *testing.T) {
	if _, _, err := execCLI(t, "skills", "export"); err == nil {
		t.Fatalf("expected error when <dir> arg is omitted, got nil")
	}
}
