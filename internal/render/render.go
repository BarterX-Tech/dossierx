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
	"errors"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"
	"time"

	"html"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
	"github.com/BarterX-Tech/dossierx/internal/visibility"
)

// The graph and viewer-runtime paths are named individually rather than embedding the
// whole directory: //go:embed resolves at COMPILE time, so a directive
// naming a file that does not exist fails "go build" loudly. Embedding
// viewer/template/* and reading the graph files at render time would turn
// exactly the same mistake — a client file deleted, renamed or never
// written — into a silently empty pane.
//
//go:embed viewer/template/shell.html viewer/template/style.css viewer/template/system-record.js viewer/template/viewer-runtime.js viewer/template/graph-core.js viewer/template/graph-ui.js viewer/template/graph.css viewer/template/fonts/inter-latin-wght.woff2 viewer/template/fonts/source-serif-4-latin-opsz-wght.woff2 viewer/template/fonts/ibm-plex-mono-latin-400.woff2 viewer/template/fonts/ibm-plex-mono-latin-500.woff2 viewer/template/fonts/ibm-plex-mono-latin-600.woff2
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
// .GeneratedAt, see buildShellStaticData) always agree — a reviewer comparing the
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

	// GeneratedAt is the same render timestamp stamped into generatedHeader's
	// leading HTML comment, emitted here as RFC3339 (a machine-readable
	// instant, not a human sentence) into the freshness footer's
	// data-generated-at attribute. reference-rules.md R10.4 requires the
	// elapsed phrase a reviewer actually reads to be computed in the
	// browser, never baked into the generated HTML; system-record.js's
	// enhanceTimestamp is the sole consumer that turns this into "Updated
	// N hours ago" / "Live".
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
	// ConformanceStatusGuardJS is an engine-owned fetch freshness guard emitted
	// only for viewers with structured conformance. It intentionally is not
	// part of ViewerRuntimeJS so no-feature viewer bytes remain unchanged.
	ConformanceStatusGuardJS template.JS

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

	// SoftMount is true when the corpus is large enough that claim cards should
	// ship inside inert <template class="dossierx-surface-template"> nodes and
	// be cloned into a host on first visit (see softMountClaimThreshold). Small
	// corpora keep the historical eager DOM so print
	// probes, and getElementById witnesses see the same box tree they always
	// have. The client threshold in viewer-runtime.js must stay in lockstep.
	SoftMount bool

	// Tracks is the project's declared cross-cutting tracks, one section each,
	// rendered after every module section and listed after every module in the
	// sidebar. NIL FOR A PROJECT THAT DECLARES NONE, and shell.html guards
	// every byte of track markup on that — a corpus with no tracks must render
	// exactly as it did before the axis existed. See track_view.go.
	Tracks []TrackSection

	// Constitution is the project roof, pinned above Modules. Always present
	// as a nav target; Present is false when the file is absent (NIT-11).
	Constitution ConstitutionView
}

// ConstitutionView is the thin A1/A2 roof surface: The file | Project claims.
type ConstitutionView struct {
	Present       bool
	Status        string
	Words         int
	WordCap       int
	FileHTML      template.HTML
	ProjectClaims []template.HTML
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
	// ClaimCount and LockedCount are catalog facts for this facet. The
	// viewer header reads the module-level sums, not live DOM cards.
	ClaimCount  int
	LockedCount int
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
	// Facets are this module's peer tabs in engine-fixed order:
	// Manifest | Contract | Internals. Always three — empty tabs stay
	// peers so Manifest is never a banner above the other two.
	Facets []Group
	// FirstFacetID is the Contract section's ID — the section that renders
	// visible-by-default when this module's sec-tab is chosen, whether by
	// click or by a bare "#module" hash with no facet suffix.
	FirstFacetID string
	// HasSubNav is true when the module has more than one peer tab. With
	// engine-fixed Manifest | Contract | Internals that is always true.
	HasSubNav bool
	// AllLocked is true only when every facet in Facets has AllLocked ==
	// true (which itself requires every claim within that facet to be
	// locked). It drives the same optional lock-indicator suffix on the
	// module-level nav label that Group.AllLocked drives per facet.
	AllLocked bool
	// ClaimCount, LockedCount and FacetCount are stamped onto the module
	// <section> so the header metric does not depend on mounted claim cards.
	ClaimCount  int
	LockedCount int
	FacetCount  int
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
		out[i].FirstFacetID = defaultPeerTabID(out[i].Facets)

		claimCount, lockedCount := 0, 0
		for _, f := range out[i].Facets {
			claimCount += f.ClaimCount
			lockedCount += f.LockedCount
		}
		out[i].AllLocked = claimCount > 0 && lockedCount == claimCount
		out[i].ClaimCount = claimCount
		out[i].LockedCount = lockedCount
		out[i].FacetCount = len(out[i].Facets)
	}

	return out
}

// defaultPeerTabID picks the section that should be visible when a module
// is chosen: always the Contract tab. The strip order is Manifest |
// Contract | Internals, but a module is read through its contract first,
// so Contract opens by default even when it is empty. A module with no
// Contract tab (the ungrouped bucket) falls back to its first tab.
func defaultPeerTabID(facets []Group) string {
	if len(facets) == 0 {
		return ""
	}
	for _, f := range facets {
		if f.Facet == config.FacetContract {
			return f.ID
		}
	}
	return facets[0].ID
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
// template itself) against cfg.Viewer.TemplateOverrides; renderClaimsWithBudget
// turns each catalog claim into HTML via those partials; buildShellStaticData assembles
// the resulting shellData (title/eyebrow/groups) ready for shell.Execute.
// Render itself is left as the sequencing of those three calls plus the
// final template execution, so a future fourth input or grouping level only
// has to touch the stage it belongs to.
func Render(cat *catalog.Catalog, cfg *config.Config) (string, error) {
	return renderAt(cat, cfg, time.Now().UTC())
}

func renderAt(cat *catalog.Catalog, cfg *config.Config, generatedAt time.Time) (string, error) {
	return renderBoundedAt(cat, cfg, generatedAt, 0)
}

// RenderBounded caps the generated viewer while preserving the shared renderer.
func RenderBounded(cat *catalog.Catalog, cfg *config.Config, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		return "", fmt.Errorf("render: max bytes must be positive")
	}
	return renderBoundedAt(cat, cfg, time.Now().UTC(), maxBytes)
}

func renderBoundedAt(cat *catalog.Catalog, cfg *config.Config, generatedAt time.Time, maxBytes int) (string, error) {
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
	attachEdgesOverride(tmpl.partials, buildImplinkLookup(cfg), codeLinksGated(cfg), buildDependedByLookup(cat), buildTargetStatusLookup(cat))
	// Rebind mockup.html's "mockupHTML" func with the project's
	// mockup_modules allowlist so its defense-in-depth gate (DX-AUD-08) can
	// verify module membership; the default binding always escapes.
	attachMockupOverride(tmpl.partials, cfg)

	header := generatedHeader(generatedAt)
	if maxBytes > 0 && len(header) >= maxBytes {
		return "", viewerCapacityError(maxBytes)
	}
	inputs := shellInputs{
		cat:                      cat,
		cfg:                      cfg,
		css:                      viewerCSSWithConformance(tmpl.css, cat.Conformance),
		graphCSS:                 tmpl.graphCSS,
		graphCoreJS:              tmpl.graphCore,
		graphUIJS:                tmpl.graphUI,
		systemRecordJS:           tmpl.systemRecord,
		viewerRuntimeJS:          tmpl.viewerRuntime,
		conformanceStatusGuardJS: statusFetchGuardWithConformance(cat.Conformance),
		generatedAt:              generatedAt,
	}

	var data any
	if tmpl.shellOverridden {
		var memoryBudget *renderByteBudget
		if maxBytes > 0 {
			// loadTemplates has already loaded the engine's fixed embedded assets;
			// their bounded size is constant overhead, not corpus projection data.
			// This guard applies only to lazily requested, corpus-sized values.
			memoryBudget = &renderByteBudget{remaining: maxBoundedRenderIntermediateBytes, exceeded: ErrIntermediateCapacityExceeded}
		}
		data = newLazyShellData(inputs, tmpl.partials, memoryBudget)
	} else {
		var outputBudget *renderByteBudget
		if maxBytes > 0 {
			// The embedded shell emits every dynamic projection. Charging them
			// against the output budget is therefore exact lower-bound containment.
			outputBudget = &renderByteBudget{remaining: maxBytes - len(header), exceeded: conformance.ErrCapacityExceeded}
		}
		eager, err := buildEagerShellData(inputs, tmpl.partials, outputBudget)
		if err != nil {
			if errors.Is(err, conformance.ErrCapacityExceeded) {
				return "", viewerCapacityError(maxBytes)
			}
			return "", err
		}
		data = eager
	}

	var out bytes.Buffer
	var dst io.Writer = &out
	if maxBytes > 0 {
		dst = &capacityWriter{Buffer: &out, remaining: maxBytes - len(header)}
	}
	if err := tmpl.shell.Execute(dst, data); err != nil {
		if errors.Is(err, conformance.ErrCapacityExceeded) {
			return "", viewerCapacityError(maxBytes)
		}
		if errors.Is(err, ErrIntermediateCapacityExceeded) {
			return "", renderIntermediateCapacityError()
		}
		return "", fmt.Errorf("render: execute shell template: %w", err)
	}

	return header + out.String(), nil
}

func viewerCapacityError(maxBytes int) error {
	return fmt.Errorf("%w: viewer requires more than %d bytes", conformance.ErrCapacityExceeded, maxBytes)
}

const maxBoundedRenderIntermediateBytes = 128 << 20

// ErrIntermediateCapacityExceeded is distinct from the output-capacity error:
// it means pre-shell retained data crossed the renderer's memory-safety guard,
// not that the selected shell would necessarily have emitted too many bytes.
var ErrIntermediateCapacityExceeded = errors.New("render intermediate memory capacity exceeded")

func renderIntermediateCapacityError() error {
	return fmt.Errorf("%w: retained render data exceeds the %d-byte safety limit", ErrIntermediateCapacityExceeded, maxBoundedRenderIntermediateBytes)
}

// renderByteBudget caps bytes retained by pre-shell render stages. It is
// deliberately shared: a per-claim cap would still allow an unbounded number
// of small claims to accumulate before the final capacityWriter sees them.
type renderByteBudget struct {
	remaining int
	exceeded  error
}

func (b *renderByteBudget) consume(n int) error {
	if b == nil {
		return nil
	}
	if n < 0 || n > b.remaining {
		if b.exceeded != nil {
			return b.exceeded
		}
		return ErrIntermediateCapacityExceeded
	}
	b.remaining -= n
	return nil
}

// budgetBuffer checks the shared budget before bytes.Buffer grows. Template
// execution can therefore never allocate a claim or build-order fragment past
// the remaining honest viewer-output budget.
type budgetBuffer struct {
	bytes.Buffer
	budget *renderByteBudget
}

func (b *budgetBuffer) Write(p []byte) (int, error) {
	if err := b.budget.consume(len(p)); err != nil {
		return 0, err
	}
	return b.Buffer.Write(p)
}

func (b *budgetBuffer) WriteString(s string) (int, error) {
	if err := b.budget.consume(len(s)); err != nil {
		return 0, err
	}
	return b.Buffer.WriteString(s)
}

type capacityWriter struct {
	Buffer    *bytes.Buffer
	remaining int
}

func (w *capacityWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, conformance.ErrCapacityExceeded
	}
	n, err := w.Buffer.Write(p)
	w.remaining -= n
	return n, err
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
	// shellOverridden selects the runtime-lazy data facade. The embedded shell
	// has a fixed, audited projection contract; a project shell may reference
	// any subset and must pay only for fields its executed branches request.
	shellOverridden bool

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

	css, cssOverridden, err := components.OverrideFile(overrideDir, styleFileName)
	if err != nil {
		return loadedTemplates{}, fmt.Errorf("render: load %s override: %w", styleFileName, err)
	}
	if !cssOverridden {
		css, err = shellFS.ReadFile(styleTemplatePath)
		if err != nil {
			return loadedTemplates{}, fmt.Errorf("render: load default stylesheet: %w", err)
		}
		faces, err := engineFontFaceCSS()
		if err != nil {
			return loadedTemplates{}, err
		}
		css = append(faces, css...)
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

	return loadedTemplates{
		partials:        partials,
		css:             css,
		shell:           shell,
		shellOverridden: shellOverridden,
		graphCore:       graphCore,
		graphUI:         graphUI,
		graphCSS:        graphCSS,
		systemRecord:    systemRecord,
		viewerRuntime:   viewerRuntime,
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

// renderClaimsWithBudget executes each cat.Claims entry through its layout's partial
// template in partials (as loaded by loadTemplates), returning a
// claim-ID-keyed lookup (renderedByID, consumed by buildGroups/newGroup) so
// a claim is rendered exactly once regardless of how many places reference
// it afterwards. shell.html has no top-level "all claims, unordered" view
// (it renders exclusively via ModuleGroups' nested Facets[].Claims), so
// renderClaimsWithBudget does not also keep a flat catalog-order slice around for it.
func renderClaimsWithBudget(cat *catalog.Catalog, partials map[model.Layout]*template.Template, conformanceResults map[string]conformance.Result, budget *renderByteBudget) (map[string]template.HTML, error) {
	renderedByID := make(map[string]template.HTML, len(cat.Claims))
	for _, c := range cat.Claims {
		tmpl, ok := partials[c.Layout]
		if !ok {
			return nil, fmt.Errorf("render: claim %q has unsupported layout %q", c.ID, c.Layout)
		}
		buf := budgetBuffer{budget: budget}
		if err := tmpl.Execute(&buf, c); err != nil {
			return nil, fmt.Errorf("render: claim %q: %w", c.ID, err)
		}
		// Conformance is engine-owned generated evidence, not a replaceable
		// presentation partial. Inserting it inside the claim root keeps the
		// projection visible when a project overrides that entire partial and
		// keeps it inside claim collapse.
		if result, ok := conformanceResults[c.ID]; ok {
			rendered := insertEngineBlockBeforeClose(buf.String(), string(components.ConformanceHTML(result, cat.ConformanceSnapshot)))
			renderedByID[c.ID] = template.HTML(rendered)
			continue
		}
		renderedByID[c.ID] = template.HTML(buf.String())
	}
	return renderedByID, nil
}

// shellInputs is buildShellStaticData's single argument: everything Render has
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
	// payload for cat, already stamped and encoded by graphPayloadJSONWithBudget.
	graphCSS                 []byte
	graphCoreJS              []byte
	graphUIJS                []byte
	systemRecordJS           []byte
	viewerRuntimeJS          []byte
	conformanceStatusGuardJS []byte
	graphPayload             template.JS

	renderedByID map[string]template.HTML
	generatedAt  time.Time
}

// buildShellStaticData assembles the shellData passed to shell.Execute: cfg's
// title/eyebrow (with the same fallbacks Render has always applied
// when cfg is nil or leaves a field blank) and the module/facet groups
// computed from in.cat via buildGroups/buildModuleGroups, combined with the
// css/renderedByID inputs loadTemplates and renderClaimsWithBudget already produced.
//
// The four graph fields are typed on the way OUT, not on the way in: see
// shellData.GraphCSS and the block of comments there for why plain strings
// at those injection sites fail silently.
func buildShellStaticData(in shellInputs) shellData {
	cfg := in.cfg

	title := "dossierx viewer"
	eyebrow := ""
	if cfg != nil {
		if strings.TrimSpace(cfg.Title) != "" {
			title = cfg.Title
		}
		eyebrow = strings.TrimSpace(cfg.Eyebrow)
	}

	claimCount := 0
	if in.cat != nil {
		claimCount = len(in.cat.Claims)
	}
	return shellData{
		Title:                    title,
		Eyebrow:                  eyebrow,
		CSS:                      template.CSS(in.css),
		GeneratedAt:              in.generatedAt.UTC().Format(time.RFC3339),
		GraphCSS:                 template.CSS(in.graphCSS),
		GraphPayload:             in.graphPayload,
		GraphCoreJS:              template.JS(in.graphCoreJS),
		GraphUIJS:                template.JS(in.graphUIJS),
		SystemRecordJS:           template.JS(in.systemRecordJS),
		ViewerRuntimeJS:          template.JS(in.viewerRuntimeJS),
		ConformanceStatusGuardJS: template.JS(in.conformanceStatusGuardJS),
		ModuleGroups:             nil,
		SoftMount:                claimCount >= softMountClaimThreshold,
		// Built from the SAME renderedByID the module groups read, so a claim
		// a track owns is rendered exactly once no matter how many sections
		// point at it — the property newGroup's own lookup exists to hold.
		Tracks:       nil,
		Constitution: buildConstitutionView(in.cat, cfg),
	}
}

func buildConstitutionView(cat *catalog.Catalog, cfg *config.Config) ConstitutionView {
	view := ConstitutionView{WordCap: constitution.WordCap}
	// The Project claims tab lists the store whether or not the roof file
	// exists yet: the two are independent inputs, and a project that authored
	// project claims before writing its constitution (serve renders it; the
	// gate only stops check and lock) must not read "No project claims."
	if cat != nil {
		for _, c := range cat.Claims {
			if !c.IsProjectClaim() {
				continue
			}
			view.ProjectClaims = append(view.ProjectClaims, template.HTML(
				`<article class="project-claim"><h4>`+html.EscapeString(c.ID)+`</h4><p>`+html.EscapeString(c.Body)+`</p></article>`))
		}
	}
	if cfg == nil {
		return view
	}
	f, err := constitution.LoadOptional(cfg.ConstitutionPath())
	if err != nil || f == nil {
		return view
	}
	d := constitution.NewDigest(cfg.ConstitutionPath(), f)
	view.Present = true
	view.Status = d.Status
	view.Words = d.Words
	view.WordCap = d.WordCap
	var b strings.Builder
	writeSection := func(title string, entries []constitution.Entry, section string) {
		if len(entries) == 0 {
			return
		}
		b.WriteString("<h3>")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</h3>")
		for _, e := range entries {
			b.WriteString(`<article class="constitution-entry" id="`)
			b.WriteString(html.EscapeString("constitution-" + section + "-" + e.Slug))
			b.WriteString(`"><h4>`)
			if e.Title != "" {
				b.WriteString(html.EscapeString(e.Title))
			} else {
				b.WriteString(html.EscapeString(e.Slug))
			}
			b.WriteString(`</h4><p>`)
			b.WriteString(html.EscapeString(e.Body))
			b.WriteString(`</p></article>`)
		}
	}
	writeSection("Invariants", f.Invariants, constitution.SectionInvariants)
	writeSection("Glossary", f.Glossary, constitution.SectionGlossary)
	writeSection("Decisions", f.Decisions, constitution.SectionDecisions)
	view.FileHTML = template.HTML(b.String())
	return view
}

// softMountClaimThreshold is the corpus size at which shell.html starts
// emitting deferred surface templates. Keep in lockstep with
// SOFT_MOUNT_MIN_CLAIMS in viewer-runtime.js.
const softMountClaimThreshold = 80

// buildGroups computes the module -> facet grouping described in NAV_SPEC.
// It never panics on an empty or nil catalog (returns nil groups) and never
// drops a claim: any claim whose module and/or facet is empty or not
// recognized by cfg lands in a single catch-all ungroupedModuleName group
// instead of being silently discarded.
func buildGroups(cat *catalog.Catalog, cfg *config.Config, renderedByID map[string]template.HTML) []Group {
	if cat == nil || len(cat.Claims) == 0 {
		return nil
	}

	var declaredModules []string
	if cfg != nil {
		declaredModules = cfg.Modules
	}
	declaredFacets := visibility.ViewerTabs()
	knownModule, knownFacet := newMembershipPredicates(declaredModules, config.EngineFacets())

	type groupKey struct{ module, facet string }
	claimsByKey := map[groupKey][]model.Claim{}
	moduleSeen := map[string]bool{}
	var ungrouped []model.Claim

	for _, c := range cat.Claims {
		// A project claim (NIT-25) has no module and no facet by design, not
		// by omission: it is rendered under the constitution's "Project
		// claims" tab (buildConstitutionView), so it is not dropped here and
		// it must not become an "ungrouped" module either — the sidebar's
		// Modules group would then count and list a pseudo-module directly
		// under the pin that says PROJECT — NOT A MODULE.
		if c.IsProjectClaim() {
			continue
		}
		if !knownModule(c.Module) || !knownFacet(c.Facet) {
			ungrouped = append(ungrouped, c)
			continue
		}
		k := groupKey{c.Module, c.Facet}
		claimsByKey[k] = append(claimsByKey[k], c)
		moduleSeen[c.Module] = true
	}

	var groups []Group
	for _, m := range orderedNames(declaredModules, moduleSeen) {
		// Every seen module gets the three peer tabs. Manifest is empty until
		// NIT-7 fills it; do not inject retired overview notes onto any tab.
		for _, f := range declaredFacets {
			var groupClaims []model.Claim
			if f != visibility.ViewerTabManifest {
				groupClaims = claimsByKey[groupKey{m, f}]
			}
			groups = append(groups, newGroup(m, f, groupClaims, renderedByID))
		}
	}

	if len(ungrouped) > 0 {
		groups = append(groups, newGroup(ungroupedModuleName, "", ungrouped, renderedByID))
	}

	markFirstInModule(groups)

	return groups
}

// stripDuplicateClaimIDs returns one already-rendered claim with every element
// id it carries removed, for use as a NON-CANONICAL copy: the same claim is
// also rendered somewhere else on the page, and that copy keeps the ids.
//
// Tracks render the claims they own inline while their modules keep
// guaranteeing them. A claim id may appear only once in a valid document.
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
// clicking one is taken to the same evidence in the claim's own module.
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
// regardless of how many places reference them. A claim's Section remains
// part of the ordering model, but the Reading View does not repeat that
// metadata as a visible heading between cards.
func newGroup(module, facet string, claims []model.Claim, renderedByID map[string]template.HTML) Group {
	claims = orderClaims(claims)
	htmls := make([]template.HTML, 0, len(claims)+1)
	if facet == visibility.ViewerTabManifest && len(claims) == 0 {
		htmls = append(htmls, template.HTML(`<p class="claims-empty">No module manifest yet.</p>`))
	}
	allLocked := len(claims) > 0
	lockedCount := 0
	for _, c := range claims {
		if c.Status == model.StatusLocked {
			lockedCount++
		} else {
			allLocked = false
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
		ClaimCount:  len(claims),
		LockedCount: lockedCount,
		ModuleLabel: displayCase(module),
		TabLabel:    displayCase(tabSource),
	}
}

// insertEngineBlockBeforeClose places engine-owned HTML inside the claim's
// root element so collapse wrapping and project layout overrides cannot leave
// it as a sibling. The last </section> is the claim root for every default
// layout; </article> covers a project override that uses a different tag.
func insertEngineBlockBeforeClose(host, block string) string {
	const footerSlot = "<!--dossierx-claim-footer-slot-->"
	if i := strings.LastIndex(host, footerSlot); i >= 0 {
		return host[:i] + block + host[i+len(footerSlot):]
	}
	for _, close := range []string{"</section>", "</article>"} {
		if i := strings.LastIndex(host, close); i >= 0 {
			return host[:i] + block + host[i:]
		}
	}
	return host + block
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
