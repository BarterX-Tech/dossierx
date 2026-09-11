package render

import (
	"fmt"
	"html/template"
	"sync"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// buildEagerShellData is the embedded shell's audited path. Every projection
// below is emitted by that shell, so one shared output-derived budget remains
// an honest early lower bound before the final exact writer.
func buildEagerShellData(in shellInputs, partials map[model.Layout]*template.Template, buildOrderTemplate *template.Template, budget *renderByteBudget) (shellData, error) {
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

	buildOrders, buildOrderPayload, err := buildOrderTabDataWithBudget(in.cat, in.cfg, buildOrderTemplate, in.generatedAt, budget)
	if err != nil {
		return shellData{}, err
	}
	in.buildOrders = buildOrders
	in.buildOrderPayload = buildOrderPayload

	data := buildShellStaticData(in)
	data.ModuleGroups = buildModuleGroups(buildGroups(in.cat, in.cfg, renderedByID))
	data.Tracks, err = buildTrackSectionsWithBudget(in.cat, in.cfg, renderedByID, budget)
	if err != nil {
		return shellData{}, fmt.Errorf("render: track sections: %w", err)
	}
	if err := buildOrderIDCollision(data.BuildOrders, data.ModuleGroups); err != nil {
		return shellData{}, err
	}
	return data, nil
}

// lazyShellData preserves the historical `.Field` syntax of custom shell
// templates while shadowing the five expensive promoted fields with zero-arg
// methods. html/template invokes those methods only when execution reaches the
// action, so a static shell or a dead conditional branch computes nothing.
type lazyShellData struct {
	shellData
	projection *lazyShellProjection
}

func newLazyShellData(in shellInputs, partials map[model.Layout]*template.Template, buildOrderTemplate *template.Template, budget *renderByteBudget) *lazyShellData {
	return &lazyShellData{
		shellData:  buildShellStaticData(in),
		projection: &lazyShellProjection{in: in, partials: partials, buildOrderTemplate: buildOrderTemplate, budget: budget},
	}
}

func (d *lazyShellData) ModuleGroups() ([]ModuleGroup, error) {
	groups, err := d.projection.moduleGroups()
	if err != nil {
		return nil, err
	}
	if err := d.projection.noteGroupsRequested(); err != nil {
		return nil, err
	}
	return groups, nil
}

func (d *lazyShellData) Tracks() ([]TrackSection, error) {
	return d.projection.trackSections()
}

func (d *lazyShellData) GraphPayload() (template.JS, error) {
	return d.projection.graphPayloadJSON()
}

func (d *lazyShellData) BuildOrders() (BuildOrderTab, error) {
	tab, _, err := d.projection.buildOrderData()
	if err != nil {
		return BuildOrderTab{}, err
	}
	if err := d.projection.noteBuildOrderTabRequested(); err != nil {
		return BuildOrderTab{}, err
	}
	return tab, nil
}

func (d *lazyShellData) BuildOrderPayload() (template.JS, error) {
	_, payload, err := d.projection.buildOrderData()
	return payload, err
}

type lazyShellProjection struct {
	in                 shellInputs
	partials           map[model.Layout]*template.Template
	buildOrderTemplate *template.Template
	budget             *renderByteBudget

	claimsOnce sync.Once
	claims     map[string]template.HTML
	claimsErr  error

	groupsOnce sync.Once
	groups     []ModuleGroup
	groupsErr  error

	tracksOnce sync.Once
	tracks     []TrackSection
	tracksErr  error

	graphOnce sync.Once
	graph     template.JS
	graphErr  error

	buildOrderOnce    sync.Once
	buildOrders       BuildOrderTab
	buildOrderPayload template.JS
	buildOrderErr     error

	requestMu              sync.Mutex
	groupsRequested        bool
	buildOrderTabRequested bool
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

func (p *lazyShellProjection) trackSections() ([]TrackSection, error) {
	p.tracksOnce.Do(func() {
		rendered, err := p.renderedClaims()
		if err != nil {
			p.tracksErr = err
			return
		}
		p.tracks, p.tracksErr = buildTrackSectionsWithBudget(p.in.cat, p.in.cfg, rendered, p.budget)
	})
	return p.tracks, p.tracksErr
}

func (p *lazyShellProjection) graphPayloadJSON() (template.JS, error) {
	p.graphOnce.Do(func() {
		p.graph, p.graphErr = graphPayloadJSONWithBudget(p.in.cat, p.in.cfg, p.in.generatedAt, p.budget)
	})
	return p.graph, p.graphErr
}

func (p *lazyShellProjection) buildOrderData() (BuildOrderTab, template.JS, error) {
	p.buildOrderOnce.Do(func() {
		p.buildOrders, p.buildOrderPayload, p.buildOrderErr = buildOrderTabDataWithBudget(
			p.in.cat, p.in.cfg, p.buildOrderTemplate, p.in.generatedAt, p.budget,
		)
	})
	return p.buildOrders, p.buildOrderPayload, p.buildOrderErr
}

func (p *lazyShellProjection) noteGroupsRequested() error {
	p.requestMu.Lock()
	p.groupsRequested = true
	mustCheck := p.buildOrderTabRequested
	p.requestMu.Unlock()
	if !mustCheck {
		return nil
	}
	tab, _, err := p.buildOrderData()
	if err != nil {
		return err
	}
	groups, err := p.moduleGroups()
	if err != nil {
		return err
	}
	return buildOrderIDCollision(tab, groups)
}

func (p *lazyShellProjection) noteBuildOrderTabRequested() error {
	p.requestMu.Lock()
	p.buildOrderTabRequested = true
	mustCheck := p.groupsRequested
	p.requestMu.Unlock()
	if !mustCheck {
		return nil
	}
	tab, _, err := p.buildOrderData()
	if err != nil {
		return err
	}
	groups, err := p.moduleGroups()
	if err != nil {
		return err
	}
	return buildOrderIDCollision(tab, groups)
}
