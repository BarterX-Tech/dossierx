// skill_bootstrap_replay_test.go replays step 4 of the router skill's
// bootstrap (skills/dossierx/SKILL.md, "Bootstrap — setting DossierX up in a
// repo") — the pre-commit-hook decision point — once per answer, with the same
// text-driven interpreter that replays README's paste block. The two documents
// give the same procedure to two audiences (README to the human who pastes it,
// the skill to the agent that loads it), and the CI postcondition is the same
// sentence in both: "CI is the authority either way."
//
// THE DEFECT THIS EXISTS TO CATCH, found by a gate run after the identical
// defect had been fixed in README's block alone: the skill's step 4 fetched
// scripts/ci/dossierx-check.yml only on the branch where the human DECLINED
// the hook ("If no, add the CI workflow instead"), so the nudged answer — yes
// — ended the bootstrap with only the local gate git skips on merges, rebases,
// cherry-picks and reverts, and that --no-verify bypasses. The README replay
// could not see it because it reads README; this file points the same
// interpreter at the skill, so the two documents' step 4 are now both
// executed, not merely both written.
//
// WHAT IS REPLAYED AND WHAT IS ENACTED. Steps 1–3 of the bootstrap are enacted
// as state, not replayed as text: the binary exists (TestMain built it from
// this tree), the config the human confirmed is written, and the skills are
// exported — the export-order half of the bootstrap is
// tests/procedures/bootstrap_test.go's subject, not this file's. Step 4 IS
// replayed from the text: each branch's fetches, `sh ...` invocations and
// CI-workflow instructions are read out of the skill and executed in the order
// the sentence gives them, with the network substitutions the README replay
// documents (the raw URL is served from this tree; "add the CI workflow" is
// realized from scripts/ci/dossierx-check.yml). Steps 5–8 are conditional or
// addressed to the human; step 6's `dossierx check` is run as the transcript's
// own sanity gate.
package tests

import (
	"path/filepath"
	"strings"
	"testing"
)

// skillBootstrapStep4 returns step 4 of the router skill's bootstrap sequence.
// Same lookup discipline as readmePasteBlock: a missing heading or a missing
// step means the replay has lost its subject, which is fatal, never a smaller
// test.
func skillBootstrapStep4(t *testing.T) string {
	t.Helper()
	skill := readRepoFile(t, filepath.Join("skills", "dossierx", "SKILL.md"))

	const anchor = "\n## Bootstrap"
	i := strings.Index(skill, anchor)
	if i < 0 {
		t.Fatal(`skills/dossierx/SKILL.md no longer carries a "## Bootstrap" heading; the sequence this replay executes has moved or gone, and the replay must be repointed at whatever replaced it`)
	}
	section := skill[i+1:]
	if j := strings.Index(section, "\n## "); j >= 0 {
		section = section[:j]
	}
	return pasteBlockStep(t, section, 4)
}

// requireSkillCIPostconditionStated fails fatally if the skill no longer
// states the requirement the tests below enforce — the same discipline as
// requireCIPostconditionStated for README. The skill states it twice: in step
// 4 itself ("CI is the authority either way") and in its staged-gate section
// ("branch protection plus a required CI check is what makes anyone obey it,
// and is the answer when a human asks you to set integrity up").
func requireSkillCIPostconditionStated(t *testing.T) {
	t.Helper()
	skill := readRepoFile(t, filepath.Join("skills", "dossierx", "SKILL.md"))
	const inStep = "CI is the authority"
	const inGateSection = "branch protection plus a required CI check"
	if !strings.Contains(skill, inStep) && !strings.Contains(skill, inGateSection) {
		t.Fatalf("skills/dossierx/SKILL.md no longer states that CI is the authority (%q / %q are both gone), so the postcondition this replay asserts has lost its written source; re-derive the requirement before trusting either branch's result", inStep, inGateSection)
	}
}

// replaySkillBootstrapStep4 puts a fixture consumer repo into the state the
// bootstrap's steps 1–3 leave behind, replays step 4's chosen branch from the
// skill's own text, and runs step 6's check as the transcript's sanity gate.
// It returns the consumer root for postcondition assertions.
func replaySkillBootstrapStep4(t *testing.T, module string, sayYes bool) string {
	t.Helper()
	step4 := skillBootstrapStep4(t)

	consumer := t.TempDir()
	gitInConsumer(t, consumer, "init")
	gitInConsumer(t, consumer, "config", "user.email", "replay@example.invalid")
	gitInConsumer(t, consumer, "config", "user.name", "Skill Bootstrap Replay")

	// Steps 1–3, enacted: the binary answers, the config the human confirmed
	// exists, the skills are exported rooted. (The ORDER of steps 2 and 3 is
	// bootstrap_test.go's subject; here it is scaffolding for step 4.)
	if _, stderr, code := run(t, consumer, "version"); code != 0 {
		t.Fatalf("bootstrap step 1's `dossierx version` exited %d: %s", code, stderr)
	}
	writeFixtureProject(t, consumer, module)
	if _, stderr, code := run(t, consumer, "skills", "export", ".claude/skills"); code != 0 {
		t.Fatalf("bootstrap step 3's `dossierx skills export .claude/skills` exited %d: %s", code, stderr)
	}

	// Step 4 — the decision point, located by its own phrasing exactly as the
	// README replay locates its twin.
	yesIdx := strings.Index(step4, "If yes")
	noIdx := strings.Index(step4, "If no")
	if yesIdx < 0 || noIdx < 0 || noIdx < yesIdx {
		t.Fatalf("step 4 of the skill's bootstrap no longer reads as an \"If yes … If no …\" decision; the replay cannot tell the branches apart:\n%s", step4)
	}
	if sayYes {
		replayDecisionBranch(t, consumer, step4[yesIdx:noIdx])
	} else {
		replayDecisionBranch(t, consumer, step4[noIdx:])
	}

	// Step 6 — `dossierx check --format text`, exiting 0 (run() passes --format
	// text). Steps 5, 7 and 8 are conditional or addressed to the human.
	if stdout, stderr, code := run(t, consumer, "check"); code != 0 {
		t.Fatalf("bootstrap step 6's `dossierx check` exited %d over the project the sequence produced:\n%s%s", code, stdout, stderr)
	}
	return consumer
}

// TestSkillBootstrapYesToHookStillEndsWithCI replays the accepting transcript —
// the nudged answer, since the hook is what step 4 is ABOUT, and the branch
// where the defect lived: the skill's old step 4 fetched the CI workflow only
// inside "If no", so saying yes ended the bootstrap with the gate git skips
// and nothing behind it.
func TestSkillBootstrapYesToHookStillEndsWithCI(t *testing.T) {
	requireSkillCIPostconditionStated(t)
	consumer := replaySkillBootstrapStep4(t, "skillyes", true)

	// The branch's own promise must hold before the shared postcondition is
	// judged, or a broken installer would masquerade as the CI defect.
	if !hookInstalled(t, consumer) {
		t.Fatal("the yes branch ran to completion and the pre-commit hook is not installed; the replay of `sh install-git-hook.sh --yes` did not do what the skill says it does, so nothing below this can be trusted")
	}
	assertCIWorkflowArrived(t, consumer, "the router skill's bootstrap", "yes")
}

// TestSkillBootstrapNoToHookEndsWithCI replays the declining transcript — the
// branch the skill always handled — as the control for the yes branch and as
// its own regression pin against the CI instruction leaving step 4 entirely.
func TestSkillBootstrapNoToHookEndsWithCI(t *testing.T) {
	requireSkillCIPostconditionStated(t)
	consumer := replaySkillBootstrapStep4(t, "skillno", false)

	if hookInstalled(t, consumer) {
		t.Error("the human answered no to the hook question and the transcript installed the pre-commit hook anyway")
	}
	assertCIWorkflowArrived(t, consumer, "the router skill's bootstrap", "no")
}
