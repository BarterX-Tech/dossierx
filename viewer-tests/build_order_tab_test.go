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
	"strings"
	"sync"
	"testing"

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
	pollTrue(t, ctx, `document.readyState === 'complete'`)
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

// The Build order tab is gone (NIT-15). A skip is a failure: this file
// still opens the viewer and asserts the tab is absent, so a regression
// that puts the diagrams back cannot stay green.

func TestBuildOrderTabIsGone(t *testing.T) {
	p := newBuildOrderProject(t)
	ctx, pe, _ := staticBuildOrderTab(t, p)
	desktopViewport(t, ctx)
	if evalBool(t, ctx, `!!document.getElementById('dossierx-build-order') || !!document.getElementById('dossierx-build-orders') || !!document.querySelector('.build-order-section')`) {
		t.Fatal("the Build order tab must stay absent")
	}
	if evalBool(t, ctx, `!!document.querySelector('[data-target="#dossierx-build-order"]')`) {
		t.Fatal("the sidebar must not offer a Build order entry")
	}
	if got := evalString(t, ctx, `document.querySelector('.module-section:not([hidden])').id`); got != "widget" {
		t.Fatalf("fresh load shows %q, want the first module", got)
	}
	if !evalBool(t, ctx, `!!document.getElementById('widget.contract.schema')`) {
		t.Fatal("ordinary claim cards must still render")
	}
	assertNoPageErrors(t, ctx, pe)
}
