package viewertests

// A viewer opened as a file asks the network for nothing, including after
// the Build order tab was removed.

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
		pollTrue(t, ctx, `document.readyState === 'complete'`)
		desktopViewport(t, ctx)
		pollTrue(t, ctx, `document.readyState === 'complete' && !document.body.classList.contains('comments-live')`)
		if evalBool(t, ctx, `!!document.getElementById('dossierx-build-order')`) {
			t.Fatal("the Build order tab must stay absent")
		}
		assertNoPageErrors(t, ctx, pe)

		mine := log.fromDocument(url)
		for _, u := range mine {
			if strings.Contains(u, "/api/") {
				t.Fatalf("a file:// viewer issued an API request: %s (all from this document: %v)", u, mine)
			}
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				t.Fatalf("a file:// viewer issued a network request: %s", u)
			}
		}
		if len(mine) == 0 {
			t.Fatalf("no request was attributed to %s — the listener never attached, or DocumentURL "+
				"attribution changed; the silence asserted above would be vacuous. All requests seen: %v",
				url, log.snapshot())
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
