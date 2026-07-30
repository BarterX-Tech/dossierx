package markdown

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/urlsafe"
)

// testAssets is the URL prefix a claim body is rendered with throughout this
// file. It is deliberately NOT the production shape (internal/render/components
// builds that); what matters here is only that RenderClaimBody prepends
// whatever it is given to an accepted src and to nothing else.
const testAssets AssetPrefix = "/claim-assets/widget.contract.demo/"

// --- the opt-in boundary ---------------------------------------------------

// TestRender_NeverEmitsAnImage is the fail-closed-by-construction test, and it
// is the most important one in this file.
//
// markdown.Render is what the shared funcMap's "markdown" binding, comments.html
// and internal/serve's commentToDTO all call. Its meaning is "no images", so
// every one of those surfaces is correct WITHOUT BEING EDITED, and a future
// caller that forgets to opt in loses a diagram rather than gaining a capability.
func TestRender_NeverEmitsAnImage(t *testing.T) {
	bodies := []string{
		"![Alt](assets/diagram.png)",
		"text ![Alt](assets/diagram.png) more",
		"- ![Alt](assets/diagram.png)",
		"> ![Alt](assets/diagram.png)",
		"| a | b |\n| - | - |\n| ![Alt](assets/diagram.png) | x |",
		"### ![Alt](assets/diagram.png)",
		"[![Alt](assets/diagram.png)](https://example.com)",
	}
	for _, body := range bodies {
		out := string(Render(body))
		if strings.Contains(out, "<img") {
			t.Errorf("Render(%q) emitted an image: %s", body, out)
		}
	}
}

// TestRender_ImageRunIsEscapedLiteralText pins the DEGRADATION, not just the
// absence: a complete "![alt](src)" run under Render is the author's own bytes,
// escaped, with no anchor. Before images existed the "!" fell through and the
// "[alt](src)" behind it became a LINK, which is what the two pre-phase-D
// goldens recorded; markdown-sanity's comment-surface finding already promises
// the reader "renders as escaped literal text", and this is that promise.
func TestRender_ImageRunIsEscapedLiteralText(t *testing.T) {
	cases := []struct{ body, want string }{
		{"![Alt text](./diagram.png)", "<p>![Alt text](./diagram.png)</p>"},
		{"![Alt](assets/a.png)", "<p>![Alt](assets/a.png)</p>"},
		{"![Alt](https://example.com/a.png)", "<p>![Alt](https://example.com/a.png)</p>"},
		// The alt and the src are still author bytes and still escaped.
		{`![a"<b>](assets/a.png)`, `<p>![a&#34;&lt;b&gt;](assets/a.png)</p>`},
	}
	for _, c := range cases {
		if got := string(Render(c.body)); got != c.want {
			t.Errorf("Render(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

// TestRender_IncompleteImageRunIsUnchanged pins that "!" is an opener ONLY for a
// COMPLETE image run. A bare "!", a "![" that never closes and a "!" before an
// ordinary link are untouched, so exclamation marks in prose keep costing
// nothing and the link construct behind a space is unaffected.
func TestRender_IncompleteImageRunIsUnchanged(t *testing.T) {
	cases := []struct{ body, want string }{
		{"Hello!", "<p>Hello!</p>"},
		{"![unterminated", "<p>![unterminated</p>"},
		{"![no paren](", "<p>![no paren](</p>"},
		{"Wow! [text](https://example.com)", `<p>Wow! <a href="https://example.com">text</a></p>`},
	}
	for _, c := range cases {
		if got := string(Render(c.body)); got != c.want {
			t.Errorf("Render(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

// TestRenderClaimBody_EmitsAnImage is the opt-in half: the SAME body that
// renders as literal text through Render renders a real <img> through
// RenderClaimBody.
func TestRenderClaimBody_EmitsAnImage(t *testing.T) {
	const body = "![Sequence diagram](assets/seq.svg)"
	want := `<p><img class="md-img" src="/claim-assets/widget.contract.demo/assets/seq.svg" alt="Sequence diagram"></p>`
	if got := string(RenderClaimBody(body, testAssets)); got != want {
		t.Errorf("RenderClaimBody(%q) = %q, want %q", body, got, want)
	}
	if got := string(Render(body)); strings.Contains(got, "<img") {
		t.Errorf("the same body must not produce an image through Render: %q", got)
	}
}

// TestRenderClaimBody_ZeroPrefixRendersNoImage pins the fail-closed default one
// level down: an AssetPrefix that was never set is not "serve it relative", it
// is "this claim has no image capability at all".
func TestRenderClaimBody_ZeroPrefixRendersNoImage(t *testing.T) {
	out := string(RenderClaimBody("![Alt](assets/a.png)", ""))
	if strings.Contains(out, "<img") {
		t.Errorf("a zero AssetPrefix must render no image: %q", out)
	}
	if want := "<p>![Alt](assets/a.png)</p>"; out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

// TestRenderClaimBody_ImagesInEveryClaimBodyContainer walks the container matrix
// for the one construct this phase adds. A claim body's paragraphs, list items,
// blockquote interiors, headings and pipe-table cells all inherit the caller's
// permission (amendment A3: "a pipe-table cell inside a claim body — yes").
func TestRenderClaimBody_ImagesInEveryClaimBodyContainer(t *testing.T) {
	cases := []struct{ name, body string }{
		{"paragraph", "![A](assets/a.png)"},
		{"list item", "- ![A](assets/a.png)"},
		{"blockquote", "> ![A](assets/a.png)"},
		{"heading", "### ![A](assets/a.png)"},
		{"table cell", "| h |\n| - |\n| ![A](assets/a.png) |"},
	}
	const want = `<img class="md-img" src="/claim-assets/widget.contract.demo/assets/a.png" alt="A">`
	for _, c := range cases {
		got := string(RenderClaimBody(c.body, testAssets))
		if !strings.Contains(got, want) {
			t.Errorf("%s: RenderClaimBody(%q) = %q, want it to contain %q", c.name, c.body, got, want)
		}
	}
}

// TestRenderClaimBody_FenceContentIsNotAnImage pins that fence content is still
// raw escaped source: an image written inside a fenced example is documentation
// ABOUT the syntax, not a reference.
func TestRenderClaimBody_FenceContentIsNotAnImage(t *testing.T) {
	body := "```\n![A](assets/a.png)\n```"
	out := string(RenderClaimBody(body, testAssets))
	if strings.Contains(out, "<img") {
		t.Errorf("a fenced example must not become an image: %q", out)
	}
}

// --- the src gate ----------------------------------------------------------

// TestRenderClaimBody_RefusedSrcIsEscapedLiteralText is the refusal contract, in
// the shape the reviewer sees it: a refused src renders as the author's own
// bytes, escaped — NEVER a broken <img>, and never an anchor either.
func TestRenderClaimBody_RefusedSrcIsEscapedLiteralText(t *testing.T) {
	cases := []struct{ name, src string }{
		{"absolute https", "https://evil.example/x.png"},
		{"absolute http", "http://evil.example/x.png"},
		{"protocol-relative", "//evil.example/x.png"},
		{"backslash authority", `\\evil.example\x.png`},
		{"mixed authority", `/\evil.example/x.png`},
		{"mixed authority 2", `\/evil.example/x.png`},
		{"data uri", "data:image/svg+xml,<svg onload=alert(1)>"},
		{"javascript scheme", "javascript:alert(1)"},
		{"control-char scheme", "ht\ttp://evil.example/x.png"},
		{"control-char scheme entity", "ht&#9;tp://evil.example/x.png"},
		{"leading control byte", "\x01//evil.example/x.png"},
		{"entity-encoded authority", "&#47;&#47;evil.example/x.png"},
		{"root relative", "/assets/x.png"},
		{"parent escape", "../other-facet/assets/x.png"},
		{"parent escape inside", "assets/../../elsewhere/x.png"},
		{"query", "assets/x.png?v=1"},
		{"fragment", "assets/x.png#frag"},
		{"not co-located", "diagram.png"},
		{"sibling pool", "shared/assets/x.png"},
		{"assets is the file", "assets"},
		{"assets dir only", "assets/"},
		{"disallowed extension", "assets/notes.txt"},
		{"disallowed extension html", "assets/page.html"},
		{"no extension", "assets/diagram"},
		{"percent encoded traversal", "assets/%2e%2e/x.png"},
		{"backslash separator", `assets\x.png`},
		{"empty", ""},
	}
	for _, c := range cases {
		body := "![Alt](" + c.src + ")"
		got := string(RenderClaimBody(body, testAssets))
		if strings.Contains(got, "<img") {
			t.Errorf("%s: src %q produced an image: %q", c.name, c.src, got)
		}
		if strings.Contains(got, "<a ") {
			t.Errorf("%s: src %q produced an anchor: %q", c.name, c.src, got)
		}
		if !strings.Contains(got, "![Alt](") {
			t.Errorf("%s: src %q did not fall through as literal text: %q", c.name, c.src, got)
		}
	}
}

// TestRenderClaimBody_AcceptedSrcShapes is the positive half of the gate.
func TestRenderClaimBody_AcceptedSrcShapes(t *testing.T) {
	cases := []struct{ src, wantRel string }{
		{"assets/a.png", "assets/a.png"},
		{"./assets/a.png", "assets/a.png"},
		{"assets/./a.png", "assets/a.png"},
		{"assets/sub/a.png", "assets/sub/a.png"},
		{"assets/a.jpg", "assets/a.jpg"},
		{"assets/a.jpeg", "assets/a.jpeg"},
		{"assets/a.gif", "assets/a.gif"},
		{"assets/a.webp", "assets/a.webp"},
		{"assets/a.svg", "assets/a.svg"},
		{"assets/a.PNG", "assets/a.PNG"},
		{"assets/my-diagram_v2.png", "assets/my-diagram_v2.png"},
	}
	for _, c := range cases {
		body := "![A](" + c.src + ")"
		want := `<img class="md-img" src="` + string(testAssets) + c.wantRel + `" alt="A">`
		got := string(RenderClaimBody(body, testAssets))
		if !strings.Contains(got, want) {
			t.Errorf("src %q: got %q, want it to contain %q", c.src, got, want)
		}
	}
}

// TestImageSrc_CanonicalizesWhatItAccepts pins that the gate returns the bytes
// the renderer emits, not the bytes the author wrote. Everything downstream —
// serve's allowlist key and its filesystem path — is built from this value, so
// "validated" and "emitted" have to be the same string or the allowlist is
// guarding a path nobody asked for.
func TestImageSrc_CanonicalizesWhatItAccepts(t *testing.T) {
	if rel, ok := ImageSrc("./assets/a.png"); !ok || rel != "assets/a.png" {
		t.Errorf(`ImageSrc("./assets/a.png") = %q, %v; want "assets/a.png", true`, rel, ok)
	}
	if _, ok := ImageSrc("../assets/a.png"); ok {
		t.Error("a parent-escaping src must not be accepted")
	}
}

// TestImageSrcOffOrigin_KnowsBackslashIsASlash is amendment A4's load-bearing
// clause, stated as a test because it is the half a simpler check gets wrong:
// browsers normalize "\" to "/" in the authority position under a special
// scheme, so "/\host", "\\host" and "\/host" leave the origin exactly as
// "//host" does. internal/lint's mockupAbsoluteURLPattern was the weaker check;
// it is gone, and both boundaries now call urlsafe.IsOffOrigin, which is what
// this test asserts against. The expectations are unchanged from when this
// package owned the rule, which is the point: routing it through urlsafe moved
// no accept/reject decision on the image path.
func TestImageSrcOffOrigin_KnowsBackslashIsASlash(t *testing.T) {
	for _, raw := range []string{
		"//evil.example/x.png",
		`/\evil.example/x.png`,
		`\\evil.example\x.png`,
		`\/evil.example/x.png`,
		"https://evil.example/x.png",
		"ht\ttp://evil.example/x.png",
		"&#47;&#47;evil.example/x.png",
		"/root-relative.png",
		"x.png?q=1",
		"x.png#f",
		"",
	} {
		if !urlsafe.IsOffOrigin(raw) {
			t.Errorf("urlsafe.IsOffOrigin(%q) = false, want true", raw)
		}
	}
	for _, raw := range []string{"assets/x.png", "./assets/x.png", "../a/x.png", "x.png"} {
		if urlsafe.IsOffOrigin(raw) {
			t.Errorf("urlsafe.IsOffOrigin(%q) = true, want false", raw)
		}
	}
}

// TestImageSrcLegal_RefusesTraversalAndNothingElse pins the split between the
// two shared gates, which is the split internal/lint's two content rules divide
// a bad src along: off-origin is markdown-sanity's finding, a ".." traversal is
// asset-scope's as well. ImageSrc composes the second of them with this
// package's own co-location, shape and extension rules.
func TestImageSrcLegal_RefusesTraversalAndNothingElse(t *testing.T) {
	if urlsafe.IsRelativePath("../a/x.png") {
		t.Error(`urlsafe.IsRelativePath("../a/x.png") = true, want false`)
	}
	if urlsafe.IsRelativePath(`a\..\x.png`) {
		t.Error(`a backslash-separated ".." must be refused too`)
	}
	if !urlsafe.IsRelativePath("x.png") {
		t.Error(`urlsafe.IsRelativePath("x.png") = false, want true (legality is not co-location)`)
	}
	if !urlsafe.IsRelativePath("assets/x.png") {
		t.Error(`urlsafe.IsRelativePath("assets/x.png") = false, want true`)
	}
}

// --- the escaping boundary -------------------------------------------------

// TestRenderClaimBody_AltIsEscapedInAttributeContext pins that alt is author
// bytes in an ATTRIBUTE and is escaped as such — a double quote in an alt may
// never close the attribute — and that alt is never itself parsed as markdown
// (amendment A3's per-surface table: "image alt — never").
func TestRenderClaimBody_AltIsEscapedInAttributeContext(t *testing.T) {
	cases := []struct{ body, want string }{
		{
			`![a" onerror="alert(1)](assets/a.png)`,
			`alt="a&#34; onerror=&#34;alert(1)"`,
		},
		{
			"![<script>x</script>](assets/a.png)",
			`alt="&lt;script&gt;x&lt;/script&gt;"`,
		},
		{
			"![a & b](assets/a.png)",
			`alt="a &amp; b"`,
		},
		{
			// Markdown inside an alt stays literal: no <strong>, no anchor,
			// nothing that could carry a second attribute.
			"![**bold** _em_ `code`](assets/a.png)",
			"alt=\"**bold** _em_ `code`\"",
		},
	}
	for _, c := range cases {
		got := string(RenderClaimBody(c.body, testAssets))
		if !strings.Contains(got, c.want) {
			t.Errorf("RenderClaimBody(%q) = %q, want it to contain %q", c.body, got, c.want)
		}
		if strings.Contains(got, "<a ") || strings.Contains(got, "<strong>") {
			t.Errorf("RenderClaimBody(%q) parsed markup inside an alt: %q", c.body, got)
		}
	}
}

// TestRenderClaimBody_LinkGrammarCeilingAppliesToImages pins that the image
// inherits parseLink's documented v0.3.1 grammar ceiling exactly — the alt runs
// to the FIRST "]" and the src to the FIRST ")", with no nested brackets and no
// escaped delimiters inside either. A "]" inside an alt therefore ends it early,
// and what follows is read as the src; here that is an off-origin URL, so the
// whole run degrades to literal text. This is a ceiling, not a hole: the failure
// direction is refusal.
func TestRenderClaimBody_LinkGrammarCeilingAppliesToImages(t *testing.T) {
	const body = "![**bold** [t](https://x.example)](assets/a.png)"
	got := string(RenderClaimBody(body, testAssets))
	if strings.Contains(got, "<img") || strings.Contains(got, "<a ") {
		t.Errorf("a bracket inside an alt must degrade to literal text, got %q", got)
	}
}

// TestRenderClaimBody_EscapedBangIsNotAnImage pins that the escape mechanism
// reaches this construct for free: "!" is in the closed escapable set, and an
// escaped byte is written straight out and can never open anything.
func TestRenderClaimBody_EscapedBangIsNotAnImage(t *testing.T) {
	out := string(RenderClaimBody(`\![A](assets/a.png)`, testAssets))
	if strings.Contains(out, "<img") {
		t.Errorf(`an escaped "!" must not open an image: %q`, out)
	}
	if !strings.Contains(out, `<a href="assets/a.png">A</a>`) {
		t.Errorf(`an escaped "!" leaves an ordinary link behind it: %q`, out)
	}
}

// TestRenderClaimBody_PrefixIsTheOnlyInterpolationPoint pins that the emitted
// tag is a fixed literal in this package plus exactly two escaped values.
func TestRenderClaimBody_PrefixIsTheOnlyInterpolationPoint(t *testing.T) {
	out := string(RenderClaimBody("![A](assets/a.png)", `"><script>`))
	if strings.Contains(out, "<script>") {
		t.Errorf("a hostile prefix must be escaped in attribute context: %q", out)
	}
	if !strings.Contains(out, `src="&#34;&gt;&lt;script&gt;assets/a.png"`) {
		t.Errorf("got %q", out)
	}
}

// --- RenderInline stays image-free -----------------------------------------

// TestRenderInline_NeverEmitsAnImage pins the third entry point. RenderInline is
// what a layout:table claim's Rows[].cell goes through (components' "cell"
// helper), and amendment A3's per-surface table says that surface renders no
// image. It has no way to be told otherwise, which is the point.
func TestRenderInline_NeverEmitsAnImage(t *testing.T) {
	out := string(RenderInline("![A](assets/a.png)"))
	if strings.Contains(out, "<img") {
		t.Errorf("RenderInline emitted an image: %q", out)
	}
	if want := "![A](assets/a.png)"; out != want {
		t.Errorf("RenderInline = %q, want %q", out, want)
	}
}

// --- the serve allowlist source --------------------------------------------

// TestClaimBodyImages_ReportsExactlyWhatWouldBeEmitted is the property serve's
// image route is safe BECAUSE of: the allowlist is computed from the loaded
// claims by the same code path that renders them, so a path the route will
// answer for is a path the renderer would have emitted, and no other.
func TestClaimBodyImages_ReportsExactlyWhatWouldBeEmitted(t *testing.T) {
	body := strings.Join([]string{
		"![one](assets/one.png)",
		"",
		"- ![two](./assets/sub/two.svg)",
		"",
		"> ![three](assets/three.webp)",
		"",
		"| h |",
		"| - |",
		"| ![four](assets/four.gif) |",
		"",
		"![refused](https://evil.example/x.png)",
		"![refused](../elsewhere/assets/x.png)",
		"![refused](assets/notes.txt)",
		"",
		"```",
		"![fenced](assets/never.png)",
		"```",
	}, "\n")

	got := ClaimBodyImages(body)
	want := []string{
		"assets/one.png",
		"assets/sub/two.svg",
		"assets/three.webp",
		"assets/four.gif",
	}
	if len(got) != len(want) {
		t.Fatalf("ClaimBodyImages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ClaimBodyImages = %v, want %v", got, want)
		}
	}

	// The agreement property, asserted rather than assumed: every reported
	// path appears in the rendered document behind the prefix, and the
	// document contains no other src.
	out := string(RenderClaimBody(body, testAssets))
	if n := strings.Count(out, "<img"); n != len(want) {
		t.Fatalf("rendered %d images, ClaimBodyImages reported %d", n, len(want))
	}
	for _, rel := range got {
		if !strings.Contains(out, `src="`+string(testAssets)+rel+`"`) {
			t.Errorf("reported %q but the render carries no such src: %s", rel, out)
		}
	}
}

// TestClaimBodyImages_EmptyForAnImagelessBody keeps the allowlist builder honest
// about the ordinary case.
func TestClaimBodyImages_EmptyForAnImagelessBody(t *testing.T) {
	if got := ClaimBodyImages("just prose with a [link](https://x.example)"); len(got) != 0 {
		t.Errorf("ClaimBodyImages = %v, want none", got)
	}
}
