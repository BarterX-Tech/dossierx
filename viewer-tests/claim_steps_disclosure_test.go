package viewertests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestStepsClaimCollapsesItsStepsWithTheBody pins that a steps claim's
// "... more" actually collapses the claim.
//
// components/steps.html renders each `.step` as a SIBLING of .claim-body, so
// the disclosure wrapper — which only ever wraps .claim-body — clamped the
// prose and left every step on screen. A collapsed steps claim therefore stood
// thousands of pixels tall with its whole content visible, which made its own
// control meaningless. Paper's collapsed card for one of these (6KC-0) is 80px
// and holds exactly two things: the truncated body and the "... more" control.
//
// The toggle must also exist regardless of how short the prose is: the steps
// are hidden while collapsed, so without a control they would be unreachable.
func TestStepsClaimCollapsesItsStepsWithTheBody(t *testing.T) {
	var steps strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&steps, "  - |\n    Step %d. %s\n", i+1,
			strings.Repeat("A declared verification step with enough prose to occupy real height. ", 6))
	}
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("stepped.yaml", `id: widget.contract.stepped
facet: contract
module: widget
status: draft
layout: steps
summary: Fixture claim used by the engine test corpus.
body: |
  a short lede that on its own would not need a disclosure at all.
rests_on:
  none: true
  reason: viewer-test fixture, not backed by any doctrine claim
steps:
`+steps.String())
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible("#widget\\.contract\\.stepped", chromedp.ByQuery))
	pollTrue(t, ctx, `!!document.querySelector('#widget\\.contract\\.stepped .step')`)

	card := `document.getElementById('widget.contract.stepped')`

	// The control must be offered even though the lede alone is short.
	pollTrue(t, ctx, `(function(){
  var t = `+card+`.querySelector('.claim-body-disclosure__toggle');
  return !!t && !t.hidden;
 })()`)

	if !evalBool(t, ctx, `(function(){
  var c = `+card+`;
  var w = c.querySelector('.claim-body-disclosure');
  if (!w || !w.classList.contains('claim-body-disclosure--collapsed')) return false;
  var steps = c.querySelectorAll(':scope > .step');
  if (steps.length !== 8) return false;
  for (var i = 0; i < steps.length; i++) {
    if (getComputedStyle(steps[i]).display !== 'none') return false;
  }
  return true;
 })()`) {
		var debug string
		runCDP(t, ctx, chromedp.Evaluate(`(function(){
   var c = `+card+`, s = c.querySelectorAll(':scope > .step');
   return JSON.stringify({height: Math.round(c.getBoundingClientRect().height),
     steps: s.length, firstStepDisplay: s[0] ? getComputedStyle(s[0]).display : null,
     wrapperClass: (c.querySelector('.claim-body-disclosure')||{}).className});
  })()`, &debug))
		t.Fatalf("a collapsed steps claim must hide its steps, not just clamp its prose: %s", debug)
	}

	collapsed := evalInt(t, ctx, `Math.round(`+card+`.getBoundingClientRect().height)`)

	// Expanding brings them back.
	runCDP(t, ctx, chromedp.Evaluate(card+`.querySelector('.claim-body-disclosure__toggle').click()`, nil))
	pollTrue(t, ctx, card+`.querySelector('.claim-body-disclosure').classList.contains('claim-body-disclosure--expanded')`)
	if !evalBool(t, ctx, `(function(){
  var steps = `+card+`.querySelectorAll(':scope > .step');
  for (var i = 0; i < steps.length; i++) {
    if (getComputedStyle(steps[i]).display === 'none') return false;
  }
  return true;
 })()`) {
		t.Fatal("expanding must reveal every step")
	}
	expanded := evalInt(t, ctx, `Math.round(`+card+`.getBoundingClientRect().height)`)
	if expanded <= collapsed*2 {
		t.Fatalf("collapsed %dpx vs expanded %dpx — the disclosure must make a real difference to the card's height", collapsed, expanded)
	}
}
