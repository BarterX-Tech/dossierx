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
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
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

// buildOrderPhaseData is one .bo-phase block as build-order.html sees it.
type buildOrderPhaseData struct {
	ModuleID   string
	Module     string
	Name       string
	Number     int
	NumLabel   string
	Definition string
	Count      int
	Levels     int
	Locked     int
	Counts     string
	Mermaid    string
	Cross      []buildOrderCross
	Excluded   []string // the excluded block's ids
	IsExcluded bool
	// ExcludedDeps are the block's rests_on targets that are excluded claims.
	ExcludedDeps []string
}

type buildOrderCross struct {
	Module string
	IDs    []string
}

type buildOrderModuleData struct {
	ID     string
	Module string
	Label  string
	Phases []buildOrderPhaseData
}

// ---- the JSON payload ----

type buildOrderPayload struct {
	GeneratedAt string                    `json:"generated_at"`
	Phases      []buildOrderPayloadPhase  `json:"phases"`
	Modules     []buildOrderPayloadModule `json:"modules"`
}

type buildOrderPayloadPhase struct {
	ID         string `json:"id"`
	Number     int    `json:"number"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

type buildOrderPayloadModule struct {
	ID         string                            `json:"id"`
	Module     string                            `json:"module"`
	Label      string                            `json:"label"`
	LockedAt   string                            `json:"locked_at"`
	Stale      bool                              `json:"stale"`
	Artifact   *buildorder.Artifact              `json:"artifact"`
	Claims     map[string]buildOrderPayloadClaim `json:"claims"`
	PhaseViews []buildOrderPayloadView           `json:"phase_views"`
	NodeIDs    map[string]string                 `json:"node_ids"`
}

// buildOrderPayloadClaim is the per-claim presentation fact the artifact
// does not carry, for every claim of the module's artifact that the catalog
// STILL holds. A claim gone from the catalog has no entry, which is what the
// client's click handler reads as "no longer in the catalog".
type buildOrderPayloadClaim struct {
	Facet  string `json:"facet"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Phase  string `json:"phase"`
	Level  int    `json:"level"`
}

type buildOrderPayloadView struct {
	Phase        string              `json:"phase"`
	Number       int                 `json:"number"`
	Definition   string              `json:"definition"`
	Claims       []string            `json:"claims"`
	Levels       [][]string          `json:"levels"`
	Ghosts       []buildorder.Ghost  `json:"ghosts"`
	CrossModule  map[string][]string `json:"cross_module"`
	ExcludedDeps []string            `json:"excluded_deps"`
	Locked       int                 `json:"locked"`
}

// buildOrderViewClaims gives Views the standalone claim facts a locked
// artifact still carries when a claim file has since been deleted. The
// synthetic entries are intentionally presentation-only: buildOrderTabData's
// byID map remains the current catalog, so the payload has no fabricated claim
// card to navigate to and the client can show its honest missing-catalog path.
func buildOrderViewClaims(artifact *buildorder.Artifact, claims []model.Claim) []model.Claim {
	if artifact == nil {
		return claims
	}
	viewClaims := append([]model.Claim(nil), claims...)
	known := make(map[string]bool, len(claims))
	for _, c := range claims {
		known[c.ID] = true
	}
	add := func(id string, restsOn []string) {
		if id == "" || known[id] {
			return
		}
		viewClaims = append(viewClaims, model.Claim{
			ID:      id,
			Module:  artifact.Module,
			RestsOn: append([]string(nil), restsOn...),
			Status:  model.StatusDraft,
		})
		known[id] = true
	}
	for _, phase := range artifact.Phases {
		for _, claim := range phase.Claims {
			add(claim.ID, claim.RestsOn)
		}
	}
	for _, id := range artifact.Excluded {
		add(id, nil)
	}
	return viewClaims
}

// buildOrderTabData loads every module's artifact, keeps the locked ones,
// computes their PhaseViews, executes tmpl per module and marshals the
// payload. Modules are visited in cfg.Modules order (a claim's module that
// the config does not declare has no artifact path of its own and is not
// visited). A nil cfg is a project with no modules: an empty tab.
func buildOrderTabData(cat *catalog.Catalog, cfg *config.Config, tmpl *template.Template, generatedAt time.Time) (BuildOrderTab, template.JS, error) {
	var tab BuildOrderTab
	if cfg == nil || tmpl == nil {
		return tab, "", nil
	}
	if cat == nil {
		cat = &catalog.Catalog{}
	}

	payload := buildOrderPayload{GeneratedAt: generatedAt.UTC().Format(time.RFC3339)}
	for _, phase := range buildorder.Phases {
		payload.Phases = append(payload.Phases, buildOrderPayloadPhase{
			ID: string(phase), Number: buildorder.PhaseNumber(phase), Name: string(phase), Definition: buildorder.PhaseDefinition(phase),
		})
	}
	payload.Phases = append(payload.Phases, buildOrderPayloadPhase{
		ID: string(model.BuildRoleOutOfScope), Number: 0, Name: buildorder.ExcludedPhaseName, Definition: buildorder.PhaseDefinition(model.BuildRoleOutOfScope),
	})
	payload.Modules = []buildOrderPayloadModule{}

	byID := make(map[string]model.Claim, len(cat.Claims))
	for _, c := range cat.Claims {
		byID[c.ID] = c
	}

	for _, module := range cfg.Modules {
		artifact, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
		if err != nil || !artifact.Locked {
			continue // the skip half of the policy above
		}
		views, nodeIDs, err := buildorder.Views(artifact, buildOrderViewClaims(artifact, cat.Claims))
		if err != nil {
			continue // the skip half of the policy above; BuildOrderWarnings names it
		}

		id := slugify(module)
		label := components.DisplayCase(module)
		data := buildOrderModuleData{ID: id, Module: module, Label: label}
		pm := buildOrderPayloadModule{
			ID: id, Module: module, Label: label, LockedAt: artifact.LockedAt, Stale: artifact.Stale,
			Artifact: artifact, Claims: map[string]buildOrderPayloadClaim{}, PhaseViews: []buildOrderPayloadView{}, NodeIDs: nodeIDs,
		}
		for _, v := range views {
			pd := buildOrderPhaseData{
				ModuleID: id, Module: module, Name: v.Name, Number: v.Number, Definition: v.Definition,
				Count: v.Count(), Levels: len(v.Levels), Locked: v.Locked, Counts: v.Counts(),
				ExcludedDeps: v.ExcludedDeps, IsExcluded: v.Number == 0,
			}
			if pd.IsExcluded {
				pd.NumLabel = buildorder.ExcludedPhaseName
				for _, c := range v.Claims {
					pd.Excluded = append(pd.Excluded, c.ID)
				}
			} else {
				pd.NumLabel = fmt.Sprintf("phase %d of %d", v.Number, len(buildorder.Phases))
				pd.Mermaid = buildorder.Mermaid(v, buildorder.MermaidOptions{Palette: buildorder.PaletteCSS})
			}
			for _, m := range v.CrossModuleNames() {
				pd.Cross = append(pd.Cross, buildOrderCross{Module: m, IDs: v.CrossModule[m]})
			}
			data.Phases = append(data.Phases, pd)

			pv := buildOrderPayloadView{
				Phase: v.Name, Number: v.Number, Definition: v.Definition, Claims: []string{},
				Levels: v.Levels, Ghosts: v.Ghosts, CrossModule: v.CrossModule, ExcludedDeps: v.ExcludedDeps, Locked: v.Locked,
			}
			levelOf := map[string]int{}
			for i, level := range v.Levels {
				for _, cid := range level {
					levelOf[cid] = i
				}
			}
			for _, c := range v.Claims {
				pv.Claims = append(pv.Claims, c.ID)
				cc, ok := byID[c.ID]
				if !ok {
					continue
				}
				pm.Claims[c.ID] = buildOrderPayloadClaim{
					Facet: cc.Facet, Label: components.ClaimLabel(c.ID), Status: string(cc.Status), Phase: v.Name, Level: levelOf[c.ID],
				}
			}
			pm.PhaseViews = append(pm.PhaseViews, pv)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return BuildOrderTab{}, "", fmt.Errorf("render: execute %s for module %q: %w", buildOrderFileName, module, err)
		}
		tab.Modules = append(tab.Modules, BuildOrderModule{ID: id, Module: module, Label: label, HTML: template.HTML(buf.String())})
		payload.Modules = append(payload.Modules, pm)
	}
	if len(tab.Modules) == 0 {
		return BuildOrderTab{}, "", nil
	}
	tab.FirstModuleID = tab.Modules[0].ID

	// encoding/json's DEFAULT HTML escaping is the whole guard between an
	// author-authored id and a </script> breakout in the JSON block; the
	// bytes reach template.JS verbatim (see shellData.GraphPayload).
	b, err := json.Marshal(payload)
	if err != nil {
		return BuildOrderTab{}, "", fmt.Errorf("render: encode build-order payload: %w", err)
	}
	return tab, template.JS(b), nil
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
func BuildOrderWarnings(cfg *config.Config, claims []model.Claim) []string {
	if cfg == nil {
		return nil
	}
	var out []string
	for _, module := range cfg.Modules {
		artifact, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
		if err != nil || !artifact.Locked {
			continue
		}
		if _, _, err := buildorder.Views(artifact, buildOrderViewClaims(artifact, claims)); err != nil {
			out = append(out, fmt.Sprintf("build order for module %q is not drawn in the viewer's Build order tab: %v; re-propose and lock it (dossierx build-order propose --module %s)", module, err, module))
		}
	}
	return out
}

// StyleOverrideWarnings returns the one warning "dossierx check" carries when
// a project overrides style.css AND has at least one locked build order: the
// Build order tab's node colours, its overflow rules and its sticky module
// strip all live in the engine's style.css (the .bo-* rules), and an override
// sheet that predates the tab supplies none of them, so the reader gets
// mermaid's base-theme lavender nodes and a page that scrolls sideways with
// nothing said. Render has no warnings channel, which is why check.Run asks
// here. Nil when either condition does not hold.
func StyleOverrideWarnings(cfg *config.Config) []string {
	if cfg == nil || cfg.Viewer.TemplateOverrides == "" {
		return nil
	}
	_, found, err := components.OverrideFile(cfg.Viewer.TemplateOverrides, styleFileName)
	if err != nil || !found {
		return nil
	}
	for _, module := range cfg.Modules {
		a, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
		if err == nil && a.Locked {
			return []string{"viewer.template_overrides/style.css is in force: the Build order tab's diagram colours and overflow rules come from the engine's style.css and are not supplied by the override; copy the .bo-* rules into it"}
		}
	}
	return nil
}
