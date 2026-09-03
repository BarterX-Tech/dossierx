package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// policyLockPreviewData is returned for both one-claim and group dry runs.
// Snapshot is an opaque candidate-content token a later write may supply with
// --proposal; a mismatch refuses rather than approving unseen dependency text.
type policyLockPreviewData struct {
	Would         string                `json:"would"`
	Blocked       bool                  `json:"blocked"`
	Snapshot      string                `json:"snapshot"`
	Evaluation    lock.SetEvaluation    `json:"evaluation"`
	From          string                `json:"from"`
	To            string                `json:"to"`
	Preconditions []cliout.Precondition `json:"preconditions"`
	SideEffects   []string              `json:"side_effects"`
	Missing       []string              `json:"missing"`
}

type policyLockData struct {
	ClaimIDs   []string           `json:"claim_ids"`
	Reason     string             `json:"reason"`
	Snapshot   string             `json:"snapshot"`
	Evaluation lock.SetEvaluation `json:"evaluation"`
	ClaimID    string             `json:"claim_id"`
	From       string             `json:"from"`
	To         string             `json:"to"`
	LockedAt   string             `json:"locked_at"`
}

type policyMigrationData struct {
	From       lock.PolicyVersion `json:"from"`
	To         lock.PolicyVersion `json:"to"`
	Reason     string             `json:"reason"`
	MigratedAt string             `json:"migrated_at,omitempty"`
}

// newLockPolicyMigrateCmd is the explicit adoption boundary for projects whose
// existing lock store predates local approval. It changes no claim, approval,
// baseline, receipt, or pending state.
func newLockPolicyMigrateCmd() *cobra.Command {
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate-lock-policy",
		Short: "Adopt the local-approval lock policy without rewriting existing approvals",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, _ []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock policy migration: %w", err)
			}
			if store.FileExists() && store.OnDiskVersion() < 2 {
				return cmdResult{}, cliout.Errorf(cliout.CodePreLedgerUnadopted, "lock policy migration: project predates the lock ledger; complete the ledger crossing before changing approval policy")
			}
			data := policyMigrationData{From: store.PolicyVersion, To: lock.PolicyLocalApprovalV1, Reason: reason, MigratedAt: store.PolicyMigratedAt}
			if dryRun {
				return cmdResult{Data: data, Text: func() {
					fmt.Fprintln(cmd.OutOrStdout(), "lock policy migration preview: existing approvals and baselines stay unchanged")
				}}, nil
			}
			if err := requireReason("claim migrate-lock-policy", reason); err != nil {
				return cmdResult{}, err
			}
			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "lock policy migration: %w", err)
			}
			defer release()
			store, err = lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock policy migration: %w", err)
			}
			if store.FileExists() && store.OnDiskVersion() < 2 {
				return cmdResult{}, cliout.Errorf(cliout.CodePreLedgerUnadopted, "lock policy migration: project predates the lock ledger; complete the ledger crossing before changing approval policy")
			}
			store.AdoptLocalApproval(reason)
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock policy migration: %w", err)
			}
			data.MigratedAt = store.PolicyMigratedAt
			return cmdResult{Data: data, Text: func() {
				fmt.Fprintln(cmd.OutOrStdout(), "lock policy migration: local approval adopted; existing approvals were preserved")
			}}, nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "why the human adopts this policy (required unless --dry-run)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report the migration without writing")
	return cmd
}

func policyEnabledForConfig(cfg interface{ Dir() string }) bool {
	store, err := lock.LoadStore(filepath.Join(cfg.Dir(), lock.StoreFileName))
	return err == nil && store.LocalApprovalEnabled()
}

func previewPolicyLock(cmd *cobra.Command, ids []string, reason string, conflicts []lock.SemanticConflict) (cmdResult, error) {
	cfg, claims, err := loadConfigAndClaims()
	if err != nil {
		return cmdResult{}, err
	}
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock preview: %w", err)
	}
	evaluation := lock.EvaluateSetWithSemanticConflicts(claims, ids, cfg, store, conflicts)
	data := policyLockPreviewData{
		Would:      "lock " + strings.Join(evaluation.RequestedIDs, ", "),
		Blocked:    !evaluation.Allowed() || strings.TrimSpace(reason) == "",
		Snapshot:   policySnapshot(claims, evaluation.RequestedIDs),
		Evaluation: evaluation,
		From:       "draft", To: "locked",
		Preconditions: []cliout.Precondition{{Name: "claim_is_draft", OK: evaluation.Allowed()}, {Name: "lint_clean", OK: evaluation.Allowed()}, {Name: "no_open_comment_threads", OK: evaluation.Allowed()}},
		SideEffects:   []string{"write requested claim approvals and lock ledger records"},
	}
	if strings.TrimSpace(reason) == "" {
		data.Missing = []string{"--reason"}
	}
	return cmdResult{Data: data, Text: func() {
		fmt.Fprintf(cmd.OutOrStdout(), "lock preview: %s\n", data.Would)
		for _, verdict := range data.Evaluation.Verdicts {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s local=%t conditions=%d refusals=%v\n", verdict.ClaimID, verdict.LocalAdmissible, len(verdict.Conditions), verdict.Refusals)
		}
	}}, nil
}

// runPolicySetLock is the sole local-approval write path for both a singleton
// and a group. It evaluates the final candidate before writing anything, then
// writes under the claims/store sentinels and restores captured files if a later
// write fails. It never accepts a supplied snapshot that no longer matches.
func runPolicySetLock(cmd *cobra.Command, ids []string, reason, proposal string, conflicts []lock.SemanticConflict) (cmdResult, error) {
	cfg, err := loadConfig()
	if err != nil {
		return cmdResult{}, err
	}
	releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "lock: %w", err)
	}
	defer releaseClaims()
	claims, err := loadClaims(cfg)
	if err != nil {
		return cmdResult{}, err
	}
	releaseStore, err := lock.AcquireFileLock(storePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "lock: %w", err)
	}
	defer releaseStore()
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", err)
	}
	if err := crossPreLedger(cfg, store, claims, "claim lock"); err != nil {
		return cmdResult{}, err
	}
	evaluation := lock.EvaluateSetWithSemanticConflicts(claims, ids, cfg, store, conflicts)
	snapshot := policySnapshot(claims, evaluation.RequestedIDs)
	if proposal != "" && proposal != snapshot {
		return cmdResult{Data: policyLockData{ClaimIDs: evaluation.RequestedIDs, Reason: reason, Snapshot: snapshot, Evaluation: evaluation}},
			cliout.Errorf(cliout.CodeClaimFileChanged, "lock: preview snapshot is stale; re-run --dry-run and review the current dependency content")
	}
	if !evaluation.Allowed() {
		return cmdResult{Data: policyLockData{ClaimIDs: evaluation.RequestedIDs, Reason: reason, Snapshot: snapshot, Evaluation: evaluation}},
			policyRefusalError(evaluation)
	}

	requested := map[string]bool{}
	for _, id := range evaluation.RequestedIDs {
		requested[id] = true
	}
	originals := map[string]model.Claim{}
	tokens := map[string]loader.ClaimFileToken{}
	for _, claim := range claims {
		if !requested[claim.ID] {
			continue
		}
		originals[claim.ID] = claim
		token, tokenErr := loader.CaptureClaimFileToken(claim.SourcePath)
		if tokenErr != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w", tokenErr)
		}
		tokens[claim.ID] = token
	}
	if len(originals) != len(requested) {
		return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "lock: requested claim disappeared before write")
	}

	storeRaw, storeExisted := readOptional(storePath(cfg))
	digestPath := digest.StorePathBeside(storePath(cfg))
	digestRaw, digestExisted := readOptional(digestPath)
	finalClaims := make([]model.Claim, len(claims))
	copy(finalClaims, claims)
	for i := range finalClaims {
		if requested[finalClaims[i].ID] {
			finalClaims[i].Status = model.StatusLocked
			finalClaims[i].ReviewPending = false
		}
	}

	written := []string{}
	rollback := func() error {
		var failures []string
		for _, id := range written {
			current, captureErr := loader.CaptureClaimFileToken(originals[id].SourcePath)
			if captureErr != nil || loader.SaveClaimIfUnchanged(originals[id], current) != nil {
				failures = append(failures, id)
			}
		}
		if err := restoreOptional(storePath(cfg), storeRaw, storeExisted); err != nil {
			failures = append(failures, lock.StoreFileName)
		}
		if err := restoreOptional(digestPath, digestRaw, digestExisted); err != nil {
			failures = append(failures, filepath.Base(digestPath))
		}
		if len(failures) > 0 {
			return fmt.Errorf("recovery incomplete for %s", strings.Join(failures, ", "))
		}
		return nil
	}
	for _, claim := range finalClaims {
		if !requested[claim.ID] {
			continue
		}
		if err := loader.SaveClaimIfUnchanged(claim, tokens[claim.ID]); err != nil {
			if recoverErr := rollback(); recoverErr != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: write failed and %v", recoverErr)
			}
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: %w; prior state restored", err)
		}
		written = append(written, claim.ID)
		if err := policyWriteFault("after_claim_write", claim.ID); err != nil {
			if recoverErr := rollback(); recoverErr != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: injected write failure and %v", recoverErr)
			}
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: injected write failure; prior state restored: %w", err)
		}
	}
	approval := lock.Approval{Actor: lock.DefaultActor(), Reason: reason}
	for _, claim := range finalClaims {
		if !requested[claim.ID] {
			continue
		}
		lock.RefreshBaseline(claim, finalClaims, store)
		lock.RecordApproval(store, claim, approval)
	}
	if err := policyWriteFault("before_store_save", ""); err != nil {
		if recoverErr := rollback(); recoverErr != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: injected store failure and %v", recoverErr)
		}
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: injected store failure; prior state restored: %w", err)
	}
	if err := store.Save(); err != nil {
		if recoverErr := rollback(); recoverErr != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: store write failed and %v", recoverErr)
		}
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: store write failed; prior state restored: %w", err)
	}
	lockedAt := ""
	if len(evaluation.RequestedIDs) == 1 {
		lockedAt = store.LockedAt[evaluation.RequestedIDs[0]]
	}
	return cmdResult{Data: policyLockData{ClaimIDs: evaluation.RequestedIDs, Reason: reason, Snapshot: snapshot, Evaluation: evaluation, ClaimID: firstID(evaluation.RequestedIDs), From: "draft", To: "locked", LockedAt: lockedAt}, Text: func() {
		fmt.Fprintf(cmd.OutOrStdout(), "lock: %d claim(s) locally approved: %s\n", len(evaluation.RequestedIDs), strings.Join(evaluation.RequestedIDs, ", "))
	}}, nil
}

func firstID(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// policyRefusalError retains the established envelope families for the
// highest-priority refusal while the data payload keeps every member verdict.
// A group never hides the open-thread or not-found recovery behind a generic
// lint error merely because another member was also evaluated.
func policyRefusalError(evaluation lock.SetEvaluation) error {
	for _, verdict := range evaluation.Verdicts {
		for _, refusal := range verdict.Refusals {
			details := map[string]any{"claim_id": verdict.ClaimID}
			if len(verdict.OpenThreads) > 0 {
				details["open_threads"] = verdict.OpenThreads
			}
			if len(verdict.LintFindings) > 0 {
				details["lint_findings"] = verdict.LintFindings
			}
			switch {
			case refusal == "claim_not_found":
				return cliout.Errorf(cliout.CodeClaimNotFound, "lock: claim %q not found", verdict.ClaimID).WithDetails(details)
			case refusal == "unresolved_comments":
				return cliout.Errorf(cliout.CodeUnresolvedComments, "lock: claim %q has unresolved comment threads", verdict.ClaimID).WithDetails(details)
			case strings.HasPrefix(refusal, "ledger_record_deleted"), strings.HasPrefix(refusal, "comment_digest_unrecorded"), strings.HasPrefix(refusal, "standing_ledger_record"):
				return cliout.Errorf(cliout.CodeIntegrityFailed, "lock: claim %q has an approval-record integrity refusal", verdict.ClaimID).WithDetails(details)
			case refusal == "already_locked":
				return cliout.Errorf(cliout.CodeAlreadyLocked, "lock: claim %q is already locked", verdict.ClaimID).WithDetails(details)
			case strings.HasPrefix(refusal, "semantic_contradiction_requires_human_review"):
				return cliout.Errorf(cliout.CodeReviewPending, "lock: claim %q has a semantic contradiction requiring human review", verdict.ClaimID).WithDetails(details)
			}
		}
	}
	findings := []lint.Finding{}
	for _, verdict := range evaluation.Verdicts {
		findings = append(findings, verdict.LintFindings...)
	}
	return cliout.Errorf(cliout.CodeLintFailed, "lock: one or more requested claims are refused; no approval was written").WithDetails(map[string]any{"verdicts": evaluation.Verdicts, "lint_findings": findings})
}

// policyWriteFault is a deterministic, opt-in recovery seam for disposable
// integration tests. `DOSSIERX_LOCK_FAULT=after_claim_write[:claim-id]` fails
// after a selected claim write; `before_store_save` fails after all claim
// writes and approval preparation. It is disabled unless explicitly set and
// always exercises the ordinary rollback path rather than a test-only write.
func policyWriteFault(stage, claimID string) error {
	wanted := strings.TrimSpace(os.Getenv("DOSSIERX_LOCK_FAULT"))
	if wanted == "" {
		return nil
	}
	parts := strings.SplitN(wanted, ":", 2)
	if parts[0] != stage {
		return nil
	}
	if len(parts) == 2 && parts[1] != "" && parts[1] != claimID {
		return nil
	}
	return fmt.Errorf("fault seam %s", wanted)
}

// parseSemanticConflicts accepts claim-id=dependency-id=reason. The claim id
// is mandatory, the dependency id may be empty, and the reason is preserved in
// the preview/refusal so a human can review an observed contradiction rather
// than a machine pretending a content hash decided meaning.
func parseSemanticConflicts(values []string) ([]lock.SemanticConflict, error) {
	conflicts := make([]lock.SemanticConflict, 0, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[2]) == "" {
			return nil, fmt.Errorf("semantic conflict must be claim-id=dependency-id=reason")
		}
		conflicts = append(conflicts, lock.SemanticConflict{ClaimID: strings.TrimSpace(parts[0]), DependencyID: strings.TrimSpace(parts[1]), Detail: strings.TrimSpace(parts[2])})
	}
	return conflicts, nil
}

func policySnapshot(claims []model.Claim, ids []string) string {
	byID := map[string]model.Claim{}
	for _, claim := range claims {
		byID[claim.ID] = claim
	}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		claim, ok := byID[id]
		if !ok {
			return
		}
		for _, dep := range claim.RestsOn {
			visit(dep)
		}
	}
	for _, id := range ids {
		visit(id)
	}
	ordered := make([]string, 0, len(seen))
	for id := range seen {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	h := sha256.New()
	for _, id := range ordered {
		fmt.Fprintf(h, "%s=%s\n", id, lock.ContentHash(byID[id]))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func readOptional(path string) ([]byte, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return raw, true
}

func restoreOptional(path string, raw []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".restore-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
