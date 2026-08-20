// gate_adjudication_test.go is THE OVERRIDE RECORD: gate/adjudications.json,
// where a human's ruling that one specific finding does not block one specific
// release goes — and the rules under which that ruling is honoured, refused,
// or reported back as no longer being about anything.
//
// WHY IT EXISTS, measured off this repository's own releases rather than
// asserted. The gate converges by rounds: every fix moves the tree, every tree
// move needs a fresh reading round, so rounds = fix waves + 1. Across one
// release the blocking counts ran 7, 3, 1, 2 — toward "one small thing per
// round, forever", not toward zero — because the record had no way to hold a
// judgement that was already made. Round four is the worked example: two
// blocking findings, one a genuine defect (fixed), one an agent reporting that
// it was handed no file declaring a test name and so could not verify a
// command line. The second was true, was useful, and was not a reason the
// software could not ship — the maintainer read it and said so in about a
// minute — but the only way to CLEAR it was to edit surfaces.yaml, which moved
// the tree, which bought another round, which surfaced the next one. That
// minute of judgement had nowhere to go. This file is the place it goes.
//
// WHAT A RULING IS, precisely. One human's statement, for one release, that
// one finding — named by surface and rule, and quoted by its own
// failure_scenario, verbatim — does not block that release, with the reason
// stated. It is applied at EVALUATION time, inside the verdict predicate, and
// nowhere else: not at recording (the answers describe the tree, and an
// answer rewritten by a ruling is an agent misquoted), and not at fan-out
// (the agents read surfaces, not verdicts). The answers say what is true;
// the rulings say what blocks.
//
// THE ONE RULE THAT BENDS FOR NOBODY. A finding whose consequence is
// acts-wrongly can never be cleared by a ruling — not by any signature, any
// reason, or any expiry. That is a maintainer's standing ruling one level up:
// a client-shipped procedure that makes its reader act wrongly does not ship,
// full stop. And a ruling that names such a finding is not skipped — it is a
// LOUD REFUSAL OF THE WHOLE FILE, no ruling in it applying, because a record
// that quietly declines to do what it says is worse than no record: the
// author of that ruling believes a finding is handled, and nothing has
// handled it.
//
// IDENTITY, AND THE TRAP IN IT. Findings are re-derived from scratch every
// round and `rule` is a free-text slug the agent writes, so a ruling keyed on
// (surface, rule) alone is unsafe: a later round can reuse a slug for
// different substance and the stale ruling would silently clear a finding
// nobody looked at. So the ruling records the failure_scenario it ruled on,
// verbatim, and applies only while the finding still carries that scenario
// byte for byte. The three outcomes are all reported, none silently:
//
//	the scenario matches      — the ruling applies; the finding stays on the
//	                            receipt in full and the ruling rides beside
//	                            it (who ruled, when, why) into the run
//	                            record, so "ruled non-blocking" and "never
//	                            raised" stay different facts forever.
//	the scenario differs      — the ruling does NOT apply, and the verdict
//	                            says so in words: a ruling for this rule
//	                            exists and the finding has changed, so it
//	                            needs re-ruling. The finding keeps blocking.
//	no finding matches at all — the ruling is reported stale, visibly, and
//	                            does not fail the gate on its own.
//
// VERSION-SCOPED, NEVER INHERITED. A ruling names the release it was made
// for, and a ruling from an earlier release does not apply to this one — the
// finding it cleared has to be re-read and re-ruled against the release that
// actually ships. Rulings for other releases are not stale and not applied;
// they are history, and history is kept: nothing in this mechanism deletes
// anything, which is the whole repair. The old documented recovery was
// hand-deleting the finding from gate/answers/<surface>.json, which left an
// adjudicated finding indistinguishable from one nobody raised; a ruling is
// the same judgement left ON the record instead of carved out of it.
//
// WHAT THIS MECHANISM CANNOT PROMISE, stated so nobody leans on it. ruled_by
// is paper: nothing here authenticates that the named person wrote the
// ruling, any more than the driver can authenticate its CI-evidence record —
// its own header says the strongest thing it can check is presence and
// subject. Only the forge's signed commits and branch protection make a
// ruling attributable to a person; in this tree it is a name in a file, and
// the gate treats it as the human's word the same way it treats the evidence
// record as CI's. What IS held mechanically: the file's shape (a malformed or
// unreadable record is a FAILED gate, never "no rulings" — a check that
// cannot run is a failure, per CLAUDE.md), the identity discipline above, and
// the acts-wrongly refusal.
//
// WHY gate/adjudications.json IS DELIBERATELY NOT IN gateIntegrityPatterns,
// said here because the "fix" is one line and tempting. The integrity digest
// is the gate's DEFINITION — the code that decides what a ruling can and
// cannot do, which includes this file and is already inside the
// cmd/dossierx/gate_*_test.go pattern, so weakening the RULES re-keys every
// surface and costs a full re-read, exactly as it should. The DATA file is a
// per-release decision record, the same category as the evidence files:
// adding a ruling changes no question any agent was asked and no document any
// surface owns. Fold it into the integrity digest and recording a ruling
// re-keys all thirteen surfaces and forces the full re-read the ruling exists
// to make unnecessary — the mechanism would defeat itself on first use. The
// tree still moves when a ruling is committed (it is tracked, and the driver
// reads it out of the TREE, never the working copy), so the stage-2 cycle
// still runs once over the ruled tree; what the exclusion buys is that no
// surface key moves, so that cycle re-reads nothing.
//
// Same shape as the rest of the gate: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's
// behaviour_fingerprint. gate_driver_test.go's "WHY IT IS TEST CODE" note is
// the whole argument and is not repeated here.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// gateAdjudicationsFile is the record, repo-relative. Tracked — it must reach
// the tag, because a ruling that cleared a finding and then is absent from the
// released tree is a verdict the release cannot account for — and named in
// gate/.gitignore's allow-list for that reason.
const gateAdjudicationsFile = "gate/adjudications.json"

// gateAdjudication is one ruling. Every field is mandatory; see
// gateAdjudicationProblems for what each refusal protects.
type gateAdjudication struct {
	// Version is the release this ruling was made for, spelled the way the
	// driver is invoked (vX.Y.Z). A ruling is never inherited by a later
	// release: the finding must be re-read against the tree that actually
	// ships, by a human, every time.
	Version string `json:"version"`
	// Surface and Rule name the finding, in the reading agent's own terms.
	Surface string `json:"surface"`
	Rule    string `json:"rule"`
	// FailureScenario is the finding's failure_scenario AS RULED ON, verbatim.
	// It is the identity check that makes (surface, rule) safe to key on: the
	// ruling applies only while the finding still says, byte for byte, what
	// the human read when they ruled.
	FailureScenario string `json:"failure_scenario"`
	// RuledBy is a person's name. In-tree it is paper — see the package
	// comment for what does and does not make it attributable.
	RuledBy string `json:"ruled_by"`
	// RuledAt is when, RFC 3339 or a bare 2006-01-02 date.
	RuledAt string `json:"ruled_at"`
	// Reason is why this finding does not block this release. Refused when
	// empty: a ruling without a stated reason is an assertion, not a
	// judgement, and the next reader of the record could not re-examine it.
	Reason string `json:"reason"`
}

// gateAdjudicationsDoc is the file's shape: one top-level key, so that the
// document says what it is, and so that a future second key cannot arrive
// unnoticed (the decoder refuses unknown fields).
type gateAdjudicationsDoc struct {
	Rulings []gateAdjudication `json:"rulings"`
}

// gateAdjudicationVersionRE is the one spelling a ruling's version may take —
// the spelling the driver is invoked with and the receipt records. It is
// refused rather than normalized because the failure mode of a misspelled
// version is SILENT: "0.6.0" for "v0.6.0" would simply never match any
// release, and the ruling's author would believe a finding handled that
// nothing handles.
var gateAdjudicationVersionRE = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// gateAdjudicationProblems is every way one ruling is malformed. The clauses
// are plain sentences; the caller prefixes the ruling's index.
//
// The scenario is held to gateScenarioProblem — the same bar the finding it
// quotes was held to — because a scenario the finding schema refuses can never
// have been recorded on a finding, so a ruling carrying one could never match
// anything: it would sit in the file as a permanently stale ruling whose
// author believes otherwise, which is the quiet decline this whole record
// refuses to be.
func gateAdjudicationProblems(a gateAdjudication) []string {
	var problems []string
	if !gateAdjudicationVersionRE.MatchString(a.Version) {
		problems = append(problems, fmt.Sprintf("states version %q, which is not a release spelled vX.Y.Z. A ruling is scoped to exactly one release, and a version spelled any other way matches none — the ruling would decline silently, which is worse than being refused here", a.Version))
	}
	if strings.TrimSpace(a.Surface) == "" || strings.TrimSpace(a.Rule) == "" {
		problems = append(problems, "names no surface or no rule, so it rules on no identifiable finding")
	}
	if p := gateScenarioProblem(a.FailureScenario); p != "" {
		problems = append(problems, "quotes a failure_scenario the finding schema itself would refuse, so it can never match a recorded finding and would sit stale forever: "+p)
	}
	if strings.TrimSpace(a.RuledBy) == "" {
		problems = append(problems, "carries no ruled_by; a ruling nobody signs is a ruling nobody made, and the record must say who to ask")
	}
	if _, errFull := time.Parse(time.RFC3339, a.RuledAt); errFull != nil {
		if _, errDate := time.Parse("2006-01-02", a.RuledAt); errDate != nil {
			problems = append(problems, fmt.Sprintf("states ruled_at %q, which is neither RFC 3339 nor a 2006-01-02 date; an unreadable time cannot be put in order against the rounds it ruled between", a.RuledAt))
		}
	}
	if strings.TrimSpace(a.Reason) == "" {
		problems = append(problems, "carries no reason. A ruling without a stated reason is an assertion, not a judgement — there is nothing for the next reader to re-examine or to disagree with")
	}
	return problems
}

// gateParseAdjudications turns the file's bytes into rulings, or refuses.
//
// EVERYTHING IT CANNOT STAND BEHIND IS AN ERROR, and the caller treats every
// error as a FAILED gate. There is no branch that reads a damaged record as
// "no rulings": the file may carry the acts-wrongly refusal the evaluation is
// obliged to make, and a parse that shrugged would skip it.
//
// Unknown fields are refused (DisallowUnknownFields) for the misspelling's
// sake: a hand-written `"expires"`, or `"reasons"` for `"reason"`, would
// otherwise be dropped in silence and its author would believe it in force.
// A duplicate (version, surface, rule) is refused because two rulings for one
// finding identity are two answers to one question — the record holds one
// ruling per finding per release, and re-ruling REPLACES the old one, whose
// text stays reachable in git history rather than in ambiguity here.
func gateParseAdjudications(blob []byte) ([]gateAdjudication, error) {
	dec := json.NewDecoder(bytes.NewReader(blob))
	dec.DisallowUnknownFields()
	var doc gateAdjudicationsDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s does not parse as {\"rulings\": [...]}: %w. A ruling record that cannot be read is a FAILED gate, never \"no rulings\"", gateAdjudicationsFile, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%s carries content after the rulings document; a file that is two documents is a file whose second half nobody validates", gateAdjudicationsFile)
	}

	var problems []string
	seen := map[[3]string]bool{}
	for i, a := range doc.Rulings {
		for _, p := range gateAdjudicationProblems(a) {
			problems = append(problems, fmt.Sprintf("ruling %d %s", i, p))
		}
		key := [3]string{a.Version, a.Surface, a.Rule}
		if seen[key] {
			problems = append(problems, fmt.Sprintf("ruling %d duplicates (%s, surface %s, rule %s); one finding identity holds one ruling per release, and a re-ruling replaces the old one rather than sitting beside it", i, a.Version, a.Surface, a.Rule))
		}
		seen[key] = true
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s is refused:\n%s", gateAdjudicationsFile, strings.Join(problems, "\n"))
	}
	return doc.Rulings, nil
}

// gateReadAdjudications reads the record OUT OF THE TREE being released, never
// off the working copy — gateDriverTreeFile's rule, for gateDriverTreeFile's
// reason: an uncommitted ruling would clear a finding now and be absent from
// the tag, and the published release could not account for its own verdict.
//
// A tree that carries no record at all is genuinely zero rulings, and that is
// the fail-closed direction: absence can only make the gate stricter, because
// no ruling means nothing is cleared. Every other failure — the tree
// unreadable, the blob unreadable, the document malformed — is an error the
// caller treats as a FAILED gate.
func gateReadAdjudications(dir, branch string) ([]gateAdjudication, error) {
	listing, err := gateGit(dir, "ls-tree", branch, "--", gateAdjudicationsFile)
	if err != nil {
		return nil, fmt.Errorf("%w: whether %s carries %s could not be read, so this run cannot say which rulings the release does or does not make: %w", errGateUncheckable, branch, gateAdjudicationsFile, err)
	}
	if strings.TrimSpace(listing) == "" {
		return nil, nil
	}
	blob, err := gateDriverTreeFile(dir, branch, gateAdjudicationsFile)
	if err != nil {
		return nil, fmt.Errorf("%w: %s names %s and its contents could not be read; an unreadable ruling record is a FAILED gate, never \"no rulings\": %w", errGateUncheckable, branch, gateAdjudicationsFile, err)
	}
	rulings, err := gateParseAdjudications([]byte(blob))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errGateUncheckable, err)
	}
	return rulings, nil
}

// gateAdjudicatedFinding is one finding with the ruling that touched it —
// TOGETHER, because the pair is the record's whole point: the finding stays in
// full and who ruled, when and why is printed beside it.
type gateAdjudicatedFinding struct {
	Finding gateFinding      `json:"finding"`
	Ruling  gateAdjudication `json:"ruling"`
}

// gateAdjudicationOutcome is what applying a release's rulings to a receipt's
// findings produced. The exported fields travel into the driver's run record;
// the unexported maps are evaluate's working view (finding index → ruling) and
// carry the same facts.
//
// It is a pure function of its inputs, like the verdict it feeds: the same
// receipt and the same rulings always produce the identical outcome, in the
// same order, so a re-run over an unchanged tree reproduces the identical
// record.
type gateAdjudicationOutcome struct {
	// Cleared is every finding an applicable ruling cleared. The finding is
	// NOT removed from the receipt — nothing is deleted, ever — this is the
	// annotation printed beside it.
	Cleared []gateAdjudicatedFinding `json:"cleared,omitempty"`
	// NeedsReRuling is every finding whose (surface, rule) a ruling names
	// while the recorded failure_scenario no longer matches: the finding has
	// changed since a human read it, the ruling does not apply, and the
	// finding keeps blocking until a human rules on what it says now.
	NeedsReRuling []gateAdjudicatedFinding `json:"needs_re_ruling,omitempty"`
	// Stale is every ruling for THIS release that matches no finding at all —
	// reported visibly, failing nothing on its own: most often it means the
	// finding it ruled on was also fixed in the tree, which is a better
	// ending than the ruling, not a defect in it.
	Stale []gateAdjudication `json:"stale,omitempty"`

	cleared map[int]gateAdjudication
	changed map[int]gateAdjudication
}

func (o gateAdjudicationOutcome) empty() bool {
	return len(o.Cleared) == 0 && len(o.NeedsReRuling) == 0 && len(o.Stale) == 0
}

// gateApplyAdjudications matches one release's rulings against one receipt's
// findings and says what applies, what needs re-ruling, and what is stale.
//
// The error is the one refusal that voids the whole file: a ruling whose
// verbatim scenario names an acts-wrongly finding. It is an error rather than
// a skipped ruling because the two honest options are to obey the ruling
// (forbidden — acts-wrongly is not anyone's to clear) or to say loudly that
// the record demands what cannot be done; ignoring it would leave the record
// asserting something the gate quietly declines, and every OTHER ruling in a
// file that tried it is suspended with it, because a file that misstates what
// rulings can do has to be re-examined whole. Note the refusal keys on the
// byte-identical scenario: a ruling for the same (surface, rule) whose
// scenario differs is an ordinary needs-re-ruling report — it claims nothing
// about the finding as it now stands — and the finding blocks regardless.
//
// Rulings for OTHER releases are out of scope entirely: not applied, not
// stale, not refused. They are history, and the record keeps its history.
func gateApplyAdjudications(version string, findings []gateFinding, rulings []gateAdjudication) (gateAdjudicationOutcome, error) {
	out := gateAdjudicationOutcome{
		cleared: map[int]gateAdjudication{},
		changed: map[int]gateAdjudication{},
	}
	for _, a := range rulings {
		if a.Version != version {
			continue
		}
		named := false
		for i, f := range findings {
			if f.Surface != a.Surface || f.Rule != a.Rule {
				continue
			}
			named = true
			if f.FailureScenario != a.FailureScenario {
				out.changed[i] = a
				out.NeedsReRuling = append(out.NeedsReRuling, gateAdjudicatedFinding{Finding: f, Ruling: a})
				continue
			}
			if f.Consequence == gateConsequenceActsWrongly {
				return gateAdjudicationOutcome{}, fmt.Errorf(
					"%s is refused in full: the ruling by %s (%s) for surface %s, rule %s names a finding whose consequence is %s, and such a finding is not anyone's to clear — not by any signature, reason or expiry. "+
						"No ruling in this file applies to this evaluation. A record that quietly declined to do what it says would be worse than no record, so the whole file is suspended until that ruling is removed; "+
						"the finding itself clears only when the tree is fixed, or when its failure_scenario (%q) is shown with evidence to describe no defect",
					gateAdjudicationsFile, a.RuledBy, a.RuledAt, a.Surface, a.Rule, gateConsequenceActsWrongly, f.FailureScenario)
			}
			out.cleared[i] = a
			out.Cleared = append(out.Cleared, gateAdjudicatedFinding{Finding: f, Ruling: a})
		}
		if !named {
			out.Stale = append(out.Stale, a)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// gateAdjudicationFixture is a well-formed ruling for the well-formed finding
// gateReceiptFinding builds, quoting that helper's scenario verbatim — which
// is what a real ruling does: the human copies the sentence they ruled on.
func gateAdjudicationFixture(version, surface, rule string) gateAdjudication {
	return gateAdjudication{
		Version:         version,
		Surface:         surface,
		Rule:            rule,
		FailureScenario: "a reader who trusts this line plans against a project that is not the one shipping, and their first run surprises them",
		RuledBy:         "A. Maintainer",
		RuledAt:         "2026-08-20",
		Reason:          "the line is stale but the surrounding section names the shipping version twice; no reader acts on this alone, and the fix rides the next release",
	}
}

// TestGateAdjudicationsParseRefusesEveryShapeItCannotStandBehind holds the
// file's whole grammar from the refusing side, message and all — because the
// value of a refusal is which sentence a human is sent to, and a row satisfied
// by any error at all would pass over a refusal that fired for another reason.
func TestGateAdjudicationsParseRefusesEveryShapeItCannotStandBehind(t *testing.T) {
	well := gateAdjudicationFixture("v9.9.9", "readme", "stale-count")
	wellDoc := func(mutate func(a *gateAdjudication)) []byte {
		a := well
		mutate(&a)
		blob, err := json.Marshal(gateAdjudicationsDoc{Rulings: []gateAdjudication{a}})
		if err != nil {
			t.Fatalf("marshal the fixture ruling: %v", err)
		}
		return blob
	}

	// The positive control first: the fixture parses, so every refusal below
	// is about the mutation and not about a broken base.
	if _, err := gateParseAdjudications(wellDoc(func(*gateAdjudication) {})); err != nil {
		t.Fatalf("a well-formed ruling was refused, so every row below proves nothing: %v", err)
	}
	// And the two shapes that mean "no rulings" and are NOT errors: an empty
	// list, and an absent key (null rulings).
	for _, ok := range []string{`{"rulings": []}`, `{}`} {
		if rulings, err := gateParseAdjudications([]byte(ok)); err != nil || len(rulings) != 0 {
			t.Fatalf("%q must parse as zero rulings; got %d (%v)", ok, len(rulings), err)
		}
	}

	for _, tc := range []struct {
		name string
		blob []byte
		want string
	}{
		{"not JSON at all", []byte("a ruling I typed into the wrong file\n"), "cannot be read"},
		{"a second document after the first", []byte(`{"rulings": []}{"rulings": []}`), "two documents"},
		{"an unknown field, which would otherwise be dropped in silence", []byte(`{"rulings": [], "expires": "v9.9.9"}`), "unknown field"},
		{"a version spelled without the v", wellDoc(func(a *gateAdjudication) { a.Version = "9.9.9" }), "matches none"},
		{"no surface", wellDoc(func(a *gateAdjudication) { a.Surface = " " }), "no identifiable finding"},
		{"no rule", wellDoc(func(a *gateAdjudication) { a.Rule = "" }), "no identifiable finding"},
		{"an empty scenario", wellDoc(func(a *gateAdjudication) { a.FailureScenario = "" }), "can never match a recorded finding"},
		{"an adjective where the scenario belongs", wellDoc(func(a *gateAdjudication) { a.FailureScenario = "very low" }), "can never match a recorded finding"},
		{"nobody ruled", wellDoc(func(a *gateAdjudication) { a.RuledBy = "" }), "who to ask"},
		{"an unreadable ruled_at", wellDoc(func(a *gateAdjudication) { a.RuledAt = "yesterday" }), "neither RFC 3339"},
		{"no reason", wellDoc(func(a *gateAdjudication) { a.Reason = "  " }), "an assertion, not a judgement"},
		{"two rulings for one finding identity", func() []byte {
			blob, err := json.Marshal(gateAdjudicationsDoc{Rulings: []gateAdjudication{well, well}})
			if err != nil {
				t.Fatalf("marshal the duplicate pair: %v", err)
			}
			return blob
		}(), "one ruling per release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rulings, err := gateParseAdjudications(tc.blob)
			if err == nil {
				t.Fatalf("a record the gate cannot stand behind was read as %d ruling(s); a malformed file is a FAILED gate, never \"no rulings\"", len(rulings))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not say %q:\n%v", tc.want, err)
			}
		})
	}
}

// TestGateARulingClearsExactlyTheFindingItQuotes is the identity discipline,
// all three directions at once, through the real verdict predicate:
//
//   - a ruling whose scenario matches byte for byte clears a misled finding
//     its agent judged blocking — the gate goes green — and the finding stays
//     on the receipt IN FULL with the ruling beside it in the outcome;
//   - the same ruling with one byte of scenario drift clears NOTHING, and the
//     verdict says in words that the finding has changed and needs re-ruling;
//   - the same ruling scoped to an earlier release clears nothing either, and
//     is not even stale — a ruling is never inherited across releases.
func TestGateARulingClearsExactlyTheFindingItQuotes(t *testing.T) {
	declared, current, green := gateEvaluateGreenFixture()
	finding := gateReceiptFinding("readme", "stale-count", gateConsequenceMisled, gateBlockingBlocks, "says 27, the registry holds 28")
	receipt := gateReceipt{Version: "v9.9.9", Surfaces: green, Findings: []gateFinding{finding}}
	ruling := gateAdjudicationFixture("v9.9.9", "readme", "stale-count")

	// Without any ruling, the fixture blocks — the positive control that the
	// clearing below is the ruling's doing and nobody else's.
	if verdict, _, err := receipt.evaluate(declared, current, nil); verdict != gateVerdictFailed || err == nil {
		t.Fatalf("the un-ruled fixture does not block (%s, %v), so nothing below is about clearing", verdict, err)
	}

	t.Run("the scenario matches and the ruling applies", func(t *testing.T) {
		verdict, outcome, err := receipt.evaluate(declared, current, []gateAdjudication{ruling})
		if verdict != gateVerdictPass || err != nil {
			t.Fatalf("a matching ruling did not clear the misled blocker: %s (%v)", verdict, err)
		}
		// NOTHING IS DELETED: the finding is still on the receipt, whole.
		if len(receipt.Findings) != 1 || receipt.Findings[0] != finding {
			t.Fatalf("evaluation altered the findings themselves; a cleared finding stays on the receipt in full: %+v", receipt.Findings)
		}
		// And the ruling rides beside it: who ruled, when, why.
		if len(outcome.Cleared) != 1 || outcome.Cleared[0].Finding != finding || outcome.Cleared[0].Ruling != ruling {
			t.Fatalf("the outcome does not carry the finding with its ruling beside it: %+v", outcome.Cleared)
		}
		if len(outcome.Stale) != 0 || len(outcome.NeedsReRuling) != 0 {
			t.Errorf("an applied ruling was also reported stale or mismatched: %+v", outcome)
		}
	})

	t.Run("the scenario differs and the finding keeps blocking", func(t *testing.T) {
		drifted := ruling
		drifted.FailureScenario = ruling.FailureScenario + " and the count moved again"
		verdict, outcome, err := receipt.evaluate(declared, current, []gateAdjudication{drifted})
		if verdict != gateVerdictFailed || err == nil {
			t.Fatalf("a ruling over a scenario the finding no longer carries cleared it anyway: %s (%v). A later round can reuse a slug for different substance, and this is the byte that catches it", verdict, err)
		}
		if !strings.Contains(err.Error(), "needs re-ruling") {
			t.Errorf("the verdict does not say the finding needs re-ruling; a human meeting this failure must be told a ruling exists and no longer describes the finding:\n%v", err)
		}
		if len(outcome.NeedsReRuling) != 1 || len(outcome.Cleared) != 0 || len(outcome.Stale) != 0 {
			t.Errorf("the outcome does not report exactly one needs-re-ruling pair: %+v", outcome)
		}
	})

	t.Run("a ruling from an earlier release does not apply", func(t *testing.T) {
		inherited := ruling
		inherited.Version = "v9.9.8"
		verdict, outcome, err := receipt.evaluate(declared, current, []gateAdjudication{inherited})
		if verdict != gateVerdictFailed || err == nil {
			t.Fatalf("a ruling made for v9.9.8 cleared a v9.9.9 finding: %s (%v). Rulings are version-scoped, never inherited", verdict, err)
		}
		if !outcome.empty() {
			t.Errorf("an out-of-scope ruling appeared in the outcome (%+v); another release's rulings are history, not this release's report", outcome)
		}
	})
}

// TestGateAnActsWronglyRulingRefusesTheWholeFile is the absolute rule: no
// ruling clears an acts-wrongly finding, and a file carrying one that tries is
// refused whole — including its innocent rulings, which is asserted here by a
// second, otherwise-applicable ruling that must NOT apply out of the refused
// file.
func TestGateAnActsWronglyRulingRefusesTheWholeFile(t *testing.T) {
	declared, current, green := gateEvaluateGreenFixture()
	wrong := gateReceiptFinding("site", "recovery-destroys-backup", gateConsequenceActsWrongly, gateBlockingDeferrable, "step 4 overwrites the hook before the backup step runs")
	misled := gateReceiptFinding("readme", "stale-count", gateConsequenceMisled, gateBlockingBlocks, "says 27, the registry holds 28")
	receipt := gateReceipt{Version: "v9.9.9", Surfaces: green, Findings: []gateFinding{wrong, misled}}

	overreach := gateAdjudicationFixture("v9.9.9", "site", "recovery-destroys-backup")
	innocent := gateAdjudicationFixture("v9.9.9", "readme", "stale-count")

	verdict, outcome, err := receipt.evaluate(declared, current, []gateAdjudication{innocent, overreach})
	if verdict != gateVerdictFailed || err == nil {
		t.Fatalf("a file whose ruling names an acts-wrongly finding did not fail the gate: %s (%v)", verdict, err)
	}
	if !strings.Contains(err.Error(), "refused in full") || !strings.Contains(err.Error(), gateConsequenceActsWrongly) {
		t.Errorf("the refusal must be of the whole file and must name the consequence that is not anyone's to clear:\n%v", err)
	}
	// The innocent ruling is suspended with the file: the misled finding it
	// would have cleared still blocks, in the agent's own clause.
	if !strings.Contains(err.Error(), "judged it worth stopping the release for") {
		t.Errorf("the innocent ruling applied out of a refused file — the misled finding no longer blocks:\n%v", err)
	}
	if !outcome.empty() {
		t.Errorf("a refused file still produced an outcome (%+v); no ruling in it applies", outcome)
	}

	// And the acts-wrongly finding itself still blocks as itself, whatever the
	// file said — the refusal is IN ADDITION to the finding, not instead.
	if !strings.Contains(err.Error(), "no override") {
		t.Errorf("the acts-wrongly finding's own unconditional clause is missing:\n%v", err)
	}
}

// TestGateAStaleRulingIsReportedAndFailsNothing: a ruling matching no finding
// at all rides the outcome visibly — most often its finding was ALSO fixed in
// the tree, which is a better ending than the ruling — and the gate does not
// fail on it alone.
func TestGateAStaleRulingIsReportedAndFailsNothing(t *testing.T) {
	declared, current, green := gateEvaluateGreenFixture()
	receipt := gateReceipt{Version: "v9.9.9", Surfaces: green}
	stale := gateAdjudicationFixture("v9.9.9", "readme", "a-rule-no-round-raised")

	verdict, outcome, err := receipt.evaluate(declared, current, []gateAdjudication{stale})
	if verdict != gateVerdictPass || err != nil {
		t.Fatalf("a stale ruling failed an otherwise green gate: %s (%v). It matches nothing, so there is nothing for it to wrongly clear", verdict, err)
	}
	if len(outcome.Stale) != 1 || outcome.Stale[0] != stale {
		t.Fatalf("the stale ruling is not reported in the outcome; a ruling that silently stopped mattering is one its author still believes in: %+v", outcome)
	}
}

// TestGateARulingExcusesAFailedSurfaceVerdictOnlyWithItsFindingsCleared covers
// the coupling the answer schema enforces from the other side: an agent that
// raised a blocking finding reported its surface FAILED (a PASS listing a
// blocking finding is refused at recording), so clearing the finding must also
// excuse THAT surface's FAILED in the green computation — for the green
// computation only, the receipt keeps the agent's verdict — and must excuse
// nothing else: a FAILED surface with no cleared finding, or with any finding
// still blocking, stays red.
func TestGateARulingExcusesAFailedSurfaceVerdictOnlyWithItsFindingsCleared(t *testing.T) {
	declared, current, _ := gateEvaluateGreenFixture()
	finding := gateReceiptFinding("readme", "stale-count", gateConsequenceMisled, gateBlockingBlocks, "says 27, the registry holds 28")
	failed := []gateSurfaceVerdict{
		{Surface: "readme", Verdict: gateVerdictFailed, Fingerprint: "sha256:aa"},
		{Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"},
	}
	ruling := gateAdjudicationFixture("v9.9.9", "readme", "stale-count")

	receipt := gateReceipt{Version: "v9.9.9", Surfaces: failed, Findings: []gateFinding{finding}}
	verdict, _, err := receipt.evaluate(declared, current, []gateAdjudication{ruling})
	if verdict != gateVerdictPass || err != nil {
		t.Fatalf("the agent's FAILED verdict still blocks after the one finding justifying it was cleared: %s (%v). The verdict and the finding are one judgement recorded twice, and the ruling rules on the judgement", verdict, err)
	}
	if receipt.Surfaces[0].Verdict != gateVerdictFailed {
		t.Fatalf("evaluation rewrote the receipt's own surface verdict; the record is the agent's, the excusal lives only in the green computation")
	}

	// A FAILED surface with NO finding cleared is not excused: the ruling is
	// about some other surface's finding.
	elsewhere := gateAdjudicationFixture("v9.9.9", "site", "stale-count")
	other := gateReceipt{Version: "v9.9.9", Surfaces: failed, Findings: []gateFinding{finding}}
	if verdict, _, err := other.evaluate(declared, current, []gateAdjudication{elsewhere}); verdict != gateVerdictFailed || err == nil {
		t.Fatalf("a FAILED surface was excused by a ruling that cleared none of its findings: %s (%v)", verdict, err)
	}

	// And a FAILED surface with one finding cleared and another still blocking
	// stays red: the excusal requires nothing of the surface's left blocking.
	second := gateReceiptFinding("readme", "wrong-flag", gateConsequenceMisled, gateBlockingBlocks, "--strict is not described")
	both := gateReceipt{Version: "v9.9.9", Surfaces: failed, Findings: []gateFinding{finding, second}}
	if verdict, _, err := both.evaluate(declared, current, []gateAdjudication{ruling}); verdict != gateVerdictFailed || err == nil {
		t.Fatalf("a surface with a finding still blocking was excused because a DIFFERENT finding was cleared: %s (%v)", verdict, err)
	}
}

// TestGateReadAdjudicationsReadsTheTreeAndFailsClosed is the loader against a
// real repository: the record is read out of the branch's TREE (an uncommitted
// ruling must not clear anything — it would be absent from the tag), a tree
// with no record is zero rulings, and a malformed record is an error carrying
// errGateUncheckable — a FAILED gate, never "no rulings".
func TestGateReadAdjudicationsReadsTheTreeAndFailsClosed(t *testing.T) {
	work := gateFixtureRepo(t)

	// No record in the tree: zero rulings, no error — the fail-closed absence.
	rulings, err := gateReadAdjudications(work, "release")
	if err != nil || len(rulings) != 0 {
		t.Fatalf("a tree with no %s must read as zero rulings, not as %d ruling(s) (%v)", gateAdjudicationsFile, len(rulings), err)
	}

	// A record in the WORKING COPY only is still zero rulings: the tree is
	// what ships, and a ruling that is not on the branch clears nothing.
	ruling := gateAdjudicationFixture("v9.9.9", "readme", "stale-count")
	blob, err := json.MarshalIndent(gateAdjudicationsDoc{Rulings: []gateAdjudication{ruling}}, "", "  ")
	if err != nil {
		t.Fatalf("marshal the ruling: %v", err)
	}
	gateWrite(t, work, gateAdjudicationsFile, string(blob)+"\n")
	rulings, err = gateReadAdjudications(work, "release")
	if err != nil || len(rulings) != 0 {
		t.Fatalf("an uncommitted %s was read as %d ruling(s) (%v); the working copy can carry an edit the tag will not, which is exactly the shape this loader must not honour", gateAdjudicationsFile, len(rulings), err)
	}

	// Committed, it is read — from the branch named, whole.
	gateTestGit(t, work, "add", "-A")
	gateTestGit(t, work, "commit", "-qm", "gate: record a ruling")
	rulings, err = gateReadAdjudications(work, "release")
	if err != nil || len(rulings) != 1 || rulings[0] != ruling {
		t.Fatalf("the committed record did not read back as itself: %d ruling(s), %+v (%v)", len(rulings), rulings, err)
	}

	// Malformed and committed: errGateUncheckable, so the driver classifies it
	// as a check that could not be made rather than as a no.
	gateWrite(t, work, gateAdjudicationsFile, "{\"rulings\": [{\"version\": \"v9.9.9\"}]}\n")
	gateTestGit(t, work, "add", "-A")
	gateTestGit(t, work, "commit", "-qm", "gate: damage the record")
	if _, err := gateReadAdjudications(work, "release"); !errors.Is(err, errGateUncheckable) {
		t.Fatalf("a malformed record read as something other than an uncheckable gate: %v", err)
	}
}

// TestTheDriverHonoursARulingAndCarriesItOnTheRecord is the mechanism end to
// end, through the real driver against a fixture forge: a release blocked by
// one misled finding is refused at D1; the same release with the ruling
// COMMITTED ON THE BRANCH publishes; and the run carries both halves of the
// record — the finding, whole, on the receipt, and the ruling beside it in the
// outcome. This is the test that notices if the driver ever stops loading
// gate/adjudications.json, which would silently turn every recorded ruling
// into paper.
func TestTheDriverHonoursARulingAndCarriesItOnTheRecord(t *testing.T) {
	const version = "v9.9.9"
	finding := gateFinding{
		Surface:         "readme",
		Rule:            "stale-count",
		Consequence:     gateConsequenceMisled,
		FailureScenario: "a reader deciding whether to depend on this project reads a register that still calls a shipped feature deferred",
		Blocking:        gateBlockingBlocks,
		Detail:          "the count is one release old",
	}
	evidence := gateDriverGreenEvidence()
	evidence.findings = []gateFinding{finding}
	// The agent that raised a blocking finding reported its surface FAILED —
	// the coupled shape the recorder enforces — so this exercises the excusal
	// path, not just the finding path.
	evidence.verdicts = []gateSurfaceVerdict{{Surface: "readme", Verdict: gateVerdictFailed, Fingerprint: "sha256:readme"}}

	t.Run("without the ruling the release is refused at D1", func(t *testing.T) {
		repo := gateDriverFixture(t, version)
		mainWas := gateDriverRemoteHead(t, repo, repo.Base)
		run := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, evidence)
		if run.Err == nil || run.Failed != "D1" {
			t.Fatalf("a release carrying an unruled blocking finding got past D1: failed at %q (%v)", run.Failed, run.Err)
		}
		gateDriverAssertNothingPublished(t, repo, version, mainWas)
	})

	t.Run("with the ruling committed on the branch it publishes, finding and ruling both on the record", func(t *testing.T) {
		repo := gateDriverFixture(t, version)
		ruling := gateAdjudication{
			Version:         version,
			Surface:         finding.Surface,
			Rule:            finding.Rule,
			FailureScenario: finding.FailureScenario,
			RuledBy:         "A. Maintainer",
			RuledAt:         "2026-08-20",
			Reason:          "the register's staleness misleads nobody into acting; the correction is already on the branch for the next release",
		}
		blob, err := json.MarshalIndent(gateAdjudicationsDoc{Rulings: []gateAdjudication{ruling}}, "", "  ")
		if err != nil {
			t.Fatalf("marshal the ruling: %v", err)
		}
		gateWrite(t, repo.Dir, gateAdjudicationsFile, string(blob)+"\n")
		gateTestGit(t, repo.Dir, "add", "-A")
		gateTestGit(t, repo.Dir, "commit", "-qm", "gate: rule the stale-count finding non-blocking for v9.9.9")

		// The plan is built AFTER the ruling lands, because the CI evidence
		// must name the branch head that will be released.
		run := gateDriverPublish(gateDriverAuthorized(t, repo, version), repo, evidence)
		if run.Err != nil {
			t.Fatalf("a release whose one blocker carries a matching committed ruling was refused at %s:\n%v", run.Failed, run.Err)
		}
		if tag := gateDriverRemoteTag(t, repo, version); tag == "" {
			t.Fatal("the driver reported success and origin carries no tag; nothing was published")
		}

		// The finding is ON the receipt, whole — cleared is not deleted.
		if len(run.Receipt.Findings) != 1 || run.Receipt.Findings[0] != finding {
			t.Fatalf("the cleared finding is not on the receipt in full: %+v", run.Receipt.Findings)
		}
		// And the ruling is beside it: who ruled, when, why.
		if len(run.Adjudicated.Cleared) != 1 ||
			run.Adjudicated.Cleared[0].Finding != finding ||
			run.Adjudicated.Cleared[0].Ruling != ruling {
			t.Fatalf("the run does not carry the finding with its ruling beside it: %+v", run.Adjudicated)
		}
	})
}
