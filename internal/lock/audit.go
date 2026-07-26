// audit.go implements the LEDGER GATE: the read-only rules that compare the
// claims on disk against the lock ledger and the comment digest, and name every
// disagreement.
//
// This is a gate, NOT a lint, and the distinction is a design decision the
// release depends on. A lint is registered in lint.Registry, which means it runs
// inside "dossierx check" and inside lock.Lock's own refusal gate, and an
// error-severity finding there stops the whole pipeline: no catalog, no viewer,
// and no claim in the project can be locked. One tampered claim would take the
// documentation offline for everybody, which is a denial-of-service handed to
// whoever edits a YAML file wrong. So the ledger rules live here, are evaluated
// by the pre-commit hook and by CI, and refuse a COMMIT rather than a render.
//
// Every rule is read-only by construction: Audit takes loaded state and returns
// findings. It never writes, never adopts, and never repairs — repairing is what
// an attacker would want it to do.
package lock

import (
	"fmt"
	"sort"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// The gate's rule names. They are stable strings: the hook, CI, and the agent
// skills all branch on them, so they are contract, not prose.
const (
	// RuleLockLedgerAbsent is project-scoped: locked claims exist but the lock
	// store file does not. This is the "delete the ledger to re-bless
	// everything" case, reported separately from the per-claim findings
	// because the recovery is different — you cannot fix it claim by claim,
	// you have to restore the ledger from version control.
	RuleLockLedgerAbsent = "lock-ledger-absent"

	// RuleLockLedgerMissing: a locked claim with no ledger record. Something
	// wrote status: locked without going through the approval path — most
	// often a hand edit, which walks straight past the lint gate, hub gating,
	// and the unresolved-comment gate as though all three had passed.
	RuleLockLedgerMissing = "lock-ledger-missing"

	// RuleLockContentDrift: a locked claim whose current content no longer
	// hashes to what its ledger record says was approved. This is the rule
	// that covers the nine fields ContentHash cannot see — including raw_html
	// on an allowlisted, reviewed mockup, the only path in this codebase that
	// renders author bytes unescaped.
	RuleLockContentDrift = "lock-content-drift"

	// RuleLockLedgerOrphan: a DRAFT claim holding an unreleased ledger record
	// — it was locked, and something flipped it back to draft without going
	// through unlock. That is the cheapest way to dodge review: a draft claim
	// is edited freely and can be re-locked later, and before the ledger there
	// was nothing at all to notice it had ever been locked.
	RuleLockLedgerOrphan = "lock-ledger-orphan"

	// RuleCommentLedgerDrift: a claim whose comment block no longer matches
	// the digest recorded at the engine's last comment write — a thread or
	// reply added, removed, or rewritten outside the engine. Deleting an
	// unresolved thread by hand is how a claim gets past the lock gate with a
	// review still open.
	RuleCommentLedgerDrift = "comment-ledger-drift"
)

// Finding is one ledger-gate disagreement. There is no severity field: unlike a
// lint, every finding here is a refusal. A gate that reported advisory
// integrity failures would be a gate nobody reads.
type Finding struct {
	// Rule is one of the Rule* constants above.
	Rule string `json:"rule"`
	// ClaimID is the claim the finding is about, or "" for a project-scoped
	// finding (RuleLockLedgerAbsent).
	ClaimID string `json:"claim_id,omitempty"`
	// Message is the human-facing sentence, written to be actionable on its
	// own: what was found, and what to do about it.
	Message string `json:"message"`
}

// Audit evaluates every ledger rule over claims and returns the findings, in a
// deterministic order (claim id, then rule name), so a hook's output and a CI
// log can be diffed against each other.
//
// digests may be nil — a caller that has not loaded the comment digest store
// simply gets no comment rules, rather than a false accusation that every
// claim's comments have drifted. That distinction (unknown vs. drifted) is the
// same one Store.Baseline draws for dependency hashes, and for the same reason:
// an integrity check must never manufacture a finding out of missing evidence.
func Audit(claims []model.Claim, store *Store, digests *digest.Store) []Finding {
	var findings []Finding
	unrecordedLocks := 0

	for _, c := range claims {
		record, hasRecord := ledgerRecordFor(store, c.ID)

		switch {
		case c.Status == model.StatusLocked && !hasRecord:
			unrecordedLocks++
			findings = append(findings, Finding{
				Rule:    RuleLockLedgerMissing,
				ClaimID: c.ID,
				Message: fmt.Sprintf(
					"claim %q is locked but has no lock-ledger record: its status was set outside the approval path, so the lint gate, hub gating and the unresolved-comment gate never ran on it. Set it back to status: draft and lock it properly (dossierx claim lock %s --reason \"...\").",
					c.ID, c.ID),
			})

		case c.Status == model.StatusLocked && record.Hash != LockedClaimHash(c):
			findings = append(findings, Finding{
				Rule:    RuleLockContentDrift,
				ClaimID: c.ID,
				Message: fmt.Sprintf(
					"claim %q is locked but its content no longer matches what was approved on %s (%q). Locked claims change through unlock -> edit -> lock, not by editing the file: revert the edit, or run dossierx claim unlock %s --reason \"...\", make the change, and lock it again.",
					c.ID, record.At, record.Reason, c.ID),
			})

		case c.Status != model.StatusLocked && hasRecord && !record.Released():
			findings = append(findings, Finding{
				Rule:    RuleLockLedgerOrphan,
				ClaimID: c.ID,
				Message: fmt.Sprintf(
					"claim %q is draft but still holds an active lock-ledger record from %s (%q): it was unlocked without going through dossierx claim unlock, which is how a locked claim gets edited without review. Restore status: locked, or unlock it properly (dossierx claim unlock %s --reason \"...\").",
					c.ID, record.At, record.Reason, c.ID),
			})
		}

		// Comment drift is independent of lock state on purpose: an unlocked
		// claim's review history matters just as much, and the window right
		// after an unlock is exactly when supervision is weakest.
		if digests != nil {
			if recorded, known := digests.Digest(c.ID); known && recorded != digest.CommentsDigest(c) {
				findings = append(findings, Finding{
					Rule:    RuleCommentLedgerDrift,
					ClaimID: c.ID,
					Message: fmt.Sprintf(
						"claim %q's comment threads were changed outside dossierx. Comments are engine-managed: add, reply, resolve and reopen through the CLI or the viewer, so the review history stays intact and an unresolved thread cannot be edited away.",
						c.ID),
				})
			}
		}
	}

	// The project-scoped rule is decided LAST but reported FIRST. Its trigger is
	// "the store file is gone AND that absence actually cost us records" rather
	// than "the file is gone" alone: without the second half it would fire on
	// any caller auditing against an in-memory store, and a rule that fires on
	// correct state is a rule people learn to ignore. When it does fire, every
	// lock-ledger-missing above is a symptom of this one cause, and saying so
	// once up front is what keeps the report readable — and what stops someone
	// "fixing" a deleted ledger by re-locking every claim, which would record
	// whatever the files say NOW as approved.
	if store != nil && !store.FileExists() && unrecordedLocks > 0 {
		findings = append(findings, Finding{
			Rule: RuleLockLedgerAbsent,
			Message: fmt.Sprintf(
				"%d claim(s) are locked but the lock ledger is missing entirely. The ledger is a tracked, committed file: restore it from version control rather than re-locking, because re-locking would record whatever the claims say NOW as approved.",
				unrecordedLocks),
		})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].ClaimID != findings[j].ClaimID {
			return findings[i].ClaimID < findings[j].ClaimID
		}
		return findings[i].Rule < findings[j].Rule
	})
	return findings
}

// ledgerRecordFor returns the CLAIM record for id, if any. It filters on
// Subject rather than trusting the key: a build-order record must never be
// read as a claim's approval, and checking the field (instead of parsing the
// key's shape) means a subject kind added later cannot silently start being
// audited by these rules.
func ledgerRecordFor(store *Store, id string) (LedgerRecord, bool) {
	if store == nil {
		return LedgerRecord{}, false
	}
	r, ok := store.Record(id)
	if !ok || r.Subject != SubjectClaim {
		return LedgerRecord{}, false
	}
	return r, true
}
