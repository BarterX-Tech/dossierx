package viewertests

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// Home (NIT-196), as a reader meets it in a real browser: the viewer opens on
// it, a hash the viewer does not know lands on it, a card leads to the claim
// it counts, and the project name leads back. The sidebar's Modules group is
// open only while a module is the current page.
func TestHomeIsTheLandingPage(t *testing.T) {
	p := newProject(t)
	url := p.renderStatic()

	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("#home", chromedp.ByQuery),
	)

	requireAll(t, ctx, "Home on first open",
		`var home = document.getElementById('home');
		 var tab = document.querySelector('#nav .home-tab');
		 var group = document.querySelector('#nav .system-nav-group');
		 var modules = document.querySelectorAll('.module-section:not(.home-section):not(.constitution-section)');
		 var card = home.querySelector('.home-card[data-kind="draft"]');`,
		[][2]string{
			{"Home is shown", `!home.hidden && home.offsetParent !== null`},
			{"every other section is hidden", `Array.prototype.every.call(modules, function (m) { return m.hidden; }) && document.getElementById('constitution').hidden`},
			{"the Home tab is current", `tab.classList.contains('on')`},
			{"the Modules group is closed", `group.open === false`},
			{"the draft card counts the one draft", `!!card && card.querySelector('.home-card__count').textContent.trim() === '1'`},
			{"the draft card leads to it", `card.getAttribute('href') === '#` + testClaimID + `'`},
			{"no card counts what is not there", `home.querySelectorAll('.home-card').length === 1`},
			{"the header names the project", `home.querySelector('.home-title').textContent.trim() === document.querySelector('.sidebar h1').textContent.trim()`},
		})

	// A card is a link: following it opens the claim's module on the claim.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#home .home-card[data-kind="draft"]').click();`, nil))
	pollTrue(t, ctx, `document.getElementById('home').hidden && !document.getElementById('widget').hidden`)
	requireAll(t, ctx, "the claim a card leads to",
		`var group = document.querySelector('#nav .system-nav-group');
		 var modTab = document.querySelector('#nav .sec-tab[data-target="#widget"]');`,
		[][2]string{
			{"the claim is in view", `document.getElementById('` + testClaimID + `').offsetParent !== null`},
			{"the hash names the claim", `location.hash === '#` + testClaimID + `'`},
			{"the Modules group opened", `group.open === true`},
			{"its module is the current tab", `modTab.classList.contains('on') && !document.querySelector('#nav .home-tab').classList.contains('on')`},
		})

	// The project name leads home, and the Modules group closes again.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.sidebar h1 .home-link').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('home').hidden && document.getElementById('widget').hidden`)
	requireAll(t, ctx, "back on Home",
		`var group = document.querySelector('#nav .system-nav-group');`,
		[][2]string{
			{"the Home tab is current", `document.querySelector('#nav .home-tab').classList.contains('on')`},
			{"the Modules group closed", `group.open === false`},
		})

	// A hash the viewer does not know lands on Home, not on a module.
	runCDP(t, ctx, chromedp.Navigate(url+"#no-such-page"))
	pollTrue(t, ctx, `!document.getElementById('home').hidden`)
	requireAll(t, ctx, "an unknown hash",
		``,
		[][2]string{
			{"no module is shown", `document.getElementById('widget').hidden`},
		})
}
