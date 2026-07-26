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
func PrepareStore(s *Store, claims []model.Claim) (changed bool, adopted []string) {
	changed = MigrateLegacyStore(s, claims)
	stale := s.fileExists && s.diskVersion < ledgerSchemaVersion
	adopted = AdoptLedger(s, claims)
	if len(adopted) > 0 || stale {
		changed = true
	}
	return changed, adopted
}
