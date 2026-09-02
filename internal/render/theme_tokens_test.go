package render

// Structural invariants of the two embedded stylesheets, and the link between
// them and internal/config's ThemeTokenAllowlist.
//
// These are invariants about how the source produces the output, which is the
// half a rendered-output check cannot cover: a token that no rule reads renders
// identically to one that is read, on the day it is added. Everything here is
// read through shellFS — the same embed the render package serves from — so a
// stray edit to a file that is not embedded cannot make these pass.
//
// What is NOT covered here, stated so the bound is visible: nothing in this
// file loads the CSS into an engine. It cannot tell you that a rule wins the
// cascade, only that the declaration exists and that the file's block
// structure is what the theme mechanism assumes. Cascade outcomes are the
// browser parity suite's job (viewer-tests/theme_parity_test.go).

import (
	"regexp"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
)

// embeddedCSS reads one template file through the package's own embed.FS and
// fails the test if it is missing or empty. An empty file would satisfy almost
// every "must not contain" assertion below, so the emptiness guard is what
// stops this file from passing over nothing.
func embeddedCSS(t *testing.T, path string) string {
	t.Helper()
	b, err := shellFS.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s from shellFS: %v", path, err)
	}
	if len(b) == 0 {
		t.Fatalf("%s is empty; every structural assertion below would pass vacuously", path)
	}
	return string(b)
}

// cssComment matches a CSS comment. Every structural count in this file runs
// over comment-stripped text: style.css's header comment names `@media print`,
// `:root` and `(prefers-color-scheme: light), print` in prose, and counting
// those as code would make the block counts wrong in both directions.
// CSS comments do not nest, and neither stylesheet contains `/*` inside a
// string, so the naive strip is exact here.
var cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// stripComments blanks comments while preserving byte offsets, so an offset
// comparison (the print-block placement guard) still refers to real positions
// in the file.
func stripComments(css string) string {
	return cssComment.ReplaceAllStringFunc(css, func(m string) string {
		var b strings.Builder
		for _, r := range m {
			if r == '\n' {
				b.WriteRune('\n')
			} else {
				b.WriteRune(' ')
			}
		}
		return b.String()
	})
}

// consumerPattern is the CSS arm of the consumption check: a `var(--token` whose
// next non-space character is `,` (a fallback follows) or `)` (no fallback).
// The trailing class is what keeps `--shadow` from matching `var(--shadow-strong`.
func consumerPattern(token string) *regexp.Regexp {
	return regexp.MustCompile(`var\(--` + regexp.QuoteMeta(token) + `\s*[,)]`)
}

// jsConsumerPattern is the JS arm: graph-ui.js reads custom properties by name
// through getComputedStyle().getPropertyValue(), where the name appears as a
// single-quoted literal rather than inside a var().
func jsConsumerPattern(token string) string { return "'--" + token + "'" }

// TestEveryAllowlistedTokenHasAConsumer pins the allowlist to the stylesheets:
// a token a project is allowed to set must be read by something, or setting it
// is a silent no-op the project cannot distinguish from a typo.
//
// The JS arm is a standing allowance, not a requirement. Today NO token needs
// it: all 28 are matched by the CSS arm alone (verified by the subtest below,
// which runs the CSS arm on its own and expects zero misses). graph-ui.js does
// name seven of them ('--accent', '--faint', '--ink', '--link', '--muted',
// '--paper', '--warn') for the graph pane's palette read, so the arm is live
// code rather than dead machinery — but no token depends on it.
func TestEveryAllowlistedTokenHasAConsumer(t *testing.T) {
	style := embeddedCSS(t, styleTemplatePath)
	graph := embeddedCSS(t, graphCSSTemplatePath)
	js := embeddedCSS(t, graphUITemplatePath)
	css := style + "\n" + graph

	if got, want := len(config.ThemeTokenAllowlist), 28; got != want {
		t.Errorf("len(config.ThemeTokenAllowlist) = %d, want %d; "+
			"the allowlist and this file's coverage are meant to move together, "+
			"so update both deliberately", got, want)
	}

	for _, token := range config.ThemeTokenAllowlist {
		inCSS := consumerPattern(token).MatchString(css)
		inJS := strings.Contains(js, jsConsumerPattern(token))
		if !inCSS && !inJS {
			t.Errorf("theme token %q has no consumer: no %s in style.css or graph.css, "+
				"and no %s in graph-ui.js. A project setting viewer.theme.%s would see "+
				"no change and no error.",
				token, consumerPattern(token), jsConsumerPattern(token), token)
		}
	}

	t.Run("CSSArmAloneCoversEveryToken", func(t *testing.T) {
		// This is what licenses the comment above. If a token ever legitimately
		// moves to JS-only, this subtest is the one to change, and changing it
		// is the moment to re-read that comment.
		for _, token := range config.ThemeTokenAllowlist {
			if !consumerPattern(token).MatchString(css) {
				t.Errorf("token %q is not read by any CSS rule; it is JS-only, "+
					"which is allowed but is no longer what the doc comment on "+
					"TestEveryAllowlistedTokenHasAConsumer says", token)
			}
		}
	})

	t.Run("NegativeControlCSSArm", func(t *testing.T) {
		// A name shaped exactly like a real token but present nowhere. If this
		// matched, the CSS arm would be accepting anything and every assertion
		// above would be meaningless.
		const absent = "not-a-real-token-9f3a"
		if consumerPattern(absent).MatchString(css) {
			t.Fatalf("CSS arm matched %q, which appears in neither stylesheet", absent)
		}
		if strings.Contains(js, jsConsumerPattern(absent)) {
			t.Fatalf("JS arm matched %q, which appears in graph-ui.js nowhere", absent)
		}
	})

	t.Run("NegativeControlPrefixIsNotEnough", func(t *testing.T) {
		// --shadow-strong exists and is read; --shadow-str does not. Without the
		// `\s*[,)]` tail the pattern for the shorter name would match the longer
		// token's var() and report coverage that is not there.
		if consumerPattern("shadow-str").MatchString(css) {
			t.Fatal("consumerPattern(\"shadow-str\") matched, so the pattern is " +
				"matching on prefix; --shadow would then be reported as consumed " +
				"by var(--shadow-strong)")
		}
		if !consumerPattern("shadow-strong").MatchString(css) {
			t.Fatal("consumerPattern(\"shadow-strong\") did not match; the control " +
				"above proves nothing if the real token is absent too")
		}
	})

	t.Run("PositiveControlJSArm", func(t *testing.T) {
		// The JS arm has no live dependency today, so nothing above would fail if
		// it silently stopped working. This plants the literal in the test's own
		// input to prove the arm can fire.
		const planted = "planted-token-b71c"
		synthetic := "var s = getComputedStyle(document.documentElement).getPropertyValue('--" +
			planted + "');"
		if !strings.Contains(synthetic, jsConsumerPattern(planted)) {
			t.Fatal("JS arm did not match a getPropertyValue call that names the " +
				"token; the arm cannot fire and the allowance above is dead")
		}
		if strings.Contains(js, jsConsumerPattern(planted)) {
			t.Fatalf("the planted name %q is in the real graph-ui.js; pick another", planted)
		}
	})
}

// TestStyleCSSModeAndPrintStructure pins the block structure the theme
// mechanism depends on. internal/render emits the project's theme as a second
// sheet using the same shapes; if this file's shapes drift, the two stop
// agreeing and a project's dark-only token silently reaches paper.
func TestStyleCSSModeAndPrintStructure(t *testing.T) {
	raw := embeddedCSS(t, styleTemplatePath)
	css := stripComments(raw)

	t.Run("ExactlyOneUnconditionalRoot", func(t *testing.T) {
		// Anchored at column 0: a `:root` nested inside an @media is indented.
		n := len(regexp.MustCompile(`(?m)^:root\s*\{`).FindAllString(css, -1))
		if n != 1 {
			t.Errorf("found %d unconditional `:root {` blocks in style.css, want 1. "+
				"Two of them means the palette's winner depends on source order, "+
				"which is how the dead block deleted in this branch came about.", n)
		}
	})

	t.Run("ExactlyOneScreenScopedDarkQuery", func(t *testing.T) {
		n := strings.Count(css, "@media screen and (prefers-color-scheme: dark)")
		if n != 1 {
			t.Errorf("found %d `@media screen and (prefers-color-scheme: dark)` "+
				"blocks in style.css, want 1", n)
		}
	})

	t.Run("EveryDarkQueryIsScreenScoped", func(t *testing.T) {
		assertDarkQueriesScreenScoped(t, "style.css", css)
	})

	t.Run("EveryLightQueryAlsoMatchesPrint", func(t *testing.T) {
		assertLightQueriesIncludePrint(t, "style.css", css)
	})

	t.Run("ExactlyOnePrintBlockAndItIsLast", func(t *testing.T) {
		if n := strings.Count(css, "@media print"); n != 1 {
			t.Fatalf("found %d `@media print` blocks in style.css, want 1. "+
				"More than one means the printed page's winner depends on which "+
				"block a declaration happens to sit in.", n)
		}
		printAt := strings.Index(css, "@media print")

		last := -1
		for _, m := range regexp.MustCompile(`@media`).FindAllStringIndex(css, -1) {
			if m[0] > last {
				last = m[0]
			}
		}
		if printAt != last {
			t.Errorf("the `@media print` block is at byte %d but the last @media in "+
				"style.css is at byte %d. print must be last: it overrides screen "+
				"layout rules at equal specificity and wins on source order alone.",
				printAt, last)
		}

		// The specific rule the placement exists for. `.content-area { padding: 0 }`
		// inside the print block beats the padded screen rule only because it comes
		// later; if print moved above it, the printed page silently regains the
		// sidebar gutter and nothing else in the suite would notice.
		padded := strings.Index(css, "padding: 0 282px")
		if padded < 0 {
			t.Fatal("could not find the padded `.content-area` rule " +
				"(`padding: 0 282px`) in style.css; this guard is checking " +
				"nothing until it is re-pointed at whatever replaced it")
		}
		rule := strings.LastIndex(css[:padded], ".content-area {")
		if rule < 0 {
			t.Fatalf("`padding: 0 282px` at byte %d is not inside a `.content-area {` "+
				"rule; re-point this guard", padded)
		}
		if printAt <= rule {
			t.Errorf("the `@media print` block starts at byte %d, before the screen "+
				"`.content-area` rule at byte %d that it overrides", printAt, rule)
		}
	})

	t.Run("PrintPinsLightColorScheme", func(t *testing.T) {
		printAt := strings.Index(css, "@media print")
		if printAt < 0 {
			t.Fatal("no @media print block in style.css")
		}
		if !strings.Contains(css[printAt:], "color-scheme: light;") {
			t.Error("the @media print block does not pin `color-scheme: light`. " +
				"Without it a dark-OS reader's form controls, scrollbars and canvas " +
				"backdrop are painted dark on paper, even though the custom " +
				"properties are light.")
		}
	})

	t.Run("RadiusConsumerCountIsPinned", func(t *testing.T) {
		// Pinned rather than bounded: --radius is the one length token, and the
		// four surfaces that honour it (the code block, the comment panel, the
		// comment chip and the facet card) are a deliberate set. A fifth consumer
		// changes what `radius: 10px` does to an existing project's viewer.
		n := len(regexp.MustCompile(`var\(--radius\s*[,)]`).FindAllString(css, -1))
		if n != 4 {
			t.Errorf("style.css has %d --radius consumers, want exactly 4", n)
		}
	})
}

// TestGraphCSSModeStructure pins graph.css's own conventions. graph.css keeps
// the opposite base (dark-first) from style.css deliberately; what the two must
// agree on is the print outcome.
func TestGraphCSSModeStructure(t *testing.T) {
	raw := embeddedCSS(t, graphCSSTemplatePath)
	css := stripComments(raw)

	t.Run("LightQueryIsTheDeclaredForm", func(t *testing.T) {
		const want = "@media (prefers-color-scheme: light), print {"
		if n := strings.Count(css, want); n != 1 {
			t.Errorf("found %d occurrences of %q in graph.css, want 1. graph.css is "+
				"dark-first, so its LIGHT block is the one that has to carry `print` "+
				"or a printed graph pane keeps the dark ramp on white paper.", n, want)
		}
		assertLightQueriesIncludePrint(t, "graph.css", css)
		assertDarkQueriesScreenScoped(t, "graph.css", css)
	})

	t.Run("EveryDxgReadIsResolvable", func(t *testing.T) {
		// graph.css is emitted BEFORE style.css, and style.css re-declares
		// --dxg-facet-other. Every other --dxg-* name is graph.css's alone, so an
		// unresolvable read here paints nothing at all rather than falling back.
		declared := map[string]bool{}
		for _, m := range regexp.MustCompile(`(?m)^\s*(--dxg-[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
			declared[m[1]] = true
		}
		if len(declared) == 0 {
			t.Fatal("graph.css declares no --dxg-* custom properties; the check below " +
				"would pass over nothing")
		}
		reads := regexp.MustCompile(`var\((--dxg-[a-z0-9-]+)\s*([,)])`).FindAllStringSubmatch(css, -1)
		if len(reads) == 0 {
			t.Fatal("graph.css contains no `var(--dxg-...)` reads; the check below " +
				"would pass over nothing")
		}
		for _, m := range reads {
			name, tail := m[1], m[2]
			if tail == "," {
				continue // has a fallback
			}
			if !declared[name] {
				t.Errorf("graph.css reads %s with no fallback and declares it nowhere; "+
					"the property resolves to the guaranteed-invalid value and the "+
					"element is painted with the initial value instead", name)
			}
		}
	})
}

// assertLightQueriesIncludePrint enforces plan v4 A6: a light-appearance media
// query must also match print, so paper takes the light values whatever the
// reader's OS appearance is.
func assertLightQueriesIncludePrint(t *testing.T, name, css string) {
	t.Helper()
	found := false
	for _, m := range regexp.MustCompile(`\(prefers-color-scheme:\s*light\)`).FindAllStringIndex(css, -1) {
		found = true
		rest := css[m[1]:]
		if !strings.HasPrefix(rest, ", print") {
			t.Errorf("%s byte %d: `%s` is not immediately followed by `, print`; "+
				"the light values it carries will not reach a printed page",
				name, m[0], css[m[0]:m[1]])
		}
	}
	if name == "graph.css" && !found {
		t.Error("graph.css contains no light media query at all; it is dark-first " +
			"and needs one")
	}
}

// assertDarkQueriesScreenScoped enforces plan v4 A6's other half: a dark
// query must be screen-scoped, or a dark-OS reader gets dark ink on paper.
func assertDarkQueriesScreenScoped(t *testing.T, name, css string) {
	t.Helper()
	const prefix = "screen and "
	for _, m := range regexp.MustCompile(`\(prefers-color-scheme:\s*dark\)`).FindAllStringIndex(css, -1) {
		before := css[:m[0]]
		if !strings.HasSuffix(before, prefix) {
			t.Errorf("%s byte %d: `%s` is not preceded by %q. Print media carries no "+
				"appearance, so an unscoped dark query applies to paper and nothing "+
				"puts the light palette back.",
				name, m[0], css[m[0]:m[1]], strings.TrimSpace(prefix))
		}
	}
}

// TestStripCommentsIsHonest is a control on the helper every structural count
// above depends on. If stripComments silently did nothing, style.css's header
// comment (which names @media print, :root and the light query in prose) would
// be counted as code and the block counts would be wrong.
func TestStripCommentsIsHonest(t *testing.T) {
	in := "a{}\n/* @media print { :root { color-scheme: light; } }\n(prefers-color-scheme: dark) */\nb{}\n"
	got := stripComments(in)
	for _, needle := range []string{"@media print", ":root", "prefers-color-scheme"} {
		if strings.Contains(got, needle) {
			t.Errorf("stripComments left %q inside a comment", needle)
		}
	}
	if !strings.Contains(got, "a{}") || !strings.Contains(got, "b{}") {
		t.Error("stripComments removed code outside the comment")
	}
	if len(got) != len(in) {
		t.Errorf("stripComments changed length %d -> %d; byte offsets in the print "+
			"placement guard would no longer refer to real positions", len(in), len(got))
	}
	if strings.Count(got, "\n") != strings.Count(in, "\n") {
		t.Error("stripComments changed the line count")
	}
	// And it must actually be reached: the real header comment does contain
	// these strings, so the counts above are not accidentally correct.
	raw := embeddedCSS(t, styleTemplatePath)
	if !strings.Contains(cssComment.FindString(raw), "@media print") {
		t.Error("style.css's leading comment no longer mentions `@media print`; " +
			"the stripping control is no longer exercising anything real")
	}
}
