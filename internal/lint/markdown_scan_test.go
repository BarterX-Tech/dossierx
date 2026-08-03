package lint

import (
	"reflect"
	"strings"
	"testing"

	// TEST-ONLY, and the depguard suppression is the point rather than an
	// escape from it. The rule it names — "internal/render is top-of-stack;
	// nothing under internal/{lint,...} may import it" — is about the
	// PRODUCTION dependency graph, and this import does not touch it:
	// `go list -deps ./internal/lint | grep -c render/markdown` is 0 with this
	// file in the tree. depguard matches by file path, so it sees a _test.go
	// file the same as a source one. What the import buys is the one thing
	// that keeps markdown_scan.go's deliberate MIRROR from silently drifting
	// from the renderer it mirrors: TestMDScanInline_RendererParity below
	// checks the two against each other over the whole tracked corpus. If a
	// production import of internal/render ever appears in this package, this
	// line is not the precedent for it.
	"github.com/BarterX-Tech/dossierx/internal/render/markdown" //nolint:depguard // test-only; the parity property below is what pins this package's renderer mirror, and the production dep graph is unchanged
	"github.com/BarterX-Tech/dossierx/internal/urlsafe"
)

// TestMDScanBlocks_Recognizers pins the block-level half of the scanner
// against the renderer's rules it mirrors. Each case is a whole markdown
// source; the assertions are on WHICH lines the scanner condemned, because a
// finding that points at the wrong line is worse than no finding at all.
func TestMDScanBlocks_Recognizers(t *testing.T) {
	cases := []struct {
		name             string
		source           string
		unclosedFences   []int
		reservedHeadings []int
		indentLines      []int
		tableLines       []int
		danglingSlashes  []int
	}{
		{
			name:   "clean prose has nothing to report",
			source: "A widget is the smallest unit.\n\nIt has an id.",
		},
		{
			name:   "a closed fence is clean and its content is not scanned",
			source: "before\n\n```go\n# not a heading\n- [ ] not a task\n`unclosed\n```\n\nafter",
		},
		{
			name:           "an unclosed fence is reported on its opening line",
			source:         "before\n\n```go\nfmt.Println(1)\n\nafter",
			unclosedFences: []int{3},
		},
		{
			name:   "a backslash defuses a fence marker, so there is no opener to leave unclosed",
			source: "\\```not a fence",
		},
		{
			name:   "an inline triple-backtick span is not a block fence",
			source: "Use ```x``` inline.",
		},
		{
			name:             "levels 1 and 2 are reserved; 3 to 6 are not",
			source:           "# one\n\n## two\n\n### three\n\n###### six",
			reservedHeadings: []int{1, 3},
		},
		{
			name:   "seven hashes is not a heading at all and is not reported",
			source: "####### seven",
		},
		{
			name:   "a heading indented under an open item is item prose, not a heading",
			source: "- item\n  ### still prose",
		},
		{
			name:        "an indent that matches no open level snaps down and is reported",
			source:      "- a\n - b",
			indentLines: []int{2},
		},
		{
			name:   "a two-space indent nests cleanly and is not reported",
			source: "- a\n  - b\n- c",
		},
		{
			name:   "a tab indent is four columns, so it nests cleanly",
			source: "- a\n\t- b",
		},
		{
			name:   "a dedent back to an existing marker column is resolvable",
			source: "- a\n  - b\n  - c\n- d",
		},
		{
			name:            "a trailing backslash at the end of a block has nothing to break to",
			source:          "line one\\\nline two\\",
			danglingSlashes: []int{2},
		},
		{
			name:   "an even run of trailing backslashes is an escaped literal, not a break",
			source: `path C:\\`,
		},
		{
			name:   "a well-formed pipe table is clean",
			source: "| a | b |\n| --- | --- |\n| 1 | 2 |",
		},
		{
			name:       "an alignment row with the wrong cell count is a malformed table",
			source:     "| a | b |\n| --- |\n| 1 | 2 |",
			tableLines: []int{2},
		},
		{
			name:       "a ragged body row is a malformed table",
			source:     "| a | b |\n| --- | --- |\n| 1 | 2 | 3 |",
			tableLines: []int{3},
		},
		{
			name:   "prose containing a pipe followed by a rule is not a table attempt",
			source: "pass a | to the shell\n\n---",
		},
		{
			name:   "prose containing a pipe followed by more prose is not a table attempt",
			source: "either a | b applies\nand then some more prose",
		},
		{
			name:             "a blockquote interior is scanned recursively, in outer line numbers",
			source:           "intro\n\n> # reserved inside a quote\n> and prose",
			reservedHeadings: []int{3},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mdScanBlocks(splitLines(tc.source), 1, true)
			assertIntSlicesEqual(t, "unclosed fences", s.unclosedFences, tc.unclosedFences)
			assertIntSlicesEqual(t, "reserved headings", s.reservedHeadings, tc.reservedHeadings)
			assertIntSlicesEqual(t, "indent issues", indentIssueLines(s), tc.indentLines)
			assertIntSlicesEqual(t, "table issues", tableIssueLines(s), tc.tableLines)
			assertIntSlicesEqual(t, "dangling backslashes", s.danglingSlashes, tc.danglingSlashes)
		})
	}
}

// TestMDScanInline_Constructs pins the inline half against renderInline's
// rules: which spans close, which delimiters are ambiguous enough to stay
// silent, and what parseLink's first-"]" grammar actually matches.
func TestMDScanInline_Constructs(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		unclosed   []int
		unbalanced string
		links      []string
		images     []mdImageRef
	}{
		{
			name: "a closed single-backtick span",
			text: "call `Render` first",
		},
		{
			name: "a double-backtick span may hold a literal backtick",
			text: "``a`b`` is one span",
		},
		{
			name:     "an unclosed run is reported with its length",
			text:     "the ` character",
			unclosed: []int{1},
		},
		{
			name:     "a run closed only by a different length does not close",
			text:     "``a` b",
			unclosed: []int{2, 1},
		},
		{
			name: "an escaped backtick cannot open a span",
			text: `a \` + "`" + ` b`,
		},
		{
			name: "an intraword underscore is not a delimiter",
			text: "governed_by and rests_on are set",
		},
		{
			name: "a snake_case identifier with many underscores stays silent",
			text: "the review_pending and raw_html_reviewed fields",
		},
		{
			name: "an asterisk with whitespace on both sides is not a delimiter",
			text: "2 * 3 = 6",
		},
		{
			name: "an asterisk with content on both sides is ambiguous and stays silent",
			text: "SELECT count(*) FROM t",
		},
		{
			name: "a tilde path is not a strikethrough delimiter",
			text: "see ~/notes for details",
		},
		{
			name:       "an opener with no closer is reported",
			text:       "*bold text that never closes",
			unbalanced: "*",
		},
		{
			name:       "a closer with nothing open is reported",
			text:       "text that closes* nothing",
			unbalanced: "*",
		},
		{
			name: "a balanced emphasis pair is silent",
			text: "some *emphasized* text",
		},
		{
			name: "a balanced underscore pair is silent",
			text: "some _emphasized_ text",
		},
		{
			name:       "an unbalanced strike run is reported",
			text:       "~~struck but never restored",
			unbalanced: "~",
		},
		{
			name:  "a complete link yields its url verbatim",
			text:  "see [the doc](https://example.com/a)",
			links: []string{"https://example.com/a"},
		},
		{
			name: "an incomplete link is not a link",
			text: "an unclosed [bracket run",
		},
		{
			name:   "an image opener yields alt and src",
			text:   "![a diagram](assets/flow.svg) follows",
			images: []mdImageRef{{alt: "a diagram", src: "assets/flow.svg"}},
		},
		{
			name: "an escaped bang cannot open an image",
			text: `\![not an image](assets/x.png)`,
			// The "[" that follows still opens an ordinary link.
			links: []string{"assets/x.png"},
		},
		{
			name: "a link inside a code span is consumed by the span",
			text: "`[text](javascript:alert(1))`",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := mdScanInline(tc.text, nil)
			assertIntSlicesEqual(t, "unclosed spans", in.unclosedSpans, tc.unclosed)
			if got := string(in.unbalanced); got != tc.unbalanced {
				t.Errorf("unbalanced delimiters: got %q, want %q", got, tc.unbalanced)
			}
			if !reflect.DeepEqual(in.links, tc.links) {
				t.Errorf("links: got %v, want %v", in.links, tc.links)
			}
			if !reflect.DeepEqual(in.images, tc.images) {
				t.Errorf("images: got %v, want %v", in.images, tc.images)
			}
		})
	}
}

// TestMDRunJoined_HardBreaks pins the join against markdown.joinSegments: a
// hard break is a separator SPACE recorded as an offset, and a code span may
// not span one.
func TestMDRunJoined_HardBreaks(t *testing.T) {
	s := mdScanBlocks(splitLines("open `span\\\nclose` here"), 1, true)
	if len(s.runs) != 1 {
		t.Fatalf("expected one prose run, got %d", len(s.runs))
	}
	text, breaks := s.runs[0].joined()
	if text != "open `span close` here" {
		t.Fatalf("joined text: got %q", text)
	}
	if len(breaks) != 1 {
		t.Fatalf("expected one hard-break offset, got %v", breaks)
	}
	// Neither run closes: the opener's search is capped at the break, and the
	// would-be closer then has nothing after it. Both are reported, which is
	// exactly what the renderer emits — two runs of literal backticks.
	if in := mdScanInline(text, breaks); len(in.unclosedSpans) != 2 {
		t.Errorf("a code span must not span a hard break; got unclosedSpans=%v", in.unclosedSpans)
	}
	if in := mdScanInline(text, nil); len(in.unclosedSpans) != 0 {
		t.Errorf("without the break the same span closes; got unclosedSpans=%v", in.unclosedSpans)
	}
}

// TestMDImageSrcLegal is amendment A4's gate, stated by construction. The
// backslash-as-slash rows are the half a weaker check gets wrong.
//
// The rule itself now lives in internal/urlsafe and is table-tested there; this
// test survives verbatim as the LINT's assertion about it, so that a future
// change routing markdown-sanity's image src through some other check has to
// delete this file's expectations out loud rather than quietly stop meeting
// them.
func TestMDImageSrcLegal(t *testing.T) {
	legal := []string{
		"assets/flow.svg",
		"./assets/flow.svg",
		"assets/sub/flow.png",
		"a.png",
	}
	illegal := []string{
		"",
		"https://evil.example/p.png",
		"HTTP://evil.example/p.png",
		"javascript:alert(1)",
		"data:image/svg+xml;base64,AAA",
		"//evil.example/p.png",
		`\\evil.example\p.png`,
		`/\evil.example/p.png`,
		`\/evil.example/p.png`,
		"/assets/flow.svg",
		"../other-facet/assets/flow.svg",
		"assets/../../x.png",
		"assets/flow.svg#frag",
		"assets/flow.svg?v=2",
		"&#47;&#47;evil.example/p.png",
		"ht\ttps://evil.example/p.png",
	}
	for _, src := range legal {
		if !urlsafe.IsRelativePath(src) {
			t.Errorf("src %q must be legal", src)
		}
	}
	for _, src := range illegal {
		if urlsafe.IsRelativePath(src) {
			t.Errorf("src %q must be rejected", src)
		}
	}
}

// TestMDAllowedScheme pins the anchor allowlist markdown-sanity reports
// against. It no longer "mirrors" markdown.allowedScheme — since the gate moved
// to internal/urlsafe there is one implementation and nothing left to mirror —
// but the expectations are kept here unchanged, because this is the list a
// markdown-sanity finding is generated from and it should be readable next to
// the lint that emits it.
func TestMDAllowedScheme(t *testing.T) {
	allowed := []string{
		"https://example.com",
		"http://example.com",
		"mailto:a@example.com",
		"relative/path",
		"#fragment",
		"/root-relative",
	}
	rejected := []string{
		"javascript:alert(1)",
		"  JavaScript:alert(1)",
		"java\tscript:alert(1)",
		"data:text/html,x",
		"vbscript:x",
		"//evil.example/x",
		`\\evil.example\x`,
	}
	for _, u := range allowed {
		if !urlsafe.IsAllowedHref(u) {
			t.Errorf("href %q must be allowed", u)
		}
	}
	for _, u := range rejected {
		if urlsafe.IsAllowedHref(u) {
			t.Errorf("href %q must be rejected", u)
		}
	}
}

// TestMDScanInline_DelimiterFlanking pins mdDelimRunAt's CommonMark flanking
// rule, the rule that replaced a whitespace-only one which called every
// "…bold**," shape ambiguous and then reported its partner as unmatched.
//
// The four groups are the four things the rule has to get right at once: the
// reported false positives go silent, the NON-ASCII half goes silent (a
// byte-level punctuation test cannot do this — see mdClassifyNeighbour), the
// shapes an ordinary claim corpus is full of stay silent, and a genuinely
// unmatched delimiter still reports.
func TestMDScanInline_DelimiterFlanking(t *testing.T) {
	cases := []struct {
		group      string
		text       string
		unbalanced string
	}{
		// 1. The three fixtures from the issue report. The second and third
		// are what showed the report's own diagnosis ("needs **, * and ~~
		// together") was wrong: the comma is the trigger, and the asterisk
		// case has no "~" in it at all.
		{group: "reported", text: "Only ~~strike~~, comma after."},
		{group: "reported", text: "Only ~~strike~~ no comma."},
		{group: "reported", text: "Has **bold**, and more."},

		// 2. The non-ASCII differential set. Every row reported its delimiter
		// before the change; the last three are the S-category half, and they
		// are the rows an implementation mirroring unicode.IsPunct alone still
		// gets wrong (CommonMark spec example 354).
		{group: "non-ascii", text: "*bold*—text"},       // Pd, EM DASH
		{group: "non-ascii", text: "**bold**’s edge"},   // Pf, RIGHT SINGLE QUOTATION MARK
		{group: "non-ascii", text: "**bold**…and more"}, // Po, HORIZONTAL ELLIPSIS
		{group: "non-ascii", text: "~~strike~~—dash"},   // Pd
		{group: "non-ascii", text: "x *bold* y"},        // Zs, NO-BREAK SPACE
		{group: "non-ascii", text: "*bold*£5 total."},   // Sc, POUND SIGN
		{group: "non-ascii", text: "*bold*×2 items."},   // Sm, MULTIPLICATION SIGN
		{group: "non-ascii", text: "*bold*€5 total."},   // Sc, EURO SIGN

		// 3. The non-regression set: everything the ambiguity exclusion and
		// the bracket exclusion exist for. The last six are the ones the
		// punctuation clause would otherwise have widened onto, and the URL
		// shape is in the tracked corpus
		// (testdata/markdown-cases/autolink-bare-termination.yaml).
		{group: "quiet", text: "2*3"},
		{group: "quiet", text: "SELECT count(*) FROM t"},
		{group: "quiet", text: "governed_by"},
		{group: "quiet", text: "rests_on"},
		{group: "quiet", text: "see ~/some/path"},
		{group: "quiet", text: "A pointer receiver (*Store) is used."},
		{group: "quiet", text: "The (_internal) package is private."},
		// Path separators, the second half of the carve-out. Every one of these
		// was silent before the v0.4.0 flanking rewrite; without "/" in
		// mdIsCarveOutRune the per-rune punctuation clause turns them all into
		// unmatched-"_" warnings, which is a more common shape in this project's
		// prose than any bracket case above.
		{group: "quiet", text: "The generated files live in internal/_generated today."},
		{group: "quiet", text: "Run the scan over testdata/_fixtures before locking."},
		{group: "quiet", text: "Ship it: docs/_partials/header.html is included verbatim."},
		{group: "quiet", text: "See https://example.com/docs/_index for the reference."},
		{group: "quiet", text: "Set x = a/_b in the formula."},
		{group: "quiet", text: "C-style: a[*p] deref."},
		{group: "quiet", text: "a*(b+c) is the formula."},
		{group: "quiet", text: "A file named report_(final).pdf here."},
		{group: "quiet", text: "See https://en.wikipedia.org/wiki/Foo_(bar) for details."},
		{group: "quiet", text: "<https://example.com/x_(y)>"},
		{group: "quiet", text: "A *(b)* c"},

		// 4. A genuinely unmatched run still reports, for each of the three
		// characters independently. The delimiter has to be FLANKED: "an
		// unmatched * in prose" has whitespace on both sides, is
		// neither-flanking, and has always been silent.
		{group: "unmatched", text: "an unmatched *asterisk in prose", unbalanced: "*"},
		{group: "unmatched", text: "an unmatched _under in prose", unbalanced: "_"},
		{group: "unmatched", text: "an unmatched ~~strike in prose", unbalanced: "~"},
		// The run the bracket exclusion must NOT silence: it was already
		// unambiguous under the whitespace-only rule, so it was never rescued.
		{group: "unmatched", text: "A *(b c", unbalanced: "*"},
		// The opener here is rescued and then bracket-excluded; the closer is
		// untouched, leaving one unmatched closer.
		{group: "unmatched", text: "(**bold** text", unbalanced: "*"},
	}

	for _, tc := range cases {
		t.Run(tc.group+"/"+tc.text, func(t *testing.T) {
			if got := string(mdScanInline(tc.text, nil).unbalanced); got != tc.unbalanced {
				t.Errorf("unbalanced delimiters for %q: got %q, want %q", tc.text, got, tc.unbalanced)
			}
		})
	}
}

// TestMDScanInline_RendererParity asserts the ONE-DIRECTIONAL contract that
// keeps this mirror from going stale: if the scanner says a delimiter char is
// unmatched, the renderer must not have paired it into the tag that char
// produces.
//
// The reverse is deliberately NOT asserted. The ambiguity exclusion and the
// carve-out exclusion are an under-report by design — "(*Store)",
// "Topic_(disambiguation)" and "internal/_generated" are legitimate CommonMark
// openers that this lint
// declines to warn about because a claim corpus is full of them — so a run the
// renderer leaves literal and the scanner stays quiet about is the intended
// outcome, not a parity failure.
//
// THREE PRE-EXISTING VIOLATIONS ARE PINNED IN mdParityKnownOverReports RATHER
// THAN ASSERTED AWAY. All three predate this scanner's flanking rewrite and
// none is introduced by it — measured against the release base, the rewrite
// removes eleven violations and adds none. They are here, named, because a test
// that silently skipped them would be claiming a property the scanner does not
// have. See that variable for what each one is.
//
// The import of internal/render/markdown is TEST-ONLY. internal/lint does not
// depend on that package in production and must not start (see this file's
// sibling markdown_scan.go for why the recognizers are a mirror rather than a
// call).
func TestMDScanInline_RendererParity(t *testing.T) {
	tagFor := map[byte][]string{
		'*': {"<strong>", "<em>"},
		'_': {"<strong>", "<em>"},
		'~': {"<del>"},
	}

	check := func(t *testing.T, name, text string) {
		t.Helper()
		unbalanced := mdScanInline(text, nil).unbalanced
		if len(unbalanced) == 0 {
			return
		}
		out := string(markdown.RenderInline(text))
		for _, ch := range unbalanced {
			for _, tag := range tagFor[ch] {
				if !strings.Contains(out, tag) {
					continue
				}
				if mdParityKnownOverReports[text] {
					continue
				}
				t.Errorf("%s: scanner reports %q unmatched but the renderer emitted %s\n  in: %q\n out: %s",
					name, string(ch), tag, text, out)
			}
		}
	}

	// Every fixture the flanking test asserts on, plus the delimiter fixtures
	// from TestMDScanInline_Constructs.
	for _, text := range mdParityFixtures() {
		t.Run("fixture/"+text, func(t *testing.T) { check(t, "fixture", text) })
	}

	// Plus the whole tracked corpus, so a renderer change that repairs or
	// breaks a pairing this scanner mirrors cannot land silently.
	for _, c := range mdCorpusClaims(t) {
		if c.Body == "" {
			continue
		}
		t.Run("corpus/"+c.ID, func(t *testing.T) {
			for _, run := range mdScanBlocks(splitLines(c.Body), 1, true).runs {
				text, _ := run.joined()
				check(t, c.ID, text)
			}
		})
	}
}

// mdParityKnownOverReports are the three inputs on which the scanner reports a
// delimiter the renderer did pair. Every one of them is the SAME mechanism, and
// it is not the one this release's change is about: an exclusion written to
// suppress a spurious OPENER lands on a legitimate CLOSER instead, and the
// closer's partner is then reported unmatched.
//
//   - "(**bold** text" — the bracket exclusion drops the opener it rescued, so
//     the (untouched) closer is left with nothing open. This is the exact
//     behaviour the release plan pins for this input, at the base and after:
//     the alternative, an unconditional bracket exclusion, silences "A *(b c",
//     which is a genuinely unmatched opener.
//   - "*not a list*item, run together." — the closing "*" has word characters
//     on both sides, so the AMBIGUITY exclusion drops it; CommonMark lets a
//     both-flanking "*" close, and the renderer does.
//   - "~~**Struck and bold**~~, …" — the closing "~~" has punctuation on both
//     sides, which is likewise ambiguous here and likewise closes there.
//
// Fixing that class means letting an ambiguous run CLOSE something already
// open, which is a change to mdUnbalancedDelims's pairing rather than to
// mdDelimRunAt's classification. It is out of scope here and deliberately not
// attempted. Shrinking this map is a welcome change; growing it is a
// regression, and the test will not tell you the difference — read the diff.
var mdParityKnownOverReports = map[string]bool{
	"(**bold** text":                  true,
	"*not a list*item, run together.": true,
	"~~**Struck and bold**~~, ~~*struck and italic*~~, and ~~`struck code`~~.": true,
}

// mdParityFixtures is the hand-written half of the parity property's input:
// every delimiter fixture the two inline tests above assert on.
func mdParityFixtures() []string {
	return []string{
		"Only ~~strike~~, comma after.",
		"Only ~~strike~~ no comma.",
		"Has **bold**, and more.",
		"*bold*—text",
		"**bold**’s edge",
		"**bold**…and more",
		"~~strike~~—dash",
		"x *bold* y",
		"*bold*£5 total.",
		"*bold*×2 items.",
		"*bold*€5 total.",
		"2*3",
		"SELECT count(*) FROM t",
		"governed_by",
		"rests_on",
		"see ~/some/path",
		"A pointer receiver (*Store) is used.",
		"The (_internal) package is private.",
		"C-style: a[*p] deref.",
		"a*(b+c) is the formula.",
		"A file named report_(final).pdf here.",
		"See https://en.wikipedia.org/wiki/Foo_(bar) for details.",
		"<https://example.com/x_(y)>",
		"A *(b)* c",
		"an unmatched *asterisk in prose",
		"an unmatched _under in prose",
		"an unmatched ~~strike in prose",
		"A *(b c",
		"(**bold** text",
		"*bold text that never closes",
		"text that closes* nothing",
		"some *emphasized* text",
		"some _emphasized_ text",
		"~~struck but never restored",
	}
}

// --- small shared test helpers --------------------------------------------

func splitLines(s string) []string {
	out := []string{""}
	out = out[:0]
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func indentIssueLines(s *mdScan) []int {
	var out []int
	for _, iss := range s.indentIssues {
		out = append(out, iss.line)
	}
	return out
}

func tableIssueLines(s *mdScan) []int {
	var out []int
	for _, iss := range s.tableIssues {
		out = append(out, iss.line)
	}
	return out
}

func assertIntSlicesEqual(t *testing.T, what string, got, want []int) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}
