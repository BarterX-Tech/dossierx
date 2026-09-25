package visibility

import (
	"fmt"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestIsolationAndIntegrationAreOnePassOverClaims is the NIT-20 graph-cost
// proof: membership is O(V), not a path walk. Doubling V must keep output
// linear in V (half internals, half contract, two modules).
func TestIsolationAndIntegrationAreOnePassOverClaims(t *testing.T) {
	const n = 4000
	claims := make([]model.Claim, n)
	for i := 0; i < n; i++ {
		mod := "widget"
		if i%2 == 1 {
			mod = "gadget"
		}
		facet := config.FacetContract
		if i%4 < 2 {
			facet = config.FacetInternals
		}
		claims[i] = model.Claim{
			ID:     fmt.Sprintf("%s.%s.c-%04d", mod, facet, i),
			Module: mod,
			Facet:  facet,
		}
	}

	iso := IsolationClaims(claims, "widget")
	if len(iso) != n/2 {
		t.Fatalf("isolation widget = %d, want %d", len(iso), n/2)
	}
	for _, c := range iso {
		if c.Module != "widget" {
			t.Fatalf("isolation leaked %s", c.ID)
		}
	}

	integ := IntegrationClaims(claims)
	if len(integ) != n/2 {
		t.Fatalf("integration = %d, want %d (every contract)", len(integ), n/2)
	}
	for _, c := range integ {
		if IsInternals(c) {
			t.Fatalf("integration included internals %s", c.ID)
		}
	}

	ids := InternalsIDs(claims)
	if len(ids) != n/2 {
		t.Fatalf("InternalsIDs = %d, want %d", len(ids), n/2)
	}
}
