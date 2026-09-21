// fields.go diffs the persisted claim fields that are NOT the body.
//
// It exists for the case the body diff cannot describe at all. A claim's hash
// covers every field it persists, so a claim can be honestly "edited since it
// was approved" with its prose untouched: its checks were retargeted, an
// edge was added, an audit note was written. On a corpus that has been through
// a code audit this is not a corner — in the corpus this was built against it
// is six of the fourteen affected claims. A panel that shows a prose diff and
// nothing else tells those six readers only that something changed, which is
// the state the panel was built to end.
//
// The rendering is deliberately NOT the body's. A field is not prose: it is
// the YAML a reader would see if they opened the file, and running it through
// the markdown renderer would turn a list into a list, a `*` into emphasis,
// and a nested map into a paragraph. So a field is marshalled back to YAML and
// escaped as text, and only the word marks are added — the same passage and
// word diff the body uses, with a different renderer at the end of it.
package approvaledit

import (
	"html"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/textdiff"
)

// MaxFieldBytes bounds the YAML one field is diffed as.
//
// A claim field is small in every corpus this has been run against — the
// largest embodiment block is a few kilobytes — but nothing in the schema caps
// one, and a diff is quadratic in the worst case over what it is given. Past
// this size the field is reported as changed WITHOUT a diff rather than
// diffed slowly: the row still appears and says the field moved, and
// Truncated says the comparison was not shown. A silent omission here would
// be a field a reader is never told about.
//
// It is a var rather than a const for the same reason the recovery cap is:
// the truncation BEHAVIOUR has to be executed by a test, and a fixture large
// enough to trip a 32 KiB const would be a fixture nobody reads.
var MaxFieldBytes = 32 * 1024

// FieldChange is one non-body persisted field that differs from the approval.
type FieldChange struct {
	// Field is the field's on-disk name — "embodiment", "rests_on" — which is
	// what a reader would search the YAML file for.
	Field string `json:"field"`

	// Hunks is the field's YAML, before and after, as escaped text with the
	// words that moved marked. Empty when Truncated is set.
	Hunks []Hunk `json:"hunks,omitempty"`

	// Truncated says the field was too large to diff (see MaxFieldBytes), so
	// the row states that the field moved and shows nothing. A renderer must
	// say so rather than draw an empty comparison.
	Truncated bool `json:"truncated,omitempty"`
}

// fieldChanges renders one FieldChange per name, in the order given.
//
// names comes from lock.SignedFieldsDiffering, so every entry is a field the
// approval's hash actually signs and actually differs — this does not decide
// what changed, only how to show it.
func fieldChanges(approved, current model.Claim, names []string) []FieldChange {
	var out []FieldChange
	for _, name := range names {
		before, okA := fieldYAML(approved, name)
		after, okB := fieldYAML(current, name)
		if !okA || !okB {
			// A name the reflection no longer resolves is a field that left
			// the schema between the approval and now. Say it moved; there
			// is nothing honest to draw.
			out = append(out, FieldChange{Field: name, Truncated: true})
			continue
		}
		if len(before) > MaxFieldBytes || len(after) > MaxFieldBytes {
			out = append(out, FieldChange{Field: name, Truncated: true})
			continue
		}
		change := FieldChange{Field: name}
		for _, h := range textdiff.MarkWords(textdiff.Blocks(before, after)) {
			rendered, changed := renderFieldHunk(h)
			if strings.TrimSpace(rendered) == "" {
				continue
			}
			change.Hunks = append(change.Hunks, Hunk{Op: string(h.Op), HTML: rendered, Changed: changed})
		}
		out = append(out, change)
	}
	return out
}

// fieldYAML marshals one persisted field's value back to the YAML it is stored
// as.
//
// The value is marshalled alone rather than under its own key: the field name
// is already the row's label, and repeating it inside the block would put a
// line in every diff that can never differ.
func fieldYAML(c model.Claim, name string) (string, bool) {
	value, ok := lock.SignedFieldValue(c, name)
	if !ok {
		return "", false
	}
	raw, err := yaml.Marshal(value)
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(raw), "\n"), true
}

// renderFieldHunk is renderHunk's counterpart for field YAML: it escapes the
// text and marks the words that moved, and runs no markdown renderer.
//
// Escaping first is what makes the mark substitution safe here. Once the text
// is escaped there is no `<` left in it, so substituteMarks's in-tag check can
// never fire on content — the only angle brackets it can meet are the ones
// this function is itself producing. A field holding `<script>` comes out as
// text, exactly as it would in the claim file.
func renderFieldHunk(h textdiff.Hunk) (rendered string, changedWords []string) {
	text := strings.TrimRight(h.Text, "\n")
	if len(h.Parts) == 0 {
		return html.EscapeString(text), nil
	}

	var marked strings.Builder
	var changed []string
	for _, part := range h.Parts {
		if part.Op == textdiff.OpEqual {
			marked.WriteString(html.EscapeString(part.Text))
			continue
		}
		prefix, core, suffix := splitMarkableAt(part.Text, yamlEdge)
		if core == "" {
			marked.WriteString(html.EscapeString(part.Text))
			continue
		}
		marked.WriteString(html.EscapeString(prefix))
		marked.WriteString(markOpen)
		marked.WriteString(html.EscapeString(core))
		marked.WriteString(markClose)
		marked.WriteString(html.EscapeString(suffix))
		changed = append(changed, core)
	}

	out, ok := substituteMarks(strings.TrimRight(marked.String(), "\n"), h.Op)
	if !ok {
		// Unbalanced sentinels mean the marking cannot be trusted. Show the
		// field unmarked rather than emit spans nothing checked; the reader
		// still sees both versions side by side.
		return html.EscapeString(text), nil
	}
	return out, changed
}
