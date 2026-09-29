package render

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func projectTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\n  - gateway\nclaims_dir: claims\n")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func projectTestClaim(module, slug string, status model.Status) model.Claim {
	return model.Claim{
		ID:      module + ".contract." + slug,
		Module:  module,
		Facet:   "contract",
		Status:  status,
		Layout:  model.LayoutCard,
		Body:    slug + " body",
		RestsOn: model.RestsNone("test fixture"),
	}
}

func renderProject(t *testing.T, cfg *config.Config, claims []model.Claim) string {
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

// TestRender_ProjectClaimIsNotAnUngroupedModule pins where a project claim
// (NIT-25) appears in the reading view: under the constitution's "Project
// claims" tab, and nowhere else. It has no module and no facet by design, so
// the catch-all bucket that keeps a MALFORMED claim visible must not adopt
// it — otherwise the sidebar lists an "Ungrouped" module, counts it under
// Modules, and renders the claim twice, directly beneath the pin that says
// PROJECT — NOT A MODULE. A claim that IS malformed (empty module, no scope)
// still lands in the bucket, so nothing is dropped.
func TestRender_ProjectClaimIsNotAnUngroupedModule(t *testing.T) {
	cfg := projectTestConfig(t)
	overview := projectTestClaim("widget", "overview", model.StatusDraft)
	overview.RestsOn = model.RestsOnIDs("project.scope")
	scope := model.Claim{
		ID:      "project.scope",
		Scope:   model.ScopeProject,
		Status:  model.StatusDraft,
		Layout:  model.LayoutCard,
		Body:    "Every widget is kept under <b>one</b> roof.",
		RestsOn: model.RestsNone("the roof above it is the constitution"),
	}
	malformed := model.Claim{ID: "stray.contract.x", Module: "", Facet: "contract", Status: model.StatusDraft, Layout: model.LayoutCard, Body: "no module", RestsOn: model.RestsNone("test fixture")}

	cat, err := catalog.Build([]model.Claim{scope, overview, malformed}, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	rendered := map[string]template.HTML{scope.ID: "SCOPE", overview.ID: "OVERVIEW", malformed.ID: "MALFORMED"}
	groups := buildGroups(cat, cfg, rendered)
	var ungrouped []template.HTML
	for _, g := range groups {
		for _, h := range g.Claims {
			if h == "SCOPE" {
				t.Fatalf("the project claim landed in module group %q/%q", g.Module, g.Facet)
			}
		}
		if g.Module == ungroupedModuleName {
			ungrouped = append(ungrouped, g.Claims...)
		}
	}
	if len(ungrouped) != 1 || ungrouped[0] != "MALFORMED" {
		t.Fatalf("ungrouped bucket = %v, want exactly the malformed claim", ungrouped)
	}

	// Without a roof file the Project claims tab still lists the store and
	// The file tab says so; with one, both render.
	out := renderProject(t, cfg, []model.Claim{scope, overview})
	if !strings.Contains(out, `<p class="claims-empty">No constitution.yaml yet.</p>`) {
		t.Errorf("a project with no constitution.yaml must say so under The file")
	}
	if !strings.Contains(out, ` id="project.scope"`) {
		t.Errorf("a project with no constitution.yaml must still list its project claims")
	}
	writeFile(t, cfg.ConstitutionPath(), "status: draft\ninvariants:\n  - slug: one-roof\n    title: One roof\n    body: This fixture keeps *every* module under <b>one</b> roof.\n")
	out = renderProject(t, cfg, []model.Claim{scope, overview})
	if !strings.Contains(out, `<p class="constitution-meter">12 of 800 words · draft</p>`) {
		t.Errorf("the meter must count the roof's words (title and body, letter/number runs)")
	}
	if !strings.Contains(out, `<p>This fixture keeps *every* module under &lt;b&gt;one&lt;/b&gt; roof.</p>`) {
		t.Errorf("the roof's body must render as escaped text")
	}
	for _, absent := range []string{
		`data-target="#ungrouped"`,
		`<section class="module-section" id="ungrouped"`,
		`<b>one</b>`,
	} {
		if strings.Contains(out, absent) {
			t.Errorf("rendered viewer contains %q", absent)
		}
	}
	for _, want := range []string{
		`<span>Modules</span><span class="system-nav-group__count">1</span>`,
		`Every widget is kept under &lt;b&gt;one&lt;/b&gt; roof.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered viewer lacks %q", want)
		}
	}
	if n := strings.Count(out, ` id="project.scope"`); n != 1 {
		t.Errorf("the project claim is rendered %d times, want once (under Project claims)", n)
	}
}
