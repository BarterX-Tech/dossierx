package lock

import (
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
				{ID: "widget.contract.b", Status: model.StatusDraft, RestsOn: []string{"widget.contract.a"}},
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
