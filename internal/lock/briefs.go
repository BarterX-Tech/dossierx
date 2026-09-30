package lock

import (
	"sort"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// briefs.go is the lock store's half of a brief's approval (NIT-205, the
// second half of NIT-192): the `briefs` map inside build/ledger/lock-store.json,
// one BriefRecord per brief id (<folder>.<slug>).
//
// It is DATA ONLY, and it is here rather than in internal/briefs for one
// reason: the store is one file with one strict decoder, and the decoder has to
// know every key the file may carry. Nothing in this file reads a brief, parses
// markdown or imports internal/briefs — the lock package still never learns
// what a brief is. internal/briefs computes a record's hash and baselines and
// hands them here; this file only keeps them.
//
// THE APPROVAL CONTRACT, which is why the map is a sibling of the claim ledger
// and never inside it (the constitution's record set the precedent):
//
//   - No claim-side function reads Briefs. ContentHash, LockedClaimHash, Audit,
//     DetectStale, readiness and every ledger rule iterate Hashes, Receipts,
//     LockedAt and Ledger only, so a brief record can neither approve nor block
//     a claim, and no brief byte enters any claim's ContentHash or
//     LockedClaimHash.
//   - A brief's baselines are claim ContentHashes the brief READ at lock time.
//     They point from the brief to the claim and are compared in one
//     direction: a moved claim makes the BRIEF review-pending. Nothing flows
//     back, and the claim graph never sees the edge.

// briefsSchemaVersion is the store schema at which the `briefs` map landed.
//
// A store EARNS it: the version moves to 4 only when a brief record is first
// written (RecordBriefApproval), never on an ordinary claim write. A project
// that never locks a brief keeps a version-3 store byte for byte, which is what
// "a project with no briefs sees no change" means for this file, and what keeps
// a v0.7.21 binary able to read it. A version-3 store is read as it always was;
// the version-4 read path is the version-3 one plus this map.
const briefsSchemaVersion = 4

// BriefApproved is the brief as it was approved: the three fields its lock
// hash signs, kept as text so an edited-since-approval brief can show what the
// human actually approved beside what it says now (the viewer's Changes /
// Approved / Current views, NIT-199). It is the brief's counterpart of
// LedgerRecord.Content.
type BriefApproved struct {
	Summary  string   `json:"summary"`
	RestsOn  []string `json:"rests_on"`
	Markdown string   `json:"markdown"`
}

// BriefReaudit is one confirmed `brief reaudit --confirm`: the human's words,
// when, and the claims whose moved content they accepted.
type BriefReaudit struct {
	At     string   `json:"at"`
	Actor  string   `json:"actor"`
	Reason string   `json:"reason"`
	Claims []string `json:"claims"`
}

// BriefRecord is one brief's approval on the record.
type BriefRecord struct {
	// Path is the brief's path relative to the config directory when it was
	// locked, so a finding about a record whose file is gone can name it.
	Path string `json:"path"`

	// Hash is the brief's lock hash (briefs.LockHash): summary, rests_on and
	// body. Status and comment threads are not signed, exactly as a claim's
	// status and comments are not.
	Hash   string `json:"hash"`
	At     string `json:"at"`
	Actor  string `json:"actor"`
	Reason string `json:"reason"`

	// Approved is the text behind Hash.
	Approved BriefApproved `json:"approved"`

	// Baselines is ContentHash of every rests_on claim as of the lock or the
	// last confirmed reaudit, keyed by claim id — the same shape as a claim's
	// dependency baselines (Store.Hashes[dependent]). Receipts keeps each of
	// those claims as it read then, so the wording a baseline stands for is
	// recoverable after the claim moves (the viewer's per-claim redline,
	// NIT-200), exactly as DependencyReceipt does for a claim.
	Baselines map[string]string      `json:"baselines"`
	Receipts  map[string]model.Claim `json:"receipts,omitempty"`

	// Reaudits is every confirmed reaudit since the lock, oldest first.
	Reaudits []BriefReaudit `json:"reaudits,omitempty"`

	// Released* stamp an unlock, as on a claim's LedgerRecord: the record is
	// kept, so the evidence that the brief was ever approved survives.
	ReleasedAt     string `json:"released_at,omitempty"`
	ReleasedBy     string `json:"released_by,omitempty"`
	ReleasedReason string `json:"released_reason,omitempty"`
}

// Released reports whether an unlock released this record.
func (r BriefRecord) Released() bool { return r.ReleasedAt != "" }

// BriefRecordFor returns the record for a brief id and whether one exists.
func (s *Store) BriefRecordFor(id string) (BriefRecord, bool) {
	if s == nil || s.Briefs == nil {
		return BriefRecord{}, false
	}
	r, ok := s.Briefs[id]
	return r, ok
}

// RecordBriefApproval writes (or overwrites) a brief's record: a lock, or a
// re-lock of an edited brief. Any earlier release stamp is dropped — a re-lock
// is a new approval — and the store earns the briefs schema.
//
// The caller has refused a pre-ledger store before reaching here
// (Store.PreLedger): stamping version 4 onto one would carry it past the
// ledger crossing without a single claim record, the hole MigrateLegacyStore's
// doc describes for the same stamp.
func RecordBriefApproval(store *Store, id string, rec BriefRecord) {
	if store.Briefs == nil {
		store.Briefs = map[string]BriefRecord{}
	}
	rec.ReleasedAt, rec.ReleasedBy, rec.ReleasedReason = "", "", ""
	store.Briefs[id] = rec
	store.earnBriefsSchema()
}

// RecordBriefReaudit refreshes a standing record's baselines and receipts to
// the claims as they read now and appends the human's confirmation. The
// brief's own hash, approval time and approved text are untouched: a reaudit
// accepts moved claims, it does not re-approve the brief's words.
func RecordBriefReaudit(store *Store, id string, baselines map[string]string, receipts map[string]model.Claim, ap Approval, claimIDs []string) bool {
	r, ok := store.BriefRecordFor(id)
	if !ok || r.Released() {
		return false
	}
	r.Baselines = baselines
	r.Receipts = receipts
	r.Reaudits = append(r.Reaudits, BriefReaudit{
		At:     nowFunc().UTC().Format(time.RFC3339Nano),
		Actor:  ap.Actor,
		Reason: ap.Reason,
		Claims: append([]string{}, claimIDs...),
	})
	store.Briefs[id] = r
	store.earnBriefsSchema()
	return true
}

// ReleaseBriefApproval stamps an unlock on a brief's record, keeping it. It
// reports whether there was a standing record to release; false is not an
// error, because unlock is the escape hatch and must always work.
func ReleaseBriefApproval(store *Store, id string, ap Approval) bool {
	r, ok := store.BriefRecordFor(id)
	if !ok || r.Released() {
		return false
	}
	r.ReleasedAt = nowFunc().UTC().Format(time.RFC3339Nano)
	r.ReleasedBy = ap.Actor
	r.ReleasedReason = ap.Reason
	store.Briefs[id] = r
	store.earnBriefsSchema()
	return true
}

// earnBriefsSchema raises the store to the briefs schema, never lowers it.
func (s *Store) earnBriefsSchema() {
	if s.Version < briefsSchemaVersion {
		s.Version = briefsSchemaVersion
	}
}

// RetainedClaimContent returns a claim snapshot this store holds whose
// ContentHash is hash, looked for in every place the store keeps claim text:
// the brief records' receipts, the claim dependency receipts, and the claim
// ledger's approved content. It is how a brief's baseline hash is turned back
// into wording when the brief's own receipt is missing; it never reads git.
// The search is one pass over what the store holds, and every candidate is
// re-hashed, so a snapshot is returned only when it IS that content.
func (s *Store) RetainedClaimContent(claimID, hash string) (model.Claim, bool) {
	if s == nil || hash == "" {
		return model.Claim{}, false
	}
	match := func(c model.Claim) bool { return c.ID == claimID && ContentHash(c) == hash }
	ids := make([]string, 0, len(s.Briefs))
	for id := range s.Briefs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if c, ok := s.Briefs[id].Receipts[claimID]; ok && match(c) {
			return c, true
		}
	}
	deps := make([]string, 0, len(s.Receipts))
	for id := range s.Receipts {
		deps = append(deps, id)
	}
	sort.Strings(deps)
	for _, id := range deps {
		if r, ok := s.Receipts[id][claimID]; ok && match(r.Content) {
			return r.Content, true
		}
	}
	if c, ok := s.ApprovedContent(claimID); ok && match(c) {
		return c, true
	}
	return model.Claim{}, false
}
