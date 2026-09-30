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
	probe.remove();
	function box(el) { if (!el) return null; var r = el.getBoundingClientRect(); return {l: r.left, r: r.right, t: r.top, b: r.bottom, cy: r.top + r.height / 2}; }
	var rows = Array.prototype.map.call(card.querySelectorAll('li.claim-brief'), function (li) {
		var state = li.querySelector('.claim-brief-state');
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
			badge: box(li.querySelector('.claim-relationship-badge'))
		};
	});
	var group = card.querySelector('.claim-relationship-direction--briefs');
	return {
		draft: draft,
		groupVisible: !!group && group.getBoundingClientRect().height > 0,
		chip: card.querySelector('details.claim-links .claim-footer-chip-label').textContent,
		derivedNote: getComputedStyle(card.querySelector('.claim-brief-derived-note')).display,
		rows: rows
	};
})()`

type briefBox struct{ L, R, T, B, Cy float64 }

type briefRowGeometry struct {
	Draft        string `json:"draft"`
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

// TestBriefRowsLayOutAsTheBoardDrawsThem opens the served viewer at desktop
// and phone widths, in light and dark, and holds each brief row to its board
// shape; then proves the claim file was never written.
func TestBriefRowsLayOutAsTheBoardDrawsThem(t *testing.T) {
	p := newBriefRelationsProject(t)
	before := p.claimBytes()
	base := p.ensureServe()
	ctx := browserContext(t)
	for _, theme := range []string{"light", "dark"} {
		for _, width := range []int64{1280, 375} {
			runCDP(t, ctx, chromedp.EmulateViewport(width, 900), chromedp.Navigate(base+"/"+widgetPage))
			pollTrue(t, ctx, `document.readyState === 'complete' && !!document.getElementById('widget.contract.overview')`)
			runCDP(t, ctx, chromedp.Evaluate(`document.documentElement.setAttribute('data-theme', '`+theme+`')`, nil))
			settleLayoutTransitions(t, ctx)
			g := readBriefRows(t, ctx)
			for _, r := range g.Rows {
				if r.Title == nil || r.Folder == nil || r.State == nil || r.Badge == nil {
					t.Fatalf("%s %dpx: row %s is missing a part: %+v", theme, width, r.ID, r)
				}
				if width > 520 {
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
				} else {
					// Two lines: dot + title, then folder · state and the badge.
					if r.Dot == "none" {
						t.Errorf("%s mobile: row %s lost its status dot", theme, r.ID)
					}
					if r.Folder.T < r.Title.B-1 || r.Badge.T < r.Title.B-1 {
						t.Errorf("%s mobile: row %s line two is not below the title: %+v", theme, r.ID, r)
					}
					if !(r.Folder.R <= r.State.L+1 && r.State.R <= r.Badge.L+1) {
						t.Errorf("%s mobile: row %s line two out of order: %+v", theme, r.ID, r)
					}
					if r.Row.R-r.Badge.R > 1 {
						t.Errorf("%s mobile: row %s badge is not at the row's right edge: %+v", theme, r.ID, r)
					}
					if g.DerivedNote != "none" {
						t.Errorf("%s mobile: the head's long note must drop to the bare word \"derived\"", theme)
					}
				}
			}
		}
	}
	if got := p.claimBytes(); string(got) != string(before) {
		t.Fatalf("rendering the derived brief rows changed the claim file:\nbefore:\n%s\nafter:\n%s", before, got)
	}
}
