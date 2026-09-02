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

// escapesDir reports whether abs lies outside dir. It is purely lexical:
// both paths are already absolute and cleaned by filepath.Join, so this
// catches every `../` climb without touching the filesystem.
func escapesDir(dir, abs string) bool {
	rel, err := filepath.Rel(dir, abs)
	return err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// escapesDirThroughSymlink is escapesDir after both ends have been resolved
// through any symlinks. It is the second half of the containment check and
// runs only where the filesystem is being touched anyway (ValidateTheme and
// below), because a lexically-contained path can still be a link pointing
// anywhere: `themes/house.yaml -> /etc/passwd` passes the lexical test and
// would otherwise be read and inlined into a viewer someone publishes.
//
// A path that cannot be resolved (it does not exist) is NOT an escape here:
// it falls through so the caller's own read produces the existence error,
// which is the message the author needs. "check --staged" depends on this —
// under it the file may legitimately not be in the working tree at all.
func escapesDirThroughSymlink(dir, abs string) bool {
	// No project directory to be contained by. This is a Config built in
	// process rather than decoded from a file (internal/render's tests do
	// exactly that), and there is no anchor to measure against — inventing
	// one from the working directory would refuse paths for a reason that
	// has nothing to do with the project. Every Config that came from
	// DecodeConfig has already had the lexical half of this check applied.
	if dir == "" {
		return false
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}
	realAbs, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return false
	}
	return escapesDir(realDir, realAbs)
}

// resolveThemePaths turns the theme's path-shaped fields into absolute paths
// anchored at dir, and refuses any that climbs out of the project directory.
// It reads nothing: the symlink half of the containment check runs later,
// where the filesystem is already in play.
func resolveThemePaths(t *Theme, dir string) error {
	if t.Extends != "" {
		abs := t.Extends
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, abs)
		}
		if escapesDir(dir, abs) {
			return fmt.Errorf("viewer.theme.extends: %q resolves outside the project directory", t.Extends)
		}
		t.Extends = abs
	}
	// A font is read and then embedded in a published file, so it gets the
	// same containment rule `extends` gets. Without it `src: ../../id_rsa`
	// is base64 in the viewer.
	for i := range t.Fonts {
		src := t.Fonts[i].Src
		if src == "" {
			continue
		}
		abs := src
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(dir, abs)
		}
		if escapesDir(dir, abs) {
			return fmt.Errorf("viewer.theme.fonts[%d].src: %q resolves outside the project directory", i, src)
		}
		t.Fonts[i].Src = abs
	}
	return nil
}

// wrapThemeFileErr prefixes a file error with the config field it came from,
// without repeating a path the error already names. os.ReadFile's *PathError
// carries the absolute path; the staged reader's "not staged" message carries
// the repo-relative one. A reader that returns a bare error carries neither,
// and there the absolute path is added so the reader still learns which file
// failed.
func wrapThemeFileErr(field, path string, err error) error {
	if errNamesPath(err.Error(), path) {
		return fmt.Errorf("%s: %w", field, err)
	}
	return fmt.Errorf("%s: %s: %w", field, path, err)
}

// errNamesPath reports whether msg already identifies path well enough that
// repeating it would be noise: the whole absolute path, or a trailing run of
// AT LEAST TWO of its components — which is the repo-relative form a staged
// reader prints.
//
// A bare base name deliberately does not count. Two fonts named regular.woff2
// in different directories are a normal thing for a project to have, and
// suppressing the path on a base-name match would leave the author with an
// error that names a file they have several of.
func errNamesPath(msg, path string) bool {
	if path == "" {
		return false
	}
	if strings.Contains(msg, path) {
		return true
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	// i stops at 1, and the shortest suffix tried has two components, so
	// the base name alone is never matched on its own.
	for i := len(parts) - 2; i >= 1; i-- {
		suffix := strings.Join(parts[i:], "/")
		if strings.Contains(msg, suffix) {
			return true
		}
		if native := filepath.FromSlash(suffix); native != suffix && strings.Contains(msg, native) {
			return true
		}
	}
	return false
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
		ext, err := loadThemeFile(t.Extends, cfg.dir, read)
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

	fonts, err := resolveThemeFonts(layers, cfg.dir, read, withData)
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
// grammar as viewer.theme itself, then applies the rules that only make sense
// for a file: neither of the two keys that select a layer may appear inside
// one, and its own fonts are relative to it.
//
// projectDir is the config's own directory, and every path this file names is
// checked against it: a theme file lives under the project, so the fonts it
// pulls in do too.
func loadThemeFile(path, projectDir string, read func(string) ([]byte, error)) (*Theme, error) {
	if escapesDirThroughSymlink(projectDir, path) {
		return nil, fmt.Errorf("viewer.theme.extends: %q is a link to a file outside the project directory", path)
	}
	raw, err := read(path)
	if err != nil {
		return nil, wrapThemeFileErr("viewer.theme.extends", path, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("theme file %s: %w", path, err)
	}
	root := &node
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			root = nil
		} else {
			root = root.Content[0]
		}
	}
	// An empty file (or one that is only comments) decodes to the zero
	// node, kind 0. It is refused rather than treated as a layer that
	// contributes nothing: naming a file in `extends` and getting silence
	// is indistinguishable from the theme working, and the likeliest cause
	// is that the content went somewhere else.
	if root == nil || root.Kind == 0 {
		return nil, fmt.Errorf("theme file %s: file is empty (it declares no tokens, so extending it does nothing)", path)
	}
	var t Theme
	if err := t.decode(root, "theme file "+path); err != nil {
		return nil, err
	}
	// Both layer-selecting keys are refused here. `extends` because
	// chaining is out of scope; `preset` because a theme file is a layer,
	// not a place to choose which layer sits under it — accepting and
	// ignoring it would let a project believe it had a preset applied when
	// nothing had been.
	for _, k := range []string{"extends", "preset"} {
		var set bool
		var advice string
		switch k {
		case "extends":
			set, advice = t.Extends != "", "no chaining"
		case "preset":
			set, advice = t.Preset != "", "name the preset in viewer.theme"
		}
		if set {
			return nil, fmt.Errorf("theme file %s: %q is not allowed inside a theme file (%s)", path, k, advice)
		}
	}
	if err := validateThemeBlock(&t, "theme file "+path); err != nil {
		return nil, err
	}
	// A theme file's font paths are relative to the theme file, not to the
	// config: a themes/ directory that carries its own fonts/ has to be
	// movable as a unit. Containment is still measured against the project
	// directory, not against the theme file's.
	for i := range t.Fonts {
		src := t.Fonts[i].Src
		if src == "" {
			continue
		}
		abs := src
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(filepath.Dir(path), abs)
		}
		if escapesDir(projectDir, abs) {
			return nil, fmt.Errorf("theme file %s: fonts[%d].src %q resolves outside the project directory", path, i, src)
		}
		t.Fonts[i].Src = abs
	}
	return &t, nil
}

// resolveThemeFonts concatenates every layer's fonts, applies the CSS
// defaults, de-duplicates, and then reads and checks each file.
func resolveThemeFonts(layers []themeLayer, projectDir string, read func(string) ([]byte, error), withData bool) ([]internalFont, error) {
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
		if escapesDirThroughSymlink(projectDir, f.Src) {
			return nil, fmt.Errorf("%s: src %q is a link to a file outside the project directory", k.at, f.Src)
		}
		data, err := read(f.Src)
		if err != nil {
			return nil, wrapThemeFileErr(k.at+".src", f.Src, err)
		}
		if !hasAnyPrefix(data, sigs) {
			return nil, fmt.Errorf("%s: src %s does not start with the %s signature "+
				"(the file is not the font type its extension claims, or it is truncated)",
				k.at, f.Src, ext)
		}
		total += int64(len(data))
		sizes = append(sizes, fmt.Sprintf("%s (%d bytes)", f.Src, len(data)))
		// Checked here rather than after the loop: the cap exists to bound
		// what a reader downloads, and reading every remaining font in
		// full to find out we were already over it would mean the check
		// costs the most exactly when it is going to fail. The listing
		// names the files read so far, which is the prefix that already
		// exceeds the cap.
		if total > MaxThemeFontBytes {
			return nil, fmt.Errorf("viewer.theme.fonts: %d bytes exceeds the %d byte cap at %s",
				total, int64(MaxThemeFontBytes), strings.Join(sizes, ", "))
		}
		rf := ResolvedFont{Family: f.Family, Weight: f.Weight, Style: f.Style, Ext: ext, Path: f.Src}
		if withData {
			rf.Data = data
		}
		out = append(out, internalFont{ResolvedFont: rf, size: int64(len(data))})
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
