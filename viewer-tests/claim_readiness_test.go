package viewertests

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

const (
	// These are browser-product budgets, separate from the engine's 64 MiB
	// serialized-output containment gate. They deliberately leave headroom for
	// slower CI runners while still catching a duplicated blocker list or a
	// projection that makes the claim page impractical.
	//
	// readinessScaleMaxMapMS is retired: docs/design/screens/
	// 06-claim-blocked-across-four-modules.md's R09.9 ("no inline dependency
	// map") removed the focused Mermaid trace this budget used to time.
	readinessScaleMaxLoadMS      = 10_000
	readinessScaleMaxDOMNodes    = 100_000
	readinessScaleMaxJSHeapBytes = 512 * 1024 * 1024
)

// openReadinessPanel opens a claim's readiness door and waits for its panel to
// be on screen.
//
// screens/02 section 9.1 suppresses R09.3's auto-open whenever a facet-level
// blocked banner is showing, and every fixture here raises one, so the door now
// paints CLOSED. The panel (.claim-readiness) is the door's next sibling and is
// only visible while the door is open — a bare WaitVisible on it would hang.
// Tests asserting on panel CONTENT therefore open the door first; what they are
// about is the content, not the default state, which
// TestFacetBannerSuppressesReadinessAutoOpen owns.
func openReadinessPanel(t *testing.T, ctx context.Context, doorSelector string) {
	t.Helper()
	runCDP(t, ctx, chromedp.WaitVisible(doorSelector, chromedp.ByQuery))
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var d = document.querySelector(`+strconv.Quote(doorSelector)+`);
		if (d && !d.open) { d.querySelector('summary').click(); }
	})()`, nil))
	pollTrue(t, ctx, `(function(){ var d = document.querySelector(`+strconv.Quote(doorSelector)+`); return !!d && d.open; })()`)
}

func readinessScaleProject(t *testing.T, layers, width int) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	for layer := layers - 1; layer >= 0; layer-- {
		for node := 0; node < width; node++ {
			id := fmt.Sprintf("widget.contract.l%03d-n%02d", layer, node)
			var restsOn strings.Builder
			if layer+1 < layers {
				restsOn.WriteString("rests_on:\n")
				for dependency := 0; dependency < width; dependency++ {
					fmt.Fprintf(&restsOn, "  - widget.contract.l%03d-n%02d\n", layer+1, dependency)
				}
			}
			restsBlock := restsOn.String()
			if restsBlock == "" {
				restsBlock = "rests_on:\n  none: true\n  reason: viewer-test fixture, not backed by any doctrine claim\n"
			}
			p.writeClaim(id+".yaml", fmt.Sprintf(`id: %s
facet: contract
module: widget
status: draft
body: |
  browser scale fixture at layer %d, node %d.
%s`, id, layer, node, restsBlock))
		}
	}
	return p
}

func readinessScaleFacts(layers, width int) int {
	// Every node owns one condition for every unique node in every later layer.
	// Full inter-layer fan-out creates exponentially many routes, but readiness
	// retains one condition per independently clearable dependency identity.
	return width * width * layers * (layers - 1) / 2
}

type readinessScaleMetrics struct {
	LoadMS         float64 `json:"load_ms"`
	DOMNodes       int     `json:"dom_nodes"`
	JSHeapBytes    float64 `json:"js_heap_bytes"`
	ReadinessDoors int     `json:"readiness_doors"`
	ReadyChips     int     `json:"ready_chips"`
	VisibleRows    int     `json:"visible_rows"`
	RawFacts       int     `json:"raw_facts"`
	RawPanels      int     `json:"raw_panels"`
	MapElements    int     `json:"map_elements"`
}

func readReadinessScaleMetrics(t *testing.T, ctx context.Context) readinessScaleMetrics {
	t.Helper()
	var metrics readinessScaleMetrics
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var nav = performance.getEntriesByType('navigation')[0];
		var raws = Array.from(document.querySelectorAll('.claim-readiness-raw pre'));
		return {
			load_ms: nav ? nav.loadEventEnd - nav.startTime : -1,
			dom_nodes: document.querySelectorAll('*').length,
			js_heap_bytes: performance.memory ? performance.memory.usedJSHeapSize : -1,
			readiness_doors: document.querySelectorAll('.claim-readiness-door, .claim-readiness-empty').length,
			ready_chips: document.querySelectorAll('.claim-readiness-empty').length,
			visible_rows: document.querySelectorAll('.claim-readiness-blocker').length,
			raw_facts: raws.reduce(function(total, pre){
				var raw = JSON.parse(pre.textContent);
				return total + (raw.dependency_conditions || []).length + (raw.review_causes || []).length;
			}, 0),
			raw_panels: raws.length,
			// R09.9 ("no inline dependency map"): none of these selectors
			// should ever match again. A non-zero count here is a
			// regression, not a budget.
			map_elements: document.querySelectorAll('.claim-readiness-map, .claim-readiness-trace, .claim-readiness-route').length
		};
	})()`, &metrics))
	return metrics
}

// scopeCounts parses a ".claim-readiness-scope" string, e.g. "127 across 1
// module" or "1 across 1 module", into its two numerals.
func scopeCounts(t *testing.T, text string) (facts, modules int) {
	t.Helper()
	re := regexp.MustCompile(`^(\d+) across (\d+) module`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		t.Fatalf("readiness scope text %q does not match 'N across M module(s)'", text)
	}
	facts, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("scope fact count: %v", err)
	}
	modules, err = strconv.Atoi(m[2])
	if err != nil {
		t.Fatalf("scope module count: %v", err)
	}
	return facts, modules
}

// TestReadinessBrowserScaleBudgets: docs/design/screens/
// 06-claim-blocked-across-four-modules.md's redesign changed two things this
// suite must still hold at scale even though it re-pins how it proves them.
// (1) R09.9 ("no inline dependency map") means there is no Mermaid trace to
// budget or lazily render any more — every .claim-readiness-map/-trace/-route
// selector is now permanently absent (dead-selector regression guard below).
// (2) Grouping moved from "the representative-route claim" to "the module
// that owns the fix" (06 §7.6): this fixture's every claim shares one module
// (widget), so each claim's readiness panel now has exactly ONE module group
// holding its COMPLETE fact list, rather than many small per-dependency
// groups. The within-module cap (06 §4.4's "Show 9 more in this module"; 2
// rendered up front) is what keeps a project whose fan-out reaches thousands
// of facts under the DOM-node budget: viewer-runtime.js's readinessModule
// builds the tail lazily, on the "Show N more" click, never up front.
func TestReadinessBrowserScaleBudgets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		layers int
		width  int
	}{
		{name: "deep-chain-128", layers: 128, width: 1},
		{name: "layered-dense-24x5", layers: 24, width: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := readinessScaleProject(t, tc.layers, tc.width)
			claimCount := tc.layers * tc.width
			factCount := readinessScaleFacts(tc.layers, tc.width)

			ctx := browserContext(t)
			runCDP(t, ctx,
				chromedp.Navigate(p.renderStatic()+"#widget.contract.l000-n00"),
			)
			openReadinessPanel(t, ctx, "#widget\\.contract\\.l000-n00 details.claim-readiness-door")
			pollTrue(t, ctx, `document.readyState === 'complete' && performance.getEntriesByType('navigation')[0].loadEventEnd > 0`)

			before := readReadinessScaleMetrics(t, ctx)
			if before.LoadMS < 0 || before.LoadMS > readinessScaleMaxLoadMS {
				t.Fatalf("viewer load %.0fms exceeded %dms budget", before.LoadMS, readinessScaleMaxLoadMS)
			}
			if before.DOMNodes > readinessScaleMaxDOMNodes {
				t.Fatalf("mounted DOM has %d nodes, exceeded %d-node budget", before.DOMNodes, readinessScaleMaxDOMNodes)
			}
			if before.JSHeapBytes < 0 || before.JSHeapBytes > readinessScaleMaxJSHeapBytes {
				t.Fatalf("mounted JS heap %.0f bytes outside required 0..%d budget", before.JSHeapBytes, readinessScaleMaxJSHeapBytes)
			}
			if before.ReadinessDoors != claimCount {
				t.Fatalf("readiness doors = %d, want every one of %d claims", before.ReadinessDoors, claimCount)
			}
			if before.RawFacts != factCount {
				t.Fatalf("raw diagnostics have %d facts, want all %d", before.RawFacts, factCount)
			}
			if before.RawPanels+before.ReadyChips != claimCount {
				t.Fatalf("raw panels %d + ready chips %d != %d claims (ready claims no longer keep a raw panel)",
					before.RawPanels, before.ReadyChips, claimCount)
			}
			if before.MapElements != 0 {
				t.Fatalf("06 §R09.9: found %d .claim-readiness-map/-trace/-route element(s), want zero — the inline dependency map is retired", before.MapElements)
			}

			// Root (l000-n00) is the deepest node and so carries the most
			// transitive facts of any claim in the fixture. Its own scope
			// count is the authoritative total FOR THIS ONE CLAIM — always
			// correct even though only READINESS_VISIBLE_CAP rows are in
			// the DOM at load.
			root := `document.getElementById('widget.contract.l000-n00').querySelector('.claim-readiness')`
			scopeText := evalString(t, ctx, root+`.querySelector('.claim-readiness-scope').textContent`)
			rootFacts, rootModules := scopeCounts(t, scopeText)
			if rootModules != 1 {
				t.Fatalf("root scope modules = %d, want 1 (fixture uses a single module)", rootModules)
			}
			if rootFacts <= 0 {
				t.Fatalf("root scope facts = %d, want at least one", rootFacts)
			}
			if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got > 2 {
				t.Fatalf("initial visible blocker rows = %d, want the within-module cap of 2 or fewer", got)
			}

			// "Show N more" must reveal EVERY remaining fact — the cap
			// defers rendering, it never drops data (06/07's standing
			// "nothing is deleted").
			runCDP(t, ctx, chromedp.Evaluate(root+`.querySelector('.claim-readiness-more') && `+root+`.querySelector('.claim-readiness-more').click()`, nil))
			pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length === `+strconv.Itoa(rootFacts))

			after := readReadinessScaleMetrics(t, ctx)
			if after.JSHeapBytes < 0 || after.JSHeapBytes > readinessScaleMaxJSHeapBytes {
				t.Fatalf("post-expand JS heap %.0f bytes outside required 0..%d budget", after.JSHeapBytes, readinessScaleMaxJSHeapBytes)
			}

			t.Logf("claims=%d facts=%d rootFacts=%d load=%.0fms domNodesBefore=%d domNodesAfter=%d heapBefore=%.1fMiB heapAfter=%.1fMiB",
				claimCount, factCount, rootFacts, before.LoadMS, before.DOMNodes, after.DOMNodes,
				before.JSHeapBytes/(1024*1024), after.JSHeapBytes/(1024*1024))
		})
	}
}

const readinessRootYAML = `id: widget.contract.root
facet: contract
module: widget
status: draft
body: |
  the claim whose readiness a reviewer is deciding.
rests_on:
  - widget.contract.alpha
  - widget.contract.beta
`

const readinessAlphaYAML = `id: widget.contract.alpha
facet: contract
module: widget
status: draft
body: |
  a direct prerequisite awaiting approval.
rests_on:
  none: true
  reason: viewer-test fixture, not backed by any doctrine claim
`

const readinessBetaYAML = `id: widget.contract.beta
facet: contract
module: widget
status: draft
body: |
  a direct prerequisite with an upstream prerequisite.
rests_on:
  - widget.contract.gamma
`

const readinessGammaYAML = `id: widget.contract.gamma
facet: contract
module: widget
status: draft
body: |
  an upstream prerequisite awaiting approval.
rests_on:
  none: true
  reason: viewer-test fixture, not backed by any doctrine claim
`

func newReadinessProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, defaultConfigYAML)
	p.writeClaim("root.yaml", readinessRootYAML)
	p.writeClaim("alpha.yaml", readinessAlphaYAML)
	p.writeClaim("beta.yaml", readinessBetaYAML)
	p.writeClaim("gamma.yaml", readinessGammaYAML)
	return p
}

// TestReadinessShowMoreRevealsTheAuthoritativeList re-pins this file's
// original TestReadinessMapCapNeverCapsTheAuthoritativeList. That test
// proved the old focused Mermaid trace's 12-node cap never shrank the
// COMPLETE list drawn beside it; 06 §R09.9 ("no inline dependency map")
// removed the trace entirely, so there is no longer a map to keep honest.
// The list itself now carries its own display cap (06 §4.4's within-module
// "Show N more", 2 rows up front) — this test re-points the same guarantee
// at THAT cap: clicking through must still reach every one of the 14
// authoritative facts, never fewer.
func TestReadinessShowMoreRevealsTheAuthoritativeList(t *testing.T) {
	p := newProjectRaw(t, defaultConfigYAML)
	var leafIDs []string
	for i := 0; i < 13; i++ {
		id := fmt.Sprintf("widget.contract.leaf-%02d", i)
		leafIDs = append(leafIDs, id)
		p.writeClaim(fmt.Sprintf("leaf-%02d.yaml", i), fmt.Sprintf(`id: %s
facet: contract
module: widget
status: draft
body: |
  an upstream prerequisite awaiting approval.
rests_on:
  none: true
  reason: viewer-test fixture, not backed by any doctrine claim
`, id))
	}
	p.writeClaim("hub.yaml", `id: widget.contract.hub
facet: contract
module: widget
status: draft
body: |
  the single first-hop route to many upstream blockers.
rests_on:
  - `+strings.Join(leafIDs, "\n  - ")+"\n")
	p.writeClaim("root.yaml", `id: widget.contract.root
facet: contract
module: widget
status: draft
body: |
  a root with one grouped module.
rests_on:
  - widget.contract.hub
`)

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
	)
	openReadinessPanel(t, ctx, "#widget\\.contract\\.root details.claim-readiness-door")
	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if got, want := evalString(t, ctx, root+`.querySelector('.claim-readiness-scope').textContent.trim()`), "14 across 1 module"; got != want {
		t.Fatalf("scope text = %q, want %q", got, want)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-module').length`); got != 1 {
		t.Fatalf("module groups = %d, want one (every claim shares module widget)", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 2 {
		t.Fatalf("visible blocker rows before expanding = %d, want the cap of 2", got)
	}
	if got, want := evalString(t, ctx, root+`.querySelector('.claim-readiness-more').textContent.trim()`), "Show 12 more in this module"; !strings.Contains(got, "12 more") {
		t.Fatalf("'Show N more' control reads %q, want it to say 12 more (14 total minus the 2 shown; %q)", got, want)
	}
	runCDP(t, ctx, chromedp.Evaluate(root+`.querySelector('.claim-readiness-more').click()`, nil))
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length === 14`)
	if evalBool(t, ctx, root+`.querySelector('.claim-readiness-more')`) {
		t.Fatal("'Show more' control must remove itself once every fact is shown")
	}
}

// TestStaticReadinessGroupsFactsByModuleAndPreservesEveryID re-pins this
// file's original TestStaticReadinessGroupsCompleteFactsAndRendersFocused-
// MapOnDemand: grouping moved from "the representative-route claim" to "the
// module that owns the fix" (06 §7.6), so alpha/beta/gamma — which all share
// module widget in this fixture — now collapse into ONE module group
// instead of two per-dependency route groups, and R09.9 removed the focused
// Mermaid trace this test used to open. What survives from the original
// test's intent: every id is preserved verbatim, nothing is silently capped,
// hostile/raw content stays text-safe, and the panel never causes horizontal
// overflow at 390px.
func TestStaticReadinessGroupsFactsByModuleAndPreservesEveryID(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
	)
	openReadinessPanel(t, ctx, "#widget\\.contract\\.root details.claim-readiness-door")

	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if !evalBool(t, ctx, `(function(){
		var items = Array.from(document.querySelectorAll('.facet-toc__item'));
		if (items.length < 3) { return false; }
		return items.slice(0, 3).every(function(item){
			var claim = document.getElementById(item.dataset.claimTarget);
			var door = claim && claim.querySelector('[data-readiness-fact-count]');
			var count = item.querySelector('.facet-toc__blocker-count');
			return door && count && Number(count.textContent || 0) === Number(door.dataset.readinessFactCount);
		});
	})()`) {
		t.Fatal("the first three facet-rail counts must match each claim's authoritative readiness fact total")
	}
	if got, want := evalString(t, ctx, `document.getElementById('widget.contract.root').querySelector('.claim-readiness-door').getAttribute('data-readiness-state')`), "Dependencies not ready"; got != want {
		t.Fatalf("readiness state = %q, want %q", got, want)
	}
	if got, want := evalString(t, ctx, root+`.querySelector('.claim-readiness-scope').textContent.trim()`), "3 across 1 module"; got != want {
		t.Fatalf("scope text = %q, want %q (alpha, beta and gamma all share module widget)", got, want)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-module').length`); got != 1 {
		t.Fatalf("module groups = %d, want 1", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-local li').length`); got != 1 {
		t.Fatalf("local approval reasons = %d, want the alias displayed once", got)
	}
	// Only 2 of the 3 facts render up front (the within-module cap); reveal
	// the rest before checking every id is present.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){ var m = `+root+`.querySelector('.claim-readiness-more'); if (m) { m.click(); } })()`, nil))
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length === 3`)
	if !evalBool(t, ctx, root+`.textContent.includes('widget.contract.alpha') && `+root+`.textContent.includes('widget.contract.beta') && `+root+`.textContent.includes('widget.contract.gamma')`) {
		t.Fatal("grouped view must preserve the exact ids of direct and upstream blockers")
	}
	// The upstream fact's dependency PATH is exactly two slugs — this
	// claim's own id and gamma's, never the intermediate beta hop (06 §8
	// item 11: a representative path must never grow the breadcrumb beyond
	// source/target).
	if !evalBool(t, ctx, root+`.textContent.includes('widget.contract.root') && `+root+`.textContent.includes('widget.contract.gamma')`) {
		t.Fatal("the gamma blocker row must show its own claim-id-to-target path")
	}

	// Raw diagnostics are the lossless maintainer surface, not a replacement
	// for the reviewer-facing list.
	if !evalBool(t, ctx, root+`.querySelector('.claim-readiness-raw pre').textContent.includes('"kind": "dependency_unapproved"')`) {
		t.Fatal("raw diagnostics must preserve the exact engine condition kind")
	}
	if !evalBool(t, ctx, `document.body.scrollWidth <= window.innerWidth + 1`) {
		t.Fatal("the grouped readiness component must not create page-level horizontal overflow")
	}
	runCDP(t, ctx, chromedp.EmulateViewport(390, 844, chromedp.EmulateScale(1)))
	pollTrue(t, ctx, `window.innerWidth === 390`)
	if !evalBool(t, ctx, `document.documentElement.scrollWidth <= window.innerWidth + 1`) {
		t.Fatal("the open readiness module must not create horizontal overflow at 390px")
	}
}

func TestReadinessTreatsFlagDetailsAsText(t *testing.T) {
	p := newProject(t)
	p.run("claim", "lock", testClaimID, "--reason", "viewer-test fixture")
	const hostile = `<img src=x onerror="window.__readinessInjected=true">`
	p.run("claim", "flag", testClaimID,
		"--claim-says", "a claim under review.",
		"--now-does", "a corrected claim.",
		"--reason", hostile,
	)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#"+testClaimID),
	)
	openReadinessPanel(t, ctx, "details.claim-readiness-door")
	if evalBool(t, ctx, `window.__readinessInjected === true || !!document.querySelector('.claim-readiness img')`) {
		t.Fatal("authored readiness detail became markup instead of text")
	}
	if !evalBool(t, ctx, `document.querySelector('.claim-readiness').textContent.includes(`+fmt.Sprintf("%q", hostile)+`)`) {
		t.Fatal("escaped readiness detail was dropped instead of shown to the reviewer")
	}
}

// TestLiveReadinessRefreshesAfterAnUpstreamApproval re-pins the same test's
// original Mermaid-error assertion away: R09.9 removed the inline trace, so
// there is no `window.__boErrors` global for a project with no locked Build
// order (build-order-ui.js is no longer injected at all — see render.go's
// HasReadinessMaps field comment). The live-refresh guarantee itself is
// unchanged: an upstream approval must drop the resolved fact from the
// COMPLETE list on the next poll, not just from whichever page happened to
// be open.
func TestLiveReadinessRefreshesAfterAnUpstreamApproval(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	base := p.ensureServe()
	runCDP(t, ctx,
		chromedp.Navigate(base+"/#widget.contract.root"),
	)
	openReadinessPanel(t, ctx, "#widget\\.contract\\.root details.claim-readiness-door")
	pollTrue(t, ctx, `document.body.classList.contains('comments-sse-open')`)
	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	runCDP(t, ctx, chromedp.Evaluate(`(function(){ var m = `+root+`.querySelector('.claim-readiness-more'); if (m) { m.click(); } })()`, nil))
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 3 {
		t.Fatalf("initial live blocker facts = %d, want 3", got)
	}

	p.run("claim", "lock", "widget.contract.gamma", "--reason", "viewer-test fixture")
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length === 2`)
	if evalBool(t, ctx, root+`.textContent.includes('widget.contract.gamma')`) {
		t.Fatal("live readiness kept the approved upstream blocker after the served fragment refreshed")
	}
}

// TestReadinessDoorJoinsTheFooterDisclosureGroup verifies the new door/panel
// architecture 06 §4.3/R09.1-R09.3 requires: the readiness door is a native
// <details name="claim-footer-<id>"> sharing its group with the
// relationships/sources doors components.EdgesHTMLWithLinks renders, and
// opening a sibling door closes it right back — "exactly one expansion open
// at a time" (R09.2). It starts CLOSED here: this fixture raises a
// facet-level banner, and screens/02 section 9.1 suppresses R09.3's auto-open
// while that banner shows.
func TestReadinessDoorJoinsTheFooterDisclosureGroup(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness-door", chromedp.ByQuery),
	)
	card := `document.getElementById('widget.contract.root')`
	// This fixture's blocked claims raise a facet-level banner, and
	// screens/02 section 9.1 suppresses R09.3's auto-open whenever that banner
	// is showing — "blocked" alone is not a trigger at facet scale. R09.3
	// binds the single-claim boards (05, 06, 06a, 07, 07a), not 02, so on this
	// surface the door starts CLOSED. The reader opens it below, which is what
	// the rest of this test is actually about: the door's membership of the
	// footer's one-at-a-time group.
	if evalBool(t, ctx, card+`.querySelector('.claim-readiness-door').open`) {
		t.Fatal("with a facet banner showing, the readiness door must start closed (screens/02 section 9.1)")
	}
	runCDP(t, ctx, chromedp.Evaluate(card+`.querySelector('.claim-readiness-door summary').click()`, nil))
	pollTrue(t, ctx, card+`.querySelector('.claim-readiness-door').open === true`)
	if !evalBool(t, ctx, `(function(){
		var door = `+card+`.querySelector('.claim-readiness-door');
		var links = `+card+`.querySelector('.claim-links');
		return door.getAttribute('name') !== '' && door.getAttribute('name') === links.getAttribute('name');
	})()`) {
		t.Fatal("the readiness door must share its name group with the relationships door")
	}
	runCDP(t, ctx, chromedp.Evaluate(card+`.querySelector('.claim-links summary').click()`, nil))
	pollTrue(t, ctx, card+`.querySelector('.claim-links').open === true`)
	if evalBool(t, ctx, card+`.querySelector('.claim-readiness-door').open`) {
		t.Fatal("opening the relationships door must close readiness (R09.2: exactly one door open at a time)")
	}
}
