package render

import (
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// featureFixture: two modules and a project claim, and three features in
// features/ written out of file-name order on disk order's behalf. One rests
// on claims in two modules and the project, in interleaved order, with one id
// that names no claim; one rests only on locked claims; one rests on nothing.
// A decisions/ brief is there to be linked to.
func featureFixture() (*catalog.Catalog, *config.Config, *briefs.Set) {
	cat := &catalog.Catalog{Claims: []model.Claim{
		{ID: "splitting.contract.exact-fraction", Module: "splitting", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusLocked, Body: "x"},
		{ID: "splitting.contract.shares", Module: "splitting", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusDraft, Body: "x"},
		{ID: "expenses.contract.payer", Module: "expenses", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusLocked, Body: "x"},
		{ID: "project.currency", Scope: model.ScopeProject, Layout: model.LayoutCard, Status: model.StatusLocked, Body: "x"},
	}}
	cfg := &config.Config{Modules: []string{"splitting", "expenses"}, Facets: []string{"contract", "internals"}}
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("features/split-a-bill.md", "---\nsummary: Anyone adds what they paid.\nstatus: locked\nrests_on:\n  - splitting.contract.shares\n  - expenses.contract.payer\n  - splitting.contract.exact-fraction\n  - checkout.contract.ghost\n  - project.currency\n---\n# Split a bill <script>alert(1)</script>\n\n## What it is\n\nWording follows [Talking about money](../voice/talking-about-money.md) · layout follows [Colour](briefs/voice/talking-about-money.md#type) and [again](/briefs/voice/talking-about-money.md) · see [Export](export-to-csv.md) · not a brief: [notes](../voice/nothing-here.md) · [site](https://example.com/a.md).\n"),
		briefFile("features/export-to-csv.md", "---\nsummary: A CSV of every expense.\nstatus: locked\nrests_on:\n  - expenses.contract.payer\n---\n# Export to CSV\n"),
		briefFile("features/receipt-scanning.md", "---\nsummary: Read a receipt from a photo.\n---\n# Receipt scanning\n"),
		briefFile("voice/talking-about-money.md", "---\nsummary: How we say it.\n---\n# Talking about money\n"),
	})
	return cat, cfg, set
}

// TestRender_FeaturePage is the feature contract (NIT-201) at the renderer,
// the owner of the markup the browser suite drives: features are listed once,
// under their own Features entry in file-name order with the brief's own
// state mark; a feature page is the brief page with a Feature kicker, a meta
// line counting what it rests on, and a Made of list grouped by module in
// rests_on order; its body links to other briefs open their pages; Home
// carries a Features tile. Nothing on any of it speaks of built, specified or
// conformance state.
func TestRender_FeaturePage(t *testing.T) {
	cat, cfg, set := featureFixture()
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}

	nav := out[strings.Index(out, `<nav id="nav">`):strings.Index(out, `</nav>`)]
	features := nav[strings.Index(nav, `class="system-nav-group feature-nav"`):strings.Index(nav, `class="system-nav-group brief-nav"`)]
	for _, want := range []string{
		`<span>Features</span><span class="system-nav-group__count">3</span>`,
		`data-target="#brief-features-export-to-csv"`,
		`>Export to CSV</span><span class="brief-mark" data-mark="locked" role="img" aria-label="Locked"`,
		`>Receipt scanning</span><span class="brief-mark" data-mark="draft" role="img" aria-label="Draft"`,
	} {
		if !strings.Contains(features, want) {
			t.Errorf("Features entry is missing %s\n%s", want, features)
		}
	}
	// File-name order: export-to-csv, receipt-scanning, split-a-bill.
	e, r, s := strings.Index(features, "#brief-features-export-to-csv"), strings.Index(features, "#brief-features-receipt-scanning"), strings.Index(features, "#brief-features-split-a-bill")
	if e >= r || r >= s {
		t.Errorf("Features rows must be in file-name order: export %d, receipt %d, split %d", e, r, s)
	}
	if _, tree, ok := strings.Cut(nav, `class="system-nav-group brief-nav"`); !ok || strings.Contains(tree, "features") {
		t.Error("a feature must not be listed again in the Briefs tree")
	}
	if strings.Contains(out, "<script>alert(1)") {
		t.Error("a feature's title must be escaped wherever it is written")
	}

	page := sectionHTML(t, out, "brief-features-split-a-bill")
	for _, want := range []string{
		`<p class="brief-kicker"><span class="brief-kicker__word">Feature</span><span class="brief-kicker__path"> · briefs/features/split-a-bill.md</span></p>`,
		// Four of the five name a claim; three of those are locked. Modules
		// in the order their first claim appears; the project claim's is
		// Project; the ghost id is in no module.
		`Rests on 5 claims in Splitting, Expenses and Project, 3 of 5 locked · `,
		`<span class="brief-narrow">5 claims, 3 of 5 locked</span>`,
		`<span class="brief-wide">Made of · 5 claims in 3 modules</span><span class="brief-narrow">Made of · 5 claims</span>`,
		`data-toc-label="Made of · 5 claims"`,
		`<span class="brief-relations__note">rests_on, not an order</span>`,
		`Comment on this feature`,
		// Body links to another brief, in each form a markdown link can
		// name one, open its page; anything else is left as written.
		`<a href="#brief-voice-talking-about-money">Talking about money</a>`,
		`<a href="#brief-voice-talking-about-money">Colour</a>`,
		`<a href="#brief-voice-talking-about-money">again</a>`,
		`<a href="#brief-features-export-to-csv">Export</a>`,
		`<a href="../voice/nothing-here.md">notes</a>`,
		`<a href="https://example.com/a.md">site</a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("feature page is missing %s\n%s", want, page)
		}
	}
	// Made of: groups in first-appearance order, rows in rests_on order
	// within a group, the id that names no claim last in a group of its own.
	_, madeOf, ok := strings.Cut(page, `feature-made-of`)
	if !ok {
		t.Fatal("the feature page has no Made of list")
	}
	order := []string{
		`<p class="feature-made-of__module">Splitting</p>`, `href="#splitting.contract.shares"`, `href="#splitting.contract.exact-fraction"`,
		`<p class="feature-made-of__module">Expenses</p>`, `href="#expenses.contract.payer"`,
		`<p class="feature-made-of__module">Project</p>`, `href="#project.currency"`,
		`<p class="feature-made-of__module">Not a claim</p>`, `checkout.contract.ghost`,
	}
	at := 0
	for _, want := range order {
		i := strings.Index(madeOf[at:], want)
		if i < 0 {
			t.Fatalf("Made of is missing %s after offset %d, or out of order\n%s", want, at, madeOf)
		}
		at += i + len(want)
	}
	if !strings.Contains(madeOf, `claim-relationship-badge--draft">DRAFT<`) || !strings.Contains(madeOf, `claim-relationship-badge--locked">LOCKED<`) {
		t.Error("each Made of row carries its claim's lifecycle badge")
	}
	if strings.Contains(page, "Rests on · ") {
		t.Error("the Made of list replaces the Rests on list on a feature")
	}
	lower := strings.ToLower(page + nav)
	for _, word := range []string{"built", "specified", "conformance"} {
		if strings.Contains(lower, word) {
			t.Errorf("a feature shows its brief's own state only; found %q", word)
		}
	}

	allLocked := sectionHTML(t, out, "brief-features-export-to-csv")
	if !strings.Contains(allLocked, `Rests on 1 claim in Expenses, all locked · `) || !strings.Contains(allLocked, `1 claim, all locked`) {
		t.Errorf("a feature whose every claim is locked says all locked\n%s", allLocked)
	}
	empty := sectionHTML(t, out, "brief-features-receipt-scanning")
	if !strings.Contains(empty, `Rests on 0 claims · `) || !strings.Contains(empty, `Its rests_on names no claim yet.`) || !strings.Contains(empty, `No claims yet`) {
		t.Errorf("a feature that rests on nothing says so in its meta and its Made of card\n%s", empty)
	}

	_, home, ok := strings.Cut(out, `id="_home"`)
	if !ok {
		t.Fatal("no Home section")
	}
	_, tile, ok := strings.Cut(home, `data-tile="features"`)
	if !ok {
		t.Fatal("the Features tile belongs on Home")
	}
	tile, _, _ = strings.Cut(tile, "            </div>\n")
	for _, want := range []string{
		`<a class="home-tile__title home-tile__link" href="#brief-features-export-to-csv">Features</a><span class="home-tile__count home-wide">3</span>`,
		`<a class="home-feature" href="#brief-features-export-to-csv"><span class="home-feature__title">Export to CSV</span><span class="home-feature__state" data-state="locked">locked</span></a>`,
		`<span class="home-feature__state" data-state="draft">draft</span>`,
		`<span class="home-tile__line home-narrow">3 · 2 locked · 1 draft</span>`,
	} {
		if !strings.Contains(tile, want) {
			t.Errorf("Home's Features tile is missing %s\n%s", want, tile)
		}
	}
}

// TestRender_NoFeatureNoFeatureMarkup: a project whose briefs hold no
// features/ folder gets no Features entry, no Features tile and no feature
// page, so its viewer is the NIT-197 one byte for byte outside the
// stylesheet and scripts.
func TestRender_NoFeatureNoFeatureMarkup(t *testing.T) {
	cat, cfg, _ := briefPageFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("decisions/no-bank-linking.md", "---\nsummary: No bank.\n---\nWe never link a bank.\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	body := out[strings.Index(out, `<nav id="nav">`):strings.Index(out, `</main>`)]
	for _, absent := range []string{`class="system-nav-group feature-nav"`, `data-tile="features"`, `feature-made-of`, `>Feature<`} {
		if strings.Contains(body, absent) {
			t.Errorf("a project with no features/ must not carry %s", absent)
		}
	}
}

// TestRender_BriefMarksReadTheReviewState: with the briefs' lock and review
// state (NIT-205) a feature row's mark is the one the legend names, in the
// legend's precedence — edited since approval, then review pending, then an
// open thread, then the lock — and a file that says locked with no approval
// on record is not drawn locked. Home's tile reads the same state, and a
// Briefs tree row the same mark (one briefMark for all three).
func TestRender_BriefMarksReadTheReviewState(t *testing.T) {
	cat, cfg := briefViewFixture()
	thread := "comments:\n  - id: c-1\n    status: open\n    author: human\n    created: 2026-09-30T00:00:00Z\n    body: why?\n    edited: false\n"
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("features/a-locked.md", "---\nsummary: A.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# A locked\n"),
		briefFile("features/b-review.md", "---\nsummary: B.\nstatus: locked\nrests_on: [widget.contract.overview]\n"+thread+"---\n# B review\n"),
		briefFile("features/c-edited.md", "---\nsummary: C.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# C edited\n"),
		briefFile("features/d-unrecorded.md", "---\nsummary: D.\nstatus: locked\n---\n# D unrecorded\n"),
		briefFile("features/e-thread.md", "---\nsummary: E.\n"+thread+"---\n# E thread\n"),
		briefFile("notes/f-edited.md", "---\nsummary: F.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# F edited\n"),
	})
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	then := cat.Claims[0]
	then.Body = "the wording the brief was approved against"
	for _, b := range set.Briefs {
		baseline := cat.Claims[0]
		hash := b.LockHash
		switch b.Slug {
		case "b-review":
			baseline = then
		case "c-edited", "f-edited":
			hash = "sha256:the-text-that-was-approved"
		case "d-unrecorded", "e-thread":
			continue
		}
		hashes, receipts, _ := briefs.Baselines(b, []model.Claim{baseline})
		lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{Path: b.Path, Hash: hash, At: "2026-09-30T00:00:00Z", Reason: "approved", Baselines: hashes, Receipts: receipts})
	}
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`>A locked</span><span class="brief-mark" data-mark="locked" role="img" aria-label="Locked"`,
		`>B review</span><span class="brief-mark" data-mark="review" role="img" aria-label="Review pending"`,
		`>C edited</span><span class="brief-mark" data-mark="edited" role="img" aria-label="Edited since approval"`,
		`>D unrecorded</span><span class="brief-mark" data-mark="draft" role="img" aria-label="Not locked: no approval on record"`,
		`>E thread</span><span class="brief-mark" data-mark="thread" role="img" aria-label="1 open thread"`,
		`>F edited</span><span class="brief-mark" data-mark="edited" role="img" aria-label="Edited since approval"`,
		`<span class="home-feature__title">B review</span><span class="home-feature__state" data-state="review">review</span>`,
		`<span class="home-feature__title">C edited</span><span class="home-feature__state" data-state="edited">edited</span>`,
		`<span class="home-feature__title">E thread</span><span class="home-feature__state" data-state="draft">draft</span>`,
		`<span class="home-tile__line home-narrow">5 · 1 locked · 1 edited · 1 review · 2 draft</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
}
