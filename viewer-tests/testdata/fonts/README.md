# probe.ttf

A 940-byte TrueType font generated offline with
[fontTools](https://github.com/fonttools/fonttools) for one purpose: making
"did the browser actually load this face?" a measurable question.

- **Family name:** `DossierX Probe`
- **Glyphs:** `.notdef` and `a`. The `a` glyph has an advance width of 2000
  units on a 1000-unit em — **two ems wide**. No shipped system font is, so a
  glyph-width measurement separates "the face loaded" from "the browser fell
  back and the `font-family` string was honoured on paper only".
- **Format:** TTF, not WOFF2, on purpose. The Go standard library has no
  Brotli, so a WOFF2 fixture could not be regenerated or verified from a test
  in this repository without a new dependency.
- **Licence:** CC0 1.0 Universal (public domain dedication). It contains no
  third-party outline data — the single glyph is an empty contour with a set
  advance.

## Two copies, byte-identical

The same file is committed twice:

- `testdata/fixture-theme-preset/fonts/probe.ttf` — read by the fixture's
  `viewer.theme.fonts` block, so the data:-URL emission path is exercised by a
  committed, regenerated viewer.
- `viewer-tests/testdata/fonts/probe.ttf` — read by the browser suite, which
  is a separate Go module and cannot reach `testdata/` through the engine's
  module graph.

`tests/probe_font_test.go` pins that the two are byte-identical, that the size
is 940 bytes, and that the sfnt magic is `00 01 00 00`. Regenerate both
together or neither.
