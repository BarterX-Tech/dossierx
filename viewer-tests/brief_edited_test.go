package viewertests

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
)

// A locked brief whose file moved since its approval (NIT-199, Paper B2),
// driven in a real browser: the banner, the three views and the switch, the
// view the reader chose following them from brief to brief and across a live
// reload, the rail following the view, light and dark, the phone, and the
// thread button. The markup is pinned at its owner, internal/render
// (TestRender_BriefEditedSinceApproval); what is asserted here is what only a
// browser can show.

const moneyBrief = "brief-voice-money"

const moneyApprovedMD = "---\nsummary: Plain, specific, never scolding.\nstatus: locked\n---\n# Talking about money\n\n## Principles\n\nSay the amount before the person.\n\nNever use the word debt.\n\n![An arrow](arrow.png)\n\n## Tone\n\nKind, never scolding.\n\n## Words we avoid\n\n- debt\n- owe\n"

const moneyCurrentMD = "---\nsummary: Plain, specific, never scolding.\nstatus: locked\n---\n# Talking about money\n\n## Principles\n\nSay the amount before the person.\n\nAvoid debt and owe in the interface.\n\n![An arrow](arrow.png)\n\n## Reminders\n\nA reminder names the expense.\n\n## Words we avoid\n\n- debt\n- owe\n"

// editedBriefProject holds two briefs in voice/, each locked through brief
// lock and then edited, so both are brief-content-drift: money (a changed
// paragraph, an added section, an image) and tone (one changed paragraph).
func editedBriefProject(t *testing.T) *project {
	t.Helper()
	p := newProject(t)
	write := func(rel string, data []byte) {
		path := filepath.Join(p.dir, "briefs", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		for y := 0; y < 20; y++ {
			img.Set(x, y, color.RGBA{R: 194, G: 202, B: 213, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	write("voice/arrow.png", buf.Bytes())
	write("voice/money.md", []byte(moneyApprovedMD))
	write("voice/tone.md", []byte("---\nsummary: Warm.\nstatus: locked\n---\n## Tone\n\nWarm and short.\n"))
	p.run("brief", "lock", "briefs/voice/money.md", "--reason", "voice approved")
	p.run("brief", "lock", "briefs/voice/tone.md", "--reason", "tone approved")
	write("voice/money.md", []byte(moneyCurrentMD))
	write("voice/tone.md", []byte("---\nsummary: Warm.\nstatus: locked\n---\n## Tone\n\nWarm, short and kind.\n"))
	return p
}

// renderStaticDrifted writes the static viewer of a project whose check
// fails on brief-content-drift. check renders the viewer before its ledger
// gate refuses, so the page a reader opens after a failing check shows the
// edit; the helper requires that refusal, so a check that stopped failing
// here (or stopped writing the viewer) fails the test.
func (p *project) renderStaticDrifted() string {
	p.t.Helper()
	out, err := exec.Command(p.bin, "--config", p.config, "--format", "text", "check").CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte("brief-content-drift")) {
		p.t.Fatalf("check on an edited locked brief should fail with brief-content-drift, got err=%v\n%s", err, out)
	}
	index := filepath.Join(p.dir, "build", "viewer", "index.html")
	if _, err := os.Stat(index); err != nil {
		p.t.Fatalf("check did not write %s: %v", index, err)
	}
	return "file://" + index
}

// shownView is the one view of a brief's card on screen.
func shownView(brief string) string {
	return `(function(){ var v = document.querySelectorAll('#` + brief + ` .brief-view:not([hidden])'); return v.length === 1 ? v[0].getAttribute('data-view') : 'views shown: ' + v.length; })()`
}

func tocRows() string {
	return `Array.prototype.map.call(document.querySelectorAll('#systemFacetToc .facet-toc__item strong'), function (s) { return s.textContent; }).join('|')`
}

func TestBriefEdited_ThreeViewsFollowTheReader(t *testing.T) {
	p := editedBriefProject(t)
	url := p.renderStaticDrifted()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+moneyBrief), chromedp.WaitVisible("#"+moneyBrief, chromedp.ByQuery))
	pollTrue(t, ctx, `(function(){ var i = document.querySelector('#`+moneyBrief+` .brief-view:not([hidden]) img.md-img'); return !!i && i.complete; })()`)

	requireAll(t, ctx, "an edited brief opened by its hash",
		`var sec = document.getElementById('`+moneyBrief+`');
		 var row = document.querySelector('#nav .sec-tab[data-target="#`+moneyBrief+`"]');
		 var banner = sec.querySelector('.brief-banner');`,
		[][2]string{
			{"the wide pill says edited since approval", `sec.querySelector('.brief-title-row .brief-pill').textContent === 'Edited since approval'`},
			{"the sidebar mark is the edited one", `row.querySelector('.brief-mark').getAttribute('data-mark') === 'edited' && getComputedStyle(row.querySelector('.brief-mark'), '::before').backgroundColor === getComputedStyle(sec.querySelector('.brief-title-row .brief-pill')).color`},
			{"the banner is on screen with its count", `banner.getBoundingClientRect().height > 0 && banner.querySelector('.brief-banner__title').textContent === 'Edited after you approved it · 3 passages changed'`},
			{"the meta line names the approval and its reason", `sec.querySelector('.brief-meta .brief-wide').textContent.indexOf('Approved ') === 0 && sec.querySelector('.brief-meta .brief-wide').textContent.indexOf('“voice approved”') > 0`},
			{"Changes is the view shown", shownView(moneyBrief) + ` === 'changes'`},
			{"its switch segment is pressed", `sec.querySelector('.brief-view-switch__seg[aria-pressed="true"]').textContent === 'Changes'`},
			{"the approved paragraph is struck", `getComputedStyle(sec.querySelector('.brief-view:not([hidden]) .claim-edit-passage--removed')).textDecorationLine === 'line-through'`},
			{"the image loads in Changes", `sec.querySelector('.brief-view:not([hidden]) img.md-img').naturalWidth === 40`},
			{"the rail lists the shown view's headings, not the struck one", tocRows() + ` === 'Principles|Reminders|Words we avoid'`},
			{"the thread button is disabled until threads exist", `sec.querySelector('.brief-comment[data-brief-thread]').disabled && sec.querySelector('.brief-comment-note').getBoundingClientRect().height > 0`},
		})

	// The switch keeps its width, to the subpixel, whichever segment is
	// pressed (F11: the pressed label's weight used to move it by 1px).
	evalVoid(t, ctx, `window.__switchWidths = ['changes', 'approved', 'current', 'changes'].map(function (v) {
		document.querySelector('#`+moneyBrief+` .brief-view-switch__seg[data-brief-view="' + v + '"]').click();
		return document.querySelector('#`+moneyBrief+` .brief-view-switch').getBoundingClientRect().width;
	});`)
	if got := evalString(t, ctx, `window.__switchWidths.join(',')`); !evalBool(t, ctx, `window.__switchWidths.every(function (w) { return w === window.__switchWidths[0]; })`) {
		t.Fatalf("the view switch changes width as the pressed segment moves: %s", got)
	}

	evalVoid(t, ctx, `document.querySelector('#`+moneyBrief+` .brief-view-switch__seg[data-brief-view="approved"]').click()`)
	pollTrue(t, ctx, shownView(moneyBrief)+` === 'approved'`)
	pollTrue(t, ctx, `(function(){ var i = document.querySelector('#`+moneyBrief+` .brief-view:not([hidden]) img.md-img'); return !!i && i.complete; })()`)
	requireAll(t, ctx, "the Approved view",
		`var sec = document.getElementById('`+moneyBrief+`'); var v = sec.querySelector('.brief-view:not([hidden])');`,
		[][2]string{
			{"it is the approved text", `v.textContent.indexOf('Never use the word debt.') >= 0 && v.textContent.indexOf('Reminders') < 0 && v.textContent.indexOf('Kind, never scolding.') >= 0`},
			{"it carries no marks", `!v.querySelector('.claim-edit-passage')`},
			{"the image loads", `v.querySelector('img.md-img').naturalWidth === 40`},
			{"the caption follows the view", `sec.querySelector('[data-caption-for="approved"]').getBoundingClientRect().width > 0 && sec.querySelector('[data-caption-for="changes"]').hidden`},
			{"the rail lists the approved headings", tocRows() + ` === 'Principles|Tone|Words we avoid'`},
		})

	evalVoid(t, ctx, `document.querySelector('#`+moneyBrief+` .brief-view-switch__seg[data-brief-view="current"]').click()`)
	pollTrue(t, ctx, shownView(moneyBrief)+` === 'current'`)
	requireAll(t, ctx, "the Current view",
		`var v = document.querySelector('#`+moneyBrief+` .brief-view:not([hidden])');`,
		[][2]string{
			{"no removed passage", `v.textContent.indexOf('Never use') < 0 && v.textContent.indexOf('Kind, never scolding.') < 0`},
			{"the changed run is ruled in the blocked colour", `getComputedStyle(v.querySelector('.brief-passage--current'), '::before').backgroundColor === getComputedStyle(document.querySelector('#` + moneyBrief + ` .brief-title-row .brief-pill')).color`},
			{"and captioned", `Array.prototype.map.call(v.querySelectorAll('.brief-passage__caption'), function (c) { return c.textContent; }).join('|') === 'changed since approval|changed since approval'`},
			{"the rail lists the current headings", tocRows() + ` === 'Principles|Reminders|Words we avoid'`},
		})

	// The choice follows the reader to another edited brief, and back.
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#brief-voice-tone';`, nil))
	pollTrue(t, ctx, `!document.getElementById('brief-voice-tone').hidden`)
	if got := evalString(t, ctx, shownView("brief-voice-tone")); got != "current" {
		t.Fatalf("another edited brief opened on %q; the reader chose current", got)
	}
	runCDP(t, ctx, chromedp.Evaluate(`location.hash = '#`+moneyBrief+`';`, nil))
	pollTrue(t, ctx, shownView(moneyBrief)+` === 'current'`)

	// Once the thread rail's entry point exists (NIT-198's
	// window.dossierxOpenBriefCommentPanel), the button opens it on this
	// brief.
	evalVoid(t, ctx, `window.dossierxOpenBriefCommentPanel = function (id) { window.__openedBrief = id; return true; };
		document.querySelector('#`+moneyBrief+` .brief-view-switch__seg[data-brief-view="changes"]').click();`)
	pollTrue(t, ctx, `!document.querySelector('#`+moneyBrief+` .brief-comment[data-brief-thread]').disabled`)
	evalVoid(t, ctx, `document.querySelector('#`+moneyBrief+` .brief-comment[data-brief-thread]').click()`)
	if got := evalString(t, ctx, `String(window.__openedBrief)`); got != "voice.money" {
		t.Fatalf("the thread button opened %q, want the brief's id voice.money", got)
	}
}

func TestBriefEdited_LightDarkAndPhone(t *testing.T) {
	p := editedBriefProject(t)
	url := p.renderStaticDrifted()
	ctx := withInstantScroll(t, browserContext(t))
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url+"#"+moneyBrief), chromedp.WaitVisible("#"+moneyBrief, chromedp.ByQuery))
	suppressTransitions(t, ctx, "")

	for _, tc := range []struct{ theme, warn, card, track, on string }{
		{"light", "rgb(158, 59, 54)", "rgb(255, 255, 255)", "rgb(227, 231, 237)", "rgb(255, 255, 255)"},
		{"dark", "rgb(236, 138, 131)", "rgb(22, 27, 34)", "rgb(27, 33, 43)", "rgb(56, 66, 79)"},
	} {
		evalVoid(t, ctx, `document.querySelector('.theme-control [data-theme-choice="`+tc.theme+`"]').click()`)
		requireAll(t, ctx, "the edited brief in "+tc.theme,
			`var sec = document.getElementById('`+moneyBrief+`');`,
			[][2]string{
				{"data-theme is " + tc.theme, `document.documentElement.getAttribute('data-theme') === '` + tc.theme + `'`},
				{"the pill is the blocked colour", `getComputedStyle(sec.querySelector('.brief-title-row .brief-pill')).color === '` + tc.warn + `'`},
				{"the banner title and rule are the blocked colour", `getComputedStyle(sec.querySelector('.brief-banner__title')).color === '` + tc.warn + `' && getComputedStyle(sec.querySelector('.brief-banner')).borderLeftColor === '` + tc.warn + `'`},
				{"the card is the card colour", `getComputedStyle(sec.querySelector('.brief-compare')).backgroundColor === '` + tc.card + `'`},
				{"the switch track", `getComputedStyle(sec.querySelector('.brief-view-switch')).backgroundColor === '` + tc.track + `'`},
				{"the pressed segment", `getComputedStyle(sec.querySelector('.brief-view-switch__seg[aria-pressed="true"]')).backgroundColor === '` + tc.on + `'`},
			})
	}

	evalVoid(t, ctx, `document.querySelector('.theme-control [data-theme-choice="light"]').click()`)
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844))
	requireAll(t, ctx, "the edited brief on a phone",
		`var sec = document.getElementById('`+moneyBrief+`');
		 var card = sec.querySelector('.brief-compare');
		 var sw = sec.querySelector('.brief-view-switch');
		 var first = sec.querySelector('.brief-view:not([hidden])');
		 var cs = getComputedStyle(card);
		 var inner = card.getBoundingClientRect().width - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight) - parseFloat(cs.borderLeftWidth) - parseFloat(cs.borderRightWidth);`,
		[][2]string{
			{"the phone pill is the one word", `sec.querySelector('.brief-meta-row .brief-narrow .brief-pill').textContent === 'Edited'`},
			{"the switch spans the card", `Math.abs(sw.getBoundingClientRect().width - inner) < 1`},
			{"the switch sits above the body", `sw.getBoundingClientRect().bottom <= first.getBoundingClientRect().top`},
			{"its three segments share the width", `(function(){ var s = sec.querySelectorAll('.brief-view-switch__seg'); return Math.abs(s[0].getBoundingClientRect().width - s[2].getBoundingClientRect().width) < 1; })()`},
			{"the caption is left out", `getComputedStyle(sec.querySelector('.brief-compare__caption')).display === 'none'`},
			{"the banner's rule id is left out", `getComputedStyle(sec.querySelector('.brief-banner__rule')).display === 'none'`},
			{"the thread button is 44px at the page foot", `sec.querySelector('.brief-comment[data-brief-thread]').getBoundingClientRect().height === 44`},
			{"nothing is wider than the phone", `document.documentElement.scrollWidth <= 390`},
		})
}

// A live reload swaps the brief's section for a fresh render; the view the
// reader chose is theirs, not the render's, and survives the swap.
func TestBriefEdited_LiveReloadKeepsTheView(t *testing.T) {
	p := editedBriefProject(t)
	ctx := serveAndOpenLive(t, p, "#"+moneyBrief)
	pollTrue(t, ctx, shownView(moneyBrief)+` === 'changes'`)
	evalVoid(t, ctx, `document.querySelector('#`+moneyBrief+` .brief-view-switch__seg[data-brief-view="approved"]').click()`)
	pollTrue(t, ctx, shownView(moneyBrief)+` === 'approved'`)

	if err := os.WriteFile(filepath.Join(p.dir, "briefs", "voice", "money.md"), []byte(moneyCurrentMD+"\nA line added while the page was open.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pollTrue(t, ctx, `document.querySelector('#`+moneyBrief+` .brief-view[data-view="current"]').textContent.indexOf('A line added while the page was open.') >= 0`)
	requireAll(t, ctx, "the edited brief after a live reload",
		`var sec = document.getElementById('`+moneyBrief+`');`,
		[][2]string{
			{"the Approved view is still the one shown", shownView(moneyBrief) + ` === 'approved'`},
			{"its segment is still pressed", `sec.querySelector('.brief-view-switch__seg[aria-pressed="true"]').textContent === 'Approved'`},
			{"the banner counts the new edit", `sec.querySelector('.brief-banner__title').textContent === 'Edited after you approved it · 4 passages changed'`},
		})
}
