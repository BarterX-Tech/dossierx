package briefs

import (
	"fmt"
	"sort"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// lockstate.go is a brief's lock lifecycle read against the lock store
// (NIT-205): which state each brief is in, whether it is review-pending, and
// the four findings those raise.
//
// EVERYTHING HERE IS A READ. Evaluate takes the claims and the store and
// returns values; it writes nothing, refreshes no baseline, and never changes a
// claim. The only direction anything flows is claim -> brief: a claim whose
// ContentHash moved since a brief's baseline makes that BRIEF review-pending.
// No brief state reaches a claim, the claim graph or any claim hash.
//
// Two of the findings are integrity findings — a locked brief whose approved
// content moved, or one with no approval behind it — and ride in check's
// ledger_findings, keyed by `rule`, refused through integrity_failed like the
// claim ledger's own. They are declared here, not in rules.go, because they are
// not brief lints: FORMAT.md's findings table lists every rule this file and
// the two ledger files declare (tests/docs_site_audit_test.go reads all three).
// The other two, brief-rests-on-missing and brief-dependency-drift, are lints in
// the brief rule set (rules.go) and ride in lint_findings.
const (
	// RuleContentDrift: a brief with status locked and a standing record
	// whose LockHash no longer matches the record. The recovery is the
	// human's: re-lock it (`brief lock` signs the edit) or restore the file.
	RuleContentDrift = "brief-content-drift"
	// RuleUnrecorded: a brief with status locked and no standing record — no
	// record at all, or one an unlock released. `status: locked` typed by
	// hand approves nothing.
	RuleUnrecorded = "brief-unrecorded"
)

// LockState is a brief's state against its record.
type LockState string

const (
	// LockDraft: status draft. Never review-pending, whatever its record says.
	LockDraft LockState = "draft"
	// LockLocked: status locked, a standing record, and the hash agrees.
	LockLocked LockState = "locked"
	// LockEdited: status locked, a standing record, and the hash has moved —
	// edited since approval (brief-content-drift).
	LockEdited LockState = "edited"
	// LockUnrecorded: status locked with no standing record (brief-unrecorded).
	LockUnrecorded LockState = "unrecorded"
)

// UnavailableWording is what a baseline's wording reads when neither the
// brief's record nor anything else the store retains holds a claim snapshot
// with that hash. The viewer shows it verbatim.
const UnavailableWording = "earlier wording not available"

// Review triggers, the value of Review.ReviewPendingTrigger.
const (
	TriggerDependencyDrift = "dependency_drift"
	TriggerRestsOnMissing  = "rests_on_missing"
)

// Wording is the part of a claim a reader of a brief compares: its summary,
// its body and its steps.
type Wording struct {
	Summary string   `json:"summary"`
	Body    string   `json:"body"`
	Steps   []string `json:"steps"`
}

func wordingOf(c model.Claim) *Wording {
	steps := c.Steps
	if steps == nil {
		steps = []string{}
	}
	return &Wording{Summary: c.Summary, Body: c.Body, Steps: steps}
}

// ChangedClaim is one rests_on claim whose content moved since the brief's
// baseline, or that no longer exists.
type ChangedClaim struct {
	ID string `json:"id"`
	// Missing is true when no claim carries this id any more.
	Missing bool `json:"missing"`
	// BaselineHash is the claim's ContentHash when the brief was locked or
	// last reaudited; CurrentHash is what it hashes to now ("" when missing).
	BaselineHash string `json:"baseline_hash"`
	CurrentHash  string `json:"current_hash"`
	// ChangedAt is when the claim's CURRENT content was approved — its
	// standing ledger record's time — and "" when that content has no
	// standing approval (a draft claim, a locked claim edited since its
	// approval, or a missing one): the engine keeps no clock for an edit
	// nobody has approved, and does not guess one.
	ChangedAt string `json:"changed_at"`
	// Baseline is the claim's wording at the baseline, from the brief's own
	// receipt or any other snapshot the store retains with that hash; nil,
	// with BaselineNote set to UnavailableWording, when none does. Current is
	// the claim's wording now (nil when missing).
	Baseline     *Wording `json:"baseline"`
	BaselineNote string   `json:"baseline_note,omitempty"`
	Current      *Wording `json:"current"`
}

// Review is one brief's lock and review state: what `brief list`, `brief show`
// and the viewer payload report.
type Review struct {
	LockState LockState `json:"lock_state"`
	// LockedAt, LockReason and LockedBy are the standing record's; empty for
	// a brief with none.
	LockedAt   string `json:"locked_at"`
	LockReason string `json:"lock_reason"`
	LockedBy   string `json:"locked_by"`
	// ReviewPending is true for a locked brief (standing record, status
	// locked) whose rests_on baselines no longer match the claims; a draft is
	// never pending. ReviewPendingTrigger is "" or one of the Trigger*
	// constants (dependency drift wins when both apply).
	ReviewPending        bool   `json:"review_pending"`
	ReviewPendingTrigger string `json:"review_pending_trigger"`
	// ChangedClaims is every baseline claim that moved or is gone, by id.
	ChangedClaims []ChangedClaim `json:"changed_claims"`
	// OpenThreads is the brief's unresolved comment threads.
	OpenThreads int `json:"open_threads"`
	// Approved is the retained approved text, present while a record stands.
	Approved *lock.BriefApproved `json:"approved"`
}

// Evaluation is every brief's Review plus the findings the lifecycle raises.
type Evaluation struct {
	Reviews   map[string]Review
	Lint      []lint.Finding
	Integrity []lock.Finding

	// baselined[path][claimID] is a claim id a standing, locked brief holds a
	// baseline for; brief-rests-on-unknown leaves those to
	// brief-rests-on-missing, so one gone claim is one finding.
	baselined map[string]map[string]bool
}

// Review returns the review for a brief, and a draft review for a brief the
// evaluation does not know.
func (e *Evaluation) Review(b Brief) Review {
	if e != nil {
		if r, ok := e.Reviews[b.ID]; ok {
			return r
		}
	}
	return Review{LockState: LockDraft, ChangedClaims: []ChangedClaim{}, OpenThreads: b.OpenThreads()}
}

// Evaluate reads every brief in set against store and claims. store may be nil
// (unreadable): every locked brief is then unrecorded, because there is no
// evidence it was approved, and no baseline is compared.
//
// Cost: one claim-id index, O(C); then per brief one record lookup and one
// ContentHash per baseline, O(N·R) hashes; a changed claim's baseline wording is
// one receipt lookup, or one pass over the store's retained snapshots
// (lock.Store.RetainedClaimContent). Nothing walks a path or a pair.
func Evaluate(set *Set, claims []model.Claim, store *lock.Store) *Evaluation {
	return evaluate(set, claims, store, true)
}

// EvaluateLocks is Evaluate without the claims: each brief's lock state,
// record and open threads, and the two integrity findings, but no baseline is
// compared, so nothing is review-pending and no drift or missing finding is
// raised. It is for a reader that could not load the claims and says so.
func EvaluateLocks(set *Set, store *lock.Store) *Evaluation {
	return evaluate(set, nil, store, false)
}

func evaluate(set *Set, claims []model.Claim, store *lock.Store, compare bool) *Evaluation {
	e := &Evaluation{Reviews: map[string]Review{}, baselined: map[string]map[string]bool{}}
	if set == nil {
		return e
	}
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}
	for _, b := range set.Briefs {
		r := Review{LockState: LockDraft, ChangedClaims: []ChangedClaim{}, OpenThreads: b.OpenThreads()}
		rec, has := store.BriefRecordFor(b.ID)
		standing := has && !rec.Released()
		if standing {
			r.LockedAt, r.LockReason, r.LockedBy = rec.At, rec.Reason, rec.Actor
			approved := rec.Approved
			if approved.RestsOn == nil {
				approved.RestsOn = []string{}
			}
			r.Approved = &approved
		}
		if b.Status != StatusLocked {
			e.Reviews[b.ID] = r
			continue
		}
		if !standing {
			r.LockState = LockUnrecorded
			e.Integrity = append(e.Integrity, unrecordedFinding(b, store, rec, has))
			e.Reviews[b.ID] = r
			continue
		}
		r.LockState = LockLocked
		if rec.Hash != b.LockHash {
			r.LockState = LockEdited
			e.Integrity = append(e.Integrity, lock.Finding{
				Rule:    RuleContentDrift,
				ClaimID: b.Path,
				Message: fmt.Sprintf("%s is locked, and its summary, rests_on or body has changed since it was approved on %s (%q). Its approved text is kept in %s. Either the human re-approves the edit — dossierx brief lock %s --reason \"<their words>\" signs the brief as it reads now — or restore the file from version control. Do not re-lock to make this go away without the human's yes: re-locking records whatever the file says now as approved.", b.Path, rec.At, rec.Reason, config.LockStoreDisplayPath, b.Path),
			})
		}
		if !compare {
			e.Reviews[b.ID] = r
			continue
		}
		e.baselined[b.Path] = map[string]bool{}
		ids := make([]string, 0, len(rec.Baselines))
		for id := range rec.Baselines {
			ids = append(ids, id)
			e.baselined[b.Path][id] = true
		}
		sort.Strings(ids)
		missing, drifted := 0, 0
		for _, id := range ids {
			base := rec.Baselines[id]
			c, ok := byID[id]
			if !ok {
				missing++
				w, note := baselineWording(store, rec, id, base)
				r.ChangedClaims = append(r.ChangedClaims, ChangedClaim{
					ID: id, Missing: true, BaselineHash: base, Baseline: w, BaselineNote: note,
				})
				e.Lint = append(e.Lint, lint.Finding{
					LintName: RuleRestsOnMissing, ClaimID: b.Path, Severity: severityOf(RuleRestsOnMissing),
					Message: fmt.Sprintf("rests_on names %q, which was a claim when this brief was approved and is not one now; the brief reads a claim that is gone. Rest the brief on the claims it now reads (unlock, edit rests_on, and the human re-locks), or restore the claim", id),
				})
				continue
			}
			now := lock.ContentHash(c)
			if now == base {
				continue
			}
			drifted++
			w, note := baselineWording(store, rec, id, base)
			r.ChangedClaims = append(r.ChangedClaims, ChangedClaim{
				ID: id, BaselineHash: base, CurrentHash: now, ChangedAt: changedAt(store, c),
				Baseline: w, BaselineNote: note, Current: wordingOf(c),
			})
			e.Lint = append(e.Lint, lint.Finding{
				LintName: RuleDependencyDrift, ClaimID: b.Path, Severity: severityOf(RuleDependencyDrift),
				Message: fmt.Sprintf("rests_on claim %q has changed since this brief's baseline, so the brief is review_pending: read the brief against the claim as it reads now (dossierx brief reaudit %s shows each change), and the human confirms with dossierx brief reaudit %s --confirm --reason \"<their words>\"", id, b.Path, b.Path),
			})
		}
		switch {
		case drifted > 0:
			r.ReviewPending, r.ReviewPendingTrigger = true, TriggerDependencyDrift
		case missing > 0:
			r.ReviewPending, r.ReviewPendingTrigger = true, TriggerRestsOnMissing
		}
		e.Reviews[b.ID] = r
	}
	sortFindings(e.Lint)
	sort.SliceStable(e.Integrity, func(i, j int) bool {
		if e.Integrity[i].ClaimID != e.Integrity[j].ClaimID {
			return e.Integrity[i].ClaimID < e.Integrity[j].ClaimID
		}
		return e.Integrity[i].Rule < e.Integrity[j].Rule
	})
	return e
}

func unrecordedFinding(b Brief, store *lock.Store, rec lock.BriefRecord, has bool) lock.Finding {
	why := "the lock store holds no approval record for it"
	switch {
	case store == nil:
		why = "the lock store could not be read, so there is no evidence it was approved (the unreadable store is reported above)"
	case has && rec.Released():
		why = fmt.Sprintf("its approval was released by an unlock on %s (%q), and status was set back to locked by hand", rec.ReleasedAt, rec.ReleasedReason)
	}
	return lock.Finding{
		Rule:    RuleUnrecorded,
		ClaimID: b.Path,
		Message: fmt.Sprintf("%s says status: locked, but %s. A status typed by hand approves nothing. If the human approves the brief as it reads, dossierx brief lock %s --reason \"<their words>\" records it; otherwise set status back to draft (or restore %s from version control).", b.Path, why, b.Path, config.LockStoreDisplayPath),
	}
}

// changedAt is when c's current content was approved: its standing ledger
// record's time when that record signs c as it reads now, and "" otherwise.
func changedAt(store *lock.Store, c model.Claim) string {
	if store == nil || c.Status != model.StatusLocked {
		return ""
	}
	rec, ok := store.Record(c.ID)
	if !ok || rec.Released() || rec.Hash != lock.LockedClaimHash(c) {
		return ""
	}
	return rec.At
}

// baselineWording is the wording a baseline hash stands for: the brief's own
// receipt when it hashes to the baseline, else any snapshot the store retains
// with that hash, else nil with the UnavailableWording note. It never reads
// git: the render path does not depend on a work tree (the reason
// `claim recover-approved-content` is a verb of its own), and every record
// `brief lock` and `brief reaudit --confirm` write carries its receipts, so the
// fallbacks serve only a record whose receipt was removed.
func baselineWording(store *lock.Store, rec lock.BriefRecord, id, hash string) (wording *Wording, note string) {
	if c, ok := rec.Receipts[id]; ok && lock.ContentHash(c) == hash {
		return wordingOf(c), ""
	}
	if c, ok := store.RetainedClaimContent(id, hash); ok {
		return wordingOf(c), ""
	}
	return nil, UnavailableWording
}

// Baselines returns the baseline hash and receipt of every claim b rests on
// that exists in claims — what `brief lock` and `brief reaudit --confirm`
// record — and the rests_on ids no claim carries (which refuse the lock).
func Baselines(b Brief, claims []model.Claim) (hashes map[string]string, receipts map[string]model.Claim, unknown []string) {
	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}
	hashes, receipts = map[string]string{}, map[string]model.Claim{}
	for _, id := range b.RestsOn {
		c, ok := byID[id]
		if !ok {
			unknown = append(unknown, id)
			continue
		}
		c.SourcePath = ""
		hashes[id] = lock.ContentHash(c)
		receipts[id] = c
	}
	return hashes, receipts, unknown
}
