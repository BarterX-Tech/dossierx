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
	"github.com/BarterX-Tech/dossierx/internal/config"
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

// buildTargetStatusLookup returns, for every claim id in the catalog, the
// Status/ReviewPending pair components.writeClaimRef needs to decide
// whether a claim-edge target (governed_by/mirrors/rests_on/depended-on-by)
// gets a status pill (C6, the last unshipped piece of GitHub issue #11): a
// pill renders only when the target is actionable — draft, or locked with
// review_pending — never for a healthy locked target, so the footer stays
// quiet except on the hub-gating case the pill exists to explain (see
// components.targetPillHTML).
//
// This is the whole reason the pill has to ride attachEdgesOverride rather
// than living in components.writeClaimRef unconditionally: only a render
// pass with the full catalog in hand can answer "what is claim X's status"
// for an arbitrary target id. The default, parse-time "edges" funcMap
// binding (components.edgesHTML) never sees a catalog at all, so it always
// passes a nil lookup and every target renders with no pill — degrading
// exactly the way implinkLookup and dependedByLookup already do.
func buildTargetStatusLookup(cat *catalog.Catalog) map[string]components.TargetStatus {
	if cat == nil {
		return nil
	}
	out := make(map[string]components.TargetStatus, len(cat.Claims))
	for _, c := range cat.Claims {
		out[c.ID] = components.TargetStatus{Status: c.Status, ReviewPending: c.ReviewPending}
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
//
// The 💬 comment chip + baked thread panel deliberately do NOT ride this
// override: components.EdgesHTMLWithLinks reads c.Comments directly, and the
// claim is already in scope under both this closure and the default
// components.edgesHTML binding (which also calls EdgesHTMLWithLinks), so the
// chip renders under both with no new argument here, no early-return widening,
// and — critically — no second tmpl.Funcs("edges", …) call, which would
// silently discard whichever "edges" binding was attached first. A commented
// project with no implink/depended-by data still hits the early return above
// and keeps the default binding, and still gets its chip, precisely because the
// chip lives inside EdgesHTMLWithLinks rather than in this closure.
func attachEdgesOverride(partials map[model.Layout]*template.Template, implinkLookup map[string][]implink.ViewFile, dependedByLookup map[string][]string, targetStatusLookup map[string]components.TargetStatus) {
	if len(implinkLookup) == 0 && len(dependedByLookup) == 0 && len(targetStatusLookup) == 0 {
		return
	}
	edges := func(c model.Claim) template.HTML {
		return components.EdgesHTMLWithLinks(c, implinkLookup[c.ID], dependedByLookup[c.ID], targetStatusLookup)
	}
	for _, tmpl := range partials {
		tmpl.Funcs(template.FuncMap{"edges": edges})
	}
}

// attachMockupOverride rebinds every partial's "mockupHTML" template func to a
// closure over the project's mockup_modules allowlist (cfg.MockupModules), so
// mockup.html's defense-in-depth gate (components.MockupHTML — see DX-AUD-08)
// can actually check module membership at Execute time. Unlike
// attachEdgesOverride this always rebinds: the default components.mockupHTML
// binding has no config and therefore treats NO module as allowlisted (it
// always escapes), so a mockup claim would never render live without this
// override supplying the real allowlist. A nil cfg leaves the allowlist empty,
// which keeps that always-escape behavior — the safe default.
func attachMockupOverride(partials map[model.Layout]*template.Template, cfg *config.Config) {
	var allowlist []string
	if cfg != nil {
		allowlist = cfg.MockupModules
	}
	mockupHTML := func(c model.Claim) template.HTML {
		return components.MockupHTML(c, allowlist)
	}
	for _, tmpl := range partials {
		tmpl.Funcs(template.FuncMap{"mockupHTML": mockupHTML})
	}
}
