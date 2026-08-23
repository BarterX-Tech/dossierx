// markdown_cite.go holds the second construct that exists only in a CLAIM
// body: the inline citation marker, "[1]", that carries a reader from a
// sentence to the source it rests on.
//
// IT IS SPELLED AS A CAPABILITY, NOT AS SYNTAX, and that is the whole of the
// compatibility argument. "[" is already the busiest opener in this package
// and "[0]" is already something authors write — an array index, a matrix
// element, a footnote in someone else's notation pasted into a body. Making a
// bracketed digit run MEAN something unconditionally would have rewritten
// prose in every existing corpus on the day it shipped, silently, with no
// author having asked for it.
//
// So recognition is gated twice, and both gates are about the CLAIM rather
// than about the text:
//
//  1. the claim must declare at least one source, and
//  2. the number between the brackets must be the Ref of one of them.
//
// A claim with no sources renders "array[0]" exactly as it did before this
// file existed, byte for byte. A claim WITH sources still renders "array[0]"
// literally, because no source answers to ref 0. Only a number the claim has
// actually cited becomes a link — which means the construct cannot fire on
// prose nobody wrote a citation into, and a body that loses its sources loses
// its markers with them rather than keeping anchors that point at nothing.
//
// WHAT THE LINT SCANNER MIRRORS. internal/lint's markdown_scan.go is a
// deliberate, documented copy of this package's recognition rules (it has to
// be: this package reports no diagnostics, it only degrades to literal text).
// The rules the citation family is held to are, in full:
//
//   - PROSE ONLY. A marker is recognized wherever the INLINE pass runs and
//     nowhere else, so a fenced block never sees one (fences do not reach the
//     inline pass at all) and neither does the interior of a code span (the
//     span consumes its whole extent before the "[" is ever offered).
//   - "[" + ASCII DIGITS + "]", with no space, sign or separator anywhere
//     inside. The digit run is at most maxCiteDigits long and carries no
//     leading zero: "0" alone is legal to write and never resolves, "01" is
//     not a marker at all. One citation therefore has exactly one spelling.
//   - THE LINK GRAMMAR WINS. The marker is only offered positions where
//     parseLink has already declined, so "[1](url)" is the link it always was
//     and "![1](url)" is the image run it always was.
//   - NEVER INSIDE LINK TEXT. An anchor may not contain another anchor; see
//     the recursion in inlineScan's "[" branch.
//   - RESOLUTION IS PART OF RECOGNITION. An unresolvable "[7]" is literal
//     text here. It is NOT silently fine: that is precisely the case the
//     source-ref-undefined lint exists to report, and the lint's scanner is
//     therefore wider than this one by exactly that case — it must see every
//     bracketed digit run in prose in order to say which ones resolve.
//
// Anything that changes a rule above must change the twin there in the same
// commit, or a claim renders one way and lints another.
package markdown

import (
	"html"
	"strconv"
	"strings"
)

// maxCiteDigits bounds the digit run a marker may carry. A citation number is
// a position in a list a human wrote by hand; nine digits is already three
// orders of magnitude past any real one, and the bound is what lets the scan
// decide a candidate in constant time and parse it without overflow checks.
const maxCiteDigits = 9

// Citations is a claim's citation capability: which refs its body may address
// and what anchor those addresses resolve to.
//
// THE ZERO VALUE IS NOT "CITE ANYTHING" — it is "this claim has no sources",
// and under it every "[n]" in the body stays the literal text it has always
// been. That is the same inversion AssetPrefix uses and for the same reason:
// the failure mode of forgetting to build one is a missing link in a claim,
// which the human reading the page sees; the failure mode of a permissive
// default would be bracketed numbers turning into links in prose that never
// asked for them.
//
// It is built by internal/render/components, which owns the claim's identity
// and therefore the only thing that can name its footer rows. This package
// treats the anchor as opaque bytes and escapes them like any other.
type Citations struct {
	// anchor is the already-escaped id prefix a marker's href is built from:
	// the emitted href is "#" + anchor + the decimal ref. Escaped ONCE here
	// rather than per marker because it is one value per claim and thousands
	// of markers per body; it still reaches the output through
	// html.EscapeString and by no other route.
	anchor string
	// refs is the set of numbers the body may address. Membership is the
	// second recognition gate, and it is a set rather than the source list
	// itself because this package has no business knowing what a source is.
	// It is only ever read by key — never iterated — so it cannot leak map
	// order into the rendered bytes.
	refs map[int]bool
}

// NewCitations builds the capability for a claim whose footer rows are
// addressed as anchorPrefix + the decimal ref, from the refs it declares.
//
// A claim with no refs yields the zero Citations, so "no sources" and "sources
// nobody can name" are the same absence rather than two states with two
// behaviours. An empty anchorPrefix does the same thing: an anchor a marker
// cannot spell is not a link, it is a href that lands somewhere else on the
// page, and losing the marker is the smaller failure.
func NewCitations(anchorPrefix string, refs []int) Citations {
	if anchorPrefix == "" || len(refs) == 0 {
		return Citations{}
	}
	set := make(map[int]bool, len(refs))
	for _, r := range refs {
		set[r] = true
	}
	return Citations{anchor: html.EscapeString(anchorPrefix), refs: set}
}

// citePolicy is the capability as the passes carry it — the same value under
// the name the block and inline scans use, so a reader of inlineScan is
// reading about a policy rather than about an exported type.
type citePolicy = Citations

// bodyPolicy is the whole set of constructs that exist in a CLAIM body and
// nowhere else, threaded through the block and inline passes as one ordinary
// value.
//
// It is a struct rather than two parameters because the two capabilities have
// identical plumbing requirements and identical failure modes: both must reach
// every container the block scan opens, and both must be ABSENT by default so
// that a construct added later acquires the refusal rather than the grant. One
// value means a new capability is one field and no new signature, and it means
// a caller cannot pass images along and forget citations.
type bodyPolicy struct {
	img  imagePolicy
	cite citePolicy
}

// match reports whether the "[" at text[i] opens a citation marker this claim
// can resolve, returning the marker's byte length and the ref it names.
//
// IT DECIDES IN CONSTANT TIME AND ALLOCATES NOTHING, and the loop is bounded
// rather than merely reaching a bound: it stops one byte past maxCiteDigits and
// refuses, instead of walking a megabyte of digits to discover they were too
// many. Disjointness would have made the unbounded form linear overall anyway —
// a "[" cannot appear inside a digit run, so no two brackets can scan the same
// bytes — but that argument holds only for TODAY'S opener set, and this is the
// untrusted-length surface. A refusal consumes nothing: the caller advances one
// byte, exactly as it did before this construct existed.
func (p citePolicy) match(text string, i int) (n, ref int, ok bool) {
	if len(p.refs) == 0 {
		return 0, 0, false
	}
	j, limit := i+1, i+1+maxCiteDigits
	for j < len(text) && j <= limit && text[j] >= '0' && text[j] <= '9' {
		j++
	}
	digits := text[i+1 : j]
	if digits == "" || len(digits) > maxCiteDigits {
		return 0, 0, false
	}
	// No leading zero, so one citation has exactly one spelling. "[0]" is
	// still a legal SHAPE — it simply never resolves, since a source's Ref is
	// positive — and that is deliberate: it keeps the refusal a matter of
	// which refs exist rather than a second, differently-shaped rule.
	if len(digits) > 1 && digits[0] == '0' {
		return 0, 0, false
	}
	if j >= len(text) || text[j] != ']' {
		return 0, 0, false
	}
	v, err := strconv.Atoi(digits)
	if err != nil || !p.refs[v] {
		return 0, 0, false
	}
	return j + 1 - i, v, true
}

// markerHTML builds one marker's anchor. The tag, the class, both attribute
// delimiters and the brackets around the number are fixed literals in this
// package; anchor was escaped when the capability was built and the number is
// re-rendered from an int, so no author byte reaches the output unescaped.
//
// It is a plain <a> rather than a <sup><a> pair: the raised, smaller rendering
// is a matter for style.css's .claim-cite rule, and baking it into the markup
// would put a second, unstyleable opinion in the one place a project's
// stylesheet override cannot reach.
func (p citePolicy) markerHTML(ref int) string {
	n := strconv.Itoa(ref)
	var b strings.Builder
	b.Grow(len(p.anchor) + len(n)*2 + 40)
	b.WriteString(`<a class="claim-cite" href="#`)
	b.WriteString(p.anchor)
	b.WriteString(n)
	b.WriteString(`">[`)
	b.WriteString(n)
	b.WriteString(`]</a>`)
	return b.String()
}
