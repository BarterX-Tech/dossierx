package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

const (
	projectClaimYAML = "id: project.scope\nscope: project\nstatus: draft\nbody: the roof's first claim\nrests_on:\n  none: true\n  reason: fixture\n"
	moduleClaimYAML  = "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: claim a\nrests_on:\n  - project.scope\n"
)

// writeProjectConfig writes a config whose claims_dir holds one module claim
// and returns the loaded config. extra is appended verbatim, so a test can
// point project_claims_dir wherever it needs to.
func writeProjectConfig(t *testing.T, root, extra string) (*config.Config, error) {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, claimsDir, "a.yaml", moduleClaimYAML)
	cfgPath := writeFile(t, root, "project.config.yaml",
		"schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"+extra)
	return config.LoadConfig(cfgPath)
}

func TestLoadProjectClaims_StoreAbsentIsEmptyNotAnError(t *testing.T) {
	got, err := LoadProjectClaims(filepath.Join(t.TempDir(), "project-claims"))
	if err != nil || len(got) != 0 {
		t.Fatalf("a missing project-claims store is an empty store: got %v, %v", got, err)
	}
	got, err = LoadProjectClaims("")
	if err != nil || len(got) != 0 {
		t.Fatalf("an empty path is an empty store: got %v, %v", got, err)
	}
}

func TestLoadProjectClaims_StorePresent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project-claims")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "scope.yaml", projectClaimYAML)
	writeFile(t, filepath.Join(dir, "nested"), "retention.yml", strings.Replace(projectClaimYAML, "project.scope", "project.retention", 1))
	writeFile(t, dir, "README.md", "not a claim")

	got, err := LoadProjectClaims(dir)
	if err != nil {
		t.Fatalf("LoadProjectClaims: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected the two yaml files, got %d: %+v", len(got), got)
	}
	for _, c := range got {
		if !c.IsProjectClaim() || c.Module != "" || c.Facet != "" {
			t.Fatalf("a project-claims store node must load as a project claim with no module or facet: %+v", c)
		}
		if !strings.HasPrefix(c.SourcePath, dir) {
			t.Fatalf("SourcePath %q is not under the store", c.SourcePath)
		}
	}
	if got[0].ID != "project.retention" || got[1].ID != "project.scope" {
		t.Fatalf("expected SourcePath order (nested/retention.yml before scope.yaml), got %s, %s", got[0].ID, got[1].ID)
	}
}

func TestLoadProjectClaims_NotADirectoryIsAnError(t *testing.T) {
	root := t.TempDir()
	p := writeFile(t, root, "project-claims", "a file where the store should be")
	if _, err := LoadProjectClaims(p); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("a file at the store path must be refused, got %v", err)
	}
}

func TestLoadProjectClaims_MalformedFileIsAnError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "project-claims")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "bad.yaml", "id: project.scope\nscope: project\nunknown_field: true\n")
	if _, err := LoadProjectClaims(dir); err == nil {
		t.Fatal("an unknown field in a project claim must be a hard error, exactly as in claims_dir")
	}
}

func TestLoadAll_MergesBothStoresSortedBySourcePath(t *testing.T) {
	root := t.TempDir()
	cfg, err := writeProjectConfig(t, root, "")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	store := filepath.Join(root, "project-claims")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, store, "scope.yaml", projectClaimYAML)

	got, err := LoadAll(cfg)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(got) != 2 || got[0].ID != "widget.contract.a" || got[1].ID != "project.scope" {
		t.Fatalf("expected [widget.contract.a project.scope] (claims/ sorts before project-claims/), got %+v", got)
	}
	if _, ok := FindByID(got, "project.scope"); !ok {
		t.Fatal("the merged set must resolve the project claim by id")
	}
	// The module claim's rests_on target exists only because the stores were
	// merged: dangling must stay silent on the merged set.
	for _, f := range (lint.DanglingLint{}).Check(got, cfg) {
		t.Fatalf("dangling fired on a merged set whose target lives in the project store: %+v", f)
	}
}

func TestLoadAll_StoreAbsentLoadsModuleClaimsOnly(t *testing.T) {
	cfg, err := writeProjectConfig(t, t.TempDir(), "")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	got, err := LoadAll(cfg)
	if err != nil {
		t.Fatalf("LoadAll without a project-claims store: %v", err)
	}
	if len(got) != 1 || got[0].ID != "widget.contract.a" {
		t.Fatalf("expected the one module claim, got %+v", got)
	}
}

func TestLoadAll_NilConfigIsAnError(t *testing.T) {
	if _, err := LoadAll(nil); err == nil {
		t.Fatal("LoadAll(nil) must refuse rather than read from an empty path")
	}
}

// A project-claims store inside claims_dir (or wrapping it) is refused at
// config load, so LoadAll can never be handed one: LoadClaims would otherwise
// walk the project store as module claims, and id-shape would refuse every
// project.<slug> file it found there.
func TestLoadAll_StoreInsideClaimsDirIsRefusedByConfig(t *testing.T) {
	for _, tc := range []struct{ name, extra string }{
		{"store under claims_dir", "project_claims_dir: claims/project\n"},
		{"store equal to claims_dir", "project_claims_dir: claims\n"},
		{"store wrapping claims_dir", "project_claims_dir: .\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := writeProjectConfig(t, t.TempDir(), tc.extra)
			if err == nil || !strings.Contains(err.Error(), "project_claims_dir") || !strings.Contains(err.Error(), "must sit outside claims_dir") {
				t.Fatalf("expected the containment refusal, got %v", err)
			}
		})
	}
}

func TestLoadAll_DuplicateIDAcrossStoresIsVisibleToLint(t *testing.T) {
	root := t.TempDir()
	cfg, err := writeProjectConfig(t, root, "")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	store := filepath.Join(root, "project-claims")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, store, "scope.yaml", projectClaimYAML)
	// The same id authored a second time, in the other store.
	writeFile(t, filepath.Join(root, "claims"), "scope-twin.yaml", projectClaimYAML)

	got, err := LoadAll(cfg)
	if err != nil {
		t.Fatalf("LoadAll must load both copies and leave the verdict to lint: %v", err)
	}
	n := 0
	for _, c := range got {
		if c.ID == "project.scope" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected both copies of project.scope in the merged set, got %d of %d claims", n, len(got))
	}
	hits := 0
	for _, f := range (lint.AmbiguousLint{}).Check(got, cfg) {
		if f.ClaimID == "project.scope" {
			hits++
		}
	}
	if hits != 2 {
		t.Fatalf("ambiguous must report every claim sharing the id across stores, got %d finding(s)", hits)
	}
}

func mustParse(t *testing.T, raw, sourcePath string) model.Claim {
	t.Helper()
	c, err := ParseClaim([]byte(raw), sourcePath)
	if err != nil {
		t.Fatalf("parse %s: %v", sourcePath, err)
	}
	return c
}

func TestMergeClaims(t *testing.T) {
	moduleA := mustParse(t, moduleClaimYAML, "/p/claims/a.yaml")
	moduleZ := mustParse(t, strings.Replace(moduleClaimYAML, "widget.contract.a", "widget.contract.z", 1), "/p/claims/z.yaml")
	project := mustParse(t, projectClaimYAML, "/p/project-claims/scope.yaml")
	early := mustParse(t, strings.Replace(projectClaimYAML, "project.scope", "project.early", 1), "/p/aaa-store/early.yaml")

	got := MergeClaims([]model.Claim{moduleZ, moduleA}, []model.Claim{project, early})
	want := []string{"project.early", "widget.contract.a", "widget.contract.z", "project.scope"}
	if len(got) != len(want) {
		t.Fatalf("merged %d claims, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("merged[%d] = %s, want %s (sorted by SourcePath across both stores)", i, got[i].ID, id)
		}
	}
	if merged := MergeClaims(nil, nil); merged == nil || len(merged) != 0 {
		t.Fatalf("MergeClaims(nil, nil) must be an empty, non-nil slice, got %#v", merged)
	}
	if merged := MergeClaims(nil, []model.Claim{project}); len(merged) != 1 || merged[0].ID != "project.scope" {
		t.Fatalf("a project-only corpus merges to itself, got %+v", merged)
	}
}
