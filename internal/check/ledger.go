// ledger.go wires the LOCK-LEDGER GATE into the check pipeline.
//
// internal/lock owns the RULES — lock.Audit compares the claims on disk against
// the ledger records and the comment digests, and names every disagreement.
// This file owns the three things that are not the rules' business: what state
// they are evaluated against, WHEN in the pipeline they run, and what a finding
// means to the command. That split is deliberate. The rules have to live next
// to the hash and the records they compare, or they drift from them; the
// decision to fail a command has to live in the pipeline, which is the only
// layer that knows what has already been written to disk by the time the gate
// speaks.
//
// THE GATE IS NOT A LINT, and internal/lock/audit.go explains at length why
// (registering these in lint.Registry would make one tampered claim freeze all
// locking project-wide AND stop the viewer regenerating — a denial of service
// handed to whoever edits a YAML file wrong). The consequence for THIS file is
// concrete: the gate runs as the pipeline's LAST step, after .catalog.json and
// viewer/index.html have already been written. A project whose ledger has been
// tampered with still regenerates its documentation; what it does not do is
// exit zero.
//
// Everything here is read-only. Two stores are loaded and nothing is written,
// adopted, or repaired — repairing is precisely what an attacker would want an
// integrity check to do, because "re-lock it and the error goes away" records
// whatever the files say NOW as approved.
package check

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// RuleLedgerUnreadable is the one gate finding internal/lock cannot raise,
// because it is not a statement about a claim — it is a statement about the
// gate's own evidence: a store file that exists but could not be decoded.
//
// It lives here rather than alongside lock's five rule names for that reason,
// and it is a stable string on the same terms as they are: the hook, CI and the
// skills branch on rule names, so this one is contract too.
//
// A corrupt ledger must never be quieter than a deleted one. Deleting the store
// is already caught (lock.RuleLockLedgerAbsent); truncating it to "{" would,
// without this, either crash the command with a parse error that reads like a
// bug or — far worse — be swallowed by a best-effort load and leave the gate
// silently evaluating nothing. So a load failure yields THIS finding plus a nil
// store, which makes lock.Audit report every locked claim as unapproved. The
// gate fails closed, loudly, and says which of the two stores failed.
const RuleLedgerUnreadable = "lock-ledger-unreadable"

// ledgerInputs is the read-only state the gate is evaluated against: the lock
// ledger, the comment digests, and whichever of the two failed to load.
//
// The two failures are tracked SEPARATELY, and each nils out only its own
// store, because their blast radii are not the same. A nil lock store makes
// every locked claim read as unapproved (correct: without the ledger there is
// no evidence any of them were approved). A nil digest store disables the
// comment rules entirely (also correct — lock.Audit documents that a nil digest
// store means "unknown", never "drifted", because an integrity check must not
// manufacture a finding out of missing evidence). Folding the two together
// would mean a corrupt comment digest accused every locked claim in the project
// of unapproved content, which is a false report, and a gate that files false
// reports is a gate people learn to bypass.
type ledgerInputs struct {
	// store is the lock ledger, or nil if it could not be read.
	store *lock.Store
	// digests is the comment digest store, or nil if it could not be read.
	// Note that a store file that is merely ABSENT is not nil: it loads as an
	// empty store, which reports every claim as unknown rather than drifted.
	digests *digest.Store

	// storeErr / digestErr are the load failures behind a nil above, kept so
	// the finding can name the actual decode error rather than "something went
	// wrong".
	storeErr  error
	digestErr error
}

// loadLedgerInputs reads both stores for cfg out of the WORKING TREE. It is the
// plain (non---staged) path; --staged builds its ledgerInputs from the git index
// instead — see staged.go, and see stagedLedgerInputs for why reading the index
// rather than the worktree is what makes "the claim and its approval must be
// committed together" enforceable.
//
// It deliberately does not take the lock-store sentinel. This is a read of a
// file written atomically (rename-over-path), so a concurrent writer is visible
// as either the old complete store or the new complete one, never a torn one —
// and taking a sentinel here would put a lock acquisition inside the read path
// that "dossierx serve" polls, for no integrity gain at all.
func loadLedgerInputs(cfg *config.Config) ledgerInputs {
	var in ledgerInputs

	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		in.storeErr = err
	} else {
		in.store = store
	}

	digests, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		in.digestErr = err
	} else {
		in.digests = digests
	}

	return in
}

// ledgerGate evaluates every ledger rule over claims and returns the findings,
// unreadable-store findings first (they are the CAUSE of whatever follows, and
// a reader who sees "42 claims are unapproved" without being told the ledger
// failed to parse will draw exactly the wrong conclusion), then lock.Audit's
// own deterministically-ordered output.
//
// An empty result is the only passing verdict. There is no severity here and no
// advisory tier: unlike a lint, every ledger finding is a refusal, because the
// condition each one names is "something changed that nobody approved".
func ledgerGate(claims []model.Claim, in ledgerInputs) []lock.Finding {
	var findings []lock.Finding

	if in.storeErr != nil {
		findings = append(findings, lock.Finding{
			Rule: RuleLedgerUnreadable,
			Message: fmt.Sprintf(
				"the lock ledger could not be read: %v. Every locked claim is reported unapproved below because the gate has no evidence to check them against — restore the ledger from version control rather than re-locking, since re-locking would record whatever the claims say NOW as approved.",
				in.storeErr),
		})
	}
	if in.digestErr != nil {
		findings = append(findings, lock.Finding{
			Rule: RuleLedgerUnreadable,
			Message: fmt.Sprintf(
				"the comment digest store could not be read: %v. Comment-thread drift is NOT being checked on this run — restore the file from version control.",
				in.digestErr),
		})
	}

	return append(findings, lock.Audit(claims, in.store, in.digests)...)
}
