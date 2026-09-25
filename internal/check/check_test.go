package check_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/manifest/manifesttest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

const baseConfig = "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\nmax_claims_per_module: 10000\n"

// project writes a project.config.yaml (cfgBody) plus every file in files
// (path relative to root -> content) and returns the loaded config and the
// loaded claims, the exact inputs check.Run takes (a caller would have
// reconciled review_pending first; these fixtures set status/review_pending
// directly so no reconcile is needed to exercise Run).
//
// It also ARMS THE LOCK LEDGER for every claim the fixture wrote as locked (see
// armLedger). Fixtures hand-write "status: locked" because reaching that state
// through the CLI would make every one of them a lifecycle test; the ledger
// gate, correctly, treats a locked claim with no approval record as tampering.
// Arming here says what the fixture means — "these claims are legitimately
// locked" — in one place, and leaves the gate free to fail loudly everywhere it
// has not been said. Tests that WANT a ledger finding tamper after this point;
// see ledger_test.go.
func project(t *testing.T, cfgBody string, files map[string]string) (*config.Config, []model.Claim) {
	t.Helper()
	return projectWithRoof(t, cfgBody, files, true)
}

// projectUnroofed is project without the locked constitution: the shape of a
// project that has not yet locked its roof, which is what the roof gate
// (NIT-26) refuses and what a test about "never ledger-covered" must now
// build, since the roof lock is the first ledger write a project makes.
func projectUnroofed(t *testing.T, cfgBody string, files map[string]string) (*config.Config, []model.Claim) {
	t.Helper()
	return projectWithRoof(t, cfgBody, files, false)
}

func projectWithRoof(t *testing.T, cfgBody string, files map[string]string, roof bool) (*config.Config, []model.Claim) {
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
	for _, module := range cfg.Modules {
		if err := manifesttest.WriteMinimal(cfg.ClaimsDir, module); err != nil {
			t.Fatalf("write module manifest %s: %v", module, err)
		}
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	if roof {
		armConstitution(t, cfg)
	}
	armLedger(t, cfg, claims)
	armDigestsIfCommented(t, cfg, claims)
	return cfg, claims
}

// armDigestsIfCommented records the comment digest for a fixture that hand-writes
// a `comments:` block, when there is one to record.
//
// It says what those fixtures mean, exactly as armLedger does for hand-written
// "status: locked": a comment thread only ever gets onto a claim through the
// engine, which records its digest as its last act — so "threads on disk, no
// digest store" is not a state the product produces. The gate reads that state
// as the digest store having been DELETED (comment-digest-absent), which is the
// one move that makes an edited-away review thread permanently invisible.
// Fixtures that want the absence report it deliberately; see ledger_test.go.
func armDigestsIfCommented(t *testing.T, cfg *config.Config, claims []model.Claim) {
	t.Helper()
	for _, c := range claims {
		if len(c.Comments) > 0 {
			armDigests(t, cfg, claims)
			return
		}
	}
}

func draftClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
		"body: |\n  a draft claim.\n" +
		"rests_on:\n  none: true\n  reason: fixture\n"
}

func lockedClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
		"body: |\n  a locked claim.\n" +
		"rests_on:\n  none: true\n  reason: fixture\n"
}

// lockedCodeClaim is a locked claim with code behind it: the shape the
// code-link gate holds to account. Every locked module claim is, unless it
// declares `embodiment: {mode: none}`.
func lockedCodeClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
		"body: |\n  a locked claim with code behind it.\n" +
		"rests_on:\n  none: true\n  reason: fixture\n"
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
		"claims/router.yaml": "id: widget.contract.router\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
			"body: |\n  start here.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
		"claims/one.yaml": draftClaim("widget.contract.one"),
		"claims/queue.yaml": "id: widget.internals.queue\nfacet: internals\nmodule: widget\nstatus: draft\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
			"body: |\n  an internals claim.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
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
	// catalog_count reports what catalog.json holds, and catalog.json omits
	// internals: 2 of the 3 claims.
	raw, readErr := os.ReadFile(res.CatalogPath)
	if readErr != nil {
		t.Fatalf("catalog not written at %s: %v", res.CatalogPath, readErr)
	}
	var written struct {
		Claims []json.RawMessage `json:"claims"`
	}
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatal(err)
	}
	if len(claims) != 3 || res.CatalogCount != 2 || len(written.Claims) != res.CatalogCount {
		t.Fatalf("CatalogCount=%d, catalog.json entries=%d, loaded claims=%d; want 2, 2, 3", res.CatalogCount, len(written.Claims), len(claims))
	}
	if _, statErr := os.Stat(res.RenderPath); statErr != nil {
		t.Fatalf("render not written at %s: %v", res.RenderPath, statErr)
	}
	// The paths resolve under cfg.Dir().
	if res.CatalogPath != filepath.Join(cfg.Dir(), "build", "catalog", "catalog.json") {
		t.Fatalf("catalog path %q not under cfg.Dir()", res.CatalogPath)
	}
	if res.RenderPath != filepath.Join(cfg.Dir(), "build", "viewer", "index.html") {
		t.Fatalf("render path %q not under cfg.Dir()", res.RenderPath)
	}

	foundDraftHint := false
	for _, h := range res.NextSteps {
		if h == "3 claim(s) still draft -> dossierx claim lock <id> --reason \"…\" (e.g. widget.contract.one)" {
			foundDraftHint = true
		}
	}
	if !foundDraftHint {
		t.Fatalf("expected the draft next-step hint, got %#v", res.NextSteps)
	}
}

func TestRun_MalformedFlagStoreStopsBeforeArtifacts(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	buildDir := cfg.BuildDirPath()
	catalogPath := filepath.Join(buildDir, "catalog", "catalog.json")
	viewerPath := filepath.Join(buildDir, "viewer", "index.html")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatalf("mkdir catalog output: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(viewerPath), 0o755); err != nil {
		t.Fatalf("mkdir viewer output: %v", err)
	}
	for _, tc := range []struct {
		path string
		body string
	}{
		{catalogPath, "catalog sentinel\n"},
		{viewerPath, "viewer sentinel\n"},
	} {
		if err := os.WriteFile(tc.path, []byte(tc.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", tc.path, err)
		}
	}
	flagPath := filepath.Join(buildDir, "ledger", "flag-store.json")
	if err := os.MkdirAll(filepath.Dir(flagPath), 0o755); err != nil {
		t.Fatalf("mkdir ledger output: %v", err)
	}
	if err := os.WriteFile(flagPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("write malformed flag store: %v", err)
	}

	if _, err := check.Run(claims, cfg); err == nil {
		t.Fatal("Run with malformed flag store succeeded")
	}
	for _, tc := range []struct {
		path string
		want string
	}{
		{catalogPath, "catalog sentinel\n"},
		{viewerPath, "viewer sentinel\n"},
	} {
		got, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s changed to %q, want sentinel %q", tc.path, got, tc.want)
		}
	}
}

// A lint error stops Run at the lint step: OK stays false, the error text is
// exactly what the CLI wraps "check: %w", and — critically — NEITHER side
// file is written (fail-fast happens before catalog/render).
func TestRun_LintErrorFailsFastNoWrites(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/broken.yaml": "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
			"body: |\n  broken.\n" +
			"rests_on:\n  - widget.contract.missing\n",
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
	if _, statErr := os.Stat(filepath.Join(cfg.Dir(), "build", "catalog", "catalog.json")); !os.IsNotExist(statErr) {
		t.Fatalf("expected NO build/catalog/catalog.json on lint fail-fast, stat=%v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(cfg.Dir(), "build", "viewer", "index.html")); !os.IsNotExist(statErr) {
		t.Fatalf("expected NO build/viewer/index.html on lint fail-fast, stat=%v", statErr)
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
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nreview_pending: true\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
			"body: |\n  a locked claim.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n" +
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
		if h == "1 claim(s) with open comment thread(s) -> the human resolves them in the viewer (dossierx serve); an agent may only reply (e.g. widget.contract.locked c-aaaaaa)" {
			foundCommentHint = true
		}
	}
	if !foundCommentHint {
		t.Fatalf("expected the open-comment next-step, got %#v", res.NextSteps)
	}
}

// A locked claim marked review_pending with NO active trigger — no open thread,
// no drifted dependency, no pending flag — is the state left behind when a
// drifted dependency is reverted, or an open thread is hand-resolved directly in
// YAML. It must STILL surface the reaudit next-step: v0.1.2 printed the reaudit
// hint for every locked+review_pending claim, and the trigger-partitioned
// nextSteps must not let a triggerless one fall into no bucket and silently
// vanish from the advisory.
func TestRun_TriggerlessReviewPendingReauditHint(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nreview_pending: true\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\n" +
			"body: |\n  a locked claim, review_pending with no active trigger.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})

	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// A triggerless review_pending claim STILL surfaces the reaudit next-step, but
	// it must be labeled ACCURATELY — "no active trigger", not "from drift/flag"
	// (there is neither a drifted dependency nor a pending flag).
	want := "1 claim(s) review_pending with no active trigger -> dossierx claim reaudit <id> (e.g. widget.contract.locked)"
	found := false
	mislabeled := false
	for _, h := range res.NextSteps {
		if h == want {
			found = true
		}
		if h == "1 claim(s) review_pending from drift/flag -> dossierx claim reaudit <id> (e.g. widget.contract.locked)" {
			mislabeled = true
		}
	}
	if mislabeled {
		t.Fatalf("triggerless review_pending claim mislabeled as \"from drift/flag\"; got %#v", res.NextSteps)
	}
	if !found {
		t.Fatalf("triggerless review_pending claim dropped from next-steps; expected %q, got %#v", want, res.NextSteps)
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
	if len(res.ImplinkStatusStdout) != 1 || res.ImplinkStatusStdout[0] != "impl-links: 1 linked, 0 drifted, 0 partial, 0 unlinked locked claims" {
		t.Fatalf("unexpected impl-link status stdout: %#v", res.ImplinkStatusStdout)
	}
}

func TestRun_StepTagScanAndStatus(t *testing.T) {
	hash := implink.StepContentHash("do the thing")
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: steps\nsummary: Fixture claim used by the engine test corpus.\n" +
			"steps:\n  - do the thing\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
		"src/impl.go": "package impl\n\n// dossierx-step: widget.contract.locked #1 " + hash + "\nfunc Foo() {}\n",
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
	if len(res.ImplinkStatusStdout) != 1 || res.ImplinkStatusStdout[0] != "impl-links: 1 linked, 0 drifted, 0 partial, 0 unlinked locked claims" {
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

// ---------------------------------------------------------------------
// The code-link gate (issue #78). With source_dirs set, a locked
// schema/behavior/api/verification claim with no link — or a stepped claim
// not tagged on every step — fails Run AFTER the catalog and viewer were
// written and AFTER the ledger gate. Without source_dirs nothing is refused.
// ---------------------------------------------------------------------

func TestRun_CodeLinkGate_RefusesUnlinkedClaim(t *testing.T) {
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml": lockedCodeClaim("widget.contract.locked"),
		"src/impl.go":        "package impl\n\nfunc Foo() {}\n", // no tag anywhere
	})

	res, err := check.Run(claims, cfg)
	if err == nil || err.Error() != "code links: 1 claim(s) not linked" {
		t.Fatalf("expected the code-link gate to refuse, got err=%v", err)
	}
	if !res.CodeLinkGateFailed || res.OK {
		t.Fatalf("expected CodeLinkGateFailed and !OK, got %+v", res)
	}
	if res.CatalogPath == "" || res.RenderPath == "" {
		t.Fatalf("the gate must refuse AFTER the projections landed; catalog=%q render=%q", res.CatalogPath, res.RenderPath)
	}
	if len(res.LedgerFindings) != 0 || len(res.ScanErrors) != 0 {
		t.Fatalf("no earlier gate should have fired: %+v", res)
	}
	if res.CodeLinks == nil || !res.CodeLinks.Scanned || !res.CodeLinks.Gated {
		t.Fatalf("a plain check with source_dirs is scanned and gated, got %+v", res.CodeLinks)
	}
	if res.CodeLinks.Incomplete() != 1 || len(res.CodeLinks.Modules) != 1 || len(res.CodeLinks.Modules[0].Unlinked) != 1 || res.CodeLinks.Modules[0].Unlinked[0] != "widget.contract.locked" {
		t.Fatalf("expected the one unlinked claim named, got %+v", res.CodeLinks.Modules)
	}
	if _, statErr := os.Stat(res.RenderPath); statErr != nil {
		t.Fatalf("viewer must exist on disk after a link refusal: %v", statErr)
	}
}

func TestRun_CodeLinkGate_RefusesPartialSteps(t *testing.T) {
	hash := implink.StepContentHash("do the thing")
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: steps\nsummary: Fixture claim used by the engine test corpus.\n" +
			"steps:\n  - do the thing\n  - do the other thing\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
		"src/impl.go": "package impl\n\n// dossierx-step: widget.contract.locked #1 " + hash + "\nfunc Foo() {}\n",
	})

	res, err := check.Run(claims, cfg)
	if err == nil || err.Error() != "code links: 1 claim(s) not linked" {
		t.Fatalf("expected the gate to refuse a 1-of-2 stepped claim, got err=%v", err)
	}
	if !res.CodeLinkGateFailed {
		t.Fatalf("expected CodeLinkGateFailed, got %+v", res)
	}
	m := res.CodeLinks.Modules[0]
	if len(m.Unlinked) != 0 || len(m.Partial) != 1 || m.Partial[0].Covered != 1 || m.Partial[0].Total != 2 || len(m.Partial[0].Missing) != 1 || m.Partial[0].Missing[0] != 2 {
		t.Fatalf("expected partial 1 of 2 missing step 2, got %+v", m)
	}
}

func TestRun_CodeLinkGate_PassesWhenEveryClaimLinked(t *testing.T) {
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml":  lockedCodeClaim("widget.contract.locked"),
		"claims/context.yaml": "id: widget.contract.context\nfacet: contract\nmodule: widget\nstatus: locked\nsummary: Fixture claim used by the engine test corpus.\nlayout: card\nembodiment:\n  mode: none\n  reason: context only, no code\nbody: |\n  context, no code.\nrests_on:\n  none: true\n  reason: fixture\n",
		"src/impl.go":         "package impl\n\n// dossierx-claim: widget.contract.locked\nfunc Foo() {}\n",
	})

	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.OK || res.CodeLinkGateFailed {
		t.Fatalf("expected OK, got %+v", res)
	}
	if res.CodeLinks == nil || !res.CodeLinks.Gated || res.CodeLinks.Incomplete() != 0 {
		t.Fatalf("expected a gated, complete report; the code-free claim is not expected to link: %+v", res.CodeLinks)
	}
}

// Without source_dirs there is no gate: a project may hold `claim link`
// artifacts and unlinked locked claims side by side and still exit 0, because
// nothing told the engine where the code is.
func TestRun_CodeLinkGate_InactiveWithoutSourceDirs(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedCodeClaim("widget.contract.locked"),
	})
	res, err := check.Run(claims, cfg)
	if err != nil || !res.OK {
		t.Fatalf("expected OK without source_dirs, got err=%v res=%+v", err, res)
	}
	if res.CodeLinks != nil {
		t.Fatalf("a project that never opted in must carry no code_links report, got %+v", res.CodeLinks)
	}
}

// --validate and --staged read the stored artifact and refuse nothing: the
// same counts arrive with Scanned=false and Gated=false, so a consumer can
// never read a read-only green as a linked green.
func TestStatus_CodeLinks_ReportedButNeverGated(t *testing.T) {
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml": lockedCodeClaim("widget.contract.locked"),
		"src/impl.go":        "package impl\n\nfunc Foo() {}\n",
	})
	res := check.Status(claims, cfg)
	if !res.OK || res.CodeLinkGateFailed {
		t.Fatalf("Status never refuses on links, got %+v", res)
	}
	if res.CodeLinks == nil || res.CodeLinks.Scanned || res.CodeLinks.Gated {
		t.Fatalf("expected an unscanned, ungated report, got %+v", res.CodeLinks)
	}
	if res.CodeLinks.Incomplete() != 1 || res.CodeLinks.Modules[0].Unlinked[0] != "widget.contract.locked" {
		t.Fatalf("the unlinked claim must still be named, got %+v", res.CodeLinks.Modules)
	}
}

// A tampered locked claim AND an unlinked one: the ledger gate answers first,
// and the link gate is never reached — a refusal about where the code is must
// not hide a refusal about whether the claim was approved.
func TestRun_LedgerFindingPrecedesCodeLinkGate(t *testing.T) {
	cfg, claims := project(t, baseConfig+"source_dirs:\n  - src\n", map[string]string{
		"claims/locked.yaml": lockedCodeClaim("widget.contract.locked"),
		"src/impl.go":        "package impl\n\nfunc Foo() {}\n",
	})
	claims[0].Body = "a locked claim, quietly rewritten.\n"

	res, err := check.Run(claims, cfg)
	if err == nil || len(res.LedgerFindings) == 0 {
		t.Fatalf("expected the ledger gate to refuse first, got err=%v findings=%v", err, rulesOf(res.LedgerFindings))
	}
	if res.CodeLinkGateFailed || res.CodeLinks != nil {
		t.Fatalf("the link gate must not run after a ledger refusal, got failed=%v links=%+v", res.CodeLinkGateFailed, res.CodeLinks)
	}
}
