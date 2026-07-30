package components

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// images_test.go is where the image capability's BOUNDARY is tested, as opposed
// to its behaviour (which internal/render/markdown's own tests carry).
//
// This package is where the boundary actually lives: it holds the shared
// funcMap, the comments.html panel and the layout:table cell helper, and those
// are three of the four surfaces gate 0's per-surface permission table talks
// about. The test that matters most is the pairing one — the SAME markdown, on
// the same claim, rendering an image in the body and literal text in a comment.

const imageBody = "![Retry state machine](assets/retry.svg)"

// claimWithImage builds a claim carrying imageBody on every image-permitting
// surface plus a comment thread and a table cell carrying the identical bytes.
func claimWithImage() model.Claim {
	return model.Claim{
		ID:     "widget.contract.retry-policy",
		Facet:  "contract",
		Module: "widget",
		Status: model.StatusDraft,
		Body:   imageBody,
		Steps:  []string{imageBody},
		Rows:   []model.Row{{"step": imageBody}},
		Comments: []model.Comment{{
			ID:      "c-aaaaaa",
			Status:  model.CommentStatusOpen,
			Author:  model.CommentRoleHuman,
			Created: "2026-07-29T10:00:00Z",
			Body:    imageBody,
			Replies: []model.Reply{{
				ID:      "r-bbbbbb",
				Author:  model.CommentRoleAgent,
				Created: "2026-07-29T11:00:00Z",
				Body:    imageBody,
			}},
		}},
	}
}

// TestClaimBody_RendersAnImage is the opt-in half, through the real partials.
func TestClaimBody_RendersAnImage(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	claim := claimWithImage()
	want := `<img class="md-img" src="/claim-assets/widget.contract.retry-policy/assets/retry.svg" alt="Retry state machine">`

	// Every layout whose partial renders Body through the claim-body entry
	// point. tree has no Body rendering at all, so it is not in the list.
	for _, layout := range []model.Layout{
		model.LayoutCard, model.LayoutTable, model.LayoutList,
		model.LayoutSteps, model.LayoutBanner, model.LayoutMockup,
	} {
		t.Run(string(layout), func(t *testing.T) {
			var buf bytes.Buffer
			if err := partials[layout].Execute(&buf, claim); err != nil {
				t.Fatalf("execute %s: %v", layout, err)
			}
			if !strings.Contains(buf.String(), want) {
				t.Errorf("%s: no claim-body image in:\n%s", layout, buf.String())
			}
		})
	}
}

// TestSteps_RenderAnImage pins the second image-permitting surface. Steps are a
// claim's own prose in the same sense Body is (amendment A3: "claim Body/Steps
// — yes"), and steps.html renders them through a separate template call, so it
// is a separate opportunity to have forgotten.
func TestSteps_RenderAnImage(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var buf bytes.Buffer
	if err := partials[model.LayoutSteps].Execute(&buf, claimWithImage()); err != nil {
		t.Fatalf("execute steps: %v", err)
	}
	// Two images: one from Body, one from the single step.
	if n := strings.Count(buf.String(), "<img"); n != 2 {
		t.Errorf("want an image from Body and one from the step, got %d:\n%s", n, buf.String())
	}
}

// TestCommentBody_RendersNoImage is the fail-closed half and the reason the
// whole design is shaped the way it is: the SAME bytes, on the SAME claim,
// through the comment panel.
//
// The panel is reached through EdgesHTMLWithLinks (the shared "edges" func), so
// this exercises the real path a reviewer's comment takes into the static
// render, not a direct call to comments.html.
func TestCommentBody_RendersNoImage(t *testing.T) {
	out := string(EdgesHTMLWithLinks(claimWithImage(), nil, nil, nil))
	if strings.Contains(out, "<img") {
		t.Fatalf("a comment body rendered an image:\n%s", out)
	}
	// Both the thread root and the reply carry the same bytes, and both must
	// come out as the author's own escaped text.
	if n := strings.Count(out, "![Retry state machine](assets/retry.svg)"); n != 2 {
		t.Errorf("want the thread root and its reply as literal text, got %d occurrences:\n%s", n, out)
	}
	if strings.Contains(out, `<a href="assets/retry.svg"`) {
		t.Errorf("image syntax must not degrade to an anchor either:\n%s", out)
	}
}

// TestTableCell_RendersNoImage pins the third surface from the permission table:
// a layout:table claim's Rows[].cell goes through the "cell" helper, which is
// markdown.RenderInline, which has no image capability and cannot be given one.
func TestTableCell_RendersNoImage(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var buf bytes.Buffer
	if err := partials[model.LayoutTable].Execute(&buf, claimWithImage()); err != nil {
		t.Fatalf("execute table: %v", err)
	}
	out := buf.String()
	// Exactly one image: the Body's. The row cell carries identical bytes and
	// must have produced none.
	if n := strings.Count(out, "<img"); n != 1 {
		t.Errorf("want exactly the Body's image, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "<td>![Retry state machine](assets/retry.svg)</td>") {
		t.Errorf("the row cell must be escaped literal text:\n%s", out)
	}
}

// TestSharedMarkdownFunc_StaysImageFree pins that the funcMap's "markdown"
// binding — the one an arbitrary project override partial reaches, and the one
// comments.html is parsed with — is still the no-images entry point.
//
// This is the guard on the fail-closed direction. A project that ships its own
// card.html written against the documented "markdown" func loses a diagram; it
// does not gain an unreviewed capability, and neither does a future engine
// template that reaches for the obvious name.
func TestSharedMarkdownFunc_StaysImageFree(t *testing.T) {
	dir := t.TempDir()
	override := `<section id="{{.ID}}"><div class="claim-body">{{markdown .Body}}</div></section>`
	if err := os.WriteFile(filepath.Join(dir, "card.html"), []byte(override), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	partials, err := Load(dir)
	if err != nil {
		t.Fatalf("Load(%q): %v", dir, err)
	}
	var buf bytes.Buffer
	if err := partials[model.LayoutCard].Execute(&buf, claimWithImage()); err != nil {
		t.Fatalf("execute override card: %v", err)
	}
	if strings.Contains(buf.String(), "<img") {
		t.Fatalf(`the shared "markdown" func must render no image:\n%s`, buf.String())
	}
}

// --- the asset route prefix ------------------------------------------------

// TestClaimAssetURLPrefix_ShapeAndRefusals pins the one value this package
// contributes to the image path. internal/serve builds its allowlist keys from
// the same function, so a claim this refuses is a claim whose images neither
// render nor become servable — the two halves cannot disagree.
func TestClaimAssetURLPrefix_ShapeAndRefusals(t *testing.T) {
	ok := []struct{ id, want string }{
		{"widget.contract.retry-policy", "/claim-assets/widget.contract.retry-policy/"},
		{"a.b.c", "/claim-assets/a.b.c/"},
		{"Mod_1.Facet-2.slug3", "/claim-assets/Mod_1.Facet-2.slug3/"},
	}
	for _, c := range ok {
		got, usable := ClaimAssetURLPrefix(model.Claim{ID: c.id})
		if !usable || string(got) != c.want {
			t.Errorf("ClaimAssetURLPrefix(%q) = %q, %v; want %q, true", c.id, got, usable, c.want)
		}
	}

	// serve does not lint, so an id is untrusted bytes here. Anything that
	// cannot be one URL path segment loses the capability rather than
	// acquiring an ambiguous one.
	for _, id := range []string{
		"", "a/b.c.d", "a b.c.d", "../escape", "a%2Fb", "a?b", "a#b",
		"a.b.c/", ".hidden.a.b", `a\b.c.d`,
	} {
		if got, usable := ClaimAssetURLPrefix(model.Claim{ID: id}); usable {
			t.Errorf("ClaimAssetURLPrefix(%q) = %q, true; want unusable", id, got)
		}
	}
}

// TestUnroutableClaimID_RendersNoImage closes the loop: a claim whose id cannot
// be routed renders its images as literal text, which is the same degradation a
// refused src gets and the same one a comment body gets.
func TestUnroutableClaimID_RendersNoImage(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	claim := claimWithImage()
	claim.ID = "widget/contract/retry" // not one path segment
	var buf bytes.Buffer
	if err := partials[model.LayoutCard].Execute(&buf, claim); err != nil {
		t.Fatalf("execute card: %v", err)
	}
	if strings.Contains(buf.String(), "<img") {
		t.Fatalf("an unroutable claim id must render no image:\n%s", buf.String())
	}
}

// TestAssetRoutePrefix_IsOneFixedSegment pins the constant internal/serve mounts
// its route at. It is exported from here rather than duplicated there so the
// emitter and the server cannot disagree about it.
func TestAssetRoutePrefix_IsOneFixedSegment(t *testing.T) {
	if AssetRoutePrefix != "/claim-assets/" {
		t.Errorf("AssetRoutePrefix = %q", AssetRoutePrefix)
	}
}

// --- which surfaces a partial actually renders ------------------------------

// TestClaimImageSurfaces_MatchesWhatThePartialsRender is the test that keeps
// internal/serve's allowlist honest, and it is written so it cannot go stale:
// the expected answer is not a list somebody typed, it is DERIVED BY EXECUTING
// each real partial with a distinguishable image on every field and reading back
// which ones came out as an <img>.
//
// If a future edit teaches tree.html to render markdown, or teaches card.html to
// range over .Steps, this fails until ClaimImageSurfaces is updated to say so —
// which is exactly when internal/serve's index would otherwise start refusing an
// image the page emits.
func TestClaimImageSurfaces_MatchesWhatThePartialsRender(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	const (
		bodySrc  = "assets/from-body.png"
		step1Src = "assets/from-step-1.png"
		step2Src = "assets/from-step-2.png"
	)
	base := model.Claim{
		ID:     "widget.contract.surfaces",
		Facet:  "contract",
		Module: "widget",
		Status: model.StatusDraft,
		Body:   "![b](" + bodySrc + ")",
		Steps:  []string{"![s1](" + step1Src + ")", "![s2](" + step2Src + ")"},
		Rows:   []model.Row{{"cell": "![r](assets/from-row.png)"}},
	}

	for layout := range fileForLayout {
		t.Run(string(layout), func(t *testing.T) {
			claim := base
			claim.Layout = layout

			var buf bytes.Buffer
			if err := partials[layout].Execute(&buf, claim); err != nil {
				t.Fatalf("execute %s: %v", layout, err)
			}
			rendered := buf.String()

			// Ground truth: the srcs the real partial emitted, in order.
			var emitted []string
			for _, src := range []string{bodySrc, step1Src, step2Src} {
				if strings.Contains(rendered, `src="/claim-assets/widget.contract.surfaces/`+src+`"`) {
					emitted = append(emitted, src)
				}
			}

			// The claim's declared surfaces, resolved to the same srcs by
			// rendering each one on its own with images enabled.
			var declared []string
			for _, text := range ClaimImageSurfaces(claim) {
				for _, src := range []string{bodySrc, step1Src, step2Src} {
					if strings.Contains(text, src) {
						declared = append(declared, src)
					}
				}
			}

			if strings.Join(declared, ",") != strings.Join(emitted, ",") {
				t.Errorf("%s: ClaimImageSurfaces says %v, the partial emits %v",
					layout, declared, emitted)
			}
			// The <img> count must match too, or a surface could be declared
			// once and rendered twice.
			if n := strings.Count(rendered, `<img class="md-img"`); n != len(emitted) {
				t.Errorf("%s: %d image tags but %d distinct srcs", layout, n, len(emitted))
			}
		})
	}
}

// TestClaimImageSurfaces_TreeAndUnknownLayoutsCarryNothing states the two
// refusals directly, because they are the ones a reader of the table would most
// likely assume the other way round.
func TestClaimImageSurfaces_TreeAndUnknownLayoutsCarryNothing(t *testing.T) {
	c := model.Claim{Body: "![b](assets/b.png)", Steps: []string{"![s](assets/s.png)"}}

	c.Layout = model.LayoutTree
	if got := ClaimImageSurfaces(c); len(got) != 0 {
		t.Errorf("tree: got %v, want none — tree.html renders Body raw inside a <pre>", got)
	}

	c.Layout = model.Layout("no-such-layout")
	if got := ClaimImageSurfaces(c); len(got) != 0 {
		t.Errorf("unknown layout: got %v, want none", got)
	}

	// The empty layout is what a claim carries before internal/catalog infers
	// one. Callers are expected to normalise first; until they do, nothing.
	c.Layout = ""
	if got := ClaimImageSurfaces(c); len(got) != 0 {
		t.Errorf("unset layout: got %v, want none", got)
	}
}

// TestClaimImageSurfaces_StepsCarryBodyAndEveryStep pins the one layout with
// more than one surface, including their order.
func TestClaimImageSurfaces_StepsCarryBodyAndEveryStep(t *testing.T) {
	c := model.Claim{Layout: model.LayoutSteps, Body: "B", Steps: []string{"S1", "S2"}}
	got := ClaimImageSurfaces(c)
	want := []string{"B", "S1", "S2"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %v, want %v", got, want)
	}
}
