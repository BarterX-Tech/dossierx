package lint

import (
	"fmt"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestCycleLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: acyclic rests_on chain",
			claims: []model.Claim{
				{ID: "widget.contract.overview"},
				{ID: "widget.internals.fields", RestsOn: []string{"widget.contract.overview"}},
				{ID: "widget.internals.detail", RestsOn: []string{"widget.internals.fields"}},
			},
			wantFindings: 0,
		},
		{
			name: "failing: three-claim rests_on cycle",
			claims: []model.Claim{
				{ID: "widget.a.one", RestsOn: []string{"widget.b.two"}},
				{ID: "widget.b.two", RestsOn: []string{"widget.c.three"}},
				{ID: "widget.c.three", RestsOn: []string{"widget.a.one"}},
			},
			wantFindings: 3,
		},
		{
			name: "failing: claim rests_on itself is a degenerate one-claim cycle",
			claims: []model.Claim{
				{ID: "widget.contract.self", RestsOn: []string{"widget.contract.self"}},
			},
			wantFindings: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := CycleLint{}.Check(tc.claims, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
		})
	}
}

// TestCycleLintFindingShape pins the finding this lint has always produced —
// one per cycle member, in DFS discovery order, each carrying the full cycle
// path closed back onto its entry node. The DFS was rewritten from a
// recursive closure to an explicit frame stack (see findEdgeCycles); that
// rewrite was required to change nothing a reader or a downstream consumer
// sees, and this is what holds it to that.
func TestCycleLintFindingShape(t *testing.T) {
	findings := CycleLint{}.Check([]model.Claim{
		{ID: "widget.a.one", RestsOn: []string{"widget.b.two"}},
		{ID: "widget.b.two", RestsOn: []string{"widget.c.three"}},
		{ID: "widget.c.three", RestsOn: []string{"widget.a.one"}},
	}, nil)

	want := []Finding{
		{LintName: "cycle", ClaimID: "widget.a.one", Message: "rests_on cycle detected: widget.a.one -> widget.b.two -> widget.c.three -> widget.a.one"},
		{LintName: "cycle", ClaimID: "widget.b.two", Message: "rests_on cycle detected: widget.a.one -> widget.b.two -> widget.c.three -> widget.a.one"},
		{LintName: "cycle", ClaimID: "widget.c.three", Message: "rests_on cycle detected: widget.a.one -> widget.b.two -> widget.c.three -> widget.a.one"},
	}
	if len(findings) != len(want) {
		t.Fatalf("got %d findings, want %d: %+v", len(findings), len(want), findings)
	}
	for i, w := range want {
		if findings[i] != w {
			t.Errorf("finding %d:\n got %+v\nwant %+v", i, findings[i], w)
		}
	}
}

// TestCycleLintDeepChainDoesNotPanic is the regression test for the recursion
// removal. Chain length here is authored data — nothing in the engine bounds
// how deep a rests_on chain a generator or a bulk migration can produce — so
// the old recursive walk turned a deep-enough project into an unrecoverable
// goroutine-stack overflow: the binary died instead of reporting, which is
// the one thing a lint must never do. The iterative walk is bounded by the
// heap instead, so a chain far deeper than anything hand-authored still
// finishes and still finds the cycle sitting at the end of it.
func TestCycleLintDeepChainDoesNotPanic(t *testing.T) {
	const depth = 100_000

	claims := make([]model.Claim, 0, depth+2)
	for i := 0; i < depth; i++ {
		next := fmt.Sprintf("widget.chain.n%d", i+1)
		if i == depth-1 {
			// The tail hands off to a small, self-contained 2-claim cycle,
			// rather than closing the whole chain into one giant cycle: that
			// keeps the assertion about depth (does the walk survive it?)
			// instead of about the size of a cycle-path message.
			next = "widget.chain.loop-a"
		}
		claims = append(claims, model.Claim{
			ID:      fmt.Sprintf("widget.chain.n%d", i),
			RestsOn: []string{next},
		})
	}
	claims = append(claims,
		model.Claim{ID: "widget.chain.loop-a", RestsOn: []string{"widget.chain.loop-b"}},
		model.Claim{ID: "widget.chain.loop-b", RestsOn: []string{"widget.chain.loop-a"}},
	)

	findings := CycleLint{}.Check(claims, nil)
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2 (the two claims in the loop at the end of the chain): %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.ClaimID != "widget.chain.loop-a" && f.ClaimID != "widget.chain.loop-b" {
			t.Errorf("unexpected claim reported: %+v", f)
		}
	}
}
