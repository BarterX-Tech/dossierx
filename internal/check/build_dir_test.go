package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/layout"
)

// TestRunWritesEverythingUnderBuildDir: from an empty project directory, one
// check.Run writes the catalog, the viewer and the build directory's own
// .gitignore under build/, and nothing at the project root. The comment digest
// is NOT in this list on purpose: check.Run never writes it — the CLI's store
// preparation does (cmd/dossierx/main.go prepareStore, through
// lock.PrepareStore's digest sweep) before Run is reached — so its placement
// under build/ledger/ is pinned where it is written, by the CLI-level
// TestCheckOnAFreshProjectWritesOnlyUnderBuild in cmd/dossierx.
func TestRunWritesEverythingUnderBuildDir(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("Run: %v (findings %v)", err, res.LedgerFindings)
	}
	for _, want := range []string{
		filepath.Join("build", "catalog", "catalog.json"),
		filepath.Join("build", "viewer", "index.html"),
		filepath.Join("build", ".gitignore"),
	} {
		if _, err := os.Stat(filepath.Join(cfg.Dir(), want)); err != nil {
			t.Fatalf("check did not write %s: %v", want, err)
		}
	}
	if res.CatalogPath != cfg.CatalogPath() || res.RenderPath != cfg.ViewerPath() {
		t.Fatalf("Result paths %q/%q disagree with the config's %q/%q", res.CatalogPath, res.RenderPath, cfg.CatalogPath(), cfg.ViewerPath())
	}
	got, err := os.ReadFile(cfg.BuildGitignorePath())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != layout.BuildGitignoreContent {
		t.Fatalf("build/.gitignore = %q", got)
	}
	entries, err := os.ReadDir(cfg.Dir())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "build" && e.Name() != "claims" && e.Name() != "project.config.yaml" {
			t.Fatalf("check wrote %q at the project root", e.Name())
		}
	}
	// Outside a work tree the guard gives no verdict, and says so.
	if res.GitignoreCheck != check.GitignoreNotAWorkTree {
		t.Fatalf("GitignoreCheck = %q, want %q", res.GitignoreCheck, check.GitignoreNotAWorkTree)
	}
}
