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

	// RuleLockLedgerDowngraded is project-scoped: the lock store says it
	// predates the lock ledger, and the project around it says otherwise.
	//
	// It is the read-only half of the guard AdoptLedger enforces on the write
	// path (see Store.LedgerDowngraded for the evidence and the attack). The two
	// have to exist together: without the write-path half, one edited number
	// re-arms grandfathering and re-blesses tampered content as approved;
	// without THIS half, a read-only command — `check --validate`, `check
	// --staged`, the pre-commit hook, CI — would see the downgraded store, take
	// it for an honest v0.2.x project, grandfather it in memory and report
	// nothing at all.
	//
	// It is reported FIRST and does not replace the per-claim findings under it:
	// the downgrade is the cause, "these locked claims have no standing
	// approval" is what it cost, and a reader needs both to know what to
	// restore.
	RuleLockLedgerDowngraded = "lock-ledger-downgraded"

	// RuleLockLedgerMissing: a locked claim with no ledger record. Something
	// wrote status: locked without going through the approval path — most
	// often a hand edit, which walks straight past the lint gate, hub gating,
	// and the unresolved-comment gate as though all three had passed.
	RuleLockLedgerMissing = "lock-ledger-missing"

	// RuleLockLedgerReleased: a LOCKED claim whose record exists but was
	// RELEASED by an unlock. Unlock deliberately keeps the record and stamps
	// ReleasedAt rather than deleting it, so the evidence a claim was ever
	// locked survives — but that means "a record exists" is satisfied by a
	// record that says the opposite of an approval.
	//
	// Without this rule the whole approval path is bypassable in two ordinary
	// commands and one hand edit: lock, unlock (which releases the record and
	// sets status: draft), then edit "status: draft" back to "status: locked"
	// in the YAML. lock-ledger-missing does not fire, because a record is
	// present. lock-content-drift does not fire either, because
	// LockedClaimHash deliberately EXCLUDES status — so as long as the body is
	// restored to what was approved before the flip, the hash still matches.
	// And lock-ledger-orphan is the mirror case (draft claim, UNreleased
	// record), so it does not fire either. The claim ends up locked, with no
	// standing approval, and every gate silent.
	RuleLockLedgerReleased = "lock-ledger-released"

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

	// RuleLockLedgerAbandoned: a standing (unreleased) CLAIM ledger record whose
	// claim is no longer in the project at all — the claim file was deleted.
	//
	// Every other rule here iterates the CLAIMS and looks for a disagreeing
	// record. A deleted claim has no entry to iterate, so it slipped past all of
	// them: `rm claims/whatever.yaml` on a LOCKED claim produced a completely
	// silent gate, and deleting a claim is the most destructive edit available
	// to anyone holding the repository. This rule is the reverse sweep — the
	// LEDGER is iterated and each standing record is asked whether its claim is
	// still there.
	//
	// It fires only on UNRELEASED records, which is what keeps the honest path
	// quiet: unlock stamps ReleasedAt and leaves the record behind, so
	// unlock-then-delete (a human deciding on the record that the claim should
	// go) is silent, while delete-alone is a refusal.
	RuleLockLedgerAbandoned = "lock-ledger-abandoned"

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
//
// digests is also EVIDENCE ABOUT THE LOCK STORE, which is the second, less
// obvious reason it is passed here. Its presence is what tells an honest
// pre-ledger project (grandfathered in memory, silently — see
// Store.PreLedgerExempt) apart from a lock store that was edited back to a
// pre-ledger version to re-arm that same grandfathering (refused, loudly — see
// Store.LedgerDowngraded). Both are decided once, up front, from the state the
// caller loaded.
func Audit(claims []model.Claim, store *Store, digests *digest.Store) []Finding {
	var findings []Finding
	unrecordedLocks := 0

	// Has this project ever been through a ledger-aware build? The comment
	// digest store's presence is the evidence that does not live in the file
	// being audited, and it is taken from the store the CALLER loaded — so
	// "check --staged" reads the INDEX's answer, exactly as it reads the
	// index's ledger. A nil digest store (one that failed to decode, already
	// reported as lock-ledger-unreadable) is not evidence of anything and is
	// read as absent, which is the conservative direction: it can only make the
	// pre-ledger exemption WIDER, and the exemption's other half — the ledger
	// map itself — still applies.
	digestsPresent := digests != nil && digests.FileExists()

	// preLedgerExempt: this project locked things before this build gave locks a
	// record, so a locked claim without one is an upgrade state, not a tamper.
	// Grandfathering it here — in memory, writing nothing — is what lets the
	// read-only commands pass an honest v0.2.x project instead of accusing every
	// claim in it. See Store.PreLedgerExempt.
	preLedgerExempt := store.PreLedgerExempt(digestsPresent)

	for _, c := range claims {
		record, hasRecord := ledgerRecordFor(store, c.ID)

		switch {
		// The pre-ledger exemption is spent HERE and only here: on a locked
		// claim whose record was never written because the build that locked it
		// had nowhere to write one. Every other rule below needs a record to
		// exist, and a genuinely pre-ledger store has none — so nothing else has
		// to know about the exemption.
		case c.Status == model.StatusLocked && !hasRecord && preLedgerExempt:

		case c.Status == model.StatusLocked && !hasRecord:
			unrecordedLocks++
			findings = append(findings, Finding{
				Rule:    RuleLockLedgerMissing,
				ClaimID: c.ID,
				Message: fmt.Sprintf(
					"claim %q is locked but has no lock-ledger record: its status was set outside the approval path, so the lint gate, hub gating and the unresolved-comment gate never ran on it. Set it back to status: draft and lock it properly (dossierx claim lock %s --reason \"...\").",
					c.ID, c.ID),
			})

		// Evaluated BEFORE the content check, because a released record is the
		// more fundamental fact and the one that explains the state: the
		// content may well still match, since the hash excludes status and a
		// hand-flipped claim is usually flipped back to exactly what was
		// approved. Reporting drift here (when there is none) or nothing at all
		// (which is what shipped) would both send the reader looking for the
		// wrong thing.
		case c.Status == model.StatusLocked && record.Released():
			findings = append(findings, Finding{
				Rule:    RuleLockLedgerReleased,
				ClaimID: c.ID,
				Message: fmt.Sprintf(
					"claim %q is locked, but its only ledger record was RELEASED by an unlock on %s: a released record is a record of an approval that was withdrawn, not a standing approval. Something set status: locked outside the approval path — the content hash cannot see it, because the hash deliberately excludes status. Set it back to status: draft and lock it properly (dossierx claim lock %s --reason \"...\").",
					c.ID, record.ReleasedAt, c.ID),
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

	// The reverse sweep: the ledger's own records, checked against the claims
	// that DO exist. Every rule above starts from a claim, so a claim that was
	// deleted outright is invisible to all of them — see
	// RuleLockLedgerAbandoned.
	//
	// It is skipped entirely when the store file is absent. An absent ledger is
	// already reported once, as lock-ledger-absent, and an in-memory store built
	// by a caller that passed a subset of the project's claims must not have
	// every claim it did not pass reported as deleted.
	if store != nil && store.FileExists() {
		present := make(map[string]bool, len(claims))
		for _, c := range claims {
			present[c.ID] = true
		}
		for key, record := range store.Ledger {
			if record.Subject != SubjectClaim || record.Released() || present[key] {
				continue
			}
			findings = append(findings, Finding{
				Rule:    RuleLockLedgerAbandoned,
				ClaimID: key,
				Message: fmt.Sprintf(
					"claim %q has a standing lock-ledger record from %s (%q) but no longer exists in the project: a LOCKED claim's file was deleted without going through the approval path, so nothing recorded that the human agreed to drop reviewed content. Restore the claim file from version control, or — if the deletion was intended — restore it, run dossierx claim unlock %s --reason \"...\", and delete it again so the release is on the record.",
					key, record.At, record.Reason, key),
			})
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

	// The downgrade is the other project-scoped rule, and it is deliberately
	// evaluated whether or not any claim is locked. A store edited back to a
	// pre-ledger version is a statement about the LEDGER, not about any one
	// claim: reporting it only when it happened to cost a record would let the
	// same edit land quietly in the commit before the one that uses it.
	if store.LedgerDowngraded(digestsPresent) {
		findings = append(findings, Finding{
			Rule: RuleLockLedgerDowngraded,
			Message: fmt.Sprintf(
				"the lock store says it predates the lock ledger (schema version %d), but this project has already been through a ledger-aware build — its comment digest store is present, or the store itself still carries ledger records. A store's own version field is what triggers the one-time grandfathering of already-locked artifacts, so a store that can lower its own version can re-adopt every locked claim as-found and clear any finding against it. Nothing was grandfathered on this run. Restore the lock store from version control; do not re-lock, which would record whatever the claims say NOW as approved.",
				store.OnDiskVersion()),
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
