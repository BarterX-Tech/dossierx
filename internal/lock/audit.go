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
	// It is the read-only half of the guard CrossPreLedger enforces on the write
	// path (see Store.LedgerDowngraded for the evidence and the attack). The two
	// have to exist together: without the write-path half, one edited number
	// re-opens the crossing and re-blesses tampered content as approved;
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

	// RuleLockLedgerPreLedger is project-scoped: this project's lock store
	// predates the lock ledger, so nothing in it carries an approval record — and
	// this build does not grandfather anything in on its own.
	//
	// It is the visible half of DECISION "the ledger fails closed". The state it
	// reports is the one that used to be SILENT: PrepareStore adopted it on the
	// write path and the pre-ledger predicate grandfathered it in memory on the
	// read path, so a project presenting the pre-ledger shape passed every gate.
	// And presenting that shape is two hand edits — lower "version", delete the
	// "ledger" key — which is why LedgerDowngraded had to be invented, and why it
	// is not enough on its own: delete the comment digest store in the same commit
	// and no evidence in this directory can tell the result from an honest v0.2.x
	// project (locked_at, the only other pre-ledger artifact, shipped in v0.2.0 and
	// looks identical either way — verified against git show v0.2.0).
	//
	// So the answer is not a better predicate but a refusal: this project fails
	// the gate until it holds nothing that predates the ledger, at which point the
	// next lock crosses it (CrossPreLedger). Nothing writes a grandfathered
	// record any more.
	//
	// IT IS EMITTED ONLY WHEN SOMETHING IS ACTUALLY LOCKED, which is a v0.4.0
	// behaviour change from the unconditional emission this comment used to
	// argue for. That argument was: "It fires whether or not any claim is locked:
	// the migration is what puts this store on the ledger schema at all, and a
	// project that skips it while it happens to have nothing locked would simply
	// meet the same refusal later, from a state where a lock had already been
	// refused. One command, run once, and every command after it behaves
	// normally." It died with the migration: there is no command to skip and no
	// later refusal to meet, because a pre-ledger project holding nothing locked
	// crosses silently and correctly on its next lock. Reporting it there would
	// be a finding on correct state, which is how gates get switched off.
	//
	// The condition is therefore the one the WRITE PATH refuses on —
	// countLocked(claims) + lockedBuildOrders > 0 — and it is emitted from two
	// disjoint places, because lock.Audit has no build-order input and
	// structurally cannot have one (internal/buildorder imports internal/lock).
	// This file owns the claims term; internal/check's ledgerGate owns the
	// build-orders-only term. See internal/check/ledger.go.
	//
	// It replaces the per-claim lock-ledger-missing findings for these claims
	// rather than sitting on top of them — one cause, said once, with the recovery
	// that clears it. Repeating "this claim is locked but has no record" N times,
	// each with a recovery that says to set the claim back to draft and re-lock
	// it, is exactly the destructive advice the old exemption existed to avoid
	// giving an honest project.
	RuleLockLedgerPreLedger = "lock-ledger-pre-ledger"

	// RuleLockLedgerMissing: a locked claim with no ledger record. Something
	// wrote status: locked without going through the approval path — most
	// often a hand edit, which walks straight past the lint gate, hub gating,
	// and the unresolved-comment gate as though all three had passed.
	RuleLockLedgerMissing = "lock-ledger-missing"

	// RuleLockLedgerDeleted: a claim THIS ENGINE LOCKED, whose ledger record is
	// gone. It is lock-ledger-missing's sharper twin, and it exists because the
	// gate keyed every one of its rules on a record EXISTING — so deleting one
	// took the claim out of the switch entirely.
	//
	// The attack is two edits inside one file plus one line of YAML, and it was
	// completely silent on HEAD: delete the claim's entry from the "ledger" map,
	// flip its `status: locked` to `status: draft`, and the claim is an ordinary
	// draft again — free to edit, and re-lockable afterwards with an
	// agent-supplied --reason that produces a record indistinguishable from a
	// human's approval. lock-ledger-missing needs status: locked. lock-ledger-
	// orphan needs a record. lock-content-drift needs a record. lock-ledger-absent
	// needs the whole file gone. `check --validate` reported ok:true with zero
	// findings.
	//
	// THE EVIDENCE THE DELETION DOES NOT REACH is in the same store file, one key
	// away: locked_at, which every lock and every confirmed reaudit stamps and
	// which nothing in this build removes (unlock does not; see Unlock), and the
	// claim's own dependency baselines under "hashes", written by the same two
	// operations. Either one says "this engine locked this claim" about a claim
	// the ledger now says nothing about, and the only path that legitimately ends
	// a claim's approval — unlock — KEEPS the record and stamps ReleasedAt on it.
	// So a record that is absent rather than released was deleted by hand.
	//
	// WHAT IT DOES NOT CLOSE, AND THIS BUILD DOES NOT DETECT IT AT ALL — one
	// instance of the principle in "THE BOUNDARY OF THIS GATE" below, at this
	// rule's own target. An attacker who deletes the locked_at entry and the dependency
	// baselines in the SAME edit as the record leaves nothing behind for this rule
	// to read: engineLocked's two keys are exactly the two that went. Verified
	// against this build: delete ledger[id], locked_at[id] and hashes[id] in one
	// edit, flip the YAML to draft, rewrite the body, and Audit returns [] while
	// Store.LedgerRecordDeleted (the write-path gate that should refuse the
	// re-lock) returns false. There is NO rule in this file, and no other gate in
	// the product, that reports it. Do not read the paragraphs above as covering
	// it; they cover the cheaper edit that leaves one of the three keys behind.
	//
	// It used to be reported by AuditAgainstParent, from the parent commit's copy
	// of the store — and that whole comparison was REMOVED on purpose (see
	// internal/check/staged.go's tombstone: the parent commit is outside the
	// COMMIT but not outside the COMMITTER, so --orphan, a second config or a
	// rebase switched it off, and it could not tell an honest `git revert` of a
	// lock commit from an erasure). Do not re-add it. The cost is three keys in a
	// tracked file instead of one, in a diff whose whole purpose is to be read —
	// the same trade the ledger itself makes (see ledger.go's "two deliberate
	// non-goals"), and the trade the whole gate now rests on: DOSSIERX DETECTS,
	// THE FORGE ENFORCES.
	//
	// TWO IN-DIRECTORY EVIDENCE SOURCES WERE TRIED AND REJECTED, recorded here so
	// the next round does not re-derive them:
	//
	//   - OTHER CLAIMS' BASELINES that name this claim as a dependency
	//     (hashes[dependent][id]). Unsound in both directions. Baselines are
	//     recorded for BaselineDependencyIDs = mirrors ++ rests_on ++ a
	//     claim-valued governed_by.type, and a LOCKED claim may
	//     legitimately mirror a DRAFT one — mirror-mismatch compares Layout, Body,
	//     Rows and Steps and documents status as EXPECTED to differ — so the rule
	//     would fire on correct state, which is the outage this gate exists to
	//     avoid. Narrowing it to rests_on does not save it: baselines are never
	//     removed when an edge is removed, so one written while the target sat in
	//     mirrors outlives a later draft edit that moves it to rests_on. And in
	//     the one shape where the inference IS sound (the dependent is currently
	//     locked and currently rests_on the claim) the error-severity
	//     rest-on-locked lint already refuses, so the rule buys nothing there.
	//
	//   - THE BUILD-ORDER RECORD for the claim's module. A standing build-order
	//     record does prove every claim in that module was locked when it was
	//     approved (buildorder.Propose's completeness gate). But it does not say
	//     WHICH claims those were, and authoring a NEW claim in a module whose
	//     build order is locked is ordinary work — the artifact simply reports
	//     stale. The rule would accuse a brand-new draft of having had its record
	//     deleted, and hand it "restore the lock store from version control".
	//     Making it sound needs the covered claim ids ON the record, a store-schema
	//     change its one writer (cmd/dossierx) would have to start supplying.
	RuleLockLedgerDeleted = "lock-ledger-deleted"

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
	// that covers the ten fields ContentHash cannot see — headed by
	// raw_html_reviewed, the flag that promotes an allowlisted claim's
	// raw_html to trusted, unescaped output.
	//
	// sources and tracks joined that list when they were added to the schema,
	// and sources is the clearest case for this rule since raw_html_reviewed:
	// a claim's evidence is exactly what a reader re-validates months later,
	// so a citation swapped after approval is a changed claim even though
	// nothing it PROMISES moved. That is also why ContentHash stays blind to
	// it — provenance is not contract, and editing a citation must not flip
	// every dependent to review_pending.
	//
	// raw_html itself moved off that list in v0.4.1, when ContentHash took it
	// on so an edited attachment marks dependents stale (see lock.ContentHash).
	// That does not weaken this rule: being stale is not being tampered with,
	// and LockedClaimHash signs every persisted field regardless of what any
	// staleness baseline happens to cover.
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

	// RuleCommentDigestUnrecorded: a claim that CARRIES COMMENT THREADS and has
	// no entry in the comment digest store, in a project the lock ledger already
	// covers.
	//
	// It closes comment-ledger-drift's one escape. That rule compares a claim's
	// block against a RECORDED digest, and a claim the store has never seen is
	// "unknown", never "drifted" — correctly, since an integrity check must not
	// manufacture a finding out of missing evidence. But "unknown" was reachable
	// by deleting ONE key: on a fully covered project with the store present and
	// readable (so comment-digest-absent cannot fire), forge `status: resolved,
	// resolved_by: human` onto a human's open thread — `check` reports
	// comment-ledger-drift — then drop that claim's key from the "digests" map and
	// the finding is gone, the claim locks, and the sweep re-adopts the forged
	// block on the next ordinary command. The gate the whole review loop rests on
	// (a human's open objection blocks the lock) was cleared by an edit smaller
	// than the one it was catching.
	//
	// THE PREDICATE IS THE THREADS THEMSELVES, which is what makes it survive the
	// tamper: comments are engine-managed, and the single path that writes a
	// thread into a claim file (internal/comments' mutate) records the claim's
	// digest in the same act. So in a covered project, a claim holding threads with
	// no entry beside them has exactly two explanations — the entry was removed, or
	// the threads were never written by the engine — and both are the finding.
	//
	// THE ONE THING IT CANNOT SEE, AND NOTHING IN THIS BUILD SEES IT EITHER — one
	// instance of the principle in "THE BOUNDARY OF THIS GATE" below, at this
	// rule's own target. The predicate is the threads, so ERASING THE THREADS TOO takes the
	// claim out of this rule's own evidence set. Erasing the block ALONE is not
	// silent — the surviving entry recorded threads, the claim now hashes to the
	// empty digest, and comment-ledger-drift fires (verified). Dropping the key
	// ALONE is not silent either — that is this rule. What is silent, completely,
	// is erasing BOTH, two deletions in one change:
	//
	//	delete the `comments:` block from the claim YAML
	//	delete the claim's key from the "digests" map
	//	dossierx check   ok:true, [] — and the sweep then adopts the claim at its
	//	                 EMPTY digest, so the state is clean permanently
	//
	// A human's open thread on a DRAFT claim is the sharpest target: an open
	// thread is what BLOCKS the lock, so erasing it is exactly what the attack
	// buys, and the claim locks afterwards with a clean approval record and a
	// review nobody had.
	//
	// IT IS CONFINED TO DRAFT CLAIMS, and the confinement comes from the digest
	// rules' trigger rather than from the lock hash. check.RuleCommentDigest-
	// Missing keys on a STANDING (unreleased) lock-ledger record that has no
	// entry in the digest store: a LOCKED claim has such a record, so the
	// dropped key is reported; a DRAFT claim has none, so the rule is never
	// asked — and that silence is the whole of this gap. Measured on a locked
	// claim: erasing the comments block alone gives comment-ledger-drift,
	// erasing it together with its digest key gives comment-digest-missing.
	//
	// lock-content-drift is NOT what catches it, and saying so would send the
	// next reader to the wrong file. `comments` is one of the three fields
	// lockedClaimHashExcluded removes from the locked-claim hash, deliberately,
	// because dossierx serve writes comments and has no write authority over
	// the lock store — comment integrity is the digest store's job precisely so
	// the lock hash does not have to carry it. No comments edit, on any claim in
	// any status, can produce lock-content-drift.
	//
	// It used to be reported by AuditAgainstParent, whose evidence was the PARENT
	// COMMIT's digest entry; that comparison was removed on purpose and must not
	// be re-added (see RuleLockLedgerDeleted's paragraph on it, and
	// internal/check/staged.go's tombstone). No single-tree replacement is
	// available and this is not for want of looking: "a draft with no threads and
	// no digest entry" is exactly what most drafts look like, so a rule that
	// refused it would refuse almost every uncommented draft in every project.
	// What catches it is the diff — two coordinated deletions in tracked files,
	// under branch protection and review.
	//
	// It is deliberately silent where the evidence is honestly absent: a project
	// with no ledger coverage at all (nothing here has been approved), an absent
	// digest store (comment-digest-absent is the one cause, said once), a claim
	// with NO threads (adopted at its empty digest, which is what makes a
	// hand-added thread report as drift later), and a claim holding a STANDING
	// approval — that one is internal/check's comment-digest-missing, built on the
	// ledger record instead, and reporting both would name one state twice.
	RuleCommentDigestUnrecorded = "comment-digest-unrecorded"
)

// ---------------------------------------------------------------------
// THE BOUNDARY OF THIS GATE
// ---------------------------------------------------------------------
//
// AN IN-REPO LEDGER CANNOT ATTEST ANYTHING AGAINST THE PERSON WHO CAN WRITE IT.
//
// That sentence is the boundary, and it is stated as a principle because the
// alternative was tried and failed. Earlier revisions of this comment ENUMERATED
// the arrangements this gate does not detect. The count went one, then two, then
// three, then "at least six" — it went stale every time somebody looked harder —
// and two of the enumerated entries were, on inspection, simply false. A list
// that keeps growing and is sometimes wrong is worse than no list, because the
// next reader believes it is complete. So: no list, and no number.
//
// WHAT THE PRINCIPLE MEANS HERE. The claim files, the lock store, the digest
// store and the build-order artifacts are this gate's entire evidence base, and
// every one of them is a tracked file in the tree the committer is editing. The
// line therefore falls between UNCOORDINATED and COORDINATED change:
//
//   - UNCOORDINATED — one file edited, one record removed, one status flipped,
//     one thread erased. The files that were NOT touched disagree with the one
//     that was, and the disagreement is the finding. Every rule above is one of
//     those, and that is what a drifting agent, a careless hand-edit and a bad
//     merge all produce.
//   - COORDINATED — a claim and the record approving it written TOGETHER, by
//     someone entitled to write both. Nothing is left over to disagree, so Audit
//     returns [] and is right to: there is no evidence in the tree that anything
//     is wrong.
//
// The per-rule paragraphs above each say where their own rule's evidence runs
// out (see RuleLockLedgerDeleted and RuleCommentDigestUnrecorded). Read those as
// instances of this principle, not as an inventory of it.
//
// WHY THERE IS NO RULE FOR IT. The coordinated case was once partly covered by
// comparing the commit under judgement against its PARENT — `check --staged`'s
// history half and this file's AuditAgainstParent. That machinery was REMOVED,
// deliberately, and must not be re-added: the parent commit is outside the
// COMMIT but not outside the COMMITTER, so `git checkout --orphan`, a second
// config file or a rebase all move the other side of the comparison, and it
// could not tell a legitimate `git revert` of a lock commit from an erasure — so
// it refused ordinary git work. internal/check/staged.go carries the full
// tombstone.
//
// And there is no cheap single-tree replacement, which is worth saying plainly
// so nobody spends a day looking for one: "no claims in scope and no ledger" is
// also exactly what a brand-new project looks like, and "a draft with no threads
// and no digest entry" is exactly what most drafts look like. A rule that
// refused those would refuse every project's first commit and every uncommented
// draft — the outage this gate exists to avoid.
//
// THE ACCEPTED MODEL, therefore: DOSSIERX DETECTS, THE FORGE ENFORCES. Branch
// protection, a required CI check running this gate, and human review are what
// stand between a repository and a coordinated change. Moving the boundary
// rather than working around it needs evidence outside the COMMITTER as well as
// outside the commit — a signature, or a server-side record — not another read
// of the same person's git history. FORMAT.md's "What the gate detects, what it
// does not, and where the rest is caught" is the canonical statement, with one
// worked example and an explicit note that it is an example rather than a list.

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
// pre-ledger project (exempt from the per-claim rules — see
// Store.PreLedgerUnadopted) apart from a lock store that was edited back to a
// pre-ledger version to re-arm the old grandfathering (refused, loudly — see
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
	// read as absent, which is the conservative direction: it can only widen the
	// set of projects offered the migration, and the other half of the evidence —
	// the ledger map itself — still applies.
	digestsPresent := digests != nil && digests.FileExists()

	// preLedgerUnadopted: this project locked things before this build gave locks
	// a record, and it has not crossed onto the ledger yet. Its locked claims
	// are NOT accused one by one (that recovery text is destructive advice for a
	// project that has done nothing wrong) and they are NOT grandfathered in
	// memory either, which is what used to happen and what made the whole state
	// silent. They are reported once, project-scoped, with the recovery that
	// clears it. See Store.PreLedgerUnadopted and RuleLockLedgerPreLedger.
	preLedgerUnadopted := store.PreLedgerUnadopted(digestsPresent)

	// covered: the ledger is in force here — the store is on disk at the ledger
	// schema, which in this build happens only because CrossPreLedger or a real
	// approval put it there. It arms the two rules whose evidence is only
	// meaningful once every locked claim is supposed to have a record:
	// lock-ledger-deleted and comment-digest-unrecorded. A downgraded store is
	// deliberately NOT covered: it gets its own project-scoped finding below, and
	// piling per-claim findings on top of it would bury the one that names the
	// cause.
	covered := store.LedgerCovered()

	for _, c := range claims {
		record, hasRecord := ledgerRecordFor(store, c.ID)

		switch {
		// The pre-ledger project is spent HERE and only here: on a claim whose
		// record was never written because the build that locked it had nowhere
		// to write one. Every other rule below needs a record to exist, and a
		// genuinely pre-ledger store has none — so nothing else has to know.
		case !hasRecord && preLedgerUnadopted:

		// Evaluated BEFORE lock-ledger-missing, and for both lock states,
		// because it is the more precise diagnosis wherever it applies: the
		// claim did not merely arrive at status: locked without a record, it HAD
		// a record and no longer does. The two need different recoveries —
		// missing says "lock it properly", deleted says "restore the store" —
		// and giving the second one the first's advice would tell a human to
		// re-lock content whose approved bytes are sitting in version control.
		case !hasRecord && covered && engineLocked(store, c.ID):
			findings = append(findings, Finding{
				Rule:    RuleLockLedgerDeleted,
				ClaimID: c.ID,
				Message: fmt.Sprintf(
					"claim %q was locked by dossierx itself — the lock store still carries its own locked_at stamp and/or its dependency baselines — but its lock-ledger record is GONE. Nothing in this build deletes a record: unlock KEEPS it and stamps released_at on it, precisely so the evidence survives. A record that is absent rather than released was deleted by hand, which is how a locked claim is turned back into a freely editable draft (status: %s here) and re-locked later with a reason that reads exactly like a human's approval. Restore the lock store from version control; do NOT re-lock, which would record whatever the claim says NOW as approved.",
					c.ID, c.Status),
			})

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
			recorded, known := digests.Digest(c.ID)
			switch {
			case known && recorded != digest.CommentsDigest(c):
				findings = append(findings, Finding{
					Rule:    RuleCommentLedgerDrift,
					ClaimID: c.ID,
					Message: fmt.Sprintf(
						"claim %q's comment threads were changed outside dossierx. Comments are engine-managed: add, reply, resolve and reopen through the CLI or the viewer, so the review history stays intact and an unresolved thread cannot be edited away.",
						c.ID),
				})

			// The drift rule's escape hatch, closed: an entry that is DELETED
			// rather than edited takes the claim from "drifted" to "unknown",
			// and unknown is silent. See RuleCommentDigestUnrecorded for the
			// reproduction and for why the threads themselves are the trigger.
			// The standing-approval case is excluded because internal/check's
			// comment-digest-missing already names it from the ledger record's
			// side; naming one state twice teaches people to skim the list.
			case !known && covered && digestsPresent && len(c.Comments) > 0 &&
				!(hasRecord && !record.Released()):
				findings = append(findings, Finding{
					Rule:    RuleCommentDigestUnrecorded,
					ClaimID: c.ID,
					Message: fmt.Sprintf(
						"claim %q carries %d comment thread(s) but has no entry in %s, so they are not being checked against anything — a forged or edited thread on this claim would read as unknown rather than drifted, and an unresolved objection edited away would not be reported at all. Comments are engine-managed: the one path that writes a thread records the claim's digest in the same act, so threads without an entry mean the entry was removed or the threads were written by hand. Restore %s from version control (or git add it, if this commit is the one that updated it). Do NOT run a comment op to re-create the entry: that records whatever the claim says NOW as the truth, which is exactly what removing it was for.",
						c.ID, len(c.Comments), digest.StoreFileName, digest.StoreFileName),
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

	// The pre-ledger project, reported once for the whole project, and ONLY when
	// this project still holds a locked CLAIM.
	//
	// The claims-only condition is not an oversight: this function has no
	// build-order input and structurally cannot have one, so the other half of
	// the union — a pre-ledger project holding a locked BUILD ORDER and zero
	// locked claims — is emitted by internal/check's ledgerGate, which holds both
	// inputs. The two conditions are mutually exclusive, so the finding appears
	// exactly once in every state. See RuleLockLedgerPreLedger for why the
	// emission is conditional at all.
	//
	// It is DELIBERATELY NOT accompanied by the per-claim findings for the same
	// claims (see the switch above): the cause is the project's, the recovery is
	// the project's, and lock-ledger-missing's own advice — set it back to draft
	// and re-lock — would destroy the very approvals the crossing preserves.
	if lockedClaims := countLocked(claims); preLedgerUnadopted && lockedClaims > 0 {
		findings = append(findings, PreLedgerFinding(store, lockedClaims, 0))
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
				"the lock store says it predates the lock ledger (schema version %d), but this project has already been through a ledger-aware build — its comment digest store is present, or the store itself still carries the ledger key (a key that did not exist before the ledger, with or without records in it). A store's own version field is what triggers the one-time grandfathering of already-locked artifacts, so a store that can lower its own version can re-adopt every locked claim as-found and clear any finding against it. Nothing was grandfathered on this run. Restore the lock store from version control; do not re-lock, which would record whatever the claims say NOW as approved.",
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

// PreLedgerFinding builds the project-scoped RuleLockLedgerPreLedger finding.
//
// It is exported because the finding is emitted from TWO places — this package,
// for the locked-CLAIMS half, and internal/check's ledgerGate for the locked-
// BUILD-ORDERS-only half, which is the only package that holds both inputs. One
// constructor rather than two restatements is what keeps the two emitters from
// producing different bytes for the same condition.
//
// The count is rendered as both terms so a reader can see WHICH half fired.
func PreLedgerFinding(store *Store, lockedClaims, lockedBuildOrders int) Finding {
	version := 0
	if store != nil {
		version = store.OnDiskVersion()
	}
	return Finding{
		Rule: RuleLockLedgerPreLedger,
		Message: fmt.Sprintf(
			"this project's lock store predates the lock ledger (schema version %d), so %d locked claim(s) and %d locked build order(s) here have no approval record — and nothing can attest to content no ledger ever recorded. There is no automatic adoption and no migration command any more.\n\n%s\n\nCommit the updated %s and %s with the re-locks.",
			version, lockedClaims, lockedBuildOrders, preLedgerCrossingSteps, StoreFileName, digest.StoreFileName),
	}
}

// engineLocked reports whether the STORE says this engine locked claim id at
// some point, from the two pieces of bookkeeping a deleted ledger record leaves
// behind (see RuleLockLedgerDeleted):
//
//   - locked_at, stamped by every Lock and every confirmed reaudit
//     (RefreshBaseline), and removed by nothing — not even unlock, which is what
//     makes it evidence rather than a lock-state mirror.
//   - the claim's own per-dependent dependency baselines, written by the same two
//     operations, which is a second key an attacker has to remember. It is only
//     present for a claim that HAS dependencies, so it widens the rule rather
//     than replacing locked_at.
//
// Neither is proof of an approval — that is what the ledger is for. Both are
// proof that a record ONCE EXISTED, which is the only question this rule asks.
func engineLocked(s *Store, id string) bool {
	if s == nil {
		return false
	}
	if _, ok := s.LockedAt[id]; ok {
		return true
	}
	return len(s.Hashes[id]) > 0
}

// countLocked counts the locked claims in the set under audit — for the
// pre-ledger finding (see PreLedgerFinding), which is more useful when it says
// how much of the project still predates the ledger than when it says only that
// something does.
func countLocked(claims []model.Claim) int {
	n := 0
	for _, c := range claims {
		if c.Status == model.StatusLocked {
			n++
		}
	}
	return n
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
