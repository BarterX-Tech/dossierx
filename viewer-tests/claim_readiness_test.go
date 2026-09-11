package viewertests

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

const (
	// These are browser-product budgets, separate from the engine's 64 MiB
	// serialized-output containment gate. They deliberately leave headroom for
	// slower CI runners while still catching an eager Mermaid render, duplicated
	// blocker lists, or a projection that makes the claim page impractical.
	readinessScaleMaxLoadMS      = 10_000
	readinessScaleMaxMapMS       = 5_000
	readinessScaleMaxDOMNodes    = 100_000
	readinessScaleMaxJSHeapBytes = 512 * 1024 * 1024
)

func readinessScaleProject(t *testing.T, layers, width int) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	for layer := layers - 1; layer >= 0; layer-- {
		for node := 0; node < width; node++ {
			id := fmt.Sprintf("widget.contract.l%03d-n%02d", layer, node)
			var restsOn strings.Builder
			if layer+1 < layers {
				restsOn.WriteString("rests_on:\n")
				for dependency := 0; dependency < width; dependency++ {
					fmt.Fprintf(&restsOn, "  - widget.contract.l%03d-n%02d\n", layer+1, dependency)
				}
			}
			p.writeClaim(id+".yaml", fmt.Sprintf(`id: %s
facet: contract
module: widget
status: draft
body: |
  browser scale fixture at layer %d, node %d.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
%s`, id, layer, node, restsOn.String()))
		}
	}
	return p
}

func readinessScaleFacts(layers, width int) int {
	// Every node owns one condition for every unique node in every later layer.
	// Full inter-layer fan-out creates exponentially many routes, but readiness
	// retains one condition per independently clearable dependency identity.
	return width * width * layers * (layers - 1) / 2
}

type readinessScaleMetrics struct {
	LoadMS         float64 `json:"load_ms"`
	DOMNodes       int     `json:"dom_nodes"`
	JSHeapBytes    float64 `json:"js_heap_bytes"`
	ReadinessCards int     `json:"readiness_cards"`
	BlockerRows    int     `json:"blocker_rows"`
	RawFacts       int     `json:"raw_facts"`
	RawPanels      int     `json:"raw_panels"`
	RouteGroups    int     `json:"route_groups"`
	MermaidSources int     `json:"mermaid_sources"`
	ProcessedMaps  int     `json:"processed_maps"`
	SVGs           int     `json:"svgs"`
}

func readReadinessScaleMetrics(t *testing.T, ctx context.Context) readinessScaleMetrics {
	t.Helper()
	var metrics readinessScaleMetrics
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var nav = performance.getEntriesByType('navigation')[0];
		var raws = Array.from(document.querySelectorAll('.claim-readiness-raw pre'));
		return {
			load_ms: nav ? nav.loadEventEnd - nav.startTime : -1,
			dom_nodes: document.querySelectorAll('*').length,
			js_heap_bytes: performance.memory ? performance.memory.usedJSHeapSize : -1,
			readiness_cards: document.querySelectorAll('.claim-readiness').length,
			blocker_rows: document.querySelectorAll('.claim-readiness-blocker').length,
			raw_facts: raws.reduce(function(total, pre){
				var raw = JSON.parse(pre.textContent);
				return total + (raw.dependency_conditions || []).length + (raw.review_causes || []).length;
			}, 0),
			raw_panels: raws.length,
			route_groups: document.querySelectorAll('.claim-readiness-route').length,
			mermaid_sources: document.querySelectorAll('.claim-readiness-map pre.mermaid').length,
			processed_maps: document.querySelectorAll('.claim-readiness-map pre.mermaid[data-processed]').length,
			svgs: document.querySelectorAll('.claim-readiness-map svg').length
		};
	})()`, &metrics))
	return metrics
}

func TestReadinessBrowserScaleBudgets(t *testing.T) {
	for _, tc := range []struct {
		name          string
		layers        int
		width         int
		maxRouteFacts int
	}{
		{name: "deep-chain-128", layers: 128, width: 1, maxRouteFacts: 127},
		{name: "layered-dense-24x5", layers: 24, width: 5, maxRouteFacts: 111},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := readinessScaleProject(t, tc.layers, tc.width)
			claimCount := tc.layers * tc.width
			factCount := readinessScaleFacts(tc.layers, tc.width)
			routeCount := (tc.layers - 1) * tc.width * tc.width

			ctx := browserContext(t)
			runCDP(t, ctx,
				chromedp.Navigate(p.renderStatic()+"#widget.contract.l000-n00"),
				chromedp.WaitVisible("#widget\\.contract\\.l000-n00 .claim-readiness", chromedp.ByQuery),
			)
			pollTrue(t, ctx, fmt.Sprintf(`
				document.readyState === 'complete' &&
				performance.getEntriesByType('navigation')[0].loadEventEnd > 0 &&
				document.querySelectorAll('.claim-readiness-blocker').length === %d`, factCount))

			before := readReadinessScaleMetrics(t, ctx)
			if before.LoadMS < 0 || before.LoadMS > readinessScaleMaxLoadMS {
				t.Fatalf("viewer load %.0fms exceeded %dms budget", before.LoadMS, readinessScaleMaxLoadMS)
			}
			if before.DOMNodes > readinessScaleMaxDOMNodes {
				t.Fatalf("mounted DOM has %d nodes, exceeded %d-node budget", before.DOMNodes, readinessScaleMaxDOMNodes)
			}
			if before.JSHeapBytes < 0 || before.JSHeapBytes > readinessScaleMaxJSHeapBytes {
				t.Fatalf("mounted JS heap %.0f bytes outside required 0..%d budget", before.JSHeapBytes, readinessScaleMaxJSHeapBytes)
			}
			if before.ReadinessCards != claimCount {
				t.Fatalf("readiness cards = %d, want every one of %d claims", before.ReadinessCards, claimCount)
			}
			if before.BlockerRows != factCount {
				t.Fatalf("authoritative blocker rows = %d, want all %d engine facts", before.BlockerRows, factCount)
			}
			if before.RawPanels != claimCount || before.RawFacts != factCount {
				t.Fatalf("raw diagnostics have %d panels and %d facts, want %d panels and all %d facts", before.RawPanels, before.RawFacts, claimCount, factCount)
			}
			if before.RouteGroups != routeCount || before.MermaidSources != routeCount {
				t.Fatalf("route groups / map sources = %d / %d, want %d / %d", before.RouteGroups, before.MermaidSources, routeCount, routeCount)
			}
			if before.ProcessedMaps != 0 || before.SVGs != 0 {
				t.Fatalf("closed maps processed %d sources and rendered %d SVGs, want lazy zero / zero", before.ProcessedMaps, before.SVGs)
			}

			var opened struct {
				Facts int `json:"facts"`
			}
			runCDP(t, ctx, chromedp.Evaluate(`(function(){
				var routes = Array.from(document.querySelectorAll('.claim-readiness-route'));
				var route = routes.reduce(function(best, item){
					return item.querySelectorAll('.claim-readiness-blocker').length > best.querySelectorAll('.claim-readiness-blocker').length ? item : best;
				});
				var facts = route.querySelectorAll('.claim-readiness-blocker').length;
				window.__readinessScaleMapStarted = performance.now();
				route.open = true;
				route.querySelector('.claim-readiness-trace').open = true;
				return { facts: facts };
			})()`, &opened))
			if opened.Facts != tc.maxRouteFacts {
				t.Fatalf("largest route group has %d facts, want %d", opened.Facts, tc.maxRouteFacts)
			}
			pollTrue(t, ctx, `document.querySelectorAll('.claim-readiness-map svg').length === 1`)

			var after struct {
				MapMS         float64 `json:"map_ms"`
				FactNodes     int     `json:"fact_nodes"`
				ProcessedMaps int     `json:"processed_maps"`
				SVGs          int     `json:"svgs"`
				Errors        int     `json:"errors"`
				CapText       string  `json:"cap_text"`
				JSHeapBytes   float64 `json:"js_heap_bytes"`
			}
			runCDP(t, ctx, chromedp.Evaluate(`(function(){
				var svg = document.querySelector('.claim-readiness-map svg');
				var map = svg.closest('.claim-readiness-map');
				var cap = map.querySelector('.claim-readiness-map-limit');
				return {
					map_ms: performance.now() - window.__readinessScaleMapStarted,
					fact_nodes: svg.querySelectorAll('.node.fact').length,
					processed_maps: document.querySelectorAll('.claim-readiness-map pre.mermaid[data-processed]').length,
					svgs: document.querySelectorAll('.claim-readiness-map svg').length,
					errors: (window.__boErrors || []).length,
					cap_text: cap ? cap.textContent : '',
					js_heap_bytes: performance.memory ? performance.memory.usedJSHeapSize : -1
				};
			})()`, &after))
			if after.MapMS > readinessScaleMaxMapMS {
				t.Fatalf("focused Mermaid map took %.0fms, exceeded %dms budget", after.MapMS, readinessScaleMaxMapMS)
			}
			if after.FactNodes != 12 {
				t.Fatalf("focused Mermaid map has %d fact nodes, want explicit cap of 12", after.FactNodes)
			}
			if after.ProcessedMaps != 1 || after.SVGs != 1 {
				t.Fatalf("opening one map processed %d sources and rendered %d SVGs, want one / one", after.ProcessedMaps, after.SVGs)
			}
			if after.Errors != 0 {
				t.Fatalf("Mermaid renderer recorded %d errors", after.Errors)
			}
			wantCap := fmt.Sprintf("Showing 12 of %d blocker facts", opened.Facts)
			if !strings.Contains(after.CapText, wantCap) {
				t.Fatalf("map cap disclosure = %q, want it to contain %q", after.CapText, wantCap)
			}
			if after.JSHeapBytes < 0 || after.JSHeapBytes > readinessScaleMaxJSHeapBytes {
				t.Fatalf("post-map JS heap %.0f bytes outside required 0..%d budget", after.JSHeapBytes, readinessScaleMaxJSHeapBytes)
			}

			t.Logf("claims=%d facts=%d routes=%d load=%.0fms domNodes=%d heapBefore=%.1fMiB map=%.0fms heapAfter=%.1fMiB",
				claimCount, factCount, routeCount, before.LoadMS, before.DOMNodes,
				before.JSHeapBytes/(1024*1024), after.MapMS, after.JSHeapBytes/(1024*1024))
		})
	}
}

const readinessRootYAML = `id: widget.contract.root
facet: contract
module: widget
status: draft
body: |
  the claim whose readiness a reviewer is deciding.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - widget.contract.alpha
  - widget.contract.beta
`

const readinessAlphaYAML = `id: widget.contract.alpha
facet: contract
module: widget
status: draft
body: |
  a direct prerequisite awaiting approval.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const readinessBetaYAML = `id: widget.contract.beta
facet: contract
module: widget
status: draft
body: |
  a direct prerequisite with an upstream prerequisite.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - widget.contract.gamma
`

const readinessGammaYAML = `id: widget.contract.gamma
facet: contract
module: widget
status: draft
body: |
  an upstream prerequisite awaiting approval.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

func newReadinessProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("root.yaml", readinessRootYAML)
	p.writeClaim("alpha.yaml", readinessAlphaYAML)
	p.writeClaim("beta.yaml", readinessBetaYAML)
	p.writeClaim("gamma.yaml", readinessGammaYAML)
	return p
}

func TestReadinessMapCapNeverCapsTheAuthoritativeList(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	var leafIDs []string
	for i := 0; i < 13; i++ {
		id := fmt.Sprintf("widget.contract.leaf-%02d", i)
		leafIDs = append(leafIDs, id)
		p.writeClaim(fmt.Sprintf("leaf-%02d.yaml", i), fmt.Sprintf(`id: %s
facet: contract
module: widget
status: draft
body: |
  an upstream prerequisite awaiting approval.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`, id))
	}
	p.writeClaim("hub.yaml", `id: widget.contract.hub
facet: contract
module: widget
status: draft
body: |
  the single first-hop route to many upstream blockers.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - `+strings.Join(leafIDs, "\n  - ")+"\n")
	p.writeClaim("root.yaml", `id: widget.contract.root
facet: contract
module: widget
status: draft
body: |
  a root with one grouped route.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - widget.contract.hub
`)

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness", chromedp.ByQuery),
	)
	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 14 {
		t.Fatalf("authoritative list has %d facts, want all 14", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-route').length`); got != 1 {
		t.Fatalf("first-hop groups = %d, want one hub route", got)
	}
	runCDP(t, ctx, chromedp.Evaluate(root+`.querySelector('.claim-readiness-trace').open = true`, nil))
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-map svg').length === 1`)
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-map .node.fact').length`); got != 12 {
		t.Fatalf("bounded map has %d fact nodes, want explicit cap of 12", got)
	}
	if !evalBool(t, ctx, root+`.querySelector('.claim-readiness-map-limit').textContent.includes('Showing 12 of 14')`) {
		t.Fatal("bounded map must disclose its 12-of-14 cap beside the complete list")
	}
}

func TestStaticReadinessGroupsCompleteFactsAndRendersFocusedMapOnDemand(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness", chromedp.ByQuery),
	)

	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if got := evalString(t, ctx, root+`.querySelector('.claim-readiness-state').textContent.trim()`); got != "Dependencies not ready" {
		t.Fatalf("readiness state = %q, want Dependencies not ready", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 3 {
		t.Fatalf("rendered blocker facts = %d, want all 3 independent conditions", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-route').length`); got != 2 {
		t.Fatalf("first-hop groups = %d, want alpha and beta", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-local li').length`); got != 1 {
		t.Fatalf("local approval reasons = %d, want the alias displayed once", got)
	}
	if !evalBool(t, ctx, root+`.textContent.includes('widget.contract.alpha') && `+root+`.textContent.includes('widget.contract.beta') && `+root+`.textContent.includes('widget.contract.gamma')`) {
		t.Fatal("grouped view must preserve the exact ids of direct and upstream blockers")
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-map svg').length`); got != 0 {
		t.Fatalf("closed map disclosures rendered %d SVGs, want lazy zero", got)
	}

	// The beta route is the group with two facts: beta itself and gamma via
	// beta. Open its native disclosure, reveal the representative path, and
	// then opt into its focused Mermaid trace.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var routes = Array.from(`+root+`.querySelectorAll('.claim-readiness-route'));
		var beta = routes.find(function(route){ return route.querySelector('.claim-readiness-route-id small').textContent === 'widget.contract.beta'; });
		beta.open = true;
		beta.querySelector('.claim-readiness-path').open = true;
		beta.querySelector('.claim-readiness-trace').open = true;
	})()`, nil))
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-map svg').length === 1`)

	if !evalBool(t, ctx, root+`.textContent.includes('widget.contract.root → widget.contract.beta → widget.contract.gamma')`) {
		t.Fatal("upstream fact must retain its complete representative path")
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-map .node.fact').length`); got != 2 {
		t.Fatalf("focused beta map fact nodes = %d, want 2", got)
	}
	if got := evalInt(t, ctx, `window.__boErrors.length`); got != 0 {
		t.Fatalf("Mermaid renderer recorded %d errors", got)
	}

	// Raw diagnostics are the lossless maintainer surface, not a replacement
	// for the reviewer-facing list.
	if !evalBool(t, ctx, root+`.querySelector('.claim-readiness-raw pre').textContent.includes('"kind": "dependency_unapproved"')`) {
		t.Fatal("raw diagnostics must preserve the exact engine condition kind")
	}
	if !evalBool(t, ctx, `document.body.scrollWidth <= window.innerWidth + 1`) {
		t.Fatal("the grouped readiness component must not create page-level horizontal overflow")
	}
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(1)))
	pollTrue(t, ctx, `window.innerWidth === 390`)
	if !evalBool(t, ctx, `document.documentElement.scrollWidth <= window.innerWidth + 1`) {
		t.Fatal("the open readiness route and map must keep horizontal scrolling inside the map at 390px")
	}
	if !evalBool(t, ctx, root+`.querySelector('.claim-readiness-map-scroll').scrollWidth > `+root+`.querySelector('.claim-readiness-map-scroll').clientWidth`) {
		t.Fatal("the narrow layout must retain an internally scrollable dependency map")
	}
}

func TestReadinessTreatsFlagDetailsAsText(t *testing.T) {
	p := newProject(t)
	p.run("claim", "lock", testClaimID, "--reason", "viewer-test fixture")
	const hostile = `<img src=x onerror="window.__readinessInjected=true">`
	p.run("claim", "flag", testClaimID,
		"--claim-says", "a claim under review.",
		"--now-does", "a corrected claim.",
		"--reason", hostile,
	)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#"+testClaimID),
		chromedp.WaitVisible(".claim-readiness", chromedp.ByQuery),
	)
	if evalBool(t, ctx, `window.__readinessInjected === true || !!document.querySelector('.claim-readiness img')`) {
		t.Fatal("authored readiness detail became markup instead of text")
	}
	if !evalBool(t, ctx, `document.querySelector('.claim-readiness').textContent.includes(`+fmt.Sprintf("%q", hostile)+`)`) {
		t.Fatal("escaped readiness detail was dropped instead of shown to the reviewer")
	}
}

func TestLiveReadinessRefreshesAfterAnUpstreamApproval(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	base := p.ensureServe()
	runCDP(t, ctx,
		chromedp.Navigate(base+"/#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.body.classList.contains('comments-sse-open')`)
	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 3 {
		t.Fatalf("initial live blocker facts = %d, want 3", got)
	}

	p.run("claim", "lock", "widget.contract.gamma", "--reason", "viewer-test fixture")
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length === 2`)
	if evalBool(t, ctx, root+`.textContent.includes('widget.contract.gamma')`) {
		t.Fatal("live readiness kept the approved upstream blocker after the served fragment refreshed")
	}
	if got := evalInt(t, ctx, `window.__boErrors.length`); got != 0 {
		t.Fatalf("live refresh left %d Mermaid renderer errors", got)
	}
}
