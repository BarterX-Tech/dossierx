package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

// hostile is the string that has to survive the whole pipeline as DATA. It
// is what an author gets by naming a claim slug or a facet after it, which
// is reachable under "dossierx serve" because serve never lints.
const hostile = `</script><img src=x>`

// TestEncodeEscapesScriptClose is the first test written in this package,
// deliberately, and it is written against the one property that turns a
// documentation viewer into an XSS vector.
//
// The payload is injected as an html/template template.JS value inside a
// <script type="application/json"> block. html/template applies NO escaping
// in that context, so if a literal script-closing tag reaches the template it
// writes straight out of the block and everything after it parses as HTML.
// encoding/json's default HTML escaping is the entire guard: it writes '<'
// as \u003c and '>' as \u003e, which a JSON parser reads back as the original
// characters and an HTML parser never sees as a tag at all.
//
// The three assertions below are not three styles of the same check. The
// first says the breakout cannot happen. The second says the reason it cannot
// happen is escaping — and not, say, the string having been stripped or
// dropped, which would also satisfy the first. The third says the escaping is
// lossless, because the pane's detail panel shows the author their real facet
// name and a mangled one would be its own bug.
func TestEncodeEscapesScriptClose(t *testing.T) {
	// A claim id whose slug segment yields a hostile derived label, plus a
	// facet of the same shape. Both are author-authored YAML scalars.
	id := "widget.contract." + hostile
	label := claimLabel(id)
	if !strings.Contains(label, "</script>") {
		t.Fatalf("precondition: derived label %q does not carry the hostile sequence; this test would pass vacuously", label)
	}

	p := Payload{
		Schema: SchemaVersion,
		Nodes: []Node{{
			ID:     id,
			Title:  label,
			Module: "widget",
			Facet:  hostile,
			Status: "draft",
			Kind:   "fact",
		}},
		Edges:  []Edge{},
		Groups: Groups{Modules: []string{"widget"}, Facets: []string{hostile}},
	}

	out, err := Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := string(out)

	if strings.Contains(got, "</script>") {
		t.Errorf("encoded payload contains a literal </script>, which breaks out of the JSON script block:\n%s", got)
	}
	if !strings.Contains(got, `\u003c/script`) {
		t.Errorf("encoded payload does not contain the escaped form \\u003c/script; the hostile string was not escaped (was it dropped?):\n%s", got)
	}

	var back Payload
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("json.Unmarshal of encoded payload: %v", err)
	}
	if len(back.Nodes) != 1 {
		t.Fatalf("round-trip node count = %d, want 1", len(back.Nodes))
	}
	if back.Nodes[0].Title != label {
		t.Errorf("round-trip title = %q, want %q (escaping must be lossless)", back.Nodes[0].Title, label)
	}
	if back.Nodes[0].Facet != hostile {
		t.Errorf("round-trip facet = %q, want %q (escaping must be lossless)", back.Nodes[0].Facet, hostile)
	}
	if len(back.Groups.Facets) != 1 || back.Groups.Facets[0] != hostile {
		t.Errorf("round-trip groups.facets = %#v, want [%q]", back.Groups.Facets, hostile)
	}
}

// TestEncodeHasNoTrailingNewline pins the difference between json.Marshal and
// json.Encoder that internal/serve's "the endpoint's bytes equal the inline
// block's bytes" test rests on: Encoder appends a newline, Marshal does not.
func TestEncodeHasNoTrailingNewline(t *testing.T) {
	out, err := Encode(Payload{Schema: SchemaVersion, Nodes: []Node{}, Edges: []Edge{}})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(out) == 0 || out[len(out)-1] == '\n' {
		t.Errorf("Encode output ends with a newline; json.Encoder semantics leaked in")
	}
}
