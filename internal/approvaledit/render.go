// render.go turns a passage of marked-up claim source into the HTML the
// viewer draws.
//
// THE PANEL SHOWS PROSE, NOT SOURCE. A reader looking at a claim sees its
// markdown rendered — bold is bold, a blockquote is indented. A diff of the
// same claim that shows `**like this**` puts the reader in a second document
// written in a notation they were never asked to read, directly under the
// first. Board DON-0 draws the changed passages as rendered prose, and this
// is what makes that possible.
//
// The hard part is marking individual WORDS inside rendered markdown. Marking
// them after rendering would mean doing text surgery on HTML, which is how a
// renderer starts emitting broken tags. So the marks go in BEFORE rendering,
// as two private-use runes that no author can type by accident and that a
// markdown renderer carries through as ordinary text, and are swapped for
// real elements afterwards — but only once the result has been checked (see
// substituteMarks). If the check fails the passage renders unmarked, which is
// a worse diff and never a broken one.
package approvaledit

import (
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
	"github.com/BarterX-Tech/dossierx/internal/textdiff"
)

// markOpen and markClose are U+E000 and U+E001, the first two code points of
// the Basic Multilingual Plane's Private Use Area.
//
// Private-use runes are the right choice for three reasons: Unicode
// guarantees no standard character will ever be assigned to them, so an
// author cannot type one by accident; markdown gives them no meaning, so they
// pass through the renderer as ordinary text; and an HTML escaper leaves them
// alone, so they are still findable in the output. A sentinel made of ASCII
// would fail all three.
const (
	markOpen  = ""
	markClose = ""
)

// renderHunk returns the rendered HTML for one passage and the words marked
// inside it, in reading order, for the label a screen reader is given.
//
// A hunk with no Parts (unchanged, unpaired, or too dissimilar to mark) is
// rendered plainly. That is not a fallback path bolted on: it is the ordinary
// case for every passage a reader did not change.
func renderHunk(h textdiff.Hunk) (rendered string, changedWords []string) {
	text := strings.TrimRight(h.Text, "\n")
	if len(h.Parts) == 0 {
		return string(markdown.Render(text)), nil
	}

	var marked strings.Builder
	var changed []string
	for _, part := range h.Parts {
		if part.Op == textdiff.OpEqual {
			marked.WriteString(part.Text)
			continue
		}
		prefix, core, suffix := splitMarkable(part.Text)
		if core == "" {
			marked.WriteString(part.Text)
			continue
		}
		marked.WriteString(prefix)
		marked.WriteString(markOpen)
		marked.WriteString(core)
		marked.WriteString(markClose)
		marked.WriteString(suffix)
		changed = append(changed, core)
	}

	plain := string(markdown.Render(text))
	withMarks := string(markdown.Render(strings.TrimRight(marked.String(), "\n")))

	// THE MARKS MUST NOT CHANGE THE PROSE. Stripped of the sentinels, the
	// marked render has to be the unmarked render, character for character.
	//
	// This is a check and not a hope because markdown is positional: a
	// sentinel dropped between `internal.` and the `**` that closes its
	// emphasis stops the renderer seeing a pair, and the passage comes back
	// with literal asterisks in it. splitMarkable keeps the sentinels off
	// syntax so that does not happen; this catches every case it did not
	// think of, including ones a future renderer change introduces.
	if strings.NewReplacer(markOpen, "", markClose, "").Replace(withMarks) != plain {
		return plain, nil
	}
	out, ok := substituteMarks(withMarks, h.Op)
	if !ok {
		// The sentinels did not land where they can become elements. Render
		// the passage as prose with no marks rather than emit HTML nobody
		// checked: the reader still sees the two passages and can compare
		// them, which is exactly where this feature started.
		return plain, nil
	}
	return out, changed
}

// splitMarkable divides one word into the markdown syntax that opens it, the
// word itself, and the syntax that closes it.
//
// A mark has to go INSIDE the markup and not around it. `internal.**` is the
// last word of a bold run, and wrapping the whole token puts a sentinel
// between the word and the `**` that closes the emphasis — markdown then sees
// no pair and renders the asterisks as literal text, which is how a diff
// panel starts showing source in the middle of prose. Marking `internal.` and
// leaving the `**` outside keeps the emphasis intact AND puts the highlight
// on the word rather than on the punctuation, which is what a reader is
// looking for anyway.
//
// The set is markdown's own inline delimiters plus the whitespace a token
// carries. It is deliberately a CLOSED set of characters that can only be
// syntax at a word's edge: letters, digits and sentence punctuation are never
// stripped, so "not" and "internal." survive whole.
func splitMarkable(token string) (prefix, core, suffix string) {
	const edge = "*_~`[]()<>\"'“”‘’ \t\n\r"
	core = token
	for core != "" && strings.ContainsRune(edge, rune(core[0])) {
		prefix += core[:1]
		core = core[1:]
	}
	for core != "" && strings.ContainsRune(edge, rune(core[len(core)-1])) {
		suffix = core[len(core)-1:] + suffix
		core = core[:len(core)-1]
	}
	return prefix, core, suffix
}

// substituteMarks replaces the sentinel pairs with real spans, and reports
// false if the rendered HTML is not one it may safely rewrite.
//
// Three things have to hold, and all three are checked rather than assumed:
// every sentinel sits in TEXT and not inside a tag (a span opened inside an
// attribute would corrupt the element); opens and closes strictly alternate;
// and none is left over at the end. Markdown can move text — a reference link
// definition disappears, a fenced block escapes its content — so a sentinel
// that went in is not guaranteed to come out anywhere useful, and guessing
// would mean shipping markup nothing verified.
func substituteMarks(rendered string, op textdiff.Op) (string, bool) {
	class := "claim-edit-word--removed"
	if op == textdiff.OpAdd {
		class = "claim-edit-word--added"
	}
	open := `<span class="claim-edit-word ` + class + `">`

	var b strings.Builder
	b.Grow(len(rendered))
	inTag, inMark := false, false
	for _, r := range rendered {
		switch {
		case r == '<':
			inTag = true
			b.WriteRune(r)
		case r == '>':
			inTag = false
			b.WriteRune(r)
		case r == '':
			if inTag || inMark {
				return "", false
			}
			inMark = true
			b.WriteString(open)
		case r == '':
			if inTag || !inMark {
				return "", false
			}
			inMark = false
			b.WriteString(`</span>`)
		default:
			b.WriteRune(r)
		}
	}
	if inMark {
		return "", false
	}
	return b.String(), true
}
