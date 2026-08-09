// WHAT THIS FILE ESTABLISHES, AND WHY IT IS A DIFFERENT CLAIM FROM THE FILE NEXT
// TO IT. Read this before adding an assertion, because the boundary is again the
// point.
//
// tests/ci_workflow_test.go establishes what .github/workflows/ci.yml DECLARES,
// and its header says plainly that nothing which reads a file can establish that
// CI ran anything. That is still true and this file does not contradict it. This
// file's subject is the RUN: the per-test account the run itself emitted, fetched
// from GitHub for one commit, parsed, and adjudicated. Nothing on disk is
// evidence for that half, and nothing here pretends otherwise.
//
// THE INVARIANT, in one sentence: for the exact commit a release is about to be
// tagged on, the tests CI is configured to run were in fact executed, everywhere
// the workflow says they run, none of them failed — and the machinery that
// establishes this cannot report a pass unless it actually examined the run.
//
// WHY A CONCLUSION IS NEVER THE EVIDENCE. A check-run conclusion is `success`
// over zero tests. A job conclusion is `success` over a step that was forgiven by
// `continue-on-error`. This repository has never produced a run containing a
// forgiven step, so what the jobs API reports for one is unknown here, and an
// earlier draft of this lane rested on step conclusions and was wrong to: the run
// it cited had a job that concluded `failure`, i.e. ordinary propagation, not
// forgiveness. So NOTHING HERE READS A CONCLUSION AS EVIDENCE. The verdict rests
// on what the test binary itself emitted. Conclusions are recorded in the verdict
// record for a human to look at and are adjudicated by nothing — which is also
// why the forgiven-step case needs no fixture captured from a forgiven run: a
// forgiven failure and an unforgiven one leave the same `"Action":"fail"` event,
// and that event is the whole input.
//
// WHY THE ACCOUNT IS `go test -json` IN THE JOB LOG, AND NOT A FILE OR AN
// ARTIFACT. The suite step's body must be ONE command — that rule is what closes
// the `|| true` / `set +e` / `exit 0` / `| tee` suppression channel without a
// case per trick, and ciRunsModuleSuite refuses a second command outright rather
// than modelling a shell. Capturing a stream to a file needs a redirect (tokens
// outside the closed argument vocabulary) or a pipe (a second command), so every
// route that makes the account a separately addressable object requires breaking
// the one rule this project paid the most for. It is not broken. `-json` was
// already inside the closed vocabulary, `go test -race -json ./...` is still ONE
// command spelled entirely out of it, and the account therefore rides in the job
// log, where the binding problem below has to be solved rather than dodged.
//
// HOW A PARSED LINE IS TIED TO THE SUITE THAT PRODUCED IT — the hard part, and
// the reason two earlier attempts at this lane were rejected. Three facts bound
// the answer:
//
//   - `##[group]Run go test …` is PRINTABLE TEXT. A step body that echoes
//     `::group::Run go test -count=1 -json ./...` produces a byte-identical
//     header, so binding to it is text matching wearing a structural costume,
//     and this project has already paid three times for text matching.
//   - THE JOBS API's STEP TIMESTAMPS ARE SECOND-GRANULARITY while log lines carry
//     sub-second prefixes. On v0.5.0's healthy viewer job BOTH boundaries of the
//     suite step fall inside a second shared with a neighbouring step, including
//     the second holding the suite's own final line. Bracketing log lines by API
//     step windows and failing closed on ambiguity would FAIL a healthy run.
//   - THE RUN-LEVEL LOG ARCHIVE IS PER-JOB, not per-step: 24 entries for 12 jobs,
//     one file per job. There is no per-step object to address.
//
// So per-STEP attribution from a log is not available, and this file does not
// claim it. What IS available is three things a step body cannot control:
//
//  1. JOB IDENTITY, from the API. Each account is read from
//     `/actions/jobs/<id>/logs` for a job id the jobs API returned, matched to a
//     job name this file derived from ci.yml's matrix. Which job a line is in is
//     never inferred from the line.
//  2. ORDER. Steps run in sequence and the runner writes their output in that
//     order, so one step's lines cannot be interleaved into another's. Every
//     `go test -json` event in the job's log is therefore read as ONE ordered
//     stream and required to be strictly well formed: exactly one `start` and one
//     terminal per package, exactly one `run` and one terminal per test, and every
//     event inside its own package's brackets. A fabricated block for a package
//     the real suite also reports lands outside those brackets or duplicates
//     them, and the account is rejected as AMBIGUOUS rather than counted.
//  3. THE ACCOUNTING RULE, which is what defeats the forgery that IS well formed.
//     A step can print a complete, coherent, fabricated account for packages the
//     real suite never names. It cannot make the real packages report tests they
//     did not run: EVERY PACKAGE WHOSE TERMINAL IS `pass` MUST CARRY AT LEAST ONE
//     TEST-BEARING TERMINAL EVENT. A suite emptied by a run-time `$GITHUB_ENV`
//     export reports `ok <pkg> [no tests to run]` for every real package — five
//     events, zero of them test-bearing — and every one of those is a finding,
//     however much fabricated traffic sits beside it.
//
// WHAT THAT LEAVES, said rather than implied: this is not a fraud detector. A
// fabricated block ADDED BESIDE a real, complete, passing account changes nothing
// the invariant claims — the suite did run, everywhere, and nothing failed. What
// the rules above make inexpressible is a fabricated block that stands IN PLACE
// of one.
//
// A THIRD RESIDUE, in the pin rather than in the account. The Makefile recipe is
// held as TEXT — the four lines that hand the sha and the record path to
// `go test`, that refuse to succeed without a record, and that refuse a record
// about another commit are pinned as whole lines, not executed. Executing them
// from the suite would need `make` on the machine and a network-capable `gh`,
// and the root suite runs on three operating systems, so a check that shelled
// out to `make` would fail on a runner that has none — which is a red gate on an
// ordinary day, the failure mode that gets checks switched off. What the text
// pin cannot see is a recipe that names the right things and does the wrong one.
//
// TWO RESIDUES THIS FILE DOES NOT CLOSE. (1) The `hooks` job runs
// scripts/hook-smoke-test.sh on three platforms; a shell script emits no
// countable per-test account, so a smoke test that degenerates into asserting
// nothing still reaches a release as a green conclusion. (2) The `$GITHUB_ENV`
// export channel is closed here only for its effect on TEST SELECTION. The same
// channel can alter the Build and Vet steps with no test-count symptom at all,
// and no automated reader in this repository observes that.
//
// AND ONE RESIDUE INSIDE THE CLOSED HALF, split out rather than promised and
// disclaimed in the same breath. The ZERO case is closed: a selector that empties
// a package is a finding on that package. A selector that leaves a NONZERO BUT
// TINY selection in every package — one that still names at least one test
// everywhere — is not, because the only method for it is a comparison against
// what the tree declares, and subtests, build tags and per-platform test sets
// make that comparison wrong in the strict direction on ordinary days, which is
// the failure mode that gets a check switched off. Note how narrow the residue
// actually is: `GOFLAGS=-run=TestEnvelope` exported at run time leaves most of
// this repository's packages at zero tests and IS caught.
//
// RETENTION IS A DESIGNED FAILURE. GitHub log retention is finite (~90 days by
// default). Re-running this gate against an older release will report that the
// account could not be obtained, and that is a FAILURE, not a pass. There is no
// result here that means "we did not check" and reads as "it is fine": a missing
// run, an unfetchable log, an unparseable one, an instantiation with no account,
// and an account whose events cannot be attributed are each a finding.
//
// WHY THE ADJUDICATION IS CODE AND NOT A PROMPT, and the reason is inherited
// rather than invented here: it is the one the retired release checklist gave for
// making its agent PURE TRANSPORT — run one command, carry its output back
// verbatim, read nothing, count nothing, decide nothing. A prompt that says
// "confirm the suites actually executed" is satisfied by an agent that reads a
// conclusion and paraphrases it, and nothing downstream can tell that answer from
// one obtained by parsing the account: both arrive as fluent prose asserting a
// pass. So the fetching, the parsing, the counting and the deciding are all in
// this file, where a mutation makes a test go red rather than making a sentence
// read slightly differently. The checklist is gone and the transport role with
// it; the rule survives it, and it is why the driver's D1 reads a machine-written
// record instead of asking anybody how CI went.
//
// WHY THE GATE'S OWN ENTRY POINT IS HELD TO THE SAME INVARIANT. `go test` exits 0
// for a skipped test and exits 0 for a selector that matches nothing
// (`ok … [no tests to run]`, both observed on go1.26.5), so an exit status cannot
// tell "adjudicated and cleared" from "adjudicated nothing". The release-time
// invocation and the code it invokes can therefore drift apart by one identifier
// with `go test ./...` green forever — the same defect this lane exists to close,
// living at the one place that was supposed to close it. Three things answer it,
// and all three are needed:
//
//   - POSITIVE EVIDENCE. The stage writes a verdict record naming the commit it
//     examined, the suites it derived, every instantiation and what each one
//     accounted for. `make ci-evidence` refuses to exit 0 unless that record
//     exists and names the sha it was asked about, so an invocation that
//     adjudicated nothing is as loud as a suite that ran nothing.
//   - A CONSUMER THAT REFUSES WITHOUT IT. The record used to be carried to a
//     human by an agent transporting it into a checklist phase, where an absent
//     record was reported as COULD_NOT_RUN and the phase's own `clear`
//     computation treated that as blocking. That checklist is retired — it
//     published main before the tag, the forbidden order — and the record's
//     consumer is now the release driver itself: clause 6 of
//     cmd/dossierx/gate_driver_test.go, whose D1 reads the record at
//     DOSSIERX_GATE_CI_EVIDENCE_OUT and refuses the release as uncheckable
//     unless it names an object carrying the tree about to be published. The
//     transport was only ever as honest as the agent doing it; the driver's
//     refusal is not a result anybody reports, it is a release that does not
//     happen.
//   - THE STRUCTURAL PIN, which holds regardless of who runs what.
//     TestTheReleaseTimeInvocationNamesThisStage reads this file, the Makefile,
//     the driver and docs/RELEASING.md, and requires all four to name the same
//     identifiers. Rename any one side and the ORDINARY suite goes red. The
//     precedent is cmd/dossierx/gate_receipt_test.go's
//     TestReleaseProcedurePinsTheAncestryPrecondition, whose comment records a
//     review deleting a whole procedure item with the package still green.
//
// AND WHY THE ASSEMBLY ITSELF IS HELD TO IT, which is a third thing and was
// missing. Each stage above — the run selection, the matrix expansion, the
// adjudicator, the missing-cell guard, the cross-cell comparison — has its own
// test and goes red under mutation. The WIRING between them had none, because
// the only function that ran it was the live stage, which skips unless a commit
// is named and so was executed by nothing that ever ran. Sever the decide stage
// from the fetch and parse stages and every constituent test stays green while
// the one decision a release consumes reports PASS for any commit. So the
// decision is a function over a SOURCE — ciEvAssemble — and
// TestTheStageConsumesEverythingItFetched drives it over the recorded accounts
// in testdata/ci-run-evidence, asserting that what the record holds is what the
// fetched logs hold, that every constituent's finding reaches the verdict and
// the release-time answer, and that an account was fetched for every declared
// instantiation and for nothing else.
package tests

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// The identifiers BOTH sides of the release-time invocation must name.
//
// They are constants here, and TestTheReleaseTimeInvocationNamesThisStage
// requires the Makefile target, the release driver that consumes the record and
// the written procedure to spell each of them. That is the whole defence against
// the invoker and the invoked drifting apart: change one of these and the
// ordinary suite fails until every side has been changed with it.
// ---------------------------------------------------------------------------
const (
	// ciEvShaEnv names the commit under verification. Nothing is fetched without
	// it, and it is the merge commit, never HEAD — commits landing on main after
	// the merge are routine around a release (the content.ts sha stamp
	// necessarily lands after it), so evidence keyed to "the newest run on main"
	// is evidence about a tree that is not the one being tagged.
	ciEvShaEnv = "DOSSIERX_GATE_CI_SHA"

	// ciEvOutEnv names where the verdict record is written. It is REQUIRED
	// whenever ciEvShaEnv is set: the record is the only positive evidence that
	// the stage adjudicated anything, and a gate run that produces none has not
	// established that it ran.
	ciEvOutEnv = "DOSSIERX_GATE_CI_EVIDENCE_OUT"

	// ciEvMakeTarget is the one named command the procedure invokes, so the
	// invocation is not a memorised `go test -run` incantation whose selector can
	// drift by one character into matching nothing.
	ciEvMakeTarget = "ci-evidence"

	// ciEvStageTestName is the function below that does the fetching, parsing and
	// deciding. The Makefile selects it by this exact name.
	ciEvStageTestName = "TestReleaseGateCIRunEvidence"

	// ciEvStageFile is this file, addressed from the repository root. The pin
	// reads it to confirm the function ciEvStageTestName names actually exists,
	// so renaming the function without renaming the constant is a failure rather
	// than a selector that quietly matches nothing.
	ciEvStageFile = "tests/ci_run_evidence_test.go"

	// ciEvDriverFile is the release driver, and it is this record's CONSUMER.
	// Its D1 refuses to publish unless the record at ciEvOutEnv names an object
	// carrying the tree being released, which is what makes writing the record
	// worth doing: an unread record is a file, not a gate. An agent used to
	// carry the record into a checklist phase instead; that checklist is
	// retired, and this is the path that replaced it.
	ciEvDriverFile = "cmd/dossierx/gate_driver_test.go"

	// ciEvProcedureFile is the WRITTEN release procedure, and since the encoded
	// one was retired it is the ONLY one — which is what CLAUDE.md has always
	// required and what was not true while a second, drifting copy of it lived
	// in .claude/workflows/.
	ciEvProcedureFile = "docs/RELEASING.md"

	// ciEvFixtureDir holds the recorded accounts the parser and the adjudicator
	// are held against.
	ciEvFixtureDir = "testdata/ci-run-evidence"
)

// ciEvJSONFlag is the argument that makes a suite emit an account at all.
// Already inside ciAllowedTestFlags, so requiring it needs no vocabulary edit.
const ciEvJSONFlag = "-json"

// ---------------------------------------------------------------------------
// Findings. Every refusal carries a CODE, and every fixture test below asserts
// the code it expects rather than merely that something went wrong — an
// adjudicator that rejected everything would otherwise pass all six negative
// fixtures and only the healthy one would notice.
// ---------------------------------------------------------------------------
const (
	ciEvNoAccount         = "no-account"
	ciEvTruncated         = "truncated"
	ciEvUnterminatedPkg   = "unterminated-package"
	ciEvUnterminatedTest  = "unterminated-test"
	ciEvUnattributable    = "unattributable-event"
	ciEvAmbiguous         = "ambiguous-account"
	ciEvPackageRanNoTests = "package-ran-no-tests"
	ciEvTestFailed        = "test-failed"
	ciEvBuildFailed       = "build-failed"
	ciEvNoTestsAtAll      = "no-tests-at-all"
	ciEvMissingCell       = "missing-instantiation"
	// ciEvIncompleteCell is deliberately NOT ciEvMissingCell, though both say
	// "this instantiation has no account you may read". They are two guards one
	// `if` apart, and while they shared a code they masked each other: a listing
	// with a cell REMOVED still produced a finding when the absent-job branch was
	// disabled, because the zero-value job that branch then fell through with
	// carries a status that is not `completed`. Either guard could be deleted
	// with the suite green. One code each is what makes each of them provable.
	ciEvIncompleteCell    = "instantiation-not-completed"
	ciEvPackageSetDiffers = "package-set-differs"
	ciEvNoRun             = "no-ci-run-for-sha"
	ciEvAmbiguousRun      = "more-than-one-ci-run-for-sha"
)

type ciEvFinding struct {
	Code   string `json:"code"`
	Where  string `json:"where"`
	Detail string `json:"detail"`
}

func (f ciEvFinding) String() string { return f.Code + " [" + f.Where + "]: " + f.Detail }

func ciEvCodes(fs []ciEvFinding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func ciEvHasCode(fs []ciEvFinding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Parsing a job log into a `go test -json` event stream.
// ---------------------------------------------------------------------------

// ciEvRunnerStamp is the RFC3339 prefix the Actions runner puts on every line of
// a job log. EXACTLY ONE is stripped, and that is load-bearing rather than
// tidiness: a step body echoing a line that already begins with a timestamp gets
// the runner's own stamp prepended to it, so after one strip the remainder still
// begins with the echoed stamp and does not parse as an event. The runner's
// prefix is the one thing on a log line that a step cannot write.
var ciEvRunnerStamp = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d+Z `)

// ciEvEvent is one event of a `go test -json` stream. Only the fields this file
// reasons about are declared.
type ciEvEvent struct {
	Action     string `json:"Action"`
	Package    string `json:"Package"`
	Test       string `json:"Test"`
	ImportPath string `json:"ImportPath"`
	Output     string `json:"Output"`
}

// ciEvParseEvents extracts every `go test -json` event from a job log, in log
// order, and reports whether the log arrived complete.
//
// A line is an event when, after one runner stamp is removed, it parses as a
// JSON object carrying an `Action`. Everything else in the log — the runner's
// group framing, the `env:` block, the post-job cleanup — is skipped. Order is
// preserved because order is the only structural fact available: steps run in
// sequence, so one step's output cannot be interleaved into another's.
func ciEvParseEvents(log string) (events []ciEvEvent, complete bool) {
	complete = strings.HasSuffix(log, "\n")
	for _, line := range strings.Split(log, "\n") {
		line = ciEvRunnerStamp.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			continue
		}
		var e ciEvEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil || e.Action == "" {
			continue
		}
		events = append(events, e)
	}
	return events, complete
}

// ciEvAccount is what one instantiation accounted for.
type ciEvAccount struct {
	Packages []string `json:"packages"` // sorted import paths, whatever their terminal
	Tests    int      `json:"tests"`    // test-bearing terminal events
	Events   int      `json:"events"`
}

func ciEvIsTerminal(a string) bool { return a == "pass" || a == "fail" || a == "skip" }

// ciEvAdjudicate reads one job's log and says what its suite accounted for and
// what is wrong with it.
//
// The four rules are the ones the header argues for, and each earns its place
// against a different defeat:
//
//   - COMPLETENESS. A log that does not end in a newline arrived partial, and a
//     package that started without terminating is a stream that stopped. Both are
//     "we could not read the account", which is a failure and never a pass.
//   - WELL-FORMEDNESS. One `start` and one terminal per package, one `run` and one
//     terminal per test, every event inside its own package's brackets. This is
//     what makes a fabricated block for a package the real suite also reports
//     inexpressible: it either duplicates a bracket or falls outside one.
//   - NO FAILURE. Any `fail`, at package or test level, and any `build-fail`. Read
//     from the binary's own account, so a step forgiven by `continue-on-error`
//     is indistinguishable from one that was not — which is the point.
//   - EVERY PASSING PACKAGE ACCOUNTED FOR. A package whose terminal is `pass` and
//     which carries no test-bearing terminal ran nothing: that is `[no tests to
//     run]` seen structurally instead of by matching the string a job can print.
//     A `skip` terminal is a package with no test files and is not a finding.
func ciEvAdjudicate(where, log string) (ciEvAccount, []ciEvFinding) {
	var findings []ciEvFinding
	add := func(code, detail string) { findings = append(findings, ciEvFinding{code, where, detail}) }

	events, complete := ciEvParseEvents(log)
	acct := ciEvAccount{Events: len(events)}
	if !complete {
		add(ciEvTruncated, fmt.Sprintf("the job log does not end in a newline, so the last of its %d bytes is a partial line: the account arrived incomplete and an incomplete account is not evidence that the suite finished", len(log)))
	}
	if len(events) == 0 {
		add(ciEvNoAccount, "the job log carries no `go test -json` events at all. Either the suite step does not pass "+ciEvJSONFlag+", or this is not the job that ran it, or the step produced no output — each of which is a run this check could not examine, never a run it cleared")
		return acct, findings
	}

	type tkey struct{ pkg, test string }
	var (
		openPkg   = map[string]bool{}
		donePkg   = map[string]string{} // package -> terminal action
		openTest  = map[tkey]bool{}
		doneTest  = map[tkey]bool{}
		pkgTests  = map[string]int{}
		failures  []string
		pkgsOrder []string
	)

	for i, e := range events {
		switch e.Action {
		case "build-fail":
			add(ciEvBuildFailed, fmt.Sprintf("event %d reports a build failure for %q. A suite that did not compile ran no tests, whatever the job concluded", i, e.ImportPath))
			continue
		case "build-output", "start-build":
			continue
		}
		if e.Package == "" {
			add(ciEvUnattributable, fmt.Sprintf("event %d carries action %q and names no package, so nothing in the stream says what it is an account of. Real `go test -json` events always name one; an object that does not is either a malformed event or another tool's JSON that happens to carry an `Action` key, and this reader refuses rather than dropping what it cannot read", i, e.Action))
			continue
		}
		if e.Action == "start" {
			if openPkg[e.Package] || donePkg[e.Package] != "" {
				add(ciEvAmbiguous, fmt.Sprintf("event %d opens a SECOND account for package %s. One package reported twice in one job means two streams share this log, and which of them is the suite's is not a question this reader will answer by picking one", i, e.Package))
				continue
			}
			openPkg[e.Package] = true
			pkgsOrder = append(pkgsOrder, e.Package)
			continue
		}
		if !openPkg[e.Package] {
			add(ciEvAmbiguous, fmt.Sprintf("event %d (action %q, test %q) reports on package %s outside that package's own start..terminal bracket. Steps run in sequence and the runner writes their output in that order, so an event that lands outside its package's block was printed by something other than the run that opened it", i, e.Action, e.Test, e.Package))
			continue
		}
		if e.Test == "" {
			if ciEvIsTerminal(e.Action) {
				delete(openPkg, e.Package)
				donePkg[e.Package] = e.Action
				if e.Action == "fail" {
					failures = append(failures, e.Package)
				}
			}
			continue
		}
		k := tkey{e.Package, e.Test}
		switch {
		case e.Action == "run":
			if openTest[k] || doneTest[k] {
				add(ciEvAmbiguous, fmt.Sprintf("event %d starts test %s in %s a second time", i, e.Test, e.Package))
				continue
			}
			openTest[k] = true
		case ciEvIsTerminal(e.Action):
			if !openTest[k] {
				add(ciEvAmbiguous, fmt.Sprintf("event %d reports test %s in %s finishing without the stream ever starting it", i, e.Test, e.Package))
				continue
			}
			delete(openTest, k)
			doneTest[k] = true
			pkgTests[e.Package]++
			acct.Tests++
			if e.Action == "fail" {
				failures = append(failures, e.Package+"."+e.Test)
			}
		default:
			if !openTest[k] {
				add(ciEvAmbiguous, fmt.Sprintf("event %d (action %q) reports on test %s in %s which the stream never started", i, e.Action, e.Test, e.Package))
			}
		}
	}

	for pkg := range openPkg {
		add(ciEvUnterminatedPkg, fmt.Sprintf("package %s opened an account that never terminated. The stream stopped mid-package, so what that package did is unknown", pkg))
	}
	for k := range openTest {
		add(ciEvUnterminatedTest, fmt.Sprintf("test %s in %s started and never reported a result", k.test, k.pkg))
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		add(ciEvTestFailed, fmt.Sprintf("the account records %d failure(s): %s. This is read from the test binary's own events, so a step forgiven by `continue-on-error` reaches this check exactly as a step that was not — the conclusion the API reports for it is not an input here", len(failures), strings.Join(failures, ", ")))
	}

	var ranNothing []string
	for pkg, terminal := range donePkg {
		acct.Packages = append(acct.Packages, pkg)
		if terminal == "pass" && pkgTests[pkg] == 0 {
			ranNothing = append(ranNothing, pkg)
		}
	}
	sort.Strings(acct.Packages)
	if len(ranNothing) > 0 {
		sort.Strings(ranNothing)
		add(ciEvPackageRanNoTests, fmt.Sprintf("%d package(s) reported PASS having executed no test at all: %s. That is `ok <pkg> [no tests to run]` read structurally rather than by matching a string a job can print — a selector reached the suite at run time, or the tests were compiled out. A pass over zero tests is the exact result this check exists to refuse", len(ranNothing), strings.Join(ranNothing, ", ")))
	}
	if acct.Tests == 0 && !ciEvHasCode(findings, ciEvPackageRanNoTests) {
		add(ciEvNoTestsAtAll, fmt.Sprintf("the account holds %d events and not one executed test. Whatever this job did, it did not run a suite", len(events)))
	}
	return acct, findings
}

// ---------------------------------------------------------------------------
// Which suites must be accounted for, and in how many instantiations.
//
// Derived from ci.yml through the parser tests/ci_workflow_test.go already owns,
// so the set cannot go stale the first time a job is added — and so a suite step
// narrowed in the file stops being recognised as a suite step, which is what
// makes the declared half of the narrowing question closable at all.
// ---------------------------------------------------------------------------

// ciEvMatrixWorkflow re-reads ci.yml for the two keys the shared model
// deliberately does not carry: a job's `name:` template and its
// `strategy.matrix`. That is not the boundary tests/ci_workflow_test.go's header
// draws — it refuses `if:` and `continue-on-error:` because reading them was an
// attempt to prove that a declared job EXECUTES, which no reader of a file can
// do. Enumerating a matrix proves nothing about execution: it says how many
// accounts must be found in the run, and a cell that produced none is a failure
// here rather than an absence.
type ciEvMatrixWorkflow struct {
	Jobs map[string]ciEvMatrixJob `yaml:"jobs"`
}

type ciEvMatrixJob struct {
	Name     string `yaml:"name"`
	Strategy *struct {
		Matrix any `yaml:"matrix"`
	} `yaml:"strategy"`
}

// ciEvSuite is one declared suite: the module, the job that runs it, the step
// that does, and every name the run will report that job under.
type ciEvSuite struct {
	Module         string   `json:"module"`
	JobKey         string   `json:"job"`
	Instantiations []string `json:"instantiations"`
}

var ciEvMatrixRef = regexp.MustCompile(`\$\{\{\s*matrix\.([A-Za-z0-9_.-]+)\s*\}\}`)

// ciEvExpandInstantiations turns one job's `name:` template and `strategy.matrix`
// into the exact job names the run will report.
//
// EVERYTHING IT CANNOT ENUMERATE IS A FAILURE, never a reason to count what
// showed up. `include`/`exclude`, a matrix that is an expression, a value that is
// itself an expression, a matrixed job with no explicit `name:` — each returns an
// error, because the alternative is a check whose expected count is derived from
// the answer it is supposed to be testing.
func ciEvExpandInstantiations(key string, job ciEvMatrixJob) ([]string, error) {
	tmpl := job.Name
	if tmpl == "" {
		tmpl = key
	}

	if job.Strategy == nil || job.Strategy.Matrix == nil {
		if ciEvMatrixRef.MatchString(tmpl) {
			return nil, fmt.Errorf("job %q names matrix values in its `name:` (%q) but declares no `strategy.matrix`", key, tmpl)
		}
		if strings.Contains(tmpl, "${{") {
			return nil, fmt.Errorf("job %q's name %q resolves at run time, so the name this job will report cannot be known before the run", key, tmpl)
		}
		return []string{tmpl}, nil
	}

	matrix, ok := job.Strategy.Matrix.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("job %q's `strategy.matrix` is not a mapping (%T); a matrix built by an expression cannot be enumerated from the file", key, job.Strategy.Matrix)
	}
	for _, reserved := range []string{"include", "exclude"} {
		if _, present := matrix[reserved]; present {
			return nil, fmt.Errorf("job %q's matrix declares `%s`, which this reader does not model. Supporting it is a deliberate edit to tests/ci_run_evidence_test.go with a reason attached; guessing at the cell set would make the expected number of accounts depend on the same document the check is trying to hold", key, reserved)
		}
	}
	if job.Name == "" {
		return nil, fmt.Errorf("job %q declares a matrix but no `name:`, so the names the runner will report are its own default form. This reader will not guess at that form — give the job an explicit `name:` naming its matrix values", key)
	}

	keys := make([]string, 0, len(matrix))
	for k := range matrix {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	cells := []map[string]string{{}}
	for _, k := range keys {
		list, ok := matrix[k].([]any)
		if !ok {
			return nil, fmt.Errorf("job %q's matrix dimension %q is not a list (%T)", key, k, matrix[k])
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("job %q's matrix dimension %q is empty, so the job instantiates zero times and the suite it declares runs nowhere", key, k)
		}
		var next []map[string]string
		for _, cell := range cells {
			for _, v := range list {
				s := ciScalar(v)
				if strings.Contains(s, "${{") {
					return nil, fmt.Errorf("job %q's matrix value %q resolves at run time", key, s)
				}
				grown := map[string]string{}
				for kk, vv := range cell {
					grown[kk] = vv
				}
				grown[k] = s
				next = append(next, grown)
			}
		}
		cells = next
	}

	var names []string
	for _, cell := range cells {
		var missing string
		name := ciEvMatrixRef.ReplaceAllStringFunc(tmpl, func(m string) string {
			dim := ciEvMatrixRef.FindStringSubmatch(m)[1]
			v, ok := cell[dim]
			if !ok {
				missing = dim
				return m
			}
			return v
		})
		if missing != "" {
			return nil, fmt.Errorf("job %q's name %q names matrix dimension %q, which the matrix does not declare", key, tmpl, missing)
		}
		if strings.Contains(name, "${{") {
			return nil, fmt.Errorf("job %q's name resolves to %q, which still holds a run-time expression", key, name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// ciEvMatrixJobFromYAML builds one job declaration out of the YAML a maintainer
// would actually type, so the refusals below are exercised through the same
// decode ci.yml goes through rather than against hand-built structs that could
// not arise from a document.
func ciEvMatrixJobFromYAML(t *testing.T, doc string) ciEvMatrixJob {
	t.Helper()
	var job ciEvMatrixJob
	if err := yaml.Unmarshal([]byte(doc), &job); err != nil {
		t.Fatalf("this test's own job declaration does not parse as YAML: %v\n%s", err, doc)
	}
	return job
}

// TestTheInstantiationExpansionRefusesEveryShapeItCannotEnumerate walks every
// refusal in ciEvExpandInstantiations.
//
// WHY THESE NEED A TEST AT ALL. Not one of them has a shape in ci.yml today: the
// workflow declares two plain dimensions and an explicit `name:`, so the only
// branches a run of this repository's suite ever reaches are the happy ones. Each
// refusal is what stands between a matrix this reader cannot enumerate and an
// expected-instantiation count derived from whatever the run happened to produce
// — which is precisely how six healthy cells come to stand in for the one that
// silently skipped — and an assertion nothing reaches is an assertion nobody has
// seen work.
//
// EACH CASE ASSERTS A DISTINCT FRAGMENT of the refusal it expects rather than
// merely that an error came back, because these are thirteen branches of one
// function and a test that only asked "did it fail" would be satisfied by twelve
// of them being unreachable.
func TestTheInstantiationExpansionRefusesEveryShapeItCannotEnumerate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		key       string
		yaml      string
		wantNames []string
		wantSaid  string
		why       string
	}{
		{
			name: "the real shape: two dimensions and an explicit name",
			key:  "test",
			yaml: "name: test (go ${{ matrix.go }}, ${{ matrix.os }})\nstrategy:\n  matrix:\n    go: [\"stable\", \"1.26.x\"]\n    os: [ubuntu-latest, windows-latest]\n",
			wantNames: []string{
				"test (go 1.26.x, ubuntu-latest)",
				"test (go 1.26.x, windows-latest)",
				"test (go stable, ubuntu-latest)",
				"test (go stable, windows-latest)",
			},
			why: "the cross product, in the shape ci.yml declares it. Without this case a function that refused everything would satisfy every other case here",
		},
		{
			name:      "no matrix, a plain name",
			key:       "lint",
			yaml:      "name: lint (root and viewer)\n",
			wantNames: []string{"lint (root and viewer)"},
			why:       "a job that instantiates once reports one name — the `name:` it declares, NOT its key, which is what ci.yml's own jobs mostly do. A reader that reported the key would demand accounts under names the run never uses",
		},
		{
			name:      "no matrix and no name at all",
			key:       "tidy",
			yaml:      "{}\n",
			wantNames: []string{"tidy"},
			why:       "GitHub reports a job with no `name:` under its key, and that is the one default form this reader will assume — the only one that is not a guess",
		},
		{
			name:     "a name that reads matrix values from a matrix that is not there",
			key:      "test",
			yaml:     "name: test (${{ matrix.os }})\n",
			wantSaid: "declares no `strategy.matrix`",
			why:      "the job will report a literal `${{ matrix.os }}` or nothing recognisable; either way the derived name matches no job in the run and every cell would be reported missing on a healthy release",
		},
		{
			name:     "a name resolved at run time",
			key:      "test",
			yaml:     "name: test (${{ github.event_name }})\n",
			wantSaid: "cannot be known before the run",
			why:      "a name computed from the event is not derivable from the file, so the expected set of accounts would have to come from the run itself — the answer this check is supposed to be testing",
		},
		{
			name:     "a matrix built by an expression",
			key:      "test",
			yaml:     "name: test (${{ matrix.os }})\nstrategy:\n  matrix: \"${{ fromJSON(needs.plan.outputs.matrix) }}\"\n",
			wantSaid: "is not a mapping",
			why:      "a matrix computed from another job's output cannot be enumerated from the document at all. Counting the cells that showed up instead is exactly the hole clause 4 of the invariant names",
		},
		{
			name:     "a matrix with include",
			key:      "test",
			yaml:     "name: test (${{ matrix.os }})\nstrategy:\n  matrix:\n    os: [ubuntu-latest]\n    include:\n      - os: macos-latest\n",
			wantSaid: "`include`",
			why:      "`include` adds cells and can add dimensions to some of them; a cross product computed without it is short, and a short expectation is satisfied by a run that is short in the same place",
		},
		{
			name:     "a matrix with exclude",
			key:      "test",
			yaml:     "name: test (${{ matrix.os }})\nstrategy:\n  matrix:\n    os: [ubuntu-latest, windows-latest]\n    exclude:\n      - os: windows-latest\n",
			wantSaid: "`exclude`",
			why:      "`exclude` removes cells, so the cross product is LONG and the gate would demand an account for an instantiation the workflow deliberately does not run — a red gate on an ordinary day, which is the failure mode that gets a check switched off",
		},
		{
			name:     "a matrixed job with no explicit name",
			key:      "test",
			yaml:     "strategy:\n  matrix:\n    os: [ubuntu-latest, windows-latest]\n",
			wantSaid: "declares a matrix but no `name:`",
			why:      "the runner then invents the reported name from the cell values in an order this reader would have to guess at, and a guess that is wrong reports every cell missing",
		},
		{
			name:     "a dimension that is not a list",
			key:      "test",
			yaml:     "name: test (${{ matrix.go }})\nstrategy:\n  matrix:\n    go: stable\n",
			wantSaid: "is not a list",
			why:      "a scalar where a list belongs is a malformed matrix; reading it as a one-element list would be this reader deciding what the workflow meant",
		},
		{
			name:     "an empty dimension",
			key:      "test",
			yaml:     "name: test (${{ matrix.go }})\nstrategy:\n  matrix:\n    go: []\n",
			wantSaid: "is empty, so the job instantiates zero times",
			why:      "an empty dimension means the suite runs NOWHERE while the workflow still declares it. Zero expected accounts and zero accounts found would agree perfectly, and the release would be gated on a suite that never ran",
		},
		{
			name:     "a matrix value that resolves at run time",
			key:      "test",
			yaml:     "name: test (${{ matrix.os }})\nstrategy:\n  matrix:\n    os: [\"${{ vars.DEFAULT_RUNNER }}\"]\n",
			wantSaid: "matrix value",
			why:      "the cell exists but the name it reports is decided outside the file, so the derived name matches nothing in the run",
		},
		{
			name:     "a name reading a dimension the matrix does not declare",
			key:      "test",
			yaml:     "name: test (${{ matrix.node }})\nstrategy:\n  matrix:\n    os: [ubuntu-latest]\n",
			wantSaid: "which the matrix does not declare",
			why:      "a renamed dimension leaves the name template pointing at nothing — the likeliest of all these edits, and it would silently derive names no job ever reports",
		},
		{
			name:     "a name that still holds an expression after substitution",
			key:      "test",
			yaml:     "name: test (${{ matrix.os }} @ ${{ github.sha }})\nstrategy:\n  matrix:\n    os: [ubuntu-latest]\n",
			wantSaid: "still holds a run-time expression",
			why:      "half the name is derivable and half is not, which is the case a reader that only checked the template BEFORE substitution would wave through",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ciEvExpandInstantiations(tc.key, ciEvMatrixJobFromYAML(t, tc.yaml))
			if tc.wantSaid == "" {
				if err != nil {
					t.Fatalf("expanding a job this reader must handle failed: %v\nWhy this case is here: %s", err, tc.why)
				}
				if strings.Join(got, "\n") != strings.Join(tc.wantNames, "\n") {
					t.Fatalf("expanded to %v, want %v.\nWhy this case is here: %s", got, tc.wantNames, tc.why)
				}
				return
			}
			if err == nil {
				t.Fatalf("this job expanded to %v rather than being refused.\nThe expected number of accounts comes from the statically declared matrix, and a matrix this reader cannot enumerate must be a FAILURE rather than a licence to count whatever the run produced.\nWhy this case is here: %s", got, tc.why)
			}
			if !strings.Contains(err.Error(), tc.wantSaid) {
				t.Fatalf("the refusal does not say %q, so it is not the refusal this case is about — some other branch fired and the branch under test may never run at all.\nWhy this case is here: %s\nIt said:\n%v", tc.wantSaid, tc.why, err)
			}
		})
	}
}

// ciEvSuiteStepFor returns the step in a job that declares a run of the module's
// suite, through the shared parser rather than a second model of one.
func ciEvSuiteStepFor(wf ciWorkflow, jobKey, mod string) (ciStep, bool) {
	outer := []ciEnvScope{
		{where: "the workflow's top-level `env:`", env: wf.Env},
		{where: fmt.Sprintf("job `%s`'s `env:`", jobKey), env: wf.Jobs[jobKey].Env},
	}
	for _, step := range wf.Jobs[jobKey].Steps {
		if ok, _ := ciRunsModuleSuite(step, mod, outer); ok {
			return step, true
		}
	}
	return ciStep{}, false
}

// ciEvGoTestNearMisses reports the steps that enter a module and run a `go test`
// that this file's vocabulary refused — and ONLY those.
//
// The shared ciSuiteJobsFor returns every near miss, which reads well for a
// nested module and is unusable at the root: with mod "" every step in every job
// that declares no `working-directory` qualifies, so hooks, tidy, gofmt, Build
// and Vet all arrive as near misses and the failure message is noise nobody
// reads. The step a maintainer needs named is the one that was a suite run and
// stopped being one, so that is the only one reported, with a count of the rest.
func ciEvGoTestNearMisses(wf ciWorkflow, mod string) (named []string, others int) {
	for _, jobKey := range ciJobNames(wf) {
		outer := []ciEnvScope{
			{where: "the workflow's top-level `env:`", env: wf.Env},
			{where: fmt.Sprintf("job `%s`'s `env:`", jobKey), env: wf.Jobs[jobKey].Env},
		}
		for _, step := range wf.Jobs[jobKey].Steps {
			ok, why := ciRunsModuleSuite(step, mod, outer)
			if ok || why == "" {
				continue
			}
			isGoTest := false
			for _, argv := range ciCommands(step.Run) {
				name, rest, _, ok := ciCommandName(argv)
				if ok && name == "go" && len(rest) > 0 && rest[0] == "test" {
					isGoTest = true
				}
			}
			if isGoTest {
				named = append(named, fmt.Sprintf("job %s, step %q: %s", jobKey, step.Name, why))
			} else {
				others++
			}
		}
	}
	return named, others
}

// ciEvResolveSuiteJob resolves one module to THE job that declares a run of its
// suite, or says why it will not.
//
// Anything other than exactly one job is a refusal. Zero is the deletion case and
// the narrowing case at once — a `-run` selector written into the step makes it
// stop parsing as a suite run — and two means this reader cannot say which job's
// account is the account.
//
// IT RETURNS AN ERROR RATHER THAN FATALLING so that both refusals can be
// exercised against a constructed workflow. Neither has a shape in this
// repository's ci.yml — there is exactly one root suite job and exactly one
// viewer one — and a refusal nothing can reach is a refusal nothing has ever
// shown to work.
func ciEvResolveSuiteJob(wf ciWorkflow, mod string) (string, error) {
	label := mod
	if label == "" {
		label = "the root module"
	}
	found, _ := ciSuiteJobsFor(wf, mod)
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		named, others := ciEvGoTestNearMisses(wf, mod)
		detail := fmt.Sprintf("No step runs a `go test` there at all (%d step(s) enter it and run something else).", others)
		if len(named) > 0 {
			detail = fmt.Sprintf("Step(s) that run a `go test` there and were refused:\n\t%s\n(%d further step(s) enter it and run something that is not a `go test` — not listed, because at the root that is every step in the workflow.)",
				strings.Join(named, "\n\t"), others)
		}
		return "", fmt.Errorf("no job in %s declares a run of %s's suite, so there is no account for this check to require and nothing to fetch.\n%s\n"+
			"A `-run`/`-skip`/`-tags` selector written into the step, or a narrowing `env:`, makes the step stop parsing as a suite run — which is this failure, and it is the only thing in this repository that catches a narrowing declared in the file for the ROOT suite: tests/nested_module_coverage_test.go returns early on the root module by design, and both tests in tests/ci_workflow_test.go key on the viewer job",
			ciWorkflowPath, label, detail)
	default:
		return "", fmt.Errorf("%d jobs in %s declare a run of %s (%v). Each is a separate run job with its own account, and a check that picked one would be reporting on a suite nobody chose — while the other job's cells went unexamined and its failures unread",
			len(found), ciWorkflowPath, label, found)
	}
}

// TestTheSuiteLookupRefusesZeroJobsAndTwo exercises both refusals above against
// workflows built here, because neither can be reached through ci.yml as it
// stands and an assertion that cannot be reached has never been shown to fire.
//
// The two-job case is the one with no natural shape at all: a maintainer adds a
// second job that runs the same module's suite — a nightly variant, a
// second-platform copy — and a reader that took the first would silently gate the
// release on one of them.
func TestTheSuiteLookupRefusesZeroJobsAndTwo(t *testing.T) {
	suite := func(name string) ciStep {
		return ciStep{Name: name, WorkingDirectory: ciViewerModule, Run: "go test -count=1 " + ciAllPackages}
	}
	narrowed := ciStep{Name: "Viewer browser suite", WorkingDirectory: ciViewerModule, Run: "go test -count=1 -run TestSmoke " + ciAllPackages}

	for _, tc := range []struct {
		name     string
		wf       ciWorkflow
		wantJob  string
		wantSaid []string
		why      string
	}{
		{
			name:    "exactly one job declares it",
			wf:      ciWorkflow{Jobs: map[string]ciJob{"viewer": {Steps: []ciStep{suite("Viewer browser suite")}}}},
			wantJob: "viewer",
			why:     "the ordinary shape. A resolver that refused this would fail every gate run, so this case is what stops the two below from being satisfied by a function that always errors",
		},
		{
			name:     "the step stopped being a suite run",
			wf:       ciWorkflow{Jobs: map[string]ciJob{"viewer": {Steps: []ciStep{narrowed}}}},
			wantSaid: []string{"no job in", "Viewer browser suite", "-run TestSmoke"},
			why:      "a `-run` selector written into the step is the declared narrowing this lane must catch, and the refusal has to NAME the step that stopped qualifying — a bare `no job runs the viewer suite` in a workflow that plainly has a viewer job sends a maintainer looking for a deleted job instead of at the argument that was added",
		},
		{
			name: "two jobs declare the same module's suite",
			wf: ciWorkflow{Jobs: map[string]ciJob{
				"viewer":         {Steps: []ciStep{suite("Viewer browser suite")}},
				"viewer-nightly": {Steps: []ciStep{suite("Viewer browser suite (nightly)")}},
			}},
			wantSaid: []string{"viewer", "viewer-nightly"},
			why:      "two jobs each run the suite, each with its own matrix and its own account. Picking one is an answer nobody decided, and the release would then be gated on whichever the map iterated to first",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ciEvResolveSuiteJob(tc.wf, ciViewerModule)
			if tc.wantJob != "" {
				if err != nil {
					t.Fatalf("resolving a workflow with one suite job failed: %v\nWhy this case is here: %s", err, tc.why)
				}
				if got != tc.wantJob {
					t.Fatalf("resolved to job %q, want %q. Why this case is here: %s", got, tc.wantJob, tc.why)
				}
				return
			}
			if err == nil {
				t.Fatalf("resolving this workflow returned job %q and no error at all.\nWhy this case is here: %s", got, tc.why)
			}
			for _, said := range tc.wantSaid {
				if !strings.Contains(err.Error(), said) {
					t.Fatalf("the refusal does not name %q, so a maintainer reading it cannot tell which job or step it is about.\nWhy this case is here: %s\nIt said:\n%v", said, tc.why, err)
				}
			}
		})
	}
}

// ciEvDeclaredSuites derives every suite the workflow declares: the root module
// and every nested one, each resolved to exactly one job and one step.
func ciEvDeclaredSuites(t *testing.T, wf ciWorkflow) []ciEvSuite {
	t.Helper()

	mods := append([]string{""}, wiringNestedModules(t)...)
	if len(mods) < 2 {
		t.Fatalf("the module walk found %d module(s) (%v). This repository has a root module and at least viewer-tests, so a walk that finds fewer has broken — and every assertion below would then pass over a set of suites that is not the set of suites",
			len(mods), mods)
	}

	root := wiringRepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ciWorkflowPath)))
	if err != nil {
		t.Fatalf("read %s: %v", ciWorkflowPath, err)
	}
	var matrixWF ciEvMatrixWorkflow
	if err := yaml.Unmarshal(raw, &matrixWF); err != nil {
		t.Fatalf("parse %s for job names and matrices: %v", ciWorkflowPath, err)
	}

	var suites []ciEvSuite
	for _, mod := range mods {
		label := mod
		if label == "" {
			label = "the root module"
		}
		jobKey, err := ciEvResolveSuiteJob(wf, mod)
		if err != nil {
			t.Fatalf("%v\nThis is a FAILED check, not an absent one", err)
		}
		names, err := ciEvExpandInstantiations(jobKey, matrixWF.Jobs[jobKey])
		if err != nil {
			t.Fatalf("cannot enumerate the instantiations of %s's suite job: %v.\n"+
				"The expected number of accounts comes from the statically declared matrix, and a matrix that cannot be enumerated is a FAILURE rather than a licence to count whatever the run happened to produce — that is how six healthy matrix cells come to stand in for the one that silently skipped",
				label, err)
		}
		suites = append(suites, ciEvSuite{Module: mod, JobKey: jobKey, Instantiations: names})
	}
	return suites
}

// ---------------------------------------------------------------------------
// The static half: what ci.yml must declare for an account to exist at all.
// ---------------------------------------------------------------------------

// TestEverySuiteTheWorkflowDeclaresEmitsAnAccount is the declaration half of this
// lane, and it is the first thing in this repository to measure the ROOT suite
// step against the closed vocabularies that guard the viewer one.
//
// Two claims. Every suite the workflow declares resolves to exactly one job —
// which is where a `-run TestEnvelope` written into ci.yml:149 lands, because a
// narrowed invocation stops parsing as a suite run — and that job's suite step
// passes `-json`, without which the run emits one `ok <pkg> <time>` line per
// package and there is no account to fetch for any commit.
func TestEverySuiteTheWorkflowDeclaresEmitsAnAccount(t *testing.T) {
	wf := ciLoadWorkflow(t, ciWorkflowPath)
	suites := ciEvDeclaredSuites(t, wf)

	for _, suite := range suites {
		label := suite.Module
		if label == "" {
			label = "the root module"
		}
		step, ok := ciEvSuiteStepFor(wf, suite.JobKey, suite.Module)
		if !ok {
			t.Fatalf("job %s was derived as the one that runs %s's suite, but re-reading its steps through the same parser finds no such step. The two reads disagree, which means this file's model of a suite step and ciSuiteJobsFor's have drifted apart", suite.JobKey, label)
		}

		emits := false
		for _, argv := range ciCommands(step.Run) {
			for _, arg := range argv {
				if arg == ciEvJSONFlag {
					emits = true
				}
			}
		}
		if !emits {
			t.Errorf("%s's suite step (job %s, step %q) runs `%s`, which passes no `%s`.\n"+
				"Without it the step's entire output is one `ok <pkg> <time>` line per package — that is what v0.5.0's viewer job emitted, and no per-test account can be derived from it by this check or by the person docs/RELEASING.md asks to open the step.\n"+
				"`%s` is already inside ciAllowedTestFlags in tests/ci_workflow_test.go, so adding it needs no vocabulary edit and keeps the body ONE command: `go test -count=1 %s ./...`.\n"+
				"A redirect or a pipe to capture the stream elsewhere is NOT the fix — ciRunsModuleSuite refuses a second command outright, and that refusal is what closes the `|| true` / `set +e` suppression channel",
				label, suite.JobKey, step.Name, strings.TrimSpace(step.Run), ciEvJSONFlag, ciEvJSONFlag, ciEvJSONFlag)
		}
	}
}

// TestTheDerivedInstantiationsAreTheNamesGitHubReports holds this file's matrix
// expansion against the names GitHub ACTUALLY reported for a real run.
//
// It is the one thing that makes the expansion more than a plausible-looking
// string substitution. testdata/ci-run-evidence/jobs.json is a verbatim capture
// of `/actions/runs/31153428132/jobs` — v0.5.0's CI run — and the six names the
// expansion produces for the `test` job must appear in it exactly.
//
// IT WILL RED IF THE SUITE JOB'S `name:` OR MATRIX CHANGES, and that is intended
// rather than tolerated: the instantiation set IS what this gate requires an
// account for, so changing it is a deliberate edit, and the fix is to re-capture
// the fixture from a run of the changed workflow. The command is in the header of
// the fixture directory's own test below.
func TestTheDerivedInstantiationsAreTheNamesGitHubReports(t *testing.T) {
	wf := ciLoadWorkflow(t, ciWorkflowPath)
	suites := ciEvDeclaredSuites(t, wf)

	reported := map[string]bool{}
	for _, job := range ciEvLoadJobsFixture(t, "jobs.json") {
		reported[job.Name] = true
	}
	if len(reported) == 0 {
		t.Fatal("the recorded jobs listing holds no jobs, so every name below would be compared against nothing")
	}

	checked := 0
	for _, suite := range suites {
		for _, name := range suite.Instantiations {
			checked++
			if !reported[name] {
				got := make([]string, 0, len(reported))
				for n := range reported {
					got = append(got, n)
				}
				sort.Strings(got)
				t.Errorf("this file expands %s's suite job %q to an instantiation named %q, which GitHub never reported for the recorded run.\n"+
					"Reported: %v\n"+
					"Either the expansion is wrong — in which case a live gate run would report every cell missing and block a healthy release — or the job's `name:`/matrix changed, in which case re-capture %s/jobs.json from a run of the changed workflow:\n"+
					"    gh api \"repos/<owner>/<repo>/actions/runs/<run-id>/jobs?per_page=100\" --jq '{total_count, jobs: [.jobs[] | {id, run_id, name, status, conclusion}]}'",
					suite.Module, suite.JobKey, name, got, ciEvFixtureDir)
			}
		}
	}
	if checked < 2 {
		t.Fatalf("only %d instantiation(s) were compared. The root suite alone declares a six-cell matrix, so a run of this test that compares fewer than two names has lost its subject and is passing over nothing", checked)
	}
	t.Logf("compared %d derived instantiation(s) against %d reported job(s)", checked, len(reported))
}

// ---------------------------------------------------------------------------
// The fixtures.
// ---------------------------------------------------------------------------

type ciEvJob struct {
	ID         int64  `json:"id"`
	RunID      int64  `json:"run_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type ciEvJobsPage struct {
	TotalCount int       `json:"total_count"`
	Jobs       []ciEvJob `json:"jobs"`
}

type ciEvRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadSha    string `json:"head_sha"`
	RunAttempt int    `json:"run_attempt"`
}

type ciEvRunsPage struct {
	TotalCount   int       `json:"total_count"`
	WorkflowRuns []ciEvRun `json:"workflow_runs"`
}

func ciEvFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(wiringRepoRoot(t), filepath.FromSlash(ciEvFixtureDir), name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v\nThe fixtures ARE the tests here: without them the adjudicator below is exercised by nothing and every assertion about it passes over zero accounts", name, err)
	}
	return string(b)
}

func ciEvLoadJobsFixture(t *testing.T, name string) []ciEvJob {
	t.Helper()
	var page ciEvJobsPage
	if err := json.Unmarshal([]byte(ciEvFixture(t, name)), &page); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return page.Jobs
}

// TestTheAdjudicatorClearsAHealthyAccount is the one fixture that must produce
// NOTHING, and it is what stops every other test below from being satisfied by a
// reader that refuses everything.
func TestTheAdjudicatorClearsAHealthyAccount(t *testing.T) {
	acct, findings := ciEvAdjudicate("job-healthy.log", ciEvFixture(t, "job-healthy.log"))
	if len(findings) != 0 {
		t.Fatalf("a healthy recorded account produced %d finding(s): %v.\nEvery negative fixture below asserts a specific code, but a reader that refuses everything would satisfy all of them; this is the test that says the reader can clear a real run. A check that reds an ordinary day is a check that gets switched off",
			len(findings), findings)
	}
	if acct.Tests == 0 || len(acct.Packages) == 0 {
		t.Fatalf("the healthy account cleared but accounted for %d test(s) across %d package(s). Clearing an account is only meaningful if the account had something in it", acct.Tests, len(acct.Packages))
	}
	t.Logf("healthy account: %d packages, %d tests, %d events", len(acct.Packages), acct.Tests, acct.Events)
}

// TestTheAdjudicatorRefusesEachRecordedDefect walks the recorded defects, each
// asserting the CODE it must produce rather than merely that something was
// wrong — the distinction the healthy fixture above makes load-bearing.
func TestTheAdjudicatorRefusesEachRecordedDefect(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		codes   []string
		why     string
	}{
		{"job-zero-tests.log", []string{ciEvPackageRanNoTests},
			"a run-time selector emptied the suite: every package reports `ok <pkg> [no tests to run]`, its step concludes success, its job concludes success and the check run is green. This is the failure the lane exists for, and the string `[no tests to run]` is not what catches it — the absence of a test-bearing terminal event under a passing package is"},
		{"job-failed-test.log", []string{ciEvTestFailed},
			"a test failed and the step was forgiven by `continue-on-error`. The account is the test binary's own, so the forgiveness is invisible to it and irrelevant: this reader never asks the API what the step concluded"},
		{"job-forged-header.log", []string{ciEvPackageRanNoTests},
			"a step printed a COMPLETE, well-formed, fabricated account under a fabricated `::group::Run go test …` header while the real suite ran nothing. The forgery is well formed, so no structural rule rejects it — what rejects the run is that the real packages passed having executed nothing, which no amount of fabricated traffic beside them can change"},
		{"job-forged-overlap.log", []string{ciEvAmbiguous},
			"the fabricated events name the same packages the real suite reports, so the log carries two accounts for one package. Steps run in sequence, so the second block cannot be inside the first: the stream is ambiguous and is refused rather than counted"},
		{"job-orphan-events.log", []string{ciEvAmbiguous},
			"an earlier step replayed a handful of per-test events it had captured while chasing a flake — no `start`, no package terminal, just the interesting lines — and they land outside any package's bracket. This is the accidental version of the forgery and the likelier one: nobody set out to deceive anything. A reader that counted every test-bearing event it could find would add three passes to a suite that ran none"},
		{"job-forged-passes-only.log", []string{ciEvAmbiguous},
			"a step replayed a fabricated block built the way anyone would build one — by keeping the `pass` lines and dropping the rest — so three tests report a result the stream never started them for. A real account states every test twice, once to open it and once to close it, and a reader that accepted a terminal on its own would count exactly the events a `grep '\"Action\":\"pass\"'` produces"},
		{"job-empty.log", []string{ciEvNoAccount},
			"the suite step produced no output at all. Nothing was examined, so nothing is cleared"},
		{"job-truncated.log", []string{ciEvTruncated},
			"the fetch delivered a partial log, cut mid-line. GitHub's log retention is finite and a `-json` account runs to thousands of lines; a truncated account must read as could-not-examine, never as a shorter pass"},
		{"job-truncated-at-line-boundary.log", []string{ciEvUnterminatedPkg, ciEvUnterminatedTest},
			"the same truncation landing exactly on a line boundary, which is the case the trailing-newline test cannot see. What gives it away is a package that opened an account and never closed it: the stream stopped, so what that package did is unknown"},
		{"job-build-failure.log", []string{ciEvBuildFailed},
			"a package did not compile. Real bytes from go1.26.5: `build-output`, `build-fail`, and a package terminal of `fail`. A suite that did not build ran no tests whatever the job concluded, and this is named separately from an ordinary failure because the two send a maintainer to different places"},
		{"job-no-test-files.log", []string{ciEvNoTestsAtAll},
			"every package in the account reports `[no test files]` and terminates `skip`. No package passed over zero tests — there is nothing for that rule to catch — and yet the suite executed nothing at all. Without this rule a module whose tests were deleted wholesale would clear"},
		{"job-foreign-json.log", []string{ciEvUnattributable},
			"a step printed an unrelated tool's JSON that happens to carry an `Action` key. This reader cannot tell that from a malformed event and does not try: an object it cannot attribute to a package makes the account ambiguous and the run is refused. Failing closed on an unrecognised line is the deliberate choice — the alternative is a reader that silently drops what it does not understand"},
		{"job-replayed-twice.log", []string{ciEvAmbiguous},
			"a captured stream was replayed twice inside one block, so every test opens and closes twice. This is the one duplicate the terminal-without-a-start rule does not catch on its own — run/pass/run/pass is individually well formed at every step — and counting it would inflate a suite's test count with tests that ran once"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			_, findings := ciEvAdjudicate(tc.fixture, ciEvFixture(t, tc.fixture))
			for _, code := range tc.codes {
				if !ciEvHasCode(findings, code) {
					t.Fatalf("%s produced %v, which does not include %q.\nWhat this fixture records: %s", tc.fixture, ciEvCodes(findings), code, tc.why)
				}
			}
		})
	}
}

// TestAMissingInstantiationIsAFailureNotAnAbsence holds clause 4 of the
// invariant: the accounting unit is the run job, not the declared suite.
//
// A total over the whole run is satisfied by five healthy matrix cells while the
// sixth contributes nothing, and "one record per declared suite" has the same
// hole one level down. The recorded jobs listing with one cell removed is what a
// step-level `if: runner.os == 'Linux'` looks like from the API — every job still
// concluding success, one instantiation simply not there.
//
// THERE ARE TWO WAYS FOR A CELL TO HAVE NO ACCOUNT and each fixture pins exactly
// one of them, asserting the OTHER code's absence as well. That is not
// thoroughness for its own sake: the two guards are one `if` apart, and while
// they shared a code they masked each other. Disabling the absent-job branch left
// the removed-cell listing still producing a finding — the zero-value job it then
// fell through with reports a status that is not `completed` — so either guard
// could be deleted with this test green, which is a guard that pins nothing.
//
// The second fixture is the one with no natural shape in a finished run:
// jobs-pending-cell.json is the recorded listing with one cell flipped to
// `in_progress` / `conclusion: null`, which is what this gate sees when it is run
// while CI is still working — reachable at exactly the moment the gate runs, and
// the reason the sibling `ci-on-merge-commit` check already warns that a check on
// the merge commit can still be pending.
func TestAMissingInstantiationIsAFailureNotAnAbsence(t *testing.T) {
	wf := ciLoadWorkflow(t, ciWorkflowPath)
	suites := ciEvDeclaredSuites(t, wf)

	for _, tc := range []struct {
		fixture string
		want    []string
		absent  []string
		why     string
	}{
		{"jobs.json", nil, []string{ciEvMissingCell, ciEvIncompleteCell},
			"every cell the workflow declares is present and completed. A run like this must clear, or the check reds on ordinary days and gets switched off"},
		{"jobs-missing-cell.json", []string{ciEvMissingCell}, []string{ciEvIncompleteCell},
			"one matrix cell is absent from the listing entirely — what a step-level `if: runner.os == 'Linux'` or a never-created job looks like from the API. Five healthy cells must not stand in for the sixth, and this is NOT the not-yet-finished case: the job is not there at all"},
		{"jobs-pending-cell.json", []string{ciEvIncompleteCell}, []string{ciEvMissingCell},
			"one cell is present and still running. Its log holds a partial account or none, so there is nothing final to read — and a gate that shrugged at a cell in progress would clear a release over a suite that had not finished. This is NOT the absent case: the job exists and the run simply is not done"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			present := map[string]ciEvJob{}
			for _, job := range ciEvLoadJobsFixture(t, tc.fixture) {
				present[job.Name] = job
			}
			var findings []ciEvFinding
			for _, suite := range suites {
				findings = append(findings, ciEvMissingInstantiations(suite, present)...)
			}
			if len(tc.want) == 0 && len(findings) != 0 {
				t.Fatalf("%s reported %v. What this listing records: %s", tc.fixture, findings, tc.why)
			}
			for _, code := range tc.want {
				if !ciEvHasCode(findings, code) {
					t.Fatalf("%s produced %v, which does not include %q.\nWhat this listing records: %s", tc.fixture, ciEvCodes(findings), code, tc.why)
				}
			}
			for _, code := range tc.absent {
				if ciEvHasCode(findings, code) {
					t.Fatalf("%s produced %q, which is the OTHER cell-level finding and not the one this listing records.\nA cell that is absent and a cell that is unfinished are different things to a maintainer — one is a workflow that stopped instantiating, the other is a gate run too early — and reporting either as the other sends them to the wrong place. It also means the two guards can stand in for each other, which is how one of them comes to be deletable with this test green.\nWhat this listing records: %s",
						tc.fixture, code, tc.why)
				}
			}
		})
	}
}

// ciEvMissingInstantiations reports every declared instantiation the run does not
// carry a job for.
func ciEvMissingInstantiations(suite ciEvSuite, present map[string]ciEvJob) []ciEvFinding {
	var out []ciEvFinding
	label := suite.Module
	if label == "" {
		label = "the root module"
	}
	for _, name := range suite.Instantiations {
		job, ok := present[name]
		if !ok {
			out = append(out, ciEvFinding{ciEvMissingCell, name, fmt.Sprintf(
				"%s's suite is declared to run as %q and the run reports no job by that name. A skipped or never-created instantiation leaves every other job concluding success, so nothing above the account can see it: the engine is then verified on the platforms that happened to run and on no others",
				label, name)})
			continue
		}
		if job.Status != "completed" {
			out = append(out, ciEvFinding{ciEvIncompleteCell, name, fmt.Sprintf(
				"instantiation %q is present in the run but reports status %q rather than `completed`, so its account is not final and this stage does not fetch its log. A gate that read it anyway would be reporting on a suite still running; a gate that ignored it would be clearing a release on a cell that has accounted for nothing yet. It is reachable at exactly the moment this gate runs — the sibling `ci-on-merge-commit` check exists because a check on the merge commit can still be pending — and the answer is to wait for the run and ask again, not to tag over it", name, job.Status)})
		}
	}
	return out
}

// TestAccountsFromOneSuiteMustAgreeOnTheirPackageSet is the completeness rule a
// per-log check cannot supply on its own.
//
// A truncation that lands exactly on a package boundary leaves an account that is
// internally well formed and simply short, and nothing inside one log can tell
// that from a suite with fewer packages. Across the six cells of one suite it is
// obvious: they run the same module and must report the same packages. The same
// comparison catches a platform-conditional narrowing that leaves a package out
// on one operating system.
//
// WHAT IT COSTS, said plainly: a package that legitimately exists on only some
// platforms would red this, and the answer is a deliberate edit here with a
// reason, not a quiet exemption — the alternative is a check that cannot tell a
// missing package from an intended one.
func TestAccountsFromOneSuiteMustAgreeOnTheirPackageSet(t *testing.T) {
	full, findings := ciEvAdjudicate("a", ciEvFixture(t, "job-healthy.log"))
	if len(findings) != 0 {
		t.Fatalf("the healthy fixture no longer clears (%v), so this test's subject is gone", findings)
	}
	short := ciEvAccount{Packages: full.Packages[:len(full.Packages)-1], Tests: full.Tests}

	if got := ciEvPackageSetDisagreements("root", map[string]ciEvAccount{"cell-a": full, "cell-b": full}); len(got) != 0 {
		t.Fatalf("two identical accounts disagreed: %v. A comparison that fires on agreement fires on every healthy run", got)
	}
	got := ciEvPackageSetDisagreements("root", map[string]ciEvAccount{"cell-a": full, "cell-b": short})
	if !ciEvHasCode(got, ciEvPackageSetDiffers) {
		t.Fatalf("an account missing a package agreed with a complete one: %v. That is a truncation on a package boundary, and it reads as a shorter pass", ciEvCodes(got))
	}
}

// ciEvPackageSetDisagreements compares the package sets every instantiation of
// one suite reported.
func ciEvPackageSetDisagreements(label string, accounts map[string]ciEvAccount) []ciEvFinding {
	if len(accounts) < 2 {
		return nil
	}
	names := make([]string, 0, len(accounts))
	for n := range accounts {
		names = append(names, n)
	}
	sort.Strings(names)

	union := map[string]bool{}
	for _, n := range names {
		for _, p := range accounts[n].Packages {
			union[p] = true
		}
	}
	var out []ciEvFinding
	for _, n := range names {
		have := map[string]bool{}
		for _, p := range accounts[n].Packages {
			have[p] = true
		}
		var missing []string
		for p := range union {
			if !have[p] {
				missing = append(missing, p)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			out = append(out, ciEvFinding{ciEvPackageSetDiffers, n, fmt.Sprintf(
				"instantiation %q of %s accounts for %d package(s) while its siblings between them account for %d: %s are missing here. Every cell runs the same module, so a short account is either a log this reader received truncated or a package that stopped being built on this platform — and both are things a release must not be tagged over",
				n, label, len(accounts[n].Packages), len(union), strings.Join(missing, ", "))})
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// The live stage.
// ---------------------------------------------------------------------------

// ciEvSelectRun picks the CI run for a sha out of everything the API returned
// for it, or says why it will not.
//
// THE WORKFLOW IS IDENTIFIED BY ITS PATH, an API-supplied fact, and not by the
// `name:` at the top of the file, which is a string somebody can change. And
// anything other than exactly one run is a finding: zero is a workflow whose
// triggers or `paths:` filter stopped matching — a commit with nothing to report
// reads as a commit with nothing wrong — and is also what an audit of an older
// release meets once GitHub's retention has expired. More than one is a question
// about which account is the account, and it is not answered by picking the
// first.
func ciEvSelectRun(sha string, runs []ciEvRun) (ciEvRun, []ciEvFinding) {
	var mine []ciEvRun
	for _, run := range runs {
		if run.Path == ciWorkflowPath {
			mine = append(mine, run)
		}
	}
	switch len(mine) {
	case 1:
		return mine[0], nil
	case 0:
		other := make([]string, 0, len(runs))
		for _, r := range runs {
			other = append(other, r.Path)
		}
		return ciEvRun{}, []ciEvFinding{{ciEvNoRun, sha, fmt.Sprintf(
			"no run of %s exists for %s (%d run(s) of other workflows do: %v). A workflow whose triggers or `paths:` filter stopped matching produces no run at all, and a commit with nothing to report reads as a commit with nothing wrong. It is also what an audit of an older release meets once GitHub's log retention has expired — a designed failure, and a failure either way",
			ciWorkflowPath, sha, len(runs), other)}}
	default:
		names := make([]string, 0, len(mine))
		for _, r := range mine {
			names = append(names, fmt.Sprintf("%d (event %s, attempt %d)", r.ID, r.Event, r.RunAttempt))
		}
		return ciEvRun{}, []ciEvFinding{{ciEvAmbiguousRun, sha, fmt.Sprintf(
			"%d runs of %s exist for %s: %s. Which of them is the account is not a question this reader answers by picking one",
			len(mine), ciWorkflowPath, sha, strings.Join(names, ", "))}}
	}
}

// TestTheRunIsChosenByShaAndByWorkflowPath holds that selection against real
// captures. The healthy one is verbatim `/actions/runs?head_sha=<v0.5.0's merge
// commit>`: four runs, of which exactly one is CI — so the selection has to
// discriminate rather than take what it is given, and a reader that took the
// first would have picked the Release workflow.
func TestTheRunIsChosenByShaAndByWorkflowPath(t *testing.T) {
	load := func(name string) []ciEvRun {
		var page ciEvRunsPage
		if err := json.Unmarshal([]byte(ciEvFixture(t, name)), &page); err != nil {
			t.Fatalf("parse fixture %s: %v", name, err)
		}
		if len(page.WorkflowRuns) != page.TotalCount {
			t.Fatalf("fixture %s carries %d run(s) against a total_count of %d", name, len(page.WorkflowRuns), page.TotalCount)
		}
		return page.WorkflowRuns
	}

	real := load("runs-by-sha.json")
	if len(real) < 2 {
		t.Fatalf("the recorded listing holds %d run(s). It is meant to hold several workflows' runs for one commit — that is what makes choosing one a choice", len(real))
	}
	run, findings := ciEvSelectRun("3217a48b", real)
	if len(findings) != 0 {
		t.Fatalf("the recorded listing for a real merge commit did not yield a run: %v", findings)
	}
	if run.Path != ciWorkflowPath {
		t.Fatalf("selected a run of %s rather than %s. The listing holds the Release, Deploy site and CodeQL runs for the same commit, and any of them concluding success says nothing about the suites", run.Path, ciWorkflowPath)
	}

	if _, f := ciEvSelectRun("3217a48b", load("runs-none.json")); !ciEvHasCode(f, ciEvNoRun) {
		t.Fatalf("a commit with no runs at all produced %v rather than %q. A commit with nothing to report is the easiest thing in this whole file to mistake for a commit with nothing wrong", ciEvCodes(f), ciEvNoRun)
	}
	if _, f := ciEvSelectRun("3217a48b", append(append([]ciEvRun{}, real...), real...)); !ciEvHasCode(f, ciEvAmbiguousRun) {
		t.Fatalf("two CI runs for one commit produced %v rather than %q. Re-runs and a push racing a pull_request both make this reachable, and picking one would be an answer nobody decided", ciEvCodes(f), ciEvAmbiguousRun)
	}
}

type ciEvSuiteRecord struct {
	Module         string                 `json:"module"`
	Job            string                 `json:"job"`
	Instantiations []string               `json:"instantiations"`
	Accounts       map[string]ciEvAccount `json:"accounts"`
	Conclusions    map[string]string      `json:"conclusions_recorded_not_adjudicated"`
}

// The two verdicts the record can carry. They are constants rather than literals
// in three places because the record is read outside this file — a human reads it
// at docs/RELEASING.md's CI item, and the driver scans it for the object it is
// about to publish — so the spelling is part of the record's contract, not a
// detail of how this stage happens to print.
const (
	ciEvVerdictPass   = "PASS"
	ciEvVerdictFailed = "FAILED"
)

type ciEvRecord struct {
	Sha        string            `json:"sha"`
	Repo       string            `json:"repo"`
	Workflow   string            `json:"workflow"`
	RunID      int64             `json:"run_id"`
	RunAttempt int               `json:"run_attempt"`
	Suites     []ciEvSuiteRecord `json:"suites"`
	Findings   []ciEvFinding     `json:"findings"`
	Verdict    string            `json:"verdict"`
	WrittenBy  string            `json:"written_by"`
}

// ciEvRepoSlug derives owner/repo from go.mod rather than writing it down, so a
// fork gates itself rather than the upstream it was forked from.
func ciEvRepoSlug(t *testing.T) string {
	t.Helper()
	for _, line := range strings.Split(wiringReadFile(t, "go.mod"), "\n") {
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		if !strings.HasPrefix(mod, "github.com/") {
			t.Fatalf("go.mod's module path is %q, which is not a github.com path. This stage fetches the run from the GitHub API and has no way to name the repository otherwise", mod)
		}
		parts := strings.Split(strings.TrimPrefix(mod, "github.com/"), "/")
		if len(parts) < 2 {
			t.Fatalf("go.mod's module path %q does not name an owner and a repository", mod)
		}
		return parts[0] + "/" + parts[1]
	}
	t.Fatal("go.mod declares no module path")
	return ""
}

// ciEvGH runs one GitHub API request. A missing or unauthenticated `gh` is a
// FAILURE and never a skip: CLAUDE.md is explicit that a check which cannot
// execute is reported and fails, because there is no result that means "we did
// not check" and reads as "it is fine".
func ciEvGH(t *testing.T, path string) []byte {
	t.Helper()
	out, err := exec.Command("gh", "api", path).Output()
	if err != nil {
		detail := ""
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			detail = "\nstderr: " + strings.TrimSpace(string(ee.Stderr))
		}
		t.Fatalf("`gh api %s` failed: %v%s\n"+
			"The account this gate reads exists only in the run, so a request that could not be made is a check that could not run — which is a FAILED gate. Install and authenticate the GitHub CLI (`gh auth login`) and run it again; do not tag on the strength of a request that was never answered",
			path, err, detail)
	}
	return out
}

// ciEvFetchRuns returns every workflow run for a sha, following pages until the
// API's own total_count is accounted for.
//
// "Some of the runs, because the first page held them" is exactly the silent
// narrowing CLAUDE.md forbids, so the loop is written against total_count and a
// page that returns nothing before the total is reached is a failure rather than
// the end of the list.
func ciEvFetchRuns(t *testing.T, slug, sha string) []ciEvRun {
	t.Helper()
	var all []ciEvRun
	total := -1
	for page := 1; page <= 50; page++ {
		var got ciEvRunsPage
		raw := ciEvGH(t, fmt.Sprintf("repos/%s/actions/runs?head_sha=%s&per_page=100&page=%d", slug, sha, page))
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("parse the runs listing for %s page %d: %v", sha, page, err)
		}
		if total < 0 {
			total = got.TotalCount
		}
		all = append(all, got.WorkflowRuns...)
		if len(all) >= total {
			return all
		}
		if len(got.WorkflowRuns) == 0 {
			t.Fatalf("the runs listing for %s reports total_count %d but stopped returning runs after %d. A partial listing would let this gate clear a release on the suites that happened to land on the first page", sha, total, len(all))
		}
	}
	t.Fatalf("the runs listing for %s did not terminate within 50 pages", sha)
	return nil
}

func ciEvFetchJobs(t *testing.T, slug string, runID int64) []ciEvJob {
	t.Helper()
	var all []ciEvJob
	total := -1
	for page := 1; page <= 50; page++ {
		var got ciEvJobsPage
		raw := ciEvGH(t, fmt.Sprintf("repos/%s/actions/runs/%d/jobs?per_page=100&page=%d", slug, runID, page))
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("parse the jobs listing for run %d page %d: %v", runID, page, err)
		}
		if total < 0 {
			total = got.TotalCount
		}
		all = append(all, got.Jobs...)
		if len(all) >= total {
			return all
		}
		if len(got.Jobs) == 0 {
			t.Fatalf("the jobs listing for run %d reports total_count %d but stopped returning jobs after %d. A declared instantiation missing from a truncated page is indistinguishable from one that never ran, and this check refuses to guess which it is", runID, total, len(all))
		}
	}
	t.Fatalf("the jobs listing for run %d did not terminate within 50 pages", runID)
	return nil
}

// ---------------------------------------------------------------------------
// The assembly: the one gating decision, and the seam that lets it be examined.
//
// Every stage below this line had a test before this seam existed — the run
// selection, the matrix expansion, the adjudicator, the missing-cell guard, the
// package-set comparison — and the WIRING BETWEEN THEM had none. The parts were
// proven and the whole was not, which is this file's own subject turned on
// itself one level up: sever the decide stage from the fetch and parse stages,
// or hand the record an account nothing ever parsed, and the release's single
// gating decision reports PASS with every constituent test still green. Both
// were reproduced against this file before the seam landed: dropping the one
// line that carries the adjudicator's findings into the verdict, and deleting
// the fetch-and-adjudicate block outright, each left `go test ./tests` fully
// green.
//
// So the assembly is a function over a SOURCE rather than a body that calls the
// GitHub CLI inline. The live source is the same three requests as before; the
// fixture source in TestTheStageConsumesEverythingItFetched serves recorded
// accounts and records what was asked for, which is what makes "did the decision
// consume what it fetched" a question with an answer.
// ---------------------------------------------------------------------------

// ciEvSource is the three fetches the assembly makes, and the ONLY way it
// reaches the run. It is an interface so the assembly can be driven by recorded
// accounts; it is deliberately not an escape hatch in the stage itself — nothing
// at release time chooses a source, TestReleaseGateCIRunEvidence constructs the
// live one unconditionally, and a fixture source reachable from a release-time
// invocation would be a gate that could be pointed away from the run.
type ciEvSource interface {
	Runs(t *testing.T, sha string) []ciEvRun
	Jobs(t *testing.T, runID int64) []ciEvJob
	JobLog(t *testing.T, jobID int64) string
}

// ciEvLiveSource is the release-time source: the GitHub API, through `gh`, with
// a missing or unauthenticated CLI a failure rather than a skip.
type ciEvLiveSource struct{ slug string }

func (s ciEvLiveSource) Runs(t *testing.T, sha string) []ciEvRun {
	return ciEvFetchRuns(t, s.slug, sha)
}

func (s ciEvLiveSource) Jobs(t *testing.T, runID int64) []ciEvJob {
	return ciEvFetchJobs(t, s.slug, runID)
}

func (s ciEvLiveSource) JobLog(t *testing.T, jobID int64) string {
	return string(ciEvGH(t, fmt.Sprintf("repos/%s/actions/jobs/%d/logs", s.slug, jobID)))
}

// ciEvAssemble is the whole gating decision: choose the run for the sha, list
// its jobs, require an account for every declared instantiation, adjudicate each
// one, compare the accounts across a suite's cells, and record what all of that
// found. The verdict is PASS only when nothing did.
func ciEvAssemble(t *testing.T, src ciEvSource, slug, sha string, suites []ciEvSuite) ciEvRecord {
	t.Helper()

	record := ciEvRecord{Sha: sha, Repo: slug, Workflow: ciWorkflowPath, WrittenBy: ciEvStageTestName, Verdict: ciEvVerdictFailed}
	var findings []ciEvFinding

	// Which run. Keyed to the sha, never to "the newest run on main": commits
	// land on main between the merge and the tag as a matter of routine, and a
	// run fetched by branch is evidence about a tree that is not being tagged.
	run, runFindings := ciEvSelectRun(sha, src.Runs(t, sha))
	findings = append(findings, runFindings...)
	record.RunID, record.RunAttempt = run.ID, run.RunAttempt

	if record.RunID != 0 {
		present := map[string]ciEvJob{}
		for _, job := range src.Jobs(t, record.RunID) {
			present[job.Name] = job
		}
		for _, suite := range suites {
			rec := ciEvSuiteRecord{
				Module: suite.Module, Job: suite.JobKey, Instantiations: suite.Instantiations,
				Accounts: map[string]ciEvAccount{}, Conclusions: map[string]string{},
			}
			missing := ciEvMissingInstantiations(suite, present)
			findings = append(findings, missing...)
			for _, name := range suite.Instantiations {
				job, ok := present[name]
				if !ok || job.Status != "completed" {
					continue
				}
				rec.Conclusions[name] = job.Conclusion
				acct, f := ciEvAdjudicate(name, src.JobLog(t, job.ID))
				rec.Accounts[name] = acct
				findings = append(findings, f...)
			}
			label := suite.Module
			if label == "" {
				label = "the root module"
			}
			findings = append(findings, ciEvPackageSetDisagreements(label, rec.Accounts)...)
			record.Suites = append(record.Suites, rec)
		}
	}

	record.Findings = findings
	if len(findings) == 0 {
		record.Verdict = ciEvVerdictPass
	}
	return record
}

// ciEvGateFailure turns a finished record into the stage's release-time answer:
// the empty string when the release may proceed on this evidence, and the text
// of the failure otherwise.
//
// IT IS A FUNCTION RATHER THAN AN `if` INSIDE THE STAGE for the same reason the
// assembly is: a decision reached and then not acted on is the last place the
// severing could hide. A record carrying findings, or carrying a verdict that is
// not PASS, must never leave this stage as a pass — including the case that
// should be impossible, where the verdict says FAILED and no finding says why,
// which is a decision nobody can review and is treated as the failure it is.
func ciEvGateFailure(record ciEvRecord, out string) string {
	if record.Verdict == ciEvVerdictPass && len(record.Findings) == 0 {
		return ""
	}
	lines := make([]string, 0, len(record.Findings))
	for _, f := range record.Findings {
		lines = append(lines, "  - "+f.String())
	}
	if len(lines) == 0 {
		lines = append(lines, fmt.Sprintf("  - the verdict is %q and the record names no finding at all, so this stage decided against evidence it did not record. A gate nobody can review is not a gate that passed", record.Verdict))
	}
	return fmt.Sprintf("the CI run for %s does not establish that the declared suites executed and passed — %d finding(s):\n%s\n\nThe verdict record is at %s. A finding here is not waived by re-running this check; it is waived by a new run of CI over a fixed tree",
		record.Sha, len(record.Findings), strings.Join(lines, "\n"), out)
}

// ---------------------------------------------------------------------------
// The assembly test.
// ---------------------------------------------------------------------------

// ciEvFixtureSource serves recorded accounts to the assembly and records what
// the assembly asked for.
//
// WHAT IT ASKS FOR IS HALF THE ASSERTION. A run id it did not select, a job id
// that is in no listing it read, or a sha nobody named are each a fatal here
// rather than a lookup that quietly returns nothing — an assembly fetching logs
// for a run other than the one it chose would otherwise adjudicate real accounts
// belonging to another commit and clear the release on them.
type ciEvFixtureSource struct {
	sha      string
	runs     []ciEvRun
	runID    int64
	jobs     []ciEvJob
	logs     map[int64]string
	fetched  []string // instantiation names whose logs were actually asked for
	runsRead int
	jobsRead int
}

func (s *ciEvFixtureSource) Runs(t *testing.T, sha string) []ciEvRun {
	t.Helper()
	if sha != s.sha {
		t.Fatalf("the assembly asked for the runs of %q; the commit it was given is %q. Evidence fetched for another commit is evidence about a tree that is not being tagged", sha, s.sha)
	}
	s.runsRead++
	return s.runs
}

func (s *ciEvFixtureSource) Jobs(t *testing.T, runID int64) []ciEvJob {
	t.Helper()
	if runID != s.runID {
		t.Fatalf("the assembly listed the jobs of run %d, which is not the run it selected for this commit (%d)", runID, s.runID)
	}
	s.jobsRead++
	return s.jobs
}

func (s *ciEvFixtureSource) JobLog(t *testing.T, jobID int64) string {
	t.Helper()
	log, ok := s.logs[jobID]
	if !ok {
		t.Fatalf("the assembly fetched the log of job %d, which appears in no listing it was given", jobID)
	}
	for _, job := range s.jobs {
		if job.ID == jobID {
			s.fetched = append(s.fetched, job.Name)
		}
	}
	return log
}

// ciEvNewFixtureSource builds a source out of recorded captures. `logs` maps an
// instantiation's job NAME to the log fixture it should serve; the entry under
// the empty key is what every other job gets.
func ciEvNewFixtureSource(t *testing.T, sha, runsFixture, jobsFixture string, logs map[string]string) *ciEvFixtureSource {
	t.Helper()

	var runsPage ciEvRunsPage
	if err := json.Unmarshal([]byte(ciEvFixture(t, runsFixture)), &runsPage); err != nil {
		t.Fatalf("parse fixture %s: %v", runsFixture, err)
	}
	src := &ciEvFixtureSource{sha: sha, runs: runsPage.WorkflowRuns, logs: map[int64]string{}}
	for _, run := range runsPage.WorkflowRuns {
		if run.Path == ciWorkflowPath {
			src.runID = run.ID
		}
	}
	if jobsFixture == "" {
		return src
	}
	src.jobs = ciEvLoadJobsFixture(t, jobsFixture)
	for _, job := range src.jobs {
		name, ok := logs[job.Name]
		if !ok {
			name = logs[""]
		}
		if name == "" {
			continue
		}
		src.logs[job.ID] = ciEvFixture(t, name)
	}
	return src
}

// TestTheStageConsumesEverythingItFetched holds the ASSEMBLY — the single gating
// decision a release actually consumes — to the same standard every stage inside
// it is already held to.
//
// THE FAILURE IT EXISTS FOR, and it was reproduced against this file rather than
// imagined. Every constituent here goes red under mutation: break the
// adjudicator and eleven recorded defects red, break the run selection and three
// captures red, break the matrix expansion and fourteen refusals red. The
// assembly that wires them together had no test at all, because the only
// function that ran it was the live stage — which SKIPS on a developer's laptop
// and in CI, so its body was executed by nothing that ever ran. Two blindings
// were applied to it and `go test ./tests` stayed green for both: dropping the
// single line that carries the adjudicator's findings into the verdict, so every
// zero-test and failed-test account was parsed and thrown away; and deleting the
// fetch-and-adjudicate block entirely, so the stage examined no job at all and
// wrote a PASS record for any commit. Either edit is one line of a plausible
// refactor, and after either one the gate that this whole file exists to be
// clears every release forever.
//
// WHAT IT ASSERTS, in the two directions that matter. That the accounts written
// into the record are the accounts the fetched logs actually hold — compared
// against an independent adjudication of the same bytes, so a record filled from
// anywhere else is a failure. And that every finding a constituent produces
// reaches the verdict and the release-time answer: a cell that ran no tests, a
// cell whose test failed, a cell absent from the listing, a cell still running, a
// cell whose package set does not match its siblings, and a commit with no CI run
// at all. It also asserts WHICH logs were fetched, exactly — an assembly that
// adjudicated the first cell and stopped would otherwise satisfy every one of
// those.
func TestTheStageConsumesEverythingItFetched(t *testing.T) {
	const (
		sha  = "3217a48b4a123ea4b8b02f93fac6337b985eb7ce"
		slug = "BarterX-Tech/dossierx"
	)
	suites := ciEvDeclaredSuites(t, ciLoadWorkflow(t, ciWorkflowPath))

	// The instantiations the workflow declares, which is what an account is owed
	// for. Derived here rather than written down, for the reason clause 5 of the
	// invariant gives: a written list goes stale the first time a job is added.
	var declared []string
	for _, suite := range suites {
		declared = append(declared, suite.Instantiations...)
	}
	sort.Strings(declared)
	if len(declared) < 2 {
		t.Fatalf("the workflow declares %d instantiation(s) (%v); the root suite alone declares six, so this test has lost its subject", len(declared), declared)
	}

	healthy := ciEvFixture(t, "job-healthy.log")
	healthyAcct, healthyFindings := ciEvAdjudicate("reference", healthy)
	if len(healthyFindings) != 0 || healthyAcct.Tests == 0 {
		t.Fatalf("the healthy fixture no longer clears with tests in it (%d test(s), %v), so every case below would be comparing against a broken reference", healthyAcct.Tests, healthyFindings)
	}

	for _, tc := range []struct {
		name        string
		runs        string
		jobs        string
		logs        map[string]string
		wantVerdict string
		wantCodes   []string
		wantAt      map[string]string // finding code -> the instantiation it must name
		wantFetched []string          // nil means "every declared instantiation, and nothing else"
		why         string
	}{
		{
			name: "a healthy run clears, having fetched an account for every declared instantiation",
			runs: "runs-by-sha.json", jobs: "jobs.json",
			logs:        map[string]string{"": "job-healthy.log"},
			wantVerdict: ciEvVerdictPass,
			why:         "the ordinary day. Without this case every assertion below would be satisfied by an assembly that failed everything, and a gate that reds an ordinary day is a gate that gets switched off",
		},
		{
			name: "one cell executed no tests",
			runs: "runs-by-sha.json", jobs: "jobs.json",
			logs:        map[string]string{"": "job-healthy.log", "test (go stable, windows-latest)": "job-zero-tests.log"},
			wantVerdict: ciEvVerdictFailed,
			wantCodes:   []string{ciEvPackageRanNoTests},
			wantAt:      map[string]string{ciEvPackageRanNoTests: "test (go stable, windows-latest)"},
			why:         "the lane's own target failure, arriving through the assembly rather than at the adjudicator's doorstep. This is the case that dies if the line carrying the adjudicator's findings into the verdict is dropped — which was reproduced, and left the whole suite green",
		},
		{
			name: "one cell failed a test",
			runs: "runs-by-sha.json", jobs: "jobs.json",
			logs:        map[string]string{"": "job-healthy.log", "test (go 1.26.x, macos-latest)": "job-failed-test.log"},
			wantVerdict: ciEvVerdictFailed,
			wantCodes:   []string{ciEvTestFailed},
			wantAt:      map[string]string{ciEvTestFailed: "test (go 1.26.x, macos-latest)"},
			why:         "a failure read from the test binary's own account. The job concluded success in this listing — it is the recorded healthy listing — so an assembly that let a conclusion anywhere near the verdict would clear this",
		},
		{
			name: "one cell reports a different package set from its siblings",
			runs: "runs-by-sha.json", jobs: "jobs.json",
			logs:        map[string]string{"": "job-healthy.log", "test (go stable, ubuntu-latest)": "job-no-test-files.log"},
			wantVerdict: ciEvVerdictFailed,
			wantCodes:   []string{ciEvPackageSetDiffers, ciEvNoTestsAtAll},
			why:         "the cross-cell comparison is the only thing that sees a truncation landing on a package boundary, and it is reached from nowhere but here: it takes the accounts of one suite's cells, which only the assembly assembles",
		},
		{
			name: "a declared cell is absent from the run",
			runs: "runs-by-sha.json", jobs: "jobs-missing-cell.json",
			logs:        map[string]string{"": "job-healthy.log"},
			wantVerdict: ciEvVerdictFailed,
			wantCodes:   []string{ciEvMissingCell},
			wantAt:      map[string]string{ciEvMissingCell: "test (go stable, windows-latest)"},
			wantFetched: ciEvWithout(declared, "test (go stable, windows-latest)"),
			why:         "five healthy cells must not stand in for the sixth. The fetched list is asserted too: the absent cell has no log to read, and an assembly that invented one would be adjudicating something nobody ran",
		},
		{
			name: "a declared cell is still running",
			runs: "runs-by-sha.json", jobs: "jobs-pending-cell.json",
			logs:        map[string]string{"": "job-healthy.log"},
			wantVerdict: ciEvVerdictFailed,
			wantCodes:   []string{ciEvIncompleteCell},
			wantAt:      map[string]string{ciEvIncompleteCell: "test (go stable, windows-latest)"},
			wantFetched: ciEvWithout(declared, "test (go stable, windows-latest)"),
			why:         "reachable at exactly the moment this gate runs. Its log holds a partial account or none, so the assembly must not read it — and must not shrug at the cell either",
		},
		{
			name: "the commit has no CI run at all",
			runs: "runs-none.json", jobs: "",
			wantVerdict: ciEvVerdictFailed,
			wantCodes:   []string{ciEvNoRun},
			wantFetched: []string{},
			why:         "a commit with nothing to report reads as a commit with nothing wrong. The assembly must stop here — no jobs listed, no logs read — and it must stop by FAILING, which is what an assembly that returned its zero record would not do",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := ciEvNewFixtureSource(t, sha, tc.runs, tc.jobs, tc.logs)
			record := ciEvAssemble(t, src, slug, sha, suites)

			if record.Verdict != tc.wantVerdict {
				t.Fatalf("the assembly returned verdict %q, want %q, over findings %v.\nWhy this case is here: %s", record.Verdict, tc.wantVerdict, ciEvCodes(record.Findings), tc.why)
			}
			if record.Sha != sha {
				t.Errorf("the record names commit %q rather than the one the assembly was asked about (%q). A record about another commit is what the Makefile's own `grep` guard exists to catch, and it must not be produced in the first place", record.Sha, sha)
			}

			// The release-time answer, which is what the stage acts on. A verdict
			// computed and then not consumed is the last place the severing hides.
			failure := ciEvGateFailure(record, "/tmp/record.json")
			if tc.wantVerdict == ciEvVerdictPass && failure != "" {
				t.Fatalf("a clearing record still produced a release-time failure:\n%s", failure)
			}
			if tc.wantVerdict != ciEvVerdictPass && failure == "" {
				t.Fatalf("a record carrying %v left this stage as a PASS. The findings were produced, recorded — and then not acted on.\nWhy this case is here: %s", ciEvCodes(record.Findings), tc.why)
			}

			for _, code := range tc.wantCodes {
				if !ciEvHasCode(record.Findings, code) {
					t.Fatalf("the assembly reported %v, which does not include %q.\nThe stage that produces it has its own test and passes it; what this case asserts is that its finding REACHES the one decision a release consumes.\nWhy this case is here: %s", ciEvCodes(record.Findings), code, tc.why)
				}
				if !strings.Contains(failure, code) {
					t.Errorf("the release-time failure text does not name %q, so a maintainer reading the gate's own output cannot see what it found:\n%s", code, failure)
				}
			}
			for code, where := range tc.wantAt {
				found := false
				for _, f := range record.Findings {
					if f.Code == code && f.Where == where {
						found = true
					}
				}
				if !found {
					t.Errorf("no %q finding names instantiation %q. The accounting unit is the run job: a finding that reaches the verdict without saying WHICH cell produced it is a gate that cannot be acted on, and it is also how one cell's account comes to stand in for another's.\nThe findings were: %v", code, where, record.Findings)
				}
			}

			// WHICH ACCOUNTS THE RECORD HOLDS, compared against an independent
			// adjudication of the very bytes the source served. This is the half
			// that catches a record filled from anywhere but the logs that were
			// fetched — the assembly handed an account it never parsed.
			for _, suite := range record.Suites {
				for name, got := range suite.Accounts {
					want, _ := ciEvAdjudicate(name, src.logs[ciEvJobID(t, src, name)])
					if !reflect.DeepEqual(got, want) {
						t.Errorf("the record's account for %q is %+v; adjudicating the log the source actually served for it gives %+v.\nThe record must be what was read, not what was expected: an account that does not come from the fetched bytes is evidence about nothing", name, got, want)
					}
				}
			}

			want := tc.wantFetched
			if want == nil {
				want = declared
			}
			got := append([]string{}, src.fetched...)
			sort.Strings(got)
			sorted := append([]string{}, want...)
			sort.Strings(sorted)
			if strings.Join(got, "\n") != strings.Join(sorted, "\n") {
				t.Errorf("the assembly fetched accounts for %v; it owes one for exactly %v.\nToo few and some instantiation was never examined while the gate cleared anyway — the hole clause 4 of the invariant names. Too many and it is reading logs for jobs the workflow does not declare.\nWhy this case is here: %s", got, sorted, tc.why)
			}
		})
	}
}

// ciEvWithout is the declared instantiation list minus one cell — the expected
// fetch set when a cell has no readable account.
func ciEvWithout(names []string, drop string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n != drop {
			out = append(out, n)
		}
	}
	if len(out) == len(names) {
		panic("ciEvWithout: " + drop + " is not a declared instantiation, so this case is asserting against a set it did not change")
	}
	return out
}

// ciEvJobID resolves an instantiation name to the job id the fixture source
// holds for it, so an account in the record can be compared against the bytes
// that were served for that exact job.
func ciEvJobID(t *testing.T, src *ciEvFixtureSource, name string) int64 {
	t.Helper()
	for _, job := range src.jobs {
		if job.Name == name {
			return job.ID
		}
	}
	t.Fatalf("the record carries an account for %q, which is in no jobs listing the assembly read. The account cannot have come from a log this run fetched", name)
	return 0
}

// TestReleaseGateCIRunEvidence is the stage the release procedure invokes.
//
// UNSET, IT SKIPS; NAMED, IT FAILS. That is the pattern
// viewer-tests/harness_test.go:40-59 landed for a check with an external
// prerequisite, and its comment is the reasoning: a skip "is the right answer on
// a developer's laptop and the WRONG one in CI, where it is indistinguishable
// from a pass over zero assertions". The skip is therefore NOT the gate's safety
// net — `make ci-evidence` is, by refusing to exit 0 unless the verdict record
// exists and names the sha it was asked about. `go test` exits 0 for a skip and
// for a selector that matches nothing, so an exit status alone could never have
// been the release-time signal.
func TestReleaseGateCIRunEvidence(t *testing.T) {
	sha := strings.TrimSpace(os.Getenv(ciEvShaEnv))
	out := strings.TrimSpace(os.Getenv(ciEvOutEnv))
	switch mode, why := ciEvGateMode(sha, out); mode {
	case ciEvGateSkip:
		t.Skip(why)
	case ciEvGateRefuse:
		t.Fatal(why)
	}

	wf := ciLoadWorkflow(t, ciWorkflowPath)
	suites := ciEvDeclaredSuites(t, wf)
	slug := ciEvRepoSlug(t)

	// The whole decision, in one call that TestTheStageConsumesEverythingItFetched
	// can drive against recorded accounts. What is left here is what cannot be:
	// the live source, the record on disk, and acting on the answer.
	record := ciEvAssemble(t, ciEvLiveSource{slug: slug}, slug, sha, suites)

	// The record is written BEFORE the verdict is reported, and on a failing run
	// as well as a clearing one: the human reviewing a blocked release needs to
	// see what was examined at least as much as the human clearing one does.
	blob, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal the verdict record: %v", err)
	}
	if err := os.WriteFile(out, append(blob, '\n'), 0o600); err != nil {
		t.Fatalf("write the verdict record to %s: %v\nWithout the record this run cannot be told from one that adjudicated nothing, so it is reported as a failure rather than as a pass nobody can check", out, err)
	}
	t.Logf("verdict record written to %s:\n%s", out, blob)

	if msg := ciEvGateFailure(record, out); msg != "" {
		t.Fatal(msg)
	}
}

// ciEvGateMode decides what a given pair of environment settings means, kept
// separate from the stage so the decision can be exercised without an
// environment — a test that must set process-wide variables to reach a branch is
// a test that cannot reach the other branches in the same run.
//
// The three answers, and why they are not two:
//
//   - NEITHER SET: skip. This is a developer's laptop, where fetching a run for
//     a commit nobody named is not a thing to do. The skip is safe ONLY because
//     `make ci-evidence` refuses to succeed without a verdict record — an exit
//     status could never carry this, since `go test` exits 0 for a skip.
//   - THE SHA WITHOUT THE OUT PATH: refuse. The stage would adjudicate and leave
//     no record of having done so, and a gate whose result nobody can inspect is
//     a gate that reported itself.
//   - THE OUT PATH WITHOUT THE SHA: refuse, and this is the one worth spelling
//     out. Something is plainly invoking this as a gate and has not said which
//     commit — the shape of a release-time invocation whose sha variable was
//     renamed on one side. Skipping there would be the exact silent no-op this
//     file exists to make impossible.
type ciEvGateDecision int

const (
	ciEvGateRun ciEvGateDecision = iota
	ciEvGateSkip
	ciEvGateRefuse
)

func ciEvGateMode(sha, out string) (ciEvGateDecision, string) {
	switch {
	case sha == "" && out == "":
		return ciEvGateSkip, fmt.Sprintf("%s is unset, so there is no commit to fetch an account for. This is the right answer on a developer's laptop and the WRONG one at release time: `make %s` supplies both %s and %s and refuses to exit 0 unless a verdict record was written",
			ciEvShaEnv, ciEvMakeTarget, ciEvShaEnv, ciEvOutEnv)
	case sha == "":
		return ciEvGateRefuse, fmt.Sprintf("%s is set (%q) but %s is not. Something is invoking this stage as a gate and has not told it which commit to examine — and a gate that examines no commit and exits 0 is the failure this whole file exists to refuse. Run it as `make %s`",
			ciEvOutEnv, out, ciEvShaEnv, ciEvMakeTarget)
	case out == "":
		return ciEvGateRefuse, fmt.Sprintf("%s names commit %s but %s is unset, so this stage would adjudicate and leave no record of having done so. The record is the only positive evidence a transporting agent can carry; without it the invocation is indistinguishable from one that ran nothing. Run it as `make %s`",
			ciEvShaEnv, sha, ciEvOutEnv, ciEvMakeTarget)
	}
	return ciEvGateRun, ""
}

// TestTheStageRefusesToBeInvokedHalfway walks that decision, and the case it
// exists for is the third one: a release-time invocation that supplies the
// record path and not the sha must FAIL, not skip. That is what a renamed sha
// variable looks like from inside the stage, and a skip there is a green gate
// over zero examinations — this lane's own subject, turned on itself.
func TestTheStageRefusesToBeInvokedHalfway(t *testing.T) {
	for _, tc := range []struct {
		name, sha, out string
		want           ciEvGateDecision
	}{
		{"a developer's laptop", "", "", ciEvGateSkip},
		{"gated, but nobody said which commit", "", "/tmp/record.json", ciEvGateRefuse},
		{"a commit with nowhere to record the verdict", "deadbeef", "", ciEvGateRefuse},
		{"both supplied", "deadbeef", "/tmp/record.json", ciEvGateRun},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := ciEvGateMode(tc.sha, tc.out)
			if got != tc.want {
				t.Fatalf("ciEvGateMode(%q, %q) = %d, want %d (%s)", tc.sha, tc.out, got, tc.want, why)
			}
			if tc.want != ciEvGateRun && why == "" {
				t.Fatalf("ciEvGateMode(%q, %q) refuses or skips without saying why. A gate that stops without a reason is one somebody deletes", tc.sha, tc.out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The pin.
// ---------------------------------------------------------------------------

// ciEvMakeTargetRE isolates the Makefile recipe for the release-time target: the
// target line and every tab-indented line under it.
var ciEvMakeTargetRE = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(ciEvMakeTarget) + `:[^\n]*\n(?:\t[^\n]*\n?)+`)

// ciEvProcedureItemRE isolates docs/RELEASING.md's "CI is green on `main`" item,
// from its bolded title to the next item at the same level.
var ciEvProcedureItemRE = regexp.MustCompile("(?s)- \\[ \\] \\*\\*CI is green on `main`\\*\\*.*?\\n- \\[ \\] ")

// ciEvProcedureHumanHalf marks where the written item stops describing what the
// machine does and starts telling a person what to look at themselves. C33
// settles that the human half survives this lane, and it is the half whose
// description of the log this lane made stale.
const ciEvProcedureHumanHalf = "**Then open the run**"

// TestTheReleaseTimeInvocationNamesThisStage is the half of the blocking finding
// that holds regardless of how honestly an agent reports.
//
// THE FAILURE IT EXISTS FOR. The identifier that selects this stage drifts from
// the identifier the release-time procedure invokes — an environment variable
// renamed on one side, a test selector edited on the other, one token in one
// file. From then on the invocation runs, adjudicates nothing, exits 0, and the
// release is gated by an examination of no commit at all; this lane's own target
// defect, living at the one place that was supposed to close it. That drift is
// invisible to every other check in this repository, because none of them reads
// two of these documents at once.
//
// It is written as four reads of four documents rather than one, so a failure
// says WHICH side moved.
func TestTheReleaseTimeInvocationNamesThisStage(t *testing.T) {
	// (1) The constant above must name a function that exists. Without this the
	// Makefile could select a test that was renamed away and `go test` would
	// print `ok … [no tests to run]` and exit 0 for every release after.
	stage := wiringReadFile(t, ciEvStageFile)
	if !strings.Contains(stage, "func "+ciEvStageTestName+"(t *testing.T)") {
		t.Fatalf("%s declares no `func %s(t *testing.T)`, but that is the name the Makefile selects and the procedure invokes.\n"+
			"`go test -run` exits 0 when its selector matches nothing — `ok … [no tests to run]` — so a stage renamed out from under its selector does not fail, it silently stops adjudicating and every release after is gated by an invocation that examined no commit",
			ciEvStageFile, ciEvStageTestName)
	}

	// (2) The Makefile target, which is what a human types.
	makefile := wiringReadFile(t, "Makefile")
	recipe := ciEvMakeTargetRE.FindString(makefile)
	if recipe == "" {
		t.Fatalf("the Makefile no longer declares a `%s:` target with a recipe. That target IS the release-time invocation — %s and %s both name it — and it is also the only thing that turns `go test`'s exit 0 over an empty selection into a non-zero exit, by refusing to succeed unless a verdict record was written",
			ciEvMakeTarget, ciEvProcedureFile, ciEvDriverFile)
	}
	//
	// EVERY MECHANISM HERE IS PINNED AS A WHOLE LINE RATHER THAN AS A NAME, and
	// that is the correction of a real defect in this test rather than a
	// preference. The variable names appear all over the recipe — in the guard
	// messages, in the `grep`, in the `rm` — so requiring the NAME proves only
	// that the recipe talks about it. An earlier version of this list pinned
	// DOSSIERX_GATE_CI_EVIDENCE_OUT by name; deleting the line that actually hands
	// the record path to the stage left the bare name present in the two guards
	// below it and the pin stayed green over a recipe missing its own mechanism.
	// Each of these four lines IS a mechanism — hand over the sha, hand over the
	// record path, refuse an empty record, refuse a record about another commit —
	// so each is pinned as the line it is. Make is not a language this file
	// parses, and a text pin is what the landed precedent (gate_receipt_test.go on
	// docs/RELEASING.md) uses for the same reason.
	for _, want := range []struct{ fragment, why string }{
		{ciEvStageTestName, "the recipe selects the stage by name; a recipe that names something else runs a different test, or none, and `go test -run` reports `ok` either way"},
		{ciEvShaEnv + `="$(` + ciEvShaEnv + `)"`, "make does not export its variables to a recipe's environment, so this is the line that actually hands the commit to the stage. Without it the stage sees no sha, SKIPS, and `go test` exits 0"},
		{ciEvOutEnv + `="$(` + ciEvOutEnv + `)"`, "the twin of the line above, and it fails the same way: make exports nothing, so without this line the stage is handed no path to write its record to. It then refuses — the sha is set and the record path is not — but the recipe would be relying on that refusal instead of on doing its job, and the recipe is what the procedure invokes"},
		{`test -s "$(` + ciEvOutEnv + `)"`, "this is the line that turns `go test`'s exit 0 over an empty selection into a failed gate. The verdict record is the only positive evidence the stage adjudicated anything, and without this line an invocation that adjudicated nothing reports a clean gate"},
		{`grep -q "$(` + ciEvShaEnv + `)" "$(` + ciEvOutEnv + `)"`, "a record left over from an earlier release is a file that exists and is not empty, so the emptiness check above clears it. This is the line that requires the record to be about the commit being tagged"},
	} {
		if !strings.Contains(recipe, want.fragment) {
			t.Errorf("the Makefile's `%s` recipe no longer names %q. %s.\nThe recipe reads:\n%s", ciEvMakeTarget, want.fragment, want.why, recipe)
		}
	}

	// (3) THE CONSUMER. A record nobody reads is a file, and this side of the pin
	// is what keeps it from becoming one.
	//
	// It used to be an agent, transporting the record into a checklist phase that
	// treated its absence as blocking. That checklist is retired — it published
	// main before the tag and knew nothing about the driver — and the consumer is
	// now the driver's own D1, which reads the record at ciEvOutEnv and refuses
	// the release unless it names an object carrying the tree about to be
	// published. Neither side may drift alone: rename the variable here and the
	// driver goes looking at a path nothing writes; rename the target and its
	// refusal tells an operator to run a command that does not exist.
	//
	// Pinned as ASSIGNED VALUES rather than as bare names, for the reason the
	// recipe list above gives. Both identifiers are discussed in that file's prose
	// as well as used by it, so `Contains(driver, ciEvOutEnv)` would be satisfied
	// by a comment that mentions this stage while the code reads somewhere else.
	// `= "NAME"` is the declaration.
	driver := wiringReadFile(t, ciEvDriverFile)
	for _, want := range []struct{ fragment, why string }{
		{`= "` + ciEvOutEnv + `"`, "the driver must declare the SAME record path this recipe writes to. Nothing hands it over — make does not export a `?=` default into a recipe's environment — so the driver carries its own spelling of the name and the two are equal only because this line requires it. Drift here does not silently pass a release: it refuses every release with a message about a file nothing writes, which is the same gate broken the other way"},
		{`= "` + ciEvMakeTarget + `"`, "the driver names this target in every refusal about a missing record, because \"the record is absent\" without the command that produces it is a refusal an operator has to go and research — and a researched refusal is one somebody switches off"},
		{ciEvShaEnv + "=", "the driver's recovery must tell an operator to key the stage to a commit. `make " + ciEvMakeTarget + "` with no sha refuses, so a recovery that omits it sends the operator into the guard rather than through it"},
	} {
		if !strings.Contains(driver, want.fragment) {
			t.Errorf("%s no longer names %q. %s.\n"+
				"That file is this record's only consumer: it is what makes writing the record a gate rather than a filing habit. If the driver has stopped reading the record, this stage can be run, skipped or never invoked and a release publishes either way",
				ciEvDriverFile, want.fragment, want.why)
		}
	}

	// (4) The WRITTEN procedure — and since the encoded one was retired, the ONLY
	// procedure, which is what CLAUDE.md always required.
	procedure := wiringReadFile(t, ciEvProcedureFile)
	item := ciEvProcedureItemRE.FindString(procedure)
	if item == "" {
		t.Fatalf("%s no longer carries an item titled **CI is green on `main`**. That item is where the written procedure sends a person to look at the RUN, and it is one side of the drift this test exists to catch — the other three sides can all agree while the document a maintainer actually reads says something else",
			ciEvProcedureFile)
	}
	for _, want := range []struct{ fragment, why string }{
		{"make " + ciEvMakeTarget, "the written procedure must name the command a maintainer actually runs. It is the only description of this gate a person reads, so a procedure that names something else is not a second opinion, it is the instruction"},
		{ciEvShaEnv, "it must tell a maintainer to key the run to the merge commit"},
		{ciEvOutEnv, "it must tell a maintainer where the record they are asked to read lands"},
	} {
		if !strings.Contains(item, want.fragment) {
			t.Errorf("%s's `CI is green on `main`` item no longer names %q. %s.\nThe item reads:\n%s", ciEvProcedureFile, want.fragment, want.why, item)
		}
	}
}

// TestTheProcedureNoLongerAsksForSomethingTheLogCannotShow closes the loop this
// lane opened in the written procedure.
//
// Before this lane the item asked a person to confirm the suite step "shows Go
// tests that actually ran — named tests and timings, not `[no tests to run]`
// beside every `ok`", and the entire output of that step on v0.5.0's run was one
// line: `ok  github.com/BarterX-Tech/dossierx/viewer-tests  89.327s`. No test
// names, no timings. The look-for could not be performed, so a conscientious
// maintainer either reported a pass they could not support or quietly stopped
// reading the item. An unsatisfiable instruction in a checklist is worse than a
// missing one, because it trains the reader to sign off on look-fors.
func TestTheProcedureNoLongerAsksForSomethingTheLogCannotShow(t *testing.T) {
	item := ciEvProcedureItemRE.FindString(wiringReadFile(t, ciEvProcedureFile))
	if item == "" {
		t.Fatalf("%s no longer carries the **CI is green on `main`** item at all", ciEvProcedureFile)
	}
	if strings.Contains(item, "named tests and timings") {
		t.Errorf("%s still asks a person to look for \"named tests and timings\" in the suite step's log.\n"+
			"That instruction was unsatisfiable while the step emitted one `ok <pkg> <time>` line, and it is not satisfiable now either: with `%s` the step emits a JSON event stream, which is a machine's account and not a thing a person skims. The item must say what the log ACTUALLY holds and which look-for now has a machine behind it.\nThe item reads:\n%s",
			ciEvProcedureFile, ciEvJSONFlag, item)
	}
	// SCOPED TO THE HALF THAT IS STILL A PERSON'S, and that scoping is the whole
	// assertion. `-json` appears in the machine half of this item too — the
	// paragraph describing what `make ci-evidence` reads — so requiring it
	// anywhere in the item proves only that the item mentions the flag, and the
	// sentence telling a maintainer what they will SEE when they open the step
	// could be rewritten or deleted with this test green. An earlier version of it
	// was. What has to be true is that the half sending a person to the run
	// describes the log that person will actually find.
	i := strings.Index(item, ciEvProcedureHumanHalf)
	if i < 0 {
		t.Fatalf("%s's CI item no longer carries a %q half.\n"+
			"C33 settles that the human look at the run SURVIVES this lane and is not made redundant by the machine check: the command accounts for the suites the workflow declares, and it accounts for nothing else — `hooks` runs a shell script that emits no countable account of anything, and a run that fired no CI at all is a different question again. Deleting the human half narrows the gate to what the machine can see, quietly",
			ciEvProcedureFile, ciEvProcedureHumanHalf)
	}
	if human := item[i:]; !strings.Contains(human, ciEvJSONFlag) {
		t.Errorf("%s's CI item sends a person to open the run without saying that the suite steps now emit a `%s` event stream.\n"+
			"Before this lane the step's whole output was one `ok <pkg> <time>` line; it is now several thousand lines of JSON. A maintainer who opens it expecting the old shape concludes something is broken, and one who expects to skim it for test names is being asked for the same unsatisfiable look-for in a new costume — which is what trains a reader to sign off on look-fors.\n"+
			"The half that is still a person's reads:\n%s",
			ciEvProcedureFile, ciEvJSONFlag, human)
	}
}

// ciEvHumanSectionRE isolates the block of docs/RELEASING.md holding the checks
// that were left to a person when the encoded checklist was retired, from its
// heading to the next section. Scoped rather than searched whole-file for the
// reason every item regex in this repository is: the document discusses the
// Release workflow and the site deploy in several places, and the question is
// what a maintainer is TOLD TO DO, not what the file mentions.
var ciEvHumanSectionRE = regexp.MustCompile(`(?s)### Three checks that stay a person's\n.*?\n## `)

// ciEvHumanItemRE counts the items inside it that are marked as a person's.
var ciEvHumanItemRE = regexp.MustCompile(`(?m)^- \[ \] \*\*\(human\) `)

// TestTheThreeChecksThatStayHumanSurvivedTheRetirement holds the part of the
// retired checklist that could not be inherited by anything.
//
// WHAT WAS RETIRED AND WHAT IT CARRIED. The encoded checklist's post-release
// phase asked an agent whether the Release workflow itself passed, whether
// deploy-site fired for this release, and whether the bundle the site is serving
// is the one that was built. Every other check it carried was either already
// machine-covered or became so; these three were not, and cannot be: each asks
// whether a system OUTSIDE this repository did what it was told, and a workflow
// that never fired, a deploy that kept serving yesterday's bundle and a run that
// concluded without producing an artifact all leave this tree byte-identical to
// the release that went right. There is nothing here for a check to read.
//
// WHY THAT NEEDS A TEST AT ALL, given that no test can make the checks. Because
// an item a machine cannot perform is the easiest thing in a long procedure to
// tidy away: it reads as a leftover from before the automation, and the reader
// deleting it is not doing anything obviously wrong. The landed precedent is
// cmd/dossierx/gate_receipt_test.go's ancestry pin, whose comment records a
// review deleting an entire procedure item with the package green. This asserts
// that the three survive, that each still says which system it is about, and
// that each is still MARKED as a person's — an item quietly restyled as though
// something checked it is the same deletion with the words left in.
func TestTheThreeChecksThatStayHumanSurvivedTheRetirement(t *testing.T) {
	procedure := wiringReadFile(t, ciEvProcedureFile)

	section := ciEvHumanSectionRE.FindString(procedure)
	if section == "" {
		t.Fatalf("%s no longer carries a `### Three checks that stay a person's` section.\n"+
			"Those three checks were the part of the retired release checklist that nothing inherited, because each one asks whether a system outside this repository obeyed and this tree is byte-identical either way. Deleting them does not remove a description of a check; it removes the only performance of it",
			ciEvProcedureFile)
	}

	if n := len(ciEvHumanItemRE.FindAllString(section, -1)); n != 3 {
		t.Errorf("%s's human section carries %d items marked `- [ ] **(human) `, not 3.\n"+
			"The marker is not decoration: it is what tells a maintainer that nothing behind this item ran, so the item is theirs to perform rather than theirs to confirm. An item restyled as an ordinary one reads as something already checked.\nThe section reads:\n%s",
			ciEvProcedureFile, n, section)
	}

	for _, want := range []struct{ fragment, why string }{
		{"The `Release` workflow itself passed",
			"a tag on the forge is not an artifact on the forge. A run that failed or stopped halfway leaves a published tag with nothing behind it, and the tag is what every consumer resolves"},
		{"`deploy-site` ran for this release",
			"deploy-site.yml triggers only on changes under `site/**`, so a release touching no site file publishes nothing, fails nowhere, and leaves the site serving the previous version. Nothing in this repository can tell that state from a successful deploy"},
		{"The deployed bundle is the one that was built",
			"a deploy that succeeded while a cache served an older build is a green workflow over a stale page. The asset hashes Vite content-hashes into the live index.html are the only thing that separates them, and they are read off the live site"},
	} {
		if !strings.Contains(section, want.fragment) {
			t.Errorf("%s's human section no longer names %q. %s.\nThe section reads:\n%s",
				ciEvProcedureFile, want.fragment, want.why, section)
		}
	}

	// And the REASON, because an item whose reason has been trimmed is an item
	// the next reader deletes. "We have not automated this yet" and "this cannot
	// be automated from a file" are read very differently by somebody tidying.
	if !strings.Contains(section, "no file in this tree can answer that") {
		t.Errorf("%s's human section no longer says WHY these three are a person's.\n"+
			"Without the reason they read as an automation backlog, and the honest response to a backlog item is to close it. The reason is that the answer is not in this repository at all",
			ciEvProcedureFile)
	}
}

// TestTheWorkflowFileNoLongerClaimsNothingReadsStepOutput corrects the three
// sentences this lane made false, in the only file where they can be corrected.
//
// tests/ci_workflow_test.go's header stated as PRESENT FACT that the automated
// reader "reads conclusions, never step output", that a suite step printing
// `[no tests to run]` "reaches it as a green check", and that "that residue is
// exactly what the human item's third look-for was added to catch". All three
// were true when written and none is true now, and a header that describes a gap
// which has since been closed sends the next maintainer looking for a check they
// already have.
func TestTheWorkflowFileNoLongerClaimsNothingReadsStepOutput(t *testing.T) {
	header := wiringReadFile(t, "tests/ci_workflow_test.go")
	if i := strings.Index(header, "package tests"); i > 0 {
		header = header[:i]
	}
	for _, stale := range []string{
		"It reads conclusions, never step output",
		"reaches it as a green check",
	} {
		if strings.Contains(header, stale) {
			t.Errorf("tests/ci_workflow_test.go's header still says %q.\n"+
				"%s is now fetched, parsed and adjudicated per instantiation by %s in %s, so that sentence describes a gap this repository no longer has. Correct the header; do not add an assertion to that file — the boundary it draws is not to be re-crossed",
				stale, ciEvStageFile, ciEvStageTestName, ciEvStageFile)
		}
	}
	if !strings.Contains(header, ciEvStageFile) {
		t.Errorf("tests/ci_workflow_test.go's header does not name %s. That file is where the question its header declares unanswerable from disk is now answered from the run, and a reader of the boundary needs to be sent there", ciEvStageFile)
	}
}

// ---------------------------------------------------------------------------
// The retirement itself, held.
//
// Everything above this point pins what the retired checklist LEFT BEHIND — the
// three human checks, the record's consumer, the identifiers both sides name.
// None of it pins the retirement. Restoring
// .claude/workflows/release-checklist.js verbatim from the commit that deleted
// it leaves every test in this repository green, because nothing here has ever
// read that directory: the deletion was a fact about one commit and not an
// invariant about the tree.
// ---------------------------------------------------------------------------

const (
	// ciEvWorkflowsDir is the directory the agent harness loads as INVOCABLE
	// workflows. That is why this directory and not some general search: a file
	// here is not inert text that happens to describe a release, it is a
	// procedure something can be asked to run, and while release-checklist.js sat
	// here it was offered to every agent in this repository as a live skill under
	// its own name.
	ciEvWorkflowsDir = ".claude/workflows"

	// ciEvRetiredChecklist is the encoded release procedure the gate pipeline
	// retired. It is named as an exact path because that is the only part of this
	// question that can be answered exactly.
	ciEvRetiredChecklist = ciEvWorkflowsDir + "/release-checklist.js"

	// ciEvRetiredChecklistName is the identity it declared to the harness in its
	// `meta` block — the name an agent invoked it by.
	ciEvRetiredChecklistName = "release-checklist"

	// ciEvMetaDecl opens the declaration the harness reads to learn what a
	// workflow file IS. It is a machine-read structure rather than prose, which
	// is what makes it something this file can parse and adjudicate instead of
	// pattern-matching over.
	ciEvMetaDecl = "export const meta = {"
)

// ciEvMetaStrings returns every string literal declared inside a workflow file's
// `export const meta = { … }` object, at any depth, with its escapes decoded.
//
// It is a scanner and not a search. It finds the declaration, walks the object
// from its opening brace to the matching close while tracking string literals
// and comments, and collects only what is inside a quoted literal — so a file's
// comments, its identifiers and its code are all outside what this reads. That
// boundary is the point: an assertion over a parsed declared value says
// something about what the file CLAIMS TO BE, where the same assertion over the
// whole file text would only say which words it contains.
//
// Anything it cannot walk is an error rather than an empty result. A file under
// this directory whose declaration cannot be read is a file this check cannot
// clear, and under CLAUDE.md's rule an examination that could not be made is a
// failure and never a pass.
func ciEvMetaStrings(src string) ([]string, error) {
	i := strings.Index(src, ciEvMetaDecl)
	if i < 0 {
		return nil, fmt.Errorf("no `%s` declaration", ciEvMetaDecl)
	}
	if strings.Contains(src[i+len(ciEvMetaDecl):], ciEvMetaDecl) {
		return nil, fmt.Errorf("more than one `%s` declaration, so which one the harness reads is not decidable here", ciEvMetaDecl)
	}

	var out []string
	depth := 0
	for p := i + len(ciEvMetaDecl) - 1; p < len(src); p++ {
		switch c := src[p]; {
		case c == '{' || c == '[':
			depth++
		case c == '}' || c == ']':
			depth--
			if depth == 0 {
				return out, nil
			}
		case c == '\'' || c == '"':
			lit, next, err := ciEvReadJSString(src, p)
			if err != nil {
				return nil, err
			}
			out, p = append(out, lit), next
		case c == '`':
			// Refused rather than skipped. A template literal can interpolate an
			// expression containing braces, so walking past one correctly means
			// implementing more of the grammar than this file should carry — and
			// guessing is how a scanner comes to report an empty declaration for a
			// file it never actually read.
			return nil, fmt.Errorf("a template literal at offset %d, which this scanner does not read", p)
		case c == '/' && p+1 < len(src) && src[p+1] == '/':
			if n := strings.IndexByte(src[p:], '\n'); n < 0 {
				p = len(src)
			} else {
				p += n
			}
		case c == '/' && p+1 < len(src) && src[p+1] == '*':
			n := strings.Index(src[p+2:], "*/")
			if n < 0 {
				return nil, fmt.Errorf("an unterminated block comment at offset %d", p)
			}
			p += 2 + n + 1
		}
	}
	return nil, fmt.Errorf("the `%s` object is never closed", ciEvMetaDecl)
}

// ciEvReadJSString reads the quoted literal beginning at src[start] and returns
// its decoded content and the index of its closing quote.
func ciEvReadJSString(src string, start int) (string, int, error) {
	quote := src[start]
	var b strings.Builder
	for p := start + 1; p < len(src); p++ {
		switch src[p] {
		case '\\':
			if p+1 >= len(src) {
				return "", 0, fmt.Errorf("a trailing backslash at offset %d", p)
			}
			p++
			switch src[p] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(src[p])
			}
		case quote:
			return b.String(), p, nil
		case '\n':
			return "", 0, fmt.Errorf("an unterminated string literal at offset %d", start)
		default:
			b.WriteByte(src[p])
		}
	}
	return "", 0, fmt.Errorf("an unterminated string literal at offset %d", start)
}

// TestTheWorkflowDeclarationScannerReadsWhatItClaimsTo holds the scanner to the
// two properties the test below rests on, because both are claims about what
// this file DOES NOT read and neither is visible from a passing repository.
//
// The first is the boundary: only string literals inside `meta` are collected,
// so a comment or a body naming the release procedure is deliberately outside.
// If that stopped being true the check would start refusing files on the words
// they contain, which is the pattern-matching it exists instead of.
//
// The second is fail-closed. Every shape the scanner cannot walk returns an
// error, and the caller turns an error into a failing test. A scanner that
// quietly returned no strings for a file it could not parse would report a
// clean directory for exactly the file worth looking at — the silent no-op this
// whole file exists to refuse, one level down.
func TestTheWorkflowDeclarationScannerReadsWhatItClaimsTo(t *testing.T) {
	const tick = "`"
	for _, tc := range []struct {
		name, src string
		want      []string
		wantErr   bool
	}{
		{
			name: "the retired checklist's own shape, nesting and all",
			src: "export const meta = {\n" +
				"  name: 'release-checklist',\n" +
				"  description: 'Runs docs/RELEASING.md as three verification gates.',\n" +
				"  phases: [\n" +
				"    { title: 'Pre-merge', detail: 'pin sweep' },\n" +
				"  ],\n}\nconst REPO = '/somewhere/else'\n",
			want: []string{"release-checklist", "Runs docs/RELEASING.md as three verification gates.", "Pre-merge", "pin sweep"},
		},
		{
			name: "a brace inside a string does not end the object early",
			src:  "export const meta = {\n  name: 'a}b',\n  after: 'docs/RELEASING.md',\n}\n",
			want: []string{"a}b", "docs/RELEASING.md"},
		},
		{
			// The boundary, stated as a test. release-checklist.js's header called
			// itself "docs/RELEASING.md, encoded" IN A COMMENT, and that sentence is
			// not what the check reads — it was refused on its declared description
			// and on its declared name instead.
			name: "comments inside the declaration are not read",
			src:  "export const meta = {\n  // docs/RELEASING.md, encoded\n  /* also docs/RELEASING.md */\n  name: 'chores',\n}\n",
			want: []string{"chores"},
		},
		{name: "no declaration at all", src: "const meta = {name: 'x'}\n", wantErr: true},
		{name: "two declarations", src: "export const meta = {a:'1'}\nexport const meta = {b:'2'}\n", wantErr: true},
		{name: "a template literal", src: "export const meta = {\n  name: " + tick + "release-checklist" + tick + ",\n}\n", wantErr: true},
		{name: "an object that never closes", src: "export const meta = {\n  name: 'x',\n", wantErr: true},
		{name: "a string that never closes", src: "export const meta = {\n  name: 'x,\n}\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ciEvMetaStrings(tc.src)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ciEvMetaStrings returned %q and no error.\nThis shape must be an ERROR, because the caller turns an error into a failing test and a nil-error empty result into a cleared file. A declaration the scanner cannot walk must not read as a declaration that said nothing", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ciEvMetaStrings: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ciEvMetaStrings = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTheRetiredReleaseChecklistStaysRetired refuses a second release procedure
// under .claude/workflows/.
//
// THE FAILURE IT EXISTS FOR. Somebody restores
// .claude/workflows/release-checklist.js — from the commit that deleted it, from
// a stale worktree, from a revert of the retirement commit — and this repository
// once again offers two release procedures. That is the state CLAUDE.md calls a
// defect in as many words: "there is exactly one of them: if you find a second
// release procedure anywhere in this repository, that is a defect to report, not
// a fallback to use". Until this test existed, the finding-of-it was left
// entirely to a person noticing, and the restored file is not a dormant
// document — it is a runnable skill the harness offers by name, it publishes
// `main` before the tag (the order the driver refuses to perform), it blocks on
// a content.ts commit field that no longer exists, and no gate in this pipeline
// can see it.
//
// WHAT IT PINS, IN TWO PARTS.
//
//   - THE EXACT PATH, which is exact and needs no judgement.
//   - THE DECLARED IDENTITY of every other file in that directory: its parsed
//     `meta` object may not name the written release procedure, and may not
//     re-claim the retired workflow's name. release-checklist.js announced
//     itself to the harness as running docs/RELEASING.md, so a successor doing
//     the same job under a different filename is caught by what it says it is
//     rather than by what it is called.
//
// WHAT IT DOES NOT PIN, stated plainly because a check whose limits are unwritten
// gets trusted past them:
//
//   - A file under this directory that encodes the release procedure WITHOUT
//     saying so in its `meta` — one whose description says "post-merge chores"
//     while its body walks the tag and the pushes — passes this. Deciding that
//     from the body means asking whether a paragraph of English is a release
//     procedure, which is not a question with a mechanical answer and is not
//     attempted here.
//   - Anywhere else in the tree. A second procedure dropped in docs/, scripts/
//     or skills/ is not this test's subject. .claude/workflows/ is pinned
//     because it is where the retired file lived and where a file is offered as
//     something to RUN; the general search is the unbounded one this test
//     declines to write.
//
// So this is a floor and not a ceiling, and CLAUDE.md's rule still needs a
// reader. What it guarantees is that the specific retirement this pipeline
// performed cannot be quietly undone.
func TestTheRetiredReleaseChecklistStaysRetired(t *testing.T) {
	root := repoRoot(t)

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ciEvRetiredChecklist))); err == nil {
		t.Errorf("%s exists again.\n"+
			"It is the release procedure this pipeline retired, and its own header called it %q — a second, runnable copy of the document CLAUDE.md requires there to be exactly one of. Restored, it publishes `main` before the tag, which is the ordering the release driver refuses to perform; it blocks on a commit field that no longer exists; and no gate in this pipeline reads it, so it can disagree with %s indefinitely without anything saying so.\n"+
			"If it came back by a revert or a merge, the deletion is what to keep. If it came back on purpose, that is a decision for a human to record, not for this file to accommodate",
			ciEvRetiredChecklist, "docs/RELEASING.md, encoded", ciEvProcedureFile)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cannot determine whether %s exists: %v.\nA check that could not look is reported as a failure and never as a pass", ciEvRetiredChecklist, err)
	}

	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(ciEvWorkflowsDir)))
	if errors.Is(err, os.ErrNotExist) {
		// The whole directory is gone, which is the intended end state — git does
		// not track empty directories and release-checklist.js was the only file
		// in it. Nothing to adjudicate.
		return
	}
	if err != nil {
		t.Fatalf("cannot read %s: %v.\nA check that could not look is reported as a failure and never as a pass", ciEvWorkflowsDir, err)
	}

	for _, entry := range entries {
		rel := ciEvWorkflowsDir + "/" + entry.Name()
		if entry.IsDir() {
			t.Errorf("%s is a directory, and this check reads files.\n"+
				"It is refused rather than descended into because a workflow this check has not read is one it has not cleared, and an unexamined corner of the directory the harness loads is where a second release procedure would sit unnoticed. Either flatten it or teach this test to walk it",
				rel)
			continue
		}

		declared, err := ciEvMetaStrings(wiringReadFile(t, rel))
		if err != nil {
			t.Errorf("%s: this check cannot read the declaration the harness reads — %v.\n"+
				"Every file here is offered to agents as something to run, and the `%s` block is what says what it is. A file whose declaration cannot be parsed is one this check cannot clear, so it is a failure rather than a pass: it is exactly where a restored release procedure would be invisible",
				rel, err, ciEvMetaDecl)
			continue
		}

		for _, s := range declared {
			if strings.Contains(s, ciEvProcedureFile) {
				t.Errorf("%s declares itself as running %s: %q.\n"+
					"That is a second release procedure, which CLAUDE.md calls a defect to report rather than a fallback to use — the retired checklist announced itself the same way. The one procedure is %s and it is read by a person and by the driver; a runnable encoding of it drifts from it silently, because nothing compares the two",
					rel, ciEvProcedureFile, s, ciEvProcedureFile)
			}
			if s == ciEvRetiredChecklistName {
				t.Errorf("%s declares the name %q, which is the identity the retired release checklist was invoked by.\n"+
					"A successor under a new filename is the retirement undone with the path changed. If this workflow does something else entirely, give it a name that is not the retired one",
					rel, ciEvRetiredChecklistName)
			}
		}
	}
}
