package viewertests

// Fix 3 — viewer JS regression suite. These specs drive a REAL `dossierx serve`
// in a headless browser and pin four confirmed defects fixed in shell.html /
// style.css:
//
//   - MAJOR: an SSE reload must not wipe unsent composer/reply/edit text (it
//     must skip the destructive rebuild when the open claim is unchanged, and
//     preserve any dirty draft across a rebuild it does have to do).
//   - MINOR: the dimming backdrop + body scroll-lock are the mobile bottom-sheet
//     behaviour only; the > 860px desktop rail is a non-modal complementary panel
//     with neither (Escape still closes it).
//   - MINOR: on tab re-show the client fires a catch-up refresh, so a change made
//     while the tab was hidden (and its SSE closed) still lands in the DOM.
//   - MINOR: a failed add/reply/edit keeps the user's typed text (repopulates the
//     input / keeps the edit form open) instead of discarding it.
//
// Every wait is deterministic (Poll / WaitVisible / a blocking SSE read), never a
// fixed sleep, so the suite is safe under -count=2.

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------

// evalString reads a string-valued JS expression from the page.
func evalString(t *testing.T, ctx context.Context, expr string) string {
	t.Helper()
	var v string
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &v)); err != nil {
		t.Fatalf("evaluate %q: %v", expr, err)
	}
	return v
}

// serveOpenTabWithStop starts serve for p, opens a tab, waits until the live
// runtime is mounted, and returns the browser context PLUS serve's base URL and
// stop func (the server-down and SSE-witness specs need both, which newLiveTab
// discards).
func serveOpenTabWithStop(t *testing.T, p *project) (ctx context.Context, base string, stop func()) {
	t.Helper()
	base, stop = p.serve()
	ctx = browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(".sec-tab", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.body.classList.contains('comments-live')`)
	return ctx, base, stop
}

// setTabHidden / setTabVisible drive the page's visibilitychange handler by
// shadowing document.visibilityState with an own getter (the real property is a
// read-only prototype getter) and dispatching the event the handler listens for.
// This is how the SSE lifecycle (close on hidden, reopen + catch-up on visible)
// is exercised without a real tab switch.
func setTabHidden(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		Object.defineProperty(document, 'visibilityState', {configurable:true, get:function(){return 'hidden';}});
		Object.defineProperty(document, 'hidden', {configurable:true, get:function(){return true;}});
		document.dispatchEvent(new Event('visibilitychange'));
		return true;
	})()`, nil))
}

func setTabVisible(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		Object.defineProperty(document, 'visibilityState', {configurable:true, get:function(){return 'visible';}});
		Object.defineProperty(document, 'hidden', {configurable:true, get:function(){return false;}});
		document.dispatchEvent(new Event('visibilitychange'));
		return true;
	})()`, nil))
}

// sseWitness is a server-side /api/events subscriber. A plain Go GET passes serve
// admission (only mutating methods are gated), so the test can hold a real SSE
// subscription that is NOT the browser. The re-show catch-up spec uses it to
// consume the (single, debounced) "changed" broadcast while the browser's own
// stream is closed, guaranteeing the browser's later reconnect gets no replay.
type sseWitness struct {
	resp *http.Response
	sc   *bufio.Scanner
}

func openSSEWitness(t *testing.T, base string) *sseWitness {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/events", http.NoBody)
	if err != nil {
		t.Fatalf("sse witness new request: %v", err)
	}
	// A generous timeout is only a safety net: the broadcast arrives ~one watcher
	// cycle after the change, well inside this bound, in both fixed and unfixed
	// runs (the witness is server-side, unaffected by the client fix).
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("sse witness connect: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("sse witness status = %d, want 200", resp.StatusCode)
	}
	return &sseWitness{resp: resp, sc: bufio.NewScanner(resp.Body)}
}

func (w *sseWitness) Close() {
	if w.resp != nil {
		_ = w.resp.Body.Close()
	}
}

// waitLine blocks (on the stream's IO) until a line matching pred is read, or
// fails if the stream ends first.
func (w *sseWitness) waitLine(t *testing.T, what string, pred func(string) bool) {
	t.Helper()
	for w.sc.Scan() {
		if pred(w.sc.Text()) {
			return
		}
	}
	t.Fatalf("sse witness stream ended before %s; err=%v", what, w.sc.Err())
}

func (w *sseWitness) waitConnected(t *testing.T) {
	t.Helper()
	w.waitLine(t, "\": connected\"", func(s string) bool { return strings.Contains(s, ": connected") })
}

func (w *sseWitness) waitChanged(t *testing.T) {
	t.Helper()
	w.waitLine(t, "\"event: changed\"", func(s string) bool { return strings.Contains(s, "event: changed") })
}

// openPanelByChip opens the comment panel by dispatching a real (bubbling) DOM
// click on the first chip in JS. It is coordinate-independent, so it is robust
// under an emulated mobile viewport where a fixed mobile-nav element can sit over
// the chip; the click still flows through the delegated document listener exactly
// as a user tap would.
func openPanelByChip(t *testing.T, ctx context.Context) {
	t.Helper()
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.comment-chip').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
}

// ---------------------------------------------------------------------
// MINOR — the > 860px desktop rail is non-modal: no backdrop, no scroll-lock,
// but Escape still closes it. The <= 860px bottom sheet keeps both.
// ---------------------------------------------------------------------

func TestDesktopPanelIsNonModalNoBackdropNoScrollLock(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "seed")
	ctx := newLiveTab(t, p)

	// Desktop viewport, above the 860px bottom-sheet breakpoint.
	runCDP(t, ctx, chromedp.EmulateViewport(1200, 800))
	if evalBool(t, ctx, `window.matchMedia('(max-width: 860px)').matches`) {
		t.Fatal("EmulateViewport(1200) did not take effect: still matches the <=860px media query")
	}

	openPanelByChip(t, ctx)

	// A non-modal complementary rail shows NO dimming backdrop...
	if d := evalString(t, ctx, `getComputedStyle(document.getElementById('commentsOverlay')).display`); d != "none" {
		t.Fatalf("desktop backdrop display = %q, want \"none\" (the desktop rail is non-modal, no backdrop)", d)
	}
	// ...and does NOT lock body scroll.
	if ov := evalString(t, ctx, `getComputedStyle(document.body).overflow`); ov == "hidden" {
		t.Fatalf("desktop body overflow = %q, want not \"hidden\" (the desktop rail must not scroll-lock)", ov)
	}
	// Escape still closes it even though it is non-modal.
	runCDP(t, ctx, chromedp.Evaluate(`window.dispatchEvent(new KeyboardEvent('keydown', {key:'Escape'}))`, nil))
	pollTrue(t, ctx, `!document.body.classList.contains('comments-open')`)
}

func TestMobilePanelIsModalHasBackdropAndScrollLock(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "seed")
	ctx := newLiveTab(t, p)

	// Mobile viewport, at/below the 860px bottom-sheet breakpoint.
	runCDP(t, ctx, chromedp.EmulateViewport(600, 800))
	if !evalBool(t, ctx, `window.matchMedia('(max-width: 860px)').matches`) {
		t.Fatal("EmulateViewport(600) did not take effect: does not match the <=860px media query")
	}

	openPanelByChip(t, ctx)

	// The mobile bottom sheet is modal: dimming backdrop + body scroll-lock.
	if d := evalString(t, ctx, `getComputedStyle(document.getElementById('commentsOverlay')).display`); d != "block" {
		t.Fatalf("mobile backdrop display = %q, want \"block\" (the mobile bottom sheet is modal)", d)
	}
	if ov := evalString(t, ctx, `getComputedStyle(document.body).overflow`); ov != "hidden" {
		t.Fatalf("mobile body overflow = %q, want \"hidden\" (the mobile bottom sheet must scroll-lock)", ov)
	}
}

// ---------------------------------------------------------------------
// MINOR — a change made while the tab is hidden (its SSE closed) is caught up on
// re-show: becoming visible fires one onServerChanged() so the DOM refreshes even
// though the reconnected stream never replays the missed "changed".
// ---------------------------------------------------------------------

func TestTabReshowCatchesUpChangeMadeWhileHidden(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	ctx, base, _ := serveOpenTabWithStop(t, p)

	// The live SSE stream is connected.
	pollTrue(t, ctx, `document.body.classList.contains('comments-sse-open')`)

	// Hide the tab -> the client closes its EventSource.
	setTabHidden(t, ctx)
	pollTrue(t, ctx, `!document.body.classList.contains('comments-sse-open')`)

	// A server-side witness now holds the ONLY live subscription, so the change's
	// single debounced "changed" is delivered to it — not to the (closed) browser
	// stream — reproducing "a change the tab missed while hidden".
	witness := openSSEWitness(t, base)
	defer witness.Close()
	witness.waitConnected(t)

	// Change a claim on disk while the tab is hidden.
	p.writeClaim("ctr2.yaml", facetClaim("widget.contract.added", "contract"))

	// The witness consumes the broadcast; the browser's later reconnect gets no
	// replay of it (the server only re-sends ": connected" on subscribe).
	witness.waitChanged(t)

	// Re-show the tab. Without the catch-up fix the reconnected stream sees only
	// ": connected" and the DOM never refreshes; with it, re-show fires
	// onServerChanged() once and the claim added while hidden lands in the DOM.
	setTabVisible(t, ctx)
	pollTrue(t, ctx, `!!document.getElementById('widget.contract.added')`)
}

// ---------------------------------------------------------------------
// MINOR — a failed add / reply / edit keeps the user's typed text: the composer
// repopulates and the edit form stays open, instead of discarding the input.
// Covers both a network failure (server down) and a real HTTP 409.
// ---------------------------------------------------------------------

func TestAddRepopulatesComposerOnServerDown(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "seed")
	ctx, _, stop := serveOpenTabWithStop(t, p)
	openPanelLive(t, ctx)

	const draft = "an unsent comment draft"
	runCDP(t, ctx, chromedp.SendKeys("#commentsPanel .comment-composer .comment-composer-input", draft, chromedp.ByQuery))

	// Kill the server so the submit's POST fails at the network layer.
	stop()
	runCDP(t, ctx, chromedp.Click("#commentsPanel .comment-composer .comment-composer-submit", chromedp.ByQuery))

	// The optimistic placeholder rolls back, a toast surfaces, and — the fix — the
	// composer is repopulated with the user's text instead of silently losing it.
	pollTrue(t, ctx, `(function(){var el=document.getElementById('commentsToast');return el && !el.hasAttribute('hidden') && el.textContent.length > 0;})()`)
	pollTrue(t, ctx, `document.querySelector('#commentsPanel .comment-composer .comment-composer-input').value === 'an unsent comment draft'`)
	if evalInt(t, ctx, `document.querySelectorAll('#commentsPanel .comment-thread--optimistic').length`) != 0 {
		t.Fatal("the optimistic placeholder must be rolled back on a failed add")
	}
}

func TestReplyRepopulatesOnResolvedConflict(t *testing.T) {
	p := newProject(t)
	tid := p.seedComment("human", "parent thread")
	ctx, _, _ := serveOpenTabWithStop(t, p)
	openPanelLive(t, ctx)
	waitVisible(t, ctx, "#commentsPanel .comment-reply-composer .comment-composer-input")

	const draft = "an unsent reply draft"
	runCDP(t, ctx, chromedp.SendKeys("#commentsPanel .comment-reply-composer .comment-composer-input", draft, chromedp.ByQuery))

	// Hide the tab so the client closes its SSE: the out-of-band resolve below then
	// triggers no live reload (which would rebuild the panel and drop the resolved
	// thread's reply composer), isolating the 409 to the submit itself.
	setTabHidden(t, ctx)
	pollTrue(t, ctx, `!document.body.classList.contains('comments-sse-open')`)

	// Resolve the thread out-of-band; the reply POST then races a now-resolved
	// thread -> 409 thread_resolved.
	p.resolveViaAPI(tid, "human")
	runCDP(t, ctx, chromedp.Click("#commentsPanel .comment-reply-composer .comment-composer-submit", chromedp.ByQuery))

	// A toast surfaces and the reply composer is repopulated (not silently lost).
	pollTrue(t, ctx, `(function(){var el=document.getElementById('commentsToast');return el && !el.hasAttribute('hidden') && el.textContent.length > 0;})()`)
	pollTrue(t, ctx, `document.querySelector('#commentsPanel .comment-reply-composer .comment-composer-input').value === 'an unsent reply draft'`)
}

func TestEditKeepsFormOpenOnServerDown(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "original body")
	ctx, _, stop := serveOpenTabWithStop(t, p)
	openPanelLive(t, ctx)

	// Open the inline editor on the thread root and change the text.
	editInput := "#commentsPanel .comment-edit-form .comment-composer-input"
	runCDP(t, ctx,
		chromedp.WaitVisible("#commentsPanel .comment-thread > .comment-message .comment-edit", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-thread > .comment-message .comment-edit", chromedp.ByQuery),
		chromedp.WaitVisible(editInput, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('`+editInput+`').value = '';`, nil),
		chromedp.SendKeys(editInput, "a revision in progress", chromedp.ByQuery),
	)

	// Kill the server so the PATCH fails.
	stop()
	runCDP(t, ctx, chromedp.Click("#commentsPanel .comment-edit-form .comment-composer-submit", chromedp.ByQuery))

	// A toast surfaces; the edit form STAYS OPEN with the user's revision (the fix)
	// instead of reverting to the rendered body and discarding it.
	pollTrue(t, ctx, `(function(){var el=document.getElementById('commentsToast');return el && !el.hasAttribute('hidden') && el.textContent.length > 0;})()`)
	pollTrue(t, ctx, `(function(){var ta=document.querySelector('`+editInput+`');return !!ta && ta.value === 'a revision in progress';})()`)
}

// ---------------------------------------------------------------------
// MAJOR — an SSE reload must not wipe unsent composer / reply / edit text. An
// unrelated-claim "changed" (the advertised concurrent-agent workflow) must skip
// the destructive rebuild of the open panel; a same-claim "changed" that DOES
// rebuild must capture and restore the dirty draft.
// ---------------------------------------------------------------------

func TestReloadPreservesComposerDraft(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	p.run("comment", "add", "widget.contract.base", "--as", "human", "--body", "seed thread")
	ctx := serveAndOpenLive(t, p)

	// Open the panel on the base claim and type a comment WITHOUT submitting it.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.comment-chip[data-claim-id="widget.contract.base"]').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	waitVisible(t, ctx, "#commentsPanel .comment-composer .comment-composer-input")
	const draft = "a composer draft in flight"
	runCDP(t, ctx, chromedp.SendKeys("#commentsPanel .comment-composer .comment-composer-input", draft, chromedp.ByQuery))

	// (part b) An UNRELATED claim changes -> a reload whose fragment brings in the
	// new card. The open claim's own threads are unchanged, so the destructive
	// panel rebuild must be skipped.
	p.writeClaim("des.yaml", facetClaim("widget.design.unrelated", "design"))
	pollTrue(t, ctx, `!!document.getElementById('widget.design.unrelated')`)

	// (part a) ...then the OPEN claim itself changes (a second thread added
	// out-of-band) -> a reload that DOES rebuild the panel. Anchor on the second
	// thread so both reloads have been fully processed by the assertion below.
	p.run("comment", "add", "widget.contract.base", "--as", "human", "--body", "external second thread")
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread').length === 2`)

	// The unsent composer draft survived BOTH the unrelated and the same-claim reload.
	if v := evalString(t, ctx, `document.querySelector('#commentsPanel .comment-composer .comment-composer-input').value`); v != draft {
		t.Fatalf("composer draft = %q after an unrelated + a same-claim reload, want %q (an SSE reload must not wipe unsent composer text)", v, draft)
	}
}

func TestReloadPreservesReplyDraft(t *testing.T) {
	p := newProject(t)
	tid := p.seedComment("human", "parent thread")
	ctx := serveAndOpenLive(t, p)

	// Open the panel and type a reply to the seeded thread WITHOUT submitting.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.comment-chip').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	replyInput := `#commentsPanel .comment-thread[data-thread-id="` + tid + `"] .comment-reply-composer .comment-composer-input`
	waitVisible(t, ctx, replyInput)
	const draft = "an unsent reply draft"
	runCDP(t, ctx, chromedp.SendKeys(replyInput, draft, chromedp.ByQuery))

	// A same-claim change (a NEW thread added out-of-band) -> a reload that rebuilds
	// the panel. Anchor on the second thread.
	p.run("comment", "add", testClaimID, "--as", "human", "--body", "external new thread")
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread').length === 2`)

	// The reply draft, keyed to its owning thread, survived the rebuild.
	if v := evalString(t, ctx, `document.querySelector('`+replyInput+`').value`); v != draft {
		t.Fatalf("reply draft = %q after a same-claim rebuild, want %q (a reload must preserve unsent reply text)", v, draft)
	}
}

func TestReloadPreservesEditDraft(t *testing.T) {
	p := newProject(t)
	tid := p.seedComment("human", "original body")
	ctx := serveAndOpenLive(t, p)

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.comment-chip').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)

	// Open the inline editor on the thread root and change the text (do not save).
	rootEdit := `#commentsPanel .comment-thread[data-thread-id="` + tid + `"] > .comment-message .comment-edit`
	editInput := `#commentsPanel .comment-thread[data-thread-id="` + tid + `"] > .comment-message .comment-edit-form .comment-composer-input`
	runCDP(t, ctx,
		chromedp.WaitVisible(rootEdit, chromedp.ByQuery),
		chromedp.Click(rootEdit, chromedp.ByQuery),
		chromedp.WaitVisible(editInput, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('`+editInput+`').value = '';`, nil),
		chromedp.SendKeys(editInput, "an edited draft in progress", chromedp.ByQuery),
	)

	// A same-claim change (a NEW thread out-of-band) -> a reload that rebuilds the
	// panel. Anchor on the second thread.
	p.run("comment", "add", testClaimID, "--as", "human", "--body", "external new thread")
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread').length === 2`)

	// The in-progress edit survived: the editor is re-opened on the same message
	// with the user's revision (not reverted to the rendered body).
	if v := evalString(t, ctx, `(function(){var ta=document.querySelector('`+editInput+`');return ta ? ta.value : '<no editor>';})()`); v != "an edited draft in progress" {
		t.Fatalf("edit draft = %q after a same-claim rebuild, want the re-opened editor holding the revision", v)
	}
}
