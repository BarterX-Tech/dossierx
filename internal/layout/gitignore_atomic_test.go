package layout

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/atomicfile"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

func TestConformanceGitignoreInterruptedTransitionPreservesBytesAndRetries(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{BuildDir: filepath.Join(dir, "build")}
	path := cfg.BuildGitignorePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(BuildGitignoreContent), 0o644); err != nil {
		t.Fatal(err)
	}

	originalWriter := writeBuildGitignore
	defer func() { writeBuildGitignore = originalWriter }()
	writeBuildGitignore = func(string, []byte, os.FileMode) error { return errors.New("injected interruption") }
	if err := EnsureBuildGitignoreForConformance(cfg, true); err == nil {
		t.Fatal("expected injected write failure")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != BuildGitignoreContent {
		t.Fatalf("interrupted transition changed bytes: %q err=%v", got, err)
	}

	writeBuildGitignore = atomicfile.Write
	if err := EnsureBuildGitignoreForConformance(cfg, true); err != nil {
		t.Fatalf("retry enable: %v", err)
	}
	if err := EnsureBuildGitignoreForConformance(cfg, false); err != nil {
		t.Fatalf("retry disable: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != BuildGitignoreContent {
		t.Fatalf("disable bytes = %q err=%v", got, err)
	}
}
