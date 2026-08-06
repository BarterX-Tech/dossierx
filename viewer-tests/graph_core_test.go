package viewertests

// graph-core.js is the DossierX claims graph's pure-computation half: scope
// filtering, the representative-node rule, edge aggregation, scope-relative
// degrees, iterative Tarjan SCC, facet slot assignment, the governance
// channels, the gap rules and the hash-state codec. It has no DOM, no canvas
// and no global beyond one namespace — which is exactly what lets this file
// prove all of it through a single chromedp.Evaluate against ONE loaded page.
//
// THE TEST SHAPE HERE IS LOAD-BEARING, NOT STYLISTIC.
//
// One Go test func = one page load = N table cases. A case in that shape costs
// about 0.00s; the naive one-test-func-per-case shape costs about 1.0s each
// (browser tab setup, navigation, a render), so a 100-case suite is either
// free or a hundred seconds. browserContext also imposes a 60s per-tab
// ceiling, so the cases are split across three funcs rather than piled into
// one that creeps toward it. All three share the TestGraphCore prefix so a
// single -run pattern still runs the lot.
//
// THE BOUNDARY. Inputs cross as json.Marshal + Sprintf into the expression
// string; outputs cross as CDP returnByValue and are compared as canonical
// JSON — both sides re-marshalled by encoding/json, so map key order is
// Go's on both sides and never the browser's. That is why every exported
// client function takes and returns plain JSON-able values only: a Map or a
// Set arrives here as {} and a cyclic object arrives not at all.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// coreCase is one assertion against one exported function.
//
// The common shape is fn + args: the expression is built as
// window.dossierxGraphCore.<fn>(<args as JSON>), optionally with post
// appended to project the result down to the part under test (".repByClaim",
// a .map to ids). A case whose INPUT is too large to ship from Go — the
// 10,000-node chain that proves the SCC walk is iterative — sets expr
// instead and builds its own input in the browser.
type coreCase struct {
	name string
	fn   string
	args []any
	post string
	expr string
	want any
}

func (c coreCase) expression(t *testing.T) string {
	t.Helper()
	if c.expr != "" {
		return c.expr
	}
	parts := make([]string, 0, len(c.args))
	for _, a := range c.args {
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatalf("case %s: marshal argument: %v", c.name, err)
		}
		parts = append(parts, string(b))
	}
	return "window.dossierxGraphCore." + c.fn + "(" + strings.Join(parts, ",") + ")" + c.post
}

// canonicalJSON re-marshals a decoded value through encoding/json so both
// sides of a comparison have Go's key ordering rather than the browser's
// property-insertion order.
func canonicalJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// loadCorePage renders a throwaway project statically and opens it, waiting
// until the core namespace is present. The corpus does not matter: every case
// below passes its own inputs in, so the page is only here to be a browser
// with graph-core.js loaded in it.
func loadCorePage(t *testing.T) context.Context {
	t.Helper()
	p := newProject(t)
	url := p.renderStatic()
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `!!(window.dossierxGraphCore && window.dossierxGraphCore.scc)`)
	return ctx
}

func runCoreCases(t *testing.T, cases []coreCase) {
	t.Helper()
	ctx := loadCorePage(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			expr := tc.expression(t)
			var got any
			if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &got)); err != nil {
				t.Fatalf("evaluate %s: %v", expr, err)
			}
			gotJSON := canonicalJSON(t, got)
			wantJSON := canonicalJSON(t, tc.want)
			if gotJSON != wantJSON {
				t.Fatalf("%s\n  expr: %s\n   got: %s\n  want: %s", tc.name, expr, gotJSON, wantJSON)
			}
		})
	}
}

// ---------------------------------------------------------------------
// Shared fixtures
// ---------------------------------------------------------------------

// coreNodes is the payload-shaped node set most cases run over: three
// modules, two facets, one locked claim carrying review_pending, one draft
// claim carrying open comments, and one claim (c.one) that no edge touches.
func coreNodes() []any {
	return []any{
		map[string]any{"id": "a.one", "module": "a", "facet": "contract", "status": "locked", "build_role": "api", "review_pending": true, "open_comments": 0},
		map[string]any{"id": "a.two", "module": "a", "facet": "contract", "status": "draft", "build_role": "", "review_pending": false, "open_comments": 2},
		map[string]any{"id": "b.one", "module": "b", "facet": "contract", "status": "draft", "build_role": "", "review_pending": false, "open_comments": 0},
		map[string]any{"id": "c.one", "module": "c", "facet": "schema", "status": "draft", "build_role": "", "review_pending": false, "open_comments": 0},
	}
}

// coreEdges pairs with coreNodes: one cross-module rests_on and one
// intra-module mirrors, so scope, aggregation and the connectivity rules all
// have something to disagree about.
func coreEdges() []any {
	return []any{
		map[string]any{"from": "a.one", "to": "b.one", "type": "rests_on"},
		map[string]any{"from": "a.two", "to": "a.one", "type": "mirrors"},
	}
}

// coreNodesJSON is coreNodes() as a JS array literal, for the handful of cases
// that build their own expression instead of going through fn + args — the ones
// asking what happens when an ARGUMENT IS ABSENT, which fn + args cannot say
// because it always passes exactly the arguments it is given.
var coreNodesJSON = func() string {
	b, err := json.Marshal(coreNodes())
	if err != nil {
		panic("marshal coreNodes: " + err.Error())
	}
	return string(b)
}()

func edge(from, to, typ string) map[string]any {
	return map[string]any{"from": from, "to": to, "type": typ}
}

func fact(rule string, ids ...string) map[string]any {
	return finding(rule, "fact", ids)
}

func hint(rule string, ids ...string) map[string]any {
	return finding(rule, "hint", ids)
}

func finding(rule, kind string, ids []string) map[string]any {
	list := []any{}
	for _, id := range ids {
		list = append(list, id)
	}
	return map[string]any{"rule": rule, "node_ids": list, "kind": kind}
}

// ---------------------------------------------------------------------
// Constants, helpers, scope, representatives, aggregation, degrees
// ---------------------------------------------------------------------

func TestGraphCoreScopeRepresentativesAndEdges(t *testing.T) {
	groupNodeA := map[string]any{
		"id": "module:a", "title": "a", "kind": "group", "group_type": "module",
		"group_name": "a", "module": "a", "facet": "", "size": 2,
		"members": []any{"a.one", "a.two"},
	}
	groupNodeB := map[string]any{
		"id": "module:b", "title": "b", "kind": "group", "group_type": "module",
		"group_name": "b", "module": "b", "facet": "", "size": 1,
		"members": []any{"b.one"},
	}
	groupNodeC := map[string]any{
		"id": "module:c", "title": "c", "kind": "group", "group_type": "module",
		"group_name": "c", "module": "c", "facet": "", "size": 1,
		"members": []any{"c.one"},
	}

	runCoreCases(t, []coreCase{
		// The exported constants are a stated API: the pane, the CSS ramp and
		// this suite all key off them, so they are pinned rather than assumed.
		{name: "EDGE_TYPES", expr: "window.dossierxGraphCore.EDGE_TYPES",
			want: []any{"rests_on", "mirrors", "governed_by"}},
		{name: "DIRECTED_EDGE_TYPES excludes mirrors", expr: "window.dossierxGraphCore.DIRECTED_EDGE_TYPES",
			want: []any{"rests_on", "governed_by"}},
		{name: "GHOST_PREFIX", expr: "window.dossierxGraphCore.GHOST_PREFIX", want: "ghost:"},
		{name: "FACET_SLOT_COUNT", expr: "window.dossierxGraphCore.FACET_SLOT_COUNT", want: 20},
		{name: "FACT_RULE_IDS", expr: "window.dossierxGraphCore.FACT_RULE_IDS",
			want: []any{"cycle", "self_edge", "isolated", "weakly_linked", "review_pending", "open_threads", "sink_group", "orphan_group"}},
		{name: "HINT_RULE_IDS", expr: "window.dossierxGraphCore.HINT_RULE_IDS",
			want: []any{"missing_build_phase", "density_outlier"}},
		{name: "OVERLAYS", expr: "window.dossierxGraphCore.OVERLAYS",
			want: []any{"none", "isolated", "cycles", "governance", "review", "comments", "status"}},
		{name: "BUILD_PHASES", expr: "window.dossierxGraphCore.BUILD_PHASES",
			want: []any{"orientation", "schema", "behavior", "api", "verification"}},

		{name: "groupId", fn: "groupId", args: []any{"module", "a"}, want: "module:a"},
		{name: "edgeKey", fn: "edgeKey", args: []any{edge("a", "b", "rests_on")}, want: "a|rests_on|b"},
		{name: "edgeKey of an unusable edge is empty", fn: "edgeKey", args: []any{map[string]any{"from": "a"}}, want: ""},

		// scopeFilter is TWO INDEPENDENT AXES intersected, and returns the node
		// objects themselves, unchanged.
		//
		// coreNodes is shaped to make every row here distinguishable: module a
		// holds two contract claims, module b one contract claim, module c one
		// schema claim. So `contract` spans two modules, `a`×`contract` is a
		// strict subset of module a's claims, and `a`×`schema` is empty.
		{name: "scopeFilter module keeps only that module", fn: "scopeFilter",
			args: []any{coreNodes(), "a", ""},
			want: []any{coreNodes()[0], coreNodes()[1]}},
		{name: "scopeFilter facet", fn: "scopeFilter", args: []any{coreNodes(), "", "schema"},
			post: ".map(function (n) { return n.id; })", want: []any{"c.one"}},
		// The view the single flat control could never express alongside a
		// module: one facet ACROSS EVERY MODULE. a and b both contribute.
		{name: "scopeFilter facet alone spans every module", fn: "scopeFilter",
			args: []any{coreNodes(), "", "contract"},
			post: ".map(function (n) { return n.id; })", want: []any{"a.one", "a.two", "b.one"}},
		{name: "scopeFilter both axes empty is the whole project", fn: "scopeFilter",
			args: []any{coreNodes(), "", ""},
			post: ".map(function (n) { return n.id; })", want: []any{"a.one", "a.two", "b.one", "c.one"}},
		// THE INTERSECTION. b.one is in facet contract and c.one is in module
		// c, so a filter that ORed the axes would return both; an AND returns
		// neither, and only a.one/a.two are in both.
		{name: "scopeFilter intersects the two axes", fn: "scopeFilter",
			args: []any{coreNodes(), "a", "contract"},
			post: ".map(function (n) { return n.id; })", want: []any{"a.one", "a.two"}},
		{name: "scopeFilter drops a claim in the module but not the facet", fn: "scopeFilter",
			args: []any{coreNodes(), "c", "contract"},
			post: ".map(function (n) { return n.id; })", want: []any{}},
		// An empty intersection is a real answer, not a fault: module a simply
		// has no schema claims. The pane states the pair in words rather than
		// widening it back out behind the reader's back.
		{name: "scopeFilter an empty intersection is empty", fn: "scopeFilter",
			args: []any{coreNodes(), "a", "schema"},
			post: ".map(function (n) { return n.id; })", want: []any{}},
		// "" is the only "all" sentinel, so nothing is reserved: a project may
		// name a module `all` and scoping to it means that module, not the lot.
		{name: "scopeFilter treats all as an ordinary name", fn: "scopeFilter",
			args: []any{coreNodes(), "all", ""},
			post: ".map(function (n) { return n.id; })", want: []any{}},
		// Total on junk: null and undefined are "" on their axis, so a
		// half-built call filters on the axis it was given and nothing else.
		{name: "scopeFilter is total on null axes", fn: "scopeFilter",
			args: []any{coreNodes(), nil, nil},
			post: ".map(function (n) { return n.id; })", want: []any{"a.one", "a.two", "b.one", "c.one"}},
		{name: "scopeFilter with a missing facet argument filters on module only",
			expr: `window.dossierxGraphCore.scopeFilter(` + coreNodesJSON + `, 'b').map(function (n) { return n.id; })`,
			want: []any{"b.one"}},

		{name: "representatives claims map every claim to itself", fn: "representatives",
			args: []any{coreNodes(), "claims", []any{}}, post: ".repByClaim",
			want: map[string]any{"a.one": "a.one", "a.two": "a.two", "b.one": "b.one", "c.one": "c.one"}},
		{name: "representatives module synthesises group nodes", fn: "representatives",
			args: []any{coreNodes(), "module", []any{}},
			want: map[string]any{
				"repByClaim": map[string]any{"a.one": "module:a", "a.two": "module:a", "b.one": "module:b", "c.one": "module:c"},
				"repNodes":   []any{groupNodeA, groupNodeB, groupNodeC},
			}},
		// The expanded set is the per-group override a double-click writes,
		// and it takes either a bare group name or a full group id.
		{name: "representatives honour an expanded group by bare name", fn: "representatives",
			args: []any{coreNodes(), "module", []any{"a"}}, post: ".repByClaim",
			want: map[string]any{"a.one": "a.one", "a.two": "a.two", "b.one": "module:b", "c.one": "module:c"}},
		{name: "representatives honour an expanded group by id", fn: "representatives",
			args: []any{coreNodes(), "module", []any{"module:a"}},
			post: ".repNodes.map(function (n) { return n.id; })",
			want: []any{"a.one", "a.two", "module:b", "module:c"}},
		{name: "representatives facet granularity", fn: "representatives",
			args: []any{coreNodes(), "facet", []any{}}, post: ".repByClaim",
			want: map[string]any{"a.one": "facet:contract", "a.two": "facet:contract", "b.one": "facet:contract", "c.one": "facet:schema"}},

		// Aggregation: the type toggle drops mirrors, two claim-level edges
		// collapse into one weighted group edge, and the intra-module edge
		// becomes a self-loop and is dropped rather than drawn.
		{name: "aggregateEdges collapses, weights and drops self-loops", fn: "aggregateEdges",
			args: []any{
				[]any{
					edge("a.one", "b.one", "rests_on"),
					edge("a.two", "b.one", "rests_on"),
					edge("a.one", "a.two", "rests_on"),
					edge("a.one", "b.one", "mirrors"),
				},
				map[string]any{"a.one": "module:a", "a.two": "module:a", "b.one": "module:b"},
				[]any{"rests_on"},
			},
			want: []any{map[string]any{"from": "module:a", "to": "module:b", "type": "rests_on", "weight": 2}}},
		// An endpoint with no representative becomes a ghost, so scoping never
		// hides that a claim reaches outward; an edge between two ghosts is
		// not this view's business and is dropped.
		{name: "aggregateEdges ghosts an out-of-scope endpoint", fn: "aggregateEdges",
			args: []any{
				[]any{edge("a.one", "z.zz", "rests_on"), edge("q.q", "z.zz", "rests_on")},
				map[string]any{"a.one": "a.one"},
				nil,
			},
			want: []any{map[string]any{"from": "a.one", "to": "ghost:z.zz", "type": "rests_on", "weight": 1}}},

		{name: "degrees are scope-relative and count both ends", fn: "degrees",
			args: []any{
				[]any{"x", "y"},
				[]any{[]any{"x", "y"}, edge("y", "x", "mirrors"), edge("x", "q", "rests_on")},
			},
			want: map[string]any{
				"x": map[string]any{"in": 1, "out": 2, "total": 3},
				"y": map[string]any{"in": 1, "out": 1, "total": 2},
			}},
		{name: "degrees honour an aggregated edge's weight", fn: "degrees",
			args: []any{
				[]any{"x", "y"},
				[]any{map[string]any{"from": "x", "to": "y", "type": "rests_on", "weight": 3}},
			},
			want: map[string]any{
				"x": map[string]any{"in": 0, "out": 3, "total": 3},
				"y": map[string]any{"in": 3, "out": 0, "total": 3},
			}},
	})
}

// ---------------------------------------------------------------------
// SCC, self-edges, facet slots and the governance channels
// ---------------------------------------------------------------------

func TestGraphCoreStructureAndChannels(t *testing.T) {
	// facets23 is longer than the palette so the wrap is exercised with a
	// real list rather than an assertion about arithmetic.
	facets23 := []any{}
	for i := 0; i < 23; i++ {
		facets23 = append(facets23, "f"+string(rune('a'+i)))
	}

	govEdges := []any{
		edge("a", "g", "governed_by"),
		edge("b", "g", "governed_by"),
		edge("a", "b", "rests_on"),
	}

	runCoreCases(t, []coreCase{
		// The loop that neither engine cycle lint could see before v0.5.0:
		// one rests_on hop and one governed_by hop. scc walks the union.
		{name: "scc finds a mixed rests_on/governed_by cycle", fn: "scc",
			args: []any{[]any{"a", "b"}, []any{edge("a", "b", "rests_on"), edge("b", "a", "governed_by")}},
			want: []any{[]any{"a", "b"}}},
		{name: "scc ignores mirrors", fn: "scc",
			args: []any{[]any{"a", "b"}, []any{edge("a", "b", "mirrors"), edge("b", "a", "mirrors")}},
			want: []any{}},
		// A singleton is a component only when it carries a literal directed
		// self-edge; every other singleton is just a node.
		{name: "scc returns a singleton only for a self-edge", fn: "scc",
			args: []any{[]any{"a", "b"}, []any{edge("a", "a", "rests_on")}},
			want: []any{[]any{"a"}}},
		{name: "scc does not count a mirrors self-edge", fn: "scc",
			args: []any{[]any{"a"}, []any{edge("a", "a", "mirrors")}},
			want: []any{}},
		{name: "scc separates disjoint components, ordered by smallest member", fn: "scc",
			args: []any{
				[]any{"a", "b", "c", "d"},
				[]any{[]any{"c", "d"}, []any{"d", "c"}, []any{"a", "b"}, []any{"b", "a"}},
			},
			want: []any{[]any{"a", "b"}, []any{"c", "d"}}},
		{name: "scc drops an edge whose endpoint is not in the node set", fn: "scc",
			args: []any{[]any{"a"}, []any{edge("a", "gone", "rests_on"), edge("gone", "a", "rests_on")}},
			want: []any{}},

		// The two cases that prove the walk is iterative rather than
		// recursive: a 10,000-link chain, and the same chain closed into one
		// component. A recursive Tarjan blows the JS stack in the low
		// thousands, so a stack overflow here fails the case rather than
		// hiding.
		{name: "scc walks a 10000-link chain without recursing",
			expr: `(function () {
				var ids = [], edges = [];
				for (var i = 0; i < 10000; i++) {
					var id = 'n' + ('0000' + i).slice(-5);
					ids.push(id);
					if (i > 0) { edges.push({ from: 'n' + ('0000' + (i - 1)).slice(-5), to: id, type: 'rests_on' }); }
				}
				return window.dossierxGraphCore.scc(ids, edges).length;
			})()`,
			want: 0},
		{name: "scc closes a 10000-link chain into one component",
			expr: `(function () {
				var ids = [], edges = [];
				for (var i = 0; i < 10000; i++) {
					var id = 'n' + ('0000' + i).slice(-5);
					ids.push(id);
					if (i > 0) { edges.push({ from: 'n' + ('0000' + (i - 1)).slice(-5), to: id, type: 'rests_on' }); }
				}
				edges.push({ from: 'n09999', to: 'n00000', type: 'rests_on' });
				var r = window.dossierxGraphCore.scc(ids, edges);
				return { components: r.length, size: r[0].length, first: r[0][0], last: r[0][r[0].length - 1] };
			})()`,
			want: map[string]any{"components": 1, "size": 10000, "first": "n00000", "last": "n09999"}},

		// self_edge is reported under its own name and over ALL three types,
		// because the engine has a dedicated self-edge lint distinct from
		// cycle and the rail must tell the same story check does.
		{name: "selfEdges spans every edge type", fn: "selfEdges",
			args: []any{[]any{"a", "b"}, []any{edge("a", "a", "mirrors"), edge("b", "b", "governed_by")}},
			want: []any{"a", "b"}},
		{name: "selfEdges ignores an id outside the node set", fn: "selfEdges",
			args: []any{[]any{"a"}, []any{edge("z", "z", "rests_on")}},
			want: []any{}},

		// Slots are assigned by POSITION in the project's own facet list,
		// never by name — the engine hardcodes no facet name anywhere.
		{name: "facetSlot first", fn: "facetSlot", args: []any{facets23, "fa"}, want: 0},
		{name: "facetSlot last before the wrap", fn: "facetSlot", args: []any{facets23, "ft"}, want: 19},
		{name: "facetSlot wraps at 20", fn: "facetSlot", args: []any{facets23, "fu"}, want: 0},
		{name: "facetSlot past the wrap", fn: "facetSlot", args: []any{facets23, "fw"}, want: 2},
		{name: "facetSlot of an unknown facet is the other slot", fn: "facetSlot", args: []any{facets23, "nope"}, want: -1},
		{name: "facetSlot of no facet is the other slot", fn: "facetSlot", args: []any{facets23, ""}, want: -1},

		// A claim declares governed_by, so the edge runs claim -> governor and
		// the wedge marker belongs on `to`.
		{name: "governors are the targets of governance edges", fn: "governors",
			args: []any{govEdges}, want: []any{"g"}},
		{name: "governanceScope keeps only governance edges", fn: "governanceScope",
			args: []any{govEdges},
			want: map[string]any{
				"nodeIds":  []any{"a", "b", "g"},
				"edgeKeys": []any{"a|governed_by|g", "b|governed_by|g"},
			}},
	})
}

// ---------------------------------------------------------------------
// Gap rules and the hash-state codec
// ---------------------------------------------------------------------

func TestGraphCoreVerdictsAndHashState(t *testing.T) {
	// densityNodes: two modules carrying three contract claims each and a
	// third carrying none, which is the shape the density heuristic exists to
	// notice. Nothing is locked, so the other heuristic stays silent and the
	// two are proven apart rather than together.
	densityNodes := []any{}
	for _, m := range []string{"m1", "m2"} {
		for i := 0; i < 3; i++ {
			densityNodes = append(densityNodes, map[string]any{
				"id": m + ".contract." + string(rune('a'+i)), "module": m, "facet": "contract", "status": "draft",
			})
		}
	}
	densityNodes = append(densityNodes, map[string]any{
		"id": "m3.schema.a", "module": "m3", "facet": "schema", "status": "draft",
	})

	// A cycle that alternates edge types, with only mirrors enabled: the
	// structural rules must ignore the edge-type toggles while the
	// connectivity rules honour them.
	mixedCycle := []any{edge("a.one", "a.two", "rests_on"), edge("a.two", "a.one", "governed_by")}

	runCoreCases(t, []coreCase{
		{name: "gapRules facts over the default type set", fn: "gapRules",
			args: []any{coreNodes(), coreEdges(), map[string]any{}}, post: ".facts",
			want: []any{
				fact("cycle"),
				fact("self_edge"),
				fact("isolated", "c.one"),
				fact("weakly_linked", "a.two", "b.one"),
				fact("review_pending", "a.one"),
				fact("open_threads", "a.two"),
				fact("sink_group", "module:a"),
				fact("orphan_group", "module:c"),
			}},
		// Turning mirrors off moves a.two into isolated and a.one into
		// weakly_linked: "connected" means connected by the relations the
		// reader is currently looking at.
		{name: "connectivity rules honour the edge-type toggles", fn: "gapRules",
			args: []any{coreNodes(), coreEdges(), map[string]any{"enabledTypes": []any{"rests_on"}}},
			post: ".facts.filter(function (f) { return f.rule === 'isolated' || f.rule === 'weakly_linked'; })",
			want: []any{
				fact("isolated", "a.two", "c.one"),
				fact("weakly_linked", "a.one", "b.one"),
			}},
		// ...while the structural rules do not: a cycle is drawn in every
		// overlay and must not be hideable behind a toggle.
		{name: "structural rules ignore the edge-type toggles", fn: "gapRules",
			args: []any{coreNodes(), mixedCycle, map[string]any{"enabledTypes": []any{"mirrors"}}},
			post: ".facts.filter(function (f) { return f.rule === 'cycle'; })",
			want: []any{fact("cycle", "a.one", "a.two")}},
		{name: "gapRules groups by facet on request", fn: "gapRules",
			args: []any{coreNodes(), coreEdges(), map[string]any{"groupBy": "facet"}},
			post: ".facts.filter(function (f) { return f.rule === 'sink_group' || f.rule === 'orphan_group'; })",
			want: []any{
				fact("sink_group"),
				fact("orphan_group", "facet:contract", "facet:schema"),
			}},
		{name: "hints are kept apart from facts", fn: "gapRules",
			args: []any{coreNodes(), coreEdges(), map[string]any{}}, post: ".hints",
			want: []any{hint("missing_build_phase", "module:a"), hint("density_outlier")}},
		{name: "the density heuristic fires on a thin facet", fn: "gapRules",
			args: []any{densityNodes, []any{}, map[string]any{}}, post: ".hints",
			want: []any{hint("missing_build_phase"), hint("density_outlier", "module:m3")}},

		{name: "encodeState of the default state", fn: "encodeState",
			args: []any{nil}, want: "md=&fc=&gr=claims&ov=none&ty=rmg&lb=1&ex=&se="},
		{name: "encodeState escapes the separator and the delimiters", fn: "encodeState",
			args: []any{map[string]any{
				"scopeModule": "a b", "scopeFacet": "c&d", "granularity": "facet", "overlay": "governance",
				"types": []any{"mirrors"}, "labels": false,
				"expanded": []any{"module:b", "module:a"}, "selected": "x.y",
			}},
			want: "md=a%20b&fc=c%26d&gr=facet&ov=governance&ty=m&lb=0&ex=module%3Aa,module%3Ab&se=x.y"},
		// The two axes are INDEPENDENT in the hash as well as in the control
		// bar: either one alone encodes, and neither implies the other.
		{name: "encodeState carries the module axis alone", fn: "encodeState",
			args: []any{map[string]any{"scopeModule": "viewer"}},
			want: "md=viewer&fc=&gr=claims&ov=none&ty=rmg&lb=1&ex=&se="},
		{name: "encodeState carries the facet axis alone", fn: "encodeState",
			args: []any{map[string]any{"scopeFacet": "contract"}},
			want: "md=&fc=contract&gr=claims&ov=none&ty=rmg&lb=1&ex=&se="},
		{name: "encodeState carries both axes at once", fn: "encodeState",
			args: []any{map[string]any{"scopeModule": "viewer", "scopeFacet": "contract"}},
			want: "md=viewer&fc=contract&gr=claims&ov=none&ty=rmg&lb=1&ex=&se="},
		{name: "decodeState reads both axes back", fn: "decodeState",
			args: []any{"md=viewer&fc=contract"},
			want: map[string]any{
				"scopeModule": "viewer", "scopeFacet": "contract", "granularity": "claims", "overlay": "none",
				"types": []any{"rests_on", "mirrors", "governed_by"}, "labels": true,
				"expanded": []any{}, "selected": "",
			}},
		// Each axis decodes on its own, so a hash carrying one is not silently
		// completed with the other.
		{name: "decodeState reads the module axis alone", fn: "decodeState",
			args: []any{"md=viewer"}, post: ".scopeFacet", want: ""},
		{name: "decodeState reads the facet axis alone", fn: "decodeState",
			args: []any{"fc=contract"}, post: ".scopeModule", want: ""},
		// The retired key. v0.5.0 is the release that introduces this format,
		// so `sc` is owed no shim — it is simply a key this version does not
		// know, and an unknown key falls back to the default rather than being
		// half-understood or throwing.
		{name: "decodeState ignores the retired sc key", fn: "decodeState",
			args: []any{"sc=module%3Aviewer&gr=module"},
			want: map[string]any{
				"scopeModule": "", "scopeFacet": "", "granularity": "module", "overlay": "none",
				"types": []any{"rests_on", "mirrors", "governed_by"}, "labels": true,
				"expanded": []any{}, "selected": "",
			}},
		// Same MEANING, same string: the codec canonicalises the two
		// order-sensitive fields so a hash never churns on array order.
		{name: "encodeState is stable under argument order",
			expr: `window.dossierxGraphCore.encodeState({ types: ['governed_by', 'rests_on'] }) ===
				window.dossierxGraphCore.encodeState({ types: ['rests_on', 'governed_by'] })`,
			want: true},
		{name: "encodeState/decodeState round-trip losslessly",
			expr: `(function () {
				var s = { scopeModule: 'a b', scopeFacet: 'c d', granularity: 'module', overlay: 'cycles',
					types: ['governed_by'], labels: false, expanded: ['facet:d', 'facet:c'], selected: 'a.b' };
				var once = window.dossierxGraphCore.encodeState(s);
				return window.dossierxGraphCore.encodeState(window.dossierxGraphCore.decodeState(once)) === once;
			})()`,
			want: true},
		// The round trip above compares strings, which a codec that dropped
		// BOTH axes would also satisfy. This one reads the fields back.
		{name: "a round trip returns both axes by value",
			expr: `(function () {
				var c = window.dossierxGraphCore;
				var back = c.decodeState(c.encodeState({ scopeModule: 'a b', scopeFacet: 'c d' }));
				return back.scopeModule + '|' + back.scopeFacet;
			})()`,
			want: "a b|c d"},
		{name: "decodeState of an empty string is the default state", fn: "decodeState",
			args: []any{""},
			want: map[string]any{
				"scopeModule": "", "scopeFacet": "", "granularity": "claims", "overlay": "none",
				"types": []any{"rests_on", "mirrors", "governed_by"}, "labels": true,
				"expanded": []any{}, "selected": "",
			}},
		// A key present with an empty value means the empty value; that is
		// what makes "no edge types enabled" survive a round trip.
		{name: "decodeState distinguishes an empty value from an absent key", fn: "decodeState",
			args: []any{"ty="}, post: ".types", want: []any{}},
		// Total on a hand-edited hash: unknown keys, a bare word with no "=",
		// a truncated percent escape on each axis. Nothing throws, and every
		// field a reader did not mangle keeps its default.
		{name: "decodeState is total on a hand-mangled hash", fn: "decodeState",
			args: []any{"ov=nonsense&gr=nonsense&junk&md=%E0%A4%A&fc=%E0%A4%A"},
			want: map[string]any{
				"scopeModule": "%E0%A4%A", "scopeFacet": "%E0%A4%A", "granularity": "claims", "overlay": "none",
				"types": []any{"rests_on", "mirrors", "governed_by"}, "labels": true,
				"expanded": []any{}, "selected": "",
			}},
	})
}
