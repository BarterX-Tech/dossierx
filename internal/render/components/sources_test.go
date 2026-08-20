package components

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// sources_test.go covers the evidence half of the claim footer and the anchor
// it shares with the body renderer. The single most important assertion in the
// file is TestEdges_NoSourcesRendersIdentically: everything else describes a
// new capability, and that one describes what happens to every project that
// never asked for it.

func sourcedClaim(sources ...model.Source) model.Claim {
	return model.Claim{
		ID:      "widget.contract.retry",
		Module:  "widget",
		Facet:   "contract",
		Status:  model.StatusDraft,
		Layout:  model.LayoutCard,
		Body:    "body",
		Sources: sources,
	}
}

// TestEdges_NoSourcesRendersIdentically is the zero-cost contract at the one
// place it could break: the footer emitter itself. A claim with no sources must
// produce the same bytes it did before the feature existed — no "0 sources"
// segment, no empty <li>, no <ul> — because that is every claim in every corpus
// that has not adopted citations.
func TestEdges_NoSourcesRendersIdentically(t *testing.T) {
	c := sourcedClaim()
	c.RestsOn = []string{"widget.contract.other"}
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	for _, absent := range []string{"claim-sources", "claim-source-list", "source</summary>", "sources</summary>", "0 sources"} {
		if strings.Contains(got, absent) {
			t.Errorf("a source-less claim emitted %q: %s", absent, got)
		}
	}
	if want := `<summary class="claim-links-summary">1 link - 0 files</summary>`; !strings.Contains(got, want) {
		t.Errorf("expected the untouched summary %q, got: %s", want, got)
	}
}

// TestEdges_SourcesOpenTheFooterAlone pins the suppression rule's new leg: a
// claim whose ONLY footer content is its evidence must still be able to
// disclose it. Before sources joined the test, such a claim emitted no
// <details> at all and the citations in its body pointed at nothing.
func TestEdges_SourcesOpenTheFooterAlone(t *testing.T) {
	c := sourcedClaim(model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "A page", URL: "http" + "s://example.test/p", AccessedOn: "2026-01-02"})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if !strings.Contains(got, `<details class="claim-links"`) {
		t.Fatalf("a claim whose only footer content is a source emitted no disclosure: %s", got)
	}
	if want := `<summary class="claim-links-summary">0 links - 0 files - 1 source</summary>`; !strings.Contains(got, want) {
		t.Fatalf("expected the summary %q, got: %s", want, got)
	}
}

// TestEdges_SourceCountIsPluralisedAndOrdered holds the digest's shape: the
// sources segment is singular at exactly one, rides after "files", and appears
// before "drifted" so the adjective stays next to the count it qualifies.
func TestEdges_SourceCountIsPluralisedAndOrdered(t *testing.T) {
	c := sourcedClaim(
		model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "One"},
		model.Source{Ref: 2, Kind: model.SourceKindExternal, Title: "Two"},
	)
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if want := `<summary class="claim-links-summary">0 links - 0 files - 2 sources</summary>`; !strings.Contains(got, want) {
		t.Fatalf("expected %q, got: %s", want, got)
	}
}

// TestEdges_ExternalSourceRendersItsAnchoringFields checks that the two fields
// that make an external citation falsifiable — the URL and the date it was read
// — both reach the page. A citation the reader cannot go and refute is the
// failure model.Source's doc comment names.
func TestEdges_ExternalSourceRendersItsAnchoringFields(t *testing.T) {
	url := "http" + "s://docs.example.test/retry"
	c := sourcedClaim(model.Source{
		Ref:        1,
		Kind:       model.SourceKindExternal,
		Title:      "Retry semantics",
		URL:        url,
		AccessedOn: "2026-01-02",
		Supports:   "the three-attempt budget",
	})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	for _, want := range []string{
		`<li class="claim-source" id="widget.contract.retry-source-1">`,
		`<span class="claim-source-ref">[1]</span>`,
		`<a class="claim-source-title" href="` + url + `">Retry semantics</a>`,
		`docs.example.test`,
		`accessed 2026-01-02`,
		`<span class="claim-source-note-label">supports:</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the footer, got: %s", want, got)
		}
	}
}

// TestEdges_InternalSourceRendersItsAnchoringFields is the internal twin: the
// path (with its optional record) and a recognizable prefix of the hash that
// pins it.
func TestEdges_InternalSourceRendersItsAnchoringFields(t *testing.T) {
	c := sourcedClaim(model.Source{
		Ref:            2,
		Kind:           model.SourceKindInternal,
		Title:          "Extraction ledger",
		Path:           "research/ledger.jsonl",
		RecordID:       "rec-14",
		SHA256:         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		DoesNotSupport: "the retry jitter",
	})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	for _, want := range []string{
		`id="widget.contract.retry-source-2"`,
		`<code class="claim-source-anchor">research/ledger.jsonl#rec-14</code>`,
		`>0123456789ab<`,
		`<span class="claim-source-note-label">does_not_support:</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the footer, got: %s", want, got)
		}
	}
	if strings.Contains(got, ">0123456789abc<") {
		t.Errorf("the hash was not truncated to %d characters: %s", sourceHashPrefixLen, got)
	}
}

// TestEdges_SourceURLPassesTheHrefGate holds the one place this feature could
// have opened a second unescaped path into the document. A URL urlsafe refuses
// produces NO anchor: the title stays plain text and the refused string is shown
// as escaped literal text, exactly as a refused markdown link is.
func TestEdges_SourceURLPassesTheHrefGate(t *testing.T) {
	c := sourcedClaim(model.Source{
		Ref:   1,
		Kind:  model.SourceKindExternal,
		Title: "Hostile",
		URL:   "javascript:alert(1)",
	})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if strings.Contains(got, `href="javascript:`) {
		t.Fatalf("a refused scheme reached an href: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-source-title">Hostile</span>`) {
		t.Errorf("expected the title as plain text under a refused URL, got: %s", got)
	}
	if !strings.Contains(got, `javascript:alert(1)`) {
		t.Errorf("expected the refused URL shown as literal text, got: %s", got)
	}
}

// TestEdges_SourceFieldsAreEscaped checks the hand-escaping this file's markup
// depends on. A FuncMap-returned template.HTML bypasses html/template, so
// nothing downstream would catch a missed field.
func TestEdges_SourceFieldsAreEscaped(t *testing.T) {
	c := sourcedClaim(model.Source{
		Ref:      1,
		Kind:     model.SourceKindInternal,
		Title:    `<script>t</script>`,
		Path:     `notes/<img src=x>.md`,
		RecordID: `"><b>`,
		SHA256:   `<hash>`,
	})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	for _, forbidden := range []string{"<script>", "<img src=x>", "<b>", "<hash>"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("unescaped %q reached the footer: %s", forbidden, got)
		}
	}
}

// TestEdges_SourceNotesRunThroughTheInlineRenderer pins the ceiling the two
// authored boundary lines are held to: the same code-span-and-link subset
// governed.reason gets, and no block constructs.
func TestEdges_SourceNotesRunThroughTheInlineRenderer(t *testing.T) {
	c := sourcedClaim(model.Source{
		Ref:      1,
		Kind:     model.SourceKindExternal,
		Title:    "A page",
		Supports: "the `retry_budget` field",
	})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if !strings.Contains(got, "<code>retry_budget</code>") {
		t.Errorf("expected the note's code span rendered, got: %s", got)
	}
	if strings.Contains(got, "<p>") {
		t.Errorf("a note produced a block construct: %s", got)
	}
}

// TestEdges_SourceNoteShipsWholeWithAHiddenControl is the degradation contract
// for the three-line clamp, asserted at the only place it is decided: the
// emitted bytes. The server cannot know whether a note runs past three lines —
// that depends on the reader's box — so it ships the note WHOLE and the control
// HIDDEN, and the viewer's script applies the clamp and reveals the button on
// the notes that earn one.
//
// The direction matters more than the markup. A page whose script never ran — a
// printout, a text browser, a reader who blocks it — must show the citation's
// stated limit in full rather than truncate it behind a button that cannot
// work, so `hidden` here and no clamp class anywhere is the whole guarantee.
func TestEdges_SourceNoteShipsWholeWithAHiddenControl(t *testing.T) {
	c := sourcedClaim(model.Source{
		Ref:            1,
		Kind:           model.SourceKindExternal,
		Title:          "A page",
		Supports:       "the retry budget is three",
		DoesNotSupport: "says nothing about the backoff curve",
	})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if n := strings.Count(got, `<button class="claim-source-note-toggle"`); n != 2 {
		t.Fatalf("note controls = %d, want one per authored note: %s", n, got)
	}
	if n := strings.Count(got, `aria-expanded="false"`); n != 2 {
		t.Errorf("controls not shipped collapsed: %s", got)
	}
	if n := strings.Count(got, ` hidden>`); n != 2 {
		t.Errorf("controls not shipped hidden — a page with no script would truncate: %s", got)
	}
	if !strings.Contains(got, `data-collapse-label="show less"`) || !strings.Contains(got, `>show more</button>`) {
		t.Errorf("expected both labels written by this package: %s", got)
	}
	// The clamp is a runtime class and must never be in the emitted bytes.
	if strings.Contains(got, "is-clamped") {
		t.Errorf("the server applied the clamp itself: %s", got)
	}
	// The text stays inside a wrapper the clamp can be applied to WITHOUT
	// swallowing the control, which is why the wrapper exists at all.
	if !strings.Contains(got, `<div class="claim-source-note-body">`) {
		t.Errorf("expected the clampable body wrapper: %s", got)
	}
}

// TestEdges_SourceNoteControlCarriesNoID is a constraint from a different
// feature, pinned here because nothing else would notice it breaking. A claim
// owned by a track is rendered a SECOND time inside that track's section, and
// render.stripDuplicateClaimIDs removes only the ids it can enumerate — the
// claim's own and its source rows'. An id on this control would survive into
// the copy, putting a duplicate id in the document; the control is wired by DOM
// position precisely so that it cannot.
func TestEdges_SourceNoteControlCarriesNoID(t *testing.T) {
	c := sourcedClaim(model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "A page", Supports: "a note"})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	toggle := strings.Index(got, `<button class="claim-source-note-toggle"`)
	if toggle < 0 {
		t.Fatalf("no control emitted: %s", got)
	}
	end := strings.Index(got[toggle:], ">")
	if end < 0 {
		t.Fatalf("unterminated control tag: %s", got)
	}
	if tag := got[toggle : toggle+end]; strings.Contains(tag, ` id="`) {
		t.Errorf("the note control carries an id, which a track copy would duplicate: %s", tag)
	}
}

// TestEdges_SourceWithoutNotesEmitsNoControl keeps the clutter rule honest one
// level up from the clamp: a source that states no boundary line has nothing to
// expand, so it must not carry a control at all.
func TestEdges_SourceWithoutNotesEmitsNoControl(t *testing.T) {
	c := sourcedClaim(model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "A page", URL: "http" + "s://example.test/p"})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	for _, absent := range []string{"claim-source-note", "claim-source-note-toggle", "show more"} {
		if strings.Contains(got, absent) {
			t.Errorf("a note-less source emitted %q: %s", absent, got)
		}
	}
}

// TestClaimSourceAnchor_RefusesAnUnroutableID holds the closed-character-set
// gate. An id that cannot be one HTML id and one URL fragment with the same
// bytes loses its anchors — and keeps its sources, which is the half that
// matters.
func TestClaimSourceAnchor_RefusesAnUnroutableID(t *testing.T) {
	c := sourcedClaim(model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "A page"})
	c.ID = `widget/contract#retry`

	if _, ok := ClaimSourceAnchorPrefix(c); ok {
		t.Fatalf("expected an id outside the closed set to be refused an anchor prefix")
	}
	if got := ClaimSourceAnchorID(c, 1); got != "" {
		t.Errorf("ClaimSourceAnchorID on a refused id = %q, want \"\"", got)
	}

	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if strings.Contains(got, ` id="`) {
		t.Errorf("a refused id still emitted an anchor: %s", got)
	}
	if !strings.Contains(got, `<li class="claim-source">`) {
		t.Errorf("the source row itself must still render under a refused id, got: %s", got)
	}
}

// TestClaimMarkdown_MarkersResolveAgainstTheFooterAnchor is the end-to-end
// agreement between the two halves: the href the body renderer emits and the id
// the footer emits are the same string. They are built by one function, and this
// is what would notice if they ever stopped being.
func TestClaimMarkdown_MarkersResolveAgainstTheFooterAnchor(t *testing.T) {
	c := sourcedClaim(model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "A page"})
	c.Body = "the budget is three [1]."

	body := string(claimMarkdown(c, c.Body))
	footer := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	anchor := ClaimSourceAnchorID(c, 1)
	if anchor == "" {
		t.Fatal("expected a routable id to yield an anchor")
	}
	if !strings.Contains(body, `href="#`+anchor+`"`) {
		t.Errorf("the body marker does not name the anchor %q: %s", anchor, body)
	}
	if !strings.Contains(footer, ` id="`+anchor+`"`) {
		t.Errorf("the footer row does not carry the anchor %q: %s", anchor, footer)
	}
}
