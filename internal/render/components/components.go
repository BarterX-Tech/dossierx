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
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

//go:embed card.html table.html list.html steps.html tree.html banner.html mockup.html comments.html
var FS embed.FS

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
//
// "markdown" IS THE NO-IMAGES RENDERER AND MUST STAY THAT WAY. It is the name
// an arbitrary project override partial reaches for, it is the name
// comments.html is parsed with, and it is the name a future engine template
// will reach for by habit. Binding the image-permitting entry point to it would
// hand the capability to all three by default; binding it to markdown.Render
// means the worst a forgotten opt-in can cost is a missing diagram in a claim,
// which the human reading the page sees at once.
//
// "claimMarkdown" is the opt-in, and it takes the CLAIM rather than the text
// alone, because a claim's images are addressed relative to that claim — see
// claimMarkdown and ClaimAssetURLPrefix.
var funcMap = template.FuncMap{
	"rowKeys":       rowKeys,
	"markdown":      markdown.Render,
	"claimMarkdown": claimMarkdown,
	"cell":          cell,
	"edges":         edgesHTML,
	"inc":           inc,
	"pillClass":     pillClass,
	"statusLabel":   StatusLabel,
	"statusIcon":    StatusIconHTML,
	"colClass":      colClass,
	"mockupHTML":    mockupHTML,
	"claimLabel":    ClaimLabel,
	"commentChip":   CommentChipHTML,
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
// "commentChip" is not added here for the same reason: comments.html never
// calls it, and widening this restricted map back toward the full funcMap is
// exactly how the initialization cycle above comes back.
//
// Unlike the per-layout partials, this panel is not wired
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
// state out across every rendered copy of the same claim) plus its threads
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

// StatusLabel is the sentence-case word a claim status pill shows.
func StatusLabel(status model.Status, reviewPending bool) string {
	if status == model.StatusLocked && reviewPending {
		return "Review pending"
	}
	switch status {
	case model.StatusLocked:
		return "Locked"
	case model.StatusDraft:
		return "Draft"
	default:
		if status == "" {
			return ""
		}
		s := string(status)
		return strings.ToUpper(s[:1]) + s[1:]
	}
}

// StatusIconHTML returns the padlock glyph that rides inside every claim
// status pill. The components board (section F, "Claim status chip") is
// explicit that the chip is a closed set of two shapes, not an icon plus a
// bare-word fallback: "Always a padlock and a word — the padlock is closed
// when an approval is on record and open when it is not, so the two states
// differ in shape as well as in colour" and "The chip never appears without
// its icon. A word alone reads as a label; the padlock is what makes it a
// state." A locked claim (pillClass "ps") and a locked-but-review_pending one
// ("pw") both have an approval on record — review_pending is a flag on an
// already-locked claim, not a withdrawal of its lock — so both get the closed
// #dx-icon-lock glyph; only a draft claim ("pv") has never been approved, and
// gets the open #dx-icon-lock-open glyph instead.
//
// docs/design/screens/07a-claim-draft-not-yet-approved.md's board (node
// 4BJ-0) draws the DRAFT chip with the SAME open padlock this returns for
// StatusDraft — a pixel comparison against the components board confirms the
// shackle is lifted, not closed, on both. There is no Paper defect here: the
// open-padlock choice below agrees with 07a as well as with section F. The
// actual inaccuracy is prose, not a board: docs/design/LANES.md's L3
// ownership section says "the DRAFT form carries no padlock," which reads
// narrower than section F's own rule (a closed set of two shapes, never
// "icon or nothing") and is being corrected there, not here.
func StatusIconHTML(status model.Status, reviewPending bool) template.HTML {
	if status == model.StatusLocked {
		return template.HTML(`<svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-lock"/></svg>`)
	}
	if status == model.StatusDraft {
		return template.HTML(`<svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-lock-open"/></svg>`)
	}
	return ""
}

// edgesHTML renders the edge/metadata footer shared by every non-banner
// component: rests_on, migrated_from, and a
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
// — the default, parse-time "edges" funcMap binding can
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
	label := StatusLabel(st.Status, st.ReviewPending)
	return ` <span class="pill ` + pillClass(st.Status, st.ReviewPending) + `">` + html.EscapeString(label) + `</span>`
}

// EdgesHTMLWithLinks renders the shared evidence-and-relationships footer —
// docs/design/screens/05-claim-one-expansion-at-a-time.md's "one strip, four
// doors" redesign (R09.1/R-F.1) — for the two doors this package owns:
// RELATIONSHIPS (R09.4's fixed directions, DEPENDS ON / DEPENDED ON BY —
// GOVERNED BY retired with the edge, NIT-29 — plus the
// migrated_from/review_pending/implemented-in
// facts that ride along after them) and SOURCES, split out into its own peer
// disclosure per R09.5. The other two doors the board names — readiness and
// implementation checks — are rendered by sibling components
// (viewer-runtime.js's renderClaimReadiness and conformance_view.go's
// ConformanceHTML) that this package does not own; see docs/design/LANES.md's
// L4 section for the ownership split. The <div class="claim-footer"> this
// function opens is the shared flex strip those two doors slot into:
// `.claim-footer > .claim-readiness` (a layout-integration rule this
// package's style.css section adds, not a rewrite of readiness's own rule)
// gives the readiness box the strip's full-width row once
// viewer-runtime.js's renderClaimReadiness inserts it before `.claim-links`
// (an unchanged call site — see that function) — because
// `card.querySelector('.claim-links')`'s `.parentNode` becomes this <div>
// the moment it exists. Implementation checks (`.claim-conformance`) render
// as their own disclosure immediately after this <div> rather than inside
// it — internal/render's insertEngineBlockBeforeClose (L7-owned) splices it
// in after this function's whole return value, so it cannot become a flex
// child of a container this function has already closed. That is a real gap
// against R-F.1's single row, left standing as a known gap rather than
// silently worked around here.
//
// Exported (unlike edgesHTML) so internal/render can bind it into a
// per-render "edges" template-func override; see that package's
// attachEdgesOverride for why a template func override, rather than a
// second template field, is how this data reaches the existing per-layout
// partials without editing any of them.
//
// Each door is its own native <details name="claim-footer-{id}">. The shared
// `name` is what gives R09.2 ("exactly one expansion open at a time") to
// relationships and sources for free, with no JS: the HTML disclosure-group
// mechanism closes every other <details> sharing a name the instant one
// opens, regardless of DOM position, so the day a sibling lane's own
// <details> (readiness, once it is one; `.claim-conformance` already is)
// carries the same `name="claim-footer-<id>"` attribute, it joins the same
// one-at-a-time group at zero cost. The name is per-claim (the id itself) so
// two different claim cards on one page never cross-close each other.
//
// THIRD RETRY FIX (verifier item 2): a door's <details> now contains ONLY its
// <summary> chip — its `.claim-footer-panel` is a plain <div> written as
// that <details>'s next SIBLING in this function's output, not its child.
// This is what lets .claim-footer lay the chip out as an ordinary flex item
// (always in the strip row, open or closed) while the panel independently
// takes the full row on its own line once open — see style.css's
// `.claim-links[open] + .claim-links-panel` rule and the doc comment beside
// the `.claim-links, .claim-sources` reset for the measured browser bug
// (`display: contents` on a <details>) that ruled out the alternative of
// promoting the panel out from INSIDE the <details>. The disclosure-group
// mechanics above (shared `name`, native `open`) are entirely unaffected:
// they key off the <details> elements, which are unchanged.
//
// Two signals still write the bare ` open` attribute server-side on the
// relationships door — any linked file Drifted, or the claim locked +
// review_pending — so the two states a reader must not miss are never
// hidden behind a click (unchanged from the pre-redesign digest's rule).
// The comment chip is the footer's final control, outside every details door.
// Claims no longer collapse, so this placement stays reachable in the default
// reading view and its live comment APIs retain the canonical claim ID.
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
	return EdgesHTMLWithCodeLinks(c, files, false, dependedBy, targetStatuses)
}

// EdgesHTMLWithCodeLinks is EdgesHTMLWithLinks for a project that check's
// code-link gate holds to account (linksGated: `source_dirs` is set). It
// adds the two rows the gate refuses on, in the same "implemented in" slot
// of the relationships panel: a locked schema/behavior/api/verification
// claim with no linked file at all reads "not linked to code", and a
// stepped claim whose dossierx-step tags miss a step reads "steps linked:
// k of N" naming the missing indexes. Both count as a relationship so the
// chip never says "No relationships" over a gap `dossierx check` exits 1
// on. Neither row exists for an ungated project: with no `source_dirs`
// there is no gate, and a claim nobody linked is not a claim in breach.
func EdgesHTMLWithCodeLinks(c model.Claim, files []implink.ViewFile, linksGated bool, dependedBy []string, targetStatuses map[string]TargetStatus) template.HTML {
	links := 0

	// The R09.4 direction blocks, built independently of the "extra" facts
	// below so their fixed order — RESTS ON, DEPENDED ON BY — never depends
	// on which edge kinds a given claim happens to carry. GOVERNED BY was the
	// first of the three until the edge retired (NIT-29); DEPENDS ON became
	// RESTS ON when the stated-absence form joined it (NIT-24).
	var restsOnBody strings.Builder
	restsOnHas := false
	if c.RestsOn.None {
		restsOnHas = true
		restsOnBody.WriteString(`<li class="claim-rests-on rests-on-none claim-relationship-none">none`)
		if c.RestsOn.Reason != "" {
			restsOnBody.WriteString(`<span class="claim-rests-on-reason"> — `)
			restsOnBody.WriteString(string(markdown.RenderInline(c.RestsOn.Reason)))
			restsOnBody.WriteString(`</span>`)
		}
		restsOnBody.WriteString(`</li>`)
	} else if len(c.RestsOn.IDs) > 0 {
		restsOnHas = true
		links += len(c.RestsOn.IDs)
		for _, id := range c.RestsOn.IDs {
			writeRelationshipRow(&restsOnBody, "claim-rests-on", id, c.Module, c.Facet, targetStatuses)
		}
	}

	var dependedOnByBody strings.Builder
	if len(dependedBy) > 0 {
		links += len(dependedBy)
		for _, id := range dependedBy {
			writeRelationshipRow(&dependedOnByBody, "claim-depended-by", id, c.Module, c.Facet, targetStatuses)
		}
	}

	// Facts that do not fit R09.4's three fixed directions (
	// migrated_from, review_pending, implemented-in/drifted) ride after the
	// three direction blocks inside the same relationships panel, in the
	// same hairline-divided row form, rather than inventing a fourth
	// direction the reference rules do not name. review_pending's proper
	// home is the readiness door once L5 lands it (05 §8 item 3); until
	// then it stays here, exactly where a reader could already find it, so
	// nothing regresses to invisible.
	var extra strings.Builder
	if c.MigratedFrom != "" {
		links++
		extra.WriteString(`<li class="claim-migrated claim-relationship-extra">migrated_from: `)
		extra.WriteString(html.EscapeString(c.MigratedFrom))
		extra.WriteString(`</li>`)
	}
	reviewPending := c.Status == model.StatusLocked && c.ReviewPending
	if reviewPending {
		links++
		extra.WriteString(`<li class="claim-review-pending claim-relationship-extra">review_pending</li>`)
	}
	for _, f := range files {
		extra.WriteString(`<li class="claim-implemented-in claim-relationship-extra">implemented in: <code>`)
		extra.WriteString(html.EscapeString(f.File))
		if f.Symbol != "" {
			extra.WriteString(`#`)
			extra.WriteString(html.EscapeString(f.Symbol))
		}
		extra.WriteString(`</code>`)
		if f.Drifted {
			extra.WriteString(` <span class="pill pw">drifted</span>`)
		}
		extra.WriteString(`</li>`)
	}
	if linksGated && implink.Expects(c) {
		if len(files) == 0 {
			links++
			extra.WriteString(`<li class="claim-unlinked claim-relationship-extra">implemented in: <span class="pill pw">not linked to code</span></li>`)
		} else if total := len(c.Steps); total > 0 {
			steps := make([]int, 0, len(files))
			for _, f := range files {
				if f.Step > 0 {
					steps = append(steps, f.Step)
				}
			}
			if covered, missing := implink.StepCoverage(total, steps); covered < total {
				links++
				extra.WriteString(`<li class="claim-partial-link claim-relationship-extra">steps linked: `)
				extra.WriteString(strconv.Itoa(covered))
				extra.WriteString(` of `)
				extra.WriteString(strconv.Itoa(total))
				extra.WriteString(` <span class="pill pw">missing step `)
				for i, n := range missing {
					if i > 0 {
						extra.WriteString(`, `)
					}
					extra.WriteString(strconv.Itoa(n))
				}
				extra.WriteString(`</span></li>`)
			}
		}
	}

	var b strings.Builder

	// RETRY FIX (05 §8 item 5: "An entirely edgeless, sourceless, checkless
	// claim still shows four zeros and the comment count."; fix-list item
	// 10). The strip now renders UNCONDITIONALLY: the previous all-zero gate
	// (`links > 0 || len(files) > 0 || len(c.Sources) > 0`) hid the whole
	// footer on any claim with no countable relationship. R-F.1 fixes
	// the strip's four positions; a reader who has learned "this position is
	// sources" must not find the position itself missing on the next claim,
	// only its count at zero.
	footerName := html.EscapeString("claim-footer-" + c.ID)

	// The relationships door has NO auto-open signal. Paper's placement board
	// states it outright (node Z7-0: "Never opens by default"), and the
	// reading view draws all four footer doors closed. The drifted /
	// review_pending signals that used to force it open contradicted that and
	// were removed; those states are still carried by the claim's status pill
	// and by the readiness door.
	//
	// The deep-link/fragment case stays CSS-only (viewer/template/style.css's
	// `.claim:target .claim-links…` rule) since a URL fragment is never sent
	// to the server. That is reader-initiated navigation, not a default state.

	b.WriteString(`<div class="claim-footer">`)

	// ---- relationships door (R09.4) ----
	//
	// THIRD RETRY FIX (verifier item 2). The panel used to be this <details>'s
	// own child, closed with it inside one `</div></details>` pair. Probed
	// directly: `display: contents` on a <details> — the mechanism a second
	// draft of this fix used to promote the summary chip and the panel into
	// independent flex items of .claim-footer, so the chip stays in the strip
	// row while only the panel wraps onto its own full-width row below —
	// measurably breaks in the tested engine (Chrome/Chromium). Two separate
	// defects, both confirmed by reading getComputedStyle: a CLOSED door's
	// panel still computed `content-visibility: visible` (the native
	// closed-<details> hiding the whole point of `display: contents` here
	// was supposed to leave alone did not fire), and a promoted child's own
	// percentage `width`/`flex-basis` resolved against the wrong containing
	// block (a measured 744px against .claim-footer's own, simultaneously
	// measured, 780px content box — no CSS in this file asks for that
	// number, and it is not a rounding artifact: it reproduced exactly,
	// repeatedly, independent of which flex properties this rule set).
	//
	// The panel is therefore now a genuine SIBLING of this <details>, not its
	// child: a plain, always-present <div> immediately following
	// `</details>`, hidden by default (`.claim-footer-panel { display: none
	// }`) and revealed purely by CSS sibling selectors keyed off this
	// <details>'s `open` attribute or a `:target` inside it — see that rule
	// and the deep-link block below. This sidesteps the display:contents bug
	// entirely: the <details> here is now a NORMAL, always-content-sized
	// (chip-only) box, never asked to promote a child or to hide one
	// natively, and the panel is a NORMAL flex item of .claim-footer from
	// the start, sized with ordinary CSS (no promoted-child percentage
	// resolution involved). The `name`-grouped native "close my sibling
	// <details>" mechanism (R09.2) is unaffected — it keys off the
	// <details> elements themselves, which still exist, still share
	// `name="claim-footer-<id>"`, and still carry the real `open` attribute
	// exactly as before.
	if links == 0 && !restsOnHas && extra.Len() == 0 {
		b.WriteString(`<span class="claim-footer-chip claim-footer-chip--relationships claim-footer-chip--empty"><span class="claim-footer-chip-label">No relationships</span></span>`)
	} else {
		b.WriteString(`<details class="claim-links" name="`)
		b.WriteString(footerName)
		b.WriteString(`"`)
		b.WriteString(`><summary class="claim-footer-chip claim-footer-chip--relationships"><span class="claim-footer-chip-label">`)
		b.WriteString(countSegment(links, "relationship"))
		b.WriteString(`</span>`)
		b.WriteString(claimFooterChevronHTML)
		b.WriteString(`</summary></details>`)
		b.WriteString(`<div class="claim-footer-panel claim-links-panel">`)
		b.WriteString(`<div class="claim-footer-panel-head"><span class="claim-footer-eyebrow">RELATIONSHIPS</span><span class="claim-footer-rule" aria-hidden="true"></span><span class="claim-footer-panel-count">`)
		b.WriteString(strconv.Itoa(links))
		b.WriteString(`</span></div>`)

		// The direction count (2nd param) is 05 §4.10's optional mono count,
		// shown for RESTS ON / DEPENDED ON BY (07a §6 pins "DEPENDS ON · 1",
		// "DEPENDED ON BY · 1"; the label is RESTS ON since NIT-24); a negative
		// count omits the element.
		writeRelationshipDirection(&b, "down", "RESTS ON", len(c.RestsOn.IDs), restsOnBody.String(), restsOnHas)
		writeRelationshipDirection(&b, "right", "DEPENDED ON BY", len(dependedBy), dependedOnByBody.String(), len(dependedBy) > 0)

		if extra.Len() > 0 {
			b.WriteString(`<ul class="claim-edges claim-edges-extra">`)
			b.WriteString(extra.String())
			b.WriteString(`</ul>`)
		}
		b.WriteString(`</div>`)
	}

	// ---- sources door (R09.5) ----
	// RETRY FIX: always rendered now, even at zero sources. 07a §6/§8's
	// "Sources present" state is explicit that the word changes FROM "No
	// sources" TO "N sources" once len(c.Sources) > 0 — meaning the zero
	// state has its own, non-numeral wording, never "0 sources". Per 05 §8
	// item 5 that zero-state chip is also closed, un-openable and
	// --color-faint rather than the ordinary --color-muted+chevron chip —
	// hence claim-footer-chip--empty and the omitted chevron span below.
	if len(c.Sources) == 0 {
		b.WriteString(`<span class="claim-footer-chip claim-footer-chip--sources claim-footer-chip--empty"><span class="claim-footer-chip-label">No sources</span></span>`)
	} else {
		b.WriteString(`<details class="claim-sources" name="`)
		b.WriteString(footerName)
		b.WriteString(`"><summary class="claim-footer-chip claim-footer-chip--sources"><span class="claim-footer-chip-label">`)
		b.WriteString(countSegment(len(c.Sources), "source"))
		b.WriteString(`</span>`)
		b.WriteString(claimFooterChevronHTML)
		b.WriteString(`</summary></details>`)
		b.WriteString(`<div class="claim-footer-panel claim-sources-panel">`)
		b.WriteString(`<div class="claim-footer-panel-head"><span class="claim-footer-eyebrow">SOURCES</span><span class="claim-footer-rule" aria-hidden="true"></span><span class="claim-footer-panel-count">`)
		b.WriteString(strconv.Itoa(len(c.Sources)))
		b.WriteString(`</span></div>`)
		b.WriteString(`<ul class="claim-source-list">`)
		writeSourcesRow(&b, c)
		b.WriteString(`</ul>`)
		b.WriteString(`</div>`)
	}

	b.WriteString(`<!--dossierx-claim-footer-slot-->`)
	b.WriteString(string(CommentChipHTML(c)))
	b.WriteString(`</div>`)

	// The baked-in thread panel follows the whole footer strip (a <div>, so it
	// can't be an <li> inside a <ul>) but stays inside the claim's <section>,
	// since {{edges .}} is the last thing every non-banner partial emits before
	// </section>. It is a SIBLING of the disclosures, never a child: a claim's
	// threads must stay readable without expanding its edges, and — crucially —
	// a claim with comments but no edges at all suppresses the strip above
	// while still rendering its panel here. comments.html auto-escapes its
	// bodies via the shared "markdown" func, so no hand-escaping is needed for
	// the panel. On the (embedded, tested) template this Execute cannot fail; a
	// defensive error path drops the panel rather than corrupt the footer with
	// partial output.
	if len(c.Comments) > 0 {
		var pb strings.Builder
		if err := commentsPanelTmpl.Execute(&pb, newCommentsPanelView(c)); err == nil {
			b.WriteString(pb.String())
		}
	}

	return template.HTML(b.String())
}

// writeRelationshipDirection writes one of R09.4's fixed-order direction
// blocks — DEPENDS ON / DEPENDED ON BY — as its own header (arrow + label +
// optional count) followed by the rows body already built for it. A
// direction with nothing to show (has is false) is omitted entirely rather
// than printed empty.
//
// count is 05 §4.10's "Optional mono count (1, 2) IBM Plex Mono 11px / 14px
// --color-faint" — omitted (no element at all, not a "0") when count is
// negative. DEPENDS ON and DEPENDED ON BY always pass their real row count,
// matching 07a §6's pinned "DEPENDS ON · 1" / "DEPENDED ON BY · 1"; the
// negative sentinel was GOVERNED BY's and is kept for a later count-less
// direction.
//
// arrow is a plain glyph rather than an SVG sprite reference — the frozen
// theme-parity baselines already accept a CSS/glyph chevron for this exact
// footer (see the .claim-footer__chevron border-triangle), and a bare
// Unicode arrow in the same Inter run needs no new icon plumbing.
func writeRelationshipDirection(b *strings.Builder, arrow, label string, count int, rowsHTML string, has bool) {
	if !has {
		return
	}
	glyph := "→"
	switch arrow {
	case "up":
		glyph = "↑"
	case "down":
		glyph = "↓"
	}
	b.WriteString(`<div class="claim-relationship-direction">`)
	b.WriteString(`<div class="claim-relationship-direction-head"><span class="claim-relationship-arrow" aria-hidden="true">`)
	b.WriteString(glyph)
	b.WriteString(`</span><span class="claim-relationship-direction-label">`)
	b.WriteString(label)
	b.WriteString(`</span>`)
	if count >= 0 {
		b.WriteString(`<span class="claim-relationship-direction-count">`)
		b.WriteString(strconv.Itoa(count))
		b.WriteString(`</span>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`<ul class="claim-edges claim-relationship-list">`)
	b.WriteString(rowsHTML)
	b.WriteString(`</ul></div>`)
}

// writeRelationshipRow writes one R09.4 relationship row: a lifecycle dot,
// the shared writeClaimRef anchor (title only — showPrefix=false, see below),
// the target's own `module · facet` meta column, and a right-ranged
// lifecycle badge — 05 §4.10's four columns. Unlike targetPillHTML (which
// still governs the "extra" rests_on-adjacent rows via
// writeIDListItems), a fixed-direction relationship row ALWAYS carries a
// badge when the target's lifecycle is known — a healthy locked target gets
// "LOCKED" rather than nothing, because R-I.2 states the badge is the row's
// lifecycle fact, not an alert. An unknown target (no catalog lookup — the
// default, parse-time "edges" binding) renders no dot, no meta and no badge,
// degrading to the plain link the pre-redesign row already was.
//
// The meta column and the badge are wrapped together in a
// claim-relationship-line2 span, which style.css unwraps with
// `display: contents` at every width down to the 520px tier — so the four
// columns still lay out as direct flex children of the <li> on desktop —
// and turns into a real flex row at 520px, where R-I.2 stacks them together
// as the relationship row's mobile "line two".
func writeRelationshipRow(b *strings.Builder, liClass, targetID, fromModule, fromFacet string, targetStatuses map[string]TargetStatus) {
	st, known := targetStatuses[targetID]
	b.WriteString(`<li class="`)
	b.WriteString(liClass)
	b.WriteString(` claim-relationship">`)
	if known {
		b.WriteString(`<span class="claim-relationship-dot claim-relationship-dot--`)
		b.WriteString(lifecycleModifier(st))
		b.WriteString(`" aria-hidden="true"></span>`)
	}
	writeClaimRef(b, targetID, fromModule, fromFacet, nil, false)
	b.WriteString(`<span class="claim-relationship-line2">`)
	writeRelationshipMeta(b, targetID)
	if known {
		b.WriteString(`<span class="claim-relationship-badge claim-relationship-badge--`)
		b.WriteString(lifecycleModifier(st))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(strings.ToUpper(StatusLabel(st.Status, st.ReviewPending))))
		b.WriteString(`</span>`)
	}
	b.WriteString(`</span>`)
	b.WriteString(`</li>`)
}

// writeRelationshipMeta writes 05 §4.10's "module · facet" meta column for a
// relationship row — the target's OWN module and facet, unconditionally.
// Unlike writeClaimRef's prefix (elided against the reading claim's own
// module/facet, and shown inline before the label), this column always
// shows both segments, because both 05 §4.10 ("Meta Curtainly · Doctrine")
// and 07 §4.10 ("Row meta … text form Module · Facet") measure it
// unqualified — there is no same-module/same-facet special case for this
// column, only for writeClaimRef's own inline prefix on every OTHER edge
// list this footer renders (rests_on-adjacent extras).
//
// An unshaped id (splitClaimID fails) has no module/facet to show and
// writes no meta span at all — the same graceful degradation writeClaimRef's
// own raw-id fallback uses.
func writeRelationshipMeta(b *strings.Builder, targetID string) {
	module, facet, _, ok := splitClaimID(targetID)
	if !ok {
		return
	}
	b.WriteString(`<span class="claim-relationship-meta">`)
	b.WriteString(html.EscapeString(DisplayCase(module) + claimRefModuleSep + DisplayCase(facet)))
	b.WriteString(`</span>`)
}

// lifecycleModifier maps a target's status to the BEM-style modifier suffix
// its dot/badge take: "locked" or "draft" — the two lifecycle hues 05 §4.10
// draws (`--color-locked`, `--color-draft`; tokens.md's G2 mapping makes
// those this engine's `--accent` and `--status-draft`). A locked-but-
// review_pending target still reads "locked" here (its own status pill
// elsewhere already carries the review_pending distinction); nothing in
// R-I.2 asks the relationship badge to encode a second signal.
func lifecycleModifier(st TargetStatus) string {
	if st.Status == model.StatusDraft {
		return "draft"
	}
	return "locked"
}

// countSegment renders one chip's noun-and-count text — R-F.1's "a noun and
// a count, never a score" — pluralised with a plain trailing "s" unless the
// count is exactly 1 ("1 relationship", "0 relationships", "1 source",
// "3 sources").
//
// English irregulars are deliberately not handled: the two nouns are fixed
// literals in this file's callers, and a general pluraliser would be
// machinery for a set of size two.
func countSegment(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

const claimFooterChevronHTML = `<svg class="claim-footer__chevron" aria-hidden="true" viewBox="0 0 24 24" fill="none"><path d="m6 9 6 6 6-6"/></svg>`

// CommentChipHTML renders the 💬 comment chip for one claim, as a
// <span class="claim-comments-slot"> holding the chip <button>. It is bound
// into funcMap as "commentChip" for compatibility and emitted by
// EdgesHTMLWithLinks as the footer's final control. It is outside the
// independently collapsible evidence doors and remains visible when they close.
//
// It reads c.Comments directly rather than a per-render lookup, so it needs no
// config, no allowlist and therefore no override binding in internal/render —
// unlike "edges" and "mockupHTML", the exported func IS the binding, under both
// the parse-time funcMap and every Render pass. banner.html simply does not call
// it, which is what keeps banner claims out of the whole comment surface (it no
// longer calls {{edges .}} either).
//
// Values are hand-escaped because a FuncMap-returned template.HTML bypasses
// html/template's automatic escaping — nothing downstream will escape c.ID or
// the aria-label for us.
//
// The chip is emitted for EVERY claim, not only ones that already carry
// threads. Gating it on len(c.Comments) > 0 made the FIRST comment on a card
// unreachable from the viewer — the only surface the human has — so a claim
// nobody had questioned yet could never be questioned. The zero state is its own
// third variant (comment-chip--empty, reading "💬 0") rather than a borrowed
// --resolved, because "no one has commented" and "everything raised was settled"
// are different facts and the --resolved count would lie about the second.
//
// The zero-state slot ships with the bare `hidden` attribute and is revealed by
// shell.html only once its /api/ping probe confirms a live serve. A static
// file:// export has no reachable comment API and therefore mounts no composer,
// so an empty chip there would open a rail with nothing in it and no way to add
// anything — a dead control. Hidden-by-default (rather than
// shown-then-hidden-on-probe-failure) is deliberate: the capability appears when
// it is confirmed available, instead of flashing away ~1s after load and
// inviting a click that resolves to nothing. The attribute lives on the SLOT,
// not the button, which is the element shell.html's syncEmptyChips and the
// chromedp suite reach for via closest('.claim-comments-slot').
//
// This func emits no ` id="` sequence anywhere, deliberately: render's
// stripDuplicateClaimIDs matches a leading-space ` id="<claim-id>"` literal
// to strip ids from a track's non-canonical copy, and it must keep hitting
// only the root <section>.
func CommentChipHTML(c model.Claim) template.HTML {
	open := len(c.OpenThreadIDs())
	total := len(c.Comments)
	slotOpenTag := `<span class="claim-comments-slot" hidden>`
	chipClass := "comment-chip comment-chip--empty"
	count := 0
	label := "add the first comment on this claim"
	if total > 0 {
		slotOpenTag = `<span class="claim-comments-slot">`
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
	var b strings.Builder
	b.WriteString(slotOpenTag)
	b.WriteString(`<button type="button" class="`)
	b.WriteString(chipClass)
	b.WriteString(`" data-claim-id="`)
	b.WriteString(html.EscapeString(c.ID))
	b.WriteString(`" aria-controls="commentsPanel" aria-expanded="false" aria-label="`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`"><span class="comment-chip-glyph" aria-hidden="true"><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-message-circle"/></svg></span> <span class="comment-chip-count">`)
	b.WriteString(fmt.Sprintf("%d", count))
	b.WriteString(`</span></button></span>`)
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
//
// THE trusted CONDITION BELOW DELIBERATELY CARRIES NO LAYOUT TERM, and must not
// grow one. raw_html is legal on ANY layout as of v0.4.1 (issue #25): the
// raw-html-scope lint used to gate it on layout: mockup as well as on the
// module allowlist and raw_html_reviewed, and that layout leg was a second,
// hand-maintained spelling of a rule the other two legs already enforced —
// duplication that made a card claim's reviewed, allowlisted markup illegal for
// no reason anyone could state. Re-adding a layout term here "defensively" — a
// third conjunct testing the claim's Layout field against LayoutMockup — would
// re-encode exactly that duplication one layer down, and would silently escape
// the raw_html of every card/table/list/steps/tree/banner claim this release
// exists to let through. The whole file is grepped for that field name by this
// release's plan precisely so the term cannot creep back in unnoticed.
//
// The real second layer is unchanged and is what this func actually checks: the
// claim must be LOCKED (so a human signed off on the bytes), RawHTMLReviewed
// (so a human signed off on them AS MARKUP), and its module must be in the
// project's mockup_modules allowlist (so raw_html is a capability a project
// grants a module, not one any claim can take). Those three are layout-blind by
// design, and a draft, unreviewed or non-allowlisted claim's raw_html is
// emitted html-escaped on every layout, exactly as before.
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
// func, so all seven layout partials' <div class="k"> heading share this one
// implementation instead of each re-deriving a label from {{.ID}} in template
// syntax.
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
//
// showPrefix is the RETRY addition: a fixed-direction relationship row (see
// writeRelationshipRow) now carries the target's module/facet in its own,
// UNCONDITIONAL meta column (writeRelationshipMeta), so folding the same
// information into an elided inline prefix here as well would print it
// twice on those rows. Every other caller — writeIDListItems, for
// rests_on-adjacent "extra" rows, which have no meta column of their
// own — passes true and keeps this function's original elision behaviour
// exactly as it was.
func writeClaimRef(b *strings.Builder, targetID, fromModule, fromFacet string, targetStatuses map[string]TargetStatus, showPrefix bool) {
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
	if showPrefix {
		switch {
		case module != fromModule:
			prefix = DisplayCase(module) + claimRefModuleSep + DisplayCase(facet)
		case facet != fromFacet:
			prefix = DisplayCase(facet)
		}
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
