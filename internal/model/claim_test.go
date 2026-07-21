// claim_test.go covers the YAML round-trip of every model.Claim field and
// the model.Row authored-column-order mechanism (RowColumns / Row's
// UnmarshalYAML+MarshalYAML pair). internal/loader's tests cover the same
// ground end-to-end through actual claim files on disk; these tests
// isolate the model package's own decode/encode behavior directly against
// gopkg.in/yaml.v3, independent of the filesystem.
package model

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func decodeClaim(t *testing.T, doc string) Claim {
	t.Helper()
	var c Claim
	dec := yaml.NewDecoder(strings.NewReader(doc))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		t.Fatalf("decode claim: %v\ndoc:\n%s", err, doc)
	}
	return c
}

func TestClaim_SectionField_RoundTrips(t *testing.T) {
	c := decodeClaim(t, "id: widget.contract.a\nfacet: contract\nstatus: draft\nsection: 5 - workflows / lifecycle\ngoverned_by:\n  type: none\n  reason: fixture\n")
	if c.Section != "5 - workflows / lifecycle" {
		t.Fatalf("Section = %q, want %q", c.Section, "5 - workflows / lifecycle")
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reloaded := decodeClaim(t, string(out))
	if reloaded.Section != c.Section {
		t.Fatalf("Section did not round-trip: got %q, want %q", reloaded.Section, c.Section)
	}
}

func TestClaim_SectionField_OptionalAndOmitted(t *testing.T) {
	c := decodeClaim(t, "id: widget.contract.a\nfacet: contract\nstatus: draft\ngoverned_by:\n  type: none\n  reason: fixture\n")
	if c.Section != "" {
		t.Fatalf("Section = %q, want empty when omitted", c.Section)
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "section:") {
		t.Fatalf("expected omitempty to drop an unset section field, got:\n%s", out)
	}
}

func TestClaim_RawHTMLFields_RoundTrip(t *testing.T) {
	c := decodeClaim(t, "id: widget.contract.a\nfacet: contract\nstatus: draft\nlayout: mockup\nraw_html: \"<div>mock</div>\"\nraw_html_reviewed: true\ngoverned_by:\n  type: none\n  reason: fixture\n")
	if c.Layout != LayoutMockup {
		t.Fatalf("Layout = %q, want %q", c.Layout, LayoutMockup)
	}
	if c.RawHTML != "<div>mock</div>" {
		t.Fatalf("RawHTML = %q, want %q", c.RawHTML, "<div>mock</div>")
	}
	if !c.RawHTMLReviewed {
		t.Fatalf("RawHTMLReviewed = false, want true")
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reloaded := decodeClaim(t, string(out))
	if reloaded.RawHTML != c.RawHTML {
		t.Fatalf("RawHTML did not round-trip: got %q, want %q", reloaded.RawHTML, c.RawHTML)
	}
	if reloaded.RawHTMLReviewed != c.RawHTMLReviewed {
		t.Fatalf("RawHTMLReviewed did not round-trip: got %v, want %v", reloaded.RawHTMLReviewed, c.RawHTMLReviewed)
	}
	if reloaded.Layout != c.Layout {
		t.Fatalf("Layout did not round-trip: got %q, want %q", reloaded.Layout, c.Layout)
	}
}

func TestClaim_RawHTMLFields_OptionalAndOmitted(t *testing.T) {
	c := decodeClaim(t, "id: widget.contract.a\nfacet: contract\nstatus: draft\ngoverned_by:\n  type: none\n  reason: fixture\n")
	if c.RawHTML != "" {
		t.Fatalf("RawHTML = %q, want empty when omitted", c.RawHTML)
	}
	if c.RawHTMLReviewed {
		t.Fatalf("RawHTMLReviewed = true, want false (default) when omitted")
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "raw_html:") || strings.Contains(string(out), "raw_html_reviewed:") {
		t.Fatalf("expected omitempty to drop unset raw_html/raw_html_reviewed fields, got:\n%s", out)
	}
}

func TestLayoutMockup_IsAValidLayoutValue(t *testing.T) {
	if LayoutMockup != "mockup" {
		t.Fatalf("LayoutMockup = %q, want %q", LayoutMockup, "mockup")
	}
	c := decodeClaim(t, "id: widget.contract.a\nfacet: contract\nstatus: draft\nlayout: mockup\ngoverned_by:\n  type: none\n  reason: fixture\n")
	if c.Layout != LayoutMockup {
		t.Fatalf("Layout = %q, want %q", c.Layout, LayoutMockup)
	}
}

// --- Row authored-column-order ---

func decodeRow(t *testing.T, doc string) Row {
	t.Helper()
	var r Row
	if err := yaml.Unmarshal([]byte(doc), &r); err != nil {
		t.Fatalf("decode row: %v\ndoc:\n%s", err, doc)
	}
	return r
}

func TestRow_UnmarshalYAML_PreservesAuthoredOrder(t *testing.T) {
	// Deliberately not alphabetically sortable, so a bug that falls back
	// to sort.Strings would be caught.
	r := decodeRow(t, "zeta: 1\nalpha: 2\nmiddle: 3\n")

	got := RowColumns(r)
	want := []string{"zeta", "alpha", "middle"}
	if len(got) != len(want) {
		t.Fatalf("RowColumns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RowColumns = %v, want %v", got, want)
		}
	}

	// The real column data must still be present and correct, keyed
	// normally.
	if r["zeta"] != 1 || r["alpha"] != 2 || r["middle"] != 3 {
		t.Fatalf("row data wrong: %+v", r)
	}
}

func TestRow_UnmarshalYAML_RejectsNonMapping(t *testing.T) {
	var r Row
	err := yaml.Unmarshal([]byte("- not\n- a\n- mapping\n"), &r)
	if err == nil {
		t.Fatal("expected an error decoding a non-mapping node into Row, got nil")
	}
}

func TestRow_UnmarshalYAML_RejectsUndecodableKey(t *testing.T) {
	// A complex-mapping key (YAML's "? key" explicit-key form) cannot
	// decode into Go's string, so this must surface as an error rather
	// than panicking or silently dropping the entry.
	var r Row
	err := yaml.Unmarshal([]byte("? {a: 1}\n: 2\n"), &r)
	if err == nil {
		t.Fatal("expected an error decoding a non-scalar row key, got nil")
	}
	if !strings.Contains(err.Error(), "model: row key") {
		t.Fatalf("error = %q, want it to mention %q", err.Error(), "model: row key")
	}
}

func TestRowColumns_NilWhenOrderKeyHoldsWrongType(t *testing.T) {
	// A hand-built Row that happens to carry the reserved order key with
	// a value that isn't a []string (something UnmarshalYAML itself would
	// never produce, but RowColumns must still degrade gracefully rather
	// than panicking on the failed type assertion).
	r := Row{"a": 1, rowOrderKey: "not-a-slice"}
	if got := RowColumns(r); got != nil {
		t.Fatalf("RowColumns = %v, want nil when the order key holds the wrong type", got)
	}
}

func TestRow_MarshalYAML_StripsOrderKeyWhenItHoldsWrongType(t *testing.T) {
	// Same malformed-row shape as above, exercised through MarshalYAML:
	// RowColumns returns nil (wrong type), but the reserved key is still
	// present, so MarshalYAML must take its "strip it back out" path
	// rather than its "no order captured at all" fast path.
	r := Row{"a": 1, rowOrderKey: "not-a-slice"}

	out, err := yaml.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "\x00") || strings.Contains(string(out), "order") {
		t.Fatalf("marshaled row leaked the malformed reserved order key, got:\n%s", out)
	}

	var reloaded map[string]any
	if err := yaml.Unmarshal(out, &reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(reloaded) != 1 || reloaded["a"] != 1 {
		t.Fatalf("reloaded = %+v, want only {a: 1}", reloaded)
	}
}

func TestRowColumns_NilForHandBuiltRow(t *testing.T) {
	r := Row{"b": 1, "a": 2}
	if got := RowColumns(r); got != nil {
		t.Fatalf("RowColumns(hand-built row) = %v, want nil", got)
	}
}

func TestRow_MarshalYAML_RoundTripsAuthoredOrder(t *testing.T) {
	r := decodeRow(t, "zeta: 1\nalpha: 2\nmiddle: 3\n")

	out, err := yaml.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The reserved order-tracking entry must never leak onto disk.
	if strings.Contains(string(out), "\x00") {
		t.Fatalf("marshaled row leaked the reserved order key, got:\n%s", out)
	}

	// Re-decoding must reproduce the same authored order, and the
	// authored order must literally match the line order in the marshaled
	// YAML (not merely be recoverable via RowColumns) — this is what
	// "renders in authored order" cashes out to for a value that flows
	// straight from YAML to YAML, without going through render/components.
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	wantOrder := []string{"zeta", "alpha", "middle"}
	if len(lines) != len(wantOrder) {
		t.Fatalf("marshaled row has %d lines, want %d:\n%s", len(lines), len(wantOrder), out)
	}
	for i, key := range wantOrder {
		if !strings.HasPrefix(lines[i], key+":") {
			t.Fatalf("marshaled row line %d = %q, want it to start with %q", i, lines[i], key+":")
		}
	}

	reloaded := decodeRow(t, string(out))
	got := RowColumns(reloaded)
	if len(got) != len(wantOrder) {
		t.Fatalf("RowColumns after round-trip = %v, want %v", got, wantOrder)
	}
	for i := range wantOrder {
		if got[i] != wantOrder[i] {
			t.Fatalf("RowColumns after round-trip = %v, want %v", got, wantOrder)
		}
	}
}

func TestClaim_EffectiveKind(t *testing.T) {
	cases := []struct {
		name string
		c    Claim
		want Kind
	}{
		{"default fact", Claim{Facet: "contract"}, KindFact},
		{"explicit orientation-note", Claim{Facet: "contract", Kind: KindOrientationNote}, KindOrientationNote},
		{"overview facet implies orientation-note", Claim{Facet: "overview"}, KindOrientationNote},
		{"overview facet with explicit kind stays orientation-note", Claim{Facet: "overview", Kind: KindOrientationNote}, KindOrientationNote},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.EffectiveKind(); got != tc.want {
				t.Errorf("EffectiveKind() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClaim_RowsField_PreservesAuthoredOrderThroughFullClaimRoundTrip(t *testing.T) {
	doc := "id: widget.contract.a\nfacet: contract\nstatus: draft\nlayout: table\n" +
		"rows:\n  - zeta: 1\n    alpha: 2\n    middle: 3\n  - zeta: 4\n    alpha: 5\n    middle: 6\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	c := decodeClaim(t, doc)

	if len(c.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(c.Rows))
	}
	wantOrder := []string{"zeta", "alpha", "middle"}
	for i, row := range c.Rows {
		got := RowColumns(row)
		if len(got) != len(wantOrder) {
			t.Fatalf("row %d RowColumns = %v, want %v", i, got, wantOrder)
		}
		for j := range wantOrder {
			if got[j] != wantOrder[j] {
				t.Fatalf("row %d RowColumns = %v, want %v", i, got, wantOrder)
			}
		}
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reloaded := decodeClaim(t, string(out))
	for i, row := range reloaded.Rows {
		got := RowColumns(row)
		for j := range wantOrder {
			if got[j] != wantOrder[j] {
				t.Fatalf("reloaded row %d RowColumns = %v, want %v", i, got, wantOrder)
			}
		}
	}
}
