package viewertests

// Phase 5c — SSE live-reload of the viewer. These specs drive a REAL `dossierx
// serve` in a headless browser and prove that an external claim change delivers
// a "changed" over /api/events, the client re-fetches /api/fragment and swaps
// <main class="content-area"> + <nav id="nav"> in place, re-runs initViewer(),
// and RESTORES the view (active module/facet, scroll, open comment panel)
// instead of running the deep-link jump. A marker-less (read-only) shell must
// not wire live reload at all.
//
// Every wait is deterministic (Poll / WaitVisible), never a fixed sleep, so the
// suite is safe under -count=2. A change is only made AFTER comments-sse-open is
// observed, which guarantees the server has registered this tab's SSE
// subscriber (handleEvents subscribes before writing its response head) so the
// resulting "changed" cannot be missed.

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

// ---------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------

// twoFacetConfig is one module (widget) with two real facets, so a claim can
// live in a NON-default facet and the sub-nav renders both subtabs.
const twoFacetConfig = `schema_version: 1
facets:
  - contract
  - design
modules:
  - widget
claims_dir: claims
`

// readOnlyOverrideConfig points viewer.template_overrides at a "tmpl" dir the
// test fills with a marker-less shell.html (see writeMarkerlessShell).
const readOnlyOverrideConfig = `schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
viewer:
  template_overrides: tmpl
`

// facetClaim is a plain lockable draft card in the given facet of module widget.
func facetClaim(id, facet string) string {
	return "id: " + id + `
facet: ` + facet + `
module: widget
status: draft
body: |
  a claim in the ` + facet + ` facet.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
}

// overviewCardClaim is a reserved-overview-facet claim forced to layout: card
// (instead of the orientation-note default of banner) so it CARRIES a comment
// chip and is injected — chip and all — into every facet group of its module.
// That duplication (one id-bearing canonical copy in the default facet, id-less
// copies elsewhere) is exactly the fan-out the chip's data-claim-id keying must
// survive.
func overviewCardClaim(id string) string {
	return "id: " + id + `
facet: overview
module: widget
status: draft
layout: card
body: |
  an overview note rendered as a card so it carries a comment chip.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`
}

// longBodyClaim is a single card whose body is tall enough that the document
// (the default desktop layout scrolls the WINDOW, not .content-area) exceeds any
// headless viewport, so window scroll is meaningful.
func longBodyClaim(id, facet string, paragraphs int) string {
	var b strings.Builder
	b.WriteString("id: " + id + "\nfacet: " + facet + "\nmodule: widget\nstatus: draft\nbody: |\n")
	for i := 0; i < paragraphs; i++ {
		b.WriteString("  Paragraph " + strconv.Itoa(i) + " lorem ipsum dolor sit amet consectetur adipiscing.\n\n")
	}
	b.WriteString("governed_by:\n  type: none\n  reason: viewer-test fixture, not backed by any doctrine claim\n")
	return b.String()
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

// serveAndOpenLive starts serve for p, opens a tab, and waits until the live
// runtime is mounted (comments-live), the SSE handler is wired
// (comments-livereload — the synchronous attach decision), AND the stream has
// actually connected (comments-sse-open). Only after comments-sse-open is a
// subsequent claim change guaranteed to be delivered as a reload.
func serveAndOpenLive(t *testing.T, p *project) context.Context {
	t.Helper()
	base, _ := p.serve()
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(".sec-tab", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.body.classList.contains('comments-live')`)
	pollTrue(t, ctx, `document.body.classList.contains('comments-livereload')`)
	pollTrue(t, ctx, `document.body.classList.contains('comments-sse-open')`)
	return ctx
}

// facetVisible reports the JS expression that is true iff facet group id is the
// visible (non-hidden) one.
func facetVisibleExpr(id string) string {
	return `(function(){var s=document.getElementById('` + id + `');return !!s && !s.hidden;})()`
}

// ---------------------------------------------------------------------
// Restore view: active module/facet is preserved (not reset to the first facet,
// not deep-link-jumped) across a reload.
// ---------------------------------------------------------------------

func TestReloadKeepsActiveFacetVisible(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	p.writeClaim("des.yaml", facetClaim("widget.design.thing", "design"))
	ctx := serveAndOpenLive(t, p)

	// Switch to the SECOND facet (design). Its subtab click records the facet in
	// the hash, which is what the restore-view path re-derives after a reload.
	runCDP(t, ctx, chromedp.Click(`.subtab[data-target="#widget-design"]`, chromedp.ByQuery))
	pollTrue(t, ctx, facetVisibleExpr("widget-design"))
	if !evalBool(t, ctx, `document.getElementById('widget-contract').hidden`) {
		t.Fatal("contract facet should be hidden after switching to design")
	}

	// An external process adds a NEW claim to the (currently hidden) contract
	// facet -> a "changed" -> a live reload. The new card lands in the swapped DOM
	// even though its facet is hidden, giving a deterministic "reload happened"
	// signal without disturbing the active facet.
	p.writeClaim("ctr2.yaml", facetClaim("widget.contract.added", "contract"))
	pollTrue(t, ctx, `!!document.getElementById('widget.contract.added')`)

	// The active module-section is NOT hidden and the active (design) facet is
	// still the visible one: the reload restored the view rather than resetting to
	// the first facet or deep-linking away.
	if evalBool(t, ctx, `document.querySelector('.module-section').hidden`) {
		t.Fatal("active module-section must not be hidden after a reload")
	}
	if !evalBool(t, ctx, facetVisibleExpr("widget-design")) {
		t.Fatal("the active facet (design) must remain visible across a reload")
	}
	if !evalBool(t, ctx, `document.getElementById('widget-contract').hidden`) {
		t.Fatal("the non-active facet (contract) must stay hidden across a reload")
	}

	// A subtab click STILL switches facets after the reload: the listener is
	// delegated on document (the swapped sub-nav buttons carry none) and reads the
	// facetToModule map initViewer just rebuilt. Switch back to contract.
	runCDP(t, ctx, chromedp.Click(`.subtab[data-target="#widget-contract"]`, chromedp.ByQuery))
	pollTrue(t, ctx, facetVisibleExpr("widget-contract"))
	if !evalBool(t, ctx, `document.getElementById('widget-design').hidden`) {
		t.Fatal("a post-reload subtab click must switch the visible facet")
	}
}

// ---------------------------------------------------------------------
// The delegated tab listener (on document, not on the swapped buttons) still
// switches modules after a reload.
// ---------------------------------------------------------------------

func TestReloadDelegatedTabStillSwitchesModules(t *testing.T) {
	p := newProjectRaw(t, twoModuleConfig)
	p.writeClaim("widget.yaml", twoModuleClaim("widget.contract.overview", "widget"))
	p.writeClaim("gadget.yaml", twoModuleClaim("gadget.contract.overview", "gadget"))
	ctx := serveAndOpenLive(t, p)

	// On load: first module shown, second hidden.
	pollTrue(t, ctx, `document.querySelectorAll('.module-section').length === 2 && !document.querySelectorAll('.module-section')[0].hidden && document.querySelectorAll('.module-section')[1].hidden`)

	// External change -> reload (a fresh card in gadget proves the swap ran).
	p.writeClaim("gadget2.yaml", twoModuleClaim("gadget.contract.extra", "gadget"))
	pollTrue(t, ctx, `!!document.getElementById('gadget.contract.extra')`)

	// After the reload, click the SECOND sidebar tab. Its listener is delegated on
	// document (the swapped <nav> buttons carry none), so a working switch proves
	// delegation survived the fragment swap.
	runCDP(t, ctx, chromedp.Evaluate(`document.querySelectorAll('.sec-tab')[1].click();`, nil))
	pollTrue(t, ctx, `document.querySelectorAll('.module-section')[0].hidden && !document.querySelectorAll('.module-section')[1].hidden`)
}

// ---------------------------------------------------------------------
// The reader's scroll position is unchanged across a reload.
// ---------------------------------------------------------------------

func TestReloadPreservesScrollPosition(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("tall.yaml", longBodyClaim("widget.contract.tall", "contract", 150))
	// Seed one comment so the chip reads "1" on load; the reload-trigger below
	// bumps it to "2", a deterministic signal that sits in the card footer (below
	// any reasonable scroll line, so nothing above the fold moves).
	p.run("comment", "add", "widget.contract.tall", "--as", "human", "--body", "seed")
	ctx := serveAndOpenLive(t, p)

	// The tall card makes the document exceed the viewport, so the WINDOW scrolls.
	runCDP(t, ctx, chromedp.Evaluate(`window.scrollTo(0, 300);`, nil))
	pollTrue(t, ctx, `Math.round(window.pageYOffset) === 300`)

	// Add a second comment out-of-band -> reload. The chip flips 1 -> 2.
	p.run("comment", "add", "widget.contract.tall", "--as", "human", "--body", "second")
	pollTrue(t, ctx, `(function(){var c=document.querySelector('.comment-chip .comment-chip-count');return !!c && c.textContent === '2';})()`)

	// The reader's window scroll survived the swap.
	if got := evalInt(t, ctx, `Math.round(window.pageYOffset)`); got != 300 {
		t.Fatalf("window scroll = %d after reload, want 300 (restore-view must preserve scroll)", got)
	}
	// And .content-area's own scrollTop (0 in the default desktop layout) is
	// unchanged too — the literal content-area.scrollTop the restore path also
	// captures and re-applies.
	if got := evalInt(t, ctx, `Math.round(document.querySelector('.content-area').scrollTop)`); got != 0 {
		t.Fatalf("content-area scrollTop = %d, want 0 (unchanged across reload)", got)
	}
}

// ---------------------------------------------------------------------
// A newly added claim id resolves via the rebuilt claimToFacet.
// ---------------------------------------------------------------------

func TestReloadNewClaimResolvesViaClaimToFacet(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	p.writeClaim("des.yaml", facetClaim("widget.design.thing", "design"))
	ctx := serveAndOpenLive(t, p)

	// Default view: contract facet visible, design hidden.
	pollTrue(t, ctx, facetVisibleExpr("widget-contract"))
	if !evalBool(t, ctx, `document.getElementById('widget-design').hidden`) {
		t.Fatal("design facet should start hidden")
	}

	// External process adds a NEW claim to the DESIGN facet -> reload. initViewer
	// must rebuild claimToFacet to include it.
	p.writeClaim("des2.yaml", facetClaim("widget.design.added", "design"))
	pollTrue(t, ctx, `!!document.getElementById('widget.design.added')`)

	// Navigate to the new claim by hash. resolve() consults the freshly rebuilt
	// claimToFacet to map it into the design facet and switch there — if the map
	// were stale, the hash would fall through to the first-module default.
	runCDP(t, ctx, chromedp.Evaluate(`window.location.hash = '#widget.design.added';`, nil))
	pollTrue(t, ctx, facetVisibleExpr("widget-design"))
	if !evalBool(t, ctx, `document.getElementById('widget-contract').hidden`) {
		t.Fatal("navigating to the new claim must switch away from the contract facet")
	}
	if !evalBool(t, ctx, `document.getElementById('widget.design.added').closest('.claim-group').id === 'widget-design'`) {
		t.Fatal("the new claim card did not resolve into the design facet group")
	}
}

// ---------------------------------------------------------------------
// A chip click on a second facet opens the visible (id-less) card's thread —
// the overview-duplication fan-out — and it still works after a reload.
// ---------------------------------------------------------------------

func TestReloadSecondFacetChipOpensThread(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	p.writeClaim("des.yaml", facetClaim("widget.design.thing", "design"))
	p.writeClaim("ov.yaml", overviewCardClaim("widget.overview.summary"))
	p.run("comment", "add", "widget.overview.summary", "--as", "human", "--body", "overview discussion")
	ctx := serveAndOpenLive(t, p)

	// Switch to the design (second) facet. The overview claim is duplicated into
	// it as an id-LESS copy that still carries the chip (data-claim-id).
	runCDP(t, ctx, chromedp.Click(`.subtab[data-target="#widget-design"]`, chromedp.ByQuery))
	pollTrue(t, ctx, facetVisibleExpr("widget-design"))

	// External change -> reload; the design facet stays active and the delegated
	// chip listener survives the swap.
	p.run("comment", "add", "widget.contract.base", "--as", "human", "--body", "ping")
	pollTrue(t, ctx, `!!document.querySelector('#widget-contract [data-claim-id="widget.contract.base"]')`)
	pollTrue(t, ctx, facetVisibleExpr("widget-design"))

	// The design-facet copy of the overview card must be id-less (the canonical
	// id-bearing copy lives in the first/default facet).
	if evalBool(t, ctx, `!!document.querySelector('#widget-design [id="widget.overview.summary"]')`) {
		t.Fatal("the design-facet overview copy should be id-less")
	}

	// Click the chip on that VISIBLE (design-facet) copy. It opens the panel for
	// the overview claim purely by data-claim-id fan-out.
	runCDP(t, ctx, chromedp.Click(`#widget-design [data-claim-id="widget.overview.summary"].comment-chip`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	pollTrue(t, ctx, `Array.from(document.querySelectorAll('#commentsPanel .comment-body')).some(function(b){return b.textContent.indexOf('overview discussion') >= 0;})`)
}

// ---------------------------------------------------------------------
// An open comment panel survives a reload and is re-opened by (claim/thread) id,
// refreshed with whatever change triggered the reload.
// ---------------------------------------------------------------------

func TestReloadOpenPanelSurvives(t *testing.T) {
	p := newProject(t) // single module/facet, claim widget.contract.overview
	tid := p.seedComment("human", "root thread")
	ctx := serveAndOpenLive(t, p)

	// Open the panel on the seeded claim; its thread shows by data-thread-id.
	runCDP(t, ctx, chromedp.Click(".comment-chip", chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	pollTrue(t, ctx, `!!document.querySelector('#commentsPanel .comment-thread[data-thread-id="`+tid+`"]')`)

	// An EXTERNAL reply lands on the open thread -> reload. The panel node lives
	// OUTSIDE the swapped subtree so it survives; the restore path re-opens it by
	// id and refreshes the thread list, which now carries the external reply.
	p.run("comment", "reply", testClaimID, tid, "--as", "human", "--body", "external reply text")
	pollTrue(t, ctx, `Array.from(document.querySelectorAll('#commentsPanel .comment-reply .comment-body')).some(function(b){return b.textContent.indexOf('external reply text') >= 0;})`)

	// Still open, still the same thread by id.
	if !evalBool(t, ctx, `document.body.classList.contains('comments-open')`) {
		t.Fatal("the open comment panel must survive a reload")
	}
	if !evalBool(t, ctx, `!!document.querySelector('#commentsPanel .comment-thread[data-thread-id="`+tid+`"]')`) {
		t.Fatal("the reloaded panel must re-open the same thread by id")
	}
}

// ---------------------------------------------------------------------
// A marker-less (read-only) shell.html override must NOT wire live reload.
// ---------------------------------------------------------------------

func TestReadOnlyOverrideDoesNotWireLiveReload(t *testing.T) {
	p := newProjectRaw(t, readOnlyOverrideConfig)
	p.writeClaim("overview.yaml", draftClaimYAML)
	writeMarkerlessShell(t, p)

	base, _ := p.serve()
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible(".content-area", chromedp.ByQuery),
	)
	// The reachability probe still succeeds — GET /api/ping answers "serve" even
	// in read-only mode — so the read-only viewer mounts (comments-live)...
	pollTrue(t, ctx, `document.body.classList.contains('comments-live')`)

	// ...but because the served shell dropped the runtime marker, the SSE handler
	// never attaches. comments-livereload records the attach DECISION
	// synchronously (the moment comments-live is set), so its absence here is
	// deterministic, not a race with an async connect.
	if evalBool(t, ctx, `document.body.classList.contains('comments-livereload')`) {
		t.Fatal("a marker-less (read-only) shell must NOT wire live reload")
	}
	if evalBool(t, ctx, `document.body.classList.contains('comments-sse-open')`) {
		t.Fatal("a marker-less shell must not open an SSE stream")
	}
}

// writeMarkerlessShell copies the engine's real shell.html into the project's
// template-override dir with ONLY the runtime-marker <meta> line removed. Serve
// then detects the missing marker (render.ShellHasViewerRuntime -> false) and
// runs read-only, and the served shell's own JS sees no marker meta and declines
// to wire live reload. It strips the meta LINE specifically (not every line
// mentioning the token) because the token now occurs in exactly one place — the
// tag — the client JS keys off the meta's content value instead.
func writeMarkerlessShell(t *testing.T, p *project) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	src, err := os.ReadFile(filepath.Join(root, "internal", "render", "viewer", "template", "shell.html"))
	if err != nil {
		t.Fatalf("read engine shell.html: %v", err)
	}
	var kept []string
	removed := false
	for _, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, `<meta name="dossierx-viewer-runtime"`) {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	if !removed {
		t.Fatal("did not find the runtime-marker <meta> line to strip; shell.html shape changed")
	}
	tmplDir := filepath.Join(p.dir, "tmpl")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("mkdir override dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "shell.html"), []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatalf("write marker-less shell override: %v", err)
	}
}
