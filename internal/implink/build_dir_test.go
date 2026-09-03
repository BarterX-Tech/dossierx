package implink

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArtifactPathLandsUnderBuildDirAndIsCreatedOnFirstWrite pins the
// build/ layout for code links: build/code-links/<module>.json, created on
// the first write from an empty project directory.
func TestArtifactPathLandsUnderBuildDirAndIsCreatedOnFirstWrite(t *testing.T) {
	cfg := testConfig(t, "widget")
	path := ArtifactPath(cfg, "widget")
	if want := filepath.Join(cfg.Dir(), "build", "code-links", "widget.json"); path != want {
		t.Fatalf("ArtifactPath = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Join(cfg.Dir(), "build")); !os.IsNotExist(err) {
		t.Fatalf("precondition: build/ must not exist before the first write (stat err=%v)", err)
	}
	if err := WriteArtifact(&Artifact{Module: "widget"}, path); err != nil {
		t.Fatalf("WriteArtifact from an empty project: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the artifact was not written under build/code-links/: %v", err)
	}
}
