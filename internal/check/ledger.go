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
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
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

// RuleStoreGitignored is the project-scoped finding for an engine-written path
// under the build directory that .gitignore matches and the index does not
// hold: the lock ledger, the comment digest, the flag store, the build
// directory's own .gitignore, or a module's build-order or code-links artifact.
// A collaborator or CI cloning the project would have no approval record to
// compare against, so `check` reports it as an error-severity finding and the
// approval-recording verbs refuse with error.code store_gitignored. Its
// evidence is git rather than the stores — see gitignore.go's Gitignored —
// but it is declared here, beside every other rule, because FORMAT.md's and
// README's rule tables are derived from this file's Rule* declarations.
const RuleStoreGitignored = "store-gitignored"

// RuleCommentDigestAbsent is project-scoped: this project is already covered by
// the lock ledger, and the COMMENT DIGEST STORE is not there.
//
// It narrows the cheapest bypass in the comment half of the gate. comment-
// ledger-drift compares a claim's comment block against a recorded digest, and a
// claim the store has never seen is "unknown", never "drifted" — correctly, since
// an integrity check must not manufacture a finding out of missing evidence. But
// that made the whole file a delete-to-clear switch: hand-delete an unresolved
// thread (which is how a claim gets past the lock gate with a review still open),
// then delete build/ledger/comment-digest.json in the same commit, and the finding
// that named the edit was gone before any command ran. The lock ledger has
// guarded exactly this shape from the start (lock-ledger-absent, plus AdoptLedger
// refusing to adopt when its file is absent); the digest store had neither half.
//
// The trigger is deliberately NOT "the digest store is missing". It is "missing,
// in a project whose LOCK STORE says it has already been through a ledger-aware
// build" (lock.Store.LedgerCovered). That qualifier keeps it off the one state
// that is innocent: a project upgrading INTO this feature has no digest store
// and has done nothing wrong. Its lock store is still at the pre-ledger version,
// so it is exempt — and lock.CrossPreLedger CREATES the digest store at the very
// moment it stamps that version (as does Store.Save at the moment it creates a
// lock store for a fresh project), so a project crosses both lines together and
// never sees this finding.
//
// IT USED TO HAVE A SECOND QUALIFIER — at least one claim still carrying a
// comment thread — and that qualifier WAS the hole. It was computed from the
// state under audit, so the complete launder cost nothing: delete a claim's only
// thread AND the digest store in the same commit, and the project has no
// comments anywhere, so the count was zero and the rule whose whole job is to
// report the missing store said nothing. The PARTIAL launder was reported and
// the TOTAL one was not — the exact inversion an integrity gate cannot afford,
// and the same shape as "deleting the file is quieter than editing it" that
// lock-ledger-abandoned exists to prevent. A trigger may only be built out of
// evidence that survives the tamper it is looking for; lock coverage survives,
// the claims do not.
//
// What dropping it costs, stated plainly: `check --staged` now refuses a commit
// whose INDEX carries a lock ledger but no digest store beside it — a project
// that never `git add`ed the file the engine wrote for it. That is a real state
// with a one-command fix, and the message names it. The alternative was a rule a
// two-file commit walks straight through.
//
// Recovery is version control, exactly as for lock-ledger-absent: the file is
// tracked, so restoring it is what brings the evidence back. Re-creating it by
// running a comment op would adopt whatever the claims say NOW, which is what an
// attacker wants — internal/comments refuses to adopt in a covered project for
// that reason.
const RuleCommentDigestAbsent = "comment-digest-absent"

// RuleCommentDigestMissing is per-claim, and it is comment-digest-absent's other
// half: the digest STORE is there, and this claim — which holds a STANDING
// (unreleased) lock-ledger record — has no entry in it.
//
// The rule exists because the store was protected against deletion and not
// against being EMPTIED, and emptying it is strictly cheaper to hide in a review
// diff than the `rm` the absence rule catches. The full launder, reproduced:
// unlock a claim, open a human thread on it ("I do not agree…"), and `claim
// lock` correctly refuses with unresolved_comments naming the thread. Then
// hand-delete the `comments:` block from the YAML AND overwrite the digest store
// with `{"version":1,"digests":{}}` — and `claim lock --reason "the human agreed
// offline"` succeeds, writing a REAL, non-grandfathered ledger record, after
// which `check --validate` reports ok:true and no findings. Measured on the same
// tampered claim: delete the FILE -> ok:false ['comment-digest-absent']; leave
// the file and empty the map -> ok:true, [].
//
// COVERAGE, NOT FILE PRESENCE, IS THE TRIGGER, and the predicate is built only
// out of the LEDGER RECORD, which the tamper does not control: every approval
// records the claim's comment digest in the same act that records the approval
// (lock.RecordApproval), so a standing record without an entry is a statement
// about the digest store, not about the claim. It is silent exactly where it
// should be — a project with no ledger coverage at all is not asked (nothing has
// been approved here), an uncommented DRAFT has no record so it is not asked
// either, and a released record describes a claim that is allowed to be out of
// the approval path.
//
// comment-digest-absent stays as the project-scoped CAUSE: when the whole file
// is gone, this rule is suppressed rather than repeating the same cause once per
// claim.
const RuleCommentDigestMissing = "comment-digest-missing"

// RuleCommentDigestAbandoned is the reverse sweep for comments, symmetric with
// lock-ledger-abandoned: a digest entry whose claim id is no longer anywhere in
// the project, and which recorded review history.
//
// It is what makes the RENAME launder visible. Hand-delete a claim's `comments:`
// block alone and comment-ledger-drift fires (correct). Delete the block AND
// change `id:` in the same edit and every rule that starts from the claim went
// quiet, because the claim the store knows about no longer exists and the claim
// that exists is one the store has never seen — verified ok:true, zero findings,
// zero lint errors, after which `claim lock <new id>` succeeded on a claim whose
// human review had been erased. The old id's entry survives that edit, because
// it is not reachable from the file the tamper rewrote, and that is exactly the
// property a trigger has to have.
//
// It does not fire on the two departures that are accounted for — an entry that
// recorded no threads at all, and a claim whose ledger record was released by an
// honest unlock — and lock.SweepCommentDigests drops those entries so they do
// not accumulate. See lock.AbandonedCommentDigests, which owns the predicate for
// both this rule and that sweep so the gate and the sweep cannot disagree.
const RuleCommentDigestAbandoned = "comment-digest-abandoned"

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
	flags   *reaudit.FlagStore

	// storeErr / digestErr are the load failures behind a nil above, kept so
	// the finding can name the actual decode error rather than "something went
	// wrong".
	storeErr  error
	digestErr error
	flagsErr  error

	// THERE ARE NO HISTORY FIELDS HERE ANY MORE, and that is deliberate. This
	// struct used to carry scopeFindings, parentFindings and scopeNote — refusals
	// and one advisory produced by comparing the commit under judgement against
	// its PARENT commit under --staged. The whole comparison was removed; see
	// staged.go's "REMOVED, DELIBERATELY" section for what it caught, what its
	// removal costs (the COORDINATED change — a claim and the record approving it
	// rewritten together, which leaves nothing behind to disagree; earlier notes
	// here put a NUMBER on that cost and every number went stale, so the cost is
	// now stated as the principle it follows from) and why the parent commit is
	// the wrong place to look for evidence about the committer.
	// Every field above is answerable from ONE tree, which is what makes this
	// value mean the same thing on the worktree path and on the index path.
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

	flags, err := reaudit.LoadFlagStore(flagStorePath(cfg))
	if err != nil {
		in.flagsErr = err
	} else {
		in.flags = flags
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

	if f, ok := commentDigestAbsent(claims, in); ok {
		findings = append(findings, f)
	} else {
		// Only when the store is THERE: with the file gone, the finding above is
		// the one cause, and repeating it once per locked claim would bury it.
		findings = append(findings, commentDigestCoverage(claims, in)...)
	}

	findings = append(findings, lock.Audit(claims, in.store, in.digests)...)
	// Build-order artifacts and SubjectBuildOrder ledger rows are leftover
	// from the removed product. They are not client obligations: do not
	// refuse check for stale, drifted, missing, orphan, abandoned, or
	// unreadable sequences, and do not treat a locked leftover order as
	// the pre-ledger "still holds locked artifacts" half.
	return findings
}

// countLockedClaims is lock.Audit's own countLocked, which is unexported there.
// It is three lines and duplicating it is cheaper than exporting a counter whose
// only other caller would be this one.
func countLockedClaims(claims []model.Claim) int {
	n := 0
	for _, c := range claims {
		if c.Status == model.StatusLocked {
			n++
		}
	}
	return n
}

// commentDigestAbsent evaluates RuleCommentDigestAbsent (see it for the whole
// argument). It is reported with the other cause-level findings, before
// lock.Audit's per-claim output, because when it fires it explains why the
// comment rules below said nothing at all.
//
// IT DOES NOT LOOK AT THE CLAIMS. It used to: the rule fired only when at least
// one claim still carried a comment thread, on the reasoning that a project with
// no threads has nothing for the store to record. That qualifier was computed
// from the very state an attacker controls, and it made the TOTAL launder free —
// delete a claim's only thread AND the digest store in one commit, and the count
// is zero, so the rule that exists to report the deleted store stayed silent
// about the deletion that hid the deleted thread. A gate whose trigger is
// derived from the tampered evidence is not a gate.
//
// The remaining qualifier is the one that cannot be tampered into: has this
// project already been through a ledger-aware build (lock.Store.LedgerCovered)?
// A covered project is one whose lock store exists at the ledger schema — and
// this build creates the digest store at the very instant it creates or stamps
// that lock store (lock.Store.Save's ensureCommentDigestStore for a fresh
// project, lock.CrossPreLedger for one crossing onto the ledger), so
// coverage without a digest store is a state the product does not produce. A
// project that predates the ledger, or has never locked anything, is not covered
// and is not asked about.
//
// The case this newly reports is the one flagged in the old note as needing a
// broader test: `check --staged` in a project whose lock store is in the index
// but whose digest store was never `git add`ed. That commit really does carry a
// ledger with no comment evidence beside it, and the fix — stage the file the
// engine already wrote — is one command, which the message names. Refusing it is
// the right side of the trade now that the alternative is a rule with a hole in
// the middle.
// commentDigestCoverage evaluates the two coverage rules that read the digest
// store's CONTENT rather than its existence: comment-digest-missing (a standing
// approval with no entry) and comment-digest-abandoned (an entry with no claim).
//
// They are computed together because they are the two halves of one question —
// does the set of entries still line up with the set of claims the approval path
// has been through? — and because both are silent on exactly the same
// preconditions: an unreadable store (nil, already reported), an absent one (the
// caller reports the project-scoped cause instead), and a project that has never
// been through a ledger-aware build (nothing here has been approved, so there is
// nothing to be covered).
func commentDigestCoverage(claims []model.Claim, in ledgerInputs) []lock.Finding {
	if in.store == nil || in.digests == nil || !in.digests.FileExists() {
		return nil
	}
	if !in.store.LedgerCovered() {
		return nil
	}

	var findings []lock.Finding
	for _, c := range claims {
		record, ok := in.store.Record(c.ID)
		if !ok || record.Subject != lock.SubjectClaim || record.Released() {
			continue
		}
		if _, known := in.digests.Digest(c.ID); known {
			continue
		}
		findings = append(findings, lock.Finding{
			Rule:    RuleCommentDigestMissing,
			ClaimID: c.ID,
			Message: fmt.Sprintf(
				"claim %q holds a standing lock-ledger approval from %s (%q) but has no entry in %s, so its comment threads are not being checked against anything. Every approval records the claim's comment digest in the same act, so an approved claim with no entry means the entry was removed — emptying the map is how an unresolved review is edited away without the deletion of the file itself being reported. Restore %s from version control, or git add it if this commit is the one that updated it. Do NOT run a comment op to re-create the entry: that records whatever the claim says NOW as the truth, which is exactly what removing it was for.",
				c.ID, record.At, record.Reason, config.CommentDigestDisplayPath, config.CommentDigestDisplayPath),
		})
	}

	for _, id := range lock.AbandonedCommentDigests(claims, in.store, in.digests) {
		findings = append(findings, lock.Finding{
			Rule:    RuleCommentDigestAbandoned,
			ClaimID: id,
			Message: fmt.Sprintf(
				"%s records comment threads for claim %q, which is no longer in the project: the claim file was deleted, or its id was changed — and changing the id in the same edit that deletes a comments block is how an unresolved review disappears with nothing reported against the claim that replaces it. Restore the claim (under its recorded id) from version control, or — if the removal was intended — restore it, resolve or delete its threads through dossierx so the removal is on the record, and delete it again.",
				config.CommentDigestDisplayPath, id),
		})
	}
	return findings
}

func commentDigestAbsent(claims []model.Claim, in ledgerInputs) (lock.Finding, bool) {
	// A store that failed to DECODE is a different condition, already reported
	// as lock-ledger-unreadable above; nil here means exactly that.
	if in.digests == nil || in.digests.FileExists() {
		return lock.Finding{}, false
	}
	if !in.store.LedgerCovered() {
		return lock.Finding{}, false
	}
	return lock.Finding{
		Rule: RuleCommentDigestAbsent,
		Message: fmt.Sprintf(
			"this project has a lock ledger but no comment digest store (%s), so comment-thread drift is not being checked AT ALL on this run — for any of its %d claim(s). The engine writes that file the moment a project acquires a lock ledger, so its absence means it was deleted (which is how an edited-away review thread stops being reported, and it stays quiet even when the last thread went with it) or it is not part of this commit. Restore it from version control, or git add it if this commit is the one that created it. Do not re-create it by running a comment op: a re-created store records whatever the claims say NOW as the truth, which is exactly what a deletion was for.",
			config.CommentDigestDisplayPath, len(claims)),
	}, true
}
