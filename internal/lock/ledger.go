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
//	swap raw_html on a locked mockup       rendered UNESCAPED, and ContentHash
//	                                       does not cover raw_html at all
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

	// Grandfathered marks a record ADOPTED on upgrade rather than earned
	// through the approval path — its Hash is whatever the claim happened to
	// contain on adoption day, which is NOT evidence that a human ever approved
	// those exact bytes. It stays on the record permanently so nobody
	// mistakes an adoption for an approval, and so a project can find and
	// re-lock its grandfathered claims deliberately.
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

// ledgerAnnounceWriter is where grandfathering announces itself. It is a
// package-level var rather than a writer threaded through every caller
// precisely BECAUSE adoption must never be silent: adoption happens deep inside
// a store-load path reached from five different commands, and an announcement
// that each caller has to remember to print is an announcement that one of them
// will eventually forget. Tests redirect it; nothing else should.
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
// build that PREDATES the lock ledger — the one condition under which adopting
// existing locks is honest rather than a bypass.
//
// It is the same predicate AdoptLedger keys on, exported because build-order
// artifacts have to be grandfathered on exactly the same terms and cannot be
// reached from this package (internal/buildorder imports internal/lock, so the
// edge cannot run the other way). Callers must consult it BEFORE PrepareStore,
// which stamps the current version as its last act.
//
// Both halves matter. A store at the current version never adopts again: after
// the upgrade, a locked artifact without a record is a finding, not an
// invitation. And an ABSENT store never adopts at all, because absence is
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
//     PrepareStore's adoptCommentDigests for a project that migrates across, and
//     Store.Save's ensureCommentDigestStore for a fresh project that crosses on
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

// PreLedgerExempt reports whether a locked artifact WITHOUT a ledger record in
// this store is an UPGRADE STATE — a project that locked things before this
// build gave locks a record — rather than a tamper.
//
// It is the read-only path's half of grandfathering, and it is the answer to a
// gate that used to accuse every honest v0.2.x project of tampering. Adoption
// runs in the CLI's WRITE path (PrepareStore), so `check --validate` and
// `check --staged` — both of which write nothing, on purpose — saw a pre-ledger
// project as N locked claims with no approval records and reported each one as
// lock-ledger-missing, whose recovery text says to set the claim back to draft
// and re-lock it. That is destructive advice for a project that has done nothing
// wrong, and it made the pre-commit hook refuse every commit until the human
// followed it. A read-only command must not demand a write in order to pass, and
// it certainly must not demand THAT one.
//
// So the read-only paths grandfather IN MEMORY on exactly the terms the write
// path adopts on disk: same predicate, same evidence, no file touched. What the
// human sees instead is a next-step advisory (see internal/check's nextSteps)
// telling them to run the write path once so the adoption is on the record.
//
// It is a conjunction, not a synonym for PreLedger: a store whose pre-ledger
// claim is CONTRADICTED (see LedgerDowngraded) is exempt from nothing, so the
// downgrade attack cannot buy silence on the read-only path either.
func (s *Store) PreLedgerExempt(digestStorePresent bool) bool {
	return s.PreLedger() && !s.LedgerDowngraded(digestStorePresent)
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
// PrepareStore, which creates the digest store at the same moment it stamps this
// version so an upgrading project crosses both lines together.
func (s *Store) LedgerCovered() bool {
	return s != nil && s.fileExists && s.diskVersion >= ledgerSchemaVersion
}

// AdoptBuildOrderApproval grandfathers one module's already-locked build-order
// artifact into the ledger, and reports whether it wrote anything.
//
// It is the build-order twin of AdoptLedger's per-claim adoption, for projects
// that locked a build order before this release gave build orders a record. The
// Grandfathered flag stays on permanently and says honestly what was
// established: these are the bytes that were on disk on adoption day, not bytes
// anybody approved.
//
// An existing record is never overwritten — an adoption must not be able to
// quietly replace a real approval.
func AdoptBuildOrderApproval(store *Store, module, hash string) bool {
	if store == nil {
		return false
	}
	key := BuildOrderLedgerKey(module)
	if _, exists := store.Record(key); exists {
		return false
	}
	store.putRecord(key, LedgerRecord{
		Subject:       SubjectBuildOrder,
		Hash:          hash,
		At:            nowFunc().UTC().Format(time.RFC3339Nano),
		Actor:         DefaultActor(),
		Reason:        "grandfathered: this build order was locked before this project had a lock ledger; content adopted as-found, never approved",
		Grandfathered: true,
	})
	return true
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

// AdoptLedger performs the ONE-TIME grandfathering of a project that locked
// claims before the ledger existed: every currently-locked claim with no record
// gets one, marked Grandfathered, and the adoption is announced loudly on
// stderr. It returns the adopted claim ids (sorted) so a caller can also surface
// them in a machine envelope.
//
// The trigger is deliberately "the store file EXISTS and its on-disk schema
// version predates the ledger", NOT "the ledger map is empty". The distinction
// is the whole security property:
//
//   - A store file that exists at an older version is evidence that this
//     project's locks were made by an older build, which had no ledger to write
//     to. Adopting them is the only way to upgrade without demanding every
//     project re-lock every claim by hand, and the Grandfathered flag records
//     honestly that these hashes were observed, not approved.
//
//   - An ABSENT store file with locked claims in it is the opposite: it is
//     indistinguishable from someone deleting the ledger to re-bless a tampered
//     project. "Empty ledger means adopt everything" would make deletion the
//     universal bypass — the attack would be `rm .dossierx-lock-store.json`.
//     So absence never adopts. Those claims surface from the gate as
//     lock-ledger-missing (and the project-scoped lock-ledger-absent), which is
//     a refusal a human has to look at.
//
//   - A store already at the ledger version never adopts again, no matter what
//     its ledger contains. After the upgrade, a locked claim WITHOUT a record is
//     a finding, not an invitation.
//
//   - A store whose pre-ledger claim is CONTRADICTED by the project around it
//     never adopts either. The version field lives in the audited file, so
//     without this the trigger was re-armable with a text editor: set the
//     version back to 1, delete the ledger key, and the next command adopted
//     whatever the tampered claims said as approved. See LedgerDowngraded for
//     the two pieces of evidence a downgrade has to survive, and for what this
//     does not close.
//
// The caller must hold the store's file lock across adopt-and-Save, like any
// other load-mutate-save on the shared store file.
func AdoptLedger(s *Store, claims []model.Claim) (adopted []string) {
	if s == nil || s.diskVersion >= ledgerSchemaVersion {
		return nil
	}
	if !s.fileExists {
		// Fresh project (nothing locked yet) or a deleted ledger (something
		// locked). Either way: adopt nothing. See this function's doc comment.
		return nil
	}
	if s.LedgerDowngraded(digestStorePresentBeside(s.path)) {
		// This project has already been through a ledger-aware build, whatever
		// its version field now says. Adopt nothing, say so, and leave the gate
		// to report every locked claim as unrecorded — which is the honest
		// description of a project whose approval records are gone.
		announceDowngradeRefusal(s.path)
		return nil
	}

	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		if _, exists := s.Record(c.ID); exists {
			continue
		}
		s.putRecord(c.ID, LedgerRecord{
			Subject:       SubjectClaim,
			Hash:          LockedClaimHash(c),
			At:            nowFunc().UTC().Format(time.RFC3339Nano),
			Actor:         DefaultActor(),
			Reason:        "grandfathered: locked before this project had a lock ledger; content adopted as-found, never approved",
			Grandfathered: true,
		})
		adopted = append(adopted, c.ID)
	}
	sort.Strings(adopted)

	// Stamp the in-memory disk version even when nothing was adopted (a
	// pre-ledger store whose claims are all draft), so the next load sees a
	// current store and this branch never runs twice. The Save that persists
	// Version is the caller's; see PrepareStore's changed return.
	s.diskVersion = ledgerSchemaVersion
	s.Version = storeSchemaVersion

	if len(adopted) > 0 {
		announceAdoption(adopted)
	}
	return adopted
}

// announceAdoption writes the one-time grandfathering notice. It is loud on
// purpose and says what was and was NOT established: adoption proves only that
// these were the bytes present on adoption day. Anything tampered with BEFORE
// the upgrade is adopted along with everything else — that is unavoidable (no
// record of the original exists to compare against) and is exactly why the
// notice tells the human to review, and why Grandfathered stays on the record.
func announceAdoption(adopted []string) {
	if ledgerAnnounceWriter == nil {
		return
	}
	fmt.Fprintf(ledgerAnnounceWriter,
		"dossierx: lock ledger created — %d already-locked claim(s) adopted as grandfathered.\n"+
			"  Their recorded content is what was on disk just now, NOT content anyone approved:\n"+
			"  any edit made before this upgrade is adopted with it. From here on, every change to\n"+
			"  a locked claim is detected. Review the adopted claims, and re-lock any you are not\n"+
			"  sure of (dossierx claim unlock <id> --reason ... then dossierx claim lock <id> --reason ...):\n",
		len(adopted))
	for _, id := range adopted {
		fmt.Fprintf(ledgerAnnounceWriter, "    %s\n", id)
	}
}

// announceDowngradeRefusal writes the notice for the other half of AdoptLedger's
// decision: a store that asked to be grandfathered and was refused.
//
// It is as loud as the adoption notice, and for the sharper reason. Adoption is
// a one-time upgrade event; a refused adoption means the lock store on disk
// disagrees with the rest of the project about whether this project has ever had
// a ledger, and the only two ways to reach that state are a hand-edited store
// and a partly-restored one. Both need a human. The recovery named is version
// control, never re-locking — re-locking records whatever the claims say NOW,
// which is precisely what the edit was for.
func announceDowngradeRefusal(path string) {
	if ledgerAnnounceWriter == nil {
		return
	}
	fmt.Fprintf(ledgerAnnounceWriter,
		"dossierx: %s says it predates the lock ledger, but this project has already been through a\n"+
			"  ledger-aware build (its comment digest store exists, or the store still carries the\n"+
			"  ledger key, which did not exist before the ledger). Nothing was grandfathered: a\n"+
			"  store's own version field must not be able to re-arm adoption, or editing one number\n"+
			"  would re-bless every locked claim as-found.\n"+
			"  Restore the lock store from version control — do NOT re-lock, which would record whatever\n"+
			"  the claims say now as approved.\n",
		path)
}

// PrepareStore runs BOTH on-load store migrations, in the one correct order,
// and is what every command that opens the store for writing should call. It
// exists so the two migrations can never be run separately or in the wrong
// order by a caller that only remembered one of them:
//
//  1. MigrateLegacyStore re-arms per-dependent DEPENDENCY baselines for a store
//     that predates them (schema 0). This must run first: it is the older
//     migration and it keys on the baselines map being empty, a condition
//     ledger adoption does not touch.
//
//  2. AdoptLedger grandfathers already-locked claims into the ledger for a
//     store that predates IT (schema < 2).
//
// It reports changed=true if either migration modified the store, so the caller
// knows to Save; adopted carries the grandfathered ids for the envelope.
//
// It also runs the COMMENT DIGEST COVERAGE SWEEP, on every call rather than only
// on a migration — see SweepCommentDigests for why coverage that only ever
// extended at a project's first lock was a hole rather than a conservative
// default.
func PrepareStore(s *Store, claims []model.Claim) (changed bool, adopted []string) {
	changed = MigrateLegacyStore(s, claims)

	// A store whose pre-ledger claim is contradicted (LedgerDowngraded) is NOT
	// stale, and the distinction decides whether this run rewrites the file.
	// Treating it as stale would stamp the current version over the downgrade —
	// silently destroying the one piece of evidence that says an edit happened,
	// and demoting the gate's precise "this store was downgraded" report to N
	// anonymous lock-ledger-missing findings on the very next run. Leaving the
	// file exactly as found keeps the refusal reproducible until a human
	// restores the store from version control.
	stale := s.fileExists && s.diskVersion < ledgerSchemaVersion &&
		!s.LedgerDowngraded(digestStorePresentBeside(s.path))

	// Whether this project was ALREADY ledger-covered is read HERE, before
	// AdoptLedger — which stamps the current schema version onto the store as
	// its last act, so asking afterwards would be asking a question this run has
	// already changed the answer to. A DOWNGRADED store counts as covered: it is
	// a project that has been through a ledger-aware build whatever its version
	// field now says, and letting the downgrade re-open comment adoption would
	// hand the same edit a second laundering path.
	//
	// It is the sweep's absence rule (see SweepCommentDigests): in a covered
	// project a missing digest store is a DELETED one, and adopting there
	// re-derives coverage from the files the deletion was meant to bless.
	coveredBefore := s.LedgerCovered() || s.LedgerDowngraded(digestStorePresentBeside(s.path))

	adopted = AdoptLedger(s, claims)
	if len(adopted) > 0 || stale {
		changed = true
	}
	s.commentDigestsAdopted, _ = SweepCommentDigests(s, claims, coveredBefore)

	// KNOWN GAP, recorded here because it is invisible from the rules
	// themselves: every comment-digest guard above is armed by coveredBefore,
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
	return changed, adopted
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
// So this sweep now asks the two questions internal/comments' recordCommentDigest
// already asks at its own adoption point (see the ledgerCovered guard there):
//
//   - ledgerCovered: in a project that has been through a ledger-aware build, an
//     ABSENT digest store is a deleted one, not an upgrade — exactly the rule
//     AdoptLedger has always applied to the lock ledger itself, where "empty
//     means adopt everything" would make `rm` the universal bypass. Nothing is
//     adopted, the file is left absent so check can report comment-digest-absent,
//     and the recovery is version control (or the reason-carrying `comment
//     reaudit --claim`), never an adoption an attacker can trigger.
//   - a STANDING approval: a claim whose lock-ledger record is unreleased had its
//     comment digest recorded at the instant of that approval (RecordApproval),
//     so a missing entry for it is never "a claim the store has not met yet" —
//     it is comment-digest-missing, a finding, and adopting would clear it.
//     Released records are not standing: an unlocked claim is a draft again, and
//     its ordinary edits must stay free.
//
// The per-id half is skipped on the UPGRADE CROSSING (a pre-ledger project whose
// digest store does not exist yet) and only there. AdoptLedger runs first inside
// PrepareStore, so by the time the sweep sees that project every grandfathered
// claim already holds a standing record — applying the filter there would leave
// every locked claim in an honest v0.2.x project permanently uncovered, which is
// the same outage the pre-ledger exemption exists to avoid.
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

	// The upgrade crossing: a project that has never been through a
	// ledger-aware build, whose digest store therefore does not exist yet. This
	// is the one state in which absence is honest, and the one in which the
	// whole project is adopted at once.
	crossing := !store.FileExists() && !ledgerCovered

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
// allowed to reach: everything except a claim holding a STANDING (unreleased)
// lock-ledger record, whose digest was written at the moment of that approval
// and whose absence is therefore a finding rather than an introduction. See
// SweepCommentDigests.
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
