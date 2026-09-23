package render

// build_order_view.go builds the Build order tab: one entry per module whose
// build-order artifact on disk is LOCKED, each holding the per-module HTML
// build-order.html renders (six blocks in the fixed sequence, one mermaid
// flowchart per non-empty phase) and the JSON payload the tab's client file
// reads. Everything drawn comes from internal/buildorder's ONE generator
// (Views + Mermaid), which "dossierx build-order show" also calls, so the
// page and the export are the same text apart from the classDef lines.
//
// Load-error policy — one module's artifact never costs the viewer:
//
//   - A LoadArtifact failure, or an artifact with Locked == false, SKIPS that
//     module. A build-order artifact is a generated, regenerable side file,
//     and one corrupt build/build-order/<m>.json in a 26-module project costs
//     that module's tab and nothing else — never every module's viewer.
//   - A buildorder.Views error (a hand-edited artifact whose stored edges
//     contradict the phase sequence, a phase block under a name the engine
//     does not know or stored twice, or a same-module target the artifact
//     neither places nor excludes) SKIPS that module the same way, and is
//     NOT silent: BuildOrderWarnings recomputes it for "dossierx check",
//     which carries one warnings[] line naming the module and the reason.
//     The diagram that would lie about the order is never drawn; the
//     module's tab entry is the cost, not the project's viewer. (Two claim
//     ids sanitising to one node id used to be a Views error too; the
//     allocator in internal/buildorder now gives each its own node.)
//
//     A locked artifact can outlive one of its catalog claims. Before calling
//     Views, buildOrderViewClaims supplies those artifact entries as synthetic
//     draft claims carrying only their stored id and edges. This lets the
//     diagram retain the approved node and its Mermaid scripts while the
//     payload deliberately omits the synthetic claim, so the client marks a
//     click as a missing-catalog miss instead of silently dropping the tab.
//   - A template execution error is RETURNED and fails Render, named: that
//     is a defect in this package's own template, not in a project's file.

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// legacyBuildOrderOverrideName is the file name build_order.html was
// overridable under. loadTemplates refuses a viewer.template_overrides
// directory that still carries it.
const legacyBuildOrderOverrideName = "build_order.html"

// BuildOrderTab is shellData.BuildOrders: the modules with a locked build
// order, in project module order, and the first one's id for the sidebar
// button's data-default-target.
type BuildOrderTab struct {
	Modules       []BuildOrderModule
	FirstModuleID string
}

// BuildOrderModule is one module's entry in the tab.
type BuildOrderModule struct {
	// ID is slugify(Module), the id the section carries as
	// "dossierx-build-order-<ID>" and the .bo-modules button targets.
	ID     string
	Module string
	Label  string
	// HTML is build-order.html executed for this module.
	HTML template.HTML
}

// buildOrderTabData used to load locked artifacts into the viewer tab.
// The tab is gone: leftover artifacts are not a viewer obligation.
func buildOrderTabData(cat *catalog.Catalog, cfg *config.Config, tmpl *template.Template, generatedAt time.Time) (BuildOrderTab, template.JS, error) {
	return buildOrderTabDataWithBudget(cat, cfg, tmpl, generatedAt, nil)
}

func buildOrderTabDataWithBudget(_ *catalog.Catalog, _ *config.Config, _ *template.Template, _ time.Time, _ *renderByteBudget) (BuildOrderTab, template.JS, error) {
	return BuildOrderTab{}, "", nil
}

// buildOrderSectionID is the id of the tab's own section, and the prefix
// of every per-module group inside it ("dossierx-build-order-<slug>"). The
// "dossierx-" prefix keeps them out of the id space slugify maps a module's
// name into (a module named "build-order" gets id="build-order"); slugify
// CAN still produce these for a module named "dossierx build order" or a
// module "dossierx" with a facet "build-order", which is what
// BuildOrderIDCollision refuses by name.
const buildOrderSectionID = "dossierx-build-order"

// BuildOrderIDCollision reports the first module or facet section whose id
// is the tab's own section id or carries its per-module prefix, for a
// render that emits the tab — two elements with one id would make the
// sidebar's Build order entry show that module's cards and no diagram. Nil
// when no id collides or the tab is not emitted.
func buildOrderIDCollision(tab BuildOrderTab, groups []ModuleGroup) error {
	if len(tab.Modules) == 0 {
		return nil
	}
	collides := func(id string) bool {
		return id == buildOrderSectionID || strings.HasPrefix(id, buildOrderSectionID+"-")
	}
	for _, mg := range groups {
		if collides(mg.ID) {
			return fmt.Errorf("render: module %q renders with id %q, which the Build order tab reserves for itself; rename the module", mg.Module, mg.ID)
		}
		for _, g := range mg.Facets {
			if collides(g.ID) {
				return fmt.Errorf("render: facet %q of module %q renders with id %q, which the Build order tab reserves for itself; rename one of them", g.Facet, g.Module, g.ID)
			}
		}
	}
	return nil
}

// BuildOrderWarnings returns one line per module whose LOCKED artifact
// buildOrderTabData skipped because buildorder.Views refused it, so the
// skip is on "dossierx check"'s warnings[] beside StyleOverrideWarnings
// rather than a tab entry that is silently absent. It re-runs the same
// LoadArtifact + Views the render ran (Render has no warnings channel; see
// StyleOverrideWarnings), over the same claims. An artifact that does not
// load or is not locked is not a warning here: that skip is the ordinary
// "no order yet" state. Nil when nothing was skipped.
func BuildOrderWarnings(_ *config.Config, _ []model.Claim) []string {
	return nil
}

// StyleOverrideWarnings returns the one warning "dossierx check" carries when
// a project overrides style.css AND has at least one locked build order: the
// Build order tab's node colours, its overflow rules and its sticky module
// strip all live in the engine's style.css (the .bo-* rules), and an override
// sheet that predates the tab supplies none of them, so the reader gets
// mermaid's base-theme lavender nodes and a page that scrolls sideways with
// nothing said. Render has no warnings channel, which is why check.Run asks
// here. Nil when either condition does not hold.
func StyleOverrideWarnings(_ *config.Config) []string {
	return nil
}
