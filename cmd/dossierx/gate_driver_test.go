// gate_driver_test.go is THE DRIVER: the ordered, deterministic sequence that
// performs the one half of a release that cannot be taken back — a tag on a
// public forge, six archives somebody will `go install`, and a site page that
// tells the world a release exists.
//
// THE INVARIANT IT EXISTS FOR. No irreversible act happens except as the tail of
// a process that, in that same process, read the tree being published and found
// it green — and no such process starts except when a human asked for that
// specific release. Everything below is one of the four clauses that make up
// that sentence, and none of them is a consequence of the others.
//
//  1. THE RECEIPT IS MEASURED, NOT ACCEPTED. The value the driver publishes
//     against is produced by calling gateRecordReceipt in THIS process, which
//     resolves the branch head and `<branch>^{tree}` from git itself and
//     re-asserts the ancestry precondition before it will return anything. There
//     is no parameter, flag, path or environment variable through which a
//     gateReceipt assembled anywhere else can enter the publish path, because no
//     function in this file takes one — TestTheDriverCanBeHandedNoReceipt is
//     what holds that, structurally, rather than by anyone remembering. The
//     failure it forecloses is not hypothetical: a complete, committed,
//     hand-editable receipt already exists on disk in this repository, nested
//     inside gate-cost-ledger.json's newest run, and the most natural driver
//     anybody would write reads it from there and checks it. Every check then
//     passes, because a receipt is fields and fields are what an editor writes.
//     Paper proves what the paper says, not that anyone looked.
//
//  2. GREEN IS RECOMPUTED, NEVER READ. The verdict is receipt.evaluate(declared,
//     current) with `declared` from the manifest and `current` fingerprinted
//     against the tree about to be released, computed here. No PASS is carried in
//     on paper — the receipt deliberately carries no verdict field — and no
//     shortcut ("the findings array is empty") stands in for the six refusals
//     gateIsGreen makes.
//
//  3. THE OBJECT PUBLISHED IS THE OBJECT HANDSHAKED. The merge commit is named
//     explicitly and carried BY VALUE from D2 to D8; the tag is created on that
//     named object; `<tag>^{tree}` is compared against the receipt again
//     immediately before the tag is pushed; and both pushes name an object sha
//     rather than a branch, so nothing that lands between two steps can be
//     published by either of them. The token "HEAD" appears nowhere in this file
//     as the thing being tagged. The window between merge and tag is exactly
//     where main advances, and an unattended driver is more exposed to it than a
//     human is, not less.
//
//  4. EVERY STEP THAT CANNOT RUN IS A FAILURE THAT NAMES WHAT IS ALREADY
//     PUBLISHED. Nothing downstream of a failed step executes, and a step whose
//     machinery does not exist yet reports errGateUncheckable rather than
//     falling through. Because the irreversible half is ORDERED — the tag is
//     pushed before main is — a failure after the first irreversible act must
//     state which acts already completed. A bare non-zero exit after a partial
//     release sends the operator in blind.
//
// AND ONE THING THE INVARIANT DOES NOT CLAIM, stated here rather than discovered
// later: it says nothing about whether the thirteen agents' verdicts are honest.
// Running in one process removes the forgeable RECEIPT; it does not remove the
// forgeable EVIDENCE. What stands behind the evidence is the freshness machinery
// in gate_stage2_test.go — the run manifest that records this tree, this
// resolved baseline and the digest of every artifact the keys cover — and that
// boundary is here, at gateDriverEvidence, not implied away.
//
// WHAT THIS DRIVER CANNOT DO TODAY, and it is deliberate rather than unfinished.
// gateDriverUnwired is the evidence source the real repository gets, and every
// one of its four answers is errGateUncheckable naming the lane that owes it:
// nothing in this tree turns thirteen agent answers into []gateSurfaceVerdict,
// nothing downloads and verifies a published archive, and nothing reads the
// rendered site. So an authorized run against this repository refuses at D1 and
// publishes nothing at all, and it will keep refusing one step later at a time
// as those lanes land. That is CLAUDE.md's rule applied to the driver's own
// incompleteness — a check that cannot run is a failure, not a pass — and it is
// what makes this file safe to land before the pipeline exists: it can refuse,
// and it cannot complete a release.
//
// WHY IT IS TEST CODE, which is a resolution rather than a preference, recorded
// here so the next person does not re-derive it. Every gate symbol this driver
// calls — gateRecordReceipt, gatePreMergePrecondition,
// gateAssertMergeMatchesReceipt, evaluate, gateIsGreen, gateDeclaredSurfaces,
// gateStage2Plan — is declared in a _test.go file in package main and is
// unexported, so the ONLY compile unit that can reach the gate is the
// cmd/dossierx test binary. The two alternatives each break something outside
// this file: a Go package under scripts/ reds
// TestSurfaceBehaviourFingerprintCoversEveryPackage, which requires every Go
// package git carries to be fingerprinted or excluded (scripts/gate-stage2/run.sh
// hit this exact wall and recorded it in its own header); and promoting the gate
// into internal/ or a second cmd/ main puts it inside behaviourRoots, so
// surface.json would claim the gate's own machinery as shipped behaviour.
//
// THE COST OF BEING TEST CODE, and the reason the two guards below exist.
// `go test ./...` runs this file on a maintainer's machine, with the maintainer's
// push credentials, at every lane landing. So the driver acts only when it is
// AUTHORIZED, never when its preconditions merely happen to be satisfiable — see
// gateDriverMode — and `go test`'s result cache is refused by the invocation
// itself, because a cached success replays `ok` in under a second for a process
// that merged nothing, tagged nothing and pushed nothing.
//
// WHAT THIS FILE DELIBERATELY DOES NOT BUILD: a check that the caller is a human.
// Nothing inside this repository can tell a human's `make` invocation from an
// agent's, and the guard everyone reaches for first — os.Stdin.Stat() and
// Mode()&os.ModeCharDevice — is SATISFIED by the exact invocation shape it exists
// to refuse: under `go test` stdin is /dev/null, and /dev/null is a character
// device (measured: MODE=Dcrw-rw-rw- CHARDEV=true). A guard that reports "a human
// is present" for every automated caller is worse than no guard, because it is
// read as one. The authorization here is therefore what a human typed — the
// version, twice, on a named target — and the residual is stated plainly in the
// refusal text rather than papered over: the barrier between an agent and this
// target belongs to the harness's permission system, which lives outside this
// repository and outside every test in it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------
// the identifiers every side of the release-time invocation must name
//
// They are constants because the Makefile recipe, this file's stage and
// docs/RELEASING.md have to spell each of them, and
// TestTheReleaseInvocationCannotSucceedHavingDoneNothing is what requires it.
// That is the whole defence against the invoker and the invoked drifting apart:
// change one of these and the ordinary suite fails until every side moves with
// it.
// ---------------------------------------------------------------------

const (
	// gateDriverVersionEnv names the release being published.
	gateDriverVersionEnv = "DOSSIERX_RELEASE_VERSION"

	// gateDriverAuthorizeEnv is the human's authorization, and it is a SECOND
	// spelling of the version rather than a boolean on purpose. Decision 3 is
	// that a human authorizes THAT SPECIFIC RELEASE; a `=1` or `=yes` left in a
	// shell's history, an exported profile variable or a CI secret authorizes
	// every release forever, including the next one somebody triggers by
	// accident.
	gateDriverAuthorizeEnv = "DOSSIERX_RELEASE_AUTHORIZE"

	// gateDriverRecordEnv names where the run record is written AFTER the fact.
	// Nothing ever trusts that file — see the package comment's clause 1 — but a
	// release whose irreversible steps left no readable account is a release
	// nobody can audit, and the invocation refuses to exit 0 without one.
	gateDriverRecordEnv = "DOSSIERX_RELEASE_RECORD_OUT"

	// gateDriverMakeTarget is the one named command a human types. It is a make
	// target rather than a `go test -run` line typed from memory for the reason
	// the ci-evidence recipe already gives: `go test` exits 0 for a selector that
	// matches nothing, so an invocation whose selector drifted by one character
	// would report a clean release forever.
	gateDriverMakeTarget = "release-publish"

	// gateDriverTestName is the stage below that runs the sequence. The Makefile
	// selects it by this exact name.
	gateDriverTestName = "TestReleaseDriverPublishes"

	// gateDriverFile is this file, addressed from the repository root.
	gateDriverFile = "cmd/dossierx/gate_driver_test.go"

	// gateDriverPackage is the package the invocation must test. A selector that
	// matched this name in another package would run nothing.
	gateDriverPackage = "./cmd/dossierx/"

	// gateDriverProcedureFile is the WRITTEN release procedure. CLAUDE.md: there
	// is exactly one of these, so the order the driver performs and the order the
	// document describes cannot be allowed to disagree.
	gateDriverProcedureFile = "docs/RELEASING.md"
)

// gateDriverTimeoutFloor is the smallest `-timeout` the release-time invocation
// may carry.
//
// `go test`'s default is ten minutes and the answer to exceeding it is a PANIC —
// "If a test binary runs longer than duration d, panic". D7 waits for the Release
// workflow to build six GOOS/GOARCH archives and then verifies them, which
// routinely exceeds ten minutes. On the default, the test binary panics between
// the tag push and the main push: the operator receives a goroutine dump, the tag
// is on the forge, main is not, and the one artifact this file owes a human — a
// per-step statement of what is already published — is never printed.
const gateDriverTimeoutFloor = 30 * time.Minute

// ---------------------------------------------------------------------
// D0: the authorization decision
// ---------------------------------------------------------------------

// gateDriverPlan is what the environment asked for. It is a value, and the
// decision below is a pure function of it, so the decision can be exercised
// without an environment — a test that must set process-wide variables to reach
// one branch is a test that cannot reach the others in the same run.
type gateDriverPlan struct {
	Version   string
	Authorize string
	Record    string
}

func gateDriverPlanFromEnv() gateDriverPlan {
	return gateDriverPlan{
		Version:   strings.TrimSpace(os.Getenv(gateDriverVersionEnv)),
		Authorize: strings.TrimSpace(os.Getenv(gateDriverAuthorizeEnv)),
		Record:    strings.TrimSpace(os.Getenv(gateDriverRecordEnv)),
	}
}

// gateDriverDecision is what a given environment MEANS.
//
// THE DIRECTION IS INVERTED FROM ci_run_evidence_test.go's ciEvGateMode, and the
// inversion is the point. That stage is read-only, so "nobody named a commit"
// means skip. This one ACTS, so "nobody named a release" must mean do nothing AND
// BE SEEN TO DO NOTHING: gateDriverInert is asserted positively — the sequence is
// entered and completes zero steps — rather than reported as a t.Skip that prints
// the same `ok` a real run prints.
type gateDriverDecision int

const (
	gateDriverGo gateDriverDecision = iota
	gateDriverInert
	gateDriverRefuse
)

// gateDriverMode decides whether this process may publish.
//
// The five answers, and why none of them collapses into another:
//
//   - NOTHING SET: inert. This is `go test ./...` — `make test`, a lane landing,
//     a maintainer's laptop. Nothing about the tree is wrong at that moment; the
//     gate may well be green, the ancestry may well hold, the trees may well
//     match. That is precisely why every guard INSIDE the driver would pass, and
//     why the guard has to be about the CALLER's intent rather than about the
//     tree's state.
//   - THE VERSION WITHOUT THE AUTHORIZATION: refuse. Something is invoking the
//     driver as a publisher and no human has named the release. It is the shape
//     of a script that learned half the recipe.
//   - THE AUTHORIZATION WITHOUT THE VERSION: refuse, and this is the one worth
//     spelling out. A standing authorization with nothing to apply it to is an
//     environment where the NEXT invocation to supply a version publishes,
//     whatever it is.
//   - THE TWO DISAGREEING: refuse. The authorization is for one release. An
//     authorization for v0.6.0 does not clear v0.7.0, and the case this catches
//     is the ordinary one — a variable left set from the last release.
//   - NO RECORD PATH: refuse. The irreversible steps would happen and leave no
//     readable account of which of them completed, which is the single thing an
//     operator needs when a release stops halfway.
func gateDriverMode(p gateDriverPlan) (gateDriverDecision, string) {
	switch {
	case p.Version == "" && p.Authorize == "":
		return gateDriverInert, fmt.Sprintf(
			"%s is unset, so no human has asked for a release and this driver does nothing. "+
				"This is what `go test ./...` looks like from inside the driver, and it is the ONLY thing that keeps a lane landing from publishing: "+
				"at that moment the gate may be green, the ancestry may hold and the trees may match, so every precondition below would pass. "+
				"Publishing is authorized by a human running `make %s`, never by the preconditions being satisfiable.\n"+
				"WHAT THIS DOES NOT ESTABLISH: that the caller is a human. Nothing in this repository can tell a person's `make` from an agent's, "+
				"and the mode-bit check people reach for (os.ModeCharDevice on stdin) is satisfied by `go test` itself, where stdin is /dev/null — a character device. "+
				"The barrier between an agent and this target is the harness's permission system, which lives outside this repository and outside every test in it",
			gateDriverVersionEnv, gateDriverMakeTarget)

	case p.Authorize == "":
		return gateDriverRefuse, fmt.Sprintf(
			"%s names %q but %s is unset. Something is invoking this driver as a publisher and no human has authorized that release. "+
				"The authorization is the version typed a second time, deliberately: a boolean would authorize every release forever, including the next one somebody triggers by accident. Run `make %s %s=%s %s=%s`",
			gateDriverVersionEnv, p.Version, gateDriverAuthorizeEnv, gateDriverMakeTarget,
			gateDriverVersionEnv, p.Version, gateDriverAuthorizeEnv, p.Version)

	case p.Version == "":
		return gateDriverRefuse, fmt.Sprintf(
			"%s is set (%q) but %s is not, so an authorization is standing in this environment with no release attached to it. "+
				"That is worse than an unauthorized invocation: the next process to supply a version publishes, whatever it is",
			gateDriverAuthorizeEnv, p.Authorize, gateDriverVersionEnv)

	case p.Version != p.Authorize:
		return gateDriverRefuse, fmt.Sprintf(
			"%s names %q and %s authorizes %q. A human authorizes one release, not releases in general — the ordinary way to reach this is an authorization left set from the last one — so the two must be the same string and this driver publishes neither",
			gateDriverVersionEnv, p.Version, gateDriverAuthorizeEnv, p.Authorize)

	case p.Record == "":
		return gateDriverRefuse, fmt.Sprintf(
			"%s authorizes %s but %s is unset, so the irreversible steps would run and leave no account of which of them completed. "+
				"When a release stops between the tag push and the main push, that account is the only thing that tells an operator what is already on the forge. Run it as `make %s`",
			gateDriverAuthorizeEnv, p.Version, gateDriverRecordEnv, gateDriverMakeTarget)
	}
	return gateDriverGo, ""
}

// ---------------------------------------------------------------------
// D1-D9: the sequence
// ---------------------------------------------------------------------

// gateDriverStep is one step of the sequence and, for the two that publish, the
// sentence a human reads when the run stops after it.
//
// Published is a specification and not commentary: the whole reason the
// irreversible half is ordered is that a failure between the two pushes leaves a
// state somebody has to reason about, and a bare non-zero exit there sends them
// in blind.
type gateDriverStep struct {
	ID           string
	What         string
	Irreversible bool
	Published    string // %s is the version; empty when nothing leaves this machine
}

// gateDriverSequence is the order, and the order is the audit's finding rather
// than a preference: the release branch edits site/src/content.ts, so pushing
// main fires deploy-site.yml (on: push, branches [main], paths ['site/**']) and
// publishes a page reading "the current release is vX.Y.Z" while no tag and no
// archive exists, because release.yml fires only on push: tags ['v*']. The tag
// goes first, the archives are verified, and main goes last.
var gateDriverSequence = []gateDriverStep{
	{ID: "D0", What: "authorize — a human named this release, twice, on the named target"},
	{ID: "D1", What: "precondition — refuse a partly-published release, record the receipt IN THIS PROCESS, recompute the verdict against the tree about to be released"},
	{ID: "D2", What: "merge the release branch into main with --no-ff, capturing the merge commit by value"},
	{ID: "D3", What: "handshake — the named merge commit's tree is the tree the receipt records"},
	{ID: "D4", What: "tag the named merge commit, never HEAD"},
	{ID: "D5", What: "re-read <tag>^{tree} and hand it to the handshake again, immediately before the push"},
	{ID: "D6", What: "push the tag", Irreversible: true,
		Published: "the tag %s is on the forge, and the Release workflow is building six archives from it. Deleting a published tag is not an undo: anything that fetched it has it"},
	{ID: "D7", What: "verify the published archives"},
	{ID: "D8", What: "push main", Irreversible: true,
		Published: "main carries the release commit, and deploy-site has published a site announcing %s. There is no automatic rewrite of a published page; the gate surfaces, it does not fix"},
	{ID: "D9", What: "G3 — the rendered site describes this release"},
}

// gateDriverRepo is the repository the sequence acts on.
//
// Env is extra environment for the WRITING git commands only. Production passes
// none — a release tool that overrode a maintainer's git configuration would be
// changing the environment it was asked to act in — and the fixtures pass the
// contributor-config neutralization gateTestGit applies, so a developer's
// commit.gpgsign cannot decide whether these tests pass.
type gateDriverRepo struct {
	Dir    string
	Branch string // the release branch
	Base   string // the local branch the release lands on
	Remote string
	Env    []string
}

// gateDriverEvidence is everything the driver must be TOLD rather than compute,
// and it is one interface so that the boundary is one place a reader can look at.
//
// It carries no gateReceipt and cannot: the receipt is measured here (clause 1),
// and what crosses this boundary is the material a receipt is measured FROM. The
// production implementation is gateDriverUnwired, which answers every one of
// these with errGateUncheckable naming the lane that owes it.
type gateDriverEvidence interface {
	// Verdicts are the per-surface answers and findings the gate run produced
	// for tree. They are the receipt's contents, never the receipt.
	Verdicts(tree string) ([]gateSurfaceVerdict, []gateFinding, error)
	// Current is the manifest's declared surfaces and the fingerprint each one
	// has in tree — the pair receipt.evaluate needs to recompute green.
	Current(tree string) (declared []string, current map[string]string, err error)
	// Archives is D7: the published artifacts for version, verified.
	Archives(version, commit string) error
	// Site is D9: the rendered site, read as a browser sees it.
	Site(version string) error
}

// gateDriverUnwired is the evidence source the real repository gets today.
//
// Every answer is errGateUncheckable, which is not a stub standing in for a
// missing implementation — it IS the implementation, and it is the correct one
// under CLAUDE.md until the lanes below land. A driver that assumed any of these
// would be narrowing coverage silently at the only moment it matters.
type gateDriverUnwired struct{}

func (gateDriverUnwired) Verdicts(string) ([]gateSurfaceVerdict, []gateFinding, error) {
	return nil, nil, fmt.Errorf("%w: nothing in this tree transports a gate run's per-surface verdicts into this process. "+
		"Every gateSurfaceVerdict in the repository is built inside a test fixture, and the stage-2 harness writes a run manifest and per-surface artifacts that no reader turns into verdicts. "+
		"The driver will not invent them: a receipt measured over verdicts nobody produced is a receipt about nothing, and it would evaluate PASS", errGateUncheckable)
}

func (gateDriverUnwired) Current(string) ([]string, map[string]string, error) {
	return nil, nil, fmt.Errorf("%w: the driver has no wired path to this tree's per-surface fingerprints. "+
		"gateDeclaredSurfaces and gateStage2Plan compute them, and until the harness invocation that feeds them is wired to this driver, recomputing green here would be reading a map nobody filled", errGateUncheckable)
}

func (gateDriverUnwired) Archives(version, commit string) error {
	return fmt.Errorf("%w: verifying the published archives for %s at %s is not built in this tree. "+
		"Nothing here downloads a release artifact, checks a sha256 against checksums.txt, or reads the stamped version, commit and date out of an extracted binary. "+
		"So the driver stops HERE, after the tag is published and before main is, and says so — which is the failure-not-skip rule applied to the driver's own incompleteness. "+
		"A driver that pushed main without verifying the archives would announce a release whose artifacts nobody checked", errGateUncheckable, version, commit)
}

func (gateDriverUnwired) Site(version string) error {
	return fmt.Errorf("%w: reading the rendered site to confirm it describes %s is not built in this tree. "+
		"The site is checked as rendered DOM from a real build, and that extractor is not wired to this driver", errGateUncheckable, version)
}

// gateDriverRun is one execution: what it completed, what it failed at, and the
// values it carried between steps.
//
// Receipt is a FIELD and never a parameter. That distinction is the whole of
// clause 1 — a field is written by exactly one statement in this file, the one
// that calls gateRecordReceipt, while a parameter is a doorway anything holding a
// decoded JSON object can walk through.
type gateDriverRun struct {
	Plan    gateDriverPlan
	Repo    gateDriverRepo
	Done    []string
	Failed  string
	Verdict string
	Merge   string // the merge commit, captured once at D2 and used by value after
	TagObj  string // the tag object read back at D5 and pushed by value at D6
	Receipt gateReceipt
	Err     error

	// after is fault injection for the tests below, and nil in production. It
	// lets a test land a commit, or move a tag, in the window between two steps —
	// which is the only way to prove that a re-assertion made after that window
	// is load-bearing rather than arithmetic that always agrees with itself.
	after map[string]func()
}

func (r *gateDriverRun) stop(id string, err error) *gateDriverRun {
	r.Failed, r.Err = id, err
	return r
}

func (r *gateDriverRun) step(id string) {
	r.Done = append(r.Done, id)
	if hook := r.after[id]; hook != nil {
		hook()
	}
}

func (r *gateDriverRun) completed(id string) bool {
	for _, done := range r.Done {
		if done == id {
			return true
		}
	}
	return false
}

// git runs one WRITING git command in the repository under release.
func (r *gateDriverRun) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Repo.Dir
	if len(r.Repo.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Repo.Env...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// report is what a human reads when a run stops. It names the failed step, then
// what is already public, then what is not, and then the one thing this driver
// will not do about it.
func (r *gateDriverRun) report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "the release of %s stopped at %s (%s):\n%v\n\n", r.Plan.Version, r.Failed, gateDriverWhat(r.Failed), r.Err)

	var published, remaining []string
	for _, step := range gateDriverSequence {
		if !step.Irreversible {
			continue
		}
		line := fmt.Sprintf("  %s %s — %s", step.ID, step.What, fmt.Sprintf(step.Published, r.Plan.Version))
		if r.completed(step.ID) {
			published = append(published, line)
		} else {
			remaining = append(remaining, line)
		}
	}

	if len(published) == 0 {
		b.WriteString("ALREADY PUBLISHED: nothing. No irreversible step ran, so this release has left no trace outside this machine.\n")
	} else {
		b.WriteString("ALREADY PUBLISHED — these have happened and cannot be taken back:\n")
		b.WriteString(strings.Join(published, "\n") + "\n")
	}
	if len(remaining) > 0 {
		b.WriteString("NOT PUBLISHED — these did not run:\n")
		b.WriteString(strings.Join(remaining, "\n") + "\n")
	}
	if len(published) > 0 {
		fmt.Fprintf(&b, "\nRE-ENTRY: this driver will not resume past this point and will not undo anything. "+
			"Invoked again for %s it refuses, naming these same two lists. A human completes the release by hand or unwinds it by hand; "+
			"the driver surfaces, it does not fix.\n", r.Plan.Version)
	}
	return b.String()
}

func gateDriverWhat(id string) string {
	for _, step := range gateDriverSequence {
		if step.ID == id {
			return step.What
		}
	}
	return "an unknown step"
}

// gateDriverPublish is the sequence.
//
// It is fail-closed in both directions: nothing downstream of a failed step
// executes, and no step reports success it did not establish. Read it as the four
// clauses of the package comment in order.
func gateDriverPublish(plan gateDriverPlan, repo gateDriverRepo, ev gateDriverEvidence) *gateDriverRun {
	return gateDriverExecute(&gateDriverRun{Plan: plan, Repo: repo}, ev)
}

// gateDriverExecute is the sequence itself, over a run that already exists. The
// split is what lets the tests below hand it a run carrying fault injection; in
// production nothing but gateDriverPublish calls it, and r.after is nil.
func gateDriverExecute(r *gateDriverRun, ev gateDriverEvidence) *gateDriverRun {
	plan, repo := r.Plan, r.Repo

	// D0. Before anything at all — before a single git command runs, before the
	// repository is even looked at.
	if mode, why := gateDriverMode(plan); mode != gateDriverGo {
		return r.stop("D0", errors.New(why))
	}
	r.step("D0")

	// D1.
	if err := r.precondition(ev); err != nil {
		return r.stop("D1", err)
	}
	r.step("D1")

	// D2. The merge commit is read once, by name, and every later step uses that
	// value. Reading it again later would re-introduce exactly the window this
	// records its way out of.
	if err := r.merge(); err != nil {
		return r.stop("D2", err)
	}
	r.step("D2")

	// D3. The handshake, against the named object.
	mergeTree, err := gateTreeSHA(repo.Dir, r.Merge)
	if err != nil {
		return r.stop("D3", fmt.Errorf("%w: the merge commit %s has no readable tree: %w", errGateUncheckable, r.Merge, err))
	}
	if err := gateAssertMergeMatchesReceipt(r.Receipt, mergeTree); err != nil {
		return r.stop("D3", err)
	}
	r.step("D3")

	// D4. The object is NAMED. `git tag -a v -m …` with no ref tags HEAD, which
	// is right only when nothing has landed since the merge — and between D2 and
	// here, another operator, a bot, or this driver's own steps can land anything.
	if _, err := r.git("tag", "-a", plan.Version, "-m", plan.Version, r.Merge); err != nil {
		return r.stop("D4", fmt.Errorf("could not create the tag on the merge commit %s: %w", r.Merge, err))
	}
	r.step("D4")

	// D5. Read the tag BACK, by ref, and hand its tree to the handshake again.
	// This is not the same question D3 asked: D3 asked what the merge holds, this
	// asks what the ref that is about to be published points at. A tag that moved
	// between D4 and here — a force-move by another process, a stale local tag
	// resurrected — passes every check made before it.
	if err := r.recheckTag(); err != nil {
		return r.stop("D5", err)
	}
	r.step("D5")

	// D6. THE FIRST IRREVERSIBLE ACT. The object is pushed by VALUE: the tag
	// object D5 read, not the name it read it through.
	if _, err := r.git("push", repo.Remote, r.TagObj+":refs/tags/"+plan.Version); err != nil {
		return r.stop("D6", err)
	}
	r.step("D6")

	// D7.
	if err := ev.Archives(plan.Version, r.Merge); err != nil {
		return r.stop("D7", err)
	}
	r.step("D7")

	// D8. THE LAST IRREVERSIBLE ACT, and by value again: the merge commit
	// captured at D2, never whatever the local branch points at now.
	if _, err := r.git("push", repo.Remote, r.Merge+":refs/heads/"+repo.Base); err != nil {
		return r.stop("D8", err)
	}
	r.step("D8")

	// D9.
	if err := ev.Site(plan.Version); err != nil {
		return r.stop("D9", err)
	}
	r.step("D9")
	return r
}

// precondition is D1, and it is four questions rather than one.
func (r *gateDriverRun) precondition(ev gateDriverEvidence) error {
	// The base ref this driver re-asserts against is gateBaseRef, which is
	// origin/main and deliberately not "main". A repository whose remote and base
	// branch spell something else would have the ancestry question asked about a
	// ref nobody named, so it is refused rather than answered.
	if base := r.Repo.Remote + "/" + r.Repo.Base; base != gateBaseRef {
		return fmt.Errorf("%w: this driver re-asserts the ancestry precondition against %s, and it was pointed at %s. "+
			"Answering from a differently-named base would make every check below about a ref the receipt does not record", errGateUncheckable, gateBaseRef, base)
	}

	// (a) Re-entry. A release that already published something is not resumed and
	// not undone; it is refused, with both lists printed.
	if err := r.refuseIfAlreadyPublished(); err != nil {
		return err
	}

	// (b) The tree the evidence is about, read before the evidence is asked for.
	tree, err := gateTreeSHA(r.Repo.Dir, r.Repo.Branch)
	if err != nil {
		return fmt.Errorf("%w: %s has no readable tree, so there is nothing to gather evidence about: %w", errGateUncheckable, r.Repo.Branch, err)
	}
	verdicts, findings, err := ev.Verdicts(tree)
	if err != nil {
		return err
	}
	declared, current, err := ev.Current(tree)
	if err != nil {
		return err
	}

	// (c) The receipt, MEASURED here. gateRecordReceipt re-asserts the ancestry
	// precondition itself, resolves the head and the tree from git, and refuses a
	// surface reported twice.
	receipt, err := gateRecordReceipt(r.Repo.Dir, r.Plan.Version, r.Repo.Branch, verdicts, findings)
	if err != nil {
		return err
	}
	if receipt.Tree != tree {
		return fmt.Errorf("%w: the evidence was gathered about tree %s and the receipt records %s, so the branch moved while the run was being read. "+
			"The receipt would then carry verdicts about content it does not name — which is the forgeable-evidence boundary this driver cannot close, so it refuses instead", errGateUncheckable, tree, receipt.Tree)
	}
	r.Receipt = receipt

	// The last moment anything can refuse before the merge. It re-asks BOTH
	// questions rather than trusting the receipt that was written seconds ago,
	// because it is the only assertion made after a human's authorization: more
	// time passes there than passes inside a gate run.
	if err := gatePreMergePrecondition(r.Repo.Dir, r.Receipt); err != nil {
		return err
	}

	// (d) Green, RECOMPUTED. Not read, not inferred from an empty findings list —
	// evaluate makes six separate refusals about coverage that "no findings"
	// cannot stand in for.
	verdict, err := r.Receipt.evaluate(declared, current)
	r.Verdict = verdict
	if err != nil {
		return fmt.Errorf("the gate is %s for %s and nothing is published on a %s gate:\n%w", verdict, r.Plan.Version, verdict, err)
	}
	if verdict != gateVerdictPass {
		return fmt.Errorf("%w: the recomputed verdict is %q, which is not %s", errGateUncheckable, verdict, gateVerdictPass)
	}
	return nil
}

// refuseIfAlreadyPublished is the answer to "the tag is pushed and D7 failed;
// what does the next invocation do?".
//
// It refuses. It does not resume past the refusal and it does not undo anything:
// resuming would mean trusting that the steps before the tag were performed by
// some earlier process this one cannot see, and undoing would mean the gate
// fixing something, which it never does. What it owes the human is the two lists,
// and it computes the second one from the forge rather than assuming it.
func (r *gateDriverRun) refuseIfAlreadyPublished() error {
	remoteTag, err := gateGit(r.Repo.Dir, "ls-remote", "--tags", r.Repo.Remote, "refs/tags/"+r.Plan.Version)
	if err != nil {
		return fmt.Errorf("%w: could not ask %s whether %s is already published. "+
			"Publishing without that answer risks re-entering a release that is already half done, so it is a failed check rather than an absent one: %w",
			errGateUncheckable, r.Repo.Remote, r.Plan.Version, err)
	}
	if strings.TrimSpace(remoteTag) == "" {
		return nil
	}

	object := strings.Fields(remoteTag)[0]
	mainState := "could not be determined from this clone"
	if err := gateRefreshBase(r.Repo.Dir, gateBaseRef); err == nil {
		if commit, resolveErr := gateResolve(r.Repo.Dir, object+"^{commit}"); resolveErr == nil {
			cmd := exec.Command("git", "merge-base", "--is-ancestor", commit, gateBaseRef)
			cmd.Dir = r.Repo.Dir
			switch runErr := cmd.Run(); {
			case runErr == nil:
				mainState = "YES — " + gateBaseRef + " already contains the tagged commit, so the release is complete as far as the pushes go"
			default:
				var exit *exec.ExitError
				if errors.As(runErr, &exit) && exit.ExitCode() == 1 {
					mainState = "NO — " + gateBaseRef + " does not contain the tagged commit, so the tag is published and main is not"
				}
			}
		}
	}

	return fmt.Errorf("%w: %s is ALREADY PUBLISHED on %s (tag object %s), so this is a re-entry into a release that has already performed an irreversible act.\n"+
		"  D6 push the tag — DONE, and it cannot be taken back\n"+
		"  D8 push main    — %s\n"+
		"This driver refuses rather than resuming: resuming would mean trusting steps performed by a process this one cannot see, and it will not delete the tag either, because the gate surfaces and never fixes. "+
		"A human finishes this release by hand or unwinds it by hand",
		errGateUncheckable, r.Plan.Version, r.Repo.Remote, object, mainState)
}

// merge is D2. The merge commit is captured from the BRANCH it landed on, by
// value, and that value is what every later step names.
func (r *gateDriverRun) merge() error {
	if _, err := r.git("checkout", "-q", r.Repo.Base); err != nil {
		return err
	}
	if _, err := r.git("merge", "--no-ff", "--no-edit", "-m",
		fmt.Sprintf("Merge %s for %s", r.Repo.Branch, r.Plan.Version), r.Repo.Branch); err != nil {
		return err
	}
	merge, err := gateResolve(r.Repo.Dir, r.Repo.Base)
	if err != nil {
		return fmt.Errorf("%w: the merge completed and its commit could not be read, so nothing downstream could name the object it produced: %w", errGateUncheckable, err)
	}
	r.Merge = merge
	return nil
}

// recheckTag is D5: the tag is read back through the ref and handed to the SAME
// handshake D3 used, then the object itself is captured so D6 publishes what was
// read rather than what the name resolves to a moment later.
func (r *gateDriverRun) recheckTag() error {
	tagTree, err := gateTreeSHA(r.Repo.Dir, "refs/tags/"+r.Plan.Version)
	if err != nil {
		return fmt.Errorf("%w: the tag %s has no readable tree, so what is about to be pushed cannot be compared with what the gate read: %w", errGateUncheckable, r.Plan.Version, err)
	}
	if err := gateAssertMergeMatchesReceipt(r.Receipt, tagTree); err != nil {
		return fmt.Errorf("the tag about to be pushed does not cover the tree the gate verified: %w", err)
	}
	commit, err := gateResolve(r.Repo.Dir, "refs/tags/"+r.Plan.Version+"^{commit}")
	if err != nil {
		return fmt.Errorf("%w: the tag %s does not resolve to a commit: %w", errGateUncheckable, r.Plan.Version, err)
	}
	if commit != r.Merge {
		return fmt.Errorf("%w: the tag %s names commit %s, and the object this run handshaked is %s. "+
			"Something moved the tag between its creation and this check; publishing it would tag a commit nobody read",
			errGateTreeMismatch, r.Plan.Version, commit, r.Merge)
	}
	object, err := gateResolve(r.Repo.Dir, "refs/tags/"+r.Plan.Version)
	if err != nil {
		return fmt.Errorf("%w: the tag object for %s could not be read: %w", errGateUncheckable, r.Plan.Version, err)
	}
	r.TagObj = object
	return nil
}

// ---------------------------------------------------------------------
// the run record, written AFTER the fact and trusted by nothing
// ---------------------------------------------------------------------

// gateDriverRecord is the account a human reads afterwards. It is written after
// the sequence has finished, it is written for a stopped run as well as a
// completed one, and NOTHING reads it: the driver measures its receipt in-process
// precisely so no file can stand in for an inspection.
//
// It lives outside the repository by default (see the Makefile), for the reason
// the ci-evidence record does — the pre-tag phase requires `git status
// --porcelain` to be empty — and for a second one: gate/.gitignore is written as
// ignore-everything-then-name-the-tracked, so a record dropped in gate/ would be
// invisible to git, which is the wrong property for something a human is meant to
// find.
type gateDriverRecord struct {
	Version     string      `json:"version"`
	Branch      string      `json:"branch"`
	WrittenBy   string      `json:"written_by"`
	Completed   []string    `json:"completed"`
	FailedAt    string      `json:"failed_at,omitempty"`
	Verdict     string      `json:"verdict,omitempty"`
	MergeCommit string      `json:"merge_commit,omitempty"`
	Receipt     gateReceipt `json:"receipt"`
	Report      string      `json:"report,omitempty"`
}

func gateDriverWriteRecord(t *testing.T, path string, r *gateDriverRun) {
	t.Helper()
	record := gateDriverRecord{
		Version:     r.Plan.Version,
		Branch:      r.Repo.Branch,
		WrittenBy:   gateDriverTestName,
		Completed:   r.Done,
		FailedAt:    r.Failed,
		Verdict:     r.Verdict,
		MergeCommit: r.Merge,
		Receipt:     r.Receipt,
	}
	if r.Err != nil {
		record.Report = r.report()
	}
	blob, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatalf("marshal the run record: %v", err)
	}
	if err := os.WriteFile(path, append(blob, '\n'), 0o600); err != nil {
		t.Fatalf("write the run record to %s: %v\nThe record is what an operator reads to learn which irreversible steps completed; a run that cannot write one is reported as a failure", path, err)
	}
}

// ---------------------------------------------------------------------
// the stage
// ---------------------------------------------------------------------

// TestReleaseDriverPublishes is the driver as `make release-publish` invokes it,
// and as `go test ./...` also invokes it — which is the whole reason its first
// act is the authorization decision.
//
// UNAUTHORIZED, IT IS INERT AND SAYS SO IN AN ASSERTION rather than in a skip.
// The precedent next door (ciEvGateMode) skips when unset, and that is right for
// a read-only stage; here the inert path is the one that runs on every
// maintainer's machine, so it is the one that has to be checked. The sequence is
// entered with the ambient plan and must complete ZERO steps: not "we did not
// reach the publish code", but "the publish code was entered and refused".
func TestReleaseDriverPublishes(t *testing.T) {
	plan := gateDriverPlanFromEnv()
	if mode, why := gateDriverMode(plan); mode == gateDriverRefuse {
		t.Fatal(why)
	}

	root := surfaceRepoRoot(t)
	repo := gateDriverRepo{Dir: root, Branch: gateDriverReleaseBranch(), Base: "main", Remote: "origin"}
	run := gateDriverPublish(plan, repo, gateDriverUnwired{})

	if plan.Record != "" {
		gateDriverWriteRecord(t, plan.Record, run)
	}

	if run.Err == nil {
		t.Logf("%s published: %v", plan.Version, run.Done)
		return
	}

	// The inert case is asserted, not assumed: the sequence ran and did nothing.
	if mode, why := gateDriverMode(plan); mode == gateDriverInert {
		if run.Failed != "D0" || len(run.Done) != 0 {
			t.Fatalf("the driver was invoked with no release authorized and completed step(s) %v before stopping at %s. "+
				"An unauthorized invocation must perform NO step of the sequence — this is what a lane landing looks like from inside the driver, and every precondition below D0 would pass at that moment",
				run.Done, run.Failed)
		}
		t.Log(why)
		return
	}
	t.Fatal(run.report())
}

// gateDriverReleaseBranch is the branch the release is cut on. The current branch
// is the honest default — a driver that guessed a name would act on a branch
// nobody named — and the checks in D1 are what decide whether it is releasable.
func gateDriverReleaseBranch() string {
	branch, err := gateGit(".", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return branch
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// gateDriverFixtureEvidence is a gate run's outputs, supplied by a test. It is
// the seam W12b implements for real; a fixture on this side is what lets the
// spine be driven end to end before any of that exists.
type gateDriverFixtureEvidence struct {
	verdicts []gateSurfaceVerdict
	findings []gateFinding
	declared []string
	current  map[string]string
	archives error
	site     error
}

func (e gateDriverFixtureEvidence) Verdicts(string) ([]gateSurfaceVerdict, []gateFinding, error) {
	return e.verdicts, e.findings, nil
}

func (e gateDriverFixtureEvidence) Current(string) ([]string, map[string]string, error) {
	return e.declared, e.current, nil
}

func (e gateDriverFixtureEvidence) Archives(string, string) error { return e.archives }
func (e gateDriverFixtureEvidence) Site(string) error             { return e.site }

// gateDriverGreenEvidence is a clean run over one surface: a PASS whose
// fingerprint is the one this tree produces, no findings, and both post-tag
// checks satisfied.
func gateDriverGreenEvidence() gateDriverFixtureEvidence {
	return gateDriverFixtureEvidence{
		verdicts: gatePassingSurfaces("readme"),
		declared: []string{"readme"},
		current:  map[string]string{"readme": "sha256:readme"},
	}
}

// gateDriverFixture builds the converging topology with a real `origin` and
// returns the repo descriptor the driver acts on.
func gateDriverFixture(t *testing.T) gateDriverRepo {
	t.Helper()
	work := gateFixtureRepo(t)
	return gateDriverRepo{
		Dir:    work,
		Branch: "release",
		Base:   "main",
		Remote: "origin",
		Env: []string{
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_DATE=2026-08-09T00:00:00Z",
			"GIT_COMMITTER_DATE=2026-08-09T00:00:00Z",
		},
	}
}

// gateDriverAuthorized is a plan a human authorized for one version.
func gateDriverAuthorized(t *testing.T, version string) gateDriverPlan {
	t.Helper()
	return gateDriverPlan{Version: version, Authorize: version, Record: filepath.Join(t.TempDir(), "record.json")}
}

// gateDriverRemoteTag is the object `origin` carries for version, or "" when it
// carries none. Every refusal below is asserted THROUGH this rather than through
// the driver's own return value: "the driver reported a failure" and "nothing was
// published" are different claims, and only the second one is the invariant.
func gateDriverRemoteTag(t *testing.T, repo gateDriverRepo, version string) string {
	t.Helper()
	out := gateTestGit(t, repo.Dir, "ls-remote", "--tags", repo.Remote, "refs/tags/"+version)
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return strings.Fields(out)[0]
}

// gateDriverRemoteHead is what `origin` holds for a branch.
func gateDriverRemoteHead(t *testing.T, repo gateDriverRepo, branch string) string {
	t.Helper()
	out := gateTestGit(t, repo.Dir, "ls-remote", repo.Remote, "refs/heads/"+branch)
	if strings.TrimSpace(out) == "" {
		return ""
	}
	return strings.Fields(out)[0]
}

// gateDriverAssertNothingPublished is the behavioural half of every refusal in
// this file: the fixture's origin carries no tag for the version and its main has
// not moved.
func gateDriverAssertNothingPublished(t *testing.T, repo gateDriverRepo, version, mainWas string) {
	t.Helper()
	if tag := gateDriverRemoteTag(t, repo, version); tag != "" {
		t.Errorf("the refusal was reported and %s was published anyway: origin carries refs/tags/%s at %s", version, version, tag)
	}
	if now := gateDriverRemoteHead(t, repo, repo.Base); now != mainWas {
		t.Errorf("the refusal was reported and origin's %s moved from %s to %s", repo.Base, mainWas, now)
	}
}

// TestTheAuthorizationDecisionIsExhaustive walks every shape an environment can
// have, because the danger here is inverted from every other gate in this
// repository: the wrong answer does not clear a release, it PERFORMS one.
//
// The row that matters most is the first: nothing set is INERT, and inert is not
// a synonym for "we did not check". `go test ./...` runs this file at every lane
// landing on the maintainer's machine with the maintainer's push credentials, and
// at that moment the gate can be green, the ancestry can hold and the trees can
// match — so a driver keyed to its preconditions being SATISFIABLE rather than to
// a human having AUTHORIZED would merge, tag and push on the next lane landed on
// a converging release branch.
func TestTheAuthorizationDecisionIsExhaustive(t *testing.T) {
	const record = "/tmp/record.json"
	for _, tc := range []struct {
		name string
		plan gateDriverPlan
		want gateDriverDecision
		why  string
	}{
		{"a lane landing: nothing was asked for", gateDriverPlan{}, gateDriverInert,
			"`go test ./...` on a maintainer's machine. Every precondition below D0 can be green here, which is exactly why the guard is about the caller and not about the tree"},
		{"a version with nobody authorizing it", gateDriverPlan{Version: "v9.9.9", Record: record}, gateDriverRefuse,
			"a script that learned half the recipe"},
		{"an authorization with no release attached", gateDriverPlan{Authorize: "v9.9.9", Record: record}, gateDriverRefuse,
			"a standing authorization is an environment where the next invocation to supply a version publishes"},
		{"an authorization left over from the last release", gateDriverPlan{Version: "v9.9.9", Authorize: "v9.9.8", Record: record}, gateDriverRefuse,
			"a human authorizes one release, not releases in general"},
		{"authorized, with nowhere to record what it did", gateDriverPlan{Version: "v9.9.9", Authorize: "v9.9.9"}, gateDriverRefuse,
			"a release that stops between the two pushes and leaves no account sends the operator in blind"},
		{"a human named this release", gateDriverPlan{Version: "v9.9.9", Authorize: "v9.9.9", Record: record}, gateDriverGo, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := gateDriverMode(tc.plan)
			if got != tc.want {
				t.Fatalf("gateDriverMode(%+v) = %d, want %d. %s", tc.plan, got, tc.want, tc.why)
			}
			if tc.want != gateDriverGo && why == "" {
				t.Fatal("the driver stopped without saying why. A refusal nobody can read is one somebody deletes")
			}
		})
	}
}

// TestTheDriverPublishesNothingUnlessAHumanAuthorizedIt is the same table asked
// BEHAVIOURALLY, against a fixture with a real origin: for every unauthorized
// shape, the sequence is entered, completes zero steps, and the forge is
// untouched.
//
// The distinction it holds is the one a text assertion cannot: "gateDriverMode
// returned refuse" and "nothing was published" are different claims. A driver
// that computed the refusal and then ran the merge anyway satisfies the first.
func TestTheDriverPublishesNothingUnlessAHumanAuthorizedIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan gateDriverPlan
	}{
		{"a lane landing", gateDriverPlan{}},
		{"a version nobody authorized", gateDriverPlan{Version: "v9.9.9", Record: "/tmp/record.json"}},
		{"an authorization for another release", gateDriverPlan{Version: "v9.9.9", Authorize: "v9.9.8", Record: "/tmp/record.json"}},
		{"authorized with nowhere to record it", gateDriverPlan{Version: "v9.9.9", Authorize: "v9.9.9"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gateDriverFixture(t)
			mainWas := gateDriverRemoteHead(t, repo, repo.Base)

			run := gateDriverPublish(tc.plan, repo, gateDriverGreenEvidence())

			if run.Err == nil {
				t.Fatal("the driver published a release nobody authorized")
			}
			if run.Failed != "D0" {
				t.Errorf("the run stopped at %s; an unauthorized invocation must stop at D0, before a single git command runs", run.Failed)
			}
			if len(run.Done) != 0 {
				t.Errorf("the run completed step(s) %v for a release nobody authorized", run.Done)
			}
			gateDriverAssertNothingPublished(t, repo, "v9.9.9", mainWas)
			if local := gateTestGit(t, repo.Dir, "tag", "--list"); strings.TrimSpace(local) != "" {
				t.Errorf("the unauthorized run created local tag(s) %q; it must not have reached D4 at all", local)
			}
			if !strings.Contains(run.report(), "ALREADY PUBLISHED: nothing") {
				t.Errorf("the report of a run that published nothing must say so in as many words; got:\n%s", run.report())
			}
		})
	}
}

// TestTheDriverPublishesInOrderAndOnlyWhatItHandshaked is the happy path, and
// the ORDER is what it exists to assert.
//
// The order is not cosmetic and it is not the written procedure's: the release
// branch edits site/src/content.ts, so pushing main fires deploy-site.yml and
// publishes a page reading "the current release is vX.Y.Z" while no tag and no
// archive exists, because release.yml fires only on a tag push. The assertion is
// made from inside the window — a hook fires the moment D6 completes, and at that
// moment origin must carry the tag and must NOT have moved main.
func TestTheDriverPublishesInOrderAndOnlyWhatItHandshaked(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)

	plan := gateDriverAuthorized(t, version)
	var sawTagBeforeMain bool

	run := gateDriverPublishWithHooks(plan, repo, gateDriverGreenEvidence(), map[string]func(){
		"D6": func() {
			if gateDriverRemoteTag(t, repo, version) == "" {
				t.Error("D6 completed and origin carries no tag, so the step reported an act it did not perform")
			}
			if gateDriverRemoteHead(t, repo, repo.Base) != mainWas {
				t.Error("origin's main moved before the tag push completed. The order is the audit's finding: main first publishes a site announcing a release whose archives do not exist yet")
			}
			sawTagBeforeMain = true
		},
	})

	if run.Err != nil {
		t.Fatalf("a clean, authorized, converging release was refused:\n%s", run.report())
	}
	if !sawTagBeforeMain {
		t.Fatal("D6 never completed, so nothing observed the window between the two pushes")
	}
	if got := strings.Join(run.Done, " "); got != "D0 D1 D2 D3 D4 D5 D6 D7 D8 D9" {
		t.Fatalf("the sequence ran %q; every step must run, in order — a release that skipped one reported a check it did not make", got)
	}

	// The object published is the object handshaked, asserted on the forge rather
	// than in this process.
	tagObject := gateDriverRemoteTag(t, repo, version)
	if tagObject == "" {
		t.Fatal("the run completed and origin carries no tag")
	}
	tagCommit := gateTestGit(t, repo.Dir, "rev-parse", tagObject+"^{commit}")
	if tagCommit != run.Merge {
		t.Errorf("origin's tag names commit %s and the run handshaked %s", tagCommit, run.Merge)
	}
	tagTree := gateTestGit(t, repo.Dir, "rev-parse", tagObject+"^{tree}")
	if tagTree != run.Receipt.Tree {
		t.Errorf("origin's tag covers tree %s and the receipt records %s", tagTree, run.Receipt.Tree)
	}
	if now := gateDriverRemoteHead(t, repo, repo.Base); now != run.Merge {
		t.Errorf("origin's %s is %s; the merge commit this run handshaked is %s", repo.Base, now, run.Merge)
	}
}

// gateDriverPublishWithHooks runs the sequence with fault injection between
// steps. It exists so the two windows this driver is built around — merge to tag,
// and tag to push — can be made real in a test instead of argued about in a
// comment.
func gateDriverPublishWithHooks(plan gateDriverPlan, repo gateDriverRepo, ev gateDriverEvidence, after map[string]func()) *gateDriverRun {
	return gateDriverExecute(&gateDriverRun{Plan: plan, Repo: repo, after: after}, ev)
}

// TestTheDriverTagsTheMergeItHandshakedAndNeverHEAD is failure 4, reproduced.
//
// `git tag -a vX.Y.Z -m …` with no ref tags HEAD, which is right only when
// nothing has landed since the merge. So the fixture lands something: a commit
// arrives on main in the window between D2 and D4, exactly as another operator's
// merge or a bot's push would. A driver that tagged HEAD would then tag a tree
// nobody read, and every check it made before that stays green.
func TestTheDriverTagsTheMergeItHandshakedAndNeverHEAD(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t)
	plan := gateDriverAuthorized(t, version)

	var intruder string
	run := gateDriverPublishWithHooks(plan, repo, gateDriverGreenEvidence(), map[string]func(){
		"D3": func() {
			gateWrite(t, repo.Dir, "landed-after-the-merge.md", "somebody else's PR\n")
			gateTestGit(t, repo.Dir, "add", "-A")
			gateTestGit(t, repo.Dir, "commit", "-qm", "docs: landed while the release was mid-flight")
			intruder = gateTestGit(t, repo.Dir, "rev-parse", "HEAD")
		},
	})
	if run.Err != nil {
		t.Fatalf("the release was refused:\n%s", run.report())
	}
	if intruder == "" || intruder == run.Merge {
		t.Fatalf("the fixture did not land a commit after the merge, so this test cannot tell a tag on the merge from a tag on HEAD (intruder=%q, merge=%q)", intruder, run.Merge)
	}

	tagObject := gateDriverRemoteTag(t, repo, version)
	if tagObject == "" {
		t.Fatal("origin carries no tag")
	}
	tagged := gateTestGit(t, repo.Dir, "rev-parse", tagObject+"^{commit}")
	if tagged == intruder {
		t.Fatalf("the tag names %s, the commit that landed AFTER the merge — its tree carries content the gate never read, and the receipt still says PASS", intruder)
	}
	if tagged != run.Merge {
		t.Fatalf("the tag names %s and the run handshaked %s", tagged, run.Merge)
	}
	// And main was published by value too: the intruder rode along on the branch,
	// but the object pushed is the one that was read.
	if now := gateDriverRemoteHead(t, repo, repo.Base); now != run.Merge {
		t.Errorf("origin's %s is %s, not the handshaked merge commit %s — the push named a branch rather than the object", repo.Base, now, run.Merge)
	}
}

// TestThePreTagHandshakeRefusesATagThatMoved is the assertion D5 exists for, and
// it is the one the spec asks to be mutation-proved: delete the recheckTag call
// from gateDriverPublish and this test goes red because a tag covering a tree the
// gate never read reaches the forge.
//
// The window is real. Between creating the tag and pushing it, a tag is just a
// local ref: another process force-moves it, a stale local tag is resurrected by a
// script, an operator in a second terminal "fixes" the name. D3 cannot see that —
// it asked what the MERGE holds, and the merge still holds it. D5 asks what the
// ref about to be published points at.
func TestThePreTagHandshakeRefusesATagThatMoved(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)
	plan := gateDriverAuthorized(t, version)

	run := gateDriverPublishWithHooks(plan, repo, gateDriverGreenEvidence(), map[string]func(){
		"D4": func() {
			gateWrite(t, repo.Dir, "not-in-the-tagged-tree.md", "content nobody read\n")
			gateTestGit(t, repo.Dir, "add", "-A")
			gateTestGit(t, repo.Dir, "commit", "-qm", "chore: something else entirely")
			gateTestGit(t, repo.Dir, "tag", "-f", "-a", version, "-m", version, "HEAD")
		},
	})

	if run.Err == nil {
		t.Fatal("the driver pushed a tag that had moved after it was created; nothing compared what was about to be published against what the gate read")
	}
	if run.Failed != "D5" {
		t.Fatalf("the run stopped at %s, not at D5. D3 cannot catch this: the merge commit still holds the tree it held", run.Failed)
	}
	if !errors.Is(run.Err, errGateTreeMismatch) && !errors.Is(run.Err, errGateUncheckable) {
		t.Errorf("the refusal must be the handshake's own sentinel; got %v", run.Err)
	}
	gateDriverAssertNothingPublished(t, repo, version, mainWas)
	if !strings.Contains(run.report(), "ALREADY PUBLISHED: nothing") {
		t.Errorf("a run that refused before D6 must report that nothing was published; got:\n%s", run.report())
	}
}

// TestTheDriverNamesWhatIsAlreadyPublishedWhenItStopsHalfway is clause 4 of the
// invariant, at the one place it cannot be recovered from.
//
// D7 is unbuilt (see gateDriverUnwired.Archives) and therefore
// errGateUncheckable, so an authorized run stops after the tag push, forever,
// until W12b lands. That is the correct state and not a defect — a driver that
// pushed main without verifying the archives would be narrowing coverage silently
// at the only moment it matters — but it means the report is the whole
// deliverable at that point.
func TestTheDriverNamesWhatIsAlreadyPublishedWhenItStopsHalfway(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)
	plan := gateDriverAuthorized(t, version)

	evidence := gateDriverGreenEvidence()
	evidence.archives = gateDriverUnwired{}.Archives(version, "the merge commit")

	run := gateDriverPublish(plan, repo, evidence)

	if run.Err == nil {
		t.Fatal("a step whose machinery does not exist reported success")
	}
	if run.Failed != "D7" {
		t.Fatalf("the run stopped at %s, want D7", run.Failed)
	}
	if !errors.Is(run.Err, errGateUncheckable) {
		t.Errorf("a step that could not run must be uncheckable, never a pass and never a mismatch; got %v", run.Err)
	}

	// The tag IS published, and main is not. Both halves are asserted on the
	// forge, because that is what the operator is about to go looking at.
	if gateDriverRemoteTag(t, repo, version) == "" {
		t.Error("the run reached D7 and origin carries no tag, so D6 reported an act it did not perform")
	}
	if now := gateDriverRemoteHead(t, repo, repo.Base); now != mainWas {
		t.Errorf("origin's %s moved to %s after a failure at D7; nothing downstream of a failed step may execute", repo.Base, now)
	}

	report := run.report()
	for _, want := range []struct{ fragment, why string }{
		{"ALREADY PUBLISHED", "the operator's first question after a failed release is what is already on the forge, and a bare non-zero exit sends them in blind"},
		{"D6 push the tag", "the step that published must be named, not implied"},
		{"the tag " + version + " is on the forge", "named in plain words and with the version in it, because the reader is looking at a forge with several tags on it"},
		{"NOT PUBLISHED", "what did NOT happen is half the state, and it is the half that decides whether a site is announcing a release that does not exist"},
		{"D8 push main", "the step that did not run must be named too"},
		{"RE-ENTRY", "the next question is always whether to run it again"},
		{"will not resume", "the answer is no, and it has to be in the report rather than in a design document"},
	} {
		if !strings.Contains(report, want.fragment) {
			t.Errorf("the halfway report does not name %q. %s.\nThe report reads:\n%s", want.fragment, want.why, report)
		}
	}
}

// TestTheDriverRefusesToReEnterAPartlyPublishedRelease is the second invocation:
// the tag is on the forge and D7 failed, and somebody runs the target again.
//
// It refuses, before it merges anything, and prints the two lists. It does not
// resume — the steps before the tag were performed by a process this one cannot
// see, and trusting them is exactly the paper-over-inspection failure the whole
// file is built against — and it does not delete the tag, because the gate
// surfaces and never fixes.
func TestTheDriverRefusesToReEnterAPartlyPublishedRelease(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)
	plan := gateDriverAuthorized(t, version)

	// The first run: it publishes the tag and stops at D7.
	first := gateDriverPublish(plan, repo, func() gateDriverFixtureEvidence {
		e := gateDriverGreenEvidence()
		e.archives = errors.New("the Release workflow failed")
		return e
	}())
	if first.Failed != "D7" {
		t.Fatalf("the first run stopped at %s, so this fixture is not a partly-published release", first.Failed)
	}
	tagWas := gateDriverRemoteTag(t, repo, version)
	if tagWas == "" {
		t.Fatal("the first run published no tag, so there is no re-entry to test")
	}

	// The second run, with everything now working.
	second := gateDriverPublish(plan, repo, gateDriverGreenEvidence())

	if second.Err == nil {
		t.Fatal("the driver resumed a partly-published release, publishing main on the strength of steps performed by a process it cannot see")
	}
	if second.Failed != "D1" {
		t.Fatalf("the re-entry stopped at %s; it must refuse in the precondition, before it merges anything", second.Failed)
	}
	if !errors.Is(second.Err, errGateUncheckable) {
		t.Errorf("a re-entry must be reported as a check that cannot be made, not as a tree or ancestry problem; got %v", second.Err)
	}
	for _, want := range []struct{ fragment, why string }{
		{"ALREADY PUBLISHED", "the refusal's whole job is to say which irreversible steps happened"},
		{"D6 push the tag — DONE", "the tag is on the forge and cannot be taken back"},
		{"D8 push main", "and whether main followed it is the other half of the state"},
		{"refuses rather than resuming", "a driver that resumed would be publishing on the strength of steps it did not perform"},
	} {
		if !strings.Contains(second.Err.Error(), want.fragment) {
			t.Errorf("the re-entry refusal does not name %q. %s.\nIt reads:\n%v", want.fragment, want.why, second.Err)
		}
	}

	// And it undid nothing: the tag is where it was, main is where it was.
	if now := gateDriverRemoteTag(t, repo, version); now != tagWas {
		t.Errorf("the re-entry moved or deleted the published tag (%s -> %q). The gate surfaces; it never fixes", tagWas, now)
	}
	if now := gateDriverRemoteHead(t, repo, repo.Base); now != mainWas {
		t.Errorf("the re-entry pushed %s to %s", repo.Base, now)
	}
}

// TestTheDriverRefusesAGateThatIsNotGreen covers clause 2 from the other side:
// green is RECOMPUTED, and every way of failing to be green stops the release
// before the merge.
//
// The rows are the shapes evaluate distinguishes and an "is the findings array
// empty?" shortcut does not. A driver written to that shortcut passes rows two
// through four, and each of them is a tree that no agent cleared.
func TestTheDriverRefusesAGateThatIsNotGreen(t *testing.T) {
	const version = "v9.9.9"
	for _, tc := range []struct {
		name     string
		evidence func(gateDriverFixtureEvidence) gateDriverFixtureEvidence
		why      string
	}{
		{"a finding reached the report", func(e gateDriverFixtureEvidence) gateDriverFixtureEvidence {
			e.findings = []gateFinding{{Surface: "readme", Rule: "undocumented-flag", Severity: "minor", Detail: "--strict is not described"}}
			return e
		}, "a human confirms what blocks a release; the driver does not clear itself"},
		{"a declared surface holds no verdict", func(e gateDriverFixtureEvidence) gateDriverFixtureEvidence {
			e.declared = []string{"readme", "site"}
			e.current = map[string]string{"readme": "sha256:readme", "site": "sha256:site"}
			return e
		}, "the findings array is empty and one surface was never looked at — this is the row the shortcut clears"},
		{"a surface holds a FAIL", func(e gateDriverFixtureEvidence) gateDriverFixtureEvidence {
			e.verdicts = []gateSurfaceVerdict{{Surface: "readme", Verdict: gateVerdictFailed, Fingerprint: "sha256:readme"}}
			return e
		}, "a verdict that is not PASS"},
		{"a PASS fingerprinted against another tree", func(e gateDriverFixtureEvidence) gateDriverFixtureEvidence {
			e.current = map[string]string{"readme": "sha256:some-other-tree"}
			return e
		}, "it looked, and it looked at a different tree — the receipt's PASS is attached to nothing here"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gateDriverFixture(t)
			mainWas := gateDriverRemoteHead(t, repo, repo.Base)

			run := gateDriverPublish(gateDriverAuthorized(t, version), repo, tc.evidence(gateDriverGreenEvidence()))

			if run.Err == nil {
				t.Fatalf("the driver published on a gate that is not green: %s", tc.why)
			}
			if run.Failed != "D1" {
				t.Errorf("the run stopped at %s; a gate that is not green must stop in the precondition, before the merge", run.Failed)
			}
			if run.Verdict == gateVerdictPass {
				t.Errorf("the recomputed verdict is %s. %s", run.Verdict, tc.why)
			}
			gateDriverAssertNothingPublished(t, repo, version, mainWas)
		})
	}
}

// TestTheDriverStopsBeforeTheMergeWhenItsEvidenceIsUnwired is the state this file
// lands in, asserted rather than described: pointed at a real gate run source
// that does not exist yet, an authorized driver refuses at D1 and publishes
// nothing.
//
// It is the failure-not-skip rule turned on the driver itself. The alternative —
// treating an absent verdict transport as "no findings, therefore green" — is a
// release published on evidence nobody produced, and it is the single most
// natural thing for the next implementer to write.
func TestTheDriverStopsBeforeTheMergeWhenItsEvidenceIsUnwired(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)

	run := gateDriverPublish(gateDriverAuthorized(t, version), repo, gateDriverUnwired{})

	if run.Err == nil {
		t.Fatal("the driver published a release with no gate run behind it")
	}
	if run.Failed != "D1" || !errors.Is(run.Err, errGateUncheckable) {
		t.Fatalf("an unwired evidence source must stop the run at D1 as uncheckable; got %s / %v", run.Failed, run.Err)
	}
	if !strings.Contains(run.Err.Error(), "will not invent them") {
		t.Errorf("the refusal must say the driver does not invent the verdicts it was not given; got:\n%v", run.Err)
	}
	gateDriverAssertNothingPublished(t, repo, version, mainWas)
}

// ---------------------------------------------------------------------
// the structural pins
// ---------------------------------------------------------------------

// gateDriverAST parses this file. Parsed and not grepped, for the reason
// gateDeclaredTests gives: every phrase these pins forbid appears in this file's
// own comments explaining why it is forbidden, and a text search would fail on
// the explanation.
func gateDriverAST(t *testing.T) (*token.FileSet, *ast.File) {
	t.Helper()
	path := filepath.Join(surfaceRepoRoot(t), filepath.FromSlash(gateDriverFile))
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", gateDriverFile, err)
	}
	return fset, file
}

// TestTheDriverCanBeHandedNoReceipt is clause 1 of the invariant, held
// structurally rather than by anyone remembering it.
//
// THE FAILURE IT EXISTS FOR is on disk in this repository right now. A complete,
// committed, hand-editable gateReceipt is nested inside gate-cost-ledger.json's
// newest run entry. The most natural driver anybody would write — "take the
// newest ledger entry for this version, check its receipt's tree still equals
// what is about to be tagged, publish" — reads that JSON, and every check it then
// makes passes, because a receipt is fields and fields are what an editor writes.
// It even looks principled: it reuses the run's own record instead of inventing a
// second one. A forged receipt cannot be handed to a function that does not take
// one, so the rule is enforced on the SHAPE of this file:
//
//   - no function here accepts a gateReceipt, by value or by pointer;
//   - no gateReceipt is constructed here as a literal;
//   - nothing here decodes JSON at all;
//   - and gateRecordReceipt is called, so the receipt exists by measurement.
//
// gateDriverRun.Receipt is a FIELD, which is the deliberate exception: it is
// written by exactly one statement in this file — the one that assigns
// gateRecordReceipt's result — while a parameter is a doorway.
func TestTheDriverCanBeHandedNoReceipt(t *testing.T) {
	fset, file := gateDriverAST(t)
	at := func(n ast.Node) string { return fset.Position(n.Pos()).String() }

	isReceipt := func(expr ast.Expr) bool {
		if star, ok := expr.(*ast.StarExpr); ok {
			expr = star.X
		}
		ident, ok := expr.(*ast.Ident)
		return ok && ident.Name == "gateReceipt"
	}

	var recorded int
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Type.Params == nil {
				return true
			}
			for _, param := range node.Type.Params.List {
				if isReceipt(param.Type) {
					t.Errorf("%s takes a gateReceipt parameter (%s).\n"+
						"That parameter is the doorway this whole file is built to remove: a receipt assembled anywhere else — decoded from gate-cost-ledger.json, typed into a file, printed by a tool that inspected nothing — becomes an input to the publish path, and every check made on it afterwards passes. "+
						"The driver measures its receipt by calling gateRecordReceipt in this process; it never accepts one",
						node.Name.Name, at(param))
				}
			}
		case *ast.CompositeLit:
			if isReceipt(node.Type) {
				t.Errorf("a gateReceipt literal is constructed at %s.\n"+
					"A receipt in this file may come into existence exactly one way — gateRecordReceipt, which resolves the head and the tree from git itself and re-asserts the ancestry precondition before it returns anything. A literal is a receipt nobody measured",
					at(node))
			}
		case *ast.SelectorExpr:
			pkg, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}
			if (pkg.Name == "json" || pkg.Name == "yaml") && (node.Sel.Name == "Unmarshal" || node.Sel.Name == "NewDecoder") {
				t.Errorf("%s.%s is called at %s. Nothing in the publish path decodes a document: the receipt is measured here, and a decoder is how a measured receipt becomes an accepted one. Marshalling — writing the run record afterwards — is fine, because nothing reads it",
					pkg.Name, node.Sel.Name, at(node))
			}
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "gateRecordReceipt" {
				recorded++
			}
		}
		return true
	})

	if recorded == 0 {
		t.Errorf("%s never calls gateRecordReceipt. The receipt the driver publishes against must be MEASURED in this process — resolved from git, with the ancestry re-asserted — and a driver that measures none is publishing against nothing at all", gateDriverFile)
	}
}

// TestTheDriverDoesNotPretendToDetectAHuman pins the guard this file must NOT
// grow, because somebody will otherwise write it in good faith.
//
// Decision 3 says a human initiates every release, and the obvious implementation
// — os.Stdin.Stat() with Mode()&os.ModeCharDevice — is SATISFIED by the exact
// invocation shape it exists to refuse. Under `go test`, stdin is /dev/null, and
// /dev/null is a character device: measured on this toolchain as
// MODE=Dcrw-rw-rw-, CHARDEV=true, with a read returning EOF immediately. The
// guard would report "a human is present" for every automated caller, and a guard
// that is read as a barrier while refusing nothing is worse than no guard.
//
// So the honest position is stated in the refusal text instead: the authorization
// is what a human typed, and the barrier between an agent and this target belongs
// to the harness's permission system, outside this repository and outside every
// test in it.
func TestTheDriverDoesNotPretendToDetectAHuman(t *testing.T) {
	_, file := gateDriverAST(t)

	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "ModeCharDevice", "ModeDevice", "IsTerminal":
			t.Errorf("this file tests for a terminal (%s). Under `go test` stdin is /dev/null, which IS a character device, so the check answers \"a human is present\" for the automated caller it exists to refuse — and it then reads, to everyone after, as the barrier that is missing. "+
				"The authorization here is the version a human typed twice on a named target; the barrier against an agent is the harness's permission system, which lives outside this repository", sel.Sel.Name)
		}
		return true
	})

	// And the refusal has to SAY that, because a residual nobody wrote down is a
	// residual somebody closes badly.
	_, why := gateDriverMode(gateDriverPlan{})
	for _, want := range []struct{ fragment, why string }{
		{"harness's permission system", "the barrier is named, and named as something that is not in this repository"},
		{"/dev/null", "the specific reason the obvious guard does not work, so the next person does not write it"},
	} {
		if !strings.Contains(why, want.fragment) {
			t.Errorf("the inert message does not name %q. %s.\nIt reads:\n%s", want.fragment, want.why, why)
		}
	}
}

// ---------------------------------------------------------------------
// the invocation
// ---------------------------------------------------------------------

// gateDriverMakeTargetRE isolates the Makefile recipe: the target line and every
// tab-indented line under it.
var gateDriverMakeTargetRE = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(gateDriverMakeTarget) + `:[^\n]*\n(?:\t[^\n]*\n?)+`)

// gateDriverJoinContinuations turns a make recipe into the lines a shell sees, by
// joining backslash continuations. Every question below is about a COMMAND, and a
// command that spans four physical lines is one command.
func gateDriverJoinContinuations(recipe string) string {
	return strings.ReplaceAll(recipe, "\\\n", " ")
}

// gateDriverGoTestLine returns the recipe's logical line that invokes `go test`,
// and the commands that line parses into.
func gateDriverGoTestLine(recipe string) (line string, commands [][]string) {
	for _, candidate := range strings.Split(gateDriverJoinContinuations(recipe), "\n") {
		parsed := gateCommands(candidate)
		for _, argv := range parsed {
			name, rest, ok := gateCommandName(argv)
			if ok && name == "go" && len(rest) > 0 && rest[0] == "test" {
				return candidate, parsed
			}
		}
	}
	return "", nil
}

// gateDriverGoTestArgs is the `go test` argument list on that line.
func gateDriverGoTestArgs(commands [][]string) []string {
	for _, argv := range commands {
		if name, rest, ok := gateCommandName(argv); ok && name == "go" && len(rest) > 0 && rest[0] == "test" {
			return rest
		}
	}
	return nil
}

// gateDriverFlagValue reads `-name value` and `-name=value` alike, because a
// check that read one spelling would be defeated by typing the other.
func gateDriverFlagValue(args []string, name string) (string, bool) {
	for i, arg := range args {
		switch {
		case arg == name:
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		case strings.HasPrefix(arg, name+"="):
			return strings.TrimPrefix(arg, name+"="), true
		}
	}
	return "", false
}

// TestTheReleaseInvocationCannotSucceedHavingDoneNothing holds the release-time
// invocation and this stage to the same identifiers, and pins the three
// properties an invocation of an ACTING stage needs that a read-only one does
// not.
//
// It PARSES the recipe rather than searching it for words — gateArgv,
// gateCommands and gateCommandName are the existing machinery, and the difference
// matters here more than usual: `echo "-count=1"` contains the string this check
// wants and is not a flag at all.
//
// THE THREE PROPERTIES, each with the release it saves:
//
//   - `-count=1`. `go test` caches a successful package result and replays it,
//     and subprocess effects are not tracked inputs — measured: same env, `ok
//     (cached)` in under a second with the subprocess not run. So the driver
//     publishes v0.6.0 and exits 0; the operator, paged away or simply wanting to
//     see it through, runs the same command for the same version again; it prints
//     `ok (cached)` and exits 0 having merged nothing, tagged nothing and pushed
//     nothing. Exit 0 then answers "did the site get pushed?" with "yes" on
//     behalf of a process that did not run.
//   - an explicit `-timeout`, above a floor. The default is ten minutes and the
//     answer to exceeding it is a PANIC. D7 waits for six GOOS/GOARCH archives to
//     build and then verifies them, which routinely exceeds ten. On the default,
//     the binary panics between the tag push and the main push: goroutine dump,
//     tag on the forge, main behind, and the per-step report this file owes a
//     human never printed.
//   - the invocation stands ALONE on its line. `go test … || true` and a `-`
//     prefix both make the driver's refusals invisible to the target, which is the
//     one thing a driver's invocation may never allow.
//
// The two record guards are the ci-evidence recipe's, for the same reason it
// gives: `go test` exits 0 for a skip and for a selector that matches nothing, so
// an exit status alone cannot tell "published" from "did nothing".
func TestTheReleaseInvocationCannotSucceedHavingDoneNothing(t *testing.T) {
	root := surfaceRepoRoot(t)

	// (1) The name the recipe selects must be declared here, or `go test -run`
	// prints `ok … [no tests to run]` and exits 0 for every release after.
	stage := gateReadRepoFile(t, root, filepath.FromSlash(gateDriverFile))
	if !strings.Contains(stage, "func "+gateDriverTestName+"(t *testing.T)") {
		t.Fatalf("%s declares no `func %s(t *testing.T)`, and that is the name the Makefile selects", gateDriverFile, gateDriverTestName)
	}

	// (2) The recipe.
	makefile := gateReadRepoFile(t, root, "Makefile")
	recipe := gateDriverMakeTargetRE.FindString(makefile)
	if recipe == "" {
		t.Fatalf("the Makefile declares no `%s:` target with a recipe. That target IS the authorization: the driver publishes when a human runs it and at no other time, and it is also the only thing that turns `go test`'s exit 0 over a cached or empty run into a failed release",
			gateDriverMakeTarget)
	}

	line, commands := gateDriverGoTestLine(recipe)
	if line == "" {
		t.Fatalf("the `%s` recipe runs no `go test` at all, so nothing in it invokes the driver.\nThe recipe reads:\n%s", gateDriverMakeTarget, recipe)
	}

	// The invocation stands alone. `go test … || true` parses as two commands on
	// this line, and a `-` prefix tells make to ignore the exit status — either
	// way the driver's refusal stops being the target's refusal.
	if len(commands) != 1 {
		t.Errorf("the line that invokes the driver runs %d commands. A driver's invocation may not be followed by anything that can swallow its exit status — `|| true` is the shape — because every refusal in this file reaches the operator only through that status.\nThe line reads:\n%s",
			len(commands), strings.TrimSpace(line))
	}
	if trimmed := strings.TrimLeft(line, "\t @"); strings.HasPrefix(trimmed, "-") {
		t.Errorf("the driver's invocation is prefixed with `-`, which tells make to ignore its exit status. A release whose driver can refuse without failing the target is a release that publishes on a refusal.\nThe line reads:\n%s", strings.TrimSpace(line))
	}

	args := gateDriverGoTestArgs(commands)
	if count, ok := gateDriverFlagValue(args, "-count"); !ok || count != "1" {
		t.Errorf("the invocation does not pass `-count=1`. `go test` replays a cached success — measured, with the subprocess not re-run — so a second invocation for the same version prints `ok (cached)` in under a second and exits 0 having merged, tagged and pushed nothing. That exit status is then read as \"the release happened\".\nIt passes: %v", args)
	}

	switch value, ok := gateDriverFlagValue(args, "-timeout"); {
	case !ok:
		t.Errorf("the invocation passes no `-timeout`. The default is ten minutes and exceeding it PANICS the test binary — between the tag push and the main push, which is where waiting for six archives to build puts it. The operator then gets a goroutine dump instead of the report naming what is already published.\nIt passes: %v", args)
	default:
		d, err := time.ParseDuration(value)
		if err != nil {
			t.Errorf("the invocation's `-timeout %s` is not a duration go test accepts: %v", value, err)
		} else if d < gateDriverTimeoutFloor {
			t.Errorf("the invocation passes `-timeout %s`, below the %s floor. D7 waits for the Release workflow to build six GOOS/GOARCH archives and then verifies them; a timeout that expires there panics the binary with the tag published and main behind it", value, gateDriverTimeoutFloor)
		}
	}

	if selector, ok := gateDriverFlagValue(args, "-run"); !ok || !strings.Contains(selector, gateDriverTestName) {
		t.Errorf("the invocation's `-run` selector (%q) does not name %s. A selector that matches nothing prints `ok … [no tests to run]` and exits 0", selector, gateDriverTestName)
	}
	if !gateDriverHasArg(args, gateDriverPackage) && !gateDriverHasArg(args, strings.TrimSuffix(gateDriverPackage, "/")) {
		t.Errorf("the invocation does not name %s. The selector matches nothing in any other package, and `go test` exits 0 when it matches nothing.\nIt passes: %v", gateDriverPackage, args)
	}

	// (3) The mechanisms, each pinned as the LINE it is rather than as a name.
	// The variable names appear all over the recipe — in the guards, in the grep,
	// in the rm — so requiring the name proves only that the recipe talks about
	// it. Make exports nothing to a recipe's environment, so these three
	// assignments are what actually hand the plan to the driver.
	for _, want := range []struct{ fragment, why string }{
		{gateDriverVersionEnv + `="$(` + gateDriverVersionEnv + `)"`,
			"make does not export its variables, so this line is what tells the driver which release. Without it the driver is inert and `go test` exits 0"},
		{gateDriverAuthorizeEnv + `="$(` + gateDriverAuthorizeEnv + `)"`,
			"the human's authorization, handed over the same way. Without it the driver refuses — correctly — but the recipe would be relying on that refusal instead of doing its job"},
		{gateDriverRecordEnv + `="$(` + gateDriverRecordEnv + `)"`,
			"without a record path the driver refuses, and with one it writes the account of which irreversible steps completed — the only thing an operator has to read after a release stops halfway"},
		{`rm -f "$(` + gateDriverRecordEnv + `)"`,
			"a record left by the PREVIOUS release is a file that exists and is not empty. Removing it first is what makes the guard below an assertion about this run"},
		{`test -s "$(` + gateDriverRecordEnv + `)"`,
			"this is the line that turns a cached `ok`, a skipped test and an empty selection into a failed target. The record is the only positive evidence the driver ran at all"},
		{`grep -q "$(` + gateDriverVersionEnv + `)" "$(` + gateDriverRecordEnv + `)"`,
			"and this is the line that requires the record to be about the release that was asked for, rather than one left behind by another"},
	} {
		if !strings.Contains(recipe, want.fragment) {
			t.Errorf("the `%s` recipe no longer carries %q. %s.\nThe recipe reads:\n%s", gateDriverMakeTarget, want.fragment, want.why, recipe)
		}
	}
}

func gateDriverHasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// the written procedure
// ---------------------------------------------------------------------

// gateDriverTaggingItemRE isolates docs/RELEASING.md's tag-and-push item, from
// its bolded title to the next item at the same level. Scoped to the item rather
// than searched whole-file for the reason gateAncestryItemRE gives: the document
// discusses tagging and pushing in several places, and the question is what the
// item a maintainer reads TELLS THEM TO DO.
var gateDriverTaggingItemRE = regexp.MustCompile(`(?s)- \[ \] \*\*Tag and push, in the driver's order\.\*\*.*?\n- \[ \] `)

// TestTheWrittenProcedureAndTheDriverAgreeAboutThePushOrder is CLAUDE.md's
// one-procedure rule applied to the thing this lane changes about the procedure.
//
// The driver's order is the OPPOSITE of the order this document carried before
// this lane, and the reason is audit round 2's finding rather than taste: the
// release branch edits site/src/content.ts, so `git push origin main` fires
// deploy-site.yml (`on: push, branches: [main], paths: ['site/**']`) and publishes
// a page reading "the current release is vX.Y.Z" while no tag and no archive
// exists — release.yml fires only on `push: tags: ['v*']`. Landing the driver
// without changing this file would leave two descriptions of how this project
// releases that disagree about the exact ordering the audit was about, and
// CLAUDE.md is explicit that a second release procedure is a defect to report and
// not a fallback to use.
//
// The two existing pins over this file cannot see this: one reads the ancestry
// item, the other requires that the Go test names it mentions exist. Neither
// reads a push order.
func TestTheWrittenProcedureAndTheDriverAgreeAboutThePushOrder(t *testing.T) {
	root := surfaceRepoRoot(t)
	procedure := gateReadRepoFile(t, root, filepath.Join("docs", "RELEASING.md"))

	item := gateDriverTaggingItemRE.FindString(procedure)
	if item == "" {
		t.Fatal("docs/RELEASING.md no longer carries an item titled **Tag and push, in the driver's order.**. " +
			"That item is the written half of the ordering this driver performs, and without it this repository carries two descriptions of how it releases that disagree about which push happens first — which CLAUDE.md calls a defect to report, not a fallback to use")
	}

	tag := strings.Index(item, "git push origin vX.Y.Z")
	main := strings.Index(item, "git push origin main")
	switch {
	case tag < 0:
		t.Error("the item no longer tells a maintainer to push the TAG. The tag push is what fires the Release workflow, and it is the first irreversible act")
	case main < 0:
		t.Error("the item no longer tells a maintainer to push main")
	case tag > main:
		t.Errorf("the item pushes main before the tag, which is the order the driver refuses to perform.\n"+
			"Pushing main first fires deploy-site.yml — the release branch edits site/src/content.ts — and publishes a page announcing a release while no tag and no archive exists, because the Release workflow fires only on a tag push. "+
			"The item reads:\n%s", item)
	}

	verify := strings.Index(item, "Verify the archives")
	if verify < 0 {
		t.Errorf("the item does not tell a maintainer to verify the archives at all. It is the step BETWEEN the two pushes, and it is the reason the order is what it is: main is what announces the release, so it goes last, after the artifacts it announces have been checked.\nThe item reads:\n%s", item)
	} else if tag >= 0 && main >= 0 && !(tag < verify && verify < main) {
		t.Errorf("the item does not put the archive verification BETWEEN the tag push and the main push. That position is the whole ordering: tag, verify what the tag built, then announce.\nThe item reads:\n%s", item)
	}

	for _, want := range []struct{ fragment, why string }{
		{"make " + gateDriverMakeTarget, "the written procedure must name the same command that performs these steps, or the document and the driver are two procedures"},
		{gateDriverVersionEnv, "a maintainer has to be told which release they are naming"},
		{gateDriverAuthorizeEnv, "and that the authorization is the version typed a second time, for that release only"},
		{"deploy-site", "the ORDER needs its reason in the document, or the next person to tidy this item will restore the old one"},
	} {
		if !strings.Contains(item, want.fragment) {
			t.Errorf("the tag-and-push item no longer names %q. %s.\nThe item reads:\n%s", want.fragment, want.why, item)
		}
	}

	// And the old order must not survive anywhere else in the document.
	if strings.Contains(procedure, "git push origin main\n      git push origin vX.Y.Z") {
		t.Error("docs/RELEASING.md still carries the old command block that pushes main before the tag. Two orders in one document is the second release procedure CLAUDE.md forbids, and this one publishes a site announcing a release whose archives do not exist")
	}
}
