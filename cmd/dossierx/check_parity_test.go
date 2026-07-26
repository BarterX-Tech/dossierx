// check_parity_test.go is the byte-for-byte guard for "dossierx check"'s
// terminal output. Phase 4a extracts the check pipeline out of newCheckCmd's
// RunE into a value-returning internal/check.Run, and rewires the command to
// format that Result for the terminal. The point of the extraction is that the
// CLI's observable behavior — every byte of stdout, every byte of stderr, and
// whether the command failed — is preserved. These golden fixtures pin the
// reporter's CURRENT intended output exactly (==, not Contains), so any silent
// drift in the reporter, the fail-fast ordering, or the next-steps advisory
// fails the build.
//
// Scope of the "vs v0.1.2" guarantee — one deliberate exception. The output is
// byte-identical to the pre-extraction command EXCEPT for a single, intentional
// wording improvement: the drift/flag reaudit next-step now reads "N claim(s)
// review_pending from drift/flag -> dossierx reaudit <id> ..." (see check.Run's
// nextSteps and fixtures F and I). That one-line reword is the ONLY byte-diff
// from v0.1.2 and is captured in the goldens below as the intended text — the
// goldens are the source of truth for the current reporter, not a promise that
// every byte still matches v0.1.2 verbatim. Everything else is unchanged.
//
// The fixtures span every output segment check can print: the lint block
// (warnings and the error/fail-fast path), the catalog+render write lines,
// the impl-links scan summary and per-module status line, "check: OK", the
// orientation-notes / open-comments per-module summaries, and each next-steps
// hint (draft, open-comment-thread, drift/flag reaudit including the
// triggerless drift-then-revert catch-all, and the fully-locked build-order
// prompt).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCheckFixture writes project.config.yaml (cfgBody already includes the
// trailing newline) plus every claim/source file in files (path relative to
// root -> content), returning the config path.
func writeCheckFixture(t *testing.T, root, cfgBody string, files map[string]string) string {
	t.Helper()
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return cfgPath
}

// assertCheckParity runs "dossierx check" against cfgPath and asserts stdout,
// stderr, and the pass/fail outcome match the golden values exactly.
func assertCheckParity(t *testing.T, cfgPath, wantStdout, wantStderr string, wantErr bool) {
	t.Helper()
	stdout, stderr, err := execCLI(t, "--config", cfgPath, "check")
	if (err != nil) != wantErr {
		t.Fatalf("check error outcome: got err=%v, wantErr=%v\nstdout:\n%s\nstderr:\n%s", err, wantErr, stdout, stderr)
	}
	if stdout != wantStdout {
		t.Fatalf("check stdout drift:\n--- got ----\n%q\n--- want ---\n%q", stdout, wantStdout)
	}
	if stderr != wantStderr {
		t.Fatalf("check stderr drift:\n--- got ----\n%q\n--- want ---\n%q", stderr, wantStderr)
	}
}

const parityConfig = "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"

// A: draft claims plus an overview (orientation-note) claim — exercises the
// lint-warning block, catalog/render writes, "check: OK", the orientation
// summary line, and the "still draft -> lock" next step.
func TestCheckParity_DraftAndOrientation(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/overview.yaml": "id: widget.overview.router\nfacet: overview\nmodule: widget\nstatus: draft\nlayout: banner\n" +
			"body: |\n  fixture orientation-note claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/c1.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  fixture claim one.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	catalog := filepath.Join(root, ".catalog.json")
	viewer := filepath.Join(root, "viewer", "index.html")
	want := "[warning] orphan: widget.contract.one: claim has no mirrors/rests_on edges in either direction\n" +
		"[warning] orphan: widget.overview.router: claim has no mirrors/rests_on edges in either direction\n" +
		"lint: 2 finding(s), 0 error(s)\n" +
		"catalog: wrote " + catalog + " (2 claim(s))\n" +
		"render: wrote " + viewer + "\n" +
		"check: OK\n" +
		"orientation notes: module \"widget\": 1 (1 in overview)\n" +
		"next steps:\n" +
		"  2 claim(s) still draft -> dossierx lock <id> (e.g. widget.contract.one)\n"
	assertCheckParity(t, cfgPath, want, "", false)
}

// B: a locked claim carrying an open comment thread — exercises the
// comments-unresolved warning, the "open comments" per-module summary, and
// the "open comment thread(s) -> comment resolve" next step.
func TestCheckParity_LockedWithOpenThread(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"body: |\n  a locked claim with an open comment thread.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n",
	})
	catalog := filepath.Join(root, ".catalog.json")
	viewer := filepath.Join(root, "viewer", "index.html")
	want := "[warning] comments-unresolved: widget.contract.locked: 1 unresolved comment thread(s)\n" +
		"[warning] orphan: widget.contract.locked: claim has no mirrors/rests_on edges in either direction\n" +
		"lint: 2 finding(s), 0 error(s)\n" +
		"catalog: wrote " + catalog + " (1 claim(s))\n" +
		"render: wrote " + viewer + "\n" +
		"check: OK\n" +
		"open comments: module \"widget\": 1\n" +
		"next steps:\n" +
		"  1 claim(s) with open comment thread(s) -> dossierx comment resolve <id> <thread-id> (e.g. widget.contract.locked c-aaaaaa)\n"
	assertCheckParity(t, cfgPath, want, "", false)
}

// C: a dangling rests_on target — the error-severity lint finding that makes
// check fail fast: the lint block prints, then the command returns an error
// (cobra prints the "Error:" line to stderr) and nothing past lint runs.
func TestCheckParity_LintErrorFailsFast(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/broken.yaml": "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  broken fixture.\n" +
			"rests_on:\n  - widget.contract.does-not-exist\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	wantStdout := "[error] dangling: widget.contract.broken: rests_on references unknown claim id widget.contract.does-not-exist\n" +
		"lint: 1 finding(s), 1 error(s)\n"
	wantStderr := "Error: check: lint: 1 error-level finding(s)\n"
	assertCheckParity(t, cfgPath, wantStdout, wantStderr, true)
	// Fail-fast must have stopped before catalog/render: neither file exists.
	if _, err := os.Stat(filepath.Join(root, ".catalog.json")); !os.IsNotExist(err) {
		t.Fatalf("expected NO .catalog.json after a lint-error check, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "viewer", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected NO viewer/index.html after a lint-error check, stat err=%v", err)
	}
}

// D: a single fully-locked claim, no comments — exercises the "fully locked
// with no build order yet -> build-order propose" next step.
func TestCheckParity_FullyLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"body: |\n  a fully locked claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	catalog := filepath.Join(root, ".catalog.json")
	viewer := filepath.Join(root, "viewer", "index.html")
	want := "[warning] orphan: widget.contract.locked: claim has no mirrors/rests_on edges in either direction\n" +
		"lint: 1 finding(s), 0 error(s)\n" +
		"catalog: wrote " + catalog + " (1 claim(s))\n" +
		"render: wrote " + viewer + "\n" +
		"check: OK\n" +
		"next steps:\n" +
		"  module \"widget\" is fully locked with no build order yet -> dossierx build-order propose --module widget\n"
	assertCheckParity(t, cfgPath, want, "", false)
}

// E: a locked claim linked from a tagged source file — exercises the
// impl-links scan summary (a fourth fail-fast step) and the impl-links
// status line printed after "check: OK".
func TestCheckParity_ImplinkScanAndStatus(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"body: |\n  a locked claim linked from code.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"src/impl.go": "package impl\n\n// dossierx-claim: widget.contract.locked\nfunc Foo() {}\n",
	})
	catalog := filepath.Join(root, ".catalog.json")
	viewer := filepath.Join(root, "viewer", "index.html")
	want := "[warning] orphan: widget.contract.locked: claim has no mirrors/rests_on edges in either direction\n" +
		"lint: 1 finding(s), 0 error(s)\n" +
		"catalog: wrote " + catalog + " (1 claim(s))\n" +
		"render: wrote " + viewer + "\n" +
		"impl-links: scanned 1 file(s), found 1 tag(s), reconciled 1 link(s) (0 error(s))\n" +
		"check: OK\n" +
		"impl-links: 1 linked, 0 drifted, 0 unlinked-in-schema/behavior/api/verification-phases\n" +
		"next steps:\n" +
		"  module \"widget\" is fully locked with no build order yet -> dossierx build-order propose --module widget\n"
	assertCheckParity(t, cfgPath, want, "", false)
}

// G: a draft claim carrying raw_html on a non-mockup layout — the raw-html-scope
// lint (which itself runs the mockup gate) reports three error-severity findings,
// so check fails at the LINT step. This pins the fact that check's raw_html
// enforcement happens through lint, before catalog's own mockup gate is ever
// reached — the reason check.Run need not re-run that gate.
func TestCheckParity_RawHTMLFailsAtLint(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/mock.yaml": "id: widget.contract.mock\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a claim.\nraw_html: \"<b>hi</b>\"\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	wantStdout := "[warning] orphan: widget.contract.mock: claim has no mirrors/rests_on edges in either direction\n" +
		"[error] raw-html-scope: widget.contract.mock: raw_html is only legal on layout: mockup, got layout: \"card\"\n" +
		"[error] raw-html-scope: widget.contract.mock: module \"widget\" is not in the project's mockup_modules allowlist and may not author layout: mockup / raw_html claims\n" +
		"[error] raw-html-scope: widget.contract.mock: raw_html is set but raw_html_reviewed is not true; a human must review this markup and set raw_html_reviewed: true before this claim can lock\n" +
		"lint: 4 finding(s), 3 error(s)\n"
	wantStderr := "Error: check: lint: 3 error-level finding(s)\n"
	assertCheckParity(t, cfgPath, wantStdout, wantStderr, true)
}

// H: a draft claim tagged by a source file under source_dirs — the impl-link
// scan cannot reconcile a tag naming a not-locked claim, so check fails at the
// SCAN step (the fourth fail-fast step). Uniquely this exercises the ordering
// where catalog+render already wrote (stdout) and then the scan errors print
// (stderr) ahead of the wrapped "check:" error — the one formatCheckResult
// path that emits to stderr after a run of successful steps. The golden below
// was verified byte-for-byte against the pre-extraction command.
func TestCheckParity_ScanErrorFailsAtScan(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/draft.yaml": "id: widget.contract.draft\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a draft claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"src/impl.go": "package impl\n\n// dossierx-claim: widget.contract.draft\nfunc Foo() {}\n",
	})
	catalog := filepath.Join(root, ".catalog.json")
	viewer := filepath.Join(root, "viewer", "index.html")
	wantStdout := "[warning] orphan: widget.contract.draft: claim has no mirrors/rests_on edges in either direction\n" +
		"lint: 1 finding(s), 0 error(s)\n" +
		"catalog: wrote " + catalog + " (1 claim(s))\n" +
		"render: wrote " + viewer + "\n"
	// The scan-error line uses the source-relative path (src/impl.go), so no
	// temp-root substitution is needed here.
	wantStderr := "impl-links: scan error in src/impl.go:3: dossierx-claim references \"widget.contract.draft\": claim is not locked (status \"draft\")\n" +
		"Error: check: 1 impl-link scan error(s)\n"
	assertCheckParity(t, cfgPath, wantStdout, wantStderr, true)
}

// F: a locked claim flagged for reaudit — exercises the "review_pending from
// drift/flag -> reaudit" next step (distinct from B's comment-thread hint).
func TestCheckParity_DriftFlagReauditHint(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"body: |\n  a locked claim to flag.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	if _, _, err := execCLI(t, "--config", cfgPath, "flag", "widget.contract.locked",
		"--claim-says", "old", "--now-does", "new", "--reason", "changed"); err != nil {
		t.Fatalf("flag setup: %v", err)
	}
	catalog := filepath.Join(root, ".catalog.json")
	viewer := filepath.Join(root, "viewer", "index.html")
	want := "[warning] orphan: widget.contract.locked: claim has no mirrors/rests_on edges in either direction\n" +
		"lint: 1 finding(s), 0 error(s)\n" +
		"catalog: wrote " + catalog + " (1 claim(s))\n" +
		"render: wrote " + viewer + "\n" +
		"check: OK\n" +
		"next steps:\n" +
		"  1 claim(s) review_pending from drift/flag -> dossierx reaudit <id> (e.g. widget.contract.locked)\n"
	assertCheckParity(t, cfgPath, want, "", false)
}

// I: a GENUINE drift-then-revert — a dependency drifts (setting review_pending
// on its dependent), then is reverted byte-identically so the dependent is no
// longer drifted, yet stays review_pending (only reaudit --confirm / unlock /
// resolving a thread clears it). The dependent is then locked+review_pending
// with NO active trigger (no drift, no flag, no open thread), and the reaudit
// next-step MUST still print — the triggerless catch-all. Distinct from F
// (which keeps a live flag trigger): here every trigger is cleared, so a naive
// trigger-partition would drop the claim from next-steps entirely.
func TestCheckParity_DriftThenRevertReauditHint(t *testing.T) {
	root := t.TempDir()
	baseV1 := "id: widget.contract.base\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"body: |\n  base body, version one.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	baseV2 := "id: widget.contract.base\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"body: |\n  base body, version TWO — changed to drift the dependent.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	dep := "id: widget.contract.dep\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  the dependent claim.\n" +
		"rests_on:\n  - widget.contract.base\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/base.yaml": baseV1,
		"claims/dep.yaml":  dep,
	})
	basePath := filepath.Join(root, "claims", "base.yaml")

	// Lock dep (base is authored locked already) so the store captures dep's
	// per-dependent baseline for base = ContentHash(base v1).
	if _, stderr, err := execCLI(t, "--config", cfgPath, "lock", "widget.contract.dep"); err != nil {
		t.Fatalf("lock dep: %v (stderr=%s)", err, stderr)
	}
	// Drift base: change its body so dep's stored baseline no longer matches.
	if err := os.WriteFile(basePath, []byte(baseV2), 0o644); err != nil {
		t.Fatalf("drift base: %v", err)
	}
	// A check run reconciles dep -> locked+review_pending (drift detected).
	if _, stderr, err := execCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("drift check: %v (stderr=%s)", err, stderr)
	}
	// Revert base to v1 (byte-identical body) so ContentHash matches the stored
	// baseline again: dep is no longer drifted, but review_pending persists.
	if err := os.WriteFile(basePath, []byte(baseV1), 0o644); err != nil {
		t.Fatalf("revert base: %v", err)
	}

	catalog := filepath.Join(root, ".catalog.json")
	viewer := filepath.Join(root, "viewer", "index.html")
	// The dependent is review_pending with NO active trigger (drift reverted, no
	// flag, no open thread), so the reaudit hint must be labeled "no active
	// trigger" — NOT "from drift/flag" (F's live-flag case keeps that label).
	want := "lint: 0 findings\n" +
		"catalog: wrote " + catalog + " (2 claim(s))\n" +
		"render: wrote " + viewer + "\n" +
		"check: OK\n" +
		"next steps:\n" +
		"  1 claim(s) review_pending with no active trigger -> dossierx reaudit <id> (e.g. widget.contract.dep)\n"
	assertCheckParity(t, cfgPath, want, "", false)
}

// A malformed claim YAML must fail "dossierx check" with the load error reported
// as "Error: load claims: ..." — NOT "Error: check: load claims: ...". The
// claims load is a precondition that predates the check pipeline, so its error
// keeps v0.1.2's unprefixed shape; the "check:" wrap belongs only to the
// pipeline steps. The yaml message and temp path vary, so this pins the stable
// error PREFIX rather than full byte-parity.
func TestCheckParity_MalformedClaimLoadErrorPrefix(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		// An unclosed YAML flow sequence: a hard parse error in loader.LoadClaims.
		"claims/broken.yaml": "id: [oops\n",
	})
	stdout, stderr, err := execCLI(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatalf("expected check to fail on a malformed claim; stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.HasPrefix(stderr, "Error: check:") {
		t.Fatalf("regression: claims-load error wrapped with a \"check:\" prefix: %q", stderr)
	}
	if !strings.HasPrefix(stderr, "Error: load claims: ") {
		t.Fatalf("claims-load error must be reported unprefixed as \"Error: load claims: ...\"; got %q", stderr)
	}
}
