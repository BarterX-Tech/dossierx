package viewertests

import (
	"context"
	"testing"

	"github.com/chromedp/chromedp"
)

// The System Record claim card owns the reading width. A fixed prose measure
// leaves most of a wide card empty and makes long claims wrap into a narrow
// column, so the body must follow the card's available content width at both
// desktop and mobile sizes.
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
		return Math.abs(body.getBoundingClientRect().width - available) <= 1;
	})()`) {
		t.Fatalf("%s claim body does not use the card's available content width", viewport)
	}
}
