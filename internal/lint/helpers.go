// This file holds small helpers shared by more than one of the lint files
// in this package (claim lookup by id, extraction of claim-id-shaped tokens
// from free text, and extraction of "[n]" citation markers from a claim's
// prose). It intentionally implements no Lint itself.
package lint

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// claimByID returns the claim with the given id, if any.
func claimByID(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}

// idShapedToken matches the on-disk id grammar (FORMAT.md): three
// dot-separated, kebab-case-ish segments. It is deliberately loose at the
// regex level (module/facet membership is checked separately against
// config) so it doesn't need to know anything project-specific itself.
var idShapedToken = regexp.MustCompile(`\b[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)

// codeFence matches a fenced ```...``` code block, across lines.
var codeFence = regexp.MustCompile("(?s)```.*?```")

// extractCandidateIDs finds every substring of text that is shaped like a
// claim id (module.facet.slug) AND whose module/facet segments are both
// declared in cfg — this ties the heuristic to the project's own
// vocabulary instead of matching arbitrary dotted text (version numbers,
// IP-shaped strings, etc.) that happens to contain two dots.
func extractCandidateIDs(text string, cfg *config.Config) []string {
	var out []string
	for _, tok := range idShapedToken.FindAllString(text, -1) {
		parts := strings.SplitN(tok, ".", 3)
		if len(parts) != 3 {
			continue
		}
		if contains(cfg.Modules, parts[0]) && contains(cfg.Facets, parts[1]) {
			out = append(out, tok)
		}
	}
	return out
}

// splitFencedAndProse separates a claim's Body into the text inside fenced
// code blocks and the remaining prose (fences stripped out), so lints can
// scan each half separately (code-orphan looks inside fences,
// body-edge-hint looks outside them).
func splitFencedAndProse(body string) (fenced, prose string) {
	var fencedParts []string
	for _, m := range codeFence.FindAllString(body, -1) {
		fencedParts = append(fencedParts, m)
	}
	fenced = strings.Join(fencedParts, "\n")
	prose = codeFence.ReplaceAllString(body, "")
	return fenced, prose
}

// citationMarker matches a "[n]" citation marker: an opening bracket, a run
// of one to maxCitationDigits ASCII digits, a closing bracket. Nothing else —
// no spaces, no letters, no nested brackets, no "[1,2]" and no "[1-3]".
//
// The grammar is this small ON PURPOSE. It is implemented twice — here, and
// in the renderer that turns a marker into a link to the citation footer
// (internal/render/markdown/markdown_cite.go) — and two implementations of a
// rule agree only for as long as the rule is simple enough to hold in one
// sentence. Anything richer (ranges, comma lists, "[ref: 1]") would have to be
// re-derived identically in both places by whoever changes one of them, and
// the failure mode when they drift is silent: the lint declares a marker
// resolved that the viewer renders as literal text, or the reverse. See
// source_ref_undefined.go.
//
// The digit bound and the leading-zero rule below are not decoration; they are
// the two places this scanner previously DID drift from the renderer, and each
// drift was silent in the direction that matters least visibly. The renderer
// refuses a leading zero so that one citation has exactly one spelling, and
// refuses a run longer than nine digits so a hostile body costs a fixed number
// of comparisons per bracket. A scanner without those rules reads "[01]" as
// ref 1 and "[0000000001]" as ref 1, so a body citing either would satisfy
// source-ref-unused — no warning — while the viewer rendered plain text and
// the reader saw a source nothing pointed at.
var citationMarker = regexp.MustCompile(`\[(\d+)\]`)

// maxCitationDigits mirrors maxCiteDigits in
// internal/render/markdown/markdown_cite.go. A citation number is a position
// in a list a human wrote by hand; nine digits is already three orders of
// magnitude past any real one.
const maxCitationDigits = 9

// citationRef decides whether the digit run captured between a marker's
// brackets is a citation number, and which one, applying the renderer's rules
// exactly.
//
// Two runs are refused. A run longer than maxCitationDigits is not a number
// any author typed as a citation. A run with a leading zero ("01", "007") is
// not a marker at all, which is what makes a citation's spelling unique.
//
// "0" ALONE is a legal shape and is refused here for a different reason: no
// source can carry it, because source-shape requires a positive ref. The
// renderer therefore leaves "[0]" as literal text, and a scanner that
// collected it would report source-ref-undefined against "array[0]" written
// in ordinary running prose — a finding about text the reader sees rendered
// exactly as the author wrote it. Describing what the reader actually gets is
// the whole job; reporting a defect with no visible consequence is how a lint
// suite teaches people to stop reading it.
func citationRef(digits string) (int, bool) {
	if digits == "" || len(digits) > maxCitationDigits {
		return 0, false
	}
	if len(digits) > 1 && digits[0] == '0' {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// citedSourceRefs returns, in first-appearance order and without repeats,
// every source ref the body's PROSE cites with a "[n]" marker.
//
// Prose means: fenced code blocks removed (splitFencedAndProse, shared with
// body-edge-hint) and inline code spans removed on top of that. Both
// exclusions exist for the same reason and it is not fastidiousness — real
// claim bodies are full of "argv[0]", "rows[2]" and "matrix[10]" inside code,
// and a citation lint that read those as citations would report an undefined
// ref on the most ordinary technical prose in the corpus, which is the fastest
// way to teach an author to stop reading its findings.
//
// Prose OUTSIDE code is a different bargain: "array[0]" written in running
// text is genuinely ambiguous with a citation, and the callers resolve that
// ambiguity by only consulting this function for claims that declare at least
// one source. A claim with no sources[] cites nothing, so nothing it writes
// in prose can be a marker, and the ambiguity never arises. That gate belongs
// to the callers rather than here so that this function stays a pure
// description of the syntax.
//
// A bracketed digit run the renderer would not treat as a marker is dropped
// rather than reported — see citationRef for which runs those are and why
// reporting them would be worse than silence.
//
// The ONE way this scanner is deliberately wider than the renderer: it
// collects every syntactically valid marker, including refs no source
// declares. It has to. The renderer treats resolution as part of recognition
// and degrades an unresolvable "[7]" to literal text, because it has no way to
// report anything; saying which markers fail to resolve is exactly what
// source-ref-undefined exists for, so this side must see them.
func citedSourceRefs(body string) []int {
	if body == "" {
		return nil
	}
	_, prose := splitFencedAndProse(body)
	prose = stripInlineCodeSpans(prose)

	var out []int
	seen := make(map[int]bool)
	for _, loc := range citationMarker.FindAllStringSubmatchIndex(prose, -1) {
		// THE LINK GRAMMAR WINS, mirroring markdown_cite.go: a "[1]" followed
		// immediately by "(" is the opening of an ordinary markdown link (or,
		// with a leading "!", an image), and the renderer parses it as one —
		// it becomes an anchor labelled 1, never a citation. Counting it here
		// would mark ref 1 cited while the reader sees no citation anywhere,
		// which is the same silent divergence the digit rules above exist to
		// prevent.
		if end := loc[1]; end < len(prose) && prose[end] == '(' {
			continue
		}
		n, ok := citationRef(prose[loc[2]:loc[3]])
		if !ok || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// stripInlineCodeSpans removes every backtick-delimited inline code span from
// s, leaving the surrounding text (and its byte order) intact.
//
// It mirrors the renderer's code-span rule — an opening run of N backticks is
// closed by the next run of EXACTLY N backticks — which is why a span opened
// by a run of two backticks swallows any single-backtick pair inside it and is
// removed whole rather than in two pieces. An opener that never finds its
// closer is left in place as the literal text the renderer would also print,
// because dropping the rest of the body at an unmatched backtick would hide
// every marker after it and turn a typo into silent under-reporting.
//
// This is deliberately NOT a call into markdown_scan.go's inline machinery.
// That scanner walks a parsed block structure to report positions; all this
// needs is "which bytes are code", and the smaller routine is the one a future
// reader can check against the renderer by eye.
func stripInlineCodeSpans(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '`' {
			b.WriteByte(s[i])
			i++
			continue
		}
		run := backtickRunLen(s, i)
		closer := nextBacktickRun(s, i+run, run)
		if closer < 0 {
			b.WriteString(s[i : i+run])
			i += run
			continue
		}
		i = closer + run
	}
	return b.String()
}

// backtickRunLen returns the length of the run of backticks starting at i.
func backtickRunLen(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// nextBacktickRun returns the index of the first run of EXACTLY n backticks
// at or after from, or -1 if there is none. Runs of a different length are
// skipped whole, so a longer run never satisfies a shorter opener.
func nextBacktickRun(s string, from, n int) int {
	for i := from; i < len(s); i++ {
		if s[i] != '`' {
			continue
		}
		run := backtickRunLen(s, i)
		if run == n {
			return i
		}
		i += run - 1
	}
	return -1
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func dedupeStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
