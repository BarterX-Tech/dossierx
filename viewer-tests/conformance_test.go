package viewertests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const conformanceConfigYAML = defaultConfigYAML + `conformance:
  observations: observations.json
`

const conformanceClaimYAML = draftClaimYAML + `embodiment:
  mode: compare
  checks:
    - id: public-values
      adapter: neutral/v1
      target: widget://state
      expectation:
        shape: set
        value: [blocked, ready]
    - id: schema-version
      adapter: neutral/v1
      target: widget://version
      expectation:
        shape: scalar
        value: "3"
`

func writeConformanceObservation(t *testing.T, p *project, values string) {
	t.Helper()
	body := `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":` + values + `},{"adapter":"neutral/v1","target":"widget://version","shape":"scalar","value":"3"}]}`
	if err := os.WriteFile(filepath.Join(p.dir, "observations.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConformanceStatusFreshnessGuardRejectsInvertedCompletion(t *testing.T) {
	opted := newProjectRaw(t, conformanceConfigYAML)
	opted.writeClaim("overview.yaml", conformanceClaimYAML)
	writeConformanceObservation(t, opted, `["blocked","ready"]`)
	_ = opted.renderStatic()
	raw, err := os.ReadFile(filepath.Join(opted.dir, "build", "viewer", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	marker := strings.Index(html, "/* dossierx-conformance-status-freshness */")
	if marker < 0 {
		t.Fatal("opted-in viewer omitted status freshness guard")
	}
	runtimeStart := strings.Index(html, "      'use strict';")
	if runtimeStart < 0 || marker >= runtimeStart {
		t.Fatalf("guard/runtime order marker=%d runtime=%d", marker, runtimeStart)
	}
	scriptStart := strings.LastIndex(html[:marker], "<script>")
	scriptEndRel := strings.Index(html[marker:], "</script>")
	if scriptStart < 0 || scriptEndRel < 0 {
		t.Fatal("cannot isolate rendered freshness guard")
	}
	guard := html[scriptStart+len("<script>") : marker+scriptEndRel]

	plain := newProjectRaw(t, defaultConfigYAML)
	plain.writeClaim("overview.yaml", draftClaimYAML)
	_ = plain.renderStatic()
	plainRaw, err := os.ReadFile(filepath.Join(plain.dir, "build", "viewer", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plainRaw), "dossierx-conformance-status-freshness") {
		t.Fatal("no-conformance viewer changed by freshness guard")
	}

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate("about:blank"),
		chromedp.Evaluate(`window.__statusPending = []; window.fetch = function () { return new Promise(function (resolve) { window.__statusPending.push(resolve); }); };`, nil),
		chromedp.Evaluate(guard, nil),
	)
	var verdict string
	expr := `new Promise(function (done) {
  var lastPaint = '';
  var firstError = '';
  function response(state) { return {ok:true, json:function(){ return Promise.resolve({state:state}); }}; }
  var first = fetch('/api/status').then(function(r){ return r.json(); }).then(function(v){ lastPaint=v.state; }).catch(function(e){ firstError=e.name; });
  var second = fetch('/api/status').then(function(r){ return r.json(); }).then(function(v){ lastPaint=v.state; });
  window.__statusPending[1](response('new'));
  setTimeout(function () {
    window.__statusPending[0](response('old'));
    Promise.all([first, second]).then(function () { done(JSON.stringify({last:lastPaint, first_error:firstError})); });
  }, 0);
})`
	runCDP(t, ctx, chromedp.Evaluate(expr, &verdict, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	if verdict != `{"last":"new","first_error":"AbortError"}` {
		t.Fatalf("inverted completion verdict=%s", verdict)
	}
}

func TestConformancePanelVisibleStaticAndRefreshesWhenServed(t *testing.T) {
	staticProject := newProjectRaw(t, conformanceConfigYAML)
	staticProject.writeClaim("overview.yaml", conformanceClaimYAML)
	writeConformanceObservation(t, staticProject, `["ready","paused"]`)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(staticProject.renderStatic()),
		chromedp.WaitVisible(`.claim-conformance-check[data-check-id="public-values"][data-conformance-state="mismatch"]`, chromedp.ByQuery),
	)
	if !evalBool(t, ctx, `document.querySelector('.claim-conformance').textContent.includes('blocked') && document.querySelector('.claim-conformance').textContent.includes('paused') && document.querySelector('.claim-conformance').textContent.includes('schema-version') && document.querySelector('.claim-conformance-check[data-check-id="schema-version"]').getAttribute('data-shape') === 'scalar'`) {
		t.Fatal("static conformance panel does not show exact missing and extra members")
	}
	if !evalBool(t, ctx, `(function () {
  var scalar = document.querySelector('.claim-conformance-check[data-check-id="schema-version"]');
  if (!scalar) { return false; }
  var lines = Array.from(scalar.querySelectorAll('.claim-conformance-line')).map(function (line) { return line.textContent.trim(); });
  return lines.includes('expected: 3') && lines.includes('observed: 3') && !lines.some(function (line) { return line.startsWith('missing:') || line.startsWith('extra:'); });
})()`) {
		t.Fatal("static scalar check does not show exact expected/observed values or leaked set-only differences")
	}
	if !evalBool(t, ctx, `document.querySelector('.claim-conformance').getAttribute('data-implementation-ready') === 'false'`) {
		t.Fatal("static mismatch does not expose readiness=false")
	}

	liveProject := newProjectRaw(t, conformanceConfigYAML)
	liveProject.writeClaim("overview.yaml", conformanceClaimYAML)
	liveCtx := browserContext(t)
	runCDP(t, liveCtx,
		chromedp.Navigate(liveProject.ensureServe()+"/"),
		chromedp.WaitVisible(`.claim-conformance[data-implementation-ready="false"] .claim-conformance-check[data-conformance-state="uncheckable"]`, chromedp.ByQuery),
	)
	// Missing -> created.
	writeConformanceObservation(t, liveProject, `["blocked","ready"]`)
	pollTrue(t, liveCtx, `document.querySelector('.claim-conformance') && document.querySelector('.claim-conformance').getAttribute('data-implementation-ready') === 'true' && document.querySelectorAll('.claim-conformance-check[data-conformance-state="matched"]').length === 2`)

	// Malformed -> recovery.
	if err := os.WriteFile(filepath.Join(liveProject.dir, "observations.json"), []byte(`{"format_version":1`), 0o644); err != nil {
		t.Fatal(err)
	}
	pollTrue(t, liveCtx, `document.querySelector('.claim-conformance') && document.querySelector('.claim-conformance').getAttribute('data-implementation-ready') === 'false' && document.querySelectorAll('.claim-conformance-check[data-conformance-state="uncheckable"]').length === 2`)
	writeConformanceObservation(t, liveProject, `["blocked","ready"]`)
	pollTrue(t, liveCtx, `document.querySelector('.claim-conformance') && document.querySelector('.claim-conformance').getAttribute('data-implementation-ready') === 'true' && document.querySelectorAll('.claim-conformance-check[data-conformance-state="matched"]').length === 2`)

	// Deletion -> recreated mismatch.
	if err := os.Remove(filepath.Join(liveProject.dir, "observations.json")); err != nil {
		t.Fatal(err)
	}
	pollTrue(t, liveCtx, `document.querySelector('.claim-conformance') && document.querySelector('.claim-conformance').getAttribute('data-implementation-ready') === 'false' && document.querySelectorAll('.claim-conformance-check[data-conformance-state="uncheckable"]').length === 2`)
	writeConformanceObservation(t, liveProject, `["ready","paused"]`)
	pollTrue(t, liveCtx, `document.querySelector('.claim-conformance') && document.querySelector('.claim-conformance').getAttribute('data-implementation-ready') === 'false' && document.querySelector('.claim-conformance-check[data-check-id="public-values"]').getAttribute('data-conformance-state') === 'mismatch'`)
}
