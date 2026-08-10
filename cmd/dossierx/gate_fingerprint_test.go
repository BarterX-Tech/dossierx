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
	"io"
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
// gateBaselineFile is the RESOLVED baseline inventory — the previous release's
// surface.json, written into the run's own evidence directory by the producer
// that resolved it (scripts/gate-stage2/run.sh delta).
//
// It is in the key because gate/delta.json is a LOSSY READ of a pair of
// inventories, and a projection never stands in for its source. This tree's half
// of that pair is already here as surface.json; without the other half a key
// carrying the delta is a key that trusts a summary of bytes it never saw. And
// the thing that belongs here is the baseline's BYTES, never the baseline tag's
// NAME: a tag is a mutable pointer (`git tag -f` re-points an annotated tag
// under anything that names only the tag), so "v0.5.0" hashes identically before
// and after it is made to mean a different commit.
const (
	gateSurfaceInventoryFile = "surface.json"
	gateBaselineFile         = "gate/baseline.json"
	gateDeltaFile            = "gate/delta.json"
	gateSiteTextFile         = "gate/site-text.json"
)

// gateSharedEvidence is those four, in one place, so a fifth evidence file is
// added once and every surface's fingerprint moves.
//
// SHARED means read by EVERY surface agent. An artifact only one agent reads —
// the skills export capture, the release-notes prediction, the cross-release
// render diff — does not belong here: folding it in would move all thirteen keys
// whenever any one of them moved, and the whole value of the key is that a
// one-document fix re-runs one agent. Those reach the key through the BUNDLE
// instead: the assembler hands the surface its capture verbatim, so the bundle
// digest covers those bytes for the one surface that reads them and for no
// other. TestGateStage2ACaptureReachesOneSurfaceKeyAndNoOther is what holds that
// true on the run path.
func gateSharedEvidence() []string {
	return []string{gateSurfaceInventoryFile, gateBaselineFile, gateDeltaFile, gateSiteTextFile}
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
// hashing a surface's documents
// ---------------------------------------------------------------------

// gateHashDocuments is hashRepoFiles EXTENDED to the one document shape this
// repository actually has and that function cannot read: a tracked symlink.
//
// WHY IT IS NOT A SECOND HASH FUNCTION. hashRepoFiles (cmd/dossierx/surface_test.go)
// is surface.json's own hash stream and stays that; this reproduces it BYTE FOR
// BYTE for every regular file and only adds records for shapes it refuses
// outright. TestGateDocumentHashAgreesWithTheOneHashFunction holds the two to
// that equality, the same way TestGateManifestPatternsAgreeWithTheTree holds
// this file's copy of the pattern grammar to the manifest test's. What must not
// exist is a second ANSWER to "what are these bytes" — not a second caller.
//
// WHY IT HAD TO EXIST AT ALL. surfaces.yaml's `exported-skills` surface is five
// tracked entries under .claude/skills/, and all five are symlinks to
// directories (`git ls-files -s` shows mode 120000). os.ReadFile on one returns
// "is a directory", so the landed key could not be computed for that surface at
// all — thirteen surfaces declared, twelve keys computable, and the repair
// anyone reaches for under time pressure is "skip a document that will not
// open", which leaves the run green over twelve surfaces while the manifest
// declares thirteen. Nothing here skips anything: an entry it cannot read is an
// error, and an entry of a shape it does not understand is an error.
//
// A symlink contributes BOTH its target string and the bytes reachable through
// it, because both can change independently and each is a different release
// defect. Re-pointing .claude/skills/dossierx at another bundle changes what
// this repository's agents read while every byte under skills/ stays put;
// editing skills/dossierx/SKILL.md changes what they read with the link
// untouched. A key that covered only one of the two would carry a verdict
// forward across the other.
func gateHashDocuments(root string, rels []string) (string, error) {
	sorted := append([]string(nil), rels...)
	sort.Strings(sorted)

	h := sha256.New()
	for _, rel := range sorted {
		if err := gateHashOneDocument(h, root, rel); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// gateHashOneDocument writes one document's records into h.
//
// The regular-file record is `rel\0len\0data`, which is hashRepoFiles' record
// exactly. The symlink record is `rel\0symlink\0target`, which cannot be
// mistaken for one: the second field of a regular-file record is a decimal
// length and "symlink" is not. The bytes reachable through a directory link
// follow as ordinary `rel/sub\0len\0data` records.
//
// THE REACHABLE-FILE COUNT USED TO BE IN THE SYMLINK RECORD AND IS NOT ANY
// MORE. It could not be made to fail: every length-prefixed per-file record that
// follows already distinguishes any two trees the count could, so dropping it
// left every assertion in this file green. A field in a hash stream that no
// mutation can redden is a field that looks load-bearing and pins nothing, which
// is the same vacuity this lane exists to remove one level down. The refusal of
// a link reaching NO file is what that count was reaching for, and that refusal
// is still here — asserted by "the link points at a tree holding no file".
func gateHashOneDocument(h io.Writer, root, rel string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("hash %s: %w", rel, err)
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, linkErr := os.Readlink(path)
		if linkErr != nil {
			return fmt.Errorf("hash %s: read the link: %w", rel, linkErr)
		}
		// os.Stat FOLLOWS the link, so this is the shape of what an agent
		// reading through it would actually get. A dangling link fails here
		// rather than hashing to "a link pointing at nothing", which is a
		// perfectly stable value for a surface nobody can read.
		resolved, statErr := os.Stat(path)
		if statErr != nil {
			return fmt.Errorf("hash %s: the link points at %q, which cannot be read: %w", rel, target, statErr)
		}
		if !resolved.IsDir() {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("hash %s: %w", rel, readErr)
			}
			fmt.Fprintf(h, "%s\x00symlink\x00%s", rel, filepath.ToSlash(target))
			fmt.Fprintf(h, "%s\x00%d\x00%s", rel, len(data), data)
			return nil
		}
		reachable, walkErr := gateReachableFiles(path)
		if walkErr != nil {
			return fmt.Errorf("hash %s: %w", rel, walkErr)
		}
		if len(reachable) == 0 {
			return fmt.Errorf("hash %s: the link points at %q, which holds no file; a surface whose documents reach nothing fingerprints to a constant", rel, target)
		}
		fmt.Fprintf(h, "%s\x00symlink\x00%s", rel, filepath.ToSlash(target))
		for _, sub := range reachable {
			data, readErr := os.ReadFile(filepath.Join(path, filepath.FromSlash(sub)))
			if readErr != nil {
				return fmt.Errorf("hash %s: %w", rel, readErr)
			}
			fmt.Fprintf(h, "%s/%s\x00%d\x00%s", rel, sub, len(data), data)
		}
		return nil

	case info.Mode().IsRegular():
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("hash %s: %w", rel, readErr)
		}
		fmt.Fprintf(h, "%s\x00%d\x00%s", rel, len(data), data)
		return nil

	default:
		return fmt.Errorf("hash %s: it is %s, which is neither a regular file nor a symlink; "+
			"the gate has no reading of those bytes, and a document it cannot read is a surface it is not covering", rel, info.Mode().Type())
	}
}

// gateReachableFiles lists every regular file beneath dir, as sorted
// slash-separated paths relative to dir.
//
// It walks with os.ReadDir rather than filepath.WalkDir because dir is itself
// reached through a symlink and WalkDir does not descend into one. A symlink
// found INSIDE is refused rather than followed or skipped: nothing in this
// repository has that shape today, so the honest answer to meeting one is that
// the gate does not know what it is looking at.
func gateReachableFiles(dir string) ([]string, error) {
	var out []string
	var walk func(prefix string) error
	walk = func(prefix string) error {
		entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(prefix)))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			sub := entry.Name()
			if prefix != "" {
				sub = prefix + "/" + sub
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				return fmt.Errorf("%s is a symlink inside a linked document tree; the gate refuses a shape it has no reading of rather than following or skipping it", sub)
			case entry.IsDir():
				if err := walk(sub); err != nil {
					return err
				}
			case info.Mode().IsRegular():
				out = append(out, sub)
			default:
				return fmt.Errorf("%s is %s, which is neither a regular file nor a directory", sub, info.Mode().Type())
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// ---------------------------------------------------------------------
// the fingerprint
// ---------------------------------------------------------------------

// gateSurfaceInputs is everything one surface agent reads, AND everything the
// surface it is judging claims.
//
// TOTALITY IS TWO-SIDED, and the two sides are different objects. Coverage of
// what the agent READS (Bundle, the shared evidence, the method) is
// what makes a stale QUESTION impossible. Coverage of what the surface CLAIMS
// (Documents) is what makes a stale ANSWER impossible, because every projection
// into a bundle is lossy and the loss is where the defeat lives: `site` resolves
// to 47 files of TSX/TS source and the agent is handed the rendered DOM text,
// and the DOM extractor captures no href at all, so re-pointing a link in
// site/src/content.ts leaves every byte the agent reads byte-identical.
type gateSurfaceInputs struct {
	// Surface is the name declared in surfaces.yaml.
	Surface string
	// Documents are the surface's own files, repo-relative. They come from
	// gateSurfaceDocuments, resolved from the manifest against `git ls-files`,
	// rather than from a hand list — a hand list is how a surface's fingerprint
	// silently stops covering a file the surface actually has. They are hashed
	// whether or not the bundle handed them over.
	Documents []string
	// THE PER-SURFACE CAPTURES ARE NOT A COMPONENT HERE, and that is a decision
	// rather than an omission. The skills export capture, the release-notes
	// prediction and the cross-release render diff are read by ONE surface each
	// and are handed to that agent verbatim, so the Bundle digest already covers
	// their bytes on the run path. A separate `Artifacts` component hashed the
	// same bytes a second time: no edit to a capture could move it without also
	// moving the bundle, so no mutation could redden it, and it survived being
	// deleted from the run's inputs with the whole suite green. A digest
	// component that cannot fail is the shape this lane exists to remove, so it
	// is gone. What replaces it is an assertion that CAN fail —
	// TestGateStage2ACaptureReachesOneSurfaceKeyAndNoOther edits a real capture
	// on the run path and requires exactly that surface's key to move — which is
	// also what would notice a future assembler that stopped carrying them.
	//
	// Bundle is the assembled bytes actually handed to the agent, framing and
	// all. Hashing the ASSEMBLED OUTPUT rather than a list of its parts is the
	// point: the text that wraps the parts — "report FAILED on any mismatch" —
	// lives in a template and an assembler, and softening it asks all thirteen
	// agents a materially weaker question while no part file moves at all.
	Bundle []byte
	Method gateMethod
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

	if len(in.Bundle) == 0 {
		return "", fmt.Errorf("surface fingerprint: surface %q was handed no bundle; a key over zero bytes handed over is a key over a question nobody asked, and it is the same constant for every surface in that state", in.Surface)
	}

	documents, err := gateHashDocuments(root, in.Documents)
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
	bundle := sha256.Sum256(in.Bundle)

	h := sha256.New()
	fmt.Fprintf(h, "dossierx-gate-surface\x00v3\x00%s\x00%s\x00%s\x00sha256:%s\x00%s",
		in.Surface, documents, evidence, hex.EncodeToString(bundle[:]), method)
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
	// Reads are documents this surface DOES NOT OWN but whose bytes its agent
	// needs to judge its own. They are exact repo-relative paths, never
	// patterns: a surface borrowing another's material has to name what it
	// borrowed, and a glob would let the borrowed set grow silently as the other
	// surface does.
	//
	// They take no part in ownership. gateEntryClaims reads Paths and Not only,
	// so a `reads:` entry never makes this surface a second claimant of a file
	// and never disturbs the manifest's exactly-one rule.
	Reads []string `yaml:"reads"`
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

// gateSurfaceReferences resolves every surface's `reads:` list against the
// tracked set, or refuses.
//
// EVERY REFUSAL HERE IS A REFUSAL AND NOT A SHORTER LIST, for the reason
// gateBundleAssemble gives about its own: a bundle assembled over less material
// than it should be still hashes, still looks like a match, and still carries a
// verdict forward. A `reads:` entry that resolves to nothing is a question the
// agent was supposed to be able to answer and now cannot, and it would go
// unnoticed — the finding it produces looks exactly like the coverage gap this
// mechanism exists to close.
func gateSurfaceReferences(root string, tracked []string) (map[string][]string, error) {
	m, err := gateLoadManifest(root)
	if err != nil {
		return nil, err
	}
	isTracked := make(map[string]bool, len(tracked))
	for _, file := range tracked {
		isTracked[file] = true
	}

	out := make(map[string][]string, len(m.Surfaces))
	for _, entry := range m.Surfaces {
		if len(entry.Reads) == 0 {
			continue
		}
		seen := map[string]bool{}
		refs := make([]string, 0, len(entry.Reads))
		for _, rel := range entry.Reads {
			switch {
			case !isTracked[rel]:
				return nil, fmt.Errorf("surface %q reads %q, which is not a tracked file. A reads: entry is an exact repository-relative path, never a pattern; if the file moved, move this entry with it. An unresolvable one is refused rather than dropped, because a dropped one leaves the agent reporting the coverage gap this list exists to close", entry.Name, rel)
			case seen[rel]:
				return nil, fmt.Errorf("surface %q reads %q twice; the bundle would carry it twice under the same heading", entry.Name, rel)
			case gateEntryClaims(entry, rel):
				return nil, fmt.Errorf("surface %q reads %q, which its own paths: already claim. reads: is for documents another surface owns — borrowing your own is either a stale entry or a paths: pattern that has grown, and the two are different edits", entry.Name, rel)
			}
			seen[rel] = true
			refs = append(refs, rel)
		}
		sort.Strings(refs)
		out[entry.Name] = refs
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
// the four shared evidence files, one per-surface capture and a prompt — and
// returns the root and the inputs over it.
//
// It is synthetic rather than the real repository because the point of the tests
// below is to move ONE input at a time and watch the digest move. Three of the
// four evidence files do not exist in this tree until a run produces them (see
// gateDeltaFile), and a fixture is also the only way to assert the direction the
// hashing errs in when one of them is missing.
//
// ITS TOOL GRANT IS THE REAL ONE. It used to grant {"Bash", "Grep", "Read"} —
// precisely the file-reading tools a surface agent must not have, in the value
// an implementer building the harness would copy. A fixture that inverts the
// defence it sits next to is a fixture that will be pasted into the runner, so
// it now carries gateStage2PinnedGrant's own members and gets them from there.
func gateFingerprintFixture(t *testing.T) (root string, in gateSurfaceInputs) {
	t.Helper()
	root = t.TempDir()
	gateWrite(t, root, "site/src/content.ts", "export const latestVersion = \"v9.9.9\";\n")
	gateWrite(t, root, gateSurfaceInventoryFile, "{\"counts\":{\"lint_rules\":28}}\n")
	gateWrite(t, root, gateBaselineFile, "{\"counts\":{\"lint_rules\":27}}\n")
	gateWrite(t, root, gateDeltaFile, "{\"lint_rules\":{\"added\":[\"mixed-cycle\"]}}\n")
	gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX v9.9.9\"}\n")
	gateWrite(t, root, "gate/prompts/site.md", "Read the rendered site text against surface.json.\n")

	return root, gateSurfaceInputs{
		Surface:   "site",
		Documents: []string{"site/src/content.ts"},
		Bundle:    []byte("--- the site question ---\nRead the rendered site text against surface.json.\n"),
		Method: gateMethod{
			Prompts: []string{"gate/prompts/site.md"},
			Model:   "claude-opus-5",
			Tools:   append([]string(nil), gateStage2PinnedGrant...),
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
	shuffled.Method.Tools = gateReversedStrings(in.Method.Tools)
	if len(shuffled.Method.Tools) < 2 || gateEqualStrings(shuffled.Method.Tools, in.Method.Tools) {
		t.Fatalf("the fixture's grant %v cannot be reordered, so the assertion below would compare a list against itself", in.Method.Tools)
	}
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
		// THE ROW THAT DEFEATS FAILURE 1. The fixture's bundle does not contain
		// site/src/content.ts — it holds the question and the rendered text, the
		// same lossy projection the real site agent is handed — so this edit
		// moves not one byte the agent reads. The document component is the only
		// thing in the digest that can see it, which is exactly the shape of an
		// href re-pointed in site/src/content.ts: the DOM extractor captures no
		// href, so gate/site-text.json is byte-identical after the edit, and
		// site/ is outside behaviourRoots so surface.json does not move either.
		{"a document the surface claims but the bundle never contained", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, "site/src/content.ts", "export const latestVersion = \"v9.9.10\";\n")
		}},
		{"a file joins the surface", func(t *testing.T, root string, in *gateSurfaceInputs) {
			gateWrite(t, root, "site/src/new.ts", "export const extra = 1;\n")
			in.Documents = append(in.Documents, "site/src/new.ts")
		}},
		{"surface.json", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, gateSurfaceInventoryFile, "{\"counts\":{\"lint_rules\":29}}\n")
		}},
		{"the resolved baseline the delta was computed against", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, gateBaselineFile, "{\"counts\":{\"lint_rules\":26}}\n")
		}},
		{"the release delta", func(t *testing.T, root string, _ *gateSurfaceInputs) {
			gateWrite(t, root, gateDeltaFile, "{\"lint_rules\":{\"added\":[]}}\n")
		}},
		// The per-surface captures are not a row here on purpose: they reach the
		// key through the assembled bundle, which is where they are actually
		// handed over, and the row that proves it is on the run path in
		// gate_stage2_test.go where a real capture and a real assembler exist. A
		// row here would have had to hold the bundle still while the capture
		// moved, which the run path never does — a fixture arrangement that
		// cannot occur, proving a component nothing else could redden.
		//
		// THE ROW THAT DEFEATS FAILURE 3. Nothing on disk moves: the same
		// prompt, the same documents, the same evidence, the same model and
		// grant. Only the assembled text differs, which is what softening the
		// assembler's framing does — "report FAILED on any mismatch" becoming
		// "note mismatches" edits no file the parts list names.
		{"the framing the assembler wrapped around the parts", func(t *testing.T, _ string, in *gateSurfaceInputs) {
			in.Bundle = []byte("--- the site question ---\nNote any mismatches between the rendered site text and surface.json.\n")
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

	// A missing PER-SURFACE capture — failure 6, the export capture and the
	// release-notes prediction written by flag-driven entry points a driver has
	// to remember to run — is refused two doors down rather than here, because
	// that is where it is actually reachable: the assembler cannot build a
	// bundle without it (gate_bundle_test.go, "this surface's capture is gone")
	// and the run refuses to fingerprint at all when the run manifest does not
	// record it as produced (gate_stage2_test.go). By the time a bundle exists,
	// the capture's bytes are inside it.

	// A bundle of zero bytes is the shape a broken assembler produces, and it
	// hashes to the same constant for every surface that reaches it.
	t.Run("an empty bundle", func(t *testing.T) {
		root, in := gateFingerprintFixture(t)
		in.Bundle = nil
		if _, err := gateSurfaceFingerprint(root, in); err == nil {
			t.Error("a fingerprint was produced for a surface that was handed no bundle")
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

// TestGateDocumentHashAgreesWithTheOneHashFunction is what keeps gateHashDocuments
// from becoming a second answer to "what are these bytes".
//
// hashRepoFiles is this repository's one hash stream and surface.json's own;
// gateHashDocuments exists only because that function cannot read a tracked
// symlink. So wherever both apply — every set of regular files, which is twelve
// of the thirteen surfaces — they must agree to the byte. If they ever diverge,
// the gate and the machine contract are measuring different trees while both
// report a digest, and nothing downstream could tell.
//
// The last row is the honest half: two files whose CONCATENATED contents are
// identical but split differently. hashRepoFiles puts each file's path and byte
// length in the stream precisely so that shape does not collide, and asserting
// it here means the agreement above is over a stream that discriminates rather
// than over two functions that both flatten everything to the same mush.
func TestGateDocumentHashAgreesWithTheOneHashFunction(t *testing.T) {
	root := t.TempDir()
	gateWrite(t, root, "README.md", "the front door\n")
	gateWrite(t, root, "docs/RELEASING.md", "the procedure\n")
	gateWrite(t, root, "site/src/content.ts", "export const x = 1;\n")

	for _, rels := range [][]string{
		{"README.md"},
		{"README.md", "docs/RELEASING.md"},
		{"site/src/content.ts", "README.md", "docs/RELEASING.md"},
	} {
		want, err := hashRepoFiles(root, rels)
		if err != nil {
			t.Fatalf("hashRepoFiles %v: %v", rels, err)
		}
		got, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("gateHashDocuments %v: %v", rels, err)
		}
		if got != want {
			t.Errorf("the gate's document hash disagrees with the one hash function over regular files %v:\n  contract: %s\n      gate: %s", rels, want, got)
		}
	}

	split := t.TempDir()
	gateWrite(t, split, "a.txt", "onetwo")
	gateWrite(t, split, "b.txt", "")
	first, err := gateHashDocuments(split, []string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatalf("hash the first split: %v", err)
	}
	gateWrite(t, split, "a.txt", "one")
	gateWrite(t, split, "b.txt", "two")
	second, err := gateHashDocuments(split, []string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatalf("hash the second split: %v", err)
	}
	if first == second {
		t.Error("moving bytes between two documents left the digest unmoved; the stream is not carrying each document's path and length, so the agreement asserted above is over a hash that cannot tell two trees apart")
	}
}

// TestGateDocumentHashReadsASymlinkedSurface is the repair for the one surface
// whose key could not be computed at all.
//
// `exported-skills` is five tracked symlinks to directories; os.ReadFile on one
// returns "is a directory". The three rows are the three ways that surface can
// move, and the third is the one a "skip what will not open" repair would leave
// uncovered forever.
func TestGateDocumentHashReadsASymlinkedSurface(t *testing.T) {
	fixture := func(t *testing.T) (root string, rels []string) {
		t.Helper()
		root = t.TempDir()
		gateWrite(t, root, "skills/dossierx/SKILL.md", "the router bundle\n")
		gateWrite(t, root, "skills/dossierx/reference.md", "the error table\n")
		// A second bundle whose files are named the same and hold the same
		// bytes. Re-pointing at THIS is the row that isolates the link target:
		// re-pointing at a bundle with different contents would move the digest
		// through the contents, and the target string could be dropped from the
		// stream with the assertion still green. Verified — it was, and it was.
		gateWrite(t, root, "skills/twin/SKILL.md", "the router bundle\n")
		gateWrite(t, root, "skills/twin/reference.md", "the error table\n")
		if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Symlink(filepath.Join("..", "..", "skills", "dossierx"), filepath.Join(root, ".claude", "skills", "dossierx")); err != nil {
			// Not a skip. This repository tracks five symlinks; a machine that
			// cannot make one cannot check out the tree the gate is supposed to
			// judge, and reporting `ok` there is a pass over zero assertions.
			t.Fatalf("this filesystem cannot create a symlink, so the `exported-skills` surface cannot be built here — and it cannot be checked out here either: %v", err)
		}
		return root, []string{".claude/skills/dossierx"}
	}

	root, rels := fixture(t)
	base, err := gateHashDocuments(root, rels)
	if err != nil {
		t.Fatalf("a surface whose document is a symlink to a directory could not be hashed at all: %v", err)
	}

	t.Run("the link is re-pointed at a byte-identical twin", func(t *testing.T) {
		// The twin holds the same file names and the same bytes on purpose. So
		// the ONLY thing that differs is where the link points — which is what
		// this row is for. An earlier version re-pointed at a bundle with
		// different contents, and the digest moved through the contents: the
		// link target could be dropped from the hash stream entirely with this
		// assertion still green. It was a check that passed either way.
		root, rels := fixture(t)
		before, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		link := filepath.Join(root, ".claude", "skills", "dossierx")
		if err := os.Remove(link); err != nil {
			t.Fatalf("remove the link: %v", err)
		}
		if err := os.Symlink(filepath.Join("..", "..", "skills", "twin"), link); err != nil {
			t.Fatalf("re-point the link: %v", err)
		}
		after, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash after re-pointing: %v", err)
		}
		if after == before {
			t.Error("re-pointing the link at a different directory left the digest unmoved. " +
				"The export would be tracking another bundle while every key in the system said nothing had changed")
		}
	})

	t.Run("bytes reachable through the link change", func(t *testing.T) {
		root, rels := fixture(t)
		before, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		gateWrite(t, root, "skills/dossierx/reference.md", "the error table, rewritten\n")
		after, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash after the edit: %v", err)
		}
		if after == before {
			t.Error("editing a file reachable through the link left the digest unmoved")
		}
	})

	t.Run("a SAME-LENGTH edit behind the link", func(t *testing.T) {
		// The row above changes the file's LENGTH, so it passes on the length
		// field alone: the CONTENTS could be dropped from the record entirely
		// and it would stay green. Verified — they could, and it did. This row
		// rewrites the file to exactly the same number of bytes, which is what a
		// version bump, a re-spelled error code or a re-pointed URL of the same
		// width actually looks like inside an exported skill bundle. The only
		// thing left that can move the digest is the bytes themselves.
		root, rels := fixture(t)
		const was = "the error table\n"
		const now = "the erro7 table\n"
		if len(was) != len(now) {
			t.Fatalf("this row only means anything if the two bodies are the same length: %d vs %d", len(was), len(now))
		}
		before, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		gateWrite(t, root, "skills/dossierx/reference.md", now)
		after, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash after the edit: %v", err)
		}
		if after == before {
			t.Error("a same-length edit to a file reachable through the link left the digest unmoved. " +
				"The key covers the file's name and size and not what it says, so the exported bundle could be rewritten under a carried-forward PASS")
		}
	})

	t.Run("the link points at a tree holding no file", func(t *testing.T) {
		// A surface whose documents reach nothing fingerprints to a constant —
		// stable, indistinguishable from a match, and covering nothing that can
		// change. It is the end state of a "repair" that empties the export
		// rather than fixing it, and until this row existed the refusal was
		// unexercised: replacing it with `if false` left the suite green.
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "skills", "hollow"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.Symlink(filepath.Join("..", "..", "skills", "hollow"), filepath.Join(root, ".claude", "skills", "hollow")); err != nil {
			t.Fatalf("this filesystem cannot create a symlink: %v", err)
		}
		if _, err := gateHashDocuments(root, []string{".claude/skills/hollow"}); err == nil {
			t.Error("a digest was produced for a link pointing at a tree holding no file; that value is a constant, and a constant carries every verdict forward forever")
		}

		// And a tree holding only an empty SUBDIRECTORY is the same nothing,
		// reached one level down.
		if err := os.MkdirAll(filepath.Join(root, "skills", "hollow", "nested"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if _, err := gateHashDocuments(root, []string{".claude/skills/hollow"}); err == nil {
			t.Error("a digest was produced for a link reaching only empty directories")
		}
	})

	t.Run("a file joins the tree behind the link", func(t *testing.T) {
		root, rels := fixture(t)
		before, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		gateWrite(t, root, "skills/dossierx/recovery.md", "a new page in the bundle\n")
		after, err := gateHashDocuments(root, rels)
		if err != nil {
			t.Fatalf("hash after the addition: %v", err)
		}
		if after == before {
			t.Error("a file added behind the link left the digest unmoved; the surface grew and its key did not")
		}
	})

	t.Run("the link is dangling", func(t *testing.T) {
		root, rels := fixture(t)
		if err := os.RemoveAll(filepath.Join(root, "skills", "dossierx")); err != nil {
			t.Fatalf("remove the target: %v", err)
		}
		if _, err := gateHashDocuments(root, rels); err == nil {
			t.Error("a digest was produced for a link pointing at nothing; that value is stable, looks like a match, and covers a surface no agent can read")
		}
	})

	t.Run("a document that is neither a file nor a link", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if _, err := gateHashDocuments(root, []string{"docs/decisions"}); err == nil {
			t.Error("a digest was produced for a document that is a plain directory")
		}
	})

	// The fixture is honest: the untouched tree hashes, so every row above is
	// the mechanism under test rather than a hasher that refuses everything.
	if again, err := gateHashDocuments(root, rels); err != nil || again != base {
		t.Fatalf("two hashes of one unchanged linked surface disagree (%v): %s vs %s", err, base, again)
	}
}

// gateReversedStrings returns a reversed copy, for the ordering assertions that
// need a list that is the same SET in a different order.
func gateReversedStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for i := len(in) - 1; i >= 0; i-- {
		out = append(out, in[i])
	}
	return out
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
