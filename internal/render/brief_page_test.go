package render

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func briefImage(rel string, size int64) briefs.File {
	return briefs.File{Rel: rel, Regular: true, Size: size}
}

// briefPageFixture is a project with briefs in two folders, one of them a
// locked brief with an image that rests on one claim and is cited by another,
// plus a features/ brief, and a claim-only catalog beside them.
func briefPageFixture() (*catalog.Catalog, *config.Config, *briefs.Set) {
	cat := &catalog.Catalog{Claims: []model.Claim{
		{ID: "widget.contract.overview", Module: "widget", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusLocked, Body: "The overview."},
		{ID: "widget.internals.cites", Module: "widget", Facet: "internals", Layout: model.LayoutCard, Status: model.StatusDraft, Body: "Cites the brief [1].",
			Sources: []model.Source{{Ref: 1, Kind: model.SourceKindInternal, Title: "The rounding brief", Path: "./briefs/decisions/round-to-the-cent.md"}}},
	}}
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract", "internals"}}
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("decisions/round-to-the-cent.md", "---\nsummary: Round once, at display.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# Balances round to the cent, once\n\n## Context\n\nThree people split a bill.\n\n![Split](split.png)\n\n## Decision\n\nRound once.\n\n# A stray top heading\n\n### Detail\n"),
		briefImage("decisions/split.png", 2048),
		briefFile("decisions/no-bank-linking.md", "---\nsummary: No bank.\n---\nWe never link a bank.\n"),
		briefFile("research/interviews.md", "---\nsummary: Twelve households.\n---\n## Findings\n\nMonthly.\n"),
		briefFile("features/split-a-bill.md", "---\nsummary: Splitting a bill.\n---\n# Split a bill\n"),
	})
	return cat, cfg, set
}

func sectionHTML(t *testing.T, page, id string) string {
	t.Helper()
	start := strings.Index(page, `<section class="module-section brief-section" id="`+id+`"`)
	if start < 0 {
		t.Fatalf("no brief section with id %q", id)
	}
	end := strings.Index(page[start:], "\n        </section>")
	if end < 0 {
		t.Fatalf("brief section %q is not closed", id)
	}
	return page[start : start+end]
}

// TestRender_BriefPageAndTree is the brief page contract (NIT-197) at the
// renderer, the owner of the markup the browser suite then drives: the tree
// lists every folder but features/, each brief page carries its header, the
// body with its title heading lifted into the header and its image on the
// relative brief-assets path, the claims it rests on and is cited by with
// their lifecycle badges, and the counts against the caps.
func TestRender_BriefPageAndTree(t *testing.T) {
	cat, cfg, set := briefPageFixture()
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}

	nav := out[strings.Index(out, `<nav id="nav">`):strings.Index(out, `</nav>`)]
	for _, want := range []string{
		`placeholder="Search claims and briefs"`,
		`<span>Briefs</span><span class="system-nav-group__count">3</span>`,
		`data-target="#_briefs"`,
		`data-folder="decisions"`, `<span class="brief-folder__label">Decisions</span><span class="brief-folder__count">2</span>`,
		`data-folder="research"`,
		`data-target="#brief-decisions-round-to-the-cent"`,
		// A row is the brief's own title, the page heading's words (NIT-249),
		// not its file name in sentence case; with no heading the title is
		// the file name title-cased.
		`title="Balances round to the cent, once">Balances round to the cent, once</span><span class="brief-mark" data-mark="locked"`,
		`>No Bank Linking</span><span class="brief-mark" data-mark="draft"`,
		`class="brief-legend"`,
	} {
		if !strings.Contains(nav, want) {
			t.Errorf("sidebar is missing %s", want)
		}
	}
	_, tree, ok := strings.Cut(nav, `class="system-nav-group brief-nav"`)
	if !ok {
		t.Fatal("no Briefs group in the sidebar")
	}
	if strings.Contains(tree, "features") || strings.Contains(tree, "split-a-bill") {
		t.Error("features/ must be left out of the Briefs tree: a feature is listed once, under Features")
	}
	if !strings.Contains(out, `id="brief-features-split-a-bill"`) {
		t.Error("a features/ brief still renders its page, so a link to it resolves")
	}

	page := sectionHTML(t, out, "brief-decisions-round-to-the-cent")
	for _, want := range []string{
		`data-brief-path="briefs/decisions/round-to-the-cent.md"`,
		`<span class="brief-kicker__word">Brief</span> · <span class="brief-kicker__word">Decisions</span><span class="brief-kicker__path"> · briefs/decisions/round-to-the-cent.md</span>`,
		`>Balances round to the cent, once</h2>`,
		`<p class="brief-lede">Round once, at display.</p>`,
		`class="pill ps brief-pill"`,
		`of 2,000 words · 1 of 3 images`,
		`<h3>Context</h3>`,
		`<h3>A stray top heading</h3>`,
		`<h4>Detail</h4>`,
		`<img class="md-img" src="brief-assets/decisions/split.png" alt="Split">`,
		`Rests on · 1 claim`,
		`href="#widget.contract.overview"`,
		`claim-relationship-badge--locked">LOCKED<`,
		`Cited by · 1 claim`,
		`href="#widget.internals.cites"`,
		`claim-relationship-badge--draft">DRAFT<`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("brief page is missing %s\n%s", want, page)
		}
	}
	// The title is the page's top heading: the body's sections sit below it,
	// and a stray "#" after the title may not outrank it.
	_, body, ok := strings.Cut(page, `<article class="brief-body">`)
	if !ok {
		t.Fatal("the brief page has no body")
	}
	for _, tag := range []string{"<h1>", "<h2>"} {
		if strings.Contains(body, tag) {
			t.Errorf("the body must hold no %s: the title is the page's h2 and its sections sit below it", tag)
		}
	}
	draft := sectionHTML(t, out, "brief-decisions-no-bank-linking")
	if !strings.Contains(draft, `class="pill pv brief-pill"`) || strings.Contains(draft, "brief-relations") {
		t.Error("a draft brief with no rests_on and no citation: a draft pill and no relations card")
	}

	// The payload names each brief's page and carries the body with its image.
	m := briefsBlock.FindStringSubmatch(out)
	var p briefsPayload
	if len(m) < 2 || json.Unmarshal([]byte(m[1]), &p) != nil {
		t.Fatal("expected the briefs payload")
	}
	for _, b := range p.Briefs {
		if b.ID == "decisions.round-to-the-cent" {
			if b.Anchor != "brief-decisions-round-to-the-cent" || !strings.Contains(b.BodyHTML, `src="brief-assets/decisions/split.png"`) {
				t.Errorf("payload entry: anchor %q, body %s", b.Anchor, b.BodyHTML)
			}
		}
	}
}

// TestRender_BriefPageIDsAreUniqueOnThePage: two briefs, and a brief and a
// module facet, can spell the same brief-<folder>-<slug>. Every id on the page
// must name one element, and the payload's anchor must be the id the section
// actually carries.
func TestRender_BriefPageIDsAreUniqueOnThePage(t *testing.T) {
	cat := &catalog.Catalog{Claims: []model.Claim{
		{ID: "brief-q.contract.one", Module: "brief-q", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusDraft, Body: "x"},
	}}
	cfg := &config.Config{Modules: []string{"brief-q"}, Facets: []string{"contract", "internals"}}
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("a-b/c.md", "---\nsummary: One.\n---\none\n"),
		briefFile("a/b-c.md", "---\nsummary: Two.\n---\ntwo\n"),
		briefFile("q/contract.md", "---\nsummary: Three.\n---\nthree\n"),
		// A title heading's id must not be a second brief's section id.
		briefFile("f/x.md", "---\nsummary: Four.\n---\n# Ex\n"),
		briefFile("f/x-title.md", "---\nsummary: Five.\n---\n# Ex title\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	main := out[strings.Index(out, `<main class="content-area">`):strings.Index(out, `</main>`)]
	ids := regexp.MustCompile(`<[a-z]+[^>]*\sid="([^"]+)"`).FindAllStringSubmatch(main, -1)
	seen := map[string]int{}
	for _, id := range ids {
		seen[id[1]]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("id %q names %d elements", id, n)
		}
	}
	var p briefsPayload
	if err := json.Unmarshal([]byte(briefsBlock.FindStringSubmatch(out)[1]), &p); err != nil {
		t.Fatal(err)
	}
	for _, b := range p.Briefs {
		if !strings.Contains(out, `<section class="module-section brief-section" id="`+b.Anchor+`"`) {
			t.Errorf("payload anchor %q names no brief section", b.Anchor)
		}
	}
}

// TestRenderBoundedWith_ChargesBriefImagesToTheBound: the static viewer is the
// page plus the images check copies beside it, so a bound that holds the page
// exactly holds it only when the images fit too. One byte short is the
// capacity refusal.
func TestRenderBoundedWith_ChargesBriefImagesToTheBound(t *testing.T) {
	cat, cfg, set := briefPageFixture()
	at := time.Unix(1_700_000_000, 0).UTC()
	page, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, at, 0)
	if err != nil {
		t.Fatal(err)
	}
	const imageBytes = 2048
	if _, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, at, len(page)+imageBytes); err != nil {
		t.Fatalf("the page and its image fit exactly: %v", err)
	}
	_, err = renderBoundedAt(cat, cfg, Extras{Briefs: set}, at, len(page)+imageBytes-1)
	if !errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("a bound one byte short of page+image must be refused, got %v", err)
	}
	// The refusal names the images as a part of the bound (F2): here they fit
	// alone and the page tips it, and the message still says what they take.
	if !errors.Is(err, ErrBriefImagesOverBound) || !strings.Contains(err.Error(), "briefs/decisions/split.png 2048 bytes") ||
		!strings.Contains(err.Error(), "take 2048 of them") || !strings.Contains(err.Error(), "shrink or remove brief images") {
		t.Fatalf("the refusal must name the brief images, their total and the recovery: %v", err)
	}
	// With no image in the bound, the refusal stays the page's own.
	_, err = renderBoundedAt(cat, cfg, Extras{}, at, 1024)
	if errors.Is(err, ErrBriefImagesOverBound) {
		t.Fatalf("a viewer with no brief image must not blame images: %v", err)
	}
}

// TestRender_FeaturesOnlyProjectMakesNoBriefsPromise: a project whose only
// briefs are under features/ has no tree, so it gets no Briefs group and no
// "All briefs" index (NIT-197 F14). Since NIT-201 its features are listed
// under Features, which the search reaches and whose rows carry marks, so it
// does get the "…and briefs" search promise and the marks legend.
func TestRender_FeaturesOnlyProjectMakesNoBriefsPromise(t *testing.T) {
	cat, cfg := briefViewFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("features/split-a-bill.md", "---\nsummary: Splitting a bill.\n---\n# Split a bill\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{`id="_briefs"`, `class="system-nav-group brief-nav"`} {
		if strings.Contains(out, absent) {
			t.Errorf("a features-only project must not carry %s", absent)
		}
	}
	for _, present := range []string{"Search claims and briefs", `class="system-nav-group feature-nav"`, `class="brief-legend"`} {
		if !strings.Contains(out, present) {
			t.Errorf("a features-only project lists its features, so it must carry %s", present)
		}
	}
	if !strings.Contains(out, `id="brief-features-split-a-bill"`) {
		t.Error("the feature brief's page must still render")
	}
}

// TestRender_BriefThreadsOnThePageTreeAndIndex is the render half of threads
// on a brief (NIT-198). A locked brief with an open thread shows the thread
// mark in the tree (it outranks locked), its count on the "All briefs" index,
// and its page carries its id and counts for the comment rail and the baked
// threads a static build's rail reads, keyed data-brief-id and inert. A brief
// whose only thread is resolved keeps its own mark, its page-foot button is
// live (there is something to read), and a brief with none has it disabled.
func TestRender_BriefThreadsOnThePageTreeAndIndex(t *testing.T) {
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract", "internals"}}
	thread := func(id, status, body string) string {
		return "  - id: " + id + "\n    status: " + status + "\n    author: human\n    created: \"2026-09-01T10:00:00Z\"\n    body: " + body + "\n    edited: false\n"
	}
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("decisions/open.md", "---\nsummary: Open.\nstatus: locked\ncomments:\n"+thread("c-aaaaaa", "open", "\"<img src=x onerror=alert(1)>\"")+"---\n# Open\n\n## One\n\nText.\n"),
		briefFile("decisions/settled.md", "---\nsummary: Settled.\nstatus: locked\ncomments:\n"+thread("c-bbbbbb", "resolved", "done")+"---\n# Settled\n\nText.\n"),
		briefFile("decisions/quiet.md", "---\nsummary: Quiet.\n---\n# Quiet\n\nText.\n"),
	})
	out, err := renderBoundedAt(&catalog.Catalog{}, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	nav := out[strings.Index(out, `<nav id="nav">`):strings.Index(out, `</nav>`)]
	for _, want := range []string{
		`>Open</span><span class="brief-mark" data-mark="thread" role="img" aria-label="1 open thread"`,
		`>Settled</span><span class="brief-mark" data-mark="locked"`,
		`>Quiet</span><span class="brief-mark" data-mark="draft"`,
	} {
		if !strings.Contains(nav, want) {
			t.Errorf("sidebar is missing %s", want)
		}
	}

	_, index, ok := strings.Cut(out, `id="_briefs"`)
	if !ok {
		t.Fatal("no All briefs index")
	}
	index, _, _ = strings.Cut(index, "</section>\n        </section>")
	if !strings.Contains(index, `<a href="#brief-decisions-open">Open</a>`) || strings.Count(index, `class="briefs-index__threads"`) != 1 {
		t.Errorf("the index must give the brief with an open thread, and only it, its count:\n%s", index)
	}

	open := sectionHTML(t, out, "brief-decisions-open")
	for _, want := range []string{
		`data-brief-id="decisions.open" data-open-threads="1" data-threads="1"`,
		`<div class="comments-panel" data-brief-id="decisions.open" hidden>`,
		`data-thread-id="c-aaaaaa"`,
	} {
		if !strings.Contains(open, want) {
			t.Errorf("the brief page is missing %s", want)
		}
	}
	if strings.Contains(open, "<img src=x") || strings.Contains(open, `data-claim-id="decisions.open"`) {
		t.Error("a baked brief thread must be escaped, and keyed by data-brief-id alone")
	}
	settled := sectionHTML(t, out, "brief-decisions-settled")
	if !strings.Contains(settled, `data-open-threads="0" data-threads="1"`) || strings.Contains(settled, `class="brief-comment" aria-controls="commentsPanel" aria-expanded="false" disabled`) {
		t.Error("a brief with a resolved thread has none open, and its button opens the thread read only")
	}
	quiet := sectionHTML(t, out, "brief-decisions-quiet")
	if !strings.Contains(quiet, `class="brief-comment" aria-controls="commentsPanel" aria-expanded="false" disabled`) || strings.Contains(quiet, "comments-panel") {
		t.Error("a brief with no thread renders its button disabled (the viewer enables it under serve) and bakes no panel")
	}
}
