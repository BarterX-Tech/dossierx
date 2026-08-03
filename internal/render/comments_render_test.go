package render

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// commentedClaim builds a grouped claim carrying the given comments, valid
// enough for catalog.Build + a full Render pass.
func commentedClaim(id, module, facet string, layout model.Layout, comments []model.Comment) model.Claim {
	return model.Claim{
		ID:       id,
		Module:   module,
		Facet:    facet,
		Status:   model.StatusDraft,
		Layout:   layout,
		Body:     "orientation prose",
		Comments: comments,
		Governed: model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
	}
}

func openComment(id, body string) model.Comment {
	return model.Comment{ID: id, Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-24T10:00:00Z", Body: body}
}

// TestRender_CommentedOverviewChipFansOutToEveryFacet is the overview
// N-copies case: an overview/orientation claim is injected into every facet
// group of its module, so its chip + baked panel must appear once per facet
// (here 2), all keyed by data-claim-id — while stripOverviewIDs still keeps
// exactly ONE id-bearing canonical copy (comments add no new id= anywhere).
func TestRender_CommentedOverviewChipFansOutToEveryFacet(t *testing.T) {
	overview := commentedClaim(
		"widget.overview.router", "widget", "overview", model.LayoutCard,
		[]model.Comment{openComment("c-aaaaaa", "ORIENTATION-THREAD-BODY")},
	)
	claims := []model.Claim{
		overview,
		commentedClaim("widget.contract.a", "widget", "contract", model.LayoutCard, nil),
		commentedClaim("widget.internals.b", "widget", "internals", model.LayoutCard, nil),
	}
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract", "internals"}}
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// The overview note is injected into both facet tabs, so its open chip and
	// its baked thread body both appear twice. Match the full class attribute,
	// not the bare token, so the .comment-chip--open rule embedded in the page's
	// <style> block isn't miscounted as a third chip.
	if got := strings.Count(out, `class="comment-chip comment-chip--open"`); got != 2 {
		t.Fatalf("overview open chip appears %d times, want 2 (one per facet):\n%s", got, out)
	}
	if got := strings.Count(out, "ORIENTATION-THREAD-BODY"); got != 2 {
		t.Fatalf("overview baked thread body appears %d times, want 2 (one per facet):\n%s", got, out)
	}
	if got := strings.Count(out, `class="comments-panel"`); got != 2 {
		t.Fatalf("overview baked panel appears %d times, want 2 (one per facet):\n%s", got, out)
	}
	// The .k heading, the chip <button> and the baked panel each carry
	// data-claim-id, so an overview claim in two facets yields 2 copies * 3 = 6
	// — every one a data-* hook, never a duplicate id=. Re-derived against
	// v0.4.1's markup rather than carried over: the chip moved out of the edges
	// footer and into the .k heading, but it moved as a whole <button> inside a
	// new <span class="claim-comments-slot"> — the slot span carries no
	// data-claim-id of its own — so the per-copy count is still heading + chip +
	// panel = 3, not 2 and not 4.
	// The heading's copy is the point of that attribute: its visible text is
	// now the derived label ("Router"), so data-claim-id is what keeps the id
	// "dossierx claim lock <id>" needs greppable in the rendered document.
	if got := strings.Count(out, `data-claim-id="widget.overview.router"`); got != 6 {
		t.Fatalf("overview data-claim-id appears %d times, want 6 (heading+chip+panel per facet):\n%s", got, out)
	}

	// Comments must not perturb the canonical-id-appears-once invariant
	// (DX-AUD-16): the chip/panel use data-claim-id, never id=.
	if got := strings.Count(out, ` id="widget.overview.router"`); got != 1 {
		t.Fatalf("overview canonical id appears %d times, want exactly 1 (comments add no id=):\n%s", got, out)
	}
}

// v0.2.1 — the chip must reach EVERY non-banner claim through the full Render
// pipeline, not only ones that already have threads, or the first comment on a
// quiet card is unreachable from the viewer. This is the render-level companion
// to components.TestEdgesHTMLWithLinks_NoComments_EmptyChipHiddenByDefault: it
// pins that nothing between the footer and the finished document (facet
// grouping, overview injection, the shell) drops the zero-state chip, and that
// its slot arrives `hidden` — the static file:// export has no comment API and
// therefore no composer, so shell.html's probe is what reveals these.
//
// v0.4.1 moved that slot: the chip used to be an <li class="claim-comments">
// inside the edges footer's <ul>, and is now a <span class="claim-comments-slot">
// in the claim's .k heading, so the footer can collapse without taking the chip
// with it. The `hidden` attribute rides the new span.
//
// Counting note, load-bearing: `claim-comments` is a strict PREFIX of
// `claim-comments-slot`, so a leftover Count on the old class name would keep
// matching the new markup and read as a pass forever. Every assertion below
// counts the full opening tag `<span class="claim-comments-slot"` instead —
// which also cannot collide with the selector text in the shell's inline
// <style>/<script>, the way a bare-token scan would.
func TestRender_EmptyChipOnQuietClaimHiddenInStaticRender(t *testing.T) {
	claims := []model.Claim{
		commentedClaim("widget.contract.loud", "widget", "contract", model.LayoutCard,
			[]model.Comment{openComment("c-aaaaaa", "a real thread")}),
		commentedClaim("widget.contract.quiet", "widget", "contract", model.LayoutCard, nil),
		commentedClaim("widget.contract.notice", "widget", "contract", model.LayoutBanner, nil),
	}
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// The quiet claim carries an --empty chip keyed to itself...
	if !strings.Contains(out, `class="comment-chip comment-chip--empty" data-claim-id="widget.contract.quiet"`) {
		t.Fatalf("a claim with zero threads must still carry an --empty chip:\n%s", out)
	}
	// ...inside a hidden slot span. Exactly one slot is hidden: the quiet
	// claim's. The commented claim's chip is visible from the server render,
	// since a static export can still SHOW an existing discussion — only ADDING
	// one needs the API.
	if got := strings.Count(out, `<span class="claim-comments-slot" hidden>`); got != 1 {
		t.Fatalf("hidden zero-state slot appears %d times, want exactly 1 (the quiet claim's):\n%s", got, out)
	}
	if !strings.Contains(out, `<span class="claim-comments-slot"><button type="button" class="comment-chip comment-chip--open" data-claim-id="widget.contract.loud"`) {
		t.Fatalf("a claim with an open thread must keep its VISIBLE (not hidden) chip:\n%s", out)
	}
	// The banner layout never calls {{edges .}} OR {{commentChip .}}, so the
	// whole comment surface — including the new zero state — stays off it. That
	// used to be free: one omitted call kept both the panel and the chip away,
	// because the chip rode the footer. Since v0.4.1 the chip is its own
	// template func called from the .k heading, so banner stays chip-free only
	// because banner.html deliberately keeps the flat heading and does not call
	// it (C3/C6) — which is exactly why this stays asserted here.
	//
	// Asserted against the comment surface's OWN markup rather than a bare
	// data-claim-id scan: since the claim-edge label work every partial's .k
	// heading carries data-claim-id, banner.html included (it is the one piece
	// of that work that does reach banner — the heading label needs the id
	// reachable, and that has nothing to do with comments). So the invariant is
	// "no chip and no panel for a banner claim", which is what this checks.
	for _, forbidden := range []string{
		`class="comment-chip comment-chip--empty" data-claim-id="widget.contract.notice"`,
		`class="comments-panel" data-claim-id="widget.contract.notice"`,
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("banner claims must be excluded from the comment surface entirely, found %q:\n%s", forbidden, out)
		}
	}
	// Two slots total, one per non-banner claim: the hidden one counted above
	// plus the loud claim's visible one. Counting the opening tag up to its
	// closing quote catches both the ` hidden>` and the bare `>` forms in a
	// single literal, and matches nothing in the shell's <style>/<script>.
	if got := strings.Count(out, `<span class="claim-comments-slot"`); got != 2 {
		t.Fatalf("expected exactly 2 comment slots (the two non-banner claims), got %d:\n%s", got, out)
	}
	// The old <li class="claim-comments"> shape must be gone from the document
	// entirely — asserted on the full opening tag, since the bare class token
	// still substring-matches its `claim-comments-slot` successor.
	if strings.Contains(out, `<li class="claim-comments"`) {
		t.Fatalf("the pre-v0.4.1 chip <li> must not survive anywhere in the render:\n%s", out)
	}
	// A zero-thread claim bakes in no panel: there are no threads to bake.
	if got := strings.Count(out, `class="comments-panel"`); got != 1 {
		t.Fatalf("baked panel appears %d times, want 1 (only the commented claim):\n%s", got, out)
	}
}

// A tree claim with an open thread must carry claim-card--commented on the same
// root node that carries claim-tree (tree has no .card class), so the dedicated
// .claim-tree.claim-card--commented rule can reach it — verified through the
// full Render pipeline, not just the isolated partial.
func TestRender_TreeCommentedRootModifier(t *testing.T) {
	claim := commentedClaim(
		"widget.internals.tree", "widget", "internals", model.LayoutTree,
		[]model.Comment{openComment("c-aaaaaa", "open thread on a tree claim")},
	)
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"internals"}}
	cat, err := catalog.Build([]model.Claim{claim}, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `class="claim claim-tree claim-card--commented"`) {
		t.Fatalf("tree claim with an open thread must render class=\"claim claim-tree claim-card--commented\":\n%s", out)
	}
}

// The whole rendered document must contain no composer markup — the static
// render is a read-only snapshot; interactive controls mount client-side only
// when served (Phase 5).
func TestRender_NoComposerInStaticDocument(t *testing.T) {
	claim := commentedClaim(
		"widget.contract.x", "widget", "contract", model.LayoutCard,
		[]model.Comment{openComment("c-aaaaaa", "an open thread")},
	)
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}
	cat, err := catalog.Build([]model.Claim{claim}, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Sanity: the chip + panel really are present (so the negative assertions
	// below aren't vacuously true). Match the rendered class attributes, not the
	// bare tokens, which also appear as selectors in the page's <style> block.
	if !strings.Contains(out, `class="comment-chip`) || !strings.Contains(out, `class="comments-panel"`) {
		t.Fatalf("expected the chip + baked panel in the rendered document:\n%s", out)
	}
	// No composer MARKUP may be baked into a claim's static HTML. The checks are
	// attribute-precise on purpose: since Phase 5 the shell's inline <style> and
	// <script> legitimately DEFINE the client-side composer (its CSS selectors
	// like ".comment-composer" and JS class strings like 'comment-composer'), so
	// a bare-token scan would false-positive on the runtime the test is meant to
	// tolerate. A composer rendered as real DOM by comments.html would instead
	// emit a `class="comment-composer"` attribute or a literal `<textarea` tag —
	// exactly what these forbid — so the read-only-snapshot guarantee still holds.
	for _, forbidden := range []string{"<textarea", `class="comment-composer`, `class="comment-reply-composer`} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("static document must contain no composer markup, found %q:\n%s", forbidden, out)
		}
	}
}
