package viewertests

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// The brief pages and the Briefs sidebar tree (NIT-197), driven in a real
// browser against a static build: routing by hash, the tree's open state, the
// image the build copies beside the viewer, light and dark, the phone drawer
// and the "On this page" sheet, search, and brief findings in the status strip.
// The markup itself is pinned at its owner, internal/render
// (TestRender_BriefPageAndTree); what is asserted here is what only a browser
// can show.

const roundBrief = "brief-decisions-round-to-the-cent"

// briefProject is the brief fixture: two folders, a locked brief with a real
// PNG that rests on the fixture claim, a draft beside it, a brief in a second
// folder, and a features/ brief the tree must leave out.
func briefProject(t *testing.T) *project {
	t.Helper()
	p := newProject(t)
	// A second claim, so the brief rests on two and its relations card has a
	// row with the hairline under it.
	p.writeClaim("rounding.yaml", railClaim("widget.contract.rounding", "contract", "widget", ""))
	write := func(rel string, data []byte) {
		path := filepath.Join(p.dir, "briefs", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("decisions/round-to-the-cent.md", []byte("---\nsummary: Every share is rounded exactly once, at display.\nstatus: locked\nrests_on:\n  - "+testClaimID+"\n  - widget.contract.rounding\n---\n"+
		"# Balances round to the cent, once\n\n## Context\n\nThree people split a bill.\n\n![The payer's share carries the cent](split.png)\n\n## Decision\n\nRound once, at display.\n"))
	write("decisions/no-bank-linking.md", []byte("---\nsummary: Tally never connects to a bank.\n---\n# No bank linking\n\nPeople type what they spent.\n"))
	write("research/interviews.md", []byte("---\nsummary: What twelve households told us.\n---\n## Findings\n\nMost settle monthly.\n"))
	write("features/split-a-bill.md", []byte("---\nsummary: Splitting one bill.\n---\n# Split a bill\n\nA feature brief.\n"))

	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		for y := 0; y < 20; y++ {
			img.Set(x, y, color.RGBA{R: 194, G: 202, B: 213, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	write("decisions/split.png", buf.Bytes())
	// A brief that says locked is approved through brief lock (NIT-205);
	// the status line alone would be brief-unrecorded and fail check.
	p.run("brief", "lock", "briefs/decisions/round-to-the-cent.md", "--reason", "fixture approval")
	return p
}

// briefOnlyShown is true when the round-to-the-cent page is the one section on
// screen.
const briefOnlyShown = `(function(){
  var shown = Array.prototype.filter.call(document.querySelectorAll('.module-section'), function (s) { return !s.hidden; });
  return shown.length === 1 && shown[0].id === '` + roundBrief + `';
})()`

func TestBriefPage_OpensByHashWithItsTreeAndImage(t *testing.T) {
	p := briefProject(t)
	url := p.renderStatic()

	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+roundBrief), chromedp.WaitVisible("#"+roundBrief, chromedp.ByQuery))
	pollTrue(t, ctx, `(function(){ var i = document.querySelector('#`+roundBrief+` img.md-img'); return !!i && i.complete; })()`)

	requireAll(t, ctx, "a brief opened by its hash",
		`var sec = document.getElementById('`+roundBrief+`');
		 var nav = document.getElementById('nav');
		 var groups = nav.querySelectorAll('.system-nav-group');
		 var briefsGroup = nav.querySelector('.system-nav-group.brief-nav');
		 var row = nav.querySelector('.sec-tab[data-target="#`+roundBrief+`"]');
		 var img = sec.querySelector('img.md-img');
		 var toc = document.getElementById('systemFacetToc');`,
		[][2]string{
			{"the brief alone is shown", briefOnlyShown},
			{"its row is the current one", `row.classList.contains('on')`},
			{"the Briefs group is open", `briefsGroup.open === true`},
			{"the Modules group is closed", `groups[0] !== briefsGroup && groups[0].open === false`},
			{"its folder is open", `row.closest('.brief-folder').open === true`},
			{"another folder is closed", `nav.querySelector('.brief-folder[data-folder="research"]').open === false`},
			// A feature is listed once, under Features (NIT-201), never in the tree.
			{"features/ is not in the tree", `!briefsGroup.querySelector('[data-folder="features"]') && !briefsGroup.querySelector('[data-target="#brief-features-split-a-bill"]') && !!nav.querySelector('.feature-nav [data-target="#brief-features-split-a-bill"]')`},
			{"the locked brief carries the padlock mark", `!!row.querySelector('.brief-mark[data-mark="locked"] .dx-icon')`},
			{"the static build carries the image", `img.getAttribute('src') === 'brief-assets/decisions/split.png' && img.naturalWidth === 40`},
			{"the title is the header's, not the body's", `sec.querySelector('.brief-title').textContent === 'Balances round to the cent, once' && !sec.querySelector('.brief-body h1')`},
			{"On this page lists the two sections", `!toc.hidden && toc.dataset.kind === 'brief' && Array.prototype.map.call(toc.querySelectorAll('.facet-toc__item strong'), function (s) { return s.textContent; }).join('|') === 'Context|Decision'`},
			// NIT-198: a static build has nothing to write to, and this
			// brief has no thread to read, so its Comment is disabled.
			{"the Threads block's Comment is inert", `toc.querySelector('.facet-toc__comment').disabled === true`},
			{"the hash names the brief", `location.hash === '#` + roundBrief + `'`},
			// F8: the group holding the current row reads as current.
			{"the Briefs heading is in the accent at 600", `getComputedStyle(briefsGroup.querySelector(':scope > summary')).color === getComputedStyle(row).color && getComputedStyle(briefsGroup.querySelector(':scope > summary')).fontWeight === '600'`},
			{"the Briefs count is in the accent", `getComputedStyle(briefsGroup.querySelector('.system-nav-group__count')).color === getComputedStyle(row).color`},
			{"the Modules heading is not", `getComputedStyle(groups[0].querySelector(':scope > summary')).fontWeight !== '600'`},
			// F7 / F16: every mark is an 8px circle, in the tree and the legend.
			// A mark's box is as tall as its dot, and a legend mark as wide, so
			// the rects measure the dot's outer size, border included.
			{"the draft mark is 8px tall", `nav.querySelector('.brief-mark[data-mark="draft"]').getBoundingClientRect().height === 8`},
			{"every legend mark is 8 by 8", `Array.prototype.every.call(document.querySelectorAll('.brief-legend .brief-mark'), function (m) { var r = m.getBoundingClientRect(); return r.width === 8 && r.height === 8; })`},
			// F16: the legend sits above the theme switch, in the footer.
			{"the legend is in the footer above the theme switch", `(function(){ var l = document.querySelector('.sidebar-footer > .brief-legend'); var c = document.querySelector('.sidebar-footer > .theme-control'); return !!l && !!c && l.nextElementSibling === c && l.getBoundingClientRect().bottom <= c.getBoundingClientRect().top; })()`},
			// F13: B1's rail geometry and row hairline.
			{"Threads sits 20px under the last row", `(function(){ var items = toc.querySelectorAll('.facet-toc__item'); var th = toc.querySelector('.facet-toc__threads'); return Math.round(th.getBoundingClientRect().top - items[items.length - 1].getBoundingClientRect().bottom) === 20; })()`},
			{"the first row sits 4px under the kicker's band", `Math.round(toc.querySelector('.facet-toc__item').getBoundingClientRect().top - toc.querySelector('.facet-toc__head').getBoundingClientRect().bottom) === 4`},
			// F6: the inert Comment reads as disabled and says why (NIT-198's
			// words: a static build is read only).
			{"the rail's Comment is drawn disabled", `(function(){ var c = toc.querySelector('.facet-toc__comment'); return c.disabled && getComputedStyle(c).color !== getComputedStyle(document.querySelector('.brief-title')).color; })()`},
			{"a visible line says why", `toc.querySelector('.facet-toc__threads-later').textContent === 'Read only: comments are written through dossierx serve.' && toc.querySelector('.facet-toc__threads-later').getBoundingClientRect().height > 0`},
		})

	// F12: a brief with no section heading has no rail; its Comment row
	// comes to the page foot instead.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#brief-decisions-no-bank-linking';`, nil))
	pollTrue(t, ctx, `!document.getElementById('brief-decisions-no-bank-linking').hidden`)
	requireAll(t, ctx, "a brief with no section heading",
		`var sec = document.getElementById('brief-decisions-no-bank-linking');`,
		[][2]string{
			{"no On this page rail", `document.getElementById('systemFacetToc').hidden === true`},
			{"no trigger", `!sec.querySelector('.facet-toc-trigger')`},
			{"the Comment row is at the page foot", `sec.querySelector('.brief-comment-row').getBoundingClientRect().height > 0`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#`+roundBrief+`';`, nil))
	pollTrue(t, ctx, `!document.getElementById('systemFacetToc').hidden`)

	// Between 861 and 1180px the panel is the corner popover every facet
	// gets: a select over the headings, and no Threads block.
	runCDP(t, ctx, chromedp.EmulateViewport(1024, 900))
	requireAll(t, ctx, "On this page at 1024px",
		`var toc = document.getElementById('systemFacetToc');`,
		[][2]string{
			{"the select lists the headings", `getComputedStyle(toc.querySelector('.facet-toc__select')).display !== 'none' && toc.querySelector('.facet-toc__select').options.length === 2`},
			{"the Threads block is left out", `getComputedStyle(toc.querySelector('.facet-toc__threads')).display === 'none'`},
			// NIT-198: so the page-foot Comment row stands in for it.
			{"the page-foot Comment row stands in", `document.querySelector('#` + roundBrief + ` .brief-comment').getBoundingClientRect().height === 44`},
			{"the popover sits in the corner, not over the page", `toc.getBoundingClientRect().height < 200`},
		})
	desktopViewport(t, ctx)

	// A features/ brief is left out of the tree, but its page resolves.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#brief-features-split-a-bill';`, nil))
	pollTrue(t, ctx, `!document.getElementById('brief-features-split-a-bill').hidden`)

	// Clicking the Briefs entry only opens the group: the page stays Home.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#_home';`, nil))
	pollTrue(t, ctx, homeVisible)
	pollTrue(t, ctx, `document.querySelector('#nav .brief-nav').open === false`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .brief-nav > summary').click();`, nil))
	pollTrue(t, ctx, `document.querySelector('#nav .brief-nav').open === true`)
	if !evalBool(t, ctx, homeVisible+` && location.hash === '#_home'`) {
		t.Fatal("clicking the Briefs entry navigated; it must only open the group")
	}

	// "All briefs" opens the index; a brief there opens its page.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .brief-nav__all').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('_briefs').hidden && location.hash === '#_briefs'`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#_briefs a[href="#`+roundBrief+`"]').click();`, nil))
	pollTrue(t, ctx, briefOnlyShown)
}

func TestBriefPage_LightAndDark(t *testing.T) {
	p := briefProject(t)
	url := p.renderStatic()
	ctx := browserContext(t)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+roundBrief), chromedp.WaitVisible("#"+roundBrief, chromedp.ByQuery))
	suppressTransitions(t, ctx, "")

	for _, tc := range []struct{ theme, card, ink, locked, hairline string }{
		{"light", "rgb(255, 255, 255)", "rgb(16, 23, 32)", "rgb(44, 107, 82)", "rgb(238, 241, 245)"},
		{"dark", "rgb(22, 27, 34)", "rgb(230, 234, 240)", "rgb(99, 190, 154)", "rgb(30, 37, 48)"},
	} {
		evalVoid(t, ctx, `document.querySelector('.theme-control [data-theme-choice="`+tc.theme+`"]').click()`)
		requireAll(t, ctx, "the brief page in "+tc.theme,
			`var sec = document.getElementById('`+roundBrief+`');
			 var body = getComputedStyle(sec.querySelector('.brief-body'));
			 var pill = getComputedStyle(sec.querySelector('.brief-title-row .brief-pill'));`,
			[][2]string{
				{"data-theme is " + tc.theme, `document.documentElement.getAttribute('data-theme') === '` + tc.theme + `'`},
				{"the body card is the card colour", `body.backgroundColor === '` + tc.card + `'`},
				{"the body text is ink", `body.color === '` + tc.ink + `'`},
				{"the locked pill is the locked colour", `pill.color === '` + tc.locked + `'`},
				// F13: B1's hairline between relation rows, in each theme.
				{"a relation row's hairline is B1's", `getComputedStyle(sec.querySelector('.brief-relation')).borderBottomColor === '` + tc.hairline + `'`},
			})
	}
}

func TestBriefPage_PhoneDrawerAndOnThisPageSheet(t *testing.T) {
	p := briefProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844), chromedp.Navigate(url), chromedp.WaitVisible("#_home", chromedp.ByQuery))

	// The tree lives in the menu drawer.
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('navToggle').click();`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('nav-open')`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .brief-nav > summary').click();`, nil))
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .brief-folder[data-folder="decisions"] > summary').click();`, nil))
	pollTrue(t, ctx, `(function(){ var r = document.querySelector('#nav .sec-tab[data-target="#`+roundBrief+`"]').getBoundingClientRect(); return r.width > 0 && r.right <= 390; })()`)
	requireAll(t, ctx, "tap targets in the drawer's tree", ``, [][2]string{
		{"All briefs is 44px tall", `document.querySelector('#nav .brief-nav__all').getBoundingClientRect().height >= 44`},
		{"a folder row is 44px tall", `document.querySelector('#nav .brief-folder__toggle').getBoundingClientRect().height >= 44`},
		{"a brief row is 44px tall", `document.querySelector('#nav .brief-nav__row').getBoundingClientRect().height >= 44`},
	})
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .sec-tab[data-target="#`+roundBrief+`"]').click();`, nil))
	pollTrue(t, ctx, briefOnlyShown+` && !document.body.classList.contains('nav-open')`)

	// "On this page" is a sheet opened from the header.
	requireAll(t, ctx, "the brief page on a phone",
		`var sec = document.getElementById('`+roundBrief+`');
		 var trigger = sec.querySelector('.brief-toc-slot .facet-toc-trigger');`,
		[][2]string{
			{"the trigger sits in the header and is visible", `!!trigger && trigger.getBoundingClientRect().width > 0`},
			{"the trigger says On this page", `trigger.textContent.indexOf('On this page') === 0`},
			{"the path is left out of the phone kicker", `getComputedStyle(sec.querySelector('.brief-kicker__path')).display === 'none'`},
			{"the page-foot Comment button shows", `sec.querySelector('.brief-comment').getBoundingClientRect().height === 44`},
			{"it is disabled and says why", `sec.querySelector('.brief-comment').disabled && sec.querySelector('.brief-comment-note').getBoundingClientRect().height > 0`},
			{"nothing is wider than the phone", `document.documentElement.scrollWidth <= 390`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#`+roundBrief+` .brief-toc-slot .facet-toc-trigger').click();`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open')`)
	requireAll(t, ctx, "the On this page sheet",
		`var toc = document.getElementById('systemFacetToc');`,
		[][2]string{
			{"the sheet is on screen", `toc.getBoundingClientRect().top < 844 && toc.getBoundingClientRect().height > 0`},
			{"it lists the brief's sections", `toc.querySelectorAll('.facet-toc__item').length === 2`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelectorAll('#systemFacetToc .facet-toc__item')[1].click();`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('facet-toc-open')`)
}

func TestBriefSearch_MatchesASummaryAndOpensThePage(t *testing.T) {
	p := briefProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url), chromedp.WaitVisible("#_home", chromedp.ByQuery))

	if got := evalString(t, ctx, `document.getElementById('navSearch').placeholder`); got != "Search claims and briefs" {
		t.Fatalf("search placeholder = %q", got)
	}
	// "twelve households" is in a summary only: not the title, not the row.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){ var s = document.getElementById('navSearch'); s.value = 'twelve households'; s.dispatchEvent(new Event('input', {bubbles: true})); })()`, nil))
	requireAll(t, ctx, "a search that matches one brief's summary",
		`var nav = document.getElementById('nav');
		 var hit = nav.querySelector('.sec-tab[data-target="#brief-research-interviews"]');
		 var research = nav.querySelector('.brief-folder[data-folder="research"]');
		 var decisions = nav.querySelector('.brief-folder[data-folder="decisions"]');`,
		[][2]string{
			{"the Briefs group is open", `nav.querySelector('.brief-nav').open === true`},
			{"the hit is visible", `!hit.hidden && hit.getBoundingClientRect().height > 0`},
			{"its folder is open", `research.open === true && !research.hidden`},
			{"a folder with no hit is hidden", `decisions.hidden === true`},
			{"a non-matching brief is hidden", `nav.querySelector('.sec-tab[data-target="#` + roundBrief + `"]').hidden === true`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .sec-tab[data-target="#brief-research-interviews"]').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('brief-research-interviews').hidden && location.hash === '#brief-research-interviews'`)

	// A folder name finds every brief in it.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){ var s = document.getElementById('navSearch'); s.value = 'decisions'; s.dispatchEvent(new Event('input', {bubbles: true})); })()`, nil))
	pollTrue(t, ctx, `document.querySelectorAll('#nav .brief-folder[data-folder="decisions"] .sec-tab:not([hidden])').length === 2`)
}

// A check finding about a brief carries the brief's path as its claim_id
// (internal/briefs). It shows in the strip on that brief's page, with the
// folder's and the tree's, and nowhere else; the Issues screen leads back to
// the brief. The findings are painted through the strip's own entry point, as
// the static viewer has no /api/status to poll.
func TestBriefFindings_ShowOnTheirBriefPage(t *testing.T) {
	p := briefProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+roundBrief), chromedp.WaitVisible("#"+roundBrief, chromedp.ByQuery))

	paint := `window.dossierxRenderStatusStrip({
		readiness: {},
		lint_errors: [
			{lint: 'brief-word-cap', claim_id: 'briefs/decisions/round-to-the-cent.md', severity: 'error', message: 'over the cap'},
			{lint: 'brief-folder-cap', claim_id: 'briefs/decisions/', severity: 'error', message: 'too many'},
			{lint: 'brief-frontmatter', claim_id: 'briefs/research/interviews.md', severity: 'error', message: 'another brief'}
		]
	})`
	evalVoid(t, ctx, paint)
	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)
	requireAll(t, ctx, "brief findings on the brief's page",
		`var strip = document.getElementById('statusStrip');
		 var rules = Array.prototype.map.call(document.querySelectorAll('#statusStripBody .status-finding-rule'), function (n) { return n.textContent; }).sort().join('|');`,
		[][2]string{
			{"the strip sits under the brief's header", `document.querySelector('#` + roundBrief + ` > .brief-head').nextElementSibling === strip`},
			{"it counts the two findings about this brief and its folder", `strip.textContent.indexOf('2 issues on this brief need attention') >= 0`},
			{"another brief's finding is not here", `rules === 'Brief Folder Cap|Brief Word Cap'`},
			{"the brief's own row leads to the brief", `Array.prototype.some.call(document.querySelectorAll('#statusStripBody .status-finding-action'), function (b) { return b.textContent === 'Open brief'; })`},
		})

	// From the Issues screen, "Open brief" returns to the brief. The screen
	// speaks of the brief, not of a facet's claims (F5).
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('statusStripToggle').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('issuesView').hidden`)
	requireAll(t, ctx, "the Issues screen on a brief",
		`var view = document.getElementById('issuesView');`,
		[][2]string{
			{"its sentence is about the brief", `document.getElementById('issuesSubtitle').textContent === '2 findings on this brief need attention.'`},
			{"its scope says This brief", `view.querySelector('[data-scope="facet"]').textContent === 'This brief'`},
			{"its group counts findings on this brief", `Array.prototype.some.call(view.querySelectorAll('.status-group-head-note'), function (n) { return n.textContent === '2 findings on this brief'; })`},
			{"no claim noun anywhere on the screen", `!/\bclaims?\b/.test(view.querySelector('.issues-body').textContent + document.getElementById('issuesSubtitle').textContent + document.getElementById('issuesSortValue').textContent)`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`Array.prototype.filter.call(document.querySelectorAll('#issuesView .status-finding-action'), function (b) { return b.textContent === 'Open brief'; })[0].click();`, nil))
	pollTrue(t, ctx, `document.getElementById('issuesView').hidden && `+briefOnlyShown)

	// On a module page the same findings are not the module's.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#widget';`, nil))
	pollTrue(t, ctx, `!document.getElementById('widget').hidden`)
	evalVoid(t, ctx, paint)
	pollTrue(t, ctx, `document.getElementById('statusStrip').hidden`)
}

// F1: the search box promises claims, so a claim's title, summary or id finds
// the module that holds it; clicking the row opens that module, as a module
// search always has.
func TestSearch_FindsAClaimByItsTitleSummaryOrID(t *testing.T) {
	p := briefProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url), chromedp.WaitVisible("#_home", chromedp.ByQuery))

	search := func(q string) {
		evalVoid(t, ctx, `(function(){ var s = document.getElementById('navSearch'); s.value = `+jsQuote(q)+`; s.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	}
	for _, q := range []string{"overview", "engine test corpus", testClaimID} {
		search(q)
		requireAll(t, ctx, "a search for the claim's "+q,
			`var modules = document.querySelector('#nav .system-nav-group');
			 var row = modules.querySelector('.sec-tab[data-target="#widget"]');`,
			[][2]string{
				{"the Modules group is open", `modules.open === true && !modules.hidden`},
				{"the claim's module is visible", `!row.hidden && row.getBoundingClientRect().height > 0`},
			})
	}
	search("no such words anywhere")
	pollTrue(t, ctx, `document.querySelector('#nav .system-nav-group').hidden === true`)
	search("overview")
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .sec-tab[data-target="#widget"]').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('widget').hidden && !!document.getElementById('`+testClaimID+`')`)
	// F8: the shared current-group rule holds for Modules too.
	if !evalBool(t, ctx, `(function(){ var s = document.querySelector('#nav .system-nav-group > summary'); var on = document.querySelector('#nav .sec-tab.on'); return getComputedStyle(s).fontWeight === '600' && getComputedStyle(s).color === getComputedStyle(on).color; })()`) {
		t.Fatal("on a module page the Modules heading must read as current: accent, weight 600")
	}
}

// F16: the tree works from the keyboard alone: Tab reaches the Briefs
// heading and Enter opens it, Tab reaches a folder and Enter opens it, Tab
// reaches a brief and Enter opens its page.
func TestBriefTree_KeyboardPath(t *testing.T) {
	p := briefProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url), chromedp.WaitVisible("#_home", chromedp.ByQuery))

	tabTo := func(what, test string) {
		t.Helper()
		for i := 0; i < 40; i++ {
			if evalBool(t, ctx, `(function(){ var a = document.activeElement; return !!a && (`+test+`); })()`) {
				return
			}
			runCDP(t, ctx, chromedp.KeyEvent(kb.Tab))
		}
		t.Fatalf("Tab never reached %s", what)
	}
	runCDP(t, ctx, chromedp.Focus("#navSearch", chromedp.ByQuery))
	tabTo("the Briefs heading", `a.matches('.brief-nav > summary')`)
	runCDP(t, ctx, chromedp.KeyEvent(kb.Enter))
	pollTrue(t, ctx, `document.querySelector('#nav .brief-nav').open === true`)
	tabTo("the Decisions folder", `a.matches('.brief-folder[data-folder="decisions"] > summary')`)
	runCDP(t, ctx, chromedp.KeyEvent(kb.Enter))
	pollTrue(t, ctx, `document.querySelector('#nav .brief-folder[data-folder="decisions"]').open === true`)
	tabTo("the brief's row", `a.matches('.sec-tab[data-target="#`+roundBrief+`"]')`)
	runCDP(t, ctx, chromedp.KeyEvent(kb.Enter))
	pollTrue(t, ctx, briefOnlyShown)
}

// F4: under serve, the first brief added to a project that had none brings
// the Briefs group in with its icon drawn. The live reload swaps <nav> and
// <main> only, so an icon that pointed into the page's sprite (which sits
// outside both) would come in blank.
func TestBriefTree_IconSurvivesALiveReload(t *testing.T) {
	p := newProject(t)
	ctx := newLiveTab(t, p)
	path := filepath.Join(p.dir, "briefs", "flow", "overview.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nsummary: The flow.\n---\n## Why\n\nBecause.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pollTrue(t, ctx, `!!document.querySelector('#nav .brief-nav')`)
	requireAll(t, ctx, "the Briefs icon after a live reload",
		`var icon = document.querySelector('#nav .brief-nav > summary .site-nav__icon');`,
		[][2]string{
			{"the icon draws its own paths", `!!icon && icon.querySelectorAll('path').length === 3 && !icon.querySelector('use')`},
			{"it has a size", `icon.getBoundingClientRect().width === 16`},
		})
}
