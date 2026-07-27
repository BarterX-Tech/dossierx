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

// RuleCommentDigestAbsent is project-scoped: claims carry comment threads, this
// project is already covered by the lock ledger, and the COMMENT DIGEST STORE is
// not there.
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
// build" (lock.Store.LedgerCovered), AND at least one claim actually carries
// comments. Both qualifiers exist to keep it off correct state:
//
//   - A project upgrading INTO this feature has no digest store and has done
//     nothing wrong. Its lock store is still at the pre-ledger version, so it is
//     exempt — and lock.PrepareStore CREATES the digest store at the very moment
//     it stamps that version (as does Store.Save at the moment it creates a lock
//     store for a fresh project), so a project crosses both lines together and
//     never sees this finding.
//   - A project with no comment threads anywhere has nothing for the store to
//     record, so its absence says nothing.
//
// WHAT THIS RULE DOES NOT CATCH, stated plainly because the second qualifier is
// what limits it. An attacker who deletes a claim's ONLY thread and the digest
// store in one commit leaves a project with no comments anywhere, so `commented`
// is zero and this rule stays silent. It fires on the PARTIAL launder (threads
// survive somewhere in the project) and on the bare deletion; it does not fire
// on the total one.
//
// That qualifier cannot simply be dropped. check --staged reads both stores out
// of the git INDEX (see stagedLedgerInputs), so "no digest store in the index"
// is also the state of every project that has not committed one yet — including
// a brand-new project that has never been commented on. Firing there would make
// the pre-commit hook and CI refuse every commit until someone git-adds a file
// they have no reason to know about, which is a strictly worse failure than the
// residue above. Closing it properly needs positive evidence that the project
// HAD threads (a per-claim "this claim is digest-uncovered" marker, or a count
// carried in the lock store), not a broader absence test.
//
// Recovery is version control, exactly as for lock-ledger-absent: the file is
// tracked, so restoring it is what brings the evidence back. Re-creating it by
// running a comment op would adopt whatever the claims say NOW, which is what an
// attacker wants — internal/comments refuses to adopt in a covered project for
// that reason.
const RuleCommentDigestAbsent = "comment-digest-absent"

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

	// Present is true when an artifact file was found and decoded.
	Present bool
	// Locked is the artifact's own locked flag (meaningful only when Present).
	Locked bool
	// Unreadable is true when a file IS there but could not be read, decoded or
	// hashed. Distinct from !Present on purpose: absence is evidence about the
	// ledger, a corrupt file is not (see collectBuildOrderStates).
	Unreadable bool
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
		default:
			hash, hashErr := buildOrderSignature(artifact)
			if hashErr != nil {
				state.Unreadable = true
				break
			}
			state.Present = true
			state.Locked = artifact.Locked
			state.Hash = hash
		}
		states = append(states, state)
	}
	return states
}

// buildOrderGate evaluates the two build-order ledger rules over the locked
// artifacts the caller collected.
//
// A nil store means the ledger could not be read at all, which
// RuleLedgerUnreadable has already reported; adding "and every build order is
// unapproved" on top would be noise attributing one cause to many symptoms.
func buildOrderGate(orders []buildOrderState, store *lock.Store) []lock.Finding {
	if store == nil {
		return nil
	}

	var findings []lock.Finding
	for _, o := range orders {
		if !o.Present || !o.Locked {
			continue
		}
		record, ok := store.Record(lock.BuildOrderLedgerKey(o.Module))
		if !ok || record.Subject != lock.SubjectBuildOrder {
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
	return append(findings, abandonedBuildOrders(orders, store)...)
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
// It deliberately does NOT fire on an artifact that is PRESENT but unlocked.
// That state is the honest re-propose window — "build-order propose" overwrites
// a locked artifact with a fresh unlocked one and does not (yet) release the
// record — so reporting it would refuse every commit between a re-propose and
// the lock that follows it. Closing that half needs the release wired into
// propose (see lock.ReleaseBuildOrderApproval); until then, a hand-flipped
// `"locked": false` stays invisible and is a known gap rather than a rule that
// fires on correct state, which is the one thing that makes a gate get turned
// off.
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
	}

	findings = append(findings, lock.Audit(claims, in.store, in.digests)...)
	return append(findings, buildOrderGate(in.buildOrders, in.store)...)
}

// commentDigestAbsent evaluates RuleCommentDigestAbsent (see it for the whole
// argument). It is reported with the other cause-level findings, before
// lock.Audit's per-claim output, because when it fires it explains why the
// comment rules below said nothing at all.
func commentDigestAbsent(claims []model.Claim, in ledgerInputs) (lock.Finding, bool) {
	// A store that failed to DECODE is a different condition, already reported
	// as lock-ledger-unreadable above; nil here means exactly that.
	if in.digests == nil || in.digests.FileExists() {
		return lock.Finding{}, false
	}
	if !in.store.LedgerCovered() {
		return lock.Finding{}, false
	}
	commented := 0
	for _, c := range claims {
		if len(c.Comments) > 0 {
			commented++
		}
	}
	if commented == 0 {
		return lock.Finding{}, false
	}
	return lock.Finding{
		Rule: RuleCommentDigestAbsent,
		Message: fmt.Sprintf(
			"%d claim(s) carry comment threads but the comment digest store (%s) is missing, so comment-thread drift is not being checked AT ALL on this run. Either that file was deleted — which is how an edited-away review thread stops being reported — or the threads were written into the YAML by hand, outside the engine that records them. Restore the file from version control rather than re-creating it: a re-created store records whatever the claims say NOW as the truth, which is exactly what a deletion was for.",
			commented, digest.StoreFileName),
	}, true
}
