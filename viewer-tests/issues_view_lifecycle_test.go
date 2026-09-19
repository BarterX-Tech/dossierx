package viewertests

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// issuesLifecycleProject is a facet with real blockers, so the status strip
// has findings to hand to the Issues screen.
func issuesLifecycleProject(t *testing.T) *project {
	t.Helper()
	p := newReadinessProject(t)
	p.writeClaim("issue.yaml", strings.ReplaceAll(contractIssueClaimYAML, "widget.contract.missing", "widget.contract.alpha"))
	return p
}

// TestIssuesViewSurvivesCloseAndReopen pins the bug where the Issues screen
// showed "Nothing in this facet is blocked." on the SECOND open while its own
// rail still counted the real blockers — one screen contradicting itself.
//
// The cause is that issuesSyncFromStrip MOVES #statusStripBody's children into
// the findings card rather than cloning them, so the strip is left drained and
// closing never gave them back. The next open then synced from an empty source
// and printed the empty state, while the rail — which recomputes from
// lastStatusData rather than the DOM — still reported the true count.
func TestIssuesViewSurvivesCloseAndReopen(t *testing.T) {
	p := issuesLifecycleProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible("#statusStrip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelectorAll('#statusStripBody .status-group').length > 0`)
	groups := evalInt(t, ctx, `document.querySelectorAll('#statusStripBody .status-group').length`)

	open := func(pass string) {
		runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('statusStripToggle').click()`, nil))
		pollTrue(t, ctx, `document.getElementById('issuesView').hidden === false`)
		if got := evalInt(t, ctx, `document.querySelectorAll('#issuesFindingsCard .status-group').length`); got != groups {
			t.Fatalf("%s: findings card has %d groups, want the %d the strip carried", pass, got, groups)
		}
		if evalBool(t, ctx, `!!document.querySelector('.issues-findings-empty')`) {
			t.Fatalf("%s: the empty state must not render while this facet has blockers", pass)
		}
		if !evalBool(t, ctx, `!!document.querySelector('#issuesFilters .status-severity-chip')`) {
			t.Fatalf("%s: the severity chips must travel with the findings", pass)
		}
	}

	open("first open")
	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('issuesBack').click()`, nil))
	pollTrue(t, ctx, `document.getElementById('issuesView').hidden === true`)

	// The nodes must be back where the reading view expects them, or its strip
	// renders expanded-but-blank until the next status poll repaints it.
	if got := evalInt(t, ctx, `document.querySelectorAll('#statusStripBody .status-group').length`); got != groups {
		t.Fatalf("after closing, #statusStripBody has %d groups, want its original %d back", got, groups)
	}
	open("second open")
}

// TestHashChangeLeavesTheIssuesView pins that asking for a claim closes the
// Issues screen.
//
// The Issues view hides .content-area. A hash change is a request for a claim
// in the reading view — the graph pane's "back to this claim", a facet-TOC
// row, the browser Back button, a shared link — but nothing closed the Issues
// view, so the reader stayed stranded on it while showModuleFacet silently
// re-synced the findings to the NEW facet behind a breadcrumb still naming the
// old one. The deep-link scroll was a no-op too, since scrollIntoView does
// nothing under a hidden ancestor.
func TestHashChangeLeavesTheIssuesView(t *testing.T) {
	p := issuesLifecycleProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.renderStatic()),
		chromedp.WaitVisible("#statusStrip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.querySelectorAll('#statusStripBody .status-group').length > 0`)

	runCDP(t, ctx, chromedp.Evaluate(`document.getElementById('statusStripToggle').click()`, nil))
	pollTrue(t, ctx, `document.getElementById('issuesView').hidden === false && document.querySelector('.content-area').hidden === true`)

	runCDP(t, ctx, chromedp.Evaluate(`window.location.hash = '#widget.contract.root'`, nil))
	pollTrue(t, ctx, `document.getElementById('issuesView').hidden === true`)
	if !evalBool(t, ctx, `document.querySelector('.content-area').hidden === false`) {
		t.Fatal("asking for a claim must return the reader to the reading view, not leave .content-area hidden behind the Issues screen")
	}
	// And the claim really is reachable, which a hidden ancestor would prevent.
	if !evalBool(t, ctx, `(function(){
  var c = document.getElementById('widget.contract.root');
  return !!c && c.getBoundingClientRect().height > 0;
 })()`) {
		t.Fatal("the deep-linked claim must be laid out and visible after the Issues view closes")
	}
}
