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
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/lint"
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
		for _, dep := range c.RestsOn.IDs {
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

// buildBriefsExplainingLookup is buildDependedByLookup's twin for briefs
// (NIT-202): for every claim id some brief's rests_on names, the rows of
// those briefs, sorted by brief id. It reverses the briefs' own frontmatter
// every render and stores nothing — a claim never learns about briefs
// (internal/briefs' coupling rules), so the only place this relationship can
// exist on the claim card is here, derived.
//
// Only ids the catalog holds get an entry: a rests_on id that names no claim
// (brief-rests-on-unknown's finding) has no card to draw a row on, and
// keeping it out bounds the map by the claims rather than by what briefs
// happen to say. A guidance brief — one that rests on nothing — lands in no
// entry at all.
func buildBriefsExplainingLookup(cat *catalog.Catalog, set *briefs.Set, anchors map[string]string, review *briefs.Evaluation) map[string][]components.BriefRow {
	if cat == nil || set.Empty() {
		return nil
	}
	claims := make(map[string]bool, len(cat.Claims))
	for _, c := range cat.Claims {
		claims[c.ID] = true
	}
	out := map[string][]components.BriefRow{}
	for _, b := range set.Briefs {
		for _, id := range b.RestsOn {
			if claims[id] {
				out[id] = append(out[id], briefRow(b, anchors, review))
			}
		}
	}
	for id := range out {
		rows := out[id]
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildBriefsCitedLookup returns, for every claim with an internal source
// whose path names a brief file (briefs_dir/<folder>/<slug>.md), one row per
// cited brief. The row carries the source's pin and whether
// source-internal-drift reports that source — the lint's own check, run on
// that one source, so the card says "pin out of date" exactly when `check`
// would; nothing here re-implements the hashing rule.
//
// A source path is matched as written, after slash-normalising and path.Clean:
// briefs' names are [a-z0-9-] by rule, so "Briefs/Flow/X.md" is not a brief
// path and stays in the Sources panel only. A brief path the tree does not
// hold (a deleted or misspelt brief) still gets a row, with no link and no
// lock state, because the claim still cites it; its pin is out of date by
// the lint's verdict (the file cannot be read). Two sources citing one brief
// make one row, out of date if either source is; its pin is the first
// drifted source's when one has drifted, and the first source's otherwise.
// Rows are sorted by brief path.
//
// A project holding no brief gets no row at all, even for a source whose
// path looks like one: FORMAT.md's promise is that a project with no briefs
// sees no brief-derived byte in its viewer, and such a source is still in the
// Sources panel with its drift finding in `check`.
func buildBriefsCitedLookup(cat *catalog.Catalog, cfg *config.Config, set *briefs.Set, anchors map[string]string, review *briefs.Evaluation) map[string][]components.BriefRow {
	if cat == nil || set.Empty() {
		return nil
	}
	byPath := make(map[string]briefs.Brief, len(set.Briefs))
	for _, b := range set.Briefs {
		byPath[b.Path] = b
	}
	drift := sourceDriftCheck(cfg)
	out := map[string][]components.BriefRow{}
	for _, c := range cat.Claims {
		var rows []components.BriefRow
		index := map[string]int{}
		for _, s := range c.Sources {
			if !s.IsInternal() {
				continue
			}
			p := path.Clean(filepath.ToSlash(strings.TrimSpace(s.Path)))
			// A brief file is exactly one folder below briefs_dir.
			folderDir := path.Dir(p)
			if path.Dir(folderDir) != set.DisplayDir || path.Ext(p) != ".md" {
				continue
			}
			outOfDate := drift(c.ID, s)
			if i, seen := index[p]; seen {
				// The row's pin is the one its hover shows: once any source
				// has drifted, that is the drifted source's pin, never a
				// holding one beside "pin out of date".
				if outOfDate && !rows[i].PinOutOfDate {
					rows[i].Pin, rows[i].PinOutOfDate = s.SHA256, true
				}
				continue
			}
			row := components.BriefRow{Path: p, Folder: path.Base(folderDir), Title: s.Title}
			if b, ok := byPath[p]; ok {
				row = briefRow(b, anchors, review)
			} else if row.Title == "" {
				row.Title = p
			}
			row.Pin, row.PinOutOfDate = s.SHA256, outOfDate
			index[p] = len(rows)
			rows = append(rows, row)
		}
		if len(rows) == 0 {
			continue
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
		out[c.ID] = rows
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// sourceDriftCheck returns a func reporting whether source-internal-drift
// finds anything wrong with one internal source of claim id: the registered
// lint, run over a claim holding that one source. With no such lint
// registered every pin reads out of date, the direction that never shows an
// unchecked pin as holding.
//
// The verdict depends only on the source's path, record_id and recorded
// sha256 (the claim id only names the finding), so it is memoised on those
// three within one render: a brief cited by many claims is read and hashed
// once per distinct pin, not once per citing source.
func sourceDriftCheck(cfg *config.Config) func(string, model.Source) bool {
	type pinKey struct{ path, record, sum string }
	for _, l := range lint.Registry {
		if l.Name() == "source-internal-drift" {
			seen := map[pinKey]bool{}
			return func(id string, s model.Source) bool {
				k := pinKey{s.Path, s.RecordID, s.SHA256}
				if v, ok := seen[k]; ok {
					return v
				}
				v := len(l.Check([]model.Claim{{ID: id, Sources: []model.Source{s}}}, cfg)) > 0
				seen[k] = v
				return v
			}
		}
	}
	return func(string, model.Source) bool { return true }
}

// briefRow is the row fields a brief itself supplies, and its page id from
// anchors (briefAnchors' map, the one the brief pages are given).
func briefRow(b briefs.Brief, anchors map[string]string, review *briefs.Evaluation) components.BriefRow {
	return components.BriefRow{
		ID:            b.ID,
		Anchor:        anchors[b.ID],
		Path:          b.Path,
		Folder:        b.Folder,
		Slug:          b.Slug,
		Title:         b.Title,
		Locked:        b.Status == briefs.StatusLocked,
		ReviewPending: briefReviewOf(review, b).ReviewPending,
	}
}

// buildBriefRelationsLookup joins the two brief lookups into the one value
// per claim the edges footer takes. Each row's link is the brief page's own
// id from briefAnchors, computed from the same set, catalog and config the
// pages are rendered from — so a brief whose plain id another brief or a
// module already spells links to its own -2 section, not the other one's.
func buildBriefRelationsLookup(cat *catalog.Catalog, cfg *config.Config, set *briefs.Set, review *briefs.Evaluation) map[string]components.BriefRelations {
	if set.Empty() {
		return nil
	}
	anchors := briefAnchors(set, cat, cfg)
	explained := buildBriefsExplainingLookup(cat, set, anchors, review)
	cited := buildBriefsCitedLookup(cat, cfg, set, anchors, review)
	if len(explained) == 0 && len(cited) == 0 {
		return nil
	}
	out := make(map[string]components.BriefRelations, len(explained)+len(cited))
	for id, rows := range explained {
		r := out[id]
		r.ExplainedBy = rows
		out[id] = r
	}
	for id, rows := range cited {
		r := out[id]
		r.CitedAsEvidence = rows
		out[id] = r
	}
	return out
}

// buildTargetStatusLookup returns, for every claim id in the catalog, the
// Status/ReviewPending pair components.writeClaimRef needs to decide
// whether a claim-edge target (rests_on/depended-on-by)
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
// "depended on by" lines) on top of the shared rests_on/etc footer
// components.edgesHTML already renders. Both
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
// The 💬 comment chip no longer rides this shared footer at all. As of v0.4.1
// it is emitted by components.CommentChipHTML, bound once as the "commentChip"
// template func and called straight from each chip-bearing partial's claim
// head, because the footer is now a collapsed <details> and a chip inside it
// would be invisible and unclickable on every claim whose footer starts closed.
// That func takes only the claim — no config, no allowlist, no catalog — so it
// needs no override of its own here: the exported func IS the binding, under
// both the parse-time funcMap and every Render pass. Nothing about the chip
// touches this closure, this file's early return, or this override's arguments.
//
// The baked thread panel DOES still come out of components.EdgesHTMLWithLinks,
// and still deliberately does not ride this override: that func reads
// c.Comments directly, and the claim is already in scope under both this
// closure and the default components.edgesHTML binding (which also calls
// EdgesHTMLWithLinks), so the panel renders under both with no new argument
// here, no early-return widening, and — critically — no second
// tmpl.Funcs("edges", …) call, which would silently discard whichever "edges"
// binding was attached first. A commented project with no implink/depended-by
// data still hits the early return above and keeps the default binding, and
// still gets its panel, precisely because the panel lives inside
// EdgesHTMLWithLinks rather than in this closure.
//
// linksGated is check's code-link precondition (`source_dirs` set — see
// implink_view.go's codeLinksGated). It widens the early return on its own:
// a gated project with no artifact at all is the loudest case of the row
// this override exists to draw — every locked, code-producing claim is
// unlinked — and the default binding cannot know that.
//
// briefLookup (NIT-202) is the derived BRIEFS group per claim; a project with
// no brief row anywhere passes nil and gets the output it had before.
func attachEdgesOverride(partials map[model.Layout]*template.Template, implinkLookup map[string][]implink.ViewFile, linksGated bool, dependedByLookup map[string][]string, targetStatusLookup map[string]components.TargetStatus, briefLookup map[string]components.BriefRelations) {
	if len(implinkLookup) == 0 && !linksGated && len(dependedByLookup) == 0 && len(targetStatusLookup) == 0 && len(briefLookup) == 0 {
		return
	}
	edges := func(c model.Claim) template.HTML {
		return components.EdgesHTMLWithCodeLinks(c, implinkLookup[c.ID], linksGated, dependedByLookup[c.ID], targetStatusLookup, briefLookup[c.ID])
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
