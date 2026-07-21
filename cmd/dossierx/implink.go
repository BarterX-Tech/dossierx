// implink.go wires internal/implink into the CLI as "docs implink
// set|status", split out of main.go for the same file-size reason
// build_order.go is its own file — the construction pattern (newXCmd()
// *cobra.Command, added to newRootCmd()'s AddCommand list) is identical to
// every other command in this package.
package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// newImplinkCmd is the "docs implink" command group: set (the one and only
// write action — takes effect immediately, no confirm step) and status
// (read-only drift/coverage reporting), each operating on a single
// --module.
func newImplinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "implink",
		Short: "Link real source files to the claims they implement, and report drift/coverage",
	}
	cmd.AddCommand(newImplinkSetCmd(), newImplinkStatusCmd())
	return cmd
}

func newImplinkSetCmd() *cobra.Command {
	var module, claimID, file, symbol string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Link --file (optionally at --symbol) to --claim as its implementation (immediate, no confirm step)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if module == "" {
				return fmt.Errorf("implink set: --module is required")
			}
			if claimID == "" {
				return fmt.Errorf("implink set: --claim is required")
			}
			if file == "" {
				return fmt.Errorf("implink set: --file is required")
			}

			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}

			// Serializes concurrent "docs implink set" invocations that
			// share this module's artifact file — same reasoning and
			// pattern as newLockCmd's use of AcquireFileLock over
			// internal/lock.Store's shared file, applied here to
			// implink.ArtifactPath instead.
			path := implink.ArtifactPath(cfg, module)
			release, err := lock.AcquireFileLock(path)
			if err != nil {
				return fmt.Errorf("implink set: %w", err)
			}
			defer release()

			artifact, err := implink.Set(claims, cfg, module, claimID, file, symbol)
			if err != nil {
				return fmt.Errorf("implink set: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "implink set: %s -> %s", claimID, file)
			if symbol != "" {
				fmt.Fprintf(out, "#%s", symbol)
			}
			fmt.Fprintln(out)
			fmt.Fprintf(out, "implink set: wrote %s (%d claim(s) linked in module %q)\n", path, len(artifact.Links), module)
			return nil
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "module the claim belongs to (required)")
	cmd.Flags().StringVar(&claimID, "claim", "", "claim id to link (required)")
	cmd.Flags().StringVar(&file, "file", "", "project-relative path to the implementing file (required)")
	cmd.Flags().StringVar(&symbol, "symbol", "", "optional symbol (function/type/etc) within --file")
	return cmd
}

func newImplinkStatusCmd() *cobra.Command {
	var module string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report --module's implementation-link coverage and drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			if module == "" {
				return fmt.Errorf("implink status: --module is required")
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			report, err := implink.Status(claims, cfg, module)
			if err != nil {
				if errors.Is(err, implink.ErrNoArtifact) {
					fmt.Fprintf(out, "implink status: %s: nothing linked yet (run \"docs implink set --module %s ...\")\n", module, module)
					return nil
				}
				return fmt.Errorf("implink status: %w", err)
			}

			fmt.Fprintln(out, report.Summary())
			for _, d := range report.Drifted {
				fmt.Fprintf(out, "  drifted: %s %s: %s\n", d.ClaimID, d.File, d.Reason)
			}
			for _, id := range report.UnlinkedIDs {
				fmt.Fprintf(out, "  unlinked: %s\n", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "module to report implementation-link status for (required)")
	return cmd
}
