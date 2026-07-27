// history_test.go covers the SCOPE GUARD: "check --staged" comparing the commit
// under judgement against its parent, so that deleting the lock ledger or
// repointing claims_dir is visible as a CHANGE rather than as an absence.
//
// The headline test is TestStaged_RefusesTheTwoCommitScopeCollapse, which is the
// verified reproduction from the review written down. Everything else in this
// file exists to prove the guard does not become a trap: a first commit, a
// pre-adoption parent, a genuine directory move, a widening move, a shallow
// checkout, a detached HEAD, a merge, a rebase/cherry-pick replay, and a commit
// that removes the project outright all have to pass.
package check_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// scopeFixture is a committed, fully-armed project with one LOCKED claim whose
// approval is on the ledger, plus a SECOND tracked directory that holds no
// claims at all.
//
// That second directory is the whole point. The reproduction repoints claims_dir
// at a directory that is already tracked and already innocent, so the tamper
// adds no new file to the diff and the registry it produces is EMPTY — which is
// what makes every existing rule silent, since each of them starts from a claim
// or from a store that is no longer there.
func scopeFixture(t *testing.T) *config.Config {
	t.Helper()
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
		"archive/NOTES.md":   "an ordinary tracked directory with no claims in it\n",
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")
	return cfg
}

// configWithClaimsDir is baseConfig with claims_dir set to dir. It is built
// rather than substituted so a fixture can point claims_dir anywhere, including
// at a nested directory the substitution would have mangled.
func configWithClaimsDir(dir string) string {
	return "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: " + dir + "\n"
}

// repoint rewrites claims_dir in the working tree's project.config.yaml and
// returns the config as it now reads.
func repoint(t *testing.T, cfg *config.Config, dir string) *config.Config {
	t.Helper()
	path := filepath.Join(cfg.Dir(), config.FileName)
	if err := os.WriteFile(path, []byte(configWithClaimsDir(dir)), 0o644); err != nil {
		t.Fatalf("repoint claims_dir: %v", err)
	}
	reloaded, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("reload repointed config: %v", err)
	}
	return reloaded
}

// stagedRules is the rule set "check --staged" reports for cfg, and it fails the
// test rather than returning an error, because every caller below treats a
// Staged error as a broken fixture.
func stagedRules(t *testing.T, cfg *config.Config) ([]string, check.Result) {
	t.Helper()
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	return rulesOf(res.LedgerFindings), res
}

// THE REPRODUCTION, exactly as the review recorded it.
//
// In a clean, fully-locked, hook-installed project, TWO commits defeated
// everything:
//
//	commit 1  claims_dir: claims -> archive   AND   git rm the lock ledger
//	commit 2  rewrite the locked claim freely
//
// After commit 1 the registry is EMPTY, so lock-ledger-missing has no claim to
// name, lock-ledger-absent's "and that cost us records" trigger counts zero, and
// the reverse sweep that would have called the standing record abandoned is
// skipped because the store file is gone. Measured on the shipped binary before
// this change: `check`, `check --validate`, `check --staged` and a fresh clone
// all printed zero findings and exited 0, and the pre-commit hook accepted both
// commits without a word.
//
// Every individual rule was behaving correctly. SCOPE is what failed, and scope
// is defined entirely by data inside the commit being judged — which is why the
// fix is a comparison against the parent and cannot be another rule.
func TestStaged_RefusesTheTwoCommitScopeCollapse(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	// Control: the fixture passes, and it passes having actually judged the
	// claim rather than having found nothing to judge.
	rules, res := stagedRules(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("fixture precondition: expected a clean gate, got %v", rules)
	}
	if len(res.LintErrors) != 0 {
		t.Fatalf("fixture precondition: expected a clean lint, got %v", res.LintErrors)
	}

	// COMMIT 1, staged but not yet made: repoint claims_dir at the innocent
	// tracked directory, and delete the ledger.
	collapsed := repoint(t, cfg, "archive")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")

	rules, _ = stagedRules(t, collapsed)
	if !contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("repointing claims_dir away from tracked, locked claims must be refused: got %v", rules)
	}
	if !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("deleting the lock ledger must be refused: got %v", rules)
	}

	// And the refusal has to name what a human must do. A gate that refuses
	// without a recovery is a gate that teaches --no-verify.
	sp, err := check.Staged(collapsed)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	joined := strings.Join(messagesOf(check.StatusStaged(sp, collapsed).LedgerFindings), "\n")
	for _, want := range []string{"claims/locked.yaml", "git checkout", "git mv"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the refusal must name the recovery (%q missing) — got:\n%s", want, joined)
		}
	}
}

// The comment digest store gets the same treatment as the lock ledger: it is the
// review history's fingerprint, and deleting it is how a hand-edited comment
// thread stops being compared against anything.
func TestStaged_RefusesDeletingTheCommentDigestStore(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	if _, err := os.Stat(digest.StorePath(cfg)); err != nil {
		t.Skipf("fixture has no comment digest store to delete (%v)", err)
	}
	git(t, root, "rm", "-q", digest.StoreFileName)

	rules, _ := stagedRules(t, cfg)
	if !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("deleting %s must be refused: got %v", digest.StoreFileName, rules)
	}
}

// THE FIRST COMMIT IN A REPOSITORY has no parent, and every file in it is new.
// Refusing it — "your ledger was not here before" — would make the guard
// unusable for the one flow every project starts with.
func TestStaged_FirstCommitInARepositoryIsNotRefused(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	// Staged, never committed: HEAD is unborn.

	rules, _ := stagedRules(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("a legitimate initial commit must pass: got %v", rules)
	}
}

// A PROJECT WHOSE PREVIOUS COMMIT LEGITIMATELY HAD NO LEDGER — the pre-adoption
// state every existing v0.2.x project is in. The parent carried claims and no
// store, and the commit under judgement adds one. "It was not there before" is
// true and is not a finding; the rule reports a store that DISAPPEARED, never
// one that appeared or one that was never there.
func TestStaged_ParentWithNoLedgerIsNotRefused(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	root := cfg.Dir()
	gitRepo(t, root)
	git(t, root, "add", "claims", config.FileName)
	git(t, root, "commit", "-qm", "pre-adoption: claims, no stores")

	// Now the adoption commit: the stores appear for the first time.
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	armDigests(t, cfg, claims)
	git(t, root, "add", "-A")

	rules, _ := stagedRules(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("a commit that ADDS the stores to a pre-adoption project must pass: got %v", rules)
	}
}

// THE SANCTIONED CLAIMS_DIR MOVE. There has to be one, or the guard is a trap.
//
// It is deliberately not a flag, an env var or a new command: a move that TAKES
// ITS CLAIMS WITH IT strands nothing, so the rule has nothing to fire on. This is
// the ordinary "git mv, edit claims_dir, commit both together" flow, and the
// test is what makes "documented" mean "true".
func TestStaged_SanctionedClaimsDirMoveIsAccepted(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	git(t, root, "mv", "claims", filepath.Join("docs", "claims"))
	moved := repoint(t, cfg, "docs/claims")
	git(t, root, "add", "-A")

	rules, res := stagedRules(t, moved)
	if len(rules) != 0 {
		t.Fatalf("a move that takes its claims with it must be accepted: got %v", rules)
	}
	// And it has to still be judging the claim afterwards — an "accepted" that
	// silently audits nothing would be the very failure this guard exists for.
	sp, err := check.Staged(moved)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if len(sp.Claims) != 1 {
		t.Fatalf("after the move the gate must still see the claim, got %d", len(sp.Claims))
	}
	if len(res.LintErrors) != 0 {
		t.Fatalf("unexpected lint errors after a sanctioned move: %v", res.LintErrors)
	}
}

// WIDENING the scope strands nothing either: every claim the parent judged is
// still inside the new claims_dir. Refusing it would be a false positive with no
// recovery except "put it back".
func TestStaged_WideningClaimsDirIsAccepted(t *testing.T) {
	cfg, _ := project(t, configWithClaimsDir("claims/nested"), map[string]string{
		"claims/nested/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	root := cfg.Dir()
	gitRepo(t, root)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "fixture")

	widened := repoint(t, cfg, "claims")
	git(t, root, "add", "-A")

	rules, _ := stagedRules(t, widened)
	if contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("widening claims_dir strands nothing and must not be refused: got %v", rules)
	}
}

// A COMMIT THAT REMOVES THE PROJECT ENTIRELY — config, claims and stores — is
// not a scope collapse, it is a deletion, and accusing someone of deleting their
// ledger while they delete their project would be a refusal with no fix.
//
// The dangerous middle (stores gone, config gone, claims still there) never
// reaches this guard: stagedConfig refuses it as ErrUntrackedConfig.
func TestStaged_RemovingTheProjectEntirelyIsNotRefused(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	git(t, root, "rm", "-r", "-q", "claims", config.FileName, ".dossierx-lock-store.json")
	if _, err := os.Stat(digest.StorePath(cfg)); err == nil {
		git(t, root, "rm", "-q", digest.StoreFileName)
	}

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("removing the project must not be an error: %v", err)
	}
	if rules := rulesOf(check.StatusStaged(sp, cfg).LedgerFindings); len(rules) != 0 {
		t.Fatalf("a commit that removes the project outright must pass: got %v", rules)
	}
}

// NOTHING STAGED: the CI shape. The index is identical to HEAD, so the commit
// under judgement is HEAD itself and its parent is HEAD's own parent. Comparing
// the index against HEAD there would be vacuous — it would report "no change"
// over a checkout of an already-collapsed history, which is the same silence in
// a new place.
func TestStaged_CleanCheckoutJudgesHeadAgainstItsParent(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	collapsed := repoint(t, cfg, "archive")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "chore: relocate claims")

	// Nothing staged now; this is exactly what CI checks out.
	if dirty := git(t, root, "status", "--porcelain"); strings.TrimSpace(dirty) != "" {
		t.Fatalf("fixture precondition: expected a clean tree, got:\n%s", dirty)
	}

	rules, _ := stagedRules(t, collapsed)
	if !contains(rules, check.RuleClaimsScopeNarrowed) || !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("a clean checkout of the collapsing commit must still be refused: got %v", rules)
	}
}

// A MERGE COMMIT is judged against its FIRST parent as well as the branch it
// merged, which is what gives CI its coverage of the whole reproduction rather
// than of its last commit.
//
// GitHub's pull_request event checks out the MERGE of the branch into its base,
// so HEAD's first parent is the base branch. Comparing against it spans every
// commit in the pull request at once — so the two-commit collapse, which is
// invisible when commit 2 is compared against commit 1, is reported when the
// merge is compared against main.
func TestStaged_MergeCommitJudgesTheWholeBranchAgainstItsBase(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	git(t, root, "checkout", "-q", "-b", "tamper")
	// Commit 1 of the branch: the scope collapse.
	collapsed := repoint(t, cfg, "archive")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "chore: relocate claims")
	// Commit 2: rewrite the locked claim, now that nothing audits it.
	claimFile := filepath.Join(root, "claims", "locked.yaml")
	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if tampered == string(original) {
		t.Fatalf("fixture precondition: the tamper substitution did not apply")
	}
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper claim: %v", err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "docs: tweak")

	// Judging the TIP against its own parent sees nothing: both commits agree
	// about the collapsed scope, because the collapse happened before them.
	// That is not a bug, it is why the merge comparison below has to exist.
	tipRules, _ := stagedRules(t, collapsed)
	if contains(tipRules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("precondition: commit-2-vs-commit-1 cannot see a collapse that landed in commit 1, got %v", tipRules)
	}

	// The merge, as CI checks it out.
	git(t, root, "checkout", "-q", "main")
	git(t, root, "merge", "-q", "--no-ff", "-m", "merge tamper", "tamper")
	merged, err := config.LoadConfig(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("load merged config: %v", err)
	}

	rules, _ := stagedRules(t, merged)
	if !contains(rules, check.RuleClaimsScopeNarrowed) || !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("a merge commit must be judged against its base, so the whole branch's collapse is reported: got %v", rules)
	}
}

// DETACHED HEAD is the state a CI checkout of a specific sha is in, and the
// state a bisect or a "git checkout <sha>" leaves behind. Nothing in this guard
// reads a branch name, and this is what says so.
func TestStaged_DetachedHeadStillCompares(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	collapsed := repoint(t, cfg, "archive")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "chore: relocate claims")
	head := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
	git(t, root, "checkout", "-q", "--detach", head)

	rules, _ := stagedRules(t, collapsed)
	if !contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("a detached HEAD must be compared exactly like an attached one: got %v", rules)
	}
}

// A REPLAYED COMMIT — cherry-pick, and by the same mechanism rebase — lands on a
// different parent, and the guard's answer follows THAT parent rather than the
// one the commit was originally written against. Here the collapse is replayed
// onto a branch that still has its ledger, and it is refused there too.
//
// This is the property that matters for rebase and cherry-pick: git does not run
// pre-commit for either, so the refusal has to survive into CI's clean-checkout
// comparison, which is where this test evaluates it.
func TestStaged_CherryPickedCollapseIsRefusedOnItsNewParent(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	git(t, root, "checkout", "-q", "-b", "tamper")
	collapsed := repoint(t, cfg, "archive")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "chore: relocate claims")
	sha := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))

	// A fresh, untouched base, and the same change replayed onto it.
	git(t, root, "checkout", "-q", "main")
	git(t, root, "checkout", "-q", "-b", "replayed")
	git(t, root, "cherry-pick", sha)

	replayed, err := config.LoadConfig(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("load replayed config: %v", err)
	}
	rules, _ := stagedRules(t, replayed)
	if !contains(rules, check.RuleClaimsScopeNarrowed) || !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("a cherry-picked collapse must be refused against its NEW parent: got %v", rules)
	}
	_ = collapsed
}

// A SHALLOW CHECKOUT cannot answer the question, and the honest report of that
// is neither a refusal (which would break every default actions/checkout, and a
// gate that refuses honest work gets deleted) nor silence.
//
// The failure mode this pins is specific and was verified by hand: a shallow
// clone GRAFTS its boundary, so "git rev-list --parents -n 1 HEAD" prints the
// sha alone — byte-identical to a genuine root commit. Reading that as "this is
// the first commit, there is nothing to compare" would make depth-1 CI the one
// configuration where the whole comparison silently does not happen.
func TestStaged_ShallowCheckoutSaysSoInsteadOfPassingSilently(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	collapsed := repoint(t, cfg, "archive")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "chore: relocate claims")

	dst := filepath.Join(t.TempDir(), "shallow")
	git(t, root, "clone", "-q", "--depth", "1", "file://"+root, dst)
	shallow, err := config.LoadConfig(filepath.Join(dst, config.FileName))
	if err != nil {
		t.Fatalf("load shallow clone config: %v", err)
	}

	sp, err := check.Staged(shallow)
	if err != nil {
		t.Fatalf("Staged in a shallow clone: %v", err)
	}
	res := check.StatusStaged(sp, shallow)
	if rules := rulesOf(res.LedgerFindings); len(rules) != 0 {
		t.Fatalf("a shallow checkout must not manufacture a refusal it cannot evidence: got %v", rules)
	}
	if len(res.NextSteps) == 0 || !strings.Contains(res.NextSteps[0], "could NOT compare") {
		t.Fatalf("a shallow checkout must SAY the comparison did not happen; next_steps was %v", res.NextSteps)
	}
	if !strings.Contains(res.NextSteps[0], "fetch-depth") {
		t.Fatalf("the advisory must name the fix; got %q", res.NextSteps[0])
	}

	// Deepened, the same clone reports the refusal it could not reach before.
	git(t, dst, "fetch", "-q", "--deepen", "1")
	deepened, err := config.LoadConfig(filepath.Join(dst, config.FileName))
	if err != nil {
		t.Fatalf("reload deepened config: %v", err)
	}
	rules, _ := stagedRules(t, deepened)
	if !contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("once the history is there the comparison must happen: got %v", rules)
	}
	_ = collapsed
}

// A CONFLICTED MERGE is the one shape where pre-commit DOES fire mid-merge, and
// the commit it is about to make has two parents. Judging it against HEAD alone
// would let a collapse ride in on the merged branch; both parents are consulted.
func TestStaged_ConflictedMergeConsultsBothParents(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	// A branch that collapses the scope AND touches a file main will also touch,
	// so the merge conflicts and reaches the pre-commit path.
	git(t, root, "checkout", "-q", "-b", "tamper")
	collapsed := repoint(t, cfg, "archive")
	if err := os.WriteFile(filepath.Join(root, "archive", "NOTES.md"), []byte("branch\n"), 0o644); err != nil {
		t.Fatalf("write NOTES.md: %v", err)
	}
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "chore: relocate claims")

	git(t, root, "checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(root, "archive", "NOTES.md"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("write NOTES.md: %v", err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "notes")

	// The merge conflicts; resolve it and stage, exactly as a human would before
	// the commit that fires the hook.
	mergeConflict(t, root, "tamper")
	if err := os.WriteFile(filepath.Join(root, "archive", "NOTES.md"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("resolve NOTES.md: %v", err)
	}
	git(t, root, "add", "-A")

	rules, _ := stagedRules(t, collapsed)
	if !contains(rules, check.RuleClaimsScopeNarrowed) || !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("a conflicted merge must be judged against both parents: got %v", rules)
	}
}

// CI RUNS ONE ENTRY POINT AND THE HOOK RUNS ANOTHER, so a tree on which they
// disagree is a hole by construction: whichever is LAXER is the one an edit
// travels through.
//
// The scope guard cannot be symmetric, and it is worth being exact about why
// rather than pretending. `check` and `check --validate` are defined against ONE
// tree — no git, no index, no history — and the collapse this guard reports is
// not a property of any single tree: from inside one, a repointed claims_dir and
// an honestly small project are the same picture. So the guard lives in the one
// entry point that is defined against git, and the invariant that keeps the
// three honest is DIRECTIONAL:
//
//	--staged's findings are a SUPERSET of the worktree gate's, never a subset.
//
// A stricter hook is not the hole the review found; a laxer CI is. The shipped
// workflow template (scripts/ci/dossierx-check.yml) therefore runs `check
// --staged` alongside `check`, so the entry point that HAS the guard is the one
// with the authority.
//
// This test pins the direction on the tampered tree the worktree gate does see,
// with the index and the working copy holding identical bytes so the two are
// being asked exactly the same question.
func TestStaged_IsNeverLaxerThanTheWorktreeGate(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	claimFile := filepath.Join(root, "claims", "locked.yaml")
	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	tampered := strings.Replace(string(original), "a locked claim.", "a locked claim, quietly rewritten.", 1)
	if tampered == string(original) {
		t.Fatalf("fixture precondition: the tamper substitution did not apply")
	}
	if err := os.WriteFile(claimFile, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper claim: %v", err)
	}
	git(t, root, "add", "-A")

	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	worktree := rulesOf(check.Status(claims, cfg).LedgerFindings)
	staged, _ := stagedRules(t, cfg)

	if len(worktree) == 0 {
		t.Fatalf("fixture precondition: the worktree gate must already refuse this tree")
	}
	for _, want := range worktree {
		if !contains(staged, want) {
			t.Fatalf("check --staged is LAXER than the worktree gate on an identical tree: it is missing %q\n--staged:   %v\n--validate: %v", want, staged, worktree)
		}
	}
}

// The other half of the same invariant, on the tree only --staged can judge: the
// collapse IS refused there, and the worktree gate's silence on it is the hole
// this whole file exists to name — which is why the CI template runs --staged.
func TestStaged_RefusesTheCollapseTheWorktreeGateCannotSee(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	collapsed := repoint(t, cfg, "archive")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")

	claims, err := loader.LoadClaims(collapsed.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	if worktree := check.Status(claims, collapsed); len(worktree.LedgerFindings) != 0 {
		t.Fatalf("precondition: the one-tree gate cannot see a scope collapse; if it now can, this file's argument needs revisiting. Got %v", rulesOf(worktree.LedgerFindings))
	}

	staged, _ := stagedRules(t, collapsed)
	if len(staged) == 0 {
		t.Fatalf("--staged must refuse the collapsed tree that the one-tree gate cannot see")
	}
}

// mergeConflict runs a merge that is EXPECTED to conflict, so it cannot use the
// git helper (which fails the test on a non-zero exit). It fails the test if the
// merge unexpectedly succeeds, because the fixture's whole point is to reach the
// mid-merge index a conflicted merge leaves behind.
func mergeConflict(t *testing.T, dir, branch string) {
	t.Helper()
	cmd := exec.Command("git", "merge", "--no-ff", "-m", "merge "+branch, branch)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE=2026-07-26T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-07-26T00:00:00Z",
	)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("fixture precondition: the merge was expected to conflict, but it succeeded:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err != nil {
		t.Fatalf("fixture precondition: no MERGE_HEAD after a conflicted merge: %v", err)
	}
}

// contains is a rule-name membership test, kept here rather than reusing
// hasRule so these tests read against the []string that stagedRules returns.
func contains(rules []string, want string) bool {
	for _, r := range rules {
		if r == want {
			return true
		}
	}
	return false
}

func messagesOf(findings []lock.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Message)
	}
	return out
}
