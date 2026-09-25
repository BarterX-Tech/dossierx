// Package visibility is the engine-fixed facet and citation surface.
//
// NIT-20 hard-locks claim facets to contract | internals. Other modules may
// read and cite only contract. Internals stay inside the owning module:
// check and lock refuse foreign-internals edges. Isolation of a module may
// include that module's own internals. Integration never includes another
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

// IsForeignInternals reports whether target is internals owned by a module
// other than fromModule. A missing or empty module on either side is still
// foreign when the target is internals: an unscoped cite is not ownership.
func IsForeignInternals(fromModule string, target model.Claim) bool {
	return IsInternals(target) && target.Module != fromModule
}

// IsolationIncludes reports whether claim belongs in module's isolation
// surface: every claim that module owns, including its internals.
func IsolationIncludes(module string, claim model.Claim) bool {
	return module != "" && claim.Module == module
}

// IntegrationIncludes reports whether claim belongs in a project-wide
// integration surface. Internals never do — from any other module they are
// foreign, and a global view has no "own" module.
func IntegrationIncludes(claim model.Claim) bool {
	return !IsInternals(claim)
}

// IsolationClaims returns the isolation surface of module. Order follows
// claims. Work is one pass over claims: O(V), not a path walk.
func IsolationClaims(claims []model.Claim, module string) []model.Claim {
	if module == "" {
		return nil
	}
	out := make([]model.Claim, 0, len(claims))
	for _, c := range claims {
		if IsolationIncludes(module, c) {
			out = append(out, c)
		}
	}
	return out
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
