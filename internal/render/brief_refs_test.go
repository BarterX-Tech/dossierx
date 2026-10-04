package render

import (
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
)

// TestRender_BriefCodeRefsLinkOnlyWhatResolves: a code span in a brief body
// that names another brief or a claim is a link to it; any other code span,
// a span already inside a link, and a code block stay as written (NIT-248).
//
// Authoring gate: (1) a reader can follow a cited brief or claim from the
// body, and nothing reads as a link that goes nowhere; (2) linking every
// code span, linking by folder alone, or wrapping a span inside a markdown
// link (a nested <a>) would each ship a wrong or broken page; (3) the
// markdown-link test (feature_page_test) covers [text](path.md) only, never
// a bare code span; (4) no new seam: it renders through renderBoundedAt.
func TestRender_BriefCodeRefsLinkOnlyWhatResolves(t *testing.T) {
	cat, cfg := briefViewFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("notes/refs.md", "---\nsummary: Refs.\n---\n# Refs\n\n"+
			"- **Lead.** See `voice/talking` and `voice/talking.md` and `"+cat.Claims[0].ID+"`.\n"+
			"- Not refs: `setActivationPolicy`, `voice/nothing`, `future/part`, `updates.example.tech`.\n"+
			"- Already a link: [`voice/talking`](../voice/talking.md).\n\n"+
			"```\nvoice/talking\n```\n"),
		briefFile("voice/talking.md", "---\nsummary: Voice.\n---\n# Talking\n"),
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	page := sectionHTML(t, out, "brief-notes-refs")
	claim := cat.Claims[0].ID
	for _, want := range []string{
		`<a class="code-ref code-ref--brief" href="#brief-voice-talking"><code>voice/talking</code></a>`,
		`<a class="code-ref code-ref--brief" href="#brief-voice-talking"><code>voice/talking.md</code></a>`,
		`<a class="code-ref code-ref--claim" href="#` + claim + `" data-claim-id="` + claim + `"><code>` + claim + `</code></a>`,
		`<code>setActivationPolicy</code>`,
		`<code>voice/nothing</code>`,
		`<code>future/part</code>`,
		`<code>updates.example.tech</code>`,
		`<a href="#brief-voice-talking"><code>voice/talking</code></a>`,
		`<pre><code>voice/talking`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("brief body is missing %s\n%s", want, page)
		}
	}
	for _, bad := range []string{
		`code-ref"><code>setActivationPolicy`,
		`code-ref--brief" href="#brief-voice-talking"><code>voice/nothing`,
		`<a href="#brief-voice-talking"><a `,
		`<pre><a `,
	} {
		if strings.Contains(page, bad) {
			t.Errorf("brief body must not contain %s\n%s", bad, page)
		}
	}
}
