package render

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// These tests pin the SHAPE of the theme stylesheet: which parts appear, in
// what order, with which media lists, and byte-for-byte what a given theme
// emits. They go through config.ResolveTheme rather than hand-building a
// ResolvedTheme wherever ordering is the subject, because the ordering
// property that matters to a reader is the one the whole path produces from a
// YAML mapping — Go randomizes map iteration, so a test that fed emission an
// already-ordered slice would pass on a build where the merge had lost the
// order entirely.

// fontBytes returns a file whose first bytes are the signature config checks
// for ext, followed by filler. config validates the SIGNATURE, not the font
// tables, so these are the smallest inputs that exercise the real rule; a
// test that needed a real font file would be testing fontTools, not this.
func fontBytes(ext string) []byte {
	sig := map[string]string{
		".woff2": "wOF2",
		".woff":  "wOFF",
		".otf":   "OTTO",
		".ttf":   "\x00\x01\x00\x00",
	}[ext]
	return append([]byte(sig), []byte("-payload")...)
}

// resolveForTest runs the real merge over theme, reading any font through an
// in-memory table.
func resolveForTest(t *testing.T, theme config.Theme, files map[string][]byte) *config.ResolvedTheme {
	t.Helper()
	cfg := &config.Config{Viewer: config.Viewer{Theme: theme}}
	rt, err := config.ResolveTheme(cfg, func(path string) ([]byte, error) {
		b, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("no such test file %q", path)
		}
		return b, nil
	})
	if err != nil {
		t.Fatalf("config.ResolveTheme: %v", err)
	}
	return rt
}

// TestThemeOverrideCSS_FourParts is the whole emitted document for a theme
// that exercises every part: two fonts, a shared token, a light-only value
// and a dark-only value. The want string is written out in full rather than
// asserted piecewise, because the failure this catches is a part appearing in
// the wrong ORDER or with the wrong media list, and a set of Contains checks
// cannot see either.
//
// The maps are populated in an order that is neither allowlist order nor
// alphabetical, and the whole thing is run 20 times: identical output on every
// iteration is the evidence that the order comes from
// config.ThemeTokenAllowlist and not from whatever Go's map iteration handed
// this run.
func TestThemeOverrideCSS_FourParts(t *testing.T) {
	files := map[string][]byte{
		"/f/one.woff2": fontBytes(".woff2"),
		"/f/two.ttf":   fontBytes(".ttf"),
	}
	want := `@font-face{font-family:"One";src:url(data:font/woff2;base64,` +
		base64.StdEncoding.EncodeToString(files["/f/one.woff2"]) +
		`) format("woff2");font-weight:400;font-style:normal;font-display:swap;}` +
		`@font-face{font-family:"Two";src:url(data:font/ttf;base64,` +
		base64.StdEncoding.EncodeToString(files["/f/two.ttf"]) +
		`) format("truetype");font-weight:300 800;font-style:italic;font-display:swap;}` +
		`:root{--accent:#111111;--font-sans:One, Two;--radius:8px;}` +
		`@media (prefers-color-scheme: light), print{:root{--ink:#101010;--paper:#ffffff;}}` +
		`@media screen and (prefers-color-scheme: dark){:root{--ink:#eeeeee;--paper:#151515;}}`

	for i := 0; i < 20; i++ {
		theme := config.Theme{
			Shared: map[string]string{"radius": "8px", "font-sans": "One, Two", "accent": "#111111"},
			Light:  map[string]string{"ink": "#101010", "paper": "#ffffff"},
			Dark:   map[string]string{"paper": "#151515", "ink": "#eeeeee"},
			Fonts: []config.ThemeFont{
				{Family: "One", Src: "/f/one.woff2"},
				{Family: "Two", Src: "/f/two.ttf", Weight: "300 800", Style: "italic"},
			},
		}
		got := string(themeOverrideCSS(resolveForTest(t, theme, files)))
		if got != want {
			t.Fatalf("emission mismatch on iteration %d:\ngot:  %s\nwant: %s", i, got, want)
		}
	}
}

// TestThemeOverrideCSS_PartOrderByIndex pins the four parts' RELATIVE order by
// position rather than by the exact-string test above, so a future change that
// alters a declaration still fails here for the right reason if it also moves
// a block.
func TestThemeOverrideCSS_PartOrderByIndex(t *testing.T) {
	files := map[string][]byte{"/f/one.woff2": fontBytes(".woff2")}
	rt := resolveForTest(t, config.Theme{
		Shared: map[string]string{"font-sans": "One", "accent": "#111111"},
		Light:  map[string]string{"paper": "#ffffff"},
		Dark:   map[string]string{"paper": "#151515"},
		Fonts:  []config.ThemeFont{{Family: "One", Src: "/f/one.woff2"}},
	}, files)
	got := string(themeOverrideCSS(rt))

	parts := []struct{ name, marker string }{
		{"@font-face", "@font-face{"},
		{"shared :root", ":root{--accent"},
		{"light block", "@media (prefers-color-scheme: light), print{"},
		{"dark block", "@media screen and (prefers-color-scheme: dark){"},
	}
	prev := -1
	for _, p := range parts {
		at := strings.Index(got, p.marker)
		if at < 0 {
			t.Fatalf("%s part missing from output:\n%s", p.name, got)
		}
		if at <= prev {
			t.Fatalf("%s part is out of order (at %d, previous part at %d):\n%s", p.name, at, prev, got)
		}
		prev = at
	}
}

// TestThemeOverrideCSS_FlatOnlyEmitsOnlyRoot is the compatibility property
// every project that already has a viewer.theme depends on: a theme with no
// light:/dark: sub-mapping merges to shared-only declarations, so the output
// is the single ":root{...}" block this engine emitted before per-mode values
// existed — no media query, and therefore no change to any computed style.
func TestThemeOverrideCSS_FlatOnlyEmitsOnlyRoot(t *testing.T) {
	rt := resolveForTest(t, config.Theme{
		Shared: map[string]string{"accent": "#c6613f", "ink": "#141413", "radius": "10px"},
	}, nil)
	got := string(themeOverrideCSS(rt))

	want := ":root{--accent:#c6613f;--ink:#141413;--radius:10px;}"
	if got != want {
		t.Fatalf("flat-only theme emitted:\n%s\nwant:\n%s", got, want)
	}
	for _, forbidden := range []string{"@media", "@font-face"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("flat-only theme emitted a %s block: %s", forbidden, got)
		}
	}
}

// TestThemeOverrideCSS_MediaLists is the print rule of plan v4 A1 stated
// directly: the light block covers print, and the dark block does not.
//
// Everything about printing a themed viewer rests on these two strings. If the
// light block loses its ", print" term, a project's own light palette stops
// applying to paper; if the dark block loses "screen and", a project's dark
// palette reaches paper and a reader prints white-on-black.
func TestThemeOverrideCSS_MediaLists(t *testing.T) {
	rt := resolveForTest(t, config.Theme{
		Light: map[string]string{"paper": "#ffffff"},
		Dark:  map[string]string{"paper": "#151515"},
	}, nil)
	got := string(themeOverrideCSS(rt))

	light, dark := blockFor(t, got, "light"), blockFor(t, got, "dark")
	if !strings.Contains(light, "print") {
		t.Errorf("the light block's media list does not cover print: %q", light)
	}
	if !strings.HasPrefix(dark, "@media screen and ") {
		t.Errorf("the dark block's media list does not begin with \"screen and\": %q", dark)
	}
	if strings.Contains(dark, "print") {
		t.Errorf("the dark block's media list mentions print: %q", dark)
	}
}

// TestThemeOverrideCSS_DarkOnlyTokenHasNoLightBlock is the case that made the
// print pin unnecessary (plan v4 A1): a token the project set ONLY under dark:
// produces a dark block and no light block at all. Under print neither the
// dark block nor any light override matches, so the token keeps the engine's
// own light value — which is the whole point of scoping the dark block to
// screen instead of restating the light palette inside @media print.
func TestThemeOverrideCSS_DarkOnlyTokenHasNoLightBlock(t *testing.T) {
	rt := resolveForTest(t, config.Theme{Dark: map[string]string{"ink": "#eeeeee"}}, nil)
	got := string(themeOverrideCSS(rt))

	if want := "@media screen and (prefers-color-scheme: dark){:root{--ink:#eeeeee;}}"; got != want {
		t.Fatalf("dark-only theme emitted:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "prefers-color-scheme: light") {
		t.Errorf("a dark-only token produced a light block: %s", got)
	}
}

// blockFor returns the "@media ...{" prefix of the light or dark block.
func blockFor(t *testing.T, css, mode string) string {
	t.Helper()
	for _, at := range mediaStarts(css) {
		block := css[at:]
		if end := strings.Index(block, "{"); end >= 0 {
			head := block[:end]
			if strings.Contains(head, "prefers-color-scheme: "+mode) {
				return head
			}
		}
	}
	t.Fatalf("no %s block in output:\n%s", mode, css)
	return ""
}

func mediaStarts(css string) []int {
	var out []int
	for i := 0; ; {
		at := strings.Index(css[i:], "@media")
		if at < 0 {
			return out
		}
		out = append(out, i+at)
		i += at + len("@media")
	}
}

// TestThemeOverrideCSS_FontFacePrefix pins the exact head of an @font-face
// rule. The declaration order inside it is not cosmetic: src must carry a
// format() that matches the data: URL's MIME type, and a browser that
// disagrees with either silently drops the face.
func TestThemeOverrideCSS_FontFacePrefix(t *testing.T) {
	files := map[string][]byte{"/f/probe.woff2": fontBytes(".woff2")}
	rt := resolveForTest(t, config.Theme{
		Shared: map[string]string{"font-sans": "Probe"},
		Fonts:  []config.ThemeFont{{Family: "Probe", Src: "/f/probe.woff2"}},
	}, files)
	got := string(themeOverrideCSS(rt))

	want := `@font-face{font-family:"Probe";src:url(data:font/woff2;base64,` +
		base64.StdEncoding.EncodeToString(files["/f/probe.woff2"]) +
		`) format("woff2");font-weight:400;font-style:normal;font-display:swap;}`
	if !strings.HasPrefix(got, want) {
		t.Fatalf("@font-face rule:\ngot:  %s\nwant prefix: %s", got, want)
	}
}

// TestFontFormat pins both directions of the extension mapping, including the
// two pairs where the MIME type and the format() keyword deliberately
// disagree: .ttf is font/ttf but format("truetype"), .otf is font/otf but
// format("opentype"). Writing "ttf" or "otf" in format() is a rule the
// browser ignores, and nothing else in the pipeline would notice.
func TestFontFormat(t *testing.T) {
	for _, tc := range []struct{ ext, mime, format string }{
		{".woff2", "font/woff2", "woff2"},
		{".woff", "font/woff", "woff"},
		{".ttf", "font/ttf", "truetype"},
		{".otf", "font/otf", "opentype"},
		{".bogus", "", ""},
	} {
		mime, format := fontFormat(tc.ext)
		if mime != tc.mime || format != tc.format {
			t.Errorf("fontFormat(%q) = (%q, %q), want (%q, %q)", tc.ext, mime, format, tc.mime, tc.format)
		}
	}
}

// TestRender_StyleBlockCountUnchanged holds the count of <style> elements at
// three on a themed document AND on an unthemed one.
//
// Both halves matter. The unthemed half is the one the empty ":root" guard
// exists for: shell.html's third <style> must still be emitted, empty, or an
// override sheet that expects three blocks (and every test that locates the
// theme block with LastIndex) is looking at the wrong element. The themed half
// says the fonts and the two media blocks went INTO that same block rather
// than adding a fourth.
func TestRender_StyleBlockCountUnchanged(t *testing.T) {
	dir := t.TempDir()
	fontPath := dir + "/probe.woff2"
	if err := os.WriteFile(fontPath, fontBytes(".woff2"), 0o644); err != nil {
		t.Fatalf("write probe font: %v", err)
	}

	themed := &config.Config{Viewer: config.Viewer{Theme: config.Theme{
		Shared: map[string]string{"accent": "#123456", "font-sans": "Probe"},
		Light:  map[string]string{"paper": "#ffffff"},
		Dark:   map[string]string{"paper": "#151515"},
		Fonts:  []config.ThemeFont{{Family: "Probe", Src: fontPath}},
	}}}

	for _, tc := range []struct {
		name string
		cfg  *config.Config
	}{
		{"themed", themed},
		{"unthemed", nil},
	} {
		out, err := Render(&catalog.Catalog{}, tc.cfg)
		if err != nil {
			t.Fatalf("%s: Render: %v", tc.name, err)
		}
		// Counted by the CLOSING tag. The opening one also appears inside
		// style.css's own comments, which are injected into the document as
		// text, so counting "<style>" counts prose as well as elements.
		if got := strings.Count(out, "</style>"); got != 3 {
			t.Errorf("%s: document has %d <style> elements, want 3", tc.name, got)
		}
	}

	out, err := Render(&catalog.Catalog{}, themed)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	last := out[strings.LastIndex(out, "<style>"):]
	for _, want := range []string{
		`@font-face{font-family:"Probe";`,
		"@media (prefers-color-scheme: light), print{",
		"@media screen and (prefers-color-scheme: dark){",
	} {
		if !strings.Contains(last, want) {
			t.Errorf("last <style> block is missing %q:\n%s", want, last)
		}
	}
}
