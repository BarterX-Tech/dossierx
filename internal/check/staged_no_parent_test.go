// staged_no_parent_test.go is the pin on what "check --staged" does now that the
// PARENT-COMMIT COMPARISON has been removed. There used to be a history.go, a
// history_test.go and a parent_scope_test.go beside this file; all three are
// gone, and staged.go's "REMOVED, DELIBERATELY" section carries the argument.
//
// This file is the executable half of that argument, in three parts:
//
//   - THE TWO REGRESSIONS THAT FORCED THE REMOVAL, reproduced end to end so
//     neither can come back unnoticed: a `git revert` of a commit that locked a
//     claim, and a brand-new project added in a monorepo commit that retires an
//     unrelated one. Both were refused by the comparison, and neither is anything
//     but ordinary git work.
//
//   - WHAT STILL HOLDS FROM ONE TREE. Either half of each shape below is still
//     refused on its own, because the pieces defend each other: repoint claims_dir
//     and the standing approvals have no claims left to cover
//     (lock-ledger-abandoned); delete the ledger and the locked claims have no
//     approvals (lock-ledger-absent); and so on for the other two.
//
//   - WHAT WAS GIVEN UP, written down as passing tests rather than left as
//     folklore. There are THREE such shapes. This file has said ONE in one round
//     and TWO in another, and both were under-counts; the three are SHAPE 1, the
//     collapsed scope (both halves of the scope collapse in one change); SHAPE 2,
//     the disowned claim (one claim's ledger record, locked_at stamp and
//     baselines deleted, its status flipped to draft and its body rewritten, in
//     one change); and SHAPE 3, the erased review (a DRAFT claim's `comments:`
//     block and its digest entry deleted in one change). All three are here so a
//     reader who finds them does not conclude the gate is broken and re-add a
//     comparison against history the committer writes — read staged.go first.
//     internal/lock/audit_boundary_test.go pins 2 and 3 again at the rules' own
//     level; these are the end-to-end `check --staged` half.
package check_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// singleTreeFixture is a committed, fully-armed project with one LOCKED claim
// whose approval is on the ledger, plus a SECOND tracked directory holding no
// claims at all.
//
// The empty second directory is inherited from the deleted scope tests and is
// still the sharpest shape available: repointing claims_dir at a directory that
// is already tracked and already innocent adds no new file to the diff and
// produces an EMPTY registry, which is the state that makes every per-claim rule
// silent. What is left standing over it — the reverse sweep — is exactly what
// these tests measure.
func singleTreeFixture(t *testing.T) *config.Config {
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

// claimsDirConfig is baseConfig with claims_dir set to dir. It is built rather
// than substituted so a fixture can point claims_dir anywhere, including at a
// nested directory a substitution would have mangled.
func claimsDirConfig(dir string) string {
	return "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: " + dir + "\n"
}

// repointClaimsDir rewrites claims_dir in the working tree's
// project.config.yaml and returns the config as it now reads.
func repointClaimsDir(t *testing.T, cfg *config.Config, dir string) *config.Config {
	t.Helper()
	path := filepath.Join(cfg.Dir(), config.FileName)
	if err := os.WriteFile(path, []byte(claimsDirConfig(dir)), 0o644); err != nil {
		t.Fatalf("repoint claims_dir: %v", err)
	}
	reloaded, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("reload repointed config: %v", err)
	}
	return reloaded
}

// stagedRulesFor is the rule set "check --staged" reports for cfg. It fails the
// test rather than returning an error, because every caller below treats a
// Staged error as a broken fixture.
func stagedRulesFor(t *testing.T, cfg *config.Config) ([]string, check.Result) {
	t.Helper()
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	res := check.StatusStaged(sp, cfg)
	return rulesOf(res.LedgerFindings), res
}

// hasName reports whether rules contains want.
func hasName(rules []string, want string) bool {
	for _, r := range rules {
		if r == want {
			return true
		}
	}
	return false
}

// joinedMessages is every finding's prose in one string, for the assertions that
// care about WHAT a refusal names rather than which rule it is.
func joinedMessages(findings []lock.Finding) string {
	var b strings.Builder
	for _, f := range findings {
		b.WriteString(f.Message)
		b.WriteString("\n")
	}
	return b.String()
}

// ---------------------------------------------------------------------
// REGRESSION 1 — git revert
// ---------------------------------------------------------------------

// A LEGITIMATE `git revert` OF A LOCK COMMIT MUST PASS, and this is the case
// that made the parent comparison indefensible rather than merely arguable.
//
// Reverting the commit that locked a claim removes that lock's records, and the
// resulting tree is byte-identical to one where somebody erased them: nothing in
// either tree records the intent, and "the parent had a record this commit does
// not" is true of both. The comparison therefore refused the revert — as
// integrity-store-removed when the lock commit had introduced the stores, and as
// lock-ledger-deleted when it had only added a record to stores that already
// existed (the second half of this test).
//
// AND IT REFUSED IT IN THE WORST PLACE. git does not run pre-commit hooks for
// revert, so the refusal never fired at the keyboard: the revert committed
// locally at rc=0 and only CI objected, telling the author to restore the thing
// they had just deliberately removed.
func TestStaged_RevertOfALockCommitIsAccepted(t *testing.T) {
	// The WHOLE-STORE shape: the project's first lock is what creates the
	// ledger, so reverting that commit takes the file away with it.
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	root := cfg.Dir()
	gitRepo(t, root)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "one draft claim, no ledger yet")

	lockThroughTheLedger(t, cfg, "widget.contract.one")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "lock the claim, and record the approval")

	if rules, res := stagedRulesFor(t, cfg); len(rules) != 0 {
		t.Fatalf("fixture precondition: the lock commit itself must pass; got %v\n%s", rules, joinedMessages(res.LedgerFindings))
	}

	git(t, root, "revert", "--no-edit", "HEAD")

	rules, res := stagedRulesFor(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("a git revert of a lock commit must pass; got %v\n%s", rules, joinedMessages(res.LedgerFindings))
	}
}

// lockThroughTheLedger takes a claim from draft to locked the way `dossierx
// claim lock` does — the file's status flips AND the approval is recorded —
// because a fixture that flips only one of the two is testing a tamper.
func lockThroughTheLedger(t *testing.T, cfg *config.Config, id string) {
	t.Helper()
	for _, c := range loadFixtureClaims(t, cfg) {
		if c.ID != id {
			continue
		}
		if err := os.WriteFile(c.SourcePath, []byte(lockedClaim(id)), 0o644); err != nil {
			t.Fatalf("write the locked claim: %v", err)
		}
	}
	claims := loadFixtureClaims(t, cfg)
	store, err := lock.LoadStore(filepath.Join(cfg.Dir(), ".dossierx-lock-store.json"))
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	for _, c := range claims {
		if c.ID == id {
			lock.RecordApproval(store, c, lock.Approval{Actor: "fixture", Reason: "the human approved it"})
		}
	}
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
}

// loadFixtureClaims is loader.LoadClaims with the test's error handling.
func loadFixtureClaims(t *testing.T, cfg *config.Config) []model.Claim {
	t.Helper()
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	return claims
}

// THE SAME REVERT, ONE COMMIT LATER, where the ledger already existed and the
// reverted commit only ADDED a record to it. The store survives the revert with
// its other records intact and just this claim's approval gone, which is the
// exact predicate the per-claim half of the comparison (lock.AuditAgainstParent,
// reached through the deleted parentLedgerContent) fired on as
// lock-ledger-deleted.
//
// It is a separate test because the two shapes were caught by two different
// pieces of the machinery, and removing only one of them would have left this
// one refusing reverts on every project past its first lock.
func TestStaged_RevertOfASecondLockCommitIsAccepted(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml": lockedClaim("widget.contract.one"),
		"claims/two.yaml": draftClaim("widget.contract.two"),
	})
	root := cfg.Dir()
	gitRepo(t, root)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "one locked claim, its approval on the ledger")

	// Lock the second claim through the ledger, exactly as `claim lock` does,
	// and commit it: the ledger now carries two standing records.
	lockThroughTheLedger(t, cfg, "widget.contract.two")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "lock the second claim")

	if rules, res := stagedRulesFor(t, cfg); len(rules) != 0 {
		t.Fatalf("fixture precondition: the lock commit itself must pass; got %v\n%s", rules, joinedMessages(res.LedgerFindings))
	}

	git(t, root, "revert", "--no-edit", "HEAD")

	rules, res := stagedRulesFor(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("a git revert that removes ONE approval record must pass; got %v\n%s", rules, joinedMessages(res.LedgerFindings))
	}
}

// ---------------------------------------------------------------------
// REGRESSION 2 — a new project in a monorepo
// ---------------------------------------------------------------------

// A PROJECT THAT IS NEW IN A COMMIT MUST NOT BE AUDITED AGAINST AN UNRELATED
// PROJECT'S LEDGER.
//
// The comparison resolved every path — claims_dir, both stores — beside the
// PARENT's own project.config.yaml, which it had to find first. For a project
// that does not exist in the parent at all there is nothing to find, so the
// lookup fell through to "the one project.config.yaml that VANISHED between the
// two commits" — and in a monorepo commit that retires projB while adding projC,
// that is projB's. projC was then refused for deleting projB's ledger, in a
// message naming projB's files.
//
// There is no version of that lookup that gets this right, because the tree does
// not record which project a new config is a continuation of, or whether it is a
// continuation at all. This test pins the only correct answer: judge projC on
// its own, from its own tree.
func TestStaged_NewProjectIsNotAuditedAgainstARetiredOnesLedger(t *testing.T) {
	root := t.TempDir()
	projB := filepath.Join(root, "projB")
	if err := os.MkdirAll(filepath.Join(projB, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir projB: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projB, config.FileName), []byte(baseConfig), 0o644); err != nil {
		t.Fatalf("write projB config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projB, "claims", "locked.yaml"), []byte(lockedClaim("widget.contract.retired")), 0o644); err != nil {
		t.Fatalf("write projB claim: %v", err)
	}
	cfgB, err := config.LoadConfig(filepath.Join(projB, config.FileName))
	if err != nil {
		t.Fatalf("load projB config: %v", err)
	}
	claimsB, err := loader.LoadClaims(cfgB.ClaimsDir)
	if err != nil {
		t.Fatalf("load projB claims: %v", err)
	}
	armLedger(t, cfgB, claimsB)
	armDigests(t, cfgB, claimsB)

	gitRepo(t, root)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "projB, fully locked")

	// ONE commit: projB is retired outright and projC — a brand-new project,
	// draft claims only, no ledger because it has never locked anything — is
	// added beside it.
	git(t, root, "rm", "-r", "-q", "projB")
	projC := filepath.Join(root, "projC")
	if err := os.MkdirAll(filepath.Join(projC, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir projC: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projC, config.FileName), []byte(baseConfig), 0o644); err != nil {
		t.Fatalf("write projC config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projC, "claims", "one.yaml"), []byte(draftClaim("widget.contract.one")), 0o644); err != nil {
		t.Fatalf("write projC claim: %v", err)
	}
	git(t, root, "add", "-A")

	cfgC, err := config.LoadConfig(filepath.Join(projC, config.FileName))
	if err != nil {
		t.Fatalf("load projC config: %v", err)
	}
	rules, res := stagedRulesFor(t, cfgC)
	if len(rules) != 0 {
		t.Fatalf("a brand-new project must be judged on its own; got %v\n%s", rules, joinedMessages(res.LedgerFindings))
	}
	// The stronger half of the assertion: even if some future rule does have
	// something to say here, it must never be about the other project.
	if joined := joinedMessages(res.LedgerFindings); strings.Contains(joined, "projB") || strings.Contains(joined, "retired") {
		t.Fatalf("a finding on projC named projB's files or claims:\n%s", joined)
	}
}

// ---------------------------------------------------------------------
// WHAT STILL HOLDS FROM ONE TREE
// ---------------------------------------------------------------------

// EITHER HALF OF THE SCOPE COLLAPSE, ALONE, IS STILL REFUSED — which is the
// measurement the removal was decided on, so it is measured here rather than
// asserted in a comment.
//
// The lock ledger and claims_dir defend each other: each one's records are the
// other's alibi, so removing one leaves the other testifying. Only removing both
// at once gets past both rules — that is SHAPE 1 of the three detections this
// package gave up (see the test after this one; shapes 2 and 3, the disowned
// claim and the erased review, are further down).
func TestStaged_EitherSabotageAloneIsStillRefused(t *testing.T) {
	t.Run("claims_dir repointed, ledger left in place", func(t *testing.T) {
		cfg := singleTreeFixture(t)
		collapsed := repointClaimsDir(t, cfg, "archive")
		git(t, cfg.Dir(), "add", "-A")

		rules, res := stagedRulesFor(t, collapsed)
		if !hasName(rules, lock.RuleLockLedgerAbandoned) {
			t.Fatalf("a repointed claims_dir leaves standing approvals covering claims that are no longer in scope, which is lock-ledger-abandoned; got %v\n%s",
				rules, joinedMessages(res.LedgerFindings))
		}
	})

	t.Run("ledger deleted, claims_dir left in place", func(t *testing.T) {
		cfg := singleTreeFixture(t)
		git(t, cfg.Dir(), "rm", "-q", ".dossierx-lock-store.json")
		git(t, cfg.Dir(), "add", "-A")

		rules, res := stagedRulesFor(t, cfg)
		if !hasName(rules, lock.RuleLockLedgerAbsent) {
			t.Fatalf("a deleted ledger leaves locked claims with no approval at all, which is lock-ledger-absent; got %v\n%s",
				rules, joinedMessages(res.LedgerFindings))
		}
	})
}

// SHAPE 1 — THE COLLAPSED SCOPE, the first of the THREE detections that left
// with the parent comparison, written down as a PASSING test on purpose.
//
// Repointing claims_dir AND removing the lock ledger in the SAME change empties
// the registry and removes the records at once, so neither rule above has an
// input: lock-ledger-abandoned reads records that are gone, lock-ledger-absent
// counts locked claims that are out of scope. From this one tree the result is
// indistinguishable from a brand-new project — zero claims, no ledger — which is
// precisely why there is no cheap single-tree rule to put here. A rule that
// refused this shape would refuse every project's first commit.
//
// IF YOU ARE READING THIS BECAUSE THE TEST FAILED, something re-introduced a
// detection for this shape. That is not automatically wrong — but read staged.go's
// "REMOVED, DELIBERATELY" section first, and make sure whatever you added is not
// another comparison against history the committer is free to rewrite, and does
// not refuse a `git revert` or a new project in a monorepo. Then delete this
// test and pin the new rule instead.
func TestStaged_Shape1CollapsedScopeIsUndetected(t *testing.T) {
	cfg := singleTreeFixture(t)
	collapsed := repointClaimsDir(t, cfg, "archive")
	git(t, cfg.Dir(), "rm", "-q", ".dossierx-lock-store.json")
	git(t, cfg.Dir(), "add", "-A")

	rules, _ := stagedRulesFor(t, collapsed)
	if len(rules) != 0 {
		t.Fatalf("this shape is a KNOWN, DELIBERATE gap and this test records it; something now refuses it: %v", rules)
	}
}

// EITHER HALF OF THE DISOWNED CLAIM, ALONE, IS STILL REFUSED — the same
// mutual-defence structure the ledger and claims_dir have, one claim wide.
//
// The claim's own `status:` and the ledger's record are each other's alibi:
// leave the status locked and a deleted record is lock-ledger-missing; leave the
// record standing and a claim flipped to draft is lock-ledger-orphan. It takes
// both, in one change, to leave neither rule an input — which is why shape 2 is
// a conjunction and not a soft spot, and why deleting this pair of assertions
// would let the gap test below pass by testing nothing.
func TestStaged_EitherHalfOfTheDisownedClaimIsStillRefused(t *testing.T) {
	t.Run("ledger record dropped, the claim left locked", func(t *testing.T) {
		cfg := singleTreeFixture(t)
		dropLedgerRecord(t, cfg, "widget.contract.locked")
		git(t, cfg.Dir(), "add", "-A")

		rules, res := stagedRulesFor(t, cfg)
		if !hasName(rules, lock.RuleLockLedgerMissing) {
			t.Fatalf("a locked claim whose record was deleted has no approval at all, which is lock-ledger-missing; got %v\n%s",
				rules, joinedMessages(res.LedgerFindings))
		}
	})

	t.Run("claim flipped to draft, its record left standing", func(t *testing.T) {
		cfg := singleTreeFixture(t)
		if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "locked.yaml"), []byte(draftClaim("widget.contract.locked")), 0o644); err != nil {
			t.Fatalf("disown the claim: %v", err)
		}
		git(t, cfg.Dir(), "add", "-A")

		rules, res := stagedRulesFor(t, cfg)
		if !hasName(rules, lock.RuleLockLedgerOrphan) {
			t.Fatalf("a draft claim under a standing approval is lock-ledger-orphan; got %v\n%s",
				rules, joinedMessages(res.LedgerFindings))
		}
	})
}

// dropLedgerRecord deletes EVERYTHING the engine wrote when it locked id: the
// ledger record, the locked_at stamp and the dependency baselines. All three,
// because the two beside the record are the evidence "this claim was locked by
// the engine" is derived from (lock.engineLocked) — a fixture that removed only
// the record would be testing a different, noisier tamper.
func dropLedgerRecord(t *testing.T, cfg *config.Config, id string) {
	t.Helper()
	store, err := lock.LoadStore(filepath.Join(cfg.Dir(), ".dossierx-lock-store.json"))
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	delete(store.Ledger, id)
	delete(store.LockedAt, id)
	delete(store.Hashes, id)
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
}

// SHAPE 2 — THE DISOWNED CLAIM, the second of the THREE detections that left
// with the parent comparison, and the one the round that measured "TWO" missed.
//
// THE SHAPE, in ONE change, on ONE claim:
//
//	delete ledger[id], locked_at[id] and hashes[id] from the lock store
//	flip `status: locked` to `status: draft` in the claim
//	rewrite the body
//
// It is CHEAPER than shape 1 in every dimension a reviewer scans: no claims_dir
// edit, no store deleted, no second directory — one claim's worth of diff and a
// few keys out of a JSON file. And it is invisible from this tree because every
// rule that could speak reads evidence the same change removed: lock-ledger-
// missing needs a LOCKED claim, lock-ledger-orphan needs a STANDING record,
// lock-ledger-deleted needs the engine's own marks that the claim was ever
// locked, and lock-ledger-abandoned needs a record whose claim has left. What is
// left looks exactly like an ordinary draft claim that nobody has ever locked —
// which is what most claims are — so there is no cheap single-tree rule to put
// here either. Re-locking it afterwards writes a record indistinguishable from a
// first approval.
//
// A VARIANT moves the claim file out of the registry's scope instead of flipping
// its status; it is the same erasure with the same evidence removed.
//
// IF YOU ARE READING THIS BECAUSE THE TEST FAILED, see the note on the shape 1
// test above: read staged.go's "REMOVED, DELIBERATELY" section before concluding
// that re-adding a parent comparison is the fix, then delete this test and pin
// the new rule instead.
func TestStaged_Shape2DisownedClaimIsUndetected(t *testing.T) {
	cfg := singleTreeFixture(t)

	dropLedgerRecord(t, cfg, "widget.contract.locked")
	// draftClaim is the same id at `status: draft` with a different body: the
	// status flip and the rewrite the record used to cover, in one write.
	if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "locked.yaml"), []byte(draftClaim("widget.contract.locked")), 0o644); err != nil {
		t.Fatalf("disown the claim: %v", err)
	}
	git(t, cfg.Dir(), "add", "-A")

	rules, res := stagedRulesFor(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("this shape is a KNOWN, DELIBERATE gap and this test records it; something now refuses it: %v\n%s",
			rules, joinedMessages(res.LedgerFindings))
	}
}

// EITHER HALF OF THE ERASED REVIEW, ALONE, IS STILL REFUSED — the same
// mutual-defence structure the lock ledger and claims_dir have, and the reason
// the gap below is a CONJUNCTION rather than a soft spot.
//
// It is also what keeps the gap test honest: without this, a fixture that
// quietly stopped arming the digest store would make that test pass by testing
// nothing at all.
func TestStaged_EitherHalfOfTheErasedReviewIsStillRefused(t *testing.T) {
	t.Run("comments block erased, digest entry left in place", func(t *testing.T) {
		cfg := erasedReviewFixture(t)
		if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "draft.yaml"), []byte(draftClaim("widget.contract.draft")), 0o644); err != nil {
			t.Fatalf("erase the comments block: %v", err)
		}
		git(t, cfg.Dir(), "add", "-A")

		rules, res := stagedRulesFor(t, cfg)
		if !hasName(rules, lock.RuleCommentLedgerDrift) {
			t.Fatalf("an erased block still has a recorded digest to disagree with, which is comment-ledger-drift; got %v\n%s",
				rules, joinedMessages(res.LedgerFindings))
		}
	})

	t.Run("digest entry dropped, comments block left in place", func(t *testing.T) {
		cfg := erasedReviewFixture(t)
		store, err := digest.LoadStore(digest.StorePath(cfg))
		if err != nil {
			t.Fatalf("load digest store: %v", err)
		}
		writeDigestStore(t, cfg, `{"version":1,"digests":{"widget.contract.locked":"`+store.Digests["widget.contract.locked"]+`"}}`)
		git(t, cfg.Dir(), "add", "-A")

		rules, res := stagedRulesFor(t, cfg)
		if !hasName(rules, lock.RuleCommentDigestUnrecorded) {
			t.Fatalf("threads with no entry beside them is comment-digest-unrecorded; got %v\n%s",
				rules, joinedMessages(res.LedgerFindings))
		}
	})
}

// erasedReviewFixture is a committed, ledger-covered project holding one LOCKED
// claim and one DRAFT claim that carries a human's OPEN thread.
func erasedReviewFixture(t *testing.T) *config.Config {
	t.Helper()
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
		"claims/draft.yaml":  commentedDraftClaim("widget.contract.draft"),
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "a draft carrying a human's open objection")

	if res := check.Status(claims, cfg); len(res.LedgerFindings) != 0 {
		t.Fatalf("fixture precondition: the honest project must be silent, got %v", rulesOf(res.LedgerFindings))
	}
	store, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}
	if _, ok := store.Digests["widget.contract.draft"]; !ok {
		t.Fatal("fixture precondition: the draft's thread must be recorded in the digest store")
	}
	return cfg
}

// SHAPE 3 — THE ERASED REVIEW, the third of the THREE detections that left with
// the parent comparison.
//
// This one is NOT the scope collapse and NOT the disowned claim above, and it
// was missed by the measurement the removal was first decided on ("exactly one
// detection"). It is written down here for the same reason those are: a gap
// nobody has measured is folklore, and folklore is what gets a comparison
// against rewritable history re-added.
//
// THE SHAPE, on a DRAFT claim in a ledger-covered project, in ONE change:
//
//	delete the `comments:` block from the claim YAML
//	delete that claim's key from the digest store's "digests" map
//
// Neither surviving rule has an input afterwards. comment-ledger-drift compares
// the block against a RECORDED digest, and the record is gone;
// comment-digest-unrecorded's predicate is THE THREADS THEMSELVES, and the
// threads are gone — erasing them takes the claim out of that rule's own
// evidence set (internal/lock/audit.go's RuleCommentDigestUnrecorded says so in
// as many words). The claim is left looking like one nobody has ever commented
// on, which is what a normal draft looks like.
//
// WHY IT MATTERS MORE THAN ITS SIZE SUGGESTS: an OPEN thread on a draft claim is
// precisely what BLOCKS `claim lock` (lock refuses with unresolved_comments). So
// the erasure buys the lock. Measured end to end against a built binary: a human's
// open objection is erased, `check` reports ok:true, the claim then locks cleanly
// with a fresh record, and the project stays clean permanently.
//
// WHAT STILL CATCHES IT: the same erasure on a LOCKED claim is refused with no
// history at all — as comment-digest-missing, which keys on a STANDING
// lock-ledger record that has no entry in the digest store. A locked claim has
// such a record; a DRAFT claim has none, so the rule is never asked, and that is
// exactly why the gap is specific to drafts — which is where review pressure
// lives. (Erasing the block alone, digest key intact, is comment-ledger-drift at
// either status.) It is NOT lock-content-drift: `comments` is one of the three
// fields lock.lockedClaimHashExcluded keeps out of the locked-claim hash, so no
// comments edit on any claim in any status can produce that rule.
//
// IF YOU ARE READING THIS BECAUSE THE TEST FAILED, see the note on the test
// above: read staged.go's "REMOVED, DELIBERATELY" section before concluding that
// re-adding a parent comparison is the fix, then delete this test and pin the new
// rule instead.
func TestStaged_Shape3ErasedReviewIsUndetected(t *testing.T) {
	cfg := erasedReviewFixture(t)
	before, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		t.Fatalf("load digest store: %v", err)
	}

	// THE ERASURE, both halves in one change.
	if err := os.WriteFile(filepath.Join(cfg.ClaimsDir, "draft.yaml"), []byte(draftClaim("widget.contract.draft")), 0o644); err != nil {
		t.Fatalf("erase the comments block: %v", err)
	}
	writeDigestStore(t, cfg, `{"version":1,"digests":{"widget.contract.locked":"`+before.Digests["widget.contract.locked"]+`"}}`)
	git(t, cfg.Dir(), "add", "-A")

	rules, res := stagedRulesFor(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("this shape is a KNOWN, DELIBERATE gap and this test records it; something now refuses it: %v\n%s",
			rules, joinedMessages(res.LedgerFindings))
	}
}

// commentedDraftClaim is a DRAFT claim carrying a human's OPEN thread — the
// state that blocks `claim lock`, and therefore the state worth erasing.
func commentedDraftClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"comments:\n" +
		"  - id: c-9f21ab\n    status: open\n    author: human\n" +
		"    created: \"2026-07-26T10:00:00Z\"\n    body: not sustainable as written; do not lock this.\n    edited: false\n"
}

// ---------------------------------------------------------------------
// THE DEGRADATION PATHS, simplified now that there is no parent to look up
// ---------------------------------------------------------------------

// AN UNBORN HEAD — the repository's very first commit, staged and never
// committed. It used to be a special case in the parent lookup ("there is no
// parent, so compare nothing"); now there is simply nothing to look up, and the
// commit is judged exactly like any other. It must pass, because refusing a
// legitimate initial commit is the one false positive that makes a gate get
// uninstalled.
func TestStaged_FirstCommitInARepositoryIsNotRefused(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	// Staged, never committed: HEAD is unborn.

	rules, res := stagedRulesFor(t, cfg)
	if len(rules) != 0 {
		t.Fatalf("a legitimate initial commit must pass: got %v\n%s", rules, joinedMessages(res.LedgerFindings))
	}
}

// A claims_dir OUTSIDE THE WORK TREE is ErrNoIndex — the exit-0 escape hatch —
// again, and unconditionally. The parent comparison used to run AHEAD of this
// exit so that a claims_dir repointed OUT of the repository was refused rather
// than waved through; with the comparison gone the escape hatch is the first
// answer once more, which is what keeps "run check --staged in CI" working for a
// checkout whose claims genuinely live somewhere else.
func TestStaged_ClaimsDirOutsideTheWorkTreeIsErrNoIndex(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "repo")
	outer := filepath.Join(root, "outside-claims")
	for _, d := range []string{inner, outer} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	cfgPath := filepath.Join(inner, config.FileName)
	if err := os.WriteFile(cfgPath, []byte(claimsDirConfig("../outside-claims")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outer, "draft.yaml"), []byte(draftClaim("widget.contract.one")), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	gitRepo(t, inner)
	git(t, inner, "add", "-A")
	git(t, inner, "commit", "-qm", "claims live outside the repository")

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, err := check.Staged(cfg); !errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("a claims_dir outside the work tree must be the exit-0 escape hatch, got %v", err)
	}
}

// A COMMIT THAT REMOVES THE PROJECT ENTIRELY — config, claims and stores — is a
// deletion, not a tamper, and must pass. The dangerous middle (stores gone,
// config gone, claims still tracked) never gets this far: stagedConfig refuses
// it as ErrUntrackedConfig, which is a single-tree rule and is unaffected by
// anything removed here.
func TestStaged_RemovingTheProjectEntirelyIsNotRefused(t *testing.T) {
	cfg := singleTreeFixture(t)
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

// THE SANCTIONED claims_dir MOVE — git mv the claims, edit claims_dir, commit
// both together — still passes, and the gate still SEES the claim afterwards.
//
// It was the parent comparison's escape valve, and it is kept because the second
// half of the assertion is the one that matters either way: an "accepted" that
// silently audits nothing is the failure mode this whole area exists to prevent,
// and that half is answerable from one tree.
func TestStaged_SanctionedClaimsDirMoveIsAccepted(t *testing.T) {
	cfg := singleTreeFixture(t)
	root := cfg.Dir()

	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	git(t, root, "mv", "claims", filepath.Join("docs", "claims"))
	moved := repointClaimsDir(t, cfg, "docs/claims")
	git(t, root, "add", "-A")

	rules, res := stagedRulesFor(t, moved)
	if len(rules) != 0 {
		t.Fatalf("a move that takes its claims with it must be accepted: got %v\n%s", rules, joinedMessages(res.LedgerFindings))
	}
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
