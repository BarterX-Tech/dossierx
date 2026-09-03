package viewertests

// A THEME THE PROJECT WROTE IS A THEME THE READER GETS.
//
// theme_parity_test.go proves the negative half of this feature: a project that
// sets nothing sees nothing change. This file proves the positive half, which is
// the half a project actually buys — that a token written in
// project.config.yaml reaches the pixels, in the right colour scheme, at both
// widths, and that a font the project ships is loaded rather than merely named.
//
// The claim needs care because the cheap version of it is worthless. "The value
// appears in the HTML" proves the emitter ran. "The custom property resolves on
// :root" proves the declaration parsed. Neither says a single consumer reads it,
// and a token nothing reads is a token that does nothing. So the per-token
// promise below finds each token's CONSUMERS in the shipped stylesheet — through
// the browser's own CSSOM, not a hand-written table that can rot — and requires
// the computed value on a matching element to CHANGE against an otherwise
// identical unthemed render.
//
// Coverage is a number this file asserts: 28 of 28 tokens, in both modes, at
// both widths. Not "the ones that happened to work".

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// The themed project
// ---------------------------------------------------------------------

// themeValue returns a distinctive, valid value for one token in one mode.
//
// Distinctive matters twice over: the value must differ from the engine default
// (or "the token changed something" is unprovable) AND from every other token's
// value (or a consumer reading the wrong token would still look right). The
// colour is derived from the token's index so both properties hold by
// construction rather than by inspection.
func themeValue(token string, index int, mode string) string {
	switch token {
	case "font-sans":
		if mode == "dark" {
			return "DXParityDarkSans, ui-sans-serif, sans-serif"
		}
		return "DXParityLightSans, ui-sans-serif, sans-serif"
	case "font-mono":
		if mode == "dark" {
			return "DXParityDarkMono, ui-monospace, monospace"
		}
		return "DXParityLightMono, ui-monospace, monospace"
	case "radius":
		if mode == "dark" {
			return "17px"
		}
		return "13px"
	}
	// 28 tokens, two modes: a red channel of 0x20+index*4 and a blue channel
	// that separates the modes gives 56 values none of which collide and none
	// of which is an engine default.
	blue := 0x10
	if mode == "dark" {
		blue = 0xd0
	}
	return fmt.Sprintf("#%02x40%02x", 0x20+index*4, blue)
}

// themedConfigYAML writes a config whose viewer.theme sets ALL 28 tokens in
// BOTH modes, so the per-token promise can be read in either scheme.
func themedConfigYAML() string {
	var b strings.Builder
	b.WriteString("schema_version: 1\ntitle: Theme Modes Project\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\nviewer:\n  theme:\n")
	for _, mode := range []string{"light", "dark"} {
		fmt.Fprintf(&b, "    %s:\n", mode)
		for i, tok := range themeTokens {
			fmt.Fprintf(&b, "      %s: %q\n", tok, themeValue(tok, i, mode))
		}
	}
	return b.String()
}

const unthemedConfigYAML = `schema_version: 1
title: Theme Modes Project
facets:
  - contract
modules:
  - widget
claims_dir: claims
`

// themedClaimYAML instantiates the constructs the token consumers hang off, so
// the probes below have real elements wherever the project can supply them.
const themedClaimYAML = `id: widget.contract.overview
facet: contract
module: widget
status: draft
layout: card
body: |
  An inline ` + "`code`" + ` span, a fenced block:

  ` + "```" + `
  fenced
  ` + "```" + `

  | field | meaning |
  | --- | --- |
  | id | identity |
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

// newThemedProject builds a project with the given config, one claim, and
// (optionally) the committed probe font copied in.
func newThemedProject(t *testing.T, configYAML string, withFont bool) *project {
	t.Helper()
	p := newProjectRaw(t, configYAML)
	p.writeClaim("overview.yaml", themedClaimYAML)
	if withFont {
		copyProbeFont(t, p)
	}
	return p
}

// copyProbeFont puts viewer-tests/testdata/fonts/probe.ttf into the project's
// own fonts/ directory. tests/probe_font_test.go pins that this copy is
// byte-identical to the one testdata/fixture-theme-preset ships, so the face
// measured here is the face the committed viewer embeds.
func copyProbeFont(t *testing.T, p *project) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	src := filepath.Join(root, "viewer-tests", "testdata", "fonts", "probe.ttf")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read the committed probe font: %v", err)
	}
	if len(b) != 940 {
		t.Fatalf("the committed probe font is %d bytes, want 940 — the font changed and every "+
			"glyph-width floor below was calibrated against the old one", len(b))
	}
	dir := filepath.Join(p.dir, "fonts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fonts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe.ttf"), b, 0o644); err != nil {
		t.Fatalf("write probe font: %v", err)
	}
}

// ---------------------------------------------------------------------
// Consumer discovery
// ---------------------------------------------------------------------

// consumerProbeJS installs window.__dxTheme, which walks the page's OWN
// stylesheets to find every declaration that reads a token, and reads the
// resulting computed value off a matching element.
//
// Discovering consumers from the CSSOM rather than from a table in this file is
// deliberate. A hand-written table says "this is where --shadow was read when I
// wrote it down"; the stylesheet says where it is read now. The table would go
// quietly stale on the first refactor and keep passing, which is the failure
// mode a coverage claim must not have.
const consumerProbeJS = `
window.__dxTheme = (function () {
  function rules() {
    var out = [];
    for (var i = 0; i < document.styleSheets.length; i++) {
      var sheet = document.styleSheets[i], list;
      try { list = sheet.cssRules; } catch (e) { continue; }
      if (!list) { continue; }
      walk(list, out);
    }
    return out;
  }
  // A style rule is collected AND descended into. Since CSS nesting shipped,
  // CSSStyleRule itself exposes a (usually empty) cssRules list, so the
  // obvious "if (r.cssRules) { recurse } else { collect }" collects nothing at
  // all — it treats every ordinary rule as a grouping rule. That is not a
  // near-miss: it reports zero consumers for all 28 tokens and the coverage
  // assertion below then fails loudly, which is how it was found.
  function walk(list, out) {
    for (var i = 0; i < list.length; i++) {
      var r = list[i];
      if (r.selectorText && r.style) { out.push(r); }
      if (r.cssRules && r.cssRules.length) { walk(r.cssRules, out); }
    }
  }

  // consumers(token) -> [{selector, prop}] for every declaration in the
  // shipped sheets whose value reads var(--token, ...).
  //
  // The declarations are read out of rule.cssText rather than by iterating
  // CSSStyleDeclaration, because a shorthand written with a var() —
  // "background: var(--paper)" — stores a PENDING-SUBSTITUTION value on its
  // longhands, and getPropertyValue returns "" for every one of them. Walking
  // the style object therefore finds nothing at all, which is not a small
  // inaccuracy: it is a coverage claim that silently reports zero. (It did, on
  // the first run of this file: 0 consumers for all 28 tokens.)
  //
  // Custom-property declarations are excluded on purpose. ":root {
  // --code-inline-bg: color-mix(..., var(--paper), ...) }" reads --paper, but
  // satisfying the promise with it would only prove one token feeds another.
  // A token has to reach a real CSS property to have reached a reader.
  var SHORTHAND = {
    'background': 'background-color',
    'border': 'border-top-color',
    'border-top': 'border-top-color',
    'border-right': 'border-right-color',
    'border-bottom': 'border-bottom-color',
    'border-left': 'border-left-color',
    'border-color': 'border-top-color',
    'outline': 'outline-color',
    'font': 'font-family',
    'flex': 'flex-basis'
  };

  function declarationsReading(cssText, token) {
    var needle = 'var(--' + token;
    var body = cssText.slice(cssText.indexOf('{') + 1, cssText.lastIndexOf('}'));
    var out = [], from = 0;
    while (true) {
      var at = body.indexOf(needle, from);
      if (at < 0) { break; }
      from = at + needle.length;
      var after = body.charAt(from);
      // "--shadow" must not match a read of "--shadow-strong".
      if (after !== ',' && after !== ')' && after !== ' ') { continue; }
      // The property name is what precedes the nearest ':' after the last ';'
      // (or the start of the block) before the match.
      var start = body.lastIndexOf(';', at) + 1;
      var colon = body.indexOf(':', start);
      if (colon < 0 || colon > at) { continue; }
      var prop = body.slice(start, colon).trim();
      if (!prop || prop.indexOf('--') === 0) { continue; }
      out.push(SHORTHAND[prop] || prop);
    }
    return out;
  }

  function consumers(token) {
    var seen = {}, out = [];
    var rs = rules();
    for (var i = 0; i < rs.length; i++) {
      var props = declarationsReading(rs[i].cssText, token);
      for (var j = 0; j < props.length; j++) {
        var key = rs[i].selectorText + '|' + props[j];
        if (seen[key]) { continue; }
        seen[key] = true;
        out.push({ selector: rs[i].selectorText, prop: props[j] });
      }
    }
    return out;
  }

  // readOne(selector, prop) -> the computed value on the first element the
  // selector matches, or null.
  //
  // A selector LIST is split so one non-matching member does not lose the whole
  // rule. Within a member, a state pseudo-CLASS (:hover, :target, ...) is
  // dropped, because a computed-style read cannot see one; the parity test's
  // forcePseudoState pass is what covers those. A pseudo-ELEMENT (::selection
  // and friends) is kept and passed to getComputedStyle as its second argument
  // instead — dropping it would silently turn a ::selection rule into a read of
  // the element itself, and --selection-bg has exactly one consumer, a bare
  // "::selection" rule, so it is the whole coverage of that token.
  var PSEUDO_ELEMENT = /::(selection|before|after|marker|placeholder|first-line|first-letter)\b/;

  function readOne(selector, prop) {
    var parts = selector.split(',');
    for (var i = 0; i < parts.length; i++) {
      var raw = parts[i].trim();
      var pe = null;
      var m = raw.match(PSEUDO_ELEMENT);
      if (m) { pe = m[0]; }
      var s = raw
        .replace(PSEUDO_ELEMENT, '')
        .replace(/::?(hover|focus|focus-visible|focus-within|active|target|visited|checked|disabled)\b/g, '')
        .replace(/::[a-z-]+(\([^)]*\))?/g, '')
        .trim();
      if (!s && pe) { s = 'body'; }   // a bare "::selection" rule
      if (!s) { continue; }
      var el = null;
      try { el = document.querySelector(s); } catch (e) { continue; }
      if (!el) { continue; }
      var v = getComputedStyle(el, pe).getPropertyValue(prop);
      return { selector: s + (pe || ''), value: String(v == null ? '' : v).trim() };
    }
    return null;
  }

  // tokenReadings(tokens) -> { token: {consumers: n, readings: {"sel|prop": value}} }
  function tokenReadings(tokens) {
    var out = {};
    for (var i = 0; i < tokens.length; i++) {
      var tok = tokens[i];
      var cs = consumers(tok);
      var readings = {};
      for (var j = 0; j < cs.length; j++) {
        var got = readOne(cs[j].selector, cs[j].prop);
        if (got) { readings[got.selector + '|' + cs[j].prop] = got.value; }
      }
      out[tok] = { consumers: cs.length, readings: readings };
    }
    return out;
  }

  function rootToken(name) {
    return String(getComputedStyle(document.documentElement).getPropertyValue('--' + name) || '').trim();
  }

  return { consumers: consumers, tokenReadings: tokenReadings, rootToken: rootToken };
})();
`

type tokenReading struct {
	Consumers int               `json:"consumers"`
	Readings  map[string]string `json:"readings"`
}

func themeTab(t *testing.T, browser, url string, w, h int64) context.Context {
	t.Helper()
	ctx := browserContextFor(t, browser)
	runCDP(t, ctx,
		chromedp.EmulateViewport(w, h, chromedp.EmulateScale(1)),
		chromedp.Navigate(url),
	)
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	evalVoid(t, ctx, parityProbeJS)
	evalVoid(t, ctx, `window.__dxParity.install()`)
	evalVoid(t, ctx, consumerProbeJS)
	return ctx
}

func readTokenReadings(t *testing.T, ctx context.Context) map[string]tokenReading {
	t.Helper()
	var out map[string]tokenReading
	spec, err := jsonList(themeTokens)
	if err != nil {
		t.Fatalf("marshal token list: %v", err)
	}
	evalInto(t, ctx, `window.__dxTheme.tokenReadings(`+spec+`)`, &out)
	if len(out) != len(themeTokens) {
		t.Fatalf("the consumer probe returned %d token(s), want %d", len(out), len(themeTokens))
	}
	return out
}

func jsonList(ss []string) (string, error) {
	var b strings.Builder
	b.WriteString("[")
	for i, s := range ss {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(jsQuote(s))
	}
	b.WriteString("]")
	return b.String(), nil
}

// ---------------------------------------------------------------------
// The per-token promise
// ---------------------------------------------------------------------

func TestEveryThemeTokenReachesTheReader(t *testing.T) {
	browser := resolveBrowser(t)

	themed := newThemedProject(t, themedConfigYAML(), false)
	plain := newThemedProject(t, unthemedConfigYAML, false)
	themedURL := themed.renderStatic()
	plainURL := plain.renderStatic()

	for _, vp := range parityWidths {
		vp := vp
		t.Run(vp.name, func(t *testing.T) {
			ctxThemed := themeTab(t, browser, themedURL, vp.w, vp.h)
			ctxPlain := themeTab(t, browser, plainURL, vp.w, vp.h)

			for _, mode := range []string{"light", "dark"} {
				mode := mode
				t.Run(mode, func(t *testing.T) {
					emulateColorScheme(t, ctxThemed, mode)
					emulateColorScheme(t, ctxPlain, mode)

					themedReadings := readTokenReadings(t, ctxThemed)
					plainReadings := readTokenReadings(t, ctxPlain)

					var noConsumer, noElement, noChange []string
					verified := 0
					for i, tok := range themeTokens {
						want := themeValue(tok, i, mode)

						// The declaration must resolve on the root, and to the
						// value the project wrote. A token that reaches :root
						// with someone else's value is a merge bug.
						got := evalString(t, ctxThemed, `window.__dxTheme.rootToken(`+jsQuote(tok)+`)`)
						if got != want {
							t.Errorf("--%s resolves to %q on the root in %s mode, want %q",
								tok, got, mode, want)
						}

						tr := themedReadings[tok]
						pr := plainReadings[tok]
						if tr.Consumers == 0 {
							noConsumer = append(noConsumer, tok)
							continue
						}
						if len(tr.Readings) == 0 {
							noElement = append(noElement, fmt.Sprintf("%s (%d consumer declaration(s), none matched an element)", tok, tr.Consumers))
							continue
						}
						changedAt := ""
						for key, themedVal := range tr.Readings {
							if plainVal, ok := pr.Readings[key]; ok && plainVal != themedVal {
								changedAt = fmt.Sprintf("%s: %q -> %q", key, plainVal, themedVal)
								break
							}
						}
						if changedAt == "" {
							noChange = append(noChange, fmt.Sprintf("%s (consumers: %v)", tok, sortedKeys(tr.Readings)))
							continue
						}
						verified++
						t.Logf("--%s %s", tok, changedAt)
					}

					sort.Strings(noConsumer)
					sort.Strings(noElement)
					sort.Strings(noChange)
					if len(noConsumer) > 0 {
						t.Errorf("%d token(s) are read by NO declaration in the shipped stylesheets, "+
							"so setting them does nothing at all: %v", len(noConsumer), noConsumer)
					}
					if len(noElement) > 0 {
						t.Errorf("%d token(s) have consumer declarations but no element on this page "+
							"matched any of them, so the promise is unverified rather than kept: %v",
							len(noElement), noElement)
					}
					if len(noChange) > 0 {
						t.Errorf("%d token(s) changed no computed value against the unthemed render: %v",
							len(noChange), noChange)
					}

					// THE COVERAGE BOUND, asserted. A run that verified 26 of 28
					// and said nothing would be the exact shape of a check that
					// did not run and reads as a pass.
					if verified != len(themeTokens) {
						t.Fatalf("verified %d of %d tokens in %s mode at %spx; every token must be "+
							"shown to change something a reader sees",
							verified, len(themeTokens), mode, vp.name)
					}
					t.Logf("%s mode at %spx: %d/%d tokens verified", mode, vp.name, verified, len(themeTokens))
				})
			}
		})
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------
// Two modes, one page
// ---------------------------------------------------------------------

// twoModeConfigYAML is the shape the feature exists for: one token that applies
// in both schemes and one that is split. Its values are chosen to differ from
// the engine defaults, which the test asserts rather than assumes.
const twoModeConfigYAML = `schema_version: 1
title: Two Mode Project
facets:
  - contract
modules:
  - widget
claims_dir: claims
viewer:
  theme:
    accent: "#c6613f"
    light:
      paper: "#fffdf6"
    dark:
      paper: "#141014"
`

func TestThemeModesTrackTheColourScheme(t *testing.T) {
	browser := resolveBrowser(t)
	p := newThemedProject(t, twoModeConfigYAML, false)
	url := p.renderStatic()

	plain := newThemedProject(t, unthemedConfigYAML, false)
	plainURL := plain.renderStatic()

	ctx := themeTab(t, browser, url, 1280, 900)
	ctxPlain := themeTab(t, browser, plainURL, 1280, 900)

	const (
		wantAccent    = "#c6613f"
		wantLightPap  = "#fffdf6"
		wantDarkPaper = "#141014"
	)
	// rgb() is what a browser serializes a hex background to, so the body
	// assertion is written against that rather than against the hex.
	want := map[string]string{"light": "rgb(255, 253, 246)", "dark": "rgb(20, 16, 20)"}

	seen := map[string]string{}
	for _, mode := range []string{"light", "dark"} {
		emulateColorScheme(t, ctx, mode)
		emulateColorScheme(t, ctxPlain, mode)

		// The configured value must differ from the engine default, or "the
		// project's value is showing" is not a statement about anything.
		defPaper := evalString(t, ctxPlain, `window.__dxTheme.rootToken("paper")`)
		cfgPaper := evalString(t, ctx, `window.__dxTheme.rootToken("paper")`)
		if defPaper == cfgPaper {
			t.Fatalf("in %s mode the configured --paper (%q) equals the engine default; this test "+
				"could not tell a working override from a broken one", mode, cfgPaper)
		}

		bg := evalString(t, ctx, `getComputedStyle(document.querySelector('body.shell-body')).backgroundColor`)
		if bg != want[mode] {
			t.Errorf("in %s mode the body background is %q, want %q (the configured --paper)", mode, bg, want[mode])
		}
		seen[mode] = bg

		// The SHARED token is identical in both modes, non-empty, and equal to
		// what the project wrote.
		acc := evalString(t, ctx, `window.__dxTheme.rootToken("accent")`)
		if acc != wantAccent {
			t.Errorf("in %s mode --accent is %q, want %q — a flat token must apply in both schemes",
				mode, acc, wantAccent)
		}
	}
	if seen["light"] == seen["dark"] {
		t.Fatalf("the body background is %q in both modes: the per-mode blocks are not being "+
			"selected, so every assertion above measured one scheme twice", seen["light"])
	}

	// The graph pane's chrome tracks the mode too. It is drawn from the same
	// custom properties, and it is the one surface that reads them from
	// JavaScript at draw time rather than through the cascade.
	openGraphPane(t, ctx)
	waitVisible(t, ctx, "#dxgPane .dxg-canvas")
	paneSeen := map[string]string{}
	for _, mode := range []string{"light", "dark"} {
		emulateColorScheme(t, ctx, mode)
		paneSeen[mode] = evalString(t, ctx,
			`getComputedStyle(document.querySelector('#dxgPane')).backgroundColor`)
	}
	if paneSeen["light"] == paneSeen["dark"] {
		t.Errorf("the graph pane's background is %q in both modes; it does not track the colour scheme",
			paneSeen["light"])
	}
	t.Logf("graph pane background: light %s, dark %s", paneSeen["light"], paneSeen["dark"])
}

// ---------------------------------------------------------------------
// A dark-only token must not escape into print (plan-v4 A1/A5, C-N14)
// ---------------------------------------------------------------------

// darkOnlyConfigYAML sets one token in `dark:` and gives it no light
// counterpart. That is legal, and it is the case that used to leak: with the
// engine's dark block unscoped, a printed page in a dark OS scheme printed the
// project's dark ink on white paper.
const darkOnlyConfigYAML = `schema_version: 1
title: Dark Only Project
facets:
  - contract
modules:
  - widget
claims_dir: claims
viewer:
  theme:
    dark:
      ink: "#eeeeee"
`

func TestDarkOnlyTokenDoesNotApplyToPrint(t *testing.T) {
	browser := resolveBrowser(t)
	p := newThemedProject(t, darkOnlyConfigYAML, false)
	url := p.renderStatic()
	plain := newThemedProject(t, unthemedConfigYAML, false)
	plainURL := plain.renderStatic()

	ctx := themeTab(t, browser, url, 1280, 900)
	ctxPlain := themeTab(t, browser, plainURL, 1280, 900)

	// The engine's own light value, read off an unthemed render in light mode.
	// Reading it rather than hardcoding it means this test cannot disagree with
	// the stylesheet about what "the engine light value" is.
	emulateColorScheme(t, ctxPlain, "light")
	engineLightInk := evalString(t, ctxPlain, `window.__dxTheme.rootToken("ink")`)
	if engineLightInk == "" {
		t.Fatal("the unthemed render declares no --ink at all; there is nothing to compare against")
	}

	// On screen, in dark, the project's value wins.
	emulateColorScheme(t, ctx, "dark")
	if got := evalString(t, ctx, `window.__dxTheme.rootToken("ink")`); got != "#eeeeee" {
		t.Fatalf("on screen in dark mode --ink is %q, want %q — the dark-only override did not land, "+
			"so the print assertion below would pass for the wrong reason", got, "#eeeeee")
	}

	// Under print in a dark OS scheme, it must not.
	//
	// emulatePrintWithScheme issues ONE Emulation.setEmulatedMedia call carrying
	// both dimensions, having first put the tab in the OPPOSITE scheme and
	// asserted that. The two-call mutation this guards against —
	// emulateColorScheme(dark) then a bare WithMedia("print") — clears the
	// colour-scheme feature and falls back to the HOST's scheme, so on a machine
	// whose OS is dark a bare "is it dark?" check passes over a page that was
	// never emulated. Coming from light makes the answer about the emulation.
	//
	// Transitions are suppressed first for the same reason the parity print
	// passes do it: a computed read taken while a transition is running is the
	// interpolated value, not the one the rule sets.
	suppressTransitions(t, ctx, "")
	emulatePrintWithScheme(t, ctx, "dark", "dark-only")
	got := evalString(t, ctx, `window.__dxTheme.rootToken("ink")`)
	if got == "#eeeeee" {
		t.Fatalf("under print in a dark OS scheme --ink is still the project's DARK value %q. "+
			"A dark-only token has escaped the print pin: this prints near-white ink on white "+
			"paper and the page comes out blank.", got)
	}
	if got != engineLightInk {
		t.Errorf("under print in a dark OS scheme --ink is %q, want the engine's light value %q",
			got, engineLightInk)
	}
	t.Logf("dark-only --ink: screen/dark %q, print/dark %q (engine light %q)",
		"#eeeeee", got, engineLightInk)
}

// ---------------------------------------------------------------------
// Fonts
// ---------------------------------------------------------------------

// fontConfigYAML declares the committed probe face and names it in font-sans,
// which is what the engine requires before it will emit the @font-face at all.
const fontConfigYAML = `schema_version: 1
title: Font Project
facets:
  - contract
modules:
  - widget
claims_dir: claims
viewer:
  theme:
    font-sans: "DossierX Probe, ui-sans-serif, sans-serif"
    fonts:
      - family: DossierX Probe
        src: fonts/probe.ttf
        weight: "400"
        style: normal
`

// fontEvidenceJS measures the face three ways in one page: the face registry's
// own status, and the rendered width of one glyph in the custom family against
// the same glyph in `monospace` and in a family that does not exist.
//
// Measuring self-relatively inside ONE page is the point. An absolute width is a
// statement about the machine's fonts; a ratio between two spans on the same
// line in the same document is a statement about which face was used.
const fontEvidenceJS = `
window.__dxFont = function () {
  var host = document.createElement('div');
  host.setAttribute('aria-hidden', 'true');
  host.style.cssText = 'position:absolute;left:-99999px;top:0;font-size:100px;white-space:pre';
  host.innerHTML =
    '<span id="dxf-probe" style="font-family:\'DossierX Probe\'">a</span>' +
    '<span id="dxf-mono" style="font-family:monospace">a</span>' +
    '<span id="dxf-missing" style="font-family:\'DXNoSuchFamily\'">a</span>';
  document.body.appendChild(host);
  var faces = [];
  document.fonts.forEach(function (f) { faces.push(f.family + '=' + f.status); });
  function w(id) { return document.getElementById(id).getBoundingClientRect().width; }
  var out = {
    faces: faces,
    probeWidth: w('dxf-probe'),
    monoWidth: w('dxf-mono'),
    missingWidth: w('dxf-missing'),
    check: document.fonts.check('100px "DossierX Probe"'),
    bodyFamily: getComputedStyle(document.body).fontFamily
  };
  host.parentNode.removeChild(host);
  return out;
};
`

type fontEvidence struct {
	Faces        []string `json:"faces"`
	ProbeWidth   float64  `json:"probeWidth"`
	MonoWidth    float64  `json:"monoWidth"`
	MissingWidth float64  `json:"missingWidth"`
	Check        bool     `json:"check"`
	BodyFamily   string   `json:"bodyFamily"`
}

// glyphWidthFloor is the minimum probeWidth/monoWidth ratio that counts as
// evidence the custom face was used. The probe glyph is 2000 units on a
// 1000-unit em — two ems, 200px at font-size:100px — and no shipped monospace
// face is anywhere near that; measured at 200 vs 60.2 (a ratio of 3.3) in this
// suite's browser. The floor is set at 2.0 so a normal-width fallback (ratio
// well under 1.5) cannot clear it and a differently-metricked monospace cannot
// break it.
const glyphWidthFloor = 2.0

func assertProbeFontLoaded(t *testing.T, ctx context.Context, where string) {
	t.Helper()
	// Wait on the FontFaceSet's own status first. It is a native property, so
	// unlike a flag set from a promise callback it survives the page reload the
	// live-reload channel can trigger moments after a served page loads — which
	// is exactly what left the serve leg of this test hanging on a flag that a
	// wiped window would never set again.
	pollTrue(t, ctx, `document.fonts.status === 'loaded'`)
	// Then document.fonts.ready itself, which is what the plan asks for. It is
	// AWAITED in the browser rather than parked on a window flag this side
	// polls: under serve the live-reload channel can reload the page just after
	// load, and a reload wipes the flag, leaving the poll to time out against a
	// page whose fonts have long since finished. (It did — twenty seconds of it,
	// twice.) Awaiting the promise in-place has no state to lose.
	var fontsReady bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(
		`document.fonts.ready.then(function () { return true; })`, &fontsReady,
		func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) },
	)); err != nil {
		t.Fatalf("%s: awaiting document.fonts.ready: %v", where, err)
	}
	if !fontsReady {
		t.Fatalf("%s: document.fonts.ready did not resolve true", where)
	}
	evalVoid(t, ctx, fontEvidenceJS)

	var ev fontEvidence
	evalInto(t, ctx, `window.__dxFont()`, &ev)

	loaded := false
	for _, f := range ev.Faces {
		if f == "DossierX Probe=loaded" {
			loaded = true
		}
	}
	if !loaded {
		t.Fatalf("%s: the face registry does not report DossierX Probe as loaded; it holds %v", where, ev.Faces)
	}
	if !ev.Check {
		t.Errorf("%s: document.fonts.check says 100px \"DossierX Probe\" is not available", where)
	}
	if !strings.Contains(ev.BodyFamily, "DossierX Probe") {
		t.Errorf("%s: the body's font-family is %q and does not name the project's face", where, ev.BodyFamily)
	}
	if ev.MonoWidth <= 0 || ev.MissingWidth <= 0 {
		t.Fatalf("%s: a control glyph measured %v/%v px wide; the ratios below would be meaningless",
			where, ev.MonoWidth, ev.MissingWidth)
	}
	ratio := ev.ProbeWidth / ev.MonoWidth
	if ratio < glyphWidthFloor {
		t.Errorf("%s: the probe glyph is %.1fpx wide against %.1fpx for monospace (ratio %.2f, floor "+
			"%.1f). The face is REGISTERED but the browser did not draw with it, which is exactly "+
			"what a font-family string proves and a glyph does not.",
			where, ev.ProbeWidth, ev.MonoWidth, ratio, glyphWidthFloor)
	}
	// The negative control: a family that does not exist must NOT produce the
	// same width. Without it, a measurement that always returned the fallback's
	// width would satisfy the floor on a machine whose default font happens to
	// be wide.
	if ev.MissingWidth == ev.ProbeWidth {
		t.Errorf("%s: a glyph in a non-existent family measured the same %.1fpx as the probe face; "+
			"the measurement is not sensitive to which family is asked for", where, ev.MissingWidth)
	}
	t.Logf("%s: probe %.1fpx, monospace %.1fpx (ratio %.2f), missing-family %.1fpx",
		where, ev.ProbeWidth, ev.MonoWidth, ratio, ev.MissingWidth)
}

func TestProjectFontLoadsOnAFileURLAndUnderServe(t *testing.T) {
	browser := resolveBrowser(t)

	t.Run("file://", func(t *testing.T) {
		p := newThemedProject(t, fontConfigYAML, true)
		url := p.renderStatic()
		ctx := themeTab(t, browser, url, 1280, 900)
		assertProbeFontLoaded(t, ctx, "file://")
	})

	// Under serve, the same face has to survive the Content-Security-Policy the
	// server sends. A data: font URL is refused by a policy that does not name
	// font-src data:, and refused silently — the page simply falls back — so
	// this leg is the only place that gate is exercised against a real browser.
	t.Run("serve", func(t *testing.T) {
		p := newThemedProject(t, fontConfigYAML, true)
		p.run("check")
		base := p.ensureServe()
		ctx := browserContextFor(t, browser)
		runCDP(t, ctx,
			chromedp.EmulateViewport(1280, 900, chromedp.EmulateScale(1)),
			chromedp.Navigate(base+"/"),
		)
		pollTrue(t, ctx, `document.readyState === 'complete'`)
		assertProbeFontLoaded(t, ctx, "under serve")
	})
}

// TestServeDoesNotPickUpAThemeFileEditUntilRestart pins documented behaviour
// rather than wished-for behaviour.
//
// serve resolves the theme once, at startup. A project editing its theme FILE
// while the server runs sees nothing change until it restarts, and that is worth
// a test because the live-reload machinery next door makes the opposite a
// perfectly reasonable expectation. Pinning it means the day it changes, it
// changes on purpose.
func TestServeDoesNotPickUpAThemeFileEditUntilRestart(t *testing.T) {
	browser := resolveBrowser(t)

	dir := t.TempDir()
	themeDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatalf("mkdir themes: %v", err)
	}
	themeFile := filepath.Join(themeDir, "house.yaml")
	writeTheme := func(paper string) {
		if err := os.WriteFile(themeFile, []byte("paper: \""+paper+"\"\n"), 0o644); err != nil {
			t.Fatalf("write theme file: %v", err)
		}
	}
	writeTheme("#fffdf6")

	cfg := `schema_version: 1
title: Theme File Project
facets:
  - contract
modules:
  - widget
claims_dir: claims
viewer:
  theme:
    extends: themes/house.yaml
`
	p := newProjectRaw(t, cfg)
	p.writeClaim("overview.yaml", themedClaimYAML)
	// newProjectRaw made its own temp dir; move the theme file beside its config.
	if err := os.MkdirAll(filepath.Join(p.dir, "themes"), 0o755); err != nil {
		t.Fatalf("mkdir themes in project: %v", err)
	}
	themeFile = filepath.Join(p.dir, "themes", "house.yaml")
	writeTheme("#fffdf6")

	p.run("check")
	base, stop := p.serve()

	read := func(ctx context.Context) string {
		return evalString(t, ctx, `window.__dxTheme.rootToken("paper")`)
	}
	openTab := func() context.Context {
		ctx := browserContextFor(t, browser)
		runCDP(t, ctx, chromedp.Navigate(base+"/"))
		pollTrue(t, ctx, `document.readyState === 'complete'`)
		evalVoid(t, ctx, consumerProbeJS)
		return ctx
	}

	before := read(openTab())
	if before != "#fffdf6" {
		t.Fatalf("the served page starts with --paper %q, want %q; the rest of this test would "+
			"be measuring the wrong thing", before, "#fffdf6")
	}

	writeTheme("#101010")
	// A fresh tab, so nothing is a caching artifact of the first one.
	during := read(openTab())
	if during != "#fffdf6" {
		t.Errorf("after editing the theme FILE, a freshly loaded page under the SAME serve process "+
			"reports --paper %q. serve resolves the theme once at startup; if that has changed, it "+
			"is a behaviour change and this test is where it should be re-decided", during)
	}

	stop()
	p.base = ""
	base, _ = p.serve()
	after := read(openTab())
	if after != "#101010" {
		t.Errorf("after restarting serve, the page reports --paper %q, want the edited %q — the "+
			"edit is not picked up even on restart, which is a different and worse bug", after, "#101010")
	}
	t.Logf("theme file edit: %q before, %q under the same server, %q after restart", before, during, after)
}
