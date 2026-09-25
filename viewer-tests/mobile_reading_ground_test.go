package viewertests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestMobileCardGroundStartsAtTheHeadBand pins where the reading surface's
// background begins at phone width.
//
// Paper 4O0-0 starts the reading surface (4R8-0, which carries --color-card)
// immediately where the head band ends, and puts the blocked banner 14px
// INSIDE it. The engine had no block-formatting context on .reading-canvas at
// this width (border: 0, overflow: visible), so the banner's own margin-top
// collapsed OUT of the canvas and dragged its top edge down to the banner:
// the card ground began at the banner and the strip above it was page ground,
// which is what a reader sees as the background "starting at the issue view".
//
// Both module shapes are covered, because they reach the same result by
// different routes: with a facet tab row the band ends at the tabs, and
// without one it ends at the module head's own bottom padding.
func TestMobileCardGroundStartsAtTheHeadBand(t *testing.T) {
	for _, tc := range []struct {
		name   string
		twoFac bool
	}{{"with-facet-tabs", true}, {"single-facet", false}} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultConfigYAML
			if tc.twoFac {
				cfg = group02NavigationConfigYAML
			}
			p := newProjectRaw(t, cfg)
			for i := 0; i < 3; i++ {
				id := fmt.Sprintf("widget.contract.f%02d", i)
				p.writeClaim(id+".yaml", longBodyClaim(id, "contract", 20))
			}
			p.writeClaim("issue.yaml", strings.ReplaceAll(contractIssueClaimYAML, "widget.contract.missing", "widget.contract.f00"))
			if tc.twoFac {
				p.writeClaim("internals.yaml", longBodyClaim("widget.internals.detail", "internals", 4))
			}
			ctx := browserContext(t)
			runCDP(t, ctx, chromedp.EmulateViewport(390, 844), chromedp.Navigate(p.renderStatic()),
				chromedp.WaitVisible(".reading-canvas:not([hidden])", chromedp.ByQuery))
			pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)

			if !evalBool(t, ctx, `(function(){
  var sec = document.querySelector('.module-section:not([hidden])');
  var head = sec.querySelector(':scope > .system-record-head');
  var nav = sec.querySelector(':scope > .sub-nav');
  var canvas = sec.querySelector(':scope > .reading-canvas:not([hidden])');
  // The BANNER CARD, not the strip that holds it: the strip stopped being
  // the painted surface when the head became a card with a row per finding
  // (Paper EH1-0) and is now the card's gutter, so its own top IS the
  // canvas top and measuring it could no longer see the 14px at all.
  var strip = document.getElementById('statusStripCard');
  if (!head || !canvas || !strip) return false;
  var bandBottom = Math.round((nav || head).getBoundingClientRect().bottom);
  var canvasTop = Math.round(canvas.getBoundingClientRect().top);
  var stripTop = Math.round(strip.getBoundingClientRect().top);
  // The card ground begins AT the band, not at the banner...
  if (Math.abs(canvasTop - bandBottom) > 1) return false;
  // ...and the banner sits Paper's 14px inside it, so that gap is card ground.
  if (Math.abs(stripTop - canvasTop - 14) > 1) return false;
  // The canvas must actually paint a ground distinct from the page.
  var cg = getComputedStyle(canvas).backgroundColor;
  return cg !== 'rgba(0, 0, 0, 0)' && cg !== 'transparent';
 })()`) {
				var debug string
				runCDP(t, ctx, chromedp.Evaluate(`(function(){
   var sec=document.querySelector('.module-section:not([hidden])');
   var head=sec.querySelector(':scope > .system-record-head');
   var nav=sec.querySelector(':scope > .sub-nav');
   var canvas=sec.querySelector(':scope > .reading-canvas:not([hidden])');
   var strip=document.getElementById('statusStripCard');
   var b=function(e){return e?Math.round(e.getBoundingClientRect().bottom):null};
   var r=function(e){return e?Math.round(e.getBoundingClientRect().top):null};
   return JSON.stringify({hasNav:!!nav, headBottom:b(head), navBottom:b(nav),
     canvasTop:r(canvas), stripTop:r(strip), canvasBg:getComputedStyle(canvas).backgroundColor});
  })()`, &debug))
				t.Fatalf("the card ground must start at the head band with the banner 14px inside it: %s", debug)
			}
		})
	}
}
