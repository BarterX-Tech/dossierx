package graph

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestBuildProjectClaimIsANodeWithEdgesAndNoGroup pins the projection of a
// project claim (NIT-25): a node like any other, with empty module and facet,
// every declared rests_on edge to and from it resolved, degrees counted over
// those edges, and no "" group invented for it under modules or facets. The
// constitution itself is not a claim and never reaches the payload.
func TestBuildProjectClaimIsANodeWithEdgesAndNoGroup(t *testing.T) {
	cfg := &config.Config{
		Modules: []string{"widget"},
		Facets:  []string{"contract", "internals"},
	}
	scope := model.Claim{ID: "project.scope", Scope: model.ScopeProject, Status: model.StatusLocked, RestsOn: model.RestsNone("the roof above it is the constitution")}
	overview := model.Claim{ID: "widget.contract.overview", Module: "widget", Facet: "contract", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(scope.ID)}
	retention := model.Claim{ID: "project.retention", Scope: model.ScopeProject, Status: model.StatusDraft, RestsOn: model.RestsOnIDs(overview.ID)}
	fields := model.Claim{ID: "widget.internals.fields", Module: "widget", Facet: "internals", Status: model.StatusDraft, RestsOn: model.RestsOnIDs(overview.ID)}

	p := buildFrom(t, cfg, fields, retention, overview, scope)

	if len(p.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d: %+v", len(p.Nodes), p.Nodes)
	}
	for _, id := range []string{scope.ID, retention.ID} {
		n := nodeByID(t, p, id)
		if n.Module != "" || n.Facet != "" {
			t.Errorf("project claim %s projected with module %q facet %q; both must be empty", id, n.Module, n.Facet)
		}
		if n.Title == "" {
			t.Errorf("project claim %s has no derived title", id)
		}
	}
	if n := nodeByID(t, p, scope.ID); n.Status != "locked" || n.InDegree != 1 || n.OutDegree != 0 {
		t.Errorf("project.scope: status %q in %d out %d, want locked/1/0 (one edge in from overview, none out)", n.Status, n.InDegree, n.OutDegree)
	}
	if n := nodeByID(t, p, retention.ID); n.Status != "draft" || n.InDegree != 0 || n.OutDegree != 1 {
		t.Errorf("project.retention: status %q in %d out %d, want draft/0/1", n.Status, n.InDegree, n.OutDegree)
	}
	if n := nodeByID(t, p, overview.ID); n.InDegree != 2 || n.OutDegree != 1 {
		t.Errorf("overview: in %d out %d, want 2/1 (retention and fields rest on it; it rests on scope)", n.InDegree, n.OutDegree)
	}

	wantEdges := []Edge{
		{From: retention.ID, To: overview.ID, Type: EdgeRestsOn},
		{From: overview.ID, To: scope.ID, Type: EdgeRestsOn},
		{From: fields.ID, To: overview.ID, Type: EdgeRestsOn},
	}
	if !reflect.DeepEqual(p.Edges, wantEdges) {
		t.Errorf("edges = %+v\nwant %+v (sorted by from, type, to)", p.Edges, wantEdges)
	}
	if p.Dropped.UnresolvedEdges != 0 {
		t.Errorf("an edge to or from a project claim was dropped as unresolved: %+v", p.Dropped)
	}

	if !reflect.DeepEqual(p.Groups.Modules, []string{"widget"}) {
		t.Errorf("groups.modules = %v, want [widget]: a project claim is not a module and must not add a group", p.Groups.Modules)
	}
	if !reflect.DeepEqual(p.Groups.Facets, []string{"contract", "internals"}) {
		t.Errorf("groups.facets = %v, want [contract internals]", p.Groups.Facets)
	}

	// The wire form: the project claim's module and facet are present and
	// empty (the client buckets them under its catch-all), not omitted. Its
	// title is the whole id: claimLabel derives a label from the slug of a
	// module.facet.slug id and falls back to the id itself for any other
	// shape, so the pane labels a project claim "project.scope", not "Scope".
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"id":"project.scope","title":"project.scope","module":"","facet":"","status":"locked"`,
		`{"from":"widget.contract.overview","to":"project.scope","type":"rests_on"}`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("payload lacks %s:\n%s", want, raw)
		}
	}
}
