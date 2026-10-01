package main

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// TestLedgerRecoveryHint_BriefCommentDriftNamesBriefUnlock is combo-audit
// ENG-2: a comment-ledger-drift finding keyed by a brief path must not tell
// an agent to claim unlock / claim lock.
func TestLedgerRecoveryHint_BriefCommentDriftNamesBriefUnlock(t *testing.T) {
	hint := ledgerRecoveryHint([]lock.Finding{{
		Rule:    lock.RuleCommentLedgerDrift,
		ClaimID: "briefs/widget/flow.md",
		Message: "forged",
	}})
	if strings.Contains(hint, "claim unlock") || strings.Contains(hint, "claim lock") {
		t.Fatalf("brief-path drift must not name claim unlock/lock: %q", hint)
	}
	for _, want := range []string{"brief unlock", "brief lock", "check --validate"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("hint %q does not name %q", hint, want)
		}
	}
	claimHint := ledgerRecoveryHint([]lock.Finding{{
		Rule:    lock.RuleCommentLedgerDrift,
		ClaimID: "widget.contract.overview",
		Message: "forged",
	}})
	if !strings.Contains(claimHint, "claim unlock") {
		t.Fatalf("a claim-id finding still names claim unlock: %q", claimHint)
	}
}
