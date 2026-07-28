package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// lockedWithRecord returns a locked claim and a store holding its (honest)
// ledger record — the clean baseline every rule below deviates from.
func lockedWithRecord(t *testing.T, c model.Claim) (model.Claim, *Store) {
	t.Helper()
	c.Status = model.StatusLocked
	store := newStore(t)
	RecordApproval(store, c, Approval{Actor: "alice", Reason: "approved"})
	return c, store
}

// TestAuditIsSilentOnAnHonestProject: a gate that fires on correct state is a
// gate people turn off. Every rule below must stay quiet here.
func TestAuditIsSilentOnAnHonestProject(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})
	draft := model.Claim{ID: "widget.contract.draft", Facet: "contract", Module: "widget", Status: model.StatusDraft}

	if findings := Audit([]model.Claim{locked, draft}, store, nil); len(findings) != 0 {
		t.Fatalf("expected no findings for an honest project, got %+v", findings)
	}
}

// TestAuditCatchesAHandFlippedStatus is the first row of the audit's table:
// editing "status: draft" to "status: locked" walked past the lint gate, hub
// gating and the unresolved-comment gate as though all three had passed, and
// nothing in the engine noticed.
func TestAuditCatchesAHandFlippedStatus(t *testing.T) {
	store := newStore(t)
	handFlipped := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "body"}

	findings := Audit([]model.Claim{handFlipped}, store, nil)
	if !hasRule(findings, RuleLockLedgerMissing) {
		t.Fatalf("expected %s, got %+v", RuleLockLedgerMissing, findings)
	}
}

// TestAuditCatchesEditedLockedContent covers the audit's second, third and
// fourth rows at once — an edited body, a swapped raw_html payload, and a
// flipped build_role/section/order/emphasis. ContentHash sees none of these
// unless a dependent happens to exist; LockedClaimHash sees all of them.
func TestAuditCatchesEditedLockedContent(t *testing.T) {
	tamper := map[string]func(*model.Claim){
		"body edited":       func(c *model.Claim) { c.Body = "quietly rewritten" },
		"raw_html swapped":  func(c *model.Claim) { c.RawHTML = `<img src=x onerror=alert(1)>` },
		"raw_html reviewed": func(c *model.Claim) { c.RawHTMLReviewed = false },
		"build_role":        func(c *model.Claim) { c.BuildRole = model.BuildRoleOutOfScope },
		"section":           func(c *model.Claim) { c.Section = "moved elsewhere" },
		"order":             func(c *model.Claim) { c.Order = 42 },
		"emphasis":          func(c *model.Claim) { c.Emphasis = true },
	}

	for name, mutate := range tamper {
		t.Run(name, func(t *testing.T) {
			approved := model.Claim{
				ID: "widget.contract.mockup", Facet: "contract", Module: "widget",
				Layout: model.LayoutMockup, Body: "approved body",
				RawHTML: `<div>approved markup</div>`, RawHTMLReviewed: true,
				BuildRole: model.BuildRoleSchema, Section: "1 - orientation", Order: 1,
			}
			locked, store := lockedWithRecord(t, approved)

			mutate(&locked)
			findings := Audit([]model.Claim{locked}, store, nil)
			if !hasRule(findings, RuleLockContentDrift) {
				t.Fatalf("expected %s after %s, got %+v", RuleLockContentDrift, name, findings)
			}
		})
	}
}

// TestAuditIgnoresTheThreeEngineManagedFields is the deny-list, end to end. A
// comment op, a review_pending reconcile, and (on its own) a status field must
// never read as tampering — otherwise the gate would fire on every routine run
// of "dossierx check" and be worthless within a day.
func TestAuditIgnoresTheThreeEngineManagedFields(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})

	locked.ReviewPending = true
	locked.Comments = append(locked.Comments, model.Comment{
		ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
		Created: "2026-07-27T10:00:00Z", Body: "is this still true?",
	})

	if findings := Audit([]model.Claim{locked}, store, nil); len(findings) != 0 {
		t.Fatalf("a comment and a review_pending flip are ordinary engine writes, not tampering; got %+v", findings)
	}
}

// TestAuditCatchesTheOrphanedLockRecord is the audit's last row: flipping
// locked -> draft by hand to dodge review. A draft claim is edited freely and
// can be re-locked later, and before the ledger there was nothing at all to
// notice it had ever been locked. Releasing on a REAL unlock is what keeps this
// rule from firing on honest work.
func TestAuditCatchesTheOrphanedLockRecord(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "body"})

	handFlipped := locked
	handFlipped.Status = model.StatusDraft
	if !hasRule(Audit([]model.Claim{handFlipped}, store, nil), RuleLockLedgerOrphan) {
		t.Fatalf("expected %s for a draft claim still holding an active record", RuleLockLedgerOrphan)
	}

	// The same claim, unlocked properly, is silent.
	properly := Unlock(locked, store, Approval{Actor: "alice", Reason: "needs a fix"})
	if findings := Audit([]model.Claim{properly}, store, nil); len(findings) != 0 {
		t.Fatalf("an honest unlock must not trip the orphan rule, got %+v", findings)
	}
}

// TestAuditCatchesTheRelockedReleasedRecord is the orphan rule's mirror image,
// and the cheapest complete bypass of the approval path that existed before it:
//
//	dossierx claim lock <id>    -> record written
//	dossierx claim unlock <id>  -> record RELEASED (kept, stamped ReleasedAt),
//	                               status: draft
//	edit the YAML: status back to locked
//
// Nothing fired. lock-ledger-missing did not, because a record is present;
// lock-content-drift did not, because LockedClaimHash excludes status and the
// body is whatever it was; lock-ledger-orphan did not, because that rule is
// about a DRAFT claim holding an UNreleased record. The claim ends up locked
// with its approval withdrawn, which is the exact state the ledger exists to
// make impossible.
//
// The body is left untouched on purpose: the point is that content equality is
// not evidence of approval, so the rule must fire with the hash still matching.
func TestAuditCatchesTheRelockedReleasedRecord(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})
	released := Unlock(locked, store, Approval{Actor: "alice", Reason: "reopening for a fix"})

	// The hand edit: status: draft -> status: locked, nothing else changed.
	relocked := released
	relocked.Status = model.StatusLocked

	findings := Audit([]model.Claim{relocked}, store, nil)
	if !hasRule(findings, RuleLockLedgerReleased) {
		t.Fatalf("expected %s for a locked claim whose only record was released; got %+v", RuleLockLedgerReleased, findings)
	}
	// It must not ALSO be reported as drift: the content really does still
	// match what was approved, and a false drift finding sends the reader
	// hunting for an edit that never happened.
	if hasRule(findings, RuleLockContentDrift) {
		t.Fatalf("the content still hashes to what was approved; %s is a false report here: %+v", RuleLockContentDrift, findings)
	}

	// And the approval record a genuine re-lock writes clears it, so the rule
	// cannot fire on honest work — RecordApproval is the write Lock performs
	// once its three gates pass, which is exactly what a released record is
	// missing.
	RecordApproval(store, relocked, Approval{Actor: "alice", Reason: "re-approved"})
	if findings := Audit([]model.Claim{relocked}, store, nil); len(findings) != 0 {
		t.Fatalf("a properly re-locked claim must be silent, got %+v", findings)
	}
}

// TestAuditCatchesHandEditedComments: deleting an unresolved thread straight out
// of the YAML is how a claim gets past the lock gate with a review still open.
func TestAuditCatchesHandEditedComments(t *testing.T) {
	claim := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Comments: []model.Comment{{
			ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman,
			Created: "2026-07-27T10:00:00Z", Body: "this is wrong, please fix",
		}},
	}
	digests, err := digest.LoadStore(filepath.Join(t.TempDir(), "digest.json"))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	digests.Record(claim)

	if findings := Audit([]model.Claim{claim}, nil, digests); len(findings) != 0 {
		t.Fatalf("expected silence while the comment block matches its digest, got %+v", findings)
	}

	// The open thread is deleted by hand.
	tampered := claim
	tampered.Comments = nil
	if !hasRule(Audit([]model.Claim{tampered}, nil, digests), RuleCommentLedgerDrift) {
		t.Fatalf("expected %s after an unresolved thread was deleted from the YAML", RuleCommentLedgerDrift)
	}

	// So is a thread ADDED by hand to a claim whose comments were legitimately
	// emptied — which is why the digest records the empty state rather than
	// dropping the entry.
	emptied := claim
	emptied.Comments = nil
	digests.Record(emptied)
	forged := claim
	forged.Comments = []model.Comment{{ID: "c-forged", Status: model.CommentStatusResolved, Author: model.CommentRoleAgent, Created: "2026-07-27T11:00:00Z", Body: "looks fine to me"}}
	if !hasRule(Audit([]model.Claim{forged}, nil, digests), RuleCommentLedgerDrift) {
		t.Fatalf("expected %s after a thread was hand-added to a claim with an empty recorded digest", RuleCommentLedgerDrift)
	}
}

// TestAuditTreatsUnknownAsUnknown: a claim the digest store has never seen
// reports nothing. An integrity check must never manufacture a finding out of
// missing evidence — the same distinction Store.Baseline draws for dependency
// hashes.
func TestAuditTreatsUnknownAsUnknown(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Comments: []model.Comment{{ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-27T10:00:00Z", Body: "hello"}}}
	digests, err := digest.LoadStore(filepath.Join(t.TempDir(), "digest.json"))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}

	if findings := Audit([]model.Claim{claim}, nil, digests); len(findings) != 0 {
		t.Fatalf("a claim with no recorded digest is uncovered, not drifted; got %+v", findings)
	}
	if findings := Audit([]model.Claim{claim}, nil, nil); len(findings) != 0 {
		t.Fatalf("a nil digest store must disable the comment rules, not fire them; got %+v", findings)
	}
}

// preLedgerStore writes a v0.2.x-shaped lock store — on disk, at the schema that
// predates the ledger, with no ledger key at all — and loads it. It is the state
// every existing project is in on the day it upgrades.
func preLedgerStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"hashes":{},"locked_at":{"widget.contract.main":"2026-01-01T00:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write pre-ledger store: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	return store
}

// TestAuditRefusesAnUnmigratedProjectOnceByName is the read-only half of
// ADOPTION FAILS CLOSED, and it replaces the in-memory grandfathering that used
// to make this state pass silently.
//
// Two things have to be true at once, and only one of them used to be. The gate
// must NOT accuse the project claim by claim — lock-ledger-missing's recovery
// says to set the claim back to draft and lock it again, which is destructive
// advice for a project whose only fault is being a version behind, and it is what
// the old exemption was written to avoid. And it must NOT pass either, which is
// what the old exemption did: a project that presents the pre-ledger shape (two
// hand edits) was grandfathered in memory by every read-only command, so `check
// --validate`, the pre-commit hook and CI all reported a clean project.
//
// So: exactly one finding, project-scoped, naming the one-time command.
func TestAuditRefusesAnUnmigratedProjectOnceByName(t *testing.T) {
	store := preLedgerStore(t)
	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "locked long before the ledger existed"}
	draft := model.Claim{ID: "widget.contract.draft", Facet: "contract", Module: "widget", Status: model.StatusDraft}

	// No digest store: the file did not exist before this release, so an honest
	// pre-ledger project has none.
	findings := Audit([]model.Claim{locked, draft}, store, nil)
	if len(findings) != 1 || findings[0].Rule != RuleLockLedgerAdoptionRequired {
		t.Fatalf("an un-migrated project must fail with exactly one project-scoped finding, got %+v", findings)
	}
	if findings[0].ClaimID != "" {
		t.Errorf("the adoption refusal is about the PROJECT, not a claim; got claim_id %q", findings[0].ClaimID)
	}
	if !strings.Contains(findings[0].Message, "dossierx migrate --adopt") {
		t.Errorf("the refusal must name the one command that clears it, got %q", findings[0].Message)
	}
	if hasRule(findings, RuleLockLedgerMissing) {
		t.Errorf("the per-claim accusation must NOT accompany it: its recovery would destroy the approvals the migration is about to record")
	}
	// The read-only gate stays read-only: it must not write a record, or it
	// would be silently doing the migration's job — the very thing that made
	// implicit adoption a laundering path.
	if _, ok := store.Record(locked.ID); ok {
		t.Fatalf("the read-only gate must not record anything")
	}
}

// The exemption is spent only on the one thing a pre-ledger build could not have
// written. Everything else the gate knows how to say, it still says.
func TestPreLedgerExemptionDoesNotSuppressTheOtherRules(t *testing.T) {
	store := preLedgerStore(t)
	digests, err := digest.LoadStore(filepath.Join(t.TempDir(), "digest.json"))
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked,
		Comments: []model.Comment{{ID: "c-abc123", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-27T10:00:00Z", Body: "is this still true?"}}}

	// A recorded digest that no longer matches is comment drift whatever the lock
	// store's schema version is: comment history is not what grandfathering is
	// about.
	digests.Record(claim)
	edited := claim
	edited.Comments = nil

	if !hasRule(Audit([]model.Claim{edited}, store, digests), RuleCommentLedgerDrift) {
		t.Fatalf("expected %s: the pre-ledger exemption covers missing lock records, nothing else", RuleCommentLedgerDrift)
	}
}

// TestAuditDoesNotReadABuildOrderRecordAsAClaim: the ledger holds two subject
// kinds in one map. If the claim rules matched on the key alone, a build-order
// record would have to be parsed out by shape — and a subject added later would
// silently start being audited as a claim.
func TestAuditDoesNotReadABuildOrderRecordAsAClaim(t *testing.T) {
	store := newStore(t)
	RecordBuildOrderApproval(store, "widget", "artifact-signature", Approval{Actor: "alice", Reason: "order approved"})

	// A claim whose id collides with the build-order key would still be
	// reported: the record it finds is not a claim record.
	collide := model.Claim{ID: BuildOrderLedgerKey("widget"), Facet: "contract", Module: "widget", Status: model.StatusLocked}
	if !hasRule(Audit([]model.Claim{collide}, store, nil), RuleLockLedgerMissing) {
		t.Fatalf("a build-order record must never satisfy a claim's approval requirement")
	}
}

// TestAuditOrderIsDeterministic: the hook's output and a CI log get diffed
// against each other, so the same state must always print the same way.
func TestAuditOrderIsDeterministic(t *testing.T) {
	store := newStore(t)
	claims := []model.Claim{
		{ID: "widget.contract.c", Facet: "contract", Module: "widget", Status: model.StatusLocked},
		{ID: "widget.contract.a", Facet: "contract", Module: "widget", Status: model.StatusLocked},
		{ID: "widget.contract.b", Facet: "contract", Module: "widget", Status: model.StatusLocked},
	}

	first := Audit(claims, store, nil)
	for i := 0; i < 20; i++ {
		again := Audit(claims, store, nil)
		if len(again) != len(first) {
			t.Fatalf("finding count varies between runs")
		}
		for j := range again {
			if again[j] != first[j] {
				t.Fatalf("finding %d varies between runs: %+v vs %+v", j, again[j], first[j])
			}
		}
	}
	// Project-scoped findings sort first (empty claim id), then by claim id.
	if first[0].Rule != RuleLockLedgerAbsent {
		t.Fatalf("expected the project-scoped finding first, got %+v", first[0])
	}
	if first[1].ClaimID != "widget.contract.a" || first[3].ClaimID != "widget.contract.c" {
		t.Fatalf("expected per-claim findings sorted by id, got %+v", first)
	}
}

// TestAuditCatchesTheDeletedLockedClaim is the reverse sweep, and the hole every
// other rule in this file shares by construction: they all iterate the CLAIMS
// and look for a record that disagrees. A claim whose file was DELETED has no
// entry to iterate, so `rm claims/whatever.yaml` on a locked claim produced a
// completely silent gate — while being the most destructive edit available to
// anyone holding the repository. Reviewed content simply vanished, and the
// ledger went on carrying a standing approval for a claim that was not there.
func TestAuditCatchesTheDeletedLockedClaim(t *testing.T) {
	locked, store := savedStoreWithRecord(t, model.Claim{ID: "widget.contract.gone", Facet: "contract", Module: "widget", Body: "approved body"})
	survivor, _ := lockedWithRecord(t, model.Claim{ID: "widget.contract.here", Facet: "contract", Module: "widget", Body: "still here"})
	RecordApproval(store, survivor, Approval{Actor: "alice", Reason: "approved"})

	// The claim set the loader would now produce: everything EXCEPT the deleted
	// one. Its record is still standing in the ledger.
	findings := Audit([]model.Claim{survivor}, store, nil)
	if !hasRule(findings, RuleLockLedgerAbandoned) {
		t.Fatalf("expected %s for a locked claim whose file was deleted; got %+v", RuleLockLedgerAbandoned, findings)
	}
	for _, f := range findings {
		if f.Rule == RuleLockLedgerAbandoned && f.ClaimID != locked.ID {
			t.Fatalf("the finding must name the deleted claim, got %q", f.ClaimID)
		}
	}

	// The HONEST path stays silent. Unlock stamps ReleasedAt and keeps the
	// record, so unlock-then-delete is a human deciding on the record that the
	// claim should go — a rule that fired there would make the documented
	// recovery itself a finding.
	Unlock(locked, store, Approval{Actor: "alice", Reason: "agreed to drop it"})
	if findings := Audit([]model.Claim{survivor}, store, nil); hasRule(findings, RuleLockLedgerAbandoned) {
		t.Fatalf("unlock-then-delete must be silent, got %+v", findings)
	}
}

// savedStoreWithRecord is lockedWithRecord with the store actually PERSISTED, so
// Store.FileExists() reports true. The abandoned rule is scoped to a store whose
// file is real: an absent ledger is already reported once as
// lock-ledger-absent, and firing there as well would accuse every claim of
// having been deleted whenever the ledger is the thing that is missing.
func savedStoreWithRecord(t *testing.T, c model.Claim) (model.Claim, *Store) {
	t.Helper()
	c.Status = model.StatusLocked

	path := filepath.Join(t.TempDir(), "store.json")
	seed, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	RecordApproval(seed, c, Approval{Actor: "alice", Reason: "approved"})
	if err := seed.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reopen): %v", err)
	}
	return c, store
}

// TestLockRefusesAnAlreadyLockedClaim closes the cheapest laundering path the
// ledger had: one ordinary command, no unlock, no hand edit of the ledger.
//
//	edit a LOCKED claim's body          -> check reports lock-content-drift
//	dossierx claim lock <id> --reason … -> RecordApproval overwrites the record
//	                                       with a hash of the EDITED content
//
// The finding disappears and the ledger now attests that a human approved bytes
// they never saw. The dry run had advertised "claim_is_draft" as a precondition
// all along; only the real run failed to enforce it.
func TestLockRefusesAnAlreadyLockedClaim(t *testing.T) {
	locked, store := lockedWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})
	approvedHash := LockedClaimHash(locked)

	tampered := locked
	tampered.Body = "quietly rewritten"

	if _, err := Lock(tampered, []model.Claim{tampered}, nil, store, Approval{Actor: "mallory", Reason: "re-approved"}); !errors.Is(err, ErrAlreadyLocked) {
		t.Fatalf("expected ErrAlreadyLocked when re-locking a locked claim, got %v", err)
	}

	// The record must be UNTOUCHED — a refusal that still wrote would be no
	// refusal at all.
	record, ok := store.Record(locked.ID)
	if !ok || record.Hash != approvedHash {
		t.Fatalf("the ledger record was rewritten by a refused lock: %+v", record)
	}
	if record.Reason != "approved" || record.Actor != "alice" {
		t.Fatalf("the refused lock overwrote the approval's provenance: %+v", record)
	}
	// And the drift is still reported, which is the whole point.
	if findings := Audit([]model.Claim{tampered}, store, nil); !hasRule(findings, RuleLockContentDrift) {
		t.Fatalf("the tamper must still be reported after a refused re-lock, got %+v", findings)
	}

	// The documented recovery must not be blocked BY THIS GATE: once unlock has
	// released the record and set the claim back to draft, the already-locked
	// refusal is gone. (Whatever the lint suite then says about this bare
	// fixture claim is a different gate's business and is covered elsewhere.)
	released := Unlock(tampered, store, Approval{Actor: "alice", Reason: "reopening"})
	if _, err := Lock(released, []model.Claim{released}, nil, store, Approval{Actor: "alice", Reason: "re-approved properly"}); errors.Is(err, ErrAlreadyLocked) {
		t.Fatalf("unlock must clear the already-locked refusal, got %v", err)
	}
}

// TestLockRefusesADraftClaimHoldingAStandingRecord closes the ONE-LINE bypass of
// the refusal above. The already-locked guard was STATUS-based, and status is a
// line in the audited file:
//
//	lock a claim                        -> record written, hash of the approved body
//	edit its body by hand               -> check reports lock-content-drift
//	dossierx claim lock <id>            -> correctly refused (already_locked)
//	edit "status: locked" -> "draft"    -> the guard no longer sees a locked claim
//	dossierx claim lock <id> --reason … -> SUCCEEDS, and RecordApproval replaces
//	                                       the record's hash with a hash of the
//	                                       TAMPERED bytes
//
// After that run the gate is green forever, no unlock ever happened, and there
// is no released_at/released_by anywhere to say the approval was withdrawn — the
// exact evidence the ledger exists to keep. The correct predicate is not what
// the file's status line says (the attacker writes that) but whether a STANDING,
// unreleased record still vouches for this claim: a draft claim holding one IS
// lock-ledger-orphan by definition, and re-locking it is a re-signing, not the
// draft -> locked transition Lock implements.
func TestLockRefusesADraftClaimHoldingAStandingRecord(t *testing.T) {
	withRegistry(t) // empty registry: the refusal under test must be the ledger's
	locked, store := savedStoreWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})
	approvedHash := LockedClaimHash(locked)

	// The whole attack, in one hand edit of the claim file: the body rewritten
	// and the status line flipped back to draft.
	flipped := locked
	flipped.Body = "quietly rewritten"
	flipped.Status = model.StatusDraft

	// The state is already a finding — this is the claim the gate calls an
	// orphan — and the re-lock must not be the thing that clears it.
	if findings := Audit([]model.Claim{flipped}, store, nil); !hasRule(findings, RuleLockLedgerOrphan) {
		t.Fatalf("fixture is not the attack state: expected %s, got %+v", RuleLockLedgerOrphan, findings)
	}

	if _, err := Lock(flipped, []model.Claim{flipped}, testConfig(), store, Approval{Actor: "mallory", Reason: "re-approved"}); !errors.Is(err, ErrAlreadyLocked) {
		t.Fatalf("expected the standing record to refuse the lock, got %v", err)
	}

	record, ok := store.Record(locked.ID)
	if !ok || record.Hash != approvedHash {
		t.Fatalf("the refused lock rewrote the approval with the tampered content: %+v", record)
	}
	if record.Reason != "approved" || record.Actor != "alice" || record.Released() {
		t.Fatalf("the refused lock rewrote the approval's provenance: %+v", record)
	}
	if findings := Audit([]model.Claim{flipped}, store, nil); !hasRule(findings, RuleLockLedgerOrphan) {
		t.Fatalf("the orphan must still be reported after a refused re-lock, got %+v", findings)
	}

	// And the documented recovery is still open: unlock RELEASES the record on
	// the record, after which the same claim locks normally.
	released := Unlock(flipped, store, Approval{Actor: "alice", Reason: "reopening deliberately"})
	if _, err := Lock(released, []model.Claim{released}, testConfig(), store, Approval{Actor: "alice", Reason: "re-approved properly"}); err != nil {
		t.Fatalf("unlock must clear the standing-record refusal, got %v", err)
	}
}

// A RELEASED record must not refuse anything: release is what unlock records,
// and a claim whose approval was withdrawn is an ordinary draft that locks the
// ordinary way. A guard that refused here would break the one recovery every
// other refusal in this package points at.
func TestLockAllowsADraftClaimWhoseRecordWasReleased(t *testing.T) {
	withRegistry(t) // empty registry: lint is a different gate, tested elsewhere
	locked, store := savedStoreWithRecord(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "approved body"})

	draft := Unlock(locked, store, Approval{Actor: "alice", Reason: "reopening"})
	draft.Body = "the edit the unlock was for"

	relocked, err := Lock(draft, []model.Claim{draft}, testConfig(), store, Approval{Actor: "alice", Reason: "approved the edit"})
	if err != nil {
		t.Fatalf("a released record must not refuse a lock: %v", err)
	}
	if relocked.Status != model.StatusLocked {
		t.Fatalf("expected the claim locked, got %q", relocked.Status)
	}
	if record, _ := store.Record(draft.ID); record.Hash != LockedClaimHash(relocked) || record.Released() {
		t.Fatalf("the honest re-lock must record a new standing approval: %+v", record)
	}
}

// lockedProjectOnDisk locks c through the real approval path into a real store
// file and hands back the RELOADED store — a ledger-covered project holding one
// honest approval, with everything a real lock leaves behind: the record, the
// locked_at stamp, and the dependency baselines.
//
// Reloading matters: the deleted-record rule is armed by Store.LedgerCovered,
// which reads the version the file was DECODED from, so a fixture that never
// went through disk would test a state no project is ever in.
func lockedProjectOnDisk(t *testing.T, c model.Claim) (model.Claim, *Store) {
	t.Helper()
	withRegistry(t)

	path := filepath.Join(t.TempDir(), "store.json")
	seed, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	locked, err := Lock(c, []model.Claim{c}, testConfig(), seed, Approval{Actor: "alice", Reason: "approved"})
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := seed.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reopen): %v", err)
	}
	if _, stamped := store.LockedAt[locked.ID]; !stamped {
		t.Fatalf("fixture precondition: a real lock must leave a locked_at stamp")
	}
	return locked, store
}

// TestADeletedLedgerRecordIsReported is the round-6 critical, reproduced here
// exactly as it was reproduced against the shipped binary: delete ONE claim's
// entry from the "ledger" map and flip its `status: locked` back to `status:
// draft`, and every rule in this file went quiet.
//
//	lock-ledger-missing   needs status: locked        -> not it, the flip saw to that
//	lock-ledger-orphan    needs a record to exist     -> not it, the delete saw to that
//	lock-content-drift    needs a record to compare   -> not it, same
//	lock-ledger-absent    needs the whole file gone   -> not it, one key was removed
//
// `check --validate` reported ok:true with ZERO findings, the claim was then
// freely editable as an ordinary draft, and re-locking it later wrote a fresh
// record with an agent-supplied reason indistinguishable from a human's.
//
// The evidence the delete does not reach is one key away in the same file:
// locked_at, which unlock KEEPS (it releases the record instead of deleting it),
// so a claim the engine locked and the ledger says nothing about had its record
// removed by hand.
func TestADeletedLedgerRecordIsReported(t *testing.T) {
	locked, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

	// THE ATTACK, both halves.
	delete(store.Ledger, locked.ID)
	tampered := locked
	tampered.Status = model.StatusDraft
	tampered.Body = "rewritten now that nothing vouches for it"

	findings := Audit([]model.Claim{tampered}, store, nil)
	if !hasRule(findings, RuleLockLedgerDeleted) {
		t.Fatalf("expected %s: a claim this engine locked, whose record is gone, must not be silent; got %+v", RuleLockLedgerDeleted, findings)
	}
	for _, f := range findings {
		if f.Rule == RuleLockLedgerDeleted && !strings.Contains(f.Message, "Restore the lock store") {
			t.Errorf("the recovery must be a RESTORE, never a re-lock (which records whatever the claim says now): %q", f.Message)
		}
	}
}

// The same deletion with the status left alone reports the deletion too, and
// reports it INSTEAD OF lock-ledger-missing: the two need different recoveries,
// and missing's ("set it back to draft and lock it properly") would tell a human
// to re-approve content whose approved bytes are sitting in version control.
func TestADeletedRecordIsDistinctFromANeverRecordedOne(t *testing.T) {
	locked, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})
	delete(store.Ledger, locked.ID)

	findings := Audit([]model.Claim{locked}, store, nil)
	if !hasRule(findings, RuleLockLedgerDeleted) {
		t.Fatalf("expected %s, got %+v", RuleLockLedgerDeleted, findings)
	}
	if hasRule(findings, RuleLockLedgerMissing) {
		t.Fatalf("a DELETED record must not also be reported as never-recorded: one state, one name, one recovery; got %+v", findings)
	}

	// And a claim the engine never locked — hand-flipped to locked in a covered
	// project, no locked_at, no baselines — is still the other rule.
	handFlipped := model.Claim{ID: "widget.contract.other", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "never locked by anything"}
	findings = Audit([]model.Claim{handFlipped}, store, nil)
	if !hasRule(findings, RuleLockLedgerMissing) || hasRule(findings, RuleLockLedgerDeleted) {
		t.Fatalf("a claim with no locked_at was never locked by this engine: expected %s alone, got %+v", RuleLockLedgerMissing, findings)
	}
}

// The honest side, which is what keeps the rule off correct state: unlock ->
// fix -> lock leaves a locked_at stamp on a DRAFT claim for as long as it is
// being fixed, and that must stay silent. Unlock releases the record rather than
// deleting it, and a released record is exactly the evidence that says so.
func TestAnHonestlyUnlockedClaimIsNotReportedAsDeleted(t *testing.T) {
	locked, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

	drafted := Unlock(locked, store, Approval{Actor: "alice", Reason: "needs a fix"})
	drafted.Body = "being fixed right now"

	if findings := Audit([]model.Claim{drafted}, store, nil); len(findings) != 0 {
		t.Fatalf("the sanctioned unlock -> fix -> lock window must be silent, got %+v", findings)
	}
}

// And the rule is armed by COVERAGE, not by the stamp alone: an un-migrated
// project's locked_at stamps are not evidence of a deleted record, because that
// project never had records to delete. It gets the adoption refusal instead —
// one finding, with the command that clears it.
func TestTheDeletedRecordRuleIsSilentOnAnUnmigratedProject(t *testing.T) {
	store := preLedgerStore(t) // its locked_at names widget.contract.main
	locked := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "locked by v0.2.x"}

	findings := Audit([]model.Claim{locked}, store, nil)
	if hasRule(findings, RuleLockLedgerDeleted) {
		t.Fatalf("a pre-ledger project's locked_at is not a deleted record: it never had one; got %+v", findings)
	}
	if !hasRule(findings, RuleLockLedgerAdoptionRequired) {
		t.Fatalf("expected %s, got %+v", RuleLockLedgerAdoptionRequired, findings)
	}
}
