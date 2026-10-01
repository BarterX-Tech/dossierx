package check

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// briefAssetTree writes a project with one brief referencing flow.png and
// returns its config, loaded through the real decoder, and the viewer dir.
func briefAssetTree(t *testing.T) (cfg *config.Config, root, viewerDir string) {
	t.Helper()
	root = t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("briefs/flow/overview.md", "---\nsummary: The flow.\n---\n![Flow](flow.png)\n")
	write("briefs/flow/flow.png", "png-bytes")
	cfg, err := config.DecodeConfig([]byte("schema_version: 1\nfacets: [contract, internals]\nmodules: [widget]\nclaims_dir: claims\n"), root, "project.config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return cfg, root, filepath.Join(root, "build", "viewer")
}

// TestWriteBriefAssets_CopiesWhatThePageReferences is the static build
// carrying a brief's image (NIT-197): the file lands at the relative path the
// page's <img> names, and a copy left by an earlier build that no brief
// references any more is gone.
func TestWriteBriefAssets_CopiesWhatThePageReferences(t *testing.T) {
	cfg, _, viewerDir := briefAssetTree(t)
	stale := filepath.Join(viewerDir, "brief-assets", "old", "gone.png")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeBriefAssets(cfg, briefs.Load(cfg), viewerDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(viewerDir, "brief-assets", "flow", "flow.png"))
	if err != nil || string(got) != "png-bytes" {
		t.Fatalf("the referenced image must be copied beside the viewer: %q, %v", got, err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an image no brief references must not survive a rebuild: %v", err)
	}
}

// TestWriteBriefAssets_RefusesAnImageThatChangedUnderIt: discovery read a
// plain file of some size; a link or a different size at copy time is refused
// with the brief-relative path named, never copied through.
func TestWriteBriefAssets_RefusesAnImageThatChangedUnderIt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, img, root string)
	}{
		{"swapped for a symlink", func(t *testing.T, img, root string) {
			secret := filepath.Join(root, "secret.png")
			if err := os.WriteFile(secret, []byte("png-bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(img); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(secret, img); err != nil {
				t.Skipf("symlinks unavailable on this platform: %v", err)
			}
		}},
		{"its folder swapped for a symlink", func(t *testing.T, img, root string) {
			outside := filepath.Join(root, "outside")
			if err := os.MkdirAll(outside, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outside, "flow.png"), []byte("png-bytes"), 0o644); err != nil {
				t.Fatal(err)
			}
			folder := filepath.Dir(img)
			if err := os.RemoveAll(folder); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, folder); err != nil {
				t.Skipf("symlinks unavailable on this platform: %v", err)
			}
		}},
		{"a different size", func(t *testing.T, img, _ string) {
			if err := os.WriteFile(img, []byte("a much larger image than was counted"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, root, viewerDir := briefAssetTree(t)
			set := briefs.Load(cfg)
			tc.mutate(t, filepath.Join(root, "briefs", "flow", "flow.png"), root)
			err := writeBriefAssets(cfg, set, viewerDir)
			if err == nil || !strings.Contains(err.Error(), "briefs/flow/flow.png") {
				t.Fatalf("expected a refusal naming briefs/flow/flow.png, got %v", err)
			}
			if strings.Contains(err.Error(), root) {
				t.Fatalf("the refusal must name the brief-relative path, not the absolute one: %v", err)
			}
			if _, err := os.Stat(filepath.Join(viewerDir, "brief-assets", "flow", "flow.png")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("nothing may be copied through a refused source: %v", err)
			}
		})
	}
}
