// WHAT THIS FILE ESTABLISHES, AND WHAT IT CANNOT. Read this before adding an
// assertion to it, because the boundary is the point of the file.
//
// IT ESTABLISHES FACTS ABOUT TWO DOCUMENTS: .github/workflows/ci.yml and
// .github/workflows/deploy-site.yml, unmarshalled as YAML and read from a
// package that is not part of either. Every assertion here has the form "this
// workflow DECLARES x", every one of them names the declaration it reads, and
// every one of them goes red when that declaration is deleted or changed. Four
// declarations are read and nothing else:
//
//   - exactly one job declares a step that enters viewer-tests/ and runs
//     `go test ./...` there — as the body's ONE command, with every argument
//     drawn from a closed vocabulary, under a declared environment whose
//     variable NAMES are drawn from another one;
//   - every actions/setup-node step in THAT job pins the same node-version;
//   - that pin equals the one deploy-site.yml publishes the site with;
//   - that job's steps declare DOSSIERX_TEST_BROWSER, as an `env:` key or as an
//     assignment token in a parsed command.
//
// IT CANNOT ESTABLISH THAT CI RAN THE SUITE, and nothing that reads a file can.
// This is not a gap to be closed by reading more keys. That was tried: an
// earlier version of this file also parsed `if:`, `continue-on-error:` and the
// workflow's `on:` trigger list, on the reasoning that a job which parses is not
// a job which runs. The reasoning is correct and the remedy was the same mistake
// one level up, because the list of ways a declared job fails to execute is not
// finite:
//
//	if: ${{ github.event_name == 'push' }}   cannot be evaluated statically
//	paths: / paths-ignore:                   depends on the diff, not the file
//	needs: a job that skipped                skips its dependents
//	runs-on: a label no runner carries       queues forever
//
// Each new technique is one more line in ci.yml and one more special case here,
// and a check whose completeness argument is "we thought of the ones we thought
// of" reports "we did not check" and reads as "it is fine". CLAUDE.md forbids
// exactly that. So those keys are read NOWHERE in this repository now, on
// purpose, and the boundary is stated here instead of being pretended away.
//
// `|| true` INSIDE A RUN BODY USED TO BE ON THAT LIST AND DOES NOT BELONG ON IT.
// Nothing about it is decided by the runner: it is decided by the bytes on disk,
// and this file already tokenises `||`, `&&`, `;` and `|` when it splits a body
// into commands — it saw the suppression and threw it away. Calling a question
// unanswerable to avoid answering it is the same defect this file exists to
// remove, one level down, and it cost exactly what you would expect:
// `run: go test -count=1 ./... || true` left every assertion here green, as did
// a body of `set +e` / `go test -count=1 ./...` / `exit 0`.
//
// WHAT CLOSES IT IS NOT A CASE PER TRICK. `|| true`, `set +e`, a trailing
// `exit 0`, a pipeline that eats the status — enumerating those is the mistake
// again in miniature. The rule is the same inversion used on the arguments: the
// suite step's body is ONE command, and that command is the invocation. A second
// command in that body is REFUSED rather than interpreted, because deciding what
// it does to the step's exit status means modelling a shell, and this file does
// not model a shell. Setup goes in its own step, where its own failure is its own.
//
// WHERE THE OTHER QUESTION IS ANSWERED, and this paragraph has been rewritten
// because the answer changed. It is answered from the RUN, in
// tests/ci_run_evidence_test.go, which fetches the CI run for one commit,
// derives from this same workflow which suites exist and in how many matrix
// instantiations, fetches each instantiation's job log, and parses the
// `go test -json` account the suite steps now emit. That file's subject is the
// run and this file's subject is the document; neither can do the other's job,
// and the boundary drawn above is unchanged — it is a boundary on what a reader
// of a FILE can establish, and nothing here has started reading a run.
//
// TWO AUTOMATED READERS EXIST AND THEY ANSWER DIFFERENT QUESTIONS.
// .claude/workflows/release-checklist.js carries a pre-tag check keyed
// `ci-on-merge-commit` that runs
// `gh api repos/<owner>/<repo>/commits/<merge-sha>/check-runs` and requires every
// check on the merge commit to be a pass, counting a pending one as
// COULD_NOT_RUN. That reader closes the two cases this file cannot see from
// disk: a job that was skipped, and a workflow that never fired at all — a
// commit with no check runs is not "every check passed". What it cannot close is
// a check run whose conclusion is success over nothing, because a conclusion is
// all it reads. Its sibling check `ci-run-evidence` closes that half by reading
// the account the test binary itself emitted: a suite step that printed
// `[no tests to run]` beside every `ok` is a package that passed having executed
// no test, and a step marked continue-on-error that failed and was forgiven
// leaves a `"Action":"fail"` event that no forgiveness above the binary can
// remove. Neither reader is a substitute for the other and both are required.
//
// THE HUMAN ITEM SURVIVES ANYWAY, and for a reason that has also changed. It used
// to survive because it was the only thing that could see a suite that ran
// nothing. It survives now because the machine reads only what it derives — the
// suites this workflow declares — and the `hooks` job runs a shell script, which
// emits no countable account of anything.
//
// WHY THE JOB IS FOUND BY WHAT ITS STEPS DO, NOT BY NAME. The invariant used to
// be a `run:` guard step inside ci.yml, in the very job it protected: it read the
// workflow off disk, sliced out the job containing its own step name, and grepped
// the slice. Moving the pin AND the guard together into another job left the
// guard slicing out that job, finding the pin it had moved with, and exiting 0. A
// guard that travels with the thing it guards is a guard the thing can take with
// it. Naming the job here would reintroduce that with an extra step: rename the
// job, or move the suite into a differently-named one, and a name-keyed lookup
// either fails for the wrong reason or silently reads somebody else's job.
//
// FOUR DEFEATS OF THE OLD ARRANGEMENTS, AND HOW STRUCTURE CLOSES THEM:
//
//   - a PROSE COMMENT satisfying a search: comments do not survive a parse.
//   - a COMMENTED-OUT step: likewise. A step not in the `steps` list does not exist.
//   - a DUPLICATE pin added later: every setup-node step in the identified job is
//     collected and they must AGREE, so a second one is a failure, not a first match.
//   - RELOCATION: only the identified job's own step list is read. A setup-node in
//     another job is another runner with another Node and is invisible here.
//
// AND WHAT "RUNS THE SUITE" MEANS, because that was a correction too. It used to
// mean a step whose `working-directory` was the module and whose `run` was not
// empty — which is a step that ENTERS the directory, not one that runs anything
// in it. Replacing `run: go test -count=1 ./...` with `run: echo 'nope'` left every
// assertion green. So the body is PARSED — split into commands and tokenised into
// argv the way a shell would — and one command must be a `go test` over `./...`.
// `echo "go test ./..."` is an echo with one argument.
//
// THE ARGUMENT VOCABULARY IS CLOSED, and that is the other correction. Requiring
// `./...` and refusing a narrowed package pattern by name refuses ONE narrowing
// channel: `go test -count=1 -run '^$' ./...` still names `./...`, runs zero
// tests, and is green. `-skip '.'` and `-tags` do the same. Adding a special case
// per flag is the unbounded enumeration this file exists to stop doing, so the
// rule is inverted: every argument of that `go test` must appear in
// ciAllowedTestFlags or ciAllowedTestFlagPrefixes below, and anything else is a
// failure that says "add it here, on purpose, with a reason".
//
// AND THE ENVIRONMENT IS A VOCABULARY TOO, which the first version of that rule
// missed. `go test` takes selectors from its environment as readily as from its
// argv: `GOFLAGS: -run=^$` runs zero tests in every package and prints
// `ok <pkg> 0.001s [no tests to run]`, a green job over nothing, and `-tags` does
// it a second way. Leading `VAR=value` assignments in the command were refused
// from the start — but a step's own `env:` mapping does the identical thing in a
// key nothing was reading, and the failure message for the refused assignment
// used to direct maintainers to exactly that unread place. So the `env:` mappings
// that reach the suite step (the workflow's, the job's and the step's) are read
// as well, and every variable NAME in them must appear in ciAllowedSuiteEnv
// below. That is not a new model of anything: it is one more closed vocabulary
// over the keys of a map this file already parses.
//
// WHAT THE ENVIRONMENT RULE DOES NOT COVER, stated rather than implied: a
// variable an EARLIER STEP exports at run time by appending to `$GITHUB_ENV`.
// That is a redirect inside a shell script — the same construct
// TestTheViewerJobDeclaresTheBrowserVariable declines to model below, for the
// same reason — and it appears in no `env:` mapping for this file to read. The
// declared environment is checked; a run-time export is not, and the check says
// so instead of counting it as covered.
//
// EVERY FAILURE HERE IS LOUD. A workflow that will not parse, a module directory
// that is not a module, no job that runs the suite, two jobs that do — each is
// "this check could not run", and each is a t.Fatal. There is no path through
// this file that reports having checked something it did not read.
package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The two workflows, repo-relative. deploy-site.yml is the source of truth for
// the Node major: it is the build a visitor's page is actually produced by, and
// everything else is matched to it.
const (
	ciWorkflowPath     = ".github/workflows/ci.yml"
	deployWorkflowPath = ".github/workflows/deploy-site.yml"
)

// ciViewerModule is the nested module whose suite reads the built site as
// rendered DOM. It is a DIRECTORY, checked to hold a go.mod below, and not a job
// name or a label — the Node assertion is about the runner that builds site/,
// and the only durable handle on that runner is the module it runs.
//
// It is a constant HERE because the Node invariant is specifically about this
// module. The generic "every nested module is wired into CI" question is not
// keyed to it: tests/nested_module_coverage_test.go walks the tree for go.mod
// files and asks the same parsed question about each one it finds, through the
// same helpers below, so the two files cannot disagree about what wiring means.
const ciViewerModule = "viewer-tests"

// setupNodeAction is the action that installs Node. Matched as a `uses:` VALUE
// on a parsed step, so the prose in either workflow that names it cannot be
// mistaken for a step that runs it.
const setupNodeAction = "actions/setup-node@"

// nodeVersionKey is the `with:` key that carries the pin.
const nodeVersionKey = "node-version"

// ciStep is one step of one job. Only the keys this file reasons about are
// declared; `with` and `env` are `any`-valued because a workflow's scalars are a
// mix of strings, integers and booleans (`node-version: '20'`, `fetch-depth: 0`,
// `cache: true`) and a stricter type would make this file fail to parse a
// perfectly valid workflow.
//
// `if` and `continue-on-error` are NOT declared, and their absence is a decision
// rather than an oversight — see the header. Reading them was an attempt to
// establish that a declared job executes, which no reader of this document can
// do, and stopping at those two keys made the incompleteness look like coverage.
type ciStep struct {
	Name             string         `yaml:"name"`
	Uses             string         `yaml:"uses"`
	Run              string         `yaml:"run"`
	WorkingDirectory string         `yaml:"working-directory"`
	With             map[string]any `yaml:"with"`
	Env              map[string]any `yaml:"env"`
}

// A job and a workflow each carry an `env:` mapping of their own, and both reach
// the suite step — GitHub merges workflow, job and step mappings in that order.
// They are declared here because a selector in any of the three narrows the run
// identically; reading only the innermost one would be a closed vocabulary with
// two doors left open beside it.
type ciJob struct {
	Name  string         `yaml:"name"`
	Env   map[string]any `yaml:"env"`
	Steps []ciStep       `yaml:"steps"`
}

type ciWorkflow struct {
	Env  map[string]any   `yaml:"env"`
	Jobs map[string]ciJob `yaml:"jobs"`
}

// ciEnvScope is one `env:` mapping together with a phrase naming where the
// document declares it, so a refusal can tell a maintainer which of the three
// mappings to go and edit rather than just which variable offended.
type ciEnvScope struct {
	where string
	env   map[string]any
}

// ciAllowedSuiteEnv is the CLOSED vocabulary of variable names the suite step may
// run under, and it is an allowlist for the same reason ciAllowedTestFlags is.
//
// `go test` reads selectors out of its environment: `GOFLAGS: -run=^$` runs zero
// tests over the full package pattern, `GOFLAGS: -tags=none` can empty the suite
// a second way, and both leave a green job whose log says `ok` for every package.
// A denylist naming GOFLAGS would refuse the one somebody thought of today.
//
// Nothing here can change WHICH tests run: two of them name a tool path the suite
// resolves (a browser, a GoReleaser binary) and the third is the version pin
// ci.yml declares at the top of the file for every job. If you are about to add
// GOFLAGS, GOOS, GOARCH or CGO_ENABLED, stop — those select, and the whole point
// of the list is that adding them is not something you can do by accident.
var ciAllowedSuiteEnv = map[string]bool{
	"DOSSIERX_TEST_BROWSER":       true,
	"DOSSIERX_TEST_GORELEASER":    true,
	"DOSSIERX_GORELEASER_VERSION": true,
}

// ciSuiteEnvRefusals lists every declared variable outside that vocabulary, each
// tagged with the mapping that declares it. Sorted, so a failure reads the same
// on every run.
func ciSuiteEnvRefusals(scopes []ciEnvScope) []string {
	var out []string
	for _, scope := range scopes {
		names := make([]string, 0, len(scope.env))
		for name := range scope.env {
			if !ciAllowedSuiteEnv[name] {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			out = append(out, fmt.Sprintf("%s declares `%s`", scope.where, name))
		}
	}
	return out
}

// ciRepoRoot locates the repository root from THIS source file, so the reads
// below are unaffected by how `go test` was invoked.
func ciRepoRoot(t *testing.T) string {
	t.Helper()
	return wiringRepoRoot(t)
}

// ciLoadWorkflow parses a workflow, or fails saying so.
//
// A parse error is a t.Fatal and not a skip: this file's whole subject is the
// content of these two documents, and a document it could not read is a document
// it has not checked. It also refuses an EMPTY job map, which is what a file that
// parsed into the wrong shape looks like from here — every assertion below would
// pass over zero jobs.
func ciLoadWorkflow(t *testing.T, rel string) ciWorkflow {
	t.Helper()
	path := filepath.Join(ciRepoRoot(t), filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v\nThis check is about what that workflow declares; without the file there is nothing to declare and nothing to check", rel, err)
	}
	var wf ciWorkflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		t.Fatalf("parse %s as YAML: %v\nEvery assertion here reads this file as a workflow rather than as text — that is the whole reason this check exists — so a document that will not parse is a FAILED check, never a quiet one", rel, err)
	}
	if len(wf.Jobs) == 0 {
		t.Fatalf("%s parsed, but declares no jobs. Either the top-level `jobs:` key was renamed or this file's model of a workflow no longer matches the document; both leave every assertion below passing over nothing", rel)
	}
	return wf
}

// ciJobNames is a workflow's job keys in order, so a failure message reads the
// same on every run — `jobs` is a map, and Go's iteration order is not.
func ciJobNames(wf ciWorkflow) []string {
	out := make([]string, 0, len(wf.Jobs))
	for name := range wf.Jobs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ciScalar renders a `with:`/`env:` value as the string a runner would see.
// `node-version: '20'` parses as a string and `node-version: 20` as an integer;
// they configure the same runner, and a check that told them apart would be
// asserting a quoting style.
func ciScalar(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// ciNodePins returns every Node version pinned by a step in the given list,
// alongside how many setup-node steps were seen at all.
//
// The two numbers are returned separately because they name different defects: a
// setup-node step with no `node-version` is an UNPINNED setup — it installs
// whatever the action defaults to — and reporting that as "no pin found" would
// send a maintainer looking for a step that is right there.
func ciNodePins(steps []ciStep) (pins []string, setups int) {
	for _, step := range steps {
		if !strings.Contains(step.Uses, setupNodeAction) {
			continue
		}
		setups++
		if pin := ciScalar(step.With[nodeVersionKey]); pin != "" {
			pins = append(pins, pin)
		}
	}
	return pins, setups
}

// ciOnePin collapses a step list's pins to the single version it names, or
// explains which of the three ways it failed to.
func ciOnePin(steps []ciStep, where string) (string, error) {
	pins, setups := ciNodePins(steps)
	switch {
	case setups == 0:
		return "", fmt.Errorf("%s declares no %s step", where, strings.TrimSuffix(setupNodeAction, "@"))
	case len(pins) == 0:
		return "", fmt.Errorf("%s declares %d %s step(s), none of which sets `%s`. An unpinned setup installs whatever the action defaults to, which moves on its own schedule under nobody's review",
			where, setups, strings.TrimSuffix(setupNodeAction, "@"), nodeVersionKey)
	}
	for _, pin := range pins[1:] {
		if pin != pins[0] {
			return "", fmt.Errorf("%s pins more than one Node version (%v). Which one is in force is then a question about step order rather than about the workflow, and a later step silently wins over an earlier one",
				where, pins)
		}
	}
	if strings.Contains(pins[0], "${{") {
		return "", fmt.Errorf("%s pins Node to the expression %q, which resolves at run time. A pin nothing can read before the job starts is not a pin this check can hold the publish build to",
			where, pins[0])
	}
	return pins[0], nil
}

// ciArgv splits one command line into arguments the way a runner's shell would
// for the forms a workflow uses: runs of whitespace separate, single and double
// quotes group.
//
// This is a PARSE and not a search. `echo 'go test ./...'` tokenises to an `echo`
// carrying one argument; `strings.Contains(step.Run, "go test")` cannot tell it
// from the command. It is deliberately not a shell — no expansion, no escapes
// beyond quoting, no redirects — because a `run:` body needing any of that would
// be a suite this file could not read, and the checks below say what they read
// rather than guessing at what the runner would do with the rest.
func ciArgv(line string) []string {
	var (
		out   []string
		cur   strings.Builder
		open  bool
		quote byte
	)
	flush := func() {
		if open || cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			cur.WriteByte(c)
		case c == '\'' || c == '"':
			quote, open = c, true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
			open = false
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}

// ciCommands splits a `run:` body into the commands it executes, as argv lists.
//
// Comments are dropped and the operators that end a command — `&&`, `||`, `;`,
// `|` — split it, so a body's every command is examined rather than its first.
// A `#`-prefixed line yields nothing at all, which is the point: a commented-out
// invocation is not an invocation, and that is one of the four ways the guards
// this file replaces were defeated.
func ciCommands(body string) [][]string {
	var out [][]string
	for _, line := range strings.Split(body, "\n") {
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			if hash == 0 || line[hash-1] == ' ' || line[hash-1] == '\t' {
				line = line[:hash]
			}
		}
		parts := []string{line}
		for _, op := range []string{"&&", "||", ";", "|"} {
			var next []string
			for _, part := range parts {
				next = append(next, strings.Split(part, op)...)
			}
			parts = next
		}
		for _, part := range parts {
			if argv := ciArgv(part); len(argv) > 0 {
				out = append(out, argv)
			}
		}
	}
	return out
}

// ciAllPackages is the package pattern a suite has to be run over. Written out
// because the alternative to requiring it is accepting a narrowed selector, and
// a job running `go test ./somepackage` is a job reporting success over the
// packages it stopped naming.
const ciAllPackages = "./..."

// ciAllowedTestFlags and ciAllowedTestFlagPrefixes are the CLOSED vocabulary a
// suite invocation may be spelled with, exact tokens and `-flag=` prefixes
// respectively.
//
// An allowlist rather than a denylist, and that inversion is the whole design.
// `go test` narrows what it runs through several independent channels — `-run`,
// `-skip`, `-tags`, a narrowed package pattern — and a check that refuses them by
// name refuses the ones somebody remembered on the day they wrote it. Refusing
// everything not listed here means the next narrowing flag fails closed on
// arrival, and adding a genuinely harmless one is a deliberate edit to this list
// with a reason attached, which is what "never narrow coverage silently" asks for.
//
// Nothing here changes WHICH tests run. `-count=1` disables the result cache,
// `-v` and `-json` change reporting, `-race` and `-timeout`/`-shuffle`/`-parallel`
// change how they are executed. If you are about to add one that selects, stop:
// that is the edit this list exists to make visible.
var ciAllowedTestFlags = map[string]bool{
	"-count=1": true,
	"-v":       true,
	"-json":    true,
	"-race":    true,
}

var ciAllowedTestFlagPrefixes = []string{"-timeout=", "-shuffle=", "-parallel=", "-p="}

// ciTestArgAllowed reports whether one argument of a `go test` invocation is in
// the vocabulary above (the package pattern itself included).
func ciTestArgAllowed(arg string) bool {
	if arg == ciAllPackages || ciAllowedTestFlags[arg] {
		return true
	}
	for _, prefix := range ciAllowedTestFlagPrefixes {
		if strings.HasPrefix(arg, prefix) && len(arg) > len(prefix) {
			return true
		}
	}
	return false
}

// ciRunsModuleSuite reports whether a step declares a run of the given nested
// module's whole test suite, and says why not when it does not. `outer` is the
// `env:` mappings that reach the step from above it — the workflow's and the
// job's.
//
// Five claims, all about the declaration, all load-bearing:
//
//   - the step RUNS something rather than uses an action. ci.yml's lint step
//     enters this very module, and takes the directory as an INPUT to
//     golangci-lint-action under `with:`, which is where an action takes one.
//     Linting a module is not testing it, so an action step is not a candidate
//     here and is not reported as a near miss either.
//   - `working-directory` is the runner ENTERING the module. The root
//     `go test ./...` does not descend into a nested module, so a step that never
//     changes directory declares nothing about this suite.
//   - the body is ONE command, and that is what closes the suppression channel
//     rather than a case per trick. `go test ./... || true`, a `set +e` above and
//     an `exit 0` below, a pipe into `tee` — each turns a red suite into a green
//     step, each is plainly visible in the bytes, and each stops being expressible
//     when the body may hold exactly one command. A second command is refused, not
//     interpreted: interpreting it means modelling a shell.
//   - that command is a parsed `go test` over `./...`, spelled out of the closed
//     argument vocabulary above and carrying no leading `VAR=value` assignments.
//     Entering a directory and doing something there is not running a suite, and
//     the difference is one `echo`; `-run '^$'` names `./...` and runs nothing.
//   - the environment declared for it is drawn from ciAllowedSuiteEnv. A selector
//     travels in `env:` as easily as in argv, and the mapping is right there in
//     the document to be read.
func ciRunsModuleSuite(step ciStep, mod string, outer []ciEnvScope) (runs bool, why string) {
	if step.Uses != "" {
		return false, ""
	}
	if step.WorkingDirectory != mod {
		return false, ""
	}
	body := strings.TrimSpace(step.Run)
	if body == "" {
		return false, "declares no `run:` body"
	}

	cmds := ciCommands(step.Run)
	if len(cmds) == 0 {
		return false, "declares a `run:` body that parses to no commands at all — every line of it is a comment"
	}
	if len(cmds) > 1 {
		lines := make([]string, 0, len(cmds))
		for _, argv := range cmds {
			lines = append(lines, strings.Join(argv, " "))
		}
		return false, fmt.Sprintf("enters %s with a body of %d commands (%v), and the suite step's body must be ONE. A second command is where a failure goes to be swallowed — `go test ./... || true`, a `set +e` above it, an `exit 0` below it, a pipe into `tee` — and telling a harmless neighbour from a suppressing one means modelling a shell's exit status, which this file will not do and will not pretend to have done. Put the setup in its own step, where its own failure is its own",
			mod, len(cmds), lines)
	}

	argv := cmds[0]
	line := strings.Join(argv, " ")
	name, rest, assigns, ok := ciCommandName(argv)
	if !ok {
		return false, fmt.Sprintf("enters %s and its body is `%s`, which sets variables and runs no program", mod, line)
	}
	if name != "go" || len(rest) < 1 || rest[0] != "test" {
		return false, fmt.Sprintf("enters %s and runs `%s`, which is not a `go test %s`", mod, line, ciAllPackages)
	}
	if len(assigns) > 0 {
		return false, fmt.Sprintf("runs `%s`, whose `go test` carries the leading assignment(s) %v. A prefixed invocation is refused rather than half-read: `GOFLAGS=-run=^$` narrows the run to nothing from in front of the command. Declare what the suite needs in the step's `env:` mapping, whose names are checked against ciAllowedSuiteEnv in tests/ci_workflow_test.go — it is read, unlike a prefix",
			line, assigns)
	}
	var unknown []string
	pattern := false
	for _, arg := range rest[1:] {
		if arg == ciAllPackages {
			pattern = true
			continue
		}
		if !ciTestArgAllowed(arg) {
			unknown = append(unknown, arg)
		}
	}
	if !pattern {
		return false, fmt.Sprintf("runs `%s`, which names no `%s` — a narrowed package pattern reports success over the packages it stopped naming", line, ciAllPackages)
	}
	if len(unknown) > 0 {
		return false, fmt.Sprintf("runs `%s`, whose argument(s) %v are not in this file's allowed `go test` vocabulary. Naming `%s` is not enough on its own: `-run '^$'`, `-skip '.'` and `-tags` all leave the package pattern in place and run nothing or nearly nothing. If %v genuinely does not select which tests run, add it to ciAllowedTestFlags in tests/ci_workflow_test.go and say why there",
			line, unknown, ciAllPackages, unknown)
	}

	scopes := append(append([]ciEnvScope{}, outer...), ciEnvScope{where: "the step's own `env:`", env: step.Env})
	if refused := ciSuiteEnvRefusals(scopes); len(refused) > 0 {
		return false, fmt.Sprintf("runs `%s`, but under an environment this file cannot read as harmless: %s. `go test` takes selectors from its environment as well as its argv — `GOFLAGS: -run=^$` runs zero tests in every package and prints `ok <pkg> [no tests to run]`, which is a green job over nothing that no reader of a run log is likely to challenge. So the variable NAMES declared for this step are an allowlist for the same reason its arguments are. If one of these cannot change which tests run, add it to ciAllowedSuiteEnv in tests/ci_workflow_test.go and say why there; GOFLAGS, GOOS, GOARCH and CGO_ENABLED can, and never belong in it",
			line, strings.Join(refused, "; "))
	}
	return true, ""
}

// ciStepDirectory reports the directory a step declares it operates in, reading
// the key that carries it for the KIND of step it is.
//
// A `run:` step carries it in the step-level `working-directory:` key. A `uses:`
// step does not: GitHub ignores a step-level `working-directory` on an action
// step, and an action that works in a subdirectory takes one as an INPUT under
// `with:` — golangci-lint-action does exactly that. Reading only the step-level
// key therefore made the lint arm of tests/nested_module_coverage_test.go's guard
// unsatisfiable by any valid workflow: it could never be true, so the lint
// requirement had silently narrowed to "a Makefile target exists", and deleting
// the CI lint step changed nothing. A guard that no correct configuration can
// satisfy is not strict, it is inert.
func ciStepDirectory(step ciStep) string {
	if step.Uses != "" {
		return ciScalar(step.With["working-directory"])
	}
	return step.WorkingDirectory
}

// ciCommandName resolves the program a parsed command runs, returning the leading
// `VAR=value` assignments POSIX allows in front of one separately and reporting
// `false` for a line that is nothing but assignments.
//
// All three parts earn their place. `DOSSIERX_TEST_GORELEASER=x go test ./...`
// runs the suite and a check reading argv[0] would call the program
// "DOSSIERX_TEST_GORELEASER=x"; `g="$(go env GOPATH)/bin/goreleaser"` is a
// variable being set, whose single token ends in `/goreleaser` and which the
// sibling check in cmd/dossierx did briefly read as an invocation of the release
// tool; and the assignments are RETURNED rather than discarded because a prefix
// is not always harmless — the caller above refuses them on a `go test`.
func ciCommandName(argv []string) (name string, args []string, assigns []string, ok bool) {
	for i, arg := range argv {
		if eq := strings.IndexByte(arg, '='); eq > 0 && ciIsIdentifier(arg[:eq]) {
			assigns = append(assigns, arg)
			continue
		}
		name = arg
		if slash := strings.LastIndexAny(name, "/\\"); slash >= 0 {
			name = name[slash+1:]
		}
		return strings.TrimSuffix(name, ".exe"), argv[i+1:], assigns, true
	}
	return "", nil, assigns, false
}

// ciIsIdentifier reports whether s is a shell variable name, which is what tells
// `FOO=bar cmd` from a path that happens to contain an `=`.
func ciIsIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// ciSuiteJobsFor returns the job keys that declare a run of the given module's
// suite, and — separately — the near misses: steps that enter the module and do
// something else.
//
// THE NEAR MISS IS REPORTED SEPARATELY, and that is what makes the zero case
// readable. A step that enters the module and runs something else is not "no job
// runs the suite" to a maintainer looking at a workflow that plainly has a viewer
// job; it is "that step stopped running the suite", and the message can then say
// which step and what it declares instead.
func ciSuiteJobsFor(wf ciWorkflow, mod string) (found, nearly []string) {
	for _, name := range ciJobNames(wf) {
		// The two `env:` mappings the document declares ABOVE the step. Both
		// reach it, so both are read: a `GOFLAGS` at the top of the workflow
		// narrows this suite exactly as thoroughly as one on the step.
		outer := []ciEnvScope{
			{where: "the workflow's top-level `env:`", env: wf.Env},
			{where: fmt.Sprintf("job `%s`'s `env:`", name), env: wf.Jobs[name].Env},
		}
		runs := false
		for _, step := range wf.Jobs[name].Steps {
			ok, why := ciRunsModuleSuite(step, mod, outer)
			if ok {
				runs = true
				break
			}
			if why != "" {
				nearly = append(nearly, fmt.Sprintf("job %s, step %q: %s", name, step.Name, why))
			}
		}
		if runs {
			found = append(found, name)
		}
	}
	return found, nearly
}

// ciViewerSuiteJob finds THE job that declares a run of the viewer module, and
// returns the job key and the job.
//
// Anything other than exactly one such job is a t.Fatal. Zero means the suite is
// declared by no job — the deletion case, and the one an earlier arrangement of
// this invariant could not see at all, because the guard was inside the job that
// was deleted. Two means this file cannot say which runner's Node it is talking
// about, which is not a thing to resolve by picking the first.
func ciViewerSuiteJob(t *testing.T, wf ciWorkflow) (string, ciJob) {
	t.Helper()

	// The module has to BE a module. Without this the identifier below is just a
	// string that both this file and the workflow happen to spell the same way,
	// and a renamed directory would leave this check hunting for a job that
	// correctly no longer exists.
	modPath := filepath.Join(ciRepoRoot(t), filepath.FromSlash(ciViewerModule), "go.mod")
	if _, err := os.Stat(modPath); err != nil {
		t.Fatalf("%s is not a Go module (%v). This file identifies the job that runs the browser suite by the step whose `working-directory` is that directory; "+
			"if the module moved, move this constant with it — do not leave the identifier pointing at nothing, which would make every assertion below unreachable", ciViewerModule, err)
	}

	found, nearly := ciSuiteJobsFor(wf, ciViewerModule)
	switch len(found) {
	case 1:
		return found[0], wf.Jobs[found[0]]
	case 0:
		detail := "No step comes close: none declares `working-directory: " + ciViewerModule + "` at all."
		if len(nearly) > 0 {
			detail = "Steps that enter the module without declaring a run of its suite:\n\t" + strings.Join(nearly, "\n\t")
		}
		t.Fatalf("no job in %s declares a run of %s's test suite: no `run:` step carries `working-directory: %s` AND a single-command body that is a `go test %s` spelled out of this file's allowed argument vocabulary, under an environment drawn from its allowed variable names. Jobs declared: %v.\n%s\n"+
			"The root `go test ./...` does not descend into a nested module, so in this state the browser suite — which reads the built site as rendered DOM, and which also runs the GoReleaser dry run — is declared to run on no machine but a maintainer's, and the Node that builds site/ in CI is pinned by nothing.\n"+
			"The body is PARSED rather than searched, so `echo \"go test ./...\"` is an echo: an earlier version of this check accepted any non-empty `run`, and replacing the suite with an echo left every assertion in this file green.\n"+
			"This is reported as a FAILURE and not as \"nothing to check\": a check that cannot locate its subject has not passed over it",
			ciWorkflowPath, ciViewerModule, ciViewerModule, ciAllPackages, ciJobNames(wf), detail)
	default:
		t.Fatalf("%d jobs in %s declare a run of %s (%v). Each is a separate runner with its own Node, and this check cannot say which of them builds the site the DOM assertions read. "+
			"Either fold them into one job or give this file a rule for choosing; picking the first would be an answer nobody decided", len(found), ciWorkflowPath, ciViewerModule, found)
	}
	return "", ciJob{}
}

// TestTheViewerJobsNodePinAgreesWithThePublishWorkflow is a claim about two
// documents and is named for exactly that.
//
// IT DOES NOT CLAIM THE JOB BUILDS site/ WITH THAT NODE. It used to be named as
// though it did, and the name was a lie a parse cannot support: move the
// actions/setup-node step BELOW the suite step in ci.yml and both pins still
// agree, this test is still green, and the npm build inside the suite has
// already run on the runner image's default Node. Step order, and whether the
// step ran at all, are facts about a run — see the header for where those are
// answered. What is checked here is that ci.yml's viewer job and
// deploy-site.yml name the SAME Node major, which is a fact about the files and
// goes red the moment either pin moves.
//
// WHY THAT AGREEMENT IS WORTH PINNING. The DOM that viewer-tests/site_dom_test.go
// reads is evidence about the page a visitor is served only for as long as it was
// produced by the toolchain that serves it, and the drift is silent in the
// direction that matters: a newer Node that BROKE vite or tsc fails the build
// loudly and gets noticed, while one that merely emits a slightly different tree
// does not fail at all.
func TestTheViewerJobsNodePinAgreesWithThePublishWorkflow(t *testing.T) {
	publish := ciLoadWorkflow(t, deployWorkflowPath)
	ci := ciLoadWorkflow(t, ciWorkflowPath)

	// The publish pin, read across the WHOLE publish workflow. Unlike ci.yml this
	// is a workflow whose only purpose is to build and publish site/, so every
	// setup-node in it is a setup-node on the publish path and they must agree.
	var publishSteps []ciStep
	for _, name := range ciJobNames(publish) {
		publishSteps = append(publishSteps, publish.Jobs[name].Steps...)
	}
	publishPin, err := ciOnePin(publishSteps, deployWorkflowPath)
	if err != nil {
		t.Fatalf("%v.\nThat workflow is the build a visitor's page actually comes from, so it is the source of truth here: until it names one Node version there is nothing for the suite's own job to match against",
			err)
	}

	jobKey, job := ciViewerSuiteJob(t, ci)
	t.Logf("the job declaring a run of %s is %s (name: %q); %s publishes with Node %s", ciViewerModule, jobKey, job.Name, deployWorkflowPath, publishPin)

	suitePin, err := ciOnePin(job.Steps, fmt.Sprintf("%s's `%s` job — the one that declares a run of %s", ciWorkflowPath, jobKey, ciViewerModule))
	if err != nil {
		t.Fatalf("%v.\nThat job's suite builds site/ with npm and reads the result as rendered DOM. With no pin declared, nothing in this repository names the Node it would be built under, and every assertion in the browser suite can stay green while describing a page no visitor is served.\n"+
			"The step is looked for in THAT JOB and nowhere else in %s, on purpose: a setup-node in another job is another runner with another Node and says nothing about this one. Add the step there, pinned to %s.",
			err, ciWorkflowPath, publishPin)
	}

	if suitePin != publishPin {
		t.Fatalf("%s's `%s` job pins Node %s, but %s publishes the site with Node %s.\n"+
			"The DOM that job's suite reads is evidence about the page a visitor gets only while the two toolchains are the same one. Move the job's pin to %s — a pin elsewhere in %s belongs to a different runner and is not read here.",
			ciWorkflowPath, jobKey, suitePin, deployWorkflowPath, publishPin, publishPin, ciWorkflowPath)
	}
}

// browserPathVariable is the knob the browser suite reads to locate Chrome.
const browserPathVariable = "DOSSIERX_TEST_BROWSER"

// TestTheViewerJobDeclaresTheBrowserVariable is, again, named for what it checks.
//
// IT DOES NOT CLAIM THE VARIABLE REACHES THE SUITE. The check accepts an `env:`
// mapping key, which does reach it, and it accepts a `NAME=` token in any parsed
// command, which may not: delete the `>>"$GITHUB_ENV"` redirect from the Chrome
// resolver in ci.yml and the token is still an argument to `echo`, this test is
// still green, and nothing is exported. Closing that would mean modelling
// redirects, $GITHUB_ENV semantics, `docker -e`, per-command env prefixes — one
// more shell feature per attack, which is the enumeration this file's header
// refuses. So the claim stops where the parse does: the job DECLARES the
// variable, somewhere in its own steps.
//
// WHY EVEN THE WEAK CLAIM IS WORTH KEEPING. It goes red when the declaration is
// deleted, and that is what actually happened: the predecessor of this check
// asserted the literal appeared SOMEWHERE in ci.yml, so two mentions in two jobs
// read as one fact about the runner that drives the browser, and the entire
// viewer job could be — and was — deleted with the assertion still green because
// another job said the word. Scoping it to the job that declares the suite is the
// whole of the improvement, and it is a real one.
//
// AND IT IS PARSED, not searched, which is a separate correction. Its first
// version asked `strings.Contains(step.Run, browserPathVariable)`, satisfied by
// the name appearing in a `#` comment inside a body — and the resolver step this
// is about is a shell script long enough to carry comments. A job whose resolver
// had been reduced to `# DOSSIERX_TEST_BROWSER is set elsewhere` would have
// passed. A mention is not an assignment.
//
// WHAT THE VARIABLE IS FOR, so the failure message means something: viewer-tests
// resolves a browser and t.Skip()s when it finds none, and a skip is
// indistinguishable from a pass over zero assertions. Named explicitly, the
// harness FATALs on a path that is not there instead.
func TestTheViewerJobDeclaresTheBrowserVariable(t *testing.T) {
	jobKey, job := ciViewerSuiteJob(t, ciLoadWorkflow(t, ciWorkflowPath))

	assignment := browserPathVariable + "="
	for _, step := range job.Steps {
		for key := range step.Env {
			if key == browserPathVariable {
				return
			}
		}
		for _, argv := range ciCommands(step.Run) {
			for _, arg := range argv {
				if strings.HasPrefix(arg, assignment) {
					return
				}
			}
		}
	}
	t.Fatalf("%s's `%s` job — the one job declaring a run of %s — never mentions %s in a way this file can read: no step declares it under `env:`, and no parsed command in any step's `run:` body carries a `%s` token.\n"+
		"That suite resolves a browser and its harness FATALS on a path that is not there, but only when one is named; left to its own probe it reports success over zero browser assertions on a runner image that quietly dropped Chrome.\n"+
		"The body is parsed rather than searched, so the name appearing in a comment inside a `run:` block satisfies nothing — that is a mention, and a mention declares no variable.\n"+
		"It is required IN THIS JOB rather than anywhere in %s: a mention in another job is prose as far as this runner is concerned, and a whole-file version of this check stayed green while this job did not exist.",
		ciWorkflowPath, jobKey, ciViewerModule, browserPathVariable, assignment, ciWorkflowPath)
}
