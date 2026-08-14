// gate_override_test.go is THE OVERRIDE RECORD: the one way a finding the human
// has ruled non-blocking stops failing the gate without being deleted.
//
// WHY IT EXISTS, and why it was deliberately absent until now.
// gate_stage3_test.go has carried the argument since the join was written: the
// receipt has no field for a human waving a finding through, and evaluate()
// returns FAILED whenever any finding is present, so the only way past a finding
// somebody has judged non-blocking was to delete it from the record — which
// leaves an adjudicated finding and a finding nobody raised byte-identical to the
// next release. That was defensible while nothing needed it. The v0.5.2 gate's
// fifth round made it routine: 58 findings, two of which are the reading agents
// reporting that the harness could not withhold the tools the design grants them
// — a fact about the runtime that no edit to this tree can fix, and that
// therefore recurs on every round for ever.
//
// THE THREE PROPERTIES, which are the whole design. An override that lacks any
// one of them is worse than no override at all, because it makes a green receipt
// mean less while looking like it means the same.
//
//	HARDER TO WRITE THAN A FIX. An override names a DIGEST over the finding's own
//	    text. Change the finding — or fix the defect so the wording moves — and
//	    the override matches nothing and is REFUSED as stale. It cannot be written
//	    ahead of time, cannot be copied between findings, and cannot survive the
//	    thing it excuses being reworded.
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
// WHAT AN OVERRIDE IS NOT. It is not a severity threshold, and nothing here reads
// gateFinding.Severity. Severity is free text the reporting agent wrote about its
// own work; an override is a named person's decision about one specific finding,
// recorded in their words. The two must not be confused, which is why this file
// refuses an override that carries no reason: a rule that fires automatically is
// a threshold with extra steps.
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
}

// gateFindingDigest is the identity an override names.
//
// It covers the finding's SUBSTANCE — surface, rule and the detail text — and
// nothing else. Severity is excluded on purpose: it is the reporting agent's own
// word about its own work, and an override that survived a severity edit while
// dying on a wording edit would be keyed to the wrong half of the finding.
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

// gateApplyOverrides matches a receipt's findings against the human's rulings.
//
// It returns the findings that remain — the ones nobody ruled on, which still
// fail the gate — and the overrides that were actually applied. EVERY other
// outcome is an error, and that is the point: a stale override, an override for
// another release, one with no reason, one naming a finding that is not there,
// two rulings on the same finding. None of those is a no-op. Each one means the
// record and the run disagree about what was decided, and a gate that shrugged
// at that would be carrying a document nobody can trust into the one decision it
// exists to inform.
func gateApplyOverrides(receipt gateReceipt, overrides []gateOverride) (remaining []gateFinding, applied []gateOverride, err error) {
	byDigest := make(map[string]gateFinding, len(receipt.Findings))
	for _, f := range receipt.Findings {
		byDigest[gateFindingDigest(f)] = f
	}

	var problems []string
	seen := map[string]bool{}
	cleared := map[string]bool{}

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
		cleared[o.Finding] = true
		applied = append(applied, o)
	}

	for _, f := range receipt.Findings {
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
// human's rulings are applied: a surface whose every finding was ruled on reads
// PASS, and one holding a single unruled finding does not.
//
// It builds a new slice rather than editing the receipt's, because the receipt
// records what the AGENTS reported and that must stay readable next to what the
// human decided. Two different questions, two different answers, both kept.
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
	ruled := gateFinding{Surface: "contributing", Rule: "harness-did-not-withhold-tools", Severity: "blocking", Detail: "the grant was not enforced"}
	unruled := gateFinding{Surface: "readme", Rule: "one-envelope-claim", Severity: "minor", Detail: "serve is not one envelope"}
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
		t.Error("a surface holding an unruled finding was cleared; an override clears one finding, never a surface")
	}
	for _, v := range receipt.Surfaces {
		if v.Verdict != gateVerdictFailed {
			t.Errorf("the receipt's own record of surface %q was rewritten to %q; what the agents reported has to stay readable beside what the human decided", v.Surface, v.Verdict)
		}
	}
}

func TestGateOverrideIsHarderToWriteThanAFix(t *testing.T) {
	f := gateFinding{Surface: "site", Rule: "text-parity-overclaim", Severity: "minor", Detail: "byte-for-byte identical to v0.2.0's output"}
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
	f := gateFinding{Surface: "release-procedure", Rule: "tool-grant-was-not-enforced", Severity: "high", Detail: "the harness held file and shell tools"}
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
	f := gateFinding{Surface: "readme", Rule: "stage-order", Severity: "moderate", Detail: "the gate is not the last step"}
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
	f := gateFinding{Surface: "contributing", Rule: "harness-did-not-withhold-tools", Severity: "blocking", Detail: "the grant was not enforced for this run"}
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
