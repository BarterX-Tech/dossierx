package render

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRender_ClaimBodiesStayInSurfaceTemplates(t *testing.T) {
	claims := []model.Claim{
		{
			ID: "widget.contract.alpha", Module: "widget", Facet: "contract",
			Status: model.StatusDraft, Layout: model.LayoutCard, Body: "alpha body",
			Governed: model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
		},
		{
			ID: "widget.internals.beta", Module: "widget", Facet: "internals",
			Status: model.StatusDraft, Layout: model.LayoutCard, Body: "beta body",
			Governed: model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
		},
	}
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract", "internals"}}
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := strings.Count(out, `class="dossierx-surface-template"`); got < 2 {
		t.Fatalf("surface templates = %d, want at least one per facet", got)
	}
	if got := strings.Count(out, `data-dossierx-surface-host`); got < 2 {
		t.Fatalf("surface hosts = %d, want at least one per facet", got)
	}
	if !strings.Contains(out, `function mountSurface`) {
		t.Fatal("viewer-runtime.js must ship mountSurface for deferred mounting")
	}
	// Claim prose must appear inside the inert template payloads, not only as
	// chrome. The host divs themselves stay empty in the static HTML.
	if !strings.Contains(out, "alpha body") || !strings.Contains(out, "beta body") {
		t.Fatal("rendered claim bodies missing from viewer HTML")
	}
}
