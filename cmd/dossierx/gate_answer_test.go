// gate_answer_test.go is THE RECORDER of a gate run: the file that turns what
// one reading agent produced into an answer this run can be held to.
//
// WHY IT EXISTS. gate_fanout_test.go mints the run and hands thirteen agents
// thirteen bundles. gate_stage3_test.go reads gate/answers/<surface>.json back —
// gateStage3LoadAnswer, gateStage3ValidateAnswer, gateStage3Collect — holds each
// answer to this run's identifier and to stage 2's key for that surface, and
// refuses a record with a hole in it. Between those two halves there was
// nothing. The only writer of an answer file in this repository was
// gateStage3WriteAnswer (gate_stage3_test.go), which takes a *testing.T and
// exists to build fixtures; no flag-guarded entry point, no mode of
// scripts/gate-stage2/run.sh, and nothing else a release can invoke wrote one.
//
// AND THE AGENT CANNOT WRITE ONE ITSELF. An answer must carry the surface's
// CURRENT stage-2 key: gateStage3ValidateAnswer compares it, and every
// carry-forward decision gatePlanRerun makes rests on it. That key is a digest
// over the assembled bundle, the manifest-resolved documents and the method
// declaration, computed by gateStage2Plan — only the Go side can produce it. So
// the thirteen agents could be run and their answers could not lawfully land:
// every file written by hand would carry no fingerprint (refused), an invented
// one (refused), or one copied from somewhere, which is the found-on-disk input
// this whole gate is written against.
//
// This is the third instance of one defect class this release has met: no bundle
// producer, then no run name, now no answer recorder. Each time the reading side
// was complete and under test, and nothing could feed it.
//
// WHAT THE AGENT OWNS AND WHAT THE HARNESS OWES. The payload file this recorder
// reads is the agent's three facts and nothing else:
//
//	{"verdict": "PASS"|"FAILED", "findings": [...], "subjects": {...}}
//
// The run identifier, the surface name and the fingerprint are the harness's,
// and the recorder supplies all three. A payload that states one of them is
// REFUSED rather than ignored: a payload carrying "surface": "changelog" written
// under -answer-surface=roadmap has said something about itself that the gate
// must not silently overwrite, and quietly dropping the key is exactly how one
// surface's answer lands under another surface's name with every check green.
//
// IT VALIDATES BEFORE IT WRITES, WITH THE COLLECTOR'S OWN FUNCTION. The
// assembled answer goes through gateStage3ValidateAnswer — not a check of its
// own — so a malformed answer is refused here, in front of the human running the
// agent, with the identical words gateStage3Collect would use at the end of the
// run. A recorder holding answers to a weaker standard than the collector writes
// files that pass on the way in and fail on the way out, after all thirteen
// agents have been paid for.
//
// HOW IT IS INVOKED. With -answer-record absent it records nothing and asserts
// nothing, and it does that by RETURNING rather than by t.Skip, for
// TestGateFanoutProduce's reason: a SKIP line in a job log is what a maintainer
// reads as "checked", and there is nothing here to check.
//
//	go test ./cmd/dossierx -run '^TestGateAnswerRecord$' -count=1 -args \
//	  -answer-record -answer-surface=<name> -answer-file=<payload.json>
//
// -count=1 IS PART OF THE INVOCATION, as defence in depth and not as the thing
// currently holding the door. The hazard it names is real: `go test` replays a
// cached package result as "ok (cached)" and exits 0 without running anything,
// and a re-record that never executes never reaches the duplicate refusal
// below, which reads to the operator as an answer that landed. What rules that
// out TODAY is not this flag but the shape of the command line — `go test`
// declines to cache any run carrying a flag outside its own cacheable set, and
// everything after `-args` is outside it, so every invocation of this recorder
// re-executes with or without -count=1. Measured on go1.26.5 in a throwaway
// module: an identical run carrying a custom flag after -args re-executed both
// times, while the same test with no custom flags printed "(cached)" on the
// second run. The flag is written down anyway, here and in docs/RELEASING.md,
// because that is the toolchain's guarantee about its own caching rules rather
// than this repository's about its gate: it costs nothing, it matches the
// producer invocation the shell already uses (scripts/gate-stage2/run.sh), and
// a caching rule that widens later must not be able to turn this file's
// refusals into a cached "ok".
//
// Same shape as the rest of the gate files: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var (
	// PRESENCE IS THE SWITCH, and the destination is not the caller's to choose.
	// An answer is addressed by surface name under gateStage3AnswerDir by every
	// reader there is — the collector, the stray check, the driver — so a
	// caller-supplied output path would be a second answer to "where does this
	// answer live", and an answer that used it would be invisible to all three
	// while `go test` exited 0.
	gateAnswerRecordOut = flag.Bool("answer-record", false, "record one agent's answer into gate/answers/ of the checkout under test. Requires -answer-surface and -answer-file")
	// The surface is SUPPLIED and never inferred from the payload: the payload is
	// the agent's, and letting it choose which surface it is filed under is the
	// one thing gateStage3ValidateAnswer's path-versus-document check exists to
	// catch, moved to the one place it cannot be enforced.
	gateAnswerSurface = flag.String("answer-surface", "", "the declared surface this answer is for, as printed by `run.sh fanout`. Requires -answer-record")
	gateAnswerFile    = flag.String("answer-file", "", "the payload file the agent's runner wrote: {\"verdict\", \"findings\", \"subjects\"} and nothing else. Requires -answer-record")
)

// ---------------------------------------------------------------------
// the payload: the agent's half of an answer, and the whole of its half
// ---------------------------------------------------------------------

// gateAnswerPayload is what the agent produced.
//
// It is a SEPARATE TYPE from gateStage3Answer rather than the same struct with
// three fields left blank, because the two have different authors. Decoding a
// payload straight into gateStage3Answer would accept a run identifier, a
// surface and a fingerprint from the agent and then overwrite them, so the file
// on disk would state three things the agent never said while the payload it was
// built from stated three other things — and nothing would ever report the
// disagreement.
type gateAnswerPayload struct {
	Verdict string `json:"verdict"`
	// Findings is a POINTER for gateStage3Answer.Findings's reason, restated at
	// the boundary the payload crosses: an absent list and an empty one are
	// different facts, and the payload has to distinguish them the same way or the
	// distinction is lost before the answer is ever assembled.
	Findings *[]gateFinding `json:"findings"`
	// Subjects is one value for every subject the frame's vocabulary declares.
	// gateStage3SubjectProblems is what holds it to that; nothing here restates
	// the rule.
	Subjects map[string]string `json:"subjects"`
}

// gateAnswerPayloadKeys is the closed set of keys a payload may carry, sorted so
// that the refusal below reads the same on every run.
var gateAnswerPayloadKeys = []string{"findings", "subjects", "verdict"}

// gateAnswerFindingKeys is the same rule one level down: the closed set of keys
// ONE FINDING may carry, sorted for the same reason.
//
// WHAT AN AGENT OWNS ABOUT A FINDING IS FIVE THINGS. The rule it broke, the
// consequence of breaking it, the failure scenario that consequence stands for,
// the detail, and — optionally — the path the finding is ABOUT when that is not a
// document this surface owns. The fifth is the only one that is a fact rather than
// a judgement, and it is the only one an agent may state that moves the rank
// without going through the matrix's own axes; it can move it in one direction,
// upwards, which is what makes it safe to hand over. See gateAnswerRank.
//
// Everything else on gateFinding is the harness's:
//
//	surface   — supplied from -answer-surface. It used to be the agent's, and the
//	            recorder deliberately did not restamp it so that a finding filed
//	            under the wrong surface stayed visible. That argument was about a
//	            field the agent had to write anyway; now that the harness knows the
//	            surface at filing time and refuses a payload that names one, there
//	            is no wrong VALUE left to preserve.
//
//	            WHAT IS LEFT, AND THIS COMMENT USED TO DENY IT. The sentence here
//	            said the failure it protected against "cannot be expressed", and
//	            that was wrong. An agent reading one surface can perfectly well
//	            report a defect whose substance is about ANOTHER surface's document
//	            — release-procedure reading docs/RELEASING.md and finding that it
//	            describes a command the shipped binary does not have. That finding
//	            files under the surface that READ it, which is correct and is what
//	            makes it attributable; what it used to do next was rank at that
//	            surface's reach_class, so a client-shipped defect reported by a
//	            maintainer-class reader ranked P2 and was deferred, with nothing
//	            anywhere naming the document it was about. `about` is the mitigation
//	            and not a cure: an agent that names the path gets the finding ranked
//	            at the higher of the two classes, and an agent that names nothing
//	            leaves the residue exactly where it was.
//	digest    — derived from the finding's own text, and hashing an agent-supplied
//	            one would let a payload choose which ruling it matches.
//	priority  — derived from the surface's reach_class, the consequence below, and
//	            the reach_class of whatever `about` names. An agent that could write
//	            it would be choosing whether its own finding stops the release.
//	severity  — GONE. It is named in the refusal rather than silently unknown,
//	            because a runner ported from the previous schema writes it, and
//	            "unknown key" is a worse message than "that field was replaced".
var gateAnswerFindingKeys = []string{"about", "consequence", "detail", "failure_scenario", "rule"}

// gateAnswerFindingProblems refuses every key in one finding the agent does not
// own, naming them all rather than the first.
//
// It is a separate function from the payload's own key check because the message
// is different: at the top level an unknown key is a runner writing something the
// schema never had, and here the likeliest one by far is `severity`, from a
// runner that predates the priority matrix.
func gateAnswerFindingProblems(path string, index int, keys map[string]json.RawMessage) error {
	var extra []string
	for key := range keys {
		known := false
		for _, allowed := range gateAnswerFindingKeys {
			if key == allowed {
				known = true
				break
			}
		}
		if !known {
			extra = append(extra, "`"+key+"`")
		}
	}
	if len(extra) == 0 {
		return nil
	}
	sort.Strings(extra)
	hint := ""
	if _, ported := keys["severity"]; ported {
		hint = " `severity` is not part of this schema any more: what replaced it is `consequence` (one of " +
			strings.Join(gateConsequences, ", ") + ") crossed with the surface's reach_class in " + gateManifestFile + " to give a priority the agent does not choose."
	}
	return fmt.Errorf("%s finding %d states %s, which the agent does not own. A finding carries exactly `%s`; the surface, the digest and the priority are the harness's to supply, and a key dropped in silence would file a finding stating something other than what was reported.%s",
		path, index, strings.Join(extra, ", "), strings.Join(gateAnswerFindingKeys, "`, `"), hint)
}

// gateAnswerReadPayload reads the agent's file and refuses everything that is
// not exactly those three facts.
//
// THE UNKNOWN-KEY REFUSAL IS THE LOAD-BEARING ONE. Ignoring a key an agent wrote
// is the failure that looks like nothing: a payload naming its own `surface` is
// filed under whatever -answer-surface said, a payload carrying its own
// `fingerprint` has it silently replaced, and in both cases a well-formed answer
// lands stating something the agent did not report. The gate's own rule is that
// a finding reaches the human unfiltered; a key dropped in silence is the same
// wrong at the level of the record.
func gateAnswerReadPayload(surface, path string) (gateAnswerPayload, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return gateAnswerPayload{}, fmt.Errorf("the answer for surface %q could not be read from %s: %w. That file is what the agent produced, and there is no reading of a missing one that says an agent answered — an agent that errored, a runner that wrote nowhere, and a mistyped path all present exactly like this",
			surface, path, err)
	}

	// DECODED AS A KEY SET FIRST, and only then into the type. The type alone
	// cannot see the difference between `"findings": null` and no `findings` key
	// at all — both leave the pointer nil — and it cannot see a key it has no
	// field for at all.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return gateAnswerPayload{}, fmt.Errorf("%s does not parse as a JSON object, so what the agent's runner left at that path is not an answer this run can record: %w",
			path, err)
	}

	var extra []string
	for key := range keys {
		known := false
		for _, allowed := range gateAnswerPayloadKeys {
			if key == allowed {
				known = true
				break
			}
		}
		if !known {
			extra = append(extra, "`"+key+"`")
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return gateAnswerPayload{}, fmt.Errorf("%s states %s, which the agent does not own. A payload carries exactly the three facts the agent produced — `%s` — and the run identifier, the surface name and the fingerprint are the harness's to supply. Dropping a key an agent wrote would file an answer that states something other than what was reported, and nothing downstream could ever see the difference",
			path, strings.Join(extra, ", "), strings.Join(gateAnswerPayloadKeys, "`, `"))
	}

	// A NULL IS NOT AN ABSENCE, and it must not be recorded as one. Left alone it
	// decodes to a nil pointer, gateStage3ValidateAnswer refuses the answer with
	// "has no `findings` key at all", and the human goes looking for a truncated
	// write in a file that is whole and says null on its face.
	if value, ok := keys["findings"]; ok && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return gateAnswerPayload{}, fmt.Errorf("%s states `findings` as null. Three things are being distinguished here and only two of them are answers: `[]` says the agent found nothing, an absent key says its runner never recorded what it found, and null says the runner wrote that absence down as a value. Write `[]` for a surface the agent passed",
			path)
	}

	// AND THE SAME QUESTION ASKED OF EVERY FINDING. The type cannot see a key
	// gateFinding has no field for, and — worse — it CAN see three the agent must
	// not set: a payload naming its own `surface`, `digest` or `priority` decodes
	// perfectly and is then silently overwritten by the recorder, which is the
	// same wrong the top-level check exists for, one level down and against the
	// three fields that decide which ruling a finding matches and whether it stops
	// the release.
	if value, ok := keys["findings"]; ok {
		var list []map[string]json.RawMessage
		if err := json.Unmarshal(value, &list); err != nil {
			return gateAnswerPayload{}, fmt.Errorf("%s states `findings` as something other than a list of findings, so what the agent reported cannot be read one finding at a time: %w", path, err)
		}
		for i, finding := range list {
			if err := gateAnswerFindingProblems(path, i, finding); err != nil {
				return gateAnswerPayload{}, err
			}
		}
	}

	var payload gateAnswerPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return gateAnswerPayload{}, fmt.Errorf("%s parses as an object and not as an answer: %w", path, err)
	}
	return payload, nil
}

// ---------------------------------------------------------------------
// recording one answer
// ---------------------------------------------------------------------

// gateAnswerRank stamps the harness's half onto every finding: the surface it
// was filed under, and the priority the matrix gives for that surface's
// reach_class crossed with the agent's own consequence.
//
// AND THE BORROWED-DOCUMENT CASE, which is what `about` is for. A finding ranks at
// the READING surface's reach_class, and that is wrong whenever the defect's
// substance is about a document some other surface owns: release-procedure is
// maintainer-class, so an acts-wrongly defect it reports in a file owned by
// binary-and-viewer — client-shipped — ranked P2 and was deferred, when the same
// defect reported by the surface that owns the file is a P0. An agent may name the
// path in `about`, and the finding then ranks at whichever of the two reach classes
// gives the HIGHER priority.
//
// WHY A LEVER AN AGENT PULLS IS SAFE HERE, when the whole point of this design is
// that the agent does not choose its own rank. Because it only ever RAISES. The
// crossing is a max over two classes, both of them read out of the reviewed
// manifest, so the worst an agent can do with `about` — by naming the wrong file,
// by naming a file it happens to have read, by naming nothing at all — is stop the
// release over something that was going to be deferred. It cannot lower a rank,
// cannot reach a class the manifest does not declare, and cannot invent one: an
// `about` that resolves to no surface is REFUSED, because a path nobody owns is
// either a typo or a file that moved, and both are the agent's to fix before this
// answer lands.
//
// An `about` whose owner would rank LOWER is ignored, silently and by design. The
// reading surface's class is a fact about where the finding was filed, not a claim
// anybody made, and there is nothing for a refusal to tell the agent to do about it
// — "you named a document that matters less than the one you were reading" is not a
// defect in the answer. Refusing it would only teach agents to leave the key off.
//
// IT REFUSES RATHER THAN RANKS ANYTHING IT CANNOT LOOK UP, and the three ways
// that happens are kept as three separate messages because they are three
// different people's work. A consequence outside the closed set and an empty
// failure scenario are the reading agent's, and the fix is to re-read the frame
// and answer again. A surface with no reach_class is the manifest's, and the fix
// is one reviewed line in surfaces.yaml — a fix the agent cannot make and must
// not be sent to attempt.
//
// A finding that cannot be prioritized is NOT a smaller finding, so none of these
// defaults to P3. The rank decides whether this release stops; inventing one for
// an input nobody declared would be the gate answering its own question.
//
// It names every problem at once, for gateStage3ValidateAnswer's reason: the
// reader is a human running thirteen agents, and a recorder that reveals one
// problem per re-run is a recorder people stop running.
//
// A nil list is passed through UNTOUCHED. An absent `findings` key is a fact
// about the producer that gateStage3ValidateAnswer reports in its own words, and
// a rank step that turned it into an empty list would answer that refusal before
// it could be made.
func gateAnswerRank(root, surface string, findings *[]gateFinding, tracked []string) (*[]gateFinding, error) {
	if findings == nil {
		return nil, nil
	}
	classes, err := gateSurfaceReachClasses(root)
	if err != nil {
		return nil, fmt.Errorf("the reach class of surface %q could not be read, so no finding in this answer can be ranked: %w", surface, err)
	}

	// THE OWNERSHIP MAP IS BUILT ONLY IF SOMETHING ASKS FOR IT. It reads the
	// manifest and walks the tracked set, and gateSurfaceDocuments refuses a
	// manifest whose surface owns no tracked file — a refusal that is right where it
	// lives and would be gratuitous here, over an answer whose findings name no
	// document at all. So an answer with no `about` in it ranks exactly as it did
	// before this key existed, including on a tree where that resolution would fail.
	var owners map[string]string
	for _, f := range *findings {
		if strings.TrimSpace(f.About) == "" {
			continue
		}
		owners, err = gateDocumentOwners(root, tracked)
		if err != nil {
			return nil, fmt.Errorf("a finding in the answer for surface %q names the document it is about, and which surface owns that document could not be resolved, so the finding cannot be ranked: %w", surface, err)
		}
		break
	}

	ranked := make([]gateFinding, 0, len(*findings))
	var problems []string
	for i, f := range *findings {
		f.Surface = surface

		if strings.TrimSpace(f.FailureScenario) == "" {
			problems = append(problems, fmt.Sprintf("finding %d names no `failure_scenario`. It is the sentence that says who is worse off and how, and it is the only thing a human reading the record can check the consequence against — a finding without one asserts a rank and shows nobody the reasoning behind it", i))
		}
		known := false
		for _, c := range gateConsequences {
			if f.Consequence == c {
				known = true
				break
			}
		}
		if !known {
			problems = append(problems, fmt.Sprintf("finding %d states consequence %q, which is not one of %s. The three are what an agent judging one surface is placed to answer — does a reader ACT wrongly, is a reader MISLED, or is it COSMETIC — and a fourth word is a rank nothing can look up",
				i, f.Consequence, strings.Join(gateConsequences, ", ")))
			ranked = append(ranked, f)
			continue
		}

		class, ok := classes[surface]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s declares no reach_class for surface %q, so finding %d cannot be ranked: half of every priority is how far the surface carrying the defect travels, and that half is the manifest's to declare. Add one of %s to that surface's entry",
				gateManifestFile, surface, i, strings.Join(gateReachClasses, ", ")))
			ranked = append(ranked, f)
			continue
		}
		priority, err := gatePriorityFor(class, f.Consequence)
		if err != nil {
			problems = append(problems, fmt.Sprintf("finding %d could not be ranked: %v", i, err))
			ranked = append(ranked, f)
			continue
		}

		// THE MAX, and it is a max rather than an override: whichever of the two
		// classes ranks this consequence higher wins. See the function comment for
		// why that direction is the whole safety argument.
		if about := strings.TrimSpace(f.About); about != "" {
			owner, owned := owners[about]
			if !owned {
				problems = append(problems, fmt.Sprintf("finding %d is about %q, and no surface in %s owns that path. An about that resolves to nothing ranks nothing — name the tracked file, or drop the key. A path that has moved, a path outside the repository and a typo all arrive here identically, and ranking the finding as though the key were absent would silently discard the one thing that raises it",
					i, about, gateManifestFile))
				ranked = append(ranked, f)
				continue
			}
			ownerClass, declared := classes[owner]
			if !declared {
				problems = append(problems, fmt.Sprintf("finding %d is about %q, which surface %q owns, and %s declares no reach_class for that surface — so the half of the priority that says how far the defect travels cannot be read. Add one of %s to that surface's entry",
					i, about, owner, gateManifestFile, strings.Join(gateReachClasses, ", ")))
				ranked = append(ranked, f)
				continue
			}
			ownerPriority, ownerErr := gatePriorityFor(ownerClass, f.Consequence)
			if ownerErr != nil {
				problems = append(problems, fmt.Sprintf("finding %d is about %q, owned by surface %q, and could not be ranked against that surface: %v", i, about, owner, ownerErr))
				ranked = append(ranked, f)
				continue
			}
			if gatePriorityRank[ownerPriority] < gatePriorityRank[priority] {
				priority = ownerPriority
			}
		}

		f.Priority = priority
		ranked = append(ranked, f)
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("the answer for surface %q carries findings this run cannot rank, and an unranked finding is not a smaller finding — it is one nothing can say blocks the release or does not:\n  %s",
			surface, strings.Join(problems, "\n  "))
	}
	return &ranked, nil
}

// gateAnswerRecord assembles one surface's whole answer and writes it, or
// refuses and writes nothing.
//
// IT IS SEPARATE FROM THE FLAG HANDLING, for gateFanoutProduce's reason: every
// refusal below is then exercisable in-process against a fixture rather than
// only through a real run against the real checkout, which is a tree no test may
// write into.
//
// THE ORDER IS THE CALLER'S ARGUMENTS, THEN THIS CHECKOUT'S STATE, THEN THE
// EXPENSIVE QUESTION — the same ordering gateFanoutProduce states. A mistyped
// surface name and an unreadable payload are things the caller got wrong, and
// answering them with "no fan-out was recorded" or with a freshness refusal
// sends an operator to re-produce a run's evidence over a typo. The key comes
// last because computing it fingerprints every declared surface, and none of
// that work changes any refusal above it.
//
// checkoutTree is what `git rev-parse HEAD^{tree}` answers for root, resolved by
// the caller so that "the record on disk is some other tree's" can be
// constructed by a test at all. The entry point below supplies the real one.
//
// tracked is `git ls-files` for root — the same authority gateSurfaceDocuments
// resolves the manifest's patterns against everywhere else in this gate.
func gateAnswerRecord(root, surface, payloadPath, checkoutTree string, tracked []string) (gateStage3Answer, error) {
	// THE SURFACE IS THE MANIFEST'S. Without this the refusal for a typo'd name
	// would come from the key lookup below, as "no key was computed for surface
	// %q on this tree" — which reads as a stage-2 evidence problem and sends the
	// operator to re-run the delta and the captures, when what happened is that
	// `-answer-surface` was spelled wrong. It also names what IS declared,
	// because a list of thirteen is the whole recovery.
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		return gateStage3Answer{}, fmt.Errorf("the declared surfaces could not be read from %s, so nothing can say whether %q is a surface this gate covers: %w", gateManifestFile, surface, err)
	}
	isDeclared := false
	for _, name := range declared {
		if name == surface {
			isDeclared = true
			break
		}
	}
	if !isDeclared {
		return gateStage3Answer{}, fmt.Errorf("%s does not declare surface %q, so this answer would be filed for a surface the gate never fanned out. gateStage3StrayAnswers refuses it later, at the end of the run and after the agents have been paid for; the fan-out covers %s",
			gateManifestFile, surface, strings.Join(declared, ", "))
	}

	// THE AGENT'S OWN BYTES, read before anything about this checkout is asked.
	// An unreadable or over-full payload is an answer that does not exist yet,
	// whatever state the run is in.
	payload, err := gateAnswerReadPayload(surface, payloadPath)
	if err != nil {
		return gateStage3Answer{}, err
	}

	// THE FINDINGS ARE RANKED HERE, at the one moment both halves of a priority
	// are in the same room: the agent's consequence, just read, and the surface's
	// reach_class, declared in the manifest read above. Nothing downstream can do
	// it — the receipt is handed findings and never sees which surface's manifest
	// entry they came from — and a finding that arrives at the receipt unranked
	// blocks the release with a message about a broken producer.
	//
	// It sits between the caller's arguments and this checkout's expensive
	// questions on purpose. A consequence outside the three and an empty failure
	// scenario are the agent's to fix and are refused before a single fingerprint
	// is computed; the reach_class lookup is the tree's state and is asked here
	// too, because a finding nobody can prioritize is not a smaller finding and
	// the operator should hear about the manifest before they hear about the run.
	ranked, err := gateAnswerRank(root, surface, payload.Findings, tracked)
	if err != nil {
		return gateStage3Answer{}, err
	}

	// THE RUN, WHICH IS THE ONLY THING THAT MAKES THIS AN ANSWER RATHER THAN A
	// FILE. gateReadFanout refuses an absent record, one that does not parse, one
	// naming no minted identifier, and one recorded over another tree — all four
	// as errGateStage2NotProduced, because all four mean the same thing here: the
	// agent that produced this payload was reading bundles nobody can attribute to
	// the release being published.
	//
	// It is asked against the CHECKOUT'S tree rather than against a tree passed
	// in, because the bundles the agent read were assembled from these files. A
	// record for another tree sitting beside them is the previous release's run.
	fanout, err := gateReadFanout(root, checkoutTree)
	if err != nil {
		return gateStage3Answer{}, err
	}

	// ONE ANSWER PER SURFACE PER RUN. The fan-out already refuses to start while
	// any file is in the answer directory, so anything here belongs to THIS run —
	// which makes an existing file a second agent answering a surface that has
	// already answered. Resolving that by overwriting is last-wins over two
	// opinions, and last-wins over {FAILED, PASS} silently converts a FAILED into
	// a PASS with nothing left on disk to say the FAILED was ever given. That is
	// errGateDuplicateVerdict's reasoning, reached one stage earlier, where the
	// losing answer can still be kept.
	rel := gateStage3AnswerFile(surface)
	dest := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(dest); err == nil {
		return gateStage3Answer{}, fmt.Errorf("%s already exists and this run records one answer per surface. Overwriting it would let a second opinion replace the first with nothing on disk to say the first was ever given, and if the two disagree the release proceeds on whichever was written last. A wrong answer is not edited or re-recorded: delete %s in full and produce the fan-out again (`scripts/gate-stage2/run.sh fanout --tree %s`), which mints a new identifier every answer must then name",
			rel, gateStage3AnswerDir, checkoutTree)
	} else if !errors.Is(err, os.ErrNotExist) {
		return gateStage3Answer{}, fmt.Errorf("%s could not be stat'd, so nothing can say whether this run already holds an answer for surface %q: %w", rel, surface, err)
	}

	// THE VOCABULARY IS THE FRAME'S — the one file all thirteen agents were
	// handed. Reading it here rather than trusting the payload's key set is what
	// makes "state every subject, invent none" enforceable at write time; a
	// recorder that accepted whatever map it was given would let the join degrade
	// to a group-by over free text one answer at a time.
	vocabulary, err := gateStage3ReadVocabulary(root)
	if err != nil {
		return gateStage3Answer{}, err
	}

	// THE KEY, AND IT IS STAGE 2'S. gateStage2Plan and not gateStage2Keys: the
	// keys alone are computable over evidence left behind by the last release,
	// and an answer fingerprinted against those is an answer about a tree nobody
	// produced evidence for. The plan is freshness, the delta, their agreement,
	// and then the keys — the same precondition the fan-out ran under, asked
	// again here because the tree may have moved since.
	keys, err := gateStage2Plan(root, fanout.Tree, tracked)
	if err != nil {
		return gateStage3Answer{}, fmt.Errorf("surface %q's key for tree %s could not be computed, so this answer could only be recorded against a fingerprint nobody derived from the tree being released: %w", surface, fanout.Tree, err)
	}

	answer := gateStage3Answer{
		Run:         fanout.Run,
		Surface:     surface,
		Verdict:     payload.Verdict,
		Fingerprint: keys[surface],
		Findings:    ranked,
		Subjects:    payload.Subjects,
	}

	// THE FINDINGS ARE STAMPED WITH THIS SURFACE, which is a reversal, and the
	// reason it is safe is that the agent can no longer state one. The old
	// argument was that gateStage3ValidateAnswer requires every finding to name
	// the surface it was raised for, so a recorder that overwrote the field would
	// erase an agent's true statement that it had filed one elsewhere. That
	// depended on the agent writing the field at all: `surface` is now outside
	// gateAnswerFindingKeys and a payload carrying one is refused by name, so
	// there is no agent statement left to erase — only an empty field that would
	// otherwise reach the receipt attributed to nobody. The check in the collector
	// is untouched and still holds every answer read back from disk.

	// THE COLLECTOR'S OWN VALIDATION, AT WRITE TIME. wantRun is this fan-out's
	// identifier and Run was just set from it, so that one comparison cannot fire
	// here; it is passed anyway because this must be the collector's function and
	// not a subset of it. The day a check is added to gateStage3ValidateAnswer,
	// the recorder enforces it with no edit to this file.
	if err := gateStage3ValidateAnswer(answer, surface, fanout.Run, keys[surface], vocabulary); err != nil {
		return gateStage3Answer{}, fmt.Errorf("the answer assembled from %s is one gateStage3Collect would refuse at the end of this run, so it is refused now, before it is on disk looking collected: %w", payloadPath, err)
	}

	if err := gateAnswerWrite(root, answer); err != nil {
		return gateStage3Answer{}, err
	}
	return answer, nil
}

// gateAnswerWrite writes the answer through a temporary file in the destination
// directory and renames, so that a failure partway through cannot leave a
// document that parses far enough to look like an answer — and so that a reader
// racing the write sees the whole file or no file.
//
// The existence check in gateAnswerRecord is a stat and this is a rename, so two
// recorders started for the SAME surface at the same instant could both pass the
// stat. That race is two agents answering one surface simultaneously, which a
// fan-out printing one invocation per surface does not produce; the check is
// against the sequence that does happen, which is an agent re-run by hand after
// its answer already landed.
func gateAnswerWrite(root string, a gateStage3Answer) error {
	rel := gateStage3AnswerFile(a.Surface)
	dest := filepath.Join(root, filepath.FromSlash(rel))
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create the directory for %s: %w", rel, err)
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", rel, err)
	}
	data = append(data, '\n')

	f, err := os.CreateTemp(dir, ".answer-*.json")
	if err != nil {
		return fmt.Errorf("create a temporary file beside %s: %w", rel, err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmp, err)
	}
	// CreateTemp makes the file 0600; the answer is read by the driver that
	// collects the run, which may not be this process's user.
	if err := os.Chmod(tmp, 0o644); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmp, rel, err)
	}
	return nil
}

// ---------------------------------------------------------------------
// the entry point
// ---------------------------------------------------------------------

// TestGateAnswerRecord is how a gate run records an agent's answer. See this
// file's header for the invocation.
//
// PRESENCE, NOT VALUE, for TestGateFanoutProduce's reason: a driver expanding an
// unset shell variable gives a flag EMPTY, which a value check cannot tell apart
// from the flag never having been passed — and under a value check that run
// records nothing, exits 0, and the operator moves on to the next surface
// believing this one answered.
func TestGateAnswerRecord(t *testing.T) {
	var recordGiven, surfaceGiven, fileGiven bool
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "answer-record":
			recordGiven = true
		case "answer-surface":
			surfaceGiven = true
		case "answer-file":
			fileGiven = true
		}
	})

	if !recordGiven {
		// Both other flags IMPLY it: a caller that named a surface or a payload
		// meant to record an answer, not to no-op over one.
		if surfaceGiven || fileGiven {
			t.Fatalf("-answer-surface or -answer-file was given without -answer-record; both imply it, and a run that names an agent's answer and then records nothing is an answer the driver will later look for and not find")
		}
		// The ordinary `go test ./cmd/dossierx` invocation, and every CI run of
		// this package. It RETURNS rather than skipping: see the file header.
		t.Logf("no -answer-record given; this test is the gate's answer recorder, not a correctness check (the tests below are that, and they have already run)")
		return
	}
	if !*gateAnswerRecordOut {
		t.Fatalf("-answer-record was given as false. This flag is the switch that records the answer, so a caller that passed it meant to record one; nothing would be written while `go test` exits 0, and the run would reach its collection missing exactly this surface")
	}
	if !surfaceGiven {
		t.Fatalf("-answer-record was given without -answer-surface; an answer that does not say which surface it is for cannot be filed, and inferring it from the payload would let the agent choose — which is the one thing gateStage3ValidateAnswer's path-versus-document check exists to catch")
	}
	if !fileGiven {
		t.Fatalf("-answer-record was given without -answer-file; there is nothing to record. A recorder that invented an empty answer here would write a PASS over zero findings for a surface no agent reported on")
	}

	root := surfaceRepoRoot(t)
	// THE CHECKOUT'S OWN TREE, resolved here and compared inside gateReadFanout. A
	// git that cannot answer is fatal rather than an empty string: an empty answer
	// would compare unequal to every recorded tree and the refusal would read as a
	// stale fan-out. A check that cannot run is a failure, not a pass.
	checkoutTree, err := gateTreeSHA(root, "HEAD")
	if err != nil {
		t.Fatalf("the tree of the checkout under %s could not be resolved, so nothing can say whether the fan-out on disk is the one this agent read: %v", root, err)
	}

	answer, err := gateAnswerRecord(root, *gateAnswerSurface, *gateAnswerFile, checkoutTree, surfaceTrackedFiles(t, root))
	if err != nil {
		t.Fatalf("refusing to record this answer: %v", err)
	}
	t.Logf("recorded %s for surface %s: run %s, verdict %s, %d finding(s), fingerprint %s",
		gateStage3AnswerFile(answer.Surface), answer.Surface, answer.Run, answer.Verdict, len(*answer.Findings), answer.Fingerprint)
}

// ---------------------------------------------------------------------
// the fixture: a real fan-out, over the real manifest and the real documents
// ---------------------------------------------------------------------

// gateAnswerFixture is a checkout that has really been fanned out: the REAL
// manifest, the REAL documents, the REAL frame and prompts, this run's per-run
// evidence stubbed, and a fan-out produced by gateFanoutProduce itself.
//
// It produces the fan-out rather than writing a plausible gate/fanout.json,
// because every assertion below is about an answer being attributable to a run —
// and a hand-written record would let this whole file pass over a recorder that
// cannot record against a run the producer actually minted.
func gateAnswerFixture(t *testing.T) (root string, tracked []string, run string) {
	t.Helper()
	root, tracked = gateFanoutFixture(t)
	// EVERY SURFACE CARRIES A REACH CLASS BEFORE THE FAN-OUT IS PRODUCED.
	// surfaces.yaml is another lane's file, and a recorder fixture that only works
	// once that lane has classified all thirteen surfaces is a fixture that reds
	// for somebody else's reason — every row below would refuse over an unranked
	// finding instead of over the thing it is about. gatePriorityFillManifest
	// leaves the classes the manifest really declares alone; see its comment for
	// why what it fills with is deliberately not a plausible value.
	gatePriorityFillManifest(t, root)
	doc, err := gateFanoutProduce(root, gateStage2FixtureTree, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the fan-out this recorder records against could not be produced, so every assertion below would be about a run that does not exist: %v", err)
	}
	return root, tracked, doc.Run
}

// gateAnswerStatedSubjects is what the fixture's agent claims to have found, for
// the subjects the frame declares today.
//
// The rest of the vocabulary is answered `not-claimed`, and that is deliberate:
// the map is applied to whatever the frame declares rather than replacing it, so
// a fifth subject declared tomorrow does not red this file for a reason that has
// nothing to do with recording an answer. gateAnswerSubjects asserts the values
// still satisfy the frame's own patterns, so a subject whose form changes reds
// once, here, with the reason stated.
var gateAnswerStatedSubjects = map[string]string{
	"cli-operator":       "agent",
	"go-toolchain-floor": "1.26",
}

// gateAnswerSubjects builds a subject map that answers EVERY subject the real
// frame declares.
func gateAnswerSubjects(t *testing.T, root string) map[string]string {
	t.Helper()
	vocabulary, err := gateStage3ReadVocabulary(root)
	if err != nil {
		t.Fatalf("the fixture's frame declares no usable vocabulary, so every payload below would be refused for a reason no row is about: %v", err)
	}
	out := make(map[string]string, len(vocabulary))
	stated := 0
	for _, subject := range vocabulary {
		value, ok := gateAnswerStatedSubjects[subject.ID]
		if !ok {
			out[subject.ID] = gateStage3NotClaimed
			continue
		}
		if !subject.Match.MatchString(value) {
			t.Fatalf("the fixture answers subject %q as %q and the frame's own pattern %s rejects it; every payload built from this would be refused for a reason no row below is about", subject.ID, value, subject.Match.String())
		}
		out[subject.ID] = value
		stated++
	}
	if stated == 0 {
		t.Fatalf("the fixture's answer states %q for every one of the %d declared subjects; the rows about subject values would then be about a payload that claims nothing", gateStage3NotClaimed, len(vocabulary))
	}
	return out
}

// gateAnswerHonestPayload is a payload every test below then breaks in exactly
// one place. It is a map rather than a struct so that a row can add a key the
// type has no field for, which is one of the shapes being refused.
func gateAnswerHonestPayload(t *testing.T, root string) map[string]any {
	t.Helper()
	return map[string]any{
		"verdict":  gateVerdictPass,
		"findings": []gateFinding{},
		"subjects": gateAnswerSubjects(t, root),
	}
}

// gateAnswerWritePayload marshals a payload into a file OUTSIDE the checkout, at
// the path the recorder is pointed at.
//
// Outside on purpose: a payload written under root would be a file in the tree
// whose key the recorder is about to compute, and a fixture that changed the
// thing it is measuring would make every fingerprint assertion here a function
// of where the test put its scratch file.
func gateAnswerWritePayload(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal the fixture's payload: %v", err)
	}
	return gateAnswerWriteRawPayload(t, string(raw)+"\n")
}

// gateAnswerWriteRawPayload writes bytes verbatim, for the shapes no marshalled
// map can express.
func gateAnswerWriteRawPayload(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write the fixture's payload: %v", err)
	}
	return path
}

// gateAnswerFindingPayload is one finding in the shape an AGENT reports it: the
// four keys it owns and nothing else.
//
// It is a map and not a gateFinding for the reason gateAnswerHonestPayload is:
// marshalling the struct would emit `surface`, `priority` and — for anything the
// recorder stamps — values the payload must not carry, so every row below would
// be refused for the key check rather than for the thing it is about. mutate
// breaks exactly one field; nil leaves an honest finding.
func gateAnswerFindingPayload(mutate func(map[string]any)) map[string]any {
	f := map[string]any{
		"rule":             "counted-claim-mismatch",
		"consequence":      gateConsequenceMisled,
		"failure_scenario": "a reader counts on nineteen commands being all of them and writes a script that misses one",
		"detail":           "the document says nineteen commands and the inventory holds twenty",
	}
	if mutate != nil {
		mutate(f)
	}
	return f
}

// ---------------------------------------------------------------------
// the round trip: what this writes is what the collector reads
// ---------------------------------------------------------------------

// TestGateAnswerRecordWritesAnAnswerTheCollectorAccepts is the positive control
// and the whole point of the file in one: an answer this recorder writes is read
// back by gateStage3LoadAnswer and accepted by gateStage3ValidateAnswer, held to
// THIS run's identifier and to stage 2's key for the surface on this tree.
//
// The fingerprint is asserted against a key computed independently — by calling
// gateStage2Plan here rather than by reading what the recorder wrote — because a
// recorder that wrote a fingerprint of its own invention would round-trip
// perfectly through its own output and be refused by the driver at the end of
// the run.
func TestGateAnswerRecordWritesAnAnswerTheCollectorAccepts(t *testing.T) {
	root, tracked, run := gateAnswerFixture(t)
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	surface := declared[0]

	keys, err := gateStage2Plan(root, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("this tree's keys: %v", err)
	}
	vocabulary, err := gateStage3ReadVocabulary(root)
	if err != nil {
		t.Fatalf("the frame's vocabulary: %v", err)
	}

	written, err := gateAnswerRecord(root, surface, gateAnswerWritePayload(t, gateAnswerHonestPayload(t, root)), gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the recorder refused an honest answer, so every refusal below would pass over a check that fires unconditionally:\n%v", err)
	}
	if written.Run != run {
		t.Errorf("the answer names run %s and the fan-out on disk minted %s; an answer attributed to another run is one gateStage3Collect reads as given by no agent this release", written.Run, run)
	}
	if written.Fingerprint != keys[surface] {
		t.Errorf("the answer carries fingerprint %s and this tree keys surface %q as %s; the recorder is not writing stage 2's key", written.Fingerprint, surface, keys[surface])
	}

	loaded, err := gateStage3LoadAnswer(root, surface)
	if err != nil {
		t.Fatalf("what the recorder wrote is not something the collector's own reader can read: %v", err)
	}
	if err := gateStage3ValidateAnswer(loaded, surface, run, keys[surface], vocabulary); err != nil {
		t.Fatalf("what the recorder wrote is refused by the collector's own validation, so this run's answers could not lawfully land:\n%v", err)
	}
	if !reflect.DeepEqual(loaded, written) {
		t.Errorf("the answer read back is not the answer recorded.\nwrote: %+v\nread:  %+v", written, loaded)
	}
}

// TestGateAnswerRecordFeedsAWholeRunToTheCollector is the assertion the gap this
// file closes was actually about: thirteen agents answer, thirteen answers are
// recorded, and gateStage3Collect accepts the run.
//
// It is separate from the round trip above because one answer proves one file is
// well formed and says nothing about COVERAGE. gateStage3Collect asks the
// manifest what the run was supposed to cover and refuses a record with a hole
// in it, so this is the only assertion here that a run recorded surface by
// surface adds up to a run at all — and it is the assertion that would fail if
// the recorder wrote to a path only its own reader agreed on.
func TestGateAnswerRecordFeedsAWholeRunToTheCollector(t *testing.T) {
	root, tracked, run := gateAnswerFixture(t)
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	keys, err := gateStage2Plan(root, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("this tree's keys: %v", err)
	}
	vocabulary, err := gateStage3ReadVocabulary(root)
	if err != nil {
		t.Fatalf("the frame's vocabulary: %v", err)
	}

	for _, surface := range declared {
		if _, err := gateAnswerRecord(root, surface, gateAnswerWritePayload(t, gateAnswerHonestPayload(t, root)), gateStage2FixtureTree, tracked); err != nil {
			t.Fatalf("surface %q's answer was refused, so this run would be collected over %d of the %d surfaces %s declares:\n%v",
				surface, len(declared)-1, len(declared), gateManifestFile, err)
		}
	}

	collected, err := gateStage3Collect(gateStage3Inputs{
		Root:       root,
		Run:        run,
		Declared:   declared,
		Current:    keys,
		Plan:       gateRerunPlan{Rerun: declared},
		Vocabulary: vocabulary,
	})
	if err != nil {
		t.Fatalf("a run whose every answer this recorder wrote cannot state its own coverage:\n%v", err)
	}
	if len(collected) != len(declared) {
		t.Fatalf("the run collected %d answers over %d declared surfaces", len(collected), len(declared))
	}
	for _, a := range collected {
		if a.Run != run {
			t.Errorf("surface %q's collected answer names run %s, not this run's %s", a.Surface, a.Run, run)
		}
	}
}

// ---------------------------------------------------------------------
// the payload is the agent's three facts, or it is a refusal
// ---------------------------------------------------------------------

// TestGateAnswerRecordRefusesAPayloadItCannotStandBehind walks every shape an
// agent's runner can leave at that path. Each row would otherwise produce a
// perfectly well-formed file under gate/answers/, and each is an answer that
// states something no agent reported.
//
// The rows that assert gateStage3ValidateAnswer's own words are not duplicating
// gate_stage3_test.go. They are what holds the CALL in place: a recorder that
// dropped the validation, or replaced it with a check of its own, would write
// every one of these to disk and the run would fail at collection instead —
// after all thirteen agents had been paid for, with the refusal pointing at a
// file rather than at the agent that produced it.
func TestGateAnswerRecordRefusesAPayloadItCannotStandBehind(t *testing.T) {
	for _, tc := range []struct {
		name string
		// mutate edits the honest payload. Exactly one of mutate and raw is set.
		mutate func(t *testing.T, root string, payload map[string]any)
		// raw replaces the payload bytes entirely, for shapes no map expresses.
		raw string
		// absent removes the payload file altogether.
		absent bool
		want   string
	}{
		{
			name:   "no payload at all",
			absent: true,
			want:   "could not be read from",
		},
		{
			name: "a payload that does not parse",
			raw:  "{\n",
			want: "does not parse as a JSON object",
		},
		{
			name: "a payload that is a list of findings rather than an answer",
			// The shape a runner produces when it serializes the SurfaceFinding
			// calls and forgets the SurfaceVerdict one.
			raw:  "[]\n",
			want: "does not parse as a JSON object",
		},
		{
			name: "a payload that supplies its own fingerprint",
			// The key is the harness's, computed from the tree. An agent that
			// wrote one would have it replaced, and the answer would then carry a
			// fingerprint the agent believes it did not.
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["fingerprint"] = strings.Repeat("e", 64)
			},
			want: "states `fingerprint`",
		},
		{
			name: "a payload that names its own surface",
			// The sharpest one. Ignoring this key files an agent's answer for one
			// surface under another surface's name, and every check downstream
			// passes: the path and the document agree, because the recorder wrote
			// both.
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["surface"] = "changelog"
			},
			want: "states `surface`",
		},
		{
			name: "a payload whose findings are null",
			raw:  "{\"verdict\": \"PASS\", \"findings\": null, \"subjects\": {}}\n",
			want: "states `findings` as null",
		},
		{
			name: "a payload with no findings key at all",
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				delete(payload, "findings")
			},
			want: "has no `findings` key at all",
		},
		{
			name: "a verdict that means neither of the two things a verdict means",
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["verdict"] = "SKIPPED"
			},
			want: "there are exactly two verdicts",
		},
		{
			name: "a PASS that lists what it found",
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["findings"] = []any{gateAnswerFindingPayload(nil)}
			},
			want: "holds a PASS and 1 finding(s)",
		},
		{
			name: "a FAILED that does not say what is wrong",
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["verdict"] = gateVerdictFailed
			},
			want: "holds a FAILED and no findings",
		},
		{
			name: "a finding that files itself under another surface",
			// The sharpest one at this level, and the reason `surface` left the
			// agent's key set: a finding naming a surface would be silently
			// restamped with the one the recorder was pointed at, so a payload
			// stating something about its own reading would land saying something
			// else. It is refused by name rather than dropped.
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["verdict"] = gateVerdictFailed
				payload["findings"] = []any{gateAnswerFindingPayload(func(f map[string]any) {
					f["surface"] = "some-other-surface"
				})}
			},
			want: "finding 0 states `surface`",
		},
		{
			name: "a finding carrying its own priority",
			// The rank decides whether this release stops, so an agent that could
			// write it would be judging its own finding.
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["verdict"] = gateVerdictFailed
				payload["findings"] = []any{gateAnswerFindingPayload(func(f map[string]any) {
					f["priority"] = gatePriorityP3
				})}
			},
			want: "finding 0 states `priority`",
		},
		{
			name: "a finding from a runner that predates the priority matrix",
			// `severity` is refused with its own sentence rather than as an
			// anonymous unknown key: a runner ported from the old schema writes it,
			// and "that field was replaced, here is what by" is the difference
			// between a five-minute fix and a hunt.
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["verdict"] = gateVerdictFailed
				payload["findings"] = []any{gateAnswerFindingPayload(func(f map[string]any) {
					f["severity"] = "major"
				})}
			},
			want: "`severity` is not part of this schema any more",
		},
		{
			name: "a finding a human cannot act on",
			mutate: func(_ *testing.T, _ string, payload map[string]any) {
				payload["verdict"] = gateVerdictFailed
				payload["findings"] = []any{gateAnswerFindingPayload(func(f map[string]any) {
					f["detail"] = "  "
				})}
			},
			want: "names no rule or carries no detail",
		},
		{
			name: "a subject left out",
			mutate: func(t *testing.T, root string, payload map[string]any) {
				subjects := gateAnswerSubjects(t, root)
				for id := range subjects {
					delete(subjects, id)
					break
				}
				payload["subjects"] = subjects
			},
			want: "states no value for subject",
		},
		{
			name: "a subject answered in prose",
			mutate: func(t *testing.T, root string, payload map[string]any) {
				subjects := gateAnswerSubjects(t, root)
				subjects["go-toolchain-floor"] = "Go 1.26 or newer"
				payload["subjects"] = subjects
			},
			want: "is not the declared form",
		},
		{
			name: "a subject the agent invented",
			mutate: func(t *testing.T, root string, payload map[string]any) {
				subjects := gateAnswerSubjects(t, root)
				subjects["release-cadence"] = "quarterly"
				payload["subjects"] = subjects
			},
			want: "does not declare. A subject one agent invents",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, tracked, _ := gateAnswerFixture(t)
			declared, err := gateDeclaredSurfaces(root)
			if err != nil {
				t.Fatalf("declared surfaces: %v", err)
			}
			surface := declared[0]

			var path string
			switch {
			case tc.absent:
				path = filepath.Join(t.TempDir(), "never-written.json")
			case tc.raw != "":
				path = gateAnswerWriteRawPayload(t, tc.raw)
			default:
				payload := gateAnswerHonestPayload(t, root)
				tc.mutate(t, root, payload)
				path = gateAnswerWritePayload(t, payload)
			}

			_, err = gateAnswerRecord(root, surface, path, gateStage2FixtureTree, tracked)
			if err == nil {
				t.Fatalf("the answer was recorded. gate/answers/%s.json now states something no agent reported, and the driver would collect it as this release's reading of that surface", surface)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %q:\n%v", tc.want, err)
			}
			if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile(surface)))); statErr == nil {
				t.Errorf("a refusal left %s on disk; the collector reads whatever is in that directory and cannot tell a refused answer from a given one", gateStage3AnswerFile(surface))
			}
		})
	}
}

// ---------------------------------------------------------------------
// an answer belongs to a run, and to this checkout
// ---------------------------------------------------------------------

// TestGateAnswerRecordRefusesARunItCannotAttributeTo covers the other half: a
// payload that is perfectly well formed, recorded into a checkout whose state
// means the answer would be attributed to nothing, to the previous release, or
// to a tree nobody produced evidence for.
//
// Every row leaves the answer file untouched, which is asserted rather than
// assumed: gateStage3StrayAnswers reads whatever is in that directory, so a
// refusal that wrote first and refused afterwards would leave a file that fails
// the NEXT run instead of this one.
func TestGateAnswerRecordRefusesARunItCannotAttributeTo(t *testing.T) {
	for _, tc := range []struct {
		name string
		// surface overrides the fixture's, for the rows about the name itself.
		surface string
		mutate  func(t *testing.T, root string)
		want    string
	}{
		{
			name:    "a surface the manifest does not declare",
			surface: "chagelog",
			want:    "does not declare surface",
		},
		{
			name: "no fan-out record at all",
			// The agent was run against a bundle from some previous production, or
			// by hand. Nothing on disk says which run this answer belongs to.
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateFanoutFile))); err != nil {
					t.Fatalf("remove the fan-out record: %v", err)
				}
			},
			want: "no fan-out was recorded for tree",
		},
		{
			name: "a fan-out record that does not parse",
			mutate: func(t *testing.T, root string) {
				gateWrite(t, root, gateFanoutFile, "{\n")
			},
			want: "is not a fan-out record",
		},
		{
			name: "a fan-out record left behind by the previous release",
			// The ordinary sequence: a run was fanned out, findings landed, fixes
			// moved the tree, and the record beside the answers is the old one.
			mutate: func(t *testing.T, root string) {
				gateWrite(t, root, gateFanoutFile, "{\n  \"run\": \"0123456789abcdef0123456789abcdef\",\n  \"tree\": \""+strings.Repeat("d", 40)+"\"\n}\n")
			},
			want: "records a fan-out over tree",
		},
		{
			name: "a fan-out record that names no run",
			// `printf '{}' > gate/fanout.json` is the one-line workaround for a
			// fan-out that has been refusing, and an empty run identifier compares
			// equal to the empty Run of every half-written answer.
			mutate: func(t *testing.T, root string) {
				gateWrite(t, root, gateFanoutFile, "{\"tree\": \""+gateStage2FixtureTree+"\"}\n")
			},
			want: "not a minted run identifier",
		},
		{
			name: "the run's own evidence was never produced",
			// gateStage2Plan and not gateStage2Keys. The keys are computable from
			// the files alone, so a recorder that asked only for keys would
			// fingerprint this answer against a delta, a baseline and captures left
			// behind by the last release — and every fingerprint would be perfectly
			// current for the tree.
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateStage2RunFile))); err != nil {
					t.Fatalf("remove the run manifest: %v", err)
				}
			},
			want: gateStage2RunFile,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, tracked, _ := gateAnswerFixture(t)
			declared, err := gateDeclaredSurfaces(root)
			if err != nil {
				t.Fatalf("declared surfaces: %v", err)
			}
			surface := declared[0]
			if tc.surface != "" {
				surface = tc.surface
			}
			path := gateAnswerWritePayload(t, gateAnswerHonestPayload(t, root))
			if tc.mutate != nil {
				tc.mutate(t, root)
			}

			_, err = gateAnswerRecord(root, surface, path, gateStage2FixtureTree, tracked)
			if err == nil {
				t.Fatalf("the answer was recorded, so this run holds an answer for surface %q that belongs to no fan-out over this tree", surface)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %q:\n%v", tc.want, err)
			}
			if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile(surface)))); statErr == nil {
				t.Errorf("a refusal left %s on disk", gateStage3AnswerFile(surface))
			}
		})
	}
}

// ---------------------------------------------------------------------
// one answer per surface per run
// ---------------------------------------------------------------------

// TestGateAnswerRecordKeepsTheFirstAnswerASurfaceGave is proven in both
// directions, because "it refused" is only half of what matters.
//
// The failure this stops is not a lost file. It is last-wins over two opinions:
// an agent re-run by hand after its first answer landed — because the first
// FAILED and the surface "looks fine on a second read" — replaces a FAILED with
// a PASS, and nothing on disk records that the FAILED was ever given. The gate
// then reports a clean surface, the receipt carries no finding, and the human
// who would have ruled on it never sees it. So the second write is refused AND
// the first answer must still be byte-identical afterwards.
func TestGateAnswerRecordKeepsTheFirstAnswerASurfaceGave(t *testing.T) {
	root, tracked, _ := gateAnswerFixture(t)
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	surface := declared[0]

	failing := gateAnswerHonestPayload(t, root)
	failing["verdict"] = gateVerdictFailed
	failing["findings"] = []any{gateAnswerFindingPayload(nil)}
	if _, err := gateAnswerRecord(root, surface, gateAnswerWritePayload(t, failing), gateStage2FixtureTree, tracked); err != nil {
		t.Fatalf("the first answer was refused, so this test is not about a second one: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile(surface))))
	if err != nil {
		t.Fatalf("read the answer that was just recorded: %v", err)
	}
	if !strings.Contains(string(first), gateVerdictFailed) {
		t.Fatalf("the first answer does not read as FAILED, so the overwrite below would not be the verdict-flipping case this test is about:\n%s", first)
	}

	_, err = gateAnswerRecord(root, surface, gateAnswerWritePayload(t, gateAnswerHonestPayload(t, root)), gateStage2FixtureTree, tracked)
	if err == nil {
		t.Fatalf("a second answer for surface %q was recorded over the first. The FAILED that surface reported is gone, and the run now reads as one where nobody found anything", surface)
	}
	if !strings.Contains(err.Error(), "one answer per surface") {
		t.Errorf("the refusal does not say that a run holds one answer per surface:\n%v", err)
	}
	// The recovery has to be in the message, because the move a human reaches for
	// here — editing the file, or deleting just this one — produces exactly the
	// record gateStage3StrayAnswers and gateReadFanout exist to refuse.
	if !strings.Contains(err.Error(), gateStage3AnswerDir) || !strings.Contains(err.Error(), "fanout") {
		t.Errorf("the refusal does not name the recovery — delete the whole answer directory and fan out again:\n%v", err)
	}

	second, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile(surface))))
	if err != nil {
		t.Fatalf("read the answer after the refusal: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("the first answer was rewritten by a recorder that refused.\nwas:\n%s\nnow:\n%s", first, second)
	}
}

// ---------------------------------------------------------------------
// the flag contract
// ---------------------------------------------------------------------

// TestGateAnswerRecord_FlagContract exercises the entry point by running it in a
// SUBPROCESS, which is the only way to observe it: flag values are process-global
// and this binary only ever sees the flags it was started with.
//
// Every row refuses before anything is written, and that is a constraint on the
// rows rather than a coincidence — the entry point runs against the REAL
// checkout, which no test may write into. What the rows prove is exactly the
// wiring the in-process tests cannot: that -answer-record switches, that the
// VALUES of -answer-surface and -answer-file reach the recorder rather than
// being invented there, and that the tree the fan-out record is held to is read
// from git rather than assumed. The recording itself is exercised in-process,
// over a fixture built from the real manifest, the real documents and a real
// fan-out.
//
// WHAT NO ROW HERE CAN PROVE, stated rather than left to be discovered: that the
// tree read from git is the tree gateReadFanout is given. Reaching that
// comparison against the real checkout means getting past the payload read, and
// what happens next then depends on whether this machine happens to hold a
// fan-out record for HEAD — on a checkout mid-release it does, and the row would
// go on to write into a tree other suites are reading. The in-process row "a
// fan-out record left behind by the previous release" is what holds the
// comparison itself in place. The residue is small by construction: every way
// that wiring can break makes the recorder refuse EVERY answer on the first
// surface, loudly, rather than record a wrong one quietly.
// gateAnswerSnapshot reads a path that may legitimately hold nothing, and fails
// on any read error that is not absence.
//
// The distinction is the whole value of the rows below. They assert that a
// refusing run CHANGED NOTHING at that path rather than that the path is empty,
// because the real checkout can hold an answer from a release run in progress —
// and a read that failed for some other reason would compare equal to absence
// and make the assertion pass over a file nobody could see.
func gateAnswerSnapshot(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func TestGateAnswerRecord_FlagContract(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("the go toolchain is not on PATH, so the flag contract of the gate's answer recorder cannot be exercised. A check that cannot run is a failure, not a pass.")
	}
	root := surfaceRepoRoot(t)
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}

	// env is added to this process's environment for the one row that has to
	// take git away; "" leaves it inherited.
	run := func(t *testing.T, env string, args ...string) (output string, code int) {
		t.Helper()
		full := append([]string{"test", "./cmd/dossierx", "-run", "^TestGateAnswerRecord$", "-count=1", "-v", "-args"}, args...)
		cmd := exec.Command("go", full...)
		cmd.Dir = root
		if env != "" {
			cmd.Env = append(os.Environ(), env)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return string(out), exitErr.ExitCode()
			}
			t.Fatalf("go %s: %v\n%s", strings.Join(full, " "), err, out)
		}
		return string(out), 0
	}

	t.Run("no flags at all passes without recording and WITHOUT skipping", func(t *testing.T) {
		out, code := run(t, "")
		if code != 0 {
			t.Fatalf("the ordinary `go test ./cmd/dossierx` invocation must not fail, got exit %d:\n%s", code, out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("the recorder SKIPPED. A skip in a job log is what a maintainer reads as \"checked\"; with no -answer-record there is nothing to record and the test must simply pass:\n%s", out)
		}
	})

	// A path that exists nowhere, used by the rows that must refuse before any
	// payload is read.
	missing := filepath.Join(t.TempDir(), "no-such-payload.json")

	for _, tc := range []struct {
		name    string
		surface string
		args    []string
		// env is added to the subprocess's environment; "" inherits this one's.
		env  string
		want string
	}{
		{
			name:    "a surface named with no -answer-record",
			surface: declared[0],
			args:    []string{"-answer-surface=" + declared[0]},
			want:    "without -answer-record",
		},
		{
			name:    "a payload named with no -answer-record",
			surface: declared[0],
			args:    []string{"-answer-file=" + missing},
			want:    "without -answer-record",
		},
		{
			name:    "-answer-record given as false",
			surface: declared[0],
			args:    []string{"-answer-record=false", "-answer-surface=" + declared[0], "-answer-file=" + missing},
			want:    "-answer-record was given as false",
		},
		{
			name:    "a recording with no surface",
			surface: declared[0],
			args:    []string{"-answer-record"},
			want:    "without -answer-surface",
		},
		{
			name:    "a recording with no payload",
			surface: declared[0],
			args:    []string{"-answer-record", "-answer-surface=" + declared[0]},
			want:    "without -answer-file",
		},
		{
			// The row that proves -answer-surface's VALUE reaches the recorder: a
			// name the real manifest does not declare, refused with the real
			// manifest's own list, which the entry point could not have invented.
			name:    "a surface this repository does not declare",
			surface: "not-a-declared-surface",
			args:    []string{"-answer-record", "-answer-surface=not-a-declared-surface", "-answer-file=" + missing},
			want:    "does not declare surface",
		},
		{
			// And the row that proves -answer-file's VALUE reaches it: a declared
			// surface, so the refusal can only come from the payload read, quoting
			// the exact path passed in.
			name:    "a payload the runner never wrote",
			surface: declared[0],
			args:    []string{"-answer-record", "-answer-surface=" + declared[0], "-answer-file=" + missing},
			want:    missing,
		},
		{
			// The tree this checkout is at is READ FROM GIT, and a git that cannot
			// answer is a failure rather than an empty string. Without this row the
			// resolution could be deleted outright — every in-process test passes
			// the tree in as a parameter, so nothing else here says the entry point
			// obtains a real one. An empty answer would compare unequal to whatever
			// gate/fanout.json records and the refusal would read as a stale
			// fan-out, sending the operator to fan out again over a git problem.
			name:    "a checkout whose tree git cannot answer for",
			surface: declared[0],
			args:    []string{"-answer-record", "-answer-surface=" + declared[0], "-answer-file=" + missing},
			env:     "GIT_DIR=" + filepath.Join(t.TempDir(), "not-a-git-dir"),
			want:    "could not be resolved",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The real checkout may legitimately hold an answer from a release run
			// in progress, so the assertion is that this row changed nothing —
			// never that the path is empty.
			dest := filepath.Join(root, filepath.FromSlash(gateStage3AnswerFile(tc.surface)))
			before := gateAnswerSnapshot(t, dest)

			out, code := run(t, tc.env, tc.args...)
			if code == 0 {
				t.Fatalf("expected a refusal, got exit 0. The operator would move on to the next surface believing this one answered:\n%s", out)
			}
			if strings.Contains(out, "--- SKIP") {
				t.Errorf("expected a FAIL, not a SKIP:\n%s", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected the refusal to name %q, got:\n%s", tc.want, out)
			}
			if after := gateAnswerSnapshot(t, dest); !bytes.Equal(before, after) {
				t.Errorf("a run that refused wrote %s in the checkout under test", gateStage3AnswerFile(tc.surface))
			}
		})
	}
}
