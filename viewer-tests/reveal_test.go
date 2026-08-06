package viewertests

// Collapsed-footer reveal suite — the browser half of decision C9 (deep-link
// auto-open) and Q4 (print). v0.4.1 made details.claim-links collapse by
// default, and TWO cases must still show its contents without a click:
//
//   - .claim:target                — someone followed a link to #<claim-id>
//   - @media print                 — paper has no disclosure widget
//
// Both are implemented as CSS ONLY (style.css), and both had NO automated test
// until this file. That gap is not academic: the pre-repair implementation set
// content-visibility on the details' CHILDREN, which is a no-op. A closed
// <details> hides its contents through the UA's ::details-content
// pseudo-element, and a descendant cannot opt back out of an ancestor the UA
// has already skipped — so both reveals shipped broken behind a green suite,
// and the next stylesheet edit could silently undo the repair the same way.
//
// What is asserted is the RELATIONSHIP, never the pixel numbers: the numbers
// depend on the fixture text and the machine's fonts, so a test that pinned
// them would fail for reasons that have nothing to do with the reveal.
//
//	before:  getComputedStyle(details, '::details-content').contentVisibility === 'hidden'
//	after:   ... === 'visible', and the details box is strictly taller
//
// Plus the invariant that must NOT change in either revealed state: the `open`
// content attribute stays ABSENT. CSS cannot write one, that is the whole
// reason the reveal is expressed as content-visibility, and a future "fix" that
// starts setting `open` from JS or from the server would be a regression of
// C9 — it would not survive the static file:// export the reveal exists for.
//
// The fixture is a throwaway two-claim project written by this file, so no
// committed testdata viewer has to be regenerated with it.
//
// A third test, TestPrintCoversOnlyTheOnScreenFacet, pins the print reveal's
// stated SCOPE BOUNDARY rather than its behaviour — see the long comment on
// that test before changing it.
//
// FAILURE STYLE. Every reveal here is asserted by READING ONCE and comparing,
// never by polling for the flip as the pass condition. A poll that never goes
// true reports "condition never became true", which names neither what was
// measured nor what it should have been; a developer who breaks the stylesheet
// deserves a message that names ::details-content and the rule responsible.
// The bounded settleFor below exists only so the read happens after the style
// recalculation, and it deliberately does NOT decide pass or fail.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// The footer only renders when a claim has at least one link or file (see
// components.EdgesHTMLWithLinks — "governed_by: none" is a stated absence and
// does not count), and it must render CLOSED, so neither auto-open signal may
// fire: no drifted implemented-in file, and status draft rather than
// locked + review_pending. One rests_on edge between two draft claims is the
// smallest fixture with those properties — and it gives BOTH claims a footer,
// since the reverse index hands the base claim a "depended on by" row.
const revealBaseClaim = `id: widget.contract.base
facet: contract
module: widget
status: draft
body: |
  the claim the deep-linked one rests on; its own footer must stay collapsed.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const revealDeepClaim = `id: widget.contract.deep
facet: contract
module: widget
status: draft
body: |
  the deep-link target: landing on its id must reveal the footer below.
rests_on:
  - widget.contract.base
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const (
	revealDeepID = "widget.contract.deep"
	revealBaseID = "widget.contract.base"
)

// The two stylesheet rules under test, quoted so a failure can name the exact
// selector responsible without the reader opening style.css. If a selector here
// stops matching the stylesheet the test still fails correctly — only the
// message's pointer goes stale — so these are documentation, not assertions.
const (
	targetRule = ".claim:target .claim-links::details-content"
	printRule  = "@media print { details.claim-links::details-content }"
)

// minGrowthPx is a float-noise guard, NOT a measurement of the fixture. The
// revealed content is a whole <ul> of edge rows, worth tens of pixels; 1px only
// rules out a "taller" that is really subpixel rounding on a fractional
// getBoundingClientRect.
const minGrowthPx = 1.0

// settleTimeout caps every bounded wait in this file. Both reveal signals are a
// style recalculation away, not a network round trip away — measured on a cold
// headless tab the flip is already done by the NEXT CDP round trip (~50ms), so
// three seconds is roughly two orders of magnitude of slack for a loaded CI box
// and still fails ~7x faster than the 20s poll this replaced.
const settleTimeout = 3 * time.Second

// settleFor waits at most settleTimeout for expr to become true and then
// returns REGARDLESS of whether it did. That is the whole point: it is a
// settle, never an assertion. The pass/fail decision always belongs to the
// read-once assertion that follows, so a reveal that is genuinely broken fails
// through a message naming ::details-content and the CSS rule responsible,
// instead of through pollTrue's generic "condition never became true within
// timeout" — which names the cause of nothing.
//
// Discarding the poll error is safe for the tab. chromedp implements the
// polling timeout INSIDE the predicate function it injects into the page and
// surfaces the expiry as ErrPollingTimeout; the Go context is never cancelled,
// so a timed-out settle leaves this tab fully usable for the measurement that
// follows. (pollTrue cannot be reused here for exactly the opposite reason: it
// t.Fatalf's on that error, which is the slow generic failure being removed.)
func settleFor(t *testing.T, ctx context.Context, expr string) {
	t.Helper()
	var ok bool
	// A timeout here is the ordinary outcome — that is the whole reason this
	// helper exists instead of pollTrue — so it is swallowed deliberately
	// rather than ignored implicitly.
	if err := chromedp.Run(ctx, chromedp.Poll(expr, &ok,
		chromedp.WithPollingInterval(20*time.Millisecond),
		chromedp.WithPollingTimeout(settleTimeout),
	)); err != nil && !errors.Is(err, chromedp.ErrPollingTimeout) {
		t.Fatalf("settle %q: %v", expr, err)
	}
}

// revealedExpr is the settle condition for a claim's footer: the pseudo-element
// has flipped. It is only ever passed to settleFor — never to pollTrue — so a
// false result ends a bounded wait rather than ending the test.
func revealedExpr(claimID string) string {
	return fmt.Sprintf(`(function () {
		var sec = document.getElementById(%q);
		var d = sec && sec.querySelector('details.claim-links');
		return !!d && getComputedStyle(d, '::details-content').contentVisibility === 'visible';
	})()`, claimID)
}

// footerState is one observation of a claim's disclosure: the computed
// content-visibility of the UA pseudo-element that does the hiding, the height
// of the <details> box, and whether the `open` content attribute is set.
type footerState struct {
	Found             bool    `json:"found"`
	ContentVisibility string  `json:"contentVisibility"`
	Height            float64 `json:"height"`
	Open              bool    `json:"open"`
}

func (s footerState) String() string {
	return fmt.Sprintf("::details-content content-visibility=%q, details height=%.1fpx, open=%v",
		s.ContentVisibility, s.Height, s.Open)
}

// readFooter measures the disclosure inside the named claim. It reads the
// pseudo-element rather than the <ul> because the pseudo-element is where the
// UA's hiding actually lives — reading the child is exactly the mistake that
// let the broken implementation look fine.
func readFooter(t *testing.T, ctx context.Context, claimID string) footerState {
	t.Helper()
	var s footerState
	expr := fmt.Sprintf(`(function () {
		var sec = document.getElementById(%q);
		var d = sec && sec.querySelector('details.claim-links');
		if (!d) { return { found: false, contentVisibility: '', height: 0, open: false }; }
		return {
			found: true,
			contentVisibility: getComputedStyle(d, '::details-content').contentVisibility,
			height: d.getBoundingClientRect().height,
			open: d.hasAttribute('open')
		};
	})()`, claimID)
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &s)); err != nil {
		t.Fatalf("read footer of %s: %v", claimID, err)
	}
	if !s.Found {
		t.Fatalf("%s rendered no <details class=\"claim-links\"> — the fixture no longer produces a footer to reveal", claimID)
	}
	return s
}

// assertCollapsed is the precondition BOTH cases share: the footer must start
// hidden, or "it is visible afterwards" proves nothing. A value that is neither
// "hidden" nor "visible" (or an unexpected "visible" here) also catches an
// engine that does not implement ::details-content at all — on which this
// repair does not apply and the suite must not report a silent pass.
func assertCollapsed(t *testing.T, s footerState, claimID string) {
	t.Helper()
	if s.ContentVisibility != "hidden" {
		t.Fatalf("%s: a closed footer must start with ::details-content hidden, got %s\n"+
			"(if this reads %q the browser may predate ::details-content — Chrome/Edge 131+, Safari 18.4+, Firefox 139+ — which this reveal requires)",
			claimID, s, s.ContentVisibility)
	}
	if s.Open {
		t.Fatalf("%s: the fixture footer must ship CLOSED (no auto-open signal), got %s", claimID, s)
	}
}

// assertRevealed is the postcondition both cases share: the pseudo-element
// flipped to visible, the box actually grew by it, and no `open` attribute
// appeared — CSS cannot write one, and anything that starts writing one has
// left the CSS-only contract C9 depends on.
//
// This is the ONLY path by which a broken reveal may fail, which is why every
// message names the value measured, the value required, and the stylesheet rule
// that produces it (passed as `rule`). A reader should be able to act on the
// failure without opening style.css to find out what a reveal even is.
func assertRevealed(t *testing.T, before, after footerState, claimID, how, rule string) {
	t.Helper()
	t.Logf("%s %s:\n  before: %s\n  after:  %s", claimID, how, before, after)

	if after.ContentVisibility != "visible" {
		t.Errorf("%s: ::details-content stayed content-visibility:%q %s — it must compute to \"visible\", so the collapsed footer is NOT being revealed.\n"+
			"  measured: %s\n"+
			"  mechanism: the reveal is performed by `%s` in internal/render/viewer/template/style.css, which sets content-visibility: visible. If that rule was edited, re-scoped or dropped, that is the cause.\n"+
			"  note: the rule MUST target the ::details-content pseudo-element. Setting content-visibility on the details' CHILDREN instead is a no-op — a closed <details> hides its contents through that UA pseudo-element, and a descendant cannot opt back out of an ancestor the UA has already skipped. That exact mistake is what shipped broken before v0.4.1.",
			claimID, after.ContentVisibility, how, after, rule)
	}
	if !(after.Height > before.Height+minGrowthPx) {
		t.Errorf("%s: the <details> box measured %.1fpx %s but was %.1fpx collapsed — a revealed footer must be strictly taller (by more than %.1fpx of float noise).\n"+
			"  mechanism: `%s` in internal/render/viewer/template/style.css.\n"+
			"  note: a 'visible' computed value that does not grow the box means the contents still are not being laid out.",
			claimID, after.Height, how, before.Height, minGrowthPx, rule)
	}
	if after.Open {
		t.Errorf("%s: the footer revealed %s carries the `open` content attribute, and must not.\n"+
			"  measured: %s\n"+
			"  why: the reveal is CSS-only by decision C9 (a fragment is unknowable server-side, and JS would not survive the static file:// export this reveal exists for), and CSS cannot write `open`. An `open` here means something started setting it from JS or from the server — a regression of C9, not a fix.",
			claimID, how, after)
	}
}

// revealFixtureTab renders the two-claim fixture as a STATIC viewer and opens
// it. file:// is the destination that matters here: the deep-link reveal exists
// precisely because it must work with no server and no runtime behind it.
func revealFixtureTab(t *testing.T) context.Context {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("base.yaml", revealBaseClaim)
	p.writeClaim("deep.yaml", revealDeepClaim)
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("details.claim-links", chromedp.ByQuery),
	)
	return ctx
}

// ---------------------------------------------------------------------
// C9 — landing on #<claim-id> reveals that claim's footer, on screen.
// ---------------------------------------------------------------------

func TestDeepLinkRevealsCollapsedFooter(t *testing.T) {
	ctx := revealFixtureTab(t)

	// No fragment yet, and no print emulation: this is the ordinary on-screen
	// reading state, in which every footer is collapsed.
	if got := evalString(t, ctx, `window.location.hash`); got != "" {
		t.Fatalf("the fixture tab must start with no fragment, got %q", got)
	}
	before := readFooter(t, ctx, revealDeepID)
	assertCollapsed(t, before, revealDeepID)
	baseBefore := readFooter(t, ctx, revealBaseID)
	assertCollapsed(t, baseBefore, revealBaseID)

	// Follow the deep link, give the style recalculation a bounded moment to
	// land, then READ ONCE and assert. The settle is not the test — see
	// settleFor: if the reveal is broken this costs settleTimeout and then fails
	// through assertRevealed, which names ::details-content and the rule, rather
	// than through a 20s poll that names neither.
	runCDP(t, ctx, chromedp.Evaluate(fmt.Sprintf(`window.location.hash = %q;`, revealDeepID), nil))
	settleFor(t, ctx, revealedExpr(revealDeepID))

	after := readFooter(t, ctx, revealDeepID)
	assertRevealed(t, before, after, revealDeepID, "after :target matched", targetRule)

	// The rule must not fan out. render.stripOverviewIDs guarantees at most one
	// element can match :target, so the OTHER claim's footer — which has its own
	// edges and would be just as revealable — must still be collapsed.
	baseAfter := readFooter(t, ctx, revealBaseID)
	if baseAfter.ContentVisibility != "hidden" {
		t.Errorf("%s: a claim that is not the :target must stay collapsed, got %s", revealBaseID, baseAfter)
	}
	if baseAfter.Height != baseBefore.Height {
		t.Errorf("%s: a non-targeted footer must not change height, got %.1fpx after vs %.1fpx before",
			revealBaseID, baseAfter.Height, baseBefore.Height)
	}
}

// ---------------------------------------------------------------------
// Q4 — under print media EVERY on-screen claim's footer is revealed, with no
// fragment involved at all.
// ---------------------------------------------------------------------

func TestPrintMediaRevealsCollapsedFooter(t *testing.T) {
	ctx := revealFixtureTab(t)

	// Deliberately NO fragment: this must be the @media print block doing the
	// work, not the .claim:target rule riding along.
	if got := evalString(t, ctx, `window.location.hash`); got != "" {
		t.Fatalf("the print case must run with no fragment set, got %q", got)
	}
	deepBefore := readFooter(t, ctx, revealDeepID)
	assertCollapsed(t, deepBefore, revealDeepID)
	baseBefore := readFooter(t, ctx, revealBaseID)
	assertCollapsed(t, baseBefore, revealBaseID)

	// Emulate print on the SAME tab, so the two measurements differ only by the
	// media type. (Emulation.setEmulatedMedia is what the DevTools "Emulate CSS
	// media type" control drives; it re-resolves @media and recomputes style.)
	emulatePrint(t, ctx)

	// Same fast-and-loud shape as the deep-link case: bounded settle, read once,
	// assert through the descriptive path.
	settleFor(t, ctx, revealedExpr(revealDeepID))
	deepAfter := readFooter(t, ctx, revealDeepID)
	assertRevealed(t, deepBefore, deepAfter, revealDeepID, "under print media", printRule)

	// Unlike :target, print is not scoped to one claim — every footer on the
	// printed facet is revealed, which is the whole point (a printed page is the
	// one surface with no way to click a disclosure open). "on the printed
	// facet" is load-bearing and is not an accident of this single-facet
	// fixture; TestPrintCoversOnlyTheOnScreenFacet pins the boundary.
	settleFor(t, ctx, revealedExpr(revealBaseID))
	baseAfter := readFooter(t, ctx, revealBaseID)
	assertRevealed(t, baseBefore, baseAfter, revealBaseID, "under print media", printRule)
}

// emulatePrint switches the tab to print media and confirms the switch actually
// took effect, bounded and with its own message. Without this check a failure to
// emulate would surface as "the footer did not reveal", sending the reader to
// the stylesheet to debug a CDP problem.
func emulatePrint(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, emulation.SetEmulatedMedia().WithMedia("print"))
	settleFor(t, ctx, `window.matchMedia('print').matches`)
	if !evalBool(t, ctx, `window.matchMedia('print').matches`) {
		t.Fatalf("print emulation never took effect: window.matchMedia('print').matches is still false %v after Emulation.setEmulatedMedia(media=print)\n"+
			"  this is a harness/CDP problem, NOT a stylesheet problem — the @media print block cannot be under test until the tab reports print media.",
			settleTimeout)
	}
}

// ---------------------------------------------------------------------
// Q4, SCOPE — only the ON-SCREEN facet prints.
// ---------------------------------------------------------------------

// twoFacetConfigYAML gives the one module TWO facets, which is what makes a
// hidden facet exist at all: shell.html renders one section.claim-group per
// facet, shows the first, and leaves every other one carrying `hidden`. A
// single-facet project — which every other fixture in this suite is — never
// produces the state this test is about.
const twoFacetConfigYAML = `schema_version: 1
facets:
  - contract
  - interface
modules:
  - widget
claims_dir: claims
`

// Four claims, two per facet, each facet an internal rests_on pair so that BOTH
// facets contain a claim with a real footer to reveal. The edges stay within
// their own facet on purpose: the only difference between the two claims being
// compared must be which facet they live in.
const printFacetInterfaceBase = `id: widget.interface.base
facet: interface
module: widget
status: draft
body: |
  the inactive facet's base claim.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const printFacetInterfaceDeep = `id: widget.interface.deep
facet: interface
module: widget
status: draft
body: |
  a claim in the facet the reader is NOT looking at; its footer never reaches paper.
rests_on:
  - widget.interface.base
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const (
	// The facet group ids are slugify("<module>-<facet>") — see render.Group.ID.
	activeGroupID   = "widget-contract"
	inactiveGroupID = "widget-interface"
	inactiveDeepID  = "widget.interface.deep"
)

// facetFooterState is a WIDER observation than footerState: for a claim inside a
// display:none subtree the interesting quantities are not the disclosure's
// computed values but whether the thing is in the box tree at all, plus the
// state of the ancestor group that removed it.
type facetFooterState struct {
	Found             bool    `json:"found"`
	ContentVisibility string  `json:"contentVisibility"`
	Height            float64 `json:"height"`
	ClientRects       int     `json:"clientRects"`
	GroupID           string  `json:"groupID"`
	GroupHidden       bool    `json:"groupHidden"`
	GroupDisplay      string  `json:"groupDisplay"`
}

func (s facetFooterState) String() string {
	return fmt.Sprintf("details height=%.1fpx, clientRects=%d, ::details-content content-visibility=%q; ancestor %s[hidden=%v, display=%q]",
		s.Height, s.ClientRects, s.ContentVisibility, s.GroupID, s.GroupHidden, s.GroupDisplay)
}

// readFacetFooter measures a claim's footer TOGETHER with the facet group that
// contains it.
func readFacetFooter(t *testing.T, ctx context.Context, claimID string) facetFooterState {
	t.Helper()
	var s facetFooterState
	expr := fmt.Sprintf(`(function () {
		var sec = document.getElementById(%q);
		var d = sec && sec.querySelector('details.claim-links');
		if (!d) { return { found: false }; }
		var g = sec.closest('.claim-group');
		return {
			found: true,
			contentVisibility: getComputedStyle(d, '::details-content').contentVisibility,
			height: d.getBoundingClientRect().height,
			clientRects: d.getClientRects().length,
			groupID: g ? g.id : '',
			groupHidden: g ? g.hasAttribute('hidden') : false,
			groupDisplay: g ? getComputedStyle(g).display : ''
		};
	})()`, claimID)
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &s)); err != nil {
		t.Fatalf("read facet footer of %s: %v", claimID, err)
	}
	if !s.Found {
		t.Fatalf("%s rendered no <details class=\"claim-links\"> — the two-facet fixture no longer produces a footer in that facet", claimID)
	}
	return s
}

// TestPrintCoversOnlyTheOnScreenFacet pins the SCOPE BOUNDARY of the print
// reveal: printing covers the facet on screen, and only that one.
//
// ============================ READ THIS FIRST ============================
// THIS TEST PINS A DOCUMENTED LIMITATION, NOT A DESIRED INVARIANT.
//
// The decision it encodes is the SCOPE clause of the @media print block in
// internal/render/viewer/template/style.css (decision Q4, v0.4.1): "only the
// facet currently ON SCREEN prints", because un-hiding every facet for print is
// a page-order / per-facet-heading / duplicated-overview design change larger
// than a patch release should make. A human ruled on that deliberately; nothing
// exercised it, because every other fixture in this suite has one facet.
//
// So: if someone later decides printing SHOULD cover every facet, this test is
// EXPECTED TO FAIL, and the right response is to DELETE OR REWRITE IT along
// with the SCOPE clause it quotes — never to work around it, and never to
// preserve the limitation just because a green test appears to demand it. The
// failure is the notification that the boundary moved, which is the entire
// reason to pin a limitation at all. A test that silently defends a limitation
// nobody meant to keep is worse than no test.
// =========================================================================
//
// ONE MORE TRAP, measured rather than assumed. Under print emulation the hidden
// facet's ::details-content DOES compute to "visible" — the cascade does not
// care that an ancestor is display:none, so the print rule matches there just as
// it does on screen. What does not happen is any RENDERING: the box is 0px with
// zero client rects, because display:none removed the subtree from the box tree
// and content-visibility on a descendant cannot resurrect it. That is exactly
// what the stylesheet's SCOPE clause says, and it is why the assertion below is
// about the box and NOT about content-visibility. Do not "tighten" this test by
// requiring the hidden facet's content-visibility to stay "hidden": that would
// be asserting something the engine does not do, and it would fail.
func TestPrintCoversOnlyTheOnScreenFacet(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfigYAML)
	p.writeClaim("base.yaml", revealBaseClaim)
	p.writeClaim("deep.yaml", revealDeepClaim)
	p.writeClaim("iface-base.yaml", printFacetInterfaceBase)
	p.writeClaim("iface-deep.yaml", printFacetInterfaceDeep)
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("details.claim-links", chromedp.ByQuery),
	)

	// No fragment: the facet on screen must be the one shell.html activates by
	// default, not one a deep link selected.
	if got := evalString(t, ctx, `window.location.hash`); got != "" {
		t.Fatalf("the print-scope case must run with no fragment set, got %q", got)
	}

	// The fixture is only meaningful if the two facets really are in the two
	// states this test names, so establish that first and say so loudly if the
	// viewer's default-facet behaviour has changed underneath.
	activeBefore := readFacetFooter(t, ctx, revealDeepID)
	inactiveBefore := readFacetFooter(t, ctx, inactiveDeepID)
	t.Logf("on screen:\n  active   %s: %s\n  inactive %s: %s",
		revealDeepID, activeBefore, inactiveDeepID, inactiveBefore)

	if activeBefore.GroupID != activeGroupID || activeBefore.GroupHidden {
		t.Fatalf("fixture precondition: %s must sit in the VISIBLE facet group %q, got %s\n"+
			"(shell.html activates a module's first facet on load; if that changed, this fixture no longer sets up the comparison it claims to)",
			revealDeepID, activeGroupID, activeBefore)
	}
	if inactiveBefore.GroupID != inactiveGroupID || !inactiveBefore.GroupHidden {
		t.Fatalf("fixture precondition: %s must sit in the HIDDEN facet group %q, got %s\n"+
			"(without a genuinely hidden second facet this test proves nothing)",
			inactiveDeepID, inactiveGroupID, inactiveBefore)
	}
	if inactiveBefore.GroupDisplay != "none" {
		t.Fatalf("fixture precondition: the hidden facet group %s must compute display:none, got %q — the `hidden` attribute is what removes it from the box tree, and the whole limitation follows from that",
			inactiveGroupID, inactiveBefore.GroupDisplay)
	}

	// The active facet's footer starts collapsed, exactly as in the single-facet
	// print test — this half is the ordinary Q4 behaviour, re-asserted here so a
	// failure distinguishes "print stopped working" from "print scope changed".
	activeFooterBefore := readFooter(t, ctx, revealDeepID)
	assertCollapsed(t, activeFooterBefore, revealDeepID)

	emulatePrint(t, ctx)

	// (a) the ACTIVE facet is revealed — the documented behaviour.
	settleFor(t, ctx, revealedExpr(revealDeepID))
	activeFooterAfter := readFooter(t, ctx, revealDeepID)
	assertRevealed(t, activeFooterBefore, activeFooterAfter, revealDeepID, "under print media, in the on-screen facet", printRule)

	// (b) the INACTIVE facet is NOT revealed — the documented LIMITATION. The
	// measurement is rendering, not computed style: see the trap note above.
	inactiveAfter := readFacetFooter(t, ctx, inactiveDeepID)
	t.Logf("under print media, inactive facet %s: %s\n"+
		"  (whatever ::details-content computes to here is beside the point — the print rule still MATCHES inside a display:none subtree, it simply produces no rendering. The assertions below are about the box, never the computed value.)",
		inactiveDeepID, inactiveAfter)

	if inactiveAfter.GroupHidden != true || inactiveAfter.GroupDisplay != "none" {
		t.Errorf("%s: printing must not un-hide the other facet, but its ancestor group %s is now %s\n"+
			"  if this is intentional — printing was CHANGED to cover every facet — then the SCOPE clause in the @media print block of internal/render/viewer/template/style.css is now wrong, and THIS TEST should be rewritten or deleted rather than worked around.",
			inactiveDeepID, inactiveGroupID, inactiveAfter)
	}
	if inactiveAfter.Height != 0 || inactiveAfter.ClientRects != 0 {
		t.Errorf("%s: a collapsed footer in the INACTIVE facet must stay UNRENDERED under print, but it measured %.1fpx across %d client rect(s).\n"+
			"  measured: %s\n"+
			"  this is decision Q4's stated SCOPE: only the on-screen facet prints, because the other facet's ancestor section.claim-group carries `hidden` (display:none) and a disclosure rule cannot resurrect a subtree removed from the box tree.\n"+
			"  if printing every facet is now WANTED, that is a real design change (page order, per-facet headings, duplicated overview copies) — make it, update the SCOPE clause in internal/render/viewer/template/style.css, and rewrite or delete this test. Do not work around it.",
			inactiveDeepID, inactiveAfter.Height, inactiveAfter.ClientRects, inactiveAfter)
	}
}
