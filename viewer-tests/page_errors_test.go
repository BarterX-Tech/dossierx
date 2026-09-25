package viewertests

// PAGE ERRORS, COLLECTED FROM BEFORE NAVIGATION.
//
// A viewer that renders the right DOM while throwing on the way there is
// still broken for the reviewer, and nothing in the DOM says so. These
// helpers record every exception the page throws and every error- or
// warning-level console call, so a test can fail on any of them.

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// pageErrors is the running list of exceptions and error/warning console
// calls one tab has raised.
type pageErrors struct {
	mu    sync.Mutex
	items []string
}

func (e *pageErrors) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.items = append(e.items, s)
}

func (e *pageErrors) snapshot() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.items...)
}

// watchPageErrors attaches the listener to ctx's tab and enables the runtime
// domain. Call it before navigating to see errors raised during load.
func watchPageErrors(t *testing.T, ctx context.Context) *pageErrors {
	t.Helper()
	pe := &pageErrors{}
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			text := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil {
				text += " " + e.ExceptionDetails.Exception.Description
			}
			pe.add("exception: " + text)
		case *runtime.EventConsoleAPICalled:
			if e.Type != runtime.APITypeError && e.Type != runtime.APITypeWarning {
				return
			}
			var parts []string
			for _, a := range e.Args {
				if a.Value != nil {
					parts = append(parts, string(a.Value))
				} else {
					parts = append(parts, a.Description)
				}
			}
			pe.add("console." + string(e.Type) + ": " + strings.Join(parts, " "))
		}
	})
	runCDP(t, ctx, runtime.Enable())
	return pe
}

// assertNoPageErrors fails on any recorded exception or error/warning
// console call.
func assertNoPageErrors(t *testing.T, pe *pageErrors) {
	t.Helper()
	if got := pe.snapshot(); len(got) > 0 {
		t.Fatalf("the page raised %d error(s)/warning(s): %v", len(got), got)
	}
}
