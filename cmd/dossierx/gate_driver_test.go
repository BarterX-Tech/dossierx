// gate_driver_test.go is THE DRIVER: the ordered, deterministic sequence that
// performs the one half of a release that cannot be taken back — a tag on a
// public forge, six archives somebody will `go install`, and a site page that
// tells the world a release exists.
//
// THE INVARIANT IT EXISTS FOR. No irreversible act happens except as the tail of
// a process that, in that same process, read the tree being published, found it
// green, and found that the tree declares the very release being published — and
// no such process starts except when a human asked for that specific release.
// Everything below is one of the six clauses that make up that sentence, and none
// of them is a consequence of the others.
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
//  5. THE RELEASE BEING TAGGED IS THE RELEASE THE TREE DECLARES. The version
//     arrives as a string a human typed into two environment variables, and
//     before this clause existed nothing ever compared it to anything: the
//     receipt stored it verbatim, the tag was created from it, and every other
//     check in this file is about CONTENT — which tree, which commit, which
//     fingerprint — and therefore silent about the NAME. So D1 derives the
//     version from the tree it is about to publish, out of the two documents
//     that tell a reader which release this is (CHANGELOG.md's newest heading
//     and site/src/content.ts's last releases[] entry), and refuses when the
//     typed version disagrees with either of them, or when those two disagree
//     with each other. The failure it forecloses is a tree that is perfectly
//     self-consistent about one release, tagged as another: every content check
//     passes, and the forge ends up carrying a tag whose archives, changelog and
//     site all describe a different release from the one the tag names.
//
//  6. CI RAN OVER THIS CONTENT, AND THE RECORD THAT SAYS SO IS REQUIRED. The
//     gate's own thirteen surfaces do not include "the test suites passed on a
//     machine that is not the maintainer's"; that is `make ci-evidence`, whose
//     stage writes a record naming the commit it adjudicated. D1 requires that
//     record to exist and to be about the tree being released, and reports its
//     absence as errGateUncheckable — a release published with no CI evidence at
//     all is the skip-that-reads-as-a-pass CLAUDE.md forbids, arrived at by
//     nobody running the stage rather than by the stage saying nothing.
//     WHAT THAT CHECK DOES NOT ESTABLISH, because the honest boundary is the
//     same one clause 1 draws: the record is PAPER. Nothing here authenticates
//     it, and a file typed by hand that names the right object satisfies it.
//     What it closes is "the stage was never run for this content", which is the
//     failure that actually happens; what stands behind the record's contents is
//     the stage that wrote it, which fails loudly rather than writing a clearing
//     record. The driver cannot close that gap without re-running CI itself, and
//     it says so rather than implying it is closed.
//
// AND ONE THING THE INVARIANT DOES NOT CLAIM, stated here rather than discovered
// later: it says nothing about whether the thirteen agents' verdicts are honest.
// Running in one process removes the forgeable RECEIPT; it does not remove the
// forgeable EVIDENCE. What stands behind the evidence is the freshness machinery
// in gate_stage2_test.go — the run manifest that records this tree, this
// resolved baseline and the digest of every artifact the keys cover — and that
// boundary is here, at gateDriverEvidence, not implied away.
//
// WHAT THIS DRIVER DOES AND DOES NOT DO TODAY, and both halves are deliberate.
// gateDriverWired (gate_evidence_test.go) is the evidence source the real
// repository gets, and every answer it owes is wired to this tree: the
// per-surface verdicts and findings are collected from the fan-out record this
// run produced and the answers given against it, the fingerprints green is
// recomputed against are the manifest's surfaces and stage 2's keys, and D7
// verifies the archives the forge is actually serving.
//
// So a run whose gate is green performs D0 through D8 and then ENDS AT D9, THE
// HANDOFF. D9 asks the evidence nothing. It is a terminal state that says two
// things in one breath — the release is published, and the three checks
// docs/RELEASING.md keeps a person's have been made by nothing — and it is a
// different claim from every other ending in this file, which are all failures.
// The ruling that made it one, and the earlier ruling that made reading the
// deployed site a person's work at all, are recorded on the D9 step itself.
// Nothing here has stopped being true: the driver still never reports a check it
// did not make, and the handoff's whole text is about the checks it did not
// make.
//
// AND IT STOPS FAR EARLIER THAN THAT UNTIL A FAN-OUT HAS ACTUALLY BEEN
// PRODUCED. Everything gateDriverWired reads under gate/ is per-run evidence
// with no committed form (gate/.gitignore), so between releases there is no
// gate/fanout.json for the tree being released and D1 refuses before the merge,
// naming the missing record. That is this repository's ordinary state, and it is
// the correct one: the alternative is a driver that reads "no fan-out was
// produced" as "nothing was found".
//
// gateDriverUnwired stays in this file beside it, and several tests below still
// hand it to the driver on purpose. It is how they construct a step whose
// machinery does not exist, which is the only way to prove that the sequence
// stops there rather than falling through.
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

	// The two documents that DECLARE which release a tree is. They are the
	// version's tree-side sources (clause 5) and there are exactly two because
	// these are the two files a human reads to learn what the current release is
	// — the changelog a consumer opens, and the page the site publishes. A third
	// source would be a third thing to keep in step; one source would make a
	// typo in it indistinguishable from a correct rename.
	gateDriverChangelogFile = "CHANGELOG.md"
	gateDriverSiteFile      = "site/src/content.ts"

	// gateDriverSiteReleasesDecl and gateDriverSiteLatestDecl bound the array in
	// that file, and gateDriverSiteLatestDecl is also the reason "the last entry"
	// is the newest one rather than an assumption this driver makes: the site
	// itself derives its current release that way, and if that line ever goes,
	// the ordering this parser depends on has gone with it and the version
	// becomes unreadable rather than quietly read backwards.
	gateDriverSiteReleasesDecl = "const releases: Release[] = ["
	gateDriverSiteLatestDecl   = "releases[releases.length - 1]"

	// gateDriverCIEvidenceEnv names where `make ci-evidence` wrote its verdict
	// record, and gateDriverCIEvidenceDefault is the path it defaults to.
	//
	// THE DEFAULT IS DUPLICATED FROM THE MAKEFILE ON PURPOSE, and
	// TestTheDriverLooksForTheCIEvidenceRecordWhereTheRecipeWritesIt is what
	// keeps the two equal. Make exports a recipe's environment from the
	// variables that came from the environment or the command line — measured:
	// `make show FOO=cmdline` prints FOO=[cmdline] and a `FOO ?= defaultval` in
	// the makefile prints FOO=[] — so `make ci-evidence
	// DOSSIERX_GATE_CI_EVIDENCE_OUT=/elsewhere` reaches this driver on the next
	// invocation and the `?=` default never does. If the two spellings of the
	// default drift apart, the driver looks for a record at a path nothing
	// writes and refuses every release with a message about a missing file.
	gateDriverCIEvidenceEnv     = "DOSSIERX_GATE_CI_EVIDENCE_OUT"
	gateDriverCIEvidenceDefault = "/tmp/dossierx-ci-run-evidence.json"

	// gateDriverCIEvidenceTarget is the target that writes that record. It is
	// named in every refusal about a missing one, because "the record is absent"
	// without the command that produces it is a refusal an operator has to go
	// and research.
	gateDriverCIEvidenceTarget = "ci-evidence"
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

	// CIEvidence is where `make ci-evidence` left its verdict record. It is a
	// PATH and never the record's contents, which is the same distinction the
	// receipt draws: what crosses this boundary is where to look, and the
	// looking is done here. It is a plan field rather than a read of the
	// environment deep inside the sequence so that the tests below can point one
	// run at a record and another at nothing without mutating the environment of
	// every other test in this binary.
	CIEvidence string
}

func gateDriverPlanFromEnv() gateDriverPlan {
	evidence := strings.TrimSpace(os.Getenv(gateDriverCIEvidenceEnv))
	if evidence == "" {
		evidence = gateDriverCIEvidenceDefault
	}
	return gateDriverPlan{
		Version:    strings.TrimSpace(os.Getenv(gateDriverVersionEnv)),
		Authorize:  strings.TrimSpace(os.Getenv(gateDriverAuthorizeEnv)),
		Record:     strings.TrimSpace(os.Getenv(gateDriverRecordEnv)),
		CIEvidence: evidence,
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
func gateDriverMode(p gateDriverPlan) (decision gateDriverDecision, why string) {
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
	{ID: "D1", What: "precondition — refuse an environment D2 cannot run in, refuse a partly-published release, derive the version from the tree and refuse a tag that disagrees with it, record the receipt IN THIS PROCESS, recompute the verdict against the tree about to be released, and require the CI-run evidence for that tree"},
	{ID: "D2", What: "merge the release branch into main with --no-ff, capturing the merge commit by value"},
	{ID: "D3", What: "handshake — the named merge commit's tree is the tree the receipt records"},
	{ID: "D4", What: "tag the named merge commit, never HEAD"},
	{ID: "D5", What: "re-read <tag>^{tree} and hand it to the handshake again, immediately before the push"},
	{ID: "D6", What: "push the tag", Irreversible: true,
		Published: "the tag %s is on the forge, and the Release workflow is building six archives from it. Deleting a published tag is not an undo: anything that fetched it has it"},
	{ID: "D7", What: "verify the published archives"},
	{ID: "D8", What: "push main", Irreversible: true,
		Published: "main carries the release commit, and deploy-site has published a site announcing %s. There is no automatic rewrite of a published page; the gate surfaces, it does not fix"},

	// D9 IS A HANDOFF AND NOT A CHECK, by a maintainer's ruling of 10 Aug 2026,
	// and this is the recorded form of that ruling rather than a shape somebody
	// tidied into.
	//
	// It used to be "G3 — the rendered site describes this release", asked of
	// gateDriverEvidence.Site, which answered errGateUncheckable by an EARLIER
	// ruling: reading the deployed site is one of the three checks
	// docs/RELEASING.md keeps a person's, because a workflow that never fired and
	// a deploy still serving yesterday's bundle both leave this repository
	// byte-identical to the release that went right, so there is nothing in this
	// tree for a check to read. That ruling stands. What it produced, though, was
	// that EVERY successful release ended with the driver reporting FAILURE at
	// D9, both pushes done — and a red that fires on every success is a red an
	// operator learns to scroll past, which costs the driver the only signal it
	// has for the releases that really did stop halfway.
	//
	// So the ending is a different claim, not a softer one. "A check could not
	// run" is what D7 says when the archives cannot be read, and it is a failure.
	// "Handed off" says the driver examined NOTHING about the deployed release
	// and that three named checks now begin — which is not a skip that reads as a
	// pass, because the report's subject is precisely what was not examined and
	// by whom it now must be. The words for it are gateDriverHumanChecks and
	// gateDriverHandoffNotChecked, one of which is pinned to the procedure and
	// the other of which the handoff must carry verbatim.
	{ID: "D9", What: "hand off — the release is published, and the three checks that stay a person's begin"},
}

// ---------------------------------------------------------------------
// the three checks D9 hands over
// ---------------------------------------------------------------------

// gateDriverHumanCheckPrefix opens each of those checks in docs/RELEASING.md.
// The "(human)" is the document's own marking and is what makes the section
// countable: TestTheHandoffNamesEveryCheckTheProcedureKeepsAPersons requires the
// number of items carrying this prefix to equal the number of checks declared
// below, so a fourth check added to the procedure and not to the driver is a
// handoff that quietly hands over three of four.
const gateDriverHumanCheckPrefix = "- [ ] **(human) "

// gateDriverHumanCheckSection is the heading those items live under, spelled as
// the section is NAMED rather than as markdown writes it: the handoff prints
// this to send a reader there, and the pin below is what knows it is an `###`.
const gateDriverHumanCheckSection = "Three checks that stay a person's"

// gateDriverHumanCheck is one check the driver hands to a person at D9.
//
// Item is the checklist item's title in docs/RELEASING.md, spelled exactly as
// that document spells it, and the handoff report prints Title() — the same
// string with the markdown taken off. ONE string with two readers, deliberately:
// a report that named these checks in its own words would be a second wording of
// the procedure, and the day the two drift the operator reads a name that is not
// in the document they are being sent to.
type gateDriverHumanCheck struct {
	Item string `json:"item"`

	// Do is what the person does, in the imperative. It is here because the
	// handoff is read at the one moment when going and finding the procedure is
	// least likely: the release is already public.
	Do string `json:"do"`

	// Silently is what a release nobody performed this check on looks like.
	// Every one of the three is the same shape — a failure that leaves the
	// forge, the site and this repository looking exactly like a release that
	// worked — and that is the whole reason none of them can be checked here.
	Silently string `json:"silently"`
}

// Title is Item without the checklist markup.
func (c gateDriverHumanCheck) Title() string {
	return strings.TrimSuffix(strings.TrimPrefix(c.Item, gateDriverHumanCheckPrefix), "**")
}

// gateDriverHumanChecks are docs/RELEASING.md's three, in the document's order.
//
// THEY ARE DECLARED HERE RATHER THAN READ OFF THE DOCUMENT AT RELEASE TIME, and
// that is a decision about WHEN they are needed. The handoff is composed after
// the last irreversible act, so any I/O it depends on is I/O that can fail with
// the tag and main already published — and the report that could not be produced
// is the report the operator has nothing to read instead of. Nothing is read
// from disk after D8. What keeps these equal to the document is a test at
// lane-landing time, where a disagreement costs a red run and not a release.
var gateDriverHumanChecks = []gateDriverHumanCheck{
	{
		Item: gateDriverHumanCheckPrefix + "The `Release` workflow itself passed.**",
		Do:   "open the run for this tag on the forge and read the run's own conclusion — not that the tag is there, and not that the release page loads",
		Silently: "a run that failed or stopped halfway leaves a published tag with no archives behind it, " +
			"and the tag is what every consumer resolves",
	},
	{
		Item: gateDriverHumanCheckPrefix + "`deploy-site` ran for this release.**",
		Do:   "look at deploy-site.yml's runs for this release's commit; if it never fired, the fix is a workflow_dispatch",
		Silently: "deploy-site triggers only on changes under site/**, so a release touching no site file publishes nothing, " +
			"fails nowhere, and leaves the live site naming the previous release",
	},
	{
		Item: gateDriverHumanCheckPrefix + "The deployed bundle is the one that was built.**",
		Do: "fetch the live index.html, read Vite's content-hashed asset names out of it and compare them against your own dist/ — " +
			"or at minimum satisfy yourself that they CHANGED from what the last deploy served",
		Silently: "a deploy that succeeded while a CDN kept serving yesterday's bundle looks like a deploy that worked from every angle but this one",
	},
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
// production implementation is gateDriverWired in gate_evidence_test.go, which
// answers all three from this repository's own gate run. gateDriverUnwired below
// answers all three with errGateUncheckable and is what the tests hand the
// driver when they need a step that cannot run.
//
// THERE ARE THREE QUESTIONS AND NOT FOUR. Site was the fourth until D9 became a
// handoff; it is gone rather than left answering errGateUncheckable, because an
// interface method nobody asks is a question the next reader will wire an answer
// to. The deployed site is read by a person — see the D9 step for the two
// rulings — and the driver's honesty about that is now a state it ENDS in rather
// than an answer it receives.
type gateDriverEvidence interface {
	// Verdicts are the per-surface answers and findings the gate run produced
	// for tree. They are the receipt's contents, never the receipt.
	Verdicts(tree string) ([]gateSurfaceVerdict, []gateFinding, error)
	// Current is the manifest's declared surfaces and the fingerprint each one
	// has in tree — the pair receipt.evaluate needs to recompute green.
	Current(tree string) (declared []string, current map[string]string, err error)
	// Archives is D7: the published artifacts for version, verified.
	Archives(version, commit string) error
}

// gateDriverUnwired is the evidence source that answers nothing.
//
// Every answer is errGateUncheckable. It was the real repository's evidence
// source until gateDriverWired landed, and it is kept — rather than deleted with
// the wiring — because it is the only way to construct a step whose machinery
// does not exist, which is what the tests below need in order to prove the
// sequence stops rather than falling through. Its texts still name what is
// missing, because a refusal that does not say what was needed sends an operator
// nowhere. A driver that assumed any of these would be narrowing coverage
// silently at the only moment it matters.
type gateDriverUnwired struct{}

func (gateDriverUnwired) Verdicts(string) ([]gateSurfaceVerdict, []gateFinding, error) {
	return nil, nil, fmt.Errorf("%w: nothing in this tree transports a gate run's per-surface verdicts into this process. "+
		"Every gateSurfaceVerdict in the repository is built inside a test fixture, and the stage-2 harness writes a run manifest and per-surface artifacts that no reader turns into verdicts. "+
		"The driver will not invent them: a receipt measured over verdicts nobody produced is a receipt about nothing, and it would evaluate PASS", errGateUncheckable)
}

func (gateDriverUnwired) Current(string) (declared []string, current map[string]string, err error) {
	return nil, nil, fmt.Errorf("%w: the driver has no wired path to this tree's per-surface fingerprints. "+
		"gateDeclaredSurfaces and gateStage2Plan compute them, and until the harness invocation that feeds them is wired to this driver, recomputing green here would be reading a map nobody filled", errGateUncheckable)
}

func (gateDriverUnwired) Archives(version, commit string) error {
	return fmt.Errorf("%w: verifying the published archives for %s at %s is not built in this tree. "+
		"Nothing here downloads a release artifact, checks a sha256 against checksums.txt, or reads the stamped version, commit and date out of an extracted binary. "+
		"So the driver stops HERE, after the tag is published and before main is, and says so — which is the failure-not-skip rule applied to the driver's own incompleteness. "+
		"A driver that pushed main without verifying the archives would announce a release whose artifacts nobody checked", errGateUncheckable, version, commit)
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

// gateDriverHandoffNotChecked is the sentence the terminal state exists to say,
// and it is a named constant because two things carry it — the report a human
// reads and the record they read afterwards — and because a test requires it
// verbatim in both. A sentence assembled twice is a sentence that ends up said
// two ways, and the two ways of saying this one are not equally true.
//
// It is phrased as what the driver DID: it read nothing. That is the whole
// difference between this state and a pass. CLAUDE.md's rule is that a check
// which did not happen must never read as one that did, and the ending a release
// now gets says exactly what was not examined and by whom it now must be.
const gateDriverHandoffNotChecked = "THIS DRIVER MADE NONE OF THEM. After it pushed main it read nothing at all — not the workflow run, " +
	"not the release page, not one byte of the live site — so the list below is not an account of how this release came back. " +
	"It is three pieces of work nobody has started, on a release that is already public."

// gateDriverHandoff is the state a run ENDS IN when it completed the sequence,
// and it is the one ending in this file that is not a failure.
//
// WHY IT IS A VALUE WITH A CONSTRUCTOR rather than a message printed at the
// bottom of gateDriverExecute: the property that matters about it is
// reachability. "Published, and here are the three checks nobody has made" is
// true only after main is pushed; printed anywhere else it is a driver telling
// an operator to go and read a site that no release put there. So the state can
// be built exactly one way — gateDriverRun.handoff, which refuses a run that has
// not completed D8 — and both readers of it, the report and the record, go
// through that constructor.
type gateDriverHandoff struct {
	Version string `json:"version"`

	// Published is the irreversible half, in the same words the failure report
	// uses for it, because it is the same fact and an operator should not have
	// to notice that two endings describe one forge differently.
	Published []string `json:"published"`

	// Checks are gateDriverHumanChecks, carried into the record so that the
	// account of a release names them even when nobody kept the terminal output.
	Checks []gateDriverHumanCheck `json:"checks_that_stay_a_persons"`

	// Examined is gateDriverHandoffNotChecked. The field is called what it is
	// called because its VALUE is the answer to "what did this driver examine
	// about the published release", and the answer is nothing.
	Examined string `json:"what_this_driver_examined"`
}

// handoff is the only constructor of a terminal state, and its two refusals are
// the safety property rather than defensive habit.
//
//   - NOT PAST D8, so nothing that stopped earlier can wear this ending. A run
//     that failed at D7 has published a tag and no main; handing it off would
//     tell an operator that a release is out and that their remaining work is
//     three checks, when their actual position is a half-published release the
//     report for a stopped run is written to describe.
//   - AND NOTHING FAILED. D8 is the last step that can fail today, so this
//     clause refuses nothing the first one does not — until a step is added
//     after D8, at which point it is the difference between "the sequence
//     finished" and "the sequence got past the last push". Both clauses are
//     exercised below; neither is here on the strength of an argument.
func (r *gateDriverRun) handoff() (*gateDriverHandoff, error) {
	if !r.completed("D8") {
		return nil, fmt.Errorf("this run has not completed D8, so there is no release to hand off: it completed %v. "+
			"The handoff says a release is published and that three named checks are now a person's; said over a run that did not push main, it sends an operator to read a site nothing deployed and to open a workflow run that does not exist", r.Done)
	}
	if r.Failed != "" {
		return nil, fmt.Errorf("this run completed D8 and then stopped at %s (%s), so it ended as a failure and a failure is what it reports. "+
			"A handoff over a run that stopped would be this driver reporting the release as finished at the one moment the operator most needs to be told it is not", r.Failed, gateDriverWhat(r.Failed))
	}
	published, _ := r.irreversible()
	return &gateDriverHandoff{
		Version:   r.Plan.Version,
		Published: published,
		Checks:    gateDriverHumanChecks,
		Examined:  gateDriverHandoffNotChecked,
	}, nil
}

// report is the terminal state as a human reads it.
func (h *gateDriverHandoff) report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s IS PUBLISHED. The driver's part of this release is over; the release itself is not.\n\n", h.Version)

	b.WriteString("ALREADY PUBLISHED — these have happened and cannot be taken back:\n")
	b.WriteString(strings.Join(h.Published, "\n") + "\n\n")

	b.WriteString("HANDED OFF — three things begin here, and they are a person's:\n")
	b.WriteString(h.Examined + "\n\n")
	for i, check := range h.Checks {
		fmt.Fprintf(&b, "  %d. %s\n     WHAT A PERSON DOES: %s.\n     WHAT NOBODY DOING IT LEAVES: %s.\n", i+1, check.Title(), check.Do, check.Silently)
	}

	fmt.Fprintf(&b, "\nThese three are the section %q in %s. Each asks whether a system outside this repository did what it was told, and a workflow that never fired, "+
		"a deploy still serving yesterday's bundle and a run that concluded having produced nothing all leave this repository byte-identical to the release that went right — "+
		"so there is nothing in the tree for any of them to be read from. Skipping one is not \"the machine has it\"; it is nobody having looked.\n",
		gateDriverHumanCheckSection, gateDriverProcedureFile)
	return b.String()
}

// irreversible splits the acts that leave this machine into the ones this run
// performed and the ones it did not. Both endings print it, from here, because
// "what is already on the forge" is one fact and a release that described it two
// ways would be asking an operator to work out which description they are in.
func (r *gateDriverRun) irreversible() (published, remaining []string) {
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
	return published, remaining
}

// report is what a human reads when a run ENDS, and a run ends one of two ways.
//
// Handed off, which is the state above and says the release is published and
// unread; or stopped, which names the failed step, then what is already public,
// then what is not, and then the one thing this driver will not do about it. The
// choice between them is made by the handoff's own constructor and not by a
// condition spelled a second time here — the question "may this run be handed
// off?" has one answer in this file.
func (r *gateDriverRun) report() string {
	handoff, handoffErr := r.handoff()
	switch {
	case handoffErr == nil:
		return handoff.report()
	case r.Failed == "":
		// Not reachable from gateDriverExecute, which stops at a named step or
		// reaches D9. It is reachable from a run assembled by hand, and what it
		// must not do then is fall through to a failure report naming an empty
		// step, which reads as a release that stopped at nothing.
		return fmt.Sprintf("the release of %s is in no state this driver can report: it recorded no failed step, and it cannot be handed off either.\n%v\n\n"+
			"Treat that as a release whose position on the forge is unknown and go and look, rather than as either ending.\n", r.Plan.Version, handoffErr)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "the release of %s stopped at %s (%s):\n%v\n\n", r.Plan.Version, r.Failed, gateDriverWhat(r.Failed), r.Err)

	published, remaining := r.irreversible()

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

	// D9. The handoff, asked of nothing outside this process — the whole of it is
	// what this driver did not do. It is constructed here rather than only at
	// print time so that the sequence is fail-closed in this direction too: a run
	// whose terminal state cannot be built has not finished, and it says so at a
	// named step instead of returning a completed run nobody can describe.
	if _, err := r.handoff(); err != nil {
		return r.stop("D9", err)
	}
	r.step("D9")
	return r
}

// precondition is D1, and it is eight questions rather than one: two about the
// ENVIRONMENT this driver was invoked in, then six about the release.
//
// THE TWO ENVIRONMENT QUESTIONS ARE ASKED FIRST BECAUSE THEY ARE FREE. Neither
// needs the network, the evidence or a receipt — one is a string comparison, the
// other is a single git command — and neither can be changed by anything read
// below it. A refusal that was always going to happen belongs before the
// expensive half of the precondition, not after it, and one of these two belongs
// before D2 for a stronger reason than cost: D2 is a WRITE, and it is the last
// step whose failure is still free.
func (r *gateDriverRun) precondition(ev gateDriverEvidence) error {
	// (i) The base ref this driver re-asserts against is gateBaseRef, which is
	// origin/main and deliberately not "main". A repository whose remote and base
	// branch spell something else would have the ancestry question asked about a
	// ref nobody named, so it is refused rather than answered.
	if base := r.Repo.Remote + "/" + r.Repo.Base; base != gateBaseRef {
		return fmt.Errorf("%w: this driver re-asserts the ancestry precondition against %s, and it was pointed at %s. "+
			"Answering from a differently-named base would make every check below about a ref the receipt does not record", errGateUncheckable, gateBaseRef, base)
	}

	// (ii) And whether the base branch can be checked out here at all, which is
	// the one thing D2 needs from this machine that no amount of correct content
	// supplies.
	if err := r.requireTheBaseIsNotCheckedOutElsewhere(); err != nil {
		return err
	}

	// (a) Re-entry. A release that already published something is not resumed and
	// not undone; it is refused, with both lists printed.
	if err := r.refuseIfAlreadyPublished(); err != nil {
		return err
	}

	// (b) The NAME. Every other question in this function is about content, so
	// this is the only one that can catch a self-consistent tree tagged as some
	// other release. It is asked before the merge, before the receipt and before
	// a single write, because it needs nothing but the branch.
	if err := r.requireTheTreeDeclaresThisRelease(); err != nil {
		return err
	}

	// (c) The tree the evidence is about, read before the evidence is asked for.
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

	// (d) The receipt, MEASURED here. gateRecordReceipt re-asserts the ancestry
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

	// (e) Green, RECOMPUTED. Not read, not inferred from an empty findings list —
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

	// (f) And the half of the evidence this gate does not gather: the CI run over
	// this content. It is asked last because it is the only question here whose
	// answer lives in a file rather than in git, and it is asked at all because
	// nothing else in the sequence would notice that `make ci-evidence` was never
	// run — which is a check that did not happen reading as a check that passed.
	return r.requireCIRunEvidence()
}

// ---------------------------------------------------------------------
// the environment D2 needs, asked for before anything is read
// ---------------------------------------------------------------------

// The two lines of `git worktree list --porcelain` this driver reads. Each record
// opens with the worktree's path and states what it holds; a worktree with no
// branch says `detached` or `bare` and carries no `branch` line at all — which is
// precisely the state the recovery below puts one into.
const (
	gateDriverWorktreePathLine   = "worktree "
	gateDriverWorktreeBranchLine = "branch refs/heads/"
)

// gateDriverWorktreesHolding is every worktree of dir's repository that has
// branch checked out.
//
// The listing is PARSED — record by record, on the line that declares the branch
// — rather than searched for the branch's name. A worktree living under a path
// that contains `refs/heads/main`, or simply the word `main`, would otherwise
// answer this question by accident, and the branch this is asked about is `main`
// in every release this repository will ever cut.
func gateDriverWorktreesHolding(dir, branch string) ([]string, error) {
	listing, err := gateGit(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var (
		holders []string
		path    string
	)
	for _, line := range strings.Split(listing, "\n") {
		switch {
		case strings.HasPrefix(line, gateDriverWorktreePathLine):
			path = strings.TrimPrefix(line, gateDriverWorktreePathLine)
		case line == gateDriverWorktreeBranchLine+branch:
			holders = append(holders, path)
		}
	}
	return holders, nil
}

// requireTheBaseIsNotCheckedOutElsewhere refuses an invocation whose D2 git would
// refuse, and refuses it in this driver's voice instead of git's.
//
// THE FAILURE IT MOVES. D2 opens with `git checkout main` in the invoking
// checkout. Git allows a branch to be checked out in exactly one worktree at a
// time, so in a linked-worktree layout — a release cut on a branch in a worktree
// while `main` sits in the primary checkout, which is the layout this repository's
// own releases are prepared in — git refuses that checkout with `fatal: 'main' is
// already used by worktree at '<path>'`.
//
// WHAT IS AND IS NOT WRONG WITH THAT. Nothing is published: D2 is before the first
// irreversible act, so the release stops with the forge untouched, and the
// fail-closed sequence does exactly what it is built to do. Two things are wrong
// anyway. It is LATE — it arrives after a whole green gate run, whose cost is the
// expensive part of a release — and it is not this driver SPEAKING: git's sentence
// names a worktree and a branch, says nothing about the release, and offers no
// recovery, so the operator meets an unexplained fatal at the one moment the
// pipeline was supposed to be telling them what to do next.
//
// So the question is asked here, where it costs two git commands and no network,
// and the refusal carries the recovery. It is not a new precondition on the
// release; it is the same precondition D2 already had, moved to where it can be
// answered usefully.
func (r *gateDriverRun) requireTheBaseIsNotCheckedOutElsewhere() error {
	// The one state in which no other worktree can be in the way: this checkout
	// already holds the base branch, so D2's checkout is a no-op that git
	// performs happily. It is asked FIRST because the listing below names this
	// worktree too, and reading its own entry as an obstacle would refuse a
	// release for the crime of already standing where D2 wants to be.
	head, err := gateGit(r.Repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("%w: the branch checked out in %s could not be read, so whether D2 can check %s out there is unknown: %w",
			errGateUncheckable, r.Repo.Dir, r.Repo.Base, err)
	}
	if head == r.Repo.Base {
		return nil
	}

	holders, err := gateDriverWorktreesHolding(r.Repo.Dir, r.Repo.Base)
	if err != nil {
		return fmt.Errorf("%w: this repository's worktrees could not be listed, so it is not known whether %s is checked out somewhere else. "+
			"That is a failed check and not an absent one — the answer decides whether D2 can run at all, and a driver that assumed it could would be assuming its way past the last step whose failure is still free: %w",
			errGateUncheckable, r.Repo.Base, err)
	}
	if len(holders) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s is checked out in another worktree of this repository, so D2 cannot check it out here.\n"+
		"  this driver was invoked in %s, which is on %s\n"+
		"  %s is checked out in %s\n"+
		"Git allows a branch to be checked out in one worktree at a time, so D2's `git checkout %s` would be refused by git itself — with a sentence about worktrees that names no release and no way forward. Refusing there publishes nothing, because D2 is before the first irreversible act; it just costs the whole gate run that came before it and tells the operator nothing.\n"+
		"THE RECOVERY is one command, and one to undo it afterwards:\n"+
		"\tgit -C %s switch --detach\n"+
		"Re-run this target, and switch that worktree back to %s once the release is published. Nothing about the release itself changes — the tree, the receipt and the version are all still exactly what they were",
		errGateUncheckable, r.Repo.Base,
		r.Repo.Dir, head,
		r.Repo.Base, strings.Join(holders, ", "),
		r.Repo.Base,
		holders[0],
		r.Repo.Base)
}

// ---------------------------------------------------------------------
// the version, derived from the tree rather than accepted from the caller
// ---------------------------------------------------------------------

// errGateVersionMismatch is the ANSWER "no": the tree was read, it says which
// release it is, and it is not the release being published. It is a separate
// sentinel from errGateUncheckable for the reason errGateTreeMismatch is — the
// two are different accusations with different recoveries. A mismatch accuses the
// INVOCATION (or the tree): somebody typed the wrong version, or the release
// branch was never updated for this one, and the recovery is to correct one of
// them. Uncheckable accuses the READING: the document is missing or its shape
// moved, and the recovery is to make the tree declare its release again.
var errGateVersionMismatch = errors.New("the version being published is not the version this tree declares")

// gateDriverChangelogHeadingRE is Keep a Changelog's release heading. The
// document's newest release is its FIRST such heading — the format is
// newest-first, which is the opposite of the site's array, and getting the two
// backwards is the failure this pair of regexps has to not have.
var gateDriverChangelogHeadingRE = regexp.MustCompile(`(?m)^## \[v?(\d+\.\d+\.\d+)\]`)

// gateDriverSiteVersionRE is one entry's version field in the site's releases
// array.
var gateDriverSiteVersionRE = regexp.MustCompile(`(?m)^\s*version:\s*"v?(\d+\.\d+\.\d+)"`)

// gateDriverNormalizeVersion strips the leading v, because the two documents
// spell the same release differently on purpose — the changelog heading is
// `## [0.5.0]` and the site entry is `version: "v0.5.0"` — and a comparison that
// treated that as a disagreement would refuse every correct release.
func gateDriverNormalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// gateDriverTreeFile reads one file OUT OF THE TREE being released, never off
// the working copy. `git show <branch>:<path>` is the whole reason: the working
// copy can carry an edit that is not on the branch, and an uncommitted CHANGELOG
// heading is exactly the shape of a maintainer who bumped the version and has not
// committed it — which would clear this check and then be absent from the tag.
func gateDriverTreeFile(dir, branch, path string) (string, error) {
	return gateGit(dir, "show", branch+":"+path)
}

// gateDriverChangelogVersion is the release CHANGELOG.md's newest heading names.
func gateDriverChangelogVersion(dir, branch string) (string, error) {
	body, err := gateDriverTreeFile(dir, branch, gateDriverChangelogFile)
	if err != nil {
		return "", fmt.Errorf("%w: %s carries no readable %s, so the tree cannot say which release it is: %w",
			errGateUncheckable, branch, gateDriverChangelogFile, err)
	}
	m := gateDriverChangelogHeadingRE.FindStringSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("%w: %s in %s declares no `## [X.Y.Z]` release heading. "+
			"That heading is one of the two places this tree states which release it is, and a changelog that names no release cannot be compared with the version being tagged",
			errGateUncheckable, gateDriverChangelogFile, branch)
	}
	return m[1], nil
}

// gateDriverSiteVersion is the release the site's newest releases[] entry names.
//
// It is scoped to the array and it REQUIRES the line the site derives its own
// current release from. Without that line, "the newest entry" is this driver's
// guess about somebody else's data structure, and a guess that is wrong reads a
// release out of the wrong end of the array and refuses (or clears) the wrong
// thing silently. With it, a site that started prepending its entries makes this
// unreadable instead, which is the failure-not-guess direction.
func gateDriverSiteVersion(dir, branch string) (string, error) {
	body, err := gateDriverTreeFile(dir, branch, gateDriverSiteFile)
	if err != nil {
		return "", fmt.Errorf("%w: %s carries no readable %s, so the site half of the release's own name cannot be read: %w",
			errGateUncheckable, branch, gateDriverSiteFile, err)
	}
	start := strings.Index(body, gateDriverSiteReleasesDecl)
	if start < 0 {
		return "", fmt.Errorf("%w: %s in %s no longer declares `%s`, so there is no array to read the current release out of",
			errGateUncheckable, gateDriverSiteFile, branch, gateDriverSiteReleasesDecl)
	}
	end := strings.Index(body[start:], gateDriverSiteLatestDecl)
	if end < 0 {
		return "", fmt.Errorf("%w: %s in %s no longer derives its current release as `%s`. "+
			"This driver reads the LAST entry of that array because the site itself does; without that line, 'the newest release' is a guess about the order somebody else's data structure is written in, and a wrong guess reads the wrong release and says nothing",
			errGateUncheckable, gateDriverSiteFile, branch, gateDriverSiteLatestDecl)
	}
	matches := gateDriverSiteVersionRE.FindAllStringSubmatch(body[start:start+end], -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("%w: %s in %s declares a releases array holding no `version: \"vX.Y.Z\"` entry, so the site names no current release",
			errGateUncheckable, gateDriverSiteFile, branch)
	}
	return matches[len(matches)-1][1], nil
}

// requireTheTreeDeclaresThisRelease is clause 5.
//
// Three values, and the refusal names all three whichever pair disagrees,
// because the reader's next question is always "which one is wrong?" and a
// message naming two of them makes them go and look up the third.
func (r *gateDriverRun) requireTheTreeDeclaresThisRelease() error {
	typed := gateDriverNormalizeVersion(r.Plan.Version)
	if typed == "" {
		return fmt.Errorf("%w: the release was named as %q, which carries no X.Y.Z version to compare with what the tree declares", errGateUncheckable, r.Plan.Version)
	}

	changelog, err := gateDriverChangelogVersion(r.Repo.Dir, r.Repo.Branch)
	if err != nil {
		return err
	}
	site, err := gateDriverSiteVersion(r.Repo.Dir, r.Repo.Branch)
	if err != nil {
		return err
	}
	changelog, site = gateDriverNormalizeVersion(changelog), gateDriverNormalizeVersion(site)

	if changelog != site {
		return fmt.Errorf("%w: this tree does not agree with itself about which release it is.\n"+
			"  typed on the command line: %s\n"+
			"  %s newest heading declares: %s\n"+
			"  %s last releases[] entry declares: %s\n"+
			"One of the two documents was bumped and the other was not, so whichever version is tagged, one of the two things a reader consults about this release is wrong from the moment it is published",
			errGateVersionMismatch, r.Plan.Version, gateDriverChangelogFile, changelog, gateDriverSiteFile, site)
	}
	if typed != changelog {
		// The recovery spells the tree's version the way the human spelled
		// theirs — prefix is whatever came before the digits, so a repository
		// that tags `v0.6.0` is told `v0.6.0` and one that tags `0.6.0` is told
		// `0.6.0`, rather than being handed this file's assumption about tag
		// names as a command to paste.
		prefix := strings.TrimSuffix(r.Plan.Version, typed)
		return fmt.Errorf("%w: the tree is self-consistent about %s%s and this run was told to publish %s.\n"+
			"  typed on the command line: %s\n"+
			"  %s newest heading declares: %s\n"+
			"  %s last releases[] entry declares: %s\n"+
			"Every other check this driver makes is about CONTENT — which tree, which commit, which fingerprint — and all of them pass here, because the tree is not wrong, the NAME is. Publishing would put a tag on the forge whose archives, changelog and site all describe %s%s. "+
			"Either correct %s=%s%s and %s=%s%s, or bump the tree to %s and re-run the gate",
			errGateVersionMismatch, prefix, changelog, r.Plan.Version,
			r.Plan.Version, gateDriverChangelogFile, changelog, gateDriverSiteFile, site,
			prefix, changelog,
			gateDriverVersionEnv, prefix, changelog, gateDriverAuthorizeEnv, prefix, changelog, r.Plan.Version)
	}
	return nil
}

// ---------------------------------------------------------------------
// the CI-run evidence, required rather than assumed
// ---------------------------------------------------------------------

// gateDriverObjectRE is a full object name as it appears anywhere in the record.
//
// The record is read as TEXT and matched, never decoded — the same rule the AST
// pin below enforces for the receipt, and here it is also the honest shape of the
// claim being made. Decoding would let this file talk about the record's fields
// as though it trusted them; matching an object name and then asking GIT what
// that object holds means the only thing taken from the file is a pointer, and
// every conclusion drawn from it is measured here.
var gateDriverObjectRE = regexp.MustCompile(`\b[0-9a-f]{40}\b`)

// requireCIRunEvidence is clause 6.
//
// It requires a record that names an object whose TREE is the tree being
// released. Naming the object is not enough and naming the version would be
// worse: `make ci-evidence` is keyed to a commit, the merge that lands a
// converging release branch carries the same tree as the branch head, and it is
// the CONTENT that CI ran over — which is the same reason the receipt compares
// trees rather than commits.
func (r *gateDriverRun) requireCIRunEvidence() error {
	recovery := fmt.Sprintf("Run `make %s DOSSIERX_GATE_CI_SHA=<the commit CI ran over>` and re-run this target. "+
		"If that record lives somewhere other than %s, hand the same %s=<path> to this target too — make exports a command-line variable into a recipe's environment, and its `?=` default is not exported at all",
		gateDriverCIEvidenceTarget, gateDriverCIEvidenceDefault, gateDriverCIEvidenceEnv)

	path := r.Plan.CIEvidence
	if path == "" {
		return fmt.Errorf("%w: no path to a CI-run evidence record was given, so this run cannot tell a release whose suites were adjudicated from one where nobody looked. %s",
			errGateUncheckable, recovery)
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w: there is no readable CI-run evidence record at %s (%v), so nothing in this release says the test suites were accounted for on any machine but this one. "+
			"An absent record is a FAILED check and never a skipped one: `go test` exits 0 for a suite that ran nothing, which is the exact failure that stage exists to catch, and not running the stage at all is that failure with no one watching. %s",
			errGateUncheckable, path, err, recovery)
	}
	if strings.TrimSpace(string(blob)) == "" {
		return fmt.Errorf("%w: the CI-run evidence record at %s is empty. An empty file is what a stage that died mid-write leaves, and it says nothing about any commit. %s",
			errGateUncheckable, path, recovery)
	}

	seen := map[string]bool{}
	var named []string
	for _, object := range gateDriverObjectRE.FindAllString(string(blob), -1) {
		if seen[object] {
			continue
		}
		seen[object] = true
		named = append(named, object)
		if tree, err := gateTreeSHA(r.Repo.Dir, object+"^{commit}"); err == nil && tree == r.Receipt.Tree {
			return nil
		}
	}

	return fmt.Errorf("%w: the CI-run evidence record at %s names no object carrying the tree being released.\n"+
		"  the tree about to be tagged: %s\n"+
		"  objects the record names:    %v\n"+
		"So the record is about some other adjudication — an earlier release, an earlier push, or a commit this clone has never fetched — and it says nothing about the content %s would publish. %s",
		errGateUncheckable, path, r.Receipt.Tree, named, r.Plan.Version, recovery)
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
//
// Handoff is present exactly when the run ended handed off, and it is here
// rather than left to the prose because the record outlives the terminal it was
// printed to. A release is audited weeks later from this file, and "which three
// checks were outstanding when the driver let go, and had anything examined
// them" is the question that gets asked; a record that carried only "completed
// D0…D9" answers it with a list of step names that read like nine things going
// right.
type gateDriverRecord struct {
	Version     string             `json:"version"`
	Branch      string             `json:"branch"`
	WrittenBy   string             `json:"written_by"`
	Completed   []string           `json:"completed"`
	FailedAt    string             `json:"failed_at,omitempty"`
	Verdict     string             `json:"verdict,omitempty"`
	MergeCommit string             `json:"merge_commit,omitempty"`
	Receipt     gateReceipt        `json:"receipt"`
	Handoff     *gateDriverHandoff `json:"handoff,omitempty"`
	Report      string             `json:"report,omitempty"`
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
	// The report is written for BOTH endings now. It used to be conditional on
	// there being an error, which was right while every ending that was not a
	// failure was a run that had published nothing — and wrong the moment a
	// completed release started ending in a state somebody has to act on.
	record.Report = r.report()
	if handoff, err := r.handoff(); err == nil {
		record.Handoff = handoff
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
	// The real evidence source, pointed at the checkout being released. It is the
	// same root the repo descriptor names, and that is not a coincidence to be
	// tidied away: the verdicts, the fingerprints and the tree D1 handshakes over
	// all have to be about one checkout, and two roots in this call would be two
	// releases being reasoned about in one process.
	run := gateDriverPublish(plan, repo, gateDriverWired{Root: root})

	if plan.Record != "" {
		gateDriverWriteRecord(t, plan.Record, run)
	}

	// A completed run PRINTS THE HANDOFF, because that text is the deliverable of
	// the last step. `%s published: [D0 D1 …]` was the old line, and a list of
	// nine step names is the shape of a release reporting nine things that went
	// right — with no mention that nothing has looked at the published release at
	// all.
	if run.Err == nil {
		t.Log(run.report())
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
}

func (e gateDriverFixtureEvidence) Verdicts(string) ([]gateSurfaceVerdict, []gateFinding, error) {
	return e.verdicts, e.findings, nil
}

func (e gateDriverFixtureEvidence) Current(string) (declared []string, current map[string]string, err error) {
	return e.declared, e.current, nil
}

func (e gateDriverFixtureEvidence) Archives(string, string) error { return e.archives }

// gateDriverGreenEvidence is a clean run over one surface: a PASS whose
// fingerprint is the one this tree produces, no findings, and the one post-tag
// check satisfied.
func gateDriverGreenEvidence() gateDriverFixtureEvidence {
	return gateDriverFixtureEvidence{
		verdicts: gatePassingSurfaces("readme"),
		declared: []string{"readme"},
		current:  map[string]string{"readme": "sha256:readme"},
	}
}

// gateDriverFixture builds the converging topology with a real `origin`, makes
// the branch DECLARE the release it is for, and returns the repo descriptor the
// driver acts on.
//
// declares is a parameter rather than a constant because the tree's own
// statement of which release it is became a precondition (clause 5), so a fixture
// that could not be pointed at a different release could not construct the
// failure that precondition exists for.
func gateDriverFixture(t *testing.T, declares string) gateDriverRepo {
	t.Helper()
	work := gateFixtureRepo(t)
	repo := gateDriverRepo{
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
	gateDriverDeclareRelease(t, repo, declares, declares)
	return repo
}

// gateDriverDeclareRelease writes the two documents that state which release a
// tree is, and commits them on the release branch.
//
// The two versions are separate parameters so a tree can be made to disagree with
// ITSELF, which is one of the three shapes clause 5 refuses and the only one that
// cannot be constructed by changing what the human typed.
//
// The site fixture carries an OLDER entry ahead of the release one on purpose: it
// is what makes "the last entry" load-bearing. A parser that took the first match
// would read v0.0.1 here and every assertion below would be about the wrong end of
// the array.
func gateDriverDeclareRelease(t *testing.T, repo gateDriverRepo, changelog, site string) {
	t.Helper()
	gateWrite(t, repo.Dir, gateDriverChangelogFile, fmt.Sprintf(
		"# Changelog\n\n## [%s] - 2026-08-09\n\nThe release under test.\n\n## [0.0.1] - 2026-07-21\n\nThe one before it.\n",
		gateDriverNormalizeVersion(changelog)))
	gateWrite(t, repo.Dir, gateDriverSiteFile, fmt.Sprintf(
		"type Release = { version: string };\n\n%s\n"+
			"  {\n    version: \"v0.0.1\",\n  },\n"+
			"  {\n    version: %q,\n  },\n"+
			"];\n\nexport const latestRelease: Release = %s;\n",
		gateDriverSiteReleasesDecl, site, gateDriverSiteLatestDecl))
	gateTestGit(t, repo.Dir, "add", "-A")
	gateTestGit(t, repo.Dir, "commit", "-qm", "docs: declare the release this branch is for")
}

// gateDriverCIEvidenceRecord writes a fixture stand-in for what `make
// ci-evidence` leaves behind, naming one object, and returns its path.
//
// It is a hand-written file, which is the point rather than a shortcut: the
// driver's check is a presence-and-subject check over paper it cannot
// authenticate (clause 6), and a fixture that pretended otherwise would be
// claiming a guarantee this driver does not make.
func gateDriverCIEvidenceRecord(t *testing.T, object string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ci-run-evidence.json")
	body := fmt.Sprintf("{\n  \"sha\": %q,\n  \"verdict\": \"PASS\",\n  \"written_by\": \"a fixture, not the stage\"\n}\n", object)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the fixture CI-run evidence record: %v", err)
	}
	return path
}

// gateDriverAuthorized is a plan a human authorized for one version, with the
// CI-run evidence for the branch head in place — the ordinary state of a release
// that is ready to go.
func gateDriverAuthorized(t *testing.T, repo gateDriverRepo, version string) gateDriverPlan {
	t.Helper()
	return gateDriverPlan{
		Version:    version,
		Authorize:  version,
		Record:     filepath.Join(t.TempDir(), "record.json"),
		CIEvidence: gateDriverCIEvidenceRecord(t, gateTestGit(t, repo.Dir, "rev-parse", repo.Branch)),
	}
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
	const version = "v9.9.9"
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
			repo := gateDriverFixture(t, version)
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
	repo := gateDriverFixture(t, version)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)

	plan := gateDriverAuthorized(t, repo, version)
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

// TestAGreenRunEndsHandedOverAndNotRead is the ending itself: what a release
// that went entirely right leaves behind, in the terminal and in the record.
//
// THE THING BEING ASSERTED IS A DISTINCTION, not a success. A completed run is
// allowed to say the release is published — it pushed it — and is not allowed to
// say anything at all about whether that release is any good, because after D8
// it read nothing. Those two are one sentence apart in the writing and a long
// way apart in what an operator does next, and the second one is the release
// that ships broken while its report reads fine.
//
// So this asks for all four halves of the ending in one place: the state exists,
// it names what is already public, it names the three checks by the procedure's
// own titles, and it carries the sentence saying nothing made them — and then
// the denylist below asks the opposite question, which is whether anything in
// the driver's own prose claims otherwise.
func TestAGreenRunEndsHandedOverAndNotRead(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t, version)
	plan := gateDriverAuthorized(t, repo, version)

	run := gateDriverPublish(plan, repo, gateDriverGreenEvidence())
	if run.Err != nil {
		t.Fatalf("a clean, authorized, converging release was refused:\n%s", run.report())
	}
	if run.Failed != "" {
		t.Fatalf("a completed release recorded a failed step (%s). Every successful release used to end this way — the site read was a check nobody could make, so the run reported FAILURE with both pushes done — and a red that fires on every success is one an operator stops reading", run.Failed)
	}
	if last := run.Done[len(run.Done)-1]; last != "D9" {
		t.Fatalf("the run ended after %s rather than at D9. D9 is the handoff, and a run that stops before it has published a release and said nothing about who now looks at it", last)
	}

	handoff, err := run.handoff()
	if err != nil {
		t.Fatalf("a run that completed the whole sequence cannot state its own ending: %v", err)
	}
	if handoff.Version != version {
		t.Errorf("the handoff names %q and the release is %s; an operator reads it beside a forge carrying several tags", handoff.Version, version)
	}
	if len(handoff.Checks) != len(gateDriverHumanChecks) {
		t.Fatalf("the handoff carries %d check(s) and this repository keeps %d a person's", len(handoff.Checks), len(gateDriverHumanChecks))
	}

	report := run.report()
	for _, want := range []struct{ fragment, why string }{
		{version + " IS PUBLISHED", "the first thing the reader needs is which release this is about and that it is out"},
		{"ALREADY PUBLISHED", "the irreversible half is named in the same words a stopped run names it in; it is the same forge either way"},
		{"D6 push the tag", "the tag is public"},
		{"D8 push main", "and so is main, which is what fired deploy-site"},
		{gateDriverHandoffNotChecked, "the sentence the whole state exists to say, verbatim — that this driver examined none of what follows"},
		{gateDriverHumanCheckSection, "the reader is being sent to a section of the procedure, so the section is named as it is spelled there"},
		{gateDriverProcedureFile, "and the file it is in, because this is read at the moment when going and finding it is least likely"},
	} {
		if !strings.Contains(report, want.fragment) {
			t.Errorf("the handoff does not carry %q. %s.\nThe report reads:\n%s", want.fragment, want.why, report)
		}
	}
	for _, check := range gateDriverHumanChecks {
		for _, want := range []string{check.Title(), check.Do, check.Silently} {
			if !strings.Contains(report, want) {
				t.Errorf("the handoff does not name %q.\nThe three checks are the entire content of this state: one dropped is a release handed over as though two checks were all of it, and nothing downstream says otherwise.\nThe report reads:\n%s", want, report)
			}
		}
	}

	// THE DRIVER'S OWN PROSE, which is the report with the procedure's three
	// titles taken out of it. One of those titles is "The `Release` workflow
	// itself passed." — the document's words for a check a person makes — and
	// quoting it is not the driver claiming it. Everything left after they are
	// removed is this file talking about what it did.
	own := report
	for _, check := range gateDriverHumanChecks {
		own = strings.ReplaceAll(own, check.Title(), "")
	}
	for _, forbidden := range gateDriverHandoffMustNotSay {
		if strings.Contains(own, forbidden.word) {
			t.Errorf("the handoff says %q about the release it published. %s.\n"+
				"Handed off is the claim and checked is never the claim: this driver read nothing after the push, so any word in its own prose asserting an examination is a check that did not happen reading as one that did — which is the one thing CLAUDE.md forbids by name.\nThe report reads:\n%s",
				forbidden.word, forbidden.why, report)
		}
	}

	// AND THE RECORD, because the terminal output belongs to whoever was at the
	// keyboard and the record is what the release is audited from later. It is
	// read as TEXT — this file decodes nothing, by clause 1 — which is enough:
	// the question is whether the words reached the file.
	gateDriverWriteRecord(t, plan.Record, run)
	blob, err := os.ReadFile(plan.Record)
	if err != nil {
		t.Fatalf("read the run record the release wrote: %v", err)
	}
	record := string(blob)
	for _, want := range []struct{ fragment, why string }{
		{`"handoff"`, "the ending is a field of the record and not only a paragraph inside its report, so a reader can ask the file what state the release ended in"},
		{gateDriverHandoffNotChecked, "the sentence has to survive into the file; a record listing `completed: [D0 … D9]` reads like nine things that went right"},
	} {
		if !strings.Contains(record, want.fragment) {
			t.Errorf("the run record does not carry %q. %s.\nIt reads:\n%s", want.fragment, want.why, record)
		}
	}
	for _, check := range gateDriverHumanChecks {
		if !strings.Contains(record, check.Title()) {
			t.Errorf("the run record does not name the check %q. Weeks later this file is the only account of which checks were outstanding when the driver let go.\nIt reads:\n%s", check.Title(), record)
		}
	}
	if strings.Contains(record, `"failed_at"`) {
		t.Errorf("the record of a completed release carries a failed_at. The handoff is a different ending from a failure, and a record carrying both is one that will be read as either.\nIt reads:\n%s", record)
	}
}

// gateDriverHandoffMustNotSay are the words the handoff may not use about the
// release it just published, and the list is a RULE rather than a memory of
// phrasings that went wrong.
//
// Every one of them asserts that something was examined. Nothing was: after D8
// this driver reads no workflow run, no release page and no byte of the live
// site, and the three checks below the handoff are exactly the examinations that
// have not happened. So a handoff carrying one of these words is the failure the
// whole gate is written against — a check nobody made reading as a check that
// passed — arrived at through the report rather than through a verdict, which is
// the one route left now that D9 asks nothing.
//
// The denylist is applied to the driver's own prose, never to the record: the
// record embeds the receipt, whose surfaces really do carry PASS, and those are
// verdicts about documents in this tree that thirteen agents read. The deployed
// release is not one of those documents. That distinction is the reason the two
// are separated here rather than the reason to weaken the rule.
var gateDriverHandoffMustNotSay = []struct{ word, why string }{
	{"PASS", "PASS is the receipt's verdict over a surface of this TREE. Nothing about a deployed release is fingerprintable here, so the word carried into a handoff extends a verdict to a forge, a workflow and a CDN that no agent looked at"},
	{"passed", "the same claim in prose. `Release` passing is the first of the three checks and it is a person's to make"},
	{"verified", "D7's word, and D7 earned it by downloading archives and hashing them. Nothing between D8 and this report did anything"},
	{"confirmed", "a release is confirmed by somebody opening the run and the live site; this driver closed its last connection at the push"},
	{"checked", "the word the ending must never be readable as. Handed off is the claim"},
	{"clean", "'it came back clean' is the sentence an operator wants to see at 2am and the one thing nothing here can say"},
	{"green", "green is the gate's word for a tree it read. The published release is not a tree"},
	{"✓", "a tick is a verdict with no sentence attached, so nobody can even say which claim they are disagreeing with"},
}

// TestTheHandoffIsUnreachableWithoutTheLastIrreversibleAct asks the reachability
// question directly, over runs assembled by hand.
//
// It is asked here rather than only through the sequence because the sequence
// cannot construct these two shapes — which is the point. The guard's job is not
// to catch gateDriverExecute, which reaches D9 only by having pushed main; it is
// to make the terminal state unbuildable from anywhere else, including from the
// step somebody adds after D8 next year. A handoff that could be constructed by
// a run that published nothing is a driver that can tell an operator to go and
// read a site no release put there.
func TestTheHandoffIsUnreachableWithoutTheLastIrreversibleAct(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  *gateDriverRun
		says string
		why  string
	}{
		{
			name: "everything but the push of main",
			run: &gateDriverRun{
				Plan: gateDriverPlan{Version: "v9.9.9"},
				Done: []string{"D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7"},
			},
			says: "has not completed D8",
			why:  "the tag is published, the archives are verified and main is not out. The release the handoff would announce does not exist, and the three checks it hands over are checks on a deploy that never fired",
		},
		{
			name: "past the last push and then stopped",
			run: &gateDriverRun{
				Plan:   gateDriverPlan{Version: "v9.9.9"},
				Done:   []string{"D0", "D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8"},
				Failed: "D10",
				Err:    errors.New("a step added after the pushes could not run"),
			},
			says: "ended as a failure",
			why: "this is the shape the sequence will have the day a step is added after D8. A handoff keyed only to 'main is out' would report that release as finished, " +
				"at the one moment an operator most needs to be told it is not",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handoff, err := tc.run.handoff()
			if err == nil {
				t.Fatalf("this run was handed off: %+v.\n%s", handoff, tc.why)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the refusal does not say %q, so a reader cannot tell which of the two conditions it failed.\nIt reads:\n%v", tc.says, err)
			}
			gateDriverAssertNoHandoff(t, tc.run.report(), tc.name)
		})
	}
}

// gateDriverAssertNoHandoff is the negative half, in one place because both the
// halfway-failure test and the reachability rows below need it: a report that is
// not a handoff must carry nothing OF a handoff.
// Asserting only that `handoff()` refused would leave the failure that matters
// untouched — the operator does not call a Go method, they read the text, and a
// report naming the three checks is a handoff whatever built it.
func gateDriverAssertNoHandoff(t *testing.T, report, what string) {
	t.Helper()
	if strings.Contains(report, gateDriverHandoffNotChecked) {
		t.Errorf("%s printed the handoff's sentence. That sentence says a release is published and unread; over a run that did not publish one it is simply false.\nThe report reads:\n%s", what, report)
	}
	for _, check := range gateDriverHumanChecks {
		if strings.Contains(report, check.Title()) {
			t.Errorf("%s named the human check %q. These three begin when a release is public; handing them to somebody whose release is not is how a half-published release gets closed as done.\nThe report reads:\n%s", what, check.Title(), report)
		}
	}
}

// gateDriverHumanCheckSectionRE isolates docs/RELEASING.md's post-publish
// section, from its heading to the next one at any level. Scoped to the section
// for gateLdflagsItemRE's reason: the document discusses the site, the workflow
// and the deployed bundle in several places, and the question here is what the
// SECTION a maintainer is handed at D9 actually contains.
var gateDriverHumanCheckSectionRE = regexp.MustCompile(`(?s)### ` + regexp.QuoteMeta(gateDriverHumanCheckSection) + `.*?\n## `)

// TestTheHandoffNamesEveryCheckTheProcedureKeepsAPersons keeps the ending and
// the procedure equal, and it is the whole reason the handoff may hold its own
// copy of the three checks.
//
// The driver composes that list after the last irreversible act, so it reads
// nothing off disk to do it — a report that could fail to be produced is the
// report an operator has nothing to read INSTEAD of, and the moment it would
// fail is the moment after main is public. The cost of that decision is a second
// copy of somebody else's list, and the copy that drifts is always the one no
// test drives. This is that test, and it fails at lane-landing time, where a
// disagreement costs a red run rather than a release.
//
// The count is asserted, not only the contents. Three items named and a fourth
// added to the procedure is the failure this catches: the handoff would hand
// over three of four checks and read exactly as complete as it does now.
func TestTheHandoffNamesEveryCheckTheProcedureKeepsAPersons(t *testing.T) {
	procedure := gateReadRepoFile(t, surfaceRepoRoot(t), filepath.FromSlash(gateDriverProcedureFile))

	section := gateDriverHumanCheckSectionRE.FindString(procedure)
	if section == "" {
		t.Fatalf("%s no longer carries a %q section. That section is what D9 hands a person, and the driver names it in the report of every published release; without it the handoff sends the reader to a heading that is not there",
			gateDriverProcedureFile, gateDriverHumanCheckSection)
	}

	if got, want := strings.Count(section, gateDriverHumanCheckPrefix), len(gateDriverHumanChecks); got != want {
		t.Errorf("%s's %q section holds %d item(s) marked %q and this driver hands over %d.\n"+
			"The handoff is the last thing a release prints, so a check the procedure keeps and the driver does not name is one nobody is told about at the only moment they would act on it — and the report reads just as complete without it.\nThe section reads:\n%s",
			gateDriverProcedureFile, gateDriverHumanCheckSection, got, gateDriverHumanCheckPrefix, want, section)
	}

	for _, check := range gateDriverHumanChecks {
		if !strings.Contains(section, check.Item) {
			t.Errorf("%s's %q section carries no item spelled %q.\n"+
				"The handoff prints that title and sends the reader to this section to act on it; a title the section does not carry sends them looking for a check that, as far as the document is concerned, does not exist.\nThe section reads:\n%s",
				gateDriverProcedureFile, gateDriverHumanCheckSection, check.Item, section)
		}
	}
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
	repo := gateDriverFixture(t, version)
	plan := gateDriverAuthorized(t, repo, version)

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
	repo := gateDriverFixture(t, version)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)
	plan := gateDriverAuthorized(t, repo, version)

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
// D7 is wired now (gateArchivesVerify, through gateDriverWired.Archives), so the
// state this constructs is no longer the tree's own — it is the state a failed
// archive check produces on any release: the tag is out, main is not, and the
// human is looking at a half-published release. The fixture builds it from
// gateDriverUnwired.Archives because that refusal is a step whose machinery does
// not exist, which is the cheapest honest way to stop the sequence exactly
// between the two irreversible acts. A driver that pushed main anyway would be
// narrowing coverage silently at the only moment it matters, and at that point
// the report is the whole deliverable.
func TestTheDriverNamesWhatIsAlreadyPublishedWhenItStopsHalfway(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t, version)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)
	plan := gateDriverAuthorized(t, repo, version)

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

	// AND IT IS A FAILURE, not the terminal handoff. This is the state D9 used to
	// leave on every release before the handoff existed — tag public, main not —
	// so it is the one an ending keyed to anything looser than "D8 completed"
	// would hand over as a finished release with three checks remaining.
	if _, handoffErr := run.handoff(); handoffErr == nil {
		t.Error("a run that never pushed main can be handed off. The handoff says a release is out and that three checks are all that is left; this one left a tag on the forge with main behind it, which is what the per-step report below exists to describe")
	}

	report := run.report()
	gateDriverAssertNoHandoff(t, report, "a run that stopped at D7")
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
	repo := gateDriverFixture(t, version)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)
	plan := gateDriverAuthorized(t, repo, version)

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
			e.findings = []gateFinding{{
				Surface:         "readme",
				Rule:            "undocumented-flag",
				Consequence:     gateConsequenceActsWrongly,
				FailureScenario: "a reader who never learns --strict exists runs the loose check in CI and merges what it would have caught",
				Detail:          "--strict is not described",
				Priority:        gatePriorityP1,
			}}
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
			repo := gateDriverFixture(t, version)
			mainWas := gateDriverRemoteHead(t, repo, repo.Base)

			run := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, tc.evidence(gateDriverGreenEvidence()))

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

// TestTheDriverStopsBeforeTheMergeWhenItsEvidenceIsUnwired asserts what an
// evidence source that cannot answer does to the sequence: pointed at one, an
// authorized driver refuses at D1 and publishes nothing.
//
// It survives the wiring unchanged and is not obsolete, because the shape it
// pins is not "this repository has no gate run" — it is the failure-not-skip
// rule turned on the driver itself. The alternative — treating an absent verdict
// transport as "no findings, therefore green" — is a release published on
// evidence nobody produced, and it is the single most natural thing for the next
// implementer to write. gateDriverWired reaches this same D1 refusal whenever
// the tree being released has no fan-out record, which between releases is
// always.
func TestTheDriverStopsBeforeTheMergeWhenItsEvidenceIsUnwired(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t, version)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)

	run := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, gateDriverUnwired{})

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

// TestTheDriverRefusesToTagAReleaseTheTreeDoesNotDeclare is clause 5, and its
// first row is the failure the clause exists for.
//
// THAT ROW IS THE ONE TO READ. The tree is not broken in it: the changelog, the
// site and the code all agree, the branch converges, the gate is green and the
// receipt is measured over exactly the content about to be tagged. Everything
// this driver checked before clause 5 existed passes, because every one of those
// checks is about CONTENT and the thing that is wrong is the NAME the human
// typed. What reaches the forge is a tag reading v9.9.8 over archives, a
// changelog and a site that all say v9.9.9, and no later check can catch it: the
// archives are built FROM the tag, so they are stamped consistently wrong, and
// the site page the release announces is the one in the tree.
//
// The other rows are the shapes where the tree cannot be read at all. They are
// errGateUncheckable rather than a mismatch because they accuse a different
// party: nothing here says the version is wrong, only that this driver could not
// find out, and CLAUDE.md makes that a failed check rather than a quiet pass.
func TestTheDriverRefusesToTagAReleaseTheTreeDoesNotDeclare(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declares string
		mutate   func(*testing.T, gateDriverRepo)
		publish  string
		want     error
		names    []string
		why      string
	}{
		{
			name:     "a self-consistent tree for one release, tagged as another",
			declares: "v9.9.9",
			publish:  "v9.9.8",
			want:     errGateVersionMismatch,
			names:    []string{"v9.9.8", "9.9.9", gateDriverChangelogFile, gateDriverSiteFile},
			why:      "the reader has to see all three values — what was typed, what the changelog says, what the site says — or they have to go and look the third one up before they can act",
		},
		{
			name:     "the changelog was bumped and the site was not",
			declares: "v9.9.9",
			mutate:   func(t *testing.T, repo gateDriverRepo) { gateDriverDeclareRelease(t, repo, "v9.9.9", "v9.9.8") },
			publish:  "v9.9.9",
			want:     errGateVersionMismatch,
			names:    []string{"9.9.9", "9.9.8"},
			why:      "whichever version is tagged, one of the two documents a reader consults about this release is wrong the moment it is published",
		},
		{
			name:     "the tree carries no changelog at all",
			declares: "v9.9.9",
			mutate: func(t *testing.T, repo gateDriverRepo) {
				gateTestGit(t, repo.Dir, "rm", "-q", gateDriverChangelogFile)
				gateTestGit(t, repo.Dir, "commit", "-qm", "chore: drop the changelog")
			},
			publish: "v9.9.9",
			want:    errGateUncheckable,
			names:   []string{gateDriverChangelogFile},
			why:     "a tree that states no release cannot be checked against the version being tagged, and 'could not check' is a failed gate",
		},
		{
			name:     "the site declares no releases array",
			declares: "v9.9.9",
			mutate: func(t *testing.T, repo gateDriverRepo) {
				gateWrite(t, repo.Dir, gateDriverSiteFile, "export const contentSpec = { siteTitle: \"DossierX\" };\n")
				gateTestGit(t, repo.Dir, "add", "-A")
				gateTestGit(t, repo.Dir, "commit", "-qm", "refactor: rewrite the site content")
			},
			publish: "v9.9.9",
			want:    errGateUncheckable,
			names:   []string{gateDriverSiteReleasesDecl},
			why:     "the site is the other half of what the tree says it is, and half an answer is not one",
		},
		{
			name:     "the site no longer derives its current release from the last entry",
			declares: "v9.9.9",
			mutate: func(t *testing.T, repo gateDriverRepo) {
				gateWrite(t, repo.Dir, gateDriverSiteFile,
					gateDriverSiteReleasesDecl+"\n  {\n    version: \"v9.9.9\",\n  },\n  {\n    version: \"v0.0.1\",\n  },\n];\n\nexport const latestRelease = releases[0];\n")
				gateTestGit(t, repo.Dir, "add", "-A")
				gateTestGit(t, repo.Dir, "commit", "-qm", "refactor: newest release first")
			},
			publish: "v9.9.9",
			want:    errGateUncheckable,
			names:   []string{gateDriverSiteLatestDecl},
			why:     "reading the last entry is right only while the site itself reads the last entry; once it does not, 'newest' is this driver's guess about someone else's array, and a wrong guess would have cleared v0.0.1 here",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gateDriverFixture(t, tc.declares)
			if tc.mutate != nil {
				tc.mutate(t, repo)
			}
			mainWas := gateDriverRemoteHead(t, repo, repo.Base)

			run := gateDriverPublish(gateDriverAuthorized(t, repo, tc.publish), repo, gateDriverGreenEvidence())

			if run.Err == nil {
				t.Fatalf("the driver published %s over a tree that does not declare it. %s", tc.publish, tc.why)
			}
			if run.Failed != "D1" {
				t.Errorf("the run stopped at %s; the version is checked in the precondition, before anything is merged", run.Failed)
			}
			if !errors.Is(run.Err, tc.want) {
				t.Errorf("the refusal is %v, and it must be %v — a version that disagrees accuses the invocation, a version that cannot be read accuses the tree, and they take different recoveries", run.Err, tc.want)
			}
			for _, want := range tc.names {
				if !strings.Contains(run.Err.Error(), want) {
					t.Errorf("the refusal does not name %q. %s.\nIt reads:\n%v", want, tc.why, run.Err)
				}
			}
			gateDriverAssertNothingPublished(t, repo, tc.publish, mainWas)
		})
	}
}

// TestTheVersionParsersReadThisRepositorysOwnDocuments points clause 5's two
// parsers at the real files, because every other test of them runs against a
// fixture this file wrote.
//
// A fixture proves the parser reads the shape the fixture has. It cannot prove
// that shape is the one CHANGELOG.md and site/src/content.ts actually carry — and
// if it is not, the driver refuses every release with "this tree declares no
// version", which is safe but useless, or worse, reads a heading out of the wrong
// end of a document and refuses the correct release. So this asks the two
// documents in THIS repository, at HEAD, and requires them to be readable and to
// agree with each other. It deliberately does not require any particular version:
// which release this tree is depends on where in a release cycle it sits, and a
// test that pinned a number would have to be edited by every release.
func TestTheVersionParsersReadThisRepositorysOwnDocuments(t *testing.T) {
	root := surfaceRepoRoot(t)

	changelog, err := gateDriverChangelogVersion(root, "HEAD")
	if err != nil {
		t.Fatalf("clause 5's changelog parser cannot read this repository's own %s: %v\n"+
			"Its only other exercise is against a fixture this file writes, so a shape mismatch here would be invisible until a release refused for a reason that has nothing to do with the release", gateDriverChangelogFile, err)
	}
	site, err := gateDriverSiteVersion(root, "HEAD")
	if err != nil {
		t.Fatalf("clause 5's site parser cannot read this repository's own %s: %v", gateDriverSiteFile, err)
	}
	if gateDriverNormalizeVersion(changelog) != gateDriverNormalizeVersion(site) {
		t.Errorf("this repository's two release declarations disagree: %s's newest heading says %s and %s's last releases[] entry says %s.\n"+
			"That is the release-time refusal, met here at lane-landing time instead: one of the two documents was bumped and the other was not, and whichever version is tagged, one of them is wrong the moment it is published",
			gateDriverChangelogFile, changelog, gateDriverSiteFile, site)
	}
}

// TestTheDriverRefusesToPublishWithoutTheCIRunEvidenceForThisTree is clause 6.
//
// The first row is the whole finding: a gate that is green in every one of its
// own thirteen surfaces, over a tree that declares its own release, with a
// measured receipt — and nobody ran `make ci-evidence`. Before this check the
// driver merged, tagged and pushed that release, because no step of the sequence
// had any reason to notice: the record is written by a stage the driver does not
// invoke, and a file that was never written raises nothing.
//
// The third row is the one that makes the check about THIS release rather than
// about a file existing. A record from the previous release is on disk at the
// default path on any machine that has released before — `make ci-evidence`
// writes to /tmp and nothing cleans it up — so "the file is there" would be
// satisfied on exactly the machines this check exists to protect.
func TestTheDriverRefusesToPublishWithoutTheCIRunEvidenceForThisTree(t *testing.T) {
	const version = "v9.9.9"
	for _, tc := range []struct {
		name  string
		point func(*testing.T, gateDriverRepo) string
		why   string
	}{
		{
			name:  "nobody ran the stage",
			point: func(t *testing.T, _ gateDriverRepo) string { return filepath.Join(t.TempDir(), "never-written.json") },
			why:   "a release published with no CI evidence at all is the skip that reads as a pass, arrived at by nobody running the stage rather than by the stage saying nothing",
		},
		{
			name: "the stage died mid-write and left an empty file",
			point: func(t *testing.T, _ gateDriverRepo) string {
				path := filepath.Join(t.TempDir(), "ci-run-evidence.json")
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatalf("write the empty record: %v", err)
				}
				return path
			},
			why: "an empty file is present, readable, and says nothing about any commit",
		},
		{
			name: "the record left over from the last release",
			point: func(t *testing.T, repo gateDriverRepo) string {
				return gateDriverCIEvidenceRecord(t, gateTestGit(t, repo.Dir, "rev-parse", gateBaseRef))
			},
			why: "the record names a real commit that CI really ran over, and it is not the content this release would publish — which is what every stale record in /tmp looks like",
		},
		{
			name:  "no path to look at",
			point: func(*testing.T, gateDriverRepo) string { return "" },
			why:   "an unset path must refuse rather than skip the question",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := gateDriverFixture(t, version)
			mainWas := gateDriverRemoteHead(t, repo, repo.Base)

			plan := gateDriverAuthorized(t, repo, version)
			plan.CIEvidence = tc.point(t, repo)

			run := gateDriverPublish(plan, repo, gateDriverGreenEvidence())

			if run.Err == nil {
				t.Fatalf("the driver published a release whose CI evidence it never had. %s", tc.why)
			}
			if run.Failed != "D1" {
				t.Errorf("the run stopped at %s; the CI-run evidence is required in the precondition, before anything is merged", run.Failed)
			}
			if !errors.Is(run.Err, errGateUncheckable) {
				t.Errorf("a missing CI-run evidence record must be reported as a check that could not be made; got %v", run.Err)
			}
			if !strings.Contains(run.Err.Error(), "make "+gateDriverCIEvidenceTarget) {
				t.Errorf("the refusal does not name `make %s`. A refusal that states a missing file without the command that produces it is one the operator has to go and research at the worst moment.\nIt reads:\n%v",
					gateDriverCIEvidenceTarget, run.Err)
			}
			gateDriverAssertNothingPublished(t, repo, version, mainWas)
		})
	}
}

// TestTheDriverLooksForTheCIEvidenceRecordWhereTheRecipeWritesIt keeps the two
// spellings of one path equal.
//
// The driver and `make ci-evidence` are in different languages and there is no
// compile-time link between them, so the default path exists twice: once as a
// `?=` assignment in the Makefile and once as a Go constant here. Make does not
// export a `?=` default into a recipe's environment — measured: a makefile
// carrying `FOO ?= defaultval` prints FOO=[] from `echo $$FOO`, while `make show
// FOO=cmdline` prints FOO=[cmdline] — so the Go constant is what the driver
// actually uses on an ordinary run, and if the two drift the driver looks for a
// record at a path nothing writes and refuses every release for a reason that has
// nothing to do with the release.
func TestTheDriverLooksForTheCIEvidenceRecordWhereTheRecipeWritesIt(t *testing.T) {
	makefile := gateReadRepoFile(t, surfaceRepoRoot(t), "Makefile")

	if want := gateDriverCIEvidenceEnv + " ?= " + gateDriverCIEvidenceDefault; !strings.Contains(makefile, want) {
		t.Errorf("the Makefile no longer carries `%s`. That assignment is where the CI-run evidence record is written, and %s is where this driver looks for it; the two are the same path spelled in two languages, and nothing but this test holds them together",
			want, gateDriverCIEvidenceDefault)
	}
	if want := gateDriverCIEvidenceEnv + `="$(` + gateDriverCIEvidenceEnv + `)"`; !strings.Contains(makefile, want) {
		t.Errorf("the `%s` recipe no longer hands %q to the stage that writes the record. Make does not export its own variables to a recipe's environment, so without that line the stage writes wherever ITS default points and this driver looks somewhere else",
			gateDriverCIEvidenceTarget, want)
	}
}

// gateDriverWorktreeHolding checks branch out in a SECOND worktree of the
// fixture's repository — the linked-worktree layout this repository's own
// releases are prepared in — and returns the path git records for it.
//
// It returns GIT'S spelling of the path rather than the one it was handed. A
// t.TempDir() under /var on macOS is /private/var once git has resolved it, and
// the refusal below is required to NAME the worktree; a test comparing the two
// spellings would go red over a symlink and tell whoever met it that the driver
// was broken.
func gateDriverWorktreeHolding(t *testing.T, repo gateDriverRepo, branch string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "the-other-checkout")
	gateTestGit(t, repo.Dir, "worktree", "add", "-q", path, branch)
	return gateTestGit(t, path, "rev-parse", "--show-toplevel")
}

// TestTheDriverRefusesWhenTheBaseBranchIsCheckedOutInAnotherWorktree is the
// environment question, and the release it is about is this one.
//
// THE FAILURE WITHOUT IT IS SAFE, LATE AND MUTE, and all three words matter. D2
// runs `git checkout main`, git allows a branch in one worktree at a time, and in
// the layout a release is actually prepared in — the branch in a worktree, `main`
// in the primary checkout — git refuses. Nothing is published, because D2 is
// before the first irreversible act. But the refusal arrives after a whole green
// gate run has been paid for, and what the operator reads is `fatal: 'main' is
// already used by worktree at …`, which names no release, no step and no way
// forward. Asked in D1 it costs two git commands, and the sentence it produces is
// this driver's.
//
// The second half is what stops the check from being a wall. Detach the worktree
// that holds the branch — the recovery the refusal prints, verbatim — and the
// identical invocation goes on to publish. A precondition that refuses and then
// refuses the fixed state too is a precondition nobody can satisfy.
func TestTheDriverRefusesWhenTheBaseBranchIsCheckedOutInAnotherWorktree(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t, version)
	mainWas := gateDriverRemoteHead(t, repo, repo.Base)
	elsewhere := gateDriverWorktreeHolding(t, repo, repo.Base)

	run := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, gateDriverGreenEvidence())

	if run.Err == nil {
		t.Fatal("the driver ran to completion in a layout where D2's `git checkout` cannot succeed")
	}
	if run.Failed != "D1" {
		t.Errorf("the run stopped at %s. This question is asked in the precondition, before the evidence is gathered and before the merge: it needs nothing but git, and every step between here and D2 is work that was always going to be thrown away", run.Failed)
	}
	if !errors.Is(run.Err, errGateUncheckable) {
		t.Errorf("an environment D2 cannot run in must be reported as a check that could not be made; got %v", run.Err)
	}
	for _, want := range []struct{ fragment, why string }{
		{elsewhere, "the operator cannot act on this without being told WHICH checkout is holding the branch, and in a repository with a dozen worktrees they are not going to guess"},
		{"git -C " + elsewhere + " switch --detach", "the recovery has to be the command, ready to run against that path. A refusal that describes a fix in prose is one the operator has to translate at the worst moment"},
		{"switch that worktree back", "detaching is half a recovery; the other half is putting the worktree back afterwards, and nothing else will remind them"},
	} {
		if !strings.Contains(run.Err.Error(), want.fragment) {
			t.Errorf("the refusal does not carry %q. %s.\nIt reads:\n%v", want.fragment, want.why, run.Err)
		}
	}
	gateDriverAssertNothingPublished(t, repo, version, mainWas)

	// The recovery, performed exactly as the refusal spells it — and then the
	// same release, with nothing else changed.
	gateTestGit(t, elsewhere, "switch", "--detach")

	second := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, gateDriverGreenEvidence())
	if second.Err != nil {
		t.Fatalf("the recovery the refusal printed did not clear it, so this precondition cannot be satisfied by doing what it says:\n%s", second.report())
	}
	if !second.completed("D2") {
		t.Errorf("the run cleared D1 and never reached D2 (%v). The whole subject of this check is whether D2's checkout can run", second.Done)
	}
}

// TestTheWorktreeCheckIgnoresTheCheckoutItIsRunningIn is the false refusal the
// check would otherwise produce, and it is not a hypothetical shape: `git
// worktree list` names the invoking checkout alongside every other one, so a
// version of this question that only looked for the branch in the listing would
// find it in THIS worktree and refuse a release that was about to work perfectly.
//
// A maintainer who keeps `main` checked out and cuts the release branch elsewhere
// is in exactly that state. D2's `git checkout main` is then a no-op git performs
// without complaint, which is why the driver asks what this checkout holds before
// it asks what anything else does.
func TestTheWorktreeCheckIgnoresTheCheckoutItIsRunningIn(t *testing.T) {
	const version = "v9.9.9"
	repo := gateDriverFixture(t, version)

	// The invoking checkout now holds the base branch itself. The release branch
	// is unchanged and still carries the release; only where HEAD points moved.
	gateTestGit(t, repo.Dir, "checkout", "-q", repo.Base)

	run := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, gateDriverGreenEvidence())
	if run.Err != nil {
		t.Fatalf("a release was refused because the checkout it is running in holds %s — which is the one worktree whose holding it can never be an obstacle, since D2's checkout there is a no-op:\n%s",
			repo.Base, run.report())
	}
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

	// THE TAG PUSH IS MATCHED FULLY QUALIFIED, and that is the assertion rather
	// than a spelling preference. A release developed on a branch named `vX.Y.Z`
	// makes the bare form fail outright — `error: src refspec v0.5.0 matches more
	// than one` — so a document prescribing it prescribes a command that does not
	// run for the release most likely to be reading it. The driver already pushes
	// `<tag-object>:refs/tags/<version>`; this keeps the written half from
	// drifting back to the ambiguous spelling.
	tag := strings.Index(item, "git push origin refs/tags/vX.Y.Z")
	main := strings.Index(item, "git push origin main")
	switch {
	case tag < 0:
		t.Error("the item no longer tells a maintainer to push the TAG as `git push origin refs/tags/vX.Y.Z`. The tag push is what fires the Release workflow and it is the first irreversible act — and it is written fully qualified because a bare `git push origin vX.Y.Z` is refused by git as an ambiguous refspec whenever a branch shares the name, which is exactly the case on a release developed on a branch called vX.Y.Z")
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
	if strings.Contains(procedure, "git push origin main\n      git push origin refs/tags/vX.Y.Z") {
		t.Error("docs/RELEASING.md still carries the old command block that pushes main before the tag. Two orders in one document is the second release procedure CLAUDE.md forbids, and this one publishes a site announcing a release whose archives do not exist")
	}

	// The ambiguous spelling must not survive as an INSTRUCTION anywhere in the
	// document. It is still quoted inside the branch-naming item, deliberately,
	// as the transcript of the error a colliding branch produces — so the search
	// is for the command in a prescribed position, at the indentation the
	// document's command blocks use, and not for the characters anywhere at all.
	if strings.Contains(procedure, "\n      git push origin vX.Y.Z") {
		t.Error("docs/RELEASING.md still prescribes `git push origin vX.Y.Z` in a command block. That form is ambiguous the moment a branch shares the tag's name: git refuses it with `src refspec ... matches more than one` rather than choosing, so it is a command that fails for the release most likely to be following this document. Write it as `git push origin refs/tags/vX.Y.Z`, which the driver already does")
	}
}
