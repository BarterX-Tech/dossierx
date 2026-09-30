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
		briefFile("decisions/round-to-the-cent.md", "---\nsummary: Round once, at display.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# Balances round to the cent, once\n\n## Context\n\nThree people split a bill.\n\n![Split](split.png)\n\n## Decision\n\nRound once.\n"),
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
		`title="Balances round to the cent, once">Round to the cent</span><span class="brief-mark" data-mark="locked"`,
		`>No bank linking</span><span class="brief-mark" data-mark="draft"`,
		`class="brief-legend"`,
	} {
		if !strings.Contains(nav, want) {
			t.Errorf("sidebar is missing %s", want)
		}
	}
	if strings.Contains(nav, "features") || strings.Contains(nav, "split-a-bill") {
		t.Error("features/ must be left out of the Briefs tree")
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
		`<h2>Context</h2>`,
		`<img class="md-img" src="brief-assets/decisions/split.png" alt="Split">`,
		`Rests on · 1 claim`,
		`href="#widget.contract.overview"`,
		`claim-relationship-badge--locked">LOCKED<`,
		`Cited by · 1 claim`,
		`href="#widget.internals.cites"`,
		`claim-relationship-badge--draft">DRAFT<`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("brief page is missing %s", want)
		}
	}
	if strings.Contains(page, "<h1>") {
		t.Error("the title heading belongs in the header; the body must not repeat it")
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
}
