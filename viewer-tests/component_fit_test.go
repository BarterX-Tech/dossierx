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
	// Re-pinned: 02 §4.7 row 1 / 02 §6 "Module eyebrow (64-0)" replaces the
	// removed .system-record-head__summary ("N claims across N record
	// sections") — a mono "MODULE NN / NN" eyebrow, not a count sentence.
	// This project has exactly one module, so the eyebrow reads
	// "MODULE 01 / 01".
	if !evalBool(t, ctx, `document.querySelector('.system-record-head__metric').textContent.includes('0 of 80') && document.querySelector('.system-record-head__eyebrow').textContent.trim() === 'MODULE 01 / 01'`) {
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
	// lane L8 (docs/design/screens/04-issues-screen.md §2, §9 item 6): the
	// promoted Issues body now groups these rows by owning MODULE rather than
	// by severity, so they may land under more than one <ul>. This assertion
	// still holds because it counts the .status-finding--group <li> rows
	// themselves, not the .status-group headings around them — the total
	// number of unique blockers this fixture produces is unchanged by which
	// axis groups them.
	if got := evalInt(t, ctx, `document.querySelectorAll('#statusStripBody .status-finding--group').length`); got < 1 || got > 6 {
		t.Fatalf("grouped strip rows = %d, want a small unique-blocker set", got)
	}
	if evalBool(t, ctx, `document.getElementById('statusStrip').classList.contains('status-strip--open')`) {
		t.Fatal("blocker-only strip must stay collapsed")
	}
	// lane L8 (this lane's third attempt; docs/design/screens/
	// 04-issues-screen.md §8 item 11's coordinator ruling): #statusStripNote
	// (the "Critical 0 · Needs you 3 · ..." tally) is removed from the
	// collapsed banner — no 02/03/04 board draws it, and the severity chips
	// already carry every count it used to repeat. This assertion now reads
	// the Blocker chip's own count span instead of that removed string.
	if !evalBool(t, ctx, `(function(){
		var claims = document.querySelectorAll('.module-section:not([hidden]) .claim-group:not([hidden]) .claim').length;
		var title = document.getElementById('statusStripTitle').textContent;
		var chipCount = document.querySelector('.status-severity-chip--blocker .status-severity-chip__count');
		var blocked = title.match(/(\d+) claims? blocked/);
		return blocked && chipCount && Number(blocked[1]) <= claims && Number(chipCount.textContent) <= claims;
	})()`) {
		t.Fatalf("blocker counts must be unique claims, not summed paths (title=%q chip=%q)",
			evalString(t, ctx, `document.getElementById('statusStripTitle').textContent`),
			evalString(t, ctx, `(document.querySelector('.status-severity-chip--blocker .status-severity-chip__count') || {}).textContent`))
	}
}

func TestGroup02MobileNavigationAndFacetSheet(t *testing.T) {
	p := softMountFitProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(390, 844),
		chromedp.Navigate(p.renderStatic()),
	)
	pollTrue(t, ctx, `!!(document.querySelector('.mobile-app-bar') && document.querySelector('.facet-toc-trigger'))`)
	runCDP(t, ctx, chromedp.Evaluate(`window.dossierxPositionStatusStrip()`, nil))
	if !evalBool(t, ctx, `(function () {
		var section = document.querySelector('.module-section:not([hidden])');
		var header = section && section.querySelector(':scope > .system-record-head');
		var tabs = section && section.querySelector(':scope > .sub-nav');
		var strip = document.getElementById('statusStrip');
		var firstClaims = section && section.querySelector(':scope > .claim-group:not([hidden])');
		return header && tabs && strip && firstClaims && tabs.nextElementSibling === strip &&
		  strip.nextElementSibling === firstClaims;
	})()`) {
		t.Fatal("reading order must be module heading, facet tabs, status, then claims")
	}
	if !evalBool(t, ctx, `(function () {
		var menu = document.getElementById('navToggle');
		var search = document.getElementById('mobileSearchToggle');
		var light = document.querySelector('.mobile-theme-control [data-theme-choice="light"]');
		var dark = document.querySelector('.mobile-theme-control [data-theme-choice="dark"]');
		return menu && search && light && dark && menu.getAttribute('aria-label') === 'Open modules' &&
		  !/Sections/.test(menu.textContent) && getComputedStyle(menu).minHeight === '44px';
	})()`) {
		t.Fatal("mobile app bar must expose the compact menu, search, and two-way theme controls")
	}
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('mobileSearchToggle').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('nav-open') && document.activeElement === document.getElementById('navSearch')`)
	if !evalBool(t, ctx, `getComputedStyle(document.querySelector('.sidebar')).width === '328px' && !!document.getElementById('navDrawerClose')`) {
		t.Fatal("mobile modules navigation must be a 328px drawer with a visible close control")
	}
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('navDrawerClose').click()`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('nav-open')`)

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc-trigger').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open') && !!document.querySelector('.facet-toc__grabber') && !!document.querySelector('.facet-toc__close')`)
	if !evalBool(t, ctx, `(function () {
		var sheet = document.getElementById('systemFacetToc');
		var close = sheet.querySelector('.facet-toc__close');
		return sheet && close && close.getAttribute('aria-label') === 'Close facet panel' &&
		  parseFloat(getComputedStyle(sheet).height) <= 660 && getComputedStyle(close).minHeight === '44px';
	})()`) {
		t.Fatal("facet index must use the accessible, bounded bottom-sheet shell")
	}
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc__close').click()`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('facet-toc-open')`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc-trigger').click(); document.dispatchEvent(new KeyboardEvent('keydown', {key:'Escape'}))`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('facet-toc-open')`)
}

func TestEmphasisDoesNotTurnLockedCardIntoWarning(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("locked.yaml", `id: widget.contract.locked
facet: contract
module: widget
status: draft
emphasis: true
body: |
  An approved claim can be important without being a warning.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	p.run("claim", "lock", "widget.contract.locked", "--reason", "viewer test lock")
	p.writeClaim("warning.yaml", `id: widget.contract.warning
facet: contract
module: widget
status: draft
layout: banner
body: |
  This is an explicit warning callout.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(p.renderStatic()))
	pollTrue(t, ctx, `!!document.getElementById('widget.contract.locked')`)
	if !evalBool(t, ctx, `(function () {
		var card = document.getElementById('widget.contract.locked');
		var banner = document.getElementById('widget.contract.warning');
		return card && banner && !card.classList.contains('claim-card--warn') &&
		  banner.classList.contains('claim-banner') &&
		  getComputedStyle(banner).borderTopColor !== getComputedStyle(card).borderTopColor;
	})()`) {
		t.Fatal("ordinary emphasis must not paint a locked card as warning-red, while explicit banners stay semantic warnings")
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
	// The literal is the engine's dark --paper in style.css's two dark blocks.
	// It moved from #0a1220 to #0D1117 when the design revamp re-pointed the
	// default palette onto the design's tokens; the mechanism this test is
	// about — an explicit Dark choice beating a light OS — is unchanged, only
	// the value it lands on. Pinned rather than read back from the sheet on
	// purpose: comparing the page against itself would pass for a Dark control
	// that did nothing at all.
	if darkPaper != "#0D1117" {
		t.Fatalf("--paper after Dark = %q, want the engine dark token #0D1117", darkPaper)
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
