package check

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestConformanceArtifactBounds(t *testing.T) {
	for _, kind := range []string{"catalog", "render: viewer"} {
		if err := conformanceOutputBound(kind, make([]byte, conformance.MaxOutputBytes)); err != nil {
			t.Fatalf("%s exact limit: %v", kind, err)
		}
		if err := conformanceOutputBound(kind, make([]byte, conformance.MaxOutputBytes+1)); err == nil || !strings.Contains(err.Error(), kind) {
			t.Fatalf("%s oversized error = %v", kind, err)
		}
	}
}

func TestViewerMultiplicityOverflowPreservesAllPreviousArtifacts(t *testing.T) {
	var facets []string
	for i := 0; i < 72; i++ {
		facets = append(facets, fmt.Sprintf("facet-%02d", i))
	}
	cfgBody := "schema_version: 1\nfacets: [" + strings.Join(facets, ", ") + "]\nmodules: [widget]\nclaims_dir: claims\n"
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(configPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	claims := []model.Claim{{
		ID: "widget.overview.router", Facet: "overview", Module: "widget", Status: model.StatusDraft,
		Layout: model.LayoutBanner, Kind: model.KindOrientationNote, Body: "orientation fixture",
		Governed:   model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
		Embodiment: &model.Embodiment{Mode: model.EmbodimentModeNone, Reason: strings.Repeat("x", 1<<20)},
	}}
	for _, facet := range facets {
		claims = append(claims, model.Claim{
			ID: fmt.Sprintf("widget.%s.one", facet), Facet: facet, Module: "widget", Status: model.StatusDraft,
			Layout: model.LayoutCard, Body: "facet fixture", Governed: model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
		})
	}
	old := []byte("previous-complete-artifact")
	for _, path := range []string{cfg.CatalogPath(), cfg.ConformanceStatusPath(), cfg.ViewerPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, old, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Run(claims, cfg)
	if err == nil || !errors.Is(err, conformance.ErrCapacityExceeded) || !strings.Contains(err.Error(), "viewer requires more than") || res.CatalogPath != "" || res.ConformancePath != "" || res.RenderPath != "" {
		t.Fatalf("Run result=%+v error=%v", res, err)
	}
	for _, path := range []string{cfg.CatalogPath(), cfg.ConformanceStatusPath(), cfg.ViewerPath()} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(got, old) {
			t.Fatalf("%s changed before preflight completed: %q err=%v", path, got, readErr)
		}
	}
}

func TestPlainViewerCapacityOverflowPreservesPreviousCatalogAndViewer(t *testing.T) {
	var facets []string
	for i := 0; i < 72; i++ {
		facets = append(facets, fmt.Sprintf("facet-%02d", i))
	}
	root := t.TempDir()
	cfgBody := "schema_version: 1\nfacets: [" + strings.Join(facets, ", ") + "]\nmodules: [widget]\nclaims_dir: claims\n"
	configPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(configPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	claims := []model.Claim{{
		ID: "widget.overview.router", Facet: "overview", Module: "widget", Status: model.StatusDraft,
		Layout: model.LayoutBanner, Kind: model.KindOrientationNote, Body: strings.Repeat("x", 1<<20),
		Governed: model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
	}}
	for _, facet := range facets {
		claims = append(claims, model.Claim{
			ID: fmt.Sprintf("widget.%s.one", facet), Facet: facet, Module: "widget", Status: model.StatusDraft,
			Layout: model.LayoutCard, Body: "facet fixture", Governed: model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
		})
	}
	old := []byte("previous-complete-artifact")
	for _, path := range []string{cfg.CatalogPath(), cfg.ViewerPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, old, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Run(claims, cfg)
	if err == nil || !errors.Is(err, conformance.ErrCapacityExceeded) || !strings.Contains(err.Error(), "viewer requires more than") {
		t.Fatalf("Run result=%+v error=%v", res, err)
	}
	if res.Conformance != nil || res.RenderError == "" || res.ConformanceFailurePhase != "render" || res.CatalogPath != "" || res.RenderPath != "" {
		t.Fatalf("plain capacity refusal reported a replacement: %+v", res)
	}
	for _, path := range []string{cfg.CatalogPath(), cfg.ViewerPath()} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(got, old) {
			t.Fatalf("%s changed before all capacity checks completed: %q err=%v", path, got, readErr)
		}
	}
}

func TestSharedTargetProjectionOverflowPreservesAllPreviousArtifacts(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(configPath, []byte("schema_version: 1\nfacets: [contract]\nmodules: [widget]\nclaims_dir: claims\nconformance:\n  observations: observations.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	claims := make([]model.Claim, 96)
	for i := range claims {
		claims[i] = model.Claim{
			ID: fmt.Sprintf("widget.contract.shared-%03d", i), Facet: "contract", Module: "widget", Status: model.StatusDraft,
			Layout: model.LayoutCard, Body: "shared target fixture", Governed: model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
			Embodiment: &model.Embodiment{Mode: model.EmbodimentModeCompare, Checks: []model.EmbodimentCheck{{ID: "state", Adapter: "neutral/v1", Target: "widget://shared", Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeSet, Value: []string{fmt.Sprintf("expected-%03d", i)}}}}},
		}
	}
	large := strings.Repeat("x", 1<<20)
	if err := os.WriteFile(cfg.Conformance.Observations, []byte(`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://shared","shape":"set","value":["`+large+`"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := []byte("previous-complete-artifact")
	for _, path := range []string{cfg.CatalogPath(), cfg.ConformanceStatusPath(), cfg.ViewerPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, old, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Run(claims, cfg)
	if err == nil || !strings.Contains(err.Error(), "conformance projection requires more than") || res.ConformanceError == "" || res.CatalogPath != "" || res.ConformancePath != "" || res.RenderPath != "" {
		t.Fatalf("Run result=%+v error=%v", res, err)
	}
	for _, path := range []string{cfg.CatalogPath(), cfg.ConformanceStatusPath(), cfg.ViewerPath()} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(got, old) {
			t.Fatalf("%s changed before projection preflight completed: %q err=%v", path, got, readErr)
		}
	}
}

func TestPlainCatalogCapacityUsesCatalogDomainBeforeWrites(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(configPath, []byte("schema_version: 1\nfacets: [contract]\nmodules: [widget]\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	shared := strings.Repeat("x", 20<<20)
	claims := make([]model.Claim, 4)
	for i := range claims {
		claims[i] = model.Claim{
			ID: fmt.Sprintf("widget.contract.capacity-%d", i), Facet: "contract", Module: "widget",
			Status: model.StatusDraft, Layout: model.LayoutCard, Body: "plain capacity fixture",
			Governed: model.Governed{Type: string(model.GovernedNone), Reason: shared},
		}
	}
	old := []byte("previous-complete-artifact")
	for _, path := range []string{cfg.CatalogPath(), cfg.ViewerPath()} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, old, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Run(claims, cfg)
	if err == nil || !errors.Is(err, conformance.ErrCapacityExceeded) {
		t.Fatalf("Run error=%v result=%+v", err, res)
	}
	if res.ConformanceError != "" || res.CatalogError == "" || res.RenderError != "" || res.ConformanceFailurePhase != "catalog" || !res.ConformanceCapacityExceeded {
		t.Fatalf("capacity error domain=%+v", res)
	}
	if res.CatalogPath != "" || res.RenderPath != "" {
		t.Fatalf("capacity refusal reported writes: catalog=%q viewer=%q", res.CatalogPath, res.RenderPath)
	}
	for _, path := range []string{cfg.CatalogPath(), cfg.ViewerPath()} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(got, old) {
			t.Fatalf("%s changed before capacity refusal: %q err=%v", path, got, readErr)
		}
	}
}

func TestReadOnlyOptOutDoesNotBuildOrBoundCatalog(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(configPath, []byte("schema_version: 1\nfacets: [contract]\nmodules: [widget]\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	shared := strings.Repeat("x", 20<<20)
	claims := make([]model.Claim, 4)
	for i := range claims {
		claims[i] = model.Claim{
			ID: fmt.Sprintf("widget.contract.capacity-%d", i), Facet: "contract", Module: "widget",
			Status: model.StatusDraft, Layout: model.LayoutCard, Body: "plain capacity fixture",
			Governed: model.Governed{Type: string(model.GovernedNone), Reason: shared},
		}
	}

	res := Status(claims, cfg)
	if !res.OK || res.Conformance != nil || res.ConformanceError != "" || res.CatalogError != "" || res.RenderError != "" || res.ConformanceCapacityExceeded {
		t.Fatalf("non-conformance read-only result changed by projection capacity: %+v", res)
	}
	if res.CatalogPath != "" || res.RenderPath != "" {
		t.Fatalf("read-only check reported writes: catalog=%q viewer=%q", res.CatalogPath, res.RenderPath)
	}
}
