// build_order_render_test.go covers the optional Build Order viewer tab's
// graceful-degradation contract: present when a module has a LOCKED
// internal/buildorder.Artifact on disk, entirely absent (byte-for-byte
// unchanged output shape) when it does not — the common case for every
// project that hasn't adopted model.BuildRole/internal/buildorder at all.
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

func TestRender_BuildOrderTab_AbsentWhenNoArtifactProposed(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}

	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "build order") {
		t.Fatalf("expected no Build Order section when no artifact has been proposed at all, got:\n%s", out)
	}
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

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "build order") {
		t.Fatalf("expected no Build Order section for a proposed-but-not-yet-locked artifact, got:\n%s", out)
	}
}

func TestRender_BuildOrderTab_PresentWhenLockedArtifactExists(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)

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

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, "build order") {
		t.Fatalf("expected a Build Order section for a locked artifact, got:\n%s", out)
	}
	if !strings.Contains(out, "schema") || !strings.Contains(out, "behavior") {
		t.Fatalf("expected both phase names rendered, got:\n%s", out)
	}
	if !strings.Contains(out, module+".contract.schema") || !strings.Contains(out, module+".contract.behavior") {
		t.Fatalf("expected both claim ids rendered in the Build Order section, got:\n%s", out)
	}
	// The behavior claim's rests_on edge to the schema claim must be
	// rendered as a same-page link, matching every other component's edge
	// convention (components.writeIDListItems).
	if !strings.Contains(out, `href="#`+module+`.contract.schema"`) {
		t.Fatalf("expected a rests_on link to the schema claim, got:\n%s", out)
	}
	// C5: build_order.html used to render rests_on itself, as an inline
	// comma-separated run of bare-id <a> tags — a second, drifted copy of what
	// the shared edges footer emits. It now goes through the same
	// claimEdgeList/writeIDListItems path, so it must produce the shared
	// bulleted container and the shared labeled anchor, prefix-elided against
	// the rendering entry's own module+facet (both claims are widget.contract,
	// so the schema target is the bare-label tier).
	if !strings.Contains(out, `<div class="claim-rests-on">rests_on: <ul class="claim-edge-id-list">`) {
		t.Fatalf("expected build_order rests_on to use the shared edge-id list container, got:\n%s", out)
	}
	if !strings.Contains(out, `<a class="claim-ref" href="#`+module+`.contract.schema" data-claim-id="`+module+`.contract.schema" title="`+module+`.contract.schema"><span class="claim-ref-label">Schema</span></a>`) {
		t.Fatalf("expected build_order rests_on to use the shared labeled, bare-tier claim ref, got:\n%s", out)
	}
	// The per-claim heading is labeled too, with the machine id still on the
	// element — a build-order card is precisely where a reader needs the id,
	// since the next thing they do with it is "dossierx claim show <id>".
	if !strings.Contains(out, `<div class="k" data-claim-id="`+module+`.contract.behavior" title="`+module+`.contract.behavior">Behavior `) {
		t.Fatalf("expected a labeled, id-bearing build-order claim heading, got:\n%s", out)
	}
}

func TestRender_BuildOrderTab_OtherModuleUnaffected(t *testing.T) {
	// A project with two modules: only one has a locked build-order
	// artifact. The other module's rendered output must be completely
	// unaffected.
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

	artifact, err := buildorder.Propose(widgetClaims, cfg, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	path := buildorder.ArtifactPath(cfg, "widget")
	if err := buildorder.WriteArtifact(artifact, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
	if _, err := buildorder.Lock(path, widgetClaims, cfg); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	cat, err := catalog.Build(all, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, "Widget — build order") {
		t.Fatalf("expected widget's Build Order section present, got:\n%s", out)
	}
	if !strings.Contains(out, "gadget.contract.overview") {
		t.Fatalf("expected gadget's ordinary claim content still rendered, got:\n%s", out)
	}
	// gadget has no build-order artifact at all: its module-section must
	// not gain a "Gadget — build order" heading.
	if strings.Contains(out, "Gadget — build order") {
		t.Fatalf("expected no Build Order section for gadget (never proposed), got:\n%s", out)
	}
}

// TestRender_BuildOrderSectionVisibleNotAFacetGroup covers DX-AUD-15: the
// Build Order section must carry its OWN real id and a DISTINCT class (never
// the facet .claim-group class the tab JS's hide loop keys on), so it is no
// longer hidden on load and after every facet nav (a dead feature), and its
// cards stay deep-linkable via a dedicated resolver map in shell.html.
func TestRender_BuildOrderSectionVisibleNotAFacetGroup(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)

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

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// The section must no longer be a .claim-group (that class made the tab
	// JS hide it on load and on every nav — the whole DX-AUD-15 bug).
	if strings.Contains(out, "claim-group build-order-module") {
		t.Fatalf("Build Order section still tagged .claim-group (hide loop would keep it hidden):\n%s", out)
	}
	// It must carry its own real, distinct id + class.
	if !strings.Contains(out, `<section class="build-order-module" id="build-order-`+module+`">`) {
		t.Fatalf("Build Order section missing its own id / distinct class:\n%s", out)
	}
	// Its cards must be deep-linkable, and shell.html must ship the resolver
	// that maps a #build-order-... hash to the owning module.
	if !strings.Contains(out, `id="build-order-`+module+`.contract.schema"`) {
		t.Fatalf("Build Order card missing a deep-linkable id:\n%s", out)
	}
	if !strings.Contains(out, "buildOrderToModule") {
		t.Fatalf("shell.html missing buildOrderToModule deep-link resolver:\n%s", out)
	}
}
