// theme_cli_test.go covers the "dossierx theme" noun end to end through the
// real command tree.
//
// The tests that matter most here are not about output shape. TestThemeLeaves
// NeedNoProject pins the property that makes the noun usable at all — it
// answers before a project exists — TestThemeExportRefusesToClobber pins the
// refusal that stands between a second run of a half-remembered command and
// somebody's edited palette, and TestThemeExportOutputLoadsAsATheme pins the
// only thing the exported bytes are for: that the engine reads them back.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// The theme payloads are registered here rather than written into
// surface_test.go's literal, for the reason track_cli_test.go states: that file
// is compiled inside OLDER trees to re-manufacture a frozen baseline, and a
// tree with no theme noun has no themeListData to name.
func init() {
	registerSurfacePayloadType("themeListData", themeListData{})
	registerSurfacePayloadType("themeExportData", themeExportData{})
}

// TestThemeLeavesNeedNoProject: both leaves answer with no project.config.yaml
// anywhere up the tree.
//
// It is the property the noun exists for. Choosing a palette is something a
// person does while SETTING DossierX UP, and every other noun in this surface
// refuses with config_not_found before it does anything. A theme command that
// needed a project would be answerable only after the decision it informs had
// already been made.
func TestThemeLeavesNeedNoProject(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	for _, args := range [][]string{
		{"theme", "list"},
		{"theme", "export", "claude"},
	} {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			env, _, err := execCLIJSON(t, args...)
			if err != nil {
				t.Fatalf("%s in a directory with no project: %v", name, err)
			}
			if !env.OK {
				t.Fatalf("%s must succeed with no project, got %+v", name, env.Error)
			}
		})
	}
}

// TestThemeListDescribesEveryPreset keeps themePresetDescriptions honest
// against the registry it describes. A preset added to internal/config without
// a sentence here would list with a blank description, which reads as "this
// preset does nothing in particular" rather than as the omission it is.
func TestThemeListDescribesEveryPreset(t *testing.T) {
	names := config.PresetNames()
	if len(names) == 0 {
		t.Fatal("config.PresetNames() is empty; the whole noun has nothing to report")
	}
	for _, name := range names {
		if strings.TrimSpace(themePresetDescriptions[name]) == "" {
			t.Errorf("preset %q has no description in themePresetDescriptions", name)
		}
	}
	for name := range themePresetDescriptions {
		if _, ok := config.Preset(name); !ok {
			t.Errorf("themePresetDescriptions describes %q, which is not a preset", name)
		}
	}

	env, _, err := execCLIJSON(t, "theme", "list")
	if err != nil {
		t.Fatalf("theme list: %v", err)
	}
	var data themeListData
	envData(t, env, &data)
	if data.Count != len(names) {
		t.Fatalf("theme list reported %d preset(s), the registry has %d", data.Count, len(names))
	}
	for _, p := range data.Presets {
		if p.Description == "" {
			t.Errorf("preset %q listed with an empty description", p.Name)
		}
		if len(p.Tokens) == 0 {
			t.Errorf("preset %q listed as setting no tokens", p.Name)
		}
		// Every reported token has to be one the engine would accept, or the
		// list is advertising a key that cannot load.
		allowed := map[string]bool{}
		for _, k := range config.ThemeTokenAllowlist {
			allowed[k] = true
		}
		for _, tok := range p.Tokens {
			if !allowed[tok] {
				t.Errorf("preset %q reports token %q, which is not in ThemeTokenAllowlist", p.Name, tok)
			}
		}
	}
}

// TestThemeExportUnknownPresetIsItsOwnRefusal: a name the binary does not carry
// gets unknown_preset and a hint naming what it does carry.
//
// The hint is the assertion that matters. The realistic cause is a typo or a
// binary older than the preset, and in both cases the recovery is reading the
// list — so a refusal that did not name the alternatives would send the caller
// to the documentation this noun exists to replace.
func TestThemeExportUnknownPresetIsItsOwnRefusal(t *testing.T) {
	env, _, err := execCLIJSON(t, "theme", "export", "clode")
	if err == nil {
		t.Fatal("an unknown preset must fail; it did not")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUnknownPreset {
		t.Fatalf("want error.code %q, got %+v", cliout.CodeUnknownPreset, env.Error)
	}
	if exitStatusFor(err) != 1 {
		t.Fatalf("unknown_preset exits 1, got %d", exitStatusFor(err))
	}
	for _, name := range config.PresetNames() {
		if !strings.Contains(env.Error.Hint, name) {
			t.Errorf("hint %q does not name the real preset %q", env.Error.Hint, name)
		}
	}
}

// TestThemeExportWithNoPathReturnsYAMLInTheEnvelope: the content rides in
// data.yaml, not on a bare stdout.
//
// This is the machine contract and not a formatting preference. A leaf that
// printed raw YAML under --format json would be the one invocation in the
// surface whose stdout is not an envelope, and every consumer that decodes
// stdout would break on it.
func TestThemeExportWithNoPathReturnsYAMLInTheEnvelope(t *testing.T) {
	env, _, err := execCLIJSON(t, "theme", "export", "claude")
	if err != nil {
		t.Fatalf("theme export claude: %v", err)
	}
	var data themeExportData
	envData(t, env, &data)
	if data.Preset != "claude" {
		t.Errorf("data.preset = %q, want %q", data.Preset, "claude")
	}
	if data.YAML == "" {
		t.Fatal("data.yaml is empty; the no-path form has nothing else to return")
	}
	if data.Path != "" || data.Bytes != 0 {
		t.Errorf("the no-path form must not report a path or a byte count, got %q/%d", data.Path, data.Bytes)
	}
	// execCLIJSON decodes stdout as one envelope and fails if it is not, so
	// reaching this line is itself the assertion that the YAML did not go out
	// on a bare stdout.
}

// TestThemeExportOutputLoadsAsATheme is the only assertion the exported bytes
// really owe: a project that points `extends` at them loads.
//
// It goes through config.DecodeConfig and ValidateTheme rather than through a
// YAML parse, because "it is valid YAML" is not the claim — the claim is that
// every value passes the grammars, every key is in the vocabulary, and the file
// is accepted where a theme file is accepted. Asserting the parse would pass on
// a file the engine refuses.
func TestThemeExportOutputLoadsAsATheme(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "themes", "mine.yaml")

	env, _, err := execCLIJSON(t, "theme", "export", "claude", themePath)
	if err != nil {
		t.Fatalf("theme export claude <path>: %v", err)
	}
	var data themeExportData
	envData(t, env, &data)
	if data.YAML != "" {
		t.Error("the path form must not also carry the content; the file is the answer")
	}
	raw, readErr := os.ReadFile(themePath)
	if readErr != nil {
		t.Fatalf("read exported theme: %v", readErr)
	}
	if data.Bytes != len(raw) {
		t.Errorf("data.bytes = %d, the file is %d bytes", data.Bytes, len(raw))
	}
	// An exported theme file must not chain: the decoder refuses `extends`
	// inside one, so emitting it would produce a file that cannot load.
	if strings.Contains(string(raw), "\nextends:") {
		t.Error("the exported file carries an extends: key, which a theme file may not have")
	}

	cfgPath := filepath.Join(root, "project.config.yaml")
	mustWriteThemeFile(t, cfgPath, strings.Join([]string{
		"schema_version: 1",
		"title: Theme export fixture",
		"claims_dir: claims",
		"facets:",
		"  - contract",
		"modules:",
		"  - widget",
		"viewer:",
		"  theme:",
		"    extends: themes/mine.yaml",
		"",
	}, "\n"))
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("the exported theme does not load through a project config: %v", err)
	}
	rt, err := config.ResolveTheme(cfg, os.ReadFile)
	if err != nil {
		t.Fatalf("the exported theme does not resolve: %v", err)
	}
	if len(rt.Shared)+len(rt.Light)+len(rt.Dark) == 0 {
		t.Fatal("the exported theme resolved to no declarations at all")
	}
}

// TestThemeExportRefusesToClobber: an existing file is write_conflict, and
// --force is the way past it.
//
// The refusal is the whole reason export is worth having. The file it writes is
// one the project EDITS; a second run of a half-remembered command that
// silently replaced it would discard those edits, and nothing in the resulting
// file would show that it had happened.
func TestThemeExportRefusesToClobber(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mine.yaml")
	mustWriteThemeFile(t, path, "paper: '#123456'\n")

	env, _, err := execCLIJSON(t, "theme", "export", "claude", path)
	if err == nil {
		t.Fatal("exporting over an existing file must fail; it did not")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeWriteConflict {
		t.Fatalf("want error.code %q, got %+v", cliout.CodeWriteConflict, env.Error)
	}
	if got, _ := os.ReadFile(path); string(got) != "paper: '#123456'\n" {
		t.Fatalf("the refused export changed the file anyway: %q", string(got))
	}

	if _, _, err := execCLIJSON(t, "theme", "export", "claude", path, "--force"); err != nil {
		t.Fatalf("--force must overwrite: %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read forced export: %v", readErr)
	}
	if !strings.Contains(string(got), "# DossierX theme file") {
		t.Fatalf("--force did not replace the file, got:\n%s", string(got))
	}
}

// TestThemeExportIsDeterministic: two exports of the same preset from the same
// binary are byte-identical.
//
// It is what makes a diff in a checked-in theme file mean something. The header
// carries no version and no timestamp precisely so that the only thing that can
// move these bytes is the preset itself moving, which is a CHANGELOG-worthy
// event rather than a re-run.
func TestThemeExportIsDeterministic(t *testing.T) {
	first := renderThemeFile("claude", mustPreset(t, "claude"))
	for i := 0; i < 20; i++ {
		if got := renderThemeFile("claude", mustPreset(t, "claude")); got != first {
			t.Fatalf("export %d differs from the first", i)
		}
	}
	// Token order is the allowlist's, which is what lets a reader compare the
	// file against FORMAT.md's table and the emitted :root block as one
	// sequence. Checked on the light block, the only one that carries enough
	// tokens for an ordering to be visible.
	lines := strings.Split(first, "\n")
	var seen []string
	inLight := false
	for _, line := range lines {
		switch {
		case line == "light:":
			inLight = true
		case inLight && strings.HasPrefix(line, "  ") && strings.Contains(line, ":"):
			seen = append(seen, strings.TrimSpace(strings.SplitN(strings.TrimSpace(line), ":", 2)[0]))
		case inLight && !strings.HasPrefix(line, "  "):
			inLight = false
		}
	}
	if len(seen) == 0 {
		t.Fatal("no light: block found in the exported theme")
	}
	pos := map[string]int{}
	for i, k := range config.ThemeTokenAllowlist {
		pos[k] = i
	}
	for i := 1; i < len(seen); i++ {
		if pos[seen[i-1]] >= pos[seen[i]] {
			t.Fatalf("light block is not in ThemeTokenAllowlist order: %q before %q", seen[i-1], seen[i])
		}
	}
}

// mustWriteThemeFile writes a fixture file, creating its parent directory.
func mustWriteThemeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustPreset(t *testing.T, name string) config.Theme {
	t.Helper()
	p, ok := config.Preset(name)
	if !ok {
		t.Fatalf("preset %q is not registered", name)
	}
	return p
}
