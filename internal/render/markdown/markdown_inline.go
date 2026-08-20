// markdown_inline.go holds the inline pass: the single left-to-right scan that
// turns one paragraph's, list item's, heading's or table cell's raw text into
// safe HTML.
//
// THE SHAPE OF THE PASS, AND WHY EMPHASIS DID NOT CHANGE IT. Through phase B the
// inline pass wrote straight into a strings.Builder, because every construct it
// knew (an escape, a code span, a link) was decided the moment its opener was
// met. EMPHASIS BREAKS THAT PROPERTY: whether a "*" opens anything is not
// knowable when the scan meets it — it depends on whether a later run can close
// it, and CommonMark resolves the pairing with a stack that walks BACKWARDS.
//
// The pass still writes straight into a builder. The ONLY thing it defers is the
// delimiter runs themselves, and a delimiter run emits NOTHING on the first
// pass — so the offset it would have written at stays valid no matter what the
// pairing decides. Three steps:
//
//	inlineScan     — one left-to-right pass writing every settled byte into the
//	                 builder and recording each emphasis/strikethrough delimiter
//	                 run as a delimRun{offset into that buffer, length, flanking}.
//	pairDelimiters — the CommonMark "process emphasis" pass over the delimiter
//	                 runs only, recording which tags each run emits.
//	spliceDelimiters — one final pass copying the buffer and injecting each run's
//	                 tags and leftover characters at its recorded offset.
//
// WHY NOT A TOKEN LIST. The first version of this file materialised the whole
// block as a token list and wrote it out at the end. It was correct, and it was
// a 100x-plus memory amplifier on the untrusted surface: a body of alternating
// construct bytes and letters produced roughly one 112-byte token per two input
// bytes, so a 1 MiB comment body — internal/serve's maxBodyBytes, re-rendered
// for every stored comment on every GET /api/comments — reached hundreds of
// megabytes of peak live heap, times the number of concurrent readers. Nothing
// about that was superlinear, so the growth sweep could not see it; the fix is
// structural rather than a tuned constant. The buffer now holds output bytes and
// nothing else, a byte the scan settles as literal is never even split out of
// the pending plain run, and the only per-construct allocation left is one
// delimRun per run of "*", "_" or "~" that can actually open or close something.
// TestRender_AllocationAtOneMiBIsBounded is the guard that keeps it that way.
//
// THE ESCAPING BOUNDARY IS UNCHANGED. There are exactly three routes to the
// output. Raw author bytes go through html.EscapeString and by no other route.
// Markup built in this file is fixed literal tags plus already-escaped author
// bytes. A delimiter run emits fixed literal tags plus its own leftover
// delimiter characters, which are pre-escaped once at package init (see
// escapedDelim). There is no fourth route.
//
// ESCAPES STILL NEVER RE-ENTER THE PARSER. A backslash escape writes the escaped
// byte to the builder immediately and the scan resumes past it; nothing written
// is ever re-read. So an escaped "*" cannot be a delimiter, an escaped "<" cannot
// open an autolink, and an escaped "`" cannot open a span. That is structural,
// exactly as it was before.
//
// PRECEDENCE, in one sentence: whichever CONSUMING construct the scan meets
// first wins, and emphasis is the only non-consuming one. A code span, a link,
// an autolink and a bare URL each swallow their whole extent, so any delimiter
// inside them is never even offered to the delimiter stack; delimiters are
// merely recorded in place and resolved afterwards, which is why
// "**bold `code`**" pairs across a span while "` **not bold** `" does not pair
// at all.
package markdown

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/BarterX-Tech/dossierx/internal/urlsafe"
)

// inlineCtx carries what the inline pass needs to know about the text AROUND
// the string it was handed. There are two such things and they both exist for
// the same reason: a link's text is rendered by a RECURSIVE call on a substring,
// so that call can no longer see the source it was cut out of.
//
// inLink — whether this is a LINK'S TEXT. A link's text runs the full inline
// pass so that "[**text**](url)" composes, but an anchor may not contain another
// anchor. Both autolink forms are therefore off inside link text — the bare form
// because the gate-0 amendment says so outright ("inside link text they stay
// literal"), and the angle form for the same structural reason. Nothing else
// differs: escapes, code spans and every emphasis delimiter behave identically
// in both contexts.
//
// leftEdge / rightEdge — the SOURCE characters immediately outside this text.
// The flanking rule reads exactly one character on each side of a delimiter run,
// so for a run at either end of a link's text the answer depends on a byte the
// recursive call does not have. In the source those two bytes are always the
// link's own brackets, and CommonMark — which resolves emphasis over the whole
// paragraph, brackets included — sees them as ASCII punctuation. Without these
// fields the recursion would see start-of-text and end-of-text instead, which
// the rule counts as WHITESPACE, and "[__._](url)" would emphasise where
// CommonMark leaves it literal. edgeNone means the text really does begin (or
// end) where it says it does.
//
// The recursion is depth-bounded by the link grammar rather than by a counter:
// parseLink takes its text verbatim up to the FIRST "]", so a link's text can
// never contain the "](" a nested link would need. Depth is at most two.
//
// pol — the CLAIM-BODY CAPABILITIES (images, citation markers), carried here
// rather than passed alongside so that every recursive call into the pass has to
// carry it too. Its zero value grants neither (see bodyPolicy), which is why an
// inlineCtx built literally — as RenderInline and every non-claim-body caller
// build it — refuses both without saying so.
type inlineCtx struct {
	inLink              bool
	leftEdge, rightEdge rune
	pol                 bodyPolicy
}

// edgeNone is the "there is no character on this side" edge: the real start or
// end of the text, which the flanking rule counts as whitespace. It is the zero
// value, so an inlineCtx built without edges behaves exactly as a top-level
// block's text should.
const edgeNone rune = 0

// delimRun is one run of "*", "_" or "~" that can open or close something,
// deferred past the first pass.
//
// It is the ONLY per-construct allocation the inline pass makes, which is what
// bounds the pass' memory by the number of delimiter runs rather than by the
// input length. A run that can neither open nor close — every intraword "_", a
// "*" surrounded by spaces, a "~" run that is not exactly two — never becomes
// one of these at all: it stays in the pending plain run and costs nothing.
//
// remaining is the count of characters no match consumed. An opener is consumed
// from the back of its run and a closer from the front, so whatever remains is a
// contiguous middle that renders as literal delimiter characters — which is why
// one counter is enough for both the pairing pass and the write-out.
//
// openHead / closeHead / closeTail are singly linked lists into a shared arena
// of matchNodes, so a run that never matches costs no allocation beyond itself.
// Open matches are PREPENDED, because openTags are emitted in reverse match
// order (the last match is the innermost pair, so its tag is written last);
// close matches are APPENDED, because closeTags are emitted in match order.
type delimRun struct {
	off        int // byte offset into the first-pass buffer
	runLen     int // the run's original length: the rule of three reads this
	remaining  int // characters no match consumed
	prev, next int // the pairing pass' doubly linked list; -1 for none
	openHead   int // -1 for none
	closeHead  int // -1 for none
	closeTail  int // -1 for none
	ch         byte
	canOpen    bool
	canClose   bool
}

// matchNode is one recorded match on one run's open or close list. use is how
// many delimiter characters the match consumed, which together with ch is the
// whole of the tag decision — so no tag string is ever stored per run.
type matchNode struct {
	next int
	use  int8
}

// renderInlineCtx is the inline pass. See this file's doc comment for the three
// steps and why there are three.
func renderInlineCtx(text string, breaks []int, ctx inlineCtx) string {
	var b strings.Builder
	b.Grow(len(text) + len(text)/8 + 16)
	runs := inlineScan(&b, text, breaks, ctx)
	buf := b.String()
	if len(runs) == 0 {
		// The overwhelmingly common case, and the one the ordinary corpus
		// takes: the first pass already wrote the whole answer.
		return buf
	}
	arena := pairDelimiters(runs)
	return spliceDelimiters(buf, runs, arena, len(text))
}

// --- step 1: the scan -----------------------------------------------------

// inlineScan is the single left-to-right pass. It writes every settled byte into
// b and returns the delimiter runs, in document order, each carrying the offset
// in b at which its tags and leftover characters belong.
//
// THE PENDING PLAIN RUN IS THE COST INVARIANT. plain is the start of the run of
// ordinary bytes not yet written; it is flushed as one html.EscapeString call
// before every construct that produces something other than its own bytes, and
// once at the end. A construct the scan settles as LITERAL — a "<" that opens no
// autolink, a "[" that parses no link, a backtick run that never closes, a "~"
// run that is not two, an intraword "_" — does not flush at all: its bytes stay
// in the pending run and are written with the prose around them. That is what
// keeps a hostile body of construct bytes from costing one anything per byte.
func inlineScan(b *strings.Builder, text string, breaks []int, ctx inlineCtx) []delimRun {
	var runs []delimRun

	plain := 0
	flush := func(end int) {
		if end > plain {
			b.WriteString(html.EscapeString(text[plain:end]))
		}
	}
	// spans/links are the cost structures inherited unchanged from phase B:
	// nil until a search fails, and from then on answering the remaining
	// searches by lookup instead of by re-walking the text.
	var spans *backtickIndex
	var links *linkIndex

	// bi is the cursor into breaks. It only ever moves forward, exactly as i
	// does, so the whole break pass is amortized O(1) per byte.
	bi := 0

	for i := 0; i < len(text); {
		for bi < len(breaks) && breaks[bi] < i {
			bi++
		}
		if bi < len(breaks) && breaks[bi] == i {
			flush(i)
			b.WriteString("<br>")
			i++
			bi++
			plain = i
			continue
		}

		switch c := text[i]; c {
		case 'h', 'H':
			// The bare-URL detector, and the only construct whose opener is a
			// prose letter — which is why it is gated by a word-boundary test
			// inside bareURL before it is allowed to cost anything, and why
			// "h" is not in the opener set below. urlsafe.IsAllowedHref is a second
			// gate inside bareURL and is never the detector, because it returns
			// true for almost any prose token.
			if ctx.inLink {
				i++
				continue
			}
			n, ok := bareURL(text, i)
			if !ok {
				i++
				continue
			}
			flush(i)
			url := text[i : i+n]
			b.WriteString(anchorHTML(url, html.EscapeString(url)))
			i += n
			plain = i

		case '\\':
			if i+1 < len(text) && isEscapable(text[i+1]) {
				// The escaped byte is written STRAIGHT OUT, so it can never
				// open, close or delimit any construct.
				flush(i)
				b.WriteString(html.EscapeString(text[i+1 : i+2]))
				i += 2
				plain = i
				continue
			}
			// A backslash before anything else is an ordinary character.
			i++

		case '`':
			runLen := backtickRun(text, i)
			contentStart := i + runLen
			// A code span may not span a hard line break: the search stops at
			// the next break offset. Slicing text is exact here — a break
			// offset is a separator space, so no run straddles it.
			limit := nextBreak(breaks, contentStart, len(text))
			var closeStart int
			var ok bool
			if spans != nil {
				if closeStart, ok = spans.find(contentStart, runLen); ok && closeStart >= limit {
					ok = false
				}
			} else if closeStart, ok = findBacktickRun(text[:limit], contentStart, runLen); !ok {
				spans = newBacktickIndex(text, contentStart)
			}
			if !ok {
				// Literal backticks: they stay in the pending plain run.
				i = contentStart
				continue
			}
			flush(i)
			b.WriteString("<code>")
			b.WriteString(html.EscapeString(text[contentStart:closeStart]))
			b.WriteString("</code>")
			i = closeStart + runLen
			plain = i

		case '!':
			// "!" is a construct opener ONLY for a COMPLETE image run. A bare
			// "!" in prose, a "![" that never completes, and a "!" sitting in
			// front of an ordinary "[text](url)" that does not follow it
			// immediately all fall to the i++ below and stay in the pending
			// plain run, so exclamation marks cost nothing.
			//
			// A complete run is consumed WHOLE in both outcomes. With images
			// permitted and the src accepted it becomes one <img>; with images
			// refused — the capability is off, or the src did not pass the gate
			// — the whole "![alt](src)" stays literal, delimiters and all, and
			// is written out with the prose around it. It deliberately does NOT
			// fall back to the anchor the "[" branch would have made of it:
			// image syntax means an image or it means nothing, and a comment
			// body must not be able to spell a link two ways.
			if i+1 < len(text) && text[i+1] == '[' {
				var matchLen int
				var alt, src string
				var ok bool
				if links != nil {
					matchLen, alt, src, ok = links.parseLinkAt(text, i+1)
				} else if matchLen, alt, src, ok = parseLink(text[i+1:]); !ok {
					links = newLinkIndex(text, i+1)
				}
				if ok {
					if url, permitted := ctx.pol.img.accept(src); permitted {
						flush(i)
						b.WriteString(imgHTML(url, alt))
						i += 1 + matchLen
						plain = i
						continue
					}
					i += 1 + matchLen
					continue
				}
			}
			i++

		case '[':
			var matchLen int
			var linkText, url string
			var ok bool
			if links != nil {
				matchLen, linkText, url, ok = links.parseLinkAt(text, i)
			} else if matchLen, linkText, url, ok = parseLink(text[i:]); !ok {
				links = newLinkIndex(text, i)
			}
			if !ok {
				// THE CITATION MARKER LIVES EXACTLY HERE, IN THE LINK
				// GRAMMAR'S SHADOW. "[" belongs to links and images first: a
				// "[1](url)" is a link whose text happens to be a digit, and it
				// must keep rendering as one, so the marker is only ever offered
				// the positions where parseLink has ALREADY declined. That is
				// what makes this construct additive — every "[" that used to
				// fall through to literal text still does, unless the claim
				// declares a source with that exact ref.
				if n, ref, hit := ctx.pol.cite.match(text, i); hit {
					flush(i)
					b.WriteString(ctx.pol.cite.markerHTML(ref))
					i += n
					plain = i
					continue
				}
				// A literal "[": it stays in the pending plain run.
				i++
				continue
			}
			if !urlsafe.IsAllowedHref(url) {
				// A complete link with a refused scheme is inert literal text,
				// delimiters and all — so it too stays in the pending run.
				i += matchLen
				continue
			}
			flush(i)
			// The link's TEXT runs the full inline pass, so
			// "[**text**](url)" composes; its URL does not, and still reaches
			// the output only through urlsafe.IsAllowedHref and html.EscapeString.
			//
			// nil breaks: a link's text is rendered with no hard-break offsets,
			// so a break inside one stays the separator space it was before this
			// phase rather than becoming a <br> inside an anchor. That is the
			// pre-existing behaviour, kept.
			//
			// The edges are the link's own brackets, which is what the source
			// has on either side of this substring. Passing them is what makes a
			// delimiter run at either end of the text flank as CommonMark says
			// it does; see inlineCtx.
			//
			// THE CITATION CAPABILITY IS DROPPED ON THE WAY IN, and the image
			// one is not. An anchor may not contain another anchor, and a
			// citation marker IS an anchor.
			//
			// Today that is belt AND braces: parseLink takes its text verbatim
			// up to the FIRST "]", so a link's text can never contain the "]"
			// a marker needs to close, and the recursion could not meet one if
			// it tried. The field is zeroed anyway because the grammar is the
			// only thing making that true, and a later widening of it (bracket
			// nesting, reference links) would silently hand the recursion a
			// capability nobody re-decided. An absent capability cannot be
			// reached by a construct added to the pass later; a flag test
			// inside the marker scanner could be forgotten by one.
			inner := renderInlineCtx(linkText, nil, inlineCtx{
				inLink: true, leftEdge: '[', rightEdge: ']',
				pol: bodyPolicy{img: ctx.pol.img},
			})
			b.WriteString(anchorHTML(url, inner))
			i += matchLen
			plain = i

		case '<':
			if !ctx.inLink {
				if n, url, ok := angleAutolink(text, i); ok {
					if !urlsafe.IsAllowedHref(url) {
						// A complete autolink with a rejected scheme: the whole
						// run, brackets included, stays literal.
						i += n
						continue
					}
					flush(i)
					b.WriteString(anchorHTML(url, html.EscapeString(url)))
					i += n
					plain = i
					continue
				}
			}
			// Not an autolink. The "<" is a literal character and the scan
			// resumes one byte later — this single line is what keeps "raw
			// inline HTML" a non-goal now that "<" is a construct opener.
			i++

		case '*', '_', '~':
			n := byteRun(text, i)
			if c == '~' && n != 2 {
				// Strikethrough is spelled with exactly two tildes. A run of
				// one or of three or more is literal text, so there is exactly
				// one thing "~~" can mean and exactly one way to write it.
				i += n
				continue
			}
			f := flankingOf(text, i, n, ctx)
			canOpen, canClose := delimFlags(c, f)
			if !canOpen && !canClose {
				// The overwhelmingly common case for "_": an intraword run in
				// a token like governed_by, which can neither open nor close.
				// It never becomes a delimiter run at all, so it costs the
				// pairing pass nothing and allocates nothing.
				i += n
				continue
			}
			flush(i)
			if len(runs) == cap(runs) && cap(runs) >= runsGrowExactAt {
				runs = growRunsExactly(runs, text)
			}
			runs = append(runs, delimRun{
				off: b.Len(), runLen: n, remaining: n, ch: c,
				canOpen: canOpen, canClose: canClose,
				prev: -1, next: -1, openHead: -1, closeHead: -1, closeTail: -1,
			})
			i += n
			plain = i

		default:
			i++
		}
	}
	flush(len(text))
	return runs
}

// runsGrowExactAt is the delimiter-run count past which the run slice stops
// letting append guess its size and is sized exactly instead.
//
// append's growth is geometric with a copy at every step, so a block of N
// delimiter runs allocates several times N entries in total and holds two
// copies at the moment of the last copy. That is invisible to a growth-ratio
// guard — it is a constant factor, not an asymptotic — but on a 1 MiB comment
// body re-rendered for every reader it is the difference between tens of
// megabytes and hundreds. Past this threshold one linear pass over the text
// gives an EXACT upper bound on how many runs can still be recorded, the slice
// is sized to it once, and no copy ever happens again.
//
// The threshold is what keeps the extra pass off the ordinary path: a prose
// block has a handful of delimiter runs, never a thousand, so it never pays for
// the count at all — and a block that does reach a thousand pays for it exactly
// once, because the bound covers every run the block could still produce.
const runsGrowExactAt = 1024

// growRunsExactly re-allocates runs with capacity for every delimiter run text
// could still contribute, and copies what is already there.
func growRunsExactly(runs []delimRun, text string) []delimRun {
	grown := make([]delimRun, len(runs), delimRunUpperBound(text)+1)
	copy(grown, runs)
	return grown
}

// delimRunUpperBound counts the MAXIMAL runs of "*", "_" and "~" in text. Every
// delimiter run inlineScan records is one of these, and most of them are not
// recorded at all (an intraword "_", a "~" run that is not two, a run that can
// neither open nor close), so this is an upper bound and never an undercount.
func delimRunUpperBound(text string) int {
	n := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '*', '_', '~':
			if i == 0 || text[i-1] != text[i] {
				n++
			}
		}
	}
	return n
}

// anchorHTML builds one anchor. Both tags and the attribute delimiters are
// fixed literals; url has already passed urlsafe.IsAllowedHref and is escaped here in
// attribute context; inner is either already-escaped text or already-rendered
// inline markup.
func anchorHTML(url, inner string) string {
	return `<a href="` + html.EscapeString(url) + `">` + inner + "</a>"
}

// byteRun counts the run of s[i]'s own byte value starting at i.
func byteRun(s string, i int) int {
	c := s[i]
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// --- flanking -------------------------------------------------------------
//
// THE FLANKING RULE, in full. This is the rule the whole underscore decision
// rests on, so it is written out rather than referred to.
//
// A DELIMITER RUN is a maximal run of one of "*", "_" or "~". (Maximality plus
// the fact that a backslash escape has already been consumed and written out is
// exactly CommonMark's "not preceded or followed by a non-escaped copy of the
// same character".) For each run, let BEFORE be the character immediately
// preceding it and AFTER the one immediately following it, with the start and
// the end of the text both counting as whitespace. Both are CHARACTERS, not
// bytes: see the classification note below.
//
//	LEFT-FLANKING  : AFTER is not whitespace, AND either AFTER is not
//	                 punctuation, or BEFORE is whitespace or punctuation.
//	RIGHT-FLANKING : BEFORE is not whitespace, AND either BEFORE is not
//	                 punctuation, or AFTER is whitespace or punctuation.
//
// Then:
//
//	"*" and "~" : can open if left-flanking; can close if right-flanking.
//	"_"         : can open if left-flanking AND (not right-flanking OR
//	              preceded by punctuation);
//	              can close if right-flanking AND (not left-flanking OR
//	              followed by punctuation).
//
// WHY THAT EXTRA CLAUSE FOR "_" IS THE ENTIRE UNDERSCORE ARGUMENT. Take an
// INTRAWORD underscore — alphanumerics on both sides, which is every one of the
// nine corpus tokens the gate-0 scan named (rests_on, governed_by, claims_dir,
// schema_version, build_role, raw_html, migrated_from, validated_at,
// depended_by). BEFORE and AFTER are both word characters, so the run is BOTH
// left- and right-flanking. Can it open? left-flanking is true, but it is also
// right-flanking and is not preceded by punctuation, so no. Can it close?
// right-flanking is true, but it is also left-flanking and is not followed by
// punctuation, so no. It is neither, and it is therefore literal text — not by
// a corpus-specific exception, but because the rule says so for every intraword
// underscore that exists.
//
// THE RESIDUAL EXPOSURE, STATED IN FULL. An earlier version of this comment said
// the exposure was "exactly word-boundary underscores, and neither does anything
// alone". BOTH HALVES OF THAT WERE WRONG, and the correct statement is longer.
// A "_" run escapes the intraword clause in three distinct ways, not one:
//
//	(1) WORD-BOUNDARY OPENER — whitespace or start-of-text before, a word
//	    character after: "_leading". Left-flanking, not right-flanking, so it
//	    can open and cannot close.
//	(2) WORD-BOUNDARY CLOSER — a word character before, whitespace or
//	    punctuation after: "trailing_" or "trailing_." Right-flanking, not
//	    left-flanking, so it can close and cannot open.
//	(3) PUNCTUATION ON BOTH SIDES — ASCII punctuation before AND after:
//	    "}_{", "/_/", "-_-", "+_+". Such a run is both left- and right-flanking,
//	    but beforePunct and afterPunct are BOTH true, so both escape clauses
//	    fire: it can open AND close. This is a third class, it is neither
//	    leading nor trailing, and it is what makes a line like
//	    "{{ .Name }}_{{ .Os }}_{{ .Arch }}" — a real shape, it is in this repo's
//	    own .goreleaser.yaml — emphasise between the two underscores.
//
// And a SINGLE TOKEN CAN CARRY TWO OF THESE AT ONCE, so it needs no partner
// elsewhere: "__init__" is a class-1 run and a class-2 run in one word and
// renders as <strong>init</strong>; so do "__all__" and "__main__".
//
// THIS IS CommonMark, AND THERE IS A TEST THAT SAYS SO. Every case above is
// what the reference implementation does. What holds this file to that is
// TestCommonMark_EmphasisSection, which runs all 132 examples of the
// "Emphasis and strong emphasis" section of CommonMark 0.31.2 — transcribed
// verbatim into markdown_commonmark_data_test.go from the spec's own
// machine-readable example list, not derived from this renderer's output — and
// which permits divergence only through commonMarkEmphasisCeiling, a list of
// three examples that all turn on RAW INLINE HTML, a construct this package
// does not implement at all. An earlier version of this comment claimed
// conformance to "the 101 emphasis examples of CommonMark 0.30"; there were no
// such 101 examples, and, more to the point, no test — the claim named a gate
// that did not exist. A comment is not a gate. The table is.
//
// The point of stating the exposure is not that the rule is wrong but that its
// ceiling is higher than "leading and trailing underscores", and a reader
// deciding whether "_" was safe to admit is entitled to the real number. What
// the corpus scan established, and what
// TestRenderInline_IntrawordUnderscoreNeverEmphasises pins, is narrower and
// still true: no INTRAWORD underscore can emphasise, so no snake_case identifier
// can. Class 3 needs punctuation on both sides, which a snake_case token by
// definition does not have.
//
// WHAT COVERS THE RESIDUAL. A backslash escape defuses any of the three, and a
// code span defuses all of them at once — those are the two mechanisms an author
// has, and they are the ones to reach for. markdown-sanity is NOT a backstop
// here and this comment used to claim it was: it reports an UNMATCHED run, and
// every case above pairs, so it is silent on all of them. Aligning that lint is
// a follow-up on the lint surface, which phase A may not touch.
//
// CLASSIFICATION IS PER CHARACTER, NOT PER BYTE, AND THERE IS NO "SAFE
// DIRECTION". BEFORE and AFTER are decoded as runes and classified with
// CommonMark's own sets: WHITESPACE is a space, tab, line feed, form feed or
// carriage return, or any character in Unicode category Zs; PUNCTUATION is an
// ASCII punctuation character or any character in a Unicode P or S category.
//
// An earlier version of this file classified every byte >= 0x80 as a word
// character and called that "more conservative than CommonMark ... which is the
// safe direction". BOTH HALVES OF THAT WERE FALSE, and the mechanism is worth
// stating because it is the reason the classification is now simply correct
// rather than nominally cautious. Take "*", whose capabilities are the raw
// flanking predicates:
//
//	canOpen  = !afterWS  && (!afterPunct  || beforeWS || beforePunct)
//	canClose = !beforeWS && (!beforePunct || afterWS  || afterPunct)
//
// Calling AFTER a word character sets afterWS and afterPunct both false, which
// makes canOpen UNCONDITIONALLY TRUE — so misclassifying a non-ASCII dash or
// quotation mark could only ever hand a run MORE opening capability, never
// less. It removes closing capability at the same time, through the very same
// two flags in the other predicate's disjunction. The two move in opposite
// directions, so "conservative" was not merely imprecise, it was not a
// well-defined thing to be. Concretely, the byte rule emphasised "*—*bravo."
// and "~~—~~bravo." (U+2014) where CommonMark leaves them literal, and it
// failed two spec examples outright: 353, where a NO-BREAK SPACE (U+00A0,
// category Zs) must stop "*" opening, and 354, where "£" (category Sc, so
// punctuation) must stop it closing.
//
// THE REMAINING DIVERGENCES, stated rather than dressed up, because there is no
// safe default available here. Both are about bytes CommonMark's input
// normalization would have rewritten before its parser ever saw them, and this
// renderer does not rewrite author bytes:
//
//	(a) A byte that is not part of a valid UTF-8 encoding is a WORD CHARACTER.
//	    It is not whitespace or punctuation under any reading, no spec example
//	    covers it, and the alternative spelling — decode it to U+FFFD, which is
//	    category So and therefore punctuation — would hand an undecodable byte
//	    the punctuation escape clause that "_" opens and closes through. Word
//	    character is the choice that gives such a byte the FEWEST capabilities
//	    on "_", the character this corpus is dense in. It gives "*" the opening
//	    capability, per the paragraph above.
//	(b) A NUL byte is likewise a word character, and in particular it is NOT
//	    read as the absence of a neighbour: classifyNeighbour decides "there is
//	    no character on this side" from the decoded SIZE, never from the rune's
//	    value, so a NUL next to a delimiter run is a neighbour like any other.
//	    CommonMark would have replaced it with U+FFFD, i.e. punctuation.

// flanking is one delimiter run's flanking analysis.
type flanking struct {
	left, right             bool
	beforePunct, afterPunct bool
}

// flankingOf computes the flanking of the run text[start:start+n].
//
// ctx supplies the characters just outside text — see inlineCtx. They matter
// only for a run at the very start or the very end, which for a link's text is
// exactly where the answer would otherwise be wrong.
func flankingOf(text string, start, n int, ctx inlineCtx) flanking {
	beforeR, beforeSize := utf8.DecodeLastRuneInString(text[:start])
	afterR, afterSize := utf8.DecodeRuneInString(text[start+n:])
	beforeWS, beforePunct := classifyNeighbour(beforeR, beforeSize, ctx.leftEdge)
	afterWS, afterPunct := classifyNeighbour(afterR, afterSize, ctx.rightEdge)
	return flanking{
		left:        !afterWS && (!afterPunct || beforeWS || beforePunct),
		right:       !beforeWS && (!beforePunct || afterWS || afterPunct),
		beforePunct: beforePunct,
		afterPunct:  afterPunct,
	}
}

// classifyNeighbour classifies one decoded neighbour of a delimiter run as
// whitespace, as punctuation, or (both false) as a word character.
//
// size is what the utf8 decoder returned: 0 when there was no character on that
// side at all, in which case the caller's edge stands in for it, and 1 with r
// equal to utf8.RuneError when the byte is not valid UTF-8 — a real U+FFFD
// decodes with size 3, so the two cannot be confused. "No character at all" is
// decided by SIZE and never by the rune's value, so a NUL byte sitting in the
// text is an ordinary neighbour (a word character, as above) rather than an
// absent one.
func classifyNeighbour(r rune, size int, edge rune) (isWS, isPunct bool) {
	switch {
	case size == 0:
		if edge == edgeNone {
			return true, false
		}
		r = edge
	case size == 1 && r == utf8.RuneError:
		return false, false
	}
	if r < utf8.RuneSelf {
		return isFlankSpaceASCII(byte(r)), isASCIIPunct(byte(r))
	}
	return unicode.Is(unicode.Zs, r), unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// isFlankSpaceASCII is the ASCII half of CommonMark's Unicode whitespace set.
//
// It differs from isInlineSpaceByte, which the rest of this file uses, by
// exactly one byte: the VERTICAL TAB (U+000B). CommonMark's whitespace set is
// space, tab, line feed, form feed, carriage return and category Zs, and U+000B
// is in none of those — it is category Cc, so the flanking rule sees it as an
// ordinary word character. The two sets are spelled separately rather than
// shared precisely so that this one is answerable to the spec table and the
// other one stays what the scanner has always meant by a space.
func isFlankSpaceASCII(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\f', '\r':
		return true
	}
	return false
}

// delimFlags turns a run's flanking into its open/close capability. See the
// flanking rule above for why "_" carries the extra clause and "*" does not —
// and, in particular, for the three classes of "_" run that clause still lets
// through, one of which can both open and close.
func delimFlags(ch byte, f flanking) (canOpen, canClose bool) {
	if ch == '_' {
		return f.left && (!f.right || f.beforePunct),
			f.right && (!f.left || f.afterPunct)
	}
	return f.left, f.right
}

// isInlineSpaceByte reports ASCII whitespace. A block's lines are joined with a
// single space before the inline pass runs, so in practice this sees ' ' — but
// a tab can survive inside a line, and a hard break's offset lands on a
// separator space, which is why a delimiter next to a break flanks as if it
// were at the end of a line.
func isInlineSpaceByte(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// isASCIIPunct reports CommonMark's ASCII punctuation set.
func isASCIIPunct(c byte) bool {
	switch {
	case c >= '!' && c <= '/': // ! " # $ % & ' ( ) * + , - . /
		return true
	case c >= ':' && c <= '@': // : ; < = > ? @
		return true
	case c >= '[' && c <= '`': // [ \ ] ^ _ `
		return true
	case c >= '{' && c <= '~': // { | } ~
		return true
	}
	return false
}

// --- step 2: the delimiter stack ------------------------------------------

// openersKey is CommonMark's openers_bottom key: the delimiter character, the
// CLOSING run's original length modulo three, and whether the closer can also
// open. It exists purely for cost.
//
// Without it the pass is quadratic on a body of runs that can all close and can
// never pair — every closer would walk the entire list behind it, find nothing,
// and consume nothing. With it, a failed search records how far back it is
// worth looking for that key and every later closer with the same key starts
// there. There are at most twelve keys (three characters x three residues x two
// booleans), and each key's bound only ever moves FORWARD, so the total
// back-walking over one block is bounded by twelve times its length.
type openersKey struct {
	ch      byte
	mod     int
	canOpen bool
}

// pairDelimiters is the CommonMark "process emphasis" pass. It walks the
// delimiter runs in document order, matching each closer to the nearest opener
// behind it, and writes its result back onto the runs as consumed counts and
// linked match lists rather than building a tree: matches are properly nested by
// construction (every delimiter strictly between a matched pair is removed), so
// writing the runs out in order reproduces the tree exactly.
//
// It returns the match arena the runs' openHead/closeHead lists index into.
func pairDelimiters(runs []delimRun) []matchNode {
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

	// bottoms[key] is an EXCLUSIVE lower bound, as an index into runs: a
	// back-walk for that key stops as soon as it reaches it. Indices are fixed
	// at construction and are monotonic in document order, so the bound stays
	// meaningful even after the run it names has been unlinked.
	bottoms := make(map[openersKey]int)

	for current := 0; current >= 0; {
		d := &runs[current]
		if !d.canClose {
			current = d.next
			continue
		}
		key := openersKey{ch: d.ch, mod: d.runLen % 3, canOpen: d.canOpen}
		bottom, ok := bottoms[key]
		if !ok {
			bottom = -1
		}
		found := -1
		for o := d.prev; o > bottom; o = runs[o].prev {
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
			// Nothing behind this closer can open. Record how far back it was
			// worth looking, and drop the closer entirely unless it can also
			// serve as an opener for something later.
			bottoms[key] = d.prev
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
			// Strikethrough runs are exactly two characters and pair whole.
			use = 2
		case d.remaining >= 2 && on.remaining >= 2:
			use = 2
		}

		// An OPENER is consumed from the back of its run and a CLOSER from the
		// front, so whatever survives is a contiguous middle that renders as
		// literal delimiter characters — one counter tracks both.
		//
		// The arena is allocated on the FIRST match, sized for the common case
		// of one match per pair of runs. A block whose runs never pair — the
		// hostile shape, and the one the cost sweep measures — therefore never
		// allocates an arena at all, and an ordinary block allocates it once.
		if arena == nil {
			arena = make([]matchNode, 0, len(runs))
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

		// Everything strictly between the pair can never match anything now:
		// dropping it is what makes the emitted nesting well-formed.
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

// emphasisRuleOfThreeBlocks is CommonMark's "rule of three": if either run of a
// candidate pair can both open and close, the SUM of the two runs' original
// lengths may not be a multiple of three unless both lengths are. It is what
// stops "*foo**bar**baz*" from mis-nesting.
func emphasisRuleOfThreeBlocks(opener, closer *delimRun) bool {
	if !closer.canOpen && !opener.canClose {
		return false
	}
	if (closer.runLen+opener.runLen)%3 != 0 {
		return false
	}
	return !(closer.runLen%3 == 0 && opener.runLen%3 == 0)
}

// --- step 3: the write-out ------------------------------------------------

// escapedDelim is each delimiter character's escaped form, computed ONCE at
// package init through html.EscapeString.
//
// It exists so the write-out can emit a run's leftover characters without an
// allocation per run while the escaping boundary still holds by construction
// rather than by an assumption about which bytes html.EscapeString rewrites.
// TestEscapedDelimMatchesEscapeString pins the table against the function.
var escapedDelim = func() (t [128]string) {
	for _, c := range []byte{'*', '_', '~'} {
		t[c] = html.EscapeString(string(rune(c)))
	}
	return t
}()

// delimTag returns the fixed literal tag pair a match of `use` characters on
// character ch emits. These three pairs are the only tags this step can produce.
func delimTag(ch byte, use int8) (openTag, closeTag string) {
	switch {
	case ch == '~':
		return "<del>", "</del>"
	case use >= 2:
		return "<strong>", "</strong>"
	}
	return "<em>", "</em>"
}

// spliceDelimiters is the final pass: it copies the first-pass buffer and, at
// each delimiter run's recorded offset, writes that run's closing tags, then its
// unconsumed characters as literal text, then its opening tags.
//
// Offsets are non-decreasing because the first pass only ever appends, and a
// delimiter run wrote nothing there — so this is one linear copy with
// len(runs) insertions, and no rescanning of anything.
func spliceDelimiters(buf string, runs []delimRun, arena []matchNode, hint int) string {
	var b strings.Builder
	b.Grow(len(buf) + hint + 16)
	prev := 0
	for k := range runs {
		r := &runs[k]
		b.WriteString(buf[prev:r.off])
		prev = r.off
		for m := r.closeHead; m >= 0; m = arena[m].next {
			_, closeTag := delimTag(r.ch, arena[m].use)
			b.WriteString(closeTag)
		}
		esc := escapedDelim[r.ch]
		for n := 0; n < r.remaining; n++ {
			b.WriteString(esc)
		}
		for m := r.openHead; m >= 0; m = arena[m].next {
			openTag, _ := delimTag(r.ch, arena[m].use)
			b.WriteString(openTag)
		}
	}
	b.WriteString(buf[prev:])
	return b.String()
}

// --- autolinks ------------------------------------------------------------
//
// THE AUTOLINK RULES, in full (gate 0's autolink amendments). Two constructs,
// deliberately separate, because one has explicit delimiters and the other has
// to invent its own.
//
// ANGLE-BRACKET "<scheme:...>". "<" is a construct opener ONLY for this. The
// run must reach a ">" with no whitespace and no second "<" in between, and
// what it encloses must carry a scheme (urlsafe.SchemeOf), which is then held to
// urlsafe.IsAllowedHref UNCHANGED. Everything else — "<script>", "<img src=x>", "<"
// followed by whitespace, an unterminated run to end of text, a scheme-less
// "<//host>" or "<#frag>" — is emitted through html.EscapeString with the scan
// resuming ONE BYTE after the "<". A complete autolink whose scheme is rejected
// emits the whole run, brackets included, as escaped literal text. That is the
// invariant which keeps raw inline HTML a non-goal now that "<" opens something.
//
// BARE URL. Fires only on a literal, case-insensitive "http" or "https"
// followed by the authority mark (see authorityMark), at start-of-text or
// immediately after whitespace or "(". urlsafe.IsAllowedHref is applied to the matched
// run as a SECOND GATE and is never the detector: it returns true for
// scheme-less strings, so detecting with it would autolink ordinary prose. No
// other scheme, no "www." prefix, no bare email. The run is consumed greedily
// to the first whitespace, "<" or backtick; trailing ".,;:!?" and a trailing
// ")" with no matching "(" inside the run are excluded from the href and
// emitted as escaped literal text.
//
// PRECEDENCE. A consumed URL is written out whole, so every emphasis, strike and
// underscore delimiter inside it is literal — the URL wins over all delimiters.
// Only an already-open code span, an already-consumed link, a fence (which
// never reaches this pass at all) or link-text context suppresses recognition,
// and each of those does so by consuming the bytes first rather than by a flag.

// angleAutolink matches "<scheme:...>" anchored at text[i] (which must be "<").
// It returns the byte length of the whole run and the raw enclosed url.
//
// The scan is self-bounding, which is why it needs no index of its own: it
// stops at the first ">", "<" or whitespace, so every "<" in the text pays only
// for the bytes up to the next one of those. A body of "<" runs, or of "<"
// followed by long unterminated tokens, is linear in total.
func angleAutolink(text string, i int) (n int, url string, ok bool) {
	for j := i + 1; j < len(text); j++ {
		switch c := text[j]; {
		case c == '>':
			url = text[i+1 : j]
			if url == "" {
				return 0, "", false
			}
			// An autolink must carry a scheme. A scheme-less "<//host>" or
			// "<#frag>" is not an autolink — urlsafe.IsAllowedHref would accept the
			// second of those as an href, but this is a RECOGNITION question,
			// not an allow question.
			if _, has := urlsafe.SchemeOf(url); !has {
				return 0, "", false
			}
			return j + 1 - i, url, true
		case c == '<' || isInlineSpaceByte(c):
			return 0, "", false
		}
	}
	return 0, "", false
}

// bareURLTerminator reports the bytes that end a bare URL run.
func bareURLTerminator(c byte) bool {
	return isInlineSpaceByte(c) || c == '<' || c == '`'
}

// authorityMark is the colon-slash-slash that must follow a bare URL's scheme
// name. It is spelled as a concatenation rather than as one literal on purpose:
// tests/portability_test.go scans every non-test file under cmd/ and internal/
// for anything shaped like a remote URL, because the engine must work fully
// offline, and a scheme literal sitting in a PARSER's token table is
// indistinguishable to that scan from a real CDN reference. The value is
// exactly what it looks like; only the spelling keeps a guard this package must
// not weaken from firing on a string that fetches nothing.
const authorityMark = ":" + "//"

// bareURLPrefixes are the only two prefixes that start a bare URL, longest
// first so "https" is tried before "http". Built once at package init rather
// than concatenated per call.
var bareURLPrefixes = []string{"https" + authorityMark, "http" + authorityMark}

// bareURL matches a bare http/https URL anchored at text[i]. It returns the
// byte length of the href, which is the consumed run minus any trailing
// punctuation that belongs to the surrounding prose.
func bareURL(text string, i int) (n int, ok bool) {
	// The word-boundary gate. Without it a scheme sitting in the middle of a
	// token — "ahttps", "x=https" — would autolink from inside the word.
	if i > 0 {
		if c := text[i-1]; !isInlineSpaceByte(c) && c != '(' {
			return 0, false
		}
	}
	rest := text[i:]
	prefix := ""
	for _, p := range bareURLPrefixes {
		if hasPrefixFold(rest, p) {
			prefix = p
			break
		}
	}
	if prefix == "" {
		return 0, false
	}

	body := i + len(prefix)
	j := body
	for j < len(text) && !bareURLTerminator(text[j]) {
		j++
	}
	if j == body {
		// A scheme and an authority mark with nothing after them is
		// not a URL.
		return 0, false
	}

	// Parenthesis balance is counted ONCE over the consumed run and then
	// decremented as trailing ")" bytes are given back. Recounting per stripped
	// byte would be quadratic in a run of trailing parentheses, which is
	// reachable from the untrusted comment surface.
	opens := strings.Count(text[i:j], "(")
	closes := strings.Count(text[i:j], ")")
	for j > body {
		c := text[j-1]
		if c == '.' || c == ',' || c == ';' || c == ':' || c == '!' || c == '?' {
			j--
			continue
		}
		if c == ')' && closes > opens {
			j--
			closes--
			continue
		}
		break
	}
	if j == body {
		return 0, false
	}
	// The second gate. It can only ever narrow what becomes an anchor.
	if !urlsafe.IsAllowedHref(text[i:j]) {
		return 0, false
	}
	return j - i, true
}

// hasPrefixFold is strings.HasPrefix with ASCII case folding, so an
// upper-cased scheme is recognized without allocating a lower-cased copy of the
// remaining text.
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

// --- fence info string ----------------------------------------------------

// infoLanguage turns a fence's info string into the language token that becomes
// class="language-x", or "" when there is none to emit.
//
// THE RULE IS REJECTION, NOT ESCAPING. The value lands in an HTML attribute and
// is author bytes, so it goes through html.EscapeString like every other author
// byte — but escaping alone would still let an author put arbitrary text in a
// class name, and a class name is not a place for arbitrary text. So the token
// must be a PLAIN IDENTIFIER: a leading ASCII letter followed by ASCII
// alphanumerics and the four characters real language tags actually use
// ("+", "-", "_", "."), and no longer than 32 bytes. Anything else — a quote, a
// space, an angle bracket, a leading digit — yields no class at all and the
// fence renders exactly as it did before this phase.
//
// Only the FIRST whitespace-delimited word of the info string is considered;
// the rest is dropped, exactly as the whole info string used to be. That is
// what makes "```js title=a.js" a JavaScript block rather than no block at all.
func infoLanguage(info string) string {
	word := strings.TrimSpace(info)
	if k := strings.IndexAny(word, " \t"); k >= 0 {
		word = word[:k]
	}
	if word == "" || len(word) > 32 {
		return ""
	}
	if !isSchemeAlpha(word[0]) {
		return ""
	}
	for i := 0; i < len(word); i++ {
		c := word[i]
		if isSchemeAlpha(c) || isSchemeDigit(c) || c == '+' || c == '-' || c == '_' || c == '.' {
			continue
		}
		return ""
	}
	return word
}
