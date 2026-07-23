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
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
	"github.com/BarterX-Tech/dossierx/internal/render"
)

var configPath string

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
		// A missing config file is its own exit code (2) distinct from
		// other failures (1), so scripts/CI can tell "nothing to load"
		// apart from "loaded but invalid" or "ran but failed".
		if errors.Is(err, config.ErrNotFound) {
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
		newFlagCmd(),
		newImplinkCmd(),
		newSkillsCmd(),
	)
	return root
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

func runCatalog(cmd *cobra.Command, cfg *config.Config, claims []model.Claim) error {
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

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Run lint, catalog, and render in one shot, stopping at first failure",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}

			// Dependency-drift detection: flip locked claims whose
			// mirrors/rests_on content has changed since the last lock or
			// confirmed reaudit to locked+review_pending. Persist the flag
			// change back to the claim's own file so it survives to the
			// next run and shows up in "dossierx stale".
			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return fmt.Errorf("check: %w", err)
			}
			updated := lock.DetectStale(claims, store)
			for i := range updated {
				if updated[i].ReviewPending != claims[i].ReviewPending {
					if err := loader.SaveClaim(updated[i]); err != nil {
						return fmt.Errorf("check: persist review_pending for %q: %w", updated[i].ID, err)
					}
				}
			}
			claims = updated

			findings := lint.RunAll(claims, cfg)
			if err := reportLintFindings(cmd, findings, false); err != nil {
				return fmt.Errorf("check: %w", err)
			}
			if err := runCatalog(cmd, cfg, claims); err != nil {
				return fmt.Errorf("check: %w", err)
			}
			if err := runRender(cmd, cfg, claims); err != nil {
				return fmt.Errorf("check: %w", err)
			}

			// Fourth step, and the one exception to "everything past this
			// point is non-blocking reporting": scan cfg.SourceDirs for
			// "dossierx-claim: <id>" comments and reconcile each valid one into
			// internal/implink's artifact automatically (same Set logic any
			// explicit "dossierx implink set" call already goes through — see
			// implink.Scan's doc comment). A tag naming an unknown or
			// not-yet-locked claim is a hard check FAILURE, not a warning —
			// the whole point of this step is that an unbacked or stale tag
			// can never sit silently wrong in the codebase. Entirely silent
			// (and this hard-fail path unreachable) for a project that has
			// never set source_dirs at all.
			scanReport, err := implink.Scan(claims, cfg)
			if err != nil {
				return fmt.Errorf("check: %w", err)
			}
			if len(scanReport.Errors) > 0 {
				for _, e := range scanReport.Errors {
					fmt.Fprintf(cmd.ErrOrStderr(), "impl-links: scan error in %s:%d: dossierx-claim references %q: %s\n", e.File, e.Line, e.ClaimID, e.Message)
				}
				return fmt.Errorf("check: %d impl-link scan error(s)", len(scanReport.Errors))
			}
			if scanReport.FilesScanned > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), scanReport.Summary())
			}

			fmt.Fprintln(cmd.OutOrStdout(), "check: OK")

			reportOrientationNotes(cmd, cfg, claims)

			// Remaining steps are purely additional, non-blocking reporting
			// — the same relationship lint WARNINGS already have to a
			// successful check — and are entirely silent for any project
			// that has never opted into the feature they report on.
			implinkHints := reportImplinkStatus(cmd, cfg, claims)
			reportNextSteps(cmd, cfg, claims, implinkHints)
			return nil
		},
	}
}

// orientationCounts accumulates one module's orientation-note claims,
// broken down by facet, for reportOrientationNotes below. Named (rather
// than an inline anonymous struct) because Go does not allow methods on
// anonymous struct types, and orderedFacets below needs to be a method.
type orientationCounts struct {
	total   int
	byFacet map[string]int
}

// orderedFacets returns c's facet keys sorted, so reportOrientationNotes's
// output is deterministic across runs (map iteration order is not).
func (c *orientationCounts) orderedFacets() []string {
	facets := make([]string, 0, len(c.byFacet))
	for f := range c.byFacet {
		facets = append(facets, f)
	}
	sort.Strings(facets)
	return facets
}

// reportOrientationNotes prints one non-blocking line per module that has
// at least one orientation-note claim (module.overview.* and/or
// kind: orientation-note claims in a regular facet), broken down by
// facet — so "dossierx check" alone is enough to confirm an orientation set
// exists for a module before diving into its other claims, per this
// engine's "the one command you run routinely" contract.
func reportOrientationNotes(cmd *cobra.Command, cfg *config.Config, claims []model.Claim) {
	byModule := map[string]*orientationCounts{}
	for _, c := range claims {
		if c.EffectiveKind() != model.KindOrientationNote {
			continue
		}
		cnt, ok := byModule[c.Module]
		if !ok {
			cnt = &orientationCounts{byFacet: map[string]int{}}
			byModule[c.Module] = cnt
		}
		cnt.total++
		cnt.byFacet[c.Facet]++
	}

	out := cmd.OutOrStdout()
	for _, module := range cfg.Modules {
		cnt, ok := byModule[module]
		if !ok {
			continue
		}
		var parts []string
		for _, f := range cnt.orderedFacets() {
			parts = append(parts, fmt.Sprintf("%d in %s", cnt.byFacet[f], f))
		}
		fmt.Fprintf(out, "orientation notes: module %q: %d (%s)\n", module, cnt.total, strings.Join(parts, ", "))
	}
}

// reportImplinkStatus prints one implink.Status summary line (plus one line
// per drifted entry) for every module in cfg.Modules that has an existing
// implementation-link artifact on disk, silently skipping every module that
// has none at all — the "zero-cost/silent when unused" contract this
// feature must uphold, mirroring internal/render's attachBuildOrders (a
// module with no build-order artifact gets nothing extra rendered, either).
// Any error other than "no artifact yet" is reported to stderr but never
// turns into a non-nil return from newCheckCmd's RunE — this step is purely
// additional reporting, never a reason "dossierx check" itself fails.
// reportImplinkStatus prints the per-module drift/unlinked report (as
// before) and additionally returns one "next steps" hint line per drifted
// entry and per module with any unlinked claims, so newCheckCmd's final
// summary can point at exactly what to run next instead of a reader having
// to infer it from the raw report above.
func reportImplinkStatus(cmd *cobra.Command, cfg *config.Config, claims []model.Claim) []string {
	out := cmd.OutOrStdout()
	var hints []string
	for _, module := range cfg.Modules {
		report, err := implink.Status(claims, cfg, module)
		if err != nil {
			if errors.Is(err, implink.ErrNoArtifact) {
				continue
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "check: implink status for %q: %v\n", module, err)
			continue
		}
		fmt.Fprintln(out, report.Summary())
		for _, d := range report.Drifted {
			fmt.Fprintf(out, "  drifted: %s %s: %s\n", d.ClaimID, d.File, d.Reason)
			hints = append(hints, fmt.Sprintf("%s is drifted -> re-tag or dossierx implink set --module %s --claim %s --file %s", d.ClaimID, module, d.ClaimID, d.File))
		}
		for _, id := range report.UnlinkedIDs {
			fmt.Fprintf(out, "  unlinked: %s\n", id)
		}
		if len(report.UnlinkedIDs) > 0 {
			hints = append(hints, fmt.Sprintf("%d claim(s) in module %q have no code link yet -> add a dossierx-claim tag or dossierx implink set", len(report.UnlinkedIDs), module))
		}
	}
	return hints
}

// reportNextSteps prints a short, always-present-when-non-empty "what to
// run next" block at the very end of "dossierx check" — the answer to "I don't
// want to have to remember which of several commands applies": run check,
// read this block, do what it says, run check again. It is derived
// entirely from state check already computed (draft/review_pending claims,
// implinkHints from reportImplinkStatus above, and per-module Build Order
// readiness) rather than requiring a human to cross-reference several
// separate reports by hand.
func reportNextSteps(cmd *cobra.Command, cfg *config.Config, claims []model.Claim, implinkHints []string) {
	var hints []string

	var draftIDs []string
	var reviewPendingIDs []string
	for _, c := range claims {
		switch {
		case c.Status == model.StatusDraft:
			draftIDs = append(draftIDs, c.ID)
		case c.Status == model.StatusLocked && c.ReviewPending:
			reviewPendingIDs = append(reviewPendingIDs, c.ID)
		}
	}
	if len(draftIDs) > 0 {
		hints = append(hints, fmt.Sprintf("%d claim(s) still draft -> dossierx lock <id> (e.g. %s)", len(draftIDs), draftIDs[0]))
	}
	if len(reviewPendingIDs) > 0 {
		hints = append(hints, fmt.Sprintf("%d claim(s) review_pending -> dossierx reaudit <id> (e.g. %s)", len(reviewPendingIDs), reviewPendingIDs[0]))
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
			if c.Status != model.StatusLocked || c.ReviewPending {
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

	if len(hints) == 0 {
		return
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "next steps:")
	for _, h := range hints {
		fmt.Fprintf(out, "  %s\n", h)
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
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return fmt.Errorf("lock: claim %q not found", id)
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

			updated, err := lock.Lock(claim, claims, cfg, store)
			if err != nil {
				return fmt.Errorf("lock: %w", err)
			}
			if err := loader.SaveClaim(updated); err != nil {
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
			_, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return fmt.Errorf("unlock: claim %q not found", id)
			}

			updated := lock.Unlock(claim)
			if err := loader.SaveClaim(updated); err != nil {
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
			applied = lock.ClearReviewPending(applied, claims, store)
			if err := loader.SaveClaim(applied); err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			if err := store.Save(); err != nil {
				return fmt.Errorf("reaudit: %w", err)
			}
			if flagged {
				// A flag is a one-shot trigger (see PendingFlag's doc
				// comment): once its reaudit is confirmed, remove it so a
				// future dependency-drift reaudit on this same claim
				// doesn't mistake a stale flag for a still-pending one.
				delete(flagStore.Flags, id)
				if err := flagStore.Save(); err != nil {
					return fmt.Errorf("reaudit: %w", err)
				}
			}
			fmt.Fprintf(out, "reaudit: %s applied, review_pending cleared\n", id)
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
		if stored, known := store.Hashes[dep]; known && stored != lock.ContentHash(depClaim) {
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
