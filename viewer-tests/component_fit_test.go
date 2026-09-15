package viewertests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

func softMountFitProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, twoFacetConfigYAML)
	for i := 0; i < 80; i++ {
		facet := "contract"
		if i >= 40 {
			facet = "interface"
		}
		id := fmt.Sprintf("widget.%s.c%02d", facet, i)
		p.writeClaim(fmt.Sprintf("%s.yaml", id), fmt.Sprintf(`id: %s
facet: %s
module: widget
status: draft
body: |
  soft-mount lock-metric fixture %d.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`, id, facet, i))
	}
	return p
}

func TestSoftMountLockMetricUsesCatalogAttrs(t *testing.T) {
	p := softMountFitProject(t)
	url := p.renderStatic()
	raw, err := os.ReadFile(filepath.Join(p.dir, "build", "viewer", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(raw)
	if !strings.Contains(html, `data-claim-count="80"`) || !strings.Contains(html, `class="dossierx-surface-template"`) {
		t.Fatal("soft-mount render must stamp catalog counts and defer cards into templates")
	}
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `!!(document.querySelector('.module-section#widget') && document.querySelector('.system-record-head__metric'))`)
	if !evalBool(t, ctx, `document.querySelector('#widget').getAttribute('data-claim-count') === '80' && document.querySelector('#widget').getAttribute('data-locked-count') === '0' && document.querySelector('#widget').getAttribute('data-facet-count') === '2'`) {
		t.Fatal("soft-mounted module must stamp catalog lock counts on the section")
	}
	if !evalBool(t, ctx, `document.querySelector('.system-record-head__metric').textContent.includes('0 of 80') && document.querySelector('.system-record-head__summary').textContent.includes('80 claims across 2 record sections')`) {
		t.Fatal("header metric must read catalog attrs, not live cards")
	}
	unmounted := evalBool(t, ctx, `document.querySelectorAll('[data-dossierx-surface-host] .claim').length === 0`)
	pollTrue(t, ctx, `document.querySelectorAll('[data-dossierx-surface-host] .claim').length > 0`)
	if unmounted && !evalBool(t, ctx, `document.querySelector('.system-record-head__metric').textContent.includes('0 of 80')`) {
		t.Fatal("header metric must stay catalog-backed after the active surface mounts")
	}
	if !evalBool(t, ctx, `document.querySelector('.system-record-head__metric').textContent.includes('0 of 80')`) {
		t.Fatal("header metric must stay 0 of 80 after mount")
	}
}

func TestStatusStripGroupsBlockersAndStaysCollapsed(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)
	if !evalBool(t, ctx, `document.getElementById('statusStripTitle').textContent.indexOf('blocked by unapproved dependencies') >= 0`) {
		t.Fatal("blocker-only strip must summarize unique unapproved dependencies, not dump one row per path")
	}
	if got := evalInt(t, ctx, `document.querySelectorAll('#statusStripBody .status-finding--group').length`); got < 1 || got > 6 {
		t.Fatalf("grouped strip rows = %d, want a small unique-blocker set", got)
	}
	if evalBool(t, ctx, `document.getElementById('statusStrip').classList.contains('status-strip--open')`) {
		t.Fatal("blocker-only strip must stay collapsed")
	}
}

func TestReadyConformanceStaysInsideCollapsedClaim(t *testing.T) {
	p := newProjectRaw(t, conformanceConfigYAML)
	p.writeClaim("overview.yaml", conformanceClaimYAML)
	writeConformanceObservation(t, p, `["blocked","ready"]`)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(p.renderStatic()))
	pollTrue(t, ctx, `!!(document.querySelector('.claim-conformance') && document.querySelector('.claim .claim-conformance'))`)
	if evalBool(t, ctx, `document.querySelector('.claim-conformance').open`) {
		t.Fatal("ready implementation checks must render closed")
	}
	runCDP(t, ctx, chromedp.Click(".claim-collapse-toggle", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.claim').classList.contains('claim--collapsed')`)
	if evalBool(t, ctx, `(function(){
		var panel = document.querySelector('.claim-conformance');
		if (!panel) { return true; }
		var style = getComputedStyle(panel);
		return style.display !== 'none' && style.visibility !== 'hidden' && panel.getClientRects().length > 0;
	})()`) {
		t.Fatal("ready conformance must not stay visible on a collapsed claim")
	}
}

func TestThemeControlDarkOverridesLightOS(t *testing.T) {
	p := newProject(t)
	url := p.renderStatic()
	raw, err := os.ReadFile(filepath.Join(p.dir, "build", "viewer", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `data-theme-choice="dark"`) {
		t.Fatal("rendered viewer is missing the Dark theme control")
	}
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	if !evalBool(t, ctx, `!!document.querySelector('.theme-control')`) {
		t.Fatalf("loaded page has no .theme-control; title=%q body=%d", evalString(t, ctx, `document.title`), evalInt(t, ctx, `document.body ? document.body.innerHTML.length : -1`))
	}
	runCDP(t, ctx, emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
		{Name: "prefers-color-scheme", Value: "light"},
	}))
	if !evalBool(t, ctx, `window.matchMedia('(prefers-color-scheme: light)').matches`) {
		t.Fatal("OS must stay light so Dark is a real override")
	}
	lightPaper := evalString(t, ctx, `getComputedStyle(document.documentElement).getPropertyValue('--paper').trim()`)
	evalVoid(t, ctx, `(function(){
		var button = document.querySelector('.theme-control [data-theme-choice="dark"]');
		if (!button) { throw new Error('dark theme control is missing'); }
		button.click();
	})()`)
	if !evalBool(t, ctx, `document.documentElement.getAttribute('data-theme') === 'dark'`) {
		t.Fatalf("Dark control did not set data-theme=dark (got %q, bound=%v, buttons=%d)",
			evalString(t, ctx, `document.documentElement.getAttribute('data-theme') || ''`),
			evalBool(t, ctx, `document.documentElement.dataset.themeControlBound === 'true'`),
			evalInt(t, ctx, `document.querySelectorAll('.theme-control [data-theme-choice]').length`))
	}
	darkPaper := evalString(t, ctx, `getComputedStyle(document.documentElement).getPropertyValue('--paper').trim()`)
	if darkPaper == "" || darkPaper == lightPaper {
		t.Fatalf("Dark toggle left --paper at %q under a light OS (was %q)", darkPaper, lightPaper)
	}
	if darkPaper != "#0a1220" {
		t.Fatalf("--paper after Dark = %q, want the engine dark token #0a1220", darkPaper)
	}
}

func TestPhone390SoftMountSmoke(t *testing.T) {
	p := softMountFitProject(t)
	blocker := newReadinessProject(t)
	ready := newProjectRaw(t, conformanceConfigYAML)
	ready.writeClaim("overview.yaml", conformanceClaimYAML)
	writeConformanceObservation(t, ready, `["blocked","ready"]`)

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(1)),
		chromedp.Navigate(p.renderStatic()),
	)
	pollTrue(t, ctx, `window.innerWidth === 390`)
	if !evalBool(t, ctx, `getComputedStyle(document.getElementById('sidebar')).transform !== 'none'`) {
		t.Fatal("390px sidebar must sit off-canvas")
	}
	if !evalBool(t, ctx, `document.querySelector('.system-record-head__metric').textContent.includes('0 of 80')`) {
		t.Fatal("390px soft-mount header metric must be non-zero")
	}

	runCDP(t, ctx, chromedp.Navigate(blocker.renderStatic()+"#widget.contract.root"))
	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)
	if evalBool(t, ctx, `document.getElementById('statusStrip').classList.contains('status-strip--open')`) {
		t.Fatal("390px blocker-only strip must stay collapsed")
	}

	runCDP(t, ctx, chromedp.Navigate(ready.renderStatic()))
	pollTrue(t, ctx, `!!document.querySelector('.claim-conformance')`)
	runCDP(t, ctx, chromedp.Click(".claim-collapse-toggle", chromedp.ByQuery))
	pollTrue(t, ctx, `!!document.querySelector('.claim.claim--collapsed')`)
	if evalBool(t, ctx, `(function(){
		var panel = document.querySelector('.claim-conformance');
		return !!(panel && panel.getClientRects().length);
	})()`) {
		t.Fatal("390px collapsed ready claim must hide implementation checks")
	}
}
