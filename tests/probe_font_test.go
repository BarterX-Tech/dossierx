// probe_font_test.go pins the committed test font.
//
// probe.ttf is committed TWICE — once under testdata/fixture-theme-preset/fonts/
// (read by the fixture, so the data:-URL emission path is exercised by a
// regenerated, committed viewer) and once under viewer-tests/testdata/fonts/
// (read by the browser suite, which is a SEPARATE Go module and cannot reach
// testdata/ through the engine's module graph). Two copies is the price of that
// module split, and the failure it invites is the ordinary one: someone
// regenerates or replaces one of them and the other silently stays behind.
//
// The browser suite's font evidence is a GLYPH WIDTH — the face's single "a" is
// two ems wide, which no shipped system font is — so the two copies drifting
// would not make the suite red. It would make the suite measure a different
// font than the fixture ships, and pass. That is the whole reason this file
// exists, and it is why the assertions below are on bytes rather than on
// "both files are present".
package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The two committed paths, relative to the repository root (this test file
// lives in tests/, one level down).
const (
	probeFontFixturePath = "../testdata/fixture-theme-preset/fonts/probe.ttf"
	probeFontSuitePath   = "../viewer-tests/testdata/fonts/probe.ttf"
)

// probeFontSize is the byte size of the generated face, pinned so a
// regeneration that changes the font at all has to come through this file and
// state what changed. 940 bytes is what fontTools emitted for the one-glyph,
// 2000/1000-advance face described in the README beside it.
const probeFontSize = 940

// sfntTrueTypeMagic is the four-byte version tag every TrueType-outline sfnt
// file opens with (0x00010000). A WOFF/WOFF2/OTF file opens with "wOFF",
// "wOF2" and "OTTO" respectively, so this one check distinguishes the format
// the fixture's `format("truetype")` hint promises from every neighbouring one
// — a promise a browser is entitled to act on.
var sfntTrueTypeMagic = []byte{0x00, 0x01, 0x00, 0x00}

func readProbeFont(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read committed probe font %s: %v", path, err)
	}
	return b
}

func TestProbeFontCopiesAreByteIdentical(t *testing.T) {
	a := readProbeFont(t, probeFontFixturePath)
	b := readProbeFont(t, probeFontSuitePath)

	// Vacuity guard: an empty file would satisfy bytes.Equal against another
	// empty file and every "is it a TTF" check below would then be reading
	// past the end of a slice rather than a header.
	if len(a) == 0 || len(b) == 0 {
		t.Fatalf("a committed probe font is empty (%s: %d bytes, %s: %d bytes); "+
			"every assertion in this file would be vacuous",
			probeFontFixturePath, len(a), probeFontSuitePath, len(b))
	}

	if !bytes.Equal(a, b) {
		t.Fatalf("the two committed probe.ttf copies differ (%s: %d bytes, %s: %d bytes).\n"+
			"The browser suite measures the SUITE copy and the committed viewer embeds the "+
			"FIXTURE copy, so a drift here makes the suite green about a font the fixture "+
			"does not ship. Regenerate both or neither — see the README beside either file.",
			probeFontFixturePath, len(a), probeFontSuitePath, len(b))
	}
}

func TestProbeFontIsAPinnedTrueTypeSfnt(t *testing.T) {
	for _, path := range []string{probeFontFixturePath, probeFontSuitePath} {
		b := readProbeFont(t, path)
		if got := len(b); got != probeFontSize {
			t.Errorf("%s is %d bytes, want %d — the font changed; update probeFontSize "+
				"in the same commit and say what changed", path, got, probeFontSize)
		}
		if len(b) < len(sfntTrueTypeMagic) {
			t.Errorf("%s is %d bytes, too short to carry an sfnt version tag", path, len(b))
			continue
		}
		if got := b[:len(sfntTrueTypeMagic)]; !bytes.Equal(got, sfntTrueTypeMagic) {
			t.Errorf("%s opens with % x, want % x (a TrueType-outline sfnt). "+
				"The fixture's @font-face declares format(\"truetype\"); a WOFF (\"wOFF\"), "+
				"WOFF2 (\"wOF2\") or CFF/OTF (\"OTTO\") file there would make that hint a lie.",
				path, got, sfntTrueTypeMagic)
		}
	}
}
