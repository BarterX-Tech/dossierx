package viewertests

// THE VIEWER LOOKS EXACTLY THE SAME AS IT DID BEFORE IT COULD BE THEMED.
//
// The custom-theme work turned ~14 hard-coded literals in style.css into
// var(--token, <the same literal>) reads. Every one of those edits is supposed
// to be a no-op for a project that sets no token: the fallback IS the literal
// that used to be there. "Supposed to" is the operative phrase — a fallback
// with a typo, a var() placed on a declaration something later overrides, a
// token read inside a shorthand that resets a sibling property, all render
// as a page that is subtly not the page that shipped, and no unit test over
// the CSS source can see it. Only a browser resolving the cascade can.
//
// So this file opens TWO documents in the same browser process: the frozen
// pre-change render (viewer-tests/testdata/theme-parity/baseline-*.html,
// produced by the binary built from 01b70d0) and a render made by the engine
// under test, of the same fixture. It then reads COMPUTED STYLE — never a
// CSSRule's own .style, which reports what an author wrote rather than what a
// reader gets, and would therefore report "identical" for exactly the class of
// bug this exists to catch — off the same 28-row probe table in both, and
// diffs the two maps in Go.
//
// WHAT IS EXPECTED TO DIFFER is asserted positively rather than tolerated:
// `color-scheme` on the root moves from `light` to `light dark`, and under
// print in a dark OS scheme the whole palette changes (the pre-change sheet
// let its dark block apply to print; the new one scopes it to `screen and`).
// A change that is expected has to be SEEN to be a change, or "expected" is
// just another word for unchecked.
//
// THE BOUNDS, stated because a probe table is a coverage claim:
//   - Two fixtures (fixture-theme-flat, fixture-basic) x two colour schemes x
//     two viewport widths (1280, 375). fixture-theme-flat exists for this test
//     and its README lists every construct it does and does not instantiate.
//   - A selector the fixture has no element for is probed against a SYNTHETIC
//     subtree appended identically to both documents. That measures the
//     stylesheet rather than the render, which is the honest thing to say
//     about it; the test records per selector whether the match was real or
//     synthetic and FAILS if the two documents disagree.
//   - The screenshot passes are clipped to the content area, because the
//     sidebar footer carries a generation timestamp that legitimately differs
//     between two renders taken minutes apart. An innerText equality assertion
//     runs over the clipped element first, so a pixel mismatch cannot be
//     shrugged off as "the text moved".

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/css"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// The probe table (plan §6.2, as amended by plan-v4 A3)
// ---------------------------------------------------------------------

// parityProbe is one row: a set of selectors that must each match at least one
// element, and the computed properties read off the FIRST match of each.
type parityProbe struct {
	name      string
	selectors []string
	props     []string
}

// themeTokens is the engine's 28-token allowlist, in allowlist order. It is
// duplicated here rather than imported because viewer-tests is a separate Go
// module and cannot reach internal/config. internal/render's own tests pin the
// list at 28; the guard below pins this copy's length to the same number, so a
// token added upstream without touching this file fails here rather than
// silently going unprobed.
var themeTokens = []string{
	"accent", "accent-bg", "ink", "muted", "faint", "paper", "card-bg",
	"border", "link", "warn", "warn-bg", "font-sans", "font-mono", "radius",
	"code-inline-bg", "code-bg", "table-head-bg", "image-bg", "hover-bg",
	"border-strong", "shadow", "shadow-strong", "shadow-cast", "scrim",
	"selection-bg", "status-draft", "status-draft-bg", "mockup-bg",
}

// rootProps is probe row 1's property list: every token, read off the root
// element, plus one graph token and the colour-scheme declaration itself.
func rootProps() []string {
	out := make([]string, 0, len(themeTokens)+2)
	for _, t := range themeTokens {
		out = append(out, "--"+t)
	}
	return append(out, "--dxg-facet-other", "color-scheme")
}

// parityProbes is the normative table. len(parityProbes) is pinned below: a row
// deleted to make a run green is the failure mode this table has.
//
// Rows 22 and 23 are the mobile chrome. They are probed at BOTH widths rather
// than only at 375px: the elements are in the markup at every width (the
// narrow media query changes their painting, not their existence), and a
// computed style read at 1280 is a real reading of a real rule, so probing
// both is strictly more coverage than the plan's "375px" note asked for.
var parityProbes = []parityProbe{
	{"01 root tokens", []string{"html"}, rootProps()},
	{"02 body", []string{"body.shell-body"}, []string{"background-color", "color", "font-family", "font-size", "line-height"}},
	{"03 inline code", []string{"code"}, []string{"background-color", "color", "font-family", "border-radius"}},
	{"04 fenced pre", []string{".claim-body pre"}, []string{"background-color", "border-color", "border-radius"}},
	{"05 pre code", []string{".claim-body pre code"}, []string{"background-color", "padding"}},
	{"06 comment table head", []string{".comment-body .md-table th"}, []string{"background-color", "color"}},
	{"07 step image", []string{".sbody .md-img"}, []string{"background-color", "border-color", "border-radius"}},
	{"08 status-draft", []string{".status-draft"}, []string{"background-color", "color"}},
	{"09 draft pill", []string{".pill.pv"}, []string{"background-color", "color"}},
	{"10 enum marker", []string{".en"}, []string{"color"}},
	{"11 locked and warn pills", []string{".pill.ps", ".pill.pw"}, []string{"background-color", "color"}},
	{"12 review pending", []string{".claim-review-pending"}, []string{"background-color", "color", "padding", "border-radius"}},
	{"13 logo", []string{".logo"}, []string{"border-color", "color"}},
	{"14 mockup diagram", []string{".mockup-diagram"}, []string{"background-color"}},
	{"15 console mockup", []string{".gcp-console"}, []string{"background-color", "color", "border-color", "box-shadow"}},
	{"16 record heads", []string{".system-record-head", ".track-head", ".build-order-module > .system-build-title"}, []string{"border-bottom-color"}},
	{"17 comments panel", []string{".comments-panel"}, []string{"box-shadow", "background-color", "border-radius"}},
	{"18 comments rail", []string{".comments-rail"}, []string{"box-shadow", "background-color", "border-left-color"}},
	{"19 comments toast", []string{".comments-toast"}, []string{"box-shadow", "background-color", "color", "border-color"}},
	{"20 comments overlay", []string{".comments-overlay"}, []string{"background-color"}},
	{"21 nav overlay", []string{".nav-overlay"}, []string{"background-color"}},
	{"22 cast shadows", []string{".nav-toggle", ".facet-toc"}, []string{"box-shadow"}},
	{"23 facet select", []string{".facet-toc__select"}, []string{"background-color", "color"}},
	{"24 surfaces", []string{
		".sidebar", ".card", ".claim-banner", ".sec-tab.on", ".facet-toc__item.on",
		".system-nav-group__count", ".comment-chip", ".comment-composer-input",
		".comment-composer-submit", ".snum", ".status-strip",
	}, []string{"background-color", "color", "border-color"}},
	// plan-v4 A3: outline-color dropped; the rule at style.css:1079 sets
	// background and border-radius and nothing else.
	{"25 targeted source row", []string{".claim-source:target"}, []string{"background-color", "border-radius"}},
	{"26 hover", []string{
		".system-nav-group__toggle", ".sec-tab", ".facet-toc__item",
		".claim-collapse-toggle", ".claim-links-summary",
	}, []string{"background-color"}},
	{"27 selection", []string{"#dxparity-selection"}, []string{"background-color"}},
	{"28 graph palette", nil, nil}, // read through readPalette(), not the DOM
}

// hoverProbeIndex and selectionProbeIndex name the two rows that are not plain
// computed-style reads, so the reader of this file does not have to count.
const (
	hoverProbeIndex     = 25 // "26 hover" (0-based)
	selectionProbeIndex = 26 // "27 selection"
	graphProbeIndex     = 27 // "28 graph palette"
)

// ---------------------------------------------------------------------
// The in-page probe program
// ---------------------------------------------------------------------

// parityProbeJS installs window.__dxParity. It is injected into both documents
// verbatim, so a bug in the probe is a bug in both readings and cancels; a bug
// in the STYLESHEET does not.
const parityProbeJS = `
window.__dxParity = (function () {
  var CONTAINER_ID = 'dxparity-synthetic';

  // The synthetic subtree. Every selector in the probe table that a given
  // fixture may not instantiate has a match in here, nested deeply enough to
  // satisfy the descendant selectors the stylesheet actually writes
  // (".claim-body pre", ".sbody .md-img", ".comment-body .md-table th", ...).
  // It is appended LAST, so document.querySelector always prefers a real
  // element when the page has one.
  var SYNTHETIC = [
    '<div class="claim-body"><pre><code>x</code></pre>',
      '<table class="md-table"><thead><tr><th>h</th></tr></thead><tbody><tr><td>c</td></tr></tbody></table>',
      '<img class="md-img" alt="">',
    '</div>',
    '<div class="claim-list-items"><table class="md-table"><thead><tr><th>h</th></tr></thead></table><img class="md-img" alt=""></div>',
    '<div class="comment-body"><table class="md-table"><thead><tr><th>h</th></tr></thead></table></div>',
    '<div class="sbody"><table class="md-table"><thead><tr><th>h</th></tr></thead></table><img class="md-img" alt=""></div>',
    '<div class="claim-tree-body">t</div>',
    '<span class="pill pv">draft</span><span class="pill ps">locked</span><span class="pill pw">warn</span>',
    '<span class="status-draft">draft</span>',
    '<span class="key">k</span><span class="ty">t</span><span class="en">e</span><span class="ex">x</span>',
    '<ul class="claim-edges"><li class="claim-review-pending">review_pending</li></ul>',
    '<div class="mockup-diagram"><div class="gcp-console"><div class="gcp-row">r</div></div></div>',
    '<div class="system-record-head">h</div><div class="track-head">h</div>',
    '<div class="build-order-module"><div class="system-build-title">t</div></div>',
    '<ul class="claim-source-list"><li class="claim-source" id="dxparity-source">s</li></ul>',
    '<div class="logo">l</div><div class="card">c</div><div class="claim-banner">b</div>',
    '<button type="button" class="sec-tab on">t</button>',
    '<button type="button" class="sec-tab">t</button>',
    '<button type="button" class="facet-toc__item on">i</button>',
    '<button type="button" class="facet-toc__item">i</button>',
    '<span class="system-nav-group__count">3</span>',
    '<button type="button" class="comment-chip">c</button>',
    '<textarea class="comment-composer-input"></textarea>',
    '<button type="button" class="comment-composer-submit">s</button>',
    '<div class="snum">1</div><div class="status-strip">s</div>',
    '<button type="button" class="system-nav-group__toggle">t</button>',
    '<button type="button" class="claim-collapse-toggle">t</button>',
    '<details><summary class="claim-links-summary">t</summary></details>',
    '<div class="comments-panel">p</div><div class="comments-rail">r</div>',
    '<div class="comments-toast">t</div><div class="comments-overlay">o</div>',
    '<div class="nav-overlay">o</div><button type="button" class="nav-toggle">n</button>',
    '<div class="facet-toc"><select class="facet-toc__select"><option>a</option></select></div>',
    // Probe 27's first leg: an element whose background IS the token read,
    // fallback included, so the ::selection colour is measurable without a
    // live selection.
    '<div id="dxparity-selection" style="background: var(--selection-bg, rgba(40, 112, 82, .20))">sel</div>'
  ].join('');

  function install() {
    if (document.getElementById(CONTAINER_ID)) { return; }
    var c = document.createElement('div');
    c.id = CONTAINER_ID;
    c.setAttribute('aria-hidden', 'true');
    // Off-screen but LAID OUT: display:none would make every layout-derived
    // computed value report the specified value instead of the used one.
    c.style.cssText = 'position:absolute;left:-99999px;top:0;width:900px';
    c.innerHTML = SYNTHETIC;
    document.body.appendChild(c);
  }

  function remove() {
    var c = document.getElementById(CONTAINER_ID);
    if (c) { c.parentNode.removeChild(c); }
  }

  function inSynthetic(el) {
    var c = document.getElementById(CONTAINER_ID);
    return !!(c && c.contains(el));
  }

  // read(spec) -> { values: {"row|selector|prop": "..."}, origins: {"row|selector": "real|synthetic|none"} }
  function read(spec) {
    var values = {}, origins = {};
    for (var i = 0; i < spec.length; i++) {
      var row = spec[i];
      for (var j = 0; j < row.selectors.length; j++) {
        var sel = row.selectors[j];
        var el = null;
        try { el = document.querySelector(sel); } catch (e) { el = null; }
        var key = row.name + '|' + sel;
        if (!el) { origins[key] = 'none'; continue; }
        origins[key] = inSynthetic(el) ? 'synthetic' : 'real';
        var cs = getComputedStyle(el);
        for (var k = 0; k < row.props.length; k++) {
          var p = row.props[k];
          var v = p.indexOf('--') === 0 ? cs.getPropertyValue(p) : cs[p];
          values[key + '|' + p] = String(v === undefined || v === null ? '' : v).trim();
        }
      }
    }
    return { values: values, origins: origins };
  }

  // The ::selection pseudo-element read. The spike (plan §6.2 step 0) measured
  // this in the browser this suite drives and it DOES return the selection
  // colour, with and without a live selection, so no pixel sampling is needed.
  // Its bound: it reports the pseudo-element's resolved background, which is
  // what ::selection { background: var(--selection-bg, ...) } sets and nothing
  // else on the page sets, so a wrong fallback shows up here.
  function selectionPseudo() {
    var cs = getComputedStyle(document.body, '::selection');
    return String(cs.backgroundColor || '');
  }

  // The graph pane's own palette read, reproduced token for token from
  // readPalette() in graph-ui.js (which is inside a closure and not reachable
  // from here), including its '#808080' empty-value fallback.
  function palette() {
    var cs = getComputedStyle(document.documentElement);
    function v(n) {
      var s = cs.getPropertyValue(n);
      s = s ? s.trim() : '';
      return s === '' ? '#808080' : s;
    }
    var out = {};
    for (var i = 1; i <= 20; i++) { out['facet-' + i] = v('--dxg-facet-' + i); }
    var rest = ['--dxg-facet-other', '--dxg-cycle', '--dxg-halo', '--dxg-governed',
                '--ink', '--muted', '--faint', '--paper', '--accent', '--link', '--warn'];
    for (var j = 0; j < rest.length; j++) { out[rest[j]] = v(rest[j]); }
    return out;
  }

  // The print pass's expected-IDENTICAL layout probes (plan-v4 A2). The
  // consolidated print block moved to the END of style.css, which is what keeps
  // it winning against Layer B's screen rules at equal specificity. If it had
  // been placed anywhere else, these three change and a layout regression would
  // be hiding inside a sanctioned palette diff.
  function printLayout() {
    function one(sel, props) {
      var el = document.querySelector(sel), o = {};
      if (!el) { return null; }
      var cs = getComputedStyle(el);
      for (var i = 0; i < props.length; i++) { o[props[i]] = String(cs[props[i]]); }
      return o;
    }
    return {
      'content-area': one('.content-area', ['paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft']),
      'system-record-head': one('.system-record-head', ['paddingTop']),
      'sidebar': one('.sidebar', ['display'])
    };
  }

  // The .on class on the first sidebar tab outranks .sec-tab:hover — same
  // specificity, later in the sheet — so a hover probe on it measures the
  // active-tab rule and reports "no change" for a hover rule that is in fact
  // broken. Both documents get the same strip, so this changes what is
  // compared, not whether the comparison is fair. Recorded in the result.
  function stripActive() {
    var n = 0;
    document.querySelectorAll('.sec-tab.on, .facet-toc__item.on').forEach(function (e) {
      if (e.closest('#' + CONTAINER_ID)) { return; }
      e.classList.remove('on'); n++;
    });
    return n;
  }

  function targetSource() {
    var real = null;
    var all = document.querySelectorAll('.claim-source[id]');
    for (var i = 0; i < all.length; i++) {
      if (!inSynthetic(all[i])) { real = all[i]; break; }
    }
    var id = real ? real.id : 'dxparity-source';
    location.hash = '#' + id;
    return id;
  }

  return {
    install: install, remove: remove, read: read, palette: palette,
    selectionPseudo: selectionPseudo, printLayout: printLayout,
    stripActive: stripActive, targetSource: targetSource
  };
})();
`

// ---------------------------------------------------------------------
// Go-side plumbing
// ---------------------------------------------------------------------

type probeResult struct {
	Values  map[string]string `json:"values"`
	Origins map[string]string `json:"origins"`
}

// probeSpec is the JSON the probe program consumes.
func probeSpec() string {
	type row struct {
		Name      string   `json:"name"`
		Selectors []string `json:"selectors"`
		Props     []string `json:"props"`
	}
	var rows []row
	for i, p := range parityProbes {
		if i == graphProbeIndex {
			continue // read through palette(), not the DOM
		}
		rows = append(rows, row{p.name, p.selectors, p.props})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		panic(err) // a compile-time-constant table cannot fail to marshal
	}
	return string(b)
}

// renderFixtureFresh copies testdata/<fixture> to a temp dir, drops the
// committed viewer/ and .catalog.json, and runs the engine under test over the
// copy. The file:// URL it returns is the page a reader would open today.
func renderFixtureFresh(t *testing.T, fixture string) string {
	t.Helper()
	bin := requireBin(t)
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	src := filepath.Join(root, "testdata", fixture)
	dst := filepath.Join(t.TempDir(), fixture)
	cp := exec.Command("cp", "-R", src, dst)
	if out, err := cp.CombinedOutput(); err != nil {
		t.Fatalf("copy fixture %s: %v\n%s", fixture, err, out)
	}
	if err := os.RemoveAll(filepath.Join(dst, "viewer")); err != nil {
		t.Fatalf("drop committed viewer: %v", err)
	}
	if err := os.Remove(filepath.Join(dst, ".catalog.json")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("drop committed catalog: %v", err)
	}
	cfg := filepath.Join(dst, "project.config.yaml")
	cmd := exec.Command(bin, "--config", cfg, "--format", "text", "check")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("check %s: %v\n%s", fixture, err, out)
	}
	out := filepath.Join(dst, "viewer", "index.html")
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("check wrote no viewer for %s: %v", fixture, err)
	}
	return "file://" + out
}

// unscopedDarkMediaQuery matches a dark colour-scheme @media block that is NOT
// prefixed with `screen and`. Exactly one exists in each frozen baseline and
// none in anything the current engine renders, which is what makes it usable as
// proof of which engine produced a file.
var unscopedDarkMediaQuery = regexp.MustCompile(`@media\s*\(prefers-color-scheme:\s*dark\)`)

// baselinePath returns the frozen pre-change render for a fixture and asserts
// the two guards that make it evidence: it must carry the OLD unconditional
// colour-scheme pin, and it must not carry any of the fourteen new tokens.
// Without these, a baseline accidentally regenerated with the current engine
// would make every comparison below a tautology that passes.
func baselinePath(t *testing.T, fixture string) string {
	t.Helper()
	name := map[string]string{
		"fixture-theme-flat": "baseline-flat.html",
		"fixture-basic":      "baseline-basic.html",
	}[fixture]
	if name == "" {
		t.Fatalf("no frozen baseline is committed for fixture %q", fixture)
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	p := filepath.Join(root, "viewer-tests", "testdata", "theme-parity", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read frozen baseline %s: %v", p, err)
	}
	body := string(b)
	// GUARD 1: an UNSCOPED dark colour-scheme media query.
	//
	// This used to look for "color-scheme: light;" and was inert: the current
	// engine's consolidated print block contains that string too, so a baseline
	// accidentally regenerated with the engine under test passed it. What only
	// the old sheet has is a dark block with no `screen and` prefix — scoping it
	// to screen is the change, and it is why a dark-mode page no longer prints
	// dark. Counted here as one occurrence in the baseline against zero in the
	// current sheet (the current viewer's remaining unscoped mention is a
	// matchMedia STRING in the runtime script, which is why this matches on the
	// `@media` keyword rather than on the feature alone).
	if n := len(unscopedDarkMediaQuery.FindAllString(body, -1)); n == 0 {
		t.Fatalf("%s contains no unscoped dark colour-scheme @media block. Only the PRE-CHANGE "+
			"sheet has one — the engine under test scopes every dark block to `screen and` — so "+
			"this file is not the pre-change render, and every comparison against it would be a "+
			"comparison of the current engine with itself", p)
	}
	if strings.Contains(body, "--code-bg") {
		t.Fatalf("%s contains \"--code-bg\", one of the fourteen tokens the theme work ADDED — "+
			"it is not the pre-change render", p)
	}
	return "file://" + p
}

// parityTab opens one tab, navigates it, pins its viewport metrics and device
// pixel ratio, installs the probe program and the synthetic subtree, and
// returns the context.
//
// The DPR pin is not decoration: chromedp.EmulateViewport defaults the scale
// factor from the host, and two tabs on a machine whose display changed
// between them would report different used values for anything the layout
// rounds — a difference in the harness reported as a difference in the
// stylesheet.
func parityTab(t *testing.T, browser, url string, w, h int64) context.Context {
	t.Helper()
	ctx := browserContextFor(t, browser)
	runCDP(t, ctx,
		chromedp.EmulateViewport(w, h, chromedp.EmulateScale(1)),
		chromedp.Navigate(url),
	)
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	evalVoid(t, ctx, parityProbeJS)
	evalVoid(t, ctx, `window.__dxParity.install()`)
	return ctx
}

func resizeParityTab(t *testing.T, ctx context.Context, w, h int64) {
	t.Helper()
	runCDP(t, ctx, chromedp.EmulateViewport(w, h, chromedp.EmulateScale(1)))
	pollTrue(t, ctx, fmt.Sprintf(`window.innerWidth === %d`, w))
}

func readProbes(t *testing.T, ctx context.Context) probeResult {
	t.Helper()
	var r probeResult
	evalInto(t, ctx, `window.__dxParity.read(`+probeSpec()+`)`, &r)
	if len(r.Values) == 0 {
		t.Fatal("the probe program returned no readings at all; every comparison below would be vacuous")
	}
	return r
}

// diffMaps returns the sorted keys whose values differ, plus the keys present
// in only one side (which is itself a difference).
func diffMaps(a, b map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	for k := range seen {
		if a[k] != b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func describeDiff(a, b map[string]string, keys []string, beforeLabel, afterLabel string) string {
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "\n  %s\n    %s: %q\n    %s: %q", k, beforeLabel, a[k], afterLabel, b[k])
	}
	return sb.String()
}

// ---------------------------------------------------------------------
// The test
// ---------------------------------------------------------------------

// parityFixtures are the two corpora compared. fixture-theme-flat carries a
// flat viewer.theme block copied from a real client project and instantiates
// every themed construct; fixture-basic carries no theme at all, which is the
// case the whole no-op claim is about.
//
// DECLARED DEVIATION FROM THE PLAN, so the gap is on the page rather than in
// someone's memory: testdata/fixture-theme-preset appears in NO browser test.
// plan §6.2 put it in the print pass so a project with a dark block would be
// exercised there. It is not needed for that any more — plan-v4 A1 made a dark
// block unable to reach print at all, and the case is covered directly by
// TestDarkOnlyTokenDoesNotApplyToPrint in theme_modes_test.go, which builds a
// dark-only project with newProjectRaw and asserts --ink computes to the
// engine's light value under print. What fixture-theme-preset still covers is
// the COMMITTED artifact: it is the only viewer in the repository carrying a
// preset, a two-mode override and a data:-inlined @font-face, and
// tests/fixture_staleness_test.go re-renders it on every run.
var parityFixtures = []string{"fixture-theme-flat", "fixture-basic"}

var parityWidths = []struct {
	name string
	w, h int64
}{
	{"1280", 1280, 900},
	{"375", 375, 812},
}

func TestThemeParityAgainstThePreChangeRender(t *testing.T) {
	// Coverage bound, asserted rather than assumed: a row removed to make a
	// run green is the failure mode a probe table has.
	if got, want := len(parityProbes), 28; got != want {
		t.Fatalf("the parity probe table has %d rows, want %d — plan §6.2's table is normative", got, want)
	}
	if got, want := len(themeTokens), 28; got != want {
		t.Fatalf("themeTokens has %d entries, want 28 (the engine's allowlist length)", got)
	}

	browser := resolveBrowser(t)

	for _, fixture := range parityFixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			before := baselinePath(t, fixture)
			after := renderFixtureFresh(t, fixture)

			ctxBefore := parityTab(t, browser, before, 1280, 900)
			ctxAfter := parityTab(t, browser, after, 1280, 900)

			// The .on strip and the :target navigation are applied to BOTH
			// documents, and the counts are compared: an asymmetry there would
			// make every later reading a comparison of two different pages.
			strippedBefore := evalInt(t, ctxBefore, `window.__dxParity.stripActive()`)
			strippedAfter := evalInt(t, ctxAfter, `window.__dxParity.stripActive()`)
			if strippedBefore != strippedAfter {
				t.Fatalf("the two documents carry a different number of active sidebar tabs "+
					"(%d before, %d after): they are not the same page, so nothing below is a "+
					"comparison of stylesheets", strippedBefore, strippedAfter)
			}
			targetBefore := evalString(t, ctxBefore, `window.__dxParity.targetSource()`)
			targetAfter := evalString(t, ctxAfter, `window.__dxParity.targetSource()`)
			if targetBefore != targetAfter {
				t.Fatalf("the :target probe deep-linked to %q before and %q after", targetBefore, targetAfter)
			}
			waitScrollSettled(t, ctxBefore)
			waitScrollSettled(t, ctxAfter)
			t.Logf("%s: stripped %d active tab(s) in each document; :target = #%s",
				fixture, strippedBefore, targetBefore)

			for _, vp := range parityWidths {
				vp := vp
				t.Run(vp.name, func(t *testing.T) {
					resizeParityTab(t, ctxBefore, vp.w, vp.h)
					resizeParityTab(t, ctxAfter, vp.w, vp.h)

					for _, scheme := range []string{"light", "dark"} {
						scheme := scheme
						t.Run(scheme, func(t *testing.T) {
							emulateColorScheme(t, ctxBefore, scheme)
							emulateColorScheme(t, ctxAfter, scheme)
							runOneParityPass(t, ctxBefore, ctxAfter, fixture, scheme, vp.name)
						})
					}
				})
			}

			// The print and screenshot passes run on CLEAN LOADS, in their own
			// tabs. The tabs above have been resized four times, had a synthetic
			// subtree appended, a class stripped and a fragment navigated; a
			// print layout measured on top of all that is measuring the harness.
			// (It was: the first version of this file reused them and reported a
			// content-area padding difference that a clean load does not have.)
			t.Run("print-x-light-layout", func(t *testing.T) {
				pb := parityTab(t, browser, before, 1280, 900)
				pa := parityTab(t, browser, after, 1280, 900)
				runPrintLightPass(t, pb, pa, fixture)
			})
			t.Run("print-x-dark", func(t *testing.T) {
				pb := parityTab(t, browser, before, 1280, 900)
				pa := parityTab(t, browser, after, 1280, 900)
				runPrintDarkPass(t, pb, pa, fixture)
			})
			t.Run("screenshot", func(t *testing.T) {
				runScreenshotPass(t, browser, before, after, fixture)
			})
		})
	}
}

// runOneParityPass reads both documents and diffs them, holding the
// expected-different set to a positive assertion.
func runOneParityPass(t *testing.T, ctxBefore, ctxAfter context.Context, fixture, scheme, width string) {
	rb := readProbes(t, ctxBefore)
	ra := readProbes(t, ctxAfter)

	// Every selector must have matched something, in both documents, and must
	// have matched the same KIND of something. "none" on both sides would make
	// that row silently uncovered, which is the shape of a check that did not
	// run and reads as a pass.
	var unmatched, asymmetric []string
	for key, ob := range rb.Origins {
		oa := ra.Origins[key]
		if ob == "none" || oa == "none" {
			unmatched = append(unmatched, fmt.Sprintf("%s (before=%s, after=%s)", key, ob, oa))
			continue
		}
		if ob != oa {
			asymmetric = append(asymmetric, fmt.Sprintf("%s (before=%s, after=%s)", key, ob, oa))
		}
	}
	sort.Strings(unmatched)
	sort.Strings(asymmetric)
	if len(unmatched) > 0 {
		t.Errorf("%d probe selector(s) matched no element, so those rows measured nothing:\n  %s",
			len(unmatched), strings.Join(unmatched, "\n  "))
	}
	if len(asymmetric) > 0 {
		t.Errorf("%d probe selector(s) resolved to a real element in one document and a synthetic "+
			"one in the other, so the two readings are not comparable:\n  %s",
			len(asymmetric), strings.Join(asymmetric, "\n  "))
	}

	// The selection pseudo-element, probe 27's second leg.
	rb.Values["27 selection|::selection|background-color"] = evalString(t, ctxBefore, `window.__dxParity.selectionPseudo()`)
	ra.Values["27 selection|::selection|background-color"] = evalString(t, ctxAfter, `window.__dxParity.selectionPseudo()`)

	// The hover pass, probe 26. forcePseudoState is a CDP call, so it cannot
	// live in the page program.
	forceHoverInto(t, ctxBefore, rb.Values, "before")
	forceHoverInto(t, ctxAfter, ra.Values, "after")

	// The graph palette, probe 28.
	var palBefore, palAfter map[string]string
	evalInto(t, ctxBefore, `window.__dxParity.palette()`, &palBefore)
	evalInto(t, ctxAfter, `window.__dxParity.palette()`, &palAfter)
	if len(palBefore) != 31 || len(palAfter) != 31 {
		t.Fatalf("the graph palette read returned %d/%d entries, want 31 each "+
			"(20 facet slots + other + cycle + halo + governed + 7 shared tokens)",
			len(palBefore), len(palAfter))
	}
	for k, v := range palBefore {
		rb.Values["28 graph palette|"+k] = v
	}
	for k, v := range palAfter {
		ra.Values["28 graph palette|"+k] = v
	}

	// ---- the expected-different set, asserted positively ----
	//
	// The fourteen tokens the theme work ADDED are declared on :root by the new
	// stylesheet and by nothing in the old one, so getPropertyValue returns ""
	// before and the default value after. That is the change, not a symptom of
	// one, and it is held to a positive assertion in both directions: empty
	// before, non-empty after. The fourteen tokens that predate this work must
	// be byte-identical, and they are left in the map below to be diffed.
	//
	// (plan-v3 §6.2's row 1 asks for "getPropertyValue of all 28 tokens" and its
	// expected-different set names only color-scheme and the print palette. It
	// could not have been written any other way and still pass: a token that did
	// not exist reads as "". This is that gap closed, not coverage dropped.)
	for _, tok := range themeTokens[14:] {
		key := "01 root tokens|html|--" + tok
		vb, va := rb.Values[key], ra.Values[key]
		if vb != "" {
			t.Errorf("the pre-change render already declares --%s on :root (%q); it is supposed to "+
				"be one of the fourteen tokens this work ADDS, so the baseline is wrong", tok, vb)
		}
		if strings.TrimSpace(va) == "" {
			t.Errorf("the current render declares no value for --%s on :root. An unset token means "+
				"every var(--%s, <literal>) read in the stylesheet silently falls back, which is "+
				"indistinguishable from the token never having been added", tok, tok)
		}
		delete(rb.Values, key)
		delete(ra.Values, key)
	}

	// The SECOND sanctioned difference, and it only exists in dark mode.
	//
	// `color-scheme: light dark` opts the page into the UA's dark rendering of
	// form controls. Two probed elements have no rule of their own painting them
	// at 1280px — .nav-overlay (a <button>) and .facet-toc__select — so in dark
	// mode they pick up the UA's dark widget colours where the pre-change render,
	// pinned to `color-scheme: light`, kept the light ones. Both are
	// `display: none` at that width, which is asserted below: no reader sees the
	// change. At 375px, where they ARE displayed, the narrow media query paints
	// both and the readings are identical — also asserted, in the same place, so
	// neither half can rot without the other going red.
	uaWidgetKeys := []string{
		"21 nav overlay|.nav-overlay|background-color",
		"23 facet select|.facet-toc__select|background-color",
		"23 facet select|.facet-toc__select|color",
	}
	if scheme == "dark" && width == "1280" {
		for _, k := range uaWidgetKeys {
			if rb.Values[k] == ra.Values[k] {
				t.Errorf("%s is %q in both renders. Adopting `color-scheme: light dark` is supposed "+
					"to hand this control to the UA's dark rendering; if it did not, the change did "+
					"not land", k, ra.Values[k])
			}
			delete(rb.Values, k)
			delete(ra.Values, k)
		}
		for _, sel := range []string{".nav-overlay", ".facet-toc__select"} {
			got := evalString(t, ctxAfter, `getComputedStyle(document.querySelector(`+jsQuote(sel)+`)).display`)
			if got != "none" {
				t.Errorf("%s computes display:%s at 1280px. The UA-widget colour change above is "+
					"only harmless because nobody can see these two at this width; if one is now "+
					"visible, that is a reader-facing dark-mode change and it needs its own decision",
					sel, got)
			}
		}
	}

	const colourSchemeKey = "01 root tokens|html|color-scheme"
	gotBefore, gotAfter := rb.Values[colourSchemeKey], ra.Values[colourSchemeKey]
	if gotBefore != "light" {
		t.Errorf("the pre-change render computes color-scheme %q on the root, want %q — "+
			"the baseline is not what this test believes it is", gotBefore, "light")
	}
	if gotAfter != "light dark" {
		t.Errorf("the current render computes color-scheme %q on the root, want %q — "+
			"the theme work's one deliberate screen-mode change did not land", gotAfter, "light dark")
	}
	delete(rb.Values, colourSchemeKey)
	delete(ra.Values, colourSchemeKey)

	// ---- everything else must be identical ----
	if keys := diffMaps(rb.Values, ra.Values); len(keys) > 0 {
		t.Errorf("%s at %spx in %s mode: %d computed value(s) differ between the pre-change render "+
			"and the engine under test. Every one of these is a change a reader sees in a project "+
			"that sets no theme token:%s",
			fixture, width, scheme, len(keys), describeDiff(rb.Values, ra.Values, keys, "before", "after"))
	}
	t.Logf("%s at %spx in %s mode: %d computed value(s) compared, all identical (plus the one "+
		"sanctioned color-scheme change)", fixture, width, scheme, len(ra.Values))
}

// releasePseudoClass is how a forced :hover is UNDONE.
//
// The obvious spelling — CSS.forcePseudoState with an empty class list — is
// silently a no-op in the browser this suite drives: the call returns success
// and the element stays hovered (measured; the release below is the fix). So
// the release REPLACES the forced set with one inert class instead. :visited is
// the choice because neither style.css nor graph.css contains a single
// :visited selector (a test in internal/render would have to change for that to
// stop being true), and because the browser restricts visited styling to a
// handful of colour properties on anchors anyway — so forcing it on a <button>
// or a <summary> can affect nothing. Every release is verified by reading the
// value back regardless, so this reasoning is belt and the read is braces.
const releasePseudoClass = "visited"

// waitScrollSettled blocks until the page has stopped scrolling.
//
// Navigating to a fragment starts a smooth scroll, and every scroll event makes
// system-record.js re-run renderFacetToc, which replaces the whole facet TOC
// list. Forcing a pseudo-state on one of those buttons while that is happening
// cannot win: the node is gone before the second round trip lands. On a short
// fixture the scroll ends in a few frames and a retry covers it; on an 11 MB
// document it does not, and the retry loop simply races the same rebuild forty
// times. Waiting for the scroll to stop removes the cause rather than the
// symptom.
func waitScrollSettled(t *testing.T, ctx context.Context) {
	t.Helper()
	last := -1.0
	for i := 0; i < 40; i++ {
		var y float64
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.scrollY`, &y)); err != nil {
			t.Fatalf("read scroll position: %v", err)
		}
		if y == last {
			return
		}
		last = y
		if err := chromedp.Run(ctx, chromedp.Sleep(50*time.Millisecond)); err != nil {
			t.Fatalf("wait for the scroll to settle: %v", err)
		}
	}
	t.Fatalf("the page was still scrolling after 2s (last position %v); a hover probe on the "+
		"facet TOC cannot land while the scroll spy is rebuilding it", last)
}

// setPseudoState forces (or clears) pseudo-classes on the first element
// matching sel.
//
// It resolves the node through DOM.getDocument + DOM.querySelector on every
// call rather than through chromedp.Nodes, because chromedp caches a node tree
// and Emulation.setDeviceMetricsOverride invalidates the backend's ids without
// telling that cache: after a viewport resize the cached id is answered with
// "Could not find node with given id (-32000)". Re-asking the document each
// time costs one round trip and removes the whole class.
func setPseudoState(ctx context.Context, sel string, classes []string) error {
	// THE RETRY IS FOR A LIVE PAGE, not flakiness tolerance.
	//
	// system-record.js rebuilds the facet TOC with list.replaceChildren() every
	// time the scroll spy re-evaluates, so every .facet-toc__item is a NEW node
	// each pass. Resolving an id and then forcing a pseudo-state on it are two
	// round trips, and on a large document the rebuild lands between them often
	// enough that a single attempt loses reliably — the CDP answer is "Could not
	// find node with given id (-32000)". Re-resolving after a short pause is the
	// correct response to exactly that, and the pause is what lets an attempt
	// fall between two rebuilds rather than racing the same one eight times.
	//
	// The last attempt's error is still returned, so an element that genuinely
	// is not there fails rather than being waited out.
	var last error
	for attempt := 0; attempt < 40; attempt++ {
		if attempt > 0 {
			if err := chromedp.Run(ctx, chromedp.Sleep(50*time.Millisecond)); err != nil {
				return err
			}
		}
		last = chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
			root, err := dom.GetDocument().Do(c)
			if err != nil {
				return fmt.Errorf("get document: %w", err)
			}
			id, err := dom.QuerySelector(root.NodeID, sel).Do(c)
			if err != nil {
				return fmt.Errorf("query %s: %w", sel, err)
			}
			if id == 0 {
				return fmt.Errorf("no element matches %s", sel)
			}
			return css.ForcePseudoState(id, classes).Do(c)
		}))
		if last == nil {
			return nil
		}
	}
	return last
}

// forceHoverInto forces :hover on each of probe 26's selectors via CDP and
// writes the resulting background into vals. It also asserts the positive
// control the spike established: forcing must CHANGE at least one reading in
// this document, or the whole row is a comparison of two unhovered pages.
func forceHoverInto(t *testing.T, ctx context.Context, vals map[string]string, label string) {
	t.Helper()
	runCDP(t, ctx, css.Enable())
	changed := 0
	for _, sel := range parityProbes[hoverProbeIndex].selectors {
		unforced := evalString(t, ctx, `(function(){var e=document.querySelector(`+jsQuote(sel)+`);`+
			`return e ? getComputedStyle(e).backgroundColor : '<no element>';})()`)
		if err := setPseudoState(ctx, sel, []string{"hover"}); err != nil {
			t.Fatalf("force :hover on %s in the %s document: %v", sel, label, err)
		}
		forced := evalString(t, ctx, `(function(){var e=document.querySelector(`+jsQuote(sel)+`);`+
			`return e ? getComputedStyle(e).backgroundColor : '<no element>';})()`)
		if forced != unforced {
			changed++
		}
		vals["26 hover|"+sel+"|background-color"] = forced

		// Release it again, so the next row's reads see an unhovered page.
		//
		// Whether or not the release call reports success, VERIFY by reading the
		// value back: what matters is that the element is unhovered again, and
		// the read is the evidence for that where a return code is not.
		if err := setPseudoState(ctx, sel, []string{releasePseudoClass}); err != nil {
			t.Logf("releasing :hover on %s in the %s document reported %v; verifying by reading back", sel, label, err)
		}
		back := evalString(t, ctx, `(function(){var e=document.querySelector(`+jsQuote(sel)+`);`+
			`return e ? getComputedStyle(e).backgroundColor : '<no element>';})()`)
		if back != unforced {
			t.Fatalf("%s stayed hovered in the %s document after the release (%q, was %q before "+
				"forcing): every later probe on this tab would be reading a hovered page",
				sel, label, back, unforced)
		}
	}
	if changed == 0 {
		t.Fatalf("forcing :hover changed no background at all in the %s document across %d "+
			"selector(s): CSS.forcePseudoState is not taking effect, so probe 26 compared two "+
			"unhovered pages and proved nothing", label, len(parityProbes[hoverProbeIndex].selectors))
	}
}

// clearEmulatedMedia drops a tab's print/colour-scheme override and REPORTS a
// failure to do so.
//
// Discarding the error here is not harmless tidying. Emulation.setEmulatedMedia
// is per-tab state that outlives the pass that set it, and these tabs are read
// again afterwards; a reset that silently did not land leaves the tab in
// print — in the dark case, print AND dark — so the next reader measures the
// printed palette and calls it the screen one. That is a wrong reading dressed
// as a right one, which is exactly what this file exists to prevent, so it is
// an error rather than a log line.
func clearEmulatedMedia(t *testing.T, ctx context.Context, label string) {
	t.Helper()
	if err := chromedp.Run(ctx, emulation.SetEmulatedMedia()); err != nil {
		t.Errorf("clearing the emulated media on the %s document: %v — that tab is still in the "+
			"emulated mode, so any later read of it reports the wrong scheme", label, err)
	}
}

// transitionSuppressionCSS stops every CSS transition and animation on a page.
//
// It exists because a computed style read during a transition is the
// INTERPOLATED value, not the value the rule sets — and `.content-area` carries
// `transition: padding 180ms ease-out`. Under print emulation the printed
// padding (0) is transitioned to from the screen padding, so a probe that
// arrives mid-flight reads some fraction of the SCREEN value and calls it the
// printed one. Measured factors between 0.86 and 0.92 of the screen padding,
// and the pass failed 2 out of 2 runs under concurrent load while passing on an
// idle machine — which is the worst shape a check can have: green when nothing
// else is happening, and reading the wrong number when it is green.
//
// Suppressing transitions makes the read land on the final value. Both
// documents get the identical sheet, so it changes what is measured, never
// which side of the comparison is favoured.
const transitionSuppressionCSS = "*,*::before,*::after{transition:none !important;" +
	"animation:none !important;caret-color:transparent !important;}"

// suppressTransitions injects transitionSuppressionCSS (plus any extra rules)
// and waits for the page to have no running animation left.
func suppressTransitions(t *testing.T, ctx context.Context, extraCSS string) {
	t.Helper()
	evalVoid(t, ctx, `(function(){
		var st = document.createElement('style');
		st.textContent = `+jsQuote(transitionSuppressionCSS+extraCSS)+`;
		document.head.appendChild(st);
	})()`)
	pollTrue(t, ctx, `document.getAnimations().every(function(a){return a.playState !== 'running';})`)
}

// emulatePrintWithScheme issues plan-v4 A5's ONE combined
// Emulation.setEmulatedMedia call and proves it landed.
//
// The proof needs care. setEmulatedMedia REPLACES the whole override, so the
// mutation this guards against — emulateColorScheme(scheme) followed by a bare
// WithMedia("print") — clears the colour-scheme feature and silently tests
// print in the HOST's scheme. Asserting `matchMedia(scheme).matches` after the
// fact does not catch that on a host whose OS is already in `scheme`: the
// cleared override falls back to the host, and the check passes over a page
// that was never emulated.
//
// So the override is first set to the OPPOSITE feature and that is asserted, in
// the same tab, immediately before. Now the host's own scheme is not the
// expected answer, and "the feature survived" is a statement about the
// emulation rather than about the machine the suite happens to run on. Verified
// by re-applying the two-call mutation in a scratch copy: with this ordering
// the guard fires; without it, it did not.
func emulatePrintWithScheme(t *testing.T, ctx context.Context, scheme, label string) {
	t.Helper()
	opposite := "light"
	if scheme == "light" {
		opposite = "dark"
	}
	runCDP(t, ctx, emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
		{Name: "prefers-color-scheme", Value: opposite},
	}))
	if !evalBool(t, ctx, fmt.Sprintf(`window.matchMedia('(prefers-color-scheme: %s)').matches`, opposite)) {
		t.Fatalf("%s document: could not put the tab in %s before the print pass, so the guard "+
			"below could not tell an emulated %s from this host's own scheme", label, opposite, scheme)
	}

	runCDP(t, ctx, emulation.SetEmulatedMedia().
		WithMedia("print").
		WithFeatures([]*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: scheme}}))
	if !evalBool(t, ctx, `window.matchMedia('print').matches`) {
		t.Fatalf("%s document: print emulation did not take effect", label)
	}
	if !evalBool(t, ctx, fmt.Sprintf(`window.matchMedia('(prefers-color-scheme: %s)').matches`, scheme)) {
		t.Fatalf("%s document: the %s feature did not survive the print emulation — the tab is "+
			"still in %s, so this pass would have measured print in the wrong scheme, which is "+
			"exactly the case that cannot fail", label, scheme, opposite)
	}
}

// runPrintDarkPass is plan-v4 A5's single combined emulation call.
//
// Emulation.setEmulatedMedia REPLACES the whole override, so issuing
// emulateColorScheme and then a bare WithMedia("print") silently clears the
// colour-scheme feature and tests print-in-LIGHT — which is exactly the case
// that cannot fail. One call, both dimensions, and both matchMedia checks
// before a single value is read.
func runPrintDarkPass(t *testing.T, ctxBefore, ctxAfter context.Context, fixture string) {
	// Transitions off BEFORE the media switch, or the values read below are the
	// browser interpolating from the screen state towards the printed one.
	suppressTransitions(t, ctxBefore, "")
	suppressTransitions(t, ctxAfter, "")
	emulatePrintWithScheme(t, ctxBefore, "dark", "before")
	emulatePrintWithScheme(t, ctxAfter, "dark", "after")
	t.Cleanup(func() {
		clearEmulatedMedia(t, ctxBefore, "before")
		clearEmulatedMedia(t, ctxAfter, "after")
	})

	var palBefore, palAfter map[string]string
	evalInto(t, ctxBefore, `window.__dxParity.palette()`, &palBefore)
	evalInto(t, ctxAfter, `window.__dxParity.palette()`, &palAfter)

	// EXPECTED DIFFERENT, asserted positively. The pre-change sheet's dark
	// block had no `screen and` prefix, so a printed page in a dark OS scheme
	// printed the dark palette. The new sheet scopes dark to screen and adds
	// `print` to the light block's media list, so it prints light.
	if palBefore["--paper"] == palAfter["--paper"] {
		t.Errorf("%s: under print in a dark OS scheme the root --paper computes to %q in BOTH "+
			"renders. The whole point of scoping the dark block to `screen and` is that it stops "+
			"applying here, so this is either the fix missing or the emulation not landing.",
			fixture, palBefore["--paper"])
	}
	if palAfter["--ink"] == palBefore["--ink"] {
		t.Errorf("%s: under print in a dark OS scheme --ink is unchanged (%q) between the two renders",
			fixture, palAfter["--ink"])
	}
	t.Logf("%s print x dark: --paper %q -> %q, --ink %q -> %q (the sanctioned palette change)",
		fixture, palBefore["--paper"], palAfter["--paper"], palBefore["--ink"], palAfter["--ink"])

}

// runPrintLightPass is plan-v4 A2's expected-IDENTICAL half, and it is
// deliberately a SEPARATE emulation from the palette pass above.
//
// The consolidated @media print block moved to the end of style.css, which is
// what keeps it beating Layer B's screen rules at equal specificity
// (.content-area { padding: 0 }, .system-record-head { padding-top: 0 }). Had
// it landed anywhere earlier, the printed layout would change — and if that
// were measured in the same pass as the sanctioned dark-to-light palette
// change, a layout regression could hide inside it. Measured in LIGHT, where
// nothing about the palette is supposed to move, every difference here is a
// layout difference and nothing else.
func runPrintLightPass(t *testing.T, ctxBefore, ctxAfter context.Context, fixture string) {
	suppressTransitions(t, ctxBefore, "")
	suppressTransitions(t, ctxAfter, "")
	emulatePrintWithScheme(t, ctxBefore, "light", "before")
	emulatePrintWithScheme(t, ctxAfter, "light", "after")
	t.Cleanup(func() {
		clearEmulatedMedia(t, ctxBefore, "before")
		clearEmulatedMedia(t, ctxAfter, "after")
	})

	var layoutBefore, layoutAfter map[string]map[string]string
	evalInto(t, ctxBefore, `window.__dxParity.printLayout()`, &layoutBefore)
	evalInto(t, ctxAfter, `window.__dxParity.printLayout()`, &layoutAfter)
	if len(layoutAfter) == 0 {
		t.Fatal("the print layout probe returned nothing")
	}
	compared, mismatched := 0, 0
	for sel, propsBefore := range layoutBefore {
		propsAfter := layoutAfter[sel]
		if propsBefore == nil || propsAfter == nil {
			t.Errorf("print layout probe %s matched no element (before=%v, after=%v)", sel, propsBefore, propsAfter)
			mismatched++
			continue
		}
		for prop, want := range propsBefore {
			compared++
			if got := propsAfter[prop]; got != want {
				mismatched++
				t.Errorf("%s: printed layout changed — %s { %s } is %q, was %q. The consolidated "+
					"@media print block must be the LAST block in style.css so it still beats the "+
					"screen rules it used to beat at equal specificity.", fixture, sel, prop, got, want)
			}
		}
	}
	if compared == 0 {
		t.Fatal("the print layout probe compared nothing")
	}

	// THE PRINTED VALUES THEMSELVES, not only "the two agree".
	//
	// Equality between two renders is satisfied just as well by both of them
	// being wrong, and there is a specific way for both to be wrong here: read
	// mid-transition, both report a fraction of the SCREEN padding and match
	// each other to the decimal. Naming what printing is supposed to produce —
	// the print block's `padding: 0`, `padding-top: 0`, and a sidebar that does
	// not print — is what makes a green run mean "this is the printed layout"
	// rather than "these two numbers were equal".
	wantPrinted := map[string]map[string]string{
		"content-area": {
			"paddingTop": "0px", "paddingRight": "0px",
			"paddingBottom": "0px", "paddingLeft": "0px",
		},
		"system-record-head": {"paddingTop": "0px"},
		"sidebar":            {"display": "none"},
	}
	for _, side := range []struct {
		label  string
		layout map[string]map[string]string
	}{{"pre-change", layoutBefore}, {"current", layoutAfter}} {
		for sel, props := range wantPrinted {
			got := side.layout[sel]
			if got == nil {
				t.Errorf("%s: the %s render's print layout probe has no reading for %s", fixture, side.label, sel)
				mismatched++
				continue
			}
			for prop, want := range props {
				if got[prop] != want {
					mismatched++
					t.Errorf("%s: under print, the %s render computes %s { %s } = %q, want %q. "+
						"Either the @media print block is not winning, or this value was read "+
						"while a transition was still running towards it.",
						fixture, side.label, sel, prop, got[prop], want)
				}
			}
		}
	}

	if mismatched == 0 {
		t.Logf("%s print x light: %d layout value(s) compared, unchanged and equal to the printed "+
			"values the print block sets", fixture, compared)
	}
}

// runScreenshotPass compares the painted content area of the two renders.
//
// The clip excludes the document header and the sidebar (whose footer carries a
// generation timestamp two renders taken minutes apart legitimately differ in),
// and an innerText equality assertion runs over the clipped element FIRST, so a
// pixel difference can only be a painting difference.
func runScreenshotPass(t *testing.T, browser, before, after, fixture string) {
	shot := func(url string) (png []byte, text string, w, h float64) {
		ctx := browserContextFor(t, browser)
		runCDP(t, ctx,
			chromedp.EmulateViewport(1280, 900, chromedp.EmulateScale(1)),
			chromedp.Navigate(url),
		)
		pollTrue(t, ctx, `document.readyState === 'complete'`)
		waitVisible(t, ctx, ".content-area")
		emulateColorScheme(t, ctx, "light")
		// FREEZE THE ANIMATIONS AND HIDE THE FACET TOC, identically in both
		// documents, and say what that costs.
		//
		// The sidebar's panel toggles transition their colour, so two captures
		// taken at different points in that transition differ by a unit or two
		// over a 28x28 box. The first version of this pass reported exactly that
		// as a painting difference between the two ENGINES; it was not, and the
		// self-comparison control below is what proved it. Waiting for
		// getAnimations() to quiesce was not enough (a transition that has not
		// started yet is not "running"), so both documents get the same
		// suppression stylesheet instead. This narrows what the screenshot
		// covers — a difference that exists ONLY mid-transition would not be
		// seen — and that bound is the price of the comparison being
		// deterministic at all. The computed-style probes above are unaffected;
		// they read final values, which is what a transition transitions TO.
		//
		// The facet TOC is hidden for a second and worse reason: the viewer
		// rebuilds it continuously (system-record.js replaces its whole list on
		// every scroll-spy pass, measured at ~120 rebuilds a second on a large
		// corpus with the page idle), so the 28x28 box its panel toggle occupies
		// is not the same two frames running. Two captures of the SAME document
		// differ there. That region is therefore OUTSIDE the pixel comparison,
		// and the exclusion is deliberate rather than a tolerance: probes 22, 23
		// and 24 read .facet-toc, .facet-toc__select and .facet-toc__item
		// through computed style, which is not affected by how often the nodes
		// are recreated.
		suppressTransitions(t, ctx, ".facet-toc{visibility:hidden !important;}")

		var rect struct{ X, Y, W, H float64 }
		evalInto(t, ctx, `(function(){var r=document.querySelector('.content-area').getBoundingClientRect();`+
			`return {X:Math.round(r.left+window.scrollX),Y:Math.round(r.top+window.scrollY),`+
			`W:Math.round(r.width),H:Math.round(Math.min(r.height,3000))};})()`, &rect)
		if rect.W <= 0 || rect.H <= 0 {
			t.Fatalf("the .content-area clip is %vx%v on %s — nothing would be captured", rect.W, rect.H, url)
		}
		txt := evalString(t, ctx, `document.querySelector('.content-area').innerText`)

		var buf []byte
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
			var err error
			buf, err = page.CaptureScreenshot().
				WithCaptureBeyondViewport(true).
				WithClip(&page.Viewport{
					X: rect.X, Y: rect.Y, Width: rect.W, Height: rect.H, Scale: 1,
				}).Do(c)
			return err
		})); err != nil {
			t.Fatalf("screenshot %s: %v", url, err)
		}
		return buf, txt, rect.W, rect.H
	}

	// THE NOISE FLOOR, measured rather than assumed. Two captures of the SAME
	// document must be pixel-identical, or a difference between two DIFFERENT
	// documents says nothing. This is the control that turns the comparison
	// below into evidence; without it a flaky renderer and a broken stylesheet
	// are the same red.
	pngSelfA, _, _, _ := shot(after)
	pngSelfB, _, _, _ := shot(after)
	if n, maxD, fx, fy := comparePNGs(t, pngSelfA, pngSelfB); n > 0 {
		t.Fatalf("%s: two captures of the SAME document differ in %d pixel(s), first at (%d,%d), "+
			"largest channel delta %d. The capture is not deterministic, so the before/after "+
			"comparison below could not distinguish a stylesheet change from capture noise.",
			fixture, n, fx, fy, maxD)
	}

	pngBefore, textBefore, wb, hb := shot(before)
	pngAfter, textAfter, wa, ha := shot(after)

	if len(pngBefore) == 0 || len(pngAfter) == 0 {
		t.Fatalf("a clipped screenshot came back empty (%d / %d bytes)", len(pngBefore), len(pngAfter))
	}
	if strings.TrimSpace(textBefore) == "" {
		t.Fatal("the clipped content area has no text at all; the pixel comparison below would be " +
			"a comparison of two blank images")
	}
	if textBefore != textAfter {
		line, wl, gl := firstDifferingParityLine(textBefore, textAfter)
		t.Fatalf("%s: the clipped content area's TEXT differs at line %d, so a pixel comparison "+
			"would report a difference that is not about painting:\n  before: %q\n  after:  %q",
			fixture, line, wl, gl)
	}
	if wb != wa || hb != ha {
		t.Fatalf("%s: the content area is %vx%v in the pre-change render and %vx%v in the current "+
			"one — the layout moved", fixture, wb, hb, wa, ha)
	}
	// PIXELS, not PNG bytes. Two encoders (or one encoder on two runs) can spell
	// the same image differently, so a byte comparison answers a question nobody
	// asked and would report a difference no reader could see. Decoding and
	// walking the pixels answers the question this test is actually about — did
	// anything paint differently — and it can say WHERE and BY HOW MUCH, which a
	// byte count cannot.
	diff, maxDelta, firstX, firstY := comparePNGs(t, pngBefore, pngAfter)
	if diff > 0 {
		t.Errorf("%s: the painted content area (%vx%v, clipped to exclude the header and the "+
			"timestamped sidebar footer) differs in %d pixel(s), first at (%d,%d), largest "+
			"channel delta %d — with identical text and identical geometry. Something the "+
			"fourteen var() reads touch is painting differently.",
			fixture, wa, ha, diff, firstX, firstY, maxDelta)
	} else {
		t.Logf("%s: the painted content area (%vx%v) is pixel-identical", fixture, wa, ha)
	}
}

// comparePNGs decodes two PNGs and reports the number of differing pixels, the
// largest per-channel delta, and the first differing coordinate. A size or
// decode mismatch fails immediately: comparing images of different sizes would
// silently compare only their overlap.
func comparePNGs(t *testing.T, a, b []byte) (diff, maxDelta, firstX, firstY int) {
	t.Helper()
	ia, _, err := image.Decode(bytes.NewReader(a))
	if err != nil {
		t.Fatalf("decode the pre-change screenshot: %v", err)
	}
	ib, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode the current screenshot: %v", err)
	}
	ra, rb := ia.Bounds(), ib.Bounds()
	if ra != rb {
		t.Fatalf("the two screenshots are %v and %v; a comparison would only cover their overlap", ra, rb)
	}
	if ra.Dx() == 0 || ra.Dy() == 0 {
		t.Fatalf("the screenshots are empty (%v); a pixel comparison would be vacuous", ra)
	}
	firstX, firstY = -1, -1
	for y := ra.Min.Y; y < ra.Max.Y; y++ {
		for x := ra.Min.X; x < ra.Max.X; x++ {
			r1, g1, b1, a1 := ia.At(x, y).RGBA()
			r2, g2, b2, a2 := ib.At(x, y).RGBA()
			if r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2 {
				continue
			}
			diff++
			if firstX < 0 {
				firstX, firstY = x, y
			}
			for _, d := range []int{int(r1) - int(r2), int(g1) - int(g2), int(b1) - int(b2), int(a1) - int(a2)} {
				if d < 0 {
					d = -d
				}
				if d>>8 > maxDelta {
					maxDelta = d >> 8
				}
			}
		}
	}
	return diff, maxDelta, firstX, firstY
}

func firstDifferingParityLine(a, b string) (line int, aLine, bLine string) {
	as, bs := strings.Split(a, "\n"), strings.Split(b, "\n")
	n := len(as)
	if len(bs) < n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		if as[i] != bs[i] {
			return i + 1, as[i], bs[i]
		}
	}
	return n + 1, "<end>", "<end>"
}

// ---------------------------------------------------------------------
// The negative control
// ---------------------------------------------------------------------

// TestThemeParityHarnessSeesAPerturbedFallback is the control on the control.
//
// Everything above is a "these two agree" assertion, and such an assertion
// passes just as loudly when the harness is broken — a probe that reads nothing,
// a diff that compares two empty maps, a hover pass that never fires. So this
// takes the CURRENT render, changes exactly one character in one fallback
// (--hover-bg's .08 becomes .09, a change no reader would notice and no unit
// test over the CSS source would see, because it is in the rendered artifact),
// and requires the same probe program to report it.
func TestThemeParityHarnessSeesAPerturbedTokenValue(t *testing.T) {
	browser := resolveBrowser(t)
	clean := renderFixtureFresh(t, "fixture-theme-flat")
	src := strings.TrimPrefix(clean, "file://")
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read the fresh render: %v", err)
	}

	// TWO perturbations, because they answer two different questions.
	//
	//   decl     the :root declaration of --hover-bg. This is the value a
	//            reader actually gets, so changing it MUST be reported. That is
	//            the control on the harness.
	//
	//   fallback the literal inside every var(--hover-bg, <literal>) read.
	//            Changing it must be reported as NOTHING, because the token is
	//            declared on :root and the fallback is therefore dead in the
	//            shipped sheet. It is not decoration: it is what a project
	//            using viewer.template_overrides to ship its OWN style.css
	//            falls back to. Pinning that here is the only place the
	//            difference between the two is stated in a running test.
	//
	// (plan §6.2 asked for the fallback perturbation alone as the negative
	// control. On the sheet that shipped, it cannot fire — the token is set —
	// so a control built only on it would have passed vacuously forever.)
	const declFrom = "--hover-bg: rgba(125, 137, 154, .08);"
	const declTo = "--hover-bg: rgba(125, 137, 154, .09);"
	const fbFrom = "var(--hover-bg, rgba(125, 137, 154, .08))"
	const fbTo = "var(--hover-bg, rgba(125, 137, 154, .09))"
	for _, needle := range []string{declFrom, fbFrom} {
		if !strings.Contains(string(body), needle) {
			t.Fatalf("the current render does not contain %q, so this control cannot perturb "+
				"anything — the spelling changed and this test needs updating with it", needle)
		}
	}

	write := func(name, content string) string {
		p := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return "file://" + p
	}
	declURL := write("perturbed-decl.html", strings.Replace(string(body), declFrom, declTo, 1))
	fbURL := write("perturbed-fallback.html", strings.ReplaceAll(string(body), fbFrom, fbTo))

	readHover := func(url, label string) map[string]string {
		ctx := parityTab(t, browser, url, 1280, 900)
		evalVoid(t, ctx, `window.__dxParity.stripActive()`)
		emulateColorScheme(t, ctx, "light")
		vals := map[string]string{}
		forceHoverInto(t, ctx, vals, label)
		return vals
	}

	valsClean := readHover(clean, "clean")
	valsDecl := readHover(declURL, "perturbed-declaration")
	valsFallback := readHover(fbURL, "perturbed-fallback")

	if keys := diffMaps(valsClean, valsDecl); len(keys) == 0 {
		t.Fatalf("the hover probe reported NO difference between a render and a copy of it whose "+
			":root --hover-bg was changed from .08 to .09. Every parity assertion in this file is "+
			"a \"they agree\" assertion, so a harness that cannot see this change would report "+
			"agreement no matter what shipped.\n  readings: %v", valsClean)
	} else {
		t.Logf("negative control: the perturbed :root token was reported on %d probe key(s):%s",
			len(keys), describeDiff(valsClean, valsDecl, keys, "clean", "perturbed"))
	}

	if keys := diffMaps(valsClean, valsFallback); len(keys) != 0 {
		t.Errorf("changing the FALLBACK literal in every var(--hover-bg, ...) read changed %d "+
			"computed value(s):%s\nThat should be impossible while --hover-bg is declared on "+
			":root — if a fallback is winning, some consumer is reading a token the engine does "+
			"not set.", len(keys), describeDiff(valsClean, valsFallback, keys, "clean", "perturbed"))
	} else {
		t.Log("the var() fallbacks are dead in the shipped sheet, as designed: every token is " +
			"declared on :root, so the literals only matter to a project shipping its own " +
			"style.css through viewer.template_overrides")
	}
}
