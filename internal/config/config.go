// Package config loads and validates project.config.yaml — the single
// project-specific input that keeps this engine generic. Nothing in this
// package (or anywhere else in the engine) may hardcode a project name
// or module. Claim facets are engine-fixed: exactly contract and internals
// (NIT-20). Every other project-specific value comes from the Config
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

// DefaultMaxClaimBodyChars is the omitted-field default for
// Config.MaxClaimBodyChars: body + steps + rows cells, counted as
// Unicode code points. Derived with the module cap (NIT-14): ten claims of
// 2,000 characters is a 20,000-character full-module read, one OpenClaw
// bootstrap file.
const DefaultMaxClaimBodyChars = 2000

// DefaultMaxClaimSummaryChars is the omitted-field default for
// Config.MaxClaimSummaryChars. One line, counted as Unicode code points.
const DefaultMaxClaimSummaryChars = 200

// DefaultMaxClaimsPerModule is the hard default for
// Config.MaxClaimsPerModule when the field is omitted (NIT-14). Ten
// 200-character summaries are a 2,000-character module index, under
// Hermes's 2,200-character MEMORY.md; ten 2,000-character bodies are one
// 20,000-character OpenClaw bootstrap file. A project that already has
// larger modules sets max_claims_per_module in project.config.yaml.
const DefaultMaxClaimsPerModule = 10

// The brief caps (NIT-204). A brief is a prose document beside the claims —
// briefs/<folder>/<slug>.md — and these are its size ceilings, each an ERROR
// that `check` enforces through internal/briefs. They are defaults with a
// config override apiece, exactly like the claim caps above, and like those
// the override is the human's call: an agent that meets a cap splits or trims
// the brief and never raises the number unasked.
//
// 2,000 words per brief is the claim body budget restated in words: one brief
// is meant to be read in one sitting, the way one module is. Three images of
// at most 1 MiB each keeps a brief a document with figures rather than a
// gallery. Twelve briefs per folder and sixty in total keep the set small
// enough that `brief list` is a table of contents a human reads, not a search
// result. Images count toward neither the folder nor the total cap: those two
// count documents.
const (
	DefaultMaxBriefWords      = 2000
	DefaultMaxBriefImages     = 3
	DefaultMaxBriefImageBytes = 1 << 20
	DefaultMaxBriefsPerFolder = 12
	DefaultMaxBriefs          = 60
)

// removedOverviewFacet is the retired reserved facet name. Listing it in
// facets[] is refused; leftover claims with facet: overview fail id-shape
// like any other undeclared facet.
const removedOverviewFacet = "overview"

// Engine-fixed claim facets (NIT-20). project.config.yaml must list exactly
// these two names; no other facet is legal. Manifest is a viewer tab, not a
// claim facet — see internal/visibility.ViewerTabManifest.
const (
	FacetContract  = "contract"
	FacetInternals = "internals"
)

// EngineFacets is the only legal facets[] value, in viewer-peer order after
// Manifest.
func EngineFacets() []string {
	return []string{FacetContract, FacetInternals}
}

// IsEngineFacet reports whether name is contract or internals.
func IsEngineFacet(name string) bool {
	return name == FacetContract || name == FacetInternals
}

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

	// DeprecatedTheme is retained only to reject legacy configuration clearly.
	DeprecatedTheme yaml.Node `yaml:"theme,omitempty"`
}

// Conformance configures the project-owned normalized observation input. The
// engine resolves Observations relative to project.config.yaml but does not
// require it to exist at config-load time: an unreadable configured snapshot is
// an explicit uncheckable result, not a missing viewer.
type Conformance struct {
	Observations string `yaml:"observations,omitempty"`
	// Blocking turns unsatisfied compare checks into a check-command gate. It is
	// deliberately opt-in: the zero value preserves the report-only v1 behavior.
	Blocking bool `yaml:"blocking,omitempty"`
}

// UnmarshalYAML keeps the observation path textual. yaml.v3 otherwise coerces
// numbers and booleans into strings, which would turn a malformed declaration
// into a different filesystem lookup instead of rejecting the config.
func (c *Conformance) UnmarshalYAML(node *yaml.Node) error {
	n := deref(node)
	if n == nil || isNull(n) {
		return fmt.Errorf("conformance: expected a mapping, got null")
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("conformance: expected a mapping, got %s", nodeKindName(n))
	}
	*c = Conformance{}
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return fmt.Errorf("conformance: key on line %d is not a name", key.Line)
		}
		if key.Value != "observations" && key.Value != "blocking" {
			return fmt.Errorf("conformance: field %q not found", key.Value)
		}
		if seen[key.Value] {
			return fmt.Errorf("conformance: key %q is defined twice", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "observations":
			v := deref(value)
			if v == nil || v.Kind != yaml.ScalarNode || v.Tag != "!!str" {
				kind := "null"
				if v != nil {
					kind = nodeKindName(v)
				}
				return fmt.Errorf("conformance.observations: expected a string, got %s", kind)
			}
			c.Observations = v.Value
		case "blocking":
			v := deref(value)
			if v == nil || v.Kind != yaml.ScalarNode || v.Tag != "!!bool" {
				kind := "null"
				if v != nil {
					kind = nodeKindName(v)
				}
				return fmt.Errorf("conformance.blocking: expected a boolean, got %s", kind)
			}
			if err := v.Decode(&c.Blocking); err != nil {
				return fmt.Errorf("conformance.blocking: decode boolean: %w", err)
			}
		}
	}
	return nil
}

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
	Eyebrow   string   `yaml:"eyebrow,omitempty"`
	Facets    []string `yaml:"facets"`
	Modules   []string `yaml:"modules"`
	ClaimsDir string   `yaml:"claims_dir"`
	// Constitution is the project-root roof file, default constitution.yaml.
	// It is not a module and is never walked by LoadClaims.
	Constitution string `yaml:"constitution,omitempty"`
	// ProjectClaimsDir is the store for scope: project claims, default
	// project-claims. Outside claims_dir. Missing directory is empty, not an error.
	ProjectClaimsDir string `yaml:"project_claims_dir,omitempty"`

	// ManifestTree is claims_dir-relative slash paths to file bytes. When
	// non-nil, the module-manifest lint reads this tree instead of the
	// working-tree claims_dir. StatusStaged sets it from the git index so
	// --staged never judges an unstaged manifest. Not a config field.
	ManifestTree map[string][]byte `yaml:"-"`
	Viewer       Viewer            `yaml:"viewer,omitempty"`
	Conformance  Conformance       `yaml:"conformance,omitempty"`

	// ConstitutionIndex, when non-nil, is constitution.yaml as the git index
	// carries it. StatusStaged sets it so a lint that reads the roof's text
	// (shared-context-budget) judges the commit, not the working tree: the
	// same copy the constitution gate reads. Not a config field.
	ConstitutionIndex *IndexedFile `yaml:"-"`

	// BriefTree is briefs_dir files this run is judging, keyed by
	// config-relative slash path ("briefs/folder/slug.md"). check sets it
	// from the same Load / index tree the rest of the gate uses, so a cited
	// brief's pin is hashed from those bytes and not a leftover worktree
	// copy. Not a config field.
	BriefTree map[string][]byte `yaml:"-"`

	// BuildDir is the directory every runtime-generated file lives under —
	// the code-links artifacts, the three ledger stores, the
	// catalog and the viewer — one subdirectory per kind (see paths.go for the
	// layout). Optional; it defaults to "build" and, like ClaimsDir, is
	// resolved against the config file's own directory, never the process
	// cwd. Read it through BuildDirPath(), which is the resolved form; the
	// field keeps the YAML name and the accessor keeps the resolved value,
	// because Go refuses a struct with a field and a method of one name.
	//
	// It must not sit inside claims_dir, contain it, equal it, or equal the
	// config directory itself; DecodeConfig refuses every one of those AFTER
	// both paths are resolved (see the note on validate for why the rule
	// cannot live there).
	BuildDir string `yaml:"build_dir,omitempty"`

	// SourceDirs is the optional list of directories (relative to this
	// config file's own directory, like ClaimsDir) the engine scans for
	// "dossierx-claim: <id>" and "dossierx-step: <id> #<n> <hash>" comments —
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
	// anything, which almost certainly indicates a typo.
	MockupModules []string `yaml:"mockup_modules,omitempty"`

	// MaxClaimBodyChars is the project-wide ceiling on one claim's
	// body+steps+rows cells, counted as Unicode code points. Omit the
	// field to take DefaultMaxClaimBodyChars (2000). Zero and negatives
	// are refused at load time — they are not a "no cap" sentinel.
	MaxClaimBodyChars *int `yaml:"max_claim_body_chars,omitempty"`

	// MaxClaimSummaryChars is the project-wide ceiling on summary,
	// counted as Unicode code points. Omit the field to take
	// DefaultMaxClaimSummaryChars (200). Zero and negatives are refused
	// at load time.
	MaxClaimSummaryChars *int `yaml:"max_claim_summary_chars,omitempty"`

	// MaxClaimsPerModule is the project-wide ceiling on how many claim
	// files may sit in one module. Omit the field to take
	// DefaultMaxClaimsPerModule (10). There is no per-module override:
	// a fat corpus raises this one number. Zero and negatives are
	// refused at load time — they are not a "no cap" sentinel.
	MaxClaimsPerModule *int `yaml:"max_claims_per_module,omitempty"`

	// BriefsDir is the directory holding the project's briefs (NIT-204):
	// briefs/<folder>/<slug>.md, one folder level, beside claims_dir rather
	// than inside it. Optional; it defaults to "briefs" and is resolved against
	// the config file's own directory like every other path here. A directory
	// that does not exist is an empty set of briefs, not an error — most
	// projects never write one, and a project with no briefs must see no
	// change at all. Read it through BriefsDirPath().
	//
	// It is a separate tree and never an overlap: DecodeConfig refuses a
	// briefs_dir that is the config directory itself, or that sits inside or
	// contains claims_dir, project_claims_dir or build_dir. A brief is not a
	// claim (the claims loader would otherwise have to learn to skip it), and
	// the build directory is engine output.
	BriefsDir string `yaml:"briefs_dir,omitempty"`

	// The five brief cap overrides (see DefaultMaxBriefWords and its
	// siblings). Omit a field to take its default. Zero and negatives are
	// refused at load time — they are not a "no cap" sentinel. Every one of
	// them is set only on the human's explicit yes; the finding each cap
	// raises says so.
	MaxBriefWords      *int `yaml:"max_brief_words,omitempty"`
	MaxBriefImages     *int `yaml:"max_brief_images,omitempty"`
	MaxBriefImageBytes *int `yaml:"max_brief_image_bytes,omitempty"`
	MaxBriefsPerFolder *int `yaml:"max_briefs_per_folder,omitempty"`
	MaxBriefs          *int `yaml:"max_briefs,omitempty"`

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
	if strings.TrimSpace(cfg.Constitution) == "" {
		cfg.Constitution = DefaultConstitution
	}
	if !filepath.IsAbs(cfg.Constitution) {
		cfg.Constitution = filepath.Join(dir, cfg.Constitution)
	}
	if strings.TrimSpace(cfg.ProjectClaimsDir) == "" {
		cfg.ProjectClaimsDir = DefaultProjectClaimsDir
	}
	if !filepath.IsAbs(cfg.ProjectClaimsDir) {
		cfg.ProjectClaimsDir = filepath.Join(dir, cfg.ProjectClaimsDir)
	}
	if strings.TrimSpace(cfg.BriefsDir) == "" {
		cfg.BriefsDir = DefaultBriefsDir
	}
	if !filepath.IsAbs(cfg.BriefsDir) {
		cfg.BriefsDir = filepath.Join(dir, cfg.BriefsDir)
	}
	cfg.BriefsDir = filepath.Clean(cfg.BriefsDir)
	if strings.TrimSpace(cfg.BuildDir) == "" {
		cfg.BuildDir = DefaultBuildDir
	}
	if !filepath.IsAbs(cfg.BuildDir) {
		cfg.BuildDir = filepath.Join(dir, cfg.BuildDir)
	}
	cfg.BuildDir = filepath.Clean(cfg.BuildDir)
	if cfg.Viewer.TemplateOverrides != "" && !filepath.IsAbs(cfg.Viewer.TemplateOverrides) {
		cfg.Viewer.TemplateOverrides = filepath.Join(dir, cfg.Viewer.TemplateOverrides)
	}
	if cfg.Conformance.Observations != "" && !filepath.IsAbs(cfg.Conformance.Observations) {
		cfg.Conformance.Observations = filepath.Join(dir, cfg.Conformance.Observations)
	}
	for i, sd := range cfg.SourceDirs {
		if !filepath.IsAbs(sd) {
			cfg.SourceDirs[i] = filepath.Join(dir, sd)
		}
	}

	// THE CONTAINMENT RULE, evaluated only now that both paths are absolute
	// and cleaned. The engine's outputs may not sit inside the claims tree
	// (serve's watcher polls claims_dir and would re-render on every one of
	// its own writes, forever), the claims may not sit inside the build
	// directory (build/.gitignore and the printed recoveries treat everything
	// under it as engine output), and the two may not be one directory. The
	// build directory may not be the config directory either: then ViewerPath
	// is viewer/index.html — the legacy path — and check would write a file
	// that the legacy-layout refusal then refuses on every following command,
	// a loop with no exit.
	if err := checkBuildDirContainment(cfg.BuildDir, cfg.ClaimsDir, dir); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	if pathContains(cfg.ClaimsDir, filepath.Clean(cfg.Constitution)) {
		return nil, fmt.Errorf("config: %s: constitution (%s) must sit outside claims_dir (%s)", path, cfg.Constitution, cfg.ClaimsDir)
	}
	if pathContains(cfg.ClaimsDir, filepath.Clean(cfg.ProjectClaimsDir)) || pathContains(cfg.ProjectClaimsDir, cfg.ClaimsDir) {
		return nil, fmt.Errorf("config: %s: project_claims_dir (%s) must sit outside claims_dir (%s)", path, cfg.ProjectClaimsDir, cfg.ClaimsDir)
	}
	if err := checkBriefsDirContainment(&cfg, dir); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	if cfg.Conformance.Observations != "" {
		if pathContains(cfg.BuildDir, filepath.Clean(cfg.Conformance.Observations)) {
			return nil, fmt.Errorf("config: %s: conformance.observations (%s) must be outside build_dir (%s); generated output cannot be used as observation input", path, cfg.Conformance.Observations, cfg.BuildDir)
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

// validate checks the SHAPE of a decoded config: required fields, duplicates,
// membership, and retired viewer settings. It runs BEFORE DecodeConfig resolves the
// path-shaped fields, so inside it claims_dir, build_dir and every other path
// is still the raw YAML string — "claims/x" against "./claims/../x", or "." —
// and no comparison here can answer whether two of them overlap. Path
// RELATIONSHIPS (build_dir against claims_dir, either against the config
// directory) are therefore checked after resolution in DecodeConfig, and no
// future path relationship should be checked here either.
func (c *Config) validate() error {
	if c.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("unknown schema_version %d (engine supports %d)", c.SchemaVersion, CurrentSchemaVersion)
	}

	if err := validateEngineFacets(c.Facets); err != nil {
		return err
	}
	// Normalize declaration order so every loaded config agrees with the
	// viewer tab strip (Contract then Internals). YAML order is not a
	// project vocabulary.
	c.Facets = EngineFacets()

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

	if c.Viewer.DeprecatedTheme.Kind != 0 {
		return fmt.Errorf("viewer.theme is no longer supported; remove viewer.theme from project.config.yaml to use the built-in Light and Dark viewer themes")
	}

	if c.MaxClaimBodyChars != nil && *c.MaxClaimBodyChars < 1 {
		return fmt.Errorf("max_claim_body_chars must be >= 1 (got %d); omit the field for the default of %d", *c.MaxClaimBodyChars, DefaultMaxClaimBodyChars)
	}
	if c.MaxClaimSummaryChars != nil && *c.MaxClaimSummaryChars < 1 {
		return fmt.Errorf("max_claim_summary_chars must be >= 1 (got %d); omit the field for the default of %d", *c.MaxClaimSummaryChars, DefaultMaxClaimSummaryChars)
	}
	if c.MaxClaimsPerModule != nil && *c.MaxClaimsPerModule < 1 {
		return fmt.Errorf("max_claims_per_module must be >= 1 (got %d); omit the field for the default of %d", *c.MaxClaimsPerModule, DefaultMaxClaimsPerModule)
	}
	for _, limit := range []struct {
		key string
		v   *int
		def int
	}{
		{"max_brief_words", c.MaxBriefWords, DefaultMaxBriefWords},
		{"max_brief_images", c.MaxBriefImages, DefaultMaxBriefImages},
		{"max_brief_image_bytes", c.MaxBriefImageBytes, DefaultMaxBriefImageBytes},
		{"max_briefs_per_folder", c.MaxBriefsPerFolder, DefaultMaxBriefsPerFolder},
		{"max_briefs", c.MaxBriefs, DefaultMaxBriefs},
	} {
		if limit.v != nil && *limit.v < 1 {
			return fmt.Errorf("%s must be >= 1 (got %d); omit the field for the default of %d", limit.key, *limit.v, limit.def)
		}
	}

	return nil
}

// BriefCaps is the five effective brief ceilings, each the configured value
// when set and the default otherwise. It is one value rather than five
// accessors because every consumer — internal/briefs' cap rules, the render
// payload that shows each count against its cap — needs all five together,
// and a struct cannot be read with one of them forgotten.
type BriefCaps struct {
	Words      int `json:"words_per_brief"`
	Images     int `json:"images_per_brief"`
	ImageBytes int `json:"bytes_per_image"`
	PerFolder  int `json:"briefs_per_folder"`
	Total      int `json:"briefs_total"`
}

// BriefCapLimits returns the effective brief caps. A nil Config still returns
// every default, so a caller without a config does not silently drop a
// ceiling.
func (c *Config) BriefCapLimits() BriefCaps {
	pick := func(v *int, def int) int {
		if v == nil {
			return def
		}
		return *v
	}
	if c == nil {
		c = &Config{}
	}
	return BriefCaps{
		Words:      pick(c.MaxBriefWords, DefaultMaxBriefWords),
		Images:     pick(c.MaxBriefImages, DefaultMaxBriefImages),
		ImageBytes: pick(c.MaxBriefImageBytes, DefaultMaxBriefImageBytes),
		PerFolder:  pick(c.MaxBriefsPerFolder, DefaultMaxBriefsPerFolder),
		Total:      pick(c.MaxBriefs, DefaultMaxBriefs),
	}
}

// checkBriefsDirContainment is briefs_dir's resolved-path rule, evaluated with
// every path absolute and cleaned (see validate for why path relationships are
// never judged there). briefs_dir is a tree of its own: it may not be the
// config directory — every file in the project would then be a brief-shape
// refusal — it may not sit inside a .git directory, and it may not sit
// inside, or contain, claims_dir, project_claims_dir or build_dir. Each of those is walked by something else
// (the claims loader, the project-claims loader, nothing at all because it is
// engine output), and a brief that also lived in one of them would have two
// readers with two different ideas of what the file is.
func checkBriefsDirContainment(cfg *Config, configDir string) error {
	briefs := filepath.Clean(cfg.BriefsDir)
	if briefs == filepath.Clean(configDir) {
		return fmt.Errorf("briefs_dir (%s) is the config file's own directory; briefs need a directory of their own — leave briefs_dir unset (it defaults to briefs) or set it to a subdirectory", briefs)
	}
	// A .git directory is git's own store, never a project tree: briefs there
	// could never be committed, and every read would walk git's objects. Only
	// the part of the path the config names is judged — the elements below the
	// config directory, or climbing out of it — so a checkout that happens to
	// live under some ancestor named .git is not refused for it.
	if rel, err := filepath.Rel(filepath.Clean(configDir), briefs); err == nil {
		for _, elem := range strings.Split(filepath.ToSlash(rel), "/") {
			if elem == ".git" {
				return fmt.Errorf("briefs_dir (%s) is inside a .git directory, which is git's own store; set briefs_dir to a directory of the project", briefs)
			}
		}
	}
	for _, other := range []struct {
		key string
		dir string
	}{
		{"claims_dir", cfg.ClaimsDir},
		{"project_claims_dir", cfg.ProjectClaimsDir},
		{"build_dir", cfg.BuildDir},
	} {
		dir := filepath.Clean(other.dir)
		if pathContains(dir, briefs) || pathContains(briefs, dir) {
			return fmt.Errorf("briefs_dir (%s) and %s (%s) overlap; briefs sit beside the claims, never inside or around another tree the engine reads or writes", briefs, other.key, dir)
		}
	}
	return nil
}

// BriefsDirPath is the resolved, absolute briefs directory (default
// <config dir>/briefs). It may not exist; internal/briefs reads an absent
// directory as a project with no briefs.
func (c *Config) BriefsDirPath() string {
	if c == nil {
		return ""
	}
	return c.BriefsDir
}

// ClaimBodyCharLimit is the effective body+steps+rows cap: the configured
// value when set, otherwise DefaultMaxClaimBodyChars. A nil Config still
// returns the default so a lint called without a config does not silently
// drop the ceiling.
func (c *Config) ClaimBodyCharLimit() int {
	if c == nil || c.MaxClaimBodyChars == nil {
		return DefaultMaxClaimBodyChars
	}
	return *c.MaxClaimBodyChars
}

// ClaimSummaryCharLimit is the effective summary cap. A nil Config still
// returns DefaultMaxClaimSummaryChars.
func (c *Config) ClaimSummaryCharLimit() int {
	if c == nil || c.MaxClaimSummaryChars == nil {
		return DefaultMaxClaimSummaryChars
	}
	return *c.MaxClaimSummaryChars
}

// ClaimsPerModuleLimit is the effective module-size cap: the configured
// value when set, otherwise DefaultMaxClaimsPerModule. A nil Config
// still returns the default so a lint called without a config does not
// silently drop the ceiling.
func (c *Config) ClaimsPerModuleLimit() int {
	if c == nil || c.MaxClaimsPerModule == nil {
		return DefaultMaxClaimsPerModule
	}
	return *c.MaxClaimsPerModule
}

// checkBuildDirContainment is the resolved-path half of build_dir's
// validation: buildDir, claimsDir and configDir are absolute and cleaned, and
// the rule refuses buildDir inside claimsDir, claimsDir inside buildDir, the
// two equal, or buildDir equal to configDir. filepath.Rel is computed both
// ways; a result of "." or one that does not start with ".." means one
// contains the other.
func checkBuildDirContainment(buildDir, claimsDir, configDir string) error {
	buildDir = filepath.Clean(buildDir)
	claimsDir = filepath.Clean(claimsDir)
	configDir = filepath.Clean(configDir)
	if buildDir == configDir {
		return fmt.Errorf("build_dir (%s) is the config file's own directory; the engine's outputs need a directory of their own — leave build_dir unset (it defaults to build) or set it to a subdirectory", buildDir)
	}
	if pathContains(claimsDir, buildDir) || pathContains(buildDir, claimsDir) {
		return fmt.Errorf("build_dir (%s) and claims_dir (%s) overlap; the engine's outputs cannot sit inside the claims tree or contain it — move the claims under a subdirectory (FORMAT.md's claims_dir move procedure) or set build_dir to a path outside claims_dir", buildDir, claimsDir)
	}
	return nil
}

// pathContains reports whether child is dir itself or sits anywhere under it,
// both already absolute and cleaned.
func pathContains(dir, child string) bool {
	rel, err := filepath.Rel(dir, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validateEngineFacets(facets []string) error {
	if len(facets) == 0 {
		return fmt.Errorf("facets must be exactly %q and %q (engine-fixed)", FacetContract, FacetInternals)
	}
	seen := make(map[string]bool, len(facets))
	for i, f := range facets {
		if strings.TrimSpace(f) == "" {
			return fmt.Errorf("facets[%d] is empty", i)
		}
		if f == removedOverviewFacet {
			return fmt.Errorf("facets[%d] %q is not allowed: the reserved overview facet has been removed", i, f)
		}
		if !IsEngineFacet(f) {
			return fmt.Errorf("facets contains %q; the only legal facets are %q and %q", f, FacetContract, FacetInternals)
		}
		if seen[f] {
			return fmt.Errorf("facets contains duplicate %q", f)
		}
		seen[f] = true
	}
	if !seen[FacetContract] || !seen[FacetInternals] {
		return fmt.Errorf("facets must be exactly %q and %q (engine-fixed)", FacetContract, FacetInternals)
	}
	return nil
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

// IndexedFile is one file as the git index holds it. Tracked false means the
// index has no such file, which readers treat exactly like an absent file.
type IndexedFile struct {
	Tracked bool
	Raw     []byte
}

// ConstitutionPath is the resolved constitution.yaml path.
func (c *Config) ConstitutionPath() string {
	if c == nil {
		return ""
	}
	return c.Constitution
}

// ProjectClaimsDirPath is the resolved project-claims directory.
func (c *Config) ProjectClaimsDirPath() string {
	if c == nil {
		return ""
	}
	return c.ProjectClaimsDir
}
