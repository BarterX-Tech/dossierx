package lock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestEvaluateSetRefusesRetiredAndUnreadableRequiredDependencies(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  model.Status
		refusal string
	}{
		{name: "retired", status: model.Status("retired"), refusal: "retired_dependency:widget.contract.a"},
		{name: "unreadable", status: model.Status("migration-unknown"), refusal: "unreadable_dependency:widget.contract.a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := []model.Claim{
				{ID: "widget.contract.a", Status: tc.status},
				{ID: "widget.contract.b", Status: model.StatusDraft, RestsOn: model.RestsOnIDs("widget.contract.a")},
			}
			evaluation := EvaluateSet(claims, []string{"widget.contract.b"}, nil, &Store{PolicyVersion: PolicyLocalApprovalV1})
			if len(evaluation.Verdicts) != 1 || evaluation.Verdicts[0].LocalAdmissible {
				t.Fatalf("required %s dependency must refuse local approval: %+v", tc.name, evaluation)
			}
			found := false
			for _, refusal := range evaluation.Verdicts[0].Refusals {
				if refusal == tc.refusal {
					found = true
				}
			}
			if !found {
				t.Fatalf("required %s dependency refusal = %v, want %q", tc.name, evaluation.Verdicts[0].Refusals, tc.refusal)
			}
		})
	}
}

// TestLoadStorePolicyCarryOverStampsWithTheStoreClock: the policy-0 carry-over
// stamp is written by the next Save beside every other store timestamp, so it
// must come from the store's clock in the store's RFC3339Nano format, not the
// wall clock at second precision.
func TestLoadStorePolicyCarryOverStampsWithTheStoreClock(t *testing.T) {
	const at = "2026-03-04T05:06:07.123456789Z"
	freezeClock(t, at)
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"version": 2, "policy_version": 0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if s.PolicyVersion != PolicyLocalApprovalV1 || s.PolicyMigratedAt != at {
		t.Fatalf("carry-over = version %d at %q, want version %d at %q", s.PolicyVersion, s.PolicyMigratedAt, PolicyLocalApprovalV1, at)
	}
}
