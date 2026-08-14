// gate_stage3_test.go is the JOIN: the point where thirteen separate answers
// become one thing a human can act on.
//
// Stage 2 decides which agents run and computes the key each of their answers
// must match. Stage 3 is what happens when they come back. It is the only place
// in the pipeline where an answer that never arrived, an answer about a different
// tree, or an answer whose findings were dropped on the way, can still be told
// apart from a clean pass — after this file, everything downstream is holding a
// []gateSurfaceVerdict and a []gateFinding and has no way to know what is missing
// from them.
//
// THE INVARIANT, in three parts, all three of them load-bearing:
//
//	ONE ANSWER PER DECLARED SURFACE — every surface surfaces.yaml declares holds
//	    exactly one answer, fresh from this run's fan-out or carried from a
//	    previous run whose key for that surface is byte-identical. Absent,
//	    unreadable, half-parsed, doubled, or fingerprinted against a tree this run
//	    did not produce is a FAILED run naming the surface. There is no shape that
//	    means "this surface did not answer" and reads as green.
//	A WHOLE ANSWER OR NONE — a verdict is one FIELD of an answer, not the answer.
//	    The findings that justified a FAILED and the subject values the join is
//	    computed over travel with the verdict through a carry-forward, or nothing
//	    is carried. A carry-forward that keeps the verdict and drops the rest is a
//	    report that gets thinner every time it is re-run, silently, which is the
//	    one thing CLAUDE.md forbids by name.
//	EVERY FINDING IS IN THE RECORD — nothing is filtered, deduplicated away or
//	    dropped on its way to the human. A run over a finding nobody can see is a
//	    run that did not happen.
//
// WHAT THE JOIN DOES NOT CLAIM. It does not find every cross-surface
// contradiction. Thirteen surfaces that agree with each other and are all wrong
// produce no collision, and nothing in the tree arbitrates them. What it does
// claim is that the join is CAPABLE OF FIRING — which rests entirely on the
// subject vocabulary being closed, because a group-by over free-text subjects
// produces thirteen groups of one and reports green forever. The vocabulary is
// declared in gate/prompts/_frame.md, the one file every agent is handed, and
// gateStage3ReadVocabulary below is the parser that makes the declaration
// load-bearing rather than decorative.
//
// WHAT THIS FILE DELIBERATELY DOES NOT BUILD, and the residue that leaves:
//
//   - THE EVIDENCE-DERIVED CLASSIFIER. Findings are collected whole and ordered
//     by nothing; gateFinding.Severity remains a string the agent writes about
//     its own work, read only by gateRecordReceipt's sort comparator. Deriving a
//     classification from evidence needs an evidence bar that the bundle W6
//     landed can actually meet — surface.json carries no source path and no line
//     number anywhere, so C4's "a file:line in code plus the contradicting prose
//     span" is unsatisfiable by construction and every finding would classify
//     UNSUPPORTED. That is a decision about a closed design point, not an
//     implementation gap, and it is left open rather than guessed at.
//   - THE OVERRIDE RECORD. gateReceipt has no field for a human waving a finding
//     through with a rationale, and evaluate() returns FAILED whenever any
//     finding is present, so today the only way to ship past a finding the human
//     has judged non-blocking is to delete it from the record. Adding the field
//     is an edit to gate_receipt_test.go, which is in no lane's write set this
//     wave. Until it exists, an adjudicated finding and a finding nobody raised
//     are indistinguishable to the next release. THIS NOW HAS A ROUTINE CALLER:
//     a subject no surface claims is a legitimate state of a tree and is raised
//     as a finding (gateStage3JoinFindings), so the first release with a quiet
//     subject blocks until a human either answers the subject, removes it from
//     the frame's vocabulary, or has somewhere to record the override. That is
//     the honest ordering — the human is asked — but it is a real cost and it is
//     the strongest argument for the field.
//   - A PRODUCER. Nothing in this repository writes gate/answers/<surface>.json
//     yet; scripts/gate-stage2/run.sh has nine modes and none of them reads an
//     agent's answer back. (It said six until v0.5.2, which was wrong at seven
//     and is the reason the count is now checked rather than recalled — see
//     TestGateStage3ModeCountIsCountedAndNotRemembered below.) This file pins the
//     SCHEMA and refuses everything
//     malformed, which is the same position the receipt is in and is defensible
//     — but it means the first real run's harness has a contract to satisfy
//     rather than a contract to invent.
//   - PROVENANCE ON THE RECEIPT. Stage 3 knows which answers were fresh and
//     which were carried (gateStage3Fresh below), and gateSurfaceVerdict has no
//     field to carry it, so a PASS carried across three releases still reads to
//     the human exactly like one produced minutes ago. That is a field on a type
//     in gate_fingerprint_test.go, outside this write set.
//
// Same shape as the rest of the gate: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// the subject vocabulary, read from the frame the agents are handed
// ---------------------------------------------------------------------

// gateStage3SubjectsHeading is the section of gate/prompts/_frame.md that
// declares the closed vocabulary.
//
// IT IS IN THE FRAME AND NOWHERE ELSE ON PURPOSE. The vocabulary has to be seen
// by all thirteen agents or the join matches nothing, and the frame is the one
// file every bundle contains by construction — so one edit reaches thirteen
// agents with no assembler change, no new tracked artifact and no gate/.gitignore
// entry. A vocabulary that lived only here, in the stage-3 test, would be a
// vocabulary the agents cannot read: every answer would use its own wording, the
// group-by would produce singletons, and every assertion below would be vacuous
// while staying green.
//
// Editing the frame correctly re-runs all thirteen surfaces, because the frame
// reaches every key through the bundle digest. That is the right price: the
// question changed.
const gateStage3SubjectsHeading = "## The subjects you must place"

// gateStage3NotClaimed is the value an agent states for a subject its surface
// says nothing about.
//
// It exists so that silence is an ANSWER rather than an absence. Without it,
// "this surface makes no claim about the Go floor" and "this agent did not look
// at the Go floor" are the same bytes — the skip-reading-as-a-pass shape at
// tuple granularity, and the reason gateStage3ValidateAnswer refuses an answer
// that omits a subject instead of defaulting it.
const gateStage3NotClaimed = "not-claimed"

// gateStage3AnswerDir is where the fan-out's answers land. It is per-run
// evidence with no committed form, so gate/.gitignore's ignore-everything rule
// already covers it and must not be widened for it.
const gateStage3AnswerDir = "gate/answers"

// gateStage3AnswerFile is one surface's answer, addressed by surface name. The
// name is in the PATH and repeated in the document, and
// gateStage3ValidateAnswer requires them to agree — an answer for `changelog`
// sitting at site.json is two answers for changelog, which is exactly the
// duplicate gateIndexVerdicts refuses one stage down.
func gateStage3AnswerFile(surface string) string {
	return gateStage3AnswerDir + "/" + surface + ".json"
}

// gateStage3Subject is one question every surface must answer under the same
// name, and the form its answer must take.
type gateStage3Subject struct {
	ID    string
	Match *regexp.Regexp
}

var (
	// A subject opens a list item in the frame: "- `subject-id` — prose".
	gateStage3SubjectItem = regexp.MustCompile("(?m)^- `([a-z0-9-]+)` ")
	// Its value form is stated as "Match: `<regexp>`", possibly wrapped onto the
	// next line, which is why this is not anchored to a line.
	gateStage3SubjectMatch = regexp.MustCompile("Match:\\s*`([^`]+)`")
)

// gateStage3ReadVocabulary parses the closed subject vocabulary out of the frame.
//
// EVERY REFUSAL HERE IS A REFUSAL RATHER THAN A SHORTER VOCABULARY, for the same
// reason gateStage2Keys never returns a partial key map: a vocabulary with one
// subject quietly dropped is a join that still runs, still reports, and can no
// longer see the disagreement that subject existed to catch. The caller cannot
// tell that from a clean run.
//
// The anchoring rule is the one that looks pedantic and is not. An unanchored
// pattern matches a SUBSTRING, so `[0-9]+\.[0-9]+` accepts "a Go 1.26+ toolchain
// or newer, see CONTRIBUTING" — and the moment one surface's value is prose, the
// group-by is back to comparing free text and the join is back to producing
// singletons. The whole point of the vocabulary is that two surfaces answering
// the same way produce the same STRING.
func gateStage3ReadVocabulary(root string) ([]gateStage3Subject, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateBundleFrameFile)))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w. The vocabulary is declared in the frame because that is the only file all thirteen agents are handed; with no frame there is no shared vocabulary and the join can only produce singletons",
			errGateUncheckable, gateBundleFrameFile, err)
	}
	text := string(raw)
	start := strings.Index(text, gateStage3SubjectsHeading)
	if start < 0 {
		return nil, fmt.Errorf("%w: %s carries no %q section, so no agent is being asked for a subject value at all and the cross-surface join has nothing to group",
			errGateUncheckable, gateBundleFrameFile, gateStage3SubjectsHeading)
	}
	body := text[start+len(gateStage3SubjectsHeading):]
	if end := strings.Index(body, "\n## "); end >= 0 {
		body = body[:end]
	}

	starts := gateStage3SubjectItem.FindAllStringSubmatchIndex(body, -1)
	if len(starts) == 0 {
		return nil, fmt.Errorf("%w: %s declares the %q section and lists no subject in it; a join over an empty vocabulary is a function with one output, and it reports no collision on every tree forever",
			errGateUncheckable, gateBundleFrameFile, gateStage3SubjectsHeading)
	}

	var out []gateStage3Subject
	seen := map[string]bool{}
	var problems []string
	for i, loc := range starts {
		id := body[loc[2]:loc[3]]
		end := len(body)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		block := body[loc[0]:end]

		if seen[id] {
			problems = append(problems, fmt.Sprintf("subject %q is declared twice; two declarations of one subject can state two different value forms and the second silently wins", id))
			continue
		}
		seen[id] = true

		m := gateStage3SubjectMatch.FindStringSubmatch(block)
		if m == nil {
			problems = append(problems, fmt.Sprintf("subject %q states no `Match:` pattern, so nothing constrains the value an agent writes and two surfaces that agree can still answer in two different strings", id))
			continue
		}
		pattern := m[1]
		if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
			problems = append(problems, fmt.Sprintf("subject %q's pattern %q is not anchored at both ends; an unanchored pattern accepts a value with prose around it, and prose is what the vocabulary exists to eliminate", id, pattern))
			continue
		}
		re, compileErr := regexp.Compile(pattern)
		if compileErr != nil {
			problems = append(problems, fmt.Sprintf("subject %q's pattern %q does not compile: %v", id, pattern, compileErr))
			continue
		}
		if re.MatchString(gateStage3NotClaimed) {
			problems = append(problems, fmt.Sprintf("subject %q's pattern also matches %q, so a deliberate silence and a stated value are the same token and the join would treat thirteen silences as thirteen agreements", id, gateStage3NotClaimed))
			continue
		}
		out = append(out, gateStage3Subject{ID: id, Match: re})
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("%w: the subject vocabulary in %s is not usable:\n  %s", errGateUncheckable, gateBundleFrameFile, strings.Join(problems, "\n  "))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ---------------------------------------------------------------------
// the answer record
// ---------------------------------------------------------------------

// gateStage3Answer is ONE surface's whole answer — the shape the harness writes
// when an agent returns and the only shape stage 3 will read.
//
// It is the answer and not the verdict. gateSurfaceVerdict is three strings and
// is what the receipt carries; everything else an agent produced — what it found
// and where it placed each subject — has to survive the trip from the fan-out to
// the report, including across a carry-forward, or the report gets thinner every
// re-run.
type gateStage3Answer struct {
	// Run is the identifier of the fan-out that produced this file. It is minted
	// per RUN and never derived from the tree: stage 2's freshness check cannot
	// reach an artifact that does not exist until after the fan-out (gate/run.json
	// is sealed before any answer is written), so this field is the only thing
	// standing between "this run produced it" and "it was on disk". A tree-derived
	// id would be identical across the re-run that follows a partial fan-out,
	// which is precisely the case that must be caught.
	Run     string `json:"run"`
	Surface string `json:"surface"`
	Verdict string `json:"verdict"`
	// Fingerprint is the stage-2 key of the inputs this answer was produced
	// over — the same value gatePlanRerun compares. It travels inside the answer
	// for gateSurfaceVerdict's reason: provenance stored beside a verdict can be
	// re-attached to a tree the verdict was never computed over.
	Fingerprint string `json:"fingerprint"`
	// Findings is a POINTER so that an absent `findings` key and an empty list are
	// different facts, the same distinction gateStage2Delta.Changed draws. An
	// empty list is a real answer — a PASS says exactly that. An absent key means
	// the producer never wrote what the agent found, which is a truncated or
	// hand-made answer and must not read as "it found nothing".
	Findings *[]gateFinding `json:"findings"`
	// Subjects is one value for EVERY subject the vocabulary declares, including
	// gateStage3NotClaimed for the ones this surface says nothing about.
	Subjects map[string]string `json:"subjects"`
}

// gateStage3MintRun mints a run identifier.
//
// Random rather than derived, for the reason on Answer.Run above: two fan-outs
// over the same tree must be distinguishable, and after a partial fan-out the
// tree is exactly what has not changed.
func gateStage3MintRun() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("%w: a run identifier could not be minted, so no answer this run collects could be distinguished from one left on disk by the last: %w", errGateUncheckable, err)
	}
	return hex.EncodeToString(b[:]), nil
}

var gateStage3RunID = regexp.MustCompile(`^[0-9a-f]{16,64}$`)

// gateStage3LoadAnswer reads one answer file. Absent and unparseable are both
// UNCHECKABLE rather than absent-therefore-fine: the surface was fanned out, so
// something was supposed to answer, and nothing did.
func gateStage3LoadAnswer(root, surface string) (gateStage3Answer, error) {
	rel := gateStage3AnswerFile(surface)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return gateStage3Answer{}, fmt.Errorf("%w: surface %q was fanned out and %s does not exist: %w. The agent errored, the write failed, or the harness never wrote it; none of those is a surface that passed",
			errGateUncheckable, surface, rel, err)
	}
	var a gateStage3Answer
	if err := json.Unmarshal(raw, &a); err != nil {
		return gateStage3Answer{}, fmt.Errorf("%w: %s does not parse, so whatever is at that path is not an answer: %w", errGateUncheckable, rel, err)
	}
	return a, nil
}

// gateStage3ValidateAnswer is every refusal that applies to one answer,
// whether it arrived fresh from this run's fan-out or was carried from a
// previous one.
//
// IT IS ONE FUNCTION FOR BOTH ON PURPOSE. A carried answer that is held to a
// weaker standard than a fresh one is an answer that gets weaker every time it is
// carried, and the whole argument for carrying anything forward is that the
// carried answer is the SAME answer.
//
// It names every problem at once rather than returning at the first, for
// gateIsGreen's reason: the reader is a human, and a gate that reveals its
// findings one re-run at a time is a gate people stop running.
//
// wantRun is "" for a carried answer, which legitimately carries the identifier
// of the run that produced it. The identifier is still required to be present and
// well-formed — an answer that names no run at all cannot be attributed to one,
// carried or not.
func gateStage3ValidateAnswer(a gateStage3Answer, surface, wantRun, wantFingerprint string, vocabulary []gateStage3Subject) error {
	rel := gateStage3AnswerFile(surface)
	var problems []string

	if !gateStage3RunID.MatchString(a.Run) {
		problems = append(problems, fmt.Sprintf("%s records run %q, which is not a minted run identifier; an answer that cannot name the fan-out that produced it is an answer found on disk, and found on disk is not produced", rel, a.Run))
	} else if wantRun != "" && a.Run != wantRun {
		return fmt.Errorf("%w: %s was produced by run %s and this run is %s. Its fingerprint may well match — most surfaces do not move between two runs over a tree that barely moved — and it is still an answer no agent gave this release, which is the case a hand-written or half-written file also presents as",
			errGateStage2NotProduced, rel, a.Run, wantRun)
	}

	if a.Surface != surface {
		problems = append(problems, fmt.Sprintf("%s answers for surface %q; the path says %q. Two files claiming one surface is two answers for it, and the gate cannot say which one covers it", rel, a.Surface, surface))
	}
	if a.Verdict != gateVerdictPass && a.Verdict != gateVerdictFailed {
		problems = append(problems, fmt.Sprintf("%s holds verdict %q; there are exactly two verdicts and nothing that means \"we did not check\"", rel, a.Verdict))
	}
	switch {
	case a.Fingerprint == "":
		problems = append(problems, fmt.Sprintf("%s carries no fingerprint, so nothing attaches this answer to the tree being released", rel))
	case wantFingerprint == "":
		problems = append(problems, fmt.Sprintf("%s carries a fingerprint and no key was computed for surface %q on this tree, so the two cannot be compared at all", rel, surface))
	case a.Fingerprint != wantFingerprint:
		problems = append(problems, fmt.Sprintf("%s was produced over fingerprint %s and this tree fingerprints surface %q as %s; the agent answered about different inputs", rel, a.Fingerprint, surface, wantFingerprint))
	}

	switch {
	case a.Findings == nil:
		problems = append(problems, fmt.Sprintf("%s has no `findings` key at all. An absent list is not an empty one: it says the producer never recorded what the agent reported, and a truncated write stops in exactly this shape", rel))
	case a.Verdict == gateVerdictPass && len(*a.Findings) > 0:
		problems = append(problems, fmt.Sprintf("%s holds a PASS and %d finding(s). A PASS says nothing demonstrable was found while the answer lists what was found; whichever half is true, the other one reaches the human as a lie", rel, len(*a.Findings)))
	case a.Verdict == gateVerdictFailed && len(*a.Findings) == 0:
		problems = append(problems, fmt.Sprintf("%s holds a FAILED and no findings. It blocks the release without stating what is wrong with it, and the only recovery a human has is to re-run the surface — paying for the agent the cache exists to avoid, for exactly the surfaces that found something", rel))
	}
	if a.Findings != nil {
		for i, f := range *a.Findings {
			switch {
			case f.Surface != surface:
				problems = append(problems, fmt.Sprintf("%s finding %d is attributed to surface %q; a finding filed under a surface that did not raise it is a finding the human cannot trace and a surface that looks worse than it is", rel, i, f.Surface))
			case strings.TrimSpace(f.Rule) == "" || strings.TrimSpace(f.Detail) == "":
				problems = append(problems, fmt.Sprintf("%s finding %d names no rule or carries no detail; it reaches the report as an unactionable line and the release is blocked by it anyway", rel, i))
			}
		}
	}

	problems = append(problems, gateStage3SubjectProblems(rel, a.Subjects, vocabulary)...)

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%w: %s", errGateUncheckable, strings.Join(problems, "\n  "))
	}
	return nil
}

// gateStage3SubjectProblems is the totality rule over the vocabulary: every
// subject stated, no subject invented, every value in the declared form.
//
// AN OMISSION IS A REFUSAL, NOT A DEFAULT. Defaulting a missing subject to
// not-claimed is the one-line change that makes this whole file green over a
// fan-out that answered nothing: every agent omits every subject, every answer
// validates, the join groups thirteen silences and reports no collision, and the
// report reads exactly like a run that looked.
func gateStage3SubjectProblems(rel string, stated map[string]string, vocabulary []gateStage3Subject) []string {
	var problems []string
	if len(vocabulary) == 0 {
		return []string{fmt.Sprintf("%s was validated against an empty subject vocabulary, so no subject was required of it and the cross-surface join has nothing to compare", rel)}
	}
	declared := make(map[string]bool, len(vocabulary))
	for _, subject := range vocabulary {
		declared[subject.ID] = true
		value, ok := stated[subject.ID]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s states no value for subject %q. An omission and a deliberate %q are the same bytes to every later reader, and one of them means the subject was never looked at", rel, subject.ID, gateStage3NotClaimed))
		case value == gateStage3NotClaimed:
			// A deliberate silence. It is an answer, and it joins with nothing.
		case !subject.Match.MatchString(value):
			problems = append(problems, fmt.Sprintf("%s states subject %q as %q, which is not the declared form %q. A value in prose groups with nothing, so the disagreement this subject exists to expose would be reported by no one", rel, subject.ID, value, subject.Match.String()))
		}
	}
	for id := range stated {
		if !declared[id] {
			problems = append(problems, fmt.Sprintf("%s states subject %q, which the vocabulary in %s does not declare. A subject one agent invents is a group of one on every run", rel, id, gateBundleFrameFile))
		}
	}
	return problems
}

// ---------------------------------------------------------------------
// collecting the run: fresh answers, carried answers, total coverage
// ---------------------------------------------------------------------

// gateStage3Inputs is everything the collection needs. It is a struct rather
// than eight parameters because six of them are strings and slices of strings,
// and a call site that transposes two of those compiles.
type gateStage3Inputs struct {
	Root string
	// Run is this fan-out's minted identifier — see gateStage3Answer.Run.
	Run string
	// Declared is the manifest's fan-out, read from surfaces.yaml. It is never a
	// count and never a literal: the day a fourteenth surface is declared, a
	// hard-coded thirteen describes a run shape the manifest no longer has.
	Declared []string
	// Current is stage 2's key per declared surface. Stage 3 asks stage 2 for it
	// and never recomputes one.
	Current map[string]string
	// Plan is gatePlanRerun's decision: what this run paid for and what it kept.
	Plan gateRerunPlan
	// Previous is the previous run's WHOLE answers, not its verdicts. This is the
	// carrier gateRerunPlan does not have: gateRerunPlan.Carried is a
	// []gateSurfaceVerdict, three strings, with no room for the findings that
	// justified a carried FAILED or the subject values the join is computed over.
	// Keying this store on the same fingerprint gatePlanRerun compares is the
	// workaround for that, and it is deliberate — a second re-run planner in this
	// package would be the two-procedures defect CLAUDE.md names.
	Previous   []gateStage3Answer
	Vocabulary []gateStage3Subject
}

// gateStage3Collect turns one run's fan-out into exactly one answer per declared
// surface, or refuses the run.
//
// It never returns a partial record. Twelve answers and an error would be twelve
// surfaces a caller could plausibly report on, and the thirteenth would reach the
// human only if every caller remembered to read the error — the reading CLAUDE.md
// forbids, arrived at by an omission rather than a decision.
//
// It does NOT restate anything gateIsGreen, gateIndexVerdicts or gatePlanRerun
// already refuse. Its job is to hand them a record whose shape those refusals can
// judge: they answer "is this run green", and a second answer to that question in
// this package would be the two-procedures defect one directory over.
func gateStage3Collect(in gateStage3Inputs) ([]gateStage3Answer, error) {
	if len(in.Declared) == 0 {
		return nil, fmt.Errorf("%w: no surfaces are declared, so a record over zero answers would state a verdict over zero assertions", errGateUncheckable)
	}
	if len(in.Vocabulary) == 0 {
		return nil, fmt.Errorf("%w: the subject vocabulary is empty. Every answer would validate, the join would group nothing, and the run would report no collision on every tree forever", errGateUncheckable)
	}
	if !gateStage3RunID.MatchString(in.Run) {
		return nil, fmt.Errorf("%w: this run has no minted identifier (%q), so no answer on disk could be attributed to it and every leftover file from the previous run would be accepted as fresh", errGateUncheckable, in.Run)
	}

	previous := map[string]gateStage3Answer{}
	var duplicated []string
	for _, a := range in.Previous {
		if _, seen := previous[a.Surface]; seen {
			duplicated = append(duplicated, a.Surface)
			continue
		}
		previous[a.Surface] = a
	}
	if len(duplicated) > 0 {
		sort.Strings(duplicated)
		return nil, fmt.Errorf("%w: the previous run's record holds more than one answer for %s; carrying one of them forward would drop the other, and a dropped FAILED is how a re-run inherits a pass nobody gave it",
			errGateDuplicateVerdict, strings.Join(duplicated, ", "))
	}

	collected := map[string]gateStage3Answer{}
	var problems []error

	rerun := append([]string(nil), in.Plan.Rerun...)
	sort.Strings(rerun)
	for _, surface := range rerun {
		a, err := gateStage3LoadAnswer(in.Root, surface)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if err := gateStage3ValidateAnswer(a, surface, in.Run, in.Current[surface], in.Vocabulary); err != nil {
			problems = append(problems, err)
			continue
		}
		collected[surface] = a
	}

	for _, v := range in.Plan.Carried {
		a, ok := previous[v.Surface]
		if !ok {
			problems = append(problems, fmt.Errorf("%w: the plan carries %s=%s and the previous record holds no ANSWER for it, only the verdict. The findings that justified it and the subjects it placed are gone, so this run would report a blocked surface with nothing attached and a join computed over a hole",
				errGateUncheckable, v.Surface, v.Verdict))
			continue
		}
		if a.Verdict != v.Verdict {
			problems = append(problems, fmt.Errorf("%w: the plan carries %s=%s and the previous answer for it reads %s; two records of one answer disagree and neither is evidence of anything",
				errGateUncheckable, v.Surface, v.Verdict, a.Verdict))
			continue
		}
		if err := gateStage3ValidateAnswer(a, v.Surface, "", in.Current[v.Surface], in.Vocabulary); err != nil {
			problems = append(problems, err)
			continue
		}
		if _, fresh := collected[v.Surface]; fresh {
			problems = append(problems, fmt.Errorf("%w: surface %q is both re-run and carried in one plan; the run's shape is not what the manifest describes and resolving it by position would let one of the two answers vanish",
				errGateUncheckable, v.Surface))
			continue
		}
		collected[v.Surface] = a
	}

	// TOTAL COVERAGE, asked of the MANIFEST rather than of what was collected.
	// Iterating the collected answers and reporting what they cover is the shape
	// that reports twelve surfaces and states a verdict as though it covered
	// thirteen.
	for _, surface := range in.Declared {
		if _, ok := collected[surface]; !ok {
			problems = append(problems, fmt.Errorf("%w: surface %q is declared in %s and this run holds no answer for it — it was neither re-run nor carried, so the gate did not cover it",
				errGateUncheckable, surface, gateManifestFile))
		}
	}
	isDeclared := make(map[string]bool, len(in.Declared))
	for _, surface := range in.Declared {
		isDeclared[surface] = true
	}
	for surface := range collected {
		if !isDeclared[surface] {
			problems = append(problems, fmt.Errorf("%w: an answer was collected for surface %q, which %s does not declare; the manifest and the run disagree about the fan-out",
				errGateUncheckable, surface, gateManifestFile))
		}
	}

	problems = append(problems, gateStage3StrayAnswers(in.Root, in.Plan.Rerun)...)

	if len(problems) > 0 {
		// JOINED RATHER THAN CONCATENATED INTO ONE STRING. Every refusal above is
		// an instance of an existing sentinel — errGateUncheckable for "the check
		// could not be made", errGateStage2NotProduced for "found on disk is not
		// produced" — and flattening them into a message loses exactly that. A
		// caller that can no longer ask errors.Is which KIND of failure this was
		// is a caller that sends an operator to investigate the wrong thing, which
		// is the reason those two are separate sentinels in the first place.
		sort.Slice(problems, func(i, j int) bool { return problems[i].Error() < problems[j].Error() })
		return nil, errors.Join(append([]error{errors.New("this run cannot state its own coverage:")}, problems...)...)
	}

	out := make([]gateStage3Answer, 0, len(collected))
	for _, surface := range in.Declared {
		out = append(out, collected[surface])
	}
	return out, nil
}

// gateStage3StrayAnswers names every answer file on disk that this run did not
// ask for.
//
// FOUND ON DISK IS NOT PRODUCED, one stage further along than stage 2 can reach.
// gate/run.json is sealed before the fan-out, so no freshness check can cover an
// artifact that does not exist until after it; this is where that sentence gets
// enforced for answers. The concrete sequence is ordinary: the gate FAILS, fixes
// land, the driver re-runs, and the first run's answer files are still where the
// first run left them. Most of their fingerprints still match, because most
// surfaces genuinely did not move — so an implementation that reads whatever is
// in the directory and matches fingerprints carries them, and cannot tell that
// from an answer nobody produced.
func gateStage3StrayAnswers(root string, rerun []string) []error {
	dir := filepath.Join(root, filepath.FromSlash(gateStage3AnswerDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		// No directory at all is not a stray; if the run fanned anything out, the
		// missing answers are already reported one by one above.
		return nil
	}
	asked := make(map[string]bool, len(rerun))
	for _, surface := range rerun {
		asked[surface] = true
	}
	var problems []error
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		surface := strings.TrimSuffix(name, ".json")
		if asked[surface] {
			continue
		}
		problems = append(problems, fmt.Errorf("%w: %s is on disk and this run did not fan surface %q out. It is a leftover from a previous run, an answer for a surface the manifest does not declare, or a file someone wrote by hand; every one of those is an answer no agent gave this release",
			errGateStage2NotProduced, gateStage3AnswerFile(surface), surface))
	}
	return problems
}

// gateStage3Verdicts and gateStage3Findings are the two values gateRecordReceipt
// takes, projected out of the record.
//
// The projection is one-way and lossy by design — the receipt has room for three
// strings per surface and for findings, and for nothing else stage 3 derived.
// What matters is that it loses nothing it was HANDED: every collected answer
// contributes its verdict, and every finding of every collected answer is in the
// list, fresh or carried alike. Nothing is filtered, deduplicated or downgraded
// on the way, because a run over a finding nobody can see is a run that did not
// happen.
//
// AND NOTHING STAGE 3 DERIVED ITSELF IS LEFT BEHIND EITHER. The verdicts are a
// per-surface projection and have nowhere to carry a cross-surface result, so
// the join's collisions and its vocabulary-decay report travel out through the
// FINDINGS — see gateStage3JoinFindings. Before they did, both computations ran
// only inside their own fixture tests and a real disagreement reached no
// verdict, no report and no human.
func gateStage3Verdicts(answers []gateStage3Answer) []gateSurfaceVerdict {
	out := make([]gateSurfaceVerdict, 0, len(answers))
	for _, a := range answers {
		out = append(out, gateSurfaceVerdict{Surface: a.Surface, Verdict: a.Verdict, Fingerprint: a.Fingerprint})
	}
	return out
}

// gateStage3JoinSurface is the surface name every finding the JOIN raises is
// filed under.
//
// IT IS DELIBERATELY NOT A DECLARED SURFACE, and TestGateStage3TheJoinsFindings
// ReachTheReceipt asserts surfaces.yaml does not declare it. A collision belongs
// to no single surface by construction: each of the disagreeing agents is
// internally consistent, none of them was handed the others' documents, and the
// disagreement exists only in the union. Filing it under one of the four
// surfaces that stated a value would accuse whichever one the sort happened to
// reach first, and filing it under all of them would multiply one disagreement
// into four things a human has to read and reconcile.
const gateStage3JoinSurface = "cross-surface-join"

// The two rules the join reports under. They are named constants because the
// receipt sorts findings by rule and a human filters on it.
const (
	gateStage3RuleCollision = "cross-surface-disagreement"
	gateStage3RuleUnclaimed = "subject-claimed-by-nobody"
)

// gateStage3JoinFindings turns the join's two results into findings.
//
// WHY THIS EXISTS AT ALL. gateStage3Join and gateStage3UnclaimedSubjects were
// computed only inside their own tests: the fixture built four answers, called
// the join, asserted a collision came back, and threw it away. Nothing on the
// path from a fan-out to a receipt called either of them. So on the release the
// join was specified for — go.mod's floor moves, README and the CI template are
// updated, CONTRIBUTING and the site's badge are not — all thirteen agents
// PASS, the record is green to gateIsGreen, the receipt carries no finding,
// evaluate() returns PASS, and the driver publishes. The disagreement was
// computable at every moment of that run and reached no verdict, no report and
// no human.
//
// A COLLISION IS A FINDING AND NOT A VERDICT. It does not overrule any surface's
// answer — every one of them may be right about its own document — it says the
// union is inconsistent and names who said what, which is the only form a human
// can act on. That a finding blocks the release is gateReceipt.evaluate's rule
// and CLAUDE.md's: every finding reaches the human, and the human confirms what
// blocks.
//
// AN UNCLAIMED SUBJECT IS A FINDING FOR THE SAME REASON A SKIPPED CHECK IS A
// FAILURE. A subject every surface answers `not-claimed` is a subject the join
// CANNOT fire on: it reports agreement over that subject on this tree and on
// every future tree, and it looks identical to a subject that genuinely agrees.
// That is the "indistinguishable from a pass over zero assertions" shape, and
// this repository's rule is that it reaches the human rather than being logged
// where nobody reads it. It is a legitimate state of a tree — which is why the
// finding says so and leaves the call to the human — but it is never invisible.
func gateStage3JoinFindings(answers []gateStage3Answer, vocabulary []gateStage3Subject) ([]gateFinding, error) {
	collisions, err := gateStage3Join(answers, vocabulary)
	if err != nil {
		return nil, err
	}
	var out []gateFinding
	for _, c := range collisions {
		var said []string
		for _, v := range c.Values {
			said = append(said, fmt.Sprintf("%s say %q", strings.Join(v.Surfaces, ", "), v.Value))
		}
		out = append(out, gateFinding{
			Surface:  gateStage3JoinSurface,
			Rule:     gateStage3RuleCollision,
			Severity: "major",
			Detail: fmt.Sprintf("the surfaces do not agree about %s: %s. No per-surface agent can see this — each document is internally consistent and none of them was handed the others — so every one of these surfaces passed. Decide which value is right and correct the documents that state the other.",
				c.Subject, strings.Join(said, "; ")),
		})
	}
	for _, subject := range gateStage3UnclaimedSubjects(answers, vocabulary) {
		out = append(out, gateFinding{
			Surface:  gateStage3JoinSurface,
			Rule:     gateStage3RuleUnclaimed,
			Severity: "major",
			Detail: fmt.Sprintf("not one surface stated a value for %s, so the cross-surface join cannot fire on it: it reports agreement on this tree and on every future one, and that reads exactly like %d surfaces agreeing. Either a surface stopped stating it, or the subject no longer describes anything this project publishes and belongs out of the vocabulary in %s.",
				subject, len(answers), gateBundleFrameFile),
		})
	}
	return out, nil
}

// gateStage3Findings collects every finding the record carries, and then the
// join's own.
//
// The join's findings are appended HERE rather than left to the caller for the
// reason the whole lane exists: a projection the receipt takes is a thing that
// runs, and a computation beside it that the caller must remember to invoke is a
// computation that reaches no human the first time somebody forgets.
func gateStage3Findings(answers []gateStage3Answer, vocabulary []gateStage3Subject) ([]gateFinding, error) {
	var out []gateFinding
	for _, a := range answers {
		if a.Findings == nil {
			continue
		}
		out = append(out, *a.Findings...)
	}
	joined, err := gateStage3JoinFindings(answers, vocabulary)
	if err != nil {
		return nil, err
	}
	return append(out, joined...), nil
}

// gateStage3Fresh names the surfaces whose answers were produced by this run, as
// against the ones carried from a previous one.
//
// It exists so the split is a value stage 3 can report rather than a fact only
// the plan knew. It does not reach the receipt — gateSurfaceVerdict has no
// provenance field and that type is in another lane's file — so today a PASS
// carried across three releases still reads to the human like one produced
// minutes ago. That residue is named in the package comment above.
func gateStage3Fresh(in gateStage3Inputs, answers []gateStage3Answer) []string {
	asked := make(map[string]bool, len(in.Plan.Rerun))
	for _, surface := range in.Plan.Rerun {
		asked[surface] = true
	}
	var out []string
	for _, a := range answers {
		if asked[a.Surface] && a.Run == in.Run {
			out = append(out, a.Surface)
		}
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------
// the join
// ---------------------------------------------------------------------

// gateStage3SubjectValue is one value of one subject and every surface that
// stated it.
type gateStage3SubjectValue struct {
	Value    string
	Surfaces []string
}

// gateStage3Collision is one subject two or more surfaces answered differently.
//
// It is not a finding and does not carry a verdict. It is the one thing no
// per-surface agent can see: each of them is internally consistent, none of them
// is handed the others' documents, and the disagreement exists only in the union.
type gateStage3Collision struct {
	Subject string
	Values  []gateStage3SubjectValue
}

// gateStage3Join groups the record's subject values and reports every subject
// the surfaces do not agree on.
//
// THE FAILURE THIS EXISTS FOR IS LIVE ON THIS TREE. The module's Go floor is
// stated in README.md, in CONTRIBUTING.md, in the CI merge-gate template clients
// copy into their own repositories, and as a badge on the site — four declared
// surfaces, four documents, one number. They agree today. On the release that
// bumps go.mod and updates one of the four, every per-surface agent still passes:
// each document is internally consistent, none of them is handed go.mod (it is
// claimed by an out_of_scope entry and is in no surface's document set), and
// surface.json has no field for a toolchain floor, so gate/delta.json cannot
// carry it either. This group-by is the only reader that sees all four.
//
// It is computed over the WHOLE record — fresh and carried answers alike —
// because a collision between a carried surface and a fresh one is exactly the
// case a re-run would otherwise lose. That is what the carried answers being
// whole answers buys.
func gateStage3Join(answers []gateStage3Answer, vocabulary []gateStage3Subject) ([]gateStage3Collision, error) {
	if len(vocabulary) == 0 {
		return nil, fmt.Errorf("%w: a join over an empty vocabulary groups nothing and returns no collision on every tree, which is a check whose condition never holds", errGateUncheckable)
	}
	if len(answers) == 0 {
		return nil, fmt.Errorf("%w: a join over zero answers reports no collision, which is indistinguishable from thirteen surfaces that agree", errGateUncheckable)
	}

	var out []gateStage3Collision
	for _, subject := range vocabulary {
		byValue := map[string][]string{}
		for _, a := range answers {
			value, ok := a.Subjects[subject.ID]
			if !ok || value == gateStage3NotClaimed {
				continue
			}
			byValue[value] = append(byValue[value], a.Surface)
		}
		if len(byValue) < 2 {
			continue
		}
		values := make([]gateStage3SubjectValue, 0, len(byValue))
		for value, surfaces := range byValue {
			sort.Strings(surfaces)
			values = append(values, gateStage3SubjectValue{Value: value, Surfaces: surfaces})
		}
		sort.Slice(values, func(i, j int) bool { return values[i].Value < values[j].Value })
		out = append(out, gateStage3Collision{Subject: subject.ID, Values: values})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Subject < out[j].Subject })
	return out, nil
}

// gateStage3UnclaimedSubjects names every subject that not one surface claimed.
//
// It is REPORTED rather than refused: the run still collects, the record is
// still whole, and no surface's answer is overruled — because a subject nobody
// speaks to is a legitimate state of a tree and refusing it outright would be a
// check that fires on an honest release. What it must never be is invisible: a
// subject every surface answers `not-claimed` is a subject the join CANNOT fire
// on, and a vocabulary quietly decaying into subjects like that is how this
// whole stage becomes a group-by that reports green forever while looking busy.
//
// So the report leaves here as a FINDING (gateStage3JoinFindings), which reaches
// the receipt and therefore the human, who decides whether it blocks. That is
// the difference between reported and refused: an agent does not make the call,
// and it does not quietly make it by saying nothing either.
func gateStage3UnclaimedSubjects(answers []gateStage3Answer, vocabulary []gateStage3Subject) []string {
	var out []string
	for _, subject := range vocabulary {
		claimed := false
		for _, a := range answers {
			if value, ok := a.Subjects[subject.ID]; ok && value != gateStage3NotClaimed {
				claimed = true
				break
			}
		}
		if !claimed {
			out = append(out, subject.ID)
		}
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------
// test fixtures
// ---------------------------------------------------------------------

// gateStage3FixtureDir is the repo-root testdata directory holding the malformed
// answers. It is repo-root testdata/ rather than cmd/dossierx/testdata/ because
// surfaces.yaml's patterns are root-anchored: the idiomatic Go location would put
// gate fixtures under the binary-and-viewer surface and declare them part of a
// client-facing surface.
const gateStage3FixtureDir = "../../testdata/gate-stage3"

// The placeholders every fixture uses for the two values a test must supply, so
// that a fixture meant to break ONE thing does not silently start from a base
// that broke two.
const (
	gateStage3FixtureRunMarker         = "<<RUN>>"
	gateStage3FixtureFingerprintMarker = "<<FINGERPRINT>>"
)

// gateStage3Fixture reads one fixture and substitutes this run's identifier and
// the surface's key for this tree.
func gateStage3Fixture(t *testing.T, name, run, fingerprint string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(gateStage3FixtureDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	body := strings.ReplaceAll(string(raw), gateStage3FixtureRunMarker, run)
	return strings.ReplaceAll(body, gateStage3FixtureFingerprintMarker, fingerprint)
}

// gateStage3Vocab is a two-subject vocabulary for the unit tests, in the same
// shape gateStage3ReadVocabulary returns from the real frame.
//
// The tests that assert COVERAGE run against the real manifest and the real
// frame; this is for the tests that move one field of one answer at a time,
// where a synthetic vocabulary keeps the fixture readable.
func gateStage3Vocab(t *testing.T) []gateStage3Subject {
	t.Helper()
	return []gateStage3Subject{
		{ID: "cli-operator", Match: regexp.MustCompile(`^(?:agent|human|either|ci)$`)},
		{ID: "go-toolchain-floor", Match: regexp.MustCompile(`^\d+\.\d+$`)},
	}
}

// gateStage3Findings builds a one-finding list attributed to surface.
func gateStage3OneFinding(surface string) *[]gateFinding {
	return &[]gateFinding{{
		Surface:  surface,
		Rule:     "counted-claim-mismatch",
		Severity: "major",
		Detail:   "the document says nineteen commands and the inventory holds twenty",
	}}
}

// gateStage3Good is a well-formed answer, which every test below then breaks in
// exactly one place.
func gateStage3Good(surface, run, fingerprint, verdict string) gateStage3Answer {
	a := gateStage3Answer{
		Run:         run,
		Surface:     surface,
		Verdict:     verdict,
		Fingerprint: fingerprint,
		Findings:    &[]gateFinding{},
		Subjects: map[string]string{
			"cli-operator":       "agent",
			"go-toolchain-floor": "1.26",
		},
	}
	if verdict == gateVerdictFailed {
		a.Findings = gateStage3OneFinding(surface)
	}
	return a
}

// gateStage3WriteAnswer writes an answer to the path the harness would write it
// to.
func gateStage3WriteAnswer(t *testing.T, root string, a gateStage3Answer) {
	t.Helper()
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatalf("marshal answer for %s: %v", a.Surface, err)
	}
	gateWrite(t, root, gateStage3AnswerFile(a.Surface), string(raw)+"\n")
}

// ---------------------------------------------------------------------
// the vocabulary is real, closed, and declared where the agents can see it
// ---------------------------------------------------------------------

// The join's entire capacity to fire rests on this. A vocabulary that is missing
// from the frame, or is declared only here, produces a group-by over free text —
// thirteen groups of one, no collision, green forever — and every other assertion
// in this file would still pass.
func TestGateStage3TheFrameDeclaresAClosedSubjectVocabulary(t *testing.T) {
	root := surfaceRepoRoot(t)
	vocabulary, err := gateStage3ReadVocabulary(root)
	if err != nil {
		t.Fatalf("the real frame does not declare a usable subject vocabulary: %v", err)
	}
	if len(vocabulary) < 2 {
		t.Fatalf("the vocabulary declares %d subject(s); a join needs at least two surfaces answering at least one shared question, and a vocabulary this small is one edit from being empty", len(vocabulary))
	}

	var floor *gateStage3Subject
	for i := range vocabulary {
		if vocabulary[i].ID == "go-toolchain-floor" {
			floor = &vocabulary[i]
		}
		if vocabulary[i].Match.MatchString(gateStage3NotClaimed) {
			t.Errorf("subject %q accepts %q as a stated value, so silence and an answer are the same token", vocabulary[i].ID, gateStage3NotClaimed)
		}
	}
	if floor == nil {
		t.Fatalf("the vocabulary declares no go-toolchain-floor subject; it is the subject the join was specified against, and it is stated by four declared surfaces on this tree while surface.json has no field for it")
	}
	// The renderings that are live on this tree today must normalize to the same
	// token, or four agreeing surfaces produce four values and a false collision.
	if !floor.Match.MatchString("1.26") {
		t.Errorf("go-toolchain-floor rejects %q, which is the value every one of README.md, CONTRIBUTING.md, scripts/ci/dossierx-check.yml and site/src/content.ts states today", "1.26")
	}
	for _, prose := range []string{"Go 1.26+", "Go **1.26** or newer", "1.26.x", "a Go 1.26+ toolchain"} {
		if floor.Match.MatchString(prose) {
			t.Errorf("go-toolchain-floor accepts %q; a value carried as prose groups with nothing and the join is back to producing singletons", prose)
		}
	}
}

// A vocabulary parsed out of prose has to refuse the ways prose goes wrong, or
// an edit to the frame silently shrinks what the join compares.
func TestGateStage3RefusesAnUnusableVocabulary(t *testing.T) {
	realFrame := gateReadRepoFile(t, surfaceRepoRoot(t), gateBundleFrameFile)

	cases := []struct {
		name  string
		frame string
		want  string
	}{
		{
			name:  "no section at all",
			frame: strings.Replace(realFrame, gateStage3SubjectsHeading, "## Something else entirely", 1),
			want:  "carries no",
		},
		{
			name:  "the section lists nothing",
			frame: gateStage3SubjectsHeading + "\n\nThere are no subjects.\n\n## The material\n\n<<PARTS>>\n<<SURFACE>>\n",
			want:  "lists no subject",
		},
		{
			name:  "a subject with no value form",
			frame: gateStage3SubjectsHeading + "\n\n- `go-toolchain-floor` — the oldest Go toolchain.\n\n## x\n",
			want:  "states no `Match:` pattern",
		},
		{
			name:  "an unanchored value form",
			frame: gateStage3SubjectsHeading + "\n\n- `go-toolchain-floor` — x. Match: `[0-9]+\\.[0-9]+`\n\n## x\n",
			want:  "is not anchored",
		},
		{
			name:  "a value form that does not compile",
			frame: gateStage3SubjectsHeading + "\n\n- `go-toolchain-floor` — x. Match: `^[0-9$`\n\n## x\n",
			want:  "does not compile",
		},
		{
			name:  "a value form that also matches the silence token",
			frame: gateStage3SubjectsHeading + "\n\n- `go-toolchain-floor` — x. Match: `^.*$`\n\n## x\n",
			want:  "deliberate silence and a stated value are the same token",
		},
		{
			name:  "one subject declared twice",
			frame: gateStage3SubjectsHeading + "\n\n- `go-toolchain-floor` — x. Match: `^[0-9]+\\.[0-9]+$`\n- `go-toolchain-floor` — y. Match: `^[a-z]+$`\n\n## x\n",
			want:  "declared twice",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			gateWrite(t, root, gateBundleFrameFile, tc.frame)
			_, err := gateStage3ReadVocabulary(root)
			if err == nil {
				t.Fatalf("the vocabulary parsed; a join built on it would group nothing and report green forever")
			}
			if !errors.Is(err, errGateUncheckable) {
				t.Errorf("error is not errGateUncheckable: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not name the defect %q:\n%v", tc.want, err)
			}
		})
	}
}

// ---------------------------------------------------------------------
// the answer record and its refusals
// ---------------------------------------------------------------------

// Every fixture here PARSES. An answer that fails to parse at all is the easy
// case and proves the least; these are the shapes a hand-written answer, an
// interrupted write, or an obliging producer actually produces.
func TestGateStage3RefusesEveryMalformedAnswer(t *testing.T) {
	const (
		surface = "changelog"
		run     = "0123456789abcdef0123456789abcdef"
		key     = "sha256:aaaa"
	)
	vocabulary := gateStage3Vocab(t)

	cases := []struct {
		name        string
		fixture     string
		want        string
		notProduced bool
	}{
		{
			name:    "a FAILED whose findings never arrived",
			fixture: "failed-with-no-findings.json",
			want:    "blocks the release without stating what is wrong",
		},
		{
			name:    "a PASS that lists what it found",
			fixture: "pass-with-findings.json",
			want:    "the other one reaches the human as a lie",
		},
		{
			name:    "no findings key at all",
			fixture: "no-findings-key.json",
			want:    "An absent list is not an empty one",
		},
		{
			name:    "a fingerprint from the previous tree",
			fixture: "stale-fingerprint.json",
			want:    "the agent answered about different inputs",
		},
		{
			name:    "an answer for another surface, under this surface's path",
			fixture: "doubled-surface.json",
			want:    "Two files claiming one surface is two answers for it",
		},
		{
			name:    "a third verdict",
			fixture: "unknown-verdict.json",
			want:    "there are exactly two verdicts",
		},
		{
			name:    "a subject of the vocabulary left out",
			fixture: "omits-a-subject.json",
			want:    "states no value for subject \"go-toolchain-floor\"",
		},
		{
			name:    "a subject value in prose",
			fixture: "subject-value-in-prose.json",
			want:    "which is not the declared form",
		},
		{
			name:    "a subject the vocabulary does not declare",
			fixture: "invented-subject.json",
			want:    "does not declare",
		},
		{
			name:    "a finding attributed to a different surface",
			fixture: "finding-for-another-surface.json",
			want:    "a finding filed under a surface that did not raise it",
		},
		{
			name:        "an answer left behind by the previous run",
			fixture:     "previous-run.json",
			want:        "was produced by run",
			notProduced: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			gateWrite(t, root, gateStage3AnswerFile(surface), gateStage3Fixture(t, tc.fixture, run, key))
			a, err := gateStage3LoadAnswer(root, surface)
			if err != nil {
				t.Fatalf("the fixture must PARSE — an unparseable answer is the easy case: %v", err)
			}
			err = gateStage3ValidateAnswer(a, surface, run, key, vocabulary)
			if err == nil {
				t.Fatalf("the answer was accepted; this surface would report to the human as though an agent had answered it")
			}
			want := errGateUncheckable
			if tc.notProduced {
				want = errGateStage2NotProduced
			}
			if !errors.Is(err, want) {
				t.Errorf("error is not %v: %v", want, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not name the defect %q:\n%v", tc.want, err)
			}
		})
	}
}

// The base every case above breaks must itself be accepted, or each of those
// tests would pass over a fixture that was already wrong for some other reason.
func TestGateStage3AcceptsAWellFormedAnswer(t *testing.T) {
	const (
		surface = "changelog"
		run     = "0123456789abcdef0123456789abcdef"
		key     = "sha256:aaaa"
	)
	root := t.TempDir()
	gateWrite(t, root, gateStage3AnswerFile(surface), gateStage3Fixture(t, "well-formed.json", run, key))
	a, err := gateStage3LoadAnswer(root, surface)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := gateStage3ValidateAnswer(a, surface, run, key, gateStage3Vocab(t)); err != nil {
		t.Fatalf("the well-formed fixture was refused, so every refusal test above proves nothing: %v", err)
	}
	if a.Findings == nil || len(*a.Findings) != 1 {
		t.Fatalf("the well-formed fixture is meant to carry exactly one finding; it carries %v", a.Findings)
	}
}

// A surface that was fanned out and produced no file at all is the plainest way
// the record narrows, and the one an orchestrating agent is most tempted to
// patch by hand.
func TestGateStage3AMissingAnswerIsAFailedRunNamingTheSurface(t *testing.T) {
	root := t.TempDir()
	_, err := gateStage3LoadAnswer(root, "site")
	if err == nil {
		t.Fatalf("a surface with no answer file was accepted")
	}
	if !errors.Is(err, errGateUncheckable) {
		t.Errorf("error is not errGateUncheckable: %v", err)
	}
	for _, want := range []string{"site", "none of those is a surface that passed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not say %q:\n%v", want, err)
		}
	}
}

// ---------------------------------------------------------------------
// coverage, against the real manifest
// ---------------------------------------------------------------------

// gateStage3RealRun builds a run over the REAL repository: the real thirteen,
// real keys computed by stage 2, and one well-formed answer per surface.
//
// It is the real manifest rather than a synthetic fixture for the reason the
// stage-2 header gives: a fixture holds thirteen well-formed answers by
// construction, and it is exactly that shape that let the landed code ship with a
// surface whose key could not be computed.
func gateStage3RealRun(t *testing.T) (root, run string, declared []string, current map[string]string, vocabulary []gateStage3Subject) {
	t.Helper()
	overlay, realRoot := gateStage2Overlay(t)

	declared, err := gateDeclaredSurfaces(overlay)
	if err != nil {
		t.Fatalf("read the declared surfaces: %v", err)
	}
	tracked := surfaceTrackedFiles(t, realRoot)
	gateStage2StampRun(t, overlay, gateStage2FixtureTree, declared)
	current, err = gateStage2Plan(overlay, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("stage 2 could not plan this run, so stage 3 has no keys to hold answers to: %v", err)
	}
	if len(current) != len(declared) {
		t.Fatalf("stage 2 produced %d keys for %d declared surfaces", len(current), len(declared))
	}
	vocabulary, err = gateStage3ReadVocabulary(overlay)
	if err != nil {
		t.Fatalf("read the vocabulary from the real frame: %v", err)
	}
	run, err = gateStage3MintRun()
	if err != nil {
		t.Fatalf("mint a run identifier: %v", err)
	}
	return overlay, run, declared, current, vocabulary
}

// gateStage3AnswerEvery writes one well-formed answer per surface, over the real
// vocabulary.
func gateStage3AnswerEvery(t *testing.T, root, run string, declared []string, current map[string]string, vocabulary []gateStage3Subject) {
	t.Helper()
	for _, surface := range declared {
		a := gateStage3Answer{
			Run:         run,
			Surface:     surface,
			Verdict:     gateVerdictPass,
			Fingerprint: current[surface],
			Findings:    &[]gateFinding{},
			Subjects:    map[string]string{},
		}
		for _, subject := range vocabulary {
			a.Subjects[subject.ID] = gateStage3NotClaimed
		}
		gateStage3WriteAnswer(t, root, a)
	}
}

// TOTAL COVERAGE. The record covers every surface surfaces.yaml declares, or the
// run is FAILED naming the ones it does not — never twelve answers under a
// verdict that reads as though it covered thirteen.
func TestGateStage3CoversEveryDeclaredSurfaceOfTheRealManifest(t *testing.T) {
	root, run, declared, current, vocabulary := gateStage3RealRun(t)
	gateStage3AnswerEvery(t, root, run, declared, current, vocabulary)

	plan := gateRerunPlan{Rerun: append([]string(nil), declared...)}
	in := gateStage3Inputs{Root: root, Run: run, Declared: declared, Current: current, Plan: plan, Vocabulary: vocabulary}

	answers, err := gateStage3Collect(in)
	if err != nil {
		t.Fatalf("a run holding a well-formed answer for every declared surface was refused: %v", err)
	}
	if len(answers) != len(declared) {
		t.Fatalf("collected %d answers for %d declared surfaces", len(answers), len(declared))
	}
	if fresh := gateStage3Fresh(in, answers); len(fresh) != len(declared) {
		t.Errorf("%d of %d answers are recorded as produced by this run; the rest would read to a human as fresh when they are not", len(fresh), len(declared))
	}
	// The verdicts stage 3 hands on must satisfy the landed greenness rule, which
	// stage 3 feeds and never restates.
	if err := gateIsGreen(declared, gateStage3Verdicts(answers), current); err != nil {
		t.Fatalf("the record stage 3 produced is not green to gateIsGreen: %v", err)
	}

	// EVERY ANSWER HERE IS `not-claimed` FOR EVERY SUBJECT, which is the shape a
	// fan-out that looked at nothing also produces: thirteen well-formed answers,
	// thirteen PASSes, a green record, and a join that can fire on nothing. It
	// must reach the human as findings rather than as a clean run.
	findings, err := gateStage3Findings(answers, vocabulary)
	if err != nil {
		t.Fatalf("project the findings: %v", err)
	}
	if len(findings) != len(vocabulary) {
		t.Errorf("a run in which not one of the %d real subjects was claimed by any of the %d surfaces produced %d findings; a vocabulary nobody answers reports agreement forever and reads exactly like agreement", len(vocabulary), len(declared), len(findings))
	}
	for _, f := range findings {
		if f.Rule != gateStage3RuleUnclaimed {
			t.Errorf("an all-silent run raised a %q finding: %s", f.Rule, f.Detail)
		}
	}

	// A PLAN THAT COVERS TWELVE. This is the shape the coverage rule exists for
	// and the one the missing-file case does NOT exercise: nothing failed to
	// load, nothing was malformed, and every answer the run asked for is present
	// and well-formed — the run simply never asked about the thirteenth surface.
	// Reading coverage off what was collected reports thirteen clean answers over
	// twelve surfaces; it has to be read off the manifest.
	unasked := declared[len(declared)-1]
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile(unasked)))); err != nil {
		t.Fatalf("remove %s: %v", unasked, err)
	}

	t.Run("a plan that never asks about one declared surface", func(t *testing.T) {
		narrow := in
		narrow.Plan = gateRerunPlan{Rerun: append([]string(nil), declared[:len(declared)-1]...)}
		_, err := gateStage3Collect(narrow)
		if err == nil {
			t.Fatalf("a plan covering %d of %d declared surfaces was collected without complaint; the record would state a verdict as though it covered all of them", len(declared)-1, len(declared))
		}
		if !strings.Contains(err.Error(), unasked) {
			t.Errorf("the refusal does not name the surface nobody asked about (%q):\n%v", unasked, err)
		}
	})

	// The other way the same surface goes missing: the run DID ask, and the agent
	// errored so nothing was written. Named too, rather than reported over twelve.
	t.Run("a surface that was asked and never answered", func(t *testing.T) {
		_, err := gateStage3Collect(in)
		if err == nil {
			t.Fatalf("a run missing surface %q's answer was collected anyway; the record would cover %d surfaces under a verdict stating it covered %d", unasked, len(declared)-1, len(declared))
		}
		if !strings.Contains(err.Error(), unasked) {
			t.Errorf("the refusal does not name %q:\n%v", unasked, err)
		}
	})
}

// FOUND ON DISK IS NOT PRODUCED, at the one point in the pipeline where stage
// 2's freshness check structurally cannot reach: gate/run.json is sealed before
// the fan-out, so no answer file is in the set it digests.
func TestGateStage3RefusesAnAnswerThisRunDidNotFanOut(t *testing.T) {
	root, run, declared, current, vocabulary := gateStage3RealRun(t)
	gateStage3AnswerEvery(t, root, run, declared, current, vocabulary)

	// The ordinary sequence: the gate FAILED, fixes landed, one surface moved and
	// is re-run, and every other surface's answer file is still where the previous
	// run left it — with a fingerprint that still matches, because those surfaces
	// genuinely did not move.
	moved := declared[0]
	plan := gateRerunPlan{Rerun: []string{moved}}
	for _, surface := range declared[1:] {
		plan.Carried = append(plan.Carried, gateSurfaceVerdict{Surface: surface, Verdict: gateVerdictPass, Fingerprint: current[surface]})
	}
	var previous []gateStage3Answer
	for _, surface := range declared[1:] {
		a, err := gateStage3LoadAnswer(root, surface)
		if err != nil {
			t.Fatalf("load %s: %v", surface, err)
		}
		previous = append(previous, a)
	}

	in := gateStage3Inputs{Root: root, Run: run, Declared: declared, Current: current, Plan: plan, Previous: previous, Vocabulary: vocabulary}
	_, err := gateStage3Collect(in)
	if err == nil {
		t.Fatalf("the leftover answer files were accepted; a hand-written, truncated or copied answer at one of those paths would be indistinguishable from an agent's")
	}
	stale := declared[1]
	if !strings.Contains(err.Error(), gateStage3AnswerFile(stale)) {
		t.Errorf("the refusal does not name the leftover %s:\n%v", gateStage3AnswerFile(stale), err)
	}

	// With the leftovers cleared, the same plan is accepted — so the refusal is
	// about the files on disk and not about the carry-forward itself.
	for _, surface := range declared[1:] {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile(surface)))); err != nil {
			t.Fatalf("remove %s: %v", surface, err)
		}
	}
	if _, err := gateStage3Collect(in); err != nil {
		t.Fatalf("the same plan with no leftovers on disk was refused: %v", err)
	}
}

// An answer whose run identifier is the PREVIOUS run's, at the path this run
// expects, with a fingerprint that matches this tree.
func TestGateStage3RefusesAnAnswerFromAPreviousRunAtTheRightPath(t *testing.T) {
	root, run, declared, current, vocabulary := gateStage3RealRun(t)
	before, err := gateStage3MintRun()
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	gateStage3AnswerEvery(t, root, before, declared, current, vocabulary)

	in := gateStage3Inputs{Root: root, Run: run, Declared: declared, Current: current,
		Plan: gateRerunPlan{Rerun: append([]string(nil), declared...)}, Vocabulary: vocabulary}
	_, err = gateStage3Collect(in)
	if err == nil {
		t.Fatalf("answers from run %s were collected as run %s's own; every fingerprint matches, so nothing else in the pipeline would notice", before, run)
	}
	if !errors.Is(err, errGateStage2NotProduced) {
		t.Errorf("the refusal is not an instance of errGateStage2NotProduced: %v", err)
	}
}

// ---------------------------------------------------------------------
// carry-forward as a whole answer
// ---------------------------------------------------------------------

// A carried FAILED must arrive with the findings that justified it. Carrying the
// verdict alone gives the human a blocked release and no statement of what is
// wrong with it, and the recovery a person actually takes then is to re-run the
// surface — paying for the agent the cache exists to avoid, every round, for
// exactly the surfaces that found something.
func TestGateStage3CarriesTheWholeAnswerOrNothing(t *testing.T) {
	const (
		run  = "0123456789abcdef0123456789abcdef"
		prev = "fedcba9876543210fedcba9876543210"
	)
	declared := []string{"changelog", "site"}
	current := map[string]string{"changelog": "sha256:c", "site": "sha256:s"}
	vocabulary := gateStage3Vocab(t)

	// Run 1: both FAILED, each with a finding. Run 2: changelog's documents moved
	// so it is re-run; site's did not, so it is carried.
	previousSite := gateStage3Good("site", prev, current["site"], gateVerdictFailed)
	plan := gateRerunPlan{
		Rerun:   []string{"changelog"},
		Carried: []gateSurfaceVerdict{{Surface: "site", Verdict: gateVerdictFailed, Fingerprint: current["site"]}},
	}

	root := t.TempDir()
	gateStage3WriteAnswer(t, root, gateStage3Good("changelog", run, current["changelog"], gateVerdictFailed))

	in := gateStage3Inputs{Root: root, Run: run, Declared: declared, Current: current, Plan: plan,
		Previous: []gateStage3Answer{previousSite}, Vocabulary: vocabulary}

	answers, err := gateStage3Collect(in)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	findings, err := gateStage3Findings(answers, vocabulary)
	if err != nil {
		t.Fatalf("project the findings: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("the record holds %d findings; run 1 raised one per surface and a re-run must lose neither: %+v", len(findings), findings)
	}
	var sawSite bool
	for _, f := range findings {
		if f.Surface == "site" {
			sawSite = true
		}
	}
	if !sawSite {
		t.Errorf("site was carried as FAILED and its finding is not in the record; the human has a blocked release and no statement of what is wrong with it")
	}
	// The subject values travel too, or the join in run 2 is computed over a hole
	// exactly where run 1 found something.
	for _, a := range answers {
		if a.Surface != "site" {
			continue
		}
		if len(a.Subjects) != len(vocabulary) {
			t.Errorf("the carried answer states %d of %d subjects; a collision between a carried surface and a fresh one would be gone in run 2", len(a.Subjects), len(vocabulary))
		}
	}
	if fresh := gateStage3Fresh(in, answers); len(fresh) != 1 || fresh[0] != "changelog" {
		t.Errorf("the fresh/carried split reads %v; only changelog was paid for this run", fresh)
	}

	// The verdict-only carrier — which is exactly what gateRerunPlan.Carried is —
	// must not be enough on its own.
	t.Run("a carried verdict with no answer behind it", func(t *testing.T) {
		bare := in
		bare.Previous = nil
		_, err := gateStage3Collect(bare)
		if err == nil {
			t.Fatalf("site was carried with no answer behind it; the record would say site=FAILED with nothing attached")
		}
		if !strings.Contains(err.Error(), "only the verdict") {
			t.Errorf("the refusal does not name what was lost:\n%v", err)
		}
	})

	// A carried answer is held to the same standard as a fresh one, or it gets
	// weaker every time it is carried.
	t.Run("a carried answer that is itself malformed", func(t *testing.T) {
		weak := in
		empty := []gateFinding{}
		thin := previousSite
		thin.Findings = &empty
		weak.Previous = []gateStage3Answer{thin}
		_, err := gateStage3Collect(weak)
		if err == nil {
			t.Fatalf("a carried FAILED with its findings stripped was accepted; that is a report that gets thinner every time it is re-run")
		}
		if !strings.Contains(err.Error(), "blocks the release without stating what is wrong") {
			t.Errorf("the refusal does not name the defect:\n%v", err)
		}
	})

	// A carried answer whose verdict disagrees with the plan's copy of it.
	t.Run("the plan and the answer disagree", func(t *testing.T) {
		split := in
		flipped := previousSite
		flipped.Verdict = gateVerdictPass
		flipped.Findings = &[]gateFinding{}
		split.Previous = []gateStage3Answer{flipped}
		_, err := gateStage3Collect(split)
		if err == nil {
			t.Fatalf("the plan carried site=FAILED and the answer read PASS, and both were accepted")
		}
		if !strings.Contains(err.Error(), "neither is evidence of anything") {
			t.Errorf("the refusal does not name the disagreement:\n%v", err)
		}
	})

	// Two answers for one surface in the previous record.
	t.Run("the previous record holds a surface twice", func(t *testing.T) {
		doubled := in
		doubled.Previous = []gateStage3Answer{previousSite, previousSite}
		_, err := gateStage3Collect(doubled)
		if err == nil {
			t.Fatalf("a previous record holding site twice was accepted")
		}
		if !errors.Is(err, errGateDuplicateVerdict) {
			t.Errorf("the refusal is not an instance of errGateDuplicateVerdict: %v", err)
		}
	})
}

// Every finding of every collected answer reaches the receipt's list, unfiltered.
func TestGateStage3HandsTheReceiptEveryFindingItCollected(t *testing.T) {
	const run = "0123456789abcdef0123456789abcdef"
	declared := []string{"changelog", "readme", "site"}
	current := map[string]string{"changelog": "sha256:c", "readme": "sha256:r", "site": "sha256:s"}

	root := t.TempDir()
	var want int
	for _, surface := range declared {
		a := gateStage3Good(surface, run, current[surface], gateVerdictFailed)
		extra := append(*a.Findings, gateFinding{Surface: surface, Rule: "stale-version-pin", Severity: "minor", Detail: "the install line pins the previous release"})
		a.Findings = &extra
		want += len(extra)
		gateStage3WriteAnswer(t, root, a)
	}

	vocabulary := gateStage3Vocab(t)
	answers, err := gateStage3Collect(gateStage3Inputs{
		Root: root, Run: run, Declared: declared, Current: current,
		Plan:       gateRerunPlan{Rerun: append([]string(nil), declared...)},
		Vocabulary: vocabulary,
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	projected, err := gateStage3Findings(answers, vocabulary)
	if err != nil {
		t.Fatalf("project the findings: %v", err)
	}
	// Every surface states the same two subject values, so the join raises
	// nothing here and the count is the record's own findings exactly.
	if got := len(projected); got != want {
		t.Fatalf("%d of %d findings reached the record; a finding nobody can see is a run that did not happen", got, want)
	}
	// Including the ones an agent labelled minor: severity orders a report and
	// decides nothing, because the human confirms what blocks a release.
	var minor int
	for _, f := range projected {
		if f.Severity == "minor" {
			minor++
		}
	}
	if minor != len(declared) {
		t.Errorf("%d of %d self-labelled minor findings survived collection", minor, len(declared))
	}
}

// ---------------------------------------------------------------------
// the join
// ---------------------------------------------------------------------

// The failure the join exists for, built as it will actually arrive: go.mod's
// floor moves, README and the CI template are updated, CONTRIBUTING and the
// site's badge are not. Every per-surface agent PASSES — each document is
// internally consistent, none of them is handed go.mod, and surface.json has no
// field for a toolchain floor. The join is the only reader that sees all four.
func TestGateStage3TheJoinFiresOnASubjectNoSurfaceCanSeeAlone(t *testing.T) {
	const run = "0123456789abcdef0123456789abcdef"
	declared := []string{"ci-merge-gate-template", "contributing", "readme", "site"}
	current := map[string]string{}
	vocabulary := gateStage3Vocab(t)
	floors := map[string]string{
		"readme":                 "1.27",
		"ci-merge-gate-template": "1.27",
		"contributing":           "1.26",
		"site":                   "1.26",
	}

	root := t.TempDir()
	for _, surface := range declared {
		current[surface] = "sha256:" + surface
		a := gateStage3Good(surface, run, current[surface], gateVerdictPass)
		a.Subjects["go-toolchain-floor"] = floors[surface]
		gateStage3WriteAnswer(t, root, a)
	}
	in := gateStage3Inputs{Root: root, Run: run, Declared: declared, Current: current,
		Plan: gateRerunPlan{Rerun: append([]string(nil), declared...)}, Vocabulary: vocabulary}

	answers, err := gateStage3Collect(in)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	// Every surface passed, and the run is green to everything downstream of a
	// verdict. That is the point: nothing else in the pipeline can see this.
	if err := gateIsGreen(declared, gateStage3Verdicts(answers), current); err != nil {
		t.Fatalf("the fixture is not the case being described — some surface did not pass: %v", err)
	}

	collisions, err := gateStage3Join(answers, vocabulary)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if len(collisions) != 1 {
		t.Fatalf("the join reported %d collisions over four surfaces stating two different Go floors: %+v", len(collisions), collisions)
	}
	c := collisions[0]
	if c.Subject != "go-toolchain-floor" {
		t.Fatalf("the collision is on subject %q", c.Subject)
	}
	if len(c.Values) != 2 {
		t.Fatalf("the collision holds %d values: %+v", len(c.Values), c.Values)
	}
	if c.Values[0].Value != "1.26" || c.Values[1].Value != "1.27" {
		t.Errorf("the collision's values are %q and %q", c.Values[0].Value, c.Values[1].Value)
	}
	if strings.Join(c.Values[0].Surfaces, ",") != "contributing,site" {
		t.Errorf("1.26 is attributed to %v", c.Values[0].Surfaces)
	}
	if strings.Join(c.Values[1].Surfaces, ",") != "ci-merge-gate-template,readme" {
		t.Errorf("1.27 is attributed to %v", c.Values[1].Surfaces)
	}

	// AND IT LEAVES STAGE 3. Everything above was true before this lane and
	// reached nobody: the join ran here, in this test, and the value it returned
	// was asserted about and dropped. The projection the receipt takes is the only
	// way out of stage 3, so the collision has to be in it.
	findings, err := gateStage3Findings(answers, vocabulary)
	if err != nil {
		t.Fatalf("project the findings: %v", err)
	}
	var raised []gateFinding
	for _, f := range findings {
		if f.Rule == gateStage3RuleCollision {
			raised = append(raised, f)
		}
	}
	if len(raised) != 1 {
		t.Fatalf("the projection carries %d collision findings over four surfaces stating two Go floors; every surface PASSED, so this is the only thing that can reach the human: %+v", len(raised), findings)
	}
	for _, want := range []string{"go-toolchain-floor", "1.26", "1.27", "contributing", "readme"} {
		if !strings.Contains(raised[0].Detail, want) {
			t.Errorf("the finding does not mention %q, so the human is told there is a disagreement and not what it is or who is in it:\n%s", want, raised[0].Detail)
		}
	}
	if raised[0].Surface != gateStage3JoinSurface {
		t.Errorf("the collision is filed under surface %q; it belongs to none of the four, and filing it under one of them accuses whichever the sort reached first", raised[0].Surface)
	}

	// Four surfaces AGREEING — this tree today — produce no collision, or the
	// join is a check that fires on every release and is read by nobody.
	t.Run("agreement is not a collision", func(t *testing.T) {
		for i := range answers {
			answers[i].Subjects["go-toolchain-floor"] = "1.26"
		}
		collisions, err := gateStage3Join(answers, vocabulary)
		if err != nil {
			t.Fatalf("join: %v", err)
		}
		if len(collisions) != 0 {
			t.Fatalf("four agreeing surfaces produced %d collisions: %+v", len(collisions), collisions)
		}
	})

	// A deliberate silence joins with nothing, and is not a value that collides
	// with the surfaces that did speak.
	t.Run("silence is not a value", func(t *testing.T) {
		for i := range answers {
			answers[i].Subjects["go-toolchain-floor"] = "1.26"
			answers[i].Subjects["cli-operator"] = gateStage3NotClaimed
		}
		answers[0].Subjects["go-toolchain-floor"] = gateStage3NotClaimed
		collisions, err := gateStage3Join(answers, vocabulary)
		if err != nil {
			t.Fatalf("join: %v", err)
		}
		if len(collisions) != 0 {
			t.Fatalf("a not-claimed subject collided with the surfaces that stated one: %+v", collisions)
		}
		// A subject every surface answered `not-claimed` is a subject the join
		// CANNOT fire on. It is reported rather than refused — that is a real
		// state of a tree — but it must never be invisible, because a vocabulary
		// decaying into subjects like that is how this stage becomes a group-by
		// that reports green forever while looking busy.
		if unclaimed := gateStage3UnclaimedSubjects(answers, vocabulary); len(unclaimed) != 1 || unclaimed[0] != "cli-operator" {
			t.Errorf("the unclaimed subjects read %v; go-toolchain-floor is claimed by three surfaces here and cli-operator by none", unclaimed)
		}
		// And "never invisible" means it leaves stage 3, not that it is computed.
		findings, err := gateStage3Findings(answers, vocabulary)
		if err != nil {
			t.Fatalf("project the findings: %v", err)
		}
		var decay []gateFinding
		for _, f := range findings {
			if f.Rule == gateStage3RuleUnclaimed {
				decay = append(decay, f)
			}
		}
		if len(decay) != 1 || !strings.Contains(decay[0].Detail, "cli-operator") {
			t.Fatalf("a subject not one surface claimed produced %d findings; the join cannot fire on it, on this tree or any future one, and that is indistinguishable from four surfaces agreeing: %+v", len(decay), findings)
		}
	})
}

// TestGateStage3TheJoinsFindingsReachTheReceipt is the end of the path I7 is
// about: not that the join computes a collision, but that a human reading the
// release's own record is told about it.
//
// The run below is the specified failure in full. Two surfaces state two
// different Go floors; both agents PASS, because each document is internally
// consistent and neither was handed the other's; the record is green to
// gateIsGreen; and the receipt is written by the same call the driver makes.
// Before this lane, that receipt held no finding and evaluate() returned PASS —
// the release publishes, with the disagreement computable at every moment of the
// run and recorded nowhere.
func TestGateStage3TheJoinsFindingsReachTheReceipt(t *testing.T) {
	const run = "0123456789abcdef0123456789abcdef"
	declared := []string{"contributing", "readme"}
	current := map[string]string{"contributing": "sha256:c", "readme": "sha256:r"}
	vocabulary := gateStage3Vocab(t)

	root := t.TempDir()
	floors := map[string]string{"contributing": "1.26", "readme": "1.27"}
	for _, surface := range declared {
		a := gateStage3Good(surface, run, current[surface], gateVerdictPass)
		a.Subjects["go-toolchain-floor"] = floors[surface]
		gateStage3WriteAnswer(t, root, a)
	}

	in := gateStage3Inputs{Root: root, Run: run, Declared: declared, Current: current,
		Plan: gateRerunPlan{Rerun: append([]string(nil), declared...)}, Vocabulary: vocabulary}
	answers, err := gateStage3Collect(in)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	verdicts := gateStage3Verdicts(answers)
	if err := gateIsGreen(declared, verdicts, current); err != nil {
		t.Fatalf("the fixture is not the case being described — some surface did not pass: %v", err)
	}
	findings, err := gateStage3Findings(answers, vocabulary)
	if err != nil {
		t.Fatalf("project the findings: %v", err)
	}

	work := gateFixtureRepo(t)
	receipt, err := gateRecordReceipt(work, "v9.9.9", "release", verdicts, findings)
	if err != nil {
		t.Fatalf("record the receipt: %v", err)
	}
	var got *gateFinding
	for i := range receipt.Findings {
		if receipt.Findings[i].Rule == gateStage3RuleCollision {
			got = &receipt.Findings[i]
		}
	}
	if got == nil {
		t.Fatalf("the receipt carries no cross-surface finding over a run where two surfaces state two different Go floors and both PASSED; it is the only reader of all thirteen answers, and its verdict would be PASS: %+v", receipt.Findings)
	}
	if !strings.Contains(got.Detail, "go-toolchain-floor") {
		t.Errorf("the receipt's finding does not name the subject:\n%s", got.Detail)
	}

	// The verdict the driver recomputes, which is the thing that actually stops a
	// release. A finding in a receipt nobody evaluates would be a longer document
	// and the same published tag.
	verdict, err := receipt.evaluate(declared, current)
	if verdict != gateVerdictFailed {
		t.Fatalf("the recomputed verdict is %q over a receipt carrying a cross-surface disagreement; the driver publishes on PASS", verdict)
	}
	if err == nil || !strings.Contains(err.Error(), "finding(s) reached the report") {
		t.Errorf("the FAILED does not say why, so the human is stopped and not told what to look at; got %v", err)
	}

	// The join's own name is not a surface anybody declares. If it ever became
	// one, a real surface's findings and the join's would be indistinguishable in
	// the receipt, and gateIsGreen would start demanding a verdict for it.
	realDeclared, err := gateDeclaredSurfaces(surfaceRepoRoot(t))
	if err != nil {
		t.Fatalf("read the declared surfaces: %v", err)
	}
	for _, name := range realDeclared {
		if name == gateStage3JoinSurface {
			t.Errorf("%s declares a surface called %q, which is the name the join files its own findings under", gateManifestFile, name)
		}
	}
}

// A collision between a CARRIED surface and a FRESH one is the case a re-run
// would otherwise lose, and it is the reason carried answers have to be whole.
func TestGateStage3TheJoinSeesCarriedAnswersToo(t *testing.T) {
	const (
		run  = "0123456789abcdef0123456789abcdef"
		prev = "fedcba9876543210fedcba9876543210"
	)
	declared := []string{"contributing", "readme"}
	current := map[string]string{"contributing": "sha256:c", "readme": "sha256:r"}
	vocabulary := gateStage3Vocab(t)

	fresh := gateStage3Good("readme", run, current["readme"], gateVerdictPass)
	fresh.Subjects["go-toolchain-floor"] = "1.27"
	carried := gateStage3Good("contributing", prev, current["contributing"], gateVerdictPass)
	carried.Subjects["go-toolchain-floor"] = "1.26"

	root := t.TempDir()
	gateStage3WriteAnswer(t, root, fresh)

	answers, err := gateStage3Collect(gateStage3Inputs{
		Root: root, Run: run, Declared: declared, Current: current,
		Plan: gateRerunPlan{
			Rerun:   []string{"readme"},
			Carried: []gateSurfaceVerdict{{Surface: "contributing", Verdict: gateVerdictPass, Fingerprint: current["contributing"]}},
		},
		Previous:   []gateStage3Answer{carried},
		Vocabulary: vocabulary,
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	collisions, err := gateStage3Join(answers, vocabulary)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if len(collisions) != 1 || collisions[0].Subject != "go-toolchain-floor" {
		t.Fatalf("the join over one fresh and one carried answer reported %+v; README moved to 1.27 and the carried CONTRIBUTING still says 1.26", collisions)
	}
}

// A join over an empty vocabulary, or over no answers, is a function with one
// output. Both are refused rather than returning "no collisions".
func TestGateStage3TheJoinRefusesWhatItCannotJoin(t *testing.T) {
	answers := []gateStage3Answer{gateStage3Good("readme", "0123456789abcdef", "sha256:r", gateVerdictPass)}
	if _, err := gateStage3Join(answers, nil); err == nil {
		t.Errorf("a join over an empty vocabulary returned no collisions rather than refusing")
	} else if !errors.Is(err, errGateUncheckable) {
		t.Errorf("error is not errGateUncheckable: %v", err)
	}
	if _, err := gateStage3Join(nil, gateStage3Vocab(t)); err == nil {
		t.Errorf("a join over zero answers returned no collisions rather than refusing")
	} else if !errors.Is(err, errGateUncheckable) {
		t.Errorf("error is not errGateUncheckable: %v", err)
	}
	if _, err := gateStage3Collect(gateStage3Inputs{Root: t.TempDir(), Run: "0123456789abcdef", Declared: []string{"readme"}}); err == nil {
		t.Errorf("a run collected against an empty vocabulary was accepted; every answer would validate and the join would compare nothing")
	}
}

// ---------------------------------------------------------------------
// the mode count, counted rather than recalled
// ---------------------------------------------------------------------

// TestGateStage3ModeCountIsCountedAndNotRemembered closes a defect this file
// carried for two releases: the comment above said the harness "has six modes"
// when it had seven, and nothing noticed because nothing counted.
//
// It is the same failure shape the v0.5.2 gate spent four rounds on — prose
// restating a number the tree already knows — and the same answer: derive it. The
// comment states a number, so the number is checked, and the two independent
// lists inside the harness (what `usage` advertises, and what the `case` actually
// dispatches) are checked against each other. A mode that dispatches without
// being advertised is a mode nobody can find; one advertised without dispatching
// is an instruction that fails when followed.
func TestGateStage3ModeCountIsCountedAndNotRemembered(t *testing.T) {
	root := surfaceRepoRoot(t)

	script, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("scripts/gate-stage2/run.sh")))
	if err != nil {
		t.Fatalf("read the stage-2 harness: %v", err)
	}
	advertised, dispatched := gateStage3HarnessModes(t, string(script))

	if len(advertised) == 0 || len(dispatched) == 0 {
		t.Fatalf("read %d advertised and %d dispatched modes out of the harness; a comparison over an empty list is a pass over zero assertions", len(advertised), len(dispatched))
	}
	for mode := range dispatched {
		if !advertised[mode] {
			t.Errorf("`%s` is dispatched by the harness and absent from its usage text, so it is a mode nobody reading the script can find", mode)
		}
	}
	for mode := range advertised {
		if !dispatched[mode] {
			t.Errorf("`%s` is advertised by the harness's usage text and dispatched nowhere, so following the instruction fails", mode)
		}
	}

	self, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("cmd/dossierx/gate_stage3_test.go")))
	if err != nil {
		t.Fatalf("read this file to check the number it states: %v", err)
	}
	stated := regexp.MustCompile(`run\.sh has ([a-z]+) modes`).FindStringSubmatch(string(self))
	if stated == nil {
		t.Fatal("the comment at the top of this file no longer states a mode count in the form `run.sh has <word> modes`, so there is nothing to check it against")
	}
	words := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
		"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
	}
	want, ok := words[stated[1]]
	if !ok {
		t.Fatalf("the comment spells its mode count %q, which is not a number word this test reads", stated[1])
	}
	if want != len(dispatched) {
		t.Errorf("the comment at the top of this file says the harness has %s modes; it dispatches %d. "+
			"This is the defect the test was written for, arriving again.", stated[1], len(dispatched))
	}
}

// gateStage3HarnessModes returns the modes run.sh advertises in `usage` and the
// modes its `case` dispatches, read separately so the two can be compared.
func gateStage3HarnessModes(t *testing.T, script string) (advertised, dispatched map[string]bool) {
	t.Helper()
	advertised, dispatched = map[string]bool{}, map[string]bool{}

	inUsage := false
	inCase := false
	for _, line := range strings.Split(script, "\n") {
		switch {
		case strings.Contains(line, "<<'USAGE'"):
			inUsage = true
			continue
		case strings.HasPrefix(line, "USAGE"):
			inUsage = false
		case strings.HasPrefix(line, `case "$MODE" in`):
			inCase = true
			continue
		case inCase && strings.HasPrefix(line, "esac"):
			inCase = false
		}

		if inUsage {
			// A mode line opens at exactly two spaces with a lower-case word;
			// continuation lines are indented far further, and `--root` is an
			// option rather than a mode.
			if m := regexp.MustCompile(`^  ([a-z][a-z-]*)\s`).FindStringSubmatch(line); m != nil {
				advertised[m[1]] = true
			}
		}
		if inCase {
			if m := regexp.MustCompile(`^  ([a-z][a-z-]*)\)`).FindStringSubmatch(line); m != nil {
				dispatched[m[1]] = true
			}
		}
	}
	return advertised, dispatched
}
