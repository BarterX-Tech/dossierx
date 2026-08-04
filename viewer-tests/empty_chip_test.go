package viewertests

// v0.2.1 — the comment chip on a claim with NO threads. Before this, a card
// only grew a 💬 chip once it already had a comment, which made the FIRST
// comment on any card unreachable from the viewer — the human's only surface.
//
// Fixing it took TWO gates, and these specs pin both ends of the seam:
//
//   - SERVER (components.EdgesHTMLWithLinks): the chip is emitted for every
//     non-banner claim, in an --empty "💬 0" variant whose <li> ships `hidden`.
//   - CLIENT (shell.html): hiding is PROBE-AWARE. syncEmptyChips reveals the
//     empty chips once /api/ping confirms a live serve, and updateChips hides
//     one only when `!mounted` — because buildPanel calls updateChips(id, 0, 0)
//     on exactly the empty claim whose chip was just clicked, and the old bare
//     `totalCount === 0` would have vanished that chip mid-click.
//
// Every wait is deterministic (Poll / WaitVisible), never a fixed sleep, so the
// suite is safe under -count=2.

import (
	"bytes"
	"testing"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// Live serve: the chip is there on a claim nobody has commented on, the rail
// opens on it with a composer, and the first comment posts without a reload.
// ---------------------------------------------------------------------

// TestEmptyClaimChipOpensRailAndPostsFirstComment is the release gate for
// v0.2.1: "first comment openable, and the chip survives the click".
func TestEmptyClaimChipOpensRailAndPostsFirstComment(t *testing.T) {
	p := newProject(t) // one claim, deliberately NOT seeded with any comment
	ctx := newLiveTab(t, p)

	// The probe revealed the zero-state chip: visible, --empty, reading 0.
	pollTrue(t, ctx, `(function(){
		var c = document.querySelector('.comment-chip');
		return !!c && c.classList.contains('comment-chip--empty') && !c.closest('.claim-comments-slot').hidden;
	})()`)
	if got := evalString(t, ctx, `document.querySelector('.comment-chip .comment-chip-count').textContent`); got != "0" {
		t.Fatalf("zero-state chip count = %q, want \"0\"", got)
	}
	if got := evalString(t, ctx, `document.querySelector('.comment-chip').getAttribute('aria-label')`); got != "add the first comment on this claim" {
		t.Fatalf("zero-state chip aria-label = %q, want the add-the-first-comment invitation", got)
	}

	// Click it. The rail must open on a claim with no threads, mounting the
	// composer PLUS an empty-state line...
	runCDP(t, ctx, chromedp.Click(".comment-chip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	waitVisible(t, ctx, "#commentsPanel .comment-composer .comment-composer-input")
	waitVisible(t, ctx, "#commentsPanel .comments-empty")

	// ...and the chip that opened it must SURVIVE. buildPanel's
	// updateChips(claimID, 0, 0) runs on this exact claim; the pre-fix client
	// would have hidden the chip out from under the click.
	if evalBool(t, ctx, `document.querySelector('.comment-chip').closest('.claim-comments-slot').hidden`) {
		t.Fatal("clicking an empty chip must not hide it (buildPanel's updateChips(id, 0, 0) re-hide bug)")
	}
	if !evalBool(t, ctx, `document.querySelector('.comment-chip').getAttribute('aria-expanded') === 'true'`) {
		t.Fatal("the empty chip must report aria-expanded=true while its rail is open")
	}

	// Post the FIRST comment from the rail.
	runCDP(t, ctx,
		chromedp.SendKeys("#commentsPanel .comment-composer .comment-composer-input", "the very first comment", chromedp.ByQuery),
		chromedp.Click("#commentsPanel .comment-composer .comment-composer-submit", chromedp.ByQuery),
	)
	// Gate on the authoritative (post-round-trip) thread so the write really
	// reached the server, then assert the chip flipped to the open state reading
	// 1 — all without a page reload (this tab was never navigated again).
	pollTrue(t, ctx, `document.querySelectorAll('#commentsPanel .comments-threads > .comment-thread:not(.comment-thread--optimistic)').length === 1`)
	pollTrue(t, ctx, `(function(){
		var c = document.querySelector('.comment-chip');
		return !!c && c.classList.contains('comment-chip--open') && !c.classList.contains('comment-chip--empty') &&
			c.querySelector('.comment-chip-count').textContent === '1';
	})()`)
	// The empty-state line yields to the real thread.
	if evalInt(t, ctx, `document.querySelectorAll('#commentsPanel .comments-empty').length`) != 0 {
		t.Fatal("the empty-state line must be gone once the claim has a thread")
	}

	if !bytes.Contains(p.claimBytes(), []byte("the very first comment")) {
		t.Fatalf("the first comment was not persisted to the claim file:\n%s", p.claimBytes())
	}
}

// A quiet claim's chip must not steal the accent left-border that marks cards
// with an OPEN discussion — the zero state is an affordance, not a signal.
func TestEmptyChipDoesNotMarkCardAsCommented(t *testing.T) {
	p := newProject(t)
	ctx := newLiveTab(t, p)
	if evalBool(t, ctx, `!!document.querySelector('.claim-card--commented')`) {
		t.Fatal("a claim with no threads must not render the commented-card accent")
	}
}

// ---------------------------------------------------------------------
// Static file://: no API answers, so the empty chips stay hidden.
// ---------------------------------------------------------------------

// TestFileURLHidesEmptyChips is the other half of the probe-aware gate. On a
// static export there is no comment API, so no composer mounts — an empty chip
// there would open a rail holding nothing and offering nothing. It must stay
// hidden, while a chip that HAS threads stays visible (a static export can
// still show an existing discussion; only adding to one needs the API).
func TestFileURLHidesEmptyChips(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	p.writeClaim("ctr2.yaml", facetClaim("widget.contract.quiet", "contract"))
	p.run("comment", "add", "widget.contract.base", "--as", "human", "--body", "a baked thread")
	url := p.renderStatic()

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.comment-chip[data-claim-id="widget.contract.base"]`, chromedp.ByQuery),
	)

	// The probe cannot reach a server from file://. Its ~1s AbortController
	// timeout is the upper bound on comments-live ever appearing, so wait for the
	// commented chip to be visible (above) and then confirm the runtime never
	// mounted — poll it, since asserting "still false" immediately could pass
	// before a (hypothetically) succeeding probe had resolved.
	pollTrue(t, ctx, `(function(){
		var quiet = document.querySelector('.comment-chip[data-claim-id="widget.contract.quiet"]');
		return !!quiet && quiet.closest('.claim-comments-slot').hidden;
	})()`)
	if evalBool(t, ctx, `document.body.classList.contains('comments-live')`) {
		t.Fatal("a static file:// viewer must never mount the runtime")
	}

	// The chip markup is PRESENT (same render for both destinations) but the
	// zero-state one is not offered: hidden means it cannot be seen or clicked.
	if !evalBool(t, ctx, `!!document.querySelector('.comment-chip[data-claim-id="widget.contract.quiet"]')`) {
		t.Fatal("the empty chip should be in the markup, just hidden")
	}
	if evalInt(t, ctx, `Array.from(document.querySelectorAll('.comment-chip')).filter(function(c){return c.offsetParent !== null;}).length`) != 1 {
		t.Fatal("exactly one chip (the one with threads) may be visible on a static export")
	}
	if evalBool(t, ctx, `document.querySelector('.comment-chip[data-claim-id="widget.contract.base"]').closest('.claim-comments-slot').hidden`) {
		t.Fatal("a chip that HAS threads must stay visible on a static export")
	}
}

// ---------------------------------------------------------------------
// The SSE fragment swap ships freshly server-rendered (hidden) chips, so the
// reveal has to be re-applied — otherwise every empty chip on the page silently
// disappears the first time an agent touches any claim.
// ---------------------------------------------------------------------

func TestEmptyChipsStayRevealedAcrossReload(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	ctx := serveAndOpenLive(t, p)

	pollTrue(t, ctx, `(function(){
		var c = document.querySelector('.comment-chip[data-claim-id="widget.contract.base"]');
		return !!c && !c.closest('.claim-comments-slot').hidden;
	})()`)

	// An external change -> a fragment swap that replaces every card (and thus
	// every chip) with fresh server markup, in which the empty <li> is hidden
	// again. initViewer's syncEmptyChips pass is what must re-reveal them.
	p.writeClaim("ctr2.yaml", facetClaim("widget.contract.added", "contract"))
	pollTrue(t, ctx, `!!document.getElementById('widget.contract.added')`)

	if evalInt(t, ctx, `Array.from(document.querySelectorAll('.comment-chip')).filter(function(c){return c.closest('.claim-comments-slot').hidden;}).length`) != 0 {
		t.Fatal("empty chips must stay revealed after an SSE fragment swap")
	}
	// The chip on the claim that only just appeared is offered too.
	if evalBool(t, ctx, `document.querySelector('.comment-chip[data-claim-id="widget.contract.added"]').closest('.claim-comments-slot').hidden`) {
		t.Fatal("a claim added by a live reload must arrive with a usable chip")
	}
}
