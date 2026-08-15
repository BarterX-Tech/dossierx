package viewertests

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// chromedp helpers — every browser wait is deterministic (Poll / WaitVisible),
// never a fixed sleep, so the suite is safe under -count=2.
// ---------------------------------------------------------------------

func runCDP(t *testing.T, ctx context.Context, actions ...chromedp.Action) {
	t.Helper()
	if err := chromedp.Run(ctx, actions...); err != nil {
		t.Fatalf("chromedp.Run: %v", err)
	}
}

// pollTrue waits until a JavaScript boolean expression becomes true, failing the
// test (with the expression) if it does not within the timeout.
func pollTrue(t *testing.T, ctx context.Context, expr string) {
	t.Helper()
	var ok bool
	err := chromedp.Run(ctx, chromedp.Poll(expr, &ok,
		chromedp.WithPollingInterval(40*time.Millisecond),
		chromedp.WithPollingTimeout(20*time.Second),
	))
	if err != nil {
		t.Fatalf("condition never became true within timeout:\n  %s\n  err: %v", expr, err)
	}
}

func evalBool(t *testing.T, ctx context.Context, expr string) bool {
	t.Helper()
	var v bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &v)); err != nil {
		t.Fatalf("evaluate %q: %v", expr, err)
	}
	return v
}

func evalInt(t *testing.T, ctx context.Context, expr string) int {
	t.Helper()
	var v int
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &v)); err != nil {
		t.Fatalf("evaluate %q: %v", expr, err)
	}
	return v
}

func waitVisible(t *testing.T, ctx context.Context, sel string) {
	t.Helper()
	runCDP(t, ctx, chromedp.WaitVisible(sel, chromedp.ByQuery))
}

// newLiveTab starts serve for p, opens a fresh browser tab pointed at it, and
// waits until the reachability probe has mounted the write controls
// (body.comments-live). The chip is present and visible on return.
func newLiveTab(t *testing.T, p *project) context.Context {
	t.Helper()
	// ensureServe, not serve: a test may already have started the server to
	// reach the HTTP API (resolveViaAPI) before opening a tab, and a second
	// serve process on the same project directory would race the first.
	base := p.ensureServe()
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(".comment-chip", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.body.classList.contains('comments-live')`)
	return ctx
}

// openPanelLive clicks the (single) chip and waits for the live composer to
// mount inside the rail.
func openPanelLive(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.Click(".comment-chip", chromedp.ByQuery))
	waitVisible(t, ctx, "#commentsPanel .comment-composer .comment-composer-input")
}

// ---------------------------------------------------------------------
// Probe: mounts controls against a live serve; NOT on a static file://
// ---------------------------------------------------------------------

func TestServeProbeMountsComposer(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "seed thread")
	ctx := newLiveTab(t, p)

	// Before opening the panel: the probe has flipped comments-live on.
	if !evalBool(t, ctx, `document.body.classList.contains('comments-live')`) {
		t.Fatal("live serve must mount the runtime (comments-live)")
	}
	openPanelLive(t, ctx)
	if evalInt(t, ctx, `document.querySelectorAll('#commentsPanel .comment-composer .comment-composer-input').length`) != 1 {
		t.Fatal("expected exactly one live composer textarea in the panel")
	}
	// The rail carries a complementary/dialog role and the chip is expanded.
	if !evalBool(t, ctx, `['complementary','dialog'].indexOf(document.getElementById('commentsPanel').getAttribute('role')) >= 0`) {
		t.Fatal("rail must expose a complementary/dialog role")
	}
	if !evalBool(t, ctx, `document.querySelector('.comment-chip').getAttribute('aria-expanded') === 'true'`) {
		t.Fatal("chip aria-expanded must be true while its panel is open")
	}
}

func TestFileURLStaysReadOnly(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "a baked thread")
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(".comment-chip", chromedp.ByQuery),
	)
	// Open the panel; on file:// the probe cannot reach a server, so the panel is
	// a read-only clone of the baked threads and never mounts controls.
	runCDP(t, ctx, chromedp.Click(".comment-chip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comment-thread').length > 0`)

	if evalBool(t, ctx, `document.body.classList.contains('comments-live')`) {
		t.Fatal("a static file:// viewer must never mount the runtime")
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('#commentsPanel .comment-composer, #commentsPanel .comment-reply-composer, #commentsPanel textarea, #commentsPanel .comment-action').length`); n != 0 {
		t.Fatalf("file:// viewer must mount no composer/action controls, found %d", n)
	}
	// The baked discussion is still readable.
	if !evalBool(t, ctx, `Array.from(document.querySelectorAll('#commentsPanel .comment-body')).some(function(b){return b.textContent.indexOf('a baked thread') >= 0})`) {
		t.Fatal("read-only panel should show the baked comment body")
	}
}

// ---------------------------------------------------------------------
// Add / resolve / reply / edit / delete through the UI
// ---------------------------------------------------------------------

func TestUIAddCommentIncrementsChip(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "first thread")
	ctx := newLiveTab(t, p)

	// Chip starts at one open thread.
	pollTrue(t, ctx, `document.querySelector('.comment-chip .comment-chip-count').textContent === '1'`)
	openPanelLive(t, ctx)

	runCDP(t, ctx,
		chromedp.SendKeys("#commentsPanel .comment-composer .comment-composer-input", "a second thread", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-composer .comment-composer-submit", chromedp.ByQuery),
	)
	// Wait for the authoritative (post-round-trip) state: two non-optimistic
	// threads in the panel — which also proves the write reached the server.
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread:not(.comment-thread--optimistic)').length === 2`)
	pollTrue(t, ctx, `document.querySelector('.comment-chip .comment-chip-count').textContent === '2'`)

	if !bytes.Contains(p.claimBytes(), []byte("a second thread")) {
		t.Fatalf("the added comment was not persisted to the claim file:\n%s", p.claimBytes())
	}
}

func TestUIResolveCollapsesThreadAndUpdatesChip(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "resolve me")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	runCDP(t, ctx,
		chromedp.WaitVisible("#commentsPanel .comment-resolve", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-resolve", chromedp.ByQuery),
	)
	// The thread collapses into the resolved <details> and the chip flips to the
	// muted resolved style with the total count.
	pollTrue(t, ctx, `!!document.querySelector('#commentsPanel .comments-resolved .comment-thread')`)
	pollTrue(t, ctx, `document.querySelector('.comment-chip').classList.contains('comment-chip--resolved')`)
	if evalInt(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread').length`) != 0 {
		t.Fatal("no thread should remain open after resolve")
	}
	if !bytes.Contains(p.claimBytes(), []byte("status: resolved")) {
		t.Fatalf("resolve was not persisted:\n%s", p.claimBytes())
	}
}

func TestUIReplyAppearsAndPersists(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "parent thread")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	runCDP(t, ctx,
		chromedp.WaitVisible("#commentsPanel .comment-reply-composer .comment-composer-input", chromedp.ByQuery),
		chromedp.SendKeys("#commentsPanel .comment-reply-composer .comment-composer-input", "a considered reply", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-reply-composer .comment-composer-submit", chromedp.ByQuery),
	)
	// Gate on the authoritative (non-optimistic) reply so the server round-trip
	// has completed and the reply is on disk.
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comment-thread .comment-reply:not(.comment-reply--optimistic)').length >= 1`)
	if !bytes.Contains(p.claimBytes(), []byte("a considered reply")) {
		t.Fatalf("reply was not persisted:\n%s", p.claimBytes())
	}
}

func TestUIEditMarksEditedAndPersists(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "original body")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	// Open the inline editor on the thread root (own-role human message). The
	// editor is prefilled with the raw body; empty it (a plain value reset —
	// chromedp.Clear expects a textarea child #text node the JS-set .value lacks)
	// then type the replacement so a real edit round-trips.
	editInput := "#commentsPanel .comment-edit-form .comment-composer-input"
	runCDP(t, ctx,
		chromedp.WaitVisible("#commentsPanel .comment-thread > .comment-message .comment-edit", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-thread > .comment-message .comment-edit", chromedp.ByQuery),
		chromedp.WaitVisible(editInput, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('`+editInput+`').value = '';`, nil),
		chromedp.SendKeys(editInput, "a revised body", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-edit-form .comment-composer-submit", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('#commentsPanel .comment-edited')`)
	pollTrue(t, ctx, `Array.from(document.querySelectorAll('#commentsPanel .comment-body')).some(function(b){return b.textContent.indexOf('a revised body') >= 0})`)

	got := p.claimBytes()
	if !bytes.Contains(got, []byte("a revised body")) || !bytes.Contains(got, []byte("edited: true")) {
		t.Fatalf("edit was not persisted (want body + edited: true):\n%s", got)
	}
}

func TestUIDeleteReplyRemovesIt(t *testing.T) {
	p := newProject(t)
	tid := p.seedComment("human", "thread with a reply")
	p.run("comment", "reply", testClaimID, tid, "--as", "human", "--body", "reply to be deleted")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	// A reply delete (rid present) takes no confirm.
	runCDP(t, ctx,
		chromedp.WaitVisible("#commentsPanel .comment-reply .comment-delete", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-reply .comment-delete", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comment-reply').length === 0`)
	if bytes.Contains(p.claimBytes(), []byte("reply to be deleted")) {
		t.Fatalf("reply delete was not persisted:\n%s", p.claimBytes())
	}
}

func TestUIDeleteWholeThreadConfirmed(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "whole thread to delete")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	// A whole-thread delete asks for confirmation; auto-confirm it, then click
	// the thread root's ✕.
	runCDP(t, ctx,
		chromedp.Evaluate(`window.confirm = function () { return true; };`, nil),
		chromedp.WaitVisible("#commentsPanel .comment-thread > .comment-message .comment-delete", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-thread > .comment-message .comment-delete", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comment-thread').length === 0`)
	// The last thread is gone, so the chip falls back to its zero state — but
	// against a live serve it STAYS VISIBLE (v0.3.0): the claim can still be
	// commented on, and hiding the chip here would strand the panel the user is
	// currently looking at behind a control that no longer exists.
	pollTrue(t, ctx, `(function(){var c=document.querySelector('.comment-chip');return !!c && c.classList.contains('comment-chip--empty') && !c.closest('.claim-comments-slot').hidden;})()`)
	if evalString(t, ctx, `document.querySelector('.comment-chip .comment-chip-count').textContent`) != "0" {
		t.Fatal("the zero-state chip must read 0 after the last thread is deleted")
	}
	if bytes.Contains(p.claimBytes(), []byte("whole thread to delete")) {
		t.Fatalf("thread delete was not persisted:\n%s", p.claimBytes())
	}
}

func TestUIReopenReturnsThreadToOpen(t *testing.T) {
	p := newProject(t)
	tid := p.seedComment("human", "resolved then reopened")
	p.resolveViaAPI(tid, "human") // starts resolved (viewer-only op; see resolveViaAPI)
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	// The thread starts resolved, collapsed inside <details>. Expand it, then
	// reopen it.
	pollTrue(t, ctx, `!!document.querySelector('#commentsPanel .comments-resolved .comment-reopen')`)
	runCDP(t, ctx,
		chromedp.Evaluate(`document.querySelector('#commentsPanel .comments-resolved').open = true;`, nil),
		chromedp.WaitVisible("#commentsPanel .comment-reopen", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-reopen", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread:not(.comment-thread--resolved)').length === 1`)
	pollTrue(t, ctx, `document.querySelector('.comment-chip').classList.contains('comment-chip--open')`)
	if !bytes.Contains(p.claimBytes(), []byte("status: open")) {
		t.Fatalf("reopen was not persisted (want a comment status: open):\n%s", p.claimBytes())
	}
}

// ---------------------------------------------------------------------
// The delegated (not per-element) tab listener still switches modules — the
// refactor that sets up Phase 5c's fragment-swap re-render.
// ---------------------------------------------------------------------

const twoModuleConfig = `schema_version: 1
facets:
  - contract
modules:
  - widget
  - gadget
claims_dir: claims
`

func twoModuleClaim(id, module string) string {
	return "id: " + id + `
facet: contract
module: ` + module + `
status: draft
body: |
  a claim in module ` + module + `.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
}

func TestDelegatedTabNavigationSwitchesModules(t *testing.T) {
	p := newProjectRaw(t, twoModuleConfig)
	p.writeClaim("widget.yaml", twoModuleClaim("widget.contract.overview", "widget"))
	p.writeClaim("gadget.yaml", twoModuleClaim("gadget.contract.overview", "gadget"))

	base, _ := p.serve()
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(".sec-tab", chromedp.ByQuery),
	)
	// On load the first module is shown, the second hidden.
	pollTrue(t, ctx, `document.querySelectorAll('.module-section').length === 2 && !document.querySelectorAll('.module-section')[0].hidden && document.querySelectorAll('.module-section')[1].hidden`)

	// Click the SECOND sidebar tab. Its handler is bound by delegation on
	// document (not on the button), so this exercises the delegated path.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelectorAll('.sec-tab')[1].click();`, nil))
	pollTrue(t, ctx, `document.querySelectorAll('.module-section')[0].hidden && !document.querySelectorAll('.module-section')[1].hidden`)
}

// ---------------------------------------------------------------------
// Optimistic rollback + toast on a server error (409 conflict)
// ---------------------------------------------------------------------

func TestUIToastAndRollbackOnConflict(t *testing.T) {
	p := newProject(t)
	tid := p.seedComment("human", "conflict thread")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)
	waitVisible(t, ctx, "#commentsPanel .comment-resolve")

	// Resolve the thread out-of-band via the CLI AFTER the panel loaded it as
	// open. The next UI resolve then races a now-resolved thread -> 409.
	p.resolveViaAPI(tid, "human")

	runCDP(t, ctx, chromedp.Click("#commentsPanel .comment-resolve", chromedp.ByQuery))

	// A toast surfaces and the optimistic "resolved" is rolled back (the thread
	// never collapses into a resolved <details>).
	pollTrue(t, ctx, `(function(){var el=document.getElementById('commentsToast');return el && !el.hasAttribute('hidden') && el.textContent.length > 0;})()`)
	if evalBool(t, ctx, `!!document.querySelector('#commentsPanel .comments-resolved')`) {
		t.Fatal("a failed resolve must not collapse the thread (rollback expected)")
	}
	if evalInt(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread').length`) != 1 {
		t.Fatal("the thread should remain open in the UI after the failed resolve")
	}
}

// ---------------------------------------------------------------------
// Escape + overlay ownership
// ---------------------------------------------------------------------

func TestEscapeClosesPanelNotNav(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "escape me")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	if !evalBool(t, ctx, `document.body.classList.contains('comments-open')`) {
		t.Fatal("panel should be open")
	}
	// Escape with a panel open closes the panel and returns — it must NOT toggle
	// the nav drawer.
	runCDP(t, ctx, chromedp.Evaluate(`window.dispatchEvent(new KeyboardEvent('keydown', {key:'Escape'}))`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('comments-open')`)
	if evalBool(t, ctx, `document.body.classList.contains('nav-open')`) {
		t.Fatal("Escape that closed the panel must not open the nav drawer")
	}

	// With no panel open, the same single listener's else-branch closes the nav.
	runCDP(t, ctx,
		chromedp.Evaluate(`document.body.classList.add('nav-open')`, nil),
		chromedp.Evaluate(`window.dispatchEvent(new KeyboardEvent('keydown', {key:'Escape'}))`, nil),
	)
	pollTrue(t, ctx, `!document.body.classList.contains('nav-open')`)
}

func TestOverlaysAreMutuallyExclusive(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "overlap")
	ctx := newLiveTab(t, p)

	// Pin an explicit DESKTOP viewport (above the 860px bottom-sheet breakpoint)
	// so this spec is deterministic under any Chromium. The default browser window
	// is ~800px wide in some builds — below 860px — which puts the viewer in mobile
	// mode where the modal nav drawer's full-viewport scrim (#navOverlay,
	// pointer-events:auto while open) intercepts the real chip click below, so the
	// final "re-open comments closes the nav" step would flakily never fire. The
	// mutual-exclusivity assertions are JS class-state and hold at any width; this
	// only removes the click-interception flake, it does not weaken them.
	runCDP(t, ctx, chromedp.EmulateViewport(1200, 800))
	if evalBool(t, ctx, `window.matchMedia('(max-width: 860px)').matches`) {
		t.Fatal("EmulateViewport(1200) did not take effect: still matches the <=860px media query")
	}

	// Open the comment panel.
	runCDP(t, ctx, chromedp.Click(".comment-chip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)

	// Opening the nav drawer closes the comment panel.
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('navToggle').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('nav-open') && !document.body.classList.contains('comments-open')`)

	// Re-opening the comment panel closes the nav drawer.
	runCDP(t, ctx, chromedp.Click(".comment-chip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open') && !document.body.classList.contains('nav-open')`)
}

// ---------------------------------------------------------------------
// XSS: a hostile body typed in the composer renders inert after the round-trip
// ---------------------------------------------------------------------

func TestUIXSSBodyRendersInert(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "seed")
	ctx := newLiveTab(t, p)
	openPanelLive(t, ctx)

	const payload = `<img src=x onerror="window.__xssFired=1">`
	runCDP(t, ctx,
		chromedp.SendKeys("#commentsPanel .comment-composer .comment-composer-input", payload, chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-composer .comment-composer-submit", chromedp.ByQuery),
	)
	// Wait for the authoritative second thread (server body_html swapped in).
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread:not(.comment-thread--optimistic)').length === 2`)

	if evalBool(t, ctx, `!!document.querySelector('#commentsPanel .comment-body img')`) {
		t.Fatal("hostile body must not become a live <img> element")
	}
	if evalBool(t, ctx, `window.__xssFired === 1`) {
		t.Fatal("the injected onerror handler executed — body was not escaped")
	}
	if !evalBool(t, ctx, `Array.from(document.querySelectorAll('#commentsPanel .comment-body')).some(function(b){return b.textContent.indexOf('<img') >= 0})`) {
		t.Fatal("expected the hostile markup to appear as inert escaped text")
	}
}
