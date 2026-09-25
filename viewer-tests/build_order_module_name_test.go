package viewertests

// A MODULE NAMED "build-order", IN A REAL BROWSER.
//
// slugify maps a module's name to its section id, so a module named
// "build-order" renders as <section class="module-section" id="build-order">.
// The Build order tab's own section once carried that same id: the sidebar's
// Build order entry then resolved to the MODULE's section (both stayed
// visible, the diagrams' section held zero SVGs), and applyFragment's
// zero-to-one reload guard — keyed on getElementById('build-order') —
// matched on every fragment swap, so a project with that module name (with
// or without a locked order) took a full page reload on every claim edit.
// The tab's ids now carry the "dossierx-build-order" prefix and the guard
// reads the section's class. Both are proven here, over file:// and under
// serve, with the module name that used to collide.

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// buildOrderCollidingConfig: a module NAMED build-order beside gamma, which
// carries the locked order.
const buildOrderCollidingConfig = `schema_version: 1
facets:
  - contract
  - internals
modules:
  - build-order
  - gamma
claims_dir: claims
`

// newCollidingProject writes the build-order module (one draft orientation
// claim, never ordered) and gamma (two locked claims across two phases),
// and locks gamma's order through the CLI when lockGamma is set.
func newCollidingProject(t *testing.T, lockGamma bool) *project {
	t.Helper()
	p := newProjectRaw(t, buildOrderCollidingConfig)
	p.writeClaim("bo-thing.yaml", boClaim("build-order.contract.thing", "contract", "build-order", "orientation"))
	p.writeClaim("gamma-schema.yaml", boClaim("gamma.contract.schema", "contract", "gamma", "schema"))
	p.writeClaim("gamma-behavior.yaml", boClaim("gamma.contract.behavior", "contract", "gamma", "behavior", "gamma.contract.schema"))
	if lockGamma {
		for _, id := range []string{"gamma.contract.schema", "gamma.contract.behavior"} {
			p.run("claim", "lock", id, "--reason", "viewer-test fixture")
		}
	}
	return p
}

// visibleSectionsExpr lists the ids and classes of every un-hidden
// .module-section, so a failure names what was on screen.
const visibleSectionsExpr = `Array.from(document.querySelectorAll('.module-section:not([hidden])')).map(function(s){return s.id+'|'+s.className;}).join(', ')`

func TestBuildOrderTabSurvivesAModuleNamedBuildOrder(t *testing.T) {
	p := newCollidingProject(t, true)
	ctx, pe, _ := staticBuildOrderTab(t, p)
	desktopViewport(t, ctx)

	if n := evalInt(t, ctx, `document.querySelectorAll('#build-order').length`); n != 1 {
		t.Fatalf("elements with id=build-order: %d, want exactly 1 (the module's section)", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order').length`); n != 0 {
		t.Fatalf("the retired Build order tab leaked back: %d #dossierx-build-order nodes", n)
	}

	runCDP(t, ctx, chromedp.Click(`.sec-tab[data-target="#build-order"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `!document.getElementById('build-order').hidden`)
	if got := evalString(t, ctx, visibleSectionsExpr); got != "build-order|module-section" {
		t.Fatalf("visible sections after clicking the build-order module = %q, want the module's section alone", got)
	}
	if !evalBool(t, ctx, `document.getElementById('build-order.contract.thing').getBoundingClientRect().height > 0`) {
		t.Fatal("the build-order module's claim card is not laid out under its own tab")
	}
	assertNoPageErrors(t, ctx, pe)
}

// TestReloadSwapsInPlaceWithAModuleNamedBuildOrder: under serve, a claim
// edit is a fragment swap, never a full reload, whether or not the project
// has a locked order — a window marker set before the edit is still there
// after it, and with an order the diagrams come back rendered.
func TestReloadSwapsInPlaceWithAModuleNamedBuildOrder(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lockGamma bool
	}{
		{"locked order", true},
		{"no order", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newCollidingProject(t, tc.lockGamma)
			ctx := serveAndOpenLive(t, p)
			pe := watchPageErrors(t, ctx)
			desktopViewport(t, ctx)
			if evalBool(t, ctx, `!!document.querySelector('.build-order-section')`) {
				t.Fatal(".build-order-section must stay absent")
			}
			runCDP(t, ctx, chromedp.Evaluate(`window.__boMarker = true;`, nil))

			p.writeClaim("bo-extra.yaml", boClaim("build-order.contract.extra", "contract", "build-order", "orientation"))
			pollTrue(t, ctx, `!!document.getElementById('build-order.contract.extra')`)
			if !evalBool(t, ctx, `window.__boMarker === true`) {
				t.Fatal("the marker is gone: the swap became a full page reload")
			}
			assertNoPageErrors(t, ctx, pe)
		})
	}
}
