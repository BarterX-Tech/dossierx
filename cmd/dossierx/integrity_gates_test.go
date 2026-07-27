// integrity_gates_test.go pins the refusals and the reports that stand between
// "something already approved" and "something rewritten without an approval".
//
// Every test here shares one shape, and it is the shape that makes these bugs
// expensive rather than merely wrong: each of the paths below USED TO SUCCEED.
// A confirmed reaudit re-signed a tampered claim and returned ok:true; a bare
// propose destroyed a locked build order and returned ok:true; check on a
// corrupt ledger returned a write error for a run that wrote nothing; claim show
// reported a tampered claim as settled and recommended the one recovery the
// skills forbid for it. A refusal that is missing is invisible — nothing in the
// output says "this gate did not run" — so these assertions are the only thing
// that keeps them present.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// ---------------------------------------------------------------------
// reaudit --confirm may not re-sign content nobody approved
// ---------------------------------------------------------------------

// TestReauditConfirmRefusesALaunderedClaim reproduces the laundering path end to
// end, using nothing but documented commands.
//
// A locked claim is tampered with out of band. A separate and entirely
// legitimate unlock -> edit -> lock on one of its DEPENDENCIES flips it to
// review_pending with trigger "drift" — no human has seen the tampered fields.
// The documented recovery for a drifted claim is `reaudit <id> --confirm
// --reason "..."`, and reaudit's apply path re-signs the WHOLE claim: without
// the pre-reaudit integrity gate it recorded a fresh approval over the tampered
// bytes, the standing lock-content-drift finding vanished permanently, and check
// went back to reporting ok:true. No unlock ever happened.
func TestReauditConfirmRefusesALaunderedClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, alphaPath, betaPath := restsOnPairProject(t, root)
	const alpha, beta = "widget.contract.alpha", "widget.contract.beta"

	for _, id := range []string{alpha, beta} {
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed together"); err != nil {
			t.Fatalf("claim lock %s: %v", id, err)
		}
	}

	// The tampering: a hand edit to a LOCKED claim's signed content. It stays
	// status: locked, so nothing about the claim's own lifecycle changed.
	tamper(t, betaPath, "the beta body.", "a beta body nobody approved.")

	// The unrelated, legitimate change to its dependency. This is what puts beta
	// into review_pending, and it is the whole point: the operator who runs the
	// reaudit has an honest reason to.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "unlock", alpha, "--reason", "wording fix"); err != nil {
		t.Fatalf("claim unlock alpha: %v", err)
	}
	tamper(t, alphaPath, "the alpha body.", "the corrected alpha body.")
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", alpha, "--reason", "fix approved"); err != nil {
		t.Fatalf("claim lock alpha: %v", err)
	}
	// check reconciles beta to review_pending. It also FAILS, on the tampering —
	// which is the state the reaudit is about to be asked to bless away.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err == nil {
		t.Fatalf("check must already be reporting the tampered claim")
	}

	before := ledgerRecordOf(t, cfgPath, beta)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", beta, "--confirm", "--reason", "re-read it, still true")
	if err == nil || env.OK {
		t.Fatalf("a confirmed reaudit must refuse a claim that no longer matches its ledger record, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	// The refusal has to name both ways out, or its only effect is to strand the
	// operator on a claim no command will touch.
	if !strings.Contains(env.Error.Message, "version control") || !strings.Contains(env.Error.Message, "unlock") {
		t.Fatalf("the refusal must name restore-from-git AND unlock -> fix -> lock: %q", env.Error.Message)
	}

	// Nothing was written: not the claim, not the record.
	if after := ledgerRecordOf(t, cfgPath, beta); after.Hash != before.Hash || after.Reason != before.Reason {
		t.Fatalf("a refused reaudit must not touch the ledger record: %+v -> %+v", before, after)
	}
	raw, readErr := os.ReadFile(betaPath)
	if readErr != nil {
		t.Fatalf("read beta: %v", readErr)
	}
	if !strings.Contains(string(raw), "a beta body nobody approved.") {
		t.Fatalf("a refused reaudit must not rewrite the claim:\n%s", raw)
	}

	// And the finding is still standing, which is the property that used to be
	// destroyed permanently.
	if !hasCLIRule(auditProject(t, cfgPath), lock.RuleLockContentDrift) {
		t.Fatalf("the lock-content-drift finding must survive a refused reaudit")
	}
}

// TestReauditDryRunPreviewsTheIntegrityGate: preview and write path must agree
// about which gate fires. A dry run that reported "not blocked" for a claim the
// real run refuses sends an agent to compose a --reason (usually after asking
// its human for the words) for a command that cannot run.
func TestReauditDryRunPreviewsTheIntegrityGate(t *testing.T) {
	root := t.TempDir()
	cfgPath, _, betaPath := restsOnPairProject(t, root)
	const beta = "widget.contract.beta"

	for _, id := range []string{"widget.contract.alpha", beta} {
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed"); err != nil {
			t.Fatalf("claim lock %s: %v", id, err)
		}
	}

	// Clean tree: the precondition passes and is reported as passing, so a reader
	// can tell "the gate was evaluated" from "the gate does not exist".
	dr := dryRunOf(t, "--config", cfgPath, "claim", "reaudit", beta, "--reason", "why not")
	if !hasPrecondition(dr, "content_matches_ledger", true) {
		t.Fatalf("content_matches_ledger must be reported and passing on a clean claim: %+v", dr.Preconditions)
	}

	tamper(t, betaPath, "the beta body.", "a beta body nobody approved.")
	dr = dryRunOf(t, "--config", cfgPath, "claim", "reaudit", beta, "--reason", "why not")
	if !hasPrecondition(dr, "content_matches_ledger", false) {
		t.Fatalf("a tampered claim's preview must report content_matches_ledger as blocked: %+v", dr.Preconditions)
	}
	if !dr.Blocked {
		t.Fatalf("a preview whose integrity precondition failed must be blocked: %+v", dr)
	}
}

// ---------------------------------------------------------------------
// build-order propose may not discard an approved order
// ---------------------------------------------------------------------

// TestBuildOrderProposeRefusesToDiscardALockedOrder.
//
// propose writes the artifact in FULL, with locked:false and a freshly
// recomputed sequence, and it takes no --reason and touches no ledger. Run
// against a locked, current order that is a reason-less, read-looking command
// destroying an implementation sequence a human approved — and the destruction
// is invisible afterwards, because internal/check only audits artifacts whose
// locked flag is true. The build-order:<module> approval record was left
// standing, pointing at content that existed nowhere.
func TestBuildOrderProposeRefusesToDiscardALockedOrder(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	artifactPath := filepath.Join(filepath.Dir(cfgPath), ".build-order.widget.json")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("first propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}
	approved, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatalf("re-proposing over a locked, current order must be refused, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeAlreadyLocked {
		t.Fatalf("expected %q, got %+v", cliout.CodeAlreadyLocked, env.Error)
	}
	after, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("re-read artifact: %v", err)
	}
	if string(after) != string(approved) {
		t.Fatalf("the approved order must be byte-identical after a refused propose:\n%s", after)
	}

	// The preview agrees with the write path about WHICH gate fired.
	dr := dryRunOf(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if !hasPrecondition(dr, "no_approved_order_to_discard", false) {
		t.Fatalf("the preview must report the blocked precondition: %+v", dr.Preconditions)
	}
}

// TestBuildOrderProposeStillRecomputesAStaleOrder is the other half, and it is
// why the refusal above is scoped to a CURRENT locked order rather than to
// "locked" alone.
//
// Re-proposing a stale order is the documented recovery for build_order_stale —
// buildorder.Lock's own refusal message and the dossierx-build-order skill both
// say "re-propose, then re-lock". A refusal that covered every locked artifact
// would leave a stale order with no way forward at all, which is a worse bug
// than the one being fixed.
func TestBuildOrderProposeStillRecomputesAStaleOrder(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	claimFile := filepath.Join(filepath.Dir(cfgPath), "claims", "a.yaml")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("first propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}

	// Move a covered claim underneath the frozen order, the legitimate way.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "unlock", "widget.contract.a", "--reason", "reopening"); err != nil {
		t.Fatalf("claim unlock: %v", err)
	}
	tamper(t, claimFile, "a locked claim with a build role.", "a rewritten claim with a build role.")
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.a", "--reason", "re-approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err != nil {
		t.Fatalf("re-proposing a STALE order is the documented recovery and must still work: %v (%+v)", err, env.Error)
	}
	var proposed buildOrderProposeData
	envData(t, env, &proposed)
	if proposed.Locked {
		t.Fatalf("a fresh proposal is never locked: %+v", proposed)
	}
	// And the recovery completes: the fresh order can be locked again.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "new order approved"); err != nil {
		t.Fatalf("re-locking the freshly proposed order: %v", err)
	}
}

// ---------------------------------------------------------------------
// a corrupt ledger is a ledger finding, not a write error
// ---------------------------------------------------------------------

// TestCheckOnACorruptLedgerReachesTheLedgerRule.
//
// internal/check's RuleLedgerUnreadable exists precisely so a corrupt store does
// not "crash the command with a parse error that reads like a bug", and it
// carries the one recovery that is correct: restore the ledger from version
// control rather than re-locking, because re-locking would record whatever the
// claims say NOW as approved.
//
// `dossierx check` never reached it. reconcileReviewPending runs first and
// returned the decode failure as a hard error, so the command answered
// write_failed — a code whose documented recovery is retry / check permissions /
// free disk — on a run that had written nothing, and the rule with the right
// advice was unreachable from the command a human and an agent both reach for
// first. internal/check's own test for the property calls check.Run directly,
// which is the seam AFTER reconcile, so it passed throughout.
func TestCheckOnACorruptLedgerReachesTheLedgerRule(t *testing.T) {
	cfgPath, _, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	// What a merge with conflict markers, or a truncated write, leaves behind.
	if err := os.WriteFile(storeFile, []byte("{ truncated"), 0o644); err != nil {
		t.Fatalf("corrupt store: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.OK {
		t.Fatalf("check must fail on a corrupt ledger, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("a corrupt ledger is an integrity failure, not %q: %+v", cliout.CodeWriteFailed, env.Error)
	}

	var data checkData
	envData(t, env, &data)
	var rules []string
	for _, f := range data.LedgerFindings {
		rules = append(rules, f.Rule)
	}
	if !containsStr(rules, check.RuleLedgerUnreadable) {
		t.Fatalf("the envelope must carry %s so the agent gets the restore-from-git recovery, got %v", check.RuleLedgerUnreadable, rules)
	}
	if !containsStr(rules, lock.RuleLockLedgerMissing) {
		t.Fatalf("with no readable evidence every locked claim must read as unapproved, got %v", rules)
	}

	// --validate already had this property; the two doors must not disagree
	// about the same tree.
	validateEnv, _, validateErr := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if validateErr == nil || validateEnv.Error == nil || validateEnv.Error.Code != env.Error.Code {
		t.Fatalf("check and check --validate must agree on a corrupt ledger: %+v vs %+v", env.Error, validateEnv.Error)
	}
}

// ---------------------------------------------------------------------
// adoption fails closed, and the one-time migration is what clears it
// ---------------------------------------------------------------------

// TestUpgradeFailsClosedUntilTheMigrationRuns is the end-to-end shape of the
// fail-closed decision, from the CLI's side.
//
// It replaces a test that asserted the opposite — that the first `check` after an
// upgrade SUCCEEDS, adopting every locked claim as-found and reporting the ids in
// its envelope. That was the best available answer while adoption was implicit
// (an adoption reported only on stderr is worse), but the adoption itself was the
// defect: presenting the pre-ledger shape is two hand edits, so any command that
// adopts on its own is a command an attacker can aim. Now the ordinary command
// REFUSES and names the migration, and the migration is the only thing that
// writes a grandfathered record.
//
// Four things are pinned here, and each one was a hole at some point in review:
// the refusal happens at all; it carries a hint naming a command that EXISTS;
// the migration reports what it adopted in its own envelope rather than on stderr
// alone; and it is one-time, so re-running it is refused rather than being a
// second chance to bless whatever the files say now.
func TestUpgradeFailsClosedUntilTheMigrationRuns(t *testing.T) {
	cfgPath, _, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	// Rewind to what a pre-ledger build left behind: the file exists, at the old
	// schema version, with no ledger at all. (An ABSENT store never adopts, and
	// the migration refuses it too — see TestCLI_DeletingTheLedgerIsNotSilentAdoption
	// and TestMigrateRefusesAnAbsentLedger.)
	rewindStoreToPreLedger(t, storeFile)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.OK {
		t.Fatalf("an un-migrated project must fail closed, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	if !strings.Contains(env.Error.Hint, "dossierx migrate --adopt") {
		t.Fatalf("the refusal must name the command that clears it: %+v", env.Error)
	}
	var data checkData
	envData(t, env, &data)
	var rules []string
	for _, f := range data.LedgerFindings {
		rules = append(rules, f.Rule)
	}
	if !containsStr(rules, lock.RuleLockLedgerAdoptionRequired) {
		t.Fatalf("expected %s among the findings, got %v", lock.RuleLockLedgerAdoptionRequired, rules)
	}

	// The migration. Its envelope carries what it adopted — the whole point of
	// the field: an adoption announced on stderr alone is an adoption an agent
	// following the machine contract never sees.
	migrateEnv, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err != nil {
		t.Fatalf("migrate --adopt: %v", err)
	}
	var migrated migrateData
	envData(t, migrateEnv, &migrated)
	if !containsStr(migrated.Adopted, id) {
		t.Fatalf("the adopted ids must be in the payload, got %v", migrated.Adopted)
	}
	if !migrated.Grandfathered {
		t.Fatalf("the payload must say what these records are: %+v", migrated)
	}
	joined := strings.Join(migrateEnv.Warnings, "\n")
	if !strings.Contains(joined, "GRANDFATHERED") || !strings.Contains(joined, id) {
		t.Fatalf("the adoption must be a warning in the envelope, not stderr only, got %v", migrateEnv.Warnings)
	}
	// The record itself says so, permanently. This is what stops an adoption
	// from ever reading as an approval in a later diff.
	rec, ok := readLedger(t, storeFile)[id]
	if !ok || !rec.Grandfathered {
		t.Fatalf("the adopted record must be marked grandfathered: %+v", rec)
	}

	// The gate is satisfied afterwards: a migration that leaves check still
	// refusing is a migration nobody can complete.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check must pass once the migration has run: %v", err)
	}

	// And it is ONE-TIME. Re-running it must be a refusal, not a second chance
	// to record whatever is on disk now as approved — which is exactly what a
	// re-runnable migration would be for anyone who deleted a record.
	againEnv, _, againErr := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if againErr == nil || againEnv.OK {
		t.Fatalf("a second migration must be refused, got %+v", againEnv)
	}
	if againEnv.Error == nil || againEnv.Error.Code != cliout.CodeAlreadyMigrated {
		t.Fatalf("expected %q, got %+v", cliout.CodeAlreadyMigrated, againEnv.Error)
	}
}

// ---------------------------------------------------------------------
// claim link's refusals are the codes the skills publish
// ---------------------------------------------------------------------

// TestClaimLinkRefusalsCarryTheirOwnCodes.
//
// Every claim link refusal collapsed to implink_refused at exit 1, whose skill
// row reads "This is your invocation or your tag, not a gate: fix it and re-run.
// Do not branch on which". So an agent that hit the not-locked GATE retried with
// a corrected --file and never reached the real recovery (ask the human; lock
// the claim), and an unknown id never reached "dossierx claim list --match"
// either. Both codes are documented in cliout and in two skills, and the dry run
// already computed the distinction correctly — so preview and write path
// disagreed about which gate fired.
func TestClaimLinkRefusalsCarryTheirOwnCodes(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	const id = "widget.contract.overview"
	if err := os.WriteFile(filepath.Join(root, "impl.go"), []byte("package impl\n"), 0o644); err != nil {
		t.Fatalf("write impl file: %v", err)
	}

	// The fixture claim is DRAFT.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "impl.go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeNotLocked {
		t.Fatalf("linking a draft claim must be %q, got %+v", cliout.CodeNotLocked, env.Error)
	}
	if exitStatusFor(err) != 2 {
		t.Fatalf("not_locked is the exit-2 family, got %d", exitStatusFor(err))
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", "widget.contract.nope", "--file", "impl.go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeClaimNotFound {
		t.Fatalf("linking an unknown id must be %q, got %+v", cliout.CodeClaimNotFound, env.Error)
	}
	if exitStatusFor(err) != 2 {
		t.Fatalf("claim_not_found is the exit-2 family, got %d", exitStatusFor(err))
	}

	// The genuinely caller-error refusals stay where they were: implink_refused's
	// "fix your invocation and re-run" is exactly right for a missing file.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "no-such-file.go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeImplinkRefused {
		t.Fatalf("a missing --file stays %q, got %+v", cliout.CodeImplinkRefused, env.Error)
	}

	// And the happy path is unchanged.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "impl.go"); err != nil {
		t.Fatalf("linking a locked claim to a real file must succeed: %v", err)
	}
}

// ---------------------------------------------------------------------
// claim show must not recommend the recovery the skills forbid
// ---------------------------------------------------------------------

// TestClaimShowOnATamperedClaimNeverSuggestsRelocking.
//
// show carried no lock-ledger state at all, so it reported a tampered locked
// claim as locked / not review_pending / settled — the exact opposite of `check
// --validate`'s verdict on the same tree — and its next_actions said "to change
// it: unlock, edit, relock". Following that advice releases the record and
// re-signs the tampered bytes under a fresh approval, and the standing
// lock-content-drift finding disappears with no human ever seeing the diff. The
// router skill's integrity_failed row says it outright: "Do not re-lock to make
// it go away".
func TestClaimShowOnATamperedClaimNeverSuggestsRelocking(t *testing.T) {
	cfgPath, claimPath, _ := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}

	// Clean tree first: the ledger block is present and agrees with the gate, so
	// a failure below is about the tampering and not about the block's shape.
	var data claimShowData
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", id)
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	envData(t, env, &data)
	if data.Ledger == nil || !data.Ledger.Recorded || !data.Ledger.ContentMatches {
		t.Fatalf("an honestly locked claim must report a matching ledger record, got %+v", data.Ledger)
	}

	tamper(t, claimPath, "the approved body.", "a body nobody approved.")

	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "show", id)
	if err != nil {
		t.Fatalf("claim show on a tampered claim must still report (it is a read): %v", err)
	}
	envData(t, env, &data)
	if data.Ledger == nil || data.Ledger.ContentMatches {
		t.Fatalf("show must report the disagreement its own gate reports, got %+v", data.Ledger)
	}

	joined := strings.Join(data.NextActions, "\n")
	if strings.Contains(joined, "claim unlock") {
		t.Fatalf("show must not send an agent to unlock -> relock a tampered claim; that re-signs it: %v", data.NextActions)
	}
	if strings.Contains(joined, "claim reaudit") {
		t.Fatalf("reaudit --confirm now refuses this claim, so show must not offer it: %v", data.NextActions)
	}
	if !strings.Contains(joined, "version control") {
		t.Fatalf("the one correct recovery — restore from version control — must be named: %v", data.NextActions)
	}

	// The verdict show now reports is the gate's own.
	if !hasCLIRule(auditProject(t, cfgPath), lock.RuleLockContentDrift) {
		t.Fatalf("fixture precondition: the gate must be reporting drift")
	}
}

// TestClaimShowNeverSuggestsFlaggingAStructuredLayout.
//
// claim flag structurally refuses any claim whose rendered content lives outside
// Body — table rows, steps, raw HTML — because a flag-sourced reaudit rewrites
// Body only and would clear review_pending while leaving the rendered content
// stale (DX-AUD-11). show's drifted-link action suggested it anyway, with no
// layout check, so an agent composed --claim-says/--now-does/--reason (real work,
// often after asking the human for the wording) and was answered
// structured_layout at exit 1. The route that does work was never offered.
//
// claimNextActions' contract is that its advice can never disagree with what the
// command would actually do; this pins the two together for the one case where
// they had.
func TestClaimShowNeverSuggestsFlaggingAStructuredLayout(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	const id = "widget.contract.tbl"
	claimPath := filepath.Join(claimsDir, "tbl.yaml")
	claim := "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: table\n" +
		"body: |\n  a table claim.\n" +
		"rows:\n  - name: alpha\n    value: one\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	implPath := filepath.Join(root, "impl.go")
	if err := os.WriteFile(implPath, []byte("package impl\n"), 0o644); err != nil {
		t.Fatalf("write impl: %v", err)
	}

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "impl.go"); err != nil {
		t.Fatalf("claim link: %v", err)
	}
	// Drift the link: the file the claim is grounded in changed.
	if err := os.WriteFile(implPath, []byte("package impl\n\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite impl: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", id)
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	var data claimShowData
	envData(t, env, &data)
	drifted := false
	for _, l := range data.ImplementedIn {
		if l.Drifted {
			drifted = true
		}
	}
	if !drifted {
		t.Fatalf("fixture precondition: the link must be reported as drifted, got %+v", data.ImplementedIn)
	}

	joined := strings.Join(data.NextActions, "\n")
	if strings.Contains(joined, "claim flag") {
		t.Fatalf("claim flag is structurally refused for a table layout; show must not suggest it: %v", data.NextActions)
	}
	if !strings.Contains(joined, "claim unlock") {
		t.Fatalf("the route that DOES work — unlock, edit, relock — must be named: %v", data.NextActions)
	}

	// The half that makes the assertion above meaningful: flag really does refuse.
	flagEnv, _, flagErr := execCLIJSON(t, "--config", cfgPath, "claim", "flag", id,
		"--claim-says", "a", "--now-does", "b", "--reason", "c")
	if flagErr == nil || flagEnv.Error == nil || flagEnv.Error.Code != cliout.CodeStructuredLayout {
		t.Fatalf("fixture precondition: claim flag must refuse a table layout, got %+v", flagEnv.Error)
	}
}

// ---------------------------------------------------------------------
// an undeclared facet is reported, not answered with an empty report
// ---------------------------------------------------------------------

// TestClaimListRefusesAnUndeclaredFacet: --module already refused an unknown
// value, for the reason cliout states — "an empty report for a typo'd module
// looks exactly like success" — and --facet, declared in the config the same
// way, filtered with a bare comparison. A human says "show me the contracts
// facet", the project declares `contract`, and the agent reports "there are no
// claims in that facet" at exit 0.
func TestClaimListRefusesAnUndeclaredFacet(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "list", "--facet", "contracts")
	if err == nil || env.OK {
		t.Fatalf("an undeclared facet must be refused, not answered with count 0: %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeBadRequest {
		t.Fatalf("expected %q, got %+v", cliout.CodeBadRequest, env.Error)
	}
	if !strings.Contains(env.Error.Message, "contract") {
		t.Fatalf("the refusal must name what the project DOES declare: %q", env.Error.Message)
	}

	// The declared facet still works, and so does the reserved overview facet.
	for _, facet := range []string{"contract", "overview"} {
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "list", "--facet", facet); err != nil {
			t.Fatalf("--facet %q must be accepted: %v", facet, err)
		}
	}
}

// ---------------------------------------------------------------------
// shared fixtures
// ---------------------------------------------------------------------

// restsOnPairProject writes alpha and beta, both draft, with beta rests_on
// alpha — the minimum shape in which a legitimate change to one claim puts
// another into review_pending.
func restsOnPairProject(t *testing.T, root string) (cfgPath, alphaPath, betaPath string) {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	alphaPath = filepath.Join(claimsDir, "alpha.yaml")
	if err := os.WriteFile(alphaPath, []byte("id: widget.contract.alpha\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
		"build_role: schema\n"+
		"body: |\n  the alpha body.\n"+
		"governed_by:\n  type: none\n  reason: fixture\n"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	betaPath = filepath.Join(claimsDir, "beta.yaml")
	if err := os.WriteFile(betaPath, []byte("id: widget.contract.beta\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
		"build_role: behavior\n"+
		"rests_on:\n  - widget.contract.alpha\n"+
		"body: |\n  the beta body.\n"+
		"governed_by:\n  type: none\n  reason: fixture\n"), 0o644); err != nil {
		t.Fatalf("write beta: %v", err)
	}
	return cfgPath, alphaPath, betaPath
}

// tamper replaces old with new in the file at path, failing the test if the
// substitution did not apply — a fixture whose edit silently did nothing would
// assert the absence of a problem it never created.
func tamper(t *testing.T, path, old, updated string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	edited := strings.Replace(string(raw), old, updated, 1)
	if edited == string(raw) {
		t.Fatalf("fixture precondition: %q not found in %s:\n%s", old, path, raw)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ledgerRecordOf reads one claim's ledger record straight off disk.
func ledgerRecordOf(t *testing.T, cfgPath, id string) lock.LedgerRecord {
	t.Helper()
	storeFile := filepath.Join(filepath.Dir(cfgPath), ".dossierx-lock-store.json")
	rec, ok := readLedger(t, storeFile)[id]
	if !ok {
		t.Fatalf("expected a ledger record for %s", id)
	}
	return rec
}

// rewindStoreToPreLedger rewrites the store as a pre-ledger build would have
// left it: the file EXISTS, at the old schema version, with no ledger key. That
// is the only state adoption triggers on — an absent store never adopts, or
// deleting the ledger would be the universal bypass.
//
// The sibling comment digest store has to go too, and that is not fixture
// tidiness — it is the difference between the two states this helper has to be
// able to tell apart. lock.Store.LedgerDowngraded treats a
// .dossierx-comment-digest.json sitting beside a store that says "version 1" as
// proof the project HAS been through a ledger-aware build, because this build
// writes that file at the exact instant a project becomes ledger-covered. A
// genuine v0.2.x project has never had one: the file did not exist before
// v0.3.0. So a fixture that rewinds only the store is not simulating an honest
// pre-ledger project at all — it is reproducing the downgrade attack, and the
// gate is right to refuse it. Rewinding the whole ledger-era footprint is what
// makes this an upgrade fixture again.
func rewindStoreToPreLedger(t *testing.T, storeFile string) {
	t.Helper()
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	delete(doc, "ledger")
	doc["version"] = 1
	rewound, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(storeFile, rewound, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
	digestStore := filepath.Join(filepath.Dir(storeFile), digest.StoreFileName)
	if err := os.Remove(digestStore); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove comment digest store: %v", err)
	}
}

// hasPrecondition reports whether dr carries a precondition named name with the
// given OK verdict. Preview/write-path parity assertions need BOTH halves: a
// precondition that is absent and one that is present-and-passing are different
// answers, and only the second one means the gate exists.
func hasPrecondition(dr cliout.DryRun, name string, ok bool) bool {
	for _, p := range dr.Preconditions {
		if p.Name == name {
			return p.OK == ok
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Refusals must carry a code whose documented recovery actually applies
// ---------------------------------------------------------------------

// TestBuildOrderLockHandEditReportsItsOwnCode pins the hand-edit refusal to
// build_order_hand_edited rather than the generic build_order_refused.
//
// The refusal itself already existed; only its classification was wrong, and
// that is not cosmetic. Every recovery skills/dossierx-build-order/SKILL.md
// documents for build_order_refused is a repair to the CLAIMS — lock the ones
// still draft, reply to an open thread, set a missing build_role, break a
// rests_on cycle. Here the claims are all fine and the ARTIFACT is what was
// tampered with, so an agent following any of them inspects correct claims,
// finds nothing to fix, and loops. The test drives the documented recovery
// afterwards to prove the code it now reports is the one that works.
func TestBuildOrderLockHandEditReportsItsOwnCode(t *testing.T) {
	// A TWO-phase fixture on purpose: the hand edit below is a reversal of the
	// phase SEQUENCE, which needs at least two phases to be observable and which
	// leaves every per-claim signature byte-identical. A single-phase artifact
	// cannot express the edit that only a re-derivation can catch.
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/schema.yaml": "id: widget.contract.schema\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  a locked schema claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/behavior.yaml": "id: widget.contract.behavior\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: behavior\n" +
			"rests_on:\n  - widget.contract.schema\n" +
			"body: |\n  a locked behavior claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	armLedgerFixture(t, cfgPath)
	artifactPath := filepath.Join(filepath.Dir(cfgPath), ".build-order.widget.json")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}

	// The hand edit: reverse the phase sequence. Every per-claim signature stays
	// byte-identical, so only a re-derivation can see it.
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var artifact map[string]any
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	phases, ok := artifact["phases"].([]any)
	if !ok || len(phases) < 2 {
		t.Fatalf("fixture must derive at least 2 phases for the sequence reversal to be observable, got %d", len(phases))
	}
	for i, j := 0, len(phases)-1; i < j; i, j = i+1, j-1 {
		phases[i], phases[j] = phases[j], phases[i]
	}
	artifact["phases"] = phases
	edited, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("encode artifact: %v", err)
	}
	if err := os.WriteFile(artifactPath, edited, 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved")
	if err == nil {
		t.Fatalf("expected build-order lock to refuse a hand-edited artifact")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeBuildOrderHandEdited {
		t.Fatalf("expected %s, got %+v", cliout.CodeBuildOrderHandEdited, env.Error)
	}

	// The documented recovery for THIS code — re-propose, then lock — must work,
	// which is what makes the code worth splitting out.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("re-propose after a hand edit must be allowed: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
		t.Fatalf("lock after the re-propose must succeed: %v", err)
	}
}

// TestCommentOnAnUnreadableDigestStoreIsNotReportedAsInternal pins the comment
// refusal to comment_digest_unavailable rather than internal.
//
// internal is defined in cliout/codes.go as an unclassified failure — "a bug
// report, not a branch target" — and the reflex it invites is a retry. This
// refusal is deterministic and will fail identically until the store is
// restored, so reporting it as internal sends a caller into a retry loop over a
// comment op. The test also asserts the property that makes the code safe to
// act on: across two attempts, nothing is written.
func TestCommentOnAnUnreadableDigestStoreIsNotReportedAsInternal(t *testing.T) {
	root := t.TempDir()
	cfgPath, alphaPath, _ := restsOnPairProject(t, root)
	const alpha = "widget.contract.alpha"

	// One honest comment first, so the digest store exists and the project is
	// unambiguously covered.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", alpha, "--as", "human", "--body", "please clarify"); err != nil {
		t.Fatalf("comment add (first): %v", err)
	}
	digestPath := filepath.Join(filepath.Dir(cfgPath), ".dossierx-comment-digest.json")
	if err := os.WriteFile(digestPath, []byte("not json at all {{{"), 0o644); err != nil {
		t.Fatalf("corrupt digest store: %v", err)
	}

	before, err := os.ReadFile(alphaPath)
	if err != nil {
		t.Fatalf("read claim before: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		env, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", alpha, "--as", "human", "--body", "a second thread")
		if err == nil {
			t.Fatalf("attempt %d: expected the comment op to be refused", attempt)
		}
		if env.Error == nil || env.Error.Code != cliout.CodeCommentDigestUnavailable {
			t.Fatalf("attempt %d: expected %s, got %+v", attempt, cliout.CodeCommentDigestUnavailable, env.Error)
		}
		after, err := os.ReadFile(alphaPath)
		if err != nil {
			t.Fatalf("attempt %d: read claim after: %v", attempt, err)
		}
		if string(after) != string(before) {
			t.Fatalf("attempt %d: the refusal wrote to the claim; a retry would duplicate the thread", attempt)
		}
	}
}

// ---------------------------------------------------------------------
// build-order grandfathering may not be re-armed from inside the ledger
// ---------------------------------------------------------------------

// TestBuildOrderAdoptionRefusesADowngradedLedger closes the last door the
// downgrade attack still had open, and it was a complete bypass of the
// release's headline invariant in ONE ordinary command.
//
// The claim half of grandfathering has been guarded since it shipped: adoption
// keys on the store's own "version" field, so lock.AdoptProject weighs that claim
// against evidence the audited file does not own (a sibling comment digest
// store, or ledger records the old schema could not have held) and refuses when
// the two contradict. The BUILD-ORDER half — which lives in cmd/, because
// internal/lock cannot import internal/buildorder — was guarded by nothing but
// Store.PreLedger.
//
// So the whole sequence was: reorder .build-order.widget.json by hand, set the
// store's "version" back to 1, delete the single build-order:<module> key, and
// run `dossierx check`. The run adopted the HAND-REORDERED bytes as a
// grandfathered approval, re-stamped the version, exited 0 with ok:true — and
// printed the downgrade refusal ("Nothing was grandfathered") on stderr in the
// same breath, because the claim half had correctly refused. Every later
// `check --validate` was clean, and the evidence was gone.
func TestBuildOrderAdoptionRefusesADowngradedLedger(t *testing.T) {
	cfgPath := twoPhaseBuildOrderFixture(t)
	dir := filepath.Dir(cfgPath)
	artifactPath := filepath.Join(dir, ".build-order.widget.json")
	storeFile := filepath.Join(dir, ".dossierx-lock-store.json")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}

	// The hand edit. Reversing the phase blocks changes the implementation
	// sequence an agent would follow without touching a single claim, which is
	// exactly what RuleBuildOrderContentDrift exists to catch.
	reverseArtifactPhases(t, artifactPath)
	if !validateReportsRule(t, cfgPath, "build-order-content-drift") {
		t.Fatalf("fixture precondition: the hand edit must be reported as drift before the downgrade")
	}

	// The downgrade: the store says it predates the ledger, while still holding
	// the claim's own record (and sitting beside the comment digest store), so
	// both halves of the contradiction lock.Store.LedgerDowngraded weighs are
	// present.
	downgradeStoreAndDropKey(t, storeFile, lock.BuildOrderLedgerKey("widget"))
	if !validateReportsRule(t, cfgPath, "lock-ledger-downgraded") {
		t.Fatalf("fixture precondition: the downgrade must be reported before the writing run")
	}

	// The writing run. Its own verdict is not what is on trial here (with the
	// ledger downgraded it has plenty to report); what matters is that it
	// adopted nothing and left the evidence intact.
	env, _, _ := execCLIJSON(t, "--config", cfgPath, "check")
	for _, w := range env.Warnings {
		if strings.Contains(w, lock.BuildOrderLedgerKey("widget")) {
			t.Fatalf("the run announced adopting a build order on a downgraded ledger: %q", w)
		}
	}
	if rec, ok := rawLedgerOf(t, storeFile)[lock.BuildOrderLedgerKey("widget")]; ok {
		t.Fatalf("a downgraded ledger must never be re-signed; got %+v", rec)
	}

	// And the gate still says so afterwards. This is the assertion that the
	// original defect turned false: a second `check --validate` reported ok:true
	// with zero findings, forever.
	if !validateReportsRule(t, cfgPath, "lock-ledger-downgraded") {
		t.Fatalf("the downgrade must still be reported after the writing run; the evidence was destroyed")
	}
}

// TestBuildOrderAdoptionMovedToTheMigrationCommand is the other half, and it is
// where the last automatic adoption in the binary went.
//
// A genuine v0.2.x project has a build order locked before this build gave build
// orders a record, no ledger records at all, and no comment digest store. It has
// to end up with a grandfathered record — refusing outright would fail every
// honest upgrade forever with a build-order-ledger-missing the project had no way
// to have avoided, which is how a gate gets switched off rather than fixed.
//
// What changed is WHO does it. `check` used to, from inside cmd/dossierx's
// prepareStore, on the same PreLedgerExempt predicate the claim half used — so
// when ledger adoption became explicit, this was left as the one path by which an
// ordinary command still signed locked content as-found. It moved into
// planMigration. This test pins both halves of that move: the ordinary command
// adopts NOTHING, and the migration adopts it, grandfathered.
func TestBuildOrderAdoptionMovedToTheMigrationCommand(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	dir := filepath.Dir(cfgPath)
	storeFile := filepath.Join(dir, ".dossierx-lock-store.json")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}

	// rewindStoreToPreLedger removes the WHOLE ledger-era footprint — records,
	// version stamp and the sibling digest store — which is what makes this an
	// upgrade fixture rather than a reproduction of the attack above.
	rewindStoreToPreLedger(t, storeFile)

	// The ordinary command: it may fail (this project is un-migrated, so the
	// gate refuses it — that is the fail-closed decision working), but whatever
	// it reports, it must not have signed the build order.
	execCLIJSON(t, "--config", cfgPath, "check") //nolint:errcheck // the verdict is not what is on trial; the ledger is
	// rawLedgerOf, not readLedger: the store is still at the PRE-ledger schema
	// version at this point (nothing has migrated it), and readLedger asserts the
	// current one.
	if rec, ok := rawLedgerOf(t, storeFile)[lock.BuildOrderLedgerKey("widget")]; ok {
		t.Fatalf("an ordinary command must not adopt a build order any more; got %+v", rec)
	}

	// The migration: the deliberate act, and the only one that writes a
	// grandfathered record.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err != nil {
		t.Fatalf("migrate --adopt on an honest pre-ledger project must succeed: %v", err)
	}
	var data migrateData
	envData(t, env, &data)
	if !containsStr(data.Adopted, lock.BuildOrderLedgerKey("widget")) {
		t.Fatalf("the migration must name the build order it adopted, got %v", data.Adopted)
	}
	rec, ok := readLedger(t, storeFile)[lock.BuildOrderLedgerKey("widget")]
	if !ok {
		t.Fatalf("the migration must adopt the locked build order")
	}
	if !rec.Grandfathered {
		t.Fatalf("an adopted build order is grandfathered, never approved: %+v", rec)
	}
	// And the project is clean afterwards: a migration that leaves the gate
	// still refusing is a migration nobody can complete.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check must pass once the migration has run: %v", err)
	}
}

// ---------------------------------------------------------------------
// an unbacked "locked": true must be recoverable, and must never read ok
// ---------------------------------------------------------------------

// TestBuildOrderLockFailsWhenTheLedgerRecordCannotBeWritten.
//
// "build-order lock" writes two files: the artifact, then the ledger record. It
// used to report the second one's failure as a WARNING on an ok:true envelope,
// which is a false machine contract — `check --validate` refuses the very next
// run on a locked build order with no record, so the only consumer that mattered
// disagreed with the answer the command had just given. An agent reading ok and
// the exit status concluded the order was approved.
func TestBuildOrderLockFailsWhenTheLedgerRecordCannotBeWritten(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	dir := filepath.Dir(cfgPath)
	artifactPath := filepath.Join(dir, ".build-order.widget.json")
	storeFile := filepath.Join(dir, ".dossierx-lock-store.json")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	good, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	if err := os.WriteFile(storeFile, []byte("not json at all {{{"), 0o644); err != nil {
		t.Fatalf("corrupt store: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved")
	if err == nil || env.OK {
		t.Fatalf("a lock whose approval could not be recorded must not report ok:true, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	// The message has to say what IS on disk. The artifact is locked; telling
	// the caller only "the write failed" invites a retry of a command whose
	// first half already happened.
	if !strings.Contains(env.Error.Message, "written to disk as locked") {
		t.Fatalf("the refusal must say the artifact is on disk: %+v", env.Error)
	}
	if !strings.Contains(env.Error.Hint, "build-order propose --module widget") {
		t.Fatalf("the refusal must name the recovery: %+v", env.Error)
	}

	// And the recovery is real. This is the half that used to wedge the module:
	// the artifact says locked:true, so propose refused ("already_locked") and
	// lock refused ("already locked and not stale"), and there is no unlock
	// verb. Restoring the store leaves exactly that state.
	if err := os.WriteFile(storeFile, good, 0o644); err != nil {
		t.Fatalf("restore store: %v", err)
	}
	if artifact := readArtifactJSON(t, artifactPath); artifact["locked"] != true {
		t.Fatalf("fixture precondition: the artifact must be locked on disk, got %v", artifact["locked"])
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("re-proposing over an UNBACKED locked artifact must be allowed; it discards nothing: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved for real"); err != nil {
		t.Fatalf("the recovery must complete: %v", err)
	}
	if _, ok := readLedger(t, storeFile)[lock.BuildOrderLedgerKey("widget")]; !ok {
		t.Fatalf("the recovery must leave a standing approval on the record")
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate"); err != nil {
		t.Fatalf("the repaired project must validate clean: %v", err)
	}
}

// TestBuildOrderLockOnAnUnbackedArtifactPointsAtPropose.
//
// buildorder.Lock's own refusal for a locked, non-stale artifact is "already
// locked and not stale", classified already_locked — a code whose documented
// meaning is "there is nothing to do". For an artifact whose locked flag nothing
// backs that is exactly wrong: `check --validate` is refusing every commit with
// build-order-ledger-missing, so there is a great deal to do, and the two verbs
// the finding's own message named were the two that refused.
func TestBuildOrderLockOnAnUnbackedArtifactPointsAtPropose(t *testing.T) {
	cfgPath := buildOrderFixture(t)
	dir := filepath.Dir(cfgPath)
	storeFile := filepath.Join(dir, ".dossierx-lock-store.json")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}
	// The state a crash between the two writes leaves: artifact locked, record
	// gone, store otherwise current (so this is NOT the pre-ledger path).
	downgradeStoreAndDropKey(t, storeFile, lock.BuildOrderLedgerKey("widget"))
	restoreStoreVersion(t, storeFile)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approving again")
	if err == nil || env.OK {
		t.Fatalf("locking an artifact with no standing approval must be refused, got %+v", env)
	}
	if env.Error == nil || env.Error.Code == cliout.CodeAlreadyLocked {
		t.Fatalf("already_locked means \"nothing to do\", which is the opposite of this state: %+v", env.Error)
	}
	if env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	if !strings.Contains(env.Error.Hint, "build-order propose --module widget") {
		t.Fatalf("the refusal must name the one command that unwedges the module: %+v", env.Error)
	}

	// Preview/write-path parity. A preview that reported only
	// "not_already_current" would send the reader to already_locked's recovery
	// ("there is nothing to do") for the one state where there is.
	dr := dryRunOf(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approving again")
	if !hasPrecondition(dr, "lock_flag_is_backed_by_an_approval", false) {
		t.Fatalf("the preview must report the unbacked lock as what blocks it: %+v", dr.Preconditions)
	}
}

// TestBuildOrderLockRefusesBeforeWritingWhenTheStoreIsHeld reproduces the
// reported wedge exactly: another process holds .dossierx-lock-store.json.lock
// — which an ordinary concurrent `check` or `claim lock` does — while
// "build-order lock" runs.
//
// It used to write the artifact first and take that sentinel second, so
// contention produced a locked artifact with no record, ok:true, exit 0, and a
// project `check --validate` refused. Taking the sentinel BEFORE the artifact
// write converts the whole class into a clean refusal that wrote nothing, under
// a code whose documented recovery — retry — actually works.
func TestBuildOrderLockRefusesBeforeWritingWhenTheStoreIsHeld(t *testing.T) {
	if testing.Short() {
		t.Skip("the file lock's acquisition timeout is 10s and is not overridable from this package")
	}
	cfgPath := buildOrderFixture(t)
	dir := filepath.Dir(cfgPath)
	artifactPath := filepath.Join(dir, ".build-order.widget.json")
	storeFile := filepath.Join(dir, ".dossierx-lock-store.json")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	release, err := lock.AcquireFileLock(storeFile)
	if err != nil {
		t.Fatalf("hold the lock-store sentinel: %v", err)
	}
	defer release()

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved")
	if err == nil || env.OK {
		t.Fatalf("a lock that could not take the lock-store sentinel must not report ok:true, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeWriteConflict {
		t.Fatalf("contention is %q, the one code whose recovery is a retry: %+v", cliout.CodeWriteConflict, env.Error)
	}
	if artifact := readArtifactJSON(t, artifactPath); artifact["locked"] == true {
		t.Fatalf("the refusal must have written nothing; the artifact was left locked with no record")
	}
}

// ---------------------------------------------------------------------
// helpers for the two sections above
// ---------------------------------------------------------------------

// twoPhaseBuildOrderFixture is buildOrderFixture with a second locked claim in
// a LATER build phase, so the artifact it produces has two phase blocks to
// reorder. A single-phase order cannot express the edit the drift rule exists to
// catch — changing what gets built first — so the fixture that tests that edit
// has to have somewhere for it to happen.
func twoPhaseBuildOrderFixture(t *testing.T) string {
	t.Helper()
	return writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  a locked claim with a build role.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/b.yaml": "id: widget.contract.b\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: behavior\n" +
			"rests_on:\n  - widget.contract.a\n" +
			"body: |\n  a second locked claim, one phase later.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
}

// readArtifactJSON decodes a build-order artifact as a bare map, so a fixture
// can assert on (and edit) the BYTES on disk rather than whatever
// buildorder.Status recomputes in memory.
func readArtifactJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	return doc
}

// reverseArtifactPhases rewrites a build-order artifact with its phase blocks in
// the opposite order: a hand edit that changes the implementation sequence an
// agent would follow while touching no claim at all, and that no lint can see.
func reverseArtifactPhases(t *testing.T, path string) {
	t.Helper()
	doc := readArtifactJSON(t, path)
	phases, ok := doc["phases"].([]any)
	if !ok || len(phases) < 2 {
		t.Fatalf("fixture precondition: the artifact needs at least two phase blocks to reorder, got %v", doc["phases"])
	}
	for i, j := 0, len(phases)-1; i < j; i, j = i+1, j-1 {
		phases[i], phases[j] = phases[j], phases[i]
	}
	doc["phases"] = phases
	edited, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
}

// rawLedgerOf reads the store's ledger map WITHOUT asserting the schema
// version, which readLedger deliberately does. A fixture that has just
// downgraded the version on purpose needs to read the file it downgraded; the
// version assertion is right for every other caller and wrong for this one.
func rawLedgerOf(t *testing.T, storeFile string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc struct {
		Ledger map[string]any `json:"ledger"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	return doc.Ledger
}

// downgradeStoreAndDropKey is the ATTACK, as distinct from
// rewindStoreToPreLedger's honest upgrade: it sets the version back to 1 and
// removes exactly ONE ledger key, leaving every other record — and the sibling
// comment digest store — in place. Both halves of the contradiction
// lock.Store.LedgerDowngraded weighs are therefore present, which is precisely
// what a genuine pre-ledger project cannot have.
func downgradeStoreAndDropKey(t *testing.T, storeFile, key string) {
	t.Helper()
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	ledger, ok := doc["ledger"].(map[string]any)
	if !ok {
		t.Fatalf("fixture precondition: the store has no ledger to edit:\n%s", raw)
	}
	if _, ok := ledger[key]; !ok {
		t.Fatalf("fixture precondition: no ledger record for %q:\n%s", key, raw)
	}
	delete(ledger, key)
	doc["ledger"] = ledger
	doc["version"] = 1
	edited, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(storeFile, edited, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
}

// restoreStoreVersion puts the schema version back, so a fixture can build the
// "record simply missing" state (a torn write, a crash) without also building
// the "store claims to predate the ledger" one. The two are different findings
// with different recoveries and must be testable apart.
func restoreStoreVersion(t *testing.T, storeFile string) {
	t.Helper()
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	doc["version"] = 2
	edited, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(storeFile, edited, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
}

// validateReportsRule reports whether "check --validate" — the read-only pass,
// which writes nothing and so cannot itself change the state under test —
// reports a ledger finding with the given rule name.
func validateReportsRule(t *testing.T, cfgPath, rule string) bool {
	t.Helper()
	env, _, _ := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	var data struct {
		LedgerFindings []struct {
			Rule string `json:"rule"`
		} `json:"ledger_findings"`
	}
	envData(t, env, &data)
	for _, f := range data.LedgerFindings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}
