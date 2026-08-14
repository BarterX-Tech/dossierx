// gate_priority_test.go is THE MATRIX: the one place a finding's consequence and
// its surface's reach are crossed into the word that decides whether this release
// stops.
//
// WHY IT EXISTS. The gate's rule was that every finding blocks and a named human
// clears the ones that should not. That is the right rule for a record and the
// wrong rule for a release: the v0.5.2 rounds produced fifty-eight findings, and
// a cosmetic line in a maintainer-only document stopped the release exactly as
// hard as a wrong install command in the README. The two moves left were to fix
// everything or to spend a human's ruling on everything, and the second one is how
// an override record turns into a rubber stamp — the third property
// gate_override_test.go names ("harder to write than a fix") survives only while
// overrides stay rare. So the gate now blocks on the findings that are worth
// blocking on, and writes the rest down where they cannot be lost.
//
// THE TWO HALVES, AND WHY NEITHER ONE IS ENOUGH ALONE:
//
//	CONSEQUENCE is the reading agent's, and it is the only half an agent handed
//	    one surface is placed to answer: does somebody ACT wrongly because of this
//	    (a command that fails, a flag that does not exist, an install line that
//	    installs the wrong thing), is somebody MISLED (a true-sounding sentence
//	    that is not true, which costs a reader time and trust but not a broken
//	    command), or is it COSMETIC. Three values, closed.
//	REACH_CLASS is surfaces.yaml's, and it is emphatically NOT the agent's. It
//	    says how far the surface carrying the defect travels: client-shipped |
//	    consumer-docs | maintainer | process. It is a reviewed line in a tracked
//	    file, decided once per surface by a human, and the agent judging that
//	    surface never sees a choice about it.
//
// The split is the whole defence. A single agent-written word — the old
// `severity` — is a release-blocking decision made by the party whose work is
// under review, in free text nothing could check. Crossing a closed agent choice
// with a reviewed declaration means an agent can move a finding one row and never
// one column, and moving the column is a diff somebody reviews.
//
// WHAT THE MATRIX SAYS, and it is a judgement rather than an arithmetic:
//
//	                 acts-wrongly   misled     cosmetic
//	client-shipped   P0             P1         P2
//	consumer-docs    P1             P2         P3
//	maintainer       P2             P3         P3
//	process          P2             P3         P3
//
// P0 is the one cell that cannot be ruled away: a client-shipped surface that
// makes its reader ACT wrongly is a defect the release exists to not ship, and a
// signature is not a fix for it. maintainer and process share a row because the
// audience is the same people who can fix it, and the worst case for both is a
// contributor losing an afternoon.
//
// P2 AND P3 DO NOT BLOCK, AND THEY ARE NOT DROPPED. They stay on the receipt,
// unfiltered, exactly as before — and they are additionally projected into
// gate/deferred.json, a tracked file, so that "this did not block the release"
// cannot decay into "nobody ever wrote it down" one release later. That ledger is
// the price of the exit rule; without it this file would be a filter, which is
// the thing CLAUDE.md forbids by name.
//
// Same shape as the rest of the gate: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// the two closed vocabularies
// ---------------------------------------------------------------------

// The consequences an agent may state. Three, and closed: an open field here
// would put the matrix's row lookup at the mercy of whichever synonym an agent
// reached for ("major", "important", "high"), which is the free-text severity
// this design replaced, wearing a different key.
const (
	gateConsequenceActsWrongly = "acts-wrongly"
	gateConsequenceMisled      = "misled"
	gateConsequenceCosmetic    = "cosmetic"
)

// The reach classes surfaces.yaml may declare. `process` is reserved for surfaces
// internal to the gate itself and no surface carries it today; it is in the set
// because the manifest's own header declares it, and a vocabulary the manifest
// states and the code does not accept would refuse the first surface to use it.
const (
	gateReachClientShipped = "client-shipped"
	gateReachConsumerDocs  = "consumer-docs"
	gateReachMaintainer    = "maintainer"
	gateReachProcess       = "process"
)

// The priorities. P0 and P1 block a release; P2 and P3 are deferred to the
// ledger. Four rather than three because P2 and P3 are a real distinction to the
// human reading deferred.json — "worth a fix next release" against "worth knowing"
// — even though the gate treats them identically.
const (
	gatePriorityP0 = "P0"
	gatePriorityP1 = "P1"
	gatePriorityP2 = "P2"
	gatePriorityP3 = "P3"
)

// gateConsequences is the closed set, sorted so that a refusal naming it reads
// the same on every run.
var gateConsequences = []string{gateConsequenceActsWrongly, gateConsequenceCosmetic, gateConsequenceMisled}

// gateReachClasses is the closed set, sorted for the same reason.
var gateReachClasses = []string{gateReachClientShipped, gateReachConsumerDocs, gateReachMaintainer, gateReachProcess}

// gatePriorityMatrix is the table in the file header, as data.
//
// It is a total map over both vocabularies rather than a default plus exceptions.
// A default is what turns a vocabulary entry nobody thought about into a silently
// ranked finding: add a fifth reach_class with a `default: P3` in place and every
// finding on that surface is deferred, with nothing red anywhere.
// TestGatePriorityMatrixIsTotalOverBothVocabularies is what holds that.
var gatePriorityMatrix = map[string]map[string]string{
	gateReachClientShipped: {
		gateConsequenceActsWrongly: gatePriorityP0,
		gateConsequenceMisled:      gatePriorityP1,
		gateConsequenceCosmetic:    gatePriorityP2,
	},
	gateReachConsumerDocs: {
		gateConsequenceActsWrongly: gatePriorityP1,
		gateConsequenceMisled:      gatePriorityP2,
		gateConsequenceCosmetic:    gatePriorityP3,
	},
	gateReachMaintainer: {
		gateConsequenceActsWrongly: gatePriorityP2,
		gateConsequenceMisled:      gatePriorityP3,
		gateConsequenceCosmetic:    gatePriorityP3,
	},
	gateReachProcess: {
		gateConsequenceActsWrongly: gatePriorityP2,
		gateConsequenceMisled:      gatePriorityP3,
		gateConsequenceCosmetic:    gatePriorityP3,
	},
}

// gatePriorityFor crosses a reach class with a consequence, or refuses.
//
// It NEVER returns a priority and an error together, and never a priority for an
// input it does not recognise. A lookup that fell back to P3 on an unknown row
// would rank every finding on a mistyped reach_class as deferred, which is the
// silent-narrowing failure this gate is written against: the manifest typo would
// present as a quiet release rather than as a red one.
func gatePriorityFor(reach, consequence string) (string, error) {
	row, ok := gatePriorityMatrix[reach]
	if !ok {
		return "", fmt.Errorf("reach_class %q is not one of %s, so no priority can be crossed for it. The class is declared per surface in %s and is not a reviewing agent's to choose; a value outside the set is a manifest edit to fix, not a finding to rank",
			reach, strings.Join(gateReachClasses, ", "), gateManifestFile)
	}
	priority, ok := row[consequence]
	if !ok {
		return "", fmt.Errorf("consequence %q is not one of %s, so this finding cannot be ranked. The three are what an agent judging one surface is placed to answer — does a reader ACT wrongly, is a reader MISLED, or is it COSMETIC — and a fourth word is a rank nobody can look up",
			consequence, strings.Join(gateConsequences, ", "))
	}
	return priority, nil
}

// ---------------------------------------------------------------------
// reach_class, read from the manifest
// ---------------------------------------------------------------------

// gateSurfaceReachClasses is every declared surface's reach class, read from
// surfaces.yaml the way gateDeclaredSurfaces reads the fan-out from it: the
// manifest is the declaration, and a second copy of it in the gate would be the
// hand-maintained list this whole file exists to not have.
//
// A surface that declares NO reach_class is simply absent from the map, and the
// refusal for it is the caller's — it belongs where the finding is, so that the
// message can name the finding that cannot be ranked rather than a manifest
// somebody is not editing.
//
// A surface declaring a class OUTSIDE the closed set refuses the whole read,
// including for callers asking about other surfaces. That looks harsh and is the
// point: a typo'd class is a surface whose findings would rank against a row that
// does not exist, and the run that meets it must fail loudly rather than one
// finding at a time.
func gateSurfaceReachClasses(root string) (map[string]string, error) {
	m, err := gateLoadManifest(root)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(gateReachClasses))
	for _, class := range gateReachClasses {
		known[class] = true
	}

	out := make(map[string]string, len(m.Surfaces))
	var problems []string
	for _, entry := range m.Surfaces {
		class := strings.TrimSpace(entry.ReachClass)
		if class == "" {
			continue
		}
		if !known[class] {
			problems = append(problems, fmt.Sprintf("surface %q declares reach_class %q", entry.Name, class))
			continue
		}
		out[entry.Name] = class
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("%s declares a reach class outside the closed set %s: %s. Every finding on such a surface would rank against a row that does not exist, so the read is refused whole rather than surface by surface",
			gateManifestFile, strings.Join(gateReachClasses, ", "), strings.Join(problems, "; "))
	}
	return out, nil
}

// ---------------------------------------------------------------------
// what a priority does to a run
// ---------------------------------------------------------------------

// gatePartitionByPriority splits findings three ways: the ones that BLOCK the
// release, the ones that are DEFERRED to the ledger, and the ones nothing could
// rank.
//
// The third bucket is not a tidy-up. A finding with no priority reached the
// receipt by a path that skipped the ranking, and both ways of folding it into one
// of the other two are wrong in a way that matters: deferring it makes a broken
// producer silent, and blocking it as P0 asserts a classification nobody made. So
// it is kept separate and reported as itself — see gateReceipt.evaluate.
func gatePartitionByPriority(findings []gateFinding) (blocking, deferred, unranked []gateFinding) {
	for _, f := range findings {
		switch f.Priority {
		case gatePriorityP0, gatePriorityP1:
			blocking = append(blocking, f)
		case gatePriorityP2, gatePriorityP3:
			deferred = append(deferred, f)
		default:
			unranked = append(unranked, f)
		}
	}
	return blocking, deferred, unranked
}

// ---------------------------------------------------------------------
// the deferred ledger
// ---------------------------------------------------------------------

// gateDeferredFile is the tracked projection of the receipt's P2/P3 findings.
//
// TRACKED, for gate/overrides.json's reason one step along: a finding the gate
// decided not to block on is a decision, and a decision git cannot see is one
// nobody reviews. It needs an `!deferred.json` line in gate/.gitignore, which
// ignores everything and names what is tracked — that file belongs to the docs
// lane and is not edited here.
const gateDeferredFile = "gate/deferred.json"

// gateDeferredLedger is the file's whole shape.
//
// VERSION IS ON IT because the ledger is a projection of ONE receipt and not an
// append log: it is overwritten on every recording, so without the release it was
// projected from, a reader meeting it in a diff cannot say which run decided
// these were deferred. Findings is never null and never omitted — a release that
// deferred nothing writes `[]`, which is a fact, where an absent key would be
// indistinguishable from a recording that never wrote the file.
type gateDeferredLedger struct {
	Version  string        `json:"version"`
	Findings []gateFinding `json:"findings"`
}

// gateDeferredFindings is the P2/P3 subset of a receipt's findings, in the order
// the receipt already sorted them.
//
// It re-uses that order rather than sorting again, so the ledger cannot disagree
// with the receipt about which of two equal-looking findings came first — one
// comparator, one answer, and a re-run over an unchanged tree reproduces both
// documents byte for byte.
func gateDeferredFindings(findings []gateFinding) []gateFinding {
	_, deferred, _ := gatePartitionByPriority(findings)
	if deferred == nil {
		// A non-nil empty slice: `[]`, not `null`. See the type comment.
		return []gateFinding{}
	}
	return deferred
}

// gateWriteDeferred writes the ledger, replacing whatever was there.
//
// IT OVERWRITES, and that is the design rather than a shortcut. The ledger is a
// projection of the receipt this run measured; an append log would accumulate
// findings that were fixed three releases ago and read, to the human who opens it,
// as a backlog nobody is working — which is the shape that gets a file deleted
// wholesale. What was deferred by an earlier release is in that release's history,
// where git keeps it.
//
// It writes through a temporary file in the same directory and renames, for
// gateAnswerWrite's reason: a projection half-written by a process that died
// parses far enough to look like a complete one.
func gateWriteDeferred(root, version string, findings []gateFinding) error {
	ledger := gateDeferredLedger{Version: version, Findings: gateDeferredFindings(findings)}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", gateDeferredFile, err)
	}
	data = append(data, '\n')

	dest := filepath.Join(root, filepath.FromSlash(gateDeferredFile))
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create the directory for %s: %w", gateDeferredFile, err)
	}
	f, err := os.CreateTemp(dir, ".deferred-*.json")
	if err != nil {
		return fmt.Errorf("create a temporary file beside %s: %w", gateDeferredFile, err)
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
	// CreateTemp makes the file 0600 and this one is committed and read by
	// whoever opens the repository.
	if err := os.Chmod(tmp, 0o644); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmp, gateDeferredFile, err)
	}
	return nil
}

// gateReadDeferred reads the ledger back, for the tests below and for anything
// that later wants to show a human what this release deferred.
func gateReadDeferred(root string) (gateDeferredLedger, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateDeferredFile)))
	if err != nil {
		return gateDeferredLedger{}, nil, fmt.Errorf("read %s: %w", gateDeferredFile, err)
	}
	var ledger gateDeferredLedger
	if err := json.Unmarshal(raw, &ledger); err != nil {
		return gateDeferredLedger{}, raw, fmt.Errorf("parse %s: %w", gateDeferredFile, err)
	}
	return ledger, raw, nil
}

// ---------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------

// gatePriorityNameLine matches one surface entry's opening line in surfaces.yaml.
// The helper below is a text edit rather than a parse-and-re-emit because
// re-emitting the manifest through gateManifest would silently drop every field
// that type does not model — `what:`, `reach:`, the comments — and a fixture that
// quietly rewrites the document it is meant to vary is a fixture whose failures
// are about itself.
var gatePriorityNameLine = regexp.MustCompile(`^  - name: (\S+)\s*$`)

// gatePriorityManifestClasses rewrites root's copy of surfaces.yaml so that each
// named surface carries exactly the given reach_class — or none at all, for the
// empty string, which is the shape of a manifest the reach_class lane has not
// reached yet.
//
// Surfaces not named are left exactly as the real manifest has them, so a test
// varies one thing.
func gatePriorityManifestClasses(t *testing.T, root string, classes map[string]string) {
	t.Helper()
	path := filepath.Join(root, gateManifestFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the fixture's %s: %v", gateManifestFile, err)
	}

	touched := map[string]bool{}
	var out []string
	current := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if m := gatePriorityNameLine.FindStringSubmatch(line); m != nil {
			current = m[1]
			out = append(out, line)
			if class, ok := classes[current]; ok {
				touched[current] = true
				if class != "" {
					out = append(out, "    reach_class: "+class)
				}
			}
			continue
		}
		// Any reach_class the manifest already declares for a surface this call is
		// setting is dropped here; the replacement was written above.
		if _, ok := classes[current]; ok && strings.HasPrefix(line, "    reach_class:") {
			continue
		}
		out = append(out, line)
	}
	for name := range classes {
		if !touched[name] {
			t.Fatalf("%s declares no surface called %q, so this fixture would vary nothing and every assertion below would be about the manifest as it stands", gateManifestFile, name)
		}
	}

	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		t.Fatalf("write the fixture's %s: %v", gateManifestFile, err)
	}
}

// gatePriorityFillManifest gives a reach class to every declared surface that has
// none, and returns the whole map afterwards.
//
// WHY A FIXTURE FILLS AT ALL. surfaces.yaml is another lane's file, and a test
// that only passes once that lane has classified all thirteen surfaces is a test
// that reds for somebody else's reason. Filling makes every recording fixture
// self-contained. The value it fills with is `process`, which the manifest's own
// header says no surface carries yet — so a class that came from this fixture is
// visibly the fixture's, and a test that meant to assert a real one cannot pass by
// accident. Every row that cares about the priority sets the class explicitly.
func gatePriorityFillManifest(t *testing.T, root string) map[string]string {
	t.Helper()
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	classes, err := gateSurfaceReachClasses(root)
	if err != nil {
		t.Fatalf("the fixture's manifest declares an unusable reach class: %v", err)
	}
	fill := map[string]string{}
	for _, name := range declared {
		if classes[name] == "" {
			fill[name] = gateReachProcess
		}
	}
	if len(fill) > 0 {
		gatePriorityManifestClasses(t, root, fill)
	}
	filled, err := gateSurfaceReachClasses(root)
	if err != nil {
		t.Fatalf("the filled manifest declares an unusable reach class: %v", err)
	}
	for _, name := range declared {
		if filled[name] == "" {
			t.Fatalf("surface %q still declares no reach_class after the fixture filled the manifest; every recording below would refuse for that reason instead of the one under test", name)
		}
	}
	return filled
}

// gatePriorityFixture is a fanned-out checkout in which the named surfaces carry
// the named reach classes and every other declared surface carries one too.
//
// THE MANIFEST IS EDITED BEFORE THE FAN-OUT IS PRODUCED, never after. The
// fan-out freezes what the run's question was — the digest of surfaces.yaml at
// the first production — so a fixture that moved the manifest afterwards would be
// exercising the freeze rather than the matrix.
func gatePriorityFixture(t *testing.T, classes map[string]string) (root string, tracked []string, surface string) {
	t.Helper()
	root, tracked = gateFanoutFixture(t)
	gatePriorityFillManifest(t, root)
	if len(classes) > 0 {
		gatePriorityManifestClasses(t, root, classes)
	}
	if _, err := gateFanoutProduce(root, gateStage2FixtureTree, gateStage2FixtureTree, tracked); err != nil {
		t.Fatalf("the fan-out this recorder records against could not be produced: %v", err)
	}
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	surface = declared[0]
	for name := range classes {
		surface = name
	}
	return root, tracked, surface
}

// gatePriorityFinding is a receipt-shaped finding at one priority, for the exit
// and ledger tests that do not go through the recorder.
func gatePriorityFinding(surface, rule, consequence, priority string) gateFinding {
	return gateFinding{
		Surface:         surface,
		Rule:            rule,
		Consequence:     consequence,
		FailureScenario: "a reader following " + surface + " meets " + rule + " and is worse off for it",
		Detail:          "the document says one thing and the tree does another",
		Priority:        priority,
	}
}

// ---------------------------------------------------------------------
// the matrix itself
// ---------------------------------------------------------------------

// TestGatePriorityMatrixIsTotalOverBothVocabularies walks all twelve cells by
// NAME and by expected value, written out rather than derived from the table it
// is checking.
//
// A test that iterated gatePriorityMatrix would assert that the table equals
// itself. The twelve rows below are the maintainer's judgement written twice, so
// that changing one cell is a red build and a conversation rather than a silent
// re-ranking of every finding on that row.
func TestGatePriorityMatrixIsTotalOverBothVocabularies(t *testing.T) {
	for _, tc := range []struct {
		reach, consequence, want string
	}{
		{gateReachClientShipped, gateConsequenceActsWrongly, gatePriorityP0},
		{gateReachClientShipped, gateConsequenceMisled, gatePriorityP1},
		{gateReachClientShipped, gateConsequenceCosmetic, gatePriorityP2},
		{gateReachConsumerDocs, gateConsequenceActsWrongly, gatePriorityP1},
		{gateReachConsumerDocs, gateConsequenceMisled, gatePriorityP2},
		{gateReachConsumerDocs, gateConsequenceCosmetic, gatePriorityP3},
		{gateReachMaintainer, gateConsequenceActsWrongly, gatePriorityP2},
		{gateReachMaintainer, gateConsequenceMisled, gatePriorityP3},
		{gateReachMaintainer, gateConsequenceCosmetic, gatePriorityP3},
		{gateReachProcess, gateConsequenceActsWrongly, gatePriorityP2},
		{gateReachProcess, gateConsequenceMisled, gatePriorityP3},
		{gateReachProcess, gateConsequenceCosmetic, gatePriorityP3},
	} {
		t.Run(tc.reach+"/"+tc.consequence, func(t *testing.T) {
			got, err := gatePriorityFor(tc.reach, tc.consequence)
			if err != nil {
				t.Fatalf("the matrix cannot rank a %s finding on a %s surface, which is a cell of its own table: %v", tc.consequence, tc.reach, err)
			}
			if got != tc.want {
				t.Errorf("a %s finding on a %s surface ranks %s; this table says %s, and the difference is whether the release stops", tc.consequence, tc.reach, got, tc.want)
			}
		})
	}

	// Every vocabulary member has a row and every row is full: a value added to
	// either set with no cell for it must red here rather than rank as whatever a
	// missing-key lookup returns.
	for _, reach := range gateReachClasses {
		for _, consequence := range gateConsequences {
			if _, err := gatePriorityFor(reach, consequence); err != nil {
				t.Errorf("%s × %s has no cell; a declared vocabulary member the matrix cannot rank would refuse every finding on that surface: %v", reach, consequence, err)
			}
		}
	}
}

// TestGatePriorityRefusesWhatItCannotLookUp is the direction that matters more
// than the twelve cells: an input outside either vocabulary produces a REFUSAL and
// never a priority.
//
// The failure it stops is quiet. A lookup returning P3 for a mistyped
// reach_class defers every finding on that surface, and the release goes out green
// over a manifest typo — the silent narrowing this gate's first rule forbids.
func TestGatePriorityRefusesWhatItCannotLookUp(t *testing.T) {
	for _, tc := range []struct {
		name, reach, consequence, want string
	}{
		{"a reach class the manifest invented", "client-facing", gateConsequenceActsWrongly, "reach_class \"client-facing\""},
		{"no reach class at all", "", gateConsequenceMisled, "reach_class \"\""},
		{"a consequence the agent invented", gateReachClientShipped, "major", "consequence \"major\""},
		{"no consequence at all", gateReachClientShipped, "", "consequence \"\""},
		{"the old severity vocabulary", gateReachClientShipped, "blocking", "consequence \"blocking\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := gatePriorityFor(tc.reach, tc.consequence)
			if err == nil {
				t.Fatalf("%s × %s ranked as %s; an input the matrix does not recognise must refuse, or a typo becomes a ranking", tc.reach, tc.consequence, got)
			}
			if got != "" {
				t.Errorf("the refusal also returned priority %q; a caller that checks the value before the error would rank on it", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %s:\n%v", tc.want, err)
			}
		})
	}
}

// TestGatePriorityReadsTheReachClassTheManifestDeclares holds the parse against
// the real file, and the closed-set refusal against a fixture.
func TestGatePriorityReadsTheReachClassTheManifestDeclares(t *testing.T) {
	root := surfaceRepoRoot(t)
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	classes, err := gateSurfaceReachClasses(root)
	if err != nil {
		t.Fatalf("%s declares a reach class the matrix cannot use, so every finding recorded against this tree would refuse: %v", gateManifestFile, err)
	}
	// Every class the real manifest states is one the matrix has a row for. A
	// surface that states none is NOT failed here: this lane crosses the two
	// vocabularies, and classifying thirteen surfaces is the manifest lane's work.
	// What must not happen is a class that reads fine and ranks nothing.
	for _, name := range declared {
		class, ok := classes[name]
		if !ok {
			continue
		}
		if _, err := gatePriorityFor(class, gateConsequenceMisled); err != nil {
			t.Errorf("surface %q declares reach_class %q, which the matrix has no row for: %v", name, class, err)
		}
	}

	t.Run("a class outside the closed set refuses the whole read", func(t *testing.T) {
		overlay, _ := gateStage2Overlay(t)
		gatePriorityManifestClasses(t, overlay, map[string]string{declared[0]: "client-facing"})
		if _, err := gateSurfaceReachClasses(overlay); err == nil {
			t.Fatalf("a manifest declaring reach_class %q was read without complaint; every finding on that surface would then rank against a row that does not exist", "client-facing")
		} else if !strings.Contains(err.Error(), declared[0]) || !strings.Contains(err.Error(), "client-facing") {
			t.Errorf("the refusal names neither the surface nor the class, so nobody can find the line to fix:\n%v", err)
		}
	})
}

// ---------------------------------------------------------------------
// the recorder ranks, and refuses what it cannot rank
// ---------------------------------------------------------------------

// TestGateAnswerRecordStampsThePriorityTheMatrixGives is the positive control:
// an agent states a consequence, the manifest states a reach class, and the
// answer that lands on disk carries the crossing of the two.
//
// The priority is asserted on the FILE and not only on the returned value,
// because the file is what the collector reads and what the receipt is built
// from. A recorder that computed the rank and did not write it would pass every
// assertion made against its own return.
func TestGateAnswerRecordStampsThePriorityTheMatrixGives(t *testing.T) {
	root, tracked, surface := gatePriorityFixture(t, map[string]string{"readme": gateReachClientShipped})

	payload := gateAnswerHonestPayload(t, root)
	payload["verdict"] = gateVerdictFailed
	payload["findings"] = []any{gateAnswerFindingPayload(func(f map[string]any) {
		f["consequence"] = gateConsequenceActsWrongly
	})}

	written, err := gateAnswerRecord(root, surface, gateAnswerWritePayload(t, payload), gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the recorder refused an answer whose finding it can rank: %v", err)
	}
	if len(*written.Findings) != 1 {
		t.Fatalf("the recorder wrote %d finding(s) for a payload carrying one", len(*written.Findings))
	}
	got := (*written.Findings)[0]
	if got.Priority != gatePriorityP0 {
		t.Errorf("an acts-wrongly finding on a client-shipped surface was ranked %q; the matrix says %s, and the difference is whether this release can be published at all", got.Priority, gatePriorityP0)
	}
	// The surface is the harness's, stamped from -answer-surface: the payload
	// never named one.
	if got.Surface != surface {
		t.Errorf("the recorded finding is attributed to %q rather than to the surface it was filed under, %q", got.Surface, surface)
	}

	loaded, err := gateStage3LoadAnswer(root, surface)
	if err != nil {
		t.Fatalf("read back the answer the recorder wrote: %v", err)
	}
	if len(*loaded.Findings) != 1 || (*loaded.Findings)[0].Priority != gatePriorityP0 {
		t.Fatalf("the priority did not reach the file the collector reads; the receipt would be built from findings nothing ranked:\n%+v", *loaded.Findings)
	}
}

// TestGateAnswerRecordRefusesAFindingItCannotRank is every shape that must not
// land, and the whole of the argument for refusing rather than defaulting: a
// finding that cannot be prioritized is not a smaller finding.
//
// Each row would otherwise produce a perfectly well-formed answer file whose
// finding carries an empty priority — which evaluate() blocks on, at the end of
// the run, with a message about a producer rather than about the agent that
// wrote it.
func TestGateAnswerRecordRefusesAFindingItCannotRank(t *testing.T) {
	for _, tc := range []struct {
		name string
		// classes edits the fixture's manifest before the fan-out.
		classes map[string]string
		mutate  func(f map[string]any)
		want    string
	}{
		{
			name:   "a consequence outside the three",
			mutate: func(f map[string]any) { f["consequence"] = "major" },
			want:   "consequence \"major\"",
		},
		{
			name:   "the severity vocabulary the schema replaced",
			mutate: func(f map[string]any) { f["consequence"] = "blocking" },
			want:   "consequence \"blocking\"",
		},
		{
			name:   "no consequence at all",
			mutate: func(f map[string]any) { delete(f, "consequence") },
			want:   "consequence \"\"",
		},
		{
			name:   "no failure scenario",
			mutate: func(f map[string]any) { delete(f, "failure_scenario") },
			want:   "names no `failure_scenario`",
		},
		{
			name:   "a failure scenario that is whitespace",
			mutate: func(f map[string]any) { f["failure_scenario"] = "   " },
			want:   "names no `failure_scenario`",
		},
		{
			// The manifest as it stands before the reach_class lane reaches a
			// surface. It is a refusal and not a default, because a default here
			// would rank every finding on every unclassified surface at whatever
			// the default is, invisibly.
			name:    "a surface the manifest gives no reach class",
			classes: map[string]string{"readme": ""},
			want:    "declares no reach_class",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			classes := tc.classes
			if classes == nil {
				classes = map[string]string{"readme": gateReachClientShipped}
			}
			root, tracked, surface := gatePriorityFixture(t, classes)

			payload := gateAnswerHonestPayload(t, root)
			payload["verdict"] = gateVerdictFailed
			payload["findings"] = []any{gateAnswerFindingPayload(tc.mutate)}

			_, err := gateAnswerRecord(root, surface, gateAnswerWritePayload(t, payload), gateStage2FixtureTree, tracked)
			if err == nil {
				t.Fatalf("the answer was recorded. gate/answers/%s.json now holds a finding nothing ranked, and the run's own exit rule cannot say whether it blocks the release", surface)
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

// TestGatePriorityMatrixIsActuallyConsultedByTheRecordingPath is the wire, and it
// exists because this gate has already shipped the failure it guards against
// TWICE in one release: an override loader wired to nothing but its own test, and
// before that a cross-surface join whose findings reached no receipt. Every
// property of the matrix above can be true of a table nothing reads.
//
// It is a MUTATION rather than an equality: the same payload, recorded under two
// manifests that differ in one line, must produce two different priorities. A
// recorder that stamped a constant, that ranked from the consequence alone, or
// that read a reach class from anywhere but surfaces.yaml passes an equality
// assertion and fails this one.
func TestGatePriorityMatrixIsActuallyConsultedByTheRecordingPath(t *testing.T) {
	record := func(t *testing.T, class, consequence string) gateFinding {
		t.Helper()
		root, tracked, surface := gatePriorityFixture(t, map[string]string{"readme": class})
		payload := gateAnswerHonestPayload(t, root)
		payload["verdict"] = gateVerdictFailed
		payload["findings"] = []any{gateAnswerFindingPayload(func(f map[string]any) {
			f["consequence"] = consequence
		})}
		written, err := gateAnswerRecord(root, surface, gateAnswerWritePayload(t, payload), gateStage2FixtureTree, tracked)
		if err != nil {
			t.Fatalf("recording a %s finding on a %s surface was refused: %v", consequence, class, err)
		}
		return (*written.Findings)[0]
	}

	// One consequence, two reach classes: the column is fixed, so any difference
	// is the manifest's line reaching the recorder.
	shipped := record(t, gateReachClientShipped, gateConsequenceActsWrongly)
	maintainer := record(t, gateReachMaintainer, gateConsequenceActsWrongly)
	if shipped.Priority == maintainer.Priority {
		t.Fatalf("the same finding ranked %s on a client-shipped surface and %s on a maintainer one. The recorder is not reading %s: a table nothing consults ranks nothing, which is the defect this gate shipped an override loader with one release ago",
			shipped.Priority, maintainer.Priority, gateManifestFile)
	}
	if want, _ := gatePriorityFor(gateReachClientShipped, gateConsequenceActsWrongly); shipped.Priority != want {
		t.Errorf("a client-shipped acts-wrongly finding recorded as %s; the matrix says %s", shipped.Priority, want)
	}
	if want, _ := gatePriorityFor(gateReachMaintainer, gateConsequenceActsWrongly); maintainer.Priority != want {
		t.Errorf("a maintainer acts-wrongly finding recorded as %s; the matrix says %s", maintainer.Priority, want)
	}

	// And one reach class, two consequences: the row is fixed, so any difference
	// is the agent's own word reaching the recorder.
	misled := record(t, gateReachClientShipped, gateConsequenceMisled)
	if misled.Priority == shipped.Priority {
		t.Fatalf("acts-wrongly and misled both ranked %s on the same surface; the recorder is stamping a rank from the reach class alone, and the agent's judgement decides nothing", misled.Priority)
	}
}

// ---------------------------------------------------------------------
// what blocks, what defers, and what a ruling can reach
// ---------------------------------------------------------------------

// gatePriorityReceipt is a receipt over one surface holding the given findings,
// with the surface reporting FAILED — the shape a run produces when an agent
// found something.
func gatePriorityReceipt(t *testing.T, work, version string, findings ...gateFinding) gateReceipt {
	t.Helper()
	verdict := gateVerdictFailed
	if len(findings) == 0 {
		verdict = gateVerdictPass
	}
	receipt, err := gateRecordReceipt(work, version, "release",
		[]gateSurfaceVerdict{{Surface: "readme", Verdict: verdict, Fingerprint: "sha256:readme"}}, findings)
	if err != nil {
		t.Fatalf("record the receipt: %v", err)
	}
	return receipt
}

var (
	gatePriorityDeclared = []string{"readme"}
	gatePriorityCurrent  = map[string]string{"readme": "sha256:readme"}
)

// TestGateP0CannotBeWavedThroughBySignature is the one cell no ruling reaches.
//
// The override record is the gate's escape hatch and this is its boundary: a
// client-shipped surface that makes its reader ACT wrongly — an install command
// that installs the wrong thing, a flag that does not exist — is what the release
// exists to not ship. A named human's signature does not make the command work.
// The refusal is on the RECORD rather than on the finding, so the whole
// evaluation fails and the file is visibly the thing that failed it.
func TestGateP0CannotBeWavedThroughBySignature(t *testing.T) {
	work := gateFixtureRepo(t)
	blocking := gatePriorityFinding("readme", "install-command-is-wrong", gateConsequenceActsWrongly, gatePriorityP0)
	receipt := gatePriorityReceipt(t, work, "v0.5.2", blocking)

	receipt.Overrides = []gateOverride{gateOverrideFor(receipt.Findings[0], nil)}
	verdict, err := receipt.evaluate(gatePriorityDeclared, gatePriorityCurrent)
	if verdict != gateVerdictFailed {
		t.Fatalf("a P0 was cleared by a ruling; the release publishes with a client-shipped surface that makes its reader act wrongly")
	}
	for _, want := range []string{"P0", "cannot be waved through", "fix"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q, so the human is stopped without being told that this one is not theirs to rule on:\n%v", want, err)
		}
	}

	// And the finding is still blocking after the record is refused: a refused
	// override must not leave the run reading as though nobody had raised it.
	if !strings.Contains(err.Error(), "install-command-is-wrong") {
		t.Errorf("the FAILED does not name the finding that is blocking it:\n%v", err)
	}
}

// TestGateP1BlocksUntilAHumanRulesOnIt is the other side of the boundary: P1 is
// where the override record still works, unchanged, with every refusal
// gate_override_test.go pins still in force.
func TestGateP1BlocksUntilAHumanRulesOnIt(t *testing.T) {
	work := gateFixtureRepo(t)
	f := gatePriorityFinding("readme", "one-envelope-claim", gateConsequenceMisled, gatePriorityP1)
	receipt := gatePriorityReceipt(t, work, "v0.5.2", f)

	verdict, err := receipt.evaluate(gatePriorityDeclared, gatePriorityCurrent)
	if verdict != gateVerdictFailed {
		t.Fatalf("an unruled P1 did not stop the release")
	}
	if !strings.Contains(err.Error(), "P0/P1 finding(s) reached the report") {
		t.Errorf("the FAILED does not say that what stopped it was an unruled P0/P1:\n%v", err)
	}

	receipt.Overrides = []gateOverride{gateOverrideFor(receipt.Findings[0], nil)}
	if verdict, err := receipt.evaluate(gatePriorityDeclared, gatePriorityCurrent); verdict != gateVerdictPass {
		t.Fatalf("a P1 ruled on by a named human with a reason did not clear: %s %v", verdict, err)
	}

	// The ruling is still harder to write than a fix: the same override against
	// the same finding in the NEXT release is refused, which is the property that
	// keeps P1 from becoming a permanent waiver.
	next := receipt
	next.Version = "v0.5.3"
	if verdict, _ := next.evaluate(gatePriorityDeclared, gatePriorityCurrent); verdict != gateVerdictFailed {
		t.Error("a v0.5.2 ruling cleared the same P1 in v0.5.3; an override is never inherited")
	}
}

// TestGateDeferredFindingsDoNotBlockAndAreStillOnTheReceipt is the exit rule's
// whole point, and its whole risk, asserted together.
//
// The point: a receipt whose only findings are P2/P3 evaluates PASS with nobody
// ruling on anything, so a cosmetic line in a maintainer document no longer costs
// a human's signature. The risk: that this becomes a filter. So the same test
// asserts the findings are still on the receipt, that the surface's own reported
// FAILED is still readable there, and that the ledger holds them.
func TestGateDeferredFindingsDoNotBlockAndAreStillOnTheReceipt(t *testing.T) {
	work := gateFixtureRepo(t)
	receipt := gatePriorityReceipt(t, work, "v0.5.2",
		gatePriorityFinding("readme", "stale-count", gateConsequenceCosmetic, gatePriorityP2),
		gatePriorityFinding("readme", "wording-drift", gateConsequenceCosmetic, gatePriorityP3),
	)

	verdict, err := receipt.evaluate(gatePriorityDeclared, gatePriorityCurrent)
	if verdict != gateVerdictPass {
		t.Fatalf("a receipt whose only findings are deferred did not pass: %v", err)
	}
	if len(receipt.Findings) != 2 {
		t.Fatalf("the receipt carries %d of the 2 findings the run reported; a priority that removes a finding from the record is a filter, which is the one thing this gate must not be", len(receipt.Findings))
	}
	for _, v := range receipt.Surfaces {
		if v.Verdict != gateVerdictFailed {
			t.Errorf("the receipt's own record of surface %q reads %q; what the agent reported has to stay readable beside what the priority decided", v.Surface, v.Verdict)
		}
	}

	ledger, _, err := gateReadDeferred(work)
	if err != nil {
		t.Fatalf("the recording did not leave a readable %s: %v", gateDeferredFile, err)
	}
	if ledger.Version != "v0.5.2" {
		t.Errorf("%s names release %q; a projection of one receipt that does not say which one cannot be read in a diff", gateDeferredFile, ledger.Version)
	}
	if len(ledger.Findings) != 2 {
		t.Fatalf("%s holds %d of the 2 deferred findings. A finding that neither blocks nor lands here has been dropped, and the release reads as one that found nothing:\n%+v", gateDeferredFile, len(ledger.Findings), ledger.Findings)
	}
	for _, f := range ledger.Findings {
		if f.Priority != gatePriorityP2 && f.Priority != gatePriorityP3 {
			t.Errorf("%s carries a %s finding; the ledger is the deferred findings and a blocking one filed here reads as decided", gateDeferredFile, f.Priority)
		}
		if f.Digest == "" || f.FailureScenario == "" {
			t.Errorf("the ledger's entry for %s/%s carries no digest or no failure scenario, so a human reading it later cannot rule on it or tell what it costs: %+v", f.Surface, f.Rule, f)
		}
	}

	// A blocking finding is NOT in the ledger — it is not deferred, it is
	// stopping the release.
	blocking := gatePriorityReceipt(t, work, "v0.5.2",
		gatePriorityFinding("readme", "install-command-is-wrong", gateConsequenceActsWrongly, gatePriorityP0),
		gatePriorityFinding("readme", "stale-count", gateConsequenceCosmetic, gatePriorityP2),
	)
	if verdict, _ := blocking.evaluate(gatePriorityDeclared, gatePriorityCurrent); verdict != gateVerdictFailed {
		t.Error("a receipt holding a P0 beside two deferred findings passed")
	}
	ledger, _, err = gateReadDeferred(work)
	if err != nil {
		t.Fatalf("read the ledger of the second recording: %v", err)
	}
	if len(ledger.Findings) != 1 || ledger.Findings[0].Rule != "stale-count" {
		t.Errorf("the ledger of a run holding one deferred finding and one P0 holds %+v", ledger.Findings)
	}
}

// TestGateDeferredLedgerIsAProjectionAndNotAnAppendLog covers the two properties
// the file's usefulness rests on: the same run written twice produces the same
// bytes, and a second recording REPLACES the first rather than accumulating.
//
// Byte-stability is not tidiness. The receipt's whole argument is that a re-run
// over an unchanged tree reproduces the identical document — that is why a
// diverged branch wedges and why the merge is the only way out. A ledger written
// beside it whose bytes moved on every run would make every gate re-run a dirty
// tree, and the landing script's stray check refuses on exactly that.
func TestGateDeferredLedgerIsAProjectionAndNotAnAppendLog(t *testing.T) {
	work := gateFixtureRepo(t)
	deferred := []gateFinding{
		gatePriorityFinding("readme", "stale-count", gateConsequenceCosmetic, gatePriorityP2),
		gatePriorityFinding("readme", "wording-drift", gateConsequenceCosmetic, gatePriorityP3),
	}

	gatePriorityReceipt(t, work, "v0.5.2", deferred...)
	_, first, err := gateReadDeferred(work)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}

	// The same findings, handed over in the other order.
	gatePriorityReceipt(t, work, "v0.5.2", deferred[1], deferred[0])
	_, second, err := gateReadDeferred(work)
	if err != nil {
		t.Fatalf("read the ledger after the second recording: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("the ledger's bytes depend on the order the findings arrived in, so a re-run over an unchanged tree leaves a modified tracked file:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	// A recording that defers nothing leaves an EMPTY ledger, not the previous
	// one. An append log here would report findings this release does not have.
	gatePriorityReceipt(t, work, "v0.5.2")
	ledger, raw, err := gateReadDeferred(work)
	if err != nil {
		t.Fatalf("read the ledger after a clean recording: %v", err)
	}
	if len(ledger.Findings) != 0 {
		t.Fatalf("a run that deferred nothing left %d finding(s) in %s; the ledger is a projection of this receipt, and one that accumulates reads as a backlog nobody is working", len(ledger.Findings), gateDeferredFile)
	}
	if !strings.Contains(string(raw), "\"findings\": []") {
		t.Errorf("a run that deferred nothing wrote its ledger as something other than an empty list; `null` and an absent key are both indistinguishable from a recording that never wrote the file:\n%s", raw)
	}
}

// TestGateUnrankedFindingBlocksAndSaysWhy pins the direction the exit rule errs
// in for a finding no priority reached.
//
// It is the shape a producer bug makes: a finding assembled somewhere that did not
// go through the recorder. Defaulting it to P3 would make every such path silent —
// a whole surface's findings could stop counting with nothing red — so it blocks,
// and it is reported as itself rather than folded in with the findings a human
// could rule on.
func TestGateUnrankedFindingBlocksAndSaysWhy(t *testing.T) {
	work := gateFixtureRepo(t)
	f := gatePriorityFinding("readme", "came-from-nowhere", gateConsequenceMisled, "")
	receipt := gatePriorityReceipt(t, work, "v0.5.2", f)

	verdict, err := receipt.evaluate(gatePriorityDeclared, gatePriorityCurrent)
	if verdict != gateVerdictFailed {
		t.Fatalf("a finding nothing ranked passed the gate; every producer that skips the ranking is then invisible")
	}
	if !strings.Contains(err.Error(), "no priority") {
		t.Errorf("the FAILED does not say that the finding carries no priority, so it reads as an ordinary unruled finding and the producer is never fixed:\n%v", err)
	}

	// It is also not in the ledger: an unranked finding is not a deferred one.
	ledger, _, err := gateReadDeferred(work)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if len(ledger.Findings) != 0 {
		t.Errorf("a finding nothing ranked was written to %s as deferred: %+v", gateDeferredFile, ledger.Findings)
	}
}
