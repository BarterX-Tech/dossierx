// build_order.go wires internal/buildorder into the CLI as "dossierx
// build-order propose|status|lock", split out of main.go (already ~660
// lines before this feature) purely for file size — the construction
// pattern (newXCmd() *cobra.Command, added to newRootCmd()'s AddCommand
// list) is identical to every other command in main.go.
package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
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
	return cmd
}

// requireModuleFlag validates the shared --module flag every build-order
// subcommand takes: it must be set, since (unlike "dossierx lock <id>", which
// names one claim) a build order is always computed for one whole module
// at a time.
func requireModuleFlag(module string) error {
	if module == "" {
		return fmt.Errorf("build-order: --module is required")
	}
	return nil
}

func newBuildOrderProposeCmd() *cobra.Command {
	var module string
	cmd := &cobra.Command{
		Use:   "propose",
		Short: "Propose (and write) a build-order artifact for --module (refused unless every claim in it is locked)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireModuleFlag(module); err != nil {
				return err
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}

			artifact, err := buildorder.Propose(claims, cfg, module)
			if err != nil {
				return fmt.Errorf("build-order propose: %w", err)
			}

			path := buildorder.ArtifactPath(cfg, module)
			if err := buildorder.WriteArtifact(artifact, path); err != nil {
				return fmt.Errorf("build-order propose: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "build-order propose: wrote %s\n", path)
			for _, p := range artifact.Phases {
				fmt.Fprintf(out, "  %-14s %d claim(s)\n", p.Phase, len(p.Claims))
			}
			fmt.Fprintf(out, "  %-14s %d claim(s)\n", "excluded", len(artifact.Excluded))
			fmt.Fprintln(out, "  locked: false")
			return nil
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "module to compute the build order for (required)")
	return cmd
}

func newBuildOrderStatusCmd() *cobra.Command {
	var module string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show --module's build-order artifact state: proposed/locked/stale and coverage",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireModuleFlag(module); err != nil {
				return err
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			path := buildorder.ArtifactPath(cfg, module)
			artifact, err := buildorder.Status(path, claims)
			if err != nil {
				if errors.Is(err, buildorder.ErrNotProposed) {
					fmt.Fprintf(out, "build-order status: %s: not proposed yet (run \"dossierx build-order propose --module %s\")\n", module, module)
					return nil
				}
				return fmt.Errorf("build-order status: %w", err)
			}

			total := 0
			for _, c := range claims {
				if c.Module == module {
					total++
				}
			}
			covered := len(artifact.ClaimIDs())

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
			return nil
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "module to report build-order status for (required)")
	return cmd
}

func newBuildOrderLockCmd() *cobra.Command {
	var module string
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock --module's proposed build-order artifact, snapshotting a content-hash baseline",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireModuleFlag(module); err != nil {
				return err
			}
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return err
			}

			path := buildorder.ArtifactPath(cfg, module)
			artifact, err := buildorder.Lock(path, claims)
			if err != nil {
				return fmt.Errorf("build-order lock: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "build-order lock: %s locked at %s\n", module, artifact.LockedAt)
			return nil
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "module to lock the build order for (required)")
	return cmd
}
