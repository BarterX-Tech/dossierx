// buildorder_recovery_test.go executes the recovery row from
// skills/dossierx-build-order/SKILL.md's refusal table:
//
//	| a locked claim has no `build_role` | `build_order_refused` | set it, then re-propose |
//
// THE DEFECT THIS DETECTS. The refusal this row recovers from can only occur
// on a claim that is already LOCKED — propose's completeness gate guarantees
// it ("refuses outright ... unless 100% of the module's claims are locked").
// A locked claim without build_role is a reachable state (the lint that
// requires build_role at lock time is adoption-gated: it stays silent in a
// module where no claim has ever set the field). And there is no command that
// sets build_role: `claim new` takes it at creation, `claim flag`/`reaudit`
// rewrite body only. So "set it" can only mean editing the locked claim's
// file by hand — the one act every other document in this repository names as
// the thing the ledger exists to catch. An agent that obeys the row verbatim
// clears the propose refusal and walks straight into `integrity_failed` /
// lock-content-drift on the next `dossierx check`, with a recovery table
// telling it the claims were fine a moment ago.
//
// The row's recovery must CLEAR the refusal — a recovery that trades one
// refusal for a worse one is not a recovery. So the scenario enacts the row
// verbatim (set it by hand, re-propose) and asserts the documented outcome:
// the re-propose succeeds AND the tree still passes `dossierx check`. The
// second half is RED today. It goes green when the row is rewritten to route
// through the approval path (unlock → set → lock), or when a command to set
// build_role on a locked claim exists — either fix satisfies the same
// postcondition.
package procedures

import "testing"

func TestBuildOrderRecovery_SetItThenReproposeMustSurviveCheck(t *testing.T) {
	f := newFixture(t)

	requireDocAnchor(t, "skills/dossierx-build-order/SKILL.md",
		"| a locked claim has no `build_role` | `build_order_refused` | set it, then re-propose |")

	// Setup — the exact state the table's row describes: a module whose claims
	// are ALL locked and NONE carries build_role. Locking succeeds because the
	// build-role lint is adoption-gated and this module never adopted; that is
	// what makes the row's refusal reachable at all.
	f.NewClaim("widget.contract.alpha", "alpha behavior.", "")
	f.LockClaim(defaultClaimID, "approved for the recovery scenario")
	f.LockClaim("widget.contract.alpha", "approved for the recovery scenario")

	f.Plan("build-order recovery: set it, then re-propose (dossierx-build-order)",
		"dossierx build-order propose --module <m>",
		"set build_role on each locked claim by editing its file (the row's \"set it\" — no command sets build_role on a locked claim)",
		"dossierx build-order propose --module <m>",
		"dossierx check",
	)

	// The refusal the row recovers from. This is a documented PREMISE, so it is
	// asserted with the code the table itself names — validated against
	// surface.json's vocabulary, exit status read from surface.json — and any
	// other outcome is fatal: without this refusal there is nothing to recover
	// from and the row (and this scenario) has no subject.
	refusal := f.Run("dossierx build-order propose --module <m>", map[string]string{"m": "widget"})
	f.RequireFailure(refusal, "build_order_refused",
		"the table's own premise: propose refuses a locked claim with no build_role")

	// The recovery, verbatim. "Set it": the only available meaning is the hand
	// edit — enacted on both locked claims, defect included, because enacting a
	// SAFER procedure than the one documented would test a document nobody wrote.
	f.Enact("set build_role on each locked claim by editing its file (the row's \"set it\" — no command sets build_role on a locked claim)", func() {
		f.SetBuildRoleByHand(defaultClaimID, "behavior")
		f.SetBuildRoleByHand("widget.contract.alpha", "behavior")
	})

	// "...then re-propose." The row promises this works now.
	repropose := f.Run("dossierx build-order propose --module <m>", map[string]string{"m": "widget"})
	f.DocumentedSuccess(repropose, "the row's recovery ends in a propose that succeeds")

	// THE ASSERTION THIS SCENARIO EXISTS FOR. A recovery the skill hands to an
	// agent must leave a tree the project's own gate accepts — the whole
	// pipeline, `dossierx check`, is the loop the router skill tells the agent
	// to run "as often as you like" and CI runs on every push. If the row's
	// recovery leaves check failing, the row did not recover anything: it
	// converted a propose refusal into an integrity finding.
	check := f.Run("dossierx check", nil)
	f.DocumentedSuccess(check,
		"the documented recovery must CLEAR the refusal, which includes not manufacturing a new one: dossierx check over the recovered tree must pass")
}
