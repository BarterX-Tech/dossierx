package viewertests

import (
	"context"
	"testing"

	"github.com/chromedp/chromedp"
)

// The System Record claim card owns the reading width, up to a fixed prose
// measure. Pre-revamp this asserted the body always filled the card's full
// available width, on the reasoning that a fixed measure would leave a wide
// card empty; the design revamp reverses that on purpose.
// docs/design/screens/07a-claim-draft-not-yet-approved.md §3/§9 (open
// decision 5: "the reading measure on this screen is 760px, and it is
// confirmed three times") and reference-rules.md R11.2 ("the measure never
// changes") both make 760px a hard cap on claim prose specifically so a wide
// screen does not stretch a paragraph's line length past a comfortable
// reading width. So the rule below is now "the card's available width, or
// 760px, whichever is narrower" — unchanged at the 600px viewport this test
// also checks, where the card's own content width is already well under
// 760px and the cap never binds.
func TestClaimBodyUsesAvailableCardWidth(t *testing.T) {
	p := newProject(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1600, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-body", chromedp.ByQuery),
	)
	assertClaimBodyUsesCardWidth(t, ctx, "desktop")

	runCDP(t, ctx, chromedp.EmulateViewport(600, 800))
	assertClaimBodyUsesCardWidth(t, ctx, "mobile")
}

func assertClaimBodyUsesCardWidth(t *testing.T, ctx context.Context, viewport string) {
	t.Helper()
	if !evalBool(t, ctx, `(function(){
		var body = document.querySelector('.claim-body');
		var card = body && body.closest('.card');
		if (!body || !card) return false;
		var style = getComputedStyle(card);
		var available = card.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
		var want = Math.min(available, 760);
		return Math.abs(body.getBoundingClientRect().width - want) <= 1;
	})()`) {
		t.Fatalf("%s claim body does not use min(card's available content width, the 760px reading measure)", viewport)
	}
}
