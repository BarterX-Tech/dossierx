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

// TestEdges_NoSourcesRendersItsOwnEmptyDoor is the RETRY re-pin (fix-list
// item 10) of what used to be TestEdges_NoSourcesRendersIdentically. That
// test pinned the OPPOSITE of the correct behaviour: 05 §8 item 5's "an
// entirely edgeless, sourceless, checkless claim still shows four zeros and
// the comment count" and 07a §6/§8's "Sources present" state (which is
// explicit that the wording changes FROM "No sources" TO "N sources" once
// len(c.Sources) > 0, implying a distinct zero-state wording exists) both
// say a source-less claim's SOURCES door renders — worded "No sources",
// never a "0 sources" numeral, never absent. The evidence that drove this
// fix: 852 rendered footers against only 290 sources chips, because the old
// gate suppressed the whole sources door at zero.
// 05 §4.11/R09.5: sources is its own peer door, <details class="claim-
// sources">, split out of relationships, with its own chip — never a
// segment of the relationships chip's text.
func TestEdges_NoSourcesRendersPlainTruthfulZeroState(t *testing.T) {
	c := sourcedClaim()
	c.RestsOn = model.RestsOnIDs("widget.contract.other")
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if strings.Contains(got, `<details class="claim-sources"`) {
		t.Fatalf("zero sources must not render an empty disclosure, got: %s", got)
	}
	if want := `<span class="claim-footer-chip-label">No sources</span>`; !strings.Contains(got, want) {
		t.Errorf("expected the zero-sources chip to read %q, got: %s", want, got)
	}
	if strings.Contains(got, "claim-source-list") {
		t.Errorf("a source-less claim's sources door should have no <ul> of rows: %s", got)
	}
	if strings.Contains(got, "0 sources") {
		t.Errorf("zero sources must read \"No sources\", never the numeral form: %s", got)
	}
	const emptyChip = `<span class="claim-footer-chip claim-footer-chip--sources claim-footer-chip--empty"><span class="claim-footer-chip-label">No sources</span></span>`
	if !strings.Contains(got, emptyChip) {
		t.Fatalf("expected the zero-sources chip to carry the --empty modifier, got: %s", got)
	}
	if strings.Contains(got, `claim-footer-chip--sources claim-footer-chip--empty"><span class="claim-footer-chip-label">No sources</span><svg`) {
		t.Errorf("the zero-sources chip must be chevron-less, got: %s", got)
	}
	if want := `<span class="claim-footer-chip-label">1 relationship</span>`; !strings.Contains(got, want) {
		t.Errorf("expected the untouched relationships chip %q, got: %s", want, got)
	}
}

// TestEdges_SourcesOpenTheFooterAlone pins the suppression rule's new leg: a
// claim whose ONLY footer content is its evidence must still be able to
// disclose it, as its own door — with no relationships door alongside it,
// since this claim has zero relationships (05 §4.11/R09.5).
func TestEdges_SourcesOpenTheFooterAlone(t *testing.T) {
	c := sourcedClaim(model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "A page", URL: "http" + "s://example.test/p", AccessedOn: "2026-01-02"})
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if !strings.Contains(got, `<details class="claim-sources"`) {
		t.Fatalf("a claim whose only footer content is a source emitted no sources door: %s", got)
	}
	if want := `<span class="claim-footer-chip-label">1 source</span>`; !strings.Contains(got, want) {
		t.Fatalf("expected the sources chip %q, got: %s", want, got)
	}
	if want := `<span class="claim-footer-chip-label">No relationships</span>`; !strings.Contains(got, want) {
		t.Fatalf("expected the truthful plain zero-relationships state, got: %s", got)
	}
}

// TestEdges_SourceCountIsPluralisedAndOrdered holds the sources chip's
// pluralisation.
func TestEdges_SourceCountIsPluralisedAndOrdered(t *testing.T) {
	c := sourcedClaim(
		model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "One"},
		model.Source{Ref: 2, Kind: model.SourceKindExternal, Title: "Two"},
	)
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if want := `<span class="claim-footer-chip-label">2 sources</span>`; !strings.Contains(got, want) {
		t.Fatalf("expected %q, got: %s", want, got)
	}
	if want := `<svg class="claim-footer__chevron" aria-hidden="true" viewBox="0 0 24 24" fill="none"><path d="m6 9 6 6 6-6"/></svg>`; !strings.Contains(got, want) {
		t.Fatalf("populated footer doors must use the Paper 12x12 SVG chevron, got: %s", got)
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
// every other footer prose field gets, and no block constructs.
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
	if n := strings.Count(got, `class="claim-source-note-toggle" type="button" aria-expanded="false"`); n != 2 {
		t.Errorf("source-note controls not shipped collapsed: %s", got)
	}
	if n := strings.Count(got, `data-collapse-label="show less" hidden>show more</button>`); n != 2 {
		t.Errorf("source-note controls not shipped hidden — a page with no script would truncate: %s", got)
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

// ---------------------------------------------------------------------
// RETRY ADDITION (fix-list item 11) — the citation-count column. None of
// this surface existed before this pass: no counter, no column, no wording.
// 05 §4.11's "cited once" / D12's "derived from the count of `[n]` markers
// in the claim body … computed at render time, never authored".
// ---------------------------------------------------------------------

// TestCitedLabel pins D12's exact wording for every count, including the
// zero case (a source nobody has cited yet still shows a count, never a
// blank column — see writeSourcesRow).
func TestCitedLabel(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "cited 0 times"},
		{1, "cited once"},
		{2, "cited 2 times"},
		{10, "cited 10 times"},
	}
	for _, tc := range cases {
		if got := citedLabel(tc.n); got != tc.want {
			t.Errorf("citedLabel(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestSourcesCitationCounts_CountsOnlyResolvedMarkers is the whole argument
// for rendering the body to count rather than scanning it for "[n]"
// substrings by hand: a marker only counts when it is one this claim's
// sources can actually resolve (markdown_cite.go's recognition rules), so a
// ref naming no source, and a bracketed number that merely LOOKS like a
// marker inside a fenced code block, must not inflate any source's count.
func TestSourcesCitationCounts_CountsOnlyResolvedMarkers(t *testing.T) {
	c := sourcedClaim(
		model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "One"},
		model.Source{Ref: 2, Kind: model.SourceKindExternal, Title: "Two"},
	)
	c.Body = "budget [1] and retries [1] and [7] and an array `array[2]` access, " +
		"then a fence:\n\n```\n[2] not a marker in here\n```\n"

	counts := sourcesCitationCounts(c)
	if got := counts[1]; got != 2 {
		t.Errorf("ref 1: got %d resolved citations, want 2", got)
	}
	if got := counts[2]; got != 0 {
		t.Errorf("ref 2: got %d resolved citations, want 0 (its only two spellings are a code span and a fence, neither of which the inline scanner ever reaches)", got)
	}
}

// TestSourcesCitationCounts_NoSourcesIsNil holds the same zero-cost shape
// every capability in this family has: a claim that cannot carry citations
// gets a nil map, not an empty one that happens to behave the same — nil is
// what counts[anything] reads as 0 through anyway, so callers need no special
// case, but the function must never panic or render the body needlessly for
// a claim with nothing to count.
func TestSourcesCitationCounts_NoSourcesIsNil(t *testing.T) {
	c := sourcedClaim()
	c.Body = "see [1] for details"
	if got := sourcesCitationCounts(c); got != nil {
		t.Errorf("expected a nil map for a claim with no sources, got: %#v", got)
	}
}

// TestWriteSourcesRow_CitationCountColumn is the end-to-end check: the
// rendered footer carries the right "cited once" / "cited N times" text for
// each source, matching how many times its ref actually resolves in the
// claim's own body — never an authored field, always derived.
func TestWriteSourcesRow_CitationCountColumn(t *testing.T) {
	c := sourcedClaim(
		model.Source{Ref: 1, Kind: model.SourceKindExternal, Title: "Cited once"},
		model.Source{Ref: 2, Kind: model.SourceKindExternal, Title: "Cited twice"},
		model.Source{Ref: 3, Kind: model.SourceKindExternal, Title: "Never cited"},
	)
	c.Body = "the retry bound rests on [1]. the budget is checked twice, at [2] and again [2]."

	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	for _, want := range []string{
		`<span class="claim-source-cite">cited once</span>`,
		`<span class="claim-source-cite">cited 2 times</span>`,
		`<span class="claim-source-cite">cited 0 times</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the footer, got: %s", want, got)
		}
	}
}
