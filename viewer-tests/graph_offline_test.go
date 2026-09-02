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
// A request the tab reported, kept beside the document that issued it.
//
// The document matters because the log is the BROWSER's, not the page's. A
// browser that phones home on its own — the Comet fallback in harness_test.go
// posts Sentry telemetry to a URL whose path contains "/api/" — puts a request
// in this log that the viewer never made. Asserting on the bare URL therefore
// attributes the browser's traffic to the page, which is how this suite went
// red on a machine with no real Chrome while CI (pinned to Chrome via
// DOSSIERX_TEST_BROWSER) stayed green. Worse, it was intermittent: the
// telemetry only fires inside some runs' observation window, so a single green
// run proved nothing. Filter by issuing document and the whole class goes away.
type reqEntry struct{ url, doc string }

type requestLog struct {
	mu      sync.Mutex
	entries []reqEntry
}

func (l *requestLog) add(u, doc string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, reqEntry{url: u, doc: doc})
}

func (l *requestLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.entries))
	for _, e := range l.entries {
		out = append(out, e.url)
	}
	return out
}

// fromDocument returns only the requests issued BY the given document, which is
// what "did the viewer ask for anything" actually means.
func (l *requestLog) fromDocument(docURL string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, e := range l.entries {
		if e.doc == docURL {
			out = append(out, e.url)
		}
	}
	return out
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
			log.add(e.Request.URL, e.DocumentURL)
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

		mine := log.fromDocument(url)
		for _, u := range mine {
			if strings.Contains(u, "/api/") {
				t.Fatalf("a file:// viewer issued an API request: %s (all from this document: %v)", u, mine)
			}
		}
		// Nothing at all left the document — no ping, no graph endpoint, no
		// font, no favicon over http. Every request on this tab is the file
		// itself.
		for _, u := range mine {
			if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
				t.Fatalf("a file:// viewer issued a network request: %s", u)
			}
		}
		// The vacuity guard, and it is not optional: every assertion above is a
		// "no such request" over a FILTERED list, so an empty list would pass
		// them all while proving nothing. The document's own fetch is reported
		// with DocumentURL == its own URL, so a correctly-attached listener
		// always leaves at least that one entry here. Measured on the fallback
		// browser: 119 request events on the tab, 118 of them the browser's own
		// chrome:// UI, exactly 1 attributed to this document.
		if len(mine) == 0 {
			t.Fatalf("no request was attributed to %s — the listener never attached, or DocumentURL "+
				"attribution changed; the silence asserted above would be vacuous. All requests seen: %v",
				url, log.snapshot())
		}

		// ...and the pane it opened is fully working, which is what rules out
		// "no requests because nothing ran".
		if evalInt(t, ctx, `document.querySelectorAll('#dxgPane .dxg-canvas').length`) != 1 {
			t.Fatal("the pane did not open, so the silence above is not evidence of anything")
		}
	})

	// A PROJECT'S OWN FONT MUST NOT REOPEN THE HOLE THIS FILE CLOSED.
	//
	// viewer.theme.fonts is the first feature that puts a project-supplied
	// BINARY into the viewer, and the whole single-file guarantee turns on how:
	// inlined as a data: URL, never as a src the browser has to fetch. A
	// relative "fonts/probe.ttf" left in the emitted @font-face would look
	// perfect beside the config, render perfectly on the author's machine where
	// the file is next to the viewer, and be a missing font — or, over http, a
	// network request — for the client the viewer was sent to. That is a
	// property of a browser resolving a src, so it is provable only here.
	t.Run("a project font issues no request of its own", func(t *testing.T) {
		p := newThemedProject(t, fontConfigYAML, true)
		url := p.renderStatic()

		ctx := browserContext(t)
		log := watchRequests(t, ctx)
		runCDP(t, ctx, chromedp.Navigate(url))
		pollTrue(t, ctx, `document.readyState === 'complete'`)
		// The face has to have finished loading before "it asked for nothing"
		// says anything: a request not yet issued is not a request not made.
		assertProbeFontLoaded(t, ctx, "offline check")

		mine := log.fromDocument(url)
		dataFonts := 0
		for _, u := range mine {
			switch {
			case strings.HasPrefix(u, "data:font/"):
				dataFonts++
			case strings.HasPrefix(u, "file:"):
			default:
				short := u
				if len(short) > 120 {
					short = short[:120] + "…"
				}
				t.Fatalf("the viewer issued a request that is neither file: nor an inline data: "+
					"font while loading a project font: %s", short)
			}
		}
		// The plan's wording for this subtest was "zero non-file: requests". A
		// browser reports the inline payload itself as a request with a data:
		// URL, so that literal reading fails on a correctly inlined font — and
		// the interesting assertion is the other way round anyway. Requiring at
		// least ONE data:font/ request is what positively proves the face came
		// from the inlined payload rather than from a relative src that happened
		// to resolve on this machine, which is the actual failure mode.
		if dataFonts == 0 {
			t.Fatalf("the document issued no data:font/ request at all. Either the face is not "+
				"inlined — a relative src resolves on the author's machine and is a missing font "+
				"for the client the viewer was sent to — or nothing on the page uses it. "+
				"Requests from this document: %v", mine)
		}
		// The same vacuity guard the file:// case above needs, for the same
		// reason: every assertion here is a "no such request" over a filtered
		// list, and an empty list would pass them all while proving nothing.
		if len(mine) == 0 {
			t.Fatalf("no request was attributed to %s — the listener never attached, or DocumentURL "+
				"attribution changed; the silence asserted above would be vacuous. All requests seen: %v",
				url, log.snapshot())
		}
		t.Logf("%d request(s) attributed to the document: %d inline data:font/, the rest file:",
			len(mine), dataFonts)
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
