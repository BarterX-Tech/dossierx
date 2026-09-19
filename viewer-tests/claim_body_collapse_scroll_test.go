package viewertests

import (
	"fmt"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestCollapsingAClaimBodyKeepsTheCardInView pins the scroll response to the
// body disclosure's "less".
//
// Paper specifies nothing here — the design file carries fourteen "...more"
// nodes and no "less" node, and neither screen 02 nor 05 says what a collapse
// does to scroll position. This is the decided convention: a collapse that
// leaves the card's top edge above the deep-link line scrolls it back to that
// line, and a collapse that leaves it on screen scrolls nothing at all.
//
// The assertion is the card's TOP EDGE POSITION, not that scrollY changed.
// Before the fix the top edge was already perfectly stationary in document
// coordinates — it was just hundreds of pixels off the top of the screen, so
// a scrollY-delta assertion would have been satisfied by doing nothing.
func TestCollapsingAClaimBodyKeepsTheCardInView(t *testing.T) {
	for _, tier := range []struct {
		name string
		w, h int64
	}{{"desktop", 1440, 900}, {"mobile", 390, 844}} {
		t.Run(tier.name, func(t *testing.T) {
			p := newProject(t)
			// One long body to collapse, then enough further claims that the
			// document can actually scroll past it. With a single card the
			// page is shorter than the viewport, scrollTo clamps, and the
			// "top edge above the line" precondition can never be set up.
			p.writeClaim("tall.yaml", longBodyClaim("widget.contract.tall", "contract", 120))
			for i := 0; i < 6; i++ {
				id := fmt.Sprintf("widget.contract.filler-%02d", i)
				p.writeClaim(id+".yaml", longBodyClaim(id, "contract", 40))
			}
			ctx := browserContext(t)
			runCDP(t, ctx, chromedp.EmulateViewport(tier.w, tier.h),
				chromedp.Navigate(p.renderStatic()),
				chromedp.WaitVisible(".claim-body-disclosure__toggle:not([hidden])", chromedp.ByQuery))

			// Case 1: the card's top has been pushed above the sticky line.
			// Expand, park the card top 300px above the viewport, collapse.
			// The top edge must come to rest on the deep-link line with the
			// whole card on screen.
			if !evalBool(t, ctx, `(function(){
  var t = document.querySelector('.claim-body-disclosure__toggle:not([hidden])');
  var claim = t.closest('.claim');
  t.click();
  var offset = parseFloat(getComputedStyle(claim).scrollMarginTop) || 0;
  window.scrollTo({ top: Math.round(claim.getBoundingClientRect().top + window.scrollY + 300), behavior: 'instant' });
  if (claim.getBoundingClientRect().top >= offset) return false;
  t.click();
  var r = claim.getBoundingClientRect();
  return Math.abs(r.top - offset) <= 1 && r.bottom <= window.innerHeight + 1;
 })()`) {
				t.Fatal("collapsing a claim whose top had scrolled above the deep-link line must bring its top edge back to scroll-margin-top and leave the whole card on screen")
			}

			// Case 2: the card's top is already on screen. Nothing may move —
			// a scroll nobody asked for is its own surprise. This is the half
			// a naive "always scroll into view" fix breaks.
			if !evalBool(t, ctx, `(function(){
  var t = document.querySelector('.claim-body-disclosure__toggle:not([hidden])');
  var claim = t.closest('.claim');
  t.click();
  var offset = parseFloat(getComputedStyle(claim).scrollMarginTop) || 0;
  window.scrollTo({ top: Math.round(claim.getBoundingClientRect().top + window.scrollY - (offset + 120)), behavior: 'instant' });
  var before = claim.getBoundingClientRect().top, y = window.scrollY;
  if (before < offset) return false;
  t.click();
  return Math.abs(claim.getBoundingClientRect().top - before) <= 1 && window.scrollY === y;
 })()`) {
				t.Fatal("collapsing a claim whose top edge is already visible must not scroll the page")
			}
		})
	}
}
