package viewertests

// SCOPE IS TWO INDEPENDENT AXES, AND THE PANE HAS TO SURVIVE THEIR EMPTY
// INTERSECTION.
//
// One flat "Scope" select listing every module AND every facet was fine on a
// four-claim fixture and unusable on a real one: a dozen modules and eight
// facets is a twenty-one-row dropdown whose two kinds of entry differ only by a
// prefix, and in which "this facet across every module" and "this module" can
// never be asked at the same time. Two selects make the second question
// expressible and each list short enough to read.
//
// Splitting it creates one genuinely new state. An intersection can be EMPTY —
// module `probe` and facet `design` may both hold claims while no claim is in
// both — and the pane's response to a zero-node scene is to draw nothing. A
// blank canvas is exactly what a broken pane looks like, so the empty
// combination has to be STATED, in words, naming both selections. That is the
// assertion this file exists for; the rest establishes that the ordinary,
// non-empty combinations behave.
//
// WHAT THE SCOPED SET IS READ OFF. The fixture below carries NO EDGES, so at
// claims granularity every drawn node is `isolated` and the gaps rail's
// isolated block is a faithful, id-carrying list of exactly the claims in
// scope. That is deliberate on two counts: it is a DOM readout rather than a
// re-run of the same core function the pane used, and — because gapRules
// computes over the drawn representative graph — it simultaneously proves the
// rail is seeing the filtered set rather than the whole payload. The canvas's
// own node count is checked alongside it, so neither number is trusted alone.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// Fixture: three modules × two facets, with one combination missing
// ---------------------------------------------------------------------

const scopeConfig = `schema_version: 1
facets:
  - contract
  - design
modules:
  - widget
  - gadget
  - probe
claims_dir: claims
`

// newScopeProject is shaped so that every row of the axis table below is
// distinguishable from every other, and so that ONE combination is empty while
// neither of its axes is:
//
//	widget  contract + design
//	gadget  contract + design
//	probe   contract ONLY
//
// So `probe` alone has a claim, `design` alone has two, and `probe` × `design`
// has none — the state a single Scope control could not reach. `contract`
// alone spans all THREE modules, which is the cross-module facet view the flat
// list could express only INSTEAD of a module selection, never alongside one.
//
// No claim rests on any other. Every claim is therefore isolated, and the gaps
// rail's isolated block is the scoped claim set — see the file header.
func newScopeProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, scopeConfig)
	p.writeClaim("wc.yaml", railClaim("widget.contract.base", "contract", "widget", ""))
	p.writeClaim("wd.yaml", railClaim("widget.design.thing", "design", "widget", ""))
	p.writeClaim("gc.yaml", railClaim("gadget.contract.core", "contract", "gadget", ""))
	p.writeClaim("gd.yaml", railClaim("gadget.design.extra", "design", "gadget", ""))
	p.writeClaim("pc.yaml", railClaim("probe.contract.only", "contract", "probe", ""))
	return p
}

// scopedClaimIDs returns the claims currently in scope, read off the gaps
// rail's `isolated` block. Valid only for an edge-free corpus at claims
// granularity, which is what newScopeProject is.
func scopedClaimIDs(t *testing.T, ctx context.Context) []string {
	t.Helper()
	ids := ruleIDs(t, ctx, "isolated")
	sort.Strings(ids)
	return ids
}

// drawnDiscCount reads the number of node discs in the last recorded frame
// WITHOUT fataling on zero, which readFrame does and which the empty-scope case
// needs to be able to observe.
func drawnDiscCount(t *testing.T, ctx context.Context) int {
	t.Helper()
	evalVoid(t, ctx, forceDraw)
	var f canvasFrame
	evalInto(t, ctx, frameSummaryJS, &f)
	return len(f.Nodes)
}

// setScopePair drives both selects, in that order, and is the only way this
// file changes scope — passing "" for an axis is that axis's "all".
func setScopePair(t *testing.T, ctx context.Context, module, facet string) {
	t.Helper()
	setScopeModule(t, ctx, module)
	setScopeFacet(t, ctx, facet)
}

// ---------------------------------------------------------------------
// The two axes compose as an intersection
// ---------------------------------------------------------------------

func TestGraphScopeIsTwoIndependentAxesIntersected(t *testing.T) {
	p := newScopeProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)
	settleFrames(t, ctx)

	// Both controls exist, both are populated from the payload's own group
	// lists, and each offers ONLY its own axis — the flat list that mixed the
	// two is what this replaced.
	modules := evalStrings(t, ctx, `Array.from(document.querySelectorAll('#dxgModule option')).map(function (o) { return o.value; })`)
	facets := evalStrings(t, ctx, `Array.from(document.querySelectorAll('#dxgFacet option')).map(function (o) { return o.value; })`)
	if fmt.Sprint(modules) != fmt.Sprint([]string{"", "widget", "gadget", "probe"}) {
		t.Fatalf("module options = %v, want the all-sentinel then the project's three modules", modules)
	}
	if fmt.Sprint(facets) != fmt.Sprint([]string{"", "contract", "design"}) {
		t.Fatalf("facet options = %v, want the all-sentinel then the project's two facets", facets)
	}
	labels := evalStrings(t, ctx, `[document.querySelector('#dxgModule option').textContent,
		document.querySelector('#dxgFacet option').textContent]`)
	if fmt.Sprint(labels) != fmt.Sprint([]string{"all modules", "all facets"}) {
		t.Fatalf("the two all-options read %v, want them to name their own axis", labels)
	}
	// Neither is ever disabled: any pair is a legal selection, including the
	// pairs that select nothing.
	if evalBool(t, ctx, `document.getElementById('dxgModule').disabled || document.getElementById('dxgFacet').disabled`) {
		t.Fatal("a scope select is disabled; both axes stay enabled at all times")
	}

	cases := []struct {
		name   string
		module string
		facet  string
		want   []string
	}{
		{
			name: "both axes all is the whole project",
			want: []string{
				"gadget.contract.core", "gadget.design.extra",
				"probe.contract.only", "widget.contract.base", "widget.design.thing",
			},
		},
		{
			name: "a module alone", module: "widget",
			want: []string{"widget.contract.base", "widget.design.thing"},
		},
		{
			// The view the flat control could not express alongside a module.
			// All three modules contribute a claim to it.
			name: "a facet alone spans every module", facet: "contract",
			want: []string{"gadget.contract.core", "probe.contract.only", "widget.contract.base"},
		},
		{
			// THE INTERSECTION. widget.design.thing is in the module and not
			// the facet; gadget.contract.core is in the facet and not the
			// module. A filter that ORed the axes would keep both. Only the
			// claim in BOTH survives.
			name:   "the intersection drops a claim in the module but not the facet",
			module: "widget", facet: "contract",
			want: []string{"widget.contract.base"},
		},
		{
			name:   "the other intersection, to pin that the first was not a coincidence",
			module: "gadget", facet: "design",
			want: []string{"gadget.design.extra"},
		},
		{
			// Widening one axis while the other stays narrowed: the axes do
			// not latch, and there is no ordering rule between them.
			name:  "widening the module axis alone restores the facet-wide view",
			facet: "design",
			want:  []string{"gadget.design.extra", "widget.design.thing"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			setScopePair(t, ctx, tc.module, tc.facet)

			if got := scopedClaimIDs(t, ctx); fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("claims in scope for module=%q facet=%q = %v, want %v",
					tc.module, tc.facet, got, tc.want)
			}
			// ...and the canvas drew exactly that many discs, so the rail is
			// describing the picture rather than a set only it can see.
			if got := drawnDiscCount(t, ctx); got != len(tc.want) {
				t.Fatalf("discs drawn for module=%q facet=%q = %d, want %d",
					tc.module, tc.facet, got, len(tc.want))
			}
			// A non-empty scope says nothing about being empty.
			if n := noticeText(t, ctx); strings.Contains(n, "no claims") {
				t.Fatalf("notice = %q for a scope holding %d claims", n, len(tc.want))
			}
		})
	}
}

// ---------------------------------------------------------------------
// The empty intersection is stated, and does not read as a broken pane
// ---------------------------------------------------------------------

func TestGraphEmptyScopeIntersectionIsStatedNotBlank(t *testing.T) {
	p := newScopeProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)
	settleFrames(t, ctx)

	// The two controls, each on its own, select something. That is what makes
	// the pair below an INTERSECTION being empty rather than either axis being
	// empty — and without it, the assertions that follow would pass for a
	// reason that has nothing to do with the intersection.
	setScopePair(t, ctx, "probe", "")
	if got := scopedClaimIDs(t, ctx); fmt.Sprint(got) != fmt.Sprint([]string{"probe.contract.only"}) {
		t.Fatalf("module probe alone = %v, want the one claim it holds", got)
	}
	setScopePair(t, ctx, "", "design")
	if got := scopedClaimIDs(t, ctx); fmt.Sprint(got) != fmt.Sprint([]string{"gadget.design.extra", "widget.design.thing"}) {
		t.Fatalf("facet design alone = %v, want the two claims it holds", got)
	}

	// Now the pair. No claim is in both.
	setScopePair(t, ctx, "probe", "design")

	if got := drawnDiscCount(t, ctx); got != 0 {
		t.Fatalf("discs drawn for an empty intersection = %d, want 0", got)
	}
	if got := scopedClaimIDs(t, ctx); len(got) != 0 {
		t.Fatalf("claims in scope for probe × design = %v, want none", got)
	}

	// THE CANVAS IS BLANK, SO THE WORDS ARE THE ONLY THING BETWEEN THE READER
	// AND "the graph broke". They must name BOTH selections.
	notice := noticeText(t, ctx)
	for _, want := range []string{"probe", "design", "no claim is in", "no claims"} {
		if !strings.Contains(notice, want) {
			t.Fatalf("empty-scope notice = %q, want it to contain %q — a reader must be able to tell "+
				"\"this combination has no claims\" from \"the pane broke\", and a blank canvas says neither", notice, want)
		}
	}

	// ...and the pane is otherwise INTACT. A message on a pane that has also
	// lost its canvas, its controls or its selections would still read as
	// broken, so each of those is asserted rather than assumed.
	if n := evalInt(t, ctx, `document.querySelectorAll('#dxgPane .dxg-canvas').length`); n != 1 {
		t.Fatalf("canvases = %d on an empty scope, want the pane still standing", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('.dxg-controls .dxg-ctl').length`); n != 6 {
		t.Fatalf("control groups = %d on an empty scope, want all 6 still there", n)
	}
	if got := evalStrings(t, ctx, `[document.getElementById('dxgModule').value, document.getElementById('dxgFacet').value]`); fmt.Sprint(got) != fmt.Sprint([]string{"probe", "design"}) {
		t.Fatalf("the selects read %v after an empty intersection, want the reader's own selection still shown "+
			"— silently resetting it would hide the combination the notice is naming", got)
	}
	if evalBool(t, ctx, `document.getElementById('dxgModule').disabled || document.getElementById('dxgFacet').disabled`) {
		t.Fatal("a scope select was disabled by the empty result; both axes stay enabled at all times")
	}

	// The notice is not only a message: it offers the way out, and it works.
	// It widens BOTH axes and touches nothing else — the reader's granularity
	// is not what went wrong.
	setGranularity(t, ctx, "module")
	setScopePair(t, ctx, "probe", "design")
	runCDP(t, ctx, chromedp.Click(`.dxg-notices .dxg-notice-action`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.getElementById('dxgModule').value === '' && document.getElementById('dxgFacet').value === ''`)
	if got := evalString(t, ctx, `document.getElementById('dxgGranularity').value`); got != "module" {
		t.Fatalf("granularity after widening the scope = %q, want the reader's own choice of module left alone", got)
	}
	if n := noticeText(t, ctx); strings.Contains(n, "no claim is in") {
		t.Fatalf("notice after widening the scope = %q, want it gone", n)
	}
	if got := drawnDiscCount(t, ctx); got == 0 {
		t.Fatal("the graph did not come back after widening the scope")
	}
}

// A project that is genuinely empty is NOT the filter's doing, and the notice
// must not blame a filter that is not running.
//
// The empty corpus is INJECTED rather than authored, because a project with no
// claims does not render a viewer to open. The pane parses its payload at first
// open, so swapping the block before the first click is enough — the same
// mechanism TestGraphPaneRendersInjectedCycles uses, and for the same reason.
func TestGraphEmptyScopeNoticeIsSilentWhenNeitherAxisFilters(t *testing.T) {
	p := newScopeProject(t)
	ctx := staticGraphTab(t, p)

	evalVoid(t, ctx, `(function () {
		var el = document.getElementById('dossierx-graph');
		var payload = JSON.parse(el.textContent);
		payload.nodes = [];
		payload.edges = [];
		el.textContent = JSON.stringify(payload);
	})();`)

	openGraphPane(t, ctx)

	if got := evalStrings(t, ctx, `[document.getElementById('dxgModule').value, document.getElementById('dxgFacet').value]`); fmt.Sprint(got) != fmt.Sprint([]string{"", ""}) {
		t.Fatalf("the two axes opened at %v, want both at their all-sentinel", got)
	}
	if n := noticeText(t, ctx); strings.Contains(n, "no claim is in") {
		t.Fatalf("notice = %q with neither axis filtering — an empty project is the project's own "+
			"emptiness, and blaming the scope control would send a reader to fix the wrong thing", n)
	}

	// The control. Narrow one axis on that same empty payload and the notice
	// DOES fire, so the silence above is the guard doing its job rather than a
	// notice that never appears on this page at all.
	setScopePair(t, ctx, "widget", "")
	if n := noticeText(t, ctx); !strings.Contains(n, "no claim is in") || !strings.Contains(n, "widget") {
		t.Fatalf("notice = %q once the module axis is narrowed, want it to name the selection", n)
	}
}

// ---------------------------------------------------------------------
// Both axes survive a reload through the hash
// ---------------------------------------------------------------------

func TestGraphScopeHashRoundTripsBothAxesThroughAReload(t *testing.T) {
	p := newScopeProject(t)
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `!!window.dossierxGraphCore`)
	desktopViewport(t, ctx)
	openGraphPane(t, ctx)

	setScopePair(t, ctx, "widget", "contract")
	hash := evalString(t, ctx, `window.location.hash`)
	if !strings.Contains(hash, "md=widget") || !strings.Contains(hash, "fc=contract") {
		t.Fatalf("hash after selecting module widget × facet contract = %q, want both axes in it — "+
			"one key cannot carry two independent selections", hash)
	}

	// A SECOND TAB, navigated to the URL the reader would have pasted. This is
	// a reload rather than a hash assignment on the live page: the state has to
	// arrive with the document, which is the only path that proves the codec
	// rather than the in-memory state object.
	ctx2 := browserContext(t)
	runCDP(t, ctx2, chromedp.Navigate(url+hash))
	pollTrue(t, ctx2, `!!window.dossierxGraphCore`)
	desktopViewport(t, ctx2)
	waitVisible(t, ctx2, "#dxgPane .dxg-canvas")

	if got := evalString(t, ctx2, `document.getElementById('dxgModule').value`); got != "widget" {
		t.Fatalf("module axis after the reload = %q, want widget", got)
	}
	if got := evalString(t, ctx2, `document.getElementById('dxgFacet').value`); got != "contract" {
		t.Fatalf("facet axis after the reload = %q, want contract", got)
	}
	// The restored controls are not decoration: the graph they describe is
	// the intersection, not either axis alone.
	if got := scopedClaimIDs(t, ctx2); fmt.Sprint(got) != fmt.Sprint([]string{"widget.contract.base"}) {
		t.Fatalf("claims in scope after the reload = %v, want the one claim in both axes", got)
	}

	// An EMPTY intersection survives the same trip, message and all — through a
	// shared link is the worst place to meet a blank canvas, because it is the
	// first thing the recipient sees.
	setScopePair(t, ctx, "probe", "design")
	emptyHash := evalString(t, ctx, `window.location.hash`)

	ctx3 := browserContext(t)
	runCDP(t, ctx3, chromedp.Navigate(url+emptyHash))
	pollTrue(t, ctx3, `!!window.dossierxGraphCore`)
	desktopViewport(t, ctx3)
	waitVisible(t, ctx3, "#dxgPane .dxg-canvas")
	if got := evalStrings(t, ctx3, `[document.getElementById('dxgModule').value, document.getElementById('dxgFacet').value]`); fmt.Sprint(got) != fmt.Sprint([]string{"probe", "design"}) {
		t.Fatalf("the selects after reloading an empty-intersection link = %v, want probe and design", got)
	}
	notice := evalString(t, ctx3, `document.querySelector('.dxg-notices').textContent`)
	if !strings.Contains(notice, "probe") || !strings.Contains(notice, "design") {
		t.Fatalf("notice after reloading an empty-intersection link = %q, want it to name both selections", notice)
	}
}

// ---------------------------------------------------------------------
// Ghost stubs still work when the scope is an intersection
// ---------------------------------------------------------------------

// Scoping must never hide that a claim reaches outward, and an intersection is
// a scope like any other. newRailProject's widget.design.thing rests_on
// gadget.contract.core, so module=widget × facet=design puts the source in
// scope and the target outside it along BOTH axes at once — the case a
// single-axis scope could not construct.
func TestGraphGhostStubsSurviveAnIntersectionScope(t *testing.T) {
	p := newRailProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)
	settleFrames(t, ctx)

	setScopePair(t, ctx, "widget", "design")

	evalVoid(t, ctx, forceDraw)
	var f canvasFrame
	evalInto(t, ctx, frameSummaryJS, &f)

	// One claim is in scope. The out-of-scope target is drawn anyway, as a
	// hollow stub filled with the page colour — that is what a ghost is.
	var ghosts, solid int
	for _, n := range f.Nodes {
		if n.Fill != nil && n.Fill.Color == f.Pal["paper"] {
			ghosts++
		} else {
			solid++
		}
	}
	if solid != 1 {
		t.Fatalf("solid nodes under module=widget × facet=design = %d, want 1 (widget.design.thing)", solid)
	}
	if ghosts != 1 {
		t.Fatalf("ghost stubs = %d, want 1 — scoping must never hide that a claim reaches outward, "+
			"and an intersection is a scope like any other", ghosts)
	}
	if len(f.Edges) == 0 {
		t.Fatal("no edge was drawn to the ghost, so the stub stands for nothing")
	}

	// The control: widen the facet axis back out and the target is a real node
	// again, with no ghost left.
	setScopePair(t, ctx, "widget", "")
	evalVoid(t, ctx, forceDraw)
	evalInto(t, ctx, frameSummaryJS, &f)
	ghosts = 0
	for _, n := range f.Nodes {
		if n.Fill != nil && n.Fill.Color == f.Pal["paper"] {
			ghosts++
		}
	}
	if ghosts != 1 {
		t.Fatalf("ghost stubs at module=widget with the facet axis wide = %d, want 1 "+
			"(the cross-module target is still out of scope)", ghosts)
	}
}
