package viewertests

// One parity test, load-bearing for a property nobody can see by reading
// either side.
//
// The graph pane computes its gap verdicts in the BROWSER, against the
// reader's current scope, because a gap list precomputed in Go would be
// confidently wrong the moment a reader narrows the view to one module. The
// engine's lint computes its verdicts in Go, project-wide. Those two are
// allowed to disagree — scope makes divergence unavoidable — but there is
// exactly one configuration in which they are defined over the same edges,
// and in that configuration they must agree exactly.
//
// Both sides now walk rests_on only. The panel's default `isolated` set
// and lint's `orphan` set are the same unscoped, rests_on-only question,
// so they must agree on this corpus. The corpus still contains a claim
// with no edges at all (widget.contract.base) and a second no-edge claim
// (widget.contract.lonely), so the assertion is not accidentally true of
// a corpus with nothing isolated.

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
)

// parityConfig is one module and one facet: the parity is over edges, not
// over grouping, so nothing here needs a second module to be interesting.
const parityConfig = `schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
`

// parityClaims seeds four claims covering both sides of the rule:
//
//	base    no edges at all                  -> orphan, isolated
//	root    the target of leaf's rests_on    -> neither
//	leaf    rests_on root                    -> neither
//	lonely  no edges either                  -> orphan, isolated
//
// Every one of them is legal for `dossierx check`: orphan is a WARNING, so
// the corpus renders and the process exits 0.
var parityClaims = map[string]string{
	"base.yaml": `id: widget.contract.base
facet: contract
module: widget
status: draft
body: |
  a claim with no edges in either direction.`,
	"root.yaml": `id: widget.contract.root
facet: contract
module: widget
status: draft
body: |
  a claim other claims rest on.`,
	"leaf.yaml": `id: widget.contract.leaf
facet: contract
module: widget
status: draft
rests_on:
  - widget.contract.root
body: |
  a claim that rests on the root.`,
	"lonely.yaml": `id: widget.contract.lonely
facet: contract
module: widget
status: draft
body: |
  a claim with no rests_on edges.
`,
}

// checkEnvelope is the subset of `dossierx check --format json` this test
// reads. The per-finding rule id lives at data.lint_findings[].lint — the
// same key tests/lint_fixtures_test.go's lintFinding struct reads.
type checkEnvelope struct {
	Data struct {
		LintFindings []struct {
			LintName string `json:"lint"`
			ClaimID  string `json:"claim_id"`
			Severity string `json:"severity"`
		} `json:"lint_findings"`
	} `json:"data"`
}

// orphanIDs runs check on the machine surface and returns the claim ids the
// orphan lint fired on, sorted.
func orphanIDs(t *testing.T, p *project) []string {
	t.Helper()
	// --format json is passed after the subcommand and wins over the
	// --format text the run helper pins, because pflag takes the LAST
	// occurrence of a repeated flag.
	out := p.run("check", "--format", "json")
	var env checkEnvelope
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("parse check envelope: %v\n%s", err, out)
	}
	var ids []string
	for _, f := range env.Data.LintFindings {
		if f.LintName == "orphan" {
			ids = append(ids, f.ClaimID)
		}
	}
	sort.Strings(ids)
	return ids
}

func TestGraphIsolatedMatchesOrphanLintUnscoped(t *testing.T) {
	p := newProjectRaw(t, parityConfig)
	for name, body := range parityClaims {
		p.writeClaim(name, body)
	}

	wantOrphans := []string{"widget.contract.base", "widget.contract.lonely"}
	orphans := orphanIDs(t, p)
	if fmt.Sprint(orphans) != fmt.Sprint(wantOrphans) {
		t.Fatalf("orphan lint findings = %v, want %v", orphans, wantOrphans)
	}

	ctx := staticGraphTab(t, p)

	// The panel's verdict, computed the way the panel computes it: unscoped
	// (every node), default type set (rests_on only). The pane need not even
	// be open — these are pure functions over the payload the document carries.
	isolated := evalStrings(t, ctx, `(function () {
		var p = JSON.parse(document.getElementById('dossierx-graph').textContent);
		var gaps = window.dossierxGraphCore.gapRules(p.nodes, p.edges, {});
		var out = [];
		for (var i = 0; i < gaps.facts.length; i++) {
			if (gaps.facts[i].rule === 'isolated') { out = out.concat(gaps.facts[i].node_ids); }
		}
		return out;
	})()`)
	sort.Strings(isolated)

	if fmt.Sprint(isolated) != fmt.Sprint(orphans) {
		t.Fatalf("unscoped isolated set = %v, orphan lint findings = %v — these two must agree exactly", isolated, orphans)
	}
	if len(isolated) == 0 {
		t.Fatal("both sets are empty: this corpus proves nothing")
	}
}
