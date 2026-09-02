// theme.go is the "dossierx theme" noun: the two read-only leaves that make the
// viewer's built-in palettes discoverable and editable.
//
// NEITHER LEAF TOUCHES A PROJECT. "theme list" reads the compiled-in preset
// registry and "theme export" turns one preset into a theme file; neither loads
// project.config.yaml, neither reads claims_dir, and neither takes a write
// sentinel. That is why they answer in a repo DossierX has not been set up in
// yet — which is exactly where a person is when they are choosing a palette —
// and it is why "theme export" writing a file is not a project mutation: the
// file it writes is inert until a human points `viewer.theme.extends` at it.
//
// # WHY EXPORT EXISTS AT ALL
//
// `preset: claude` alone is a fine starting point and needs no file. The
// export path is for the case the preset cannot serve: a project that wants
// the palette as a STARTING point and then diverges from it. Without a way to
// see the values, "start from claude and change the accent" means reading
// this binary's source, and "why is my card grey?" means guessing which of
// twenty-eight tokens the preset happened to set. Exporting materializes the
// whole layer as ordinary YAML the project then owns.
//
// The exported file deliberately carries no `extends:` of its own — chaining
// is refused by the decoder, so an export that emitted one would produce a
// file the engine rejects — and no version stamp, so re-exporting the same
// preset from the same binary is byte-identical and a diff means the preset
// moved.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// newThemeCmd is the "dossierx theme" command group: list and export.
func newThemeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "theme",
		Short: "Read the viewer's built-in colour presets, and export one as a theme file a project can edit and extend",
	}
	cmd.AddCommand(
		newThemeListCmd(),
		newThemeExportCmd(),
	)
	return commandGroup(cmd)
}

// themePresetDescriptions is the one-line prose for each built-in preset.
//
// It lives here rather than in internal/config because it is a CLI affordance
// and not part of the palette: a preset is a set of token values, and a
// sentence about where those values came from is something a reader of THIS
// surface needs. TestThemeList_DescribesEveryPreset pins that the table covers
// config.PresetNames(), so a preset added without a description is a failing
// test rather than a blank column.
var themePresetDescriptions = map[string]string{
	"claude": "The Claude desktop app's palette — warm off-white paper in light, near-black in dark, " +
		"terracotta accent — mapped onto this engine's tokens by role. Font-family stacks only: it " +
		"names the Anthropic faces first and falls through to system fonts, because a preset cannot " +
		"ship font files.",
}

// ---------------------------------------------------------------------
// theme list
// ---------------------------------------------------------------------

// themePresetEntry is one built-in preset as this surface reports it.
//
// Tokens carries the NAMES the preset sets, not their values. The names are
// what answers "will naming this preset change my accent?" in one line, and
// the values are what "theme export" is for — repeating a hundred-odd colours
// inside a list command would make the common call unreadable in a terminal
// and would put two copies of the palette in the machine surface.
type themePresetEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tokens      []string `json:"tokens"`
}

// themeListData is "dossierx theme list"'s machine payload.
type themeListData struct {
	Count   int                `json:"count"`
	Presets []themePresetEntry `json:"presets"`
}

func newThemeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List every built-in theme preset with the token names it sets",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			names := config.PresetNames()
			entries := make([]themePresetEntry, 0, len(names))
			for _, name := range names {
				preset, ok := config.Preset(name)
				if !ok {
					// PresetNames and Preset read the same registry, so this is
					// unreachable rather than merely unlikely. It is reported
					// instead of skipped because a list that silently drops a
					// preset is indistinguishable from a list of everything
					// there is.
					return cmdResult{}, cliout.Errorf(cliout.CodeInternal,
						"theme list: preset %q is named by PresetNames but not readable", name)
				}
				entries = append(entries, themePresetEntry{
					Name:        name,
					Description: themePresetDescriptions[name],
					Tokens:      presetTokenNames(preset),
				})
			}
			data := themeListData{Count: len(entries), Presets: entries}
			return cmdResult{
				Data: data,
				Text: func() { writeThemeListText(cmd, data) },
			}, nil
		}),
	}
}

// presetTokenNames is every token the preset sets, in ThemeTokenAllowlist
// order.
//
// Allowlist order rather than alphabetical, and for the reason internal/render
// uses it too: it is the engine's own declaration order, so a reader comparing
// this list against an emitted `:root{…}` block or against FORMAT.md's table is
// reading three renderings of one sequence. A token set in more than one of
// shared/light/dark appears once.
func presetTokenNames(t config.Theme) []string {
	set := map[string]bool{}
	for _, m := range []map[string]string{t.Shared, t.Light, t.Dark} {
		for k := range m {
			set[k] = true
		}
	}
	out := make([]string, 0, len(set))
	for _, k := range config.ThemeTokenAllowlist {
		if set[k] {
			out = append(out, k)
			delete(set, k)
		}
	}
	// Anything left is a preset key the allowlist does not carry, which
	// config's own tests refuse. Appended sorted rather than dropped, for
	// presetTokenNames's stated job: report what the preset sets.
	leftover := make([]string, 0, len(set))
	for k := range set {
		leftover = append(leftover, k)
	}
	sort.Strings(leftover)
	return append(out, leftover...)
}

func writeThemeListText(cmd *cobra.Command, d themeListData) {
	out := cmd.OutOrStdout()
	for _, p := range d.Presets {
		fmt.Fprintf(out, "%s (%d token(s))\n", p.Name, len(p.Tokens))
		if p.Description != "" {
			fmt.Fprintf(out, "  %s\n", p.Description)
		}
		fmt.Fprintf(out, "  sets: %s\n", strings.Join(p.Tokens, ", "))
	}
	fmt.Fprintf(out, "theme list: %d preset(s)\n", d.Count)
	fmt.Fprintln(out, "use one with viewer.theme.preset, or run \"dossierx theme export <name> <path>\" to edit its values")
}

// ---------------------------------------------------------------------
// theme export <preset> [path]
// ---------------------------------------------------------------------

// themeExportData is "dossierx theme export"'s machine payload.
//
// The two shapes are one struct with omitempty rather than two payload types,
// because they are one question answered at two destinations: YAML is the
// content when the caller named no path, Path and Bytes are the receipt when
// they did. A caller reads whichever of the two it asked for, and a caller that
// wants both writes the file and reads it back.
//
// The content rides in the ENVELOPE when there is no path — not on a bare
// stdout — because this command is not exempt from the machine contract. A
// leaf that printed raw YAML would be the one invocation in the surface whose
// stdout is not an envelope, which is the hole the root's --version handling
// exists to close.
type themeExportData struct {
	Preset string `json:"preset"`
	YAML   string `json:"yaml,omitempty"`
	Path   string `json:"path,omitempty"`
	Bytes  int    `json:"bytes,omitempty"`
}

func newThemeExportCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "export <preset> [path]",
		Short: "Write a preset out as an editable theme file, or return its YAML when no path is given",
		Args:  cobra.RangeArgs(1, 2),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			name := args[0]
			preset, ok := config.Preset(name)
			if !ok {
				return cmdResult{}, cliout.Errorf(cliout.CodeUnknownPreset,
					"theme export: %q is not a built-in preset", name).
					WithHint(fmt.Sprintf("run \"dossierx theme export <preset> [path]\" with one of: %s",
						strings.Join(config.PresetNames(), ", ")))
			}
			doc := renderThemeFile(name, preset)

			if len(args) == 1 {
				return cmdResult{
					Data: themeExportData{Preset: name, YAML: doc},
					Text: func() { fmt.Fprint(cmd.OutOrStdout(), doc) },
				}, nil
			}

			path := args[1]
			// Refuse an existing file rather than overwriting it. The file this
			// command writes is one a project EDITS — that is the entire point of
			// exporting instead of naming the preset — so the second run of a
			// command someone half-remembers would silently discard the edits,
			// and nothing in the theme file's own content would show that it had
			// happened.
			//
			// write_conflict is reused rather than given a new code because the
			// recovery is the one that code already means on the claims lock: the
			// caller is contending with something that is already there, and the
			// move is to look at it. --force is the deliberate override, and it
			// is a LOCAL flag: nothing else in this surface overwrites a file, so
			// a persistent --force would be a promise the rest of the CLI does
			// not keep.
			if _, err := os.Stat(path); err == nil && !force {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict,
					"theme export: %s already exists, so it was not overwritten", path).
					WithHint("read it first — if it is a theme you have edited, export to a different path; pass --force to replace it")
			}
			if dir := filepath.Dir(path); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed,
						"theme export: create directory for %s: %v", path, err)
				}
			}
			if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed,
					"theme export: write %s: %v", path, err)
			}
			data := themeExportData{Preset: name, Path: path, Bytes: len(doc)}
			return cmdResult{
				Data: data,
				Text: func() {
					out := cmd.OutOrStdout()
					fmt.Fprintf(out, "theme export: wrote %s (%d bytes) from preset %q\n", data.Path, data.Bytes, data.Preset)
					fmt.Fprintln(out, "point a project at it with viewer.theme.extends, then run \"dossierx check\"")
				},
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the file if it already exists (it is refused as write_conflict otherwise)")
	return cmd
}

// renderThemeFile turns a preset into the bytes of a theme file.
//
// It is hand-emitted rather than yaml.Marshal'd for the property the merge
// algorithm depends on being READABLE: token order is
// config.ThemeTokenAllowlist's, which is the order the engine emits
// declarations in and the order FORMAT.md's table lists them in, where
// marshalling a map would sort alphabetically and scatter the three shadows
// across the file. Every value is single-quoted whether or not it needs to be,
// because the single most common mistake writing this block by hand is a bare
// `accent: #C6613F` — which YAML reads as a key with a comment and therefore a
// null value — and a file that models the fix everywhere is worth more than
// three saved characters.
//
// No `extends:` is emitted: chaining is refused inside a theme file, so
// emitting one would produce a file the engine rejects. No version or
// timestamp is emitted either, so two exports of the same preset from the same
// binary are byte-identical and any diff is the preset having moved.
func renderThemeFile(name string, t config.Theme) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# DossierX theme file, exported from the built-in %q preset.\n", name)
	b.WriteString("#\n")
	b.WriteString("# Point a project at it from project.config.yaml:\n")
	b.WriteString("#\n")
	b.WriteString("#   viewer:\n")
	b.WriteString("#     theme:\n")
	b.WriteString("#       extends: themes/mine.yaml\n")
	b.WriteString("#\n")
	b.WriteString("# Keys at the top level apply to BOTH colour schemes; keys under light: and\n")
	b.WriteString("# dark: apply to one. Delete any token to fall back to the engine's own\n")
	b.WriteString("# default for it — an absent token is not an empty one. Inline keys under\n")
	b.WriteString("# viewer.theme still beat everything here.\n")
	b.WriteString("#\n")
	b.WriteString("# \"extends\" is not allowed inside a theme file: theme files do not chain.\n")
	b.WriteString("# Preset values may change between minor releases, so this file is a\n")
	b.WriteString("# snapshot of the preset and not a link to it — which is the point of\n")
	b.WriteString("# exporting rather than naming the preset.\n")

	writeThemeTokenBlock(&b, "", t.Shared, "")
	writeThemeTokenBlock(&b, "light", t.Light, "  ")
	writeThemeTokenBlock(&b, "dark", t.Dark, "  ")
	return b.String()
}

// writeThemeTokenBlock emits one layer's tokens, in allowlist order, under an
// optional mapping key. An empty layer emits nothing at all rather than a bare
// `light:` — a null mapping is legal input but says nothing, and a reader would
// have to know that to be sure the export was not truncated.
func writeThemeTokenBlock(b *strings.Builder, key string, tokens map[string]string, indent string) {
	if len(tokens) == 0 {
		return
	}
	b.WriteString("\n")
	if key != "" {
		fmt.Fprintf(b, "%s:\n", key)
	}
	seen := map[string]bool{}
	for _, token := range config.ThemeTokenAllowlist {
		v, ok := tokens[token]
		if !ok {
			continue
		}
		seen[token] = true
		fmt.Fprintf(b, "%s%s: %s\n", indent, token, quoteThemeValue(v))
	}
	// A key the allowlist does not carry cannot load, so emitting it would
	// produce a broken file. It is reported in a comment rather than dropped
	// silently, because a token that vanished between the preset and the export
	// is the failure this whole file is meant to make visible.
	leftover := make([]string, 0)
	for token := range tokens {
		if !seen[token] {
			leftover = append(leftover, token)
		}
	}
	sort.Strings(leftover)
	for _, token := range leftover {
		fmt.Fprintf(b, "%s# %s: %s   # not in this engine's token vocabulary; left commented out\n",
			indent, token, quoteThemeValue(tokens[token]))
	}
}

// quoteThemeValue renders a token value as a YAML single-quoted scalar.
//
// Single quotes rather than double: a single-quoted YAML scalar has exactly one
// escape (a doubled quote) and no backslash processing, so a font stack full of
// double quotes — which every font-sans value is — passes through untouched and
// there is nothing for a reader to get wrong.
func quoteThemeValue(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
