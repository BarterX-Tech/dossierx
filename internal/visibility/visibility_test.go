package visibility

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

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
