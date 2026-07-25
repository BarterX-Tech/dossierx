// Command dossierx is the CLI entrypoint for the engine. All project-specific
// behavior comes from the --config file (project.config.yaml); this
// binary itself has zero hardcoded references to any particular project.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
	"github.com/BarterX-Tech/dossierx/internal/render"
)

var configPath string

// version, commit, and date are stamped in at release time by goreleaser's
// -X ldflags (see .goreleaser.yaml, which targets these exact
// package-qualified names). A plain "go build"/"go install" sets no ldflags,
// leaving them empty; resolveVersionInfo falls back to
// runtime/debug.ReadBuildInfo in that case so the binary always reports
// something sensible instead of blank.
var (
	version string
	commit  string
	date    string
)

// errClaimNotFound and errWrongState are sentinels the lock/unlock/flag
// subcommands wrap into their not-found / wrong-state errors so main() can
// map both to exit code 2 — the documented "not found / not in the right
// state" family (see README's exit-codes table). deps/reaudit reach exit 2
// via a direct os.Exit(2) instead; lock/unlock/flag hold deferred
// file-lock releases that a bare os.Exit would skip, so they signal through
// these sentinels and let RunE unwind cleanly.
var (
	errClaimNotFound = errors.New("claim not found")
	errWrongState    = errors.New("claim not in the required state")
)

// main, and the os.Exit(2) calls inside newDepsCmd/newReauditCmd's "not
// found" / "not locked+review_pending" branches, are deliberately not
// exercised by this package's own tests: calling any of them in-process
// would terminate the test binary itself. They are covered end-to-end by
// tests/, which execs a built "dossierx" binary as a subprocess instead (see
// tests/cli_test.go's TestMain/run and, for those specific exit(2)
// branches, tests/cli_test.go's TestNestedConfigNearestWins and
// tests/lock_lifecycle_test.go's TestLockLifecycle_ReauditRefusedWhenNotPending).
// Everything else in this package — command wiring, flag parsing, the
// pure helpers — is unit- and CLI-tested in-process here, in main_test.go
// and cli_inprocess_test.go.
func main() {
	if err := newRootCmd().Execute(); err != nil {
		// Exit 2 is the "not found / not in the right state" family: a
		// missing config file, a claim id that doesn't exist, or a claim
		// that isn't in the state a command requires. Everything else
		// (lint errors, validation failures, write errors) is exit 1, so
		// scripts/CI can tell "nothing/nobody there" apart from "loaded
		// but failed". deps/reaudit reach exit 2 via their own os.Exit(2);
		// lock/unlock/flag wrap errClaimNotFound/errWrongState instead.
		if errors.Is(err, config.ErrNotFound) ||
			errors.Is(err, errClaimNotFound) ||
			errors.Is(err, errWrongState) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "dossierx",
		Short:        "Render claims into a static HTML viewer",
		SilenceUsage: true,
	}
	// Setting Version is what makes cobra wire up the built-in --version
	// flag on the root command. The resolved value (ldflag-stamped, or a
	// debug.ReadBuildInfo fallback) is used so --version never prints blank.
	v, _, _ := resolveVersionInfo()
	root.Version = v
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to project.config.yaml (default: search upward from the current directory, like git finds .git)")

	root.AddCommand(
		newLintCmd(),
		newCatalogCmd(),
		newRenderCmd(),
		newCheckCmd(),
		newDepsCmd(),
		newCoverageCmd(),
		newStaleCmd(),
		newLockCmd(),
		newUnlockCmd(),
		newReauditCmd(),
		newBuildOrderCmd(),
		newCommentCmd(),
		newServeCmd(),
		newFlagCmd(),
		newImplinkCmd(),
		newSkillsCmd(),
		newVersionCmd(),
	)
	return root
}

// resolveVersionInfo returns the version, commit, and build date to report.
// It prefers the values stamped in by goreleaser's -X ldflags and falls back
// to runtime/debug.ReadBuildInfo for plain "go build"/"go install" builds
// (which set no ldflags): a module version if one was recorded (e.g. a
// "go install ...@v1.2.3" build), otherwise "dev"; and the VCS
// revision/time Go embeds by default. Empty leftovers become "unknown" so no
// field is ever reported blank.
func resolveVersionInfo() (v, c, d string) {
	v, c, d = version, commit, date

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if c == "" {
					c = s.Value
				}
			case "vcs.time":
				if d == "" {
					d = s.Value
				}
			}
		}
		if v == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}

	if v == "" {
		v = "dev"
	}
	if c == "" {
		c = "unknown"
	}
	if d == "" {
		d = "unknown"
	}
	return v, c, d
}

// newVersionCmd prints the binary's version, commit, and build date. Unlike
// every other subcommand it never loads a project config — it describes the
// binary itself, so it works from anywhere, with or without a project.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the dossierx version, commit, and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			v, c, d := resolveVersionInfo()
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "dossierx %s\n", v)
			fmt.Fprintf(out, "  commit: %s\n", c)
			fmt.Fprintf(out, "  date:   %s\n", d)
			return nil
		},
	}
}

// requireKnownModule validates a --module against the modules this project
// actually declares (cfg.Modules). Every build-order/implink subcommand
// takes a --module, and an unknown or typo'd one would otherwise silently
// report an empty "not proposed yet"/"nothing linked yet" state and exit 0 —
// a success-looking result for a module that does not exist. A valid but
// unused module passes this check and still reaches its normal report.
func requireKnownModule(cfg *config.Config, module string) error {
	if containsStr(cfg.Modules, module) {
		return nil
	}
	return fmt.Errorf("unknown module %q; known: %s", module, strings.Join(cfg.Modules, ", "))
}

// ---------------------------------------------------------------------
// config discovery + claim loading
// ---------------------------------------------------------------------

// resolveConfigPath returns the path to project.config.yaml: the explicit
// --config value if set, otherwise the result of walking upward from the
// current directory looking for a file named project.config.yaml (the
// same discovery strategy git uses for .git).
func resolveConfigPath() (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, "project.config.yaml")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no project.config.yaml found (searched upward from %s): %w; pass --config explicitly", cwd, config.ErrNotFound)
}

// loadConfig proves CLI<->config plumbing: every subcommand loads and
// validates project.config.yaml before doing its work, and fails loudly
// and with a nonzero exit code if that fails.
func loadConfig() (*config.Config, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// loadClaims loads every claim under cfg's claims_dir.
func loadClaims(cfg *config.Config) ([]model.Claim, error) {
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		return nil, fmt.Errorf("load claims: %w", err)
	}
	return claims, nil
}

// loadConfigAndClaims is the common setup every subcommand beyond bare
// config-loading needs.
func loadConfigAndClaims() (*config.Config, []model.Claim, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	claims, err := loadClaims(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, claims, nil
}

// storePath is the on-disk location of the lock content-hash store,
// resolved relative to the config file's own directory (never cwd), same
// convention as claims_dir and viewer.template_overrides.
func storePath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
}

// claimsSentinelPath is the base path of the ONE project-wide claim-file
// write sentinel (lock.AcquireFileLock appends ".lock", so the real lock file
// is cfg.Dir()/.dossierx-claims.lock). It deliberately lives under cfg.Dir(),
// never cwd, and OUTSIDE claims_dir — so it is never itself loaded as a claim
// (LoadClaims only decodes *.yaml/*.yml, and the file is outside claims_dir
// besides) and, in a later serve phase, never trips a claims_dir file watcher.
//
// Unlike storePath/flagStorePath, each of which guards its OWN single JSON
// store, this sentinel guards EVERY claim file in the project. loader.SaveClaim
// rewrites a claim's entire file, so two writers that each loaded the same
// pre-mutation snapshot would have whichever saved last silently erase the
// other's change. Every claim-file writer (lock/unlock/check/flag/reaudit)
// therefore takes THIS sentinel FIRST — before any lock-store or flag-store
// sentinel — then re-reads claims inside it, so the global acquisition order
// is always claims -> lock-store -> flag-store and no AB-BA deadlock is
// possible. The critical section is bounded to load->mutate->SaveClaim;
// render/catalog/scan work runs after it is released.
//
// The path is defined once, in internal/lock.ClaimsSentinelPath, and delegated
// to here: internal/comments' CLI/serve ops (which cannot import package main)
// take the very same sentinel through lock.AcquireClaimsLock, so there is a
// single source of truth for which file every claim-file writer serializes on.
func claimsSentinelPath(cfg *config.Config) string {
	return lock.ClaimsSentinelPath(cfg)
}

// loadStoreForRead loads the lock content-hash store for a read-mostly command
// (currently "check"), first re-arming per-dependent baselines if the store
// predates them (a legacy migration — see lock.MigrateLegacyStore) and
// persisting that one-time re-arm. The load/migrate/Save runs under the same
// cross-process file lock "dossierx lock"/"dossierx reaudit" take, so the
// migration Save can never race their load-mutate-save on the shared store
// file. The returned store is safe to read (e.g. lock.DetectStale) after the
// lock is released: DetectStale only consults the already-loaded in-memory
// store, never the file again.
func loadStoreForRead(cfg *config.Config, claims []model.Claim) (*lock.Store, error) {
	release, err := lock.AcquireFileLock(storePath(cfg))
	if err != nil {
		return nil, err
	}
	defer release()

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return nil, err
	}
	if lock.MigrateLegacyStore(store, claims) {
		if err := store.Save(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

// catalogPath is where "dossierx catalog" (and "dossierx check") writes the built
// catalog.
func catalogPath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".catalog.json")
}

// renderOutPath is where "dossierx render" (and "dossierx check") writes the
// generated viewer.
func renderOutPath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), "viewer", "index.html")
}

// ---------------------------------------------------------------------
// lint
// ---------------------------------------------------------------------

func newLintCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run all lints against claims_dir",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			findings := lint.RunAll(claims, cfg)
			return reportLintFindings(cmd, findings, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print findings as JSON")
	return cmd
}

// reportLintFindings prints findings (text or JSON) and returns a non-nil
// error (for a nonzero exit code) if any error-severity finding is
// present. Warnings are reported but do not fail the command.
func reportLintFindings(cmd *cobra.Command, findings []lint.Finding, asJSON bool) error {
	// A nil findings slice JSON-encodes as "null"; coerce it to an empty
	// slice so "dossierx lint --json" always emits a JSON array — "[]" for a
	// clean run — which is what machine consumers parse. (Text output is
	// unaffected: len(nil) and len([]) are both 0.)
	if findings == nil {
		findings = []lint.Finding{}
	}

	errCount := 0
	for _, f := range findings {
		if f.Severity != lint.SeverityWarning {
			errCount++
		}
	}

	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(findings); err != nil {
			return fmt.Errorf("lint: encode findings: %w", err)
		}
	} else {
		out := cmd.OutOrStdout()
		if len(findings) == 0 {
			fmt.Fprintln(out, "lint: 0 findings")
		} else {
			for _, f := range findings {
				fmt.Fprintf(out, "[%s] %s: %s: %s\n", f.Severity, f.LintName, f.ClaimID, f.Message)
			}
			fmt.Fprintf(out, "lint: %d finding(s), %d error(s)\n", len(findings), errCount)
		}
	}

	if errCount > 0 {
		return fmt.Errorf("lint: %d error-level finding(s)", errCount)
	}
	return nil
}

// ---------------------------------------------------------------------
// catalog
// ---------------------------------------------------------------------

func newCatalogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "Build .catalog.json from claims",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			return runCatalog(cmd, cfg, claims)
		},
	}
}

// enforceMockupGate runs the raw-html-scope mockup gate (the checkMockupGate
// subset — error-severity findings only, deliberately NOT the full lint suite,
// so draft-authoring relationship WARNINGS still never block a plain
// render/catalog) and returns a non-nil error when any claim's RawHTML fails
// it. render and catalog both call this before doing their work so neither
// ever publishes live HTML/JS from a draft, unreviewed, or non-allowlisted
// mockup into the client-shared viewer (DX-AUD-08). Callers wrap the returned
// error with their own "render:"/"catalog:" prefix; findings are printed to
// stderr so the failure is self-explanatory.
func enforceMockupGate(cmd *cobra.Command, cfg *config.Config, claims []model.Claim) error {
	findings := lint.MockupGateFindings(claims, cfg)
	if len(findings) == 0 {
		return nil
	}
	for _, f := range findings {
		fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s: %s: %s\n", f.Severity, f.LintName, f.ClaimID, f.Message)
	}
	return fmt.Errorf("%d raw_html mockup-gate error(s); review, allowlist, and lock the mockup before rendering", len(findings))
}

func runCatalog(cmd *cobra.Command, cfg *config.Config, claims []model.Claim) error {
	if err := enforceMockupGate(cmd, cfg, claims); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		return fmt.Errorf("catalog: build: %w", err)
	}
	path := catalogPath(cfg)
	if err := catalog.WriteJSON(cat, path); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "catalog: wrote %s (%d claim(s))\n", path, len(claims))
	return nil
}

// ---------------------------------------------------------------------
// render
// ---------------------------------------------------------------------

func newRenderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "render",
		Short: "Generate viewer/index.html from the catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			return runRender(cmd, cfg, claims)
		},
	}
}

func runRender(cmd *cobra.Command, cfg *config.Config, claims []model.Claim) error {
	if err := enforceMockupGate(cmd, cfg, claims); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		return fmt.Errorf("render: build catalog: %w", err)
	}
	html, err := render.Render(cat, cfg)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	path := renderOutPath(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("render: create output dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return fmt.Errorf("render: write %s: %w", path, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "render: wrote %s\n", path)
	return nil
}

// ---------------------------------------------------------------------
// check (lint + catalog + render, stop at first failure)
// ---------------------------------------------------------------------

// reconcileReviewPending runs "check"'s only claim-file-writing phase under
// the project-wide claims sentinel: it loads claims, flips every locked claim
// whose mirrors/rests_on content has drifted since its last lock or confirmed
// reaudit — OR that carries an unresolved comment thread — to
// locked+review_pending, and persists each flip back to the claim's
// own file so it survives to the next run and shows up in "dossierx stale". It
// returns the reconciled claims for the caller's (non-writing)
// lint/catalog/render/scan pipeline. Like DetectStale it only ever SETS
// review_pending; clearing it is reaudit --confirm / unlock / resolving the
// last open thread's job.
//
// The claims sentinel is taken FIRST — before loadStoreForRead's own
// lock-store sentinel, preserving the global claims -> lock-store order — and
// released (via defer, when this function returns) BEFORE the caller renders,
// so a full render never blocks a concurrent agent CLI write. loadStoreForRead
// also re-arms a legacy (pre-versioning) store's per-dependent baselines from
// current content on the first run after upgrade, so already-locked claims
// regain drift detection immediately (no manual re-lock) without any spurious
// review_pending — see lock.MigrateLegacyStore.
//
// Plain loader.SaveClaim (not SaveClaimIfUnchanged) is correct here: the
// claims sentinel is held across this whole load->detect->save, so no
// cooperating writer can change a claim file underneath the loop.
func reconcileReviewPending(cfg *config.Config) ([]model.Claim, error) {
	releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
	if err != nil {
		return nil, err
	}
	defer releaseClaims()

	claims, err := loadClaims(cfg)
	if err != nil {
		return nil, err
	}
	store, err := loadStoreForRead(cfg, claims)
	if err != nil {
		return nil, err
	}
	updated := lock.DetectStale(claims, store)
	for i := range updated {
		// Third review_pending trigger, reconciled alongside dependency drift:
		// a LOCKED claim carrying an open comment thread is review_pending too.
		// The lock gate forbids locking WITH an open thread, but a thread can
		// be opened on an already-locked claim (or hand-authored into its
		// YAML), so check reconciles it here. Purely additive — like
		// DetectStale it only SETS the flag, never clears one.
		if updated[i].Status == model.StatusLocked && updated[i].HasOpenThreads() {
			updated[i].ReviewPending = true
		}
		if updated[i].ReviewPending != claims[i].ReviewPending {
			if err := loader.SaveClaim(updated[i]); err != nil {
				return nil, fmt.Errorf("persist review_pending for %q: %w", updated[i].ID, err)
			}
		}
	}
	return updated, nil
}

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Run lint, catalog, and render in one shot, stopping at first failure",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			// Parse-check claims up front, OUTSIDE the "check:"-wrapped reconcile
			// below, so a malformed claim YAML is reported as "load claims: ..."
			// (v0.1.2's unprefixed shape) rather than "check: load claims: ...":
			// loading claims is a precondition that predates the check pipeline,
			// so the "check:" wrap must not attach to it. reconcile re-reads
			// claims inside the project-wide claims sentinel (the Phase-0 write
			// discipline); this early load only fixes the error's provenance.
			if _, err := loadClaims(cfg); err != nil {
				return err
			}

			// Dependency-drift detection + review_pending persistence is
			// "check"'s only claim-file-writing phase; it runs under the
			// project-wide claims sentinel and releases it before the
			// (non-writing) lint/catalog/render/scan pipeline below, so a full
			// render never blocks a concurrent agent CLI write. It stays here,
			// in the command, rather than inside check.Run: Run takes the
			// already-reconciled claims so ALL claim-file persistence — and the
			// Phase-0 claims-lock discipline guarding it — stays with the caller
			// that holds the sentinel (serve reconciles at startup the same way
			// before handing the claims to the same check.Run).
			claims, err := reconcileReviewPending(cfg)
			if err != nil {
				return fmt.Errorf("check: %w", err)
			}

			// check.Run is the shared, value-returning pipeline (lint, catalog,
			// render, impl-link scan, and the non-blocking per-module
			// reporting) that "dossierx serve" reuses without this fail-fast
			// contract. Here we APPLY the fail-fast contract: format the Result
			// to the terminal — byte-for-byte the output the previously inlined
			// RunE produced (guarded by check_parity_test.go) — and return the
			// first failing step's error wrapped "check: %w", exactly as each
			// inlined step used to. Run's error is unprefixed; the wrap here is
			// the single choke point that used to live at every call site.
			res, runErr := check.Run(claims, cfg)
			formatCheckResult(cmd, res)
			if runErr != nil {
				return fmt.Errorf("check: %w", runErr)
			}
			return nil
		},
	}
}

// formatCheckResult writes res to the command's stdout/stderr in the exact
// segment order — and with the exact format strings — the pre-extraction
// newCheckCmd RunE used, so "dossierx check" output stays byte-identical (the
// guard is check_parity_test.go). It prints only the segments the run
// actually reached: an empty CatalogPath/RenderPath, a false OK, an empty
// reporting slice are precisely how a fail-fast stop reproduces as "that line
// never printed". It never decides the exit code — the RunE returns
// check.Run's error for that.
func formatCheckResult(cmd *cobra.Command, res check.Result) {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	// Lint block — always present (lint is the first step, so LintFindings is
	// always populated). Reuse the shared reporter for byte-identical output;
	// its error return is intentionally discarded here — check.Run already
	// surfaced the fail-fast error, and the RunE returns that, not this.
	_ = reportLintFindings(cmd, res.LintFindings, false)

	// Catalog/render write confirmations. Empty paths mean the run stopped
	// before that write (e.g. a lint error), so the line is correctly skipped.
	if res.CatalogPath != "" {
		fmt.Fprintf(out, "catalog: wrote %s (%d claim(s))\n", res.CatalogPath, res.CatalogCount)
	}
	if res.RenderPath != "" {
		fmt.Fprintf(out, "render: wrote %s\n", res.RenderPath)
	}

	// Impl-link scan errors print (to stderr) whether or not the run then
	// failed — they preceded the wrapped "check:" error in the old RunE too.
	for _, e := range res.ScanErrors {
		fmt.Fprintf(errOut, "impl-links: scan error in %s:%d: dossierx-claim references %q: %s\n", e.File, e.Line, e.ClaimID, e.Message)
	}

	if !res.OK {
		return
	}

	// Success tail: the scan summary (only on a clean scan, and only when a
	// file was actually scanned), "check: OK", then the non-blocking
	// per-module reporting — orientation notes, open comments, impl-link
	// status, and the next-steps advisory — in the same order as before.
	if res.ScanFilesScanned > 0 {
		fmt.Fprintln(out, res.ScanSummary)
	}
	fmt.Fprintln(out, "check: OK")
	for _, line := range res.OrientationNotes {
		fmt.Fprintln(out, line)
	}
	if len(res.OpenComments) > 0 {
		modules := make([]string, 0, len(res.OpenComments))
		for m := range res.OpenComments {
			modules = append(modules, m)
		}
		sort.Strings(modules)
		for _, m := range modules {
			fmt.Fprintf(out, "open comments: module %q: %d\n", m, res.OpenComments[m])
		}
	}
	for _, line := range res.ImplinkStatusStdout {
		fmt.Fprintln(out, line)
	}
	for _, line := range res.ImplinkStatusStderr {
		fmt.Fprintln(errOut, line)
	}
	if len(res.NextSteps) > 0 {
		fmt.Fprintln(out, "next steps:")
		for _, h := range res.NextSteps {
			fmt.Fprintf(out, "  %s\n", h)
		}
	}
}

// ---------------------------------------------------------------------
// deps <id>
// ---------------------------------------------------------------------

func newDepsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deps <id>",
		Short: "Print a claim's edge graph in both directions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			_, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				fmt.Fprintf(cmd.ErrOrStderr(), "deps: claim %q not found\n", id)
				os.Exit(2)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "deps: %s\n", claim.ID)
			fmt.Fprintf(out, "  outgoing mirrors:   %v\n", claim.Mirrors)
			fmt.Fprintf(out, "  outgoing rests_on:  %v\n", claim.RestsOn)
			if claim.Governed.Type != "" {
				fmt.Fprintf(out, "  governed_by:        %s", claim.Governed.Type)
				if claim.Governed.Reason != "" {
					fmt.Fprintf(out, " (%s)", claim.Governed.Reason)
				}
				fmt.Fprintln(out)
			} else {
				fmt.Fprintln(out, "  governed_by:        (unset)")
			}

			var inMirrors, inRestsOn []string
			for _, c := range claims {
				if c.ID == claim.ID {
					continue
				}
				if containsStr(c.Mirrors, claim.ID) {
					inMirrors = append(inMirrors, c.ID)
				}
				if containsStr(c.RestsOn, claim.ID) {
					inRestsOn = append(inRestsOn, c.ID)
				}
			}
			sort.Strings(inMirrors)
			sort.Strings(inRestsOn)
			fmt.Fprintf(out, "  incoming mirrors:   %v\n", inMirrors)
			fmt.Fprintf(out, "  incoming rests_on:  %v\n", inRestsOn)
			return nil
		},
	}
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// coverage
// ---------------------------------------------------------------------

func newCoverageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "coverage",
		Short: "Report the percentage of claims carrying a migrated_from note",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			total := len(claims)
			withNote := 0
			for _, c := range claims {
				if c.MigratedFrom != "" {
					withNote++
				}
			}
			pct := 0.0
			if total > 0 {
				pct = 100 * float64(withNote) / float64(total)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "coverage: %d/%d claim(s) carry migrated_from (%.1f%%)\n", withNote, total, pct)
			return nil
		},
	}
}

// ---------------------------------------------------------------------
// stale
// ---------------------------------------------------------------------

func newStaleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stale",
		Short: "List locked claims currently flagged review_pending",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			anyLocked := false
			var ids []string
			for _, c := range claims {
				if c.Status != model.StatusLocked {
					continue
				}
				anyLocked = true
				if c.ReviewPending {
					ids = append(ids, c.ID)
				}
			}
			sort.Strings(ids)
			if !anyLocked {
				fmt.Fprintln(out, "stale: nothing locked")
				return nil
			}
			if len(ids) == 0 {
				fmt.Fprintln(out, "stale: 0 claim(s) flagged review_pending")
				return nil
			}
			for _, id := range ids {
				fmt.Fprintf(out, "review_pending: %s\n", id)
			}
			fmt.Fprintf(out, "stale: %d claim(s) flagged review_pending\n", len(ids))
			return nil
		},
	}
}

// ---------------------------------------------------------------------
// lock / unlock
// ---------------------------------------------------------------------

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock <id>",
		Short: "Lock a draft claim (refused if lint fails)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			// Claim-file write discipline (Phase 0): take the project-wide
			// claims sentinel FIRST — before the lock-store sentinel below —
			// and load claims INSIDE it, so this whole load->mutate->SaveClaim
			// runs against a snapshot no concurrent claim-file writer can have
			// changed underneath us (loader.SaveClaim rewrites the entire file,
			// so a stale snapshot would silently erase a co-writer's edit).
			// Acquiring claims before lock-store keeps the global order
			// (claims -> lock-store -> flag-store) deadlock-free.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			defer releaseClaims()

			claims, err := loadClaims(cfg)
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return fmt.Errorf("lock: claim %q not found: %w", id, errClaimNotFound)
			}
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return fmt.Errorf("lock: %w", err)
			}

			// Serialize concurrent "dossierx lock"/"dossierx reaudit --confirm"
			// invocations that share this project's store file: each does
			// LoadStore -> mutate -> Save, and without this lock two
			// concurrent runs (e.g. locking two different claims in
			// parallel) would race on the store's Hashes/LockedAt map,
			// silently losing whichever saved first.
			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			defer release()

			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return fmt.Errorf("lock: %w", err)
			}

			// Re-arm a legacy (pre-versioning) store's per-dependent baselines
			// from current content before recording this claim's own, so an
			// upgrade caught mid-lock still restores drift detection for every
			// already-locked claim (not just this one) — see
			// lock.MigrateLegacyStore. Persisted here rather than relying on the
			// Save below so the re-arm survives even a subsequently refused lock.
			if lock.MigrateLegacyStore(store, claims) {
				if err := store.Save(); err != nil {
					return fmt.Errorf("lock: %w", err)
				}
			}

			updated, err := lock.Lock(claim, claims, cfg, store)
			if err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			if err := loader.SaveClaimIfUnchanged(updated, token); err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			if err := store.Save(); err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "lock: %s is now locked\n", id)
			return nil
		},
	}
}

func newUnlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlock <id>",
		Short: "Unlock a locked claim back to draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			// Claim-file write discipline (Phase 0): take the project-wide
			// claims sentinel FIRST — before the flag-store sentinel below —
			// and load claims INSIDE it, so this load->mutate->SaveClaim runs
			// against a snapshot no concurrent claim-file writer can change
			// underneath us. claims-then-flag-store keeps the global order
			// (claims -> lock-store -> flag-store) deadlock-free.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return fmt.Errorf("unlock: %w", err)
			}
			defer releaseClaims()

			claims, err := loadClaims(cfg)
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return fmt.Errorf("unlock: claim %q not found: %w", id, errClaimNotFound)
			}
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return fmt.Errorf("unlock: %w", err)
			}

			// Unlocking must also drop any pending "dossierx flag" trigger for
			// this claim (DX-AUD-10): a flag records an agent's before/after
			// assertion against the claim's currently-locked body, and once the
			// claim is unlocked and (presumably) edited, that stale assertion
			// must not survive to be silently applied by a later
			// drift-triggered "dossierx reaudit --confirm" after the claim is
			// relocked. Serialize against concurrent flag/reaudit invocations
			// on the shared flag-store file, the same way newLockCmd and
			// newReauditCmd do (AcquireFileLock). Shared lock.Store.Hashes
			// entries are deliberately NOT touched here — they belong to
			// co-dependents and are harmlessly overwritten on the next relock.
			release, err := lock.AcquireFileLock(flagStorePath(cfg))
			if err != nil {
				return fmt.Errorf("unlock: %w", err)
			}
			defer release()

			// Clearing the pending flag is best-effort: unlock is the recovery
			// escape hatch ("get this claim back to draft so I can fix things")
			// and must never be blocked by an unrelated broken flag-store file.
			// A missing store is already the empty-store case (LoadFlagStore
			// treats it as fresh — nothing to clear, silently). An existing but
			// unparseable/unreadable store is warned about on stderr and skipped
			// rather than fatal, so the status revert below still happens. Only a
			// failing Save of a store we DID read + mutate is a hard error (that
			// is a real write failure, not the corruption case this tolerates).
			// The lock-acquire above is still fatal on error: that is contention,
			// not corruption.
			if flagStore, ferr := reaudit.LoadFlagStore(flagStorePath(cfg)); ferr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read flag store (%v); a pending flag for %s was not cleared\n", ferr, id)
			} else if _, flagged := flagStore.Flags[id]; flagged {
				delete(flagStore.Flags, id)
				if err := flagStore.Save(); err != nil {
					return fmt.Errorf("unlock: %w", err)
				}
			}

			updated := lock.Unlock(claim)
			if err := loader.SaveClaimIfUnchanged(updated, token); err != nil {
				return fmt.Errorf("unlock: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "unlock: %s is now draft\n", id)
			return nil
		},
	}
}

// ---------------------------------------------------------------------
// reaudit <id> [--confirm]
// ---------------------------------------------------------------------

func newReauditCmd() *cobra.Command {
	var confirm bool
	cmd := &cobra.Command{
		Use:   "reaudit <id>",
		Short: "Propose (and, with --confirm, apply) a diff for a review_pending claim",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				fmt.Fprintf(cmd.ErrOrStderr(), "reaudit: claim %q not found\n", id)
				os.Exit(2)
			}

			// Per SPEC, reaudit is only ever valid on a locked+review_pending
			// claim; anything else is exit 2.
			if claim.Status != model.StatusLocked || !claim.ReviewPending {
				fmt.Fprintf(cmd.ErrOrStderr(), "reaudit: claim %q is not locked+review_pending\n", id)
				os.Exit(2)
			}

			// Claim-file write discipline (Phase 0): the two guards above ran
			// on a lock-free load DELIBERATELY — they os.Exit(2) (which skips
			// defers) only while no file lock is held, so nothing is leaked.
			// Now take the project-wide claims sentinel FIRST (before the
			// lock-store and flag-store sentinels below, preserving the global
			// claims -> lock-store -> flag-store order) and RE-READ claims
			// inside it, so a confirmed reaudit's load->mutate->SaveClaim runs
			// against a snapshot no concurrent claim-file writer can change
			// underneath us.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			defer releaseClaims()

			claims, err = loadClaims(cfg)
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			claim, ok = loader.FindByID(claims, id)
			if !ok {
				// Vanished between the guard and the sentinel (an out-of-band
				// delete). Refuse via the sentinel error so the deferred
				// release still runs; main() maps it to the same exit 2.
				return fmt.Errorf("reaudit: claim %q not found: %w", id, errClaimNotFound)
			}
			if claim.Status != model.StatusLocked || !claim.ReviewPending {
				return fmt.Errorf("reaudit: claim %q is not locked+review_pending: %w", id, errWrongState)
			}
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}

			// See newLockCmd's comment: serializes against any concurrent
			// "dossierx lock"/"dossierx reaudit --confirm" invocation touching the
			// same store file, so a confirmed reaudit's store.Save() below
			// never races another process's load-mutate-save on the same
			// Hashes/LockedAt map.
			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			defer release()

			// Same reasoning, applied to the (separate) pending-flag store:
			// a confirmed reaudit for a flag-sourced claim deletes that
			// claim's entry below, and two concurrent reaudit/flag
			// invocations must not race on that shared file either.
			flagRelease, err := lock.AcquireFileLock(flagStorePath(cfg))
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			defer flagRelease()

			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			// Re-arm a legacy (pre-versioning) store's per-dependent baselines
			// from current content — see lock.MigrateLegacyStore. Persisted here
			// (not only on the --confirm path below) so a mere propose still
			// restores drift detection for the project's already-locked claims.
			if lock.MigrateLegacyStore(store, claims) {
				if err := store.Save(); err != nil {
					return fmt.Errorf("reaudit: %w", err)
				}
			}
			flagStore, err := reaudit.LoadFlagStore(flagStorePath(cfg))
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}

			// Two trigger sources converge here: a claim with a pending
			// "dossierx flag" entry gets the real, ready-to-review diff
			// ProposeFlagDiff builds from it; every other review_pending
			// claim (the pre-existing, and still only, case for a project
			// that has never used "dossierx flag") keeps going through
			// ProposeDiff's dependency-diff stub exactly as before.
			pendingFlag, flagged := flagStore.Flags[id]

			// A claim whose ONLY pending trigger is an open comment thread has
			// nothing for reaudit to do: reaudit reviews a proposed CONTENT
			// change (a drifted dependency, or a "dossierx flag"), and a comment
			// thread is discussion, not an edit to diff-and-confirm. Refuse with
			// exit 2 BEFORE proposing or writing anything. Crucially this point
			// is PAST both file locks (store + flag, acquired above), so it must
			// NOT os.Exit(2) — that skips the deferred releases and leaks the
			// two held locks. Returning a wrapped errWrongState lets the defers
			// run and main() map it to exit 2. The remedy is to resolve the
			// thread, which clears review_pending on its own.
			if drift, flag, open := comments.PendingTriggers(claim, claims, store, flagStore); !drift && !flag && open > 0 {
				return fmt.Errorf("reaudit: claim %q is review_pending only because of %d open comment thread(s); resolve them with \"dossierx comment resolve %s <thread-id>\" — nothing to reaudit: %w", id, open, id, errWrongState)
			}

			var diff reaudit.Diff
			if flagged {
				diff, err = reaudit.ProposeFlagDiff(claim, pendingFlag)
			} else {
				changedDep := pickChangedDependency(claim, claims, store)
				diff, err = reaudit.ProposeDiff(claim, changedDep)
			}
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "reaudit: %s (no_change=%v)\n", diff.ClaimID, diff.NoChange)
			fmt.Fprintf(out, "note: %s\n", diff.Note)
			fmt.Fprintln(out, "---")
			fmt.Fprintln(out, diff.Body)
			fmt.Fprintln(out, "---")

			if !confirm {
				fmt.Fprintln(out, "reaudit: not applied (pass --confirm to apply)")
				return nil
			}

			applied, err := reaudit.Apply(claim, diff)
			if err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			// Re-baseline this claim's dependency hashes and refresh its lock
			// timestamp — the drift-clearing half of the old ClearReviewPending.
			// review_pending is deliberately NOT hard-cleared here: it becomes
			// the Recompute verdict below, so a claim still carrying an
			// independent open comment thread stays review_pending even though
			// this confirmed reaudit cleared the drift/flag that prompted it
			// (a comment is a third trigger reaudit does not resolve).
			lock.RefreshBaseline(applied, claims, store)
			if flagged {
				// A flag is a one-shot trigger (see PendingFlag's doc comment):
				// remove it BEFORE recomputing so the verdict sees it cleared,
				// and so a future dependency-drift reaudit on this same claim
				// doesn't mistake a stale flag for a still-pending one.
				delete(flagStore.Flags, id)
			}
			applied.ReviewPending = comments.Recompute(applied, claims, store, flagStore)
			if err := loader.SaveClaimIfUnchanged(applied, token); err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			if err := store.Save(); err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			if flagged {
				if err := flagStore.Save(); err != nil {
					return fmt.Errorf("reaudit: %w", err)
				}
			}
			if applied.ReviewPending {
				fmt.Fprintf(out, "reaudit: %s applied; review_pending retained (open comment thread(s) remain — resolve them to clear)\n", id)
			} else {
				fmt.Fprintf(out, "reaudit: %s applied, review_pending cleared\n", id)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the proposed diff (otherwise only prints it)")
	return cmd
}

// pickChangedDependency picks a dependency to report as "the" changed
// dependency for a reaudit proposal: the first mirrors/rests_on target
// whose current content hash no longer matches the stored baseline, or —
// if none can be identified that way (e.g. an empty store) — simply the
// first declared dependency, since ProposeDiff's stub only uses this for
// its note text.
func pickChangedDependency(claim model.Claim, claims []model.Claim, store *lock.Store) model.Claim {
	deps := append(append([]string(nil), claim.Mirrors...), claim.RestsOn...)
	for _, dep := range deps {
		depClaim, ok := loader.FindByID(claims, dep)
		if !ok {
			continue
		}
		if stored, known := store.Baseline(claim.ID, dep); known && stored != lock.ContentHash(depClaim) {
			return depClaim
		}
	}
	if len(deps) > 0 {
		if depClaim, ok := loader.FindByID(claims, deps[0]); ok {
			return depClaim
		}
	}
	return model.Claim{}
}
