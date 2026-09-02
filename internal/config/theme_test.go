package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// themeConfig wraps a viewer.theme body in the minimum valid project config.
func themeConfig(themeBody string) string {
	return "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nviewer:\n  theme:\n" + themeBody
}

// loadTheme decodes a config carrying the given viewer.theme body, anchored
// at dir. It uses DecodeConfig rather than LoadConfig so a case can supply
// its own anchor directory without writing a file.
func loadTheme(t *testing.T, dir, themeBody string) (*Config, error) {
	t.Helper()
	return DecodeConfig([]byte(themeConfig(themeBody)), dir, "project.config.yaml")
}

// -------------------------------------------------------------------------
// §1.2 — the decoder contract, one case per row of the behaviour table.
// -------------------------------------------------------------------------

func TestThemeDecoder_BehaviourTable(t *testing.T) {
	cases := []struct {
		name string
		// body is the whole config, not just the theme block, because
		// several rows need `theme:` itself to hold a non-mapping.
		body    string
		wantErr string // "" means the config must load
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name: "theme null is no theme",
			body: "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nviewer:\n  theme:\n",
			check: func(t *testing.T, cfg *Config) {
				if !cfg.Viewer.Theme.IsZero() {
					t.Errorf("theme = %+v, want zero", cfg.Viewer.Theme)
				}
			},
		},
		{
			name:    "theme empty string",
			body:    "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nviewer:\n  theme: ''\n",
			wantErr: `viewer.theme: expected a mapping, got scalar`,
		},
		{
			name:    "theme sequence",
			body:    "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nviewer:\n  theme: []\n",
			wantErr: `viewer.theme: expected a mapping, got sequence`,
		},
		{
			name:    "theme number",
			body:    "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nviewer:\n  theme: 5\n",
			wantErr: `viewer.theme: expected a mapping, got scalar`,
		},
		{
			name: "theme is an alias to a mapping",
			body: "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\n" +
				"x-house: &house\n  accent: '#C6613F'\nviewer:\n  theme: *house\n",
			wantErr: `field x-house not found`, // strict decode rejects the anchor holder itself
		},
		{
			name: "aliased sub-mapping is accepted",
			body: "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\n" +
				"viewer:\n  theme:\n    light: &pal\n      accent: '#C6613F'\n    dark: *pal\n",
			check: func(t *testing.T, cfg *Config) {
				if got := cfg.Viewer.Theme.Dark["accent"]; got != "#C6613F" {
					t.Errorf("dark.accent = %q via alias, want #C6613F", got)
				}
			},
		},
		{
			name: "light null is an empty map",
			body: themeConfig("    light:\n"),
			check: func(t *testing.T, cfg *Config) {
				if cfg.Viewer.Theme.Light == nil || len(cfg.Viewer.Theme.Light) != 0 {
					t.Errorf("light = %v, want an empty map", cfg.Viewer.Theme.Light)
				}
			},
		},
		{
			name:    "light sequence",
			body:    themeConfig("    light: []\n"),
			wantErr: `viewer.theme.light: expected a mapping of token names to values, got sequence`,
		},
		{
			name:    "light scalar",
			body:    themeConfig("    light: hello\n"),
			wantErr: `viewer.theme.light: expected a mapping of token names to values, got scalar`,
		},
		{
			name:    "duplicate key at the top level",
			body:    themeConfig("    paper: '#fff'\n    paper: '#eee'\n"),
			wantErr: `viewer.theme: key "paper" is defined twice (lines 7 and 8)`,
		},
		{
			name:    "duplicate key inside light",
			body:    themeConfig("    light:\n      paper: '#fff'\n      paper: '#eee'\n"),
			wantErr: `viewer.theme.light: key "paper" is defined twice (lines 8 and 9)`,
		},
		{
			name: "merge key directly under theme",
			body: "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\n" +
				"viewer:\n  theme:\n    <<: {accent: '#C6613F'}\n",
			wantErr: `viewer.theme: YAML merge keys (<<) are not supported here`,
		},
		{
			name:    "merge key inside a fonts entry",
			body:    themeConfig("    fonts:\n      - <<: {family: A}\n        src: a.woff2\n"),
			wantErr: `viewer.theme.fonts[0]: YAML merge keys (<<) are not supported here`,
		},
		{
			name: "merge key inside a mode",
			body: "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\n" +
				"viewer:\n  theme:\n    light:\n      accent: '#C6613F'\n    dark:\n      <<: {accent: '#D97757'}\n",
			wantErr: `viewer.theme.dark: YAML merge keys (<<) are not supported here`,
		},
		{
			name:    "fonts scalar",
			body:    themeConfig("    fonts: hello\n"),
			wantErr: `viewer.theme.fonts: expected a list of font entries, got scalar`,
		},
		{
			name:    "empty fonts item",
			body:    themeConfig("    fonts:\n      -\n"),
			wantErr: `viewer.theme.fonts[0]: entry is empty`,
		},
		{
			name: "unknown fonts key",
			body: themeConfig("    fonts:\n      - {family: A, src: a.woff2}\n" +
				"      - {family: B, src: b.woff2, wieght: '400'}\n"),
			wantErr: `viewer.theme.fonts[1]: unknown field "wieght" (allowed: family, src, weight, style)`,
		},
		{
			name:    "non-scalar token value",
			body:    themeConfig("    accent: [a, b]\n"),
			wantErr: `viewer.theme.accent (line 7): expected a scalar value, got sequence`,
		},
		{
			name:    "null token value quotes the fix",
			body:    themeConfig("    accent: #C6613F\n"),
			wantErr: `viewer.theme.accent (line 7): value must not be empty (a bare #hex is a YAML comment; quote it: '#FAF9F5')`,
		},
		{
			name:    "boolean is rejected by the colour grammar",
			body:    themeConfig("    accent: true\n"),
			wantErr: `does not look like a colour`,
		},
		{
			name:    "unitless radius",
			body:    themeConfig("    radius: 10\n"),
			wantErr: `is not a CSS length`,
		},
		{
			name:    "block scalar with a newline",
			body:    themeConfig("    font-sans: |\n      Arial\n      Helvetica\n"),
			wantErr: `contains a control character`,
		},
		{
			name:    "reserved word as a token name under a mode",
			body:    themeConfig("    light:\n      fonts: '#fff'\n"),
			wantErr: `"fonts" is a reserved word of the theme block`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg, err := DecodeConfig([]byte(tc.body), dir, "project.config.yaml")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("DecodeConfig: %v", err)
				}
				tc.check(t, cfg)
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v\nwant it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// -------------------------------------------------------------------------
// §1.3 — validation classes.
// -------------------------------------------------------------------------

func TestThemeColourGrammar(t *testing.T) {
	accept := []string{
		"#fff", "#ffff", "#FAF9F5", "#FAF9F5CC",
		"rgb(1, 2, 3)", "rgba(217, 119, 87, 0.14)", "rgba(127,127,127,.12)",
		"hsl(210 50% 40%)", "hsla(210, 50%, 40%, 0.5)",
		"lab(52.2% 40.1 59.9)", "lch(52.2% 72.2 56.1)",
		"oklab(0.7 0.1 0.1)", "oklch(0.7 0.1 30)",
		"color(display-p3 1 0.5 0)",
		"color-mix(in srgb, red 58%, transparent)",
		"transparent", "currentcolor", "CurrentColor",
		"rebeccapurple", "cornflowerblue", "Red",
	}
	for _, v := range accept {
		t.Run("accept/"+v, func(t *testing.T) {
			if err := checkThemeChars("t", v); err != nil {
				t.Fatalf("universal checks rejected %q: %v", v, err)
			}
			if err := checkThemeColor("t", v); err != nil {
				t.Errorf("checkThemeColor(%q) = %v, want nil", v, err)
			}
		})
	}

	reject := []struct {
		val  string
		want string
	}{
		// var() is NOT on the allowed function list even though every
		// character inside it is legal: a token whose meaning depends on
		// a property the engine cannot see is not a colour this engine
		// can validate or a reader can rely on.
		{"color-mix(in srgb, var(--x) 58%, transparent)", "var(), which is not allowed"},
		{"url(evil.png)", "url(), which is not allowed"},
		{"red/*", "CSS comment delimiter"},
		{`"unbalanced`, "unbalanced double quote"},
		{"'unbalanced", "unbalanced single quote"},
		{"red\nblue", "control character"},
		// F1: these are ACCEPTED by every shape rule below once trimmed,
		// which is exactly why they must be refused before the trim can
		// happen: the bytes emitted into the stylesheet are the bytes
		// written here, not a cleaned copy of them.
		{"red ", "leading or trailing whitespace"},
		{" red", "leading or trailing whitespace"},
		{"#FAF9F5\t", "control character"},
		{"red\u00a0", "non-ASCII whitespace"},     // NBSP
		{"\u00a0red", "non-ASCII whitespace"},     // NBSP
		{"red\u2028blue", "non-ASCII whitespace"}, // U+2028 LINE SEPARATOR
		{"red\u2029", "non-ASCII whitespace"},     // U+2029 PARAGRAPH SEPARATOR
		{"red\u3000", "non-ASCII whitespace"},     // ideographic space
		{"#ff", "does not look like a colour"},
		{"#fffff", "does not look like a colour"},
		{"#gggggg", "does not look like a colour"},
		{"123", "does not look like a colour"},
		{"true", "does not look like a colour"},
		{"nonsense", "does not look like a colour"},
		{"rgb(1,2,3", "does not look like a colour"},
		{"rgb(1,2,3))", "does not look like a colour"},
		{"#fff; color: red", "disallowed character"},
		{"<script>", "disallowed character"},
	}
	for _, tc := range reject {
		t.Run("reject/"+tc.val, func(t *testing.T) {
			err := checkThemeChars("t", tc.val)
			if err == nil {
				err = checkThemeColor("t", tc.val)
			}
			if err == nil {
				t.Fatalf("checkThemeColor(%q) = nil, want an error", tc.val)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tc.want)
			}
		})
	}
}

func TestTheme_WhitespaceIsRefusedByTheLoader(t *testing.T) {
	// The helper-level cases above prove checkThemeChars rejects these.
	// This proves the rejection is reachable from a real config, which is
	// what matters: before the fix `accent: "red "` loaded and was emitted
	// verbatim, so every reader got the engine default and no diagnostic
	// existed anywhere in the pipeline.
	cases := []struct{ name, body, want string }{
		{"trailing space in a colour", "    accent: 'red '\n", "leading or trailing whitespace"},
		{"leading space in a colour", "    accent: ' red'\n", "leading or trailing whitespace"},
		{"NBSP inside a colour", "    accent: \"red\u00a0\"\n", "non-ASCII whitespace"},
		{"trailing space in a length", "    radius: '8px '\n", "leading or trailing whitespace"},
		{"trailing space in a font stack", "    font-sans: 'Arial, sans-serif '\n", "leading or trailing whitespace"},
		{"NBSP in a font stack", "    font-sans: \"Arial,\u00a0sans-serif\"\n", "non-ASCII whitespace"},
		{"trailing space in a per-mode colour", "    dark:\n      ink: '#eeeeee '\n", "leading or trailing whitespace"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			_, err := loadTheme(t, dir, tc.body)
			if err == nil {
				t.Fatal("the value was accepted and would have been emitted as written")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tc.want)
			}
		})
	}
}

// TestTheme_StoredValueIsTheValidatedValue is the invariant behind F1: no
// path through the loader may accept a value and then store a different one.
func TestTheme_StoredValueIsTheValidatedValue(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadTheme(t, dir, "    accent: 'red'\n    radius: '8px'\n    font-sans: 'Arial, sans-serif'\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for k, v := range cfg.Viewer.Theme.Shared {
		if v != strings.TrimSpace(v) {
			t.Errorf("stored %s = %q, which differs from the value a validator would have judged", k, v)
		}
	}
}

func TestThemeFontFamilyGrammar(t *testing.T) {
	accept := []string{
		// The two stacks the first real client (Curtainly) ships. If
		// these ever stop parsing, a project that loads today stops
		// loading, so they are pinned by value and not by shape.
		"Avenir Next, ui-sans-serif, sans-serif",
		"SFMono-Regular, Menlo, monospace",
		"-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
		`"Anthropic Sans", -apple-system, sans-serif`,
		"Helvetica",
	}
	for _, v := range accept {
		t.Run("accept/"+v, func(t *testing.T) {
			if err := checkThemeChars("t", v); err != nil {
				t.Fatalf("universal checks rejected %q: %v", v, err)
			}
			if err := checkThemeFontFamily("t", v); err != nil {
				t.Errorf("checkThemeFontFamily(%q) = %v, want nil", v, err)
			}
		})
	}
	reject := []struct{ val, want string }{
		{"Arial, , sans-serif", "empty family name"},
		{`"Arial, sans-serif`, "unbalanced double quote"},
		{`Arial", sans-serif"`, "not a plain name"},
		{"Arial{}", "disallowed character"},
		{"Arial;", "disallowed character"},
		{"Segoe/*x*/UI", "CSS comment delimiter"},
	}
	for _, tc := range reject {
		t.Run("reject/"+tc.val, func(t *testing.T) {
			err := checkThemeChars("t", tc.val)
			if err == nil {
				err = checkThemeFontFamily("t", tc.val)
			}
			if err == nil {
				t.Fatalf("%q was accepted, want an error", tc.val)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v\nwant it to contain %q", err, tc.want)
			}
		})
	}
}

func TestThemeLengthGrammar(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"0", "8px", "0.5rem", "1.25em", "2ch", "50%", "10vw", "10vh", "-4px"} {
		if _, err := loadTheme(t, dir, "    radius: '"+v+"'\n"); err != nil {
			t.Errorf("radius %q rejected: %v", v, err)
		}
	}
	for _, v := range []string{"10", "px", "10 px", "10pt", "big"} {
		if _, err := loadTheme(t, dir, "    radius: '"+v+"'\n"); err == nil {
			t.Errorf("radius %q accepted, want rejected", v)
		}
	}
}

// -------------------------------------------------------------------------
// Presets.
// -------------------------------------------------------------------------

func TestPreset_UnknownNameListsTheKnownOnes(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadTheme(t, dir, "    preset: clauda\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = ValidateTheme(cfg, os.ReadFile)
	if err == nil {
		t.Fatal("expected an error for an unknown preset")
	}
	if !strings.Contains(err.Error(), `unknown preset "clauda"`) {
		t.Errorf("error = %v, want it to name the bad preset", err)
	}
	for _, n := range PresetNames() {
		if !strings.Contains(err.Error(), n) {
			t.Errorf("error = %v, want it to list the known preset %q", err, n)
		}
	}
	if len(PresetNames()) == 0 {
		t.Fatal("PresetNames is empty, so the assertion above proved nothing")
	}
}

func TestPreset_ClaudeIsValidUnderTheEngineGrammar(t *testing.T) {
	p, ok := Preset("claude")
	if !ok {
		t.Fatal(`Preset("claude") not found`)
	}
	if err := validateThemeBlock(&p, "preset claude"); err != nil {
		t.Fatalf("the built-in preset does not satisfy the engine's own grammar: %v", err)
	}
	// Every key must be an allowlisted token; a preset naming a token the
	// engine dropped would be dead weight nothing reports.
	for _, m := range []map[string]string{p.Shared, p.Light, p.Dark} {
		for k := range m {
			if !themeTokenAllowed[k] {
				t.Errorf("preset claude declares %q, which is not in ThemeTokenAllowlist", k)
			}
		}
	}
	// Every colour token is set in both modes or in neither: a preset that
	// styled only one scheme would leave the other half-themed.
	for k := range p.Light {
		if _, ok := p.Dark[k]; !ok {
			t.Errorf("preset claude sets light.%s but not dark.%s", k, k)
		}
	}
	for k := range p.Dark {
		if _, ok := p.Light[k]; !ok {
			t.Errorf("preset claude sets dark.%s but not light.%s", k, k)
		}
	}
	// 24 per-mode colours + mockup-bg + font-sans + font-mono + radius:
	// the whole 28-token vocabulary, so a preset never leaves a token to
	// the engine default by accident.
	if got := len(p.Light) + len(p.Shared); got != len(ThemeTokenAllowlist) {
		t.Errorf("preset claude covers %d tokens, want %d (the whole allowlist)", got, len(ThemeTokenAllowlist))
	}
	if len(p.Fonts) != 0 {
		t.Errorf("a preset must declare no fonts (it cannot ship files), got %d", len(p.Fonts))
	}
}

func TestPreset_ReturnsACopy(t *testing.T) {
	a, _ := Preset("claude")
	a.Light["paper"] = "#000000"
	b, _ := Preset("claude")
	if b.Light["paper"] == "#000000" {
		t.Fatal("Preset handed out the registry's own map; mutating one caller's theme changed the preset")
	}
}

func TestPresets_ContainNoURLs(t *testing.T) {
	// The viewer is an offline artifact and the portability scan walks
	// internal/. A citation here is filename:line, never a link.
	raw, err := os.ReadFile("presets.go")
	if err != nil {
		t.Fatalf("read presets.go: %v", err)
	}
	for _, needle := range []string{"http://", "https://", "//fonts.", "www."} {
		if strings.Contains(string(raw), needle) {
			t.Errorf("presets.go contains %q; preset sources are cited as filename:line, not as URLs", needle)
		}
	}
}

// -------------------------------------------------------------------------
// extends.
// -------------------------------------------------------------------------

func writeThemeFile(t *testing.T, dir, rel, body string) string {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtends_ResolvedButNotReadByDecodeConfig(t *testing.T) {
	dir := t.TempDir()
	// No file on disk: decoding must still succeed, because DecodeConfig
	// resolves the path and reads nothing.
	cfg, err := loadTheme(t, dir, "    extends: themes/house.yaml\n")
	if err != nil {
		t.Fatalf("DecodeConfig must not read the theme file: %v", err)
	}
	want := filepath.Join(dir, "themes", "house.yaml")
	if cfg.Viewer.Theme.Extends != want {
		t.Errorf("Extends = %q, want the absolute %q", cfg.Viewer.Theme.Extends, want)
	}
	// ...and the read is what fails, naming the path.
	if _, err := ValidateTheme(cfg, os.ReadFile); err == nil {
		t.Fatal("ValidateTheme accepted a missing theme file")
	} else if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v, want it to name %q", err, want)
	}
}

func TestExtends_EscapingTheProjectDirectoryIsRefused(t *testing.T) {
	dir := t.TempDir()
	_, err := loadTheme(t, dir, "    extends: ../x.yaml\n")
	if err == nil {
		t.Fatal("expected an error for an extends that climbs out of the project")
	}
	if !strings.Contains(err.Error(), `viewer.theme.extends: "../x.yaml" resolves outside the project directory`) {
		t.Errorf("error = %v, want the single escaping message", err)
	}
}

func TestExtends_ChainingIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeThemeFile(t, dir, "themes/house.yaml", "extends: other.yaml\naccent: '#C6613F'\n")
	cfg, err := loadTheme(t, dir, "    extends: themes/house.yaml\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = ValidateTheme(cfg, os.ReadFile)
	if err == nil {
		t.Fatal("expected an error for a chained theme file")
	}
	if !strings.Contains(err.Error(), `"extends" is not allowed inside a theme file (no chaining)`) {
		t.Errorf("error = %v, want the no-chaining message", err)
	}
}

func TestExtends_PointingAtTheProjectConfigFails(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "project.config.yaml", themeConfig("    extends: project.config.yaml\n"))
	cfg, err := LoadConfig(filepath.Join(dir, "project.config.yaml"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = ValidateTheme(cfg, os.ReadFile)
	if err == nil {
		t.Fatal("expected an error for extends pointing at the project config")
	}
	// The first thing that fails is a SHAPE rule, not the allowlist: the
	// theme walker reads `facets: [contract]` as a token whose value is a
	// sequence and stops there, because shape is checked during the walk
	// and allowlist membership afterwards. Either way the file is refused
	// and named; the assertion pins what actually happens rather than the
	// message the plan predicted.
	if !strings.Contains(err.Error(), "theme file ") || !strings.Contains(err.Error(), "project.config.yaml") {
		t.Errorf("error = %v, want it to name the file it refused as a theme file", err)
	}
	if !strings.Contains(err.Error(), "facets") {
		t.Errorf("error = %v, want it to name the first key that is not a theme token", err)
	}
}

func TestExtends_ConfigKeysAreNotThemeTokens(t *testing.T) {
	// The allowlist arm of the same refusal: a theme file whose keys are
	// all scalars still cannot be a config, because schema_version is not
	// a theme token.
	dir := t.TempDir()
	writeThemeFile(t, dir, "house.yaml", "schema_version: 1\n")
	cfg, err := loadTheme(t, dir, "    extends: house.yaml\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = ValidateTheme(cfg, os.ReadFile)
	if err == nil {
		t.Fatal("expected an error for config keys in a theme file")
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("error = %v, want it to name schema_version", err)
	}
}

func TestExtends_PresetInsideAThemeFileIsRefused(t *testing.T) {
	// Accepting and ignoring `preset:` here is the worst outcome: the
	// project believes a palette is applied, every token it did not
	// override stays at the engine default, and nothing anywhere says so.
	dir := t.TempDir()
	writeThemeFile(t, dir, "house.yaml", "preset: claude\naccent: '#C6613F'\n")
	cfg, err := loadTheme(t, dir, "    extends: house.yaml\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = ValidateTheme(cfg, os.ReadFile)
	if err == nil {
		t.Fatal("a theme file naming a preset was accepted")
	}
	want := `"preset" is not allowed inside a theme file (name the preset in viewer.theme)`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v\nwant it to contain %q", err, want)
	}
}

func TestExtends_EmptyThemeFileIsRefused(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"zero bytes", ""},
		{"comments only", "# a house theme\n# (nothing here yet)\n"},
		{"whitespace only", "\n\n  \n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeThemeFile(t, dir, "house.yaml", tc.body)
			cfg, err := loadTheme(t, dir, "    extends: house.yaml\n")
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			_, err = ValidateTheme(cfg, os.ReadFile)
			if err == nil {
				t.Fatal("an empty theme file was accepted; extending it does nothing and says nothing")
			}
			if !strings.Contains(err.Error(), "file is empty") {
				t.Errorf("error = %v, want it to say the file is empty", err)
			}
		})
	}
}

func TestThemePaths_FontsAreContainedLikeExtends(t *testing.T) {
	// A font is read and then base64'd into a file someone publishes, so
	// it gets the same containment rule the theme file gets. Without it
	// `src: ../../secret.ttf` ships in the viewer.
	t.Run("relative climb out of the project", func(t *testing.T) {
		dir := t.TempDir()
		_, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
			"      - {family: Probe, src: ../x.ttf}\n")
		if err == nil {
			t.Fatal("a font outside the project directory was accepted")
		}
		if !strings.Contains(err.Error(), `viewer.theme.fonts[0].src: "../x.ttf" resolves outside the project directory`) {
			t.Errorf("error = %v, want the containment message naming the field", err)
		}
	})

	t.Run("theme file font climbing out of the project", func(t *testing.T) {
		dir := t.TempDir()
		// Lexically inside the theme file's own directory would be
		// "themes/../../x.ttf" — outside the project. Containment is
		// measured against the project, not against themes/.
		writeThemeFile(t, dir, "themes/house.yaml",
			"font-sans: 'Probe, sans-serif'\nfonts:\n  - {family: Probe, src: ../../x.ttf}\n")
		cfg, err := loadTheme(t, dir, "    extends: themes/house.yaml\n")
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		_, err = ValidateTheme(cfg, os.ReadFile)
		if err == nil {
			t.Fatal("a theme-file font outside the project directory was accepted")
		}
		if !strings.Contains(err.Error(), "resolves outside the project directory") {
			t.Errorf("error = %v, want the containment message", err)
		}
	})

	t.Run("symlink escape is caught where the lexical check cannot see it", func(t *testing.T) {
		outside := t.TempDir()
		secret := filepath.Join(outside, "secret.ttf")
		writeFontFixture(t, secret, ttfSignature)

		dir := t.TempDir()
		// Lexically "fonts/probe.ttf" is inside the project. Only
		// resolving the link shows where it really points.
		if err := os.MkdirAll(filepath.Join(dir, "fonts"), 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "fonts", "probe.ttf")
		if err := os.Symlink(secret, link); err != nil {
			// Not skipped: a containment check that cannot run is a
			// check that did not happen, and reporting it as a pass
			// is exactly the failure mode CLAUDE.md names.
			t.Fatalf("could not create the symlink this check needs: %v", err)
		}
		cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
			"      - {family: Probe, src: fonts/probe.ttf}\n")
		if err != nil {
			t.Fatalf("the lexical check should pass here: %v", err)
		}
		_, err = ResolveTheme(cfg, os.ReadFile)
		if err == nil {
			t.Fatal("a symlinked font pointing outside the project was read and embedded")
		}
		if !strings.Contains(err.Error(), "link to a file outside the project directory") {
			t.Errorf("error = %v, want the symlink containment message", err)
		}
	})

	t.Run("extends symlink escape", func(t *testing.T) {
		outside := t.TempDir()
		target := filepath.Join(outside, "elsewhere.yaml")
		if err := os.WriteFile(target, []byte("accent: '#C6613F'\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "themes"), 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "themes", "house.yaml")
		if err := os.Symlink(target, link); err != nil {
			// Not skipped: a containment check that cannot run is a
			// check that did not happen, and reporting it as a pass
			// is exactly the failure mode CLAUDE.md names.
			t.Fatalf("could not create the symlink this check needs: %v", err)
		}
		cfg, err := loadTheme(t, dir, "    extends: themes/house.yaml\n")
		if err != nil {
			t.Fatalf("the lexical check should pass here: %v", err)
		}
		if _, err := ValidateTheme(cfg, os.ReadFile); err == nil {
			t.Fatal("a symlinked theme file pointing outside the project was read")
		} else if !strings.Contains(err.Error(), "link to a file outside the project directory") {
			t.Errorf("error = %v, want the symlink containment message", err)
		}
	})

	t.Run("a symlink that stays inside the project is fine", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "assets", "probe.ttf")
		writeFontFixture(t, target, ttfSignature)
		if err := os.MkdirAll(filepath.Join(dir, "fonts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "fonts", "probe.ttf")); err != nil {
			// Not skipped: a containment check that cannot run is a
			// check that did not happen, and reporting it as a pass
			// is exactly the failure mode CLAUDE.md names.
			t.Fatalf("could not create the symlink this check needs: %v", err)
		}
		cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
			"      - {family: Probe, src: fonts/probe.ttf}\n")
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, err := ResolveTheme(cfg, os.ReadFile); err != nil {
			t.Errorf("a symlink resolving inside the project was refused: %v", err)
		}
	})

	t.Run("a missing file falls through to the existence error", func(t *testing.T) {
		// EvalSymlinks cannot resolve a path that is not there. That must
		// not read as an escape, or "check --staged" (where the working
		// tree may legitimately lack the file) would report containment
		// instead of the real problem.
		dir := t.TempDir()
		cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
			"      - {family: Probe, src: fonts/gone.ttf}\n")
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		_, err = ResolveTheme(cfg, os.ReadFile)
		if err == nil {
			t.Fatal("a missing font was accepted")
		}
		if strings.Contains(err.Error(), "outside the project directory") {
			t.Errorf("error = %v, want the existence error, not a containment error", err)
		}
		if !strings.Contains(err.Error(), "gone.ttf") {
			t.Errorf("error = %v, want it to name the missing file", err)
		}
	})
}

func TestExtends_ThemeFileFontsAreRelativeToTheThemeFile(t *testing.T) {
	dir := t.TempDir()
	writeThemeFile(t, dir, "themes/house.yaml",
		"font-sans: 'Probe, sans-serif'\nfonts:\n  - family: Probe\n    src: fonts/probe.ttf\n")
	writeFontFixture(t, filepath.Join(dir, "themes", "fonts", "probe.ttf"), ttfSignature)
	cfg, err := loadTheme(t, dir, "    extends: themes/house.yaml\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rt, err := ResolveTheme(cfg, os.ReadFile)
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	want := filepath.Join(dir, "themes", "fonts", "probe.ttf")
	if len(rt.Fonts) != 1 || rt.Fonts[0].Path != want {
		t.Fatalf("font path = %+v, want one font at %q", rt.Fonts, want)
	}
}

// -------------------------------------------------------------------------
// fonts.
// -------------------------------------------------------------------------

const (
	woff2Signature  = "wOF2"
	woffSignature   = "wOFF"
	otfSignature    = "OTTO"
	ttfSignature    = "\x00\x01\x00\x00"
	ttfSignatureAlt = "true"
)

// writeFontFixture writes a four-byte header plus filler. Four bytes is
// exactly what the signature check reads, so the fixture proves the check
// and nothing about a real font's internals — which is the point: a real
// font file in testdata would make the test pass for reasons it does not
// state.
func writeFontFixture(t *testing.T, path, signature string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := append([]byte(signature), make([]byte, 64)...)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFonts_SignatureSniffPerExtension(t *testing.T) {
	cases := []struct {
		ext string
		sig string
	}{
		{".woff2", woff2Signature},
		{".woff", woffSignature},
		{".otf", otfSignature},
		{".ttf", ttfSignature},
		{".ttf", ttfSignatureAlt},
	}
	for _, tc := range cases {
		t.Run(tc.ext+"/"+fmt.Sprintf("%q", tc.sig), func(t *testing.T) {
			dir := t.TempDir()
			name := "probe" + tc.ext
			writeFontFixture(t, filepath.Join(dir, "fonts", name), tc.sig)
			cfg, err := loadTheme(t, dir,
				"    font-sans: 'Probe, sans-serif'\n    fonts:\n      - family: Probe\n        src: fonts/"+name+"\n")
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if _, err := ResolveTheme(cfg, os.ReadFile); err != nil {
				t.Fatalf("a correct %s signature was rejected: %v", tc.ext, err)
			}
		})
	}

	// Negative control: each extension must reject a signature that
	// belongs to one of the others, or the check above would pass for a
	// file of any type.
	wrong := []struct{ ext, sig string }{
		{".woff2", woffSignature},
		{".woff", woff2Signature},
		{".otf", ttfSignature},
		{".ttf", otfSignature},
	}
	for _, tc := range wrong {
		t.Run("mismatch"+tc.ext, func(t *testing.T) {
			dir := t.TempDir()
			name := "probe" + tc.ext
			writeFontFixture(t, filepath.Join(dir, "fonts", name), tc.sig)
			cfg, err := loadTheme(t, dir,
				"    font-sans: 'Probe, sans-serif'\n    fonts:\n      - family: Probe\n        src: fonts/"+name+"\n")
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			_, err = ResolveTheme(cfg, os.ReadFile)
			if err == nil {
				t.Fatalf("a %s file with a %q header was accepted", tc.ext, tc.sig)
			}
			if !strings.Contains(err.Error(), "signature") {
				t.Errorf("error = %v, want it to say the signature did not match", err)
			}
		})
	}
}

func TestFonts_FieldRules(t *testing.T) {
	cases := []struct {
		name    string
		entry   string
		wantErr string
	}{
		{"family required", "      - src: fonts/p.woff2\n", "family is required"},
		{"src required", "      - family: Probe\n", "src is required"},
		{"family with a comma", "      - {family: 'A, B', src: fonts/p.woff2}\n", "not a plain name"},
		{"bad extension", "      - {family: Probe, src: fonts/p.svg}\n", "must end in one of"},
		{"bad weight", "      - {family: Probe, src: fonts/p.woff2, weight: bold}\n", "is not a CSS font-weight"},
		{"bad style", "      - {family: Probe, src: fonts/p.woff2, style: oblique}\n", `must be "normal" or "italic"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			_, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+tc.entry)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v\nwant it to contain %q", err, tc.wantErr)
			}
		})
	}
	// The two legal weight shapes and both styles load.
	for _, ok := range []string{
		"      - {family: Probe, src: fonts/p.woff2, weight: '400'}\n",
		"      - {family: Probe, src: fonts/p.woff2, weight: '300 800'}\n",
		"      - {family: Probe, src: fonts/p.woff2, style: normal}\n",
		"      - {family: Probe, src: fonts/p.woff2, style: italic}\n",
	} {
		dir := t.TempDir()
		if _, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+ok); err != nil {
			t.Errorf("entry %q rejected: %v", ok, err)
		}
	}
}

func TestFonts_DedupAfterDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFontFixture(t, filepath.Join(dir, "a.woff2"), woff2Signature)
	writeFontFixture(t, filepath.Join(dir, "b.woff2"), woff2Signature)
	// The two entries name the SAME face: one leaves weight and style
	// implicit, the other writes their defaults out. Before the defaults
	// are applied they look different, so a dedup that ran first would
	// emit both.
	cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
		"      - {family: Probe, src: a.woff2}\n"+
		"      - {family: Probe, src: b.woff2, weight: '400', style: normal}\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rt, err := ResolveTheme(cfg, os.ReadFile)
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	if len(rt.Fonts) != 1 {
		t.Fatalf("got %d faces, want 1 after dedup on (family, weight, style)", len(rt.Fonts))
	}
	if got := filepath.Base(rt.Fonts[0].Path); got != "b.woff2" {
		t.Errorf("kept %q, want b.woff2 — the later entry wins the value", got)
	}
	if rt.Fonts[0].Weight != "400" || rt.Fonts[0].Style != "normal" {
		t.Errorf("defaults not applied: weight=%q style=%q", rt.Fonts[0].Weight, rt.Fonts[0].Style)
	}
}

func TestFonts_TotalSizeCap(t *testing.T) {
	dir := t.TempDir()
	big := append([]byte(woff2Signature), make([]byte, MaxThemeFontBytes)...)
	if err := os.WriteFile(filepath.Join(dir, "big.woff2"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
		"      - {family: Probe, src: big.woff2}\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = ResolveTheme(cfg, os.ReadFile)
	if err == nil {
		t.Fatal("a font over the cap was accepted")
	}
	if !strings.Contains(err.Error(), "exceeds") || !strings.Contains(err.Error(), "big.woff2") {
		t.Errorf("error = %v, want it to say the cap was exceeded and name each file with its size", err)
	}
}

func TestFonts_CapStopsReadingAsSoonAsItIsExceeded(t *testing.T) {
	// The cap bounds what a reader downloads. Reading every remaining font
	// in full before comparing would make the check cost the most in
	// exactly the case where it is going to fail, so the comparison is
	// inside the loop. This proves it by counting reads: the second font
	// must never be opened.
	dir := t.TempDir()
	big := append([]byte(woff2Signature), make([]byte, MaxThemeFontBytes)...)
	for _, name := range []string{"a.woff2", "b.woff2"} {
		if err := os.WriteFile(filepath.Join(dir, name), big, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
		"      - {family: Probe, src: a.woff2}\n"+
		"      - {family: Probe, src: b.woff2, weight: '700'}\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var reads []string
	counting := func(path string) ([]byte, error) {
		reads = append(reads, filepath.Base(path))
		return os.ReadFile(path)
	}
	if _, err := ResolveTheme(cfg, counting); err == nil {
		t.Fatal("two oversized fonts were accepted")
	}
	if len(reads) != 1 || reads[0] != "a.woff2" {
		t.Errorf("reads = %v, want exactly [a.woff2] — the cap must fail before reading the rest", reads)
	}
}

func TestFonts_UnreadableFontIsSurfacedWithItsPath(t *testing.T) {
	dir := t.TempDir()
	writeFontFixture(t, filepath.Join(dir, "p.woff2"), woff2Signature)
	cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
		"      - {family: Probe, src: p.woff2}\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	boom := errors.New("permission denied by the injected reader")
	failing := func(path string) ([]byte, error) { return nil, boom }
	_, err = ResolveTheme(cfg, failing)
	if err == nil {
		t.Fatal("a reader that fails was treated as a pass")
	}
	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want the reader's own error wrapped", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "p.woff2")) {
		t.Errorf("error = %v, want it to name the font path", err)
	}
}

func TestFonts_FamilyConsistency(t *testing.T) {
	setup := func(t *testing.T, extra string) *Config {
		t.Helper()
		dir := t.TempDir()
		writeFontFixture(t, filepath.Join(dir, "p.woff2"), woff2Signature)
		if extra != "" {
			if err := os.MkdirAll(filepath.Join(dir, extra), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		body := "schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\nviewer:\n"
		if extra != "" {
			body += "  template_overrides: " + extra + "\n"
		}
		body += "  theme:\n    fonts:\n      - {family: Probe, src: p.woff2}\n"
		cfg, err := DecodeConfig([]byte(body), dir, "project.config.yaml")
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		return cfg
	}

	t.Run("unused family is refused", func(t *testing.T) {
		cfg := setup(t, "")
		_, err := ResolveTheme(cfg, os.ReadFile)
		if err == nil {
			t.Fatal("a font no stack names was accepted")
		}
		if !strings.Contains(err.Error(), `family "Probe" is declared but no font-sans/font-mono token names it`) {
			t.Errorf("error = %v, want the family-consistency message", err)
		}
	})

	t.Run("named family is accepted", func(t *testing.T) {
		dir := t.TempDir()
		writeFontFixture(t, filepath.Join(dir, "p.woff2"), woff2Signature)
		cfg, err := loadTheme(t, dir, "    font-mono: '\"Probe\", monospace'\n    fonts:\n"+
			"      - {family: Probe, src: p.woff2}\n")
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, err := ResolveTheme(cfg, os.ReadFile); err != nil {
			t.Errorf("ResolveTheme: %v", err)
		}
	})

	t.Run("skipped when template_overrides is set", func(t *testing.T) {
		cfg := setup(t, "overrides")
		if _, err := ResolveTheme(cfg, os.ReadFile); err != nil {
			t.Errorf("family consistency must be skipped under template_overrides, got: %v", err)
		}
	})
}

// -------------------------------------------------------------------------
// §1.5 — merge precedence.
// -------------------------------------------------------------------------

func declMap(decls []ThemeDecl) map[string]string {
	m := make(map[string]string, len(decls))
	for _, d := range decls {
		m[d.Token] = d.Value
	}
	return m
}

func TestMerge_PrecedenceTable(t *testing.T) {
	dir := t.TempDir()
	writeThemeFile(t, dir, "house.yaml",
		"accent: '#111111'\nink: '#222222'\nlight:\n  paper: '#333333'\ndark:\n  paper: '#444444'\n")

	cases := []struct {
		name  string
		theme string
		// wantShared/Light/Dark are checked as subsets, so a case names
		// only the tokens it is about.
		wantShared map[string]string
		wantLight  map[string]string
		wantDark   map[string]string
		// absent names tokens that must NOT appear in Light or Dark.
		absentPerMode []string
	}{
		{
			name:          "flat only factors entirely into shared",
			theme:         "    accent: '#C6613F'\n    radius: 8px\n",
			wantShared:    map[string]string{"accent": "#C6613F", "radius": "8px"},
			absentPerMode: []string{"accent", "radius"},
		},
		{
			name:      "per-mode beats shared in the same layer",
			theme:     "    accent: '#000000'\n    light:\n      accent: '#111111'\n",
			wantLight: map[string]string{"accent": "#111111"},
			wantDark:  map[string]string{"accent": "#000000"},
		},
		{
			name:          "per-mode values that agree collapse back into shared",
			theme:         "    light:\n      accent: '#C6613F'\n    dark:\n      accent: '#C6613F'\n",
			wantShared:    map[string]string{"accent": "#C6613F"},
			absentPerMode: []string{"accent"},
		},
		{
			name:      "dark only stays dark only",
			theme:     "    dark:\n      ink: '#eeeeee'\n",
			wantDark:  map[string]string{"ink": "#eeeeee"},
			wantLight: map[string]string{},
		},
		{
			name:       "inline beats the theme file",
			theme:      "    extends: house.yaml\n    accent: '#C6613F'\n",
			wantShared: map[string]string{"accent": "#C6613F", "ink": "#222222"},
			wantLight:  map[string]string{"paper": "#333333"},
			wantDark:   map[string]string{"paper": "#444444"},
		},
		{
			name:       "theme file beats the preset",
			theme:      "    preset: claude\n    extends: house.yaml\n",
			wantShared: map[string]string{"accent": "#111111", "ink": "#222222"},
			wantLight:  map[string]string{"paper": "#333333"},
			wantDark:   map[string]string{"paper": "#444444"},
		},
		{
			name: "a later layer's per-mode value beats an earlier layer's shared value",
			// house.yaml sets accent shared (#111111); the inline dark
			// block sets it only for dark. Light must keep the file's
			// value, dark must take the inline one.
			theme:     "    extends: house.yaml\n    dark:\n      accent: '#999999'\n",
			wantLight: map[string]string{"accent": "#111111"},
			wantDark:  map[string]string{"accent": "#999999"},
		},
		{
			name: "an earlier layer's per-mode value is beaten by a later layer's shared value",
			// house.yaml sets paper per-mode; the inline flat key applies
			// to both modes and, being a later layer, wins both.
			theme:         "    extends: house.yaml\n    paper: '#ffffff'\n",
			wantShared:    map[string]string{"paper": "#ffffff"},
			absentPerMode: []string{"paper"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadTheme(t, dir, tc.theme)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			rt, err := ResolveTheme(cfg, os.ReadFile)
			if err != nil {
				t.Fatalf("ResolveTheme: %v", err)
			}
			shared, light, dark := declMap(rt.Shared), declMap(rt.Light), declMap(rt.Dark)
			for k, v := range tc.wantShared {
				if shared[k] != v {
					t.Errorf("shared[%q] = %q, want %q", k, shared[k], v)
				}
			}
			for k, v := range tc.wantLight {
				if light[k] != v {
					t.Errorf("light[%q] = %q, want %q", k, light[k], v)
				}
			}
			for k, v := range tc.wantDark {
				if dark[k] != v {
					t.Errorf("dark[%q] = %q, want %q", k, dark[k], v)
				}
			}
			for _, k := range tc.absentPerMode {
				if _, ok := light[k]; ok {
					t.Errorf("light[%q] is set, want it factored into shared", k)
				}
				if _, ok := dark[k]; ok {
					t.Errorf("dark[%q] is set, want it factored into shared", k)
				}
			}
		})
	}
}

func TestMerge_EmissionOrderFollowsTheAllowlist(t *testing.T) {
	dir := t.TempDir()
	// Written in reverse allowlist order on purpose.
	cfg, err := loadTheme(t, dir, "    radius: 8px\n    accent: '#C6613F'\n    ink: '#141413'\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rt, err := ResolveTheme(cfg, os.ReadFile)
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	var got []string
	for _, d := range rt.Shared {
		got = append(got, d.Token)
	}
	want := []string{"accent", "ink", "radius"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v (ThemeTokenAllowlist order, not source order)", got, want)
	}
}

func TestValidateTheme_ReportsFontCountAndBytes(t *testing.T) {
	dir := t.TempDir()
	writeFontFixture(t, filepath.Join(dir, "p.woff2"), woff2Signature)
	cfg, err := loadTheme(t, dir, "    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
		"      - {family: Probe, src: p.woff2}\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rep, err := ValidateTheme(cfg, os.ReadFile)
	if err != nil {
		t.Fatalf("ValidateTheme: %v", err)
	}
	if rep.FontCount != 1 {
		t.Errorf("FontCount = %d, want 1", rep.FontCount)
	}
	if rep.FontBytes != int64(len(woff2Signature)+64) {
		t.Errorf("FontBytes = %d, want %d", rep.FontBytes, len(woff2Signature)+64)
	}
	// ValidateTheme must not hold the payload; ResolveTheme must.
	rt, err := ResolveTheme(cfg, os.ReadFile)
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	if len(rt.Fonts[0].Data) == 0 {
		t.Error("ResolveTheme returned a font with no bytes to emit")
	}
}

func TestResolveTheme_ZeroThemeResolvesToNothing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := DecodeConfig([]byte("schema_version: 1\nfacets: [contract]\nmodules: [ledger]\nclaims_dir: claims\n"),
		dir, "project.config.yaml")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rt, err := ResolveTheme(cfg, func(string) ([]byte, error) {
		t.Fatal("resolving an absent theme must read no file")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	if !rt.IsZero() {
		t.Errorf("resolved = %+v, want nothing to emit", rt)
	}
}

// -------------------------------------------------------------------------
// Path resolution: LoadConfig vs DecodeConfig.
// -------------------------------------------------------------------------

func TestThemePaths_LoadConfigAnchorsAtTheConfigFile(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "project")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, sub, "project.config.yaml",
		themeConfig("    extends: themes/house.yaml\n    font-sans: 'Probe, sans-serif'\n    fonts:\n"+
			"      - {family: Probe, src: fonts/p.woff2}\n"))
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if want := filepath.Join(sub, "themes", "house.yaml"); cfg.Viewer.Theme.Extends != want {
		t.Errorf("Extends = %q, want %q (relative to the config file, not the cwd)", cfg.Viewer.Theme.Extends, want)
	}
	if want := filepath.Join(sub, "fonts", "p.woff2"); cfg.Viewer.Theme.Fonts[0].Src != want {
		t.Errorf("fonts[0].Src = %q, want %q", cfg.Viewer.Theme.Fonts[0].Src, want)
	}
}

func TestThemePaths_DecodeConfigAnchorsAtTheSuppliedDir(t *testing.T) {
	// "check --staged" hands DecodeConfig the index's bytes and the real
	// working-tree directory. The theme's paths must follow that anchor.
	anchor := t.TempDir()
	cfg, err := DecodeConfig([]byte(themeConfig("    extends: themes/house.yaml\n")), anchor, "<index>:project.config.yaml")
	if err != nil {
		t.Fatalf("DecodeConfig: %v", err)
	}
	if want := filepath.Join(anchor, "themes", "house.yaml"); cfg.Viewer.Theme.Extends != want {
		t.Errorf("Extends = %q, want %q", cfg.Viewer.Theme.Extends, want)
	}
}

func TestThemePaths_AbsolutePathsAreLeftAlone(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "themes", "house.yaml")
	cfg, err := loadTheme(t, dir, "    extends: "+abs+"\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.Viewer.Theme.Extends != abs {
		t.Errorf("Extends = %q, want the absolute path unchanged", cfg.Viewer.Theme.Extends)
	}
}

// -------------------------------------------------------------------------
// The allowlist itself.
// -------------------------------------------------------------------------

func TestThemeTokenAllowlist_ShapeIsPinned(t *testing.T) {
	if len(ThemeTokenAllowlist) != 28 {
		t.Errorf("ThemeTokenAllowlist has %d tokens, want 28", len(ThemeTokenAllowlist))
	}
	if dup, ok := firstDuplicate(ThemeTokenAllowlist); ok {
		t.Errorf("ThemeTokenAllowlist contains duplicate %q", dup)
	}
	// The first fourteen are the pre-theme vocabulary, in their original
	// order: internal/render emits by walking this list, so reordering
	// them would silently change every existing project's output.
	original := []string{
		"accent", "accent-bg", "ink", "muted", "faint", "paper", "card-bg",
		"border", "link", "warn", "warn-bg", "font-sans", "font-mono", "radius",
	}
	for i, want := range original {
		if ThemeTokenAllowlist[i] != want {
			t.Errorf("ThemeTokenAllowlist[%d] = %q, want %q", i, ThemeTokenAllowlist[i], want)
		}
	}
	added := []string{
		"code-inline-bg", "code-bg", "table-head-bg", "image-bg", "hover-bg",
		"border-strong", "shadow", "shadow-strong", "shadow-cast", "scrim",
		"selection-bg", "status-draft", "status-draft-bg", "mockup-bg",
	}
	for i, want := range added {
		if got := ThemeTokenAllowlist[len(original)+i]; got != want {
			t.Errorf("ThemeTokenAllowlist[%d] = %q, want %q", len(original)+i, got, want)
		}
		if !themeColorTokens[want] {
			t.Errorf("%q must be a colour token", want)
		}
	}
	for _, k := range []string{"font-sans", "font-mono", "radius"} {
		if themeColorTokens[k] {
			t.Errorf("%q must not be a colour token", k)
		}
	}
	if len(themeColorTokens) != 25 {
		t.Errorf("themeColorTokens has %d entries, want 25 (28 tokens less font-sans, font-mono, radius)", len(themeColorTokens))
	}
}

func TestCSSNamedColors_Count(t *testing.T) {
	// 148 CSS named colours plus the two colour keywords.
	if len(cssNamedColors) != 150 {
		t.Errorf("cssNamedColors has %d entries, want 150 (148 named + transparent + currentcolor)", len(cssNamedColors))
	}
	for _, must := range []string{"rebeccapurple", "aliceblue", "yellowgreen", "transparent", "currentcolor"} {
		if !cssNamedColors[must] {
			t.Errorf("cssNamedColors is missing %q", must)
		}
	}
	if cssNamedColors["notacolour"] {
		t.Error("cssNamedColors matched a word that is not a colour")
	}
}

// TestThemeFileErr_DoesNotRepeatThePath pins W2-4: the field prefix stays,
// the absolute path is not printed twice when the wrapped error already
// names the file, and it IS printed when the wrapped error does not.
func TestThemeFileErr_DoesNotRepeatThePath(t *testing.T) {
	dir := t.TempDir()
	cfg, err := loadTheme(t, dir, "    extends: themes/house.yaml\n")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	abs := filepath.Join(dir, "themes", "house.yaml")

	t.Run("staged reader names its own relative path", func(t *testing.T) {
		staged := func(string) ([]byte, error) {
			return nil, errors.New(`"themes/house.yaml" is not staged (git add it)`)
		}
		_, err := ValidateTheme(cfg, staged)
		if err == nil {
			t.Fatal("expected an error")
		}
		msg := err.Error()
		if !strings.HasPrefix(msg, "viewer.theme.extends: ") {
			t.Errorf("message = %q, want the field prefix kept", msg)
		}
		if strings.Contains(msg, abs) {
			t.Errorf("message = %q, want the absolute path dropped (the reader already names the file)", msg)
		}
		if strings.Count(msg, "house.yaml") != 1 {
			t.Errorf("message = %q, want the path named exactly once", msg)
		}
	})

	t.Run("bare reader error still gets the absolute path", func(t *testing.T) {
		_, err := ValidateTheme(cfg, func(string) ([]byte, error) {
			return nil, errors.New("permission denied")
		})
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), abs) {
			t.Errorf("message = %q, want the absolute path added when the error names no file", err)
		}
	})

	t.Run("os.ReadFile PathError is not doubled", func(t *testing.T) {
		_, err := ValidateTheme(cfg, os.ReadFile)
		if err == nil {
			t.Fatal("expected an error")
		}
		if n := strings.Count(err.Error(), abs); n != 1 {
			t.Errorf("message = %q names the absolute path %d times, want 1", err, n)
		}
	})
}
