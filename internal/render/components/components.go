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

//go:embed card.html table.html list.html steps.html tree.html banner.html mockup.html build_order.html
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
	"rowKeys":   rowKeys,
	"markdown":  markdown.Render,
	"cell":      cell,
	"edges":     edgesHTML,
	"inc":        inc,
	"pillClass":  pillClass,
	"colClass":   colClass,
	"mockupHTML": mockupHTML,
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
	return EdgesHTMLWithLinks(c, nil, nil)
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
func EdgesHTMLWithLinks(c model.Claim, files []implink.ViewFile, dependedBy []string) template.HTML {
	var b strings.Builder
	b.WriteString(`<ul class="claim-edges">`)

	if c.Governed.Type != "" {
		if c.Governed.Type == string(model.GovernedNone) {
			b.WriteString(`<li class="claim-governed governed-none">governed_by: none`)
			if c.Governed.Reason != "" {
				b.WriteString(` — `)
				b.WriteString(html.EscapeString(c.Governed.Reason))
			}
			b.WriteString(`</li>`)
		} else {
			b.WriteString(`<li class="claim-governed">governed_by: <a href="#`)
			b.WriteString(html.EscapeString(c.Governed.Type))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(c.Governed.Type))
			b.WriteString(`</a></li>`)
		}
	}

	if len(c.Mirrors) > 0 {
		b.WriteString(`<li class="claim-mirrors">mirrors:`)
		writeIDListItems(&b, c.Mirrors)
		b.WriteString(`</li>`)
	}

	if len(c.RestsOn) > 0 {
		b.WriteString(`<li class="claim-rests-on">rests_on:`)
		writeIDListItems(&b, c.RestsOn)
		b.WriteString(`</li>`)
	}

	if len(dependedBy) > 0 {
		b.WriteString(`<li class="claim-depended-by">depended on by:`)
		writeIDListItems(&b, dependedBy)
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

// writeIDListItems renders ids as a nested <ul> of <li><a href="#id">id</a>
// </li> entries, used for every edges-footer field that lists other claim
// ids (mirrors, rests_on, depended-by) so each id gets its own bulleted
// line rather than a run-on comma list.
func writeIDListItems(b *strings.Builder, ids []string) {
	b.WriteString(`<ul class="claim-edge-id-list">`)
	for _, id := range ids {
		b.WriteString(`<li><a href="#`)
		b.WriteString(html.EscapeString(id))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(id))
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ul>`)
}
