package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Theme values are interpolated verbatim into a <style> block by
// internal/render, so every one of them is validated as hostile input, not
// as a typo risk. The rules below come in two phases, and the split is
// deliberate (§1.3):
//
//   - SHAPE, ALLOWLIST and GRAMMAR run in Config.validate(), on bytes alone.
//     Loading a config never touches the filesystem for a theme.
//   - EXISTENCE, TYPE, SIGNATURE, SIZE CAP and FAMILY CONSISTENCY run in
//     ValidateTheme/ResolveTheme, through an injected reader, so that
//     "check --staged" applies the identical rule set to the index's copy of
//     every file instead of a weaker one.

// dangerousThemeChars are rejected outright from any theme value regardless
// of the token's shape. `;{}<>` end a declaration or a <style> element.
const dangerousThemeChars = ";{}<>"

var (
	// themeLengthRe is the grammar for `radius`. A unit is required: CSS
	// treats a unitless non-zero length as invalid and drops the whole
	// declaration, so `radius: 10` would render as the engine default with
	// no diagnostic anywhere.
	themeLengthRe = regexp.MustCompile(`^-?\d+(\.\d+)?(px|rem|em|ch|%|vw|vh)$`)

	// themeFontFamilyItemRe is one unquoted item of a font-family stack.
	themeFontFamilyItemRe = regexp.MustCompile(`^[A-Za-z0-9 _-]+$`)

	// themeFontFamilyRe is the `fonts[].family` name — the CSS identifier
	// the @font-face declares, so no comma and no quotes.
	themeFontFamilyRe = regexp.MustCompile(`^[A-Za-z0-9 _-]+$`)

	// themeFontWeightRe accepts a single weight ("400") or a variable-font
	// range ("300 800").
	themeFontWeightRe = regexp.MustCompile(`^\d{3}( \d{3})?$`)

	// themeFuncCallRe finds every function call in a value: an identifier
	// immediately followed by "(". Every match must name an allowed
	// function, which is what rejects url() and var() inside an otherwise
	// well-formed color-mix().
	themeFuncCallRe = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_-]*)\s*\(`)
)

// themeFontExtensions maps a permitted `fonts[].src` extension to the four
// byte-level signatures a file of that type may legitimately start with. The
// extension alone is not evidence: it is chosen by the same person who chose
// the file, so a mislabelled or truncated file would otherwise be inlined as
// a data: URL the browser silently refuses, leaving the reader with a
// fallback face and nobody told.
var themeFontExtensions = map[string][]string{
	".woff2": {"wOF2"},
	".woff":  {"wOFF"},
	".otf":   {"OTTO"},
	".ttf":   {"\x00\x01\x00\x00", "true"},
}

// checkThemeChars applies the rejections that hold for every token class.
func checkThemeChars(where, val string) error {
	for _, r := range val {
		if r < 0x20 || r == 0x7F {
			return fmt.Errorf("%s: value %q contains a control character", where, val)
		}
	}
	if strings.ContainsAny(val, dangerousThemeChars) {
		return fmt.Errorf("%s: value %q contains a disallowed character (one of %q)", where, val, dangerousThemeChars)
	}
	if strings.Contains(val, "/*") || strings.Contains(val, "*/") {
		return fmt.Errorf("%s: value %q contains a CSS comment delimiter", where, val)
	}
	if strings.Count(val, `"`)%2 != 0 {
		return fmt.Errorf("%s: value %q has an unbalanced double quote", where, val)
	}
	if strings.Count(val, `'`)%2 != 0 {
		return fmt.Errorf("%s: value %q has an unbalanced single quote", where, val)
	}
	return nil
}

// checkThemeColor is the colour grammar: a hex literal, a named colour, or a
// call to one of the functions in colorFunctions.
func checkThemeColor(where, val string) error {
	v := strings.TrimSpace(val)
	if v == "" {
		return fmt.Errorf("%s: value must not be empty", where)
	}

	if strings.HasPrefix(v, "#") {
		hex := v[1:]
		switch len(hex) {
		case 3, 4, 6, 8:
			for _, r := range hex {
				if !isHexDigit(r) {
					return colorErr(where, val)
				}
			}
			return nil
		default:
			return colorErr(where, val)
		}
	}

	if !strings.Contains(v, "(") {
		if cssNamedColors[strings.ToLower(v)] {
			return nil
		}
		return colorErr(where, val)
	}

	// Function form. Every call in the value must name an allowed function
	// (this is what rejects url(...) and var(...)), the parens must
	// balance, and nothing outside the permitted character run may appear.
	for _, m := range themeFuncCallRe.FindAllStringSubmatch(v, -1) {
		if !colorFunctions[strings.ToLower(m[1])] {
			return fmt.Errorf("%s: value %q calls %s(), which is not allowed in a theme colour "+
				"(allowed: %s)", where, val, m[1], strings.Join(sortedKeys(colorFunctions), ", "))
		}
	}
	depth := 0
	for _, r := range v {
		switch {
		case r == '(':
			depth++
		case r == ')':
			depth--
			if depth < 0 {
				return colorErr(where, val)
			}
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == ' ' || r == ',' || r == '.' || r == '%' || r == '/' || r == '+' || r == '-':
		default:
			return colorErr(where, val)
		}
	}
	if depth != 0 {
		return colorErr(where, val)
	}
	if !strings.HasSuffix(v, ")") {
		return colorErr(where, val)
	}
	return nil
}

func colorErr(where, val string) error {
	return fmt.Errorf("%s: value %q does not look like a colour "+
		"(expected #hex, a CSS named colour, or one of %s)",
		where, val, strings.Join(sortedKeys(colorFunctions), "()/ ")+"()")
}

// checkThemeFontFamily validates a font-family STACK (the `font-sans` and
// `font-mono` tokens), which is a comma-separated list of family names.
func checkThemeFontFamily(where, val string) error {
	items := strings.Split(val, ",")
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" {
			return fmt.Errorf("%s: value %q has an empty family name", where, val)
		}
		if q := item[0]; q == '"' || q == '\'' {
			if len(item) < 2 || item[len(item)-1] != q {
				return fmt.Errorf("%s: family %q is not closed by a matching quote", where, item)
			}
			inner := item[1 : len(item)-1]
			if strings.ContainsRune(inner, rune(q)) || strings.Contains(inner, `\`) {
				return fmt.Errorf("%s: family %q contains a quote or backslash", where, item)
			}
			continue
		}
		if !themeFontFamilyItemRe.MatchString(item) {
			return fmt.Errorf("%s: family %q is not a plain name "+
				"(letters, digits, spaces, _ and -) and is not quoted", where, item)
		}
	}
	return nil
}

// validateThemeBlock runs the shape/allowlist/grammar phase over one decoded
// theme, whether it came from viewer.theme or from an extends file. prefix is
// what the reader sees ("viewer.theme", or the theme file's path).
func validateThemeBlock(t *Theme, prefix string) error {
	if err := validateThemeTokens(t.Shared, t.lines, "", prefix); err != nil {
		return err
	}
	if err := validateThemeTokens(t.Light, t.lines, "light", prefix+".light"); err != nil {
		return err
	}
	if err := validateThemeTokens(t.Dark, t.lines, "dark", prefix+".dark"); err != nil {
		return err
	}
	for i, f := range t.Fonts {
		if err := validateThemeFont(f, fmt.Sprintf("%s.fonts[%d]", prefix, i)); err != nil {
			return err
		}
	}
	return nil
}

// validateThemeTokens checks one map of token name to value. Known tokens are
// walked in ThemeTokenAllowlist order so the first complaint about a config
// with several problems is stable rather than map-iteration-random.
func validateThemeTokens(m map[string]string, lines map[string]int, mode, prefix string) error {
	if len(m) == 0 {
		return nil
	}
	for _, key := range ThemeTokenAllowlist {
		val, ok := m[key]
		if !ok {
			continue
		}
		where := prefix + "." + key
		lineKey := key
		if mode != "" {
			lineKey = mode + "." + key
		}
		if line, ok := lines[lineKey]; ok && line > 0 {
			where = fmt.Sprintf("%s.%s (line %d)", prefix, key, line)
		}
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("%s: value must not be empty", where)
		}
		if err := checkThemeChars(where, val); err != nil {
			return err
		}
		switch {
		case themeColorTokens[key]:
			if err := checkThemeColor(where, val); err != nil {
				return err
			}
		case key == "font-sans" || key == "font-mono":
			if err := checkThemeFontFamily(where, val); err != nil {
				return err
			}
		case key == "radius":
			if strings.TrimSpace(val) != "0" && !themeLengthRe.MatchString(strings.TrimSpace(val)) {
				return fmt.Errorf("%s: value %q is not a CSS length "+
					"(a number with a unit, e.g. 10px, 0.5rem, or a bare 0)", where, val)
			}
		}
	}
	// Sorted, not map order: two runs of the same engine over the same
	// config must produce the same message. A config with several unknown
	// tokens would otherwise report a different one each time, which makes
	// the error impossible to pin in a test and confusing to fix by hand.
	unknown := make([]string, 0, len(m))
	for key := range m {
		if !themeTokenAllowed[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		key := unknown[0]
		// `light: { fonts: [...] }` and friends: the structural words are
		// reserved at the top of the theme block only, so under a mode
		// they are neither structure nor a token. Saying so beats
		// printing the 28-name allowlist at someone who wrote a word
		// this format does use, one level up.
		if mode != "" && themeReservedKeys[key] {
			return fmt.Errorf("%s: %q is a reserved word of the theme block and cannot be a token name here "+
				"(it belongs directly under viewer.theme)", prefix, key)
		}
		return fmt.Errorf("%s: unknown theme token %q (must be one of %s)",
			prefix, key, strings.Join(ThemeTokenAllowlist, ", "))
	}
	return nil
}

func validateThemeFont(f ThemeFont, where string) error {
	if strings.TrimSpace(f.Family) == "" {
		return fmt.Errorf("%s: family is required", where)
	}
	if err := checkThemeChars(where+".family", f.Family); err != nil {
		return err
	}
	if !themeFontFamilyRe.MatchString(f.Family) {
		return fmt.Errorf("%s: family %q is not a plain name (letters, digits, spaces, _ and -)", where, f.Family)
	}
	if strings.TrimSpace(f.Src) == "" {
		return fmt.Errorf("%s: src is required", where)
	}
	if err := checkThemeChars(where+".src", f.Src); err != nil {
		return err
	}
	if f.Weight != "" && !themeFontWeightRe.MatchString(f.Weight) {
		return fmt.Errorf("%s: weight %q is not a CSS font-weight "+
			"(three digits, or two separated by a space for a variable range, e.g. \"400\" or \"300 800\")", where, f.Weight)
	}
	if f.Style != "" && f.Style != "normal" && f.Style != "italic" {
		return fmt.Errorf("%s: style %q must be \"normal\" or \"italic\"", where, f.Style)
	}
	ext := strings.ToLower(filepath.Ext(f.Src))
	if _, ok := themeFontExtensions[ext]; !ok {
		return fmt.Errorf("%s: src %q must end in one of %s", where, f.Src, strings.Join(sortedKeys2(themeFontExtensions), ", "))
	}
	return nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedKeys2(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
