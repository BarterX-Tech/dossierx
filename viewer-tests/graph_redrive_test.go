package viewertests

// THE THREE FIXES THE RE-DRIVE REFUSED TO SIGN OFF, PINNED IN A BROWSER.
//
// e622acf changed graph-ui.js and graph.css and touched no test file. A
// mutation pass over the suite that shipped with it found three behaviours
// that could be deleted from the source with every existing test still green:
//
//   1. openFromHashOnLoad() — a deep link that arrives WITH the document.
//      TestGraphHashDoesNotClobberReadingView looks like it covers this and
//      does not: it ASSIGNS window.location.hash after load, which fires
//      hashchange and is served by the pre-existing bindHashListener. The
//      case the fix exists for — a pasted URL that is already in the address
//      bar when the document loads — is the one case hashchange never fires
//      for, so nothing in the suite reached it.
//
//      THIS ONE IS RED, AND CORRECTLY SO. Writing it found that the fix has
//      never run: openFromHashOnLoad() is called from bindWindow(), which is
//      called only from mountPane(), which is called only from openPane() —
//      so the only way to reach the load-time check is to have already done
//      the thing it decides whether to do. Worse, when it IS reached, it
//      re-enters openPane() before mountPane() has set `mounted`, and the
//      recursion blows the stack: on any page whose hash carries a graph
//      segment, clicking the graph trigger throws RangeError and leaves a
//      half-built, permanently hidden pane. The reader the fix was written
//      for — someone opening a shared link — is the one reader who now
//      cannot open the pane at all. The failure message below carries that
//      diagnosis. The fix belongs in graph-ui.js and is not this file's to
//      make; VERIFIED that this test goes green the moment the call is moved
//      out of bindWindow() and armed on DOMContentLoaded instead.
//
//   2. The requestFit() calls in expandGroup and collapseGroup. Scope and
//      granularity already asked for a fit; drill-down and collapse change
//      which graph is drawn in exactly the same way and did not, so the
//      camera stayed framed on the folded view and the newly spread claims
//      were drawn 28px off the top of the canvas and 17px off the bottom.
//
//   3. The light-theme contrast of two graph controls. There was no contrast
//      assertion of any kind anywhere in this suite, so reverting both tokens
//      to the accent-derived values that measured 2.07:1 and 2.64:1 changed
//      nothing that anything checked.
//
// The canvas frame recorder in graph_canvas_test.go is the machinery for (2):
// one recorded frame's node positions, projected through that same frame's
// camera, are the drawn ink, and fitToView's contract is a statement about
// exactly that box.

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// =====================================================================
// 1 — A DEEP LINK THAT ARRIVES WITH THE DOCUMENT OPENS THE PANE
// =====================================================================

// deepLinkHash is a full shared link: a reading-view target, then the graph
// segment. Every graph field in it differs from defaultState() —
// granularity claims -> module, overlay none -> governance, labels on -> off,
// all three edge types -> two, nothing selected -> a node, facet scope all ->
// contract — so "the pane restored this state" cannot be satisfied by a pane
// that simply opened on its defaults.
//
// The MODULE axis is the one field deliberately left at its default, because
// the selection assertion below counts the nodes the canvas drew and wants
// both module groups. The facet axis carries the scope half of the proof
// instead: this fixture's two claims are both in facet `contract`, so
// `fc=contract` is a real, restored, non-default selection that still leaves
// both modules on screen. Leaving BOTH axes at their default would have made
// this test blind to a codec that dropped scope entirely.
const deepLinkHash = "#gadget.contract.overview" +
	"!g=md=&fc=contract&gr=module&ov=governance&ty=rm&lb=0&se=module%3Awidget&ex="

// deepLinkSelected is the node the link says was selected. It is a GROUP id
// because the link also says granularity=module, and the selection is asserted
// on the canvas — a restored selection sets state and draws a ring, but
// renders no detail panel until the reader clicks something, which is a
// separate gap and not one of the three fixes here.
const deepLinkSelected = "module:widget"

// THE ONE THING THAT MUST NOT BE DONE HERE IS ASSIGN THE HASH AFTER LOAD.
// That is the blind spot this test closes: an assignment fires hashchange,
// which bindHashListener has always handled. The hash has to be part of the
// navigation, so the document arrives with it and no hashchange ever fires.
//
// The negative control — a load with NO graph segment leaves the pane hidden,
// unmounted and unparsed — is TestGraphPaneInertUntilOpened, which asserts
// exactly that on a plain load. Without it, a pane that opened itself
// unconditionally would pass this test.
func TestGraphDeepLinkOnLoadOpensAndRestoresThePane(t *testing.T) {
	p := newProjectRaw(t, twoModuleConfig)
	p.writeClaim("widget.yaml", twoModuleClaim("widget.contract.overview", "widget"))
	p.writeClaim("gadget.yaml", twoModuleClaim("gadget.contract.overview", "gadget"))
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(url+deepLinkHash))
	pollTrue(t, ctx, `!!window.dossierxGraphCore`)

	// The hash was carried by the navigation, and nothing has touched it since:
	// no hashchange can have fired, so whatever opens the pane below did so by
	// looking at the hash on load.
	if got := evalString(t, ctx, `window.location.hash`); !strings.Contains(got, "!g=") {
		t.Fatalf("the document loaded with hash %q, want the deep link's graph segment intact", got)
	}

	// IT OPENED BY ITSELF. No click on [data-dxg-open] happens anywhere in
	// this test.
	//
	// A bounded settle and then a read-once assertion, so a pane that never
	// opens fails through a message that names the state it is actually in
	// rather than through a 20s poll that names nothing.
	settleFor(t, ctx, `document.getElementById('dxgPane').hidden === false`)
	if evalBool(t, ctx, `document.getElementById('dxgPane').hidden`) {
		t.Fatalf("the pane is still hidden %v after loading a URL whose hash already carried a !g= graph segment "+
			"(pane children = %d, canvases = %d, body.dxg-open = %v).\n"+
			"  A hash that arrives WITH the document fires no hashchange, so bindHashListener cannot see it and "+
			"openFromHashOnLoad() is the only thing that can. %s",
			settleTimeout,
			evalInt(t, ctx, `document.getElementById('dxgPane').children.length`),
			evalInt(t, ctx, `document.querySelectorAll('.dxg-canvas').length`),
			evalBool(t, ctx, `document.body.classList.contains('dxg-open')`),
			diagnoseTriggerOpen(t, ctx))
	}
	waitVisible(t, ctx, "#dxgPane .dxg-canvas")
	if !evalBool(t, ctx, `document.body.classList.contains('dxg-open')`) {
		t.Fatal("the pane reports itself open but the body scroll lock was never taken")
	}
	desktopViewport(t, ctx)

	// ...and what it restored is the link's state, field by field.
	cases := []struct {
		name string
		expr string
		want string
	}{
		{"module scope", `document.getElementById('dxgModule').value`, ""},
		{"facet scope", `document.getElementById('dxgFacet').value`, "contract"},
		{"granularity", `document.getElementById('dxgGranularity').value`, "module"},
		{"overlay", `document.getElementById('dxgOverlay').value`, "governance"},
		{"labels toggle", `document.querySelector('[data-dxg-labels]').getAttribute('aria-pressed')`, "false"},
		{"rests_on toggle", `document.querySelector('[data-dxg-type="rests_on"]').getAttribute('aria-pressed')`, "true"},
		{"mirrors toggle", `document.querySelector('[data-dxg-type="mirrors"]').getAttribute('aria-pressed')`, "true"},
		{"governed_by toggle", `document.querySelector('[data-dxg-type="governed_by"]').getAttribute('aria-pressed')`, "false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evalString(t, ctx, tc.expr); got != tc.want {
				t.Fatalf("%s restored from the deep link = %q, want %q — the pane opened but did not restore the link's state", tc.name, got, tc.want)
			}
		})
	}

	// The seventh field is not a control, so it is read off the canvas: the
	// node the link named is the one wearing the selection ring. The recorder
	// is installed AFTER the load here (it cannot be installed before a pane
	// that opens during the load) and lastFrame forces the repaint it reads.
	t.Run("selection", func(t *testing.T) {
		installRecorder(t, ctx)
		settleFrames(t, ctx)
		f := lastFrame(t, ctx)
		if len(f.Nodes) != 2 {
			t.Fatalf("nodes drawn = %d, want the 2 module groups this link's granularity asks for", len(f.Nodes))
		}
		selectedNode(t, f, deepLinkSelected) // fatals unless exactly one node is ringed
	})

	// The reading view's own half of the same hash still landed: opening the
	// pane on load must not cost the reader the claim they were sent to.
	t.Run("the reading view honoured its half too", func(t *testing.T) {
		pollTrue(t, ctx, `document.querySelectorAll('.module-section').length === 2`)
		if !evalBool(t, ctx, `document.querySelectorAll('.module-section')[0].hidden && !document.querySelectorAll('.module-section')[1].hidden`) {
			t.Fatal("the deep link's reading-view target was lost: the second module must be the visible one")
		}
	})
}

// diagnoseTriggerOpen is only ever called on the failure path above, and it
// exists because the two ways this can be broken need different fixes and are
// indistinguishable from "the pane is hidden".
//
// If openFromHashOnLoad() is simply never REACHED, clicking the trigger opens
// the pane normally. If it is reached from somewhere that re-enters openPane()
// before `mounted` has been set, the click blows the stack instead — and the
// reader who was sent the link is left with a pane that cannot be opened at
// all, which is strictly worse than the deep link not working.
func diagnoseTriggerOpen(t *testing.T, ctx context.Context) string {
	t.Helper()
	evalVoid(t, ctx, `(function () {
		window.__dxgLoadErrors = [];
		window.addEventListener('error', function (e) { window.__dxgLoadErrors.push(String(e.message)); });
	})();`)
	runCDP(t, ctx, chromedp.Click("[data-dxg-open]", chromedp.ByQuery))
	settleFor(t, ctx, `document.getElementById('dxgPane').hidden === false`)
	errs := evalStrings(t, ctx, `window.__dxgLoadErrors`)
	if len(errs) > 0 {
		return fmt.Sprintf("Clicking the trigger on this same page then threw %v and left the pane hidden — "+
			"the load-time open is re-entering openPane() before mountPane() has set `mounted`.", errs)
	}
	if evalBool(t, ctx, `document.getElementById('dxgPane').hidden`) {
		return "Clicking the trigger on this same page did not open it either."
	}
	return "Clicking the trigger on this same page opens it fine, so the pane works and nothing looked at the hash on load."
}

// =====================================================================
// 2 — DRILL-DOWN AND COLLAPSE BOTH RE-FRAME THE CAMERA
// =====================================================================

// fitConfig is ONE module, so the module-granularity view is a single group
// node and the double-click that expands it needs no search: there is exactly
// one node on the canvas and it is a group. After the expand every node on
// the canvas is a claim, so the double-click that collapses it needs no
// search either.
const fitConfig = `schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
`

// newFitProject chains twenty claims. Twenty is not decoration: the folded
// view is ONE disc about 17px across, which fitToView frames at its 1.6
// zoom ceiling, and twenty claims spread by the layout at that same zoom
// cover several times the canvas. A camera left where the folded view put it
// therefore draws most of the graph off the edge, which is the regression.
func newFitProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, fitConfig)
	prev := ""
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("widget.contract.claim-%02d", i)
		p.writeClaim(fmt.Sprintf("c%02d.yaml", i), graphClaim(id, "contract", prev))
		prev = id
	}
	return p
}

// inkBounds is the screen-space box the frame's node discs were drawn into:
// each node's world centre and its drawn (moat) radius, through that same
// frame's camera. This is the box fitToView exists to place inside the
// canvas, so it is the box to ask about.
func inkBounds(f canvasFrame) box {
	b := box{x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1)}
	for _, n := range f.Nodes {
		sx := n.X*f.Cam.Zoom + f.Cam.X
		sy := n.Y*f.Cam.Zoom + f.Cam.Y
		sr := n.MoatR * f.Cam.Zoom
		b.x0 = math.Min(b.x0, sx-sr)
		b.y0 = math.Min(b.y0, sy-sr)
		b.x1 = math.Max(b.x1, sx+sr)
		b.y1 = math.Max(b.y1, sy+sr)
	}
	return b
}

// dblClickAt dispatches a double-click on the canvas at a screen point, in
// canvas-local coordinates. A bare dblclick and not a pointer sequence: the
// pane's pointerdown handler cancels an armed fit on purpose (a reader who
// touches the canvas has said where they want to be looking), and a real
// double-click still works because its pointerdowns all precede the dblclick
// that arms the fit. Dispatching the gesture's own event keeps this test
// about expandGroup/collapseGroup rather than about that ordering.
func dblClickAt(t *testing.T, ctx context.Context, x, y float64) {
	t.Helper()
	evalVoid(t, ctx, fmt.Sprintf(`(function () {
		var cv = document.querySelector('.dxg-canvas');
		var r = cv.getBoundingClientRect();
		cv.dispatchEvent(new MouseEvent('dblclick', {
			clientX: r.left + %v, clientY: r.top + %v, bubbles: true, cancelable: true
		}));
	})();`, x, y))
}

func TestGraphDrillDownAndCollapseFitTheCanvas(t *testing.T) {
	p := newFitProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)

	// Fold the corpus into its one module group. Granularity has always asked
	// for a fit, so this leaves the camera correctly framed and the drill-down
	// below is the only thing that could break it.
	setGranularity(t, ctx, "module")
	settleFrames(t, ctx)
	folded := lastFrame(t, ctx)
	if len(folded.Nodes) != 1 {
		t.Fatalf("nodes at module granularity = %d, want the corpus's 1 module group", len(folded.Nodes))
	}

	// Each phase is the same question asked of a different gesture: after the
	// graph changed underneath the camera, is what was drawn still on screen
	// and still centred? Centring is fitToView's own arithmetic —
	// camera.x = width/2 - inkCentreX * zoom — and it is what separates "the
	// pane re-framed" from "the camera happened to be wide enough".
	phases := []struct {
		name      string
		wantNodes int
		// minSpanFrac guards against a corpus too small to be evidence: it is
		// checked only where an unfitted camera would visibly overflow.
		minSpanFrac float64
	}{
		{"double-clicking the group expands it", 20, 0.30},
		{"double-clicking a claim collapses it again", 1, 0},
	}

	for _, ph := range phases {
		t.Run(ph.name, func(t *testing.T) {
			// The point to hit comes from the frame immediately before the
			// gesture: the one group node for the expand, and after it any
			// node at all, because every node on this canvas is then a claim.
			f := lastFrame(t, ctx)
			target := f.Nodes[0]
			dblClickAt(t, ctx,
				target.X*f.Cam.Zoom+f.Cam.X,
				target.Y*f.Cam.Zoom+f.Cam.Y)

			settleFrames(t, ctx)
			after := lastFrame(t, ctx)
			if len(after.Nodes) != ph.wantNodes {
				t.Fatalf("nodes drawn after the double-click = %d, want %d — the gesture did not reach expandGroup/collapseGroup at all",
					len(after.Nodes), ph.wantNodes)
			}

			ink := inkBounds(after)
			if after.CSSW < 200 || after.CSSH < 200 {
				t.Fatalf("canvas is %vx%v CSS px — too small for an on-canvas assertion to mean anything", after.CSSW, after.CSSH)
			}

			// THE REGRESSION, MEASURED: no drawn disc may hang off any edge.
			if ink.x0 < 0 || ink.y0 < 0 || ink.x1 > after.CSSW || ink.y1 > after.CSSH {
				t.Fatalf("the drawn graph runs off the canvas after %s: ink x[%.1f,%.1f] y[%.1f,%.1f] on a %vx%v canvas "+
					"(clipped %.1fpx left, %.1fpx top, %.1fpx right, %.1fpx bottom) — the camera was left framed on the previous graph",
					ph.name, ink.x0, ink.x1, ink.y0, ink.y1, after.CSSW, after.CSSH,
					math.Max(0, -ink.x0), math.Max(0, -ink.y0),
					math.Max(0, ink.x1-after.CSSW), math.Max(0, ink.y1-after.CSSH))
			}

			// ...and it is not merely on screen, it is FRAMED. A camera that
			// was never re-fitted leaves the new graph wherever the old one
			// happened to sit.
			const centreTol = 1.5
			gotCX := (ink.x0 + ink.x1) / 2
			gotCY := (ink.y0 + ink.y1) / 2
			if math.Abs(gotCX-after.CSSW/2) > centreTol || math.Abs(gotCY-after.CSSH/2) > centreTol {
				t.Fatalf("the drawn graph is not centred after %s: ink centre (%.1f,%.1f), canvas centre (%.1f,%.1f) "+
					"— fitToView centres what it fits, so this camera was never re-fitted",
					ph.name, gotCX, gotCY, after.CSSW/2, after.CSSH/2)
			}

			if ph.minSpanFrac > 0 {
				span := math.Max(ink.x1-ink.x0, ink.y1-ink.y0) / math.Max(after.CSSW, after.CSSH)
				if span < ph.minSpanFrac {
					t.Fatalf("the expanded graph spans only %.0f%% of the canvas: this corpus is too small to prove anything about clipping",
						span*100)
				}
			}
		})
	}
}

// =====================================================================
// 3 — THE PANE'S TEXT MEETS WCAG AA IN BOTH THEMES
// =====================================================================

// contrastProbe installs a WCAG contrast reader.
//
// TWO THINGS MAKE IT A REAL MEASUREMENT rather than a token comparison.
// First, the background is RESOLVED, not read: .dxg-toggle[aria-pressed=true]
// paints --accent-bg, which is a 14% tint, so the colour a reader's eye
// actually sees is that tint composited over whatever opaque surface is
// underneath it. The probe walks ancestors, collecting every non-transparent
// layer until it reaches an opaque one, and composites them back down.
// Second, the ratio comes from the WCAG relative-luminance formula on the
// composited sRGB values, so it is the number the guideline names and not a
// proxy for it.
const contrastProbe = `(function () {
	function parse(v) {
		var m = String(v).match(/^rgba?\(([^)]*)\)$/);
		if (!m) { return null; }
		var p = m[1].split(/[\s,\/]+/).filter(function (x) { return x !== ''; }).map(Number);
		if (p.length < 3) { return null; }
		for (var i = 0; i < p.length; i++) { if (!isFinite(p[i])) { return null; } }
		return { r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1 };
	}
	function show(c) {
		return 'rgba(' + Math.round(c.r) + ',' + Math.round(c.g) + ',' + Math.round(c.b) + ',' + c.a + ')';
	}
	function over(top, bottom) {
		var a = top.a;
		return {
			r: top.r * a + bottom.r * (1 - a),
			g: top.g * a + bottom.g * (1 - a),
			b: top.b * a + bottom.b * (1 - a),
			a: 1
		};
	}
	function lin(v) {
		v = v / 255;
		return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
	}
	function lum(c) { return 0.2126 * lin(c.r) + 0.7152 * lin(c.g) + 0.0722 * lin(c.b); }

	window.__dxgContrast = function (sel) {
		var el = document.querySelector(sel);
		if (!el) { return { error: 'no element matches ' + sel }; }
		var cs = getComputedStyle(el);

		// The painted ground under this element's text, bottom layer first.
		var stack = [];
		var opaque = false;
		for (var n = el; n; n = n.parentElement) {
			var c = parse(getComputedStyle(n).backgroundColor);
			if (!c || c.a === 0) { continue; }
			stack.push(c);
			if (c.a === 1) { opaque = true; break; }
		}
		// A page that never paints an opaque background is composited by the
		// browser over white; say so rather than guessing a token.
		if (!opaque) { stack.push({ r: 255, g: 255, b: 255, a: 1 }); }
		var bg = stack[stack.length - 1];
		for (var i = stack.length - 2; i >= 0; i--) { bg = over(stack[i], bg); }

		var fg = parse(cs.color);
		if (!fg) { return { error: 'unparseable color ' + cs.color + ' on ' + sel }; }
		var ownBg = parse(getComputedStyle(el).backgroundColor);
		if (fg.a < 1) { fg = over(fg, bg); }

		var l1 = lum(fg), l2 = lum(bg);
		var hi = Math.max(l1, l2), lo = Math.min(l1, l2);
		var layers = [];
		for (var j = 0; j < stack.length; j++) { layers.push(show(stack[j])); }
		return {
			ratio: (hi + 0.05) / (lo + 0.05),
			fg: show(fg),
			bg: show(bg),
			ownBgAlpha: ownBg ? ownBg.a : 1,
			layers: layers,
			font: cs.fontSize,
			text: (el.textContent || '').slice(0, 40)
		};
	};
})();`

type contrastReading struct {
	Ratio      float64  `json:"ratio"`
	FG         string   `json:"fg"`
	BG         string   `json:"bg"`
	OwnBgAlpha float64  `json:"ownBgAlpha"`
	Layers     []string `json:"layers"`
	Font       string   `json:"font"`
	Text       string   `json:"text"`
	Error      string   `json:"error"`
}

func readContrast(t *testing.T, ctx context.Context, sel string) contrastReading {
	t.Helper()
	var r contrastReading
	evalInto(t, ctx, `window.__dxgContrast(`+jsString(t, sel)+`)`, &r)
	if r.Error != "" {
		t.Fatalf("contrast probe on %s: %s", sel, r.Error)
	}
	return r
}

// emulateColorScheme puts the tab in one OS colour mode and refuses to
// continue until the page agrees. Without the check a CDP that silently did
// nothing would report the dark theme's numbers twice, and the light theme —
// the only one that ever failed — would go unmeasured.
func emulateColorScheme(t *testing.T, ctx context.Context, scheme string) {
	t.Helper()
	runCDP(t, ctx, emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
		{Name: "prefers-color-scheme", Value: scheme},
	}))
	pollTrue(t, ctx, fmt.Sprintf(`window.matchMedia('(prefers-color-scheme: %s)').matches`, scheme))
}

// wcagAANormal is the ratio WCAG 2.1 SC 1.4.3 requires of text below 18.66px
// bold / 24px regular. Both controls below are 11-12px.
const wcagAANormal = 4.5

func TestGraphPaneControlsMeetContrastAA(t *testing.T) {
	p := newGraphProject(t)
	ctx := staticGraphTab(t, p)
	evalVoid(t, ctx, contrastProbe)
	openGraphPane(t, ctx)

	// The open-claim button only exists once a claim is selected, so select
	// one. The edge-type toggles start pressed, which is the ON state whose
	// rule is under test.
	runCDP(t, ctx, chromedp.Click(`[data-dxg-jump="widget.design.thing"]`, chromedp.ByQuery))
	waitVisible(t, ctx, `[data-dxg-open-claim]`)
	if got := evalString(t, ctx, `document.querySelector('[data-dxg-type="rests_on"]').getAttribute('aria-pressed')`); got != "true" {
		t.Fatalf("the rests_on toggle is aria-pressed=%q, want true — the ON state is the state this measures", got)
	}

	controls := []struct {
		name string
		sel  string
	}{
		// Measured at 2.07:1 in light mode before e622acf: the accent painted
		// on its own 14% tint.
		{"active edge-type toggle", `.dxg-toggle[aria-pressed="true"][data-dxg-type="rests_on"]`},
		// Measured at 2.64:1: --link, an accent-derived token, on --paper.
		{"open-claim button", `[data-dxg-open-claim]`},
	}
	schemes := []string{"dark", "light"}

	// Keyed by scheme+control so the vacuity check below can prove the two
	// themes were genuinely different pages.
	seen := map[string]contrastReading{}

	for _, scheme := range schemes {
		scheme := scheme
		t.Run(scheme, func(t *testing.T) {
			emulateColorScheme(t, ctx, scheme)
			for _, c := range controls {
				t.Run(c.name, func(t *testing.T) {
					r := readContrast(t, ctx, c.sel)
					seen[scheme+"/"+c.name] = r
					if r.Ratio < wcagAANormal {
						t.Fatalf("%s in %s mode: contrast %.2f:1, want at least %.1f:1 (WCAG AA, normal text at %s)\n"+
							"  text %q\n  colour %s on a resolved ground of %s\n  background layers, top first: %v",
							c.name, scheme, r.Ratio, wcagAANormal, r.Font, r.Text, r.FG, r.BG, r.Layers)
					}
				})
			}
		})
	}

	// Two guards, so a pass here cannot be an accident of the harness.
	t.Run("the toggle's tint really is composited over a surface", func(t *testing.T) {
		r := seen["light/active edge-type toggle"]
		if r.OwnBgAlpha >= 1 {
			t.Fatalf("the active toggle paints an opaque background (alpha %v): the ancestor walk this test does is not being exercised, so the resolved ground may be wrong for the wrong reason", r.OwnBgAlpha)
		}
		if len(r.Layers) < 2 {
			t.Fatalf("the resolved ground under the active toggle came from %d layer(s): a 14%% tint must be composited over at least one surface below it", len(r.Layers))
		}
	})

	t.Run("light and dark were genuinely different pages", func(t *testing.T) {
		for _, c := range controls {
			dark := seen["dark/"+c.name]
			light := seen["light/"+c.name]
			if dark.BG == light.BG {
				t.Fatalf("%s resolved the same ground %s in both modes: the colour-scheme emulation never took effect, so only one theme was measured",
					c.name, dark.BG)
			}
		}
	})
}
