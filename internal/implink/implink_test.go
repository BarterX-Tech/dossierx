package implink

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// testConfig writes a minimal, valid project.config.yaml under a fresh temp
// dir and loads it via config.LoadConfig — the only way to get a
// *config.Config whose unexported dir field (and therefore Dir(), which
// ArtifactPath/Set/Status all resolve paths against) actually points
// somewhere real, mirroring internal/render's own buildOrderTestConfig
// helper for the exact same reason.
func testConfig(t *testing.T, modules ...string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	claimsDir := filepath.Join(dir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	modYAML := ""
	for _, m := range modules {
		modYAML += "  - " + m + "\n"
	}
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n" + modYAML + "claims_dir: claims\n"
	cfgPath := filepath.Join(dir, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

// writeSourceFile writes content to a project-relative path under cfg's
// directory (creating parent directories as needed) and returns that
// project-relative path, ready to pass straight to Set.
func writeSourceFile(t *testing.T, cfg *config.Config, rel, content string) string {
	t.Helper()
	full := filepath.Join(cfg.Dir(), rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return rel
}

func lockedClaim(id, module string, role model.BuildRole) model.Claim {
	return model.Claim{
		ID:        id,
		Module:    module,
		Facet:     "contract",
		Status:    model.StatusLocked,
		BuildRole: role,
	}
}

func fixedNow(t *testing.T, ts time.Time) {
	t.Helper()
	old := nowFunc
	nowFunc = func() time.Time { return ts }
	t.Cleanup(func() { nowFunc = old })
}

// ---------------------------------------------------------------------
// Set: validation gates
// ---------------------------------------------------------------------

func TestSet_RefusesUnknownClaim(t *testing.T) {
	cfg := testConfig(t, "widget")
	if _, err := Set(nil, cfg, "widget", "widget.contract.missing", "main.go", ""); err == nil {
		t.Fatalf("expected an error linking a claim that does not exist")
	}
}

func TestSet_RefusesWrongModule(t *testing.T) {
	cfg := testConfig(t, "widget", "gadget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	rel := writeSourceFile(t, cfg, "main.go", "package widget")

	if _, err := Set(claims, cfg, "gadget", "widget.contract.main", rel, ""); err == nil {
		t.Fatalf("expected an error linking a claim into the wrong module")
	}
}

func TestSet_RefusesDraftClaim(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{{ID: "widget.contract.main", Module: "widget", Status: model.StatusDraft, BuildRole: model.BuildRoleBehavior}}
	rel := writeSourceFile(t, cfg, "main.go", "package widget")

	if _, err := Set(claims, cfg, "widget", "widget.contract.main", rel, ""); err == nil {
		t.Fatalf("expected an error linking a draft (not-yet-locked) claim")
	}
}

func TestSet_RefusesMissingFile(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}

	if _, err := Set(claims, cfg, "widget", "widget.contract.main", "does-not-exist.go", ""); err == nil {
		t.Fatalf("expected an error linking a file that does not exist on disk")
	}
}

func TestSet_RefusesAbsolutePath(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	abs := filepath.Join(cfg.Dir(), "main.go")
	if err := os.WriteFile(abs, []byte("package widget"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Set(claims, cfg, "widget", "widget.contract.main", abs, ""); err == nil {
		t.Fatalf("expected an error linking an absolute file path")
	}
}

func TestSet_RefusesPathTraversalOutsideProjectDir(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}

	// A real file that exists, but outside cfg.Dir() — reached via a
	// relative path that climbs out with "..".
	outside := filepath.Join(filepath.Dir(cfg.Dir()), "outside.txt")
	if err := os.WriteFile(outside, []byte("not part of the project"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	traversal := filepath.Join("..", filepath.Base(cfg.Dir()), "..", "outside.txt")

	if _, err := Set(claims, cfg, "widget", "widget.contract.main", traversal, ""); err == nil {
		t.Fatalf("expected an error linking a relative path that traverses outside the project directory")
	}
}

// ---------------------------------------------------------------------
// Set: append vs upsert, multi-file-per-claim, multi-claim-per-file
// ---------------------------------------------------------------------

func TestSet_AppendsNewFileForSameClaim(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	fileA := writeSourceFile(t, cfg, "a.go", "package widget // a")
	fileB := writeSourceFile(t, cfg, "b.go", "package widget // b")

	if _, err := Set(claims, cfg, "widget", "widget.contract.main", fileA, "FuncA"); err != nil {
		t.Fatalf("Set fileA: %v", err)
	}
	artifact, err := Set(claims, cfg, "widget", "widget.contract.main", fileB, "FuncB")
	if err != nil {
		t.Fatalf("Set fileB: %v", err)
	}

	if len(artifact.Links) != 1 {
		t.Fatalf("expected a single Link entry for one claim, got %d", len(artifact.Links))
	}
	if got := len(artifact.Links[0].Files); got != 2 {
		t.Fatalf("expected 2 files appended for the same claim, got %d", got)
	}
}

func TestSet_UpsertsExistingFileEntryInPlace(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	file := writeSourceFile(t, cfg, "a.go", "package widget // v1")

	fixedNow(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if _, err := Set(claims, cfg, "widget", "widget.contract.main", file, "FuncA"); err != nil {
		t.Fatalf("Set v1: %v", err)
	}

	// Mutate the file's content on disk, then re-Set with a new symbol —
	// this must update the SAME entry in place, not append a duplicate.
	full := filepath.Join(cfg.Dir(), file)
	if err := os.WriteFile(full, []byte("package widget // v2"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	fixedNow(t, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	artifact, err := Set(claims, cfg, "widget", "widget.contract.main", file, "FuncARenamed")
	if err != nil {
		t.Fatalf("Set v2: %v", err)
	}

	if len(artifact.Links) != 1 || len(artifact.Links[0].Files) != 1 {
		t.Fatalf("expected the existing file entry updated in place, not appended, got: %+v", artifact.Links)
	}
	got := artifact.Links[0].Files[0]
	if got.Symbol != "FuncARenamed" {
		t.Fatalf("expected symbol refreshed to FuncARenamed, got %q", got.Symbol)
	}
	if artifact.Links[0].LinkedAt != "2026-01-02T00:00:00Z" {
		t.Fatalf("expected LinkedAt refreshed to the second Set's timestamp, got %q", artifact.Links[0].LinkedAt)
	}
}

func TestSet_SameFileLinkedFromMultipleClaims(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{
		lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior),
		lockedClaim("widget.contract.other", "widget", model.BuildRoleAPI),
	}
	file := writeSourceFile(t, cfg, "shared.go", "package widget")

	if _, err := Set(claims, cfg, "widget", "widget.contract.main", file, ""); err != nil {
		t.Fatalf("Set (main): %v", err)
	}
	artifact, err := Set(claims, cfg, "widget", "widget.contract.other", file, "")
	if err != nil {
		t.Fatalf("Set (other): %v", err)
	}

	if len(artifact.Links) != 2 {
		t.Fatalf("expected 2 distinct claims each linking the same file, got %d", len(artifact.Links))
	}
}

func TestSet_VerificationClaimWithMultipleFiles(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.checklist", "widget", model.BuildRoleVerification)}
	test1 := writeSourceFile(t, cfg, "widget_test.go", "package widget_test // 1")
	test2 := writeSourceFile(t, cfg, "widget_extra_test.go", "package widget_test // 2")

	if _, err := Set(claims, cfg, "widget", "widget.contract.checklist", test1, "TestOne"); err != nil {
		t.Fatalf("Set test1: %v", err)
	}
	artifact, err := Set(claims, cfg, "widget", "widget.contract.checklist", test2, "TestTwo")
	if err != nil {
		t.Fatalf("Set test2: %v", err)
	}

	if len(artifact.Links) != 1 || len(artifact.Links[0].Files) != 2 {
		t.Fatalf("expected a verification claim linked to 2 test files exactly like any other claim, got: %+v", artifact.Links)
	}
}

// ---------------------------------------------------------------------
// Round trip
// ---------------------------------------------------------------------

func TestSet_WriteLoadRoundTrip(t *testing.T) {
	cfg := testConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	file := writeSourceFile(t, cfg, "a.go", "package widget")

	if _, err := Set(claims, cfg, "widget", "widget.contract.main", file, "Func"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	loaded, err := LoadArtifact(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if loaded.Module != "widget" || len(loaded.Links) != 1 {
		t.Fatalf("unexpected round-tripped artifact: %+v", loaded)
	}
}

func TestLoadArtifact_MissingFile_WrapsErrNoArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".implementation.widget.json")
	_, err := LoadArtifact(path)
	if err == nil || !errors.Is(err, ErrNoArtifact) {
		t.Fatalf("expected an ErrNoArtifact-wrapping error, got: %v", err)
	}
}
