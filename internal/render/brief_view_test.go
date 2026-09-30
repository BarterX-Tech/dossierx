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

// briefViewSet is a set whose payload is a few kilobytes: enough to move a
// budget measurably, small enough to fit an ordinary one.
func briefViewSet(cfg *config.Config) *briefs.Set {
	return briefs.FromFiles(cfg, []briefs.File{
		briefFile("widget/long.md", "---\nsummary: Long.\n---\n"+strings.Repeat("words ", 500)+"\n"),
	})
}

// TestBuildEagerShellData_ChargesTheBriefsPayloadToTheOutputBudget is the
// eager (embedded-shell) half of A8's "charged to the render byte budget", at
// the stage that does the charging: the briefs payload's bytes come off the
// shared output budget exactly, and a budget that holds the claims and the
// graph but not the payload is refused there, with the briefs-payload error,
// before any shell executes. The end-to-end test below cannot tell this from
// the final writer catching the overflow later, which is why it passed with
// the charge removed; this one fails without it.
func TestBuildEagerShellData_ChargesTheBriefsPayloadToTheOutputBudget(t *testing.T) {
	cat, cfg := briefViewFixture()
	tmpl, err := loadTemplates("")
	if err != nil {
		t.Fatal(err)
	}
	in := func(set *briefs.Set) shellInputs {
		return shellInputs{cat: cat, cfg: cfg, briefs: set, generatedAt: time.Unix(1_700_000_000, 0).UTC()}
	}
	const roomy = 1 << 30
	budget := func(n int) *renderByteBudget {
		return &renderByteBudget{remaining: n, exceeded: conformance.ErrCapacityExceeded}
	}

	claimsOnly := budget(roomy)
	if _, err := buildEagerShellData(in(nil), tmpl.partials, claimsOnly); err != nil {
		t.Fatal(err)
	}
	withBriefs := budget(roomy)
	data, err := buildEagerShellData(in(briefViewSet(cfg)), tmpl.partials, withBriefs)
	if err != nil {
		t.Fatal(err)
	}
	charged := claimsOnly.remaining - withBriefs.remaining
	if len(data.BriefsPayload) == 0 || charged != len(data.BriefsPayload) {
		t.Fatalf("the briefs payload (%d bytes) must be charged exactly; the budget moved by %d", len(data.BriefsPayload), charged)
	}

	short := budget(roomy - withBriefs.remaining - 1)
	_, err = buildEagerShellData(in(briefViewSet(cfg)), tmpl.partials, short)
	if !errors.Is(err, conformance.ErrCapacityExceeded) || !strings.Contains(err.Error(), "briefs payload") {
		t.Fatalf("a budget one byte short of claims+graph+briefs must be refused at the briefs payload, got %v", err)
	}
}

// TestLazyShell_BriefsPayload is the template-override path: a project shell
// that references {{.BriefsPayload}} gets the same bytes the embedded shell
// carries, the projection is computed once however often it is referenced,
// its bytes come off the intermediate budget exactly, and a budget smaller than
// the payload refuses it with the intermediate-capacity error.
func TestLazyShell_BriefsPayload(t *testing.T) {
	cat, cfg := briefViewFixture()
	at := time.Unix(1_700_000_000, 0).UTC()
	set := briefViewSet(cfg)
	want, err := briefsPayloadJSONWithBudget(set, nil)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeFile(t, dir+"/shell.html", `<!doctype html><script type="application/json" id="dossierx-briefs">{{.BriefsPayload}}</script>{{if .BriefsPayload}}twice{{end}}`)
	override := *cfg
	override.Viewer.TemplateOverrides = dir
	out, err := renderBoundedAt(cat, &override, Extras{Briefs: set}, at, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if m := briefsBlock.FindStringSubmatch(out); len(m) < 2 || m[1] != string(want) || !strings.Contains(out, "twice") {
		t.Fatalf("an override shell must carry the same briefs payload the embedded shell does")
	}

	tmpl, err := loadTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	in := shellInputs{cat: cat, cfg: &override, briefs: set, generatedAt: at}
	const roomy = 1 << 30
	b := &renderByteBudget{remaining: roomy, exceeded: ErrIntermediateCapacityExceeded}
	lazy := newLazyShellData(in, tmpl.partials, b)
	for i := 0; i < 2; i++ {
		got, err := lazy.BriefsPayload()
		if err != nil || got != want {
			t.Fatalf("lazy payload differs from the eager one (err %v)", err)
		}
	}
	if charged := roomy - b.remaining; charged != len(want) {
		t.Fatalf("the lazy payload (%d bytes) must be charged once, exactly; the budget moved by %d", len(want), charged)
	}

	short := &renderByteBudget{remaining: len(want) - 1, exceeded: ErrIntermediateCapacityExceeded}
	if _, err := newLazyShellData(in, tmpl.partials, short).BriefsPayload(); !errors.Is(err, ErrIntermediateCapacityExceeded) {
		t.Fatalf("a budget smaller than the payload must refuse it with the intermediate error, got %v", err)
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

// TestRenderWith_RefusesABriefsPayloadPastTheOutputCap is the unbounded path —
// RenderWith, which is what serve renders through when no claim declares an
// embodiment and there is no conformance report. Its claims are charged to no
// budget, as they never were; its briefs payload is charged to one of its own,
// conformance.MaxOutputBytes, the cap a bounded render holds the whole viewer
// to. A payload past it is refused with the capacity error on both shell
// paths, where it used to be served whole. Removing the briefs budget from the
// unbounded render fails both assertions.
//
// Each "<" in a body renders as "&lt;" and reaches the JSON payload as
// "&lt;", nine bytes, so a 7.5 MB body makes a payload past 64 MiB
// without a 64 MiB input.
func TestRenderWith_RefusesABriefsPayloadPastTheOutputCap(t *testing.T) {
	cat, cfg := briefViewFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("widget/big.md", "---\nsummary: Big.\n---\n"+strings.Repeat("<", conformance.MaxOutputBytes/9+1024)+"\n"),
	})
	want := "briefs payload requires more than 67108864 bytes"

	_, err := RenderWith(cat, cfg, Extras{Briefs: set})
	if !errors.Is(err, conformance.ErrCapacityExceeded) || !strings.Contains(err.Error(), want) {
		t.Fatalf("embedded shell: expected the briefs-payload capacity refusal, got %v", err)
	}

	dir := t.TempDir()
	writeFile(t, dir+"/shell.html", `<!doctype html><script type="application/json" id="dossierx-briefs">{{.BriefsPayload}}</script>`)
	override := *cfg
	override.Viewer.TemplateOverrides = dir
	_, err = RenderWith(cat, &override, Extras{Briefs: set})
	if !errors.Is(err, conformance.ErrCapacityExceeded) || !strings.Contains(err.Error(), want) {
		t.Fatalf("override shell: expected the briefs-payload capacity refusal, got %v", err)
	}
}
