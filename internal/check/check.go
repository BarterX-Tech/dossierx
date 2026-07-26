// Package check is the value-returning core of the "dossierx check" pipeline:
// lint, catalog build+write, viewer render+write, impl-link scan, and the
// per-module non-blocking reporting (orientation notes, open comments,
// impl-link status, and the next-steps advisory). It exists so BOTH the
// "dossierx check" CLI command and "dossierx serve" can drive the same
// pipeline without duplicating it — the CLI (cmd/dossierx) formats the
// returned Result for the terminal under its fail-fast contract (stop at the
// first failing step, map to a nonzero exit), while serve reuses the same
// Result as page data WITHOUT that contract (a lint error is something to
// show, not a reason to refuse the page).
//
// Run performs exactly the work newCheckCmd's RunE used to inline, in the
// same order and with the same fail-fast semantics and the same side effects
// (it writes .catalog.json and viewer/index.html and reconciles impl-links),
// but instead of printing it records every datum the terminal reporter needs
// in Result and returns the first step error (unprefixed; the caller wraps it
// "check: %w", exactly as the RunE did at each call site). A caller that
// wants byte-identical CLI output prints Result's segments in field order and
// returns the wrapped error; see cmd/dossierx's formatCheckResult.
//
// Since v0.3.0 the pipeline has one more step: the LOCK-LEDGER GATE, which
// compares the claims against the approval records and the comment digests and
// refuses a run that finds a disagreement. It is a gate rather than a lint, and
// it runs LAST — after the catalog and the viewer are on disk — so a tampered
// claim costs a project its exit status and not its documentation. See
// ledger.go. Run enforces it; Status reports it and lets its caller decide,
// which is what lets "dossierx serve" keep rendering a disputed project.
//
// StatusStaged (staged.go) is the third entry point: Status evaluated against
// the GIT INDEX rather than the working tree, which is what "dossierx check
// --staged" and the pre-commit hook run.
//
// Run does NOT load or reconcile claims or acquire the claims sentinel: it
// takes the already-reconciled claim slice its caller produced under that
// sentinel (cmd/dossierx.reconcileReviewPending for the CLI; a startup
// reconcile for serve), keeping all claim-file persistence — and the Phase-0
// claims-lock discipline that guards it — in the caller where the sentinel is
// held.
package check

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
	"github.com/BarterX-Tech/dossierx/internal/render"
)

// Result is the value form of one "dossierx check" run — everything the
// terminal reporter prints and everything serve shows as page data, computed
// once by Run. Fields are populated in pipeline order and left zero for any
// step the run stopped before reaching, so a formatter can reproduce the
// fail-fast CLI output by emitting only the segments that are present.
type Result struct {
	// LintFindings is lint.RunAll's output verbatim, in Registry order — the
	// order the terminal prints them (a mixed error/warning run interleaves by
	// registry, not by severity, so this full slice, not the split ones below,
	// is what a byte-identical printer must iterate). LintErrors and
	// LintWarnings are its severity partition (each preserving Registry order):
	// LintErrors is every finding that is NOT SeverityWarning — matching the
	// exit-code count reportLintFindings uses — and its length drives the
	// fail-fast lint error. Always populated (lint is the first step).
	LintFindings []lint.Finding
	LintErrors   []lint.Finding
	LintWarnings []lint.Finding

	// CatalogPath/CatalogCount and RenderPath record check's two disk writes.
	// CatalogPath/RenderPath are empty when the run stopped before that write
	// (e.g. a lint error), which is exactly the "did this line print?" test.
	CatalogPath  string
	CatalogCount int
	RenderPath   string

	// ScanFilesScanned/ScanSummary/ScanErrors capture the impl-link scan.
	// ScanErrors is printed (to stderr) whether or not the run then failed;
	// the summary prints only on success (see OK), matching the pre-extraction
	// order where the summary followed a clean scan and preceded "check: OK".
	ScanFilesScanned int
	ScanSummary      string
	ScanErrors       []implink.ScanError

	// LedgerFindings is the lock-ledger gate's verdict: every claim whose
	// locked state or comment history disagrees with the approval record (see
	// ledger.go). It is NOT a lint partition — there is no severity, because
	// every finding here is a refusal.
	//
	// Run and Status treat it differently ON PURPOSE, and the asymmetry is the
	// point of having two entry points. Run ENFORCES: a non-empty slice stops
	// the run and fails the command. Status REPORTS: it fills this field and
	// leaves OK alone, because Status is the read model behind "dossierx serve"'s
	// status strip, and a page that must keep rendering for a human to read the
	// tampered claim cannot also be the thing that refuses to render because of
	// it. Callers that want the gate ENFORCED through Status — "check --validate"
	// and "check --staged" — test this slice themselves and say so.
	LedgerFindings []lock.Finding

	// OK is true once every fail-fast step passed and the run reached the
	// "check: OK" line. The reporting fields below are populated only then.
	OK bool

	// OrientationNotes and NextSteps are fully-composed lines/hints; the
	// terminal prints each verbatim (NextSteps under a "next steps:" header,
	// two-space indented). OpenComments maps module -> open-thread count, left
	// to the caller to sort and format ("open comments: module %q: %d").
	// ImplinkStatusStdout/Stderr are the impl-link status reporter's stdout and
	// stderr lines respectively, already formatted.
	OrientationNotes    []string
	OpenComments        map[string]int
	ImplinkStatusStdout []string
	ImplinkStatusStderr []string
	NextSteps           []string
}

// Run executes the check pipeline against claims (already loaded and
// review_pending-reconciled by the caller) and cfg, returning a fully
// populated Result and the first step's error (nil on success). The error is
// unprefixed — the caller wraps it "check: %w" — and Run stops at the first
// failure exactly as the CLI's fail-fast contract requires; serve ignores the
// error as a page gate but still reads whatever Result carries.
//
// The mockup gate is intentionally not re-run here: lint.RunAll's
// raw-html-scope rule already includes the mockup-gate findings as
// error-severity, so any raw_html violation fails at the lint step below —
// before catalog would reach its own gate. (The standalone "catalog"/"render"
// commands, which do not lint first, keep that gate; check does not need it.)
func Run(claims []model.Claim, cfg *config.Config) (Result, error) {
	var res Result

	// 1. Lint. A single error-severity finding fails the whole run here,
	// before any catalog/render write happens.
	res.LintFindings = lint.RunAll(claims, cfg)
	for _, f := range res.LintFindings {
		if f.Severity == lint.SeverityWarning {
			res.LintWarnings = append(res.LintWarnings, f)
		} else {
			res.LintErrors = append(res.LintErrors, f)
		}
	}
	if len(res.LintErrors) > 0 {
		return res, fmt.Errorf("lint: %d error-level finding(s)", len(res.LintErrors))
	}

	// 2. Catalog: build then persist .catalog.json. The built catalog is
	// reused for the render below (deterministic — rebuilding would only
	// repeat work and could not diverge).
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		return res, fmt.Errorf("catalog: build: %w", err)
	}
	catPath := catalogPath(cfg)
	if err := catalog.WriteJSON(cat, catPath); err != nil {
		return res, fmt.Errorf("catalog: %w", err)
	}
	res.CatalogPath = catPath
	res.CatalogCount = len(claims)

	// 3. Render the viewer to viewer/index.html.
	html, err := render.Render(cat, cfg)
	if err != nil {
		return res, fmt.Errorf("render: %w", err)
	}
	renderPath := renderOutPath(cfg)
	if err := os.MkdirAll(filepath.Dir(renderPath), 0o755); err != nil {
		return res, fmt.Errorf("render: create output dir: %w", err)
	}
	if err := os.WriteFile(renderPath, []byte(html), 0o644); err != nil {
		return res, fmt.Errorf("render: write %s: %w", renderPath, err)
	}
	res.RenderPath = renderPath

	// 4. Impl-link scan: reconcile every "dossierx-claim: <id>" source tag.
	// A scan I/O error stops the run; per-tag reconciliation errors are
	// reported (to stderr, by the caller) and also fail the run, but only
	// after every one is recorded so the reporter can print them all.
	scanReport, err := implink.Scan(claims, cfg)
	if err != nil {
		return res, err
	}
	res.ScanFilesScanned = scanReport.FilesScanned
	res.ScanSummary = scanReport.Summary()
	res.ScanErrors = scanReport.Errors
	if len(scanReport.Errors) > 0 {
		return res, fmt.Errorf("%d impl-link scan error(s)", len(scanReport.Errors))
	}

	// 5. The lock-ledger gate: does every locked claim still match the content
	// somebody approved, and does every claim's comment history still match
	// what the engine last wrote?
	//
	// Its position — dead last, AFTER the catalog and the viewer are already on
	// disk — is the whole design. These rules are deliberately not lints
	// (internal/lock/audit.go says why at length), and the practical expression
	// of that decision is right here: a tampered claim must not take a
	// project's documentation offline. Everything a reader needs in order to
	// SEE the tampered claim has been regenerated by the time the gate refuses,
	// and what the refusal costs is the exit status, not the viewer.
	res.LedgerFindings = ledgerGate(claims, loadLedgerInputs(cfg))
	if len(res.LedgerFindings) > 0 {
		return res, fmt.Errorf("ledger: %d integrity finding(s)", len(res.LedgerFindings))
	}

	// 6. Success. Everything below is non-blocking per-module reporting,
	// derived from the same claims/cfg (and the read-only lock/flag stores),
	// exactly as check's RunE tail produced it.
	res.OK = true
	res.OrientationNotes = orientationNotes(cfg, claims)
	res.OpenComments = openCommentCounts(claims)
	stdout, stderr, implinkHints := implinkStatus(cfg, claims)
	res.ImplinkStatusStdout = stdout
	res.ImplinkStatusStderr = stderr
	res.NextSteps = nextSteps(cfg, claims, implinkHints)
	return res, nil
}

// Status computes the subset of Result the serve status strip renders — the
// lint partition, per-module open-comment counts, and the next-steps advisory —
// WITHOUT any of Run's disk writes. It exists so GET/HEAD /api/status can drive
// the same check data as a page-poll WITHOUT truncating viewer/index.html or
// .catalog.json (Run's os.WriteFile side effects) and WITHOUT the per-request
// impl-link Scan, which mutates link artifacts. Those writers are the "dossierx
// check" writer's job, gated to the CLI / serve startup — never a read endpoint
// reachable by a bare, CSRF-exempt GET or HEAD. It reads only (lint in memory,
// the lock/flag/build-order stores, and the READ-ONLY implink.Status for the
// drift/unlinked next-step hints — never implink.Scan), so it can never itself
// re-trigger the watcher or corrupt a half-written viewer.
//
// It mirrors Run's fail-fast shape for the fields it populates: LintFindings /
// LintErrors / LintWarnings are always set; OK is true only when there is no
// error-severity finding (the sole fail-fast signal computable without the
// catalog/render/scan steps), and the best-effort reporting (OpenComments,
// NextSteps) is populated only then — exactly as Run leaves those fields empty
// when it stops at the lint step. It never returns an error: a lint failure is
// data for the status strip to show, not a reason to fail the endpoint.
//
// It also evaluates the lock-ledger gate and fills Result.LedgerFindings, but
// — unlike Run — does not treat a finding as a stop. Reporting rather than
// refusing is what lets one function serve both a browser-facing status poll
// and the two enforcing read-only CLI paths (--validate, --staged), which read
// the same field and decide for themselves.
func Status(claims []model.Claim, cfg *config.Config) Result {
	return status(claims, cfg, loadLedgerInputs(cfg))
}

// StatusStaged is Status evaluated against the GIT INDEX: the claim registry
// Staged assembled from staged blobs, and — just as importantly — the lock
// ledger and comment digest store as the INDEX holds them, not as the working
// tree holds them. See staged.go for why the stores have to come from the same
// place the claims do.
//
// It is the seam "dossierx check --staged" drives, and it inherits Status's
// contract exactly: no writes of any kind, no review_pending reconcile, no
// catalog, no viewer, no impl-link scan. The caller enforces — it inspects
// LintErrors and LedgerFindings and decides the exit status.
func StatusStaged(sp StagedProject, cfg *config.Config) Result {
	return status(sp.Claims, cfg, sp.ledger)
}

// status is the shared body of Status and StatusStaged. The only thing that
// varies between them is WHERE the ledger state came from; everything else —
// the lint pass, the reporting, and the "never returns an error" contract — is
// identical by construction, which is what keeps "what --staged checks" and
// "what --validate checks" the same set of rules rather than two lists that
// have to be kept in step by hand.
func status(claims []model.Claim, cfg *config.Config, in ledgerInputs) Result {
	var res Result

	res.LintFindings = lint.RunAll(claims, cfg)
	for _, f := range res.LintFindings {
		if f.Severity == lint.SeverityWarning {
			res.LintWarnings = append(res.LintWarnings, f)
		} else {
			res.LintErrors = append(res.LintErrors, f)
		}
	}
	if len(res.LintErrors) > 0 {
		// Mirror Run's lint fail-fast: surface the errors, leave the best-effort
		// reporting below empty exactly as the disk-writing Run leaves it.
		return res
	}

	// The ledger gate is EVALUATED here but does not decide anything: see
	// Result.LedgerFindings for why Status reports where Run refuses. OK stays
	// lint-driven so serve's status strip keeps rendering a project whose
	// ledger is in dispute — the alternative is a viewer that goes dark exactly
	// when a human most needs to read the claim that is in dispute.
	res.LedgerFindings = ledgerGate(claims, in)

	res.OK = true
	res.OpenComments = openCommentCounts(claims)
	// The impl-link hints come from the READ-ONLY implink.Status (drift/unlinked),
	// the same source Run's nextSteps uses — NOT implink.Scan, which is the
	// mutating reconcile and stays out of the memory-only status path.
	_, _, implinkHints := implinkStatus(cfg, claims)
	res.NextSteps = nextSteps(cfg, claims, implinkHints)
	return res
}

// orientationNotes returns one "orientation notes: module %q: %d (…)" line
// per module (in cfg.Modules order) that has at least one orientation-note
// claim, broken down by facet (facets sorted for determinism). It is the
// value form of cmd/dossierx.reportOrientationNotes.
func orientationNotes(cfg *config.Config, claims []model.Claim) []string {
	type counts struct {
		total   int
		byFacet map[string]int
	}
	byModule := map[string]*counts{}
	for _, c := range claims {
		if c.EffectiveKind() != model.KindOrientationNote {
			continue
		}
		cnt, ok := byModule[c.Module]
		if !ok {
			cnt = &counts{byFacet: map[string]int{}}
			byModule[c.Module] = cnt
		}
		cnt.total++
		cnt.byFacet[c.Facet]++
	}

	var lines []string
	for _, module := range cfg.Modules {
		cnt, ok := byModule[module]
		if !ok {
			continue
		}
		facets := make([]string, 0, len(cnt.byFacet))
		for f := range cnt.byFacet {
			facets = append(facets, f)
		}
		sort.Strings(facets)
		var parts []string
		for _, f := range facets {
			parts = append(parts, fmt.Sprintf("%d in %s", cnt.byFacet[f], f))
		}
		lines = append(lines, fmt.Sprintf("orientation notes: module %q: %d (%s)", module, cnt.total, strings.Join(parts, ", ")))
	}
	return lines
}

// openCommentCounts returns module -> number of open comment threads across
// that module's claims, for modules with at least one. The value form of
// cmd/dossierx.reportOpenComments (the caller sorts modules and formats the
// "open comments: module %q: %d" lines).
func openCommentCounts(claims []model.Claim) map[string]int {
	counts := map[string]int{}
	for _, c := range claims {
		if n := len(c.OpenThreadIDs()); n > 0 {
			counts[c.Module] += n
		}
	}
	return counts
}

// implinkStatus returns the impl-link status reporter's stdout lines, stderr
// lines, and the next-steps hints it contributes, for every module in
// cfg.Modules that has an implementation-link artifact (modules with none are
// silently skipped). The value form of cmd/dossierx.reportImplinkStatus.
func implinkStatus(cfg *config.Config, claims []model.Claim) (stdout, stderr, hints []string) {
	for _, module := range cfg.Modules {
		report, err := implink.Status(claims, cfg, module)
		if err != nil {
			if errors.Is(err, implink.ErrNoArtifact) {
				continue
			}
			stderr = append(stderr, fmt.Sprintf("check: implink status for %q: %v", module, err))
			continue
		}
		stdout = append(stdout, report.Summary())
		for _, d := range report.Drifted {
			stdout = append(stdout, fmt.Sprintf("  drifted: %s %s: %s", d.ClaimID, d.File, d.Reason))
			hints = append(hints, fmt.Sprintf("%s is drifted -> re-tag or dossierx claim link --module %s --claim %s --file %s", d.ClaimID, module, d.ClaimID, d.File))
		}
		for _, id := range report.UnlinkedIDs {
			stdout = append(stdout, fmt.Sprintf("  unlinked: %s", id))
		}
		if len(report.UnlinkedIDs) > 0 {
			hints = append(hints, fmt.Sprintf("%d claim(s) in module %q have no code link yet -> add a dossierx-claim tag or dossierx claim link", len(report.UnlinkedIDs), module))
		}
	}
	return stdout, stderr, hints
}

// nextSteps returns the ordered "what to run next" hint lines: draft claims,
// then claims pending on an open comment thread (comment first — a thread
// must be resolved before the claim can lock and reaudit refuses a
// comment-only pending), then drift/flag review_pending claims, then the
// caller's implink hints, then a build-order prompt per fully-locked module
// with no artifact yet. review_pending claims are partitioned by WHY via
// comments.PendingTriggers, read against the lock and flag stores loaded
// best-effort (a load error degrades to "no drift/flag"). The value form of
// cmd/dossierx.reportNextSteps.
func nextSteps(cfg *config.Config, claims []model.Claim, implinkHints []string) []string {
	var hints []string

	// Best-effort: a load error just degrades the drift/flag partition to "none"
	// (PendingTriggers nil-checks both stores), so status stays a pure read.
	store, storeErr := lock.LoadStore(storePath(cfg))
	if storeErr != nil {
		store = nil
	}
	flagStore, flagErr := reaudit.LoadFlagStore(flagStorePath(cfg))
	if flagErr != nil {
		flagStore = nil
	}

	var draftIDs []string
	var commentPending []model.Claim // review_pending with >=1 open thread
	var reauditTriggered []string    // review_pending from an ACTIVE drift/flag trigger
	var reauditTriggerless []string  // review_pending but NO active trigger at all
	for _, c := range claims {
		switch {
		case c.Status == model.StatusDraft:
			draftIDs = append(draftIDs, c.ID)
		case c.Status == model.StatusLocked && c.ReviewPending:
			drift, flag, open := comments.PendingTriggers(c, claims, store, flagStore)
			if open > 0 {
				commentPending = append(commentPending, c)
			}
			// Partition the reaudit next-step by WHY the claim is review_pending so
			// its label is accurate:
			//   - an ACTIVE drift/flag trigger -> "from drift/flag".
			//   - NO active trigger at all (open==0 && !drift && !flag) -> "no
			//     active trigger": the state left when a drifted dependency is
			//     reverted, or an open thread is hand-resolved directly in YAML.
			//     v0.1.2 printed the reaudit hint for EVERY locked+review_pending
			//     claim, so a triggerless one must still surface it — just not
			//     MISLABELED as drift/flag (its cause is neither).
			// The only case in neither bucket is a purely comment-pending claim
			// (open>0 && !drift && !flag): its remedy is to resolve the thread
			// (reaudit refuses a comment-only pending), already carried by
			// commentPending above.
			switch {
			case drift || flag:
				reauditTriggered = append(reauditTriggered, c.ID)
			case open == 0:
				reauditTriggerless = append(reauditTriggerless, c.ID)
			}
		}
	}
	// Every invocation below must be a command that EXISTS and would SUCCEED as
	// printed. That is a sharper requirement than it was before v0.3.0: the
	// agent skills now instruct an agent to read next_actions and error.hint
	// instead of re-deriving the lifecycle itself, so a stale hint is no longer
	// a cosmetic wart — it is wrong advice an agent will act on. Two
	// consequences show up here:
	//
	//   - the verbs carry their noun ("claim lock", not "lock"), because the
	//     bare forms were retired when the surface went 26 -> 19; and
	//   - "claim lock" is printed WITH --reason, because --reason is required on
	//     the writing path. A hint of "dossierx claim lock <id>" names a real
	//     command that then refuses, which is worse than naming none.
	if len(draftIDs) > 0 {
		hints = append(hints, fmt.Sprintf("%d claim(s) still draft -> dossierx claim lock <id> --reason \"…\" (e.g. %s)", len(draftIDs), draftIDs[0]))
	}
	if len(commentPending) > 0 {
		// There is deliberately NO command here. "comment resolve" was removed
		// from the CLI in v0.3.0 and lives only in the viewer, because resolving
		// a thread IS the human's approval and the agent is not the rights
		// holder. Printing a resolve invocation would tell an agent to do the
		// one thing this release exists to stop it doing, so the hint names the
		// human's surface (serve) and the thread that needs their attention.
		example := commentPending[0]
		hints = append(hints, fmt.Sprintf("%d claim(s) with open comment thread(s) -> the human resolves them in the viewer (dossierx serve); an agent may only reply (e.g. %s %s)", len(commentPending), example.ID, example.OpenThreadIDs()[0]))
	}
	if len(reauditTriggered) > 0 {
		// Bare "claim reaudit" is the PREVIEW form and needs no --reason, so it
		// is correct as printed; --reason joins it only at --confirm.
		hints = append(hints, fmt.Sprintf("%d claim(s) review_pending from drift/flag -> dossierx claim reaudit <id> (e.g. %s)", len(reauditTriggered), reauditTriggered[0]))
	}
	if len(reauditTriggerless) > 0 {
		hints = append(hints, fmt.Sprintf("%d claim(s) review_pending with no active trigger -> dossierx claim reaudit <id> (e.g. %s)", len(reauditTriggerless), reauditTriggerless[0]))
	}
	hints = append(hints, implinkHints...)

	byModule := make(map[string][]model.Claim, len(cfg.Modules))
	for _, c := range claims {
		byModule[c.Module] = append(byModule[c.Module], c)
	}
	for _, module := range cfg.Modules {
		mClaims := byModule[module]
		if len(mClaims) == 0 {
			continue
		}
		fullyLocked := true
		for _, c := range mClaims {
			// Same predicate as the buildorder Propose gate: an open comment
			// thread makes a module not build-ready even if every claim is
			// locked, so it must not be reported "fully locked, propose now".
			if c.Status != model.StatusLocked || c.ReviewPending || c.HasOpenThreads() {
				fullyLocked = false
				break
			}
		}
		if !fullyLocked {
			continue
		}
		if _, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module)); errors.Is(err, buildorder.ErrNotProposed) {
			hints = append(hints, fmt.Sprintf("module %q is fully locked with no build order yet -> dossierx build-order propose --module %s", module, module))
		}
	}
	return hints
}

// catalogPath, renderOutPath, storePath, and flagStorePath resolve check's
// side-file locations under cfg.Dir() (absolute), mirroring the identically
// named helpers in cmd/dossierx — the same four paths the CLI writes/reads —
// so Run can persist and re-read them without importing package main. They
// are duplicated (not shared) deliberately: these string constants are
// stable, and the parity test in cmd/dossierx fails loudly if the two copies
// ever disagree about where check writes.
func catalogPath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".catalog.json")
}

func renderOutPath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), "viewer", "index.html")
}

func storePath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
}

func flagStorePath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".dossierx-flag-store.json")
}
