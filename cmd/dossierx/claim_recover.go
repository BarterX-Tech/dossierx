// claim_recover.go is `dossierx claim recover-approved-content`: the explicit,
// one-time boundary at which a project whose approvals predate
// LedgerRecord.Content gets that content back from its own git history.
//
// It is a SEPARATE VERB, run deliberately, rather than something `check` does
// on its way past, and that is the whole design. Three reasons, in order of
// how much they matter:
//
//  1. It writes the lock store. Everything else that writes the lock store in
//     this product is a verb a human asked for, carrying the human's own
//     approving words in --reason. Recovery changes no approval — it can only
//     store bytes the record's existing hash already certifies — but it edits
//     the file that IS the approval record, and a file like that should not
//     change as a side effect of a command someone ran to render a viewer.
//  2. It reads git history, which `check` does not otherwise need. Making the
//     render path depend on a git work tree would mean a project built from a
//     tarball, or in a container without git, silently produced a different
//     viewer from the same claims.
//  3. It is finished after one run. Every approval taken from here on stores
//     its own wording at lock time, so this verb has nothing left to do on a
//     corpus it has already swept. A permanent per-build history walk to serve
//     a one-time migration is the wrong shape.
package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/approvalrecovery"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// recoverData is the envelope body for both the preview and the write. The
// two carry the same fields on purpose: a dry run's report of what it found is
// exactly what the write reports having done, so a reader comparing them is
// comparing like with like rather than two differently-shaped summaries.
type recoverData struct {
	Would  string `json:"would,omitempty"`
	Reason string `json:"reason,omitempty"`

	// Eligible and AlreadyRetained describe the corpus: how many claims are
	// edited-since-approval with no wording on the record, and how many are
	// edited-since-approval but already carry it. Both are stated so a run
	// that recovers nothing says WHY — nothing to do, or nothing findable.
	Eligible        int `json:"eligible"`
	AlreadyRetained int `json:"already_retained"`

	Recovered   []approvalrecovery.Outcome `json:"recovered"`
	Unrecovered []approvalrecovery.Outcome `json:"unrecovered"`

	// Written is the claim ids whose records gained content. On a dry run it
	// is absent; on a write it is what actually changed on disk, which can be
	// shorter than Recovered if a record gained its wording by another route
	// between the search and the write.
	Written []string `json:"written,omitempty"`

	Preconditions []cliout.Precondition `json:"preconditions,omitempty"`
	SideEffects   []string              `json:"side_effects,omitempty"`
	Missing       []string              `json:"missing,omitempty"`
}

// newClaimRecoverApprovedContentCmd builds the verb.
func newClaimRecoverApprovedContentCmd() *cobra.Command {
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "recover-approved-content",
		Short: "Recover approved wording from git history for approvals recorded before the ledger kept it",
		Long: "Find, for each claim edited since a released approval whose record carries no wording, " +
			"the revision in this repository's history whose content hashes to exactly the hash that " +
			"approval signed, and record it. A revision that does not hash equal is never used, so " +
			"nothing here can widen or invent an approval; claims whose approved revision cannot be " +
			"found are reported by name and left alone.",
		Args: cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, _ []string) (cmdResult, error) {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}
			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "claim recover-approved-content: %w", err)
			}
			result, err := approvalrecovery.Recover(claims, store, cfg.Dir())
			if err != nil {
				if errors.Is(err, approvalrecovery.ErrGitUnavailable) {
					return cmdResult{}, cliout.Errorf(cliout.CodeGitUnavailable,
						"claim recover-approved-content: %w", err).
						WithHint("run this from inside the project's git work tree, with git installed")
				}
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "claim recover-approved-content: %w", err)
			}

			data := recoverData{
				Would: "record approved wording for " + strconv.Itoa(len(result.Recovered)) +
					" " + word(len(result.Recovered), "claim"),
				Reason:          reason,
				Eligible:        result.Eligible,
				AlreadyRetained: result.AlreadyRetained,
				Recovered:       result.Recovered,
				Unrecovered:     result.Unrecovered,
			}

			if dryRun {
				data.SideEffects = []string{"write the recovered wording onto existing lock ledger records; no approval, hash, status or claim file changes"}
				data.Preconditions = []cliout.Precondition{
					{Name: "every_recovered_revision_hashes_to_its_approval", OK: true},
				}
				if reason == "" {
					data.Missing = []string{"--reason"}
				}
				return cmdResult{Data: data, Text: func() { printRecoverReport(cmd, data, false) }}, nil
			}

			if err := requireReason("claim recover-approved-content", reason); err != nil {
				return cmdResult{}, err
			}
			// The ledger is how an approval reaches a collaborator. A store
			// git would ignore is the same refusal here as it is for the
			// verbs that create approvals.
			warnings, err := refuseIfStoresGitignored(cfg, "claim recover-approved-content")
			if err != nil {
				return cmdResult{}, err
			}
			if len(result.Recovered) == 0 {
				// Nothing to write. Say so and touch no file: an empty save
				// would still rewrite the store and show up as a diff for a
				// run that changed nothing.
				return cmdResult{Data: data, Warnings: warnings, Text: func() { printRecoverReport(cmd, data, true) }}, nil
			}

			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "claim recover-approved-content: %w", err)
			}
			defer release()
			// Re-read under the lock. The search ran against a store nobody
			// was holding, so a concurrent lock or unlock could have moved a
			// record since; RetainApprovedContent re-checks every hash against
			// THIS store and refuses rather than writing a stale match.
			store, err = lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "claim recover-approved-content: %w", err)
			}
			written, err := approvalrecovery.Apply(store, result)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "claim recover-approved-content: %w", err)
			}
			if len(written) > 0 {
				if err := store.Save(); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "claim recover-approved-content: %w", err)
				}
			}
			data.Written = written
			return cmdResult{Data: data, Warnings: warnings, Text: func() { printRecoverReport(cmd, data, true) }}, nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "why the human is recovering this wording (required unless --dry-run)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be recovered without writing")
	return cmd
}

// printRecoverReport renders the text form. It names every unrecovered claim
// rather than counting them: an unrecovered claim is one a reviewer will keep
// seeing "the approved wording was not kept" on, and a number alone gives them
// no way to find out which.
func printRecoverReport(cmd *cobra.Command, data recoverData, wrote bool) {
	out := cmd.OutOrStdout()
	verb := "would record"
	if wrote {
		verb = "recorded"
	}
	fmt.Fprintf(out, "approved-content recovery: %d of %d eligible %s %s\n",
		len(data.Recovered), data.Eligible, word(data.Eligible, "claim"), verb)
	if data.AlreadyRetained > 0 {
		fmt.Fprintf(out, "  %d edited %s already carried the approved wording and were not searched\n",
			data.AlreadyRetained, word(data.AlreadyRetained, "claim"))
	}
	for _, o := range data.Recovered {
		fmt.Fprintf(out, "  + %s\n      approved %s by %s, found in %s (%s), %d %s searched\n",
			o.ClaimID, o.ApprovedAt, o.ApprovedBy, o.Commit, o.CommitDate,
			o.Revisions, word(o.Revisions, "revision"))
	}
	for _, o := range data.Unrecovered {
		fmt.Fprintf(out, "  - %s\n      %s\n", o.ClaimID, o.Reason)
	}
	if wrote && len(data.Written) < len(data.Recovered) {
		fmt.Fprintf(out, "  %d recovered %s already gained wording by another route and was left alone\n",
			len(data.Recovered)-len(data.Written), word(len(data.Recovered)-len(data.Written), "claim"))
	}
	if len(data.Unrecovered) > 0 {
		fmt.Fprintln(out, "  claims listed with - keep reporting that the approved wording was not retained")
	}
}

// word is the plural of noun for n, without the number — the callers here
// already print the count, sometimes from a different expression than the one
// being pluralised.
func word(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}
