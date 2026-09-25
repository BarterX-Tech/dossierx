package lint

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestSharedContextBudget(t *testing.T) {
	cfg := &config.Config{}
	summary := strings.Repeat("s", 200)
	var claims []model.Claim
	add := func(n int) {
		for i := len(claims) + 1; i <= n; i++ {
			claims = append(claims, model.Claim{ID: fmt.Sprintf("project.p%03d", i), Scope: model.ScopeProject, Status: model.StatusDraft, Summary: summary})
		}
	}

	add(10)
	if got := (sharedContextBudgetLint{}).Check(claims, cfg); len(got) != 0 {
		t.Fatalf("10 project claims fit: %+v", got)
	}
	if got := (sharedContextBudgetLint{}).Check(claims, nil); got != nil {
		t.Fatalf("nil cfg: %+v", got)
	}

	add(60)
	got := (sharedContextBudgetLint{}).Check(claims, cfg)
	over, err := manifest.CheckSharedBudget(manifest.BuildSharedContext(claims, cfg))
	if err != nil || over == nil {
		t.Fatalf("60 project claims must be over: %+v %v", over, err)
	}
	if len(got) != 1 || got[0].ClaimID != over.ClaimID || got[0].Severity != SeverityError {
		t.Fatalf("finding = %+v, want one error on %s", got, over.ClaimID)
	}
	for _, want := range []string{"60 project claim summaries", fmt.Sprint(manifest.SharedBudgetBytes), over.ClaimID} {
		if !strings.Contains(got[0].Message, want) {
			t.Fatalf("message %q does not say %q", got[0].Message, want)
		}
	}
	// Module claims never count toward the shared budget.
	claims = claims[:10]
	for i := 0; i < 50; i++ {
		claims = append(claims, model.Claim{ID: fmt.Sprintf("mod.contract.c%02d", i), Module: "mod", Facet: "contract", Summary: summary})
	}
	if got := (sharedContextBudgetLint{}).Check(claims, cfg); len(got) != 0 {
		t.Fatalf("module claims counted: %+v", got)
	}
}
