// implink_view.go wires internal/implink's linked-file data into the
// shared claim-card edges footer (components.EdgesHTMLWithLinks) without
// touching a single component partial: buildImplinkLookup gathers every
// module's implementation-link artifact (if any) into one project-wide
// claim-id -> linked-files map. See depended_by_view.go's
// attachEdgesOverride for how this lookup is actually bound into each
// partial's "edges" template func — it's folded together with that
// file's dependedBy lookup into one combined closure, since a template
// func name can only ever have one implementation bound to it at a time.
package render

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
)

// buildImplinkLookup merges every module in cfg.Modules' implementation-
// link artifact (internal/implink.ViewsByClaim) into a single, flat,
// project-wide claim-id -> linked-files map. A flat map keyed by claim id
// alone (not also by module) is safe because claim ids are already unique
// project-wide (the id-shape lint enforces this as a hard error before a
// claim can ever lock) — and it is the shape attachImplinkOverride's
// template-func closure actually needs, since a partial template only ever
// has a bare model.Claim (with no separate "which module is this render
// pass currently in" context) to key a lookup off of at Execute time.
//
// A module with no implementation-link artifact at all — the overwhelming
// common case for any project that hasn't adopted this feature — is
// skipped silently: implink.ViewsByClaim's ErrNoArtifact (and, defensively,
// any other error) is treated identically, the same "never fail Render
// over a missing or malformed optional side file" reasoning
// attachBuildOrders already applies to a module with no build-order
// artifact. Returns nil (not an empty, non-nil map) when no module has
// ever linked anything, so attachImplinkOverride's len(lookup) == 0 check
// can tell "nothing to do at all" apart from "checked every module, all
// empty" without caring about the distinction itself.
func buildImplinkLookup(cfg *config.Config) map[string][]implink.ViewFile {
	if cfg == nil {
		return nil
	}
	var out map[string][]implink.ViewFile
	for _, module := range cfg.Modules {
		views, err := implink.ViewsByClaim(cfg, module)
		if err != nil {
			continue
		}
		if out == nil {
			out = make(map[string][]implink.ViewFile, len(views))
		}
		for id, files := range views {
			out[id] = files
		}
	}
	return out
}
