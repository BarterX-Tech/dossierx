package viewertests

// NOTHING IS PAINTED BETWEEN THE LINES OF A FENCED CODE BLOCK.
//
// The defect this pins: the stylesheet's inline-code pill rule
// (`code { border: 1px solid var(--border) }`) applies to the <code> inside a
// <pre> as well, and the `.claim-body pre code` reset cancelled the pill's
// background, padding and radius but not its border. That <code> is ONE inline
// box, and an inline box FRAGMENTS across line boxes; box-decoration-break's
// initial value `slice` paints the border's top and bottom edge on every
// fragment. So a two-line fenced block was drawn with a 1px --border line under
// its first line and another above its second — a seam the maintainer saw in a
// browser, in both colour schemes, on every multi-line code block in the corpus.
//
// A computed-style probe alone would not have caught it and did not: the parity
// table read `background-color` and `padding` off `.claim-body pre code` and
// both were correct. What was wrong was a property nobody read, painted at a
// position nobody sampled. So this test reads PIXELS, at the y-positions where
// the fragments meet, and asks the only question a reader can ask: is anything
// drawn there that is not the block's own background?
//
// WHAT IT COVERS, stated because a pixel sample is a coverage claim:
// fixture-theme-flat's fenced block (the only one in the parity fixtures),
// rendered fresh by the engine under test, in light and dark, at 1280px. It
// does not cover the print sheet, other widths, or the fenced blocks in
// .sbody / .comment-body / .claim-list-items, which the parity fixtures do not
// instantiate — see the seam's own row in theme_parity_test.go for the
// computed-style half, which runs at both widths.

import (
	"bytes"
	"context"
	"image"
	_ "image/png"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// seamTolerance is the per-channel slack allowed between a sampled row and the
// <pre>'s own background. It is NOT there to absorb a seam: the seam this test
// exists for was a 17-unit step in dark and an 11-unit step in light, both an
// order of magnitude above this. It is there because a background painted from
// a color-mix() and a background read back through getComputedStyle can differ
// by a rounding unit in the last place.
const seamTolerance = 2

func TestNothingIsPaintedBetweenFencedCodeLines(t *testing.T) {
	browser := resolveBrowser(t)
	url := renderFixtureFresh(t, "fixture-theme-flat")

	for _, scheme := range []string{"light", "dark"} {
		scheme := scheme
		t.Run(scheme, func(t *testing.T) {
			assertNoSeam(t, browser, url, scheme)
		})
	}
}

func assertNoSeam(t *testing.T, browser, url, scheme string) {
	t.Helper()
	ctx := browserContextFor(t, browser)
	runCDP(t, ctx,
		chromedp.EmulateViewport(1280, 900, chromedp.EmulateScale(1)),
		chromedp.Navigate(url),
	)
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	waitVisible(t, ctx, ".claim-body pre")
	emulateColorScheme(t, ctx, scheme)
	suppressTransitions(t, ctx, "")

	var m struct {
		Rect struct {
			X, Y, W, H float64
		} `json:"rect"`
		// Gaps are the y-positions, relative to the pre's border box, where two
		// line-box fragments of the <code> meet: the bottom of fragment i and
		// the top of fragment i+1.
		Gaps    []float64 `json:"gaps"`
		NRects  int       `json:"nRects"`
		BG      string    `json:"bg"`
		Alpha   float64   `json:"alpha"`
		PadLeft float64   `json:"padLeft"`
	}
	evalInto(t, ctx, `(function(){
		var pre = document.querySelector('.claim-body pre');
		var code = pre.querySelector('code');
		var pr = pre.getBoundingClientRect();
		var rects = Array.prototype.slice.call(code.getClientRects());
		rects.sort(function(a, b){ return a.top - b.top; });
		var gaps = [];
		for (var i = 0; i + 1 < rects.length; i++) {
			if (rects[i + 1].top - rects[i].bottom > 4) { continue; }
			if (rects[i + 1].top <= rects[i].top) { continue; }
			gaps.push((rects[i].bottom + rects[i + 1].top) / 2 - pr.top);
		}
		var cs = getComputedStyle(pre);
		var probe = document.createElement('span');
		probe.style.color = cs.backgroundColor;
		document.body.appendChild(probe);
		var resolved = getComputedStyle(probe).color;
		document.body.removeChild(probe);
		var alpha = 1;
		var mm = /^rgba?\(([^)]*)\)$/.exec(resolved);
		if (mm) { var parts = mm[1].split(','); if (parts.length > 3) { alpha = parseFloat(parts[3]); } }
		return {
			rect: {X: pr.left + window.scrollX, Y: pr.top + window.scrollY, W: pr.width, H: pr.height},
			gaps: gaps, nRects: rects.length, bg: resolved, alpha: alpha,
			padLeft: parseFloat(cs.paddingLeft)
		};
	})()`, &m)

	// ---- VACUITY GUARDS ----
	//
	// Each of these is a way this test could sample nothing and read as a pass.
	if m.NRects < 2 {
		t.Fatalf("%s: the fenced block's <code> reports %d client rect(s). A seam BETWEEN two "+
			"line boxes cannot exist, or be sampled, below two — so this test would have passed "+
			"over zero assertions. The fixture is supposed to carry a multi-line fenced block.",
			scheme, m.NRects)
	}
	if len(m.Gaps) == 0 {
		t.Fatalf("%s: the <code> has %d line-box fragments but no adjacent pair to sample "+
			"between; nothing would be checked", scheme, m.NRects)
	}
	if m.Alpha < 1 {
		t.Fatalf("%s: the <pre>'s background resolves to %q (alpha %.3f). Against a transparent "+
			"or translucent block the \"differs from the background\" test below is not a test of "+
			"anything, because whatever shows through is not the pre's own paint.",
			scheme, m.BG, m.Alpha)
	}
	if m.Rect.W <= 0 || m.Rect.H <= 0 {
		t.Fatalf("%s: the fenced block measures %vx%v", scheme, m.Rect.W, m.Rect.H)
	}

	// A 2x capture of the block, so a 1px CSS seam is two full device rows and
	// cannot be averaged away by a fractional device-pixel boundary.
	const scale = 2
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
		var err error
		buf, err = page.CaptureScreenshot().WithCaptureBeyondViewport(true).
			WithClip(&page.Viewport{
				X: m.Rect.X, Y: m.Rect.Y, Width: m.Rect.W, Height: m.Rect.H, Scale: scale,
			}).Do(c)
		return err
	})); err != nil {
		t.Fatalf("%s: screenshot the fenced block: %v", scheme, err)
	}
	// Restore the tab's device metrics before it is closed. A 2x
	// captureBeyondViewport clip leaves the shared browser in a state the NEXT
	// test's tab inherits: measured on this suite, TestOverlaysAreMutuallyExclusive
	// (viewer_test.go) ran directly after this test, pinned its own 1200x800
	// viewport, and its real click on .comment-chip never opened the panel —
	// reproducibly in the full run and in a two-test run of just these two,
	// and never with this test excluded, with the clip at scale 1, or with
	// this reset in place. The reset goes through runCDP so a browser that
	// cannot clear its override fails this test here rather than the next one.
	runCDP(t, ctx, emulation.ClearDeviceMetricsOverride())
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("%s: decode the fenced block screenshot: %v", scheme, err)
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		t.Fatalf("%s: the fenced block screenshot is empty (%v)", scheme, b)
	}

	wantR, wantG, wantB := pixelAt(t, img, b.Min.X+b.Dx()/2, b.Min.Y+2)
	t.Logf("%s: fenced block %.0fx%.0f, %d line-box fragment(s), %d gap(s), background %s "+
		"(sampled rgb(%d,%d,%d) in the top padding)",
		scheme, m.Rect.W, m.Rect.H, m.NRects, len(m.Gaps), m.BG, wantR, wantG, wantB)

	// The sampled columns: strictly inside the pre's left padding and short of
	// its right border, so the pre's OWN border is never in the sample. Glyphs
	// are irrelevant here — the rows sampled are BETWEEN two line boxes, where
	// no glyph is painted except the odd descender, which is why the failure
	// message names the offending row rather than a count alone.
	x0 := b.Min.X + int(m.PadLeft*scale) + 2
	x1 := b.Max.X - int(m.PadLeft*scale) - 2
	if x1-x0 < 20 {
		t.Fatalf("%s: only %d device column(s) lie inside the block's padding; the row sample "+
			"would be too narrow to mean anything", scheme, x1-x0)
	}

	checked := 0
	for _, gap := range m.Gaps {
		// A 1px CSS seam sits within half a pixel of the fragment boundary, so
		// the two CSS pixels straddling the gap are the rows to read.
		for _, off := range []float64{-1, -0.5, 0, 0.5} {
			y := int((gap + off) * scale)
			if y < b.Min.Y || y >= b.Max.Y {
				continue
			}
			checked++
			var worstX, worstD int
			for x := x0; x < x1; x++ {
				r, g, bl := pixelAt(t, img, x, y)
				d := maxInt3(iabsSeam(r-wantR), iabsSeam(g-wantG), iabsSeam(bl-wantB))
				if d > worstD {
					worstD, worstX = d, x
				}
			}
			if worstD > seamTolerance {
				r, g, bl := pixelAt(t, img, worstX, y)
				t.Errorf("%s: device row %d (css y=%.2f in the block, between two line boxes of "+
					"the fenced <code>) paints rgb(%d,%d,%d) at x=%d, which is %d off the block's "+
					"own background rgb(%d,%d,%d). Something is drawn between the lines of the "+
					"code block — the shape of the sliced inline-code border this test exists for.",
					scheme, y, float64(y)/scale, r, g, bl, worstX, worstD, wantR, wantG, wantB)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("%s: no device row fell inside the captured block, so no pixel was compared",
			scheme)
	}
	t.Logf("%s: %d device row(s) between line boxes are within %d/channel of the block's "+
		"background", scheme, checked, seamTolerance)
}

func pixelAt(t *testing.T, img image.Image, x, y int) (r, g, b int) {
	t.Helper()
	cr, cg, cb, _ := img.At(x, y).RGBA()
	return int(cr >> 8), int(cg >> 8), int(cb >> 8)
}

func iabsSeam(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt3(a, b, c int) int {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}
