package viewertests

// THE RAIL, THE DETAIL PANEL AND THE LEGEND MUST DESCRIBE THE PICTURE THAT IS
// ACTUALLY ON THE CANVAS.
//
// Each of the three used to describe a different one. The gaps rail computed
// four of its eight fact rules over the claim graph while the canvas drew
// module groups, so it named claims a reader could not see and could not click
// to. The detail panel looked a claim's representative up in the drawn graph's
// degree map and printed the answer under the claim's own name, so a claim with
// no edges at all read "degree (view) 6" — the 6 was its module's. The legend
// described one of three relations, and went on describing facet swatches while
// an overlay had recoloured every node on screen.
//
// The tests below are written against the rendered DOM and, where the question
// is "is this thing drawn?", against the recorded canvas frame from
// graph_canvas_test.go — because "drawn" is a fact about the canvas and asking
// the same module that produced the rail would only prove it agrees with
// itself.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// Fixture: two modules, one cross-module edge, one isolated commented claim
// ---------------------------------------------------------------------

const railConfig = `schema_version: 1
facets:
  - contract
  - design
modules:
  - widget
  - gadget
claims_dir: claims
`

func railClaim(id, facet, module, restsOn string) string {
	body := "id: " + id + `
facet: ` + facet + `
module: ` + module + `
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

// newRailProject is shaped so that every rule this file cares about has a
// NON-EMPTY answer at both granularities, and a DIFFERENT one at each:
//
//	widget.contract.base   no edges at all, and one open comment thread
//	widget.design.thing    rests_on gadget.contract.core — the one cross-module edge
//	gadget.contract.core   two inbound edges
//	gadget.design.extra    rests_on gadget.contract.core — an intra-module edge
//
// At claims granularity `open_threads` answers "widget.contract.base"; at module
// granularity it answers "module:widget", because that is the node standing for
// it on the canvas. Those two answers being different is the whole fix.
func newRailProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, railConfig)
	p.writeClaim("base.yaml", railClaim("widget.contract.base", "contract", "widget", ""))
	p.writeClaim("thing.yaml", railClaim("widget.design.thing", "design", "widget", "gadget.contract.core"))
	p.writeClaim("core.yaml", railClaim("gadget.contract.core", "contract", "gadget", ""))
	p.writeClaim("extra.yaml", railClaim("gadget.design.extra", "design", "gadget", "gadget.contract.core"))
	// An open thread is engine-managed state the rail reports on, and the only
	// one a test can create from the CLI. Without it `open_threads` is empty
	// at both granularities and proves nothing about which vocabulary it uses.
	p.run("comment", "add", "widget.contract.base", "--as", "human", "--body", "does this claim still belong here?")
	return p
}

// setGranularity drives the granularity select the way a reader does.
func setGranularity(t *testing.T, ctx context.Context, g string) {
	t.Helper()
	evalVoid(t, ctx, `(function () {
		var s = document.getElementById('dxgGranularity');
		s.value = `+jsQuote(g)+`;
		s.dispatchEvent(new Event('change'));
	})();`)
	pollTrue(t, ctx, `document.getElementById('dxgGranularity').value === `+jsQuote(g))
}

// setScopeModule and setScopeFacet drive the two scope selects the way a
// reader does. They are separate helpers rather than one taking an axis
// because the two controls are independent: a test that narrows one and not
// the other is the common case, and composing them is how the intersection
// gets exercised.
func setScopeSelect(t *testing.T, ctx context.Context, id, value string) {
	t.Helper()
	evalVoid(t, ctx, `(function () {
		var s = document.getElementById(`+jsQuote(id)+`);
		s.value = `+jsQuote(value)+`;
		s.dispatchEvent(new Event('change'));
	})();`)
	pollTrue(t, ctx, `document.getElementById(`+jsQuote(id)+`).value === `+jsQuote(value))
}

func setScopeModule(t *testing.T, ctx context.Context, module string) {
	t.Helper()
	setScopeSelect(t, ctx, "dxgModule", module)
}

func setScopeFacet(t *testing.T, ctx context.Context, facet string) {
	t.Helper()
	setScopeSelect(t, ctx, "dxgFacet", facet)
}

// drawnNodeIDs asks graph-core.js which nodes this granularity draws, from the
// payload in the document. It is the representative mapping the canvas itself
// consumes; the test then checks the canvas drew exactly that many discs, so
// this is anchored to pixels rather than trusted on its own.
func drawnNodeIDs(t *testing.T, ctx context.Context, granularity string) []string {
	t.Helper()
	return evalStrings(t, ctx, `(function () {
		var c = window.dossierxGraphCore;
		var p = JSON.parse(document.getElementById('dossierx-graph').textContent);
		var reps = c.representatives(c.scopeFilter(p.nodes, '', ''), `+jsQuote(granularity)+`, []);
		return reps.repNodes.map(function (n) { return n.id; });
	})()`)
}

func payloadClaimIDs(t *testing.T, ctx context.Context) []string {
	t.Helper()
	return evalStrings(t, ctx, `JSON.parse(document.getElementById('dossierx-graph').textContent).nodes.map(function (n) { return n.id; })`)
}

// railJumpIDs returns every id the rail offers as a click target, across every
// rule block — facts and heuristics alike.
func railJumpIDs(t *testing.T, ctx context.Context) []string {
	t.Helper()
	return evalStrings(t, ctx, `Array.from(document.querySelectorAll('#dxgPane [data-dxg-jump]'))
		.map(function (b) { return b.getAttribute('data-dxg-jump'); })`)
}

func setOf(ids []string) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// ---------------------------------------------------------------------
// Finding 4a — at module granularity the rail names only drawn nodes
// ---------------------------------------------------------------------

func TestGraphRailNamesOnlyNodesTheCanvasDraws(t *testing.T) {
	p := newRailProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)

	claimIDs := payloadClaimIDs(t, ctx)
	if len(claimIDs) != 4 {
		t.Fatalf("payload claims = %d, want 4", len(claimIDs))
	}
	claims := setOf(claimIDs)

	cases := []struct {
		name        string
		granularity string
		// the rules whose answers must be stated in the drawn graph's
		// vocabulary, with the exact answer this corpus produces
		want map[string][]string
	}{
		{
			name:        "every claim drawn",
			granularity: "claims",
			want: map[string][]string{
				"isolated":      {"widget.contract.base"},
				"weakly_linked": {"gadget.design.extra", "widget.design.thing"},
				"open_threads":  {"widget.contract.base"},
			},
		},
		{
			name:        "collapsed to modules",
			granularity: "module",
			want: map[string][]string{
				// Not "widget.contract.base": that claim is not on the canvas,
				// and the node that IS answers for it.
				"isolated":      {},
				"weakly_linked": {"module:gadget", "module:widget"},
				"open_threads":  {"module:widget"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setGranularity(t, ctx, tc.granularity)
			settleFrames(t, ctx)

			drawn := drawnNodeIDs(t, ctx, tc.granularity)
			if len(drawn) == 0 {
				t.Fatal("the representative mapping drew nothing")
			}
			// Anchor to the canvas: one disc per drawn node, no more.
			if f := lastFrame(t, ctx); len(f.Nodes) != len(drawn) {
				t.Fatalf("canvas drew %d node discs but the representative mapping names %d (%v)", len(f.Nodes), len(drawn), drawn)
			}
			drawnSet := setOf(drawn)

			for rule, want := range tc.want {
				got := ruleIDs(t, ctx, rule)
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("rule %s at %s granularity = %v, want %v", rule, tc.granularity, got, want)
				}
				for _, id := range got {
					if !drawnSet[id] {
						t.Fatalf("rule %s names %q, which is not a node this view draws (%v)", rule, id, drawn)
					}
				}
			}

			if tc.granularity != "module" {
				return
			}

			// The strong form of the fix, over the WHOLE rail rather than
			// three chosen rules: with the canvas drawing groups, not one
			// clickable id anywhere in the rail may be a claim id.
			//
			// This holds for every corpus that renders. The two rules that
			// are claim-level by nature — cycle and self_edge — cannot have a
			// member here, because all three cycle lints are error severity
			// and a corpus carrying one never reaches the render stage.
			jumps := railJumpIDs(t, ctx)
			if len(jumps) == 0 {
				t.Fatal("the rail offered no click targets at all: this asserts nothing")
			}
			for _, id := range jumps {
				if claims[id] {
					t.Fatalf("the rail names claim %q while the canvas draws %v — a reader cannot see it or click to it", id, drawn)
				}
				if !drawnSet[id] {
					t.Fatalf("the rail names %q, which this view does not draw (drawn: %v)", id, drawn)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------
// Finding 4b — a degree is never printed under the wrong node's name
// ---------------------------------------------------------------------

// degreeRow reads the detail panel's "degree (view)" value, and whether it
// carries the note that says whose number it is.
func degreeRow(t *testing.T, ctx context.Context) (string, bool) {
	t.Helper()
	text := evalString(t, ctx, `(function () {
		var dts = document.querySelectorAll('.dxg-detail-rows dt');
		for (var i = 0; i < dts.length; i++) {
			if (dts[i].textContent === 'degree (view)') { return dts[i].nextElementSibling.textContent; }
		}
		return '';
	})()`)
	noted := evalBool(t, ctx, `!!document.querySelector('.dxg-detail-rows .dxg-detail-note')`)
	return text, noted
}

func TestGraphDetailDegreeNamesTheNodeItsNumbersBelongTo(t *testing.T) {
	p := newRailProject(t)
	ctx := staticGraphTab(t, p)
	openGraphPane(t, ctx)

	// widget.contract.base has NO edges. Its module has one. Selecting it here,
	// at claims granularity, is the only place the rail offers a claim id to
	// click; the selection then survives every control change below, which is
	// what puts the panel in each of degreeFor's three cases in turn.
	clickJump(t, ctx, "widget.contract.base")

	t.Run("drawn itself: the numbers are its own and are stated plainly", func(t *testing.T) {
		text, noted := degreeRow(t, ctx)
		if text != "0 — 0 in, 0 out" {
			t.Fatalf("degree (view) = %q, want the claim's own %q", text, "0 — 0 in, 0 out")
		}
		if noted {
			t.Fatal("a node reporting its OWN degree must not carry the 'not this claim's' note")
		}
	})

	t.Run("collapsed away: the number is the group's and says so", func(t *testing.T) {
		setGranularity(t, ctx, "module")
		if got := evalString(t, ctx, `document.querySelector('.dxg-detail-id').textContent`); got != "widget.contract.base" {
			t.Fatalf("selection after collapsing = %q, want it preserved", got)
		}
		text, noted := degreeRow(t, ctx)
		if !noted {
			t.Fatalf("degree (view) = %q with no note: this is the module's degree printed under a claim's name", text)
		}
		// The number is still shown — "the module this claim collapsed into
		// has one edge" is useful — but it is attributed, and it names the
		// node it belongs to.
		if !strings.Contains(text, "module:widget") {
			t.Fatalf("degree (view) = %q, want it to name module:widget as the owner of the numbers", text)
		}
		if !strings.Contains(text, "not this claim") {
			t.Fatalf("degree (view) = %q, want it to say the numbers are not this claim's", text)
		}
		if !strings.HasPrefix(text, "1 — 0 in, 1 out") {
			t.Fatalf("degree (view) = %q, want it to open with the module's real degree (1 — 0 in, 1 out)", text)
		}
	})

	t.Run("not drawn at all: no number is offered", func(t *testing.T) {
		setGranularity(t, ctx, "claims")
		setScopeModule(t, ctx, "gadget")
		text, noted := degreeRow(t, ctx)
		if text != "not drawn in this view" {
			t.Fatalf("degree (view) for an out-of-scope claim = %q, want %q", text, "not drawn in this view")
		}
		if noted {
			t.Fatal("there is no owner to name when nothing is drawn")
		}
	})
}

// ---------------------------------------------------------------------
// Finding 5 — the legend describes all three relations, and follows the overlay
// ---------------------------------------------------------------------

func legendEdgeRows(t *testing.T, ctx context.Context) []string {
	t.Helper()
	return evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend [data-dxg-edge]'))
		.map(function (e) { return e.getAttribute('data-dxg-edge'); })`)
}

func legendGroups(t *testing.T, ctx context.Context) []string {
	t.Helper()
	return evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend .dxg-legend-group'))
		.map(function (e) { return e.textContent; })`)
}

func legendOverlayKeys(t *testing.T, ctx context.Context) []string {
	t.Helper()
	return evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend [data-dxg-overlay-key]'))
		.map(function (e) { return e.getAttribute('data-dxg-overlay-key'); })`)
}

func legendFacetNames(t *testing.T, ctx context.Context) []string {
	t.Helper()
	return evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend [data-dxg-facet]'))
		.map(function (e) { return e.getAttribute('data-dxg-facet'); })`)
}

func TestGraphLegendDescribesEveryRelationAndFollowsTheOverlay(t *testing.T) {
	p := newGraphProject(t)
	ctx := staticGraphTab(t, p)
	openGraphPane(t, ctx)

	wantEdges := []string{"rests_on", "mirrors", "governed_by"}

	cases := []struct {
		name     string
		overlay  string
		group    string   // the caption over the first block
		facets   []string // facet rows, or none while an overlay is active
		overlays []string // overlay rows, keyed off the swatch class
	}{
		{
			name:    "no overlay: the facets",
			overlay: "none",
			group:   "facets",
			facets:  []string{"contract", "design"},
		},
		{
			name:     "an overlay describes what it painted, not the facets",
			overlay:  "status",
			group:    "draft vs locked",
			overlays: []string{"accent", "halo"},
		},
		{
			name:     "and it changes again with the overlay",
			overlay:  "governance",
			group:    "governance",
			overlays: []string{"governed", "dim"},
		},
		{
			name:     "isolated",
			overlay:  "isolated",
			group:    "isolated & weakly linked",
			overlays: []string{"warn", "dim"},
		},
		{
			name:    "and back",
			overlay: "none",
			group:   "facets",
			facets:  []string{"contract", "design"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setOverlay(t, ctx, tc.overlay)

			// ALL THREE RELATIONS, ALWAYS. The strip used to name governed_by
			// alone, leaving rests_on and mirrors to be told apart by an
			// arrowhead — the part of an edge most often hidden under the node
			// it points at. An overlay recolours fills and never changes what
			// a line means, so this block is invariant across the table.
			if got := legendEdgeRows(t, ctx); fmt.Sprint(got) != fmt.Sprint(wantEdges) {
				t.Fatalf("legend edge rows under overlay %q = %v, want %v", tc.overlay, got, wantEdges)
			}
			if n := evalInt(t, ctx, `document.querySelectorAll('.dxg-legend [data-dxg-edge] svg').length`); n != 3 {
				t.Fatalf("legend edge samples that actually draw a line = %d, want 3", n)
			}

			groups := legendGroups(t, ctx)
			if len(groups) != 2 || groups[0] != tc.group || groups[1] != "Relationships" {
				t.Fatalf("legend captions under overlay %q = %v, want [%q Relationships]", tc.overlay, groups, tc.group)
			}

			facets := legendFacetNames(t, ctx)
			keys := legendOverlayKeys(t, ctx)
			if fmt.Sprint(facets) != fmt.Sprint(tc.facets) {
				t.Fatalf("legend facet rows under overlay %q = %v, want %v", tc.overlay, facets, tc.facets)
			}
			if fmt.Sprint(keys) != fmt.Sprint(tc.overlays) {
				t.Fatalf("legend overlay rows under overlay %q = %v, want %v", tc.overlay, keys, tc.overlays)
			}
		})
	}

	// An edge type the reader turned off is not on the canvas, and the strip
	// says so rather than describing a line that is not there.
	runCDP(t, ctx, chromedp.Click(`[data-dxg-type="mirrors"]`, chromedp.ByQuery))
	hidden := evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend [data-dxg-edge]'))
		.filter(function (e) { return e.textContent.indexOf('(hidden)') >= 0; })
		.map(function (e) { return e.getAttribute('data-dxg-edge'); })`)
	if fmt.Sprint(hidden) != fmt.Sprint([]string{"mirrors"}) {
		t.Fatalf("legend rows marked hidden = %v, want [mirrors] after toggling it off", hidden)
	}
	if got := legendEdgeRows(t, ctx); fmt.Sprint(got) != fmt.Sprint(wantEdges) {
		t.Fatalf("legend edge rows after a toggle = %v, want all three still described %v", got, wantEdges)
	}
}
