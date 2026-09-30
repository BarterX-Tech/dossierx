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

var briefsBlock = regexp.MustCompile(`<script type="application/json" id="dossierx-briefs">([^<]*)</script>`)

func briefViewFixture() (*catalog.Catalog, *config.Config) {
	cat := &catalog.Catalog{Claims: []model.Claim{{
		ID: "widget.contract.overview", Module: "widget", Facet: "contract",
		Layout: model.LayoutCard, Status: model.StatusDraft, Body: "# Not a heading in a claim",
	}}}
	return cat, &config.Config{Modules: []string{"widget"}, Facets: []string{"contract", "internals"}}
}

func briefFile(rel, body string) briefs.File {
	return briefs.File{Rel: rel, Regular: true, Data: []byte(body), Size: int64(len(body))}
}

// TestRenderWith_BriefsPayload is the viewer-payload contract (NIT-204): with
// no brief the page is byte-identical to one rendered before briefs existed;
// with briefs, one JSON block carries every brief with the fields the UI will
// read — its body in document mode, so "# Title" is an h1 there while the
// claim's own "#" line on the same page stays literal text — and its counts
// beside the caps.
func TestRenderWith_BriefsPayload(t *testing.T) {
	cat, cfg := briefViewFixture()
	at := time.Unix(1_700_000_000, 0).UTC()

	plain, err := renderAt(cat, cfg, at)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := renderBoundedAt(cat, cfg, Extras{Briefs: briefs.FromFiles(cfg, nil)}, at, 0)
	if err != nil {
		t.Fatal(err)
	}
	if empty != plain {
		t.Fatal("a project with no briefs must render byte-identically")
	}
	if strings.Contains(plain, "dossierx-briefs") {
		t.Fatal("no briefs, no payload element")
	}

	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("widget/flow.md", "---\nsummary: The widget flow.\nrests_on: [widget.contract.overview]\n---\n# Widget flow\n\n## Why\n\n<b>escaped</b> text.\n"),
		briefFile("widget/notes.md", "---\nsummary: Notes.\nstatus: locked\n---\nplain\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, at, 0)
	if err != nil {
		t.Fatal(err)
	}
	m := briefsBlock.FindStringSubmatch(out)
	if m == nil {
		t.Fatal("expected one dossierx-briefs payload block")
	}
	if strings.Contains(m[1], "<") {
		t.Fatal("the payload must reach the page with every < JSON-escaped")
	}
	var p briefsPayload
	if err := json.Unmarshal([]byte(m[1]), &p); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if p.Total != 2 || len(p.Folders) != 1 || p.Folders[0].Count != 2 || p.Caps != cfg.BriefCapLimits() {
		t.Fatalf("unexpected counts or caps: %+v", p)
	}
	flow := p.Briefs[0]
	if flow.ID != "widget.flow" || flow.Path != "briefs/widget/flow.md" || flow.Folder != "widget" || flow.Title != "Widget flow" ||
		flow.Summary != "The widget flow." || flow.Status != "draft" || strings.Join(flow.RestsOn, ",") != "widget.contract.overview" ||
		flow.Words != 7 || flow.ImageCount != 0 {
		t.Fatalf("unexpected brief entry: %+v", flow)
	}
	if !strings.Contains(flow.BodyHTML, "<h1>Widget flow</h1>") || !strings.Contains(flow.BodyHTML, "<h2>Why</h2>") ||
		!strings.Contains(flow.BodyHTML, "&lt;b&gt;escaped&lt;/b&gt;") {
		t.Fatalf("body must render in document mode and escape author markup: %s", flow.BodyHTML)
	}
	if p.Briefs[1].Status != "locked" {
		t.Fatalf("status comes from the frontmatter: %+v", p.Briefs[1])
	}
	if !strings.Contains(out, "# Not a heading in a claim") {
		t.Fatal("the claim body on the same page must keep refusing #")
	}
}

// TestRenderBoundedWith_BriefsAreChargedToTheBudget pins that brief bytes count
// against the viewer's bound: a budget the claims alone fit under is refused
// with the capacity error once the briefs are added.
func TestRenderBoundedWith_BriefsAreChargedToTheBudget(t *testing.T) {
	cat, cfg := briefViewFixture()
	at := time.Unix(1_700_000_000, 0).UTC()
	plain, err := renderAt(cat, cfg, at)
	if err != nil {
		t.Fatal(err)
	}
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("widget/long.md", "---\nsummary: Long.\n---\n"+strings.Repeat("words ", 1500)+"\n"),
	})
	budget := len(plain) + 1024
	if _, err := renderBoundedAt(cat, cfg, Extras{}, at, budget); err != nil {
		t.Fatalf("the claims alone must fit the budget: %v", err)
	}
	_, err = renderBoundedAt(cat, cfg, Extras{Briefs: set}, at, budget)
	if !errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("expected the capacity refusal once the briefs are charged, got %v", err)
	}
}
