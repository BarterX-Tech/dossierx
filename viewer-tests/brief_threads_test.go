package viewertests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// Comment threads on a brief (NIT-198), driven in a real browser. The routes
// and the rights are pinned at serve (internal/serve's
// TestBriefComments_*), the markup at the renderer
// (TestRender_BriefThreadsOnThePageTreeAndIndex); what is asserted here is
// what only a browser can show: the brief page's Comment button opening the
// shared rail on the brief, a thread written from it reaching the file, the
// sidebar mark and the index on the live reload, resolving clearing both, the
// phone's bottom sheet, the static build's read-only rail, and the entry
// point B2/B3 call.

const roundBriefID = "decisions.round-to-the-cent"
const roundBriefPath = "briefs/decisions/round-to-the-cent.md"

// roundRow is the round-to-the-cent row's mark in the sidebar tree.
const roundMark = `document.querySelector('#nav .sec-tab[data-target="#` + roundBrief + `"] .brief-mark')`

func (p *project) briefBytes(rel string) string {
	p.t.Helper()
	b, err := os.ReadFile(filepath.Join(p.dir, filepath.FromSlash(rel)))
	if err != nil {
		p.t.Fatal(err)
	}
	return string(b)
}

// TestBriefThreads_OpenFromThePageThenResolveClearsTheMark: under serve, the
// rail's Comment opens the rail on the brief, a thread written there lands in
// the brief's frontmatter, the live reload gives the brief the thread mark,
// the Threads block's count and the index's line, and resolving it clears all
// three. A hostile body renders inert.
func TestBriefThreads_OpenFromThePageThenResolveClearsTheMark(t *testing.T) {
	p := briefProject(t)
	ctx := serveAndOpenLive(t, p, "#"+roundBrief)
	desktopViewport(t, ctx)
	pollTrue(t, ctx, `!document.getElementById('`+roundBrief+`').hidden && !!document.querySelector('#systemFacetToc .facet-toc__comment')`)

	requireAll(t, ctx, "the brief page under serve, no thread yet",
		`var toc = document.getElementById('systemFacetToc');
		 var btn = toc.querySelector('.facet-toc__comment');`,
		[][2]string{
			{"the rail's Comment is live", `btn.disabled === false && btn.getAttribute('data-brief-id') === '` + roundBriefID + `'`},
			{"the Threads block says none is open", `toc.querySelector('.facet-toc__threads-note').textContent.indexOf('None open.') === 0`},
			{"no read-only line under serve", `toc.querySelector('.facet-toc__threads-later').hidden === true`},
			{"the brief carries its locked mark", roundMark + `.getAttribute('data-mark') === 'locked'`},
		})

	evalVoid(t, ctx, `document.querySelector('#systemFacetToc .facet-toc__comment').click()`)
	pollTrue(t, ctx, `document.body.classList.contains('comments-open') && !!document.querySelector('#commentsPanel .comment-composer .comment-composer-input')`)
	requireAll(t, ctx, "the rail opened on the brief",
		`var rail = document.getElementById('commentsPanel');`,
		[][2]string{
			{"it names the brief", `document.getElementById('commentsRailSubtitle').textContent === 'on Balances round to the cent, once'`},
			{"its title is the brief's path", `rail.title === '` + roundBriefPath + `'`},
			{"it is the desktop's non-modal rail", `rail.getAttribute('role') === 'complementary'`},
			{"the button says the rail is open", `document.querySelector('#systemFacetToc .facet-toc__comment').getAttribute('aria-expanded') === 'true'`},
			{"the empty line invites the first comment", `!!rail.querySelector('.comments-empty')`},
			// F4: a brief's comment is written into the brief's own file.
			{"the caption says where the comment goes", `rail.querySelector('.comment-composer-caption').textContent === "Saved in the brief's file by dossierx serve."`},
		})

	evalVoid(t, ctx, `(function(){ var ta = document.querySelector('#commentsPanel .comment-composer .comment-composer-input'); ta.value = 'Does <img src=x onerror="window.__briefXSS=1"> still hold?'; document.querySelector('#commentsPanel .comment-composer .comment-composer-submit').click(); })()`)
	pollTrue(t, ctx, `!!document.querySelector('#commentsPanel .comment-thread[data-thread-id]:not(.comment-thread--optimistic)')`)
	if !strings.Contains(p.briefBytes(roundBriefPath), "still hold?") {
		t.Fatalf("the thread must be written into the brief's frontmatter:\n%s", p.briefBytes(roundBriefPath))
	}

	// The live reload's fresh render marks the brief and counts the thread.
	pollTrue(t, ctx, roundMark+`.getAttribute('data-mark') === 'thread'`)
	requireAll(t, ctx, "a brief with an open thread",
		`var rail = document.getElementById('commentsPanel');
		 var body = rail.querySelector('.comment-thread .comment-body');`,
		[][2]string{
			{"the mark is the legend's open-thread dot", `getComputedStyle(` + roundMark + `, '::before').backgroundColor === getComputedStyle(document.querySelector('.brief-legend .brief-mark[data-mark="thread"]'), '::before').backgroundColor && getComputedStyle(` + roundMark + `, '::before').backgroundColor !== 'rgba(0, 0, 0, 0)'`},
			{"the mark says so", roundMark + `.getAttribute('aria-label') === '1 open thread'`},
			{"the Threads block counts it", `document.querySelector('#systemFacetToc .facet-toc__threads-note').textContent.indexOf('1 open.') === 0`},
			{"the rail stayed open through the reload", `document.body.classList.contains('comments-open')`},
			{"the hostile body is text", `body.textContent.indexOf('<img src=x') >= 0 && !body.querySelector('img') && window.__briefXSS === undefined`},
			{"the index counts it", `(function(){ var li = document.querySelector('#_briefs a[href="#` + roundBrief + `"]').parentNode; var n = li.querySelector('.briefs-index__threads'); return !!n && n.textContent === '1 open thread'; })()`},
		})

	evalVoid(t, ctx, `document.querySelector('#commentsPanel .comment-resolve').click()`)
	pollTrue(t, ctx, roundMark+`.getAttribute('data-mark') === 'locked'`)
	requireAll(t, ctx, "the thread resolved", ``, [][2]string{
		{"the Threads block says none is open", `document.querySelector('#systemFacetToc .facet-toc__threads-note').textContent.indexOf('None open.') === 0`},
		{"the index line is gone", `!document.querySelector('#_briefs .briefs-index__threads')`},
		{"the rail shows it resolved", `!!document.querySelector('#commentsPanel .comments-resolved .comment-thread--resolved')`},
	})
	if raw := p.briefBytes(roundBriefPath); !strings.Contains(raw, "status: resolved") || !strings.Contains(raw, "resolved_by: human") {
		t.Fatalf("the human's resolve must reach the file:\n%s", raw)
	}

	// F9: the rail's Comment follows the page on screen, so it is
	// "expanded" only on the brief the rail shows.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#brief-research-interviews';`, nil))
	pollTrue(t, ctx, `!document.getElementById('brief-research-interviews').hidden && document.querySelector('#systemFacetToc .facet-toc__comment').getAttribute('data-brief-id') === 'research.interviews'`)
	requireAll(t, ctx, "another brief while the rail shows the first", ``, [][2]string{
		{"the rail is still open", `document.body.classList.contains('comments-open')`},
		{"this brief's Comment is not expanded", `document.querySelector('#systemFacetToc .facet-toc__comment').getAttribute('aria-expanded') === 'false'`},
	})
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#`+roundBrief+`';`, nil))
	pollTrue(t, ctx, `document.querySelector('#systemFacetToc .facet-toc__comment').getAttribute('data-brief-id') === '`+roundBriefID+`' && document.querySelector('#systemFacetToc .facet-toc__comment').getAttribute('aria-expanded') === 'true'`)

	// Dark: the rail and the page share the theme.
	evalVoid(t, ctx, `document.querySelector('.theme-control [data-theme-choice="dark"]').click()`)
	requireAll(t, ctx, "the rail on a brief in dark", ``, [][2]string{
		{"data-theme is dark", `document.documentElement.getAttribute('data-theme') === 'dark'`},
		{"the rail is the dark card", `getComputedStyle(document.getElementById('commentsPanel')).backgroundColor === 'rgb(22, 27, 34)'`},
	})
}

// TestBriefThreads_OnAPhoneThePageFootOpensTheBottomSheet: at 390px the
// page-foot "Comment on this brief" opens the rail as the modal bottom sheet
// claims get, and the entry point B2 and B3 call opens it too.
func TestBriefThreads_OnAPhoneThePageFootOpensTheBottomSheet(t *testing.T) {
	p := briefProject(t)
	p.run("comment", "add", roundBriefPath, "--as", "agent", "--body", "Is the leftover cent still the payer's?")
	ctx := serveAndOpenLive(t, p, "#"+roundBrief)
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844))
	pollTrue(t, ctx, `document.querySelector('#`+roundBrief+` .brief-comment').disabled === false`)
	// F3: the Threads block is not drawn below 1181px, so the page-foot row
	// carries the open count, at 1024px as on the phone.
	requireAll(t, ctx, "the page-foot count on a phone",
		`var n = document.querySelector('#`+roundBrief+` .brief-comment-count');`,
		[][2]string{
			{"it says one is open", `n.textContent === '1 open thread' && !n.hidden`},
			{"it is on screen under the button", `n.getBoundingClientRect().height > 0 && n.getBoundingClientRect().top >= document.querySelector('#` + roundBrief + ` .brief-comment').getBoundingClientRect().bottom`},
		})
	runCDP(t, ctx, chromedp.EmulateViewport(1024, 900))
	pollTrue(t, ctx, `document.querySelector('#`+roundBrief+` .brief-comment-count').getBoundingClientRect().height > 0`)
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844))

	evalVoid(t, ctx, `document.querySelector('#`+roundBrief+` .brief-comment').click()`)
	// comments-open is set before GET /api/briefs/.../comments returns and
	// buildComposer runs; wait for the input the same way the desktop
	// TestBriefThreads_OpenFromThePageThenResolveClearsTheMark does.
	pollTrue(t, ctx, `document.body.classList.contains('comments-open') && !!document.querySelector('#commentsPanel .comment-composer .comment-composer-input')`)
	requireAll(t, ctx, "the bottom sheet on a brief",
		`var rail = document.getElementById('commentsPanel');
		 var r = rail.getBoundingClientRect();`,
		[][2]string{
			{"it is a modal dialog", `rail.getAttribute('role') === 'dialog' && rail.getAttribute('aria-modal') === 'true'`},
			{"the backdrop shows", `getComputedStyle(document.getElementById('commentsOverlay')).display === 'block'`},
			{"it sits at the bottom, full width", `Math.round(r.bottom) === 844 && Math.round(r.width) === 390`},
			{"it names the brief", `document.getElementById('commentsRailSubtitle').textContent === 'on Balances round to the cent, once'`},
			{"the composer is on screen", `(function(){ var c = document.querySelector('#commentsPanel .comment-composer .comment-composer-input').getBoundingClientRect(); return c.height > 0 && c.bottom <= 844; })()`},
			{"the page-foot button says the sheet is open", `document.querySelector('#` + roundBrief + ` .brief-comment').getAttribute('aria-expanded') === 'true'`},
		})
	evalVoid(t, ctx, `document.getElementById('commentsRailClose').click()`)
	pollTrue(t, ctx, `!document.body.classList.contains('comments-open')`)

	// The entry point for B2's "Approve or restore in a thread" and B3's
	// "Confirm in a thread": an id opens the brief's rail, an unknown id
	// opens nothing.
	if evalBool(t, ctx, `window.dossierxOpenBriefCommentPanel('decisions.no-such-brief')`) || evalBool(t, ctx, `document.body.classList.contains('comments-open')`) {
		t.Fatal("an unknown brief id must open nothing and say so")
	}
	if !evalBool(t, ctx, `window.dossierxOpenBriefCommentPanel('`+roundBriefID+`')`) {
		t.Fatal("the entry point must open a brief on the page")
	}
	pollTrue(t, ctx, `document.body.classList.contains('comments-open') && document.getElementById('commentsRailSubtitle').textContent === 'on Balances round to the cent, once'`)
}

// TestBriefThreads_AStaticBuildReadsThemReadOnly: a static build has no API.
// A brief with a thread opens it read only, from the baked panel, with no
// composer; a brief with none keeps its Comment disabled with its line.
func TestBriefThreads_AStaticBuildReadsThemReadOnly(t *testing.T) {
	p := briefProject(t)
	p.run("comment", "add", roundBriefPath, "--as", "agent", "--body", "Is the leftover cent still the payer's?")
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844), chromedp.Navigate(url+"#"+roundBrief), chromedp.WaitVisible("#"+roundBrief, chromedp.ByQuery))

	requireAll(t, ctx, "a static brief page with a thread",
		`var sec = document.getElementById('`+roundBrief+`');`,
		[][2]string{
			{"its button opens the thread to read", `sec.querySelector('.brief-comment').disabled === false`},
			{"its line says it is read only", `sec.querySelector('.brief-comment-note').textContent === 'Read only: comments are written through dossierx serve.' && !sec.querySelector('.brief-comment-note').hidden`},
			{"the tree marks it", roundMark + `.getAttribute('data-mark') === 'thread'`},
		})
	evalVoid(t, ctx, `document.querySelector('#`+roundBrief+` .brief-comment').click()`)
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	requireAll(t, ctx, "the read-only rail on a brief", ``, [][2]string{
		{"the thread shows", `Array.prototype.some.call(document.querySelectorAll('#commentsPanel .comment-body'), function (b) { return b.textContent.indexOf("leftover cent") >= 0; })`},
		{"no composer", `!document.querySelector('#commentsPanel .comment-composer')`},
		{"the read-only note", `!!document.querySelector('#commentsPanel .comments-readonly-note')`},
		{"no resolve control", `!document.querySelector('#commentsPanel .comment-resolve')`},
	})
	evalVoid(t, ctx, `document.getElementById('commentsRailClose').click()`)

	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#brief-decisions-no-bank-linking';`, nil))
	pollTrue(t, ctx, `!document.getElementById('brief-decisions-no-bank-linking').hidden`)
	if !evalBool(t, ctx, `document.querySelector('#brief-decisions-no-bank-linking .brief-comment').disabled`) {
		t.Fatal("a static brief with no thread has nothing to open: its Comment stays disabled")
	}

	// F10: on the wide rail a static build's Threads note states the count
	// and asks for nothing; the read-only line says why.
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#`+roundBrief+`';`, nil))
	pollTrue(t, ctx, `!document.getElementById('`+roundBrief+`').hidden && !!document.querySelector('#systemFacetToc .facet-toc__threads-note')`)
	pollTrue(t, ctx, `document.querySelector('#systemFacetToc .facet-toc__threads-note').textContent === '1 open.'`)
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#brief-research-interviews';`, nil))
	pollTrue(t, ctx, `!document.getElementById('brief-research-interviews').hidden && document.querySelector('#systemFacetToc .facet-toc__comment').getAttribute('data-brief-id') === 'research.interviews'`)
	requireAll(t, ctx, "a static brief with no thread, on the rail", `var toc = document.getElementById('systemFacetToc');`, [][2]string{
		{"the note only counts", `toc.querySelector('.facet-toc__threads-note').textContent === 'None open.'`},
		{"the read-only line says why", `toc.querySelector('.facet-toc__threads-later').textContent === 'Read only: comments are written through dossierx serve.' && !toc.querySelector('.facet-toc__threads-later').hidden`},
	})
}
