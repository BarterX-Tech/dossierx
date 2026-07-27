// build_order.go wires internal/buildorder into the CLI as "dossierx
// build-order propose|status|lock", split out of main.go (already ~660
// lines before this feature) purely for file size — the construction
// pattern (newXCmd() *cobra.Command, added to newRootCmd()'s AddCommand
// list) is identical to every other command in main.go.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// newBuildOrderCmd is the "dossierx build-order" command group: propose, status,
// and lock, one subcommand each, all operating on a single --module.
func newBuildOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-order",
		Short: "Compute, inspect, and lock a module's build (implementation) order",
	}
	cmd.AddCommand(
		newBuildOrderProposeCmd(),
		newBuildOrderStatusCmd(),
		newBuildOrderLockCmd(),
	)
	return commandGroup(cmd)
}

// requireModuleFlag validates the shared --module flag every build-order
// subcommand takes: it must be set, since (unlike "dossierx claim lock <id>", which
// names one claim) a build order is always computed for one whole module
// at a time.
func requireModuleFlag(module string) error {
	if module == "" {
		return cliout.Errorf(cliout.CodeMissingFlag, "build-order: --module is required")
	}
	return nil
}

// buildOrderPhaseData is one phase of a proposed order, flattened for the
// envelope: the phase name and the claim ids in it, in order. Only the ids are
// carried — the artifact's per-claim File and RestsOn are already on disk in
// the artifact itself, and what a caller wants from this envelope is the
// sequence.
type buildOrderPhaseData struct {
	Phase  string   `json:"phase"`
	Claims []string `json:"claims"`
}

// buildOrderProposeData is "build-order propose"'s machine payload. The
// per-phase claim IDS are included, not just the counts the terminal prints:
// the whole reason an agent asks for a build order is to know what to implement
// next, and a count cannot answer that.
type buildOrderProposeData struct {
	Module   string                `json:"module"`
	Path     string                `json:"path"`
	Phases   []buildOrderPhaseData `json:"phases"`
	Excluded []string              `json:"excluded"`
	Locked   bool                  `json:"locked"`
}

// buildOrderStatusData is "build-order status"'s machine payload. Proposed
// false is a legitimate, successful answer — the module simply has no artifact
// yet — so it is data, not an error, exactly as the text path has always
// treated it.
type buildOrderStatusData struct {
	Module        string   `json:"module"`
	Proposed      bool     `json:"proposed"`
	Locked        bool     `json:"locked"`
	LockedAt      string   `json:"locked_at,omitempty"`
	Stale         bool     `json:"stale"`
	StaleIDs      []string `json:"stale_ids"`
	CoveredClaims int      `json:"covered_claims"`
	TotalClaims   int      `json:"total_claims"`
	ExcludedCount int      `json:"excluded_count"`
}

// buildOrderLockData is "build-order lock"'s machine payload, carrying the
// human words that authorized it — a locked build order is a locked artifact,
// and this release's invariant is that nothing already locked changes without
// an approval on the record.
type buildOrderLockData struct {
	Module   string `json:"module"`
	Path     string `json:"path"`
	LockedAt string `json:"locked_at"`
	Reason   string `json:"reason"`
}

// phaseData projects an artifact's phases into the envelope shape.
func phaseData(a *buildorder.Artifact) []buildOrderPhaseData {
	phases := make([]buildOrderPhaseData, 0, len(a.Phases))
	for _, p := range a.Phases {
		ids := make([]string, 0, len(p.Claims))
		for _, entry := range p.Claims {
			ids = append(ids, entry.ID)
		}
		phases = append(phases, buildOrderPhaseData{Phase: p.Phase, Claims: ids})
	}
	return phases
}

func newBuildOrderProposeCmd() *cobra.Command {
	var module string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "propose",
		Short: "Propose (and write) a build-order artifact for --module (refused unless every claim in it is locked)",
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			if err := requireModuleFlag(module); err != nil {
				return cmdResult{}, err
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			if err := requireKnownModule(cfg, module); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeUnknownModule, "build-order propose: %w", err)
			}

			// Propose is a pure computation over claims plus one file write, so
			// the dry run is the same computation with the write skipped — the
			// preview cannot disagree with the real run about what it would
			// produce, because it IS the real run minus WriteArtifact.
			artifact, proposeErr := buildorder.Propose(claims, cfg, module)
			path := buildorder.ArtifactPath(cfg, module)

			// What is on disk already, and would a propose DESTROY it? See
			// approvedOrderWouldBeDiscarded: a locked, non-stale artifact is an
			// approved implementation sequence with a standing ledger record, and
			// propose rewrites the file in full with locked:false.
			existing, existingErr := buildorder.Status(path, claims, cfg)
			discards := approvedOrderWouldBeDiscarded(existing, existingErr)

			if dryRun {
				dr := cliout.NewDryRun("propose a build order for module " + module)
				dr.Require("module_is_orderable", proposeErr == nil, proposeErrDetail(proposeErr))
				dr.Require("no_approved_order_to_discard", !discards, existingOrderDetail(existing, existingErr))
				dr.Effect("writes " + path + ", overwriting any existing proposal for this module")
				if proposeErr == nil {
					for _, p := range phaseData(artifact) {
						dr.Propose(p.Phase, p.Claims)
					}
					dr.Propose("excluded", artifact.Excluded).Propose("locked", false)
				}
				return dryRunResult(cmd, "build-order propose", dr), nil
			}

			if proposeErr != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeBuildOrderRefused, "build-order propose: %w", proposeErr)
			}
			if discards {
				return cmdResult{}, cliout.Errorf(cliout.CodeAlreadyLocked,
					"build-order propose: module %q's build order is locked and current; re-proposing would discard an approved order and replace it with an unlocked recomputation, leaving its lock-ledger record pointing at content that no longer exists", module).
					WithHint(fmt.Sprintf("if the order genuinely needs to change, change what it is derived from — dossierx claim unlock <id> --reason \"...\", edit, relock — which makes the order stale, and a stale order may be re-proposed and re-locked (dossierx build-order status --module %s)", module))
			}
			if err := buildorder.WriteArtifact(artifact, path); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "build-order propose: %w", err)
			}

			excluded := artifact.Excluded
			if excluded == nil {
				excluded = []string{}
			}
			return cmdResult{
				Data: buildOrderProposeData{
					Module:   module,
					Path:     path,
					Phases:   phaseData(artifact),
					Excluded: excluded,
					Locked:   false,
				},
				Text: func() {
					out := cmd.OutOrStdout()
					fmt.Fprintf(out, "build-order propose: wrote %s\n", path)
					for _, p := range artifact.Phases {
						fmt.Fprintf(out, "  %-14s %d claim(s)\n", p.Phase, len(p.Claims))
					}
					fmt.Fprintf(out, "  %-14s %d claim(s)\n", "excluded", len(artifact.Excluded))
					fmt.Fprintln(out, "  locked: false")
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&module, "module", "", "module to compute the build order for (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report the order that would be written, and write nothing")
	return cmd
}

// approvedOrderWouldBeDiscarded reports whether re-proposing over the artifact
// already on disk would destroy something a human approved.
//
// "build-order propose" writes the artifact in FULL, with locked:false and a
// freshly recomputed sequence. Against a locked, current artifact that is a
// silent, reason-less, read-looking command overwriting the implementation order
// a human reviewed and locked — and it is invisible to every gate afterwards,
// because internal/check only audits artifacts whose locked flag is true. The
// approval record for build-order:<module> is left standing, unreleased, and
// pointing at content that now exists nowhere. buildorder.Lock refuses to
// re-lock an already-locked artifact for the same reason; propose was the
// remaining door.
//
// A STALE locked order is deliberately NOT refused. Re-proposing a stale order
// is the documented recovery for build_order_stale — buildorder.Lock's own
// refusal message and the dossierx-build-order skill both say "re-propose, then
// re-lock" — so refusing it here would leave a stale order with no way forward
// at all, which is strictly worse than the hole being closed.
//
// An artifact that is absent (ErrNotProposed) or unreadable is not an approved
// order either. Absent is the ordinary first-propose case; unreadable means
// nobody can follow the sequence in it anyway, and propose is the regeneration
// path — refusing there would make a corrupt artifact fixable only by deleting
// it by hand, which is precisely the hand-editing this release exists to gate.
func approvedOrderWouldBeDiscarded(existing *buildorder.Artifact, err error) bool {
	if err != nil || existing == nil {
		return false
	}
	return existing.Locked && !existing.Stale
}

// existingOrderDetail renders the state of the artifact already on disk as a
// dry-run precondition detail, so the preview says WHY it is (or is not)
// blocked rather than only that it is.
func existingOrderDetail(existing *buildorder.Artifact, err error) string {
	switch {
	case errors.Is(err, buildorder.ErrNotProposed):
		return "no build order proposed yet for this module"
	case err != nil:
		return fmt.Sprintf("the existing artifact could not be read (%v); propose would regenerate it", err)
	case existing == nil:
		return "no build order proposed yet for this module"
	case existing.Locked && existing.Stale:
		return fmt.Sprintf("locked=true stale=true (%d claim(s) moved) — re-proposing a stale order is the documented recovery", len(existing.StaleIDs))
	default:
		return fmt.Sprintf("locked=%v stale=%v", existing.Locked, existing.Stale)
	}
}

// proposeErrDetail renders a Propose failure as a dry-run precondition detail,
// or the passing case's detail when there was none.
func proposeErrDetail(err error) string {
	if err == nil {
		return "every claim in the module is locked and carries a build_role"
	}
	return err.Error()
}

func newBuildOrderStatusCmd() *cobra.Command {
	var module string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show --module's build-order artifact state: proposed/locked/stale and coverage",
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			if err := requireModuleFlag(module); err != nil {
				return cmdResult{}, err
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			if err := requireKnownModule(cfg, module); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeUnknownModule, "build-order status: %w", err)
			}

			path := buildorder.ArtifactPath(cfg, module)
			artifact, err := buildorder.Status(path, claims, cfg)
			if err != nil {
				if errors.Is(err, buildorder.ErrNotProposed) {
					// Not an error: "nothing here yet" is the answer, and a
					// status command that failed for want of a thing to report
					// on would be unusable in exactly the case it is most
					// needed.
					return cmdResult{
						Data: buildOrderStatusData{Module: module, StaleIDs: []string{}},
						Text: func() {
							fmt.Fprintf(cmd.OutOrStdout(), "build-order status: %s: not proposed yet (run \"dossierx build-order propose --module %s\")\n", module, module)
						},
					}, nil
				}
				return cmdResult{}, cliout.Errorf(cliout.CodeInternal, "build-order status: %w", err)
			}

			total := 0
			for _, c := range claims {
				if c.Module == module {
					total++
				}
			}
			covered := len(artifact.ClaimIDs())
			staleIDs := artifact.StaleIDs
			if staleIDs == nil {
				staleIDs = []string{}
			}

			return cmdResult{
				Data: buildOrderStatusData{
					Module:        module,
					Proposed:      true,
					Locked:        artifact.Locked,
					LockedAt:      artifact.LockedAt,
					Stale:         artifact.Stale,
					StaleIDs:      staleIDs,
					CoveredClaims: covered,
					TotalClaims:   total,
					ExcludedCount: len(artifact.Excluded),
				},
				Text: func() {
					out := cmd.OutOrStdout()
					fmt.Fprintf(out, "build-order status: %s\n", module)
					fmt.Fprintf(out, "  proposed: true\n")
					if artifact.Locked {
						fmt.Fprintf(out, "  locked:   true (locked_at: %s)\n", artifact.LockedAt)
					} else {
						fmt.Fprintf(out, "  locked:   false\n")
					}
					if artifact.Stale {
						fmt.Fprintf(out, "  stale:    true (%d claim(s): %v)\n", len(artifact.StaleIDs), artifact.StaleIDs)
					} else {
						fmt.Fprintf(out, "  stale:    false\n")
					}
					fmt.Fprintf(out, "  coverage: %d of %d claim(s) covered (%d excluded as out-of-scope)\n",
						covered, total, len(artifact.Excluded))
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&module, "module", "", "module to report build-order status for (required)")
	return cmd
}

// newBuildOrderLockCmd wires "dossierx build-order lock --module <m>".
//
// It takes --reason for the same reason claim lock does, and the audit is
// explicit about why: a locked build order is a SECOND class of locked artifact,
// and leaving it outside the approval path would make this release's headline
// invariant — "nothing already locked changes without your approval on the
// record" — an overclaim.
func newBuildOrderLockCmd() *cobra.Command {
	var module string
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock --module's proposed build-order artifact, snapshotting a content-hash baseline",
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			if err := requireModuleFlag(module); err != nil {
				return cmdResult{}, err
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			if err := requireKnownModule(cfg, module); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeUnknownModule, "build-order lock: %w", err)
			}
			path := buildorder.ArtifactPath(cfg, module)

			if dryRun {
				dr := cliout.NewDryRun("lock the build order for module " + module)
				if strings.TrimSpace(reason) == "" {
					dr.Lacking("--reason")
				}
				// Status is the read-only half of Lock's own preconditions:
				// it answers "is there an artifact, and is it already locked
				// and current?" without touching the file.
				artifact, statusErr := buildorder.Status(path, claims, cfg)
				switch {
				case errors.Is(statusErr, buildorder.ErrNotProposed):
					dr.Require("build_order_proposed", false, "no artifact at "+path)
				case statusErr != nil:
					dr.Require("build_order_readable", false, statusErr.Error())
				default:
					dr.Require("build_order_proposed", true, path)
					dr.Require("not_already_current", !artifact.Locked || artifact.Stale,
						fmt.Sprintf("locked=%v stale=%v", artifact.Locked, artifact.Stale))
					dr.Propose("phases", phaseData(artifact))
				}
				dr.Effect("rewrites " + path + " with locked: true and a content-hash baseline").
					Effect("the order becomes the implementation sequence: it goes stale, rather than silently changing, when its claims move")
				dr.Propose("locked", true).Propose("reason", reason)
				return dryRunResult(cmd, "build-order lock", dr), nil
			}

			if err := requireReason("build-order lock", reason); err != nil {
				return cmdResult{}, err
			}

			artifact, err := buildorder.Lock(path, claims, cfg)
			if err != nil {
				return cmdResult{}, cliout.Errorf(buildOrderLockCode(err), "build-order lock: %w", err)
			}

			// Record the approval in the lock ledger. A locked build order is
			// the SECOND class of locked artifact this engine has, and leaving
			// it outside the ledger would make the release's headline invariant
			// — "nothing already locked changes without your approval on the
			// record" — an overclaim about half the locked things in a project.
			//
			// The ledger write is best-effort and never fails the command: the
			// artifact itself is already written by the time we get here (Lock
			// writes it), so failing now would report an error for an operation
			// that succeeded, and re-running would refuse ("already locked and
			// not stale"). The warning goes into the envelope AND onto stderr,
			// so a missing record is visible rather than assumed.
			var warnings []string
			if warning := recordBuildOrderApproval(cfg, module, artifact, reason); warning != "" {
				warnings = append(warnings, warning)
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
			}
			return cmdResult{
				Warnings: warnings,
				Data:     buildOrderLockData{Module: module, Path: path, LockedAt: artifact.LockedAt, Reason: reason},
				Text: func() {
					fmt.Fprintf(cmd.OutOrStdout(), "build-order lock: %s locked at %s\n", module, artifact.LockedAt)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&module, "module", "", "module to lock the build order for (required)")
	cmd.Flags().StringVar(&reason, "reason", "", "the human approval this lock executes, in their words (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what locking the build order would do, and write nothing")
	return cmd
}

// recordBuildOrderApproval writes module's freshly-locked build-order artifact
// into the lock ledger, returning "" on success or a human-facing warning
// sentence on failure (see the call site for why this never fails the command).
//
// The artifact's signature is hashed HERE rather than in internal/lock because
// internal/buildorder imports internal/lock (for ContentHash), so computing it
// there would invert that edge into an import cycle. It hashes the artifact's
// canonical JSON — module, phases, per-claim entries, excluded set, the frozen
// hash baseline, and the LockedAt stamp — which is precisely the content
// buildorder.WriteArtifact just persisted, so a later hand edit of
// .build-order.<module>.json no longer matches its record.
func recordBuildOrderApproval(cfg *config.Config, module string, artifact *buildorder.Artifact, reason string) string {
	release, err := lock.AcquireFileLock(storePath(cfg))
	if err != nil {
		return fmt.Sprintf("the build order for %s was locked, but its lock-ledger record could not be written (%v)", module, err)
	}
	defer release()

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return fmt.Sprintf("the build order for %s was locked, but the lock ledger could not be read (%v)", module, err)
	}

	hash, err := buildOrderSignature(artifact)
	if err != nil {
		return fmt.Sprintf("the build order for %s was locked, but its content could not be hashed for the lock ledger (%v)", module, err)
	}

	lock.RecordBuildOrderApproval(store, module, hash,
		lock.Approval{Actor: lock.DefaultActor(), Reason: reason})
	if err := store.Save(); err != nil {
		return fmt.Sprintf("the build order for %s was locked, but its lock-ledger record could not be saved (%v)", module, err)
	}
	return ""
}

// buildOrderSignature hashes a build-order artifact for the lock ledger:
// sha256 over encoding/json's canonical marshalling of the whole artifact —
// module, phases, per-claim entries, excluded set, the frozen hash baseline and
// the LockedAt stamp, which is precisely the content buildorder.WriteArtifact
// persists.
//
// The READING side (internal/check's build-order gate) must compute this
// byte-for-byte identically or every honestly-locked build order in every
// project would report drift. The two are deliberately separate small functions
// rather than one shared export, because sharing would mean either cmd/ or
// check/ importing the other; TestBuildOrderSignatureMatchesTheGate pins them
// in agreement instead.
func buildOrderSignature(a *buildorder.Artifact) (string, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// buildOrderLockCode classifies a buildorder.Lock refusal.
//
// ErrNotProposed and ErrStale are sentinels, so those two are matched
// structurally. The stale case USED to fall through to the default and report
// build_order_refused, which made cliout.CodeBuildOrderStale a code the binary
// declared and never emitted — a dead branch in every skill that documents it,
// with the agent instead receiving build_order_refused's recoveries ("lock every
// claim in the module first", "give each claim a build_role") which the stale
// artifact has already satisfied. The remaining "already locked and not stale"
// refusal is still prose, matched on the one stable fragment of its own message;
// TestBuildOrderLockCodes pins the classification until it too is a sentinel.
func buildOrderLockCode(err error) cliout.Code {
	switch {
	case errors.Is(err, buildorder.ErrNotProposed):
		return cliout.CodeNotProposed
	case errors.Is(err, buildorder.ErrStale):
		return cliout.CodeBuildOrderStale
	// The hand-edit refusal must be classified ABOVE the default, and separately
	// from CodeBuildOrderRefused: the artifact is wrong while the claims are
	// correct, which is the exact inverse of every cause build_order_refused
	// documents. See cliout.CodeBuildOrderHandEdited.
	case errors.Is(err, buildorder.ErrHandEdited):
		return cliout.CodeBuildOrderHandEdited
	case strings.Contains(err.Error(), "already locked and not stale"):
		return cliout.CodeAlreadyLocked
	default:
		return cliout.CodeBuildOrderRefused
	}
}
