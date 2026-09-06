// edge_lints_test.go covers the "Edges & lints" scenarios that need a real
// end-to-end CLI run to prove, rather than a single package's unit test:
// scenario 4 from the edge-case list — "rests_on a locked claim -> allowed
// via rest-on-locked; the locked claim now tracks this new dependent for
// future review_pending checks" — exercises lint (rest-on-locked), lock,
// and the lock-content-hash Store together, across two separate CLI
// invocations ("dossierx claim lock" then "dossierx check"), which no single package's
// unit test can exercise on its own.
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRestOnLockedFixture writes a two-claim project: "a" (initially
// draft, no edges) and "b" (draft, rests_on "a"). Both are lint-clean on
// their own so both can be locked via the CLI without any other lint
// getting in the way.
func writeRestOnLockedFixture(t *testing.T, root, module string) (aPath, bPath string) {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}

	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - " + module + "\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}

	aPath = filepath.Join(claimsDir, "a.yaml")
	aClaim := "id: " + module + ".contract.a\n" +
		"facet: contract\nmodule: " + module + "\nstatus: draft\nlayout: card\n" +
		"body: |\n  original body for A.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(aPath, []byte(aClaim), 0o644); err != nil {
		t.Fatalf("write claim a: %v", err)
	}

	bPath = filepath.Join(claimsDir, "b.yaml")
	bClaim := "id: " + module + ".contract.b\n" +
		"facet: contract\nmodule: " + module + "\nstatus: draft\nlayout: card\n" +
		"body: |\n  B rests on A.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n" +
		"rests_on:\n  - " + module + ".contract.a\n"
	if err := os.WriteFile(bPath, []byte(bClaim), 0o644); err != nil {
		t.Fatalf("write claim b: %v", err)
	}

	return aPath, bPath
}

// TestRestOnLockedTracksDependentForReviewPending exercises scenario 4 of
// the edge-case list end to end:
//
//  1. B rests_on A while both are still draft: lint is clean (rest-on-locked
//     only fires for a *locked* claim resting on a non-locked one).
//  2. Locking A, then B, succeeds — rest-on-locked allows B (now locked) to
//     rest on A (already locked at the time B is locked).
//  3. A's content is edited on disk after both are locked (simulating a
//     dependency changing underneath a locked dependent).
//  4. "dossierx check" must flip B's review_pending to true (persisted to B's
//     own claim file), because B now has A tracked as a dependency whose
//     content-hash baseline no longer matches. B's status must remain
//     "locked" throughout — it never reverts to draft.
func TestRestOnLockedTracksDependentForReviewPending(t *testing.T) {
	root := t.TempDir()
	aPath, bPath := writeRestOnLockedFixture(t, root, "restlockmod")

	// Step 1: lint is clean while both are draft.
	stdout, stderr, code := reviewedRun(t, root, "check", "--validate")
	if code != 0 {
		t.Fatalf("lint before locking: expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// Step 2: lock A, then lock B (which rests_on the now-locked A).
	if _, stderr, code := reviewedRun(t, root, "claim", "lock", "restlockmod.contract.a", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock A: expected exit 0, got %d\nstderr: %s", code, stderr)
	}
	if _, stderr, code := reviewedRun(t, root, "claim", "lock", "restlockmod.contract.b", "--reason", "test fixture"); code != 0 {
		t.Fatalf("lock B (rests_on locked A): expected exit 0 via rest-on-locked, got %d\nstderr: %s", code, stderr)
	}

	// Sanity: both files now say status: locked.
	for _, p := range []string{aPath, bPath} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		if !strings.Contains(string(raw), "status: locked") {
			t.Fatalf("expected %s to be locked after \"dossierx lock\", got:\n%s", p, raw)
		}
	}

	// Step 3: change A's content underneath the now-locked B.
	changedA := "id: restlockmod.contract.a\n" +
		"facet: contract\nmodule: restlockmod\nstatus: locked\nlayout: card\n" +
		"body: |\n  CHANGED body for A, after B was locked against it.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(aPath, []byte(changedA), 0o644); err != nil {
		t.Fatalf("rewrite claim a: %v", err)
	}

	// Step 4: "dossierx check" must detect the drift and flag B review_pending,
	// while B stays locked. Run "dossierx claim list --review-pending" afterward to confirm via the
	// CLI's own reporting surface, not just the raw file.
	if stdout, stderr, code := reviewedRun(t, root, "check"); code == 0 {
		// "dossierx check" is expected to still report clean lint/catalog/render
		// (a review_pending flag by itself isn't a lint finding), so a
		// successful exit here is fine — what matters is what it persisted.
		_ = stdout
		_ = stderr
	}

	raw, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatalf("read %s after check: %v", bPath, err)
	}
	if !strings.Contains(string(raw), "status: locked") {
		t.Fatalf("expected B to remain locked (never auto-revert to draft), got:\n%s", raw)
	}
	if !strings.Contains(string(raw), "review_pending: true") {
		t.Fatalf("expected B's dependency drift on A to flip review_pending: true, got:\n%s", raw)
	}

	stdout, stderr, code = reviewedRun(t, root, "claim", "list", "--review-pending")
	if code != 0 {
		t.Fatalf("stale: expected exit 0 (report-only), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "restlockmod.contract.b") {
		t.Fatalf("expected \"dossierx claim list --review-pending\" to list restlockmod.contract.b, got: %s", stdout)
	}

	// A carries its own approval drift after its authored content changes.
	if !strings.Contains(stdout, "restlockmod.contract.a") {
		t.Fatalf("expected A's own approval drift to remain visible, got: %s", stdout)
	}
}

// TestRestOnLockedAllowsLocalApprovalAgainstDraftTarget pins the policy-v1
// replacement for the old write gate: a readable draft prerequisite leaves a
// visible dependency_unapproved condition, not an unapproved local claim.
func TestRestOnLockedAllowsLocalApprovalAgainstDraftTarget(t *testing.T) {
	root := t.TempDir()
	writeRestOnLockedFixture(t, root, "restlockneg")

	// The reviewed write locally approves B while A remains draft.
	stdout, stderr, code := reviewedRun(t, root, "claim", "lock", "restlockneg.contract.b", "--reason", "test fixture")
	if code != 0 {
		t.Fatalf("expected local approval against a readable draft dependency, got exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	show, showErr, showCode := run(t, root, "claim", "show", "restlockneg.contract.b")
	if showCode != 0 {
		t.Fatalf("show locally approved dependent: exit %d stdout=%s stderr=%s", showCode, show, showErr)
	}
	if !strings.Contains(show, "local/dependency ready: true / false") || !strings.Contains(show, "dependency_unapproved") {
		t.Fatalf("local approval must expose the draft dependency condition, got: %s", show)
	}

	raw, err := os.ReadFile(filepath.Join(root, "claims", "b.yaml"))
	if err != nil {
		t.Fatalf("read b.yaml: %v", err)
	}
	if !strings.Contains(string(raw), "status: locked") {
		t.Fatalf("B must be locally approved after the reviewed write, got:\n%s", raw)
	}
}

// TestLockSucceedsWithOnlyWarningSeverityFinding proves "dossierx claim lock"'s lint
// gate mirrors "dossierx check --validate"/"dossierx check"'s own pass/fail semantics: a claim
// that trips only a warning-severity lint (here, the real "orphan" lint —
// a lone claim with no mirrors/rests_on edges in either direction) must
// still lock successfully, exactly as "dossierx check --validate" exits 0 for it (findings
// reported, no error-level findings). This is the CLI-level companion to
// internal/lock/lock_test.go's TestLockSucceedsWithOnlyWarningFindings,
// which proves the same thing against the real Registry instead of a
// package-internal stub.
func TestLockSucceedsWithOnlyWarningSeverityFinding(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - orphanmod\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	claimPath := filepath.Join(claimsDir, "lonely.yaml")
	claim := "id: orphanmod.contract.lonely\n" +
		"facet: contract\nmodule: orphanmod\nstatus: draft\nlayout: card\n" +
		"body: |\n  a claim with no edges at all, so only the warning-severity orphan lint fires.\n" +
		"governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	// Sanity: "dossierx check --validate" reports the orphan finding but exits 0 (warning,
	// not error).
	lintOut, lintErr, lintCode := reviewedRun(t, root, "check", "--validate")
	if lintCode != 0 {
		t.Fatalf("expected \"dossierx check --validate\" to exit 0 for a warning-only orphan finding, got %d\nstdout: %s\nstderr: %s", lintCode, lintOut, lintErr)
	}
	if !strings.Contains(lintOut, "orphan") {
		t.Fatalf("expected the orphan finding to be reported (even though it doesn't fail), got: %s", lintOut)
	}

	stdout, stderr, code := reviewedRun(t, root, "claim", "lock", "orphanmod.contract.lonely", "--reason", "test fixture")
	if code != 0 {
		t.Fatalf("expected \"dossierx lock\" to succeed with only a warning-severity finding outstanding, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	raw, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim after lock: %v", err)
	}
	if !strings.Contains(string(raw), "status: locked") {
		t.Fatalf("expected claim to be locked on disk, got:\n%s", raw)
	}
}
