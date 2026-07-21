// lint_coverage_meta_test.go is the regression gate for the whole
// testdata/fixture-coverage corpus: TestEveryRegisteredLintHasACoverageFixture
// iterates internal/lint's actual Registry (not a hardcoded rule-name list,
// so a future lint rule is picked up automatically), runs "docs lint --json"
// against every directory under testdata/fixture-coverage/lint/* and
// testdata/fixture-coverage/lifecycle/*, unions every lint rule name that
// fired anywhere across that whole set, and fails with a clear message
// naming any registered rule that never fired -- catching a future lint
// rule that ships with zero fixture coverage.
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lint"
)

func TestEveryRegisteredLintHasACoverageFixture(t *testing.T) {
	registered := make(map[string]bool, len(lint.Registry))
	for _, l := range lint.Registry {
		registered[l.Name()] = true
	}
	if len(registered) == 0 {
		t.Fatalf("internal/lint.Registry is empty; nothing to check coverage against")
	}

	fired := make(map[string]bool)
	for _, group := range []string{"lint", "lifecycle"} {
		groupDir := filepath.Join(coverageRoot(t), group)
		entries, err := os.ReadDir(groupDir)
		if err != nil {
			t.Fatalf("read %s: %v", groupDir, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			fixtureDir := filepath.Join(groupDir, e.Name())
			cfgPath := filepath.Join(fixtureDir, "project.config.yaml")
			if _, err := os.Stat(cfgPath); err != nil {
				// Not every lifecycle scenario is a static fixture (see
				// tests/lifecycle_fixtures_test.go's scenarios 4 and 5,
				// built programmatically instead); skip anything without
				// its own project.config.yaml rather than failing here.
				continue
			}

			stdout, stderr, _ := run(t, fixtureDir, "--config", cfgPath, "lint", "--json")
			var findings []lintFinding
			if err := json.Unmarshal([]byte(stdout), &findings); err != nil {
				t.Fatalf("fixture %s: lint --json output is not valid JSON: %v\nstdout: %s\nstderr: %s", fixtureDir, err, stdout, stderr)
			}
			for _, f := range findings {
				fired[f.LintName] = true
			}
		}
	}

	var missing []string
	for name := range registered {
		if !fired[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("the following %d registered lint rule(s) never fired across the entire testdata/fixture-coverage corpus (add a testdata/fixture-coverage/lint/<rule-name>/ fixture for each): %v", len(missing), missing)
	}

	t.Logf("all %d registered lint rules fired at least once across the fixture-coverage corpus: %v", len(registered), sortedKeys(fired))
}

// coverageRoot resolves testdata/fixture-coverage relative to this test
// file's own package directory (tests/), independent of the test binary's
// working directory.
func coverageRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "testdata", "fixture-coverage"))
	if err != nil {
		t.Fatalf("resolve fixture-coverage path: %v", err)
	}
	return abs
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
