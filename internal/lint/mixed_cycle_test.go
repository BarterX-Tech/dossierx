package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestMixedCycle drives the rule over []model.Claim literals rather than a
// fixture directory, for the reason the design gives: no corpus that passes
// "dossierx check" can contain a cycle of any shape, so the only place this
// defect class can be exercised at scale is in memory.
//
// The two negative cases are the load-bearing ones. A pure rests_on cycle and
// a pure governed_by cycle must NOT fire here, because
// tests/lint_fixtures_test.go requires every coverage fixture to trip its own
// rule and nothing else -- a mixed-cycle that also fired on the "cycle" and
// "governed-cycle" fixtures would break both of them.
func TestMixedCycle(t *testing.T) {
	// deepChainThenMixedCycle builds an acyclic chain n0 -rests_on-> n1 -> …
	// -> n(depth-1) and hangs a 2-claim mixed cycle off its far end. The walk
	// therefore has to descend `depth` frames before it finds anything, which
	// is the whole point: recursion here would be bounded only by the longest
	// authored edge chain, and that is exactly the overflow findEdgeCycles was
	// rewritten to remove.
	//
	// The cycle is kept to two members deliberately. A 10,000-member cycle
	// would be a legitimate finding too, but its message names the whole path
	// once per member, so the case would allocate gigabytes to prove a
	// property the small cycle already proves.
	deepChainThenMixedCycle := func(depth int) []model.Claim {
		claims := make([]model.Claim, 0, depth+2)
		none := model.Governed{Type: "none", Reason: "chain fixture"}
		for i := 0; i < depth; i++ {
			next := "deep.contract.n" + itoaTest(i+1)
			if i+1 == depth {
				next = "deep.contract.loop-a"
			}
			claims = append(claims, model.Claim{
				ID:       "deep.contract.n" + itoaTest(i),
				RestsOn:  []string{next},
				Governed: none,
			})
		}
		claims = append(claims,
			model.Claim{ID: "deep.contract.loop-a", RestsOn: []string{"deep.contract.loop-b"}, Governed: none},
			model.Claim{ID: "deep.contract.loop-b", Governed: model.Governed{Type: "deep.contract.loop-a"}},
		)
		return claims
	}

	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
		wantInMsg    []string
	}{
		{
			// The hole this rule closes: neither existing cycle walk takes
			// the union, so this loop has no back edge either can find.
			name: "failing: a rests_on b, b governed_by a",
			claims: []model.Claim{
				{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
				{ID: "widget.contract.b", Governed: model.Governed{Type: "widget.contract.a"}},
			},
			wantFindings: 2,
			wantInMsg:    []string{"-(rests_on)->", "-(governed_by)->"},
		},
		{
			name: "failing: three-claim alternating cycle",
			claims: []model.Claim{
				{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
				{ID: "widget.contract.b", Governed: model.Governed{Type: "widget.contract.c"}},
				{ID: "widget.contract.c", RestsOn: []string{"widget.contract.a"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
			},
			wantFindings: 3,
			wantInMsg:    []string{"-(rests_on)->", "-(governed_by)->"},
		},
		{
			// "cycle"'s own coverage fixture, in miniature. It must stay
			// silent here.
			name: "passing: pure rests_on cycle belongs to the cycle lint",
			claims: []model.Claim{
				{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
				{ID: "widget.contract.b", RestsOn: []string{"widget.contract.a"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
			},
			wantFindings: 0,
		},
		{
			// "governed-cycle"'s own coverage fixture, in miniature.
			name: "passing: pure governed_by cycle belongs to the governed-cycle lint",
			claims: []model.Claim{
				{ID: "widget.contract.a", Governed: model.Governed{Type: "widget.contract.b"}},
				{ID: "widget.contract.b", Governed: model.Governed{Type: "widget.contract.a"}},
			},
			wantFindings: 0,
		},
		{
			// A one-hop loop can never carry two edge kinds, so it is
			// self-edge's finding and never this rule's.
			name: "passing: rests_on self-edge",
			claims: []model.Claim{
				{ID: "widget.contract.self", RestsOn: []string{"widget.contract.self"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
			},
			wantFindings: 0,
		},
		{
			name: "passing: governed_by self-edge",
			claims: []model.Claim{
				{ID: "widget.contract.self", Governed: model.Governed{Type: "widget.contract.self"}},
			},
			wantFindings: 0,
		},
		{
			// The two sentinel values must contribute no edge at all, the
			// same guard governedByEdges already applies.
			name: "passing: type none and type empty contribute no edge",
			claims: []model.Claim{
				{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
				{ID: "widget.contract.b", RestsOn: []string{"widget.contract.a"}, Governed: model.Governed{Type: ""}},
			},
			wantFindings: 0,
		},
		{
			// An edge naming an id nothing defines is dangling's finding.
			name: "passing: unresolved governed_by target is another lint's concern",
			claims: []model.Claim{
				{ID: "widget.contract.a", RestsOn: []string{"widget.contract.ghost"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
				{ID: "widget.contract.b", Governed: model.Governed{Type: "widget.contract.ghost"}},
			},
			wantFindings: 0,
		},
		{
			// mirrors is not part of the union: it is reciprocal by design,
			// so every mirrored pair would otherwise be a "cycle".
			name: "passing: mirrors is not one of the two union edge kinds",
			claims: []model.Claim{
				{ID: "widget.contract.a", Mirrors: []string{"widget.contract.b"}, Governed: model.Governed{Type: "widget.contract.b"}},
				{ID: "widget.contract.b", Mirrors: []string{"widget.contract.a"}, Governed: model.Governed{Type: "none", Reason: "fixture"}},
			},
			wantFindings: 0,
		},
		{
			// Depth probe. Recursion here would be bounded only by the
			// longest authored edge chain, which is exactly the failure
			// findEdgeCycles removed; this rule must not reintroduce it.
			name:         "failing: a 10,000-link chain is walked to its end without a stack overflow",
			claims:       deepChainThenMixedCycle(10000),
			wantFindings: 2,
			wantInMsg:    []string{"-(rests_on)->", "-(governed_by)->"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := MixedCycleLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, truncateFindings(findings))
			}
			for _, f := range findings {
				if f.LintName != "mixed-cycle" {
					t.Errorf("finding %+v: LintName = %q, want %q", f, f.LintName, "mixed-cycle")
				}
				// Severity is set explicitly rather than left for RunAll to
				// normalize, so a caller reading Check's return directly
				// (internal/lock.Lock does) sees "error" too.
				if f.Severity != SeverityError {
					t.Errorf("finding %+v: Severity = %q, want %q", f, f.Severity, SeverityError)
				}
				if !strings.Contains(f.Message, "mixed rests_on/governed_by cycle detected") {
					t.Errorf("message %q does not name the defect class", f.Message)
				}
				for _, want := range tc.wantInMsg {
					if !strings.Contains(f.Message, want) {
						t.Errorf("message %q does not carry %q; the path must name each hop's edge kind", f.Message, want)
					}
				}
			}
		})
	}
}

// TestMixedCycleIsRegistered guards the coverage meta-gate's premise: the rule
// has to be in Registry for RunAll (and therefore "dossierx check") to run it.
func TestMixedCycleIsRegistered(t *testing.T) {
	for _, l := range Registry {
		if l.Name() == "mixed-cycle" {
			return
		}
	}
	t.Fatal("mixed-cycle lint is not registered in the lint Registry")
}

// truncateFindings keeps a failure message readable when the 10,000-claim case
// is the one that failed.
func truncateFindings(f []Finding) []Finding {
	if len(f) > 5 {
		return f[:5]
	}
	return f
}

// itoaTest is a local decimal formatter so this file needs no strconv import
// alongside the package's existing ones.
func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
