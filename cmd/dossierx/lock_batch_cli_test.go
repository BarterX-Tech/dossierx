// lock_batch_cli_test.go covers the batch form of "dossierx claim lock" —
// runBatchLock in lock_batch.go. See that file's header comment for the
// deadlock class it exists to break: the single-claim lock gate lints the
// WHOLE project with only the one claim being locked flipped, so every
// error-severity finding ANYWHERE in the corpus (related to that claim or
// not) refuses it. That is invisible on a clean project and becomes a
// hostage-taking deadlock the moment a module is unlocked in bulk.
package main

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// ---------------------------------------------------------------------
// batch succeeds where the same locks serially deadlock
// ---------------------------------------------------------------------

// batchDeadlockFixture reproduces the hostage shape directly: ONE pre-existing
// LOCKED claim (widget.contract.anchor) rests_on a claim that is DRAFT
// (widget.contract.sibling) — a state an unlock of "sibling" alone (a
// perfectly ordinary, permitted edit) produces. Two further claims, "one" and
// "two", carry no rests_on edges at all and have nothing to do with anchor or
// sibling.
//
// Under the single-claim, project-wide lint gate, "anchor rests_on sibling
// (draft)" is an error-severity finding that exists in the corpus REGARDLESS
// of which claim is being locked, so evaluateLockGates' unscoped count
// refuses locking "one" and refuses locking "two" — neither is related to
// anchor or sibling at all. There is no order that escapes it serially: every
// single lock sees the same whole-corpus finding. The batch path scopes the
// finding to the requested set (see findingBlocksBatch) and drops it, because
// neither "one" nor "two" is anchor or sibling and the message names neither.
func batchDeadlockFixture(t *testing.T) string {
	t.Helper()
	return writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/anchor.yaml": "id: widget.contract.anchor\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"body: |\n  the anchor, locked, resting on a claim that has since gone draft.\n" +
			"rests_on:\n  - widget.contract.sibling\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/sibling.yaml": "id: widget.contract.sibling\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  the sibling, unlocked back to draft for an ordinary edit.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/one.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  unrelated draft claim one.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/two.yaml": "id: widget.contract.two\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  unrelated draft claim two.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
}

func TestLocalApprovalDoesNotLetUnrelatedLintBlockACandidate(t *testing.T) {
	cfgPath := batchDeadlockFixture(t)

	// Local-approval v1 evaluates the candidate rather than making an
	// unrelated historical rests_on finding hostage the request.
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.one", "--reason", "go")
	if err != nil || !env.OK {
		t.Fatalf("single locally admissible claim must not be refused by unrelated historical lint: %+v (err=%v)", env, err)
	}

	// A set of one takes the same policy path after the first independent
	// approval, retaining the public group result shape for policy writes.
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock",
		"widget.contract.two", "--reason", "batch relock, unrelated to the anchor/sibling finding")
	if err != nil {
		t.Fatalf("batch lock over unrelated claims must succeed: %+v (err=%v)", env, err)
	}
	if !env.OK {
		t.Fatalf("expected ok envelope, got %+v", env)
	}

	var data policyLockData
	envData(t, env, &data)
	if len(data.ClaimIDs) != 1 || data.ClaimIDs[0] != "widget.contract.two" || data.To != "locked" {
		t.Fatalf("expected policy lock result for requested claim, got %+v", data)
	}

	// And it is durable: re-reading the claims off disk shows both locked.
	for _, id := range []string{"widget.contract.one", "widget.contract.two"} {
		showEnv, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", id)
		if err != nil {
			t.Fatalf("claim show %s: %v", id, err)
		}
		var show struct {
			Status string `json:"status"`
		}
		envData(t, showEnv, &show)
		if show.Status != "locked" {
			t.Fatalf("%s: expected status locked on disk, got %q", id, show.Status)
		}
	}
}

// ---------------------------------------------------------------------
// per-claim gates still refuse the WHOLE batch, and write nothing
// ---------------------------------------------------------------------

// batchOpenThreadFixture is two draft claims with no rests_on edges (so
// neither trips the lint gate) where one, "flagged", carries an open comment
// thread — the one per-claim gate that has no automated recovery at all.
func batchOpenThreadFixture(t *testing.T) string {
	t.Helper()
	return writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/clean.yaml": "id: widget.contract.clean\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim with nothing standing in its way.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/flagged.yaml": "id: widget.contract.flagged\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim carrying an unresolved comment thread.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-open001\n    status: open\n    author: human\n    created: \"2026-01-01T00:00:00Z\"\n    body: an open question.\n    edited: false\n",
	})
}

func TestBatchLockRefusedWhenOneMemberHasAnOpenThread_WritesNothing(t *testing.T) {
	cfgPath := batchOpenThreadFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock",
		"widget.contract.clean", "widget.contract.flagged", "--reason", "go")
	if err == nil || env.OK {
		t.Fatalf("a batch with an open-thread member must be refused, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUnresolvedComments {
		t.Fatalf("expected %q, got %+v", cliout.CodeUnresolvedComments, env.Error)
	}

	var data lockRefusedData
	envData(t, env, &data)
	if data.ClaimID != "widget.contract.flagged" || data.Gate != string(cliout.CodeUnresolvedComments) {
		t.Fatalf("the policy refusal must name the open-thread member and gate, got %+v", data)
	}

	// NOTHING WAS WRITTEN — not even the clean claim, which on its own has no
	// gate standing in its way at all. Atomicity: all requested ids lock, or
	// none do.
	showEnv, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.clean")
	if err != nil {
		t.Fatalf("claim show widget.contract.clean: %v", err)
	}
	var show struct {
		Status string `json:"status"`
	}
	envData(t, showEnv, &show)
	if show.Status != "draft" {
		t.Fatalf("the clean claim must remain untouched by the refused batch, got status %q", show.Status)
	}
}

// ---------------------------------------------------------------------
// a requested claim resting on a readable draft outside the requested set can
// be approved locally, with its dependency condition retained for readiness.
// ---------------------------------------------------------------------

// batchOutsideDraftDependencyFixture: "dependent" rests_on "outsider", and
// only "dependent" (plus an unrelated "bystander") is requested. "outsider"
// stays draft and is never part of the batch. Policy v1 admits that readable
// draft edge locally but carries it as a dependency-readiness condition.
func batchOutsideDraftDependencyFixture(t *testing.T) string {
	t.Helper()
	return writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/outsider.yaml": "id: widget.contract.outsider\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim that is never requested by the batch.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/dependent.yaml": "id: widget.contract.dependent\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a requested claim that rests on the outsider, still draft.\n" +
			"rests_on:\n  - widget.contract.outsider\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/bystander.yaml": "id: widget.contract.bystander\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a second requested claim with no edges of its own.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
}

func TestBatchLockLocallyApprovesWhenRequestedClaimRestsOnDraftOutsideTheBatch(t *testing.T) {
	cfgPath := batchOutsideDraftDependencyFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock",
		"widget.contract.dependent", "widget.contract.bystander", "--reason", "go")
	if err != nil || !env.OK {
		t.Fatalf("readable draft dependency is locally approvable, got %+v (%v)", env, err)
	}
	var data policyLockData
	envData(t, env, &data)
	if len(data.Evaluation.Verdicts) != 2 || len(data.Evaluation.Verdicts[0].Conditions)+len(data.Evaluation.Verdicts[1].Conditions) == 0 {
		t.Fatalf("local approval must disclose the draft dependency condition, got %+v", data.Evaluation)
	}

	for id, want := range map[string]string{
		"widget.contract.dependent": "locked",
		"widget.contract.bystander": "locked",
		"widget.contract.outsider":  "draft",
	} {
		showEnv, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", id)
		if err != nil {
			t.Fatalf("claim show %s: %v", id, err)
		}
		var show struct {
			Status    string `json:"status"`
			Readiness struct {
				DependencyReady      bool `json:"dependency_ready"`
				DependencyConditions []struct {
					DependencyID string `json:"dependency_id"`
					Kind         string `json:"kind"`
				} `json:"dependency_conditions"`
			} `json:"readiness"`
		}
		envData(t, showEnv, &show)
		if show.Status != want {
			t.Fatalf("%s: status = %q, want %q", id, show.Status, want)
		}
		if id == "widget.contract.dependent" {
			if show.Readiness.DependencyReady {
				t.Fatalf("dependent must not be dependency-ready while outsider remains draft: %+v", show.Readiness)
			}
			conditioned := false
			for _, condition := range show.Readiness.DependencyConditions {
				if condition.DependencyID == "widget.contract.outsider" && condition.Kind == "dependency_unapproved" {
					conditioned = true
				}
			}
			if !conditioned {
				t.Fatalf("dependent readiness must name the readable draft outsider: %+v", show.Readiness)
			}
		}
	}
}

// ---------------------------------------------------------------------
// single-id behavior is untouched
// ---------------------------------------------------------------------

// TestSingleIDLockStillProducesLockDataNotBatchShape pins that
// "dossierx claim lock <id>" with exactly one id still goes through the
// original single-lock code path (unmodified, only moved a few lines later
// in newLockCmd's RunE) and still reports lockData, not the new batch shape —
// the batch branch in newLockCmd is only ever taken for len(args) > 1.
func TestSingleIDLockStillProducesLockDataNotBatchShape(t *testing.T) {
	cfgPath := batchOpenThreadFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.clean", "--reason", "go")
	if err != nil {
		t.Fatalf("single-id lock: %v (%+v)", err, env)
	}
	var data lockData
	envData(t, env, &data)
	if data.ClaimID != "widget.contract.clean" || data.To != "locked" {
		t.Fatalf("expected the single-lock lockData shape, got %+v", data)
	}
}

// TestPolicyLockDryRunPreviewsMultipleIDs pins the policy-v1 set preview. It
// reports every requested member and mints a token bound to this exact request,
// while leaving the fixture untouched.
func TestPolicyLockDryRunPreviewsMultipleIDs(t *testing.T) {
	cfgPath := batchOpenThreadFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock",
		"widget.contract.clean", "widget.contract.flagged", "--dry-run")
	if err != nil || !env.OK {
		t.Fatalf("--dry-run with more than one id must produce the shared set preview, got %+v (%v)", env, err)
	}
	var data policyLockPreviewData
	envData(t, env, &data)
	if !strings.HasPrefix(data.Snapshot, "v2:") {
		t.Fatalf("group preview must issue a v2 request-bound proposal, got %q", data.Snapshot)
	}
	if len(data.Evaluation.Verdicts) != 2 {
		t.Fatalf("group preview must return one verdict per requested id, got %+v", data.Evaluation)
	}
	flagged := false
	for _, verdict := range data.Evaluation.Verdicts {
		if verdict.ClaimID == "widget.contract.flagged" && len(verdict.Refusals) > 0 {
			flagged = true
		}
	}
	if !flagged {
		t.Fatalf("group preview must expose the flagged member refusal, got %+v", data.Evaluation)
	}
}
