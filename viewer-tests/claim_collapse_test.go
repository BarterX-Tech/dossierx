package viewertests

import (
	"testing"

	"github.com/chromedp/chromedp"
)

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
