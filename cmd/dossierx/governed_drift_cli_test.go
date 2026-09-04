// governed_drift_cli_test.go is the CLI half of #21: a claim-valued
// governed_by.type is a DRIFT dependency. Editing a governing doctrine claim
// has to move the locked claims it governs to review_pending, and every surface
// that reads review_pending has to agree about WHY.
//
// The bug was not one missing edge in one function. lock.dependencyIDs was
// hand-copied into internal/comments and into pickChangedDependency here, so
// widening only lock's copy would have shipped an engine that contradicts
// itself: check announcing "review_pending with no active trigger" over a claim
// whose file says review_pending: true, `claim show` reporting review_trigger
// none, any comment op silently erasing the flag through Recompute, and
// reaudit --confirm reporting `dependency "" changed`. All three copies now go
// through lock.BaselineDependencyIDs, and the tests below are written surface
// by surface so a future re-copy fails somewhere loud.
//
// What is deliberately NOT here: hub gating. governance is a drift edge, not a
// gating edge (internal/lint/governed_cycle.go and FORMAT.md both say so), and
// TestGovernedByIsNotAGatingEdge pins that a child naming an UNLOCKED
// doctrine-facet claim only through governed_by still locks.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// governedDriftConfig declares both facets and names the doctrine facet, so
// hub gating is ON for every test in this file — the only way the "governance
// does not gate" assertion means anything.
const governedDriftConfig = "schema_version: 1\nfacets:\n  - contract\n  - doctrine\nmodules:\n  - widget\nclaims_dir: claims\ndoctrine_facet: doctrine\n"

const (
	govHub        = "widget.doctrine.hub"
	govChild      = "widget.contract.child"
	govDownstream = "widget.contract.downstream"
	govTwoEdge    = "widget.contract.two-edge"
	govUngoverned = "widget.contract.ungoverned"
)

// governedDriftFixture writes a project whose claims exercise every edge shape
// the baseline set has to distinguish, and returns the config path plus the
// project root. Claims are written as DRAFTS and locked through the CLI, so
// every lock carries a real ledger approval and a real baseline — a hand-written
// "status: locked" would prove nothing about what Lock records.
//
// The ids follow the module.facet.slug grammar the id-shape lint enforces, so
// the plan's shorthand ("doctrine.hub", "child") appears here as
// widget.doctrine.hub, widget.contract.child, and so on.
func governedDriftFixture(t *testing.T) (cfgPath, projectDir string) {
	t.Helper()
	projectDir = t.TempDir()
	claimsDir := filepath.Join(projectDir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath = filepath.Join(projectDir, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(governedDriftConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	files := map[string]string{
		// The governor. Its facet is the configured doctrine facet, which is
		// what makes it a hub-gating candidate for anything that RESTS on it.
		"hub.yaml": "id: " + govHub + "\nfacet: doctrine\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: orientation\n" +
			"body: |\n  the governing doctrine, first wording.\n" +
			"governed_by:\n  type: none\n  reason: the hub is the doctrine\n",
		// governed_by is the ONLY edge to the hub: no mirrors, no rests_on.
		"child.yaml": "id: " + govChild + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  a claim the doctrine governs.\n" +
			"governed_by:\n  type: " + govHub + "\n  reason: this contract is backed by the hub doctrine\n",
		// One hop past the directly governed claim, for the staged-propagation
		// assertion: it must NOT be flagged in the same pass.
		"downstream.yaml": "id: " + govDownstream + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: behavior\n" +
			"rests_on:\n  - " + govChild + "\n" +
			"body: |\n  a claim resting on the governed claim.\n" +
			"governed_by:\n  type: none\n  reason: downstream of the governed claim\n",
		// The same target through two edge types, for the dedupe assertion.
		"two-edge.yaml": "id: " + govTwoEdge + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: behavior\n" +
			"rests_on:\n  - " + govHub + "\n" +
			"body: |\n  a claim reaching the hub through two edge types.\n" +
			"governed_by:\n  type: " + govHub + "\n  reason: rests on and is governed by the same hub\n",
		// The "none" sentinel, for the assertion that it never becomes a key.
		"ungoverned.yaml": "id: " + govUngoverned + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"build_role: behavior\n" +
			"rests_on:\n  - " + govChild + "\n" +
			"body: |\n  a claim with no doctrine backing at all.\n" +
			"governed_by:\n  type: none\n  reason: deliberately ungoverned\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(claimsDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return cfgPath, projectDir
}

// lockGovernedFixture locks every fixture claim through the CLI, in an order
// that satisfies rest-on-locked and hub gating.
func lockGovernedFixture(t *testing.T, cfgPath string) {
	t.Helper()
	for _, id := range []string{govHub, govChild, govTwoEdge, govDownstream, govUngoverned} {
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "the human approved "+id); err != nil {
			t.Fatalf("claim lock %s: %v", id, err)
		}
	}
}

// rewordGovernor takes the governing claim through the only sanctioned path for
// changing locked content — unlock, edit, re-lock — and returns nothing but the
// fact that the hub's comparable content is now different. Editing the file in
// place while it is locked would trip the lock-ledger content gate, which is a
// different test.
func rewordGovernor(t *testing.T, cfgPath, projectDir string) {
	t.Helper()
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "unlock", govHub, "--reason", "reword the doctrine"); err != nil {
		t.Fatalf("claim unlock %s: %v", govHub, err)
	}
	path := filepath.Join(projectDir, "claims", "hub.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hub: %v", err)
	}
	reworded := strings.Replace(string(body), "first wording", "second wording", 1)
	if reworded == string(body) {
		t.Fatalf("hub body did not change; the fixture wording drifted:\n%s", body)
	}
	if err := os.WriteFile(path, []byte(reworded), 0o644); err != nil {
		t.Fatalf("write hub: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", govHub, "--reason", "the human approved the reworded doctrine"); err != nil {
		t.Fatalf("re-lock %s: %v", govHub, err)
	}
}

// lockStoreOf decodes the project's lock store the way an operator's jq would.
func lockStoreOf(t *testing.T, projectDir string) struct {
	Hashes map[string]map[string]string `json:"hashes"`
} {
	t.Helper()
	var store struct {
		Hashes map[string]map[string]string `json:"hashes"`
	}
	raw, err := os.ReadFile(filepath.Join(projectDir, "build", "ledger", "lock-store.json"))
	if err != nil {
		t.Fatalf("read lock store: %v", err)
	}
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatalf("decode lock store: %v", err)
	}
	return store
}

// countReviewPendingTrue is `grep -c 'review_pending: true' <file>`. It reads
// the FILE rather than the payload deliberately: review_pending lives in the
// claim's YAML, and a surface that reports it correctly while the file says
// otherwise is the exact failure mode this whole file is about.
func countReviewPendingTrue(t *testing.T, projectDir, file string) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(projectDir, "claims", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return strings.Count(string(raw), "review_pending: true")
}

// TestGovernedByIsNotAGatingEdge is D-6 branch (a) at the CLI surface: hub
// gating is a lock REFUSAL and governance is not one of its edges. The child
// names an UNLOCKED doctrine-facet claim through governed_by.type only, and
// locks. The contrast case — the same unlocked hub named through rests_on — is
// asserted in the same test so the two can never drift apart.
func TestGovernedByIsNotAGatingEdge(t *testing.T) {
	cfgPath, _ := governedDriftFixture(t)

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", govChild, "--reason", "the doctrine hub is still draft and that is fine"); err != nil {
		t.Fatalf("a claim naming an unlocked doctrine claim ONLY through governed_by must still lock; got %v", err)
	}

	// The contrast, in the same test so the two can never drift apart: the same
	// still-unlocked hub named through rests_on is refused.
	//
	// The refusal that arrives first is rest-on-locked's lint gate, which runs
	// ahead of hub gating and shadows it whenever the dependency is a claim in
	// the project. That is fine for what this asserts — an ordinary edge to an
	// unlocked hub is blocked while a governance edge is not — and hub gating
	// itself is pinned precisely, with an empty lint registry, by
	// TestHubGatingIgnoresGovernedBy in internal/lock. What matters at this
	// surface is that the refusal is CLASSIFIED: lockGate.code() falls through
	// to CodeInternal ("file a bug") whenever a refusal fires that the dry-run
	// twin did not predict, which is exactly what widening dependencyIDs would
	// have produced.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", govTwoEdge, "--reason", "should be refused")
	if err == nil {
		t.Fatalf("rests_on an unlocked doctrine claim must still be refused")
	}
	if env.Error == nil {
		t.Fatalf("a refusal must carry an error envelope: %+v", env)
	}
	if env.Error.Code == cliout.CodeInternal {
		t.Fatalf("the refusal must be classified, never \"file a bug\": %+v", env.Error)
	}
}

// TestLockingRecordsAGovernanceBaseline is done-when 2: the governor's content
// hash becomes a per-dependent baseline, keyed under the governed claim.
func TestLockingRecordsAGovernanceBaseline(t *testing.T) {
	cfgPath, projectDir := governedDriftFixture(t)
	lockGovernedFixture(t, cfgPath)

	store := lockStoreOf(t, projectDir)
	if _, ok := store.Hashes[govChild][govHub]; !ok {
		t.Fatalf("hashes[%q][%q] missing: a claim-valued governed_by.type must be baselined; got %v", govChild, govHub, store.Hashes[govChild])
	}
}

// TestGovernedByNoneIsNeverABaselineKey is done-when 10: "none" is a sentinel,
// not a claim id, and a store row keyed "none" would compare forever against a
// claim that cannot exist.
func TestGovernedByNoneIsNeverABaselineKey(t *testing.T) {
	cfgPath, projectDir := governedDriftFixture(t)
	lockGovernedFixture(t, cfgPath)

	store := lockStoreOf(t, projectDir)
	if _, ok := store.Hashes[govUngoverned]["none"]; ok {
		t.Fatalf("governed_by.type: none must create no baseline; got %v", store.Hashes[govUngoverned])
	}
}

// TestTwoEdgeTargetIsOneBaselineEntry is done-when 12: rests_on X plus
// governed_by X is ONE dependency, recorded once, deterministically.
func TestTwoEdgeTargetIsOneBaselineEntry(t *testing.T) {
	cfgPath, projectDir := governedDriftFixture(t)
	lockGovernedFixture(t, cfgPath)

	first := lockStoreOf(t, projectDir).Hashes[govTwoEdge]
	if len(first) != 1 {
		t.Fatalf("a target reached through two edge types must produce exactly one baseline entry, got %d: %v", len(first), first)
	}
	if _, ok := first[govHub]; !ok {
		t.Fatalf("the single entry must be keyed by the hub, got %v", first)
	}

	// Determinism: a second full pipeline run over the same project must not
	// reorder or duplicate it.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v", err)
	}
	second := lockStoreOf(t, projectDir).Hashes[govTwoEdge]
	if len(second) != 1 || second[govHub] != first[govHub] {
		t.Fatalf("baseline entries must be stable across runs: %v then %v", first, second)
	}
}

// TestGovernorEditFlagsTheGovernedClaimEverywhere is the heart of #21: one
// governor reword, and every surface has to agree. It is written as one test
// rather than six because the failure this file exists for was surfaces
// DISAGREEING, and splitting them lets a partial fix look green.
func TestGovernorEditFlagsTheGovernedClaimEverywhere(t *testing.T) {
	cfgPath, projectDir := governedDriftFixture(t)
	lockGovernedFixture(t, cfgPath)
	rewordGovernor(t, cfgPath, projectDir)

	// done-when 3: check writes review_pending to the governed claim's FILE.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v", err)
	}
	if n := countReviewPendingTrue(t, projectDir, "child.yaml"); n != 1 {
		t.Fatalf("grep -c 'review_pending: true' claims/child.yaml = %d, want 1", n)
	}

	// done-when 11: propagation is STAGED. Flagging the directly governed claim
	// does not flag what rests on it.
	//
	// The plan's bullet says "assert the downstream claim's file still reads
	// review_pending: false"; the engine never writes that line, because
	// model.Claim tags ReviewPending `omitempty` — so the assertable form of
	// "still false" is the absence of the true line.
	if n := countReviewPendingTrue(t, projectDir, "downstream.yaml"); n != 0 {
		t.Fatalf("propagation must be staged: claims/downstream.yaml has %d review_pending: true line(s), want 0", n)
	}

	// done-when 5: claim show names the trigger. Never "none" — that is the
	// state the hand-copied dependency list produced, and it tells an agent
	// reaudit has nothing to offer.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", govChild)
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	var shown claimShowData
	envData(t, env, &shown)
	if !shown.ReviewPending {
		t.Fatalf("claim show must report review_pending for the governed claim: %+v", shown)
	}
	// The payload key is review_pending_trigger (the plan's bullet writes it as
	// review_trigger, which is the field's prose name, not its JSON key).
	if shown.Trigger != "drift" {
		t.Fatalf("review_pending_trigger = %q, want \"drift\"", shown.Trigger)
	}

	// done-when 6: check's next_steps routes it to reaudit as a drift/flag
	// trigger, never to the triggerless bucket.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	var checked checkData
	envData(t, env, &checked)
	steps := strings.Join(checked.NextSteps, "\n")
	if !strings.Contains(steps, "review_pending from drift/flag") {
		t.Fatalf("next_steps must route the governed claim to the drift/flag reaudit step, got:\n%s", steps)
	}
	if strings.Contains(steps, "review_pending with no active trigger") {
		t.Fatalf("next_steps must NOT report a triggerless review_pending; the governor edit IS the trigger:\n%s", steps)
	}
}

// TestCommentOpsDoNotEraseGovernanceDrift is done-when 7. Every comment op
// reassigns review_pending from comments.Recompute — a straight assignment, not
// an OR — so a Recompute that cannot see the governance edge silently erases a
// drift the lock store still knows about.
func TestCommentOpsDoNotEraseGovernanceDrift(t *testing.T) {
	cfgPath, projectDir := governedDriftFixture(t)
	lockGovernedFixture(t, cfgPath)
	rewordGovernor(t, cfgPath, projectDir)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v", err)
	}
	if n := countReviewPendingTrue(t, projectDir, "child.yaml"); n != 1 {
		t.Fatalf("precondition: the governed claim must be review_pending before the comment ops, got %d", n)
	}

	// A human opens a thread, the agent replies — both through the CLI.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "add", govChild,
		"--as", "human", "--body", "does the reworded doctrine still cover this?")
	if err != nil {
		t.Fatalf("comment add: %v", err)
	}
	var added commentWriteData
	envData(t, env, &added)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "comment", "reply", govChild, added.ThreadID,
		"--as", "agent", "--body", "it does; the change was editorial"); err != nil {
		t.Fatalf("comment reply: %v", err)
	}

	// The human resolves the last open thread. "comment resolve" is deliberately
	// not a CLI verb (resolving IS the human's approval and lives in the
	// viewer), so this goes through the same internal/comments op the viewer
	// calls, with the same Deps the CLI builds.
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	deps, err := mutatingCommentDeps(cfg)
	if err != nil {
		t.Fatalf("comment deps: %v", err)
	}
	if _, err := deps.Resolve(govChild, added.ThreadID, model.CommentRoleHuman); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if n := countReviewPendingTrue(t, projectDir, "child.yaml"); n != 1 {
		t.Fatalf("resolving the last open thread must NOT erase the governance drift: grep -c 'review_pending: true' claims/child.yaml = %d, want 1", n)
	}
}

// TestReauditNamesTheGovernorAndRefreshesItsBaseline is done-when 8 and 9. The
// inline mirrors ++ rests_on copy inside pickChangedDependency returned a zero
// model.Claim for a governance-only drift, so the proposal read
// `dependency "" changed but no proposal was generated` — a preview that names
// nothing, asking a human to approve nothing in particular.
func TestReauditNamesTheGovernorAndRefreshesItsBaseline(t *testing.T) {
	cfgPath, projectDir := governedDriftFixture(t)
	lockGovernedFixture(t, cfgPath)
	rewordGovernor(t, cfgPath, projectDir)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v", err)
	}

	dr := dryRunOf(t, "--config", cfgPath, "claim", "reaudit", govChild)
	satisfied := false
	for _, p := range dr.Preconditions {
		if p.Name == "has_a_content_trigger" {
			satisfied = p.OK
			if !strings.Contains(p.Detail, "drift=true") {
				t.Fatalf("has_a_content_trigger detail must report the drift: %q", p.Detail)
			}
		}
	}
	if !satisfied {
		t.Fatalf("has_a_content_trigger must be satisfied for a governance drift: %+v", dr.Preconditions)
	}
	note, ok := dr.Proposed["note"].(string)
	if !ok {
		t.Fatalf("the proposal's note must be a string, got %T: %+v", dr.Proposed["note"], dr.Proposed)
	}
	if !strings.Contains(note, govHub) {
		t.Fatalf("the proposal must name the GOVERNOR as the changed dependency, got %q", note)
	}
	if strings.Contains(note, `dependency ""`) {
		t.Fatalf("the proposal must never report an unnamed dependency: %q", note)
	}

	// done-when 9: a confirmed reaudit re-snapshots the governor.
	before := lockStoreOf(t, projectDir).Hashes[govChild][govHub]
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", govChild,
		"--confirm", "--reason", "the human read the reworded doctrine and agreed"); err != nil {
		t.Fatalf("claim reaudit --confirm: %v", err)
	}
	after := lockStoreOf(t, projectDir).Hashes[govChild][govHub]
	if after == "" || after == before {
		t.Fatalf("a confirmed reaudit must refresh the governance baseline: before=%q after=%q", before, after)
	}

	// And the drift is gone: a fresh check leaves review_pending cleared.
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v", err)
	}
	if n := countReviewPendingTrue(t, projectDir, "child.yaml"); n != 0 {
		t.Fatalf("a confirmed reaudit must clear the drift; claims/child.yaml still has %d review_pending: true line(s)", n)
	}
}

// TestDanglingGovernedByIsALintFindingNotAPanic is done-when 13: a
// governed_by.type naming a claim that does not exist is the dangling lint's
// job. Widening the baseline set must not turn an authoring mistake into a
// crash or a silent no-op.
func TestDanglingGovernedByIsALintFindingNotAPanic(t *testing.T) {
	cfgPath, projectDir := governedDriftFixture(t)
	path := filepath.Join(projectDir, "claims", "child.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read child: %v", err)
	}
	broken := strings.Replace(string(raw), "type: "+govHub, "type: widget.doctrine.nope", 1)
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err == nil {
		t.Fatalf("check --validate must fail on a dangling governed_by")
	}
	var checked checkData
	envData(t, env, &checked)
	found := false
	for _, f := range checked.LintFindings {
		if f.Lint == "dangling" && strings.Contains(f.Message, "governed_by") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a dangling finding naming governed_by, got %+v", checked.LintFindings)
	}
}
