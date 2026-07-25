package markdown

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// --- test helpers -----------------------------------------------------

// markdownCasesDir resolves testdata/markdown-cases relative to this test
// file's own location (via runtime.Caller), so the test works regardless
// of the directory `go test` is invoked from. The corpus is a small,
// self-authored set of claim fixtures that lives inside this module (see
// the package doc comment on fencedClaims below for why), so — unlike an
// external project's real, independently-evolving claims — it has zero
// dependency on any path outside this repository.
func markdownCasesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// markdown_test.go is at
	// internal/render/markdown/markdown_test.go; the corpus lives at
	// testdata/markdown-cases, three directories up.
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "markdown-cases")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("markdown-cases dir not found at %q: %v", dir, err)
	}
	return dir
}

type claimYAML struct {
	ID     string `yaml:"id"`
	Layout string `yaml:"layout"`
	Body   string `yaml:"body"`
}

func loadClaim(t *testing.T, dir, rel string) claimYAML {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var c claimYAML
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("unmarshal %s: %v", rel, err)
	}
	return c
}

// countBalanced fails the test unless every opening tag has a matching
// closing tag count, for a fixed set of tags this package ever emits.
func assertTagBalance(t *testing.T, out string) {
	t.Helper()
	for _, tag := range []string{"p", "pre", "code", "ul", "ol", "li"} {
		openCount := strings.Count(out, "<"+tag+">")
		closeCount := strings.Count(out, "</"+tag+">")
		if openCount != closeCount {
			t.Errorf("tag balance: <%s> open=%d close=%d in output:\n%s", tag, openCount, closeCount, out)
		}
	}
}

// --- the fenced-claim corpus --------------------------------------------
//
// This is a small, self-authored corpus under testdata/markdown-cases (see
// TestFencedClaimsListIsComplete), not a live snapshot of any external
// project's claims — it exists purely to exercise this package's
// fence-handling behavior: a plain fence, a fence with a dropped info
// string, multiple fences in one body, and HTML-injection content inside a
// fence (script tags, ampersands, quotes) all surviving as verbatim,
// escaped content.
var fencedClaims = []string{
	"fenced-basic.yaml",
	"fenced-with-info-string.yaml",
	"fenced-multi-block.yaml",
	"fenced-html-injection.yaml",
}

func TestRender_AllFencedClaims(t *testing.T) {
	dir := markdownCasesDir(t)
	if len(fencedClaims) == 0 {
		t.Fatal("fencedClaims is empty — nothing to test")
	}

	for _, rel := range fencedClaims {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			c := loadClaim(t, dir, rel)
			if c.Body == "" {
				t.Fatalf("%s: empty body", rel)
			}

			// Expected fence count/content computed via the same
			// ported bodyFence regex this package uses — this is
			// intentionally the same regex, not a reimplementation,
			// since the test is verifying Render's use of it, not
			// second-guessing the regex itself.
			matches := bodyFence.FindAllStringSubmatchIndex(c.Body, -1)
			if len(matches) == 0 {
				t.Fatalf("%s: expected at least one fence, found none", rel)
			}

			out := string(Render(c.Body))
			assertTagBalance(t, out)

			gotPre := strings.Count(out, "<pre><code>")
			gotClose := strings.Count(out, "</code></pre>")
			if gotPre != len(matches) || gotClose != len(matches) {
				t.Errorf("%s: fence count mismatch: source=%d rendered open=%d close=%d", rel, len(matches), gotPre, gotClose)
			}

			for i, loc := range matches {
				codeStart, codeEnd := loc[2], loc[3]
				want := c.Body[codeStart:codeEnd]
				wantEscaped := escapeForTest(want)
				if !strings.Contains(out, wantEscaped) {
					t.Errorf("%s: fence #%d escaped content not found verbatim in output.\nwant substring:\n%s\ngot output:\n%s", rel, i, wantEscaped, out)
				}
			}
		})
	}
}

// --- fixed synthetic cases ---------------------------------------------

func TestRender_InlineBacktickCode(t *testing.T) {
	body := "The API is `get_client(module: str) -> WidgetLogger` and returns a client."
	out := string(Render(body))
	assertTagBalance(t, out)
	want := "<code>get_client(module: str) -&gt; WidgetLogger</code>"
	if !strings.Contains(out, want) {
		t.Errorf("inline code span not rendered.\nwant substring: %s\ngot: %s", want, out)
	}
	if strings.Contains(out, "`get_client") {
		t.Errorf("literal backtick leaked into output: %s", out)
	}
}

// --- inline links (DX-AUD-03) ------------------------------------------
//
// RenderInline is the inline pass in isolation (no <p>/list block wrapping),
// so these assert its exact output byte-for-byte: link grammar, the scheme
// allowlist's neutralization of dangerous schemes, attribute/text escaping,
// and composition with backtick code spans in a single left-to-right scan.

func TestRenderInline_Links(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "http",
			in:   "see [docs](http://example.com/a) now",
			want: `see <a href="http://example.com/a">docs</a> now`,
		},
		{
			name: "https",
			in:   "[Home](https://a.b/c)",
			want: `<a href="https://a.b/c">Home</a>`,
		},
		{
			name: "mailto",
			in:   "[mail](mailto:a@b.com)",
			want: `<a href="mailto:a@b.com">mail</a>`,
		},
		{
			name: "relative path resolves",
			in:   "[up](../widget/run.go)",
			want: `<a href="../widget/run.go">up</a>`,
		},
		{
			name: "fragment resolves",
			in:   "[sec](#widget.contract.x)",
			want: `<a href="#widget.contract.x">sec</a>`,
		},
		{
			name: "url and text html-escaped",
			in:   `[<b> & "q"](http://x/?a=1&b=2)`,
			want: `<a href="http://x/?a=1&amp;b=2">&lt;b&gt; &amp; &#34;q&#34;</a>`,
		},
		{
			name: "javascript scheme neutralized",
			in:   "[click](javascript:alert(1))",
			want: `[click](javascript:alert(1))`,
		},
		{
			name: "data scheme neutralized",
			in:   "[x](data:text/html,<script>)",
			want: `[x](data:text/html,&lt;script&gt;)`,
		},
		{
			name: "vbscript scheme neutralized",
			in:   "[x](vbscript:msgbox(1))",
			want: `[x](vbscript:msgbox(1))`,
		},
		{
			name: "javascript with leading space neutralized",
			in:   "[x]( javascript:alert(1))",
			want: `[x]( javascript:alert(1))`,
		},
		{
			name: "javascript with embedded tab neutralized",
			in:   "[x](java\tscript:alert(1))",
			want: "[x](java\tscript:alert(1))",
		},
		{
			// A scheme-less "//host" is a protocol-relative / network-path
			// reference: it resolves against the PAGE's scheme to an arbitrary
			// off-origin host, outside the documented relative-path/#fragment
			// scope. It must be neutralized, not emitted as a live anchor.
			name: "protocol-relative network-path neutralized",
			in:   "[x](//evil.example)",
			want: "[x](//evil.example)",
		},
		{
			name: "protocol-relative with path neutralized",
			in:   "see [x](//evil.example/a) end",
			want: "see [x](//evil.example/a) end",
		},
		{
			// Browsers normalize "\" to "/" in a URL's authority under a special
			// (http/https) scheme, so "/\host" is just as off-origin as "//host".
			name: "backslash network-path neutralized",
			in:   `[x](/\evil.example)`,
			want: `[x](/\evil.example)`,
		},
		{
			name: "network-path with leading space neutralized",
			in:   "[x]( //evil.example)",
			want: "[x]( //evil.example)",
		},
		{
			// A single leading slash is a root-relative (same-origin) path, not a
			// network-path — it stays a live anchor (the rejection is scoped to
			// "//host", it must not over-reject ordinary relative references).
			name: "single-slash root-relative still resolves",
			in:   "[x](/local/path)",
			want: `<a href="/local/path">x</a>`,
		},
		{
			name: "unclosed link no paren falls through",
			in:   "[text](http://x",
			want: "[text](http://x",
		},
		{
			name: "bracket without paren falls through",
			in:   "see [text] here",
			want: "see [text] here",
		},
		{
			name: "link inside code span stays literal",
			in:   "`[a](http://evil)`",
			want: "<code>[a](http://evil)</code>",
		},
		{
			name: "code span then link then pipe",
			in:   "use `get()` see [d](http://x/a?b=1&c=2) | end",
			want: `use <code>get()</code> see <a href="http://x/a?b=1&amp;c=2">d</a> | end`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := string(RenderInline(tc.in))
			if got != tc.want {
				t.Errorf("RenderInline(%q):\n got: %s\nwant: %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestRender_BodyLinkInParagraph(t *testing.T) {
	out := string(Render("See [docs](http://example.com) for more."))
	assertTagBalance(t, out)
	want := `<p>See <a href="http://example.com">docs</a> for more.</p>`
	if out != want {
		t.Errorf("body-level link:\n got: %s\nwant: %s", out, want)
	}
}

func TestRender_BodyLinkInBulletFolds(t *testing.T) {
	out := string(Render("- see [docs](#widget.contract.x) here"))
	assertTagBalance(t, out)
	want := `<ul><li>see <a href="#widget.contract.x">docs</a> here</li></ul>`
	if out != want {
		t.Errorf("link in a list item:\n got: %s\nwant: %s", out, want)
	}
}

func TestRender_WrappedBulletContinuationFolds(t *testing.T) {
	dir := markdownCasesDir(t)
	c := loadClaim(t, dir, "wrapped-bullet-continuation.yaml")
	if c.Layout != "list" {
		t.Fatalf("expected layout: list, got %q", c.Layout)
	}

	// Expected item count: number of lines with zero leading whitespace
	// that start with "- " (top-level bullet markers), computed
	// independently of Render so this is a real regression check, not a
	// tautology.
	wantItems := 0
	for _, line := range strings.Split(c.Body, "\n") {
		if leadingSpaces(line) == 0 && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			wantItems++
		}
	}
	if wantItems < 2 {
		t.Fatalf("sanity check failed: expected multiple top-level bullets, counted %d", wantItems)
	}

	out := string(Render(c.Body))
	assertTagBalance(t, out)

	gotItems := strings.Count(out, "<li>")
	if gotItems != wantItems {
		t.Errorf("li count mismatch: want %d (authored bullets) got %d — wrapped continuation lines are not folding into their bullet", wantItems, gotItems)
	}

	// A wrapped bullet's continuation text must appear joined with its
	// opening line in the SAME <li>, not as a separate fragment. Spot
	// check the first bullet, whose continuation is "current cloud-enabled
	// logs, regardless of `APP_ENV`."
	if !strings.Contains(out, "the destination for all current cloud-enabled logs") {
		t.Errorf("continuation line did not fold into its bullet's text; output:\n%s", out)
	}
}

func TestRender_OrderedListWithLetteredSubItems(t *testing.T) {
	dir := markdownCasesDir(t)
	c := loadClaim(t, dir, "ordered-list-lettered-subitems.yaml")

	out := string(Render(c.Body))
	assertTagBalance(t, out)

	if !strings.Contains(out, "<ol>") {
		t.Fatalf("expected an <ol> for the numbered list, got:\n%s", out)
	}
	if strings.Count(out, "<ol>") != 1 {
		t.Errorf("expected exactly one top-level <ol>, got %d", strings.Count(out, "<ol>"))
	}
	if strings.Count(out, "<ul>") != 1 {
		t.Errorf("expected exactly one nested <ul> (item 1's lettered sub-items), got %d", strings.Count(out, "<ul>"))
	}
	// 4 top-level items (1,2,3,4) + 4 nested items (1a-1d) = 8 <li>.
	if got := strings.Count(out, "<li>"); got != 8 {
		t.Errorf("expected 8 <li> total (4 top-level + 4 nested), got %d:\n%s", got, out)
	}
	for _, want := range []string{"1a", "1b", "1c", "1d"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected lettered sub-item %q in output", want)
		}
	}
	// The nested <ul> must sit between item 1's own text ("lifecycle and
	// propagation") and item 2's text ("state immutability") in document
	// order — i.e. nested inside item 1's <li>, not floated out as a
	// sibling of the top-level <ol>'s items. (The marker digits
	// themselves, e.g. "1.", are stripped by design and never appear in
	// rendered text — only the lettered sub-item labels like "1a" do,
	// since those are part of the item's own prose, marked with a "-"
	// bullet, not a marker this package parses as an ordered item.)
	idx1 := strings.Index(out, "lifecycle and propagation")
	idxUl := strings.Index(out, "<ul>")
	idx2 := strings.Index(out, "state immutability")
	if idx1 == -1 || idxUl == -1 || idx2 == -1 || !(idx1 < idxUl && idxUl < idx2) {
		t.Errorf("nested <ul> does not appear to be nested inside item 1's <li> (idx1=%d idxUl=%d idx2=%d): %s", idx1, idxUl, idx2, out)
	}
}

func TestRender_FenceInsideList_Synthetic(t *testing.T) {
	body := "- item one\n```\ncode here\n```\n- item two"
	out := string(Render(body))
	assertTagBalance(t, out)

	if strings.Count(out, "<pre><code>") != 1 {
		t.Fatalf("expected exactly one fence, got output:\n%s", out)
	}
	if !strings.Contains(out, "<pre><code>code here\n</code></pre>") {
		t.Errorf("fence content not rendered as expected:\n%s", out)
	}
	if !strings.Contains(out, "<li>item one</li>") {
		t.Errorf("expected item one as its own list item:\n%s", out)
	}
	if !strings.Contains(out, "<li>item two</li>") {
		t.Errorf("expected item two as its own list item:\n%s", out)
	}
	// The fence must split the list into two separate <ul> blocks, not
	// merge item one and item two into one list around it.
	if strings.Count(out, "<ul>") != 2 {
		t.Errorf("expected two separate <ul> lists split by the fence, got %d in:\n%s", strings.Count(out, "<ul>"), out)
	}
	// And the fence itself must sit between the two lists.
	firstUlClose := strings.Index(out, "</ul>")
	fenceIdx := strings.Index(out, "<pre><code>")
	secondUlOpen := strings.LastIndex(out, "<ul>")
	if !(firstUlClose < fenceIdx && fenceIdx < secondUlOpen) {
		t.Errorf("fence not positioned between the two lists:\n%s", out)
	}
}

func TestRender_UnclosedFence_FallsThroughAsText(t *testing.T) {
	body := "Intro line.\n\n```\nnever closed"
	out := string(Render(body))
	assertTagBalance(t, out)

	if strings.Contains(out, "<pre>") || strings.Contains(out, "<pre><code>") {
		t.Errorf("unclosed fence must not produce a <pre> block, got:\n%s", out)
	}
	if !strings.Contains(out, "Intro line.") {
		t.Errorf("expected intro paragraph preserved, got:\n%s", out)
	}
	if !strings.Contains(out, "never closed") {
		t.Errorf("expected trailing unclosed-fence text preserved (not dropped), got:\n%s", out)
	}
}

func TestRender_UnclosedInlineCodeSpan_FallsThroughAsText(t *testing.T) {
	body := "Value is `unterminated and stays literal."
	out := string(Render(body))
	assertTagBalance(t, out)

	want := "<p>Value is `unterminated and stays literal.</p>"
	if out != want {
		t.Errorf("unclosed inline code span mismatch.\nwant: %s\ngot:  %s", want, out)
	}
}

func TestRender_MultipleFencesInOneBody(t *testing.T) {
	body := "Before.\n\n```\nfirst\n```\n\nMiddle.\n\n```\nsecond\n```\n\nAfter."
	out := string(Render(body))
	assertTagBalance(t, out)

	if got := strings.Count(out, "<pre><code>"); got != 2 {
		t.Fatalf("expected 2 fences, got %d in:\n%s", got, out)
	}
	if !strings.Contains(out, "<pre><code>first\n</code></pre>") {
		t.Errorf("first fence content missing/mismatched:\n%s", out)
	}
	if !strings.Contains(out, "<pre><code>second\n</code></pre>") {
		t.Errorf("second fence content missing/mismatched:\n%s", out)
	}
	for _, want := range []string{"<p>Before.</p>", "<p>Middle.</p>", "<p>After.</p>"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected paragraph %q in output:\n%s", want, out)
		}
	}
}

func TestRender_HTMLInjectionProbes(t *testing.T) {
	t.Run("paragraph", func(t *testing.T) {
		out := string(Render("Say <script>alert(1)</script> and use bare & here."))
		assertTagBalance(t, out)
		if strings.Contains(out, "<script>") {
			t.Fatalf("unescaped <script> leaked into paragraph output: %s", out)
		}
		if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
			t.Errorf("expected escaped script tag in output: %s", out)
		}
		if !strings.Contains(out, "bare &amp; here") {
			t.Errorf("expected bare & escaped to &amp;: %s", out)
		}
	})

	t.Run("fence", func(t *testing.T) {
		body := "Example:\n\n```\n<script>alert(1)</script> & more\n```"
		out := string(Render(body))
		assertTagBalance(t, out)
		if strings.Contains(out, "<script>alert(1)</script> &") {
			t.Fatalf("unescaped content leaked into fence output: %s", out)
		}
		if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt; &amp; more") {
			t.Errorf("expected escaped script+ampersand in fence output: %s", out)
		}
	})

	t.Run("code_span", func(t *testing.T) {
		out := string(Render("Inline: `<script>alert(1)</script>&x` end."))
		assertTagBalance(t, out)
		if strings.Contains(out, "<script>alert(1)</script>") {
			t.Fatalf("unescaped content leaked into code span output: %s", out)
		}
		want := "<code>&lt;script&gt;alert(1)&lt;/script&gt;&amp;x</code>"
		if !strings.Contains(out, want) {
			t.Errorf("expected escaped code span %q in output: %s", want, out)
		}
	})
}

func TestRender_EmptyBody(t *testing.T) {
	for _, body := range []string{"", "   ", "\n\n\n"} {
		out := string(Render(body))
		if strings.TrimSpace(out) != "" {
			t.Errorf("Render(%q) = %q, want empty/blank output", body, out)
		}
	}
}

// escapeForTest reproduces html.EscapeString via the same stdlib call the
// package under test uses, so fence-content assertions aren't hand-rolling
// a second escaping implementation that could quietly drift from the real
// one.
func escapeForTest(s string) string {
	r := strings.NewReplacer(
		`&`, "&amp;",
		`'`, "&#39;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&#34;",
	)
	return r.Replace(s)
}

// sanity: confirm the fixed fencedClaims list actually matches every fenced
// case file under testdata/markdown-cases, so the list can't silently drift
// stale as fixtures are added/removed.
func TestFencedClaimsListIsComplete(t *testing.T) {
	dir := markdownCasesDir(t)
	fenceRe := regexp.MustCompile("```")

	tracked := make(map[string]bool, len(fencedClaims))
	for _, rel := range fencedClaims {
		tracked[filepath.ToSlash(rel)] = true
	}

	var found []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if filepath.Ext(path) != ".yaml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if fenceRe.Match(data) {
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				return relErr
			}
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk markdown-cases dir: %v", err)
	}

	if len(found) != len(fencedClaims) {
		t.Fatalf("found %d fixture(s) containing ``` on disk, but fencedClaims tracks %d — list is stale.\nfound: %v", len(found), len(fencedClaims), found)
	}
	for _, rel := range found {
		if !tracked[rel] {
			t.Errorf("fixture %s contains a fence but is not in fencedClaims", rel)
		}
	}
}
