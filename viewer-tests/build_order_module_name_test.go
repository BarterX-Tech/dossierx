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
modules:
  - build-order
  - gamma
claims_dir: claims
`

// gammaSVGs is the number of non-empty phases in gamma's order: schema (1
// claim) and behavior (1).
const gammaSVGs = 2

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
		p.run("build-order", "propose", "--module", "gamma")
		p.run("build-order", "lock", "--module", "gamma", "--reason", "viewer-test fixture")
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

	// Exactly one element per id: the module's own, and the tab's own.
	if n := evalInt(t, ctx, `document.querySelectorAll('#build-order').length`); n != 1 {
		t.Fatalf("elements with id=build-order: %d, want exactly 1 (the module's section)", n)
	}
	if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order').length`); n != 1 {
		t.Fatalf("elements with id=dossierx-build-order: %d, want exactly 1 (the tab's section)", n)
	}

	// The sidebar's Build order entry shows the diagrams, and ONLY them.
	openBuildOrderTab(t, ctx)
	waitDiagrams(t, ctx, "gamma", gammaSVGs)
	if got := evalString(t, ctx, visibleSectionsExpr); got != "dossierx-build-order|module-section build-order-section" {
		t.Fatalf("visible sections after clicking Build order = %q, want the tab's section alone", got)
	}
	if evalBool(t, ctx, `!document.getElementById('build-order').hidden`) {
		t.Fatal("the build-order MODULE's section is visible under the Build order tab")
	}

	// The module's own sidebar entry still shows the module's cards, and
	// hides the tab.
	runCDP(t, ctx, chromedp.Click(`.sec-tab[data-target="#build-order"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `!document.getElementById('build-order').hidden && document.getElementById('dossierx-build-order').hidden`)
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
		name        string
		lockGamma   bool
		wantSection bool
	}{
		{"locked order", true, true},
		{"no order", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newCollidingProject(t, tc.lockGamma)
			ctx := serveAndOpenLive(t, p)
			pe := watchPageErrors(t, ctx)
			desktopViewport(t, ctx)
			if got := evalBool(t, ctx, `!!document.querySelector('.build-order-section')`); got != tc.wantSection {
				t.Fatalf(".build-order-section present = %v, want %v", got, tc.wantSection)
			}
			if tc.lockGamma {
				openBuildOrderTab(t, ctx)
				waitDiagrams(t, ctx, "gamma", gammaSVGs)
			}
			runCDP(t, ctx, chromedp.Evaluate(`window.__boMarker = true;`, nil))

			// An external edit in the build-order module -> "changed" -> a swap.
			p.writeClaim("bo-extra.yaml", boClaim("build-order.contract.extra", "contract", "build-order", "orientation"))
			pollTrue(t, ctx, `!!document.getElementById('build-order.contract.extra')`)
			if !evalBool(t, ctx, `window.__boMarker === true`) {
				t.Fatal("the marker is gone: the swap became a full page reload")
			}
			if tc.lockGamma {
				waitDiagrams(t, ctx, "gamma", gammaSVGs)
				if !evalBool(t, ctx, `!document.getElementById('dossierx-build-order').hidden && document.getElementById('build-order').hidden`) {
					t.Fatalf("after the swap the visible sections are %q, want the tab alone", evalString(t, ctx, visibleSectionsExpr))
				}
				if n := evalInt(t, ctx, `document.querySelectorAll('#dossierx-build-order .bo-error').length`); n != 0 {
					t.Fatalf(".bo-error count after the swap = %d", n)
				}
			}
			assertNoPageErrors(t, ctx, pe)
		})
	}
}
