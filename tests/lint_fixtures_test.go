// lint_fixtures_test.go proves every registered lint rule (internal/lint's
// Registry) actually fires via the real CLI ("dossierx check --validate"), not
// just in a package-internal unit test: it table-drives every synthetic project
// under testdata/fixture-coverage/lint/<rule-name>/, one directory per
// rule, each deliberately shaped to trip exactly that one rule.
//
// These fixtures are read-only from the command's point of view — which is now
// a guarantee rather than an accident. Before v0.3.0 they were driven by
// "dossierx check --validate", which happened not to write; that verb is gone, and
// "check --validate" is its replacement precisely BECAUSE it is specified never
// to write (plain "check" reconciles review_pending, rewrites the lock store,
// and regenerates .catalog.json and the viewer, all of which would dirty a
// checked-in fixture directory). So these still run directly against their
// testdata directory via --config, with no need to copy into a t.TempDir().
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// lintFinding is one entry of the check envelope's data.lint_findings.
//
// Before v0.3.0 this mirrored the Go field names of internal/lint.Finding,
// because "dossierx check --validate" json.Marshal'd that struct directly and it
// carries no tags. The envelope projects it into snake_case instead (see
// cmd/dossierx's lintFindingData and TestEnvelopeKeysAreSnakeCase), so the
// keys here are the contract's, not Go's.
type lintFinding struct {
	LintName string `json:"lint"`
	ClaimID  string `json:"claim_id"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// validateEnvelope is the subset of the check envelope these fixtures read.
// Data survives onto a FAILED envelope too (that is the whole point of the
// contract's partial-result rule), which is what lets one helper serve both
// the warning-severity fixtures that exit 0 and the error-severity ones that
// exit 1.
type validateEnvelope struct {
	OK   bool `json:"ok"`
	Data struct {
		ReadOnly     bool          `json:"read_only"`
		LintFindings []lintFinding `json:"lint_findings"`
	} `json:"data"`
}

// runValidateFindings runs "dossierx check --validate --format json" in
// fixtureDir against cfgPath and returns the findings plus the exit code.
//
// "--format", "json" is passed explicitly and wins over the "--format text"
// the run helper prepends, because pflag takes the LAST occurrence of a
// repeated flag — see run's own doc comment.
func runValidateFindings(t *testing.T, fixtureDir, cfgPath string) (findings []lintFinding, exitCode int) {
	t.Helper()
	stdout, stderr, code := run(t, fixtureDir, "--config", cfgPath, "--format", "json", "check", "--validate")
	var env validateEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("check --validate output is not a single envelope: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !env.Data.ReadOnly {
		t.Fatalf("check --validate must mark its payload read_only; got: %s", stdout)
	}
	return env.Data.LintFindings, code
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
//   - cycle: its fixture includes the degenerate one-node case (a claim
//     whose rests_on names itself), which is by construction also a
//     self-edge. Both rules are telling the truth about that claim -- it is
//     a self-reference AND a cycle in the dependency graph -- and see
//     internal/lint/self_edge.go for why neither suppresses the other.
//
// Every other fixture is built to trip its target rule alone.
var coFiresWith = map[string][]string{
	"mirror-unanchored":    {"dangling"},
	"validated-on-missing": {"dangling"},
	"cycle":                {"self-edge"},
}

// lintFixtureExpectedExit is the exit code "dossierx check --validate" must
// produce for a given rule's fixture: 0 for the three WARNING-severity rules (orphan,
// body-edge-hint, comments-unresolved -- see their own doc comments in
// internal/lint), 1 (a lint failure) for every ERROR-severity rule.
var lintFixtureExpectedExit = map[string]int{
	"orphan":              0,
	"body-edge-hint":      0,
	"comments-unresolved": 0,
}

func TestLintRuleCoverageFixtures(t *testing.T) {
	root := lintFixturesRoot(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}

	if len(entries) != 28 {
		t.Fatalf("expected exactly 28 lint fixture directories (one per registered lint rule), found %d: %v", len(entries), entries)
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

// testLintFixtureFiresExactlyOneRule runs "dossierx check --validate" against
// fixtureDir and asserts: (a) the expected exit code for wantRule's
// severity, (b) wantRule appears at least once in the parsed findings, and
// (c) every OTHER rule name present is one of wantRule's documented,
// legitimate co-firing rules (coFiresWith) -- proving each fixture is
// isolated to its target rule (plus only the overlap the engine's own
// design intends).
func testLintFixtureFiresExactlyOneRule(t *testing.T, fixtureDir, wantRule string) {
	t.Helper()

	cfgPath := filepath.Join(fixtureDir, "project.config.yaml")
	findings, code := runValidateFindings(t, fixtureDir, cfgPath)

	wantExit := 1
	if e, ok := lintFixtureExpectedExit[wantRule]; ok {
		wantExit = e
	}
	if code != wantExit {
		t.Fatalf("fixture %q: expected exit %d, got %d (findings: %+v)", wantRule, wantExit, code, findings)
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
