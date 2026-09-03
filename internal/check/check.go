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
	"github.com/BarterX-Tech/dossierx/internal/digest"
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

	// BuildOrders is one entry per module that HAS a build-order artifact,
	// with staleness recomputed live against the current claims (never read off
	// the artifact's own persisted "stale" field, which nothing refreshes).
	//
	// It exists because check reported nothing at all about build orders, and a
	// locked one going stale is a NORMAL consequence of the sanctioned lifecycle:
	// unlock a covered claim, edit it, re-lock it, and the module's approved
	// implementation sequence no longer matches the claims — `build-order status`
	// says stale:true while `check`, `check --validate` and `check --staged`ndash;
	// the loop command, the pre-commit hook and CI — all said ok:true with no
	// mention of it. The build-order skill tells an agent to act "whenever a
	// locked build order reports stale"; this is the field that lets it, without
	// parsing a hint string.
	BuildOrders []BuildOrderReport

	// ThemeError is the viewer theme's refusal — an unreadable or unstaged
	// theme file, a font whose bytes are not the format its extension
	// claims, a font family nothing names, the total font cap — or "" when
	// the theme is fine or absent. It is a STRING rather than an error
	// because Result is a value the machine surface projects, and it is a
	// separate field rather than a lint finding because it is not one: no
	// claim is at fault and no lint rule was run.
	//
	// A non-empty ThemeError clears OK. The viewer this project would render
	// does not exist, so a status that said "ok" would be describing a
	// document nobody can produce.
	ThemeError string

	// ThemeFontCount/ThemeFontBytes are how much of the reader's download
	// the project's own fonts account for: the number of faces the theme
	// inlines and their total RAW size (base64 expands it by a third in the
	// emitted viewer). Zero for a project with no fonts, which is almost all
	// of them, and zero when ThemeError is set — nothing was accepted.
	ThemeFontCount int
	ThemeFontBytes int64

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

// BuildOrderReport is one module's build-order state as check found it: whether
// an artifact exists, whether it is locked, and whether it is stale RIGHT NOW.
//
// Stale/StaleIDs are recomputed from the claims on every run (buildorder.Status,
// which is read-only), never taken from the artifact's own persisted fields. The
// artifact writes `"stale": false` at lock time and nothing ever revises it, so
// the file on disk goes on asserting false while the order is stale — reading it
// would make this report repeat the lie rather than replace it.
type BuildOrderReport struct {
	Module   string   `json:"module"`
	Locked   bool     `json:"locked"`
	Stale    bool     `json:"stale"`
	StaleIDs []string `json:"stale_ids,omitempty"`
}

// buildOrderReports returns one BuildOrderReport per module in cfg.Modules that
// HAS an artifact, in cfg.Modules order. A module with no artifact is omitted
// entirely (that is the ordinary state of most modules, and nextSteps already
// has its own hint for a fully-locked module that has never proposed one), and a
// module whose artifact cannot be read is omitted too — the ledger gate reports
// that as build-order-unreadable, and a report that guessed at its contents
// would be worse than one that says nothing.
func buildOrderReports(cfg *config.Config, claims []model.Claim) []BuildOrderReport {
	if cfg == nil {
		return nil
	}
	var out []BuildOrderReport
	for _, module := range cfg.Modules {
		a, err := buildorder.Status(buildorder.ArtifactPath(cfg, module), claims, cfg)
		if err != nil || a == nil {
			continue
		}
		out = append(out, BuildOrderReport{
			Module:   module,
			Locked:   a.Locked,
			Stale:    a.Stale,
			StaleIDs: a.StaleIDs,
		})
	}
	return out
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
	inputs := loadLedgerInputs(cfg)

	// 1. Lint. A single error-severity finding fails the whole run here,
	// before any catalog/render write happens.
	res.LintFindings = policyLintFindings(lint.RunAll(claims, cfg), inputs.store)
	for _, f := range res.LintFindings {
		if f.Severity == lint.SeverityWarning {
			res.LintWarnings = append(res.LintWarnings, f)
		} else {
			res.LintErrors = append(res.LintErrors, f)
		}
	}
	if len(res.LintErrors) > 0 {
		// The gate still runs. A lint error stops the PIPELINE — no catalog, no
		// viewer, no scan — but it must not stop the ledger REPORT, and the two
		// used to be the same return: a project with one dangling rests_on in a
		// draft claim reported ledger_findings: [] no matter what had been done
		// to its locked ones, and an empty array is indistinguishable from "the
		// ledger is clean". The documented recovery for lint_failed is "fix the
		// findings and re-run", so the integrity finding surfaced only after the
		// typo was fixed — or, if that hook run was bypassed, never.
		//
		// The gate is a pure read over the claims and the two stores (ledger.go),
		// so a lint error cannot make it wrong: it can only make it incomplete in
		// the same way the claims themselves are. The caller's exit-code
		// precedence is unchanged — cmd/dossierx tests LintErrors first, so this
		// still reports lint_failed / stopped_at "lint"; what changes is that
		// data.ledger_findings is populated rather than silently empty.
		res.LedgerFindings = ledgerGate(claims, inputs)
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
	//
	// The theme is resolved HERE rather than inside render.Render so that the
	// same numbers the read-only modes report — how many fonts the reader
	// downloads and how many bytes of them — are on this Result too, and so
	// that a theme refusal is one error rather than one per rebuild.
	rt, err := config.ResolveTheme(cfg, os.ReadFile)
	if err != nil {
		res.ThemeError = err.Error()
		return res, fmt.Errorf("render: %w", err)
	}
	res.ThemeFontCount = len(rt.Fonts)
	for _, f := range rt.Fonts {
		res.ThemeFontBytes += int64(len(f.Data))
	}
	html, err := render.RenderWithTheme(cat, cfg, rt)
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
	res.BuildOrders = buildOrderReports(cfg, claims)
	stdout, stderr, implinkHints := implinkStatus(cfg, claims)
	res.ImplinkStatusStdout = stdout
	res.ImplinkStatusStderr = stderr
	res.NextSteps = nextSteps(cfg, claims, implinkHints, res.BuildOrders)
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
	return status(claims, cfg, loadLedgerInputs(cfg), os.ReadFile)
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
// The config it evaluates against is sp.Config — project.config.yaml AS THE
// INDEX HOLDS IT — not the caller's worktree config. That is not a detail: cfg
// supplies the facet and module vocabularies, the doctrine facet, and the hub
// gating switch, so linting staged claims against a worktree config would let
// an unstaged config edit change the verdict on a commit that does not contain
// it. The cfg parameter survives only as the fallback for the case
// StagedProject documents — a project.config.yaml that is not tracked at all,
// where sp.Config already IS the caller's config.
func StatusStaged(sp StagedProject, cfg *config.Config) Result {
	if sp.Config != nil {
		cfg = sp.Config
	}
	read := sp.readIndex
	if read == nil {
		// A StagedProject a caller built by hand (only tests do) has no
		// index reader. Refusing to fall back to os.ReadFile is the point:
		// silently grading the theme against the working tree under
		// --staged is exactly the bypass the rest of this file exists to
		// close, so the theme rules report that they could not run.
		read = func(path string) ([]byte, error) {
			return nil, fmt.Errorf("%s: no git index reader (this StagedProject was not built by Staged)", path)
		}
	}
	return status(sp.Claims, cfg, sp.ledger, read)
}

// status is the shared body of Status and StatusStaged. The only thing that
// varies between them is WHERE the ledger state came from; everything else —
// the lint pass, the reporting, and the "never returns an error" contract — is
// identical by construction, which is what keeps "what --staged checks" and
// "what --validate checks" the same set of rules rather than two lists that
// have to be kept in step by hand.
func status(claims []model.Claim, cfg *config.Config, in ledgerInputs, read func(string) ([]byte, error)) Result {
	var res Result

	// The theme is evaluated through an INJECTED reader, which is the whole
	// reason this parameter exists: --staged passes a reader that answers
	// from the git index, so the theme file's content, every font's
	// signature and the total size cap are judged against the bytes the
	// commit will carry rather than against the working tree beside it. All
	// three modes therefore run one rule set (config.ValidateTheme) instead
	// of a strict one and a lenient one that have to be kept in step by hand.
	if rep, err := config.ValidateTheme(cfg, read); err != nil {
		res.ThemeError = err.Error()
	} else {
		res.ThemeFontCount = rep.FontCount
		res.ThemeFontBytes = rep.FontBytes
	}

	res.LintFindings = policyLintFindings(lint.RunAll(claims, cfg), in.store)
	for _, f := range res.LintFindings {
		if f.Severity == lint.SeverityWarning {
			res.LintWarnings = append(res.LintWarnings, f)
		} else {
			res.LintErrors = append(res.LintErrors, f)
		}
	}
	// The ledger gate is EVALUATED here but does not decide anything: see
	// Result.LedgerFindings for why Status reports where Run refuses. OK stays
	// lint-driven so serve's status strip keeps rendering a project whose
	// ledger is in dispute — the alternative is a viewer that goes dark exactly
	// when a human most needs to read the claim that is in dispute.
	//
	// It runs ABOVE the lint fail-fast below, and that ordering is load-bearing
	// for the two ENFORCING callers this body also serves. "check --validate"
	// and "check --staged" (and therefore the pre-commit hook and CI) read
	// LedgerFindings; when the gate sat under the fail-fast, a single unrelated
	// error-severity lint finding emptied it, so a commit that hand-edited a
	// locked claim AND typo'd a draft's rests_on was reported as a lint problem
	// only. Fix the typo, commit again, and the integrity finding had never been
	// shown. The gate is a pure read (ledger.go) and cannot be made wrong by a
	// lint error, so there is nothing to gain by deferring it and one whole
	// class of silence to lose.
	res.LedgerFindings = ledgerGate(claims, in)

	if len(res.LintErrors) > 0 {
		// Mirror Run's lint fail-fast: surface the errors, leave the best-effort
		// reporting below empty exactly as the disk-writing Run leaves it.
		return res
	}

	if res.ThemeError != "" {
		// Same shape as the lint fail-fast above: report the refusal, leave
		// the best-effort reporting below empty.
		return res
	}

	res.OK = true
	res.OpenComments = openCommentCounts(claims)
	// Build-order state is recomputed here too, for --validate, --staged and the
	// serve strip: buildorder.Status is a read (it never writes the artifact
	// back), so it belongs on the non-writing path exactly as implink.Status
	// does.
	res.BuildOrders = buildOrderReports(cfg, claims)
	// The impl-link hints come from the READ-ONLY implink.Status (drift/unlinked),
	// the same source Run's nextSteps uses — NOT implink.Scan, which is the
	// mutating reconcile and stays out of the memory-only status path.
	_, _, implinkHints := implinkStatus(cfg, claims)
	res.NextSteps = nextSteps(cfg, claims, implinkHints, res.BuildOrders)
	// NOTHING IS PREPENDED HERE ANY MORE. A scope advisory used to go first,
	// ahead of every claim-level hint: under --staged, a shallow checkout could
	// not reach the parent commit, so the run had to say "this run could not
	// check one of the things you believe it checks". With the parent comparison
	// gone (see staged.go's "REMOVED, DELIBERATELY" section) every rule this gate
	// runs is answerable from the one tree in front of it, so there is no longer
	// a state in which the gate looked at less than it claims to.
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
// firstLockableDraft returns the id of the first draft claim in drafts that
// would survive ALL THREE of lock.Lock's gates, or "" if none would.
//
// It evaluates them in lock.Lock's own order — lint, hub gating, open threads —
// so the claim it names is a claim the real command would accept.
//
// The LINT gate is the one that cannot be skipped, and the reason is that it is
// evaluated against the ABOUT-TO-BE-LOCKED form, not the current one. Two lints
// key off a claim's own status: rest-on-locked (a locked claim's rests_on
// targets must themselves be locked) and roll-up (a locked banner's
// module-mates must be locked). So a project can pass `check` completely
// cleanly and still have `claim lock <id>` refuse with lint_failed — which is
// exactly what happened on a module drafted alongside its own dependencies, the
// ordinary case, where the first draft in load order rests on a sibling that is
// also still draft.
//
// It is evaluated LAZILY, stopping at the first claim that passes, because that
// is what keeps the cost proportionate: naming an example is an advisory line,
// and in the common case the answer is the first or second candidate. The two
// cheap gates are tested first so a full lint pass is only spent on a candidate
// that could still qualify.
func firstLockableDraft(drafts, claims []model.Claim, cfg *config.Config, store *lock.Store) string {
	for _, c := range drafts {
		if c.HasOpenThreads() || hasUnlockedDoctrineDep(c, claims, cfg) {
			continue
		}
		if lintErrorsForCandidate(c, claims, cfg, store) == 0 {
			return c.ID
		}
	}
	return ""
}

// hasUnlockedDoctrineDep is lock.Lock's hub-gating refusal as a pure read: a
// dependency in the doctrine facet must itself be locked first.
func hasUnlockedDoctrineDep(c model.Claim, claims []model.Claim, cfg *config.Config) bool {
	if cfg == nil || !cfg.HubGatingEnabled() {
		return false
	}
	deps := make([]string, 0, len(c.Mirrors)+len(c.RestsOn))
	deps = append(deps, c.Mirrors...)
	deps = append(deps, c.RestsOn...)
	for _, dep := range deps {
		for _, d := range claims {
			if d.ID == dep && d.Facet == cfg.DoctrineFacet && d.Status != model.StatusLocked {
				return true
			}
		}
	}
	return false
}

// lintErrorsForCandidate counts the error-severity findings the lint suite would
// raise if c were locked RIGHT NOW — the corpus with c's own entry replaced by
// its locked form, which is precisely what lock.Lock lints. Linting the
// still-draft entry instead would report zero for the very claims this is meant
// to filter out.
func lintErrorsForCandidate(c model.Claim, claims []model.Claim, cfg *config.Config, store *lock.Store) int {
	candidate := c
	candidate.Status = model.StatusLocked
	candidate.ReviewPending = false

	lintClaims := make([]model.Claim, len(claims))
	copy(lintClaims, claims)
	for i := range lintClaims {
		if lintClaims[i].ID == c.ID {
			lintClaims[i] = candidate
		}
	}

	errs := 0
	for _, f := range policyLintFindings(lint.RunAll(lintClaims, cfg), store) {
		if f.Severity != lint.SeverityWarning {
			errs++
		}
	}
	return errs
}

func nextSteps(cfg *config.Config, claims []model.Claim, implinkHints []string, buildOrders []BuildOrderReport) []string {
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

	// The pre-ledger crossing, FIRST because it is the only hint here about the
	// project rather than about a claim.
	//
	// It is the human-readable half of the lock-ledger-pre-ledger finding the
	// ledger gate reports beside it. The finding is what fails the gate; the hint
	// is what says how to clear it, in the next_steps list an agent reads for its
	// next move.
	//
	// IT IS GATED ON THE SAME UNION THE FINDING IS — at least one locked claim or
	// one locked build order — and not on the bare predicate. A pre-ledger project
	// holding nothing locked crosses silently and correctly on its next lock, so a
	// hint firing there would tell a human to fix a state this same run reports as
	// clean.
	lockedBuildOrders := 0
	for _, b := range buildOrders {
		if b.Locked {
			lockedBuildOrders++
		}
	}
	if store != nil && store.PreLedgerUnadopted(digestStorePresent(cfg)) &&
		countLockedClaims(claims)+lockedBuildOrders > 0 {
		hints = append(hints, "this project's locks predate the lock ledger -> re-propose any locked build order FIRST, then unlock every locked claim, then re-lock only what you still stand behind; the FIRST lock in a project holding nothing crosses the store onto the ledger schema — unlocking alone does not. There is no automatic adoption and no migration command")
	}

	var draftIDs []string
	var drafts []model.Claim         // the same claims, for the lock-gate evaluation
	var commentPending []model.Claim // ANY claim with >=1 open thread, draft or locked
	var reauditTriggered []string    // review_pending from an ACTIVE drift/flag trigger
	var reauditTriggerless []string  // review_pending but NO active trigger at all
	for _, c := range claims {
		// The open-thread hint is keyed on the THREADS, not on the claim's lock
		// state, and it sits outside the switch below for that reason. It used to
		// be appended only inside the locked+review_pending arm, which made the
		// hint disagree with the two counts printed beside it in the same
		// envelope: a DRAFT claim carrying an open thread was reported by
		// open_comments and by the comments-unresolved lint, and by no next_step
		// at all — while `claim lock` on it refuses with unresolved_comments. A
		// draft's thread is exactly as much a thing the human has to act on as a
		// locked one's (it is the gate that stops the claim locking), and a hint
		// that undercounts the work is a hint an agent uses to conclude there is
		// none.
		if len(c.OpenThreadIDs()) > 0 {
			commentPending = append(commentPending, c)
		}
		switch {
		case c.Status == model.StatusDraft:
			draftIDs = append(draftIDs, c.ID)
			drafts = append(drafts, c)
		case c.Status == model.StatusLocked && c.ReviewPending:
			drift, flag, open := comments.PendingTriggers(c, claims, store, flagStore)
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
	//
	// The third consequence is the example id. draftIDs[0] is whichever draft
	// claim happens to sort first, which is not the same thing as a draft claim
	// that would actually LOCK: all three of lock.Lock's gates can refuse it,
	// and the lint gate refuses it while the project as a whole lints clean
	// (rest-on-locked and roll-up are evaluated against the ABOUT-TO-BE-LOCKED
	// form). Naming such a claim produces a command that exists, reads as
	// recommended, and then exits 1 — and the agent, which the skills tell to
	// trust next_actions rather than re-derive the lifecycle, acts on it. So the
	// example is the first draft that passes every gate, and when none does the
	// hint says so instead of pretending to know where to start.
	if len(draftIDs) > 0 {
		if example := firstLockableDraft(drafts, claims, cfg, store); example != "" {
			hints = append(hints, fmt.Sprintf("%d claim(s) still draft -> dossierx claim lock <id> --reason \"…\" (e.g. %s)", len(draftIDs), example))
		} else {
			hints = append(hints, fmt.Sprintf("%d claim(s) still draft -> dossierx claim lock <id> --reason \"…\" (none is lockable yet: every draft is blocked by a lint error, an open comment thread, or an unlocked dependency)", len(draftIDs)))
		}
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

	// The build-order hints. There used to be exactly one — "fully locked, no
	// artifact yet" — reached through a branch that tested only for
	// ErrNotProposed and discarded every other answer, so the two states a
	// project actually spends time in were both silent:
	//
	//   - a LOCKED order that has gone STALE. That is the ordinary outcome of the
	//     fully sanctioned lifecycle (unlock a covered claim, edit it, re-lock),
	//     and it means the approved implementation sequence no longer matches the
	//     claims. `build-order status` reported stale:true while `check`,
	//     `check --staged` and therefore the hook and CI said ok:true and named
	//     only the dependent claim's review_pending. The skill tells an agent to
	//     act "whenever a locked build order reports stale"; the loop command
	//     never reported it.
	//   - an artifact that exists and is UNLOCKED — an abandoned propose->lock
	//     flow, silent forever, with next_steps null.
	//
	// The states come from buildOrders (recomputed live, never from the
	// artifact's persisted "stale" field), so the hint and Result.BuildOrders can
	// never disagree.
	reported := make(map[string]bool, len(buildOrders))
	for _, bo := range buildOrders {
		reported[bo.Module] = true
		switch {
		case bo.Locked && bo.Stale:
			hints = append(hints, fmt.Sprintf(
				"module %q's locked build order is stale (%d claim(s) changed: %s) -> dossierx build-order propose --module %s, then dossierx build-order lock --module %s --reason \"…\"",
				bo.Module, len(bo.StaleIDs), strings.Join(bo.StaleIDs, ", "), bo.Module, bo.Module))
		case !bo.Locked:
			hints = append(hints, fmt.Sprintf(
				"module %q has a proposed build order that was never locked -> dossierx build-order lock --module %s --reason \"…\"",
				bo.Module, bo.Module))
		}
	}

	byModule := make(map[string][]model.Claim, len(cfg.Modules))
	for _, c := range claims {
		byModule[c.Module] = append(byModule[c.Module], c)
	}
	for _, module := range cfg.Modules {
		mClaims := byModule[module]
		if len(mClaims) == 0 || reported[module] {
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

// lockStoreFileName is the lock ledger's file name, named rather than inlined
// because staged.go also has to RECOGNISE it — by base name, anywhere in the
// index — when deciding whether an untracked project.config.yaml is a
// first-commit project or a bypass (see indexHoldsJudgeableContent).
//
// It is an alias for lock.StoreFileName rather than a second literal: the engine
// owns the name, and a copy here could drift from the file the engine actually
// opens while every test in this package still passed.
const lockStoreFileName = lock.StoreFileName

func storePath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), lockStoreFileName)
}

// digestStorePresent reports whether the comment digest store is on disk for
// cfg. It is the evidence lock.Store.PreLedgerUnadopted weighs against a store that
// claims to predate the ledger (see lock.Store.LedgerDowngraded), read here from
// the WORKING TREE because that is where the rest of nextSteps' advisory inputs
// come from — this is a hint about what to run next, never a verdict, and the
// verdict's copy of the same question comes out of ledgerInputs (the index,
// under --staged).
func digestStorePresent(cfg *config.Config) bool {
	_, err := os.Stat(digest.StorePath(cfg))
	return err == nil
}

func flagStorePath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".dossierx-flag-store.json")
}

// policyLintFindings keeps the registry authoritative while reconciling the
// one legacy rule that local-approval v1 explicitly replaces. A v1 approval
// may rest on a readable draft boundary; readiness reports that condition and
// withholds integrated readiness. Stores without recorded adoption preserve
// the old lint result, so upgrading a binary does not relax old projects.
func policyLintFindings(findings []lint.Finding, store *lock.Store) []lint.Finding {
	if store == nil || !store.LocalApprovalEnabled() {
		return findings
	}
	out := make([]lint.Finding, 0, len(findings))
	for _, finding := range findings {
		if finding.LintName == "rest-on-locked" {
			continue
		}
		out = append(out, finding)
	}
	return out
}
