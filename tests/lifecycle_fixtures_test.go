// lifecycle_fixtures_test.go covers five claim-lifecycle scenarios end to
// end via the built CLI binary:
//
//  1. doctrine-gate: a doctrine_facet hub claim left in draft blocks
//     locking a dependent that mirrors it, via testdata/fixture-coverage/
//     lifecycle/doctrine-gate.
//  2. undeclared-facet: a claim whose facet isn't in config.facets fails
//     lint, via testdata/fixture-coverage/lifecycle/undeclared-facet.
//  3. empty-claims: a valid config with an empty claims_dir lints clean,
//     exit 0, via testdata/fixture-coverage/lifecycle/empty-claims.
//  4. dependency content-hash drift: locking B (rests_on A) then editing
//     A's body flips B to locked+review_pending on "dossierx check", built
//     programmatically (reusing writeRestOnLockedFixture from
//     edge_lints_test.go) rather than as a static fixture, since it
//     mutates state across multiple CLI invocations.
//  5. an explicit "dossierx claim flag" trigger: flags a locked claim, proves it
//     shows up in "dossierx claim list --review-pending", and that "dossierx claim reaudit" (without
//     --confirm) proposes a real, non-stub diff -- built programmatically
//     (reusing llWriteConfig/llWriteClaim from lock_lifecycle_test.go) for
//     the same reason as scenario 4.
//
// Scenarios 1-3 use static, checked-in testdata fixtures (per the task
// brief); scenarios 4-5 build their own throwaway t.TempDir() project,
// since the "dossierx claim lock"/"dossierx claim flag"/"dossierx check" commands they exercise
// mutate claim files and write lock-store/catalog/viewer artifacts, which
// must never happen against a checked-in testdata directory.
package tests

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyFixtureDir recursively copies every file under src into dst,
// preserving relative paths, so a static testdata fixture that a test is
// about to mutate (lock/flag/check all write to disk) can be copied into a
// throwaway t.TempDir() first, keeping the checked-in fixture pristine and
// every test run idempotent.
func copyFixtureDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatalf("copy fixture %s -> %s: %v", src, dst, err)
	}
}

// lifecycleFixturesRoot resolves testdata/fixture-coverage/lifecycle
// relative to this test file's own package directory (tests/).
func lifecycleFixturesRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "testdata", "fixture-coverage", "lifecycle"))
	if err != nil {
		t.Fatalf("resolve fixture-coverage/lifecycle path: %v", err)
	}
	return abs
}

// ---------------------------------------------------------------------
// 1. doctrine-gate: hub left draft blocks locking its dependent, per
//    doctrine_facet hub-gating.
// ---------------------------------------------------------------------

func TestLifecycle_DoctrineGateFixture(t *testing.T) {
	src := filepath.Join(lifecycleFixturesRoot(t), "doctrine-gate")
	root := t.TempDir()
	copyFixtureDir(t, src, root)

	childPath := filepath.Join(root, "claims", "child.yaml")
	hubPath := filepath.Join(root, "claims", "hub.yaml")

	// Hub still draft: locking the child (which mirrors the hub) must be
	// refused by hub-gating, mentioning the doctrine facet.
	_, stderr, code := reviewedRun(t, root, "claim", "lock", "dgatemod.contract.child", "--reason", "test fixture")
	if code == 0 {
		t.Fatalf("expected lock of child to be refused while doctrine hub is still draft")
	}
	if !strings.Contains(stderr, "doctrine") {
		t.Fatalf("expected hub-gating refusal to mention doctrine, got stderr: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, childPath), "status: draft") {
		t.Fatalf("expected child to remain draft after refused lock")
	}

	// Lock the hub, then the child locks successfully.
	if _, stderr, code := reviewedRun(t, root, "claim", "lock", "dgatemod.doctrine.hub", "--reason", "test fixture"); code != 0 {
		t.Fatalf("expected hub to lock successfully: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, hubPath), "status: locked") {
		t.Fatalf("expected hub to be locked on disk")
	}
	if _, stderr, code := reviewedRun(t, root, "claim", "lock", "dgatemod.contract.child", "--reason", "test fixture"); code != 0 {
		t.Fatalf("expected child lock to succeed once hub is locked: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, childPath), "status: locked") {
		t.Fatalf("expected child to be locked on disk")
	}
}

// ---------------------------------------------------------------------
// 2. undeclared-facet: a claim's facet isn't in config.facets -> lint
//    fails (id-shape), never silently accepted.
// ---------------------------------------------------------------------

func TestLifecycle_UndeclaredFacetFixture(t *testing.T) {
	dir := filepath.Join(lifecycleFixturesRoot(t), "undeclared-facet")
	cfgPath := filepath.Join(dir, "project.config.yaml")

	stdout, stderr, code := reviewedRun(t, dir, "--config", cfgPath, "check", "--validate")
	if code == 0 {
		t.Fatalf("expected non-zero exit for a claim whose facet is not declared in config.facets, got 0 (stdout: %s)", stdout)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "facet") {
		t.Fatalf("expected the lint failure to mention the undeclared facet, got: %s", combined)
	}
}

// ---------------------------------------------------------------------
// 3. empty-claims: a valid config with claims_dir present but empty lints
//    clean, exit 0, zero findings.
// ---------------------------------------------------------------------

func TestLifecycle_EmptyClaimsFixture(t *testing.T) {
	dir := filepath.Join(lifecycleFixturesRoot(t), "empty-claims")
	cfgPath := filepath.Join(dir, "project.config.yaml")

	stdout, stderr, code := reviewedRun(t, dir, "--config", cfgPath, "check", "--validate")
	if code != 0 {
		t.Fatalf("expected exit 0 for an empty claims_dir, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "0 findings") {
		t.Fatalf("expected zero findings reported, got: %s", stdout)
	}
}

// ---------------------------------------------------------------------
// 4. Dependency content-hash drift: lock A, lock B (rests_on A), edit A's
//    body on disk, re-run check, assert B is now review_pending while
//    still locked. Built programmatically via writeRestOnLockedFixture
//    (edge_lints_test.go), with a module name of its own to avoid any
//    fixture collision.
// ---------------------------------------------------------------------

func TestLifecycle_DependencyDriftFlipsReviewPending(t *testing.T) {
	root := t.TempDir()
	aPath, bPath := writeRestOnLockedFixture(t, root, "lifecycledriftmod")

	if _, stderr, code := reviewedRun(t, root, "claim", "lock", "lifecycledriftmod.contract.a", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock A: %s", stderr)
	}
	if _, stderr, code := reviewedRun(t, root, "claim", "lock", "lifecycledriftmod.contract.b", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock B: %s", stderr)
	}

	// Edit A's body underneath the now-locked B.
	changedA := "id: lifecycledriftmod.contract.a\n" +
		"facet: contract\nmodule: lifecycledriftmod\nstatus: locked\nlayout: card\n" +
		"body: |\n  CHANGED body for A, after B locked against it.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(aPath, []byte(changedA), 0o644); err != nil {
		t.Fatalf("rewrite claim a: %v", err)
	}
	// The in-place rewrite of a LOCKED claim above stands in for the real
	// approval path (unlock -> edit -> lock), which records the new content on
	// the ledger. Re-record it, so the lock-ledger gate sees an approved edit
	// rather than tampering and this test keeps measuring dependency drift.
	armLedger(t, root)

	if _, stderr, code := reviewedRun(t, root, "check"); code != 0 {
		t.Fatalf("check: %s", stderr)
	}

	after := llReadFile(t, bPath)
	if !strings.Contains(after, "status: locked") {
		t.Fatalf("expected B to remain locked (never auto-revert to draft), got:\n%s", after)
	}
	if !strings.Contains(after, "review_pending: true") {
		t.Fatalf("expected B's dependency drift on A to flip review_pending: true, got:\n%s", after)
	}
}

// ---------------------------------------------------------------------
// 5. An explicit "dossierx claim flag" trigger: lock A, "dossierx claim flag A --claim-says
//    ... --now-does ... --reason ...", then confirm A shows up in
//    "dossierx claim list --review-pending" and that "dossierx claim reaudit A" (propose-only) prints a real,
//    non-stub diff carrying the flagged content. Built programmatically
//    via llWriteConfig/llWriteClaim (lock_lifecycle_test.go).
// ---------------------------------------------------------------------

func TestLifecycle_DocsFlagTriggersReviewPendingWithRealDiff(t *testing.T) {
	root := t.TempDir()
	llWriteConfig(t, root, []string{"contract"}, []string{"flagmod"}, "")
	claimPath := llWriteClaim(t, root, llClaimSpec{
		id: "flagmod.contract.a", facet: "contract", module: "flagmod", status: "draft",
		body: "the claim's original, soon-to-be-flagged assertion.",
	})

	if _, stderr, code := reviewedRun(t, root, "claim", "lock", "flagmod.contract.a", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock: %s", stderr)
	}
	if !strings.Contains(llReadFile(t, claimPath), "status: locked") {
		t.Fatalf("expected claim to be locked before flagging")
	}

	claimSays := "OLD: legacy assumption text"
	nowDoes := "NEW: corrected fact text"
	_, stderr, code := reviewedRun(t, root, "claim", "flag", "flagmod.contract.a",
		"--claim-says", claimSays,
		"--now-does", nowDoes,
		"--reason", "corrected during code review",
	)
	if code != 0 {
		t.Fatalf("flag: %s", stderr)
	}

	afterFlag := llReadFile(t, claimPath)
	if !strings.Contains(afterFlag, "status: locked") {
		t.Fatalf("expected claim to remain locked after being flagged, got:\n%s", afterFlag)
	}
	if !strings.Contains(afterFlag, "review_pending: true") {
		t.Fatalf("expected \"dossierx flag\" to set review_pending: true, got:\n%s", afterFlag)
	}

	staleOut, staleErr, staleCode := reviewedRun(t, root, "claim", "list", "--review-pending")
	if staleCode != 0 {
		t.Fatalf("stale: %s", staleErr)
	}
	if !strings.Contains(staleOut, "flagmod.contract.a") {
		t.Fatalf("expected \"dossierx claim list --review-pending\" to list the flagged claim, got: %s", staleOut)
	}

	// "dossierx claim reaudit" (propose-only, no --confirm) must produce a real diff
	// sourced from the flag -- not ProposeDiff's dependency-diff stub.
	reauditOut, reauditErr, reauditCode := reviewedRun(t, root, "claim", "reaudit", "flagmod.contract.a")
	if reauditCode != 0 {
		t.Fatalf("reaudit: %s", reauditErr)
	}
	if !strings.Contains(reauditOut, "no_change=false") {
		t.Fatalf("expected a real (not no-change) proposal for a flag-sourced reaudit, got: %s", reauditOut)
	}
	if strings.Contains(reauditOut, "stub: ProposeDiff") {
		t.Fatalf("expected the flag-sourced diff, not the dependency-diff stub, got: %s", reauditOut)
	}
	if !strings.Contains(reauditOut, claimSays) || !strings.Contains(reauditOut, nowDoes) {
		t.Fatalf("expected the proposed diff to carry the flagged claim-says/now-does text, got: %s", reauditOut)
	}
	if !strings.Contains(reauditOut, "<mark") {
		t.Fatalf("expected the proposed diff to carry real git-diff-style <mark> markup, got: %s", reauditOut)
	}
	if !strings.Contains(reauditOut, "not applied") {
		t.Fatalf("expected reaudit without --confirm to report the proposal was not applied, got: %s", reauditOut)
	}

	// Propose-only must not have touched the claim file at all.
	if llReadFile(t, claimPath) != afterFlag {
		t.Fatalf("expected claim file untouched by a propose-only reaudit")
	}

	// Now confirm it, and check the flag's now-does text actually lands as
	// the claim's new body, review_pending clears, and status stays locked.
	confirmOut, confirmErr, confirmCode := reviewedRun(t, root, "claim", "reaudit", "flagmod.contract.a", "--confirm", "--reason", "test fixture")
	if confirmCode != 0 {
		t.Fatalf("confirmed reaudit: %s", confirmErr)
	}
	_ = confirmOut
	afterConfirm := llReadFile(t, claimPath)
	if strings.Contains(afterConfirm, "review_pending: true") {
		t.Fatalf("expected review_pending cleared after confirmed reaudit, got:\n%s", afterConfirm)
	}
	if !strings.Contains(afterConfirm, "status: locked") {
		t.Fatalf("expected claim to remain locked after confirmed reaudit, got:\n%s", afterConfirm)
	}
	if !strings.Contains(afterConfirm, nowDoes) {
		t.Fatalf("expected the confirmed reaudit to replace the claim body with now-does text, got:\n%s", afterConfirm)
	}
}
