package catalog

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/projectclaims"
)

// TestDocumentProjectsAProjectClaimWithNoModuleAndItsEdges pins the catalog
// projection of a project claim (NIT-25): an entry with empty module and
// facet, its rests_on edges (a list, or none with its reason) intact, absent
// from by_module and by_facet, and listed by the project-claims index that
// the manifest and the viewer read.
//
// What the catalog does NOT carry is stated here too: Entry has no scope
// field, so catalog.json distinguishes a project claim only by its id shape
// and its empty module/facet. Adding one is a projection change for the
// engine, not something a test can assert into being.
func TestDocumentProjectsAProjectClaimWithNoModuleAndItsEdges(t *testing.T) {
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}
	scope := model.Claim{ID: "project.scope", Scope: model.ScopeProject, Status: model.StatusLocked, Summary: "Every widget is kept under one roof.", Body: "Every widget is kept under one roof.", RestsOn: model.RestsNone("the roof above it is the constitution")}
	overview := model.Claim{ID: "widget.contract.overview", Module: "widget", Facet: "contract", Status: model.StatusLocked, Summary: "A widget is the smallest unit.", Body: "A widget is the smallest unit.", RestsOn: model.RestsOnIDs(scope.ID)}
	retention := model.Claim{ID: "project.retention", Scope: model.ScopeProject, Status: model.StatusDraft, Summary: "Kept for thirty days.", Body: "Kept for thirty days.\nLonger on request.", RestsOn: model.RestsOnIDs(overview.ID)}

	cat, err := Build([]model.Claim{retention, overview, scope}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	doc := cat.Document()

	byID := map[string]Entry{}
	for _, e := range doc.Claims {
		byID[e.ID] = e
	}
	if len(byID) != 3 {
		t.Fatalf("expected 3 entries, got %+v", doc.Claims)
	}
	sc := byID[scope.ID]
	if sc.Module != "" || sc.Facet != "" || sc.Status != model.StatusLocked || sc.Layout != model.LayoutCard {
		t.Errorf("project.scope entry = %+v; want no module, no facet, locked, card", sc)
	}
	if !sc.Edges.RestsOnNone || sc.Edges.RestsOnReason != "the roof above it is the constitution" || sc.Edges.RestsOn != nil {
		t.Errorf("project.scope edges = %+v; want rests_on_none with its reason and no targets", sc.Edges)
	}
	rt := byID[retention.ID]
	if rt.Module != "" || rt.Facet != "" || rt.Status != model.StatusDraft {
		t.Errorf("project.retention entry = %+v; want no module, no facet, draft", rt)
	}
	if !reflect.DeepEqual(rt.Edges.RestsOn, []string{overview.ID}) || rt.Edges.RestsOnNone {
		t.Errorf("project.retention edges = %+v; want rests_on [%s]", rt.Edges, overview.ID)
	}
	if ov := byID[overview.ID]; !reflect.DeepEqual(ov.Edges.RestsOn, []string{scope.ID}) {
		t.Errorf("overview edges = %+v; want rests_on [%s]", ov.Edges, scope.ID)
	}

	if !reflect.DeepEqual(doc.ByModule, map[string][]string{"widget": {overview.ID}}) {
		t.Errorf("by_module = %v; a project claim belongs to no module bucket", doc.ByModule)
	}
	if !reflect.DeepEqual(doc.ByFacet, map[string][]string{"contract": {overview.ID}}) {
		t.Errorf("by_facet = %v; a project claim belongs to no facet bucket", doc.ByFacet)
	}

	raw, err := EncodeJSON(cat)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"id": "project.scope"`,
		`"rests_on_reason": "the roof above it is the constitution"`,
		`"id": "project.retention"`,
		`"rests_on": [`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("catalog.json lacks %s:\n%s", want, raw)
		}
	}
	if !regexp.MustCompile(`"rests_on": \[\s*"widget\.contract\.overview"\s*\]`).Match(raw) {
		t.Errorf("catalog.json does not serialize project.retention's rests_on edge as a list:\n%s", raw)
	}
	if strings.Contains(string(raw), `"scope"`) {
		t.Errorf("catalog.json now carries a scope key; update this test's stated boundary and the consumers that read it:\n%s", raw)
	}
	if strings.Contains(string(raw), "Every widget is kept") {
		t.Errorf("a project claim's body leaked into the catalog projection")
	}

	// The tier-1 read over the merged catalog: one line per project claim,
	// sorted by id, first non-blank body line as the summary.
	want := []projectclaims.Entry{
		{ID: retention.ID, Summary: "Kept for thirty days.", Status: "draft"},
		{ID: scope.ID, Summary: "Every widget is kept under one roof.", Status: "locked"},
	}
	if got := projectclaims.Index(cat.Claims); !reflect.DeepEqual(got, want) {
		t.Errorf("project-claims index = %+v, want %+v", got, want)
	}
}
