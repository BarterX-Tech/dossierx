// buildorder_recovery_test.go executes the recovery row from
// skills/dossierx-build-order/SKILL.md's refusal table for a locked claim with
// no `build_role`.
//
// THE DEFECT THIS DETECTED, kept red until the row was fixed. The row used to
// read "set it, then re-propose". The refusal it recovers from can only occur
// on a claim that is already LOCKED — propose's completeness gate guarantees
// it ("refuses outright ... unless 100% of the module's claims are locked") —
// and there is no command that sets build_role: `claim new` takes it at
// creation, `claim flag`/`reaudit` rewrite body only. So "set it" could only
// mean editing the locked claim's file by hand — the one act every other
// document in this repository names as the thing the ledger exists to catch.
// An agent that obeyed the row verbatim cleared the propose refusal and walked
// straight into `integrity_failed` / lock-content-drift on the next
// `dossierx check`, with a recovery table telling it the claims were fine a
// moment ago.
//
// The fixed row routes through the approval path: unlock the claims propose
// named, set `build_role` on each while it is draft (the workshop, where a
// hand edit is the ordinary move), lock them again with the human's reason,
// then re-propose. This scenario enacts that row verbatim and asserts the
// documented outcome the old row could not deliver: the re-propose succeeds
// AND the tree still passes `dossierx check` — a recovery that trades one
// refusal for a worse one is not a recovery. The anchor below trips on any
// rewrite of the row, so a regression to "set it" on the locked file cannot
// keep this scenario green by accident.
package procedures

import "testing"

func TestBuildOrderRecovery_SetItThenReproposeMustSurviveCheck(t *testing.T) {
	f := newFixture(t)

	requireDocAnchor(t, "skills/dossierx-build-order/SKILL.md",
		"| a locked claim has no `build_role` | `build_order_refused` | unlock the claims it names (their yes, `--reason` with their words), set `build_role` on each, lock them again, then re-propose")

	// Setup — the exact state the table's row describes: a module whose claims
	// are ALL locked and NONE carries build_role. Locking succeeds because the
	// build-role lint is adoption-gated and this module never adopted; that is
	// what makes the row's refusal reachable at all.
	f.NewClaim("widget.contract.alpha", "alpha behavior.", "")
	f.LockClaim(defaultClaimID, "approved for the recovery scenario")
	f.LockClaim("widget.contract.alpha", "approved for the recovery scenario")

	f.Plan("build-order recovery: set it, then re-propose (dossierx-build-order)",
		"dossierx build-order propose --module <m>",
		"dossierx claim unlock <id> --reason <words>",
		"dossierx claim unlock <id> --reason <words>",
		"set build_role on each draft claim by editing its file (the row's \"set `build_role` on each\" — a workshop edit now, both claims being draft after unlock)",
		"dossierx claim lock <id> --reason <words>",
		"dossierx claim lock <id> --reason <words>",
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

	// The recovery, verbatim. First move: unlock every claim the refusal named,
	// with the human's yes carried in --reason — releasing the approvals is
	// exactly what makes the edit legal.
	unlockOverview := f.Run("dossierx claim unlock <id> --reason <words>",
		map[string]string{"id": defaultClaimID, "words": "releasing the approval to set build_role, per the recovery row"})
	f.DocumentedSuccess(unlockOverview, "the row's recovery starts by unlocking each named claim")
	unlockAlpha := f.Run("dossierx claim unlock <id> --reason <words>",
		map[string]string{"id": "widget.contract.alpha", "words": "releasing the approval to set build_role, per the recovery row"})
	f.DocumentedSuccess(unlockAlpha, "the row's recovery starts by unlocking each named claim")

	// "set `build_role` on each": the hand edit, now performed where every
	// document licenses it — on a DRAFT claim, the workshop.
	f.Enact("set build_role on each draft claim by editing its file (the row's \"set `build_role` on each\" — a workshop edit now, both claims being draft after unlock)", func() {
		f.SetBuildRoleByHand(defaultClaimID, "behavior")
		f.SetBuildRoleByHand("widget.contract.alpha", "behavior")
	})

	// "lock them again": the `lock` half of unlock → fix → lock, recording a
	// fresh approval over the content that now carries build_role.
	lockOverview := f.Run("dossierx claim lock <id> --reason <words>",
		map[string]string{"id": defaultClaimID, "words": "re-approved with build_role set, per the recovery row"})
	f.DocumentedSuccess(lockOverview, "the row's recovery locks each claim again with the human's reason")
	lockAlpha := f.Run("dossierx claim lock <id> --reason <words>",
		map[string]string{"id": "widget.contract.alpha", "words": "re-approved with build_role set, per the recovery row"})
	f.DocumentedSuccess(lockAlpha, "the row's recovery locks each claim again with the human's reason")

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
