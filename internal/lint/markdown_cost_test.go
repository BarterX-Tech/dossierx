package lint

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// This file bounds markdown-sanity's COST, and it exists because the lint is a
// mirror of internal/render/markdown — which means it inherits that package's
// quadratic paths unless it deliberately does not.
//
// THE UNTRUSTED SURFACE IS THE SAME ONE. Reviewer-authored comment bodies are
// capped at 1 MiB by internal/serve's maxBodyBytes and are stored in the claim
// file, so every one of them is read back by the loader and walked by this
// lint on every "dossierx check", every "dossierx lock" (internal/lock.Lock
// runs the whole registry over the whole corpus for every single lock), and
// every reaudit. A quadratic here is not milder than a quadratic in the
// renderer — it is the same bytes on a path that runs more often.
//
// The renderer's four cost repairs are mirrored in markdown_scan.go:
// mdCloserRuns (fence openers that never close), mdInlineIndex (backtick runs
// that never close, and brackets that never complete a link), and the
// segment-accumulator shape of mdRun (prose folded into a run is appended, and
// joined exactly once). This sweep is what keeps all four honest.
//
// WHY A SHAPE SWEEP. A guard that pins one literal body only ever catches that
// body again — the renderer's own cost file records a shape that was 417x
// slower one space to the right of the pinned guard, which stayed green. So
// this is a table of generators measured at four increasing sizes, asserting
// the GROWTH RATIO stays near-linear, with an absolute 1 MiB budget alongside
// it because the attacker's budget is an absolute one.

// lintOneMiB is internal/serve's maxBodyBytes: the largest single body that can
// reach this lint from the untrusted surface.
const lintOneMiB = 1 << 20

// lintCostShape is one body generator plus the path it reaches, so a failure
// names the defect instead of just a number.
type lintCostShape struct {
	name string
	why  string
	gen  func(n int) string
	// inComment routes the body through a comment instead of the claim Body,
	// which is the surface internal/serve's 1 MiB cap actually governs.
	inComment bool
}

func lintCostShapes() []lintCostShape {
	return []lintCostShape{
		{
			name: "fence-openers-never-close",
			why:  "mdScanFence's walk to end-of-document per opener, bounded by mdCloserRuns",
			gen:  func(n int) string { return lintRepeatTo("```go\n", n) },
		},
		{
			name: "fence-openers-indented-under-an-item",
			why:  "the same walk on the item-content path, one space to the right",
			gen:  func(n int) string { return "- x\n" + lintRepeatTo("  ```go\n", n) },
		},
		{
			name: "backtick-runs-never-close",
			why:  "mdFindBacktickRun's re-walk per opener, bounded by mdInlineIndex",
			gen:  lintDistinctBacktickRuns,
		},
		{
			name: "bare-bracket-openers",
			why:  "parseLink's two IndexByte searches per '[', bounded by mdInlineIndex",
			gen:  func(n int) string { return lintRepeatTo("[", n) },
		},
		{
			name: "empty-link-openers",
			why:  "the same searches, with a ']' that always resolves and a ')' that never does",
			gen:  func(n int) string { return lintRepeatTo("[](", n) },
		},
		{
			name: "partial-link-openers",
			why:  "the same searches, one byte of link text further along",
			gen:  func(n int) string { return lintRepeatTo("[a](", n) },
		},
		{
			name: "image-openers-never-complete",
			why:  "the image branch's parseLink attempt, which consumes nothing on failure",
			gen:  func(n int) string { return lintRepeatTo("![a](", n) },
		},
		{
			name: "list-continuations",
			why:  "mdRun's segment accumulator and mdRun.joined's single join",
			gen:  func(n int) string { return "- x\n" + lintRepeatTo("  a\n", n) },
		},
		{
			name: "list-continuations-across-blank-lines",
			why:  "the same accumulator on the loose-list path a blank line arms",
			gen:  func(n int) string { return "- x\n" + lintRepeatTo("\n  a\n", n) },
		},
		{
			name: "unresolvable-indents",
			why:  "the level stack's pop loop, once per marker",
			gen:  lintSawtoothIndents,
		},
		{
			name: "pipe-bearing-lines",
			why:  "the table recognizer's two-line lookahead per column-zero line",
			gen:  func(n int) string { return lintRepeatTo("a | b | c\n", n) },
		},
		{
			name: "table-rows",
			why:  "mdSplitRow per body row of one recognized table",
			gen:  func(n int) string { return "| a | b |\n| --- | --- |\n" + lintRepeatTo("| 1 | 2 |\n", n) },
		},
		{
			name: "blockquote-interior",
			why:  "the recursive re-scan of a quote's interior",
			gen:  func(n int) string { return lintRepeatTo("> a `b\n", n) },
		},
		{
			name:      "comment-body-bracket-openers",
			why:       "mdImagesIn over a reviewer-authored comment, the 1 MiB-capped surface",
			gen:       func(n int) string { return lintRepeatTo("[a](", n) },
			inComment: true,
		},
		{
			name:      "comment-body-backtick-runs",
			why:       "the same path, reaching the backtick index instead",
			gen:       lintDistinctBacktickRuns,
			inComment: true,
		},
	}
}

// claimFor wraps a generated body in the claim shape the shape asks for.
func (sh lintCostShape) claimFor(n int) model.Claim {
	body := sh.gen(n)
	if sh.inComment {
		return model.Claim{
			ID:       "widget.contract.hostile",
			Comments: []model.Comment{{ID: "c1", Body: body}},
		}
	}
	return model.Claim{ID: "widget.contract.hostile", Body: body}
}

// lintCostSweepSizes spans 8x, so linear is ~8x and quadratic is ~64x. The
// sizes match internal/render/markdown's own sweep deliberately: they start
// large enough that fixed per-call costs and one GC cycle do not dominate the
// smallest measurement and inflate the ratio.
var lintCostSweepSizes = []int{64 << 10, 128 << 10, 256 << 10, 512 << 10}

// lintCostGrowthLimit is the ratio between the largest and smallest size. It
// sits well above linear's ~8x and well below quadratic's ~64x, so it fails on
// a reintroduced rescan and not on a slow machine. Same value the renderer's
// sweep uses, for the same shapes.
const lintCostGrowthLimit = 25.0

// lintCostMeasurementCeiling stops the sweep before it spends minutes
// confirming what the first two sizes already showed.
const lintCostMeasurementCeiling = 2 * time.Second * lintCostTimeScale

// TestMarkdownSanity_CostScalesLinearlyAcrossShapes is the growth half of the
// guard: every shape is linted at four increasing sizes and must not cost
// anything like the square of its input.
func TestMarkdownSanity_CostScalesLinearlyAcrossShapes(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	for _, sh := range lintCostShapes() {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			times := make([]time.Duration, 0, len(lintCostSweepSizes))
			for _, n := range lintCostSweepSizes {
				d, ok := lintBestCheckTime(sh.claimFor(n))
				times = append(times, d)
				if !ok {
					t.Fatalf("shape %q: one lint of %d bytes took %v, past the %v measurement "+
						"ceiling — this shape is superlinear and the sweep stops here\n"+
						"  measurements: %s\n  this shape reaches: %s",
						sh.name, n, d, lintCostMeasurementCeiling, lintFormatSweep(lintCostSweepSizes, times), sh.why)
				}
			}
			ratio := float64(times[len(times)-1]) / float64(times[0])
			t.Logf("%s: %s  (span 8x, ratio %.1fx)", sh.name, lintFormatSweep(lintCostSweepSizes, times), ratio)
			if ratio > lintCostGrowthLimit {
				t.Errorf("shape %q: 8x the bytes cost %.1fx the time "+
					"(linear is ~8x, quadratic ~64x, limit %.0fx)\n"+
					"  measurements: %s\n  this shape reaches: %s",
					sh.name, ratio, lintCostGrowthLimit, lintFormatSweep(lintCostSweepSizes, times), sh.why)
			}
		})
	}
}

// TestMarkdownSanity_CostAtOneMiBIsBounded is the absolute half: the
// attacker's real budget is not a ratio, it is one 1 MiB body. A shape could
// in principle grow linearly and still be ruinously expensive per byte, and a
// growth ratio would never say so.
func TestMarkdownSanity_CostAtOneMiBIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	if lintRaceEnabled {
		t.Skip("absolute wall-clock budget is not meaningful under -race; the growth sweep still runs")
	}
	const budget = 1500 * time.Millisecond
	for _, sh := range lintCostShapes() {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			claim := sh.claimFor(lintOneMiB)
			start := time.Now()
			MarkdownSanity{}.Check([]model.Claim{claim}, nil)
			if elapsed := time.Since(start); elapsed > budget {
				t.Errorf("shape %q: 1 MiB took %v, over the %v budget\n  this shape reaches: %s",
					sh.name, elapsed, budget, sh.why)
			}
		})
	}
}

// TestAssetScope_CostAtOneMiBIsBounded holds the second lint to the same
// budget. It walks the same scanner over the same bodies, so a bound that
// covered only markdown-sanity would leave the corpus just as expensive to
// lock.
func TestAssetScope_CostAtOneMiBIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	if lintRaceEnabled {
		t.Skip("absolute wall-clock budget is not meaningful under -race")
	}
	const budget = 1500 * time.Millisecond
	for _, sh := range lintCostShapes() {
		if sh.inComment {
			continue // asset-scope does not read comments
		}
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			claim := sh.claimFor(lintOneMiB)
			claim.SourcePath = "claims/widget/hostile.yaml"
			start := time.Now()
			AssetScope{}.Check([]model.Claim{claim}, nil)
			if elapsed := time.Since(start); elapsed > budget {
				t.Errorf("shape %q: 1 MiB took %v, over the %v budget\n  this shape reaches: %s",
					sh.name, elapsed, budget, sh.why)
			}
		})
	}
}

// --- generators and measurement -------------------------------------------

// lintRepeatTo repeats unit until the result is at least n bytes.
func lintRepeatTo(unit string, n int) string {
	if len(unit) == 0 {
		return ""
	}
	return strings.Repeat(unit, n/len(unit)+1)
}

// lintDistinctBacktickRuns builds a paragraph of backtick runs of strictly
// increasing length, so no run can ever close another — the shape that made
// the renderer's span search superlinear.
func lintDistinctBacktickRuns(n int) string {
	var b strings.Builder
	for run := 1; b.Len() < n; run++ {
		b.WriteString(strings.Repeat("`", run))
		b.WriteString(" x ")
		if run > 64 {
			run = 0 // restart the ladder rather than growing one run forever
		}
	}
	return b.String()
}

// lintSawtoothIndents builds list markers whose indents never match an open
// level, so every line takes the stack's pop-and-snap path.
func lintSawtoothIndents(n int) string {
	var b strings.Builder
	depth := 0
	for b.Len() < n {
		b.WriteString(strings.Repeat(" ", depth))
		b.WriteString("- x\n")
		depth += 3
		if depth > 30 {
			depth = 1
		}
	}
	return b.String()
}

// lintBestCheckTime runs the lint a few times and keeps the fastest, so one
// scheduling hiccup does not become a false quadratic. ok is false when even
// the fastest run passed the measurement ceiling.
func lintBestCheckTime(claim model.Claim) (time.Duration, bool) {
	claims := []model.Claim{claim}
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 3; i++ {
		start := time.Now()
		MarkdownSanity{}.Check(claims, nil)
		if d := time.Since(start); d < best {
			best = d
		}
		if best > lintCostMeasurementCeiling {
			return best, false
		}
	}
	return best, true
}

func lintFormatSweep(sizes []int, times []time.Duration) string {
	parts := make([]string, 0, len(times))
	for i, d := range times {
		parts = append(parts, fmt.Sprintf("%dB=%v", sizes[i], d))
	}
	return strings.Join(parts, "  ")
}
