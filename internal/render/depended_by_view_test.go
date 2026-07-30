// depended_by_view_test.go covers buildDependedByLookup, buildTargetStatusLookup,
// and attachEdgesOverride's graceful-degradation contract for both: present
// (an extra "depended on by" line / a C6 status pill on an actionable
// target) only when the catalog actually has something to say, absent
// (byte-for-byte unchanged from a project that never touched either
// feature) otherwise — mirroring implink_view_test.go's approach for the
// sibling "implemented in" extension.
package render

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

func TestBuildTargetStatusLookup_NilCatalogReturnsNil(t *testing.T) {
	if got := buildTargetStatusLookup(nil); got != nil {
		t.Fatalf("buildTargetStatusLookup(nil) = %#v, want nil", got)
	}
}

func TestBuildTargetStatusLookup_EmptyCatalogReturnsNil(t *testing.T) {
	if got := buildTargetStatusLookup(&catalog.Catalog{}); got != nil {
		t.Fatalf("buildTargetStatusLookup(empty) = %#v, want nil", got)
	}
}

func TestBuildTargetStatusLookup_CapturesStatusAndReviewPending(t *testing.T) {
	cat := &catalog.Catalog{Claims: []model.Claim{
		{ID: "widget.contract.a", Status: model.StatusDraft},
		{ID: "widget.contract.b", Status: model.StatusLocked, ReviewPending: true},
		{ID: "widget.contract.c", Status: model.StatusLocked},
	}}
	got := buildTargetStatusLookup(cat)
	want := map[string]components.TargetStatus{
		"widget.contract.a": {Status: model.StatusDraft},
		"widget.contract.b": {Status: model.StatusLocked, ReviewPending: true},
		"widget.contract.c": {Status: model.StatusLocked},
	}
	if len(got) != len(want) {
		t.Fatalf("buildTargetStatusLookup: got %d entries, want %d: %#v", len(got), len(want), got)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("buildTargetStatusLookup[%q] = %#v, want %#v", id, got[id], w)
		}
	}
}

// targetPillTestConfig mirrors implink_view_test.go's implinkTestConfig: a
// minimal, valid project.config.yaml loaded via config.LoadConfig.
func targetPillTestConfig(t *testing.T, module string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - " + module + "\nclaims_dir: claims\n"
	cfgPath := dir + "/project.config.yaml"
	writeFile(t, cfgPath, cfgYAML)
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func TestRender_TargetPill_ActionableRestsOnTargetGetsPill(t *testing.T) {
	module := "widget"
	cfg := targetPillTestConfig(t, module)
	claims := []model.Claim{
		{
			ID: module + ".contract.main", Module: module, Facet: "contract",
			Status: model.StatusLocked, Layout: model.LayoutCard, Body: "main claim",
			BuildRole: model.BuildRoleBehavior,
			Governed:  model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
			RestsOn:   []string{module + ".contract.dep"},
		},
		{
			ID: module + ".contract.dep", Module: module, Facet: "contract",
			Status: model.StatusDraft, Layout: model.LayoutCard, Body: "dep claim",
			BuildRole: model.BuildRoleBehavior,
			Governed:  model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
		},
	}

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, `<span class="pill pv">draft</span>`) {
		t.Fatalf("expected a draft pill on the rests_on target that is itself draft, got:\n%s", out)
	}
}

func TestRender_TargetPill_HealthyLockedTargetGetsNoPillOnEdge(t *testing.T) {
	module := "widget"
	cfg := targetPillTestConfig(t, module)
	claims := []model.Claim{
		{
			ID: module + ".contract.main", Module: module, Facet: "contract",
			Status: model.StatusLocked, Layout: model.LayoutCard, Body: "main claim",
			BuildRole: model.BuildRoleBehavior,
			Governed:  model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
			RestsOn:   []string{module + ".contract.dep"},
		},
		{
			ID: module + ".contract.dep", Module: module, Facet: "contract",
			Status: model.StatusLocked, Layout: model.LayoutCard, Body: "dep claim",
			BuildRole: model.BuildRoleBehavior,
			Governed:  model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
		},
	}

	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, `class="pill pv"`) || strings.Contains(out, `pill pw">review_pending`) {
		t.Fatalf("a healthy locked rests_on target must get no status pill, got:\n%s", out)
	}
}
