// skill_theme_test.go EXECUTES skills/dossierx-theme/SKILL.md rather than
// reading it.
//
// The bundle is the client-facing document an agent loads to restyle somebody
// else's viewer, and every concrete thing in it is a claim about a binary the
// agent will actually run: this YAML loads, this token exists, this command is
// real, this procedure works in this order. A skill whose examples do not load
// is worse than no skill, because an agent trusts it and reports the failure as
// the project's.
//
// So each of those claim families is run against the built binary:
//
//	every fenced yaml block   spliced into a lint-clean skeleton, then rendered
//	every token in the table  checked against the engine's own vocabulary, read
//	                          out of "dossierx theme list" rather than restated
//	every "dossierx ..." it   checked against surface.json's command paths
//	names
//	its numbered procedure    replayed step by step against a real themed project
//
// THE SKELETON IS ITS OWN ASSERTION. Every splice test needs a project that
// passes `check --validate` BEFORE the block goes in, or a failure afterwards
// says nothing about the block. TestThemeSkillSkeletonIsCleanBeforeAnySplice
// pins that separately so the precondition cannot decay into the thing being
// measured.
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const themeSkillPath = "skills/dossierx-theme/SKILL.md"

// themeSkillSource reads the bundle. A missing file is fatal rather than a
// skipped test: this file's entire subject would be gone, and a suite that
// reports "ok" over zero assertions is indistinguishable from one that checked
// something.
func themeSkillSource(t *testing.T) string {
	t.Helper()
	return readRepoFile(t, filepath.FromSlash(themeSkillPath))
}

// ---------------------------------------------------------------------
// the skeleton, and splicing a block into it
// ---------------------------------------------------------------------

// newThemeSkillSkeleton builds the minimal lint-clean project every splice test
// starts from: one draft claim, no theme.
func newThemeSkillSkeleton(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureProject(t, root, "themeskill")
	return root
}

func TestThemeSkillSkeletonIsCleanBeforeAnySplice(t *testing.T) {
	root := newThemeSkillSkeleton(t)
	stdout, stderr, code := run(t, root, "check", "--validate")
	if code != 0 {
		t.Fatalf("the un-themed skeleton must exit 0 under check --validate, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	// And it must RENDER, because the splice tests compare a themed render
	// against this one.
	if _, stderr, code = run(t, root, "check"); code != 0 {
		t.Fatalf("the un-themed skeleton must render, got exit %d\nstderr:\n%s", code, stderr)
	}
}

// fencedYAMLBlocks returns every ```yaml block in the document, in order.
var fenceOpen = regexp.MustCompile("(?m)^```yaml\n")

func fencedYAMLBlocks(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	rest := src
	for {
		loc := fenceOpen.FindStringIndex(rest)
		if loc == nil {
			break
		}
		body := rest[loc[1]:]
		end := strings.Index(body, "\n```")
		if end < 0 {
			t.Fatalf("%s has an unterminated ```yaml fence; the document a client reads is malformed", themeSkillPath)
		}
		out = append(out, body[:end+1])
		rest = body[end+1:]
	}
	return out
}

var (
	themeSrcLine     = regexp.MustCompile(`(?m)^\s*src:\s*(\S+)\s*$`)
	themeExtendsLine = regexp.MustCompile(`(?m)^\s*extends:\s*(\S+)\s*$`)
)

// TestThemeSkillYAMLBlocksLoadAndTheme splices each fenced block into the
// skeleton's project.config.yaml and requires two things of it: `check` exits 0,
// and the viewer it produces DIFFERS from the un-themed one.
//
// The difference is the assertion that matters, and "it exited 0" alone would
// not be it. A block whose keys were quietly dropped — the failure mode a
// silently-ignoring decoder would produce, and the one this vocabulary's
// typo-protection exists to prevent — exits 0 and renders the default viewer.
// Comparing the rendered bytes is the only way to say the theme was APPLIED
// rather than merely accepted.
//
// Two substitutions make the document's own examples runnable, and neither
// weakens what is being tested: a block naming `extends: <path>` gets that file
// materialized by `dossierx theme export claude <path>` — the exact command the
// skill tells the reader to run one paragraph earlier — and a block naming a
// font `src:` gets a real file with the right signature written there, because
// the skill's point is the DECLARATION and shipping a woff2 in testdata to
// prove it would test the fixture.
func TestThemeSkillYAMLBlocksLoadAndTheme(t *testing.T) {
	blocks := fencedYAMLBlocks(t, themeSkillSource(t))
	if len(blocks) < 3 {
		t.Fatalf("%s has %d fenced yaml block(s); the acceptance floor is 3, and a document with fewer has stopped showing the reader the shapes it describes", themeSkillPath, len(blocks))
	}

	for i, block := range blocks {
		t.Run(strings.Fields(block)[0]+"-"+itoa(i), func(t *testing.T) {
			root := newThemeSkillSkeleton(t)

			// Baseline: the same project, no theme.
			if _, stderr, code := run(t, root, "check"); code != 0 {
				t.Fatalf("baseline render failed: exit %d\n%s", code, stderr)
			}
			baseline := readFileOrFail(t, filepath.Join(root, "viewer", "index.html"))

			for _, m := range themeExtendsLine.FindAllStringSubmatch(block, -1) {
				dest := filepath.Join(root, filepath.FromSlash(m[1]))
				if _, stderr, code := run(t, root, "theme", "export", "claude", m[1]); code != 0 {
					t.Fatalf("the skill's own export command failed for %s: exit %d\n%s", m[1], code, stderr)
				}
				if _, err := os.Stat(dest); err != nil {
					t.Fatalf("theme export reported success but wrote nothing at %s: %v", dest, err)
				}
			}
			for _, m := range themeSrcLine.FindAllStringSubmatch(block, -1) {
				writeFontFile(t, filepath.Join(root, filepath.FromSlash(m[1])))
			}

			cfgPath := filepath.Join(root, "project.config.yaml")
			base := readFileOrFail(t, cfgPath)
			if err := os.WriteFile(cfgPath, []byte(base+block), 0o644); err != nil {
				t.Fatalf("splice block %d: %v", i, err)
			}

			if stdout, stderr, code := run(t, root, "check", "--validate"); code != 0 {
				t.Fatalf("block %d does not validate:\n%s\nexit %d\nstdout:\n%s\nstderr:\n%s", i, block, code, stdout, stderr)
			}
			if _, stderr, code := run(t, root, "check"); code != 0 {
				t.Fatalf("block %d does not render:\n%s\nexit %d\nstderr:\n%s", i, block, code, stderr)
			}
			themed := readFileOrFail(t, filepath.Join(root, "viewer", "index.html"))
			if themed == baseline {
				t.Fatalf("block %d loaded but changed nothing in the rendered viewer, so the skill is showing a reader a theme that does nothing:\n%s", i, block)
			}
		})
	}
}

// writeFontFile writes a file whose first four bytes are the signature the
// engine requires for its extension. Anything else is refused by design (a
// browser handed a mislabelled face drops it silently), so a placeholder with
// the wrong magic would fail the splice for a reason that has nothing to do
// with the skill.
func writeFontFile(t *testing.T, path string) {
	t.Helper()
	sig := map[string]string{
		".woff2": "wOF2",
		".woff":  "wOFF",
		".otf":   "OTTO",
		".ttf":   "\x00\x01\x00\x00",
	}[strings.ToLower(filepath.Ext(path))]
	if sig == "" {
		t.Fatalf("%s: the skill names a font extension this test does not know how to fabricate; either the skill added one or the engine's accepted set moved", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(sig+"-fabricated-for-the-skill-acceptance-test"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ---------------------------------------------------------------------
// the token table
// ---------------------------------------------------------------------

// engineThemeTokens reads the engine's whole token vocabulary out of "dossierx
// theme list".
//
// Through the BINARY rather than by importing internal/config, for this
// package's standing reason (it reads what a client reads) and for a sharper
// one here: the vocabulary an agent can discover is the vocabulary the skill
// has to agree with, and those are the same list only because `theme list`
// reports it. The claude preset covers the whole allowlist —
// internal/config/theme_test.go pins that — so its token set IS the vocabulary.
func engineThemeTokens(t *testing.T) map[string]bool {
	t.Helper()
	root := t.TempDir()
	stdout, stderr, code := run(t, root, "--format", "json", "theme", "list")
	if code != 0 {
		t.Fatalf("theme list: exit %d\n%s", code, stderr)
	}
	var env struct {
		Data struct {
			Presets []struct {
				Name   string   `json:"name"`
				Tokens []string `json:"tokens"`
			} `json:"presets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("theme list envelope: %v\n%s", err, stdout)
	}
	out := map[string]bool{}
	for _, p := range env.Data.Presets {
		if p.Name != "claude" {
			continue
		}
		for _, tok := range p.Tokens {
			out[tok] = true
		}
	}
	if len(out) == 0 {
		t.Fatal("theme list reported no tokens for the claude preset; this test's source of truth is empty and every comparison below would pass vacuously")
	}
	return out
}

// themeSkillTableTokens returns the first column of the skill's TOKEN table,
// located by its header row.
//
// Scoped to that one table on purpose: the bundle also carries an error-code
// table whose first column is `code`-shaped backticked identifiers, and a
// whole-document scan could not tell "this is a CSS token" from "this is an
// error code" — it would report every code as a bogus token and every token as
// a bogus code.
func themeSkillTableTokens(t *testing.T, src string) []string {
	t.Helper()
	const header = "| token | light default | dark default | what it paints |"
	i := strings.Index(src, header)
	if i < 0 {
		t.Fatalf("%s no longer carries the token table header %q; the table this test checks has moved or gone, and the test must be repointed rather than quietly finding nothing", themeSkillPath, header)
	}
	body := src[i+len(header):]
	if j := strings.Index(body, "\n\n"); j >= 0 {
		body = body[:j]
	}
	var out []string
	for _, r := range parseThemeTableRows(body) {
		out = append(out, r.token)
	}
	return out
}

// themeTableRow is one row of the skill's token table. The dark cell has three
// legal forms and they are three different CLAIMS about the stylesheet, not one
// claim and two ways of writing it:
//
//	`value`        this token is re-declared in the screen-dark :root
//	*(same)*       it is not, and its value is a literal, so it really is the
//	               same colour in both modes — flat is safe
//	*(derived)*    it is not re-declared either, but its value is an expression
//	               over OTHER tokens (color-mix over paper/card-bg), so its
//	               computed result still varies with the mode — flat is a trap
//
// The third form exists because collapsing it into *(same)* is a wrong answer
// to the question the column is asked: a reader scanning for "which of these
// can I set flat?" would be told yes for two tokens where flat freezes an
// expression that had been tracking the scheme.
type themeTableRow struct {
	token       string
	light       string
	dark        string
	sameAsLight bool
	derived     bool
}

// themeTableRowPattern reads "| `token` | `light` | <dark cell> | prose |".
var themeTableRowPattern = regexp.MustCompile("(?m)^\\| `([^`]+)` \\| `([^`]+)` \\| (`[^`]+`|\\*\\(same\\)\\*|\\*\\(derived\\)\\*) \\|")

func parseThemeTableRows(body string) []themeTableRow {
	var out []themeTableRow
	for _, m := range themeTableRowPattern.FindAllStringSubmatch(body, -1) {
		row := themeTableRow{token: m[1], light: m[2]}
		switch m[3] {
		case "*(same)*":
			row.sameAsLight = true
		case "*(derived)*":
			row.sameAsLight, row.derived = true, true
		default:
			row.dark = strings.Trim(m[3], "`")
		}
		out = append(out, row)
	}
	return out
}

// themeSkillTableBody returns the token table's rows as raw text.
func themeSkillTableBody(t *testing.T, src string) string {
	t.Helper()
	const header = "| token | light default | dark default | what it paints |"
	i := strings.Index(src, header)
	if i < 0 {
		t.Fatalf("%s no longer carries the token table header %q; the table this test checks has moved or gone", themeSkillPath, header)
	}
	body := src[i+len(header):]
	if j := strings.Index(body, "\n\n"); j >= 0 {
		body = body[:j]
	}
	return body
}

// cssTokenBlocks parses the engine stylesheet's unconditional :root and its
// screen-dark :root into token -> declared value.
//
// It reads internal/render/viewer/template/style.css DIRECTLY, and that is the
// point: the skill's table is a transcription of those two blocks, and the only
// way a transcription can be checked is against the thing transcribed. The
// failure this exists to catch already happened once — the table carried the
// rgba() literals from a plan while the live declarations had become
// color-mix() expressions, so a reader was told the wrong default for the two
// tokens most likely to be overridden first.
//
// --dxg-* is skipped: the graph ramp is not part of the theme vocabulary (a
// theme cannot repoint it, which the skill says in as many words), so a table
// that documented it would be documenting a token nobody can set.
func cssTokenBlocks(t *testing.T) (root, dark map[string]string) {
	t.Helper()
	css := readRepoFile(t, filepath.Join("internal", "render", "viewer", "template", "style.css"))

	rootBlock := captureBlock(t, css, "\n:root {", "\n}")
	darkOuter := captureBlock(t, css, "\n@media screen and (prefers-color-scheme: dark) {", "\n}")
	darkBlock := captureBlock(t, darkOuter, "  :root {", "\n  }")

	decl := regexp.MustCompile(`(?m)^\s*--([a-z0-9-]+):\s*(.+?);\s*$`)
	parse := func(block string) map[string]string {
		out := map[string]string{}
		for _, m := range decl.FindAllStringSubmatch(block, -1) {
			if strings.HasPrefix(m[1], "dxg-") {
				continue
			}
			out[m[1]] = strings.TrimSpace(m[2])
		}
		return out
	}
	return parse(rootBlock), parse(darkBlock)
}

// captureBlock returns the text between the first open marker and the first
// close marker after it. A marker that is not found is fatal: the stylesheet
// has been restructured, and a parser that silently returned nothing would make
// every comparison below pass over an empty map.
func captureBlock(t *testing.T, src, openMarker, closeMarker string) string {
	t.Helper()
	i := strings.Index(src, openMarker)
	if i < 0 {
		t.Fatalf("style.css no longer contains %q; the block this test parses has moved, and the test must be repointed rather than quietly matching nothing", openMarker)
	}
	rest := src[i+len(openMarker):]
	j := strings.Index(rest, closeMarker)
	if j < 0 {
		t.Fatalf("style.css: no %q closing %q", closeMarker, openMarker)
	}
	return rest[:j]
}

// normalizeCSSValue collapses runs of whitespace, so `rgba(40,112,82,.12)` and
// `rgba(40, 112, 82, .12)` compare equal.
//
// Whitespace INSIDE a CSS function is insignificant to every engine that will
// read it, so a reader who types either spelling gets the same colour, and
// failing over a respacing would be a false alarm. Nothing else is normalized:
// no case folding, no hex expansion, no alpha rounding — those would let a
// genuinely different value pass, which is the whole thing being checked.
func normalizeCSSValue(v string) string {
	return strings.Join(strings.Fields(v), " ")
}

func TestThemeSkillTokenTableMatchesTheEngineVocabulary(t *testing.T) {
	src := themeSkillSource(t)
	engine := engineThemeTokens(t)
	listed := themeSkillTableTokens(t, src)

	seen := map[string]bool{}
	for _, tok := range listed {
		if !engine[tok] {
			t.Errorf("%s documents token %q, which the engine's vocabulary does not carry; a reader who writes it gets a load error naming every real token", themeSkillPath, tok)
		}
		if seen[tok] {
			t.Errorf("%s lists token %q twice", themeSkillPath, tok)
		}
		seen[tok] = true
	}
	// BOTH directions. A table missing a token is the failure that does not
	// announce itself: the reader simply never learns the token exists, and
	// there is nothing in the document to notice.
	for tok := range engine {
		if !seen[tok] {
			t.Errorf("the engine carries token %q and %s does not document it; a token nobody is told about is a token nobody uses", tok, themeSkillPath)
		}
	}

	// ---- the DEFAULT VALUES, against the stylesheet they are copied from ----
	//
	// A token name that exists is the cheap half. The expensive half is the
	// value beside it, because a reader takes the light/dark columns as the
	// starting point they are diverging FROM: a wrong default sends them to
	// override a token that already had the colour they wanted, or to leave one
	// alone that did not.
	root, darkBlock := cssTokenBlocks(t)
	rows := parseThemeTableRows(themeSkillTableBody(t, src))

	// Vacuity guards, both directions. Either number arriving at zero — a
	// changed table format, a restructured stylesheet — would make every
	// assertion in this section pass over nothing.
	if len(rows) != len(engine) {
		t.Fatalf("parsed %d table row(s) from %s but the engine carries %d token(s); the row parser and the table have diverged, and every value comparison below would be over a partial set", len(rows), themeSkillPath, len(engine))
	}
	darkRepoints := 0
	for tok := range darkBlock {
		if engine[tok] {
			darkRepoints++
		}
	}
	if darkRepoints == 0 {
		t.Fatal("style.css's screen-dark :root re-declares no theme token at all; the dark column of the skill's table would be unfalsifiable")
	}
	tableDarkRepoints := 0
	for _, r := range rows {
		if !r.sameAsLight {
			tableDarkRepoints++
		}
	}
	if tableDarkRepoints == 0 {
		t.Fatal("no table row claims a separate dark default; the row parser and the table have diverged")
	}
	if tableDarkRepoints != darkRepoints {
		t.Errorf("%s marks %d row(s) as differing in dark mode; style.css re-declares %d theme token(s) in its screen-dark :root", themeSkillPath, tableDarkRepoints, darkRepoints)
	}

	for _, r := range rows {
		declared, ok := root[r.token]
		if !ok {
			t.Errorf("%s documents a default for %q, which style.css's :root does not declare", themeSkillPath, r.token)
			continue
		}
		if normalizeCSSValue(r.light) != normalizeCSSValue(declared) {
			t.Errorf("%s says %q defaults to %q; style.css's :root declares %q — a reader is being told the wrong starting point for the token they are about to override",
				themeSkillPath, r.token, r.light, declared)
		}
		darkDeclared, repointed := darkBlock[r.token]
		switch {
		case r.sameAsLight && repointed:
			t.Errorf("%s marks %q as *(same)* in dark mode, but style.css re-declares it as %q; a reader setting it flat would be told they are safe when they are pinning a value the engine varies",
				themeSkillPath, r.token, darkDeclared)
		case !r.sameAsLight && !repointed:
			t.Errorf("%s gives %q a separate dark default (%q) that style.css does not declare", themeSkillPath, r.token, r.dark)
		case !r.sameAsLight && normalizeCSSValue(r.dark) != normalizeCSSValue(darkDeclared):
			t.Errorf("%s says %q is %q in dark mode; style.css declares %q", themeSkillPath, r.token, r.dark, darkDeclared)
		}

		// *(derived)* and *(same)* both mean "not re-declared in dark", so the
		// switch above cannot tell them apart — and the whole point of the
		// third form is the distinction. It is decided by the LIGHT default:
		// a value containing var() is an expression over other tokens and its
		// computed result follows them, where a literal does not. Checked in
		// both directions, because either mistake tells a reader the wrong
		// thing about setting the token flat.
		expression := strings.Contains(declared, "var(--")
		switch {
		case expression && !r.derived:
			t.Errorf("%s marks %q as *(same)*, but style.css declares it as the expression %q — its computed value follows the tokens it names, so a reader told it is mode-invariant would freeze it by setting it flat",
				themeSkillPath, r.token, declared)
		case !expression && r.derived:
			t.Errorf("%s marks %q as *(derived)*, but style.css declares it as the literal %q, which derives from nothing",
				themeSkillPath, r.token, declared)
		}
	}

	// A vacuity guard for the arm above: the *(derived)* form must actually be
	// exercised. If the stylesheet stopped using expressions the form should be
	// retired from the document rather than left as a rule nothing tests.
	derivedRows := 0
	for _, r := range rows {
		if r.derived {
			derivedRows++
		}
	}
	expressionDefaults := 0
	for tok, v := range root {
		if engine[tok] && strings.Contains(v, "var(--") {
			expressionDefaults++
		}
	}
	if derivedRows != expressionDefaults {
		t.Errorf("%s marks %d row(s) *(derived)*; style.css declares %d theme token(s) as expressions over other tokens", themeSkillPath, derivedRows, expressionDefaults)
	}
	if expressionDefaults == 0 && derivedRows == 0 {
		t.Log("no theme token is declared as an expression any more; the *(derived)* column form is now untested and should be removed from the skill")
	}

	// The prose count has to agree with the table it introduces, or the
	// document contradicts itself in the one place a reader would trust.
	if got, want := len(listed), len(engine); got == want {
		if !strings.Contains(strings.ToLower(src), numberWord(t, want)+"-token") &&
			!strings.Contains(strings.ToLower(src), numberWord(t, want)+",") {
			t.Errorf("%s documents %d tokens but never says %q in prose", themeSkillPath, want, numberWord(t, want))
		}
	}
}

// TestThemeSkillTokenExtractionCanFail is the negative control for the check
// above: a synthetic token planted in a copy of the table must be reported.
//
// Without it, an extractor that silently matched nothing — a changed table
// format, a tightened regexp — would make every assertion above pass over an
// empty list, which is the shape of "checked nothing" that reads as "fine".
func TestThemeSkillTokenExtractionCanFail(t *testing.T) {
	src := themeSkillSource(t)
	const planted = "not-a-real-token"
	engine := engineThemeTokens(t)
	if engine[planted] {
		t.Fatalf("%q is a real token; pick a name the engine does not carry", planted)
	}

	const header = "| token | light default | dark default | what it paints |"
	i := strings.Index(src, header)
	if i < 0 {
		t.Fatalf("%s: token table header is gone", themeSkillPath)
	}
	tampered := src[:i+len(header)] + "\n| `" + planted + "` | `#000` | *(same)* | nothing |" + src[i+len(header):]

	found := false
	for _, tok := range themeSkillTableTokens(t, tampered) {
		if tok == planted {
			found = true
		}
	}
	if !found {
		t.Fatalf("the token extractor did not see a planted %q row, so its silence over the real table proves nothing", planted)
	}
}

// numberWord spells a small count, so the prose check above compares against a
// derived word rather than a second hardcoded number.
func numberWord(t *testing.T, n int) string {
	t.Helper()
	words := map[int]string{
		20: "twenty", 21: "twenty-one", 22: "twenty-two", 23: "twenty-three",
		24: "twenty-four", 25: "twenty-five", 26: "twenty-six", 27: "twenty-seven",
		28: "twenty-eight", 29: "twenty-nine", 30: "thirty", 31: "thirty-one",
		32: "thirty-two",
	}
	w, ok := words[n]
	if !ok {
		t.Fatalf("no spelling for %d; extend the table rather than dropping the prose check", n)
	}
	return w
}

// ---------------------------------------------------------------------
// the commands it names
// ---------------------------------------------------------------------

var dossierxInvocation = regexp.MustCompile(`dossierx ([a-z][a-z-]*)(?: ([a-z][a-z-]*))?`)

// TestThemeSkillNamesOnlyRealCommands checks every "dossierx ..." in the bundle
// against surface.json's command paths.
//
// A skill that names a command the binary does not have sends an agent into a
// `usage` failure it cannot recover from by reading the skill again, which is
// the loop the whole error-code contract exists to prevent.
func TestThemeSkillNamesOnlyRealCommands(t *testing.T) {
	var surface struct {
		Commands []struct {
			Path string `json:"path"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(readRepoFile(t, "surface.json")), &surface); err != nil {
		t.Fatalf("surface.json: %v", err)
	}
	known := map[string]bool{}
	nouns := map[string]bool{}
	for _, c := range surface.Commands {
		known[c.Path] = true
		nouns[strings.SplitN(c.Path, " ", 2)[0]] = true
	}
	if len(known) == 0 {
		t.Fatal("surface.json lists no commands; this test's source of truth is empty")
	}

	src := themeSkillSource(t)
	checked := 0
	for _, m := range dossierxInvocation.FindAllStringSubmatch(src, -1) {
		noun, leaf := m[1], m[2]
		if !nouns[noun] {
			t.Errorf("%s names `dossierx %s`, which is not a noun in surface.json", themeSkillPath, noun)
			continue
		}
		checked++
		if known[noun] {
			// A noun that IS a leaf (check, serve, version); the second word is
			// prose, not a subcommand.
			continue
		}
		if leaf == "" {
			// A bare command group, named as a group ("the dossierx theme
			// commands"). Nothing to resolve.
			continue
		}
		if !known[noun+" "+leaf] {
			t.Errorf("%s names `dossierx %s %s`, which surface.json does not carry", themeSkillPath, noun, leaf)
		}
	}
	if checked == 0 {
		t.Fatalf("%s names no dossierx command at all; either the document stopped telling the reader what to run, or this extractor stopped seeing it", themeSkillPath)
	}
}

// ---------------------------------------------------------------------
// the numbered verification procedure
// ---------------------------------------------------------------------

// TestThemeSkillVerificationProcedureReplays runs the bundle's numbered
// verification steps, in order, against a project themed the way the bundle
// says to theme one.
//
// It is a DEDICATED replay rather than a reuse of skill_bootstrap_replay_test's
// interpreter, and the reason is a shape difference and not a preference: that
// interpreter executes `sh`/fetch instructions out of a step's prose, and every
// step here is either a command with a POSTCONDITION to assert (step 1, step 4)
// or an instruction addressed to a human's eyes in a browser (steps 2 and 3),
// which no interpreter can execute and which this file must therefore say
// plainly it does not. The discipline is the same one: the step text is read
// out of the document, so a step that is renamed, renumbered or deleted takes
// its assertion with it and fails loudly rather than passing over nothing.
//
// WHAT IS NOT REPLAYED, stated where a reader will see it: steps 2 and 3 open
// the rendered viewer in a browser and compare it against the human's intent in
// each OS colour scheme. That is a browser-level check and it lives in
// viewer-tests, not here. This test asserts what a Go test can: the artifact
// step 2 tells the reader to open EXISTS and carries the value the theme set,
// and the token step 3 is about is genuinely mode-varying in the render.
func TestThemeSkillVerificationProcedureReplays(t *testing.T) {
	src := themeSkillSource(t)
	section := themeSkillSection(t, src, "## Verifying a theme actually applied")
	steps := map[int]string{}
	for n := 1; n <= 4; n++ {
		steps[n] = pasteBlockStep(t, section, n)
	}

	root := newThemeSkillSkeleton(t)
	writeFontFile(t, filepath.Join(root, "fonts", "probe.woff2"))
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(readFileOrFail(t, cfgPath)+strings.Join([]string{
		"viewer:",
		"  theme:",
		"    font-sans: '\"Probe Face\", sans-serif'",
		"    light:",
		"      paper: '#fafafa'",
		"    dark:",
		"      paper: '#101010'",
		"    fonts:",
		"      - family: Probe Face",
		"        src: fonts/probe.woff2",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatalf("write themed config: %v", err)
	}

	// Step 1 — "Run `dossierx check`."
	requireStepMentions(t, steps[1], "dossierx check")
	if _, stderr, code := run(t, root, "check"); code != 0 {
		t.Fatalf("step 1: the skill's own theme shape does not pass check: exit %d\n%s", code, stderr)
	}

	// Step 2 — "Open the rendered viewer/index.html". Not opened; asserted to
	// exist and to carry what the theme set.
	requireStepMentions(t, steps[2], "viewer/index.html")
	viewer := readFileOrFail(t, filepath.Join(root, "viewer", "index.html"))
	if !strings.Contains(viewer, "#fafafa") {
		t.Error("step 2: the rendered viewer does not carry the light paper value the theme set, so the artifact the reader is told to open would not show them their change")
	}

	// Step 3 — "Check both OS colour schemes." Not switched; asserted that the
	// render really does distinguish them, which is the property the step is
	// asking the reader to look for.
	requireStepMentions(t, steps[3], "both")
	if !strings.Contains(viewer, "#101010") {
		t.Error("step 3: the dark value never reaches the render, so a reader switching schemes would see no difference and conclude the theme failed")
	}

	// Step 4 — "Read data.theme_font_count and data.theme_font_bytes".
	requireStepMentions(t, steps[4], "theme_font_bytes")
	stdout, stderr, code := run(t, root, "--format", "json", "check")
	if code != 0 {
		t.Fatalf("step 4: check: exit %d\n%s", code, stderr)
	}
	var env struct {
		Data struct {
			FontCount int   `json:"theme_font_count"`
			FontBytes int64 `json:"theme_font_bytes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("step 4: envelope: %v\n%s", err, stdout)
	}
	if env.Data.FontCount != 1 {
		t.Errorf("step 4: data.theme_font_count = %d, want 1 — the field the skill tells a reader to read does not report the face they declared", env.Data.FontCount)
	}
	if env.Data.FontBytes <= 0 {
		t.Errorf("step 4: data.theme_font_bytes = %d, want the face's size — the number the skill tells a reader to say out loud is not there", env.Data.FontBytes)
	}
}

// requireStepMentions fails when a replayed step no longer says the thing its
// assertion was written against. Without it, editing a step's text would leave
// the assertion below it running against a procedure that no longer asks for it
// — a test that still passes and no longer means anything.
func requireStepMentions(t *testing.T, step, want string) {
	t.Helper()
	if !strings.Contains(step, want) {
		t.Fatalf("a verification step no longer mentions %q; the assertion written for it is now checking something the document does not ask for:\n%s", want, step)
	}
}

// themeSkillSection returns one "## ..." section of the bundle.
func themeSkillSection(t *testing.T, src, heading string) string {
	t.Helper()
	i := strings.Index(src, "\n"+heading)
	if i < 0 {
		t.Fatalf("%s no longer carries the %q heading; the procedure this test replays has moved or gone", themeSkillPath, heading)
	}
	body := src[i+1:]
	if j := strings.Index(body, "\n## "); j >= 0 {
		body = body[:j]
	}
	return body
}

// ---------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------

func readFileOrFail(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
