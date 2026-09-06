// Package render turns a built catalog.Catalog into the viewer's
// index.html. The shell + CSS live under viewer/template/ and are embedded
// into the binary via go:embed, with soft-fallback overrides for
// shell.html/style.css sourced from cfg.Viewer.TemplateOverrides (see
// components.OverrideFile); per-layout partials live in the components/
// subpackage and are selected via a plain map lookup on each claim's Layout
// — no per-project branching lives here or in components.
//
// The override mechanism covers the shell and CSS ONLY. The claims-graph
// client files — graph-core.js, graph-ui.js and graph.css — are embedded and
// read straight from shellFS with no OverrideFile branch. They are engine
// internals with a tight contract against the graph payload's shape and
// against each other, so a project that swapped one for its own copy would
// get a pane that fails in ways no error message could usefully describe.
// Anything that must vary per project reaches them through the JSON payload,
// never through a template action: the three files are injected as DATA and
// are never parsed as templates, so "{{" in their source is inert.
package render

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// The graph and viewer-runtime paths are named individually rather than embedding the
// whole directory: //go:embed resolves at COMPILE time, so a directive
// naming a file that does not exist fails "go build" loudly. Embedding
// viewer/template/* and reading the graph files at render time would turn
// exactly the same mistake — a client file deleted, renamed or never
// written — into a silently empty pane.
//
//go:embed viewer/template/shell.html viewer/template/style.css viewer/template/system-record.js viewer/template/viewer-runtime.js viewer/template/graph-core.js viewer/template/graph-ui.js viewer/template/graph.css viewer/template/build-order.html viewer/template/build-order-ui.js viewer/template/vendor/mermaid.min.js
var shellFS embed.FS

// shellFileName and styleFileName are the override-lookup names for the
// shell template and stylesheet respectively (see components.OverrideFile),
// and also the leaf names of their embedded viewer/template/ counterparts
// below. They are hoisted into constants — rather than repeated as string
// literals at each use site — so a future rename of either file only has to
// change one place instead of staying manually in sync across the
// OverrideFile lookup, the template.New name, and the embedded path.
//
// graphCoreFileName, graphUIFileName and graphCSSFileName follow the same
// pattern for one reason less: they have no OverrideFile lookup at all (see
// the package doc comment), so their only two uses are the embed directive
// and the ReadFile below.
const (
	shellFileName         = "shell.html"
	styleFileName         = "style.css"
	graphCoreFileName     = "graph-core.js"
	graphUIFileName       = "graph-ui.js"
	graphCSSFileName      = "graph.css"
	systemRecordFileName  = "system-record.js"
	viewerRuntimeFileName = "viewer-runtime.js"

	// buildOrderFileName is the Build order tab's per-module partial. It is
	// parsed from the embedded FS ONLY — it is not an override point (see
	// loadTemplates' refusal of a legacy build_order.html override), so it
	// lives beside the shell rather than under components/.
	buildOrderFileName = "build-order.html"
	// buildOrderUIFileName and mermaidFileName are the tab's two client
	// files: the engine's own renderer glue and the vendored mermaid build
	// (third_party/mermaid/ records its version, licence and hash). Both are
	// injected as template.JS and only into a viewer with at least one
	// locked build order — see shell.html's guard.
	buildOrderUIFileName = "build-order-ui.js"
	mermaidFileName      = "vendor/mermaid.min.js"
)

// shellTemplatePath and styleTemplatePath are the embedded paths backing
// the embed directive above; embed.FS.ReadFile/ParseFS calls below
// reference these constants (viewer/template/ + the corresponding
// *FileName constant) instead of repeating the path as a separate literal.
const (
	shellTemplatePath         = "viewer/template/" + shellFileName
	styleTemplatePath         = "viewer/template/" + styleFileName
	graphCoreTemplatePath     = "viewer/template/" + graphCoreFileName
	graphUITemplatePath       = "viewer/template/" + graphUIFileName
	graphCSSTemplatePath      = "viewer/template/" + graphCSSFileName
	systemRecordTemplatePath  = "viewer/template/" + systemRecordFileName
	viewerRuntimeTemplatePath = "viewer/template/" + viewerRuntimeFileName
	buildOrderTemplatePath    = "viewer/template/" + buildOrderFileName
	buildOrderUITemplatePath  = "viewer/template/" + buildOrderUIFileName
	mermaidTemplatePath       = "viewer/template/" + mermaidFileName
)

// generatedHeader returns the comment prepended to every rendered document
// ahead of the shell template's own output, stamped with the render time
// (UTC, RFC3339) so provenance in the emitted HTML is unambiguous. It is
// built in Go, not baked into shell.html, because html/template silently
// strips literal HTML comments out of parsed templates as part of its
// escaping pass — a header comment placed inside the template source would
// never reach the rendered output.
//
// generatedAt is threaded in from Render rather than each stamping its own
// time.Now(), so the comment and the sidebar's visible timestamp (shellData
// .GeneratedAt, see buildShellData) always agree — a reviewer comparing the
// two never sees a mismatch from two clock reads a few instructions apart.
func generatedHeader(generatedAt time.Time) string {
	// The banner names "dossierx check", not the "dossierx render" it named
	// before v0.3.0: render stopped being a verb when the surface went 26 -> 19
	// and became a STAGE of check. This string is stamped into every generated
	// index.html, so a stale verb here is the single most widely-read wrong
	// instruction the engine can emit — every reader who opens the file sees it.
	return fmt.Sprintf("<!-- generated by dossierx check at %s — do not edit. Re-run \"dossierx check\" to regenerate. -->\n", generatedAt.Format(time.RFC3339))
}

type shellData struct {
	Title string
	// Eyebrow is cfg.Eyebrow verbatim, rendered as a one-line subtitle under
	// Title in the sidebar header. Empty means shell.html renders no eyebrow
	// element at all (unlike Title, there is no generic fallback text).
	Eyebrow string
	CSS     template.CSS
	// ThemeCSS is a second, optional :root{...} block built from
	// cfg.Viewer.Theme (see themeOverrideCSS). shell.html emits it in a
	// <style> block immediately after the one carrying CSS, and after
	// style.css's own @media (prefers-color-scheme: light) block, so any
	// project-supplied token wins over the engine default in both OS color
	// modes, per-token (unset tokens keep falling back to style.css's own
	// defaults via normal cascade). Empty when the project sets no theme.
	ThemeCSS template.CSS

	// GeneratedAt is the same render timestamp stamped into generatedHeader's
	// leading HTML comment, formatted for human display in the sidebar
	// footer so a reviewer can tell at a glance how fresh the page they're
	// looking at is (the comment alone is invisible in the rendered page —
	// only visible in "view source").
	GeneratedAt string

	// ---- the claims graph pane's four injection sites ----
	//
	// THE TYPES ON THESE FOUR FIELDS ARE LOAD-BEARING AND FAIL SILENTLY.
	//
	// html/template escapes CONTEXTUALLY. A plain string reaching
	// <script>{{.GraphCoreJS}}</script> is not emitted as source — it is
	// JSON-marshalled into a quoted JS string literal, so 1,300 lines of
	// program become one inert string expression. Inside <style> a plain
	// string is filtered to the literal ZgotmplZ. Neither produces an error
	// at build time, at render time, or in any test that asserts on the Go
	// value, because the Go value is correct in both worlds. The only visible
	// symptom is a pane that never initializes.
	//
	// template.CSS and template.JS are the "already made safe" declarations
	// that suppress that escaping. They are safe here for a reason that must
	// stay true: all three client files are ENGINE-OWNED bytes straight off
	// the embedded FS, never project input and never concatenated with any.
	// GraphPayload is the one value derived from author input, and it is safe
	// by a different mechanism — see its own comment below.
	//
	// graph_render_test.go asserts every one of these against the RENDERED
	// DOCUMENT rather than against the Go value, for exactly this reason.
	GraphCSS template.CSS

	// GraphPayload is graph.Encode's bytes, injected into
	// <script type="application/json" id="dossierx-graph">. html/template
	// applies NO escaping at all in that context, so the guard is entirely
	// encoding/json's DEFAULT HTML escaping, which writes '<' as <
	// before the bytes ever reach this field. A JSON parser reads that back
	// as the original character; an HTML parser never sees a tag. Do not
	// re-marshal, post-process or "clean up" the escaped output here, and
	// never turn that escaping off anywhere on its path. (The encoder toggle
	// that would turn it off is deliberately not named here: it is enforced by
	// a repo-wide grep for the identifier, so writing it out — even to forbid
	// it — is the one thing that makes the gate fail. internal/graph's package
	// doc comment makes the same point at length.)
	GraphPayload template.JS

	// GraphCoreJS and GraphUIJS are the pane's two script files, injected in
	// that order (core exports the namespace ui consumes) after the shell's
	// own inline runtime.
	GraphCoreJS     template.JS
	GraphUIJS       template.JS
	SystemRecordJS  template.JS
	ViewerRuntimeJS template.JS

	// BuildOrders is the Build order tab: one entry per module with a LOCKED
	// build-order artifact, in module order (see build_order_view.go). Its
	// Modules slice is nil for a project with no locked order, and shell.html
	// guards every byte of the tab — the sidebar group, the section, the
	// payload block and the two script tags — on that, so such a project
	// renders not one byte of it and never carries the vendored renderer.
	BuildOrders BuildOrderTab
	// BuildOrderPayload is the tab's JSON payload (buildOrderPayloadJSON),
	// injected into <script type="application/json" id="dossierx-build-orders">
	// under the same escaping contract as GraphPayload: encoding/json's
	// default HTML escaping is the whole guard, applied before these bytes
	// exist. It sits INSIDE <main class="content-area"> so a serve fragment
	// swap re-delivers it beside the diagrams it describes.
	BuildOrderPayload template.JS
	// MermaidJS is the vendored mermaid build and BuildOrderUIJS the engine's
	// renderer glue, both engine-owned bytes off the embedded FS, injected
	// after GraphUIJS and only inside the {{if .BuildOrders.Modules}} guard.
	MermaidJS      template.JS
	BuildOrderUIJS template.JS

	// ModuleGroups is cat.Claims folded into the two-level Module -> []Facet
	// shape fix 5 describes (one sidebar entry per module, a nested
	// .sub-nav/.subtab strip per module with more than one facet), computed
	// by buildModuleGroups from the flat, facet-level Groups buildGroups
	// produces. shell.html ranges exclusively over ModuleGroups (and each
	// entry's nested Facets) — the flat Groups slice is not exposed on
	// shellData; it exists only as buildModuleGroups' internal input so the
	// module/facet nav ordering logic in buildGroups stays the single source
	// of truth without being duplicated here.
	ModuleGroups []ModuleGroup

	// Tracks is the project's declared cross-cutting tracks, one section each,
	// rendered after every module section and listed after every module in the
	// sidebar. NIL FOR A PROJECT THAT DECLARES NONE, and shell.html guards
	// every byte of track markup on that — a corpus with no tracks must render
	// exactly as it did before the axis existed. See track_view.go.
	Tracks []TrackSection
}

// Group is one module/facet section of the sidebar nav + content area, as
// described in NAV_SPEC. A claim whose module and/or facet is empty or not
// recognized by the project config lands in a single catch-all group with
// Module == ungroupedModuleName instead of being dropped.
type Group struct {
	// Module and Facet are the raw (unslugified) group keys, suitable for
	// display labels. Module == ungroupedModuleName and Facet == "" for the
	// catch-all bucket.
	Module string
	Facet  string
	// ID is a URL/HTML-id-safe slug of "<module>-<facet>" (or just
	// "<module>" when Facet is empty), used as both the claim-group
	// section id and the sec-tab's data-target/hash.
	ID string
	// Claims are this group's claims, already rendered via the existing
	// per-layout partials, ordered per orderClaims (explicit Order first,
	// then a stable SourcePath-order fallback for everything else).
	Claims []template.HTML
	// AllLocked is true only when the group is non-empty and every claim in
	// it has Status == locked; it drives an optional lock-indicator suffix
	// on the nav label. A group can never be empty in practice (it only
	// exists because at least one claim produced it), but the check is
	// written defensively regardless.
	AllLocked bool
	// ModuleLabel is a display-cased version of Module, used for the
	// sec-label heading shown once per module run.
	ModuleLabel string
	// TabLabel is a display-cased version of Facet (or of Module when Facet
	// is empty, e.g. the ungrouped catch-all bucket), used as the sec-tab's
	// button text.
	TabLabel string
	// firstInModule is true for the first group of each consecutive module
	// run in buildGroups' output. It is unexported (rather than
	// FirstInModule) precisely because it is not part of the template-facing
	// contract: no shell.html or partial ever ranges over a []Group and
	// reads this field, only buildModuleGroups does, in the same package, to
	// find each run's start when folding Groups into ModuleGroups. Keeping
	// it unexported makes that "internal plumbing only" claim something the
	// compiler enforces (a template accessing it would fail at execute time)
	// rather than something only a comment asserts.
	firstInModule bool
}

// ModuleGroup is one module's sidebar nav entry together with the one or
// more facet-level Groups shown inside its content section — the two-level
// Module -> []Facet shape fix 5 describes (docs/'s real nav has one sec-tab
// per MODULE; a module with more than one facet gets a secondary
// .sub-nav/.subtab strip inside its content area instead of one flat
// sidebar entry per module+facet pair). It is built by buildModuleGroups
// from the same flat, already-ordered Groups buildGroups produces, so the
// module/facet nav ordering logic in buildGroups stays the single source of
// truth and is not duplicated here.
type ModuleGroup struct {
	// Module is the raw (unslugified) module key; Module ==
	// ungroupedModuleName for the catch-all bucket, matching Group.Module.
	Module string
	// ModuleLabel is the display-cased Module, used for the sidebar's
	// module-level sec-tab label. It is copied from the first Facet rather
	// than recomputed, so ModuleGroup never disagrees with its own Facets
	// about what a module's display name is.
	ModuleLabel string
	// ID is slugify(Module) alone — never "module-facet" — used as the
	// module-level sec-tab's data-target/hash. It intentionally does not
	// have to equal Facets[0].ID (that ID may carry a facet suffix even for
	// a single-facet module); resolving a bare "#module" hash to
	// FirstFacetID's section is the later shell.html step's job.
	ID string
	// Facets are this module's facet-level groups, already in the nav
	// order buildGroups computed for them (declared facets first, then any
	// remaining present facets alphabetically). Always non-empty — a
	// ModuleGroup only ever exists because at least one Group produced it.
	Facets []Group
	// FirstFacetID is Facets[0].ID — the facet section that should render
	// visible-by-default when this module's sec-tab is chosen, whether by
	// click or by a bare "#module" hash with no facet suffix.
	FirstFacetID string
	// HasSubNav is true only when len(Facets) > 1. A module with exactly
	// one facet renders no .sub-nav/.subtab strip at all — there is
	// nothing to switch between — per fix 5's "skip the sub-nav entirely
	// for a module with exactly 1 facet" requirement.
	HasSubNav bool
	// AllLocked is true only when every facet in Facets has AllLocked ==
	// true (which itself requires every claim within that facet to be
	// locked). It drives the same optional lock-indicator suffix on the
	// module-level nav label that Group.AllLocked drives per facet.
	AllLocked bool
}

// buildModuleGroups folds buildGroups' flat, facet-level Groups into the
// two-level ModuleGroup shape above: one ModuleGroup per consecutive run of
// same-Module Groups. It trusts Group.firstInModule (already computed by
// buildGroups) to find each run's boundary instead of re-deriving "did the
// module change" here — buildGroups already guarantees a given module's
// Groups are contiguous in its output. groups and firstInModule are purely
// an internal handoff between the two functions: shell.html never sees a
// []Group directly, only the ModuleGroups this returns. Returns nil for
// nil/empty groups.
func buildModuleGroups(groups []Group) []ModuleGroup {
	if len(groups) == 0 {
		return nil
	}

	var out []ModuleGroup
	for _, g := range groups {
		if g.firstInModule {
			out = append(out, ModuleGroup{
				Module:      g.Module,
				ModuleLabel: g.ModuleLabel,
				ID:          slugify(g.Module),
			})
		}
		last := &out[len(out)-1]
		last.Facets = append(last.Facets, g)
	}

	for i := range out {
		out[i].HasSubNav = len(out[i].Facets) > 1
		out[i].FirstFacetID = out[i].Facets[0].ID

		allLocked := true
		for _, f := range out[i].Facets {
			if !f.AllLocked {
				allLocked = false
				break
			}
		}
		out[i].AllLocked = allLocked
	}

	return out
}

// ungroupedModuleName is the catch-all bucket's Module value for claims
// whose module and/or facet is empty or not declared in the project config.
const ungroupedModuleName = "ungrouped"

// Render builds the full viewer/index.html document for cat and returns it
// as a string. It never panics on an empty catalog: Render(&catalog.Catalog{}, cfg)
// returns a valid, claim-less document.
//
// The work is split into three independently testable stages: loadTemplates
// resolves every override-able input (component partials, CSS, the shell
// template itself) against cfg.Viewer.TemplateOverrides; renderClaims turns
// each catalog claim into HTML via those partials; buildShellData assembles
// the resulting shellData (title/eyebrow/groups) ready for shell.Execute.
// Render itself is left as the sequencing of those three calls plus the
// final template execution, so a future fourth input or grouping level only
// has to touch the stage it belongs to.
func Render(cat *catalog.Catalog, cfg *config.Config) (string, error) {
	rt, err := config.ResolveTheme(cfg, os.ReadFile)
	if err != nil {
		return "", err
	}
	return RenderWithTheme(cat, cfg, rt)
}

// RenderWithTheme is Render with the theme already resolved. It is the real
// entry point; Render is the convenience wrapper that resolves against the
// working tree with os.ReadFile.
//
// The split exists because two callers must NOT read the working tree.
// "dossierx check --staged" evaluates the index's bytes, through a reader
// built on git plumbing, and "dossierx serve" resolves once at startup so
// that every rebuild in a long-running server emits the same theme rather
// than re-reading font files that may be half-written under the user's
// editor. Both call config.ResolveTheme themselves with the reader they
// need and hand the result here. Keeping Render's signature unchanged keeps
// every other caller — and every existing test — untouched.
func RenderWithTheme(cat *catalog.Catalog, cfg *config.Config, rt *config.ResolvedTheme) (string, error) {
	if cat == nil {
		cat = &catalog.Catalog{}
	}

	overrideDir := ""
	if cfg != nil {
		overrideDir = cfg.Viewer.TemplateOverrides
	}

	tmpl, err := loadTemplates(overrideDir)
	if err != nil {
		return "", err
	}

	// Optional, graceful-degradation-by-default extensions of the shared
	// edges footer — see implink_view.go and depended_by_view.go's doc
	// comments for why this is a no-op for a project that has never
	// called "dossierx implink set" and has no claim any other claim rests
	// on.
	attachEdgesOverride(tmpl.partials, buildImplinkLookup(cfg), buildDependedByLookup(cat), buildTargetStatusLookup(cat))

	// Rebind mockup.html's "mockupHTML" func with the project's
	// mockup_modules allowlist so its defense-in-depth gate (DX-AUD-08) can
	// verify module membership; the default binding always escapes.
	attachMockupOverride(tmpl.partials, cfg)

	renderedByID, err := renderClaims(cat, tmpl.partials)
	if err != nil {
		return "", err
	}

	generatedAt := time.Now().UTC()

	graphPayload, err := graphPayloadJSON(cat, cfg, generatedAt)
	if err != nil {
		return "", err
	}

	buildOrders, buildOrderPayload, err := buildOrderTabData(cat, cfg, tmpl.buildOrder, generatedAt)
	if err != nil {
		return "", err
	}

	data := buildShellData(shellInputs{
		cat:             cat,
		cfg:             cfg,
		css:             tmpl.css,
		graphCSS:        tmpl.graphCSS,
		graphCoreJS:     tmpl.graphCore,
		graphUIJS:       tmpl.graphUI,
		systemRecordJS:  tmpl.systemRecord,
		viewerRuntimeJS: tmpl.viewerRuntime,
		graphPayload:    graphPayload,
		renderedByID:    renderedByID,
		generatedAt:     generatedAt,
		theme:           rt,

		buildOrders:       buildOrders,
		buildOrderPayload: buildOrderPayload,
		mermaidJS:         tmpl.mermaidJS,
		buildOrderUIJS:    tmpl.buildOrderUI,
	})

	// The tab's section and per-module group ids are namespaced out of the
	// module slug space; the one shape slugify can still produce is refused
	// by name here, the way loadTemplates refuses a legacy override.
	if err := buildOrderIDCollision(data.BuildOrders, data.ModuleGroups); err != nil {
		return "", err
	}

	var out bytes.Buffer
	if err := tmpl.shell.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render: execute shell template: %w", err)
	}

	return generatedHeader(generatedAt) + out.String(), nil
}

// loadedTemplates bundles every override-able render input resolved by
// loadTemplates: the per-layout component partials, the stylesheet bytes,
// and the parsed shell template. Grouping them lets loadTemplates return a
// single value instead of Render having to thread three separate return
// values through to their eventual use sites.
type loadedTemplates struct {
	partials map[model.Layout]*template.Template
	css      []byte
	shell    *template.Template
	// buildOrder is the Build order tab's per-module partial
	// (viewer/template/build-order.html), parsed off the embedded FS with NO
	// override branch — see loadTemplates for the refusal a legacy
	// build_order.html override meets. Whether it is ever executed depends on
	// buildOrderTabData finding a module with a locked artifact.
	buildOrder *template.Template
	// mermaidJS and buildOrderUI are the tab's two client files, raw bytes
	// like the graph files above and typed template.JS at the shellData
	// boundary.
	mermaidJS    []byte
	buildOrderUI []byte

	// graphCore, graphUI and graphCSS are the claims-graph client files,
	// always the embedded engine copies. Unlike css and shell above they have
	// no override branch at all — see the package doc comment. They are kept
	// as raw []byte here and typed (template.JS / template.CSS) only at the
	// shellData boundary, which is the one place the typing is load-bearing.
	graphCore     []byte
	graphUI       []byte
	graphCSS      []byte
	systemRecord  []byte
	viewerRuntime []byte
}

// loadTemplates resolves all of Render's template and CSS inputs, applying
// the project's override directory (overrideDir, from
// cfg.Viewer.TemplateOverrides) over the engine's embedded defaults: a
// project may override style.css and/or shell.html independently, and falls
// back to the embedded copy of each when it does not.
func loadTemplates(overrideDir string) (loadedTemplates, error) {
	partials, err := components.Load(overrideDir)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load component templates: %w", err)
	}

	// build_order.html was an override point until the Build order tab
	// replaced the list it rendered; its data shape is gone with the list. A
	// project still carrying one is TOLD, by name, rather than handed a
	// template executed against a shape it was never written for — or,
	// worse, silently ignored.
	if _, found, err := components.OverrideFile(overrideDir, legacyBuildOrderOverrideName); err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s override: %w", legacyBuildOrderOverrideName, err)
	} else if found {
		return loadedTemplates{}, fmt.Errorf("render: viewer.template_overrides contains %s, which is no longer an override point — the Build order tab is not overridable; delete the file", legacyBuildOrderOverrideName)
	}
	buildOrderTmpl, err := template.ParseFS(shellFS, buildOrderTemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: parse %s: %w", buildOrderFileName, err)
	}

	css, cssOverridden, err := components.OverrideFile(overrideDir, styleFileName)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s override: %w", styleFileName, err)
	}
	if !cssOverridden {
		css, err = shellFS.ReadFile(styleTemplatePath)
		if err != nil {
			return loadedTemplates{}, fmt.Errorf("render: load default stylesheet: %w", err)
		}
	}

	shellSrc, shellOverridden, err := components.OverrideFile(overrideDir, shellFileName)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s override: %w", shellFileName, err)
	}
	var shell *template.Template
	if shellOverridden {
		shell, err = template.New(shellFileName).Parse(string(shellSrc))
		if err != nil {
			return loadedTemplates{}, fmt.Errorf("render: parse shell template override: %w", err)
		}
	} else {
		shell, err = template.ParseFS(shellFS, shellTemplatePath)
		if err != nil {
			return loadedTemplates{}, fmt.Errorf("render: parse shell template: %w", err)
		}
	}

	// The three graph client files: plain reads off the embedded FS, no
	// override lookup. A failure here means the embedded FS itself is
	// inconsistent with the embed directive, which is a build-level bug and
	// deserves the error rather than an empty pane.
	graphCore, err := shellFS.ReadFile(graphCoreTemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s: %w", graphCoreFileName, err)
	}
	graphUI, err := shellFS.ReadFile(graphUITemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s: %w", graphUIFileName, err)
	}
	graphCSS, err := shellFS.ReadFile(graphCSSTemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s: %w", graphCSSFileName, err)
	}
	systemRecord, err := shellFS.ReadFile(systemRecordTemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s: %w", systemRecordFileName, err)
	}
	viewerRuntime, err := shellFS.ReadFile(viewerRuntimeTemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s: %w", viewerRuntimeFileName, err)
	}
	mermaidJS, err := shellFS.ReadFile(mermaidTemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s: %w", mermaidFileName, err)
	}
	buildOrderUI, err := shellFS.ReadFile(buildOrderUITemplatePath)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s: %w", buildOrderUIFileName, err)
	}

	return loadedTemplates{
		partials:      partials,
		css:           css,
		shell:         shell,
		buildOrder:    buildOrderTmpl,
		graphCore:     graphCore,
		graphUI:       graphUI,
		graphCSS:      graphCSS,
		systemRecord:  systemRecord,
		viewerRuntime: viewerRuntime,
		mermaidJS:     mermaidJS,
		buildOrderUI:  buildOrderUI,
	}, nil
}

// ViewerRuntimeMarker is a stable token the default shell.html emits (a
// <meta name="dossierx-viewer-runtime"> in <head>) to attest that the template
// carries the DossierX live-viewer runtime: the client-side hooks that mount the
// comment UI and consume /api/fragment on an SSE "changed" event. "dossierx
// serve" checks for it at startup — a project that ships its own shell.html
// override WITHOUT this marker (an older copy, or a hand-written minimal shell)
// cannot support live comments, so serve degrades to read-only and warns rather
// than letting writes silently no-op in a viewer that cannot reflect them. The
// token is intentionally not version-specific, so an override that copies the
// current default keeps working across engine releases.
const ViewerRuntimeMarker = "dossierx-viewer-runtime"

// ShellHasViewerRuntime reports whether the EFFECTIVE shell template for cfg —
// the project's shell.html override when present, else the embedded default —
// carries ViewerRuntimeMarker. It resolves the override exactly as loadTemplates
// does (cfg.Viewer.TemplateOverrides via components.OverrideFile, falling back to
// the embedded default), so the answer matches the shell GET / actually renders.
// A nil cfg, or one with no shell.html override, is the embedded default, which
// always carries the marker; only a shell.html override that drops it returns
// false. A stat/read failure other than "no override" is surfaced as an error.
func ShellHasViewerRuntime(cfg *config.Config) (bool, error) {
	overrideDir := ""
	if cfg != nil {
		overrideDir = cfg.Viewer.TemplateOverrides
	}
	src, overridden, err := components.OverrideFile(overrideDir, shellFileName)
	if err != nil {
		return false, fmt.Errorf("render: check viewer runtime: %w", err)
	}
	if !overridden {
		src, err = shellFS.ReadFile(shellTemplatePath)
		if err != nil {
			return false, fmt.Errorf("render: read default shell: %w", err)
		}
	}
	return bytes.Contains(src, []byte(ViewerRuntimeMarker)), nil
}

// renderClaims executes each cat.Claims entry through its layout's partial
// template in partials (as loaded by loadTemplates), returning a
// claim-ID-keyed lookup (renderedByID, consumed by buildGroups/newGroup) so
// a claim is rendered exactly once regardless of how many places reference
// it afterwards. shell.html has no top-level "all claims, unordered" view
// (it renders exclusively via ModuleGroups' nested Facets[].Claims), so
// renderClaims does not also keep a flat catalog-order slice around for it.
func renderClaims(cat *catalog.Catalog, partials map[model.Layout]*template.Template) (map[string]template.HTML, error) {
	renderedByID := make(map[string]template.HTML, len(cat.Claims))
	for _, c := range cat.Claims {
		tmpl, ok := partials[c.Layout]
		if !ok {
			return nil, fmt.Errorf("render: claim %q has unsupported layout %q", c.ID, c.Layout)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, c); err != nil {
			return nil, fmt.Errorf("render: claim %q: %w", c.ID, err)
		}
		renderedByID[c.ID] = template.HTML(buf.String())
	}
	return renderedByID, nil
}

// shellInputs is buildShellData's single argument: everything Render has
// already computed by the time the shell is assembled. It replaced six
// positional parameters when the graph pane added four more values to thread
// through — ten positional arguments at one call site is a shape where a
// transposed pair of []byte/template.JS values compiles and renders and is
// found only by a reader. Named fields make that particular mistake a
// compile error instead.
type shellInputs struct {
	cat *catalog.Catalog
	cfg *config.Config
	// css is style.css's bytes as loadTemplates resolved them (the project's
	// override when it has one, the embedded default otherwise).
	css []byte
	// graphCSS, graphCoreJS and graphUIJS are the three embedded client files
	// backing the graph pane, always the engine's own copies — they carry no
	// override branch (design section 7.2). graphPayload is the JSON graph
	// payload for cat, already stamped and encoded by graphPayloadJSON.
	graphCSS        []byte
	graphCoreJS     []byte
	graphUIJS       []byte
	systemRecordJS  []byte
	viewerRuntimeJS []byte
	graphPayload    template.JS

	renderedByID map[string]template.HTML
	generatedAt  time.Time
	// theme is the already-resolved viewer.theme, never re-read here.
	theme *config.ResolvedTheme

	// buildOrders and buildOrderPayload are buildOrderTabData's two outputs
	// for cat; mermaidJS and buildOrderUIJS the tab's two client files.
	buildOrders       BuildOrderTab
	buildOrderPayload template.JS
	mermaidJS         []byte
	buildOrderUIJS    []byte
}

// buildShellData assembles the shellData passed to shell.Execute: cfg's
// title/eyebrow/theme (with the same fallbacks Render has always applied
// when cfg is nil or leaves a field blank) and the module/facet groups
// computed from in.cat via buildGroups/buildModuleGroups, combined with the
// css/renderedByID inputs loadTemplates and renderClaims already produced.
// The Build order tab's data arrives already computed (buildOrderTabData,
// which does the artifact reads) and is copied through.
//
// The four graph fields are typed on the way OUT, not on the way in: see
// shellData.GraphCSS and the block of comments there for why plain strings
// at those injection sites fail silently.
func buildShellData(in shellInputs) shellData {
	cat, cfg := in.cat, in.cfg

	// groups is buildModuleGroups' input only — the flat, facet-level
	// grouping is not exposed on shellData (shell.html renders exclusively
	// via ModuleGroups below).
	groups := buildGroups(cat, cfg, in.renderedByID)
	moduleGroups := buildModuleGroups(groups)

	title := "dossierx viewer"
	eyebrow := ""
	if cfg != nil {
		if strings.TrimSpace(cfg.Title) != "" {
			title = cfg.Title
		}
		eyebrow = strings.TrimSpace(cfg.Eyebrow)
	}

	return shellData{
		Title:           title,
		Eyebrow:         eyebrow,
		CSS:             template.CSS(in.css),
		ThemeCSS:        themeOverrideCSS(in.theme),
		GeneratedAt:     in.generatedAt.Format("2006-01-02 15:04 UTC"),
		GraphCSS:        template.CSS(in.graphCSS),
		GraphPayload:    in.graphPayload,
		GraphCoreJS:     template.JS(in.graphCoreJS),
		GraphUIJS:       template.JS(in.graphUIJS),
		SystemRecordJS:  template.JS(in.systemRecordJS),
		ViewerRuntimeJS: template.JS(in.viewerRuntimeJS),
		ModuleGroups:    moduleGroups,
		// The Build order tab. Typed template.JS on the way out like the
		// graph fields, for the same silent-failure reason.
		BuildOrders:       in.buildOrders,
		BuildOrderPayload: in.buildOrderPayload,
		MermaidJS:         template.JS(in.mermaidJS),
		BuildOrderUIJS:    template.JS(in.buildOrderUIJS),
		// Built from the SAME renderedByID the module groups read, so a claim
		// a track owns is rendered exactly once no matter how many sections
		// point at it — the property newGroup's own lookup exists to hold.
		Tracks: buildTrackSections(cat, cfg, in.renderedByID),
	}
}

// themeOverrideCSS builds the project's theme stylesheet from an already
// resolved theme: the @font-face rules for its inlined fonts, then up to
// three token blocks. Parts are omitted entirely when they would be empty,
// and a wholly empty theme returns "" rather than an empty ":root{}" rule,
// so the shell's <style></style> element still exists (an override sheet
// that expects the element does not break) with nothing in it.
//
//  1. one @font-face per font, in the resolved slice's order;
//  2. ":root{...}" for tokens whose value is the same in both colour schemes;
//  3. "@media (prefers-color-scheme: light), print{:root{...}}";
//  4. "@media screen and (prefers-color-scheme: dark){:root{...}}".
//
// The two media lists are the whole of the print story (plan v4 A1). A
// project's light values apply to print as well as to the light scheme; its
// dark values are scoped to `screen`, so no dark override can reach a
// printed page even for a token the project only declared under `dark:`.
// That is why nothing restates the light palette inside an @media print
// block: under print the dark block simply does not match.
//
// Token order inside every block is config.ThemeTokenAllowlist's fixed
// order, which config.ResolveTheme has already imposed on the slices — not
// map iteration order, which Go randomizes — so two runs of the same engine
// over the same config produce byte-identical output.
func themeOverrideCSS(rt *config.ResolvedTheme) template.CSS {
	if rt.IsZero() {
		return ""
	}

	var b strings.Builder
	for _, f := range rt.Fonts {
		mime, format := fontFormat(f.Ext)
		if mime == "" {
			// config.ResolveTheme rejects any other extension; a font that
			// reached here with one is an engine bug, and emitting a rule
			// with an empty format() would be a silent one.
			continue
		}
		b.WriteString(`@font-face{font-family:"`)
		b.WriteString(f.Family)
		b.WriteString(`";src:url(data:`)
		b.WriteString(mime)
		b.WriteString(";base64,")
		b.WriteString(base64.StdEncoding.EncodeToString(f.Data))
		b.WriteString(`) format("`)
		b.WriteString(format)
		b.WriteString(`");font-weight:`)
		b.WriteString(f.Weight)
		b.WriteString(";font-style:")
		b.WriteString(f.Style)
		b.WriteString(";font-display:swap;}")
	}

	writeBlock(&b, "", rt.Shared)
	writeBlock(&b, "@media (prefers-color-scheme: light), print", rt.Light)
	writeBlock(&b, "@media screen and (prefers-color-scheme: dark)", rt.Dark)

	return template.CSS(b.String())
}

// writeBlock emits ":root{...}" for decls, wrapped in the media query at
// media when that is non-empty. An empty decls list writes nothing at all,
// which is what keeps a flat-only theme's output byte-identical to what
// this engine emitted before per-mode values existed.
func writeBlock(b *strings.Builder, media string, decls []config.ThemeDecl) {
	if len(decls) == 0 {
		return
	}
	if media != "" {
		b.WriteString(media)
		b.WriteString("{")
	}
	b.WriteString(":root{")
	for _, d := range decls {
		b.WriteString("--")
		b.WriteString(d.Token)
		b.WriteString(":")
		b.WriteString(d.Value)
		b.WriteString(";")
	}
	b.WriteString("}")
	if media != "" {
		b.WriteString("}")
	}
}

// fontFormat maps a lower-cased font file extension (including the dot) to
// the MIME type its data: URL carries and the string CSS's format() wants.
// Note that the two disagree for the sfnt formats — ".ttf" is font/ttf but
// format("truetype") — which is exactly the kind of pair that is wrong for
// years without anyone noticing, so both directions are pinned by test.
// An unknown extension returns two empty strings; config rejects those
// before emission ever sees them.
func fontFormat(ext string) (mime, format string) {
	switch ext {
	case ".woff2":
		return "font/woff2", "woff2"
	case ".woff":
		return "font/woff", "woff"
	case ".ttf":
		return "font/ttf", "truetype"
	case ".otf":
		return "font/otf", "opentype"
	}
	return "", ""
}

// buildGroups computes the module -> facet grouping described in NAV_SPEC.
// It never panics on an empty or nil catalog (returns nil groups) and never
// drops a claim: any claim whose module and/or facet is empty or not
// recognized by cfg lands in a single catch-all ungroupedModuleName group
// instead of being silently discarded.
func buildGroups(cat *catalog.Catalog, cfg *config.Config, renderedByID map[string]template.HTML) []Group {
	if cat == nil || len(cat.Claims) == 0 {
		return nil
	}

	var declaredModules, declaredFacets []string
	if cfg != nil {
		declaredModules = cfg.Modules
		declaredFacets = cfg.Facets
	}

	knownModule, knownFacet := newMembershipPredicates(declaredModules, declaredFacets)

	type groupKey struct{ module, facet string }
	claimsByKey := map[groupKey][]model.Claim{}
	overviewByModule := map[string][]model.Claim{}
	moduleSeen := map[string]bool{}
	facetSeenByModule := map[string]map[string]bool{}
	var ungrouped []model.Claim

	for _, c := range cat.Claims {
		if c.Facet == config.ReservedOverviewFacet && knownModule(c.Module) {
			overviewByModule[c.Module] = append(overviewByModule[c.Module], c)
			moduleSeen[c.Module] = true
			continue
		}
		if !knownModule(c.Module) || !knownFacet(c.Facet) {
			ungrouped = append(ungrouped, c)
			continue
		}
		k := groupKey{c.Module, c.Facet}
		claimsByKey[k] = append(claimsByKey[k], c)
		moduleSeen[c.Module] = true
		if facetSeenByModule[c.Module] == nil {
			facetSeenByModule[c.Module] = map[string]bool{}
		}
		facetSeenByModule[c.Module][c.Facet] = true
	}

	var groups []Group
	for _, m := range orderedNames(declaredModules, moduleSeen) {
		overview := model.OrderClaims(overviewByModule[m])
		// The overview note renders on every facet tab of its module, but a
		// given claim id may appear only once in a valid document: the first
		// (default) facet keeps the canonical, id-bearing copy, every other
		// facet gets an id-less, purely-presentational copy (DX-AUD-16).
		canonicalOverview := renderOverviewHTML(overview, renderedByID)
		idlessOverview := stripOverviewIDs(canonicalOverview, overview)
		for fi, f := range orderedNames(declaredFacets, facetSeenByModule[m]) {
			overviewHTML := idlessOverview
			if fi == 0 {
				overviewHTML = canonicalOverview
			}
			groups = append(groups, newGroup(m, f, claimsByKey[groupKey{m, f}], renderedByID, overviewHTML))
		}
	}

	if len(ungrouped) > 0 {
		groups = append(groups, newGroup(ungroupedModuleName, "", ungrouped, renderedByID, nil))
	}

	markFirstInModule(groups)

	return groups
}

// renderOverviewHTML pulls the already-rendered HTML (from renderedByID,
// keyed by claim ID, same lookup newGroup itself uses) for a module's
// overview claims, in the given order. It never re-renders — renderClaims
// already rendered every catalog claim, including overview-facet ones,
// exactly once.
func renderOverviewHTML(overview []model.Claim, renderedByID map[string]template.HTML) []template.HTML {
	if len(overview) == 0 {
		return nil
	}
	out := make([]template.HTML, 0, len(overview))
	for _, c := range overview {
		out = append(out, renderedByID[c.ID])
	}
	return out
}

// stripOverviewIDs returns copies of a module's already-rendered overview
// HTML (canonical, from renderOverviewHTML) with each claim's root
// id="<claim-id>" attribute removed — one entry per input, in the same
// order (canonical and overview are index-aligned, both built from the same
// ordered claim slice). buildGroups injects a module's overview claims into
// every one of that module's facet groups so the orientation note stays
// visible on every facet tab (see newGroup); but a given id may appear only
// once in a valid document, so only the module's first/default facet keeps
// the canonical (id-bearing) copy and every other facet gets these id-less,
// purely-presentational copies (DX-AUD-16). Only the exact ` id="<claim-id>"`
// attribute — and only its first occurrence, the root element's — is
// removed, so the visible content is untouched: a #<claim-id> deep-link
// resolves to the single canonical copy while the note still renders
// identically on every tab. Claim ids are constrained to [A-Za-z0-9_.-]
// (internal/lint's id-shape lint), none of which html/template escapes in a
// double-quoted attribute value, so the literal match is exact.
//
// The leading space in the match pattern is load-bearing, and more so since
// the claim-edge label work: the .k header now also carries the claim id, as
// data-claim-id="<claim-id>" and title="<claim-id>" (its visible text is the
// derived label — see components.ClaimLabel). Neither is preceded by a space
// immediately before `id="`, so neither can be mistaken for the root
// attribute, and both survive on every copy on purpose — a data-* hook and a
// tooltip are not document-unique the way id= is, and the reader of an
// injected copy still needs the machine id to act on it.
//
// v0.4.1 moved the comment chip out of the edges footer and into that same .k
// header, inside <span class="claim-comments-slot"> (components.CommentChipHTML,
// bound as the "commentChip" template func). Re-verified against the new
// markup, and the code below is unchanged: the slot span and the chip <button>
// it wraps carry class / hidden / data-claim-id / aria-* only — no ` id="`
// sequence anywhere — so the single Replace still lands on the root
// <section>'s id and nothing else. A third data-claim-id per copy is exactly
// the intended fan-out, for the reason above: shell.html's chip handlers
// address claims by data-claim-id precisely so an overview note injected into
// N facet tabs stays clickable in all N, while only one copy keeps id=.
func stripOverviewIDs(canonical []template.HTML, overview []model.Claim) []template.HTML {
	if len(canonical) == 0 {
		return nil
	}
	out := make([]template.HTML, len(canonical))
	for i, h := range canonical {
		out[i] = stripDuplicateClaimIDs(h, overview[i])
	}
	return out
}

// stripDuplicateClaimIDs returns one already-rendered claim with every element
// id it carries removed, for use as a NON-CANONICAL copy: the same claim is
// also rendered somewhere else on the page, and that copy keeps the ids.
//
// It exists because two features now inject a second copy of a claim — a
// module's overview note, repeated on each of that module's facet tabs, and a
// track section, which renders the claims the track owns inline while their
// modules keep guaranteeing them. Both need exactly this, and a claim id may
// appear only once in a valid document.
//
// TWO KINDS OF ID, BOTH FROM THE SAME PLACE THAT WROTE THEM. The root
// <section>'s ` id="<claim-id>"` is matched with its leading space and its
// closing quote, so the .k header's data-claim-id and title — which are not
// preceded by a space before `id="` and are not document-unique anyway — are
// untouched and survive on every copy, exactly as they did before. The source
// footer's row ids are enumerated from the claim's own Sources through
// components.ClaimSourceAnchorID rather than pattern-matched, so this function
// cannot disagree with the function that emitted them.
//
// The consequence for the duplicate copy is a degraded, never wrong, landing:
// its citation markers still name the canonical copy's rows, so a reader
// clicking one is taken to the same evidence in the claim's own module. That
// is the same trade the overview note has always made with `#<claim-id>`.
//
// Claim ids are constrained to [A-Za-z0-9_.-] (internal/lint's id-shape lint),
// none of which html/template escapes in a double-quoted attribute value, so
// the literal match is exact; components refuses to emit a source anchor at all
// for an id outside that set (see ClaimSourceAnchorPrefix), so an unlinted
// claim has nothing here to miss.
func stripDuplicateClaimIDs(h template.HTML, c model.Claim) template.HTML {
	s := strings.Replace(string(h), ` id="`+c.ID+`"`, "", 1)
	for _, src := range c.Sources {
		id := components.ClaimSourceAnchorID(c, src.Ref)
		if id == "" {
			continue
		}
		s = strings.Replace(s, ` id="`+id+`"`, "", 1)
	}
	return template.HTML(s)
}

// newMembershipPredicates builds the knownModule/knownFacet predicates used
// by buildGroups to decide whether a claim's module and facet belong to the
// project's declared taxonomy. Lookup sets are built once so the returned
// predicates are O(1) per call instead of re-scanning declaredModules/
// declaredFacets for every claim in the catalog (O(claims) overall rather
// than O(claims * declared)).
//
// A module/facet is "recognized" when the project config declares a list
// and the value appears in it. When the config declares no list at all (nil
// cfg, or a config with an empty Modules/Facets — validate() normally
// forbids the latter, but this stays defensive), any non-empty value is
// accepted and grouping falls back to alphabetical order for it in
// buildGroups.
func newMembershipPredicates(declaredModules, declaredFacets []string) (knownModule, knownFacet func(string) bool) {
	declaredModuleSet := stringSet(declaredModules)
	declaredFacetSet := stringSet(declaredFacets)

	knownModule = func(m string) bool {
		if m == "" {
			return false
		}
		if len(declaredModuleSet) == 0 {
			return true
		}
		return declaredModuleSet[m]
	}
	knownFacet = func(f string) bool {
		if f == "" {
			return false
		}
		if len(declaredFacetSet) == 0 {
			return true
		}
		return declaredFacetSet[f]
	}
	return knownModule, knownFacet
}

// markFirstInModule stamps firstInModule on each group in place: true for
// the first group of a new module (including the very first group overall),
// false for subsequent groups within the same module. groups is assumed to
// already be ordered by module (as buildGroups produces it).
func markFirstInModule(groups []Group) {
	prevModule := ""
	for i := range groups {
		groups[i].firstInModule = i == 0 || groups[i].Module != prevModule
		prevModule = groups[i].Module
	}
}

// orderedNames returns every name in present (a set), ordered by its
// position in preferred first, then any remaining present names not found
// in preferred, appended in alphabetical order. It is used for both
// module ordering (against cfg.Modules) and facet ordering (against
// cfg.Facets) so unlisted-but-present names still get a stable, deterministic
// nav position instead of being dropped or ordered randomly.
//
// The "rest" branch below (names present but not in preferred) is
// intentionally kept even though it is unreachable in practice for a
// lint-clean project: the id-shape lint (internal/lint.IDShapeLint) already
// rejects any claim whose module/facet segment isn't in the project's
// configured cfg.Modules/cfg.Facets, so by the time buildGroups calls this
// function, every module/facet name it has actually seen is guaranteed to
// already be in preferred — present is always a subset of preferred. This
// is not dead code to delete: it is the deliberate fallback for callers
// that bypass that guarantee (a nil cfg, as newMembershipPredicates'
// doc comment notes some defensive paths allow; a pre-lint or malformed
// catalog; a future caller of this helper that doesn't go through the
// lint-gated flow). Do not remove it on the assumption it can never run —
// it can, just not from today's lint-clean render path.
func orderedNames(preferred []string, present map[string]bool) []string {
	if len(present) == 0 {
		return nil
	}

	used := make(map[string]bool, len(present))
	out := make([]string, 0, len(present))
	for _, p := range preferred {
		if present[p] && !used[p] {
			out = append(out, p)
			used[p] = true
		}
	}

	var rest []string
	for name := range present {
		if !used[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)

	return append(out, rest...)
}

// orderClaims delegates to model.OrderClaims — see that function's doc
// comment for the full algorithm description. Kept as a thin
// package-local wrapper (rather than calling model.OrderClaims directly
// at newGroup's one call site) so this package's existing
// TestOrderClaims_* tests keep exercising the same package-local name.
func orderClaims(claims []model.Claim) []model.Claim {
	return model.OrderClaims(claims)
}

// newGroup builds one Group, pulling each claim's already-rendered HTML out
// of renderedByID (keyed by claim ID) so claims are rendered exactly once
// regardless of how many places reference them. It also injects a section
// heading (sectionHeadingHTML) ahead of the first claim of each run of
// consecutive, same-Section claims — see sectionHeadingHTML's doc comment
// for the exact detection rule. overviewHTML, if non-empty, is the calling
// module's already-rendered overview-facet claims (see renderOverviewHTML)
// and is prepended ahead of any section heading — a module-level
// orientation note isn't its own tab, so buildGroups renders it once per
// module and injects the same HTML into every one of that module's facet
// groups.
func newGroup(module, facet string, claims []model.Claim, renderedByID map[string]template.HTML, overviewHTML []template.HTML) Group {
	claims = orderClaims(claims)
	htmls := make([]template.HTML, 0, len(claims)+len(overviewHTML))
	htmls = append(htmls, overviewHTML...)
	allLocked := len(claims) > 0
	prevSection := ""
	for _, c := range claims {
		if c.Status != model.StatusLocked {
			allLocked = false
		}
		if c.Section != "" && c.Section != prevSection {
			htmls = append(htmls, sectionHeadingHTML(c.Section))
			prevSection = c.Section
		}
		htmls = append(htmls, renderedByID[c.ID])
	}

	id := module
	if facet != "" {
		id = module + "-" + facet
	}

	tabSource := facet
	if tabSource == "" {
		tabSource = module
	}

	return Group{
		Module:      module,
		Facet:       facet,
		ID:          slugify(id),
		Claims:      htmls,
		AllLocked:   allLocked,
		ModuleLabel: displayCase(module),
		TabLabel:    displayCase(tabSource),
	}
}

// sectionHeadingHTML renders a claim's optional model.Claim.Section value as
// a standalone in-content heading marker, injected by newGroup ahead of the
// first claim of each new section run within a facet's claim sequence — the
// round-3 QA finding's fix for a flat, undifferentiated card stream on a
// long document, where the sidebar/sub-nav section identity scrolls out of
// view. It is deliberately a plain, semantic heading element rather than
// reusing the reference docs stylesheet's field-level .lbl class (a close visual
// cousin — mono, uppercase, letter-spaced, muted-color label) because .lbl
// is sized/spaced to sit inside a single card as a field caption, not to
// read as a break between many cards; section-heading instead gets its own
// rule in style.css so it can carry a top border and larger vertical rhythm
// befitting a document-level section break.
func sectionHeadingHTML(section string) template.HTML {
	return template.HTML(`<h4 class="section-heading">` + template.HTMLEscapeString(section) + `</h4>`)
}

// displayCase renders a raw module/facet value (e.g. "token-ledger" or
// "token_ledger") as a human-readable nav label ("Token Ledger"): '-' and
// '_' become spaces, and each resulting word is capitalized. It is a
// display-only transform — Group.Module/Facet and Group.ID (used for
// hashes/element ids) are untouched.
//
// The implementation itself moved to components.DisplayCase when the edges
// footer and every partial's claim heading started deriving readable labels
// from a claim id's segments (components.ClaimLabel). components cannot
// import render — render imports components — so the single copy has to live
// there, and a second copy here would let a card's "Contract › Retry Policy"
// drift away from the nav entry naming that same facet. This wrapper stays so
// render's own call sites read exactly as they always did.
func displayCase(s string) string {
	return components.DisplayCase(s)
}

// slugify lowercases s and collapses every run of characters outside
// [a-z0-9] into a single '-', trimming any leading/trailing '-'. It keeps
// Group.ID safe to use verbatim as an HTML id and a URL hash fragment
// regardless of what characters a project's module/facet names contain.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// stringSet builds a lookup set from ss for O(1) membership checks.
func stringSet(ss []string) map[string]bool {
	if len(ss) == 0 {
		return nil
	}
	set := make(map[string]bool, len(ss))
	for _, x := range ss {
		set[x] = true
	}
	return set
}
