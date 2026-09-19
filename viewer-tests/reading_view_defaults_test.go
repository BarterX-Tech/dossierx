package viewertests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestReadingCanvasCentresInTheColumn pins Paper board 3B-0's centred reading
// column. Paper lays the 1440px board out as rail 268 + centre column 928 +
// right rail 244, and centres the 760px canvas inside that centre column
// (`6N-0` justify-content:center). The implementation's measure was already
// 760px, but `.content-area`'s `margin: 0` (style.css:5405) overrode the
// earlier `margin: 0 auto` (style.css:4839), pinning the card to the 42px
// left padding and dumping every pixel of slack on the right.
//
// The assertion is symmetry, not an absolute x: the canvas must sit centred
// within `.content-area`'s content box, with its head and tab strip in the
// same lane. Measuring symmetry keeps this honest if the rail widths are
// retuned later.
func TestReadingCanvasCentresInTheColumn(t *testing.T) {
	p := newProject(t)
	p.writeClaim("long.yaml", longClaimYAML)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".reading-canvas .claim", chromedp.ByQuery))

	if !evalBool(t, ctx, `(function(){
  var area = document.querySelector('.content-area');
  var canvas = document.querySelector('.reading-canvas:not([hidden])');
  if (!area || !canvas) return false;
  var cs = getComputedStyle(area);
  var box = area.getBoundingClientRect();
  // Content-box edges, excluding the padding that reserves the fixed rails.
  var left  = box.left + parseFloat(cs.paddingLeft);
  var right = box.right - parseFloat(cs.paddingRight);
  var r = canvas.getBoundingClientRect();
  if (Math.round(r.width) !== 760) return false;
  var slackLeft = r.left - left, slackRight = right - r.right;
  // Centred: the two gutters match. 1px tolerance for sub-pixel layout.
  return slackLeft > 0 && Math.abs(slackLeft - slackRight) <= 1;
 })()`) {
		var debug string
		runCDP(t, ctx, chromedp.Evaluate(`(function(){
   var a=document.querySelector('.content-area'), c=document.querySelector('.reading-canvas:not([hidden])');
   var cs=getComputedStyle(a), b=a.getBoundingClientRect(), r=c.getBoundingClientRect();
   return JSON.stringify({areaLeft:b.left,areaRight:b.right,padL:cs.paddingLeft,padR:cs.paddingRight,
     canvasLeft:r.left,canvasRight:r.right,canvasWidth:r.width,margin:cs.margin});
  })()`, &debug))
		t.Fatalf("the 760px reading canvas must centre in the content column (Paper 3B-0): %s", debug)
	}

	// The record head and the facet tab strip share the canvas's lane; if only
	// the canvas centred, the head would visibly detach to the left.
	if !evalBool(t, ctx, `(function(){
  var canvas = document.querySelector('.reading-canvas:not([hidden])');
  var section = canvas.closest('.module-section');
  var head = section.querySelector(':scope > .system-record-head');
  var nav  = section.querySelector(':scope > .sub-nav');
  var c = canvas.getBoundingClientRect();
  // The head always exists; .sub-nav only when the module has more than one
  // facet, so it is checked when present rather than required.
  if (!head) return false;
  return [head, nav].every(function(el){
    if (!el) return true;
    var r = el.getBoundingClientRect();
    return Math.abs(r.left - c.left) <= 1 && Math.abs(r.width - c.width) <= 1;
  });
 })()`) {
		var debug string
		runCDP(t, ctx, chromedp.Evaluate(`(function(){
   var canvas=document.querySelector('.reading-canvas:not([hidden])');
   var section=canvas.closest('.module-section');
   var pick=function(sel){var e=section.querySelector(sel); if(!e) return {sel:sel,missing:true};
     var r=e.getBoundingClientRect(), s=getComputedStyle(e);
     return {sel:sel,left:r.left,width:r.width,margin:s.margin,maxWidth:s.maxWidth,parent:e.parentElement.className};};
   var c=canvas.getBoundingClientRect();
   return JSON.stringify({canvas:{left:c.left,width:c.width},
     sectionClass:section.className, canvasParent:canvas.parentElement.className,
     head:pick(':scope > .system-record-head'), nav:pick(':scope > .sub-nav'),
     anyHead:pick('.system-record-head'), anyNav:pick('.sub-nav')});
  })()`, &debug))
		t.Fatalf("the record head and facet tab strip must stay in the canvas's lane: %s", debug)
	}
}

// TestFacetBannerSuppressesReadinessAutoOpen pins screens/02 section 9.1:
// R09.3's readiness auto-open is per-claim and is suppressed while a
// facet-level blocked banner is showing. Paper board 3B-0 draws all four
// footer doors closed on a facet where every claim is blocked — the banner
// already says so once, and opening a door on every card buries the prose it
// is meant to annotate.
//
// R09.3 itself is unchanged and still covered by
// TestReadinessDoorJoinsTheFooterDisclosureGroup, which has no banner.
func TestFacetBannerSuppressesReadinessAutoOpen(t *testing.T) {
	p := newReadinessProject(t)
	// Another blocked claim in the same facet, resting on a claim that exists
	// (a dangling id would fail `check` outright). This is what raises the
	// facet-level strip the gate keys off.
	p.writeClaim("issue.yaml", strings.ReplaceAll(contractIssueClaimYAML, "widget.contract.missing", "widget.contract.alpha"))
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible("#statusStrip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelectorAll('.claim[id]').length > 0`)

	// Guard the premise: if the fixture stopped raising a visible banner this
	// test would pass vacuously, which would be indistinguishable from the
	// gate working.
	if !evalBool(t, ctx, `(function(){
  var s = document.getElementById('statusStrip');
  return !!s && !s.hidden && s.offsetParent !== null;
 })()`) {
		t.Fatal("fixture premise failed: the facet-level banner must be showing for this test to mean anything")
	}

	// All four footer doors closed. They are exactly the disclosures sharing a
	// claim's `name="claim-footer-<id>"` mutual-exclusion group; sub-disclosures
	// *inside* a panel (claim-readiness-module's per-module grouping) are not
	// footer doors and are deliberately not asserted on — they sit inside a
	// closed panel and are invisible until the reader opens it.
	if !evalBool(t, ctx, `(function(){
  var doors = document.querySelectorAll('.claim[id] details.claim-readiness-door');
  if (doors.length === 0) return false;
  return document.querySelectorAll('.claim[id] details[name^="claim-footer-"][open]').length === 0;
 })()`) {
		var debug string
		runCDP(t, ctx, chromedp.Evaluate(`JSON.stringify(Array.from(document.querySelectorAll('.claim[id] .claim-footer details')).map(function(d){
   return {claim:(d.closest('.claim')||{}).id, cls:d.className, open:d.open};
  }))`, &debug))
		t.Fatalf("with a facet banner showing, all four footer doors must start collapsed: %s", debug)
	}

	// The gate must suppress the auto-open, not disable the door: a reader can
	// still open it, and R09.2's mutual exclusion still holds.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.claim[id] details.claim-readiness-door summary').click()`, nil))
	pollTrue(t, ctx, `document.querySelector('.claim[id] details.claim-readiness-door').open === true`)
}

// TestFreshnessReportsMinutesNotAnHour pins R10.2's sub-hour band. The
// formatter clamped every age under 90 minutes up to "Updated 1 hour ago"
// (Math.max(1, Math.round(hours))), so a viewer opened seconds after its own
// build announced it was an hour stale — the exact defect a reader would read
// as "these claims are older than they are".
//
// renderStatic stamps the build instant, so the page under test is seconds
// old and must report itself as such.
func TestFreshnessReportsMinutesNotAnHour(t *testing.T) {
	p := newProject(t)
	p.writeClaim("long.yaml", longClaimYAML)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".freshness-footer__phrase", chromedp.ByQuery))
	pollTrue(t, ctx, `!/Updated recently/.test(document.querySelector('.freshness-footer__phrase').textContent)`)

	var phrase string
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.freshness-footer__phrase').textContent.trim()`, &phrase))
	if phrase != "Updated just now" {
		t.Fatalf("a freshly built page must report itself as just now, got %q", phrase)
	}

	// The minutes band itself, driven off a synthetic stamp so the assertion
	// does not depend on wall-clock timing. 1, 2 and 59 minutes cover the
	// singular/plural seam and the hour boundary.
	for _, tc := range []struct {
		minutesAgo int
		want       string
	}{
		{0, "Updated just now"},
		{1, "Updated 1 minute ago"},
		{2, "Updated 2 minutes ago"},
		{59, "Updated 59 minutes ago"},
		{60, "Updated 1 hour ago"},
		{125, "Updated 2 hours ago"},
		{60 * 24 * 3, "Updated 3 days ago"},
	} {
		// enhanceTimestamp has no public hook; system-record.js re-runs it from
		// a MutationObserver on <body>'s class attribute, so toggling a throwaway
		// class is how a test drives a recompute.
		runCDP(t, ctx, chromedp.Evaluate(`(function(){
   var el = document.querySelector('.freshness-footer__phrase');
   el.setAttribute('data-generated-at', new Date(Date.now() - `+strconv.Itoa(tc.minutesAgo)+` * 60000).toISOString());
   document.body.classList.toggle('dx-test-freshness-tick');
  })()`, nil))
		pollTrue(t, ctx, `document.querySelector('.freshness-footer__phrase').textContent.trim() === `+strconv.Quote(tc.want))
	}
}

// TestMobileFacetSheetRowNavigatesToItsClaim pins the behaviour a reader
// expects from the mobile "On this facet" sheet: tapping a row goes to that
// claim.
//
// A handler existed before this, but it captured the claim NODE and called
// scrollIntoView on it. Three things made that unreliable: closeFacetToc
// restores focus in a rAF, which cancels an in-flight smooth scroll; renderToc
// replaces the whole row list on almost any document event, detaching the
// captured node and making scrollIntoView a silent no-op; and under SoftMount
// a claim that still lives in a <template> is invisible to getElementById.
//
// The fix routes through location.hash, which is the established cross-file
// mechanism (graph-ui.js's "back to this claim" does the same): hashchange
// reaches viewer-runtime.js's showFromHash -> showModuleFacet, which mounts the
// surface BEFORE resolving the id. This test therefore asserts the OBSERVABLE
// outcome — hash, scroll position, sheet closed, focus restored — not the
// mechanism.
func TestMobileFacetSheetRowNavigatesToItsClaim(t *testing.T) {
	// The same fixture TestGroup02MobileNavigationAndFacetSheet uses: the facet
	// trigger is only built when the module has a .sub-nav, i.e. more than one
	// facet, so a single-facet project would never surface the sheet at all.
	p := group02NavigationProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844), chromedp.Navigate(p.renderStatic()))
	pollTrue(t, ctx, `!!document.querySelector('.facet-toc-trigger')`)

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.facet-toc-trigger').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('facet-toc-open')`)

	// Paper 50U-0 shows a count on every row, but a late-cascade
	// `.facet-toc__item > span { display: none }` (0,1,1) silently beat
	// .facet-toc__blocker-count's own (0,1,0) and hid all of them. This
	// fixture's claims carry no blockers, so their counts render empty and
	// nothing would be visible either way — the rule itself is therefore
	// exercised directly, by giving one count text and checking it paints.
	if !evalBool(t, ctx, `(function(){
  var count = document.querySelector('#systemFacetToc .facet-toc__blocker-count');
  if (!count) return false;
  count.textContent = '7';
  return getComputedStyle(count).display !== 'none';
 })()`) {
		var debug string
		runCDP(t, ctx, chromedp.Evaluate(`(function(){
   var rows = document.querySelectorAll('#systemFacetToc .facet-toc__item');
   var c = document.querySelector('#systemFacetToc .facet-toc__blocker-count');
   return JSON.stringify({rows: rows.length, hasCount: !!c,
     display: c ? getComputedStyle(c).display : null,
     firstRowHTML: rows[0] ? rows[0].outerHTML.slice(0, 300) : null});
  })()`, &debug))
		t.Fatalf("a facet-sheet row's count must not be hidden at 390px: %s", debug)
	}

	// Tap the LAST row, so a real scroll has to happen to reach it.
	target := evalString(t, ctx, `(function(){
  var rows = document.querySelectorAll('#systemFacetToc .facet-toc__item');
  return rows[rows.length - 1].dataset.claimTarget || '';
 })()`)
	if target == "" {
		t.Fatal("facet-sheet rows must carry their claim id, not a captured node")
	}
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
  var rows = document.querySelectorAll('#systemFacetToc .facet-toc__item');
  rows[rows.length - 1].click();
 })()`, nil))

	// The sheet closes and focus returns to the trigger.
	pollTrue(t, ctx, `!document.body.classList.contains('facet-toc-open')`)
	pollTrue(t, ctx, `document.activeElement === document.querySelector('.facet-toc-trigger')`)

	// The navigation actually happened: the hash names the claim...
	pollTrue(t, ctx, `window.location.hash.replace(/^#/, '').split('!')[0] === `+strconv.Quote(target))

	// ...and the claim is really on screen, which is the part a captured,
	// detached node silently failed to deliver.
	if !evalBool(t, ctx, `(function(){
  var claim = document.getElementById(`+strconv.Quote(target)+`);
  if (!claim) return false;
  var r = claim.getBoundingClientRect();
  return r.top < innerHeight && r.bottom > 0;
 })()`) {
		t.Fatalf("tapping the %q row must bring that claim into view", target)
	}
}

// TestFocusModeGivesTheFreedWidthToTheEvidence pins R11.3 and board 13J-0.
//
// When the rails leave, Paper grows the CARD from 758 to 1138 (canvas 760 to
// 1140) and the module head and facet tab strip with it, so blockers,
// breadcrumb paths, fenced code and conformance sets get the room they were
// short of. Capping only .content-area widened an empty wrapper: the canvas
// stayed at 760 and centred, so the width the rails gave back became dead
// whitespace — which is what a reader sees as "focus mode does nothing".
//
// R11.2 is the other half and pulls the opposite way: the PROSE measure must
// not move, because "the paragraph you are reading is pixel-identical before
// and after". Both are asserted here, since a fix for one that breaks the
// other is not a fix.
func TestFocusModeGivesTheFreedWidthToTheEvidence(t *testing.T) {
	p := newProject(t)
	p.writeClaim("long.yaml", longClaimYAML)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".focus-toggle", chromedp.ByQuery),
		chromedp.WaitVisible(".reading-canvas .claim", chromedp.ByQuery))

	measure := func() (canvas, prose, head int) {
		canvas = evalInt(t, ctx, `Math.round(document.querySelector('.reading-canvas:not([hidden])').getBoundingClientRect().width)`)
		prose = evalInt(t, ctx, `Math.round(document.querySelector('.reading-canvas:not([hidden]) .claim-body').getBoundingClientRect().width)`)
		head = evalInt(t, ctx, `Math.round(document.querySelector('.module-section:not([hidden]) > .system-record-head').getBoundingClientRect().width)`)
		return
	}

	offCanvas, offProse, offHead := measure()
	if offCanvas != 760 {
		t.Fatalf("default canvas = %d, want the 760px measure", offCanvas)
	}
	if offHead != 760 {
		t.Fatalf("default record head = %d, want it in the canvas's lane at 760", offHead)
	}

	runCDP(t, ctx, chromedp.Click(".focus-toggle", chromedp.ByQuery))
	pollTrue(t, ctx, `document.documentElement.getAttribute('data-focus') === 'on'`)
	pollTrue(t, ctx, `getComputedStyle(document.querySelector('.sidebar')).display === 'none'`)
	// .content-area carries `transition: padding 180ms`, so measuring straight
	// after the toggle catches the layout mid-animation and reads the OLD
	// 282px rail reservation. Wait for the padding to settle before measuring.
	pollTrue(t, ctx, `getComputedStyle(document.querySelector('.content-area')).paddingRight === '42px'`)

	onCanvas, onProse, onHead := measure()

	// R11.3: the card takes the freed width.
	if onCanvas <= offCanvas {
		t.Fatalf("focus canvas = %d, was %d — R11.3: the freed width must go to the card, not to whitespace", onCanvas, offCanvas)
	}
	if onCanvas != 1140 {
		t.Fatalf("focus canvas = %d, want Paper 13J-0's 1140px", onCanvas)
	}
	if onHead != onCanvas {
		t.Fatalf("focus record head = %d, canvas = %d — the head must grow with the card", onHead, onCanvas)
	}

	// R11.2 AS AMENDED (2026-09-19): the prose widens, but stays bounded. The
	// original rule froze it across the toggle; the maintainer chose to spend
	// some of the freed width on the paragraph rather than leave an empty
	// column beside it. The bound is the point of the assertion — the failure
	// this guards against is prose relaxing out to the card's full 1074px
	// content box, which at 17px/28px runs ~145 characters a line.
	if onProse <= offProse {
		t.Fatalf("prose = %d in focus, %d outside it — it must take some of the freed width", onProse, offProse)
	}
	if onProse != 900 {
		t.Fatalf("focus prose = %d, want the bounded 900px measure", onProse)
	}
	if onProse >= onCanvas-2*32 {
		t.Fatalf("prose = %d has filled the card's %d content box — the bound is what keeps long claim bodies readable", onProse, onCanvas-2*32)
	}
}
