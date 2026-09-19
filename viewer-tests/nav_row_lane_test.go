package viewertests

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// TestNavRowsShareOneLaneWhenSelected pins that selecting a module or track
// row does not move its label.
//
// `.sec-tab.on` carried `padding-left: 18px`, a fossil of a 2px accent
// left-marker the redesign removed (20 - 2 = 18). Because `.sec-tab.on`
// outranks `.sec-tab`, that orphaned padding survived `border: 0` and pushed
// the selected label 8px right of every other row. Paper 4A-0 gives selected
// and unselected rows the same 10px padding and marks selection with ground
// and weight only, so the labels share one lane.
//
// Asserted at desktop AND in the mobile drawer, because the 18px was declared
// twice — once unconditionally and once inside the 860px block.
func TestNavRowsShareOneLaneWhenSelected(t *testing.T) {
	for _, tier := range []struct {
		name   string
		w, h   int64
		drawer bool
	}{{"desktop", 1440, 900, false}, {"drawer", 700, 900, true}} {
		t.Run(tier.name, func(t *testing.T) {
			p := group02NavigationProject(t)
			ctx := browserContext(t)
			runCDP(t, ctx, chromedp.EmulateViewport(tier.w, tier.h), chromedp.Navigate(p.renderStatic()),
				chromedp.WaitVisible(".sec-tab", chromedp.ByQuery))
			if tier.drawer {
				runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('navToggle').click()`, nil))
				pollTrue(t, ctx, `document.body.classList.contains('nav-open')`)
			}
			pollTrue(t, ctx, `!!document.querySelector('.system-nav-group .sec-tab.on')`)

			if !evalBool(t, ctx, `(function(){
  var rows = Array.prototype.slice.call(document.querySelectorAll('.system-nav-group .sec-tab'))
    .filter(function(r){ return r.getClientRects().length > 0; });
  if (rows.length < 2) return false;
  var on = rows.filter(function(r){ return r.classList.contains('on'); });
  if (on.length !== 1) return false;
  // Every row's own left padding edge must sit on one lane, selected or not.
  var lane = rows.map(function(r){
    return Math.round(r.getBoundingClientRect().left + parseFloat(getComputedStyle(r).paddingLeft));
  });
  var first = lane[0];
  return lane.every(function(x){ return Math.abs(x - first) <= 1; });
 })()`) {
				var debug string
				runCDP(t, ctx, chromedp.Evaluate(`JSON.stringify(Array.prototype.slice.call(
   document.querySelectorAll('.system-nav-group .sec-tab'))
   .filter(function(r){ return r.getClientRects().length > 0; })
   .slice(0, 6).map(function(r){
     return {on: r.classList.contains('on'),
       padLeft: getComputedStyle(r).paddingLeft,
       textX: Math.round(r.getBoundingClientRect().left + parseFloat(getComputedStyle(r).paddingLeft))};
   }))`, &debug))
				t.Fatalf("a selected nav row must keep its label in the same lane as every other row: %s", debug)
			}
		})
	}
}
