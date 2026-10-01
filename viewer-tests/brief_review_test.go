package viewertests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// A locked brief whose rests_on claim moved (NIT-200, Paper B3), driven in
// a real browser: the banner and per-claim redline, the amber pill and
// mark, the Rests on "changed" note, Confirm in a thread opening NIT-198's
// rail (one handler), light/dark/phone, the claim-card row's mobile amber,
// and the page clearing after brief reaudit --confirm.

const reviewFlowBrief = "brief-widget-flow"

func reviewPendingProject(t *testing.T) *project {
	t.Helper()
	p := newProject(t)
	writeBrief(t, p, "widget/flow.md", "---\nsummary: The widget flow.\nstatus: locked\nrests_on:\n  - "+testClaimID+"\n---\n# Widget flow\n\n## Why\n\nIt matters.\n")
	p.run("brief", "lock", "briefs/widget/flow.md", "--reason", "flow approved")
	p.writeClaim("overview.yaml", strings.Replace(draftClaimYAML, "a claim under review.", "a claim under review, since reworded.", 1))
	return p
}

func TestBriefReview_PendingPageAndRail(t *testing.T) {
	p := reviewPendingProject(t)
	ctx := serveAndOpenLive(t, p, "#"+reviewFlowBrief)
	desktopViewport(t, ctx)
	pollTrue(t, ctx, onlyShown(reviewFlowBrief)+` && !!document.querySelector('#`+reviewFlowBrief+` .brief-banner--review')`)

	requireAll(t, ctx, "the review-pending brief under serve",
		`var sec = document.getElementById('`+reviewFlowBrief+`');
		 var btn = sec.querySelector('.brief-comment');`,
		[][2]string{
			{"the section is pending", `sec.getAttribute('data-review-pending') === 'true'`},
			{"the pill says Review pending", `sec.querySelector('.brief-title-row .brief-pill').textContent.trim() === 'Review pending'`},
			{"the tree mark is review", `document.querySelector('#nav .brief-nav .sec-tab[data-target="#` + reviewFlowBrief + `"] .brief-mark').getAttribute('data-mark') === 'review'`},
			{"the banner names the claim", `sec.querySelector('.brief-banner--review .brief-banner__title').textContent.indexOf('` + testClaimID + `') >= 0`},
			{"the redline is on screen", `sec.querySelector('.brief-review-diff .claim-edit-passage--added') && sec.querySelector('.brief-review-diff .claim-edit-passage--added').getBoundingClientRect().height > 0`},
			{"the rests-on row says changed", `sec.querySelector('.brief-relation-changed') && /changed/.test(sec.querySelector('.brief-relation-changed').textContent)`},
			{"Confirm in a thread is the action", `btn.textContent === 'Confirm in a thread'`},
			{"it is enabled under serve", `btn.disabled === false`},
		})

	evalVoid(t, ctx, `document.querySelector('#`+reviewFlowBrief+` .brief-comment').click()`)
	pollTrue(t, ctx, `document.body.classList.contains('comments-open') && window.dossierxCommentRailBrief() === 'widget.flow'`)
	runCDP(t, ctx, chromedp.Sleep(600*time.Millisecond))
	requireAll(t, ctx, "the rail after Confirm in a thread",
		`var note = document.querySelector('#systemFacetToc .facet-toc__threads-note');
		 var action = document.querySelector('#systemFacetToc .facet-toc__comment-label');`,
		[][2]string{
			{"it stayed open", `document.body.classList.contains('comments-open')`},
			{"on this brief", `window.dossierxCommentRailBrief() === 'widget.flow'`},
			{"the Threads note is B3's", `note.textContent.indexOf('To confirm the brief still holds') >= 0`},
			{"the Threads action is Start a thread", `action.textContent === 'Start a thread'`},
		})
	evalVoid(t, ctx, `document.querySelector('#`+reviewFlowBrief+` .brief-comment').click()`)
	pollTrue(t, ctx, `!document.body.classList.contains('comments-open')`)
}

func TestBriefReview_LightDarkAndPhone(t *testing.T) {
	p := reviewPendingProject(t)
	url := p.renderStatic()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+reviewFlowBrief), chromedp.WaitVisible("#"+reviewFlowBrief, chromedp.ByQuery))
	suppressTransitions(t, ctx, "")

	for _, tc := range []struct{ theme, draft, fill string }{
		{"light", "rgb(154, 106, 22)", "rgba(154, 106, 22, 0.11)"},
		{"dark", "rgb(221, 169, 78)", "rgba(221, 169, 78, 0.13)"},
	} {
		evalVoid(t, ctx, `document.querySelector('.theme-control [data-theme-choice="`+tc.theme+`"]').click()`)
		requireAll(t, ctx, "the pending brief in "+tc.theme,
			`var sec = document.getElementById('`+reviewFlowBrief+`');
			 var pill = sec.querySelector('.brief-title-row .brief-pill');
			 var banner = sec.querySelector('.brief-banner--review');`,
			[][2]string{
				{"data-theme is " + tc.theme, `document.documentElement.getAttribute('data-theme') === '` + tc.theme + `'`},
				{"the pill is the draft hue", `getComputedStyle(pill).color === '` + tc.draft + `'`},
				{"the banner rule is the draft hue", `getComputedStyle(banner).borderLeftColor === '` + tc.draft + `'`},
				{"the banner fill is the draft wash", `getComputedStyle(banner).backgroundColor === '` + tc.fill + `'`},
				{"the banner title is the draft hue", `getComputedStyle(sec.querySelector('.brief-banner--review .brief-banner__title')).color === '` + tc.draft + `'`},
				{"the rests-on changed note is amber", `getComputedStyle(sec.querySelector('.brief-relation-changed')).color === '` + tc.draft + `'`},
			})
	}

	evalVoid(t, ctx, `document.querySelector('.theme-control [data-theme-choice="light"]').click()`)
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#`+testClaimID+`';`, nil))
	pollTrue(t, ctx, `!document.getElementById('`+testClaimID+`').hidden`)
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844))
	requireAll(t, ctx, "the explaining brief row on a phone",
		`var row = document.querySelector('[id="`+testClaimID+`"] .claim-brief[data-brief-id="widget.flow"]');
		 var state = row.querySelector('.claim-brief-state--pending');`,
		[][2]string{
			{"the row is on screen", `row.getBoundingClientRect().height > 0`},
			{"review pending is still amber", `getComputedStyle(state).color === 'rgb(154, 106, 22)'`},
			{"the state text is review pending", `state.textContent.indexOf('review pending') >= 0`},
		})
}

func TestBriefReview_ReauditClearsThePage(t *testing.T) {
	p := reviewPendingProject(t)
	ctx := serveAndOpenLive(t, p, "#"+reviewFlowBrief)
	pollTrue(t, ctx, onlyShown(reviewFlowBrief)+` && !!document.querySelector('#`+reviewFlowBrief+` .brief-banner--review')`)
	p.run("brief", "reaudit", "briefs/widget/flow.md", "--confirm", "--reason", "the brief still holds")
	pollTrue(t, ctx, onlyShown(reviewFlowBrief)+` && !document.querySelector('#`+reviewFlowBrief+` .brief-banner--review')`)
	requireAll(t, ctx, "the brief after reaudit",
		`var sec = document.getElementById('`+reviewFlowBrief+`');`,
		[][2]string{
			{"it is no longer pending", `sec.getAttribute('data-review-pending') !== 'true'`},
			{"the pill is Locked", `sec.querySelector('.brief-title-row .brief-pill').textContent.trim() === 'Locked'`},
			{"the tree mark is locked", `document.querySelector('#nav .brief-nav .sec-tab[data-target="#` + reviewFlowBrief + `"] .brief-mark').getAttribute('data-mark') === 'locked'`},
			{"the body is the plain article", `!!sec.querySelector('article.brief-body') && !sec.querySelector('.brief-review-diff')`},
		})
}

func TestBriefReview_LiveReloadKeepsNavAndMain(t *testing.T) {
	p := reviewPendingProject(t)
	ctx := serveAndOpenLive(t, p, "#"+reviewFlowBrief)
	pollTrue(t, ctx, onlyShown(reviewFlowBrief))
	evalVoid(t, ctx, `window.__dxNav = document.getElementById('nav'); window.__dxMain = document.querySelector('main'); window.__dxRoot = document.documentElement;`)
	if err := os.WriteFile(filepath.Join(p.dir, "briefs", "widget", "flow.md"), []byte("---\nsummary: The widget flow.\nstatus: locked\nrests_on:\n  - "+testClaimID+"\n---\n# Widget flow\n\n## Why\n\nIt matters.\n\nA line added while open.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pollTrue(t, ctx, `document.querySelector('#`+reviewFlowBrief+` .brief-body').textContent.indexOf('A line added while open.') >= 0`)
	requireAll(t, ctx, "live reload of a pending brief",
		``,
		[][2]string{
			{"the document element stayed", `document.documentElement === window.__dxRoot`},
			{"nav was swapped, not the whole document", `document.getElementById('nav') !== window.__dxNav`},
			{"main was swapped", `document.querySelector('main') !== window.__dxMain`},
			{"the pending banner is still there", `!!document.querySelector('#` + reviewFlowBrief + ` .brief-banner--review')`},
		})
}
