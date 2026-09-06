// lock_gate_cli_test.go covers the two lock-gate defects the round-5 merge gate
// reproduced: a lint refusal that hid its findings under error.details only
// (making the router's documented recovery an infinite loop), and the roll-up
// rule, which as a project-wide error deadlocked any module holding a locked
// banner alongside more than one draft.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// ---------------------------------------------------------------------
// the lint refusal publishes its findings where the router says to look
// ---------------------------------------------------------------------

// The router skill's lint_failed row reads: "read data.lint_findings, fix the
// claims, re-run". `claim lock` published them ONLY under
// error.details.lint_findings, so an agent following the documented recovery
// literally found no such key, learned nothing, and re-ran the same command —
// the loop. Every other refusal in the CLI carries its payload in data; this was
// the exception, and nothing about lock justified being one.
func TestLockLintRefusalCarriesFindingsInTopLevelData(t *testing.T) {
	cfgPath := buildRoleAdoptedFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.b", "--reason", "go")
	if err == nil || env.OK {
		t.Fatalf("fixture precondition: this lock must be refused, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("expected %q, got %+v", cliout.CodeLintFailed, env.Error)
	}
	if env.Data == nil {
		t.Fatalf("a refused lock must still carry a top-level data payload: %+v", env)
	}

	var data lockRefusedData
	envData(t, env, &data)
	if data.ClaimID != "widget.contract.b" {
		t.Fatalf("the refusal payload must name the claim, got %+v", data)
	}
	if data.Gate != string(cliout.CodeLintFailed) {
		t.Fatalf("the payload must name the gate that refused, got %q", data.Gate)
	}
	if len(data.LintFindings) == 0 {
		t.Fatalf("data.lint_findings is the key the router tells agents to read: %+v", data)
	}
	named := false
	for _, f := range data.LintFindings {
		if f.Lint == "build-role-required-for-locked" && f.ClaimID == "widget.contract.b" {
			named = true
		}
		if f.Message == "" || f.Severity == "" {
			t.Fatalf("a finding must carry its message and severity: %+v", f)
		}
	}
	if !named {
		t.Fatalf("data.lint_findings must name the rule and the claim, got %+v", data.LintFindings)
	}

	// error.details keeps its copy: an agent written against the old shape is
	// not broken by the fix.
	details, ok := env.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("error.details must survive for compatibility, got %#v", env.Error.Details)
	}
	if _, present := details["lint_findings"]; !present {
		t.Fatalf("error.details.lint_findings must still be published: %#v", details)
	}
}

// internal/lock's refusals are written to be read on their own and already
// begin with "lock: "; the CLI added a second one. In a machine contract a
// duplicated prefix is worse than untidy — it is the sort of thing a consumer
// writes a TrimPrefix against and then breaks on.
func TestLockRefusalMessageDoesNotDoubleItsVerb(t *testing.T) {
	cfgPath := buildRoleAdoptedFixture(t)

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.b", "--reason", "go")
	if err == nil {
		t.Fatalf("fixture precondition: this lock must be refused")
	}
	if env.Error == nil {
		t.Fatalf("expected an error envelope, got %+v", env)
	}
	if strings.Contains(env.Error.Message, "lock: lock:") {
		t.Fatalf("the refusal must not carry a doubled verb, got %q", env.Error.Message)
	}
	if !strings.HasPrefix(env.Error.Message, "lock: ") {
		t.Fatalf("the refusal must still name the verb once, got %q", env.Error.Message)
	}
}

// ---------------------------------------------------------------------
// the roll-up deadlock
// ---------------------------------------------------------------------

// rollUpDeadlockFixture is the shape that had no legal move: one LOCKED banner
// claim (an orientation note, the ordinary way a banner exists) and TWO draft
// claims in the same module.
//
// With roll-up as a project-wide error this failed `check`, `check --validate`,
// `check --staged`, the pre-commit hook and CI at once, AND refused `claim lock`
// on both drafts — locking either leaves the other draft, so the banner's
// finding survives into the about-to-be-locked lint and the gate refuses. The
// only escape was unlocking the banner, which is a human-approved action an
// agent may not invent a reason for.
func rollUpDeadlockFixture(t *testing.T, bannerStatus string) string {
	t.Helper()
	return writeCheckFixture(t, t.TempDir(), parityConfig, map[string]string{
		"claims/banner.yaml": "id: widget.overview.router\nfacet: overview\nmodule: widget\nstatus: " + bannerStatus + "\nlayout: banner\n" +
			"build_role: orientation\n" +
			"body: |\n  read the contract claims below in order.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/one.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  the first draft claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/two.yaml": "id: widget.contract.two\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: behavior\n" +
			"body: |\n  the second draft claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
}

// TestRollUpDoesNotDeadlockAModule: every gate in the product has a legal move
// available in the fixture shape above.
func TestRollUpDoesNotDeadlockAModule(t *testing.T) {
	cfgPath := rollUpDeadlockFixture(t, "locked")

	// The read-only gates pass: a locked banner whose module gained a draft is
	// reached by editing DRAFT claims, which this release deliberately does not
	// gate. It is reported, not refused.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate"); err != nil {
		t.Fatalf("check --validate must not fail on a locked banner with draft siblings: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check must not fail on a locked banner with draft siblings: %v", err)
	}

	// And it IS reported — demoting the rule must not silence it.
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil {
		t.Fatalf("check --validate: %v", err)
	}
	if !strings.Contains(strings.Join(env.Warnings, "\n"), "roll-up") {
		t.Fatalf("the roll-up condition must still be reported as a warning, got %#v", env.Warnings)
	}

	// The legal move exists: a draft sibling locks. That is the whole fix —
	// under the old rule this was refused, and so was the other one.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.one", "--reason", "approved on the call"); err != nil {
		t.Fatalf("locking a draft sibling must be legal: %v", err)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.two", "--reason", "approved on the call"); err != nil {
		t.Fatalf("locking the last draft sibling must be legal: %v", err)
	}
}

// TestRollUpStillRefusesTheBannersOwnLock: the promise the rule exists for
// survives the demotion. A banner is a module-wide "reviewed" callout, so
// locking one while its module still holds a draft is exactly the
// misrepresentation the rule is about — and it is still refused, with the same
// code and the same payload shape as any other lint refusal.
func TestRollUpStillRefusesTheBannersOwnLock(t *testing.T) {
	// A DRAFT banner from the start: this test locks it, and a fixture that
	// flipped an already-locked one to draft would leave a standing ledger
	// approval behind — a lock-ledger-orphan, which is a more serious finding and
	// would (correctly) refuse first, testing the wrong gate.
	cfgPath := rollUpDeadlockFixture(t, "draft")

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.overview.router", "--reason", "go")
	if err == nil || env.OK {
		t.Fatalf("a banner must not lock while its module holds a draft, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("the banner refusal must classify as %q, got %+v", cliout.CodeLintFailed, env.Error)
	}

	var data lockRefusedData
	envData(t, env, &data)
	if len(data.LintFindings) == 0 {
		t.Fatalf("the banner refusal must carry data.lint_findings like every other lint refusal: %+v", data)
	}
	joined, err := json.Marshal(data.LintFindings)
	if err != nil {
		t.Fatalf("marshal findings: %v", err)
	}
	if !strings.Contains(string(joined), "widget.contract.one") {
		t.Fatalf("the refusal must name the sibling holding the banner open: %s", joined)
	}
}

// The refusal is only useful if it names the claim the caller has to ACT on, and
// for roll-up that is never the claim the finding hangs off: the finding is
// attached to the banner, and it is cleared by locking a sibling. Both surfaces
// an agent reads before acting — the dry run and `claim show` — have to carry
// both ids.
func TestRollUpBlockerNamesBothClaimsInThePreviewAndInShow(t *testing.T) {
	cfgPath := rollUpDeadlockFixture(t, "draft")

	dr := dryRunOf(t, "--config", cfgPath, "claim", "lock", "widget.overview.router")
	detail := ""
	for _, p := range dr.Preconditions {
		if p.Name == "lint_clean" {
			if p.OK {
				t.Fatalf("the preview must agree with the write path that roll-up blocks: %+v", p)
			}
			detail = p.Detail
		}
	}
	if !strings.Contains(detail, "roll-up") {
		t.Fatalf("the lint_clean detail must name the rule, got %q", detail)
	}
	if !strings.Contains(detail, "widget.overview.router") {
		t.Fatalf("the lint_clean detail must name the offending banner, got %q", detail)
	}
	if !strings.Contains(detail, "widget.contract.one") {
		t.Fatalf("the lint_clean detail must name the blocking sibling, got %q", detail)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.overview.router")
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	var show struct {
		NextActions []string `json:"next_actions"`
	}
	envData(t, env, &show)
	actions := strings.Join(show.NextActions, "\n")
	if !strings.Contains(actions, "roll-up") || !strings.Contains(actions, "widget.contract.one") {
		t.Fatalf("claim show must name the rule and the blocking sibling, got %v", show.NextActions)
	}
}
