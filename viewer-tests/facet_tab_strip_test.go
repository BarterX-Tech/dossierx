package viewertests

// THE FACET TAB STRIP, WITH THREE FACETS AND A COUNT ON EACH.
//
// docs/design/screens/02-reading-view-default.md § 4.9 measures the strip in
// two states that only differ from each other when more than one facet exists,
// and gives each tab a per-facet claim count (nodes 6F-0 selected / 6I-0 rest)
// that only becomes distinguishable from the MODULE's total when the facets
// hold different numbers of claims:
//
//	Selected  label  Inter 14/18 weight 600, --color-accent (the engine's --link)
//	Selected  count  mono 11/14, same colour as its label
//	Rest      label  Inter 14/18 weight 400, --color-muted
//	Rest      count  mono 11/14, --color-faint
//
// Until this fixture landed, nothing in the repository rendered that strip with
// three tabs. Every other fixture here carries one or two facets, and
// docs/design/LANES.md's claim that the client corpus supplies a three-facet
// module was wrong — its "3 facets" is the PROJECT's facet vocabulary, and no
// module in it carries more than two. A two-tab strip
// cannot catch a rule that is written against `:first-child`/`:last-child`, a
// count that reports the module total instead of the facet's own, or a
// selected-state rule that happens to match every tab: all three read as
// correct at two tabs and wrong at three.
//
// WHAT THIS COVERS, stated because a rendered-state claim is a coverage claim:
// viewer-tests/testdata/three-facets rendered fresh by the engine under test,
// at 1280px, in light and dark, in the two selection states (the default facet
// selected, and a different facet selected after a real click). It does not
// cover the mobile form of the strip (02 § 5), the sidebar module row that
// shares .sec-tab__count, or the build-order strip (.bo-modules), which is a
// different component that reuses the .subtab class.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chromedp/chromedp"
)

// threeFacetFacets is the fixture's facet list, in project.config.yaml order,
// with the claim count each facet actually holds. The counts are deliberately
// NOT all equal: 2/1/1 is what makes "the tab shows its own facet's count"
// distinguishable from "the tab shows the module's 4".
var threeFacetFacets = []struct {
	GroupID string
	Label   string
	Count   int
}{
	{"widget-contract", "Contract", 2},
	{"widget-interface", "Interface", 1},
	{"widget-internals", "Internals", 1},
}

// renderThreeFacetFixture copies viewer-tests/testdata/three-facets into a temp
// dir, renders it with the binary under test and returns a file:// URL. It is a
// local twin of theme_parity_test.go's renderFixtureFresh, which resolves its
// fixtures under the repo's own testdata/ tree; this fixture lives beside the
// suite that is its only consumer.
func renderThreeFacetFixture(t *testing.T) string {
	t.Helper()
	bin := requireBin(t)
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	src := filepath.Join(root, "viewer-tests", "testdata", "three-facets")
	if _, err := os.Stat(filepath.Join(src, "project.config.yaml")); err != nil {
		t.Fatalf("the three-facet fixture is missing: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "three-facets")
	if out, err := exec.Command("cp", "-R", src, dst).CombinedOutput(); err != nil {
		t.Fatalf("copy the three-facet fixture: %v\n%s", err, out)
	}
	cfg := filepath.Join(dst, "project.config.yaml")
	if out, err := exec.Command(bin, "--config", cfg, "--format", "text", "check").CombinedOutput(); err != nil {
		t.Fatalf("check the three-facet fixture: %v\n%s", err, out)
	}
	index := filepath.Join(dst, "build", "viewer", "index.html")
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("check wrote no viewer for the three-facet fixture: %v", err)
	}
	return "file://" + index
}

// subtabState is one tab, read the way a reader sees it: the painted colours,
// not the class names that produce them.
type subtabState struct {
	Target      string `json:"target"`
	Label       string `json:"label"`
	Count       string `json:"count"`
	On          bool   `json:"on"`
	Color       string `json:"color"`
	Weight      string `json:"weight"`
	BorderColor string `json:"borderColor"`
	BorderWidth string `json:"borderWidth"`
	HasCount    bool   `json:"hasCount"`
	CountColor  string `json:"countColor"`
	CountFamily string `json:"countFamily"`
}

// stripReading is the whole strip plus the three token values its rules are
// written against, all resolved in the SAME tab and colour scheme, so light and
// dark are compared against their own palettes rather than against literals.
type stripReading struct {
	Found     bool           `json:"found"`
	NSubNavs  int            `json:"nSubNavs"`
	Tabs      []subtabState  `json:"tabs"`
	Link      string         `json:"link"`
	Muted     string         `json:"muted"`
	Faint     string         `json:"faint"`
	GroupSize map[string]int `json:"groupSize"`
}

// readStrip measures the module's facet strip. Token values are resolved
// through a probe element rather than read as raw custom-property text, so
// `--link` and a tab's `color` are compared in the same resolved rgb() form.
const readStripJS = `(function () {
	var nav = document.querySelectorAll('.module-section > .sub-nav, .module > .sub-nav, .sub-nav');
	var strip = null;
	for (var i = 0; i < nav.length; i++) {
		if (!nav[i].closest('.bo-modules') && nav[i].querySelector('.subtab')) { strip = nav[i]; break; }
	}
	if (!strip) { return { found: false, nSubNavs: nav.length, tabs: [], link: '', muted: '', faint: '', groupSize: {} }; }

	function resolve(expr) {
		var probe = document.createElement('span');
		probe.style.color = expr;
		strip.appendChild(probe);
		var v = getComputedStyle(probe).color;
		strip.removeChild(probe);
		return v;
	}

	var tabs = [];
	strip.querySelectorAll('.subtab').forEach(function (b) {
		var cs = getComputedStyle(b);
		var label = b.querySelector('.sec-tab__label');
		var count = b.querySelector('.sec-tab__count');
		tabs.push({
			target: b.getAttribute('data-target') || '',
			label: label ? label.textContent.trim() : b.textContent.trim(),
			count: count ? count.textContent.trim() : '',
			on: b.classList.contains('on'),
			color: cs.color,
			weight: cs.fontWeight,
			borderColor: cs.borderBottomColor,
			borderWidth: cs.borderBottomWidth,
			hasCount: !!count,
			countColor: count ? getComputedStyle(count).color : '',
			countFamily: count ? getComputedStyle(count).fontFamily : ''
		});
	});

	var groupSize = {};
	document.querySelectorAll('section.claim-group').forEach(function (g) {
		groupSize[g.id] = g.querySelectorAll('section.claim').length;
	});

	return {
		found: true,
		nSubNavs: nav.length,
		tabs: tabs,
		link: resolve('var(--link)'),
		muted: resolve('var(--muted)'),
		faint: resolve('var(--faint)'),
		groupSize: groupSize
	};
})()`

func TestThreeFacetTabStripPaintsTheSelectedFacetAndACountPerFacet(t *testing.T) {
	browser := resolveBrowser(t)
	url := renderThreeFacetFixture(t)

	for _, scheme := range []string{"light", "dark"} {
		scheme := scheme
		t.Run(scheme, func(t *testing.T) {
			ctx := browserContextFor(t, browser)
			runCDP(t, ctx,
				chromedp.EmulateViewport(1280, 900, chromedp.EmulateScale(1)),
				chromedp.Navigate(url),
			)
			pollTrue(t, ctx, `document.readyState === 'complete'`)
			waitVisible(t, ctx, ".sub-nav .subtab")
			emulateColorScheme(t, ctx, scheme)
			suppressTransitions(t, ctx, "")
			// The `on` class is written by viewer-runtime.js once it has picked
			// the default facet; reading before that would measure three
			// unselected tabs and pass the "exactly one selected" guard below
			// for the wrong reason.
			pollTrue(t, ctx, `document.querySelectorAll('.sub-nav .subtab.on').length === 1`)

			var r stripReading
			evalInto(t, ctx, readStripJS, &r)
			assertThreeFacetStrip(t, r, scheme, "as rendered", 0)

			// Selecting a DIFFERENT facet is what proves the selected-state
			// rules are about selection and not about position: at this point
			// the first tab must have gone back to the rest treatment and the
			// second must have taken the selected one.
			runCDP(t, ctx, chromedp.Click(
				`.sub-nav .subtab[data-target="#`+threeFacetFacets[1].GroupID+`"]`, chromedp.ByQuery))
			pollTrue(t, ctx, fmt.Sprintf(
				`(document.querySelector('.sub-nav .subtab[data-target="#%s"]') || {classList: {contains: function(){return false;}}}).classList.contains('on')`,
				threeFacetFacets[1].GroupID))

			var after stripReading
			evalInto(t, ctx, readStripJS, &after)
			assertThreeFacetStrip(t, after, scheme, "after clicking the second facet", 1)
		})
	}
}

// assertThreeFacetStrip checks one reading of the strip. wantSelected is the
// index into threeFacetFacets that must carry the selected treatment.
func assertThreeFacetStrip(t *testing.T, r stripReading, scheme, when string, wantSelected int) {
	t.Helper()

	// ---- VACUITY GUARDS ----
	if !r.Found {
		t.Fatalf("%s, %s: no facet .sub-nav is in the document (%d .sub-nav element(s) found). "+
			"A module with three facets must render one; with no strip every assertion "+
			"below would be skipped and this test would pass over nothing.", scheme, when, r.NSubNavs)
	}
	if len(r.Tabs) != 3 {
		t.Fatalf("%s, %s: the facet strip carries %d .subtab(s), want 3 — the whole point of "+
			"this fixture is the three-tab state, and at two tabs a first-child/last-child "+
			"rule and a per-facet count are both indistinguishable from correct.\n  tabs: %+v",
			scheme, when, len(r.Tabs), r.Tabs)
	}
	// The three token values must differ from each other, or "the selected tab
	// is --link and the rest are --muted" is not a statement about anything.
	if r.Link == r.Muted || r.Link == r.Faint {
		t.Fatalf("%s, %s: --link resolves to %s, --muted to %s and --faint to %s. With two of "+
			"them equal the colour assertions below cannot tell the selected treatment from "+
			"the rest treatment.", scheme, when, r.Link, r.Muted, r.Faint)
	}

	var selected []string
	for _, tab := range r.Tabs {
		if tab.On {
			selected = append(selected, tab.Label)
		}
	}
	if len(selected) != 1 {
		t.Fatalf("%s, %s: %d tab(s) carry the selected class (%v), want exactly 1",
			scheme, when, len(selected), selected)
	}

	for i, want := range threeFacetFacets {
		tab := r.Tabs[i]
		isSelected := i == wantSelected

		if tab.Label != want.Label {
			t.Errorf("%s, %s: tab %d reads %q, want %q — the strip must list every facet in "+
				"project.config.yaml order", scheme, when, i, tab.Label, want.Label)
		}
		if tab.Target != "#"+want.GroupID {
			t.Errorf("%s, %s: tab %q targets %q, want %q", scheme, when, tab.Label, tab.Target, "#"+want.GroupID)
		}
		if tab.On != isSelected {
			t.Errorf("%s, %s: tab %q is %sselected and must be the other way round",
				scheme, when, tab.Label, map[bool]string{true: "", false: "un"}[tab.On])
			continue
		}

		// ---- the count (02 § 4.9, nodes 6F-0 / 6I-0) ----
		if !tab.HasCount {
			t.Errorf("%s, %s: tab %q carries no .sec-tab__count. 02 § 4.9 gives every facet tab "+
				"a count beside its label; without one a reader cannot see how much is behind "+
				"the tab they are not looking at.", scheme, when, tab.Label)
		} else {
			if tab.Count != fmt.Sprint(want.Count) {
				t.Errorf("%s, %s: tab %q shows count %q, want %q — this facet's OWN claims. "+
					"The fixture's module holds 4 claims across 2/1/1; a tab that reports 4 "+
					"is showing the module total, which is the defect this fixture exists to "+
					"make visible.", scheme, when, tab.Label, tab.Count, fmt.Sprint(want.Count))
			}
			if got, ok := r.GroupSize[want.GroupID]; ok && got != want.Count {
				t.Errorf("%s, %s: facet %q renders %d claim card(s) but this test expects %d; "+
					"the fixture changed and the counts above are no longer the facts",
					scheme, when, want.Label, got, want.Count)
			}
			wantCountColor := r.Faint
			if isSelected {
				wantCountColor = r.Link
			}
			if tab.CountColor != wantCountColor {
				t.Errorf("%s, %s: the %s tab %q paints its count %s, want %s (%s)",
					scheme, when, selectedWord(isSelected), tab.Label, tab.CountColor,
					wantCountColor, tokenName(isSelected, "--link", "--faint"))
			}
		}

		// ---- the label and the underline (02 § 4.9, nodes 6D-0 / 6E-0 / 6H-0) ----
		wantColor, wantWeight, wantToken := r.Muted, "400", "--muted"
		if isSelected {
			wantColor, wantWeight, wantToken = r.Link, "600", "--link"
		}
		if tab.Color != wantColor {
			t.Errorf("%s, %s: the %s tab %q paints its label %s, want %s (%s). "+
				"02 § 4.9 gives the selected facet the navigation colour and the rest --muted; "+
				"--accent is the LOCKED green and must never stand in for it (tokens.md G2).",
				scheme, when, selectedWord(isSelected), tab.Label, tab.Color, wantColor, wantToken)
		}
		if tab.Weight != wantWeight {
			t.Errorf("%s, %s: the %s tab %q is font-weight %s, want %s",
				scheme, when, selectedWord(isSelected), tab.Label, tab.Weight, wantWeight)
		}
		if isSelected {
			if tab.BorderColor != r.Link {
				t.Errorf("%s, %s: the selected tab %q underlines in %s, want %s (--link)",
					scheme, when, tab.Label, tab.BorderColor, r.Link)
			}
			if tab.BorderWidth != "2px" {
				t.Errorf("%s, %s: the selected tab %q underlines at %s, want 2px (02 § 4.9)",
					scheme, when, tab.Label, tab.BorderWidth)
			}
		} else if tab.BorderWidth != "0px" && tab.BorderColor == r.Link {
			t.Errorf("%s, %s: the unselected tab %q carries a %s --link underline; only the "+
				"selected facet is underlined", scheme, when, tab.Label, tab.BorderWidth)
		}
	}

	t.Logf("%s, %s: %s | --link %s, --muted %s, --faint %s",
		scheme, when, describeTabs(r.Tabs), r.Link, r.Muted, r.Faint)
}

func selectedWord(selected bool) string {
	if selected {
		return "selected"
	}
	return "rest"
}

func tokenName(selected bool, whenSelected, whenRest string) string {
	if selected {
		return whenSelected
	}
	return whenRest
}

func describeTabs(tabs []subtabState) string {
	out := ""
	for i, tab := range tabs {
		if i > 0 {
			out += "  "
		}
		mark := " "
		if tab.On {
			mark = "*"
		}
		out += fmt.Sprintf("%s%s(%s)", mark, tab.Label, tab.Count)
	}
	return out
}
