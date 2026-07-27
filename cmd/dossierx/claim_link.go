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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
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
				// The preview reproduces implink.Set's refusals IN ITS ORDER —
				// claim exists, claim belongs to --module, claim is locked, file
				// is project-relative, file exists — because a preview that
				// checked only some of them answered "blocked: false" for
				// invocations the real run refuses outright. Three of the five
				// were missing: a --file that does not exist, a --file that is
				// absolute or escapes the project directory, and a --claim
				// belonging to a different module than --module. All three are
				// ordinary agent mistakes (a path typo, a stale module name), so
				// they were exactly the cases a preview is for.
				if claimID != "" {
					claim, ok := loader.FindByID(claims, claimID)
					dr.Require("claim_exists", ok, claimID)
					if ok {
						// The module test comes BEFORE the locked test, matching
						// implink.Set: a claim named with the wrong --module is a
						// caller error whatever its status, and reporting it as
						// not-locked would send the agent to lock a claim that was
						// never the one it meant.
						if module != "" {
							dr.Require("claim_is_in_module", claim.Module == module,
								fmt.Sprintf("claim %q belongs to module %q", claimID, claim.Module))
						}
						// implink.Set refuses a claim that is not locked: an
						// implementation link asserts "this code implements a
						// reviewed fact", and a draft claim is not yet a fact.
						dr.Require("claim_is_locked", claim.Status == model.StatusLocked,
							fmt.Sprintf("status is %q", claim.Status))
					}
				}
				if file != "" {
					inside, detail := fileIsInsideProject(cfg, file)
					dr.Require("file_is_project_relative", inside, detail)
					if inside {
						// implink.Set hashes the file to take its drift baseline,
						// and a file it cannot open is the refusal an agent hits
						// most: the link records a project-RELATIVE path, so a
						// path that was correct in the agent's own cwd is not.
						resolved := filepath.Join(cfg.Dir(), file)
						dr.Require("file_exists", fileExists(resolved), resolved)
					}
				}
				dr.Effect("rewrites " + path).
					Effect("\"dossierx check\" will report this link as drifted if the file or symbol later moves")
				dr.Propose("file", file).Propose("symbol", symbol).Propose("claim_id", claimID)
				return dryRunResult(cmd, "claim link", dr), nil
			}

			// The two STATE refusals, classified here rather than left to
			// implink.Set's single error return.
			//
			// implink.Set returns plain fmt.Errorf values for four structurally
			// different refusals — unknown claim, wrong module, not locked,
			// missing file — and wrapping all of them in implink_refused told an
			// agent the wrong thing about two of them. The router skill's row for
			// implink_refused reads "This is your invocation or your tag, not a
			// gate: fix it and re-run", so an agent that hit the not-locked gate
			// retried with a corrected --file and never reached the real recovery
			// (ask the human, lock the claim); an unknown id never reached
			// "dossierx claim list --match" either. Both codes are documented:
			// cliout.CodeNotLocked names linking explicitly, and two skills
			// publish the exit-2 rows.
			//
			// The distinction is computed from the SAME seam the dry run already
			// uses fifteen lines above (claim_exists / claim_is_locked), which is
			// what makes preview and write path agree about which gate fired
			// rather than merely agreeing by inspection. The genuinely
			// caller-error refusals — wrong module, absolute or escaping path,
			// missing file — stay on implink_refused, where that row's advice is
			// exactly right.
			linkClaim, found := loader.FindByID(claims, claimID)
			if !found {
				return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound,
					"claim link: claim %q not found: %w", claimID, errClaimNotFound).
					WithHint("run: dossierx claim list --match \"<words from the claim>\"")
			}
			// The module test is left to implink.Set so the REFUSAL ORDER stays
			// identical to it: a claim named with the wrong --module is a caller
			// error (implink_refused) whatever its status, and reporting it as
			// not_locked would send the agent to lock a claim that was never the
			// one it meant.
			if linkClaim.Module == module && linkClaim.Status != model.StatusLocked {
				return cmdResult{}, cliout.Errorf(cliout.CodeNotLocked,
					"claim link: claim %q is not locked (status %q); an implementation link asserts that code implements a REVIEWED fact, and a draft claim is not yet one: %w", claimID, linkClaim.Status, errWrongState).
					WithHint(fmt.Sprintf("ask the human, then: dossierx claim lock %s --reason \"<their words>\"", claimID))
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

// fileIsInsideProject reproduces implink.Set's path contract for the dry run: a
// --file must be RELATIVE and must resolve to somewhere inside the project
// directory. It returns the verdict plus the detail string the precondition
// reports.
//
// The rule is duplicated here rather than reached through internal/implink
// because Set performs it as part of a write and exposes no read-only form. It
// is deliberately the same three tests in the same order (absolute, resolve,
// escape) so the two cannot disagree about a path either accepts; the write path
// remains the authority, and this only ever has to STOP being wrong about a
// refusal it would perform.
func fileIsInsideProject(cfg *config.Config, file string) (ok bool, detail string) {
	if filepath.IsAbs(file) {
		return false, fmt.Sprintf("%q is absolute; a link records a project-relative path so it means the same thing on every machine", file)
	}
	absDir, err := filepath.Abs(cfg.Dir())
	if err != nil {
		return false, fmt.Sprintf("cannot resolve the project directory %q: %v", cfg.Dir(), err)
	}
	rel, err := filepath.Rel(absDir, filepath.Join(absDir, file))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, fmt.Sprintf("%q escapes the project directory via \"..\"", file)
	}
	return true, file
}

// linkTarget renders "<file>" or "<file>#<symbol>" for a dry run's would-phrase.
func linkTarget(file, symbol string) string {
	if symbol == "" {
		return file
	}
	return file + "#" + symbol
}
