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
	// Both the chip and the panel carry data-claim-id, so an overview claim in
	// two facets yields 2 copies * 2 = 4 — every one a data-* hook the viewer
	// JS fans state out over, never a duplicate id=.
	if got := strings.Count(out, `data-claim-id="widget.overview.router"`); got != 4 {
		t.Fatalf("overview data-claim-id appears %d times, want 4 (chip+panel per facet):\n%s", got, out)
	}

	// Comments must not perturb the canonical-id-appears-once invariant
	// (DX-AUD-16): the chip/panel use data-claim-id, never id=.
	if got := strings.Count(out, ` id="widget.overview.router"`); got != 1 {
		t.Fatalf("overview canonical id appears %d times, want exactly 1 (comments add no id=):\n%s", got, out)
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
