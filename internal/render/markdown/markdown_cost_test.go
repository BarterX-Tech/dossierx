package markdown

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// This file guards the COST regressions. None of them ever produced a wrong
// byte: every one was a quadratic-ish rescan or accumulator that failed no test
// while making a single request cost seconds-to-minutes of CPU. So each repair
// gets two kinds of test — one proving the fast path answers EXACTLY what the
// walk it replaced would have answered (correctness), and one bounding cost —
// plus benchmarks, so a future rewrite that reintroduces the rescan is visible.
//
// THE UNTRUSTED SURFACE. Reviewer-authored comment bodies are capped at 1 MiB
// by internal/serve's maxBodyBytes and reach Render at
// internal/serve/handlers.go:639 and :650. handleListComments re-renders EVERY
// stored comment on EVERY GET /api/comments, so one stored hostile body is
// permanently amplified across every later read. That is the budget every size
// below is chosen against — not a microbenchmark.
//
// WHY A SHAPE SWEEP AND NOT PINNED BODIES. This file used to pin one literal
// body shape per fix: column-zero "```go" openers, and backtick runs of
// increasing length. Both guards were one leading space away from useless — the
// same bytes indented by one column under an open list item took a completely
// different code path (item continuation instead of top-level paragraph) and
// were quadratic there while the pinned guard stayed green, and no bracket
// shape was covered at all. A guard that pins the one input the author happened
// to think of only ever catches that input again.
//
// So the cost guard is now a TABLE OF SHAPE GENERATORS measured at four
// increasing sizes, asserting the GROWTH RATIO stays near-linear rather than
// asserting one wall-clock number on one body. A growth assertion survives a
// change of machine, and adding a newly-suspected shape is one table row.
// A 1 MiB absolute-budget guard rides alongside it, because the attacker's real
// budget is an absolute one.

// oneMiB is internal/serve's maxBodyBytes: the largest body that can reach
// Render from the untrusted surface.
const oneMiB = 1 << 20

// --- the shape sweep ------------------------------------------------------

// costShape is one body generator. gen(n) returns a body of approximately n
// bytes; sizing in BYTES rather than in items is what makes shapes with very
// different line lengths comparable, and bytes are what the 1 MiB cap bounds.
//
// Every shape is measured through Render, the entry point internal/serve
// actually calls, so a shape is exercised through the same block scan, item
// accumulation and inline scan a hostile comment body would be.
type costShape struct {
	name string
	// why records which quadratic path this shape reaches, so a failure names
	// the defect instead of just a number.
	why string
	gen func(n int) string
}

// costShapes is the sweep. Each entry is a shape that has been, or plausibly
// could be, superlinear. Add a row rather than a bespoke test.
func costShapes() []costShape {
	return []costShape{
		{
			name: "list-continuation-unordered",
			why:  "itemNode.addText's prose accumulator (was string +=, O(K^2))",
			gen:  func(n int) string { return "- x\n" + repeatTo("  a\n", n) },
		},
		{
			name: "list-continuation-ordered",
			why:  "same accumulator, reached through an ordered item",
			gen:  func(n int) string { return "1. x\n" + repeatTo("   a\n", n) },
		},
		{
			name: "list-continuation-nested",
			why:  "same accumulator, reached through a nested item",
			gen:  func(n int) string { return "- x\n  - y\n" + repeatTo("    a\n", n) },
		},
		{
			name: "fence-openers-indented",
			why:  "unclosed indented fences fall through to addText, not the paragraph path",
			gen:  func(n int) string { return "- x\n" + repeatTo(" ```go\n", n) },
		},
		{
			name: "fence-openers-column-zero",
			why:  "scanFence's walk over the remainder for an opener that never closes",
			gen:  func(n int) string { return repeatTo("```go\n", n) },
		},
		{
			name: "fence-openers-blank-separated",
			why:  "unclosed openers with the loose-list rule armed between them",
			gen:  func(n int) string { return "- x\n" + repeatTo("\n ```go\n", n) },
		},
		{
			name: "brackets-bare",
			why:  "parseLink's IndexByte(']') rescan over the whole remainder",
			gen:  func(n int) string { return repeatTo("[", n) },
		},
		{
			name: "brackets-closed-no-paren",
			why:  "parseLink fails at the '(' test, consumes nothing, next '[' rescans",
			gen:  func(n int) string { return repeatTo("[]", n) },
		},
		{
			// The shape that proves the link index must be built on ANY
			// failure rather than only on one that ran off the end: here
			// every IndexByte SUCCEEDS, and the scan is still quadratic
			// because each of the N brackets walks to the same distant ']'
			// only to be rejected by the '(' test and consume nothing.
			name: "brackets-distant-close-no-paren",
			why:  "parseLink walks the whole remainder to a ']' that fails the '(' test",
			gen:  func(n int) string { return repeatTo("[", n) + "]x" },
		},
		{
			name: "brackets-open-paren-empty-text",
			why:  "parseLink's IndexByte(')') rescan over the whole remainder",
			gen:  func(n int) string { return repeatTo("[](", n) },
		},
		{
			name: "brackets-open-paren-with-text",
			why:  "same ')' rescan, with a non-empty link text",
			gen:  func(n int) string { return repeatTo("[a](", n) },
		},
		{
			name: "backtick-runs-increasing",
			why:  "a code-span opener whose search fails consumes nothing and re-walks",
			gen:  distinctBacktickRuns,
		},
		{
			name: "list-indent-deepening",
			why:  "indent scanning and list-state churn under ever-deeper indentation",
			gen:  deepeningIndentedList,
		},
		{
			name: "list-blank-line-runs",
			why:  "the loose-list rule's pendingBlank arming, repeated",
			gen:  func(n int) string { return "- x\n" + repeatTo("\n", n) },
		},
		{
			name: "list-blank-then-continuation",
			why:  "blank/continuation alternation: the exact shape the loose-list rule made reachable",
			gen:  func(n int) string { return "- x\n" + repeatTo("\n  a\n", n) },
		},
		// --- phase B constructs ---------------------------------------
		//
		// Every construct added at P4-P8 gets a row here BEFORE it is
		// believed to be linear, because each one added either a new
		// per-line search, a new recursion, or a new per-block join, and
		// each of those is exactly the shape of the four defects above.
		{
			name: "blockquote-one-long-quote",
			why:  "the blockquote recursion re-scans its interior: one closerRuns table per quote",
			gen:  func(n int) string { return repeatTo("> x\n", n) },
		},
		{
			name: "blockquote-many-short-quotes",
			why:  "one recursion, one line slice and one closerRuns table PER quote",
			gen:  func(n int) string { return repeatTo("> x\n\ny\n\n", n) },
		},
		{
			name: "blockquote-nested-literal",
			why:  "a second > is literal text, so it must not re-enter the quote scanner",
			gen:  func(n int) string { return repeatTo("> > x\n", n) },
		},
		{
			name: "list-nesting-deepening",
			why:  "the depth stack: one push per level, and writeList recurses to that depth",
			gen:  deepeningNestedList,
		},
		{
			name: "list-dedent-churn",
			why:  "the pop loops: a dedent closes levels, and pops must be amortized by pushes",
			gen:  func(n int) string { return repeatTo("- a\n  - b\n    - c\n- d\n", n) },
		},
		{
			name: "hard-breaks-backslash",
			why:  "joinSegments builds one block string plus one break offset per line",
			gen:  func(n int) string { return repeatTo("a\\\n", n) },
		},
		{
			name: "hard-breaks-two-spaces",
			why:  "the same accumulation reached through the trailing-space spelling",
			gen:  func(n int) string { return repeatTo("a  \n", n) },
		},
		{
			name: "hard-breaks-in-list-item",
			why:  "breaks threaded through item continuation, where the O(K^2) accumulator lived",
			gen:  func(n int) string { return "- x\n" + repeatTo("  a\\\n", n) },
		},
		{
			name: "hard-breaks-with-unclosed-backticks",
			why:  "every code-span opener bounds its search with a lookup over ALL break offsets",
			gen:  func(n int) string { return repeatTo("`x\\\n", n) },
		},
		{
			name: "headings",
			why:  "one heading recognition and one inline pass per line",
			gen:  func(n int) string { return repeatTo("### h\n", n) },
		},
		{
			name: "thematic-breaks",
			why:  "a rule flushes both open blocks on every line",
			gen:  func(n int) string { return repeatTo("---\n", n) },
		},
		{
			name: "task-items",
			why:  "the checkbox scan runs on every unordered item's text",
			gen:  func(n int) string { return repeatTo("- [x] a\n", n) },
		},
		{
			name: "loose-list-items",
			why:  "looseness makes every item re-enter the <p> path at flush time",
			gen:  func(n int) string { return repeatTo("- a\n\n", n) },
		},
		{
			name: "prose-control",
			why:  "the ordinary shape: it must not have paid for any of the repairs",
			gen: func(n int) string {
				return repeatTo("The `id` field is required; see [the docs](https://x/y).\n\n", n)
			},
		},
	}
}

// repeatTo repeats unit until the result is at least n bytes.
func repeatTo(unit string, n int) string {
	return strings.Repeat(unit, n/len(unit)+1)
}

// distinctBacktickRuns builds a paragraph of backtick runs of strictly
// increasing length, separated by a non-backtick byte, up to about n bytes.
// No two runs share a length, so no code span can ever close and every opener's
// search fails.
func distinctBacktickRuns(n int) string {
	var sb strings.Builder
	sb.Grow(n + 64)
	for run := 1; sb.Len() < n; run++ {
		sb.WriteString(strings.Repeat("`", run))
		sb.WriteByte('x')
	}
	return sb.String()
}

// deepeningIndentedList builds a list whose every line is indented one column
// deeper than the last, to about n bytes. Line count grows as sqrt(n), so a
// per-line cost that is linear in the line's own indent is still linear in
// BYTES — which is the property being asserted.
func deepeningIndentedList(n int) string {
	var sb strings.Builder
	sb.Grow(n + 64)
	sb.WriteString("- x\n")
	for depth := 1; sb.Len() < n; depth++ {
		sb.WriteString(strings.Repeat(" ", depth))
		sb.WriteString("- y\n")
	}
	return sb.String()
}

// deepeningNestedList builds a list that genuinely OPENS a level on every
// line — each marker is indented two columns past the previous item's content
// column — to about n bytes. deepeningIndentedList's one-column steps do not
// reach a content column and so stay flat; this one exercises the depth stack
// and writeList's recursion at their real depth. Line count grows as sqrt(n),
// so per-line work linear in the line's own indent is still linear in BYTES.
func deepeningNestedList(n int) string {
	var sb strings.Builder
	sb.Grow(n + 64)
	for depth := 0; sb.Len() < n; depth++ {
		sb.WriteString(strings.Repeat(" ", 2*depth))
		sb.WriteString("- a\n")
	}
	return sb.String()
}

// costSweepSizes are the four measurement points, an 8x span. Linear work
// across an 8x span costs ~8x; the quadratic paths this file guards measured a
// consistent ~4x per doubling, so ~64x across the same span.
//
// The base is deliberately large (64 KiB, a sixteenth of the attacker's cap)
// so that even the cheapest shape's smallest measurement is far above timer
// resolution — a growth ratio computed from a noisy denominator is a flake
// generator, not a guard.
var costSweepSizes = []int{64 << 10, 128 << 10, 256 << 10, 512 << 10}

// costGrowthLimit is the ceiling on t(512 KiB)/t(64 KiB). Linear is ~8.
// Quadratic is ~64. 25 splits them with room for the constant-factor and
// allocator effects that make even a linear scan measure somewhat above 8x
// (larger inputs cost more per byte in cache misses and GC), and still sits
// well under every quadratic ratio this file has measured.
const costGrowthLimit = 25.0

// TestRender_CostScalesLinearlyAcrossShapes is the growth half of the guard.
// Every shape in the sweep is rendered at four increasing sizes and must not
// cost anything like the square of its input.
//
// This replaces two tests that each pinned one literal body. The failure of the
// pinned form was not that it was wrong but that it was narrow: the identical
// bytes indented by one space took a different path through renderBlocks and
// were 417x slower there while the pinned guard stayed green.
func TestRender_CostScalesLinearlyAcrossShapes(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	for _, sh := range costShapes() {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			times := make([]time.Duration, 0, len(costSweepSizes))
			sizes := make([]int, 0, len(costSweepSizes))
			for _, n := range costSweepSizes {
				d, ok := bestRenderTime(sh.gen(n))
				times = append(times, d)
				sizes = append(sizes, n)
				if !ok {
					t.Errorf("shape %q: one render of %d bytes took %v, past the %v "+
						"measurement ceiling — this shape is superlinear and the sweep "+
						"stops here rather than measuring the larger sizes\n"+
						"  measurements: %s\n"+
						"  this shape reaches: %s",
						sh.name, n, d, costMeasurementCeiling, formatSweep(sizes, times), sh.why)
					return
				}
			}
			ratio := float64(times[len(times)-1]) / float64(times[0])
			t.Logf("%s: %s  (span %dx, ratio %.1fx)",
				sh.name, formatSweep(costSweepSizes, times),
				costSweepSizes[len(costSweepSizes)-1]/costSweepSizes[0], ratio)
			if ratio > costGrowthLimit {
				t.Errorf("shape %q: %dx the bytes cost %.1fx the time "+
					"(linear is ~8x, quadratic ~64x, limit %.0fx)\n"+
					"  measurements: %s\n"+
					"  this shape reaches: %s",
					sh.name, costSweepSizes[len(costSweepSizes)-1]/costSweepSizes[0],
					ratio, costGrowthLimit, formatSweep(costSweepSizes, times), sh.why)
			}
		})
	}
}

// TestRender_CostAtOneMiBIsBounded is the absolute half of the guard: the
// attacker's real budget is not a ratio, it is one 1 MiB comment body. A shape
// could in principle grow linearly and still be ruinously expensive per byte,
// and a growth ratio would never say so.
//
// The budget is loose against the measured cost — every shape here renders
// 1 MiB in under 25ms, so it has more than an order of magnitude of headroom
// for a CI box several times slower than a laptop — while still sitting under
// every pre-repair measurement of these same shapes at this same size: 5.0s
// for list continuation, 4.7s for indented fences, and 1.5s to 18s+ for the
// bracket shapes. That last figure is why the budget is not looser still: at
// 1.5s one genuinely quadratic shape squeaked under it.
func TestRender_CostAtOneMiBIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	if raceEnabled {
		// Not a flake being papered over: under -race every shape here is
		// ~41x slower, so this budget would measure the race detector rather
		// than the renderer. The growth sweep still runs under -race and
		// still catches every defect this file guards, because a ratio of two
		// equally-instrumented measurements is unaffected.
		t.Skip("absolute wall-clock budget is not meaningful under -race; the growth sweep still runs")
	}
	const budget = 750 * time.Millisecond
	for _, sh := range costShapes() {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			body := sh.gen(oneMiB)
			out, elapsed, finished := renderWithin(body, costMeasurementCeiling)
			if !finished {
				t.Fatalf("shape %q: %d bytes had not rendered after %v (budget %v)\n"+
					"  this shape reaches: %s", sh.name, len(body), elapsed, budget, sh.why)
			}
			t.Logf("%s: %d bytes in %v", sh.name, len(body), elapsed)
			if elapsed > budget {
				t.Errorf("shape %q: %d bytes took %v, over the %v budget\n"+
					"  this shape reaches: %s", sh.name, len(body), elapsed, budget, sh.why)
			}
			// Cheap belt-and-braces: a hostile body at the cap must still
			// come out escaped and structurally sound.
			if strings.Contains(out, "<script") {
				t.Fatalf("shape %q: unescaped markup leaked", sh.name)
			}
			assertTagBalance(t, out)
		})
	}
}

// TestRender_UnclosedOpenersAreNotFences pins the output invariant the two
// fence shapes in the sweep depend on: a body of openers that never close
// contains no fence at all, at either indentation. Without this, a "fix" that
// made the sweep fast by treating an unclosed opener as a closed fence would
// pass the cost guard while changing what the page shows.
func TestRender_UnclosedOpenersAreNotFences(t *testing.T) {
	for _, body := range []string{
		strings.Repeat("```go\n", 500),
		"- x\n" + strings.Repeat(" ```go\n", 500),
	} {
		if out := string(Render(body)); strings.Contains(out, "<pre>") {
			t.Errorf("unclosed openers must not produce a fence: %.80s", out)
		}
	}
}

// TestRenderInline_UnpairedRunsDoNotPair is the same output invariant for the
// backtick shape: runs of all-different lengths never close each other.
func TestRenderInline_UnpairedRunsDoNotPair(t *testing.T) {
	out := string(RenderInline(distinctBacktickRuns(64 << 10)))
	if strings.Contains(out, "<code>") {
		t.Errorf("runs of all-different lengths must not pair into a span")
	}
}

// costMeasurementCeiling bounds how long this guard may take to FAIL. Every
// shape here renders its largest sweep size in about ten milliseconds; a single
// render two hundred times slower than that has already proved the point, and
// repeating it four more times — then going on to the next, larger size — only
// makes the guard slower to say so. A cost guard that takes ten minutes to fail
// gets read as a CI hang and disabled, which is the same as not having one.
const costMeasurementCeiling = 2 * time.Second * costTimeScale

// bestRenderTime is best-of-N: scheduler noise, GC and CPU frequency scaling
// only ever ADD time, so the minimum of several runs is the most stable
// estimator of the real cost — which matters when the number is a denominator.
//
// It returns ok=false when even the first render blew the measurement ceiling,
// so the caller can stop measuring and report immediately.
func bestRenderTime(body string) (best time.Duration, ok bool) {
	start := time.Now()
	_ = Render(body)
	best = time.Since(start)
	if best > costMeasurementCeiling {
		return best, false
	}
	for r := 1; r < 5; r++ {
		start = time.Now()
		_ = Render(body)
		if d := time.Since(start); d < best {
			best = d
		}
	}
	return best, true
}

// renderWithin runs Render(body) with a deadline, so a shape that has gone
// quadratic FAILS the guard instead of hanging it. Same reasoning as
// costMeasurementCeiling: at 1 MiB the pre-repair shapes took 5 to 20 seconds
// each and there are fifteen of them, and a test binary that stops producing
// output for several minutes is indistinguishable from a wedged one.
//
// The abandoned goroutine is left to finish on its own. Render is a pure
// function over a string with no I/O, no locks and no shared state, so an
// abandoned one burns CPU until it returns and affects nothing else; the test
// has already failed by then.
func renderWithin(body string, limit time.Duration) (out string, elapsed time.Duration, ok bool) {
	type result struct {
		out     string
		elapsed time.Duration
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		s := string(Render(body))
		done <- result{s, time.Since(start)}
	}()
	select {
	case r := <-done:
		return r.out, r.elapsed, true
	case <-time.After(limit):
		return "", time.Since(start), false
	}
}

func formatSweep(sizes []int, times []time.Duration) string {
	parts := make([]string, len(sizes))
	for i := range sizes {
		parts[i] = fmt.Sprintf("%dKiB=%v", sizes[i]>>10, times[i].Round(time.Microsecond))
	}
	return strings.Join(parts, " ")
}

// --- fence scanning: correctness of the fast path -------------------------

// scanFenceByWalk is the pre-fix scanFence: no suffix-maximum table, so every
// opener that never closes walks all remaining lines. It is the reference
// TestScanFence_TableMatchesWalk holds the fast path to.
func scanFenceByWalk(lines []string, start, indent, openLen int) (content string, closeIdx int, closed bool) {
	var body []string
	for j := start + 1; j < len(lines); j++ {
		if fenceCloses(lines[j], openLen) {
			if len(body) == 0 {
				return "", j, true
			}
			return strings.Join(body, "\n") + "\n", j, true
		}
		body = append(body, stripIndent(lines[j], indent))
	}
	return "", start, false
}

// TestScanFence_TableMatchesWalk is the correctness half of the fence-cost
// fix: for every line of every corpus body, and for every opener length, the
// closerRuns-gated scanFence must return exactly what the plain walk returns.
// The table may only make the answer CHEAPER, never different.
func TestScanFence_TableMatchesWalk(t *testing.T) {
	for _, body := range costCorpus() {
		lines := strings.Split(body, "\n")
		closers := closerRuns(lines)
		for i := range lines {
			for openLen := 3; openLen <= 6; openLen++ {
				gotC, gotIdx, gotOK := scanFence(lines, closers, i, leadingSpaces(lines[i]), openLen)
				wantC, wantIdx, wantOK := scanFenceByWalk(lines, i, leadingSpaces(lines[i]), openLen)
				if gotC != wantC || gotIdx != wantIdx || gotOK != wantOK {
					t.Fatalf("scanFence(line %d, openLen %d) of %q\n got: (%q, %d, %v)\nwant: (%q, %d, %v)",
						i, openLen, body, gotC, gotIdx, gotOK, wantC, wantIdx, wantOK)
				}
			}
		}
	}
}

func BenchmarkRenderUnclosedFenceOpeners(b *testing.B) {
	body := strings.Repeat("```go\n", oneMiB/6)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Render(body)
	}
}

// --- code-span scanning: correctness of the fast path ---------------------

// TestBacktickIndex_MatchesLinearScan is the correctness half of the
// code-span cost fix: the index must answer exactly what findBacktickRun
// answers. Lookups only ever start at a run BOUNDARY (every construct in
// renderInline's scan advances past the whole run it consumed), so that is
// where the two are compared.
func TestBacktickIndex_MatchesLinearScan(t *testing.T) {
	for _, s := range costCorpus() {
		for _, from := range runBoundaries(s) {
			idx := newBacktickIndex(s, from)
			for n := 1; n <= 8; n++ {
				gotPos, gotOK := idx.find(from, n)
				wantPos, wantOK := findBacktickRun(s, from, n)
				if gotOK != wantOK || (gotOK && gotPos != wantPos) {
					t.Fatalf("backtickIndex.find(%d, %d) of %q = (%d, %v), findBacktickRun = (%d, %v)",
						from, n, s, gotPos, gotOK, wantPos, wantOK)
				}
			}
		}
	}
}

// runBoundaries returns every offset in s that is NOT strictly inside a
// backtick run — the only offsets renderInline ever searches from.
func runBoundaries(s string) []int {
	var out []int
	inRun := false
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			if !inRun {
				out = append(out, i)
			}
			inRun = true
			continue
		}
		inRun = false
		out = append(out, i)
	}
	return append(out, len(s))
}

func BenchmarkRenderInlineUnpairedBacktickRuns(b *testing.B) {
	body := distinctBacktickRuns(oneMiB)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderInline(body)
	}
}

// --- link scanning: correctness of the fast path --------------------------

// TestLinkIndex_MatchesParseLink is the correctness half of the link-cost fix,
// and it carries more weight than the other two differentials: parseLink's url
// output feeds allowedScheme/schemeOf, the package's security boundary. If the
// indexed path ever disagreed about where a link ends, it would disagree about
// which url is checked — so this asserts equality of ALL FOUR return values,
// including the verbatim url and link text, at every '[' of every corpus body.
//
// The index is built at the offset of the '[' whose parseLink first failed, and
// renderInline's scan only moves forward, so every lookup starts at or after
// the build offset. That is exactly the condition tested here: for each corpus
// body and each build offset, every later '[' must agree.
func TestLinkIndex_MatchesParseLink(t *testing.T) {
	for _, s := range costCorpus() {
		for _, from := range bracketOffsets(s) {
			x := newLinkIndex(s, from)
			for _, at := range bracketOffsets(s) {
				if at < from {
					continue
				}
				gotLen, gotText, gotURL, gotOK := x.parseLinkAt(s, at)
				wantLen, wantText, wantURL, wantOK := parseLink(s[at:])
				if gotOK != wantOK || gotLen != wantLen || gotText != wantText || gotURL != wantURL {
					t.Fatalf("parseLinkAt(%q, from=%d, at=%d) = (%d, %q, %q, %v), parseLink = (%d, %q, %q, %v)",
						s, from, at, gotLen, gotText, gotURL, gotOK, wantLen, wantText, wantURL, wantOK)
				}
			}
		}
	}
}

// TestLinkIndex_PreservesSchemeDecisions closes the loop from the differential
// above to the thing that actually matters: the accept/reject decision. Every
// url the two paths produce must not only be equal but must be judged the same
// by allowedScheme, so no bounding of the scan can widen or narrow what becomes
// a live anchor.
func TestLinkIndex_PreservesSchemeDecisions(t *testing.T) {
	corpus := append(costCorpus(), schemeCorpus()...)
	for _, s := range corpus {
		for _, from := range bracketOffsets(s) {
			x := newLinkIndex(s, from)
			for _, at := range bracketOffsets(s) {
				if at < from {
					continue
				}
				_, _, gotURL, gotOK := x.parseLinkAt(s, at)
				_, _, wantURL, wantOK := parseLink(s[at:])
				if gotOK != wantOK {
					t.Fatalf("parseLinkAt/parseLink disagree on match at %d of %q", at, s)
				}
				if gotOK && allowedScheme(gotURL) != allowedScheme(wantURL) {
					t.Fatalf("scheme decision diverged at %d of %q: %q -> %v vs %q -> %v",
						at, s, gotURL, allowedScheme(gotURL), wantURL, allowedScheme(wantURL))
				}
			}
		}
	}
}

// schemeCorpus is the rejected/accepted-scheme material, embedded in the shapes
// that force the indexed path (a preceding failed bracket), so the scheme
// boundary is exercised through the fast path and not only through the slow one.
func schemeCorpus() []string {
	urls := []string{
		"https://example.test/p", "http://example.test/p", "mailto:a@b.test",
		"javascript:alert(1)", "data:text/html,x", "vbscript:x", "  JavaScript:x",
		"java\tscript:alert(1)", "//evil.test/p", "/\\evil.test/p", "\\\\evil.test/p",
		"\\/evil.test/p", "#frag", "relative/path", "", "HTTPS://example.test",
	}
	var out []string
	for _, u := range urls {
		// A bare link, and the same link after a bracket that cannot complete
		// one — the second forces newLinkIndex to have been built first.
		out = append(out, "[t]("+u+")", "[ [t]("+u+")", "[](["+u+") [t]("+u+")")
	}
	return out
}

// bracketOffsets returns every offset of '[' in s. These are the only offsets
// renderInline ever builds a link index at, and the only ones it ever asks
// about: both parseLink and parseLinkAt require a '[' at the position given,
// so querying anywhere else would be testing a contract neither one has.
func bracketOffsets(s string) []int {
	var out []int
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			out = append(out, i)
		}
	}
	return out
}

func BenchmarkRenderInlineUnclosedBrackets(b *testing.B) {
	body := strings.Repeat("[a](", oneMiB/4)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderInline(body)
	}
}

// --- list-item prose accumulation: correctness of the fast path -----------

// TestItemProse_SegmentJoinMatchesConcatenation is the correctness half of the
// addText fix. The accumulator changed from repeated `text += " " + s` to a
// segment slice joined once, and the whole claim is that this is
// byte-identical. So: render K continuation lines under an item and require the
// item's text to be exactly the segments joined by a single space, for every K
// from none to many, at top level and nested.
//
// K = 0 and K = 1 are in the sweep on purpose: a join that special-cases the
// first segment wrongly (a leading or doubled space) shows up only there.
func TestItemProse_SegmentJoinMatchesConcatenation(t *testing.T) {
	for _, k := range []int{0, 1, 2, 3, 7, 64} {
		segs := []string{"first"}
		var body strings.Builder
		body.WriteString("- first\n")
		for i := 0; i < k; i++ {
			s := fmt.Sprintf("cont%d", i)
			segs = append(segs, s)
			body.WriteString("  " + s + "\n")
		}
		want := "<ul><li>" + strings.Join(segs, " ") + "</li></ul>"
		if got := string(Render(body.String())); got != want {
			t.Errorf("K=%d continuation lines:\n got: %q\nwant: %q", k, got, want)
		}
	}
	// The same, with a blank line before each continuation — the loose-list
	// path, which is what made this accumulator reachable at all. A blank line
	// is a soft break like any other: it must not change the JOINED TEXT. It
	// does change the wrapper, because a blank line inside a list now makes
	// the list loose and every item's prose run is <p>-wrapped (CommonMark);
	// the assertion is deliberately still written as "the same joined text,
	// inside <p>" so a join regression cannot hide behind the new wrapper.
	for _, k := range []int{1, 2, 5} {
		segs := []string{"first"}
		var body strings.Builder
		body.WriteString("- first\n")
		for i := 0; i < k; i++ {
			s := fmt.Sprintf("cont%d", i)
			segs = append(segs, s)
			body.WriteString("\n  " + s + "\n")
		}
		want := "<ul><li><p>" + strings.Join(segs, " ") + "</p></li></ul>"
		if got := string(Render(body.String())); got != want {
			t.Errorf("K=%d blank-separated continuation lines:\n got: %q\nwant: %q", k, got, want)
		}
	}
}

// TestItemProse_RunsRestartAfterABlock pins the other half of addText's
// contract, which the segment change must not disturb: a fence or a nested list
// CLOSES the open prose run, so prose written after one renders after it rather
// than folding backwards into it.
func TestItemProse_RunsRestartAfterABlock(t *testing.T) {
	body := "- one\n  two\n  ```\n  code\n  ```\n  three\n  four\n"
	want := "<ul><li>one two<pre><code>code\n</code></pre>three four</li></ul>"
	if got := string(Render(body)); got != want {
		t.Errorf("prose runs around a fence:\n got: %q\nwant: %q", got, want)
	}
}

func BenchmarkRenderListContinuation(b *testing.B) {
	body := "- x\n" + strings.Repeat("  a\n", oneMiB/4)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Render(body)
	}
}

// BenchmarkRenderInlineOrdinaryProse is the control: the overwhelmingly
// common shape, whose code spans and links all close. It must not have paid for
// any of the cost fixes — no index of any kind is built for it.
func BenchmarkRenderInlineOrdinaryProse(b *testing.B) {
	body := strings.Repeat("The `id` field is required; see `claim new` and [the docs](https://x/y). ", 200)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderInline(body)
	}
}

// --- shared corpus -------------------------------------------------------

// costCorpus is a differential-testing corpus: hand-written shapes that
// stress fence, backtick-run and link-delimiter edges, plus deterministic
// pseudo-random strings over an alphabet of every byte the block and inline
// scanners special-case. Deterministic seed, so a failure is always
// reproducible.
func costCorpus() []string {
	corpus := []string{
		"",
		"`",
		"```",
		"```\n```",
		"```go\n```",
		"``` ```",
		"`a``b```c",
		"a\\`b`c`",
		"\\``x`",
		"```\nx\n```\n```\ny\n```",
		"   ```\n   x\n   ```",
		"```\nunclosed",
		"````\n```\n````",
		"``````\n```\n``````",
		"- x:\n  ```\n  a\n  ```",
		"1. x\n\n   ```\n   a\n   ```\n\n2. y",
		"````js\n```\n````\ntail",
		// Link-delimiter edges, for the linkIndex differential.
		"[", "[]", "[](", "[a](", "[a](b)", "[a](b", "[]()", "[[a](b)",
		"[a](b)[c](d)", "[a][b](c)", "[a](b))", "[a]((b)", "]a(b)",
		"[a\\](b)", "[`a`](b)", "[a](b c)", "[](())", "[[[]]]",
		"[a](javascript:x)", "[a](//h/p)", "[a](#f)", "[x](mailto:a@b)",
	}
	rng := rand.New(rand.NewSource(0x5EED))
	alphabet := []byte("``` \n\\ax[](){}-*.1#")
	for c := 0; c < 400; c++ {
		n := 1 + rng.Intn(120)
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = alphabet[rng.Intn(len(alphabet))]
		}
		corpus = append(corpus, string(buf))
	}
	return corpus
}

// TestCostCorpusRendersSafely is a cheap belt-and-braces pass: the whole
// differential corpus, hostile bytes and all, must still come out escaped and
// must never panic. The cost fixes touch the scanners, so they get the same
// escaping-boundary smoke test every other scanner change gets.
func TestCostCorpusRendersSafely(t *testing.T) {
	for _, body := range costCorpus() {
		out := string(Render(body))
		if strings.Contains(out, "<script") {
			t.Fatalf("unescaped markup leaked from %q: %s", body, out)
		}
		assertTagBalance(t, out)
	}
}
