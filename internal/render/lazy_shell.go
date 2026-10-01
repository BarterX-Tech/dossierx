package render

import (
	"html/template"
	"sync"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// buildEagerShellData is the embedded shell's audited path. Every projection
// below is emitted by that shell, so one shared output-derived budget remains
// an honest early lower bound before the final exact writer.
func buildEagerShellData(in shellInputs, partials map[model.Layout]*template.Template, budget *renderByteBudget) (shellData, error) {
	renderedByID, err := renderClaimsWithBudget(in.cat, partials, in.cat.Conformance, budget)
	if err != nil {
		return shellData{}, err
	}
	in.renderedByID = renderedByID

	graphPayload, err := graphPayloadJSONWithBudget(in.cat, in.cfg, in.generatedAt, budget)
	if err != nil {
		return shellData{}, err
	}
	in.graphPayload = graphPayload

	rendered := renderBriefs(in.briefs, in.cat, in.cfg)
	briefsPayload, err := briefsPayloadJSON(in.briefs, rendered, in.briefReview, in.briefsBudgetOr(budget))
	if err != nil {
		return shellData{}, err
	}
	in.briefsPayload = briefsPayload

	data := buildShellStaticData(in)
	data.ModuleGroups = buildModuleGroups(buildGroups(in.cat, in.cfg, renderedByID))
	data.Home = buildHomeView(in.cat, in.cfg, data.ModuleGroups, homeBriefThreads(in.briefs, rendered))
	data.Briefs = buildBriefsView(in.briefs, rendered, in.cat, in.briefReview)
	return data, nil
}

// lazyShellData preserves the historical `.Field` syntax of custom shell
// templates while shadowing the expensive promoted fields with zero-arg
// methods. html/template invokes those methods only when execution reaches the
// action, so a static shell or a dead conditional branch computes nothing.
type lazyShellData struct {
	shellData
	projection *lazyShellProjection
}

func newLazyShellData(in shellInputs, partials map[model.Layout]*template.Template, budget *renderByteBudget) *lazyShellData {
	return &lazyShellData{
		shellData:  buildShellStaticData(in),
		projection: &lazyShellProjection{in: in, partials: partials, budget: budget},
	}
}

func (d *lazyShellData) ModuleGroups() ([]ModuleGroup, error) {
	return d.projection.moduleGroups()
}

func (d *lazyShellData) Home() (HomeView, error) {
	return d.projection.home()
}

// Briefs is the brief tree and pages (NIT-197), built only if a project shell
// references them.
func (d *lazyShellData) Briefs() (BriefsView, error) {
	return d.projection.briefsView(), nil
}

func (d *lazyShellData) GraphPayload() (template.JS, error) {
	return d.projection.graphPayloadJSON()
}

// BriefsPayload is computed only if a project shell references it, like every
// other corpus-sized projection here.
func (d *lazyShellData) BriefsPayload() (template.JS, error) {
	return d.projection.briefsPayloadJSON()
}

type lazyShellProjection struct {
	in       shellInputs
	partials map[model.Layout]*template.Template
	budget   *renderByteBudget

	claimsOnce sync.Once
	claims     map[string]template.HTML
	claimsErr  error

	groupsOnce sync.Once
	groups     []ModuleGroup
	groupsErr  error

	graphOnce sync.Once
	graph     template.JS
	graphErr  error

	briefsOnce sync.Once
	briefs     template.JS
	briefsErr  error

	homeOnce sync.Once
	homeView HomeView
	homeErr  error

	renderedOnce sync.Once
	rendered     map[string]renderedBrief

	briefsViewOnce sync.Once
	briefsViewVal  BriefsView
}

// renderedBriefs renders every brief body once, shared by the payload and the
// pages.
func (p *lazyShellProjection) renderedBriefs() map[string]renderedBrief {
	p.renderedOnce.Do(func() {
		p.rendered = renderBriefs(p.in.briefs, p.in.cat, p.in.cfg)
	})
	return p.rendered
}

func (p *lazyShellProjection) briefsView() BriefsView {
	p.briefsViewOnce.Do(func() {
		p.briefsViewVal = buildBriefsView(p.in.briefs, p.renderedBriefs(), p.in.cat, p.in.briefReview)
	})
	return p.briefsViewVal
}

// home builds the Home projection once per render: a shell references .Home
// several times, and each build reads constitution.yaml and the lock store.
func (p *lazyShellProjection) home() (HomeView, error) {
	p.homeOnce.Do(func() {
		groups, err := p.moduleGroups()
		if err != nil {
			p.homeErr = err
			return
		}
		p.homeView = buildHomeView(p.in.cat, p.in.cfg, groups, homeBriefThreads(p.in.briefs, p.renderedBriefs()))
	})
	return p.homeView, p.homeErr
}

func (p *lazyShellProjection) renderedClaims() (map[string]template.HTML, error) {
	p.claimsOnce.Do(func() {
		p.claims, p.claimsErr = renderClaimsWithBudget(p.in.cat, p.partials, p.in.cat.Conformance, p.budget)
	})
	return p.claims, p.claimsErr
}

func (p *lazyShellProjection) moduleGroups() ([]ModuleGroup, error) {
	p.groupsOnce.Do(func() {
		rendered, err := p.renderedClaims()
		if err != nil {
			p.groupsErr = err
			return
		}
		p.groups = buildModuleGroups(buildGroups(p.in.cat, p.in.cfg, rendered))
	})
	return p.groups, p.groupsErr
}

func (p *lazyShellProjection) graphPayloadJSON() (template.JS, error) {
	p.graphOnce.Do(func() {
		p.graph, p.graphErr = graphPayloadJSONWithBudget(p.in.cat, p.in.cfg, p.in.generatedAt, p.budget)
	})
	return p.graph, p.graphErr
}

func (p *lazyShellProjection) briefsPayloadJSON() (template.JS, error) {
	p.briefsOnce.Do(func() {
		p.briefs, p.briefsErr = briefsPayloadJSON(p.in.briefs, p.renderedBriefs(), p.in.briefReview, p.in.briefsBudgetOr(p.budget))
	})
	return p.briefs, p.briefsErr
}

// briefsBudgetOr is the budget the briefs payload is charged to: its own when
// the render set one (an unbounded render; see shellInputs.briefsBudget),
// otherwise the shared budget every other projection is charged to.
func (in *shellInputs) briefsBudgetOr(shared *renderByteBudget) *renderByteBudget {
	if in.briefsBudget != nil {
		return in.briefsBudget
	}
	return shared
}
