// flag_implink_cli_test.go covers the CLI wiring for the two Channel
// A/B additions: "docs flag <id> --claim-says --now-does --reason" (which
// extends the existing reaudit lifecycle with a second, agent-sourced
// trigger) and "docs implink set|status" (the fully agent-autonomous
// implementation-link feature). Mirrors TestCLI_LockCheckStaleReauditUnlockFlow's
// style: one on-disk fixture project, driven end to end through execCLI.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLockedFixtureClaim(t *testing.T, claimsDir, id, module, body string) string {
	t.Helper()
	path := filepath.Join(claimsDir, strings.ReplaceAll(id, ".", "_")+".yaml")
	src := "id: " + id + "\nfacet: contract\nmodule: " + module + "\nstatus: locked\nlayout: card\nbuild_role: behavior\n" +
		"body: |\n  " + body + "\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write claim %s: %v", id, err)
	}
	return path
}

// ---------------------------------------------------------------------
// docs flag <id> --claim-says --now-does --reason
// ---------------------------------------------------------------------

func TestCLI_Flag_RequiresAllThreeFlags(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeLockedFixtureClaim(t, claimsDir, "widget.contract.main", "widget", "original body")

	if _, _, err := execCLI(t, "--config", cfgPath, "flag", "widget.contract.main", "--claim-says", "x"); err == nil {
		t.Fatalf("expected flag to refuse when --now-does/--reason are missing")
	}
}

func TestCLI_Flag_RefusesUnlockedClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget") // draft claim.

	_, _, err := execCLI(t, "--config", cfgPath, "flag", "widget.contract.overview",
		"--claim-says", "a", "--now-does", "b", "--reason", "c")
	if err == nil {
		t.Fatalf("expected flag to refuse a draft (not locked) claim")
	}
}

func TestCLI_Flag_RefusesUnknownClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	_, _, err := execCLI(t, "--config", cfgPath, "flag", "widget.contract.does-not-exist",
		"--claim-says", "a", "--now-does", "b", "--reason", "c")
	if err == nil {
		t.Fatalf("expected flag to refuse an unknown claim id")
	}
}

// TestCLI_FlagThenReauditConfirm_EndToEnd exercises the full Channel A
// lifecycle: flag a locked claim, confirm it's now review_pending, run
// "docs reaudit --confirm" and check the claim's body became exactly
// --now-does (per ProposeFlagDiff's doc comment: a flag replaces the whole
// body, it isn't a surgical in-place edit), review_pending cleared, and an
// audit note recorded the flag's reason.
func TestCLI_FlagThenReauditConfirm_EndToEnd(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	claimPath := writeLockedFixtureClaim(t, claimsDir, "widget.contract.main", "widget", "the old assertion")

	flagOut, _, err := execCLI(t, "--config", cfgPath, "flag", "widget.contract.main",
		"--claim-says", "the old assertion", "--now-does", "the corrected assertion", "--reason", "code changed under it")
	if err != nil {
		t.Fatalf("flag: %v (out: %s)", err, flagOut)
	}
	if !strings.Contains(flagOut, "review_pending=true") {
		t.Fatalf("expected flag output to confirm review_pending=true, got: %s", flagOut)
	}

	afterFlag, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if !strings.Contains(string(afterFlag), "review_pending: true") {
		t.Fatalf("expected the claim file itself to carry review_pending: true, got:\n%s", afterFlag)
	}

	// Propose-only reaudit should show claim-says/now-does as a red/green diff.
	proposeOut, _, err := execCLI(t, "--config", cfgPath, "reaudit", "widget.contract.main")
	if err != nil {
		t.Fatalf("reaudit (propose-only): %v (out: %s)", err, proposeOut)
	}
	if !strings.Contains(proposeOut, "the old assertion") || !strings.Contains(proposeOut, "the corrected assertion") {
		t.Fatalf("expected the proposed diff to show both claim-says and now-does, got: %s", proposeOut)
	}
	if !strings.Contains(proposeOut, "flagged: code changed under it") {
		t.Fatalf("expected the flag's reason surfaced in the proposal note, got: %s", proposeOut)
	}

	confirmOut, _, err := execCLI(t, "--config", cfgPath, "reaudit", "widget.contract.main", "--confirm")
	if err != nil {
		t.Fatalf("reaudit --confirm: %v (out: %s)", err, confirmOut)
	}

	afterConfirm, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if strings.Contains(string(afterConfirm), "review_pending: true") {
		t.Fatalf("expected review_pending cleared after confirmed reaudit, got:\n%s", afterConfirm)
	}
	if !strings.Contains(string(afterConfirm), "the corrected assertion") {
		t.Fatalf("expected the claim body replaced with now-does, got:\n%s", afterConfirm)
	}
	if strings.Contains(string(afterConfirm), "the old assertion") {
		t.Fatalf("expected the old claim-says text gone from the applied body, got:\n%s", afterConfirm)
	}
	if !strings.Contains(string(afterConfirm), "code changed under it") {
		t.Fatalf("expected the flag's reason recorded in audit_notes, got:\n%s", afterConfirm)
	}

	// The flag store entry must be consumed (deleted) by the confirmed
	// reaudit — re-running reaudit now must fall back to exit 2 (not
	// locked+review_pending), proving there is no leftover pending flag.
	if _, _, err := execCLI(t, "--config", cfgPath, "flag", "widget.contract.main",
		"--claim-says", "x", "--now-does", "y", "--reason", "z"); err != nil {
		t.Fatalf("re-flagging after a confirmed reaudit should still work: %v", err)
	}
}

func TestCLI_Flag_AnyLockedClaimCanBeReflagged(t *testing.T) {
	// A claim that is locked but NOT yet review_pending may still be
	// flagged (unlike "docs reaudit", which requires review_pending
	// already true) — this is the whole point of "docs flag": it is what
	// PUTS a claim into review_pending.
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeLockedFixtureClaim(t, claimsDir, "widget.contract.main", "widget", "assertion")

	if _, _, err := execCLI(t, "--config", cfgPath, "flag", "widget.contract.main",
		"--claim-says", "a", "--now-does", "b", "--reason", "c"); err != nil {
		t.Fatalf("expected flag to succeed on a locked-but-not-yet-pending claim: %v", err)
	}
}

// ---------------------------------------------------------------------
// docs implink set / status
// ---------------------------------------------------------------------

func TestCLI_ImplinkSet_ThenStatus(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeLockedFixtureClaim(t, claimsDir, "widget.contract.main", "widget", "assertion")

	srcPath := filepath.Join(root, "widget.go")
	if err := os.WriteFile(srcPath, []byte("package widget"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	setOut, _, err := execCLI(t, "--config", cfgPath, "implink", "set",
		"--module", "widget", "--claim", "widget.contract.main", "--file", "widget.go", "--symbol", "Run")
	if err != nil {
		t.Fatalf("implink set: %v (out: %s)", err, setOut)
	}
	if !strings.Contains(setOut, "widget.contract.main -> widget.go#Run") {
		t.Fatalf("expected implink set to echo the claim->file#symbol link, got: %s", setOut)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".implementation.widget.json")); statErr != nil {
		t.Fatalf("expected .implementation.widget.json to exist: %v", statErr)
	}

	statusOut, _, err := execCLI(t, "--config", cfgPath, "implink", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("implink status: %v (out: %s)", err, statusOut)
	}
	if !strings.Contains(statusOut, "impl-links: 1 linked, 0 drifted") {
		t.Fatalf("expected a 1 linked, 0 drifted summary, got: %s", statusOut)
	}
}

func TestCLI_ImplinkSet_RefusesDraftClaim(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget") // draft claim.
	srcPath := filepath.Join(root, "widget.go")
	if err := os.WriteFile(srcPath, []byte("package widget"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, _, err := execCLI(t, "--config", cfgPath, "implink", "set",
		"--module", "widget", "--claim", "widget.contract.overview", "--file", "widget.go")
	if err == nil {
		t.Fatalf("expected implink set to refuse linking a draft (not locked) claim")
	}
}

func TestCLI_ImplinkStatus_NothingLinkedYet(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	out, _, err := execCLI(t, "--config", cfgPath, "implink", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("implink status: %v", err)
	}
	if !strings.Contains(out, "nothing linked yet") {
		t.Fatalf("expected a graceful 'nothing linked yet' message, got: %s", out)
	}
}

// TestCLI_Check_ImplinkLine_ZeroCostWhenUnused proves "docs check" prints
// no impl-links line at all for a project that has never called "docs
// implink set" — the zero-cost/silent-when-unused contract this step must
// uphold, mirroring build_order's own graceful-degradation precedent.
func TestCLI_Check_ImplinkLine_ZeroCostWhenUnused(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	out, _, err := execCLI(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check: %v (out: %s)", err, out)
	}
	if strings.Contains(out, "impl-links") {
		t.Fatalf("expected no impl-links line for a project that has never used implink, got: %s", out)
	}
}

// TestCLI_Check_ImplinkLine_PresentWhenUsed proves the fourth, non-blocking
// step actually reports once a module has an implementation-link artifact,
// and that it never affects check's own exit code even in the presence of
// drift/unlinked claims.
func TestCLI_Check_ImplinkLine_PresentWhenUsed(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	writeLockedFixtureClaim(t, claimsDir, "widget.contract.main", "widget", "assertion")
	writeLockedFixtureClaim(t, claimsDir, "widget.contract.unlinked", "widget", "never gets an implink Set call")

	srcPath := filepath.Join(root, "widget.go")
	if err := os.WriteFile(srcPath, []byte("package widget v1"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if _, _, err := execCLI(t, "--config", cfgPath, "implink", "set",
		"--module", "widget", "--claim", "widget.contract.main", "--file", "widget.go"); err != nil {
		t.Fatalf("implink set: %v", err)
	}

	// Mutate the linked file so check's report shows drift.
	if err := os.WriteFile(srcPath, []byte("package widget v2, mutated"), 0o644); err != nil {
		t.Fatalf("mutate source: %v", err)
	}

	out, _, err := execCLI(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("expected check to still succeed despite implink drift, got: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "check: OK") {
		t.Fatalf("expected check: OK regardless of implink drift, got: %s", out)
	}
	if !strings.Contains(out, "impl-links: 1 linked, 1 drifted, 1 unlinked-in-schema/behavior/api/verification-phases") {
		t.Fatalf("expected an impl-links summary line reporting the drift and the unlinked claim, got: %s", out)
	}
	if !strings.Contains(out, "drifted: widget.contract.main widget.go") {
		t.Fatalf("expected a per-drifted-entry detail line, got: %s", out)
	}
	if !strings.Contains(out, "unlinked: widget.contract.unlinked") {
		t.Fatalf("expected a per-unlinked-claim detail line naming which claim is unlinked, got: %s", out)
	}
}

// ---------------------------------------------------------------------
// docs-claim source scanning (source_dirs) — folded into "docs check"
// ---------------------------------------------------------------------

// icWriteScanFixtureProject writes a project.config.yaml with source_dirs
// set, one locked claim, and a "src/" directory ready for scan-tag source
// files to be dropped into by each test.
func icWriteScanFixtureProject(t *testing.T, root, module, claimID string) (cfgPath, srcDir string) {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	srcDir = filepath.Join(root, "src")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - " + module +
		"\nclaims_dir: claims\nsource_dirs:\n  - src\n"
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	writeLockedFixtureClaim(t, claimsDir, claimID, module, "fixture for source-scan tests")
	return cfgPath, srcDir
}

// TestCLI_Check_ScansSourceDirs_ReconcilesValidTag proves a "docs-claim:"
// comment alone — no "docs implink set" call at all — is enough for "docs
// check" to record the link and show it in impl-links reporting.
func TestCLI_Check_ScansSourceDirs_ReconcilesValidTag(t *testing.T) {
	root := t.TempDir()
	cfgPath, srcDir := icWriteScanFixtureProject(t, root, "widget", "widget.contract.main")
	if err := os.WriteFile(filepath.Join(srcDir, "main.py"),
		[]byte("# docs-claim: widget.contract.main\ndef do_thing():\n    pass\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	out, _, err := execCLI(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("expected check to succeed for a valid scanned tag, got: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "impl-links: scanned 1 file(s), found 1 tag(s), reconciled 1 link(s) (0 error(s))") {
		t.Fatalf("expected a scan summary line, got: %s", out)
	}
	if !strings.Contains(out, "check: OK") {
		t.Fatalf("expected check: OK, got: %s", out)
	}
	if !strings.Contains(out, "impl-links: 1 linked, 0 drifted, 0 unlinked-in-schema/behavior/api/verification-phases") {
		t.Fatalf("expected the status line to reflect the auto-reconciled link, got: %s", out)
	}
}

// TestCLI_Check_ScansSourceDirs_UnknownClaimIsHardFailure proves an
// unbacked/typo'd docs-claim tag fails "docs check" outright rather than
// being a silent or soft-warned note.
func TestCLI_Check_ScansSourceDirs_UnknownClaimIsHardFailure(t *testing.T) {
	root := t.TempDir()
	cfgPath, srcDir := icWriteScanFixtureProject(t, root, "widget", "widget.contract.main")
	if err := os.WriteFile(filepath.Join(srcDir, "main.py"),
		[]byte("# docs-claim: widget.contract.mian\ndef do_thing():\n    pass\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	out, errOut, err := execCLI(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatalf("expected check to fail for a docs-claim tag naming an unknown claim, got success (out: %s)", out)
	}
	if !strings.Contains(errOut, "widget.contract.mian") {
		t.Fatalf("expected the error to name the bad claim id, got stderr: %s", errOut)
	}
}

// TestCLI_Check_NextSteps_ListsDraftAndUnlinkedHints proves the "next
// steps" summary actually surfaces actionable hints derived from real
// check state, not just the raw per-section reports.
func TestCLI_Check_NextSteps_ListsDraftAndUnlinkedHints(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget") // one draft claim, by construction

	out, _, err := execCLI(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "next steps:") {
		t.Fatalf("expected a next steps block, got: %s", out)
	}
	if !strings.Contains(out, "claim(s) still draft -> docs lock <id>") {
		t.Fatalf("expected a draft-claims hint, got: %s", out)
	}
}
