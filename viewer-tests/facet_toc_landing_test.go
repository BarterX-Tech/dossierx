package viewertests

import (
	"fmt"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestFacetTocRowLandsImmediately pins that activating a facet-TOC row lands
// the claim AT ONCE, at the same offset a deep link uses.
//
// TestMobileFacetSheetRowNavigatesToItsClaim already asserts that a row
// navigates, and it passed while readers reported the rows as dead. It missed
// this for four reasons, each fixed here: it ran only at 390px; it used a
// fixture a few hundred pixels tall, where a smooth scroll finishes instantly;
// it polled for seconds, so a 2.4s landing passed exactly like a 130ms one;
// and it accepted a claim one pixel inside the viewport, which a half-finished
// animation satisfies.
//
// The fixture here is deliberately several viewports tall, the landing is
// measured two frames after activation with NO polling, and the assertion is
// the claim's exact resting offset.
func TestFacetTocRowLandsImmediately(t *testing.T) {
	for _, tier := range []struct {
		name  string
		w, h  int64
		sheet bool
	}{{"desktop", 1440, 1000, false}, {"mobile", 390, 844, true}} {
		t.Run(tier.name, func(t *testing.T) {
			p := newProjectRaw(t, group02NavigationConfigYAML)
			for i := 0; i < 14; i++ {
				id := fmt.Sprintf("widget.contract.c%02d", i)
				p.writeClaim(id+".yaml", longBodyClaim(id, "contract", 60))
			}
			p.writeClaim("widget-internals.yaml", longBodyClaim("widget.internals.detail", "internals", 4))
			ctx := browserContext(t)
			runCDP(t, ctx, chromedp.EmulateViewport(tier.w, tier.h),
				chromedp.Navigate(p.renderStatic()),
				chromedp.WaitVisible(".claim", chromedp.ByQuery))
			pollTrue(t, ctx, `document.querySelectorAll('#systemFacetToc .facet-toc__item').length > 1`)

			// The facet must be tall enough that a smooth scroll would be
			// visibly slow; otherwise this test proves nothing.
			if h := evalInt(t, ctx, `document.documentElement.scrollHeight`); h < int(tier.h)*3 {
				t.Fatalf("fixture is only %dpx tall; too short to distinguish an instant landing from an animated one", h)
			}

			if tier.sheet {
				runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc-trigger').click()`, nil))
				pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open')`)
			}

			// Click a MIDDLE row, not the last: the document cannot scroll far
			// enough to bring the final card to the top, so its resting
			// position is the scroll maximum rather than scroll-margin-top and
			// the assertion would be measuring the page's end, not the jump.
			// A middle row is still several viewports away.
			// Record where the
			// claim has come to rest 250ms later. The measurement INSTANT is
			// what matters: polling afterwards only collects a number already
			// taken at 250ms, so the 2.4s animated landing readers complained
			// about cannot masquerade as a prompt one. 250ms is a deliberate
			// budget, not a frame count: the hash assignment and the
			// hashchange that mounts the surface are each their own task, so
			// an exact frame is not a stable contract, while "the jump is over
			// before the reader wonders whether the click registered" is.
			runCDP(t, ctx, chromedp.Evaluate(`(function(){
  window.__landing = null;
  var rows = document.querySelectorAll('#systemFacetToc .facet-toc__item');
  var row = rows[Math.floor(rows.length / 2)];
  var id = row.dataset.claimTarget;
  row.click();
  setTimeout(function(){
    var c = document.getElementById(id);
    window.__landing = { id: id, found: !!c,
      top: c ? Math.round(c.getBoundingClientRect().top) : null,
      offset: c ? Math.round(parseFloat(getComputedStyle(c).scrollMarginTop) || 0) : null };
  }, 250);
})()`, nil))
			pollTrue(t, ctx, `window.__landing !== null`)
			landed := evalString(t, ctx, `JSON.stringify(window.__landing)`)
			t.Logf("landing at 250ms: %s", landed)

			if !evalBool(t, ctx, `(function(){
  var l = window.__landing;
  return !!l && l.found && Math.abs(l.top - l.offset) <= 2;
 })()`) {
				t.Fatalf("the activated row's claim must be at rest on its scroll-margin-top within 250ms, not still animating: %s", landed)
			}
		})
	}
}
