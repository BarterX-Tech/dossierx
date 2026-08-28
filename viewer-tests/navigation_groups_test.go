package viewertests

import (
	"fmt"
	"testing"

	"github.com/chromedp/chromedp"
)

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

	runCDP(t, ctx, chromedp.Click(".system-nav-group:first-child > summary", chromedp.ByQuery))
	pollTrue(t, ctx, `!document.querySelectorAll('.system-nav-group')[0].open`)
	if !evalBool(t, ctx, `(function(){
		var nav = document.getElementById('nav').getBoundingClientRect();
		var tracks = document.querySelectorAll('.system-nav-group')[1].querySelector('summary').getBoundingClientRect();
		return tracks.top >= nav.top && tracks.bottom <= nav.bottom;
	})()`) {
		t.Fatal("collapsing Modules must bring the Tracks header into the visible navigation area")
	}

	runCDP(t, ctx, chromedp.Click(".system-nav-group:nth-child(2) .sec-tab", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelectorAll('.system-nav-group')[1].querySelector('.sec-tab').classList.contains('on')`)

	runCDP(t, ctx, chromedp.Click(".system-nav-group:nth-child(2) > summary", chromedp.ByQuery))
	pollTrue(t, ctx, `!document.querySelectorAll('.system-nav-group')[1].open`)

	runCDP(t, ctx, chromedp.Click(".system-nav-group:nth-child(2) > summary", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelectorAll('.system-nav-group')[1].open`)
}
