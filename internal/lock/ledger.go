// ledger.go implements the LOCK LEDGER: a per-locked-artifact record of the
// content that was approved, when, by whom, and on whose words.
//
// The problem it solves is the one thing this engine could not previously see.
// Claims are YAML files in git, so nothing can PREVENT a hand edit — and the
// goal was never prevention, it is that no out-of-band edit is SILENT. Before
// the ledger, all of these were invisible:
//
//	flip status: draft -> locked by hand   walks past the lint gate, hub
//	                                       gating, and the unresolved-comment
//	                                       gate as though they had all passed
//	edit a locked body with no dependents  no dependent means no baseline
//	                                       compares it, so nothing looks
//	swap raw_html on a locked mockup       rendered UNESCAPED, with no baseline
//	                                       comparing it either — a mockup has no
//	                                       dependents. (ContentHash has covered
//	                                       raw_html since v0.4.1, but that only
//	                                       marks a DEPENDENT stale; being stale
//	                                       is not being tampered with, and this
//	                                       row still holds.)
//	flip build_role/section/order/emphasis ContentHash covers none of them
//	flip locked -> draft to dodge review   the claim simply looks like a draft
//
// The ledger closes all five by recording, at every legitimate approval, the
// LockedClaimHash of what was approved. The gate in audit.go then compares the
// world against the ledger; a mismatch is a refusal with a name, not a guess.
//
// Two deliberate non-goals. The ledger is NOT authentication — Actor is
// provenance ("which account ran the command"), not identity, and anyone who
// can edit a claim can edit the ledger. What it buys is that tampering now
// requires editing TWO tracked files consistently instead of one, and the
// second is a file whose whole purpose is to be reviewed in the diff. And the
// ledger is NOT a lint: registering these rules in the lint registry would make
// every one of them an error-severity finding project-wide, which would freeze
// all locking AND stop "dossierx check" from regenerating the viewer — turning
// a single tampered claim into a total outage. It is its own gate, evaluated by
// the pre-commit hook and by CI, which is the authority.
package lock

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// LedgerSubject is what kind of artifact a ledger record covers. The ledger is
// keyed by a single string map (so it round-trips as plain JSON), and a claim id
// can never collide with a build-order key by construction — but the audit gate
// filters on THIS field rather than by parsing keys, so a future subject kind
// cannot quietly start being audited as a claim.
type LedgerSubject string

const (
	// SubjectClaim: the record covers one locked model.Claim, keyed by its id.
	SubjectClaim LedgerSubject = "claim"

	// SubjectBuildOrder: the record covers one module's locked build-order
	// artifact, keyed by BuildOrderLedgerKey(module). A locked build order is a
	// SECOND class of locked artifact; leaving it outside the approval path
	// would make this release's headline invariant — "nothing already locked
	// changes without your approval on the record" — an overclaim.
	SubjectBuildOrder LedgerSubject = "build-order"
)

// LedgerRecord is one approval on the record.
//
// Released* is what makes "keep the record alive across Unlock" work. Unlock
// does NOT delete a claim's record; it marks it released. Deleting it would
// destroy the only evidence that the claim was ever locked, and two things
// depend on that evidence surviving:
//
//   - lock-ledger-orphan needs a precise predicate for "someone flipped locked
//     -> draft by hand to dodge review". With releases recorded, the predicate
//     is exact: a draft claim holding an UNRELEASED record. Without them, the
//     only available predicate would be a guess about content, which would fire
//     on every honest unlock.
//
//   - comment-drift detection must survive the unlock window. An unlocked claim
//     is still a claim whose comment history a human is relying on; dropping
//     its record at unlock would open a laundering path (unlock, edit comments,
//     relock) precisely at the moment supervision is weakest.
type LedgerRecord struct {
	// Subject is what this record covers (see LedgerSubject).
	Subject LedgerSubject `json:"subject"`

	// Hash is LockedClaimHash of the claim as approved (or, for a build-order
	// record, the caller-supplied signature of the frozen artifact).
	Hash string `json:"hash"`

	// At is the RFC3339Nano UTC time the approval was recorded.
	At string `json:"at"`

	// Actor is who the machine says ran the command (see DefaultActor).
	// Provenance, not authentication.
	Actor string `json:"actor"`

	// Reason is the human's own approving words, carried in from --reason.
	// This is the field a reviewer actually reads in a diff: it is the only
	// part of the record that a machine cannot generate for itself.
	Reason string `json:"reason"`

	// Grandfathered is a HISTORICAL marker, read-only in this build. Nothing
	// writes it any more: v0.4.0 removed the adoption path that minted records
	// from content nobody approved, so every record this build creates is an
	// approval and carries false. The field survives because it is persisted
	// state — Store.Save marshals this struct wholesale — and dropping it would
	// silently rewrite every surviving legacy record into one indistinguishable
	// from a human approval on the very next ordinary write. Readers still read
	// it, and a project can still find and re-lock the records it marks.
	Grandfathered bool `json:"grandfathered,omitempty"`

	// ReleasedAt/By/Reason record a legitimate unlock. A record with
	// ReleasedAt set describes a claim that IS allowed to be draft.
	ReleasedAt     string `json:"released_at,omitempty"`
	ReleasedBy     string `json:"released_by,omitempty"`
	ReleasedReason string `json:"released_reason,omitempty"`
}

// Released reports whether this record has been legitimately released by an
// unlock (as opposed to still standing as an active approval).
func (r LedgerRecord) Released() bool { return r.ReleasedAt != "" }

// Approval is the human authority a ledger write attests to: whose words
// approved it, and which account executed it. Every write hook takes one, by
// value, so no caller can record an approval without having something to put
// in it — the CLI's --reason gate is what fills Reason, and requireReason
// refuses an empty one before any of this is reached.
type Approval struct {
	// Actor: typically DefaultActor(). Provenance, not identity.
	Actor string
	// Reason: the human's own approving words (--reason).
	Reason string
}

// BuildOrderLedgerKey is the ledger key for module's build-order artifact. The
// "build-order:" prefix cannot collide with a claim id (claim ids are
// dot-separated kebab-case segments; the id-shape lint refuses a colon), and
// the record's Subject field is what the audit filters on regardless.
func BuildOrderLedgerKey(module string) string { return "build-order:" + module }

// DefaultActor resolves the actor string for a ledger write from the
// environment, in priority order: DOSSIERX_ACTOR (an explicit override, which
// is what CI and any wrapper should set), then USER (POSIX), then USERNAME
// (Windows), then the literal "unknown".
//
// It deliberately does NOT use os/user: that package needs cgo for a complete
// answer on some platforms, and this engine's portability bar (a pure-Go build
// that runs identically on the Windows leg of the CI matrix) is worth more here
// than a marginally better guess at a name. "unknown" is an honest answer — the
// ledger's integrity rests on Hash and Reason, not on Actor.
func DefaultActor() string {
	for _, key := range []string{"DOSSIERX_ACTOR", "USER", "USERNAME"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "unknown"
}

// ledgerAnnounceWriter is where a one-time store migration announces itself. It
// is a package-level var rather than a writer threaded through every caller
// precisely BECAUSE such a migration must never be silent: it happens deep
// inside a store-load path reached from five different commands, and an
// announcement that each caller has to remember to print is an announcement
// that one of them will eventually forget. Tests redirect it; nothing else
// should.
var ledgerAnnounceWriter io.Writer = os.Stderr

// Record returns the ledger record for key (a claim id, or
// BuildOrderLedgerKey(module)) and whether one exists.
func (s *Store) Record(key string) (LedgerRecord, bool) {
	if s.Ledger == nil {
		return LedgerRecord{}, false
	}
	r, ok := s.Ledger[key]
	return r, ok
}

// putRecord stores r under key, allocating the ledger map on first use.
func (s *Store) putRecord(key string, r LedgerRecord) {
	if s.Ledger == nil {
		s.Ledger = map[string]LedgerRecord{}
	}
	s.Ledger[key] = r
}

// RecordApproval writes (or overwrites) claim's ledger record with the hash of
// its currently-approved content. Lock calls it for every successful lock; the
// reaudit apply path calls it after writing a confirmed proposal.
//
// ANY path that legitimately rewrites a LOCKED claim's persisted content must
// call this, or the gate will correctly report the honest write as tampering.
// Today that is exactly two paths: Lock and reaudit --confirm. Everything else
// that touches a locked claim on disk writes only status, review_pending, or
// comments — the three fields LockedClaimHash deliberately does not sign — which
// is why the deny-list has exactly those three entries and not one more.
//
// It clears any prior release fields: re-locking a previously-released claim is
// a NEW approval, and leaving the old release stamped on it would let the
// record read as "this claim is allowed to be draft" while it is locked.
func RecordApproval(store *Store, claim model.Claim, ap Approval) {
	if store == nil {
		return
	}
	store.putRecord(claim.ID, LedgerRecord{
		Subject: SubjectClaim,
		Hash:    LockedClaimHash(claim),
		At:      nowFunc().UTC().Format(time.RFC3339Nano),
		Actor:   ap.Actor,
		Reason:  ap.Reason,
	})

	// The claim's COMMENT DIGEST is recorded at the same instant, and that is
	// what makes internal/check's comment-digest-missing rule a rule rather than
	// a wish: a claim holding a standing approval must, from this moment, have a
	// digest entry, so an emptied `"digests": {}` is a finding instead of
	// silence.
	//
	// Before this, the digest store was protected against DELETION (the
	// project-scoped comment-digest-absent rule) and not against being emptied,
	// and emptying it is strictly cheaper to hide in a review diff than the `rm`
	// the rule did catch. The measured asymmetry on one tampered claim: delete
	// the file -> ok:false ['comment-digest-absent']; leave the file and empty
	// the map -> ok:true, []. An approval that records the lock and not the
	// review it was granted under leaves the coverage question answerable only
	// from evidence the tamper controls.
	//
	// Best-effort, like every other digest write on this path (see
	// ensureCommentDigestStore): a project that cannot write the file is not one
	// whose lock should fail. If it does not land, the claim reads as uncovered
	// and the gate says so, which is the loud direction.
	recordCommentDigestBeside(store, claim)
}

// LedgerRecordDeleted reports whether claim is in the state Lock refuses with
// ErrLedgerRecordDeleted: no ledger record, on a claim this engine demonstrably
// locked (see engineLocked).
//
// It is exported so the CLI's `claim lock --dry-run` can ask the question the
// real run asks, with the same implementation rather than a restatement of it.
// A preview that disagrees with its write path is the defect this codebase
// treats most seriously, because the agent takes the preview to its human, gets
// a yes, and then cannot deliver it.
//
// It deliberately does NOT fold in the pre-ledger check: a project whose store
// predates the ledger has locked_at on every locked claim and no records at
// all, and Lock refuses that case FIRST with the crossing instructions, which
// is the more actionable answer. The preview asks that question separately
// (preLedgerPrecondition).
func (s *Store) LedgerRecordDeleted(claim model.Claim) bool {
	if s == nil || s.preLedgerUnadoptedOnDisk() {
		return false
	}
	_, ok := ledgerRecordFor(s, claim.ID)
	return !ok && engineLocked(s, claim.ID)
}

// CommentDigestUnrecorded reports whether claim is in the state Lock refuses
// with ErrCommentDigestUnrecorded — the exported form of commentDigestUnrecorded,
// for the same preview/run-agreement reason as LedgerRecordDeleted above.
func (s *Store) CommentDigestUnrecorded(claim model.Claim) bool {
	return commentDigestUnrecorded(s, claim)
}

// commentDigestUnrecorded reports whether claim is in the state
// RuleCommentDigestUnrecorded names: it carries comment threads, this project is
// covered by the lock ledger, the digest store is present — and there is no
// entry for the claim in it.
//
// It reads the digest store beside the lock store rather than taking one as a
// parameter, because its caller is Lock, whose signature is fixed by four call
// sites and which already reaches that file the same way one function down (see
// recordCommentDigestBeside). It takes no sentinel: this is a READ, and the
// caller is inside the claims sentinel, under which every writer of this file
// already runs.
//
// A store that cannot be read yields false — the claim is not accused on the
// strength of evidence that could not be loaded, and the unreadable store is its
// own finding with its own recovery.
func commentDigestUnrecorded(s *Store, claim model.Claim) bool {
	if s == nil || s.path == "" || len(claim.Comments) == 0 {
		return false
	}
	digests, err := digest.LoadStore(digest.StorePathBeside(s.path))
	if err != nil || digests == nil || !digests.FileExists() {
		return false
	}
	// LedgerEstablished, not LedgerCovered: this is a REFUSAL, and a refusal an
	// attacker can disarm by editing the audited file's own version field is not
	// one. See LedgerEstablished. The digest store's presence is read from the
	// store just loaded, which is the same evidence LedgerDowngraded wants and
	// which this function has in hand anyway.
	if !s.LedgerEstablished(digests.FileExists()) {
		return false
	}
	_, known := digests.Digest(claim.ID)
	return !known
}

// recordCommentDigestBeside records claims' current comment blocks into the
// comment digest store that sits beside this lock store.
//
// It takes the digest store's own sentinel underneath whatever the caller
// already holds — which is the claims sentinel, for every command that reaches
// RecordApproval — so the claims -> digest ordering internal/digest's package
// comment fixes is unchanged, and the digest sentinel is still never taken
// alone.
func recordCommentDigestBeside(s *Store, claims ...model.Claim) {
	if s == nil || s.path == "" || len(claims) == 0 {
		return
	}
	path := digest.StorePathBeside(s.path)

	release, err := AcquireFileLock(path)
	if err != nil {
		return
	}
	defer release()

	store, err := digest.LoadStore(path)
	if err != nil {
		return
	}
	for _, c := range claims {
		store.Record(c)
	}
	store.Save() //nolint:errcheck // best-effort: see RecordApproval
}

// RecordBuildOrderApproval writes a module's build-order artifact into the
// ledger. The hash is supplied by the caller rather than computed here because
// internal/buildorder imports this package (for ContentHash), so computing it
// here would invert that dependency into a cycle. The caller passes a signature
// of the frozen artifact.
func RecordBuildOrderApproval(store *Store, module, hash string, ap Approval) {
	if store == nil {
		return
	}
	store.putRecord(BuildOrderLedgerKey(module), LedgerRecord{
		Subject: SubjectBuildOrder,
		Hash:    hash,
		At:      nowFunc().UTC().Format(time.RFC3339Nano),
		Actor:   ap.Actor,
		Reason:  ap.Reason,
	})
}

// PreLedger reports whether this store was loaded from a file written by a
// build that PREDATES the lock ledger — the one condition under which crossing
// onto the ledger is honest rather than a bypass.
//
// It is the same predicate CrossPreLedger keys on, exported because build-order
// artifacts sit under exactly the same exemption and cannot be reached from this
// package (internal/buildorder imports internal/lock, so the edge cannot run
// the other way).
//
// Both halves matter. A store at the current version never crosses again: after
// the crossing, a locked artifact without a record is a finding, not an
// invitation. And an ABSENT store never crosses at all, because absence is
// indistinguishable from someone deleting the ledger to re-bless a tampered
// project — which would make `rm .dossierx-lock-store.json` the universal
// bypass.
func (s *Store) PreLedger() bool {
	return s != nil && s.fileExists && s.diskVersion < ledgerSchemaVersion
}

// LedgerDowngraded reports whether this store CLAIMS to predate the lock ledger
// while the project around it proves that it does not.
//
// It exists because PreLedger, on its own, let the audited file disarm the gate
// with a text editor. Grandfathering keys on the store's own "version" field, so
// the whole approval path could be re-armed from inside the one file the gate is
// checking:
//
//	lock a claim (store is stamped version 2, record written)
//	edit the claim's body by hand          -> lock-content-drift, correctly
//	set "version": 2 -> 1, delete "ledger" -> adoption fires again and records
//	                                          the EDITED bytes as approved
//
// One hand edit to the ledger file and `check` said ok. That is worse than
// having no ledger, because it reads as an audit that passed.
//
// The fix is that a downgrade has to survive contact with evidence the audited
// file does not own. Two pieces exist, and either one is conclusive:
//
//   - digestStorePresent: .dossierx-comment-digest.json is a SIBLING file that
//     this build creates at the exact moment a project becomes ledger-covered —
//     CrossPreLedger for a pre-ledger project that crosses, and Store.Save's
//     ensureCommentDigestStore for a fresh project that crosses on
//     its first lock. A genuine v0.2.x project has never had one (the file did
//     not exist before v0.3.0), so its presence beside a store that says
//     "version 1" is a contradiction: this project HAS been through a
//     ledger-aware build.
//
//   - the LEDGER KEY, present at all: it did not exist before schema 2, so a
//     store that predates the ledger cannot carry it — with or without records
//     inside. This half costs nothing and catches the lazier edit (drop the
//     version, keep the records) AND the one that used to slip past it: this
//     evidence used to read the map's SIZE, so emptying the map in the same edit
//     that lowered the version ("ledger": { … } -> {}) satisfied it, and an
//     emptied map is a smaller diff than a deleted key. The in-memory half
//     (len(s.Ledger) > 0) is kept beside it for callers that build a store
//     without a file.
//
// WHAT IT DOES NOT CLOSE, stated plainly. An attacker who deletes the digest
// store as well as the ledger, in the same commit, produces a project that is
// byte-for-byte the shape of a legitimate pre-ledger one, and nothing in these
// three files can tell them apart — the evidence is gone. What that costs the
// attacker is two tracked files deleted in a reviewable diff instead of one
// number changed inside a file nobody re-reads, and the digest store's own
// absence rule (internal/check's comment-digest-absent) is what is left to say
// so. Closing it completely needs evidence that lives outside the project
// directory entirely (a signature, or the commit history), which is a different
// release.
func (s *Store) LedgerDowngraded(digestStorePresent bool) bool {
	if !s.PreLedger() {
		return false
	}
	return digestStorePresent || s.ledgerKeyOnDisk || len(s.Ledger) > 0
}

// PreLedgerUnadopted reports whether this project's store predates the lock
// ledger and has not crossed onto it yet, with the project around it not
// contradicting that claim. Nothing here can carry a lock-ledger approval until
// it crosses (CrossPreLedger).
//
// THE LEDGER FAILS CLOSED, and this predicate is where that decision lives. It
// used to be spelled as an EXEMPTION, and the name was the design: a pre-ledger
// project was exempt — the read-only gate grandfathered it in memory, the write
// path (any `dossierx check`, any claim command) grandfathered it on disk, and
// both happened without anybody asking for it. That was chosen to keep an honest
// v0.2.x project from being accused of tampering on upgrade day, and it worked
// for that. What it also did was make GRANDFATHERING SOMETHING AN ATTACKER COULD
// TRIGGER: the trigger is the store's own version field plus the absence of
// records, so downgrading the version and deleting the ledger key re-armed the
// blessing of whatever the claims said at that moment. LedgerDowngraded closed
// the version-field half by finding evidence the audited file does not own — but
// no evidence in this directory can tell an honest v0.2.x store from a downgraded
// one once BOTH the ledger key and the comment digest store are gone in the same
// commit, because locked_at (the only other pre-ledger artifact) shipped in
// v0.2.0 and looks identical either way.
//
// So the answer is not a cleverer predicate. It is that nothing is EVER blessed
// implicitly: while this project still holds a locked artifact the state below
// is a REFUSAL with a name (RuleLockLedgerPreLedger), and the only way across is
// to empty the project of everything that predates the ledger and re-lock what a
// human still stands behind. A blessing a command performs on its own is a
// blessing an attacker can perform on their own.
//
// It is a conjunction, not a synonym for PreLedger: a store whose pre-ledger
// claim is CONTRADICTED (see LedgerDowngraded) is not offered the crossing at
// all — it is reported as downgraded, and the recovery is version control, never
// a re-lock that would record the tampered content as approved.
func (s *Store) PreLedgerUnadopted(digestStorePresent bool) bool {
	return s.PreLedger() && !s.LedgerDowngraded(digestStorePresent)
}

// preLedgerUnadoptedOnDisk is PreLedgerUnadopted for the WRITE paths, which have
// a real directory to look in and so read the digest store's presence from it —
// the same split LedgerDowngraded/digestStorePresentBeside already makes, and
// for the same reason: the read paths must take that evidence from the store
// they were handed (which is what makes `check --staged` answer for the INDEX
// rather than for the working tree).
func (s *Store) preLedgerUnadoptedOnDisk() bool {
	return s.PreLedgerUnadopted(digestStorePresentBeside(s.path))
}

// digestStorePresentBeside reports whether the comment digest store is on disk
// next to the lock store at lockStorePath — the evidence LedgerDowngraded reads
// on the WRITE path, where there is a real directory to look in. The read path
// takes the same answer from the *digest.Store it has already loaded, which is
// what makes "check --staged" evaluate it against the INDEX's copy rather than
// the working tree's.
func digestStorePresentBeside(lockStorePath string) bool {
	if lockStorePath == "" {
		return false
	}
	_, err := os.Stat(digest.StorePathBeside(lockStorePath))
	return err == nil
}

// LedgerCovered is PreLedger's complement over the same evidence: this project's
// lock store is on disk AND already at the ledger schema, so this build (or one
// like it) has run here and written it.
//
// It exists because "has this project been through a ledger-aware build?" is the
// only migration-safe trigger available to the COMMENT DIGEST rules, which have
// no equivalent of their own. The digest store's absence cannot be keyed on the
// digest store's own history — the file whose absence is the question cannot
// also be the evidence — so it is keyed on this instead: a project still at an
// older lock-store version is mid-upgrade and exempt, a project already stamped
// current is not. See internal/check's comment-digest-absent rule, and
// CrossPreLedger, which creates the digest store at the same moment it stamps
// this version so an upgrading project crosses both lines together.
func (s *Store) LedgerCovered() bool {
	return s != nil && s.fileExists && s.diskVersion >= ledgerSchemaVersion
}

// LedgerEstablished is the question the WRITE PATHS have to ask instead of
// LedgerCovered: has a ledger-aware build ever run in this project, WHATEVER the
// store's version field now says?
//
// LedgerCovered reads that version field, which lives inside the audited file,
// so a store edited back to version 1 answers "no" — and every comment-digest
// guard armed by it silently disarms. That is not hypothetical: the downgrade
// is two hand edits (see LedgerDowngraded), and with the guards down the
// laundering path they exist to close re-opens one file over. On a downgraded
// store, before this predicate existed:
//
//	forge a human's open thread to `status: resolved` in the YAML
//	drop that claim's key from .dossierx-comment-digest.json
//	set the lock store's "version": 2 -> 1   (the ledger key stays; the gate
//	                                          reports lock-ledger-downgraded)
//	dossierx comment reply <id> …            ACCEPTED — checkCommentDigest's
//	                                          threads-without-an-entry arm was
//	                                          armed by LedgerCovered, so it did
//	                                          not run, and the write RECORDED the
//	                                          forged block as the truth
//	dossierx claim lock <id> --reason …      ACCEPTED for the same reason
//
// The project-scoped downgrade finding still fires, so the state is not silent —
// but the evidence it points at is destroyed in the meantime, and the recovery
// it names (restore the lock store) no longer brings the review history back.
// A refusal must not be disarmable by the same edit that the gate is already
// reporting.
//
// PrepareStore's comment-digest sweep has always used this wider question,
// spelled out inline; this is that expression given a name so the sweep, the
// comment write path (internal/comments' mutate) and the lock write path
// (commentDigestUnrecorded) cannot answer it three different ways.
//
// lock.Audit deliberately does NOT use it, and that asymmetry is the design: a
// downgraded store gets ONE project-scoped finding naming the cause, and piling
// per-claim findings on top of it would bury the one sentence a reader needs.
// REPORTING says it once; REFUSING has to hold everywhere.
//
// It stays silent on the honest pre-ledger project — a genuine v0.2.x store is
// pre-ledger and NOT downgraded (no ledger key, no digest store beside it), so
// it answers "no" and every gate below it stays off, which is what keeps the
// crossing reachable.
func (s *Store) LedgerEstablished(digestStorePresent bool) bool {
	return s.LedgerCovered() || s.LedgerDowngraded(digestStorePresent)
}

// ReleaseApproval marks claimID's ledger record released by a legitimate
// unlock, KEEPING the record (see LedgerRecord's doc comment for why deleting
// it would be a laundering path and would leave lock-ledger-orphan without a
// predicate). It reports whether a record was there to release; false means the
// claim had no record at all, which the gate has already reported as
// lock-ledger-missing and which unlock must not treat as an error — unlock is
// the recovery escape hatch and must always work.
func ReleaseApproval(store *Store, claimID string, ap Approval) bool {
	if store == nil {
		return false
	}
	r, ok := store.Record(claimID)
	if !ok {
		return false
	}
	r.ReleasedAt = nowFunc().UTC().Format(time.RFC3339Nano)
	r.ReleasedBy = ap.Actor
	r.ReleasedReason = ap.Reason
	store.putRecord(claimID, r)
	return true
}

// ReleaseBuildOrderApproval marks module's build-order record released, KEEPING
// the record for the same reason ReleaseApproval keeps a claim's: the evidence
// that this module's order was ever approved is what a later sweep needs, and
// deleting it would make removal quieter than editing. It reports whether a
// record was there to release.
//
// It is the build-order twin of ReleaseApproval, and the act that legitimately
// releases a build order is "dossierx build-order propose": propose overwrites a
// locked artifact with a fresh, unlocked one, which is precisely "this approved
// order no longer stands". propose calls this immediately after WriteArtifact,
// under the same lock-store sentinel, and that call is what makes the orphan
// half of the check gate safe to state without exceptions: the honest
// propose-then-lock window is now the only unlocked artifact whose record is
// RELEASED, so internal/check can refuse every unlocked artifact under a
// STANDING record (see check.RuleBuildOrderLedgerOrphan). Before that wiring
// existed the gate had to guess, by re-signing the artifact as if its locked
// flag were still true — which caught a lone flag flip and missed a flip made
// together with a content edit.
func ReleaseBuildOrderApproval(store *Store, module string, ap Approval) bool {
	if store == nil {
		return false
	}
	key := BuildOrderLedgerKey(module)
	r, ok := store.Record(key)
	if !ok || r.Subject != SubjectBuildOrder {
		return false
	}
	r.ReleasedAt = nowFunc().UTC().Format(time.RFC3339Nano)
	r.ReleasedBy = ap.Actor
	r.ReleasedReason = ap.Reason
	store.putRecord(key, r)
	return true
}

// preLedgerCrossingSteps is the recovery every pre-ledger refusal names, and it
// is the same words everywhere so the write path, the audit gate and the CLI
// hint cannot send a reader three different ways.
//
// The ORDER is not cosmetic. "build-order propose" requires the module still
// FULLY LOCKED, so re-proposing has to happen BEFORE any claim is unlocked; the
// other order deadlocks — unlock a claim first and propose then refuses, leaving
// the locked order with no way to be released.
const preLedgerCrossingSteps = `Cross onto the ledger by emptying the project of everything that predates it, in this order:
  1. dossierx build-order propose --module <m>
     for every module whose build order is locked. Do this FIRST: propose requires the module still fully locked, so unlocking a claim first leaves the order stuck.
  2. dossierx claim unlock <id> --reason "..."
     for every locked claim. Unlock is gateless and always has been.
  3. dossierx claim lock <id> --reason "..."
     re-lock only what you still stand behind. The FIRST of these crosses the store onto the ledger and records a real approval — locking is what says a human approved these exact bytes.
  4. dossierx build-order propose --module <m>
     dossierx build-order lock --module <m> --reason "..."`

// preLedgerRefusal composes the refusal ErrPreLedgerUnadopted carries, naming
// how much of the project still predates the ledger so a reader can see which
// half of the count is keeping them out.
func preLedgerRefusal(lockedClaims, lockedBuildOrders int) error {
	return fmt.Errorf("%w: this project's lock store predates the lock ledger, so nothing locked here has an approval record — and nothing can attest to content no ledger ever recorded. There is no automatic adoption and no migration command any more. %d locked claim(s) and %d locked build order(s) still predate it.\n\n%s",
		ErrPreLedgerUnadopted, lockedClaims, lockedBuildOrders, preLedgerCrossingSteps)
}

// CrossPreLedger is the ONE place in this build that raises a store's schema
// version. It crosses a pre-ledger project into the ledger schema at the only
// moment that requires no adoption at all: when the project has NOTHING LEFT
// that predates the ledger.
//
// Nothing is grandfathered, because there is nothing to grandfather. That is the
// whole difference from the removed adoption path (see CHANGELOG v0.4.0), and it
// is why this is safe to do from an ordinary write path: an attacker who empties
// the project of every locked artifact to trigger it has destroyed the approvals
// they were trying to launder.
//
// IT CROSSES BOTH LINES IN ONE ACT: the comment digest store is created FIRST
// and the lock store stamped and saved SECOND, because a store at the ledger
// schema with no digest store beside it is what internal/check reports as
// comment-digest-absent (internal/check/ledger.go's RuleCommentDigestAbsent) — a
// finding whose "restore it from version control" recovery would name a file
// that never existed. Nothing else will create it for this project:
// SweepCommentDigests excludes a pre-ledger store from its crossing on purpose
// (see its `crossing` predicate) and Store.Save's ensureCommentDigestStore is
// gated on !s.fileExists, which is false here.
//
// A DOWNGRADED store is left alone and returns nil, preserving today's exact
// behaviour: the write path never refused a downgraded store either (see
// preLedgerUnadoptedOnDisk's conjunction), because RuleLockLedgerDowngraded owns
// that diagnosis and its recovery is version control.
//
// LOCKING — READ THIS BEFORE ADDING ANY ACQUISITION. The caller must hold the
// lock store's file lock (AcquireFileLock on s.path). NOTHING ELSE is required,
// and nothing else may be taken:
//
//   - The project's claims sentinel is NOT a precondition. The removed adoption
//     path required it because it wrote CLAIM-DERIVED content into the digest
//     store. This does not: the digest store it creates is EMPTY, so claims are
//     read here for exactly one thing — counting locked artifacts — and nothing
//     claim-derived is written. The count is stable for the duration anyway,
//     because every command that changes a claim's LOCK STATUS takes the
//     lock-store sentinel this caller is already holding (claim lock, claim
//     unlock, claim reaudit --confirm, build-order lock).
//   - The digest store's own sentinel is taken and released INSIDE this call, as
//     a leaf, holding nothing else while acquiring it. That is not a new
//     pattern: Store.Save already does exactly this through
//     ensureCommentDigestStore, and that path is already reached from
//     `build-order lock`, which holds the lock-store sentinel and never the
//     claims sentinel.
//
// Requiring the claims sentinel here would be a DEADLOCK, not a nicety. The
// project-wide order is claims -> lock-store -> flag-store. `build-order lock`
// takes only the lock-store sentinel and says so in its own comment; a claims
// acquisition inside that held lock inverts the order against `claim lock` and
// `claim reaudit --confirm`, both of which take claims FIRST.
func CrossPreLedger(s *Store, claims []model.Claim, lockedBuildOrders int) error {
	if s == nil || !s.PreLedger() {
		return nil
	}
	// The downgraded store: its pre-ledger claim is contradicted by the project
	// around it, so it is not offered the crossing at all. See
	// RuleLockLedgerDowngraded, which owns the diagnosis and whose recovery is
	// version control rather than a re-lock.
	if !s.preLedgerUnadoptedOnDisk() {
		return nil
	}
	lockedClaims := countLocked(claims)
	if lockedClaims+lockedBuildOrders > 0 {
		return preLedgerRefusal(lockedClaims, lockedBuildOrders)
	}

	digestStoreExisted := digestStorePresentBeside(s.path)
	if err := createCommentDigestStore(s.path); err != nil {
		return err // nothing stamped, nothing else written
	}
	prevDisk, prevVersion := s.diskVersion, s.Version
	prevLockedAt, prevHashes := s.LockedAt, s.Hashes
	s.diskVersion = ledgerSchemaVersion
	s.Version = storeSchemaVersion
	// THE PRE-LEDGER BOOKKEEPING GOES WITH THE STAMP, and this is not tidying.
	//
	// locked_at and the per-dependent baselines are what engineLocked reads as
	// "this engine locked that claim" (audit.go). In a pre-ledger project they
	// were written by a build that had no ledger, and the pre-ledger predicate is
	// what stops RuleLockLedgerDeleted and Lock's ErrLedgerRecordDeleted from
	// reading them as a record somebody DELETED. The stamp removes that
	// suppression while leaving the evidence behind — so without this, the very
	// first re-lock of step 3 is refused as a deleted record, and `check` accuses
	// every previously-locked claim of tampering. The recovery the refusal names
	// would be unfollowable, which is the exact defect this release exists to fix.
	//
	// It costs nothing, and row 4's precondition is why: the project holds ZERO
	// locked artifacts here, so no drift baseline belongs to anything (DetectStale
	// reads baselines for LOCKED claims only) and no locked_at describes a
	// standing approval. Every re-lock in step 3 writes both again, from content a
	// human has just approved.
	s.LockedAt = map[string]string{}
	s.Hashes = map[string]map[string]string{}
	if err := s.Save(); err != nil {
		s.diskVersion, s.Version = prevDisk, prevVersion // un-stamp in memory
		s.LockedAt, s.Hashes = prevLockedAt, prevHashes
		if !digestStoreExisted {
			// undo: leave the project exactly as found. A digest store left
			// beside an un-stamped lock store is read by LedgerDowngraded —
			// correctly, by its own rules — as a downgrade, which would turn a
			// failed disk write into an accusation of tampering.
			os.Remove(digest.StorePathBeside(s.path)) //nolint:errcheck // best-effort undo of a write that just failed
		}
		return err
	}
	return nil
}

// PrepareStore runs the on-load store migrations a command that opens the store
// for writing is allowed to run BY ITSELF, and is what every such command should
// call. Today that is exactly one:
//
//  1. MigrateLegacyStore re-arms per-dependent DEPENDENCY baselines for a store
//     that predates them (schema 0).
//
// CROSSING ONTO THE LEDGER IS NOT ONE OF THEM. Grandfathering used to be step 2
// here, which meant every ordinary command blessed any project presenting the
// pre-ledger shape — and presenting that shape is two hand edits. There is no
// grandfathering left at all: a pre-ledger project crosses only through
// CrossPreLedger, which refuses while anything locked still predates the ledger,
// and audit.go's RuleLockLedgerPreLedger is what a pre-ledger project sees
// instead of silence.
//
// It reports changed=true if the migration modified the store, so the caller
// knows to Save. It returns NOTHING about grandfathering, and that is the point:
// it briefly kept an `adopted []string` return that was always nil, to spare the
// call sites a change, and a permanently-nil return on a function whose old job
// was grandfathering is a trap — it reads as "nothing was blessed this run" when
// what it means is "this function does not bless anything". Nothing in this
// build produces any.
//
// It also runs the COMMENT DIGEST COVERAGE SWEEP, on every call rather than only
// on a migration — see SweepCommentDigests for why coverage that only ever
// extended at a project's first lock was a hole rather than a conservative
// default.
func PrepareStore(s *Store, claims []model.Claim) (changed bool) {
	changed = MigrateLegacyStore(s, claims)

	// Whether this project is ALREADY ledger-covered is the sweep's absence rule
	// (see SweepCommentDigests): in a covered project a missing digest store is a
	// DELETED one, and adopting there re-derives coverage from the files the
	// deletion was meant to bless. A DOWNGRADED store counts as covered: it is a
	// project that has been through a ledger-aware build whatever its version
	// field now says, and letting the downgrade re-open comment adoption would
	// hand the same edit a second laundering path.
	//
	// It no longer has to be read BEFORE anything, because nothing in this
	// function stamps the schema version any more. That is the same reason the
	// `stale` flag this used to compute is gone: a pre-ledger store is left
	// exactly as found — un-stamped, uncrossed, and reported by the gate — until
	// CrossPreLedger takes it over.
	covered := s.LedgerEstablished(digestStorePresentBeside(s.path))

	s.commentDigestsAdopted, _ = SweepCommentDigests(s, claims, covered)

	// KNOWN GAP, recorded here because it is invisible from the rules
	// themselves: every comment-digest guard above is armed by `covered`,
	// and coverage is LedgerCovered — the LOCK store existing at this schema
	// version. A project that has never locked a claim has no lock store at all
	// (this function deliberately does not create one; see
	// TestPrepareStoreDoesNotCreateAStoreForAFreshProject, which protects
	// lock-ledger-ABSENT's evidence), while the sweep DOES create the digest
	// store. So up to its first lock a project sits with a digest store and no
	// coverage, and there the comment gate can be cleared by deleting the digest
	// store: nothing reports the absence, and the next run re-adopts whatever
	// the claims say.
	//
	// Reproduced end to end on a fresh project: a draft claim with an open human
	// thread refuses to lock (unresolved_comments); hand-forge the thread to
	// `status: resolved` and `check` correctly reports comment-ledger-drift;
	// delete .dossierx-comment-digest.json and the next `check` reports ok:true,
	// re-creates the store around the FORGED block, and `claim lock` then
	// succeeds, leaving check --validate permanently clean. In a project that
	// holds even one locked claim the same three edits are refused.
	//
	// Stamping the version here to close it was tried and REJECTED, on evidence:
	// it converts lock-ledger-absent into lock-ledger-missing for a project
	// whose ledger was deleted (eroding exactly the signal the test above
	// protects), and it does not actually close the hole — deleting BOTH stores
	// instead of one restores the crossing and launders the same forged thread
	// just as completely. Raising the price from one deleted file to two is not
	// worth a real diagnosis.
	//
	// Closing it properly needs evidence this directory cannot hold, for the
	// same reason LedgerDowngraded's own "what it does not close" paragraph
	// gives: when every file that could testify is deleted in one commit, the
	// evidence is gone. The place that evidence still exists is version control,
	// which is why `check --staged` reads the git index — the pre-commit hook
	// and CI see the deletion of a tracked store as a staged deletion even when
	// a worktree run cannot.
	return changed
}

// SweepCommentDigests extends COMMENT DIGEST COVERAGE to every claim that has
// none, and releases the entries of claims whose departure is accounted for. It
// runs on every PrepareStore — i.e. on every command that opens the lock store
// for writing, including the writing `dossierx check` — and reports the ids it
// adopted and forgot.
//
// WHY IT IS A SWEEP AND NOT A ONE-TIME ADOPTION. It replaces a function that
// adopted only at the single moment a project crossed from pre-ledger to
// ledger-covered, and only when the digest file did not exist. That made
// coverage a set fixed at a project's FIRST lock, and never extended again:
// Store.Save's ensureCommentDigestStore creates the file (empty) on that first
// lock, so from then on the file existed, so nothing could ever adopt into it.
// Two things followed, both reproduced:
//
//   - a claim authored after that moment was covered by NOTHING. A hand-written
//     `id: c-fake01 / status: resolved / author: human / resolved_by: human`
//     thread appended to a LOCKED, uncovered claim passed `check --validate`
//     with zero findings and rendered in `comment list` as a genuine resolved
//     human review.
//   - RENAMING a covered claim reset it to uncovered. Delete the comments block
//     and change `id:` in the same edit and the drift finding that fires for the
//     deletion alone (comment-ledger-drift) does not fire at all, because the
//     new id is a claim the store has never seen.
//
// Adopting a claim's current block is exactly the strength digest.Adopt already
// promises on the migration path, and it is strictly stronger than leaving the
// claim unknown forever. It can only ADD entries — digest.Adopt skips every id
// that already has one — so a recorded digest can never be laundered by this,
// which is the property that keeps the sweep from becoming the bypass it exists
// to close. What it does NOT establish is that anybody approved the block it
// adopted, the same honest caveat grandfathering carries; the rename above is
// caught by the entry the tamper cannot reach, the OLD id's, which the second
// half of this function deliberately leaves in place.
//
// WHAT ADOPTION IS NOT ALLOWED TO REACH. "It can only ADD entries" is true and
// was not enough, because there are two ways to turn a RECORDED entry into an
// absent one, and both were free:
//
//	edit a locked claim's comments by hand   -> comment-ledger-drift, correctly
//	rm .dossierx-comment-digest.json         -> comment-digest-absent, correctly
//	dossierx check (the writing form)        -> this sweep re-derived the whole
//	                                            store from the claim files: both
//	                                            findings gone, ok:true, and the
//	                                            commit then passes the hook and CI
//
// The same run with the file left in place and the ONE tampered id deleted from
// its map ends identically, and is cheaper to hide in a review diff. A human's
// open objection on a LOCKED claim is erased permanently by an ordinary command
// nobody would look twice at.
//
// So this sweep asks the question internal/comments' recordCommentDigest already
// asks at its own adoption point (see the ledgerCovered guard there):
//
//   - ledgerCovered: in a project that has been through a ledger-aware build, an
//     ABSENT digest store is a deleted one, not an upgrade — exactly the rule
//     PreLedger applies to the lock ledger itself, where "empty means bless
//     everything" would make `rm` the universal bypass. Nothing is adopted, the
//     file is left absent so check can report comment-digest-absent, and the
//     recovery is version control (or the reason-carrying `comment reaudit
//     --claim`), never an adoption an attacker can trigger.
//
// and then, per claim, the two questions adoptableClaims answers: does this claim
// hold a STANDING approval (whose digest was recorded at the instant of that
// approval, so a missing entry is a finding), and does it CARRY THREADS AT ALL
// (which only the engine can put there, and only while recording the digest in
// the same act)? See adoptableClaims for both, and for the one-deleted-key
// launder the second closes.
//
// The per-id half is skipped on the UPGRADE CROSSING (a fresh project with no
// stores at all) and only there — nothing in such a project has been approved or
// reviewed, so there is nothing an adoption could launder. A PRE-LEDGER project
// is not a crossing and is not adopted here at all: its digest store is created
// by CrossPreLedger, EMPTY, in the same act that stamps the schema — and by then
// the project holds nothing locked, so there is nothing left for a filter to
// leave uncovered.
//
// THE RELEASE HALF is the only caller of digest.Store.Forget, and it is narrow
// on purpose: an entry is dropped only when its claim is gone AND its departure
// erased no review history (see AbandonedCommentDigests). Everything else stays,
// as the evidence internal/check's comment-digest-abandoned rule reads.
//
// Best-effort, like every other digest write reached from this package: a
// project whose digest store cannot be written is not one whose command should
// fail, and the consequence of a skipped sweep is a claim that reads as
// uncovered — never one that reads as approved.
func SweepCommentDigests(s *Store, claims []model.Claim, ledgerCovered bool) (adopted, forgotten []string) {
	if s == nil || s.path == "" {
		return nil, nil
	}
	path := digest.StorePathBeside(s.path)

	release, err := AcquireFileLock(path)
	if err != nil {
		return nil, nil
	}
	defer release()

	store, err := digest.LoadStore(path)
	if err != nil {
		return nil, nil
	}

	// The upgrade crossing: a FRESH project — one that has never been through a
	// ledger-aware build and has no lock store either, so nothing here has ever
	// been approved. This is the one state in which absence is honest, and the one
	// in which the whole project is adopted at once.
	//
	// A PRE-LEDGER project is deliberately NOT a crossing here, and the
	// !s.PreLedger() term is what excludes it. While grandfathering was implicit
	// the two crossed together inside PrepareStore — the ledger was stamped to
	// schema 2 and this created the digest store in the same run — so the pair was
	// always consistent. Creating the digest store here now would leave a v0.2.x
	// project carrying a comment digest store beside a version-1 lock store, which
	// is precisely the contradiction LedgerDowngraded is built to detect: the very
	// next `dossierx check` would report lock-ledger-downgraded against a project
	// whose only crime was predating the ledger. The digest store for such a
	// project is written by CrossPreLedger, with the schema stamp, in one act.
	crossing := !store.FileExists() && !ledgerCovered && !s.PreLedger()

	// Neither case below matches the third state — a COVERED project with no
	// digest store, i.e. one that was deleted — and that is the fix: nothing is
	// adopted, and the file is left absent so check keeps reporting
	// comment-digest-absent. Re-creating it (even empty) would replace that
	// finding with silence, which is the deletion getting what it came for.
	switch {
	case store.FileExists():
		adopted = digest.Adopt(store, adoptableClaims(s, claims))
	case crossing:
		adopted = digest.Adopt(store, claims)
	}

	forgotten = releasableCommentDigests(claims, s, store)
	for _, id := range forgotten {
		store.Forget(id)
	}
	if crossing || len(adopted) > 0 || len(forgotten) > 0 {
		store.Save() //nolint:errcheck // best-effort: see this function's doc comment
	}
	return adopted, forgotten
}

// adoptableClaims is the subset of claims whose comment block adoption is
// allowed to reach, once the digest store EXISTS. Two kinds of claim are held
// back, and each one is a laundering path that adoption would otherwise walk:
//
//   - a claim holding a STANDING (unreleased) lock-ledger record, whose digest
//     was written at the moment of that approval (RecordApproval) and whose
//     absence is therefore a finding rather than an introduction. This is
//     internal/check's comment-digest-missing.
//
//   - a claim that CARRIES COMMENT THREADS. Comments are engine-managed: the one
//     path that puts a thread in a claim file (internal/comments' mutate) records
//     the claim's digest in the same act, so in a project whose digest store
//     exists, threads-without-an-entry is never "a claim the store has not met
//     yet" — the entry was removed, or the threads were written by hand. Adopting
//     there re-records whatever the block says NOW as the truth. Reproduced: on a
//     fully covered project, forge `status: resolved / resolved_by: human` onto a
//     human's open thread and `check` reports comment-ledger-drift; delete that
//     ONE key from .dossierx-comment-digest.json (the file stays, so
//     comment-digest-absent cannot fire) and the next ordinary run re-adopted the
//     forged block, reported ok:true, and the claim locked — defeating the gate
//     the whole review loop rests on. It is now audit.go's
//     RuleCommentDigestUnrecorded instead, and this filter is what stops an
//     ordinary command from clearing it.
//
// What stays adoptable is the case adoption exists for: a claim with NO threads
// and no entry, which is adopted at its EMPTY digest — the value that makes a
// thread hand-added to it afterwards report as drift. So a claim authored after
// the crossing is still covered without anybody commenting on it, which is the
// hole the sweep was written to close.
//
// It filters the CLAIMS rather than teaching digest.Adopt about the ledger:
// internal/digest is imported BY this package precisely so it knows nothing
// about locks, and the ledger is this package's business.
func adoptableClaims(s *Store, claims []model.Claim) []model.Claim {
	out := make([]model.Claim, 0, len(claims))
	for _, c := range claims {
		if rec, ok := ledgerRecordFor(s, c.ID); ok && !rec.Released() {
			continue
		}
		if len(c.Comments) > 0 {
			continue
		}
		out = append(out, c)
	}
	return out
}

// AbandonedCommentDigests returns the ids whose COMMENT DIGEST entry survives a
// claim that is no longer in the project AND still describes review history
// somebody would have to answer for — sorted.
//
// It is the reverse sweep for comments, symmetric with the ledger's
// lock-ledger-abandoned, and it is what makes the rename launder visible: the
// renamed claim's OLD entry is not reachable from the claim file the tamper
// rewrote, so it survives the edit and says that a claim with threads used to be
// here.
//
// Two departures are ACCOUNTED FOR and are not reported, which is what keeps the
// rule off correct state:
//
//   - the entry records NO threads (it hashes to digest.EmptyCommentsDigest for
//     its own id). Deleting that claim erased no review history at all, so there
//     is nothing for a human to look at.
//   - the claim's lock-ledger record was RELEASED by an honest unlock. That is
//     the documented deliberate-removal path (restore, unlock on the record, then
//     delete), and lock-ledger-abandoned already treats a released record the
//     same way. Reporting it here would refuse the one flow the other rule tells
//     people to use.
//
// A store that is nil (unreadable, already reported) or a claim registry the
// caller assembled as a SUBSET yields nothing: like every other reverse sweep
// here, it is skipped when the store file is absent, so an in-memory store can
// never have every claim the caller did not pass reported as deleted.
func AbandonedCommentDigests(claims []model.Claim, store *Store, digests *digest.Store) []string {
	if store == nil || !store.FileExists() || digests == nil || !digests.FileExists() {
		return nil
	}
	present := make(map[string]bool, len(claims))
	for _, c := range claims {
		present[c.ID] = true
	}
	var out []string
	for id, recorded := range digests.Digests {
		if present[id] || commentDigestReleased(id, recorded, store) {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// releasableCommentDigests is AbandonedCommentDigests' complement: the entries
// for departed claims whose departure IS accounted for, which the sweep drops so
// the store does not accumulate them forever. Keeping the two in one file, built
// from the same predicate, is what stops the gate and the sweep from disagreeing
// about which orphan is evidence.
func releasableCommentDigests(claims []model.Claim, store *Store, digests *digest.Store) []string {
	if store == nil || digests == nil {
		return nil
	}
	present := make(map[string]bool, len(claims))
	for _, c := range claims {
		present[c.ID] = true
	}
	var out []string
	for id, recorded := range digests.Digests {
		if present[id] || !commentDigestReleased(id, recorded, store) {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// commentDigestReleased reports whether a digest entry for a claim that is no
// longer in the project describes a departure nobody has to answer for. See
// AbandonedCommentDigests for both halves.
func commentDigestReleased(id, recorded string, store *Store) bool {
	if recorded == digest.EmptyCommentsDigest(id) {
		return true
	}
	r, ok := store.Record(id)
	return ok && r.Subject == SubjectClaim && r.Released()
}
