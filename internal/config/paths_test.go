package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuildDirPathHelpersResolveUnderTheBuildDirectory pins every generated
// path the engine writes to <config dir>/build/<kind>/<name>, through the ten
// accessors in paths.go, so a command, the check pipeline and serve cannot
// disagree about where a file lives.
func TestBuildDirPathHelpersResolveUnderTheBuildDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger, widget]
claims_dir: claims
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	build := filepath.Join(dir, "build")
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"BuildDirPath", cfg.BuildDirPath(), build},
		{"BuildOrderPath", cfg.BuildOrderPath("widget"), filepath.Join(build, "build-order", "widget.json")},
		{"CodeLinksPath", cfg.CodeLinksPath("widget"), filepath.Join(build, "code-links", "widget.json")},
		{"LockStorePath", cfg.LockStorePath(), filepath.Join(build, "ledger", "lock-store.json")},
		{"CommentDigestPath", cfg.CommentDigestPath(), filepath.Join(build, "ledger", "comment-digest.json")},
		{"FlagStorePath", cfg.FlagStorePath(), filepath.Join(build, "ledger", "flag-store.json")},
		{"CatalogPath", cfg.CatalogPath(), filepath.Join(build, "catalog", "catalog.json")},
		{"ViewerPath", cfg.ViewerPath(), filepath.Join(build, "viewer", "index.html")},
		{"ClaimsSentinelPath", cfg.ClaimsSentinelPath(), filepath.Join(build, "ledger", "claims")},
		{"BuildGitignorePath", cfg.BuildGitignorePath(), filepath.Join(build, ".gitignore")},
	}
	if len(cases) != 10 {
		t.Fatalf("paths.go declares ten accessors; this table covers %d", len(cases))
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
		if !filepath.IsAbs(tc.got) {
			t.Errorf("%s = %q is not absolute", tc.name, tc.got)
		}
	}
}

// TestTrackedStoreFileNamesAreTheLedgerBaseNames pins the slice
// tests/docs_site_audit_test.go derives the documented-store set from, and
// that each of its names is what the matching accessor writes.
func TestTrackedStoreFileNamesAreTheLedgerBaseNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", "schema_version: 1\nfacets: [contract]\nmodules: [m]\nclaims_dir: claims\n")
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := map[string]string{
		LockStoreFileName:     cfg.LockStorePath(),
		CommentDigestFileName: cfg.CommentDigestPath(),
		FlagStoreFileName:     cfg.FlagStorePath(),
	}
	if len(TrackedStoreFileNames) != len(want) {
		t.Fatalf("TrackedStoreFileNames = %v, want the %d ledger stores", TrackedStoreFileNames, len(want))
	}
	for _, name := range TrackedStoreFileNames {
		path, ok := want[name]
		if !ok {
			t.Errorf("TrackedStoreFileNames names %q, which no ledger accessor writes", name)
			continue
		}
		if filepath.Base(path) != name {
			t.Errorf("accessor for %q writes %q", name, path)
		}
	}
	for _, display := range []string{LockStoreDisplayPath, CommentDigestDisplayPath, FlagStoreDisplayPath} {
		if _, ok := want[filepath.Base(display)]; !ok {
			t.Errorf("display path %q does not end in a tracked store name", display)
		}
	}
}
