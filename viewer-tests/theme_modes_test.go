package viewertests

// Built-in reader mode choices remain supported after custom project themes are removed.
import (
	"context"
	"github.com/chromedp/chromedp"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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

func newReaderModeProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, unthemedConfigYAML)
	p.writeClaim("overview.yaml", themedClaimYAML)
	return p
}

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
	if err := os.RemoveAll(filepath.Join(dst, "build", "viewer")); err != nil {
		t.Fatalf("drop committed viewer: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dst, "build", "catalog")); err != nil {
		t.Fatalf("drop committed catalog: %v", err)
	}
	cfg := filepath.Join(dst, "project.config.yaml")
	cmd := exec.Command(bin, "--config", cfg, "--format", "text", "check")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("check %s: %v\n%s", fixture, err, out)
	}
	out := filepath.Join(dst, "build", "viewer", "index.html")
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("check wrote no viewer for %s: %v", fixture, err)
	}
	return "file://" + out
}

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

var engineLightDefaults = map[string]string{
	"paper":       "#EFF1F4",
	"ink":         "#101720",
	"accent":      "#2C6B52",
	"link":        "#1C4E8C",
	"dxg-facet-1": "#4257C4",
}

// engineDarkStart are the same five under a dark OS with the control on
// System. They are asserted BEFORE the Light press, so "the values moved" is a
// statement about the press and not about a page that was light all along.
var engineDarkStart = map[string]string{
	"paper":       "#0D1117",
	"ink":         "#E6EAF0",
	"accent":      "#63BE9A",
	"link":        "#6AA6E8",
	"dxg-facet-1": "#7C8CE8",
}

// TestThemeControlLightOverridesDarkOS is the mirror of
// TestThemeControlDarkOverridesLightOS (viewer-tests/component_fit_test.go) and
// it was red when it was written.
//
// The viewer's theme control has three positions and the engine had blocks for
// two of them. style.css is light-first — its light values are the unconditional
// `:root`, specificity (0,1,0) — and its OS-dark query re-points twenty-three of
// them at that same specificity, LATER in the file. So on a dark OS the dark
// values won on source order, and the explicit Light choice had nothing at
// (0,1,1) to answer with: html[data-theme="light"] simply did not exist. Every
// token a project had not themed itself stayed dark under a lit Light control.
// graph.css had the same hole for its --dxg-* ramp, from the opposite side.
//
// The check runs on an UNTHEMED project on purpose: a themed one would be
// carried by the project's own html[data-theme="light"] rule, which is the one
// path that already worked, and would hide exactly the defect this is for.
func TestThemeControlLightOverridesDarkOS(t *testing.T) {
	browser := resolveBrowser(t)

	readToken := func(ctx context.Context, token string) string {
		return evalString(t, ctx,
			`getComputedStyle(document.documentElement).getPropertyValue('--`+token+`').trim()`)
	}
	press := func(ctx context.Context, choice string) {
		if choice == "system" {
			evalVoid(t, ctx, `localStorage.setItem('dossierx-theme', 'system')`)
			runCDP(t, ctx, chromedp.Reload())
			pollTrue(t, ctx, `document.readyState === 'complete' && document.documentElement.getAttribute('data-theme') === 'system'`)
			return
		}
		evalVoid(t, ctx, `(function(){
			var b = document.querySelector('.theme-control [data-theme-choice="`+choice+`"]');
			if (!b) { throw new Error('`+choice+` theme control is missing'); }
			b.click();
		})()`)
		if got := evalString(t, ctx, `document.documentElement.getAttribute('data-theme') || ''`); got != choice {
			t.Fatalf("the %s control did not set data-theme=%s (got %q); nothing below "+
				"would be measuring the explicit-%s path", choice, choice, got, choice)
		}
	}
	openOnADarkOS := func(url string) context.Context {
		ctx := browserContextFor(t, browser)
		runCDP(t, ctx,
			chromedp.EmulateViewport(1280, 900, chromedp.EmulateScale(1)),
			chromedp.Navigate(url),
		)
		pollTrue(t, ctx, `document.readyState === 'complete'`)
		emulateColorScheme(t, ctx, "dark")
		if !evalBool(t, ctx, `window.matchMedia('(prefers-color-scheme: dark)').matches`) {
			t.Fatal("the OS must stay DARK, or pressing Light is not an override of anything")
		}
		pollTrue(t, ctx, `!!document.querySelector('.theme-control [data-theme-choice="light"]')`)
		return ctx
	}

	// ---- the unthemed engine: the whole palette has to move ----
	plain := newReaderModeProject(t)
	ctx := openOnADarkOS(plain.renderStatic())

	for token, want := range engineDarkStart {
		if got := readToken(ctx, token); got != want {
			t.Fatalf("on a dark OS with the control on System, --%s is %q, want the engine's "+
				"dark default %q. The page is not in the state this test measures a change "+
				"away from, so every assertion below would prove nothing.", token, got, want)
		}
	}

	press(ctx, "light")

	for token, want := range engineLightDefaults {
		got := readToken(ctx, token)
		if got == engineDarkStart[token] {
			t.Errorf("after the reader pressed Light on a dark OS, --%s is still the engine's "+
				"DARK value %q. An explicit Light choice has to beat the OS for every token, "+
				"not only the ones a project themed.", token, got)
			continue
		}
		if got != want {
			t.Errorf("after the reader pressed Light on a dark OS, --%s is %q, want the engine's "+
				"light value %q", token, got, want)
		}
	}

	// color-scheme has to follow the choice too, or the palette is light and the
	// form controls, the scrollbars and every light-dark() resolution are dark.
	scheme := func() string {
		return evalString(t, ctx,
			`getComputedStyle(document.documentElement).getPropertyValue('color-scheme').trim()`)
	}
	if got := scheme(); got != "light" {
		t.Errorf("after pressing Light on a dark OS, :root color-scheme is %q, want \"light\"", got)
	}
	press(ctx, "dark")
	if got := scheme(); got != "dark" {
		t.Errorf("after pressing Dark, :root color-scheme is %q, want \"dark\"", got)
	}
	press(ctx, "system")
	if got := scheme(); got != "light dark" {
		t.Errorf("on System, :root color-scheme is %q, want the unconditional \"light dark\" "+
			"so the OS decides", got)
	}

}
