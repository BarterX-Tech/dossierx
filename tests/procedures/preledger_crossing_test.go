// preledger_crossing_test.go executes README's "Upgrading a pre-ledger
// project" fence — the four numbered command groups — and the same recipe as
// scripts/ci/dossierx-check.yml's commented recovery carries it. Both
// documents license the same branch at step 3: "re-lock only what you still
// stand behind".
//
// THE DEFECT THIS DETECTS. Step 4 says "then the build orders again:
// propose + lock" — unconditionally. But `build-order propose` refuses any
// module that is not 100% locked, so the moment a human exercises the
// documented license to stand behind only SOME claims, step 4 cannot succeed:
// propose refuses (`build_order_refused`, module not fully locked) and the
// subsequent lock has nothing approved to freeze. The crossing's own header
// calls its ordering "load-bearing, not cosmetic" — and then hands out a step
// 3 whose licensed branch breaks its step 4. A team following the fence
// verbatim finishes the crossing with their build orders un-derivable and no
// documented way out that does not contradict "only what you still stand
// behind".
//
// TWO SCENARIOS, ON PURPOSE. The partial-re-lock scenario asserts step 4
// succeeds and is RED today — that is the finding. The all-re-lock sibling
// asserts the same fence GREEN end to end, so the cheapest "fix" — deleting
// the fence, or deleting the license to re-lock partially — cannot turn this
// file green: a real fix has to keep the working path working while repairing
// the licensed branch (condition step 4 on the module's state, or say what a
// partially-re-locked module does about its build orders).
package procedures

import "testing"

// buildPreLedgerProject drives a fixture into the exact state the crossing is
// written for: a module fully locked THROUGH the CLI, its build order proposed
// and locked, and then the ledger-era footprint rewound — lock store back to
// schema 1 with no ledger key, comment digest store removed (see
// RewindStoreToPreLedger for why both files, not one). The rewind is verified
// by running the gate: `check` must report integrity_failed carrying exactly
// the project-scoped lock-ledger-pre-ledger finding. That rule name is typed
// here rather than derived — surface.json carries no ledger-rule inventory —
// and it is fixture sanity, not a scenario assertion: a fixture that is not in
// the documented starting state would make every step after it a test of
// nothing.
func buildPreLedgerProject(f *fixture, t *testing.T) {
	t.Helper()
	// The default claim is still draft; give it a build_role while it is —
	// "draft is your workshop" — so the module can carry a locked build order.
	f.SetBuildRoleByHand(defaultClaimID, "schema")
	f.NewClaim("widget.contract.alpha", "alpha behavior.", "behavior")
	f.LockClaim(defaultClaimID, "approved before the ledger era")
	f.LockClaim("widget.contract.alpha", "approved before the ledger era")
	f.Setup("dossierx build-order propose --module <m>", map[string]string{"m": "widget"})
	f.Setup("dossierx build-order lock --module <m> --reason <words>",
		map[string]string{"m": "widget", "words": "sequence approved before the ledger era"})

	f.RewindStoreToPreLedger()

	sanity := f.exec("dossierx check", nil)
	f.RequireFailure(sanity, "integrity_failed",
		"the rewound fixture must present as a pre-ledger project holding locked artifacts")
	rules := sanity.ledgerRules()
	if len(rules) != 1 || rules[0] != "lock-ledger-pre-ledger" {
		t.Fatalf("the rewound fixture must trip exactly the project-scoped lock-ledger-pre-ledger finding (said once, per README); got rules %v\nstdout: %s", rules, sanity.Stdout)
	}
}

// crossingAnchors pins the fence in BOTH documents that carry the recipe. The
// README fence is the one the scenarios enact; the CI template's commented
// copy is anchored too because it licenses the same branch to a different
// reader, and a fix that repairs one document while the other still hands out
// the broken sequence has not fixed the procedure.
func crossingAnchors(t *testing.T) {
	t.Helper()
	requireDocAnchor(t, "README.md", "re-lock only what you still stand behind")
	requireDocAnchor(t, "README.md", "then the build orders again")
	requireDocAnchor(t, "scripts/ci/dossierx-check.yml", "re-lock only what you still stand")
}

func TestPreLedgerCrossing_PartialRelockThenBuildOrders(t *testing.T) {
	f := newFixture(t)
	crossingAnchors(t)
	buildPreLedgerProject(f, t)

	f.Plan("pre-ledger crossing, partial re-lock (README fence + CI template recipe)",
		"dossierx build-order propose --module <m>",
		"dossierx claim unlock <idA> --reason <words>",
		"dossierx claim unlock <idB> --reason <words>",
		"dossierx claim lock <idA> --reason <words>",
		"dossierx build-order propose --module <m>",
		"dossierx build-order lock --module <m> --reason <words>",
	)

	// Fence step 1: "FIRST, for every module whose build order is locked" —
	// first because propose needs the module's claims still locked, which they
	// are. This releases the locked order so the unlocks can happen at all.
	step1 := f.Run("dossierx build-order propose --module <m>", map[string]string{"m": "widget"})
	f.DocumentedSuccess(step1, "crossing step 1: re-propose every locked build order while the claims are still locked")

	// Fence step 2: "then every locked claim — unlock is gateless".
	for _, bind := range []map[string]string{
		{"idA": defaultClaimID, "words": "crossing: releasing the pre-ledger approval"},
		{"idB": "widget.contract.alpha", "words": "crossing: releasing the pre-ledger approval"},
	} {
		key := "idA"
		if _, ok := bind["idB"]; ok {
			key = "idB"
		}
		step2 := f.Run("dossierx claim unlock <"+key+"> --reason <words>", bind)
		f.DocumentedSuccess(step2, "crossing step 2: unlock is documented gateless")
	}

	// Fence step 3, the LICENSED BRANCH: "re-lock only what you still stand
	// behind" — the human stands behind one claim of two. The first lock in a
	// project holding nothing locked is what crosses the store; the fence says
	// so, and this lock is it.
	step3 := f.Run("dossierx claim lock <idA> --reason <words>",
		map[string]string{"idA": defaultClaimID, "words": "still standing behind this one"})
	f.DocumentedSuccess(step3, "crossing step 3: the first re-lock, which stamps the store onto the ledger")

	// THE ASSERTION THIS SCENARIO EXISTS FOR. Fence step 4, verbatim and
	// unconditional: "then the build orders again". The document that licensed
	// keeping a claim draft two lines earlier promises these two commands next.
	step4a := f.Run("dossierx build-order propose --module <m>", map[string]string{"m": "widget"})
	f.DocumentedSuccess(step4a,
		"crossing step 4 is unconditional in the fence, so it must succeed on every state step 3's license can produce — including a partially re-locked module")
	step4b := f.Run("dossierx build-order lock --module <m> --reason <words>",
		map[string]string{"m": "widget", "words": "sequence re-approved after the crossing"})
	f.DocumentedSuccess(step4b,
		"crossing step 4's second half: lock the re-proposed order")
}

// TestPreLedgerCrossing_FullRelockStaysGreen is the sibling that must PASS on
// every tree, today included: the same fence, taking step 3's other branch
// (stand behind everything). Its job is to make "delete the doc" — or "delete
// the partial-re-lock license" — insufficient as a fix for the red sibling:
// the crossing itself is real, documented, and must keep working.
func TestPreLedgerCrossing_FullRelockStaysGreen(t *testing.T) {
	f := newFixture(t)
	crossingAnchors(t)
	buildPreLedgerProject(f, t)

	f.Plan("pre-ledger crossing, full re-lock (README fence + CI template recipe)",
		"dossierx build-order propose --module <m>",
		"dossierx claim unlock <idA> --reason <words>",
		"dossierx claim unlock <idB> --reason <words>",
		"dossierx claim lock <idA> --reason <words>",
		"dossierx claim lock <idB> --reason <words>",
		"dossierx build-order propose --module <m>",
		"dossierx build-order lock --module <m> --reason <words>",
		"dossierx check",
	)

	step1 := f.Run("dossierx build-order propose --module <m>", map[string]string{"m": "widget"})
	f.DocumentedSuccess(step1, "crossing step 1: re-propose while still locked")

	u1 := f.Run("dossierx claim unlock <idA> --reason <words>",
		map[string]string{"idA": defaultClaimID, "words": "crossing: releasing the pre-ledger approval"})
	f.DocumentedSuccess(u1, "crossing step 2")
	u2 := f.Run("dossierx claim unlock <idB> --reason <words>",
		map[string]string{"idB": "widget.contract.alpha", "words": "crossing: releasing the pre-ledger approval"})
	f.DocumentedSuccess(u2, "crossing step 2")

	l1 := f.Run("dossierx claim lock <idA> --reason <words>",
		map[string]string{"idA": defaultClaimID, "words": "still standing behind this one"})
	f.DocumentedSuccess(l1, "crossing step 3: the first re-lock crosses the store")
	l2 := f.Run("dossierx claim lock <idB> --reason <words>",
		map[string]string{"idB": "widget.contract.alpha", "words": "still standing behind this one too"})
	f.DocumentedSuccess(l2, "crossing step 3: the second re-lock")

	step4a := f.Run("dossierx build-order propose --module <m>", map[string]string{"m": "widget"})
	f.DocumentedSuccess(step4a, "crossing step 4 on a fully re-locked module")
	step4b := f.Run("dossierx build-order lock --module <m> --reason <words>",
		map[string]string{"m": "widget", "words": "sequence re-approved after the crossing"})
	f.DocumentedSuccess(step4b, "crossing step 4's second half")

	// The crossing's terminal promise: "after it lands, every record in the
	// ledger CI reads is an approval a human actually gave" — mechanically,
	// the gate that refused the pre-ledger store must now pass the whole tree.
	check := f.Run("dossierx check", nil)
	f.DocumentedSuccess(check, "a completed crossing leaves a tree the gate accepts")
}
