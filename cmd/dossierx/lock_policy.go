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
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// policyLockPreviewData is returned for both one-claim and group dry runs.
// Snapshot is an opaque candidate-content token a later write may supply with
// --proposal; a mismatch refuses rather than approving unseen dependency text.
type policyLockPreviewData struct {
	Would      string             `json:"would"`
	Blocked    bool               `json:"blocked"`
	Snapshot   string             `json:"snapshot"`
	Evaluation lock.SetEvaluation `json:"evaluation"`
}

type policyLockData struct {
	ClaimIDs   []string           `json:"claim_ids"`
	Reason     string             `json:"reason"`
	Snapshot   string             `json:"snapshot"`
	Evaluation lock.SetEvaluation `json:"evaluation"`
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

func previewPolicyLock(cmd *cobra.Command, ids []string, reason string) (cmdResult, error) {
	cfg, claims, err := loadConfigAndClaims()
	if err != nil {
		return cmdResult{}, err
	}
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock preview: %w", err)
	}
	evaluation := lock.EvaluateSet(claims, ids, cfg, store)
	data := policyLockPreviewData{
		Would:      "lock " + strings.Join(evaluation.RequestedIDs, ", "),
		Blocked:    !evaluation.Allowed() || strings.TrimSpace(reason) == "",
		Snapshot:   policySnapshot(claims, evaluation.RequestedIDs),
		Evaluation: evaluation,
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
func runPolicySetLock(cmd *cobra.Command, ids []string, reason, proposal string) (cmdResult, error) {
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
	evaluation := lock.EvaluateSet(claims, ids, cfg, store)
	snapshot := policySnapshot(claims, evaluation.RequestedIDs)
	if proposal != "" && proposal != snapshot {
		return cmdResult{Data: policyLockData{ClaimIDs: evaluation.RequestedIDs, Reason: reason, Snapshot: snapshot, Evaluation: evaluation}},
			cliout.Errorf(cliout.CodeClaimFileChanged, "lock: preview snapshot is stale; re-run --dry-run and review the current dependency content")
	}
	if !evaluation.Allowed() {
		return cmdResult{Data: policyLockData{ClaimIDs: evaluation.RequestedIDs, Reason: reason, Snapshot: snapshot, Evaluation: evaluation}},
			cliout.Errorf(cliout.CodeLintFailed, "lock: one or more requested claims are refused; no approval was written")
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
	}
	approval := lock.Approval{Actor: lock.DefaultActor(), Reason: reason}
	for _, claim := range finalClaims {
		if !requested[claim.ID] {
			continue
		}
		lock.RefreshBaseline(claim, finalClaims, store)
		lock.RecordApproval(store, claim, approval)
	}
	if err := store.Save(); err != nil {
		if recoverErr := rollback(); recoverErr != nil {
			return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: store write failed and %v", recoverErr)
		}
		return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "lock: store write failed; prior state restored: %w", err)
	}
	return cmdResult{Data: policyLockData{ClaimIDs: evaluation.RequestedIDs, Reason: reason, Snapshot: snapshot, Evaluation: evaluation}, Text: func() {
		fmt.Fprintf(cmd.OutOrStdout(), "lock: %d claim(s) locally approved: %s\n", len(evaluation.RequestedIDs), strings.Join(evaluation.RequestedIDs, ", "))
	}}, nil
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
