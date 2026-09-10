package viewertests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
)

const readinessRootYAML = `id: widget.contract.root
facet: contract
module: widget
status: draft
body: |
  the claim whose readiness a reviewer is deciding.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
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
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`

const readinessBetaYAML = `id: widget.contract.beta
facet: contract
module: widget
status: draft
body: |
  a direct prerequisite with an upstream prerequisite.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - widget.contract.gamma
`

const readinessGammaYAML = `id: widget.contract.gamma
facet: contract
module: widget
status: draft
body: |
  an upstream prerequisite awaiting approval.
governed_by:
  type: none
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

func TestReadinessMapCapNeverCapsTheAuthoritativeList(t *testing.T) {
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
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
`, id))
	}
	p.writeClaim("hub.yaml", `id: widget.contract.hub
facet: contract
module: widget
status: draft
body: |
  the single first-hop route to many upstream blockers.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - `+strings.Join(leafIDs, "\n  - ")+"\n")
	p.writeClaim("root.yaml", `id: widget.contract.root
facet: contract
module: widget
status: draft
body: |
  a root with one grouped route.
governed_by:
  type: none
  reason: viewer-test fixture, not backed by any doctrine claim
rests_on:
  - widget.contract.hub
`)

	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness", chromedp.ByQuery),
	)
	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 14 {
		t.Fatalf("authoritative list has %d facts, want all 14", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-route').length`); got != 1 {
		t.Fatalf("first-hop groups = %d, want one hub route", got)
	}
	runCDP(t, ctx, chromedp.Evaluate(root+`.querySelector('.claim-readiness-trace').open = true`, nil))
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-map svg').length === 1`)
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-map .node.fact').length`); got != 12 {
		t.Fatalf("bounded map has %d fact nodes, want explicit cap of 12", got)
	}
	if !evalBool(t, ctx, root+`.querySelector('.claim-readiness-map-limit').textContent.includes('Showing 12 of 14')`) {
		t.Fatal("bounded map must disclose its 12-of-14 cap beside the complete list")
	}
}

func TestStaticReadinessGroupsCompleteFactsAndRendersFocusedMapOnDemand(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	runCDP(t, ctx,
		chromedp.Navigate(p.renderStatic()+"#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness", chromedp.ByQuery),
	)

	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if got := evalString(t, ctx, root+`.querySelector('.claim-readiness-state').textContent.trim()`); got != "Dependencies not ready" {
		t.Fatalf("readiness state = %q, want Dependencies not ready", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 3 {
		t.Fatalf("rendered blocker facts = %d, want all 3 independent conditions", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-route').length`); got != 2 {
		t.Fatalf("first-hop groups = %d, want alpha and beta", got)
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-local li').length`); got != 1 {
		t.Fatalf("local approval reasons = %d, want the alias displayed once", got)
	}
	if !evalBool(t, ctx, root+`.textContent.includes('widget.contract.alpha') && `+root+`.textContent.includes('widget.contract.beta') && `+root+`.textContent.includes('widget.contract.gamma')`) {
		t.Fatal("grouped view must preserve the exact ids of direct and upstream blockers")
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-map svg').length`); got != 0 {
		t.Fatalf("closed map disclosures rendered %d SVGs, want lazy zero", got)
	}

	// The beta route is the group with two facts: beta itself and gamma via
	// beta. Open its native disclosure, reveal the representative path, and
	// then opt into its focused Mermaid trace.
	runCDP(t, ctx, chromedp.Evaluate(`(function(){
		var routes = Array.from(`+root+`.querySelectorAll('.claim-readiness-route'));
		var beta = routes.find(function(route){ return route.querySelector('.claim-readiness-route-id small').textContent === 'widget.contract.beta'; });
		beta.open = true;
		beta.querySelector('.claim-readiness-path').open = true;
		beta.querySelector('.claim-readiness-trace').open = true;
	})()`, nil))
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-map svg').length === 1`)

	if !evalBool(t, ctx, root+`.textContent.includes('widget.contract.root → widget.contract.beta → widget.contract.gamma')`) {
		t.Fatal("upstream fact must retain its complete representative path")
	}
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-map .node.fact').length`); got != 2 {
		t.Fatalf("focused beta map fact nodes = %d, want 2", got)
	}
	if got := evalInt(t, ctx, `window.__boErrors.length`); got != 0 {
		t.Fatalf("Mermaid renderer recorded %d errors", got)
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
		t.Fatal("the open readiness route and map must keep horizontal scrolling inside the map at 390px")
	}
	if !evalBool(t, ctx, root+`.querySelector('.claim-readiness-map-scroll').scrollWidth > `+root+`.querySelector('.claim-readiness-map-scroll').clientWidth`) {
		t.Fatal("the narrow layout must retain an internally scrollable dependency map")
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
		chromedp.WaitVisible(".claim-readiness", chromedp.ByQuery),
	)
	if evalBool(t, ctx, `window.__readinessInjected === true || !!document.querySelector('.claim-readiness img')`) {
		t.Fatal("authored readiness detail became markup instead of text")
	}
	if !evalBool(t, ctx, `document.querySelector('.claim-readiness').textContent.includes(`+fmt.Sprintf("%q", hostile)+`)`) {
		t.Fatal("escaped readiness detail was dropped instead of shown to the reviewer")
	}
}

func TestLiveReadinessRefreshesAfterAnUpstreamApproval(t *testing.T) {
	p := newReadinessProject(t)
	ctx := browserContext(t)
	base := p.ensureServe()
	runCDP(t, ctx,
		chromedp.Navigate(base+"/#widget.contract.root"),
		chromedp.WaitVisible("#widget\\.contract\\.root .claim-readiness", chromedp.ByQuery),
	)
	pollTrue(t, ctx, `document.body.classList.contains('comments-sse-open')`)
	root := `document.getElementById('widget.contract.root').querySelector('.claim-readiness')`
	if got := evalInt(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length`); got != 3 {
		t.Fatalf("initial live blocker facts = %d, want 3", got)
	}

	p.run("claim", "lock", "widget.contract.gamma", "--reason", "viewer-test fixture")
	pollTrue(t, ctx, root+`.querySelectorAll('.claim-readiness-blocker').length === 2`)
	if evalBool(t, ctx, root+`.textContent.includes('widget.contract.gamma')`) {
		t.Fatal("live readiness kept the approved upstream blocker after the served fragment refreshed")
	}
	if got := evalInt(t, ctx, `window.__boErrors.length`); got != 0 {
		t.Fatalf("live refresh left %d Mermaid renderer errors", got)
	}
}
