package viewertests

// A MODULE NAMED "build-order", IN A REAL BROWSER.
//
// slugify maps a module's name to its section id, so a module named
// "build-order" renders as <section class="module-section" id="build-order">.
// The retired Build order tab once owned that id too, and the viewer keyed a
// full-page-reload guard on it, so a project with that module name lost the
// module under its own tab and took a full reload on every claim edit. The
// name is an ordinary module name now: it must render under its own tab and
// reload in place, over file:// and under serve.

import (
	"testing"

	"github.com/chromedp/chromedp"
)

// buildOrderModuleConfig: gamma first, so the fresh load opens gamma and
// reaching the build-order module takes a real tab click.
const buildOrderModuleConfig = `schema_version: 1
facets:
  - contract
  - internals
modules:
  - gamma
  - build-order
claims_dir: claims
`

// newBuildOrderModuleProject writes one draft claim in each module.
func newBuildOrderModuleProject(t *testing.T) *project {
	t.Helper()
	p := newProjectRaw(t, buildOrderModuleConfig)
	p.writeClaim("gamma-schema.yaml", railClaim("gamma.contract.schema", "contract", "gamma", ""))
	p.writeClaim("bo-thing.yaml", railClaim("build-order.contract.thing", "contract", "build-order", ""))
	return p
}

// visibleSectionsExpr lists the ids and classes of every un-hidden
// .module-section, so a failure names what was on screen.
const visibleSectionsExpr = `Array.from(document.querySelectorAll('.module-section:not([hidden])')).map(function(s){return s.id+'|'+s.className;}).join(', ')`

func TestModuleNamedBuildOrderRendersUnderItsOwnTab(t *testing.T) {
	p := newBuildOrderModuleProject(t)
	url := p.renderStatic()
	ctx := browserContext(t)
	pe := watchPageErrors(t, ctx)
	runCDP(t, ctx, chromedp.Navigate(url))
	pollTrue(t, ctx, `document.readyState === 'complete'`)
	desktopViewport(t, ctx)

	if n := evalInt(t, ctx, `document.querySelectorAll('#build-order').length`); n != 1 {
		t.Fatalf("elements with id=build-order: %d, want exactly 1 (the module's section)", n)
	}
	if got := evalString(t, ctx, visibleSectionsExpr); got != "gamma|module-section" {
		t.Fatalf("visible sections on a fresh load = %q, want gamma's section alone", got)
	}

	runCDP(t, ctx, chromedp.Click(`.sec-tab[data-target="#build-order"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `!document.getElementById('build-order').hidden`)
	if got := evalString(t, ctx, visibleSectionsExpr); got != "build-order|module-section" {
		t.Fatalf("visible sections after clicking the build-order module = %q, want the module's section alone", got)
	}
	if !evalBool(t, ctx, `document.getElementById('build-order.contract.thing').getBoundingClientRect().height > 0`) {
		t.Fatal("the build-order module's claim card is not laid out under its own tab")
	}
	assertNoPageErrors(t, pe)
}

// TestReloadSwapsInPlaceWithAModuleNamedBuildOrder: under serve, a claim
// edit in the build-order module is a fragment swap, never a full reload — a
// window marker set before the edit is still there after it.
func TestReloadSwapsInPlaceWithAModuleNamedBuildOrder(t *testing.T) {
	p := newBuildOrderModuleProject(t)
	ctx := serveAndOpenLive(t, p)
	pe := watchPageErrors(t, ctx)
	desktopViewport(t, ctx)
	runCDP(t, ctx, chromedp.Click(`.sec-tab[data-target="#build-order"]`, chromedp.ByQuery))
	pollTrue(t, ctx, `!document.getElementById('build-order').hidden`)
	runCDP(t, ctx, chromedp.Evaluate(`window.__reloadMarker = true;`, nil))

	p.writeClaim("bo-extra.yaml", railClaim("build-order.contract.extra", "contract", "build-order", ""))
	pollTrue(t, ctx, `!!document.getElementById('build-order.contract.extra')`)
	if !evalBool(t, ctx, `window.__reloadMarker === true`) {
		t.Fatal("the marker is gone: the swap became a full page reload")
	}
	assertNoPageErrors(t, pe)
}
