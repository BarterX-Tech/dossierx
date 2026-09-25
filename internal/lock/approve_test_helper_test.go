package lock

import (
	"fmt"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// approve is this package's stand-in for a one-claim `dossierx claim lock`. It
// replays cmd/dossierx's runPolicySetLock step for step, minus the claim-file
// writes and the store save, which stay with the caller:
//
//  1. CrossPreLedger — a pre-ledger project still holding locked claims is
//     refused with ErrPreLedgerUnadopted before anything is evaluated;
//  2. EvaluateSetWithSemanticConflicts on the requested id — any refusal is
//     returned, naming every refusal, and nothing is written;
//  3. the requested claim flips to locked with review_pending cleared in a copy
//     of the claim set, and RefreshBaseline then RecordApproval run against it.
//
// claim is the requested claim as it is on disk; it replaces the entry with
// the same id in claims (or joins them), because the CLI evaluates the loaded
// set and the requested claim is always a member of it.
func approve(claim model.Claim, claims []model.Claim, cfg *config.Config, store *Store, ap Approval) (model.Claim, error) {
	loaded := make([]model.Claim, 0, len(claims)+1)
	present := false
	for _, c := range claims {
		if c.ID == claim.ID {
			c = claim
			present = true
		}
		loaded = append(loaded, c)
	}
	if !present {
		loaded = append(loaded, claim)
	}

	if err := CrossPreLedger(store, loaded); err != nil {
		return claim, fmt.Errorf("approve %s: %w", claim.ID, err)
	}
	evaluation := EvaluateSetWithSemanticConflicts(loaded, []string{claim.ID}, cfg, store, nil)
	if !evaluation.Allowed() {
		for _, verdict := range evaluation.Verdicts {
			if err := verdict.Error(); err != nil {
				return claim, err
			}
		}
		return claim, fmt.Errorf("approve %s: refused", claim.ID)
	}

	final := make([]model.Claim, len(loaded))
	copy(final, loaded)
	var approved model.Claim
	for i := range final {
		if final[i].ID == claim.ID {
			final[i].Status = model.StatusLocked
			final[i].ReviewPending = false
			approved = final[i]
		}
	}
	RefreshBaseline(approved, final, store)
	RecordApproval(store, approved, ap)
	return approved, nil
}

// verdictFor evaluates a one-claim lock of id exactly as the write path does.
func verdictFor(t *testing.T, claims []model.Claim, id string, cfg *config.Config, store *Store) CandidateVerdict {
	t.Helper()
	evaluation := EvaluateSet(claims, []string{id}, cfg, store)
	if len(evaluation.Verdicts) != 1 {
		t.Fatalf("EvaluateSet(%s) returned %d verdicts, want 1: %+v", id, len(evaluation.Verdicts), evaluation)
	}
	verdict := evaluation.Verdicts[0]
	if verdict.LocalAdmissible != (len(verdict.Refusals) == 0) {
		t.Fatalf("verdict for %s is inconsistent: admissible=%v refusals=%v", id, verdict.LocalAdmissible, verdict.Refusals)
	}
	return verdict
}

// requireRefusal fails unless a one-claim lock of id is refused with want among
// its refusals, and returns the verdict for further assertions.
func requireRefusal(t *testing.T, claims []model.Claim, id string, cfg *config.Config, store *Store, want string) CandidateVerdict {
	t.Helper()
	verdict := verdictFor(t, claims, id, cfg, store)
	for _, r := range verdict.Refusals {
		if r == want {
			return verdict
		}
	}
	t.Fatalf("lock of %s: refusals = %v, want %q among them", id, verdict.Refusals, want)
	return verdict
}
