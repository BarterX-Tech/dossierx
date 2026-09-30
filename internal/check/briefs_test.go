// briefs_test.go pins how the brief rule set (internal/briefs, NIT-204) rides
// through the check pipeline: its findings join lint_findings in every mode,
// each mode reads the briefs from the SAME tree it reads the claims from — the
// working tree for Run and Status, the git index for StatusStaged — and a brief
// error stops the writing run at the lint step like any claim error.
package check_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
)

const cleanBrief = "---\nsummary: How the widget flow reads end to end.\nrests_on:\n  - widget.contract.overview\n---\n# Widget flow\n\nText.\n"

// briefRulesIn keeps only the brief findings of a verdict, by name.
func briefRulesIn(verdict []string) []string {
	var out []string
	for _, line := range verdict {
		if strings.HasPrefix(line, "lint|brief-") {
			parts := strings.SplitN(line, "|", 4)
			out = append(out, parts[1]+" "+parts[2])
		}
	}
	return out
}

// TestBriefFindingsFollowTheTreeEachModeJudges is the parity contract for the
// brief rules: --validate judges the working tree and --staged the index, so an
// unstaged edit that breaks a brief is visible to the first and not the second,
// and staging it makes both agree. A regression that read briefs from the
// working tree under --staged (the bypass staged.go exists to close) fails the
// second assertion.
func TestBriefFindingsFollowTheTreeEachModeJudges(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	cfg := writeProjectFiles(t, repo, baseConfig, map[string]string{
		"claims/overview.yaml":    draftClaim("widget.contract.overview"),
		"briefs/widget/flow.md":   cleanBrief,
		"briefs/widget/.DS_Store": "finder litter",
	})
	gitRepo(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "fixture")

	if got := briefRulesIn(worktreeVerdict(t, cfg)); len(got) != 0 {
		t.Fatalf("a clean brief must raise nothing under --validate, got %v", got)
	}
	if got, _ := stagedVerdict(t, cfg); len(briefRulesIn(got)) != 0 {
		t.Fatalf("a clean brief must raise nothing under --staged, got %v", briefRulesIn(got))
	}

	broken := strings.Replace(cleanBrief, "widget.contract.overview", "widget.contract.ghost", 1)
	writeFixtureFile(t, filepath.Join(repo, "briefs", "widget", "flow.md"), broken)
	if err := os.WriteFile(filepath.Join(repo, "briefs", "loose.md"), []byte(cleanBrief), 0o644); err != nil {
		t.Fatal(err)
	}

	want := []string{"brief-shape briefs/loose.md", "brief-rests-on-unknown briefs/widget/flow.md"}
	if got := briefRulesIn(worktreeVerdict(t, cfg)); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("--validate must see the unstaged breakage: got %v, want %v", got, want)
	}
	if got, _ := stagedVerdict(t, cfg); len(briefRulesIn(got)) != 0 {
		t.Fatalf("--staged must judge the index, which is still clean; got %v", briefRulesIn(got))
	}

	git(t, repo, "add", "-A")
	if got, _ := stagedVerdict(t, cfg); strings.Join(briefRulesIn(got), ",") != strings.Join(want, ",") {
		t.Fatalf("once staged, --staged must agree with --validate: got %v, want %v", briefRulesIn(got), want)
	}
}

// TestBriefSymlinksAreRefusedInBothModes pins --staged's parity with
// --validate on the entries the claims registry drops from the index: a
// symlink (mode 120000) under briefs_dir. The working tree refuses a symlinked
// brief, a symlinked image, a symlinked folder and a symlinked briefs_dir as
// brief-shape; the index must refuse exactly the same paths, where it used to
// drop the entries and pass a tree --validate refused. The root case also pins
// that a linked briefs_dir is refused rather than read as no briefs.
func TestBriefSymlinksAreRefusedInBothModes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		links map[string]string // link path (repo-relative) -> target
		want  []string
	}{
		{
			name: "brief, image and folder",
			links: map[string]string{
				"briefs/widget/linked.md": "../../elsewhere/real.md",
				"briefs/widget/pic.svg":   "../../elsewhere/pic.svg",
				"briefs/other":            "../elsewhere",
			},
			want: []string{
				"brief-shape briefs/other",
				"brief-shape briefs/widget/flow.md",
				"brief-shape briefs/widget/linked.md",
				"brief-shape briefs/widget/pic.svg",
			},
		},
		{
			name:  "briefs_dir itself",
			links: map[string]string{"briefs": "elsewhere"},
			want:  []string{"brief-shape briefs/"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "repo")
			files := map[string]string{
				"claims/overview.yaml":  draftClaim("widget.contract.overview"),
				"elsewhere/real.md":     cleanBrief,
				"elsewhere/pic.svg":     "<svg/>",
				"elsewhere/widget/x.md": cleanBrief,
			}
			if _, isRoot := tc.links["briefs"]; !isRoot {
				// flow.md references the symlinked image, so it is refused
				// too: a link is not an image its folder holds.
				files["briefs/widget/flow.md"] = cleanBrief + "\n![Pic](pic.svg)\n"
			}
			cfg := writeProjectFiles(t, repo, baseConfig, files)
			for link, target := range tc.links {
				if err := os.Symlink(target, filepath.Join(repo, filepath.FromSlash(link))); err != nil {
					t.Fatal(err)
				}
			}
			gitRepo(t, repo)
			git(t, repo, "add", "-A")
			git(t, repo, "commit", "-qm", "fixture")
			for link := range tc.links {
				if mode := strings.Fields(git(t, repo, "ls-files", "-s", "--", link)); len(mode) == 0 || mode[0] != "120000" {
					t.Fatalf("fixture precondition: %s must be a symlink in the index, got %v", link, mode)
				}
			}

			worktree := briefRulesIn(worktreeVerdict(t, cfg))
			staged, _ := stagedVerdict(t, cfg)
			if strings.Join(worktree, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("--validate: got %v, want %v", worktree, tc.want)
			}
			if got := briefRulesIn(staged); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("--staged must refuse the same links --validate refuses: got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRunStopsAtLintOnABriefError pins that a brief ERROR is a lint error of
// the writing run: it stops before the catalog and the viewer, exactly as a
// claim's would — the caps are final, not advisory.
func TestRunStopsAtLintOnABriefError(t *testing.T) {
	cfg, claims := project(t, baseConfig+"max_brief_words: 2\n", map[string]string{
		"claims/overview.yaml":  draftClaim("widget.contract.overview"),
		"briefs/widget/flow.md": cleanBrief,
	})
	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatal("an over-cap brief must fail check")
	}
	if res.RenderPath != "" || res.CatalogPath != "" {
		t.Fatalf("check must stop at lint, before any write; wrote catalog %q viewer %q", res.CatalogPath, res.RenderPath)
	}
	found := false
	for _, f := range res.ClaimLintErrors() {
		if f.LintName == "brief-word-cap" && f.ClaimID == "briefs/widget/flow.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a brief-word-cap error on briefs/widget/flow.md, got %+v", res.LintErrors)
	}
}
