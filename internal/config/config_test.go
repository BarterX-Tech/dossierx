package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract, internals]
modules: [ledger]
claims_dir: claims
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := filepath.Join(dir, "claims")
	if cfg.ClaimsDir != want {
		t.Errorf("ClaimsDir = %q, want %q", cfg.ClaimsDir, want)
	}
	if !strings.HasSuffix(cfg.Constitution, DefaultConstitution) {
		t.Errorf("Constitution = %q, want default %q", cfg.Constitution, DefaultConstitution)
	}
	if !strings.HasSuffix(cfg.ProjectClaimsDir, DefaultProjectClaimsDir) {
		t.Errorf("ProjectClaimsDir = %q, want default %q", cfg.ProjectClaimsDir, DefaultProjectClaimsDir)
	}
}

func TestLoadConfig_ClaimsDirResolvedAgainstConfigNotCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
`)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	otherDir := t.TempDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Logf("restore cwd: %v", err)
		}
	}()

	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := filepath.Join(dir, "claims")
	if cfg.ClaimsDir != want {
		t.Errorf("ClaimsDir = %q, want %q (must resolve against config dir, not cwd)", cfg.ClaimsDir, want)
	}
}

func TestLoadConfig_UnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 99
facets: [contract]
modules: [ledger]
claims_dir: claims
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for unknown schema_version, got nil")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("expected error to name the offending schema_version 99, got: %v", err)
	}
}

func TestLoadConfig_EmptyFacets(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: []
modules: [ledger]
claims_dir: claims
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for empty facets, got nil")
	}
	if !strings.Contains(err.Error(), "facets") {
		t.Errorf("expected error to name facets as the empty field, got: %v", err)
	}
}

func TestLoadConfig_OverviewFacetRefused(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract, overview]
modules: [ledger]
claims_dir: claims
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for reserved overview facet, got nil")
	}
	if !strings.Contains(err.Error(), "overview") || !strings.Contains(err.Error(), "removed") {
		t.Errorf("expected error to refuse overview as removed, got: %v", err)
	}
}

func TestLoadConfig_DuplicateFacets(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract, contract]
modules: [ledger]
claims_dir: claims
`)
	err := func() error { _, err := LoadConfig(p); return err }()
	if err == nil {
		t.Fatal("expected error for duplicate facets, got nil")
	}
	if !strings.Contains(err.Error(), "contract") || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected error to name the duplicate facet %q, got: %v", "contract", err)
	}
}

func TestLoadConfig_EmptyModules(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: []
claims_dir: claims
`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for empty modules, got nil")
	}
}

func TestLoadConfig_MissingClaimsDir(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for missing claims_dir, got nil")
	}
	if !strings.Contains(err.Error(), "claims_dir") {
		t.Errorf("expected error to name the missing field claims_dir, got: %v", err)
	}
}

func TestLoadConfig_UnknownField(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
totally_unknown_field: true
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected strict-decode error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "totally_unknown_field") {
		t.Errorf("expected error to name the unknown field totally_unknown_field, got: %v", err)
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("expected error to name the config file path %q, got: %v", p, err)
	}
}

func TestLoadConfig_UnknownDoctrineFacetFieldRefused(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
doctrine_facet: doctrine
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for retired doctrine_facet field, got nil")
	}
	if !strings.Contains(err.Error(), "doctrine_facet") {
		t.Errorf("expected error to name doctrine_facet, got: %v", err)
	}
}

func TestLoadConfig_TemplateOverridesMissingDirIsHardError(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
viewer:
  template_overrides: does-not-exist
`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for missing template_overrides dir, got nil")
	}
}

func TestLoadConfig_TemplateOverridesValidDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "overrides"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
viewer:
  template_overrides: overrides
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := filepath.Join(dir, "overrides")
	if cfg.Viewer.TemplateOverrides != want {
		t.Errorf("TemplateOverrides = %q, want %q", cfg.Viewer.TemplateOverrides, want)
	}
}

func TestLoadConfig_SourceDirsMissingDirIsHardError(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
source_dirs:
  - does-not-exist
`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for a missing source_dirs entry, got nil")
	}
}

func TestLoadConfig_SourceDirsValidDirsResolvedAbsolute(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src", "widget"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
source_dirs:
  - src
  - src/widget
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := []string{filepath.Join(dir, "src"), filepath.Join(dir, "src", "widget")}
	if len(cfg.SourceDirs) != len(want) {
		t.Fatalf("SourceDirs = %v, want %v", cfg.SourceDirs, want)
	}
	for i := range want {
		if cfg.SourceDirs[i] != want[i] {
			t.Errorf("SourceDirs[%d] = %q, want %q", i, cfg.SourceDirs[i], want[i])
		}
	}
}

func TestLoadConfig_SourceDirsUnsetIsFine(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.SourceDirs) != 0 {
		t.Errorf("expected no SourceDirs when unset, got %v", cfg.SourceDirs)
	}
}

func TestLoadConfig_CustomThemesRemoved(t *testing.T) {
	for _, body := range []string{"{}", "null", "{preset: claude}", "{extends: themes/house.yaml}", "{paper: '#fff'}", "{fonts: []}", "invalid"} {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			raw := "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nviewer:\n  theme: " + body + "\n"
			_, err := DecodeConfig([]byte(raw), dir, "project.config.yaml")
			if err == nil || !strings.Contains(err.Error(), "viewer.theme is no longer supported; remove viewer.theme") {
				t.Fatalf("legacy theme %s: %v", body, err)
			}
		})
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	missing := "/nonexistent/project.config.yaml"
	_, err := LoadConfig(missing)
	if err == nil {
		t.Fatal("expected error for nonexistent config file, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected errors.Is(err, ErrNotFound) to hold so callers (the CLI) can react distinctly, got: %v", err)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("expected error to name the exact missing path %q, got: %v", missing, err)
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	// A zero-byte project.config.yaml is a distinct edge case from
	// "facets: []" (TestLoadConfig_EmptyFacets): it is an empty YAML
	// document, which yaml.v3's Decoder reports as io.EOF rather than
	// decoding to a zero-value Config. This must still surface as a clear,
	// file-naming parse error — not a panic, and not silently treated as
	// ErrNotFound (the file does exist).
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", "")
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for a completely empty config file, got nil")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("expected error to name the config file path %q, got: %v", p, err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("an empty (but existing) file must not be reported as ErrNotFound, got: %v", err)
	}
}

func TestLoadConfig_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", "not: [valid: yaml")
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("expected error to name the config file path %q, got: %v", p, err)
	}
	if !strings.Contains(err.Error(), "line") {
		t.Errorf("expected error to name a parse position (line N), got: %v", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("malformed YAML must not be reported as ErrNotFound (file exists, just doesn't parse), got: %v", err)
	}
}

// ---------------------------------------------------------------------
// build_dir: the directory every runtime-generated file lives under
// ---------------------------------------------------------------------

func TestLoadConfig_BuildDirDefaultsToBuildUnderConfigDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
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
		t.Fatalf("LoadConfig: %v", err)
	}
	if got, want := cfg.BuildDirPath(), filepath.Join(dir, "build"); got != want {
		t.Fatalf("BuildDirPath() = %q, want %q (build_dir unset must default to build beside the config)", got, want)
	}
}

func TestLoadConfig_BuildDirRelativeResolvesAgainstConfigNotCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
build_dir: out/dossierx
`)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	otherDir := t.TempDir()
	if err := os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatal(err)
		}
	}()

	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := filepath.Join(dir, "out", "dossierx")
	if cfg.BuildDirPath() != want {
		t.Fatalf("BuildDirPath() = %q, want %q (resolved against the config's directory, not cwd %s)", cfg.BuildDirPath(), want, otherDir)
	}
	if strings.HasPrefix(cfg.BuildDirPath(), otherDir) {
		t.Fatalf("BuildDirPath() %q resolved against the process cwd", cfg.BuildDirPath())
	}
}

func TestLoadConfig_ConformanceObservationCannotAliasGeneratedOutput(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, observation := range []string{"build/observations.json", "build/catalog/catalog.json", "build/conformance/status.json", "build/viewer/index.html"} {
		t.Run(observation, func(t *testing.T) {
			p := writeConfig(t, dir, "project.config.yaml", "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nconformance:\n  observations: "+observation+"\n")
			_, err := LoadConfig(p)
			if err == nil || !strings.Contains(err.Error(), "outside build_dir") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfig_ConformanceObservationsRequiresExplicitYAMLString(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"number":   "123",
		"boolean":  "true",
		"null":     "null",
		"mapping":  "{path: observations.json}",
		"sequence": "[observations.json]",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			body := "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nconformance:\n  observations: " + value + "\n"
			_, err := LoadConfig(writeConfig(t, dir, "project.config.yaml", body))
			if err == nil || !strings.Contains(err.Error(), "conformance.observations: expected a string") {
				t.Fatalf("error = %v", err)
			}
		})
	}

	cfg, err := LoadConfig(writeConfig(t, dir, "project.config.yaml", "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nconformance:\n  observations: \"123\"\n"))
	if err != nil {
		t.Fatalf("quoted string rejected: %v", err)
	}
	if got, want := cfg.Conformance.Observations, filepath.Join(dir, "123"); got != want {
		t.Fatalf("observations = %q, want %q", got, want)
	}
}

func TestLoadConfig_ConformanceBlockingDefaultsFalseAndAcceptsExplicitBoolean(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\n"

	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{name: "conformance omitted", body: base, want: false},
		{name: "blocking omitted", body: base + "conformance:\n  observations: observations.json\n", want: false},
		{name: "explicit false", body: base + "conformance:\n  blocking: false\n", want: false},
		{name: "explicit true", body: base + "conformance:\n  blocking: true\n", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeConfig(t, dir, tc.name+".yaml", tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Conformance.Blocking != tc.want {
				t.Fatalf("conformance.blocking = %v, want %v", cfg.Conformance.Blocking, tc.want)
			}
		})
	}
}

func TestLoadConfig_ConformanceBlockingRequiresExplicitYAMLBoolean(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nconformance:\n  blocking: "
	for name, value := range map[string]string{
		"quoted":   `"true"`,
		"number":   "1",
		"null":     "null",
		"mapping":  "{enabled: true}",
		"sequence": "[true]",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, dir, name+".yaml", base+value+"\n"))
			if err == nil || !strings.Contains(err.Error(), "conformance.blocking: expected a boolean") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLoadConfig_ConformanceKeysRejectDuplicatesIndependently(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nconformance:\n"
	for name, fields := range map[string]string{
		"observations": "  observations: one.json\n  observations: two.json\n",
		"blocking":     "  blocking: true\n  blocking: false\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, dir, name+".yaml", base+fields))
			if err == nil || !strings.Contains(err.Error(), `key "`+name+`" is defined twice`) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

// TestLoadConfig_BuildDirInsideClaimsDirIsRefused walks every overlap the
// containment rule refuses, plus one accepted row so the table is not vacuous.
// The rule runs after resolution (see the note on validate), which is what
// lets the traversal and absolute rows be judged at all.
func TestLoadConfig_BuildDirInsideClaimsDirIsRefused(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"claims", filepath.Join("claims", "x"), filepath.Join("build", "claims")} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	absInsideClaims := filepath.Join(dir, "claims", "out")

	cases := []struct {
		name      string
		claimsDir string
		buildDir  string
		refused   bool
	}{
		{"build_dir equal to claims_dir", "claims", "claims", true},
		{"build_dir under claims_dir", "claims", "claims/x", true},
		{"claims_dir under build_dir", "build/claims", "build", true},
		{"absolute build_dir inside claims_dir", "claims", absInsideClaims, true},
		{"traversal that lands inside claims_dir", "claims", "./claims/../claims/out", true},
		{"build_dir is the config directory", "claims", ".", true},
		{"accepted: claims_dir claims, build_dir out", "claims", "out", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: " + tc.claimsDir + "\nbuild_dir: " + tc.buildDir + "\n"
			p := writeConfig(t, dir, "project.config.yaml", body)
			cfg, err := LoadConfig(p)
			if !tc.refused {
				if err != nil {
					t.Fatalf("expected the config to load, got: %v", err)
				}
				if got, want := cfg.BuildDirPath(), filepath.Join(dir, tc.buildDir); got != want {
					t.Fatalf("BuildDirPath() = %q, want %q", got, want)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected build_dir %q against claims_dir %q to be refused, got BuildDirPath %q", tc.buildDir, tc.claimsDir, cfg.BuildDirPath())
			}
			if !strings.Contains(err.Error(), "build_dir") {
				t.Fatalf("the refusal must name build_dir, got: %v", err)
			}
		})
	}
}
