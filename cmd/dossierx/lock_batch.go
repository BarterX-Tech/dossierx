// lock_batch.go implements "dossierx claim lock <id> <id> ..." — the batch
// form of claim lock, for two or more ids in one invocation.
//
// WHY THIS EXISTS.
//
// evaluateLockGates (and, on the write path, internal/lock.Lock's own internal
// lint gate) lint the WHOLE PROJECT with exactly ONE claim flipped to its
// about-to-be-locked form. That is the right question for a single lock —
// rest-on-locked and roll-up are properties of the claim once it is locked,
// so the whole corpus has to be linted to find them (see evaluateLockGates'
// own comment) — but it means every error-severity finding ANYWHERE in the
// project, related to this claim or not, refuses the lock. Ordinarily that
// is invisible, because a project that passes `check` has none. It stops
// being invisible the moment a module is unlocked in bulk: internal/lint's
// rest-on-locked rule fires on every claim that RESTS ON a still-draft
// sibling, so N drafts sitting in one module produce, for every one of the N
// attempted single-claim locks, findings about the OTHER N-1 — in every
// order. There is no legal sequence: the first lock attempted in any order
// finds its siblings still draft and refuses; unlocking one of the
// "blocking" siblings to clear the finding does not help, because nothing
// was ever locked to begin with, and simply moves which member looks like
// the blocker. This is the identical shape internal/lint/roll_up.go's file
// comment already documents for exactly one lint (a project-wide
// error-severity roll-up "deadlocked every ordinary module"); this file is
// the general fix, for every lint that keys off a claim's own about-to-be-
// locked status, not only that one.
//
// THE FIX: lint the BATCH's own about-to-be-locked form once — every
// requested id flipped to locked in the SAME candidate corpus, in ONE
// lint.RunAll call — and then SCOPE which of the resulting findings are
// allowed to block. A finding between two claims OUTSIDE the requested set is
// a pre-existing condition this batch did not create and cannot fix by
// omitting members from itself; counting it would only move the hostage
// effect up one level, from "no single lock succeeds" to "no batch
// succeeds". A finding that DOES involve a requested claim is exactly the
// case the single-claim gate exists to catch, and scoping preserves it
// exactly: if a requested claim rests_on ANOTHER requested claim, the batch
// SUCCEEDS — both flip together in the one candidate corpus, so the
// dependency is already locked by the time rest-on-locked runs, which is
// the deadlock actually breaking. If a requested claim rests_on a claim
// OUTSIDE the requested set that is still draft, the batch still REFUSES —
// that claim was never flipped, so the finding still fires, scoped to the
// requested claim resting on it. No locked claim can ever end up resting on
// a draft; the promise the gate exists to keep is intact at batch scope,
// not merely at single-claim scope.
package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// batchLockOffender is one requested claim that fails a PER-CLAIM gate — the
// gates that are properties of one claim alone (already locked, a standing
// ledger record, integrity, hub gating, an open comment thread) rather than of
// the lint pass over the batch's candidate corpus. See runBatchLock.
type batchLockOffender struct {
	ClaimID string `json:"claim_id"`
	Gate    string `json:"gate"`
	Detail  string `json:"detail"`
}

// batchLockRefusedData is the payload a REFUSED batch "dossierx claim lock"
// carries — the batch-scoped twin of lockRefusedData. It reports EVERY
// offender and every scoped lint finding in one shot, mirroring
// lockRefusedData's own reasoning: a caller that has to re-run to discover the
// second blocker one at a time has been sent around a loop this envelope can
// close in the first response.
type batchLockRefusedData struct {
	RequestedIDs []string            `json:"requested_ids"`
	Offenders    []batchLockOffender `json:"offenders"`
	LintErrors   int                 `json:"lint_errors"`
	LintFindings []lintFindingData   `json:"lint_findings"`
}

// batchLockData is a successful batch lock's machine payload: every claim
// that moved, in the order requested, each carrying the same fields a single
// "dossierx claim lock" reports for itself.
type batchLockData struct {
	ClaimIDs []string   `json:"claim_ids"`
	Reason   string     `json:"reason"`
	Locked   []lockData `json:"locked"`
}

// findingBlocksBatch reports whether lint finding f should block a batch lock
// over requested — the scoping rule runBatchLock's file comment argues for.
//
// lint.Finding (internal/lint/lint.go) carries no structured field naming the
// OTHER claim in a two-claim relationship — no "DependsOn" or "Target", only
// LintName, ClaimID, Message and Severity. rest-on-locked's unmet dependency,
// a cycle's other members, a governed-cycle's other members: every one of
// them is named ONLY in Message, in prose ("locked claim rests_on X which is
// not locked", "rests_on cycle detected: a -> b -> c -> a"). So the "names a
// requested claim as the unmet dependency" half of the scoping rule falls
// back to a substring search over Message — there is no structured field to
// prefer it over. This is exact for rest-on-locked, cycle, mixed-cycle,
// governed-cycle and dangling, because each of those writes the OTHER
// claim's id verbatim into its Message; a lint added later that names a
// dependency only in free prose (no id substring) would silently stop being
// caught by this fallback, the same way a human reading that finding would
// have no id to act on either.
//
// For rest-on-locked specifically the fallback is provably redundant with
// the ClaimID check: the only claim rest-on-locked ever attaches a finding
// to is the one doing the resting (the "locked claim" in its own message),
// so f.ClaimID already names the requested claim whenever this finding
// exists at all. The fallback earns its keep on graph-shaped lints (cycle,
// mixed-cycle, governed-cycle) where every member of a cycle gets its OWN
// finding with its OWN ClaimID — a cycle finding on a non-requested member
// still names every other member of the same cycle in its Message, including
// a requested one, and that cycle is exactly as real a problem for the
// requested claim as one rest-on-locked finding would be.
func findingBlocksBatch(f lint.Finding, requested map[string]bool) bool {
	if f.Severity == lint.SeverityWarning {
		// The one warning this codebase escalates back to a blocker — see
		// evaluateLockGates and isOwnRollUp. At batch scope it blocks only the
		// banner actually in the batch; a roll-up warning about a banner
		// outside the requested set is exactly the kind of pre-existing,
		// unrelated finding this whole file exists to stop hostage-taking a
		// batch.
		return f.LintName == rollUpLintName && requested[f.ClaimID]
	}
	if requested[f.ClaimID] {
		return true
	}
	for id := range requested {
		if strings.Contains(f.Message, id) {
			return true
		}
	}
	return false
}

// runBatchLock is "dossierx claim lock <id> <id> ..."'s write path for two or
// more ids in one invocation. See this file's WHY comment for the deadlock it
// exists to break and the soundness argument for scoping the lint gate the
// way it does.
//
// Structurally it is the single-lock write path (newLockCmd's non-dry-run
// branch) with two differences: the lint gate is evaluated ONCE, batch-scoped
// (see findingBlocksBatch), and every OTHER gate — already locked, a standing
// ledger record, integrity, hub gating, open comment threads — is checked for
// EVERY requested id BEFORE any of them writes, rather than for one id inline
// with its own write. Nothing is written until every gate has passed for
// every id: a batch that fails ANY gate on ANY member writes nothing at all,
// and the refusal names every offender it found, not just the first — the
// same reasoning lockRefusedData documents for the single-claim case, at
// batch scope.
//
// It reuses internal/lock's own exported write primitives for the actual
// mutation — lock.RefreshBaseline and lock.RecordApproval are the identical
// calls lock.Lock makes internally once ITS gates pass (see lock.go's Lock,
// the block after its open-thread check) — rather than calling lock.Lock
// itself. It cannot call lock.Lock per id: Lock's own internal lint gate
// counts every unscoped error-severity finding project-wide, which is
// precisely the check this file exists to replace with a scoped one: calling
// Lock claim-by-claim would re-run the single-claim, project-wide lint this
// whole batch path is built to avoid, and refuse on the very deadlock it is
// meant to break.
func runBatchLock(cmd *cobra.Command, ids []string, reason string) (cmdResult, error) {
	// Duplicate ids collapse to one write and one ledger record. "lock a b a"
	// is "lock a b" with a repeated argument, not three transitions — treating
	// it as three would make the second occurrence of "a" fail already_locked
	// against the first occurrence's own write, mid-batch, which is not a
	// meaningful refusal of anything the caller asked for.
	seen := make(map[string]bool, len(ids))
	uniqueIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		uniqueIDs = append(uniqueIDs, id)
	}
	ids = uniqueIDs

	requested := make(map[string]bool, len(ids))
	for _, id := range ids {
		requested[id] = true
	}

	cfg, err := loadConfig()
	if err != nil {
		return cmdResult{}, err
	}

	// The gitignore guard, before the claims sentinel — the batch has no
	// dry-run twin (see newLockCmd's refusal of --dry-run with two ids), so
	// this refusal is the only place the state is reported for it.
	gitignoreWarnings, err := refuseIfStoresGitignored(cfg, "lock")
	if err != nil {
		return cmdResult{}, err
	}

	// Same write discipline as the single-lock path: the project-wide claims
	// sentinel first, loaded inside it, before the lock-store sentinel below —
	// see newLockCmd's identical comment for why the order is deadlock-free.
	releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "lock: %w", err)
	}
	defer releaseClaims()

	claims, err := loadClaims(cfg)
	if err != nil {
		return cmdResult{}, err
	}

	// Every requested id must exist, and every token must be captured, before
	// any gate runs — an unknown id refuses the WHOLE batch (nothing about the
	// other, perfectly valid ids is touched), matching the single lock's own
	// claim-not-found refusal for the id that fails.
	requestedClaims := make(map[string]model.Claim, len(ids))
	tokens := make(map[string]loader.ClaimFileToken, len(ids))
	for _, id := range ids {
		claim, ok := loader.FindByID(claims, id)
		if !ok {
			return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "lock: claim %q not found: %w", id, errClaimNotFound)
		}
		requestedClaims[id] = claim
		token, err := loader.CaptureClaimFileToken(claim.SourcePath)
		if err != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
		}
		tokens[id] = token
	}

	release, err := lock.AcquireFileLock(storePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "lock: %w", err)
	}
	defer release()

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
	}

	changed, adopted := prepareStore(cfg, store, claims)
	if changed {
		if err := store.Save(); err != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
		}
	}

	// The pre-ledger project gate is a property of the PROJECT, not of any one
	// requested claim, so it runs once here — exactly as it does in the
	// single-lock path — rather than once per id.
	if err := crossPreLedger(cfg, store, claims, "lock"); err != nil {
		return cmdResult{Warnings: adoptionWarnings(adopted)}, err
	}

	// PER-CLAIM GATES, CHECKED FOR EVERY REQUESTED ID BEFORE ANYTHING WRITES.
	// Each mirrors the corresponding check in internal/lock.Lock, in Lock's own
	// order (already locked -> standing ledger record -> ledger record deleted
	// -> comment digest unrecorded -> hub gating -> open comment threads); the
	// batch-scoped lint gate runs separately, after this loop, because it is
	// evaluated once for the whole batch rather than once per claim.
	var offenders []batchLockOffender
	for _, id := range ids {
		claim := requestedClaims[id]

		if claim.Status == model.StatusLocked {
			offenders = append(offenders, batchLockOffender{
				ClaimID: id,
				Gate:    string(cliout.CodeAlreadyLocked),
				Detail:  fmt.Sprintf("already locked; run: dossierx claim unlock %s --reason \"...\" first, then lock again", id),
			})
			continue
		}

		if rec, standing, _ := standingLedgerRecord(store, claim); standing {
			offenders = append(offenders, batchLockOffender{
				ClaimID: id,
				Gate:    string(cliout.CodeAlreadyLocked),
				Detail: fmt.Sprintf("status: draft, but the lock ledger still holds a STANDING approval for it from %s (%q), unreleased — restore the approved content from version control first, or dossierx claim unlock %s --reason \"...\" if the change is wanted",
					rec.At, rec.Reason, id),
			})
			continue
		}

		if store.LedgerRecordDeleted(claim) {
			offenders = append(offenders, batchLockOffender{
				ClaimID: id,
				Gate:    string(cliout.CodeIntegrityFailed),
				Detail:  "this claim's lock-ledger record was deleted — restore " + config.LockStoreDisplayPath + " from version control; do not unlock-and-relock",
			})
			continue
		}

		if store.CommentDigestUnrecorded(claim) {
			offenders = append(offenders, batchLockOffender{
				ClaimID: id,
				Gate:    string(cliout.CodeIntegrityFailed),
				Detail:  fmt.Sprintf("%d comment thread(s) with no entry in the comment digest store — restore %s from version control", len(claim.Comments), config.CommentDigestDisplayPath),
			})
			continue
		}

		// Hub (doctrine) gating — the same predicate evaluateLockGates
		// evaluates inline for a single claim, since internal/lock's own
		// checkHubGating is unexported and this package already keeps its own
		// copy in step for that reason.
		if cfg != nil && cfg.HubGatingEnabled() {
			deps := append(append([]string(nil), claim.Mirrors...), claim.RestsOn...)
			var depBlock string
			for _, dep := range deps {
				depClaim, ok := loader.FindByID(claims, dep)
				if !ok {
					continue
				}
				if depClaim.Facet == cfg.DoctrineFacet && depClaim.Status != model.StatusLocked {
					depBlock = dep
					break
				}
			}
			if depBlock != "" {
				offenders = append(offenders, batchLockOffender{
					ClaimID: id,
					Gate:    string(cliout.CodeDependencyNotLocked),
					Detail:  fmt.Sprintf("dependency %q is in doctrine facet %q and is not yet locked", depBlock, cfg.DoctrineFacet),
				})
				continue
			}
		}

		if open := claim.OpenThreadIDs(); len(open) > 0 {
			offenders = append(offenders, batchLockOffender{
				ClaimID: id,
				Gate:    string(cliout.CodeUnresolvedComments),
				Detail:  fmt.Sprintf("%d unresolved thread(s) %v — the human resolves them in the viewer (\"dossierx serve\"); an agent may reply but never resolve", len(open), open),
			})
			continue
		}
	}

	// THE BATCH-SCOPED LINT GATE. ONE candidate corpus, every requested claim
	// flipped to its about-to-be-locked form (Status=locked, ReviewPending=
	// false) in the SAME copy, ONE lint.RunAll call — see this file's header
	// comment for why this is the fix and findingBlocksBatch for the scoping
	// rule that keeps it sound.
	lintClaims := make([]model.Claim, len(claims))
	copy(lintClaims, claims)
	for i := range lintClaims {
		if requested[lintClaims[i].ID] {
			lintClaims[i].Status = model.StatusLocked
			lintClaims[i].ReviewPending = false
		}
	}
	var lintFindings []lint.Finding
	for _, f := range lint.RunAll(lintClaims, cfg) {
		if findingBlocksBatch(f, requested) {
			lintFindings = append(lintFindings, f)
		}
	}

	if len(offenders) > 0 || len(lintFindings) > 0 {
		g := lockGate{LintErrors: len(lintFindings), LintFindings: lintFindings}
		// The top-level error.code is the FIRST offender's gate, in the same
		// per-claim gate order checked above, unless nothing but lint blocked —
		// mirroring lockGate.code()'s "first gate that would refuse" rule at
		// batch scope. The full detail, every offender and every scoped
		// finding, still rides in data and error.details either way, so the
		// single code at the top never hides the rest.
		code := cliout.CodeLintFailed
		if len(offenders) > 0 {
			code = cliout.Code(offenders[0].Gate)
		}
		data := batchLockRefusedData{
			RequestedIDs: ids,
			Offenders:    offenders,
			LintErrors:   g.LintErrors,
			LintFindings: g.lockLintFindingData(),
		}
		return cmdResult{
				Warnings: append(adoptionWarnings(adopted), gitignoreWarnings...),
				Data:     data,
			}, cliout.Errorf(code,
				"lock: batch refused, %d of %d claim(s) failed a per-claim gate, %d error-level lint finding(s) scoped to the requested set outstanding — nothing was written",
				len(offenders), len(ids), len(lintFindings)).
				WithDetails(map[string]any{
					"requested_ids": ids,
					"offenders":     offenders,
					"lint_errors":   g.LintErrors,
					"lint_findings": g.lockLintFindingData(),
				}).
				WithHint("re-run with the offending id(s) removed from the argument list; `dossierx claim lock <id> --dry-run` on any single offender shows its own gate in detail")
	}

	// EVERY GATE PASSED FOR EVERY REQUESTED ID. Nothing below this line can
	// refuse — every check that could have is already behind it — so the batch
	// writes for real, one claim at a time, all under the two sentinels already
	// held for the whole call. That is what makes "all or none" true in
	// practice and not only in the gate-ordering on paper: no other writer
	// holding the same sentinels can be touching these claim files or this
	// store concurrently, so a mid-loop write failure here would mean the
	// token captured moments ago under the same sentinel no longer matches —
	// the same class of failure the single-lock path already treats as
	// write_failed rather than as a gate refusal.
	locked := make([]lockData, 0, len(ids))
	ap := lock.Approval{Actor: lock.DefaultActor(), Reason: reason}
	for _, id := range ids {
		claim := requestedClaims[id]
		from := string(claim.Status)
		claim.Status = model.StatusLocked
		claim.ReviewPending = false

		// The exact two calls lock.Lock makes internally once ITS gates pass
		// (lock.go, the block right after the open-thread check): baseline
		// recording plus the LockedAt stamp, then the ledger record (which
		// also records the comment digest beside it). Reusing them rather than
		// re-deriving their effect keeps this file from ever re-implementing a
		// ledger write.
		lock.RefreshBaseline(claim, claims, store)
		lock.RecordApproval(store, claim, ap)

		if err := loader.SaveClaimIfUnchanged(claim, tokens[id]); err != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
		}

		locked = append(locked, lockData{
			ClaimID:  id,
			From:     from,
			To:       string(model.StatusLocked),
			Reason:   reason,
			LockedAt: store.LockedAt[id],
		})
	}

	if err := store.Save(); err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
	}

	return cmdResult{
		Warnings: append(adoptionWarnings(adopted), gitignoreWarnings...),
		Data:     batchLockData{ClaimIDs: ids, Reason: reason, Locked: locked},
		Text: func() {
			fmt.Fprintf(cmd.OutOrStdout(), "lock: %d claim(s) now locked: %s\n", len(locked), strings.Join(ids, ", "))
		},
	}, nil
}
