// Package tests: this file covers the "Lock lifecycle & reaudit" edge-case
// category end to end via the built "docs" binary (see cli_test.go's
// TestMain/run/binPath, reused here rather than redeclared):
//
//  1. "dossierx claim lock" on a claim that still fails lint -> refused
//  2. a locked claim's dependency changes later -> flips to
//     locked+review_pending on the next "dossierx check", never auto-unlocks
//  3. "dossierx claim reaudit" on a claim that is not review_pending -> exit 2
//  4. reaudit concludes the claim still holds -> diff-free confirmation;
//     confirming clears review_pending, appends an audit note, stays locked
//  5. (see internal/reaudit's own tests for the mark-rendering contract;
//     the real LLM proposer is a documented stub here, see row 5 note below)
//  6. rejecting (skipping --confirm) leaves the claim completely untouched
//  7. confirming strips markup, refreshes the lock timestamp + dependency
//     hash, clears review_pending, and status stays locked (never draft)
//  8. multiple dependents go review_pending from one upstream change ->
//     "dossierx claim list --review-pending" lists all of them; "dossierx claim reaudit" only ever processes
//     the one id it was given
//  9. "dossierx claim list --review-pending" with nothing locked at all -> reports "nothing locked"
//
// 10. doctrine facet configured, hub not locked -> hub-gating blocks
// 11. doctrine facet not configured at all -> hub-gating does not run
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// llWriteConfig writes a project.config.yaml with the given facets/modules
// (and, if non-empty, doctrine_facet) plus an empty claims/ dir under root.
func llWriteConfig(t *testing.T, root string, facets, modules []string, doctrineFacet string) string {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	var b strings.Builder
	b.WriteString("schema_version: 1\nfacets:\n")
	for _, f := range facets {
		b.WriteString("  - " + f + "\n")
	}
	b.WriteString("modules:\n")
	for _, m := range modules {
		b.WriteString("  - " + m + "\n")
	}
	b.WriteString("claims_dir: claims\n")
	if doctrineFacet != "" {
		b.WriteString("doctrine_facet: " + doctrineFacet + "\n")
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	return cfgPath
}

type llClaimSpec struct {
	id, facet, module, status, body string
	restsOn, mirrors                []string
	reviewPending                   bool
}

func llWriteClaim(t *testing.T, root string, spec llClaimSpec) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("id: " + spec.id + "\n")
	b.WriteString("facet: " + spec.facet + "\n")
	b.WriteString("module: " + spec.module + "\n")
	b.WriteString("status: " + spec.status + "\n")
	if spec.reviewPending {
		b.WriteString("review_pending: true\n")
	}
	b.WriteString("body: |\n  " + spec.body + "\n")
	if len(spec.restsOn) > 0 {
		b.WriteString("rests_on:\n")
		for _, r := range spec.restsOn {
			b.WriteString("  - " + r + "\n")
		}
	}
	if len(spec.mirrors) > 0 {
		b.WriteString("mirrors:\n")
		for _, m := range spec.mirrors {
			b.WriteString("  - " + m + "\n")
		}
	}
	b.WriteString("governed_by:\n  type: none\n  reason: lock-lifecycle test fixture, not backed by any doctrine claim\n")

	path := filepath.Join(root, "claims", lastSegment(spec.id)+".yaml")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write claim %s: %v", spec.id, err)
	}
	return path
}

func lastSegment(id string) string {
	parts := strings.Split(id, ".")
	return parts[len(parts)-1]
}

func llReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// ---------------------------------------------------------------------
// Row 1: dossierx claim lock refused on a lint failure; claim stays untouched.
// ---------------------------------------------------------------------

func TestLockLifecycle_LockRefusedOnLintFailure(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")

	claimPath := llWriteClaim(t, root, llClaimSpec{
		id: "widget.contract.broken", facet: "contract", module: "widget", status: "draft",
		body:    "broken fixture claim with a dangling dependency.",
		restsOn: []string{"widget.contract.does-not-exist"},
	})
	before := llReadFile(t, claimPath)

	stdout, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.broken", "--reason", "test fixture")
	if code == 0 {
		t.Fatalf("expected non-zero exit locking a claim with a dangling dependency, got 0\nstdout: %s\nstderr: %s", stdout, stderr)
	}

	after := llReadFile(t, claimPath)
	if after != before {
		t.Fatalf("expected claim file untouched on refused lock\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if !strings.Contains(after, "status: draft") {
		t.Fatalf("expected claim to remain draft on disk, got:\n%s", after)
	}
}

// ---------------------------------------------------------------------
// Row 2: a locked claim's dependency changes -> flips to
// locked+review_pending on the next "dossierx check", never reverts to draft.
// ---------------------------------------------------------------------

func TestLockLifecycle_DependencyChangeFlipsReviewPendingOnCheck(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")

	depPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.dep", facet: "contract", module: "widget", status: "draft", body: "dependency claim, v1."})
	mainPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "main claim resting on the dependency.", restsOn: []string{"widget.contract.dep"}})

	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.dep", "--reason", "test fixture"); code != 0 {
		t.Fatalf("locking dependency claim failed: %s", stderr)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
		t.Fatalf("locking main claim failed: %s", stderr)
	}

	// Simulate the dependency's content changing underneath the lock.
	changedDep := llReadFile(t, depPath)
	changedDep = strings.Replace(changedDep, "dependency claim, v1.", "dependency claim, v2 (content changed).", 1)
	if err := os.WriteFile(depPath, []byte(changedDep), 0o644); err != nil {
		t.Fatalf("rewrite dependency claim: %v", err)
	}
	// The in-place rewrite of a LOCKED claim above stands in for the real
	// approval path (unlock -> edit -> lock), which records the new content on
	// the ledger. Re-record it, so the lock-ledger gate sees an approved edit
	// rather than tampering and this test keeps measuring dependency drift.
	armLedger(t, root)

	if _, stderr, code := run(t, root, "--config", cfgPath, "check"); code != 0 {
		t.Fatalf("dossierx check failed after dependency changed: %s", stderr)
	}

	mainAfter := llReadFile(t, mainPath)
	if !strings.Contains(mainAfter, "status: locked") {
		t.Fatalf("expected main claim to remain locked, got:\n%s", mainAfter)
	}
	if !strings.Contains(mainAfter, "review_pending: true") {
		t.Fatalf("expected main claim to be flagged review_pending after its dependency changed, got:\n%s", mainAfter)
	}

	staleOut, _, staleCode := run(t, root, "--config", cfgPath, "claim", "list", "--review-pending")
	if staleCode != 0 {
		t.Fatalf("dossierx claim list --review-pending exited non-zero: %d", staleCode)
	}
	if !strings.Contains(staleOut, "widget.contract.main") {
		t.Fatalf("expected dossierx claim list --review-pending to list widget.contract.main, got: %s", staleOut)
	}
}

// ---------------------------------------------------------------------
// Row 3: dossierx claim reaudit on a claim that is not review_pending -> exit 2.
// ---------------------------------------------------------------------

func TestLockLifecycle_ReauditRefusedWhenNotPending(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")

	claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.stable", facet: "contract", module: "widget", status: "locked", body: "a stable, already-locked claim."})
	before := llReadFile(t, claimPath)

	_, stderr, code := run(t, root, "--config", cfgPath, "claim", "reaudit", "widget.contract.stable")
	if code != 2 {
		t.Fatalf("expected exit code 2 for reaudit on a non-pending claim, got %d (stderr: %s)", code, stderr)
	}

	after := llReadFile(t, claimPath)
	if after != before {
		t.Fatalf("expected claim untouched when reaudit is refused")
	}
}

// ---------------------------------------------------------------------
// Rows 4, 6, 7: no-change reaudit confirmation clears review_pending,
// appends an audit note, and leaves status locked; rejecting (no
// --confirm) leaves the claim completely untouched.
// ---------------------------------------------------------------------

func TestLockLifecycle_ReauditRejectThenConfirm(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")

	depPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.dep", facet: "contract", module: "widget", status: "draft", body: "dependency claim, v1."})
	_ = depPath
	mainPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "main claim resting on the dependency.", restsOn: []string{"widget.contract.dep"}})

	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.dep", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock dep: %s", stderr)
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock main: %s", stderr)
	}

	depContents := llReadFile(t, depPath)
	depContents = strings.Replace(depContents, "dependency claim, v1.", "dependency claim, v2.", 1)
	if err := os.WriteFile(depPath, []byte(depContents), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	// The in-place rewrite of a LOCKED claim above stands in for the real
	// approval path (unlock -> edit -> lock), which records the new content on
	// the ledger. Re-record it, so the lock-ledger gate sees an approved edit
	// rather than tampering and this test keeps measuring dependency drift.
	armLedger(t, root)
	if _, stderr, code := run(t, root, "--config", cfgPath, "check"); code != 0 {
		t.Fatalf("check: %s", stderr)
	}

	pendingSnapshot := llReadFile(t, mainPath)
	if !strings.Contains(pendingSnapshot, "review_pending: true") {
		t.Fatalf("expected main to be review_pending before reaudit, got:\n%s", pendingSnapshot)
	}

	// Row 6: reject (no --confirm) -> claim on disk completely untouched.
	rejectOut, _, rejectCode := run(t, root, "--config", cfgPath, "claim", "reaudit", "widget.contract.main")
	if rejectCode != 0 {
		t.Fatalf("reaudit without --confirm should exit 0 (propose-only), got %d", rejectCode)
	}
	if !strings.Contains(rejectOut, "not applied") {
		t.Fatalf("expected reaudit output to say the proposal was not applied, got: %s", rejectOut)
	}
	afterReject := llReadFile(t, mainPath)
	if afterReject != pendingSnapshot {
		t.Fatalf("expected claim file completely untouched after rejecting a reaudit proposal\nbefore:\n%s\nafter:\n%s", pendingSnapshot, afterReject)
	}

	// Rows 4 & 7: confirm -> review_pending clears, status stays locked, an
	// audit note is appended.
	confirmOut, stderr, confirmCode := run(t, root, "--config", cfgPath, "claim", "reaudit", "widget.contract.main", "--confirm", "--reason", "test fixture")
	if confirmCode != 0 {
		t.Fatalf("confirmed reaudit failed: %d\nstdout: %s\nstderr: %s", confirmCode, confirmOut, stderr)
	}

	afterConfirm := llReadFile(t, mainPath)
	if strings.Contains(afterConfirm, "review_pending: true") {
		t.Fatalf("expected review_pending cleared after confirmed reaudit, got:\n%s", afterConfirm)
	}
	if !strings.Contains(afterConfirm, "status: locked") {
		t.Fatalf("expected claim to remain locked after confirmed reaudit (never draft), got:\n%s", afterConfirm)
	}
	if !strings.Contains(afterConfirm, "audit_notes:") {
		t.Fatalf("expected a durable audit note to be appended on confirmed reaudit, got:\n%s", afterConfirm)
	}
}

// ---------------------------------------------------------------------
// Row 8: multiple dependents go review_pending from one upstream change;
// dossierx claim list --review-pending lists all of them; dossierx claim reaudit only ever touches the one id
// it is given, never batch-applies the rest.
// ---------------------------------------------------------------------

func TestLockLifecycle_MultipleDependentsStaleListsAllReauditOneAtATime(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")

	depPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.dep", facet: "contract", module: "widget", status: "draft", body: "shared dependency, v1."})
	mainAPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.maina", facet: "contract", module: "widget", status: "draft", body: "dependent A.", restsOn: []string{"widget.contract.dep"}})
	mainBPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.mainb", facet: "contract", module: "widget", status: "draft", body: "dependent B.", restsOn: []string{"widget.contract.dep"}})

	for _, id := range []string{"widget.contract.dep", "widget.contract.maina", "widget.contract.mainb"} {
		if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", id, "--reason", "test fixture"); code != 0 {
			t.Fatalf("lock %s: %s", id, stderr)
		}
	}

	depContents := llReadFile(t, depPath)
	depContents = strings.Replace(depContents, "shared dependency, v1.", "shared dependency, v2.", 1)
	if err := os.WriteFile(depPath, []byte(depContents), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	// The in-place rewrite of a LOCKED claim above stands in for the real
	// approval path (unlock -> edit -> lock), which records the new content on
	// the ledger. Re-record it, so the lock-ledger gate sees an approved edit
	// rather than tampering and this test keeps measuring dependency drift.
	armLedger(t, root)
	if _, stderr, code := run(t, root, "--config", cfgPath, "check"); code != 0 {
		t.Fatalf("check: %s", stderr)
	}

	staleOut, _, staleCode := run(t, root, "--config", cfgPath, "claim", "list", "--review-pending")
	if staleCode != 0 {
		t.Fatalf("stale exited %d", staleCode)
	}
	if !strings.Contains(staleOut, "widget.contract.maina") || !strings.Contains(staleOut, "widget.contract.mainb") {
		t.Fatalf("expected dossierx claim list --review-pending to list both dependents, got: %s", staleOut)
	}

	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "reaudit", "widget.contract.maina", "--confirm", "--reason", "test fixture"); code != 0 {
		t.Fatalf("reaudit maina: %s", stderr)
	}

	mainAAfter := llReadFile(t, mainAPath)
	if strings.Contains(mainAAfter, "review_pending: true") {
		t.Fatalf("expected maina's review_pending cleared, got:\n%s", mainAAfter)
	}
	mainBAfter := llReadFile(t, mainBPath)
	if !strings.Contains(mainBAfter, "review_pending: true") {
		t.Fatalf("expected mainb to remain review_pending untouched (no batch auto-apply), got:\n%s", mainBAfter)
	}
}

// ---------------------------------------------------------------------
// Row 9: the review-pending filter over a project with nothing locked yet ->
// an empty result set, exit 0.
//
// The retired "stale" verb had a bespoke "nothing locked" message for this. Its
// replacement reports the same fact in the one shape every claim list uses, so
// a caller parses one thing rather than special-casing an empty project.
// ---------------------------------------------------------------------

func TestLockLifecycle_ReviewPendingFilterIsEmptyWhenNothingIsLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
	llWriteClaim(t, root, llClaimSpec{id: "widget.contract.draftonly", facet: "contract", module: "widget", status: "draft", body: "never locked."})

	out, stderr, code := run(t, root, "--config", cfgPath, "claim", "list", "--review-pending")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(out, "claim list: 0 of 1 claim(s)") {
		t.Fatalf("expected an empty review-pending result over one claim, got: %s", out)
	}
	if strings.Contains(out, "widget.contract.draftonly") {
		t.Fatalf("a draft claim is never review_pending; it must not be listed, got: %s", out)
	}
}

// ---------------------------------------------------------------------
// Rows 10 & 11: doctrine hub-gating, configured vs. not configured.
//
// These use a mirrors[] edge (not rests_on) between hub and child so the
// unrelated "rest-on-locked" lint can never independently block the lock
// attempt, isolating hub-gating as the only thing under test.
// ---------------------------------------------------------------------

func llWriteMirrorPair(t *testing.T, root, hubID, hubFacet, childID string) (hubPath, childPath string) {
	t.Helper()
	body := "identical shared content for a reciprocal mirrors pair."
	hubPath = llWriteClaim(t, root, llClaimSpec{id: hubID, facet: hubFacet, module: "widget", status: "draft", body: body, mirrors: []string{childID}})
	childPath = llWriteClaim(t, root, llClaimSpec{id: childID, facet: "contract", module: "widget", status: "draft", body: body, mirrors: []string{hubID}})
	return hubPath, childPath
}

func TestLockLifecycle_HubGatingBlocksWhenConfigured(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract", "doctrine"}, []string{"widget"}, "doctrine")
	hubPath, childPath := llWriteMirrorPair(t, root, "widget.doctrine.hub", "doctrine", "widget.contract.child")

	// Hub not yet locked: locking child must be refused by hub-gating.
	_, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.child", "--reason", "test fixture")
	if code == 0 {
		t.Fatalf("expected lock of child to be refused while doctrine hub is still draft")
	}
	if !strings.Contains(stderr, "doctrine") {
		t.Fatalf("expected hub-gating error to mention doctrine, got stderr: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, childPath), "status: draft") {
		t.Fatalf("expected child to remain draft after refused lock")
	}

	// Lock the hub, then locking the child succeeds.
	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.doctrine.hub", "--reason", "test fixture"); code != 0 {
		t.Fatalf("expected hub to lock successfully: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, hubPath), "status: locked") {
		t.Fatalf("expected hub to be locked on disk")
	}
	if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.child", "--reason", "test fixture"); code != 0 {
		t.Fatalf("expected child lock to succeed once hub is locked: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, childPath), "status: locked") {
		t.Fatalf("expected child to be locked on disk")
	}
}

func TestLockLifecycle_HubGatingSkippedWhenNotConfigured(t *testing.T) {
	root := t.TempDir()
	cfgPath := llWriteConfig(t, root, []string{"contract", "internals"}, []string{"widget"}, "") // no doctrine_facet
	hubPath, childPath := llWriteMirrorPair(t, root, "widget.internals.hub", "internals", "widget.contract.child")

	// Hub-gating is not configured at all: locking child must succeed even
	// though "hub" (not even in a doctrine-designated facet) is still
	// draft — nothing should gate on it.
	_, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.child", "--reason", "test fixture")
	if code != 0 {
		t.Fatalf("expected child lock to succeed when hub-gating is not configured: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, childPath), "status: locked") {
		t.Fatalf("expected child to be locked on disk")
	}
	if !strings.Contains(llReadFile(t, hubPath), "status: draft") {
		t.Fatalf("expected hub to remain untouched (still draft); nothing forced it to lock")
	}
}
