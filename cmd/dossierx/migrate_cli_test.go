// migrate_cli_test.go is the regression suite for "dossierx migrate --adopt",
// the one-time explicit adoption command that replaced the implicit
// grandfathering every ordinary command used to perform.
//
// It is organized around the four properties the command exists to have, because
// each of them was a defect in something before it was a requirement here:
//
//	it never runs silently          — --adopt is mandatory (TestMigrateRequiresTheAdoptFlag)
//	it previews truthfully          — --dry-run agrees with the write path, on every
//	                                  fixture (TestMigrateDryRunAgreesWithTheWritePath)
//	it is one-time                  — a covered project is refused, never re-adopted
//	it refuses what it must not fix — an absent or downgraded ledger
//
// The happy path itself is pinned next door in integrity_gates_test.go's
// TestUpgradeFailsClosedUntilTheMigrationRuns, which is where the check-refuses ->
// migrate -> check-passes sequence belongs: the migration is only meaningful as
// the thing that clears a gate.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// preLedgerProject is a project in the state every v0.2.x project upgrades from:
// one locked claim, a lock store at the old schema with no ledger in it, and no
// comment digest store beside it.
func preLedgerProject(t *testing.T) (cfgPath, storeFile string) {
	t.Helper()
	cfgPath, _, storeFile = ledgerProject(t)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	rewindStoreToPreLedger(t, storeFile)
	return cfgPath, storeFile
}

// migrateDryRunOf runs the preview and decodes it.
func migrateDryRunOf(t *testing.T, cfgPath string, args ...string) cliout.DryRun {
	t.Helper()
	env, _, err := execCLIJSON(t, append([]string{"--config", cfgPath, "migrate", "--dry-run"}, args...)...)
	if err != nil {
		t.Fatalf("migrate --dry-run must always answer, never fail: %v", err)
	}
	if !env.OK {
		t.Fatalf("a dry run is a successful answer even when blocked, got %+v", env)
	}
	var dr cliout.DryRun
	envData(t, env, &dr)
	return dr
}

// ---------------------------------------------------------------------
// it never runs silently
// ---------------------------------------------------------------------

// TestMigrateRequiresTheAdoptFlag.
//
// A bare "dossierx migrate" must refuse. The command records content nobody
// approved as the baseline every later change is judged against, so a default
// that does it is a default that does it by accident — and the accident is
// unrecoverable, because there is no un-adopt.
func TestMigrateRequiresTheAdoptFlag(t *testing.T) {
	cfgPath, _ := preLedgerProject(t)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate")
	if err == nil || env.OK {
		t.Fatalf("a bare migrate must refuse, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeMissingFlag {
		t.Fatalf("expected %q, got %+v", cliout.CodeMissingFlag, env.Error)
	}
	if !strings.Contains(env.Error.Hint, "dossierx migrate --adopt --dry-run") {
		t.Fatalf("the hint must send the caller to the preview first: %+v", env.Error)
	}
	// And it wrote nothing: the store is still the pre-ledger one.
	if validateReportsRule(t, cfgPath, lock.RuleLockLedgerAdoptionRequired) != true {
		t.Fatalf("a refused migration must leave the project un-migrated")
	}
}

// TestMigrateDryRunReportsAMissingAdoptFlagRatherThanRefusing.
//
// The universal preview rule, restated for this verb: --dry-run answers, it never
// refuses. A caller cannot tell "the preview itself broke" from "the preview says
// no" if both are non-zero exits, which is why lock's own missing --reason is
// reported the same way (TestDryRun_MissingReasonIsReportedNotRefused).
func TestMigrateDryRunReportsAMissingAdoptFlagRatherThanRefusing(t *testing.T) {
	cfgPath, _ := preLedgerProject(t)

	dr := migrateDryRunOf(t, cfgPath)
	if !dr.Blocked {
		t.Fatalf("without --adopt the real run refuses, so the preview must say blocked: %+v", dr)
	}
	if !containsStr(dr.Missing, "--adopt") {
		t.Fatalf("the missing flag must be named: %+v", dr.Missing)
	}
	if !hasPrecondition(dr, "adopt_flag_given", false) {
		t.Fatalf("the preview must evaluate the flag as a gate, got %+v", dr.Preconditions)
	}

	// With the flag, the same project previews as unblocked and NAMES what it
	// would grandfather — the list a human is being asked to approve.
	dr = migrateDryRunOf(t, cfgPath, "--adopt")
	if dr.Blocked {
		t.Fatalf("an honest pre-ledger project must preview as adoptable: %+v", dr)
	}
	proposed, _ := json.Marshal(dr.Proposed["adopted"])
	if !strings.Contains(string(proposed), "widget.contract.main") {
		t.Fatalf("the preview must name every artifact it would grandfather, got %s", proposed)
	}
	if dr.Proposed["grandfathered"] != true {
		t.Fatalf("the preview must say these would be adoptions, not approvals: %+v", dr.Proposed)
	}
}

// TestMigrateDryRunWritesNothing is the property the whole preview surface rests
// on, checked on the one verb where a stray write is irreversible.
func TestMigrateDryRunWritesNothing(t *testing.T) {
	cfgPath, storeFile := preLedgerProject(t)
	before, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}

	migrateDryRunOf(t, cfgPath, "--adopt")

	after, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("re-read store: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("--dry-run rewrote the lock store:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	digestStore := filepath.Join(filepath.Dir(storeFile), ".dossierx-comment-digest.json")
	if _, statErr := os.Stat(digestStore); statErr == nil {
		t.Fatalf("--dry-run created the comment digest store at %s", digestStore)
	}
}

// ---------------------------------------------------------------------
// the preview and the write path are one decision
// ---------------------------------------------------------------------

// TestMigrateDryRunAgreesWithTheWritePath is the parity test, and it is a table
// rather than one case because the ways the two CAN disagree are per-state.
//
// The rule it enforces: for every project shape, dry_run.blocked is true if and
// only if the real run refuses. A preview that says "would adopt" where the real
// run refuses sends an agent to a human for approval of something that cannot
// happen; a preview that says "blocked" where the real run adopts is worse, since
// the adoption then happens without the review the preview exists to obtain.
func TestMigrateDryRunAgreesWithTheWritePath(t *testing.T) {
	cases := []struct {
		name        string
		project     func(t *testing.T) string
		wantBlocked bool
		wantCode    cliout.Code
		wantMode    string
	}{
		{
			name:        "honest pre-ledger project adopts",
			project:     func(t *testing.T) string { cfg, _ := preLedgerProject(t); return cfg },
			wantBlocked: false,
			wantMode:    migrateModePreLedgerStore,
		},
		{
			name: "already covered is refused",
			project: func(t *testing.T) string {
				cfgPath, _, _ := ledgerProject(t)
				if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved"); err != nil {
					t.Fatalf("claim lock: %v", err)
				}
				return cfgPath
			},
			wantBlocked: true,
			wantCode:    cliout.CodeAlreadyMigrated,
			wantMode:    migrateModeAlreadyCovered,
		},
		{
			name: "absent ledger is refused",
			project: func(t *testing.T) string {
				cfgPath, storeFile := preLedgerProject(t)
				if err := os.Remove(storeFile); err != nil {
					t.Fatalf("delete store: %v", err)
				}
				return cfgPath
			},
			wantBlocked: true,
			wantCode:    cliout.CodeIntegrityFailed,
			wantMode:    migrateModeNoStore,
		},
		{
			name: "downgraded store is refused",
			project: func(t *testing.T) string {
				cfgPath, storeFile := preLedgerProject(t)
				// The downgrade, as distinct from the honest rewind: put the
				// version back to 1 while LEAVING a ledger key in place, which
				// is evidence a pre-ledger store cannot honestly hold.
				raw, err := os.ReadFile(storeFile)
				if err != nil {
					t.Fatalf("read store: %v", err)
				}
				var doc map[string]any
				if err := json.Unmarshal(raw, &doc); err != nil {
					t.Fatalf("parse store: %v", err)
				}
				doc["ledger"] = map[string]any{}
				doc["version"] = 1
				out, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					t.Fatalf("marshal store: %v", err)
				}
				if err := os.WriteFile(storeFile, out, 0o644); err != nil {
					t.Fatalf("write store: %v", err)
				}
				return cfgPath
			},
			wantBlocked: true,
			wantCode:    cliout.CodeIntegrityFailed,
			wantMode:    migrateModeDowngraded,
		},
		{
			name: "a project with nothing locked and no store is refused",
			project: func(t *testing.T) string {
				root := t.TempDir()
				cfgPath, _ := icWriteFixtureProject(t, root, "widget")
				return cfgPath
			},
			wantBlocked: true,
			wantCode:    cliout.CodeAlreadyMigrated,
			wantMode:    migrateModeNothingToAdopt,
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
				t.Fatalf("the write path reported mode=%q, want %q — the preview and the run must classify the project identically", data.Mode, tc.wantMode)
			}
			if !refused {
				return
			}
			if env.Error == nil || env.Error.Code != tc.wantCode {
				t.Fatalf("expected %q, got %+v", tc.wantCode, env.Error)
			}
			if env.Error.Hint == "" {
				t.Fatalf("every refusal carries a recovery: %+v", env.Error)
			}
		})
	}
}

// ---------------------------------------------------------------------
// it refuses what it must not repair
// ---------------------------------------------------------------------

// TestMigrateRefusesAnAbsentLedger.
//
// The single most important refusal in the command. An absent lock store is
// indistinguishable from a deleted one, so a migration that adopted there would
// be the second half of a two-command bypass: `rm .dossierx-lock-store.json &&
// dossierx migrate --adopt` would re-bless every locked claim as-found, which is
// exactly what the fail-closed decision exists to prevent. The recovery named is
// version control, and it must never be "run this again".
func TestMigrateRefusesAnAbsentLedger(t *testing.T) {
	cfgPath, storeFile := preLedgerProject(t)
	if err := os.Remove(storeFile); err != nil {
		t.Fatalf("delete store: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err == nil || env.OK {
		t.Fatalf("an absent ledger must never be adopted, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
	}
	if !strings.Contains(env.Error.Hint, "version control") {
		t.Fatalf("the recovery for a deleted ledger is version control: %+v", env.Error)
	}
	if _, statErr := os.Stat(storeFile); statErr == nil {
		t.Fatalf("a refused migration must not create the store it refused to adopt")
	}
}

// TestMigrateRefusesADowngradedStore: the same refusal for the edit that produces
// the pre-ledger SHAPE rather than the pre-ledger state. Adopting here would
// record whatever the claims say now as approved, which is what the edit was for.
func TestMigrateRefusesADowngradedStore(t *testing.T) {
	cfgPath, storeFile := preLedgerProject(t)
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse store: %v", err)
	}
	doc["ledger"] = map[string]any{}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal store: %v", err)
	}
	if err := os.WriteFile(storeFile, out, 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt")
	if err == nil || env.OK {
		t.Fatalf("a downgraded store must never be adopted, got %+v", env)
	}
	var data migrateData
	envData(t, env, &data)
	if data.Mode != migrateModeDowngraded {
		t.Fatalf("the refusal must say WHICH state it hit, got %q", data.Mode)
	}
	if !strings.Contains(env.Error.Hint, "do NOT re-lock") && !strings.Contains(env.Error.Hint, "DO NOT") {
		t.Fatalf("the hint must warn off the destructive recovery: %+v", env.Error)
	}
}

// ---------------------------------------------------------------------
// no refusal names a command that does not exist
// ---------------------------------------------------------------------

// invokedInHint finds every `dossierx <word> [<word>]` a message tells its reader
// to run. It is the same shape internal/comments uses for its own prose (see
// digest_refusal_test.go), reproduced here rather than shared because cmd and
// internal must not import each other's tests.
var invokedInHint = regexp.MustCompile(`dossierx ([a-z][a-z-]*(?: [a-z][a-z-]*)?)`)

// namesOnlyRealCommands checks every command a message names against the ACTUAL
// command tree, so it cannot drift from the surface the way a hand-maintained
// list can.
func namesOnlyRealCommands(t *testing.T, text string) {
	t.Helper()
	for _, m := range invokedInHint.FindAllStringSubmatch(text, -1) {
		if !resolvesToALeaf(strings.Fields(m[1])) {
			t.Errorf("a refusal names %q, which is not a command this binary has: %q", m[1], text)
		}
	}
}

// resolvesToALeaf reports whether words names a real command. It tries the PAIR
// first and then the first word alone, which is the only order that cannot
// produce a false alarm: three of the nouns are groups, so the second captured
// word is either their leaf or simply the next word of the sentence ("dossierx
// check --validate", "dossierx migrate --adopt").
func resolvesToALeaf(words []string) bool {
	root := newRootCmd()
	for i := len(words); i > 0; i-- {
		cmd, _, err := root.Find(words[:i])
		if err == nil && cmd != nil && cmd.Name() == words[i-1] {
			return true
		}
	}
	return false
}

// TestMigrateRefusalsNameOnlyRealCommands.
//
// A prior round shipped a refusal instructing its reader to run a verb the CLI
// does not implement, and the moment a reader follows a hint is the moment they
// are already stuck. Every refusal this command can produce is driven here and
// its message and hint are checked against the real command tree.
func TestMigrateRefusalsNameOnlyRealCommands(t *testing.T) {
	for _, mode := range []string{migrateModeAlreadyCovered, migrateModeNoStore, migrateModeDowngraded, "unknown-future-mode"} {
		plan := migrationPlan{Mode: mode, LockStorePath: ".dossierx-lock-store.json", CommentDigestStorePath: ".dossierx-comment-digest.json"}
		err := migrateRefusal(plan)
		coded := cliout.As(err)
		if coded == nil {
			t.Fatalf("every migrate refusal carries a code: %v", err)
		}
		namesOnlyRealCommands(t, coded.Message)
		namesOnlyRealCommands(t, coded.Hint)
	}

	// And the live one, through the real surface, so the wiring is covered too.
	cfgPath, _ := preLedgerProject(t)
	env, _, _ := execCLIJSON(t, "--config", cfgPath, "migrate")
	if env.Error != nil {
		namesOnlyRealCommands(t, env.Error.Hint)
	}
}

// TestLedgerHintsNameOnlyRealCommands covers the other half of Task 2's rule: the
// recovery attached to every integrity_failed envelope check can emit.
//
// It is a table over the RULE NAMES rather than over live projects because the
// point is coverage of the mapping — a rule added later that falls to the default
// branch still gets a hint, and that hint must still name real commands.
func TestLedgerHintsNameOnlyRealCommands(t *testing.T) {
	rules := []string{
		check.RuleIntegrityStoreRemoved,
		check.RuleClaimsScopeNarrowed,
		check.RuleLedgerUnreadable,
		check.RuleCommentDigestAbsent,
		check.RuleCommentDigestMissing,
		lock.RuleLockLedgerAdoptionRequired,
		lock.RuleLockLedgerDowngraded,
		lock.RuleLockLedgerAbsent,
		lock.RuleLockLedgerDeleted,
		lock.RuleLockLedgerMissing,
		lock.RuleLockLedgerOrphan,
		lock.RuleLockLedgerReleased,
		lock.RuleLockContentDrift,
		lock.RuleLockLedgerAbandoned,
		lock.RuleCommentLedgerDrift,
		lock.RuleCommentDigestUnrecorded,
		"a-rule-invented-after-this-test-was-written",
	}
	for _, rule := range rules {
		hint := ledgerRecoveryHint([]lock.Finding{{Rule: rule, ClaimID: "widget.contract.main"}})
		if strings.TrimSpace(hint) == "" {
			t.Fatalf("rule %q produced no recovery hint; an integrity refusal with an empty hint is the defect this closes", rule)
		}
		namesOnlyRealCommands(t, hint)
	}

	// The adoption-required rule is the one whose recovery is a COMMAND rather
	// than a restore, and it must name that command exactly.
	adoption := ledgerRecoveryHint([]lock.Finding{{Rule: lock.RuleLockLedgerAdoptionRequired}})
	if !strings.Contains(adoption, "dossierx migrate --adopt") {
		t.Fatalf("the fail-closed adoption refusal must name the migration: %q", adoption)
	}
	// And the destructive move must be warned off, in the same breath: re-locking
	// records whatever the claims say now as approved.
	if !strings.Contains(adoption, "re-lock") {
		t.Fatalf("the adoption hint must warn off re-locking: %q", adoption)
	}
}

// ---------------------------------------------------------------------
// every preview agrees with its write path about an un-migrated project
// ---------------------------------------------------------------------

// TestApprovalPreviewsAgreeOnAnUnmigratedProject is the other half of Task 3's
// dry-run parity rule, for the three verbs that record an APPROVAL: claim lock,
// claim reaudit --confirm, and build-order lock.
//
// All three grew a refusal when adoption became fail-closed — lock.Lock returns
// ErrAdoptionRequired, and the two that do not go through lock.Lock are refused
// by requireMigratedProject — and a preview that did not evaluate the same gate
// would disagree in the most damaging direction: the agent shows a human a
// preview that says "would lock", gets a yes, and then cannot deliver it.
//
// Each case asserts BOTH halves, since a missing precondition and a passing one
// are different answers and only the second means the gate exists.
func TestApprovalPreviewsAgreeOnAnUnmigratedProject(t *testing.T) {
	t.Run("claim lock", func(t *testing.T) {
		cfgPath, _ := preLedgerProject(t)
		// A second, draft claim: the one this would lock.
		claimsDir := filepath.Join(filepath.Dir(cfgPath), "claims")
		second := "id: widget.contract.second\nfacet: contract\nmodule: widget\nstatus: draft\nbuild_role: behavior\n" +
			"body: |\n  a second claim, still draft.\n" +
			"rests_on:\n  - widget.contract.main\n" +
			"governed_by:\n  type: none\n  reason: fixture\n"
		if err := os.WriteFile(filepath.Join(claimsDir, "second.yaml"), []byte(second), 0o644); err != nil {
			t.Fatalf("write claim: %v", err)
		}

		env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.second", "--reason", "approved", "--dry-run")
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		var dr cliout.DryRun
		envData(t, env, &dr)
		if !hasPrecondition(dr, "project_migrated", false) {
			t.Fatalf("the preview must report the un-migrated project as a blocking gate: %+v", dr.Preconditions)
		}
		if !dr.Blocked {
			t.Fatalf("the write path refuses here, so the preview must say blocked: %+v", dr)
		}

		env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.second", "--reason", "approved")
		if err == nil || env.OK {
			t.Fatalf("locking into an un-migrated project must be refused, got %+v", env)
		}
		if env.Error == nil || env.Error.Code != cliout.CodeAdoptionRequired {
			t.Fatalf("expected %q, got %+v", cliout.CodeAdoptionRequired, env.Error)
		}
		if !strings.Contains(env.Error.Hint, "dossierx migrate --adopt") {
			t.Fatalf("the refusal must name the one command that clears it: %+v", env.Error)
		}
		namesOnlyRealCommands(t, env.Error.Hint)

		// And the migration actually unblocks it — a refusal whose recovery does
		// not work is worse than no recovery at all.
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt"); err != nil {
			t.Fatalf("migrate --adopt: %v", err)
		}
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.second", "--reason", "approved"); err != nil {
			t.Fatalf("after the migration the lock must succeed: %v", err)
		}
	})

	t.Run("build-order lock", func(t *testing.T) {
		cfgPath := buildOrderFixture(t)
		storeFile := filepath.Join(filepath.Dir(cfgPath), ".dossierx-lock-store.json")
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
			t.Fatalf("propose: %v", err)
		}
		rewindStoreToPreLedger(t, storeFile)

		env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved", "--dry-run")
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		var dr cliout.DryRun
		envData(t, env, &dr)
		if !hasPrecondition(dr, "project_migrated", false) {
			t.Fatalf("the preview must evaluate the same gate the write path does: %+v", dr.Preconditions)
		}

		artifactBefore, err := os.ReadFile(filepath.Join(filepath.Dir(cfgPath), ".build-order.widget.json"))
		if err != nil {
			t.Fatalf("read artifact: %v", err)
		}
		env, _, err = execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved")
		if err == nil || env.OK {
			t.Fatalf("recording a build-order approval into an un-migrated store must be refused, got %+v", env)
		}
		if env.Error == nil || env.Error.Code != cliout.CodeAdoptionRequired {
			t.Fatalf("expected %q, got %+v", cliout.CodeAdoptionRequired, env.Error)
		}
		// The refusal lands BEFORE the artifact is written. A refusal that left
		// locked:true on disk with no record behind it would wedge the module —
		// which is the exact state build-order lock's other integrity refusal
		// exists to recover from.
		artifactAfter, err := os.ReadFile(filepath.Join(filepath.Dir(cfgPath), ".build-order.widget.json"))
		if err != nil {
			t.Fatalf("re-read artifact: %v", err)
		}
		if string(artifactBefore) != string(artifactAfter) {
			t.Fatalf("a refused build-order lock must write nothing:\nbefore:\n%s\nafter:\n%s", artifactBefore, artifactAfter)
		}
	})

	t.Run("claim reaudit --confirm", func(t *testing.T) {
		cfgPath, storeFile := preLedgerProject(t)
		// reaudit needs a locked, review_pending claim. Reaching that state
		// requires a covered project, so it is built first and the store is
		// rewound afterwards — which is exactly the shape a v0.2.x project
		// upgrading with pending drift arrives in.
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "migrate", "--adopt"); err != nil {
			t.Fatalf("migrate (fixture setup): %v", err)
		}
		claimPath := filepath.Join(filepath.Dir(cfgPath), "claims", "main.yaml")
		raw, err := os.ReadFile(claimPath)
		if err != nil {
			t.Fatalf("read claim: %v", err)
		}
		if err := os.WriteFile(claimPath, append(raw, []byte("review_pending: true\n")...), 0o644); err != nil {
			t.Fatalf("write claim: %v", err)
		}
		armLedgerFixture(t, cfgPath)
		rewindStoreToPreLedger(t, storeFile)

		env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main", "--dry-run", "--confirm", "--reason", "approved")
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		var dr cliout.DryRun
		envData(t, env, &dr)
		if !hasPrecondition(dr, "project_migrated", false) {
			t.Fatalf("the preview must evaluate the same gate the write path does: %+v", dr.Preconditions)
		}

		env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main", "--confirm", "--reason", "approved")
		if err == nil || env.OK {
			t.Fatalf("re-signing a claim in an un-migrated project must be refused, got %+v", env)
		}
		if env.Error == nil || env.Error.Code != cliout.CodeAdoptionRequired {
			t.Fatalf("expected %q, got %+v", cliout.CodeAdoptionRequired, env.Error)
		}
		namesOnlyRealCommands(t, env.Error.Hint)
	})
}

// THE PREVIEW MUST REFUSE WHAT THE RUN REFUSES, for the two integrity gates
// lock.Lock grew when re-locking turned out to be the last step of two separate
// bypasses (lock.ErrLedgerRecordDeleted, lock.ErrCommentDigestUnrecorded).
//
// A gate added only to the write path is a preview that says "would lock" and a
// run that refuses — and this is the damaging direction of that disagreement,
// not the harmless one: the agent shows its human a preview, gets a yes, and
// then cannot deliver it. The preconditions read the engine's own exported
// predicates (lock.Store.LedgerRecordDeleted / CommentDigestUnrecorded), so the
// two answers come from one implementation rather than two that agree today.
//
// Without those preconditions both sub-tests fail at `dr.Blocked`.
func TestLockPreviewAgreesWithTheRunOnTheDeletedEvidenceGates(t *testing.T) {
	t.Run("deleted ledger record", func(t *testing.T) {
		cfgPath, claimPath, storeFile := ledgerProject(t)
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved"); err != nil {
			t.Fatalf("seed lock: %v", err)
		}

		// THE ATTACK: delete the record, flip to draft, rewrite the body.
		raw, err := os.ReadFile(storeFile)
		if err != nil {
			t.Fatalf("read store: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("decode store: %v", err)
		}
		delete(doc["ledger"].(map[string]any), "widget.contract.main")
		edited, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("encode store: %v", err)
		}
		if err := os.WriteFile(storeFile, edited, 0o644); err != nil {
			t.Fatalf("write store: %v", err)
		}
		tampered := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: draft\nbuild_role: schema\n" +
			"body: |\n  rewritten now that nothing vouches for it.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n"
		if err := os.WriteFile(claimPath, []byte(tampered), 0o644); err != nil {
			t.Fatalf("write claim: %v", err)
		}

		env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved", "--dry-run")
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		var dr cliout.DryRun
		envData(t, env, &dr)
		if !hasPrecondition(dr, "ledger_record_not_deleted", false) {
			t.Fatalf("the preview must report the deleted ledger record as a blocking gate: %+v", dr.Preconditions)
		}
		if !dr.Blocked {
			t.Fatalf("the write path refuses here, so the preview must say blocked: %+v", dr)
		}

		env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved")
		if err == nil || env.OK {
			t.Fatalf("locking a claim whose ledger record was deleted must be refused, got %+v", env)
		}
		if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
			t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
		}
		// The hint must point at the restore, never at unlock: unlocking accepts
		// the attacker's edit and asks a human to sign it.
		if !strings.Contains(env.Error.Hint, "restore .dossierx-lock-store.json") {
			t.Fatalf("the hint must name the restore: %+v", env.Error)
		}
		namesOnlyRealCommands(t, env.Error.Hint)
	})

	t.Run("deleted comment digest entry", func(t *testing.T) {
		cfgPath, claimPath, storeFile := ledgerProject(t)
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved"); err != nil {
			t.Fatalf("seed lock: %v", err)
		}
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "open for review"); err != nil {
			t.Fatalf("unlock: %v", err)
		}
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "this is wrong, please fix"); err != nil {
			t.Fatalf("comment add: %v", err)
		}

		// THE ATTACK: forge the thread as resolved, drop the one digest key.
		onDisk, err := os.ReadFile(claimPath)
		if err != nil {
			t.Fatalf("read claim: %v", err)
		}
		forged := strings.Replace(string(onDisk), "status: open", "status: resolved", 1)
		if forged == string(onDisk) {
			t.Fatalf("fixture precondition: the comment add should have written an open thread")
		}
		if err := os.WriteFile(claimPath, []byte(forged), 0o644); err != nil {
			t.Fatalf("write claim: %v", err)
		}
		digestFile := filepath.Join(filepath.Dir(storeFile), ".dossierx-comment-digest.json")
		raw, err := os.ReadFile(digestFile)
		if err != nil {
			t.Fatalf("read digest store: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("decode digest store: %v", err)
		}
		delete(doc["digests"].(map[string]any), "widget.contract.main")
		edited, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("encode digest store: %v", err)
		}
		if err := os.WriteFile(digestFile, edited, 0o644); err != nil {
			t.Fatalf("write digest store: %v", err)
		}

		env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved", "--dry-run")
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		var dr cliout.DryRun
		envData(t, env, &dr)
		if !hasPrecondition(dr, "comment_threads_recorded", false) {
			t.Fatalf("the preview must report the missing digest entry as a blocking gate: %+v", dr.Preconditions)
		}
		if !dr.Blocked {
			t.Fatalf("the write path refuses here, so the preview must say blocked: %+v", dr)
		}

		env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved")
		if err == nil || env.OK {
			t.Fatalf("locking a claim whose comment digest entry was deleted must be refused, got %+v", env)
		}
		if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
			t.Fatalf("expected %q, got %+v", cliout.CodeIntegrityFailed, env.Error)
		}
		namesOnlyRealCommands(t, env.Error.Hint)
	})

	// The honest project must still preview as lockable, or the gates above are
	// refusing correct work rather than the two bypasses.
	t.Run("an honest project still previews as lockable", func(t *testing.T) {
		cfgPath, _, _ := ledgerProject(t)
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved"); err != nil {
			t.Fatalf("seed lock: %v", err)
		}
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "a correction"); err != nil {
			t.Fatalf("unlock: %v", err)
		}
		env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "approved", "--dry-run")
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		var dr cliout.DryRun
		envData(t, env, &dr)
		if dr.Blocked {
			t.Fatalf("unlock -> fix -> lock must preview as lockable: %+v", dr.Preconditions)
		}
	})
}
