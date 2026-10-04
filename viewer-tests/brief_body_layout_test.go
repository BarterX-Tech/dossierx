package viewertests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
)

// The v0.7.23 brief page hotfix (Paper "Release 0.7.23", boards H1, H1b, H2;
// NIT-246, NIT-247, NIT-248, NIT-249), driven in a real browser: what only
// layout can show. The markup is pinned at its owner, internal/render
// (TestRender_BriefCodeRefsLinkOnlyWhatResolves, TestRender_BriefPageAndTree).

const layoutBrief = "brief-research-macos-app-lifecycle"

// layoutProject is a brief shaped like Curtainly's research briefs: a bullet
// that mixes a bold lead, prose and code long enough to wrap; a claim id and
// a bare URL wider than any card; a reference to another brief; and a title
// longer than the sidebar is wide.
func layoutProject(t *testing.T) *project {
	t.Helper()
	p := newProject(t)
	write := func(rel, data string) {
		path := filepath.Join(p.dir, "briefs", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("research/macos-app-lifecycle.md", "---\nsummary: How macOS treats a menu-bar agent.\n---\n# macOS app lifecycle\n\n## Menu bar\n\n"+
		"- **Removing the menu-bar extra can quit the app.** A menu-bar-only app will be terminated if the user removes the extra, and no callback is documented, which is why `"+testClaimID+"` exists and why `aVeryLongIdentifierWithNoBreakOpportunityAnywhereInsideItAtAll.contract.removing-or-hiding-the-icon-quits-nothing` is cited.\n"+
		"- Plain code like `setActivationPolicy` stays plain; see `voice/talking` for the words.\n"+
		"- Apple DTS confirmed only that terminating other apps is blocked (https://developer.apple.com/forums/thread/122020/with/a/very/long/path/that/never/breaks/on/its/own).\n")
	write("research/what-leaks-in-screen-shares-when-the-presenter-forgets-a-window.md", "---\nsummary: What leaks.\n---\n# What leaks in screen shares when the presenter forgets a window\n\nBody.\n")
	write("voice/talking.md", "---\nsummary: Voice.\n---\n# Talking\n\nBody.\n")
	return p
}

// TestBriefBody_BulletsCodeAndRefsAtTheEdges: a bullet with markup reads as
// one paragraph, nothing in a brief widens the page, and a reference to a
// brief or claim is tinted as a link while plain code is not.
//
// Authoring gate: (1) the reader sees a bullet as one wrapped paragraph and
// never scrolls a brief sideways, at desktop and phone widths; (2) restoring
// the flex list item (v0.7.22) puts the trailing prose in its own column
// right of the code, and dropping overflow-wrap lets the 90-character id
// push the document past the viewport — both fail here, measured, not read
// from CSS; (3) the render test pins the markup only and cannot see a layout
// box; (4) no new seam.
func TestBriefBody_BulletsCodeAndRefsAtTheEdges(t *testing.T) {
	p := layoutProject(t)
	url := p.renderStatic()
	for _, vp := range []struct {
		name string
		w, h int64
	}{{"desktop", 1440, 900}, {"phone", 390, 844}} {
		t.Run(vp.name, func(t *testing.T) {
			ctx := withInstantScroll(t, browserContext(t))
			runCDP(t, ctx, chromedp.EmulateViewport(vp.w, vp.h), chromedp.Navigate(url+"#"+layoutBrief), chromedp.WaitVisible("#"+layoutBrief, chromedp.ByQuery))
			suppressTransitions(t, ctx, "")
			requireAll(t, ctx, "the brief body at "+vp.name,
				`var body = document.querySelector('#`+layoutBrief+` .brief-body');
				 var items = body.querySelectorAll('ul > li');
				 var first = items[0];
				 var lead = first.querySelector('strong').getBoundingClientRect();
				 var tail = first.querySelector('strong').nextSibling;
				 var range = document.createRange(); range.selectNodeContents(tail);
				 var tailRects = Array.prototype.slice.call(range.getClientRects());
				 var bodyRect = body.getBoundingClientRect();
				 var codes = Array.prototype.slice.call(body.querySelectorAll('code'));
				 var plain = Array.prototype.filter.call(codes, function (c) { return c.textContent === 'setActivationPolicy'; })[0];
				 var briefRef = body.querySelector('a.code-ref--brief code');
				 var claimRef = body.querySelector('a.code-ref--claim code');`,
				[][2]string{
					{"three bullets", `items.length === 3`},
					{"the trailing prose wraps back under the bold lead", `tailRects.length > 1 && tailRects.some(function (r) { return Math.abs(r.left - lead.left) < 2; })`},
					{"the dash sits left of the text", `parseFloat(getComputedStyle(first).paddingLeft) >= 20`},
					{"the document is no wider than the viewport", `document.documentElement.scrollWidth <= window.innerWidth`},
					{"the body does not overflow", `body.scrollWidth <= body.clientWidth + 1`},
					{"every code span stays inside the card", `codes.every(function (c) { return Array.prototype.every.call(c.getClientRects(), function (r) { return r.right <= bodyRect.right + 1; }); })`},
					{"a brief reference is tinted unlike plain code", `!!briefRef && getComputedStyle(briefRef).backgroundColor !== getComputedStyle(plain).backgroundColor`},
					{"a claim reference is tinted the same way", `!!claimRef && getComputedStyle(claimRef).backgroundColor === getComputedStyle(briefRef).backgroundColor`},
					{"the brief reference opens the brief", `briefRef.parentElement.getAttribute('href') === '#brief-voice-talking'`},
				})
		})
	}
}

// TestBriefTree_RowsUseTitlesAndWrap: a sidebar row reads the brief's own
// title and a long one wraps instead of truncating.
//
// Authoring gate: (1) the reader finds a brief by the words its page shows,
// and can read the whole name; (2) the v0.7.22 file-name label ("Macos app
// lifecycle") and its one-line ellipsis both fail; (3) the render test pins
// the label text, not whether the row clips it; (4) no new seam.
func TestBriefTree_RowsUseTitlesAndWrap(t *testing.T) {
	p := layoutProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+layoutBrief), chromedp.WaitVisible("#"+layoutBrief, chromedp.ByQuery))
	suppressTransitions(t, ctx, "")
	requireAll(t, ctx, "the Research rows",
		`var rows = document.querySelectorAll('#nav .brief-folder[data-folder="research"] .brief-nav__row');
		 var label = function (id) { return document.querySelector('#nav .brief-nav__row[data-target="#' + id + '"] .brief-nav__label'); };
		 var short = label('`+layoutBrief+`');
		 var long = label('brief-research-what-leaks-in-screen-shares-when-the-presenter-forgets-a-window');`,
		[][2]string{
			{"two rows", `rows.length === 2`},
			{"a row is the page heading's words", `short.textContent === document.querySelector('#` + layoutBrief + ` .brief-title').textContent`},
			{"a long title wraps to more lines", `long.getClientRects().length >= 1 && long.getBoundingClientRect().height > 1.5 * parseFloat(getComputedStyle(long).lineHeight)`},
			{"nothing is clipped", `long.scrollWidth <= long.clientWidth + 1 && getComputedStyle(long).textOverflow !== 'ellipsis'`},
			{"the row grows with it", `long.closest('.brief-nav__row').getBoundingClientRect().height > 30`},
		})
}
