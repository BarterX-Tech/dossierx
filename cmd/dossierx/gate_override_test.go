// gate_override_test.go is THE OVERRIDE RECORD: the one way a finding the human
// has ruled non-blocking stops failing the gate without being deleted.
//
// WHY IT EXISTS, and why it was deliberately absent until now.
// gate_stage3_test.go has carried the argument since the join was written: the
// receipt has no field for a human waving a finding through, and evaluate()
// returned FAILED whenever any finding was present, so the only way past a
// finding somebody has judged non-blocking was to delete it from the record —
// which leaves an adjudicated finding and a finding nobody raised byte-identical
// to the next release. That was defensible while nothing needed it. The v0.5.2
// gate's fifth round made it routine: 58 findings, two of which are the reading
// agents reporting that the harness could not withhold the tools the design
// grants them — a fact about the runtime that no edit to this tree can fix, and
// that therefore recurs on every round for ever.
//
// WHAT THE PRIORITY MATRIX LEFT FOR IT TO DO. gate_priority_test.go now decides
// which findings block at all, and P2/P3 no longer need a ruling — which is what
// keeps this record rare enough to stay meaningful. Two cells changed for this
// file. P1 is the band an override still clears, unchanged and with every refusal
// below in force. P0 — a client-shipped surface that makes its reader ACT wrongly
// — cannot be cleared here at all: a signature does not make a wrong install
// command work, and gateApplyOverrides refuses the record that tries rather than
// the finding, so the file is visibly the thing that failed the evaluation.
//
// THE THREE PROPERTIES, which are the whole design. An override that lacks any
// one of them is worse than no override at all, because it makes a green receipt
// mean less while looking like it means the same.
//
//	HARDER TO WRITE THAN A FIX. An override names a DIGEST over the finding's own
//	    text. Change the finding — or fix the defect so the wording moves — and
//	    the override matches nothing and is REFUSED as stale. It cannot be copied
//	    between findings and cannot survive the thing it excuses being reworded.
//	    What it does NOT do is make a pre-written ruling impossible — anyone can
//	    compute a digest over text they predict — but a prediction that is not
//	    word-for-word right matches nothing in the run and fails the gate, which is
//	    the property that matters.
//	VISIBLE ON THE RECEIPT'S FACE. Applied overrides are recorded ON the receipt,
//	    with who ruled and why. A receipt cleared by override is not a clean
//	    receipt wearing the same clothes: it says so, in the document the driver
//	    prints and the run record keeps.
//	NEVER INHERITED. Every override names the release it was ruled for. A record
//	    left behind by v0.5.2 is refused by v0.5.3 rather than silently carried,
//	    for the reason the subject freeze exists: a decision made about one
//	    release is not evidence about the next one.
//
// AND THE FOURTH, WHICH IS NOT A PROPERTY OF THE FIELD BUT OF THE FILE:
// gate/overrides.json is TRACKED. Every override is a line in a reviewed diff,
// which is the same argument gate/subject.json's freeze rests on. A gate whose
// escape hatch is invisible to git is a gate with no escape hatch worth having.
//
// WHAT AN OVERRIDE IS NOT. It is not a priority threshold. The matrix decides
// which findings the gate stops for; an override is a named person's decision
// about ONE specific finding, recorded in their words, and it is asked for only
// after the matrix has already said this one is worth stopping for. The two must
// not be confused, which is why this file refuses an override that carries no
// reason: a rule that fires automatically is a threshold with extra steps. The
// one place they meet is the P0 refusal, and that is the matrix removing a cell
// from this record's reach rather than this record acting on a rank.
//
// Same shape as the rest of the gate: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gateOverrideFile is the tracked record, beside the subject freeze and for the
// same reason.
const gateOverrideFile = "gate/overrides.json"

// gateOverride is one human ruling on one finding.
type gateOverride struct {
	// Version is the release this ruling was made for. An override naming
	// another release is refused rather than applied — see NEVER INHERITED.
	Version string `json:"version"`
	// Finding is the digest of the finding this rules on, and it is what the
	// match is made against. Surface and Rule are carried beside it for a human
	// reading the file, and are checked against the finding the digest names so
	// the record cannot describe one finding while clearing another.
	Finding string `json:"finding"`
	Surface string `json:"surface"`
	Rule    string `json:"rule"`
	// RuledBy is the person who made the call. Not an agent, and not a role: a
	// name, so the record answers "who decided this" a year later.
	RuledBy string `json:"ruled_by"`
	// Reason is their words. Refused when empty, because an override nobody had
	// to justify is the default path under time pressure.
	Reason string `json:"reason"`
	// PromoteTo turns this entry from a clearance into a PROMOTION: the finding
	// stays, nothing is cleared, and it is partitioned — by evaluate(), and by the
	// deferred projection — at this band rather than at the one the matrix computed.
	//
	// IT ONLY EVER RAISES. gateApplyOverrides refuses a band equal to or below the
	// matrix's, and the sentence to remember is that the matrix is the FLOOR a
	// ruling can raise and never a ceiling it lowers. A record that could lower a
	// rank would hand the party whose work is under review a way to decide their own
	// finding does not stop the release — which is precisely the free-text severity
	// this whole design replaced, wearing a signature.
	//
	// AN ENTRY IS ONE THING OR THE OTHER. A non-empty PromoteTo means promote, and
	// the clearance below never runs for it; one finding may still carry only one
	// entry, so a promoted finding cannot also be cleared by a second. That is what
	// makes "promote a P2 to P0" stick: P0 is the cell no ruling reaches, so the
	// promotion is the last word on that finding until the tree is fixed.
	PromoteTo string `json:"promote_to,omitempty"`
}

// gateFindingDigest is the identity an override names.
//
// It covers the finding's SUBSTANCE — surface, rule and the detail text — and
// nothing else, and the input is UNCHANGED by the schema that replaced severity
// with a consequence, a failure scenario and a derived priority. That is
// deliberate and it is the reason the version tag in the hashed preamble did not
// move: an override is a ruling about a defect, and re-classifying that defect —
// deciding the same wrong sentence misleads rather than makes a reader act wrongly
// — must not orphan the ruling somebody already made about it. Fold the
// consequence in and every ruling dies the day a reviewer changes one word that
// says nothing new about the defect itself.
//
// The wording of the finding is a different matter and stays inside: a detail that
// moved is a defect nobody is reading about any more, and the ruling has to be
// made again. So the rule is exactly "the ruling survives a re-classification and
// dies with a re-wording".
//
// It is also why the Digest field on gateFinding is excluded from its own input:
// the receipt stamps that field from this function, so hashing it would make the
// value depend on itself and no two runs would agree. Priority is excluded for
// both reasons at once — it is derived, and it is a classification.
func gateFindingDigest(f gateFinding) string {
	h := sha256.New()
	fmt.Fprintf(h, "dossierx-gate-finding\x00v1\x00%s\x00%s\x00%s", f.Surface, f.Rule, f.Detail)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// gateLoadOverrides reads the tracked record.
//
// An ABSENT file is the ordinary case and yields no overrides. An unreadable or
// malformed one is a refusal: a record that cannot be parsed is not an empty
// record, and treating it as one would clear nothing while looking like it
// cleared nothing — the difference matters when somebody is asking why their
// override did not apply.
func gateLoadOverrides(root string) ([]gateOverride, error) {
	path := filepath.Join(root, filepath.FromSlash(gateOverrideFile))
	blob, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gate overrides: %s exists and could not be read (%w). An unreadable record is not an empty one", gateOverrideFile, err)
	}
	var doc struct {
		Overrides []gateOverride `json:"overrides"`
	}
	if err := json.Unmarshal(blob, &doc); err != nil {
		return nil, fmt.Errorf("gate overrides: %s is not readable as JSON (%w). Repair it or delete it; a malformed record is refused rather than skipped", gateOverrideFile, err)
	}
	return doc.Overrides, nil
}

// gateFindingPromotions is the digest-to-band map of every entry that PROMOTES,
// read off the record with no judgement made about it.
//
// It is deliberately separate from gateApplyOverrides, which is where every
// refusal lives, because two callers want two different things. evaluate() wants
// the refusals; gateRecordReceipt wants only to project the deferred ledger at the
// bands the human raised, and it runs BEFORE any verdict is computed. A record
// whose promotion is malformed — a band outside the four, a digest matching
// nothing — promotes nothing here and is refused by evaluate() a moment later, so
// the two orderings cannot combine into a quiet pass: the run fails, and the
// ledger it wrote is overwritten by the next recording either way.
func gateFindingPromotions(overrides []gateOverride) map[string]string {
	out := map[string]string{}
	for _, o := range overrides {
		if to := strings.TrimSpace(o.PromoteTo); to != "" {
			out[o.Finding] = to
		}
	}
	return out
}

// gatePromoted returns a copy of findings in which every finding a promotion names
// carries the band it was promoted to.
//
// It is a COPY. What the matrix computed stays on the receipt's own findings, for
// gateVerdictsAfterOverrides's reason one field along: what the agents reported and
// the matrix ranked, and what a human then decided, are different questions, and a
// record that answers the first with the second leaves nobody able to see that a
// ruling moved anything.
//
// A promotion naming a band outside the four is applied here as written, and that
// is safe rather than sloppy: the only two consumers are the priority partition,
// where an unrecognised band is UNRANKED and therefore blocking, and the deferred
// projection, where it is not P2/P3 and therefore not deferred. Both directions
// fail towards the release stopping, and gateApplyOverrides refuses the record by
// name in the same run.
func gatePromoted(findings []gateFinding, promotions map[string]string) []gateFinding {
	if len(promotions) == 0 {
		return findings
	}
	out := append([]gateFinding(nil), findings...)
	for i := range out {
		if to, ok := promotions[gateFindingDigest(out[i])]; ok {
			out[i].Priority = to
		}
	}
	return out
}

// gateApplyOverrides matches a receipt's findings against the human's rulings.
//
// It returns the findings that remain — the ones nobody cleared, at the band each
// one is to be judged at — and the overrides that were actually applied. EVERY
// other outcome is an error, and that is the point: a stale override, an override
// for another release, one with no reason, one naming a finding that is not there,
// two rulings on the same finding. None of those is a no-op. Each one means the
// record and the run disagree about what was decided, and a gate that shrugged
// at that would be carrying a document nobody can trust into the one decision it
// exists to inform.
//
// THE REMAINING FINDINGS CARRY THE PROMOTED BAND, not the computed one. A ruling
// that raises a P2 to P0 has to reach the partition evaluate() makes, or the
// promotion is a line in a file that changes nothing — which is the defect this
// gate met inside this very record one commit before the loader was wired up.
func gateApplyOverrides(receipt gateReceipt, overrides []gateOverride) (remaining []gateFinding, applied []gateOverride, err error) {
	byDigest := make(map[string]gateFinding, len(receipt.Findings))
	for _, f := range receipt.Findings {
		byDigest[gateFindingDigest(f)] = f
	}

	var problems []string
	seen := map[string]bool{}
	cleared := map[string]bool{}
	promoted := map[string]string{}

	for _, o := range overrides {
		switch {
		case strings.TrimSpace(o.Finding) == "":
			problems = append(problems, "an override names no finding digest, so nothing can say which finding it rules on")
			continue
		case strings.TrimSpace(o.RuledBy) == "":
			problems = append(problems, fmt.Sprintf("the override for %s names nobody in `ruled_by`; an unattributed ruling is not a ruling", o.Finding))
			continue
		case strings.TrimSpace(o.Reason) == "":
			problems = append(problems, fmt.Sprintf("the override for %s carries no reason. An override nobody had to justify becomes the default path under time pressure, which is the failure this record is built to avoid", o.Finding))
			continue
		case seen[o.Finding]:
			problems = append(problems, fmt.Sprintf("finding %s is ruled on twice; two rulings on one finding leave no answer to \"what was decided\"", o.Finding))
			continue
		}
		seen[o.Finding] = true

		if o.Version != receipt.Version {
			problems = append(problems, fmt.Sprintf(
				"the override for %s was ruled for %s and this release is %s. An override is never inherited: a decision made about one release is not evidence about the next, for the reason the subject freeze exists. Re-rule it for %s if it still applies",
				o.Finding, o.Version, receipt.Version, receipt.Version))
			continue
		}

		f, ok := byDigest[o.Finding]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"the override for %s matches no finding in this run. Either the finding was fixed — in which case delete the override, it has done its job — or its wording moved, in which case the ruling was made about text nobody is reading any more and has to be made again. A stale override is refused rather than ignored, because ignoring it would let a record accumulate rulings that clear nothing",
				o.Finding))
			continue
		}
		if o.Surface != f.Surface || o.Rule != f.Rule {
			problems = append(problems, fmt.Sprintf(
				"the override for %s says it rules on %s/%s; that digest is %s/%s. The digest is what matches, so this record would clear one finding while describing another to whoever reads it",
				o.Finding, o.Surface, o.Rule, f.Surface, f.Rule))
			continue
		}
		// A FINDING NOTHING RANKED IS NOT A FINDING TO RULE ON, in either direction.
		// gatePartitionByPriority treats an empty priority — and any band the matrix
		// does not know — as UNRANKED and blocking, precisely because such a finding
		// reached the receipt by a path that skipped the ranking. There is nothing for
		// a human to weigh: the matrix never crossed this one, so a clearance would be
		// waving through a defect nobody classified, and a promotion would be raising
		// a rank that does not exist.
		//
		// It is refused rather than silently unapplied for the P0 refusal's reason,
		// and there is a measured hole behind it: before this refusal existed, an
		// entry naming an unranked finding CLEARED it, and a receipt whose only
		// finding was unranked evaluated PASS — while three comments in this gate said
		// an unranked finding always blocks.
		if _, ranked := gatePriorityRank[strings.TrimSpace(f.Priority)]; !ranked {
			problems = append(problems, fmt.Sprintf(
				"the override for %s rules on %s/%s, which carries no priority: it came by a path that skipped the ranking, which is a producer to fix and not a finding to rule on. A finding is ranked where it is filed, from the surface's reach_class crossed with its own consequence; until this one is, there is no band for a ruling to clear or to raise. Delete this entry and fix the producer",
				o.Finding, f.Surface, f.Rule))
			continue
		}

		// A PROMOTING ENTRY IS NOT A CLEARANCE, and it is handled before the P0
		// refusal below because that refusal is about clearing: promoting TO P0 is
		// the opposite move and is exactly what a human is allowed to do. The matrix
		// is a floor, so the one thing checked here is that the ruling raises.
		if to := strings.TrimSpace(o.PromoteTo); to != "" {
			rank, known := gatePriorityRank[to]
			if !known {
				problems = append(problems, fmt.Sprintf(
					"the override for %s promotes to %q, which is not one of %s. `promote_to` names the band this finding is to be judged in, and a band nothing can look up is judged as unranked — which blocks the release while saying a producer is broken, rather than saying what the human decided",
					o.Finding, to, strings.Join(gatePriorities, ", ")))
				continue
			}
			if rank >= gatePriorityRank[f.Priority] {
				problems = append(problems, fmt.Sprintf(
					"the override for %s promotes %s/%s from %s to %s, which is not a promotion: the matrix is the floor a ruling can raise, never a ceiling it lowers. A record that could lower a rank hands the party whose work is under review a way to decide their own finding does not stop the release, which is the free-text severity this design replaced. Name a band above %s, or delete this entry",
					o.Finding, f.Surface, f.Rule, f.Priority, to, f.Priority))
				continue
			}
			promoted[o.Finding] = to
			applied = append(applied, o)
			continue
		}

		// THE ONE CELL NO RULING REACHES. A P0 is a client-shipped surface that
		// makes its reader ACT wrongly — an install line that installs the wrong
		// thing, a documented flag that does not exist — and a signature does not
		// make the command work. Every other property of this record assumes a
		// human is deciding between "this is worth stopping the release for" and
		// "this is worth recording and shipping"; at P0 there is no second option
		// to decide between, so offering one is offering the wrong choice.
		//
		// It is a REFUSAL of the record and not a silently unapplied ruling: the
		// person who wrote it has to be told that this one is not theirs to rule
		// on, and a record whose entry did nothing would tell them nothing.
		if f.Priority == gatePriorityP0 {
			problems = append(problems, fmt.Sprintf(
				"the override for %s rules on %s/%s, which is %s: a client-shipped surface that makes its reader ACT wrongly. That cannot be waved through by signature — a ruling does not make a wrong command work — so it is fixed and the gate re-run. Delete this entry",
				o.Finding, f.Surface, f.Rule, gatePriorityP0))
			continue
		}
		cleared[o.Finding] = true
		applied = append(applied, o)
	}

	// What is left is everything nobody cleared, at the band it is to be judged at:
	// gatePromoted stamps the raised ones and leaves the rest as the matrix computed
	// them.
	for _, f := range gatePromoted(receipt.Findings, promoted) {
		if !cleared[gateFindingDigest(f)] {
			remaining = append(remaining, f)
		}
	}

	sort.Slice(applied, func(i, j int) bool {
		if applied[i].Surface != applied[j].Surface {
			return applied[i].Surface < applied[j].Surface
		}
		return applied[i].Rule < applied[j].Rule
	})

	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, nil, fmt.Errorf("gate overrides: %s", strings.Join(problems, "\n"))
	}
	return remaining, applied, nil
}

// gateVerdictsAfterOverrides returns the surface verdicts as they stand once the
// findings that still stop the release are known: a surface with none of them
// left reads PASS, and one holding a single blocking finding does not.
//
// WHAT `remaining` MEANS HERE WIDENED, and the function did not. It used to be
// "the findings no ruling covered"; evaluate() now passes "the findings that
// BLOCK" — unruled P0/P1 plus anything nothing could rank — so a surface whose
// only findings are deferred to gate/deferred.json reads PASS for the same reason
// one whose findings were all ruled on does: nothing is left that stops the
// release. The arithmetic is identical, which is why there is one function and not
// two; a second one would be a second answer to "is this surface still failing".
//
// It builds a new slice rather than editing the receipt's, because the receipt
// records what the AGENTS reported and that must stay readable next to what the
// human decided and what the matrix deferred. Different questions, different
// answers, all kept.
func gateVerdictsAfterOverrides(receipt gateReceipt, remaining []gateFinding) []gateSurfaceVerdict {
	stillFailing := map[string]bool{}
	for _, f := range remaining {
		stillFailing[f.Surface] = true
	}
	reported := map[string]bool{}
	for _, f := range receipt.Findings {
		reported[f.Surface] = true
	}

	out := make([]gateSurfaceVerdict, 0, len(receipt.Surfaces))
	for _, v := range receipt.Surfaces {
		if v.Verdict != gateVerdictPass && reported[v.Surface] && !stillFailing[v.Surface] {
			v.Verdict = gateVerdictPass
		}
		out = append(out, v)
	}
	return out
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// gateOverrideFinding is one finding at P1 — the band a ruling can reach.
//
// EVERY FIXTURE IN THIS FILE IS P1 ON PURPOSE. The tests below are about the
// record's own refusals — stale, unattributed, unreasoned, inherited, doubled —
// and each of those has to fire on a finding an override is otherwise allowed to
// clear, or the test would pass on the P0 refusal instead and prove nothing about
// the property it names. The one test that wants a P0 is in
// gate_priority_test.go, where the cell it is about lives.
func gateOverrideFinding(surface, rule, detail string) gateFinding {
	return gateFinding{
		Surface:         surface,
		Rule:            rule,
		Consequence:     gateConsequenceMisled,
		FailureScenario: "a reader of " + surface + " believes something about this release that is not true, and plans around it",
		Detail:          detail,
		Priority:        gatePriorityP1,
	}
}

func gateOverrideReceipt(findings ...gateFinding) gateReceipt {
	surfaces := map[string]bool{}
	for _, f := range findings {
		surfaces[f.Surface] = true
	}
	r := gateReceipt{Version: "v0.5.2", Findings: findings}
	for name := range surfaces {
		r.Surfaces = append(r.Surfaces, gateSurfaceVerdict{Surface: name, Verdict: gateVerdictFailed, Fingerprint: "sha256:" + name})
	}
	sort.Slice(r.Surfaces, func(i, j int) bool { return r.Surfaces[i].Surface < r.Surfaces[j].Surface })
	return r
}

func gateOverrideFor(f gateFinding, mutate func(*gateOverride)) gateOverride {
	o := gateOverride{
		Version: "v0.5.2",
		Finding: gateFindingDigest(f),
		Surface: f.Surface,
		Rule:    f.Rule,
		RuledBy: "Nitin Khanna",
		Reason:  "the harness cannot withhold tools from a subagent; recorded rather than deleted",
	}
	if mutate != nil {
		mutate(&o)
	}
	return o
}

func TestGateOverrideClearsOnlyTheFindingItNames(t *testing.T) {
	ruled := gateOverrideFinding("contributing", "harness-did-not-withhold-tools", "the grant was not enforced")
	unruled := gateOverrideFinding("readme", "one-envelope-claim", "serve is not one envelope")
	receipt := gateOverrideReceipt(ruled, unruled)

	remaining, applied, err := gateApplyOverrides(receipt, []gateOverride{gateOverrideFor(ruled, nil)})
	if err != nil {
		t.Fatalf("an honest override was refused: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("applied %d override(s), want 1", len(applied))
	}
	if len(remaining) != 1 || remaining[0].Rule != unruled.Rule {
		t.Fatalf("remaining findings are %v; only the unruled one should survive", remaining)
	}

	// The ruled surface reads PASS; the unruled one does not. Both are computed
	// beside the receipt's own record of what the agents said, which is untouched.
	after := gateVerdictsAfterOverrides(receipt, remaining)
	got := map[string]string{}
	for _, v := range after {
		got[v.Surface] = v.Verdict
	}
	if got["contributing"] != gateVerdictPass {
		t.Errorf("the surface whose only finding was ruled on reads %q, want %s", got["contributing"], gateVerdictPass)
	}
	if got["readme"] == gateVerdictPass {
		t.Error("a surface holding an unruled finding was cleared; an override clears findings, and a surface reads PASS only when none of its findings is left unruled")
	}
	for _, v := range receipt.Surfaces {
		if v.Verdict != gateVerdictFailed {
			t.Errorf("the receipt's own record of surface %q was rewritten to %q; what the agents reported has to stay readable beside what the human decided", v.Surface, v.Verdict)
		}
	}
}

func TestGateOverrideIsHarderToWriteThanAFix(t *testing.T) {
	f := gateOverrideFinding("site", "text-parity-overclaim", "byte-for-byte identical to v0.2.0's output")
	receipt := gateOverrideReceipt(f)
	override := gateOverrideFor(f, nil)

	if _, _, err := gateApplyOverrides(receipt, []gateOverride{override}); err != nil {
		t.Fatalf("the override did not apply to the finding it was written for: %v", err)
	}

	// The defect is fixed, so the finding's wording moves — or it stops being
	// reported at all. Either way the ruling is about text nobody is reading.
	moved := f
	moved.Detail = "for every command that existed in v0.2.0 it is byte-for-byte what it was then"
	_, _, err := gateApplyOverrides(gateOverrideReceipt(moved), []gateOverride{override})
	if err == nil {
		t.Fatal("an override written against the OLD wording still applied after the finding moved; it must go stale with the text it excuses, or it is a permanent waiver written once")
	}
	if !strings.Contains(err.Error(), "matches no finding") {
		t.Errorf("the refusal does not say the override matches nothing:\n%v", err)
	}

	// And the same override against a run where the finding is simply gone.
	if _, _, err := gateApplyOverrides(gateReceipt{Version: "v0.5.2"}, []gateOverride{override}); err == nil {
		t.Error("an override survived a run in which its finding was fixed; a record that keeps rulings for findings nobody raises accumulates permanent waivers")
	}
}

func TestGateOverrideIsNeverInherited(t *testing.T) {
	f := gateOverrideFinding("release-procedure", "tool-grant-was-not-enforced", "the harness held file and shell tools")
	next := gateOverrideReceipt(f)
	next.Version = "v0.5.3"

	_, _, err := gateApplyOverrides(next, []gateOverride{gateOverrideFor(f, nil)}) // ruled for v0.5.2
	if err == nil {
		t.Fatal("a v0.5.2 ruling cleared the identical finding in v0.5.3. An override has to be re-made per release, or the first release to meet a recurring finding waives it for every release after")
	}
	for _, want := range []string{"never inherited", "v0.5.3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%v", want, err)
		}
	}
}

func TestGateOverrideRefusesEveryRecordThatDecidesNothing(t *testing.T) {
	f := gateOverrideFinding("readme", "stage-order", "the gate is not the last step")
	receipt := gateOverrideReceipt(f)

	for _, tc := range []struct {
		name, want string
		mutate     func(*gateOverride)
	}{
		{"no reason", "carries no reason", func(o *gateOverride) { o.Reason = "  " }},
		{"nobody ruled it", "names nobody", func(o *gateOverride) { o.RuledBy = "" }},
		{"no finding named", "names no finding digest", func(o *gateOverride) { o.Finding = "" }},
		{"describes another finding", "would clear one finding while describing another", func(o *gateOverride) { o.Rule = "some-other-rule" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := gateApplyOverrides(receipt, []gateOverride{gateOverrideFor(f, tc.mutate)})
			if err == nil {
				t.Fatalf("an override with %s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not say why:\n%v", err)
			}
		})
	}

	t.Run("ruled on twice", func(t *testing.T) {
		o := gateOverrideFor(f, nil)
		second := gateOverrideFor(f, func(x *gateOverride) { x.Reason = "a different reason entirely" })
		_, _, err := gateApplyOverrides(receipt, []gateOverride{o, second})
		if err == nil || !strings.Contains(err.Error(), "ruled on twice") {
			t.Fatalf("two rulings on one finding were accepted, so the record answers \"what was decided\" two ways: %v", err)
		}
	})
}

// TestGateOverridePromotesOnlyUpwardsAndOnlyWithEverythingElseInPlace is the
// promoting half of this record, held to the identical bar as the clearing half.
//
// THE ONE NEW REFUSAL is direction: the matrix is the floor a ruling can raise and
// never a ceiling it lowers. Everything else below is an EXISTING refusal asserted
// against a promoting entry, and those rows are not duplication — they are what
// holds the new branch inside the old checks. A promotion is written by the same
// person under the same time pressure as a clearance, and an entry that can raise a
// rank with nobody's name on it, with no reason, against a stale digest or for last
// release is the same unaccountable document with the sign flipped.
//
// The fixture is P2 here rather than the file's usual P1, because a promotion needs
// somewhere to raise TO — see gateOverrideFinding's comment for why every other
// fixture in this file is P1.
func TestGateOverridePromotesOnlyUpwardsAndOnlyWithEverythingElseInPlace(t *testing.T) {
	f := gateOverrideFinding("readme", "stale-command-count", "the table says nineteen and the binary has twenty")
	f.Consequence = gateConsequenceCosmetic
	f.Priority = gatePriorityP2
	receipt := gateOverrideReceipt(f)

	promote := func(mutate func(*gateOverride)) gateOverride {
		return gateOverrideFor(f, func(o *gateOverride) {
			o.PromoteTo = gatePriorityP1
			if mutate != nil {
				mutate(o)
			}
		})
	}

	// The positive control: the finding STAYS, at the band it was raised to, and
	// nothing is cleared.
	remaining, applied, err := gateApplyOverrides(receipt, []gateOverride{promote(nil)})
	if err != nil {
		t.Fatalf("an honest promotion was refused, so every refusal below would pass over a check that fires unconditionally: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("the promotion is not reported as applied (%d), so a receipt cleared by nothing and one raised by a named human read identically", len(applied))
	}
	if len(remaining) != 1 {
		t.Fatalf("a promotion removed the finding it promoted; promoting is the opposite of clearing, and a finding raised out of the record cannot block anything")
	}
	if remaining[0].Priority != gatePriorityP1 {
		t.Errorf("the promoted finding is judged at %q; the ruling raised it to %s, and a promotion the partition never sees is a line in a file", remaining[0].Priority, gatePriorityP1)
	}
	if receipt.Findings[0].Priority != gatePriorityP2 {
		t.Errorf("the promotion rewrote the receipt's own record of the finding to %q; what the matrix computed has to stay readable beside what the human decided", receipt.Findings[0].Priority)
	}

	for _, tc := range []struct {
		name, want string
		mutate     func(*gateOverride)
	}{
		{
			// THE ONE THAT IS NEW. Equal is not a promotion either: an entry that
			// restates the matrix's own answer changes nothing while reading, to
			// whoever finds it in the record, as a decision somebody made.
			name:   "promoting to the band the matrix already gave",
			want:   "the matrix is the floor a ruling can raise",
			mutate: func(o *gateOverride) { o.PromoteTo = gatePriorityP2 },
		},
		{
			name:   "promoting downwards",
			want:   "the matrix is the floor a ruling can raise",
			mutate: func(o *gateOverride) { o.PromoteTo = gatePriorityP3 },
		},
		{
			name:   "a band outside the four",
			want:   "which is not one of",
			mutate: func(o *gateOverride) { o.PromoteTo = "critical" },
		},
		{
			name:   "the old severity vocabulary",
			want:   "which is not one of",
			mutate: func(o *gateOverride) { o.PromoteTo = "blocking" },
		},
		{"a promotion nobody signed", "names nobody", func(o *gateOverride) { o.RuledBy = "" }},
		{"a promotion with no reason", "carries no reason", func(o *gateOverride) { o.Reason = "  " }},
		{"a promotion naming no finding", "names no finding digest", func(o *gateOverride) { o.Finding = "" }},
		{"a promotion describing another finding", "would clear one finding while describing another", func(o *gateOverride) { o.Rule = "some-other-rule" }},
		{"a promotion ruled for another release", "never inherited", func(o *gateOverride) { o.Version = "v0.5.3" }},
		{"a stale promotion", "matches no finding", func(o *gateOverride) { o.Finding = "sha256:" + strings.Repeat("0", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, applied, err := gateApplyOverrides(receipt, []gateOverride{promote(tc.mutate)})
			if err == nil {
				t.Fatalf("a promotion with %s was accepted; it raised a rank with %d entry applied", tc.name, len(applied))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not say %q:\n%v", tc.want, err)
			}
		})
	}

	// AN ENTRY IS ONE THING OR THE OTHER, and `promote_to` decides which. There is
	// no shape that raises a rank and clears the finding at the same time: the
	// finding is still there afterwards, and it is still judged.
	t.Run("promote_to present means promote, never clear", func(t *testing.T) {
		remaining, _, err := gateApplyOverrides(receipt, []gateOverride{promote(nil)})
		if err != nil {
			t.Fatalf("the promotion was refused: %v", err)
		}
		if len(remaining) != 1 {
			t.Fatalf("an entry carrying both a promotion and the shape of a clearance cleared the finding; a record that does both at once answers \"what was decided\" two ways")
		}
	})
}

func TestGateOverrideRecordIsRefusedRatherThanSkipped(t *testing.T) {
	root := t.TempDir()

	// Absent is the ordinary case.
	if got, err := gateLoadOverrides(root); err != nil || got != nil {
		t.Fatalf("an absent record was not read as no overrides: %v %v", got, err)
	}

	gateWrite(t, root, gateOverrideFile, "{ this is not json")
	if _, err := gateLoadOverrides(root); err == nil {
		t.Fatal("a malformed override record was read as an empty one. A record that cannot be parsed clears nothing AND says nothing, and the two have to be told apart by whoever is asking why their ruling did not apply")
	}
}

// TestGateOverrideReachesTheVerdictAndSaysSoOnTheReceipt is the end-to-end shape:
// a receipt carrying a finding and the ruling that clears it evaluates PASS,
// while the same receipt without the ruling does not — and the ruling is on the
// receipt where a reader and the run record both meet it.
func TestGateOverrideReachesTheVerdictAndSaysSoOnTheReceipt(t *testing.T) {
	f := gateOverrideFinding("contributing", "harness-did-not-withhold-tools", "the grant was not enforced for this run")
	receipt := gateOverrideReceipt(f)
	declared := []string{"contributing"}
	current := map[string]string{"contributing": "sha256:contributing"}

	if verdict, err := receipt.evaluate(declared, current); verdict != gateVerdictFailed {
		t.Fatalf("a receipt with an unruled finding evaluated %s: %v", verdict, err)
	}

	receipt.Overrides = []gateOverride{gateOverrideFor(f, nil)}
	verdict, err := receipt.evaluate(declared, current)
	if verdict != gateVerdictPass {
		t.Fatalf("a receipt whose only finding was ruled on evaluated %s: %v", verdict, err)
	}

	// On the FACE of it: the finding is still there, and so is the ruling.
	if len(receipt.Findings) != 1 {
		t.Error("the finding left the record when it was ruled on; deleting it is the thing this field exists to replace")
	}
	blob, marshalErr := json.Marshal(receipt)
	if marshalErr != nil {
		t.Fatalf("marshal the receipt: %v", marshalErr)
	}
	for _, want := range []string{`"overrides"`, "Nitin Khanna", "harness-did-not-withhold-tools"} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("the serialised receipt does not carry %q, so a reader cannot tell it was cleared by a ruling rather than by having nothing to clear:\n%s", want, blob)
		}
	}

	// A record that decides nothing fails the whole evaluation rather than being
	// skipped — the receipt is refused, not quietly treated as unruled.
	receipt.Overrides = []gateOverride{gateOverrideFor(f, func(o *gateOverride) { o.Reason = "" })}
	if verdict, err := receipt.evaluate(declared, current); verdict != gateVerdictFailed || !strings.Contains(err.Error(), "carries no reason") {
		t.Fatalf("an unreasoned override did not fail the evaluation with its own reason: %s %v", verdict, err)
	}
}

// TestGateOverrideRecordIsActuallyLoadedByTheRunThatMeasuresTheReceipt is the
// finding this file shipped without: the type, the refusals and every property
// test existed one commit before anything on the release path read the file, so
// gateLoadOverrides had exactly one caller — its own test — and a maintainer who
// wrote gate/overrides.json exactly as documented changed nothing at all.
//
// A mechanism nothing calls is the defect this whole gate is built to catch, and
// it reached the tree inside the mechanism built to record human judgement. This
// test is the wire, so it cannot come loose in silence.
func TestGateOverrideRecordIsActuallyLoadedByTheRunThatMeasuresTheReceipt(t *testing.T) {
	work := gateFixtureRepo(t)
	f := gateOverrideFinding("readme", "some-rule", "a sentence that is not true")
	surfaces := []gateSurfaceVerdict{{Surface: "readme", Verdict: gateVerdictFailed, Fingerprint: "sha256:readme"}}

	// No record: the receipt carries no rulings, which is the ordinary case.
	receipt, err := gateRecordReceipt(work, "v0.5.2", "release", surfaces, []gateFinding{f})
	if err != nil {
		t.Fatalf("record a receipt with no override record: %v", err)
	}
	if len(receipt.Overrides) != 0 {
		t.Fatalf("a run with no override record produced %d ruling(s)", len(receipt.Overrides))
	}

	// A record on disk reaches the receipt the driver measures.
	body, marshalErr := json.Marshal(struct {
		Overrides []gateOverride `json:"overrides"`
	}{[]gateOverride{gateOverrideFor(f, nil)}})
	if marshalErr != nil {
		t.Fatalf("marshal the record: %v", marshalErr)
	}
	gateWrite(t, work, gateOverrideFile, string(body))

	receipt, err = gateRecordReceipt(work, "v0.5.2", "release", surfaces, []gateFinding{f})
	if err != nil {
		t.Fatalf("record a receipt with an override record present: %v", err)
	}
	if len(receipt.Overrides) != 1 {
		t.Fatalf("the record on disk did not reach the receipt: %d ruling(s) carried", len(receipt.Overrides))
	}
	if verdict, evalErr := receipt.evaluate([]string{"readme"}, map[string]string{"readme": "sha256:readme"}); verdict != gateVerdictPass {
		t.Fatalf("a ruling on disk did not reach the verdict: %s %v", verdict, evalErr)
	}

	// And a record that cannot be parsed refuses the recording outright, rather
	// than producing a receipt that silently carries no rulings.
	gateWrite(t, work, gateOverrideFile, "{ not json")
	if _, err := gateRecordReceipt(work, "v0.5.2", "release", surfaces, []gateFinding{f}); err == nil {
		t.Fatal("a malformed override record produced a receipt anyway; the run would then report no rulings and the human would be told their file did nothing, with no reason given")
	}
}

// TestGateOverrideDigestIsCopyableFromTheReceipt closes the second half of the
// same defect: the record was documented as naming `"finding": "sha256:…"` while
// nothing in the tree emitted that value, so writing a ruling meant hand-computing
// a sha256 over a NUL-delimited string. A mechanism only a person who wrote it can
// operate is not an escape hatch.
func TestGateOverrideDigestIsCopyableFromTheReceipt(t *testing.T) {
	work := gateFixtureRepo(t)
	f := gateOverrideFinding("readme", "some-rule", "a sentence that is not true")

	receipt, err := gateRecordReceipt(work, "v0.5.2", "release",
		[]gateSurfaceVerdict{{Surface: "readme", Verdict: gateVerdictFailed, Fingerprint: "sha256:readme"}},
		[]gateFinding{f})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(receipt.Findings) != 1 || receipt.Findings[0].Digest == "" {
		t.Fatal("the receipt carries no digest for its finding, so an override cannot be written against it without recomputing one by hand")
	}

	// The value on the receipt is the value the matcher uses — copy it verbatim
	// into a ruling and the ruling applies.
	ruling := gateOverride{
		Version: "v0.5.2",
		Finding: receipt.Findings[0].Digest,
		Surface: f.Surface,
		Rule:    f.Rule,
		RuledBy: "a maintainer",
		Reason:  "copied the digest off the receipt, which is the whole point",
	}
	remaining, applied, err := gateApplyOverrides(receipt, []gateOverride{ruling})
	if err != nil || len(applied) != 1 || len(remaining) != 0 {
		t.Fatalf("a ruling carrying the receipt's own digest did not apply: applied=%d remaining=%d err=%v", len(applied), len(remaining), err)
	}

	// And the field is DERIVED, never trusted: a digest edited by hand on the
	// receipt does not change which finding a ruling matches.
	tampered := receipt
	tampered.Findings = append([]gateFinding(nil), receipt.Findings...)
	tampered.Findings[0].Digest = "sha256:" + strings.Repeat("0", 64)
	if _, applied, err := gateApplyOverrides(tampered, []gateOverride{ruling}); err != nil || len(applied) != 1 {
		t.Fatalf("editing the digest field on the receipt changed which finding the ruling matched; the value has to be recomputed from the finding's own text: applied=%d err=%v", len(applied), err)
	}
}
