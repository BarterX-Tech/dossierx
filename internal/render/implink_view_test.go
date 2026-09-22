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

// ---------------------------------------------------------------------
// The gate's two rows (issue #78). They exist only for a project that named
// its source_dirs — the same precondition check's code-link gate has — so the
// viewer never shows a gap check does not refuse on, and never hides one it does.
// ---------------------------------------------------------------------

// implinkGatedConfig is implinkTestConfig with source_dirs: [src] (the
// directory must exist, or config refuses to load).
func implinkGatedConfig(t *testing.T, module string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"claims", "src"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - "+module+"\nclaims_dir: claims\nsource_dirs:\n  - src\n")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func renderClaims(t *testing.T, claims []model.Claim, cfg *config.Config) string {
	t.Helper()
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

func TestRender_NotLinkedRow_PresentForGatedUnlinkedClaim(t *testing.T) {
	module := "widget"
	cfg := implinkGatedConfig(t, module)
	claim := implinkTestClaim(module)
	orientation := implinkTestClaim(module)
	orientation.ID = module + ".contract.context"
	orientation.BuildRole = model.BuildRoleOrientation
	orientation.Body = "orientation, no code expected"

	out := renderClaims(t, []model.Claim{claim, orientation}, cfg)
	if n := strings.Count(out, `class="claim-unlinked`); n != 1 {
		t.Fatalf("expected exactly one unlinked row (the behavior claim, not the orientation one), got %d in:\n%s", n, out)
	}
	if !strings.Contains(out, `<span class="pill pw">not linked to code</span>`) {
		t.Fatalf("expected the not-linked pill, got:\n%s", out)
	}
	if strings.Contains(out, "No relationships") && strings.Count(out, "No relationships") > 1 {
		t.Fatalf("the unlinked claim must not read 'No relationships'")
	}
}

func TestRender_NotLinkedRow_AbsentWithoutSourceDirs(t *testing.T) {
	module := "widget"
	cfg := implinkTestConfig(t, module) // no source_dirs: no gate, no row
	out := renderClaims(t, []model.Claim{implinkTestClaim(module)}, cfg)
	if strings.Contains(out, "claim-unlinked") || strings.Contains(out, "not linked to code") {
		t.Fatalf("an ungated project must show no unlinked row:\n%s", out)
	}
}

func TestRender_PartialStepsRow_NamesMissingSteps(t *testing.T) {
	module := "widget"
	cfg := implinkGatedConfig(t, module)
	claim := implinkTestClaim(module)
	claim.Layout = model.LayoutSteps
	claim.Steps = []string{"alpha", "beta", "gamma"}
	claim.Body = ""
	claims := []model.Claim{claim}

	src := filepath.Join(cfg.Dir(), "src", "widget.go")
	writeFile(t, src, "// dossierx-step: "+claim.ID+" #2 "+implink.StepContentHash("beta")+"\nfunc Beta() {}\n")
	if _, err := implink.Scan(claims, cfg); err != nil {
		t.Fatalf("implink.Scan: %v", err)
	}

	out := renderClaims(t, claims, cfg)
	if !strings.Contains(out, `class="claim-partial-link claim-relationship-extra">steps linked: 1 of 3 <span class="pill pw">missing step 1, 3</span>`) {
		t.Fatalf("expected the partial row naming steps 1 and 3, got:\n%s", out)
	}
	if strings.Contains(out, "not linked to code") {
		t.Fatalf("a partially linked claim is linked, not unlinked")
	}

	// Cover the rest: the row goes away.
	writeFile(t, src, "// dossierx-step: "+claim.ID+" #1 "+implink.StepContentHash("alpha")+"\n// dossierx-step: "+claim.ID+" #2 "+implink.StepContentHash("beta")+"\n// dossierx-step: "+claim.ID+" #3 "+implink.StepContentHash("gamma")+"\nfunc All() {}\n")
	if _, err := implink.Scan(claims, cfg); err != nil {
		t.Fatalf("implink.Scan (full): %v", err)
	}
	out = renderClaims(t, claims, cfg)
	if strings.Contains(out, "claim-partial-link") {
		t.Fatalf("a fully stepped claim must show no partial row:\n%s", out)
	}
}

func TestRender_LinksNone_ShowsReasonAndSkipsUnlinkedPill(t *testing.T) {
	module := "widget"
	cfg := implinkGatedConfig(t, module)
	claim := implinkTestClaim(module)
	claim.Links = &model.ClaimLinks{Mode: model.LinksModeNone, Reason: "no implementing declaration"}
	out := renderClaims(t, []model.Claim{claim}, cfg)
	if strings.Contains(out, "not linked to code") {
		t.Fatalf("links.none must not show the unlinked pill:\n%s", out)
	}
	if !strings.Contains(out, "implemented in: none") || !strings.Contains(out, "no implementing declaration") {
		t.Fatalf("expected links none + reason:\n%s", out)
	}
}

func TestRender_ProcessOwnedStep_FromYAML(t *testing.T) {
	module := "widget"
	cfg := implinkGatedConfig(t, module)
	claim := implinkTestClaim(module)
	claim.Layout = model.LayoutSteps
	claim.Steps = []string{"freeze", "run"}
	claim.Body = ""
	claim.StepsOwnedBy = model.StepsOwnedBy{1: model.StepOwnerProcess, 2: model.StepOwnerProcess}
	out := renderClaims(t, []model.Claim{claim}, cfg)
	if strings.Contains(out, "not linked to code") || strings.Contains(out, "claim-partial-link") {
		t.Fatalf("fully process-owned steps must be complete:\n%s", out)
	}
	if !strings.Contains(out, "step 1: process-owned") || !strings.Contains(out, "step 2: process-owned") {
		t.Fatalf("expected process-owned rows:\n%s", out)
	}
}
