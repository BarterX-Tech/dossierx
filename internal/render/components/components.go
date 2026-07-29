// Package components holds one html/template partial per model.Layout
// value, embedded into the binary via go:embed. This is the engine's
// default set; a project may override individual partials by name via
// project.config.yaml's viewer.template_overrides (missing overrides fall
// back to these defaults, per-component; a configured-but-nonexistent
// override directory is a hard load-time error).
package components

import (
	"embed"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

//go:embed card.html table.html list.html steps.html tree.html banner.html mockup.html build_order.html comments.html
var FS embed.FS

// buildOrderFileName is the override-lookup name for the optional Build
// Order viewer partial (see LoadBuildOrder) — not part of fileForLayout
// since it isn't selected by a claim's model.Layout the way the 7 partials
// above are; it renders a whole module's build-order artifact instead of
// one claim.
const buildOrderFileName = "build_order.html"

// fileForLayout maps a model.Layout to its default partial's filename.
// This is the "plain map lookup on claim.Layout" the render package uses —
// no per-project branching lives in engine code.
var fileForLayout = map[model.Layout]string{
	model.LayoutCard:   "card.html",
	model.LayoutTable:  "table.html",
	model.LayoutList:   "list.html",
	model.LayoutSteps:  "steps.html",
	model.LayoutTree:   "tree.html",
	model.LayoutBanner: "banner.html",
	model.LayoutMockup: "mockup.html",
}

// funcMap is shared by every component template — default and override
// alike — so an override partial can use the same generic helpers as the
// built-in ones.
var funcMap = template.FuncMap{
	"rowKeys":       rowKeys,
	"markdown":      markdown.Render,
	"cell":          cell,
	"edges":         edgesHTML,
	"inc":           inc,
	"pillClass":     pillClass,
	"colClass":      colClass,
	"mockupHTML":    mockupHTML,
	"claimLabel":    ClaimLabel,
	"claimEdgeList": ClaimEdgeListHTML,
}

// commentsPanelTmpl is the parsed comments.html thread-panel partial, parsed
// once from the embedded default at package init and never reassigned, so it
// is safe to Execute concurrently — html/template permits parallel Execute so
// long as writers are not shared, and every caller here uses its own builder.
// EdgesHTMLWithLinks executes it to bake a claim's review threads into the
// static render immediately after the shared edges <ul>. Parsed with funcMap
// so comment bodies route through the same "markdown" renderer every other
// body-shaped field uses. A parse failure here (a malformed embedded default)
// is a programming error the package can't run without, so it panics at init
// rather than deferring the failure to the first render.
//
// It is parsed with only the "markdown" func — not the full funcMap — on
// purpose: funcMap binds "edges" to edgesHTML, whose body reaches
// EdgesHTMLWithLinks, which reads this very var, so parsing with funcMap would
// be a package-initialization cycle. comments.html needs nothing but "markdown"
// anyway (its other constructs — range/if/eq/len/template — are builtins).
//
// Unlike the per-layout partials and build_order.html, this panel is not wired
// to viewer.template_overrides: it reaches the render through EdgesHTMLWithLinks
// (the shared "edges" func, whose signature is fixed — a project-scoped override
// template can't be threaded to it without widening attachEdgesOverride, which
// a second consumer of the "edges" name is explicitly not allowed to do), and
// its markup is a tight contract with the viewer JS that a project override
// would silently break. Projects restyle the panel via viewer.theme / a
// style.css override instead of replacing this structure.
var commentsPanelTmpl = template.Must(
	template.New("comments.html").Funcs(template.FuncMap{"markdown": markdown.Render}).ParseFS(FS, "comments.html"),
)

// commentsPanelView is the shape comments.html executes against: the claim id
// (carried on the panel as data-claim-id, never id=, so the viewer JS can fan
// state out across an overview claim's N rendered copies) plus its threads
// split into the open ones shown inline and the resolved ones tucked into the
// <details> collapse.
type commentsPanelView struct {
	ClaimID  string
	Open     []model.Comment
	Resolved []model.Comment
}

// newCommentsPanelView partitions a claim's comments into open and resolved for
// comments.html. A thread counts as resolved only when its status is exactly
// CommentStatusResolved; anything else is shown inline, so an unexpected status
// is surfaced to the reader rather than silently swallowed into the collapse.
func newCommentsPanelView(c model.Claim) commentsPanelView {
	v := commentsPanelView{ClaimID: c.ID}
	for _, cm := range c.Comments {
		if cm.Status == model.CommentStatusResolved {
			v.Resolved = append(v.Resolved, cm)
		} else {
			v.Open = append(v.Open, cm)
		}
	}
	return v
}

// Load parses the default embedded partial for every known layout and
// returns them keyed by layout. overrideDir, if non-empty, must exist as a
// directory — a configured-and-missing override directory is a hard,
// load-time error. Within an override directory that does exist, a missing
// individual partial file is not an error: that single component falls
// back to its embedded default (soft fallback, per-component).
func Load(overrideDir string) (map[model.Layout]*template.Template, error) {
	if overrideDir != "" {
		info, err := os.Stat(overrideDir)
		if err != nil {
			return nil, fmt.Errorf("components: viewer.template_overrides %q: %w", overrideDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("components: viewer.template_overrides %q is not a directory", overrideDir)
		}
	}

	out := make(map[model.Layout]*template.Template, len(fileForLayout))
	for layout, file := range fileForLayout {
		tmpl, err := loadOne(overrideDir, layout, file)
		if err != nil {
			return nil, err
		}
		out[layout] = tmpl
	}
	return out, nil
}

func loadOne(overrideDir string, layout model.Layout, file string) (*template.Template, error) {
	data, found, err := OverrideFile(overrideDir, file)
	if err != nil {
		return nil, err
	}
	if found {
		tmpl, err := template.New(file).Funcs(funcMap).Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("components: parse override template %q for layout %q: %w", filepath.Join(overrideDir, file), layout, err)
		}
		return tmpl, nil
	}
	// Missing override file for this component only: fall back to the
	// embedded default below.

	tmpl, err := template.New(file).Funcs(funcMap).ParseFS(FS, file)
	if err != nil {
		return nil, fmt.Errorf("components: parse default template for layout %q: %w", layout, err)
	}
	return tmpl, nil
}

// LoadBuildOrder parses the Build Order viewer partial (build_order.html),
// applying the same override-then-embedded-default fallback as Load's
// per-layout partials: a project may override build_order.html by name
// inside its viewer.template_overrides directory, and falls back to the
// engine's embedded default when it does not. Unlike Load, this returns a
// single *template.Template rather than a map, since build_order.html is
// not selected per-claim by model.Layout — internal/render calls this once
// and reuses the result for every module that has a locked build-order
// artifact to render.
func LoadBuildOrder(overrideDir string) (*template.Template, error) {
	data, found, err := OverrideFile(overrideDir, buildOrderFileName)
	if err != nil {
		return nil, err
	}
	if found {
		tmpl, err := template.New(buildOrderFileName).Funcs(funcMap).Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("components: parse override template %q: %w", filepath.Join(overrideDir, buildOrderFileName), err)
		}
		return tmpl, nil
	}

	tmpl, err := template.New(buildOrderFileName).Funcs(funcMap).ParseFS(FS, buildOrderFileName)
	if err != nil {
		return nil, fmt.Errorf("components: parse default build_order template: %w", err)
	}
	return tmpl, nil
}

// OverrideFile checks whether overrideDir (if non-empty) contains a file
// named name and, if so, reads and returns its contents. It implements the
// same soft-fallback semantics as loadOne: a missing file (or an empty
// overrideDir) returns (nil, false, nil) rather than an error — the caller
// is expected to fall back to its own embedded default in that case. Any
// other stat/read failure (permissions, etc.) is a hard error.
//
// It is exported so internal/render can apply this exact fallback logic to
// shell.html and style.css, which live outside this package's per-layout
// component set but override from the same project-configured directory
// (cfg.Viewer.TemplateOverrides) as card.html/table.html/etc.
func OverrideFile(overrideDir, name string) (data []byte, found bool, err error) {
	if overrideDir == "" {
		return nil, false, nil
	}
	path := filepath.Join(overrideDir, name)
	data, err = os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("components: read override file %q: %w", path, err)
	}
	return data, true, nil
}

// rowKeys returns the union of column names across rows, in authored
// order: each row contributes its columns in the order captured by
// model.RowColumns (the order a human wrote them in that row's YAML), and
// a column already seen from an earlier row is not repeated. A row with no
// captured order (model.RowColumns returns nil — e.g. a model.Row built
// directly in Go rather than decoded from YAML, as in this package's own
// tests) falls back to that row's own keys sorted alphabetically, matching
// this function's behavior before authored-order tracking existed. It is
// deliberately generic about which columns exist since model.Row is a
// free-form string-keyed map; internal/lint's rows-shape lint is what
// enforces that a single claim's rows share consistent columns, not this
// helper.
func rowKeys(rows []model.Row) []string {
	seen := make(map[string]bool)
	var keys []string
	appendUnseen := func(ordered []string) {
		for _, k := range ordered {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	for _, row := range rows {
		if order := model.RowColumns(row); order != nil {
			appendUnseen(order)
			continue
		}
		fallback := make([]string, 0, len(row))
		for k := range row {
			fallback = append(fallback, k)
		}
		sort.Strings(fallback)
		appendUnseen(fallback)
	}
	return keys
}

// pillClass maps a claim's Status/ReviewPending pair to the docs/ source's
// status-pill CSS class (.pill.ps / .pill.pv / .pill.pw — see
// the reference docs stylesheet), so every component's claim-head pill uses the same
// three-way convention: "pw" (warn/red) takes priority when a locked claim
// has gone review_pending, "ps" (accent/green) for an ordinary locked claim,
// and "pv" (amber) for a draft claim — ReviewPending is only ever set on a
// locked claim (see model.Claim.ReviewPending), so status==draft never hits
// the pw case.
func pillClass(status model.Status, reviewPending bool) string {
	if status == model.StatusLocked && reviewPending {
		return "pw"
	}
	if status == model.StatusLocked {
		return "ps"
	}
	return "pv"
}

// edgesHTML renders the edge/metadata footer shared by every non-banner
// component: governed_by, mirrors, rests_on, migrated_from, and a
// review_pending flag. It is a Go helper rather than template markup so
// every component gets identical, balanced markup without duplicating it
// six times; values are HTML-escaped by hand since a FuncMap-returned
// template.HTML value bypasses html/template's automatic escaping.
//
// facet/module are deliberately not shown here: every claim on a rendered
// page already sits inside that facet's tab and that module's nav entry
// (see shell.html's tab/nav structure), so repeating "facet: contract
// module: widget" on every single card is redundant with page context the
// reader can already see, not new information.
//
// This is the default "edges" template func binding every partial parses
// against (see funcMap below); it is a thin wrapper over
// EdgesHTMLWithLinks(c, nil) so a project that has never used
// internal/implink renders byte-identically to before that package
// existed. internal/render overrides the "edges" binding to a closure over
// EdgesHTMLWithLinks (never over edgesHTML directly) only for a project
// where at least one module has linked at least one file — see that
// package's attachImplinkOverride.
func edgesHTML(c model.Claim) template.HTML {
	return EdgesHTMLWithLinks(c, nil, nil, nil)
}

// TargetStatus is what a claim-edge target pill (see writeClaimRef) needs to
// know about the claim it points at: just enough of that claim's own
// Status/ReviewPending pair to decide whether the target is actionable, not
// the whole model.Claim (the edges footer never had a reason to hold every
// other claim in memory before this feature, and shouldn't start now).
type TargetStatus struct {
	Status        model.Status
	ReviewPending bool
}

// targetPillHTML returns the small inline pill writeClaimRef appends after a
// target's label, or "" when the target carries no pill at all. A pill is
// shown ONLY for an actionable target — draft, or locked with
// review_pending — reusing pillClass so a target pill is drawn from the same
// three-way status→class mapping every claim-head pill already uses. A
// healthy locked target (the common case) gets nothing: the footer stays
// quiet and lights up exactly on the case it exists to explain (a claim
// gated on an edge that isn't ready yet), not on every edge indiscriminately.
//
// statuses is nil whenever the caller has no catalog to look targets up in
// — the default, parse-time "edges"/"claimEdgeList" funcMap bindings can
// never see the catalog (see this file's Load/loadOne), so every target
// silently gets no pill under those bindings. Only internal/render's
// attachEdgesOverride, which does have the whole catalog, ever supplies a
// non-nil map (see that package's buildTargetStatusLookup).
func targetPillHTML(targetID string, statuses map[string]TargetStatus) string {
	st, ok := statuses[targetID]
	if !ok {
		return ""
	}
	actionable := st.Status == model.StatusDraft || (st.Status == model.StatusLocked && st.ReviewPending)
	if !actionable {
		return ""
	}
	label := string(st.Status)
	if st.Status == model.StatusLocked && st.ReviewPending {
		label = "review_pending"
	}
	return ` <span class="pill ` + pillClass(st.Status, st.ReviewPending) + `">` + html.EscapeString(label) + `</span>`
}

// EdgesHTMLWithLinks renders the same shared edges footer as edgesHTML,
// plus one additional "implemented in: file#symbol" line per entry in
// files — the internal/implink-sourced extension to this footer — and one
// "depended on by: ..." line listing dependedBy, the reverse of rests_on.
// A file currently flagged Drifted (internal/implink.ViewFile.Drifted) gets
// the same warn-styled pill every locked+review_pending claim's status pill
// already uses (.pill.pw — see components.go's pillClass), reusing that
// existing token rather than inventing a new color for "this specific
// linked file may be stale". Exported (unlike edgesHTML) so
// internal/render can bind it into a per-render "edges" template-func
// override; see that package's attachEdgesOverride for why a template
// func override, rather than a second template field, is how this data
// reaches the existing per-layout partials without editing any of them.
//
// dependedBy is never authored — it is the reverse index of every other
// claim's rests_on, computed fresh each Render pass from the whole
// catalog (see internal/render's buildDependedByLookup). Deriving it at
// render time instead of storing a "depended_on_by" field on the claim
// itself is deliberate: a stored reverse edge is a second copy of the same
// fact rests_on already states, and second copies drift the moment either
// side is edited without the other — the very duplication this project's
// single-source-of-truth rule (see rests_on's own doc comment) exists to
// rule out.
func EdgesHTMLWithLinks(c model.Claim, files []implink.ViewFile, dependedBy []string, targetStatuses map[string]TargetStatus) template.HTML {
	var b strings.Builder
	b.WriteString(`<ul class="claim-edges">`)

	if c.Governed.Type != "" {
		if c.Governed.Type == string(model.GovernedNone) {
			b.WriteString(`<li class="claim-governed governed-none">governed_by: none`)
			if c.Governed.Reason != "" {
				b.WriteString(` — `)
				// Reason is hand-written prose, not viewer chrome (unlike
				// Claim.Section below), and routinely names a claim id or a
				// path — so it goes through the same INLINE-ceiling renderer
				// every other prose field uses (markdown.RenderInline: code
				// spans and links, no block constructs) rather than a bare
				// html.EscapeString.
				b.WriteString(string(markdown.RenderInline(c.Governed.Reason)))
			}
			b.WriteString(`</li>`)
		} else {
			// governed_by names another claim, so it renders through the same
			// writeClaimRef every other claim-to-claim edge below uses rather
			// than its own hand-built <a>: a doctrine hub is nearly always in a
			// different facet from the claim it governs, so this is precisely
			// the edge whose prefix tier ("Doctrine › Hub") carries information.
			b.WriteString(`<li class="claim-governed">governed_by: `)
			writeClaimRef(&b, c.Governed.Type, c.Module, c.Facet, targetStatuses)
			b.WriteString(`</li>`)
		}
	}

	if len(c.Mirrors) > 0 {
		b.WriteString(`<li class="claim-mirrors">mirrors:`)
		writeIDListItems(&b, c.Module, c.Facet, c.Mirrors, targetStatuses)
		b.WriteString(`</li>`)
	}

	if len(c.RestsOn) > 0 {
		b.WriteString(`<li class="claim-rests-on">rests_on:`)
		writeIDListItems(&b, c.Module, c.Facet, c.RestsOn, targetStatuses)
		b.WriteString(`</li>`)
	}

	if len(dependedBy) > 0 {
		b.WriteString(`<li class="claim-depended-by">depended on by:`)
		writeIDListItems(&b, c.Module, c.Facet, dependedBy, targetStatuses)
		b.WriteString(`</li>`)
	}

	if c.MigratedFrom != "" {
		b.WriteString(`<li class="claim-migrated">migrated_from: `)
		b.WriteString(html.EscapeString(c.MigratedFrom))
		b.WriteString(`</li>`)
	}

	if c.Status == model.StatusLocked && c.ReviewPending {
		b.WriteString(`<li class="claim-review-pending">review_pending</li>`)
	}

	// The 💬 comment chip rides this shared footer, reading c.Comments
	// directly rather than a per-render lookup — the claim is already in
	// scope under both the default "edges" binding (edgesHTML) and
	// render's attachEdgesOverride closure, so the chip renders under both
	// with no signature change and no second "edges" Funcs binding (which
	// would silently discard the first). Values are hand-escaped like the
	// rest of this func because a FuncMap-returned template.HTML bypasses
	// html/template's auto-escaping. banner.html never calls {{edges .}},
	// so banner claims are excluded from the whole comment surface for free.
	//
	// The chip is emitted for EVERY claim reaching this footer, not only ones
	// that already carry threads. Gating it on len(c.Comments) > 0 made the
	// FIRST comment on a card unreachable from the viewer — the only surface
	// the human has — so a claim nobody had questioned yet could never be
	// questioned. The zero state is its own third variant (comment-chip--empty,
	// reading "💬 0") rather than a borrowed --resolved, because "no one has
	// commented" and "everything raised was settled" are different facts and
	// the --resolved count would lie about the second.
	//
	// The zero-state <li> ships with the `hidden` attribute and is revealed by
	// shell.html only once its /api/ping probe confirms a live serve. A static
	// file:// export has no reachable comment API and therefore mounts no
	// composer, so an empty chip there would open a rail with nothing in it and
	// no way to add anything — a dead control. Hidden-by-default (rather than
	// shown-then-hidden-on-probe-failure) is deliberate: the capability appears
	// when it is confirmed available, instead of flashing away ~1s after load
	// and inviting a click that resolves to nothing.
	open := len(c.OpenThreadIDs())
	total := len(c.Comments)
	liOpenTag := `<li class="claim-comments" hidden>`
	chipClass := "comment-chip comment-chip--empty"
	count := 0
	label := "add the first comment on this claim"
	if total > 0 {
		liOpenTag = `<li class="claim-comments">`
		chipClass = "comment-chip comment-chip--resolved"
		count = total
		label = fmt.Sprintf("view %d comment thread(s), all resolved", total)
		if open > 0 {
			chipClass = "comment-chip comment-chip--open"
			count = open
			label = fmt.Sprintf("view %d open comment thread(s)", open)
		}
	}
	// aria-controls names the shared comment rail (#commentsPanel, emitted by
	// shell.html) that the viewer JS reveals on click; aria-expanded is kept in
	// sync by that JS. Both are inert in a browser-less context but make the
	// chip a proper disclosure control for assistive tech.
	b.WriteString(liOpenTag)
	b.WriteString(`<button type="button" class="`)
	b.WriteString(chipClass)
	b.WriteString(`" data-claim-id="`)
	b.WriteString(html.EscapeString(c.ID))
	b.WriteString(`" aria-controls="commentsPanel" aria-expanded="false" aria-label="`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`"><span class="comment-chip-glyph" aria-hidden="true">💬</span> <span class="comment-chip-count">`)
	b.WriteString(fmt.Sprintf("%d", count))
	b.WriteString(`</span></button></li>`)

	for _, f := range files {
		b.WriteString(`<li class="claim-implemented-in">implemented in: <code>`)
		b.WriteString(html.EscapeString(f.File))
		if f.Symbol != "" {
			b.WriteString(`#`)
			b.WriteString(html.EscapeString(f.Symbol))
		}
		b.WriteString(`</code>`)
		if f.Drifted {
			b.WriteString(` <span class="pill pw">drifted</span>`)
		}
		b.WriteString(`</li>`)
	}

	b.WriteString(`</ul>`)

	// The baked-in thread panel follows the edges <ul> (a <div>, so it can't
	// be an <li> inside it) but stays inside the claim's <section>, since
	// {{edges .}} is the last thing every non-banner partial emits before
	// </section>. comments.html auto-escapes its bodies via the shared
	// "markdown" func, so no hand-escaping is needed for the panel. On the
	// (embedded, tested) template this Execute cannot fail; a defensive error
	// path drops the panel rather than corrupt the footer with partial output.
	if len(c.Comments) > 0 {
		var pb strings.Builder
		if err := commentsPanelTmpl.Execute(&pb, newCommentsPanelView(c)); err == nil {
			b.WriteString(pb.String())
		}
	}

	return template.HTML(b.String())
}

// inc returns i+1, used to 1-index steps.html's numbered bubble from the
// 0-based index range/$i gives.
func inc(i int) int {
	return i + 1
}

// colClassByKey maps a table.html column name (a model.Row key), lower-
// cased, to the semantic CSS class ported verbatim from the reference
// stylesheet's field-meta styling: .key for a term/identifier column,
// .ty for a type column, .en for an enum/constraint column, .ex for an
// example or free-text explanation column. Only exact, known column names
// opt in — this is a closed convention list, not a heuristic, so an
// unrecognized column name (e.g. "rule", "cloud_default") renders with no
// extra class rather than a guessed one.
var colClassByKey = map[string]string{
	"key":      "key",
	"field":    "key",
	"type":     "ty",
	"enum":     "en",
	"example":  "ex",
	"examples": "ex",
}

// colClass returns the semantic CSS class for a table.html column name, or
// "" when the column name doesn't match a known convention in
// colClassByKey (the empty string renders no class attribute at all, not
// class="").
func colClass(key string) string {
	return colClassByKey[strings.ToLower(key)]
}

// mockupHTML is mockup.html's render-time gate on emitting .RawHTML
// unescaped, the defense-in-depth companion to internal/lint's raw-html-scope
// lint (DX-AUD-08). This is the DEFAULT (parse-time) binding, which has no
// project config and therefore no mockup_modules allowlist to consult, so it
// treats NO module as allowlisted and always escapes — the auto-escaping
// bypass is never reached from the default binding. internal/render rebinds
// this func name (see that package's attachMockupOverride) to a closure over
// cfg.MockupModules for the real render path, which is the only place a
// genuinely locked + reviewed + allowlisted mockup's markup is emitted live.
// No other component template uses this func; every other body-shaped field
// goes through the "markdown" func, which always auto-escapes.
func mockupHTML(c model.Claim) template.HTML {
	return MockupHTML(c, nil)
}

// MockupHTML returns c.RawHTML as trusted (unescaped) template.HTML only when
// c is locked, RawHTMLReviewed, and c.Module appears in allowlist (the
// project's mockup_modules); in every other case it returns the HTML-escaped
// text, so a draft, unreviewed, or non-allowlisted mockup can never inject
// live markup into the viewer even if it reached render — the raw-html-scope
// lint gate should already have stopped it, this is the second layer. Exported
// so internal/render can bind it with the project's allowlist.
func MockupHTML(c model.Claim, allowlist []string) template.HTML {
	trusted := c.Status == model.StatusLocked && c.RawHTMLReviewed && stringInSlice(allowlist, c.Module)
	if trusted {
		return template.HTML(c.RawHTML)
	}
	return template.HTML(html.EscapeString(c.RawHTML))
}

// stringInSlice reports whether s is present in ss.
func stringInSlice(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// cell renders one table.html cell value through the shared inline markdown
// renderer, so a <td> shows code spans and links (markdown.RenderInline's
// subset) rather than the literal `code`/[text](url) source table.html used
// to emit with a raw {{index $row .}} (DX-AUD-02). It is the inline
// counterpart to the "markdown" func card/list/steps/banner bodies use — no
// <p> wrapper, because a table cell wants inline content, not a block.
//
// A model.Row is a map[string]any, so a cell value can be any YAML scalar
// (string, number, bool) or, for an absent column in a ragged row, nil. nil
// renders as the empty string; every other value is stringified with
// fmt.Sprint before inline rendering, so a non-string cell renders its value
// instead of failing template execution the way passing a non-string to a
// string-typed renderer would.
func cell(v any) template.HTML {
	if v == nil {
		return ""
	}
	return markdown.RenderInline(fmt.Sprint(v))
}

// writeIDListItems renders ids as a nested <ul> of one <li> per id, each
// holding a writeClaimRef anchor, used for every edges-footer field that
// lists other claim ids (mirrors, rests_on, depended-by) so each id gets its
// own bulleted line rather than a run-on comma list. fromModule/fromFacet are
// the RENDERING claim's own module and facet — the context writeClaimRef
// elides each target's redundant prefix against.
func writeIDListItems(b *strings.Builder, fromModule, fromFacet string, ids []string, targetStatuses map[string]TargetStatus) {
	b.WriteString(`<ul class="claim-edge-id-list">`)
	for _, id := range ids {
		b.WriteString(`<li>`)
		writeClaimRef(b, id, fromModule, fromFacet, targetStatuses)
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul>`)
}

// ---------------------------------------------------------------------
// Claim-edge labels (issue #11)
//
// Every claim-to-claim edge used to render as its raw id. Stacked under a
// card, a column of "widget.contract.retry-policy / widget.contract.retry-
// budget / widget.contract.retry-jitter" is near-unreadable: the segments
// that differ are the last few characters of otherwise identical strings,
// and the shared "widget.contract." prefix repeats a module and facet the
// reader is already inside (the surrounding tab and nav entry state both —
// the same reasoning that keeps facet/module off the footer entirely, see
// edgesHTML). The helpers below turn an id into a readable label and elide
// exactly the prefix that is redundant in the rendering claim's context,
// while keeping the machine id itself one hover (or one "view source", or
// one querySelector) away — see writeClaimRef for why that is non-negotiable.
// ---------------------------------------------------------------------

// claimRefModuleSep joins a cross-module target's module and facet; and
// claimRefLabelSep separates the whole elided prefix from the label proper.
// Two different glyphs rather than one repeated separator so the reader can
// tell at a glance where the context ends and the claim begins — "Widget ·
// Contract › Retry Policy" reads as "over in Widget/Contract, the Retry
// Policy claim", which a uniform "Widget › Contract › Retry Policy" would
// flatten into an undifferentiated path. Both are plain text in a document
// that declares <meta charset="utf-8"> (see viewer/template/shell.html).
const (
	claimRefModuleSep = " · "
	claimRefLabelSep  = " › "
)

// splitClaimID splits a claim id into its module/facet/slug segments — the
// shape internal/lint's id-shape lint enforces (exactly three dot-separated
// segments, with module and facet agreeing with the claim's own Module/Facet
// fields). ok is false unless the id is EXACTLY three segments and none of
// them is empty.
//
// This package deliberately re-checks a shape a lint already guarantees,
// because render does not run the lint suite: a draft claim, or any claim in
// a project whose author has not run "dossierx lint" yet, reaches render
// carrying whatever id its YAML happened to say. So "not three segments" is a
// state every caller here has to have a defined answer for (see ClaimLabel,
// whose answer is "the raw id, verbatim") rather than a state that indexes
// segs[2] and panics on the first unlinted claim anyone renders.
func splitClaimID(id string) (module, facet, slug string, ok bool) {
	segs := strings.Split(id, ".")
	if len(segs) != 3 || segs[0] == "" || segs[1] == "" || segs[2] == "" {
		return "", "", "", false
	}
	return segs[0], segs[1], segs[2], true
}

// ClaimLabel turns a claim id into the readable label the viewer shows in its
// place: "widget.contract.retry-policy" -> "Retry Policy". Only the slug
// segment becomes the label, because module and facet are page context the
// reader can already see rather than facts about the claim (edgesHTML's doc
// comment makes the same argument for keeping them off the footer).
//
// The label is DERIVED, never authored: adding a "title:" field to the claim
// schema would mean backfilling every existing claim in every consumer corpus
// and then policing agreement between a claim's id and its title forever —
// two names for one thing, which is the duplication this project's
// single-source-of-truth rule exists to rule out.
//
// An id that is not exactly three non-empty segments renders as the RAW ID,
// VERBATIM — never a partial label ("just the bit after the last dot"), which
// would silently mislabel an unlinted claim, and never a panic. See
// splitClaimID for why render cannot assume the linted shape.
//
// Exported both as a Go helper and, via funcMap, as the "claimLabel" template
// func, so all seven layout partials' <div class="k"> heading and
// build_order.html's per-claim heading share this one implementation instead
// of each re-deriving a label from {{.ID}} in template syntax.
func ClaimLabel(id string) string {
	_, _, slug, ok := splitClaimID(id)
	if !ok {
		return id
	}
	return DisplayCase(slug)
}

// writeClaimRef writes one claim-to-claim edge as an anchor to targetID,
// labeled for a reader sitting in fromModule/fromFacet. It hand-escapes every
// interpolation point for the same reason the rest of EdgesHTMLWithLinks does:
// a FuncMap-returned template.HTML bypasses html/template's automatic
// escaping, so nothing downstream will escape these for us — and an id that
// failed splitClaimID flows through here VERBATIM FROM YAML, which is exactly
// the input most likely to contain a quote or an angle bracket.
//
// Three elision tiers, keyed on how far the target is from the reader:
//
//	same module + facet   ->  "Retry Policy"
//	same module, other facet -> "Contract › Retry Policy"
//	other module          ->  "Widget · Contract › Retry Policy"
//
// Cross-boundary edges are the ones that carry real information — they are
// what internal/lint's hub-gating keys off — so the prefix is kept exactly
// where it distinguishes something and dropped exactly where it repeats the
// page the reader is already on.
//
// The machine id stays reachable two ways on the same element: data-claim-id
// (greppable in the rendered HTML, queryable from the viewer JS, and the
// attribute convention the comment chip already established) and a title
// tooltip. That is not decoration — "dossierx claim lock <id>" takes the id,
// "dossierx-claim: <id>" source tags take the id, and a reader who can only
// see "Retry Policy" cannot type either. A tooltip alone would be invisible on
// touch and to keyboard users, which is why the attribute rides along with it.
//
// Prefix segments go through DisplayCase like the label does, so a prefix
// reads the same as the nav entry and tab the reader would click to get there
// ("Token Ledger · Contract ›" for module "token-ledger"), rather than showing
// a second, raw spelling of names the chrome already renders capitalized.
// targetStatuses supplies the optional C6 status pill (issue #11's last
// unshipped piece) appended after the target's label: draft, or locked with
// review_pending — see targetPillHTML for the actionable-only gate. It is
// nil under the default, parse-time funcMap binding (see this file's
// funcMap/Load), which has no catalog to look a target's status up in, so
// every target simply renders with no pill there — the same output this
// function produced before the pill existed. Only internal/render's
// attachEdgesOverride, which does have the whole catalog, ever supplies a
// non-nil map.
func writeClaimRef(b *strings.Builder, targetID, fromModule, fromFacet string, targetStatuses map[string]TargetStatus) {
	esc := html.EscapeString(targetID)
	b.WriteString(`<a class="claim-ref" href="#`)
	b.WriteString(esc)
	b.WriteString(`" data-claim-id="`)
	b.WriteString(esc)
	b.WriteString(`" title="`)
	b.WriteString(esc)
	b.WriteString(`">`)

	module, facet, slug, ok := splitClaimID(targetID)
	if !ok {
		// Unshaped id: show it exactly as authored, marked so style.css can
		// render it as the machine string it is rather than as a label.
		b.WriteString(`<span class="claim-ref-label claim-ref-raw">`)
		b.WriteString(esc)
		b.WriteString(`</span></a>`)
		b.WriteString(targetPillHTML(targetID, targetStatuses))
		return
	}

	var prefix string
	switch {
	case module != fromModule:
		prefix = DisplayCase(module) + claimRefModuleSep + DisplayCase(facet)
	case facet != fromFacet:
		prefix = DisplayCase(facet)
	}
	if prefix != "" {
		b.WriteString(`<span class="claim-ref-prefix">`)
		b.WriteString(html.EscapeString(prefix + claimRefLabelSep))
		b.WriteString(`</span>`)
	}

	b.WriteString(`<span class="claim-ref-label">`)
	b.WriteString(html.EscapeString(DisplayCase(slug)))
	b.WriteString(`</span></a>`)
	b.WriteString(targetPillHTML(targetID, targetStatuses))
}

// ClaimEdgeListHTML renders ids as the same nested <ul class="claim-edge-id-
// list"> the shared edges footer emits, labeled and prefix-elided relative to
// fromID — the id of the claim the list is being rendered UNDER.
//
// It exists for build_order.html, the one template that renders a rests_on
// edge itself instead of going through {{edges .}}: it lists a whole locked
// build-order artifact, whose claim entries are internal/buildorder's own view
// type, not a model.Claim, so the edges footer cannot reach them. That
// independent rendering had already drifted (an inline, comma-separated run
// rather than the footer's bulleted list); routing it through this func
// converges the two on one markup shape and one label implementation, so a
// future change to either lands in both.
//
// The reader's context is derived from fromID's own segments rather than
// passed separately, because a buildorder claim entry carries only an id. An
// unshaped fromID yields empty module/facet, which simply means nothing
// matches and every target keeps its full "Module · Facet ›" prefix — a
// degraded label, never a wrong one.
func ClaimEdgeListHTML(fromID string, ids []string) template.HTML {
	module, facet, _, _ := splitClaimID(fromID)
	var b strings.Builder
	// build_order.html only ever lists LOCKED claims (see that partial's
	// own doc comment — attachBuildOrders filters to locked-only
	// artifacts before this ever executes), and "claimEdgeList" has no
	// override binding of its own to carry a catalog lookup through the
	// way "edges" does, so this always renders with no target pill.
	writeIDListItems(&b, module, facet, ids, nil)
	return template.HTML(b.String())
}

// DisplayCase renders a raw module/facet/slug value (e.g. "token-ledger" or
// "token_ledger") as a human-readable label ("Token Ledger"): '-' and '_'
// become spaces, and each resulting word is capitalized. It is a display-only
// transform — nothing derived from it is ever used as an id, hash fragment, or
// lookup key.
//
// It lives here, rather than in internal/render where it started life as the
// unexported displayCase powering module/facet nav labels, because render
// imports components and not the other way round: ClaimLabel needs the exact
// same transform, and a second copy would let a card's label drift away from
// the nav label naming the very same facet. internal/render's displayCase is
// now a one-line wrapper over this.
func DisplayCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, w := range words {
		if w == "" {
			continue
		}
		r := []rune(w)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
