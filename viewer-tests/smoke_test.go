package viewertests

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// TestSmokeBrowser is a fast sanity check that chromedp can actually launch and
// drive the resolved browser headlessly before the heavier serve-backed suite
// runs. It touches no dossierx binary.
func TestSmokeBrowser(t *testing.T) {
	ctx := browserContext(t)
	var title string
	if err := chromedp.Run(ctx,
		chromedp.Navigate("data:text/html,<title>hi</title><h1 id=x>hello</h1>"),
		chromedp.WaitVisible("#x", chromedp.ByID),
		chromedp.Title(&title),
	); err != nil {
		t.Fatalf("chromedp run: %v", err)
	}
	if !strings.Contains(title, "hi") {
		t.Fatalf("title = %q, want to contain %q", title, "hi")
	}
}
