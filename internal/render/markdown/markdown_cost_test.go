package markdown

import (
	"fmt"
	"math/rand"
	"runtime"
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
// budget is an absolute one — and, since phase A, that budget is measured in
// BYTES ALLOCATED as well as in wall-clock time. A ratio cannot see a constant
// factor, and neither can a wall-clock budget with an order of magnitude of
// headroom; a rewrite of the inline pass moved allocation on this same surface
// by two orders of magnitude while both of the older guards stayed green. See
// TestRender_AllocationAtOneMiBIsBounded.

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
		// --- phase A constructs ---------------------------------------
		//
		// Emphasis is the first construct whose resolution walks BACKWARDS,
		// and a backward walk that consumes nothing is the exact shape of
		// every defect above. The delimiter stack's openers_bottom bound and
		// its removal of a failed closer are what keep these linear, and
		// these rows are what say so. The autolink rows guard the two new
		// forward searches ("<" to its ">" and a bare url to its terminator),
		// both of which are self-bounding by design.
		{
			name: "emphasis-unmatched-stars",
			why:  "every closer walks back over every opener; openers_bottom is what bounds it",
			gen:  func(n int) string { return repeatTo("a* ", n) },
		},
		{
			name: "emphasis-unmatched-openers",
			why:  "runs that can only open: the stack grows and nothing ever consumes it",
			gen:  func(n int) string { return repeatTo(" *a", n) },
		},
		{
			name: "emphasis-underscore-unmatched-openers",
			why:  "the same never-pairing stack reached through '_', the character the corpus is dense in",
			gen:  func(n int) string { return repeatTo(" _a", n) },
		},
		{
			name: "emphasis-both-flanking-runs",
			why:  "intraword runs can open AND close, so neither the closer removal nor a match is free",
			gen:  func(n int) string { return repeatTo("a*b", n) },
		},
		{
			name: "emphasis-rule-of-three-blocked",
			why:  "run lengths chosen so the rule of three refuses every pair: pure failed back-walking",
			gen:  emphasisRuleOfThreeShape,
		},
		{
			name: "emphasis-underscore-intraword",
			why:  "the corpus shape: an intraword _ is never a delimiter and must not reach the stack at all",
			gen:  func(n int) string { return repeatTo("governed_by rests_on schema_version ", n) },
		},
		{
			// THE DENSEST SHAPE THERE IS, and the one the allocation budget
			// actually exists to bound. delimRun memory is exactly linear in
			// DELIMITER RUNS PER INPUT BYTE, and every other row in this sweep
			// sits far below the reachable maximum of that ratio: the next
			// densest, emphasis-rule-of-three-blocked, reaches 0.4 runs per
			// byte. Alternating two different delimiter characters makes every
			// single byte its own maximal run — 1.0 runs per byte, which is the
			// ceiling — and every one of those runs sits between punctuation on
			// both sides, so every one of them can both open and close and none
			// of them is filtered out before it is recorded. Without this row
			// the guard added to bound per-run memory had never been evaluated
			// at the shape that maximises it.
			name: "emphasis-alternating-delimiters",
			why:  "one delimiter run PER BYTE, the maximum density reachable at all: the shape that maximises delimRun memory per input byte",
			gen:  func(n int) string { return repeatTo("*_", n) },
		},
		{
			name: "emphasis-nested-pairs",
			why:  "every pair matches and every match removes the delimiters between: amortization check",
			gen:  func(n int) string { return repeatTo("**a** *b* ~~c~~ ", n) },
		},
		{
			name: "strike-unmatched-runs",
			why:  "the same stack, reached through the tilde spelling",
			gen:  func(n int) string { return repeatTo("a~~ ", n) },
		},
		{
			name: "strike-long-tilde-run",
			why:  "one enormous run: byteRun must measure it once, not once per byte",
			gen:  func(n int) string { return strings.Repeat("~", n) },
		},
		{
			name: "angle-openers-unterminated",
			why:  "each < scans for its >; the scan is self-bounding at the next < or whitespace",
			gen:  func(n int) string { return repeatTo("<a ", n) },
		},
		{
			name: "angle-openers-dense",
			why:  "a solid run of < with no terminator anywhere",
			gen:  func(n int) string { return strings.Repeat("<", n) },
		},
		{
			name: "angle-autolinks-rejected",
			why:  "complete runs whose scheme is refused: schemeOf strips and re-reads every one",
			gen:  func(n int) string { return repeatTo("<javascript:alert(1)> ", n) },
		},
		{
			name: "bare-url-shaped-runs",
			why:  "the bare-URL detector fires at every word boundary and consumes to the terminator",
			gen:  func(n int) string { return repeatTo("https://x.test/a ", n) },
		},
		{
			name: "bare-url-near-misses",
			why:  "the word-boundary gate must reject a prose 'h' in O(1), not by parsing a url",
			gen:  func(n int) string { return repeatTo("http here http there https ", n) },
		},
		{
			name: "bare-url-trailing-parens",
			why:  "paren balance is counted ONCE per run; recounting per stripped byte would be quadratic",
			gen:  func(n int) string { return "https://x.test/" + strings.Repeat(")", n) },
		},
		{
			name: "bare-url-one-long-run",
			why:  "a single url the length of the whole body: one scan, one balance count",
			gen:  func(n int) string { return "https://x.test/" + strings.Repeat("a", n) },
		},
		{
			name: "fence-info-strings",
			why:  "infoLanguage runs per closed fence and must not be linear in the whole line",
			gen:  func(n int) string { return repeatTo("```json\nx\n```\n", n) },
		},
		// --- phase C: pipe tables -------------------------------------
		//
		// Tables USED TO BE the one construct whose output was not bounded by
		// its input: a short body row emitted one cell per HEADER column, so N
		// columns and M rows amplified N+2M bytes into N*M cells. Two bounds
		// were tried against that and both refused well-formed tables, which
		// the spec does not authorise; what shipped instead removes the padding,
		// so a row emits exactly the cells it has and the amplification is
		// closed at the source rather than capped downstream. See
		// markdown_tables.go's cost note.
		//
		// EVERY TABLE ROW IN THIS SWEEP IS NOW AN EMITTED TABLE. Three of them
		// used to be REFUSED shapes, and a refused shape renders as a paragraph
		// — so what those three measured was the cost of a paragraph, wearing a
		// table's name. They are kept, under names that say what they are, and
		// they now measure the construct at the shapes that used to break it.
		// The rest guard the per-line searches: the unescaped-pipe scan, the
		// delimiter-row pre-filter, and one splitRow allocation per candidate
		// row.
		{
			name: "table-rows-many",
			why:  "a real table: one splitRow and one inline pass per row, and the extent walked once",
			gen:  func(n int) string { return tableOf(3, "| a | b | c |\n", n) },
		},
		{
			name: "table-columns-many",
			why:  "one enormous header: cell count is linear in the row's bytes, not quadratic in it",
			gen:  wideTable,
		},
		{
			name: "table-pipe-runs-one-line",
			why:  "many pipes on one line: a solid run that splits into one empty cell per byte",
			gen:  pipeRunLines,
		},
		{
			name: "table-ragged-rows",
			why:  "ragged rows: each emits only the cells it has, and must cost only what it emits",
			gen:  func(n int) string { return tableOf(4, "| x |\n", n) },
		},
		{
			// Formerly table-ragged-rows-refused, and formerly a paragraph.
			name: "table-ragged-rows-one-cell",
			why:  "24 columns x one-cell rows: the shape padding turned into N*M cells, now N+M",
			gen:  func(n int) string { return tableOf(24, "|\n", n) },
		},
		{
			// THE SHAPE THE WHOLE ARGUMENT IS ABOUT. Spend a sixth of the budget
			// on columns and the rest on one-byte rows: under padding the cell
			// count was bytes^2/12 — ~9x10^10 cells, ~800 GB of <td>, from one
			// stored 1 MiB comment. It is now one cell per body row, and it is a
			// TABLE, which is what both previous bounds could not deliver at
			// once. If a row ever pads again, this row goes quadratic first.
			name: "table-max-amplification",
			why:  "the ex-quadratic shape: N columns x M one-byte rows, now N+M cells and still a table",
			gen:  maxAmplificationTable,
		},
		{
			// THE REGRESSION SHAPE, pinned rather than derived because it is the
			// exact input an adversarial verifier found against the CELL bound:
			// 400 centre-aligned columns, five one-byte body rows, and 395
			// trailing spaces on the header. Repeated to 1 MiB it measured 30.9x
			// and 218 MiB allocated, over oneMiBAllocBudget. With padding gone
			// the five rows emit five cells rather than 2000.
			name: "table-wide-header-short-rows",
			why:  "N columns x M one-byte rows x S header trailing spaces: the shape the CELL bound let through at 31x",
			gen:  paddedTableAtCellBound,
		},
		{
			// THE SHAPE THAT MAXIMISES THE CONSTRUCT'S INTRINSIC RATIO, and the
			// one the byte ceiling refused outright rather than admit: every
			// cell present, every cell EMPTY, every column centre-aligned, so
			// every source "|" buys the widest cell markup there is
			// (`<td class="md-col-center"></td>`, 31 bytes) and nothing else.
			// Padding never entered into it — this is what the construct costs
			// at its own supremum — so it is the row that says whether removing
			// padding was enough on its own. The byte ceiling's own comment put
			// 1000x1000 of it at 242 MiB and used that as the reason a ceiling
			// had to sit below the intrinsic maximum; measured here at the 1 MiB
			// cap instead of at an input four times over it.
			name: "table-dense-empty-cells",
			why:  "31x: the widest cell markup per source byte, with no padding involved at all — the construct's intrinsic supremum",
			gen:  denseEmptyCellTable,
		},
		{
			// AND the shape that maximises BYTES ALLOCATED, which is a different
			// shape again — ratio-maximal is not allocation-maximal, because
			// splitRow's cell slice is paid once per SOURCE row and the
			// ratio-maximal shape spends its source on one enormous header
			// instead. Many medium-wide tables cost more than one giant one at
			// the same ratio. Its parameters are pinned: it is the shape
			// oneMiBAllocBudget was last sized against.
			name: "table-many-tables-at-width",
			why:  "many medium-wide tables: splitRow's per-source-row cell slice, the term the ratio-maximal shape does not pay",
			gen:  tableManyMediumTables,
		},
		{
			name: "table-escaped-pipes",
			why:  "the escape walk in splitRow, plus unescapePipes' per-cell rebuild",
			gen:  func(n int) string { return tableOf(2, `| a \| b | c |`+"\n", n) },
		},
		{
			name: "table-code-span-pipes",
			why:  "a pipe inside a code span splits, so every row leaves unclosed backtick runs to the inline pass",
			gen:  func(n int) string { return tableOf(3, "| `a|b` | c |\n", n) },
		},
		{
			name: "table-delimiter-shaped-non-tables",
			why:  "delimiter-row-shaped lines that never become a table: every line is parsed as one and rejected on arity",
			gen:  func(n int) string { return repeatTo("|-|-|\n|-|\n", n) },
		},
		{
			name: "table-pipe-prose-never-completes",
			why:  "pipe-bearing prose: the delimiter pre-filter must reject in O(line) with no allocation",
			gen:  func(n int) string { return repeatTo("a | b | c\n", n) },
		},
		{
			name: "table-in-blockquote",
			why:  "tables reached through the quote recursion, where the interior is re-sliced per quote",
			gen:  func(n int) string { return "> | a | b |\n> | - | - |\n" + repeatTo("> | 1 | 2 |\n", n) },
		},
		// --- phase D: images ------------------------------------------
		//
		// Images make "!" a construct opener for the first time, which is the
		// same shape as every other row above: a byte that used to fall through
		// in O(1) now runs a search that may consume nothing. The link index is
		// what bounds it (an image reuses parseLink verbatim), and these rows
		// are what say so — measured through Render, where the capability is
		// OFF and every complete run therefore falls through as literal text,
		// which is the path a reviewer-authored 1 MiB comment body takes.
		// TestRenderClaimBody_ImageCostAtOneMiBIsBounded runs the same shapes
		// with the capability ON, where each one also EMITS markup.
		{
			name: "image-bangs-bare",
			why:  "a prose '!' must not cost a link parse: the '[' lookahead has to reject in O(1)",
			gen:  func(n int) string { return strings.Repeat("!", n) },
		},
		{
			name: "image-openers-never-complete",
			why:  "'![' whose link parse fails consumes nothing; without the link index every one re-walks",
			gen:  func(n int) string { return repeatTo("![", n) },
		},
		{
			name: "image-openers-distant-close",
			why:  "every '![' walks to the same distant ']' only to fail the '(' test and consume nothing",
			gen:  func(n int) string { return repeatTo("![", n) + "]x" },
		},
		{
			name: "image-openers-open-paren",
			why:  "the ')' rescan, reached through the image opener rather than the link one",
			gen:  func(n int) string { return repeatTo("![a](", n) },
		},
		{
			name: "image-src-refused",
			why:  "a complete run whose src the gate refuses: entity-decode plus ctrl-strip per src, consuming the run",
			gen:  func(n int) string { return repeatTo("![a](https://evil.example/x.png) ", n) },
		},
		{
			// The refusal class the normalisation guard added. It is the one
			// shape that reaches BOTH the guard's comparison strip and the
			// segment walk, so it is where "refuse instead of rewrite" is
			// charged. It must stay the same order as every other refusal —
			// the guard runs after the length bound, so it is a pass over at
			// most maxAssetSrcBytes per src, never over the body.
			name: "image-src-space-refused",
			why:  "a src carrying a space: the normalisation guard refuses rather than rewriting, one bounded extra pass per src",
			gen:  func(n int) string { return repeatTo("![a](assets/a b.png) ", n) },
		},
		{
			name: "image-src-accepted",
			why:  "a complete run the gate accepts: ImageSrc's segment walk, once per image",
			gen:  func(n int) string { return repeatTo("![a](assets/x.png) ", n) },
		},
		{
			// The densest EMITTED shape: the shortest source that still buys a
			// whole <img> tag. Under Render it emits nothing at all (which is
			// the point of measuring it here too — the refusal path must not be
			// the expensive one); under RenderClaimBody it is what the markup
			// ratio is maximised by, and the claim-body guard below is where
			// that is bounded.
			name: "image-minimal-runs",
			why:  "the shortest source that emits a whole <img>: the construct's markup-ratio supremum",
			gen:  func(n int) string { return repeatTo("![](assets/a.png)", n) },
		},
		{
			name: "image-long-alt",
			why:  "one image whose alt is the whole body: the alt is escaped once, not once per byte",
			gen:  func(n int) string { return "![" + strings.Repeat("a", n) + "](assets/x.png)" },
		},
		{
			name: "image-long-src",
			why:  "one src the length of the body: the gate's strip/decode/segment walk must be one pass",
			gen:  func(n int) string { return "![a](assets/" + strings.Repeat("a", n) + ".png)" },
		},
		{
			name: "image-in-table-cells",
			why:  "images reached through the cell inline pass, one gate evaluation per cell",
			gen:  func(n int) string { return tableOf(2, "| ![a](assets/x.png) | b |\n", n) },
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

// emphasisRuleOfThreeShape builds a paragraph of "*" runs whose lengths are
// chosen so that CommonMark's rule of three refuses every candidate pair: each
// run both opens and closes, and the sum of any two adjacent run lengths is a
// multiple of three while neither length is. Nothing ever matches, so every
// closer's back-walk consumes nothing — which is precisely the shape that is
// quadratic without an openers_bottom bound.
func emphasisRuleOfThreeShape(n int) string {
	var sb strings.Builder
	sb.Grow(n + 64)
	sb.WriteString("a")
	for sb.Len() < n {
		sb.WriteString("*a**a")
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

// tableOf builds a real pipe table of cols columns whose body is row repeated
// to about n bytes. The header and delimiter rows are generated to match, so a
// caller only chooses the raggedness: a row with cols cells is a dense table,
// and a shorter one exercises padding.
func tableOf(cols int, row string, n int) string {
	var sb strings.Builder
	sb.Grow(n + 8*cols + 64)
	for _, cell := range []string{" h |", " - |"} {
		sb.WriteByte('|')
		for c := 0; c < cols; c++ {
			sb.WriteString(cell)
		}
		sb.WriteByte('\n')
	}
	for sb.Len() < n {
		sb.WriteString(row)
	}
	return sb.String()
}

// wideTable builds ONE table whose header is as wide as the whole body: about
// n/8 columns, a matching delimiter row and a single body row. Row count is
// constant, so any cost that is superlinear in a single row's cell count shows
// up here and nowhere else.
func wideTable(n int) string {
	cols := n / 8
	if cols < 1 {
		cols = 1
	}
	return tableOf(cols, "| x |\n", 0)
}

// maxAmplificationTable builds the table shape whose emitted cell count WAS
// maximal for its byte budget: a header of n/6 columns, a matching delimiter
// row, and one-byte body rows for the remaining half of the budget. Column
// count and row count are each linear in n, so under padding the CELL count was
// quadratic in it — the whole reason this file's table rows exist. With padding
// gone each of those body rows emits one cell, and the shape is linear.
func maxAmplificationTable(n int) string {
	cols := n / 6
	if cols < 1 {
		cols = 1
	}
	var sb strings.Builder
	sb.Grow(n + 64)
	sb.WriteString(strings.Repeat("|", cols+1)) // cols empty header cells
	sb.WriteString("\n|")
	sb.WriteString(strings.Repeat("-|", cols))
	sb.WriteByte('\n')
	for sb.Len() < n {
		sb.WriteString("|\n")
	}
	return sb.String()
}

// tableUnit builds ONE table of cols columns whose delimiter cells are all
// delimCell and whose body is rows copies of row, followed by the blank line
// that ends it. Repeating the unit therefore repeats the TABLE; without the
// blank line the next header would just extend the previous table's extent,
// which is how an earlier attempt at measuring these shapes accidentally
// measured one enormous refused block instead of many small accepted ones.
func tableUnit(cols, rows int, delimCell, row string, headerSpaces int) string {
	var sb strings.Builder
	sb.WriteString(strings.Repeat("|", cols+1)) // cols empty header cells
	sb.WriteString(strings.Repeat(" ", headerSpaces))
	sb.WriteString("\n|")
	sb.WriteString(strings.Repeat(delimCell+"|", cols))
	sb.WriteByte('\n')
	for r := 0; r < rows; r++ {
		sb.WriteString(row)
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
	return sb.String()
}

// paddedTableAtCellBound is the shape an adversarial verifier found against the
// CELL bound: 400 centre-aligned columns, five one-byte body rows, and 395
// trailing spaces on the header. The spaces were the whole trick — they raised
// srcBytes, and strings.TrimSpace in splitRow means they add no cells, so each
// one bought a byte of allowance for nothing. Repeated to 1 MiB this amplified
// 30.9x and allocated 218 MiB against a 192 MiB budget.
//
// The parameters are pinned rather than derived, because this is a REGRESSION
// shape: it is the exact input that broke the previous bound, and it is worth
// as much in five years as it is today. What it now measures is the repair —
// those five rows emit five cells rather than 2000 — and the trailing-space
// lever no longer buys anything, because there is no longer an allowance to buy.
func paddedTableAtCellBound(n int) string {
	return repeatTo(tableUnit(400, 5, ":-:", "|", 395), n)
}

// denseEmptyCellTable builds the shape that maximises the construct's INTRINSIC
// ratio, with no raggedness and no padding anywhere in it: every body row
// carries exactly one cell per column, every cell is EMPTY, and every column is
// centre-aligned. Each source "|" therefore buys `<td class="md-col-center">`
// plus its closer — 31 bytes, the widest cell this package can write — and buys
// nothing else, so the ratio climbs toward 31x as the column count grows and is
// never reached, since the delimiter row is source that emits nothing.
//
// It is deliberately sized in BYTES rather than pinned at 1000x1000, the way the
// byte ceiling's own comment cited it: 1000x1000 is over 2 MiB of source, twice
// the untrusted cap, so measuring it there measured a body that cannot reach
// Render. 320 columns is wide enough to sit within a few percent of the
// supremum while leaving the rest of the budget for rows.
func denseEmptyCellTable(n int) string {
	const cols = 320
	var sb strings.Builder
	sb.Grow(n + 8*cols + 64)
	sb.WriteString(strings.Repeat("|", cols+1)) // cols empty header cells
	sb.WriteString("\n|")
	sb.WriteString(strings.Repeat(":-:|", cols))
	sb.WriteByte('\n')
	row := strings.Repeat("|", cols+1) + "\n"
	for sb.Len() < n {
		sb.WriteString(row)
	}
	return sb.String()
}

// tableManyMediumTables is the shape that maximises BYTES ALLOCATED, which is
// not the same shape as the one that maximises the emitted ratio: splitRow's
// cell slice is paid once per SOURCE row, so a body of many medium-wide tables
// costs more than one enormous table at the same ratio, which spends its source
// on a single header instead.
//
// Its parameters are pinned because it is a regression shape: it is the shape
// oneMiBAllocBudget was last sized against, and the cells whose one byte expands
// widest through html.EscapeString (`"` becomes `&#34;`) are what its rows carry.
func tableManyMediumTables(n int) string {
	return repeatTo(tableUnit(320, 5, "-", `|"|"|"|`, 0), n)
}

// pipeRunLines builds two lines of solid pipes, n/2 bytes each. Every byte is
// its own cell boundary, which is the maximum cell density a row can reach; the
// second line is delimiter-SHAPED (nothing but pipes) and is therefore parsed as
// a delimiter row before being rejected, so this measures the whole candidate
// path at its densest.
func pipeRunLines(n int) string {
	half := n / 2
	return strings.Repeat("|", half) + "\n" + strings.Repeat("|", half) + "\n"
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

// oneMiBAllocBudget is the ceiling on BYTES ALLOCATED by one Render of a 1 MiB
// body. See TestRender_AllocationAtOneMiBIsBounded for why there is one.
//
// THE SHAPE THE BUDGET IS ACTUALLY SIZED AGAINST is now
// table-dense-empty-cells, at 128 MiB: 1.5x headroom. This comment has named
// three shapes before it — loose-list-items at 77 MiB, then
// emphasis-alternating-delimiters at 108 MiB, then a table sitting on the
// markup ceiling at 156 MiB — each correction made for the same reason, which
// is the reason worth carrying forward rather than the numbers: A BUDGET IS
// ONLY EVER SIZED AGAINST THE SHAPES SOMEBODY THOUGHT TO SWEEP.
//
// WHAT CHANGED THIS TIME, and it is the one part of the table redesign that had
// to be paid for rather than deleted. Removing padding closed the amplification
// vector — a row now emits only the cells it has — but it did not touch the
// construct's INTRINSIC ratio, which is 31x: the widest cell markup
// (`<td class="md-col-center"></td>`) against the one source byte that buys it.
// A table of empty centre-aligned cells reaches that with no raggedness in it
// at all, and the markup ceiling's answer had been to REFUSE that table, which
// is precisely the answer the redesign rules out. Measured unrefused for the
// first time, the shape allocated 266 MiB — over this budget by 39%.
//
// It was brought inside by making the renderer cheaper rather than by making
// the budget bigger or the table illegal. Two reservations, neither of which
// changes an output byte: splitRow now sizes its cell slice once from the row's
// own pipe count, and writeTableRow reserves each row before writing it so the
// output builder grows by DOUBLING rather than by append's ~1.25x step for
// large slices. Together they took the same shape from 266 MiB to 128 MiB. See
// tableRowEstimate in markdown_tables.go for why the reservation is computed
// from the row's cells and never from the header's column count.
//
// The inline arithmetic below is still the whole cost model of the inline pass,
// and emphasis-alternating-delimiters is still the widest INLINE shape:
//
//	a delimRun is 72 bytes on a 64-bit build and a matchNode 16;
//	"*_" repeated is one maximal run per BYTE, the reachable maximum, so a
//	1 MiB body of it records 1,048,576 runs — a 72 MiB slice, sized exactly
//	once by growRunsExactly and live for the whole render;
//	on top of that sit the first-pass buffer, the match arena and 3.3 MiB of
//	output, for 108 MiB allocated and a measured peak live heap of 109 MiB.
//
// The next densest shape in the sweep, emphasis-rule-of-three-blocked, reaches
// 0.4 runs per byte and 47 MiB; the widest non-table BLOCK shape is
// loose-list-items at 77 MiB, whose 1 MiB of input becomes 3.5 MiB of output
// through the block renderer's joins.
//
// The table shapes above them are dominated by two costs and neither is the
// inline pass: the output builder's geometric growth over a result tens of
// megabytes wide, and splitRow's cell slice, paid once per SOURCE row. Both are
// now reserved rather than grown, which is what the paragraph above is about;
// what remains of the first is the doubling itself, ~2x the emitted bytes.
//
// The budget still sits well under the pre-repair figures for the inline shapes
// at this same size — 443 MiB for " *a", 443 MiB for " _a", 354 MiB for "a~~ ",
// 634 MiB for "[" — so a reintroduction of the token-list defect fails this by
// a factor of two to three rather than by a rounding error.
//
// PHASE D CHANGED NOTHING HERE, and that is worth recording rather than
// leaving to be re-derived. An image is new emitted markup from a very short
// source — "![](assets/a.png)" is 17 bytes in and 71 out — so it is exactly the
// shape that has moved this number three times before. Measured at the cap on
// the shape that maximises it (image-minimal-runs, one whole tag per 17 source
// bytes): 5.2x emitted and 46 MiB allocated through RenderClaimBody, which is a
// quarter of this budget and a third of table-dense-empty-cells, the shape the
// budget is actually sized against. It is far off the ceiling because the tag
// is a fixed literal plus two short escaped values and the pass allocates
// nothing per image — no node, no token, no slice.
//
// The image shapes are swept through BOTH entry points. In this file they run
// through Render, where the capability is OFF, which measures the REFUSAL path
// (parse, gate, fall through to literal text) — the path a hostile 1 MiB
// comment body actually takes. markdown_images_cost_test.go runs the same
// shapes through RenderClaimBody against this same constant, which is where the
// numbers above come from.
//
// TotalAlloc is deterministic for a given input, so the headroom does not have
// to absorb machine-to-machine variance the way the wall-clock budget does.
const oneMiBAllocBudget = 192 << 20

// TestRender_AllocationAtOneMiBIsBounded is the third half of the cost guard,
// and it exists because the other two could not see the defect it now guards.
//
// A tokenizing rewrite of the inline pass materialised the whole block as a
// token list before writing anything, at roughly one 112-byte token per two
// input bytes. Nothing about that was superlinear — ns/byte stayed flat out to
// 4 MiB — so the growth sweep, which compares two equally-slowed measurements,
// measured no change at all; and the wall-clock budget had fifteen times the
// headroom it needed, so it stayed green while a 1 MiB body went from 2 MiB of
// allocation to 443 MiB and from 2 MiB of peak live heap to 151 MiB. On the
// surface this file's header describes — a stored comment body re-rendered for
// every reader of every GET /api/comments, concurrently — that is R x K times a
// third of a gigabyte, and no guard here would have said a word.
//
// So the third guard is an ABSOLUTE ALLOCATION budget, for the same reason the
// second one is an absolute time budget: the attacker's budget is one 1 MiB
// body, not a ratio. TotalAlloc is what is measured because it is deterministic
// for a given input — it does not depend on when the collector happened to
// run — and because it bounds peak live heap from above.
func TestRender_AllocationAtOneMiBIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("cost guard skipped under -short")
	}
	if raceEnabled {
		// The race detector allocates its own shadow state per access, so
		// what this would measure is the instrumentation. Same reasoning as
		// the wall-clock budget above; the growth sweep still runs.
		t.Skip("absolute allocation budget is not meaningful under -race; the growth sweep still runs")
	}
	for _, sh := range costShapes() {
		sh := sh
		t.Run(sh.name, func(t *testing.T) {
			body := sh.gen(oneMiB)
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			out := Render(body)
			runtime.ReadMemStats(&after)
			runtime.KeepAlive(out)
			used := after.TotalAlloc - before.TotalAlloc
			t.Logf("%s: %d bytes in, %d bytes out, %.1f MiB allocated",
				sh.name, len(body), len(out), float64(used)/(1<<20))
			if used > oneMiBAllocBudget {
				t.Errorf("shape %q: %d bytes allocated %.1f MiB, over the %.0f MiB budget\n"+
					"  a constant-factor blow-up is invisible to the growth sweep and to the\n"+
					"  wall-clock budget; this is the guard that sees it\n"+
					"  this shape reaches: %s",
					sh.name, len(body), float64(used)/(1<<20),
					float64(oneMiBAllocBudget)/(1<<20), sh.why)
			}
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

// --- emphasis pairing: correctness of the fast path -----------------------

// pairDelimitersByFullWalk is pairDelimiters with the openers_bottom bound
// REMOVED: every closer walks the whole list behind it, every time. It is the
// reference TestPairDelimiters_BoundMatchesFullWalk holds the bounded pass to,
// and it is the same shape of guard scanFenceByWalk is for fence scanning — a
// cost repair may only make the answer cheaper, never different.
//
// It is otherwise a line-for-line copy of pairDelimiters. Keeping it a copy
// rather than a flag on the real function is deliberate: a flag would let the
// two share the bug being tested for.
func pairDelimitersByFullWalk(runs []delimRun) []matchNode {
	if len(runs) == 0 {
		return nil
	}
	for k := range runs {
		runs[k].prev = k - 1
		runs[k].next = k + 1
		runs[k].openHead = -1
		runs[k].closeHead = -1
		runs[k].closeTail = -1
	}
	runs[len(runs)-1].next = -1
	var arena []matchNode
	unlink := func(k int) {
		n := &runs[k]
		if n.prev >= 0 {
			runs[n.prev].next = n.next
		}
		if n.next >= 0 {
			runs[n.next].prev = n.prev
		}
	}
	for current := 0; current >= 0; {
		d := &runs[current]
		if !d.canClose {
			current = d.next
			continue
		}
		found := -1
		for o := d.prev; o >= 0; o = runs[o].prev {
			on := &runs[o]
			if !on.canOpen || on.ch != d.ch {
				continue
			}
			if d.ch != '~' && emphasisRuleOfThreeBlocks(on, d) {
				continue
			}
			found = o
			break
		}
		if found < 0 {
			next := d.next
			if !d.canOpen {
				unlink(current)
			}
			current = next
			continue
		}
		on := &runs[found]
		use := int8(1)
		switch {
		case d.ch == '~':
			use = 2
		case d.remaining >= 2 && on.remaining >= 2:
			use = 2
		}
		arena = append(arena, matchNode{next: on.openHead, use: use})
		on.openHead = len(arena) - 1
		arena = append(arena, matchNode{next: -1, use: use})
		mi := len(arena) - 1
		if d.closeTail >= 0 {
			arena[d.closeTail].next = mi
		} else {
			d.closeHead = mi
		}
		d.closeTail = mi
		on.remaining -= int(use)
		d.remaining -= int(use)
		for k := on.next; k >= 0 && k != current; {
			nk := runs[k].next
			unlink(k)
			k = nk
		}
		if on.remaining == 0 {
			unlink(found)
		}
		if d.remaining == 0 {
			next := d.next
			unlink(current)
			current = next
		}
	}
	return arena
}

// TestPairDelimiters_BoundMatchesFullWalk is the correctness half of the
// emphasis-cost bound. openers_bottom is what stops a body of runs that can all
// close and can never pair from being quadratic — and it is also the one place
// in the pairing pass where a wrong bound would silently CHANGE which runs pair.
// So every delimiter-bearing corpus body is paired twice, once bounded and once
// by full walk, and the rendered output must be byte-identical.
func TestPairDelimiters_BoundMatchesFullWalk(t *testing.T) {
	for _, s := range append(costCorpus(), delimiterCorpus()...) {
		var fastBuf, slowBuf strings.Builder
		fastRuns := inlineScan(&fastBuf, s, nil, inlineCtx{})
		slowRuns := inlineScan(&slowBuf, s, nil, inlineCtx{})
		got := spliceDelimiters(fastBuf.String(), fastRuns, pairDelimiters(fastRuns), len(s))
		want := spliceDelimiters(slowBuf.String(), slowRuns, pairDelimitersByFullWalk(slowRuns), len(s))
		if got != want {
			t.Fatalf("openers_bottom changed the answer for %q\n bounded: %s\nfull walk: %s", s, got, want)
		}
	}
}

// delimiterCorpus is hand-written emphasis material: the CommonMark shapes
// where the rule of three, a run used as both opener and closer, and an
// unmatchable closer all decide the outcome — plus the flanking cases the
// underscore decision rests on.
func delimiterCorpus() []string {
	return []string{
		"*a*", "**a**", "***a***", "****a****", "*****a*****",
		"*a**b*", "**a*b**", "*foo**bar**baz*", "*foo**bar*baz**",
		"**foo*bar**", "*a*b*c*", "a*b*c*d", "***", "****", "*", "**",
		"_a_", "__a__", "___a___", "a_b_c", "_a_b_c_", "foo_bar_baz",
		"governed_by rests_on schema_version", "_leading", "trailing_",
		"_leading and trailing_", "snake_case_word", "._a_.", "a_._b",
		"~~a~~", "~a~", "~~~a~~~", "~~a~~b~~c~~", "~~", "~~~~",
		"*__a__*", "__*a*__", "~~**a**~~", "**~~a~~**", "*~~a~~*",
		"**a `b` c**", "` *a* `", "*`a`*", "**[a](b)**", "[*a*](b)",
		"*a\\*b*", "\\*a*", "*a\\*", "\\**a**",
		"* a *", "a * b * c", "*a *b* a*", "**a **b** a**",
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
		// Phase A's delimiters and openers, in the shapes that decide a
		// fall-through: an angle run that is not an autolink, one whose scheme
		// is refused, a scheme with nothing after it, and a bare url whose
		// trailing punctuation has to be given back.
		"<", "<>", "<a", "<a b>", "< a>", "<script>", "<x:y>", "<javascript:x>",
		"<mailto:a@b>", "<<a:b>>", "*<a:b>*", "`<a:b>`",
		// Phase C's row-splitting edges: an outer pipe on one side only, an
		// interior empty cell, a delimiter row that is not one, an arity
		// mismatch, an escaped pipe at every position a backslash run can put
		// it, and a pipe inside a code span (which splits).
		"|", "||", "|||", "|\n|", "|a|\n|-|", "|a|\n|-|-|", "a|b\n-|-",
		"|a|b|\n|:-|-:|\n|1|2|", "|a|\n|:-:|\n|", "|a|\n|--|\n|1|2|3|",
		"|a|\n|--|\n|", "|a|b|\n|-|-|\n|1|", "| a |\n---\n| 1 |",
		`|a\|b|` + "\n|-|", `|a\\|b|` + "\n|-|-|", `|a\\\|b|` + "\n|-|",
		"|`a|b`|\n|-|-|", "|`a\\|b`|\n|-|", `|a\|` + "\n|-|",
		"> |a|b|\n> |-|-|\n> |1|2|", "- x\n  |a|b|\n  |-|-|",
		"|a|b|c|d|e|\n|-|-|-|-|-|\n|\n|\n|\n|\n|\n|\n|\n|",
	}
	rng := rand.New(rand.NewSource(0x5EED))
	// The alphabet is every byte the block and inline scanners special-case.
	// Phase A added six: the three emphasis delimiters, the angle bracket, and
	// the two bytes a bare url is detected by. Phase C added the pipe, which is
	// the only byte that is a delimiter at the LINE level rather than the inline
	// one — so the random shapes now reach row splitting too.
	alphabet := []byte("``` \n\\ax[](){}-*.1#_~<>:/hts|")
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
