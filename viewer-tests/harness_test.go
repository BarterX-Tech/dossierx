// Package viewertests is a SEPARATE Go module (see go.mod) that drives the
// BUILT dossierx binary black-box with a headless Chromium via chromedp. It is
// deliberately not part of the root module: chromedp and its transitive deps
// must never enter the engine's go.mod (which stays cobra + yaml.v3 only), so
// this harness lives under its own module that the root `go build ./...` /
// `go test ./...` skip entirely.
//
// BECAUSE the root module cannot reach it, running it takes a deliberate entry
// point, and there are exactly two: `make viewer-test` locally, and the
// `viewer` job in .github/workflows/ci.yml, which runs it on ubuntu-latest
// against the runner image's preinstalled Chrome. Anything that stops being run
// by both of those stops being covered at all — the root suite asserts the
// viewer's MARKUP (internal/render), never its behaviour in a browser.
//
// The suite proves the Phase 5 viewer comment UI in a real browser: the
// reachability probe mounts the write controls only against a live `dossierx
// serve`, a static file:// viewer stays read-only, and the composer / resolve /
// edit / delete / reply / reopen controls drive the JSON API and update the DOM.
package viewertests

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// allocMu guards the lazily-created shared browser ExecAllocator: one browser
// process for the whole suite, each test opening its own tab via NewContext.
// allocCancel is invoked by TestMain (project_test.go) at teardown.
var (
	allocMu     sync.Mutex
	allocCtx    context.Context
	allocCancel context.CancelFunc
)

// resolveBrowser locates a Chrome/Chromium executable to drive, in priority
// order: the DOSSIERX_TEST_BROWSER env var (an explicit override), then the
// common install locations for Chrome/Chromium on this platform, then the Comet
// build known to exist on the maintainer's machine. If none is found the test
// SKIPS with a clear message rather than failing — a machine without a browser
// simply cannot run the browser suite.
//
// A skip is the right answer on a developer's laptop and the WRONG one in CI,
// where it is indistinguishable from a pass over zero assertions. So the CI job
// sets DOSSIERX_TEST_BROWSER explicitly, and the override branch below turns a
// path that does not exist into a t.Fatal rather than a skip: with the variable
// set, "no browser" is a failure, and only the unset case can skip quietly.
func resolveBrowser(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("DOSSIERX_TEST_BROWSER"); p != "" {
		if fileExists(p) {
			return p
		}
		t.Fatalf("DOSSIERX_TEST_BROWSER=%q does not exist", p)
	}
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/Applications/Comet.app/Contents/MacOS/Comet",
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	t.Skip("no Chrome/Chromium browser found; set DOSSIERX_TEST_BROWSER to run the viewer browser suite")
	return ""
}

// browserAllocOpts is the chromedp ExecAllocator option set every test shares:
// the resolved browser path plus a headless, sandbox-light configuration that
// runs cleanly in CI. --no-sandbox is required because the test process is not
// guaranteed a user namespace; --disable-gpu / --headless keep it off-screen.
func browserAllocOpts(browser string) []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.ExecPath(browser),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	return opts
}

// browserContext returns a fresh browser tab (its own chromedp context off the
// shared allocator) with a per-test timeout. It resolves/launches the browser
// lazily on first use, and SKIPS the test when no browser is available. The
// returned context is cleaned up automatically via t.Cleanup.
func browserContext(t *testing.T) context.Context {
	t.Helper()
	browser := resolveBrowser(t) // may t.Skip / t.Fatal

	allocMu.Lock()
	if allocCtx == nil {
		allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), browserAllocOpts(browser)...)
	}
	shared := allocCtx
	allocMu.Unlock()

	tabCtx, cancelTab := chromedp.NewContext(shared)
	t.Cleanup(cancelTab)
	timedCtx, cancelTimeout := context.WithTimeout(tabCtx, 60*time.Second)
	t.Cleanup(cancelTimeout)
	return timedCtx
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
