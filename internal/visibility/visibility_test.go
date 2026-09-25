package visibility

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestViewerTabsArePeers(t *testing.T) {
	got := ViewerTabs()
	want := []string{ViewerTabManifest, config.FacetContract, config.FacetInternals}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("ViewerTabs() = %v, want %v", got, want)
	}
	if ViewerTabManifest == config.FacetContract || ViewerTabManifest == config.FacetInternals {
		t.Fatal("manifest is a viewer tab, not a claim facet")
	}
}

func TestForeignInternals(t *testing.T) {
	own := model.Claim{ID: "widget.internals.queue", Module: "widget", Facet: config.FacetInternals}
	foreign := model.Claim{ID: "gadget.internals.queue", Module: "gadget", Facet: config.FacetInternals}
	contract := model.Claim{ID: "gadget.contract.api", Module: "gadget", Facet: config.FacetContract}

	if IsForeignInternals("widget", own) {
		t.Error("own internals must not be foreign")
	}
	if !IsForeignInternals("widget", foreign) {
		t.Error("other module internals must be foreign")
	}
	if IsForeignInternals("widget", contract) {
		t.Error("foreign contract is the legal cite surface")
	}
}

func TestIsolationMayIncludeOwnInternals(t *testing.T) {
	claims := []model.Claim{
		{ID: "widget.contract.api", Module: "widget", Facet: config.FacetContract},
		{ID: "widget.internals.queue", Module: "widget", Facet: config.FacetInternals},
		{ID: "gadget.internals.queue", Module: "gadget", Facet: config.FacetInternals},
		{ID: "gadget.contract.api", Module: "gadget", Facet: config.FacetContract},
	}
	got := IsolationClaims(claims, "widget")
	if len(got) != 2 {
		t.Fatalf("isolation widget = %d claims, want 2 (contract + own internals)", len(got))
	}
	for _, c := range got {
		if c.Module != "widget" {
			t.Fatalf("isolation leaked %s", c.ID)
		}
	}
}

func TestIntegrationNeverIncludesInternals(t *testing.T) {
	claims := []model.Claim{
		{ID: "widget.contract.api", Module: "widget", Facet: config.FacetContract},
		{ID: "widget.internals.queue", Module: "widget", Facet: config.FacetInternals},
		{ID: "gadget.internals.queue", Module: "gadget", Facet: config.FacetInternals},
		{ID: "gadget.contract.api", Module: "gadget", Facet: config.FacetContract},
	}
	got := IntegrationClaims(claims)
	if len(got) != 2 {
		t.Fatalf("integration = %d claims, want 2 (contracts only)", len(got))
	}
	for _, c := range got {
		if IsInternals(c) {
			t.Fatalf("integration included internals %s", c.ID)
		}
	}
}

func TestDropInternalsTargets(t *testing.T) {
	internals := map[string]bool{"gadget.internals.queue": true, "widget.internals.q": true}
	got := DropInternalsTargets([]string{"gadget.contract.api", "gadget.internals.queue"}, internals)
	if len(got) != 1 || got[0] != "gadget.contract.api" {
		t.Fatalf("DropInternalsTargets = %v", got)
	}
}
