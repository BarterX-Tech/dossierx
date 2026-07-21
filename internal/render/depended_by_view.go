// depended_by_view.go wires a render-time-only reverse index of every
// claim's rests_on into the shared claim-card edges footer
// (components.EdgesHTMLWithLinks), the same "depended on by" relationship
// implink_view.go's doc comment already points readers at for the
// analogous implemented-in case: buildDependedByLookup scans the whole
// catalog once per Render call, and attachEdgesOverride rebinds each
// partial's "edges" func to a closure over both this lookup and
// implink_view.go's, so both extensions apply together through the one
// "edges" name a template can only bind once.
//
// This is deliberately never stored on a claim (no "depended_on_by" YAML
// field exists anywhere in internal/model): rests_on is the single
// authored source of truth for the relationship, and a second, hand-
// maintained copy of its inverse would only be a duplicate that drifts
// the moment one claim's rests_on changes without every claim it used to
// point at being updated to match. Recomputing it fresh every render is
// the only way to show it without ever risking that drift.
package render

import (
	"html/template"
	"sort"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// buildDependedByLookup returns, for every claim id that appears in at
// least one other claim's rests_on, the sorted list of ids of the claims
// that rest on it. A claim nothing rests on (like this project's own
// hard-boundary-ownership-table before this feature existed — the exact
// gap that motivated it) simply has no entry, so attachEdgesOverride's
// map lookup on a claim with no dependents returns nil and renders no
// line, identical to today's output for that claim.
func buildDependedByLookup(cat *catalog.Catalog) map[string][]string {
	if cat == nil {
		return nil
	}
	out := map[string][]string{}
	for _, c := range cat.Claims {
		for _, dep := range c.RestsOn {
			out[dep] = append(out[dep], c.ID)
		}
	}
	for id := range out {
		sort.Strings(out[id])
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// attachEdgesOverride rebinds every partial in partials' "edges" template
// func to a single closure combining implinkLookup (internal/implink-
// sourced "implemented in" lines) and dependedByLookup (this file's
// "depended on by" lines) on top of the shared governed_by/mirrors/
// rests_on/etc footer components.edgesHTML already renders. Both
// extensions have to be folded into one Funcs call rather than two
// separate ones — a *template.Template only ever has one function bound
// to a given name at Execute time, so a second, unrelated call to
// tmpl.Funcs(template.FuncMap{"edges": ...}) would silently discard
// whichever override was attached first instead of composing with it.
//
// When both lookups are empty (no module has ever linked a file and no
// claim in the project is ever rested on by another), this function does
// not call Funcs at all — every partial keeps its original, Load-time
// "edges" binding (components.edgesHTML) completely untouched, so a
// project that has adopted neither feature gets output that is not
// merely equivalent but byte-identical to what Render produced before
// either feature existed.
func attachEdgesOverride(partials map[model.Layout]*template.Template, implinkLookup map[string][]implink.ViewFile, dependedByLookup map[string][]string) {
	if len(implinkLookup) == 0 && len(dependedByLookup) == 0 {
		return
	}
	edges := func(c model.Claim) template.HTML {
		return components.EdgesHTMLWithLinks(c, implinkLookup[c.ID], dependedByLookup[c.ID])
	}
	for _, tmpl := range partials {
		tmpl.Funcs(template.FuncMap{"edges": edges})
	}
}
