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
modules:
  - module-01
  - module-02
  - module-03
  - module-04
  - module-05
  - module-06
  - module-07
  - module-08
  - module-09
  - module-10
  - module-11
  - module-12
  - module-13
  - module-14
  - module-15
  - module-16
  - module-17
  - module-18
  - module-19
  - module-20
  - module-21
  - module-22
  - module-23
  - module-24
  - module-25
  - module-26
tracks:
  - {id: ft-01, title: Feature Track 01}
  - {id: ft-02, title: Feature Track 02}
  - {id: ft-03, title: Feature Track 03}
  - {id: ft-04, title: Feature Track 04}
  - {id: ft-05, title: Feature Track 05}
  - {id: ft-06, title: Feature Track 06}
  - {id: ft-07, title: Feature Track 07}
  - {id: ft-08, title: Feature Track 08}
  - {id: ft-09, title: Feature Track 09}
  - {id: ft-10, title: Feature Track 10}
  - {id: ft-11, title: Feature Track 11}
  - {id: ft-12, title: Feature Track 12}
  - {id: ft-13, title: Feature Track 13}
  - {id: ft-14, title: Feature Track 14}
  - {id: ft-15, title: Feature Track 15}
  - {id: ft-16, title: Feature Track 16}
  - {id: ft-17, title: Feature Track 17}
  - {id: ft-18, title: Feature Track 18}
  - {id: ft-19, title: Feature Track 19}
  - {id: ft-20, title: Feature Track 20}
  - {id: ft-21, title: Feature Track 21}
  - {id: ft-22, title: Feature Track 22}
  - {id: ft-23, title: Feature Track 23}
  - {id: ft-24, title: Feature Track 24}
  - {id: ft-25, title: Feature Track 25}
claims_dir: claims
`

const navigationGroupsClaimYAML = `id: %[1]s.contract.overview
facet: contract
module: %[1]s
status: draft
body: |
  a claim under review.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
%[2]s
`

// A navigation group is both an automatic orientation aid and a reader-owned
// disclosure. Selecting a module or track may open its group, but an explicit
// click on the active group's summary must be allowed to close it and stay
// closed. This is also what makes the Tracks group reachable in a project with
// enough modules to fill the sidebar.
func TestActiveNavigationGroupsCanStayCollapsed(t *testing.T) {
	p := newProjectRaw(t, navigationGroupsConfigYAML)
	for i := 1; i <= 26; i++ {
		module := fmt.Sprintf("module-%02d", i)
		track := ""
		if i <= 25 {
			track = fmt.Sprintf("tracks:\n  - id: ft-%02d\n    role: owns", i)
		}
		p.writeClaim(module+".yaml", fmt.Sprintf(navigationGroupsClaimYAML, module, track))
	}
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".system-nav-group", chromedp.ByQuery),
	)

	if !evalBool(t, ctx, `(function(){
		var groups = document.querySelectorAll('.system-nav-group');
		return groups.length === 2 &&
			groups[0].querySelectorAll('.sec-tab').length === 26 &&
			groups[1].querySelectorAll('.sec-tab').length === 25;
	})()`) {
		t.Fatal("tracked project must render separate Modules and Tracks groups")
	}
	if !evalBool(t, ctx, `document.querySelectorAll('.system-nav-group')[0].open && document.querySelectorAll('.system-nav-group')[1].open`) {
		t.Fatal("navigation groups must start expanded")
	}
	if !evalBool(t, ctx, `(function(){
		var nav = document.getElementById('nav').getBoundingClientRect();
		var tracks = document.querySelectorAll('.system-nav-group')[1].getBoundingClientRect();
		return tracks.top >= nav.bottom;
	})()`) {
		t.Fatal("large module list must reproduce Tracks starting below the visible navigation area")
	}

	clickNavigationGroupSummary(t, ctx, ".system-nav-group:first-child > summary", false)
	if !evalBool(t, ctx, `(function(){
		var nav = document.getElementById('nav').getBoundingClientRect();
		var tracks = document.querySelectorAll('.system-nav-group')[1].querySelector('summary').getBoundingClientRect();
		return tracks.top >= nav.top && tracks.bottom <= nav.bottom;
	})()`) {
		t.Fatal("collapsing Modules must bring the Tracks header into the visible navigation area")
	}

	// The first track owns the initially active module-01 claim, so it already
	// has .on before any click. Use the second track: waiting for .on then proves
	// that the pointer action selected it instead of accepting stale readiness.
	const secondTrack = ".system-nav-group:nth-child(2) .sec-tab:nth-child(2)"
	if evalBool(t, ctx, `document.querySelector("`+secondTrack+`").classList.contains('on')`) {
		t.Fatal("the track click fixture must start on a different track")
	}
	// Register after production's document listener. Its zero-delay marker is
	// queued after production's zero-delay forced-open task, so this observes
	// the real navigation boundary instead of guessing with elapsed time.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		document.documentElement.dataset.testNavigationSettled = 'false';
		function afterNavigation(event) {
			if (!event.target.closest("`+secondTrack+`")) { return; }
			document.removeEventListener('click', afterNavigation);
			setTimeout(function(){ document.documentElement.dataset.testNavigationSettled = 'true'; }, 0);
		}
		document.addEventListener('click', afterNavigation);
	})()`, nil))
	runCDP(t, ctx, chromedp.Click(secondTrack, chromedp.ByQuery))
	// Navigation marks the tab synchronously, then syncs its disclosure from a
	// zero-delay callback. Waiting only for .on can therefore race that pending
	// callback with the next summary click.
	pollTrue(t, ctx, `document.documentElement.dataset.testNavigationSettled === 'true' &&
		document.querySelector("`+secondTrack+`").classList.contains('on') &&
		document.querySelectorAll('.system-nav-group')[1].open &&
		document.querySelectorAll('.system-nav-group')[1].dataset.readerClosed === 'false'`)

	clickNavigationGroupSummary(t, ctx, ".system-nav-group:nth-child(2) > summary", false)

	clickNavigationGroupSummary(t, ctx, ".system-nav-group:nth-child(2) > summary", true)
}
