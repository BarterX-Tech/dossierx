package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/manifest"
)

func writeProjectConfigFile(t *testing.T, cfgPath, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	if err := manifest.SeedMinimalFromConfigYAML(filepath.Dir(cfgPath), []byte(body)); err != nil {
		t.Fatalf("seed module manifests: %v", err)
	}
}
