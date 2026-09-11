package render

import (
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

const boundedRenderAllocationBudget = 512 << 20

func TestRenderWithThemeBounded_OversizedSingleClaimStopsBeforeFragmentAllocation(t *testing.T) {
	// Allocate the authored input before measuring the renderer. The contract is
	// about projection work, not the caller's already-resident catalog.
	body := strings.Repeat("x", 32<<20)
	cat := &catalog.Catalog{Claims: []model.Claim{{
		ID:     "module.contract.oversized",
		Module: "module",
		Facet:  "contract",
		Layout: model.LayoutTree,
		Status: model.StatusDraft,
		Body:   body,
	}}}

	var out string
	var err error
	allocated := measuredTotalAlloc(func() {
		out, err = RenderWithThemeBounded(cat, nil, nil, 1<<20)
	})
	if !errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("RenderWithThemeBounded error = %v, want capacity exceeded", err)
	}
	if out != "" {
		t.Fatalf("bounded render returned %d partial bytes", len(out))
	}
	if allocated >= boundedRenderAllocationBudget {
		t.Fatalf("bounded render allocated %d bytes, budget is <%d", allocated, boundedRenderAllocationBudget)
	}
	t.Logf("single-claim overflow: input=%d max=%d TotalAlloc=%d", len(body), 1<<20, allocated)
}

func TestRenderWithThemeBounded_ManyShortGraphRecordsStayContained(t *testing.T) {
	// Empty claims isolate graph/shell structural growth from claim-body growth.
	// Every configured module is emitted into the graph payload, so these short
	// records overflow the viewer budget without one oversized string.
	modules := make([]string, 100_000)
	for i := range modules {
		modules[i] = fmt.Sprintf("m%06d", i)
	}
	cfg := &config.Config{Modules: modules}
	cat := &catalog.Catalog{}

	var out string
	var err error
	allocated := measuredTotalAlloc(func() {
		out, err = RenderWithThemeBounded(cat, cfg, nil, 1<<20)
	})
	if !errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("RenderWithThemeBounded error = %v, want capacity exceeded", err)
	}
	if out != "" {
		t.Fatalf("bounded render returned %d partial bytes", len(out))
	}
	if allocated >= boundedRenderAllocationBudget {
		t.Fatalf("bounded structural render allocated %d bytes, budget is <%d", allocated, boundedRenderAllocationBudget)
	}
	t.Logf("structural overflow: modules=%d max=%d TotalAlloc=%d", len(modules), 1<<20, allocated)
}

func TestRenderWithThemeBounded_ManyShortClaimFragmentsStayContained(t *testing.T) {
	claims := make([]model.Claim, 20_000)
	for i := range claims {
		claims[i] = model.Claim{
			ID:     fmt.Sprintf("m.f.c%05d", i),
			Module: "m",
			Facet:  "f",
			Layout: model.LayoutTree,
			Status: model.StatusDraft,
			Body:   "x",
		}
	}
	cat := &catalog.Catalog{Claims: claims}

	var out string
	var err error
	allocated := measuredTotalAlloc(func() {
		out, err = RenderWithThemeBounded(cat, nil, nil, 1<<20)
	})
	if !errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("RenderWithThemeBounded error = %v, want capacity exceeded", err)
	}
	if out != "" {
		t.Fatalf("bounded render returned %d partial bytes", len(out))
	}
	if allocated >= boundedRenderAllocationBudget {
		t.Fatalf("bounded fragment render allocated %d bytes, budget is <%d", allocated, boundedRenderAllocationBudget)
	}
	t.Logf("fragment overflow: claims=%d max=%d TotalAlloc=%d", len(claims), 1<<20, allocated)
}

func TestRenderWithThemeBounded_UnderLimitMatchesLegacyRender(t *testing.T) {
	cat := &catalog.Catalog{Claims: []model.Claim{{
		ID:     "module.contract.small",
		Module: "module",
		Facet:  "contract",
		Layout: model.LayoutCard,
		Status: model.StatusDraft,
		Body:   "A small **bounded** claim.",
	}}}
	cfg := &config.Config{Modules: []string{"module"}, Facets: []string{"contract"}}

	legacy, err := RenderWithTheme(cat, cfg, nil)
	if err != nil {
		t.Fatalf("RenderWithTheme: %v", err)
	}
	bounded, err := RenderWithThemeBounded(cat, cfg, nil, 8<<20)
	if err != nil {
		t.Fatalf("RenderWithThemeBounded: %v", err)
	}
	if got, want := normalizeRenderTimes(bounded), normalizeRenderTimes(legacy); got != want {
		t.Fatal("bounded render changed under-limit viewer bytes apart from render timestamps")
	}
}

func TestBuildOrderTabDataWithBudget_CapsFragmentAndPayloadAccumulation(t *testing.T) {
	const module = "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)
	lockBuildOrder(t, cfg, claims, module)
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	generatedAt := time.Unix(1_700_000_000, 0).UTC()

	t.Run("module fragment", func(t *testing.T) {
		largeTemplate := template.Must(template.New("large").Parse(strings.Repeat("x", 2<<20)))
		tab, payload, err := buildOrderTabDataWithBudget(cat, cfg, largeTemplate, generatedAt, &renderByteBudget{remaining: 1 << 20})
		if !errors.Is(err, ErrIntermediateCapacityExceeded) {
			t.Fatalf("buildOrderTabDataWithBudget error = %v, want intermediate capacity exceeded", err)
		}
		if len(tab.Modules) != 0 || payload != "" {
			t.Fatal("build-order fragment overflow returned partial tab data")
		}
	})

	t.Run("JSON payload", func(t *testing.T) {
		loaded, err := loadTemplates("")
		if err != nil {
			t.Fatalf("loadTemplates: %v", err)
		}
		tab, payload, err := buildOrderTabData(cat, cfg, loaded.buildOrder, generatedAt)
		if err != nil {
			t.Fatalf("buildOrderTabData: %v", err)
		}
		retainedBytes := len(payload)
		for _, module := range tab.Modules {
			retainedBytes += len(module.HTML)
		}
		if retainedBytes < 2 {
			t.Fatalf("unexpected retained build-order size %d", retainedBytes)
		}

		gotTab, gotPayload, err := buildOrderTabDataWithBudget(cat, cfg, loaded.buildOrder, generatedAt, &renderByteBudget{remaining: retainedBytes - 1})
		if !errors.Is(err, ErrIntermediateCapacityExceeded) {
			t.Fatalf("buildOrderTabDataWithBudget error = %v, want intermediate payload capacity exceeded", err)
		}
		if len(gotTab.Modules) != 0 || gotPayload != "" {
			t.Fatal("build-order payload overflow returned partial tab data")
		}
	})
}

func TestBuildTrackSectionsWithBudget_ManyTrackRowsStayContained(t *testing.T) {
	const count = 20_000
	claims := make([]model.Claim, count)
	for i := range claims {
		claims[i] = model.Claim{
			ID:     fmt.Sprintf("m.f.c%05d", i),
			Module: "m",
			Facet:  "f",
			Status: model.StatusDraft,
			Tracks: []model.TrackRef{{ID: "feature", Role: model.TrackRoleCites}},
		}
	}
	cat := &catalog.Catalog{Claims: claims}
	cfg := &config.Config{Tracks: []config.Track{{ID: "feature", Title: "Feature"}}}

	var tracks []TrackSection
	var err error
	allocated := measuredTotalAlloc(func() {
		tracks, err = buildTrackSectionsWithBudget(cat, cfg, nil, &renderByteBudget{remaining: 1 << 20})
	})
	if !errors.Is(err, ErrIntermediateCapacityExceeded) {
		t.Fatalf("buildTrackSectionsWithBudget error = %v, want intermediate capacity exceeded", err)
	}
	if len(tracks) != 0 {
		t.Fatal("shell-data overflow returned a partial projection")
	}
	if allocated >= boundedRenderAllocationBudget {
		t.Fatalf("bounded shell-data build allocated %d bytes, budget is <%d", allocated, boundedRenderAllocationBudget)
	}
	t.Logf("shell-data overflow: cited rows=%d max=%d TotalAlloc=%d", count, 1<<20, allocated)
}

func TestRenderWithThemeBounded_CustomShellWorkingSetErrorIsNotOutputCapacity(t *testing.T) {
	claim := model.Claim{
		ID: "m.f.working-set", Module: "m", Facet: "f", Layout: model.LayoutTree,
		Status: model.StatusDraft, Body: strings.Repeat("x", 129<<20),
	}
	cat := &catalog.Catalog{Claims: []model.Claim{claim}}
	dir := t.TempDir()
	// The body itself is never emitted: ModuleGroups is used only as a truthy
	// condition. Reaching it still requires computing the projection, so the
	// separate retained-working-set guard may refuse that computation.
	writeFile(t, dir+"/shell.html", `<!doctype html>{{if .ModuleGroups}}tiny{{end}}`)
	cfg := &config.Config{Viewer: config.Viewer{TemplateOverrides: dir}}

	var out string
	var err error
	allocated := measuredTotalAlloc(func() {
		out, err = RenderWithThemeBounded(cat, cfg, nil, 1<<20)
	})
	if !errors.Is(err, ErrIntermediateCapacityExceeded) {
		t.Fatalf("custom working-set error = %v, want distinct intermediate capacity error", err)
	}
	if errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("custom working-set error was mislabeled as output capacity: %v", err)
	}
	if out != "" {
		t.Fatalf("working-set refusal returned %d partial bytes", len(out))
	}
	if strings.Contains(err.Error(), "viewer requires more than") {
		t.Fatalf("working-set error falsely claims final viewer size: %v", err)
	}
	if allocated >= boundedRenderAllocationBudget {
		t.Fatalf("working-set refusal allocated %d bytes, budget is <%d", allocated, boundedRenderAllocationBudget)
	}
	t.Logf("custom working-set refusal: input=%d retained-limit=%d TotalAlloc=%d", len(claim.Body), maxBoundedRenderIntermediateBytes, allocated)
}

func TestRenderWithThemeBounded_CustomShellChargesOnlyExecutedOutput(t *testing.T) {
	largeClaim := model.Claim{
		ID: "module.contract.large", Module: "module", Facet: "contract",
		Layout: model.LayoutTree, Status: model.StatusDraft, Body: strings.Repeat("x", 2<<20),
		Embodiment: &model.Embodiment{Mode: model.EmbodimentModeNone, Reason: "external implementation"},
	}
	cat := &catalog.Catalog{
		Claims: []model.Claim{largeClaim},
		Conformance: map[string]conformance.Result{
			largeClaim.ID: {ClaimID: largeClaim.ID, Mode: model.EmbodimentModeNone, Reason: "external implementation"},
		},
	}

	for _, tc := range []struct {
		name  string
		shell string
	}{
		{name: "static", shell: `<!doctype html><html><body>tiny</body></html>`},
		{name: "title only", shell: `<!doctype html><title>{{.Title}}</title>`},
		{name: "dead field branch", shell: `<!doctype html>{{if false}}{{range .ModuleGroups}}{{.ID}}{{end}}{{end}}tiny`},
		{name: "dead associated template", shell: `{{define "large"}}{{range .ModuleGroups}}{{range .Facets}}{{range .Claims}}{{.}}{{end}}{{end}}{{end}}{{end}}<!doctype html>{{if false}}{{template "large" .}}{{end}}tiny`},
		{name: "graph only", shell: `<!doctype html><script type="application/json">{{.GraphPayload}}</script>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir+"/shell.html", tc.shell)
			cfg := &config.Config{Title: "Tiny", Viewer: config.Viewer{TemplateOverrides: dir}}

			legacy, err := RenderWithTheme(cat, cfg, nil)
			if err != nil {
				t.Fatalf("legacy render: %v", err)
			}
			bounded, err := RenderWithThemeBounded(cat, cfg, nil, 1<<20)
			if err != nil {
				t.Fatalf("bounded render: %v", err)
			}
			if len(bounded) >= 1<<20 {
				t.Fatalf("actual custom-shell output = %d, want <1MiB", len(bounded))
			}
			if got, want := normalizeRenderTimes(bounded), normalizeRenderTimes(legacy); got != want {
				t.Fatal("bounded custom-shell output differs from legacy output")
			}
		})
	}
}

func TestRenderWithThemeBounded_CustomShellOmittedGraphIsNotCharged(t *testing.T) {
	modules := make([]string, 100_000)
	for i := range modules {
		modules[i] = fmt.Sprintf("m%06d", i)
	}
	claim := model.Claim{ID: "m.f.small", Module: "m", Facet: "f", Layout: model.LayoutTree, Status: model.StatusDraft, Body: "small"}
	cat := &catalog.Catalog{Claims: []model.Claim{claim}}
	dir := t.TempDir()
	writeFile(t, dir+"/shell.html", `<!doctype html>{{range .ModuleGroups}}{{range .Facets}}{{range .Claims}}{{.}}{{end}}{{end}}{{end}}`)
	cfg := &config.Config{Modules: modules, Viewer: config.Viewer{TemplateOverrides: dir}}

	out, err := RenderWithThemeBounded(cat, cfg, nil, 1<<20)
	if err != nil {
		t.Fatalf("claims-only bounded render: %v", err)
	}
	if !strings.Contains(out, claim.ID) || len(out) >= 1<<20 {
		t.Fatalf("claims-only custom output missing claim or oversized: bytes=%d", len(out))
	}
}

func TestRenderWithThemeBounded_CustomShellRepeatedFieldUsesFinalOutputLimit(t *testing.T) {
	claim := model.Claim{
		ID: "m.f.repeated", Module: "m", Facet: "f", Layout: model.LayoutTree,
		Status: model.StatusDraft, Body: strings.Repeat("x", 600<<10),
	}
	cat := &catalog.Catalog{Claims: []model.Claim{claim}}
	dir := t.TempDir()
	writeFile(t, dir+"/shell.html", `{{range .ModuleGroups}}{{range .Facets}}{{range .Claims}}{{.}}{{end}}{{end}}{{end}}{{range .ModuleGroups}}{{range .Facets}}{{range .Claims}}{{.}}{{end}}{{end}}{{end}}`)
	cfg := &config.Config{Viewer: config.Viewer{TemplateOverrides: dir}}

	out, err := RenderWithThemeBounded(cat, cfg, nil, 1<<20)
	if !errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("repeated emitted field error = %v, want output capacity exceeded", err)
	}
	if out != "" {
		t.Fatalf("repeated field overflow returned %d partial bytes", len(out))
	}
}

func measuredTotalAlloc(fn func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fn()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

var (
	headerTimeRE = regexp.MustCompile(`dossierx check at [0-9TZ:+-]+`)
	graphTimeRE  = regexp.MustCompile(`"generated_at":"[^"]+"`)
	footerTimeRE = regexp.MustCompile(`Generated [0-9-]+ [0-9:]+ UTC`)
)

func normalizeRenderTimes(s string) string {
	s = headerTimeRE.ReplaceAllString(s, "dossierx check at <time>")
	s = graphTimeRE.ReplaceAllString(s, `"generated_at":"<time>"`)
	return footerTimeRE.ReplaceAllString(s, "Generated <time>")
}
