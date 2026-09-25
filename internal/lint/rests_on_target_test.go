package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestRestsOnTargetLint pins the target rule (NIT-24, NIT-25) one edge at a
// time: project.<slug> and any module's *.contract.* are open to everyone, a
// module's own *.internals.* is open to that module only, and a foreign
// module's internals — including every internals claim, from a project claim's
// point of view — are refused. An unknown target is dangling's finding, not
// this rule's, so it is skipped here.
func TestRestsOnTargetLint(t *testing.T) {
	module := func(id, module, facet string, rests ...string) model.Claim {
		return model.Claim{ID: id, Module: module, Facet: facet, Status: model.StatusDraft, RestsOn: model.RestsOnIDs(rests...)}
	}
	project := func(id string, rests ...string) model.Claim {
		return model.Claim{ID: id, Scope: model.ScopeProject, Status: model.StatusDraft, RestsOn: model.RestsOnIDs(rests...)}
	}
	roof := model.Claim{ID: "project.roof", Scope: model.ScopeProject, Status: model.StatusDraft, RestsOn: model.RestsNone("the roof above it is the constitution")}
	localContract := module("local.contract.main", "local", "contract")
	localInternals := module("local.internals.detail", "local", "internals")
	otherContract := module("other.contract.api", "other", "contract")
	otherInternals := module("other.internals.secret", "other", "internals")

	cases := []struct {
		name    string
		claims  []model.Claim
		want    map[string]int // claim id -> findings on that claim
		message string         // a phrase every finding must carry, when any
	}{
		{
			name:   "a module claim may rest on a project claim",
			claims: []model.Claim{roof, module("local.contract.rests", "local", "contract", roof.ID)},
			want:   map[string]int{},
		},
		{
			name:   "a module claim may rest on any module's contract claim",
			claims: []model.Claim{otherContract, module("local.contract.rests", "local", "contract", otherContract.ID)},
			want:   map[string]int{},
		},
		{
			name:   "a module claim may rest on its own internals",
			claims: []model.Claim{localInternals, module("local.contract.rests", "local", "contract", localInternals.ID)},
			want:   map[string]int{},
		},
		{
			name:    "a module claim may not rest on a foreign module's internals",
			claims:  []model.Claim{otherInternals, module("local.contract.rests", "local", "contract", otherInternals.ID)},
			want:    map[string]int{"local.contract.rests": 1},
			message: "foreign module's internals (other.internals.secret)",
		},
		{
			name:   "a project claim may rest on a project claim and on a module's contract claim",
			claims: []model.Claim{roof, localContract, project("project.policy", roof.ID, localContract.ID)},
			want:   map[string]int{},
		},
		{
			name:    "a project claim may not rest on any module's internals",
			claims:  []model.Claim{localInternals, otherInternals, project("project.policy", localInternals.ID, otherInternals.ID)},
			want:    map[string]int{"project.policy": 2},
			message: "a project claim's rests_on may name project.* ids and any module's *.contract.* claims, never *.internals.*",
		},
		{
			name:   "an unknown target is dangling's finding, not this rule's",
			claims: []model.Claim{module("local.contract.rests", "local", "contract", "other.internals.ghost"), project("project.policy", "other.internals.ghost")},
			want:   map[string]int{},
		},
		{
			name:   "rests_on none is not an edge",
			claims: []model.Claim{roof, otherInternals},
			want:   map[string]int{},
		},
		{
			name: "one claim, one finding per refused edge, valid edges kept silent",
			claims: []model.Claim{
				roof, otherContract, otherInternals, localInternals,
				module("local.contract.rests", "local", "contract", roof.ID, otherContract.ID, localInternals.ID, otherInternals.ID),
			},
			want:    map[string]int{"local.contract.rests": 1},
			message: "(other.internals.secret)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string]int{}
			for _, f := range (RestsOnTargetLint{}).Check(tc.claims, nil) {
				if f.LintName != "rests-on-target" {
					t.Fatalf("finding carries lint name %q", f.LintName)
				}
				if tc.message != "" && !strings.Contains(f.Message, tc.message) {
					t.Fatalf("finding on %s lacks %q: %q", f.ClaimID, tc.message, f.Message)
				}
				got[f.ClaimID]++
			}
			if len(got) != len(tc.want) {
				t.Fatalf("findings on %v, want %v", got, tc.want)
			}
			for id, n := range tc.want {
				if got[id] != n {
					t.Fatalf("%s: %d finding(s), want %d (all: %v)", id, got[id], n, got)
				}
			}
		})
	}
}

func TestRestsOnTargetLintIsRegistered(t *testing.T) {
	for _, l := range Registry {
		if l.Name() == "rests-on-target" {
			return
		}
	}
	t.Fatal("rests-on-target is not in the lint registry")
}
