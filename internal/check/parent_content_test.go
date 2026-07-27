// parent_content_test.go covers the PER-CLAIM half of the parent comparison:
// internal/check wiring lock.AuditAgainstParent into the staged gate.
//
// history_test.go and parent_scope_test.go both cover questions about SCOPE —
// what the gate is able to see. This file covers the question about EVIDENCE:
// an approval the parent commit recorded that this commit no longer carries,
// where the claim it approved is gone too.
//
// That shape is invisible to every rule that reads one directory, and
// deliberately so in both directions: lock.Audit's forward rules start from a
// claim that this commit does not have, and its reverse sweep starts from a
// record that this commit does not have. Deleting the two together is what makes
// each of them silent about the other, and the only place both still exist is
// the parent commit. droppedApprovals does not reach it either — it reports a
// dropped approval only for a claim the index STILL HOLDS A FILE FOR, which is
// exactly the clause that keeps an honest retirement quiet.
//
// The honest counterpart is on the record beside it and has to stay silent: an
// unlock RELEASES a record rather than deleting it, so the sanctioned
// "unlock, then delete the claim" retirement leaves a released record behind and
// is not this rule's business.
package check_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// twoLockedFixture is a committed project with TWO locked claims, both on the
// ledger.
//
// Two rather than one is load-bearing: the tamper below removes one claim's
// record, and a store left holding NOTHING would be reported by the whole-file
// and emptied-ledger rules instead, which is a different finding with a
// different recovery. Keeping a second record means the store under judgement is
// still a populated, well-formed, current-version ledger — so the only thing
// that changed is the one approval, which is precisely the state this rule has
// to be able to name on its own.
func twoLockedFixture(t *testing.T) *config.Config {
	t.Helper()
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml": lockedClaim("widget.contract.one"),
		"claims/two.yaml": lockedClaim("widget.contract.two"),
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")
	return cfg
}

// dropRecord removes every trace that this engine ever locked id — the ledger
// record, the locked_at stamp and the claim's own dependency baselines — and
// saves the store. It is the three-key edit lock.RuleLockLedgerDeleted names,
// applied to the store on disk.
func dropRecord(t *testing.T, cfg *config.Config, id string) {
	t.Helper()
	path := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(path)
	if err != nil {
		t.Fatalf("drop record: load store: %v", err)
	}
	delete(store.Ledger, id)
	delete(store.LockedAt, id)
	delete(store.Hashes, id)
	if err := store.Save(); err != nil {
		t.Fatalf("drop record: save store: %v", err)
	}
}

// releaseRecord is what `dossierx claim unlock` does to the ledger: the record
// STAYS and is stamped released. It is the honest counterpart of dropRecord.
func releaseRecord(t *testing.T, cfg *config.Config, id string) {
	t.Helper()
	path := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(path)
	if err != nil {
		t.Fatalf("release record: load store: %v", err)
	}
	lock.ReleaseApproval(store, id, lock.Approval{Actor: "fixture", Reason: "retiring the claim"})
	if err := store.Save(); err != nil {
		t.Fatalf("release record: save store: %v", err)
	}
}

// TestStaged_RefusesAClaimDeletedWithItsApproval is the wiring's reason to
// exist, and it was verified to FAIL before lock.AuditAgainstParent had a caller:
// the same tree reported zero findings and exited 0.
//
// A locked claim and the ledger record approving it, removed in ONE commit. Each
// deletion on its own is reported — the claim alone leaves a standing record
// that the reverse sweep calls abandoned, and the record alone leaves a locked
// claim that the forward rules call unapproved — and together they are silent,
// because each rule's starting point is what the other deletion removed.
func TestStaged_RefusesAClaimDeletedWithItsApproval(t *testing.T) {
	cfg := twoLockedFixture(t)

	// Control: the fixture is clean, and clean because it judged something.
	if rules, _ := stagedRules(t, cfg); len(rules) != 0 {
		t.Fatalf("fixture precondition: expected a clean gate, got %v", rules)
	}

	git(t, cfg.Dir(), "rm", "-q", "claims/one.yaml")
	dropRecord(t, cfg, "widget.contract.one")
	git(t, cfg.Dir(), "add", "-A")

	rules, res := stagedRules(t, cfg)
	if !contains(rules, lock.RuleLockLedgerAbandoned) {
		t.Fatalf("deleting a locked claim together with its ledger record must be refused: got %v", rules)
	}

	// The refusal has to name the claim and tell the reader what to do, or it
	// is a rule name with no recovery attached.
	var msg string
	for _, f := range res.LedgerFindings {
		if f.Rule == lock.RuleLockLedgerAbandoned {
			msg = f.Message
		}
	}
	if !strings.Contains(msg, "widget.contract.one") {
		t.Fatalf("the refusal must name the claim whose approval was erased: %q", msg)
	}
	if !strings.Contains(msg, "unlock") {
		t.Fatalf("the refusal must name the sanctioned retirement path (unlock): %q", msg)
	}
}

// TestStaged_SilentWhenAReleasedClaimIsDeleted is the honest counterpart, and it
// is the half that decides whether the rule above can ship.
//
// Retiring a claim is ordinary work. The sanctioned way to do it is the way the
// refusal above tells people to: unlock it, which RELEASES the ledger record and
// keeps it as the record of the withdrawal, then delete the file. That leaves
// the parent holding a record this commit does not — the literal trigger — and
// it must not fire, because the release is the human's decision on the record.
func TestStaged_SilentWhenAReleasedClaimIsDeleted(t *testing.T) {
	cfg := twoLockedFixture(t)

	releaseRecord(t, cfg, "widget.contract.one")
	git(t, cfg.Dir(), "rm", "-q", "claims/one.yaml")
	git(t, cfg.Dir(), "add", "-A")

	if rules, _ := stagedRules(t, cfg); len(rules) != 0 {
		t.Fatalf("unlock-then-delete is the sanctioned retirement and must be silent: got %v", rules)
	}
}

// TestStaged_SilentOnAnHonestClaimDeletion covers the other retirement a project
// actually performs: deleting a DRAFT claim, which never had a record at all.
// Nothing about it involves the parent's ledger, and a gate that refused it
// would make draft claims unremovable.
func TestStaged_SilentOnAnHonestClaimDeletion(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml":   lockedClaim("widget.contract.one"),
		"claims/draft.yaml": draftClaim("widget.contract.draft"),
	})
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")

	git(t, cfg.Dir(), "rm", "-q", "claims/draft.yaml")
	git(t, cfg.Dir(), "add", "-A")

	if rules, _ := stagedRules(t, cfg); len(rules) != 0 {
		t.Fatalf("deleting a draft claim must be silent: got %v", rules)
	}
}
