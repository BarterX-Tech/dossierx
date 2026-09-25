// preledger_crossing_test.go executes README's "Upgrading a pre-ledger
// project" fence — unlock every locked claim, then re-lock only what you
// still stand behind — and the same recipe as scripts/ci/dossierx-check.yml's
// commented recovery. Both documents license the same branch at the re-lock
// step: "re-lock only what you still stand behind".
//
// ONE SCENARIO, BOTH END STATES. The crossing unlocks both claims, re-locks
// one and runs the gate: the licensed partial re-lock must already be
// gate-green. It then re-locks the other and runs the gate again, so the full
// re-lock is green too. The cheapest "fix" — deleting the fence, or deleting
// the license to re-lock partially — cannot turn this file green.
package procedures

import "testing"

// buildPreLedgerProject drives a fixture into the exact state the crossing is
// written for: a module fully locked THROUGH the CLI, then the ledger-era
// footprint rewound — lock store back to schema 1 with no ledger key, comment
// digest store removed. The rewind is verified by running the gate: `check`
// must report integrity_failed carrying exactly the project-scoped
// lock-ledger-pre-ledger finding.
func buildPreLedgerProject(f *fixture, t *testing.T) {
	t.Helper()
	f.NewClaim("widget.contract.alpha", "alpha behavior.")
	f.LockClaim(defaultClaimID, "approved before the ledger era")
	f.LockClaim("widget.contract.alpha", "approved before the ledger era")

	f.RewindStoreToPreLedger()

	sanity := f.exec("dossierx check", nil)
	f.RequireFailure(sanity, "integrity_failed",
		"the rewound fixture must present as a pre-ledger project holding locked artifacts")
	rules := sanity.ledgerRules()
	if len(rules) != 1 || rules[0] != "lock-ledger-pre-ledger" {
		t.Fatalf("the rewound fixture must trip exactly the project-scoped lock-ledger-pre-ledger finding (said once, per README); got rules %v\nstdout: %s", rules, sanity.Stdout)
	}
}

// crossingAnchors pins the fence in BOTH documents that carry the recipe.
func crossingAnchors(t *testing.T) {
	t.Helper()
	requireDocAnchor(t, "README.md", "re-lock only what you still stand behind")
	requireDocAnchor(t, "scripts/ci/dossierx-check.yml", "re-lock only what you still stand")
	requireDocAnchor(t, "README.md", "The first `claim lock` in a project holding nothing locked is what crosses it.")
	requireDocAnchor(t, "scripts/ci/dossierx-check.yml", "the first lock in a project")
}

func TestPreLedgerCrossing(t *testing.T) {
	f := newFixture(t)
	crossingAnchors(t)
	buildPreLedgerProject(f, t)

	f.Plan("pre-ledger crossing, partial then full re-lock (README fence + CI template recipe)",
		"dossierx claim unlock <idA> --reason <words>",
		"dossierx claim unlock <idB> --reason <words>",
		"dossierx claim lock <idA> --reason <words>",
		"dossierx check",
		"dossierx claim lock <idB> --reason <words>",
		"dossierx check",
	)

	u1 := f.Run("dossierx claim unlock <idA> --reason <words>",
		map[string]string{"idA": defaultClaimID, "words": "crossing: releasing the pre-ledger approval"})
	f.DocumentedSuccess(u1, "crossing: unlock is documented gateless")
	u2 := f.Run("dossierx claim unlock <idB> --reason <words>",
		map[string]string{"idB": "widget.contract.alpha", "words": "crossing: releasing the pre-ledger approval"})
	f.DocumentedSuccess(u2, "crossing: unlock is documented gateless")

	l1 := f.RunReviewedLock("dossierx claim lock <idA> --reason <words>",
		map[string]string{"idA": defaultClaimID, "words": "still standing behind this one"})
	f.DocumentedSuccess(l1, "crossing: the first re-lock, which stamps the store onto the ledger")

	partial := f.Run("dossierx check", nil)
	f.DocumentedSuccess(partial, "the fence: a partially re-locked module finishes the crossing gate-green")

	l2 := f.RunReviewedLock("dossierx claim lock <idB> --reason <words>",
		map[string]string{"idB": "widget.contract.alpha", "words": "still standing behind this one too"})
	f.DocumentedSuccess(l2, "crossing: the second re-lock")

	full := f.Run("dossierx check", nil)
	f.DocumentedSuccess(full, "a completed crossing leaves a tree the gate accepts")
}
