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
	write := func(rel string, data []byte) {
		path := filepath.Join(p.dir, "briefs", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("decisions/round-to-the-cent.md", []byte("---\nsummary: Every share is rounded exactly once, at display.\nstatus: locked\nrests_on:\n  - "+testClaimID+"\n---\n"+
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
			{"features/ is not in the tree", `!nav.querySelector('[data-folder="features"]') && !nav.querySelector('[data-target="#brief-features-split-a-bill"]')`},
			{"the locked brief carries the padlock mark", `!!row.querySelector('.brief-mark[data-mark="locked"] .dx-icon')`},
			{"the static build carries the image", `img.getAttribute('src') === 'brief-assets/decisions/split.png' && img.naturalWidth === 40`},
			{"the title is the header's, not the body's", `sec.querySelector('.brief-title').textContent === 'Balances round to the cent, once' && !sec.querySelector('.brief-body h1')`},
			{"On this page lists the two sections", `!toc.hidden && toc.dataset.kind === 'brief' && Array.prototype.map.call(toc.querySelectorAll('.facet-toc__item strong'), function (s) { return s.textContent; }).join('|') === 'Context|Decision'`},
			{"the Threads block's Comment is inert", `toc.querySelector('.facet-toc__comment').disabled === true`},
			{"the hash names the brief", `location.hash === '#` + roundBrief + `'`},
		})

	// Between 861 and 1180px the panel is the corner popover every facet
	// gets: a select over the headings, and no Threads block.
	runCDP(t, ctx, chromedp.EmulateViewport(1024, 900))
	requireAll(t, ctx, "On this page at 1024px",
		`var toc = document.getElementById('systemFacetToc');`,
		[][2]string{
			{"the select lists the headings", `getComputedStyle(toc.querySelector('.facet-toc__select')).display !== 'none' && toc.querySelector('.facet-toc__select').options.length === 2`},
			{"the Threads block is left out", `getComputedStyle(toc.querySelector('.facet-toc__threads')).display === 'none'`},
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

	for _, tc := range []struct{ theme, card, ink, locked string }{
		{"light", "rgb(255, 255, 255)", "rgb(16, 23, 32)", "rgb(44, 107, 82)"},
		{"dark", "rgb(22, 27, 34)", "rgb(230, 234, 240)", "rgb(99, 190, 154)"},
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

	// From the Issues screen, "Open brief" returns to the brief.
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('statusStripToggle').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('issuesView').hidden`)
	runCDP(t, ctx, chromedp.Evaluate(`Array.prototype.filter.call(document.querySelectorAll('#issuesView .status-finding-action'), function (b) { return b.textContent === 'Open brief'; })[0].click();`, nil))
	pollTrue(t, ctx, `document.getElementById('issuesView').hidden && `+briefOnlyShown)

	// On a module page the same findings are not the module's.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#widget';`, nil))
	pollTrue(t, ctx, `!document.getElementById('widget').hidden`)
	evalVoid(t, ctx, paint)
	pollTrue(t, ctx, `document.getElementById('statusStrip').hidden`)
}
