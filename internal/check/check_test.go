package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

const baseConfig = "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"

// project writes a project.config.yaml (cfgBody) plus every file in files
// (path relative to root -> content) and returns the loaded config and the
// loaded claims, the exact inputs check.Run takes (a caller would have
// reconciled review_pending first; these fixtures set status/review_pending
// directly so no reconcile is needed to exercise Run).
func project(t *testing.T, cfgBody string, files map[string]string) (*config.Config, []model.Claim) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	cfg, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	return cfg, claims
}

func draftClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
}

func lockedClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"body: |\n  a locked claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
}

func severities(findings []lint.Finding) map[lint.Severity]int {
	m := map[lint.Severity]int{}
	for _, f := range findings {
		m[f.Severity]++
	}
	return m
}

// A happy run: no lint errors, so Run reaches OK, writes both side files, and
// populates the reporting fields.
func TestRun_SuccessWritesAndReports(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/overview.yaml": "id: widget.overview.router\nfacet: overview\nmodule: widget\nstatus: draft\nlayout: banner\n" +
			"body: |\n  orientation note.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})

	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected OK=true on a clean run")
	}
	if len(res.LintErrors) != 0 {
		t.Fatalf("expected zero lint errors, got %d: %v", len(res.LintErrors), res.LintErrors)
	}
	if len(res.LintWarnings) == 0 {
		t.Fatalf("expected at least one warning (orphan), got none")
	}
	// LintFindings is the full slice; its severities must equal the split.
	sev := severities(res.LintFindings)
	if sev[lint.SeverityWarning] != len(res.LintWarnings) {
		t.Fatalf("LintWarnings (%d) disagrees with LintFindings warning count (%d)", len(res.LintWarnings), sev[lint.SeverityWarning])
	}
	if got := len(res.LintFindings) - sev[lint.SeverityWarning]; got != len(res.LintErrors) {
		t.Fatalf("LintErrors (%d) disagrees with non-warning finding count (%d)", len(res.LintErrors), got)
	}

	// Both side files were actually written to the paths Run reported.
	if res.CatalogPath == "" || res.RenderPath == "" {
		t.Fatalf("expected catalog+render paths to be set, got %q / %q", res.CatalogPath, res.RenderPath)
	}
	if res.CatalogCount != len(claims) {
		t.Fatalf("expected CatalogCount=%d, got %d", len(claims), res.CatalogCount)
	}
	if _, statErr := os.Stat(res.CatalogPath); statErr != nil {
		t.Fatalf("catalog not written at %s: %v", res.CatalogPath, statErr)
	}
	if _, statErr := os.Stat(res.RenderPath); statErr != nil {
		t.Fatalf("render not written at %s: %v", res.RenderPath, statErr)
	}
	// The paths resolve under cfg.Dir().
	if res.CatalogPath != filepath.Join(cfg.Dir(), ".catalog.json") {
		t.Fatalf("catalog path %q not under cfg.Dir()", res.CatalogPath)
	}
	if res.RenderPath != filepath.Join(cfg.Dir(), "viewer", "index.html") {
		t.Fatalf("render path %q not under cfg.Dir()", res.RenderPath)
	}

	// Orientation + next-steps reporting present.
	if len(res.OrientationNotes) != 1 || res.OrientationNotes[0] != "orientation notes: module \"widget\": 1 (1 in overview)" {
		t.Fatalf("unexpected orientation notes: %#v", res.OrientationNotes)
	}
	foundDraftHint := false
	for _, h := range res.NextSteps {
		if h == "2 claim(s) still draft -> dossierx lock <id> (e.g. widget.contract.one)" {
			foundDraftHint = true
		}
	}
	if !foundDraftHint {
		t.Fatalf("expected the draft next-step hint, got %#v", res.NextSteps)
	}
}

// A lint error stops Run at the lint step: OK stays false, the error text is
// exactly what the CLI wraps "check: %w", and — critically — NEITHER side
// file is written (fail-fast happens before catalog/render).
func TestRun_LintErrorFailsFastNoWrites(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/broken.yaml": "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  broken.\n" +
			"rests_on:\n  - widget.contract.missing\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})

	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected a lint error, got nil")
	}
	if err.Error() != "lint: 1 error-level finding(s)" {
		t.Fatalf("lint error text drift: %q", err.Error())
	}
	if res.OK {
		t.Fatalf("expected OK=false on a lint error")
	}
	if len(res.LintErrors) != 1 {
		t.Fatalf("expected 1 lint error in Result, got %d", len(res.LintErrors))
	}
	if res.CatalogPath != "" || res.RenderPath != "" {
		t.Fatalf("expected no catalog/render paths on fail-fast, got %q / %q", res.CatalogPath, res.RenderPath)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.Dir(), ".catalog.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected NO .catalog.json on lint fail-fast, stat=%v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.Dir(), "viewer", "index.html")); !os.IsNotExist(statErr) {
		t.Fatalf("expected NO viewer/index.html on lint fail-fast, stat=%v", statErr)
	}
}

// An open comment thread on a locked claim surfaces in OpenComments (per
// module) and drives the comment-resolution next step. The claim is passed
// already locked + review_pending — the post-reconcile state a caller feeds
// Run (reconcileReviewPending flips a locked claim with an open thread to
// review_pending before Run sees it); the comment next-step partitions the
// review_pending claims by trigger, so without that flag the summary count
// still shows but the hint does not, exactly as in the CLI.
func TestRun_OpenCommentsReported(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nreview_pending: true\nlayout: card\n" +
			"body: |\n  a locked claim.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n",
	})

	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.OpenComments["widget"]; got != 1 {
		t.Fatalf("expected OpenComments[widget]=1, got %d (map=%#v)", got, res.OpenComments)
	}
	foundCommentHint := false
	for _, h := range res.NextSteps {
		if h == "1 claim(s) with open comment thread(s) -> dossierx comment resolve <id> <thread-id> (e.g. widget.contract.locked c-aaaaaa)" {
			foundCommentHint = true
		}
	}
	if !foundCommentHint {
		t.Fatalf("expected the open-comment next-step, got %#v", res.NextSteps)
	}
}

// A fully-locked module with no build-order artifact yet drives the
// build-order propose next step and leaves OpenComments empty.
func TestRun_FullyLockedBuildOrderHint(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})

	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.OpenComments) != 0 {
		t.Fatalf("expected no open comments, got %#v", res.OpenComments)
	}
	foundBuildOrderHint := false
	for _, h := range res.NextSteps {
		if h == "module \"widget\" is fully locked with no build order yet -> dossierx build-order propose --module widget" {
			foundBuildOrderHint = true
		}
	}
	if !foundBuildOrderHint {
		t.Fatalf("expected the build-order next-step, got %#v", res.NextSteps)
	}
}

// A tagged source file under source_dirs makes Run scan and report impl-links:
// ScanFilesScanned/ScanSummary are set and the status line is produced.
func TestRun_ImplinkScanAndStatus(t *testing.T) {
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
		"src/impl.go":        "package impl\n\n// dossierx-claim: widget.contract.locked\nfunc Foo() {}\n",
	})

	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ScanFilesScanned != 1 {
		t.Fatalf("expected 1 file scanned, got %d", res.ScanFilesScanned)
	}
	if res.ScanSummary != "impl-links: scanned 1 file(s), found 1 tag(s), reconciled 1 link(s) (0 error(s))" {
		t.Fatalf("scan summary drift: %q", res.ScanSummary)
	}
	if len(res.ScanErrors) != 0 {
		t.Fatalf("expected no scan errors, got %#v", res.ScanErrors)
	}
	if len(res.ImplinkStatusStdout) != 1 || res.ImplinkStatusStdout[0] != "impl-links: 1 linked, 0 drifted, 0 unlinked-in-schema/behavior/api/verification-phases" {
		t.Fatalf("unexpected impl-link status stdout: %#v", res.ImplinkStatusStdout)
	}
}

// A dossierx-claim tag naming a claim that is not locked is a hard scan error:
// Run records it in ScanErrors and returns the "N impl-link scan error(s)"
// error the CLI wraps "check: %w" — while catalog/render were still written
// (the scan step runs after them).
func TestRun_ScanErrorSurfaced(t *testing.T) {
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		// Draft (not locked): a tag pointing at it cannot reconcile.
		"claims/draft.yaml": draftClaim("widget.contract.draft"),
		"src/impl.go":       "package impl\n\n// dossierx-claim: widget.contract.draft\nfunc Foo() {}\n",
	})

	res, err := check.Run(claims, cfg)
	if err == nil {
		t.Fatalf("expected a scan error, got nil")
	}
	if err.Error() != "1 impl-link scan error(s)" {
		t.Fatalf("scan error text drift: %q", err.Error())
	}
	if res.OK {
		t.Fatalf("expected OK=false when the scan has errors")
	}
	if len(res.ScanErrors) != 1 {
		t.Fatalf("expected 1 recorded scan error, got %d", len(res.ScanErrors))
	}
	// catalog/render precede the scan step, so both were written before it failed.
	if res.CatalogPath == "" || res.RenderPath == "" {
		t.Fatalf("expected catalog/render written before the scan failure, got %q / %q", res.CatalogPath, res.RenderPath)
	}
}
