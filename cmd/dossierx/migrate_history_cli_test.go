// migrate_history_cli_test.go is the regression suite for the GIT half of
// "dossierx migrate --adopt" (migrate_history.go): the corroboration that stops
// the command re-signing a tampered locked claim in one commit.
//
// Every refusal here was a LIVE, VERIFIED break before the code under test
// existed — each ran to completion, exited 0, and was then accepted by
// "check --staged" and "check --validate" on the resulting commit. So the suite
// is organised around the two reproductions and, just as importantly, around the
// honest paths that must stay silent:
//
//	refuses  a covered project whose ledger was removed to re-arm adoption
//	refuses  a locked claim edited since the last commit
//	silent   an honest git-tracked pre-ledger project adopting once
//	allows   a project with no git at all, LOUDLY — never silently
//	allows   a claim locked but not yet committed, LOUDLY — never silently
//
// The last two are the important half. A gate that fires on correct state is the
// outage implicit grandfathering existed to prevent, and this one degrades
// rather than refuses in every state where git cannot answer.
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/digest"
)

// ---------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------

// gitPreLedgerProject is the honest v0.2.x upgrade state, IN GIT and committed:
// one claim locked through the real approval path, the store then rewound to the
// pre-ledger schema with its comment digest store removed, and the whole thing
// committed. That commit is what an upgrader's HEAD actually looks like, which is
// what makes it the right baseline for every assertion below.
func gitPreLedgerProject(t *testing.T) (cfgPath, root, claimPath, storeFile string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; the history corroboration degrades to a warning by design")
	}

	cfgPath, claimPath, storeFile = ledgerProject(t)
	root = filepath.Dir(cfgPath)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved in review"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	rewindStoreToPreLedger(t, storeFile)
	gitCommitProject(t, root, "the v0.2.x project, as its upgrader's HEAD holds it")
	return cfgPath, root, claimPath, storeFile
}

// gitCommitProject initialises a repository at root if there is not one already
// and commits everything in it.
func gitCommitProject(t *testing.T, root, message string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		stagedGit(t, root, "init", "-q", "-b", "main")
		stagedGit(t, root, "config", "user.email", "fixture@example.invalid")
		stagedGit(t, root, "config", "user.name", "fixture")
	}
	stagedGit(t, root, "add", "-A")
	stagedGit(t, root, "commit", "-qm", message)
}

// rewriteClaimBody edits a locked claim's body in place — the tamper, performed
// exactly the way an attacker would: a text editor, no dossierx command.
func rewriteClaimBody(t *testing.T, claimPath, replacement string) {
	t.Helper()
	raw, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	edited := strings.Replace(string(raw), "the approved body.", replacement, 1)
	if edited == string(raw) {
		t.Fatalf("fixture drift: the claim no longer contains the body this test rewrites:\n%s", raw)
	}
	if err := os.WriteFile(claimPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
}

// preconditionDetail returns the Detail of the named precondition, and fails the
// test if it is absent. A precondition that is MISSING and one that is
// present-and-passing are different answers, and only the second means the gate
// exists — the same distinction hasPrecondition draws.
func preconditionDetail(t *testing.T, dr cliout.DryRun, name string) string {
	t.Helper()
	for _, p := range dr.Preconditions {
		if p.Name == name {
			return p.Detail
		}
	}
	t.Fatalf("the preview does not evaluate %q at all: %+v", name, dr.Preconditions)
	return ""
}

// storeIsStillPreLedger asserts nothing was adopted: the store on disk still
// says it predates the ledger. It is the assertion that makes a refusal test
// mean something — an error message with a write behind it is not a refusal.
func storeIsStillPreLedger(t *testing.T, storeFile string) {
	t.Helper()
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	if _, has := doc["ledger"]; has {
		t.Fatalf("a refused migration wrote a ledger anyway:\n%s", raw)
	}
	if v, _ := doc["version"].(float64); v >= 2 {
		t.Fatalf("a refused migration stamped the ledger schema anyway:\n%s", raw)
	}
}

// anyContains reports whether any element of list contains substr.
func anyContains(list []string, substr string) bool {
	for _, s := range list {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// reproduction A: a covered project that removed its ledger to re-arm adoption
// ---------------------------------------------------------------------

// TestMigrateRefusesAProjectVersionControlSaysWasAlreadyCovered.
//
// The verified break, in ONE commit: on a fully ledger-covered project, delete
// the "ledger" key, put "version" back to 1, delete the comment digest store,
// rewrite a locked claim's body, and run "dossierx migrate --adopt". The
// directory left behind is byte-for-byte the shape of an honest v0.2.x project
// — lock.Store.LedgerDowngraded says so in its own doc comment — so every
// in-directory predicate agreed it was one and the rewritten body was adopted as
// the approved baseline. Exit 0, then "check --staged" and "check --validate"
// both exit 0 on the resulting commit.
//
// What tells the two apart is not in the directory: it is that the LAST COMMIT
// carried a ledger and a comment digest store. This is that comparison.
func TestMigrateRefusesAProjectVersionControlSaysWasAlreadyCovered(t *testing.T) {
	cfgPath, claimPath, storeFile := ledgerProject(t)
	root := filepath.Dir(cfgPath)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; the history corroboration degrades to a warning by design")
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved in review"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	// HEAD is a project that has plainly been through a ledger-aware build.
	gitCommitProject(t, root, "a fully ledger-covered project")

	// The whole attack, in the working tree that is about to become one commit.
	rewindStoreToPreLedger(t, storeFile)
	rewriteClaimBody(t, claimPath, "the body an attacker substituted.")

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if !dr.Blocked {
		t.Fatalf("a project version control says was already covered must preview as blocked: %+v", dr)
	}
	if dr.From != migrateModeHistoryCovered {
		t.Fatalf("preview mode=%q, want %q", dr.From, migrateModeHistoryCovered)
	}
	if !hasPrecondition(dr, "history_confirms_pre_ledger", false) {
		t.Fatalf("the git-corroborated gate must be the one that failed: %+v", dr.Preconditions)
	}
	// And the two in-directory gates still PASS, which is the point: they are
	// looking at a directory that was forged to satisfy them.
	if !hasPrecondition(dr, "pre_ledger_claim_not_contradicted", true) {
		t.Fatalf("the in-directory gate is expected to pass here — the directory was forged to satisfy it: %+v", dr.Preconditions)
	}
	// THE CLAIM WAS ALSO REWRITTEN, and this project's mode can only name one
	// state. planMigration's switch stops at the first refusing state, so the
	// second gate would report "[ok] every locked claim holds the same approved
	// content" over a claim that demonstrably does not if it were keyed on the
	// mode instead of on the evidence — the same over-claim this round exists to
	// remove, one line further down the same preview.
	if !hasPrecondition(dr, "locked_claims_match_version_control", false) {
		t.Fatalf("the claim was rewritten too, and no gate may certify content it knows changed: %+v", dr.Preconditions)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err == nil || env.OK {
		t.Fatalf("the write path must refuse what the preview blocked, got %+v", env)
	}
	if env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	var data migrateData
	envData(t, env, &data)
	if data.Mode != migrateModeHistoryCovered {
		t.Fatalf("the refusal must say WHICH evidence produced it, got %q", data.Mode)
	}
	if !strings.Contains(env.Error.Hint, "do NOT re-lock") {
		t.Fatalf("the hint must warn off the destructive recovery: %+v", env.Error)
	}
	storeIsStillPreLedger(t, storeFile)
}

// TestMigrateReadsTheLedgerKeyOutOfHeadAndNotTheVersionField.
//
// The evidence has to be the one an attacker cannot edit away in the tree they
// are being judged against. Here HEAD's comment digest store is absent (so the
// stronger piece of evidence is gone) and HEAD's lock store carries an EMPTIED
// ledger map — "ledger": {} — which is the smaller diff and the shape that once
// slipped past lock.Store.LedgerDowngraded. The key's presence alone is
// conclusive: a store that predates the ledger cannot carry it at all.
func TestMigrateReadsTheLedgerKeyOutOfHeadAndNotTheVersionField(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; the history corroboration degrades to a warning by design")
	}
	cfgPath, _, storeFile := ledgerProject(t)
	root := filepath.Dir(cfgPath)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved in review"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	// Commit a store that is at version 1 with an EMPTY ledger map, and no
	// comment digest store at all: every in-directory predicate reads this as
	// pre-ledger, then and now.
	writeStoreDoc(t, storeFile, func(doc map[string]any) {
		doc["ledger"] = map[string]any{}
		doc["version"] = 1
	})
	if err := os.Remove(filepath.Join(root, digest.StoreFileName)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove comment digest store: %v", err)
	}
	gitCommitProject(t, root, "a store carrying an emptied ledger key")

	// The working tree removes the key entirely, which is what makes the
	// directory look honest.
	writeStoreDoc(t, storeFile, func(doc map[string]any) { delete(doc, "ledger") })

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if dr.From != migrateModeHistoryCovered {
		t.Fatalf("an emptied ledger key in HEAD is still a ledger key; mode=%q, want %q", dr.From, migrateModeHistoryCovered)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt"); err == nil {
		t.Fatalf("the write path must refuse what the preview blocked")
	}
	storeIsStillPreLedger(t, storeFile)
}

// writeStoreDoc rewrites the lock store through a mutator over its decoded JSON.
func writeStoreDoc(t *testing.T, storeFile string, mutate func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	mutate(doc)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(storeFile, out, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
}

// ---------------------------------------------------------------------
// reproduction B: a locked claim edited since the last commit
// ---------------------------------------------------------------------

// TestMigrateRefusesLockedClaimsEditedSinceTheLastCommit.
//
// The simpler break, and the one with no forgery in it at all: an honest
// pre-ledger project, a locked claim rewritten by hand, "dossierx migrate
// --adopt". A pre-ledger store holds no record of a locked claim's approved
// content, so adoption records whatever is on disk — and the announcement even
// said so ("no record of the original exists to compare against"). In a git
// project that sentence is false: the original is the claim's last committed
// blob.
//
// Refusing rather than warning is the right severity precisely BECAUSE the store
// holds no original: after the adoption there is nothing left to compare
// against, so a warning would be a notice about a state that can no longer be
// investigated.
func TestMigrateRefusesLockedClaimsEditedSinceTheLastCommit(t *testing.T) {
	cfgPath, _, claimPath, storeFile := gitPreLedgerProject(t)
	rewriteClaimBody(t, claimPath, "the body an attacker substituted.")

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if !dr.Blocked {
		t.Fatalf("an edited locked claim must preview as blocked: %+v", dr)
	}
	if dr.From != migrateModeClaimsModified {
		t.Fatalf("preview mode=%q, want %q", dr.From, migrateModeClaimsModified)
	}
	detail := preconditionDetail(t, dr, "locked_claims_match_version_control")
	if !strings.Contains(detail, "widget.contract.main") {
		t.Fatalf("the preview must NAME the claim that changed, got %q", detail)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err == nil || env.OK {
		t.Fatalf("the write path must refuse what the preview blocked, got %+v", env)
	}
	if env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	var data migrateData
	envData(t, env, &data)
	if data.Mode != migrateModeClaimsModified {
		t.Fatalf("the refusal must say WHICH state it hit, got %q", data.Mode)
	}
	if !strings.Contains(env.Error.Message, "widget.contract.main") {
		t.Fatalf("the refusal must name the claim a reader has to go and look at: %+v", env.Error)
	}
	// Both recoveries B5 names are offered: restore it, or take the edit
	// through the approval path.
	if !strings.Contains(env.Error.Hint, "git checkout") {
		t.Fatalf("the hint must name the restore: %+v", env.Error)
	}
	if !strings.Contains(env.Error.Hint, "dossierx claim unlock") || !strings.Contains(env.Error.Hint, "dossierx claim lock") {
		t.Fatalf("the hint must name the approval path for an edit the author actually wants: %+v", env.Error)
	}
	storeIsStillPreLedger(t, storeFile)
}

// TestMigrateIgnoresEditsToDraftClaims.
//
// The other half of the same rule, and the one that keeps the gate usable: DRAFT
// claims are free to edit on purpose, so an upgrader who is midway through
// editing a draft when they run the migration must not be refused. Only claims
// that are locked on BOTH sides are compared.
func TestMigrateIgnoresEditsToDraftClaims(t *testing.T) {
	cfgPath, root, _, _ := gitPreLedgerProject(t)

	draft := filepath.Join(root, "claims", "draft.yaml")
	src := "id: widget.contract.draft\nfacet: contract\nmodule: widget\nstatus: draft\nbuild_role: schema\n" +
		"body: |\n  the first draft.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(draft, []byte(src), 0o644); err != nil {
		t.Fatalf("write draft claim: %v", err)
	}
	gitCommitProject(t, root, "a draft claim alongside the locked one")

	if err := os.WriteFile(draft, []byte(strings.Replace(src, "the first draft.", "the second draft, rewritten freely.", 1)), 0o644); err != nil {
		t.Fatalf("rewrite draft claim: %v", err)
	}

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if dr.Blocked {
		t.Fatalf("editing a DRAFT claim is correct work and must not block the migration: %+v", dr)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt"); err != nil {
		t.Fatalf("the migration must run: %v", err)
	}
}

// ---------------------------------------------------------------------
// the honest paths, which must stay silent
// ---------------------------------------------------------------------

// TestMigrateAdoptsAnHonestGitProjectInSilence.
//
// The upgrade path this whole release turns on: a v0.2.x project, committed,
// running "dossierx migrate --adopt" once. It must pass, and it must pass
// QUIETLY — no degradation notice, no advisory about content that could not be
// checked — because git was in a position to answer and did.
func TestMigrateAdoptsAnHonestGitProjectInSilence(t *testing.T) {
	cfgPath, _, _, _ := gitPreLedgerProject(t)

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if dr.Blocked {
		t.Fatalf("an honest committed pre-ledger project must preview as adoptable: %+v", dr)
	}
	if !hasPrecondition(dr, "history_confirms_pre_ledger", true) {
		t.Fatalf("version control agrees, and the passing gate is the evidence: %+v", dr.Preconditions)
	}
	if !hasPrecondition(dr, "locked_claims_match_version_control", true) {
		t.Fatalf("the content matches, and the passing gate is the evidence: %+v", dr.Preconditions)
	}
	if anyContains(dr.SideEffects, "NOT CHECKED") || anyContains(dr.SideEffects, "could not be compared") {
		t.Fatalf("nothing was left unchecked here, so nothing may say so: %+v", dr.SideEffects)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err != nil || !env.OK {
		t.Fatalf("the migration must run on an honest project: %v %+v", err, env)
	}
	if anyContains(env.Warnings, "NOT CHECKED") || anyContains(env.Warnings, "could not be compared") {
		t.Fatalf("a corroborated adoption must not warn about corroboration: %+v", env.Warnings)
	}
	// The grandfathering warnings themselves are unchanged and still present:
	// this round adds notices, it does not replace the ones that were there.
	if !anyContains(env.Warnings, "GRANDFATHERED") {
		t.Fatalf("the adoption warning must survive: %+v", env.Warnings)
	}
}

// TestMigrateDegradesHonestlyOutsideAWorkTree.
//
// A project that is not in git at all still has to be able to migrate — refusing
// there would break every tarball checkout and every project that has not
// committed yet, which is the outage implicit grandfathering existed to prevent.
// So it ADOPTS, and says out loud that nothing corroborated what it adopted.
//
// The two git preconditions are reported as PASSING here, deliberately: the
// write path does not refuse for want of git, so a preview that blocked would
// disagree with its own run — the one defect class this file's neighbours have
// re-shipped three rounds running.
func TestMigrateDegradesHonestlyOutsideAWorkTree(t *testing.T) {
	cfgPath, _ := preLedgerProject(t)

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if dr.Blocked {
		t.Fatalf("a project with no git must still migrate: %+v", dr)
	}
	for _, name := range []string{"history_confirms_pre_ledger", "locked_claims_match_version_control"} {
		if !hasPrecondition(dr, name, true) {
			t.Fatalf("%s must be reported as a pass when git could not be consulted: %+v", name, dr.Preconditions)
		}
		if detail := preconditionDetail(t, dr, name); !strings.Contains(detail, "NOT CHECKED") {
			t.Fatalf("%s passed without being checked, and must say so rather than claim corroboration: %q", name, detail)
		}
	}
	if !anyContains(dr.SideEffects, "NOT CHECKED AGAINST VERSION CONTROL") {
		t.Fatalf("the preview must name the uncorroborated adoption as a side effect: %+v", dr.SideEffects)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err != nil || !env.OK {
		t.Fatalf("the migration must still run: %v %+v", err, env)
	}
	if !anyContains(env.Warnings, "NOT CHECKED AGAINST VERSION CONTROL") {
		t.Fatalf("the run must warn that nothing corroborated what it adopted: %+v", env.Warnings)
	}
}

// TestMigrateWarnsRatherThanRefusesAClaimLockedSinceTheLastCommit.
//
// A claim that reads status: locked here and did not at HEAD is exactly what a
// hand flip looks like — and also exactly what a "dossierx claim lock" run under
// the OLD v0.2.x binary, not yet committed when the upgrade happened, looks like.
// Refusing would wedge that upgrader completely: the commit that would clear the
// refusal is itself refused by the pre-commit hook until the migration runs.
//
// So it is put in front of the human instead, in the same list they are being
// asked to approve. That is a judgement call about severity, and it is written
// down here so a later round changes it deliberately rather than by accident.
func TestMigrateWarnsRatherThanRefusesAClaimLockedSinceTheLastCommit(t *testing.T) {
	cfgPath, root, _, _ := gitPreLedgerProject(t)

	second := filepath.Join(root, "claims", "second.yaml")
	src := "id: widget.contract.second\nfacet: contract\nmodule: widget\nstatus: draft\nbuild_role: behavior\n" +
		"body: |\n  the second body.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(second, []byte(src), 0o644); err != nil {
		t.Fatalf("write second claim: %v", err)
	}
	gitCommitProject(t, root, "a second claim, committed as a draft")

	// The flip, out of band — no dossierx command could have done it here,
	// because locking is refused until the migration runs.
	if err := os.WriteFile(second, []byte(strings.Replace(src, "status: draft", "status: locked", 1)), 0o644); err != nil {
		t.Fatalf("flip second claim to locked: %v", err)
	}

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if dr.Blocked {
		t.Fatalf("a status flip is a warning, not a refusal — see this test's comment: %+v", dr)
	}
	if !anyContains(dr.SideEffects, "widget.contract.second") {
		t.Fatalf("the preview must name the claim whose lock version control has never seen: %+v", dr.SideEffects)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err != nil || !env.OK {
		t.Fatalf("the migration must run: %v %+v", err, env)
	}
	if !anyContains(env.Warnings, "widget.contract.second") {
		t.Fatalf("the run must warn about the claim it adopted a never-committed lock for: %+v", env.Warnings)
	}
}

// TestMigrateReportsAnUntrackedLockedClaimWithoutRefusingIt.
//
// A locked claim git holds no copy of — untracked, newly added, or moved by a
// claims_dir reorganisation that has not been committed — has no committed state
// to differ from. Reporting it is honest; refusing it would make an ordinary
// reorganisation unmigratable.
func TestMigrateReportsAnUntrackedLockedClaimWithoutRefusingIt(t *testing.T) {
	cfgPath, root, claimPath, _ := gitPreLedgerProject(t)

	// Move the locked claim to a new filename WITHOUT committing the move: git
	// has no blob at the new path.
	moved := filepath.Join(root, "claims", "renamed.yaml")
	if err := os.Rename(claimPath, moved); err != nil {
		t.Fatalf("rename claim: %v", err)
	}

	dr := migrateDryRunOf(t, cfgPath, "--adopt")
	if dr.Blocked {
		t.Fatalf("a claim git has never seen has nothing to differ from, so it must not block: %+v", dr)
	}
	if !anyContains(dr.SideEffects, "widget.contract.main") {
		t.Fatalf("the preview must name the claim it could not corroborate: %+v", dr.SideEffects)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err != nil || !env.OK {
		t.Fatalf("the migration must run: %v %+v", err, env)
	}
	if !anyContains(env.Warnings, "could not be compared against version control") {
		t.Fatalf("the run must say what it could not corroborate: %+v", env.Warnings)
	}
}

// ---------------------------------------------------------------------
// nothing this round added names a command that does not exist
// ---------------------------------------------------------------------

// TestMigrateHistoryProseNamesOnlyRealCommands extends
// TestMigrateRefusalsNameOnlyRealCommands past the refusals, over the OTHER
// prose this round added: the degradation notices (which are emitted on
// SUCCESSFUL runs, where nobody is looking for a mistake) and every
// precondition detail, which is the sentence an agent pastes into a chat to get
// a human's yes.
//
// The history is driven directly rather than through a fixture so every branch
// is covered, including the ones a real project would have to be broken in two
// ways at once to reach.
func TestMigrateHistoryProseNamesOnlyRealCommands(t *testing.T) {
	histories := []migrateHistory{
		{Looked: false, Unlooked: "git is not installed or not on PATH"},
		{Looked: true},
		{Looked: true, Covered: true, CoveredBy: "the comment digest store was in the last commit"},
		{Looked: true, Modified: []string{"widget.contract.main"}},
		{Looked: true, NewlyLocked: []string{"widget.contract.second"}},
		{Looked: true, Uncorroborated: []string{"widget.contract.third"}},
	}
	for _, h := range histories {
		for _, note := range h.notes() {
			namesOnlyRealCommands(t, note)
		}
		plan := migrationPlan{
			Mode:                   migrateModePreLedgerStore,
			History:                h,
			LockStorePath:          ".dossierx-lock-store.json",
			CommentDigestStorePath: ".dossierx-comment-digest.json",
		}
		dr := migrateDryRun(plan, true)
		for _, p := range dr.Preconditions {
			namesOnlyRealCommands(t, p.Detail)
		}
		for _, e := range dr.SideEffects {
			namesOnlyRealCommands(t, e)
		}
	}
}

// ---------------------------------------------------------------------
// preview / write-path parity, for the gates this round added
// ---------------------------------------------------------------------

// TestMigrateDryRunAgreesWithTheWritePathOnTheGitGates is
// TestMigrateDryRunAgreesWithTheWritePath's table extended over the two states
// only git can see. It lives here rather than there because every fixture needs a
// repository, and it asserts the same three things: the preview's blocked
// verdict, the write path's refusal, and that both classify the project with the
// SAME data.mode.
func TestMigrateDryRunAgreesWithTheWritePathOnTheGitGates(t *testing.T) {
	cases := []struct {
		name        string
		project     func(t *testing.T) string
		wantBlocked bool
		wantMode    string
	}{
		{
			name:        "an honest committed pre-ledger project adopts",
			project:     func(t *testing.T) string { cfg, _, _, _ := gitPreLedgerProject(t); return cfg },
			wantBlocked: false,
			wantMode:    migrateModePreLedgerStore,
		},
		{
			name: "a project version control says was covered is refused",
			project: func(t *testing.T) string {
				if _, err := exec.LookPath("git"); err != nil {
					t.Skip("git not on PATH")
				}
				cfgPath, _, storeFile := ledgerProject(t)
				if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved"); err != nil {
					t.Fatalf("claim lock: %v", err)
				}
				gitCommitProject(t, filepath.Dir(cfgPath), "covered")
				rewindStoreToPreLedger(t, storeFile)
				return cfgPath
			},
			wantBlocked: true,
			wantMode:    migrateModeHistoryCovered,
		},
		{
			name: "an edited locked claim is refused",
			project: func(t *testing.T) string {
				cfgPath, _, claimPath, _ := gitPreLedgerProject(t)
				rewriteClaimBody(t, claimPath, "the body an attacker substituted.")
				return cfgPath
			},
			wantBlocked: true,
			wantMode:    migrateModeClaimsModified,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfgPath := tc.project(t)

			dr := migrateDryRunOf(t, cfgPath, "--adopt")
			if dr.Blocked != tc.wantBlocked {
				t.Fatalf("preview blocked=%v, want %v: %+v", dr.Blocked, tc.wantBlocked, dr)
			}
			if dr.From != tc.wantMode {
				t.Fatalf("preview mode=%q, want %q", dr.From, tc.wantMode)
			}

			env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
			refused := err != nil
			if refused != tc.wantBlocked {
				t.Fatalf("the write path refused=%v while the preview said blocked=%v — the two must be one decision (%+v)", refused, tc.wantBlocked, env)
			}
			var data migrateData
			envData(t, env, &data)
			if data.Mode != tc.wantMode {
				t.Fatalf("the write path reported mode=%q, want %q", data.Mode, tc.wantMode)
			}
			if refused && env.Error.Hint == "" {
				t.Fatalf("every refusal carries a recovery: %+v", env.Error)
			}
		})
	}
}
