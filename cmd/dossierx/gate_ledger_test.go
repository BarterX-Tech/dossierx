// gate_ledger_test.go is the cost ledger: what a gate run may record about what
// it spent, what a cost MODEL may say about what a run should cost, and the
// validator that refuses a recording which cannot be true.
//
// READ THIS FIRST — THE SCOPE, AND WHAT IS DELIBERATELY NOT BUILT.
//
// Decision C25 rescoped this lane to "a ledger that records predicted-versus-
// actual per surface per run, with the unit stored beside every number, and a k
// a human edits", and DEFERRED automatic calibration until five real runs exist
// to calibrate from. C22 says of this lane, in as many words, "W11 builds
// against this rather than against the impossible version". So there is no
// five-run window here, no median, no per-class partition of a capped run's
// vote, and no asymmetric floor. A previous draft built the calibrator and was
// blocked by both review seats; the operation is still unspecifiable, because
// the plan's capped-run floor is denominated in a per-class allowance C22
// abolished.
//
// What calibration was protecting is built, restated in quantities a C22 entry
// actually records. Seven things:
//
//  1. NO NUMBER IS BARE, AND NEITHER ITS DENOMINATION NOR ITS BASIS IS AN
//     OPINION. Every token quantity carries a unit, and every MEASURED one also
//     carries the name of the counter it was read from. Every prediction is a
//     SIZE times a k, and the size carries its unit (bytes) and the name of the
//     DOCUMENT BASIS that decided which bytes the surface's agent is held to
//     read. Phase 0 measured the harness's per-subagent figure at 1,417,000
//     against 489,218 output tokens — 2.9x — so a number whose denomination is
//     a matter of memory is not evidence.
//
//  2. WHAT THE NAMES MEAN LIVES IN CODE. gateLedgerCounters says what each
//     counter measures (unit AND scope) and where that fact came from;
//     gateLedgerBases says which bytes each basis resolves to and from what.
//     The model file references both BY NAME ONLY. A unit typed in a
//     hand-edited file and checked against a correspondence typed in the same
//     hand-edited file is a circle, and one YAML line inside it reproduces the
//     2.9x error in full. Changing what a name means is a code diff that
//     arrives with its reason.
//
//  3. THE ENFORCED UNIT IS THE ONLY UNIT. C18 pins the enforceable figure as
//     output tokens, because that is what budget.spent() returns and therefore
//     the only thing a script can stop a run on. k's numerator must be that
//     unit and its denominator must be bytes, or the model funds nothing and
//     says so. This is where the live contradiction between the plan's k — a
//     ratio whose numerator explicitly includes the document and shared context
//     the agent READS — and C18's counter stops being invisible: state the
//     plan's denomination in the model and it is refused by name.
//
//  4. CAPPEDNESS IS COMPUTED, NEVER DECLARED. A run records its ceiling and its
//     spend; capped is spend >= ceiling, and a declared outcome that contradicts
//     the entry's own arithmetic is refused.
//
//  5. COMPLETION IS NOT A FIELD ANYBODY FILLS IN. There is no receipt path
//     anywhere in this tree — gate/ is the ephemeral working-artifact namespace
//     and does not exist on disk — so an entry that referenced a receipt would
//     reference nothing, and a check over it would either skip (leaving a forged
//     completion bit green) or refuse every entry forever. So each recorded run
//     CARRIES its own gateReceipt and the tree it covered, and which surfaces
//     finished is derived from it through gateIndexVerdicts. This is the tree's
//     own rule applied one level out: provenance "travels WITH the verdict,
//     never beside it" (gate_fingerprint_test.go:180-182).
//
//     A surface that holds a verdict may record an ACTUAL. A surface that holds
//     none may record only a FLOOR — a bound on demand, by construction smaller
//     than what its work needed. Which of the two is legal is decided by the
//     receipt, not by whoever wrote the entry, and no consumer can read a number
//     out of a run without also being told which of the two it is
//     (gateLedgerObservations).
//
//  6. THE PARTITION AND THE BASIS ASSIGNMENT ARE EACH TOTAL OR EXPLICITLY
//     UNMADE. Every declared surface has a class and a document basis, or is
//     recorded as undecided for whichever is missing. There is no third state
//     and no default: quoting an undecided surface is a refusal that names the
//     surface and says which decision is missing. The two are enforced together
//     deliberately — assigning binary-and-viewer's class while its basis stays
//     defaulted is what arms the wrong-bytes failure, because until it has a
//     class the unmade-class refusal was the only thing shielding it.
//
//  7. EVERY MOVE OF k LEAVES A RECORD, IS BOUNDED, AND CARRIES ITS REASON. Under
//     C25 the thing that moves k is a human edit. The value in force, the value
//     it replaced and why it moved are in the same diff; travel is bounded
//     asymmetrically (up to +25%, down to -10%) because under-funding fails a
//     gate while over-funding wastes headroom nobody spends; and a move past the
//     bound is legal only with a recorded reason, which is refused when the move
//     did not need one. The bounds bind whatever moved the number, INCLUDING a
//     human: the parked attempt left them unenforced on the reasoning that they
//     belonged to the deferred calibrator, and the result was a ledger that
//     accepted k going from 3.5 to 9.9 in one entry with no rule broken.
//
//     EVERY means every, in both files, and each half of that is a rule the
//     obvious implementation gets wrong:
//
//     The model records a CHAIN of moves and not the latest one. A field holding
//     one edit can record the first move and no other, because the second move's
//     `previous` is the first move's value; anchoring every edit to the seed
//     refuses the truthful record of it, and the only way left to write the
//     second move down is to overwrite the first. Invariant 6 would then be
//     honoured exactly once per class, and the diff that is supposed to be the
//     record of a move would be the diff deleting the record of the one before.
//
//     And EVERY ENTRY of the ledger is held to that chain, not the newest one. A
//     seed of 1.2 with no edit at all, and three runs recording 1.2, 1.32, 1.2:
//     every step is inside the travel bounds, the newest value equals the seed,
//     each entry's arithmetic is consistent at the k it names — and the middle
//     run stands quoted at 1.32, a number the model has never held. A five-run
//     corpus walks out and back the same way, so four of five entries can carry a
//     k up to 25% from anything a human chose, and those five entries are exactly
//     what C25 defers calibration until.
//
// WHAT THIS FILE CANNOT CHECK, said plainly rather than left to be discovered:
//
//   - PROVENANCE. There is no gate runtime in this tree, so every figure in a
//     ledger is transcribed. Nothing here can confirm that a number is the
//     number the harness measured. What is checked is everything that would make
//     a transcription error self-contradictory: a counter's scope must match the
//     field it is filed under, the surfaces of a run are nested inside the run
//     whose ceiling they were measured against, the per-surface figures cannot
//     exceed the run's own spend, and the surfaces the numbers describe must
//     agree with the verdict record the entry carries. Embedding that record
//     raises the cost of a forgery from editing one boolean to fabricating a
//     coherent receipt naming a real tree. It does not eliminate it, and the
//     remaining defence is a human reading the diff.
//
//   - THAT THE CEILING IS ENFORCED AT ALL. This file governs how a run that ran
//     out is RECORDED. That a run reaching its ceiling is in fact stopped and
//     reported FAILED is C5, it is orchestration-time behaviour, and it appears
//     in no lane's write set.
//
//   - WHETHER A RECORDED TREE STILL RESOLVES. An entry must NAME its tree in the
//     form the receipt handshake compares. It is not required to resolve in the
//     repository, because a tree from a rebased-away branch can be garbage
//     collected and a gate that goes red on repository housekeeping teaches
//     people to override it.
//
// Same shape as gate_receipt_test.go and gate_fingerprint_test.go: test code,
// not a cobra command, not compiled into the shipped binary, and outside
// surface.json's behaviour_fingerprint.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
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

// ---------------------------------------------------------------------
// where the two files live
// ---------------------------------------------------------------------

// The canonical paths. There is exactly one of each, pinned here rather than
// passed in, so that "the cost model" and "the cost ledger" are paths and not
// habits. A validator that only ever opens testdata/ leaves a wrong file at the
// real path fully green — gate_receipt_test.go already recorded that failure
// shape once, where an unwired check "was not a description of the precondition
// but the only performance of it".
const (
	gateLedgerModelPath  = "gate-cost-model.yaml"
	gateLedgerPath       = "gate-cost-ledger.json"
	gateLedgerFixtureDir = "testdata/gate-ledger"
)

// The schema tags. A file that does not declare its schema is refused rather
// than read with defaults filled in: a file read under the wrong schema is a
// file whose silences are being invented.
const (
	gateLedgerModelSchema  = "dossierx.gate-cost-model/1"
	gateLedgerLedgerSchema = "dossierx.gate-cost-ledger/1"
)

// ---------------------------------------------------------------------
// units, counters and bases — the registry of what names mean
// ---------------------------------------------------------------------

// The unit vocabulary. Two of these exist so that the 2.9x confusion can be
// WRITTEN DOWN and refused by name; a vocabulary containing only the right
// answer cannot refuse the wrong one.
const (
	gateLedgerUnitOutput   = "output_tokens"
	gateLedgerUnitInput    = "input_context_tokens"
	gateLedgerUnitDocument = "document_tokens"
	gateLedgerUnitTotal    = "total_tokens"
	gateLedgerUnitBytes    = "bytes"
)

// gateLedgerEnforcedUnit is the unit the run-level ceiling is enforced in, and
// therefore the unit every token quantity in both files must be in.
//
// C18: "Every quote and every ceiling is in output tokens, because that is what
// budget.spent() returns and therefore the only thing a script can enforce."
// Under C22 the run-level ceiling is the ONLY figure enforced mid-run, so this
// is the whole of the enforcement surface.
const gateLedgerEnforcedUnit = gateLedgerUnitOutput

// gateLedgerEnforcedCounter is the counter that ceiling is enforced against. A
// run's spend must be read from exactly this one: a spend read from any other
// counter is a spend the ceiling was never compared against, whatever it says
// its unit is.
const gateLedgerEnforcedCounter = "budget.spent"

// A prediction is a size times a k. These two constants are what that sentence
// means dimensionally, and they are in code because they are the pair the plan
// and C18 disagree about.
//
// The plan's constants table defines k as "total tokens an agent spends per
// token of document it reads — its own document, plus the shared surface.json
// and delta, plus reasoning and written findings", and its anatomy figure builds
// k~3 from three roughly equal parts, two of which are INPUT the agent reads.
// C18's enforceable counter excludes both. So the plan's seeds of 3.5 / 2.5 /
// 2.0 are total_tokens per document_token, and a ceiling built from them is
// roughly 3x the real output demand and cannot bind. Stating that denomination
// in the model file is legal to write and is refused here by name — which is the
// entire point, because today the two statements sit in two untracked artifacts
// and nothing can notice they disagree.
const (
	gateLedgerRatioNumerator   = gateLedgerEnforcedUnit
	gateLedgerRatioDenominator = gateLedgerUnitBytes
)

// A counter's scope: whether it reports on the whole run or on one agent.
const (
	gateLedgerScopeRun     = "run"
	gateLedgerScopeSurface = "surface"
)

// gateLedgerCounterSpec is what a named counter actually measures, pinned beside
// the measurement that is its provenance.
//
// Both Unit and Scope are load-bearing. Unit catches the input-context-for-
// output-tokens confusion; Scope catches a run total filed as one surface's
// spend, which is the transcription slip a shared counter makes easy.
type gateLedgerCounterSpec struct {
	Unit  string
	Scope string
	// Provenance is where the fact came from. It is a field rather than a
	// comment so that a counter added with no evidence behind it is a red test
	// rather than a line somebody skims.
	Provenance string
}

// gateLedgerCounters is the registry. A counter not in it cannot be recorded
// from — not because the name is forbidden, but because nothing here knows what
// it measures, and a number whose denomination is unknown is not evidence.
//
// This map is the second of two locks, and it is the one that makes the first
// mean anything: the unit rule is a discipline, and this is a fact. Somebody who
// edits a model file to declare that the harness's per-subagent counter measures
// output tokens is editing a hand-maintained file; somebody who edits this map
// is editing code, in a diff that carries its reason.
var gateLedgerCounters = map[string]gateLedgerCounterSpec{
	gateLedgerEnforcedCounter: {
		Unit:  gateLedgerUnitOutput,
		Scope: gateLedgerScopeRun,
		Provenance: "C18: what budget.spent() returns, and therefore the only figure a script can stop a run on. " +
			"Phase 0 measured 489,218 output tokens for the run whose peak context was 1,416,932.",
	},
	"harness.run_total_tokens": {
		Unit:  gateLedgerUnitInput,
		Scope: gateLedgerScopeRun,
		Provenance: "Phase 0 reported 1,417,000 against a measured peak context of 1,416,932: this counter reports INPUT CONTEXT, " +
			"2.9x the output figure the ceiling is enforced in. Registered so that citing it is a named refusal rather than a plausible number.",
	},
	"harness.subagent_tokens": {
		Unit:  gateLedgerUnitInput,
		Scope: gateLedgerScopeSurface,
		Provenance: "The per-agent figure the harness reports today. It measures input context, like its run-level sibling, " +
			"and C22 further established that under concurrency a per-agent snapshot counts everything spent since that agent started.",
	},
	"harness.subagent_output_tokens": {
		Unit:  gateLedgerUnitOutput,
		Scope: gateLedgerScopeSurface,
		Provenance: "NOT MEASURED BY PHASE 0 AND NOT PRODUCED BY ANYTHING IN THIS TREE. Registered so that a per-surface figure has a legal " +
			"name the moment a driver can emit one; a ledger citing it today is asserting a measurement no counter in this project has yet made. " +
			"This is open question 8 recorded rather than answered: per-surface actuals are optional for exactly this reason.",
	},
}

// errGateLedgerBasisUnavailable is a basis that is registered — its meaning is
// known — but whose bytes cannot be resolved from the committed tree.
//
// It is a distinct answer from "no such basis". An unavailable basis is a
// decision the human has ratified and the tree cannot yet honour; an unknown one
// is a name nobody defined. Collapsing them would let a typo read as a deferral.
var errGateLedgerBasisUnavailable = errors.New("the named document basis cannot be resolved from the committed tree")

// gateLedgerBasisSpec is which bytes a named document basis resolves to, and
// from what.
//
// The whole reason this registry exists: the only surface-to-files function in
// the tree, gateSurfaceDocuments, is written for the FINGERPRINT'S input set,
// where covering every file is the point. Reaching for it to compute a cost is
// a one-line mistake with no local symptom — and for two of the thirteen
// surfaces it is provably the wrong basis. surfaces.yaml's own binary-and-viewer
// entry describes the surface as "cobra Short and Long text and flag usage ...
// the error messages and hints, and the viewer templates", while its paths claim
// cmd/dossierx/ and internal/ entire: 1,949,261 bytes against a plan whose
// stated budget basis is roughly 560 KB for all thirteen surfaces together.
type gateLedgerBasisSpec struct {
	// Resolves says which bytes, in words a human can check against
	// surfaces.yaml.
	Resolves string
	// Provenance says from what, and — for the two that cannot be resolved —
	// exactly why not.
	Provenance string
	// size returns the basis's byte count for one surface, or
	// errGateLedgerBasisUnavailable.
	size func(root, surface string, tracked []string) (int64, error)
}

// gateLedgerBases is the registry of document bases. The model names one per
// surface; nothing may reach a basis without naming it.
var gateLedgerBases = map[string]gateLedgerBasisSpec{
	"manifest.tracked_files": {
		Resolves:   "every tracked file surfaces.yaml claims for the surface, summed in bytes",
		Provenance: "gateSurfaceDocuments(root, git ls-files), the manifest resolved against the tree",
		size:       gateLedgerSizeFromTrackedFiles,
	},
	"binary.embedded_prose": {
		Resolves: "the prose compiled INTO the binary — cobra Short and Long text, flag usage, error messages and hints, " +
			"and the viewer templates rendered into every client's index.html",
		Provenance: "surfaces.yaml:444-451, which is surfaces.yaml's own description of what the binary-and-viewer surface IS. " +
			"NOT RESOLVABLE TODAY: surface.json's commands array carries only {path, short, flags} — no long text, no flag usage, " +
			"no error-message strings, no templates — so this document cannot be reconstructed from the committed inventory.",
		size: gateLedgerSizeUnavailable,
	},
	// "site.rendered_dom" WAS THE THIRD ENTRY AND IS RETIRED. It resolved to
	// "the rendered DOM text of a real build, plus its head metadata" and always
	// refused, because that text was gate/site-text.json — an ephemeral gate
	// artifact that did not exist on disk. The site is two static HTML pages
	// with no build now, so the surface IS its tracked files and it is funded
	// from manifest.tracked_files like everything else. The basis is deleted
	// rather than left refusing: an unreachable key nothing names is how a
	// retired concept comes back, and gate-cost-model.yaml would fail to load
	// against a name this registry does not carry.
}

// gateLedgerSizeFromTrackedFiles sums the bytes of the tracked files the
// manifest claims for a surface.
//
// It goes through gateSurfaceDocuments rather than re-deriving the claim, so
// that a file joining a surface moves its size with no second edit — and so that
// this basis means exactly what the fingerprint's input set means, which is the
// only honest thing it can mean.
func gateLedgerSizeFromTrackedFiles(root, surface string, tracked []string) (int64, error) {
	documents, err := gateSurfaceDocuments(root, tracked)
	if err != nil {
		return 0, err
	}
	owned, ok := documents[surface]
	if !ok {
		return 0, fmt.Errorf("%w: surface %q", errGateLedgerBasisOwnsNothing, surface)
	}
	var total int64
	for _, rel := range owned {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return 0, fmt.Errorf("size %s: %w", rel, err)
		}
		total += info.Size()
	}
	return total, nil
}

// gateLedgerSizeUnavailable is the resolver for a basis whose bytes this tree
// cannot produce. It refuses rather than approximating: an approximation nobody
// ratified is the wrong-bytes failure with a comment on it.
func gateLedgerSizeUnavailable(_, surface string, _ []string) (int64, error) {
	return 0, fmt.Errorf("%w: surface %q", errGateLedgerBasisUnavailable, surface)
}

// gateLedgerSeedRatification is a k a human has chosen for a class: the value
// itself, and the evidence it was chosen on.
type gateLedgerSeedRatification struct {
	Value float64
	// Provenance is the measurement or decision behind the number. It is a field
	// rather than a comment for the same reason the counters' is: a seed added
	// with nothing behind it has to be a red test rather than a line somebody
	// skims.
	Provenance string
}

// gateLedgerRatifiedSeeds is the third code-level registry, and it closes the
// one place where a number every ceiling is derived from could still move with
// no record of the move.
//
// THE CHAIN IS A HISTORY WITH NO ANCHOR AT ITS HEAD. Every `k:` move in the
// model states what it replaced and is checked against the move before it, back
// to the seed — so the seed is what the whole chain hangs from, and it is the
// only value in either file that nothing states the provenance of in a way
// anything can check. A class that has recorded no move at all (which is the
// state ALL THREE classes are in today) therefore has a k that can be rewritten
// from one number to another by editing one line of gate-cost-model.yaml, with
// every rule in this file still satisfied: the edits check returns early on an
// empty chain, and the ledger's chain check has no entry for the class to
// disagree with. That is failure 7 — "k is changed without a record" — reached
// through the seed rather than through a move.
//
// So the seed is held to a fact stated in CODE, exactly as a counter's unit and
// a basis's bytes are, and for the reason invariant 1c gives: a number typed in
// a hand-edited file and checked against a correspondence typed in the same
// hand-edited file is a circle. Supplying or changing a seed is then a code diff
// that arrives with its reason beside it, reviewed as code — never a line of
// configuration. Moving k AFTERWARDS is still the human edit C25 asks for: it
// goes in the class's `k:` list, where it is bounded, carries its rationale, and
// leaves the seed where it stands.
//
// IT IS EMPTY, AND THE EMPTINESS IS THE COMMITTED MODEL'S STATE RESTATED. All
// three classes record their seed as undecided, because the plan's 3.5 / 2.5 /
// 2.0 are total tokens per document token and C18 requires output tokens per
// byte, and no output-denominated seed has been supplied. An entry added here
// without the same number appearing in gate-cost-model.yaml is refused, and so
// is the reverse, so the two cannot drift apart in either direction.
var gateLedgerRatifiedSeeds = map[string]gateLedgerSeedRatification{}

// gateLedgerUndecided is the reserved name for a decision nobody has made. It is
// not a class and not a basis; it is the explicit third thing that keeps the
// partition total without inventing an assignment.
const gateLedgerUndecided = "undecided"

// gateLedgerTrackedFiles is `git ls-files` for a root, as repo-relative slash
// paths. It is the non-test twin of surfaceTrackedFiles, because a basis
// resolver cannot take a *testing.T.
//
// A git failure is an error, never an empty set: an empty file list would make
// every surface resolve to zero bytes and every quote come out free.
//
// THE TWO REFUSALS ARE DIFFERENT ANSWERS AND CARRY DIFFERENT SENTINELS. A git
// that could not run at all is the question failing to be asked —
// errGateUncheckable, which CLAUDE.md makes a FAILED gate. A git that ran and
// reported nothing is errGateLedgerNoTrackedFiles, a statement about the
// repository. Collapsing them hides the first behind the second: a git failing
// for any reason leaves the file list empty, so the emptiness refusal fires and
// the failure is never named — and a git that failed while still printing paths
// would then be trusted, which is the case the emptiness refusal cannot see.
func gateLedgerTrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: git ls-files in %s: %w", errGateUncheckable, root, err)
	}
	var files []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			files = append(files, filepath.ToSlash(name))
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: git ls-files reported nothing in %s; every size below would resolve to zero", errGateLedgerNoTrackedFiles, root)
	}
	sort.Strings(files)
	return files, nil
}

// ---------------------------------------------------------------------
// the cost model — the hand-authored half
// ---------------------------------------------------------------------

// gateLedgerModel is gate-cost-model.yaml: what a run SHOULD cost, and the
// decisions behind that.
//
// It is hand-authored configuration in the shape of surfaces.yaml, not a
// generated artifact in the shape of surface.json. It references counters and
// bases BY NAME ONLY; what those names mean is in the registries above.
type gateLedgerModel struct {
	Schema string `yaml:"schema"`
	// Reserve is headroom on top of the summed predictions, and BackstopFactor
	// is the multiple of the ceiling at which the harness's own backstop sits.
	Reserve        gateLedgerModelQuantity      `yaml:"reserve"`
	BackstopFactor gateLedgerModelFactor        `yaml:"backstop_factor"`
	Classes        []gateLedgerModelClass       `yaml:"classes"`
	Surfaces       []gateLedgerModelSurfacePlan `yaml:"surfaces"`
}

// gateLedgerModelQuantity is a token quantity in the model: decided, with a
// value and a unit, or explicitly undecided with a reason. There is no third
// state and no zero that means "unset".
type gateLedgerModelQuantity struct {
	Undecided bool   `yaml:"undecided,omitempty"`
	Reason    string `yaml:"reason,omitempty"`
	Value     int64  `yaml:"value,omitempty"`
	Unit      string `yaml:"unit,omitempty"`
}

// gateLedgerModelFactor is a dimensionless multiplier, decided or explicitly
// undecided. Same shape as the quantity above, minus the unit — a ratio of two
// token counts has none.
type gateLedgerModelFactor struct {
	Undecided bool    `yaml:"undecided,omitempty"`
	Reason    string  `yaml:"reason,omitempty"`
	Value     float64 `yaml:"value,omitempty"`
}

// gateLedgerModelClass is one cost class: its seed, and every human edit that
// has moved k off it since.
type gateLedgerModelClass struct {
	Name string `yaml:"name"`
	// Seed is where k started, with its denomination and the measurement it was
	// derived from. It is the anchor a first edit is measured against, so that
	// a k that moved has something to have moved FROM even before any run has
	// been recorded.
	Seed gateLedgerModelSeed `yaml:"seed"`
	// K is every move of k off the seed, oldest first. Empty means nobody has
	// moved it, which is a different statement from "somebody moved it back".
	// The last entry is the value in force.
	//
	// IT IS A SEQUENCE BECAUSE K MOVES MORE THAN ONCE, and a field holding one
	// edit can record the first move and no other. The second move's `previous`
	// is the first move's value, so a schema with room for one edit refuses the
	// truthful record of the second: after a legal 1.2 -> 1.5, an edit saying it
	// replaced 1.5 with 1.8 is checked against the seed and rejected as an
	// invented predecessor. That is not a missing feature, it is a forced lie —
	// the only way to make a second move validate is to overwrite the first, and
	// then the diff that is supposed to BE the record of the move is the diff
	// that deletes the record of the one before it. Invariant 6 says every move
	// of k leaves a record showing the value in force, the value it replaced and
	// why it moved; a single-edit field can honour that exactly once.
	//
	// The chain is also what a run's recorded k is checked against: these values,
	// in this order, are every number k has ever been in force at.
	K []gateLedgerModelEdit `yaml:"k,omitempty"`
}

// gateLedgerModelSeed is k's starting value: a ratio, so it carries BOTH units.
type gateLedgerModelSeed struct {
	Undecided       bool    `yaml:"undecided,omitempty"`
	Reason          string  `yaml:"reason,omitempty"`
	Value           float64 `yaml:"value,omitempty"`
	NumeratorUnit   string  `yaml:"numerator_unit,omitempty"`
	DenominatorUnit string  `yaml:"denominator_unit,omitempty"`
	DerivedFrom     string  `yaml:"derived_from,omitempty"`
}

// gateLedgerModelEdit is a human's move of k, and everything that makes it a
// reviewable diff rather than a number that changed.
type gateLedgerModelEdit struct {
	Value           float64 `yaml:"value"`
	NumeratorUnit   string  `yaml:"numerator_unit"`
	DenominatorUnit string  `yaml:"denominator_unit"`
	// Previous is the value this edit replaced. It is checked against the move
	// before it in the chain — the seed for the first edit — so an edit cannot
	// invent its own history.
	Previous float64 `yaml:"previous"`
	// Rationale is required of a move and refused of a non-move, so it cannot
	// become the sentence every entry carries and nobody reads.
	Rationale string `yaml:"rationale,omitempty"`
	// BoundOverride is the reason this edit moves k further in one run than the
	// travel bounds allow. Refused when the move is inside them, for the same
	// reason.
	BoundOverride string `yaml:"bound_override,omitempty"`
}

// gateLedgerModelSurfacePlan is one declared surface's two decisions. Either may
// be the reserved name "undecided"; neither may be absent, and there is no
// default for either.
type gateLedgerModelSurfacePlan struct {
	Name          string `yaml:"name"`
	Class         string `yaml:"class"`
	DocumentBasis string `yaml:"document_basis"`
}

// The travel bounds. k may rise by at most 25% between two runs and fall by at
// most 10%.
//
// The two numbers are deliberately unequal. Under-funding FAILS a gate;
// over-funding wastes headroom nobody spends. So k may chase a rise quickly and
// must climb down slowly, and a single surprising run cannot halve it.
//
// They bind whatever moved the number, including a human, because they are a
// property of how far k may travel between two runs and not of what computed the
// new value.
const (
	gateLedgerMaxRaise = 0.25
	gateLedgerMaxLower = 0.10
)

// gateLedgerEpsilon absorbs binary floating point so a move landing EXACTLY on a
// bound is inside it. 3.5 * 0.9 is not 3.15 in float64, and a bound that rejects
// the value it was defined by is a bound nobody can hit on purpose.
const gateLedgerEpsilon = 1e-9

// ---------------------------------------------------------------------
// the ledger — the appended half
// ---------------------------------------------------------------------

// gateLedgerQuantity is any number of tokens in the ledger. There is no bare
// number anywhere in this schema; that is the whole point of the type.
type gateLedgerQuantity struct {
	Value int64  `json:"value"`
	Unit  string `json:"unit"`
	// Counter names where the figure was read from, and is required exactly of
	// figures that were MEASURED. A prediction carries none: a prediction read
	// off a counter is not a prediction, it is the actual copied into the wrong
	// column.
	Counter string `json:"counter,omitempty"`
}

// gateLedgerSize is a document's size: bytes, and the named basis that decided
// WHICH bytes. A size with no basis is not a legal recording and no basis is the
// default.
type gateLedgerSize struct {
	Value int64  `json:"value"`
	Unit  string `json:"unit"`
	Basis string `json:"basis"`
}

// gateLedgerRatioRecord is the k that was in force for one class during one run.
type gateLedgerRatioRecord struct {
	Class           string  `json:"class"`
	Value           float64 `json:"value"`
	NumeratorUnit   string  `json:"numerator_unit"`
	DenominatorUnit string  `json:"denominator_unit"`
	// BoundOverride is the reason k moved further since the previous recorded
	// run than the bounds allow. Same required-and-refused symmetry as the
	// model's.
	BoundOverride string `json:"bound_override,omitempty"`
}

// gateLedgerSurfaceRecord is one surface's cost inside one run.
//
// THERE IS NO COMPLETION FIELD AND NO PER-SURFACE CEILING FIELD, and neither may
// be added. Completion is derived from the run's own embedded verdict record;
// a per-surface ceiling is a promise C22 abolished and C22's predecessor proved
// unenforceable under concurrency. Decoding refuses unknown fields, so neither
// can be written into the file and silently ignored by the reader.
//
// Actual and Floor are the same number wearing two different meanings, and they
// are separate fields on purpose. A surface that never finished contributes a
// figure that is by construction less than what its work needed; filed as an
// actual it reads as "this surface came in cheap", which is the exact inversion
// of the truth — it came in cheap BECAUSE it was starved. Which field is legal
// is decided by the receipt, not by whoever wrote the entry.
type gateLedgerSurfaceRecord struct {
	Surface   string              `json:"surface"`
	Class     string              `json:"class"`
	Size      gateLedgerSize      `json:"size"`
	Predicted gateLedgerQuantity  `json:"predicted"`
	Actual    *gateLedgerQuantity `json:"actual,omitempty"`
	Floor     *gateLedgerQuantity `json:"floor,omitempty"`
}

// The two outcomes. There is no third, and neither means "we did not check".
const (
	gateLedgerOutcomeCompleted = "completed"
	gateLedgerOutcomeCapped    = "capped"
)

// gateLedgerRun is one gate run.
//
// Its surfaces are NESTED inside it rather than carrying a run id of their own,
// so a figure cannot be attributed to the wrong run's ceiling by a transcription
// slip: there is one ceiling and one spend per run and the surfaces are
// underneath them.
type gateLedgerRun struct {
	RunID string `json:"run_id"`
	// MethodFingerprint is gateMethod.version — the digest over the prompt
	// sources' bytes, the model id and the sorted tool list. C25 defers
	// calibration until five real runs exist, and those five runs are this
	// ledger's entries: an entry with no stamp cannot later be told apart from
	// one recorded under a rewritten prompt or a different model.
	MethodFingerprint string                    `json:"method_fingerprint"`
	Ceiling           gateLedgerQuantity        `json:"ceiling"`
	Reserve           gateLedgerQuantity        `json:"reserve"`
	Spend             gateLedgerQuantity        `json:"spend"`
	Outcome           string                    `json:"outcome"`
	K                 []gateLedgerRatioRecord   `json:"k"`
	Surfaces          []gateLedgerSurfaceRecord `json:"surfaces"`
	// Receipt is the run's OWN verdict record and the tree it covered, carried
	// inside the entry rather than referenced. See point 5 of the header: there
	// is no receipt path anywhere in this tree, so a reference would point at
	// nothing and a check over it would either skip or refuse forever.
	Receipt *gateReceipt `json:"receipt"`
}

// gateLedgerFile is gate-cost-ledger.json.
type gateLedgerFile struct {
	Schema string          `json:"schema"`
	Runs   []gateLedgerRun `json:"runs"`
}

// ---------------------------------------------------------------------
// what neither file may say
// ---------------------------------------------------------------------

var (
	errGateLedgerUnknownSchema = errors.New("the file does not declare the schema this validator reads")

	errGateLedgerUnitMissing            = errors.New("a quantity carries no unit")
	errGateLedgerUnitWrong              = errors.New("a quantity is not in the unit the ceiling is enforced in")
	errGateLedgerUnknownCounter         = errors.New("a quantity names a counter nothing knows the denomination of")
	errGateLedgerUnitContradictsCounter = errors.New("a quantity's declared unit is not what its counter measures")
	errGateLedgerCounterScope           = errors.New("a quantity was read from a counter of the wrong scope")
	errGateLedgerNotTheEnforcedCounter  = errors.New("a run's spend was read from a counter the ceiling is not enforced against")
	errGateLedgerUnmeasured             = errors.New("a measured quantity names no counter")
	errGateLedgerPredictionMeasured     = errors.New("a predicted quantity was read from a counter")
	errGateLedgerNonPositive            = errors.New("a quantity is not a positive number")

	errGateLedgerUnknownBasis     = errors.New("a size names a document basis the code registry does not know")
	errGateLedgerUnratifiedBasis  = errors.New("a size was measured over a document basis nobody ratified for that surface")
	errGateLedgerSizeUnitWrong    = errors.New("a size is not denominated in bytes")
	errGateLedgerPredictionArith  = errors.New("a prediction is not its size multiplied by the k the run recorded")
	errGateLedgerCeilingArith     = errors.New("a run's ceiling is not its predictions plus its reserve")
	errGateLedgerActualsExceedRun = errors.New("a run's per-surface figures sum to more than the whole run spent")

	errGateLedgerRunUnidentified = errors.New("a run carries no id")
	errGateLedgerRunTwice        = errors.New("two runs share one id")
	errGateLedgerUnstamped       = errors.New("a run carries no usable method fingerprint")
	errGateLedgerUnknownOutcome  = errors.New("a run declares an outcome that is not one of the two there are")
	errGateLedgerCapContradicted = errors.New("a run's declared outcome contradicts its own ceiling and spend")

	errGateLedgerSurfaceTwice     = errors.New("a run records one surface's cost twice")
	errGateLedgerSurfaceNotInPlan = errors.New("a run records a surface the cost model does not plan for")
	errGateLedgerSurfaceMissing   = errors.New("a run leaves out a surface the cost model plans for")
	errGateLedgerClassNotAsPlaned = errors.New("a run files a surface under a class the model does not assign it")
	errGateLedgerUndecidedSurface = errors.New("a run records a surface whose class or document basis nobody has decided")

	errGateLedgerBoundMeasured  = errors.New("a surface that never reported records a measurement of its demand")
	errGateLedgerActualIsBound  = errors.New("a surface that reported records its cost as a bound rather than as a measurement")
	errGateLedgerTwoCostFigures = errors.New("a surface records both a measurement and a bound")

	errGateLedgerUnknownClass     = errors.New("a class the model does not define")
	errGateLedgerClassTwice       = errors.New("a run declares k twice for one class")
	errGateLedgerRatioUnitWrong   = errors.New("k is not denominated in the enforced unit per byte")
	errGateLedgerSeedUndecided    = errors.New("k was recorded for a class whose seed nobody has decided")
	errGateLedgerEditBare         = errors.New("a human edit to k carries no rationale")
	errGateLedgerEditUnmoved      = errors.New("a rationale is attached to a k that did not move")
	errGateLedgerEditPrior        = errors.New("a human edit to k misstates the value it replaced")
	errGateLedgerRaisedTooFast    = errors.New("k rose further in one run than the raise bound allows, and no reason is recorded")
	errGateLedgerLoweredTooFast   = errors.New("k fell further in one run than the lower bound allows, and no reason is recorded")
	errGateLedgerOverrideUnneeded = errors.New("a reason is recorded for a k move that is inside the bounds")
	errGateLedgerKNotInForce      = errors.New("a run records a k the model has never held")
	errGateLedgerKOutOfOrder      = errors.New("a run records a k the model held only before one an earlier run was quoted at, and no move back to it is recorded")

	errGateLedgerPlanNotTotal   = errors.New("the cost model does not decide, or explicitly leave undecided, every surface the manifest declares")
	errGateLedgerPlanUndeclared = errors.New("the cost model plans for a surface surfaces.yaml does not declare")
	errGateLedgerPlanTwice      = errors.New("the cost model plans one surface twice")
	errGateLedgerClassDefTwice  = errors.New("the cost model defines one class twice")

	errGateLedgerNoModel            = errors.New("no cost model is committed, so no quote is funded from a number anybody chose")
	errGateLedgerNoDeclaredSurfaces = errors.New("no surfaces are declared, so the cost model's coverage would be total over nothing")
	errGateLedgerNoTrackedFiles     = errors.New("git reported no tracked files, so every document would size to zero bytes")
	errGateLedgerBasisOwnsNothing   = errors.New("a surface owns no documents under the named basis, so it would be funded from zero bytes")

	errGateLedgerSeedUnratified   = errors.New("the committed model states a seed that the code ratifying seeds does not")
	errGateLedgerSeedRatifiedOnly = errors.New("a seed is ratified in code for a class the committed model does not define")
)

// gateLedgerFingerprintRE is the shape gateMethod.version emits.
var gateLedgerFingerprintRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ---------------------------------------------------------------------
// reading the two files
// ---------------------------------------------------------------------

// gateLedgerDecodeModel parses a cost model with unknown fields REFUSED.
func gateLedgerDecodeModel(raw []byte) (gateLedgerModel, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var m gateLedgerModel
	if err := dec.Decode(&m); err != nil {
		return gateLedgerModel{}, fmt.Errorf("parse %s: %w", gateLedgerModelPath, err)
	}
	if m.Schema != gateLedgerModelSchema {
		return gateLedgerModel{}, fmt.Errorf("%w: %q is not %q", errGateLedgerUnknownSchema, m.Schema, gateLedgerModelSchema)
	}
	return m, nil
}

// gateLedgerLoadModel resolves the canonical model against a repository root.
//
// The three outcomes are kept apart on purpose. ABSENT is (zero, false, nil) —
// a legal state, whose consequence is that no quote is funded. UNREADABLE is an
// error, never an absence: a model that exists and cannot be opened, parsed, or
// does not declare its schema is errGateUncheckable, which CLAUDE.md makes a
// FAILED gate rather than an empty window.
func gateLedgerLoadModel(root string) (gateLedgerModel, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateLedgerModelPath)))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return gateLedgerModel{}, false, nil
	case err != nil:
		return gateLedgerModel{}, true, fmt.Errorf("%w: read %s: %w", errGateUncheckable, gateLedgerModelPath, err)
	}
	m, err := gateLedgerDecodeModel(raw)
	if err != nil {
		return gateLedgerModel{}, true, fmt.Errorf("%w: %s: %w", errGateUncheckable, gateLedgerModelPath, err)
	}
	if err := gateLedgerValidateModel(m); err != nil {
		return m, true, fmt.Errorf("%s: %w", gateLedgerModelPath, err)
	}
	return m, true, nil
}

// gateLedgerDecodeLedger parses a ledger with unknown fields REFUSED. That is
// what makes the absent completion field and the absent per-surface ceiling
// absences with teeth: a field this schema does not have cannot be written into
// the file and ignored by the reader, which is how an unenforceable promise gets
// recorded as if it were a rule.
func gateLedgerDecodeLedger(raw []byte) (gateLedgerFile, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var l gateLedgerFile
	if err := dec.Decode(&l); err != nil {
		return gateLedgerFile{}, fmt.Errorf("parse %s: %w", gateLedgerPath, err)
	}
	if dec.More() {
		return gateLedgerFile{}, fmt.Errorf("parse %s: trailing content after the ledger object", gateLedgerPath)
	}
	if l.Schema != gateLedgerLedgerSchema {
		return gateLedgerFile{}, fmt.Errorf("%w: %q is not %q", errGateLedgerUnknownSchema, l.Schema, gateLedgerLedgerSchema)
	}
	return l, nil
}

// gateLedgerLoadLedger resolves the canonical ledger against a repository root,
// validating it against the model that governs it.
//
// A ledger present with no model is errGateUncheckable rather than an accepted
// file: every rule about classes, bases and predictions is stated against the
// model, so a ledger with none is a ledger over which no question can be asked.
func gateLedgerLoadLedger(root string) (gateLedgerFile, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateLedgerPath)))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return gateLedgerFile{}, false, nil
	case err != nil:
		return gateLedgerFile{}, true, fmt.Errorf("%w: read %s: %w", errGateUncheckable, gateLedgerPath, err)
	}
	l, err := gateLedgerDecodeLedger(raw)
	if err != nil {
		return gateLedgerFile{}, true, fmt.Errorf("%w: %s: %w", errGateUncheckable, gateLedgerPath, err)
	}
	model, present, err := gateLedgerLoadModel(root)
	if err != nil {
		return l, true, fmt.Errorf("%s cannot be checked: %w", gateLedgerPath, err)
	}
	if !present {
		return l, true, fmt.Errorf("%w: %s exists and %s does not, so nothing says what its classes, bases or predictions were supposed to be",
			errGateUncheckable, gateLedgerPath, gateLedgerModelPath)
	}
	if err := gateLedgerValidateLedger(model, l); err != nil {
		return l, true, fmt.Errorf("%s: %w", gateLedgerPath, err)
	}
	return l, true, nil
}

// ---------------------------------------------------------------------
// validating the model
// ---------------------------------------------------------------------

// gateLedgerValidateModel reports EVERY way a cost model is wrong.
//
// It accumulates rather than returning the first problem, because the reader is
// a human deciding whether to trust a number, and handing them one finding at a
// time out of a file they will re-run the validator against is "every finding
// reaches the human" failing quietly.
func gateLedgerValidateModel(m gateLedgerModel) error {
	var problems []error
	add := func(errs ...error) { problems = append(problems, errs...) }

	if m.Schema != gateLedgerModelSchema {
		add(fmt.Errorf("%w: %q", errGateLedgerUnknownSchema, m.Schema))
	}
	add(gateLedgerCheckModelQuantity("reserve", m.Reserve)...)
	add(gateLedgerCheckModelFactor("backstop_factor", m.BackstopFactor)...)

	classes := map[string]gateLedgerModelClass{}
	for _, c := range m.Classes {
		where := fmt.Sprintf("class %q", c.Name)
		if strings.TrimSpace(c.Name) == "" {
			add(fmt.Errorf("the model defines a class with no name"))
			continue
		}
		if c.Name == gateLedgerUndecided {
			add(fmt.Errorf("%s: %q is the reserved name for a decision nobody has made and cannot be a class", where, gateLedgerUndecided))
		}
		if _, seen := classes[c.Name]; seen {
			add(fmt.Errorf("%s: %w", where, errGateLedgerClassDefTwice))
		}
		classes[c.Name] = c
		add(gateLedgerCheckSeed(where, c.Seed)...)
		add(gateLedgerCheckModelEdits(where, c)...)
	}

	planned := map[string]bool{}
	for _, p := range m.Surfaces {
		where := fmt.Sprintf("surface %q", p.Name)
		if strings.TrimSpace(p.Name) == "" {
			add(fmt.Errorf("the model plans a surface with no name"))
			continue
		}
		if planned[p.Name] {
			add(fmt.Errorf("%s: %w", where, errGateLedgerPlanTwice))
		}
		planned[p.Name] = true

		switch {
		case p.Class == "":
			add(fmt.Errorf("%s: no class and no %q; there is no default and no third state", where, gateLedgerUndecided))
		case p.Class == gateLedgerUndecided:
		default:
			if _, ok := classes[p.Class]; !ok {
				add(fmt.Errorf("%s: %w (%q)", where, errGateLedgerUnknownClass, p.Class))
			}
		}
		switch {
		case p.DocumentBasis == "":
			add(fmt.Errorf("%s: no document_basis and no %q; a prediction that does not name the basis its size came from is not a legal recording", where, gateLedgerUndecided))
		case p.DocumentBasis == gateLedgerUndecided:
		default:
			if _, ok := gateLedgerBases[p.DocumentBasis]; !ok {
				add(fmt.Errorf("%s: %w (%q)", where, errGateLedgerUnknownBasis, p.DocumentBasis))
			}
		}
	}
	return errors.Join(problems...)
}

// gateLedgerCheckModelQuantity holds a model token quantity to the
// decided-or-explicitly-undecided shape. A zero that means "unset" is exactly
// the silence this refuses.
func gateLedgerCheckModelQuantity(where string, q gateLedgerModelQuantity) []error {
	var problems []error
	if q.Undecided {
		if strings.TrimSpace(q.Reason) == "" {
			problems = append(problems, fmt.Errorf("%s: undecided with no reason; an unmade decision has to say what is missing", where))
		}
		if q.Value != 0 || q.Unit != "" {
			problems = append(problems, fmt.Errorf("%s: undecided AND carrying a value; one of the two is a lie", where))
		}
		return problems
	}
	if q.Reason != "" {
		problems = append(problems, fmt.Errorf("%s: a decided quantity carries an undecided reason", where))
	}
	if q.Value <= 0 {
		problems = append(problems, fmt.Errorf("%s: %w (%d)", where, errGateLedgerNonPositive, q.Value))
	}
	switch {
	case q.Unit == "":
		problems = append(problems, fmt.Errorf("%s: %w", where, errGateLedgerUnitMissing))
	case q.Unit != gateLedgerEnforcedUnit:
		problems = append(problems, fmt.Errorf("%s: %w (%q, the ceiling is enforced in %q)", where, errGateLedgerUnitWrong, q.Unit, gateLedgerEnforcedUnit))
	}
	return problems
}

func gateLedgerCheckModelFactor(where string, f gateLedgerModelFactor) []error {
	var problems []error
	if f.Undecided {
		if strings.TrimSpace(f.Reason) == "" {
			problems = append(problems, fmt.Errorf("%s: undecided with no reason", where))
		}
		if f.Value != 0 {
			problems = append(problems, fmt.Errorf("%s: undecided AND carrying a value", where))
		}
		return problems
	}
	if f.Reason != "" {
		problems = append(problems, fmt.Errorf("%s: a decided factor carries an undecided reason", where))
	}
	if f.Value <= 1 {
		problems = append(problems, fmt.Errorf("%s: %v; a backstop at or below the ceiling is not a backstop", where, f.Value))
	}
	return problems
}

// gateLedgerCheckSeed is where the plan's k meets C18's counter.
//
// The seed's denomination is checked against two code constants, and stating the
// plan's denomination — total tokens per document token — fails both. That is
// the whole of failure 3: the two statements live in two untracked artifacts
// today and nothing can notice they disagree.
func gateLedgerCheckSeed(where string, s gateLedgerModelSeed) []error {
	var problems []error
	if s.Undecided {
		if strings.TrimSpace(s.Reason) == "" {
			problems = append(problems, fmt.Errorf("%s seed: undecided with no reason", where))
		}
		if s.Value != 0 || s.NumeratorUnit != "" || s.DenominatorUnit != "" || s.DerivedFrom != "" {
			problems = append(problems, fmt.Errorf("%s seed: undecided AND carrying a value", where))
		}
		return problems
	}
	if s.Reason != "" {
		problems = append(problems, fmt.Errorf("%s seed: a decided seed carries an undecided reason", where))
	}
	if s.Value <= 0 {
		problems = append(problems, fmt.Errorf("%s seed: %w (%v)", where, errGateLedgerNonPositive, s.Value))
	}
	if strings.TrimSpace(s.DerivedFrom) == "" {
		problems = append(problems, fmt.Errorf("%s seed: no derived_from; a seed with no measurement behind it is a number nobody chose", where))
	}
	problems = append(problems, gateLedgerCheckRatioUnits(where+" seed", s.NumeratorUnit, s.DenominatorUnit)...)
	return problems
}

// gateLedgerCheckSeedsAgainstRatification holds the committed model's seeds to
// the seeds ratified in code, in both directions.
//
// It is a statement about the COMMITTED model and is not part of
// gateLedgerValidateModel, deliberately. Every fixture under testdata/ states
// whatever seed its case needs — that is what makes a bound, a rationale or an
// out-of-order k testable at all — and holding those to a registry describing
// the real repository would say nothing about the real repository and would make
// the fixtures unwritable. What has to be governed is the file at the canonical
// path, which is where a seed rewritten in place actually funds a run.
//
// The registry is a parameter rather than read from the package variable so that
// both refusals can be exercised against a registry that is not empty. A check
// whose only run is over an empty map is a pass over zero assertions.
func gateLedgerCheckSeedsAgainstRatification(m gateLedgerModel, ratified map[string]gateLedgerSeedRatification) []error {
	var problems []error
	defined := map[string]bool{}
	for _, c := range m.Classes {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		defined[c.Name] = true
		r, ok := ratified[c.Name]
		switch {
		case c.Seed.Undecided && ok:
			problems = append(problems, fmt.Errorf(`class %q: %w: the code ratifies a seed of %v and %s records the seed as undecided.
One of the two is wrong about whether anybody has chosen this number, and a quote would be funded from whichever the reader happened to open`,
				c.Name, errGateLedgerSeedRatifiedOnly, r.Value, gateLedgerModelPath))
		case c.Seed.Undecided:
		case !ok:
			problems = append(problems, fmt.Errorf(`class %q: %w: %s states a seed of %v and gateLedgerRatifiedSeeds ratifies none.
A seed is what the whole chain of recorded moves hangs from, and a class with no recorded move — which is every class today — has nothing else to be checked against, so a seed supplied or rewritten here alone moves k with no record of the move anywhere`,
				c.Name, errGateLedgerSeedUnratified, gateLedgerModelPath, c.Seed.Value))
		case math.Abs(r.Value-c.Seed.Value) > gateLedgerEpsilon:
			problems = append(problems, fmt.Errorf(`class %q: %w: %s states %v and the code ratifies %v.
k moved and no diff records the move. A move off a ratified seed belongs in the class's k: list, where it is bounded, states what it replaced and carries its reason; rewriting the seed in place instead is the same change with the record left out`,
				c.Name, errGateLedgerSeedUnratified, gateLedgerModelPath, c.Seed.Value, r.Value))
		}
	}

	names := make([]string, 0, len(ratified))
	for name := range ratified {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		r := ratified[name]
		if !defined[name] {
			problems = append(problems, fmt.Errorf("class %q: %w: the code ratifies a seed of %v for a class %s does not define",
				name, errGateLedgerSeedRatifiedOnly, r.Value, gateLedgerModelPath))
			continue
		}
		// The code half is held to the same standard the file half is, or the
		// registry becomes the softer of the two places to write a bare number
		// into and the check has only moved the hole.
		if r.Value <= 0 {
			problems = append(problems, fmt.Errorf("class %q: %w (%v)", name, errGateLedgerNonPositive, r.Value))
		}
		if strings.TrimSpace(r.Provenance) == "" {
			problems = append(problems, fmt.Errorf("class %q: a ratified seed with no provenance; a number ratified on nothing is the hand-edited line with a longer path to it", name))
		}
	}
	return problems
}

// gateLedgerCheckRatioUnits is the dimensional check that makes a prediction
// mean anything: size (bytes) times k must come out in the unit the ceiling is
// enforced in.
func gateLedgerCheckRatioUnits(where, numerator, denominator string) []error {
	var problems []error
	if numerator != gateLedgerRatioNumerator {
		problems = append(problems, fmt.Errorf("%s: %w (numerator %q, the ceiling is enforced in %q — a k whose numerator counts what the agent READS cannot fund a quote a script can stop a run on)",
			where, errGateLedgerRatioUnitWrong, numerator, gateLedgerRatioNumerator))
	}
	if denominator != gateLedgerRatioDenominator {
		problems = append(problems, fmt.Errorf("%s: %w (denominator %q, a size is measured in %q)",
			where, errGateLedgerRatioUnitWrong, denominator, gateLedgerRatioDenominator))
	}
	return problems
}

// gateLedgerCheckModelEdits checks the chain of human moves of k: each one
// against the one before it, and the first against the seed.
//
// Anchoring EVERY edit to the seed is what makes a second move unrecordable, and
// unrecordable here does not mean "inconvenient" — it means the only way to
// write down a real move is to erase the previous one. The anchor therefore
// walks: the seed, then each recorded value in turn. What that buys is that the
// chain is a history rather than a snapshot, and a history is the only thing a
// run's recorded k can be checked against.
func gateLedgerCheckModelEdits(where string, c gateLedgerModelClass) []error {
	if len(c.K) == 0 {
		return nil
	}
	if c.Seed.Undecided {
		return []error{fmt.Errorf("%s k: %w; there is nothing for the first edit to have moved from", where, errGateLedgerSeedUndecided)}
	}
	var problems []error
	anchor := c.Seed.Value
	anchorIs := fmt.Sprintf("the seed at %v", anchor)
	for i, e := range c.K {
		kw := fmt.Sprintf("%s k[%d]", where, i)
		if e.Value <= 0 {
			problems = append(problems, fmt.Errorf("%s: %w (%v)", kw, errGateLedgerNonPositive, e.Value))
		}
		problems = append(problems, gateLedgerCheckRatioUnits(kw, e.NumeratorUnit, e.DenominatorUnit)...)
		if math.Abs(e.Previous-anchor) > gateLedgerEpsilon {
			problems = append(problems, fmt.Errorf("%s: %w (says it replaced %v; what stood before it is %s)", kw, errGateLedgerEditPrior, e.Previous, anchorIs))
		}
		problems = append(problems, gateLedgerCheckMoveRationale(kw, e.Previous, e.Value, e.Rationale)...)
		problems = append(problems, gateLedgerCheckTravel(kw, e.Previous, e.Value, e.BoundOverride)...)
		anchor = e.Value
		anchorIs = fmt.Sprintf("the move before it, k[%d], at %v", i, anchor)
	}
	return problems
}

// gateLedgerStatedK is every value the model says k has held for a class, oldest
// first: the seed, then each recorded move. The bool is whether the class has a
// decided seed at all.
//
// This list is the whole of what a run may have been quoted at. A number outside
// it was never in force, so no quote was ever funded from it.
func gateLedgerStatedK(c gateLedgerModelClass) ([]float64, bool) {
	if c.Seed.Undecided {
		return nil, false
	}
	chain := make([]float64, 0, len(c.K)+1)
	chain = append(chain, c.Seed.Value)
	for _, e := range c.K {
		chain = append(chain, e.Value)
	}
	return chain, true
}

// gateLedgerDescribeStatedK renders a class's chain for a human reading a
// refusal, so the message says what the model DOES state rather than only what
// it does not.
func gateLedgerDescribeStatedK(chain []float64) string {
	if len(chain) == 1 {
		return fmt.Sprintf("only the seed %v, with no edit recording a move away from it", chain[0])
	}
	parts := make([]string, 0, len(chain))
	parts = append(parts, fmt.Sprintf("the seed %v", chain[0]))
	for _, v := range chain[1:] {
		parts = append(parts, fmt.Sprintf("%v", v))
	}
	return strings.Join(parts, ", then ")
}

// gateLedgerCheckMoveRationale requires a rationale of a move and refuses one of
// a non-move. Both halves matter: without the first, k changes with no account
// of itself; without the second, the field becomes template boilerplate and
// stops meaning anything.
func gateLedgerCheckMoveRationale(where string, prior, value float64, rationale string) []error {
	moved := math.Abs(value-prior) > gateLedgerEpsilon
	stated := strings.TrimSpace(rationale) != ""
	switch {
	case moved && !stated:
		return []error{fmt.Errorf("%s: %w (%v -> %v)", where, errGateLedgerEditBare, prior, value)}
	case !moved && stated:
		return []error{fmt.Errorf("%s: %w (%v)", where, errGateLedgerEditUnmoved, value)}
	}
	return nil
}

// gateLedgerCheckTravel is how far k moved between two runs, against the bounds.
//
// The override arm is the only way past a bound, and it is deliberately not a
// boolean: it is the sentence a reviewer reads to find out why this move was
// bigger than the rules allow. An override on a move that did not need one is
// itself refused, so the escape hatch cannot become the default.
func gateLedgerCheckTravel(where string, prior, value float64, override string) []error {
	if prior <= 0 {
		// Whatever set k to a non-positive value was refused where it was
		// recorded; a percentage of it here would be noise on top of that.
		return nil
	}
	overridden := strings.TrimSpace(override) != ""
	high := prior * (1 + gateLedgerMaxRaise)
	low := prior * (1 - gateLedgerMaxLower)
	moved := (value/prior - 1) * 100

	switch {
	case value > high+gateLedgerEpsilon:
		if !overridden {
			return []error{fmt.Errorf("%s: %w (%v -> %v, %+.1f%%, bound %+.0f%%)", where, errGateLedgerRaisedTooFast, prior, value, moved, gateLedgerMaxRaise*100)}
		}
	case value < low-gateLedgerEpsilon:
		if !overridden {
			return []error{fmt.Errorf("%s: %w (%v -> %v, %+.1f%%, bound %+.0f%%)", where, errGateLedgerLoweredTooFast, prior, value, moved, -gateLedgerMaxLower*100)}
		}
	case overridden:
		return []error{fmt.Errorf("%s: %w (%v -> %v, %+.1f%%)", where, errGateLedgerOverrideUnneeded, prior, value, moved)}
	}
	return nil
}

// gateLedgerInForce is k as the model says it stands for a class right now: the
// newest edit if any have been made, otherwise the seed. The bool is whether it
// is decided at all.
func gateLedgerInForce(m gateLedgerModel, class string) (float64, bool) {
	for _, c := range m.Classes {
		if c.Name != class {
			continue
		}
		if c.Seed.Undecided {
			return 0, false
		}
		if n := len(c.K); n > 0 {
			return c.K[n-1].Value, true
		}
		return c.Seed.Value, true
	}
	return 0, false
}

// gateLedgerPlanCoversDeclaredSurfaces is invariant 5's totality rule as a
// function rather than as logic that lives only inside a test.
//
// The fan-out is read from the manifest, never from a hard-coded thirteen:
// another lane may add a surface entry this wave, and a surface appearing in
// surfaces.yaml with neither an assignment nor an undecided marker has to stop
// the gate rather than be quoted from a default.
func gateLedgerPlanCoversDeclaredSurfaces(m gateLedgerModel, declared []string) error {
	if len(declared) == 0 {
		return errGateLedgerNoDeclaredSurfaces
	}
	planned := map[string]bool{}
	for _, p := range m.Surfaces {
		planned[p.Name] = true
	}
	isDeclared := map[string]bool{}
	for _, name := range declared {
		isDeclared[name] = true
	}
	var problems []error
	for _, name := range declared {
		if !planned[name] {
			problems = append(problems, fmt.Errorf("%w: %q", errGateLedgerPlanNotTotal, name))
		}
	}
	for _, p := range m.Surfaces {
		if !isDeclared[p.Name] {
			problems = append(problems, fmt.Errorf("%w: %q", errGateLedgerPlanUndeclared, p.Name))
		}
	}
	return errors.Join(problems...)
}

// gateLedgerPlanFor is the model's two decisions for one surface.
func gateLedgerPlanFor(m gateLedgerModel, surface string) (gateLedgerModelSurfacePlan, bool) {
	for _, p := range m.Surfaces {
		if p.Name == surface {
			return p, true
		}
	}
	return gateLedgerModelSurfacePlan{}, false
}

// ---------------------------------------------------------------------
// validating the ledger
// ---------------------------------------------------------------------

// gateLedgerValidateLedger replays a whole ledger against the model that governs
// it and reports EVERY way it is wrong.
func gateLedgerValidateLedger(m gateLedgerModel, l gateLedgerFile) error {
	var problems []error
	add := func(errs ...error) { problems = append(problems, errs...) }

	if l.Schema != gateLedgerLedgerSchema {
		add(fmt.Errorf("%w: %q", errGateLedgerUnknownSchema, l.Schema))
	}

	seen := map[string]bool{}
	// priorK is k as it stood at the end of the previous recorded run, per
	// class: what a travel bound is measured from. Before any run, the anchor is
	// the model's seed, so the very first entry is bounded too whenever a seed
	// has been decided.
	priorK := map[string]float64{}
	for _, c := range m.Classes {
		if !c.Seed.Undecided {
			priorK[c.Name] = c.Seed.Value
		}
	}

	for i, run := range l.Runs {
		where := fmt.Sprintf("run %d", i)
		if strings.TrimSpace(run.RunID) == "" {
			add(fmt.Errorf("%s: %w", where, errGateLedgerRunUnidentified))
		} else {
			where = fmt.Sprintf("run %q", run.RunID)
			if seen[run.RunID] {
				add(fmt.Errorf("%s: %w", where, errGateLedgerRunTwice))
			}
			seen[run.RunID] = true
		}
		if !gateLedgerFingerprintRE.MatchString(run.MethodFingerprint) {
			add(fmt.Errorf("%s: %w (%q); C25 defers calibration until five real runs exist, and an entry with no stamp cannot later be told apart from one recorded under a rewritten prompt",
				where, errGateLedgerUnstamped, run.MethodFingerprint))
		}

		add(gateLedgerCheckQuantity(where+" ceiling", run.Ceiling, false, "")...)
		add(gateLedgerCheckQuantity(where+" reserve", run.Reserve, false, "")...)
		add(gateLedgerCheckQuantity(where+" spend", run.Spend, true, gateLedgerScopeRun)...)
		if run.Spend.Counter != "" && run.Spend.Counter != gateLedgerEnforcedCounter {
			add(fmt.Errorf("%s spend: %w (%q, the ceiling is enforced against %q)", where, errGateLedgerNotTheEnforcedCounter, run.Spend.Counter, gateLedgerEnforcedCounter))
		}

		switch run.Outcome {
		case gateLedgerOutcomeCompleted, gateLedgerOutcomeCapped:
		default:
			add(fmt.Errorf("%s: %w (%q)", where, errGateLedgerUnknownOutcome, run.Outcome))
		}
		// Cappedness is COMPUTED. The declared outcome is checked against the
		// computation, never trusted in place of it: a self-reported bit about
		// the one condition that must never look ordinary is a bit that can be
		// wrong for free.
		capped := run.Spend.Value >= run.Ceiling.Value
		computed := gateLedgerOutcomeCompleted
		if capped {
			computed = gateLedgerOutcomeCapped
		}
		if (run.Outcome == gateLedgerOutcomeCapped) != capped {
			add(fmt.Errorf("%s: %w (spend %d, ceiling %d: computed %s, declared %q)",
				where, errGateLedgerCapContradicted, run.Spend.Value, run.Ceiling.Value, computed, run.Outcome))
		}

		reported, uncheckable := gateLedgerReportedSurfaces(where, run)
		add(uncheckable...)

		add(gateLedgerCheckRunK(where, m, run, priorK)...)
		kInRun := map[string]float64{}
		for _, k := range run.K {
			kInRun[k.Class] = k.Value
		}

		add(gateLedgerCheckRunSurfaces(where, m, run, reported, kInRun)...)

		// THE ANCHOR FOR THE NEXT ENTRY IS WHAT THIS ONE WAS QUOTED AT. This is
		// the whole of "between two runs": without it every bound in the file is
		// measured against the model's seed forever, so a ledger can walk k
		// arbitrarily far from the previous entry as long as each entry stays
		// within a fixed distance of one number nobody re-reads.
		for _, k := range run.K {
			priorK[k.Class] = k.Value
		}
	}

	// Last, because it is a statement about the ledger as a whole against the
	// model as a whole rather than about any one entry.
	add(gateLedgerCheckKAgainstTheModel(m, l)...)
	return errors.Join(problems...)
}

// gateLedgerReportedSurfaces derives, from the run's OWN embedded verdict
// record, which surfaces the run actually reported on.
//
// It goes through gateIndexVerdicts rather than building its own map, for the
// reason stated there: the refusal of a duplicate has to be unanimous, because a
// duplicate that one caller rejects and another quietly resolves still reaches
// the report through whichever caller was the more forgiving. Last-wins over
// {FAILED, PASS} converts a FAILED into a PASS.
//
// Every failure here is errGateUncheckable and never an empty set. An absent,
// unreadable or self-contradictory verdict record is the question failing to be
// asked, and a check that quietly passed over it would leave a forged completion
// bit green.
func gateLedgerReportedSurfaces(where string, run gateLedgerRun) (map[string]gateSurfaceVerdict, []error) {
	if run.Receipt == nil {
		return nil, []error{fmt.Errorf("%s: %w: the entry carries no verdict record, so which surfaces finished cannot be derived and would have to be believed", where, errGateUncheckable)}
	}
	if !gateSHARE.MatchString(run.Receipt.Tree) {
		return nil, []error{fmt.Errorf("%s: %w: the entry's verdict record names no tree (%q), so nothing attaches these numbers to a state of the repository",
			where, errGateUncheckable, run.Receipt.Tree)}
	}
	index, err := gateIndexVerdicts(run.Receipt.Surfaces)
	if err != nil {
		return nil, []error{fmt.Errorf("%s: %w: %w", where, errGateUncheckable, err)}
	}
	return index, nil
}

// gateLedgerCheckRunK checks the k a run recorded per class: its denomination,
// that the model defines the class, and how far it travelled since the previous
// recorded run.
func gateLedgerCheckRunK(where string, m gateLedgerModel, run gateLedgerRun, priorK map[string]float64) []error {
	var problems []error
	declared := map[string]bool{}
	for _, k := range run.K {
		kw := fmt.Sprintf("%s k[%q]", where, k.Class)
		known := false
		for _, c := range m.Classes {
			if c.Name == k.Class {
				known = true
				if c.Seed.Undecided {
					problems = append(problems, fmt.Errorf("%s: %w", kw, errGateLedgerSeedUndecided))
				}
			}
		}
		if !known {
			problems = append(problems, fmt.Errorf("%s: %w (%q)", kw, errGateLedgerUnknownClass, k.Class))
		}
		if declared[k.Class] {
			problems = append(problems, fmt.Errorf("%s: %w", kw, errGateLedgerClassTwice))
		}
		declared[k.Class] = true

		if k.Value <= 0 {
			problems = append(problems, fmt.Errorf("%s: %w (%v)", kw, errGateLedgerNonPositive, k.Value))
		}
		problems = append(problems, gateLedgerCheckRatioUnits(kw, k.NumeratorUnit, k.DenominatorUnit)...)
		if prior, ok := priorK[k.Class]; ok {
			problems = append(problems, gateLedgerCheckTravel(kw, prior, k.Value, k.BoundOverride)...)
		} else if strings.TrimSpace(k.BoundOverride) != "" {
			problems = append(problems, fmt.Errorf("%s: %w (nothing precedes this value, so no bound was passed)", kw, errGateLedgerOverrideUnneeded))
		}
	}
	return problems
}

// gateLedgerCheckKAgainstTheModel is the one place the two files' accounts of k
// are held to each other, and it is held over EVERY entry rather than over the
// newest one.
//
// Everything above bounds how far k TRAVELS. Nothing above asked the prior
// question: was the number a run recorded ever in force at all? A quote is
// funded through gateLedgerInForce, so the value in force is the value a run is
// quoted at, and a recorded k the model cannot account for is a number no quote
// ever used.
//
// CHECKING ONLY THE NEWEST ENTRY IS NOT A WEAKER VERSION OF THIS; IT IS A HOLE
// THE SHAPE OF THE WHOLE CORPUS. A seed of 1.2 with no edit at all, and three
// runs recording 1.2, 1.32, 1.2: every step is inside the +25%/-10% travel
// bounds, the newest value equals the seed, and the ledger validates — while the
// middle run stands quoted at 1.32, a number the model has never held and no
// diff has ever recorded. A five-run corpus can walk out and back, so four of
// five entries can carry a k up to 25% away from anything a human chose. Those
// five entries are exactly what C25's deferred calibration is to be read from,
// and the travel bound cannot see this: it measures one step against the step
// before it and against nothing the model states. The entries' own arithmetic
// cannot see it either — predictions and ceiling are internally consistent at
// whatever k the entry names.
//
// SO THE MODEL'S CHAIN IS READ AS A TIMELINE AND THE LEDGER IS WALKED ALONG IT.
// The values k has been in force at, oldest first, are the seed then each
// recorded move. A run's k must be one of them, and the runs must visit them in
// order — a pointer that never goes backwards. That admits both of the timelines
// neither file distinguishes, because nothing says whether a committed edit
// landed before or after the newest run: the ledger may stop short of the end of
// the chain (the edit landed after the last recorded run) or reach it (it landed
// before). What it refuses is a value the chain does not contain, and a value the
// chain contains only EARLIER than one an earlier entry already reached — which
// is k moving back with no record of the move back, failure 7 one level out.
func gateLedgerCheckKAgainstTheModel(m gateLedgerModel, l gateLedgerFile) []error {
	var problems []error
	for _, c := range m.Classes {
		chain, decided := gateLedgerStatedK(c)
		if !decided {
			// Refused where the seed is. A comparison against an undecided
			// number here would be noise on top of that.
			continue
		}
		// pos is how far along the chain the ledger has already walked. It never
		// decreases: once an entry has been quoted at the value a move produced,
		// no later entry was quoted at what that move replaced.
		pos := 0
		for i, run := range l.Runs {
			for _, k := range run.K {
				if k.Class != c.Name {
					continue
				}
				where := fmt.Sprintf("run %d", i)
				if id := strings.TrimSpace(run.RunID); id != "" {
					where = fmt.Sprintf("run %q", id)
				}
				at := -1
				for j := pos; j < len(chain); j++ {
					if math.Abs(chain[j]-k.Value) <= gateLedgerEpsilon {
						at = j
						break
					}
				}
				if at >= 0 {
					pos = at
					continue
				}
				earlier := false
				for j := 0; j < pos; j++ {
					if math.Abs(chain[j]-k.Value) <= gateLedgerEpsilon {
						earlier = true
						break
					}
				}
				if earlier {
					problems = append(problems, fmt.Errorf(`class %q, %s: %w: this run was quoted at k=%v, which %s does state — but only before %v, which an earlier run was already quoted at.
k went back, and no edit records the move back. The two files agree about which numbers k has held and disagree about the order it held them in, so one of them is wrong about the history of a number every ceiling is derived from`,
						c.Name, where, errGateLedgerKOutOfOrder, k.Value, gateLedgerModelPath, chain[pos]))
					continue
				}
				problems = append(problems, fmt.Errorf(`class %q, %s: %w: this run was quoted at k=%v, and %s states %s.
%v is in none of those, so no quote was ever funded at it. Every move of k is supposed to leave a record; this one left none, and the five entries C25 defers calibration until would be read as evidence about a number nobody chose.
The travel bound is no defence here — it measures this entry against the entry before it, and against nothing the model states — and neither is the entry's own arithmetic, which is consistent at whatever k the entry names`,
					c.Name, where, errGateLedgerKNotInForce, k.Value, gateLedgerModelPath, gateLedgerDescribeStatedK(chain), k.Value))
			}
		}
	}
	return problems
}

// gateLedgerCheckRunSurfaces checks the run's per-surface records: that they
// cover exactly what the model plans, that each is funded from the basis and the
// class the model assigns, that each prediction is its size times the run's own
// k, and that a figure's shape matches what the verdict record says happened to
// that surface.
func gateLedgerCheckRunSurfaces(where string, m gateLedgerModel, run gateLedgerRun, reported map[string]gateSurfaceVerdict, kInRun map[string]float64) []error {
	var problems []error
	recorded := map[string]bool{}
	var predictedTotal, figuresTotal int64

	for _, s := range run.Surfaces {
		sw := fmt.Sprintf("%s surface %q", where, s.Surface)
		if recorded[s.Surface] {
			problems = append(problems, fmt.Errorf("%s: %w", sw, errGateLedgerSurfaceTwice))
		}
		recorded[s.Surface] = true

		plan, planned := gateLedgerPlanFor(m, s.Surface)
		if !planned {
			problems = append(problems, fmt.Errorf("%s: %w", sw, errGateLedgerSurfaceNotInPlan))
		}

		// Size, and the basis that decided which bytes.
		if s.Size.Unit != gateLedgerUnitBytes {
			problems = append(problems, fmt.Errorf("%s size: %w (%q)", sw, errGateLedgerSizeUnitWrong, s.Size.Unit))
		}
		if s.Size.Value <= 0 {
			problems = append(problems, fmt.Errorf("%s size: %w (%d)", sw, errGateLedgerNonPositive, s.Size.Value))
		}
		switch {
		case s.Size.Basis == "":
			problems = append(problems, fmt.Errorf("%s size: names no document basis, and no basis is the default", sw))
		case s.Size.Basis == gateLedgerUndecided:
			problems = append(problems, fmt.Errorf("%s size: %w", sw, errGateLedgerUnratifiedBasis))
		default:
			if _, ok := gateLedgerBases[s.Size.Basis]; !ok {
				problems = append(problems, fmt.Errorf("%s size: %w (%q)", sw, errGateLedgerUnknownBasis, s.Size.Basis))
			}
		}
		if planned {
			switch {
			case plan.DocumentBasis == gateLedgerUndecided:
				problems = append(problems, fmt.Errorf("%s: %w: the model leaves this surface's document basis undecided, so its bytes were chosen by whoever wrote the entry", sw, errGateLedgerUndecidedSurface))
			case plan.DocumentBasis != s.Size.Basis:
				problems = append(problems, fmt.Errorf("%s size: %w (measured over %q, the model ratifies %q)", sw, errGateLedgerUnratifiedBasis, s.Size.Basis, plan.DocumentBasis))
			}
			switch {
			case plan.Class == gateLedgerUndecided:
				problems = append(problems, fmt.Errorf("%s: %w: the model leaves this surface's class undecided, so its k was chosen by whoever wrote the entry", sw, errGateLedgerUndecidedSurface))
			case plan.Class != s.Class:
				problems = append(problems, fmt.Errorf("%s: %w (filed as %q, the model assigns %q)", sw, errGateLedgerClassNotAsPlaned, s.Class, plan.Class))
			}
		}

		// The prediction, and the arithmetic that makes "a prediction is a size
		// times a k" true by check rather than by description.
		problems = append(problems, gateLedgerCheckQuantity(sw+" predicted", s.Predicted, false, "")...)
		predictedTotal += s.Predicted.Value
		if k, ok := kInRun[s.Class]; ok && k > 0 && s.Size.Value > 0 {
			want := float64(s.Size.Value) * k
			if math.Abs(want-float64(s.Predicted.Value)) > 1 {
				problems = append(problems, fmt.Errorf("%s: %w (%d bytes x %v = %.1f, recorded %d)", sw, errGateLedgerPredictionArith, s.Size.Value, k, want, s.Predicted.Value))
			}
		} else if !ok {
			problems = append(problems, fmt.Errorf("%s: the run records no k for class %q, so its prediction rests on a number the entry does not carry", sw, s.Class))
		}

		// The measured figure, and whether it is allowed to be a measurement at
		// all. This is the half that cannot be declared: the receipt decides.
		_, finished := reported[s.Surface]
		switch {
		case s.Actual != nil && s.Floor != nil:
			problems = append(problems, fmt.Errorf("%s: %w", sw, errGateLedgerTwoCostFigures))
		case s.Actual != nil:
			problems = append(problems, gateLedgerCheckQuantity(sw+" actual", *s.Actual, true, gateLedgerScopeSurface)...)
			figuresTotal += s.Actual.Value
			if reported != nil && !finished {
				problems = append(problems, fmt.Errorf("%s: %w: the run's own verdict record holds no verdict for it, so the gate never reported on it and this number is what it had spent when the run stopped — strictly less than what the work needed, recorded as though it were the cost of finishing",
					sw, errGateLedgerBoundMeasured))
			}
		case s.Floor != nil:
			problems = append(problems, gateLedgerCheckQuantity(sw+" floor", *s.Floor, true, gateLedgerScopeSurface)...)
			figuresTotal += s.Floor.Value
			if reported != nil && finished {
				problems = append(problems, fmt.Errorf("%s: %w: the run's verdict record holds a verdict for it, so its cost is a measurement and filing it as a bound understates what this surface is known to demand",
					sw, errGateLedgerActualIsBound))
			}
		}
	}

	for _, p := range m.Surfaces {
		if !recorded[p.Name] {
			problems = append(problems, fmt.Errorf("%s: %w (%q); the ceiling is the sum over every planned surface, so a run that leaves one out was funded for work it does not account for",
				where, errGateLedgerSurfaceMissing, p.Name))
		}
	}
	for name := range reported {
		if !recorded[name] {
			problems = append(problems, fmt.Errorf("%s: %w: surface %q holds a verdict in the run's own record and no cost at all", where, errGateUncheckable, name))
		}
	}

	if figuresTotal > run.Spend.Value {
		problems = append(problems, fmt.Errorf("%s: %w (surfaces sum to %d, the run spent %d)", where, errGateLedgerActualsExceedRun, figuresTotal, run.Spend.Value))
	}
	if want := predictedTotal + run.Reserve.Value; want != run.Ceiling.Value {
		problems = append(problems, fmt.Errorf("%s: %w (predictions %d + reserve %d = %d, recorded ceiling %d)",
			where, errGateLedgerCeilingArith, predictedTotal, run.Reserve.Value, want, run.Ceiling.Value))
	}
	return problems
}

// gateLedgerCheckQuantity is every rule that applies to a number of tokens.
//
// measured says whether this figure is supposed to have come off a counter —
// spends, actuals and floors must have, predictions and ceilings must not.
// wantScope is which counter it must have come off.
func gateLedgerCheckQuantity(where string, q gateLedgerQuantity, measured bool, wantScope string) []error {
	var problems []error
	fail := func(sentinel error, detail string) {
		problems = append(problems, fmt.Errorf("%s: %w%s", where, sentinel, detail))
	}

	if q.Value <= 0 {
		fail(errGateLedgerNonPositive, fmt.Sprintf(" (%d)", q.Value))
	}
	switch {
	case q.Unit == "":
		fail(errGateLedgerUnitMissing, "")
	case q.Unit != gateLedgerEnforcedUnit:
		fail(errGateLedgerUnitWrong, fmt.Sprintf(" (%q, the ceiling is enforced in %q)", q.Unit, gateLedgerEnforcedUnit))
	}
	switch {
	case measured && q.Counter == "":
		fail(errGateLedgerUnmeasured, "")
	case !measured && q.Counter != "":
		fail(errGateLedgerPredictionMeasured, fmt.Sprintf(" (%q)", q.Counter))
	}
	if q.Counter == "" {
		return problems
	}
	spec, known := gateLedgerCounters[q.Counter]
	if !known {
		fail(errGateLedgerUnknownCounter, fmt.Sprintf(" (%q)", q.Counter))
		return problems
	}
	if q.Unit != "" && spec.Unit != q.Unit {
		fail(errGateLedgerUnitContradictsCounter, fmt.Sprintf(" (declared %q; %s measures %q — %s)", q.Unit, q.Counter, spec.Unit, spec.Provenance))
	}
	if wantScope != "" && spec.Scope != wantScope {
		fail(errGateLedgerCounterScope, fmt.Sprintf(" (%s is a %s counter, this is a %s figure)", q.Counter, spec.Scope, wantScope))
	}
	return problems
}

// ---------------------------------------------------------------------
// reading a run back — the one consumer path
// ---------------------------------------------------------------------

// The two kinds of per-surface figure. Every consumer is bound by the
// distinction; there is no field a consumer can read that flattens the two.
const (
	gateLedgerKindMeasurement = "measurement"
	gateLedgerKindBound       = "bound-on-demand"
)

// gateLedgerObservation is one surface's figure WITH what it is. The kind is not
// optional and not a flag on the side: a consumer that wants the number gets the
// kind in the same struct or gets nothing.
type gateLedgerObservation struct {
	Surface string
	Kind    string
	Value   int64
	Unit    string
	Counter string
}

// gateLedgerObservations is the only way to read a run's per-surface figures.
//
// The kind is DERIVED from the run's own verdict record on every read, not from
// which field the entry happens to have filled in. That matters even though the
// validator already refuses a mismatch: a consumer that trusted the field would
// be trusting the entry's account of itself, which is the shape this whole file
// exists to remove. Deriving it here means an entry that somehow reached a
// consumer unvalidated still cannot present a starved surface as a measurement.
func gateLedgerObservations(run gateLedgerRun) ([]gateLedgerObservation, error) {
	reported, problems := gateLedgerReportedSurfaces("run "+run.RunID, run)
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	var out []gateLedgerObservation
	for _, s := range run.Surfaces {
		q := s.Actual
		if q == nil {
			q = s.Floor
		}
		if q == nil {
			continue
		}
		kind := gateLedgerKindBound
		if _, finished := reported[s.Surface]; finished {
			kind = gateLedgerKindMeasurement
		}
		out = append(out, gateLedgerObservation{Surface: s.Surface, Kind: kind, Value: q.Value, Unit: q.Unit, Counter: q.Counter})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Surface < out[j].Surface })
	return out, nil
}

// ---------------------------------------------------------------------
// quoting a surface
// ---------------------------------------------------------------------

// The confidence a quote carries. Neither is "calibrated": C25 defers
// calibration until five real runs exist, so nothing here computes k from
// anything, and a quote that said otherwise would be claiming a number no rule
// produced.
const (
	gateLedgerConfidenceUnvalidated = "unvalidated: no recorded run has tested this number"
	gateLedgerConfidenceRuns        = "recorded runs behind this class: %d (C25 calibrates at five; k is a human edit until then)"
)

// gateLedgerQuote is one surface's funded prediction, with everything that
// decided it.
type gateLedgerQuote struct {
	Surface    string
	Class      string
	Basis      string
	SizeBytes  int64
	Predicted  int64
	Unit       string
	Confidence string
}

// gateLedgerQuoteSurface funds one surface, or refuses and says which decision
// is missing.
//
// The size is RESOLVED through the named basis on every quote and is never a
// transcribed number, which is what keeps a prediction honest about the bytes it
// was computed over. There is no path to a size that does not go through a
// ratified basis name: reaching for gateSurfaceDocuments directly is the
// one-line mistake with no local symptom that funds binary-and-viewer from
// 1,949,261 bytes of Go source.
func gateLedgerQuoteSurface(m gateLedgerModel, root, surface string, tracked []string, recordedRuns int) (gateLedgerQuote, error) {
	plan, planned := gateLedgerPlanFor(m, surface)
	if !planned {
		return gateLedgerQuote{}, fmt.Errorf("%w: surface %q", errGateLedgerPlanNotTotal, surface)
	}
	if plan.Class == gateLedgerUndecided {
		return gateLedgerQuote{}, fmt.Errorf("cannot quote surface %q: %w — its CLASS is undecided, and there is no default k to fall back to", surface, errGateLedgerUndecidedSurface)
	}
	if plan.DocumentBasis == gateLedgerUndecided {
		return gateLedgerQuote{}, fmt.Errorf("cannot quote surface %q: %w — its DOCUMENT BASIS is undecided, and no basis is the default", surface, errGateLedgerUndecidedSurface)
	}
	basis, known := gateLedgerBases[plan.DocumentBasis]
	if !known {
		return gateLedgerQuote{}, fmt.Errorf("cannot quote surface %q: %w (%q)", surface, errGateLedgerUnknownBasis, plan.DocumentBasis)
	}
	k, decided := gateLedgerInForce(m, plan.Class)
	if !decided {
		return gateLedgerQuote{}, fmt.Errorf("cannot quote surface %q: class %q has no decided k. %s",
			surface, plan.Class, "C18 pins the enforceable unit as output tokens; a seed in any other denomination cannot fund a quote, and this one has not been supplied")
	}
	size, err := basis.size(root, surface, tracked)
	if err != nil {
		return gateLedgerQuote{}, fmt.Errorf("cannot quote surface %q from basis %q: %w", surface, plan.DocumentBasis, err)
	}
	confidence := gateLedgerConfidenceUnvalidated
	if recordedRuns > 0 {
		confidence = fmt.Sprintf(gateLedgerConfidenceRuns, recordedRuns)
	}
	return gateLedgerQuote{
		Surface:    surface,
		Class:      plan.Class,
		Basis:      plan.DocumentBasis,
		SizeBytes:  size,
		Predicted:  int64(math.Round(float64(size) * k)),
		Unit:       gateLedgerEnforcedUnit,
		Confidence: confidence,
	}, nil
}

// gateLedgerQuoteRun is the run-level ceiling: every planned surface, plus the
// reserve. It refuses as a whole if any surface refuses — the ceiling is what
// the run is held to, and a ceiling summed over the surfaces that happened to
// quote is a ceiling that does not cover the run.
func gateLedgerQuoteRun(m gateLedgerModel, root string, tracked []string, recordedRuns int) (int64, []gateLedgerQuote, error) {
	if m.Reserve.Undecided {
		return 0, nil, fmt.Errorf("cannot quote a run: the reserve is undecided (%s)", m.Reserve.Reason)
	}
	var quotes []gateLedgerQuote
	var problems []error
	total := m.Reserve.Value
	for _, p := range m.Surfaces {
		q, err := gateLedgerQuoteSurface(m, root, p.Name, tracked, recordedRuns)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		quotes = append(quotes, q)
		total += q.Predicted
	}
	if len(problems) > 0 {
		return 0, nil, errors.Join(problems...)
	}
	return total, quotes, nil
}

// gateLedgerQuoteRunFromRoot is the whole path a gate run would take: read the
// committed model, read the committed ledger to learn how many runs stand behind
// the numbers, and quote.
//
// It is where invariant 7's two legal absences have their stated consequence. No
// model is errGateLedgerNoModel — a refusal, so that no run is funded from a
// number nobody chose. No ledger is not a refusal at all: the quote is produced
// and labelled as tested by no recorded run.
func gateLedgerQuoteRunFromRoot(root string) (int64, []gateLedgerQuote, error) {
	model, present, err := gateLedgerLoadModel(root)
	if err != nil {
		return 0, nil, err
	}
	if !present {
		return 0, nil, fmt.Errorf("%w: %s is absent", errGateLedgerNoModel, gateLedgerModelPath)
	}
	ledger, _, err := gateLedgerLoadLedger(root)
	if err != nil {
		return 0, nil, err
	}
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		return 0, nil, err
	}
	return gateLedgerQuoteRun(model, root, tracked, len(ledger.Runs))
}

// ---------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------

func gateLedgerFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(surfaceRepoRoot(t), filepath.FromSlash(gateLedgerFixtureDir), name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

// gateLedgerFixtureModel is the legal model every ledger fixture is checked
// against, read from disk rather than built in Go so that the committed fixture
// is the thing under test.
func gateLedgerFixtureModel(t *testing.T) gateLedgerModel {
	t.Helper()
	m, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
	if err != nil {
		t.Fatalf("decode model-legal.yaml: %v", err)
	}
	if err := gateLedgerValidateModel(m); err != nil {
		t.Fatalf("model-legal.yaml does not validate, so every ledger fixture below is being checked against a broken model: %v", err)
	}
	return m
}

// gateLedgerFixtureRun decodes one ledger fixture and returns its single run, so
// a test that wants to mutate one field in Go can start from a committed file.
func gateLedgerFixtureRun(t *testing.T, name string) (gateLedgerFile, gateLedgerRun) {
	t.Helper()
	l, err := gateLedgerDecodeLedger(gateLedgerFixture(t, name))
	if err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	if len(l.Runs) == 0 {
		t.Fatalf("%s records no run", name)
	}
	return l, l.Runs[0]
}

// gateLedgerPlace writes a model and (optionally) a ledger at their canonical
// paths under a throwaway root, so the loaders are exercised against the real
// paths rather than against testdata/.
func gateLedgerPlace(t *testing.T, root string, model, ledger []byte) {
	t.Helper()
	if model != nil {
		gateWrite(t, root, gateLedgerModelPath, string(model))
	}
	if ledger != nil {
		gateWrite(t, root, gateLedgerPath, string(ledger))
	}
}

// ---------------------------------------------------------------------
// the registries — what the names mean, checked in code
// ---------------------------------------------------------------------

// TestGateLedgerCounterRegistryIsCompleteAboutEveryNameItAdmits is the guard on
// the registry itself.
//
// A counter admitted with no scope, or with a unit outside the vocabulary, or
// with nothing behind it, would let a quantity cite a name that passes the
// lookup and means nothing. Provenance is asserted because a registry entry with
// no evidence behind it is exactly the hand-edited correspondence this design
// moved into code to avoid.
func TestGateLedgerCounterRegistryIsCompleteAboutEveryNameItAdmits(t *testing.T) {
	units := map[string]bool{gateLedgerUnitOutput: true, gateLedgerUnitInput: true}
	scopes := map[string]bool{gateLedgerScopeRun: true, gateLedgerScopeSurface: true}
	if len(gateLedgerCounters) == 0 {
		t.Fatal("the counter registry is empty, so every unit check below passes over nothing")
	}
	for name, spec := range gateLedgerCounters {
		if !units[spec.Unit] {
			t.Errorf("counter %q reports in %q, which is not a token unit this file knows", name, spec.Unit)
		}
		if !scopes[spec.Scope] {
			t.Errorf("counter %q has scope %q; a counter whose scope is unknown cannot catch a run total filed as one surface's figure", name, spec.Scope)
		}
		if strings.TrimSpace(spec.Provenance) == "" {
			t.Errorf(`counter %q records no provenance.

The whole reason this registry is code rather than configuration is that what a
counter measures is a FACT with a measurement behind it. An entry with nothing
behind it is the hand-edited correspondence wearing a Go type.`, name)
		}
	}
}

// TestGateLedgerTheEnforcedCounterAndItsInputContextTwinBothSayWhatTheyAre is
// the 2.9x trap written down in one place.
//
// Phase 0 reported 1,417,000 against a measured peak context of 1,416,932 and
// 489,218 output tokens. The two figures are different quantities and the design
// mixed them once already. This asserts that the registry keeps them apart: the
// enforced counter reports output tokens at run scope, and the harness figures
// report input context. Nothing else in this repository states both facts beside
// each other.
func TestGateLedgerTheEnforcedCounterAndItsInputContextTwinBothSayWhatTheyAre(t *testing.T) {
	enforced, ok := gateLedgerCounters[gateLedgerEnforcedCounter]
	if !ok {
		t.Fatalf("the counter the ceiling is enforced against (%q) is not in the registry", gateLedgerEnforcedCounter)
	}
	if enforced.Unit != gateLedgerEnforcedUnit {
		t.Errorf("%s reports %q, and the ceiling is enforced in %q; a quote in one unit and a guard in the other is what made phase 0's ceiling decorative",
			gateLedgerEnforcedCounter, enforced.Unit, gateLedgerEnforcedUnit)
	}
	if enforced.Scope != gateLedgerScopeRun {
		t.Errorf("%s has scope %q; C22 leaves the run-level ceiling as the only figure enforced mid-run, so the counter it is enforced against must be a run counter",
			gateLedgerEnforcedCounter, enforced.Scope)
	}
	for _, name := range []string{"harness.run_total_tokens", "harness.subagent_tokens"} {
		spec, ok := gateLedgerCounters[name]
		if !ok {
			t.Fatalf("%q is not registered; the input-context counters have to be nameable, or citing one is a plausible number rather than a named refusal", name)
		}
		if spec.Unit != gateLedgerUnitInput {
			t.Errorf(`%s is registered as measuring %q.

Phase 0 measured it at 1,417,000 against a peak context of 1,416,932: it counts
INPUT CONTEXT. Recording it as output tokens makes every derived ceiling roughly
3x too generous, and every check over the arithmetic stays green because bare
numbers divide correctly in any unit.`, name, spec.Unit)
		}
	}
}

// TestGateLedgerBasisRegistryIsCompleteAboutEveryNameItAdmits is the same guard
// for document bases: which bytes, and from what.
func TestGateLedgerBasisRegistryIsCompleteAboutEveryNameItAdmits(t *testing.T) {
	if len(gateLedgerBases) == 0 {
		t.Fatal("the basis registry is empty, so every basis check below passes over nothing")
	}
	for name, spec := range gateLedgerBases {
		if strings.TrimSpace(spec.Resolves) == "" {
			t.Errorf("basis %q does not say WHICH bytes it resolves to", name)
		}
		if strings.TrimSpace(spec.Provenance) == "" {
			t.Errorf("basis %q does not say where that comes from", name)
		}
		if spec.size == nil {
			t.Errorf("basis %q has no resolver, so naming it would produce no size and no refusal either", name)
		}
	}
}

// TestGateLedgerTheTwoUnreconstructibleBasesRefuseRatherThanApproximate is the
// check that stands directly in front of the wrong-bytes failure.
//
// gateSurfaceDocuments is the only surface-to-files function in the tree and it
// is correct for what it is for — the fingerprint's input set, where covering
// every file is the whole point. Reaching for it to compute a COST is a one-line
// mistake with no local symptom, and for site and binary-and-viewer it is
// provably the wrong basis: surfaces.yaml's own descriptions say the agents read
// the rendered DOM and the binary's embedded prose, neither of which is
// extractable from this tree.
//
// So the basis that names that document must refuse. If it quietly resolved to
// tracked files, binary-and-viewer would be funded from 1,949,261 bytes with
// every other rule green.
//
// THERE WERE TWO, and "site.rendered_dom" was the other. It is retired rather
// than still refusing here: the site stopped being a built artifact, so its
// description and its paths now name the same bytes and there is no
// unreconstructible document left to refuse for. See gateLedgerBases.
func TestGateLedgerTheUnreconstructibleBasisRefusesRatherThanApproximates(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, name := range []string{"binary.embedded_prose"} {
		spec := gateLedgerBases[name]
		got, err := spec.size(root, "binary-and-viewer", tracked)
		if !errors.Is(err, errGateLedgerBasisUnavailable) {
			t.Errorf(`basis %q answered %d bytes instead of refusing (%v).

This document cannot be reconstructed from the committed tree — surface.json's
commands array carries only {path, short, flags}: no long text, no flag usage, no
error-message strings, no templates. A basis that answers anyway is answering with
somebody else's bytes.`, name, got, err)
		}
	}
	// The available basis must actually answer, or the refusals above would be
	// the only behaviour and the registry would be uniformly useless.
	size, err := gateLedgerBases["manifest.tracked_files"].size(root, "readme", tracked)
	if err != nil {
		t.Fatalf("manifest.tracked_files could not size readme: %v", err)
	}
	if size <= 0 {
		t.Fatalf("manifest.tracked_files sized readme at %d bytes", size)
	}
}

// TestGateLedgerTheTrackedFilesBasisIsTheManifestsAnswerAndNothingElse pins the
// one available basis to gateSurfaceDocuments rather than to a private walk, so
// that a file joining a surface moves its size with no second edit.
func TestGateLedgerTheTrackedFilesBasisIsTheManifestsAnswerAndNothingElse(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	documents, err := gateSurfaceDocuments(root, tracked)
	if err != nil {
		t.Fatalf("gateSurfaceDocuments: %v", err)
	}
	for _, surface := range []string{"readme", "changelog", "roadmap"} {
		var want int64
		for _, rel := range documents[surface] {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatalf("stat %s: %v", rel, err)
			}
			want += info.Size()
		}
		got, err := gateLedgerBases["manifest.tracked_files"].size(root, surface, tracked)
		if err != nil {
			t.Fatalf("size %s: %v", surface, err)
		}
		if got != want {
			t.Errorf("manifest.tracked_files sized %q at %d bytes; the manifest resolves it to %d", surface, got, want)
		}
	}
}

// ---------------------------------------------------------------------
// the fixtures, and the table that has to cover them
// ---------------------------------------------------------------------

// gateLedgerFixtureCase is one recorded model or ledger and what reading it must
// produce. wantErr nil means the fixture is legal.
type gateLedgerFixtureCase struct {
	file    string
	wantErr error
	why     string
}

// gateLedgerModelFixtures is every committed model fixture.
var gateLedgerModelFixtures = []gateLedgerFixtureCase{
	{file: "model-legal.yaml", why: "the model every ledger fixture is checked against"},
	{file: "model-basis-undecided.yaml", why: "legal, and unable to quote the surface whose basis is undecided"},
	{file: "model-class-undecided.yaml", why: "legal, and unable to quote the surface whose class is undecided"},
	{file: "model-seed-undecided.yaml", why: "legal, and able to quote nothing at all — the committed model's shape"},
	{file: "model-k-past-the-bound-with-a-reason.yaml", why: "a move past the bound is legal when the record says why"},
	{file: "model-k-moved-twice.yaml", why: "k moved twice and both moves are recorded — the second move is what a single-edit schema cannot hold"},

	{file: "model-k-in-the-plans-denomination.yaml", wantErr: errGateLedgerRatioUnitWrong, why: "the plan's k, refused on both units"},
	{file: "model-unknown-basis.yaml", wantErr: errGateLedgerUnknownBasis, why: "a basis name nothing defines"},
	{file: "model-k-past-the-bound.yaml", wantErr: errGateLedgerRaisedTooFast, why: "1.2 to 3.5 in one edit, no reason recorded"},
	{file: "model-override-nobody-needed.yaml", wantErr: errGateLedgerOverrideUnneeded, why: "a reason on a move inside the bounds"},
	{file: "model-k-with-no-rationale.yaml", wantErr: errGateLedgerEditBare, why: "k moved and said nothing about why"},
	{file: "model-k-misstates-what-it-replaced.yaml", wantErr: errGateLedgerEditPrior, why: "an edit that invents its own predecessor"},
	{file: "model-k-second-move-anchored-to-the-seed.yaml", wantErr: errGateLedgerEditPrior, why: "a second move naming the seed as what it replaced, when what it replaced is the move before it"},
}

// gateLedgerLedgerFixtures is every committed ledger fixture.
var gateLedgerLedgerFixtures = []gateLedgerFixtureCase{
	{file: "legal-run.json", why: "a completed run, every surface reporting"},
	{file: "legal-capped-run.json", why: "a capped run: the surface that never reported carries a floor, not an actual"},

	{file: "capped-run-declared-completed.json", wantErr: errGateLedgerCapContradicted, why: "the outcome contradicts the entry's own ceiling and spend"},
	{file: "starved-surface-filed-as-a-measurement.json", wantErr: errGateLedgerBoundMeasured, why: "a surface the run never reported on, filed as a measurement of demand"},
	{file: "finished-surface-filed-as-a-bound.json", wantErr: errGateLedgerActualIsBound, why: "the reverse: a surface that reported, filed as a bound"},
	{file: "receipt-missing.json", wantErr: errGateUncheckable, why: "no verdict record at all — the question cannot be asked"},
	{file: "receipt-names-no-tree.json", wantErr: errGateUncheckable, why: "a verdict record naming no tree"},
	{file: "receipt-reports-a-surface-twice.json", wantErr: errGateUncheckable, why: "a surface holding two verdicts, refused rather than resolved"},
	{file: "spend-from-the-input-counter.json", wantErr: errGateLedgerUnitContradictsCounter, why: "the run spend read off the input-context counter"},
	{file: "actual-from-the-per-agent-input-counter.json", wantErr: errGateLedgerUnitContradictsCounter, why: "a surface figure read off the per-agent input counter"},
	{file: "unknown-counter.json", wantErr: errGateLedgerUnknownCounter, why: "a counter nothing knows the denomination of"},
	{file: "run-total-filed-as-one-surfaces-figure.json", wantErr: errGateLedgerCounterScope, why: "a run counter cited for one surface's figure"},
	{file: "prediction-read-off-a-counter.json", wantErr: errGateLedgerPredictionMeasured, why: "a prediction is not an actual copied into the wrong column"},
	{file: "figure-with-no-counter.json", wantErr: errGateLedgerUnmeasured, why: "a measured figure that names nothing it was read from"},
	{file: "bare-number.json", wantErr: errGateLedgerUnitMissing, why: "a number with no unit at all"},
	{file: "unknown-basis.json", wantErr: errGateLedgerUnknownBasis, why: "a size measured over a basis nothing defines"},
	{file: "size-from-a-basis-the-model-does-not-ratify.json", wantErr: errGateLedgerUnratifiedBasis, why: "a real basis, ratified for a different surface"},
	{file: "unstamped-run.json", wantErr: errGateLedgerUnstamped, why: "an entry that cannot be told apart from one recorded under a rewritten prompt"},
	{file: "prediction-is-not-size-times-k.json", wantErr: errGateLedgerPredictionArith, why: "a prediction that is not its size times the run's own k"},
	{file: "ceiling-is-not-the-sum.json", wantErr: errGateLedgerCeilingArith, why: "a ceiling that is not the predictions plus the reserve"},
	{file: "figures-exceed-the-run-spend.json", wantErr: errGateLedgerActualsExceedRun, why: "surfaces summing to more than the whole run spent"},
	{file: "a-planned-surface-left-out.json", wantErr: errGateLedgerSurfaceMissing, why: "a run funded for work it does not account for"},
	{file: "a-surface-the-model-does-not-plan-for.json", wantErr: errGateLedgerSurfaceNotInPlan, why: "a surface nobody planned, funded anyway"},
	{file: "k-jumped-between-two-runs.json", wantErr: errGateLedgerRaisedTooFast, why: "the parked attempt's recorded failure: k jumping between two entries"},
	{file: "k-the-model-never-held.json", wantErr: errGateLedgerKNotInForce, why: "three runs walking k out to 1.32 and back, every step inside the bounds, against a model that has only ever held 1.2"},
}

// TestGateLedgerFixtureTableCoversTheFixtureDirectory keeps the tables above and
// the directory in step.
//
// Without it a fixture can be added and never read — which is the shape of a
// suite that looks thorough and asserts nothing about the file somebody just
// wrote — and a fixture can be deleted while its row keeps passing against a
// read error nobody looks at.
func TestGateLedgerFixtureTableCoversTheFixtureDirectory(t *testing.T) {
	dir := filepath.Join(surfaceRepoRoot(t), filepath.FromSlash(gateLedgerFixtureDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", gateLedgerFixtureDir, err)
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			onDisk[e.Name()] = true
		}
	}
	if len(onDisk) == 0 {
		t.Fatalf("%s holds no fixtures; every table row below would fail to read rather than assert anything", gateLedgerFixtureDir)
	}
	inTable := map[string]bool{}
	for _, tc := range append(append([]gateLedgerFixtureCase(nil), gateLedgerModelFixtures...), gateLedgerLedgerFixtures...) {
		if inTable[tc.file] {
			t.Errorf("%s appears in the fixture table twice", tc.file)
		}
		inTable[tc.file] = true
		if !onDisk[tc.file] {
			t.Errorf("the table names %s and no such fixture exists", tc.file)
		}
		if strings.TrimSpace(tc.why) == "" {
			t.Errorf("%s has no stated reason for existing", tc.file)
		}
	}
	for name := range onDisk {
		if !inTable[name] {
			t.Errorf("%s/%s is never read by any test", gateLedgerFixtureDir, name)
		}
	}
}

// TestGateLedgerReadsEveryRecordedModelAsTheTableSays replays every committed
// model fixture.
func TestGateLedgerReadsEveryRecordedModelAsTheTableSays(t *testing.T) {
	for _, tc := range gateLedgerModelFixtures {
		t.Run(tc.file, func(t *testing.T) {
			m, err := gateLedgerDecodeModel(gateLedgerFixture(t, tc.file))
			if err == nil {
				err = gateLedgerValidateModel(m)
			}
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("%s (%s) was refused: %v", tc.file, tc.why, err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("%s (%s) was accepted or refused for the wrong reason.\nwant: %v\n got: %v", tc.file, tc.why, tc.wantErr, err)
			}
		})
	}
}

// TestGateLedgerReadsEveryRecordedLedgerAsTheTableSays replays every committed
// ledger fixture against the legal model.
func TestGateLedgerReadsEveryRecordedLedgerAsTheTableSays(t *testing.T) {
	model := gateLedgerFixtureModel(t)
	for _, tc := range gateLedgerLedgerFixtures {
		t.Run(tc.file, func(t *testing.T) {
			l, err := gateLedgerDecodeLedger(gateLedgerFixture(t, tc.file))
			if err == nil {
				err = gateLedgerValidateLedger(model, l)
			}
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("%s (%s) was refused: %v", tc.file, tc.why, err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("%s (%s) was accepted or refused for the wrong reason.\nwant: %v\n got: %v", tc.file, tc.why, tc.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------
// cappedness, completion, and the two kinds of figure
// ---------------------------------------------------------------------

// TestGateLedgerDerivesCappednessRatherThanBelievingIt is invariant 3 exercised
// in both directions.
//
// A self-reported bit about the one condition that must never look ordinary is a
// bit that can be wrong for free, and the natural schema puts it in as a boolean
// while the natural tests exercise behaviour conditional on the boolean rather
// than the boolean's truth — a suite fully green about the wrong bit.
func TestGateLedgerDerivesCappednessRatherThanBelievingIt(t *testing.T) {
	model := gateLedgerFixtureModel(t)
	for _, tc := range []struct {
		name     string
		fixture  string
		outcome  string
		accepted bool
	}{
		{"a capped run declared capped", "legal-capped-run.json", gateLedgerOutcomeCapped, true},
		{"a capped run declared completed", "legal-capped-run.json", gateLedgerOutcomeCompleted, false},
		{"a completed run declared completed", "legal-run.json", gateLedgerOutcomeCompleted, true},
		{"a completed run declared capped", "legal-run.json", gateLedgerOutcomeCapped, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _ := gateLedgerFixtureRun(t, tc.fixture)
			l.Runs[0].Outcome = tc.outcome
			err := gateLedgerValidateLedger(model, l)
			if tc.accepted && errors.Is(err, errGateLedgerCapContradicted) {
				t.Fatalf("an outcome matching the entry's own arithmetic was refused: %v", err)
			}
			if !tc.accepted && !errors.Is(err, errGateLedgerCapContradicted) {
				t.Fatalf(`an outcome contradicting the entry's own ceiling and spend was accepted: %v

Whether a run reached its ceiling follows from the ceiling and the spend the
entry itself records. Believing the declaration is how a run that stopped early
reads as evidence that the work is cheap.`, err)
			}
		})
	}
}

// TestGateLedgerCompletionComesFromTheEntrysOwnVerdictRecord is invariant 4.
//
// A run that stops early leaves declared surfaces holding no verdict, and the
// tree already names that state exactly. So which surfaces a stopped run never
// finished is a fact about the run's verdict record, not a claim the cost entry
// gets to make — and the two directions are both wrong in a way that misleads a
// human deciding what k should be.
func TestGateLedgerCompletionComesFromTheEntrysOwnVerdictRecord(t *testing.T) {
	model := gateLedgerFixtureModel(t)

	t.Run("a surface that never reported cannot record a measurement", func(t *testing.T) {
		l, _ := gateLedgerFixtureRun(t, "legal-capped-run.json")
		rec := &l.Runs[0].Surfaces[2]
		if rec.Floor == nil {
			t.Fatalf("fixture drift: %q was expected to carry a floor", rec.Surface)
		}
		rec.Actual, rec.Floor = rec.Floor, nil
		if err := gateLedgerValidateLedger(model, l); !errors.Is(err, errGateLedgerBoundMeasured) {
			t.Fatalf(`a starved surface's partial figure was accepted as a measurement of its demand: %v

It came in cheap BECAUSE it was starved. Filed as an actual, a human comparing
predicted against actual sees the expensive surfaces coming in cheap and lowers
their class — the exact inversion of the truth.`, err)
		}
	})

	t.Run("a surface that reported cannot record a bound", func(t *testing.T) {
		l, _ := gateLedgerFixtureRun(t, "legal-run.json")
		rec := &l.Runs[0].Surfaces[0]
		rec.Floor, rec.Actual = rec.Actual, nil
		if err := gateLedgerValidateLedger(model, l); !errors.Is(err, errGateLedgerActualIsBound) {
			t.Fatalf("a finished surface's cost was accepted as a bound: %v", err)
		}
	})

	t.Run("moving the verdict, not the number, changes which is legal", func(t *testing.T) {
		// The sharpest form: the cost side is untouched and only the verdict
		// record moves. If completion were a field the entry filled in, this
		// would be invisible.
		l, _ := gateLedgerFixtureRun(t, "legal-run.json")
		l.Runs[0].Receipt.Surfaces = l.Runs[0].Receipt.Surfaces[:2]
		err := gateLedgerValidateLedger(model, l)
		if !errors.Is(err, errGateLedgerBoundMeasured) {
			t.Fatalf(`dropping a surface's verdict left its actual legal: %v

Completion is derived from the verdict record on every read. A cost entry whose
numbers stay put while the run's own record says a surface never reported must
stop being a legal recording.`, err)
		}
	})
}

// TestGateLedgerTreatsAnAbsentOrSelfContradictoryVerdictRecordAsUncheckable is
// the case the method critique made mandatory.
//
// There is no receipt path anywhere in this tree. An implementer who discovered
// that mid-lane had only two moves, both of which defeat the check: skip when no
// receipt is present, which leaves a forged completion bit green, or refuse every
// entry, which makes the ledger permanently uninhabitable. Carrying the record
// inside the entry is the third move, and these are its failure modes — each an
// errGateUncheckable and a FAILED gate, never a check that quietly passes over
// nothing.
func TestGateLedgerTreatsAnAbsentOrSelfContradictoryVerdictRecordAsUncheckable(t *testing.T) {
	model := gateLedgerFixtureModel(t)
	for _, tc := range []struct {
		name   string
		mutate func(*gateLedgerRun)
	}{
		{"no verdict record at all", func(r *gateLedgerRun) { r.Receipt = nil }},
		{"a verdict record naming no tree", func(r *gateLedgerRun) { r.Receipt.Tree = "" }},
		{"a tree that is not an object name", func(r *gateLedgerRun) { r.Receipt.Tree = "HEAD~1" }},
		{"one surface holding two verdicts", func(r *gateLedgerRun) {
			r.Receipt.Surfaces = append(r.Receipt.Surfaces, r.Receipt.Surfaces[0])
		}},
		{"a verdict for a surface carrying no cost", func(r *gateLedgerRun) {
			r.Surfaces = r.Surfaces[:2]
			r.Ceiling.Value = r.Surfaces[0].Predicted.Value + r.Surfaces[1].Predicted.Value + r.Reserve.Value
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, _ := gateLedgerFixtureRun(t, "legal-run.json")
			tc.mutate(&l.Runs[0])
			if err := gateLedgerValidateLedger(model, l); !errors.Is(err, errGateUncheckable) {
				t.Fatalf(`the entry was read as an ordinary recording: %v

"The question could not be asked" is not a pass over zero assertions. CLAUDE.md
makes it a FAILED gate.`, err)
			}
		})
	}
}

// TestGateLedgerDerivationOfWhoReportedGoesThroughTheSharedIndexer pins the
// derivation to gateIndexVerdicts rather than to a private map.
//
// The refusal of a duplicate has to be unanimous. `held[v.Surface] = v` resolves
// a duplicate by last-wins, and last-wins over {FAILED, PASS} converts a FAILED
// into a PASS — so a caller that built its own index would let a duplicated
// verdict decide completion by slice order.
func TestGateLedgerDerivationOfWhoReportedGoesThroughTheSharedIndexer(t *testing.T) {
	_, run := gateLedgerFixtureRun(t, "legal-run.json")
	run.Receipt.Surfaces = append(run.Receipt.Surfaces, gateSurfaceVerdict{
		Surface: "readme", Verdict: gateVerdictFailed, Fingerprint: "sha256:" + strings.Repeat("d", 64),
	})
	if _, err := gateLedgerObservations(run); !errors.Is(err, errGateDuplicateVerdict) {
		t.Fatalf(`a consumer read figures out of a run whose verdict record holds two answers for one surface: %v

Indexing it would have kept one of them by position, and the losing verdict does
not merely lose a comparison — it is gone.`, err)
	}
}

// TestGateLedgerNoConsumerCanReadAFigureWithoutBeingToldWhichKindItIs is the
// second half of invariant 4: the distinction has to survive the read, not only
// the write.
func TestGateLedgerNoConsumerCanReadAFigureWithoutBeingToldWhichKindItIs(t *testing.T) {
	_, run := gateLedgerFixtureRun(t, "legal-capped-run.json")
	got, err := gateLedgerObservations(run)
	if err != nil {
		t.Fatalf("read a legal capped run: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("read %d figures out of a three-surface run", len(got))
	}
	kinds := map[string]string{}
	for _, o := range got {
		if o.Kind != gateLedgerKindMeasurement && o.Kind != gateLedgerKindBound {
			t.Fatalf("surface %q's figure came back as %q, which is neither of the two kinds there are", o.Surface, o.Kind)
		}
		kinds[o.Surface] = o.Kind
	}
	if kinds["roadmap"] != gateLedgerKindBound {
		t.Errorf(`roadmap never reported in this run and its figure came back as %q.

A number produced by work that was cut short is a BOUND ON DEMAND. Every consumer
is bound by that distinction, and a consumer that reads it as a measurement
concludes the surface is cheap.`, kinds["roadmap"])
	}
	if kinds["readme"] != gateLedgerKindMeasurement {
		t.Errorf("readme reported in this run and its figure came back as %q", kinds["readme"])
	}
}

// TestGateLedgerObservationsDeriveTheKindEvenWhenTheEntryDisagrees is the same
// property proved against an entry the validator would have rejected.
//
// A consumer that trusted which FIELD was filled in would be trusting the
// entry's account of itself. Deriving on every read means an entry that somehow
// reached a consumer unvalidated still cannot present a starved surface as a
// measurement.
func TestGateLedgerObservationsDeriveTheKindEvenWhenTheEntryDisagrees(t *testing.T) {
	_, run := gateLedgerFixtureRun(t, "legal-capped-run.json")
	rec := &run.Surfaces[2]
	rec.Actual, rec.Floor = rec.Floor, nil // the forgery the validator refuses

	got, err := gateLedgerObservations(run)
	if err != nil {
		t.Fatalf("read the run: %v", err)
	}
	for _, o := range got {
		if o.Surface == "roadmap" && o.Kind != gateLedgerKindBound {
			t.Fatalf(`a figure filed under "actual" for a surface that never reported came back as %q.

The kind is a fact about the run's verdict record, not about which key the entry
used.`, o.Kind)
		}
	}
}

// TestGateLedgerSchemaHasNowhereToDeclareCompletionOrAPerSurfaceCeiling is the
// absence given teeth.
//
// Unknown fields are refused on decode, so neither a completion bit nor a
// per-surface ceiling can be written into a ledger and quietly ignored by the
// reader — which is how an unenforceable promise gets recorded as if it were a
// rule. C22 abolished the per-surface allowance; completion is derived.
func TestGateLedgerSchemaHasNowhereToDeclareCompletionOrAPerSurfaceCeiling(t *testing.T) {
	for _, field := range []string{`"completed": true`, `"ceiling": {"value": 10, "unit": "output_tokens"}`} {
		body := strings.Replace(string(gateLedgerFixture(t, "legal-run.json")),
			`"surface": "readme",`, `"surface": "readme", `+field+`,`, 1)
		if _, err := gateLedgerDecodeLedger([]byte(body)); err == nil {
			t.Fatalf(`a ledger declaring %s was decoded without complaint.

A field this schema does not have must not be writable and ignorable. Completion
is derived from the run's own verdict record, and C22 left the run-level ceiling
as the only figure enforced mid-run.`, field)
		}
	}
}

// TestGateLedgerSurfaceRecordDeclaresNoFieldThatFlattensTheTwoKinds is the
// structural half of "there is no field a consumer can read that flattens the
// two".
//
// A single `tokens` or `cost` field would make the distinction advisory: every
// consumer would reach for the one that is always populated, and the bound would
// be read as a measurement by everyone who did not read this file.
func TestGateLedgerSurfaceRecordDeclaresNoFieldThatFlattensTheTwoKinds(t *testing.T) {
	rt := reflect.TypeOf(gateLedgerSurfaceRecord{})
	allowed := map[string]bool{"Surface": true, "Class": true, "Size": true, "Predicted": true, "Actual": true, "Floor": true}
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !allowed[f.Name] {
			t.Errorf(`gateLedgerSurfaceRecord declares %s.

Every field on this struct is either the surface's identity, its plan, its
prediction, or one of the two mutually exclusive kinds of measured figure. A
field outside that set is a way to read a number without being told which kind it
is — which is the distinction this schema exists to keep.`, f.Name)
		}
	}
	for _, name := range []string{"Actual", "Floor"} {
		f, _ := rt.FieldByName(name)
		if f.Type.Kind() != reflect.Ptr {
			t.Errorf("%s is %s rather than a pointer; the two kinds have to be distinguishable from ABSENT, or a zero reads as a figure of zero", name, f.Type.Kind())
		}
	}
}

// ---------------------------------------------------------------------
// the unit the ceiling is enforced in
// ---------------------------------------------------------------------

// TestGateLedgerRefusesTheDenominationThePlanStatesForK is failure 3, which is
// already live in the design rather than hypothetical.
//
// The plan defines k as "total tokens an agent spends per token of document it
// reads — its own document, plus the shared surface.json and delta, plus
// reasoning and written findings", and builds k~3 from three roughly equal
// parts, two of which are input. C18 pins the enforceable unit as output tokens
// because that is what budget.spent() returns. The two statements live in two
// untracked scratchpad artifacts today and there is no place where the seeds and
// the enforced counter are written down beside each other, so nothing can notice
// that they disagree. This is that place.
//
// The refusal must name BOTH units, because both are wrong and a reader who
// fixes one is still holding a k that cannot fund a quote.
func TestGateLedgerRefusesTheDenominationThePlanStatesForK(t *testing.T) {
	m, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-k-in-the-plans-denomination.yaml"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	err = gateLedgerValidateModel(m)
	if !errors.Is(err, errGateLedgerRatioUnitWrong) {
		t.Fatalf(`the plan's own k was accepted: %v

Multiplying output-denominated seeds against document sizes produces a ceiling
around 3x the real output demand. The release that ships this ships a ceiling
nothing can trip, and the gate reports a budget it is not being held to.`, err)
	}
	for _, unit := range []string{gateLedgerUnitTotal, gateLedgerUnitDocument} {
		if !strings.Contains(err.Error(), unit) {
			t.Errorf("the refusal does not name %q, so a reader cannot tell which half of the denomination is wrong:\n%v", unit, err)
		}
	}
}

// TestGateLedgerRefusesAQuantityInAnyUnitButTheEnforcedOne is invariant 2 at the
// level of a single number.
func TestGateLedgerRefusesAQuantityInAnyUnitButTheEnforcedOne(t *testing.T) {
	model := gateLedgerFixtureModel(t)
	for _, unit := range []string{gateLedgerUnitInput, gateLedgerUnitTotal, gateLedgerUnitBytes} {
		t.Run(unit, func(t *testing.T) {
			l, _ := gateLedgerFixtureRun(t, "legal-run.json")
			l.Runs[0].Ceiling.Unit = unit
			if err := gateLedgerValidateLedger(model, l); !errors.Is(err, errGateLedgerUnitWrong) {
				t.Fatalf("a ceiling denominated in %q was accepted: %v", unit, err)
			}
		})
	}
}

// TestGateLedgerRefusesASpendReadFromACounterTheCeilingIsNotEnforcedAgainst is
// failure 1 at its source.
//
// The driver records the run's spend from whichever counter is easy to read.
// Only one counter is the one the ceiling is compared against, and a spend read
// from any other is a spend the ceiling was never held to — even when the units
// happen to agree, which is the case this catches that the unit rule cannot.
func TestGateLedgerRefusesASpendReadFromACounterTheCeilingIsNotEnforcedAgainst(t *testing.T) {
	model := gateLedgerFixtureModel(t)
	l, _ := gateLedgerFixtureRun(t, "legal-run.json")
	// A counter with the right unit and the right scope, and still not the one
	// the ceiling is enforced against.
	gateLedgerCounters["fixture.other_run_output"] = gateLedgerCounterSpec{
		Unit: gateLedgerUnitOutput, Scope: gateLedgerScopeRun, Provenance: "declared by this test only",
	}
	t.Cleanup(func() { delete(gateLedgerCounters, "fixture.other_run_output") })

	l.Runs[0].Spend.Counter = "fixture.other_run_output"
	if err := gateLedgerValidateLedger(model, l); !errors.Is(err, errGateLedgerNotTheEnforcedCounter) {
		t.Fatalf(`a spend read from a counter the ceiling is not enforced against was accepted: %v

Every unit rule is satisfied here. What is not satisfied is that the number the
run was actually stopped on is the number recorded.`, err)
	}
}

// ---------------------------------------------------------------------
// how far k may travel, and what a move has to carry
// ---------------------------------------------------------------------

// TestGateLedgerBoundsHowFarKTravelsInOneRun pins the bounds at the stated
// percentages, both edges, both directions, and the override in both of its
// arms.
//
// The bounds bind whatever moved the number, including a human, because they are
// a property of how far k may travel between two runs and not of what computed
// the new value. The parked attempt left them unenforced on the reasoning that
// they belonged to the deferred calibrator, and the result was a ledger that
// accepted k going from 3.5 to 9.9 in one entry with no rule broken.
func TestGateLedgerBoundsHowFarKTravelsInOneRun(t *testing.T) {
	const prior = 1.2
	for _, tc := range []struct {
		name     string
		value    float64
		override string
		wantErr  error
	}{
		{name: "unmoved", value: prior},
		{name: "a raise exactly on the bound", value: prior * 1.25},
		{name: "a fall exactly on the bound", value: prior * 0.90},
		{name: "a raise past the bound", value: prior * 1.26, wantErr: errGateLedgerRaisedTooFast},
		{name: "a fall past the bound", value: prior * 0.89, wantErr: errGateLedgerLoweredTooFast},
		{name: "3.5 to 9.9, the recorded failure", value: 9.9, wantErr: errGateLedgerRaisedTooFast},
		{name: "a raise past the bound, with the reason", value: prior * 2, override: "the 2.9x unit correction"},
		{name: "a fall past the bound, with the reason", value: prior * 0.5, override: "the 2.9x unit correction"},
		{name: "a reason for a move nobody bounded", value: prior * 1.1, override: "because the template has the field", wantErr: errGateLedgerOverrideUnneeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.Join(gateLedgerCheckTravel("k", prior, tc.value, tc.override)...)
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("%v -> %v (override %q) was refused: %v", prior, tc.value, tc.override, err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("%v -> %v (override %q):\nwant %v\n got %v", prior, tc.value, tc.override, tc.wantErr, err)
			}
		})
	}
}

// TestGateLedgerTheTravelBoundsAreAsymmetric is the reason the two numbers are
// not equal, asserted rather than left in a comment.
//
// Under-funding FAILS a gate; over-funding wastes headroom nobody spends. So a
// rise may be chased faster than a fall, and a move of the same MAGNITUDE is
// legal upward and refused downward.
func TestGateLedgerTheTravelBoundsAreAsymmetric(t *testing.T) {
	const prior = 1.0
	const magnitude = 0.20
	if err := errors.Join(gateLedgerCheckTravel("k", prior, prior*(1+magnitude), "")...); err != nil {
		t.Fatalf("a +%.0f%% move was refused: %v", magnitude*100, err)
	}
	if err := errors.Join(gateLedgerCheckTravel("k", prior, prior*(1-magnitude), "")...); !errors.Is(err, errGateLedgerLoweredTooFast) {
		t.Fatalf(`a -%.0f%% move was allowed while the +%.0f%% move also was.

The two bounds have collapsed into one. A run that halves k on one surprising
sample is exactly what the smaller downward bound exists to prevent, and a
symmetric pair cannot prevent it.`, magnitude*100, magnitude*100)
	}
}

// TestGateLedgerRequiresAMoveToCarryItsReasonAndRefusesOneThatDidNot is the
// required-and-refused symmetry on the rationale.
//
// Without the first half, k changes with no account of itself and the next run
// is quoted from a number with no history behind it. Without the second, the
// field becomes template boilerplate and stops meaning anything.
func TestGateLedgerRequiresAMoveToCarryItsReasonAndRefusesOneThatDidNot(t *testing.T) {
	for _, tc := range []struct {
		name      string
		value     float64
		rationale string
		wantErr   error
	}{
		{name: "a move with its reason", value: 1.3, rationale: "the first wave came in over quote"},
		{name: "a move with nothing", value: 1.3, wantErr: errGateLedgerEditBare},
		{name: "a non-move with nothing", value: 1.2},
		{name: "a non-move with a reason", value: 1.2, rationale: "boilerplate", wantErr: errGateLedgerEditUnmoved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.Join(gateLedgerCheckMoveRationale("k", 1.2, tc.value, tc.rationale)...)
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("refused: %v", err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestGateLedgerBoundsKBetweenTwoRecordedRuns is the same rule where the parked
// attempt actually lost it: not in the model, but between two entries.
//
// EVERY CASE BELOW KEEPS THE SEED AND THE PREVIOUS ENTRY'S K AT DIFFERENT
// NUMBERS, and that is the point of the test rather than an incidental detail.
// An earlier version of this test used the fixture pair 1.2 -> 3.0 against a
// model whose seed is also 1.2, so the anchor was the same number whichever way
// it was obtained — and the whole test, and in fact the whole file, passed
// unchanged with the carry-forward of k from one entry to the next deleted
// outright. A test that cannot tell the rule it names from the rule beside it is
// asserting neither.
//
// So the seed is 1.5, the first entry is quoted at 1.35, and each case is chosen
// so that the two possible anchors give OPPOSITE answers.
func TestGateLedgerBoundsKBetweenTwoRecordedRuns(t *testing.T) {
	const (
		seed  = 1.5
		first = 1.35 // -10% from the seed: on the lower bound, and legal.
	)
	// twoRuns is the legal single-run fixture recorded twice, with each run's k
	// restated and every number that follows from it re-derived.
	twoRuns := func(t *testing.T, k0, k1 float64, override string) gateLedgerFile {
		t.Helper()
		l := gateLedgerRunsAtK(t, "falsification-dense", k0, k1)
		l.Runs[1].K[0].BoundOverride = override
		return l
	}

	t.Run("a step legal only against the entry before it", func(t *testing.T) {
		// 1.35 -> 1.25 is -7.4%, inside the -10% lower bound. Measured against
		// the model's SEED of 1.5 the same value is -16.7% and would be refused,
		// so an implementation that anchors every entry to the seed turns a legal
		// record into a red gate and there is no way to write the truth down.
		const second = 1.25
		m := gateLedgerReseeded(t, "falsification-dense", seed,
			gateLedgerMove(first, seed, "the first wave came in under quote"),
			gateLedgerMove(second, first, "and the second under it again"))
		if err := gateLedgerValidateLedger(m, twoRuns(t, first, second, "")); err != nil {
			t.Fatalf(`a -7.4%% step from the previous entry's k was refused: %v

Measured against the entry before it, %v -> %v is inside the lower bound.
Measured against the model's seed of %v it is -16.7%% and outside it. The bound is
between two RUNS, so the anchor is the previous entry.`, err, first, second, seed)
		}
	})

	t.Run("a step illegal only against the entry before it", func(t *testing.T) {
		// 1.35 -> 1.8 is +33%, past the raise bound, and the entry records no
		// reason. Measured from the SEED of 1.5 it is +20% and perfectly legal —
		// so if the anchor is not carried forward from the previous ENTRY, this
		// ledger validates and k has travelled a third further than any rule
		// allows with nothing objecting.
		//
		// The model states 1.8 (with the reason its own bound demands), so the
		// chain check has no complaint and the travel bound between the two
		// entries is the only thing that can refuse this.
		const second = 1.8
		m := gateLedgerReseeded(t, "falsification-dense", seed,
			gateLedgerMove(first, seed, "the first wave came in under quote"),
			gateLedgerModelEdit{
				Value: second, NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes,
				Previous: first, Rationale: "the 2.9x unit correction, not a trend",
				BoundOverride: "the 2.9x unit correction, not a trend",
			})
		err := gateLedgerValidateLedger(m, twoRuns(t, first, second, ""))
		if !errors.Is(err, errGateLedgerRaisedTooFast) {
			t.Fatalf(`k moved from %v to %v between two recorded entries, +33%%, with no reason recorded, and that was not refused: %v

Measured against the model's seed of %v this move is +20%% and legal. It is only
illegal measured against the k the PREVIOUS ENTRY was quoted at, which is what a
bound "between two runs" means. If this passes, the anchor is not being carried
forward from one entry to the next and every bound in this file is really a bound
against a single fixed number.`, first, second, err, seed)
		}
	})

	t.Run("and the same move is legal once the entry says why", func(t *testing.T) {
		// Or the bound would be a prohibition and the 2.9x correction would be
		// unmakeable. Two things have to be true, not one: the entry carries the
		// reason the bound demands, AND the model states the value, because a run
		// can only have been quoted at a number the model has held.
		const second = 1.8
		const why = "the 2.9x unit correction, not a trend"
		m := gateLedgerReseeded(t, "falsification-dense", seed,
			gateLedgerMove(first, seed, "the first wave came in under quote"),
			gateLedgerModelEdit{
				Value: second, NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes,
				Previous: first, Rationale: why, BoundOverride: why,
			})
		if err := gateLedgerValidateLedger(m, twoRuns(t, first, second, why)); err != nil {
			t.Fatalf("the same move, recorded in both files with its reason, was still refused: %v", err)
		}
	})

	t.Run("the parked attempt's own recorded failure", func(t *testing.T) {
		// 1.2 -> 3.0 across two entries, against a model that has never held 3.0
		// and never recorded a move to it. Both refusals are the point: the jump
		// is unbounded AND the number was never in force.
		model := gateLedgerFixtureModel(t)
		l, err := gateLedgerDecodeLedger(gateLedgerFixture(t, "k-jumped-between-two-runs.json"))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(l.Runs) != 2 {
			t.Fatalf("fixture drift: %d runs, want 2", len(l.Runs))
		}
		err = gateLedgerValidateLedger(model, l)
		if !errors.Is(err, errGateLedgerRaisedTooFast) {
			t.Fatalf(`k moved from 1.2 to 3.0 between two recorded runs and the bound did not object: %v

Every rule about a single entry is satisfied by both of these runs. What is not
satisfied is anything about the pair.`, err)
		}
		if !errors.Is(err, errGateLedgerKNotInForce) {
			t.Fatalf(`k moved to 3.0 and the model, which states only the seed 1.2, did not object: %v

A move that big is also a move to a number nobody wrote down. The bound says the
step is too large; this says the destination was never in force.`, err)
		}
	})
}

// TestGateLedgerAnchorsTheFirstRecordedRunToTheModelsSeed closes the gap a
// first entry would otherwise open: with nothing before it, a run could record
// any k at all.
func TestGateLedgerAnchorsTheFirstRecordedRunToTheModelsSeed(t *testing.T) {
	model := gateLedgerFixtureModel(t)
	l, _ := gateLedgerFixtureRun(t, "legal-run.json")
	// 1.2 -> 9.9 in the FIRST entry, with the prediction arithmetic kept
	// consistent so nothing else objects.
	l.Runs[0].K[0].Value = 9.9
	l.Runs[0].Surfaces[0].Predicted.Value = int64(math.Round(float64(l.Runs[0].Surfaces[0].Size.Value) * 9.9))
	l.Runs[0].Ceiling.Value = l.Runs[0].Surfaces[0].Predicted.Value +
		l.Runs[0].Surfaces[1].Predicted.Value + l.Runs[0].Surfaces[2].Predicted.Value + l.Runs[0].Reserve.Value
	if err := gateLedgerValidateLedger(model, l); !errors.Is(err, errGateLedgerRaisedTooFast) {
		t.Fatalf(`the first recorded run carried a k nothing bounded: %v

Before any run, the anchor is the model's seed. Without it a ledger's opening
entry can start anywhere, and every later bound is measured from a number nobody
chose.`, err)
	}
}

// TestGateLedgerASeedMovesOnlyInACodeDiffAndTheCommittedModelIsHeldToIt closes
// the last way k can change with no record of the change.
//
// Failure 7 is "k is changed without a record", and every rule above reaches it
// through a MOVE: an edit in the class's k: list, checked against what stood
// before it, bounded, carrying its reason. The seed is what that chain hangs
// from, and a class that has recorded no move has nothing else the seed can be
// checked against. That is not a corner: all three classes in the committed
// model are in exactly that state today. So the number one line of
// gate-cost-model.yaml funds every ceiling from can be rewritten from 0.9 to 2.4
// with every other rule in this file green, and the ledger cannot object either,
// because a class with no entries has nothing to contradict.
//
// The failures below are built from the COMMITTED model rather than from a
// fixture, so they are the edit a human would actually make to the file that
// actually funds a run.
func TestGateLedgerASeedMovesOnlyInACodeDiffAndTheCommittedModelIsHeldToIt(t *testing.T) {
	root := surfaceRepoRoot(t)
	committed, present, err := gateLedgerLoadModel(root)
	if err != nil || !present {
		t.Fatalf("the committed model at %s did not load: present=%v err=%v", gateLedgerModelPath, present, err)
	}
	if len(committed.Classes) == 0 {
		t.Fatalf("%s defines no class, so there is no seed to hold to anything", gateLedgerModelPath)
	}
	class := committed.Classes[0].Name

	// The seed a human would supply, and the rewrite of it. Both are legal
	// YAML, both satisfy every other rule, and the second records no move.
	const supplied, rewritten = 0.9, 2.4
	withSeed := func(v float64) gateLedgerModel {
		m := committed
		m.Classes = append([]gateLedgerModelClass(nil), committed.Classes...)
		m.Classes[0].Seed = gateLedgerModelSeed{
			Value: v, NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes,
			DerivedFrom: "phase 0: 489,218 output tokens over the documents that run read",
		}
		return m
	}
	ratifiedAt := func(v float64) map[string]gateLedgerSeedRatification {
		return map[string]gateLedgerSeedRatification{class: {Value: v, Provenance: "phase 0, the output-denominated figure C18 pins"}}
	}

	t.Run("the committed model and the code agree today", func(t *testing.T) {
		if err := errors.Join(gateLedgerCheckSeedsAgainstRatification(committed, gateLedgerRatifiedSeeds)...); err != nil {
			t.Fatalf(`%s and gateLedgerRatifiedSeeds disagree about what k starts at: %v

Whichever of the two a reader opens, they are reading a number the other one
denies. Both have to be edited in the same diff.`, gateLedgerModelPath, err)
		}
	})

	t.Run("a seed supplied in the file alone is refused", func(t *testing.T) {
		m := withSeed(supplied)
		if err := gateLedgerValidateModel(m); err != nil {
			t.Fatalf("the constructed edit is not otherwise legal, so this test would prove nothing: %v", err)
		}
		err := errors.Join(gateLedgerCheckSeedsAgainstRatification(m, gateLedgerRatifiedSeeds)...)
		if !errors.Is(err, errGateLedgerSeedUnratified) {
			t.Fatalf(`class %q was given a seed of %v by editing %s and nothing else, and that was accepted: %v

Every other rule is satisfied — the units are the enforced pair, the value is
positive, derived_from is filled in — which is exactly why nothing else can
catch it.`, class, supplied, gateLedgerModelPath, err)
		}
	})

	t.Run("and rewriting that seed in place is refused", func(t *testing.T) {
		ratified := ratifiedAt(supplied)
		if err := errors.Join(gateLedgerCheckSeedsAgainstRatification(withSeed(supplied), ratified)...); err != nil {
			t.Fatalf("a seed stated in both files at the same number was refused: %v", err)
		}
		m := withSeed(rewritten)
		if err := gateLedgerValidateModel(m); err != nil {
			t.Fatalf("the rewrite is not otherwise legal, so this test would prove nothing: %v", err)
		}
		if len(m.Classes[0].K) != 0 {
			t.Fatalf("class %q records %d moves; the case under test is the class that records none", class, len(m.Classes[0].K))
		}
		if err := gateLedgerValidateLedger(m, gateLedgerFile{Schema: gateLedgerLedgerSchema}); err != nil {
			t.Fatalf("the empty ledger this class has is not otherwise legal: %v", err)
		}
		err := errors.Join(gateLedgerCheckSeedsAgainstRatification(m, ratified)...)
		if !errors.Is(err, errGateLedgerSeedUnratified) {
			t.Fatalf(`class %q's seed was rewritten in place from %v to %v, no move was recorded anywhere, and that was accepted: %v

k moved %+.0f%% in one line of a hand-edited file. The k: list did not object
because it is empty, and the ledger did not object because a class with no
recorded run has nothing to disagree with. This is the whole of failure 7,
reached through the seed instead of through a move.`,
				class, supplied, rewritten, err, (rewritten/supplied-1)*100)
		}
	})

	t.Run("a seed ratified in code alone is refused too", func(t *testing.T) {
		// The reverse direction, and it is not symmetry for its own sake: a
		// registry entry the file does not state is a number in force that no
		// reviewer of the configuration ever sees.
		err := errors.Join(gateLedgerCheckSeedsAgainstRatification(committed, ratifiedAt(supplied))...)
		if !errors.Is(err, errGateLedgerSeedRatifiedOnly) {
			t.Fatalf("the code ratified a seed for %q while %s records it as undecided, and that was accepted: %v", class, gateLedgerModelPath, err)
		}
		err = errors.Join(gateLedgerCheckSeedsAgainstRatification(committed,
			map[string]gateLedgerSeedRatification{"a-class-nobody-declared": {Value: supplied, Provenance: "phase 0"}})...)
		if !errors.Is(err, errGateLedgerSeedRatifiedOnly) {
			t.Fatalf("the code ratified a seed for a class the model does not define, and that was accepted: %v", err)
		}
	})

	t.Run("and the code half is held to what the file half is", func(t *testing.T) {
		// Or the registry is simply the softer of the two places to write a bare
		// number into, and the hole has moved rather than closed.
		err := errors.Join(gateLedgerCheckSeedsAgainstRatification(withSeed(supplied),
			map[string]gateLedgerSeedRatification{class: {Value: supplied}})...)
		if err == nil {
			t.Fatal("a seed ratified in code with no provenance behind it was accepted")
		}
		err = errors.Join(gateLedgerCheckSeedsAgainstRatification(withSeed(supplied),
			map[string]gateLedgerSeedRatification{class: {Value: 0, Provenance: "phase 0"}})...)
		if !errors.Is(err, errGateLedgerNonPositive) {
			t.Fatalf("a ratified seed of zero was accepted: %v", err)
		}
	})
}

// ---------------------------------------------------------------------
// the model's two decisions, total or explicitly unmade
// ---------------------------------------------------------------------

// TestGateLedgerCommittedModelDecidesOrUnmakesEveryDeclaredSurface is invariant
// 5 against the real manifest.
//
// It reads the fan-out from gateDeclaredSurfaces rather than from a hard-coded
// thirteen, because another lane may add a surface entry this wave — and a
// surface appearing in surfaces.yaml with neither an assignment nor an
// undecided marker has to stop the gate rather than be quoted from a default.
func TestGateLedgerCommittedModelDecidesOrUnmakesEveryDeclaredSurface(t *testing.T) {
	root := surfaceRepoRoot(t)
	model, present, err := gateLedgerLoadModel(root)
	if err != nil {
		t.Fatalf("the committed cost model does not validate: %v", err)
	}
	if !present {
		t.Fatalf("%s is not committed; every rule in this file would be about a file nobody wrote", gateLedgerModelPath)
	}
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("read the declared surfaces: %v", err)
	}
	if len(declared) == 0 {
		t.Fatal("surfaces.yaml declares nothing, so this check would pass over zero surfaces")
	}

	planned := map[string]bool{}
	for _, p := range model.Surfaces {
		planned[p.Name] = true
	}
	isDeclared := map[string]bool{}
	for _, name := range declared {
		isDeclared[name] = true
		if !planned[name] {
			t.Errorf(`surfaces.yaml declares %q and %s says nothing about it.

There is no third state. Either the surface has a class and a document basis, or
each missing decision is recorded as undecided — and an undecided one refuses to
be quoted rather than falling back to a default nobody chose.`, name, gateLedgerModelPath)
		}
	}
	for name := range planned {
		if !isDeclared[name] {
			t.Errorf("%s plans for %q, which surfaces.yaml does not declare", gateLedgerModelPath, name)
		}
	}
}

// TestGateLedgerTheUnmadeDecisionsAreExactlyThese writes the open questions down
// where they cannot be forgotten.
//
// The list is asserted rather than merely allowed, so that answering one is a
// visible edit to this test and not a number that quietly appeared. Answering
// binary-and-viewer's CLASS while its BASIS stays undecided must not pass here
// silently: the two are named together because deciding the class alone is what
// arms the wrong-bytes failure — until it has a class, the undecided-class
// refusal is the only thing shielding the largest document in the repository.
func TestGateLedgerTheUnmadeDecisionsAreExactlyThese(t *testing.T) {
	model, _, err := gateLedgerLoadModel(surfaceRepoRoot(t))
	if err != nil {
		t.Fatalf("read the committed cost model: %v", err)
	}
	wantClass := []string{
		"binary-and-viewer", "ci-merge-gate-template", "contributing",
		"exported-skills", "install-scripts", "release-notes", "release-procedure",
	}
	// ONE, AND IT USED TO BE TWO. `site` was the other, and its basis was
	// decided by DELETION rather than by measurement: surfaces.yaml called the
	// surface "the RENDERED DOM of a real build" while its paths claimed 375,332
	// bytes of TSX, so the right basis existed only as an ephemeral gate
	// artifact. The site is two static HTML pages with no build now — the
	// description and the paths name the same bytes — so its basis is
	// manifest.tracked_files like every other agreeing surface. That is the
	// shape an answer here can take: the disagreement went away, rather than
	// somebody picking a number for it.
	wantBasis := []string{"binary-and-viewer"}

	var gotClass, gotBasis []string
	for _, p := range model.Surfaces {
		if p.Class == gateLedgerUndecided {
			gotClass = append(gotClass, p.Name)
		}
		if p.DocumentBasis == gateLedgerUndecided {
			gotBasis = append(gotBasis, p.Name)
		}
	}
	sort.Strings(gotClass)
	sort.Strings(gotBasis)
	if !reflect.DeepEqual(gotClass, wantClass) {
		t.Errorf("the surfaces with no class decided are now %v; this test says %v.\n\nIf a class was decided, decide its document basis in the same edit and move both lists.", gotClass, wantClass)
	}
	if !reflect.DeepEqual(gotBasis, wantBasis) {
		t.Errorf(`the surfaces with no document basis decided are now %v; this test says %v.

surfaces.yaml's own text is what puts a surface here: binary-and-viewer is "cobra
Short and Long text ... the error messages and hints, and the viewer templates"
against paths claiming 1,949,261 bytes of source, so the bytes the prose is
judged against are not the bytes the paths resolve to. An entry LEAVES this list
when that disagreement is resolved — which can mean measuring the right basis, or
(as it did for site) changing the surface until its description and its paths name
the same bytes.`, gotBasis, wantBasis)
	}
	if len(gotClass) == 0 && len(gotBasis) == 0 {
		t.Log("every decision is now made; this test has become a pin on a finished partition rather than a register of open questions")
	}
}

// TestGateLedgerCommittedModelQuotesNothingItHasNotDecided is the consequence of
// all that, exercised through the quoting path rather than asserted about the
// file.
//
// Each surface must refuse for the reason its own state predicts: a surface
// missing a decision refuses on that decision, and a fully decided surface
// refuses on the seed. If they all refused for one blanket reason, the
// per-decision refusals would be untested and the model could lose a decision
// with nothing to notice.
func TestGateLedgerCommittedModelQuotesNothingItHasNotDecided(t *testing.T) {
	root := surfaceRepoRoot(t)
	model, _, err := gateLedgerLoadModel(root)
	if err != nil {
		t.Fatalf("read the committed cost model: %v", err)
	}
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, p := range model.Surfaces {
		t.Run(p.Name, func(t *testing.T) {
			_, err := gateLedgerQuoteSurface(model, root, p.Name, tracked, 0)
			undecided := p.Class == gateLedgerUndecided || p.DocumentBasis == gateLedgerUndecided
			_, seeded := gateLedgerInForce(model, p.Class)
			switch {
			case undecided:
				if !errors.Is(err, errGateLedgerUndecidedSurface) {
					t.Fatalf("a surface with an unmade decision was quoted or refused for another reason: %v", err)
				}
			case !seeded:
				if err == nil {
					t.Fatal("a surface was funded from a class whose seed nobody supplied")
				}
				if !strings.Contains(err.Error(), "no decided k") {
					t.Fatalf("the refusal does not say the seed is what is missing: %v", err)
				}
			case err != nil:
				t.Fatalf("a fully decided surface could not be quoted: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------
// quoting
// ---------------------------------------------------------------------

// TestGateLedgerQuoteResolvesTheSizeThroughTheRatifiedBasis is the positive path
// the refusals above would be decoration without, and the point where the
// wrong-bytes failure would have to happen if it were going to.
//
// The size is resolved on every quote through the basis the model NAMES. It is
// never a transcribed number and there is no path to one that skips the name.
func TestGateLedgerQuoteResolvesTheSizeThroughTheRatifiedBasis(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	model, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
	if err != nil {
		t.Fatalf("decode model-legal.yaml: %v", err)
	}
	q, err := gateLedgerQuoteSurface(model, root, "readme", tracked, 0)
	if err != nil {
		t.Fatalf("quote readme: %v", err)
	}
	want, err := gateLedgerBases["manifest.tracked_files"].size(root, "readme", tracked)
	if err != nil {
		t.Fatalf("size readme: %v", err)
	}
	if q.SizeBytes != want {
		t.Errorf("the quote sized readme at %d bytes; its ratified basis resolves to %d", q.SizeBytes, want)
	}
	if q.Basis != "manifest.tracked_files" {
		t.Errorf("the quote records basis %q", q.Basis)
	}
	if q.Unit != gateLedgerEnforcedUnit {
		t.Errorf("the quote is denominated in %q, and the ceiling is enforced in %q", q.Unit, gateLedgerEnforcedUnit)
	}
	if got, expect := q.Predicted, int64(math.Round(float64(want)*1.2)); got != expect {
		t.Errorf("the quote predicts %d output tokens; %d bytes at k=1.2 is %d", got, want, expect)
	}
}

// TestGateLedgerRefusesToQuoteADecisionNobodyMade exercises each refusal against
// a model where everything ELSE is decided, so the refusal is provably about the
// one missing decision rather than about a model that funds nothing.
func TestGateLedgerRefusesToQuoteADecisionNobodyMade(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, tc := range []struct {
		fixture string
		refused string
		quoted  string
	}{
		{fixture: "model-basis-undecided.yaml", refused: "roadmap", quoted: "readme"},
		{fixture: "model-class-undecided.yaml", refused: "roadmap", quoted: "readme"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			model, err := gateLedgerDecodeModel(gateLedgerFixture(t, tc.fixture))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if _, err := gateLedgerQuoteSurface(model, root, tc.refused, tracked, 0); !errors.Is(err, errGateLedgerUndecidedSurface) {
				t.Fatalf("%q was quoted despite an unmade decision: %v", tc.refused, err)
			}
			if _, err := gateLedgerQuoteSurface(model, root, tc.quoted, tracked, 0); err != nil {
				t.Fatalf(`%q could not be quoted, so the refusal above is not about the unmade decision: %v

A model that refuses everything proves nothing about which decision was missing.`, tc.quoted, err)
			}
		})
	}
}

// TestGateLedgerRefusesToQuoteFromAnUnextractableBasis is the other half of open
// question 6: a basis the human RATIFIED and this tree cannot honour.
//
// It has to be a refusal rather than a lookup that happens to fail, and it has
// to be distinguishable from a basis nobody defined — one is a deferral, the
// other is a typo.
func TestGateLedgerRefusesToQuoteFromAnUnextractableBasis(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	model, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i := range model.Surfaces {
		if model.Surfaces[i].Name == "readme" {
			model.Surfaces[i].DocumentBasis = "binary.embedded_prose"
		}
	}
	if _, err := gateLedgerQuoteSurface(model, root, "readme", tracked, 0); !errors.Is(err, errGateLedgerBasisUnavailable) {
		t.Fatalf(`a surface was funded from a basis this tree cannot resolve: %v

surface.json carries only {path, short, flags}, so the prose compiled into the
binary cannot be reconstructed from the committed tree. A quote that came back
anyway came back from somebody else's bytes.`, err)
	}
}

// TestGateLedgerAQuoteIsNeverLabelledCalibrated is C25 held to.
//
// Nothing here computes k from anything; k is a human edit until five real runs
// exist. A quote that claimed otherwise would be claiming a number no rule
// produced, and the deferral would end without anybody deciding it had.
func TestGateLedgerAQuoteIsNeverLabelledCalibrated(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	model, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, runs := range []int{0, 1, 5, 50} {
		q, err := gateLedgerQuoteSurface(model, root, "readme", tracked, runs)
		if err != nil {
			t.Fatalf("quote with %d recorded runs: %v", runs, err)
		}
		if strings.Contains(strings.ToLower(q.Confidence), "calibrat") && !strings.Contains(q.Confidence, "C25") {
			t.Fatalf("a quote over %d runs described itself as %q", runs, q.Confidence)
		}
		if runs == 0 && q.Confidence != gateLedgerConfidenceUnvalidated {
			t.Fatalf("a quote with no recorded run behind it described itself as %q", q.Confidence)
		}
	}
}

// TestGateLedgerARunQuoteRefusesAsAWholeWhenOneSurfaceRefuses is why a ceiling
// cannot be summed over "the surfaces that happened to quote".
//
// The ceiling is what the run is held to. A ceiling that silently covers eleven
// of thirteen surfaces is the silent narrowing CLAUDE.md forbids, wearing a
// number.
func TestGateLedgerARunQuoteRefusesAsAWholeWhenOneSurfaceRefuses(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	partial, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-basis-undecided.yaml"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if total, _, err := gateLedgerQuoteRun(partial, root, tracked, 0); err == nil {
		t.Fatalf("a run was quoted at %d output tokens with one surface unfundable", total)
	}
	whole, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	total, quotes, err := gateLedgerQuoteRun(whole, root, tracked, 0)
	if err != nil {
		t.Fatalf("a fully decided model could not quote a run: %v", err)
	}
	if len(quotes) != len(whole.Surfaces) {
		t.Fatalf("the run quote covers %d of %d surfaces", len(quotes), len(whole.Surfaces))
	}
	var sum int64
	for _, q := range quotes {
		sum += q.Predicted
	}
	if want := sum + whole.Reserve.Value; total != want {
		t.Errorf("the run ceiling is %d; the surfaces plus the reserve come to %d", total, want)
	}
}

// ---------------------------------------------------------------------
// the canonical paths — the thing all of the above is actually about
// ---------------------------------------------------------------------

// TestGateLedgerTheCanonicalPathsAreClaimedByExactlyOneManifestEntry is what
// lets these two files be committed at all.
//
// surfaces.yaml claims every tracked file exactly once. Without an entry, the
// model is a red build the moment it is committed; with two, the same. And the
// entry has to be an out_of_scope one: a cost model reviewed as prose would be
// reviewing the gate's own working notes.
func TestGateLedgerTheCanonicalPathsAreClaimedByExactlyOneManifestEntry(t *testing.T) {
	root := surfaceRepoRoot(t)
	manifest, err := gateLoadManifest(root)
	if err != nil {
		t.Fatalf("read surfaces.yaml: %v", err)
	}
	for _, path := range []string{gateLedgerModelPath, gateLedgerPath, gateLedgerFixtureDir + "/model-legal.yaml"} {
		var surfaces, excluded []string
		for _, e := range manifest.Surfaces {
			if gateEntryClaims(e, path) {
				surfaces = append(surfaces, e.Name)
			}
		}
		for _, e := range manifest.OutOfScope {
			if gateEntryClaims(e, path) {
				excluded = append(excluded, e.Name)
			}
		}
		if len(surfaces)+len(excluded) != 1 {
			t.Errorf(`%s is claimed by %d surface(s) %v and %d exclusion(s) %v, and the manifest rule is EXACTLY ONE.

This is checked here rather than left to tests/surfaces_manifest_test.go because
of WHEN each runs. That test reads git ls-files, and a lane's new files are still
untracked while land-lane.sh runs the suite — so a double-claim on a file this
lane is about to commit passes at landing time and goes red on the NEXT lane's
landing, blaming a lane that did nothing.

The fix is in surfaces.yaml, which is forbidden to every lane: out_of_scope
tests-and-fixtures already claims "testdata/", which the pattern grammar expands
to every file beneath it, so the release-gate-artifacts entries for
"testdata/gate-ledger/" (and its siblings "testdata/gate-stage2/" and
"testdata/ci-run-evidence/") are redundant AND contested. Removing those three
lines leaves every fixture claimed exactly once, by tests-and-fixtures.`,
				path, len(surfaces), surfaces, len(excluded), excluded)
		}
		if len(surfaces) != 0 {
			t.Errorf("%s is declared a client-facing SURFACE (%v); the gate would fan a reading agent out over its own working notes", path, surfaces)
		}
	}
}

// TestGateLedgerGovernsWhateverSitsAtTheCanonicalPaths is the assertion the rest
// of this file would be decoration without.
//
// Everything above validates files under testdata/. A wrong model or a ledger
// with an illegal transition committed at the repository root would leave every
// one of those tests green, and the only remaining defence would be a human
// noticing a bad number in a diff — a review habit, not a check.
func TestGateLedgerGovernsWhateverSitsAtTheCanonicalPaths(t *testing.T) {
	root := surfaceRepoRoot(t)

	model, present, err := gateLedgerLoadModel(root)
	if err != nil {
		t.Fatalf(`the committed cost model at %s is not one this validator accepts:

%v

This is the whole point of pinning a canonical path.`, gateLedgerModelPath, err)
	}
	if !present {
		t.Fatalf("%s is absent. It is this lane's primary artifact and C25's 'a k a human edits' has no home without it", gateLedgerModelPath)
	}
	if len(model.Classes) == 0 || len(model.Surfaces) == 0 {
		t.Fatalf("%s declares %d classes and %d surfaces; an empty model funds nothing and says nothing", gateLedgerModelPath, len(model.Classes), len(model.Surfaces))
	}

	ledger, present, err := gateLedgerLoadLedger(root)
	if err != nil {
		t.Fatalf(`the committed cost ledger at %s is not one this validator accepts:

%v

A ledger whose recorded history breaks its own rules is a red build, not a number
somebody might spot in a diff.`, gateLedgerPath, err)
	}
	if present && len(ledger.Runs) == 0 {
		t.Fatalf("%s exists and records no run at all", gateLedgerPath)
	}
	if !present {
		// The state this wave is in, asserted rather than assumed. There is no
		// gate runtime in this tree, so there is no honest run to record, and
		// inventing one would be the misrecorded-actual defect committed on
		// purpose. Absence is legal; its consequence is that every quote says
		// no recorded run has tested the number.
		t.Logf("%s does not exist yet. Every quote reports %q.", gateLedgerPath, gateLedgerConfidenceUnvalidated)
	}
}

// TestGateLedgerValidatorActuallyReachesFilesAtTheCanonicalPaths is the
// anti-decoration check.
//
// gate_receipt_test.go recorded the failure shape this closes: a check never
// wired to anything "was not a description of the precondition but the only
// performance of it". No ledger is committed, so the loader is proved here
// against a throwaway root — both files, both ways.
func TestGateLedgerValidatorActuallyReachesFilesAtTheCanonicalPaths(t *testing.T) {
	legalModel := gateLedgerFixture(t, "model-legal.yaml")

	t.Run("a legal pair loads", func(t *testing.T) {
		root := t.TempDir()
		gateLedgerPlace(t, root, legalModel, gateLedgerFixture(t, "legal-run.json"))
		l, present, err := gateLedgerLoadLedger(root)
		if err != nil || !present {
			t.Fatalf("present=%v err=%v", present, err)
		}
		if len(l.Runs) != 1 {
			t.Fatalf("loaded %d runs", len(l.Runs))
		}
	})

	t.Run("an illegal ledger at the canonical path is refused", func(t *testing.T) {
		root := t.TempDir()
		gateLedgerPlace(t, root, legalModel, gateLedgerFixture(t, "capped-run-declared-completed.json"))
		if _, _, err := gateLedgerLoadLedger(root); !errors.Is(err, errGateLedgerCapContradicted) {
			t.Fatalf(`an illegal ledger at the canonical path was accepted: %v

If this passes only for fixtures read out of testdata/, the validator is not
governing the committed ledger — it is describing rules the committed ledger
never has to satisfy.`, err)
		}
	})

	t.Run("an illegal model at the canonical path is refused", func(t *testing.T) {
		root := t.TempDir()
		gateLedgerPlace(t, root, gateLedgerFixture(t, "model-k-past-the-bound.yaml"), nil)
		if _, _, err := gateLedgerLoadModel(root); !errors.Is(err, errGateLedgerRaisedTooFast) {
			t.Fatalf("an illegal model at the canonical path was accepted: %v", err)
		}
	})

	t.Run("a ledger with no model beside it cannot be checked", func(t *testing.T) {
		root := t.TempDir()
		gateLedgerPlace(t, root, nil, gateLedgerFixture(t, "legal-run.json"))
		if _, _, err := gateLedgerLoadLedger(root); !errors.Is(err, errGateUncheckable) {
			t.Fatalf(`a ledger was accepted with nothing saying what its classes, bases or predictions were supposed to be: %v`, err)
		}
	})
}

// TestGateLedgerAbsenceAndUnreadabilityAreDifferentAnswers is invariant 7's last
// clause.
//
// "There is no file" is a legal state whose consequence is that no run is funded
// from a number nobody chose. "There is a file and it cannot be read, parsed, or
// does not declare its schema" is errGateUncheckable, which CLAUDE.md makes a
// FAILED gate rather than an empty window. Collapsing the two is how a corrupt
// model reads as a fresh repository.
func TestGateLedgerAbsenceAndUnreadabilityAreDifferentAnswers(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		root := t.TempDir()
		if _, present, err := gateLedgerLoadModel(root); present || err != nil {
			t.Fatalf("an absent model reported present=%v err=%v", present, err)
		}
		if _, present, err := gateLedgerLoadLedger(root); present || err != nil {
			t.Fatalf("an absent ledger reported present=%v err=%v", present, err)
		}
	})

	for _, tc := range []struct {
		name  string
		place func(t *testing.T, root string)
	}{
		{"a directory where the model should be", func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, gateLedgerModelPath), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"a model that is not YAML", func(t *testing.T, root string) {
			gateWrite(t, root, gateLedgerModelPath, "\tthis: [is not\n")
		}},
		{"a model that declares no schema", func(t *testing.T, root string) {
			gateWrite(t, root, gateLedgerModelPath, "classes: []\n")
		}},
		{"a ledger that is not JSON", func(t *testing.T, root string) {
			gateWrite(t, root, gateLedgerModelPath, string(gateLedgerFixture(t, "model-legal.yaml")))
			gateWrite(t, root, gateLedgerPath, "{not json")
		}},
		{"a ledger that declares no schema", func(t *testing.T, root string) {
			gateWrite(t, root, gateLedgerModelPath, string(gateLedgerFixture(t, "model-legal.yaml")))
			gateWrite(t, root, gateLedgerPath, `{"runs":[]}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.place(t, root)
			_, mPresent, mErr := gateLedgerLoadModel(root)
			_, lPresent, lErr := gateLedgerLoadLedger(root)
			err := mErr
			present := mPresent
			if strings.Contains(tc.name, "ledger") {
				err, present = lErr, lPresent
			}
			if !present {
				t.Fatal("a file that is there was reported absent, which is the one answer that must never be given for an unreadable one")
			}
			if !errors.Is(err, errGateUncheckable) {
				t.Fatalf("an unreadable file produced %v rather than errGateUncheckable", err)
			}
		})
	}
}

// ---------------------------------------------------------------------
// the remaining refusals, each exercised against an otherwise legal file
// ---------------------------------------------------------------------

// TestGateLedgerRefusesEveryOtherWayARecordingCannotBeTrue mutates ONE thing in
// a legal ledger per row.
//
// Every row starts from a file that validates, so a row that goes red is red
// about the thing the row changed and nothing else. The rows are here rather
// than as committed fixtures because they are variations on a shape the fixtures
// already establish; the fixtures carry the cases a human needs to read.
func TestGateLedgerRefusesEveryOtherWayARecordingCannotBeTrue(t *testing.T) {
	for _, tc := range []struct {
		name    string
		model   string
		fixture string
		mutate  func(*gateLedgerFile)
		wantErr error
	}{
		{
			name: "a run with no id", wantErr: errGateLedgerRunUnidentified,
			mutate: func(l *gateLedgerFile) { l.Runs[0].RunID = "" },
		},
		{
			name: "two runs sharing one id", wantErr: errGateLedgerRunTwice,
			mutate: func(l *gateLedgerFile) { l.Runs = append(l.Runs, l.Runs[0]) },
		},
		{
			name: "an outcome that is neither of the two there are", wantErr: errGateLedgerUnknownOutcome,
			mutate: func(l *gateLedgerFile) { l.Runs[0].Outcome = "partial" },
		},
		{
			name: "one surface's cost recorded twice", wantErr: errGateLedgerSurfaceTwice,
			mutate: func(l *gateLedgerFile) { l.Runs[0].Surfaces = append(l.Runs[0].Surfaces, l.Runs[0].Surfaces[0]) },
		},
		{
			name: "a surface carrying both a measurement and a bound", wantErr: errGateLedgerTwoCostFigures,
			mutate: func(l *gateLedgerFile) {
				s := &l.Runs[0].Surfaces[0]
				floor := *s.Actual
				s.Floor = &floor
			},
		},
		{
			name: "a size denominated in tokens", wantErr: errGateLedgerSizeUnitWrong,
			mutate: func(l *gateLedgerFile) { l.Runs[0].Surfaces[0].Size.Unit = gateLedgerUnitOutput },
		},
		{
			name: "a surface filed under a class the model does not assign it", wantErr: errGateLedgerClassNotAsPlaned,
			mutate: func(l *gateLedgerFile) { l.Runs[0].Surfaces[0].Class = "newest-entry-only" },
		},
		{
			name: "k declared twice for one class", wantErr: errGateLedgerClassTwice,
			mutate: func(l *gateLedgerFile) { l.Runs[0].K = append(l.Runs[0].K, l.Runs[0].K[0]) },
		},
		{
			name: "k for a class the model does not define", wantErr: errGateLedgerUnknownClass,
			mutate: func(l *gateLedgerFile) { l.Runs[0].K[0].Class = "invented-class" },
		},
		{
			name: "a spend of zero", wantErr: errGateLedgerNonPositive,
			mutate: func(l *gateLedgerFile) { l.Runs[0].Spend.Value = 0 },
		},
		{
			// Asserted on the sentinel rather than on "some error came back":
			// a k of zero also trips the travel bound as a fall of -100%, so a
			// row that only checked err != nil would pass with the check on
			// the ratio itself deleted.
			name: "a recorded k of zero", wantErr: errGateLedgerNonPositive,
			mutate: func(l *gateLedgerFile) { l.Runs[0].K[0].Value = 0 },
		},
		{
			name: "a ledger that declares no schema", wantErr: errGateLedgerUnknownSchema,
			mutate: func(l *gateLedgerFile) { l.Schema = "" },
		},
		{
			name: "a surface funded while its basis is undecided", wantErr: errGateLedgerUndecidedSurface,
			model: "model-basis-undecided.yaml",
		},
		{
			name: "a surface funded while its class is undecided", wantErr: errGateLedgerUndecidedSurface,
			model: "model-class-undecided.yaml",
		},
		{
			name: "a run recording k for a class whose seed nobody decided", wantErr: errGateLedgerSeedUndecided,
			model: "model-seed-undecided.yaml",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modelFile := tc.model
			if modelFile == "" {
				modelFile = "model-legal.yaml"
			}
			m, err := gateLedgerDecodeModel(gateLedgerFixture(t, modelFile))
			if err != nil {
				t.Fatalf("decode %s: %v", modelFile, err)
			}
			fixture := tc.fixture
			if fixture == "" {
				fixture = "legal-run.json"
			}
			l, _ := gateLedgerFixtureRun(t, fixture)
			if tc.mutate != nil {
				tc.mutate(&l)
			}
			if err := gateLedgerValidateLedger(m, l); !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v\n got %v", tc.wantErr, err)
			}
		})
	}
}

// TestGateLedgerRefusesEveryOtherWayAModelCannotBeTrue is the same for the
// hand-authored half.
func TestGateLedgerRefusesEveryOtherWayAModelCannotBeTrue(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*gateLedgerModel)
		wantErr error
	}{
		{
			name: "one surface planned twice", wantErr: errGateLedgerPlanTwice,
			mutate: func(m *gateLedgerModel) { m.Surfaces = append(m.Surfaces, m.Surfaces[0]) },
		},
		{
			name: "one class defined twice", wantErr: errGateLedgerClassDefTwice,
			mutate: func(m *gateLedgerModel) { m.Classes = append(m.Classes, m.Classes[0]) },
		},
		{
			name: "a surface naming a class the model does not define", wantErr: errGateLedgerUnknownClass,
			mutate: func(m *gateLedgerModel) { m.Surfaces[0].Class = "invented-class" },
		},
		{
			name: "a model that declares no schema", wantErr: errGateLedgerUnknownSchema,
			mutate: func(m *gateLedgerModel) { m.Schema = "" },
		},
		{
			name: "a reserve in the wrong unit", wantErr: errGateLedgerUnitWrong,
			mutate: func(m *gateLedgerModel) { m.Reserve.Unit = gateLedgerUnitInput },
		},
		{
			name: "a reserve of zero", wantErr: errGateLedgerNonPositive,
			mutate: func(m *gateLedgerModel) { m.Reserve.Value = 0 },
		},
		{
			// A seed of zero funds nothing at all, which fails a gate for a
			// reason nobody can see: in Go a missing map key returns the zero
			// value without complaining, so this is the shape an unassigned
			// class takes when something defaults it.
			name: "a seed of zero", wantErr: errGateLedgerNonPositive,
			mutate: func(m *gateLedgerModel) { m.Classes[0].Seed.Value = 0 },
		},
		{
			// The travel bound would call this -100% and refuse it too, but on
			// a claim about the size of the move rather than about the number
			// being a ratio at all.
			name: "an edit that moves k to zero", wantErr: errGateLedgerNonPositive,
			mutate: func(m *gateLedgerModel) {
				m.Classes[0].K = []gateLedgerModelEdit{{
					Value: 0, NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes,
					Previous: m.Classes[0].Seed.Value, Rationale: "nothing costs anything",
				}}
			},
		},
		{
			name: "an edit to k for a class whose seed is undecided", wantErr: errGateLedgerSeedUndecided,
			mutate: func(m *gateLedgerModel) {
				m.Classes[0].Seed = gateLedgerModelSeed{Undecided: true, Reason: "not supplied"}
				m.Classes[0].K = []gateLedgerModelEdit{{Value: 1.3, NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes, Previous: 1.2, Rationale: "why"}}
			},
		},
		{
			// The chain is a history, so an edit in the MIDDLE of it is checked
			// against the move before it and not against the seed. Without this
			// the second and every later move is unanchored: only the first
			// would have anything to be measured from.
			name: "a second move that invents its own predecessor", wantErr: errGateLedgerEditPrior,
			mutate: func(m *gateLedgerModel) {
				seed := m.Classes[0].Seed.Value
				m.Classes[0].K = []gateLedgerModelEdit{
					{Value: seed * 1.2, NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes,
						Previous: seed, Rationale: "the first wave came in over quote"},
					// Claiming the seed as its predecessor keeps this move inside
					// the travel bounds AS CLAIMED, so the only thing that can
					// refuse it is the chain.
					{Value: seed * 1.15, NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes,
						Previous: seed, Rationale: "and again"},
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			tc.mutate(&m)
			if err := gateLedgerValidateModel(m); !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v\n got %v", tc.wantErr, err)
			}
		})
	}
}

// TestGateLedgerRefusesADecisionThatIsBothMadeAndUnmade holds the
// decided-or-explicitly-undecided shape, in both directions.
//
// A zero that means "unset" is the silence this refuses: a reserve of 0 with no
// unit reads as a decision to spend nothing, and an undecided marker carrying a
// value reads as whichever half the reader looked at first.
func TestGateLedgerRefusesADecisionThatIsBothMadeAndUnmade(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*gateLedgerModel)
		wantOK  bool
		mustSay string
	}{
		{name: "undecided with its reason", wantOK: true, mutate: func(m *gateLedgerModel) {
			m.Reserve = gateLedgerModelQuantity{Undecided: true, Reason: "no ceiling exists yet"}
		}},
		{name: "undecided with no reason", mustSay: "undecided with no reason", mutate: func(m *gateLedgerModel) {
			m.Reserve = gateLedgerModelQuantity{Undecided: true}
		}},
		{name: "undecided and carrying a value", mustSay: "undecided AND carrying a value", mutate: func(m *gateLedgerModel) {
			m.Reserve = gateLedgerModelQuantity{Undecided: true, Reason: "r", Value: 10, Unit: gateLedgerUnitOutput}
		}},
		{name: "decided and carrying an undecided reason", mustSay: "decided quantity carries an undecided reason", mutate: func(m *gateLedgerModel) {
			m.Reserve = gateLedgerModelQuantity{Value: 10, Unit: gateLedgerUnitOutput, Reason: "leftover"}
		}},
		{name: "a backstop at the ceiling is not a backstop", mustSay: "not a backstop", mutate: func(m *gateLedgerModel) {
			m.BackstopFactor = gateLedgerModelFactor{Value: 1}
		}},
		{name: "a seed with no measurement behind it", mustSay: "no derived_from", mutate: func(m *gateLedgerModel) {
			m.Classes[0].Seed.DerivedFrom = ""
		}},
		{name: "a seed that is undecided and carries a value", mustSay: "seed: undecided AND carrying a value", mutate: func(m *gateLedgerModel) {
			m.Classes[0].Seed = gateLedgerModelSeed{
				Undecided: true, Reason: "not supplied", Value: 1.2,
				NumeratorUnit: gateLedgerUnitOutput, DenominatorUnit: gateLedgerUnitBytes,
			}
		}},
		{
			// "undecided" is the reserved name for a decision nobody made. A
			// class wearing it would make a surface's "no decision here" read
			// as a decision, and the two are the one pair that must never be
			// confusable.
			name: "a class named for the absence of a decision", mustSay: "reserved name", mutate: func(m *gateLedgerModel) {
				m.Classes[0].Name = gateLedgerUndecided
				m.Surfaces[0].Class = gateLedgerUndecided
			},
		},
		{
			// Invariant 5's no-third-state rule. A surface with no class field
			// at all is not "unassigned" — it is a surface whose k would be
			// whatever a lookup returned, which in Go is zero and funds
			// nothing.
			name: "a surface with no class field at all", mustSay: "no class and no", mutate: func(m *gateLedgerModel) {
				m.Surfaces[0].Class = ""
			},
		},
		{
			name: "a surface with no document_basis field at all", mustSay: "no document_basis and no", mutate: func(m *gateLedgerModel) {
				m.Surfaces[0].DocumentBasis = ""
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			tc.mutate(&m)
			err = gateLedgerValidateModel(m)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("a legal shape was refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.mustSay) {
				t.Fatalf("want a refusal saying %q, got %v", tc.mustSay, err)
			}
		})
	}
}

// TestGateLedgerPlanCoverageIsCheckedAgainstTheManifestBothWays is invariant 5
// as a function, exercised in both directions against a synthetic fan-out.
func TestGateLedgerPlanCoverageIsCheckedAgainstTheManifestBothWays(t *testing.T) {
	m, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := gateLedgerPlanCoversDeclaredSurfaces(m, []string{"changelog", "readme", "roadmap"}); err != nil {
		t.Fatalf("a total plan was refused: %v", err)
	}
	if err := gateLedgerPlanCoversDeclaredSurfaces(m, []string{"changelog", "readme", "roadmap", "newcomer"}); !errors.Is(err, errGateLedgerPlanNotTotal) {
		t.Fatalf(`a surface added to the manifest with no decision behind it was quoted rather than refused: %v

The fan-out is read from the manifest. A fourteenth surface must stop the gate
the moment it appears, not the moment somebody remembers to look.`, err)
	}
	if err := gateLedgerPlanCoversDeclaredSurfaces(m, []string{"changelog", "readme"}); !errors.Is(err, errGateLedgerPlanUndeclared) {
		t.Fatalf("the model plans for a surface the manifest does not declare and nothing objected: %v", err)
	}
	// Asserted on the sentinel rather than on "some error came back". An empty
	// fan-out ALSO trips the undeclared arm three times over, so a test that
	// only checked err != nil would pass with this guard deleted — which is the
	// vacuous shape this file is written against, found here by mutation.
	if err := gateLedgerPlanCoversDeclaredSurfaces(m, nil); !errors.Is(err, errGateLedgerNoDeclaredSurfaces) {
		t.Fatalf("coverage over zero declared surfaces reported %v; a plan that is total over nothing is a pass over zero assertions", err)
	}
}

// TestGateLedgerAnAbsentModelRefusesAQuoteAndAnAbsentLedgerDoesNot is invariant
// 7's two absences, told apart.
//
// They are different states with different consequences and this is the only
// place both are exercised. No model means no run is funded from a number
// anybody chose — a refusal. No ledger means the numbers exist and nothing has
// tested them — a quote, labelled.
func TestGateLedgerAnAbsentModelRefusesAQuoteAndAnAbsentLedgerDoesNot(t *testing.T) {
	t.Run("no model", func(t *testing.T) {
		root := t.TempDir()
		if _, _, err := gateLedgerQuoteRunFromRoot(root); !errors.Is(err, errGateLedgerNoModel) {
			t.Fatalf("a run was quoted with no committed model: %v", err)
		}
	})

	t.Run("a model and no ledger", func(t *testing.T) {
		root := gateLedgerQuotableRepo(t)
		total, quotes, err := gateLedgerQuoteRunFromRoot(root)
		if err != nil {
			t.Fatalf("quote with a model and no ledger: %v", err)
		}
		if total <= 0 || len(quotes) == 0 {
			t.Fatalf("quoted %d output tokens over %d surfaces", total, len(quotes))
		}
		for _, q := range quotes {
			if q.Confidence != gateLedgerConfidenceUnvalidated {
				t.Errorf(`surface %q was quoted as %q with no ledger behind it.

An absent ledger is a legal state. Its consequence is that the quote says no
recorded run has tested the number — not that the number is trusted.`, q.Surface, q.Confidence)
			}
		}
	})
}

// ---------------------------------------------------------------------
// the k a run was quoted at, against the k the model holds in force
// ---------------------------------------------------------------------

// gateLedgerRestateK rewrites a run's recorded k for one class and re-derives
// every number that follows from it — the class's predictions and the run's
// ceiling — so that a test about k is not accidentally also a test about
// arithmetic somebody forgot to update.
func gateLedgerRestateK(l *gateLedgerFile, class string, value float64) {
	for i := range l.Runs {
		run := &l.Runs[i]
		for j := range run.K {
			if run.K[j].Class == class {
				run.K[j].Value = value
			}
		}
		var predicted int64
		for s := range run.Surfaces {
			rec := &run.Surfaces[s]
			if rec.Class == class {
				rec.Predicted.Value = int64(math.Round(float64(rec.Size.Value) * value))
			}
			predicted += rec.Predicted.Value
		}
		run.Ceiling.Value = predicted + run.Reserve.Value
	}
}

// gateLedgerWithEdits returns the fixture model with one class's k moved by a
// chain of human edits, so a test can ask what the model holds IN FORCE, and
// what it has held along the way, rather than only what it started at.
//
// It validates the result, because a model that is not itself legal refuses
// everything for its own reasons and anything a test then reads out of it proves
// nothing.
func gateLedgerWithEdits(t *testing.T, class string, edits ...gateLedgerModelEdit) gateLedgerModel {
	t.Helper()
	return gateLedgerReseeded(t, class, 0, edits...)
}

// gateLedgerReseeded is the same, with the class's seed moved to a stated value
// first. A seed of 0 keeps the fixture's own.
//
// The seed is a parameter because a test about the anchor between two ENTRIES
// has to be able to make the seed a DIFFERENT number from the k the first entry
// records. When the two coincide — which they do in the fixture — no test can
// tell an anchor carried forward from one entry to the next apart from an anchor
// pinned to the seed forever.
func gateLedgerReseeded(t *testing.T, class string, seed float64, edits ...gateLedgerModelEdit) gateLedgerModel {
	t.Helper()
	m, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
	if err != nil {
		t.Fatalf("decode model-legal.yaml: %v", err)
	}
	if len(edits) == 0 && seed == 0 {
		return m
	}
	found := false
	for i := range m.Classes {
		if m.Classes[i].Name == class {
			if seed > 0 {
				m.Classes[i].Seed.Value = seed
			}
			m.Classes[i].K = edits
			found = true
		}
	}
	if !found {
		t.Fatalf("model-legal.yaml defines no class %q", class)
	}
	if err := gateLedgerValidateModel(m); err != nil {
		t.Fatalf("the edited model is not itself legal, so anything it refuses below proves nothing: %v", err)
	}
	return m
}

// gateLedgerMove is one legal human move of k, for the tests that build a chain.
func gateLedgerMove(value, previous float64, why string) gateLedgerModelEdit {
	return gateLedgerModelEdit{
		Value:           value,
		NumeratorUnit:   gateLedgerUnitOutput,
		DenominatorUnit: gateLedgerUnitBytes,
		Previous:        previous,
		Rationale:       why,
	}
}

// TestGateLedgerARunIsOnlyEverQuotedAtANumberTheModelStates is the tie between
// the two files that every other rule assumed and nothing checked.
//
// Invariant 6: wherever k is read from when a run is quoted, the value in force,
// the value it replaced and why it moved are visible in the same diff. A quote
// is funded through gateLedgerInForce, so the value in force IS the value a run
// was quoted at — and an entry naming any other number is naming a k nobody was
// quoted at.
//
// The travel bound is no defence here, and that is the whole finding. It is
// measured from the seed, so with a seed of 1.2 and a legal edit standing at 1.5
// every value in [1.08, 1.5] passes it. The entry's own arithmetic is no defence
// either: predictions and ceiling are internally consistent at whatever k the
// entry names. The human C25 is waiting for — reading five entries to decide
// what k should be — would be reading a number no quote ever used.
//
// Both timelines are admitted, because neither file says whether the committed
// edit landed before or after the newest run, and refusing one of them would
// make C25's own loop (record runs, then edit k) impossible.
func TestGateLedgerARunIsOnlyEverQuotedAtANumberTheModelStates(t *testing.T) {
	const seed = 1.2
	edit := func(value, previous float64) []gateLedgerModelEdit {
		return []gateLedgerModelEdit{gateLedgerMove(value, previous, "the first wave came in over quote")}
	}
	for _, tc := range []struct {
		name     string
		edits    []gateLedgerModelEdit
		recorded float64
		wantErr  error
	}{
		{
			name:     "no edit, and the run was quoted at the seed",
			recorded: seed,
		},
		{
			name:     "the edit landed after the run, and says what it replaced",
			edits:    edit(1.5, seed),
			recorded: seed,
		},
		{
			name:     "the edit landed before the run, so the run carries the value in force",
			edits:    edit(1.5, seed),
			recorded: 1.5,
		},
		{
			name:     "the bottom of the band the travel bound alone leaves open",
			edits:    edit(1.5, seed),
			recorded: 1.08,
			wantErr:  errGateLedgerKNotInForce,
		},
		{
			name:     "the top of that band",
			edits:    edit(1.5, seed),
			recorded: 1.44,
			wantErr:  errGateLedgerKNotInForce,
		},
		{
			name:     "no edit at all, and the run was quoted somewhere else",
			recorded: 1.32,
			wantErr:  errGateLedgerKNotInForce,
		},
		{
			// The chain has three values in it and the run names a fourth,
			// between two of them. Nothing about "the newest entry matches
			// something the model states" reaches this, and the value is inside
			// the travel bound measured from the seed, so nothing else does
			// either.
			name:     "a value between two the model has actually held",
			edits:    []gateLedgerModelEdit{gateLedgerMove(1.32, seed, "over quote"), gateLedgerMove(1.45, 1.32, "over quote again")},
			recorded: 1.38,
			wantErr:  errGateLedgerKNotInForce,
		},
		{
			// And the third value in a three-value chain is legal, which is the
			// half of invariant 6 a single-edit model could not express at all:
			// k moved twice, both moves are recorded, and a run quoted at the
			// second one validates.
			name:     "the second of two recorded moves",
			edits:    []gateLedgerModelEdit{gateLedgerMove(1.32, seed, "over quote"), gateLedgerMove(1.45, 1.32, "over quote again")},
			recorded: 1.45,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := gateLedgerWithEdits(t, "falsification-dense", tc.edits...)
			l, _ := gateLedgerFixtureRun(t, "legal-run.json")
			gateLedgerRestateK(&l, "falsification-dense", tc.recorded)

			err := gateLedgerValidateLedger(m, l)
			inForce, _ := gateLedgerInForce(m, "falsification-dense")
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("a run recording k=%v against a model holding %v in force was refused: %v", tc.recorded, inForce, err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf(`a run recording k=%v was accepted against a model that holds %v in force and never held %v: %v

Every travel bound is satisfied — they are measured from the seed %v, and this
value is inside them. Every prediction and the ceiling are consistent at %v,
because the entry did its own arithmetic. What is not satisfied is that the
number was ever in force, and that is the only thing that decides what the run
was actually funded at.`, tc.recorded, inForce, tc.recorded, err, seed, tc.recorded)
			}
		})
	}
}

// gateLedgerRunsAtK builds a ledger of the legal single-run fixture repeated
// once per k, with every number that follows from each k re-derived.
//
// The point of it is corpora rather than single entries: C25's deferred
// calibration is to be read off five recorded runs, so the rules have to hold
// over all five and not only over whichever one happens to be last.
func gateLedgerRunsAtK(t *testing.T, class string, ks ...float64) gateLedgerFile {
	t.Helper()
	var l gateLedgerFile
	for i, k := range ks {
		one, _ := gateLedgerFixtureRun(t, "legal-run.json")
		gateLedgerRestateK(&one, class, k)
		one.Runs[0].RunID = fmt.Sprintf("2026-08-%02dT09-00Z", 8+i)
		l.Schema = one.Schema
		l.Runs = append(l.Runs, one.Runs[0])
	}
	return l
}

// TestGateLedgerEveryEntryIsHeldToTheModelAndNotOnlyTheNewest is the corpus
// version of the rule above, and it is a separate test because checking only the
// newest entry passes every case in that one.
//
// THE FAILURE IT CLOSES: a model whose seed is 1.2 with no edit at all, and
// three runs recording 1.2, 1.32, 1.2. Every step is inside the +25%/-10% travel
// bounds. The newest recorded value equals the seed, so a check that compares
// only the newest entry to the model is satisfied. Each entry's predictions and
// ceiling are internally consistent at the k it names. And the middle run stands
// quoted at 1.32 — a number the model has never held and no diff has ever
// recorded.
//
// It is not one bad entry out of three. A five-run corpus can walk out and back
// the same way, so FOUR of five entries can carry a k up to 25% from anything a
// human chose while the ledger validates — and those five entries are precisely
// what C25 defers calibration until, read by a human deciding what k should be.
func TestGateLedgerEveryEntryIsHeldToTheModelAndNotOnlyTheNewest(t *testing.T) {
	const seed = 1.2
	for _, tc := range []struct {
		name    string
		edits   []gateLedgerModelEdit
		runs    []float64
		wantErr error
	}{
		{
			name: "the walk out and back, with the model stating only the seed",
			runs: []float64{seed, 1.32, seed},
			// 1.32 is +10% from the seed and 1.2 is -9.1% back: both inside the
			// bounds, and the last entry lands exactly on the value in force.
			wantErr: errGateLedgerKNotInForce,
		},
		{
			name:    "and it is the middle of five, where four entries can wander",
			runs:    []float64{seed, 1.32, 1.45, 1.32, seed},
			wantErr: errGateLedgerKNotInForce,
		},
		{
			name:    "the corpus every entry of which the model accounts for",
			edits:   []gateLedgerModelEdit{gateLedgerMove(1.32, seed, "the first wave came in over quote")},
			runs:    []float64{seed, seed, 1.32, 1.32},
			wantErr: nil,
		},
		{
			// The model records the move up and no move back down. An entry
			// after the one quoted at 1.32 cannot be at 1.2 again: the value is
			// in the chain, but only before a value an earlier entry already
			// reached, so k went back with no record of the move back.
			name:    "k walking back to a value the model states, with no move back recorded",
			edits:   []gateLedgerModelEdit{gateLedgerMove(1.32, seed, "the first wave came in over quote")},
			runs:    []float64{seed, 1.32, seed},
			wantErr: errGateLedgerKOutOfOrder,
		},
		{
			// And the same walk is legal once both moves are written down,
			// which is the thing a model holding one edit cannot say.
			name: "the same walk, with both moves recorded",
			edits: []gateLedgerModelEdit{
				gateLedgerMove(1.32, seed, "the first wave came in over quote"),
				gateLedgerMove(seed, 1.32, "and it was a one-off, so put it back"),
			},
			runs:    []float64{seed, 1.32, seed},
			wantErr: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := gateLedgerWithEdits(t, "falsification-dense", tc.edits...)
			l := gateLedgerRunsAtK(t, "falsification-dense", tc.runs...)
			err := gateLedgerValidateLedger(m, l)
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("a corpus recording k=%v, every value of which the model states in that order, was refused: %v", tc.runs, err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf(`a corpus recording k=%v was accepted against a model that states %v: %v

Every step is inside the travel bounds, because they measure each entry against
the entry before it and against nothing the model says. The newest entry is a
value the model does hold, so a check over the newest entry alone is satisfied.
Each entry's own predictions and ceiling are consistent at the k it names.
The entries in between are quoted at numbers no human ever chose, and those
entries are the corpus C25 defers calibration until.`, tc.runs, tc.edits, err)
			}
		})
	}
}

// TestGateLedgerTheSecondMoveOfKIsAlsoRecordable is invariant 6's other half:
// EVERY move of k leaves a record, not the first one.
//
// A model with room for one edit can record the first move and no other. The
// second move's `previous` is the first move's value, so anchoring every edit to
// the seed rejects the truthful record of it — and the only way left to write the
// second move down is to overwrite the first, which makes the diff that is
// supposed to be the record of a move the diff that erases the record of the one
// before it. That is not a smaller version of invariant 6; it is the opposite of
// it.
func TestGateLedgerTheSecondMoveOfKIsAlsoRecordable(t *testing.T) {
	const seed = 1.2
	first := gateLedgerMove(1.5, seed, "the first wave came in over quote")
	second := gateLedgerMove(1.8, 1.5, "and the second came in over the new quote too")

	m := gateLedgerWithEdits(t, "falsification-dense", first, second)
	if err := gateLedgerValidateModel(m); err != nil {
		t.Fatalf(`a truthful second move of k was refused: %v

The chain is %v -> %v -> %v. Each move is inside the raise bound against the move
before it and each carries its reason. If this cannot be recorded, the second
time a human edits k the only legal file is one that has forgotten the first.`,
			err, seed, first.Value, second.Value)
	}
	if got, decided := gateLedgerInForce(m, "falsification-dense"); !decided || got != second.Value {
		t.Fatalf("the value in force after two moves is %v (decided=%v), not the newest move %v", got, decided, second.Value)
	}
	chain, decided := gateLedgerStatedK(m.Classes[0])
	if !decided || len(chain) != 3 {
		t.Fatalf("the model states %v for a class with a seed and two moves; every value k has held should be there", chain)
	}

	// And the chain is a history, not a set: the second move is measured against
	// the first and not against the seed. 1.2 -> 1.5 -> 2.0 is +25% then +33%,
	// so the second move needs the reason the bound demands even though 2.0 is
	// well past the bound measured from the seed either way.
	overreach := gateLedgerMove(2.0, 1.5, "no reason that mentions the bound")
	bad := m
	bad.Classes = append([]gateLedgerModelClass(nil), m.Classes...)
	bad.Classes[0].K = []gateLedgerModelEdit{first, overreach}
	if err := gateLedgerValidateModel(bad); !errors.Is(err, errGateLedgerRaisedTooFast) {
		t.Fatalf(`a second move of +33%% past the bound was not refused: %v

The bounds bind every move, including the ones after the first. A chain that only
checks its head is a chain whose second link can go anywhere.`, err)
	}
}

// TestGateLedgerAQuoteIsFundedAtTheValueInForceAndNotAtTheSupersededSeed pins the
// arm of gateLedgerInForce that reads the human's edit.
//
// C25's whole content is "a k a human edits". If a quote read the seed instead,
// every edit would be decorative: the model would show 1.5, the reviewer would
// read 1.5, and the run would be funded at 1.2. Nothing else in this file quotes
// from a model carrying an edit, so nothing else would notice.
func TestGateLedgerAQuoteIsFundedAtTheValueInForceAndNotAtTheSupersededSeed(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked, err := gateLedgerTrackedFiles(root)
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	m := gateLedgerWithEdits(t, "falsification-dense", gateLedgerMove(1.5, 1.2, "the first wave came in over quote"))
	if got, decided := gateLedgerInForce(m, "falsification-dense"); !decided || got != 1.5 {
		t.Fatalf("the model holds %v (decided=%v) in force for a class whose seed is 1.2 and whose edit is 1.5", got, decided)
	}
	q, err := gateLedgerQuoteSurface(m, root, "readme", tracked, 0)
	if err != nil {
		t.Fatalf("quote readme: %v", err)
	}
	inForce := int64(math.Round(float64(q.SizeBytes) * 1.5))
	fromSeed := int64(math.Round(float64(q.SizeBytes) * 1.2))
	if q.Predicted != inForce {
		t.Fatalf(`readme was quoted at %d output tokens; %d bytes at the k in force (1.5) is %d.

The seed's answer is %d. A quote that reads the seed rather than the edit funds
every run at a number the human already replaced, and the diff that replaced it
says so in plain sight while changing nothing.`, q.Predicted, q.SizeBytes, inForce, fromSeed)
	}
}

// ---------------------------------------------------------------------
// what the two files may not say, at the level of the decoder
// ---------------------------------------------------------------------

// TestGateLedgerTheModelFileHasNowhereToSayWhatANameMeans is invariant 1c given
// teeth, and it is the model's missing twin of the ledger's strictness check.
//
// The counter and basis registries are code because a unit typed in a
// hand-edited file and checked against a correspondence typed in the same
// hand-edited file is a circle: one YAML line declaring that the harness's
// per-subagent counter measures output tokens reproduces the 2.9x error in full,
// with every other rule green. The model file may reference names ONLY.
//
// Refusing unknown fields is what makes that a rule rather than a convention. If
// the decoder were lenient the block would be IGNORED rather than obeyed — so no
// wrong unit would be honoured — but a human who wrote it would get no signal
// that their declaration did nothing, and the model file would silently stop
// being schema-checked at the same moment.
func TestGateLedgerTheModelFileHasNowhereToSayWhatANameMeans(t *testing.T) {
	legal := string(gateLedgerFixture(t, "model-legal.yaml"))
	for _, block := range []string{
		"counters:\n  harness.run_total_tokens:\n    unit: output_tokens\n    scope: run\n",
		"bases:\n  binary.embedded_prose:\n    resolves: manifest.tracked_files\n",
		"k_class:\n  falsification-dense: 3.5\n",
	} {
		if _, err := gateLedgerDecodeModel([]byte(legal + block)); err == nil {
			t.Fatalf(`a cost model carrying this block was decoded without complaint:

%s
What a counter measures and which bytes a basis resolves to are facts with a
measurement behind them, and they live in gateLedgerCounters and
gateLedgerBases. A model file that can state them is the circle this design
exists to break.`, block)
		}
	}
}

// TestGateLedgerRefusesContentAfterTheLedgerObject pins the trailing-content
// check.
//
// A second JSON object appended to the file — by a bad append, a merge, or a
// driver writing rather than rewriting — would be read as nothing at all: the
// decoder stops at the first value and every rule below would be applied to a
// ledger the file does not entirely contain.
func TestGateLedgerRefusesContentAfterTheLedgerObject(t *testing.T) {
	legal := string(gateLedgerFixture(t, "legal-run.json"))
	if _, err := gateLedgerDecodeLedger([]byte(legal + "\n{\"schema\":\"dossierx.gate-cost-ledger/1\",\"runs\":[]}\n")); err == nil {
		t.Fatal(`a ledger with a second object appended after it was read as though the second object were not there.

Whatever wrote it believes it recorded a run. Nothing here would ever check that run.`)
	}
}

// TestGateLedgerRefusesARepositoryWithNoTrackedFiles pins the refusal that keeps
// every size from resolving to zero.
//
// gateLedgerTrackedFiles feeds every document basis. An empty answer is not an
// empty repository as far as a cost model is concerned — it makes every surface
// size to zero bytes, every prediction to zero, and every quote come out free,
// with no rule anywhere broken.
func TestGateLedgerRefusesARepositoryWithNoTrackedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is not on PATH; this check cannot be executed, which is a failure and not a skip: %v", err)
	}
	root := t.TempDir()
	gateTestGit(t, root, "init", "-q", "-b", "main")
	files, err := gateLedgerTrackedFiles(root)
	if err == nil {
		t.Fatalf(`a repository tracking nothing answered with %d files and no error.

Every surface would size to zero bytes and every quote would come out free.`, len(files))
	}
}

// TestGateLedgerTheStampShapeIsTheOneGateMethodActuallyEmits ties the regex an
// entry's fingerprint is checked against to the digest that produces it.
//
// The two are written in different files and nothing held them together. If
// gateMethod.version's output shape ever moved, every real entry would be
// refused as unstamped and the failure would arrive in a diff that touched
// neither the ledger nor its tests.
func TestGateLedgerTheStampShapeIsTheOneGateMethodActuallyEmits(t *testing.T) {
	root := t.TempDir()
	gateWrite(t, root, "prompts/surface.md", "read the document and report what is false\n")
	stamp, err := gateMethod{
		Prompts: []string{"prompts/surface.md"},
		Model:   "a-model-id",
		Tools:   []string{"read", "grep"},
	}.version(root)
	if err != nil {
		t.Fatalf("method_version: %v", err)
	}
	if !gateLedgerFingerprintRE.MatchString(stamp) {
		t.Fatalf(`gateMethod.version emitted %q and a ledger entry's stamp is checked against %v.

Every honest entry would be refused as unstamped, and the diff that caused it
would have touched neither this file nor the ledger.`, stamp, gateLedgerFingerprintRE)
	}
}

// ---------------------------------------------------------------------
// the arms that skip a check when they are removed
// ---------------------------------------------------------------------

// TestGateLedgerARefusalIsNotSkippedByTheNumberThatWouldMakeItVacuous is the
// group of checks whose removal does not make a wrong recording legal by saying
// yes — it makes it legal by never asking.
//
// Each row is a quantity whose guard also GATES a later check: a size of zero
// skips the prediction arithmetic, a class with no k skips it too, and an
// undecided reserve makes a run ceiling that silently uses zero. Removing any of
// them leaves a suite that is green over an entry nothing examined.
func TestGateLedgerARefusalIsNotSkippedByTheNumberThatWouldMakeItVacuous(t *testing.T) {
	t.Run("a size of zero bytes", func(t *testing.T) {
		m := gateLedgerFixtureModel(t)
		l, _ := gateLedgerFixtureRun(t, "legal-run.json")
		l.Runs[0].Surfaces[0].Size.Value = 0
		if err := gateLedgerValidateLedger(m, l); !errors.Is(err, errGateLedgerNonPositive) {
			t.Fatalf(`a surface sized at zero bytes was accepted: %v

The prediction arithmetic is guarded by a positive size, so a size of zero does
not merely record a wrong number — it makes any prediction at all legal, because
nothing multiplies anything.`, err)
		}
	})

	t.Run("a surface whose class the run records no k for", func(t *testing.T) {
		m := gateLedgerFixtureModel(t)
		l, _ := gateLedgerFixtureRun(t, "legal-run.json")
		var kept []gateLedgerRatioRecord
		for _, k := range l.Runs[0].K {
			if k.Class != "falsification-dense" {
				kept = append(kept, k)
			}
		}
		l.Runs[0].K = kept
		err := gateLedgerValidateLedger(m, l)
		if err == nil || !strings.Contains(err.Error(), "records no k for class") {
			t.Fatalf(`a surface was funded from a k the entry does not carry: %v

Same shape as a size of zero: the arithmetic check is guarded by the k being
present, so a missing k does not fail the multiplication — it removes it, and
whatever the entry predicted stands.`, err)
		}
	})

	t.Run("a run quoted while the reserve is undecided", func(t *testing.T) {
		root := surfaceRepoRoot(t)
		tracked, err := gateLedgerTrackedFiles(root)
		if err != nil {
			t.Fatalf("git ls-files: %v", err)
		}
		m, err := gateLedgerDecodeModel(gateLedgerFixture(t, "model-legal.yaml"))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		m.Reserve = gateLedgerModelQuantity{Undecided: true, Reason: "no run has ever needed headroom here"}
		total, _, err := gateLedgerQuoteRun(m, root, tracked, 0)
		if err == nil {
			t.Fatalf(`a run was quoted at %d output tokens with the reserve undecided.

An undecided quantity carries no value, so the ceiling silently uses zero
headroom — a number nobody chose, arrived at by reading a field that says in as
many words that nobody chose it.`, total)
		}
	})
}

// TestGateLedgerASizeMustNameARatifiedBasisOnItsOwnTerms exercises the basis
// arms where the plan comparison cannot reach them.
//
// Both arms are shadowed for a PLANNED surface: the model's ratified basis
// refuses a mismatch first. For a surface the model does not plan for there is
// nothing to compare against, so these are the only thing standing between an
// unratified size and the arithmetic that consumes it.
func TestGateLedgerASizeMustNameARatifiedBasisOnItsOwnTerms(t *testing.T) {
	for _, tc := range []struct {
		name    string
		basis   string
		wantErr error
		mustSay string
	}{
		{name: "no basis at all", basis: "", mustSay: "names no document basis"},
		{name: "the reserved name for a decision nobody made", basis: gateLedgerUndecided, wantErr: errGateLedgerUnratifiedBasis},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := gateLedgerFixtureModel(t)
			l, _ := gateLedgerFixtureRun(t, "legal-run.json")
			run := &l.Runs[0]
			run.Surfaces = append(run.Surfaces, gateLedgerSurfaceRecord{
				Surface: "a-surface-nobody-planned",
				Class:   "newest-entry-only",
				Size:    gateLedgerSize{Value: 1000, Unit: gateLedgerUnitBytes, Basis: tc.basis},
				Predicted: gateLedgerQuantity{
					Value: 600, Unit: gateLedgerUnitOutput,
				},
			})
			run.Ceiling.Value += 600
			err := gateLedgerValidateLedger(m, l)
			switch {
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("want %v\n got %v", tc.wantErr, err)
			case tc.mustSay != "" && (err == nil || !strings.Contains(err.Error(), tc.mustSay)):
				t.Fatalf("want a refusal saying %q, got %v", tc.mustSay, err)
			}
		})
	}
}

// TestGateLedgerAnOverrideIsRefusedWhenNothingPrecedesTheValue is the last arm
// of the required-and-refused symmetry, exercised where it is reachable.
//
// Inside a whole-ledger check every decided seed pre-seeds the anchor, so this
// arm only fires for a class whose seed is undecided — a state another rule
// already refuses. It is checked here directly, because an escape hatch nobody
// exercises is an escape hatch nobody notices has stopped closing.
func TestGateLedgerAnOverrideIsRefusedWhenNothingPrecedesTheValue(t *testing.T) {
	m := gateLedgerFixtureModel(t)
	run := gateLedgerRun{
		RunID: "r",
		K: []gateLedgerRatioRecord{{
			Class:           "falsification-dense",
			Value:           1.2,
			NumeratorUnit:   gateLedgerUnitOutput,
			DenominatorUnit: gateLedgerUnitBytes,
			BoundOverride:   "because the template has the field",
		}},
	}
	err := errors.Join(gateLedgerCheckRunK("run", m, run, nil)...)
	if !errors.Is(err, errGateLedgerOverrideUnneeded) {
		t.Fatalf(`a reason was recorded for passing a bound that nothing set: %v

The override is the sentence a reviewer reads to find out why a move was bigger
than the rules allow. Accepted where no move happened, it becomes the sentence
every entry carries and nobody reads.`, err)
	}
}

// gateLedgerQuotableRepo builds a throwaway git repository the fixture model can
// actually be quoted over: a surfaces.yaml declaring the three surfaces the
// model plans for, a tracked file behind each, and the model at its canonical
// path.
//
// It is synthetic rather than the real repository for two reasons, and the
// second is the load-bearing one. A quote needs `git ls-files` and a manifest,
// so a bare t.TempDir() would exercise the refusal path only. And standing the
// fixture model up at the REAL repository's canonical path — the obvious
// alternative — would mean a test that edits the working tree while other agents
// and other test binaries are reading it.
func gateLedgerQuotableRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is not on PATH; this quote cannot be exercised, which is a failure and not a skip: %v", err)
	}
	root := t.TempDir()
	gateWrite(t, root, "README.md", strings.Repeat("the front door\n", 40))
	gateWrite(t, root, "CHANGELOG.md", strings.Repeat("the release's account of itself\n", 90))
	gateWrite(t, root, "ROADMAP.md", strings.Repeat("deferred and unverified\n", 15))
	gateWrite(t, root, gateManifestFile, `surfaces:
  - name: readme
    what: the front door
    reach: everyone
    paths: [README.md]
  - name: changelog
    what: the release's account of itself
    reach: upgraders
    paths: [CHANGELOG.md]
  - name: roadmap
    what: the deferred register
    reach: dependants
    paths: [ROADMAP.md]
`)
	gateWrite(t, root, gateLedgerModelPath, string(gateLedgerFixture(t, "model-legal.yaml")))
	gateTestGit(t, root, "init", "-q", "-b", "main")
	gateTestGit(t, root, "add", "-A")
	return root
}
