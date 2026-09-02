package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Theme is the decoded viewer.theme block: a preset name, an optional theme
// file to extend, token values that apply in both colour schemes (Shared) or
// in only one (Light, Dark), and project-supplied font faces.
//
// It is hand-decoded (see UnmarshalYAML) rather than described with struct
// tags because the useful errors here are the ones gopkg.in/yaml.v3 cannot
// produce: a token written twice in the same mapping is silently last-wins to
// the YAML library, and a bare `accent: #C6613F` is a key with a comment and
// therefore a null value, which is the single most likely mistake a person
// makes writing this block. Both get a line number and a stated fix.
type Theme struct {
	// Preset names a built-in palette to start from (see presets.go). It is
	// the lowest-priority layer of the merge in §1.5: extends beats it, and
	// inline keys beat both.
	Preset string

	// Extends is a theme file to layer on top of the preset. DecodeConfig
	// resolves it to an absolute path against the config file's directory
	// and checks that it stays under that directory; DecodeConfig never
	// reads it. The read happens in ValidateTheme/ResolveTheme, through an
	// injected reader, so "check --staged" can evaluate the index's copy.
	Extends string

	// Shared, Light and Dark hold token values. Shared comes from flat keys
	// directly under `theme:`; Light and Dark from the `light:`/`dark:`
	// sub-mappings. A flat-only theme therefore merges to exactly what this
	// engine emitted before per-mode values existed.
	Shared map[string]string
	Light  map[string]string
	Dark   map[string]string

	// Fonts are the project's own font faces, inlined into the viewer as
	// data: URLs so the single-file guarantee survives.
	Fonts []ThemeFont

	// lines maps a token's qualified name ("paper", "light.paper") to the
	// line it was written on, so a validation error can point at it. It is
	// unexported: it is diagnostic scaffolding, not part of the theme.
	lines map[string]int
}

// ThemeFont is one project-supplied @font-face. Weight and Style carry their
// CSS defaults ("400", "normal") once ResolveTheme has applied them; before
// that they may be empty, which is why dedup happens after defaults.
type ThemeFont struct {
	Family string
	Src    string
	Weight string
	Style  string
	Line   int
}

// IsZero reports a theme that declares nothing at all: no preset, no extends,
// no token in any of the three maps, and no fonts. internal/render uses it to
// emit "" (so the shell's <style></style> element still exists and an
// override sheet that expects it does not break), and tests use it to say
// "viewer.theme was absent" without reaching into the maps.
func (t Theme) IsZero() bool {
	return t.Preset == "" && t.Extends == "" &&
		len(t.Shared) == 0 && len(t.Light) == 0 && len(t.Dark) == 0 &&
		len(t.Fonts) == 0
}

// themeReservedKeys are the keys under `theme:` that are structure rather
// than token values. Everything else under `theme:` is a token name and is
// checked against ThemeTokenAllowlist.
var themeReservedKeys = map[string]bool{
	"preset":  true,
	"extends": true,
	"light":   true,
	"dark":    true,
	"fonts":   true,
}

// themeFontFields is the closed field set of one `fonts:` entry.
var themeFontFields = []string{"family", "src", "weight", "style"}

func nodeKindName(n *yaml.Node) string {
	switch n.Kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

// deref follows an alias node to the anchor it names. A theme may legally be
// written as `theme: *house`, and every mapping this decoder walks resolves
// aliases the same way, so an aliased sub-mapping behaves exactly like one
// written out.
func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && (n.Tag == "!!null" || n.Tag == "" && n.Value == "")
}

// UnmarshalYAML hand-walks the theme mapping. Everything it enforces is a
// SHAPE rule: what may appear, exactly once, with a scalar value. Whether a
// token name is in the allowlist and whether its value is a legal colour are
// decided later by validateThemeBlock, so that a theme FILE (which has no
// Config around it) runs the identical rule set.
func (t *Theme) UnmarshalYAML(node *yaml.Node) error {
	return t.decode(node, "viewer.theme")
}

func (t *Theme) decode(node *yaml.Node, prefix string) error {
	n := deref(node)
	if n == nil {
		return nil
	}
	// `theme:` with nothing after it is not an error and not an empty
	// theme-with-defaults: it is no theme at all, same as omitting the key.
	if isNull(n) {
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: expected a mapping, got %s", prefix, nodeKindName(n))
	}

	t.lines = make(map[string]int)
	seen := make(map[string]int)

	for i := 0; i+1 < len(n.Content); i += 2 {
		keyNode := n.Content[i]
		valNode := n.Content[i+1]
		if keyNode.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s: key on line %d is not a name", prefix, keyNode.Line)
		}
		key := keyNode.Value
		if key == "<<" {
			return fmt.Errorf("%s: YAML merge keys (<<) are not supported here", prefix)
		}
		if first, dup := seen[key]; dup {
			return fmt.Errorf("%s: key %q is defined twice (lines %d and %d)", prefix, key, first, keyNode.Line)
		}
		seen[key] = keyNode.Line

		switch key {
		case "preset":
			v, err := themeScalar(valNode, prefix+".preset", keyNode.Line)
			if err != nil {
				return err
			}
			t.Preset = v
		case "extends":
			v, err := themeScalar(valNode, prefix+".extends", keyNode.Line)
			if err != nil {
				return err
			}
			t.Extends = v
		case "light":
			m, lines, err := themeTokenMap(valNode, prefix+".light")
			if err != nil {
				return err
			}
			t.Light = m
			t.mergeLines("light", lines)
		case "dark":
			m, lines, err := themeTokenMap(valNode, prefix+".dark")
			if err != nil {
				return err
			}
			t.Dark = m
			t.mergeLines("dark", lines)
		case "fonts":
			fonts, err := themeFonts(valNode, prefix+".fonts")
			if err != nil {
				return err
			}
			t.Fonts = fonts
		default:
			v, err := themeTokenValue(valNode, prefix+"."+key, keyNode.Line)
			if err != nil {
				return err
			}
			if t.Shared == nil {
				t.Shared = make(map[string]string)
			}
			t.Shared[key] = v
			t.lines[key] = keyNode.Line
		}
	}
	return nil
}

func (t *Theme) mergeLines(mode string, lines map[string]int) {
	for k, v := range lines {
		t.lines[mode+"."+k] = v
	}
}

// themeScalar reads a plain string field (preset, extends).
func themeScalar(v *yaml.Node, where string, line int) (string, error) {
	n := deref(v)
	if n == nil || n.Kind != yaml.ScalarNode {
		kind := "null"
		if n != nil {
			kind = nodeKindName(n)
		}
		return "", fmt.Errorf("%s (line %d): expected a scalar value, got %s", where, line, kind)
	}
	if isNull(n) || strings.TrimSpace(n.Value) == "" {
		return "", fmt.Errorf("%s (line %d): value must not be empty", where, line)
	}
	return n.Value, nil
}

// themeTokenValue reads one token's value. The empty-value message names the
// cause rather than the symptom: `accent: #C6613F` is not a missing value to
// the person who wrote it, it is a colour, and YAML ate it as a comment.
func themeTokenValue(v *yaml.Node, where string, line int) (string, error) {
	n := deref(v)
	if n == nil || n.Kind != yaml.ScalarNode {
		kind := "null"
		if n != nil {
			kind = nodeKindName(n)
		}
		return "", fmt.Errorf("%s (line %d): expected a scalar value, got %s", where, line, kind)
	}
	if isNull(n) || strings.TrimSpace(n.Value) == "" {
		return "", fmt.Errorf("%s (line %d): value must not be empty "+
			"(a bare #hex is a YAML comment; quote it: '#FAF9F5')", where, line)
	}
	return n.Value, nil
}

// themeTokenMap walks a `light:`/`dark:` sub-mapping under the same rules as
// the theme mapping itself.
func themeTokenMap(v *yaml.Node, where string) (tokens map[string]string, lines map[string]int, err error) {
	n := deref(v)
	if n == nil || isNull(n) {
		return map[string]string{}, map[string]int{}, nil
	}
	if n.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%s: expected a mapping of token names to values, got %s", where, nodeKindName(n))
	}
	out := make(map[string]string, len(n.Content)/2)
	lines = make(map[string]int, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		keyNode := n.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			return nil, nil, fmt.Errorf("%s: key on line %d is not a name", where, keyNode.Line)
		}
		key := keyNode.Value
		if key == "<<" {
			return nil, nil, fmt.Errorf("%s: YAML merge keys (<<) are not supported here", where)
		}
		if first, dup := lines[key]; dup {
			return nil, nil, fmt.Errorf("%s: key %q is defined twice (lines %d and %d)", where, key, first, keyNode.Line)
		}
		val, err := themeTokenValue(n.Content[i+1], where+"."+key, keyNode.Line)
		if err != nil {
			return nil, nil, err
		}
		out[key] = val
		lines[key] = keyNode.Line
	}
	return out, lines, nil
}

// themeFonts walks the `fonts:` sequence.
func themeFonts(v *yaml.Node, where string) ([]ThemeFont, error) {
	n := deref(v)
	if n == nil || isNull(n) {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: expected a list of font entries, got %s", where, nodeKindName(n))
	}
	out := make([]ThemeFont, 0, len(n.Content))
	for i, itemNode := range n.Content {
		item := deref(itemNode)
		at := fmt.Sprintf("%s[%d]", where, i)
		if item == nil || isNull(item) {
			return nil, fmt.Errorf("%s: entry is empty", at)
		}
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s: expected a mapping, got %s", at, nodeKindName(item))
		}
		if len(item.Content) == 0 {
			return nil, fmt.Errorf("%s: entry is empty", at)
		}
		f := ThemeFont{Line: item.Line}
		seen := make(map[string]int, 4)
		for j := 0; j+1 < len(item.Content); j += 2 {
			keyNode := item.Content[j]
			if keyNode.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("%s: key on line %d is not a name", at, keyNode.Line)
			}
			key := keyNode.Value
			if key == "<<" {
				return nil, fmt.Errorf("%s: YAML merge keys (<<) are not supported here", at)
			}
			if first, dup := seen[key]; dup {
				return nil, fmt.Errorf("%s: key %q is defined twice (lines %d and %d)", at, key, first, keyNode.Line)
			}
			seen[key] = keyNode.Line
			if !contains(themeFontFields, key) {
				return nil, fmt.Errorf("%s: unknown field %q (allowed: %s)", at, key, strings.Join(themeFontFields, ", "))
			}
			val, err := themeScalar(item.Content[j+1], at+"."+key, keyNode.Line)
			if err != nil {
				return nil, err
			}
			switch key {
			case "family":
				f.Family = val
			case "src":
				f.Src = val
			case "weight":
				f.Weight = val
			case "style":
				f.Style = val
			}
		}
		out = append(out, f)
	}
	return out, nil
}
