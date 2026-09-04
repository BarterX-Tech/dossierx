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

// monorepoFixture writes the ORDINARY MONOREPO LAYOUT: the config in a
// subdirectory, the claims beside it rather than under it —
//
//	<root>/docs/project.config.yaml   with claims_dir: ../claims
//	<root>/claims/locked.yaml
//
// — commits the lot, and returns the loaded config. The stores live beside the
// config (docs/), because that is where every path helper resolves them.
//
// Nothing in FORMAT.md or README requires claims_dir to sit UNDER the config's
// own directory, config.validate() does not reject "..", and check, claim lock,
// claim new and serve all work on this layout. So --staged has to as well; the
// tests below are what say so.
func monorepoFixture(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	claims := filepath.Join(root, "claims")
	for _, d := range []string{docs, claims} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	cfgPath := filepath.Join(docs, "project.config.yaml")
	body := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: ../claims\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claims, "locked.yaml"), []byte(lockedClaim("widget.contract.locked")), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	loaded, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	armLedger(t, cfg, loaded)

	gitRepo(t, root)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "fixture")
	return cfg
}

// THE MONOREPO HOLE. claims_dir: ../claims resolved to a path that is not under
// the config's directory, and the pathspec was computed relative to THAT
// directory, so it came out as "../claims" — which the code read as "outside
// the repository, nothing to evaluate" and reported as ErrNoIndex: the
// deliberate exit-0 escape hatch. But the claims are not outside the work tree
// at all; git resolves them perfectly well from the repository root. The result
// was a pre-commit hook and a CI job that printed nothing, exited 0, and
// committed an out-of-band edit to a locked claim, while "check --validate" on
// the very same tree reported lock-content-drift.
//
// Nothing warned the operator, because nothing else in the product cares where
// claims_dir points: check, claim lock, claim new and serve all work on this
// layout. The gate was the only thing that quietly did not.
func TestStaged_ClaimsDirOutsideTheConfigDirectoryIsStillJudged(t *testing.T) {
	cfg := monorepoFixture(t)
	root := filepath.Dir(cfg.Dir())
	claimFile := filepath.Join(cfg.ClaimsDir, "locked.yaml")

	// The clean tree passes, and — the half that proves the gate is ARMED
	// rather than merely silent — it says it judged the one claim.
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged on a clean monorepo layout: %v", err)
	}
	if len(sp.Claims) != 1 {
		t.Fatalf("claims_dir: ../claims must still assemble the registry, got %d claim(s)", len(sp.Claims))
	}
	if res := check.StatusStaged(sp, cfg); len(res.LedgerFindings) != 0 {
		t.Fatalf("expected a clean gate on the fixture, got %v", rulesOf(res.LedgerFindings))
	}

	// Now the out-of-band edit to a locked claim, staged for commit.
	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read fixture claim: %v", err)
	}
	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if tampered == string(original) {
		t.Fatalf("fixture precondition: the tamper substitution did not apply")
	}
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper claim: %v", err)
	}
	git(t, root, "add", "claims/locked.yaml")

	sp, err = check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged (after add): %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	if !hasRule(res.LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("a staged tamper of a locked claim under claims_dir: ../claims must be refused: got %v", rulesOf(res.LedgerFindings))
	}
}

// CI runs one entry point and the pre-commit hook runs another, so a
// DISAGREEMENT between them is itself a hole: whichever one is laxer is the one
// an edit will travel through. On a tree where the index and the working copy
// are the same bytes there is nothing for them to disagree about, so the two
// must return the same rules — here on the layout that used to disarm --staged
// entirely.
func TestStaged_AgreesWithTheWorktreeGateOnATamperedTree(t *testing.T) {
	cfg := monorepoFixture(t)
	root := filepath.Dir(cfg.Dir())
	claimFile := filepath.Join(cfg.ClaimsDir, "locked.yaml")

	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read fixture claim: %v", err)
	}
	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper claim: %v", err)
	}
	// Staged AND on disk: index and worktree now hold identical bytes, so the
	// two gates are being asked exactly the same question.
	git(t, root, "add", "claims/locked.yaml")

	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("reload claims: %v", err)
	}
	worktree := check.Status(claims, cfg)

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	staged := check.StatusStaged(sp, cfg)

	if got, want := strings.Join(rulesOf(staged.LedgerFindings), ","), strings.Join(rulesOf(worktree.LedgerFindings), ","); got != want {
		t.Fatalf("check --staged and check --validate disagree on an identical tampered tree:\n--staged:   %v\n--validate: %v", got, want)
	}
	if len(worktree.LedgerFindings) == 0 {
		t.Fatalf("fixture precondition: the tampered tree must produce findings for the comparison to mean anything")
	}
}

// The fallback to the WORKTREE config exists for one case: the very first
// commit of a project, where project.config.yaml is not in the index because
// nothing is. It was reached whenever the config was merely not TRACKED — and
// an untracked config is a worktree file, which means it is attacker-writable
// without staging anything. Point claims_dir at an empty decoy directory, stage
// a tampered locked claim, and the gate audited the decoy, found nothing, and
// passed the commit: exactly the bypass reading the config from the index was
// introduced to close, reopened through the back door.
//
// The fix is not to guess a config. It is to refuse — loudly, and with an error
// that is NOT ErrNoIndex, because ErrNoIndex exits 0.
//
// The decoy here holds a TRACKED, PRISTINE copy of the claim, which is the
// shape that made the bypass silent rather than merely wrong: an empty decoy
// leaves the ledger's record pointing at a claim the gate can no longer see, and
// lock-ledger-abandoned fires for the wrong reason. A decoy that satisfies every
// record produced "check --staged: OK", exit 0, over an index carrying the
// tampered claim.
func TestStaged_UntrackedConfigWillNotJudgeTrackedContent(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	root := cfg.Dir()
	claimFile := filepath.Join(cfg.ClaimsDir, "locked.yaml")
	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read fixture claim: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "decoy"), 0o755); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "decoy", "locked.yaml"), original, 0o644); err != nil {
		t.Fatalf("write decoy copy: %v", err)
	}

	gitRepo(t, root)
	// Everything EXCEPT project.config.yaml. The claims, the archive copy and
	// the ledger are tracked; the config is not.
	git(t, root, "add", "claims", "decoy", "build/ledger/lock-store.json")
	git(t, root, "commit", "-qm", "fixture")

	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if tampered == string(original) {
		t.Fatalf("fixture precondition: the tamper substitution did not apply")
	}
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper claim: %v", err)
	}
	git(t, root, "add", "claims/locked.yaml")

	// CONTROL: this tree is a refusal. "check" and "check --validate" — the
	// worktree gate, which is what CI runs — report lock-content-drift on it
	// with the project's real config. If --staged answers anything other than a
	// refusal below, the two entry points disagree about an identical tree, and
	// the laxer one is the one the pre-commit hook runs.
	tamperedClaims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("reload claims: %v", err)
	}
	if !hasRule(check.Status(tamperedClaims, cfg).LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("control precondition: the worktree gate must already refuse this tree")
	}

	// The decoy: an untracked config pointing claims_dir at the archive copy,
	// which satisfies every ledger record.
	decoy := strings.Replace(baseConfig, "claims_dir: claims", "claims_dir: decoy", 1)
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(decoy), 0o644); err != nil {
		t.Fatalf("write decoy config: %v", err)
	}
	redirected, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("load decoy config: %v", err)
	}

	_, err = check.Staged(redirected)
	if err == nil {
		t.Fatalf("an untracked config must not be used to judge tracked content: the gate audited %q and passed", redirected.ClaimsDir)
	}
	if errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("this must not be the exit-0 escape hatch: got ErrNoIndex (%v)", err)
	}
	if !errors.Is(err, check.ErrUntrackedConfig) {
		t.Fatalf("expected ErrUntrackedConfig, got %v", err)
	}
}

// The first-commit case the fallback was written for still works: an untracked
// config over an index that holds no claims and no ledger is a project that has
// staged nothing yet, and there is nothing there to be tricked about.
func TestStaged_UntrackedConfigIsFineOnAnEmptyIndex(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	root := cfg.Dir()
	gitRepo(t, root)
	// A repository with a commit but no dossierx content in it at all: the
	// claims and the config are both still untracked.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-qm", "fixture")

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("an untracked config over an empty index is the first-commit case: %v", err)
	}
	if sp.ConfigFromIndex {
		t.Fatalf("the config is not tracked; it cannot have come from the index")
	}
	if len(sp.Claims) != 0 {
		t.Fatalf("nothing is staged, so nothing is judged; got %d claim(s)", len(sp.Claims))
	}
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

	// Stage the ledger as well and the same commit passes. The comment digest
	// store goes with it, because an approval now records the approved claim's
	// comment digest in the same act that records the approval (see
	// lock.RecordApproval and comment-digest-missing): "the claim and its
	// approval must travel together" includes the coverage the approval
	// established, or the very next commit could drop the entry that makes an
	// edited-away review reportable.
	git(t, cfg.Dir(), "add", config.LockStoreDisplayPath, config.CommentDigestDisplayPath)
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

// FromIndex is documented as "the files where index and worktree disagree", and
// under core.autocrlf=true — the Windows default, and the configuration the
// windows-latest CI leg runs in — it was saturated on a perfectly clean tree:
// git stores LF in the index and checks out CRLF, so a byte comparison called
// every claim different. Reproduced on macOS by setting the config and forcing a
// fresh checkout: `git status --porcelain` empty, and check --staged reporting
// "2 claim(s) from the git index (2 differ from the working tree)".
//
// The verdict was never wrong (YAML parsing normalises line breaks, so hashes
// and lint agree either way) — the REPORT was, and a field that is
// unconditionally saturated on one platform carries no signal at all.
func TestStaged_FromIndexIgnoresGitsOwnLineEndingConversion(t *testing.T) {
	cfg := stagedFixture(t)

	// Turn on the conversion and force git to rewrite the worktree through it.
	git(t, cfg.Dir(), "config", "core.autocrlf", "true")
	if err := os.Remove(filepath.Join(cfg.ClaimsDir, "locked.yaml")); err != nil {
		t.Fatalf("remove claim for a fresh checkout: %v", err)
	}
	git(t, cfg.Dir(), "checkout", "--", "claims/locked.yaml")

	raw, err := os.ReadFile(filepath.Join(cfg.ClaimsDir, "locked.yaml"))
	if err != nil {
		t.Fatalf("read checked-out claim: %v", err)
	}
	if !strings.Contains(string(raw), "\r\n") {
		t.Skip("this git did not apply core.autocrlf on checkout; the case under test cannot arise here")
	}
	if porcelain(t, cfg.Dir()) != "" {
		t.Fatalf("fixture precondition: the tree must be clean, got %q", porcelain(t, cfg.Dir()))
	}

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if len(sp.FromIndex) != 0 {
		t.Fatalf("a clean tree under core.autocrlf must report no differing files, got %v", sp.FromIndex)
	}
	// The verdict is unchanged: the same clean project still passes.
	if res := check.StatusStaged(sp, cfg); len(res.LedgerFindings) != 0 {
		t.Fatalf("expected a clean gate, got %v", rulesOf(res.LedgerFindings))
	}
}
