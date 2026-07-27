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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
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

// RuleCommentDigestAbsent is project-scoped: this project is already covered by
// the lock ledger, and the COMMENT DIGEST STORE is not there.
//
// It narrows the cheapest bypass in the comment half of the gate. comment-
// ledger-drift compares a claim's comment block against a recorded digest, and a
// claim the store has never seen is "unknown", never "drifted" — correctly, since
// an integrity check must not manufacture a finding out of missing evidence. But
// that made the whole file a delete-to-clear switch: hand-delete an unresolved
// thread (which is how a claim gets past the lock gate with a review still open),
// then delete .dossierx-comment-digest.json in the same commit, and the finding
// that named the edit was gone before any command ran. The lock ledger has
// guarded exactly this shape from the start (lock-ledger-absent, plus AdoptLedger
// refusing to adopt when its file is absent); the digest store had neither half.
//
// The trigger is deliberately NOT "the digest store is missing". It is "missing,
// in a project whose LOCK STORE says it has already been through a ledger-aware
// build" (lock.Store.LedgerCovered). That qualifier keeps it off the one state
// that is innocent: a project upgrading INTO this feature has no digest store
// and has done nothing wrong. Its lock store is still at the pre-ledger version,
// so it is exempt — and lock.PrepareStore CREATES the digest store at the very
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

// The BUILD-ORDER ledger rules.
//
// They live here, not in internal/lock, for a structural reason: internal/
// buildorder imports internal/lock (for ContentHash), so lock cannot import
// buildorder back to read an artifact. internal/check imports both, which makes
// it the only place the record and the artifact can be put side by side. They
// are stable strings on the same terms as lock's own rule names.
//
// WHY THEY HAD TO EXIST. "dossierx build-order lock" already WROTE a ledger
// record for the artifact it froze (cmd/dossierx.recordBuildOrderApproval), and
// FORMAT.md already called a locked build order a locked artifact inside the
// same gate as a locked claim. Nothing read the record back. lock.Audit
// deliberately filters on Subject == SubjectClaim, so the build-order records it
// walked past were the only records in the ledger that no rule could ever fire
// on — the write was pure ceremony, and a hand-edited .build-order.<module>.json
// (which is the implementation sequence an agent then follows) was exactly as
// invisible as it had been before the ledger shipped. The release's headline
// invariant covered half the locked artifacts in a project.
const (
	// RuleBuildOrderContentDrift: a LOCKED build-order artifact whose current
	// content no longer hashes to what its ledger record says was approved.
	// The artifact's phases ARE the implementation order, so an edit here
	// reorders what gets built without touching a single claim.
	RuleBuildOrderContentDrift = "build-order-content-drift"

	// RuleBuildOrderLedgerMissing: a LOCKED build-order artifact with no ledger
	// record at all — the `locked: true` flag was written outside the approval
	// path, or the record was deleted from the ledger to clear a drift finding.
	// Symmetric with lock-ledger-missing for claims, and for the same reason: a
	// gate that only catches EDITED approvals is bypassed by removing the
	// approval.
	RuleBuildOrderLedgerMissing = "build-order-ledger-missing"

	// RuleBuildOrderLedgerAbandoned: a standing (unreleased) BUILD-ORDER ledger
	// record whose artifact is no longer there — the file was deleted, or the
	// module was removed from project.config.yaml's modules list so nothing
	// audits it any more.
	//
	// It is the reverse sweep, and it exists for exactly the reason
	// lock-ledger-abandoned exists on the claim side. Both rules above start
	// from a FILE and look for a disagreeing record, so the one edit neither can
	// see is deleting the file: `rm .build-order.widget.json` left the standing
	// record in the ledger and every gate silent, which made deletion strictly
	// quieter than editing — the inversion an integrity gate can least afford.
	//
	// It fires only on UNRELEASED records (see lock.ReleaseBuildOrderApproval),
	// so a build order a human deliberately released stays silent.
	RuleBuildOrderLedgerAbandoned = "build-order-ledger-abandoned"

	// RuleBuildOrderLedgerOrphan: an artifact whose own `"locked"` flag says
	// false while its ledger record still stands unreleased.
	//
	// It is the build-order twin of lock-ledger-orphan, and it closes the
	// cheapest bypass in this whole gate: one boolean, in the audited file,
	// disarmed every rule above it. Both forward rules skip an unlocked artifact
	// (correctly — an unlocked artifact is a proposal nobody has approved), and
	// the reverse sweep skips one that is present (correctly — it is the forward
	// loop's business). So editing `"locked": true` to `"locked": false`
	// removed the artifact from every rule's evidence set at once, and `check`
	// reported ok. The approved implementation sequence is still sitting there
	// for an agent to follow, and the ledger still says a human approved it; the
	// only thing that changed is the flag that decides whether anyone checks.
	//
	// The predicate is "unlocked artifact + STANDING record", with no exception,
	// and what makes that safe to state so plainly is that the honest
	// re-propose window no longer produces it. "build-order propose" now
	// RELEASES the module's record as part of overwriting a locked order with a
	// fresh unlocked one (see lock.ReleaseBuildOrderApproval and the propose
	// command's call to it) — which is simply the truth being written down: the
	// approved order it vouched for is gone, replaced by a proposal nobody has
	// approved. A released record is not standing, so every commit between the
	// two halves of the documented propose-then-lock flow stays silent.
	//
	// The predicate USED to be exact instead — re-sign the artifact as if its
	// locked flag were still true and require that hash to match the record —
	// because without the release there was no other way to tell the honest
	// window apart from a hand flip. That exactness was itself the hole: it
	// caught flipping the flag ALONE, and missed flipping the flag AND editing
	// the phases in the same edit, since a content edit re-signs to something
	// else and was therefore indistinguishable from a re-proposal. The strictly
	// more damaging attack was the one that got through. Wiring the release into
	// propose is what let the exception go, and with it that gap.
	RuleBuildOrderLedgerOrphan = "build-order-ledger-orphan"

	// RuleBuildOrderUnreadable: a build-order artifact file that IS there and
	// cannot be decoded (or cannot be hashed once decoded).
	//
	// It is the build-order twin of RuleLedgerUnreadable, and it exists because
	// corrupting the approved implementation sequence was strictly QUIETER than
	// deleting it. Deleting the artifact is caught by the reverse sweep
	// (build-order-ledger-abandoned, a refusal); truncating the same file
	// mid-token — `{ "module": "widget", "locked": tr` — left
	// collectBuildOrderStates with Present=false and Unreadable=true, which the
	// forward loop skips (it audits present artifacts) and the reverse sweep also
	// skips (an unreadable file is not evidence of deletion). Nothing else read
	// the flag: its doc comment deferred the case to "check's own build-order
	// reporting", which was never built. So `check` exited 0 over a destroyed
	// implementation sequence.
	//
	// It is its OWN rule rather than folded into either existing one precisely
	// because it is neither: the artifact was not deleted, and its content cannot
	// be compared to anything. The recovery is the lock store's recovery, for the
	// same reason — restore the file from version control, never re-propose,
	// which would record whatever the claims say NOW as the approved order.
	RuleBuildOrderUnreadable = "build-order-unreadable"
)

// buildOrderState is one module's build-order artifact as the gate's evidence
// source found it: which module, where a human can open it, whether an artifact
// is there at all, whether it says it is locked, and the signature of the bytes
// that are actually there.
//
// It is a value rather than a *buildorder.Artifact so the two evidence sources —
// the working tree and the git index — can both produce it, which is what lets
// --staged audit build orders on the same terms it audits claims. Reading the
// artifact from the worktree while reading the ledger from the index would
// resurrect, for build orders, exactly the hole staged.go exists to close.
//
// It records the ABSENT and UNREADABLE cases instead of dropping them, which the
// forward rules never needed and the reverse sweep cannot work without: the
// sweep's whole question is "the ledger says this module has an approved build
// order — is it still there?", and a collection that silently omitted the
// missing ones could only ever answer yes.
type buildOrderState struct {
	Module string
	Path   string
	Hash   string

	// LockedHash is the signature this artifact WOULD have if its own `locked`
	// flag said true, with every other byte left exactly as found. For a locked
	// artifact it is identical to Hash; for an unlocked one it is what makes
	// "somebody flipped one boolean" separable from "somebody re-proposed",
	// which is the whole of RuleBuildOrderLedgerOrphan.
	LockedHash string

	// Present is true when an artifact file was found and decoded.
	Present bool
	// Locked is the artifact's own locked flag (meaningful only when Present).
	Locked bool
	// Unreadable is true when a file IS there but could not be read, decoded or
	// hashed. Distinct from !Present on purpose: absence is evidence about the
	// ledger, a corrupt file is not (see collectBuildOrderStates) — so it stays
	// out of the reverse sweep and is reported by its own rule,
	// RuleBuildOrderUnreadable, in the forward loop. It used to be audited by
	// NOTHING, on the strength of a doc comment deferring it to a reporter that
	// was never built, which made corrupting an approved implementation sequence
	// quieter than deleting it.
	Unreadable bool

	// Err is the decode/hash failure behind Unreadable, kept so the finding can
	// name the actual error rather than "something went wrong" — the same reason
	// ledgerInputs keeps storeErr/digestErr.
	Err error
}

// collectBuildOrderStates reduces every module's artifact, as read by the
// caller's `load` function, to a buildOrderState.
//
// Only LOCKED artifacts are audited by the forward rules. An unlocked (merely
// proposed) artifact is a working document that "build-order propose" overwrites
// freely and that nobody has approved, so it has no record to disagree with and
// demanding one would refuse every commit between propose and lock.
//
// An artifact that cannot be read or hashed is marked Unreadable and audited by
// nothing. "Not proposed" is the normal state of most modules, and an unreadable
// artifact is not evidence about the LEDGER — check's own build-order reporting
// is where a corrupt artifact belongs. Crucially it is not evidence of DELETION
// either, so it must not reach the reverse sweep.
func collectBuildOrderStates(cfg *config.Config, load func(module string) (*buildorder.Artifact, error)) []buildOrderState {
	if cfg == nil {
		return nil
	}
	var states []buildOrderState
	for _, module := range cfg.Modules {
		state := buildOrderState{Module: module, Path: buildorder.ArtifactPath(cfg, module)}

		artifact, err := load(module)
		switch {
		case errors.Is(err, buildorder.ErrNotProposed) || (err == nil && artifact == nil):
			// No artifact: the ordinary state of most modules, and the state the
			// reverse sweep is looking for when a record still stands.
		case err != nil:
			state.Unreadable = true
			state.Err = err
		default:
			hash, hashErr := buildOrderSignature(artifact)
			if hashErr != nil {
				state.Unreadable = true
				state.Err = hashErr
				break
			}
			// The as-if-locked signature is computed from a COPY: the gate is
			// read-only, and mutating the decoded artifact would leak into
			// whatever else the caller does with it.
			relocked := *artifact
			relocked.Locked = true
			lockedHash, lockedErr := buildOrderSignature(&relocked)
			if lockedErr != nil {
				state.Unreadable = true
				state.Err = lockedErr
				break
			}
			state.Present = true
			state.Locked = artifact.Locked
			state.Hash = hash
			state.LockedHash = lockedHash
		}
		states = append(states, state)
	}
	return states
}

// buildOrderGate evaluates the build-order ledger rules over the artifacts the
// caller collected.
//
// A nil store means the ledger could not be read at all, which
// RuleLedgerUnreadable has already reported; adding "and every build order is
// unapproved" on top would be noise attributing one cause to many symptoms.
//
// It takes the whole ledgerInputs rather than the store alone because the
// pre-ledger exemption needs the SAME evidence lock.Audit needs: whether this
// project has ever been through a ledger-aware build (see
// lock.Store.PreLedgerExempt). A build order locked by a v0.2.x build has no
// record either, and reporting it as build-order-ledger-missing — telling the
// human to re-propose and re-lock an order they never touched — was the same
// false accusation the claim half used to make.
func buildOrderGate(in ledgerInputs) []lock.Finding {
	store := in.store
	if store == nil {
		return nil
	}
	preLedgerExempt := store.PreLedgerExempt(in.digests != nil && in.digests.FileExists())

	var findings []lock.Finding
	for _, o := range in.buildOrders {
		// The corrupt artifact, reported before anything else about this module:
		// like the unreadable lock store, it is a statement about the gate's own
		// EVIDENCE, and it is the cause of every rule below saying nothing about
		// this module. See RuleBuildOrderUnreadable.
		if o.Unreadable {
			findings = append(findings, lock.Finding{
				Rule: RuleBuildOrderUnreadable,
				Message: fmt.Sprintf(
					"module %q's build-order artifact (%s) is there but could not be read: %v. No build-order rule can say anything about this module on this run — a corrupt artifact is not evidence that it was deleted, and it cannot be compared to the approval record either, so the approved implementation sequence is unaudited. Restore the file from version control; do NOT re-propose, which would record whatever the claims say now as the approved order.",
					o.Module, o.Path, o.Err),
			})
			continue
		}
		if !o.Present {
			continue
		}
		record, hasRecord := store.Record(lock.BuildOrderLedgerKey(o.Module))
		standing := hasRecord && record.Subject == lock.SubjectBuildOrder && !record.Released()

		// The unlocked artifact: audited by exactly one rule, and only in the
		// one shape that cannot be an honest re-proposal. See
		// RuleBuildOrderLedgerOrphan.
		if !o.Locked {
			if standing {
				// Whether the phases were edited too changes only the wording.
				// Both are the same finding: an artifact saying it needs nobody's
				// approval, under a record saying somebody gave it.
				what := "with its own \"locked\" flag set to false and nothing else changed"
				if o.LockedHash != record.Hash {
					what = "with its own \"locked\" flag set to false AND its content changed"
				}
				findings = append(findings, lock.Finding{
					Rule: RuleBuildOrderLedgerOrphan,
					Message: fmt.Sprintf(
						"module %q's build order (%s) is the artifact approved on %s (%q) %s — its lock-ledger record still stands, unreleased. An unlocked artifact is audited by nothing (it is meant to be a fresh proposal nobody has approved yet), so clearing that one boolean takes an approved implementation sequence out of the gate while leaving it in place for an agent to follow. An honest re-proposal releases the record as it overwrites the artifact, so a standing record here means nothing released it. Restore the artifact from version control, or discard the approved order deliberately by re-proposing it (dossierx build-order propose --module %s) and locking the result.",
						o.Module, o.Path, record.At, record.Reason, what, o.Module),
				})
			}
			continue
		}

		if !hasRecord || record.Subject != lock.SubjectBuildOrder {
			if preLedgerExempt {
				// Locked before this project had a ledger to record it in.
				// Grandfathered in memory, exactly as a locked claim is.
				continue
			}
			findings = append(findings, lock.Finding{
				Rule: RuleBuildOrderLedgerMissing,
				Message: fmt.Sprintf(
					"module %q's build order (%s) is locked but has no lock-ledger record: its locked flag was set outside the approval path, or the record was removed. A locked build order is the implementation sequence an agent follows, so it sits in the same gate as a locked claim. Re-propose and lock it properly (dossierx build-order propose --module %s, then dossierx build-order lock --module %s --reason \"...\").",
					o.Module, o.Path, o.Module, o.Module),
			})
			continue
		}
		if o.Hash != record.Hash {
			findings = append(findings, lock.Finding{
				Rule: RuleBuildOrderContentDrift,
				Message: fmt.Sprintf(
					"module %q's locked build order (%s) no longer matches what was approved on %s (%q). The artifact is generated, never hand-edited: revert the edit, or re-run dossierx build-order propose --module %s and lock the fresh order with the human's approval.",
					o.Module, o.Path, record.At, record.Reason, o.Module),
			})
		}
	}
	return append(findings, abandonedBuildOrders(in.buildOrders, store)...)
}

// abandonedBuildOrders is the reverse sweep: the LEDGER's own build-order
// records, checked against the artifacts that are actually there.
//
// Every rule in buildOrderGate's forward loop starts from a file on disk, so the
// gate's entire evidence set was chosen by two properties of the audited file
// itself — that it exists, and that its own `locked` flag says true. Deleting
// the artifact removed it from the evidence set altogether and the standing
// record was never consulted again: `rm .build-order.widget.json` produced a
// completely silent gate over an approved implementation sequence. This walks
// the other way, exactly as lock.Audit's lock-ledger-abandoned sweep does for
// claims.
//
// It is skipped when the store file is absent, on the same terms lock.Audit
// skips its sweep: an absent ledger is already reported once as
// lock-ledger-absent, and an in-memory store assembled by a caller must not have
// every module it did not know about reported as deleted.
//
// It deliberately does NOT fire on an artifact that is PRESENT but unlocked —
// that is the forward loop's business, not a deletion. The forward loop reports
// it as RuleBuildOrderLedgerOrphan whenever the record still STANDS, which it no
// longer does after an honest re-propose: "build-order propose" releases the
// module's record as it overwrites the artifact. So the window between a
// re-propose and the lock that follows it stays silent here and there, without
// either rule having to guess which unlocked artifact is honest.
func abandonedBuildOrders(orders []buildOrderState, store *lock.Store) []lock.Finding {
	if store == nil || !store.FileExists() {
		return nil
	}

	byModule := make(map[string]buildOrderState, len(orders))
	for _, o := range orders {
		byModule[o.Module] = o
	}

	// The key prefix is taken from the key builder itself rather than spelled
	// out again, so the two cannot drift apart.
	prefix := lock.BuildOrderLedgerKey("")

	var findings []lock.Finding
	for key, record := range store.Ledger {
		if record.Subject != lock.SubjectBuildOrder || record.Released() {
			continue
		}
		module := strings.TrimPrefix(key, prefix)
		// A present artifact — locked or not — is the forward loop's business.
		// An unreadable one is not evidence of deletion (see buildOrderState).
		if state, known := byModule[module]; known && (state.Present || state.Unreadable) {
			continue
		}
		findings = append(findings, lock.Finding{
			Rule: RuleBuildOrderLedgerAbandoned,
			Message: fmt.Sprintf(
				"module %q has a standing build-order lock-ledger record from %s (%q), but no build-order artifact can be found for it: the file was deleted, or the module was removed from project.config.yaml's modules list so nothing audits it any more. A locked build order is the implementation sequence an agent follows — restore .build-order.%s.json from version control, or re-propose and lock it with the human's approval.",
				module, record.At, record.Reason, module),
		})
	}
	// Map iteration is random; the gate's output is diffed between a hook run
	// and a CI log, so it has to be deterministic.
	sort.SliceStable(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings
}

// buildOrderSignature hashes a build-order artifact exactly as
// cmd/dossierx.recordBuildOrderApproval does when it writes the record: sha256
// over encoding/json's canonical marshalling of the whole artifact.
//
// The two must agree byte-for-byte or the gate reports drift on artifacts nobody
// touched, so TestBuildOrderSignatureMatchesTheWriter pins them against each
// other. json.Marshal is deterministic for this type — struct fields in
// declaration order, and the one map (Hashes) is emitted with sorted keys by
// encoding/json's own contract — which is what makes hashing the marshalled form
// safe rather than merely convenient.
func buildOrderSignature(a *buildorder.Artifact) (string, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

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

	// buildOrders is every module's build-order artifact, reduced to the state
	// the gate needs — including the modules whose artifact is absent, which is
	// what the reverse sweep reads. It comes from the same
	// evidence source as the two stores above — the working tree on the plain
	// path, the git index under --staged — so a build order can no more be
	// audited against the wrong copy than a claim can.
	buildOrders []buildOrderState

	// storeErr / digestErr are the load failures behind a nil above, kept so
	// the finding can name the actual decode error rather than "something went
	// wrong".
	storeErr  error
	digestErr error

	// scopeFindings are the GIT-HISTORY refusals: an integrity store that the
	// parent commit carried and this one does not, or a claims_dir move that
	// stranded tracked claims outside the audited scope. They are populated only
	// under --staged, because they are the one thing in this gate that cannot be
	// answered from a single tree — see history.go.
	//
	// They live here rather than being returned separately so that every caller
	// of ledgerGate gets them without a second code path: the pre-commit hook,
	// CI and serve's status strip all read Result.LedgerFindings, and a refusal
	// that only one of them could see would be a refusal an edit travels around.
	scopeFindings []lock.Finding

	// parentFindings are the PER-CLAIM history refusals: an approval the parent
	// commit recorded that this one has replaced with a self-issued one, or a
	// review the parent recorded that this one erased together with the digest
	// entry proving it happened. Like scopeFindings they are populated only
	// under --staged and for the same reason — no single tree contains the
	// evidence — but they are reported with lock.Audit's per-claim output rather
	// than ahead of it, because they are not statements about what the gate
	// could see. They are the same three rules lock.Audit already owns, decided
	// from the one place the evidence still exists. See lock.AuditAgainstParent.
	parentFindings []lock.Finding

	// scopeNote is the one scope answer that is NOT a refusal: a shallow
	// checkout whose parent commit was never fetched, where the comparison could
	// not be made at all. It rides in Result.NextSteps because "could not look"
	// is advice about the run, not evidence about the project — see
	// scopeReport.Note.
	scopeNote string
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

	in.buildOrders = collectBuildOrderStates(cfg, func(module string) (*buildorder.Artifact, error) {
		return buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
	})

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

	// The SCOPE findings come first, ahead of even the unreadable-store causes,
	// because they are the cause OF those causes: a commit that deleted the
	// ledger and repointed claims_dir leaves every rule below with nothing to
	// read, and a reader who sees an empty finding list without being told the
	// scope collapsed will draw exactly the wrong conclusion — the same
	// conclusion the whole product drew before this existed.
	findings = append(findings, in.scopeFindings...)

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

	// Concatenated, deliberately without de-duplicating: AuditAgainstParent
	// fires only where the evidence lock.Audit reads is ABSENT from this commit,
	// so the two lists cannot name the same claim under the same rule. See
	// lock.AuditAgainstParent's "IT NEVER DOUBLE-REPORTS WITH Audit" paragraph.
	findings = append(findings, in.parentFindings...)
	return append(findings, buildOrderGate(in)...)
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
// project, lock.PrepareStore's adoptCommentDigests for one migrating across), so
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
				c.ID, record.At, record.Reason, digest.StoreFileName, digest.StoreFileName),
		})
	}

	for _, id := range lock.AbandonedCommentDigests(claims, in.store, in.digests) {
		findings = append(findings, lock.Finding{
			Rule:    RuleCommentDigestAbandoned,
			ClaimID: id,
			Message: fmt.Sprintf(
				"%s records comment threads for claim %q, which is no longer in the project: the claim file was deleted, or its id was changed — and changing the id in the same edit that deletes a comments block is how an unresolved review disappears with nothing reported against the claim that replaces it. Restore the claim (under its recorded id) from version control, or — if the removal was intended — restore it, resolve or delete its threads through dossierx so the removal is on the record, and delete it again.",
				digest.StoreFileName, id),
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
			digest.StoreFileName, len(claims)),
	}, true
}
