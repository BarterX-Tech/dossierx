// implink_view_test.go covers the optional implementation-link viewer
// extension's graceful-degradation contract, mirroring
// build_order_render_test.go's approach for the sibling Build Order tab:
// present (an extra "implemented in" edges-footer line) only when a module
// has an implementation-link artifact covering the rendered claim,
// entirely absent (byte-for-byte unchanged output) when it does not.
package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// implinkTestConfig writes a minimal, valid project.config.yaml under a
// fresh temp dir and loads it via config.LoadConfig — the only way to get a
// *config.Config whose unexported dir field (and therefore Dir(), which
// implink.ArtifactPath/Set resolve against) points somewhere real, mirroring
// build_order_render_test.go's buildOrderTestConfig.
func implinkTestConfig(t *testing.T, module string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	claimsDir := filepath.Join(dir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - " + module + "\nclaims_dir: claims\n"
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, cfgYAML)

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func implinkTestClaim(module string) model.Claim {
	return model.Claim{
		ID:        module + ".contract.main",
		Module:    module,
		Facet:     "contract",
		Status:    model.StatusLocked,
		Layout:    model.LayoutCard,
		Body:      "main claim",
		BuildRole: model.BuildRoleBehavior,
		Governed:  model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
	}
}

func TestRender_ImplementedIn_AbsentWhenNoArtifact(t *testing.T) {
	module := "widget"
	cfg := implinkTestConfig(t, module)
	claims := []model.Claim{implinkTestClaim(module)}

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "implemented in") {
		t.Fatalf("expected no 'implemented in' line when no implink artifact exists, got:\n%s", out)
	}
	if !strings.Contains(out, module+".contract.main") {
		t.Fatalf("expected ordinary claim content still present, got:\n%s", out)
	}
}

func TestRender_ImplementedIn_PresentWhenLinked(t *testing.T) {
	module := "widget"
	cfg := implinkTestConfig(t, module)
	claim := implinkTestClaim(module)
	claims := []model.Claim{claim}

	srcPath := filepath.Join(cfg.Dir(), "widget.go")
	writeFile(t, srcPath, "package widget")

	if _, err := implink.Set(claims, cfg, module, claim.ID, "widget.go", "Run"); err != nil {
		t.Fatalf("implink.Set: %v", err)
	}

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "implemented in") {
		t.Fatalf("expected an 'implemented in' line for a linked claim, got:\n%s", out)
	}
	if !strings.Contains(out, "widget.go#Run") {
		t.Fatalf("expected the linked file and symbol rendered together, got:\n%s", out)
	}
	if strings.Contains(out, `pill pw">drifted`) {
		t.Fatalf("expected no drifted pill for a freshly-linked, unchanged file, got:\n%s", out)
	}
}

func TestRender_ImplementedIn_DriftedPillWhenFileChangedSinceLinking(t *testing.T) {
	module := "widget"
	cfg := implinkTestConfig(t, module)
	claim := implinkTestClaim(module)
	claims := []model.Claim{claim}

	srcPath := filepath.Join(cfg.Dir(), "widget.go")
	writeFile(t, srcPath, "package widget // v1")
	if _, err := implink.Set(claims, cfg, module, claim.ID, "widget.go", "Run"); err != nil {
		t.Fatalf("implink.Set: %v", err)
	}
	// Mutate the file after linking, without re-Set-ing.
	writeFile(t, srcPath, "package widget // v2, mutated")

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `pill pw">drifted`) {
		t.Fatalf("expected a drifted pill for a linked file that changed since linking, got:\n%s", out)
	}
}

func TestRender_ImplementedIn_OtherModuleUnaffected(t *testing.T) {
	dir := t.TempDir()
	claimsDir := filepath.Join(dir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - gadget\nclaims_dir: claims\n"
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, cfgYAML)
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	widgetClaim := implinkTestClaim("widget")
	gadgetClaim := implinkTestClaim("gadget")
	all := []model.Claim{widgetClaim, gadgetClaim}

	srcPath := filepath.Join(cfg.Dir(), "widget.go")
	writeFile(t, srcPath, "package widget")
	if _, err := implink.Set(all, cfg, "widget", widgetClaim.ID, "widget.go", ""); err != nil {
		t.Fatalf("implink.Set: %v", err)
	}

	cat, err := catalog.Build(all, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, "implemented in") {
		t.Fatalf("expected widget's implemented-in line present, got:\n%s", out)
	}
	if !strings.Contains(out, gadgetClaim.ID) {
		t.Fatalf("expected gadget's ordinary claim content still rendered, got:\n%s", out)
	}
}
