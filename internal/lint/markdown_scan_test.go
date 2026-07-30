package lint

import (
	"reflect"
	"testing"

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
