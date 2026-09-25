package projectclaims

import (
	"reflect"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestIndexIsOneLinePerProjectClaimSortedById(t *testing.T) {
	claims := []model.Claim{
		{ID: "widget.contract.a", Module: "widget", Facet: "contract", Status: model.StatusLocked, Body: "a module claim"},
		{ID: "project.retention", Scope: model.ScopeProject, Status: model.StatusDraft, Body: "\n\n  Data is kept for 30 days.  \nMore detail below.\n"},
		{ID: "project.audience", Scope: model.ScopeProject, Status: model.StatusLocked, Body: "Built for operators, not end users."},
		{ID: "project.empty", Scope: model.ScopeProject, Status: model.StatusDraft, Body: ""},
	}
	got := Index(claims)
	want := []Entry{
		{ID: "project.audience", Summary: "Built for operators, not end users.", Status: "locked"},
		{ID: "project.empty", Summary: "", Status: "draft"},
		{ID: "project.retention", Summary: "Data is kept for 30 days.", Status: "draft"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Index = %+v\nwant %+v", got, want)
	}
	if Index(nil) != nil {
		t.Fatal("no claims, no index")
	}
}

func TestSummaryIsTheFirstNonBlankBodyLine(t *testing.T) {
	if got := Summary(model.Claim{Body: "\n \nfirst line\nsecond"}); got != "first line" {
		t.Fatalf("Summary = %q", got)
	}
	if got := Summary(model.Claim{Body: "   "}); got != "" {
		t.Fatalf("blank body summarises to nothing, got %q", got)
	}
}

func TestAClaimWithTheProjectIdGrammarCountsEvenWithoutScope(t *testing.T) {
	got := Index([]model.Claim{{ID: "project.by-id-only", Body: "found by id"}})
	if len(got) != 1 || got[0].ID != "project.by-id-only" {
		t.Fatalf("Index = %+v", got)
	}
}
