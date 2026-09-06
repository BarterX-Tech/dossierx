package viewertests

// THE BUILD ORDER TAB, IN A REAL BROWSER.
//
// internal/render's tests pin the MARKUP of the Build order tab. What only a
// browser can show is that the vendored mermaid build turns each block's
// <pre class="mermaid"> into an SVG with one node per claim and per ghost,
// that the page's colour rules beat mermaid's own id-scoped stylesheet in BOTH
// OS colour modes, that a wide diagram scrolls inside its block while the
// page never scrolls sideways, that a deep link at load renders with no click
// and no server, and that a node click lands on the claim's card — or, for a
// claim the catalog no longer holds, on nothing. Every one of those is a
// property of a running renderer, so this is where it is proven.
//
// Nothing here skips. A browser that cannot open file:// fails the check.

import (
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------

// buildOrderConfig is four modules: widget (four claims across three phases
// with one earlier-phase edge and one cross-module edge), gadget (only an
// orientation phase, so four of its blocks are empty), single (one facet, no
// build order) and wide (twenty-two independent behavior claims, so its
// behavior diagram exceeds the column).
const buildOrderConfig = `schema_version: 1
facets:
  - contract
  - internals
modules:
  - widget
  - gadget
  - single
  - wide
claims_dir: claims
`

func boClaim(id, facet, module, role string, restsOn ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\nfacet: %s\nmodule: %s\nstatus: draft\nlayout: card\nbuild_role: %s\n", id, facet, module, role)
	if len(restsOn) > 0 {
		b.WriteString("rests_on:\n")
		for _, r := range restsOn {
			fmt.Fprintf(&b, "  - %s\n", r)
		}
	}
	fmt.Fprintf(&b, "body: |\n  the %s claim.\ngoverned_by:\n  type: none\n  reason: viewer-test fixture, not backed by any doctrine claim\n", id)
	return b.String()
}

// wideClaimCount is the number of independent behavior claims in the wide
// module: enough that mermaid's own TD output (one rank) exceeds the column.
const wideClaimCount = 22

// newBuildOrderProject writes the four-module project, locks every claim
// that takes part in an order (dependency targets first — a locked claim
// resting on an unlocked one is an error-level finding that refuses the
// lock), and proposes + locks the widget, gadget and wide orders through the
// CLI, exactly as a maintainer would.
func newBuildOrderProject(t *testing.T) *project {
	t.Helper()
	return newBuildOrderProjectWith(t, nil)
}

// extraClaim is one more claim newBuildOrderProjectWith writes and locks
// before the orders are proposed, so it is part of a locked artifact.
type extraClaim struct{ file, id, yaml string }

func newBuildOrderProjectWith(t *testing.T, extra []extraClaim) *project {
	t.Helper()
	return newBuildOrderProjectFrom(t, buildOrderConfig, extra)
}

// themedBuildOrderConfig is buildOrderConfig plus a FLAT viewer.theme that
// re-points the three tokens the diagram's shapes read — the shape of theme
// a project that predates light:/dark: writes, and the one where a literal
// and a token read look the same in light mode and diverge only in dark.
const themedBuildOrderConfig = buildOrderConfig + `viewer:
  theme:
    accent-bg: "rgba(201, 58, 129, .3)"
    card-bg: "#fff4e0"
    border: "#c08040"
`

func newBuildOrderProjectFrom(t *testing.T, config string, extra []extraClaim) *project {
	t.Helper()
	p := newProjectRaw(t, config)
	p.writeClaim("gadget-base.yaml", boClaim("gadget.contract.base", "contract", "gadget", "orientation"))
	p.writeClaim("widget-schema.yaml", boClaim("widget.contract.schema", "contract", "widget", "schema"))
	p.writeClaim("widget-behavior.yaml", boClaim("widget.contract.behavior", "contract", "widget", "behavior", "widget.contract.schema", "gadget.contract.base"))
	p.writeClaim("widget-report.yaml", boClaim("widget.internals.report", "internals", "widget", "behavior", "widget.contract.behavior"))
	p.writeClaim("widget-api.yaml", boClaim("widget.contract.api", "contract", "widget", "api", "widget.contract.behavior"))
	p.writeClaim("single-only.yaml", boClaim("single.contract.only", "contract", "single", "orientation"))
	locked := []string{"gadget.contract.base", "widget.contract.schema", "widget.contract.behavior", "widget.internals.report", "widget.contract.api"}
	for _, e := range extra {
		p.writeClaim(e.file, e.yaml)
		locked = append(locked, e.id)
	}
	for i := 0; i < wideClaimCount; i++ {
		id := fmt.Sprintf("wide.contract.step-%02d", i)
		p.writeClaim(fmt.Sprintf("wide-%02d.yaml", i), boClaim(id, "contract", "wide", "behavior"))
		locked = append(locked, id)
	}
	for _, id := range locked {
		p.run("claim", "lock", id, "--reason", "viewer-test fixture")
	}
	for _, m := range []string{"widget", "gadget", "wide"} {
		p.run("build-order", "propose", "--module", m)
		p.run("build-order", "lock", "--module", m, "--reason", "viewer-test fixture")
	}
	return p
}

// widgetSVGs is the number of non-empty phases in widget's order: schema (1
// claim), behavior (2), api (1). gadgetSVGs: orientation only.
const (
	widgetSVGs = 3
	gadgetSVGs = 1
)

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

// pageErrors collects every exception the page throws and every error- or
// warning-level console call, from before navigation. The Build order tab's
// renderer records a render rejection on window.__boErrors and re-throws it
// from a fresh task, so BOTH the exception event and the console error reach
// this; a test asserts on all three and fails on any.
type pageErrors struct {
	mu    sync.Mutex
	items []string
}

func (e *pageErrors) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.items = append(e.items, s)
}

func (e *pageErrors) snapshot() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.items...)
}

func watchPageErrors(t *testing.T, ctx context.Context) *pageErrors {
	t.Helper()
	pe := &pageErrors{}
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			text := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil {
				text += " " + e.ExceptionDetails.Exception.Description
			}
			pe.add("exception: " + text)
		case *runtime.EventConsoleAPICalled:
			if e.Type != runtime.APITypeError && e.Type != runtime.APITypeWarning {
				return
			}
			var parts []string
			for _, a := range e.Args {
				if a.Value != nil {
					parts = append(parts, string(a.Value))
				} else {
					parts = append(parts, a.Description)
				}
			}
			pe.add("console." + string(e.Type) + ": " + strings.Join(parts, " "))
		}
	})
	runCDP(t, ctx, runtime.Enable())
	return pe
}

// assertNoPageErrors fails on any recorded exception or error/warning console
// call, and on a non-empty window.__boErrors.
func assertNoPageErrors(t *testing.T, ctx context.Context, pe *pageErrors) {
	t.Helper()
	if got := pe.snapshot(); len(got) > 0 {
		t.Fatalf("the page raised %d error(s)/warning(s): %v", len(got), got)
	}
	if n := evalInt(t, ctx, `(window.__boErrors || []).length`); n != 0 {
		t.Fatalf("window.__boErrors holds %d entries: %s", n, evalString(t, ctx, `JSON.stringify(window.__boErrors)`))
	}
}

// openBuildOrderTab clicks the sidebar's Build order entry and waits for the
// section to be the visible one.
func openBuildOrderTab(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.Click(`.sec-tab[data-target="#dossierx-build-order"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `!!document.getElementById('dossierx-build-order') && !document.getElementById('dossierx-build-order').hidden`)
}

func svgCountExpr(moduleID string) string {
	return `document.querySelectorAll('#dossierx-build-order-` + moduleID + ` .bo-phase svg').length`
}

func waitDiagrams(t *testing.T, ctx context.Context, moduleID string, n int) {
	t.Helper()
	pollTrue(t, ctx, fmt.Sprintf(`%s === %d && document.querySelectorAll('#dossierx-build-order-%s .bo-diagram pre.mermaid:not([data-processed])').length === 0`, svgCountExpr(moduleID), n, moduleID))
}

func assertBuildOrderLiveDocument(t *testing.T, ctx context.Context) {
	t.Helper()
	var state struct {
		Ready, Mermaid, Payload, Section, Visible bool
	}
	evalInto(t, ctx, `(function(){var s=document.getElementById('dossierx-build-order');return {Ready:document.readyState==='complete',Mermaid:typeof window.mermaid==='object',Payload:!!document.getElementById('dossierx-build-orders'),Section:!!s,Visible:!!s&&!s.hidden};})()`, &state)
	if !state.Ready || !state.Mermaid || !state.Payload || !state.Section || !state.Visible {
		t.Fatalf("live Build order state after document-ready = %+v", state)
	}
}

func assertMissingCatalogPresentation(t *testing.T, ctx context.Context) {
	t.Helper()
	var state struct {
		Text   string
		Before bool
		SVGs   int
	}
	evalInto(t, ctx, `(function(){var m=document.querySelector('#dossierx-build-order-widget .bo-missing-claim'),p=document.querySelector('#dossierx-build-order-widget .bo-phase');return {Text:m?m.textContent:'',Before:!!m&&!!p&&!!(m.compareDocumentPosition(p)&Node.DOCUMENT_POSITION_FOLLOWING),SVGs:document.querySelectorAll('#dossierx-build-order-widget .bo-phase svg').length};})()`, &state)
	if state.Text != "Claim not found" || !state.Before || state.SVGs != widgetSVGs {
		t.Fatalf("missing-catalog presentation = %+v, want visible message before %d surviving diagrams", state, widgetSVGs)
	}
}

// staticBuildOrderTab renders p statically, opens the file:// URL with the
// error listener attached before navigation, and returns the context.
func staticBuildOrderTab(t *testing.T, p *project) (context.Context, *pageErrors, string) {
	t.Helper()
	url := p.renderStatic()
	ctx := browserContext(t)
	pe := watchPageErrors(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `!!window.mermaid`)
	desktopViewport(t, ctx)
	return ctx, pe, url
}

// tokenColourProbe resolves a custom property through a throwaway element's
// color, so both sides of every colour comparison are BROWSER-serialised: a
// custom property's getPropertyValue returns its authored text
// ("rgba(40, 112, 82, .12)", "#536179") while a computed colour property
// serialises differently ("rgba(40, 112, 82, 0.12)", "rgb(83, 97, 121)"),
// and a string comparison of the two fails on a correct implementation.
const tokenColourProbe = `function tok(name){var p=document.createElement('span');p.style.color='var('+name+')';document.body.appendChild(p);var c=getComputedStyle(p).color;p.remove();return c;}`

type diagramColours struct {
	RectFill, AccentBg, LinkStroke, Muted, GhostFill, GhostStroke, CardBg, Border string
	Locked, Links, Ghosts                                                         int
}

func readDiagramColours(t *testing.T, ctx context.Context, moduleID string) diagramColours {
	t.Helper()
	var out diagramColours
	evalInto(t, ctx, `(function(){`+tokenColourProbe+`
		var root=document.getElementById('dossierx-build-order-`+moduleID+`');
		var locked=root.querySelectorAll('.bo-diagram .node.locked_con > rect');
		var links=root.querySelectorAll('.bo-diagram .flowchart-link');
		var ghosts=root.querySelectorAll('.bo-diagram .node.ghost path');
		if(!locked.length||!links.length||!ghosts.length){return {Locked:locked.length,Links:links.length,Ghosts:ghosts.length};}
		return {Locked:locked.length,Links:links.length,Ghosts:ghosts.length,
			RectFill:getComputedStyle(locked[0]).fill, AccentBg:tok('--accent-bg'),
			LinkStroke:getComputedStyle(links[0]).stroke, Muted:tok('--muted'),
			GhostFill:getComputedStyle(ghosts[0]).fill, GhostStroke:getComputedStyle(ghosts[0]).stroke,
			CardBg:tok('--card-bg'), Border:tok('--border')};
	})()`, &out)
	return out
}

func assertDiagramColours(t *testing.T, mode string, c diagramColours) {
	t.Helper()
	if c.Locked == 0 || c.Links == 0 || c.Ghosts == 0 {
		t.Fatalf("%s: selector matched nothing (locked_con rect %d, flowchart-link %d, ghost path %d); a mermaid shape or class rename fails here by name", mode, c.Locked, c.Links, c.Ghosts)
	}
	if c.RectFill != c.AccentBg {
		t.Errorf("%s: locked node fill %q != --accent-bg %q (mermaid's id-scoped stylesheet won over the page rule)", mode, c.RectFill, c.AccentBg)
	}
	if c.LinkStroke != c.Muted {
		t.Errorf("%s: edge stroke %q != --muted %q", mode, c.LinkStroke, c.Muted)
	}
	if c.GhostFill != c.CardBg || c.GhostStroke != c.Border {
		t.Errorf("%s: ghost fill/stroke %q/%q != --card-bg/--border %q/%q", mode, c.GhostFill, c.GhostStroke, c.CardBg, c.Border)
	}
}

// dispatchNodeClick fires a bubbling click on the first g.node the selector
// finds, through the same document-level delegated listener a real click
// reaches. It returns false when the selector matched nothing.
func dispatchNodeClick(t *testing.T, ctx context.Context, sel string) bool {
	t.Helper()
	return evalBool(t, ctx, `(function(){var n=document.querySelector('`+sel+`');if(!n){return false;}n.dispatchEvent(new MouseEvent('click',{bubbles:true,cancelable:true}));return true;})()`)
}

// ---------------------------------------------------------------------
// The tab renders: six blocks per module, counts from the artifact by
// NAME, one svg per non-empty phase, nodes = claims + ghosts, edges =
// same-phase + ghost edges, cross-module listed, node ids indexed.
// ---------------------------------------------------------------------

func TestBuildOrderTabRendersEveryPhaseBlock(t *testing.T) {
	p := newBuildOrderProject(t)
	ctx, pe, url := staticBuildOrderTab(t, p)

	// A fresh file:// load with no hash lands on the first module, never on
	// the diagram tab, and so does an unresolvable hash.
	if got := evalString(t, ctx, `document.querySelector('.module-section:not([hidden])').id`); got != "widget" {
		t.Fatalf("fresh load shows %q, want the first module", got)
	}
	runCDP(t, ctx, chromedp.Navigate(url+"#not-a-real-id"))
	pollTrue(t, ctx, `!!window.mermaid`)
	if got := evalString(t, ctx, `document.querySelector('.module-section:not([hidden])').id`); got != "widget" {
		t.Fatalf("an unresolvable hash shows %q, want the first module", got)
	}
	// No per-module "Build Order" sub-tab anywhere, and the single-facet
	// module has no strip at all.
	if n := evalInt(t, ctx, `document.querySelectorAll('.module-section .sub-nav .subtab').length`); n != 2 {
		t.Fatalf("expected exactly widget's two facet subtabs, got %d", n)
	}
	if evalBool(t, ctx, `Array.from(document.querySelectorAll('.sub-nav .subtab')).some(function(b){return /build order|orientation/i.test(b.textContent);})`) {
		t.Fatal("a module facet strip still carries a Build Order / Orientation sub-tab")
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('#single .sub-nav, #single .system-created-sub-nav').length`); n != 0 {
		t.Fatalf("the single-facet module carries %d .sub-nav elements, want 0", n)
	}

	if n := evalInt(t, ctx, `document.querySelectorAll('.system-nav-group .sec-tab[data-target="#dossierx-build-order"]').length`); n != 1 {
		t.Fatalf("sidebar Build order entries = %d, want 1", n)
	}
	openBuildOrderTab(t, ctx)
	waitDiagrams(t, ctx, "widget", widgetSVGs)

	for _, m := range []string{"widget", "gadget", "wide"} {
		if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order-`+m+` .bo-phase').length`); n != 6 {
			t.Errorf("%s: %d .bo-phase blocks, want 6", m, n)
		}
	}
	// data-count per block equals the payload artifact's phase entry with the
	// MATCHING NAME (0 when absent), never by position; the excluded block
	// counts the artifact's excluded list.
	countsExpr := `(function(){
		var payload=JSON.parse(document.getElementById('dossierx-build-orders').textContent);
		var bad=[];
		payload.modules.forEach(function(m){
			var blocks=document.querySelectorAll('#dossierx-build-order-'+m.id+' .bo-phase');
			if(blocks.length!==6){bad.push(m.id+': '+blocks.length+' blocks');}
			blocks.forEach(function(b){
				var name=b.dataset.phase, want;
				if(name==='excluded'){want=(m.artifact.excluded||[]).length;}
				else{var entry=(m.artifact.phases||[]).filter(function(p){return p.phase===name;})[0];want=entry?entry.claims.length:0;}
				if(String(want)!==b.dataset.count){bad.push(m.id+'/'+name+': data-count '+b.dataset.count+' want '+want);}
				// Diagrams render lazily, per module, on first show: only the
				// visible module's blocks are held to one svg per non-empty phase.
				var visible=!document.getElementById('dossierx-build-order-'+m.id).hidden;
				var empty=want===0, hasSvg=!!b.querySelector('svg');
				if(visible&&name!=='excluded'&&empty===hasSvg){bad.push(m.id+'/'+name+': empty='+empty+' svg='+hasSvg);}
				if(visible&&name!=='excluded'&&!empty&&b.querySelectorAll('svg').length!==1){bad.push(m.id+'/'+name+': '+b.querySelectorAll('svg').length+' svgs');}
			});
		});
		return JSON.stringify(bad);
	})()`
	if got := evalString(t, ctx, countsExpr); got != "[]" {
		t.Fatalf("block counts disagree with the payload artifact: %s", got)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order .bo-error').length`); n != 0 {
		t.Fatalf(".bo-error count = %d", n)
	}
	// The phase headers: number, name, definition, counts.
	if got := evalString(t, ctx, `document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="behavior"] .bo-phase__num').textContent`); got != "phase 3 of 5" {
		t.Errorf("behavior header number = %q", got)
	}
	if got := evalString(t, ctx, `document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="behavior"] .bo-phase__meta').textContent`); got != "2 claims · 2 levels · 2 locked" {
		t.Errorf("behavior header meta = %q", got)
	}
	if got := evalString(t, ctx, `document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="verification"] .bo-empty').textContent`); got != "no claims in this module" {
		t.Errorf("empty phase text = %q", got)
	}
	if !evalBool(t, ctx, `document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="schema"] .bo-phase__def').textContent.indexOf('data-shape claims') === 0`) {
		t.Error("the schema block's definition is not BuildRoleDefinition's text")
	}

	// The behavior block: claims + ghosts nodes, same-phase + ghost edges.
	beh := `document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="behavior"]')`
	if n := evalInt(t, ctx, beh+`.querySelectorAll('svg g.node').length`); n != 3 {
		t.Errorf("behavior nodes = %d, want 3 (2 claims + 1 ghost)", n)
	}
	if n := evalInt(t, ctx, beh+`.querySelectorAll('svg .flowchart-link').length`); n != 2 {
		t.Errorf("behavior edges = %d, want 2 (1 same-phase + 1 ghost)", n)
	}
	if n := evalInt(t, ctx, beh+`.querySelectorAll('svg g.node.ghost').length`); n != 1 {
		t.Errorf("behavior ghosts = %d, want 1", n)
	}
	// Every non-ghost node's SHAPE rect (the child combinator excludes the
	// zero-width rect in every g.label) has a width; every ghost's outer path
	// has one; no label is wider than its own node's shape. All four are 0
	// inside display:none, which is the silent failure this guards.
	widthsExpr := `(function(){
		var bad=[];
		document.querySelectorAll('#dossierx-build-order-widget svg g.node').forEach(function(n){
			var ghost=n.classList.contains('ghost');
			var shape=ghost?n.querySelector('g.basic.outer-path > path'):n.querySelector(':scope > rect');
			if(!shape){bad.push(n.id+': no shape element');return;}
			var sw=shape.getBoundingClientRect().width;
			if(!(sw>0)){bad.push(n.id+': shape width '+sw);}
			var label=n.querySelector('.nodeLabel')||n.querySelector('g.label');
			if(label&&label.getBoundingClientRect().width>sw+0.5){bad.push(n.id+': label '+label.getBoundingClientRect().width+' wider than shape '+sw);}
		});
		return JSON.stringify(bad);
	})()`
	if got := evalString(t, ctx, widthsExpr); got != "[]" {
		t.Fatalf("node geometry: %s", got)
	}
	// A rendered node's id strips to a key of the payload's node_ids, and no
	// widget node maps to another module's claim.
	idsExpr := `(function(){
		var payload=JSON.parse(document.getElementById('dossierx-build-orders').textContent);
		var m=payload.modules.filter(function(x){return x.id==='widget';})[0];
		var node=document.querySelector('#dossierx-build-order-widget svg g.node');
		var key=node.id.replace(/^.*-flowchart-/,'').replace(/-\d+$/,'');
		var foreign=Object.keys(m.node_ids).filter(function(k){return m.node_ids[k].indexOf('widget.')!==0;});
		return {Key:key, Hit:Object.prototype.hasOwnProperty.call(m.node_ids,key), Foreign:foreign, Sample:node.id};
	})()`
	var ids struct {
		Key, Sample string
		Hit         bool
		Foreign     []string
	}
	evalInto(t, ctx, idsExpr, &ids)
	if !ids.Hit {
		t.Fatalf("rendered node id %q strips to %q, which is not a key of node_ids: mermaid's id scheme changed", ids.Sample, ids.Key)
	}
	if len(ids.Foreign) != 0 {
		t.Fatalf("node_ids maps to another module's claims: %v", ids.Foreign)
	}
	// Cross-module: listed, never drawn.
	if got := evalString(t, ctx, beh+`.querySelector('.bo-cross li').textContent`); !strings.Contains(got, "gadget") || !strings.Contains(got, "1 dependency") || !strings.Contains(got, "gadget.contract.base") {
		t.Errorf(".bo-cross = %q", got)
	}
	if evalBool(t, ctx, beh+`.querySelector('svg').textContent.indexOf('Base') >= 0`) {
		t.Error("the cross-module target is drawn in the behavior diagram")
	}
	// mermaid's own error graphic must not be what satisfied the svg count.
	if evalBool(t, ctx, `Array.from(document.querySelectorAll('#dossierx-build-order .bo-phase svg text')).some(function(x){return x.textContent.indexOf('Syntax error')>=0;})`) {
		t.Fatal("a diagram holds mermaid's Syntax error graphic")
	}

	// The paths that are NOT the first click: the second module's .subtab in
	// the strip.
	runCDP(t, ctx, chromedp.Click(`.bo-modules .subtab[data-target="#dossierx-build-order-gadget"]`, chromedp.ByQuery))
	waitDiagrams(t, ctx, "gadget", gadgetSVGs)
	if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order-gadget .bo-phase .bo-empty').length`); n < 4 {
		t.Errorf("gadget has %d empty blocks, want at least 4", n)
	}
	assertNoPageErrors(t, ctx, pe)

	// A deep link at load: fresh file:// load at #dossierx-build-order-gadget, no click,
	// no server — the case the parse-time render pass exists for.
	runCDP(t, ctx, chromedp.Navigate(url+"#dossierx-build-order-gadget"))
	pollTrue(t, ctx, `!!window.mermaid`)
	waitDiagrams(t, ctx, "gadget", gadgetSVGs)
	if evalBool(t, ctx, `document.getElementById('dossierx-build-order').hidden`) {
		t.Fatal("the deep link did not reveal the Build order section")
	}
	// A hashchange to the first module.
	runCDP(t, ctx, chromedp.Evaluate(`window.location.hash = '#dossierx-build-order-widget';`, nil))
	waitDiagrams(t, ctx, "widget", widgetSVGs)
	assertNoPageErrors(t, ctx, pe)
}

// ---------------------------------------------------------------------
// Width: a real rendered block wider than the column scrolls inside its
// own frame, the SVG is never scaled to fit, and the page never scrolls
// sideways. Plus a 25-button module strip that scrolls and a page that does
// not; plus the phone case for the gutter override.
// ---------------------------------------------------------------------

func TestBuildOrderTabWideDiagramScrollsInsideItsBlock(t *testing.T) {
	p := newBuildOrderProject(t)
	ctx, pe, url := staticBuildOrderTab(t, p)
	runCDP(t, ctx, chromedp.Navigate(url+"#dossierx-build-order-wide"))
	pollTrue(t, ctx, `!!window.mermaid`)
	waitDiagrams(t, ctx, "wide", 1)

	var m struct {
		MaxWidth                                          string
		SVGWidth, DiagramClient, DiagramScroll, DocScroll float64
		DocClient                                         float64
		Nodes                                             int
	}
	evalInto(t, ctx, `(function(){
		var d=document.querySelector('#dossierx-build-order-wide .bo-phase[data-phase="behavior"] .bo-diagram');
		var svg=d.querySelector('svg');
		return {MaxWidth:getComputedStyle(svg).maxWidth, SVGWidth:svg.getBoundingClientRect().width, DiagramClient:d.clientWidth, DiagramScroll:d.scrollWidth,
			DocScroll:document.documentElement.scrollWidth, DocClient:document.documentElement.clientWidth, Nodes:d.querySelectorAll('g.node').length};
	})()`, &m)
	if m.Nodes != wideClaimCount {
		t.Fatalf("wide behavior diagram holds %d nodes, want %d", m.Nodes, wideClaimCount)
	}
	if m.MaxWidth != "none" {
		t.Errorf("svg max-width = %q, want none (mermaid's inline max-width would scale it to fit)", m.MaxWidth)
	}
	if !(m.SVGWidth > m.DiagramClient) {
		t.Errorf("svg width %.0f is not wider than its block %.0f: the diagram was scaled down, or the fixture is not wide enough to test anything", m.SVGWidth, m.DiagramClient)
	}
	if !(m.DiagramScroll > m.DiagramClient) {
		t.Errorf("block scrollWidth %.0f <= clientWidth %.0f: the block does not scroll", m.DiagramScroll, m.DiagramClient)
	}
	if m.DocScroll > m.DocClient {
		t.Errorf("the page scrolls sideways: document scrollWidth %.0f > clientWidth %.0f", m.DocScroll, m.DocClient)
	}
	// Desktop: the facet-TOC rail is reclaimed on this tab. .content-area
	// transitions its padding over 180ms, so the end state is polled for
	// rather than read once mid-transition.
	pollTrue(t, ctx, `getComputedStyle(document.querySelector('.content-area')).paddingRight === '42px'`)
	assertNoPageErrors(t, ctx, pe)

	// Mobile: the :has() override is width-scoped, so both gutters are the
	// phone rule's 16px (unscoped, the right one would be 42px beside a 16px
	// left).
	runCDP(t, ctx, chromedp.EmulateViewport(375, 812))
	pollTrue(t, ctx, `window.matchMedia('(max-width: 860px)').matches`)
	pollTrue(t, ctx, `(function(){var s=getComputedStyle(document.querySelector('.content-area'));return s.paddingLeft==='16px'&&s.paddingRight==='16px';})()`)
}

// manyModulesConfig is 26 modules, 25 of which lock an order.
func manyModulesProject(t *testing.T) *project {
	t.Helper()
	var cfg strings.Builder
	cfg.WriteString("schema_version: 1\nfacets:\n  - contract\nmodules:\n")
	for i := 1; i <= 26; i++ {
		fmt.Fprintf(&cfg, "  - m%02d\n", i)
	}
	cfg.WriteString("claims_dir: claims\n")
	p := newProjectRaw(t, cfg.String())
	for i := 1; i <= 26; i++ {
		mod := fmt.Sprintf("m%02d", i)
		id := mod + ".contract.only"
		p.writeClaim(mod+".yaml", boClaim(id, "contract", mod, "schema"))
		if i == 26 {
			continue
		}
		p.run("claim", "lock", id, "--reason", "viewer-test fixture")
		p.run("build-order", "propose", "--module", mod)
		p.run("build-order", "lock", "--module", mod, "--reason", "viewer-test fixture")
	}
	return p
}

func TestBuildOrderTabModuleStripScrollsNotThePage(t *testing.T) {
	p := manyModulesProject(t)
	ctx, pe, _ := staticBuildOrderTab(t, p)
	openBuildOrderTab(t, ctx)
	waitDiagrams(t, ctx, "m01", 1)
	var m struct {
		Buttons                                        int
		StripScroll, StripClient, DocScroll, DocClient float64
	}
	evalInto(t, ctx, `(function(){var s=document.querySelector('.bo-modules');return {Buttons:s.querySelectorAll('.subtab').length,StripScroll:s.scrollWidth,StripClient:s.clientWidth,DocScroll:document.documentElement.scrollWidth,DocClient:document.documentElement.clientWidth};})()`, &m)
	if m.Buttons != 25 {
		t.Fatalf("strip holds %d buttons, want 25", m.Buttons)
	}
	if !(m.StripScroll > m.StripClient) {
		t.Errorf("strip scrollWidth %.0f <= clientWidth %.0f: 25 buttons fit, so overflow is untested — or the strip wraps", m.StripScroll, m.StripClient)
	}
	if m.DocScroll > m.DocClient {
		t.Errorf("the page scrolls sideways: %.0f > %.0f", m.DocScroll, m.DocClient)
	}
	assertNoPageErrors(t, ctx, pe)
}

// ---------------------------------------------------------------------
// Colours: in both OS modes the rendered shapes carry the page's tokens,
// not mermaid's base theme; the flip WITHOUT a reload re-renders from the
// stashed source; print pins the marker colour and a transparent backdrop.
// ---------------------------------------------------------------------

func TestBuildOrderTabColoursFollowTheTokensInBothModes(t *testing.T) {
	// A second internals leaf in widget's behavior phase, so the block keeps
	// a locked_int node once report goes back to draft below.
	p := newBuildOrderProjectWith(t, []extraClaim{{
		file: "widget-audit.yaml", id: "widget.internals.audit",
		yaml: boClaim("widget.internals.audit", "internals", "widget", "behavior", "widget.contract.behavior"),
	}})
	// Two leaf claims (nothing rests on either) go back to draft AFTER the
	// order is locked, so the widget diagrams draw a real draft_con (api)
	// and draft_int (internals.report) node beside the locked ones and every
	// one of the five node classes is asserted on mermaid's own output. The
	// order is then stale, which check reports as a hint, not a refusal.
	p.run("claim", "unlock", "widget.contract.api", "--reason", "viewer-test fixture: a draft_con node")
	p.run("claim", "unlock", "widget.internals.report", "--reason", "viewer-test fixture: a draft_int node")
	ctx, pe, url := staticBuildOrderTab(t, p)

	for _, scheme := range []string{"light", "dark"} {
		emulateColorScheme(t, ctx, scheme)
		runCDP(t, ctx, chromedp.Navigate(url+"#dossierx-build-order-widget"))
		pollTrue(t, ctx, `!!window.mermaid`)
		waitDiagrams(t, ctx, "widget", widgetSVGs)
		// The selectors are asserted before their styles are read, for every
		// one of the five node classes and the edge, on the RENDERED
		// diagrams, so a mermaid shape-element or class rename fails by name
		// instead of on a null element.
		for _, sel := range []string{
			".bo-diagram .flowchart-link", ".bo-diagram .node.locked_con rect", ".bo-diagram .node.draft_con rect",
			".bo-diagram .node.locked_int rect", ".bo-diagram .node.draft_int rect", ".bo-diagram .node.ghost path",
		} {
			if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order-widget `+sel+`').length`); n == 0 {
				t.Fatalf("%s: selector %q matches nothing", scheme, sel)
			}
		}
		assertDiagramColours(t, scheme, readDiagramColours(t, ctx, "widget"))
		// The draft strokes, read from the rendered draft nodes.
		var d struct{ Con, Accent, Int, Link string }
		evalInto(t, ctx, `(function(){`+tokenColourProbe+`
			var c=document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="api"] .node.draft_con > rect');
			var i=document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="behavior"] .node.draft_int > rect');
			if(!c||!i){return {Con:c?'':'no draft_con rect in the api block', Int:i?'':'no draft_int rect in the behavior block'};}
			return {Con:getComputedStyle(c).stroke, Accent:tok('--accent'), Int:getComputedStyle(i).stroke, Link:tok('--link')};
		})()`, &d)
		if d.Con != d.Accent || d.Int != d.Link {
			t.Errorf("%s: draft strokes %q/%q != --accent/--link %q/%q", scheme, d.Con, d.Int, d.Accent, d.Link)
		}
	}

	// Theme flip WITHOUT reload: from light, flip to dark, wait two frames,
	// and the diagrams are re-rendered from the stashed source with the dark
	// palette — the only path the matchMedia listener exists for.
	emulateColorScheme(t, ctx, "light")
	runCDP(t, ctx, chromedp.Navigate(url+"#dossierx-build-order-widget"))
	pollTrue(t, ctx, `!!window.mermaid`)
	waitDiagrams(t, ctx, "widget", widgetSVGs)
	light := readDiagramColours(t, ctx, "widget")
	emulateColorScheme(t, ctx, "dark")
	runCDP(t, ctx, chromedp.Evaluate(`new Promise(function(r){requestAnimationFrame(function(){requestAnimationFrame(r);});})`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }))
	waitDiagrams(t, ctx, "widget", widgetSVGs)
	if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order .bo-error').length`); n != 0 {
		t.Fatalf("after the flip .bo-error count = %d (mermaid was fed its own SVG?)", n)
	}
	dark := readDiagramColours(t, ctx, "widget")
	assertDiagramColours(t, "flip-to-dark", dark)
	if dark.AccentBg == light.AccentBg {
		t.Fatalf("the dark --accent-bg resolved to the light value %q; the flip did not take", dark.AccentBg)
	}
	assertNoPageErrors(t, ctx, pe)

	// Print: the marker's fill/stroke equal --muted resolved under print and
	// the svg backdrop is transparent.
	runCDP(t, ctx, emulation.SetEmulatedMedia().WithMedia("print"))
	pollTrue(t, ctx, `window.matchMedia('print').matches`)
	var pr struct{ Fill, Stroke, Muted, Background string }
	evalInto(t, ctx, `(function(){`+tokenColourProbe+`
		var m=document.querySelector('#dossierx-build-order-widget .bo-diagram marker path');
		var svg=document.querySelector('#dossierx-build-order-widget .bo-diagram svg');
		if(!m||!svg){return {Fill:'no marker path'};}
		return {Fill:getComputedStyle(m).fill, Stroke:getComputedStyle(m).stroke, Muted:tok('--muted'), Background:getComputedStyle(svg).backgroundColor};
	})()`, &pr)
	if pr.Fill != pr.Muted || pr.Stroke != pr.Muted {
		t.Errorf("print marker fill/stroke %q/%q != --muted %q", pr.Fill, pr.Stroke, pr.Muted)
	}
	if pr.Background != "rgba(0, 0, 0, 0)" {
		t.Errorf("print svg background = %q, want transparent", pr.Background)
	}
	// The printed page names its module: the strip button for the selected
	// module is the only element carrying the label (the section has no
	// heading), so it must stay displayed under print while the other
	// modules' buttons are dropped; and the print block's `.content-area
	// { padding: 0 }` must win over the screen-scoped 42px gutter.
	// .content-area transitions its padding over 180ms, so the end state is
	// polled for before the snapshot below reads it.
	pollTrue(t, ctx, `getComputedStyle(document.querySelector('.content-area')).paddingRight === '0px'`)
	var pp struct {
		Strip, On, Off, Label, PaddingRight string
		OffCount                            int
	}
	evalInto(t, ctx, `(function(){
		var strip=document.querySelector('#dossierx-build-order .bo-modules');
		var on=strip.querySelector('.subtab.on'), off=strip.querySelectorAll('.subtab:not(.on)');
		var offShown=0; off.forEach(function(b){ if(getComputedStyle(b).display!=='none'){offShown++;} });
		return {Strip:getComputedStyle(strip).display, On:on?getComputedStyle(on).display:'no .subtab.on', Off:String(off.length), OffCount:offShown,
			Label:on?on.textContent.trim():'', PaddingRight:getComputedStyle(document.querySelector('.content-area')).paddingRight};
	})()`, &pp)
	if pp.Strip == "none" || pp.On == "none" || pp.On == "no .subtab.on" || pp.Label != "Widget" {
		t.Errorf("print: the module strip does not name the module (strip display %q, selected button display %q, label %q)", pp.Strip, pp.On, pp.Label)
	}
	if pp.Off != "2" || pp.OffCount != 0 {
		t.Errorf("print: %d of %s unselected module buttons are still displayed", pp.OffCount, pp.Off)
	}
	if pp.PaddingRight != "0px" {
		t.Errorf("print: .content-area padding-right = %q, want 0px (the screen gutter leaked onto paper)", pp.PaddingRight)
	}
	runCDP(t, ctx, emulation.SetEmulatedMedia().WithMedia(""))

	// Themed project: a flat viewer.theme re-points --accent-bg, --card-bg
	// and --border for both schemes. The same assertions run in both OS
	// modes, and the themed --accent-bg must differ from the default
	// project's in each, so a rule that fell back to the engine literal
	// (which equals the token in an unthemed light page) fails here by
	// value rather than passing by coincidence.
	tp := newBuildOrderProjectFrom(t, themedBuildOrderConfig, nil)
	tctx, tpe, turl := staticBuildOrderTab(t, tp)
	defaults := map[string]string{"light": light.AccentBg, "dark": dark.AccentBg}
	for _, scheme := range []string{"light", "dark"} {
		emulateColorScheme(t, tctx, scheme)
		runCDP(t, tctx, chromedp.Navigate(turl+"#dossierx-build-order-widget"))
		pollTrue(t, tctx, `!!window.mermaid`)
		waitDiagrams(t, tctx, "widget", widgetSVGs)
		themed := readDiagramColours(t, tctx, "widget")
		assertDiagramColours(t, "themed-"+scheme, themed)
		if themed.AccentBg == defaults[scheme] {
			t.Errorf("themed %s: --accent-bg resolved to the default %q; viewer.theme did not reach the page", scheme, themed.AccentBg)
		}
	}
	assertNoPageErrors(t, tctx, tpe)
}

// ---------------------------------------------------------------------
// Node click: a hit navigates to the claim's card; a miss (the claim is no
// longer in the catalog, which a locked artifact permits) does NOT navigate
// and marks the node.
// ---------------------------------------------------------------------

func TestBuildOrderTabNodeClickHitAndMiss(t *testing.T) {
	p := newBuildOrderProject(t)
	ctx, pe, url := staticBuildOrderTab(t, p)
	runCDP(t, ctx, chromedp.Navigate(url+"#dossierx-build-order-widget"))
	pollTrue(t, ctx, `!!window.mermaid`)
	waitDiagrams(t, ctx, "widget", widgetSVGs)

	if !dispatchNodeClick(t, ctx, `#dossierx-build-order-widget .bo-phase[data-phase="schema"] g.node.locked_con`) {
		t.Fatal("no locked_con node in the schema block to click")
	}
	pollTrue(t, ctx, `window.location.hash === '#widget.contract.schema'`)
	pollTrue(t, ctx, `!document.getElementById('widget').hidden && !!document.getElementById('widget.contract.schema') && document.getElementById('widget.contract.schema').getBoundingClientRect().height > 0`)
	if !evalBool(t, ctx, `document.getElementById('dossierx-build-order').hidden`) {
		t.Fatal("the diagram tab stayed visible after a node click")
	}
	assertNoPageErrors(t, ctx, pe)

	// The miss. Delete a leaf claim's file AFTER locking (the artifact stays
	// locked) and re-render. check refuses a project whose ledger names a
	// claim that is gone (lint dangling + ledger), so the re-render is the
	// served page, which loads and renders without linting.
	if err := os.Remove(filepath.Join(p.claimsDir, "widget-api.yaml")); err != nil {
		t.Fatalf("remove claim: %v", err)
	}
	base := p.ensureServe()
	ctx2 := browserContext(t)
	pe2 := watchPageErrors(t, ctx2)
	requests2 := watchRequests(t, ctx2)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("live Build order document requests: %v", requests2.fromDocument(base+"/"))
			t.Logf("live Build order page errors: %v", pe2.snapshot())
		}
	})
	runCDP(t, ctx2, chromedp.Navigate(base+"/#dossierx-build-order-widget"))
	// The served document carries a multi-megabyte Mermaid bundle inline. Wait
	// for the browser's document-ready boundary before observing its exports;
	// an immediate poll races parsing/execution even though the response already
	// contains the Build order section, payload, and scripts.
	pollTrue(t, ctx2, `document.readyState === 'complete'`)
	assertBuildOrderLiveDocument(t, ctx2)
	desktopViewport(t, ctx2)
	waitDiagrams(t, ctx2, "widget", widgetSVGs)
	assertMissingCatalogPresentation(t, ctx2)

	assertMissingBuildOrderNode := func() {
		t.Helper()
		if !dispatchNodeClick(t, ctx2, `#dossierx-build-order-widget .bo-phase[data-phase="api"] g.node.draft_con`) {
			t.Fatal("no draft_con node in the api block: a claim gone from the catalog must still draw, as not locked")
		}
		pollTrue(t, ctx2, `!!document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="api"] g.node.bo-missing')`)
		if got := evalString(t, ctx2, `window.location.hash`); got != "#dossierx-build-order-widget" {
			t.Fatalf("a miss changed the hash to %q", got)
		}
		if evalBool(t, ctx2, `document.getElementById('dossierx-build-order').hidden`) {
			t.Fatal("a miss switched module")
		}
		if got := evalString(t, ctx2, `document.querySelector('#dossierx-build-order-widget .bo-phase[data-phase="api"] g.node.bo-missing').getAttribute('title')`); !strings.Contains(got, "no longer in the catalog") {
			t.Errorf("miss title = %q", got)
		}
	}
	// The api claim is gone from the catalog: its node draws (the artifact
	// still lists it) as not locked, and clicking it is a miss.
	assertMissingBuildOrderNode()

	// A real refresh must preserve the same complete document contract and the
	// same honest miss behavior; otherwise the first-load assertion could be
	// passing only because the tab was already warm.
	runCDP(t, ctx2, chromedp.Reload())
	pollTrue(t, ctx2, `document.readyState === 'complete'`)
	assertBuildOrderLiveDocument(t, ctx2)
	desktopViewport(t, ctx2)
	waitDiagrams(t, ctx2, "widget", widgetSVGs)
	assertMissingCatalogPresentation(t, ctx2)
	assertMissingBuildOrderNode()

	// A live fragment swap must preserve the same honest missing-claim message
	// and surviving graph. Add an unrelated claim so the swapped DOM has a
	// deterministic witness that this was an SSE /api/fragment update.
	p.writeClaim("single-extra.yaml", boClaim("single.contract.extra", "contract", "single", "orientation"))
	pollTrue(t, ctx2, `!!document.getElementById('single.contract.extra')`)
	assertBuildOrderLiveDocument(t, ctx2)
	waitDiagrams(t, ctx2, "widget", widgetSVGs)
	assertMissingCatalogPresentation(t, ctx2)
	assertNoPageErrors(t, ctx2, pe2)
}

// ---------------------------------------------------------------------
// The walk: every module the page's payload lists, clicked in turn, with six
// blocks, one svg per non-empty phase, no errors, and no request leaving the
// document's own origin. Defaults to the local four-module project over
// file://; DOSSIERX_TEST_VIEWER_URL points it at a served page (the
// reference client's, in the release verification) and adds the 25-module
// floor. It never skips.
// ---------------------------------------------------------------------

func TestBuildOrderTabWalksEveryModule(t *testing.T) {
	url := os.Getenv("DOSSIERX_TEST_VIEWER_URL")
	external := url != ""
	if !external {
		url = newBuildOrderProject(t).renderStatic()
	}
	const deadline = 10 * time.Minute
	browser := resolveBrowser(t)
	allocMu.Lock()
	if allocCtx == nil {
		allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), browserAllocOpts(browser)...)
	}
	shared := allocCtx
	allocMu.Unlock()
	tabCtx, cancelTab := chromedp.NewContext(shared)
	t.Cleanup(cancelTab)
	ctx, cancel := context.WithTimeout(tabCtx, deadline)
	t.Cleanup(cancel)

	pe := watchPageErrors(t, ctx)
	log := watchRequests(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `!!window.mermaid && !!document.getElementById('dossierx-build-orders')`)
	desktopViewport(t, ctx)
	openBuildOrderTab(t, ctx)

	var modules []struct {
		ID       string `json:"id"`
		NonEmpty int    `json:"non_empty"`
	}
	evalInto(t, ctx, `JSON.parse(document.getElementById('dossierx-build-orders').textContent).modules.map(function(m){return {id:m.id, non_empty:m.phase_views.filter(function(v){return v.number!==0&&v.claims.length>0;}).length};})`, &modules)
	if len(modules) == 0 {
		t.Fatal("the payload lists no module; the walk would assert nothing")
	}
	walked := 0
	for _, m := range modules {
		runCDP(t, ctx, chromedp.Click(`.bo-modules .subtab[data-target="#dossierx-build-order-`+m.ID+`"]`, chromedp.ByQuery))
		waitDiagrams(t, ctx, m.ID, m.NonEmpty)
		if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order-`+m.ID+` .bo-phase').length`); n != 6 {
			t.Errorf("%s: %d blocks, want 6", m.ID, n)
		}
		if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order-`+m.ID+` .bo-error').length`); n != 0 {
			t.Errorf("%s: %d .bo-error", m.ID, n)
		}
		walked++
	}
	assertNoPageErrors(t, ctx, pe)
	mine := log.fromDocument(url)
	if len(mine) == 0 {
		t.Fatalf("no request was attributed to %s; the request assertion below would be vacuous", url)
	}
	origin := ""
	if external {
		u, err := neturl.Parse(url)
		if err != nil {
			t.Fatalf("DOSSIERX_TEST_VIEWER_URL %q: %v", url, err)
		}
		origin = u.Scheme + "://" + u.Host
	}
	for _, u := range mine {
		if external {
			if !strings.HasPrefix(u, origin) {
				t.Errorf("a request left the served origin %s: %s", origin, u)
			}
		} else if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			t.Errorf("a file:// viewer issued a network request: %s", u)
		}
	}
	source := "the local project"
	if external {
		source = "DOSSIERX_TEST_VIEWER_URL"
		if walked < 25 {
			t.Errorf("walked %d module(s) from DOSSIERX_TEST_VIEWER_URL, want at least 25", walked)
		}
	}
	t.Logf("walked %d module(s) from %s (deadline %s)", walked, source, deadline)
}
