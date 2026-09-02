package config

// Built-in theme presets.
//
// A preset is the LOWEST layer of the merge in ValidateTheme: `extends` beats
// it and inline keys beat both, so naming one is a starting point, never a
// commitment. Presets carry colour tokens, two font-family STACKS and a
// radius; they never carry `fonts:` entries, because a preset cannot ship
// font files and a preset that named a family nothing could load would put
// every reader on a fallback face and tell nobody.
//
// Source citations below are filename:line into the Typora "Claude" theme
// this palette is taken from, read as a local file. There are deliberately NO
// URLs anywhere in this file: the viewer is an offline artifact and the
// offline scan walks internal/, so a URL here would be a finding whether or
// not anything fetched it.
//
// Preset values may change between minor releases (they track a palette this
// project does not own). Every change gets a CHANGELOG "Changed" line, and a
// project that needs a value frozen writes it inline, where it wins.

// themePresets is the registry PresetNames and Preset read. Values are
// treated as immutable; Preset hands out copies.
var themePresets = map[string]Theme{
	"claude": claudePreset,
}

// claudePreset is the Claude desktop app's palette, mapped onto this
// engine's token vocabulary by ROLE rather than by name: the source theme
// names surfaces (--c-bg-inset), this engine names consumers
// (table-head-bg), and the mapping below is the join between them.
//
// Where the source has one token and this engine has three at different
// strengths (--c-shadow against shadow / shadow-strong / shadow-cast), the
// source supplies the hue and this engine's own default alphas are kept, so
// a preset changes what the shadows are made of without changing how heavy
// the interface reads.
var claudePreset = Theme{
	Shared: map[string]string{
		// Stacks only — the Anthropic faces are named first and fall
		// through when they are not installed. This engine embeds no
		// font it was not handed by the project (viewer.theme.fonts).
		"font-sans": `"Anthropic Sans", -apple-system, BlinkMacSystemFont, "SF Pro Text", "Helvetica Neue", Helvetica, Arial, sans-serif`, // claude.css:112
		"font-mono": `"Anthropic Mono", "SF Mono", "JetBrains Mono", Menlo, Consolas, monospace`,                                          // claude.css:114

		// The source carries three radii (6/10/14px, claude.css:125-127)
		// for controls, cards and sheets; this engine has one token, so
		// the card radius rounded to the nearest even value is used.
		"radius": "8px", // claude.css:126

		// Mockup diagrams are drawn as light-mode artwork in both
		// schemes (they are screenshots of a product, not chrome), so
		// this one is deliberately mode-invariant.
		"mockup-bg": "#ffffff",
	},

	Light: map[string]string{
		"paper":           "#FAF9F5",                  // claude.css:9   --c-bg
		"card-bg":         "#FFFFFF",                  // claude.css:11  --c-bg-elevated
		"table-head-bg":   "#F4F2EC",                  // claude.css:12  --c-bg-inset
		"image-bg":        "#F4F2EC",                  // claude.css:12  --c-bg-inset
		"code-bg":         "#F0EEE6",                  // claude.css:13  --c-bg-code
		"code-inline-bg":  "#EDEBE3",                  // claude.css:14  --c-bg-inline-code
		"hover-bg":        "rgba(20, 20, 19, 0.05)",   // claude.css:15  --c-bg-hover
		"ink":             "#141413",                  // claude.css:19  --c-text
		"muted":           "#5E5D59",                  // claude.css:20  --c-text-2
		"faint":           "#8A8983",                  // claude.css:21  --c-text-3
		"border":          "#E5E3D9",                  // claude.css:26  --c-border
		"border-strong":   "#CFCCC0",                  // claude.css:28  --c-border-strong
		"shadow":          "rgba(20, 20, 19, 0.08)",   // claude.css:29  --c-shadow hue, engine alpha
		"shadow-strong":   "rgba(20, 20, 19, 0.14)",   // claude.css:29  --c-shadow hue, engine alpha
		"shadow-cast":     "rgba(20, 20, 19, 0.12)",   // claude.css:29  --c-shadow
		"scrim":           "rgba(20, 20, 19, 0.22)",   // claude.css:29  --c-shadow hue, engine alpha
		"accent":          "#C6613F",                  // claude.css:32  --c-accent
		"accent-bg":       "rgba(217, 119, 87, 0.14)", // claude.css:34 --c-accent-soft
		"selection-bg":    "rgba(217, 119, 87, 0.28)", // claude.css:36 --c-selection
		"link":            "#2F6FCB",                  // claude.css:40  --c-link
		"warn":            "#B8802E",                  // claude.css:46  --c-warning
		"warn-bg":         "rgba(184, 128, 46, 0.12)", // claude.css:46 --c-warning at the engine's soft alpha
		"status-draft":    "#B8802E",                  // claude.css:46  --c-warning
		"status-draft-bg": "rgba(184, 128, 46, 0.12)", // claude.css:46 --c-warning at the engine's soft alpha
	},

	Dark: map[string]string{
		"paper":           "#151515",                   // claude-dark.css:9   --c-bg
		"card-bg":         "#212121",                   // claude-dark.css:11  --c-bg-elevated
		"table-head-bg":   "#212121",                   // claude-dark.css:12  --c-bg-inset
		"image-bg":        "#212121",                   // claude-dark.css:12  --c-bg-inset
		"code-bg":         "#1C1C1C",                   // claude-dark.css:13  --c-bg-code
		"code-inline-bg":  "#212121",                   // claude-dark.css:14  --c-bg-inline-code
		"hover-bg":        "rgba(255, 255, 255, 0.05)", // claude-dark.css:15  --c-bg-hover
		"ink":             "#F0EFEC",                   // claude-dark.css:19  --c-text
		"muted":           "#B4B3AF",                   // claude-dark.css:20  --c-text-2
		"faint":           "#8A8985",                   // claude-dark.css:21  --c-text-3
		"border":          "#2D2D2D",                   // claude-dark.css:26  --c-border
		"border-strong":   "#373737",                   // claude-dark.css:28  --c-border-strong
		"shadow":          "rgba(0, 0, 0, 0.28)",       // claude-dark.css:29  --c-shadow hue, engine alpha
		"shadow-strong":   "rgba(0, 0, 0, 0.34)",       // claude-dark.css:29  --c-shadow hue, engine alpha
		"shadow-cast":     "rgba(0, 0, 0, 0.30)",       // claude-dark.css:29  --c-shadow hue, engine alpha
		"scrim":           "rgba(0, 0, 0, 0.42)",       // claude-dark.css:29  --c-shadow hue, engine alpha
		"accent":          "#D97757",                   // claude-dark.css:32  --c-accent
		"accent-bg":       "rgba(217, 119, 87, 0.18)",  // claude-dark.css:34  --c-accent-soft
		"selection-bg":    "rgba(217, 119, 87, 0.30)",  // claude-dark.css:36  --c-selection
		"link":            "#6DA7EC",                   // claude-dark.css:40  --c-link
		"warn":            "#E5A66B",                   // claude-dark.css:46  --c-warning
		"warn-bg":         "rgba(229, 166, 107, 0.12)", // claude-dark.css:46  --c-warning at the engine's soft alpha
		"status-draft":    "#E5A66B",                   // claude-dark.css:46  --c-warning
		"status-draft-bg": "rgba(229, 166, 107, 0.12)", // claude-dark.css:46  --c-warning at the engine's soft alpha
	},
}
