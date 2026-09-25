package check

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// armConstitution is the internal-package twin of the external suite's
// helper (ledger_test.go): a minimal locked roof plus its lock-store record,
// so the roof gate (NIT-26) lets these projections run.
func armConstitution(t *testing.T, cfg *config.Config) {
	t.Helper()
	path := cfg.ConstitutionPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		roof := "status: locked\ninvariants:\n  - slug: one-roof\n    title: One roof\n    body: This fixture has one lockable constitution above every module.\n"
		if err := os.WriteFile(path, []byte(roof), 0o644); err != nil {
			t.Fatalf("arm constitution: write: %v", err)
		}
	}
	f, err := constitution.Load(path)
	if err != nil {
		t.Fatalf("arm constitution: load: %v", err)
	}
	store, err := lock.LoadStore(cfg.LockStorePath())
	if err != nil {
		t.Fatalf("arm constitution: load store: %v", err)
	}
	lock.LockConstitution(store, f, "fixture roof", time.Now())
	// The crossing the real command performs on a fresh project: the comment
	// threads already on disk are taken into digest coverage now, silently,
	// so a fixture that hand-writes a thread before arming is not "unrecorded".
	if !store.LedgerCovered() && !store.PreLedger() {
		if claims, loadErr := loader.LoadAll(cfg); loadErr == nil {
			lock.SweepCommentDigests(store, claims, false)
		}
	}
	if err := store.Save(); err != nil {
		t.Fatalf("arm constitution: save store: %v", err)
	}
}

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
	// Facets are engine-fixed (contract|internals plus the Manifest peer tab),
	// so tab-strip multiplicity now comes from modules: each module's overview
	// HTML is injected into Manifest | Contract | Internals.
	modules := make([]string, 80)
	for i := range modules {
		modules[i] = fmt.Sprintf("mod%02d", i)
	}
	cfgBody := "schema_version: 1\nfacets: [contract, internals]\nmodules: [" + strings.Join(modules, ", ") + "]\nclaims_dir: claims\nmax_claim_body_chars: 100000000\nmax_claims_per_module: 10000\n"
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
	armConstitution(t, cfg)
	for _, mod := range modules {
		if err := manifest.WriteMinimal(cfg.ClaimsDir, mod); err != nil {
			t.Fatal(err)
		}
	}
	heavy := strings.Repeat("x", 1<<20)
	var claims []model.Claim
	for _, mod := range modules {
		claims = append(claims, model.Claim{
			ID: mod + ".contract.one", Facet: "contract", Module: mod, Status: model.StatusDraft,
			Layout: model.LayoutCard, Summary: "Fixture claim used by the engine test corpus.", Body: heavy, RestsOn: model.RestsNone("fixture"),
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
	modules := make([]string, 80)
	for i := range modules {
		modules[i] = fmt.Sprintf("mod%02d", i)
	}
	root := t.TempDir()
	cfgBody := "schema_version: 1\nfacets: [contract, internals]\nmodules: [" + strings.Join(modules, ", ") + "]\nclaims_dir: claims\nmax_claim_body_chars: 100000000\nmax_claims_per_module: 10000\n"
	configPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(configPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	armConstitution(t, cfg)
	for _, mod := range modules {
		if err := manifest.WriteMinimal(cfg.ClaimsDir, mod); err != nil {
			t.Fatal(err)
		}
	}
	heavy := strings.Repeat("x", 1<<20)
	var claims []model.Claim
	for _, mod := range modules {
		claims = append(claims, model.Claim{
			ID: mod + ".contract.one", Facet: "contract", Module: mod, Status: model.StatusDraft,
			Layout: model.LayoutCard, Summary: "Fixture claim used by the engine test corpus.", Body: heavy, RestsOn: model.RestsNone("fixture"),
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
	if err := os.WriteFile(configPath, []byte("schema_version: 1\nfacets: [contract, internals]\nmodules: [widget]\nclaims_dir: claims\nmax_claim_body_chars: 100000000\nmax_claims_per_module: 10000\nconformance:\n  observations: observations.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	armConstitution(t, cfg)
	if err := manifest.WriteMinimal(cfg.ClaimsDir, "widget"); err != nil {
		t.Fatal(err)
	}
	claims := make([]model.Claim, 96)
	for i := range claims {
		claims[i] = model.Claim{
			ID: fmt.Sprintf("widget.contract.shared-%03d", i), Facet: "contract", Module: "widget", Status: model.StatusDraft,
			Layout: model.LayoutCard, Summary: "Fixture claim used by the engine test corpus.", Body: "shared target fixture", RestsOn: model.RestsNone("fixture"),
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
	if err := os.WriteFile(configPath, []byte("schema_version: 1\nfacets: [contract, internals]\nmodules: [widget]\nclaims_dir: claims\nmax_claim_body_chars: 100000000\nmax_claims_per_module: 10000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	armConstitution(t, cfg)
	if err := manifest.WriteMinimal(cfg.ClaimsDir, "widget"); err != nil {
		t.Fatal(err)
	}
	shared := strings.Repeat("x", 20<<20)
	claims := make([]model.Claim, 4)
	for i := range claims {
		claims[i] = model.Claim{
			ID: fmt.Sprintf("widget.contract.capacity-%d", i), Facet: "contract", Module: "widget",
			Status: model.StatusDraft, Layout: model.LayoutCard, Summary: "Fixture claim used by the engine test corpus.", Body: "plain capacity fixture",
			RestsOn: model.RestsNone(shared),
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
	if err := os.WriteFile(configPath, []byte("schema_version: 1\nfacets: [contract, internals]\nmodules: [widget]\nclaims_dir: claims\nmax_claim_body_chars: 100000000\nmax_claims_per_module: 10000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	armConstitution(t, cfg)
	if err := manifest.WriteMinimal(cfg.ClaimsDir, "widget"); err != nil {
		t.Fatal(err)
	}
	shared := strings.Repeat("x", 20<<20)
	claims := make([]model.Claim, 4)
	for i := range claims {
		claims[i] = model.Claim{
			ID: fmt.Sprintf("widget.contract.capacity-%d", i), Facet: "contract", Module: "widget",
			Status: model.StatusDraft, Layout: model.LayoutCard, Summary: "Fixture claim used by the engine test corpus.", Body: "plain capacity fixture",
			RestsOn: model.RestsNone(shared),
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
