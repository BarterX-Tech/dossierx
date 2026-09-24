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
	"fmt"
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

// longBodyClaim is a card whose markdown body is many paragraphs. The live
// reading view clamps a long body to four lines until the reader expands it,
// so one of these cards is NOT by itself taller than a desktop viewport.
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
	// facetToModule map initViewer just rebuilt. Resolve and click the CURRENT
	// post-swap node in one browser task: coordinate-based input can otherwise
	// retain a pre-settle target while the restored page is still laying out on a
	// slow headless runner, which tests hit-testing rather than delegation.
	evalVoid(t, ctx, `document.querySelector('.subtab[data-target="#widget-contract"]').click()`)
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
	// One clamped card fits inside a desktop viewport (Chrome here measured
	// window/html/body/.layout/.content-area overflow:visible and a 0px
	// document range; .reading-canvas is overflow:clip at equal client/scroll
	// height, so it is not a scroller). Extra cards make the WINDOW — still
	// the reader-visible scroller — exceed the viewport without expanding
	// the body. Expansion would not survive a fragment swap (the disclosure
	// wrapper is recreated), so it cannot be how this fixture earns height.
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("widget.contract.filler-%02d", i)
		p.writeClaim(id+".yaml", longBodyClaim(id, "contract", 40))
	}
	// Seed one comment so the chip reads "1" on load; the reload-trigger below
	// bumps it to "2", a deterministic signal that sits in the card footer (below
	// any reasonable scroll line, so nothing above the fold moves).
	p.run("comment", "add", "widget.contract.tall", "--as", "human", "--body", "seed")
	ctx := serveAndOpenLive(t, p)
	desktopViewport(t, ctx)

	// Confirm the reader-visible scroller from computed overflow + range, then
	// take an offset from what THAT scroller can actually scroll. A hardcoded
	// 300 that clamps to the maximum would compare the clamp against itself.
	scroller := evalString(t, ctx, readingScrollerKindJS)
	rangePx := evalInt(t, ctx, readingScrollerRangeJS)
	if scroller != "window" {
		t.Fatalf("reader-visible scroller is %q (overflow/range), not window; restore-view only reapplies window + .content-area — update the product restore path and this assertion together", scroller)
	}
	target := rangePx
	if target > 300 {
		target = 300
	}
	if target < 50 {
		t.Fatalf("fixture can only scroll %dpx on %s; it is too short to prove scroll is preserved", target, scroller)
	}
	runCDP(t, ctx, chromedp.Evaluate(fmt.Sprintf(`window.scrollTo(0, %d);`, target), nil))
	pollTrue(t, ctx, fmt.Sprintf(`Math.round(window.pageYOffset) === %d`, target))

	// Add a second comment out-of-band -> reload. The chip flips 1 -> 2.
	p.run("comment", "add", "widget.contract.tall", "--as", "human", "--body", "second")
	pollTrue(t, ctx, `(function(){var c=document.querySelector('.comment-chip .comment-chip-count');return !!c && c.textContent === '2';})()`)

	if got := evalString(t, ctx, readingScrollerKindJS); got != scroller {
		t.Fatalf("reader-visible scroller became %q after reload, was %q", got, scroller)
	}
	if got := evalInt(t, ctx, `Math.round(window.pageYOffset)`); got != target {
		t.Fatalf("window scroll = %d after reload, want %d (restore-view must preserve scroll)", got, target)
	}
	// .content-area is not the scroller (overflow:visible, scrollTop 0). The
	// restore path still captures and re-applies that value; it must stay 0.
	if got := evalInt(t, ctx, `Math.round(document.querySelector('.content-area').scrollTop)`); got != 0 {
		t.Fatalf("content-area scrollTop = %d, want 0 (unchanged across reload)", got)
	}
}

// readingScrollerKindJS names the reader-visible vertical scroller: window, or
// a .content-area descendant whose overflow-y is auto/scroll/overlay and whose
// scroll range beats the document. overflow:clip is not a scroller.
const readingScrollerKindJS = `(function(){
  var winRange = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
  var best = {kind:'window', range:winRange};
  var root = document.querySelector('.content-area');
  if (!root) return 'missing-content-area';
  var nodes = [root];
  var desc = root.querySelectorAll('*');
  for (var i = 0; i < desc.length; i++) nodes.push(desc[i]);
  for (var j = 0; j < nodes.length; j++) {
    var el = nodes[j];
    var oy = getComputedStyle(el).overflowY;
    if (oy !== 'auto' && oy !== 'scroll' && oy !== 'overlay') continue;
    var range = el.scrollHeight - el.clientHeight;
    if (range > best.range + 1) {
      if (el.classList.contains('content-area')) { best = {kind:'.content-area', range:range}; continue; }
      if (el.classList.contains('reading-canvas')) { best = {kind:'.reading-canvas', range:range}; continue; }
      best = {kind:'nested:'+el.tagName.toLowerCase()+'.'+(el.className||'').toString().split(' ')[0], range:range};
    }
  }
  return best.kind;
})()`

const readingScrollerRangeJS = `(function(){
  var kind = ` + readingScrollerKindJS + `;
  if (kind === 'window') return Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
  var sel = kind.charAt(0) === '.' ? kind : null;
  var el = sel ? document.querySelector(sel) : null;
  if (!el) return 0;
  return Math.max(0, el.scrollHeight - el.clientHeight);
})()`

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

func TestReloadSecondFacetChipOpensThread(t *testing.T) {
	p := newProjectRaw(t, twoFacetConfig)
	p.writeClaim("ctr.yaml", facetClaim("widget.contract.base", "contract"))
	p.writeClaim("des.yaml", facetClaim("widget.design.thing", "design"))
	p.run("comment", "add", "widget.design.thing", "--as", "human", "--body", "design discussion")
	ctx := serveAndOpenLive(t, p)

	runCDP(t, ctx, chromedp.Click(`.subtab[data-target="#widget-design"]`, chromedp.ByQuery))
	pollTrue(t, ctx, facetVisibleExpr("widget-design"))

	p.run("comment", "add", "widget.contract.base", "--as", "human", "--body", "ping")
	pollTrue(t, ctx, `!!document.querySelector('#widget-contract [data-claim-id="widget.contract.base"]')`)
	pollTrue(t, ctx, facetVisibleExpr("widget-design"))

	runCDP(t, ctx, chromedp.Click(`#widget-design [data-claim-id="widget.design.thing"].comment-chip`, chromedp.ByQuery))
	pollTrue(t, ctx, `document.body.classList.contains('comments-open')`)
	pollTrue(t, ctx, `Array.from(document.querySelectorAll('#commentsPanel .comment-body')).some(function(b){return b.textContent.indexOf('design discussion') >= 0;})`)
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

// ---------------------------------------------------------------------
// The Build order tab across a fragment swap: the diagrams are rendered
// again from the fresh source, the payload delivered with the swap is the
// one the click handler reads, and the zero-to-one transition (a project
// that locks its FIRST order while the page is open) reloads once so the
// renderer arrives.
// ---------------------------------------------------------------------

func TestReloadRerendersBuildOrderDiagrams(t *testing.T) {
	p := newBuildOrderProject(t)
	ctx := serveAndOpenLive(t, p)
	pe := watchPageErrors(t, ctx)
	desktopViewport(t, ctx)
	if evalBool(t, ctx, `!!document.getElementById('dossierx-build-order')`) {
		t.Fatal("the Build order tab must stay absent")
	}
	p.writeClaim("single-extra.yaml", boClaim("single.contract.extra", "contract", "single", "orientation"))
	pollTrue(t, ctx, `!!document.getElementById('single.contract.extra')`)
	if evalBool(t, ctx, `!!document.getElementById('dossierx-build-order')`) {
		t.Fatal("a fragment swap must not revive the Build order tab")
	}
	assertNoPageErrors(t, ctx, pe)
}

func TestReloadZeroToOneLockedOrderReloadsForTheRenderer(t *testing.T) {
	// Freshness (2): a project with NO locked order carries no renderer. The
	// page is open while its first order locks; the next swap delivers a
	// #dossierx-build-order section with no mermaid, and the shell reloads once.
	p := newProjectRaw(t, buildOrderConfig)
	p.writeClaim("widget-schema.yaml", boClaim("widget.contract.schema", "contract", "widget", "schema"))
	p.writeClaim("widget-behavior.yaml", boClaim("widget.contract.behavior", "contract", "widget", "behavior", "widget.contract.schema"))
	p.writeClaim("single-only.yaml", boClaim("single.contract.only", "contract", "single", "orientation"))
	// Lock the dependency chain before opening the page. Draft prerequisites
	// now legitimately load Mermaid for readiness traces, so the old fixture no
	// longer represented a zero-renderer page. Locked claims with no Build order
	// retain that exact transition and keep this reload contract meaningful.
	p.run("claim", "lock", "widget.contract.schema", "--reason", "viewer-test fixture")
	p.run("claim", "lock", "widget.contract.behavior", "--reason", "viewer-test fixture")
	ctx := serveAndOpenLive(t, p)
	if !evalBool(t, ctx, `typeof window.mermaid === 'undefined'`) {
		t.Fatal("a project with no locked order must not carry the renderer")
	}
	if evalBool(t, ctx, `!!document.getElementById('dossierx-build-order')`) {
		t.Fatal("a project with no locked order must render no #dossierx-build-order section")
	}
	runCDP(t, ctx, chromedp.Evaluate(`window.__boMarker = true;`, nil))

	p.writeClaim("single-extra.yaml", boClaim("single.contract.extra", "contract", "single", "orientation"))
	pollTrue(t, ctx, `!!document.getElementById('single.contract.extra')`)
	if !evalBool(t, ctx, `window.__boMarker === true`) {
		t.Fatal("the marker is gone: adding a draft must not force a full reload for a retired tab")
	}
	if evalBool(t, ctx, `!!document.getElementById('dossierx-build-order')`) {
		t.Fatal("a fragment swap must not create a Build order tab")
	}
}
