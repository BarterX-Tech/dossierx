// briefs_test.go pins how the brief rule set (internal/briefs, NIT-204) rides
// through the check pipeline: its findings join lint_findings in every mode,
// each mode reads the briefs from the SAME tree it reads the claims from — the
// working tree for Run and Status, the git index for StatusStaged — and a brief
// error stops the writing run at the lint step like any claim error.
package check_test

import (
	"os"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix index semantics")
	}
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

// TestBriefSubmodulesAreRefusedInBothModes is the submodule half of the parity
// above: another repository checked out where a brief folder, or briefs_dir
// itself, should be. The index holds it as one gitlink (mode 160000), which
// --staged refuses as brief-shape; the working tree holds a directory whose
// .git git recognises, which --validate must refuse as the same entry rather
// than read the other repository's files as this project's briefs.
//
// The rule is git's, not "any .git entry": a .git file that does not name a git
// directory, or a .git directory that is not one, is skipped by `git add`,
// which stages the folder's files as blobs (100644). Those rows must read the
// folder normally in BOTH modes. Every row therefore asserts the index mode git
// itself produced for the folder (the gitlink) or its brief (the blob), so the
// test pins the engine against git, not against a guess about git.
//
// The brief inside the folder rests on an unknown claim, so READING it raises
// brief-rests-on-unknown and refusing it raises only brief-shape: a mode that
// reads a folder the other refuses fails on the difference, in either
// direction.
func TestBriefSubmodulesAreRefusedInBothModes(t *testing.T) {
	readBrief := strings.Replace(cleanBrief, "widget.contract.overview", "widget.contract.ghost", 1)
	writeIn := func(t *testing.T, dir string, files map[string]string) {
		t.Helper()
		for rel, body := range files {
			abs := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFixtureFile(t, abs, body)
		}
	}
	folderBrief := map[string]string{"flow.md": readBrief}
	rootBrief := map[string]string{"widget/flow.md": readBrief}
	const headRef = "ref: refs/heads/main\n"
	for _, tc := range []struct {
		name     string
		checkout string // repo-relative path of the folder under test
		// setup builds the folder at checkout (repo is already a git repo).
		setup     func(t *testing.T, repo, checkout string)
		indexPath string // the path whose index mode git produced
		indexMode string // 160000: a gitlink; 100644: git staged the files
		want      []string
	}{
		{
			name:     "a submodule added as a brief folder",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, repo, checkout string) {
				other := filepath.Join(t.TempDir(), "other")
				nestedRepo(t, other, folderBrief)
				git(t, repo, "-c", "protocol.file.allow=always", "submodule", "add", "-q", other, "briefs/vendored")
				if info, err := os.Lstat(filepath.Join(checkout, ".git")); err != nil || !info.Mode().IsRegular() {
					t.Fatalf("fixture precondition: a submodule checkout carries a .git file, got %v %v", info, err)
				}
			},
			indexPath: "briefs/vendored",
			indexMode: "160000",
			want:      []string{"brief-shape briefs/vendored"},
		},
		{
			name:     "an embedded repository as a brief folder",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, _, checkout string) {
				nestedRepo(t, checkout, folderBrief)
			},
			indexPath: "briefs/vendored",
			indexMode: "160000",
			want:      []string{"brief-shape briefs/vendored"},
		},
		{
			name:     "an embedded repository as briefs_dir itself",
			checkout: "briefs",
			setup: func(t *testing.T, _, checkout string) {
				nestedRepo(t, checkout, rootBrief)
			},
			indexPath: "briefs",
			indexMode: "160000",
			want:      []string{"brief-shape briefs/"},
		},
		{
			// A git directory with a detached HEAD and no commit behind it
			// still passes is_git_directory: git records the gitlink at the
			// named object. It pins the object-name branch of the HEAD rule.
			name:     "a git directory with a detached HEAD",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, _, checkout string) {
				writeIn(t, checkout, folderBrief)
				writeIn(t, checkout, map[string]string{".git/HEAD": strings.Repeat("ab", 20) + "\n", ".git/objects/.keep": "", ".git/refs/.keep": ""})
			},
			indexPath: "briefs/vendored",
			indexMode: "160000",
			want:      []string{"brief-shape briefs/vendored"},
		},
		{
			name:     "an empty .git file",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, _, checkout string) {
				writeIn(t, checkout, folderBrief)
				writeIn(t, checkout, map[string]string{".git": ""})
			},
			indexPath: "briefs/vendored/flow.md",
			indexMode: "100644",
			want:      []string{"brief-rests-on-unknown briefs/vendored/flow.md"},
		},
		{
			name:     "a junk .git file",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, _, checkout string) {
				writeIn(t, checkout, folderBrief)
				writeIn(t, checkout, map[string]string{".git": "not a gitdir\n"})
			},
			indexPath: "briefs/vendored/flow.md",
			indexMode: "100644",
			want:      []string{"brief-rests-on-unknown briefs/vendored/flow.md"},
		},
		{
			name:     "a .git file naming a directory that is not a git directory",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, repo, checkout string) {
				writeIn(t, checkout, folderBrief)
				writeIn(t, checkout, map[string]string{".git": "gitdir: ../../elsewhere\n"})
				writeIn(t, repo, map[string]string{"elsewhere/HEAD": headRef})
			},
			indexPath: "briefs/vendored/flow.md",
			indexMode: "100644",
			want:      []string{"brief-rests-on-unknown briefs/vendored/flow.md"},
		},
		{
			name:     "an empty .git directory",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, _, checkout string) {
				writeIn(t, checkout, folderBrief)
				if err := os.Mkdir(filepath.Join(checkout, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			indexPath: "briefs/vendored/flow.md",
			indexMode: "100644",
			want:      []string{"brief-rests-on-unknown briefs/vendored/flow.md"},
		},
		{
			name:     "a .git directory holding HEAD and objects but no refs",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, _, checkout string) {
				writeIn(t, checkout, folderBrief)
				writeIn(t, checkout, map[string]string{".git/HEAD": headRef, ".git/objects/.keep": ""})
			},
			indexPath: "briefs/vendored/flow.md",
			indexMode: "100644",
			want:      []string{"brief-rests-on-unknown briefs/vendored/flow.md"},
		},
		{
			name:     "a .git directory with objects and refs but a junk HEAD",
			checkout: "briefs/vendored",
			setup: func(t *testing.T, _, checkout string) {
				writeIn(t, checkout, folderBrief)
				writeIn(t, checkout, map[string]string{".git/HEAD": "junk\n", ".git/objects/.keep": "", ".git/refs/.keep": ""})
			},
			indexPath: "briefs/vendored/flow.md",
			indexMode: "100644",
			want:      []string{"brief-rests-on-unknown briefs/vendored/flow.md"},
		},
		{
			name:     "an empty .git directory in briefs_dir itself",
			checkout: "briefs",
			setup: func(t *testing.T, _, checkout string) {
				writeIn(t, checkout, rootBrief)
				if err := os.Mkdir(filepath.Join(checkout, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			indexPath: "briefs/widget/flow.md",
			indexMode: "100644",
			want:      []string{"brief-rests-on-unknown briefs/widget/flow.md"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "repo")
			files := map[string]string{"claims/overview.yaml": draftClaim("widget.contract.overview")}
			if tc.checkout != "briefs" {
				files["briefs/widget/flow.md"] = cleanBrief
			}
			cfg := writeProjectFiles(t, repo, baseConfig, files)
			gitRepo(t, repo)
			tc.setup(t, repo, filepath.Join(repo, filepath.FromSlash(tc.checkout)))
			git(t, repo, "add", "-A")
			git(t, repo, "commit", "-qm", "fixture")
			requireMode(t, repo, tc.indexPath, tc.indexMode)

			worktree := briefRulesIn(worktreeVerdict(t, cfg))
			staged, _ := stagedVerdict(t, cfg)
			if strings.Join(worktree, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("--validate must judge the folder as git staged it (%s %s): got %v, want %v", tc.indexMode, tc.indexPath, worktree, tc.want)
			}
			if got := briefRulesIn(staged); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("--staged must agree with --validate on the index git wrote: got %v, want %v", got, tc.want)
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
