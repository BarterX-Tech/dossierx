// adoption_warnings_cli_test.go covers the CLI half of the comment-digest
// laundering defect.
//
// internal/lock closed the engine half: SweepCommentDigests no longer adopts
// into a covered project whose digest store was deleted, and no longer adopts a
// claim holding a standing approval. What it still does — and must, or the sweep
// has no purpose — is adopt a claim it has genuinely never seen. That adoption
// records "these were the threads on disk just now", never "somebody reviewed
// them", and it was reaching NOBODY: lock.PrepareStore left the ids on the store
// (Store.CommentDigestsAdopted) and cmd/dossierx dropped them, so `dossierx
// check` printed ok:true, zero findings, exit 0 on the very run that took a
// hand-edited comment block as truth.
//
// The grandfathered ids had been threaded to every envelope for exactly this
// reason a release earlier. These are the same argument, and they now travel the
// same channel — see the adoptions struct.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adoptableCommentFixture builds a project that is already COVERED — a locked
// claim, so armLedgerFixture writes both the lock ledger and the comment digest
// store — and then adds a DRAFT claim, with NO comment block, that the digest
// store has never seen.
//
// Every part of that is load-bearing. The covered half is what makes the adoption
// notable rather than bootstrapping: on a project whose digest store this run
// CREATES, every block is adopted by definition and prepareStore stays silent
// (see its comment). The draft half is what keeps the claim adoptable at all,
// since internal/lock's sweep skips any id holding a standing approval. And the
// claim carries NO THREADS because the sweep now skips those too — a claim with
// threads and no entry is lock.RuleCommentDigestUnrecorded, a finding, precisely
// so that deleting one key from the digests map cannot be undone by the next
// ordinary command.
//
// What is left is the one adoption path that survives every guard, and it is
// still worth reporting: the claim is recorded at its EMPTY digest, which is the
// value that makes a thread hand-added to it afterwards report as drift rather
// than as an unknown.
func adoptableCommentFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Step one: a project that is genuinely covered. The locked claim arms the
	// lock ledger, and its own comment thread is what makes armLedgerFixture
	// create the digest store (it skips a project with no comments anywhere).
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/locked.yaml": "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  a locked claim, present so the project is ledger- and digest-covered.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-11aa22\n    status: resolved\n    author: human\n" +
			"    created: \"2026-07-26T09:00:00Z\"\n    body: looks right.\n    edited: false\n" +
			"    resolved_by: human\n    resolved_at: \"2026-07-26T09:30:00Z\"\n",
	})

	// Step two, AFTER coverage exists: the claim the digest store has never
	// seen. It has to be written afterwards — the fixture helper adopts every
	// claim it can see when it arms the store, so a claim present at that moment
	// would already have an entry and there would be nothing left to adopt.
	claim := "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft claim the comment digest store has never seen.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"rests_on:\n  - widget.contract.main\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "a.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatalf("write the late claim: %v", err)
	}
	return cfgPath
}

// TestCheckAnnouncesAdoptedCommentDigests.
//
// The run that adopts must not be able to report ok:true and nothing else. This
// pins both surfaces the machine contract offers — the warnings a reader of
// ok/warnings sees, and the data field a consumer acts on — because the whole
// defect was that a run with a real consequence was indistinguishable from a
// clean one.
func TestCheckAnnouncesAdoptedCommentDigests(t *testing.T) {
	cfgPath := adoptableCommentFixture(t)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check must still succeed on this project: %v", err)
	}
	if !env.OK {
		t.Fatalf("fixture precondition: this project has no findings, got %+v", env)
	}

	var data checkData
	envData(t, env, &data)
	if len(data.CommentDigestsAdopted) == 0 {
		t.Fatalf("the run that adopted a comment digest must name the claim in data.comment_digests_adopted, got %+v", data)
	}
	found := false
	for _, id := range data.CommentDigestsAdopted {
		if id == "widget.contract.a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected widget.contract.a among the adopted ids, got %v", data.CommentDigestsAdopted)
	}

	// The warnings half. An agent that reads only ok and warnings — which the
	// router skill permits — has to learn that this run recorded content nobody
	// approved, and has to be told the recovery, which is NOT the re-lock the
	// grandfathering sentence offers: no verb in this binary clears a recorded
	// comment digest.
	joined := strings.Join(env.Warnings, "\n")
	if !strings.Contains(joined, "widget.contract.a") {
		t.Fatalf("the adopted claim must be named in the envelope warnings, got %v", env.Warnings)
	}
	if !strings.Contains(joined, "ADOPTED") {
		t.Fatalf("the warning must say the block was adopted, got %v", env.Warnings)
	}
	if !strings.Contains(joined, "NOT a block anyone reviewed or approved") {
		t.Fatalf("the warning must say what adoption did NOT establish, got %v", env.Warnings)
	}
	if !strings.Contains(joined, "version control") {
		t.Fatalf("the warning must name the only recovery there is, got %v", env.Warnings)
	}
}

// TestCheckIsSilentAboutAdoptionWhenNothingWasAdopted.
//
// The guard-rail on the test above: a warning that fires on the ordinary run is
// noise, and noise on every run is how a real warning stops being read. The
// second `check` over the same project must adopt nothing — the digests are
// recorded now — and must say nothing about adoption.
func TestCheckIsSilentAboutAdoptionWhenNothingWasAdopted(t *testing.T) {
	cfgPath := adoptableCommentFixture(t)

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("first check: %v", err)
	}
	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("second check: %v", err)
	}

	var data checkData
	envData(t, env, &data)
	if len(data.CommentDigestsAdopted) != 0 {
		t.Fatalf("the second run adopts nothing and must report nothing, got %v", data.CommentDigestsAdopted)
	}
	for _, w := range env.Warnings {
		if strings.Contains(w, "comment digest") {
			t.Fatalf("a run that adopted nothing must not warn about adoption, got %v", env.Warnings)
		}
	}
}

// TestFirstCheckOfANewProjectDoesNotWarnAboutAdoption.
//
// The other guard-rail, and the one that shaped the rule. On a project whose
// digest store does not exist yet, PrepareStore CREATES it and adopts every
// comment block in the project — necessarily, since there is nothing to compare
// against. Reporting that as "content nobody approved" put a paragraph of
// integrity language on the first `check` of every new project, which is a
// warning firing on correct state: the fastest way to train a reader to skip the
// one that is real. Silence here is what buys the sentence in the test above its
// meaning.
func TestFirstCheckOfANewProjectDoesNotWarnAboutAdoption(t *testing.T) {
	root := t.TempDir()
	// Deliberately NOT writeCheckFixture: no locked claim, so nothing arms the
	// ledger or the digest store, and this run is the one that creates it.
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(parityConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	claim := "id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a brand new claim that already carries a thread.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"comments:\n" +
		"  - id: c-8136dd\n    status: open\n    author: human\n" +
		"    created: \"2026-07-26T10:00:00Z\"\n    body: is this true?\n    edited: false\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "a.yaml"), []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	var data checkData
	envData(t, env, &data)
	if len(data.CommentDigestsAdopted) != 0 {
		t.Fatalf("creating the digest store is not an adoption worth reporting, got %v", data.CommentDigestsAdopted)
	}
	for _, w := range env.Warnings {
		if strings.Contains(w, "comment digest") {
			t.Fatalf("the first check of a new project must not warn about adoption, got %v", env.Warnings)
		}
	}
}
