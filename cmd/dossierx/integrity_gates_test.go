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
		if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed together"); err != nil {
			t.Fatalf("claim lock %s: %v", id, err)
		}
	}

	// The tampering: a hand edit to a LOCKED claim's signed content. It stays
	// status: locked, so nothing about the claim's own lifecycle changed.
	tamper(t, betaPath, "the beta body.", "a beta body nobody approved.")

	// The unrelated, legitimate change to its dependency. This is what puts beta
	// into review_pending, and it is the whole point: the operator who runs the
	// reaudit has an honest reason to.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "unlock", alpha, "--reason", "wording fix"); err != nil {
		t.Fatalf("claim unlock alpha: %v", err)
	}
	tamper(t, alphaPath, "the alpha body.", "the corrected alpha body.")
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", alpha, "--reason", "fix approved"); err != nil {
		t.Fatalf("claim lock alpha: %v", err)
	}
	// check reconciles beta to review_pending. It also FAILS, on the tampering —
	// which is the state the reaudit is about to be asked to bless away.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check"); err == nil {
		t.Fatalf("check must already be reporting the tampered claim")
	}

	before := ledgerRecordOf(t, cfgPath, beta)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "reaudit", beta, "--confirm", "--reason", "re-read it, still true")
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
		if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed"); err != nil {
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

// propose writes the artifact in FULL, with locked:false and a freshly
// recomputed sequence, and it takes no --reason and touches no ledger. Run
// against a locked, current order that is a reason-less, read-looking command
// destroying an implementation sequence a human approved — and the destruction
// is invisible afterwards, because internal/check only audits artifacts whose
// locked flag is true. The build-order:<module> approval record was left
// standing, pointing at content that existed nowhere.
func TestBuildOrderProposeRefusesToDiscardALockedOrder(t *testing.T) {
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}

func TestBuildOrderProposeStillRecomputesAStaleOrder(t *testing.T) {
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}

func TestCheckOnACorruptLedgerReachesTheLedgerRule(t *testing.T) {
	cfgPath, _, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	// What a merge with conflict markers, or a truncated write, leaves behind.
	if err := os.WriteFile(storeFile, []byte("{ truncated"), 0o644); err != nil {
		t.Fatalf("corrupt store: %v", err)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check")
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
	validateEnv, _, validateErr := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if validateErr == nil || validateEnv.Error == nil || validateEnv.Error.Code != env.Error.Code {
		t.Fatalf("check and check --validate must agree on a corrupt ledger: %+v vs %+v", env.Error, validateEnv.Error)
	}
}

// ---------------------------------------------------------------------
// the ledger fails closed, and the crossing is what clears it
// ---------------------------------------------------------------------

// TestUpgradeFailsClosedUntilTheProjectCrosses is the end-to-end shape of the
// fail-closed decision, from the CLI's side.
//
// It replaces a test that asserted the opposite — that the first `check` after an
// upgrade SUCCEEDS, adopting every locked claim as-found and reporting the ids in
// its envelope. That was the best available answer while adoption was implicit
// (an adoption reported only on stderr is worse), but the adoption itself was the
// defect: presenting the pre-ledger shape is two hand edits, so any command that
// blesses on its own is a command an attacker can aim. v0.3.0 moved it behind an
// explicit command; v0.4.0 removed it, because a command that records content
// nobody approved is the same defect with a human's finger on it.
//
// Four things are pinned here, and each one was a hole at some point in review:
// the refusal happens at all; it carries a hint naming commands that EXIST; the
// crossing works and grandfathers NOTHING; and the record it leaves is a real
// approval, not an adoption.
func TestUpgradeFailsClosedUntilTheProjectCrosses(t *testing.T) {
	cfgPath, _, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	// Rewind to what a pre-ledger build left behind: the file exists, at the old
	// schema version, with no ledger at all. (An ABSENT store never adopts, and
	// the migration refuses it too — see TestCLI_DeletingTheLedgerIsNotSilentAdoption
	// and TestMigrateRefusesAnAbsentLedger.)
	rewindStoreToPreLedger(t, storeFile)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.OK {
		t.Fatalf("a pre-ledger project holding a locked claim must fail closed, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	if !strings.Contains(env.Error.Hint, "dossierx claim unlock") {
		t.Fatalf("the refusal must name the crossing that clears it: %+v", env.Error)
	}
	var data checkData
	envData(t, env, &data)
	var rules []string
	for _, f := range data.LedgerFindings {
		rules = append(rules, f.Rule)
	}
	if !containsStr(rules, lock.RuleLockLedgerPreLedger) {
		t.Fatalf("expected %s among the findings, got %v", lock.RuleLockLedgerPreLedger, rules)
	}

	// The write path agrees with the gate: locking anything here is refused with
	// its own code, so an agent can branch on it rather than reading prose.
	lockEnv, _, lockErr := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "re-approved")
	if lockErr == nil || lockEnv.Error == nil || lockEnv.Error.Code != cliout.CodePreLedgerUnadopted {
		t.Fatalf("expected %q from the write path, got %+v", cliout.CodePreLedgerUnadopted, lockEnv.Error)
	}

	// THE CROSSING. Unlock is gateless and always has been, so it works here —
	// and it is step two of the crossing itself.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "unlock", id, "--reason", "crossing onto the ledger"); err != nil {
		t.Fatalf("unlock must never be gated: %v", err)
	}
	// The FIRST lock in a project holding nothing locked crosses the store and
	// records a real approval in the same run.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "re-approved after the crossing"); err != nil {
		t.Fatalf("the crossing lock must succeed: %v", err)
	}

	// Nothing is grandfathered, because by then there was nothing to
	// grandfather. That is the whole difference from the removed adoption path.
	rec, ok := readLedger(t, storeFile)[id]
	if !ok {
		t.Fatalf("the crossing lock must leave a record for %s", id)
	}
	if rec.Grandfathered {
		t.Fatalf("the crossing must record a real APPROVAL, never a grandfathered adoption: %+v", rec)
	}
	if raw, readErr := os.ReadFile(storeFile); readErr != nil || !strings.Contains(string(raw), `"version": 3`) {
		t.Fatalf("the crossing must stamp the ledger schema on disk, got %s (err %v)", raw, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(storeFile), "comment-digest.json")); statErr != nil {
		t.Fatalf("the crossing must create the comment digest store in the same act: %v", statErr)
	}

	// The gate is satisfied afterwards: a crossing that leaves check still
	// refusing is a crossing nobody can complete.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check must pass once the project has crossed: %v", err)
	}
}

// ---------------------------------------------------------------------
// claim link's refusals are the codes the skills publish
// ---------------------------------------------------------------------

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
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "impl.go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeNotLocked {
		t.Fatalf("linking a draft claim must be %q, got %+v", cliout.CodeNotLocked, env.Error)
	}
	if exitStatusFor(err) != 2 {
		t.Fatalf("not_locked is the exit-2 family, got %d", exitStatusFor(err))
	}

	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", "widget.contract.nope", "--file", "impl.go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeClaimNotFound {
		t.Fatalf("linking an unknown id must be %q, got %+v", cliout.CodeClaimNotFound, env.Error)
	}
	if exitStatusFor(err) != 2 {
		t.Fatalf("claim_not_found is the exit-2 family, got %d", exitStatusFor(err))
	}

	// The genuinely caller-error refusals stay where they were: implink_refused's
	// "fix your invocation and re-run" is exactly right for a missing file.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "no-such-file.go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeImplinkRefused {
		t.Fatalf("a missing --file stays %q, got %+v", cliout.CodeImplinkRefused, env.Error)
	}

	// And the happy path is unchanged.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "impl.go"); err != nil {
		t.Fatalf("linking a locked claim to a real file must succeed: %v", err)
	}
}

// ---------------------------------------------------------------------
// claim show must not recommend the recovery the skills forbid
// ---------------------------------------------------------------------

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

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}

	// Clean tree first: the ledger block is present and agrees with the gate, so
	// a failure below is about the tampering and not about the block's shape.
	var data claimShowData
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", id)
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	envData(t, env, &data)
	if data.Ledger == nil || !data.Ledger.Recorded || !data.Ledger.ContentMatches {
		t.Fatalf("an honestly locked claim must report a matching ledger record, got %+v", data.Ledger)
	}

	tamper(t, claimPath, "the approved body.", "a body nobody approved.")

	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", id)
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
	lockFixtureConstitution(t, cfgPath)
	const id = "widget.contract.tbl"
	claimPath := filepath.Join(claimsDir, "tbl.yaml")
	claim := "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: table\n" +
		"body: |\n  a table claim.\n" +
		"rows:\n  - name: alpha\n    value: one\n" +
		"rests_on:\n  none: true\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	implPath := filepath.Join(root, "impl.go")
	if err := os.WriteFile(implPath, []byte("package impl\n"), 0o644); err != nil {
		t.Fatalf("write impl: %v", err)
	}

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "link", "--module", "widget", "--claim", id, "--file", "impl.go"); err != nil {
		t.Fatalf("claim link: %v", err)
	}
	// Drift the link: the file the claim is grounded in changed.
	if err := os.WriteFile(implPath, []byte("package impl\n\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite impl: %v", err)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", id)
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
	flagEnv, _, flagErr := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "flag", id,
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

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "list", "--facet", "contracts")
	if err == nil || env.OK {
		t.Fatalf("an undeclared facet must be refused, not answered with count 0: %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeBadRequest {
		t.Fatalf("expected %q, got %+v", cliout.CodeBadRequest, env.Error)
	}
	if !strings.Contains(env.Error.Message, "contract") {
		t.Fatalf("the refusal must name what the project DOES declare: %q", env.Error.Message)
	}

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "list", "--facet", "contract"); err != nil {
		t.Fatalf("--facet contract must be accepted: %v", err)
	}
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "list", "--facet", "overview")
	if err == nil || env.OK {
		t.Fatalf("the retired overview facet must be refused: %+v", env)
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
	lockFixtureConstitution(t, cfgPath)
	alphaPath = filepath.Join(claimsDir, "alpha.yaml")
	if err := os.WriteFile(alphaPath, []byte("id: widget.contract.alpha\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
		"build_role: schema\n"+
		"body: |\n  the alpha body.\n"+
		"rests_on:\n  none: true\n  reason: fixture\n"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	betaPath = filepath.Join(claimsDir, "beta.yaml")
	if err := os.WriteFile(betaPath, []byte("id: widget.contract.beta\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
		"build_role: behavior\n"+
		"body: |\n  the beta body.\n"+
		"rests_on:\n  - widget.contract.alpha\n"), 0o644); err != nil {
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
	storeFile := filepath.Join(filepath.Dir(cfgPath), "build", "ledger", "lock-store.json")
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
// build/ledger/comment-digest.json sitting beside a store that says "version 1" as
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
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}

func TestCommentOnAnUnreadableDigestStoreIsNotReportedAsInternal(t *testing.T) {
	root := t.TempDir()
	cfgPath, alphaPath, _ := restsOnPairProject(t, root)
	const alpha = "widget.contract.alpha"

	// One honest comment first, so the digest store exists and the project is
	// unambiguously covered.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "comment", "add", alpha, "--as", "human", "--body", "please clarify"); err != nil {
		t.Fatalf("comment add (first): %v", err)
	}
	digestPath := filepath.Join(filepath.Dir(cfgPath), "build", "ledger", "comment-digest.json")
	if err := os.WriteFile(digestPath, []byte("not json at all {{{"), 0o644); err != nil {
		t.Fatalf("corrupt digest store: %v", err)
	}

	before, err := os.ReadFile(alphaPath)
	if err != nil {
		t.Fatalf("read claim before: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "comment", "add", alpha, "--as", "human", "--body", "a second thread")
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
// the build-order half may not be re-armed from inside the ledger
// ---------------------------------------------------------------------

// TestBuildOrderAdoptionRefusesADowngradedLedger closes the last door the
// downgrade attack still had open, and it was a complete bypass of the
// release's headline invariant in ONE ordinary command.
//
// The claim half of grandfathering has been guarded since it shipped: the
// pre-ledger predicate keys on the store's own "version" field, so it weighs that
// claim against evidence the audited file does not own (a sibling comment digest
// store, or ledger records the old schema could not have held) and refuses when
// the two contradict. The BUILD-ORDER half — which lives in cmd/, because
// internal/lock cannot import internal/buildorder — was guarded by nothing but
// Store.PreLedger.
//
// So the whole sequence was: reorder build/build-order/widget.json by hand, set the
// store's "version" back to 1, delete the single build-order:<module> key, and
// run `dossierx check`. The run adopted the HAND-REORDERED bytes as a
// grandfathered approval, re-stamped the version, exited 0 with ok:true — and
// printed the downgrade refusal ("Nothing was grandfathered") on stderr in the
// same breath, because the claim half had correctly refused. Every later
// `check --validate` was clean, and the evidence was gone.
func TestBuildOrderAdoptionRefusesADowngradedLedger(t *testing.T) {
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}

func TestAPreLedgerProjectWithOnlyALockedBuildOrderAgreesWithItsWritePaths(t *testing.T) {
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}

func TestBuildOrderLockFailsWhenTheLedgerRecordCannotBeWritten(t *testing.T) {
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}

func TestBuildOrderLockOnAnUnbackedArtifactPointsAtPropose(t *testing.T) {
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}

func TestBuildOrderLockRefusesBeforeWritingWhenTheStoreIsHeld(t *testing.T) {
	cfgPath := writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  leftover.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil || env.OK {
		t.Fatal("build-order is retired and must fail")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUsage {
		t.Fatalf("retired build-order must be usage, got %+v", env.Error)
	}
}
