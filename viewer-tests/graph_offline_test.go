package viewertests

// A VIEWER OPENED AS A FILE ASKS THE NETWORK FOR NOTHING.
//
// The whole premise of the rendered artifact is that it is one self-contained
// document that works with no server, no CDN and no network at all — and the
// repository enforces the static half of that with an offline scan over every
// shipped .go/.html/.css/.js file. What no scan could see is that the viewer
// ISSUED A REQUEST ANYWAY: the reachability probe fetched a relative
// /api/ping on load to decide whether to mount the write controls, so opening
// any viewer as a file logged net::ERR_FILE_NOT_FOUND against a document whose
// entire claim is that it needs nothing. The probe now asks the protocol
// instead of asking the network.
//
// That is a property of a running browser, so this is where it can be proven.
// The assertion is made against the browser's own request log via CDP, not
// against the page's fetch(), because a page can reach the network through
// more than one API and the log sees all of them.
//
// The served half of the table is not decoration: it is what makes the
// file:// half mean something. If the listener were mis-wired, "no requests"
// would be true of every page and the test would pass over zero observations.

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// requestLog records every request the browser issues on a tab, from before
// the first navigation.
type requestLog struct {
	mu   sync.Mutex
	urls []string
}

func (l *requestLog) add(u string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.urls = append(l.urls, u)
}

func (l *requestLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.urls...)
}

func (l *requestLog) matching(substr string) []string {
	var out []string
	for _, u := range l.snapshot() {
		if strings.Contains(u, substr) {
			out = append(out, u)
		}
	}
	return out
}

// watchRequests attaches the request listener to a fresh tab and enables the
// Network domain. It must be called BEFORE the tab navigates, which is why this
// test does not use staticGraphTab.
func watchRequests(t *testing.T, ctx context.Context) *requestLog {
	t.Helper()
	log := &requestLog{}
	chromedp.ListenTarget(ctx, func(ev any) {
		if e, ok := ev.(*network.EventRequestWillBeSent); ok {
			log.add(e.Request.URL)
		}
	})
	runCDP(t, ctx, network.Enable())
	return log
}

func TestGraphViewerIssuesNoRequestOnAFileURL(t *testing.T) {
	t.Run("file:// asks for nothing", func(t *testing.T) {
		p := newGraphProject(t)
		url := p.renderStatic()

		ctx := browserContext(t)
		log := watchRequests(t, ctx)
		runCDP(t, ctx, chromedp.Navigate(url))
		pollTrue(t, ctx, `!!window.dossierxGraphCore`)
		desktopViewport(t, ctx)

		// The probe's own ~1s window has to have elapsed before "it never
		// fired" is a statement about anything. The read-only verdict landing
		// is the deterministic signal that the probe path has run to its end.
		pollTrue(t, ctx, `document.readyState === 'complete' && !document.body.classList.contains('comments-live')`)
		openGraphPane(t, ctx)
		waitVisible(t, ctx, "#dxgPane .dxg-canvas")

		if got := log.matching("/api/"); len(got) != 0 {
			t.Fatalf("a file:// viewer issued %d API request(s): %v", len(got), got)
		}
		// Nothing at all left the document — no ping, no graph endpoint, no
		// font, no favicon over http. Every request on this tab is the file
		// itself.
		for _, u := range log.snapshot() {
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				t.Fatalf("a file:// viewer issued a network request: %s", u)
			}
		}
		if n := len(log.snapshot()); n == 0 {
			t.Fatal("the request log is empty, including the document itself: the listener never saw anything, so this proves nothing")
		}

		// ...and the pane it opened is fully working, which is what rules out
		// "no requests because nothing ran".
		if evalInt(t, ctx, `document.querySelectorAll('#dxgPane .dxg-canvas').length`) != 1 {
			t.Fatal("the pane did not open, so the silence above is not evidence of anything")
		}
	})

	t.Run("under serve the same page does ask", func(t *testing.T) {
		p := newGraphProject(t)
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
