package viewertests

// A BUILD ORDER TAB OPENED AS A FILE ASKS THE NETWORK FOR NOTHING.
//
// The graph pane has this proof in graph_offline_test.go; the Build order tab
// needs its own because it is the one part of the viewer that carries a
// third-party renderer (the vendored mermaid build) whose first act on a page
// is to draw. A renderer that fetched a font, a stylesheet or a telemetry
// beacon would leave the static scan in tests/portability_test.go green (the
// allowlist there is over string literals, not over what runs) and put a
// request on the wire the moment a reader opened build/viewer/index.html.
//
// The shape is the graph test's: the request log is attached BEFORE the
// navigation, the diagrams are waited for so "no request" is a statement
// about a page that actually rendered, the vacuity guard requires at least
// one request attributed to the document, and the served half proves the
// listener sees requests when a page does make them.

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

func TestBuildOrderViewerIssuesNoRequestOnAFileURL(t *testing.T) {
	t.Run("file:// asks for nothing", func(t *testing.T) {
		p := newBuildOrderProject(t)
		url := p.renderStatic()

		ctx := browserContext(t)
		log := watchRequests(t, ctx)
		pe := watchPageErrors(t, ctx)
		runCDP(t, ctx, chromedp.Navigate(url))
		pollTrue(t, ctx, `!!window.mermaid`)
		desktopViewport(t, ctx)
		pollTrue(t, ctx, `document.readyState === 'complete' && !document.body.classList.contains('comments-live')`)
		openBuildOrderTab(t, ctx)
		runCDP(t, ctx, chromedp.Click(`.bo-modules .subtab[data-target="#dossierx-build-order-widget"]`, chromedp.ByQuery))
		waitDiagrams(t, ctx, "widget", widgetSVGs)
		assertNoPageErrors(t, ctx, pe)

		mine := log.fromDocument(url)
		for _, u := range mine {
			if strings.Contains(u, "/api/") {
				t.Fatalf("a file:// viewer issued an API request: %s (all from this document: %v)", u, mine)
			}
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				t.Fatalf("a file:// viewer issued a network request while rendering its build order: %s", u)
			}
		}
		if len(mine) == 0 {
			t.Fatalf("no request was attributed to %s — the listener never attached, or DocumentURL "+
				"attribution changed; the silence asserted above would be vacuous. All requests seen: %v",
				url, log.snapshot())
		}
		if n := evalInt(t, ctx, svgCountExpr("widget")); n != widgetSVGs {
			t.Fatalf("widget rendered %d diagram(s), want %d; the silence above is not evidence of anything", n, widgetSVGs)
		}
	})

	t.Run("under serve the same page does ask", func(t *testing.T) {
		p := newBuildOrderProject(t)
		base := p.ensureServe()

		ctx := browserContext(t)
		log := watchRequests(t, ctx)
		runCDP(t, ctx, chromedp.Navigate(base+"/"))
		pollTrue(t, ctx, `document.body.classList.contains('comments-live')`)

		if got := log.matching("/api/ping"); len(got) == 0 {
			t.Fatalf("the served viewer never probed /api/ping; requests seen: %v", log.snapshot())
		}
	})
}
