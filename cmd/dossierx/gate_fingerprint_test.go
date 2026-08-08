// gate_fingerprint_test.go is the per-surface fingerprint: the digest of
// everything one surface agent reads, and the three checks built over it.
//
// WHAT IT BUYS. The gate fans one read-only agent out per declared surface, and
// most releases move most surfaces not at all. A fingerprint over an agent's
// INPUTS lets a byte-identical re-run carry its previous verdict forward and
// re-run only the surfaces that actually moved. The saving is the small half of
// the point. The large half is that "green" stops meaning "the last run was a
// full pass" — a claim about an event — and starts meaning "every declared
// surface holds a PASS whose fingerprint matches the tree being released" — a
// claim about the tree in front of you. That is strictly stronger, and it is the
// same shift the receipt next door makes for the merge.
//
// WHAT IS IN THE DIGEST, and why each part has to be. The surface's own
// documents (what the agent judges), surface.json (the mechanical truth it
// judges them against), the release delta and the rendered site text (the other
// two evidence files an agent reads), and the METHOD — because a verdict is a
// function of the question as much as of the evidence, and re-using a verdict
// produced by an older prompt against a newer one is exactly the stale pass this
// design exists to remove.
//
// METHOD_VERSION IS COMPUTED, NEVER WRITTEN DOWN. It is a hash of the prompt
// sources, the model id and the tool list. A hand-maintained string is a version
// somebody eventually forgets to bump, and the run where they forget is the run
// where a stale verdict survives a real change in how the gate asks its
// question. Every input this file refuses to hash out of nothing — an empty
// prompt set, an empty model, an empty tool list, a missing evidence file — is
// refused for that reason: a digest computed over nothing is a constant, and a
// constant carries every verdict forward forever.
//
// Same shape as gate_receipt_test.go and surface_test.go: test code, not a cobra
// command, not compiled into the shipped binary, and outside surface.json's
// behaviour_fingerprint.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------
// the shared evidence
// ---------------------------------------------------------------------

// The evidence every surface agent reads, whichever surface it is assigned,
// relative to the repository root.
//
// surface.json is emitted by surface_test.go and is in the tree today. The other
// two are the gate's own working artifacts, produced by the run that fans the
// agents out — the release delta (this release's surface.json against the
// previous release's) and the rendered site text (read out of a real build's
// DOM, per "verify the thing the user sees"). They are named as constants here
// rather than passed in so that there is one spelling of each, and hashing FAILS
// when one is absent rather than quietly fingerprinting a smaller evidence set.
const (
	gateSurfaceInventoryFile = "surface.json"
	gateDeltaFile            = "gate/delta.json"
	gateSiteTextFile         = "gate/site-text.json"
)

// gateSharedEvidence is those three, in one place, so a fourth evidence file is
// added once and every surface's fingerprint moves.
func gateSharedEvidence() []string {
	return []string{gateSurfaceInventoryFile, gateDeltaFile, gateSiteTextFile}
}

// ---------------------------------------------------------------------
// the method
// ---------------------------------------------------------------------

// gateMethod is HOW a surface is judged: the prompt sources the agent is given,
// the model id it runs on, and the tool names it may call. Its digest is
// method_version.
type gateMethod struct {
	// Prompts are repo-relative paths, hashed by CONTENT. Naming the files
	// rather than inlining the text is what keeps the version honest: editing a
	// prompt moves the digest with no second edit anywhere.
	Prompts []string
	Model   string
	Tools   []string
}

// version is method_version: sha256 over the prompt sources' bytes, the model
// id, and the sorted tool list.
//
// Every emptiness is an error rather than a shorter stream. A method with no
// prompts asks nothing, a method with no model is not a method, and an agent
// with no tools reads no evidence — each would still hash to a perfectly stable
// value, and that stable value would carry verdicts forward across exactly the
// changes this digest exists to notice.
func (m gateMethod) version(root string) (string, error) {
	switch {
	case len(m.Prompts) == 0:
		return "", errors.New("method_version: no prompt sources; a method that asks nothing hashes to a constant, and a constant carries every stale verdict forward")
	case strings.TrimSpace(m.Model) == "":
		return "", errors.New("method_version: no model id; the same prompt on a different model is a different question")
	case len(m.Tools) == 0:
		return "", errors.New("method_version: no tools; an agent that can call nothing reads no evidence, and a verdict over zero evidence must not be fingerprintable")
	}

	prompts, err := hashRepoFiles(root, m.Prompts)
	if err != nil {
		return "", fmt.Errorf("method_version: %w", err)
	}
	tools := append([]string(nil), m.Tools...)
	sort.Strings(tools)

	h := sha256.New()
	// The literal domain tag keeps this digest from ever colliding with a
	// surface fingerprint computed over the same bytes.
	fmt.Fprintf(h, "dossierx-gate-method\x00v1\x00%s\x00%s\x00%d\x00%s",
		prompts, m.Model, len(tools), strings.Join(tools, "\x00"))
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ---------------------------------------------------------------------
// the fingerprint
// ---------------------------------------------------------------------

// gateSurfaceInputs is everything one surface agent reads.
type gateSurfaceInputs struct {
	// Surface is the name declared in surfaces.yaml.
	Surface string
	// Documents are the surface's own files, repo-relative. They come from
	// gateSurfaceDocuments, resolved from the manifest against `git ls-files`,
	// rather than from a hand list — a hand list is how a surface's fingerprint
	// silently stops covering a file the surface actually has.
	Documents []string
	Method    gateMethod
}

// gateSurfaceFingerprint is the digest of one agent's whole input set.
//
// It fails on ANY unreadable input rather than hashing what it could reach.
// A fingerprint over a narrowed input set is worse than no fingerprint: it is
// stable, it looks like a match, and it carries a verdict forward across a
// change to the file it stopped reading.
func gateSurfaceFingerprint(root string, in gateSurfaceInputs) (string, error) {
	if strings.TrimSpace(in.Surface) == "" {
		return "", errors.New("surface fingerprint: no surface name; the name is in the digest, so an unnamed surface would collide with every other unnamed one")
	}
	if len(in.Documents) == 0 {
		return "", fmt.Errorf("surface fingerprint: surface %q resolved to no documents; a surface with nothing to read is a surface the gate is not covering", in.Surface)
	}

	documents, err := hashRepoFiles(root, in.Documents)
	if err != nil {
		return "", fmt.Errorf("surface %q: %w", in.Surface, err)
	}
	evidence, err := hashRepoFiles(root, gateSharedEvidence())
	if err != nil {
		return "", fmt.Errorf("surface %q: %w", in.Surface, err)
	}
	method, err := in.Method.version(root)
	if err != nil {
		return "", fmt.Errorf("surface %q: %w", in.Surface, err)
	}

	h := sha256.New()
	fmt.Fprintf(h, "dossierx-gate-surface\x00v1\x00%s\x00%s\x00%s\x00%s",
		in.Surface, documents, evidence, method)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ---------------------------------------------------------------------
// verdicts, carry-forward, and greenness
// ---------------------------------------------------------------------

// gateSurfaceVerdict is one surface's answer and the fingerprint of the inputs
// that produced it. The fingerprint travels WITH the verdict, never beside it:
// a verdict whose provenance is stored somewhere else is a verdict that can be
// re-attached to a tree it was not computed over.
type gateSurfaceVerdict struct {
	Surface     string `json:"surface"`
	Verdict     string `json:"verdict"`
	Fingerprint string `json:"fingerprint"`
}

// gateRerunPlan is what a re-run has to do: the verdicts it may keep, and the
// surfaces it must pay for again.
type gateRerunPlan struct {
	Carried []gateSurfaceVerdict
	Rerun   []string
}

// errGateDuplicateVerdict is one surface holding more than one verdict.
//
// It is a sentinel, and it is the sharpest failure in this file, because the
// obvious way to index a verdict list — `held[v.Surface] = v` — resolves a
// duplicate by last-wins, and last-wins over {FAILED, PASS} converts the FAILED
// into a PASS. The losing verdict does not merely lose a comparison: it is gone,
// so nothing downstream carries any record that it was raised, and the run reads
// as a clean release. Which one survives is a function of slice ORDER alone,
// which is not a fact about the tree.
//
// That is the exact reading CLAUDE.md forbids — a result meaning "we did not
// check" presented as "it is fine" — arrived at by an ordering accident rather
// than by anyone's decision. So a duplicate is refused rather than resolved.
var errGateDuplicateVerdict = errors.New("a surface holds more than one verdict")

// gateIndexVerdicts indexes a verdict list by surface, REFUSING a second entry
// for a surface it has already seen.
//
// Every consumer of a verdict list goes through this rather than building its
// own map, because the refusal has to be unanimous: a duplicate that one caller
// rejects and another quietly resolves is a duplicate that still reaches the
// report, through whichever caller was the more forgiving.
//
// It refuses even when the two entries AGREE. The fan-out is one agent per
// declared surface, so a second verdict means the run's shape is not what the
// manifest describes — and this function cannot tell an accidental repetition
// from two agents that both judged the surface. Accepting the agreeing case
// would make the guard a function of what the duplicates happened to say, which
// is the same order-dependence one step removed.
func gateIndexVerdicts(verdicts []gateSurfaceVerdict) (map[string]gateSurfaceVerdict, error) {
	index := make(map[string]gateSurfaceVerdict, len(verdicts))
	duplicated := map[string]bool{}
	for _, v := range verdicts {
		if _, seen := index[v.Surface]; seen {
			duplicated[v.Surface] = true
			continue
		}
		index[v.Surface] = v
	}
	if len(duplicated) > 0 {
		names := make([]string, 0, len(duplicated))
		for name := range duplicated {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("%w: %s reported more than once. The gate cannot say which answer covers the surface, "+
			"and resolving it by position would let a FAILED verdict be overwritten by a PASS behind it and vanish from the report entirely",
			errGateDuplicateVerdict, strings.Join(names, ", "))
	}
	return index, nil
}

// gatePlanRerun decides, per surface, whether the previous run's verdict still
// applies to this tree.
//
// A verdict is carried forward only when the surface's fingerprint is
// byte-identical AND the previous answer was one of the two real verdicts.
// Everything else re-runs: an unknown surface, a moved fingerprint, and — the
// case worth naming — a previous entry whose verdict is empty or is some third
// string. An unrecognised verdict is not evidence of anything, and treating it
// as one would be the "we did not check" reading as "it is fine" that this whole
// gate is written against.
//
// current must cover every surface the run intends to judge; a previous entry
// for a surface no longer in current is simply dropped, which is what removing a
// surface from the manifest should do.
//
// A previous list holding two verdicts for one surface is refused outright — see
// errGateDuplicateVerdict. Carrying one of them forward would drop the other,
// and a dropped FAILED is how a re-run inherits a pass nobody gave it.
func gatePlanRerun(previous []gateSurfaceVerdict, current map[string]string) (gateRerunPlan, error) {
	if len(current) == 0 {
		return gateRerunPlan{}, errors.New("no current fingerprints; a plan over zero surfaces would report nothing to re-run, which is indistinguishable from a plan over a tree that did not move")
	}

	index, err := gateIndexVerdicts(previous)
	if err != nil {
		return gateRerunPlan{}, err
	}

	names := make([]string, 0, len(current))
	for name := range current {
		names = append(names, name)
	}
	sort.Strings(names)

	var plan gateRerunPlan
	for _, name := range names {
		fingerprint := current[name]
		if fingerprint == "" {
			return gateRerunPlan{}, fmt.Errorf("surface %q has no fingerprint for this tree; a missing fingerprint must not be matched against a missing one", name)
		}
		prev, ok := index[name]
		switch {
		case !ok, prev.Fingerprint != fingerprint:
			plan.Rerun = append(plan.Rerun, name)
		case prev.Verdict != gateVerdictPass && prev.Verdict != gateVerdictFailed:
			plan.Rerun = append(plan.Rerun, name)
		default:
			plan.Carried = append(plan.Carried, prev)
		}
	}
	return plan, nil
}

// gateIsGreen is the release-time question: does every DECLARED surface hold a
// PASS whose fingerprint matches this tree?
//
// It is written to name every reason at once rather than returning at the first,
// because the caller is a human reading a report and a gate that reveals its
// findings one re-run at a time is a gate people stop running.
//
// The six refusals are the six ways coverage narrows:
//
//	a declared surface with no verdict     — the gate did not look at it
//	a verdict that is not PASS             — it looked, and it found something
//	a PASS with no fingerprint for this    — it looked, but nothing attaches the
//	tree                                     answer to the tree being released
//	a PASS against a different fingerprint — it looked at a different tree
//	a verdict for an undeclared surface    — it looked at something the manifest
//	                                         does not list, so the manifest and
//	                                         the run disagree about the fan-out
//	a surface holding two verdicts         — it looked twice and disagreed, and
//	                                         indexing would silently keep one
//
// The third is the one that looks redundant against the fourth and is not, which
// is why it is spelled out here after a review found it undocumented and
// untested. "No fingerprint" and "the wrong fingerprint" differ in exactly the
// case where the mismatch comparison cannot fire: a verdict carrying an empty
// fingerprint — an omitted JSON key unmarshals to "" — against a surface absent
// from current, because its fingerprint failed to compute or it entered
// surfaces.yaml after the map was built. Then "" == "" and the mismatch branch
// falls through to nothing. Both sides blank is the shape, both sides blank is
// the danger, and it reports GREEN over a PASS attached to no tree at all.
//
// The last is the only one that returns EARLY rather than joining the list,
// which is a deliberate exception to the name-every-reason rule above. The other
// four are read off a surface's single verdict; with two, every one of them
// would have to judge an arbitrary member of the pair, and reporting confident
// answers derived from an arbitrary choice is worse than reporting the one thing
// that is actually known — that the run's shape is wrong.
func gateIsGreen(declared []string, verdicts []gateSurfaceVerdict, current map[string]string) error {
	if len(declared) == 0 {
		return errors.New("no surfaces are declared; a gate over zero surfaces is a pass over zero assertions")
	}

	held, err := gateIndexVerdicts(verdicts)
	if err != nil {
		return err
	}
	isDeclared := make(map[string]bool, len(declared))
	for _, name := range declared {
		isDeclared[name] = true
	}

	var problems []string
	for _, name := range declared {
		v, ok := held[name]
		fingerprint, computed := current[name]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("surface %q holds no verdict; the gate never reported on it", name))
		case v.Verdict != gateVerdictPass:
			problems = append(problems, fmt.Sprintf("surface %q holds %q, not %s", name, v.Verdict, gateVerdictPass))
		case !computed || fingerprint == "":
			problems = append(problems, fmt.Sprintf("surface %q holds a PASS but no fingerprint was computed for this tree, so the PASS cannot be attached to it", name))
		case v.Fingerprint != fingerprint:
			problems = append(problems, fmt.Sprintf("surface %q's PASS was recorded against fingerprint %s; this tree fingerprints as %s", name, v.Fingerprint, fingerprint))
		}
	}
	for _, v := range verdicts {
		if !isDeclared[v.Surface] {
			problems = append(problems, fmt.Sprintf("a verdict was reported for surface %q, which surfaces.yaml does not declare", v.Surface))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

// ---------------------------------------------------------------------
// the declared surfaces, read from the manifest
// ---------------------------------------------------------------------

// gateManifestFile is surfaces.yaml, relative to the repository root. The gate's
// fan-out is exactly as complete as that file, which is why the fan-out is READ
// from it rather than restated here: a hand list in the gate is the same defect
// as a hand-maintained method_version, one level up.
const gateManifestFile = "surfaces.yaml"

// gateManifest mirrors the shape tests/surfaces_manifest_test.go decodes. It is
// duplicated rather than shared because that test lives in package tests and Go
// forbids importing a test package — the same wall that makes surface_test.go a
// generator test. TestGateManifestPatternsAgreeWithTheTree below is what keeps
// the duplicate honest.
type gateManifest struct {
	Surfaces   []gateManifestEntry `yaml:"surfaces"`
	OutOfScope []gateManifestEntry `yaml:"out_of_scope"`
}

type gateManifestEntry struct {
	Name  string   `yaml:"name"`
	Paths []string `yaml:"paths"`
	Not   []string `yaml:"not"`
}

// gateLoadManifest reads surfaces.yaml. A manifest that cannot be read is an
// error and never an empty fan-out: with no manifest there is no coverage claim
// at all.
func gateLoadManifest(root string) (gateManifest, error) {
	raw, err := os.ReadFile(filepath.Join(root, gateManifestFile))
	if err != nil {
		return gateManifest{}, fmt.Errorf("read %s: %w", gateManifestFile, err)
	}
	var m gateManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return gateManifest{}, fmt.Errorf("parse %s: %w", gateManifestFile, err)
	}
	if len(m.Surfaces) == 0 {
		return gateManifest{}, fmt.Errorf("%s declares no surfaces; the gate's fan-out would be empty", gateManifestFile)
	}
	return m, nil
}

// gateDeclaredSurfaces is the fan-out: every surface name the manifest declares,
// sorted.
func gateDeclaredSurfaces(root string) ([]string, error) {
	m, err := gateLoadManifest(root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(m.Surfaces))
	for _, entry := range m.Surfaces {
		if entry.Name == "" {
			return nil, fmt.Errorf("%s declares a surface with no name", gateManifestFile)
		}
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names, nil
}

// gateSurfaceDocuments maps each declared surface to the tracked files it owns,
// resolved against `git ls-files`. That is the input set the fingerprint hashes,
// and resolving it from the manifest is what makes "a new file in an existing
// surface moves that surface's fingerprint" true with no second edit.
func gateSurfaceDocuments(root string, tracked []string) (map[string][]string, error) {
	m, err := gateLoadManifest(root)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(m.Surfaces))
	for _, entry := range m.Surfaces {
		var owned []string
		for _, file := range tracked {
			if gateEntryClaims(entry, file) {
				owned = append(owned, file)
			}
		}
		if len(owned) == 0 {
			return nil, fmt.Errorf("surface %q owns no tracked file; either its patterns went stale or the surface is gone, and both must be decided rather than fingerprinted over nothing", entry.Name)
		}
		sort.Strings(owned)
		out[entry.Name] = owned
	}
	return out, nil
}

// gateEntryClaims reports whether an entry owns file: it matches one of the
// entry's paths and none of its exceptions.
func gateEntryClaims(entry gateManifestEntry, file string) bool {
	matched := false
	for _, pattern := range entry.Paths {
		if gateMatchPattern(pattern, file) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	for _, pattern := range entry.Not {
		if gateMatchPattern(pattern, file) {
			return false
		}
	}
	return true
}

// gatePatternCache memoizes compiled patterns; the manifest holds a few dozen
// and the tree a few hundred files.
var gatePatternCache = map[string]*regexp.Regexp{}

// gateMatchPattern implements the grammar surfaces.yaml documents in its own
// header: a trailing "/" claims a whole subtree, "**/" spans any number of
// directory segments, "*" spans one segment, anything else is an exact path.
func gateMatchPattern(pattern, file string) bool {
	re, ok := gatePatternCache[pattern]
	if !ok {
		var b strings.Builder
		b.WriteString("^")
		rest := pattern
		for rest != "" {
			switch {
			case strings.HasPrefix(rest, "**/"):
				b.WriteString(`(?:[^/]+/)*`)
				rest = rest[3:]
			case strings.HasPrefix(rest, "*"):
				b.WriteString(`[^/]*`)
				rest = rest[1:]
			default:
				next := strings.IndexAny(rest, "*")
				if next < 0 {
					next = len(rest)
				}
				b.WriteString(regexp.QuoteMeta(rest[:next]))
				rest = rest[next:]
			}
		}
		if strings.HasSuffix(pattern, "/") {
			b.WriteString(".+")
		}
		b.WriteString("$")
		compiled, err := regexp.Compile(b.String())
		if err != nil {
			return false
		}
		gatePatternCache[pattern] = compiled
		re = compiled
	}
	return re.MatchString(file)
}

// ---------------------------------------------------------------------
// test fixtures
// ---------------------------------------------------------------------

// gateWrite writes one fixture file under root, creating parents.
func gateWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// gatePassingSurfaces is a set of PASS verdicts with distinct fingerprints, for
// the receipt tests next door that care about the receipt's arithmetic rather
// than about how a fingerprint is computed.
func gatePassingSurfaces(names ...string) []gateSurfaceVerdict {
	out := make([]gateSurfaceVerdict, 0, len(names))
	for _, name := range names {
		out = append(out, gateSurfaceVerdict{Surface: name, Verdict: gateVerdictPass, Fingerprint: "sha256:" + name})
	}
	return out
}

// gateFingerprintFixture writes a whole synthetic tree — the surface's document,
// the three shared evidence files, and a prompt — and returns the root and the
// inputs over it.
//
// It is synthetic rather than the real repository because the point of the tests
// below is to move ONE input at a time and watch the digest move. Two of the
// three evidence files do not exist in this tree yet (see gateDeltaFile), and a
// fixture is also the only way to assert the direction the hashing errs in when
// one of them is missing.
func gateFingerprintFixture(t *testing.T) (root string, in gateSurfaceInputs) {
	t.Helper()
	root = t.TempDir()
	gateWrite(t, root, "site/src/content.ts", "export const latestVersion = \"v9.9.9\";\n")
	gateWrite(t, root, gateSurfaceInventoryFile, "{\"counts\":{\"lint_rules\":28}}\n")
	gateWrite(t, root, gateDeltaFile, "{\"lint_rules\":{\"added\":[\"mixed-cycle\"]}}\n")
	gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX v9.9.9\"}\n")
	gateWrite(t, root, "gate/prompts/site.md", "Read the rendered site text against surface.json.\n")

	return root, gateSurfaceInputs{
		Surface:   "site",
		Documents: []string{"site/src/content.ts"},
		Method: gateMethod{
			Prompts: []string{"gate/prompts/site.md"},
			Model:   "claude-opus-5",
			Tools:   []string{"Bash", "Grep", "Read"},
		},
	}
}

// gateMustFingerprint is the fixture's happy path, for the tests that need a
// baseline to compare against.
func gateMustFingerprint(t *testing.T, root string, in gateSurfaceInputs) string {
	t.Helper()
	got, err := gateSurfaceFingerprint(root, in)
	if err != nil {
		t.Fatalf("fingerprint %s: %v", in.Surface, err)
	}
	return got
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// TestGateSurfaceFingerprintIsStableForAnUnchangedTree is the half that makes
// carry-forward possible at all: nothing in the digest may depend on the clock,
// the machine, or the order the inputs were listed in.
func TestGateSurfaceFingerprintIsStableForAnUnchangedTree(t *testing.T) {
	root, in := gateFingerprintFixture(t)
	first := gateMustFingerprint(t, root, in)

	shuffled := in
	shuffled.Method.Tools = []string{"Read", "Bash", "Grep"}
	if got := gateMustFingerprint(t, root, shuffled); got != first {
		t.Errorf("the tool list's ORDER moved the fingerprint; the set is what the method is, and a reordering would re-run every surface for nothing\n first: %s\nsecond: %s", first, got)
	}
	if got := gateMustFingerprint(t, root, in); got != first {
		t.Errorf("two runs over one unchanged tree disagree:\n first: %s\nsecond: %s", first, got)
	}
}

// TestGateSurfaceFingerprintMovesWhenAnyInputMoves is the other half, and it is
// the assertion that earns the whole design. Each row moves exactly one input;
// a row that did not move the digest would be an input the gate stops noticing,
// which is a stale verdict shipped as a fresh one.
func TestGateSurfaceFingerprintMovesWhenAnyInputMoves(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, root string, in *gateSurfaceInputs)
	}{
		{"the surface's own document", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, "site/src/content.ts", "export const latestVersion = \"v9.9.10\";\n")
		}},
		{"a file joins the surface", func(t *testing.T, root string, in *gateSurfaceInputs) {
			gateWrite(t, root, "site/src/new.ts", "export const extra = 1;\n")
			in.Documents = append(in.Documents, "site/src/new.ts")
		}},
		{"surface.json", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, gateSurfaceInventoryFile, "{\"counts\":{\"lint_rules\":29}}\n")
		}},
		{"the release delta", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, gateDeltaFile, "{\"lint_rules\":{\"added\":[]}}\n")
		}},
		{"the rendered site text", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX v9.9.8\"}\n")
		}},
		{"the prompt's wording", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, "gate/prompts/site.md", "Read the rendered site text against surface.json, and the delta.\n")
		}},
		{"the model id", func(t *testing.T, _ string, in *gateSurfaceInputs) {
			in.Method.Model = "claude-opus-5-mini"
		}},
		{"a tool joins the list", func(t *testing.T, _ string, in *gateSurfaceInputs) {
			in.Method.Tools = append(in.Method.Tools, "WebFetch")
		}},
		{"the surface's name", func(t *testing.T, _ string, in *gateSurfaceInputs) {
			in.Surface = "readme"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, in := gateFingerprintFixture(t)
			before := gateMustFingerprint(t, root, in)
			tc.mutate(t, root, &in)
			after := gateMustFingerprint(t, root, in)
			if after == before {
				t.Fatalf("moving %s did not move the fingerprint (%s); a verdict computed before the change would be carried forward across it", tc.name, before)
			}
		})
	}
}

// TestGateFingerprintRefusesEveryEmptyInput pins the direction this file errs
// in. Each case below would otherwise produce a perfectly stable digest over an
// input set that is missing something, and stability is exactly what makes a
// fingerprint dangerous when it is wrong.
func TestGateFingerprintRefusesEveryEmptyInput(t *testing.T) {
	t.Run("a missing evidence file", func(t *testing.T) {
		for _, rel := range gateSharedEvidence() {
			root, in := gateFingerprintFixture(t)
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
				t.Fatalf("remove %s: %v", rel, err)
			}
			if _, err := gateSurfaceFingerprint(root, in); err == nil {
				t.Errorf("a fingerprint was produced with %s absent", rel)
			}
		}
	})

	t.Run("a missing document", func(t *testing.T) {
		root, in := gateFingerprintFixture(t)
		if err := os.Remove(filepath.Join(root, "site", "src", "content.ts")); err != nil {
			t.Fatalf("remove the document: %v", err)
		}
		if _, err := gateSurfaceFingerprint(root, in); err == nil {
			t.Error("a fingerprint was produced with the surface's own document absent")
		}
	})

	t.Run("a missing prompt", func(t *testing.T) {
		root, in := gateFingerprintFixture(t)
		if err := os.Remove(filepath.Join(root, "gate", "prompts", "site.md")); err != nil {
			t.Fatalf("remove the prompt: %v", err)
		}
		if _, err := gateSurfaceFingerprint(root, in); err == nil {
			t.Error("a fingerprint was produced with the method's prompt absent")
		}
	})

	t.Run("an empty method", func(t *testing.T) {
		root, in := gateFingerprintFixture(t)
		for _, tc := range []struct {
			name   string
			method gateMethod
		}{
			{"no prompts", gateMethod{Model: in.Method.Model, Tools: in.Method.Tools}},
			{"no model", gateMethod{Prompts: in.Method.Prompts, Tools: in.Method.Tools}},
			{"no tools", gateMethod{Prompts: in.Method.Prompts, Model: in.Method.Model}},
		} {
			if _, err := tc.method.version(root); err == nil {
				t.Errorf("method_version was computed with %s", tc.name)
			}
		}
	})

	t.Run("an empty surface", func(t *testing.T) {
		root, in := gateFingerprintFixture(t)
		nameless := in
		nameless.Surface = ""
		if _, err := gateSurfaceFingerprint(root, nameless); err == nil {
			t.Error("a fingerprint was produced for a surface with no name")
		}
		empty := in
		empty.Documents = nil
		if _, err := gateSurfaceFingerprint(root, empty); err == nil {
			t.Error("a fingerprint was produced for a surface that owns no document")
		}
	})
}

// TestGateMethodVersionIsNotASurfaceFingerprint: the two digests are computed
// over overlapping bytes and must never collide, or a method change could be
// mistaken for a document change and vice versa.
func TestGateMethodVersionIsNotASurfaceFingerprint(t *testing.T) {
	root, in := gateFingerprintFixture(t)
	method, err := in.Method.version(root)
	if err != nil {
		t.Fatalf("method version: %v", err)
	}
	if method == gateMustFingerprint(t, root, in) {
		t.Fatal("the method version and the surface fingerprint are the same digest")
	}
}

// TestGateRerunPlanReRunsOnlyWhatMoved is the saving, and its one hard edge:
// a verdict is carried forward on a byte-identical fingerprint and on nothing
// else.
func TestGateRerunPlanReRunsOnlyWhatMoved(t *testing.T) {
	previous := []gateSurfaceVerdict{
		{Surface: "readme", Verdict: gateVerdictPass, Fingerprint: "sha256:aa"},
		{Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"},
		{Surface: "skills", Verdict: gateVerdictFailed, Fingerprint: "sha256:cc"},
	}
	current := map[string]string{
		"readme":    "sha256:aa",     // unmoved
		"site":      "sha256:bb-new", // moved
		"skills":    "sha256:cc",     // unmoved, and its FAIL still stands
		"changelog": "sha256:dd",     // never judged
	}

	plan, err := gatePlanRerun(previous, current)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if got, want := plan.Rerun, []string{"changelog", "site"}; !gateEqualStrings(got, want) {
		t.Errorf("re-run set: got %v, want %v", got, want)
	}
	carried := make([]string, 0, len(plan.Carried))
	for _, v := range plan.Carried {
		carried = append(carried, v.Surface+"="+v.Verdict)
	}
	if want := []string{"readme=" + gateVerdictPass, "skills=" + gateVerdictFailed}; !gateEqualStrings(carried, want) {
		t.Errorf("carried set: got %v, want %v", carried, want)
	}
}

// TestGateRerunPlanNeverCarriesAnUnrecognisedVerdict: an entry whose verdict is
// empty, or is some third string, is not evidence. It re-runs even though its
// fingerprint matches.
func TestGateRerunPlanNeverCarriesAnUnrecognisedVerdict(t *testing.T) {
	current := map[string]string{"site": "sha256:bb"}
	for _, verdict := range []string{"", "SKIPPED", "pass", "UNKNOWN"} {
		plan, err := gatePlanRerun([]gateSurfaceVerdict{{Surface: "site", Verdict: verdict, Fingerprint: "sha256:bb"}}, current)
		if err != nil {
			t.Fatalf("plan for verdict %q: %v", verdict, err)
		}
		if len(plan.Carried) != 0 || !gateEqualStrings(plan.Rerun, []string{"site"}) {
			t.Errorf("verdict %q was carried forward; only %s and %s are verdicts", verdict, gateVerdictPass, gateVerdictFailed)
		}
	}

	if _, err := gatePlanRerun(nil, nil); err == nil {
		t.Error("a plan was produced over zero current fingerprints")
	}
	if _, err := gatePlanRerun(nil, map[string]string{"site": ""}); err == nil {
		t.Error("a plan was produced for a surface with no fingerprint for this tree")
	}
}

// TestGateIsGreenNamesEveryWayCoverageNarrows walks four of the six refusals.
// Each row is a shape that would otherwise read as a clean release.
//
// The other two are next door, because neither fits a table that holds `current`
// fixed: TestGateIsGreenRefusesAPassAttachedToNoTree varies the fingerprint map,
// and TestGateRefusesASurfaceReportedTwice varies the list's shape. The
// zero-declared-surfaces guard is next door as well, in
// TestGateRefusesAFanOutItCannotDerive — asserting it from here is what a review
// found could not distinguish it from the undeclared-surface path.
func TestGateIsGreenNamesEveryWayCoverageNarrows(t *testing.T) {
	declared := []string{"readme", "site"}
	current := map[string]string{"readme": "sha256:aa", "site": "sha256:bb"}
	green := []gateSurfaceVerdict{
		{Surface: "readme", Verdict: gateVerdictPass, Fingerprint: "sha256:aa"},
		{Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"},
	}

	if err := gateIsGreen(declared, green, current); err != nil {
		t.Fatalf("a fully covered, fully passing, fully current run was refused: %v", err)
	}

	for _, tc := range []struct {
		name     string
		verdicts []gateSurfaceVerdict
		want     string
	}{
		{"a declared surface holds no verdict", green[:1], "holds no verdict"},
		{"a surface holds a FAIL", []gateSurfaceVerdict{green[0], {Surface: "site", Verdict: gateVerdictFailed, Fingerprint: "sha256:bb"}}, "not PASS"},
		{"a PASS is against another tree", []gateSurfaceVerdict{green[0], {Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:old"}}, "this tree fingerprints as"},
		{"a verdict names an undeclared surface", append(append([]gateSurfaceVerdict(nil), green...), gateSurfaceVerdict{Surface: "invented", Verdict: gateVerdictPass, Fingerprint: "sha256:zz"}), "does not declare"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := gateIsGreen(declared, tc.verdicts, current)
			if err == nil {
				t.Fatal("the gate reported green")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal must say why; want a mention of %q, got:\n%s", tc.want, err)
			}
		})
	}

}

// TestGateRefusesAFanOutItCannotDerive pins the three guards on the road from
// surfaces.yaml to a verdict, each of which admits the same thing when deleted:
// a gate that fans out over nothing, or over something with no name, and reports
// green. "A pass over zero assertions" is CLAUDE.md's phrase for it, and these
// are the three places the gate can reach that state without noticing.
//
// ALL THREE WERE MUTUALLY REDUNDANT WITH SOMETHING ELSE, which is why they get a
// test of their own. The zero-declared guard in gateIsGreen was asserted with a
// full verdict list in hand, so deleting it still produced an error — from the
// undeclared-surface loop, which finds every one of those verdicts unaccounted
// for. The two manifest guards were covered only through callers that fail for
// their own reasons on the same fixture. In each case the assertion reported a
// working guard over a deleted one, and a deleted guard fires on the day it is
// needed and not before.
//
// So each row here is chosen to leave the deleted guard as the ONLY thing that
// can produce the error, and each is asserted directly against the function that
// holds it rather than through a caller.
func TestGateRefusesAFanOutItCannotDerive(t *testing.T) {
	t.Run("a gate over zero declared surfaces", func(t *testing.T) {
		// Nothing at all: no declared surfaces, no verdicts, no fingerprints.
		// This is the shape that kills the mutant — with no verdicts there is
		// nothing for the undeclared-surface loop to object to, so a gateIsGreen
		// that has lost its guard walks two empty loops, collects no problems and
		// returns nil. It would be reporting that a fan-out which examined
		// nothing found nothing wrong.
		err := gateIsGreen(nil, nil, nil)
		if err == nil {
			t.Fatal("a gate over zero declared surfaces, holding zero verdicts, reported green; that is a pass over zero assertions and it is the one result CLAUDE.md forbids")
		}
		if !strings.Contains(err.Error(), "no surfaces are declared") {
			t.Errorf("the refusal must say the fan-out was empty; got:\n%s", err)
		}

		// And with a full, passing, current verdict list in hand the answer must
		// still be the empty fan-out rather than thirteen complaints about
		// verdicts for surfaces nobody declared. The two accusations send a
		// reader to opposite ends: one to surfaces.yaml, one to the run's shape.
		current := map[string]string{"readme": "sha256:readme"}
		err = gateIsGreen(nil, gatePassingSurfaces("readme"), current)
		if err == nil {
			t.Fatal("a gate over zero declared surfaces reported green")
		}
		if !strings.Contains(err.Error(), "no surfaces are declared") {
			t.Errorf("an empty fan-out was reported as verdicts for undeclared surfaces, which sends the reader to the run's shape when the manifest is what is empty; got:\n%s", err)
		}

		// The fixture is honest: declare the surface and the same inputs are
		// green, so the rows above are the guard and not a refusal of everything.
		if err := gateIsGreen([]string{"readme"}, gatePassingSurfaces("readme"), current); err != nil {
			t.Fatalf("a fully covered, fully passing, fully current one-surface gate was refused, so the rows above prove nothing: %v", err)
		}
	})

	t.Run("a manifest declaring no surfaces", func(t *testing.T) {
		// A manifest that is all exclusions is the realistic shape: every entry
		// moved to out_of_scope one at a time, and the last move empties the
		// fan-out with the file still parsing perfectly. Without the guard this
		// loads clean and gateDeclaredSurfaces returns an empty list, which
		// gateIsGreen would then have to be the one to catch.
		root := t.TempDir()
		gateWrite(t, root, gateManifestFile, "out_of_scope:\n"+
			"  - name: tests\n"+
			"    paths: [\"**/*_test.go\"]\n"+
			"    reason: tests are not a client-facing surface\n")

		if _, err := gateLoadManifest(root); err == nil {
			t.Fatal("a manifest declaring no surfaces loaded clean; the gate's fan-out would be empty and every later check would be judging an empty set")
		} else if !strings.Contains(err.Error(), "declares no surfaces") {
			t.Errorf("the refusal must say the manifest declares no surfaces; got:\n%s", err)
		}

		// The same file with an explicit empty list, which is the other spelling
		// and unmarshals to the same nil slice.
		gateWrite(t, root, gateManifestFile, "surfaces: []\n")
		if _, err := gateLoadManifest(root); err == nil {
			t.Fatal("a manifest whose `surfaces` list is explicitly empty loaded clean")
		}
		if _, err := gateDeclaredSurfaces(root); err == nil {
			t.Fatal("the fan-out was derived from a manifest that declares no surfaces")
		}

		// Honest fixture.
		gateWrite(t, root, gateManifestFile, "surfaces:\n"+
			"  - name: docs\n"+
			"    paths: [docs/]\n")
		if _, err := gateLoadManifest(root); err != nil {
			t.Fatalf("a manifest declaring one surface was refused, so the rows above prove nothing: %v", err)
		}
	})

	t.Run("a surface with no name", func(t *testing.T) {
		// The name is what a verdict is filed under and what the fingerprint is
		// domain-separated by, so a nameless surface is one every verdict for a
		// nameless surface collides with. Without the guard gateDeclaredSurfaces
		// returns "" as a perfectly ordinary member of the fan-out, and it sorts
		// first.
		root := t.TempDir()
		gateWrite(t, root, gateManifestFile, "surfaces:\n"+
			"  - name: docs\n"+
			"    paths: [docs/]\n"+
			"  - paths: [site/]\n")

		names, err := gateDeclaredSurfaces(root)
		if err == nil {
			t.Fatalf("a surface with no name entered the fan-out as %q; its verdict would be filed under the empty string, where every other nameless surface files theirs", names)
		}
		if !strings.Contains(err.Error(), "no name") {
			t.Errorf("the refusal must say the surface has no name; got:\n%s", err)
		}

		// Honest fixture: name it and both surfaces are declared, so the row
		// above is the guard rather than a manifest the reader cannot parse.
		gateWrite(t, root, gateManifestFile, "surfaces:\n"+
			"  - name: docs\n"+
			"    paths: [docs/]\n"+
			"  - name: site\n"+
			"    paths: [site/]\n")
		names, err = gateDeclaredSurfaces(root)
		if err != nil {
			t.Fatalf("a manifest whose surfaces are all named was refused: %v", err)
		}
		if !gateEqualStrings(names, []string{"docs", "site"}) {
			t.Errorf("the fan-out is %v, want [docs site]", names)
		}
	})
}

// TestGateIsGreenRefusesAPassAttachedToNoTree is the fifth refusal, split out
// because it is the one the table above cannot reach and the one a review found
// pinned by an assertion that went red for the wrong reason.
//
// The old assertion handed gateIsGreen a verdict carrying "sha256:bb" against a
// `current` map with no entry for that surface, and required an error. It got
// one — but not from the guard it named. Delete the guard and the very next
// branch still fires, because "sha256:bb" != "" is a perfectly good mismatch. So
// the assertion reported a working guard over a deleted one, which is the exact
// thing a test is supposed to make impossible.
//
// The shape that kills the mutant is BOTH SIDES BLANK. A verdict whose
// fingerprint is "" — which is what an omitted `fingerprint` key unmarshals to —
// against a surface `current` has no entry for, which is what a fingerprint that
// failed to compute or a surface added to surfaces.yaml after the map was built
// looks like. Then the mismatch comparison is "" == "" and finds nothing to say,
// and without the guard gateIsGreen reports GREEN over a PASS that is attached to
// no tree whatsoever: the release-time question is "does every surface hold a
// PASS whose fingerprint matches the tree being released", and this is a PASS
// with no fingerprint on either side of the match.
//
// THE FIXTURE DID NOT HAVE THAT SHAPE, and the paragraph above said it did. Both
// rows shared one `current` map that OMITTED `site` entirely, so both reached the
// guard through `!computed` and the second disjunct — `fingerprint == ""` — was
// never the reason either of them failed. A review proved it: deleting that
// disjunct, leaving `case !computed:`, left the whole package green. The
// paragraph was describing an assertion nobody had written, in the file whose
// subject is guards that certify themselves.
//
// So `current` is now per-row, and the third row is the one the prose has been
// claiming all along: an entry that EXISTS and is blank, against a verdict whose
// fingerprint is blank. That state is not hypothetical — gatePlanRerun next door
// refuses it explicitly ("a missing fingerprint must not be matched against a
// missing one"), so the same author already judged a blank entry in `current`
// real enough to refuse one function over. Here, without the disjunct, the same
// pair reports GREEN.
//
// All three rows are asserted, and they are asserted to give DIFFERENT messages.
// The blank rows are the mutation-killers; the first is there because "no
// fingerprint was computed" and "the fingerprint is for another tree" send a
// reader to two different places — one to the gate's own fingerprinting, one to
// the diff — and a guard that reported the wrong one would cost a cycle at the
// same moment the receipt's misdirections do.
func TestGateIsGreenRefusesAPassAttachedToNoTree(t *testing.T) {
	declared := []string{"readme", "site"}
	readme := gateSurfaceVerdict{Surface: "readme", Verdict: gateVerdictPass, Fingerprint: "sha256:aa"}
	// `absent` covers readme and nothing else: no fingerprint was computed for
	// `site` on this tree at all.
	absent := map[string]string{"readme": "sha256:aa"}
	// `blank` covers it and has nothing to say about it. This is the row the
	// doc comment describes and the only one that reaches the second disjunct:
	// with `blank`, `computed` is true and the guard rests entirely on the
	// fingerprint being empty.
	blank := map[string]string{"readme": "sha256:aa", "site": ""}

	for _, tc := range []struct {
		name    string
		verdict gateSurfaceVerdict
		current map[string]string
	}{
		{"the verdict carries a fingerprint the tree cannot confirm", gateSurfaceVerdict{Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"}, absent},
		{"the tree computed nothing for it and neither side carries one", gateSurfaceVerdict{Surface: "site", Verdict: gateVerdictPass, Fingerprint: ""}, absent},
		{"the tree computed a blank for it and neither side carries one", gateSurfaceVerdict{Surface: "site", Verdict: gateVerdictPass, Fingerprint: ""}, blank},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verdicts := []gateSurfaceVerdict{readme, tc.verdict}
			current := tc.current

			err := gateIsGreen(declared, verdicts, current)
			if err == nil {
				t.Fatal("a PASS for a surface this tree produced no fingerprint for reported green; the PASS is attached to nothing, which is \"we did not check\" reading as \"it is fine\"")
			}
			if !strings.Contains(err.Error(), "no fingerprint was computed") {
				t.Errorf("the refusal must say no fingerprint was computed for this tree, which sends the reader to the gate's own fingerprinting rather than to the diff; got:\n%s", err)
			}
			if strings.Contains(err.Error(), "this tree fingerprints as") {
				t.Errorf("a surface with no fingerprint at all was reported as a fingerprint MISMATCH; there is nothing for it to mismatch against, and the message sends the reader to compare two values one of which does not exist; got:\n%s", err)
			}
			// And through the receipt, because that is the path a release
			// verdict actually takes.
			if got, err := (gateReceipt{Surfaces: verdicts}).evaluate(declared, current); got != gateVerdictFailed || err == nil {
				t.Errorf("a receipt holding a PASS with no fingerprint for this tree evaluated %s (%v)", got, err)
			}
		})
	}

	// The fixture is honest: computing a fingerprint for `site` and matching it
	// is green, so the rows above are the guard rather than a `current` map that
	// refuses everything.
	full := map[string]string{"readme": "sha256:aa", "site": "sha256:bb"}
	if err := gateIsGreen(declared, []gateSurfaceVerdict{readme, {Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"}}, full); err != nil {
		t.Fatalf("a fully fingerprinted, fully passing run was refused, so the rows above prove nothing: %v", err)
	}
}

// TestGateRefusesASurfaceReportedTwice is the ordering accident that would
// otherwise turn a FAILED verdict into a clean release.
//
// The fixture is the minimal shape: `site` reported both FAILED and PASS. Under
// a last-wins index the answer is whichever entry came second, so the SAME three
// verdicts read GREEN in one order and FAILED in the other — the release verdict
// as a function of slice position. Worse than the wrong answer is the missing
// one: the losing entry is not outvoted, it is absent, so the report downstream
// shows no sign a FAILED verdict was ever raised.
//
// Every reader of a verdict list is asserted, in both orders, because a refusal
// only some of them make is a duplicate that still reaches the report through
// the ones that do not.
//
// TestGateFanOutComesFromTheManifest already forbids a surface DECLARED twice.
// This is the other half, and the half that actually carries a verdict: the
// manifest can be immaculate and the run still report on one surface twice.
func TestGateRefusesASurfaceReportedTwice(t *testing.T) {
	declared := []string{"readme", "site"}
	current := map[string]string{"readme": "sha256:aa", "site": "sha256:bb"}
	readme := gateSurfaceVerdict{Surface: "readme", Verdict: gateVerdictPass, Fingerprint: "sha256:aa"}
	failed := gateSurfaceVerdict{Surface: "site", Verdict: gateVerdictFailed, Fingerprint: "sha256:bb"}
	passed := gateSurfaceVerdict{Surface: "site", Verdict: gateVerdictPass, Fingerprint: "sha256:bb"}

	for _, tc := range []struct {
		name     string
		verdicts []gateSurfaceVerdict
	}{
		{"the PASS sits behind the FAILED, and would overwrite it", []gateSurfaceVerdict{readme, failed, passed}},
		{"the same three in the other order", []gateSurfaceVerdict{readme, passed, failed}},
		{"and two entries that agree", []gateSurfaceVerdict{readme, passed, passed}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := gateIsGreen(declared, tc.verdicts, current); !errors.Is(err, errGateDuplicateVerdict) {
				t.Errorf("gateIsGreen resolved a surface reported twice by position instead of refusing it; got %v", err)
			}
			verdict, err := gateReceipt{Surfaces: tc.verdicts}.evaluate(declared, current)
			if verdict != gateVerdictFailed || err == nil {
				t.Errorf("a receipt holding two verdicts for one surface evaluated %s (%v)", verdict, err)
			}
			if _, err := gatePlanRerun(tc.verdicts, current); !errors.Is(err, errGateDuplicateVerdict) {
				t.Errorf("gatePlanRerun carried one of two verdicts forward and dropped the other; got %v", err)
			}
		})
	}

	// The refusal names the surface, because a report that says only "a surface
	// was reported twice" over a thirteen-way fan-out sends the reader back to
	// diff the verdict list by hand.
	err := gateIsGreen(declared, []gateSurfaceVerdict{readme, failed, passed}, current)
	if err == nil || !strings.Contains(err.Error(), "site") {
		t.Errorf("the refusal must name the duplicated surface; got %v", err)
	}
	if strings.Contains(fmt.Sprint(err), "readme") {
		t.Errorf("the refusal named a surface that was reported once; got:\n%s", err)
	}
}

// TestGateFanOutComesFromTheManifest reads the REAL surfaces.yaml. The fan-out
// and each surface's document set are derived from it, so a surface added to the
// manifest is one the gate must judge on the day it is added, with no second
// edit anywhere.
func TestGateFanOutComesFromTheManifest(t *testing.T) {
	root := surfaceRepoRoot(t)

	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("read the declared surfaces: %v", err)
	}
	if len(declared) == 0 {
		t.Fatal("the manifest declares no surfaces; the fan-out would be empty")
	}
	seen := map[string]bool{}
	for _, name := range declared {
		if seen[name] {
			t.Errorf("surface %q is declared twice; its verdict would overwrite itself", name)
		}
		seen[name] = true
	}

	documents, err := gateSurfaceDocuments(root, surfaceTrackedFiles(t, root))
	if err != nil {
		t.Fatalf("resolve the surfaces' documents: %v", err)
	}
	for _, name := range declared {
		if len(documents[name]) == 0 {
			t.Errorf("surface %q resolved to no tracked document", name)
		}
	}
	if len(documents) != len(declared) {
		t.Errorf("%d surfaces are declared but %d resolved a document set", len(declared), len(documents))
	}

	// NO SURFACE MAY OWN A TEST FILE. surfaces.yaml puts every `**/*_test.go`
	// out of scope, and binary-and-viewer carves the same pattern out of
	// cmd/ and internal/ by hand — so a surface holding one means this file's
	// reading of the grammar has narrowed.
	//
	// It is asserted separately from the exactly-one-owner cross-check below
	// because that check cannot see this class of failure: a narrowing applies
	// to an entry's `paths` and its `not` alike, so a test file dropping out of
	// the out-of-scope entry and INTO the surface that excluded it leaves the
	// owner count at exactly one. The two errors cancel; this one does not.
	//
	// It also matters on its own terms. The prose agents read a surface's
	// documents, and a fingerprint that included test files would churn on every
	// test edit — re-running an expensive agent over a document nobody touched.
	for _, name := range declared {
		for _, file := range documents[name] {
			if strings.HasSuffix(file, "_test.go") {
				t.Errorf("surface %q owns %s; tests are declared out of scope, and a surface that holds one is a surface whose patterns have been read too narrowly", name, file)
			}
		}
	}
}

// TestGateSurfaceDocumentsRefusesASurfaceThatOwnsNothing pins a guard the real
// manifest cannot exercise.
//
// Every surface in surfaces.yaml owns tracked files today, so
// TestGateFanOutComesFromTheManifest's per-surface document check passes over a
// condition it never meets: delete the refusal inside gateSurfaceDocuments and
// that test stays green. It would go red only on the day a surface's patterns
// actually went stale — which is the day the guard is supposed to fire, and much
// too late to discover it was never wired up. A synthetic manifest asks the
// question now, while the answer is cheap.
//
// What the guard prevents is specific. A surface resolving to no document does
// not fail loudly downstream; it fingerprints over the shared evidence and the
// method alone, which is a perfectly stable digest that is IDENTICAL for every
// surface in that state and that never moves when the files the surface was
// meant to cover are edited. That is a verdict carried forward forever over a
// surface nobody is reading.
func TestGateSurfaceDocumentsRefusesASurfaceThatOwnsNothing(t *testing.T) {
	root := t.TempDir()
	gateWrite(t, root, gateManifestFile, "surfaces:\n"+
		"  - name: real\n"+
		"    paths: [docs/]\n"+
		"  - name: stale\n"+
		"    paths: [docs/moved-away/]\n")
	tracked := []string{"docs/RELEASING.md"}

	_, err := gateSurfaceDocuments(root, tracked)
	if err == nil {
		t.Fatal("a surface owning no tracked file resolved to an empty document set instead of being refused")
	}
	// The refusal has to name the surface: over a thirteen-way fan-out, "a
	// surface owns nothing" sends the reader back to re-run the resolution by
	// hand against every entry in the manifest.
	if !strings.Contains(err.Error(), "stale") {
		t.Errorf("the refusal must name the surface whose patterns went stale; got:\n%s", err)
	}
	if strings.Contains(err.Error(), `"real"`) {
		t.Errorf("the refusal named a surface that did resolve a document; got:\n%s", err)
	}

	// And the fixture is honest: the same manifest without the stale entry
	// resolves, so the row above is the guard rather than a broken fixture.
	gateWrite(t, root, gateManifestFile, "surfaces:\n"+
		"  - name: real\n"+
		"    paths: [docs/]\n")
	documents, err := gateSurfaceDocuments(root, tracked)
	if err != nil {
		t.Fatalf("a surface that owns a tracked file was refused: %v", err)
	}
	if !gateEqualStrings(documents["real"], tracked) {
		t.Errorf("surface \"real\" resolved to %v, want %v", documents["real"], tracked)
	}
}

// TestGateManifestPatternsAgreeWithTheTree is what keeps this file's copy of the
// pattern grammar honest.
//
// tests/surfaces_manifest_test.go owns the same grammar and asserts the same
// invariant — every tracked file claimed by EXACTLY ONE entry — and the two
// implementations cannot import each other (package tests is a test package).
// So the duplicate is held to the invariant rather than to the original's code:
// if this copy read a pattern more narrowly, files would fall out and this test
// would name them.
func TestGateManifestPatternsAgreeWithTheTree(t *testing.T) {
	root := surfaceRepoRoot(t)
	manifest, err := gateLoadManifest(root)
	if err != nil {
		t.Fatalf("load the manifest: %v", err)
	}
	entries := append(append([]gateManifestEntry(nil), manifest.Surfaces...), manifest.OutOfScope...)
	if len(manifest.OutOfScope) == 0 {
		t.Fatal("the manifest declares no exclusions; this cross-check would only see half the grammar")
	}

	var unclaimed, contested []string
	for _, file := range surfaceTrackedFiles(t, root) {
		var owners []string
		for _, entry := range entries {
			if gateEntryClaims(entry, file) {
				owners = append(owners, entry.Name)
			}
		}
		switch len(owners) {
		case 0:
			unclaimed = append(unclaimed, file)
		case 1:
		default:
			contested = append(contested, fmt.Sprintf("%s (claimed by %s)", file, strings.Join(owners, ", ")))
		}
	}
	if len(unclaimed) > 0 {
		t.Errorf("this file's reading of the manifest leaves %d tracked file(s) unclaimed, so their surface's fingerprint would not cover them:\n  %s",
			len(unclaimed), strings.Join(unclaimed, "\n  "))
	}
	if len(contested) > 0 {
		t.Errorf("this file's reading of the manifest gives %d tracked file(s) more than one owner:\n  %s",
			len(contested), strings.Join(contested, "\n  "))
	}
}

// gateEqualStrings compares two string slices element by element. A nil and an
// empty slice are equal — the plans above build with append, which yields nil
// when nothing was added.
func gateEqualStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
