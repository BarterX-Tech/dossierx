package approvaledit

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/textdiff"
)

// The panel shows prose. A diff that shows `**like this**` puts the reader in
// a second document written in a notation nobody asked them to read.
func TestRenderHunkRendersMarkdown(t *testing.T) {
	got, changed := renderHunk(textdiff.Hunk{Op: textdiff.OpEqual, Text: "**bold** and *plain*"})
	if strings.Contains(got, "**") {
		t.Fatalf("markdown must be rendered, not shown as source: %q", got)
	}
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Fatalf("emphasis must survive into the rendered passage: %q", got)
	}
	if changed != nil {
		t.Fatalf("an unchanged passage marks nothing, got %v", changed)
	}
}

// The mark has to survive the markdown renderer and come back as a real
// element around the word — not around the gap after it.
func TestRenderHunkMarksTheWordsThatMoved(t *testing.T) {
	hunks := textdiff.MarkWords(textdiff.Blocks(
		"The field is outward, not internal. A claim already depends on it.",
		"The field is outward, not private. A claim already depends on it.",
	))
	if len(hunks) != 2 {
		t.Fatalf("expected a removal and an addition, got %d hunks", len(hunks))
	}
	removed, removedWords := renderHunk(hunks[0])
	added, addedWords := renderHunk(hunks[1])

	for _, tc := range []struct {
		name, got, class, word string
		words                  []string
	}{
		{"removal", removed, "claim-edit-word--removed", "internal.", removedWords},
		{"addition", added, "claim-edit-word--added", "private.", addedWords},
	} {
		if !strings.Contains(tc.got, `<span class="claim-edit-word `+tc.class+`">`+tc.word+`</span>`) {
			t.Fatalf("%s: the moved word must be wrapped tight, got %q", tc.name, tc.got)
		}
		if len(tc.words) != 1 || tc.words[0] != tc.word {
			t.Fatalf("%s: the marked words must be reported for the label, got %v", tc.name, tc.words)
		}
		if strings.Contains(tc.got, "") || strings.Contains(tc.got, "") {
			t.Fatalf("%s: no sentinel may survive into the output: %q", tc.name, tc.got)
		}
		if !strings.Contains(tc.got, "A claim already depends on it.") {
			t.Fatalf("%s: the words around the change are the sentence they belong to: %q", tc.name, tc.got)
		}
	}
}

// substituteMarks is the check that keeps a bad rewrite from shipping. A
// sentinel inside a tag would corrupt the element, so it refuses instead.
func TestSubstituteMarksRefusesWhatItCannotSafelyRewrite(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"sentinel inside a tag", `<a href="x` + markOpen + `y">text</a>`},
		{"close with no open", `<p>text` + markClose + `</p>`},
		{"nested opens", `<p>` + markOpen + `a` + markOpen + `b` + markClose + `</p>`},
		{"unclosed at the end", `<p>` + markOpen + `text</p>`},
	}
	for _, tc := range cases {
		if _, ok := substituteMarks(tc.in, textdiff.OpRemove); ok {
			t.Fatalf("%s: must be refused, not rewritten", tc.name)
		}
	}
	got, ok := substituteMarks(`<p>a `+markOpen+`b`+markClose+` c</p>`, textdiff.OpAdd)
	if !ok {
		t.Fatal("a well-formed pair in text must be accepted")
	}
	if got != `<p>a <span class="claim-edit-word claim-edit-word--added">b</span> c</p>` {
		t.Fatalf("unexpected substitution: %q", got)
	}
}

// Author input reaches the renderer, so the renderer's escaping is what keeps
// a claim body from becoming markup. It is the same boundary every other
// claim body in the viewer crosses; this asserts the diff does not step
// around it.
func TestRenderHunkEscapesAuthorMarkup(t *testing.T) {
	got, _ := renderHunk(textdiff.Hunk{Op: textdiff.OpEqual, Text: `<script>alert(1)</script> & <b>x</b>`})
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>") {
		t.Fatalf("author angle brackets must not become tags: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("author angle brackets must be escaped: %q", got)
	}
}

// A sentinel an author typed themselves must not be able to open a mark.
func TestRenderHunkIgnoresAuthorSuppliedSentinels(t *testing.T) {
	got, changed := renderHunk(textdiff.Hunk{Op: textdiff.OpEqual, Text: "plain " + markOpen + "smuggled" + markClose + " text"})
	if strings.Contains(got, "claim-edit-word") {
		t.Fatalf("an unmarked passage must produce no mark elements: %q", got)
	}
	if changed != nil {
		t.Fatalf("nothing was marked, so nothing is reported: %v", changed)
	}
}

// A mark must never change the prose around it. `internal.**` is the last
// word of a bold run: wrapping the whole token puts a sentinel between the
// word and the `**` that closes the emphasis, markdown then sees no pair, and
// the passage comes back with literal asterisks in the middle of it.
func TestRenderHunkMarksInsideEmphasisWithoutBreakingIt(t *testing.T) {
	hunks := textdiff.MarkWords(textdiff.Blocks(
		"**The field is outward, not internal.** A claim already depends on it.",
		"**The field is outward, not private.** A claim already depends on it.",
	))
	if len(hunks) != 2 {
		t.Fatalf("expected a removal and an addition, got %d", len(hunks))
	}
	for _, tc := range []struct {
		name, class, word string
		hunk              textdiff.Hunk
	}{
		{"removal", "claim-edit-word--removed", "internal.", hunks[0]},
		{"addition", "claim-edit-word--added", "private.", hunks[1]},
	} {
		got, changed := renderHunk(tc.hunk)
		if strings.Contains(got, "**") {
			t.Fatalf("%s: emphasis must survive the mark, got source in the prose: %q", tc.name, got)
		}
		if !strings.Contains(got, "<strong>") {
			t.Fatalf("%s: the passage must still render as bold: %q", tc.name, got)
		}
		if !strings.Contains(got, `<span class="claim-edit-word `+tc.class+`">`+tc.word+`</span>`) {
			t.Fatalf("%s: the word itself must carry the mark, not the punctuation: %q", tc.name, got)
		}
		if len(changed) != 1 || changed[0] != tc.word {
			t.Fatalf("%s: the reported word must be the word, got %v", tc.name, changed)
		}
	}
}

// The check behind that fix, stated on its own: whatever the marking does,
// stripping the marks has to give back exactly the unmarked rendering. A
// passage that cannot satisfy that renders unmarked rather than wrong.
func TestRenderHunkFallsBackWhenMarkingWouldChangeTheProse(t *testing.T) {
	// A fenced code block: markdown escapes its content verbatim, so a
	// sentinel inside it would survive as a literal character rather than
	// becoming an element.
	hunks := textdiff.MarkWords(textdiff.Blocks(
		"```\nalpha beta gamma\n```",
		"```\nalpha delta gamma\n```",
	))
	for _, h := range hunks {
		got, changed := renderHunk(h)
		if strings.Contains(got, "") || strings.Contains(got, "") {
			t.Fatalf("no sentinel may survive into the output: %q", got)
		}
		if strings.Contains(got, "claim-edit-word") && len(changed) == 0 {
			t.Fatalf("a marked passage must report its words: %q", got)
		}
		if !strings.Contains(got, "<pre>") {
			t.Fatalf("the code block must still render as one: %q", got)
		}
	}
}

func TestSplitMarkableKeepsSyntaxOutsideTheWord(t *testing.T) {
	cases := []struct{ in, prefix, core, suffix string }{
		{"internal.**", "", "internal.", "**"},
		{"**The", "**", "The", ""},
		{"*both*", "*", "both", "*"},
		{"plain", "", "plain", ""},
		{"word ", "", "word", " "},
		{"`code`", "`", "code", "`"},
		{"***", "***", "", ""},
		{"", "", "", ""},
	}
	for _, tc := range cases {
		prefix, core, suffix := splitMarkable(tc.in)
		if prefix != tc.prefix || core != tc.core || suffix != tc.suffix {
			t.Fatalf("splitMarkable(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tc.in, prefix, core, suffix, tc.prefix, tc.core, tc.suffix)
		}
		if prefix+core+suffix != tc.in {
			t.Fatalf("splitMarkable(%q) lost bytes", tc.in)
		}
	}
}
