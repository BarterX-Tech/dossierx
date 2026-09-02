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
	"fmt"
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
		// style.css legitimately has ZERO light media queries: it is light-first,
		// so the light values live in the unconditional :root and there is
		// nothing for a light query to say. wantAtLeastOne is false here for that
		// reason, and the count is reported so a reader can see the check is
		// vacuous ON PURPOSE rather than by accident. graph.css, which is
		// dark-first, passes true.
		n := assertLightQueriesIncludePrint(t, "style.css", css, false)
		if n != 0 {
			t.Logf("style.css now has %d light media queries; it had none when this "+
				"guard was written, because the light values are the :root defaults. "+
				"Not a failure — but check the new query is deliberate.", n)
		}
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

	t.Run("PrintPinsLightColorSchemeInItsOwnRoot", func(t *testing.T) {
		printAt := strings.Index(css, "@media print")
		if printAt < 0 {
			t.Fatal("no @media print block in style.css")
		}
		block := css[printAt:]

		// Exactly one :root inside the print block. Two would mean the printed
		// page's custom properties depend on which of them a declaration sits in,
		// which is the same defect the unconditional-:root count guards against.
		roots := regexp.MustCompile(`:root\s*\{`).FindAllStringIndex(block, -1)
		if len(roots) != 1 {
			t.Fatalf("the @media print block contains %d `:root {` rules, want exactly 1",
				len(roots))
		}

		// And the pin must be INSIDE that :root, not merely somewhere in the
		// block. `color-scheme` on any other selector does not set the page's.
		openAt := roots[0][1]
		closeAt := strings.Index(block[openAt:], "}")
		if closeAt < 0 {
			t.Fatal("the print block's `:root {` is never closed")
		}
		body := block[openAt : openAt+closeAt]
		if !strings.Contains(body, "color-scheme: light;") {
			t.Errorf("the print block's :root does not pin `color-scheme: light` "+
				"(its body is %q). Without it a dark-OS reader's form controls, "+
				"scrollbars and canvas backdrop are painted dark on paper, even "+
				"though the custom properties are light.", strings.TrimSpace(body))
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
		assertLightQueriesIncludePrint(t, "graph.css", css, true)
		// graph.css has no dark media query at all — dark is its unconditional
		// base — so this call is vacuous BY DESIGN. It is made anyway so that a
		// dark query added to graph.css later cannot arrive unscoped.
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
// assertLightQueriesIncludePrint enforces plan v4 A6: a light-appearance media
// query must also match print, so paper takes the light values whatever the
// reader's OS appearance is. It returns how many light queries it found, and
// wantAtLeastOne says whether finding none is itself a failure — a file that is
// light-first (style.css) legitimately has none, a file that is dark-first
// (graph.css) must have one or its light ramp is unreachable.
func assertLightQueriesIncludePrint(t *testing.T, name, css string, wantAtLeastOne bool) int {
	t.Helper()
	found := 0
	for _, m := range regexp.MustCompile(`\(prefers-color-scheme:\s*light\)`).FindAllStringIndex(css, -1) {
		found++
		rest := css[m[1]:]
		if !strings.HasPrefix(rest, ", print") {
			t.Errorf("%s byte %d: `%s` is not immediately followed by `, print`; "+
				"the light values it carries will not reach a printed page",
				name, m[0], css[m[0]:m[1]])
		}
	}
	if wantAtLeastOne && found == 0 {
		t.Errorf("%s contains no light media query at all; it is dark-first and "+
			"needs one, or its light values never apply", name)
	}
	return found
}

// assertDarkQueriesScreenScoped enforces plan v4 A6's other half: a dark
// query must be screen-scoped, or a dark-OS reader gets dark ink on paper.
// For graph.css this is vacuous by design (dark is that file's unconditional
// base, so it has no dark query to scope); it is still called there so a dark
// query added later cannot arrive unscoped.
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

// cssDecl is one declaration, with the exact selector text of the rule it sits
// in (whitespace-collapsed) and its source offset.
type cssDecl struct {
	selector string
	property string
	text     string
	offset   int
}

// parseDecls walks comment-stripped CSS and yields every declaration with the
// selector of its innermost rule. It is a lexer, not a parser: it tracks brace
// depth and treats the text before each `{` as that block's prelude, which is
// enough for this stylesheet (no nested selectors, no CSS-in-strings braces).
func parseDecls(css string) []cssDecl {
	var out []cssDecl
	var stack []string
	var pending strings.Builder
	space := regexp.MustCompile(`\s+`)

	emit := func(end int) {
		d := strings.TrimSpace(pending.String())
		pending.Reset()
		if d == "" || len(stack) == 0 {
			return
		}
		sel := stack[len(stack)-1]
		if strings.HasPrefix(sel, "@") {
			return // a declaration directly inside an at-rule prelude: not ours
		}
		prop, _, ok := strings.Cut(d, ":")
		if !ok {
			return
		}
		out = append(out, cssDecl{
			selector: sel,
			property: strings.TrimSpace(prop),
			text:     d,
			offset:   end - len(d),
		})
	}

	for i, r := range css {
		switch r {
		case '{':
			stack = append(stack, space.ReplaceAllString(strings.TrimSpace(pending.String()), " "))
			pending.Reset()
		case '}':
			emit(i)
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			pending.Reset()
		case ';':
			emit(i)
		default:
			pending.WriteRune(r)
		}
	}
	return out
}

// consumerSite is one (selector text, property) pair that must read a token.
type consumerSite struct {
	selector string
	property string
}

// tokenConsumers maps each of the fourteen tokens added for the custom-theme
// work to every place it must be read. The existing fourteen tokens are not
// listed: they predate this work, are read in dozens of places, and pinning
// their sites would be a maintenance tax with no defect to prevent.
var tokenConsumers = map[string][]consumerSite{
	"code-inline-bg": {{"code", "background"}},
	// The fenced-block list is the same four bodies markdown.Render emits into
	// that table-head-bg and image-bg below already name, plus .claim-tree-body
	// (a <pre> the template writes directly, with no inner <code>).
	"code-bg": {{".claim-body pre, .claim-list-items pre, .sbody pre, " +
		".comment-body pre, .claim-tree-body", "background"}},
	"table-head-bg": {{".claim-body .md-table th, .claim-list-items .md-table th, " +
		".sbody .md-table th, .comment-body .md-table th", "background"}},
	"image-bg": {{".claim-body .md-img, .claim-list-items .md-img, .sbody .md-img", "background"}},
	"hover-bg": {
		{".system-nav-group__toggle:hover", "background"},
		{"#dxgOpen:hover, .sec-tab:hover", "background"},
		{".facet-toc__item:hover", "background"},
		{".claim-collapse-toggle:hover", "background"},
	},
	"border-strong": {
		{".system-record-head", "border-bottom"},
		{".track-head", "border-bottom"},
		{".build-order-module > .system-build-title", "border-bottom"},
	},
	"shadow":        {{".comments-panel", "box-shadow"}},
	"shadow-strong": {{".comments-toast", "box-shadow"}},
	"shadow-cast": {
		{".comments-rail", "box-shadow"},
		{".nav-toggle", "box-shadow"},
		{".facet-toc", "box-shadow"},
	},
	"scrim":           {{".comments-overlay", "background"}},
	"selection-bg":    {{"::selection", "background"}},
	"status-draft":    {{".pill.pv, .status-draft", "color"}},
	"status-draft-bg": {{".pill.pv, .status-draft", "background"}},
	"mockup-bg":       {{".mockup-diagram", "background"}},
}

// TestConsumerReadWinsCascade is the guard against the defect this branch
// actually shipped and had to fix: a token read placed on a declaration that
// something later overrides. Such a read passes
// TestEveryAllowlistedTokenHasAConsumer — the var() is right there in the file
// — and does nothing at all for a reader, which is exactly the failure mode
// "the token exists" checks are blind to.
//
// For each site it collects every declaration of that exact selector text and
// property, in source order, and requires the LAST one to carry the var().
//
// BOUND, stated because it matters: source order is the whole model here. It is
// exact when the declarations share a media context, which is the case for
// every site listed. Where they do not, the check is conservative in the safe
// direction — a later declaration inside a narrower @media would be flagged
// even though it does not always win — so it can be noisy but it cannot miss
// the class of bug it exists for: an unconditional later re-declaration of the
// same selector. It also assumes equal specificity, which identical selector
// text guarantees.
func TestConsumerReadWinsCascade(t *testing.T) {
	css := stripComments(embeddedCSS(t, styleTemplatePath))
	decls := parseDecls(css)
	if len(decls) == 0 {
		t.Fatal("parseDecls found no declarations in style.css; every assertion " +
			"below would pass over nothing")
	}

	// Vacuity guard on the table itself: the fourteen new tokens are exactly the
	// allowlist entries this map covers, so a token added to the allowlist
	// without a site here fails loudly rather than going unchecked.
	if got, want := len(tokenConsumers), 14; got != want {
		t.Errorf("tokenConsumers covers %d tokens, want %d", got, want)
	}
	for token := range tokenConsumers {
		found := false
		for _, allowed := range config.ThemeTokenAllowlist {
			if allowed == token {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tokenConsumers names %q, which is not in ThemeTokenAllowlist", token)
		}
	}

	for token, sites := range tokenConsumers {
		for _, site := range sites {
			var matches []cssDecl
			for _, d := range decls {
				if d.selector == site.selector && d.property == site.property {
					matches = append(matches, d)
				}
			}
			if len(matches) == 0 {
				t.Errorf("token %q: no `%s` declaration on selector %q found in "+
					"style.css. Either the rule was renamed or reformatted (the "+
					"selector text is compared literally, whitespace-collapsed), or "+
					"the consumer is gone — either way this token is now unchecked.",
					token, site.property, site.selector)
				continue
			}
			last := matches[len(matches)-1]
			want := "var(--" + token
			if !strings.Contains(last.text, want) {
				var others []string
				for _, m := range matches {
					others = append(others, fmt.Sprintf("byte %d: %s", m.offset, m.text))
				}
				t.Errorf("token %q: the LAST `%s` declaration on %q does not read the "+
					"token, so the read is dead and the token has no effect.\n"+
					"  winning declaration: byte %d: %s\n"+
					"  all %d declarations of this selector+property:\n    %s",
					token, site.property, site.selector, last.offset, last.text,
					len(matches), strings.Join(others, "\n    "))
			}
		}
	}

	t.Run("NegativeControlDetectsAShadowedRead", func(t *testing.T) {
		// Prove the check can fire, on synthetic input rather than by editing the
		// real sheet: a var() read followed by a plain re-declaration of the same
		// selector and property is exactly the V-W3-1 shape.
		synthetic := "code { background: var(--code-inline-bg, red); }\n" +
			"code { background: blue; }\n"
		var matches []cssDecl
		for _, d := range parseDecls(synthetic) {
			if d.selector == "code" && d.property == "background" {
				matches = append(matches, d)
			}
		}
		if len(matches) != 2 {
			t.Fatalf("parseDecls found %d `code`/background declarations in the "+
				"synthetic input, want 2; the control proves nothing", len(matches))
		}
		if strings.Contains(matches[len(matches)-1].text, "var(--code-inline-bg") {
			t.Fatal("the last declaration in the synthetic input carries the var(); " +
				"the shadowing control cannot fire")
		}
		if !strings.Contains(matches[0].text, "var(--code-inline-bg") {
			t.Fatal("the first declaration in the synthetic input does not carry the " +
				"var(); the control is not modelling the defect")
		}
	})
}
