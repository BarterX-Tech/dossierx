package viewertests

// VJF-1 — a composer/reply/edit textarea whose .value is restored
// PROGRAMMATICALLY (a dirty draft recaptured across an SSE rebuild, or a failed
// add/reply rolled back into the input) must GROW to fit its content, not render
// clipped to the one-row default. A bare `.value = draft` fires no 'input' event,
// so autoGrow's handler never runs; the fix calls growNow(ta) after each restore.
// These specs drive a real `dossierx serve` in a headless browser and assert the
// restored textarea is not clipped (content no taller than the visible box beyond
// the ~2px border) and did not collapse back to a single row.
//
// Every wait is deterministic (Poll / WaitVisible), never a fixed sleep, so the
// suite is safe under -count=2.

import (
	"context"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// draftLines is a clearly multi-row draft: enough lines that a one-row-clipped
// textarea is unmistakably distinct from a grown one. No quotes/backslashes, so
// it embeds safely in the single-quoted JS array literal below.
var draftLines = []string{
	"line one is a fairly long first line of the draft",
	"line two continues the thought",
	"line three",
	"line four",
	"line five is the last line",
}

// jsSetMultilineDraft returns JS that sets sel's textarea to the multi-line draft
// and dispatches an input event (exactly as real typing does), returning its
// grown clientHeight. Newlines are built with String.fromCharCode(10) so the Go
// source carries no embedded newline inside the JS string literal.
func jsSetMultilineDraft(sel string) string {
	return `(function(){var ta=document.querySelector('` + sel + `');if(!ta)return -1;` +
		`ta.value=['` + strings.Join(draftLines, "','") + `'].join(String.fromCharCode(10));` +
		`ta.dispatchEvent(new Event('input',{bubbles:true}));return ta.clientHeight;})()`
}

// jsNotClipped asserts sel's textarea is grown to fit its content: with
// overflow:hidden a clipped field reports scrollHeight far above clientHeight,
// while a grown one differs only by the ~2px border. 8px cleanly separates the
// two (a hidden row is ~18px).
func jsNotClipped(sel string) string {
	return `(function(){var ta=document.querySelector('` + sel + `');if(!ta)return false;` +
		`return (ta.scrollHeight - ta.clientHeight) <= 8;})()`
}

func jsClientHeight(sel string) string {
	return `(function(){var ta=document.querySelector('` + sel + `');return ta?ta.clientHeight:-1;})()`
}

// assertGrownNotClipped is the shared post-restore check: the textarea preserved
// the full draft, is not clipped, and did not collapse below its grown height.
func assertGrownNotClipped(t *testing.T, ctx context.Context, sel string, grownH int) {
	t.Helper()
	want := strings.Join(draftLines, "\n")
	if v := evalString(t, ctx, `document.querySelector('`+sel+`').value`); v != want {
		t.Fatalf("restored draft = %q, want the full multi-line draft %q", v, want)
	}
	if !evalBool(t, ctx, jsNotClipped(sel)) {
		sh := evalInt(t, ctx, `document.querySelector('`+sel+`').scrollHeight`)
		ch := evalInt(t, ctx, `document.querySelector('`+sel+`').clientHeight`)
		t.Fatalf("restored textarea is CLIPPED: scrollHeight=%d clientHeight=%d (a %d-line draft rendered to a shorter box)", sh, ch, len(draftLines))
	}
	if rh := evalInt(t, ctx, jsClientHeight(sel)); rh < grownH-8 {
		t.Fatalf("restored textarea collapsed: clientHeight=%d, grown was %d", rh, grownH)
	}
}

// growMultilineDraft sets the multi-line draft into sel (as typing would),
// asserts the setup actually grew the field well past one row, and returns the
// grown clientHeight for the later not-collapsed comparison.
func growMultilineDraft(t *testing.T, ctx context.Context, sel string) int {
	t.Helper()
	rowH := evalInt(t, ctx, jsClientHeight(sel)) // empty, one-row height
	grownH := evalInt(t, ctx, jsSetMultilineDraft(sel))
	if rowH <= 0 || grownH < rowH*2 {
		t.Fatalf("setup did not grow the composer: one-row=%d, after multi-line draft=%d (input-driven autoGrow may be broken)", rowH, grownH)
	}
	if !evalBool(t, ctx, jsNotClipped(sel)) {
		t.Fatal("setup: a freshly-typed multi-line draft should not be clipped (autoGrow's input handler)")
	}
	return grownH
}

// TestReloadRestoredComposerDraftIsNotClipped — a dirty multi-line composer draft
// recaptured across a same-claim SSE rebuild (buildComposer restoring .value)
// must come back GROWN, not clipped to one row.
func TestReloadRestoredComposerDraftIsNotClipped(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "seed thread")
	ctx := serveAndOpenLive(t, p)

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.comment-chip').click()`, nil))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	sel := "#commentsPanel .comment-composer .comment-composer-input"
	waitVisible(t, ctx, sel)

	grownH := growMultilineDraft(t, ctx, sel)

	// A same-claim change (a NEW thread added out-of-band) drives the reload that
	// rebuilds the panel and restores the dirty draft. Anchor on the second thread
	// so the rebuild has fully processed before the assertions.
	p.run("comment", "add", testClaimID, "--as", "human", "--body", "external new thread")
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread').length === 2`)

	assertGrownNotClipped(t, ctx, sel, grownH)
}

// TestFailedAddRestoredComposerDraftIsNotClipped — a multi-line composer draft
// rolled back into the input after a failed POST (doAdd's catch restoring .value)
// must come back GROWN, not clipped to one row.
func TestFailedAddRestoredComposerDraftIsNotClipped(t *testing.T) {
	p := newProject(t)
	p.seedComment("human", "seed")
	ctx, _, stop := serveOpenTabWithStop(t, p)
	openPanelLive(t, ctx)

	sel := "#commentsPanel .comment-composer .comment-composer-input"
	grownH := growMultilineDraft(t, ctx, sel)

	// Kill the server so the submit's POST fails at the network layer, triggering
	// the optimistic-rollback that restores the draft into the composer.
	stop()
	runCDP(t, ctx, chromedp.Click("#commentsPanel .comment-composer .comment-composer-submit", chromedp.ByQuery))

	// A toast surfaces and the draft is repopulated (cleared optimistically on
	// submit, then restored on the failure).
	pollTrue(t, ctx, `(function(){var el=document.getElementById('commentsToast');return el && !el.hasAttribute('hidden') && el.textContent.length > 0;})()`)
	pollTrue(t, ctx, `document.querySelector('`+sel+`').value.length > 0`)

	assertGrownNotClipped(t, ctx, sel, grownH)
}
