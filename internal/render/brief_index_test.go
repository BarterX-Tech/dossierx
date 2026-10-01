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

func briefsIndexHTML(t *testing.T, page string) string {
	t.Helper()
	_, index, ok := strings.Cut(page, `id="_briefs"`)
	if !ok {
		t.Fatal("no All briefs index")
	}
	index, _, ok = strings.Cut(index, "\n        </section>")
	if !ok {
		t.Fatal("All briefs index is not closed")
	}
	return index
}

// TestRender_BriefsIndexFoldersCapsAndNoFeatures is the B6 index at the
// renderer, the owner of the markup the browser suite then drives.
//
// Authoring gate: (1) the index lists every non-feature folder against the
// effective caps and keeps features/ out while the 60-total stays inclusive;
// (2) dropping the features exclusion, the inclusive total, or the cap
// pair would ship a page that disagrees with the sidebar and with check;
// (3) NIT-197's placeholder only counted files — it never named the cap or
// excluded features from the groups while counting them in the total;
// (4) no new production seam.
func TestRender_BriefsIndexFoldersCapsAndNoFeatures(t *testing.T) {
	cat, cfg, set := briefPageFixture()
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	index := briefsIndexHTML(t, out)
	for _, want := range []string{
		`>All briefs</h2>`,
		`class="briefs-index-meter">4 of 60, 1 is a feature</p>`,
		`data-folder="decisions"`,
		`>Decisions</h3><span class="briefs-index-folder__count">2 of 12</span>`,
		`data-folder="research"`,
		`>Research</h3><span class="briefs-index-folder__count">1 of 12</span>`,
		`href="#brief-decisions-round-to-the-cent">Balances round to the cent, once</a>`,
		`class="briefs-index__summary">Round once, at display.</span>`,
		`class="briefs-index__summary">No bank.</span>`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index is missing %s\n%s", want, index)
		}
	}
	if strings.Contains(index, "features") || strings.Contains(index, "Split a bill") {
		t.Error("features/ must be left out of the index groups")
	}

	_, home, ok := strings.Cut(out, `id="_home"`)
	if !ok {
		t.Fatal("no Home")
	}
	_, tile, ok := strings.Cut(home, `data-tile="briefs"`)
	if !ok {
		t.Fatal("Home's Briefs tile is missing")
	}
	tile, _, _ = strings.Cut(tile, "            </a>\n")
	for _, want := range []string{
		`href="#_briefs"`,
		`>Briefs</span><span class="home-tile__count home-wide">3</span>`,
		`>4 of 60, 1 is a feature</span>`,
		`>Decisions</span><span class="home-brief-folder__count">2 of 12</span>`,
		`<span class="home-tile__line home-narrow">3 · 1 locked</span>`,
	} {
		if !strings.Contains(tile, want) {
			t.Errorf("Briefs tile is missing %s\n%s", want, tile)
		}
	}
}

// TestRender_BriefsIndexUsesEffectiveCapsAndMarksOver: a raised or lowered
// config cap is the pair the index and the tile print, and a folder over
// its cap is marked with brief-folder-cap.
//
// Authoring gate: (1) the page reads BriefCapLimits, not the defaults;
// (2) hard-coding 60/12 would hide a human-raised cap and a red over-cap
// folder; (3) the fixture page's default-cap test cannot see an override;
// (4) no new seam.
func TestRender_BriefsIndexUsesEffectiveCapsAndMarksOver(t *testing.T) {
	cat, cfg := briefViewFixture()
	per, total := 1, 10
	cfg.MaxBriefsPerFolder, cfg.MaxBriefs = &per, &total
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("design-system/tokens.md", "---\nsummary: Tokens.\n---\n# Tokens\n"),
		briefFile("design-system/type.md", "---\nsummary: Type.\n---\n# Type\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	index := briefsIndexHTML(t, out)
	if !strings.Contains(index, `>2 of 10</p>`) {
		t.Errorf("the headline must use the configured total cap:\n%s", index)
	}
	if !strings.Contains(index, `briefs-index-folder__count--over">2 of 1</span>`) || !strings.Contains(index, `>brief-folder-cap</span>`) {
		t.Errorf("an over-cap folder must go red and name brief-folder-cap:\n%s", index)
	}
	if !strings.Contains(index, `>Design System</h3>`) {
		t.Error("design-system must title-case to Design System, as the sidebar does")
	}
}

// TestRender_BriefsIndexEscapesAuthorMarkup: a brief title and summary reach
// the index and the Home tile as text.
//
// Authoring gate: (1) hostile author text cannot become markup or a data
// attribute; (2) dropping html/template escaping on those fields would XSS
// a static viewer; (3) the brief page already escapes its own header — the
// index is a second writer; (4) no new seam.
func TestRender_BriefsIndexEscapesAuthorMarkup(t *testing.T) {
	cat, cfg := briefViewFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("notes/hostile.md", "---\nsummary: \"<img src=x onerror=alert(1)>\"\n---\n# <script>alert(1)</script>\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	index := briefsIndexHTML(t, out)
	if strings.Contains(index, "<script>") || strings.Contains(index, "<img src=x") {
		t.Fatalf("author markup must be escaped on the index:\n%s", index)
	}
	if !strings.Contains(index, "&lt;script&gt;alert(1)&lt;/script&gt;") || !strings.Contains(index, `&lt;img src=x onerror=alert(1)&gt;`) {
		t.Errorf("escaped title and summary missing:\n%s", index)
	}
	_, tile, ok := strings.Cut(out, `data-tile="briefs"`)
	if !ok {
		t.Fatal("no Briefs tile")
	}
	tile, _, _ = strings.Cut(tile, "</a>")
	if strings.Contains(tile, "<script>") || strings.Contains(tile, "<img src=x") {
		t.Fatal("author markup must not reach the Briefs tile as markup")
	}
}

// TestRender_HomeCardsCountBriefHalves: Edited after approval and To re-read
// name claims and briefs together; a features/ brief counts on the card and
// not on the Briefs tile groups.
//
// Authoring gate: (1) Home's waiting cards use briefMark/Evaluation, same
// as the sidebar; (2) hiding the brief half again would leave a pending
// brief off Waiting on you; (3) NIT-198 only filled the threads card;
// (4) no new seam.
func TestRender_HomeCardsCountBriefHalves(t *testing.T) {
	cfg := projectTestConfig(t)
	claim := projectTestClaim("widget", "overview", model.StatusLocked)
	cat, err := catalog.Build([]model.Claim{claim}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("notes/changed.md", "---\nsummary: Changed.\nstatus: locked\nrests_on: ["+claim.ID+"]\n---\n# Changed brief\n"),
		briefFile("features/moved.md", "---\nsummary: Feature.\nstatus: locked\nrests_on: ["+claim.ID+"]\n---\n# Moved feature\n"),
		briefFile("notes/quiet.md", "---\nsummary: Quiet.\n---\n# Quiet\n"),
	})
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	then := claim
	then.Body = "the wording the brief was approved against"
	for _, b := range set.Briefs {
		hashes, receipts, _ := briefs.Baselines(b, []model.Claim{then})
		lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{Path: b.Path, Hash: b.LockHash, At: "2026-09-30T00:00:00Z", Reason: "approved", Baselines: hashes, Receipts: receipts})
	}
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	cards, _ := homeCards(t, out)
	if got := cards["review"]; got[0] != "2" {
		t.Fatalf("to re-read must count both pending briefs, got %v", got)
	}
	if !strings.Contains(out, "2 briefs: something they rest on changed since approval.") {
		t.Error("the card must name how many briefs are waiting")
	}
	index := briefsIndexHTML(t, out)
	if !strings.Contains(index, `1 review pending`) || strings.Contains(index, "Moved feature") {
		t.Errorf("state totals cover the other folders only; features stay off the groups:\n%s", index)
	}
	if !strings.Contains(index, `3 of 60, 1 is a feature`) {
		t.Errorf("the inclusive total is missing:\n%s", index)
	}
}

// TestRender_BriefsIndexQuietFolderStartsCollapsed: a folder with nothing
// pending starts closed; one with an open thread starts open.
//
// Authoring gate: (1) collapsed quiet folders are the B6 rule; (2) opening
// every folder would hide the state-dot summary; (3) the placeholder always
// listed rows; (4) no new seam.
func TestRender_BriefsIndexQuietFolderStartsCollapsed(t *testing.T) {
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract", "internals"}}
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("voice/talking.md", "---\nsummary: Voice.\n---\n# Talking\n"),
		briefFile("notes/open.md", "---\nsummary: Open.\ncomments:\n  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-09-01T10:00:00Z\"\n    body: why?\n    edited: false\n---\n# Open\n"),
	})
	out, err := renderBoundedAt(&catalog.Catalog{}, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	index := briefsIndexHTML(t, out)
	if !strings.Contains(index, `<details class="briefs-index-folder" data-folder="voice">`) {
		t.Errorf("a quiet folder must start collapsed:\n%s", index)
	}
	if !strings.Contains(index, `data-folder="voice"`) || !strings.Contains(index, `data-mark="draft"`) {
		t.Errorf("a collapsed folder must carry its state dots:\n%s", index)
	}
	if !strings.Contains(index, `<details class="briefs-index-folder" open data-folder="notes">`) {
		t.Errorf("a folder with an open thread must start open:\n%s", index)
	}
}

// TestRender_BriefsIndexMarksTotalOver: the inclusive total over the
// effective max_briefs is red and names brief-total-cap.
//
// Authoring gate: (1) the meter must agree with check's brief-total-cap;
// (2) a folder-only over mark would hide a project-wide overflow; (3) the
// folder-over test cannot see a total overflow; (4) no new seam.
func TestRender_BriefsIndexMarksTotalOver(t *testing.T) {
	cat, cfg := briefViewFixture()
	total := 1
	cfg.MaxBriefs = &total
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("notes/a.md", "---\nsummary: A.\n---\n# A\n"),
		briefFile("notes/b.md", "---\nsummary: B.\n---\n# B\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	index := briefsIndexHTML(t, out)
	if !strings.Contains(index, `briefs-index-meter--over">2 of 1`) || !strings.Contains(index, `>brief-total-cap</span>`) {
		t.Errorf("an over-cap project must go red and name brief-total-cap:\n%s", index)
	}
}
