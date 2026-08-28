package viewertests

// The claims graph PANE, in a real browser: graph-ui.js's DOM, its
// inert-until-opened contract, its freshness story, and the escaping the
// payload block depends on.
//
// What is deliberately NOT asserted here is pixels. A force layout has no
// stable pixels, and every verdict the pane draws is computed by
// graph-core.js before anything reaches the canvas — which is where
// graph_core_test.go proves it, table-driven over one page load. A wrong
// pixel here would hide no logic, and a screenshot baseline would fail on a
// font.
//
// DRAW CALLS ARE NOT PIXELS, and the cycle test below does assert those:
// which colour a ring was stroked in, which lines were drawn as cycle edges.
// It uses the recorder in graph_canvas_test.go, which explains why that is a
// different kind of assertion from a screenshot and why it is stable under a
// simulation that is still moving.
//
// CYCLES ARE PROVEN BY INJECTION, NOT BY A FIXTURE. dossierx check returns
// above the render stage on the first error-severity lint partition, and
// cycle, governed-cycle, self-edge and (from v0.5.0) mixed-cycle are all
// error severity. So no corpus that renders at all can contain a cycle of
// any shape. Because the pane parses the payload block at FIRST OPEN rather
// than at parse time, a test can replace that block's textContent before
// opening the pane: the rendered document never contained a cycle, the pane
// did. That is also the airtight proof of the lazy-parse contract itself.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------

// graphConfig is two facets in one module, so the legend has more than one
// row to list and the detail panel has a facet worth naming.
const graphConfig = `schema_version: 1
facets:
  - contract
  - design
modules:
  - widget
claims_dir: claims
`

// graphClaim writes a claim in module widget, optionally resting on another.
func graphClaim(id, facet, restsOn string) string {
	body := "id: " + id + `
facet: ` + facet + `
module: widget
status: draft
`
	if restsOn != "" {
		body += "rests_on:\n  - " + restsOn + "\n"
	}
	return body + `body: |
  a claim in the ` + facet + ` facet.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
}

// newGraphProject is the common two-claim corpus: one claim resting on
// another, one facet each, so neither is isolated and both are weakly linked.
func newGraphProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, graphConfig)
	p.writeClaim("base.yaml", graphClaim("widget.contract.base", "contract", ""))
	p.writeClaim("thing.yaml", graphClaim("widget.design.thing", "design", "widget.contract.base"))
	return p
}

// ---------------------------------------------------------------------
// Browser helpers, all composed from the existing harness
// ---------------------------------------------------------------------

// desktopViewport pins a viewport above the viewer's single 860px
// breakpoint. Below it the sidebar becomes a modal drawer whose scrim would
// intercept a real click on the graph trigger; the pane's own layout also
// stacks the rail under the canvas there. Pinning it makes both deterministic
// under any Chromium.
func desktopViewport(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.EmulateViewport(1280, 900))
	if evalBool(t, ctx, `window.matchMedia('(max-width: 860px)').matches`) {
		t.Fatal("EmulateViewport(1280) did not take effect: still matches the <=860px media query")
	}
}

// staticGraphTab renders p statically and opens the resulting file:// URL.
func staticGraphTab(t *testing.T, p *project) context.Context {
	t.Helper()
	url := p.renderStatic()
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `!!window.dossierxGraphCore`)
	desktopViewport(t, ctx)
	return ctx
}

// openGraphPane clicks the nav trigger for real and waits for the canvas.
// The click goes through graph-ui.js's ONE delegated listener on document —
// the trigger button lives inside the subtree an SSE fragment swap replaces,
// so a listener bound to the button itself would die on the first swap.
func openGraphPane(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.Click("[data-dxg-open]", chromedp.ByQuery))
	waitVisible(t, ctx, "#dxgPane .dxg-canvas")
}

// evalInto evaluates expr and decodes the result into dst.
func evalInto(t *testing.T, ctx context.Context, expr string, dst any) {
	t.Helper()
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, dst)); err != nil {
		t.Fatalf("evaluate %s: %v", expr, err)
	}
}

func evalStrings(t *testing.T, ctx context.Context, expr string) []string {
	t.Helper()
	var out []string
	evalInto(t, ctx, expr, &out)
	return out
}

// evalVoid runs an expression for its side effect only.
func evalVoid(t *testing.T, ctx context.Context, expr string) {
	t.Helper()
	runCDP(t, ctx, chromedp.Evaluate(expr, nil))
}

// jsString renders s as a JS string literal. JSON string syntax is a subset
// of JS string syntax, so json.Marshal is the correct quoter here and the one
// that cannot be got wrong by hand for a payload containing </script>.
func jsString(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("quote js string: %v", err)
	}
	return string(b)
}

// ruleIDs returns the claim ids the gaps rail lists under a rule id. Keyed off
// the STABLE rule id (data-dxg-rule), never off display text.
func ruleIDs(t *testing.T, ctx context.Context, rule string) []string {
	t.Helper()
	return evalStrings(t, ctx, `Array.from(document.querySelectorAll('[data-dxg-rule=`+jsQuote(rule)+`] [data-dxg-jump]'))
		.map(function (b) { return b.getAttribute('data-dxg-jump'); })`)
}

func jsQuote(s string) string { return "\"" + s + "\"" }

// ---------------------------------------------------------------------
// Step 70 — cycle rendering, proven by INJECTING a cycle-carrying payload
// ---------------------------------------------------------------------

// injectedCyclePayload carries all three structural shapes at once: a plain
// rests_on loop, the MIXED rests_on/governed_by loop that neither engine
// cycle lint could see before v0.5.0, and a literal self-edge — which is
// reported under its own rule id and never merged into the cycle list.
//
// core.contract.free is in no cycle and no self-edge, and it is what makes the
// CANVAS half of this test mean something: with every node ringed red, "the
// cycle members are ringed" would be true of a pane that ringed everything.
func injectedCyclePayload(t *testing.T) string {
	t.Helper()
	node := func(id string) map[string]any {
		return map[string]any{
			"id": id, "title": id, "module": "core", "facet": "contract",
			"status": "draft", "kind": "fact", "build_role": "", "emphasis": false,
			"review_pending": false, "open_comments": 0, "in_degree": 1, "out_degree": 1,
		}
	}
	payload := map[string]any{
		"schema":       1,
		"generated_at": "2026-08-05T12:00:00Z",
		"nodes": []any{
			node("core.contract.c1"), node("core.contract.c2"),
			node("core.contract.m1"), node("core.contract.m2"),
			node("core.contract.s1"), node("core.contract.free"),
		},
		"edges": []any{
			map[string]any{"from": "core.contract.c1", "to": "core.contract.c2", "type": "rests_on"},
			map[string]any{"from": "core.contract.c2", "to": "core.contract.c1", "type": "rests_on"},
			map[string]any{"from": "core.contract.m1", "to": "core.contract.m2", "type": "rests_on"},
			map[string]any{"from": "core.contract.m2", "to": "core.contract.m1", "type": "governed_by"},
			map[string]any{"from": "core.contract.s1", "to": "core.contract.s1", "type": "rests_on"},
			// Into a cycle but not part of one. Its line must NOT be drawn as
			// a cycle edge: a cycle edge is one whose endpoints share a
			// component, not one that happens to touch a member.
			map[string]any{"from": "core.contract.free", "to": "core.contract.c1", "type": "rests_on"},
		},
		"groups":  map[string]any{"modules": []any{"core"}, "facets": []any{"contract"}},
		"dropped": map[string]any{"unresolved_edges": 0},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal injected payload: %v", err)
	}
	return string(b)
}

func TestGraphPaneRendersInjectedCycles(t *testing.T) {
	p := newGraphProject(t)
	ctx := staticGraphTab(t, p)

	// BEFORE any click: swap the payload the document was rendered with for
	// one carrying cycles. This works only because the pane parses at first
	// open, which is the contract this test also proves.
	evalVoid(t, ctx, `document.getElementById('dossierx-graph').textContent = `+jsString(t, injectedCyclePayload(t))+`;`)

	installRecorder(t, ctx)
	openGraphPane(t, ctx)

	// One finding per component — two loops are two answers, not one merged
	// id list — ordered by their smallest member.
	var cycles [][]string
	evalInto(t, ctx, `Array.from(document.querySelectorAll('[data-dxg-rule="cycle"]')).map(function (block) {
		return Array.from(block.querySelectorAll('[data-dxg-jump]')).map(function (b) { return b.getAttribute('data-dxg-jump'); });
	})`, &cycles)
	want := [][]string{
		{"core.contract.c1", "core.contract.c2"},
		{"core.contract.m1", "core.contract.m2"},
	}
	if fmt.Sprint(cycles) != fmt.Sprint(want) {
		t.Fatalf("cycle blocks = %v, want %v", cycles, want)
	}

	// The self-edge is reported under its OWN rule id. The engine has a
	// dedicated error-severity self-edge lint distinct from cycle, and a rail
	// that folded the two together would tell a reader a different story than
	// dossierx check tells them.
	if got := ruleIDs(t, ctx, "self_edge"); fmt.Sprint(got) != fmt.Sprint([]string{"core.contract.s1"}) {
		t.Fatalf("self_edge ids = %v, want [core.contract.s1]", got)
	}
	// ...and it is NOT also listed as a cycle.
	for _, comp := range cycles {
		for _, id := range comp {
			if id == "core.contract.s1" {
				t.Fatal("a self-edge must never be merged into the cycle list")
			}
		}
	}

	// -----------------------------------------------------------------
	// AND NOW WHAT WAS DRAWN.
	//
	// Everything above reads values graph-core.js returned. The cycle
	// RENDERING path — the red ring on a member, the red line on an edge
	// inside a component — had never been executed by anything, and could not
	// be: all three cycle lints are error severity, so no corpus that renders
	// at all can carry a cycle, and the demo fixture legally cannot seed one.
	// The injected payload is the only way this code has ever run.
	//
	// Every assertion below is read from ONE recorded frame, so the cooling
	// simulation moving nodes between frames cannot make two of them disagree,
	// and each is anchored to a claim id through the selection ring rather
	// than to a coordinate.
	// -----------------------------------------------------------------
	settleFrames(t, ctx)

	// The negative control first: a node in no cycle wears no cycle ring.
	// core.contract.free has one outbound edge and so is the rail's
	// weakly_linked answer.
	clickJump(t, ctx, "core.contract.free")
	freeFrame := lastFrame(t, ctx)
	cycleColor := freeFrame.Pal["cycle"]
	if cycleColor == "" {
		t.Fatal("the stylesheet resolved no --dxg-cycle: every colour assertion below would be vacuous")
	}
	free := selectedNode(t, freeFrame, "core.contract.free")
	if free.ring().Color == cycleColor {
		t.Fatalf("core.contract.free is in no cycle, but its ring was drawn in the cycle colour %s", cycleColor)
	}

	// The claim itself: a cycle member is visibly ringed, in the cycle colour
	// and at the heavier cycle weight, and it is the ring — the channel that
	// survives every overlay — rather than the fill.
	clickJump(t, ctx, "core.contract.c1")
	f := lastFrame(t, ctx)
	if len(f.Nodes) != 6 {
		t.Fatalf("nodes drawn = %d, want the injected payload's 6", len(f.Nodes))
	}
	c1 := selectedNode(t, f, "core.contract.c1")
	if ring := c1.ring(); ring.Color != cycleColor || ring.Width < 2.4 {
		t.Fatalf("the ring on cycle member core.contract.c1 = %+v, want the cycle colour %s at width 2.4", ring, cycleColor)
	}

	// Five of the six wear it: both loops' members, plus the self-edge claim,
	// which is ringed by the same channel while being reported by its own rule.
	ringed := 0
	for _, n := range f.Nodes {
		if n.ring().Color == cycleColor {
			ringed++
		}
	}
	if ringed != 5 {
		t.Fatalf("nodes ringed as in-cycle = %d, want 5 of 6 (four loop members plus the self-edge claim)", ringed)
	}

	// The edges. Four are inside a component and must be drawn as cycle edges;
	// the fifth touches a member without sharing its component and must not be.
	// The self-loop is drawn by nobody: an aggregated self-loop is dropped.
	var cycleEdges, plainEdges []frameEdge
	for _, e := range f.Edges {
		if e.Color == cycleColor {
			cycleEdges = append(cycleEdges, e)
		} else {
			plainEdges = append(plainEdges, e)
		}
	}
	if len(cycleEdges) != 4 {
		t.Fatalf("edges drawn in the cycle colour = %d, want 4 (both loops, both directions)", len(cycleEdges))
	}
	if len(plainEdges) != 1 {
		t.Fatalf("edges drawn in the ordinary colour = %d, want 1 (free -> c1, which is in no component)", len(plainEdges))
	}
	for _, e := range cycleEdges {
		if e.Alpha < 0.9 {
			t.Fatalf("a cycle edge was drawn at alpha %v, want the 0.95 that lifts it out of the ordinary 0.55", e.Alpha)
		}
	}
	if plainEdges[0].Alpha > 0.6 {
		t.Fatalf("the non-cycle edge was drawn at alpha %v, want the ordinary 0.55", plainEdges[0].Alpha)
	}

	// Tied back to the id: the selected member's own edges are the red ones.
	// An edge starts exactly at its source node's centre, so this is the same
	// node the selection ring identified.
	fromC1 := 0
	for _, e := range cycleEdges {
		if nearly(e.FX, c1.X) && nearly(e.FY, c1.Y) {
			fromC1++
		}
	}
	if fromC1 != 1 {
		t.Fatalf("cycle edges leaving the selected member = %d, want 1 (c1 -> c2)", fromC1)
	}
	if nearly(plainEdges[0].FX, c1.X) && nearly(plainEdges[0].FY, c1.Y) {
		t.Fatal("the ordinary edge was drawn leaving c1; it runs INTO c1 from a node outside every cycle")
	}
}

// ---------------------------------------------------------------------
// Step 71 — the payload block parses, and the pane says how fresh it is
// ---------------------------------------------------------------------

func TestGraphPayloadParsesAndHeaderShowsTimestamp(t *testing.T) {
	p := newProjectRaw(t, graphConfig)
	p.writeClaim("base.yaml", graphClaim("widget.contract.base", "contract", ""))
	p.writeClaim("two.yaml", graphClaim("widget.contract.two", "contract", "widget.contract.base"))
	p.writeClaim("thing.yaml", graphClaim("widget.design.thing", "design", "widget.contract.base"))
	ctx := staticGraphTab(t, p)

	if !evalBool(t, ctx, `!!document.getElementById('dossierx-graph')`) {
		t.Fatal("the rendered document carries no graph payload block")
	}
	// The block parses as JSON and describes exactly this corpus.
	if n := evalInt(t, ctx, `JSON.parse(document.getElementById('dossierx-graph').textContent).nodes.length`); n != 3 {
		t.Fatalf("payload nodes = %d, want 3 (one per claim file)", n)
	}
	if v := evalInt(t, ctx, `JSON.parse(document.getElementById('dossierx-graph').textContent).schema`); v != 1 {
		t.Fatalf("payload schema = %d, want 1", v)
	}

	openGraphPane(t, ctx)

	// The header states the payload's generation time: a relative phrase for
	// the glance, the absolute value on title so the phrase is never the only
	// answer available. In a static file:// viewer the answer is simply "when
	// check ran".
	stamp := evalString(t, ctx, `document.querySelector('[data-dxg-stamp]').getAttribute('title')`)
	generated := evalString(t, ctx, `JSON.parse(document.getElementById('dossierx-graph').textContent).generated_at`)
	if stamp == "" || stamp != generated {
		t.Fatalf("header stamp title = %q, want the payload's generated_at %q", stamp, generated)
	}
	if phrase := evalString(t, ctx, `document.querySelector('[data-dxg-stamp]').textContent`); !strings.HasPrefix(phrase, "payload generated") {
		t.Fatalf("header stamp text = %q, want a 'payload generated …' phrase", phrase)
	}
}

// ---------------------------------------------------------------------
// Step 72 — a </script> in author-authored data reaches the browser as DATA
// ---------------------------------------------------------------------

// hostileFacet is the breakout string. html/template applies NO escaping
// inside <script type="application/json"> for a template.JS value, so the
// only thing standing between this and an HTML breakout is encoding/json's
// default HTML escaping — which is why SetEscapeHTML(false) is a forbidden
// call in this repository.
const hostileFacet = `</script><img src=x>`

// hostileConfig declares that facet. THIS CORPUS IS SERVED, NOT RENDERED
// STATICALLY, and that is forced rather than chosen: the id-shape lint
// requires a claim's id facet segment to equal its facet field and to be a
// configured facet, at error severity, so `dossierx check` refuses to render
// this corpus at all. `dossierx serve` never lints — it loads, builds,
// renders — which is exactly the surface design section 2.6 names as
// reachable: under serve no lint has run to constrain what an author wrote.
const hostileConfig = `schema_version: 1
facets:
  - contract
  - "</script><img src=x>"
modules:
  - widget
claims_dir: claims
`

func TestGraphPayloadSurvivesScriptClose(t *testing.T) {
	p := newProjectRaw(t, hostileConfig)
	p.writeClaim("base.yaml", graphClaim("widget.contract.base", "contract", ""))
	p.writeClaim("hostile.yaml", `id: widget.hostile.thing
facet: "</script><img src=x>"
module: widget
status: draft
body: |
  a claim whose facet is a script-closing breakout attempt.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)

	base, _ := p.serve()
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(".content-area", chromedp.ByQuery),
	)

	// The breakout never reaches the document: json.Marshal escaped it to
	// </script> before html/template ever saw it.
	if evalBool(t, ctx, `document.getElementById('dossierx-graph').textContent.indexOf('</script>') >= 0`) {
		t.Fatal("the payload block contains a literal </script>: the JSON escaping guard is gone")
	}
	// It parses, and the string survives VERBATIM as data.
	if !evalBool(t, ctx, `(function () {
		var p = JSON.parse(document.getElementById('dossierx-graph').textContent);
		return p.groups.facets.indexOf(`+jsString(t, hostileFacet)+`) >= 0;
	})()`) {
		t.Fatal("the hostile facet did not survive JSON.parse verbatim")
	}
	// And nothing became a live element.
	if n := evalInt(t, ctx, `document.images.length`); n != 0 {
		t.Fatalf("document.images.length = %d, want 0 — the injected <img> became a live element", n)
	}

	// The pane opens and draws it as an ordinary facet, named in the legend.
	desktopViewport(t, ctx)
	openGraphPane(t, ctx)
	names := evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend [data-dxg-facet]'))
		.map(function (e) { return e.getAttribute('data-dxg-facet'); })`)
	found := false
	for _, n := range names {
		if n == hostileFacet {
			found = true
		}
	}
	if !found {
		t.Fatalf("legend facets = %v, want one named %q", names, hostileFacet)
	}
	if n := evalInt(t, ctx, `document.images.length`); n != 0 {
		t.Fatalf("document.images.length = %d after opening the pane, want 0", n)
	}
}

// ---------------------------------------------------------------------
// Step 73 — inert until opened, then the whole structure of what L4 draws
// ---------------------------------------------------------------------

// parseSpy counts JSON.parse calls whose argument looks like the graph
// payload. It is installed after load and before the first click, so it
// pins that nothing parses the payload while the reader is merely sitting on
// the page. The stronger proof of the lazy-parse contract —that nothing
// parsed it at script-parse time either— is TestGraphPaneRendersInjectedCycles,
// which replaces the block's text after load and gets the new graph.
const parseSpy = `(function () {
	window.__dxgPayloadParses = 0;
	var real = JSON.parse;
	JSON.parse = function (text) {
		if (typeof text === 'string' && text.indexOf('"schema"') >= 0 && text.indexOf('"generated_at"') >= 0) {
			window.__dxgPayloadParses++;
		}
		return real.apply(JSON, arguments);
	};
})();`

func TestGraphPaneInertUntilOpened(t *testing.T) {
	p := newGraphProject(t)
	ctx := staticGraphTab(t, p)
	evalVoid(t, ctx, parseSpy)

	// Inert: a mount point and nothing else. One delegated listener and one
	// hash read is the whole cost to a reader who never opens the pane.
	if !evalBool(t, ctx, `document.getElementById('dxgPane').hidden`) {
		t.Fatal("the pane must start hidden")
	}
	if n := evalInt(t, ctx, `document.getElementById('dxgPane').children.length`); n != 0 {
		t.Fatalf("pane children before first open = %d, want 0 (nothing is built until opened)", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('.dxg-canvas').length`); n != 0 {
		t.Fatalf("canvases before first open = %d, want 0", n)
	}
	if n := evalInt(t, ctx, `window.__dxgPayloadParses`); n != 0 {
		t.Fatalf("payload parses before first open = %d, want 0", n)
	}
	// The trigger is not a .sec-tab: the viewer's existing delegated handler
	// matches .sec-tab and would also switch modules.
	if evalBool(t, ctx, `document.querySelector('[data-dxg-open]').classList.contains('sec-tab')`) {
		t.Fatal("the graph trigger must not carry class sec-tab")
	}
	// It mounts OUTSIDE div.layout, so an SSE fragment swap cannot destroy it.
	if evalBool(t, ctx, `!!document.getElementById('dxgPane').closest('.layout')`) {
		t.Fatal("the pane must mount outside div.layout")
	}

	openGraphPane(t, ctx)

	if n := evalInt(t, ctx, `window.__dxgPayloadParses`); n < 1 {
		t.Fatalf("payload parses after first open = %d, want at least 1", n)
	}

	// The control bar: six groups, in the frozen order. Scope is two of them
	// — a module axis and a facet axis, sitting where the single Scope select
	// sat — and their order relative to everything else is unchanged.
	labels := evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-controls .dxg-ctl .dxg-ctl-label'))
		.map(function (e) { return e.textContent; })`)
	wantLabels := []string{"Module", "Facet", "Granularity", "Highlight overlay", "Relationships", "View"}
	if fmt.Sprint(labels) != fmt.Sprint(wantLabels) {
		t.Fatalf("control groups = %v, want %v", labels, wantLabels)
	}
	// One independently toggleable button per relation, from graph-core.js's
	// EDGE_TYPES rather than a literal, plus the two View controls.
	types := evalStrings(t, ctx, `Array.from(document.querySelectorAll('[data-dxg-type]'))
		.map(function (e) { return e.getAttribute('data-dxg-type'); })`)
	if fmt.Sprint(types) != fmt.Sprint([]string{"rests_on", "mirrors", "governed_by"}) {
		t.Fatalf("edge-type toggles = %v, want the three relation types", types)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('[data-dxg-labels], [data-dxg-relayout]').length`); n != 2 {
		t.Fatalf("View group controls = %d, want 2 (labels toggle, re-run layout)", n)
	}

	// The overlay select carries all six overlays plus none, including
	// governance — the channel that answers "what does this doctrine reach?"
	overlays := evalStrings(t, ctx, `Array.from(document.querySelectorAll('#dxgOverlay option'))
		.map(function (o) { return o.value; })`)
	wantOverlays := []string{"none", "isolated", "cycles", "governance", "review", "comments", "status"}
	if fmt.Sprint(overlays) != fmt.Sprint(wantOverlays) {
		t.Fatalf("overlay options = %v, want %v", overlays, wantOverlays)
	}

	// The legend names every one of the PROJECT's facets, by its own name.
	// This is facet identity's second channel, and the one that still works
	// at twenty facets where colour alone has stopped working at about twelve.
	facets := evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend [data-dxg-facet] .dxg-legend-name'))
		.map(function (e) { return e.textContent; })`)
	if fmt.Sprint(facets) != fmt.Sprint([]string{"contract", "design"}) {
		t.Fatalf("legend facet names = %v, want the project's own facets", facets)
	}

	// Selecting a node fills the detail panel — facet identity's THIRD
	// channel, which names the facet in TEXT so a reader never has to resolve
	// a colour to answer "which facet is this?".
	clickJump(t, ctx, "widget.design.thing")
	if got := evalString(t, ctx, `document.querySelector('.dxg-detail-id').textContent`); got != "widget.design.thing" {
		t.Fatalf("detail panel id = %q, want widget.design.thing", got)
	}
	facetRow := evalString(t, ctx, `(function () {
		var dts = document.querySelectorAll('.dxg-detail-rows dt');
		for (var i = 0; i < dts.length; i++) {
			if (dts[i].textContent === 'facet') { return dts[i].nextElementSibling.textContent; }
		}
		return '';
	})()`)
	if facetRow != "design" {
		t.Fatalf("detail panel facet row = %q, want design", facetRow)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('[data-dxg-open-claim="widget.design.thing"]').length`); n != 1 {
		t.Fatalf("detail panel open-claim links = %d, want 1", n)
	}

	// The gaps rail keeps facts and heuristics in separate blocks, and the
	// heuristics are labelled as guesses. False positives among them are
	// guaranteed rather than merely possible.
	if n := evalInt(t, ctx, `document.querySelectorAll('[data-dxg-hints] [data-dxg-kind="hint"]').length`); n != 2 {
		t.Fatalf("hint blocks inside the hints section = %d, want 2", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('[data-dxg-hints] [data-dxg-kind="fact"]').length`); n != 0 {
		t.Fatalf("fact blocks inside the hints section = %d, want 0 — facts and guesses never mix", n)
	}

	// Escape closes the pane back to inert-but-MOUNTED: hidden again, and its
	// DOM (with the reader's camera, positions and filters) still standing.
	evalVoid(t, ctx, `document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));`)
	pollTrue(t, ctx, `document.getElementById('dxgPane').hidden === true`)
	if evalBool(t, ctx, `document.body.classList.contains('dxg-open')`) {
		t.Fatal("closing the pane must release the body scroll lock")
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('#dxgPane .dxg-canvas').length`); n != 1 {
		t.Fatalf("canvases after close = %d, want 1 — closing must not unmount", n)
	}
}

// TestGraphPaneLargeCorpusDefaultsToClaims locks the opening contract for a
// corpus large enough that the viewer used to override the reader's default
// and collapse it to modules. Every claim is now the default at every size;
// aggregation remains available as an explicit Granularity choice.
func TestGraphPaneLargeCorpusDefaultsToClaims(t *testing.T) {
	const total = 301
	p := newProjectRaw(t, `schema_version: 1
facets:
  - contract
modules:
  - m1
  - m2
  - m3
claims_dir: claims
`)
	for i := 0; i < total; i++ {
		module := fmt.Sprintf("m%d", i%3+1)
		id := fmt.Sprintf("%s.contract.c%03d", module, i)
		p.writeClaim(fmt.Sprintf("c%03d.yaml", i), "id: "+id+`
facet: contract
module: `+module+`
status: draft
body: |
  one of many claims.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	}

	ctx := staticGraphTab(t, p)
	openGraphPane(t, ctx)

	if got := evalString(t, ctx, `document.getElementById('dxgGranularity').value`); got != "claims" {
		t.Fatalf("large-corpus default granularity = %q, want claims", got)
	}
	if notice := strings.TrimSpace(evalString(t, ctx, `document.querySelector('.dxg-notices').textContent`)); notice != "" {
		t.Fatalf("large-corpus default emitted an unsolicited collapse notice: %q", notice)
	}
}

// ---------------------------------------------------------------------
// Step 74 — refresh: absent without a server, and view-preserving with one
// ---------------------------------------------------------------------

// cameraSpy makes the pane's camera observable without exporting it.
// draw() is the ONLY caller of ctx.translate/ctx.scale in graph-ui.js (it
// wraps the whole scene in one save/translate/scale/restore), so recording
// the last arguments to each is exactly the camera the last frame drew with.
// Reading it back after a forced redraw is what lets this test assert that a
// refresh preserved zoom and pan.
const cameraSpy = `(function () {
	window.__dxgCam = {};
	var proto = CanvasRenderingContext2D.prototype;
	var translate = proto.translate, scale = proto.scale;
	proto.translate = function (x, y) { window.__dxgCam.x = x; window.__dxgCam.y = y; return translate.apply(this, arguments); };
	proto.scale = function (sx) { window.__dxgCam.zoom = sx; return scale.apply(this, arguments); };
})();`

// forceDraw triggers a synchronous repaint through the pane's own resize
// handler, so the camera spy holds a value without waiting on a layout frame.
const forceDraw = `window.dispatchEvent(new Event('resize'));`

type cameraState struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

func readCamera(t *testing.T, ctx context.Context) cameraState {
	t.Helper()
	evalVoid(t, ctx, forceDraw)
	var cam cameraState
	evalInto(t, ctx, `window.__dxgCam`, &cam)
	return cam
}

func TestGraphRefresh(t *testing.T) {
	t.Run("absent in a static file viewer", func(t *testing.T) {
		p := newGraphProject(t)
		ctx := staticGraphTab(t, p)
		openGraphPane(t, ctx)

		// Not disabled — ABSENT. A control a document cannot honour is a
		// promise, not an affordance; a control that is simply not there says
		// the truth, which is that this document is a snapshot.
		if n := evalInt(t, ctx, `document.querySelectorAll('[data-dxg-refresh]').length`); n != 0 {
			t.Fatalf("refresh controls on file:// = %d, want 0 (absent, not disabled)", n)
		}
		if evalBool(t, ctx, `document.body.classList.contains('comments-live')`) {
			t.Fatal("a static file:// viewer must never report itself live")
		}
		// The close control IS there, so the absence above is about refresh
		// rather than about the header not being built.
		if n := evalInt(t, ctx, `document.querySelectorAll('[data-dxg-close]').length`); n != 1 {
			t.Fatalf("close controls = %d, want 1", n)
		}
	})

	t.Run("present under serve and preserving the view", func(t *testing.T) {
		p := newGraphProject(t)
		ctx := serveAndOpenLive(t, p)
		desktopViewport(t, ctx)
		evalVoid(t, ctx, cameraSpy)
		openGraphPane(t, ctx)

		if n := evalInt(t, ctx, `document.querySelectorAll('[data-dxg-refresh]').length`); n != 1 {
			t.Fatalf("refresh controls under serve = %d, want 1", n)
		}

		// A non-default scope on BOTH axes, then a pan and a zoom, so
		// everything a refresh must preserve is away from its default.
		evalVoid(t, ctx, `(function () {
			var m = document.getElementById('dxgModule');
			m.value = 'widget';
			m.dispatchEvent(new Event('change'));
			var f = document.getElementById('dxgFacet');
			f.value = 'contract';
			f.dispatchEvent(new Event('change'));
		})();`)
		evalVoid(t, ctx, `(function () {
			var cv = document.querySelector('.dxg-canvas');
			// Synthetic pointer events carry no active pointer, so real capture
			// would throw; the pan path under test does not depend on it.
			cv.setPointerCapture = function () {};
			cv.releasePointerCapture = function () {};
			var r = cv.getBoundingClientRect();
			var at = { clientX: r.left + 8, clientY: r.top + 8, pointerId: 1, bubbles: false };
			cv.dispatchEvent(new PointerEvent('pointerdown', at));
			cv.dispatchEvent(new PointerEvent('pointermove', { clientX: r.left + 48, clientY: r.top + 38, pointerId: 1 }));
			cv.dispatchEvent(new PointerEvent('pointercancel', { pointerId: 1 }));
			cv.dispatchEvent(new WheelEvent('wheel', { deltaY: -240, clientX: r.left + 8, clientY: r.top + 8, cancelable: true }));
		})();`)

		before := readCamera(t, ctx)
		if before.Zoom == 1 || before.Zoom == 0 {
			t.Fatalf("zoom = %v after a wheel event, want something other than the default 1", before.Zoom)
		}
		if before.X == 0 && before.Y == 0 {
			t.Fatalf("pan = (%v,%v) after a drag, want a moved camera", before.X, before.Y)
		}

		// The header timestamp is the visible proof the button did something.
		// generated_at is RFC3339 at second resolution, so a refresh inside
		// the same second legitimately produces the same string — keep asking
		// until the second turns over rather than sleeping for one.
		stamp0 := evalString(t, ctx, `document.querySelector('[data-dxg-stamp]').getAttribute('title')`)
		if stamp0 == "" {
			t.Fatal("the header carries no payload timestamp")
		}
		pollTrue(t, ctx, `(function () {
			var s = document.querySelector('[data-dxg-stamp]');
			if (s.getAttribute('title') !== `+jsString(t, stamp0)+`) { return true; }
			var btn = document.querySelector('[data-dxg-refresh]');
			if (btn && !btn.disabled) { btn.click(); }
			return false;
		})()`)

		// ...and everything the reader was looking at survived it.
		after := readCamera(t, ctx)
		if after != before {
			t.Fatalf("camera after refresh = %+v, want %+v unchanged", after, before)
		}
		if got := evalString(t, ctx, `document.getElementById('dxgModule').value`); got != "widget" {
			t.Fatalf("module scope after refresh = %q, want widget", got)
		}
		if got := evalString(t, ctx, `document.getElementById('dxgFacet').value`); got != "contract" {
			t.Fatalf("facet scope after refresh = %q, want contract", got)
		}
		if evalInt(t, ctx, `document.querySelectorAll('#dxgPane .dxg-canvas').length`) != 1 {
			t.Fatal("the pane must survive its own refresh")
		}
	})
}

// ---------------------------------------------------------------------
// Step 75 — the hash carries both halves and neither erases the other
// ---------------------------------------------------------------------

func TestGraphHashDoesNotClobberReadingView(t *testing.T) {
	p := newProjectRaw(t, twoModuleConfig)
	p.writeClaim("widget.yaml", twoModuleClaim("widget.contract.overview", "widget"))
	p.writeClaim("gadget.yaml", twoModuleClaim("gadget.contract.overview", "gadget"))
	ctx := staticGraphTab(t, p)
	openGraphPane(t, ctx)

	// On load the first module is the visible one.
	pollTrue(t, ctx, `document.querySelectorAll('.module-section').length === 2 && !document.querySelectorAll('.module-section')[0].hidden`)

	// Change a graph filter. The pane writes its segment through
	// history.replaceState ONLY, which does not fire hashchange — so the
	// reading view's routing, which falls back to the FIRST MODULE for
	// anything it does not recognise, is never re-entered.
	evalVoid(t, ctx, `(function () {
		var s = document.getElementById('dxgOverlay');
		s.value = 'cycles';
		s.dispatchEvent(new Event('change'));
	})();`)
	hash := evalString(t, ctx, `window.location.hash`)
	if !strings.Contains(hash, "!g=") || !strings.Contains(hash, "ov=cycles") {
		t.Fatalf("hash = %q, want a !g= segment carrying the graph state", hash)
	}
	if evalBool(t, ctx, `document.querySelectorAll('.module-section')[0].hidden`) {
		t.Fatal("a graph filter change must not move the reading view")
	}

	// Now paste a full deep link: a reading-view target AND a graph state.
	// Both halves must apply.
	evalVoid(t, ctx, `window.location.hash = '#gadget.contract.overview!g=md=&fc=&gr=module&ov=governance&ty=rmg&lb=1&ex=&se=';`)
	pollTrue(t, ctx, `document.getElementById('dxgOverlay').value === 'governance'`)
	pollTrue(t, ctx, `document.querySelectorAll('.module-section')[0].hidden && !document.querySelectorAll('.module-section')[1].hidden`)
	if got := evalString(t, ctx, `document.getElementById('dxgGranularity').value`); got != "module" {
		t.Fatalf("granularity from the pasted hash = %q, want module", got)
	}
	// The reading view rewrote the hash for its own target and preserved the
	// graph half byte for byte.
	if got := evalString(t, ctx, `window.location.hash`); !strings.Contains(got, "!g=") || !strings.Contains(got, "ov=governance") {
		t.Fatalf("hash after the deep link = %q, want the graph segment preserved", got)
	}
}

// ---------------------------------------------------------------------
// Step 76 — a fragment swap leaves the pane, its state and its stamp alone
// ---------------------------------------------------------------------

func TestGraphPaneSurvivesFragmentSwap(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	p.writeClaim("des.yaml", facetClaim("widget.design.thing", "design"))
	ctx := serveAndOpenLive(t, p)
	desktopViewport(t, ctx)
	openGraphPane(t, ctx)

	// Mark the pane ELEMENT itself. An expando survives only if the node
	// does; if the swap replaced it, the mark is gone with it.
	evalVoid(t, ctx, `document.getElementById('dxgPane').__dxgSurvivalMark = 'sentinel';`)
	evalVoid(t, ctx, `(function () {
		var s = document.getElementById('dxgOverlay');
		s.value = 'review';
		s.dispatchEvent(new Event('change'));
	})();`)
	stampBefore := evalString(t, ctx, `document.querySelector('[data-dxg-stamp]').getAttribute('title')`)

	// An external claim change drives an SSE "changed", the client re-fetches
	// /api/fragment and swaps <main class="content-area"> and <nav id="nav">
	// by outerHTML. The new card landing in the DOM is the deterministic
	// signal that the swap ran.
	p.writeClaim("ctr2.yaml", facetClaim("widget.contract.added", "contract"))
	pollTrue(t, ctx, `!!document.getElementById('widget.contract.added')`)

	if got := evalString(t, ctx, `document.getElementById('dxgPane').__dxgSurvivalMark || ''`); got != "sentinel" {
		t.Fatal("the pane node did not survive the fragment swap: it must mount outside div.layout")
	}
	if evalBool(t, ctx, `document.getElementById('dxgPane').hidden`) {
		t.Fatal("an open pane must stay open across a fragment swap")
	}
	if got := evalString(t, ctx, `document.getElementById('dxgOverlay').value`); got != "review" {
		t.Fatalf("overlay after the swap = %q, want review — the pane's filter state must survive", got)
	}

	// And the pane says, honestly, that its payload did NOT come along: the
	// block sits outside both swapped anchors and is never re-delivered, so
	// the stamp is unchanged even though the reading view just updated. That
	// is what the refresh button exists to answer.
	if got := evalString(t, ctx, `document.querySelector('[data-dxg-stamp]').getAttribute('title')`); got != stampBefore {
		t.Fatalf("payload stamp = %q after a swap, want %q unchanged — a swap does not re-deliver the payload", got, stampBefore)
	}
	if n := evalInt(t, ctx, `JSON.parse(document.getElementById('dossierx-graph').textContent).nodes.length`); n != 2 {
		t.Fatalf("payload nodes after the swap = %d, want the 2 it was delivered with", n)
	}
}
