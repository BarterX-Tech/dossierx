package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRender_SmallCorpusKeepsEagerClaimDOM(t *testing.T) {
	claims := []model.Claim{
		{
			ID: "widget.contract.alpha", Module: "widget", Facet: "contract",
			Status: model.StatusDraft, Layout: model.LayoutCard, Body: "alpha body",
		},
		{
			ID: "widget.internals.beta", Module: "widget", Facet: "internals",
			Status: model.StatusDraft, Layout: model.LayoutCard, Body: "beta body",
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
	// Match the shell markup, not the viewer-runtime.js selector/comment strings
	// that mention the same tokens even when SoftMount is off.
	if strings.Contains(out, `<template class="dossierx-surface-template">`) {
		t.Fatal("small corpora must keep eager claim DOM; surface templates must stay off")
	}
	if strings.Contains(out, `data-dossierx-surface-host></div>`) {
		t.Fatal("small corpora must not emit surface hosts")
	}
	if strings.Contains(out, ` data-dossierx-surface=`) {
		t.Fatal("small corpora must not mark claim-groups as soft-mount surfaces")
	}
	if !strings.Contains(out, "alpha body") || !strings.Contains(out, "beta body") {
		t.Fatal("rendered claim bodies missing from viewer HTML")
	}
	if !strings.Contains(out, `function mountSurface`) {
		t.Fatal("viewer-runtime.js must still ship mountSurface for large corpora")
	}
}

func TestRender_LargeCorpusDefersClaimBodiesIntoSurfaceTemplates(t *testing.T) {
	claims := make([]model.Claim, 0, softMountClaimThreshold)
	for i := 0; i < softMountClaimThreshold; i++ {
		claims = append(claims, model.Claim{
			ID:     fmt.Sprintf("widget.contract.c%03d", i),
			Module: "widget", Facet: "contract",
			Status: model.StatusDraft, Layout: model.LayoutCard,
			Body: fmt.Sprintf("body-%03d", i),
		})
	}
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got := strings.Count(out, `<template class="dossierx-surface-template">`); got < 1 {
		t.Fatalf("surface templates = %d, want at least one for a soft-mounted facet", got)
	}
	if got := strings.Count(out, `data-dossierx-surface-host></div>`); got < 1 {
		t.Fatalf("surface hosts = %d, want at least one for a soft-mounted facet", got)
	}
	if !strings.Contains(out, `function mountSurface`) {
		t.Fatal("viewer-runtime.js must ship mountSurface for deferred mounting")
	}
	if !strings.Contains(out, "body-000") || !strings.Contains(out, "body-079") {
		t.Fatal("rendered claim bodies missing from viewer HTML")
	}
}
