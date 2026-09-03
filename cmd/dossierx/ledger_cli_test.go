package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// ledgerProject writes a minimal project with one lockable claim and returns
// its config path, the claim file path, and the lock store path.
func ledgerProject(t *testing.T) (cfgPath, claimPath, storeFile string) {
	t.Helper()
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	claimPath = filepath.Join(claimsDir, "main.yaml")
	claim := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: draft\nbuild_role: schema\n" +
		"body: |\n  the approved body.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	return cfgPath, claimPath, filepath.Join(root, ".dossierx-lock-store.json")
}

// readLedger decodes the ledger out of the store file.
func readLedger(t *testing.T, storeFile string) map[string]lock.LedgerRecord {
	t.Helper()
	raw, err := os.ReadFile(storeFile)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var onDisk struct {
		Version int                          `json:"version"`
		Ledger  map[string]lock.LedgerRecord `json:"ledger"`
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse store: %v\n%s", err, raw)
	}
	if onDisk.Version != 2 {
		t.Fatalf("store version = %d, want the lock-ledger schema version 3:\n%s", onDisk.Version, raw)
	}
	return onDisk.Ledger
}

// TestCLI_LockUnlockRelockKeepsTheLedgerHonest drives the real approval path end
// to end and checks the ledger tracks it at every step. This is the wiring
// test: internal/lock cannot prove on its own that the CLI actually SAVES the
// store after each of these, and a ledger that is written in memory and never
// persisted is worse than none — it would report every locked claim as tampered
// on the next run.
func TestCLI_LockUnlockRelockKeepsTheLedgerHonest(t *testing.T) {
	cfgPath, claimPath, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed with Nitin"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	rec, ok := readLedger(t, storeFile)[id]
	if !ok {
		t.Fatalf("expected a ledger record persisted by claim lock")
	}
	if rec.Reason != "reviewed with Nitin" || rec.Subject != lock.SubjectClaim || rec.Grandfathered || rec.Released() {
		t.Fatalf("unexpected record after lock: %+v", rec)
	}
	lockedHash := rec.Hash

	// A locked, untouched project is clean under the gate.
	if findings := auditProject(t, cfgPath); len(findings) != 0 {
		t.Fatalf("expected a clean gate after an honest lock, got %+v", findings)
	}

	// Unlock RELEASES the record rather than deleting it, and persists that.
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "unlock", id, "--reason", "needs a correction"); err != nil {
		t.Fatalf("claim unlock: %v", err)
	}
	rec, ok = readLedger(t, storeFile)[id]
	if !ok {
		t.Fatalf("unlock must keep the ledger record (released), not delete it")
	}
	if !rec.Released() || rec.ReleasedReason != "needs a correction" {
		t.Fatalf("unexpected record after unlock: %+v", rec)
	}
	if rec.Hash != lockedHash {
		t.Fatalf("releasing must not rewrite what was originally approved")
	}
	if findings := auditProject(t, cfgPath); len(findings) != 0 {
		t.Fatalf("an honest unlock must leave the gate clean, got %+v", findings)
	}

	// Edit while draft — entirely allowed — then re-lock: a NEW approval.
	edited := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: draft\nbuild_role: schema\n" +
		"body: |\n  the corrected body.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("edit claim: %v", err)
	}
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", id, "--reason", "correction approved"); err != nil {
		t.Fatalf("claim lock (relock): %v", err)
	}
	rec = readLedger(t, storeFile)[id]
	if rec.Released() {
		t.Fatalf("re-locking must clear the prior release: %+v", rec)
	}
	if rec.Hash == lockedHash || rec.Reason != "correction approved" {
		t.Fatalf("re-locking must record the new content and the new approval: %+v", rec)
	}
	if findings := auditProject(t, cfgPath); len(findings) != 0 {
		t.Fatalf("expected a clean gate after the relock, got %+v", findings)
	}
}

// TestCLI_HandEditingALockedClaimIsCaught is the release's headline promise, run
// against the real binary path: a locked claim edited in a text editor no longer
// passes unnoticed. The three edits here are the ones ContentHash could never
// see — a body with no dependents to compare it, a swapped raw_html payload, and
// a status flipped straight to locked.
func TestCLI_HandEditingALockedClaimIsCaught(t *testing.T) {
	cfgPath, claimPath, _ := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}

	tampered := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: locked\nbuild_role: schema\n" +
		"body: |\n  a body nobody approved.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	findings := auditProject(t, cfgPath)
	if len(findings) != 1 || findings[0].Rule != lock.RuleLockContentDrift || findings[0].ClaimID != id {
		t.Fatalf("expected exactly one %s finding for %s, got %+v", lock.RuleLockContentDrift, id, findings)
	}
	// The message has to be actionable on its own: the hook prints it and
	// nothing else, so "what do I do now" must be answerable from this sentence.
	if !strings.Contains(findings[0].Message, "unlock") {
		t.Fatalf("the drift message must name the way out (unlock -> edit -> lock): %q", findings[0].Message)
	}
}

// TestCLI_CommentOpsNeverTripTheLedgerGate is the deny-list, proven through the
// real commands rather than in memory. Comments and review_pending are written
// by the engine constantly — from the CLI and from serve — and if either were
// signed, the gate would fire on ordinary work within a day and be switched off.
func TestCLI_CommentOpsNeverTripTheLedgerGate(t *testing.T) {
	cfgPath, _, _ := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "comment", "add", id, "--as", "human", "--body", "is this still true?"); err != nil {
		t.Fatalf("comment add: %v", err)
	}
	// The comment sets review_pending on the locked claim — a second
	// engine-managed field the ledger must not sign.
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Logf("check reported: %v (expected: the claim is now review_pending)", err)
	}

	if findings := auditProject(t, cfgPath); len(findings) != 0 {
		t.Fatalf("a comment and a review_pending flip are ordinary engine writes, not tampering; got %+v", findings)
	}
}

// TestCLI_ReaduditReSignsTheClaim: a confirmed reaudit is the second path that
// legitimately rewrites a locked claim's signed content (body, and an appended
// audit_notes entry). Without the re-sign hook, every honest reaudit would be
// reported as tampering from the moment it landed — the failure this test
// exists to make impossible to reintroduce.
func TestCLI_ReauditReSignsTheClaim(t *testing.T) {
	cfgPath, _, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	before := readLedger(t, storeFile)[id]

	// A flag is the trigger reaudit can actually act on: it carries a real,
	// ready-to-review proposal rather than the dependency-drift stub.
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "flag", id,
		"--claim-says", "the approved body.", "--now-does", "the corrected body.", "--reason", "the code changed"); err != nil {
		t.Fatalf("claim flag: %v", err)
	}
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "reaudit", id, "--confirm", "--reason", "confirmed with Nitin"); err != nil {
		t.Fatalf("claim reaudit --confirm: %v", err)
	}

	after := readLedger(t, storeFile)[id]
	if after.Hash == before.Hash {
		t.Fatalf("the reaudit rewrote the claim but the ledger record did not move")
	}
	if after.Reason != "confirmed with Nitin" {
		t.Fatalf("the reaudit's own approval must go on the record, got %q", after.Reason)
	}
	if findings := auditProject(t, cfgPath); len(findings) != 0 {
		t.Fatalf("a confirmed reaudit must leave the gate clean, got %+v", findings)
	}
}

// TestCLI_BuildOrderLockIsOnTheRecord: a locked build order is the second class
// of locked artifact in this engine, and leaving it outside the ledger would
// make "nothing already locked changes without your approval on the record" an
// overclaim about half the locked things in a project.
func TestCLI_BuildOrderLockIsOnTheRecord(t *testing.T) {
	cfgPath, _, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "order approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}

	rec, ok := readLedger(t, storeFile)[lock.BuildOrderLedgerKey("widget")]
	if !ok {
		t.Fatalf("expected a ledger record for the locked build order")
	}
	if rec.Subject != lock.SubjectBuildOrder || rec.Reason != "order approved" || rec.Hash == "" {
		t.Fatalf("unexpected build-order record: %+v", rec)
	}
	// And it must not be mistaken for a claim by the claim rules.
	if findings := auditProject(t, cfgPath); len(findings) != 0 {
		t.Fatalf("a build-order record must not disturb the claim rules, got %+v", findings)
	}
}

// TestCLI_DeletingTheLedgerIsNotSilentAdoption is the bypass the grandfathering
// trigger is designed against. If an absent ledger meant "adopt everything",
// the attack on this whole feature would be one `rm`.
func TestCLI_DeletingTheLedgerIsNotSilentAdoption(t *testing.T) {
	cfgPath, _, storeFile := ledgerProject(t)
	const id = "widget.contract.main"

	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
		t.Fatalf("claim lock: %v", err)
	}
	if err := os.Remove(storeFile); err != nil {
		t.Fatalf("remove store: %v", err)
	}

	// Any command may run — check included — but none of them may re-bless the
	// project on the way past.
	if _, _, err := execReviewedCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Logf("check reported: %v", err)
	}

	findings := auditProject(t, cfgPath)
	if !hasCLIRule(findings, lock.RuleLockLedgerAbsent) || !hasCLIRule(findings, lock.RuleLockLedgerMissing) {
		t.Fatalf("deleting the ledger must be reported, never silently adopted; got %+v", findings)
	}
}

// auditProject loads the project the way a hook would — config, claims, both
// stores — and runs the ledger gate over it.
func auditProject(t *testing.T, cfgPath string) []lock.Finding {
	t.Helper()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	digests, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	return lock.Audit(claims, store, digests)
}

func hasCLIRule(findings []lock.Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}
