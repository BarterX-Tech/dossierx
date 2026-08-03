package viewertests

// Table overflow suite — the browser half of #22 (a layout: table claim needs a
// scroll container) and #23 (its cells need a wrapping rule). The two are one
// sizing decision, not two independent fixes, so they are measured together:
//
//   - The wrapper alone does not stop a 65-character identifier taking most of
//     the table, it only stops the overflow escaping the card.
//   - overflow-wrap: anywhere alone would make the wrapper DEAD MARKUP: once
//     every cell can break at any character a six-column table's min-content
//     falls below the card, the table resolves to exactly 100% of the wrapper
//     and scrollWidth == clientWidth forever.
//   - The per-cell min-width floor is what keeps the scroller real, and it
//     scales with column count, so a two-column table is NOT forced into a
//     scrollbar it does not need.
//
// Each assertion below reports the two numbers it compared, never a bare
// boolean: "the table scrolled" and "the table scrolled by 122px" fail very
// differently when a future CSS change moves the floor.
//
// The fixtures are three claims written into a throwaway project by this file
// (newProjectRaw + writeClaim + renderStatic), so no committed testdata viewer
// is perturbed and nothing here has to be regenerated with them.

import (
	"context"
	"fmt"
	"testing"

	"github.com/chromedp/chromedp"
)

// wideIdent is 65 characters with no space, hyphen or underscore in it, so a
// browser has no soft-wrap opportunity in it at all unless overflow-wrap gives
// it one. That is the whole shape of the bug: under auto table layout the
// column's min-content is the full run. It is spelled in caps (a constant name,
// the commonest real-world shape of a run this long) because the "without the
// rules" measurement below asserts a RATIO, and caps put the run's share of the
// table comfortably clear of the 80% threshold rather than a hair under it.
const wideIdent = "REPOSITORYCONNECTIONPOOLEXHAUSTIONRETRYBACKOFFCOEFFICIENTMILLISXY"

// tableWideClaim is the six-column offender. Columns b..f hold single
// characters so that WITHOUT the cell rules column a is the overwhelming
// majority of the table — which is the state issue #23 reports.
const tableWideClaim = `id: widget.contract.table-wide
facet: contract
module: widget
status: draft
layout: table
rows:
  - a: ` + wideIdent + `
    b: "1"
    c: "1"
    d: "1"
    e: "1"
    f: "1"
  - a: "2"
    b: "2"
    c: "2"
    d: "2"
    e: "2"
    f: "2"
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// tableNarrowClaim is the control: two columns of ordinary prose-length values.
// The 5rem cell floor must leave it fitting inside the card.
const tableNarrowClaim = `id: widget.contract.table-narrow
facet: contract
module: widget
status: draft
layout: table
rows:
  - name: alpha
    note: a short ordinary value
  - name: beta
    note: another ordinary value
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// bodyMDTableClaim is an ordinary card claim whose BODY holds a GFM pipe table.
// It renders as .md-table, not as a claim table, and it is the fixture that
// makes the "body tables keep scrolling rather than wrapping" decision
// checkable: a .md-table is never inside the claim-table scroll wrapper, so the
// cell rules scoped to that wrapper must not reach it.
const bodyMDTableClaim = `id: widget.contract.body-mdtable
facet: contract
module: widget
status: draft
layout: card
body: |
  A pipe table written in a claim body is prose furniture, not a record
  listing, and it stays contained by scrolling rather than by wrapping.

  | col | value |
  | --- | --- |
  | one | ` + wideIdent + ` |
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// tableFixtureTab writes the three fixture claims into a throwaway project,
// renders the static viewer, and opens it in a tab emulating the given
// viewport. The wait is on the wrapper's own table so a missing wrapper fails
// here, loudly, rather than as a null dereference inside a measurement.
func tableFixtureTab(t *testing.T, w, h int64) context.Context {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("table-wide.yaml", tableWideClaim)
	p.writeClaim("table-narrow.yaml", tableNarrowClaim)
	p.writeClaim("body-mdtable.yaml", bodyMDTableClaim)
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(w, h),
		chromedp.Navigate(url),
		chromedp.WaitVisible(".claim-table-scroll table", chromedp.ByQuery),
	)
	return ctx
}

// evalFloats reads a JS expression yielding an array of numbers. Widths are
// fractional (getBoundingClientRect, and scrollWidth on a scaled viewport), so
// evalInt would silently truncate the very quantities being compared.
func evalFloats(t *testing.T, ctx context.Context, expr string) []float64 {
	t.Helper()
	var v []float64
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &v)); err != nil {
		t.Fatalf("evaluate %q: %v", expr, err)
	}
	return v
}

// scrollAndClient returns [scrollWidth, clientWidth] of the scroll wrapper
// inside the named claim.
func scrollAndClient(t *testing.T, ctx context.Context, claimID string) (scroll, client float64) {
	t.Helper()
	got := evalFloats(t, ctx, fmt.Sprintf(`(function () {
		var w = document.getElementById(%q).querySelector('.claim-table-scroll');
		return [w.scrollWidth, w.clientWidth];
	})()`, claimID))
	if len(got) != 2 {
		t.Fatalf("%s: expected [scrollWidth, clientWidth], got %v", claimID, got)
	}
	return got[0], got[1]
}

// assertPageDoesNotScrollSideways is the backstop the wrapper exists to
// provide: whatever a table does inside its own box, the document must never
// grow a horizontal scrollbar.
func assertPageDoesNotScrollSideways(t *testing.T, ctx context.Context) {
	t.Helper()
	got := evalFloats(t, ctx, `[document.scrollingElement.scrollWidth, document.scrollingElement.clientWidth]`)
	if len(got) != 2 {
		t.Fatalf("expected [scrollWidth, clientWidth] for the scrolling element, got %v", got)
	}
	if got[0] > got[1] {
		t.Errorf("the page scrolls horizontally: document scrollWidth = %v, clientWidth = %v", got[0], got[1])
	}
}

// ---------------------------------------------------------------------
// #22 — the wrapper is real: a too-wide claim table scrolls inside it, and
// an ordinary one does not.
// ---------------------------------------------------------------------

func TestClaimTableScrollsInsideWrapperAtMobileWidth(t *testing.T) {
	ctx := tableFixtureTab(t, 390, 800)

	// The wrapper and its table both exist in the rendered viewer.
	if !evalBool(t, ctx, `document.querySelector('.claim-table-scroll table') !== null`) {
		t.Fatal("expected a table inside a .claim-table-scroll wrapper in the rendered viewer")
	}

	// The six-column table overflows its card and scrolls in its own box.
	scroll, client := scrollAndClient(t, ctx, "widget.contract.table-wide")
	if !(scroll > client) {
		t.Errorf("table-wide at 390px must scroll inside its wrapper: scrollWidth = %v, clientWidth = %v", scroll, client)
	}

	// The two-column table is left alone: the per-cell floor must not invent a
	// scrollbar for a table that fits.
	scroll, client = scrollAndClient(t, ctx, "widget.contract.table-narrow")
	if scroll != client {
		t.Errorf("table-narrow at 390px must not scroll: scrollWidth = %v, clientWidth = %v", scroll, client)
	}

	assertPageDoesNotScrollSideways(t, ctx)
}

func TestClaimTableDoesNotScrollAtDesktopWidth(t *testing.T) {
	ctx := tableFixtureTab(t, 1280, 900)

	scroll, client := scrollAndClient(t, ctx, "widget.contract.table-wide")
	if scroll != client {
		t.Errorf("table-wide at 1280px must not scroll (no gratuitous scrollbar on a wide screen): scrollWidth = %v, clientWidth = %v", scroll, client)
	}

	assertPageDoesNotScrollSideways(t, ctx)
}

// ---------------------------------------------------------------------
// #23 — the cell rules are what stop one identifier eating its column.
// Measured TWICE on the same rendered page: the "without" state is produced
// by overriding the two declarations inline (inline styles beat any stylesheet
// rule regardless of specificity), never by rendering a different build, since
// a build without the wrapper has no .claim-table-scroll to measure at all.
// ---------------------------------------------------------------------

func TestClaimTableCellRulesShrinkTheOffendingColumn(t *testing.T) {
	ctx := tableFixtureTab(t, 390, 800)

	got := evalFloats(t, ctx, `(function () {
		var wrap = document.getElementById('widget.contract.table-wide')
		                   .querySelector('.claim-table-scroll');
		var cells = function () {
			return [].slice.call(wrap.querySelectorAll('tr:first-child > *'))
			         .map(function (c) { return c.getBoundingClientRect().width; });
		};
		var tw = function () {
			return wrap.querySelector('table').getBoundingClientRect().width;
		};
		var withRules = cells()[0] / tw();

		wrap.querySelectorAll('td, th').forEach(function (c) {
			c.style.overflowWrap = 'normal';
			c.style.minWidth = '0';
		});
		void document.body.offsetHeight;                 // force reflow
		var without = cells()[0] / tw();

		return [withRules, without];
	})()`)
	if len(got) != 2 {
		t.Fatalf("expected [withRules, without], got %v", got)
	}
	withRules, without := got[0], got[1]
	// Logged on success too: these two ratios are the whole evidence for the
	// keyword choice, and a future change that erodes the margin should be
	// visible in `go test -v` before it becomes a failure.
	t.Logf("widest column / table width: with the cell rules %.4f, without %.4f", withRules, without)

	if !(withRules < 0.60) {
		t.Errorf("with the cell rules the widest column must be under 60%% of the table, got %.4f (without = %.4f)", withRules, without)
	}
	if !(without > 0.80) {
		t.Errorf("without the cell rules the widest column should be over 80%% of the table — the bug #23 reports — got %.4f (with = %.4f)", without, withRules)
	}
}

// ---------------------------------------------------------------------
// The scoping decision: the cell rules are scoped to the claim-table wrapper,
// so a pipe table in a claim BODY keeps its existing containment strategy —
// width: max-content + overflow-x: auto, i.e. it SCROLLS. If the rules were
// written unscoped (`td, th { ... }`) they would cascade into .md-table cells
// and this table would wrap instead, silently narrowing a documented
// content-sizing distinction.
// ---------------------------------------------------------------------

func TestBodyMarkdownTableStillScrolls(t *testing.T) {
	ctx := tableFixtureTab(t, 390, 800)

	if !evalBool(t, ctx, `document.querySelector('.claim-body .md-table') !== null`) {
		t.Fatal("the body-mdtable fixture did not render a .md-table inside a claim body")
	}
	got := evalFloats(t, ctx, `(function () {
		var t = document.querySelector('.claim-body .md-table');
		return [t.scrollWidth, t.clientWidth];
	})()`)
	if len(got) != 2 {
		t.Fatalf("expected [scrollWidth, clientWidth], got %v", got)
	}
	if !(got[0] > got[1]) {
		t.Errorf("a body markdown table must still SCROLL rather than wrap: scrollWidth = %v, clientWidth = %v", got[0], got[1])
	}

	assertPageDoesNotScrollSideways(t, ctx)
}
