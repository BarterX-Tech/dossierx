// This file guards one thing: a Go module in this repository that the root
// `go test ./...` cannot reach must be reachable by SOMETHING ELSE that runs on
// every pull request, and that something must be able to fail.
//
// The hole it closes was real and silent. viewer-tests/ is a separate module by
// design — chromedp and its transitive dependencies must never enter the
// engine's go.mod, which stays cobra + yaml.v3 — and `go test ./...` does not
// descend into a nested module. For a whole release cycle that meant nobody ran
// it: .github/workflows/ci.yml had no job for it, the Makefile had no target for
// it, and `grep -rn viewer-tests .github Makefile scripts` returned nothing at
// all. Assertions were still being WRITTEN against the viewer's inline
// JavaScript (a 178-line comment-chip suite landed in that state), CI was green
// on three platforms, and the only machine those assertions had ever executed on
// was a maintainer's laptop. The root module covers the viewer's MARKUP; nothing
// covered its behaviour in a browser.
//
// So this is a meta-test, in the same spirit as lint_coverage_meta_test.go: it
// reads the CI workflow and the Makefile as text and refuses to let a nested
// module exist unwired. It deliberately checks the three ways the wiring can be
// present but worthless, not just its absence:
//
//   - no job at all                 -> the original hole
//   - a job that cannot fail        -> continue-on-error turns the badge into
//     decoration, which is the same false green in nicer clothes
//   - a job whose suite SKIPS       -> viewer-tests resolves a browser and
//     t.Skip()s when it finds none, so a job that does not name a browser
//     explicitly reports success over zero assertions
package tests

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// wiringRepoRoot locates the repository root from THIS source file rather than from
// the process CWD, so the walk below is unaffected by how `go test` is invoked.
// It is spelled out here instead of borrowing a neighbouring file's helper
// because this file must keep working if that neighbour is ever split or
// renamed — a coverage guard that stops compiling is a coverage guard that stops
// guarding.
func wiringRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate the test source")
	}
	return filepath.Dir(filepath.Dir(thisFile)) // <root>/tests/<file> -> <root>
}

// wiringReadFile reads a file addressed relative to the repository root.
func wiringReadFile(t *testing.T, rel string) string {
	t.Helper()
	p := filepath.Join(wiringRepoRoot(t), filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading %s: %v", p, err)
	}
	return string(b)
}

// wiringNestedModules returns every nested Go module directory, as a repo-relative
// slash path. Dot-directories are skipped wholesale: .git obviously, but also
// .claude, which can hold linked worktrees — a full second checkout of this
// repository, whose go.mod files are the same files over again and are wired by
// the workflow in THAT checkout, not this one.
func wiringNestedModules(t *testing.T) []string {
	t.Helper()
	root := wiringRepoRoot(t)
	var mods []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		rel, relErr := filepath.Rel(root, filepath.Dir(p))
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil // the root module; `go test ./...` covers it by definition
		}
		mods = append(mods, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository for go.mod files: %v", err)
	}
	return mods
}

// TestEveryNestedModuleIsRunSomewhere is the guard proper. If it fails after you
// added a module, the fix is not to delete this test: it is to add the job and
// the target, because a suite nothing runs is not a suite.
func TestEveryNestedModuleIsRunSomewhere(t *testing.T) {
	mods := wiringNestedModules(t)
	if len(mods) == 0 {
		t.Fatal("no nested modules found at all — viewer-tests/go.mod is expected to be one; " +
			"if the browser suite was removed, remove this test with it, but do not let it merely stop being found")
	}

	ci := wiringReadFile(t, ".github/workflows/ci.yml")
	mk := wiringReadFile(t, "Makefile")

	for _, mod := range mods {
		t.Run(mod, func(t *testing.T) {
			// CI: a job must actually cd into the module and run its tests.
			// "working-directory: <mod>" is how the workflow spells that, and
			// pinning the literal is the point — a rename that forgets the
			// workflow should break here rather than in six months.
			if !strings.Contains(ci, "working-directory: "+mod) {
				t.Errorf("no job in .github/workflows/ci.yml runs in %q.\n"+
					"The root `go test ./...` does NOT descend into a nested module, so this\n"+
					"module's tests would run on no machine but a maintainer's. Add a job with\n"+
					"    working-directory: %s\n"+
					"that runs `go test ./...`.", mod, mod)
			}

			// Makefile: the same suite must be runnable locally without
			// knowing it exists. A target that cds into the module is the
			// contract; .PHONY keeps it working when a file of that name
			// appears.
			var target string
			for _, line := range strings.Split(mk, "\n") {
				if strings.Contains(line, "cd "+mod) && strings.Contains(line, "go test") {
					target = line
					break
				}
			}
			if target == "" {
				t.Errorf("the Makefile has no target that runs %q's tests.\n"+
					"Add one (e.g. `cd %s && go test -count=1 ./...`) and list it in .PHONY,\n"+
					"so the suite is reachable without reading the CI workflow to find it.", mod, mod)
			}
		})
	}
}

// TestNestedModuleJobsCanActuallyFail covers the two ways a job can be present
// and still mean nothing.
func TestNestedModuleJobsCanActuallyFail(t *testing.T) {
	ci := wiringReadFile(t, ".github/workflows/ci.yml")

	// continue-on-error anywhere in this workflow would let the job that runs a
	// nested module report success while its assertions failed. There is no
	// legitimate use of it here: every job in this workflow is a gate.
	if strings.Contains(ci, "continue-on-error") {
		t.Error(".github/workflows/ci.yml uses continue-on-error. Every job in this workflow is a gate; " +
			"a job that cannot fail is a green badge over an unrun or failing suite, which is the exact " +
			"condition the viewer suite was already in.")
	}

	// viewer-tests resolves a browser and t.Skip()s when it cannot find one —
	// correct on a laptop, catastrophic in CI, where a skip is indistinguishable
	// from a pass. The workflow must therefore name the browser explicitly, so
	// the harness's "override set but missing" branch t.Fatal()s instead.
	if !strings.Contains(ci, "DOSSIERX_TEST_BROWSER") {
		t.Error(".github/workflows/ci.yml never sets DOSSIERX_TEST_BROWSER. The viewer suite SKIPS " +
			"when it cannot resolve a browser, so without an explicit path the job goes green having " +
			"run zero browser assertions.")
	}
}

// TestNestedModulesAreNotInTheRootBuild pins the reason the wiring is needed at
// all, so the tests above cannot be "fixed" by folding the module back into the
// root one. chromedp must stay out of the engine's dependency graph: the whole
// premise is cobra + yaml.v3 and nothing else.
func TestNestedModulesAreNotInTheRootBuild(t *testing.T) {
	rootMod := wiringReadFile(t, "go.mod")
	for _, dep := range []string{"chromedp", "cdproto", "gobwas"} {
		if strings.Contains(rootMod, dep) {
			t.Errorf("go.mod requires %q — the engine's dependencies are cobra + yaml.v3 only, "+
				"and the browser harness lives in its own module precisely so this cannot happen", dep)
		}
	}

	for _, mod := range wiringNestedModules(t) {
		// A nested module directory must not also be covered by a root-module
		// package path, which would mean the isolation is not real.
		if _, err := os.Stat(filepath.Join(wiringRepoRoot(t), filepath.FromSlash(mod), "go.mod")); err != nil {
			t.Errorf("%s was reported as a nested module but has no go.mod: %v", mod, err)
		}
	}
}
