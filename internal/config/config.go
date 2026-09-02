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

// Track is one declared cross-cutting concern: a named grouping claims may
// join from any module. See Config.Tracks for why the registry is explicit,
// and model.TrackRef for the claim-side membership this declares the
// vocabulary for.
type Track struct {
	// ID is the stable identifier claims cite in their `tracks:` list and
	// the CLI takes as an argument ("dossierx track show <id>"). Required,
	// unique within the project.
	ID string `yaml:"id"`

	// Title is the human-readable name rendered in the viewer's sidebar and
	// at the head of the track's page. Required: a track exists to be read
	// about by someone asking "what does the user get", and an id is not an
	// answer to that question.
	Title string `yaml:"title"`

	// Summary is an optional one-line description of what the track covers,
	// rendered under the Title. Optional because a well-named track with an
	// owned claim already says what it is — the owned claim's body IS the
	// long form.
	Summary string `yaml:"summary,omitempty"`
}

// Viewer holds viewer/render related configuration.
type Viewer struct {
	// TemplateOverrides is a directory of partial template overrides,
	// resolved relative to the config file's directory. Missing
	// individual partial files inside it fall back to engine defaults
	// per-component (soft fallback). If set but the directory itself does
	// not exist, LoadConfig returns a hard error.
	TemplateOverrides string `yaml:"template_overrides,omitempty"`

	// Theme is the viewer's custom-theme block: an optional preset, an
	// optional theme file to extend, CSS custom-property values that apply
	// to both colour schemes or to only one, and project-supplied font
	// faces. See the Theme type for the shape and theme.go for the decoder.
	//
	// Token names (without the leading "--") must be drawn from
	// ThemeTokenAllowlist — this is the only project-specific CSS
	// vocabulary the engine recognizes, kept as a fixed list for typo
	// protection. Values are injected verbatim into generated stylesheet
	// blocks by internal/render, so they are validated as hostile input
	// (see theme_validate.go) rather than trusted as safe CSS.
	//
	// A theme that names only flat token keys — every project that
	// predates per-mode values, this engine's own fixtures included —
	// merges to exactly the single :root{...} block it produced before.
	Theme Theme `yaml:"theme,omitempty"`
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

	// Added for the custom-theme work: every consumer below is a literal
	// that internal/render/viewer/template/style.css turns into
	// var(--<token>, <the same literal>), so an unset token renders
	// exactly as it did before this list grew. Order is load-bearing —
	// internal/render emits declarations in it — so new tokens are
	// appended, never inserted.
	"code-inline-bg",
	"code-bg",
	"table-head-bg",
	"image-bg",
	"hover-bg",
	"border-strong",
	"shadow",
	"shadow-strong",
	"shadow-cast",
	"scrim",
	"selection-bg",
	"status-draft",
	"status-draft-bg",
	"mockup-bg",
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

	// All fourteen tokens added for the custom-theme work are colours;
	// font-sans, font-mono and radius remain the only non-colour tokens.
	"code-inline-bg":  true,
	"code-bg":         true,
	"table-head-bg":   true,
	"image-bg":        true,
	"hover-bg":        true,
	"border-strong":   true,
	"shadow":          true,
	"shadow-strong":   true,
	"shadow-cast":     true,
	"scrim":           true,
	"selection-bg":    true,
	"status-draft":    true,
	"status-draft-bg": true,
	"mockup-bg":       true,
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

	// Tracks is the project's declared registry of cross-cutting concerns —
	// the second axis claims may join, orthogonal to Modules. See
	// model.TrackRef for the axis itself.
	//
	// It is declared here, and not inferred from whatever ids claims happen
	// to mention, for the same reason Modules is: a vocabulary that creates
	// itself on first use cannot catch a typo, and "checkout" vs "check-out"
	// would silently become two features nobody notices are one. A claim
	// naming a track absent from this list is a lint error (track-unknown).
	//
	// Optional. A project that declares none behaves exactly as it did
	// before tracks existed — every track-* lint is a no-op, and the viewer
	// renders no Tracks group.
	Tracks []Track `yaml:"tracks,omitempty"`

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
	// author a claim carrying RawHTML AT ALL — on any layout, not only
	// model.LayoutMockup. The NAME PREDATES v0.4.1, which made raw_html an
	// attachment legal beside card, banner, list and tree content; the gate
	// widened with it and the field's name did not, so a reader who takes
	// this for a mockup-only allowlist will expect a `card` claim bearing
	// markup to be ungated, and it is not. It is the "module allowlist" leg
	// of the raw-html-scope lint's five-part gate (see
	// internal/lint/raw_html_scope.go). It is optional: a project that has
	// never authored a raw_html claim need not set it, and the lint treats
	// an unset/empty list as "no module may author one", not a vacuous
	// pass. Every entry must also appear in Modules — an
	// allowlisted module that isn't even a project module can never gate
	// anything, which almost certainly indicates a typo (same reasoning as
	// DoctrineFacet's membership check below).
	MockupModules []string `yaml:"mockup_modules,omitempty"`

	// dir is the absolute directory containing the config file itself;
	// ClaimsDir and Viewer.TemplateOverrides are resolved against it, never
	// against the process's current working directory. Unexported so it
	// can't be set directly from YAML.
	dir string

	// path is the absolute path of the config file this was loaded from, ""
	// for a config decoded from bytes with no file behind it. It exists for
	// "check --staged", which has to look this exact file up in the git index
	// and cannot assume it is named FileName — --config accepts any path.
	// Unexported so it can't be set directly from YAML.
	path string
}

// Dir returns the absolute directory the config file lives in.
func (c *Config) Dir() string { return c.dir }

// Path returns the absolute path of the config file this was loaded from, or ""
// when it was decoded from bytes. Callers that need to find the SAME file
// somewhere else (the git index, above all) must use this rather than assuming
// Dir()+FileName: --config takes an arbitrary path, and a project whose config
// is named something else would otherwise be looked up as a file that is not
// there — which, for a gate, means silently falling back to weaker evidence.
func (c *Config) Path() string { return c.path }

// FileName is the project config's fixed filename. The upward search in
// cmd/dossierx and the index lookup in internal/check both name it, so it is a
// constant rather than a literal repeated at each site.
const FileName = "project.config.yaml"

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
	cfg, err := DecodeConfig(raw, filepath.Dir(absPath), path)
	if err != nil {
		return nil, err
	}
	cfg.path = absPath
	return cfg, nil
}

// DecodeConfig is LoadConfig with the bytes already in hand and the anchor
// directory supplied separately.
//
// It exists for "dossierx check --staged", which has to evaluate the project
// against the config THE INDEX HOLDS while still resolving claims_dir and the
// stores against the real working-tree directory — the index's copy of the file
// has no directory of its own to be relative to. Splitting the read from the
// decode is what keeps that caller on this exact strict-decode-and-validate
// path instead of growing a second, drifting copy of it.
//
// name is used only in error messages, so a caller reading from somewhere other
// than the filesystem can still say which file it means.
func DecodeConfig(raw []byte, dir, name string) (*Config, error) {
	path := name

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

	// The theme's two path-shaped fields are resolved here and READ
	// NOWHERE IN THIS FUNCTION. Loading a config stays a pure function of
	// its bytes plus the directory it was anchored to, which is what lets
	// "check --staged" decode the index's copy of project.config.yaml and
	// then resolve the theme against the index's copy of every file it
	// names, instead of half of each.
	if err := resolveThemePaths(&cfg.Viewer.Theme, dir); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
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

	// tracks is optional, but each declared entry must be usable: an id to
	// cite, a title to read, and no two entries competing for the same id.
	// Same reasoning as modules above — this is the registry a claim's
	// membership is checked against, so a malformed entry would weaken every
	// track-* lint rather than just itself.
	trackIDs := make([]string, 0, len(c.Tracks))
	for i, t := range c.Tracks {
		if strings.TrimSpace(t.ID) == "" {
			return fmt.Errorf("tracks[%d].id is empty", i)
		}
		if strings.TrimSpace(t.Title) == "" {
			return fmt.Errorf("tracks[%d] (%q) has no title", i, t.ID)
		}
		trackIDs = append(trackIDs, t.ID)
	}
	if dup, ok := firstDuplicate(trackIDs); ok {
		return fmt.Errorf("tracks contains duplicate id %q", dup)
	}

	// Shape, allowlist and grammar only: nothing here reads a file. The
	// theme file named by `extends` and every `fonts[].src` are read later,
	// by ValidateTheme/ResolveTheme, through an injected reader.
	if err := validateThemeBlock(&c.Viewer.Theme, "viewer.theme"); err != nil {
		return err
	}

	return nil
}

// TrackIDs returns every declared track id, in declaration order. Callers
// that need to test membership of a claim-supplied id (the track-unknown
// lint, the CLI's track leaves) use HasTrack instead.
func (c *Config) TrackIDs() []string {
	ids := make([]string, 0, len(c.Tracks))
	for _, t := range c.Tracks {
		ids = append(ids, t.ID)
	}
	return ids
}

// HasTrack reports whether id names a track this project declares.
func (c *Config) HasTrack(id string) bool {
	for _, t := range c.Tracks {
		if t.ID == id {
			return true
		}
	}
	return false
}

// TrackByID returns the declared track with the given id and whether it was
// found. The zero Track is returned when it was not.
func (c *Config) TrackByID(id string) (Track, bool) {
	for _, t := range c.Tracks {
		if t.ID == id {
			return t, true
		}
	}
	return Track{}, false
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
