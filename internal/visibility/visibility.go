// Package visibility is the engine-fixed facet and citation surface.
//
// NIT-20 hard-locks claim facets to contract | internals. Other modules may
// read and cite only contract; internals stay inside the owning module (the
// rests-on-target lint owns that refusal). Integration never includes another
// module's internals — and a project-wide integration projection therefore
// omits every internals claim.
package visibility

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ViewerTabManifest is the first peer tab in the module strip. It is not a
// claim facet: module.manifest.* is illegal. NIT-7 owns the artifact that
// will fill it; this package only names the tab so Manifest cannot be a
// banner above Contract and Internals.
const ViewerTabManifest = "manifest"

// ViewerTabs is Manifest | Contract | Internals, in that order.
func ViewerTabs() []string {
	return []string{ViewerTabManifest, config.FacetContract, config.FacetInternals}
}

// IsInternals reports whether c is an internals claim.
func IsInternals(c model.Claim) bool {
	return c.Facet == config.FacetInternals
}

// IntegrationIncludes reports whether claim belongs in a project-wide
// integration surface. Internals never do — from any other module they are
// foreign, and a global view has no "own" module.
func IntegrationIncludes(claim model.Claim) bool {
	return !IsInternals(claim)
}

// IntegrationClaims returns the project-wide integration surface. Order
// follows claims. Work is one pass over claims: O(V).
func IntegrationClaims(claims []model.Claim) []model.Claim {
	out := make([]model.Claim, 0, len(claims))
	for _, c := range claims {
		if IntegrationIncludes(c) {
			out = append(out, c)
		}
	}
	return out
}

// InternalsIDs is the set of internals claim ids. O(V).
func InternalsIDs(claims []model.Claim) map[string]bool {
	ids := make(map[string]bool, len(claims))
	for _, c := range claims {
		if IsInternals(c) {
			ids[c.ID] = true
		}
	}
	return ids
}

// DropInternalsTargets removes ids that name internals claims. Used when
// projecting integration edges so a catalog consumer cannot follow a cite
// into another module's internals (or any internals, in a global view).
// Work is O(len(ids)).
func DropInternalsTargets(ids []string, internals map[string]bool) []string {
	if len(ids) == 0 || len(internals) == 0 {
		if len(ids) == 0 {
			return nil
		}
		return append([]string(nil), ids...)
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !internals[id] {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
