package markdown

import (
	"strings"
	"testing"
)

// markdown_images_substitution_test.go covers ONE property, because it is the
// property a shape rule over a closed character set is worth nothing without:
//
//	ImageSrc REFUSES a src it cannot spell. It never REWRITES one.
//
// The distinction is not pedantic. The gate used to normalise a src by deleting
// every byte <= 0x20 and 0x7f before the shape rule ever saw it, so
// "assets/team photo.png" did not fail — it BECAME "assets/teamphoto.png", a
// different, legal, possibly-existing filename. The renderer then emitted that
// path, the allowlist indexed it, and the route served whatever file happened to
// be at it. Nothing anywhere reported a problem, because by the time any rule
// ran the offending bytes were gone.
//
// A refusal degrades to escaped literal text, which a human reading the page
// sees. A substitution silently serves a file nobody named, which nobody sees.
// These tests pin the first and forbid the second.
func TestImageSrc_RefusesRatherThanRewrites(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"a space in the filename", "assets/team photo.png"},
		{"a space before the extension", "assets/a .png"},
		{"a NUL inside the filename", "assets/a.pn\x00g"},
		{"a trailing tab", "assets/a.png\t"},
		{"a leading space", " assets/a.png"},
		{"a trailing space", "assets/a.png "},
		{"a newline inside the path", "assets/a\nb.png"},
		{"a tab inside the directory segment", "as\tsets/a.png"},
		{"an entity-encoded space", "assets/a&#32;b.png"},
		{"an entity-encoded tab", "assets/a&#9;b.png"},
		{"a DEL byte", "assets/a\x7f.png"},
		{"a vertical tab", "assets/a\x0bb.png"},
		{"a non-ASCII byte", "assets/café.png"},
		{"a percent escape", "assets/a%20b.png"},
		{"a quote", `assets/a".png`},
	}
	for _, c := range cases {
		if rel, ok := ImageSrc(c.raw); ok {
			t.Errorf("%s: ImageSrc(%q) = %q, true; want a refusal", c.name, c.raw, rel)
		}
	}
}

// TestImageSrc_NeverNamesAFileTheAuthorDidNot is the same property stated as the
// consequence that made it a finding: the rewritten form of a refused src is a
// REAL, DIFFERENT filename, and the route would have served it.
func TestImageSrc_NeverNamesAFileTheAuthorDidNot(t *testing.T) {
	for _, c := range []struct{ raw, mustNotBe string }{
		{"assets/team photo.png", "assets/teamphoto.png"},
		{"assets/a .png", "assets/a.png"},
		{"assets/a.pn\x00g", "assets/a.png"},
		{"assets/a.png\t", "assets/a.png"},
	} {
		rel, ok := ImageSrc(c.raw)
		if ok {
			t.Errorf("ImageSrc(%q) accepted as %q; want a refusal", c.raw, rel)
		}
		if rel == c.mustNotBe {
			t.Errorf("ImageSrc(%q) named %q — a file the author did not write", c.raw, c.mustNotBe)
		}
	}
}

// TestRenderClaimBody_SubstitutableSrcIsEscapedLiteralText is the end-to-end
// half: the reviewer sees the src they wrote, unrendered, rather than a diagram
// loaded from some other file.
func TestRenderClaimBody_SubstitutableSrcIsEscapedLiteralText(t *testing.T) {
	const body = "![the file I meant](assets/team photo.png)"
	got := string(RenderClaimBody(body, testAssets))
	if strings.Contains(got, "<img") {
		t.Fatalf("emitted an image for a src with a space in it: %q", got)
	}
	if !strings.Contains(got, "![the file I meant](assets/team photo.png)") {
		t.Errorf("want the whole run as literal text, got %q", got)
	}
	if refs := ClaimBodyImages(body); len(refs) != 0 {
		t.Errorf("ClaimBodyImages = %v, want none", refs)
	}
}

// TestImageSrc_StillAcceptsTheCanonicalShapes guards the fix from over-reach:
// the closed set, the "./" elision and the nested-directory case must all still
// pass. Only bytes outside [A-Za-z0-9._-] (and "/") are newly refused.
func TestImageSrc_StillAcceptsTheCanonicalShapes(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{"assets/a.png", "assets/a.png"},
		{"./assets/a.png", "assets/a.png"},
		{"assets/./a.png", "assets/a.png"},
		{"assets/sub/dir/a-b_c.2.webp", "assets/sub/dir/a-b_c.2.webp"},
		{"assets/A.PNG", "assets/A.PNG"},
	} {
		rel, ok := ImageSrc(c.raw)
		if !ok || rel != c.want {
			t.Errorf("ImageSrc(%q) = %q, %v; want %q, true", c.raw, rel, ok, c.want)
		}
	}
}
