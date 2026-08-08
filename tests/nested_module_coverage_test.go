// This file guards one thing: a Go module in this repository that the root
// `go test ./...` cannot reach must be WIRED INTO the CI workflow, the Makefile
// and the linter — for every such module, not just the one that exists today.
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
// THE WALK IS THE POINT. Everything below is asked once per go.mod found on
// disk, so a nested module added next year is covered by the check as written
// rather than by somebody remembering to add a line to it.
//
// WHAT IS ASKED ABOUT THE WORKFLOW IS ASKED THROUGH tests/ci_workflow_test.go's
// PARSER, not with a substring search over the file, and that is a correction.
// Three checks here used to be `strings.Contains` over the whole of ci.yml —
// for `working-directory: <mod>`, for `continue-on-error` and for
// `DOSSIERX_TEST_BROWSER`. Deleting the ENTIRE viewer job left the first and the
// third of them green, because those strings survive elsewhere in the document,
// including inside comments. Two files asserting one fact by two techniques that
// can disagree is not redundancy, it is a defect: whichever is weaker sets the
// real bar. So there is now one reader of that document, and both files use it.
//
// AND WHAT IS NOT ASKED, deliberately. Nothing here reads `continue-on-error:`,
// `if:` or the workflow's trigger list any more. Those were an attempt to
// establish that a declared job EXECUTES, which no reader of a file can do, and
// the list of ways a job fails to execute is not finite — tests/ci_workflow_test.go's
// header sets out that boundary in full and names where the question is answered
// instead. This file's subject is the wiring a document declares.
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

	wf := ciLoadWorkflow(t, ciWorkflowPath)
	mk := wiringReadFile(t, "Makefile")

	for _, mod := range mods {
		t.Run(mod, func(t *testing.T) {
			// CI: some job must declare a step that cds into the module and
			// runs its whole suite there. Read through the shared parser, so
			// a commented-out step is not a step and the near miss — a step
			// that enters the module and runs something else — is reported as
			// what it is rather than as "no job at all".
			found, nearly := ciSuiteJobsFor(wf, mod)
			if len(found) == 0 {
				detail := "No step declares `working-directory: " + mod + "` at all."
				if len(nearly) > 0 {
					detail = "Steps that enter the module without declaring a run of its suite:\n\t" + strings.Join(nearly, "\n\t")
				}
				t.Errorf("no job in %s declares a run of %q's test suite.\n"+
					"The root `go test ./...` does NOT descend into a nested module, so this\n"+
					"module's tests are declared to run on no machine but a maintainer's. Add a job with\n"+
					"    working-directory: %s\n"+
					"whose `run:` body is `go test -count=1 ./...`.\n%s", ciWorkflowPath, mod, mod, detail)
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

			// Lint: same reasoning as the two above, applied to the other
			// tool. `golangci-lint run ./...` at the repository root does not
			// descend into a nested module any more than `go test ./...` does,
			// and golangci-lint-action runs at the root — so a nested module's
			// Go code was linted by nothing, in CI or locally.
			//
			// That is not hypothetical. v0.5.0 shipped a browser test that
			// asserted on the BROWSER's request log rather than the page's, and
			// it lived in exactly this unlinted module. Lint would not have
			// caught that particular bug, but the point stands: the module was
			// carrying real logic that no linter had ever read.
			//
			// Checked PER PARSED STEP. The first version asked whether ci.yml
			// contained "working-directory: <mod>" and "golangci-lint"
			// anywhere, and it passed immediately against a workflow that lints
			// nothing but the root: the working-directory belonged to the
			// browser TEST job and the golangci-lint to a separate root lint
			// job. Two true facts about different jobs read as one true fact
			// about one job.
			//
			// The second version split the raw text on the literal "- name:"
			// and probed the chunks, which fixed that and left a smaller
			// version of it: comments do not survive a parse but they do
			// survive a text split, so commenting out the whole lint step with
			// a leading `#` on every line kept both substrings inside one chunk
			// and kept this green while no linter read the module anywhere.
			// A step that is not in a `steps:` list does not exist.
			//
			// The third version — the parse — then read the wrong KEY, and
			// was worse than either: it compared the step-level
			// `working-directory:`, which GitHub ignores on a `uses:` step
			// and which golangci-lint-action therefore takes as an INPUT
			// under `with:`. No valid workflow could make this branch true.
			// So the lint requirement had quietly narrowed to "a Makefile
			// target exists", and deleting the CI lint step outright left
			// this green — this file's own subject, committed by the check
			// itself. ciStepDirectory reads the key that carries the
			// directory for the kind of step it is.
			lintedInCI := false
			for _, jobName := range ciJobNames(wf) {
				for _, step := range wf.Jobs[jobName].Steps {
					if strings.Contains(step.Uses, "golangci-lint") && ciStepDirectory(step) == mod {
						lintedInCI = true
					}
				}
			}
			var lintTarget string
			for _, line := range strings.Split(mk, "\n") {
				if strings.Contains(line, "cd "+mod) && strings.Contains(line, "golangci-lint") {
					lintTarget = line
					break
				}
			}
			if !lintedInCI && lintTarget == "" {
				t.Errorf("nothing lints %q — not CI, not the Makefile.\n"+
					"`golangci-lint run ./...` at the root does not descend into a nested module,\n"+
					"so this module's Go code is read by no linter on any machine. Add a lint step\n"+
					"with `working-directory: %s` to .github/workflows/ci.yml, and a Makefile target\n"+
					"that runs `cd %s && golangci-lint run ./...`.", mod, mod, mod)
			}
		})
	}
}

// TestNestedModuleJobsCanActuallyFail IS GONE, and its absence is a decision.
//
// It made two whole-file substring assertions over ci.yml. The first — that the
// document nowhere says `continue-on-error` — was an attempt to establish that a
// declared job can fail on the runner, which is one member of an open-ended set
// (`if:`, `paths:`, a skipped `needs:`, `|| true` in a body, a runner label
// nothing matches) and could never be finished. The second — that the document
// somewhere says `DOSSIERX_TEST_BROWSER` — was satisfied by the literal
// appearing in ANY job or comment, and stayed green through the deletion of the
// entire viewer job. tests/ci_workflow_test.go now asks the second question of
// the job that declares the suite, which is where it means something; the first
// is not asked anywhere, and that file's header states the boundary and names
// where the run-time question is answered instead.
//
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

	// The three names above are the dependencies as they stand TODAY, and a list
	// of names ages. This is the same invariant stated so it cannot: the root
	// module must not name a nested module's own module path at all. A `require`
	// on viewer-tests pulls its entire graph — chromedp, cdproto, gobwas and
	// whatever they grow next — back into the engine's, which is the one thing the
	// split exists to prevent, and it does so without ever spelling any of those
	// three words in this file.
	//
	// WHAT USED TO BE HERE was a loop that re-stat'ed `<mod>/go.mod` for every
	// path wiringNestedModules had just returned — and it builds that list by
	// walking for files named `go.mod` and returning their parent directories. The
	// condition could only fire on a race between the two walks. It was a
	// tautology wearing the shape of a guard, which is worse than nothing: it
	// added a line to the count of things that are checked while checking nothing.
	for _, mod := range wiringNestedModules(t) {
		modPath := wiringModulePath(t, mod)
		if strings.Contains(rootMod, modPath) {
			t.Errorf("the root go.mod names %q, the module path of nested module %s.\n"+
				"That module exists precisely so its dependency graph is not the engine's:\n"+
				"requiring or replacing it here pulls chromedp and everything under it back\n"+
				"into cobra + yaml.v3, and no `grep chromedp go.mod` would show it.", modPath, mod)
		}
	}
}

// wiringModulePath reads a module's declared path out of its go.mod. A go.mod
// without a `module` line is a t.Fatal and not a shrug: the check above compares
// against that path, and comparing against "" would pass over everything.
func wiringModulePath(t *testing.T, mod string) string {
	t.Helper()
	for _, line := range strings.Split(wiringReadFile(t, mod+"/go.mod"), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			if path := strings.TrimSpace(rest); path != "" {
				return path
			}
		}
	}
	t.Fatalf("%s/go.mod declares no `module` line, so this file cannot say what path the root module would have to name to break the isolation", mod)
	return ""
}
