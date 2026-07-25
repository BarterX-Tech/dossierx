// lint_fixtures_test.go proves every registered lint rule (internal/lint's
// Registry) actually fires via the real CLI ("dossierx lint --json"), not just
// in a package-internal unit test: it table-drives every synthetic project
// under testdata/fixture-coverage/lint/<rule-name>/, one directory per
// rule, each deliberately shaped to trip exactly that one rule.
//
// These fixtures are read-only from "dossierx lint"'s point of view (lint never
// writes anything to disk -- see cmd/dossierx/main.go's newLintCmd), so
// unlike the lock-lifecycle fixtures elsewhere in this package, they are
// run directly against their checked-in testdata directory via --config,
// with no need to copy into a t.TempDir() first.
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// lintFinding mirrors the JSON shape "dossierx lint --json" encodes
// internal/lint.Finding as (see cmd/dossierx/main.go's reportLintFindings,
// which json.Marshals []lint.Finding directly -- no custom json tags, so
// the field names are the Go struct's own).
type lintFinding struct {
	LintName string `json:"LintName"`
	ClaimID  string `json:"ClaimID"`
	Message  string `json:"Message"`
	Severity string `json:"Severity"`
}

// coFiresWith records the small set of *legitimate* additional rules a
// given fixture's target rule is expected to fire alongside, per real
// engine design (not an accident of fixture authoring). Two rules overlap
// on purpose:
//
//   - mirror-unanchored: a mirrors[] edge to a nonexistent id is also, by
//     internal/lint/dangling.go's own doc comment, a "dangling" edge --
//     dangling covers mirrors/rests_on/governed_by uniformly, and
//     mirror-unanchored is a strictly more specific rule layered on top
//     for this one edge kind, not a replacement for it.
//   - validated-on-missing: the same overlap, for a governed_by.type that
//     names a doctrine claim id with no matching claim.
//
// Every other fixture is built to trip its target rule alone.
var coFiresWith = map[string][]string{
	"mirror-unanchored":    {"dangling"},
	"validated-on-missing": {"dangling"},
}

// lintFixtureExpectedExit is the exit code "dossierx lint" must produce for a
// given rule's fixture: 0 for the two WARNING-severity rules (orphan,
// body-edge-hint -- see their own doc comments in internal/lint), 1 (a
// lint failure) for every ERROR-severity rule.
var lintFixtureExpectedExit = map[string]int{
	"orphan":         0,
	"body-edge-hint": 0,
}

func TestLintRuleCoverageFixtures(t *testing.T) {
	root := lintFixturesRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}

	if len(entries) != 22 {
		t.Fatalf("expected exactly 22 lint fixture directories (one per registered lint rule), found %d: %v", len(entries), entries)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rule := e.Name()
		t.Run(rule, func(t *testing.T) {
			testLintFixtureFiresExactlyOneRule(t, filepath.Join(root, rule), rule)
		})
	}
}

// lintFixturesRoot resolves testdata/fixture-coverage/lint relative to this
// test file's own package directory (tests/), independent of the test
// binary's working directory.
func lintFixturesRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "testdata", "fixture-coverage", "lint"))
	if err != nil {
		t.Fatalf("resolve fixture-coverage/lint path: %v", err)
	}
	return abs
}

// testLintFixtureFiresExactlyOneRule runs "dossierx lint --json" against
// fixtureDir and asserts: (a) the expected exit code for wantRule's
// severity, (b) wantRule appears at least once in the parsed findings, and
// (c) every OTHER rule name present is one of wantRule's documented,
// legitimate co-firing rules (coFiresWith) -- proving each fixture is
// isolated to its target rule (plus only the overlap the engine's own
// design intends).
func testLintFixtureFiresExactlyOneRule(t *testing.T, fixtureDir, wantRule string) {
	t.Helper()

	cfgPath := filepath.Join(fixtureDir, "project.config.yaml")
	stdout, stderr, code := run(t, fixtureDir, "--config", cfgPath, "lint", "--json")

	wantExit := 1
	if e, ok := lintFixtureExpectedExit[wantRule]; ok {
		wantExit = e
	}
	if code != wantExit {
		t.Fatalf("fixture %q: expected exit %d, got %d\nstdout: %s\nstderr: %s", wantRule, wantExit, code, stdout, stderr)
	}

	var findings []lintFinding
	if err := json.Unmarshal([]byte(stdout), &findings); err != nil {
		t.Fatalf("fixture %q: lint --json output is not valid JSON: %v\noutput: %s", wantRule, err, stdout)
	}

	allowed := map[string]bool{wantRule: true}
	for _, extra := range coFiresWith[wantRule] {
		allowed[extra] = true
	}

	sawWantRule := false
	var unexpected []string
	for _, f := range findings {
		if f.LintName == wantRule {
			sawWantRule = true
		}
		if !allowed[f.LintName] {
			unexpected = append(unexpected, f.LintName+": "+f.Message+" ("+f.ClaimID+")")
		}
	}

	if !sawWantRule {
		t.Fatalf("fixture %q: expected rule %q to fire at least once, but it did not; findings: %+v", wantRule, wantRule, findings)
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Fatalf("fixture %q: expected only %v to fire, but also got:\n  %s", wantRule, allowedList(allowed), joinLines(unexpected))
	}
}

func allowedList(allowed map[string]bool) []string {
	out := make([]string, 0, len(allowed))
	for k := range allowed {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n  "
		}
		out += l
	}
	return out
}
