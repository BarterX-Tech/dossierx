package lint

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// assetClaim builds a claim loaded from sourcePath whose body carries one
// image with the given src.
func assetClaim(sourcePath, src string) model.Claim {
	return model.Claim{
		ID:         "widget.internals.diagram",
		Facet:      "internals",
		Module:     "widget",
		Status:     model.StatusDraft,
		Layout:     model.LayoutCard,
		Body:       "The flow:\n\n![the flow](" + src + ")\n",
		SourcePath: sourcePath,
	}
}

func TestAssetScope(t *testing.T) {
	// Every case uses the same claim file so the expected asset root is
	// always claims/widget/assets/ — the point of the rule is that the root
	// moves with the claim FILE, not with the claim's module field.
	const claimPath = "claims/widget/diagram.yaml"

	cases := []struct {
		name    string
		claim   model.Claim
		wantIDs []string
	}{
		{
			name:  "passing: an image in the claim's own assets/ directory",
			claim: assetClaim(claimPath, "assets/flow.svg"),
		},
		{
			name:  "passing: an explicitly relative path to the same place",
			claim: assetClaim(claimPath, "./assets/flow.svg"),
		},
		{
			name:  "passing: a subdirectory under assets/ is still in scope",
			claim: assetClaim(claimPath, "assets/diagrams/flow.png"),
		},
		{
			name:  "passing: every allowed extension",
			claim: multiImageClaim(claimPath, "assets/a.png", "assets/b.jpg", "assets/c.jpeg", "assets/d.gif", "assets/e.webp", "assets/f.svg"),
		},
		{
			name:  "passing: an uppercase extension is the same extension",
			claim: assetClaim(claimPath, "assets/FLOW.SVG"),
		},
		{
			name:    "failing: a sibling facet's assets pool",
			claim:   assetClaim(claimPath, "../contract/assets/flow.svg"),
			wantIDs: []string{"widget.internals.diagram"},
		},
		{
			name:    "failing: a shared top-level pool",
			claim:   assetClaim(claimPath, "../assets/flow.svg"),
			wantIDs: []string{"widget.internals.diagram"},
		},
		{
			name:    "failing: an image beside the claim but not under assets/",
			claim:   assetClaim(claimPath, "flow.svg"),
			wantIDs: []string{"widget.internals.diagram"},
		},
		{
			name:    "failing: a differently-named directory beside the claim",
			claim:   assetClaim(claimPath, "images/flow.svg"),
			wantIDs: []string{"widget.internals.diagram"},
		},
		{
			name:    "failing: an unsupported extension, even inside assets/",
			claim:   assetClaim(claimPath, "assets/flow.pdf"),
			wantIDs: []string{"widget.internals.diagram"},
		},
		{
			name:    "failing: no extension at all",
			claim:   assetClaim(claimPath, "assets/flow"),
			wantIDs: []string{"widget.internals.diagram"},
		},
		{
			name: "failing: an image in a step is held to the same rule",
			claim: model.Claim{
				ID:         "widget.internals.diagram",
				SourcePath: claimPath,
				Steps:      []string{"Then compare with ![the old flow](../assets/old.svg)."},
			},
			wantIDs: []string{"widget.internals.diagram"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AssetScope{}.Check([]model.Claim{tc.claim}, nil)
			assertStringSlicesEqual(t, dedupeStrings(findingClaimIDs(got)), tc.wantIDs)
		})
	}
}

// multiImageClaim builds a claim whose body carries one image per src.
func multiImageClaim(sourcePath string, srcs ...string) model.Claim {
	var b strings.Builder
	for _, src := range srcs {
		b.WriteString("![x](" + src + ")\n\n")
	}
	return model.Claim{
		ID:         "widget.internals.diagram",
		SourcePath: sourcePath,
		Body:       b.String(),
	}
}

// TestAssetScope_MovesWithTheClaimFile is the rule's whole point stated as a
// test: the SAME claim, with the same module and facet fields and the same
// body, passes or fails purely on which directory its file sits in. No other
// rule in the package behaves this way — the loader takes module and facet
// from fields inside the claim and ignores directory structure entirely — and
// that asymmetry is what the finding's message has to explain.
func TestAssetScope_MovesWithTheClaimFile(t *testing.T) {
	body := "![flow](assets/flow.svg)"

	nested := model.Claim{ID: "widget.internals.diagram", SourcePath: "claims/widget/diagram.yaml", Body: body}
	flat := model.Claim{ID: "widget.internals.diagram", SourcePath: "claims/diagram.yaml", Body: body}

	if got := (AssetScope{}).Check([]model.Claim{nested}, nil); len(got) != 0 {
		t.Fatalf("claims/widget/diagram.yaml + assets/flow.svg must pass, got %+v", got)
	}
	// The flat layout resolves the SAME src to claims/assets/flow.svg, which
	// is that claim's own assets/ directory, so it passes too. What changes is
	// WHICH file on disk the src names.
	if got := (AssetScope{}).Check([]model.Claim{flat}, nil); len(got) != 0 {
		t.Fatalf("claims/diagram.yaml + assets/flow.svg must pass, got %+v", got)
	}

	// And a src written for one layout fails in the other.
	crossed := model.Claim{ID: "widget.internals.diagram", SourcePath: "claims/diagram.yaml", Body: "![flow](widget/assets/flow.svg)"}
	got := AssetScope{}.Check([]model.Claim{crossed}, nil)
	if len(got) != 1 {
		t.Fatalf("expected exactly one finding, got %+v", got)
	}
	if got[0].Severity != SeverityError {
		t.Errorf("asset-scope findings must be error-severity, got %q", got[0].Severity)
	}
	// The message has to name the resolved path, the claim FILE it resolves
	// against, the expected root, AND the reason an author with a flat layout
	// would otherwise never guess. It must never contain an absolute path:
	// SourcePath is absolute in a real project, and leaking it would put the
	// author's home directory into the JSON envelope and make the same corpus
	// lint to different bytes on two machines.
	for _, want := range []string{
		"widget/assets/flow.svg",
		"diagram.yaml",
		`"assets"`,
		`"assets/flow.svg"`,
		"takes module and facet from fields inside the claim",
	} {
		if !strings.Contains(got[0].Message, want) {
			t.Errorf("message must mention %q; got:\n%s", want, got[0].Message)
		}
	}
	if strings.Contains(got[0].Message, string(filepath.Separator)+"Users") || strings.Contains(got[0].Message, "/home/") {
		t.Errorf("message must not carry an absolute filesystem path; got:\n%s", got[0].Message)
	}
}

// TestAssetScope_CoFiresOnTraversal pins the one deliberate overlap. A
// "../other-facet/assets/x.png" is BOTH refused by the renderer and the
// canonical co-location mistake, and asset-scope's message is the only one
// that explains where images must live — so both rules speak.
func TestAssetScope_CoFiresOnTraversal(t *testing.T) {
	claim := assetClaim("claims/widget/diagram.yaml", "../contract/assets/flow.svg")

	scope := AssetScope{}.Check([]model.Claim{claim}, nil)
	if len(scope) != 1 {
		t.Fatalf("expected one asset-scope finding, got %+v", scope)
	}
	if !strings.Contains(scope[0].Message, "no ../other-facet/assets/") {
		t.Errorf("the message must name the mistake the author just made; got:\n%s", scope[0].Message)
	}

	sanity := MarkdownSanity{}.Check([]model.Claim{claim}, nil)
	if len(sanity) == 0 {
		t.Fatal("markdown-sanity must still report the traversal as a rejected image src")
	}
}

// TestAssetScope_LeavesOffOriginSrcsToMarkdownSanity keeps the two rules in
// their lanes for everything that is not a path at all: an off-origin src has
// no directory to be in or out of, so reporting it here as well would send an
// author looking for two problems.
func TestAssetScope_LeavesOffOriginSrcsToMarkdownSanity(t *testing.T) {
	for _, src := range []string{
		"https://evil.example/p.png",
		"//evil.example/p.png",
		`\\evil.example\p.png`,
		"/assets/p.png",
		"javascript:alert(1)",
	} {
		claim := assetClaim("claims/widget/diagram.yaml", src)
		if got := (AssetScope{}).Check([]model.Claim{claim}, nil); len(got) != 0 {
			t.Errorf("asset-scope must stay silent on the refused src %q, got %+v", src, got)
		}
		if got := (MarkdownSanity{}).Check([]model.Claim{claim}, nil); len(got) == 0 {
			t.Errorf("markdown-sanity must report the refused src %q", src)
		}
	}
}

// TestAssetScope_SkipsUnloadedClaims: SourcePath is populated by the loader
// and is not part of the YAML schema, so a claim built in memory has no
// directory to resolve against. There is nothing to check rather than
// something to permit.
func TestAssetScope_SkipsUnloadedClaims(t *testing.T) {
	claim := model.Claim{ID: "widget.internals.diagram", Body: "![x](../../elsewhere/x.png)"}
	if got := (AssetScope{}).Check([]model.Claim{claim}, nil); len(got) != 0 {
		t.Fatalf("expected no findings for a claim with no SourcePath, got %+v", got)
	}
}

// TestAssetScope_IgnoresFencedExamples: a fenced block is source code an
// author is SHOWING, not markup the viewer renders, so an image reference
// inside one must not be held to the co-location rule.
func TestAssetScope_IgnoresFencedExamples(t *testing.T) {
	claim := model.Claim{
		ID:         "widget.internals.diagram",
		SourcePath: "claims/widget/diagram.yaml",
		Body:       "How not to write it:\n\n```markdown\n![x](../assets/x.png)\n```\n",
	}
	if got := (AssetScope{}).Check([]model.Claim{claim}, nil); len(got) != 0 {
		t.Fatalf("expected no findings for an image inside a fence, got %+v", got)
	}
}

// TestAssetScope_IgnoresNonImageSurfaces: a table cell, a comment body and
// Governed.Reason render no image at all, so nothing there ever becomes an
// asset reference.
func TestAssetScope_IgnoresNonImageSurfaces(t *testing.T) {
	claim := model.Claim{
		ID:         "widget.internals.diagram",
		SourcePath: "claims/widget/diagram.yaml",
		Rows:       []model.Row{{"notes": "![x](../assets/x.png)"}},
		Comments:   []model.Comment{{ID: "c1", Body: "![x](../assets/x.png)"}},
	}
	if got := (AssetScope{}).Check([]model.Claim{claim}, nil); len(got) != 0 {
		t.Fatalf("expected no asset-scope findings on non-image surfaces, got %+v", got)
	}
}
