// flag.go implements "dossierx flag <id> --claim-says --now-does --reason":
// the agent-initiated trigger for internal/reaudit's second proposal
// source (see internal/reaudit/flagstore.go's doc comment). Split out of
// main.go for the same file-size reason build_order.go and implink.go are
// their own files — the construction pattern (newXCmd() *cobra.Command,
// added to newRootCmd()'s AddCommand list) is identical to every other
// command in this package.
package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

// flagStorePath is the on-disk location of the pending-flag store,
// resolved relative to the config file's own directory (never cwd) — the
// same convention storePath/catalogPath/renderOutPath already follow.
func flagStorePath(cfg *config.Config) string {
	return filepath.Join(cfg.Dir(), ".dossierx-flag-store.json")
}

func newFlagCmd() *cobra.Command {
	var claimSays, nowDoes, reason string
	cmd := &cobra.Command{
		Use:   "flag <id>",
		Short: "Flag a locked claim as needing reaudit, with an explicit before/after and reason",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if strings.TrimSpace(claimSays) == "" || strings.TrimSpace(nowDoes) == "" || strings.TrimSpace(reason) == "" {
				return fmt.Errorf("flag: --claim-says, --now-does, and --reason are all required and must be non-empty")
			}

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
				return fmt.Errorf("flag: %w", err)
			}
			defer releaseClaims()

			claims, err := loadClaims(cfg)
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return fmt.Errorf("flag: claim %q not found: %w", id, errClaimNotFound)
			}
			// Any locked claim may be (re-)flagged, review_pending or not —
			// unlike "dossierx reaudit", which only ever runs against an
			// already-pending claim, flagging is what PUTS a claim into
			// review_pending in the first place. A non-locked claim is the
			// exit-2 "not in the right state" case (like reaudit's non-pending
			// refusal), so it wraps errWrongState.
			if claim.Status != model.StatusLocked {
				return fmt.Errorf("flag: claim %q is not locked (status %q); only a locked claim can be flagged: %w", id, claim.Status, errWrongState)
			}
			// DX-AUD-11: a flag-sourced reaudit rewrites only claim.Body (see
			// internal/reaudit.ProposeFlagDiff/Apply). For a claim whose
			// rendered content lives outside Body — table rows, steps, or a
			// raw-HTML mockup — that would clear review_pending while leaving
			// the actual rendered content stale. Such claims must not be
			// flagged at all; the correct workflow is to unlock, edit the
			// rows/steps/raw_html directly, and relock.
			if lay := flagStructuredLayout(claim); lay != "" {
				return fmt.Errorf("flag: claim %q has a %s layout whose rendered content (rows/steps/raw HTML) a flag-sourced reaudit cannot update; unlock the claim, edit it directly, then relock instead", id, lay)
			}
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return fmt.Errorf("flag: %w", err)
			}

			// Serializes against any concurrent "dossierx flag"/"dossierx reaudit
			// --confirm" invocation touching this project's flag store file
			// — same reasoning and pattern as newLockCmd's use of
			// AcquireFileLock over internal/lock.Store's shared file.
			release, err := lock.AcquireFileLock(flagStorePath(cfg))
			if err != nil {
				return fmt.Errorf("flag: %w", err)
			}
			defer release()

			store, err := reaudit.LoadFlagStore(flagStorePath(cfg))
			if err != nil {
				return fmt.Errorf("flag: %w", err)
			}
			store.Flags[id] = reaudit.PendingFlag{
				ClaimSays: claimSays,
				NowDoes:   nowDoes,
				Reason:    reason,
				FlaggedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			if err := store.Save(); err != nil {
				return fmt.Errorf("flag: %w", err)
			}

			claim.ReviewPending = true
			if err := loader.SaveClaimIfUnchanged(claim, token); err != nil {
				return fmt.Errorf("flag: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "flag: %s flagged for reaudit (review_pending=true)\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&claimSays, "claim-says", "", "what the claim currently (wrongly) asserts (required)")
	cmd.Flags().StringVar(&nowDoes, "now-does", "", "what is actually true now (required)")
	cmd.Flags().StringVar(&reason, "reason", "", "why this claim is being flagged (required)")
	return cmd
}

// flagStructuredLayout returns the non-body ("structured") layout a claim
// renders with — table, steps, or mockup — or "" if the claim is body-only
// (card/banner/list/tree). It mirrors internal/catalog.inferLayout's
// shape-based inference so a claim that omits an explicit layout but carries
// rows/steps (or raw HTML) is still caught, since that is exactly what a
// flag-sourced, Body-only reaudit would leave stale (DX-AUD-11).
func flagStructuredLayout(c model.Claim) model.Layout {
	layout := c.Layout
	if layout == "" {
		switch {
		case len(c.Rows) > 0:
			layout = model.LayoutTable
		case len(c.Steps) > 0:
			layout = model.LayoutSteps
		case c.RawHTML != "":
			layout = model.LayoutMockup
		}
	}
	switch layout {
	case model.LayoutTable, model.LayoutSteps, model.LayoutMockup:
		return layout
	}
	return ""
}
