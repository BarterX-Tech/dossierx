// gate_evidence_test.go is THE WIRING: the file that turns the fan-out this
// repository actually produces into the evidence the release driver publishes
// on.
//
// WHAT WAS MISSING, read off the tree rather than off a plan. Every piece of a
// gate run existed and not one of them reached the driver. gate_fanout_test.go
// mints a run identifier and writes one bundle per declared surface;
// gate_stage2_test.go computes the key each answer must match;
// gate_stage3_test.go reads the answers back into one record with a verdict per
// surface and a join no per-surface agent can perform; gate_archives_test.go
// reads the archives a release actually published. The driver was handed
// gateDriverUnwired, whose four answers are all errGateUncheckable, so an
// authorized run refused at D1 and published nothing — correct under CLAUDE.md,
// and permanent until something connected those files. This is that something.
//
// IT IS A CALL GRAPH AND NOTHING ELSE, and that is a rule rather than a
// preference. Not one refusal below is invented here: whether an answer is
// missing is gateStage3Collect's question, whether it was produced by THIS run
// is gateReadFanout's and gateStage3ValidateAnswer's, whether the record is
// green is gateIsGreen's through gateReceipt.evaluate, and whether the
// published archives are this release is gateArchivesVerify's. A second copy of
// any of those rules here would be two answers to one question inside one
// package — the two-procedures defect CLAUDE.md names, at function scale — and
// the copy that drifts is always the one no test drives. What this file OWNS is
// the wiring itself: which questions get asked, in what order, over which tree,
// and that every error is returned rather than summarised. That is exactly what
// the tests at the bottom mutate.
//
// THREE ANSWERS WIRED AND ONE DELIBERATELY NOT. Verdicts, Current and Archives
// now read this repository. Site stays errGateUncheckable — see gateDriverWired.Site:
// D9 is the accepted human residual, recorded as such in docs/RELEASING.md's
// "Three checks that stay a person's", and its unbuilt state is a decision.
//
// WHAT THE WIRING STILL DOES NOT ESTABLISH, stated here rather than discovered
// at the worst moment. It does not make the thirteen agents' answers honest.
// Everything below rests on gate/answers/<surface>.json having been written by
// a harness that ran an agent, and the strongest thing this file can say about
// one of those files is that it names THIS run's minted identifier and this
// tree's key for its surface (gate_fanout_test.go's header states the same
// boundary from the producing side). A hand-written answer carrying the right
// run and the right fingerprint satisfies every check here. What is closed is
// the failure that actually happens — a run collecting a previous release's
// answers, or reporting on twelve surfaces while the manifest declares
// thirteen — not forgery.
//
// Same shape as the rest of the gate: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
// gate_driver_test.go's "WHY IT IS TEST CODE" note is the whole argument and is
// not repeated here.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// the evidence source the real repository gets
// ---------------------------------------------------------------------

// gateDriverWired is gateDriverEvidence answered from a checkout.
//
// Root is the repository the gate run was produced in, and it is a field rather
// than a probe for the reason gateFanoutProduce's tree is a parameter: the
// driver already knows which checkout it is releasing, and a second answer to
// "which tree is this run about" computed inside one run is how two halves of
// one release end up describing two trees.
type gateDriverWired struct{ Root string }

var _ gateDriverEvidence = gateDriverWired{}

// Current is the manifest's declared surfaces and this tree's key for each of
// them — the pair gateReceipt.evaluate recomputes green against.
//
// IT IS ALSO WHAT Verdicts USES, by calling this method rather than repeating
// its three lines. The receipt's per-surface fingerprints are compared against
// the map this returns, so two computations of it that agree today are two
// computations: the day one of them starts resolving tracked files differently,
// or reading a different tree, evaluate() compares a run's answers against a
// map somebody else filled and reports green over the disagreement.
func (w gateDriverWired) Current(tree string) ([]string, map[string]string, error) {
	declared, err := gateDeclaredSurfaces(w.Root)
	if err != nil {
		// WRAPPED IN THE SENTINEL, because the manifest reader does not carry
		// one and this is a caller that classifies. gateDriverRun.precondition
		// hands this error straight to a human, and errors.Is is how the rest of
		// the gate tells "the check could not be made" from "the check was made
		// and the answer is no" — a bare error out of here is neither, and an
		// operator reading it has no way to know which recovery they are in.
		return nil, nil, fmt.Errorf("%w: the fan-out could not be read from %s, so this run cannot say which surfaces a release is supposed to cover: %w",
			errGateUncheckable, gateManifestFile, err)
	}
	// `git ls-files`, through the resolver that already exists for callers that
	// cannot take a *testing.T. A second reader of the tracked set would be a
	// second answer to "what does this surface own", and the two would diverge
	// in silence because only one of them is under test.
	tracked, err := gateLedgerTrackedFiles(w.Root)
	if err != nil {
		return nil, nil, err
	}
	// gateStage2Plan is freshness, then the delta's own state, then the
	// agreement between them, then a key per declared surface — in that order
	// and by that one call, because it is the only path those checks are on.
	current, err := gateStage2Plan(w.Root, tree, tracked)
	if err != nil {
		return nil, nil, err
	}
	return declared, current, nil
}

// Verdicts is this run's per-surface answers and every finding they carry.
//
// THE ORDER OF THE QUESTIONS IS THE CHEAP ONES FIRST, which is
// gateFanoutProduce's rule ("THE IDENTITY FIRST") applied on the reading side.
// The fan-out record is two fields and it decides whether anything on disk
// belongs to this release at all; the vocabulary is one file and it decides
// whether the join can fire; the keys are thirteen fingerprints over every
// document the manifest resolves. Asking the expensive question first only
// delays the same refusal.
//
// EVERY ERROR IS RETURNED WHOLE. Nothing here summarises one, none of them is
// returned alongside a partial record, and no branch below returns a nil error
// with less than one answer per declared surface — that shape is a receipt
// measured over a run nobody completed, and it evaluates to PASS.
func (w gateDriverWired) Verdicts(tree string) ([]gateSurfaceVerdict, []gateFinding, error) {
	// WHICH RUN THIS IS. Without the record, the answers under gate/answers are
	// files somebody found on disk: a previous release's, an interrupted run's,
	// or hand-written. gateReadFanout is the only reader of that record and it
	// refuses all four of those shapes.
	fanout, err := gateReadFanout(w.Root, tree)
	if err != nil {
		return nil, nil, err
	}

	// The closed subject vocabulary, read from the frame all thirteen agents
	// were handed. It is read HERE, from the same tree, rather than being
	// declared in Go: a vocabulary that lived in this package would be a
	// vocabulary the agents never saw, every answer would validate against it,
	// and the join would group thirteen singletons and report no collision on
	// every tree forever.
	vocabulary, err := gateStage3ReadVocabulary(w.Root)
	if err != nil {
		return nil, nil, err
	}

	declared, current, err := w.Current(tree)
	if err != nil {
		return nil, nil, err
	}

	// A NIL PREVIOUS RUN, DELIBERATELY, and it is the one decision in this file
	// that is not plumbing.
	//
	// gatePlanRerun's whole purpose is to carry a previous run's verdict
	// forward when a surface's key is byte-identical, and that is right for a
	// re-run inside a gate cycle: it is what stops thirteen agents being paid
	// for a one-document fix. It is wrong HERE. This driver is reading a
	// COMPLETED fan-out in order to publish, and it carries nothing forward
	// from anywhere — there is no previous receipt in this process (clause 1:
	// the receipt is measured, never accepted), so the only "previous verdicts"
	// available would have to be read off disk, which is the paper-over-
	// inspection failure the whole driver is written against.
	//
	// With nil, every declared surface lands in plan.Rerun, so every one of
	// them must hold an answer PRODUCED BY THIS RUN — gateStage3ValidateAnswer
	// holds a re-run answer to fanout.Run and a carried one to nothing. That is
	// exactly the coverage a release needs: not "the surfaces that moved were
	// looked at" but "every surface was answered, this time, about this tree".
	plan, err := gatePlanRerun(nil, current)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: the run's shape could not be planned over this tree's %d declared surface(s): %w",
			errGateUncheckable, len(declared), err)
	}

	answers, err := gateStage3Collect(gateStage3Inputs{
		Root:       w.Root,
		Run:        fanout.Run,
		Declared:   declared,
		Current:    current,
		Plan:       plan,
		Vocabulary: vocabulary,
		// Previous is nil for the same reason the plan's is: nothing is carried.
	})
	if err != nil {
		return nil, nil, err
	}

	// gateStage3Findings and NOT gateStage3JoinFindings alone. The join's
	// findings are what no per-surface agent can see and they are the reason
	// this call exists at all — but Findings is the projection that carries
	// BOTH: every finding every agent raised, plus the join's. Passing only the
	// join's would drop thirteen agents' findings on the floor between the
	// record and the receipt, silently, on the exact path a release is
	// published from. That is the one thing CLAUDE.md forbids by name, and it
	// would be invisible: a receipt with no findings evaluates to PASS.
	findings, err := gateStage3Findings(answers, vocabulary)
	if err != nil {
		return nil, nil, err
	}
	return gateStage3Verdicts(answers), findings, nil
}

// Archives is D7: the published artifacts for version, verified as the person
// who downloads them reads them.
//
// The signature is already pinned from the other side by
// gateArchivesDriverShape, which is a compile-time declaration and cannot say
// whether anything calls it. This is the call, and TestTheWiredArchivesAnswerIs
// TheArchiveCheck is what keeps it from being replaced by a stub that returns
// nil.
func (gateDriverWired) Archives(version, commit string) error {
	return gateArchivesVerify(version, commit)
}

// Site is D9, and it is the one answer here that stays a refusal.
//
// ITS UNBUILT STATE IS A DECISION AND NOT AN OMISSION. Reading the deployed
// site is one of the three checks docs/RELEASING.md's "Three checks that stay a
// person's" records as a person's, by a ruling that is written down there: they
// ask whether a system outside this repository did what it was told — a
// workflow that never fired, a deploy still serving yesterday's bundle — and a
// tree that went right and a tree that went wrong are byte-identical here, so
// there is nothing in this repository for a check to read. Building it would
// mean this driver running a browser against a public URL between two
// irreversible acts and taking a verdict from it.
//
// So it refuses, and the refusal is gateDriverUnwired's own text rather than a
// second spelling of it: two sentences saying "the site is not read here" is
// the beginning of two of them disagreeing. What the refusal COSTS is real and
// is not hidden — an authorized run reaches D9 and stops there, after both
// pushes, and the report names D6 and D8 as already published. The human then
// performs the three checks. That is the honest shape: the driver never reports
// a check it did not make.
func (gateDriverWired) Site(version string) error {
	return gateDriverUnwired{}.Site(version)
}

// ---------------------------------------------------------------------
// the fixture: a checkout with a whole gate run in it
// ---------------------------------------------------------------------

// gateWiredSubjects is the fixture frame's closed vocabulary.
//
// TWO SUBJECTS, ONE OF WHICH THE TWO SURFACES DISAGREE ABOUT, because a fixture
// where every surface agrees cannot tell a join that ran from a join that was
// never called: both report nothing.
const gateWiredSubjects = "## The subjects you must place\n\n" +
	"With your single `SurfaceVerdict` call, pass a `subjects` map holding one entry for EVERY subject below.\n\n" +
	"- `go-toolchain-floor` — the oldest Go toolchain your documents tell a reader they can build this project with, as a bare `MAJOR.MINOR`. Match: `^[0-9]+\\.[0-9]+$`\n" +
	"- `lock-lifecycle` — how your documents say the content of an already-LOCKED claim is changed. Match: `^(?:unlock-fix-lock|edit-in-place)$`\n\n" +
	"If your surface says nothing at all about a subject, its value is the literal `not-claimed`.\n\n" +
	"## Your answer\n\n"

// gateWiredFrame is the fixture's gate/prompts/_frame.md: the standing
// instruction, the subject vocabulary, and both placeholders the assembler
// requires.
func gateWiredFrame(subjects string) string {
	return "# Surface review — " + gateBundleSurfaceMarker + "\n\n" +
		"Report FAILED on any mismatch you can demonstrate from the material below.\n\n" +
		subjects +
		gateBundlePartsMarker + "\n"
}

// gateWiredAnswerSpec is one surface's answer as a test wants to state it. The
// fingerprint is not here on purpose: it is this tree's key for the surface and
// is computed, because an answer whose fingerprint a test typed would pass the
// comparison that exists to catch an agent answering about other inputs.
type gateWiredAnswerSpec struct {
	Surface  string
	Verdict  string
	Findings []gateFinding
	Subjects map[string]string
}

// gateWiredHealthyAnswers is the honest record every mutation below starts
// from: both surfaces answered, both PASS, and they disagree about the Go
// floor — which is a run no per-surface agent can fault and the join can.
func gateWiredHealthyAnswers() []gateWiredAnswerSpec {
	return []gateWiredAnswerSpec{
		{Surface: "readme", Verdict: gateVerdictPass, Subjects: map[string]string{
			"go-toolchain-floor": "1.26",
			"lock-lifecycle":     "unlock-fix-lock",
		}},
		{Surface: "roadmap", Verdict: gateVerdictPass, Subjects: map[string]string{
			"go-toolchain-floor": "1.25",
			"lock-lifecycle":     gateStage3NotClaimed,
		}},
	}
}

// gateWiredFixture is a checkout gateDriverWired can be pointed at for real: a
// small, complete, honest stage-2 run (gateStage2PlanFixture's), a frame that
// declares a vocabulary, a git index so `git ls-files` answers, a fan-out
// produced by the REAL producer, and one answer per declared surface.
//
// THE FAN-OUT IS PRODUCED RATHER THAN WRITTEN. gateFanoutProduce mints the
// identifier and writes gate/fanout.json; a fixture that hand-wrote that record
// would be testing this wiring against a document shape nothing in the
// repository produces, and the producer and the reader could then disagree with
// every test here green.
//
// It is synthetic rather than the real repository for gateLedgerQuotableRepo's
// second reason: a fan-out over the real tree writes into gate/ of the checkout
// other agents and other test binaries are reading, and the real tree's own
// per-run evidence is not a thing a test may create or destroy.
func gateWiredFixture(t *testing.T) (root, run string) {
	t.Helper()
	root, _ = gateStage2PlanFixture(t)

	// The plan fixture's frame carries no subject vocabulary — it is written for
	// stage 2, which does not have one. gate/prompts/_frame.md is not one of the
	// artifacts gate/run.json records (gateSharedEvidence is the four shared
	// files and the frame is none of them), so rewriting it here does not make
	// the run manifest stale; it does move every surface key, which is why the
	// answers below are written after it and never before.
	gateWrite(t, root, gateBundleFrameFile, gateWiredFrame(gateWiredSubjects))

	// A real git index, because gateDriverWired resolves the tracked set the way
	// every other non-test caller does — `git ls-files` — and a fixture handing
	// it a literal list would exercise a path production does not take. Nothing
	// is committed: ls-files reads the index.
	gateTestGit(t, root, "init", "-q", "-b", "main")
	gateTestGit(t, root, "add", "-A")

	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("the fixture's tracked set could not be resolved, so nothing below is about the tree it claims to be about: %v", err)
	}
	doc, err := gateFanoutProduce(root, gateStage2FixtureTree, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the producer refused an honest fixture, so every assertion below would be about a run that never happened: %v", err)
	}
	gateWiredWriteAnswers(t, root, doc.Run, gateStage2FixtureTree, gateWiredHealthyAnswers()...)
	return root, doc.Run
}

// gateWiredWriteAnswers writes one answer file per spec, fingerprinted against
// the tree the fixture covers.
//
// It recomputes the keys on every call rather than taking them as a parameter,
// so that a test which moves an input — the frame's vocabulary, a document — can
// re-state the same answers honestly instead of leaving them stale and getting
// a refusal about fingerprints when it meant to construct a different one.
func gateWiredWriteAnswers(t *testing.T, root, run, tree string, specs ...gateWiredAnswerSpec) {
	t.Helper()
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("tracked files: %v", err)
	}
	current, err := gateStage2Plan(root, tree, tracked)
	if err != nil {
		t.Fatalf("this tree's per-surface keys could not be computed, so no answer could be fingerprinted honestly: %v", err)
	}
	for _, spec := range specs {
		key, ok := current[spec.Surface]
		if !ok {
			t.Fatalf("surface %q holds no key on this tree, so an answer for it could not be written honestly", spec.Surface)
		}
		findings := spec.Findings
		if findings == nil {
			// An empty list and an absent key are different facts to
			// gateStage3ValidateAnswer, and only one of them is an answer.
			findings = []gateFinding{}
		}
		blob, err := json.MarshalIndent(gateStage3Answer{
			Run:         run,
			Surface:     spec.Surface,
			Verdict:     spec.Verdict,
			Fingerprint: key,
			Findings:    &findings,
			Subjects:    spec.Subjects,
		}, "", "  ")
		if err != nil {
			t.Fatalf("marshal the answer for %q: %v", spec.Surface, err)
		}
		gateWrite(t, root, gateStage3AnswerFile(spec.Surface), string(blob)+"\n")
	}
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// TestTheWiredEvidenceIsThisRunsFanOut is the positive control, and without it
// every refusal below could be satisfied by a Verdicts that returned an error
// unconditionally.
//
// It asserts the three things the receipt is measured from: one verdict per
// DECLARED surface (asked of the manifest, never of what happened to be
// collected), each carrying this tree's key for that surface, and the findings.
func TestTheWiredEvidenceIsThisRunsFanOut(t *testing.T) {
	root, run := gateWiredFixture(t)
	wired := gateDriverWired{Root: root}

	verdicts, findings, err := wired.Verdicts(gateStage2FixtureTree)
	if err != nil {
		t.Fatalf("a complete, current fan-out was refused, so every refusal below proves nothing:\n%v", err)
	}

	declared, current, err := wired.Current(gateStage2FixtureTree)
	if err != nil {
		t.Fatalf("Current refused a tree Verdicts accepted, so evaluate() would recompute green against a map nobody filled: %v", err)
	}
	if len(declared) < 2 {
		t.Fatalf("the fixture declares %d surface(s); the coverage assertion below would be about almost nothing", len(declared))
	}

	held := map[string]gateSurfaceVerdict{}
	for _, v := range verdicts {
		held[v.Surface] = v
	}
	// COVERAGE ASKED OF THE MANIFEST. Iterating the verdicts and reporting what
	// they cover is the shape that reports on twelve surfaces and states a
	// verdict as though it covered thirteen.
	for _, surface := range declared {
		v, ok := held[surface]
		if !ok {
			t.Errorf("surface %q is declared in %s and the wired evidence holds no verdict for it; the receipt would be measured over a run that did not cover it", surface, gateManifestFile)
			continue
		}
		if v.Verdict != gateVerdictPass {
			t.Errorf("surface %q reads %q and the fixture answered %s", surface, v.Verdict, gateVerdictPass)
		}
		if v.Fingerprint != current[surface] {
			t.Errorf("surface %q's verdict is fingerprinted %s and Current says this tree fingerprints it %s. "+
				"gateIsGreen compares exactly those two, so a run whose halves disagree here reports FAILED on a tree nothing is wrong with — or PASS on one that moved",
				surface, v.Fingerprint, current[surface])
		}
	}
	if len(verdicts) != len(declared) {
		t.Errorf("the run returned %d verdict(s) for %d declared surface(s)", len(verdicts), len(declared))
	}

	// THE JOIN'S FINDING TRAVELS. Both surfaces PASS — each is internally
	// consistent and neither was handed the other's documents — and they name
	// different Go floors. Nothing but the join can see that, and before this
	// wiring existed nothing on the path from a fan-out to a receipt called it.
	var collision *gateFinding
	for i, f := range findings {
		if f.Rule == gateStage3RuleCollision {
			collision = &findings[i]
		}
	}
	if collision == nil {
		t.Fatalf("the two surfaces answered `go-toolchain-floor` as 1.26 and 1.25 and the run carries no %q finding. "+
			"Every surface passed, so this disagreement reaches the receipt through the findings or it reaches no one, and the release publishes over it.\nThe findings are: %+v",
			gateStage3RuleCollision, findings)
	}
	if collision.Surface != gateStage3JoinSurface {
		t.Errorf("the collision is filed under surface %q rather than %q; a disagreement belongs to no single surface, and filing it under one accuses whichever the sort reached first", collision.Surface, gateStage3JoinSurface)
	}
	for _, want := range []string{"1.25", "1.26"} {
		if !strings.Contains(collision.Detail, want) {
			t.Errorf("the collision does not say which surface stated %s. A human's next question is always which value is right, and a finding that names neither sends them to read four documents", want)
		}
	}

	// AND EVERY AGENT'S OWN FINDING TRAVELS TOO. This is the half a projection
	// carrying only the join's findings would drop, on the publish path, in
	// silence: a receipt with no findings evaluates to PASS.
	gateWiredWriteAnswers(t, root, run, gateStage2FixtureTree,
		gateWiredAnswerSpec{Surface: "readme", Verdict: gateVerdictPass, Subjects: map[string]string{
			"go-toolchain-floor": "1.26", "lock-lifecycle": "unlock-fix-lock"}},
		gateWiredAnswerSpec{Surface: "roadmap", Verdict: gateVerdictFailed, Subjects: map[string]string{
			"go-toolchain-floor": "1.26", "lock-lifecycle": gateStage3NotClaimed},
			Findings: []gateFinding{{Surface: "roadmap", Rule: "stale-pin", Severity: "major", Detail: "the roadmap still pins v0.4.1"}}},
	)
	_, findings, err = wired.Verdicts(gateStage2FixtureTree)
	if err != nil {
		t.Fatalf("a run carrying an agent's finding was refused: %v", err)
	}
	var raised bool
	for _, f := range findings {
		if f.Rule == "stale-pin" && f.Surface == "roadmap" {
			raised = true
		}
	}
	if !raised {
		t.Errorf("the roadmap agent reported a finding and the wired evidence does not carry it to the receipt. "+
			"Nothing is filtered, deduplicated away or dropped on its way to the human, and a receipt that lost this one evaluates PASS over a surface that reported FAILED.\nThe findings are: %+v", findings)
	}
}

// TestTheWiredEvidenceRefusesEveryRunItCannotStandBehind walks the states in
// which a fan-out on disk must not become a receipt.
//
// Every row starts from the honest fixture above, so a row that reddened
// because the fixture was broken would show up as the positive control failing
// rather than as coverage. Each row asserts the MESSAGE and not only the fact of
// an error: the whole value of the wiring is which refusal a human is sent to,
// and a row satisfied by any error at all would pass over a refusal that fired
// for some other reason.
func TestTheWiredEvidenceRefusesEveryRunItCannotStandBehind(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, root, run string) (tree string)
		want   error
		says   []string
		why    string
	}{
		{
			name: "no fan-out was recorded at all",
			mutate: func(t *testing.T, root, run string) string {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateFanoutFile))); err != nil {
					t.Fatalf("remove the fan-out record: %v", err)
				}
				return gateStage2FixtureTree
			},
			want: errGateStage2NotProduced,
			says: []string{gateFanoutFile, "is not there"},
			why: "the answers are still on disk, they still name a run, and their fingerprints are still current. " +
				"With no record, the run that produced them cannot be named — which is the difference between an answer an agent gave this release and a file that was found",
		},
		{
			name: "the fan-out covers the release before this one",
			mutate: func(t *testing.T, root, run string) string {
				// The ordinary sequence, not a contrived one: the gate FAILED, a
				// fix landed, the driver re-ran the captures and `record` for the
				// new tree — and did not fan out again. Every stage-2 artifact is
				// honestly digested against the new tree and the answers beside
				// them were given about the old one.
				const moved = "dddddddddddddddddddddddddddddddddddddddd"
				gateStage2WriteEvidence(t, root, moved, "{\"counts\":{\"lint_rules\":27}}\n", "[\"counts\"]")
				gateStage2StampRunFor(t, root, moved)
				return moved
			},
			want: errGateStage2NotProduced,
			says: []string{gateFanoutFile, "records a fan-out over tree"},
			why: "the keys do not move with the tree's NAME, so every answer's fingerprint still matches and every one of them was produced by agents reading another release's bundles. " +
				"Nothing downstream of here can see that",
		},
		{
			name: "a declared surface holds no answer",
			mutate: func(t *testing.T, root, run string) string {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile("roadmap")))); err != nil {
					t.Fatalf("remove the roadmap answer: %v", err)
				}
				return gateStage2FixtureTree
			},
			want: errGateUncheckable,
			says: []string{"roadmap", gateManifestFile},
			why:  "a run reporting on one of two declared surfaces states a verdict as though it covered both, and the surface nobody looked at is the one it says nothing about",
		},
		{
			name: "an answer left over from the previous run",
			mutate: func(t *testing.T, root, run string) string {
				gateWiredRewriteAnswer(t, root, "roadmap", func(a *gateStage3Answer) {
					a.Run = "0123456789abcdef0123456789abcdef"
				})
				return gateStage2FixtureTree
			},
			want: errGateStage2NotProduced,
			says: []string{gateStage3AnswerFile("roadmap"), "was produced by run"},
			why: "its fingerprint matches — most surfaces do not move between two runs over a tree that barely moved — and no agent gave it this release. " +
				"This is the state a re-run after a failed gate leaves on disk every time",
		},
		{
			name: "the subject vocabulary cannot be read",
			mutate: func(t *testing.T, root, run string) string {
				// The frame stays readable and keeps both placeholders, so the
				// bundles still assemble and the keys still compute: what is gone
				// is the section that closes the vocabulary. The answers are
				// re-written afterwards so their fingerprints are honest against
				// the moved frame and this row is about the vocabulary alone.
				gateWrite(t, root, gateBundleFrameFile, gateWiredFrame("A section that declares no subject at all.\n\n"))
				gateWiredWriteAnswers(t, root, run, gateStage2FixtureTree, gateWiredHealthyAnswers()...)
				return gateStage2FixtureTree
			},
			want: errGateUncheckable,
			says: []string{gateBundleFrameFile, gateStage3SubjectsHeading},
			why: "with no closed vocabulary every answer validates, the join groups nothing, and the run reports no collision on this tree and on every future one — " +
				"which is a check whose condition never holds, reported as a clean run",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, run := gateWiredFixture(t)
			tree := tc.mutate(t, root, run)

			verdicts, findings, err := gateDriverWired{Root: root}.Verdicts(tree)
			if err == nil {
				t.Fatalf("the wired evidence handed the driver %d verdict(s) and %d finding(s) for a run it cannot stand behind. %s",
					len(verdicts), len(findings), tc.why)
			}
			if verdicts != nil || findings != nil {
				t.Errorf("the refusal came back alongside %d verdict(s) and %d finding(s); a partial record is one a caller can plausibly report on, and the missing part reaches the human only if every caller remembers to read the error",
					len(verdicts), len(findings))
			}
			if !errors.Is(err, tc.want) {
				t.Errorf("the refusal does not carry %v, so a caller asking errors.Is which KIND of failure this was is sent to investigate the wrong thing.\nIt reads:\n%v", tc.want, err)
			}
			for _, want := range tc.says {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not name %q. %s.\nIt reads:\n%v", want, tc.why, err)
				}
			}
		})
	}
}

// gateWiredRewriteAnswer edits one answer file in place, which is what a
// leftover from a previous run looks like: a file that parses, names a
// well-formed run, and was given by nobody this release.
func gateWiredRewriteAnswer(t *testing.T, root, surface string, edit func(*gateStage3Answer)) {
	t.Helper()
	rel := gateStage3AnswerFile(surface)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var a gateStage3Answer
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	edit(&a)
	blob, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", rel, err)
	}
	gateWrite(t, root, rel, string(blob)+"\n")
}

// TestTheWiredCurrentIsTheSameComputationTheVerdictsWereMeasuredOver is the
// half of clause 2 that a green run cannot demonstrate.
//
// evaluate() compares the receipt's per-surface fingerprints against the map
// Current returns. If the two sides ever read different trees the comparison
// still runs, still reports, and is about nothing — so the property asserted
// here is that Current(tree) answers about the tree it was asked about and
// refuses when that tree is not the one the run produced evidence for, rather
// than answering from whatever is on disk.
func TestTheWiredCurrentIsTheSameComputationTheVerdictsWereMeasuredOver(t *testing.T) {
	root, _ := gateWiredFixture(t)
	wired := gateDriverWired{Root: root}

	verdicts, _, err := wired.Verdicts(gateStage2FixtureTree)
	if err != nil {
		t.Fatalf("the honest fixture was refused: %v", err)
	}
	declared, current, err := wired.Current(gateStage2FixtureTree)
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	// The pair the receipt is judged by, judged here: this is gateIsGreen's
	// question and it is asked through gateIsGreen rather than re-implemented.
	if err := gateIsGreen(declared, verdicts, current); err != nil {
		t.Fatalf("the verdicts this wiring produced are not green against the map it produces for the same tree, so a release over an honest run would be refused for a disagreement between two halves of one function:\n%v", err)
	}

	if _, _, err := wired.Current("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"); err == nil {
		t.Error("Current answered about a tree this run produced no evidence for. It is the map green is recomputed against, so an answer here that is not about the tree being released is a PASS attached to nothing")
	}
}

// TestTheWiredArchivesAnswerIsTheArchiveCheck holds the D7 wiring to the
// function that actually reads a published release.
//
// gateArchivesDriverShape already pins the SIGNATURE at compile time, and a
// signature cannot say whether anything calls it: an Archives that returned nil
// would satisfy it exactly, and the driver would push main having verified
// nothing. So this asserts the answer is gateArchivesVerify's own refusal and
// not the unwired one it replaced.
func TestTheWiredArchivesAnswerIsTheArchiveCheck(t *testing.T) {
	err := gateDriverWired{}.Archives("", "")
	if err == nil {
		t.Fatal("D7 reported success for a release it was not even told the name of; a driver whose archive check can return nil without reading anything announces a release nobody verified")
	}
	if !errors.Is(err, errGateUncheckable) {
		t.Errorf("a check that could not be made must be uncheckable; got %v", err)
	}
	if strings.Contains(err.Error(), "not built in this tree") {
		t.Errorf("D7 still answers with gateDriverUnwired's refusal, so the archive check landed and nothing calls it.\nIt reads:\n%v", err)
	}
	if !strings.Contains(err.Error(), "which release") {
		t.Errorf("the refusal is not gateArchivesVerify's, so what answered is neither the check nor the honest refusal it replaced.\nIt reads:\n%v", err)
	}
}

// TestTheWiredSiteAnswerIsStillARefusal pins D9 as the accepted residual.
//
// It is here so that the unbuilt state is asserted rather than assumed. A Site
// that started returning nil would be the exact failure this whole gate is
// written against — a check nobody made reading as a check that passed — and it
// would show up nowhere else: D9 is the last step, so a nil turns a run that
// stopped honestly into a completed release.
func TestTheWiredSiteAnswerIsStillARefusal(t *testing.T) {
	err := gateDriverWired{}.Site("v9.9.9")
	if err == nil {
		t.Fatal("D9 reported that the deployed site describes this release. Nothing in this tree reads the deployed site, so that is a check that did not happen reading as one that passed — and it is the last step, so the run would report a complete release")
	}
	if !errors.Is(err, errGateUncheckable) {
		t.Errorf("D9's refusal must be uncheckable — the question was not asked, and the answer is not 'no'; got %v", err)
	}
	if !strings.Contains(err.Error(), "v9.9.9") {
		t.Errorf("the refusal does not name the release it could not check, and the operator is reading it beside a forge carrying several tags.\nIt reads:\n%v", err)
	}
}
