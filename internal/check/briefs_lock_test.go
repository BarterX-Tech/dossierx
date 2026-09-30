// briefs_lock_test.go pins how a brief's lock lifecycle (NIT-205) rides
// through the check pipeline: the two integrity findings in ledger_findings,
// the two lifecycle lints in lint_findings, and the brief comment-digest rules —
// each judged from the SAME tree by both read-only modes, the working tree for
// --validate and the git index for --staged.
package check_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

const lockedBrief = "---\nsummary: How the widget flow reads end to end.\nstatus: locked\nrests_on:\n  - widget.contract.overview\n---\n# Widget flow\n\nText.\n"

// armBrief records the approval `brief lock` would record for every locked
// brief in cfg's working tree — the fixture's "a human approved this brief".
func armBrief(t *testing.T, cfg *config.Config) {
	t.Helper()
	store, err := lock.LoadStore(cfg.LockStorePath())
	if err != nil {
		t.Fatal(err)
	}
	claims := loadAllFixtureClaims(t, cfg)
	for _, b := range briefs.Load(cfg).Briefs {
		if b.Status != briefs.StatusLocked {
			continue
		}
		hashes, receipts, _ := briefs.Baselines(b, claims)
		lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{
			Path: b.Path, Hash: b.LockHash, At: "2026-09-30T00:00:00Z", Actor: "fixture", Reason: "fixture approval",
			Approved:  lock.BriefApproved{Summary: b.Summary, RestsOn: b.RestsOn, Markdown: b.Body},
			Baselines: hashes, Receipts: receipts,
		})
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
}

// lifecycleIn keeps the lifecycle findings of a verdict — the four brief lock
// rules and any comment rule on a brief path — as "kind|rule|path".
func lifecycleIn(verdict []string) []string {
	var out []string
	for _, line := range verdict {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 3 {
			continue
		}
		switch parts[1] {
		case briefs.RuleContentDrift, briefs.RuleUnrecorded, briefs.RuleDependencyDrift, briefs.RuleRestsOnMissing, briefs.RuleRestsOnUnknown:
		case lock.RuleCommentLedgerDrift, lock.RuleCommentDigestUnrecorded:
			if !strings.HasPrefix(parts[2], "briefs/") {
				continue
			}
		default:
			continue
		}
		out = append(out, parts[0]+"|"+parts[1]+"|"+parts[2])
	}
	return out
}

// TestBriefLockFindingsFollowTheTreeEachModeJudges is the --validate / --staged
// parity contract for every finding the lifecycle raises. Each row edits the
// committed, honest project into one state, UNSTAGED: --validate must report
// the row's findings and --staged, judging the still-clean index, nothing;
// staged, both must agree. A regression that read the lock store or the briefs
// from the working tree under --staged fails the middle assertion; one that
// raised the findings only in one mode fails the last.
func TestBriefLockFindingsFollowTheTreeEachModeJudges(t *testing.T) {
	flow := filepath.Join("briefs", "widget", "flow.md")
	claim := filepath.Join("claims", "overview.yaml")
	for _, tc := range []struct {
		name string
		edit func(t *testing.T, repo string)
		want []string
	}{
		{
			name: "a locked brief edited since approval",
			edit: func(t *testing.T, repo string) {
				writeFixtureFile(t, filepath.Join(repo, flow), strings.Replace(lockedBrief, "Text.", "Text, rewritten.", 1))
			},
			want: []string{"ledger|brief-content-drift|briefs/widget/flow.md"},
		},
		{
			name: "status: locked typed on a brief with no record",
			edit: func(t *testing.T, repo string) {
				writeFixtureFile(t, filepath.Join(repo, "briefs", "widget", "other.md"), "---\nsummary: Another brief.\nstatus: locked\n---\nText.\n")
			},
			want: []string{"ledger|brief-unrecorded|briefs/widget/other.md"},
		},
		{
			name: "a rests_on claim moved",
			edit: func(t *testing.T, repo string) {
				writeFixtureFile(t, filepath.Join(repo, claim), strings.Replace(draftClaim("widget.contract.overview"), "a draft claim.", "a draft claim, rewritten.", 1))
			},
			want: []string{"lint|brief-dependency-drift|briefs/widget/flow.md"},
		},
		{
			name: "a rests_on claim is gone",
			edit: func(t *testing.T, repo string) {
				writeFixtureFile(t, filepath.Join(repo, claim), draftClaim("widget.contract.renamed"))
			},
			want: []string{"lint|brief-rests-on-missing|briefs/widget/flow.md"},
		},
		{
			name: "a brief's comments block hand-edited",
			edit: func(t *testing.T, repo string) {
				writeFixtureFile(t, filepath.Join(repo, flow), strings.Replace(lockedBrief, "---\n# Widget", "comments:\n  - id: c-1\n    status: resolved\n    author: human\n    created: 2026-09-30T00:00:00Z\n    body: forged\n    edited: false\n---\n# Widget", 1))
			},
			want: []string{"ledger|comment-digest-unrecorded|briefs/widget/flow.md"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "repo")
			cfg := writeProjectFiles(t, repo, baseConfig, map[string]string{
				"claims/overview.yaml":  draftClaim("widget.contract.overview"),
				"briefs/widget/flow.md": lockedBrief,
			})
			armBrief(t, cfg)
			gitRepo(t, repo)
			git(t, repo, "add", "-A")
			git(t, repo, "commit", "-qm", "fixture")
			if got := lifecycleIn(worktreeVerdict(t, cfg)); len(got) != 0 {
				t.Fatalf("precondition: the approved brief must be silent under --validate, got %v", got)
			}

			tc.edit(t, repo)
			want := strings.Join(tc.want, ",")
			if got := strings.Join(lifecycleIn(worktreeVerdict(t, cfg)), ","); got != want {
				t.Fatalf("--validate: got %v, want %v", got, want)
			}
			if got, _ := stagedVerdict(t, cfg); len(lifecycleIn(got)) != 0 {
				t.Fatalf("--staged must judge the index, which is still clean; got %v", lifecycleIn(got))
			}
			git(t, repo, "add", "-A")
			if got, _ := stagedVerdict(t, cfg); strings.Join(lifecycleIn(got), ",") != want {
				t.Fatalf("once staged, --staged must agree with --validate: got %v, want %v", lifecycleIn(got), tc.want)
			}
		})
	}
}

// TestAStagedBriefLockTravelsWithItsRecord is the hook's reason to read the
// store from the index: a brief locked in the working tree whose status line is
// staged WITHOUT the lock store is brief-unrecorded under --staged, and staging
// the store with it clears the finding.
func TestAStagedBriefLockTravelsWithItsRecord(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	cfg := writeProjectFiles(t, repo, baseConfig, map[string]string{
		"claims/overview.yaml":  draftClaim("widget.contract.overview"),
		"briefs/widget/flow.md": strings.Replace(lockedBrief, "status: locked", "status: draft", 1),
	})
	gitRepo(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "fixture")

	writeFixtureFile(t, filepath.Join(repo, "briefs", "widget", "flow.md"), lockedBrief)
	armBrief(t, cfg)
	git(t, repo, "add", filepath.Join("briefs", "widget", "flow.md"))
	want := "ledger|brief-unrecorded|briefs/widget/flow.md"
	if got, _ := stagedVerdict(t, cfg); strings.Join(lifecycleIn(got), ",") != want {
		t.Fatalf("a lock staged without its record: got %v, want %s", lifecycleIn(got), want)
	}
	git(t, repo, "add", "-A")
	if got, _ := stagedVerdict(t, cfg); len(lifecycleIn(got)) != 0 {
		t.Fatalf("the lock and its record staged together must be clean, got %v", lifecycleIn(got))
	}
}

// TestRunRefusesABriefIntegrityFindingAfterTheProjections pins the gate: plain
// check with brief-content-drift fails at the ledger step (integrity), after the
// viewer is written, like a claim ledger finding; brief-dependency-drift alone,
// a warning, passes.
func TestRunRefusesABriefIntegrityFindingAfterTheProjections(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/overview.yaml":  draftClaim("widget.contract.overview"),
		"briefs/widget/flow.md": lockedBrief,
	})
	armBrief(t, cfg)

	writeFixtureFile(t, filepath.Join(cfg.Dir(), "claims", "overview.yaml"), strings.Replace(draftClaim("widget.contract.overview"), "a draft claim.", "a draft claim, rewritten.", 1))
	res, err := check.Run(loadAllFixtureClaims(t, cfg), cfg)
	if err != nil || !res.OK {
		t.Fatalf("a dependency-drift warning must not fail check: %v %+v", err, res.LintFindings)
	}

	writeFixtureFile(t, filepath.Join(cfg.Dir(), "briefs", "widget", "flow.md"), strings.Replace(lockedBrief, "Text.", "Text, rewritten.", 1))
	res, err = check.Run(loadAllFixtureClaims(t, cfg), cfg)
	if err == nil || !strings.Contains(err.Error(), "ledger") || res.RenderPath == "" {
		t.Fatalf("brief-content-drift must fail check at the ledger step, after the viewer is written: err=%v render=%q", err, res.RenderPath)
	}
}
