package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MaxThemeFontBytes caps the total RAW size of every font a theme inlines.
// The viewer is one self-contained file; fonts are base64 data: URLs inside
// it, so this is a bound on what a reader downloads before seeing anything.
// Two mebibytes is roughly four generous variable woff2 faces. Exceeding it
// is an error rather than a truncation, because dropping a face silently
// would render the reader's page in a fallback the project did not choose
// and did not get told about.
const MaxThemeFontBytes = 2 << 20

// ThemeDecl is one resolved token declaration. The resolved theme carries
// SLICES of these rather than maps so that emission order is a property of
// the value and not of whatever order a caller's map iteration happened to
// produce — two runs of the same engine over the same config must produce
// byte-identical viewers.
type ThemeDecl struct {
	Token string
	Value string
}

// ResolvedFont is one @font-face ready for emission: metadata, the file it
// came from, its format, and its bytes.
type ResolvedFont struct {
	Family string
	Weight string
	Style  string
	// Ext is the lower-cased file extension including the dot (".woff2").
	Ext string
	// Path is the absolute path the bytes were read from, for messages.
	Path string
	// Data is the raw file content. It is nil for ValidateTheme, which
	// checks everything about a font except what it would take to emit it.
	Data []byte
}

// ResolvedTheme is what internal/render emits: three ordered declaration
// lists and the fonts. Shared holds tokens whose value is the same in both
// colour schemes, Light and Dark only those that differ.
type ResolvedTheme struct {
	Shared []ThemeDecl
	Light  []ThemeDecl
	Dark   []ThemeDecl
	Fonts  []ResolvedFont
}

// IsZero reports a resolved theme with nothing to emit.
func (r *ResolvedTheme) IsZero() bool {
	return r == nil || (len(r.Shared) == 0 && len(r.Light) == 0 && len(r.Dark) == 0 && len(r.Fonts) == 0)
}

// ThemeReport is what a checker wants to know about a theme it has just
// validated but will not render: how much of the reader's download the
// project's fonts account for.
type ThemeReport struct {
	FontCount int
	FontBytes int64
}

// resolveThemePaths turns the theme's two path-shaped fields into absolute
// paths anchored at dir, and refuses an `extends` that climbs out of the
// project directory. It reads nothing.
func resolveThemePaths(t *Theme, dir string) error {
	if t.Extends != "" {
		abs := t.Extends
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, abs)
		}
		rel, err := filepath.Rel(dir, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("viewer.theme.extends: %q resolves outside the project directory", t.Extends)
		}
		t.Extends = abs
	}
	for i := range t.Fonts {
		if src := t.Fonts[i].Src; src != "" && !filepath.IsAbs(src) {
			t.Fonts[i].Src = filepath.Join(dir, src)
		}
	}
	return nil
}

// ValidateTheme applies every theme rule that needs to look at a file: the
// theme file `extends` names, each font's existence, extension, signature,
// the total size cap, and family consistency. It returns what a caller that
// is not rendering wants to report.
//
// read is injected rather than assumed to be os.ReadFile because "dossierx
// check --staged" must evaluate the STAGED bytes of every file, not the
// working tree's. Passing the reader in is what keeps that caller on this
// exact rule set instead of a second, weaker copy of it: signature and cap
// checks run identically under --staged.
func ValidateTheme(cfg *Config, read func(path string) ([]byte, error)) (*ThemeReport, error) {
	rt, err := resolveTheme(cfg, read, false)
	if err != nil {
		return nil, err
	}
	rep := &ThemeReport{FontCount: len(rt.Fonts)}
	for _, f := range rt.Fonts {
		rep.FontBytes += f.size
	}
	return rep, nil
}

// ResolveTheme is ValidateTheme plus the font payloads, for emission.
func ResolveTheme(cfg *Config, read func(path string) ([]byte, error)) (*ResolvedTheme, error) {
	rt, err := resolveTheme(cfg, read, true)
	if err != nil {
		return nil, err
	}
	return rt.public(), nil
}

// internalFont carries the size alongside the (possibly withheld) bytes, so
// ValidateTheme can report FontBytes without holding megabytes of payload.
type internalFont struct {
	ResolvedFont
	size int64
}

// themeLayer is one merge layer plus the name a message should use for it,
// so "fonts[0] is not a font file" points at the preset, the theme file or
// the project's own config rather than always at the last of the three.
type themeLayer struct {
	label string
	theme Theme
}

type internalTheme struct {
	Shared []ThemeDecl
	Light  []ThemeDecl
	Dark   []ThemeDecl
	Fonts  []internalFont
}

func (it *internalTheme) public() *ResolvedTheme {
	out := &ResolvedTheme{Shared: it.Shared, Light: it.Light, Dark: it.Dark}
	for _, f := range it.Fonts {
		out.Fonts = append(out.Fonts, f.ResolvedFont)
	}
	return out
}

func resolveTheme(cfg *Config, read func(path string) ([]byte, error), withData bool) (*internalTheme, error) {
	out := &internalTheme{}
	if cfg == nil || cfg.Viewer.Theme.IsZero() {
		return out, nil
	}
	t := cfg.Viewer.Theme

	// Lowest priority first: preset, then the extends file, then what the
	// project wrote inline.
	layers := make([]themeLayer, 0, 3)
	if t.Preset != "" {
		p, ok := Preset(t.Preset)
		if !ok {
			return nil, fmt.Errorf("viewer.theme.preset: unknown preset %q (known: %s)",
				t.Preset, strings.Join(PresetNames(), ", "))
		}
		layers = append(layers, themeLayer{label: "preset " + t.Preset, theme: p})
	}
	if t.Extends != "" {
		ext, err := loadThemeFile(t.Extends, read)
		if err != nil {
			return nil, err
		}
		layers = append(layers, themeLayer{label: "theme file " + t.Extends, theme: *ext})
	}
	layers = append(layers, themeLayer{label: "viewer.theme", theme: t})

	// §1.5: build each mode's effective map by applying every layer's
	// shared keys and then that layer's per-mode keys, so a per-mode value
	// beats a shared one from the SAME layer and from every earlier one.
	effLight := map[string]string{}
	effDark := map[string]string{}
	for _, layer := range layers {
		l := layer.theme
		for k, v := range l.Shared {
			effLight[k] = v
			effDark[k] = v
		}
		for k, v := range l.Light {
			effLight[k] = v
		}
		for k, v := range l.Dark {
			effDark[k] = v
		}
	}

	// Factor out the agreement: a token with the same value in both modes
	// belongs in the unconditional :root block, which is what makes a
	// flat-only theme emit exactly the block it emitted before per-mode
	// values existed.
	for _, key := range ThemeTokenAllowlist {
		lv, hasL := effLight[key]
		dv, hasD := effDark[key]
		switch {
		case hasL && hasD && lv == dv:
			out.Shared = append(out.Shared, ThemeDecl{Token: key, Value: lv})
		default:
			if hasL {
				out.Light = append(out.Light, ThemeDecl{Token: key, Value: lv})
			}
			if hasD {
				out.Dark = append(out.Dark, ThemeDecl{Token: key, Value: dv})
			}
		}
	}

	fonts, err := resolveThemeFonts(layers, read, withData)
	if err != nil {
		return nil, err
	}
	out.Fonts = fonts

	if err := checkFamilyConsistency(cfg, out); err != nil {
		return nil, err
	}
	return out, nil
}

// loadThemeFile decodes a theme file with the same walker and the same
// grammar as viewer.theme itself, then applies the two rules that only make
// sense for a file: no chaining, and its own fonts are relative to it.
func loadThemeFile(path string, read func(string) ([]byte, error)) (*Theme, error) {
	raw, err := read(path)
	if err != nil {
		return nil, fmt.Errorf("viewer.theme.extends: %s: %w", path, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("theme file %s: %w", path, err)
	}
	root := &node
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return &Theme{}, nil
		}
		root = root.Content[0]
	}
	var t Theme
	if err := t.decode(root, "theme file "+path); err != nil {
		return nil, err
	}
	if t.Extends != "" {
		return nil, fmt.Errorf("theme file %s: %q is not allowed inside a theme file (no chaining)", path, "extends")
	}
	if err := validateThemeBlock(&t, "theme file "+path); err != nil {
		return nil, err
	}
	// A theme file's font paths are relative to the theme file, not to the
	// config: a themes/ directory that carries its own fonts/ has to be
	// movable as a unit.
	for i := range t.Fonts {
		if src := t.Fonts[i].Src; src != "" && !filepath.IsAbs(src) {
			t.Fonts[i].Src = filepath.Join(filepath.Dir(path), src)
		}
	}
	return &t, nil
}

// resolveThemeFonts concatenates every layer's fonts, applies the CSS
// defaults, de-duplicates, and then reads and checks each file.
func resolveThemeFonts(layers []themeLayer, read func(string) ([]byte, error), withData bool) ([]internalFont, error) {
	type keyed struct {
		font ThemeFont
		at   string
	}
	var ordered []keyed
	index := map[string]int{}
	for _, layer := range layers {
		for i, f := range layer.theme.Fonts {
			// Defaults BEFORE dedup: `{family: F}` and
			// `{family: F, weight: "400"}` are the same face, and
			// de-duplicating first would emit both and let the second
			// win by cascade instead of by the rule stated here.
			if f.Weight == "" {
				f.Weight = "400"
			}
			if f.Style == "" {
				f.Style = "normal"
			}
			k := f.Family + "\x00" + f.Weight + "\x00" + f.Style
			entry := keyed{font: f, at: fmt.Sprintf("%s.fonts[%d]", layer.label, i)}
			if at, dup := index[k]; dup {
				// Later layer wins the VALUE; the earlier layer keeps
				// the POSITION, so adding a preset cannot reorder the
				// faces a project already had.
				ordered[at] = entry
				continue
			}
			index[k] = len(ordered)
			ordered = append(ordered, entry)
		}
	}

	out := make([]internalFont, 0, len(ordered))
	var total int64
	var sizes []string
	for _, k := range ordered {
		f := k.font
		ext := strings.ToLower(filepath.Ext(f.Src))
		sigs, ok := themeFontExtensions[ext]
		if !ok {
			return nil, fmt.Errorf("%s: src %q must end in one of %s",
				k.at, f.Src, strings.Join(sortedKeys2(themeFontExtensions), ", "))
		}
		data, err := read(f.Src)
		if err != nil {
			return nil, fmt.Errorf("%s: src %s: %w", k.at, f.Src, err)
		}
		if !hasAnyPrefix(data, sigs) {
			return nil, fmt.Errorf("%s: src %s does not start with the %s signature "+
				"(the file is not the font type its extension claims, or it is truncated)",
				k.at, f.Src, ext)
		}
		total += int64(len(data))
		sizes = append(sizes, fmt.Sprintf("%s (%d bytes)", f.Src, len(data)))
		rf := ResolvedFont{Family: f.Family, Weight: f.Weight, Style: f.Style, Ext: ext, Path: f.Src}
		if withData {
			rf.Data = data
		}
		out = append(out, internalFont{ResolvedFont: rf, size: int64(len(data))})
	}
	if total > MaxThemeFontBytes {
		return nil, fmt.Errorf("viewer.theme.fonts: %d bytes total exceeds the %d byte cap: %s",
			total, int64(MaxThemeFontBytes), strings.Join(sizes, ", "))
	}
	return out, nil
}

func hasAnyPrefix(data []byte, sigs []string) bool {
	for _, s := range sigs {
		if len(data) >= len(s) && string(data[:len(s)]) == s {
			return true
		}
	}
	return false
}

// checkFamilyConsistency refuses a font nothing would use. A face that no
// font-sans/font-mono stack names is downloaded by every reader and rendered
// to no glyph on any page, which is the most expensive kind of typo there is.
//
// It is SKIPPED when viewer.template_overrides is set: an override sheet may
// name the family itself, and this package cannot read that sheet. FORMAT.md
// records the bound rather than leaving it as a silent exemption.
func checkFamilyConsistency(cfg *Config, rt *internalTheme) error {
	if len(rt.Fonts) == 0 || cfg.Viewer.TemplateOverrides != "" {
		return nil
	}
	named := map[string]bool{}
	for _, decls := range [][]ThemeDecl{rt.Shared, rt.Light, rt.Dark} {
		for _, d := range decls {
			if d.Token != "font-sans" && d.Token != "font-mono" {
				continue
			}
			for _, item := range strings.Split(d.Value, ",") {
				item = strings.TrimSpace(item)
				item = strings.Trim(item, `"'`)
				named[strings.ToLower(item)] = true
			}
		}
	}
	for i, f := range rt.Fonts {
		if !named[strings.ToLower(f.Family)] {
			return fmt.Errorf("viewer.theme.fonts[%d]: family %q is declared but no font-sans/font-mono token names it, "+
				"so nothing would use it", i, f.Family)
		}
	}
	return nil
}

// PresetNames returns every built-in preset name, sorted, for the CLI's
// "dossierx theme list" and for the unknown-preset error.
func PresetNames() []string {
	out := make([]string, 0, len(themePresets))
	for k := range themePresets {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Preset returns the built-in preset with the given name. The returned Theme
// is a deep copy: a caller that mutates it cannot corrupt the preset for the
// rest of the process.
func Preset(name string) (Theme, bool) {
	p, ok := themePresets[name]
	if !ok {
		return Theme{}, false
	}
	return Theme{
		Shared: copyMap(p.Shared),
		Light:  copyMap(p.Light),
		Dark:   copyMap(p.Dark),
		Fonts:  append([]ThemeFont(nil), p.Fonts...),
	}, true
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
