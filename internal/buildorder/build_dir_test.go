package buildorder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
)

// TestArtifactPathLandsUnderBuildDirAndIsCreatedOnFirstWrite pins the
// build/ layout for this kind: the artifact is build/build-order/<module>.json
// under the config's directory, and the first write from an empty project
// directory creates that directory itself.
func TestArtifactPathLandsUnderBuildDirAndIsCreatedOnFirstWrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	path := ArtifactPath(cfg, "widget")
	if want := filepath.Join(dir, "build", "build-order", "widget.json"); path != want {
		t.Fatalf("ArtifactPath = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "build")); !os.IsNotExist(err) {
		t.Fatalf("precondition: build/ must not exist before the first write (stat err=%v)", err)
	}
	if err := WriteArtifact(&Artifact{Module: "widget"}, path); err != nil {
		t.Fatalf("WriteArtifact from an empty project: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the artifact was not written under build/build-order/: %v", err)
	}
}
