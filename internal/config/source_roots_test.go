package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initPinnedRepo(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "fixture@example.invalid")
	runGit(t, dir, "config", "user.name", "fixture")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-qm", "fixture")
	return strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func siblingProject(t *testing.T) (corpus, app, sha string) {
	t.Helper()
	parent := t.TempDir()
	app = filepath.Join(parent, "app")
	corpus = filepath.Join(parent, "corpus")
	if err := os.MkdirAll(filepath.Join(corpus, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	sha = initPinnedRepo(t, app)
	return corpus, app, sha
}

func TestLoadConfig_SourceRootsSiblingResolvesCommit(t *testing.T) {
	corpus, app, sha := siblingProject(t)
	p := writeConfig(t, corpus, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
source_roots:
  - path: ../app
    repo: example/app
    ref: `+sha+`
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.ScansSource() {
		t.Fatal("expected ScansSource with source_roots")
	}
	if len(cfg.SourceRoots) != 1 {
		t.Fatalf("SourceRoots = %d", len(cfg.SourceRoots))
	}
	got := cfg.SourceRoots[0]
	if got.Path != "../app" {
		t.Errorf("Path = %q", got.Path)
	}
	if got.AbsPath() != filepath.Clean(app) {
		t.Errorf("AbsPath = %q, want %q", got.AbsPath(), app)
	}
	if got.Commit() != sha {
		t.Errorf("Commit = %q, want %q", got.Commit(), sha)
	}
}

func TestLoadConfig_SourceRootsRefMismatchIsHardError(t *testing.T) {
	corpus, _, _ := siblingProject(t)
	p := writeConfig(t, corpus, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
source_roots:
  - path: ../app
    repo: example/app
    ref: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef
`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected ref mismatch or unresolved ref to fail")
	}
}

func TestLoadConfig_SourceDirsEscapingProjectIsHardError(t *testing.T) {
	corpus, _, _ := siblingProject(t)
	p := writeConfig(t, corpus, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
source_dirs:
  - ../app
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected escaping source_dirs to fail")
	}
	if !strings.Contains(err.Error(), "source_roots") {
		t.Fatalf("error should point at source_roots, got %v", err)
	}
}

func TestLoadConfig_SourceRootsMissingFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
source_roots:
  - path: src
    repo: example/app
`)
	if _, err := LoadConfig(p); err == nil || !strings.Contains(err.Error(), "ref") {
		t.Fatalf("expected empty ref error, got %v", err)
	}
}

func TestResolveLinkFile_AllowsDeclaredSibling(t *testing.T) {
	corpus, app, sha := siblingProject(t)
	src := filepath.Join(app, "main.go")
	if err := os.WriteFile(src, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, corpus, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
source_roots:
  - path: ../app
    repo: example/app
    ref: `+sha+`
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	abs, recorded, root, err := cfg.ResolveLinkFile("../app/main.go")
	if err != nil {
		t.Fatalf("ResolveLinkFile: %v", err)
	}
	if abs != src {
		t.Errorf("abs = %q, want %q", abs, src)
	}
	if recorded != "../app/main.go" {
		t.Errorf("recorded = %q", recorded)
	}
	if root == nil || root.Repo != "example/app" {
		t.Fatalf("root = %+v", root)
	}
}

func TestResolveLinkFile_StillRefusesUndeclaredEscape(t *testing.T) {
	cfg := mustLoadBare(t)
	outside := filepath.Join(filepath.Dir(cfg.Dir()), "outside.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := cfg.ResolveLinkFile("../outside.txt"); err == nil {
		t.Fatal("expected undeclared escape to fail")
	}
}

func mustLoadBare(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
