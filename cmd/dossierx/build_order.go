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
	"github.com/BarterX-Tech/dossierx/internal/model"
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
		// EVERY BUILD-ORDER LEAF SELECTS WITH --module AND NOTHING ELSE, so a
		// positional names nothing this command could act on. Left undeclared,
		// cobra's legacyArgs accepts one on a leaf and throws it away, and
		// `dossierx build-order propose my-module` then proposes for whatever
		// --module happened to hold — or fails "missing --module" while the
		// module the caller named sits ignored on the same line. Declaring
		// NoArgs turns both into a usage error naming the flag.
		//
		// See `check`'s Args in main.go for why this class of change waited for
		// an announced release: it moves an invocation from exit 0 to exit 1.
		Args: cobra.NoArgs,
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
			// approvedOrderWouldBeDiscarded: a locked, non-stale artifact WITH A
			// STANDING LEDGER RECORD is an approved implementation sequence, and
			// propose rewrites the file in full with locked:false.
			existing, existingErr := buildorder.Status(path, claims, cfg)
			discards := approvedOrderWouldBeDiscarded(cfg, module, existing, existingErr)

			if dryRun {
				dr := cliout.NewDryRun("propose a build order for module " + module)
				dr.Require("module_is_orderable", proposeErr == nil, proposeErrDetail(proposeErr))
				dr.Require("no_approved_order_to_discard", !discards, existingOrderDetail(cfg, module, existing, existingErr))
				dr.Effect("writes " + path + ", overwriting any existing proposal for this module")
				// The ledger write is a SIDE EFFECT of proposing, and one a
				// reader would not predict from the verb, so the preview names
				// it rather than letting the real run be the first mention.
				if buildOrderRecordStands(cfg, module) {
					dr.Effect(fmt.Sprintf("releases module %q's standing build-order approval in the lock ledger (the record is kept and marked released, never deleted) — the order it approved is being replaced by an unlocked proposal", module))
				}
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
			// The lock-store sentinel is taken BEFORE the artifact is written and
			// held across both writes, for the reason "build-order lock" states
			// at its own acquisition: this command now writes TWO files, and
			// contention on the second one must refuse the whole operation with
			// nothing written rather than leave the first half on disk. See the
			// releaseBuildOrderApproval call below for what the second write is.
			//
			// Acquisition order is unchanged: propose never takes the claims
			// sentinel, and the project-wide order is claims -> lock-store ->
			// flag-store.
			releaseLock, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "build-order propose: %w", err)
			}
			defer releaseLock()

			if err := buildorder.WriteArtifact(artifact, path); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "build-order propose: %w", err)
			}

			// Release the module's standing approval, because the artifact it
			// vouched for no longer exists — the line above just overwrote it
			// with an unlocked recomputation. See releaseBuildOrderApproval.
			if err := releaseBuildOrderApproval(cfg, module, path); err != nil {
				return cmdResult{}, err
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

// buildOrderApprovalStands answers the one question that separates an APPROVED
// build order from an artifact that merely says `"locked": true`: does the lock
// ledger carry a standing record that vouches for these exact bytes?
//
// It exists because "locked" is a boolean inside the audited file, and this
// release's whole premise is that such a boolean proves nothing on its own.
// Two commands used to treat it as proof, and between them they wedged a project
// that no CLI verb could repair:
//
//   - "build-order lock" wrote the artifact and THEN wrote the ledger record, so
//     a crash — or a concurrent `check`/`claim lock` holding the lock-store
//     sentinel — left locked:true with no record. `check --validate` then
//     reported build-order-ledger-missing forever.
//   - the recovery from that state was neither of the two verbs the messages
//     named: `propose` refused (already_locked, from the flag) and `lock`
//     refused (already locked and not stale, from the same flag). There is no
//     unlock verb. The module was fixable only by hand-editing a dotfile, which
//     is precisely the move the release exists to make unnecessary.
//
// An artifact whose locked flag is unbacked is provably not an approved order,
// so re-proposing it discards nothing — and the same is true of a locked
// artifact whose record no longer matches its bytes, which is
// build-order-content-drift, whose own finding message tells the reader to
// re-propose and re-lock. Both are false here, which is what makes those
// documented recoveries actually run.
//
// The conservative direction on NO EVIDENCE is the opposite one: a lock store
// that cannot be read, or an artifact that cannot be hashed, returns true and
// keeps the refusal. Not knowing whether an approval stands must never be
// spendable as permission to overwrite one.
//
// It reads the store WITHOUT taking the sentinel, exactly as internal/check's
// read-only ledger gate does: Store.Save writes through a temp file and a
// rename, so a concurrent write is never observed half-applied.
func buildOrderApprovalStands(cfg *config.Config, module string) bool {
	// The RAW artifact, via LoadArtifact rather than Status: the signature in
	// the ledger is over the bytes WriteArtifact persisted, and Status
	// recomputes `stale` in memory before returning. Hashing Status's result
	// would report drift on every locked order whose claims had moved.
	artifact, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
	if err != nil || artifact == nil {
		// No artifact, or one nobody can read: there is nothing here for a
		// record to vouch for, and propose is the regeneration path.
		return false
	}
	hash, err := buildOrderSignature(artifact)
	if err != nil {
		return true
	}
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return true
	}
	record, ok := store.Record(lock.BuildOrderLedgerKey(module))
	if !ok || record.Subject != lock.SubjectBuildOrder || record.Released() {
		return false
	}
	return record.Hash == hash
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
//
// And neither is an artifact whose `locked` flag NOTHING BACKS. The refusal used
// to read the flag alone, which made every unbacked lock — a torn two-file
// write, a hand-set boolean, a deleted ledger key — permanently unrecoverable:
// propose refused because the file said locked, lock refused because the file
// said locked, and there is no unlock verb. See buildOrderApprovalStands: the
// question this asks is now "is there a standing approval for these exact
// bytes?", not "does the audited file claim to be approved?".
func approvedOrderWouldBeDiscarded(cfg *config.Config, module string, existing *buildorder.Artifact, err error) bool {
	if err != nil || existing == nil {
		return false
	}
	if !existing.Locked || existing.Stale {
		return false
	}
	return buildOrderApprovalStands(cfg, module)
}

// existingOrderDetail renders the state of the artifact already on disk as a
// dry-run precondition detail, so the preview says WHY it is (or is not)
// blocked rather than only that it is.
func existingOrderDetail(cfg *config.Config, module string, existing *buildorder.Artifact, err error) string {
	switch {
	case errors.Is(err, buildorder.ErrNotProposed):
		return "no build order proposed yet for this module"
	case err != nil:
		return fmt.Sprintf("the existing artifact could not be read (%v); propose would regenerate it", err)
	case existing == nil:
		return "no build order proposed yet for this module"
	case existing.Locked && existing.Stale:
		return fmt.Sprintf("locked=true stale=true (%d claim(s) moved) — re-proposing a stale order is the documented recovery", len(existing.StaleIDs))
	// The preview has to name the LEDGER, not just the flag, or a caller
	// staring at locked=true has no way to know why propose is nonetheless
	// allowed — and "locked=true, blocked:false" reads as a bug rather than as
	// the recovery it is.
	case existing.Locked && !buildOrderApprovalStands(cfg, module):
		return "locked=true, but no standing lock-ledger approval matches this artifact — it is not an approved order, so re-proposing discards nothing"
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
		// Selection is --module only; see `newBuildOrderProposeCmd`'s Args for
		// what a discarded positional costs and why this shipped as an
		// announced change rather than quietly.
		Args: cobra.NoArgs,
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
		// Selection is --module only; see `newBuildOrderProposeCmd`'s Args for
		// what a discarded positional costs and why this shipped as an
		// announced change rather than quietly.
		Args: cobra.NoArgs,
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
					// buildorder.Lock refuses a STALE order before it looks at
					// anything else: a bare relock would freeze an order whose
					// claims have moved. The preview did not evaluate that at
					// all, and "not_already_current" actively PASSED a stale
					// order (it reads !locked || stale), so the one artifact
					// state that always refuses previewed as "blocked: false".
					// The documented recovery is unchanged and is named here:
					// re-propose, then lock.
					dr.Require("build_order_not_stale", !artifact.Stale,
						fmt.Sprintf("stale=%v (%d claim(s) changed, added or removed: %v); re-run \"dossierx build-order propose --module %s\" first",
							artifact.Stale, len(artifact.StaleIDs), artifact.StaleIDs, module))
					// The unbacked-lock gate, previewed for the same reason the
					// two above are: the write path refuses on it FIRST, and a
					// preview that reported only "not_already_current" would
					// send the reader to the already_locked recovery ("there is
					// nothing to do") for the one state where there is a great
					// deal to do. See buildOrderApprovalStands.
					backed := !artifact.Locked || buildOrderApprovalStands(cfg, module)
					dr.Require("lock_flag_is_backed_by_an_approval", backed,
						fmt.Sprintf("locked=%v, standing lock-ledger approval matching this artifact=%v; an unbacked flag is not an approved order, so the way out is dossierx build-order propose --module %s and then lock the result",
							artifact.Locked, backed && artifact.Locked, module))
					dr.Require("not_already_current", !artifact.Locked || artifact.Stale,
						fmt.Sprintf("locked=%v stale=%v", artifact.Locked, artifact.Stale))
					// The hand-edit gate, which is the LAST thing buildorder.Lock
					// evaluates and used to be the only refusal of its the preview
					// could not see. An artifact reordered by hand between propose
					// and lock is never stale (staleness is a locked-artifact
					// concept and this one is unlocked) and is not already
					// current, so every precondition above it passed and the run
					// previewed blocked:false — then exited 1. The predicate is
					// buildorder's own, exported read-only as HandEditDivergence
					// precisely so the preview and Lock cannot answer differently.
					//
					// A non-nil error is Lock's OTHER hand-edit refusal (the
					// current claims have no valid order at all, so the artifact
					// cannot be verified as generated), so it blocks here too —
					// treating it as "unknown, proceed" would re-open the gap in
					// the one case where the engine understands the project least.
					divergence, deriveErr := buildorder.HandEditDivergence(artifact, claims, cfg)
					switch {
					case deriveErr != nil:
						dr.Require("build_order_is_generated", false,
							fmt.Sprintf("this build order cannot be re-derived from the current claims (%v), so it cannot be verified as generated; fix the claims and re-run \"dossierx build-order propose --module %s\"", deriveErr, module))
					case divergence != "":
						dr.Require("build_order_is_generated", false,
							fmt.Sprintf("%s; the artifact is generated, never hand-edited: re-run \"dossierx build-order propose --module %s\" and lock that order", divergence, module))
					default:
						dr.Require("build_order_is_generated", true,
							"the artifact on disk is exactly what a fresh propose computes")
					}
					dr.Propose("phases", phaseData(artifact))
				}
				dr.Effect("rewrites " + path + " with locked: true and a content-hash baseline").
					Effect("the order becomes the implementation sequence: it goes stale, rather than silently changing, when its claims move")
				dr.Propose("locked", true).Propose("reason", reason)
				// The pre-ledger project: this command records an approval
				// (recordBuildOrderApproval), and lock.CrossPreLedger refuses it
				// there, so the preview evaluates the same gate.
				preLedgerPrecondition(dr, cfg, claims)
				return dryRunResult(cmd, "build-order lock", dr), nil
			}

			if err := requireReason("build-order lock", reason); err != nil {
				return cmdResult{}, err
			}

			// The unbacked lock, refused BEFORE buildorder.Lock so the message
			// is about the state the caller is actually in.
			//
			// buildorder.Lock's own refusal for this shape is "already locked
			// and not stale", classified already_locked — whose documented
			// meaning is "there is nothing to do". That is exactly wrong here:
			// there is a great deal to do, `check --validate` is reporting
			// build-order-ledger-missing, and the two verbs the finding's own
			// message names both refused on the same unbacked boolean. Naming
			// the state and pointing at propose is what turns a wedged module
			// into a recoverable one. See buildOrderApprovalStands.
			//
			// THE PRE-LEDGER EXEMPTION, the same one internal/check's
			// buildOrderGate applies before it will emit
			// build-order-ledger-missing. A build order locked by a build that
			// had no ledger to record it in has no record and never could have,
			// so "nothing backs this flag" is not a statement about the artifact
			// here — it is a statement about the PROJECT, which
			// lock-ledger-pre-ledger names once and crossPreLedger refuses with
			// the whole ordered recovery a few lines below. Without the
			// exemption this fires first and sends the human to `build-order
			// propose` under a message accusing them of writing a flag outside
			// the approval path, and the refusal an agent branches on is
			// integrity_failed rather than pre_ledger_unadopted.
			//
			// It is deliberately NOT folded into buildOrderApprovalStands, whose
			// other caller (approvedOrderWouldBeDiscarded) needs the opposite
			// answer here: re-proposing a pre-ledger locked order is step 1 of
			// the crossing, and must stay allowed.
			if existing, statusErr := buildorder.Status(path, claims, cfg); statusErr == nil &&
				existing.Locked && !existing.Stale && !buildOrderApprovalStands(cfg, module) &&
				!projectIsPreLedger(cfg) {
				return cmdResult{}, cliout.Errorf(cliout.CodeIntegrityFailed,
					"build-order lock: module %q's build order (%s) already says \"locked\": true, but no standing lock-ledger approval matches it — the flag was written outside the approval path, or its record was lost between the artifact write and the ledger write. There is nothing here for this command to lock: an unbacked flag is not an approved order", module, path).
					WithHint(fmt.Sprintf("discard the unbacked artifact and approve a fresh one: dossierx build-order propose --module %s, then dossierx build-order lock --module %s --reason \"<the human's words>\"", module, module))
			}

			// The lock-store sentinel is taken BEFORE the artifact is written,
			// and held across both writes.
			//
			// This command writes TWO files — the artifact (buildorder.Lock) and
			// the ledger record — and it used to take the sentinel between them,
			// inside recordBuildOrderApproval. Contention there is not rare: an
			// ordinary concurrent `dossierx check` or `claim lock` holds this
			// exact file. The artifact was already on disk saying locked:true
			// when the record write failed, the command reported ok:true with a
			// warning, and `check --validate` refused every commit afterwards.
			// Acquiring first converts that whole class into a clean refusal
			// with nothing written: write_conflict is a documented, retryable
			// code, and a retry actually fixes it.
			//
			// It does not close the crash-in-between window (only internal/
			// could, by making Lock take the record as an input), so the ledger
			// write below is still FATAL rather than a warning, and the refusal
			// above is the recovery for the state a crash leaves.
			//
			// Acquisition order is unchanged and no deadlock is introduced: this
			// command never takes the claims sentinel, and the project-wide
			// order is claims -> lock-store -> flag-store.
			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "build-order lock: %w", err)
			}
			defer release()

			// THE PRE-LEDGER CROSSING, evaluated here — under the sentinel, and
			// before buildorder.Lock writes the artifact, so nothing is on disk
			// when it refuses.
			//
			// It is the twin of the gate lock.Lock applies to a claim, and it
			// has to exist separately because this command reaches the ledger
			// through lock.RecordBuildOrderApproval rather than through
			// lock.Lock. Without it, a build-order lock in a pre-ledger project
			// writes the first record into a store that says the ledger does not
			// exist, which lock.Store.LedgerDowngraded reads — correctly, by its
			// own rules — as tampering from then on.
			//
			// The sentinel this function takes over the lock store — the AcquireFileLock above — is the ONLY one this call needs, and
			// that is load-bearing: lock.CrossPreLedger takes the comment digest
			// store's lock as a leaf and never the claims sentinel, so the
			// acquisition order stated above is unchanged. Do not add one here.
			if store, storeErr := lock.LoadStore(storePath(cfg)); storeErr == nil {
				if err := crossPreLedger(cfg, store, claims, "build-order lock"); err != nil {
					return cmdResult{}, err
				}
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
			// This USED to be best-effort, returning a warning on a run that
			// still reported ok:true. It cannot be: `check --validate` refuses
			// the very next run on a build order with no record, so ok:true was
			// a false machine contract — the one thing an envelope must never
			// be. An agent reading ok and exit status concluded the order was
			// approved, and the project was wedged by the time anything noticed.
			if err := recordBuildOrderApproval(cfg, module, artifact, reason); err != nil {
				return cmdResult{}, err
			}
			return cmdResult{
				Data: buildOrderLockData{Module: module, Path: path, LockedAt: artifact.LockedAt, Reason: reason},
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
// into the lock ledger, returning nil on success or a CODED error the command
// fails on.
//
// The caller already holds the lock-store sentinel — deliberately, so that
// contention refuses the whole operation before the artifact is written rather
// than after (see the call site). This function therefore takes no sentinel of
// its own; taking it again here would self-deadlock.
//
// Every failure it can still reach leaves the artifact locked with no approval
// behind it, which is a state `check --validate` refuses. So each one is
// returned with the code whose documented recovery is the one that applies, and
// the message says in the same breath what IS on disk and what is not — a
// half-completed operation reported as a bare "write failed" tells the caller to
// retry a command whose first half has already happened. The recovery out of all
// of them is the same one buildOrderApprovalStands unblocked: re-propose, then
// re-lock.
//
// The artifact's signature is hashed HERE rather than in internal/lock because
// internal/buildorder imports internal/lock (for ContentHash), so computing it
// there would invert that edge into an import cycle. It hashes the artifact's
// canonical JSON — module, phases, per-claim entries, excluded set, the frozen
// hash baseline, and the LockedAt stamp — which is precisely the content
// buildorder.WriteArtifact just persisted, so a later hand edit of
// .build-order.<module>.json no longer matches its record.
func recordBuildOrderApproval(cfg *config.Config, module string, artifact *buildorder.Artifact, reason string) error {
	recovery := fmt.Sprintf("the artifact is on disk and locked, with nothing vouching for it; discard it and approve a fresh one: dossierx build-order propose --module %s, then dossierx build-order lock --module %s --reason \"<the human's words>\"", module, module)

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return cliout.Errorf(cliout.CodeIntegrityFailed,
			"build-order lock: %s's build order was written to disk as locked, but the lock ledger could not be read, so no approval was recorded: %v", module, err).
			WithHint(recovery)
	}

	hash, err := buildOrderSignature(artifact)
	if err != nil {
		return cliout.Errorf(cliout.CodeInternal,
			"build-order lock: %s's build order was written to disk as locked, but its content could not be hashed for the lock ledger, so no approval was recorded: %v", module, err).
			WithHint(recovery)
	}

	lock.RecordBuildOrderApproval(store, module, hash,
		lock.Approval{Actor: lock.DefaultActor(), Reason: reason})
	if err := store.Save(); err != nil {
		return cliout.Errorf(cliout.CodeWriteFailed,
			"build-order lock: %s's build order was written to disk as locked, but its lock-ledger record could not be saved: %v", module, err).
			WithHint(recovery)
	}
	return nil
}

// releaseBuildOrderApproval marks module's build-order ledger record released
// after "propose" has overwritten the approved artifact with a fresh unlocked
// one, returning nil on success or a CODED error the command fails on.
//
// It is the write that makes RuleBuildOrderLedgerOrphan's predicate honest. That
// rule fires on any unlocked artifact under a STANDING record, and the only
// legitimate way to reach that state is the window between a re-propose and the
// lock that follows it. Rather than have the gate guess which unlocked artifact
// is honest — the guess it used to make, by re-signing the file as if its locked
// flag were still true, which caught a lone flag flip and missed the strictly
// worse "flag flip plus a content edit" — propose simply writes down what is
// true: this approval no longer stands, because the bytes it vouched for are
// gone. The gate then needs no exception, and a hand-cleared "locked": false is
// a finding no matter what else was edited alongside it.
//
// The record is RELEASED, never deleted, for the reason
// lock.ReleaseBuildOrderApproval's own comment gives: the evidence that this
// module's order was once approved is what the reverse sweep needs, and deleting
// it would make removal quieter than editing.
//
// A module with no record at all — the ordinary first propose — is not an error.
// ReleaseBuildOrderApproval reports false, there is nothing to write, and the
// store is left untouched rather than created as a side effect of a read-mostly
// verb.
//
// The caller already holds the lock-store sentinel, exactly as
// recordBuildOrderApproval's caller does and for the same reason, so this takes
// no sentinel of its own; taking it again here would self-deadlock.
//
// Both failures below leave the artifact on disk unlocked with its record still
// standing, which is precisely the state the orphan rule refuses. That makes
// them FATAL rather than warnings — the same call this file already made for
// recordBuildOrderApproval, and for the same reason: an ok:true envelope over a
// project the next `check --validate` refuses is a false machine contract. The
// recovery named is restoring the artifact, because the approved order is what
// was just destroyed and version control is the only place it still exists.
func releaseBuildOrderApproval(cfg *config.Config, module, path string) error {
	recovery := fmt.Sprintf("the approved order at %s has been overwritten with an unlocked proposal while its approval still stands, which dossierx check reports as build-order-ledger-orphan; restore %s from version control to undo the propose, or re-run this command once the lock ledger is writable and then dossierx build-order lock --module %s --reason \"<the human's words>\"", path, path, module)

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return cliout.Errorf(cliout.CodeIntegrityFailed,
			"build-order propose: %s's build order was rewritten as an unlocked proposal, but the lock ledger could not be read, so its previous approval could not be released: %v", module, err).
			WithHint(recovery)
	}

	if !lock.ReleaseBuildOrderApproval(store, module,
		lock.Approval{Actor: lock.DefaultActor(), Reason: "superseded by dossierx build-order propose --module " + module}) {
		// No record to release: nothing was approved, so nothing is being
		// discarded and there is nothing to persist.
		return nil
	}

	if err := store.Save(); err != nil {
		return cliout.Errorf(cliout.CodeWriteFailed,
			"build-order propose: %s's build order was rewritten as an unlocked proposal, but the release of its previous approval could not be saved to the lock ledger: %v", module, err).
			WithHint(recovery)
	}
	return nil
}

// buildOrderRecordStands reports whether module carries a STANDING (unreleased)
// build-order record right now, which is the one question the propose dry run
// needs in order to say whether the real run would release an approval.
//
// It is deliberately narrower than buildOrderApprovalStands: that function asks
// whether an approval vouches for the artifact's CURRENT bytes (and answers true
// on no evidence, so that not knowing can never be spent as permission to
// overwrite). This one asks only whether a record is there and unreleased, and
// answers false on no evidence — an unreadable store must not make the preview
// announce a side effect the real run may not perform. The real run's own
// correctness does not rest on this; it re-reads the store under the sentinel.
func buildOrderRecordStands(cfg *config.Config, module string) bool {
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil || store == nil {
		return false
	}
	record, ok := store.Record(lock.BuildOrderLedgerKey(module))
	return ok && record.Subject == lock.SubjectBuildOrder && !record.Released()
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

// lockedBuildOrders counts this project's LOCKED build-order artifacts — the
// build-order half of the count lock.CrossPreLedger refuses on.
//
// It exists because a locked build order is a locked artifact in its own right,
// and "a locked build order implies at least one locked claim" is FALSE outside
// propose time: buildorder.Propose refuses a module that is not fully locked, but
// nothing re-checks it afterwards — `claim unlock` never touches the artifact and
// internal/buildorder never clears Locked on unlock. So a project can hold a
// locked order and zero locked claims, and without this term both write paths
// would cross it silently onto the ledger with an unapproved order still in place.
//
// It cannot live in internal/lock: internal/buildorder imports internal/lock, so
// lock can never read an artifact back. That is the same constraint internal/
// check records for the build-order gate rules.
//
// It iterates the distinct modules of the LOADED CLAIMS rather than cfg.Modules
// so it asks about exactly the modules this run has evidence for, and a module
// with no artifact (buildorder.ErrNotProposed, or any other load failure) counts
// as zero — an artifact nothing can read is not evidence that something is
// locked, and the ledger gate reports it under its own rule.
func lockedBuildOrders(cfg *config.Config, claims []model.Claim) int {
	if cfg == nil {
		return 0
	}
	seen := map[string]bool{}
	locked := 0
	for _, c := range claims {
		if c.Module == "" || seen[c.Module] {
			continue
		}
		seen[c.Module] = true
		artifact, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, c.Module))
		if err != nil || artifact == nil || !artifact.Locked {
			continue
		}
		locked++
	}
	return locked
}

// projectIsPreLedger reports whether this project's lock store predates the lock
// ledger with nothing contradicting that — lock.Store.PreLedgerUnadopted, read
// off the store on disk.
//
// It exists so a refusal that assumes a ledger exists can stand aside for the
// one project where it never did. An unreadable store answers false: that is a
// different condition with a different recovery (restore from version control),
// already named by its own finding, and a guard must not manufacture a second
// diagnosis out of it.
func projectIsPreLedger(cfg *config.Config) bool {
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return false
	}
	return store.PreLedgerUnadopted(digestStorePresent(cfg))
}
