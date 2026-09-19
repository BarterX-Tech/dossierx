package components

import (
	"bytes"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// openThread and resolvedThread build the two comment shapes the render tests
// below need — an unresolved thread and a resolved one — without repeating the
// full struct literal in every case.
func openThread(id, body string) model.Comment {
	return model.Comment{
		ID:      id,
		Status:  model.CommentStatusOpen,
		Author:  model.CommentRoleHuman,
		Created: "2026-07-24T10:00:00Z",
		Body:    body,
	}
}

func resolvedThread(id, body string) model.Comment {
	return model.Comment{
		ID:         id,
		Status:     model.CommentStatusResolved,
		Author:     model.CommentRoleAgent,
		Created:    "2026-07-24T09:00:00Z",
		Body:       body,
		ResolvedBy: model.CommentRoleHuman,
		ResolvedAt: "2026-07-24T11:00:00Z",
	}
}

// ---------------------------------------------------------------------
// CommentChipHTML — the 💬 chip, and EdgesHTMLWithLinks — the baked-in
// thread panel.
//
// The comment chip is emitted by CommentChipHTML and composed into the last
// slot of the shared footer by EdgesHTMLWithLinks. The thread panel remains a
// sibling after that footer: comments are available from the claim's footer,
// without adding controls or state to the claim heading.
//
// Note the trap in every assertion below: the panel has its own, unrelated
// <details class="comments-resolved"> for resolved threads. It is not the new
// footer disclosure, and a substring check on a bare "<details" cannot tell
// them apart. Match the class.
// ---------------------------------------------------------------------

// v0.3.0 — a claim with NO comments still renders a chip, so the FIRST comment
// on it is openable from the viewer (the human's only surface). Gating chip
// emission on len(c.Comments) > 0 was the bug: a card nobody had questioned yet
// could never be questioned. The zero state is its own variant reading "💬 0",
// its slot ships `hidden` for the static file:// case, and it drags in no baked
// panel (there are no threads to bake).
func TestEdgesHTMLWithLinks_NoComments_EmptyChipHiddenByDefault(t *testing.T) {
	c := model.Claim{ID: "widget.contract.quiet", Facet: "contract", Status: model.StatusLocked}
	got := string(CommentChipHTML(c))

	// The whole zero-state chip, byte for byte. The slot's attribute order is
	// class then the BARE boolean `hidden` — never hidden="" and never
	// hidden="hidden", which are the bytes shell.html's syncEmptyChips and the
	// chromedp suite read.
	const want = `<span class="claim-comments-slot" hidden>` +
		`<button type="button" class="comment-chip comment-chip--empty" data-claim-id="widget.contract.quiet" ` +
		`aria-controls="commentsPanel" aria-expanded="false" aria-label="add the first comment on this claim">` +
		`<span class="comment-chip-glyph" aria-hidden="true"><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-message-circle"/></svg></span> <span class="comment-chip-count">0</span>` +
		`</button></span>`
	if got != want {
		t.Fatalf("zero-state chip mismatch\n want: %s\n got:  %s", want, got)
	}

	if !strings.Contains(got, `<span class="claim-comments-slot" hidden>`) {
		t.Fatalf("the zero-state chip's slot must ship hidden (revealed only by the viewer's live-API probe), got: %s", got)
	}
	if !strings.Contains(got, `class="comment-chip comment-chip--empty"`) {
		t.Fatalf("a comment-free claim must render the --empty chip variant, got: %s", got)
	}
	if !strings.Contains(got, `<span class="comment-chip-count">0</span>`) {
		t.Fatalf("the zero-state chip must read 0, got: %s", got)
	}
	// "no one has commented" is not "everything raised was settled": the empty
	// chip must not borrow either of the other two variants.
	for _, absent := range []string{"comment-chip--open", "comment-chip--resolved", "comments-panel"} {
		if strings.Contains(got, absent) {
			t.Fatalf("a comment-free claim must not render %q, got: %s", absent, got)
		}
	}
	// The aria-label invites the action rather than describing a count of zero.
	if !strings.Contains(got, `aria-label="add the first comment on this claim"`) {
		t.Fatalf("the zero-state chip must invite the first comment in its aria-label, got: %s", got)
	}

	// The footer owns the chip position, including the quiet zero state. The
	// standalone component remains the exact fragment placed in that final
	// slot, and the plain and link-aware renderers remain byte-identical when
	// there is no comments panel to append.
	footer := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if !strings.HasPrefix(footer, `<div class="claim-footer">`) || !strings.Contains(footer, got) {
		t.Fatalf("the quiet chip must occupy the footer slot, got: %s", footer)
	}
	if footer != string(edgesHTML(c)) {
		t.Fatalf("a comment-free claim's edges output must match edgesHTML(c)\n got: %s", footer)
	}
}

func TestEdgesHTMLWithLinks_OpenThread_ChipAccentAndBakedPanel(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.x",
		Module: "widget",
		Facet:  "contract",
		Status: model.StatusLocked,
		// One real edge gives the footer a populated relationship door while the
		// comments chip still remains in its fixed final slot.
		RestsOn: []string{"widget.contract.dep"},
		Comments: []model.Comment{
			openThread("c-aaaaaa", "please clarify the retry bound"),
			openThread("c-bbbbbb", "second open thread"),
			resolvedThread("c-cccccc", "already handled"),
		},
	}
	chip := string(CommentChipHTML(c))

	// Chip: accent (open), carries the claim id via
	// data-* (never id=), and shows the OPEN count (2), not the total (3).
	if !strings.HasPrefix(chip, `<span class="claim-comments-slot">`) {
		t.Fatalf("a chip with threads must sit in a slot carrying no hidden attribute, got: %s", chip)
	}
	// Careful: the glyph span legitimately carries aria-hidden, so a bare
	// "hidden" substring check would always match. It is the SLOT's boolean
	// attribute that must be absent.
	if strings.Contains(chip, `claim-comments-slot" hidden`) {
		t.Fatalf("a chip with threads must not be hidden, got: %s", chip)
	}
	if !strings.Contains(chip, "comment-chip--open") {
		t.Fatalf("expected an open (accent) chip for a claim with open threads, got: %s", chip)
	}
	if strings.Contains(chip, "comment-chip--resolved") {
		t.Fatalf("a claim with open threads must not render the muted/resolved chip, got: %s", chip)
	}
	if !strings.Contains(chip, `data-claim-id="widget.contract.x"`) {
		t.Fatalf("the chip must carry the claim id via data-claim-id, got: %s", chip)
	}
	if !strings.Contains(chip, `<span class="comment-chip-count">2</span>`) {
		t.Fatalf("chip open-count should be 2 (two open threads), got: %s", chip)
	}

	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	// Panel: baked in, both open thread bodies present, each thread keyed by
	// data-thread-id (never id=), and carrying the claim id itself.
	if !strings.Contains(got, `class="comments-panel"`) {
		t.Fatalf("expected the baked-in comments panel, got: %s", got)
	}
	if !strings.Contains(got, `data-claim-id="widget.contract.x"`) {
		t.Fatalf("the panel must carry the claim id via data-claim-id, got: %s", got)
	}
	for _, want := range []string{
		`data-thread-id="c-aaaaaa"`,
		`data-thread-id="c-bbbbbb"`,
		"please clarify the retry bound",
		"second open thread",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("panel missing %q, got: %s", want, got)
		}
	}

	// The panel is a sibling of the whole footer, immediately after the comment
	// chip's slot. It must not be nested in any relationship or source door.
	if !strings.Contains(got, `</button></span></div><div class="comments-panel"`) {
		t.Fatalf("the panel must follow the closed .claim-footer strip as a sibling with no whitespace between, got: %s", got)
	}

	footerStart := strings.Index(got, `<div class="claim-footer">`)
	panelStart := strings.Index(got, `<div class="comments-panel"`)
	chipStart := strings.Index(got, "comment-chip")
	if footerStart < 0 || panelStart < 0 || chipStart < footerStart || chipStart > panelStart {
		t.Fatalf("the chip must live in the footer before its panel, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_ResolvedOnly_ChipMutedAndDetailsCollapsed(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.r",
		Facet:  "contract",
		Status: model.StatusLocked,
		Comments: []model.Comment{
			resolvedThread("c-d1d1d1", "first resolved"),
			resolvedThread("c-d2d2d2", "second resolved"),
			resolvedThread("c-d3d3d3", "third resolved"),
		},
	}
	chip := string(CommentChipHTML(c))

	// A claim whose threads are all resolved still shows a chip, but muted,
	// with the total count (3) — never the accent/open variant.
	if !strings.Contains(chip, "comment-chip--resolved") {
		t.Fatalf("expected a muted (resolved) chip when every thread is resolved, got: %s", chip)
	}
	if strings.Contains(chip, "comment-chip--open") {
		t.Fatalf("a fully-resolved claim must not render the accent/open chip, got: %s", chip)
	}
	if !strings.Contains(chip, `<span class="comment-chip-count">3</span>`) {
		t.Fatalf("resolved chip count should be the total (3), got: %s", chip)
	}
	if !strings.HasPrefix(chip, `<span class="claim-comments-slot">`) {
		t.Fatalf("a chip with threads must sit in a slot carrying no hidden attribute, got: %s", chip)
	}

	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	// Resolved threads live inside the PANEL's own collapsed <details
	// class="comments-resolved"> — a pre-existing element unrelated to the new
	// <details class="claim-links"> footer wrapper. It is present but not
	// force-opened (needs no id, works without JS).
	if !strings.Contains(got, `<details class="comments-resolved">`) {
		t.Fatalf("expected resolved threads inside a <details> collapse, got: %s", got)
	}
	if strings.Contains(got, `<details class="comments-resolved" open`) {
		t.Fatalf("the resolved <details> must start collapsed (no open attribute), got: %s", got)
	}
	// Re-pinned per 14 §4.4 (457-0) + 14 §8.4 + R-J.6 (verifier RETRY item 1):
	// the baked read-only summary now carries the same chevron svg the live
	// path (viewer-runtime.js buildResolvedChevron) builds, since style.css
	// hides the native <summary> marker on both paths.
	if !strings.Contains(got, `<summary><svg class="comments-resolved-chevron" viewBox="0 0 24 24" width="13" height="13" fill="none" aria-hidden="true"><path d="m9 18 6-6-6-6"></path></svg><span>3 resolved</span></summary>`) {
		t.Fatalf("expected a chevron + '3 resolved' disclosure, got: %s", got)
	}
	// The Paper zero state is explicit but inert: no empty disclosure door and
	// no numeral pretending a relationship exists.
	if strings.Contains(got, `<details class="claim-links"`) {
		t.Fatalf("an edgeless claim must not render a relationships disclosure, got: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-footer-chip-label">No relationships</span>`) {
		t.Fatalf("expected the plain zero-relationships label, got: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-footer-chip-label">No sources</span>`) {
		t.Fatalf("expected the zero-sources chip worded \"No sources\", got: %s", got)
	}
}

// TestEdgesHTMLWithLinks_PanelSurvivesFooterSuppression is the interaction
// between the strip and a claim's baked comment panel: whatever the strip
// renders, the panel is never nested inside it and never depends on it.
//
// An edgeless claim still retains its footer for its stable zero labels and
// comment entry point. The panel is a sibling after that strip, never gated by
// a populated relationship or source disclosure.
func TestEdgesHTMLWithLinks_PanelSurvivesFooterSuppression(t *testing.T) {
	c := model.Claim{
		ID:       "widget.contract.overview",
		Facet:    "contract",
		Status:   model.StatusDraft,
		Comments: []model.Comment{openThread("c-aaaaaa", "still needs an answer")},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if !strings.HasPrefix(got, `<div class="claim-footer">`) {
		t.Fatalf("an edgeless claim must still open the footer strip at zero, got: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-footer-chip-label">No relationships</span>`) {
		t.Fatalf("expected the zero-relationships label, got: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-footer-chip-label">No sources</span>`) {
		t.Fatalf("expected the zero-sources chip worded \"No sources\", got: %s", got)
	}
	if !strings.Contains(got, `</div><div class="comments-panel" data-claim-id="widget.contract.overview" hidden>`) {
		t.Fatalf("the panel must follow the closed strip immediately as a sibling, got: %s", got)
	}
	if !strings.Contains(got, "still needs an answer") {
		t.Fatalf("the panel's thread body must survive, got: %s", got)
	}
}

// The comment markup must contribute ZERO id= attributes anywhere: an overview
// claim renders N times and only one copy may keep its id (see
// render.stripOverviewIDs), so the chip/panel identify claims and threads via
// data-claim-id / data-thread-id only.
func TestEdgesHTMLWithLinks_CommentMarkupHasNoIDAttributes(t *testing.T) {
	c := model.Claim{
		ID:     "widget.overview.router",
		Facet:  "overview",
		Status: model.StatusDraft,
		Comments: []model.Comment{
			openThread("c-aaaaaa", "open one"),
			resolvedThread("c-bbbbbb", "resolved one"),
		},
	}
	got := string(CommentChipHTML(c)) + string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if n := strings.Count(got, ` id="`); n != 0 {
		t.Fatalf("comment markup must emit zero id= attributes, found %d in: %s", n, got)
	}
}

// Comment bodies are server-rendered through the shared markdown renderer and
// therefore HTML-escaped: a hostile body renders inert (escaped), never as live
// markup.
func TestEdgesHTMLWithLinks_BodyMarkdownRenderedAndEscaped(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.x",
		Facet:  "contract",
		Status: model.StatusDraft,
		Comments: []model.Comment{
			openThread("c-aaaaaa", "look: <img src=x onerror=alert(1)> and <script>alert(2)</script>"),
		},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if !strings.Contains(got, "&lt;img") {
		t.Fatalf("expected the <img> body escaped to &lt;img, got: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("expected the <script> body escaped to &lt;script&gt;, got: %s", got)
	}
	if strings.Contains(got, "<img ") || strings.Contains(got, "<script>") {
		t.Fatalf("hostile comment body leaked live markup into the render, got: %s", got)
	}
}

// The static render is read-only by design: it bakes in the threads for the
// viewer JS (Phase 5) to enhance, but emits no composer (textarea/form) markup
// of its own — a composer with nowhere to POST would be a dead control.
func TestEdgesHTMLWithLinks_NoComposerMarkup(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.x",
		Facet:  "contract",
		Status: model.StatusDraft,
		Comments: []model.Comment{
			openThread("c-aaaaaa", "open one"),
			resolvedThread("c-bbbbbb", "resolved one"),
		},
	}
	got := string(CommentChipHTML(c)) + string(EdgesHTMLWithLinks(c, nil, nil, nil))
	for _, forbidden := range []string{"<textarea", "<form", "comment-composer", `type="submit"`} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("static comment render must contain no composer markup, found %q in: %s", forbidden, got)
		}
	}
}

// Every comment control carries a non-empty accessible name, and the purely
// decorative 💬 glyph is aria-hidden so a screen reader never announces it as
// content.
func TestEdgesHTMLWithLinks_AccessibleControls(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.x",
		Facet:  "contract",
		Status: model.StatusDraft,
		Comments: []model.Comment{
			openThread("c-aaaaaa", "open one"),
			resolvedThread("c-bbbbbb", "resolved one"),
		},
	}
	chip := string(CommentChipHTML(c))

	// The chip is a real button with a non-empty aria-label.
	if !strings.Contains(chip, `<button type="button"`) {
		t.Fatalf("expected the chip to be a real <button type=\"button\">, got: %s", chip)
	}
	// It is a disclosure control for the shared rail: aria-controls names the
	// panel and aria-expanded reflects (initially closed) open state. Moving
	// the chip out of the footer and into the claim head changed none of this.
	if !strings.Contains(chip, `aria-controls="commentsPanel"`) {
		t.Fatalf("chip must reference the panel via aria-controls, got: %s", chip)
	}
	if !strings.Contains(chip, `aria-expanded="false"`) {
		t.Fatalf("chip must carry an aria-expanded state, got: %s", chip)
	}
	labelIdx := strings.Index(chip, `aria-label="`)
	if labelIdx == -1 {
		t.Fatalf("chip button must carry an aria-label, got: %s", chip)
	}
	rest := chip[labelIdx+len(`aria-label="`):]
	if strings.HasPrefix(rest, `"`) {
		t.Fatalf("chip aria-label must be non-empty, got: %s", chip)
	}

	got := chip + string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if !strings.Contains(chip, `href="#dx-icon-message-circle"`) {
		t.Fatalf("chip must use the Lucide message icon, got: %s", chip)
	}
	if strings.Count(got, `href="#dx-icon-message-circle"`) != strings.Count(got, `aria-hidden="true"><svg class="dx-icon"`) &&
		strings.Count(got, "#dx-icon-message-circle") < 1 {
		t.Fatalf("every decorative comment icon must be aria-hidden, got: %s", got)
	}
}

// ---------------------------------------------------------------------
// Per-layout chip wiring: the chip appears in the footer of every non-banner
// layout. Banner remains deliberately excluded from the comment surface.
// ---------------------------------------------------------------------

func TestCommentChip_AppearsForEveryLayoutExceptBanner(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	claim := model.Claim{
		ID:     "widget.sample.x",
		Module: "widget",
		Facet:  "contract",
		Status: model.StatusDraft,
		Body:   "some prose so every layout renders something",
		// One edge keeps a populated footer door in the fixture; the chip stays
		// in its final footer slot independently of the door.
		RestsOn: []string{"widget.contract.dep"},
		Comments: []model.Comment{
			openThread("c-aaaaaa", "an open thread"),
		},
	}

	for layout, tmpl := range partials {
		t.Run(string(layout), func(t *testing.T) {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, claim); err != nil {
				t.Fatalf("execute %s partial: %v", layout, err)
			}
			out := buf.String()
			hasChip := strings.Contains(out, "comment-chip")
			if layout == model.LayoutBanner {
				if hasChip {
					t.Fatalf("banner must be excluded from commenting, but a chip appeared: %s", out)
				}
				if strings.Contains(out, "claim-comments-slot") {
					t.Fatalf("banner must carry no chip slot: %s", out)
				}
				return
			}
			if !hasChip {
				t.Fatalf("layout %q must carry a comment chip, got: %s", layout, out)
			}

			// The chip occupies the final footer slot, after the relationship and
			// source controls and before any comments panel.
			if !strings.Contains(out, `<span class="claim-comments-slot">`) {
				t.Fatalf("layout %q must wrap its chip in the head's slot span, got: %s", layout, out)
			}
			chipIdx := strings.Index(out, "comment-chip")
			footerIdx := strings.Index(out, `<div class="claim-footer">`)
			panelIdx := strings.Index(out, `<div class="comments-panel"`)
			if footerIdx < 0 || panelIdx < 0 {
				t.Fatalf("layout %q should have rendered a footer strip for a claim with a rests_on edge, got: %s", layout, out)
			}
			if chipIdx < footerIdx || chipIdx > panelIdx {
				t.Fatalf("layout %q must render its chip in the footer before the comments panel: %s", layout, out)
			}
			// And nothing re-introduced the old <li> form.
			if strings.Contains(out, `<li class="claim-comments"`) {
				t.Fatalf("layout %q still emits the retired <li class=\"claim-comments\"> chip: %s", layout, out)
			}
		})
	}
}

// ---------------------------------------------------------------------
// claim-card--commented root modifier: only on cards with an OPEN thread,
// including the tree variant (which lacks the .card class, hence the dedicated
// .claim-tree.claim-card--commented CSS rule).
// ---------------------------------------------------------------------

func TestClaimCardCommented_OnlyOnOpenThreadCards(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	base := model.Claim{ID: "widget.contract.x", Facet: "contract", Status: model.StatusDraft, Body: "prose"}

	openC := base
	openC.Comments = []model.Comment{openThread("c-aaaaaa", "open")}

	resolvedC := base
	resolvedC.Comments = []model.Comment{resolvedThread("c-bbbbbb", "resolved")}

	none := base

	for _, layout := range []model.Layout{model.LayoutCard, model.LayoutTree, model.LayoutTable} {
		t.Run(string(layout), func(t *testing.T) {
			exec := func(c model.Claim) string {
				var buf bytes.Buffer
				if err := partials[layout].Execute(&buf, c); err != nil {
					t.Fatalf("execute %s partial: %v", layout, err)
				}
				return buf.String()
			}
			if got := exec(openC); !strings.Contains(got, "claim-card--commented") {
				t.Fatalf("%s with an open thread must carry claim-card--commented, got: %s", layout, got)
			}
			if got := exec(resolvedC); strings.Contains(got, "claim-card--commented") {
				t.Fatalf("%s with only resolved threads must NOT carry claim-card--commented, got: %s", layout, got)
			}
			if got := exec(none); strings.Contains(got, "claim-card--commented") {
				t.Fatalf("%s with no comments must NOT carry claim-card--commented, got: %s", layout, got)
			}
		})
	}

	// The tree variant specifically: the modifier lands on the same root node
	// that carries claim-tree (which has no .card), so the dedicated
	// .claim-tree.claim-card--commented CSS rule can reach it.
	var buf bytes.Buffer
	if err := partials[model.LayoutTree].Execute(&buf, openC); err != nil {
		t.Fatalf("execute tree partial: %v", err)
	}
	if !strings.Contains(buf.String(), `class="claim claim-tree claim-card--commented"`) {
		t.Fatalf("tree root must be class=\"claim claim-tree claim-card--commented\", got: %s", buf.String())
	}
}
