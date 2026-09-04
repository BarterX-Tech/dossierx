// build_order_render_test.go covers the Build order tab's presence contract:
// present, as a top-level tab of its own, when a module has a LOCKED
// internal/buildorder.Artifact on disk; entirely absent — the sidebar group,
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

	for _, want := range []string{
		// the sidebar entry, its own group after Tracks, never under a facet
		`<span>Build order</span>`,
		`<button class="sec-tab" data-target="#dossierx-build-order" data-default-target="#dossierx-build-order-widget">Build order</button>`,
		// the section, the strip, the module group
		`<section class="module-section build-order-section" id="dossierx-build-order" hidden>`,
		`<div class="bo-modules">`,
		`<button class="subtab" data-target="#dossierx-build-order-widget">Widget</button>`,
		`<section class="claim-group bo-module" id="dossierx-build-order-widget" hidden>`,
		// the payload block, inside .content-area
		`<main class="content-area"><script type="application/json" id="dossierx-build-orders">`,
		// the two client files, after graph-ui
		"__esbuild_esm_mermaid_nm",
		"build-order-ui.js",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in a viewer with a locked order", want)
		}
	}

	// Six blocks in the fixed sequence, each with its definition and counts.
	phases := []string{"orientation", "schema", "behavior", "api", "verification", "excluded"}
	if got := strings.Count(out, `<section class="bo-phase" data-phase="`); got != 6 {
		t.Errorf("got %d .bo-phase blocks, want 6", got)
	}
	last := -1
	for i, phase := range phases {
		at := strings.Index(out, `<section class="bo-phase" data-phase="`+phase+`"`)
		if at < 0 {
			t.Errorf("no .bo-phase block for %s", phase)
			continue
		}
		if at < last {
			t.Errorf("%s block appears before the previous phase's; the sequence is fixed", phase)
		}
		last = at
		role := model.BuildRole(phase)
		if phase == "excluded" {
			role = model.BuildRoleOutOfScope
		}
		def := template_HTMLEscape(buildorder.PhaseDefinition(role))
		if !strings.Contains(out, `<p class="bo-phase__def">`+def+`</p>`) {
			t.Errorf("%s block lacks its definition from PhaseDefinition: want %q", phase, def)
		}
		_ = i
	}
	for _, want := range []string{
		`data-phase="schema" data-number="2" data-count="1" data-levels="1" data-locked="1"`,
		`data-phase="orientation" data-number="1" data-count="0" data-levels="0" data-locked="0"`,
		`data-phase="excluded" data-number="0" data-count="0"`,
		`<span class="bo-phase__num">phase 2 of 5</span>`,
		`<span class="bo-phase__num">excluded</span>`,
		`<p class="bo-phase__meta">1 claim · 1 level · 1 locked</p>`,
		`<p class="bo-empty">no claims in this module</p>`,
		`<p class="bo-empty">no excluded claims in this module</p>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the block markup", want)
		}
	}

	// The diagram text: a <pre class="mermaid"> whose text carries the
	// flowchart and, under the CSS palette, no colour literal.
	pre := `<pre class="mermaid" data-module="widget" data-phase="behavior">`
	at := strings.Index(out, pre)
	if at < 0 {
		t.Fatalf("no mermaid block for the behavior phase")
	}
	end := strings.Index(out[at:], "</pre>")
	text := out[at+len(pre) : at+end]
	if !strings.Contains(text, "flowchart TD") {
		t.Errorf("behavior diagram text lacks flowchart TD: %q", text)
	}
	if strings.Contains(text, "fill:") {
		t.Errorf("the page's diagram must carry no colour literal (CSS palette): %q", text)
	}
	if !strings.Contains(text, "widget_contract_schema -.-&gt; widget_contract_behavior") {
		t.Errorf("expected the ghost edge, html-escaped, in the pre text: %q", text)
	}
	if got := strings.Count(out, `<pre class="mermaid" data-module=`); got != 2 {
		t.Errorf("expected exactly two diagrams (schema, behavior), got %d", got)
	}

	assertAbsent(t, out, legacyMarkers, "locked order")
	// No per-module "Build Order" sub-tab and no card list under a diagram.
	if strings.Contains(out, `id="dossierx-build-order-widget.contract.schema"`) {
		t.Error("the old per-claim build-order card id survives")
	}
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

	if !strings.Contains(out, `id="dossierx-build-order-widget"`) {
		t.Fatalf("expected widget's Build order group present")
	}
	if !strings.Contains(out, "gadget.contract.overview") {
		t.Fatalf("expected gadget's ordinary claim content still rendered")
	}
	if strings.Contains(out, `id="dossierx-build-order-gadget"`) || strings.Contains(out, `data-target="#dossierx-build-order-gadget"`) {
		t.Fatalf("expected no Build order entry for gadget (never proposed)")
	}
	if got := strings.Count(out, `class="subtab" data-target="#dossierx-build-order-`); got != 1 {
		t.Errorf("module strip holds %d buttons, want 1", got)
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

	sectionAt := strings.Index(out, `<section class="module-section build-order-section" id="dossierx-build-order" hidden>`)
	moduleAt := strings.Index(out, `<section class="module-section" id="widget" hidden>`)
	mainEnd := strings.Index(out, "</main>")
	if sectionAt < 0 || moduleAt < 0 || mainEnd < 0 {
		t.Fatalf("missing section/module/main markers: %d %d %d", sectionAt, moduleAt, mainEnd)
	}
	if sectionAt < moduleAt {
		t.Error("the Build order section must come AFTER every module section")
	}
	if sectionAt > mainEnd {
		t.Error("the Build order section must be inside <main class=\"content-area\">")
	}
	payloadAt := strings.Index(out, `id="dossierx-build-orders"`)
	if payloadAt < 0 || payloadAt > moduleAt {
		t.Error("the payload block must be the first child of .content-area, before the first module")
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
	for _, want := range []string{
		`<section class="module-section" id="build-order" hidden>`,
		`<section class="module-section build-order-section" id="dossierx-build-order" hidden>`,
		`<section class="claim-group bo-module" id="dossierx-build-order-build-order" hidden>`,
		`data-target="#dossierx-build-order" data-default-target="#dossierx-build-order-build-order"`,
	} {
		if got := strings.Count(out, want); got != 1 {
			t.Errorf("%q appears %d times, want exactly once", want, got)
		}
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
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("module %q: Render error = %v, want one containing %q", tc.module, err, tc.wantErr)
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
