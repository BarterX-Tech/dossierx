// claim_link.go wires internal/implink's one write action into the CLI as
// "dossierx claim link".
//
// v0.3.0 deleted the "implink" noun entirely. Its two leaves went different
// ways: "implink status" was absorbed by "claim show", which reports the same
// per-claim links and drift as part of one whole-claim answer, and "implink
// set" became this — a lifecycle verb about a claim, filed under the claim
// noun where every other thing you do to a claim already lives. Nothing about
// the underlying operation changed: it is still the immediate, unreviewed,
// agent-autonomous statement of fact "this file implements that claim", with
// internal/implink's locked-claim gate as its only precondition.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// claimLinkData is "dossierx claim link"'s machine payload — the link that was
// recorded, plus the module-wide link count so a caller can see coverage grow
// without a second call.
type claimLinkData struct {
	Module      string `json:"module"`
	ClaimID     string `json:"claim_id"`
	File        string `json:"file"`
	Symbol      string `json:"symbol,omitempty"`
	Path        string `json:"path"`
	LinkedCount int    `json:"linked_count"`
}

func newClaimLinkCmd() *cobra.Command {
	var module, claimID, file, symbol string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link --file (optionally at --symbol) to --claim as its implementation (immediate, no confirm step)",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			for _, required := range []struct{ name, value string }{
				{"--module", module},
				{"--claim", claimID},
				{"--file", file},
			} {
				if required.value == "" && !dryRun {
					return cmdResult{}, cliout.Errorf(cliout.CodeMissingFlag, "claim link: %s is required", required.name)
				}
			}

			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			if module != "" {
				if err := requireKnownModule(cfg, module); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeUnknownModule, "claim link: %w", err)
				}
			}
			path := implink.ArtifactPath(cfg, module)

			if dryRun {
				dr := cliout.NewDryRun("link " + linkTarget(file, symbol) + " to claim " + claimID)
				for _, required := range []struct{ name, value string }{
					{"--module", module},
					{"--claim", claimID},
					{"--file", file},
				} {
					if required.value == "" {
						dr.Lacking(required.name)
					}
				}
				if claimID != "" {
					claim, ok := loader.FindByID(claims, claimID)
					dr.Require("claim_exists", ok, claimID)
					// implink.Set refuses a claim that is not locked: an
					// implementation link asserts "this code implements a
					// reviewed fact", and a draft claim is not yet a fact.
					if ok {
						dr.Require("claim_is_locked", claim.Status == model.StatusLocked,
							fmt.Sprintf("status is %q", claim.Status))
					}
				}
				dr.Effect("rewrites " + path).
					Effect("\"dossierx check\" will report this link as drifted if the file or symbol later moves")
				dr.Propose("file", file).Propose("symbol", symbol).Propose("claim_id", claimID)
				return dryRunResult(cmd, "claim link", dr), nil
			}

			// Serializes concurrent "dossierx claim link" invocations that
			// share this module's artifact file — same reasoning and
			// pattern as newLockCmd's use of AcquireFileLock over
			// internal/lock.Store's shared file, applied here to
			// implink.ArtifactPath instead.
			release, err := lock.AcquireFileLock(path)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "claim link: %w", err)
			}
			defer release()

			artifact, err := implink.Set(claims, cfg, module, claimID, file, symbol)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeImplinkRefused, "claim link: %w", err)
			}

			return cmdResult{
				Data: claimLinkData{
					Module:      module,
					ClaimID:     claimID,
					File:        file,
					Symbol:      symbol,
					Path:        path,
					LinkedCount: len(artifact.Links),
				},
				Text: func() {
					out := cmd.OutOrStdout()
					fmt.Fprintf(out, "claim link: %s -> %s", claimID, file)
					if symbol != "" {
						fmt.Fprintf(out, "#%s", symbol)
					}
					fmt.Fprintln(out)
					fmt.Fprintf(out, "claim link: wrote %s (%d claim(s) linked in module %q)\n", path, len(artifact.Links), module)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&module, "module", "", "module the claim belongs to (required)")
	cmd.Flags().StringVar(&claimID, "claim", "", "claim id to link (required)")
	cmd.Flags().StringVar(&file, "file", "", "project-relative path to the implementing file (required)")
	cmd.Flags().StringVar(&symbol, "symbol", "", "optional symbol (function/type/etc) within --file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what linking would do, and write nothing")
	return cmd
}

// linkTarget renders "<file>" or "<file>#<symbol>" for a dry run's would-phrase.
func linkTarget(file, symbol string) string {
	if symbol == "" {
		return file
	}
	return file + "#" + symbol
}
