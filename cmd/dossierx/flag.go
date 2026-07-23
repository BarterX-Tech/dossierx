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

			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return fmt.Errorf("flag: claim %q not found", id)
			}
			// Any locked claim may be (re-)flagged, review_pending or not —
			// unlike "dossierx reaudit", which only ever runs against an
			// already-pending claim, flagging is what PUTS a claim into
			// review_pending in the first place.
			if claim.Status != model.StatusLocked {
				return fmt.Errorf("flag: claim %q is not locked (status %q); only a locked claim can be flagged", id, claim.Status)
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
			if err := loader.SaveClaim(claim); err != nil {
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
