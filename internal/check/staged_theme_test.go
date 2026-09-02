// staged_theme_test.go covers the promise that "check --staged" runs the SAME
// theme rules as the other two modes, against the INDEX's bytes.
//
// That promise is what makes the theme part of what a commit is judged on. If
// --staged read the working tree, a project could stage a config naming a
// theme file or a font that the commit does not carry, or stage a corrupt font
// while a good one sits on disk beside it, and the pre-commit hook would pass
// on evidence the commit does not contain.
package check_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// themeConfig is baseConfig plus a viewer.theme that names one font.
const themeConfig = baseConfig +
	"viewer:\n  theme:\n    font-sans: 'Probe, sans-serif'\n" +
	"    fonts:\n      - family: Probe\n        src: fonts/probe.woff2\n"

// goodFont/badFont differ only in the four signature bytes: goodFont is a
// woff2 as far as config's rule is concerned, badFont is a file with a woff2
// extension that is not one. A browser handed badFont as a data: URL drops the
// face silently and renders a fallback, which is why the signature is checked
// at all.
var goodFont = []byte("wOF2-payload")
var badFont = []byte("NOPE-payload")

func writeAt(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// themeRepo writes a themed project whose font holds indexContent in the
// index, commits it, then leaves worktreeContent on disk. Passing the same
// bytes for both is the ordinary clean case.
func themeRepo(t *testing.T, indexContent, worktreeContent []byte) *config.Config {
	t.Helper()
	cfg, _ := project(t, themeConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	font := filepath.Join(cfg.Dir(), "fonts", "probe.woff2")
	writeAt(t, font, indexContent)
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")
	writeAt(t, font, worktreeContent)
	return cfg
}

// TestStagedTheme_SignatureIsCheckedAgainstTheIndex is the case the injected
// reader exists for. The commit carries a font that is not a font; the working
// tree holds a good one. --staged must refuse, and --validate (which reads the
// working tree, correctly) must not — the two are looking at different trees
// and are both right about the one they see.
func TestStagedTheme_SignatureIsCheckedAgainstTheIndex(t *testing.T) {
	cfg := themeRepo(t, badFont, goodFont)

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("check.Staged: %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	if res.ThemeError == "" {
		t.Fatalf("--staged accepted a font whose STAGED bytes are not a woff2; ok=%v", res.OK)
	}
	if !strings.Contains(res.ThemeError, "signature") {
		t.Errorf("--staged theme error does not name the signature rule: %s", res.ThemeError)
	}
	if res.OK {
		t.Errorf("--staged reported ok:true with a theme error: %s", res.ThemeError)
	}

	// The same tree through the worktree reader: the file on disk IS a woff2,
	// so there is nothing to refuse.
	worktree := check.Status(sp.Claims, cfg)
	if worktree.ThemeError != "" {
		t.Errorf("--validate refused the working tree's good font: %s", worktree.ThemeError)
	}
	if worktree.ThemeFontCount != 1 || worktree.ThemeFontBytes != int64(len(goodFont)) {
		t.Errorf("--validate reported %d font(s)/%d bytes, want 1/%d",
			worktree.ThemeFontCount, worktree.ThemeFontBytes, len(goodFont))
	}
}

// TestStagedTheme_UnstagedFileIsNotAnEmptyFile pins what happens when the
// index has no entry for a file the theme names. Answering with zero bytes
// would grade the commit on a theme nobody wrote, and answering from the
// working tree would grade it on content the commit does not carry; the run
// says so and names the fix.
func TestStagedTheme_UnstagedFileIsNotAnEmptyFile(t *testing.T) {
	cfg, _ := project(t, themeConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "claims", "project.config.yaml")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")
	// The font exists on disk and is perfectly valid — it is simply not in
	// the index, which is the whole point.
	writeAt(t, filepath.Join(cfg.Dir(), "fonts", "probe.woff2"), goodFont)

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("check.Staged: %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	if res.ThemeError == "" {
		t.Fatalf("--staged accepted a theme naming a font the commit does not carry; ok=%v", res.OK)
	}
	if !strings.Contains(res.ThemeError, "is not staged (git add it)") {
		t.Errorf("--staged error does not say the file is unstaged or how to fix it: %s", res.ThemeError)
	}
}

// TestStagedTheme_CleanProjectReportsTheFontBudget is the positive control for
// both tests above: with the same bytes in the index and the working tree,
// --staged accepts, and it reports the same font count and byte total the
// worktree modes do. Without this, a rule that refused everything would pass
// the two refusal tests.
func TestStagedTheme_CleanProjectReportsTheFontBudget(t *testing.T) {
	cfg := themeRepo(t, goodFont, goodFont)

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("check.Staged: %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	if res.ThemeError != "" {
		t.Fatalf("--staged refused a clean themed project: %s", res.ThemeError)
	}
	if res.ThemeFontCount != 1 || res.ThemeFontBytes != int64(len(goodFont)) {
		t.Errorf("--staged reported %d font(s)/%d bytes, want 1/%d",
			res.ThemeFontCount, res.ThemeFontBytes, len(goodFont))
	}
}

// TestStagedTheme_HandBuiltStagedProjectWillNotReadTheWorktree guards the
// fallback that must not exist. A StagedProject that did not come from
// check.Staged has no index reader; falling back to os.ReadFile there would
// reintroduce, quietly, exactly the worktree bypass this file is about.
func TestStagedTheme_HandBuiltStagedProjectWillNotReadTheWorktree(t *testing.T) {
	cfg, claims := project(t, themeConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	writeAt(t, filepath.Join(cfg.Dir(), "fonts", "probe.woff2"), goodFont)

	res := check.StatusStaged(check.StagedProject{Config: cfg, Claims: claims}, cfg)
	if res.ThemeError == "" {
		t.Fatalf("a StagedProject with no index reader read the theme anyway; ok=%v", res.OK)
	}
	if !strings.Contains(res.ThemeError, "no git index reader") {
		t.Errorf("error does not say the reader was missing: %s", res.ThemeError)
	}
}

// TestTheme_UnthemedProjectReportsNothing is the property most projects rely
// on: no viewer.theme means no theme error, no fonts, and no change to any
// other field.
func TestTheme_UnthemedProjectReportsNothing(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	res := check.Status(claims, cfg)
	if res.ThemeError != "" || res.ThemeFontCount != 0 || res.ThemeFontBytes != 0 {
		t.Errorf("unthemed project reported theme state: err=%q count=%d bytes=%d",
			res.ThemeError, res.ThemeFontCount, res.ThemeFontBytes)
	}
	if !res.OK {
		t.Errorf("unthemed project is not ok: %+v", res.LintFindings)
	}
}
