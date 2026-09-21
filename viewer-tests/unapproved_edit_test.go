package viewertests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// approvedBodyYAML / editedBodyYAML are the same claim before and after the
// edit. They differ by one replaced line inside a three-line body, which is
// the shape the panel has to get right: a reader must see one removal and one
// addition, not a whole body struck through and re-added.
const approvedBodyYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
  Every widget carries an identifier and a creation timestamp.
  A widget without an id is not a widget.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const editedBodyYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
  Every widget carries an identifier and a creation timestamp.
  An object with no identifier is not a widget and is not stored.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// editedAfterApproval drives the exact lifecycle the feature is about — lock,
// unlock, edit — through the real CLI, so the viewer under test is reading a
// lock store this engine actually wrote rather than a hand-built fixture that
// could encode an assumption the writer does not hold.
func editedAfterApproval(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("overview.yaml", approvedBodyYAML)
	p.run("claim", "lock", testClaimID, "--reason", "three sentences, checked against the schema")
	p.run("claim", "unlock", testClaimID, "--reason", "the id sentence is wrong")
	// unlock rewrites the claim file's status; the edit has to land on that
	// rewritten file, not on the copy this test remembers writing.
	current, err := os.ReadFile(filepath.Join(p.claimsDir, "overview.yaml"))
	if err != nil {
		t.Fatalf("read back the unlocked claim: %v", err)
	}
	if !strings.Contains(string(current), "status: draft") {
		t.Fatalf("unlock must leave the claim a draft, got:\n%s", current)
	}
	p.writeClaim("overview.yaml", editedBodyYAML)
	return p
}

// The chip is the whole reason a reader looks twice. DRAFT is true of an
// edited claim and of a claim nobody ever approved, and those are different
// jobs; the word has to say which one this is.
func TestEditedClaimChipSaysItWasApproved(t *testing.T) {
	p := editedAfterApproval(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .k .label .pill[data-dx-edited="1"]')`)

	requireAll(t, ctx, "the edited claim's status chip", `
		var pill = document.querySelector('[id="`+testClaimID+`"] .k .label .pill');
		var text = (pill.textContent || '').replace(/\s+/g, ' ').trim().toLowerCase();
	`, [][2]string{
		{"the chip names the edit", `text.indexOf('edited') >= 0`},
		{"the chip says an approval existed", `text.indexOf('was approved') >= 0`},
		{"the chip no longer reads only DRAFT", `text !== 'draft'`},
		// It stays the DRAFT variant: the claim IS a draft, and a fourth
		// colour would say this state is unrelated to the one the reader
		// already knows.
		{"the chip keeps the draft hue", `pill.classList.contains('pv')`},
		{"the chip keeps its padlock", `!!pill.querySelector('.dx-icon')`},
	})
}

// The diff is the answer to "then what changed?". Its contract is the board's
// (DON-0 / C9H-0): the approved passage above the current one, ruled and
// tinted, struck through and not, with the words that moved marked inside
// each — and all of it as PROSE, not as markdown source.
func TestEditedClaimShowsTheWordingThatMoved(t *testing.T) {
	p := editedAfterApproval(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff')`)

	requireAll(t, ctx, "the edited-since-approval diff", `
		var card = document.querySelector('[id="`+testClaimID+`"]');
		var removed = Array.from(card.querySelectorAll('.claim-edit-passage--removed'));
		var added = Array.from(card.querySelectorAll('.claim-edit-passage--added'));
		var text = function (nodes) { return nodes.map(function (n) { return n.textContent; }).join('\n'); };
	`, [][2]string{
		{"exactly the approved passage is shown as removed", `removed.length === 1 && text(removed).indexOf('A widget without an id is not a widget.') >= 0`},
		{"exactly the new passage is shown as added", `added.length === 1 && text(added).indexOf('An object with no identifier is not a widget and is not stored.') >= 0`},
		// Colour is not the only carrier: this has to survive a monochrome
		// print and a colour-vision deficiency.
		{"the removal is struck through", `getComputedStyle(removed[0]).textDecorationLine.indexOf('line-through') >= 0`},
		{"the addition is not struck through", `getComputedStyle(added[0]).textDecorationLine.indexOf('line-through') < 0`},
		// The approving words are the only part of a ledger record a machine
		// cannot generate, and they are what the removed wording was approved
		// AS.
		{"the bar names who approved it", `card.querySelector('.claim-edit-bar').textContent.indexOf('approved') >= 0`},
	})
}

// The board puts a two-segment switch on the bar, and opens on Changes: a
// claim that was approved and then rewritten has to say what moved before a
// reader reads a word of it.
func TestEditedClaimSwitchesBetweenChangesAndCurrent(t *testing.T) {
	p := editedAfterApproval(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-bar')`)

	requireAll(t, ctx, "the default state", `
		var card = document.querySelector('[id="`+testClaimID+`"]');
		var segs = Array.from(card.querySelectorAll('.claim-edit-switch-seg'));
		var body = card.querySelector('.claim-body');
	`, [][2]string{
		{"there are exactly two segments", `segs.length === 2 && segs[0].textContent === 'Changes' && segs[1].textContent === 'Current'`},
		{"Changes is the one selected", `segs[0].getAttribute('aria-pressed') === 'true' && segs[1].getAttribute('aria-pressed') === 'false'`},
		{"the diff is showing", `!!card.querySelector('.claim-edit-diff')`},
		{"the claim's own body is not", `!body || !body.getClientRects().length`},
		{"the bar says what state this is", `card.querySelector('.claim-edit-bar').textContent.indexOf('Edited since it was approved') >= 0`},
	})

	runCDP(t, ctx, chromedp.Click(`[id="`+testClaimID+`"] .claim-edit-switch-seg:nth-of-type(2)`, chromedp.ByQuery))
	pollTrue(t, ctx, `!document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff')`)

	requireAll(t, ctx, "after switching to Current", `
		var card = document.querySelector('[id="`+testClaimID+`"]');
		var segs = Array.from(card.querySelectorAll('.claim-edit-switch-seg'));
		var body = card.querySelector('.claim-body');
	`, [][2]string{
		{"Current is the one selected", `segs[1].getAttribute('aria-pressed') === 'true' && segs[0].getAttribute('aria-pressed') === 'false'`},
		{"the diff is gone", `!card.querySelector('.claim-edit-diff')`},
		{"the claim's own body is showing", `!!body && body.getClientRects().length > 0`},
		{"the bar says what state this is, and counts what differs", `/Showing the current wording/.test(card.querySelector('.claim-edit-bar').textContent) && /1 passage differs/.test(card.querySelector('.claim-edit-bar').textContent)`},
	})

	// And back, because a switch that only goes one way is a link.
	runCDP(t, ctx, chromedp.Click(`[id="`+testClaimID+`"] .claim-edit-switch-seg:nth-of-type(1)`, chromedp.ByQuery))
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff')`)
	if !evalBool(t, ctx, `(function(){ var b = document.querySelector('[id="`+testClaimID+`"] .claim-body'); return !b || !b.getClientRects().length; })()`) {
		t.Fatal("switching back to Changes must hide the claim's own body again")
	}
}

// ON A PHONE the banner is one CARD with a row per finding (Paper EH1-0),
// where the desktop band's single sentence has no room for a second fact. The
// card is tinted while every row shares a hue and neutral once they disagree,
// because a tinted surface is a severity claim and a surface cannot make two.
func TestEditedClaimIsARowOnThePhoneBannerCard(t *testing.T) {
	p := editedAfterApproval(t)
	// A second claim that is blocked by an unapproved dependency, so the card
	// carries an alarm row AND a draft row and has to go neutral.
	p.writeClaim("blocked.yaml", `id: widget.contract.blocked
facet: contract
module: widget
status: draft
body: |
  a claim that rests on something nobody approved.
rests_on:
  - `+testClaimID+`
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(390, 844),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `(function(){ var c = document.getElementById('statusStripCard'); return !!c && c.querySelectorAll('.status-strip-finding').length > 1; })()`)

	requireAll(t, ctx, "the banner card", `
		var card = document.getElementById('statusStripCard');
		var rows = Array.from(card.querySelectorAll('.status-strip-finding'));
		var text = rows.map(function (r) { return r.textContent.replace(/\s+/g, ' ').trim(); });
		var tones = rows.map(function (r) { return r.getAttribute('data-tone'); });
		var count = document.getElementById('statusStripCardCount');
	`, [][2]string{
		{"the edited claims get their own row", `text.some(function (t) { return t.indexOf('edits that have not been approved') >= 0; })`},
		{"that row is the draft hue", `tones[text.findIndex(function (t) { return t.indexOf('edits that have not been approved') >= 0; })] === 'draft'`},
		{"the blocked claims get their own row", `text.some(function (t) { return t.indexOf('blocked by unapproved dependencies') >= 0; })`},
		{"that row is the alarm hue", `tones[text.findIndex(function (t) { return t.indexOf('blocked by unapproved dependencies') >= 0; })] === 'alarm'`},
		{"the head counts the rows", `count && Number(count.textContent) === rows.length`},
		// The tint rule. Two hues on one surface would make the card assert a
		// severity neither row has.
		{"two hues make the card neutral", `!card.classList.contains('status-strip-card--alarm') && !card.classList.contains('status-strip-card--draft')`},
		// At this width the card IS the banner: the desktop band is off.
		{"the card is the form on screen", `card.getClientRects().length > 0`},
		{"the desktop band is not", `!document.getElementById('statusStripToggle').getClientRects().length`},
		{"every row is a real control", `rows.every(function (r) { return r.tagName === 'BUTTON'; })`},
		{"every row carries a dot and a chevron", `rows.every(function (r) { return !!r.querySelector('.status-strip-finding-dot') && !!r.querySelector('.status-strip-finding-chevron'); })`},
	})

	// It routes to the Issues screen rather than to a screen of its own: an
	// edited claim is already a readiness cause there, and a second list of
	// the same rows is a second place for the count to be wrong.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		Array.from(document.querySelectorAll('#statusStripFindings .status-strip-finding'))
			.filter(function (r) { return r.textContent.indexOf('edits that have not been approved') >= 0; })[0].click();
	})()`, nil))
	pollTrue(t, ctx, `(function(){ var v = document.getElementById('issuesView'); return !!v && !v.hidden; })()`)
	requireAll(t, ctx, "the Issues screen the row opens", `
		var view = document.getElementById('issuesView');
		var pressed = Array.from(view.querySelectorAll('.status-severity-chip[aria-pressed="true"]'));
	`, [][2]string{
		{"it lists the edited claim by what happened to it", `view.textContent.indexOf('approved, then rewritten') >= 0`},
		// On a real corpus the unfiltered screen is hundreds of dependency
		// rows and the claims this row named are somewhere inside them. A way
		// in that lands a reader in a list they then have to search has not
		// answered the question it asked.
		{"it arrives filtered to exactly one severity", `pressed.length === 1`},
		{"that severity is the row's own", `pressed[0].classList.contains('status-severity-chip--needs_you')`},
	})
}

// With nothing but edited claims the phone card has one hue, so it takes it.
func TestPhoneBannerCardIsTintedWhenEveryRowAgrees(t *testing.T) {
	p := editedAfterApproval(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(390, 844),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `(function(){ var c = document.getElementById('statusStripCard'); return !!c && c.querySelectorAll('.status-strip-finding').length === 1; })()`)

	requireAll(t, ctx, "a single-hue banner card", `
		var card = document.getElementById('statusStripCard');
		var rows = Array.from(card.querySelectorAll('.status-strip-finding'));
	`, [][2]string{
		{"one row", `rows.length === 1`},
		{"it names the edit", `rows[0].textContent.indexOf('edits that have not been approved') >= 0`},
		{"the whole card takes the draft hue", `card.classList.contains('status-strip-card--draft')`},
		{"and not the alarm hue", `!card.classList.contains('status-strip-card--alarm')`},
	})
}

// A claim nobody ever approved is the other half of the word DRAFT and must
// stay clean: no chip change, no panel, no band.
func TestNeverApprovedDraftShowsNoEditSurfaces(t *testing.T) {
	p := newProject(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)

	requireAll(t, ctx, "a draft that was never approved", `
		var pill = document.querySelector('[id="`+testClaimID+`"] .k .label .pill');
		var card = document.getElementById('statusStripCard');
		var rows = card ? Array.from(card.querySelectorAll('.status-strip-finding')) : [];
	`, [][2]string{
		{"its chip still reads DRAFT", `(pill.textContent || '').replace(/\s+/g, ' ').trim().toLowerCase() === 'draft'`},
		{"it carries no edited-since-approval bar", `!document.querySelector('[id="` + testClaimID + `"] .claim-edit-bar')`},
		{"no row claims anything was edited", `!rows.some(function (r) { return r.textContent.indexOf('edits that have not been approved') >= 0; })`},
	})
}

// rewrappedApprovedYAML / rewrappedEditedYAML are the case the word-level
// refinement exists for: ONE word changed, and the paragraph re-wrapped
// afterwards — which is what an editor with format-on-save does. Diffed by
// physical line this renders as three struck lines and three added ones,
// which is the same shape a rewritten paragraph makes.
const rewrappedApprovedYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  **The field is outward, not internal.** A claim in the support authority already depends on it.
  An observer revision changing is enumerated there among the material changes that revoke a
  support verdict. A change nobody can see revokes nothing.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const rewrappedEditedYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  **The field is outward, not private.** A claim in the support authority already depends on
  it. An observer revision changing is enumerated there among the material changes that
  revoke a support verdict. A change nobody can see revokes nothing.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// A one-word change must read as a one-word change, whatever the author's
// editor did to the line breaks afterwards — and the prose around it has to
// survive as prose.
func TestEditedClaimMarksTheWordAndKeepsTheProse(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("overview.yaml", rewrappedApprovedYAML)
	p.run("claim", "lock", testClaimID, "--reason", "the wording as it stands")
	p.run("claim", "unlock", testClaimID, "--reason", "one word is wrong")
	p.writeClaim("overview.yaml", rewrappedEditedYAML)

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff')`)

	requireAll(t, ctx, "a one-word change in a re-wrapped passage", `
		var card = document.querySelector('[id="`+testClaimID+`"]');
		var removed = Array.from(card.querySelectorAll('.claim-edit-word--removed'));
		var added = Array.from(card.querySelectorAll('.claim-edit-word--added'));
		var passages = Array.from(card.querySelectorAll('.claim-edit-passage--removed, .claim-edit-passage--added'));
		var diff = card.querySelector('.claim-edit-diff');
	`, [][2]string{
		{"exactly one word is marked as removed", `removed.length === 1 && removed[0].textContent.trim() === 'internal.'`},
		{"exactly one word is marked as added", `added.length === 1 && added[0].textContent.trim() === 'private.'`},
		// The whole point: NOT three struck lines and three added ones.
		{"the change is one passage each way", `passages.length === 2`},
		// The words around the change are the sentence they belong to.
		{"the surrounding sentence is kept", `passages[0].textContent.indexOf('already depends on') >= 0`},
		// Markdown is rendered, not shown as source, and the mark did not
		// break the emphasis it sits inside.
		{"the passage renders as prose", `diff.textContent.indexOf('**') < 0`},
		{"the emphasis around the marked word survived", `!!diff.querySelector('strong')`},
		// The mark is on the word, not on the gap after it.
		{"the mark does not paint the trailing space", `!/\s$/.test(removed[0].textContent)`},
	})
}

// The desktop band is the other form of the same head, and it must still be
// the one on screen at 1440 — the card is the phone's answer to a narrower
// column, not a replacement for a shape that already worked.
func TestDesktopKeepsTheBandAndPhoneTakesTheCard(t *testing.T) {
	p := editedAfterApproval(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)

	requireAll(t, ctx, "the desktop banner", `
		var band = document.getElementById('statusStripToggle');
		var card = document.getElementById('statusStripCard');
		var title = document.getElementById('statusStripTitle');
	`, [][2]string{
		{"the band is on screen", `band.getClientRects().length > 0`},
		{"the card is not", `!card.getClientRects().length`},
		{"the band keeps its Show issues action", `document.getElementById('statusStripAction').textContent === 'Show issues'`},
		// Both forms are written from the same findings, so they can differ
		// in shape and never in what they say.
		{"the band's sentence is the card's first row", `title.textContent === card.querySelector('.status-strip-finding-text').textContent`},
		{"and it names what is waiting", `title.textContent.indexOf('edits that have not been approved') >= 0`},
	})

	// The same document, narrowed: the band goes and the card arrives. The
	// swap is a media query, so it needs no reload — which is the property
	// being asserted, not just the two end states.
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844))
	pollTrue(t, ctx, `(function(){
		var band = document.getElementById('statusStripToggle');
		var card = document.getElementById('statusStripCard');
		return !band.getClientRects().length && card.getClientRects().length > 0;
	})()`)

	// R09.6: the band is still a way into the Issues screen. Checked last,
	// because opening that screen hides .content-area and with it both forms
	// of the banner, so a width assertion after this one would be measuring
	// two hidden elements and passing for the wrong reason.
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900))
	pollTrue(t, ctx, `document.getElementById('statusStripToggle').getClientRects().length > 0`)
	runCDP(t, ctx, chromedp.Click("#statusStripToggle", chromedp.ByQuery))
	pollTrue(t, ctx, `document.getElementById('issuesView').hidden === false`)
	// Unfiltered, unlike a card row: the band's sentence names one finding
	// but the band stands for the whole facet.
	if evalBool(t, ctx, `!!document.querySelector('#issuesView .status-severity-chip[aria-pressed="true"]')`) {
		t.Fatal("the desktop band opens the Issues screen unfiltered; only a card row names what it filters to")
	}
}

// The bar's width is the reading COLUMN's, not the window's, so no media
// query can see the moment its line runs out of room. The Current state
// carries the long one ("1 passage differs from the approval of 20 Sep") and
// it broke mid-date at 1440 before this. The sentence must stay whole at
// every width the reading view is drawn at; the switch is what moves.
func TestEditBarNeverBreaksItsSentence(t *testing.T) {
	p := editedAfterApproval(t)
	ctx := browserContext(t)

	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-bar')`)

	for _, width := range []int{1440, 1100, 900, 760, 620} {
		runCDP(t, ctx, chromedp.EmulateViewport(int64(width), 900))
		// The viewport change and the repaint that follows it are both
		// asynchronous, and this test measures boxes — so it waits for the
		// document to actually be that wide before reading any of them.
		// Measuring first is how this test failed against a layout that was
		// correct: it was reading the previous width's boxes.
		pollTrue(t, ctx, `document.documentElement.clientWidth === `+strconv.Itoa(width))
		for _, seg := range []int{1, 0} {
			runCDP(t, ctx, chromedp.Evaluate(`(function(){
				var segs = document.querySelectorAll('[id="`+testClaimID+`"] .claim-edit-switch-seg');
				segs[`+strconv.Itoa(seg)+`].click();
			})()`, nil))
			// The click rebuilds the bar, so wait for the state it asked for
			// before measuring the bar it rebuilt.
			pollTrue(t, ctx, `(function(){
				var segs = document.querySelectorAll('[id="`+testClaimID+`"] .claim-edit-switch-seg');
				return segs[`+strconv.Itoa(seg)+`].getAttribute('aria-pressed') === 'true' &&
					segs[0].getBoundingClientRect().width > 0;
			})()`)
			requireAll(t, ctx, "the edit bar at "+strconv.Itoa(width)+"px, segment "+strconv.Itoa(seg), `
				var bar = document.querySelector('[id="`+testClaimID+`"] .claim-edit-bar');
				var title = bar.querySelector('.claim-edit-bar-title');
				var meta = bar.querySelector('.claim-edit-bar-meta');
				var lineOf = function (el) {
					var lh = parseFloat(getComputedStyle(el).lineHeight);
					return Math.round(el.getBoundingClientRect().height / lh);
				};
			`, [][2]string{
				// Nothing overruns the bar, at any width. This is the clause
				// that failed when the meta was made unshrinkable: it kept its
				// sentence whole by running off the end of the card.
				{"the title stays inside the bar", `title.getBoundingClientRect().right <= bar.getBoundingClientRect().right + 1`},
				{"the machine line stays inside the bar", `meta.getBoundingClientRect().right <= bar.getBoundingClientRect().right + 1`},
				// And across the range the reading column is actually drawn at
				// (its own max-width is 760), both sentences stay on one line
				// — the switch drops to its own row instead.
				// And wherever the bar is wide enough to hold the sentence,
				// the sentence is on one line. That gate is the whole defect:
				// at 1440 the bar is 694px and the line needs 474, and it
				// still broke mid-date — because the switch was taking the
				// room on its own row. The switch now drops instead.
				//
				// The gate is MEASURED, not a viewport guess. The bar's width
				// is the reading column's, which at a 900px window in a
				// one-module fixture is 242px — narrower than 44 characters
				// of mono can ever be, so two lines there is the right answer
				// and an assertion against it would be an assertion against
				// arithmetic.
				{"the title is one line when the bar can hold it", `bar.clientWidth < 460 || lineOf(title) === 1`},
				{"the machine line is one line when the bar can hold it", `bar.clientWidth < 460 || lineOf(meta) === 1`},
			})
		}
	}
}

// stripLedgerContent removes the approved claim from every ledger record,
// leaving the hash — which is exactly the shape of a record written before
// the approved wording was kept, and the state every claim in a real corpus
// upgraded from an earlier release is in.
func stripLedgerContent(t *testing.T, p *project) {
	t.Helper()
	path := filepath.Join(p.dir, "build", "ledger", "lock-store.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock store: %v", err)
	}
	var store map[string]any
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatalf("decode lock store: %v", err)
	}
	ledger, ok := store["ledger"].(map[string]any)
	if !ok || len(ledger) == 0 {
		t.Fatalf("lock store carries no ledger to strip: %s", raw)
	}
	stripped := 0
	for _, v := range ledger {
		record, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if _, had := record["content"]; had {
			delete(record, "content")
			stripped++
		}
	}
	if stripped == 0 {
		t.Fatal("no record carried content, so this fixture is not testing the legacy shape")
	}
	out, err := json.Marshal(store)
	if err != nil {
		t.Fatalf("encode lock store: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write lock store: %v", err)
	}
}

// A claim whose approved wording was never kept has no second version to
// show. Offering the switch anyway did worse than give the reader a useless
// control: choosing "Changes" hid the claim's body to make room for a diff
// that did not exist, and the claim rendered with no content at all.
func TestClaimWithNoRetainedWordingKeepsItsBodyAndOffersNoSwitch(t *testing.T) {
	p := editedAfterApproval(t)
	stripLedgerContent(t, p)

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-bar')`)

	requireAll(t, ctx, "a claim whose approved wording is not on the record", `
		var card = document.querySelector('[id="`+testClaimID+`"]');
		var body = card.querySelector('.claim-body');
	`, [][2]string{
		// The claim is still a claim. This is the defect: its body was gone.
		{"its body is on screen", `!!body && body.getClientRects().length > 0`},
		{"it says why there is nothing to compare", `card.querySelector('.claim-edit-note').textContent.indexOf('nothing to compare against') >= 0`},
		// A control whose two states show the same thing is not a control.
		{"it offers no switch", `card.querySelectorAll('.claim-edit-switch-seg').length === 0`},
		{"and draws no diff", `!card.querySelector('.claim-edit-diff')`},
		// The chip and the bar still say what happened to it.
		{"the chip still names the state", `card.querySelector('.pill').textContent.toLowerCase().indexOf('was approved') >= 0`},
		{"the bar still names the approval", `card.querySelector('.claim-edit-bar').textContent.indexOf('approved') >= 0`},
	})
}

// The diff IS the body, shown a passage at a time, so it truncates like one.
// A claim that clamps to four lines in one state and runs to full height in
// the other is the same claim behaving as two components.
func TestDiffTruncatesLikeAClaimBody(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("overview.yaml", longEditedApprovedYAML)
	p.run("claim", "lock", testClaimID, "--reason", "the wording as it stands")
	p.run("claim", "unlock", testClaimID, "--reason", "one passage is wrong")
	p.writeClaim("overview.yaml", longEditedCurrentYAML)

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-body-disclosure--edit')`)

	requireAll(t, ctx, "a long diff", `
		var card = document.querySelector('[id="`+testClaimID+`"]');
		var wrap = card.querySelector('.claim-body-disclosure--edit');
		var diff = wrap.querySelector('.claim-edit-diff');
		var toggle = wrap.querySelector(':scope > .claim-body-disclosure__toggle');
		var line = parseFloat(getComputedStyle(diff).lineHeight);
	`, [][2]string{
		{"it is wrapped in the same disclosure a body gets", `!!wrap && !!diff`},
		{"it starts collapsed", `wrap.classList.contains('claim-body-disclosure--collapsed')`},
		{"it offers the same control", `!!toggle && !toggle.hidden && toggle.textContent.indexOf('more') >= 0`},
		// Clamped, not merely wrapped. The bound is generous on purpose: the
		// clamp counts LINE BOXES, and the diff's passages carry margins and
		// (when tinted) padding, so a collapsed diff is four lines of prose
		// plus the box the first passage sits in — close to a body's four
		// lines, never an exact multiple of them.
		{"and is clamped near a body's four lines", `diff.getBoundingClientRect().height <= line * 8`},
	})

	collapsed := evalInt(t, ctx, `Math.round(document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff').getBoundingClientRect().height)`)

	// And it opens, showing the whole diff.
	runCDP(t, ctx, chromedp.Click(`[id="`+testClaimID+`"] .claim-body-disclosure--edit > .claim-body-disclosure__toggle`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelector('[id="`+testClaimID+`"] .claim-body-disclosure--edit').classList.contains('claim-body-disclosure--expanded')`)
	if !evalBool(t, ctx, `(function(){
		var card = document.querySelector('[id="`+testClaimID+`"]');
		return card.querySelectorAll('.claim-edit-passage--removed').length === 1 &&
			card.querySelectorAll('.claim-edit-passage--added').length === 1 &&
			card.querySelector('.claim-edit-passage--added').getClientRects().length > 0;
	})()`) {
		t.Fatal("expanding the diff must show the passages that moved")
	}
	// The clamp did something: the whole diff is taller than the collapsed
	// one by a wide margin. Without this the assertions above would pass over
	// a "clamp" that clamped nothing.
	expanded := evalInt(t, ctx, `Math.round(document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff').getBoundingClientRect().height)`)
	if expanded <= collapsed*2 {
		t.Fatalf("the collapsed diff (%dpx) must be far shorter than the open one (%dpx)", collapsed, expanded)
	}
}

// Switching back and forth must leave exactly one diff and no orphaned
// wrappers behind it — the disclosure wraps the diff after it is built, so a
// repaint that only looked for the unwrapped node would strand the old one.
func TestSwitchRoundTripsWithoutStrandingTheDiff(t *testing.T) {
	p := editedAfterApproval(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.EmulateViewport(1440, 900),
		chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff')`)

	for pass := 0; pass < 3; pass++ {
		runCDP(t, ctx, chromedp.Click(`[id="`+testClaimID+`"] .claim-edit-switch-seg:nth-of-type(2)`, chromedp.ByQuery))
		pollTrue(t, ctx, `!document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff')`)
		if !evalBool(t, ctx, `(function(){
			var card = document.querySelector('[id="`+testClaimID+`"]');
			var body = card.querySelector('.claim-body');
			return card.querySelectorAll('.claim-body-disclosure--edit').length === 0 &&
				!!body && body.getClientRects().length > 0;
		})()`) {
			t.Fatalf("pass %d: Current must remove the diff and its wrapper and show the body", pass)
		}

		runCDP(t, ctx, chromedp.Click(`[id="`+testClaimID+`"] .claim-edit-switch-seg:nth-of-type(1)`, chromedp.ByQuery))
		pollTrue(t, ctx, `!!document.querySelector('[id="`+testClaimID+`"] .claim-edit-diff')`)
		if !evalBool(t, ctx, `(function(){
			var card = document.querySelector('[id="`+testClaimID+`"]');
			return card.querySelectorAll('.claim-edit-diff').length === 1 &&
				card.querySelectorAll('.claim-body-disclosure--edit').length === 1 &&
				card.querySelectorAll('.claim-edit-passage').length > 0;
		})()`) {
			t.Fatalf("pass %d: Changes must leave exactly one diff, wrapped once", pass)
		}
	}
}

// longEditedApprovedYAML / longEditedCurrentYAML are long enough that the
// four-line clamp has something to do: without them a short diff needs no
// disclosure and the test would pass over an absent control.
const longEditedApprovedYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents, and every part of it is
  written down here rather than inferred from the code that implements it.

  Every widget carries an identifier and a creation timestamp. The identifier is
  assigned once and never reissued, and the timestamp records the moment the
  widget was accepted rather than the moment it was requested.

  A widget without an id is not a widget.

  The catalogue is the only place a widget may be declared. A widget that exists
  in code and not in the catalogue is a defect in the catalogue, not a widget.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const longEditedCurrentYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents, and every part of it is
  written down here rather than inferred from the code that implements it.

  Every widget carries an identifier and a creation timestamp. The identifier is
  assigned once and never reissued, and the timestamp records the moment the
  widget was accepted rather than the moment it was requested.

  An object with no identifier is not a widget and is not stored.

  The catalogue is the only place a widget may be declared. A widget that exists
  in code and not in the catalogue is a defect in the catalogue, not a widget.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// ---------------------------------------------------------------------
// The fields that moved
// ---------------------------------------------------------------------

// metadataOnlyApprovedYAML and metadataOnlyEditedYAML differ ONLY in a field
// the approval hash signs and the reader never sees in the prose. This is the
// state six of the fourteen affected claims in the Curtainly corpus are in:
// the checks were retargeted by a code audit and not one word of the claim
// changed.
const metadataOnlyApprovedYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
  Every widget carries an identifier and a creation timestamp.
steps:
  - the shell resolves the widget id
  - the store returns the widget or a stated absence
audit_notes:
  - 2026-01-01 approved against the schema as written
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const metadataOnlyEditedYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
  Every widget carries an identifier and a creation timestamp.
steps:
  - the shell resolves the widget handle
  - the store returns the widget or a stated absence
audit_notes:
  - 2026-01-01 approved against the schema as written
  - 2026-02-01 unlocked to retarget step one after the code audit
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// editedFieldsOnly runs the same real lifecycle as editedAfterApproval, but
// the edit lands on fields instead of on the body.
func editedFieldsOnly(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("overview.yaml", metadataOnlyApprovedYAML)
	p.run("claim", "lock", testClaimID, "--reason", "two steps, checked against the code")
	p.run("claim", "unlock", testClaimID, "--reason", "step one names the wrong thing")
	p.writeClaim("overview.yaml", metadataOnlyEditedYAML)
	return p
}

// A claim whose PROSE never moved must still show the reader what did.
//
// Before this, the panel's whole answer was "Also changed: steps." — it named
// a change the reader could not see and sent them to the YAML file to find out
// what they were being asked about. The rows below are that answer.
func TestEditedFieldsAreShownAndNotMerelyNamed(t *testing.T) {
	p := editedFieldsOnly(t)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-edit-bar", chromedp.ByQuery))

	if !evalBool(t, ctx, `(function(){
  var c = document.getElementById('`+testClaimID+`');
  var note = c.querySelector('.claim-edit-note-text');
  return !!note && /wording is unchanged/i.test(note.textContent) && /2 fields moved/.test(note.textContent);
 })()`) {
		var got string
		runCDP(t, ctx, chromedp.Evaluate(`(document.querySelector('.claim-edit-note-text')||{}).textContent||''`, &got))
		t.Fatalf("a metadata-only edit must say the wording is unchanged and count the fields; got %q", got)
	}

	// One row per field that moved, named as the claim file names it.
	var names []string
	runCDP(t, ctx, chromedp.Evaluate(
		`Array.from(document.querySelectorAll('.claim-edit-field-name')).map(function(e){return e.textContent})`, &names))
	if len(names) != 2 || names[0] != "audit_notes" || names[1] != "steps" {
		t.Fatalf("want rows for audit_notes and steps in that order, got %v", names)
	}

	// And each row is a real comparison, with the words that moved marked.
	if !evalBool(t, ctx, `(function(){
  var rows = document.querySelectorAll('.claim-edit-field');
  var steps = null;
  rows.forEach(function (row) {
    if (row.querySelector('.claim-edit-field-name').textContent === 'steps') { steps = row; }
  });
  if (!steps) { return false; }
  var removed = steps.querySelector('.claim-edit-field-hunk--removed');
  var added = steps.querySelector('.claim-edit-field-hunk--added');
  if (!removed || !added) { return false; }
  return /widget id/.test(removed.textContent) && /widget handle/.test(added.textContent) &&
    !!removed.querySelector('.claim-edit-word--removed') && !!added.querySelector('.claim-edit-word--added');
 })()`) {
		var debug string
		runCDP(t, ctx, chromedp.Evaluate(`(function(){
   var out = [];
   document.querySelectorAll('.claim-edit-field').forEach(function (row) {
     out.push({field: row.querySelector('.claim-edit-field-name').textContent,
       hunks: Array.from(row.querySelectorAll('.claim-edit-field-hunk')).map(function (h) {
         return {cls: h.className, text: h.textContent.slice(0, 120), marks: h.querySelectorAll('.claim-edit-word--removed, .claim-edit-word--added').length};
       })});
   });
   return JSON.stringify(out);
  })()`, &debug))
		t.Fatalf("the steps row must show the approved value above the current one with the changed words marked: %s", debug)
	}
}

// A metadata-only edit has ONE wording, so it must offer no Changes/Current
// switch — and the claim's own body must stay on screen. The switch was
// removed for exactly this case once already, when choosing "Changes" hid the
// body to make room for a diff that did not exist; adding the field rows must
// not quietly bring it back.
func TestEditedFieldsOfferNoSwitchAndKeepTheBody(t *testing.T) {
	p := editedFieldsOnly(t)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-edit-bar", chromedp.ByQuery))

	if evalInt(t, ctx, `document.querySelectorAll('.claim-edit-switch-seg').length`) != 0 {
		t.Fatal("a claim with one wording must offer no Changes/Current switch")
	}
	if !evalBool(t, ctx, `(function(){
  var body = document.getElementById('`+testClaimID+`').querySelector('.claim-body');
  return !!body && body.getBoundingClientRect().height > 0 &&
    /smallest unit this project documents/.test(body.textContent);
 })()`) {
		t.Fatal("the claim's own body must stay on screen beside the field rows")
	}
}

// The rows open by default and the toggle closes them. Default-open is the
// same decision the bar makes: a claim that was approved and then edited opens
// showing what moved, because that is the thing a reader has to know before
// they read a word of it.
func TestEditedFieldsOpenByDefaultAndToggleClosed(t *testing.T) {
	p := editedFieldsOnly(t)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-edit-fields-toggle", chromedp.ByQuery))

	if !evalBool(t, ctx, `(function(){
  var list = document.querySelector('.claim-edit-fields-list');
  var toggle = document.querySelector('.claim-edit-fields-toggle');
  return !list.hidden && toggle.getAttribute('aria-expanded') === 'true' && /^Hide/.test(toggle.textContent);
 })()`) {
		t.Fatal("the field rows must open by default, with the toggle reporting it")
	}

	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.claim-edit-fields-toggle').click()`, nil))
	pollTrue(t, ctx, `document.querySelector('.claim-edit-fields-list').hidden === true`)
	if !evalBool(t, ctx, `(function(){
  var toggle = document.querySelector('.claim-edit-fields-toggle');
  return toggle.getAttribute('aria-expanded') === 'false' && /^Show/.test(toggle.textContent);
 })()`) {
		t.Fatal("closing the rows must be reported on the toggle, not only in the layout")
	}

	// Back open, so the control is a control and not a one-way door.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelector('.claim-edit-fields-toggle').click()`, nil))
	pollTrue(t, ctx, `document.querySelector('.claim-edit-fields-list').hidden === false`)
	if evalInt(t, ctx, `document.querySelectorAll('.claim-edit-field-hunk').length`) == 0 {
		t.Fatal("reopening must bring the comparison back")
	}
}

// A field is YAML and reaches the page through innerHTML. Markup inside a
// claim file must arrive as TEXT, and markdown inside it must stay literal —
// the row is quoting the file, and a row that renders what it quotes is not
// quoting it.
func TestEditedFieldsShowFileTextAndNotRenderedMarkup(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	const approved = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
audit_notes:
  - "before <b>bold</b> and **stars**"
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
	const edited = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
audit_notes:
  - "after <b>bold</b> and **stars**"
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
	p.writeClaim("overview.yaml", approved)
	p.run("claim", "lock", testClaimID, "--reason", "one note, checked")
	p.run("claim", "unlock", testClaimID, "--reason", "the note is wrong")
	p.writeClaim("overview.yaml", edited)

	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-edit-field-hunk", chromedp.ByQuery))

	if !evalBool(t, ctx, `(function(){
  var hunks = document.querySelectorAll('.claim-edit-field-hunk');
  if (!hunks.length) { return false; }
  for (var i = 0; i < hunks.length; i++) {
    // The tag must be TEXT on the page...
    if (!/<b>bold<\/b>/.test(hunks[i].textContent)) { return false; }
    // ...and must not have become an element.
    if (hunks[i].querySelector('b') || hunks[i].querySelector('strong')) { return false; }
    // The markdown stars stay literal too.
    if (!/\*\*stars\*\*/.test(hunks[i].textContent)) { return false; }
  }
  return true;
 })()`) {
		var debug string
		runCDP(t, ctx, chromedp.Evaluate(
			`JSON.stringify(Array.from(document.querySelectorAll('.claim-edit-field-hunk')).map(function(h){return h.innerHTML.slice(0,200)}))`, &debug))
		t.Fatalf("a field row must quote the claim file as text: %s", debug)
	}
}

// A claim whose BODY moved and whose fields also moved shows both: the
// passages, then the field rows. The field rows must not replace the prose
// diff, and must not appear beside the "Current" wording either.
func TestEditedBodyAndFieldsShowBothInTheChangesView(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	const approved = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
  A widget without an id is not a widget.
steps:
  - the shell resolves the widget id
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
	const edited = `id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  A widget is the smallest unit this project documents.
  An object with no identifier is not a widget and is not stored.
steps:
  - the shell resolves the widget handle
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
	p.writeClaim("overview.yaml", approved)
	p.run("claim", "lock", testClaimID, "--reason", "checked")
	p.run("claim", "unlock", testClaimID, "--reason", "both the sentence and the step are wrong")
	p.writeClaim("overview.yaml", edited)

	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible(".claim-edit-diff", chromedp.ByQuery))

	if evalInt(t, ctx, `document.querySelectorAll('.claim-edit-passage--removed').length`) == 0 {
		t.Fatal("the prose diff must still be drawn when the body moved")
	}
	if evalInt(t, ctx, `document.querySelectorAll('.claim-edit-diff .claim-edit-field-name').length`) != 1 {
		t.Fatal("the field rows must sit inside the changes view, under the passages")
	}
	if !evalBool(t, ctx, `(function(){
  var diff = document.querySelector('.claim-edit-diff');
  var passages = diff.querySelectorAll('.claim-edit-passage');
  var fields = diff.querySelector('.claim-edit-fields');
  if (!passages.length || !fields) { return false; }
  // The field block comes after the last passage in document order.
  return !!(passages[passages.length - 1].compareDocumentPosition(fields) & Node.DOCUMENT_POSITION_FOLLOWING);
 })()`) {
		t.Fatal("the field rows must follow the passages, not precede them")
	}

	// Switching to Current puts the claim's own body back and takes the whole
	// changes view — field rows included — away.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
  var segs = document.querySelectorAll('.claim-edit-switch-seg');
  segs[segs.length - 1].click();
 })()`, nil))
	pollTrue(t, ctx, `document.querySelectorAll('.claim-edit-field-name').length === 0`)
	if !evalBool(t, ctx, `(function(){
  var body = document.getElementById('`+testClaimID+`').querySelector('.claim-body');
  return !!body && body.getBoundingClientRect().height > 0;
 })()`) {
		t.Fatal("Current must show the claim's own body")
	}
}
