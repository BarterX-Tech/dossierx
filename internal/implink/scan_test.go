package implink

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// scanTestConfig mirrors testConfig but additionally sets source_dirs to a
// "src" subdirectory it creates, since Scan (unlike Set) needs a real,
// validated cfg.SourceDirs to have anything to walk.
func scanTestConfig(t *testing.T, modules ...string) (cfg *config.Config, srcDir string) {
	t.Helper()
	dir := t.TempDir()
	claimsDir := filepath.Join(dir, "claims")
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir src dir: %v", err)
	}
	modYAML := ""
	for _, m := range modules {
		modYAML += "  - " + m + "\n"
	}
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n" + modYAML +
		"claims_dir: claims\nsource_dirs:\n  - src\n"
	cfgPath := filepath.Join(dir, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loaded, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return loaded, src
}

func writeScanFile(t *testing.T, srcDir, rel, content string) {
	t.Helper()
	full := filepath.Join(srcDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestScan_NoSourceDirs_IsANoOp(t *testing.T) {
	cfg := testConfig(t, "widget") // no source_dirs set at all
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.FilesScanned != 0 || len(report.Matches) != 0 || len(report.Errors) != 0 {
		t.Fatalf("expected an all-zero report when source_dirs is unset, got %+v", report)
	}
}

func TestScan_ValidTag_ReconcilesALink(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	writeScanFile(t, srcDir, "main.py", "# docs-claim: widget.contract.main\ndef do_thing():\n    pass\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("expected no scan errors, got %+v", report.Errors)
	}
	if len(report.Matches) != 1 {
		t.Fatalf("expected exactly 1 match, got %+v", report.Matches)
	}
	if report.Matches[0].ClaimID != "widget.contract.main" || report.Matches[0].File != "src/main.py" {
		t.Fatalf("unexpected match: %+v", report.Matches[0])
	}
	if report.Matches[0].Symbol != "do_thing" {
		t.Fatalf("expected the symbol heuristic to capture do_thing, got %q", report.Matches[0].Symbol)
	}

	// The link must actually have landed in the real artifact via the same
	// Set() path an explicit call would use.
	artifact, err := LoadArtifact(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if len(artifact.Links) != 1 || artifact.Links[0].ClaimID != "widget.contract.main" {
		t.Fatalf("expected the scan to have written a real link, got %+v", artifact.Links)
	}
}

func TestScan_UnknownClaimID_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	writeScanFile(t, srcDir, "main.py", "# docs-claim: widget.contract.mian\ndef do_thing():\n    pass\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 {
		t.Fatalf("expected no successful matches for a typo'd claim id, got %+v", report.Matches)
	}
	if len(report.Errors) != 1 || report.Errors[0].ClaimID != "widget.contract.mian" {
		t.Fatalf("expected exactly 1 scan error naming the typo'd id, got %+v", report.Errors)
	}
}

func TestScan_DraftClaim_IsAScanError(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{{ID: "widget.contract.main", Module: "widget", Status: model.StatusDraft}}
	writeScanFile(t, srcDir, "main.py", "# docs-claim: widget.contract.main\ndef do_thing():\n    pass\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 {
		t.Fatalf("expected no successful matches for a still-draft claim, got %+v", report.Matches)
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected exactly 1 scan error for the not-locked claim, got %+v", report.Errors)
	}
}

func TestScan_MultipleTagsAcrossFiles(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{
		lockedClaim("widget.contract.a", "widget", model.BuildRoleBehavior),
		lockedClaim("widget.contract.b", "widget", model.BuildRoleBehavior),
		lockedClaim("widget.contract.c", "widget", model.BuildRoleVerification),
	}
	writeScanFile(t, srcDir, "a.go", "// docs-claim: widget.contract.a\nfunc doA() {}\n")
	writeScanFile(t, srcDir, "sub/b.go",
		"// docs-claim: widget.contract.b\nfunc doB() {}\n\n// docs-claim: widget.contract.c\nfunc doC() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors, got %+v", report.Errors)
	}
	if len(report.Matches) != 3 {
		t.Fatalf("expected 3 matches across 2 files, got %+v", report.Matches)
	}
}

func TestScan_GoModuleImportPathNeverMistakenForATag(t *testing.T) {
	// Regression guard: this package's own source imports
	// "github.com/BarterX-Tech/dossierx/internal/..." all over the place; Scan must
	// never confuse an ordinary import line for a docs-claim tag just
	// because both are lines of text in a source file.
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	writeScanFile(t, srcDir, "main.go", "import \"github.com/BarterX-Tech/dossierx/internal/model\"\n\nfunc main() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 0 || len(report.Errors) != 0 {
		t.Fatalf("expected zero matches/errors for a file with no docs-claim tag, got matches=%+v errors=%+v", report.Matches, report.Errors)
	}
}
