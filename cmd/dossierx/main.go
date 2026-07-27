// Command dossierx is the CLI entrypoint for the engine. All project-specific
// behavior comes from the --config file (project.config.yaml); this
// binary itself has zero hardcoded references to any particular project.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

var configPath string

// versionFlag is the value of the root's own --version flag. It is a package
// global for the same reason configPath and formatFlag are: cobra binds flags to
// addresses at command-construction time, and newRootCmd() re-registers it on
// every call, so an in-process test that builds a fresh root never inherits the
// previous test's value.
var versionFlag bool

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

// errClaimNotFound and errWrongState are sentinels the claim lifecycle
// subcommands wrap into their not-found / wrong-state errors so main() can
// map both to exit code 2 — the documented "not found / not in the right
// state" family (see README's exit-codes table). They are signalled rather
// than reached through a bare os.Exit because every one of these commands
// holds deferred file-lock releases that os.Exit would skip; returning the
// sentinel lets RunE unwind cleanly, and (since v0.3.0) lets the failure be
// reported as an envelope like every other failure.
var (
	errClaimNotFound = errors.New("claim not found")
	errWrongState    = errors.New("claim not in the required state")
)

// main is deliberately not exercised by this package's own tests: calling it
// in-process would terminate the test binary itself. Everything it does beyond
// the os.Exit is in runCLI, which the in-process helper (cli_inprocess_test.go's
// execCLI) drives directly, so the two paths render identically; the real
// process exit CODES are asserted end-to-end by tests/, which execs a built
// binary as a subprocess (tests/check_exit_test.go, tests/cli_uxaudit_test.go).
func main() {
	if err := runCLI(newRootCmd()); err != nil {
		// Exit 2 is the "not found / not in the right state" family: a
		// missing config file, an id that doesn't exist, or a claim that
		// isn't in the state a command requires. Everything else (lint
		// errors, validation failures, write errors) is exit 1, so scripts
		// and CI can tell "nothing/nobody there" apart from "loaded but
		// failed". v0.3.0 adds no third family — see cliout.ExitCode for why
		// the machine error CODE, not a new status, carries the detail now.
		os.Exit(exitStatusFor(err))
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use: "dossierx",
		// The summary no longer says "Render claims into a static HTML viewer".
		// Two things were wrong with it as of v0.3.0: "render" is not a verb any
		// more (it is a stage of check), and rendering was never the point — the
		// viewer is where the human reviews, and the review is what the engine
		// is actually for. This line is the first thing anyone typing
		// "dossierx --help" reads, so it states the two-role premise the rest of
		// the surface is shaped around.
		Short:        "Turn YAML claims into a reviewable viewer, and gate every change to a reviewed one",
		SilenceUsage: true,
		// Cobra's own "Error: <msg>" print cannot serve both output formats,
		// and it happens for flag/argument errors that no RunE wrapper can
		// intercept. runCLI takes the job over: it reproduces this exact line
		// under --format text and emits a failure envelope under --format json.
		SilenceErrors: true,
	}
	// The ROOT gets the same treatment as the four nouns (see
	// requireSubcommand): without a RunE, cobra's default for a parent is to
	// print help prose on STDOUT and return nil, so a bare "dossierx" — or
	// "dossierx $NOUN" where the variable is empty, which is how an agent
	// building argv programmatically actually reaches this — exited 0 with a
	// banner where the envelope should be. Both halves of the machine contract
	// broken at once, and the one place in the surface where they were: every
	// noun and every leaf already fails loudly here.
	//
	// requireSubcommand cannot be reused as-is: it labels the error with
	// commandPath, which is EMPTY for the root (it is the binary name, stripped),
	// so it would compose "': a subcommand is required; ' is a command group".
	// The message is inlined instead, naming the six nouns the way the noun
	// errors name their leaves.
	//
	// "dossierx --help" is unaffected: cobra handles it before RunE, and a
	// caller that ASKED for prose gets prose. "--version" is NOT in that
	// category any more — see versionFlag below.
	root.RunE = envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
		if versionFlag {
			return versionResult(cmd), nil
		}
		if len(args) > 0 {
			return cmdResult{}, cliout.Errorf(cliout.CodeUsage,
				"dossierx: unknown command %q", args[0]).
				WithHint("run one of: dossierx <build-order, check, claim, comment, serve, skills, version>")
		}
		return cmdResult{}, cliout.Errorf(cliout.CodeUsage,
			"dossierx: a subcommand is required; dossierx does nothing on its own").
			WithHint("run one of: dossierx <build-order, check, claim, comment, serve, skills, version>")
	})
	// --version, taken back off cobra.
	//
	// Setting root.Version is what makes cobra wire up its OWN built-in
	// --version flag, and cobra's implementation prints a prose line to stdout
	// and returns before any RunE runs. That is the last hole in the machine
	// contract of exactly the shape the bare-noun holes had: an invocation that
	// exits 0 with something on stdout that is not an envelope. An agent that
	// asked a JSON-by-default binary for the version got a sentence.
	//
	// So the flag is registered here instead, and answered by the root's RunE
	// through the SAME versionResult the "version" leaf returns — one payload,
	// one text rendering, two doors. It stays exit 0 (DX-AUD-19 pinned that
	// version reporting works with no project config on disk, and refusing the
	// flag outright would regress it) and it is deliberately NOT hidden: a flag
	// that answers correctly should be discoverable in --help.
	//
	// It is a LOCAL flag, not a persistent one. Persistent would make
	// "dossierx claim --version" parse and then be silently ignored by claim's
	// own RunE, which is the failure mode this is fixing, one level down.
	root.Flags().BoolVar(&versionFlag, "version", false, "print the version, commit and build date (same payload as \"dossierx version\")")
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to project.config.yaml (default: search upward from the current directory, like git finds .git)")
	root.PersistentFlags().StringVar(&formatFlag, "format", formatJSON, "output format: json (the machine contract — one envelope per run) or text (human prose)")

	// Validating --format here rather than at each use site means an
	// unrecognized value fails loudly at the front door instead of silently
	// falling through to a default the caller did not ask for.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		switch formatFlag {
		case formatJSON, formatText:
			return nil
		default:
			return cliout.Errorf(cliout.CodeUnsupportedFormat,
				"--format must be %q or %q, got %q", formatJSON, formatText, formatFlag)
		}
	}

	// The whole v0.3.0 surface: six nouns, nineteen leaves, and not one more.
	//
	//	check                                                            1
	//	claim   show list new lock unlock flag reaudit link               8
	//	comment inbox list add reply                                      4
	//	build-order propose status lock                                   3
	//	serve · skills export · version                                   3
	//
	// The count is a design constraint, not a coincidence. Every verb here is
	// something an AGENT does, because the agent is this CLI's operator; the
	// human's entire surface is "dossierx serve" plus the viewer it opens. The
	// ten verbs v0.3.0 removed (lint, catalog, render, deps, stale, coverage,
	// implink set/status, comment edit/delete/resolve/reopen) were either
	// pipeline stages of check, filters wearing a verb's clothes, or — for the
	// four comment verbs — surfaces that belong where the rights holder is.
	// TestSurfaceIsNineteenLeavesUnderSixNouns in main_test.go pins it, so
	// adding a leaf is a decision someone has to make on purpose.
	root.AddCommand(
		newCheckCmd(),
		newClaimCmd(),
		newCommentCmd(),
		newBuildOrderCmd(),
		newSkillsCmd(),
		newVersionCmd(),

		// serve is the one permanently text-only command: it is a long-running
		// process whose useful output (the URL to open) has to appear BEFORE it
		// blocks, which one-envelope-per-invocation cannot express — and its
		// consumer is the human anyway. See annotationTextOnly.
		markTextOnly(newServeCmd()),
	)

	// The retired top-level verbs, as hidden stubs. They add nothing to the
	// surface (they are marked, hidden, and excluded from the leaf count) and
	// exist because cobra rejects an unknown ROOT command during Execute, before
	// the RunE above can run: `dossierx lint` reported `unknown command "lint"
	// for "dossierx"` with an empty command field and no hint, for a name the
	// router documents as one agents will remember. See retired.go.
	root.AddCommand(retiredTopLevelCmds()...)

	// Cobra's own "completion" group, materialized HERE rather than left to
	// Execute, so the decision about it can be written down instead of inherited.
	//
	// The decision, in two parts. Its LEAVES (completion bash|zsh|fish|
	// powershell) stay prose: their entire product is a shell script written to
	// stdout, there is no envelope shape that could carry a bash completion
	// function, and their consumer is a shell's rc file. They are deliberately
	// NOT marked text-only — TestServeIsTheOnlyTextOnlyCommand pins serve as the
	// one permanent exemption, and widening that annotation is exactly the drift
	// it exists to prevent.
	//
	// The GROUP itself gets the same requireSubcommand treatment as the six
	// nouns, because bare "dossierx completion" is the identical hole: cobra
	// prints help prose on stdout and exits 0, so an agent that assembled the
	// wrong argv is told it succeeded. TestSurfaceIsNineteenLeavesUnderSixNouns
	// already skips "completion" as framework furniture, so materializing it
	// early does not change the pinned surface.
	root.InitDefaultCompletionCmd()
	for _, sub := range root.Commands() {
		if sub.Name() == "completion" {
			commandGroup(sub)
		}
	}
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

// versionData is "dossierx version"'s machine payload. Name is included even
// though it is a constant: a version envelope that does not say WHAT is at that
// version is useless to an agent inspecting a toolchain it did not install.
type versionData struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// versionResult is the ONE answer both doors to the version give: the "version"
// leaf and the root's --version flag. Sharing it is the whole point — two
// spellings of one question must not be able to answer differently, which is
// precisely what happened while --version was cobra's built-in and printed prose
// that no envelope reader could parse.
//
// Command is pinned to "version" rather than left to commandPath, because the
// flag is answered by the ROOT's RunE and commandPath(root) is the empty string.
// A caller correlating a response with the call it made needs the name of the
// thing it asked for, not a blank.
func versionResult(cmd *cobra.Command) cmdResult {
	v, c, d := resolveVersionInfo()
	return cmdResult{
		Command: "version",
		Data:    versionData{Name: "dossierx", Version: v, Commit: c, Date: d},
		Text: func() {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "dossierx version %s\n", v)
			fmt.Fprintf(out, "  commit: %s\n", c)
			fmt.Fprintf(out, "  date:   %s\n", d)
		},
	}
}

// newVersionCmd prints the binary's version, commit, and build date. Unlike
// every other subcommand it never loads a project config — it describes the
// binary itself, so it works from anywhere, with or without a project.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the dossierx version, commit, and build date",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			return versionResult(cmd), nil
		}),
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

	// The message is unchanged from v0.1.2 (byte-for-byte, including the
	// wrapped sentinel that makes this exit 2); the code attached here is what
	// lets a skill recognize "this project isn't set up yet" without matching
	// on prose.
	return "", cliout.Errorf(cliout.CodeConfigNotFound,
		"no project.config.yaml found (searched upward from %s): %w; pass --config explicitly", cwd, config.ErrNotFound)
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
		// An EXPLICIT --config pointing at a file that isn't there is still the
		// not-found family, not a parse failure: config.LoadConfig wraps
		// config.ErrNotFound for it, and it is documented (and pinned by
		// tests/config_test.go) as exit 2. Classifying before attaching the
		// code matters because an attached code WINS over sentinel inference
		// in errorForCLI — the whole point of attaching one — so a blanket
		// invalid_config here would silently demote it to exit 1.
		code := cliout.CodeInvalidConfig
		if errors.Is(err, config.ErrNotFound) {
			code = cliout.CodeConfigNotFound
		}
		return nil, cliout.Errorf(code, "load config: %w", err)
	}
	return cfg, nil
}

// loadClaims loads every claim under cfg's claims_dir. The "load claims:"
// prefix is load-bearing and pinned (check_parity_test.go asserts a claims-load
// failure is reported unprefixed by "check:", since it precedes the pipeline);
// cliout.Errorf reproduces fmt.Errorf's string exactly, so attaching the code
// changes no byte of the message.
func loadClaims(cfg *config.Config) ([]model.Claim, error) {
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		return nil, cliout.Errorf(cliout.CodeInvalidClaim, "load claims: %w", err)
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

// digestStorePresent reports whether the comment digest store is on disk for
// cfg — the one piece of evidence about the lock store that does not live
// INSIDE the lock store (see lock.Store.LedgerDowngraded).
//
// It mirrors internal/check's identically-named helper, deliberately duplicated
// rather than shared for the same reason catalogPath and storePath are: cmd/ and
// check/ must not import each other, and TestPathHelpersResolveAgainstConfigDir
// fails loudly if the copies disagree about where these files live.
//
// A nil cfg is read as ABSENT rather than dereferenced. That is the same
// conservative direction internal/lock takes for an unreadable digest store: it
// can only widen the pre-ledger exemption, never manufacture a downgrade
// accusation out of a caller that had no project to look in.
func digestStorePresent(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	_, err := os.Stat(digest.StorePath(cfg))
	return err == nil
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
// cross-process file lock "dossierx claim lock"/"dossierx claim reaudit" take, so the
// migration Save can never race their load-mutate-save on the shared store
// file. The returned store is safe to read (e.g. lock.DetectStale) after the
// lock is released: DetectStale only consults the already-loaded in-memory
// store, never the file again.
// errStoreUnreadable marks a lock-store LOAD failure — the file exists and
// could not be decoded — as opposed to a sentinel-acquisition or Save failure,
// which are contention and write errors respectively.
//
// The distinction is not cosmetic. A caller that degrades gracefully around a
// corrupt store (reconcileReviewPending, so the ledger gate gets to report
// lock-ledger-unreadable with the right recovery) must NOT degrade around a
// write conflict, which is a real, retryable failure of a run that intended to
// write. Matching on the error's prose would silently reclassify one as the
// other the first time internal/lock rewords a message.
var errStoreUnreadable = errors.New("lock store could not be read")

// It also returns the ids grandfathered by the one-time ledger adoption, so the
// command that called it can put them in its envelope — see prepareStore.
func loadStoreForRead(cfg *config.Config, claims []model.Claim) (*lock.Store, []string, error) {
	release, err := lock.AcquireFileLock(storePath(cfg))
	if err != nil {
		return nil, nil, err
	}
	defer release()

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errStoreUnreadable, err)
	}
	changed, adopted := prepareStore(cfg, store, claims)
	if changed {
		if err := store.Save(); err != nil {
			return nil, adopted, err
		}
	}
	return store, adopted, nil
}

// prepareStore runs every on-load store migration for a store the caller has
// already locked, and reports whether anything changed (so the caller Saves).
//
// It wraps lock.PrepareStore — which runs BOTH of internal/lock's own
// migrations, the legacy per-dependent baseline re-arm and the one-time
// lock-ledger grandfathering, in the one order that is correct — and adds the
// third: grandfathering already-locked BUILD ORDERS.
//
// That third one lives here rather than in internal/lock because it cannot live
// there: internal/buildorder imports internal/lock (for ContentHash), so lock
// cannot import buildorder back to read an artifact. It exists at all because
// build orders could be locked before this release gave them a ledger record,
// and without adoption every such project would fail `check` on upgrade with a
// build-order-ledger-missing it had no way to have avoided — a gate firing on
// correct state, which is how gates get switched off.
//
// The pre-ledger test is taken BEFORE lock.PrepareStore, which stamps the
// current version as its last act. Its two halves are the security property
// (see Store.PreLedger): a store already at the ledger version never adopts
// again, and an ABSENT store never adopts at all.
//
// And it is PreLedgerExempt, not PreLedger, for the reason
// Store.LedgerDowngraded spells out at length — this call site used to be the
// last place the downgrade attack still worked. The claim half of adoption is
// guarded inside lock.AdoptLedger (which refuses, announces "Nothing was
// grandfathered" on stderr, and leaves the gate to report every locked claim);
// the BUILD-ORDER half is this file's, and it was guarded by nothing. The
// consequence was a complete, one-command bypass of the release's headline
// invariant: reorder .build-order.<module>.json by hand, set the store's
// "version" back to 1 and delete the one build-order:<module> ledger key, and
// the very next ordinary `dossierx check` re-signed the HAND-REORDERED bytes as
// a grandfathered approval and re-stamped the version — in the same run whose
// stderr said the downgrade had been refused. `check --validate` was clean
// forever after, and the evidence that anything happened was gone. Sharing the
// exemption predicate with lock.Audit (internal/lock/audit.go) and check's
// buildOrderGate (internal/check/ledger.go) is what makes the three answer
// identically; an honest v0.2.x project has no digest store and no ledger
// records, so LedgerDowngraded is false there and adoption still fires.
//
// It returns the grandfathered claim ids alongside changed. Discarding them —
// which is what this did — left the one-time adoption announced on STDERR and
// nowhere else: `dossierx check` printed ok:true, zero findings, exit 0 on the
// very run that adopted every locked claim in the project as-found. An agent
// parsing stdout, exactly as the machine contract tells it to, reported "the
// project is clean" on the one run where a human most needs to look. Every other
// stderr note in this release is mirrored into the envelope (see
// build_order.go's ledger warning and unlock's flag-store warning) for the
// reason output.go states: "a consumer that has to check two streams to find out
// what happened does not have a machine contract". lock.AdoptLedger returns the
// ids sorted precisely so a caller can do this; there is one call site and it
// has to.
func prepareStore(cfg *config.Config, store *lock.Store, claims []model.Claim) (bool, []string) {
	// Both halves of the evidence are read BEFORE lock.PrepareStore runs:
	// PrepareStore stamps the current schema version onto the store AND (on a
	// genuine upgrade) creates the comment digest store, so asking either
	// question afterwards would be asking it of a project this run had already
	// changed.
	preLedgerExempt := store.PreLedgerExempt(digestStorePresent(cfg))

	changed, adopted := lock.PrepareStore(store, claims)
	if !preLedgerExempt {
		return changed, adopted
	}

	for _, module := range cfg.Modules {
		artifact, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
		if err != nil || artifact == nil || !artifact.Locked {
			continue
		}
		hash, err := buildOrderSignature(artifact)
		if err != nil {
			continue
		}
		if lock.AdoptBuildOrderApproval(store, module, hash) {
			changed = true
			adopted = append(adopted, lock.BuildOrderLedgerKey(module))
		}
	}
	return changed, adopted
}

// ledgerAdoptionWarnings renders the one-time grandfathering as envelope
// warnings — one sentence naming what adoption did and did not establish, then
// one line per adopted id.
//
// It is a WARNING rather than data alone because the envelope's ok field cannot
// carry it: adoption happens on a run that otherwise succeeds, and "ok:true with
// zero findings" is exactly the answer that must not be the whole answer on the
// run that blessed every locked artifact in the project as-found.
func ledgerAdoptionWarnings(adopted []string) []string {
	if len(adopted) == 0 {
		return nil
	}
	warnings := []string{fmt.Sprintf(
		"lock ledger created: %d already-locked artifact(s) were adopted as GRANDFATHERED. Their recorded content is what was on disk just now, NOT content anyone approved — any edit made before this upgrade is adopted with it. Review them, and re-lock any you are not sure of (dossierx claim unlock <id> --reason \"...\" then dossierx claim lock <id> --reason \"...\").",
		len(adopted))}
	for _, id := range adopted {
		warnings = append(warnings, "grandfathered: "+id)
	}
	return warnings
}

// standingLedgerRecord answers the one question every re-signing path has to
// ask before it writes: does this claim's content still match the approval that
// currently vouches for it?
//
// "Standing" means an approval that is in force right now — a record that
// exists, describes a CLAIM (not a build order), and has not been released by an
// unlock. A released record describes a claim that is allowed to be draft and
// allowed to change; comparing content against it would refuse the ordinary
// draft edit the release exists to keep free. No record at all is not a match
// failure either: that is lock-ledger-missing, a finding the gate already owns,
// and treating it here as tampering would turn a project whose ledger was
// deleted into a project no command can operate — which is the shape of gate
// people switch off rather than fix.
//
// matches is therefore "there is nothing standing that this content
// contradicts", and it is true in both of the no-evidence cases on purpose. Only
// a standing approval whose hash disagrees with the bytes on disk returns false.
func standingLedgerRecord(store *lock.Store, claim model.Claim) (rec lock.LedgerRecord, standing, matches bool) {
	if store == nil {
		return lock.LedgerRecord{}, false, true
	}
	rec, ok := store.Record(claim.ID)
	if !ok || rec.Subject != lock.SubjectClaim || rec.Released() {
		return rec, false, true
	}
	return rec, true, rec.Hash == lock.LockedClaimHash(claim)
}

// catalogPath is where "dossierx check" writes the built catalog. It mirrors
// internal/check's identically-named helper; the two are deliberately
// duplicated rather than shared (see that package's doc comment) and
// TestPathHelpersResolveAgainstConfigDir fails loudly if they disagree.
func catalogPath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".catalog.json")
}

// renderOutPath is where "dossierx check" writes the generated viewer — the
// file "dossierx serve" then serves and re-renders. Same duplication contract
// as catalogPath.
func renderOutPath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), "viewer", "index.html")
}

// ---------------------------------------------------------------------
// lint reporting (the shared formatter; the standalone "lint" verb is gone)
// ---------------------------------------------------------------------

// reportLintFindings prints findings as the terminal's lint block and returns
// a non-nil error (for a nonzero exit code) if any error-severity finding is
// present. Warnings are reported but do not fail the command.
//
// The pre-v0.3.0 "--json" branch is gone with the "lint" verb that owned it.
// The machine surface for findings is now the envelope: "check --format json"
// (and "check --validate --format json") carry them in data.lint_findings, in
// the same snake_case as every other payload — see lintFindingData for why the
// old bare-array shape could not simply be reused.
func reportLintFindings(cmd *cobra.Command, findings []lint.Finding) error {
	errCount := 0
	for _, f := range findings {
		if f.Severity != lint.SeverityWarning {
			errCount++
		}
	}

	out := cmd.OutOrStdout()
	if len(findings) == 0 {
		fmt.Fprintln(out, "lint: 0 findings")
	} else {
		for _, f := range findings {
			fmt.Fprintf(out, "[%s] %s: %s: %s\n", f.Severity, f.LintName, f.ClaimID, f.Message)
		}
		fmt.Fprintf(out, "lint: %d finding(s), %d error(s)\n", len(findings), errCount)
	}

	if errCount > 0 {
		return fmt.Errorf("lint: %d error-level finding(s)", errCount)
	}
	return nil
}

// reportLedgerFindings prints the lock-ledger gate's block, or NOTHING at all
// when the gate found nothing.
//
// The silence on success is deliberate and load-bearing. reportLintFindings
// prints "lint: 0 findings" because a lint run with no findings is a result the
// reader asked for; the ledger gate is a tripwire, and a tripwire that
// announces itself on every clean run trains people to skim past the one run
// where it fires. It also keeps "dossierx check"'s output byte-identical for
// every project that passes the gate, which check_parity_test.go pins.
//
// Each finding is printed on its own line, whole. The messages are written to
// stand alone — the pre-commit hook prints this block and nothing else, so a
// truncated or summarized rendering here would be the difference between a
// developer knowing what to do and a developer reaching for --no-verify.
func reportLedgerFindings(cmd *cobra.Command, findings []lock.Finding) {
	if len(findings) == 0 {
		return
	}
	out := cmd.OutOrStdout()
	for _, f := range findings {
		if f.ClaimID == "" {
			fmt.Fprintf(out, "[ledger] %s: %s\n", f.Rule, f.Message)
			continue
		}
		fmt.Fprintf(out, "[ledger] %s: %s: %s\n", f.Rule, f.ClaimID, f.Message)
	}
	fmt.Fprintf(out, "ledger: %d integrity finding(s)\n", len(findings))
}

// ---------------------------------------------------------------------
// check (lint + catalog + render, stop at first failure)
// ---------------------------------------------------------------------

// reconcileReviewPending runs "check"'s only claim-file-writing phase under
// the project-wide claims sentinel: it loads claims, flips every locked claim
// whose mirrors/rests_on content has drifted since its last lock or confirmed
// reaudit — OR that carries an unresolved comment thread — to
// locked+review_pending, and persists each flip back to the claim's
// own file so it survives to the next run and shows up in "dossierx claim list\n// --review-pending". It
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
// It returns the grandfathered ids alongside the reconciled claims, because
// reconcile is where "dossierx check" opens the lock store for writing and
// therefore where a pre-ledger project's one-time adoption happens.
func reconcileReviewPending(cfg *config.Config) ([]model.Claim, []string, error) {
	releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
	if err != nil {
		return nil, nil, err
	}
	defer releaseClaims()

	claims, err := loadClaims(cfg)
	if err != nil {
		return nil, nil, err
	}
	// The LOCK store, read best-effort in exactly one respect: a store file that
	// exists and cannot be DECODED degrades this phase to "no baselines" instead
	// of taking the command down.
	//
	// It has to. A corrupt lock store is precisely what internal/check's
	// RuleLedgerUnreadable exists to report — with the one recovery that is
	// correct ("restore the ledger from version control rather than re-locking,
	// since re-locking would record whatever the claims say NOW as approved") —
	// and reconcile runs BEFORE the pipeline that raises it. Returning the decode
	// error from here made `dossierx check`, the command a human and an agent
	// both reach for first, answer with a raw parse error under the write_failed
	// code, on a run that wrote nothing, and never reach the rule at all. The
	// documented recovery for a write error (retry, check permissions, free disk)
	// is wrong advice for a merge that left conflict markers in the ledger.
	//
	// The degradation is safe in the only direction that matters. DetectStale is
	// SKIPPED for this run (it reads baselines out of the store, and a nil store
	// has none), so review_pending is not SET where it might have been — while
	// the ledger gate below reports lock-ledger-unreadable plus every locked
	// claim as unapproved, so the run still fails, loudly, with the right code
	// and the right recovery. A best-effort load that silently succeeded would be
	// the unsafe shape; this one fails closed.
	//
	// Contention and write failures are NOT degraded: errStoreUnreadable is the
	// decode case only, so a genuine write_conflict still surfaces as itself.
	store, adopted, err := loadStoreForRead(cfg, claims)
	if err != nil {
		if !errors.Is(err, errStoreUnreadable) {
			return nil, adopted, err
		}
		store = nil
	}

	// The FLAG store, read best-effort. A load failure degrades this to "no
	// flags" rather than failing check outright, matching how internal/check's
	// nextSteps treats the same file: a reconciliation that only ever SETS a
	// flag can be incomplete without being wrong, and taking check down over an
	// advisory store would be the worse trade.
	flagStore, flagErr := reaudit.LoadFlagStore(flagStorePath(cfg))
	if flagErr != nil {
		flagStore = nil
	}

	// DetectStale dereferences the store for every dependency baseline, so the
	// degraded (nil-store) path skips it entirely and starts from the claims as
	// loaded. comments.Recompute below is nil-safe by contract and simply reports
	// no drift trigger, which is the honest answer when the baselines are
	// unreadable.
	updated := make([]model.Claim, len(claims))
	copy(updated, claims)
	if store != nil {
		updated = lock.DetectStale(claims, store)
	}
	for i := range updated {
		// All THREE documented review_pending triggers are reconciled here, not
		// just DetectStale's dependency drift:
		//
		//   - a LOCKED claim carrying an open comment thread. The lock gate
		//     forbids locking WITH an open thread, but a thread can be opened on
		//     an already-locked claim (or hand-authored into its YAML).
		//   - a pending "dossierx claim flag" entry. This one was MISSING, and
		//     its absence made check disagree with every other reader of the
		//     same state: comments.Recompute (the comment write path) and
		//     comments.PendingTriggers (check's own next_steps) both consult the
		//     flag store, so a claim whose review_pending had been cleared —
		//     unlock/relock, or a hand edit — kept a live flag entry that check
		//     could see in its hints and would not restore to the claim file.
		//     The flag stayed pending, the claim read as settled, and "claim
		//     list --review-pending" did not list it.
		//
		// Purely additive, like DetectStale: this only ever SETS review_pending.
		// Clearing it stays the job of reaudit --confirm, unlock, or resolving
		// the last open thread.
		if updated[i].Status == model.StatusLocked && comments.Recompute(updated[i], updated, store, flagStore) {
			updated[i].ReviewPending = true
		}
		if updated[i].ReviewPending != claims[i].ReviewPending {
			if err := loader.SaveClaim(updated[i]); err != nil {
				return nil, adopted, fmt.Errorf("persist review_pending for %q: %w", updated[i].ID, err)
			}
		}
	}
	return updated, adopted, nil
}

// checkData is "dossierx check"'s machine payload: everything the terminal
// reporter prints, in structured form. It is emitted on FAILED runs too — a run
// that wrote the catalog and the viewer and then failed the impl-link scan has
// produced real artifacts, and an agent that has to re-run the whole pipeline
// to learn that is being made to pay twice. Read it together with the
// envelope's stopped_at, which says how far the run got.

// lintFindingData is one lint finding in snake_case.
//
// It exists because lint.Finding carries no JSON tags at all, so encoding it
// directly would emit Go field names ("LintName", "ClaimID") into a contract
// whose every other key is snake_case. Widening lint.Finding with tags is a
// change to an engine type several packages share, for the sole benefit of one
// CLI payload; projecting here keeps the change local and the envelope
// internally consistent, which TestEnvelopeKeysAreSnakeCase pins.
type lintFindingData struct {
	Lint     string `json:"lint"`
	ClaimID  string `json:"claim_id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// scanErrorData is one impl-link scan error in snake_case, projected for the
// same reason lintFindingData is: implink.ScanError carries no JSON tags.
type scanErrorData struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	ClaimID string `json:"claim_id"`
	Message string `json:"message"`
}

type checkData struct {
	// ReadOnly is true only for "check --validate". It is in the payload
	// rather than left implicit because the two runs return the SAME shape
	// with different meanings: a --validate result with an empty viewer_path
	// means "not written, by design", where a plain check's empty viewer_path
	// means "the run stopped before it got there".
	ReadOnly bool `json:"read_only,omitempty"`

	// Staged is true for "check --staged": the run evaluated the git index
	// rather than the working tree. Skipped rides with it and is true when
	// there was no index to evaluate (no git, or not a work tree) — the one
	// combination that means "this run checked NOTHING and still exited 0", so
	// a consumer can tell it apart from a clean pass. staged_files lists what
	// was read out of the index.
	Staged      bool     `json:"staged,omitempty"`
	Skipped     bool     `json:"skipped,omitempty"`
	StagedFiles []string `json:"staged_files,omitempty"`

	LintFindings     []lintFindingData `json:"lint_findings"`
	LintErrorCount   int               `json:"lint_error_count"`
	LintWarningCount int               `json:"lint_warning_count"`

	// LedgerFindings is the lock-ledger gate's verdict. lock.Finding already
	// carries snake_case JSON tags (it is written to the same contract this
	// envelope is), so unlike lint.Finding it needs no projection — and it
	// deliberately keeps its own `rule` field rather than being folded into
	// lint_findings, because these are not lints and the skills branch on the
	// distinction. Omitted entirely when the gate found nothing.
	LedgerFindings     []lock.Finding `json:"ledger_findings,omitempty"`
	LedgerFindingCount int            `json:"ledger_finding_count"`

	// LedgerAdopted names the artifacts GRANDFATHERED by this run's one-time
	// lock-ledger adoption (claim ids, and "build-order:<module>" keys). It is
	// present only on the single run that performs the adoption, which is the
	// point: that run reports ok:true with zero findings, and without this the
	// only trace of it in the machine surface would be nothing at all. See
	// prepareStore and ledgerAdoptionWarnings.
	LedgerAdopted []string `json:"ledger_adopted,omitempty"`

	CatalogPath      string          `json:"catalog_path,omitempty"`
	CatalogCount     int             `json:"catalog_count,omitempty"`
	ViewerPath       string          `json:"viewer_path,omitempty"`
	ScanFilesScanned int             `json:"scan_files_scanned"`
	ScanErrors       []scanErrorData `json:"scan_errors"`
	OpenComments     map[string]int  `json:"open_comments,omitempty"`
	OrientationNotes []string        `json:"orientation_notes,omitempty"`
	NextSteps        []string        `json:"next_steps,omitempty"`
}

// newCheckData projects a check.Result into the machine payload. Nil slices are
// coerced to empty ones for the same reason reportLintFindings does it: a
// consumer should be able to range over lint_findings and scan_errors without
// first testing them for null.
func newCheckData(res check.Result) checkData {
	findings := make([]lintFindingData, 0, len(res.LintFindings))
	for _, f := range res.LintFindings {
		findings = append(findings, lintFindingData{
			Lint:     f.LintName,
			ClaimID:  f.ClaimID,
			Severity: string(f.Severity),
			Message:  f.Message,
		})
	}
	scanErrors := make([]scanErrorData, 0, len(res.ScanErrors))
	for _, e := range res.ScanErrors {
		scanErrors = append(scanErrors, scanErrorData{File: e.File, Line: e.Line, ClaimID: e.ClaimID, Message: e.Message})
	}
	return checkData{
		LintFindings:       findings,
		LintErrorCount:     len(res.LintErrors),
		LintWarningCount:   len(res.LintWarnings),
		LedgerFindings:     res.LedgerFindings,
		LedgerFindingCount: len(res.LedgerFindings),
		CatalogPath:        res.CatalogPath,
		CatalogCount:       res.CatalogCount,
		ViewerPath:         res.RenderPath,
		ScanFilesScanned:   res.ScanFilesScanned,
		ScanErrors:         scanErrors,
		OpenComments:       res.OpenComments,
		OrientationNotes:   res.OrientationNotes,
		NextSteps:          res.NextSteps,
	}
}

// checkStoppedAt names the step a failed check stopped at, which is the whole
// reason the envelope carries a stopped_at field at all. "ok: false" cannot
// distinguish "the viewer on disk is the previous, valid one" (stopped at scan)
// from "nothing was written and the viewer is whatever it was" (stopped at
// lint), and those call for different next moves.
//
// The value set is: config, load, reconcile, lint, catalog, render, scan,
// ledger. It is derived from which Result fields the run managed to fill rather
// than tracked separately, because check.Run's contract already IS "a field
// left zero is a step the run never reached" — deriving keeps the two from
// drifting apart.
//
// "ledger" is last for a reason worth stating in the payload: reaching it means
// the catalog and the viewer WERE regenerated and the impl-link scan passed.
// The commit is refused; the documentation is current. That is the difference
// between a gate and an outage, and stopped_at is where a caller reads it.
func checkStoppedAt(res check.Result, err error) string {
	switch {
	case err == nil:
		return ""
	case len(res.LintErrors) > 0:
		return "lint"
	case res.CatalogPath == "":
		return "catalog"
	case res.RenderPath == "":
		return "render"
	case len(res.LedgerFindings) > 0:
		return "ledger"
	default:
		return "scan"
	}
}

// checkFailureCode classifies a check step failure. Every code here resolves to
// exit status 1 (cliout.ExitCode), which is not incidental: tests/check_exit_test.go
// pins "a lint error exits 1, NOT 2 — check failures must never be mistaken for
// a missing claim or config", and that stays true for every step.
func checkFailureCode(stoppedAt string) cliout.Code {
	switch stoppedAt {
	case "lint":
		return cliout.CodeLintFailed
	case "ledger":
		return cliout.CodeIntegrityFailed
	case "scan":
		return cliout.CodeImplinkRefused
	default:
		return cliout.CodeWriteFailed
	}
}

// lintWarningLines renders warning-severity findings as the envelope's
// warnings[]: the same one-line form the terminal prints, so the two surfaces
// say the same thing and a successful run's warnings are impossible to miss.
func lintWarningLines(warnings []lint.Finding) []string {
	if len(warnings) == 0 {
		return nil
	}
	lines := make([]string, 0, len(warnings))
	for _, f := range warnings {
		lines = append(lines, fmt.Sprintf("[%s] %s: %s: %s", f.Severity, f.LintName, f.ClaimID, f.Message))
	}
	return lines
}

func newCheckCmd() *cobra.Command {
	var validate bool
	var staged bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run lint, catalog, render and the lock-ledger gate in one shot, stopping at first failure; --validate for a read-only run, --staged to judge the git index",
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			// Both read-only modes exist, and they answer DIFFERENT questions:
			// --validate judges the working tree, --staged judges what the
			// commit will contain. Silently letting one win would make a hook
			// that passed both flags validate something other than what it
			// reported, so the combination is refused rather than resolved.
			if validate && staged {
				return cmdResult{}, cliout.Errorf(cliout.CodeUsage,
					"check: --validate and --staged are different questions (the working tree vs. the git index); pass one")
			}
			if staged {
				return runCheckStaged(cmd)
			}
			if validate {
				return runCheckValidate(cmd)
			}

			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{StoppedAt: "config"}, err
			}

			// Parse-check claims up front, OUTSIDE the "check:"-wrapped reconcile
			// below, so a malformed claim YAML is reported as "load claims: ..."
			// (v0.1.2's unprefixed shape) rather than "check: load claims: ...":
			// loading claims is a precondition that predates the check pipeline,
			// so the "check:" wrap must not attach to it. reconcile re-reads
			// claims inside the project-wide claims sentinel (the Phase-0 write
			// discipline); this early load only fixes the error's provenance.
			if _, err := loadClaims(cfg); err != nil {
				return cmdResult{StoppedAt: "load"}, err
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
			claims, adopted, err := reconcileReviewPending(cfg)
			if err != nil {
				return cmdResult{StoppedAt: "reconcile"},
					cliout.Errorf(cliout.CodeWriteFailed, "check: %w", err)
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
			stoppedAt := checkStoppedAt(res, runErr)
			data := newCheckData(res)
			// The one-time ledger adoption, in the MACHINE surface. It reached
			// stderr already (lock.announceAdoption), and stderr alone is not a
			// contract — see ledgerAdoptionWarnings. Carried as both data (the
			// ids, for a consumer that wants to act on them) and warnings (the
			// sentence, so an agent that reads only ok/warnings still learns the
			// run blessed content nobody approved).
			data.LedgerAdopted = adopted
			out := cmdResult{
				Data:      data,
				Warnings:  append(ledgerAdoptionWarnings(adopted), lintWarningLines(res.LintWarnings)...),
				StoppedAt: stoppedAt,
				Text:      func() { formatCheckResult(cmd, res) },
			}
			if runErr != nil {
				// The "check: %w" wrap is the single choke point that used to
				// live at every inlined step, and its exact bytes are pinned by
				// check_parity_test.go / tests/check_exit_test.go. cliout.Errorf
				// reproduces fmt.Errorf's string precisely, so attaching the
				// code costs nothing on the text side.
				return out, cliout.Errorf(checkFailureCode(stoppedAt), "check: %w", runErr)
			}
			return out, nil
		}),
	}
	cmd.Flags().BoolVar(&validate, "validate", false, "validate only: lint in memory and report, writing NOTHING — no claim files, no lock store, no .catalog.json, no viewer")
	cmd.Flags().BoolVar(&staged, "staged", false, "judge the GIT INDEX instead of the working tree (what the commit will contain), writing NOTHING — this is what a pre-commit hook runs")
	return cmd
}

// runCheckStaged is "dossierx check --staged": the pre-commit gate.
//
// It differs from --validate in one respect and inherits everything else: the
// claim registry, the lock ledger and the comment digest store all come out of
// the GIT INDEX rather than off disk (see internal/check/staged.go for why all
// three have to come from the same place), and it enforces the lock-ledger gate
// as well as the lint gate. It writes nothing at all — no claim files, no
// stores, no catalog, no viewer — because it runs DURING a commit, and a gate
// that dirties the tree it is judging is worse than no gate.
//
// No index, no verdict, exit 0. git absent, or a project that is not inside a
// work tree, produces a warning and success. That is not laxity: --staged is
// reached from a git hook, so this condition means somebody ran it by hand
// somewhere it cannot apply, and failing there would push hook authors into
// swallowing exit codes — which would disarm every other gate in this release.
// The envelope says so explicitly (data.skipped), so CI can insist.
func runCheckStaged(cmd *cobra.Command) (cmdResult, error) {
	cfg, err := loadConfig()
	if err != nil {
		return cmdResult{StoppedAt: "config"}, err
	}

	sp, err := check.Staged(cfg)
	if err != nil {
		if errors.Is(err, check.ErrNoIndex) {
			data := checkData{ReadOnly: true, Staged: true, Skipped: true, LintFindings: []lintFindingData{}, ScanErrors: []scanErrorData{}}
			warning := fmt.Sprintf("check --staged: %v; nothing was evaluated", err)
			return cmdResult{
				Data:     data,
				Warnings: []string{warning},
				Text: func() {
					fmt.Fprintln(cmd.ErrOrStderr(), warning)
				},
			}, nil
		}
		// A real git failure is not a verdict either way, so it must not be
		// reported as a clean run. CodeInternal rather than a check-step code:
		// nothing about the project was judged.
		return cmdResult{StoppedAt: "load"}, cliout.Errorf(cliout.CodeInternal, "%w", err)
	}

	res := check.StatusStaged(sp, cfg)
	data := newCheckData(res)
	data.ReadOnly = true
	data.Staged = true
	data.StagedFiles = sp.FromIndex
	out := cmdResult{
		Data:     data,
		Warnings: lintWarningLines(res.LintWarnings),
		Text:     func() { formatCheckStagedResult(cmd, sp, res) },
	}

	// Fail-fast in the same order the writing pipeline uses: a project that
	// does not lint is not one whose ledger findings are worth reading, because
	// half of them may be artifacts of the malformed claim.
	if len(res.LintErrors) > 0 {
		out.StoppedAt = "lint"
		return out, cliout.Errorf(cliout.CodeLintFailed, "check: lint: %d error-level finding(s)", len(res.LintErrors))
	}
	if len(res.LedgerFindings) > 0 {
		out.StoppedAt = "ledger"
		return out, cliout.Errorf(cliout.CodeIntegrityFailed, "check: ledger: %d integrity finding(s)", len(res.LedgerFindings))
	}
	return out, nil
}

// formatCheckStagedResult is --staged's terminal rendering. It names what it
// judged — the number of claims and how many of them came out of the index —
// because the single most damaging way for this command to be wrong would be to
// judge the working tree while a reader believed it had judged the commit.
func formatCheckStagedResult(cmd *cobra.Command, sp check.StagedProject, res check.Result) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "check --staged: %d claim(s) from the git index (%d differ from the working tree)\n", len(sp.Claims), len(sp.FromIndex))
	// The error return is intentionally discarded: runCheckStaged decides the
	// outcome from res and returns the coded error.
	reportLintFindings(cmd, res.LintFindings) //nolint:errcheck // intentionally discarded (see comment above)
	reportLedgerFindings(cmd, res.LedgerFindings)
	if len(res.LintErrors) > 0 || len(res.LedgerFindings) > 0 {
		return
	}
	fmt.Fprintln(out, "check --staged: OK (read-only: nothing written)")
}

// runCheckValidate is "dossierx check --validate": the read-only run.
//
// It exists because v0.3.0 deleted the standalone "lint" verb, and without a
// non-writing option the per-claim authoring loop the skills teach ("write a
// claim, validate it, fix it, validate again") would have become a WRITER.
// Plain "check" writes on every single run — reconcileReviewPending saves claim
// files, loadStoreForRead saves the migrated lock store, and check.Run writes
// .catalog.json and viewer/index.html — so "validate" could not be a flag that
// merely suppresses the last two steps. It has to drive a different seam.
//
// That seam already existed: check.Status is the memory-only sibling internal/
// serve's GET /api/status uses for exactly the same reason (a bare, CSRF-exempt
// GET must never truncate the viewer). Reusing it rather than adding a
// second read-only pipeline is what keeps "what --validate checks" and "what
// the status strip shows" the same thing by construction.
//
// What --validate therefore does NOT do, and must be honest about: it does not
// reconcile review_pending (that is a write), does not rebuild the catalog or
// the viewer, and does not run the impl-link scan (implink.Scan reconciles
// artifacts — also a write). It is the lint gate, the LOCK-LEDGER gate, and the
// non-blocking reporting, and nothing else. Run plain "check" before trusting
// the viewer.
//
// It DOES enforce the ledger gate, even though check.Status only reports it.
// Reading the same rules as plain "check" is the whole value of --validate: a
// read-only mode that quietly passed a tampered project would be the easiest
// possible bypass in the release. What it cannot do is judge a COMMIT — for
// that, --staged reads the git index instead of the working tree.
func runCheckValidate(cmd *cobra.Command) (cmdResult, error) {
	cfg, err := loadConfig()
	if err != nil {
		return cmdResult{StoppedAt: "config"}, err
	}
	claims, err := loadClaims(cfg)
	if err != nil {
		return cmdResult{StoppedAt: "load"}, err
	}

	res := check.Status(claims, cfg)
	data := newCheckData(res)
	data.ReadOnly = true
	out := cmdResult{
		Data:     data,
		Warnings: lintWarningLines(res.LintWarnings),
		Text:     func() { formatCheckValidateResult(cmd, res) },
	}
	if len(res.LintErrors) > 0 {
		// Same wrap, same code, and therefore the same exit status 1 as a
		// writing check that stops at lint: a validation failure is a
		// validation failure whichever door it came through, and
		// tests/check_exit_test.go's "a lint error is 1, never 2" holds for
		// both.
		out.StoppedAt = "lint"
		return out, cliout.Errorf(cliout.CodeLintFailed, "check: lint: %d error-level finding(s)", len(res.LintErrors))
	}
	if len(res.LedgerFindings) > 0 {
		// check.Status REPORTS the ledger gate and leaves the decision to its
		// caller (so serve's status strip keeps rendering a disputed project);
		// --validate is a caller that ENFORCES. A read-only mode that quietly
		// passed a tampered project would be the easiest possible bypass —
		// "just run the one that doesn't complain".
		out.StoppedAt = "ledger"
		return out, cliout.Errorf(cliout.CodeIntegrityFailed, "check: ledger: %d integrity finding(s)", len(res.LedgerFindings))
	}
	return out, nil
}

// formatCheckValidateResult is --validate's terminal rendering.
//
// It is deliberately NOT formatCheckResult: that function prints "check: OK"
// after the catalog/viewer write confirmations, and printing the same line for
// a run that wrote nothing would tell a reader the viewer on disk had just been
// regenerated when it had not. The wording here says what actually happened.
func formatCheckValidateResult(cmd *cobra.Command, res check.Result) {
	out := cmd.OutOrStdout()

	// The error return is intentionally discarded: runCheckValidate already
	// decided the outcome from res.LintErrors and returns the coded error.
	reportLintFindings(cmd, res.LintFindings) //nolint:errcheck // intentionally discarded (see comment above)
	if !res.OK {
		return
	}
	// res.OK is lint-driven in check.Status (see its doc comment), so the
	// ledger block is printed here, after the lint verdict, and the OK line
	// below is suppressed when the gate found anything.
	reportLedgerFindings(cmd, res.LedgerFindings)
	if len(res.LedgerFindings) > 0 {
		return
	}

	fmt.Fprintln(out, "check --validate: OK (read-only: nothing written)")
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
	if len(res.NextSteps) > 0 {
		fmt.Fprintln(out, "next steps:")
		for _, h := range res.NextSteps {
			fmt.Fprintf(out, "  %s\n", h)
		}
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
	reportLintFindings(cmd, res.LintFindings) //nolint:errcheck // intentionally discarded (see comment above); check.Run already surfaced the fail-fast error

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

	// The lock-ledger gate is check.Run's last step, so its block prints last —
	// after the catalog and viewer write confirmations, which is exactly the
	// story a reader needs: the documentation WAS regenerated, and the commit
	// is still refused. It prints nothing when the gate found nothing, which is
	// why every passing project's output is unchanged.
	reportLedgerFindings(cmd, res.LedgerFindings)

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
// small shared helpers
// ---------------------------------------------------------------------

// containsStr reports whether ss contains s. It survived the deletion of the
// "deps" verb (absorbed by "claim show") because requireKnownModule and
// claim show's own incoming-edge scan both still need it, and a three-line
// linear scan is not worth a dependency.
func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// lock / unlock
// ---------------------------------------------------------------------

// lockGate is internal/lock.Lock's three refusal paths, evaluated WITHOUT
// writing anything.
//
// It is duplicated logic, deliberately and with a cost: --dry-run has to answer
// "would this lock be refused, and by which gate?" before any file is touched,
// and lock.Lock answers that question only by refusing, in prose, after it has
// already taken the claims sentinel. Reimplementing the three gates as a pure
// read is what lets the preview exist AND lets the refusal be classified into a
// machine code (lint_failed / dependency_not_locked / unresolved_comments)
// without regexing lock.Lock's message.
//
// The evaluation order below mirrors lock.Lock's exactly — lint, then hub
// gating, then open threads — so the gate this reports as the blocker is the
// gate the real run would actually refuse on. Phase 3 should promote these to
// sentinels in internal/lock and delete this; until then TestLockGateCodes
// pins that the two agree.
type lockGate struct {
	LintErrors int

	// LintFindings is the error-severity half of LintErrors, kept rather than
	// counted away.
	//
	// The count on its own was an unbreakable loop, and it was reproducible in
	// three commands. `claim lock` refused with code lint_failed and
	// details {"lint_errors": 1} — a number and no rule name. The router's
	// documented recovery for lint_failed is "read data.lint_findings", which
	// this envelope did not have. `claim show`'s next_action pointed at
	// `dossierx check --validate`, which reports ZERO findings for the whole
	// class of lints that key off a claim's own status (build-role-required-
	// for-locked, rest-on-locked, roll-up): the claim is still DRAFT on disk, so
	// the rule that will refuse the lock does not fire against the project as it
	// stands. And `check`'s next_steps offered three candidate causes, none of
	// them the real one. The word the agent needed — build_role — was reachable
	// from no command in the surface.
	//
	// The findings were already computed here (RunAll runs against the
	// ABOUT-TO-BE-LOCKED form, which is exactly why they are the only correct
	// answer) and thrown away one line later. Keeping them is what lets the
	// refusal and the preview both name the rule; see lockLintFindingData.
	LintFindings []lint.Finding

	UnlockedDoctrineDep string
	OpenThreads         []string
}

// lockLintFindingData projects the gate's error-severity findings into the same
// snake_case shape `check` publishes as data.lint_findings, so an agent that
// learned one shape can read the other. It is the payload of the lock refusal's
// error.details.lint_findings — the key the router's lint_failed row has always
// told agents to read.
func (g lockGate) lockLintFindingData() []lintFindingData {
	out := make([]lintFindingData, 0, len(g.LintFindings))
	for _, f := range g.LintFindings {
		out = append(out, lintFindingData{
			Lint:     f.LintName,
			ClaimID:  f.ClaimID,
			Severity: string(f.Severity),
			Message:  f.Message,
		})
	}
	return out
}

// lintBlockerDetail renders the lint gate as one line a human or an agent can
// act on: the count, then the rules that produced it, named.
//
// It is the dry run's lint_clean detail and `claim show`'s next_action text. The
// old detail was "%d error-level lint finding(s)" and nothing else, which named
// a quantity of a thing the caller could not see.
func (g lockGate) lintBlockerDetail() string {
	if g.LintErrors == 0 {
		return "0 error-level lint finding(s)"
	}
	names := make([]string, 0, len(g.LintFindings))
	seen := map[string]bool{}
	for _, f := range g.LintFindings {
		if seen[f.LintName] {
			continue
		}
		seen[f.LintName] = true
		names = append(names, f.LintName)
	}
	return fmt.Sprintf("%d error-level lint finding(s) (%s)", g.LintErrors, strings.Join(names, ", "))
}

// blocked reports whether any gate would refuse the lock.
func (g lockGate) blocked() bool {
	return g.LintErrors > 0 || g.UnlockedDoctrineDep != "" || len(g.OpenThreads) > 0
}

// code is the machine code for the FIRST gate that would refuse, in lock.Lock's
// own order.
func (g lockGate) code() cliout.Code {
	switch {
	case g.LintErrors > 0:
		return cliout.CodeLintFailed
	case g.UnlockedDoctrineDep != "":
		return cliout.CodeDependencyNotLocked
	case len(g.OpenThreads) > 0:
		return cliout.CodeUnresolvedComments
	default:
		return cliout.CodeInternal
	}
}

// evaluateLockGates computes lockGate for claim as if it were already locked.
//
// Linting the ABOUT-TO-BE-LOCKED form rather than the current draft form is not
// an optimization; it is the same correctness requirement lock.Lock documents.
// Two lints key off a claim's own status (rest-on-locked, roll-up), and running
// them against the still-draft entry would let a claim whose dependency is
// draft sail through the very gate that exists to stop it.
func evaluateLockGates(claim model.Claim, claims []model.Claim, cfg *config.Config) lockGate {
	candidate := claim
	candidate.Status = model.StatusLocked
	candidate.ReviewPending = false

	lintClaims := make([]model.Claim, len(claims))
	copy(lintClaims, claims)
	for i := range lintClaims {
		if lintClaims[i].ID == claim.ID {
			lintClaims[i] = candidate
		}
	}

	var g lockGate
	for _, f := range lint.RunAll(lintClaims, cfg) {
		if f.Severity != lint.SeverityWarning {
			g.LintErrors++
			g.LintFindings = append(g.LintFindings, f)
		}
	}
	if cfg != nil && cfg.HubGatingEnabled() {
		deps := append(append([]string(nil), claim.Mirrors...), claim.RestsOn...)
		for _, dep := range deps {
			depClaim, ok := loader.FindByID(claims, dep)
			if !ok {
				continue
			}
			if depClaim.Facet == cfg.DoctrineFacet && depClaim.Status != model.StatusLocked {
				g.UnlockedDoctrineDep = dep
				break
			}
		}
	}
	g.OpenThreads = claim.OpenThreadIDs()
	return g
}

// lockData is "dossierx claim lock"'s machine payload: the transition, and the human
// words that authorized it. Reason is echoed back rather than merely accepted
// so the approval is visible in the same record the agent shows its human.
type lockData struct {
	ClaimID  string `json:"claim_id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Reason   string `json:"reason"`
	LockedAt string `json:"locked_at,omitempty"`
}

// lockDryRun builds the preview for "lock <id> --dry-run": the whole answer to
// "if I ran this, what happens?", read-only.
//
// A missing --reason is reported as a MISSING INPUT rather than an error,
// which is the useful ordering for the loop this release is built around: the
// agent previews, shows the human what would change, and only then has the
// approving words to put in --reason. Failing the preview for want of the
// approval it exists to solicit would be backwards.
func lockDryRun(claim model.Claim, claims []model.Claim, cfg *config.Config, reason string) *cliout.DryRun {
	dr := cliout.NewDryRun("lock claim "+claim.ID).
		Transition(string(claim.Status), string(model.StatusLocked))

	if strings.TrimSpace(reason) == "" {
		dr.Lacking("--reason")
	}
	dr.Require("claim_is_draft", claim.Status != model.StatusLocked,
		fmt.Sprintf("status is %q", claim.Status))

	g := evaluateLockGates(claim, claims, cfg)
	// The detail NAMES the rules. A preview whose blocked precondition reads
	// "1 error-level lint finding(s)" tells the caller only that something is
	// wrong, and the rules that block a lock are precisely the ones a read-only
	// `check --validate` cannot report (they key off the locked form of a claim
	// that is still draft), so there was nowhere else to look.
	dr.Require("lint_clean", g.LintErrors == 0, g.lintBlockerDetail())
	if cfg != nil && cfg.HubGatingEnabled() {
		detail := "no unlocked doctrine dependency"
		if g.UnlockedDoctrineDep != "" {
			detail = fmt.Sprintf("dependency %q is in doctrine facet %q and is not yet locked", g.UnlockedDoctrineDep, cfg.DoctrineFacet)
		}
		dr.Require("doctrine_dependencies_locked", g.UnlockedDoctrineDep == "", detail)
	}
	dr.Require("no_open_comment_threads", len(g.OpenThreads) == 0,
		fmt.Sprintf("%d unresolved thread(s) %v", len(g.OpenThreads), g.OpenThreads))

	dr.Effect("rewrites " + claim.SourcePath).
		Effect("records this claim's per-dependency content-hash baseline and lock timestamp in " + storePath(cfg)).
		Effect("the claim becomes locked: every later change to it must go through unlock -> fix -> lock")

	dr.Propose("status", string(model.StatusLocked)).
		Propose("review_pending", false).
		Propose("reason", reason)
	return dr
}

func newLockCmd() *cobra.Command {
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "lock <id>",
		Short: "Lock a draft claim (refused if lint fails); --reason records the human approval it executes",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]

			// --dry-run answers a question; it never writes and never takes a
			// sentinel, so it runs entirely off a plain read here, before the
			// write path below is entered at all.
			if dryRun {
				cfg, claims, err := loadConfigAndClaims()
				if err != nil {
					return cmdResult{}, err
				}
				claim, ok := loader.FindByID(claims, id)
				if !ok {
					return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "lock: claim %q not found: %w", id, errClaimNotFound)
				}
				dr := lockDryRun(claim, claims, cfg, reason)
				return dryRunResult(cmd, "lock", dr), nil
			}

			if err := requireReason("claim lock", reason); err != nil {
				return cmdResult{}, err
			}

			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
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
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "lock: %w", err)
			}
			defer releaseClaims()

			claims, err := loadClaims(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "lock: claim %q not found: %w", id, errClaimNotFound)
			}
			from := string(claim.Status)
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
			}

			// Serialize concurrent "dossierx claim lock"/"dossierx claim reaudit --confirm"
			// invocations that share this project's store file: each does
			// LoadStore -> mutate -> Save, and without this lock two
			// concurrent runs (e.g. locking two different claims in
			// parallel) would race on the store's Hashes/LockedAt map,
			// silently losing whichever saved first.
			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "lock: %w", err)
			}
			defer release()

			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
			}

			// Re-arm a legacy (pre-versioning) store's per-dependent baselines
			// from current content before recording this claim's own, so an
			// upgrade caught mid-lock still restores drift detection for every
			// already-locked claim (not just this one) — see
			// lock.MigrateLegacyStore — and grandfather already-locked claims
			// into the lock ledger if this store predates it (lock.AdoptLedger,
			// which announces the adoption on stderr AND, via the warnings
			// threaded out below, in this command's envelope). Persisted here
			// rather than relying on the Save below so both survive even a
			// subsequently refused lock.
			changed, adopted := prepareStore(cfg, store, claims)
			if changed {
				if err := store.Save(); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
				}
			}

			updated, err := lock.Lock(claim, claims, cfg, store, lock.Approval{Actor: lock.DefaultActor(), Reason: reason})
			if err != nil {
				// Checked before evaluateLockGates: an already-locked claim
				// trips none of the three gates (its lint is clean, its
				// dependencies are locked, it has no open thread), so the gate
				// classifier would fall through to CodeInternal and tell the
				// agent to file a bug about its own mistake.
				if errors.Is(err, lock.ErrAlreadyLocked) {
					return cmdResult{}, cliout.Errorf(cliout.CodeAlreadyLocked, "lock: %w", err).
						WithHint(fmt.Sprintf(`run: dossierx claim unlock %s --reason "<why the human agreed to reopen it>"`, id))
				}
				// Re-evaluate the gates to name WHICH one refused. lock.Lock
				// reports its refusal only in prose, and a skill that has to
				// regex "unresolved comment thread(s)" out of a sentence to
				// learn it must ask the human to click Resolve is exactly the
				// coupling this release exists to remove.
				//
				// lint_findings rides alongside lint_errors, not instead of it:
				// the count is what the terminal line prints, and the findings
				// are the only form an agent can act on. A refusal that said
				// "1 error-level lint finding" and named neither the rule nor
				// the claim sent the agent to `check --validate`, which reports
				// zero of them (the claim is still draft; the rule that refuses
				// keys off the locked form) — an unbreakable loop. See lockGate.
				gate := evaluateLockGates(claim, claims, cfg)
				return cmdResult{}, cliout.Errorf(gate.code(), "lock: %w", err).
					WithDetails(map[string]any{
						"lint_errors":         gate.LintErrors,
						"lint_findings":       gate.lockLintFindingData(),
						"open_threads":        gate.OpenThreads,
						"unlocked_dependency": gate.UnlockedDoctrineDep,
					})
			}
			if err := loader.SaveClaimIfUnchanged(updated, token); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
			}
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
			}
			return cmdResult{
				Warnings: ledgerAdoptionWarnings(adopted),
				Data: lockData{
					ClaimID:  id,
					From:     from,
					To:       string(model.StatusLocked),
					Reason:   reason,
					LockedAt: store.LockedAt[id],
				},
				Text: func() { fmt.Fprintf(cmd.OutOrStdout(), "lock: %s is now locked\n", id) },
			}, nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "the human approval this lock executes, in their words (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what locking would do — transition, preconditions, side effects, what is missing — and write nothing")
	return cmd
}

// unlockData is "dossierx claim unlock"'s machine payload. FlagCleared reports
// whether a pending "dossierx claim flag" trigger was dropped along with the unlock,
// which the prose output never said and an agent has no other way to learn.
type unlockData struct {
	ClaimID     string `json:"claim_id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Reason      string `json:"reason"`
	FlagCleared bool   `json:"flag_cleared"`
}

// unlockDryRun previews "unlock <id>". Unlock has no gates — it is deliberately
// always allowed, since a project may need to unlock a claim precisely to fix
// what lint is complaining about — so the interesting content here is the side
// effects, which are the parts a reviewer cannot infer: review_pending is
// cleared, and a pending flag is silently dropped with it.
//
// The preview therefore evaluates NO preconditions, and specifically not
// "claim_is_locked", which it used to. lock.Unlock has no such gate and neither
// does newUnlockCmd's write path: "dossierx claim unlock" on a draft claim
// succeeds and exits 0. Declaring it blocked was the disagreement in its most
// damaging direction — the preview refused the one command that exists as the
// recovery escape hatch, so an agent following the preview would not reach for
// the move that gets a wedged project moving again. An already-draft claim is
// now reported through a side effect instead: the run is honest about being
// close to a no-op without pretending it will refuse.
func unlockDryRun(claim model.Claim, cfg *config.Config, flagged bool, reason string) *cliout.DryRun {
	dr := cliout.NewDryRun("unlock claim "+claim.ID).
		Transition(string(claim.Status), string(model.StatusDraft))

	if strings.TrimSpace(reason) == "" {
		dr.Lacking("--reason")
	}
	if claim.Status != model.StatusLocked {
		dr.Effect(fmt.Sprintf("this claim is already %q, so the status does not change; the run still clears review_pending and releases any standing lock-ledger record", claim.Status))
	}

	dr.Effect("rewrites " + claim.SourcePath).
		Effect("clears review_pending on this claim")
	if flagged {
		dr.Effect("drops this claim's pending \"dossierx claim flag\" trigger from " + flagStorePath(cfg) +
			" — the recorded before/after assertion is discarded, not applied")
	}
	dr.Effect("the claim becomes editable again: nothing gates changes to it until it is relocked")

	dr.Propose("status", string(model.StatusDraft)).
		Propose("review_pending", false).
		Propose("reason", reason)
	return dr
}

func newUnlockCmd() *cobra.Command {
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "unlock <id>",
		Short: "Unlock a locked claim back to draft; --reason records the human approval it executes",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]

			if dryRun {
				cfg, claims, err := loadConfigAndClaims()
				if err != nil {
					return cmdResult{}, err
				}
				claim, ok := loader.FindByID(claims, id)
				if !ok {
					return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "unlock: claim %q not found: %w", id, errClaimNotFound)
				}
				// Reading the flag store here is best-effort for the same reason
				// the write path tolerates a broken one: a corrupt flag store
				// must not stop a preview of the recovery escape hatch.
				flagged := false
				if flagStore, ferr := reaudit.LoadFlagStore(flagStorePath(cfg)); ferr == nil {
					_, flagged = flagStore.Flags[id]
				}
				return dryRunResult(cmd, "unlock", unlockDryRun(claim, cfg, flagged, reason)), nil
			}

			if err := requireReason("claim unlock", reason); err != nil {
				return cmdResult{}, err
			}

			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}

			// Claim-file write discipline (Phase 0): take the project-wide
			// claims sentinel FIRST — before the flag-store sentinel below —
			// and load claims INSIDE it, so this load->mutate->SaveClaim runs
			// against a snapshot no concurrent claim-file writer can change
			// underneath us. claims-then-flag-store keeps the global order
			// (claims -> lock-store -> flag-store) deadlock-free.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "unlock: %w", err)
			}
			defer releaseClaims()

			claims, err := loadClaims(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "unlock: claim %q not found: %w", id, errClaimNotFound)
			}
			from := string(claim.Status)
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "unlock: %w", err)
			}

			// Unlocking RELEASES this claim's lock-ledger record (it does not
			// delete it — see lock.LedgerRecord). Without a recorded release, a
			// claim that is draft while holding an active record is exactly
			// what the lock-ledger-orphan rule refuses, so an honest unlock
			// would look like someone flipping locked -> draft to dodge review.
			// The lock-store sentinel goes here, BETWEEN the claims sentinel
			// above and the flag-store sentinel below, keeping the global
			// acquisition order claims -> lock-store -> flag-store intact.
			storeRelease, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "unlock: %w", err)
			}
			defer storeRelease()

			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "unlock: %w", err)
			}
			changedStore, adopted := prepareStore(cfg, store, claims)
			if changedStore {
				if err := store.Save(); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "unlock: %w", err)
				}
			}

			// Unlocking must also drop any pending "dossierx claim flag" trigger for
			// this claim (DX-AUD-10): a flag records an agent's before/after
			// assertion against the claim's currently-locked body, and once the
			// claim is unlocked and (presumably) edited, that stale assertion
			// must not survive to be silently applied by a later
			// drift-triggered "dossierx claim reaudit --confirm" after the claim is
			// relocked. Serialize against concurrent flag/reaudit invocations
			// on the shared flag-store file, the same way newLockCmd and
			// newReauditCmd do (AcquireFileLock). Shared lock.Store.Hashes
			// entries are deliberately NOT touched here — they belong to
			// co-dependents and are harmlessly overwritten on the next relock.
			release, err := lock.AcquireFileLock(flagStorePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "unlock: %w", err)
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
			flagCleared := false
			warnings := ledgerAdoptionWarnings(adopted)
			if flagStore, ferr := reaudit.LoadFlagStore(flagStorePath(cfg)); ferr != nil {
				// Kept on stderr for the text surface (unchanged bytes) AND
				// promoted into the envelope's warnings[], because a machine
				// reading only stdout would otherwise never learn that a stale
				// flag survived the unlock.
				warning := fmt.Sprintf("could not read flag store (%v); a pending flag for %s was not cleared", ferr, id)
				warnings = append(warnings, warning)
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
			} else if _, flagged := flagStore.Flags[id]; flagged {
				delete(flagStore.Flags, id)
				if err := flagStore.Save(); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "unlock: %w", err)
				}
				flagCleared = true
			}

			updated := lock.Unlock(claim, store, lock.Approval{Actor: lock.DefaultActor(), Reason: reason})
			if err := loader.SaveClaimIfUnchanged(updated, token); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "unlock: %w", err)
			}
			// Persist the ledger release AFTER the claim file is written: if
			// the claim save fails, the claim is still locked, and a store
			// recording it as released would be a standing invitation to edit
			// it. The reverse ordering (release persisted, claim unchanged) is
			// the one direction that would weaken the gate rather than trip it.
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "unlock: %w", err)
			}
			return cmdResult{
				Data: unlockData{
					ClaimID:     id,
					From:        from,
					To:          string(model.StatusDraft),
					Reason:      reason,
					FlagCleared: flagCleared,
				},
				Warnings: warnings,
				Text:     func() { fmt.Fprintf(cmd.OutOrStdout(), "unlock: %s is now draft\n", id) },
			}, nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "the human approval this unlock executes, in their words (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what unlocking would do — transition, side effects, what is missing — and write nothing")
	return cmd
}

// ---------------------------------------------------------------------
// reaudit <id> [--confirm]
// ---------------------------------------------------------------------

// reauditData is "dossierx claim reaudit"'s machine payload. It carries the SAME
// proposal on the preview and the apply — Applied is what distinguishes them —
// so an agent can show a human the exact bytes it is about to write and then
// re-show that the write happened, without re-deriving anything.
//
// Trigger names why the claim is review_pending at all: "flag" (a recorded
// dossierx flag), "drift" (a dependency changed), or "none" (a drift that was
// reverted, leaving the flag set with nothing left to explain it). It matters
// because reaudit is the DRIFT tool, and an agent that finds trigger "none"
// should be reaching for unlock -> fix -> lock, not for this command.
type reauditData struct {
	ClaimID       string `json:"claim_id"`
	Trigger       string `json:"trigger"`
	NoChange      bool   `json:"no_change"`
	Note          string `json:"note"`
	Body          string `json:"body"`
	Applied       bool   `json:"applied"`
	Reason        string `json:"reason,omitempty"`
	ReviewPending bool   `json:"review_pending"`
}

// reauditTrigger names the live pending trigger behind a review_pending claim.
func reauditTrigger(drift, flagged bool) string {
	switch {
	case flagged:
		return "flag"
	case drift:
		return "drift"
	default:
		return "none"
	}
}

// newReauditCmd wires "dossierx claim reaudit <id>".
//
// Three flags interact here, and the rule that keeps them from colliding is
// stated once, in cliout.DryRun's doc, and enforced here: --dry-run NEVER
// writes and ALWAYS wins.
//
//	reaudit <id>                      preview: prints the proposed diff, writes
//	                                  nothing (its historical behavior, kept)
//	reaudit <id> --confirm            apply: writes, and therefore requires
//	                                  --reason
//	reaudit <id> --dry-run            the machine-readable preview of the apply
//	reaudit <id> --dry-run --confirm  still a preview; nothing is written
//
// --reason is required only on the writing path. Demanding the human's
// approving words before the agent may LOOK at a proposal would invert the
// order of the loop this release is built around: the agent previews, shows the
// human, and only then has words to record.
func newReauditCmd() *cobra.Command {
	var confirm bool
	var dryRun bool
	var reason string
	cmd := &cobra.Command{
		Use:   "reaudit <id>",
		Short: "Propose (and, with --confirm and --reason, apply) a diff for a review_pending claim",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				// v0.1.2 reached exit 2 here through a bare os.Exit(2), which
				// no envelope could survive. Returning the wrapped sentinel
				// instead keeps the exit status identical AND lets the failure
				// be reported as a document like every other failure.
				return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "reaudit: claim %q not found: %w", id, errClaimNotFound)
			}

			if dryRun {
				return reauditDryRunResult(cmd, cfg, claims, claim, reason)
			}

			// Per SPEC, reaudit is only ever valid on a locked+review_pending
			// claim; anything else is exit 2. reaudit is the DRIFT tool, not
			// the general edit tool — the general path is unlock -> fix -> lock
			// — so the hint points there rather than at a way around this.
			if claim.Status != model.StatusLocked || !claim.ReviewPending {
				return cmdResult{}, cliout.Errorf(cliout.CodeNotReviewPending,
					"reaudit: claim %q is not locked+review_pending: %w", id, errWrongState).
					WithHint(fmt.Sprintf("to change a locked claim that is not drifting: dossierx claim unlock %s --reason \"...\", edit, then dossierx claim lock %s --reason \"...\"", id, id))
			}

			// --reason gates the WRITE, not the preview: see this command's
			// doc comment.
			if confirm {
				if err := requireReason("claim reaudit", reason); err != nil {
					return cmdResult{}, err
				}
			}

			// Claim-file write discipline (Phase 0): the guards above ran on a
			// lock-free load DELIBERATELY — they hold no file lock, so an early
			// return leaks nothing. Now take the project-wide claims sentinel
			// FIRST (before the lock-store and flag-store sentinels below,
			// preserving the global claims -> lock-store -> flag-store order)
			// and RE-READ claims inside it, so a confirmed reaudit's
			// load->mutate->SaveClaim runs against a snapshot no concurrent
			// claim-file writer can change underneath us.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "reaudit: %w", err)
			}
			defer releaseClaims()

			claims, err = loadClaims(cfg)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeInvalidClaim, "reaudit: %w", err)
			}
			claim, ok = loader.FindByID(claims, id)
			if !ok {
				// Vanished between the guard and the sentinel (an out-of-band
				// delete). Refuse via the sentinel error so the deferred
				// release still runs; main() maps it to the same exit 2.
				return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "reaudit: claim %q not found: %w", id, errClaimNotFound)
			}
			if claim.Status != model.StatusLocked || !claim.ReviewPending {
				return cmdResult{}, cliout.Errorf(cliout.CodeNotReviewPending, "reaudit: claim %q is not locked+review_pending: %w", id, errWrongState)
			}
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "reaudit: %w", err)
			}

			// See newLockCmd's comment: serializes against any concurrent
			// "dossierx claim lock"/"dossierx claim reaudit --confirm" invocation touching the
			// same store file, so a confirmed reaudit's store.Save() below
			// never races another process's load-mutate-save on the same
			// Hashes/LockedAt map.
			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "reaudit: %w", err)
			}
			defer release()

			// Same reasoning, applied to the (separate) pending-flag store:
			// a confirmed reaudit for a flag-sourced claim deletes that
			// claim's entry below, and two concurrent reaudit/flag
			// invocations must not race on that shared file either.
			flagRelease, err := lock.AcquireFileLock(flagStorePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "reaudit: %w", err)
			}
			defer flagRelease()

			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "reaudit: %w", err)
			}
			// Re-arm a legacy (pre-versioning) store's per-dependent baselines
			// from current content — see lock.MigrateLegacyStore. Persisted here
			// (not only on the --confirm path below) so a mere propose still
			// restores drift detection for the project's already-locked claims.
			// PrepareStore also grandfathers already-locked claims into the
			// lock ledger the first time a pre-ledger store is opened.
			changedStore, adopted := prepareStore(cfg, store, claims)
			if changedStore {
				if err := store.Save(); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "reaudit: %w", err)
				}
			}
			flagStore, err := reaudit.LoadFlagStore(flagStorePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "reaudit: %w", err)
			}

			// Two trigger sources converge here: a claim with a pending
			// "dossierx claim flag" entry gets the real, ready-to-review diff
			// ProposeFlagDiff builds from it; every other review_pending
			// claim (the pre-existing, and still only, case for a project
			// that has never used "dossierx claim flag") keeps going through
			// ProposeDiff's dependency-diff stub exactly as before.
			pendingFlag, flagged := flagStore.Flags[id]

			// A claim whose ONLY pending trigger is an open comment thread has
			// nothing for reaudit to do: reaudit reviews a proposed CONTENT
			// change (a drifted dependency, or a "dossierx claim flag"), and a comment
			// thread is discussion, not an edit to diff-and-confirm. Refuse with
			// exit 2 BEFORE proposing or writing anything. Crucially this point
			// is PAST both file locks (store + flag, acquired above), so it must
			// NOT os.Exit(2) — that skips the deferred releases and leaks the
			// two held locks. Returning a wrapped errWrongState lets the defers
			// run and main() map it to exit 2. The remedy is to resolve the
			// thread, which clears review_pending on its own.
			drift, flagTrigger, open := comments.PendingTriggers(claim, claims, store, flagStore)
			if !drift && !flagTrigger && open > 0 {
				return cmdResult{}, cliout.Errorf(cliout.CodeReviewPending,
					"reaudit: claim %q is review_pending only because of %d open comment thread(s); a human resolves those in the viewer — nothing to reaudit: %w", id, open, errWrongState).
					WithDetails(map[string]any{"open_threads": open}).
					WithHint("the human resolves the thread in the viewer; that click is the approval")
			}

			var diff reaudit.Diff
			if flagged {
				diff, err = reaudit.ProposeFlagDiff(claim, pendingFlag)
			} else {
				changedDep := pickChangedDependency(claim, claims, store)
				diff, err = reaudit.ProposeDiff(claim, changedDep)
			}
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeInternal, "reaudit: %w", err)
			}

			// The proposal block is printed identically on both paths (preview
			// and apply), so it is built once here and the two paths only
			// differ in the trailing line.
			printProposal := func() {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "reaudit: %s (no_change=%v)\n", diff.ClaimID, diff.NoChange)
				fmt.Fprintf(out, "note: %s\n", diff.Note)
				fmt.Fprintln(out, "---")
				fmt.Fprintln(out, diff.Body)
				fmt.Fprintln(out, "---")
			}

			data := reauditData{
				ClaimID:  id,
				Trigger:  reauditTrigger(drift, flagged),
				NoChange: diff.NoChange,
				Note:     diff.Note,
				Body:     diff.Body,
				Reason:   reason,
			}

			if !confirm {
				data.Applied = false
				data.ReviewPending = claim.ReviewPending
				return cmdResult{
					Warnings: ledgerAdoptionWarnings(adopted),
					Data:     data,
					Text: func() {
						printProposal()
						fmt.Fprintln(cmd.OutOrStdout(), "reaudit: not applied (pass --confirm to apply)")
					},
				}, nil
			}

			// THE PRE-REAUDIT INTEGRITY GATE, and it belongs here — ahead of
			// Apply, ahead of every write — for the same reason lock.Lock's
			// already-locked refusal belongs ahead of its lint gate.
			//
			// RecordApproval below re-signs the WHOLE claim as it is on disk. A
			// confirmed reaudit is one of only two paths in the product allowed
			// to do that, and without this gate it is a laundering path made of
			// one documented command:
			//
			//	1. a locked claim is tampered with out of band (a hand edit, a bad
			//	   merge, an agent that wrote the file directly). "check" correctly
			//	   reports lock-content-drift against its standing record.
			//	2. something unrelated and entirely legitimate — an unlock -> edit
			//	   -> lock on one of its DEPENDENCIES — flips it to review_pending
			//	   with trigger "drift". No human ever saw the tampered fields.
			//	3. the documented recovery for a drifted claim, "reaudit <id>
			//	   --confirm --reason ...", now re-signs the tampered bytes under a
			//	   fresh approval. The finding disappears permanently, and no
			//	   unlock ever happened.
			//
			// The diff a reaudit confirms is about the claim's BODY. It is not,
			// and was never, an approval of whatever else the file has grown since
			// it was locked. So a claim that no longer matches its standing record
			// is refused before anything is proposed as approved, and the message
			// names the two ways out — restore from version control (the right one
			// when the edit was not intended) or unlock -> fix -> lock (the right
			// one when it was).
			if _, standing, matches := standingLedgerRecord(store, claim); standing && !matches {
				return cmdResult{}, cliout.Errorf(cliout.CodeIntegrityFailed,
					"reaudit: claim %q no longer matches the content on its lock-ledger record, so a confirmed reaudit would re-sign content nobody approved; restore the file from version control, or go through unlock -> fix -> lock", id).
					WithHint(fmt.Sprintf("run: dossierx check --validate (it names the finding), then either restore %s from git or dossierx claim unlock %s --reason \"...\"", claim.SourcePath, id))
			}

			applied, err := reaudit.Apply(claim, diff)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeInternal, "reaudit: %w", err)
			}
			// Re-baseline this claim's dependency hashes and refresh its lock
			// timestamp — the drift-clearing half of the old ClearReviewPending.
			// review_pending is deliberately NOT hard-cleared here: it becomes
			// the Recompute verdict below, so a claim still carrying an
			// independent open comment thread stays review_pending even though
			// this confirmed reaudit cleared the drift/flag that prompted it
			// (a comment is a third trigger reaudit does not resolve).
			lock.RefreshBaseline(applied, claims, store)
			// Re-sign the claim in the lock ledger. A confirmed reaudit is the
			// SECOND path in the product that legitimately rewrites a locked
			// claim's signed content — it rewrites body and appends to
			// audit_notes, both of which LockedClaimHash covers — so without
			// this the gate would report every honest reaudit as tampering
			// from the moment it landed. The --reason that authorized the
			// reaudit is what goes on the record.
			lock.RecordApproval(store, applied, lock.Approval{Actor: lock.DefaultActor(), Reason: reason})
			if flagged {
				// A flag is a one-shot trigger (see PendingFlag's doc comment):
				// remove it BEFORE recomputing so the verdict sees it cleared,
				// and so a future dependency-drift reaudit on this same claim
				// doesn't mistake a stale flag for a still-pending one.
				delete(flagStore.Flags, id)
			}
			applied.ReviewPending = comments.Recompute(applied, claims, store, flagStore)
			if err := loader.SaveClaimIfUnchanged(applied, token); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "reaudit: %w", err)
			}
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "reaudit: %w", err)
			}
			if flagged {
				if err := flagStore.Save(); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "reaudit: %w", err)
				}
			}
			data.Applied = true
			data.ReviewPending = applied.ReviewPending
			return cmdResult{
				Warnings: ledgerAdoptionWarnings(adopted),
				Data:     data,
				Text: func() {
					printProposal()
					out := cmd.OutOrStdout()
					if applied.ReviewPending {
						fmt.Fprintf(out, "reaudit: %s applied; review_pending retained (open comment thread(s) remain — resolve them to clear)\n", id)
					} else {
						fmt.Fprintf(out, "reaudit: %s applied, review_pending cleared\n", id)
					}
				},
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "apply the proposed diff (otherwise only prints it)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what a confirmed reaudit would do and write nothing; always wins over --confirm")
	cmd.Flags().StringVar(&reason, "reason", "", "the human approval this reaudit executes, in their words (required with --confirm)")
	return cmd
}

// reauditDryRunResult builds the "reaudit <id> --dry-run" preview.
//
// It reads the lock and flag stores WITHOUT taking their sentinels and without
// lock.MigrateLegacyStore's persisted re-arm, both of which the write path
// does: a dry run that quietly rewrote the lock store would be a dry run in
// name only. The cost is that a legacy store's baselines are not re-armed here,
// so on the very first run after an upgrade a preview may report drift the
// subsequent real run does not. That is the right trade — a preview may be
// pessimistic, it may not have side effects.
func reauditDryRunResult(cmd *cobra.Command, cfg *config.Config, claims []model.Claim, claim model.Claim, reason string) (cmdResult, error) {
	dr := cliout.NewDryRun("apply a reaudit diff to claim " + claim.ID)

	if strings.TrimSpace(reason) == "" {
		dr.Lacking("--reason")
	}
	dr.Require("claim_is_locked", claim.Status == model.StatusLocked,
		fmt.Sprintf("status is %q", claim.Status))
	dr.Require("claim_is_review_pending", claim.ReviewPending,
		fmt.Sprintf("review_pending is %v", claim.ReviewPending))

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeInternal, "reaudit: %w", err)
	}
	flagStore, err := reaudit.LoadFlagStore(flagStorePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeInternal, "reaudit: %w", err)
	}
	pendingFlag, flagged := flagStore.Flags[claim.ID]
	drift, flagTrigger, open := comments.PendingTriggers(claim, claims, store, flagStore)

	// The one refusal reaudit has that is neither "wrong status" nor "wrong
	// flag": review_pending caused ONLY by discussion. There is no diff to
	// confirm, and the remedy is a human clicking Resolve — so it is reported
	// as a precondition, not smuggled in as a side effect.
	dr.Require("has_a_content_trigger", drift || flagTrigger,
		fmt.Sprintf("drift=%v flag=%v open_comment_threads=%d", drift, flagTrigger, open))

	// The pre-reaudit integrity gate, previewed. The write path refuses a claim
	// whose content no longer matches its standing lock-ledger record (see the
	// long comment at that gate for the laundering path it closes), and a
	// preview that did not say so would send an agent to compose a --reason for
	// a command that is going to refuse.
	_, standing, matches := standingLedgerRecord(store, claim)
	dr.Require("content_matches_ledger", matches,
		fmt.Sprintf("a standing lock-ledger approval covers this claim=%v; its recorded content still matches the file=%v", standing, matches))

	// The proposal is computed only once the state gates hold. internal/reaudit
	// refuses to propose for a claim that is not locked+review_pending, and
	// surfacing that refusal as a command ERROR would break the rule this whole
	// mechanism rests on: a dry run fails only when it cannot compute the
	// preview at all (no config, no such claim). Every other refusal is a
	// failed precondition on a successful, blocked report — otherwise "the
	// answer is no" and "the preview itself broke" become indistinguishable.
	if !dr.Blocked {
		var diff reaudit.Diff
		if flagged {
			diff, err = reaudit.ProposeFlagDiff(claim, pendingFlag)
		} else {
			diff, err = reaudit.ProposeDiff(claim, pickChangedDependency(claim, claims, store))
		}
		if err != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeInternal, "reaudit: %w", err)
		}
		dr.Propose("no_change", diff.NoChange).
			Propose("note", diff.Note).
			Propose("body", diff.Body)
	}

	dr.Effect("rewrites " + claim.SourcePath + " — reaudit can only change body, nothing else").
		Effect("re-baselines this claim's dependency content hashes and refreshes its lock timestamp in " + storePath(cfg))
	if flagged {
		dr.Effect("consumes this claim's one-shot \"dossierx claim flag\" trigger in " + flagStorePath(cfg))
	}
	if open > 0 {
		dr.Effect(fmt.Sprintf("review_pending is RETAINED: %d open comment thread(s) remain, and reaudit does not resolve discussion", open))
	}

	dr.Propose("trigger", reauditTrigger(drift, flagged)).
		Propose("reason", reason)

	return dryRunResult(cmd, "reaudit", dr), nil
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
