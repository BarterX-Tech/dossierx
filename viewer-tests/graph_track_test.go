package viewertests

// THE TRACK AXIS: ONE FEATURE'S SUBGRAPH, AND WHO OWNS IT.
//
// A module answers "who guarantees this?" and a facet answers "what kind of
// statement is it?". Neither answers "what does the user get, and is it
// finished?" — that is assembled from claims spread across modules, and it is
// the question a track exists for. The graph pane's third scope axis is how a
// reviewer asks it: select a track and the canvas draws that feature's claims
// instead of the whole registry.
//
// Two properties are worth proving in a browser rather than in Go, because
// both are about what a reader can see:
//
//  1. THE CONTROL IS ABSENT, NOT DISABLED, for a project with no tracks — and
//     so is every other track-shaped thing in the pane. Tracks are optional,
//     and an optional axis nobody opted into must cost the pane nothing: not a
//     control, not a legend row, not a byte of payload, not a character of
//     hash. A disabled select would be a permanent unanswerable question.
//
//  2. OWNS IS DISTINGUISHABLE FROM CITES WITHOUT COLOUR. The one claim that
//     OWNS a track is the feature's own statement; the rest cite it and keep
//     their own modules' guarantees. The canvas draws that difference as an
//     inner ruling in the same ink the status ring uses — a shape channel, not
//     a hue — and the legend and the detail panel say it in words. This file
//     reads all three back.
//
// WHAT THE SCOPED SET IS READ OFF: the fixture carries NO EDGES, so at claims
// granularity every drawn node is `isolated` and the gaps rail's isolated
// block is a faithful list of exactly the claims in scope. Same device as
// graph_scope_test.go, and for the same reason — it is a DOM readout rather
// than a re-run of the core function the pane itself used.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// The fixture declares two tracks and gives each an owner, so `check` passes:
// track-unowned warns about a track with citing claims and no owner, and
// track-unknown refuses an id the config never declared.
const trackConfig = `schema_version: 1
facets:
  - contract
  - design
modules:
  - widget
  - gadget
claims_dir: claims
tracks:
  - id: checkout
    title: Checkout
    summary: taking payment and confirming the order
  - id: search
    title: Search
`

// trackClaimYAML is railClaim plus a memberships block. The role is written
// out for the owner and OMITTED for the citing claim, so this fixture also
// proves the unset role arrives as "cites" rather than as nothing.
func trackClaimYAML(id, facet, module, memberships string) string {
	body := "id: " + id + `
facet: ` + facet + `
module: ` + module + `
status: draft
`
	if memberships != "" {
		body += "tracks:\n" + memberships
	}
	return body + `body: |
  a claim in the ` + facet + ` facet.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
}

// newTrackProject:
//
//	widget.contract.base    OWNS checkout
//	widget.design.thing     cites checkout (role omitted)
//	gadget.contract.core    OWNS search
//	gadget.design.extra     in no track at all
//
// So `checkout` is two claims across two facets of one module, `search` is
// one, and one claim is outside both — which is what makes "every claim" a
// strictly larger set than the union of the tracks, the reason the axis's all
// option does not say "all tracks".
func newTrackProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, trackConfig)
	p.writeClaim("wc.yaml", trackClaimYAML("widget.contract.base", "contract", "widget",
		"  - id: checkout\n    role: owns\n"))
	p.writeClaim("wd.yaml", trackClaimYAML("widget.design.thing", "design", "widget",
		"  - id: checkout\n"))
	p.writeClaim("gc.yaml", trackClaimYAML("gadget.contract.core", "contract", "gadget",
		"  - id: search\n    role: owns\n"))
	p.writeClaim("gd.yaml", trackClaimYAML("gadget.design.extra", "design", "gadget", ""))
	return p
}

func setScopeTrack(t *testing.T, ctx context.Context, track string) {
	t.Helper()
	setScopeSelect(t, ctx, "dxgTrack", track)
}

// innerRingNodes counts the drawn nodes wearing a ring INSIDE their own rim.
// Every other ring the pane draws — the moat, the status ring, the halo, the
// selection ring — sits at the rim or outside it, so an inner ring is the
// track-owner marker and nothing else can be mistaken for it.
func innerRingNodes(t *testing.T, ctx context.Context) int {
	t.Helper()
	f := lastFrame(t, ctx)
	n := 0
	for _, node := range f.Nodes {
		for _, r := range node.Rings {
			if r.R < node.MoatR-2.5 {
				n++
				break
			}
		}
	}
	return n
}

func detailRowText(t *testing.T, ctx context.Context, label string) string {
	t.Helper()
	return evalString(t, ctx, `(function () {
		var dts = document.querySelectorAll('.dxg-detail-rows dt');
		for (var i = 0; i < dts.length; i++) {
			if (dts[i].textContent === `+jsQuote(label)+`) { return dts[i].nextElementSibling.textContent; }
		}
		return '';
	})()`)
}

func TestGraphTrackFilterDrawsOneFeaturesSubgraph(t *testing.T) {
	p := newTrackProject(t)
	ctx := staticGraphTab(t, p)
	installRecorder(t, ctx)
	openGraphPane(t, ctx)
	settleFrames(t, ctx)

	// The control sits with its scope siblings, third of seven, and the four
	// groups after it keep the order a reader's hand has learnt.
	labels := evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-controls .dxg-ctl .dxg-ctl-label'))
		.map(function (e) { return e.textContent; })`)
	want := []string{"Module", "Facet", "Track", "Granularity", "Highlight overlay", "Edge types", "View"}
	if fmt.Sprint(labels) != fmt.Sprint(want) {
		t.Fatalf("control groups = %v, want %v", labels, want)
	}

	// It selects on the ID a claim cites and SHOWS the title the project
	// wrote. A track has a title and a module does not, so deriving a label
	// here would be inventing a name over one that already exists.
	values := evalStrings(t, ctx, `Array.from(document.querySelectorAll('#dxgTrack option')).map(function (o) { return o.value; })`)
	if fmt.Sprint(values) != fmt.Sprint([]string{"", "checkout", "search"}) {
		t.Fatalf("track option values = %v, want the all-sentinel then the two declared ids", values)
	}
	texts := evalStrings(t, ctx, `Array.from(document.querySelectorAll('#dxgTrack option')).map(function (o) { return o.textContent; })`)
	if fmt.Sprint(texts) != fmt.Sprint([]string{"every claim", "Checkout", "Search"}) {
		t.Fatalf("track option labels = %v, want the titles, and an all-option that does not claim to be the union of the tracks", texts)
	}
	if evalBool(t, ctx, `document.getElementById('dxgTrack').disabled`) {
		t.Fatal("the track select is disabled; every axis stays enabled at all times")
	}

	// Nothing about the pane changes until a track is chosen: no marker on any
	// node, no legend rows. The distinction "which claim is this feature's own
	// statement?" is a question about a feature, and no feature is selected.
	if n := innerRingNodes(t, ctx); n != 0 {
		t.Fatalf("owner markers with no track selected = %d, want 0", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('.dxg-legend [data-dxg-track-role]').length`); n != 0 {
		t.Fatalf("track legend rows with no track selected = %d, want 0", n)
	}

	cases := []struct {
		track string
		want  []string
	}{
		{track: "checkout", want: []string{"widget.contract.base", "widget.design.thing"}},
		{track: "search", want: []string{"gadget.contract.core"}},
		{track: "", want: []string{
			"gadget.contract.core", "gadget.design.extra",
			"widget.contract.base", "widget.design.thing",
		}},
	}
	for _, tc := range cases {
		name := tc.track
		if name == "" {
			name = "every claim"
		}
		t.Run(name, func(t *testing.T) {
			setScopeTrack(t, ctx, tc.track)
			settleFrames(t, ctx)
			got := scopedClaimIDs(t, ctx)
			sort.Strings(got)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("claims in scope = %v, want %v", got, tc.want)
			}
			// Anchored to pixels as well as to the rail, so neither number is
			// trusted on its own.
			if n := drawnDiscCount(t, ctx); n != len(tc.want) {
				t.Fatalf("drawn discs = %d, want %d", n, len(tc.want))
			}
		})
	}

	// ---- owns vs cites, on all three channels --------------------------

	setScopeTrack(t, ctx, "checkout")
	settleFrames(t, ctx)

	// 1. The canvas. Exactly one of the two drawn nodes wears the inner
	//    ruling, and it is a SHAPE — no colour is being asked to carry it.
	if n := innerRingNodes(t, ctx); n != 1 {
		t.Fatalf("owner markers in track checkout = %d, want exactly 1 (the owning claim)", n)
	}

	// 2. The legend, which names both roles rather than leaving "no marker" to
	//    be inferred, and names the track by its title.
	roles := evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend [data-dxg-track-role]'))
		.map(function (e) { return e.getAttribute('data-dxg-track-role'); })`)
	if fmt.Sprint(roles) != fmt.Sprint([]string{"owns", "cites"}) {
		t.Fatalf("track legend rows = %v, want a row for each role", roles)
	}
	if got := evalStrings(t, ctx, `Array.from(document.querySelectorAll('.dxg-legend .dxg-legend-group'))
		.map(function (e) { return e.textContent; })`); !strings.Contains(fmt.Sprint(got), "Checkout") {
		t.Fatalf("legend group labels = %v, want one naming the selected track by title", got)
	}

	// 3. The detail panel, in words — the channel that works with no track
	//    selected and nothing marked. The owner says owns; the claim that
	//    wrote no role at all says cites, because an unset role MEANS cites
	//    and Go resolves it before it reaches the wire.
	runCDP(t, ctx, chromedp.Click(`[data-dxg-jump="widget.contract.base"]`, chromedp.ByQuery))
	if got := detailRowText(t, ctx, "tracks"); got != "Checkout (owns)" {
		t.Fatalf("owner's tracks row = %q, want Checkout (owns)", got)
	}
	runCDP(t, ctx, chromedp.Click(`[data-dxg-jump="widget.design.thing"]`, chromedp.ByQuery))
	if got := detailRowText(t, ctx, "tracks"); got != "Checkout (cites)" {
		t.Fatalf("citing claim's tracks row = %q, want Checkout (cites)", got)
	}

	// The selection rides in the hash, so one feature's subgraph is a link
	// somebody can send. It is written only when it carries a selection —
	// see graph-core.js's encodeState for why that key alone is conditional.
	if got := evalString(t, ctx, `window.location.hash`); !strings.Contains(got, "tk=checkout") {
		t.Fatalf("hash = %q, want it to carry tk=checkout", got)
	}
	setScopeTrack(t, ctx, "")
	if got := evalString(t, ctx, `window.location.hash`); strings.Contains(got, "tk=") {
		t.Fatalf("hash = %q, want no tk key once no track is selected", got)
	}
}

// TestGraphTrackAxisIsAbsentWithoutTracks is the zero-cost half. A project
// that declares no tracks must get the pane it had before the axis existed:
// no control, no legend row, no payload key, no hash key. "Absent" is checked
// rather than "disabled" deliberately — a disabled select promises a
// selection that can never be made.
func TestGraphTrackAxisIsAbsentWithoutTracks(t *testing.T) {
	p := newScopeProject(t) // declares modules and facets, and no tracks
	ctx := staticGraphTab(t, p)
	openGraphPane(t, ctx)

	if n := evalInt(t, ctx, `document.querySelectorAll('#dxgTrack').length`); n != 0 {
		t.Fatalf("track selects = %d, want 0 — absent, not disabled", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('.dxg-controls .dxg-ctl').length`); n != 6 {
		t.Fatalf("control groups = %d, want the six a track-less project has always had", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('.dxg-legend [data-dxg-track-role]').length`); n != 0 {
		t.Fatalf("track legend rows = %d, want 0", n)
	}

	// The payload carries neither new key. internal/graph holds this as bytes
	// too; it is re-read here because it is the DOCUMENT the pane parses, and
	// three tracked fixture viewers carry this block inline — a stray
	// "tracks":null per node would leave every one of them permanently dirty.
	if evalBool(t, ctx, `document.getElementById('dossierx-graph').textContent.indexOf('tracks') >= 0`) {
		t.Fatal("a track-less project's payload mentions tracks")
	}

	// And a link shared out of it is the string it was before the axis
	// existed, rather than one carrying an empty key for a feature nobody
	// declared.
	if got := evalString(t, ctx, `window.dossierxGraphCore.encodeState(window.dossierxGraphCore.defaultState())`); strings.Contains(got, "tk=") {
		t.Fatalf("default encoded state = %q, want no tk key", got)
	}
}
