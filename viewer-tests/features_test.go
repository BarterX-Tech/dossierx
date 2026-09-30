package viewertests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// Features (NIT-201, Paper B4), driven in a real browser against a static
// build: the Features entry and its open state, the feature page's Made of
// list and "On this page" row, a body link to another brief and a brief named
// by its path in the hash, Home's Features tile, light and dark, the phone
// drawer, the duplicate warning in the status strip, and the entry arriving
// with a live reload. The markup itself is pinned at its owner,
// internal/render (TestRender_FeaturePage); what is asserted here is what
// only a browser can show.

const (
	splitFeature  = "brief-features-split-a-bill"
	exportFeature = "brief-features-export-to-csv"
	moneyBrief    = "brief-voice-talking-about-money"
)

// featureProject: two modules; a locked feature resting on three claims in
// both, two of them locked; a draft feature resting on one; and a brief in
// voice/ the locked feature's body links to.
func featureProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, railConfig)
	p.writeClaim("overview.yaml", railClaim("widget.contract.overview", "contract", "widget", ""))
	p.writeClaim("rounding.yaml", railClaim("widget.contract.rounding", "contract", "widget", ""))
	p.writeClaim("core.yaml", railClaim("gadget.contract.core", "contract", "gadget", ""))
	p.run("claim", "lock", "widget.contract.overview", "--reason", "viewer-test fixture")
	p.run("claim", "lock", "gadget.contract.core", "--reason", "viewer-test fixture")
	writeBrief(t, p, "features/split-a-bill.md", "---\nsummary: Anyone in a group adds what they paid.\nstatus: locked\nrests_on:\n  - widget.contract.overview\n  - gadget.contract.core\n  - widget.contract.rounding\n---\n# Split a bill\n\n## What it is\n\nWording follows [Talking about money](../voice/talking-about-money.md).\n\n## How it works\n\nShares default to equal.\n")
	writeBrief(t, p, "features/export-to-csv.md", "---\nsummary: Every expense as a CSV.\nrests_on:\n  - gadget.contract.core\n---\n# Export to CSV\n\n## What it is\n\nOne file.\n")
	writeBrief(t, p, "voice/talking-about-money.md", "---\nsummary: How Tally says it.\n---\n# Talking about money\n\n## Words\n\nOwe, not debt.\n")
	return p
}

func writeBrief(t *testing.T, p *project, rel, body string) {
	t.Helper()
	path := filepath.Join(p.dir, "briefs", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// onlyShown is true when the section with id is the one page on screen.
func onlyShown(id string) string {
	return `(function(){
  var shown = Array.prototype.filter.call(document.querySelectorAll('.module-section'), function (s) { return !s.hidden; });
  return shown.length === 1 && shown[0].id === '` + id + `';
})()`
}

func TestFeaturePage_FromTheFeaturesEntry(t *testing.T) {
	p := featureProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url), chromedp.WaitVisible("#_home", chromedp.ByQuery))

	// On Home the entry is closed; clicking it only opens it.
	pollTrue(t, ctx, `document.querySelector('#nav .feature-nav').open === false`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .feature-nav > summary').click();`, nil))
	pollTrue(t, ctx, `document.querySelector('#nav .feature-nav').open === true && location.hash !== '#`+splitFeature+`'`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .feature-nav .sec-tab[data-target="#`+splitFeature+`"]').click();`, nil))
	pollTrue(t, ctx, onlyShown(splitFeature))

	requireAll(t, ctx, "a feature opened from the Features entry",
		`var sec = document.getElementById('`+splitFeature+`');
		 var nav = document.getElementById('nav');
		 var group = nav.querySelector('.feature-nav');
		 var rows = Array.prototype.slice.call(group.querySelectorAll('.sec-tab'));
		 var row = group.querySelector('.sec-tab[data-target="#`+splitFeature+`"]');
		 var madeOf = sec.querySelector('.feature-made-of');
		 var modules = Array.prototype.map.call(madeOf.querySelectorAll('.feature-made-of__module'), function (m) { return m.textContent; });
		 var lists = madeOf.querySelectorAll('.brief-relations__list');
		 var toc = document.getElementById('systemFacetToc');
		 var tocRows = Array.prototype.map.call(toc.querySelectorAll('.facet-toc__item strong'), function (s) { return s.textContent; });`,
		[][2]string{
			{"the rows are the features in file-name order, by title", `rows.map(function (r) { return r.textContent.trim(); }).join('|') === 'Export to CSV|Split a bill'`},
			{"its row is the current one", `row.classList.contains('on')`},
			{"the Features entry is open and reads as current", `group.open === true && getComputedStyle(group.querySelector(':scope > summary')).fontWeight === '600'`},
			{"the Briefs group is closed and lists no feature", `nav.querySelector('.brief-nav').open === false && !nav.querySelector('.brief-nav [data-target^="#brief-features-"]')`},
			{"the locked feature carries the padlock", `!!row.querySelector('.brief-mark[data-mark="locked"] .dx-icon')`},
			{"the draft feature carries the hollow dot", `!!group.querySelector('.sec-tab[data-target="#` + exportFeature + `"] .brief-mark[data-mark="draft"]')`},
			{"the kicker says Feature", `sec.querySelector('.brief-kicker').textContent.indexOf('Feature · briefs/features/split-a-bill.md') === 0`},
			{"the meta counts what it rests on", `sec.querySelector('.brief-meta .brief-wide').textContent.indexOf('Rests on 3 claims in Widget and Gadget, 2 of 3 locked · ') === 0`},
			{"Made of heads its card", `madeOf.querySelector('.brief-relations__label').textContent.indexOf('Made of · 3 claims in 2 modules') === 0 && madeOf.querySelector('.brief-relations__note').textContent === 'rests_on, not an order'`},
			{"Made of groups by module in rests_on order", `modules.join('|') === 'Widget|Gadget'`},
			{"each module's rows are in rests_on order", `Array.prototype.map.call(lists[0].querySelectorAll('.claim-ref'), function (a) { return a.getAttribute('href'); }).join('|') === '#widget.contract.overview|#widget.contract.rounding' && lists[1].querySelectorAll('.claim-ref').length === 1`},
			{"a row carries its lock badge", `lists[0].querySelector('.claim-relationship-badge--locked').textContent === 'LOCKED' && lists[0].querySelector('.claim-relationship-badge--draft').textContent === 'DRAFT'`},
			{"there is no Rests on list", `sec.textContent.indexOf('Rests on · ') < 0`},
			{"On this page ends with Made of", `!toc.hidden && tocRows.join('|') === 'What it is|How it works|Made of · 3 claims'`},
		})

	// The rail's Made of row brings the list, below the fold on a short
	// window, into view.
	runCDP(t, ctx, chromedp.EmulateViewport(1280, 420))
	madeOfHead := `document.querySelector('#` + splitFeature + ` .feature-made-of__head').getBoundingClientRect()`
	pollTrue(t, ctx, madeOfHead+`.top > window.innerHeight`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelectorAll('#systemFacetToc .facet-toc__item')[2].click();`, nil))
	pollTrue(t, ctx, `(function(){ var r = `+madeOfHead+`; return r.top >= 0 && r.bottom <= window.innerHeight; })()`)
	desktopViewport(t, ctx)

	// A Made of row opens its claim.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#`+splitFeature+` .feature-made-of a.claim-ref[href="#gadget.contract.core"]').click();`, nil))
	pollTrue(t, ctx, `!document.getElementById('gadget').hidden && location.hash === '#gadget.contract.core'`)

	// A body link to another brief, written as a relative path, opens it.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#`+splitFeature+`';`, nil))
	pollTrue(t, ctx, onlyShown(splitFeature))
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#`+splitFeature+` .brief-body a').click();`, nil))
	pollTrue(t, ctx, onlyShown(moneyBrief)+` && location.hash === '#`+moneyBrief+`'`)

	// A brief named by its path in the hash opens its page.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#briefs/features/export-to-csv.md';`, nil))
	pollTrue(t, ctx, onlyShown(exportFeature)+` && document.querySelector('#nav .sec-tab[data-target="#`+exportFeature+`"]').classList.contains('on')`)
	if got := evalString(t, ctx, `document.querySelector('#`+exportFeature+` .brief-meta .brief-wide').textContent`); !strings.HasPrefix(got, "Rests on 1 claim in Gadget, all locked · ") {
		t.Fatalf("a feature whose every claim is locked reads all locked; meta = %q", got)
	}

	// Home's Features tile: a row opens its feature, the title the first.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#_home';`, nil))
	pollTrue(t, ctx, onlyShown("_home"))
	requireAll(t, ctx, "Home's Features tile",
		`var tile = document.querySelector('#_home .home-tile[data-tile="features"]');
		 var rows = tile.querySelectorAll('.home-feature');`,
		[][2]string{
			{"one row per feature", `rows.length === 2 && rows[0].textContent === 'Export to CSVdraft' && rows[1].textContent === 'Split a billlocked'`},
			{"the rows are on screen", `rows[1].getBoundingClientRect().height === 30`},
			{"the phone line is hidden", `getComputedStyle(tile.querySelector('.home-tile__line')).display === 'none'`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelectorAll('#_home .home-feature')[1].click();`, nil))
	pollTrue(t, ctx, onlyShown(splitFeature))
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#_home';`, nil))
	pollTrue(t, ctx, onlyShown("_home"))
	// A click on the tile's body, outside any row, opens the first feature.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){ var d = document.querySelector('#_home .home-tile[data-tile="features"] .home-tile__desc').getBoundingClientRect(); document.elementFromPoint(d.left + 4, d.top + 4).click(); })()`, nil))
	pollTrue(t, ctx, onlyShown(exportFeature))
}

func TestFeaturePage_LightAndDark(t *testing.T) {
	p := featureProject(t)
	url := p.renderStatic()
	ctx := browserContext(t)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+splitFeature), chromedp.WaitVisible("#"+splitFeature, chromedp.ByQuery))
	suppressTransitions(t, ctx, "")

	for _, tc := range []struct{ theme, card, faint, locked, link string }{
		{"light", "rgb(255, 255, 255)", "rgb(110, 124, 142)", "rgb(44, 107, 82)", "rgb(28, 78, 140)"},
		{"dark", "rgb(22, 27, 34)", "rgb(132, 148, 168)", "rgb(99, 190, 154)", "rgb(106, 166, 232)"},
	} {
		evalVoid(t, ctx, `document.querySelector('.theme-control [data-theme-choice="`+tc.theme+`"]').click()`)
		requireAll(t, ctx, "the feature page in "+tc.theme,
			`var sec = document.getElementById('`+splitFeature+`');
			 var madeOf = sec.querySelector('.feature-made-of');
			 var row = document.querySelector('#nav .feature-nav .sec-tab.on');`,
			[][2]string{
				{"data-theme is " + tc.theme, `document.documentElement.getAttribute('data-theme') === '` + tc.theme + `'`},
				{"the Made of card is the card colour", `getComputedStyle(madeOf).backgroundColor === '` + tc.card + `'`},
				{"a module label is faint", `getComputedStyle(madeOf.querySelector('.feature-made-of__module')).color === '` + tc.faint + `'`},
				{"a locked badge is the locked colour", `getComputedStyle(madeOf.querySelector('.claim-relationship-badge--locked')).color === '` + tc.locked + `'`},
				{"a claim link is the link colour", `getComputedStyle(madeOf.querySelector('.claim-ref')).color === '` + tc.link + `'`},
				{"the current feature row is the link colour", `getComputedStyle(row).color === '` + tc.link + `'`},
				{"the feature's padlock is the locked colour", `getComputedStyle(row.querySelector('.brief-mark .dx-icon')).color === '` + tc.locked + `'`},
			})
	}
}

func TestFeaturePage_PhoneDrawer(t *testing.T) {
	p := featureProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844), chromedp.Navigate(url), chromedp.WaitVisible("#_home", chromedp.ByQuery))

	// Home on a phone: the tile is one row counting the features.
	requireAll(t, ctx, "the Features tile on a phone",
		`var tile = document.querySelector('#_home .home-tile[data-tile="features"]');`,
		[][2]string{
			{"its line counts them", `tile.querySelector('.home-tile__line').textContent === '2 · 1 locked · 1 draft' && tile.querySelector('.home-tile__line').getBoundingClientRect().height > 0`},
			{"the rows are left to the page", `getComputedStyle(tile.querySelector('.home-features')).display === 'none'`},
		})

	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('navToggle').click();`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('nav-open')`)
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .feature-nav > summary').click();`, nil))
	pollTrue(t, ctx, `(function(){ var r = document.querySelector('#nav .feature-nav .sec-tab[data-target="#`+splitFeature+`"]').getBoundingClientRect(); return r.width > 0 && r.right <= 390; })()`)
	if !evalBool(t, ctx, `document.querySelector('#nav .feature-nav .brief-nav__row').getBoundingClientRect().height >= 44`) {
		t.Fatal("a feature row in the phone drawer must be a 44px tap target")
	}
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#nav .feature-nav .sec-tab[data-target="#`+splitFeature+`"]').click();`, nil))
	pollTrue(t, ctx, onlyShown(splitFeature)+` && !document.body.classList.contains('nav-open')`)

	requireAll(t, ctx, "the feature page on a phone",
		`var sec = document.getElementById('`+splitFeature+`');
		 var madeOf = sec.querySelector('.feature-made-of');`,
		[][2]string{
			{"the kicker is the word alone", `sec.querySelector('.brief-kicker').innerText.trim() === 'FEATURE'`},
			{"the meta is the short one", `sec.querySelector('.brief-meta .brief-narrow').textContent === '3 claims, 2 of 3 locked' && getComputedStyle(sec.querySelector('.brief-meta .brief-wide')).display === 'none'`},
			{"Made of has the short head and no caption", `madeOf.querySelector('.brief-relations__label').innerText.trim() === 'MADE OF · 3 CLAIMS' && getComputedStyle(madeOf.querySelector('.brief-relations__note')).display === 'none'`},
			{"the module labels show", `madeOf.querySelectorAll('.feature-made-of__module').length === 2 && madeOf.querySelector('.feature-made-of__module').getBoundingClientRect().height > 0`},
			{"the Comment button names the feature", `sec.querySelector('.brief-comment').textContent.trim() === 'Comment on this feature' && sec.querySelector('.brief-comment').getBoundingClientRect().height === 44`},
			{"nothing is wider than the phone", `document.documentElement.scrollWidth <= 390`},
		})
	// "On this page" is a sheet whose last row is Made of.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('#`+splitFeature+` .brief-toc-slot .facet-toc-trigger').click();`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open')`)
	if got := evalString(t, ctx, `Array.prototype.map.call(document.querySelectorAll('#systemFacetToc .facet-toc__item strong'), function (s) { return s.textContent; }).join('|')`); got != "What it is|How it works|Made of · 3 claims" {
		t.Fatalf("the phone's On this page sheet = %q", got)
	}
}

// brief-rests-on-duplicate is a warning whose claim_id is the brief's path
// and whose message names the other feature. On a feature page it shows in
// the status strip, naming both, and adds nothing else to the page. The
// findings are painted through the strip's own entry point, as the static
// viewer has no /api/status to poll.
func TestFeatureFindings_DuplicateWarningShowsInTheStrip(t *testing.T) {
	p := featureProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+splitFeature), chromedp.WaitVisible("#"+splitFeature, chromedp.ByQuery))

	paint := `window.dossierxRenderStatusStrip({
		readiness: {},
		lint_warnings: [
			{lint: 'brief-rests-on-duplicate', claim_id: 'briefs/features/split-a-bill.md', severity: 'warning', message: 'rests_on is exactly the same set as briefs/features/export-to-csv.md; merge them'},
			{lint: 'brief-rests-on-duplicate', claim_id: 'briefs/features/export-to-csv.md', severity: 'warning', message: 'rests_on is exactly the same set as briefs/features/split-a-bill.md; merge them'}
		]
	})`
	evalVoid(t, ctx, paint)
	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)
	requireAll(t, ctx, "the duplicate warning on the feature's page",
		`var strip = document.getElementById('statusStrip');
		 var rows = document.querySelectorAll('#statusStripBody .status-finding');`,
		[][2]string{
			{"the strip sits under the feature's header", `document.querySelector('#` + splitFeature + ` > .brief-head').nextElementSibling === strip`},
			{"one finding, this feature's", `rows.length === 1 && rows[0].querySelector('.status-finding-rule').textContent === 'Brief Rests On Duplicate'`},
			{"it names this feature", `rows[0].querySelector('.status-finding-claim').textContent === 'briefs/features/split-a-bill.md'`},
			{"and the other", `rows[0].querySelector('.status-finding-message').textContent.indexOf('briefs/features/export-to-csv.md') >= 0`},
			{"the page itself is unchanged", `!document.querySelector('#` + splitFeature + ` .feature-made-of [class*="duplicate"]')`},
		})

	// On a module page the same warnings are not the module's.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#widget';`, nil))
	pollTrue(t, ctx, `!document.getElementById('widget').hidden`)
	evalVoid(t, ctx, paint)
	pollTrue(t, ctx, `document.getElementById('statusStrip').hidden`)
}

// Under serve, the first feature added to a project that had none brings the
// Features entry and Home's tile in with their stars drawn: the live reload
// swaps <nav> and <main> only, so an icon pointing into the sprite would
// come in blank, and anything outside those two would not come in at all.
func TestFeatures_ArriveWithALiveReload(t *testing.T) {
	p := newProject(t)
	ctx := newLiveTab(t, p)
	writeBrief(t, p, "features/export-to-csv.md", "---\nsummary: Every expense as a CSV.\nrests_on:\n  - "+testClaimID+"\n---\n# Export to CSV\n\n## What it is\n\nOne file.\n")
	pollTrue(t, ctx, `!!document.querySelector('#nav .feature-nav') && !!document.querySelector('#_home .home-tile[data-tile="features"]')`)
	requireAll(t, ctx, "the Features entry after a live reload",
		`var icon = document.querySelector('#nav .feature-nav > summary .site-nav__icon');
		 var tileIcon = document.querySelector('#_home .home-tile[data-tile="features"] .home-tile__icon .dx-icon');`,
		[][2]string{
			{"the entry's star draws its own path", `!!icon && icon.querySelectorAll('path').length === 1 && !icon.querySelector('use') && icon.getBoundingClientRect().width === 16`},
			{"the tile's star draws its own path", `!!tileIcon && tileIcon.querySelectorAll('path').length === 1 && !tileIcon.querySelector('use')`},
			{"the feature's page arrived", `!!document.getElementById('` + exportFeature + `')`},
		})
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#`+exportFeature+`';`, nil))
	pollTrue(t, ctx, onlyShown(exportFeature)+` && document.querySelectorAll('#`+exportFeature+` .feature-made-of .claim-ref').length === 1`)
}
