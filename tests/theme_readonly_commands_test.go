// theme_readonly_commands_test.go pins two properties of a project whose
// viewer.theme is BROKEN, and they pull in opposite directions on purpose.
//
//	the three check modes must all refuse it, with the SAME error code
//	every other command must carry on as if nothing were wrong
//
// The pair is the whole design. A theme is a presentation concern: it decides
// what the viewer looks like and nothing about what the corpus says, so a
// mistyped colour must not stand between an agent and `dossierx claim show`.
// But `dossierx check` is the command that PRODUCES the viewer, and its
// read-only modes are what a pre-commit hook and a CI job run, so a theme that
// cannot resolve has to stop all three of them or the gate is green over a
// document nobody can build.
//
// The fixture's defect is a font file with the wrong signature — a .woff2 whose
// bytes are not a woff2. It is chosen because it is the failure a browser
// handles WORST: handed a mislabelled face it drops it silently and renders a
// fallback nobody chose, with no error anywhere. It is also the one theme
// defect that cannot be caught by reading project.config.yaml, so a suite that
// only tested a mistyped token name would not be testing the rule that needs
// the engine.
package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newThemedProject builds a project that is otherwise clean and whose
// viewer.theme is whatever themeBlock says.
func newThemedProject(t *testing.T, module, themeBlock string) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureProject(t, root, module)
	cfgPath := filepath.Join(root, "project.config.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte(string(raw)+themeBlock), 0o644); err != nil {
		t.Fatalf("write themed config: %v", err)
	}
	return root
}

// newBadPresetProject names a preset this binary does not carry.
//
// It is the second fixture because it is the theme failure a reader is most
// likely to CAUSE — a preset name is the one thing in the block somebody types
// from memory — and because it is the one whose code was argued over: it is
// invalid_config here and unknown_preset from "dossierx theme export", and the
// difference is the recovery. See cliout.CodeUnknownPreset.
func newBadPresetProject(t *testing.T) string {
	t.Helper()
	return newThemedProject(t, "badpreset", "viewer:\n  theme:\n    preset: clode\n")
}

// newBadFontProject builds a project that is otherwise clean and whose only
// fault is one font file whose bytes do not match its extension.
func newBadFontProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureProject(t, root, "badfont")

	if err := os.MkdirAll(filepath.Join(root, "fonts"), 0o755); err != nil {
		t.Fatalf("mkdir fonts: %v", err)
	}
	// "NOPE" where a woff2's "wOF2" belongs. Everything else about the
	// declaration is correct, so nothing but the signature rule can refuse it.
	if err := os.WriteFile(filepath.Join(root, "fonts", "probe.woff2"), []byte("NOPE-not-a-woff2"), 0o644); err != nil {
		t.Fatalf("write bad font: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	themed := string(raw) + strings.Join([]string{
		"viewer:",
		"  theme:",
		"    font-sans: '\"Probe Face\", sans-serif'",
		"    fonts:",
		"      - family: Probe Face",
		"        src: fonts/probe.woff2",
		"",
	}, "\n")
	if err := os.WriteFile(cfgPath, []byte(themed), 0o644); err != nil {
		t.Fatalf("write themed config: %v", err)
	}
	return root
}

// envelopeOf runs the binary and decodes its stdout as one envelope.
func envelopeOf(t *testing.T, dir string, args ...string) (code int, ok bool, errCode, stoppedAt string, data map[string]any) {
	t.Helper()
	stdout, stderr, code := run(t, dir, append([]string{"--format", "json"}, args...)...)
	var env struct {
		OK    bool           `json:"ok"`
		Data  map[string]any `json:"data"`
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
		StoppedAt string `json:"stopped_at"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("%v: stdout is not one envelope (%v)\nstdout:\n%s\nstderr:\n%s", args, err, stdout, stderr)
	}
	if env.Error != nil {
		errCode = env.Error.Code
	}
	return code, env.OK, errCode, env.StoppedAt, env.Data
}

// TestBrokenThemeRefusesTheSameWayInAllThreeCheckModes requires all three check
// modes to refuse a broken theme with the same error code.
//
// The code is the assertion, not merely the refusal. `check`, `check --validate`
// and `check --staged` are three doors onto one rule set, and a skill branches
// on `error.code` and nothing else — so the same defect arriving as
// invalid_config through two doors and write_failed through the third is a
// contract break whether or not every door refuses. write_failed is documented
// as a filesystem failure and its recovery is "check permissions", which for a
// mislabelled font file sends an agent to inspect a disk that is fine.
func TestBrokenThemeRefusesTheSameWayInAllThreeCheckModes(t *testing.T) {
	for _, fixture := range []struct {
		name  string
		build func(*testing.T) string
	}{
		// Two shapes of the same defect class, and the second is here to pin a
		// decision rather than a mechanism: a preset this binary does not carry
		// reports invalid_config from every check mode, NOT unknown_preset.
		// unknown_preset belongs to "dossierx theme export"'s positional
		// argument, which loads no project and whose recovery is "run theme
		// list", where this one's recovery is "edit the config" — the same
		// recovery every other theme failure has.
		{"a font whose bytes are not its extension", newBadFontProject},
		{"a preset this binary does not carry", newBadPresetProject},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			runBrokenThemeModes(t, fixture.build(t))
		})
	}
}

func runBrokenThemeModes(t *testing.T, root string) {
	t.Helper()
	gitInitAndAddAll(t, root)

	for _, args := range [][]string{
		{"check"},
		{"check", "--validate"},
		{"check", "--staged"},
	} {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			exit, ok, errCode, stoppedAt, data := envelopeOf(t, root, args...)
			if ok || exit == 0 {
				t.Fatalf("%s passed a project whose viewer cannot be rendered (exit %d, ok %v); a gate that is green over a document nobody can build is worse than no gate", name, exit, ok)
			}
			if errCode != "invalid_config" {
				t.Errorf("%s: error.code = %q, want %q — all three modes run one theme rule set and must name the same recovery", name, errCode, "invalid_config")
			}
			if stoppedAt != "render" {
				t.Errorf("%s: stopped_at = %q, want %q", name, stoppedAt, "render")
			}
			themeErr, isString := data["theme_error"].(string)
			if !isString || themeErr == "" {
				t.Errorf("%s: data.theme_error is empty; the field that names the offending declaration is the only place a caller can read WHICH font failed", name)
			}
			// Nothing was accepted, so the two counters must not report a face
			// the run refused.
			if v, present := data["theme_font_count"]; present {
				t.Errorf("%s: data.theme_font_count is present (%v) on a refused theme; nothing was accepted", name, v)
			}
			if v, present := data["theme_font_bytes"]; present {
				t.Errorf("%s: data.theme_font_bytes is present (%v) on a refused theme", name, v)
			}
		})
	}
}

// TestBrokenThemeDoesNotGateReadCommands requires every non-rendering command
// to succeed on a project whose theme does not resolve.
//
// A theme decides what the viewer looks like and nothing about what the corpus
// says. Resolving it costs a read of every font file, so the temptation is to
// do it once at config-load time where every command would pay for it — and the
// cost is not the reason not to. The reason is that an agent whose `claim show`
// started failing because somebody committed a bad .woff2 has no way to reach
// the claim it was asked about, and nothing in the refusal would tell it that
// the claim is fine.
//
// Only `check` and `serve` produce the viewer, so only they resolve the theme.
func TestBrokenThemeDoesNotGateReadCommands(t *testing.T) {
	root := newBadFontProject(t)

	for _, args := range [][]string{
		{"claim", "show", "badfont.contract.overview"},
		{"claim", "list"},
		{"comment", "list", "badfont.contract.overview"},
		{"comment", "inbox"},
		{"build-order", "status", "--module", "badfont"},
		{"track", "list"},
	} {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			exit, ok, errCode, _, _ := envelopeOf(t, root, args...)
			if !ok || exit != 0 {
				t.Fatalf("%s failed (exit %d, error.code %q) on a project whose only fault is a font file; a theme must not stand between an agent and the corpus", name, exit, errCode)
			}
		})
	}
}

// gitInitAndAddAll makes root a git work tree with everything staged, which is
// what `check --staged` needs to have anything to judge. A project with no
// index would be SKIPPED by --staged at exit 0 (data.skipped), and a skipped
// gate reported as a pass is the one result this repository refuses to have.
func gitInitAndAddAll(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "."},
		{"config", "user.email", "fixture@example.invalid"},
		{"config", "user.name", "Fixture"},
		{"add", "-A"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	// A --staged run that reports data.skipped judged nothing, and the
	// assertions above would then be made against a verdict nobody reached.
	_, _, _, _, data := envelopeOf(t, root, "check", "--staged")
	skipped, isBool := data["skipped"].(bool)
	if isBool && skipped {
		t.Fatal("check --staged reports skipped:true in this fixture, so its refusal below would be over an index it never read")
	}
}
