package markdown

import (
	"reflect"
	"testing"
)

// TestRenderDocument_HeadingsAreAllowedOnlyInDocumentMode is the one contract
// document mode adds: "#" and "##" are headings in a brief and stay literal text
// in a claim body. Both halves are asserted in one table because the regression
// that matters is either one moving — document mode failing to lower the level,
// or the capability leaking into the claim entry point it must never reach.
func TestRenderDocument_HeadingsAreAllowedOnlyInDocumentMode(t *testing.T) {
	for _, tc := range []struct {
		in, document, claim string
	}{
		{"# Checkout flow\n", "<h1>Checkout flow</h1>", "<p># Checkout flow</p>"},
		{"## Why\n", "<h2>Why</h2>", "<p>## Why</p>"},
		{"### Detail\n", "<h3>Detail</h3>", "<h3>Detail</h3>"},
		{"####### Seven\n", "<p>####### Seven</p>", "<p>####### Seven</p>"},
		{"#x\n", "<p>#x</p>", "<p>#x</p>"},
		{"> # Quoted\n", "<blockquote><h1>Quoted</h1></blockquote>", "<blockquote><p># Quoted</p></blockquote>"},
	} {
		if got := string(RenderDocument(tc.in, "")); got != tc.document {
			t.Errorf("RenderDocument(%q)\n got: %s\nwant: %s", tc.in, got, tc.document)
		}
		if got := string(RenderClaimBody(tc.in, "/assets/c/", Citations{})); got != tc.claim {
			t.Errorf("RenderClaimBody(%q)\n got: %s\nwant: %s", tc.in, got, tc.claim)
		}
	}
}

// TestDocumentImageSrc pins the document gate: one file in the brief's own
// folder, named by the folder's own [a-z0-9-] rule, with a lowercase image
// extension. Every refusal here is a src the claim gate or a looser dialect
// would read as an image.
func TestDocumentImageSrc(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"flow.png", "flow.png", true},
		{"step-2.svg", "step-2.svg", true},
		{"assets/flow.png", "", false}, // the claim layout is not a brief's
		{"./flow.png", "", false},
		{"../other/flow.png", "", false},
		{"Flow.png", "", false}, // the folder refuses the name, so the gate does
		{"flow.PNG", "", false},
		{"flow.txt", "", false},
		{"flow", "", false},
		{"a b.png", "", false},
		{"a&#32;b.png", "", false}, // a second spelling of a space is still a space
		{"https://example.com/flow.png", "", false},
		{"/flow.png", "", false},
	} {
		got, ok := DocumentImageSrc(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("DocumentImageSrc(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// TestRenderDocument_ImagesFollowTheirPrefix pins that an accepted image is
// emitted only when the caller has somewhere to serve it from, and that the
// reference list and the title come from the same scan the page does: an image
// or a "#" inside a fenced example is neither a reference nor a title.
func TestRenderDocument_ImagesFollowTheirPrefix(t *testing.T) {
	body := "Intro ![Flow](flow.png)\n\n```\n# not a title\n![x](fenced.png)\n```\n\n# Real title\n\n![Again](flow.png) ![Step](step.svg) ![bad](assets/x.png)\n"

	if got := string(RenderDocument("![Flow](flow.png)\n", "")); got != "<p>![Flow](flow.png)</p>" {
		t.Errorf("zero prefix must render the image as literal text, got %s", got)
	}
	if got := string(RenderDocument("![Flow](flow.png)\n", "/briefs/checkout.flow/")); got != `<p><img class="md-img" src="/briefs/checkout.flow/flow.png" alt="Flow"></p>` {
		t.Errorf("prefixed image, got %s", got)
	}

	if got, want := DocumentImages(body), []string{"flow.png", "flow.png", "step.svg"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DocumentImages = %v, want %v", got, want)
	}
	if got, ok := DocumentTitle(body); !ok || got != "Real title" {
		t.Errorf("DocumentTitle = %q, %v; want %q, true", got, ok, "Real title")
	}
	if got, ok := DocumentTitle("## Only a section\n\ntext\n"); ok || got != "" {
		t.Errorf("a body with no level-1 heading has no title, got %q, %v", got, ok)
	}
}
