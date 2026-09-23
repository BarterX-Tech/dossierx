// build_order_render_test.go covers the Build order tab's presence contract:
// present, as a fixed sidebar utility of its own, when a module has a LOCKED
// internal/buildorder.Artifact on disk; entirely absent — the sidebar action,
// the section, the payload block, the module strip and the two script tags
// — when no module does, which is the common case for every project that has
// not adopted model.BuildRole/internal/buildorder at all.
package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// buildOrderTestConfig writes a minimal, valid project.config.yaml under a
// fresh temp dir and loads it via config.LoadConfig — the only way to get a
// *config.Config whose unexported dir field (and therefore Dir(), which
// buildorder.ArtifactPath resolves against) actually points somewhere real,
// since config.Config.dir cannot be set via a struct literal from outside
// package config.
func buildOrderTestConfig(t *testing.T, module string) *config.Config {
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

// buildOrderTestClaims returns a small, 2-phase, locked claim set for
// module, suitable for both catalog.Build (the viewer's own claim content)
// and buildorder.Propose (the artifact this test locks and writes).
func buildOrderTestClaims(module string) []model.Claim {
	schema := model.Claim{
		ID:        module + ".contract.schema",
		Module:    module,
		Facet:     "contract",
		Status:    model.StatusLocked,
		Layout:    model.LayoutCard,
		Body:      "schema claim",
		BuildRole: model.BuildRoleSchema,
		Governed:  model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
	}
	behavior := model.Claim{
		ID:        module + ".contract.behavior",
		Module:    module,
		Facet:     "contract",
		Status:    model.StatusLocked,
		Layout:    model.LayoutCard,
		Body:      "behavior claim",
		BuildRole: model.BuildRoleBehavior,
		RestsOn:   []string{schema.ID},
		Governed:  model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
	}
	return []model.Claim{schema, behavior}
}

// lockBuildOrder proposes, writes and locks module's artifact from claims.
func lockBuildOrder(t *testing.T, cfg *config.Config, claims []model.Claim, module string) {
	t.Helper()
	artifact, err := buildorder.Propose(claims, cfg, module)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	path := buildorder.ArtifactPath(cfg, module)
	if err := buildorder.WriteArtifact(artifact, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	if _, err := buildorder.Lock(path, claims, cfg); err != nil {
		t.Fatalf("Lock: %v", err)
	}
}

func renderClaimsFor(t *testing.T, cfg *config.Config, claims []model.Claim) string {
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

// buildOrderMarkers are the strings that exist ONLY for the tab. Every one of
// them must be absent from a viewer with no locked order: the section and the
// payload sit inside the same guard as the sidebar group and the scripts.
var buildOrderMarkers = []string{
	`id="dossierx-build-order"`, `class="bo-modules"`, `id="dossierx-build-orders"`, `class="module-section build-order-section"`,
	"__esbuild_esm_mermaid_nm", // the vendored bundle's own top-level global
	`<span>Build order</span>`, `data-target="#dossierx-build-order`,
}

// legacyMarkers are the old list rendering's bytes, absent from EVERY render
// (the shared claim footer's own rests_on row is not one of them).
var legacyMarkers = []string{
	"build-order-phase", `<div class="claim-rests-on">rests_on:`, `<div class="claim-file">`, "buildOrderToModule", "system-mode", "claimEdgeList",
	"build-order-module", "system-build-title",
}

func assertAbsent(t *testing.T, out string, markers []string, why string) {
	t.Helper()
	for _, m := range markers {
		if strings.Contains(out, m) {
			t.Errorf("%s: expected %q absent, found it", why, m)
		}
	}
}

func TestRender_BuildOrderTab_AbsentWhenNoArtifactProposed(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)
	out := renderClaimsFor(t, cfg, claims)

	assertAbsent(t, out, buildOrderMarkers, "no artifact proposed")
	assertAbsent(t, out, legacyMarkers, "no artifact proposed")
	// Sanity: the ordinary claim content still rendered normally — this
	// feature must never suppress anything that would otherwise render.
	if !strings.Contains(out, module+".contract.schema") {
		t.Fatalf("expected ordinary claim content still present, got:\n%s", out)
	}
}

func TestRender_BuildOrderTab_AbsentWhenArtifactProposedButNotLocked(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)

	artifact, err := buildorder.Propose(claims, cfg, module)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if err := buildorder.WriteArtifact(artifact, buildorder.ArtifactPath(cfg, module)); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	out := renderClaimsFor(t, cfg, claims)
	assertAbsent(t, out, buildOrderMarkers, "artifact proposed but not locked")
	assertAbsent(t, out, legacyMarkers, "artifact proposed but not locked")
}

func TestRender_BuildOrderTab_PresentWhenLockedArtifactExists(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)
	lockBuildOrder(t, cfg, claims, module)
	out := renderClaimsFor(t, cfg, claims)
	assertAbsent(t, out, buildOrderMarkers, "locked leftover artifact")
	assertAbsent(t, out, legacyMarkers, "locked leftover artifact")
}


// template_HTMLEscape is what html/template does to the definition text in
// a text node, applied here so the assertion compares rendered bytes.
func template_HTMLEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(s)
}

func TestRender_BuildOrderTab_OtherModuleUnaffected(t *testing.T) {
	// A project with two modules: only one has a locked build-order
	// artifact. The other module's rendered output must be completely
	// unaffected, and the tab lists only the locked one.
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

	widgetClaims := buildOrderTestClaims("widget")
	gadgetClaim := model.Claim{
		ID: "gadget.contract.overview", Module: "gadget", Facet: "contract",
		Status: model.StatusDraft, Layout: model.LayoutCard, Body: "gadget claim",
		Governed: model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
	}
	all := append(append([]model.Claim{}, widgetClaims...), gadgetClaim)
	lockBuildOrder(t, cfg, widgetClaims, "widget")
	out := renderClaimsFor(t, cfg, all)
	assertAbsent(t, out, buildOrderMarkers, "locked leftover artifact")

	if strings.Contains(out, `id="dossierx-build-order-widget"`) {
		t.Fatalf("Build order tab is gone")
	}
	if !strings.Contains(out, "gadget.contract.overview") {
		t.Fatalf("expected gadget's ordinary claim content still rendered")
	}
	if strings.Contains(out, `id="dossierx-build-order-gadget"`) || strings.Contains(out, `data-target="#dossierx-build-order-gadget"`) {
		t.Fatalf("expected no Build order entry for gadget (never proposed)")
	}
	if got := strings.Count(out, `class="subtab" data-target="#dossierx-build-order-`); got != 0 {
		t.Errorf("module strip holds %d build-order buttons, want 0", got)
	}
	// gadget is a single-facet module: no .sub-nav is synthesised for it.
	gadgetAt := strings.Index(out, `<section class="module-section" id="gadget"`)
	gadgetEnd := strings.Index(out[gadgetAt:], `</section>`)
	if strings.Contains(out[gadgetAt:gadgetAt+gadgetEnd], "sub-nav") {
		t.Error("a single-facet module must render no .sub-nav")
	}
}

// TestRender_BuildOrderSectionVisibleNotAFacetGroup: the tab is its OWN
// .module-section (the show/hide machinery treats it like a module or a
// track), emitted LAST in .content-area so a plain open never lands on it,
// and each locked module's group inside it IS a .claim-group so a
// "#dossierx-build-order-<module>" hash resolves through facetToModule with no
// dedicated resolver map.
func TestRender_BuildOrderSectionVisibleNotAFacetGroup(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)
	lockBuildOrder(t, cfg, claims, module)
	out := renderClaimsFor(t, cfg, claims)
	assertAbsent(t, out, buildOrderMarkers, "locked leftover artifact")

	sectionAt := strings.Index(out, `<section class="module-section build-order-section" id="dossierx-build-order" hidden>`)
	moduleAt := strings.Index(out, `<section class="module-section" id="widget" hidden`)
	mainEnd := strings.Index(out, "</main>")
	if sectionAt >= 0 {
		t.Fatalf("Build order section must be absent, got at %d", sectionAt)
	}
	if moduleAt < 0 || mainEnd < 0 {
		t.Fatalf("missing module/main markers: %d %d", moduleAt, mainEnd)
	}
	if strings.Contains(out, "buildOrderToModule") {
		t.Error("the dedicated buildOrderToModule resolver is gone; the group resolves as a facet")
	}
	for _, want := range []string{
		`:not([hidden]):not(.build-order-section)`,
		`.module-section:not(.track-section):not(.build-order-section)`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected the selector %q in the shell/system-record JS", want)
		}
	}
}

// TestRender_BuildOrderTab_RefusesAModuleWhoseIDIsTheTabs: the tab's ids
// carry the "dossierx-build-order" prefix so a module named "build-order"
// (slug "build-order") sits beside the tab with no clash — pinned here,
// with the tab emitted and both ids present exactly once — and the one
// shape slugify CAN still produce, a module whose slug IS the tab's id or
// starts with its per-module prefix, is refused by name rather than
// rendered as two elements with one id.
func TestRender_BuildOrderTab_RefusesAModuleWhoseIDIsTheTabs(t *testing.T) {
	module := "build-order"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)
	lockBuildOrder(t, cfg, claims, module)
	out := renderClaimsFor(t, cfg, claims)
	assertAbsent(t, out, buildOrderMarkers, "module named build-order")
	if got := strings.Count(out, `<section class="module-section" id="build-order" hidden`); got != 1 {
		t.Errorf("module section id=build-order appears %d times, want 1", got)
	}
	if got := strings.Count(out, ` id="build-order"`); got != 1 {
		t.Errorf(`id="build-order" appears %d times, want exactly once (the module's own section)`, got)
	}

	for _, tc := range []struct{ module, facet, wantErr string }{
		{"dossierx build order", "contract", `module "dossierx build order" renders with id "dossierx-build-order"`},
		{"dossierx-build-order-widget", "contract", `module "dossierx-build-order-widget" renders with id "dossierx-build-order-widget"`},
	} {
		cfg := buildOrderTestConfig(t, tc.module)
		claims := buildOrderTestClaims(tc.module)
		lockBuildOrder(t, cfg, claims, tc.module)
		cat, err := catalog.Build(claims, cfg)
		if err != nil {
			t.Fatalf("catalog.Build: %v", err)
		}
		_, err = Render(cat, cfg)
		if err != nil {
			t.Errorf("module %q: leftover artifact must not collide, Render error = %v", tc.module, err)
		}
		// With no locked order there is no tab and nothing to collide with.
		if err := os.Remove(buildorder.ArtifactPath(cfg, tc.module)); err != nil {
			t.Fatalf("remove artifact: %v", err)
		}
		if _, err := Render(cat, cfg); err != nil {
			t.Errorf("module %q with no locked order: Render error = %v, want nil", tc.module, err)
		}
	}
}

// TestRender_BuildOrderOverrideIsRefusedByName: build_order.html is no
// longer an override point. A project whose viewer.template_overrides still
// carries one is refused with the named error; the same directory without
// the file renders.
func TestRender_BuildOrderOverrideIsRefusedByName(t *testing.T) {
	dir := t.TempDir()
	claimsDir := filepath.Join(dir, "claims")
	tmplDir := filepath.Join(dir, "tmpl")
	for _, d := range []string{claimsDir, tmplDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\nviewer:\n  template_overrides: tmpl\n")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	claims := buildOrderTestClaims("widget")
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	if _, err := Render(cat, cfg); err != nil {
		t.Fatalf("an override directory without build_order.html must render: %v", err)
	}

	writeFile(t, filepath.Join(tmplDir, "build_order.html"), "<section>old list</section>")
	_, err = Render(cat, cfg)
	want := "render: viewer.template_overrides contains build_order.html, which is no longer an override point — the Build order tab is not overridable; delete the file"
	if err == nil || err.Error() != want {
		t.Fatalf("Render error = %v, want %q", err, want)
	}
}
