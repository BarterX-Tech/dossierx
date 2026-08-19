// review_loop_test.go executes skills/dossierx-comments/SKILL.md's "The whole
// loop" table — the seven-step review conversation between the human in the
// viewer and the agent on the CLI — in ONE fixture, in the documented order,
// with no resets between steps.
//
// THE DEFECT THIS DETECTED, kept red until the table was fixed. Step 4 used to
// end the locked-claim branch in a lock ("unlock → fix → lock"), but the
// human's thread is still open at step 4 by the table's own construction — the
// human does not click Resolve until step 5 — and an open thread is precisely
// what the lock gate refuses (`unresolved_comments`, per the same skill's "How
// an open thread gates the lifecycle"). An agent following that table verbatim
// wedged at step 4 with a refusal whose documented recovery ("the human
// resolves") was the very step the table scheduled AFTER the lock.
//
// The fixed table ends step 4 in the reply: a locked claim goes through
// unlock → fix at step 4 and WAITS in draft, its approval released, until
// step 7 — where the lock after the human's Resolve is the `lock` half of
// unlock → fix → lock. This scenario replays that sequence and asserts every
// step's DOCUMENTED outcome, so it goes red again if the table regresses to
// scheduling a lock while the thread it answers is still open (the anchor
// below trips on any rewrite of the branch), or if the gate stops honouring
// the order the table now gives.
package procedures

import (
	"strings"
	"testing"
)

func TestReviewLoop_Step4RelockRunsBeforeTheHumanResolves(t *testing.T) {
	f := newFixture(t)

	// The enactment below is only meaningful while the document still gives
	// these steps in this order. If either phrase is gone, the procedure moved:
	// re-read the skill and update the enactment, do not delete the scenario.
	requireDocAnchor(t, "skills/dossierx-comments/SKILL.md",
		"or take a locked one through unlock → fix, with their yes — the re-lock waits for their Resolve")
	requireDocAnchor(t, "skills/dossierx-comments/SKILL.md",
		"clicks **Resolve** in the viewer")

	// Setup — the state the loop's steps 1–2 leave behind: a LOCKED claim (the
	// branch of step 4 under test) carrying a thread the human opened. The
	// thread is opened with the CLI's --as human, which is the same code path
	// behind the same project-wide lock as the viewer's composer (the skill
	// says so in "The verbs"), so no browser is needed to reach this state.
	f.LockClaim(defaultClaimID, "approved before the review round")
	// The thread's text deliberately shares no token with the claim body:
	// step 4's "fix" edits the body by hand, and an overlap would make that
	// edit reach into the engine-managed comments block (see RewriteClaimBody).
	tid := f.OpenHumanThread(defaultClaimID, "is that latency budget right?")

	f.Plan("review-loop step 4 (dossierx-comments)",
		"dossierx comment inbox",
		"dossierx claim unlock <id> --reason <words>",
		"hand-edit the draft claim's body (step 4's \"fix\"; a draft is freely editable)",
		"dossierx comment reply <id> <tid> --as agent --body <body>",
		"POST /api/claims/<id>/comments/<tid>/resolve (loop step 5: the human's Resolve click, on the viewer surface)",
		"dossierx claim lock <id> --reason <words>",
	)

	// Loop step 3: one call, project-wide.
	inbox := f.Run("dossierx comment inbox", nil)
	f.DocumentedSuccess(inbox, "loop step 3: `dossierx comment inbox` — every open thread in the project, one call")
	assertInboxListsHumanThread(t, inbox, tid)

	// Loop step 4, exactly as the table orders it: unlock → fix, then reply.
	// The re-lock is NOT here — the thread is still open, and the table now
	// says so: the claim waits in draft until step 7.
	unlock := f.Run("dossierx claim unlock <id> --reason <words>",
		map[string]string{"id": defaultClaimID, "words": "taking it through unlock -> fix, per step 4; the re-lock waits for their Resolve"})
	f.DocumentedSuccess(unlock, "loop step 4: unlock is the first move of the locked-claim branch")

	f.Enact("hand-edit the draft claim's body (step 4's \"fix\"; a draft is freely editable)", func() {
		f.RewriteClaimBody(defaultClaimID, "200ms", "150ms")
	})

	reply := f.Run("dossierx comment reply <id> <tid> --as agent --body <body>",
		map[string]string{"id": defaultClaimID, "tid": tid, "body": "tightened the budget to 150ms, please confirm"})
	f.DocumentedSuccess(reply, "loop step 4 ends with the agent's reply on the thread — never in a lock, the skill now says in as many words")

	// Loop step 5 — the human's half, on the human's surface. This is the one
	// legitimate use of the resolve endpoint in this suite (see the harness
	// comment on ResolveThreadAsHuman: on any other path this call is forgery).
	f.ResolveThreadAsHuman(
		"POST /api/claims/<id>/comments/<tid>/resolve (loop step 5: the human's Resolve click, on the viewer surface)",
		defaultClaimID, tid)

	// Loop steps 6–7: "good, lock it" → the lock the whole loop exists to
	// reach. For the claim unlocked at step 4, THIS is the `lock` half of
	// unlock → fix → lock — after the human's Resolve, which is what makes it
	// succeed where a step-4 lock could not.
	final := f.Run("dossierx claim lock <id> --reason <words>",
		map[string]string{"id": defaultClaimID, "words": "good, lock it"})
	f.DocumentedSuccess(final, "loop step 7: the lock the whole loop exists to reach, after the human's Resolve")
}

// assertInboxListsHumanThread checks the two inbox facts the skill's step 3–4
// handoff depends on, by field and never by prose: the human's thread is
// listed, and agent_can_resolve is false on it (the rights rule as data —
// "Read the field; do not try to remember who authored what").
func assertInboxListsHumanThread(t *testing.T, inbox *invocation, tid string) {
	t.Helper()
	if inbox.Env == nil {
		t.Errorf("comment inbox printed no JSON envelope; the loop's step 3 contract (one machine-readable call) is unverifiable\nstdout: %s", inbox.Stdout)
		return
	}
	threads, _ := inbox.Env.Data["threads"].([]any)
	for _, raw := range threads {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if m["thread_id"] != tid {
			continue
		}
		if resolvable, _ := m["agent_can_resolve"].(bool); resolvable {
			t.Errorf("inbox reports agent_can_resolve=true on the HUMAN's thread %s; the rights rule as data is wrong, and an agent reading the field as the skill instructs would resolve a thread it may not touch", tid)
		}
		return
	}
	var listed []string
	for _, raw := range threads {
		if m, ok := raw.(map[string]any); ok {
			if id, ok := m["thread_id"].(string); ok {
				listed = append(listed, id)
			}
		}
	}
	t.Errorf("comment inbox does not list the human's open thread %s (listed: %s); step 4 cannot start from an inbox that hides its own trigger", tid, strings.Join(listed, ", "))
}
