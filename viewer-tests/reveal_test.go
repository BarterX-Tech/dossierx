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

import (
	"context"
	"fmt"
	"testing"

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

// minGrowthPx is a float-noise guard, NOT a measurement of the fixture. The
// revealed content is a whole <ul> of edge rows, worth tens of pixels; 1px only
// rules out a "taller" that is really subpixel rounding on a fractional
// getBoundingClientRect.
const minGrowthPx = 1.0

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
func assertRevealed(t *testing.T, before, after footerState, claimID, how string) {
	t.Helper()
	t.Logf("%s %s:\n  before: %s\n  after:  %s", claimID, how, before, after)

	if after.ContentVisibility != "visible" {
		t.Errorf("%s %s: ::details-content must compute to visible, got %s\n"+
			"(the reveal must target the UA pseudo-element — setting content-visibility on the details' CHILDREN is a no-op, "+
			"a descendant cannot opt out of an ancestor the UA already skipped)",
			claimID, how, after)
	}
	if !(after.Height > before.Height+minGrowthPx) {
		t.Errorf("%s %s: the details box must be strictly taller once revealed, got %.1fpx after vs %.1fpx before\n"+
			"(a 'visible' computed value that does not grow the box means the contents are still not laid out)",
			claimID, how, after.Height, before.Height)
	}
	if after.Open {
		t.Errorf("%s %s: the revealed footer must NOT carry the `open` content attribute — "+
			"the reveal is CSS-only by decision C9 (a fragment is unknowable server-side and JS would not survive the static file:// export), "+
			"and CSS cannot write `open`. Got %s", claimID, how, after)
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

	// Follow the deep link. Poll rather than assert immediately: the hash write
	// and the style/layout it triggers are not the same task.
	runCDP(t, ctx, chromedp.Evaluate(fmt.Sprintf(`window.location.hash = %q;`, revealDeepID), nil))
	pollTrue(t, ctx, fmt.Sprintf(`(function () {
		var d = document.getElementById(%q).querySelector('details.claim-links');
		return getComputedStyle(d, '::details-content').contentVisibility === 'visible';
	})()`, revealDeepID))

	after := readFooter(t, ctx, revealDeepID)
	assertRevealed(t, before, after, revealDeepID, "after location.hash")

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
	runCDP(t, ctx, emulation.SetEmulatedMedia().WithMedia("print"))
	pollTrue(t, ctx, `window.matchMedia('print').matches`)

	deepAfter := readFooter(t, ctx, revealDeepID)
	assertRevealed(t, deepBefore, deepAfter, revealDeepID, "under print media")

	// Unlike :target, print is not scoped to one claim — every footer on the
	// printed facet is revealed, which is the whole point (a printed page is the
	// one surface with no way to click a disclosure open).
	baseAfter := readFooter(t, ctx, revealBaseID)
	assertRevealed(t, baseBefore, baseAfter, revealBaseID, "under print media")
}
