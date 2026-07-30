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
// EdgesHTMLWithLinks — comment chip + baked-in thread panel
// ---------------------------------------------------------------------

// v0.2.1 — a claim with NO comments still renders a chip, so the FIRST comment
// on it is openable from the viewer (the human's only surface). Gating chip
// emission on len(c.Comments) > 0 was the bug: a card nobody had questioned yet
// could never be questioned. The zero state is its own variant reading "💬 0",
// its <li> ships `hidden` for the static file:// case, and it drags in no baked
// panel (there are no threads to bake).
func TestEdgesHTMLWithLinks_NoComments_EmptyChipHiddenByDefault(t *testing.T) {
	c := model.Claim{ID: "widget.contract.quiet", Facet: "contract", Status: model.StatusLocked}
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if !strings.Contains(got, `<li class="claim-comments" hidden>`) {
		t.Fatalf("the zero-state chip's <li> must ship hidden (revealed only by the viewer's live-API probe), got: %s", got)
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
	// The chip is the ONLY thing the zero state adds: still byte-identical to
	// plain edgesHTML, which is the graceful-degradation guard the implink
	// footer relies on (see TestEdgesHTMLWithLinks_NilFiles_MatchesPlainEdgesHTML).
	if got != string(edgesHTML(c)) {
		t.Fatalf("a comment-free claim's edges output must match edgesHTML(c)\n got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_OpenThread_ChipAccentAndBakedPanel(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.x",
		Facet:  "contract",
		Status: model.StatusLocked,
		Comments: []model.Comment{
			openThread("c-aaaaaa", "please clarify the retry bound"),
			openThread("c-bbbbbb", "second open thread"),
			resolvedThread("c-cccccc", "already handled"),
		},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	// Chip: accent (open), carries the claim id via data-* (never id=), and
	// shows the OPEN count (2), not the total (3).
	if !strings.Contains(got, "comment-chip--open") {
		t.Fatalf("expected an open (accent) chip for a claim with open threads, got: %s", got)
	}
	if strings.Contains(got, "comment-chip--resolved") {
		t.Fatalf("a claim with open threads must not render the muted/resolved chip, got: %s", got)
	}
	if !strings.Contains(got, `data-claim-id="widget.contract.x"`) {
		t.Fatalf("chip/panel must carry the claim id via data-claim-id, got: %s", got)
	}
	if !strings.Contains(got, `<span class="comment-chip-count">2</span>`) {
		t.Fatalf("chip open-count should be 2 (two open threads), got: %s", got)
	}

	// Panel: baked in, both open thread bodies present, each thread keyed by
	// data-thread-id (never id=).
	if !strings.Contains(got, `class="comments-panel"`) {
		t.Fatalf("expected the baked-in comments panel, got: %s", got)
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
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	// A claim whose threads are all resolved still shows a chip, but muted,
	// with the total count (3) — never the accent/open variant.
	if !strings.Contains(got, "comment-chip--resolved") {
		t.Fatalf("expected a muted (resolved) chip when every thread is resolved, got: %s", got)
	}
	if strings.Contains(got, "comment-chip--open") {
		t.Fatalf("a fully-resolved claim must not render the accent/open chip, got: %s", got)
	}
	if !strings.Contains(got, `<span class="comment-chip-count">3</span>`) {
		t.Fatalf("resolved chip count should be the total (3), got: %s", got)
	}

	// Resolved threads live inside a collapsed <details> — present but not
	// force-opened (needs no id, works without JS).
	if !strings.Contains(got, `<details class="comments-resolved">`) {
		t.Fatalf("expected resolved threads inside a <details> collapse, got: %s", got)
	}
	if strings.Contains(got, "<details class=\"comments-resolved\" open") || strings.Contains(got, "<details open") {
		t.Fatalf("the resolved <details> must start collapsed (no open attribute), got: %s", got)
	}
	if !strings.Contains(got, "<summary>3 resolved</summary>") {
		t.Fatalf("expected a '<summary>3 resolved</summary>' disclosure, got: %s", got)
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
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
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
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
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
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	// The chip is a real button with a non-empty aria-label.
	if !strings.Contains(got, `<button type="button"`) {
		t.Fatalf("expected the chip to be a real <button type=\"button\">, got: %s", got)
	}
	// It is a disclosure control for the shared rail: aria-controls names the
	// panel and aria-expanded reflects (initially closed) open state.
	if !strings.Contains(got, `aria-controls="commentsPanel"`) {
		t.Fatalf("chip must reference the panel via aria-controls, got: %s", got)
	}
	if !strings.Contains(got, `aria-expanded="false"`) {
		t.Fatalf("chip must carry an aria-expanded state, got: %s", got)
	}
	labelIdx := strings.Index(got, `aria-label="`)
	if labelIdx == -1 {
		t.Fatalf("chip button must carry an aria-label, got: %s", got)
	}
	rest := got[labelIdx+len(`aria-label="`):]
	if strings.HasPrefix(rest, `"`) {
		t.Fatalf("chip aria-label must be non-empty, got: %s", got)
	}

	// Every 💬 glyph is inside an aria-hidden span.
	if strings.Count(got, "💬") != strings.Count(got, `aria-hidden="true">💬`) {
		t.Fatalf("every decorative 💬 glyph must be aria-hidden, got: %s", got)
	}
}

// ---------------------------------------------------------------------
// Per-layout footer wiring: the chip appears for every layout that renders
// the shared edges footer, and NOT for banner (banner.html has no {{edges .}}
// call, so the whole comment surface is excluded automatically).
// ---------------------------------------------------------------------

func TestCommentChip_AppearsForEveryLayoutExceptBanner(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	claim := model.Claim{
		ID:     "widget.sample.x",
		Facet:  "contract",
		Status: model.StatusDraft,
		Body:   "some prose so every layout renders something",
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
					t.Fatalf("banner must be excluded from commenting (no edges footer), but a chip appeared: %s", out)
				}
				return
			}
			if !hasChip {
				t.Fatalf("layout %q renders the edges footer and must carry a comment chip, got: %s", layout, out)
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
