package viewertests

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// homeVisible is true when Home is the one section on screen and the Home tab
// is the current one.
const homeVisible = `(function(){
  var home = document.getElementById('_home');
  var shown = Array.prototype.filter.call(document.querySelectorAll('.module-section'), function (s) { return !s.hidden; });
  return !!home && !home.hidden && shown.length === 1 && document.querySelector('#nav .home-tab').classList.contains('on');
})()`

// Home (NIT-196), as a reader meets it in a real browser: the viewer opens on
// it, a hash the viewer does not know lands on it, a card leads to the claim
// it counts, and the project name leads back. The sidebar's Modules group is
// open only while a module is the current page or a search is in progress.
func TestHomeIsTheLandingPage(t *testing.T) {
	p := newProject(t)
	url := p.renderStatic()

	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("#_home", chromedp.ByQuery),
	)

	requireAll(t, ctx, "Home on first open",
		`var home = document.getElementById('_home');
		 var group = document.querySelector('#nav .system-nav-group');
		 var card = home.querySelector('.home-card[data-kind="draft"]');`,
		[][2]string{
			{"Home alone is shown and current", homeVisible},
			{"the Modules group is closed", `group.open === false`},
			{"the draft card counts the one draft", `!!card && card.querySelector('.home-card__count').textContent.trim() === '1'`},
			{"the draft card leads to it", `card.getAttribute('href') === '#` + testClaimID + `'`},
			{"no card counts what is not there", `home.querySelectorAll('.home-card').length === 1`},
			{"the header names the project", `home.querySelector('.home-title').textContent.trim() === document.querySelector('.sidebar h1').textContent.trim()`},
		})

	// A search opens the Modules group so its matches show, and clearing it
	// closes the group again while Home is the current page.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){ var s = document.getElementById('navSearch'); s.value = 'widg'; s.dispatchEvent(new Event('input', {bubbles: true})); })()`, nil))
	requireAll(t, ctx, "a search in progress on Home",
		`var group = document.querySelector('#nav .system-nav-group');
		 var row = group.querySelector('.sec-tab[data-target="#widget"]');`,
		[][2]string{
			{"the Modules group is open", `group.open === true`},
			{"the matching module is visible", `!row.hidden && row.offsetParent !== null`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`(function(){ var s = document.getElementById('navSearch'); s.value = ''; s.dispatchEvent(new Event('input', {bubbles: true})); })()`, nil))
	pollTrue(t, ctx, `document.querySelector('#nav .system-nav-group').open === false`)

	// A card is a link: following it opens the claim's module on the claim.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#_home .home-card[data-kind="draft"]').click();`, nil))
	pollTrue(t, ctx, `document.getElementById('_home').hidden && !document.getElementById('widget').hidden`)
	requireAll(t, ctx, "the claim a card leads to",
		`var group = document.querySelector('#nav .system-nav-group');
		 var modTab = document.querySelector('#nav .sec-tab[data-target="#widget"]');`,
		[][2]string{
			{"the claim is in view", `document.getElementById('` + testClaimID + `').offsetParent !== null`},
			{"the hash names the claim", `location.hash === '#` + testClaimID + `'`},
			{"the Modules group opened", `group.open === true`},
			{"its module is the current tab", `modTab.classList.contains('on') && !document.querySelector('#nav .home-tab').classList.contains('on')`},
		})

	// A hash the viewer does not know, reached from a module page, lands on
	// Home: it does not leave the reader where they were.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#no-such-page';`, nil))
	pollTrue(t, ctx, homeVisible)

	// The project name leads home from a module page, and the group closes.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#widget';`, nil))
	pollTrue(t, ctx, `!document.getElementById('widget').hidden`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.sidebar h1 .home-link').click();`, nil))
	pollTrue(t, ctx, homeVisible)
	if evalBool(t, ctx, `document.querySelector('#nav .system-nav-group').open`) {
		t.Fatal("the Modules group stayed open on Home")
	}

	// A fresh open on an unknown hash lands on Home too.
	fresh := withInstantScroll(t, browserContext(t))
	desktopViewport(t, fresh)
	runCDP(t, fresh, chromedp.Navigate(url+"#no-such-page"), chromedp.WaitVisible("#_home", chromedp.ByQuery))
	pollTrue(t, fresh, homeVisible)
}

// A module named "home" is an ordinary module (the section id slugify gives
// it is "home"; Home's own id is "_home", which slugify never produces). It
// must render under its own tab, alone, and the viewer must still open on
// Home.
func TestModuleNamedHomeIsNotHome(t *testing.T) {
	p := newProjectRaw(t, `schema_version: 1
facets:
  - contract
  - internals
modules:
  - widget
  - home
claims_dir: claims
`)
	p.writeClaim("overview.yaml", draftClaimYAML)
	p.writeClaim("home-thing.yaml", railClaim("home.contract.thing", "contract", "home", ""))
	url := p.renderStatic()
	ctx := browserContext(t)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url), chromedp.WaitVisible("#_home", chromedp.ByQuery))

	requireAll(t, ctx, "opening a project with a module named home", ``, [][2]string{
		{"one element per id", `document.querySelectorAll('[id="home"]').length === 1 && document.querySelectorAll('[id="_home"]').length === 1`},
		{"Home alone is shown and current", homeVisible},
	})

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .sec-tab[data-target="#home"]').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('home').hidden`)
	if got := evalString(t, ctx, visibleSectionsExpr); got != "home|module-section" {
		t.Fatalf("visible sections after clicking the home module's tab = %q, want the module's section alone", got)
	}
	if evalBool(t, ctx, `document.querySelector('#nav .home-tab').classList.contains('on')`) {
		t.Fatal("the Home tab is marked current while the home module is shown")
	}
}

// Home is not a facet, so the status strip, which counts findings "in this
// facet", stays off it. Only a project-wide finding (no claim id) can reach
// the strip while Home is showing: a claim-scoped one is filtered out by the
// empty facet on its own. The finding is delivered through the same hook the
// Issues tests use in place of a live /api/status answer.
func TestStatusStripStaysOffHome(t *testing.T) {
	p := newProject(t)
	url := p.renderStatic()
	ctx := browserContext(t)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+widgetPage), chromedp.WaitVisible(`[id="`+testClaimID+`"]`, chromedp.ByQuery))

	evalVoid(t, ctx, `window.dossierxRenderStatusStrip({
		ledger_findings: [{rule: 'lock-tamper', message: 'project-wide finding'}],
		readiness: {}
	})`)
	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .home-tab').click();`, nil))
	pollTrue(t, ctx, homeVisible)
	if !evalBool(t, ctx, `document.getElementById('statusStrip').hidden`) {
		t.Fatal("the status strip shows on Home for a project-wide finding")
	}

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .sec-tab[data-target="#widget"]').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('widget').hidden && !document.getElementById('statusStrip').hidden`)
}
