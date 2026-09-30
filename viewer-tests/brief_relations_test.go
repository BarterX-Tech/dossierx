package viewertests

// BRIEFS IN A CLAIM'S RELATIONSHIPS (NIT-202, Paper board B5).
//
// A claim never stores a link to a brief; the viewer derives a BRIEFS group in
// the relationships panel from the briefs' rests_on and the claim's own
// internal sources. internal/render owns WHICH rows appear and what they say
// (TestRenderWith_BriefsInAClaimsRelationships). This suite owns what only a
// browser can answer: the row is laid out as the board draws it — on desktop
// four lanes (folder, linked title, state, lock badge) with no dot; on a phone
// the ordinary two-line mobile relationship row (dot and title, then folder
// and state with the badge at the right) — the "pin out of date" state paints
// in the draft hue in both themes, and serving and rendering it leaves the
// claim file byte-identical.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
)

const briefRelationsRounding = "---\nsummary: How other apps round.\n---\n# Rounding in other apps\n\nBody.\n"

func newBriefRelationsProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	write := func(rel, body string) {
		path := filepath.Join(p.dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("briefs/decisions/round-once.md", "---\nsummary: Why balances round once.\nstatus: locked\nrests_on:\n  - "+testClaimID+"\n---\n# Balances round to the cent, once\n\nBody.\n")
	write("briefs/research/rounding.md", briefRelationsRounding)
	write("briefs/research/conversion.md", "---\nsummary: Older notes.\nstatus: locked\n---\n# Currency conversion notes\n\nBody.\n")
	write("briefs/guidance/voice.md", "---\nsummary: How we write.\nstatus: locked\n---\n# Voice\n\nBody.\n")
	sum := sha256.Sum256([]byte(briefRelationsRounding))
	p.writeClaim("overview.yaml", `id: `+testClaimID+`
facet: contract
module: widget
status: draft
summary: Fixture claim used by the engine test corpus.
body: |
  a claim that cites two briefs [1] [2].
sources:
  - ref: 1
    kind: internal
    title: Rounding research
    path: briefs/research/rounding.md
    sha256: `+hex.EncodeToString(sum[:])+`
  - ref: 2
    kind: internal
    title: Conversion notes
    path: briefs/research/conversion.md
    sha256: 0000000000000000000000000000000000000000000000000000000000000000
rests_on:
  none: true
  reason: viewer-test fixture, not backed by any doctrine claim
`)
	return p
}

// briefRowGeometry reads, for every brief row of the fixture claim, the boxes
// and paint the layout assertions need.
const briefRowGeometryJS = `(function () {
	var card = document.getElementById('widget.contract.overview');
	var d = card.querySelector('details.claim-links');
	d.open = true;
	var probe = document.createElement('span');
	probe.style.color = 'var(--status-draft)';
	document.body.appendChild(probe);
	var draft = getComputedStyle(probe).color;
	probe.style.color = 'var(--faint)';
	var faint = getComputedStyle(probe).color;
	probe.remove();
	function box(el) { if (!el) return null; var r = el.getBoundingClientRect(); return {l: r.left, r: r.right, t: r.top, b: r.bottom, cy: r.top + r.height / 2}; }
	var rows = Array.prototype.map.call(card.querySelectorAll('li.claim-brief'), function (li) {
		var state = li.querySelector('.claim-brief-state');
		var sep = li.querySelector('.claim-brief-sep');
		return {
			id: li.getAttribute('data-brief-id') || '',
			href: (li.querySelector('a.claim-brief-ref') || {getAttribute: function () { return ''; }}).getAttribute('href'),
			row: box(li),
			dot: getComputedStyle(li.querySelector('.claim-relationship-dot')).display,
			title: box(li.querySelector('.claim-brief-ref')),
			folder: box(li.querySelector('.claim-brief-folder')),
			state: box(state),
			stateText: state.textContent,
			stateColor: getComputedStyle(state).color,
			sepColor: sep ? getComputedStyle(sep).color : '',
			sepDisplay: sep ? getComputedStyle(sep).display : '',
			badge: box(li.querySelector('.claim-relationship-badge'))
		};
	});
	var group = card.querySelector('.claim-relationship-direction--briefs');
	// Each sub-head and the first row under it: the board's gap between them.
	var subgroups = Array.prototype.map.call(card.querySelectorAll('.claim-brief-subgroup'), function (g) {
		return {head: box(g.querySelector('.claim-brief-subhead')), first: box(g.querySelector('li.claim-brief'))};
	});
	return {
		draft: draft,
		faint: faint,
		subgroups: subgroups,
		groupVisible: !!group && group.getBoundingClientRect().height > 0,
		chip: card.querySelector('details.claim-links .claim-footer-chip-label').textContent,
		derivedNote: getComputedStyle(card.querySelector('.claim-brief-derived-note')).display,
		rows: rows
	};
})()`

type briefBox struct{ L, R, T, B, Cy float64 }

type briefRowGeometry struct {
	Draft     string `json:"draft"`
	Faint     string `json:"faint"`
	Subgroups []struct {
		Head  *briefBox `json:"head"`
		First *briefBox `json:"first"`
	} `json:"subgroups"`
	GroupVisible bool   `json:"groupVisible"`
	Chip         string `json:"chip"`
	DerivedNote  string `json:"derivedNote"`
	Rows         []struct {
		ID         string    `json:"id"`
		Href       string    `json:"href"`
		Row        *briefBox `json:"row"`
		Dot        string    `json:"dot"`
		Title      *briefBox `json:"title"`
		Folder     *briefBox `json:"folder"`
		State      *briefBox `json:"state"`
		StateText  string    `json:"stateText"`
		StateColor string    `json:"stateColor"`
		SepColor   string    `json:"sepColor"`
		SepDisplay string    `json:"sepDisplay"`
		Badge      *briefBox `json:"badge"`
	} `json:"rows"`
}

func readBriefRows(t *testing.T, ctx context.Context) briefRowGeometry {
	t.Helper()
	var g briefRowGeometry
	evalInto(t, ctx, briefRowGeometryJS, &g)
	if !g.GroupVisible {
		t.Fatal("the BRIEFS group is not laid out in the open relationships panel")
	}
	// round-once explains the claim; conversion (pin broken) and rounding
	// (pin holds) are cited. The guidance brief is on no card.
	if len(g.Rows) != 3 || g.Rows[0].ID != "decisions.round-once" || g.Rows[1].ID != "research.conversion" || g.Rows[2].ID != "research.rounding" {
		t.Fatalf("brief rows = %+v, want round-once, conversion, rounding", g.Rows)
	}
	if g.Chip != "3 relationships" {
		t.Fatalf("relationships chip = %q, want the three brief rows counted", g.Chip)
	}
	if g.Rows[0].Href != "#brief-decisions-round-once" {
		t.Fatalf("explaining row links to %q, want the brief page section", g.Rows[0].Href)
	}
	if g.Rows[1].StateText != "· pin out of date" || g.Rows[1].StateColor != g.Draft {
		t.Fatalf("the broken pin must read \"pin out of date\" in the draft hue %s, got %q in %s", g.Draft, g.Rows[1].StateText, g.Rows[1].StateColor)
	}
	if g.Rows[2].StateColor == g.Draft {
		t.Fatalf("a pin that holds must not paint in the draft hue")
	}
	return g
}

// TestBriefRowsLayOutAsTheBoardDrawsThem opens the served viewer at the
// boards' desktop and phone widths (1440, 390) and at 540px, in light and
// dark, and holds each brief row to its board shape; then proves the claim
// file was never written.
//
// 540px is the width the four desktop lanes squeezed a title to a few words
// a line (NIT-202 audit F4): past the phone tier, but with a panel too narrow
// for ~314px of fixed lanes beside the title. There the row must already be
// the two-line mobile row, whose title takes the whole line.
func TestBriefRowsLayOutAsTheBoardDrawsThem(t *testing.T) {
	p := newBriefRelationsProject(t)
	before := p.claimBytes()
	base := p.ensureServe()
	ctx := browserContext(t)
	for _, theme := range []string{"light", "dark"} {
		for _, width := range []int64{1440, 540, 390} {
			runCDP(t, ctx, chromedp.EmulateViewport(width, 900), chromedp.Navigate(base+"/"+widgetPage))
			pollTrue(t, ctx, `document.readyState === 'complete' && !!document.getElementById('widget.contract.overview')`)
			runCDP(t, ctx, chromedp.Evaluate(`document.documentElement.setAttribute('data-theme', '`+theme+`')`, nil))
			settleLayoutTransitions(t, ctx)
			g := readBriefRows(t, ctx)
			// Sub-head to first row: 8px on the board's desktop, 7px on its
			// phone (the phone tier is <=520px).
			wantGap := 8.0
			if width <= 520 {
				wantGap = 7
			}
			if len(g.Subgroups) != 2 {
				t.Fatalf("%s %dpx: want the two sub-groups, got %d", theme, width, len(g.Subgroups))
			}
			for i, sg := range g.Subgroups {
				if gap := sg.First.T - sg.Head.B; gap < wantGap-0.5 || gap > wantGap+0.5 {
					t.Errorf("%s %dpx: sub-group %d has %.2fpx from its sub-head to its first row, want %.0fpx", theme, width, i, gap, wantGap)
				}
			}
			for _, r := range g.Rows {
				if r.Title == nil || r.Folder == nil || r.State == nil || r.Badge == nil {
					t.Fatalf("%s %dpx: row %s is missing a part: %+v", theme, width, r.ID, r)
				}
				if width > 540 {
					// One line, four lanes: folder, title, state, badge; no dot.
					if r.Dot != "none" {
						t.Errorf("%s desktop: row %s draws a dot; the board's desktop brief row has none", theme, r.ID)
					}
					if !(r.Folder.R <= r.Title.L && r.Title.R <= r.State.L && r.State.R <= r.Badge.L) {
						t.Errorf("%s desktop: row %s lanes out of order: %+v", theme, r.ID, r)
					}
					for _, part := range []*briefBox{r.Folder, r.State, r.Badge} {
						if d := part.Cy - r.Title.Cy; d > 3 || d < -3 {
							t.Errorf("%s desktop: row %s is not one line: %+v", theme, r.ID, r)
						}
					}
					continue
				}
				// Two lines: dot + title, then folder · state and the badge.
				if r.Dot == "none" {
					t.Errorf("%s %dpx: row %s lost its status dot", theme, width, r.ID)
				}
				if r.Folder.T < r.Title.B-1 || r.Badge.T < r.Title.B-1 {
					t.Errorf("%s %dpx: row %s line two is not below the title: %+v", theme, width, r.ID, r)
				}
				if !(r.Folder.R <= r.State.L+1 && r.State.R <= r.Badge.L+1) {
					t.Errorf("%s %dpx: row %s line two out of order: %+v", theme, width, r.ID, r)
				}
				if r.Row.R-r.Badge.R > 1 {
					t.Errorf("%s %dpx: row %s badge is not at the row's right edge: %+v", theme, width, r.ID, r)
				}
				// The title has line one to itself, less the dot and its gap.
				if rowW, titleW := r.Row.R-r.Row.L, r.Title.R-r.Title.L; titleW < rowW-20 {
					t.Errorf("%s %dpx: row %s title is squeezed to %.0fpx of a %.0fpx row", theme, width, r.ID, titleW, rowW)
				}
				if r.StateText != "" && (r.SepDisplay == "none" || r.SepColor != g.Faint) {
					t.Errorf("%s %dpx: row %s separator must show in --faint %s, got display %s colour %s", theme, width, r.ID, g.Faint, r.SepDisplay, r.SepColor)
				}
			}
			if width <= 520 && g.DerivedNote != "none" {
				t.Errorf("%s %dpx: the head's long note must drop to the bare word \"derived\"", theme, width)
			}
		}
	}
	if got := p.claimBytes(); string(got) != string(before) {
		t.Fatalf("rendering the derived brief rows changed the claim file:\nbefore:\n%s\nafter:\n%s", before, got)
	}
}

// TestBriefRowClickLandsOnThatBrief is NIT-202 audit F1: a BRIEFS row's title
// is a link to the brief, so clicking it on a claim card must open that
// brief's page section — shown, addressed by the hash, headed by its title —
// not fall through to Home.
func TestBriefRowClickLandsOnThatBrief(t *testing.T) {
	p := newBriefRelationsProject(t)
	base := p.ensureServe()
	ctx := withInstantScroll(t, browserContext(t))
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(base+"/"+widgetPage))
	pollTrue(t, ctx, `document.readyState === 'complete' && !!document.getElementById('widget.contract.overview')`)
	runCDP(t, ctx, chromedp.Evaluate(`(function () {
		var c = document.getElementById('widget.contract.overview');
		c.querySelector('details.claim-links').open = true;
		c.querySelector('a.claim-brief-ref').scrollIntoView({block: 'center'});
	})()`, nil))
	settleLayoutTransitions(t, ctx)
	href := evalString(t, ctx, `document.getElementById('widget.contract.overview').querySelector('a.claim-brief-ref').getAttribute('href')`)
	if href != "#brief-decisions-round-once" {
		t.Fatalf("the first brief row links to %q, want the round-once brief's section", href)
	}
	runCDP(t, ctx, chromedp.Click(`[id="widget.contract.overview"] a.claim-brief-ref[href="#brief-decisions-round-once"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `location.hash === '#brief-decisions-round-once'`)
	settleLayoutTransitions(t, ctx)
	requireAll(t, ctx, "the clicked brief's page", `var s = document.getElementById('brief-decisions-round-once');
		var h = document.getElementById('brief-decisions-round-once_title');
		var home = document.getElementById('_home');`, [][2]string{
		{"the brief's section is shown", `!!s && !s.hidden && s.getBoundingClientRect().height > 0`},
		{"the section is headed by the brief's title", `!!h && h.textContent.trim() === 'Balances round to the cent, once'`},
		{"Home is not what the click landed on", `!home || home.hidden`},
		{"the claim's module page is left", `document.getElementById('widget.contract.overview').closest('.module-section').hidden`},
	})
}
