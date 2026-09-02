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
	if cfg.HubGatingEnabled() {
		t.Errorf("HubGatingEnabled = true, want false (doctrine_facet unset)")
	}
	// doctrine_facet must stay exactly "" — never defaulted to a guess
	// (e.g. the first facet, or a facet named "doctrine" if one happens
	// to exist).
	if cfg.DoctrineFacet != "" {
		t.Errorf("DoctrineFacet = %q, want \"\" (must not be defaulted when omitted)", cfg.DoctrineFacet)
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

func TestLoadConfig_DoctrineFacetNotInFacets(t *testing.T) {
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
		t.Fatal("expected error for doctrine_facet not present in facets, got nil")
	}
	if !strings.Contains(err.Error(), "doctrine") {
		t.Errorf("expected error to name the unknown doctrine_facet value %q, got: %v", "doctrine", err)
	}
}

func TestLoadConfig_DoctrineFacetValid(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract, doctrine]
modules: [ledger]
claims_dir: claims
doctrine_facet: doctrine
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.HubGatingEnabled() {
		t.Errorf("HubGatingEnabled = false, want true")
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

func TestLoadConfig_ThemeUnknownKeyRejected(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
viewer:
  theme:
    accent: "#3fb950"
    not-a-real-token: "#ffffff"
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for unknown theme token, got nil")
	}
	if !strings.Contains(err.Error(), "not-a-real-token") {
		t.Errorf("expected error to name the unknown theme token, got: %v", err)
	}
}

func TestLoadConfig_ThemeEmptyValueRejected(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
viewer:
  theme:
    accent: ""
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for empty theme value, got nil")
	}
	if !strings.Contains(err.Error(), "accent") {
		t.Errorf("expected error to name the offending key accent, got: %v", err)
	}
}

func TestLoadConfig_ThemeDangerousCharRejected(t *testing.T) {
	cases := []struct {
		name string
		key  string
		val  string
	}{
		{"semicolon in color", "accent", "#3fb950; background:url(evil)"},
		{"angle bracket in color", "ink", "<script>"},
		{"brace in font", "font-sans", "Arial{}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
viewer:
  theme:
    `+tc.key+`: "`+tc.val+`"
`)
			_, err := LoadConfig(p)
			if err == nil {
				t.Fatalf("expected error for dangerous character in %s, got nil", tc.key)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("expected error to name the offending key %q, got: %v", tc.key, err)
			}
		})
	}
}

func TestLoadConfig_ThemeInvalidColorFormatRejected(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
viewer:
  theme:
    accent: "123"
`)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for invalid color format, got nil")
	}
	if !strings.Contains(err.Error(), "accent") {
		t.Errorf("expected error to name the offending key accent, got: %v", err)
	}
}

func TestLoadConfig_ThemeValidPartialMapAccepted(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract]
modules: [ledger]
claims_dir: claims
viewer:
  theme:
    accent: "#3fb950"
    accent-bg: "rgba(63,185,80,.14)"
    link: "cornflowerblue"
    font-sans: "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
    radius: "12px"
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	want := map[string]string{
		"accent":    "#3fb950",
		"accent-bg": "rgba(63,185,80,.14)",
		"link":      "cornflowerblue",
		"font-sans": "-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
		"radius":    "12px",
	}
	// Flat keys land in Shared: they apply in both colour schemes, which
	// is what every theme written before per-mode values existed meant.
	got := cfg.Viewer.Theme.Shared
	if len(got) != len(want) {
		t.Fatalf("Theme.Shared = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("Theme.Shared[%q] = %q, want %q", k, got[k], v)
		}
	}
	if len(cfg.Viewer.Theme.Light) != 0 || len(cfg.Viewer.Theme.Dark) != 0 {
		t.Errorf("a flat theme must produce no per-mode keys, got light=%v dark=%v",
			cfg.Viewer.Theme.Light, cfg.Viewer.Theme.Dark)
	}
}

func TestLoadConfig_ThemeEmptyMapOmitted(t *testing.T) {
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
	if !cfg.Viewer.Theme.IsZero() {
		t.Errorf("Theme = %+v, want the zero theme when viewer.theme is unset", cfg.Viewer.Theme)
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
