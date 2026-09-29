package viewertests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// clickNavigationGroupSummary keeps the real pointer hit-target assertion and
// waits for the native <details> toggle event, the open property, and the
// reader-owned preference written by the production handler.
func clickNavigationGroupSummary(t *testing.T, ctx context.Context, selector string, wantOpen bool) {
	t.Helper()
	var before int
	observe := fmt.Sprintf(`(function(){
		var summary = document.querySelector(%q);
		var group = summary && summary.parentElement;
		if (!group) { return -1; }
		if (group.dataset.testToggleObserved !== 'true') {
			group.dataset.testToggleObserved = 'true';
			group.dataset.testToggleCount = '0';
			group.dataset.testSummaryClickCount = '0';
			summary.addEventListener('click', function (event) {
				group.dataset.testSummaryClickCount = String(Number(group.dataset.testSummaryClickCount || '0') + 1);
				group.dataset.testOpenDuringClick = String(group.open);
				group.dataset.testClickPrevented = String(event.defaultPrevented);
			});
			group.addEventListener('toggle', function () {
				group.dataset.testToggleCount = String(Number(group.dataset.testToggleCount || '0') + 1);
			});
		}
		return Number(group.dataset.testToggleCount || '0');
	})()`, selector)
	runCDP(t, ctx, chromedp.Evaluate(observe, &before))
	if before < 0 {
		t.Fatalf("navigation summary %q was not found", selector)
	}
	// The navigation itself scrolls. A summary can still be DOM-visible while
	// its centre is clipped by that scrollport, and chromedp's page-level
	// visibility check does not establish that the pointer will hit it.
	runCDP(t, ctx, chromedp.Evaluate(fmt.Sprintf(`document.querySelector(%q).scrollIntoView({block:'nearest', inline:'nearest'})`, selector), nil))
	pollTrue(t, ctx, fmt.Sprintf(`(function(){
		var summary = document.querySelector(%q);
		var nav = document.getElementById('nav');
		if (!summary || !nav) { return false; }
		var r = summary.getBoundingClientRect();
		var n = nav.getBoundingClientRect();
		var x = r.left + r.width / 2;
		var y = r.top + r.height / 2;
		var hit = document.elementFromPoint(x, y);
		return r.width > 0 && r.height > 0 && r.top >= n.top && r.bottom <= n.bottom &&
			!!hit && (hit === summary || summary.contains(hit));
	})()`, selector))
	runCDP(t, ctx, chromedp.Click(selector, chromedp.ByQuery))
	wantReaderClosed := !wantOpen
	settled := fmt.Sprintf(`(function(){
		var summary = document.querySelector(%q);
		var group = summary && summary.parentElement;
		return !!group && Number(group.dataset.testSummaryClickCount || '0') > %d &&
			Number(group.dataset.testToggleCount || '0') > %d &&
			group.open === %t && group.dataset.readerClosed === %q;
	})()`, selector, before, before, wantOpen, fmt.Sprint(wantReaderClosed))
	var ok bool
	if err := chromedp.Run(ctx, chromedp.Poll(settled, &ok,
		chromedp.WithPollingInterval(40*time.Millisecond),
		chromedp.WithPollingTimeout(20*time.Second))); err != nil {
		var state string
		diagnostic := fmt.Sprintf(`(function(){
			var summary = document.querySelector(%q);
			var group = summary && summary.parentElement;
			return group ? JSON.stringify({open:group.open, readerClosed:group.dataset.readerClosed,
				clicks:group.dataset.testSummaryClickCount, toggles:group.dataset.testToggleCount,
				openDuringClick:group.dataset.testOpenDuringClick, clickPrevented:group.dataset.testClickPrevented}) : 'missing';
		})()`, selector)
		diagnosticErr := chromedp.Run(ctx, chromedp.Evaluate(diagnostic, &state))
		t.Fatalf("navigation summary %q did not settle open=%t after a real pointer click: %v; state=%s; diagnostic=%v", selector, wantOpen, err, state, diagnosticErr)
	}
}

const navigationGroupsConfigYAML = `schema_version: 1
facets:
  - contract
  - internals
modules:
  - module-01
  - module-02
  - module-03
claims_dir: claims
`

const navigationGroupsClaimYAML = `id: %[1]s.contract.overview
facet: contract
module: %[1]s
status: draft
summary: Fixture claim used by the engine test corpus.
body: |
  a claim under review.
rests_on:
  none: true
  reason: viewer-test fixture, not backed by any doctrine claim
`

// A navigation group is both an automatic orientation aid and a reader-owned
// disclosure. Selecting a module opens its group, but an explicit click on the
// active group's summary must be allowed to close it and stay closed, and the
// native summary must reopen it from the keyboard.
func TestActiveNavigationGroupsCanStayCollapsed(t *testing.T) {
	p := newProjectRaw(t, navigationGroupsConfigYAML)
	for i := 1; i <= 3; i++ {
		module := fmt.Sprintf("module-%02d", i)
		p.writeClaim(module+".yaml", fmt.Sprintf(navigationGroupsClaimYAML, module))
	}
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".system-nav-group", chromedp.ByQuery),
	)

	if !evalBool(t, ctx, `(function(){
		var groups = document.querySelectorAll('.system-nav-group');
		return groups.length === 1 &&
			groups[0].querySelectorAll('.sec-tab').length === 3;
	})()`) {
		t.Fatal("project must render one Modules group holding every module")
	}
	if !evalBool(t, ctx, `document.querySelectorAll('.system-nav-group')[0].open`) {
		t.Fatal("Modules must start expanded")
	}

	// module-01 is initially active, so its row already has .on before any
	// click. Use the second module: waiting for .on then proves that the
	// pointer action selected it instead of accepting stale readiness.
	const secondModule = ".system-nav-group:first-child .sec-tab:nth-child(2)"
	if evalBool(t, ctx, `document.querySelector("`+secondModule+`").classList.contains('on')`) {
		t.Fatal("the module click fixture must start on a different module")
	}
	// Register after production's document listener. Its zero-delay marker is
	// queued after production's zero-delay forced-open task, so this observes
	// the real navigation boundary instead of guessing with elapsed time.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		document.documentElement.dataset.testNavigationSettled = 'false';
		function afterNavigation(event) {
			if (!event.target.closest("`+secondModule+`")) { return; }
			document.removeEventListener('click', afterNavigation);
			setTimeout(function(){ document.documentElement.dataset.testNavigationSettled = 'true'; }, 0);
		}
		document.addEventListener('click', afterNavigation);
	})()`, nil))
	runCDP(t, ctx, chromedp.Click(secondModule, chromedp.ByQuery))
	// Navigation marks the tab synchronously, then syncs its disclosure from a
	// zero-delay callback. Waiting only for .on can therefore race that pending
	// callback with the next summary click.
	pollTrue(t, ctx, `document.documentElement.dataset.testNavigationSettled === 'true' &&
		document.querySelector("`+secondModule+`").classList.contains('on') &&
		document.querySelectorAll('.system-nav-group')[0].open &&
		document.querySelectorAll('.system-nav-group')[0].dataset.readerClosed === 'false'`)

	clickNavigationGroupSummary(t, ctx, ".system-nav-group:first-child > summary", false)

	// Activate the native summary by keyboard and prove both the disclosure
	// and its reader preference changed; a selector that matched no group
	// would leave the assertion below false.
	runCDP(t, ctx, chromedp.SendKeys(".system-nav-group:first-child > summary", "\n", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelectorAll('.system-nav-group')[0].open &&
		document.querySelectorAll('.system-nav-group')[0].dataset.readerClosed === 'false'`)
}
