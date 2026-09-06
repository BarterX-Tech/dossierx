package viewertests

// THE SOURCE-NOTE THREE-LINE CLAMP.
//
// A source's supports/does_not_support line is the one field in the claim
// footer with no natural length — an author quoting or paraphrasing the page
// they cited — so a claim with several long ones buries the evidence it exists
// to present. Each note is clamped to three lines with a show more/show less
// control, and the control appears ONLY on the notes that actually run past
// three lines: a "show more" on a one-line note is the clutter the clamp was
// added to remove.
//
// WHY THIS SUITE READS PIXELS AND NOT PROPERTIES. Whether a note overflows is a
// fact about the rendered box, so the server cannot decide it and no Go test
// can either — components/sources.go ships every note whole with its control
// carrying the `hidden` attribute, and the viewer's script measures and reveals.
// That split is also where the feature's one shipped defect lived, and it is
// the reason for the assertion style here:
//
//	`hidden` IS NOT A PAINT FLAG. It is a user-agent stylesheet rule,
//	[hidden] { display: none }, and ANY author rule setting `display` on the
//	same element beats the UA origin. .claim-source-note-toggle sets
//	display: inline-block, which silently re-showed every control the script
//	had correctly hidden. Nothing in the DOM reported it: the attribute was
//	present, button.hidden read true, and only the pixels disagreed.
//
// So every "is it there?" assertion below reads getComputedStyle().display.
// A test that asserted .hidden would have passed against the broken build.
//
// The numbers are never pinned — they move with the machine's fonts. What is
// pinned is the RELATIONSHIP: clamped is shorter than its own content, expanded
// is not, and the clamp is three lines because the stylesheet says three.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const clampConfig = `schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
tracks:
  - id: checkout
    title: Checkout
`

// longNote is the note that must overflow. It is written to need EIGHT lines or
// so rather than four, and the margin is the whole point of the rewrite.
//
// WHAT WENT WRONG WITH THE FIRST VERSION, because the failure was expensive to
// read. It needed exactly four lines against a three-line clamp: one line of
// slack. Font metrics are not portable — this suite's stack starts at
// -apple-system/system-ui, which resolves to a different face on every OS the
// CI matrix runs — and on the Linux runner the same string fit in three. The
// script then did exactly the right thing (no overflow, so no clamp and no
// control) and all four tests in this file failed, reporting the FEATURE as
// broken when the fixture was.
//
// So the number to keep is the ratio, not the character count: whatever this
// string is, it must need enough lines that no plausible font drops it to
// three. requireOverflowingFixture below asserts that on every run, so the next
// person to shorten it gets told which thing broke.
const longNote = "The provider documents at-least-once delivery for transactional mail, with de-duplication keyed on an idempotency header the sender supplies, retained for twenty-four hours after the first accepted submission; messages submitted with the same key inside that window are accepted and discarded rather than rejected, so a caller that retries after a timeout cannot tell from the response whether the first attempt landed. The same page describes the retry schedule as exponential with jitter, bounded at six attempts over roughly seventy-two hours, after which the message is recorded as permanently failed and surfaced on the delivery webhook rather than in the submission response. It says nothing about ordering between distinct keys, and nothing about what happens to a key reused after the retention window has closed, which are the two questions this claim would most like answered."

// sourceNoteProbeScript wraps only the browser's ResizeObserver constructor and
// records its deliveries. It does not change the callback, entries, or page
// state; the positive assertions below read the same geometry and state a
// reader sees after the real observer has delivered. Keeping the probe at the
// new-document boundary also covers both file:// and served documents, plus a
// refresh, instead of measuring a warmed document after the fact.
const sourceNoteProbeScript = `(function () {
  var Native = window.ResizeObserver;
  window.__sourceNoteProbe = { hadObserver: typeof Native === 'function', deliveries: [], states: [] };
  if (typeof Native !== 'function') { return; }
  function Probe(callback) {
    return new Native(function (entries, observer) {
      var rows = [];
      for (var i = 0; i < entries.length; i++) {
        var target = entries[i].target;
        var rect = target.getBoundingClientRect();
        rows.push({
          className: target.className,
          width: target.clientWidth,
          height: target.clientHeight,
          rectWidth: rect.width,
          rectHeight: rect.height
        });
      }
      window.__sourceNoteProbe.deliveries.push(rows);
      return callback(entries, observer);
    });
  }
  Probe.prototype = Native.prototype;
  window.ResizeObserver = Probe;
})();`

const sourceNoteNoObserverScript = `(function () {
  window.__sourceNoteProbe = { hadObserver: false, deliveries: [], states: [] };
  window.ResizeObserver = undefined;
})();`

type sourceNoteRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type sourceNoteState struct {
	Stage      string         `json:"stage"`
	Class      string         `json:"class"`
	Label      string         `json:"label"`
	Aria       string         `json:"aria"`
	Display    string         `json:"display"`
	Active     string         `json:"active"`
	Hit        string         `json:"hit"`
	BodyClient int            `json:"bodyClient"`
	BodyScroll int            `json:"bodyScroll"`
	BodyWidth  int            `json:"bodyWidth"`
	Deliveries int            `json:"deliveries"`
	NoteRect   sourceNoteRect `json:"noteRect"`
	ButtonRect sourceNoteRect `json:"buttonRect"`
}

func installSourceNoteProbe(t *testing.T, ctx context.Context, noObserver bool) {
	t.Helper()
	script := sourceNoteProbeScript
	if noObserver {
		script = sourceNoteNoObserverScript
	}
	runCDP(t, ctx, chromedp.ActionFunc(func(c context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(script).Do(c)
		return err
	}))
}

func settleSourceNoteProbe(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.Evaluate(
		`new Promise(function (resolve) { requestAnimationFrame(function () { requestAnimationFrame(resolve); }); })`,
		nil,
		func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) },
	))
}

func sourceNoteStateAt(t *testing.T, ctx context.Context, note, stage string) sourceNoteState {
	t.Helper()
	var state sourceNoteState
	evalInto(t, ctx, fmt.Sprintf(`(function () {
		var n = %s;
		var b = n.querySelector('.claim-source-note-body');
		var btn = n.querySelector('.claim-source-note-toggle');
		var nr = n.getBoundingClientRect();
		var br = btn.getBoundingClientRect();
		var hit = document.elementFromPoint(br.left + br.width / 2, br.top + br.height / 2);
		var p = window.__sourceNoteProbe || { deliveries: [], states: [] };
		var state = {
			stage: %q,
			class: n.className,
			label: btn.textContent,
			aria: btn.getAttribute('aria-expanded'),
			display: getComputedStyle(btn).display,
			active: document.activeElement ? document.activeElement.tagName + '.' + document.activeElement.className : '',
			hit: hit ? hit.tagName + '.' + hit.className : '',
			bodyClient: b.clientHeight,
			bodyScroll: b.scrollHeight,
			bodyWidth: b.clientWidth,
			deliveries: p.deliveries.length,
			noteRect: { x: nr.x, y: nr.y, width: nr.width, height: nr.height },
			buttonRect: { x: br.x, y: br.y, width: br.width, height: br.height }
		};
		p.states.push(state);
		return state;
	})()`, note, stage), &state)
	return state
}

func assertSourceNoteGeometry(t *testing.T, state sourceNoteState) {
	t.Helper()
	if state.NoteRect.Width <= 0 || state.NoteRect.Height <= 0 || state.ButtonRect.Width <= 0 || state.ButtonRect.Height <= 0 {
		t.Fatalf("%s: note/button geometry is not laid out: %+v", state.Stage, state)
	}
	if state.BodyClient <= 0 || state.BodyWidth <= 0 {
		t.Fatalf("%s: body geometry is not laid out: %+v", state.Stage, state)
	}
	if state.Deliveries == 0 {
		t.Fatalf("%s: no ResizeObserver delivery was recorded: %+v", state.Stage, state)
	}
	if state.Hit == "" || !strings.Contains(state.Hit, "claim-source-note-toggle") {
		t.Fatalf("%s: elementFromPoint did not hit the visible note control: %+v", state.Stage, state)
	}
}

func assertSourceNoteCollapsed(t *testing.T, state sourceNoteState, height int) {
	t.Helper()
	assertSourceNoteGeometry(t, state)
	if !strings.Contains(state.Class, "is-clamped") || state.Aria != "false" || state.Label != "show more" {
		t.Fatalf("%s: collapsed state drifted: %+v", state.Stage, state)
	}
	if state.BodyClient != height || state.BodyScroll <= state.BodyClient {
		t.Fatalf("%s: collapsed geometry = client %d scroll %d, want client %d and overflow", state.Stage, state.BodyClient, state.BodyScroll, height)
	}
}

func assertSourceNoteExpanded(t *testing.T, state sourceNoteState, collapsedHeight int) {
	t.Helper()
	assertSourceNoteGeometry(t, state)
	if strings.Contains(state.Class, "is-clamped") || state.Aria != "true" || state.Label != "show less" {
		t.Fatalf("%s: expanded state drifted: %+v", state.Stage, state)
	}
	if state.BodyClient <= collapsedHeight || state.BodyScroll != state.BodyClient {
		t.Fatalf("%s: expanded geometry = client %d scroll %d, want client > %d and no overflow", state.Stage, state.BodyClient, state.BodyScroll, collapsedHeight)
	}
}

func clampTabWithProbe(t *testing.T, p *project, noObserver bool) context.Context {
	t.Helper()
	url := p.renderStatic()
	ctx := browserContext(t)
	installSourceNoteProbe(t, ctx, noObserver)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Evaluate(
		`document.querySelectorAll('details.claim-links').forEach(function (d) { d.open = true; })`, nil))
	settleFor(t, ctx, noteDecidedExpr)
	if !noObserver {
		settleFor(t, ctx, `getComputedStyle(document.querySelectorAll('.claim-source-note')[0].querySelector('.claim-source-note-toggle')).display !== 'none'`)
	}
	settleSourceNoteProbe(t, ctx)
	return ctx
}

// newClampProject writes one claim carrying two sources: the first has a note
// that overflows and a second note that does not, the second has one short note.
// Three notes, of which exactly ONE earns a control.
//
// The claim OWNS a track, so the viewer renders it a second time inside that
// track's section. That copy is what TestSourceNoteClampWorksInATrackCopy reads:
// the control carries no id precisely so it can be duplicated, and nothing else
// in the suite would notice if it gained one.
func newClampProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, clampConfig)
	p.writeClaim("c.yaml", `id: widget.contract.notes
facet: contract
module: widget
status: draft
tracks:
  - id: checkout
    role: owns
body: |
  one [1] two [2].
sources:
  - ref: 1
    kind: external
    title: Long
    url: https://example.test/long
    accessed_on: 2026-01-02
    supports: "`+longNote+`"
    does_not_support: "One short line."
  - ref: 2
    kind: external
    title: Short
    url: https://example.test/short
    accessed_on: 2026-01-02
    supports: "One line."
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	return p
}

// clampTab renders the project statically, opens it, and opens the claim
// footers so the notes have a laid-out box to be measured in. The footers are
// opened through the `open` PROPERTY rather than by clicking the summary
// because this suite is about the notes inside them, not about the disclosure —
// reveal_test.go owns that.
func clampTab(t *testing.T, p *project) context.Context {
	t.Helper()
	url := p.renderStatic()
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.Navigate(url))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Evaluate(
		`document.querySelectorAll('details.claim-links').forEach(function (d) { d.open = true; })`, nil))
	// The script decides on a ResizeObserver delivery, which lands after the
	// footers gain a box. Settling on the OUTCOME rather than on a timer means
	// this waits exactly as long as the decision takes.
	settleFor(t, ctx, noteDecidedExpr)
	return ctx
}

// noteDecidedExpr is true once every note has been judged: the script mounts
// them all clamped and then REMOVES the clamp from the ones that fit, so "some
// note is no longer clamped" is the earliest honest signal that a delivery has
// happened. Only ever passed to settleFor — a false result ends a bounded wait
// rather than the test.
const noteDecidedExpr = `document.querySelectorAll('.claim-source-note:not(.is-clamped)').length > 0`

// noteAt returns a JS expression for the nth .claim-source-note in the document,
// scoped to one root so a track copy can be addressed separately from the
// canonical card.
func noteAt(root string, i int) string {
	return fmt.Sprintf(`%s.querySelectorAll('.claim-source-note')[%d]`, root, i)
}

// controlPaints reads the one thing that matters and the one thing the broken
// build got wrong: does the reader SEE a control. Never button.hidden — see the
// file header.
func controlPaints(t *testing.T, ctx context.Context, note string) bool {
	t.Helper()
	return evalBool(t, ctx, fmt.Sprintf(
		`getComputedStyle(%s.querySelector('.claim-source-note-toggle')).display !== 'none'`, note))
}

func noteOverflows(t *testing.T, ctx context.Context, note string) bool {
	t.Helper()
	return evalBool(t, ctx, fmt.Sprintf(`(function () {
		var b = %s.querySelector('.claim-source-note-body');
		return b.scrollHeight > b.clientHeight + 1;
	})()`, note))
}

// requireOverflowingFixture asserts the premise every test here rests on: the
// note this suite calls "long" really does run past three lines in THIS
// browser. Without it, a fixture that stops overflowing — a shorter string, a
// wider box, a narrower font — reports as four failures about a working clamp,
// which is what sent the first version of this suite red on Linux while it
// passed on macOS.
//
// It measures with the clamp forced on and restores whatever was there, so it
// can be called before the assertions without deciding them.
func requireOverflowingFixture(t *testing.T, ctx context.Context, note string) {
	t.Helper()
	var lines int
	runCDP(t, ctx, chromedp.Evaluate(`(function () {
		var n = `+note+`;
		var b = n.querySelector('.claim-source-note-body');
		var was = n.classList.contains('is-clamped');
		n.classList.remove('is-clamped');
		var need = Math.round(b.scrollHeight / parseFloat(getComputedStyle(b).lineHeight));
		if (was) { n.classList.add('is-clamped'); }
		return need;
	})()`, &lines))
	if lines <= 3 {
		t.Fatalf("the fixture note renders in %d line(s) in this browser, so it does not overflow the three-line clamp and there is nothing here to test.\n\n"+
			"This is a defect in longNote, NOT in the clamp: the script is correct to leave a note that fits unclamped and uncontrolled. Lengthen longNote until it needs well more than three lines on every font the matrix runs.", lines)
	}
}

// TestSourceNoteClampControlsOnlyTheNotesThatNeedOne is the whole point of the
// feature: the long note is cut and offers a way out, and the two short ones are
// left exactly as they were. A build that clamped everything, or that offered
// every note a control, passes neither half.
func TestSourceNoteClampControlsOnlyTheNotesThatNeedOne(t *testing.T) {
	ctx := clampTab(t, newClampProject(t))
	doc := "document"

	long := noteAt(doc, 0)
	requireOverflowingFixture(t, ctx, long)
	if !evalBool(t, ctx, long+`.classList.contains('is-clamped')`) {
		t.Error("the overflowing note is not clamped")
	}
	if !controlPaints(t, ctx, long) {
		t.Error("the overflowing note offers no control, so its text is cut with no way to read it")
	}
	if !noteOverflows(t, ctx, long) {
		t.Error("the clamped note does not report overflow; the clamp is not in effect")
	}

	// The two short notes: one beside the long one under the same source, one
	// under a different source. Both must be untouched.
	for _, i := range []int{1, 2} {
		short := noteAt(doc, i)
		if evalBool(t, ctx, short+`.classList.contains('is-clamped')`) {
			t.Errorf("note %d fits in three lines but stayed clamped", i)
		}
		if controlPaints(t, ctx, short) {
			t.Errorf("note %d fits in three lines and still paints a control", i)
		}
	}
}

// TestSourceNoteClampIsThreeLines pins the number the request was written
// around, at the only place it is stated. Reading the computed property rather
// than a pixel height keeps this true on any machine's fonts.
func TestSourceNoteClampIsThreeLines(t *testing.T) {
	ctx := clampTab(t, newClampProject(t))
	requireOverflowingFixture(t, ctx, noteAt("document", 0))
	got := evalString(t, ctx, `getComputedStyle(`+noteAt("document", 0)+`.querySelector('.claim-source-note-body')).webkitLineClamp`)
	if got != "3" {
		t.Fatalf("line clamp = %q, want \"3\"", got)
	}
}

// TestSourceNoteControlExpandsAndCollapses walks the control the way a reader
// does, and asserts the height moved in the direction the label promised. The
// label itself is checked too, because a control whose text still says "show
// more" after expanding is telling the reader the opposite of what it did.
func TestSourceNoteControlExpandsAndCollapses(t *testing.T) {
	ctx := clampTabWithProbe(t, newClampProject(t), false)
	long := noteAt("document", 0)
	requireOverflowingFixture(t, ctx, long)

	initial := sourceNoteStateAt(t, ctx, long, "initial after observer")
	clamped := initial.BodyClient
	assertSourceNoteCollapsed(t, initial, clamped)

	runCDP(t, ctx, chromedp.Click(".claim-source-note-toggle", chromedp.ByQuery))
	settleSourceNoteProbe(t, ctx)
	expanded := sourceNoteStateAt(t, ctx, long, "mouse expanded after observer")
	assertSourceNoteExpanded(t, expanded, clamped)
	if noteOverflows(t, ctx, long) {
		t.Error("the expanded note still overflows; some of the citation is still hidden")
	}

	runCDP(t, ctx, chromedp.Click(".claim-source-note-toggle", chromedp.ByQuery))
	settleSourceNoteProbe(t, ctx)
	collapsed := sourceNoteStateAt(t, ctx, long, "mouse collapsed after observer")
	assertSourceNoteCollapsed(t, collapsed, clamped)

	// Keyboard activation exercises the same delegated handler through the
	// browser's native button semantics, rather than calling click() in JS.
	runCDP(t, ctx, chromedp.SendKeys(".claim-source-note-toggle", "\n", chromedp.ByQuery))
	settleSourceNoteProbe(t, ctx)
	keyboardExpanded := sourceNoteStateAt(t, ctx, long, "keyboard expanded after observer")
	assertSourceNoteExpanded(t, keyboardExpanded, clamped)
	runCDP(t, ctx, chromedp.SendKeys(".claim-source-note-toggle", " ", chromedp.ByQuery))
	settleSourceNoteProbe(t, ctx)
	keyboardCollapsed := sourceNoteStateAt(t, ctx, long, "keyboard collapsed after observer")
	assertSourceNoteCollapsed(t, keyboardCollapsed, clamped)
}

// TestSourceNoteControlWorksOverHTTPAndRefresh keeps the file:// proof above
// separate from the served path. The page is freshly instrumented before the
// first live navigation, then refreshed and driven by keyboard activation so a
// warm document cannot hide a load or re-mount regression.
func TestSourceNoteControlWorksOverHTTPAndRefresh(t *testing.T) {
	p := newClampProject(t)
	base := p.ensureServe()
	ctx := browserContext(t)
	installSourceNoteProbe(t, ctx, false)
	runCDP(t, ctx, chromedp.Navigate(base+"/"))
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Evaluate(
		`document.querySelectorAll('details.claim-links').forEach(function (d) { d.open = true; })`, nil))
	settleFor(t, ctx, noteDecidedExpr)
	settleFor(t, ctx, `getComputedStyle(document.querySelectorAll('.claim-source-note')[0].querySelector('.claim-source-note-toggle')).display !== 'none'`)
	settleSourceNoteProbe(t, ctx)
	long := noteAt("document", 0)
	initial := sourceNoteStateAt(t, ctx, long, "http initial after observer")
	assertSourceNoteCollapsed(t, initial, initial.BodyClient)

	runCDP(t, ctx, chromedp.Click(".claim-source-note-toggle", chromedp.ByQuery))
	settleSourceNoteProbe(t, ctx)
	assertSourceNoteExpanded(t, sourceNoteStateAt(t, ctx, long, "http mouse expanded after observer"), initial.BodyClient)

	runCDP(t, ctx, chromedp.Reload())
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Evaluate(
		`document.querySelectorAll('details.claim-links').forEach(function (d) { d.open = true; })`, nil))
	settleFor(t, ctx, noteDecidedExpr)
	settleFor(t, ctx, `getComputedStyle(document.querySelectorAll('.claim-source-note')[0].querySelector('.claim-source-note-toggle')).display !== 'none'`)
	settleSourceNoteProbe(t, ctx)
	long = noteAt("document", 0)
	refreshed := sourceNoteStateAt(t, ctx, long, "http refreshed after observer")
	assertSourceNoteCollapsed(t, refreshed, refreshed.BodyClient)
	runCDP(t, ctx, chromedp.SendKeys(".claim-source-note-toggle", "\n", chromedp.ByQuery))
	settleSourceNoteProbe(t, ctx)
	assertSourceNoteExpanded(t, sourceNoteStateAt(t, ctx, long, "http keyboard expanded after refresh"), refreshed.BodyClient)
}

// TestSourceNoteWithoutResizeObserverLeavesTextWhole pins the safe fallback:
// an engine with no observer keeps the complete note visible and hides the
// control, while the ordinary short-note path remains free of dead chrome.
func TestSourceNoteWithoutResizeObserverLeavesTextWhole(t *testing.T) {
	ctx := clampTabWithProbe(t, newClampProject(t), true)
	long := noteAt("document", 0)
	short := noteAt("document", 1)
	if evalBool(t, ctx, long+`.classList.contains('is-clamped')`) || noteOverflows(t, ctx, long) {
		t.Fatal("without ResizeObserver, the long note must remain whole and unclamped")
	}
	if controlPaints(t, ctx, long) || controlPaints(t, ctx, short) {
		t.Fatal("without ResizeObserver, source-note controls must remain hidden")
	}
	if evalBool(t, ctx, short+`.classList.contains('is-clamped')`) {
		t.Fatal("without ResizeObserver, the short note must remain unclamped")
	}
	if got := evalInt(t, ctx, `window.__sourceNoteProbe.deliveries.length`); got != 0 {
		t.Fatalf("the no-observer fixture recorded %d deliveries", got)
	}
}

// TestSourceNoteClampWorksInATrackCopy is the constraint that shaped the
// markup. A claim owned by a track is rendered a SECOND time inside that
// track's section with its element ids stripped, so the control cannot be wired
// by id — it finds its note through the parent it sits in. Both copies must
// therefore work, and independently: expanding one must leave the other shut.
//
// The two copies are read one at a time because the viewer shows one section at
// a time, and that is not incidental to the test — it is the second thing being
// asserted. A note in a section nobody has opened has never been laid out, so
// the script has nothing to measure and correctly leaves it alone; the control
// appears when the reader arrives, not before.
func TestSourceNoteClampWorksInATrackCopy(t *testing.T) {
	ctx := clampTab(t, newClampProject(t))

	if n := evalInt(t, ctx, `document.querySelectorAll('.claim-source-note').length`); n != 6 {
		t.Fatalf("notes in document = %d, want 6 — three per copy of the claim", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('.claim-source-note-toggle[id]').length`); n != 0 {
		t.Fatalf("%d note controls carry an id; a track copy would duplicate it", n)
	}

	canonical := noteAt(`document.querySelector('.module-section:not(.track-section)')`, 0)
	copied := noteAt(`document.querySelector('.track-section')`, 0)

	requireOverflowingFixture(t, ctx, canonical)

	// The canonical card is the one on screen, so it has been judged.
	if !controlPaints(t, ctx, canonical) {
		t.Fatal("the canonical note offers no control")
	}
	// The track copy has not: its section is hidden, and a box with no layout
	// is not a box this feature is willing to guess about.
	if controlPaints(t, ctx, copied) {
		t.Error("a note in a never-opened section was judged before it had a box")
	}

	// Arrive at the track. The copy gains a box, is measured, and earns the
	// same control — through DOM position alone, since it carries no id.
	runCDP(t, ctx, chromedp.Click(`.sec-tab[data-target="#track-checkout"]`, chromedp.ByQuery))
	runCDP(t, ctx, chromedp.Evaluate(
		`document.querySelectorAll('.track-section details.claim-links').forEach(function (d) { d.open = true; })`, nil))
	settleFor(t, ctx, `(function () {
		var n = document.querySelector('.track-section .claim-source-note');
		return !!n && getComputedStyle(n.querySelector('.claim-source-note-toggle')).display !== 'none';
	})()`)
	if !controlPaints(t, ctx, copied) {
		t.Fatal("the track copy's note offers no control once its section is open")
	}

	// Press it. The copy expands; the canonical card, still mounted in the
	// section behind, stays exactly as the reader left it.
	runCDP(t, ctx, chromedp.Evaluate(copied+`.querySelector('.claim-source-note-toggle').click()`, nil))
	if evalBool(t, ctx, copied+`.classList.contains('is-clamped')`) {
		t.Error("the track copy did not expand")
	}
	if !evalBool(t, ctx, canonical+`.classList.contains('is-clamped')`) {
		t.Error("expanding the track copy also expanded the canonical card")
	}
}
