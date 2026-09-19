package viewertests

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

func TestGroup02ContinuousReadingCanvas(t *testing.T) {
	p := newProject(t)
	p.writeClaim("long.yaml", longClaimYAML)
	p.writeClaim("secondary.yaml", secondClaimYAML)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".reading-canvas .claim", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('.reading-canvas .claim-body-disclosure') !== null`)
	for _, width := range []int64{1440, 390} {
		runCDP(t, ctx, chromedp.EmulateViewport(width, 900))
		pollTrue(t, ctx, `getComputedStyle(document.querySelector('.content-area')).paddingLeft === (innerWidth <= 520 ? '0px' : '42px')`)
		if !evalBool(t, ctx, `(function(){
   var canvas = document.querySelector('.reading-canvas:not([hidden])');
   var claims = Array.from(canvas.querySelectorAll('.claim'));
   if(claims.length < 2) return false;
   var a=claims[0], b=claims[1], ac=getComputedStyle(a), bc=getComputedStyle(b), cc=getComputedStyle(canvas);
   var mobile=innerWidth <= 520;
   return Math.abs(a.getBoundingClientRect().bottom-b.getBoundingClientRect().top)<1 &&
    ac.borderRadius==='0px' && ac.marginBottom==='0px' && ac.borderTopWidth==='0px' &&
    bc.borderTopWidth==='1px' && ac.paddingTop===(mobile?'22px':'34px') &&
    bc.paddingTop===(mobile?'20px':'30px') && ac.paddingBottom===(mobile?'18px':'28px') &&
    ac.paddingLeft===(mobile?'16px':'32px') &&
    (mobile ? cc.borderRadius==='0px' && canvas.getBoundingClientRect().width===innerWidth :
     cc.borderTopWidth==='1px' && canvas.getBoundingClientRect().width===760) &&
    !document.querySelector('#sidebarResizer, .status-strip-caret, .subtab .dx-icon--lock, .k .claim-comments-slot') &&
    claims.every(function(claim){return !!claim.querySelector('.claim-footer > .claim-comments-slot');});
  })()`) {
			var debug string
			runCDP(t, ctx, chromedp.Evaluate(`JSON.stringify(Array.from(document.querySelectorAll('.reading-canvas:not([hidden]), .reading-canvas:not([hidden]) .claim')).map(n=>({tag:n.className,rect:n.getBoundingClientRect().toJSON(),padding:getComputedStyle(n).padding,border:getComputedStyle(n).border,margin:getComputedStyle(n).margin})))`, &debug))
			t.Fatalf("continuous Paper canvas structure/geometry failed at %dpx: %s", width, debug)
		}
	}
}

func TestGroup02StatusBelongsToActiveCanvas(t *testing.T) {
	p := newProject(t)
	p.writeClaim("issue.yaml", strings.ReplaceAll(contractIssueClaimYAML, "widget.contract.missing", "widget.contract.overview"))
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible("#statusStrip", chromedp.ByQuery))
	if !evalBool(t, ctx, `(function(){
  var canvas=document.querySelector('.reading-canvas:not([hidden])');
  var strip=document.getElementById('statusStrip');
  return strip.parentElement===canvas && canvas.firstElementChild===strip &&
   document.getElementById('statusStripToggle').getAttribute('aria-expanded')==='false';
 })()`) {
		t.Fatal("default status must be the first child of the active reading canvas and stay collapsed")
	}
}

func TestGroup02CommentFooterRows(t *testing.T) {
	p := newProject(t)
	ctx := newLiveTab(t, p)
	pollTrue(t, ctx, `!!document.querySelector('.claim-footer > .claim-comments-slot:not([hidden])')`)
	for _, width := range []int64{1440, 390} {
		runCDP(t, ctx, chromedp.EmulateViewport(width, 900))
		pollTrue(t, ctx, `getComputedStyle(document.querySelector('.content-area')).paddingLeft === (innerWidth <= 520 ? '0px' : '42px')`)
		if !evalBool(t, ctx, `(function(){
   var footer=document.querySelector('.reading-canvas .claim-footer');
   var comment=footer.querySelector('.claim-comments-slot');
   var readiness=footer.querySelector('.claim-readiness-door, .claim-readiness-empty');
   var relationships=footer.querySelector('.claim-links, .claim-footer-chip--relationships');
   if(!comment || !readiness || !relationships) return false;
   var c=comment.getBoundingClientRect(),r=readiness.getBoundingClientRect(),l=relationships.getBoundingClientRect();
   return !comment.hidden && getComputedStyle(comment.querySelector('button')).opacity==='1' &&
     Math.abs((c.top+c.bottom)/2-(r.top+r.bottom)/2)<2 &&
     (innerWidth <= 520 ? l.top >= r.bottom+7 : Math.abs(l.top-r.top)<2) &&
     Math.abs(c.right-footer.getBoundingClientRect().right)<2;
  })()`) {
			t.Fatalf("comment/readiness/footer row geometry failed at %dpx", width)
		}
	}
}
