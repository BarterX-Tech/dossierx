// staged_test.go covers "check --staged"'s two load-bearing promises: that it
// judges the INDEX and not the working tree, and that it writes nothing at all.
package check_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// gitRepo turns dir into a git repository with a deterministic identity. Every
// git test skips rather than fails when git is absent: the engine's own
// contract is that a missing git produces a warning and success, so a machine
// without git must not be a machine that cannot build this project.
func gitRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; --staged degrades to a warning by design, so there is nothing to test here")
	}
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "fixture@example.invalid")
	git(t, dir, "config", "user.name", "fixture")
}

// git runs one git command in dir and fails the test if it does not succeed.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// A deterministic, isolated environment: no ambient GIT_* variables and no
	// user config, so the fixture behaves the same on a developer's machine as
	// on the CI matrix.
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE=2026-07-26T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-07-26T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// porcelain is the exact byte string the --staged read-only test compares.
func porcelain(t *testing.T, dir string) string {
	t.Helper()
	return git(t, dir, "status", "--porcelain")
}

// stagedFixture writes a project with one locked claim whose approval IS on the
// ledger, commits everything, and returns the config.
func stagedFixture(t *testing.T) *config.Config {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	_ = claims
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")
	return cfg
}

// THE test the release's read-only promise rests on. "check --staged" runs
// DURING a commit; if evaluating dirtied the tree, the hook would change the
// thing it was asked to judge, mid-commit, behind the author's back.
func TestStaged_WritesNothing(t *testing.T) {
	cfg := stagedFixture(t)

	before := porcelain(t, cfg.Dir())
	beforeTree := treeSnapshot(t, cfg.Dir())

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	if len(res.LedgerFindings) != 0 {
		t.Fatalf("expected a clean gate on the fixture, got %v", rulesOf(res.LedgerFindings))
	}

	if after := porcelain(t, cfg.Dir()); after != before {
		t.Fatalf("git status --porcelain must be byte-identical across check --staged:\nbefore: %q\nafter:  %q", before, after)
	}
	if after := treeSnapshot(t, cfg.Dir()); after != beforeTree {
		t.Fatalf("check --staged wrote to the project tree:\nbefore:\n%s\nafter:\n%s", beforeTree, after)
	}
}

// The verdict follows the INDEX. Tampering with the worktree copy of a claim
// whose staged blob is compliant must NOT fail — and staging that same tamper
// must. A hook that got this backwards would pass the commit that carries the
// tampered content and refuse the one that does not.
func TestStaged_VerdictFollowsTheIndexNotTheWorktree(t *testing.T) {
	cfg := stagedFixture(t)
	claimFile := filepath.Join(cfg.ClaimsDir, "locked.yaml")

	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read fixture claim: %v", err)
	}
	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if tampered == string(original) {
		t.Fatalf("fixture precondition: the tamper substitution did not apply")
	}
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper worktree: %v", err)
	}

	// Worktree tampered, index clean -> the commit is fine.
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if len(sp.FromIndex) != 1 {
		t.Fatalf("expected the tampered file to be read from the index, got FromIndex=%v", sp.FromIndex)
	}
	if got := check.StatusStaged(sp, cfg); len(got.LedgerFindings) != 0 {
		t.Fatalf("a tampered WORKTREE with a clean index must pass: got %v", rulesOf(got.LedgerFindings))
	}
	// And it really did read the index: the body it linted is the original one.
	if len(sp.Claims) != 1 || !strings.Contains(sp.Claims[0].Body, "a locked claim.") {
		t.Fatalf("expected the index's body, got %q", sp.Claims[0].Body)
	}

	// Stage the tamper -> the commit is refused.
	git(t, cfg.Dir(), "add", "claims/locked.yaml")
	sp, err = check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged (after add): %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	if !hasRule(res.LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("staging the tampered blob must be refused: got %v", rulesOf(res.LedgerFindings))
	}
}

// The LEDGER comes from the index too, and that is what makes "the claim and
// its approval must be committed together" enforceable: a commit carrying a
// newly-locked claim without its approval record is refused, even though the
// worktree ledger has the record sitting right there.
func TestStaged_LedgerIsReadFromTheIndex(t *testing.T) {
	cfg := stagedFixture(t)

	// A second claim, locked and approved in the WORKTREE only.
	second := filepath.Join(cfg.ClaimsDir, "second.yaml")
	if err := os.WriteFile(second, []byte(lockedClaim("widget.contract.second")), 0o644); err != nil {
		t.Fatalf("write second claim: %v", err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("reload claims: %v", err)
	}
	armLedger(t, cfg, claims)

	// Stage ONLY the claim. The ledger update stays unstaged.
	git(t, cfg.Dir(), "add", "claims/second.yaml")

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	if !hasRule(res.LedgerFindings, lock.RuleLockLedgerMissing) {
		t.Fatalf("a locked claim staged without its approval record must be refused: got %v", rulesOf(res.LedgerFindings))
	}

	// Stage the ledger as well and the same commit passes.
	git(t, cfg.Dir(), "add", ".dossierx-lock-store.json")
	sp, err = check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged (after add): %v", err)
	}
	if got := check.StatusStaged(sp, cfg); len(got.LedgerFindings) != 0 {
		t.Fatalf("claim + approval staged together must pass: got %v", rulesOf(got.LedgerFindings))
	}
}

// The index's file list is the authority on which claims exist. An UNTRACKED
// claim is not part of the commit and must not be judged; a claim staged for
// DELETION is leaving the commit and must not be judged either, even though
// both are sitting in the working tree.
func TestStaged_UntrackedAndStagedDeletionsAreNotInTheRegistry(t *testing.T) {
	cfg := stagedFixture(t)

	// An untracked claim, deliberately one that would fail lint if judged
	// (it references a claim that does not exist).
	untracked := "id: widget.contract.untracked\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  never added to the index.\n" +
		"rests_on:\n  - widget.contract.does-not-exist\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "untracked.yaml"), []byte(untracked), 0o644); err != nil {
		t.Fatalf("write untracked claim: %v", err)
	}

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	for _, c := range sp.Claims {
		if c.ID == "widget.contract.untracked" {
			t.Fatalf("an untracked claim is not part of the commit and must not be judged")
		}
	}
	if got := check.StatusStaged(sp, cfg); len(got.LintErrors) != 0 {
		t.Fatalf("the untracked claim's dangling reference must not reach the lint: got %v", got.LintErrors)
	}

	// Now stage a deletion of the only tracked claim. It is still on disk (git
	// rm --cached leaves the file), so a worktree-driven check would still see
	// it; the index says it is gone.
	git(t, cfg.Dir(), "rm", "-q", "--cached", "claims/locked.yaml")
	sp, err = check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged (after rm --cached): %v", err)
	}
	for _, c := range sp.Claims {
		if c.ID == "widget.contract.locked" {
			t.Fatalf("a claim staged for deletion must not be in the registry, got %v", c.ID)
		}
	}
}

// A claim modified in the index but not yet in the worktree… is the ordinary
// case after "git add": worktree and index agree, so nothing is read through
// git at all. This pins the optimization's correctness — FromIndex empty means
// "every tracked claim's worktree copy already IS the index content", not
// "we forgot to look".
func TestStaged_CleanTreeReadsNothingThroughGit(t *testing.T) {
	cfg := stagedFixture(t)
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if len(sp.FromIndex) != 0 {
		t.Fatalf("a clean tree needs no index reads, got %v", sp.FromIndex)
	}
	if len(sp.Claims) != 1 {
		t.Fatalf("expected the registry to be complete regardless, got %d claim(s)", len(sp.Claims))
	}
}

// Outside a work tree there is no index to evaluate, and that is ErrNoIndex —
// which the CLI answers with a warning and exit 0. Failing instead would break
// "run check --staged in CI" for any project built from a tarball, and would
// teach hook authors to swallow exit codes.
func TestStaged_OutsideAWorkTreeIsErrNoIndex(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	_, err := check.Staged(cfg)
	if !errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("expected ErrNoIndex outside a work tree, got %v", err)
	}
}

// The decode duplicated in staged.go must agree with internal/loader's, on
// exactly the inputs that could tell them apart: strict unknown-field
// rejection, the one-document-per-file rule, and an ordinary claim. A rule
// added to one and not the other would let --staged accept a file the writing
// check rejects, which is the worst possible way for these two to disagree.
func TestStagedDecodeMatchesLoader(t *testing.T) {
	cases := map[string]string{
		"ordinary": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"unknown field": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: hi\nnot_a_real_field: 1\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"two documents": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: hi\ngoverned_by:\n  type: none\n  reason: fixture\n" +
			"---\nid: widget.contract.two\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: hi\n",
		"malformed yaml": "id: [unterminated\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			claimsDir := filepath.Join(dir, "claims")
			if err := os.MkdirAll(claimsDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			path := filepath.Join(claimsDir, "c.yaml")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}

			loaded, loadErr := loader.LoadClaims(claimsDir)
			staged, stagedErr := check.DecodeClaimForTest(path, []byte(body))

			if (loadErr != nil) != (stagedErr != nil) {
				t.Fatalf("verdict disagreement: loader err=%v, staged err=%v", loadErr, stagedErr)
			}
			if loadErr != nil {
				if loadErr.Error() != stagedErr.Error() {
					t.Fatalf("error text disagreement:\nloader: %v\nstaged: %v", loadErr, stagedErr)
				}
				return
			}
			if len(loaded) != 1 {
				t.Fatalf("fixture precondition: expected 1 claim, got %d", len(loaded))
			}
			if !claimsEqual(loaded[0], staged) {
				t.Fatalf("decode disagreement:\nloader: %#v\nstaged: %#v", loaded[0], staged)
			}
		})
	}
}

// claimsEqual compares the fields that survive a decode. model.Claim contains
// only comparable fields plus slices/maps, so this goes through the engine's
// own two content hashes rather than reflect.DeepEqual: between them
// LockedClaimHash and ContentHash cover every persisted field except status,
// review_pending and comments, which are compared directly.
func claimsEqual(a, b model.Claim) bool {
	if lock.LockedClaimHash(a) != lock.LockedClaimHash(b) {
		return false
	}
	if lock.ContentHash(a) != lock.ContentHash(b) {
		return false
	}
	if a.Status != b.Status || a.ReviewPending != b.ReviewPending || len(a.Comments) != len(b.Comments) {
		return false
	}
	return a.SourcePath == b.SourcePath
}

// TestStaged_AssumeUnchangedCannotSubstituteTheWorktree.
//
// The gate used to ask "git diff" which paths differed from the index, fetch
// THOSE from the index, and read the rest off disk as a cheaper equivalent. It
// is not an equivalent. "git diff" consults git's stat cache and honours the
// per-path skip bits, so
//
//	git update-index --assume-unchanged claims/locked.yaml
//
// made git report a modified file as clean — and the gate then read the clean
// WORKTREE copy while the tampered blob sat in the index waiting to be
// committed. The refusal vanished and the commit landed. The same holds for
// --skip-worktree and for any racily-clean stat entry.
//
// The bit is set by the ATTACKER, in the repository being audited, which is what
// makes it disqualifying rather than merely unlucky: a gate whose evidence
// source is chosen by a mutable, attacker-writable flag is not a gate.
func TestStaged_AssumeUnchangedCannotSubstituteTheWorktree(t *testing.T) {
	cfg := stagedFixture(t)
	claimFile := filepath.Join(cfg.ClaimsDir, "locked.yaml")

	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read fixture claim: %v", err)
	}
	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if tampered == string(original) {
		t.Fatalf("fixture precondition: the tamper substitution did not apply")
	}

	// Stage the tamper, then restore the worktree copy from HEAD. The index now
	// holds bytes nobody approved; the file on disk looks innocent.
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper worktree: %v", err)
	}
	git(t, cfg.Dir(), "add", "claims/locked.yaml")
	if err := os.WriteFile(claimFile, original, 0o644); err != nil {
		t.Fatalf("restore worktree: %v", err)
	}

	// CONTROL: without the bit, the gate already refuses. If this half ever
	// stops holding, the test below proves nothing.
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged (control): %v", err)
	}
	if !hasRule(check.StatusStaged(sp, cfg).LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("control precondition: the staged tamper must already be refused")
	}

	// Now the attack.
	git(t, cfg.Dir(), "update-index", "--assume-unchanged", "claims/locked.yaml")
	t.Cleanup(func() { git(t, cfg.Dir(), "update-index", "--no-assume-unchanged", "claims/locked.yaml") })

	sp, err = check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged (assume-unchanged): %v", err)
	}
	if !strings.Contains(sp.Claims[0].Body, "quietly rewritten") {
		t.Fatalf("the gate read the WORKTREE copy: assume-unchanged substituted the innocent file, body=%q", sp.Claims[0].Body)
	}
	if !hasRule(check.StatusStaged(sp, cfg).LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("assume-unchanged hid the staged tamper from the gate")
	}
	// FromIndex is derived by comparing bytes, not by asking git, so it stays
	// honest under the bit too.
	if len(sp.FromIndex) != 1 {
		t.Fatalf("FromIndex must still report the divergence under assume-unchanged, got %v", sp.FromIndex)
	}
}

// TestStaged_ConfigComesFromTheIndex.
//
// project.config.yaml names claims_dir, the module list, the doctrine facet and
// the hub gating switch — every input that decides WHICH files the gate looks at
// and what it demands of them. Read from the WORKTREE, one unstaged line was a
// complete bypass: stage a tampered locked claim, then point claims_dir at an
// empty directory in the working tree only. The gate audited nothing, found
// nothing, and let the commit through — while the commit itself still carried
// the real claims_dir, because that edit was never staged.
func TestStaged_ConfigComesFromTheIndex(t *testing.T) {
	cfg := stagedFixture(t)
	claimFile := filepath.Join(cfg.ClaimsDir, "locked.yaml")

	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read fixture claim: %v", err)
	}
	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper worktree: %v", err)
	}
	git(t, cfg.Dir(), "add", "claims/locked.yaml")

	// The unstaged redirect. The decoy directory must exist, or the redirect
	// would fail for an uninteresting reason rather than being followed.
	if err := os.MkdirAll(filepath.Join(cfg.Dir(), "decoy"), 0o755); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}
	cfgPath := filepath.Join(cfg.Dir(), "project.config.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	redirected := strings.Replace(string(raw), "claims_dir: claims", "claims_dir: decoy", 1)
	if redirected == string(raw) {
		t.Fatalf("fixture precondition: the claims_dir substitution did not apply")
	}
	if err := os.WriteFile(cfgPath, []byte(redirected), 0o644); err != nil {
		t.Fatalf("redirect claims_dir: %v", err)
	}

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if !sp.ConfigFromIndex {
		t.Fatalf("the config must have come from the index")
	}
	if got := filepath.Base(sp.Config.ClaimsDir); got != "claims" {
		t.Fatalf("the index still says claims_dir: claims; the gate resolved %q", got)
	}
	if len(sp.Claims) != 1 {
		t.Fatalf("the unstaged redirect emptied the registry: %d claim(s)", len(sp.Claims))
	}
	if !hasRule(check.StatusStaged(sp, cfg).LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("an unstaged claims_dir edit disarmed the gate")
	}
}
