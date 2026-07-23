// Package config loads and validates project.config.yaml — the single
// project-specific input that keeps this engine generic. Nothing in this
// package (or anywhere else in the engine) may hardcode a project name,
// facet, or module; every project-specific value comes from the Config
// this package produces.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CurrentSchemaVersion is the only schema_version this engine build
// understands. LoadConfig refuses to run against any other value.
const CurrentSchemaVersion = 1

// ReservedOverviewFacet is the one facet name every module gets
// automatically, without a project listing it in Facets: claims under
// module.overview.* are module-level orientation notes (see
// model.Claim.EffectiveKind), injected into every one of that module's
// other facet tabs by internal/render rather than getting their own tab.
// It deliberately does not need to appear in Facets — validate() below
// never checks it, and internal/lint.IDShapeLint treats it as always
// valid regardless of what a project declares.
const ReservedOverviewFacet = "overview"

// ErrNotFound is wrapped into LoadConfig's returned error whenever the
// config file itself does not exist at the given path (as opposed to
// existing but being malformed or invalid). Callers (notably the CLI) use
// errors.Is(err, ErrNotFound) to distinguish "nothing there" from other
// load failures and react accordingly (e.g. a distinct exit code).
var ErrNotFound = errors.New("config file not found")

// Viewer holds viewer/render related configuration.
type Viewer struct {
	// TemplateOverrides is a directory of partial template overrides,
	// resolved relative to the config file's directory. Missing
	// individual partial files inside it fall back to engine defaults
	// per-component (soft fallback). If set but the directory itself does
	// not exist, LoadConfig returns a hard error.
	TemplateOverrides string `yaml:"template_overrides,omitempty"`

	// Theme maps CSS custom-property token names (without the leading
	// "--") to their values. Keys must be drawn from ThemeTokenAllowlist —
	// this is the only project-specific CSS vocabulary the engine
	// recognizes, kept as a fixed list for typo protection. Values are
	// injected verbatim into a generated :root{...} stylesheet block, so
	// they are validated defensively (see validateTheme) rather than
	// trusted as safe CSS.
	Theme map[string]string `yaml:"theme,omitempty"`
}

// ThemeTokenAllowlist is the fixed, engine-owned set of viewer.theme keys.
// Any key in viewer.theme not present here is a load-time error. This list
// is intentionally the only place that defines the engine's theme
// vocabulary; internal/render's CSS-emitting helper iterates it (in this
// order) to keep output deterministic.
var ThemeTokenAllowlist = []string{
	"accent",
	"accent-bg",
	"ink",
	"muted",
	"faint",
	"paper",
	"card-bg",
	"border",
	"link",
	"warn",
	"warn-bg",
	"font-sans",
	"font-mono",
	"radius",
}

// themeColorTokens is the subset of ThemeTokenAllowlist that holds
// color-shaped values and gets a light format sanity check on top of the
// dangerous-character rejection applied to every token.
var themeColorTokens = map[string]bool{
	"accent":    true,
	"accent-bg": true,
	"ink":       true,
	"muted":     true,
	"faint":     true,
	"paper":     true,
	"card-bg":   true,
	"border":    true,
	"link":      true,
	"warn":      true,
	"warn-bg":   true,
}

// themeTokenAllowed is ThemeTokenAllowlist as a set, built once for O(1)
// membership checks.
var themeTokenAllowed = func() map[string]bool {
	m := make(map[string]bool, len(ThemeTokenAllowlist))
	for _, k := range ThemeTokenAllowlist {
		m[k] = true
	}
	return m
}()

// Config is the fully-decoded, fully-validated project.config.yaml.
type Config struct {
	SchemaVersion int `yaml:"schema_version"`
	// Title is the project's display name, used as the viewer's <title>,
	// header, and sidebar heading. Optional; internal/render falls back to
	// a generic default ("dossierx viewer") when unset, so existing configs
	// that predate this field keep working unchanged.
	Title string `yaml:"title,omitempty"`
	// Eyebrow is an optional one-line subtitle rendered directly under the
	// title in the sidebar header (e.g. "user-intelligence service"),
	// mirroring the reference docs explainer page's .eyebrow line. Unset means no
	// eyebrow line is rendered at all — it is not required the way Title's
	// generic fallback is.
	Eyebrow       string   `yaml:"eyebrow,omitempty"`
	Facets        []string `yaml:"facets"`
	Modules       []string `yaml:"modules"`
	ClaimsDir     string   `yaml:"claims_dir"`
	DoctrineFacet string   `yaml:"doctrine_facet,omitempty"`
	Viewer        Viewer   `yaml:"viewer,omitempty"`

	// SourceDirs is the optional list of directories (relative to this
	// config file's own directory, like ClaimsDir) the engine scans for
	// "dossierx-claim: <id>" comments — the code side of internal/implink's
	// claim-to-code linking. Unset/empty means "do not scan" — "dossierx
	// check" behaves exactly as it did before this field existed, the same
	// zero-cost-when-unused contract every other optional feature in this
	// engine follows (mockup_modules, viewer.template_overrides, ...). A
	// project only opts in by naming its actual source roots, same as it
	// only opts into claim data via ClaimsDir — the engine never assumes
	// or guesses where "the code" is.
	SourceDirs []string `yaml:"source_dirs,omitempty"`

	// MockupModules is the checked-in allowlist of modules permitted to
	// author layout: mockup claims (model.LayoutMockup, model.Claim.RawHTML)
	// — the "module allowlist" leg of the raw-html-scope lint's five-part
	// gate (see internal/lint/raw_html_scope.go). It is optional: a project
	// that has never authored a mockup claim need not set it, and the lint
	// treats an unset/empty list as "no module may author layout: mockup",
	// not a vacuous pass. Every entry must also appear in Modules — an
	// allowlisted module that isn't even a project module can never gate
	// anything, which almost certainly indicates a typo (same reasoning as
	// DoctrineFacet's membership check below).
	MockupModules []string `yaml:"mockup_modules,omitempty"`

	// dir is the absolute directory containing the config file itself;
	// ClaimsDir and Viewer.TemplateOverrides are resolved against it, never
	// against the process's current working directory. Unexported so it
	// can't be set directly from YAML.
	dir string
}

// Dir returns the absolute directory the config file lives in.
func (c *Config) Dir() string { return c.dir }

// LoadConfig reads, strictly decodes, and validates the project config at
// path. "Strict" means an unknown YAML field is a hard error, not silently
// ignored. All path-shaped fields (claims_dir, viewer.template_overrides)
// are resolved relative to path's own directory, never the process cwd.
func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("config: %s: %w (%w)", path, ErrNotFound, err)
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("config: resolve absolute path for %s: %w", path, err)
	}
	dir := filepath.Dir(absPath)

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	cfg.dir = dir

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}

	// Resolve path-shaped fields against the config file's own directory.
	if !filepath.IsAbs(cfg.ClaimsDir) {
		cfg.ClaimsDir = filepath.Join(dir, cfg.ClaimsDir)
	}
	if cfg.Viewer.TemplateOverrides != "" && !filepath.IsAbs(cfg.Viewer.TemplateOverrides) {
		cfg.Viewer.TemplateOverrides = filepath.Join(dir, cfg.Viewer.TemplateOverrides)
	}
	for i, sd := range cfg.SourceDirs {
		if !filepath.IsAbs(sd) {
			cfg.SourceDirs[i] = filepath.Join(dir, sd)
		}
	}

	// A configured-and-missing override directory is a hard load-time
	// error (per SPEC); missing individual partials inside it are fine and
	// are handled later by internal/render, not here.
	if cfg.Viewer.TemplateOverrides != "" {
		info, err := os.Stat(cfg.Viewer.TemplateOverrides)
		if err != nil {
			return nil, fmt.Errorf("config: viewer.template_overrides %q: %w", cfg.Viewer.TemplateOverrides, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("config: viewer.template_overrides %q is not a directory", cfg.Viewer.TemplateOverrides)
		}
	}

	// A configured-and-missing source_dirs entry is a hard load-time error,
	// same as viewer.template_overrides above: a project that names a
	// source root gets an early, clear failure rather than "dossierx check"
	// silently scanning zero files and reporting nothing.
	for _, sd := range cfg.SourceDirs {
		info, err := os.Stat(sd)
		if err != nil {
			return nil, fmt.Errorf("config: source_dirs %q: %w", sd, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("config: source_dirs %q is not a directory", sd)
		}
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("unknown schema_version %d (engine supports %d)", c.SchemaVersion, CurrentSchemaVersion)
	}

	if len(c.Facets) == 0 {
		return fmt.Errorf("facets must be non-empty")
	}
	if dup, ok := firstDuplicate(c.Facets); ok {
		return fmt.Errorf("facets contains duplicate %q", dup)
	}
	for i, f := range c.Facets {
		if strings.TrimSpace(f) == "" {
			return fmt.Errorf("facets[%d] is empty", i)
		}
	}

	if len(c.Modules) == 0 {
		return fmt.Errorf("modules must be non-empty")
	}
	if dup, ok := firstDuplicate(c.Modules); ok {
		return fmt.Errorf("modules contains duplicate %q", dup)
	}
	for i, m := range c.Modules {
		if strings.TrimSpace(m) == "" {
			return fmt.Errorf("modules[%d] is empty", i)
		}
	}

	if strings.TrimSpace(c.ClaimsDir) == "" {
		return fmt.Errorf("claims_dir must be set")
	}

	// doctrine_facet is optional; when set, it must be a facet this project
	// actually declares (an unknown doctrine facet can never gate anything,
	// which almost certainly indicates a typo rather than intent).
	if c.DoctrineFacet != "" && !contains(c.Facets, c.DoctrineFacet) {
		return fmt.Errorf("doctrine_facet %q is not in facets", c.DoctrineFacet)
	}

	if dup, ok := firstDuplicate(c.MockupModules); ok {
		return fmt.Errorf("mockup_modules contains duplicate %q", dup)
	}
	for i, m := range c.MockupModules {
		if strings.TrimSpace(m) == "" {
			return fmt.Errorf("mockup_modules[%d] is empty", i)
		}
		if !contains(c.Modules, m) {
			return fmt.Errorf("mockup_modules[%d] %q is not in modules", i, m)
		}
	}

	if err := validateTheme(c.Viewer.Theme); err != nil {
		return fmt.Errorf("viewer.theme: %w", err)
	}

	return nil
}

// dangerousThemeChars are rejected outright from any theme token value
// regardless of the token's shape (color or font-family), since they are
// the actual injection-safety concern: theme values are interpolated
// verbatim into a generated <style> block by internal/render.
const dangerousThemeChars = ";{}<>"

// validateTheme enforces the viewer.theme contract: keys must be in
// ThemeTokenAllowlist (typo protection — this allowlist is the only engine-
// owned theme vocabulary), values must be non-empty, and values must not
// contain characters that would be unsafe to interpolate into CSS. Color-
// shaped tokens additionally get a light format sanity check.
func validateTheme(theme map[string]string) error {
	for _, key := range ThemeTokenAllowlist {
		val, ok := theme[key]
		if !ok {
			continue
		}
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("%q: value must not be empty", key)
		}
		if strings.ContainsAny(val, dangerousThemeChars) {
			return fmt.Errorf("%q: value %q contains a disallowed character (one of %q)", key, val, dangerousThemeChars)
		}
		if themeColorTokens[key] && !looksLikeColor(val) {
			return fmt.Errorf("%q: value %q does not look like a color (#hex, rgb()/rgba(), or a CSS named color)", key, val)
		}
	}
	for key := range theme {
		if !themeTokenAllowed[key] {
			return fmt.Errorf("unknown theme token %q (must be one of %s)", key, strings.Join(ThemeTokenAllowlist, ", "))
		}
	}
	return nil
}

// looksLikeColor is a light sanity check, not a full CSS color grammar: it
// accepts #hex (3/4/6/8 digits), rgb(...)/rgba(...), and bare CSS named
// colors (a run of letters, e.g. "rebeccapurple"). Its job is to catch
// obvious typos/garbage, not to validate every legal CSS color syntax.
func looksLikeColor(val string) bool {
	v := strings.TrimSpace(val)
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "#") {
		hex := v[1:]
		switch len(hex) {
		case 3, 4, 6, 8:
			for _, r := range hex {
				if !isHexDigit(r) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "rgb(") || strings.HasPrefix(lower, "rgba(") {
		return strings.HasSuffix(v, ")")
	}
	// Bare CSS named color: letters only (e.g. "forestgreen", "transparent").
	for _, r := range v {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func firstDuplicate(ss []string) (string, bool) {
	seen := make(map[string]bool, len(ss))
	for _, s := range ss {
		if seen[s] {
			return s, true
		}
		seen[s] = true
	}
	return "", false
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// HubGatingEnabled reports whether doctrine hub-gating logic should run at
// all. When false, callers must skip the check entirely rather than treat
// it as a vacuous pass.
func (c *Config) HubGatingEnabled() bool {
	return c.DoctrineFacet != ""
}
