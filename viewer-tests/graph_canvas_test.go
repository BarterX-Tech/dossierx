package viewertests

// WHAT THE CANVAS ACTUALLY DREW.
//
// graph_pane_test.go says, at the top, that pixels are deliberately not
// asserted — and that is still right. A force layout has no stable pixels, a
// screenshot baseline fails on a font, and every VERDICT the pane draws is
// computed in graph-core.js before anything reaches the canvas.
//
// This file asserts something different and narrower: the DRAW CALLS. Not
// where a node ended up, but that the node the pane says is in a cycle was
// stroked in the cycle colour; that the backing store was allocated at the
// display's ratio; that an overlay matching nothing left every node at full
// alpha instead of fading the scene to 14%; that a label the layout suppressed
// was not drawn and that zooming in draws more of them. Each of those is a
// property of the drawing code that no DOM query can see and that a pixel
// comparison would only see by accident.
//
// The mechanism is a recorder installed over CanvasRenderingContext2D's
// prototype before the pane opens. draw() opens every frame with
// setTransform(dpr, …), which is the frame boundary the recorder keys off, so
// each recorded frame is one complete pass of drawEdges → drawNodes →
// drawLabels with one camera and one palette. EVERY ASSERTION HERE IS MADE
// WITHIN A SINGLE FRAME, which is what makes it safe against a simulation that
// is still moving: two facts read from one frame agree even when the frame
// after it has moved every node.
//
// Colours are compared against the stylesheet's own tokens, resolved through a
// throwaway 2D context so both sides are normalised the same way. No hex
// literal from graph.css is copied into this file.

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// The recorder
// ---------------------------------------------------------------------

// canvasRecorderJS wraps the 2D context prototype and keeps the last two
// frames. It is installed BEFORE the pane opens and is deliberately dumb: it
// records path geometry, the paint state at each stroke/fill, and the measured
// width of every label at the moment it was drawn — measured with the context's
// own font, so a label box reconstructed from it is the box drawLabels tested
// for collisions, not an approximation of it.
//
// Re-running it resets the buffer without re-wrapping, so a test may clear the
// record between cases.
const canvasRecorderJS = `(function () {
	var R = { frames: [], at: 0 };
	window.__dxgRec = R;
	if (window.__dxgRecInstalled) { return; }
	window.__dxgRecInstalled = true;

	var P = CanvasRenderingContext2D.prototype;
	var names = ['setTransform', 'translate', 'scale', 'beginPath', 'arc', 'moveTo',
		'lineTo', 'quadraticCurveTo', 'roundRect', 'arcTo', 'stroke', 'fill', 'fillText'];
	var raw = {};
	for (var n = 0; n < names.length; n++) { raw[names[n]] = P[names[n]]; }

	var cur = null;
	var path = [];

	P.setTransform = function (a) {
		var box = this.canvas.getBoundingClientRect();
		cur = {
			dpr: typeof a === 'number' ? a : 1,
			cam: { x: 0, y: 0, zoom: 1 },
			w: this.canvas.width, h: this.canvas.height,
			cssw: box.width, cssh: box.height,
			ops: []
		};
		window.__dxgRec.frames.push(cur);
		while (window.__dxgRec.frames.length > 2) { window.__dxgRec.frames.shift(); }
		window.__dxgRec.at = Date.now();
		path = [];
		return raw.setTransform.apply(this, arguments);
	};
	P.translate = function (x, y) {
		if (cur) { cur.cam.x = x; cur.cam.y = y; }
		return raw.translate.apply(this, arguments);
	};
	P.scale = function (sx) {
		if (cur) { cur.cam.zoom = sx; }
		return raw.scale.apply(this, arguments);
	};
	P.beginPath = function () { path = []; return raw.beginPath.apply(this, arguments); };
	P.arc = function (x, y, r) { path.push({ k: 'arc', x: x, y: y, r: r }); return raw.arc.apply(this, arguments); };
	P.moveTo = function (x, y) { path.push({ k: 'p', x: x, y: y }); return raw.moveTo.apply(this, arguments); };
	P.lineTo = function (x, y) { path.push({ k: 'p', x: x, y: y }); return raw.lineTo.apply(this, arguments); };
	P.quadraticCurveTo = function (cx, cy, x, y) { path.push({ k: 'q', x: x, y: y }); return raw.quadraticCurveTo.apply(this, arguments); };
	P.roundRect = function (x, y, w, h) { path.push({ k: 'arc', x: x + w / 2, y: y + h / 2, r: w / 2 }); return raw.roundRect.apply(this, arguments); };
	P.arcTo = function (x1, y1) { path.push({ k: 'p', x: x1, y: y1 }); return raw.arcTo.apply(this, arguments); };

	function paint(kind) {
		return function () {
			if (cur) {
				cur.ops.push({
					op: kind,
					color: String(kind === 'stroke' ? this.strokeStyle : this.fillStyle),
					width: this.lineWidth,
					alpha: this.globalAlpha,
					dash: this.getLineDash().length > 0,
					path: path.slice()
				});
			}
			return raw[kind].apply(this, arguments);
		};
	}
	P.stroke = paint('stroke');
	P.fill = paint('fill');

	P.fillText = function (text, x, y) {
		if (cur) {
			cur.ops.push({
				op: 'text', text: String(text), x: x, y: y,
				alpha: this.globalAlpha, width: this.measureText(String(text)).width, path: []
			});
		}
		return raw.fillText.apply(this, arguments);
	};
})();`

// frameSummaryJS folds the last recorded frame's raw ops into nodes, edges and
// labels.
//
// The grouping rule is the MOAT: drawNodes opens every node with a stroke of
// the page colour at width 2, drawn just outside the disc, and nothing else in
// the file strokes that colour at that width (a label's halo is strokeText, not
// stroke). So a paper-coloured width-2 stroke starts a node and everything
// until the next one decorates it — fill, then the status/cycle ring, then any
// halo, selection ring and governance wedge, in drawNodes's own order. Ops
// before the first node belong to drawEdges; two-point strokes are the edge
// lines and three-point ones its chevrons.
const frameSummaryJS = `(function () {
	var R = window.__dxgRec;
	if (!R || R.frames.length === 0) { return null; }
	var f = R.frames[R.frames.length - 1];

	var cs = getComputedStyle(document.documentElement);
	var probe = document.createElement('canvas').getContext('2d');
	function tok(name) {
		var v = (cs.getPropertyValue(name) || '').trim();
		if (v === '') { return ''; }
		probe.strokeStyle = '#000000';
		probe.strokeStyle = v;
		return String(probe.strokeStyle);
	}
	var pal = {
		paper: tok('--paper'), ink: tok('--ink'), muted: tok('--muted'), faint: tok('--faint'),
		accent: tok('--accent'), warn: tok('--warn'), link: tok('--link'),
		cycle: tok('--dxg-cycle'), halo: tok('--dxg-halo'), governed: tok('--dxg-governed')
	};

	function shape(p) {
		if (p.length === 0) { return null; }
		if (p[0].k === 'arc') { return { x: p[0].x, y: p[0].y, r: p[0].r }; }
		var x0 = p[0].x, x1 = p[0].x, y0 = p[0].y, y1 = p[0].y;
		for (var i = 1; i < p.length; i++) {
			x0 = Math.min(x0, p[i].x); x1 = Math.max(x1, p[i].x);
			y0 = Math.min(y0, p[i].y); y1 = Math.max(y1, p[i].y);
		}
		return { x: (x0 + x1) / 2, y: (y0 + y1) / 2, r: Math.max(x1 - x0, y1 - y0) / 2 };
	}

	var nodes = [], edges = [], labels = [], cur = null;
	for (var i = 0; i < f.ops.length; i++) {
		var o = f.ops[i];
		if (o.op === 'text') {
			labels.push({ text: o.text, x: o.x, y: o.y, alpha: o.alpha, width: o.width });
			continue;
		}
		var s = shape(o.path);
		if (!s) { continue; }
		if (o.op === 'stroke' && o.color === pal.paper && Math.abs(o.width - 2) < 0.001) {
			cur = { x: s.x, y: s.y, moatR: s.r, alpha: o.alpha, fill: null, rings: [] };
			nodes.push(cur);
			continue;
		}
		if (cur) {
			if (o.op === 'fill') {
				if (!cur.fill) { cur.fill = { color: o.color, alpha: o.alpha }; }
			} else {
				cur.rings.push({ color: o.color, width: o.width, alpha: o.alpha, r: s.r, dash: o.dash });
			}
			continue;
		}
		if (o.op === 'stroke' && o.path.length === 2) {
			edges.push({
				color: o.color, alpha: o.alpha, width: o.width, dash: o.dash,
				curved: o.path[1].k === 'q',
				fx: o.path[0].x, fy: o.path[0].y, tx: o.path[1].x, ty: o.path[1].y
			});
		}
	}
	return {
		dpr: f.dpr, cam: f.cam, w: f.w, h: f.h, cssw: f.cssw, cssh: f.cssh,
		pal: pal, nodes: nodes, edges: edges, labels: labels
	};
})();`

type frameRing struct {
	Color string  `json:"color"`
	Width float64 `json:"width"`
	Alpha float64 `json:"alpha"`
	R     float64 `json:"r"`
	Dash  bool    `json:"dash"`
}

type frameFill struct {
	Color string  `json:"color"`
	Alpha float64 `json:"alpha"`
}

type frameNode struct {
	X     float64     `json:"x"`
	Y     float64     `json:"y"`
	MoatR float64     `json:"moatR"`
	Alpha float64     `json:"alpha"`
	Fill  *frameFill  `json:"fill"`
	Rings []frameRing `json:"rings"`
}

// ring is the status/cycle ring: the first stroke after the fill, before any
// halo or selection ring.
func (n frameNode) ring() frameRing {
	if len(n.Rings) == 0 {
		return frameRing{}
	}
	return n.Rings[0]
}

func (n frameNode) wearsRing(color string) bool {
	for _, r := range n.Rings {
		if r.Color == color {
			return true
		}
	}
	return false
}

type frameEdge struct {
	Color  string  `json:"color"`
	Alpha  float64 `json:"alpha"`
	Width  float64 `json:"width"`
	Dash   bool    `json:"dash"`
	Curved bool    `json:"curved"`
	FX     float64 `json:"fx"`
	FY     float64 `json:"fy"`
	TX     float64 `json:"tx"`
	TY     float64 `json:"ty"`
}

type frameLabel struct {
	Text  string  `json:"text"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Alpha float64 `json:"alpha"`
	Width float64 `json:"width"`
}

type canvasFrame struct {
	Dpr float64 `json:"dpr"`
	Cam struct {
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		Zoom float64 `json:"zoom"`
	} `json:"cam"`
	W      float64           `json:"w"`
	H      float64           `json:"h"`
	CSSW   float64           `json:"cssw"`
	CSSH   float64           `json:"cssh"`
	Pal    map[string]string `json:"pal"`
	Nodes  []frameNode       `json:"nodes"`
	Edges  []frameEdge       `json:"edges"`
	Labels []frameLabel      `json:"labels"`
}

// installRecorder wraps the 2D context. Call it before the pane is opened.
func installRecorder(t *testing.T, ctx context.Context) {
	t.Helper()
	evalVoid(t, ctx, canvasRecorderJS)
}

// settleFrames waits until the cooling simulation has stopped painting. The
// signal is the absence of a new frame rather than a fixed sleep: the layout
// draws on every rAF while alpha > 0.02 and then stops, so "no frame for
// 350ms" is "the graph has stopped moving".
func settleFrames(t *testing.T, ctx context.Context) {
	t.Helper()
	pollTrue(t, ctx, `!!window.__dxgRec && window.__dxgRec.frames.length > 0 && (Date.now() - window.__dxgRec.at) > 350`)
}

// lastFrame forces one repaint and returns the frame it produced. Forcing it
// through the pane's own resize handler (rather than waiting for a rAF that may
// never come once the layout has cooled) is what makes this deterministic.
func lastFrame(t *testing.T, ctx context.Context) canvasFrame {
	t.Helper()
	evalVoid(t, ctx, forceDraw)
	return readFrame(t, ctx)
}

// readFrame returns the most recent frame without asking for a new one — for
// the tests where dispatching a resize would be dispatching the very event
// under test.
func readFrame(t *testing.T, ctx context.Context) canvasFrame {
	t.Helper()
	var f canvasFrame
	evalInto(t, ctx, frameSummaryJS, &f)
	if len(f.Nodes) == 0 {
		t.Fatalf("the recorded frame drew no nodes at all: %+v", f)
	}
	return f
}

func evalFloat(t *testing.T, ctx context.Context, expr string) float64 {
	t.Helper()
	var v float64
	evalInto(t, ctx, expr, &v)
	return v
}

func nearly(a, b float64) bool { return math.Abs(a-b) < 0.01 }

// selectedNode returns the node wearing the selection ring — the accent-coloured
// ring drawn at r+7 for state.selected. It is the only way to say "THIS id's
// node" about a canvas whose coordinates carry no ids, and every id-anchored
// assertion in this file goes through it.
func selectedNode(t *testing.T, f canvasFrame, id string) frameNode {
	t.Helper()
	var found []frameNode
	for _, n := range f.Nodes {
		if n.wearsRing(f.Pal["accent"]) {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		t.Fatalf("nodes wearing the selection ring = %d, want exactly 1 (the node for %s)", len(found), id)
	}
	return found[0]
}

// clickJump clicks the gaps rail entry for an id, which selects that claim and
// centres the canvas on it.
func clickJump(t *testing.T, ctx context.Context, id string) {
	t.Helper()
	runCDP(t, ctx, chromedp.Click(`[data-dxg-jump=`+jsQuote(id)+`]`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.dxg-detail-id') && document.querySelector('.dxg-detail-id').textContent === `+jsQuote(id))
}

// ---------------------------------------------------------------------
// Finding 2 — the backing store follows devicePixelRatio
// ---------------------------------------------------------------------

// dprWatchShim stands in for the ONE browser event this environment cannot
// produce.
//
// A window moved between a 2x display and a 1x one fires no resize — the CSS
// box did not change — so graph-ui.js catches it with a matchMedia query on the
// CURRENT ratio, which stops matching the instant the ratio moves. CDP's device
// metrics override does move window.devicePixelRatio and does flip that query's
// `matches`, but it dispatches no `change` event on it [VERIFIED by probe: with
// deviceScaleFactor 2, matchMedia('(resolution: 1dppx)').matches goes false,
// resize fires 0 times and the query's change listener fires 0 times].
//
// So the shim wraps matchMedia for resolution queries only, keeps the real
// `matches`, records the listeners the pane registers, and exposes a way to
// deliver the change the browser will not. Everything under test is still the
// pane's own code — the shim delivers an event and observes which query the
// pane armed, and nothing else. It honours `once` and removeEventListener,
// because the re-arm depends on both.
const dprWatchShim = `(function () {
	var real = window.matchMedia.bind(window);
	window.__dxgMQ = { queries: [], last: '' };
	window.matchMedia = function (q) {
		var mql = real(q);
		if (String(q).indexOf('resolution:') < 0) { return mql; }
		var entry = { query: String(q), listeners: [] };
		var wrap = {
			media: String(q),
			get matches() { return mql.matches; },
			addEventListener: function (type, fn, opts) {
				if (type !== 'change') { return; }
				entry.listeners.push({ fn: fn, once: !!(opts && opts.once) });
			},
			removeEventListener: function (type, fn) {
				for (var i = entry.listeners.length - 1; i >= 0; i--) {
					if (entry.listeners[i].fn === fn) { entry.listeners.splice(i, 1); }
				}
			},
			addListener: function (fn) { entry.listeners.push({ fn: fn, once: false }); },
			removeListener: function (fn) { wrap.removeEventListener('change', fn); }
		};
		entry.mql = wrap;
		window.__dxgMQ.queries.push(entry);
		window.__dxgMQ.last = entry.query;
		return wrap;
	};
	// Deliver a change to every armed resolution query that has stopped
	// matching, exactly as a browser does when a window changes display.
	// Returns how many listeners were called, so a test can tell "the pane
	// coped" from "the pane had nothing listening".
	window.__dxgFireDprChange = function () {
		var fired = 0;
		var qs = window.__dxgMQ.queries.slice();
		for (var i = 0; i < qs.length; i++) {
			if (qs[i].mql.matches) { continue; }
			var ls = qs[i].listeners.slice();
			for (var j = 0; j < ls.length; j++) {
				if (ls[j].once) { qs[i].mql.removeEventListener('change', ls[j].fn); }
				ls[j].fn.call(qs[i].mql, { matches: false, media: qs[i].query });
				fired++;
			}
		}
		return fired;
	};
})();`

// The canvas shipped drawn at 1x on a 2x display: soft graph, crisp DOM text
// twenty pixels away. Both halves of the fix are asserted here — the backing
// store is allocated at cssPx * dpr, and the context is scaled by the same
// factor so every coordinate in graph-ui.js stays in CSS pixels — plus the
// ceiling that stops a browser reporting 4 from allocating 16x the pixels, and
// the RE-ARM without which the watcher would only ever catch the first change.
//
// No resize is dispatched anywhere in this test. Every repair below is the
// pane's own response to a display change.
func TestGraphCanvasBackingStoreFollowsDevicePixelRatio(t *testing.T) {
	p := newGraphProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	evalVoid(t, ctx, dprWatchShim)
	openGraphPane(t, ctx)

	// The baseline, before anything moves: a 1x display is already the
	// emulated state, and the pane armed its watcher on it when it opened.
	if got := evalString(t, ctx, `window.__dxgMQ.last`); got != "(resolution: 1dppx)" {
		t.Fatalf("the pane armed %q on opening, want a resolution query on the current 1x ratio", got)
	}

	cases := []struct {
		name  string
		scale float64
		want  float64 // the ratio the pane must draw at
	}{
		{"moved to a 2x display", 2, 2},
		{"moved to a 3x display", 3, 3},
		{"4x is clamped to the ceiling", 4, 3},
		{"moved back to a 1x display", 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runCDP(t, ctx, chromedp.EmulateViewport(1280, 900, chromedp.EmulateScale(tc.scale)))
			pollTrue(t, ctx, fmt.Sprintf(`window.devicePixelRatio === %v`, tc.scale))

			if fired := evalInt(t, ctx, `window.__dxgFireDprChange()`); fired == 0 {
				t.Fatal("the ratio changed and the pane had nothing listening for it: the watcher was never armed, or never re-armed after the last change")
			}
			pollTrue(t, ctx, fmt.Sprintf(`(function () {
				var c = document.querySelector('.dxg-canvas');
				var h = document.querySelector('.dxg-canvas-holder');
				return c.width === Math.max(1, Math.round(h.clientWidth * %v));
			})()`, tc.want))

			// A resolution query tests ONE value, so the pane must rebuild it
			// against the ratio it just moved to. Without this the second
			// display change goes unnoticed for ever.
			wantQuery := fmt.Sprintf("(resolution: %vdppx)", tc.want)
			if got := evalString(t, ctx, `window.__dxgMQ.last`); got != wantQuery {
				t.Fatalf("the pane re-armed on %q, want %q — a watcher on the old ratio never fires again", got, wantQuery)
			}

			cssW := evalFloat(t, ctx, `document.querySelector('.dxg-canvas-holder').clientWidth`)
			cssH := evalFloat(t, ctx, `document.querySelector('.dxg-canvas-holder').clientHeight`)
			if cssW < 100 || cssH < 100 {
				t.Fatalf("canvas holder is %vx%v CSS px — too small for this assertion to mean anything", cssW, cssH)
			}
			// The CSS box the reader sees IS the box the backing store is
			// sized against; if the canvas element were laid out at some other
			// size the ratio below would be measuring nothing.
			if box := evalFloat(t, ctx, `document.querySelector('.dxg-canvas').getBoundingClientRect().width`); math.Abs(box-cssW) > 0.5 {
				t.Fatalf("canvas CSS width = %v, holder = %v — they must be the same box", box, cssW)
			}

			gotW := evalFloat(t, ctx, `document.querySelector('.dxg-canvas').width`)
			gotH := evalFloat(t, ctx, `document.querySelector('.dxg-canvas').height`)
			if wantW := math.Round(cssW * tc.want); gotW != wantW {
				t.Fatalf("backing store width = %v at dpr %v, want %v (%v CSS px x %v)", gotW, tc.scale, wantW, cssW, tc.want)
			}
			if wantH := math.Round(cssH * tc.want); gotH != wantH {
				t.Fatalf("backing store height = %v at dpr %v, want %v (%v CSS px x %v)", gotH, tc.scale, wantH, cssH, tc.want)
			}

			// The other half: the context transform. Without it a 2x backing
			// store would draw the graph at half size in its top-left quarter.
			transform := evalFloat(t, ctx, `document.querySelector('.dxg-canvas').getContext('2d').getTransform().a`)
			if transform != tc.want {
				t.Fatalf("context transform scale = %v, want %v", transform, tc.want)
			}
			// The frame the pane drew in response — not one this test asked
			// for. onDprChange resizes and redraws; that redraw is the frame
			// read here.
			if f := readFrame(t, ctx); f.Dpr != tc.want {
				t.Fatalf("the frame was drawn at setTransform(%v), want %v", f.Dpr, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------
// Finding 3 — an overlay matching nothing says so, and does not ghost
// ---------------------------------------------------------------------

// overlayConfig is one module with two facets, enough for an isolated node and
// a two-edge hub to coexist.
const overlayConfig = `schema_version: 1
facets:
  - contract
  - design
modules:
  - widget
claims_dir: claims
`

// newOverlayProject seeds a hub (degree 2), two leaves (degree 1 each) and one
// isolated claim, so the `isolated` overlay matches SOMETHING and leaves the hub
// dimmed. Without a node the overlay does not match, "nothing was dimmed" would
// be true for a reason that has nothing to do with the fix.
func newOverlayProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, overlayConfig)
	p.writeClaim("hub.yaml", graphClaim("widget.contract.hub", "contract", ""))
	p.writeClaim("leaf1.yaml", graphClaim("widget.design.leaf-one", "design", "widget.contract.hub"))
	p.writeClaim("leaf2.yaml", graphClaim("widget.design.leaf-two", "design", "widget.contract.hub"))
	p.writeClaim("lone.yaml", graphClaim("widget.contract.lone", "contract", ""))
	return p
}

const dimmedAlpha = 0.14

// setOverlay drives the overlay select the way a reader does.
func setOverlay(t *testing.T, ctx context.Context, overlay string) {
	t.Helper()
	evalVoid(t, ctx, `(function () {
		var s = document.getElementById('dxgOverlay');
		s.value = `+jsQuote(overlay)+`;
		s.dispatchEvent(new Event('change'));
	})();`)
	pollTrue(t, ctx, `document.getElementById('dxgOverlay').value === `+jsQuote(overlay))
}

func noticeText(t *testing.T, ctx context.Context) string {
	t.Helper()
	return evalString(t, ctx, `document.querySelector('.dxg-notices').textContent`)
}

func TestGraphEmptyOverlayIsStatedAndDoesNotGhostTheGraph(t *testing.T) {
	p := newOverlayProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)
	settleFrames(t, ctx)

	cases := []struct {
		name    string
		overlay string
		label   string // the overlay's own display text, which the notice must quote
		empty   bool
	}{
		// A fresh draft corpus has no review_pending claim and no comment
		// thread, so both of these overlays match nothing at all.
		{"review pending matches nothing", "review", "review pending", true},
		{"open threads matches nothing", "comments", "open comment threads", true},
		{"no cycle can exist in a corpus that rendered", "cycles", "dependency cycles", true},
		// The control. This one matches, so it MUST dim — otherwise the
		// assertions above are passing because nothing dims, ever.
		{"isolated & weakly linked matches", "isolated", "isolated & weakly linked", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setOverlay(t, ctx, tc.overlay)
			f := lastFrame(t, ctx)

			var dimmed, lit int
			for _, n := range f.Nodes {
				if nearly(n.Alpha, dimmedAlpha) {
					dimmed++
				} else if nearly(n.Alpha, 1) {
					lit++
				} else {
					t.Fatalf("a node was drawn at alpha %v, which is neither lit (1) nor dimmed (%v)", n.Alpha, dimmedAlpha)
				}
			}
			notice := noticeText(t, ctx)

			if tc.empty {
				// THE GRAPH IS LEFT LEGIBLE. Not one node faded.
				if dimmed != 0 {
					t.Fatalf("%d of %d nodes were dimmed under an overlay that matches nothing — the graph is being ghosted", dimmed, len(f.Nodes))
				}
				for _, e := range f.Edges {
					if e.Alpha < 0.5 {
						t.Fatalf("an edge was drawn at alpha %v under an empty overlay, want the undimmed 0.55", e.Alpha)
					}
				}
				// ...and the pane SAYS why nothing is highlighted, naming the
				// overlay the reader picked.
				if !strings.Contains(notice, "matches nothing") || !strings.Contains(notice, tc.label) {
					t.Fatalf("notice = %q, want a message naming %q and saying it matches nothing", notice, tc.label)
				}
				return
			}

			if dimmed == 0 {
				t.Fatalf("no node was dimmed under %q, which matches only some of them — the dimming path is not running at all", tc.overlay)
			}
			if lit == 0 {
				t.Fatalf("every node was dimmed under %q: it is supposed to match the isolated and weakly linked ones", tc.overlay)
			}
			if strings.Contains(notice, "matches nothing") {
				t.Fatalf("notice = %q, but this overlay matches %d nodes", notice, lit)
			}
		})
	}

	// The notice is not only a message: it offers the way out, and it works.
	setOverlay(t, ctx, "review")
	runCDP(t, ctx, chromedp.Click(`.dxg-notices .dxg-notice-action`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.getElementById('dxgOverlay').value === 'none'`)
	if n := noticeText(t, ctx); strings.Contains(n, "matches nothing") {
		t.Fatalf("notice after clearing the overlay = %q, want it gone", n)
	}
}

// ---------------------------------------------------------------------
// Finding 1 — labels are laid out, not merely drawn
// ---------------------------------------------------------------------

// newLabelProject is deliberately dense and deliberately wordy: eighty claims
// in one module, chained, with derived titles seven words long. At the fitted
// zoom their label boxes come nowhere near all fitting, which is the state that
// used to render as glyph soup — about a third of the labels overprinting each
// other and running through the node discs.
func newLabelProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, graphConfig)
	prev := ""
	for i := 0; i < 80; i++ {
		id := fmt.Sprintf("widget.contract.the-claim-that-carries-a-long-title-%02d", i)
		p.writeClaim(fmt.Sprintf("c%02d.yaml", i), graphClaim(id, "contract", prev))
		prev = id
	}
	return p
}

// labelBox reconstructs the box drawLabels tested for collisions: the text is
// centred on x, its top is y, and the padding constants are graph-ui.js's.
// Width came back from the browser's own measureText with the drawing font set,
// so this is the same rectangle, not an estimate of it.
type box struct{ x0, y0, x1, y1 float64 }

func labelBox(l frameLabel) box {
	const padX, padY, fontPx = 3, 1, 11
	half := l.Width / 2
	return box{x0: l.X - half - padX, y0: l.Y - padY, x1: l.X + half + padX, y1: l.Y + fontPx + padY}
}

func (a box) overlaps(b box) bool {
	return !(a.x1 <= b.x0 || b.x1 <= a.x0 || a.y1 <= b.y0 || b.y1 <= a.y0)
}

// discs projects every drawn node into screen space — world position through
// the frame's own camera, which is the same transform drawLabels used when it
// tested a label box against a node disc.
//
// The recorded radius is the MOAT's, drawn one world pixel outside the node, so
// it is reduced by one here to recover radiusOf(node) — the radius drawLabels
// itself tested against. Comparing against the moat instead would report a
// label sitting legally in the one-pixel gap as an overlap.
func discs(f canvasFrame) []box {
	out := make([]box, 0, len(f.Nodes))
	for _, n := range f.Nodes {
		sx := n.X*f.Cam.Zoom + f.Cam.X
		sy := n.Y*f.Cam.Zoom + f.Cam.Y
		sr := (n.MoatR - 1) * f.Cam.Zoom
		out = append(out, box{x0: sx - sr, y0: sy - sr, x1: sx + sr, y1: sy + sr})
	}
	return out
}

// onScreen keeps the discs whose centre is inside the canvas — the population a
// label could be drawn for at this camera.
func onScreen(f canvasFrame, all []box) []box {
	var out []box
	for _, d := range all {
		cx := (d.x0 + d.x1) / 2
		cy := (d.y0 + d.y1) / 2
		if cx < 0 || cx > f.CSSW || cy < 0 || cy > f.CSSH {
			continue
		}
		out = append(out, d)
	}
	return out
}

func TestGraphLabelsSuppressCollisionsAndZoomRevealsMore(t *testing.T) {
	p := newLabelProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)
	settleFrames(t, ctx)

	fitted := lastFrame(t, ctx)
	fittedDiscs := discs(fitted)
	fittedNodes := onScreen(fitted, fittedDiscs)

	t.Run("suppression actually suppresses", func(t *testing.T) {
		if len(fitted.Labels) == 0 {
			t.Fatal("no labels were drawn at all: this asserts nothing about suppression")
		}
		// A wide margin, not a one-label one: this corpus is far denser than
		// the layout can label, so a change that quietly disabled suppression
		// fails here rather than sitting one label away from passing.
		if len(fitted.Labels)*2 > len(fittedNodes) {
			t.Fatalf("labels drawn = %d for %d on-screen nodes: barely anything was suppressed at the fitted zoom",
				len(fitted.Labels), len(fittedNodes))
		}
	})

	t.Run("no drawn label overprints another", func(t *testing.T) {
		boxes := make([]box, len(fitted.Labels))
		for i, l := range fitted.Labels {
			boxes[i] = labelBox(l)
		}
		for i := range boxes {
			for j := i + 1; j < len(boxes); j++ {
				if boxes[i].overlaps(boxes[j]) {
					t.Fatalf("labels %q and %q were both drawn and their boxes overlap — this is the glyph soup the layout exists to prevent",
						fitted.Labels[i].Text, fitted.Labels[j].Text)
				}
			}
		}
	})

	t.Run("no drawn label crosses a node disc", func(t *testing.T) {
		for _, l := range fitted.Labels {
			lb := labelBox(l)
			for _, d := range fittedDiscs {
				if lb.overlaps(d) {
					t.Fatalf("label %q was drawn across a node disc: text over a saturated fill is the least readable case there is", l.Text)
				}
			}
		}
	})

	t.Run("no two drawn labels are the same string", func(t *testing.T) {
		// Every title here is distinct, so nothing needs a qualifier — and two
		// identical labels would be a state a reader cannot resolve. The
		// qualifier path itself is proven by the test below, on a corpus that
		// actually has a collision.
		seen := map[string]bool{}
		for _, l := range fitted.Labels {
			if seen[l.Text] {
				t.Fatalf("the label %q was drawn twice with nothing saying which is which", l.Text)
			}
			seen[l.Text] = true
		}
	})

	t.Run("zooming in reveals more labels", func(t *testing.T) {
		// The ratio, not the count: zooming in also pushes nodes off the
		// canvas, and "more labels among the nodes you can still see" is the
		// claim the fix actually makes. A raw count would be measuring how
		// much of the graph left the viewport.
		before := float64(len(fitted.Labels)) / float64(len(fittedNodes))

		evalVoid(t, ctx, `(function () {
			var cv = document.querySelector('.dxg-canvas');
			var r = cv.getBoundingClientRect();
			cv.dispatchEvent(new WheelEvent('wheel', {
				deltaY: -300, clientX: r.left + r.width / 2, clientY: r.top + r.height / 2, cancelable: true
			}));
		})();`)

		zoomed := lastFrame(t, ctx)
		if zoomed.Cam.Zoom <= fitted.Cam.Zoom {
			t.Fatalf("zoom = %v after a wheel event, want more than the fitted %v", zoomed.Cam.Zoom, fitted.Cam.Zoom)
		}
		zoomedNodes := onScreen(zoomed, discs(zoomed))
		if len(zoomedNodes) == 0 {
			t.Fatal("zooming left no node on screen")
		}
		after := float64(len(zoomed.Labels)) / float64(len(zoomedNodes))
		if after <= before {
			t.Fatalf("labelled fraction of the visible nodes went %.2f -> %.2f on zooming in (%d/%d -> %d/%d): zoom must reveal labels, not hide them",
				before, after, len(fitted.Labels), len(fittedNodes), len(zoomed.Labels), len(zoomedNodes))
		}
	})

	t.Run("turning labels off draws none", func(t *testing.T) {
		runCDP(t, ctx, chromedp.Click(`[data-dxg-labels]`, chromedp.ByQuery))
		off := lastFrame(t, ctx)
		if len(off.Labels) != 0 {
			t.Fatalf("labels drawn with the labels toggle off = %d, want 0", len(off.Labels))
		}
	})
}

// A title is derived from the id's last slug, so two claims in different
// modules that made the same promise carry the SAME title. Drawn plain they are
// two identical labels with nothing saying which is which, and a reader
// comparing them is comparing two things they cannot tell apart. Its own test
// func because it needs its own corpus, and therefore its own page load.
func TestGraphLabelsDisambiguateSharedTitles(t *testing.T) {
	p := newProjectRaw(t, railConfig)
	p.writeClaim("w.yaml", railClaim("widget.contract.shared-title", "contract", "widget", ""))
	p.writeClaim("g.yaml", railClaim("gadget.contract.shared-title", "contract", "gadget", ""))

	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)
	settleFrames(t, ctx)

	f := lastFrame(t, ctx)
	var drawn []string
	for _, l := range f.Labels {
		drawn = append(drawn, l.Text)
	}
	if len(drawn) != 2 {
		t.Fatalf("labels drawn = %v, want both of two nodes labelled on an empty canvas", drawn)
	}
	// The qualifier is the module, because the module is what tells them apart.
	want := map[string]bool{
		"Shared Title · widget": true,
		"Shared Title · gadget": true,
	}
	for _, text := range drawn {
		if !want[text] {
			t.Fatalf("label %q, want one of %v — a shared title must carry the qualifier that says which claim it is", text, want)
		}
		delete(want, text)
	}
}
