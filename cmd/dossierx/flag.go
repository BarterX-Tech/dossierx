// flag.go implements "dossierx claim flag <id> --claim-says --now-does --reason":
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

	"github.com/BarterX-Tech/dossierx/internal/cliout"
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

// flagData is "dossierx claim flag"'s machine payload: the before/after assertion
// exactly as it was recorded, so an agent can echo back to its human precisely
// what the project now believes is wrong with the claim.
type flagData struct {
	ClaimID       string `json:"claim_id"`
	ClaimSays     string `json:"claim_says"`
	NowDoes       string `json:"now_does"`
	Reason        string `json:"reason"`
	FlaggedAt     string `json:"flagged_at"`
	ReviewPending bool   `json:"review_pending"`
}

// flagDryRun previews "flag <id>". Its preconditions are the two refusals
// newFlagCmd's write path performs — the claim must be locked, and its content
// must live in body — plus the three required assertions, reported as missing
// inputs rather than as a hard error so a preview can be run before the agent
// has composed them.
func flagDryRun(claim model.Claim, claimSays, nowDoes, reason string) *cliout.DryRun {
	dr := cliout.NewDryRun("flag claim " + claim.ID + " for reaudit")

	for _, required := range []struct{ name, value string }{
		{"--claim-says", claimSays},
		{"--now-does", nowDoes},
		{"--reason", reason},
	} {
		if strings.TrimSpace(required.value) == "" {
			dr.Lacking(required.name)
		}
	}

	dr.Require("claim_is_locked", claim.Status == model.StatusLocked,
		fmt.Sprintf("status is %q", claim.Status))
	lay := flagStructuredLayout(claim)
	dr.Require("claim_is_body_only", lay == "", boolDetail(lay == "",
		fmt.Sprintf("layout %q renders from body, which is the only field a flag-sourced reaudit rewrites", claim.Layout),
		flagNonBodyDetail(claim, lay)))

	dr.Effect("sets review_pending on " + claim.SourcePath).
		Effect("records a one-shot pending-flag entry that the next \"dossierx claim reaudit --confirm\" consumes").
		Effect("a later \"dossierx claim unlock\" DISCARDS this flag rather than applying it")

	dr.Propose("review_pending", true).
		Propose("claim_says", claimSays).
		Propose("now_does", nowDoes).
		Propose("reason", reason)
	return dr
}

func newFlagCmd() *cobra.Command {
	var claimSays, nowDoes, reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "flag <id>",
		Short: "Flag a locked claim as needing reaudit, with an explicit before/after and reason",
		Args:  cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			id := args[0]

			if dryRun {
				_, claims, err := loadConfigAndClaims()
				if err != nil {
					return cmdResult{}, err
				}
				claim, ok := loader.FindByID(claims, id)
				if !ok {
					return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "flag: claim %q not found: %w", id, errClaimNotFound)
				}
				return dryRunResult(cmd, "flag", flagDryRun(claim, claimSays, nowDoes, reason)), nil
			}

			if strings.TrimSpace(claimSays) == "" || strings.TrimSpace(nowDoes) == "" || strings.TrimSpace(reason) == "" {
				return cmdResult{}, cliout.Errorf(cliout.CodeMissingFlag,
					"flag: --claim-says, --now-does, and --reason are all required and must be non-empty")
			}

			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}

			// Claim-file write discipline (Phase 0): take the project-wide
			// claims sentinel FIRST — before the flag-store sentinel below —
			// and load claims INSIDE it, so this load->mutate->SaveClaim runs
			// against a snapshot no concurrent claim-file writer can change
			// underneath us. claims-then-flag-store keeps the global order
			// (claims -> lock-store -> flag-store) deadlock-free.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "flag: %w", err)
			}
			defer releaseClaims()

			claims, err := loadClaims(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			claim, ok := loader.FindByID(claims, id)
			if !ok {
				return cmdResult{}, cliout.Errorf(cliout.CodeClaimNotFound, "flag: claim %q not found: %w", id, errClaimNotFound)
			}
			// Any locked claim may be (re-)flagged, review_pending or not —
			// unlike "dossierx claim reaudit", which only ever runs against an
			// already-pending claim, flagging is what PUTS a claim into
			// review_pending in the first place. A non-locked claim is the
			// exit-2 "not in the right state" case (like reaudit's non-pending
			// refusal), so it wraps errWrongState.
			if claim.Status != model.StatusLocked {
				return cmdResult{}, cliout.Errorf(cliout.CodeNotLocked,
					"flag: claim %q is not locked (status %q); only a locked claim can be flagged: %w", id, claim.Status, errWrongState)
			}
			// DX-AUD-11: a flag-sourced reaudit rewrites only claim.Body (see
			// internal/reaudit.ProposeFlagDiff/Apply). For a claim whose
			// rendered content lives outside Body — table rows, steps, or
			// raw_html on ANY layout since v0.4.1 — that would clear
			// review_pending while leaving the actual rendered content stale.
			// Such claims must not be flagged at all; the correct workflow is to
			// unlock, edit the rows/steps/raw_html directly, and relock. See
			// flagStructuredLayout on why the test is on content, not layout.
			if lay := flagStructuredLayout(claim); lay != "" {
				return cmdResult{}, cliout.Errorf(cliout.CodeStructuredLayout,
					"flag: claim %q renders content a flag-sourced reaudit cannot update (%s); unlock the claim, edit it directly, then relock instead", id, flagNonBodyDetail(claim, lay)).
					WithHint(fmt.Sprintf("run: dossierx claim unlock %s --reason \"...\"", id))
			}
			token, err := loader.CaptureClaimFileToken(claim.SourcePath)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "flag: %w", err)
			}

			// Serializes against any concurrent "dossierx claim flag"/"dossierx
			// claim reaudit --confirm" invocation touching this project's flag store file
			// — same reasoning and pattern as newLockCmd's use of
			// AcquireFileLock over internal/lock.Store's shared file.
			release, err := lock.AcquireFileLock(flagStorePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "flag: %w", err)
			}
			defer release()

			store, err := reaudit.LoadFlagStore(flagStorePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "flag: %w", err)
			}
			pending := reaudit.PendingFlag{
				ClaimSays: claimSays,
				NowDoes:   nowDoes,
				Reason:    reason,
				FlaggedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			store.Flags[id] = pending
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "flag: %w", err)
			}

			claim.ReviewPending = true
			if err := loader.SaveClaimIfUnchanged(claim, token); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "flag: %w", err)
			}

			return cmdResult{
				Data: flagData{
					ClaimID:       id,
					ClaimSays:     claimSays,
					NowDoes:       nowDoes,
					Reason:        reason,
					FlaggedAt:     pending.FlaggedAt,
					ReviewPending: true,
				},
				Text: func() {
					fmt.Fprintf(cmd.OutOrStdout(), "flag: %s flagged for reaudit (review_pending=true)\n", id)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&claimSays, "claim-says", "", "what the claim currently (wrongly) asserts (required)")
	cmd.Flags().StringVar(&nowDoes, "now-does", "", "what is actually true now (required)")
	cmd.Flags().StringVar(&reason, "reason", "", "why this claim is being flagged (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what flagging would do — preconditions, side effects, what is missing — and write nothing")
	return cmd
}

// flagStructuredLayout returns the layout of a claim that renders content a
// Body-only, flag-sourced reaudit cannot update — table rows, steps, or
// raw_html — or "" if the claim really is body-only and therefore safe to flag
// (DX-AUD-11).
//
// THE TEST IS ON CONTENT, NOT ON THE LAYOUT NAME. Until v0.4.1 raw_html could
// only exist on layout: mockup, so asking "is the layout mockup?" and asking
// "does this claim carry raw_html?" were the same question and this function
// asked the first. Issue #25 separated them: raw_html is now legal on every
// layout (internal/lint's raw-html-scope gates it on the module allowlist and
// raw_html_reviewed only, components.MockupHTML's trusted condition carries no
// layout term, and every component template — card/banner/list/tree/table/steps
// — renders {{if .RawHTML}}). A card/banner/list/tree claim can therefore carry
// markup the viewer renders, and classifying it body-only by its layout would
// let a flag clear review_pending while that markup stayed stale: precisely the
// failure this gate exists to prevent. So a claim carrying raw_html is refused
// whatever its layout, and the layout returned is its real one.
//
// The layout-less inference mirrors internal/catalog.inferLayout exactly (rows
// -> table, steps -> steps, otherwise card) and deliberately has NO raw_html
// leg. raw_html says nothing about layout; inferring mockup from it would label
// a card claim's markup a mockup — disagreeing with the layout the catalog and
// the renderer use for that same claim — and would put a layout the author
// never wrote into the refusal message an agent reads.
func flagStructuredLayout(c model.Claim) model.Layout {
	layout := c.Layout
	if layout == "" {
		switch {
		case len(c.Rows) > 0:
			layout = model.LayoutTable
		case len(c.Steps) > 0:
			layout = model.LayoutSteps
		default:
			layout = model.LayoutCard
		}
	}
	switch layout {
	case model.LayoutTable, model.LayoutSteps, model.LayoutMockup:
		return layout
	}
	if c.RawHTML != "" {
		return layout
	}
	return ""
}

// flagNonBodyDetail explains WHICH non-body content flagStructuredLayout
// refused a claim over — the raw_html a v0.4.1 claim may carry on any layout,
// or the rows/steps a structured layout renders from. Without this a card claim
// bearing raw_html would be refused with nothing but `layout is "card"`, which
// reads as a bug rather than as the reason.
func flagNonBodyDetail(c model.Claim, lay model.Layout) string {
	if c.RawHTML != "" {
		return fmt.Sprintf("layout is %q and the claim carries raw_html, which a flag-sourced reaudit cannot rewrite", lay)
	}
	return fmt.Sprintf("layout is %q; a flag-sourced reaudit can only rewrite body", lay)
}
