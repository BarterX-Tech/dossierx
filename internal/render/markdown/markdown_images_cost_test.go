package markdown

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// markdown_images_cost_test.go is the cost guard for the ONE entry point the
// main sweep cannot reach.
//
// TestRender_CostScalesLinearlyAcrossShapes, TestRender_CostAtOneMiBIsBounded
// and TestRender_AllocationAtOneMiBIsBounded all measure Render, which is the
// right entry point to measure: it is what internal/serve calls on a stored
// 1 MiB comment body, and it is where a hostile input actually arrives. Under
// Render the image capability is OFF, so every image shape in that sweep
// measures the REFUSAL path — the parse, the gate, and the fall-through to
// literal text.
//
// It never measures the path that EMITS. A claim body is not a hostile surface
// the way a comment body is (a claim file is authored by whoever can write the
// repo), but it is reachable at the same 1 MiB scale, it is re-rendered on every
// GET / — and, unlike every construct before it, an image is markup this package
// emits from a very short source: "![](assets/a.png)" is 17 bytes in and 71 out.
// That ratio is the reason this file exists. A budget that was never evaluated
// at the shape that maximises it is not a budget.
//
// The three guards here are the same three, run through RenderClaimBody with a
// production-shaped prefix. They share the sweep's shapes and its budgets so a
// regression reads the same way in both places.

// claimBodyCostPrefix is the AssetPrefix the guards below render with. It is
// deliberately a REALISTIC length rather than a short one: the prefix is emitted
// once per accepted image, so it is part of the construct's markup ratio, and a
// one-byte prefix would measure a page nobody serves.
const claimBodyCostPrefix AssetPrefix = "/claim-assets/widget.contract.retry-policy/"

// TestRenderClaimBody_ImageCostScalesLinearly is the growth half. Every shape in
// the main sweep runs through RenderClaimBody at the same four sizes and must
// not cost anything like the square of its input. Running the WHOLE sweep, not
// just the image rows, is what proves the threading did not slow another
// construct down: the policy is a value copied into every container, and a
// construct that started allocating per line because of it would show up here.
func TestRenderClaimBody_ImageCostScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	for _, sh := range costShapes() {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			span := costSweepSizes[len(costSweepSizes)-1] / costSweepSizes[0]
			var times []time.Duration
			var ratio float64
			for attempt := 1; attempt <= costGrowthAttempts; attempt++ {
				times = make([]time.Duration, 0, len(costSweepSizes))
				for _, n := range costSweepSizes {
					d, ok := bestClaimBodyTime(sh.gen(n))
					times = append(times, d)
					if !ok {
						t.Fatalf("shape %q: one RenderClaimBody of %d bytes took %v, past the %v "+
							"measurement ceiling — this shape is superlinear with images ON\n"+
							"  measurements: %s\n  this shape reaches: %s",
							sh.name, n, d, costMeasurementCeiling,
							formatSweep(costSweepSizes[:len(times)], times), sh.why)
					}
				}
				ratio = float64(times[len(times)-1]) / float64(times[0])
				t.Logf("%s: %s  (ratio %.1fx, attempt %d/%d)",
					sh.name, formatSweep(costSweepSizes, times), ratio, attempt, costGrowthAttempts)
				if ratio <= costGrowthLimit {
					return
				}
			}
			t.Errorf("shape %q: %dx the bytes cost %.1fx the time through RenderClaimBody "+
				"on all %d attempts (linear is ~8x, quadratic ~64x, limit %.0fx)\n"+
				"  measurements: %s\n  this shape reaches: %s",
				sh.name, span, ratio, costGrowthAttempts, costGrowthLimit,
				formatSweep(costSweepSizes, times), sh.why)
		})
	}
}

// TestRenderClaimBody_ImageCostAtOneMiBIsBounded is the absolute wall-clock and
// allocation half, at the cap, with the capability ON.
//
// It reuses oneMiBAllocBudget — 192 MiB — deliberately rather than granting the
// new construct its own looser number. The budget's own doc comment says why a
// budget is only ever sized against the shapes somebody thought to sweep; giving
// images a private budget would be exactly the move that comment warns about.
func TestRenderClaimBody_ImageCostAtOneMiBIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	if raceEnabled {
		// Same reasoning as the Render-side budgets: under -race this would
		// measure the instrumentation. The growth sweep above still runs.
		t.Skip("absolute budgets are not meaningful under -race; the growth sweep still runs")
	}
	const budget = 750 * time.Millisecond
	for _, sh := range costShapes() {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			body := sh.gen(oneMiB)

			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			start := time.Now()
			out := string(RenderClaimBody(body, claimBodyCostPrefix))
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			used := after.TotalAlloc - before.TotalAlloc

			t.Logf("%s: %d bytes in, %d out, %v, %.1f MiB allocated",
				sh.name, len(body), len(out), elapsed, float64(used)/(1<<20))

			if elapsed > budget {
				t.Errorf("shape %q: %d bytes took %v through RenderClaimBody, over the %v budget\n"+
					"  this shape reaches: %s", sh.name, len(body), elapsed, budget, sh.why)
			}
			if used > oneMiBAllocBudget {
				t.Errorf("shape %q: %d bytes allocated %.1f MiB through RenderClaimBody, over the %.0f MiB budget\n"+
					"  this shape reaches: %s", sh.name, len(body), float64(used)/(1<<20),
					float64(oneMiBAllocBudget)/(1<<20), sh.why)
			}
			if strings.Contains(out, "<script") {
				t.Fatalf("shape %q: unescaped markup leaked", sh.name)
			}
			assertTagBalance(t, out)
		})
	}
}

// TestClaimBodyImages_CostAtOneMiBIsBounded bounds the ALLOWLIST BUILDER, which
// is a third cost surface and one nothing else in this package measures.
//
// internal/serve rebuilds its image allowlist by running ClaimBodyImages over
// every loaded claim's body and steps, and it rebuilds on every claim-file
// change the watcher sees. That is one full render's worth of work per claim,
// discarded — so if it were ever more than that, a corpus of large bodies would
// turn a file save into a stall. This says it is not: collecting costs no more
// than rendering, at the cap, on the shape that has the most to collect.
func TestClaimBodyImages_CostAtOneMiBIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	if raceEnabled {
		t.Skip("absolute budgets are not meaningful under -race")
	}
	// The shape with one accepted image per 17 source bytes: the most refs a
	// 1 MiB body can produce, and therefore the largest result slice.
	body := repeatTo("![](assets/a.png)", oneMiB)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	refs := ClaimBodyImages(body)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	used := after.TotalAlloc - before.TotalAlloc
	runtime.KeepAlive(refs)

	t.Logf("ClaimBodyImages: %d bytes in, %d refs, %v, %.1f MiB allocated",
		len(body), len(refs), elapsed, float64(used)/(1<<20))

	if elapsed > 750*time.Millisecond {
		t.Errorf("ClaimBodyImages of %d bytes took %v, over the 750ms budget", len(body), elapsed)
	}
	if used > oneMiBAllocBudget {
		t.Errorf("ClaimBodyImages of %d bytes allocated %.1f MiB, over the %.0f MiB budget",
			len(body), float64(used)/(1<<20), float64(oneMiBAllocBudget)/(1<<20))
	}
	if len(refs) == 0 {
		t.Fatal("the shape must produce refs, or this measured nothing")
	}
}

// bestClaimBodyTime is bestRenderTime for the claim-body entry point: best-of-N,
// because the number is a denominator and noise only ever adds.
func bestClaimBodyTime(body string) (best time.Duration, ok bool) {
	// measurePerOp rather than a single timed call: Windows' clock granularity
	// rounds a fast RenderClaimBody to exactly zero, which made times[0] zero
	// and the growth ratio +Inf. See costTimerFloor in markdown_cost_test.go.
	for i := 0; i < claimBodyCostRuns; i++ {
		d, measured := measurePerOp(func() {
			out := RenderClaimBody(body, claimBodyCostPrefix)
			runtime.KeepAlive(out)
		}, costMeasurementCeiling)
		if !measured {
			return d, false
		}
		if best == 0 || d < best {
			best = d
		}
	}
	return best, true
}

const claimBodyCostRuns = 3
