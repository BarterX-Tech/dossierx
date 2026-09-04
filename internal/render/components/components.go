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
// The whole footer ships inside a <details class="claim-links"> whose
// <summary> is a count digest — "N links - N files - N sources - N drifted",
// the sources and drifted segments present only when they are non-zero. A claim's edges are
// reference material a reader consults, not something they read on every pass,
// and expanded on every card they were the bulk of the page. The digest keeps
// the fact that there ARE edges (and that one of them has drifted) visible
// while closed. Two signals write the bare ` open` attribute server-side —
// any linked file Drifted, or the claim locked + review_pending — so the two
// states a reader must not miss are never hidden behind a click. The comment
// chip is NOT in here any more: it moved to the claim head (see
// CommentChipHTML), because a chip inside a collapsed footer is unclickable.
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
	// rows accumulates the <li> bodies and links/drifted counts the <summary>
	// needs, BEFORE anything is written to b — the whole <details>/<summary>/<ul>
	// prologue is conditional on there being something to disclose, and the
	// summary line quotes counts only the completed rows can supply. Emitting
	// the prologue first and unwinding it on a zero-edge claim would mean
	// carrying a "did I write anything yet" flag through every branch below.
	var rows strings.Builder
	links := 0

	if c.Governed.Type != "" {
		if c.Governed.Type == string(model.GovernedNone) {
			// "governed_by: none" DOES NOT COUNT AS A LINK. It is a stated
			// absence — the claim declaring that no doctrine backs it — not an
			// edge the reader can follow, and counting it made every claim in
			// every real project report at least "1 links" (governed_by is
			// mandatory: see internal/lint's governed-required). That in turn
			// made the zero-footer case below unreachable in practice, so the
			// design's promise that a claim with no edges and no files emits no
			// <details> at all never actually held. A NAMED governed_by target
			// still counts as one, in the else branch.
			//
			// The consequence is deliberate: on a claim whose ONLY footer
			// content would be this row, the whole <details> is suppressed and
			// the row (with its Reason) is not rendered. A disclosure control
			// that opens onto "this claim has no doctrine and nothing else
			// either" is exactly the empty triangle the suppression rule exists
			// to prevent. The moment anything else is disclosed — one edge, one
			// linked file — the row rides along inside as before.
			rows.WriteString(`<li class="claim-governed governed-none">governed_by: none`)
			if c.Governed.Reason != "" {
				rows.WriteString(` — `)
				// Reason is hand-written prose, not viewer chrome (unlike
				// Claim.Section below), and routinely names a claim id or a
				// path — so it goes through the same INLINE-ceiling renderer
				// every other prose field uses (markdown.RenderInline: code
				// spans and links, no block constructs) rather than a bare
				// html.EscapeString.
				rows.WriteString(string(markdown.RenderInline(c.Governed.Reason)))
			}
			rows.WriteString(`</li>`)
		} else {
			// governed_by names another claim, so it renders through the same
			// writeClaimRef every other claim-to-claim edge below uses rather
			// than its own hand-built <a>: a doctrine hub is nearly always in a
			// different facet from the claim it governs, so this is precisely
			// the edge whose prefix tier ("Doctrine › Hub") carries information.
			// A named target IS a link — one claim id the reader can follow —
			// so it counts, unlike the "none" branch above. review_pending
			// below counts the same way while carrying no claim id at all:
			// "links" is per-row-except-the-nested-id-lists, minus the one row
			// that states an absence.
			links++
			rows.WriteString(`<li class="claim-governed">governed_by: `)
			writeClaimRef(&rows, c.Governed.Type, c.Module, c.Facet, targetStatuses)
			rows.WriteString(`</li>`)
		}
	}

	if len(c.Mirrors) > 0 {
		links += len(c.Mirrors)
		rows.WriteString(`<li class="claim-mirrors">mirrors:`)
		writeIDListItems(&rows, c.Module, c.Facet, c.Mirrors, targetStatuses)
		rows.WriteString(`</li>`)
	}

	if len(c.RestsOn) > 0 {
		links += len(c.RestsOn)
		rows.WriteString(`<li class="claim-rests-on">rests_on:`)
		writeIDListItems(&rows, c.Module, c.Facet, c.RestsOn, targetStatuses)
		rows.WriteString(`</li>`)
	}

	if len(dependedBy) > 0 {
		links += len(dependedBy)
		rows.WriteString(`<li class="claim-depended-by">depended on by:`)
		writeIDListItems(&rows, c.Module, c.Facet, dependedBy, targetStatuses)
		rows.WriteString(`</li>`)
	}

	if c.MigratedFrom != "" {
		links++
		rows.WriteString(`<li class="claim-migrated">migrated_from: `)
		rows.WriteString(html.EscapeString(c.MigratedFrom))
		rows.WriteString(`</li>`)
	}

	// Sources sit beside migrated_from because they answer the same question
	// it does — where did this come from — and the pairing is the point: one
	// names a predecessor document in free text, the other names checkable
	// evidence. They do NOT count as links, and they are not folded into that
	// count for a reason the summary line below states: a link is a claim id
	// a reader can follow inside this corpus, and a source is evidence from
	// outside it. Counting them together would let a claim with no edges and
	// four citations report "4 links", which is not true of anything.
	if len(c.Sources) > 0 {
		writeSourcesRow(&rows, c)
	}

	// One condition, two consumers: review_pending is both a link in the
	// summary and (below) one of the two server-written auto-open signals.
	// They must stay keyed on the identical test.
	reviewPending := c.Status == model.StatusLocked && c.ReviewPending
	if reviewPending {
		links++
		rows.WriteString(`<li class="claim-review-pending">review_pending</li>`)
	}

	drifted := 0
	for _, f := range files {
		if f.Drifted {
			drifted++
		}
		rows.WriteString(`<li class="claim-implemented-in">implemented in: <code>`)
		rows.WriteString(html.EscapeString(f.File))
		if f.Symbol != "" {
			rows.WriteString(`#`)
			rows.WriteString(html.EscapeString(f.Symbol))
		}
		rows.WriteString(`</code>`)
		if f.Drifted {
			rows.WriteString(` <span class="pill pw">drifted</span>`)
		}
		rows.WriteString(`</li>`)
	}

	var b strings.Builder

	// Zero links, zero files and zero sources: emit no <details> at all — not
	// an empty disclosure reading "0 links - 0 files", which would be a control
	// that opens onto nothing on every claim with no edges yet. The two fixed
	// counts DO print as 0 whenever anything else is non-zero; it is only the
	// all-zero case that suppresses the whole footer. Sources join that test
	// rather than sitting outside it, because a claim whose only footer content
	// is its evidence must still be able to disclose it.
	//
	// Since "governed_by: none" no longer counts (see above), the both-zero
	// case now covers the claim that states an absence and nothing else, which
	// is the ordinary shape of an ungoverned claim with no edges yet — this
	// branch is what makes that claim emit a clean section with no dangling
	// disclosure triangle under it.
	if links > 0 || len(files) > 0 || len(c.Sources) > 0 {
		// Two auto-open signals, OR'd — either alone opens the footer. Both
		// read data already in scope (files' Drifted flag, the claim's own
		// status pair), which is why this needs no new parameter.
		//
		// There is a THIRD auto-open signal, the deep-link/fragment case, and
		// it is deliberately absent here: a URL fragment is never sent to the
		// server and is unknowable at render time. It is implemented as the
		// CSS rule `.claim:target .claim-links > *:not(summary)` in
		// viewer/template/style.css, and nothing in this package may try to
		// infer it.
		openAttr := ""
		for _, f := range files {
			if f.Drifted {
				openAttr = " open"
				break
			}
		}
		if reviewPending {
			openAttr = " open"
		}

		// Fixed ASCII words, a " - " separator (HYPHEN-MINUS, not the em dash
		// this file uses in prose) and plain %d counts — no claim-authored data
		// reaches this string, so it needs no escaping.
		//
		// Each count segment is pluralised: "1 link", "2 links", "1 file",
		// "0 files". The line used to be deliberately un-pluralised to keep a
		// fixed mono column, but "1 links" on a claim with exactly one edge is
		// the single most-read string in the viewer reading as a typo, and the
		// column argument never survived contact with the drifted segment
		// appearing and disappearing anyway. "drifted" is an adjective, not a
		// noun, so it is invariant ("1 drifted", "2 drifted") — nothing to
		// pluralise there. Separator, term order and the >0 gate on drifted are
		// exactly as the contract froze them; only the nouns changed.
		summary := countSegment(links, "link") + " - " + countSegment(len(files), "file")
		// The sources segment is CONDITIONAL where links and files are fixed,
		// and that asymmetry is load-bearing rather than an inconsistency: a
		// project that has never written a source must render byte-identically
		// to how it did before sources existed, and an unconditional "0
		// sources" would have changed the single most-read line in the viewer
		// on every claim in every corpus. It rides ahead of "drifted" because
		// drifted is an adjective about the FILES count it follows.
		if len(c.Sources) > 0 {
			summary += " - " + countSegment(len(c.Sources), "source")
		}
		if drifted > 0 {
			summary += fmt.Sprintf(" - %d drifted", drifted)
		}

		b.WriteString(`<details class="claim-links"`)
		b.WriteString(openAttr)
		b.WriteString(`><summary class="claim-links-summary">`)
		b.WriteString(summary)
		b.WriteString(`</summary><ul class="claim-edges">`)
		b.WriteString(rows.String())
		b.WriteString(`</ul></details>`)
	}

	// The baked-in thread panel follows the whole <details> (a <div>, so it
	// can't be an <li> inside the <ul>) but stays inside the claim's <section>,
	// since {{edges .}} is the last thing every non-banner partial emits before
	// </section>. It is a SIBLING of the disclosure, never a child: a claim's
	// threads must stay readable without expanding its edges, and — crucially —
	// a claim with comments but no edges at all suppresses the <details> above
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

// countSegment renders one segment of the <summary> digest — the count and its
// noun, pluralised with a plain trailing "s" unless the count is exactly 1
// ("1 link", "0 links", "2 files", "3 sources"). Only "link", "file" and
// "source" go through here; "drifted" is an adjective and stays invariant at
// every count.
//
// English irregulars are deliberately not handled: the three nouns are fixed
// literals in this file's only caller, and a general pluraliser would be
// machinery for a set of size three.
func countSegment(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

// CommentChipHTML renders the 💬 comment chip for one claim, as a
// <span class="claim-comments-slot"> holding the chip <button>. It is bound
// into funcMap as "commentChip" and called directly from each chip-bearing
// partial's <div class="k"> heading — {{commentChip .}}, the whole claim — so
// the chip sits in the claim HEAD rather than in the edges footer, where it
// used to ride as an <li class="claim-comments"> inside the shared
// <ul class="claim-edges">. It moved because the footer is now a collapsed
// <details> (see EdgesHTMLWithLinks): a chip inside it would be invisible, and
// unclickable, on every claim whose footer starts closed.
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
// stripOverviewIDs matches a leading-space ` id="<claim-id>"` literal to strip
// the duplicate ids an overview claim's N rendered copies would otherwise carry,
// and it must keep hitting only the root <section>.
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
	b.WriteString(`"><span class="comment-chip-glyph" aria-hidden="true">💬</span> <span class="comment-chip-count">`)
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
// func, so all seven layout partials' <div class="k"> heading share this one
// implementation instead of each re-deriving a label from {{.ID}} in template
// syntax. The Build order tab's diagram nodes label claims by the SAME rule
// through a deliberate duplicate in internal/buildorder/mermaid.go
// (claimLabel there; the render package cannot be imported from below it),
// and internal/render/build_order_view_test.go pins the two to agree.
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
