package viewertests

import (
	"testing"

	"github.com/chromedp/chromedp"
)

const secondClaimYAML = `id: widget.contract.secondary
facet: contract
module: widget
status: draft
body: |
  another claim under review.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const statusTwoFacetConfigYAML = `schema_version: 1
facets:
  - contract
  - behavior
  - schema
modules:
  - widget
  - other
claims_dir: claims
`

const contractIssueClaimYAML = `id: widget.contract.issue
facet: contract
module: widget
status: draft
body: |
  the contract facet has one review issue.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - widget.contract.missing
`

const behaviorIssueClaimYAML = `id: widget.behavior.issue
facet: behavior
module: widget
status: draft
body: |
  the behavior facet has a different review issue.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - widget.behavior.missing
`

const cleanOtherModuleClaimYAML = `id: other.contract.clean
facet: contract
module: other
status: draft
body: |
  the other module has no review issue.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const cleanWidgetFacetClaimYAML = `id: widget.schema.clean
facet: schema
module: widget
status: draft
body: |
  the widget schema facet has no review issue.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

func TestIndividualClaimCanCollapseAndExpand(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "keep comments reachable")
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-collapse-toggle", chromedp.ByQuery),
	)

	if !evalBool(t, ctx, `document.querySelector('.claim-collapse-toggle').getAttribute('aria-expanded') === 'true'`) {
		t.Fatal("claim disclosure must start expanded")
	}

	runCDP(t, ctx, chromedp.Click(".claim-collapse-toggle", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.claim-collapse-content').hidden`)
	if !evalBool(t, ctx, `document.querySelector('.claim-collapse-toggle').getAttribute('aria-expanded') === 'false'`) {
		t.Fatal("collapsed claim must expose aria-expanded=false")
	}
	if !evalBool(t, ctx, `!!document.querySelector('.claim--collapsed .comment-chip')`) {
		t.Fatal("collapsing a claim must leave its comment control reachable")
	}
	if !evalBool(t, ctx, `(function(){
		var claim = document.querySelector('.claim');
		var head = claim.querySelector(':scope > .k');
		var arrow = head.querySelector('.claim-collapse-chevron').getBoundingClientRect();
		var chip = head.querySelector('.comment-chip').getBoundingClientRect();
		var edge = head.getBoundingClientRect().right;
		return arrow.left > chip.right && Math.abs(edge - arrow.right) <= 9;
	})()`) {
		t.Fatal("claim disclosure arrow must stay at the extreme right, after comments")
	}

	runCDP(t, ctx, chromedp.Click(".comment-chip", chromedp.ByQuery))
	pollTrue(t, ctx, `!document.getElementById('commentsPanel').hidden`)
	if !evalBool(t, ctx, `document.querySelector('.claim-collapse-content').hidden`) {
		t.Fatal("opening comments must not unexpectedly expand the claim")
	}

	runCDP(t, ctx,
		chromedp.Click("#commentsRailClose", chromedp.ByQuery),
		chromedp.Click(".claim-collapse-toggle", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!document.querySelector('.claim-collapse-content').hidden`)
}

func TestClaimDeepLinkRevealsCollapsedContent(t *testing.T) {
	p := newProject(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-collapse-toggle", chromedp.ByQuery),
		chromedp.Click(".claim-collapse-toggle", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.querySelector('.claim-collapse-content').hidden`)

	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#widget.contract.overview'`, nil))
	pollTrue(t, ctx, `document.querySelector('.claim-collapse-toggle').getAttribute('aria-expanded') === 'true'`)
}

func TestFacetClaimsCanCollapseAndExpandTogether(t *testing.T) {
	p := newProject(t)
	p.writeClaim("secondary.yaml", secondClaimYAML)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".facet-claims-toggle", chromedp.ByQuery),
	)

	if !evalBool(t, ctx, `document.querySelectorAll('.claim-collapse-toggle').length === 2 && Array.from(document.querySelectorAll('.claim-collapse-toggle')).every(function(toggle){ return toggle.getAttribute('aria-expanded') === 'true'; })`) {
		t.Fatal("all claims in the active facet must start expanded")
	}

	runCDP(t, ctx, chromedp.Click(".facet-claims-toggle", chromedp.ByQuery))
	pollTrue(t, ctx, `Array.from(document.querySelectorAll('.claim-collapse-content')).every(function(content){ return content.hidden; })`)
	if !evalBool(t, ctx, `(function(){ var toggle = document.querySelector('.facet-claims-toggle'); return toggle.getAttribute('aria-pressed') === 'true' && toggle.textContent.trim() === 'Expand all claims'; })()`) {
		t.Fatal("bulk control must announce that all claims are collapsed and offer expansion")
	}

	runCDP(t, ctx, chromedp.Click(".facet-claims-toggle", chromedp.ByQuery))
	pollTrue(t, ctx, `Array.from(document.querySelectorAll('.claim-collapse-content')).every(function(content){ return !content.hidden; })`)
	if !evalBool(t, ctx, `document.querySelector('.facet-claims-toggle').getAttribute('aria-pressed') === 'false'`) {
		t.Fatal("bulk control must return to its expanded state")
	}
}

func TestDesktopNavigationPanelsCanCollapseAndExpand(t *testing.T) {
	p := newProject(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible("#sidebarCollapseToggle", chromedp.ByQuery),
		chromedp.WaitVisible(".system-panel-toggle--toc", chromedp.ByQuery),
	)

	runCDP(t, ctx, chromedp.Click("#sidebarCollapseToggle", chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('system-sidebar-collapsed')`)
	if !evalBool(t, ctx, `(function(){ var sidebar = document.getElementById('sidebar'); var toggle = document.getElementById('sidebarCollapseToggle'); return Math.round(sidebar.getBoundingClientRect().width) === 44 && toggle.getAttribute('aria-expanded') === 'false' && toggle.getAttribute('aria-label') === 'Show navigation'; })()`) {
		t.Fatal("left navigation must collapse to a reachable 44px rail")
	}

	runCDP(t, ctx, chromedp.Click("#sidebarCollapseToggle", chromedp.ByQuery))
	pollTrue(t, ctx, `!document.body.classList.contains('system-sidebar-collapsed')`)
	if !evalBool(t, ctx, `document.getElementById('sidebar').getBoundingClientRect().width >= 220`) {
		t.Fatal("left navigation must expand to its reading width")
	}

	runCDP(t, ctx, chromedp.Click(".system-panel-toggle--toc", chromedp.ByQuery))
	pollTrue(t, ctx, `Math.round(document.getElementById('systemFacetToc').getBoundingClientRect().width) === 44`)
	if !evalBool(t, ctx, `(function(){ var toc = document.getElementById('systemFacetToc'); var toggle = toc.querySelector('.system-panel-toggle--toc'); return Math.round(toc.getBoundingClientRect().width) === 44 && toggle.getAttribute('aria-expanded') === 'false' && toggle.getAttribute('aria-label') === 'Show table of contents'; })()`) {
		t.Fatal("right table of contents must collapse to a reachable 44px rail")
	}

	runCDP(t, ctx, chromedp.Click(".system-panel-toggle--toc", chromedp.ByQuery))
	pollTrue(t, ctx, `document.getElementById('systemFacetToc').getBoundingClientRect().width >= 220`)
	if !evalBool(t, ctx, `document.getElementById('systemFacetToc').getBoundingClientRect().width >= 220`) {
		t.Fatal("right table of contents must expand to its reading width")
	}
}

func TestStatusStripShowsOnlyActiveFacetIssues(t *testing.T) {
	p := newProjectRaw(t, statusTwoFacetConfigYAML)
	p.writeClaim("contract.yaml", contractIssueClaimYAML)
	p.writeClaim("behavior.yaml", behaviorIssueClaimYAML)
	p.writeClaim("other.yaml", cleanOtherModuleClaimYAML)
	p.writeClaim("schema.yaml", cleanWidgetFacetClaimYAML)
	ctx := newLiveTab(t, p)

	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)
	if !evalBool(t, ctx, `(function(){
		var strip = document.getElementById('statusStrip');
		return strip.querySelector('#statusStripTitle').textContent === '1 issue in this facet needs attention' &&
			strip.textContent.indexOf('widget.contract.issue') >= 0 &&
			strip.textContent.indexOf('widget.behavior.issue') < 0;
	})()`) {
		t.Fatal("contract facet must show only its own issue")
	}

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.sec-tab[data-target="#other"]').click()`, nil))
	pollTrue(t, ctx, `document.querySelector('.module-section:not([hidden])').id === 'other'`)
	if !evalBool(t, ctx, `document.getElementById('statusStrip').hidden`) {
		t.Fatal("the issues component must be absent from an unaffected module")
	}

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.sec-tab[data-target="#widget"]').click()`, nil))
	pollTrue(t, ctx, `document.querySelector('.module-section:not([hidden])').id === 'widget'`)
	runCDP(t, ctx, chromedp.Click(`[data-target="#widget-schema"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.module-section:not([hidden]) > .claim-group:not([hidden])').id === 'widget-schema'`)
	if !evalBool(t, ctx, `document.getElementById('statusStrip').hidden`) {
		t.Fatal("the issues component must be absent from an unaffected facet in the affected module")
	}

	runCDP(t, ctx, chromedp.Click(`[data-target="#widget-behavior"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.module-section:not([hidden]) > .claim-group:not([hidden])').id === 'widget-behavior'`)
	if !evalBool(t, ctx, `(function(){
		var strip = document.getElementById('statusStrip');
		return !strip.hidden &&
			strip.querySelector('#statusStripTitle').textContent === '1 issue in this facet needs attention' &&
			strip.textContent.indexOf('widget.behavior.issue') >= 0 &&
			strip.textContent.indexOf('widget.contract.issue') < 0;
	})()`) {
		t.Fatal("behavior facet must replace the list with only its own issue")
	}
}
