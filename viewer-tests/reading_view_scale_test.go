package viewertests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// Reading-view scale budgets for deferred surface mounting. Curtainly-shaped
// synthetic corpus (many modules × facets × cards + tracks) stays in-repo;
// Curtainly YAML never does.
const (
	readingScaleModules        = 8
	readingScaleFacets         = 3
	readingScaleClaimsPerFacet = 12
	readingScaleTracks         = 2
	readingScaleClaimsPerTrack = 6

	readingScaleMaxLoadMS        = 15_000
	readingScaleMaxEnhanceMS     = 8_000
	readingScaleMaxFacetSwitchMS = 2_500
	readingScaleMaxCollapseAllMS = 2_500
	readingScaleMaxScrollLongMS  = 200
	readingScaleMaxDOMNodes      = 80_000
	readingScaleMaxJSHeapBytes   = 512 * 1024 * 1024
)

func readingScaleConfigYAML() string {
	var b strings.Builder
	b.WriteString("schema_version: 1\nfacets:\n")
	for i := 0; i < readingScaleFacets; i++ {
		fmt.Fprintf(&b, "  - facet%02d\n", i)
	}
	b.WriteString("modules:\n")
	for i := 0; i < readingScaleModules; i++ {
		fmt.Fprintf(&b, "  - mod%02d\n", i)
	}
	b.WriteString("claims_dir: claims\ntracks:\n")
	for i := 0; i < readingScaleTracks; i++ {
		fmt.Fprintf(&b, "  - {id: track%02d, title: Track %02d}\n", i, i)
	}
	return b.String()
}

func readingScaleProject(t *testing.T) (*project, int) {
	t.Helper()
	p := newProjectRaw(t, readingScaleConfigYAML())
	total := 0
	for mi := 0; mi < readingScaleModules; mi++ {
		for fi := 0; fi < readingScaleFacets; fi++ {
			for ci := 0; ci < readingScaleClaimsPerFacet; ci++ {
				id := fmt.Sprintf("mod%02d.facet%02d.c%02d", mi, fi, ci)
				trackBlock := ""
				if mi < readingScaleTracks && fi == 0 && ci < readingScaleClaimsPerTrack {
					trackBlock = fmt.Sprintf("tracks:\n  - id: track%02d\n    role: owns\n", mi)
				}
				p.writeClaim(id+".yaml", fmt.Sprintf(`id: %s
facet: facet%02d
module: mod%02d
status: draft
body: |
  scale fixture claim %s — body text for layout cost.
governed_by:
  type: none
  reason: viewer-test scale fixture, not backed by any doctrine claim
%s`, id, fi, mi, id, trackBlock))
				total++
			}
		}
	}
	return p, total
}

type readingScaleMetrics struct {
	LiveClaimCount      int     `json:"live_claim_count"`
	TemplateClaimCount  int     `json:"template_claim_count"`
	MountedHostClaims   int     `json:"mounted_host_claims"`
	DOMNodes            int     `json:"dom_nodes"`
	JSHeapBytes         float64 `json:"js_heap_bytes"`
	LoadMS              float64 `json:"load_ms"`
	EnhanceMS           float64 `json:"enhance_ms"`
	FacetSwitchMS       float64 `json:"facet_switch_ms"`
	CollapseAllMS       float64 `json:"collapse_all_ms"`
	ScrollLongtaskMaxMS float64 `json:"scroll_longtask_max_ms"`
	ActiveSurfaceID     string  `json:"active_surface_id"`
}

func collectReadingScaleMetrics(t *testing.T, ctx context.Context) readingScaleMetrics {
	t.Helper()
	var metrics readingScaleMetrics
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var nav = performance.getEntriesByType('navigation')[0];
		var activeGroup = document.querySelector('.claim-group:not([hidden])');
		var host = activeGroup && activeGroup.querySelector('[data-dossierx-surface-host]');
		var templateClaims = 0;
		document.querySelectorAll('template.dossierx-surface-template').forEach(function(tmpl){
			templateClaims += tmpl.content.querySelectorAll('.claim').length;
		});
		return {
			live_claim_count: document.querySelectorAll('.claim').length,
			template_claim_count: templateClaims,
			mounted_host_claims: host ? host.querySelectorAll('.claim').length : 0,
			dom_nodes: document.querySelectorAll('*').length,
			js_heap_bytes: performance.memory ? performance.memory.usedJSHeapSize : -1,
			load_ms: nav ? (nav.loadEventEnd - nav.startTime) : -1,
			enhance_ms: window.__dossierxEnhanceMS || -1,
			facet_switch_ms: window.__dossierxFacetSwitchMS || -1,
			collapse_all_ms: window.__dossierxCollapseAllMS || -1,
			scroll_longtask_max_ms: window.__dossierxScrollLongtaskMaxMS || 0,
			active_surface_id: activeGroup ? activeGroup.id : ''
		};
	})()`, &metrics))
	return metrics
}

func writeReadingScaleMetricsJSON(t *testing.T, name string, metrics readingScaleMetrics) {
	t.Helper()
	dir := os.Getenv("DOSSIERX_PERF_METRICS_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir metrics dir: %v", err)
	}
	payload, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	t.Logf("reading-view scale metrics written to %s\n%s", path, payload)
}

func TestReadingViewBrowserScaleBudgets(t *testing.T) {
	p, totalClaims := readingScaleProject(t)
	activeSurfaceClaims := readingScaleClaimsPerFacet

	ctx := browserContext(t)
	url := p.renderStatic()

	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("[data-dossierx-surface-host]", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.readyState === 'complete' && document.querySelectorAll('[data-dossierx-surface-host] .claim').length > 0`)

	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var start = performance.now();
		if (typeof window.dossierxEnhanceSystemRecord === 'function') {
			window.dossierxEnhanceSystemRecord();
		}
		window.__dossierxEnhanceMS = performance.now() - start;
		return true;
	})()`, nil))

	initial := collectReadingScaleMetrics(t, ctx)
	if initial.LiveClaimCount != initial.MountedHostClaims {
		t.Fatalf("after first paint, live claims (%d) != mounted host (%d) — inactive surfaces must stay in <template>", initial.LiveClaimCount, initial.MountedHostClaims)
	}
	if initial.LiveClaimCount > activeSurfaceClaims*2 {
		t.Fatalf("after first paint, live .claim count = %d, want ~active surface (%d)", initial.LiveClaimCount, activeSurfaceClaims)
	}

	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var start = performance.now();
		var tab = document.querySelector('.module-section:not([hidden]) .subtab:not(.on)');
		if (tab) { tab.click(); }
		return new Promise(function(resolve){
			requestAnimationFrame(function(){
				requestAnimationFrame(function(){
					window.__dossierxFacetSwitchMS = performance.now() - start;
					resolve(true);
				});
			});
		});
	})()`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	pollTrue(t, ctx, `document.querySelectorAll('[data-dossierx-surface-host] .claim').length > 0`)

	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var btn = document.querySelector('.facet-claims-toggle');
		var start = performance.now();
		if (btn) { btn.click(); }
		window.__dossierxCollapseAllMS = performance.now() - start;
		return true;
	})()`, nil))

	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		window.__dossierxScrollLongtaskMaxMS = 0;
		var max = 0;
		var observer = null;
		try {
			observer = new PerformanceObserver(function(list){
				list.getEntries().forEach(function(e){
					if (e.duration > max) { max = e.duration; }
				});
			});
			observer.observe({type:'longtask', buffered:true});
		} catch (e) {}
		var start = performance.now();
		return new Promise(function(resolve){
			var y = 0;
			function step(){
				y += 400;
				window.scrollTo(0, y);
				if (performance.now() - start < 600) {
					requestAnimationFrame(step);
					return;
				}
				if (observer) { observer.disconnect(); }
				window.__dossierxScrollLongtaskMaxMS = max;
				resolve(true);
			}
			requestAnimationFrame(step);
		});
	})()`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))

	metrics := collectReadingScaleMetrics(t, ctx)
	writeReadingScaleMetricsJSON(t, "reading-view-scale", metrics)

	if metrics.TemplateClaimCount < totalClaims {
		t.Fatalf("template claim store has %d claims, want at least corpus size %d", metrics.TemplateClaimCount, totalClaims)
	}
	if metrics.MountedHostClaims == 0 {
		t.Fatal("active surface host mounted zero claims")
	}
	// Surfaces mount on first visit and stay mounted, so after a facet switch
	// live claims can exceed the active host. The invariant is that we never
	// materialize the whole corpus: live << template store.
	if metrics.LiveClaimCount >= metrics.TemplateClaimCount {
		t.Fatalf("live claims (%d) reached the full template store (%d)", metrics.LiveClaimCount, metrics.TemplateClaimCount)
	}
	if metrics.LiveClaimCount > activeSurfaceClaims*4 {
		t.Fatalf("live .claim count = %d after visiting a couple of surfaces; want to stay near active-surface scale (%d)", metrics.LiveClaimCount, activeSurfaceClaims)
	}
	if metrics.MountedHostClaims > activeSurfaceClaims*2 {
		t.Fatalf("active host claims = %d, want about one surface (%d)", metrics.MountedHostClaims, activeSurfaceClaims)
	}
	if metrics.LoadMS < 0 || metrics.LoadMS > readingScaleMaxLoadMS {
		t.Fatalf("load %.0fms exceeded %dms budget", metrics.LoadMS, readingScaleMaxLoadMS)
	}
	if metrics.EnhanceMS < 0 || metrics.EnhanceMS > readingScaleMaxEnhanceMS {
		t.Fatalf("enhance %.0fms exceeded %dms budget", metrics.EnhanceMS, readingScaleMaxEnhanceMS)
	}
	if metrics.FacetSwitchMS < 0 || metrics.FacetSwitchMS > readingScaleMaxFacetSwitchMS {
		t.Fatalf("facet switch %.0fms exceeded %dms budget", metrics.FacetSwitchMS, readingScaleMaxFacetSwitchMS)
	}
	if metrics.CollapseAllMS < 0 || metrics.CollapseAllMS > readingScaleMaxCollapseAllMS {
		t.Fatalf("collapse-all %.0fms exceeded %dms budget", metrics.CollapseAllMS, readingScaleMaxCollapseAllMS)
	}
	if metrics.ScrollLongtaskMaxMS > readingScaleMaxScrollLongMS {
		t.Fatalf("scroll longtask max %.0fms exceeded %dms budget", metrics.ScrollLongtaskMaxMS, readingScaleMaxScrollLongMS)
	}
	if metrics.DOMNodes > readingScaleMaxDOMNodes {
		t.Fatalf("DOM nodes %d exceeded %d budget", metrics.DOMNodes, readingScaleMaxDOMNodes)
	}
	if metrics.JSHeapBytes < 0 || metrics.JSHeapBytes > readingScaleMaxJSHeapBytes {
		t.Fatalf("JS heap %.0f bytes outside 0..%d budget", metrics.JSHeapBytes, readingScaleMaxJSHeapBytes)
	}

	deepID := "mod02.facet01.c03"
	runCDP(t, ctx, chromedp.Navigate(url+"#"+deepID))
	pollTrue(t, ctx, fmt.Sprintf(`!!document.getElementById(%q)`, deepID))
	if !evalBool(t, ctx, fmt.Sprintf(`document.getElementById(%q).closest('[data-dossierx-surface-host]') !== null`, deepID)) {
		t.Fatalf("deep-linked claim %s was not mounted into a surface host", deepID)
	}
	liveAfter := evalInt(t, ctx, `document.querySelectorAll('.claim').length`)
	if liveAfter >= totalClaims {
		t.Fatalf("after deep-link, live claims (%d) reached the full corpus (%d)", liveAfter, totalClaims)
	}
}
