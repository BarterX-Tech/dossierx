package viewertests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
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

const group02NavigationConfigYAML = `schema_version: 1
title: Viewer fit fixture
eyebrow: Privacy-first desktop protection
facets:
  - contract
  - internals
modules:
  - widget
  - gadget
tracks:
  - id: review
    title: Review track
claims_dir: claims
`

func group02NavigationProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, group02NavigationConfigYAML)
	p.writeClaim("widget-contract.yaml", `id: widget.contract.orientation
facet: contract
module: widget
status: draft
build_role: orientation
tracks:
  - id: review
    role: owns
body: |
  the widget orientation claim.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	p.writeClaim("widget-internals.yaml", `id: widget.internals.detail
facet: internals
module: widget
status: draft
build_role: verification
body: |
  the widget implementation detail.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	p.writeClaim("gadget-contract.yaml", `id: gadget.contract.orientation
facet: contract
module: gadget
status: draft
body: |
  the gadget orientation claim.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	p.run("claim", "lock", "widget.contract.orientation", "--reason", "viewer-test fixture")
	p.run("claim", "lock", "widget.internals.detail", "--reason", "viewer-test fixture")
	p.run("build-order", "propose", "--module", "widget")
	p.run("build-order", "lock", "--module", "widget", "--reason", "viewer-test fixture")
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
		// The readiness DOOR, not its panel: the panel is the door's next
		// sibling and is only visible while the door is open, and the door no
		// longer auto-opens while a facet banner is showing (screens/02
		// section 9.1). This test is about the status strip, so it only needs
		// the card rendered.
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness-door", chromedp.ByQuery),
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
	// "Stays collapsed" means the findings body is never shown in place — it
	// is MOVED into the Issues screen. Reading that directly replaces a class
	// check that no longer corresponds to anything and so could not fail.
	if evalBool(t, ctx, `!document.getElementById('statusStripBody').hidden`) {
		t.Fatal("blocker-only strip must never show its findings body in place")
	}
	// The banner no longer expands inline (R09.6 / Paper ZP-0: "its own screen,
	// not an expansion"); activating a row opens the Issues screen instead. The
	// blocker-count assertion below still resolves, because the severity chips
	// travel with the findings and textContent reaches a hidden body.
	runCDP(t, ctx, chromedp.SendKeys("#statusStripToggle", "\n", chromedp.ByQuery))
	pollTrue(t, ctx, `document.getElementById('issuesView').hidden === false`)
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
		var blocked = title.match(/(\d+)(?:\s+\w+)? claims? (?:are |is )?blocked/);
		return blocked && chipCount && Number(blocked[1]) <= claims && Number(chipCount.textContent) <= claims;
	})()`) {
		t.Fatalf("blocker counts must be unique claims, not summed paths (title=%q chip=%q)",
			evalString(t, ctx, `document.getElementById('statusStripTitle').textContent`),
			evalString(t, ctx, `(document.querySelector('.status-severity-chip--blocker .status-severity-chip__count') || {}).textContent`))
	}
}

func TestGroup02MobileNavigationAndFacetSheet(t *testing.T) {
	p := group02NavigationProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(390, 844),
		chromedp.Navigate(p.renderStatic()),
	)
	pollTrue(t, ctx, `!!(document.querySelector('.mobile-app-bar') && document.querySelector('.facet-toc-trigger'))`)
	runCDP(t, ctx, chromedp.Evaluate(`window.dossierxPositionStatusStrip()`, nil))
	requireAll(t, ctx, "reading order must be module heading, facet tabs, status, then claims", `
		var section = document.querySelector('.module-section:not([hidden])');
		var header = section && section.querySelector(':scope > .system-record-head');
		var tabs = section && section.querySelector(':scope > .sub-nav');
		var strip = document.getElementById('statusStrip');
		// Paper 4O0-0/4SX-0 (mobile) and 3B-0 (desktop) all place the banner
		// INSIDE the reading surface as its first child (56T-0 in 4R8-0,
		// 574-0 in 4TN-0, 6P-0 in 6O-0), with the facet tabs in a separate
		// chrome container that is that surface's PRECEDING SIBLING. The
		// reading order is therefore heading -> tabs -> canvas, and inside the
		// canvas, strip -> claims. shell.html gives one element both the
		// .claim-group and .reading-canvas classes, so the strip can never be
		// the tab strip's next sibling AND the canvas's first child; the old
		// assertion encoded the pre-canvas flat DOM and contradicted
		// TestGroup02StatusBelongsToActiveCanvas.
		//
		// The claim is looked up as a DESCENDANT, not a direct child: under
		// SoftMount the cards live inside .claim-group-host, so the strip's
		// next sibling is that host rather than a .claim. Document order is
		// what this test is actually about, so it is asserted directly.
		var canvas = section && section.querySelector(':scope > .reading-canvas:not([hidden])');
		var firstClaim = canvas && canvas.querySelector('.claim');
	`, [][2]string{
		{"the module heading exists", `header`},
		{"the facet tab strip exists", `tabs`},
		{"the status strip exists", `strip`},
		{"the reading canvas exists and is not hidden", `canvas`},
		{"the canvas holds at least one claim", `firstClaim`},
		{"the tabs follow the heading", `header.nextElementSibling === tabs`},
		{"the canvas follows the tabs", `tabs.nextElementSibling === canvas`},
		{"the strip lives inside the canvas", `strip.parentElement === canvas`},
		{"the strip is the canvas's first child", `canvas.firstElementChild === strip`},
		{"the strip precedes the first claim", `!!(strip.compareDocumentPosition(firstClaim) & Node.DOCUMENT_POSITION_FOLLOWING)`},
	})
	requireAll(t, ctx, "mobile app bar must match Paper's 52px menu/title/search/single-theme-control structure", `
		var bar = document.querySelector('.mobile-app-bar');
		var menu = document.getElementById('navToggle');
		var search = document.getElementById('mobileSearchToggle');
		var theme = document.getElementById('mobileThemeToggle');
		var style = bar && getComputedStyle(bar);
	`, [][2]string{
		{"bar exists", `bar`},
		{"menu exists", `menu`},
		{"search exists", `search`},
		{"theme control exists", `theme`},
		{"menu aria-label is 'Open modules'", `menu.getAttribute('aria-label') === 'Open modules'`},
		{"theme control is the toggle form", `theme.hasAttribute('data-theme-toggle')`},
		{"theme control is not a choice form", `!theme.hasAttribute('data-theme-choice')`},
		{"theme aria-label reads 'Use light/dark theme'", `/^Use (light|dark) theme$/.test(theme.getAttribute('aria-label'))`},
		{"exactly one [data-theme-toggle] in the bar", `document.querySelectorAll('.mobile-app-bar [data-theme-toggle]').length === 1`},
		{"no [data-theme-choice] in the bar", `document.querySelectorAll('.mobile-app-bar [data-theme-choice]').length === 0`},
		{"menu carries no 'Sections' label", `!/Sections/.test(menu.textContent)`},
		{"menu min-height is 44px", `getComputedStyle(menu).minHeight === '44px'`},
		{"bar height is 52px", `style.height === '52px'`},
		{"bar padding-left is 16px", `style.paddingLeft === '16px'`},
		{"bar padding-right is 16px", `style.paddingRight === '16px'`},
		{"bar gap is 12px", `style.gap === '12px'`},
	})
	initialTheme := evalString(t, ctx, `document.getElementById('mobileThemeToggle').dataset.themeCurrent`)
	runCDP(t, ctx, chromedp.Click("#mobileThemeToggle", chromedp.ByQuery))
	requireAll(t, ctx, "single mobile theme control must toggle the explicit theme and its current-state icon", `
		var b = document.getElementById('mobileThemeToggle');
		var now = b.dataset.themeCurrent;
		var href = b.querySelector('use').getAttribute('href');
		var was = `+fmt.Sprintf("%q", initialTheme)+`;
	`, [][2]string{
		{"themeCurrent changed from the pre-click value", `now !== was`},
		{"html[data-theme] matches the control", `document.documentElement.getAttribute('data-theme') === now`},
		{"the icon matches the new state", `href === (now === 'dark' ? '#dx-icon-moon' : '#dx-icon-sun')`},
	})
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('mobileSearchToggle').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('nav-open') && document.activeElement === document.getElementById('navSearch')`)
	requireAll(t, ctx, "mobile drawer must keep collapsed Tracks and both 40px utility actions above its fixed footer theme control", `
		var footer = document.querySelector('.sidebar-footer');
		var utilities = footer && footer.querySelector('.nav-utilities');
		var tracks = document.querySelectorAll('.system-nav-group')[1];
		var sidebar = document.querySelector('.sidebar');
	`, [][2]string{
		{"drawer width is 328px", `getComputedStyle(sidebar).width === '328px'`},
		{"the drawer close control exists", `!!document.getElementById('navDrawerClose')`},
		{"the sidebar footer exists", `footer`},
		{"the footer does not shrink", `getComputedStyle(footer).flexShrink === '0'`},
		{"the utilities row exists", `utilities`},
		{"the utilities row has two actions", `utilities.children.length === 2`},
		{"the first action is #dxgOpen", `utilities.children[0].id === 'dxgOpen'`},
		{"the second action targets the build order", `utilities.children[1].dataset.target === '#dossierx-build-order'`},
		{"the first action is 40px tall", `getComputedStyle(utilities.children[0]).height === '40px'`},
		{"a Tracks group exists", `tracks`},
		{"the Tracks group is collapsed", `!tracks.open`},
	})
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('navDrawerClose').focus(); document.getElementById('navDrawerClose').click()`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('nav-open')`)
	if !evalBool(t, ctx, `document.activeElement === document.getElementById('mobileSearchToggle')`) {
		t.Fatalf("drawer close focus = %q, want mobileSearchToggle", evalString(t, ctx, `document.activeElement ? (document.activeElement.id || document.activeElement.tagName) : 'none'`))
	}

	// Native keyboard activation opens from the menu control, and Escape returns
	// focus to that exact opener rather than dropping it on <body>.
	// Focus then Enter (not SendKeys "\n"): CI Chrome did not treat a newline
	// insert as button activation, so Escape closed a drawer whose opener was
	// still the previous search toggle — or restored onto <body> after the
	// in-drawer node was hidden.
	runCDP(t, ctx, chromedp.Focus("#navToggle", chromedp.ByQuery), chromedp.KeyEvent(kb.Enter))
	pollTrue(t, ctx, `document.body.classList.contains('nav-open')`)
	runCDP(t, ctx, chromedp.KeyEvent(kb.Escape))
	pollTrue(t, ctx, `!document.body.classList.contains('nav-open')`)
	pollTrue(t, ctx, `document.activeElement === document.getElementById('navToggle')`)

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc-trigger').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open') && !!document.querySelector('.facet-toc__grabber') && !!document.querySelector('.facet-toc__close')`)
	requireAll(t, ctx, "facet index must use the accessible, bounded bottom-sheet shell", `
		var sheet = document.getElementById('systemFacetToc');
		var close = sheet && sheet.querySelector('.facet-toc__close');
		var sheetStyle = sheet && getComputedStyle(sheet);
	`, [][2]string{
		{"the sheet exists", `sheet`},
		{"the close control exists", `close`},
		{"the close control is labelled", `close.getAttribute('aria-label') === 'Close facet panel'`},
		{"the sheet is bounded at 660px", `parseFloat(sheetStyle.height) <= 660`},
		{"the sheet has a 1px top border", `sheetStyle.borderTopWidth === '1px'`},
		{"the sheet has no left border", `sheetStyle.borderLeftWidth === '0px'`},
		{"the close control is a 44px target", `getComputedStyle(close).minHeight === '44px'`},
	})
	runCDP(t, ctx, chromedp.Focus(".facet-toc__close", chromedp.ByQuery), chromedp.Click(".facet-toc__close", chromedp.ByQuery))
	pollTrue(t, ctx, `!document.body.classList.contains('facet-toc-open') && document.activeElement === document.querySelector('.facet-toc-trigger')`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc-trigger').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open')`)
	runCDP(t, ctx, chromedp.Focus(".facet-toc__close", chromedp.ByQuery), chromedp.KeyEvent(kb.Escape))
	pollTrue(t, ctx, `!document.body.classList.contains('facet-toc-open') && document.activeElement === document.querySelector('.facet-toc-trigger')`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc-trigger').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open')`)
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var item = document.querySelector('.facet-toc__item');
		item.focus();
		item.click();
	})()`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('facet-toc-open') && document.activeElement === document.querySelector('.facet-toc-trigger')`)
}

func TestGroup02DesktopNavigationStructureAndKeyboardActions(t *testing.T) {
	p := group02NavigationProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 1024, chromedp.EmulateScale(1)),
		chromedp.Navigate(p.renderStatic()),
	)
	pollTrue(t, ctx, `!!document.querySelector('.sidebar-footer .nav-utilities')`)
	runCDP(t, ctx, chromedp.Click(`.theme-control [data-theme-choice="light"]`, chromedp.ByQuery))
	if !evalBool(t, ctx, `(function(){
		var header = document.querySelector('.sidebar-header');
		var subtitle = header && header.querySelector(':scope > .eyebrow');
		var toc = document.getElementById('systemFacetToc');
		var tocHead = toc && toc.querySelector('.facet-toc__head');
		var item = toc && toc.querySelector('.facet-toc__item');
		var metric = document.querySelector('#widget > .system-record-head .system-record-head__metric');
		var bar = metric && metric.querySelector('.system-record-head__bar');
		var fill = metric && metric.querySelector('.system-record-head__bar-fill');
		if (!header || !subtitle || !toc || !tocHead || !item || !metric || !bar || !fill) { return false; }
		var hr = header.getBoundingClientRect();
		var sr = subtitle.getBoundingClientRect();
		var ts = getComputedStyle(toc);
		var subtitleStyle = getComputedStyle(subtitle);
		var is = getComputedStyle(item);
		var fillStyle = getComputedStyle(fill);
		return Math.abs(sr.left - (hr.left + 20)) < 0.5 && Math.abs(sr.right - (hr.right - 20)) < 0.5 &&
		  subtitleStyle.fontFamily.indexOf('Inter') >= 0 && subtitleStyle.fontSize === '12px' && subtitleStyle.lineHeight === '16px' &&
		  !document.getElementById('expandAllToggle') &&
		  ts.width === '244px' && ts.top === '36px' && ts.right === '0px' && ts.borderLeftWidth === '0px' &&
		  tocHead.querySelector('small').textContent.trim().toUpperCase() === 'ON THIS FACET' &&
		  getComputedStyle(toc.querySelector('.facet-toc__mobile-identity')).display === 'none' &&
		  item.firstElementChild.matches('strong') && item.lastElementChild.matches('.facet-toc__blocker-count') &&
		  !/^\\d{2}$/.test(item.textContent.trim()) && is.borderRadius === '6px' && is.boxShadow === 'none' &&
		  metric.dataset.allLocked === 'true' && getComputedStyle(bar).width === '80px' && getComputedStyle(bar).height === '6px' &&
		  fillStyle.backgroundColor === 'rgb(44, 107, 82)' && Math.abs(fill.getBoundingClientRect().width - bar.getBoundingClientRect().width) < 0.5;
	})()`) {
		t.Fatalf("desktop Group 02 chrome must match Paper's full-width subtitle, control-free sidebar, 244px facet rail, and green all-locked metric: %s", evalString(t, ctx, `JSON.stringify((function(){
			var header=document.querySelector('.sidebar-header'), subtitle=header&&header.querySelector(':scope > .eyebrow');
			var toc=document.getElementById('systemFacetToc'), item=toc&&toc.querySelector('.facet-toc__item');
			var metric=document.querySelector('#widget > .system-record-head .system-record-head__metric');
			var bar=metric&&metric.querySelector('.system-record-head__bar'), fill=metric&&metric.querySelector('.system-record-head__bar-fill');
			var hr=header&&header.getBoundingClientRect(), sr=subtitle&&subtitle.getBoundingClientRect();
			return {header:[hr&&hr.left,hr&&hr.right],subtitle:[sr&&sr.left,sr&&sr.right,getComputedStyle(subtitle).fontFamily,getComputedStyle(subtitle).fontSize,getComputedStyle(subtitle).lineHeight],expand:!!document.getElementById('expandAllToggle'),toc:toc&&[getComputedStyle(toc).width,getComputedStyle(toc).top,getComputedStyle(toc).right,getComputedStyle(toc).borderLeftWidth],head:toc&&toc.querySelector('.facet-toc__head').textContent.trim(),mobile:toc&&getComputedStyle(toc.querySelector('.facet-toc__mobile-identity')).display,item:item&&[item.innerHTML,getComputedStyle(item).borderRadius,getComputedStyle(item).boxShadow],metric:metric&&metric.dataset.allLocked,bar:bar&&[getComputedStyle(bar).width,getComputedStyle(bar).height],fill:fill&&[getComputedStyle(fill).backgroundColor,fill.getBoundingClientRect().width,bar.getBoundingClientRect().width]};
		})())`))
	}
	if !evalBool(t, ctx, `(function(){
		var footer = document.querySelector('.freshness-footer');
		var phrase = footer && footer.querySelector('.freshness-footer__phrase');
		var caption = footer && footer.querySelector('.freshness-footer__caption');
		return footer && phrase && caption && phrase.textContent !== 'Live' && !caption.hidden &&
		  caption.textContent.trim() === 'Claims changed since then are not in this view';
	})()`) {
		t.Fatal("desktop facet footer must retain its generated freshness phrase and Paper caption")
	}
	if !evalBool(t, ctx, `(function(){
		var nav = document.getElementById('nav');
		var scroll = nav.querySelector(':scope > .sidebar-nav-scroll');
		var footer = nav.querySelector(':scope > .sidebar-footer');
		var utilities = footer && footer.querySelector(':scope > .nav-utilities');
		var choices = footer && footer.querySelectorAll('.theme-control [data-theme-choice]');
		var groups = scroll && scroll.querySelectorAll('.system-nav-group');
		return scroll && footer && getComputedStyle(footer).flexShrink === '0' &&
		  utilities && utilities.children.length === 2 && utilities.children[0].id === 'dxgOpen' &&
		  utilities.children[1].dataset.target === '#dossierx-build-order' &&
		  getComputedStyle(utilities.children[0]).height === '30px' &&
		  groups && groups.length === 2 && groups[0].open && !groups[1].open &&
		  !scroll.querySelector('[data-dxg-open]') && !scroll.querySelector('[data-target="#dossierx-build-order"]') &&
		  choices && choices.length === 2 && choices[0].dataset.themeChoice === 'light' && choices[1].dataset.themeChoice === 'dark' &&
		  !footer.querySelector('[data-theme-choice="system"]');
	})()`) {
		t.Fatal("desktop navigation must match Paper's collapsed Tracks, fixed two-action footer, and Light/Dark-only control")
	}
	runCDP(t, ctx, chromedp.Click(`.theme-control [data-theme-choice="dark"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `(function(){
		var control = document.querySelector('.theme-control');
		var selected = control && control.querySelector('[data-theme-choice="dark"]');
		return control && selected && getComputedStyle(control).columnGap === '0px' &&
		  getComputedStyle(selected).columnGap === '6px' &&
		  getComputedStyle(selected).backgroundColor === 'rgb(36, 44, 56)';
	})()`)
	if !evalBool(t, ctx, `(function(){
		var metric = document.querySelector('#widget > .system-record-head .system-record-head__metric');
		var fill = metric && metric.querySelector('.system-record-head__bar-fill');
		return metric && fill && metric.dataset.allLocked === 'true' &&
		  getComputedStyle(fill).backgroundColor === 'rgb(99, 190, 154)';
	})()`) {
		t.Fatal("dark all-locked metric must use the Paper confirmation green independent of the project accent")
	}

	// Module and Build order remain ordinary keyboard-reachable reading-view
	// selections even though Build order moved out of the accordion stack.
	runCDP(t, ctx, chromedp.SendKeys(`.system-nav-group .sec-tab[data-target="#gadget"]`, "\n", chromedp.ByQuery))
	pollTrue(t, ctx, `!document.getElementById('gadget').hidden && document.querySelector('.sec-tab[data-target="#gadget"]').classList.contains('on')`)
	runCDP(t, ctx, chromedp.SendKeys(`.sidebar-footer .sec-tab[data-target="#dossierx-build-order"]`, "\n", chromedp.ByQuery))
	pollTrue(t, ctx, `!document.getElementById('dossierx-build-order').hidden && document.querySelector('.sidebar-footer .sec-tab[data-target="#dossierx-build-order"]').classList.contains('on')`)

	// Claims graph is a button, not a reading tab. Enter opens it, focus moves
	// into the pane, and Escape returns to the exact footer trigger.
	runCDP(t, ctx, chromedp.SendKeys("#dxgOpen", "\n", chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('dxg-open') && !!document.activeElement.closest('#dxgPane')`)
	runCDP(t, ctx, chromedp.KeyEvent(kb.Escape))
	pollTrue(t, ctx, `!document.body.classList.contains('dxg-open') && document.activeElement === document.getElementById('dxgOpen')`)
}

func TestLiveFreshnessRetainsTimestampAndCaption(t *testing.T) {
	p := group02NavigationProject(t)
	base, stop := p.serve()
	defer stop()
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 1024, chromedp.EmulateScale(1)),
		chromedp.Navigate(base+"/"),
	)
	pollTrue(t, ctx, `document.body.classList.contains('comments-live') && !!document.querySelector('.freshness-footer__live:not([hidden])')`)
	if !evalBool(t, ctx, `(function(){
		var phrase = document.querySelector('.freshness-footer__phrase');
		var caption = document.querySelector('.freshness-footer__caption');
		var live = document.querySelector('.freshness-footer__live');
		return phrase && caption && live && phrase.textContent !== 'Live' &&
		  phrase.dataset.generatedAt && !caption.hidden && !live.hidden && live.textContent.trim() === 'Live' &&
		  caption.textContent.trim() === 'Claims changed since then are not in this view';
	})()`) {
		t.Fatal("live serve must add a truthful Live badge without replacing the generated freshness phrase or hiding the Paper caption")
	}
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

func TestReadyConformanceRendersAsClaimFooterDoor(t *testing.T) {
	p := newProjectRaw(t, conformanceConfigYAML)
	p.writeClaim("overview.yaml", conformanceClaimYAML)
	writeConformanceObservation(t, p, `["blocked","ready"]`)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(p.renderStatic()))
	pollTrue(t, ctx, `!!document.querySelector('.claim-footer > .claim-conformance-door')`)
	if !evalBool(t, ctx, `(function(){
		var footer = document.querySelector('.claim-footer');
		var door = footer && footer.querySelector(':scope > .claim-conformance-door');
		var panel = footer && footer.querySelector(':scope > .claim-conformance');
		return door && panel && !door.open && /^\d+ checks?$/.test(door.textContent.trim()) &&
			!footer.textContent.includes('READY') && getComputedStyle(panel).display === 'none' &&
			!document.querySelector('.claim-collapse-toggle, .facet-claims-toggle');
	})()`) {
		t.Fatalf("ready implementation checks must be a closed footer door with no standalone READY box: %s", evalString(t, ctx, `JSON.stringify((function(){var footer=document.querySelector('.claim-footer');var door=footer&&footer.querySelector(':scope > .claim-conformance-door');var panel=footer&&footer.querySelector(':scope > .claim-conformance');return {html:footer&&footer.innerHTML,open:door&&door.open,text:door&&door.textContent.trim(),panel:panel&&getComputedStyle(panel).display,collapse:!!document.querySelector('.claim-collapse-toggle, .facet-claims-toggle')};})())`))
	}
	runCDP(t, ctx, chromedp.Click(".claim-conformance-door > summary", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.claim-conformance-door').open && getComputedStyle(document.querySelector('.claim-conformance')).display !== 'none'`)
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
	// The strip has no expanded form at all — no board draws one, and the
	// grouped findings list it builds is MOVED into the Issues screen rather
	// than shown in place. The old assertion read a class that no longer
	// exists, which is a check that cannot fail; this reads the body's own
	// visibility, which is the thing "stays collapsed" actually meant.
	if evalBool(t, ctx, `!document.getElementById('statusStripBody').hidden`) {
		t.Fatal("390px blocker-only strip must never show its findings body in place")
	}

	runCDP(t, ctx, chromedp.Navigate(ready.renderStatic()))
	pollTrue(t, ctx, `!!document.querySelector('.claim-conformance')`)
	if evalBool(t, ctx, `(function(){
		var panel = document.querySelector('.claim-conformance');
		return !!(panel && panel.getClientRects().length);
	})()`) {
		t.Fatal("390px ready claim must initially hide implementation-check details behind its footer door")
	}
	runCDP(t, ctx, chromedp.Click(".claim-conformance-door > summary", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.claim-conformance-door').open && document.querySelector('.claim-conformance').getClientRects().length > 0`)
}
