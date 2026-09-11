package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/layout"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRunReportsGeneratedWriteFailuresInTheirActualPhase(t *testing.T) {
	declaredProject := func(t *testing.T) (*config.Config, []model.Claim) {
		t.Helper()
		return project(t, baseConfig+"conformance:\n  observations: observations.json\n", map[string]string{
			"claims/one.yaml":   draftClaim("widget.contract.one") + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://one\n      expectation:\n        shape: set\n        value: [ready]\n",
			"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://one","shape":"set","value":["ready"]}]}`,
		})
	}

	t.Run("opted catalog write", func(t *testing.T) {
		cfg, claims := declaredProject(t)
		if err := os.MkdirAll(cfg.CatalogPath(), 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := check.Run(claims, cfg)
		if err == nil || res.CatalogError == "" || res.RenderError != "" || res.ConformanceError != "" || res.ConformanceFailurePhase != "catalog" {
			t.Fatalf("result=%+v err=%v", res, err)
		}
	})

	t.Run("opted gitignore write", func(t *testing.T) {
		cfg, claims := declaredProject(t)
		if err := os.MkdirAll(cfg.BuildGitignorePath(), 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := check.Run(claims, cfg)
		if err == nil || res.CatalogError == "" || res.ConformanceFailurePhase != "catalog" {
			t.Fatalf("result=%+v err=%v", res, err)
		}
	})

	t.Run("plain viewer directory", func(t *testing.T) {
		cfg, claims := project(t, baseConfig, map[string]string{"claims/one.yaml": draftClaim("widget.contract.one")})
		viewerDir := filepath.Dir(cfg.ViewerPath())
		if err := os.MkdirAll(filepath.Dir(viewerDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(viewerDir, []byte("conflict"), 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := check.Run(claims, cfg)
		if err == nil || res.RenderError == "" || res.CatalogError != "" || res.ConformanceError != "" || res.ConformanceFailurePhase != "render" {
			t.Fatalf("result=%+v err=%v", res, err)
		}
	})

	t.Run("opt out gitignore downgrade", func(t *testing.T) {
		cfg, claims := declaredProject(t)
		if _, err := check.Run(claims, cfg); err != nil {
			t.Fatalf("enable conformance: %v", err)
		}
		if err := os.Remove(cfg.BuildGitignorePath()); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(cfg.BuildGitignorePath(), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "one.yaml"), []byte(draftClaim("widget.contract.one")), 0o644); err != nil {
			t.Fatal(err)
		}
		claims, err := loader.LoadClaims(cfg.ClaimsDir)
		if err != nil {
			t.Fatal(err)
		}
		res, err := check.Run(claims, cfg)
		if err == nil || res.CatalogError == "" || res.ConformanceFailurePhase != "catalog" {
			t.Fatalf("result=%+v err=%v", res, err)
		}
		if owned, ownErr := layout.ConformanceStatusOwned(cfg); ownErr != nil || !owned {
			t.Fatalf("failed downgrade lost retry marker: owned=%v err=%v", owned, ownErr)
		}
		if _, statErr := os.Stat(cfg.ConformanceStatusPath()); !os.IsNotExist(statErr) {
			t.Fatalf("failed downgrade retained status: %v", statErr)
		}
		if err := os.RemoveAll(cfg.BuildGitignorePath()); err != nil {
			t.Fatal(err)
		}
		if _, err := check.Run(claims, cfg); err != nil {
			t.Fatalf("retry downgrade: %v", err)
		}
		if got, err := os.ReadFile(cfg.BuildGitignorePath()); err != nil || string(got) != layout.BuildGitignoreContent {
			t.Fatalf("final gitignore=%q err=%v", got, err)
		}
		if owned, ownErr := layout.ConformanceStatusOwned(cfg); ownErr != nil || owned {
			t.Fatalf("retry retained marker: owned=%v err=%v", owned, ownErr)
		}
	})
}
