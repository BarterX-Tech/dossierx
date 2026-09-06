package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendGitHubEnvAppendsResolvedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "github-env")
	if err := os.WriteFile(path, []byte("existing=value\n"), 0o644); err != nil {
		t.Fatalf("seed environment file: %v", err)
	}
	lines := []string{
		"DOSSIERX_CURRENT_VERSION=0.7.8",
		"DOSSIERX_CURRENT_COMMIT=current",
		"DOSSIERX_PREV_RELEASE_TAG=v0.7.7",
		"DOSSIERX_PREV_RELEASE_COMMIT=baseline",
	}
	if err := appendGitHubEnv(path, lines); err != nil {
		t.Fatalf("appendGitHubEnv: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read environment file: %v", err)
	}
	want := "existing=value\nDOSSIERX_CURRENT_VERSION=0.7.8\nDOSSIERX_CURRENT_COMMIT=current\nDOSSIERX_PREV_RELEASE_TAG=v0.7.7\nDOSSIERX_PREV_RELEASE_COMMIT=baseline\n"
	if string(got) != want {
		t.Fatalf("environment file = %q, want %q", got, want)
	}
}
