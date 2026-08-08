// gate_stage2_test.go is the RUN: the thirteen reading agents, the machinery
// that decides which of them run, and the three properties that have to hold
// before a carried-forward PASS means anything.
//
// A carried-forward PASS says exactly one thing: this surface's agent already
// answered a question identical in every byte to the one this tree would ask it
// now. gate_fingerprint_test.go computes the digest that sentence rests on and
// gate_bundle_test.go builds the bytes it covers. This file is where the
// sentence becomes true of a RUN:
//
//	FRESHNESS      — every derived input the key covers was produced, in this
//	                 run, from this tree and the baseline that was actually
//	                 resolved for it. Found-on-disk is not produced.
//	TOTAL COVERAGE — every surface surfaces.yaml declares gets a key, or the run
//	                 is FAILED. Not dropped, not sampled, not deferred. And this
//	                 is exercised against the REAL repository, because the
//	                 structural gap that let the landed code ship with an
//	                 uncomputable surface is that every fingerprint assertion ran
//	                 against a synthetic fixture, and fixtures hold regular files.
//	THE GRANT      — the tool set the harness passes is the exact set declared in
//	                 gate/method.yaml, which is what makes "an input outside the
//	                 bundle cannot exist" a fact rather than a hope.
//
// Everything downstream of a verdict already exists and is not re-derived here:
// gatePlanRerun, gateIsGreen and the receipt already refuse a declared surface
// holding no verdict, a non-PASS, a PASS with no fingerprint for this tree, a
// PASS against a different fingerprint, a verdict for an undeclared surface, and
// a surface reported twice. This file's job is to hand them a run whose shape
// those refusals can actually judge.
//
// Same shape as the rest of the gate files: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------
// the harness and its declaration
// ---------------------------------------------------------------------

const (
	gateStage2MethodFile  = "gate/method.yaml"
	gateStage2RunFile     = "gate/run.json"
	gateStage2HarnessFile = "scripts/gate-stage2/run.sh"
)

// gateStage2PinnedGrant is the EXACT tool set a surface agent may call.
//
// It is pinned as a literal here as well as declared in gate/method.yaml, and
// that is not a redundant copy — it is the second lock. The declaration is what
// the harness reads and passes; this is what makes widening it cost a deliberate
// edit to a test that a human reviews. Either alone is defeatable: a grant that
// lived only in the config widens in a line nobody diffs, and a grant that lived
// only here would not be the value the harness passes.
//
// BOTH MEMBERS ARE REPORT-ONLY. Neither reads a byte. The whole cache design
// rests on an agent being unable to reach anything the assembler did not hand
// it: if it can, no key over the bundle is total, every property in
// gate_fingerprint_test.go degrades into a hope about what the model chose to
// read, and a verdict carried forward is carried over evidence nobody recorded.
//
// A NEW TOOL NAME IS NOT COVERED BY A DENY LIST, which is why this is stated as
// what IS granted. A screen written as "reject Read, Grep, Glob, Bash" is walked
// straight past by Task, by WebFetch, and by an MCP tool called
// mcp__filesystem__read_file — and it stays green while it happens.
var gateStage2PinnedGrant = []string{"SurfaceFinding", "SurfaceVerdict"}

// gateStage2ForbiddenGrant is the second, independent statement about the same
// list: no member of the grant may be one of these.
//
// It exists because the exact-set assertion alone has a hole. Someone widening
// the grant edits gate/method.yaml, watches the test go red, and edits the
// literal above to match — the two agree again and nothing objects. This names
// the classes that must never appear whatever the literal says, and it is
// asserted against the LITERAL rather than against the config, so it fires on
// exactly that edit. It is not a screen the runtime applies; the grant is still
// an exact set.
var gateStage2ForbiddenGrant = []string{
	"Bash", "Edit", "Glob", "Grep", "NotebookEdit", "Read", "Task",
	"WebFetch", "WebSearch", "Write", "mcp__",
}

// gateStage2ReachesBytes is the screen itself, as a function, so that it can be
// asked about a tool name rather than only read.
//
// Emptying the list above used to leave every test green: the screen was written
// inline in one test and its only inputs were the two granted tools, so the
// question "does this list still name the reaching tools" was never put.
// TestGateStage2TheForbiddenClassesAreTheReachingTools puts it, one known
// reaching tool at a time, and a widening edit that also empties the list is
// caught at the list rather than three edits later.
//
// The prefix rule is what makes it survive a name nobody has invented yet: MCP
// tools are all `mcp__<server>__<tool>`, so the class is screened rather than
// the members.
func gateStage2ReachesBytes(tool string) bool {
	for _, forbidden := range gateStage2ForbiddenGrant {
		if tool == forbidden || strings.HasPrefix(tool, forbidden) {
			return true
		}
	}
	return false
}

// gateStage2Declaration is gate/method.yaml: the model and the exact grant.
type gateStage2Declaration struct {
	Model string   `yaml:"model"`
	Tools []string `yaml:"tools"`
}

// gateStage2LoadDeclaration reads gate/method.yaml.
//
// Every emptiness is an error for the reason gateMethod.version refuses one: a
// method that asks nothing, or an agent that can call nothing, still produces a
// stable digest, and a stable digest carries every verdict forward.
func gateStage2LoadDeclaration(root string) (gateStage2Declaration, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateStage2MethodFile)))
	if err != nil {
		return gateStage2Declaration{}, fmt.Errorf("read %s: %w", gateStage2MethodFile, err)
	}
	var d gateStage2Declaration
	if err := yaml.Unmarshal(raw, &d); err != nil {
		return gateStage2Declaration{}, fmt.Errorf("parse %s: %w", gateStage2MethodFile, err)
	}
	if strings.TrimSpace(d.Model) == "" {
		return gateStage2Declaration{}, fmt.Errorf("%s declares no model", gateStage2MethodFile)
	}
	if len(d.Tools) == 0 {
		return gateStage2Declaration{}, fmt.Errorf("%s declares no tools; an agent that can call nothing cannot even report FAILED, and thirteen silent agents read as thirteen clean passes", gateStage2MethodFile)
	}
	return d, nil
}

// gateStage2Method is one surface's method: its own question, plus the model and
// grant every surface shares.
//
// The frame (gate/prompts/_frame.md) is deliberately absent — see
// gateBundleFrameFile for why, and TestGateBundleFrameReachesTheKeyOnlyThroughTheBundle
// for the assertion that keeps it absent.
func gateStage2Method(root, surface string) (gateMethod, error) {
	d, err := gateStage2LoadDeclaration(root)
	if err != nil {
		return gateMethod{}, err
	}
	return gateMethod{
		Prompts: []string{gateBundlePromptFile(surface)},
		Model:   d.Model,
		Tools:   append([]string(nil), d.Tools...),
	}, nil
}

// ---------------------------------------------------------------------
// what each surface is handed
// ---------------------------------------------------------------------

// gateStage2Artifacts is the capture artifact each surface reads ON ITS OWN.
//
// These are attached PER SURFACE rather than folded into gateSharedEvidence, and
// the choice is a cost one rather than a correctness one — the invariant holds
// either way, because either way the artifact is in the key of every surface
// that reads it and absent-or-stale is refused. Shared would move all thirteen
// keys whenever any one of these moved, and the whole value of the key is that a
// one-document fix re-runs one agent.
//
// TestGateStage2EverySurfaceNamedInAPolicyIsDeclared holds the names here to the
// manifest, so renaming a surface cannot silently orphan its capture.
func gateStage2Artifacts(surface string) []string {
	switch surface {
	case "exported-skills":
		// tests/skills_export_capture_test.go, -skills-export-capture-out
		return []string{"gate/export-output.json"}
	case "release-notes":
		// tests/release_notes_predict_test.go, -release-notes-predict-out
		return []string{"gate/release-notes-prediction.json"}
	case "changelog":
		// tests/render_across_releases_test.go: the cross-release render diff,
		// which is what a CHANGELOG entry describing a silent rendering change
		// has to be written from.
		return []string{"gate/render-diff.json"}
	}
	return nil
}

// gateStage2HandsAnExtract reports whether a surface's agent is handed an
// EXTRACT of its documents rather than their bytes.
//
// Two surfaces, for two different reasons, and in both cases the extract is
// already shared evidence. `site` resolves to 47 files of TSX/TS source and the
// surface is the RENDERED DOM of a real build — the source is what produces the
// thing under review, not the thing. `binary-and-viewer` resolves to 106 files
// and 1.95 MB of Go, and surface.json is the mechanical extraction of exactly
// the fields prose is judged against.
//
// THE EXTRACT IS LOSSY AND THAT IS WHERE THE DEFEAT LIVES. The DOM extractor
// captures text, labels, states and head metadata and captures no href at all,
// so re-pointing a link in site/src/content.ts leaves every byte the agent reads
// byte-identical. That is why the withheld documents are still hashed into the
// surface's key, and why the bundle names them without carrying them.
func gateStage2HandsAnExtract(surface string) bool {
	return surface == "site" || surface == "binary-and-viewer"
}

// gateStage2BundleSpec is what one surface's bundle is assembled from.
func gateStage2BundleSpec(surface string, documents []string) (gateBundleSpec, error) {
	if len(documents) == 0 {
		return gateBundleSpec{}, fmt.Errorf("surface %q resolved to no document", surface)
	}
	spec := gateBundleSpec{Surface: surface, Artifacts: gateStage2Artifacts(surface)}
	if gateStage2HandsAnExtract(surface) {
		spec.Withheld = append([]string(nil), documents...)
	} else {
		spec.Handed = append([]string(nil), documents...)
	}
	return spec, nil
}

// gateStage2Inputs is one surface's whole key input set.
func gateStage2Inputs(root, surface string, documents []string) (gateSurfaceInputs, error) {
	spec, err := gateStage2BundleSpec(surface, documents)
	if err != nil {
		return gateSurfaceInputs{}, err
	}
	bundle, err := gateBundleAssemble(root, spec)
	if err != nil {
		return gateSurfaceInputs{}, err
	}
	method, err := gateStage2Method(root, surface)
	if err != nil {
		return gateSurfaceInputs{}, err
	}
	return gateSurfaceInputs{
		Surface: surface,
		// The manifest-resolved set, whether the bundle handed it over or not.
		Documents: append([]string(nil), documents...),
		// The per-surface captures are NOT listed as a separate key component:
		// gateBundleAssemble hands each of them to the agent verbatim, so the
		// bundle above already carries their bytes. A second component over the
		// same bytes could not be moved without moving the bundle too, which
		// made it a component no mutation could redden — see the note on
		// gateSurfaceInputs. What holds the coverage is
		// TestGateStage2ACaptureReachesOneSurfaceKeyAndNoOther, which edits a
		// real capture on this path and requires exactly one key to move.
		Bundle: bundle,
		Method: method,
	}, nil
}

// ---------------------------------------------------------------------
// freshness
// ---------------------------------------------------------------------

// gateStage2Run is gate/run.json: what this run produced, from which tree, and
// against which resolved baseline.
type gateStage2Run struct {
	Tree     string `json:"tree"`
	Baseline struct {
		Ref    string `json:"ref"`
		Commit string `json:"commit"`
	} `json:"baseline"`
	Artifacts []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"artifacts"`
}

// gateStage2ObjectName is the 40-hex-digit object name, applied on the READING
// side and not only in the producer.
//
// A baseline is an identity or it is nothing. "v0.5.0" is a mutable pointer —
// `git tag -f` re-points an annotated tag under anything that names only the tag
// — and an abbreviation is a prefix that means different objects in different
// clones. The producer refuses both, but a run manifest is a FILE: it can be
// hand-written, half-written or edited after the fact, and a reader that accepts
// whatever it finds there has moved the rule to the one place it cannot be
// enforced. Until this existed, a run.json naming a tag passed freshness.
var gateStage2ObjectName = regexp.MustCompile(`^[0-9a-f]{40}$`)

// gateStage2ProducedArtifacts is everything a run must PRODUCE before any key is
// computed.
//
// IT IS DERIVED FROM gateSharedEvidence RATHER THAN LISTED. Every file in a key
// that has no committed form has to be produced by the run that keys it, and
// that is a consequence of being in the key rather than a fact about three
// particular paths. Writing the three out by hand made the membership of each an
// independent claim nothing checked: gate/baseline.json — the one artifact whose
// entire purpose is that a projection never stands in for its source — could be
// dropped from the list with the whole suite green, after which a baseline left
// from the previous release passed freshness. Derived, a fifth evidence file is
// covered the day it is added, and dropping one means deleting it from the key.
//
// surface.json is the one exclusion, and it earns it by being TRACKED: it is a
// committed document regenerated by the emitter and held current by
// TestGenerateSurfaceJSON, so "where did this come from" already has an answer
// in git. Nothing else here has a committed form at all, which is exactly why
// whatever happened to be at those paths on the day of a run used to hash
// cleanly into all thirteen keys.
func gateStage2ProducedArtifacts(declared []string) []string {
	var out []string
	for _, rel := range gateSharedEvidence() {
		if rel == gateSurfaceInventoryFile {
			continue
		}
		out = append(out, rel)
	}
	for _, surface := range declared {
		out = append(out, gateStage2Artifacts(surface)...)
	}
	sort.Strings(out)
	return out
}

// errGateStage2NotProduced is an input the run cannot claim to have produced.
//
// A sentinel, because the tempting handling is the wrong one in both directions:
// treat it as absent and the run reports a smaller evidence set as if that were
// the coverage it was asked for; treat it as present and a file left over from a
// previous release sits under a verdict recorded against a key that is perfectly
// current for this tree, and is carried into every later re-run.
var errGateStage2NotProduced = errors.New("an input the keys cover was not produced by this run")

// gateStage2CheckFreshness is the freshness half of the invariant.
//
// FOUND ON DISK IS NOT PRODUCED. Every one of these artifacts has no committed
// form, so "it is there" says nothing about where it came from. The run manifest
// is what says: this tree, this resolved baseline, these bytes. Without it the
// concrete defeat is one line — `mkdir -p gate && printf '{}' > gate/delta.json`
// before a run — after which the hand-written file hashes cleanly into all
// thirteen keys, every key is current for the tree, and the verdicts are
// recorded and carried forward. hashRepoFiles errors only when the read fails,
// so the refusal built into gateSurfaceFingerprint fires on a MISSING file and
// never on a wrong one.
func gateStage2CheckFreshness(root, tree string, declared []string) (gateStage2Run, error) {
	var run gateStage2Run
	if strings.TrimSpace(tree) == "" {
		return run, fmt.Errorf("%w: the run was not told which tree it covers, so nothing it produced can be attached to one", errGateStage2NotProduced)
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateStage2RunFile)))
	if err != nil {
		return run, fmt.Errorf("%w: %s: %v. Every artifact the keys cover is a per-run file with no committed form, so with no manifest there is nothing to distinguish this release's evidence from a copy left behind by the last one",
			errGateStage2NotProduced, gateStage2RunFile, err)
	}
	if err := json.Unmarshal(raw, &run); err != nil {
		return gateStage2Run{}, fmt.Errorf("%w: %s does not parse: %v", errGateStage2NotProduced, gateStage2RunFile, err)
	}

	if run.Tree != tree {
		return gateStage2Run{}, fmt.Errorf("%w: %s records tree %q and this run covers tree %q. The evidence on disk was produced against a different tree; it is not a smaller evidence set to hash, it is a run that cannot state its own coverage",
			errGateStage2NotProduced, gateStage2RunFile, run.Tree, tree)
	}
	if run.Baseline.Commit == "" {
		return gateStage2Run{}, fmt.Errorf("%w: %s names no resolved baseline commit. A baseline that could not be resolved reaches the human as a failure, never as a delta that happens to be empty",
			errGateStage2NotProduced, gateStage2RunFile)
	}
	if !gateStage2ObjectName.MatchString(run.Baseline.Commit) {
		return gateStage2Run{}, fmt.Errorf("%w: %s records baseline commit %q, which is not a full object name. A tag is a mutable pointer and an abbreviation is a prefix; either can mean a different release tomorrow than it meant when this run recorded it, and every carry-forward decision below rests on which release the comparison was against",
			errGateStage2NotProduced, gateStage2RunFile, run.Baseline.Commit)
	}

	recorded := map[string]string{}
	for _, a := range run.Artifacts {
		recorded[a.Path] = a.SHA256
	}
	var problems []string
	for _, rel := range gateStage2ProducedArtifacts(declared) {
		want, listed := recorded[rel]
		if !listed {
			problems = append(problems, fmt.Sprintf("%s is not in %s; it was found on disk rather than produced, and found on disk is not produced", rel, gateStage2RunFile))
			continue
		}
		// A PLAIN file digest, which is what `shasum -a 256` and therefore the
		// producer records. Deliberately not hashRepoFiles' stream: that one
		// carries the path and byte length too, and comparing the two would be
		// comparing two different measurements and calling every disagreement a
		// stale artifact.
		plain, plainErr := gateStage2FileDigest(root, rel)
		if plainErr != nil {
			problems = append(problems, fmt.Sprintf("%s is recorded as produced and cannot be read: %v", rel, plainErr))
			continue
		}
		if plain != want {
			problems = append(problems, fmt.Sprintf("%s hashes as %s and %s records %s; the file on disk is not the file this run produced", rel, plain, gateStage2RunFile, want))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return gateStage2Run{}, fmt.Errorf("%w:\n  %s", errGateStage2NotProduced, strings.Join(problems, "\n  "))
	}
	return run, nil
}

// gateStage2FileDigest is a plain sha256 of one file's bytes — the digest shape
// scripts/gate-stage2/run.sh records, which is what `shasum -a 256` produces.
// It is NOT hashRepoFiles' stream: that one carries the path and length too, and
// comparing the two would be comparing two different measurements and calling
// the disagreement a stale artifact.
func gateStage2FileDigest(root, rel string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256Sum(data)), nil
}

// ---------------------------------------------------------------------
// the delta: empty is an answer, unresolvable is a failure
// ---------------------------------------------------------------------

// gateStage2Delta is gate/delta.json.
//
// Changed is a POINTER so that an absent `changed` key and an empty list are
// different values. They are different facts: this project's first gated release
// legitimately changes no shipped code — `git diff v0.5.0..HEAD -- cmd/ internal/
// skills/ go.mod` is six files, every one a *_test.go — so an empty list is the
// correct answer and must not be refused. A missing list means the producer
// never computed one, which must not read as "nothing changed".
// Tree and Baseline are the delta's own account of WHICH RELEASE it compares —
// and they are read and checked rather than carried. A delta that names its tree
// and is never asked about it is a field that documents an assumption instead of
// enforcing it: see gateStage2CheckDeltaCovers for the sequence that turns that
// into a shipped release.
//
// Baseline.SHA256 is the digest of the baseline inventory the comparison was
// actually made against — gate/baseline.json's bytes. It is here because
// property 2 says a projection never stands in for its source: the delta is a
// lossy read of a PAIR of inventories, and without this the run holds a summary
// and a file that are only assumed to be about each other.
type gateStage2Delta struct {
	Tree     string `json:"tree"`
	Baseline struct {
		Ref    string `json:"ref"`
		Commit string `json:"commit"`
		SHA256 string `json:"sha256"`
	} `json:"baseline"`
	Changed *[]string `json:"changed"`
}

// errGateStage2UnresolvedBaseline is the failure that must never be reported as
// an empty delta.
var errGateStage2UnresolvedBaseline = errors.New("the release baseline could not be resolved")

// gateStage2ReadDelta reads and judges gate/delta.json.
//
// The three states it has to keep apart, and what each one means to a human:
//
//	absent / unparseable  — no delta was produced at all. Failure.
//	resolved, no list     — a delta was produced with no comparison in it.
//	                        Failure, and NOT "nothing changed".
//	resolved, empty list  — the baseline was resolved and nothing moved. This is
//	                        a legitimate and expected answer on this tree.
func gateStage2ReadDelta(root string) (gateStage2Delta, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateDeltaFile)))
	if err != nil {
		return gateStage2Delta{}, fmt.Errorf("%s is absent, so no comparison against the previous release was made at all: %w", gateDeltaFile, err)
	}
	var d gateStage2Delta
	if err := json.Unmarshal(raw, &d); err != nil {
		return gateStage2Delta{}, fmt.Errorf("%s does not parse, so whatever is at that path is not a delta: %w", gateDeltaFile, err)
	}
	if d.Baseline.Commit == "" {
		return gateStage2Delta{}, fmt.Errorf("%w: %s names no baseline commit. A tag name is a mutable pointer and an error is not an identity; neither is grounds to hand thirteen agents a document as the truth about the past",
			errGateStage2UnresolvedBaseline, gateDeltaFile)
	}
	if !gateStage2ObjectName.MatchString(d.Baseline.Commit) {
		return gateStage2Delta{}, fmt.Errorf("%w: %s names baseline commit %q, which is not a full object name. A tag is a mutable pointer and an abbreviation is a prefix; a comparison against either cannot say which release it was against",
			errGateStage2UnresolvedBaseline, gateDeltaFile, d.Baseline.Commit)
	}
	if d.Changed == nil {
		return gateStage2Delta{}, fmt.Errorf("%s resolved a baseline and carries no `changed` list; nothing was compared, and an absent comparison must not read as an empty one", gateDeltaFile)
	}
	return d, nil
}

// errGateStage2StaleDelta is a delta that is present, parseable, internally
// consistent — and about a different release than the one being gated.
var errGateStage2StaleDelta = errors.New("the release delta does not describe the release being gated")

// gateStage2CheckDeltaCovers makes the delta's own account of itself
// load-bearing.
//
// THE SEQUENCE THIS CATCHES IS ORDINARY, not adversarial. A gate run FAILS; a
// fix lands; the tree moves; the driver re-runs the captures and `record` but
// not `delta`. `record` re-digests whatever is on disk, so the stale delta is
// written into a manifest that is perfectly current for the new tree, and
// freshness — which only ever compared the MANIFEST's tree — passes. Thirteen
// agents are then handed a document describing a different release as the truth
// about what moved, and on a docs-only follow-up where surface.json does not
// move, every key is identical to the previous run's, so every PASS is carried
// forward too. Both facts needed to catch it were already on disk and neither
// was read: gate/delta.json carries its own tree, and it carries the baseline it
// resolved.
//
// THE BASELINE IS CHECKED THREE WAYS BECAUSE THERE ARE THREE CLAIMS. The delta
// says which commit it compared against; the run manifest says which commit this
// run resolved; and gate/baseline.json holds the bytes that go into every key.
// Any two of them agreeing while the third does not is a key hashing an
// inventory that belongs to neither claim, and that is not a hypothetical shape
// — it is what a re-resolved baseline with an un-recomputed delta looks like.
func gateStage2CheckDeltaCovers(root, tree string, run gateStage2Run, delta gateStage2Delta) error {
	var problems []string

	if delta.Tree != tree {
		problems = append(problems, fmt.Sprintf(
			"%s was computed over tree %q and this run covers tree %q; it describes what moved in some other release",
			gateDeltaFile, delta.Tree, tree))
	}
	if delta.Baseline.Commit != run.Baseline.Commit {
		problems = append(problems, fmt.Sprintf(
			"%s compared against baseline %q and %s records that this run resolved %q; the run and its own comparison disagree about which release is the past",
			gateDeltaFile, delta.Baseline.Commit, gateStage2RunFile, run.Baseline.Commit))
	}
	if delta.Baseline.Ref != run.Baseline.Ref {
		problems = append(problems, fmt.Sprintf(
			"%s names baseline ref %q and %s records %q",
			gateDeltaFile, delta.Baseline.Ref, gateStage2RunFile, run.Baseline.Ref))
	}

	// The BYTES the projection was derived from. gate/baseline.json is what
	// every key actually hashes; the delta is a summary of a comparison against
	// it. Without this, the summary and the bytes are only assumed to be about
	// each other.
	switch digest, err := gateStage2FileDigest(root, gateBaselineFile); {
	case err != nil:
		problems = append(problems, fmt.Sprintf("%s cannot be read, so nothing ties %s to the inventory it says it compared against: %v", gateBaselineFile, gateDeltaFile, err))
	case delta.Baseline.SHA256 == "":
		problems = append(problems, fmt.Sprintf(
			"%s does not record the digest of the baseline inventory it read; a comparison that cannot name its own source is a summary of bytes nobody can check, and those bytes are in all thirteen keys",
			gateDeltaFile))
	case delta.Baseline.SHA256 != digest:
		problems = append(problems, fmt.Sprintf(
			"%s was computed against a baseline inventory hashing %s and %s on disk hashes %s; the keys would carry bytes the comparison never saw",
			gateDeltaFile, delta.Baseline.SHA256, gateBaselineFile, digest))
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%w:\n  %s", errGateStage2StaleDelta, strings.Join(problems, "\n  "))
	}
	return nil
}

// ---------------------------------------------------------------------
// the run
// ---------------------------------------------------------------------

// gateStage2Keys computes a key for EVERY declared surface, or returns nothing.
//
// It never returns a partial map. Twelve keys and an error would be twelve
// surfaces a caller could plausibly report on, and "we did not check the
// thirteenth" would reach the human only if every caller remembered to look at
// the error — which is the reading CLAUDE.md forbids, arrived at by an
// omission rather than a decision. That is not hypothetical here: before the
// symlink repair, `exported-skills` was the surface whose key could not be
// computed, and the repair anyone reaches for under time pressure is to skip
// the document that will not open.
func gateStage2Keys(root string, tracked []string) (map[string]string, error) {
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		return nil, err
	}
	documents, err := gateSurfaceDocuments(root, tracked)
	if err != nil {
		return nil, err
	}

	keys := make(map[string]string, len(declared))
	var problems []string
	for _, surface := range declared {
		in, inErr := gateStage2Inputs(root, surface, documents[surface])
		if inErr != nil {
			problems = append(problems, inErr.Error())
			continue
		}
		key, keyErr := gateSurfaceFingerprint(root, in)
		if keyErr != nil {
			problems = append(problems, keyErr.Error())
			continue
		}
		keys[surface] = key
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%d of %d declared surfaces could not be fingerprinted, so this run cannot state its own coverage:\n  %s",
			len(problems), len(declared), strings.Join(problems, "\n  "))
	}
	return keys, nil
}

// gateStage2Plan is the whole precondition to fanning thirteen agents out:
// freshness, then the delta's own state, then the agreement between the two,
// then a key for every declared surface.
//
// EVERY STEP IS HERE BECAUSE THIS IS THE ONLY PATH. Each of the checks below is
// tested in isolation too, but a check that is thoroughly tested and not called
// is a check that does not run: deleting either of the first two calls from this
// function used to leave the whole suite green. So
// TestGateStage2PlanRefusesEveryRunItCannotStandBehind drives THIS function
// through each refusal, which is what holds the calls in place.
//
// Freshness is checked BEFORE the keys for cost, the same reasoning
// gateG1Precondition uses: evidence produced against another tree makes every
// key wrong, and computing thirteen of them first only delays the same refusal.
func gateStage2Plan(root, tree string, tracked []string) (map[string]string, error) {
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		return nil, err
	}
	run, err := gateStage2CheckFreshness(root, tree, declared)
	if err != nil {
		return nil, err
	}
	delta, err := gateStage2ReadDelta(root)
	if err != nil {
		return nil, err
	}
	if err := gateStage2CheckDeltaCovers(root, tree, run, delta); err != nil {
		return nil, err
	}
	return gateStage2Keys(root, tracked)
}

// ---------------------------------------------------------------------
// test helpers
// ---------------------------------------------------------------------

// The tree, ref and resolved baseline every fixture in this file agrees about.
// They are constants rather than literals repeated per fixture because half the
// point of the checks above is that the run's manifest and the run's delta agree
// about exactly these three values — and a fixture that disagrees with itself by
// typo would make those checks look like they fire when they do not.
const (
	gateStage2FixtureTree     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	gateStage2FixtureRef      = "v0.5.0"
	gateStage2FixtureBaseline = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// gateStage2WriteEvidence writes gate/baseline.json and a gate/delta.json that
// honestly describes it: the same tree, the same resolved baseline, and the
// digest of the baseline bytes it was actually computed from.
//
// This is what scripts/gate-stage2/run.sh delta produces, and the fixtures build
// it here rather than by hand so that a test which means to break ONE of those
// agreements does not silently start from a fixture that broke two.
func gateStage2WriteEvidence(t *testing.T, root, tree, baselineBody, changedJSON string) {
	t.Helper()
	gateWrite(t, root, gateBaselineFile, baselineBody)
	digest, err := gateStage2FileDigest(root, gateBaselineFile)
	if err != nil {
		t.Fatalf("digest %s: %v", gateBaselineFile, err)
	}
	gateWrite(t, root, gateDeltaFile, fmt.Sprintf(
		"{\n  \"tree\": %q,\n  \"baseline\": {\"ref\": %q, \"commit\": %q, \"sha256\": %q},\n  \"changed\": %s\n}\n",
		tree, gateStage2FixtureRef, gateStage2FixtureBaseline, digest, changedJSON))
}

// gateStage2Harness runs one mode of scripts/gate-stage2/run.sh and returns its
// stdout. It runs the REAL script — pinning the harness against a copy in a test
// would be pinning the guarantee to a fixture, which under this repository's own
// rules is a check that cannot execute and therefore a FAIL.
func gateStage2Harness(t *testing.T, args ...string) (string, error) {
	t.Helper()
	script := filepath.Join(surfaceRepoRoot(t), filepath.FromSlash(gateStage2HarnessFile))
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("the stage-2 harness is not in the tree, so the tool grant is pinned to nothing: %v", err)
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}

func gateStage2Lines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// gateStage2Overlay builds a working root that IS the real repository for every
// tracked document, and holds this run's per-run evidence in a directory of its
// own.
//
// Directories are symlinked and top-level files are copied, so a document
// reached through the overlay has the same SHAPE it has in the real tree: a
// regular file lstats as a regular file, and .claude/skills/* lstats as the
// symlinks they are. Copying the whole tree would work too and would be slower;
// symlinking the top-level files instead would silently turn all twenty of them
// into links and make the file-or-link assertions meaningless.
func gateStage2Overlay(t *testing.T) (overlay, realRoot string) {
	t.Helper()
	realRoot = surfaceRepoRoot(t)
	overlay = t.TempDir()

	entries, err := os.ReadDir(realRoot)
	if err != nil {
		t.Fatalf("read the repository root: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == "scratchpad" || name == "gate" {
			continue
		}
		src := filepath.Join(realRoot, name)
		dst := filepath.Join(overlay, name)
		info, statErr := os.Lstat(src)
		if statErr != nil {
			t.Fatalf("stat %s: %v", name, statErr)
		}
		if info.IsDir() {
			if err := os.Symlink(src, dst); err != nil {
				t.Fatalf("symlink %s: %v", name, err)
			}
			continue
		}
		data, readErr := os.ReadFile(src)
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// The gate's own tracked working material comes from the real tree; the
	// per-run evidence is stubbed, because producing it needs a real browser, a
	// built binary and a resolved baseline. What is being exercised here is that
	// thirteen keys are computable over the real manifest and the real
	// documents — the thing no test in this repository has ever done.
	if err := os.MkdirAll(filepath.Join(overlay, "gate"), 0o755); err != nil {
		t.Fatalf("mkdir gate: %v", err)
	}
	if err := os.Symlink(filepath.Join(realRoot, "gate", "prompts"), filepath.Join(overlay, "gate", "prompts")); err != nil {
		t.Fatalf("symlink gate/prompts: %v", err)
	}
	method, err := os.ReadFile(filepath.Join(realRoot, filepath.FromSlash(gateStage2MethodFile)))
	if err != nil {
		t.Fatalf("read %s: %v", gateStage2MethodFile, err)
	}
	gateWrite(t, overlay, gateStage2MethodFile, string(method))

	gateStage2WriteEvidence(t, overlay, gateStage2FixtureTree, "{\"counts\":{\"lint_rules\":28}}\n", "[]")
	gateWrite(t, overlay, gateSiteTextFile, "{\"/\":\"DossierX\"}\n")
	for _, surface := range []string{"exported-skills", "release-notes", "changelog"} {
		for _, rel := range gateStage2Artifacts(surface) {
			gateWrite(t, overlay, rel, "{\"captured\":\""+surface+"\"}\n")
		}
	}
	return overlay, realRoot
}

// gateStage2StampRun writes a run manifest that honestly records what the
// overlay holds, so freshness passes and the assertions above it are about
// something else.
func gateStage2StampRun(t *testing.T, root, tree string, declared []string) {
	t.Helper()
	var run gateStage2Run
	run.Tree = tree
	run.Baseline.Ref = gateStage2FixtureRef
	run.Baseline.Commit = gateStage2FixtureBaseline
	for _, rel := range gateStage2ProducedArtifacts(declared) {
		digest, err := gateStage2FileDigest(root, rel)
		if err != nil {
			t.Fatalf("digest %s: %v", rel, err)
		}
		run.Artifacts = append(run.Artifacts, struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		}{Path: rel, SHA256: digest})
	}
	raw, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		t.Fatalf("marshal the run manifest: %v", err)
	}
	gateWrite(t, root, gateStage2RunFile, string(raw)+"\n")
}

// ---------------------------------------------------------------------
// the tests: the tool grant
// ---------------------------------------------------------------------

// TestGateStage2GrantIsTheExactSetTheHarnessPasses is the lane's central
// defence, asserted at all three points it can be broken.
//
// Before this, nothing in the tree declared the surface agents' model id or tool
// list, and the ONLY gateMethod value in the repository was a fixture granting
// {"Bash", "Grep", "Read"} — precisely the file-reading tools the design
// forbids, in the value an implementer would copy.
func TestGateStage2GrantIsTheExactSetTheHarnessPasses(t *testing.T) {
	root := surfaceRepoRoot(t)

	// 1. The declaration is the pinned set, exactly. Not a superset.
	declared, err := gateStage2LoadDeclaration(root)
	if err != nil {
		t.Fatalf("load %s: %v", gateStage2MethodFile, err)
	}
	got := append([]string(nil), declared.Tools...)
	sort.Strings(got)
	want := append([]string(nil), gateStage2PinnedGrant...)
	sort.Strings(want)
	if !gateEqualStrings(got, want) {
		t.Errorf("%s grants %v; the pinned grant is %v. Widening it must cost a deliberate edit here that a human reads, not a line in a config nobody diffs", gateStage2MethodFile, got, want)
	}

	// 2. The pinned set itself holds nothing that can reach a byte. This is the
	// assertion that fires when someone widens the grant and edits the literal
	// to match, which the equality above cannot see.
	for _, tool := range gateStage2PinnedGrant {
		if gateStage2ReachesBytes(tool) {
			t.Errorf("the grant holds %q. A surface agent that can reach a byte the assembler did not hand it makes every key in this gate a key over an unknown input set", tool)
		}
	}

	// 3. The harness passes that same set, as an exclusive allow-list, with the
	// model the declaration names. This is the value the runner ACTUALLY passes
	// rather than a copy living in a test.
	grant, err := gateStage2Harness(t, "grant")
	if err != nil {
		t.Fatalf("harness grant: %v", err)
	}
	if !gateEqualStrings(gateStage2Lines(grant), want) {
		t.Errorf("the harness passes %v and %s declares %v", gateStage2Lines(grant), gateStage2MethodFile, want)
	}

	command, err := gateStage2Harness(t, "command", "--surface", "site", "--bundle", "/tmp/bundle.txt")
	if err != nil {
		t.Fatalf("harness command: %v", err)
	}
	if !strings.Contains(command, "--allowed-tools "+strings.Join(want, ",")) {
		t.Errorf("the harness invocation does not pass the grant as an exclusive allow-list:\n%s", command)
	}
	if !strings.Contains(command, "--model "+declared.Model) {
		t.Errorf("the harness invocation does not pass the declared model %q:\n%s", declared.Model, command)
	}
	if strings.Contains(command, "--disallowed-tools") {
		t.Errorf("the harness screens by DENY LIST. A list of named bad tools is walked past by the next name anybody invents — Task, WebFetch, mcp__filesystem__read_file — and the screen stays green while it happens:\n%s", command)
	}
}

// TestGateStage2TheForbiddenClassesAreTheReachingTools puts the question the
// exact-set assertion cannot: does the second lock still name the tools that
// reach bytes?
//
// gateStage2ForbiddenGrant could be emptied to []string{} with the whole suite
// green, because the only names it was ever asked about were the two granted
// ones and neither is on it. An empty list is a screen that approves everything,
// and it is one line of the same edit that widens the grant — so a widening
// becomes three green edits rather than one red one. Here the screen is asked
// about each reaching tool by name, and an emptied list fails on the first row.
func TestGateStage2TheForbiddenClassesAreTheReachingTools(t *testing.T) {
	// Each of these reaches a byte the assembler did not hand over, and each is
	// a way this gate's central defence has actually been lost somewhere: the
	// file tools directly, Task by delegating to an agent that has them,
	// WebFetch by leaving the machine, and the MCP prefix by arriving under a
	// name no deny list had heard of when it was written.
	for _, tool := range []string{
		"Bash", "Edit", "Glob", "Grep", "NotebookEdit", "Read", "Task",
		"WebFetch", "WebSearch", "Write",
		"mcp__filesystem__read_file", "mcp__github__get_file_contents",
	} {
		if !gateStage2ReachesBytes(tool) {
			t.Errorf("the forbidden classes no longer catch %q. A grant holding it makes every key in this gate a key over an input set nobody recorded, and the exact-set assertion cannot see it: widening the declaration and editing the pinned literal to match leaves the two agreeing", tool)
		}
	}

	// And it is not a screen that refuses everything, which would be the other
	// way to make the rows above pass while asserting nothing: the two granted,
	// report-only tools must get through it.
	for _, tool := range gateStage2PinnedGrant {
		if gateStage2ReachesBytes(tool) {
			t.Errorf("the screen rejects %q, which is one of the two tools the agent must have to answer at all; a screen that rejects everything proves nothing about the rows above", tool)
		}
	}
}

// TestGateStage2HarnessReadsItsDeclarationRatherThanCarryingACopy kills the
// mutant the test above cannot see.
//
// Every assertion up there compares the harness's output against the real
// gate/method.yaml and the real surfaces.yaml — and a harness with the answers
// hardcoded satisfies all of them, today, forever. So this points the harness at
// a DIFFERENT checkout with a different declaration and a different manifest and
// requires it to follow. A hardcoded harness cannot.
func TestGateStage2HarnessReadsItsDeclarationRatherThanCarryingACopy(t *testing.T) {
	root := t.TempDir()
	gateWrite(t, root, gateStage2MethodFile, "model: some-other-model\ntools:\n  - OnlyThisOne\n")
	gateWrite(t, root, gateManifestFile, "surfaces:\n"+
		"  - name: alpha\n    paths: [alpha/]\n"+
		"  - name: beta\n    paths: [beta/]\n"+
		"out_of_scope:\n"+
		"  - name: tests\n    paths: [\"**/*_test.go\"]\n")

	if got, err := gateStage2Harness(t, "model", "--root", root); err != nil || got != "some-other-model" {
		t.Errorf("the harness answered %q (%v) for a checkout declaring some-other-model; it is carrying a copy rather than reading the declaration", got, err)
	}
	grant, err := gateStage2Harness(t, "grant", "--root", root)
	if err != nil || !gateEqualStrings(gateStage2Lines(grant), []string{"OnlyThisOne"}) {
		t.Errorf("the harness passed %v (%v) for a checkout granting only OnlyThisOne", gateStage2Lines(grant), err)
	}
	surfaces, err := gateStage2Harness(t, "surfaces", "--root", root)
	if err != nil || !gateEqualStrings(gateStage2Lines(surfaces), []string{"alpha", "beta"}) {
		t.Errorf("the harness fanned out over %v (%v) for a manifest declaring alpha and beta; a written-down thirteen would report green over thirteen on the day a fourteenth surface is declared", gateStage2Lines(surfaces), err)
	}

	// And the fan-out follows the manifest when it GROWS, with nothing else
	// edited. That is the whole proviso gateIsGreen's fourteenth-surface
	// refusal depends on.
	gateWrite(t, root, gateManifestFile, "surfaces:\n"+
		"  - name: alpha\n    paths: [alpha/]\n"+
		"  - name: beta\n    paths: [beta/]\n"+
		"  - name: gamma\n    paths: [gamma/]\n"+
		"out_of_scope:\n"+
		"  - name: tests\n    paths: [\"**/*_test.go\"]\n")
	surfaces, err = gateStage2Harness(t, "surfaces", "--root", root)
	if err != nil || !gateEqualStrings(gateStage2Lines(surfaces), []string{"alpha", "beta", "gamma"}) {
		t.Errorf("a surface added to the manifest did not enter the fan-out; got %v (%v)", gateStage2Lines(surfaces), err)
	}
}

// TestGateStage2HarnessRefusesWhatItCannotDerive: an empty grant and an empty
// fan-out are both refused rather than passed on as a smaller run.
func TestGateStage2HarnessRefusesWhatItCannotDerive(t *testing.T) {
	root := t.TempDir()
	gateWrite(t, root, gateStage2MethodFile, "model: some-model\ntools:\n")
	gateWrite(t, root, gateManifestFile, "out_of_scope:\n  - name: tests\n    paths: [\"**/*_test.go\"]\n")

	if out, err := gateStage2Harness(t, "grant", "--root", root); err == nil {
		t.Errorf("the harness passed an empty grant (%q); an agent that can call nothing produces no findings, and thirteen silent agents read as thirteen clean passes", out)
	}
	if out, err := gateStage2Harness(t, "surfaces", "--root", root); err == nil {
		t.Errorf("the harness fanned out over a manifest declaring no surfaces (%q); that is a pass over zero assertions", out)
	}

	// The Go-side reader refuses the same two shapes, because the plan and the
	// harness must not disagree about what counts as a usable declaration.
	if _, err := gateStage2LoadDeclaration(root); err == nil {
		t.Error("gateStage2LoadDeclaration accepted a declaration with no tools")
	}
	gateWrite(t, root, gateStage2MethodFile, "model:\ntools:\n  - SurfaceVerdict\n")
	if _, err := gateStage2LoadDeclaration(root); err == nil {
		t.Error("gateStage2LoadDeclaration accepted a declaration with no model; the same prompt on a different model is a different question, and an unnamed one is not a question at all")
	}
}

// ---------------------------------------------------------------------
// the tests: the run's shape, against the REAL tree
// ---------------------------------------------------------------------

// TestGateStage2ComputesAKeyForEveryDeclaredSurfaceOfTheRealTree is the
// structural gap this lane exists to close.
//
// Thirteen surfaces have been declared since surfaces.yaml landed and thirteen
// keys had never been computed. TestGateFanOutComesFromTheManifest reads the
// real manifest but only resolves document SETS; every fingerprint assertion ran
// against gateFingerprintFixture's synthetic tree, and fixtures hold regular
// files. The real tree does not: `exported-skills` resolves to five symlinks to
// directories, os.ReadFile on one returns "is a directory", and the landed key
// could not be computed for that surface at all.
func TestGateStage2ComputesAKeyForEveryDeclaredSurfaceOfTheRealTree(t *testing.T) {
	overlay, realRoot := gateStage2Overlay(t)
	tracked := surfaceTrackedFiles(t, realRoot)

	declared, err := gateDeclaredSurfaces(overlay)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	gateStage2StampRun(t, overlay, gateStage2FixtureTree, declared)

	keys, err := gateStage2Plan(overlay, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the real thirteen could not be planned:\n%v", err)
	}
	if len(keys) != len(declared) {
		t.Fatalf("%d surfaces are declared and %d keys were computed", len(declared), len(keys))
	}

	// Every key distinct: two surfaces sharing a key would carry each other's
	// verdicts, and it is the shape a fingerprint over nothing but the shared
	// evidence produces.
	byKey := map[string]string{}
	for _, surface := range declared {
		key := keys[surface]
		if key == "" {
			t.Errorf("surface %q got no key", surface)
			continue
		}
		if other, clash := byKey[key]; clash {
			t.Errorf("surfaces %q and %q fingerprint identically; each would carry the other's verdict", other, surface)
		}
		byKey[key] = surface
	}

	// And the whole point: an edit to ONE surface's documents re-runs one agent.
	// `readme` is chosen because its document set is a single tracked file.
	before := keys["readme"]
	gateWrite(t, overlay, "README.md", "# DossierX\n\nan edit that changes what the front door says\n")
	after, err := gateStage2Keys(overlay, tracked)
	if err != nil {
		t.Fatalf("re-key after the edit: %v", err)
	}
	if after["readme"] == before {
		t.Error("editing README.md did not move the `readme` key")
	}
	moved := 0
	for _, surface := range declared {
		if after[surface] != keys[surface] {
			moved++
		}
	}
	if moved != 1 {
		t.Errorf("a one-document edit moved %d of %d keys; the cache buys nothing if every fix re-runs every agent", moved, len(declared))
	}
}

// TestGateStage2RefusesToProceedWhenAnySurfaceKeyCannotBeComputed is the "never
// narrow coverage silently" rule at the point it would actually be broken.
//
// The concrete history: twelve of thirteen surfaces resolved to readable
// documents and the thirteenth did not, and the repair reached for under time
// pressure — skip a document that will not open — leaves the run reporting green
// over twelve while surfaces.yaml declares thirteen.
func TestGateStage2RefusesToProceedWhenAnySurfaceKeyCannotBeComputed(t *testing.T) {
	overlay, realRoot := gateStage2Overlay(t)
	tracked := surfaceTrackedFiles(t, realRoot)
	declared, err := gateDeclaredSurfaces(overlay)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	gateStage2StampRun(t, overlay, gateStage2FixtureTree, declared)

	// Take away exactly one surface's question, inside the overlay only: the
	// symlinked prompts directory is replaced by a real copy missing one file,
	// so nothing touches the real repository. Twelve keys stay perfectly
	// computable, which is the whole shape of the failure.
	realPrompts, err := os.ReadDir(filepath.Join(realRoot, "gate", "prompts"))
	if err != nil {
		t.Fatalf("read the prompts: %v", err)
	}
	if err := os.Remove(filepath.Join(overlay, "gate", "prompts")); err != nil {
		t.Fatalf("unlink the overlay's prompts: %v", err)
	}
	dropped := 0
	for _, entry := range realPrompts {
		if entry.Name() == "roadmap.md" {
			dropped++
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(realRoot, "gate", "prompts", entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		gateWrite(t, overlay, "gate/prompts/"+entry.Name(), string(data))
	}
	if dropped != 1 {
		t.Fatalf("the fixture dropped %d prompts, not the one it means to; there is no gate/prompts/roadmap.md to remove and this test would be asserting over nothing", dropped)
	}

	keys, err := gateStage2Keys(overlay, tracked)
	if err == nil {
		t.Fatal("twelve of thirteen surfaces were keyed and the run carried on; the thirteenth would hold no verdict and the report would read green over twelve")
	}
	if keys != nil {
		t.Errorf("a PARTIAL key map was returned alongside the error (%d entries). A caller that forgets the error reports on what it got, and \"we did not check the thirteenth\" reaches nobody", len(keys))
	}
	if !strings.Contains(err.Error(), "roadmap") {
		t.Errorf("the refusal must name the surface that could not be keyed; got:\n%v", err)
	}
	if !strings.Contains(err.Error(), "of 13") {
		t.Errorf("the refusal must say how many of how many, so a reader can tell one uncomputable surface from a broken run; got:\n%v", err)
	}
}

// TestGateStage2ACaptureReachesOneSurfaceKeyAndNoOther is what carries failure 6
// now that the fingerprint has no separate artifacts component.
//
// The three per-surface captures — the skills export, the release-notes
// prediction, the cross-release render diff — are written by flag-driven entry
// points a driver has to remember to run, and the export capture skips itself
// when its flag is unset. A run that does not produce them hands over either
// last release's file or nothing, and the PASS is recorded against a key that is
// perfectly current for this tree.
//
// Two things have to be true and they pull against each other. The capture's
// bytes must be IN the surface's key, or a stale capture sits under a
// carried-forward verdict. And they must be in NO other surface's key, or the
// cache buys nothing: every release that re-renders the site would re-run all
// thirteen agents. This is also the assertion that would notice an assembler
// that stopped handing a capture over — which is the risk of having no separate
// component for them, named and covered rather than left implicit.
func TestGateStage2ACaptureReachesOneSurfaceKeyAndNoOther(t *testing.T) {
	overlay, realRoot := gateStage2Overlay(t)
	tracked := surfaceTrackedFiles(t, realRoot)

	declared, err := gateDeclaredSurfaces(overlay)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	before, err := gateStage2Keys(overlay, tracked)
	if err != nil {
		t.Fatalf("key the real thirteen: %v", err)
	}

	for _, surface := range declared {
		captures := gateStage2Artifacts(surface)
		if len(captures) == 0 {
			continue
		}
		for _, rel := range captures {
			t.Run(surface+" ← "+rel, func(t *testing.T) {
				original, readErr := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(rel)))
				if readErr != nil {
					t.Fatalf("read %s: %v", rel, readErr)
				}
				gateWrite(t, overlay, rel, "{\"captured\":\"a different release\"}\n")
				defer gateWrite(t, overlay, rel, string(original))

				after, keyErr := gateStage2Keys(overlay, tracked)
				if keyErr != nil {
					t.Fatalf("re-key: %v", keyErr)
				}
				if after[surface] == before[surface] {
					t.Errorf("replacing %s left %q's key unmoved. The agent is handed that file as this release's evidence, so last release's copy would sit under a verdict carried forward as current", rel, surface)
				}
				for _, other := range declared {
					if other == surface {
						continue
					}
					if after[other] != before[other] {
						t.Errorf("replacing %s moved %q's key as well; that capture is evidence for %q alone, and a key that moves for every surface re-runs thirteen agents for one file", rel, other, surface)
					}
				}
			})
		}
	}

	// And a capture that is not there refuses the run rather than shrinking it.
	// The assembler cannot build the bundle without it, so no key exists to
	// carry anything forward.
	t.Run("a capture the surface reads is missing", func(t *testing.T) {
		rel := gateStage2Artifacts("exported-skills")[0]
		original, readErr := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(rel)))
		if readErr != nil {
			t.Fatalf("read %s: %v", rel, readErr)
		}
		if err := os.Remove(filepath.Join(overlay, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("remove %s: %v", rel, err)
		}
		defer gateWrite(t, overlay, rel, string(original))

		keys, keyErr := gateStage2Keys(overlay, tracked)
		if keyErr == nil {
			t.Fatal("every surface was keyed with one surface's own capture absent; that verdict would rest on an evidence set the run never had")
		}
		if keys != nil {
			t.Errorf("a partial key map came back alongside the error (%d entries)", len(keys))
		}
		if !strings.Contains(keyErr.Error(), rel) {
			t.Errorf("the refusal must name the missing capture; got:\n%v", keyErr)
		}
	})
}

// TestGateStage2EveryClaimedDocumentIsAccountedForInItsBundle is the assertion
// that the bundle's own account of the surface is complete.
//
// A bundle either hands a document over or names it as withheld. A document that
// appears in neither is a file the agent is not told about and cannot say its
// reading depended on — the site surface's 47 sources going unmentioned would
// leave the agent believing the rendered text was the whole surface.
func TestGateStage2EveryClaimedDocumentIsAccountedForInItsBundle(t *testing.T) {
	overlay, realRoot := gateStage2Overlay(t)
	tracked := surfaceTrackedFiles(t, realRoot)

	declared, err := gateDeclaredSurfaces(overlay)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	documents, err := gateSurfaceDocuments(overlay, tracked)
	if err != nil {
		t.Fatalf("resolve documents: %v", err)
	}

	for _, surface := range declared {
		spec, err := gateStage2BundleSpec(surface, documents[surface])
		if err != nil {
			t.Errorf("surface %q: %v", surface, err)
			continue
		}
		bundle, err := gateBundleAssemble(overlay, spec)
		if err != nil {
			t.Errorf("surface %q: %v", surface, err)
			continue
		}
		text := string(bundle)
		var missing []string
		for _, rel := range documents[surface] {
			if !strings.Contains(text, rel) {
				missing = append(missing, rel)
			}
		}
		if len(missing) > 0 {
			t.Errorf("surface %q's bundle accounts for neither the bytes nor the name of %d of its %d documents:\n  %s",
				surface, len(missing), len(documents[surface]), strings.Join(missing, "\n  "))
		}
	}
}

// TestGateStage2EverySurfaceNamedInAPolicyIsDeclared: the two per-surface
// policies are keyed by surface NAME, and a name that no longer exists in the
// manifest is a policy that silently applies to nothing.
//
// Concretely: rename `exported-skills` and its export capture stops being in any
// key, stops being refused when absent, and stops being handed to any agent —
// with every other test in this file still green.
func TestGateStage2EverySurfaceNamedInAPolicyIsDeclared(t *testing.T) {
	root := surfaceRepoRoot(t)
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	isDeclared := map[string]bool{}
	for _, name := range declared {
		isDeclared[name] = true
	}

	for _, named := range []string{"exported-skills", "release-notes", "changelog"} {
		if !isDeclared[named] {
			t.Errorf("gateStage2Artifacts attaches a capture to surface %q, which surfaces.yaml does not declare; that capture is in no key and reaches no agent", named)
		}
		if len(gateStage2Artifacts(named)) == 0 {
			t.Errorf("surface %q is named here as reading a capture and gateStage2Artifacts returns none for it", named)
		}
	}
	for _, named := range []string{"site", "binary-and-viewer"} {
		if !isDeclared[named] {
			t.Errorf("gateStage2HandsAnExtract names surface %q, which surfaces.yaml does not declare", named)
		}
		if !gateStage2HandsAnExtract(named) {
			t.Errorf("surface %q is named here as receiving an extract and gateStage2HandsAnExtract says otherwise", named)
		}
	}
	// And no surface reads a capture unless it is named above: an artifact
	// attached to a surface nobody listed would be refused-when-absent with
	// nothing explaining why.
	for _, name := range declared {
		artifacts := gateStage2Artifacts(name)
		switch name {
		case "exported-skills", "release-notes", "changelog":
			if len(artifacts) == 0 {
				t.Errorf("surface %q lost its capture", name)
			}
		default:
			if len(artifacts) != 0 {
				t.Errorf("surface %q reads capture(s) %v that this test does not know about; every per-surface artifact needs a stated reader", name, artifacts)
			}
		}
	}
}

// TestGateStage2TheExportedSkillsSurfaceIsLinksAndNotACopy pins the one thing
// that makes that surface worth reviewing at all.
//
// surfaces.yaml declares it as "the skills export output, checked in as symlinks
// so this repository's own agents read exactly the bytes a client's agents
// would". Replace one link with a copied directory and the surface still
// resolves, still fingerprints and still gets an agent — but it is now a frozen
// duplicate that diverges from skills/ silently, which is precisely the
// divergence the surface exists to catch.
func TestGateStage2TheExportedSkillsSurfaceIsLinksAndNotACopy(t *testing.T) {
	root := surfaceRepoRoot(t)
	documents, err := gateSurfaceDocuments(root, surfaceTrackedFiles(t, root))
	if err != nil {
		t.Fatalf("resolve documents: %v", err)
	}
	claimed := documents["exported-skills"]
	if len(claimed) == 0 {
		t.Fatal("the `exported-skills` surface resolved to no document")
	}
	for _, rel := range claimed {
		info, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if statErr != nil {
			t.Errorf("%s: %v", rel, statErr)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is %s, not a symlink. This surface is declared as the export checked in AS LINKS so this repository's agents read exactly the bytes a client's would; a copy is a frozen duplicate that diverges from skills/ with nothing to notice", rel, info.Mode().Type())
		}
	}

	// And every other surface's documents are readable regular files or links —
	// there is no third shape the gate has a reading of.
	for surface, rels := range documents {
		for _, rel := range rels {
			info, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
			if statErr != nil {
				t.Errorf("surface %q: %s: %v", surface, rel, statErr)
				continue
			}
			if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("surface %q's document %s is %s, which the gate has no reading of", surface, rel, info.Mode().Type())
			}
		}
	}
}

// ---------------------------------------------------------------------
// the tests: freshness
// ---------------------------------------------------------------------

// TestGateStage2FreshnessRefusesAnInputThisRunDidNotProduce walks every way an
// input can be present and wrong.
//
// The defeat this closes is one line long: `mkdir -p gate && printf '{}' >
// gate/delta.json` before a run. The hand-written file hashes cleanly into all
// thirteen keys, every key is current for the tree, and the verdicts are
// recorded and carried forward. gateSurfaceFingerprint's refusal fires on a
// MISSING file and never on a wrong one, so the missing half is here.
func TestGateStage2FreshnessRefusesAnInputThisRunDidNotProduce(t *testing.T) {
	tree := gateStage2FixtureTree
	declared := []string{"changelog", "exported-skills", "readme", "release-notes"}

	fixture := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		gateStage2WriteEvidence(t, root, tree, "{\"counts\":{}}\n", "[]")
		gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX\"}\n")
		for _, surface := range declared {
			for _, rel := range gateStage2Artifacts(surface) {
				gateWrite(t, root, rel, "{\"captured\":true}\n")
			}
		}
		gateStage2StampRun(t, root, tree, declared)
		return root
	}

	// Honest fixture first: a run that produced everything it covers passes.
	if _, err := gateStage2CheckFreshness(fixture(t), tree, declared); err != nil {
		t.Fatalf("a run whose manifest honestly records everything it produced was refused, so the rows below prove nothing: %v", err)
	}

	// EVERY ARTIFACT IN THE PRODUCED SET, ONE AT A TIME, IN BOTH DIRECTIONS.
	// The list is written out here rather than taken from
	// gateStage2ProducedArtifacts, because a table driven by the function under
	// test cannot notice the function dropping a member: removing
	// gate/baseline.json from the produced set is exactly the mutation that used
	// to leave the suite green, and a derived table would have shrunk with it.
	// The consequence of that mutation is concrete — the previous release's
	// resolved baseline sits in all thirteen keys and passes freshness — so
	// every member is nailed down individually.
	for _, rel := range []string{
		gateBaselineFile,
		gateDeltaFile,
		gateSiteTextFile,
		"gate/export-output.json",
		"gate/release-notes-prediction.json",
		"gate/render-diff.json",
	} {
		t.Run("left from a previous run: "+rel, func(t *testing.T) {
			root := fixture(t)
			// Stamped first, rewritten after: internally consistent, current for
			// the tree, and not what this run produced.
			gateWrite(t, root, rel, "{\"left\":\"from the last release\"}\n")
			_, err := gateStage2CheckFreshness(root, tree, declared)
			if err == nil {
				t.Fatalf("%s was replaced after the run recorded it and freshness passed; that file is in every key it belongs to and the verdicts over it carry forward", rel)
			}
			if !strings.Contains(err.Error(), rel) || !strings.Contains(err.Error(), "not the file this run produced") {
				t.Errorf("the refusal must name %s and say what is wrong with it; got:\n%v", rel, err)
			}
		})

		t.Run("never recorded as produced at all: "+rel, func(t *testing.T) {
			root := fixture(t)
			gateStage2DropFromManifest(t, root, rel)
			_, err := gateStage2CheckFreshness(root, tree, declared)
			if err == nil {
				t.Fatalf("%s is in the keys and the run does not claim to have produced it, and freshness passed; found on disk is not produced", rel)
			}
			if !strings.Contains(err.Error(), rel) || !strings.Contains(err.Error(), "found on disk rather than produced") {
				t.Errorf("the refusal must name %s; got:\n%v", rel, err)
			}
		})
	}

	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, root string)
		want   string
	}{
		{"nothing says these artifacts were produced at all", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateStage2RunFile))); err != nil {
				t.Fatalf("remove the manifest: %v", err)
			}
		}, gateStage2RunFile},
		{"the evidence was produced against another tree", func(t *testing.T, root string) {
			gateStage2StampRun(t, root, strings.Repeat("c", 40), declared)
		}, "different tree"},
		{"the baseline was never resolved", func(t *testing.T, root string) {
			gateStage2RewriteManifest(t, root, gateStage2FixtureBaseline, "")
		}, "no resolved baseline"},
		// A run manifest is a FILE — hand-written, half-written, or edited after
		// the fact — so the 40-hex rule the producer enforces has to be enforced
		// again by whoever reads it. Until it was, a manifest naming the TAG
		// passed freshness, and `git tag -f` re-points an annotated tag under
		// anything that names only the tag.
		{"the baseline is a tag name rather than an identity", func(t *testing.T, root string) {
			gateStage2RewriteManifest(t, root, gateStage2FixtureBaseline, "v0.5.0")
		}, "not a full object name"},
		{"the baseline is an abbreviated object name", func(t *testing.T, root string) {
			gateStage2RewriteManifest(t, root, gateStage2FixtureBaseline, "3217a48")
		}, "not a full object name"},
		{"the baseline is forty characters of something else", func(t *testing.T, root string) {
			gateStage2RewriteManifest(t, root, gateStage2FixtureBaseline, strings.Repeat("z", 40))
		}, "not a full object name"},
		{"the run does not say which tree it covers", func(t *testing.T, _ string) {}, "not told which tree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			tc.mutate(t, root)
			covering := tree
			if tc.name == "the run does not say which tree it covers" {
				covering = ""
			}
			_, err := gateStage2CheckFreshness(root, covering, declared)
			if err == nil {
				t.Fatal("the run was allowed to fingerprint over evidence it cannot claim to have produced")
			}
			if !errors.Is(err, errGateStage2NotProduced) {
				t.Errorf("the refusal must be the not-produced sentinel; got %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal must say what is wrong; want a mention of %q, got:\n%v", tc.want, err)
			}
		})
	}

	// A surface whose capture is missing altogether is refused too, and named.
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "gate", "export-output.json")); err != nil {
		t.Fatalf("remove the capture: %v", err)
	}
	_, err := gateStage2CheckFreshness(root, tree, declared)
	if err == nil || !strings.Contains(err.Error(), "export-output.json") {
		t.Errorf("a per-surface capture the run recorded and then lost was not reported by name; got %v", err)
	}
}

// gateStage2Restamp re-records the run manifest over whatever is on disk now,
// for the tree the fixtures use.
//
// It is the laundering step, and it is in the fixtures on purpose: `record`
// re-digests what it finds, so a row that edits an artifact and then re-stamps
// is showing a run whose every digest is honest. That is the state the checks
// have to refuse, and a row that skipped the re-stamp would be caught by
// freshness instead and would prove nothing about the check it names.
func gateStage2Restamp(t *testing.T, root string) {
	t.Helper()
	gateStage2StampRunFor(t, root, gateStage2FixtureTree)
}

// gateStage2StampRunFor re-records the manifest for a named tree, deriving the
// surfaces from the root's own manifest.
func gateStage2StampRunFor(t *testing.T, root, tree string) {
	t.Helper()
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	gateStage2StampRun(t, root, tree, declared)
}

// gateStage2DropFromManifest removes one artifact's entry from gate/run.json,
// leaving the file on disk. That is what an artifact the run did not produce
// looks like: present, readable, and claimed by nobody.
func gateStage2DropFromManifest(t *testing.T, root, rel string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateStage2RunFile)))
	if err != nil {
		t.Fatalf("read the manifest: %v", err)
	}
	var run gateStage2Run
	if err := json.Unmarshal(raw, &run); err != nil {
		t.Fatalf("parse the manifest: %v", err)
	}
	kept := run.Artifacts[:0]
	dropped := 0
	for _, a := range run.Artifacts {
		if a.Path == rel {
			dropped++
			continue
		}
		kept = append(kept, a)
	}
	if dropped != 1 {
		t.Fatalf("the manifest held %d entries for %s, not the one this is meant to drop; the row below would be asserting over nothing", dropped, rel)
	}
	run.Artifacts = kept
	out, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		t.Fatalf("marshal the manifest: %v", err)
	}
	gateWrite(t, root, gateStage2RunFile, string(out)+"\n")
}

// gateStage2RewriteManifest replaces one literal inside gate/run.json — the
// baseline commit, in practice — the way a hand-edit or a half-finished driver
// would.
func gateStage2RewriteManifest(t *testing.T, root, was, now string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateStage2RunFile)))
	if err != nil {
		t.Fatalf("read the manifest: %v", err)
	}
	if !strings.Contains(string(raw), was) {
		t.Fatalf("%s does not hold %q, so replacing it would change nothing and the row would assert over an untouched fixture", gateStage2RunFile, was)
	}
	gateWrite(t, root, gateStage2RunFile, strings.Replace(string(raw), was, now, 1))
}

// TestGateStage2DistinguishesAnEmptyDeltaFromOneThatWasNeverComputed is the
// distinction this tree makes urgent.
//
// The first gated release's delta over shipped code is legitimately EMPTY:
// `git diff --stat v0.5.0..HEAD -- cmd/ internal/ skills/ go.mod` is six files,
// every one a *_test.go. So "the delta must be non-empty" is red on day one and
// is the wrong assertion. What must never be conflated is empty with absent or
// unresolvable — and the failure mode is specific: a resolver keyed on `git show
// <prev-tag>:surface.json` failing cannot tell "this release predates the
// emitter" from "this clone has no tags", and either way the honest answer is
// FAILED rather than a delta that happens to look clean.
func TestGateStage2DistinguishesAnEmptyDeltaFromOneThatWasNeverComputed(t *testing.T) {
	commit := strings.Repeat("b", 40)

	t.Run("an empty delta from a resolved baseline is a legitimate answer", func(t *testing.T) {
		root := t.TempDir()
		gateWrite(t, root, gateDeltaFile, "{\"baseline\":{\"ref\":\"v0.5.0\",\"commit\":\""+commit+"\"},\"changed\":[]}\n")
		delta, err := gateStage2ReadDelta(root)
		if err != nil {
			t.Fatalf("an empty delta computed against a resolved baseline was refused; on this tree that is the CORRECT answer for the first gated release: %v", err)
		}
		if delta.Changed == nil || len(*delta.Changed) != 0 {
			t.Errorf("the empty delta read back as %v", delta.Changed)
		}
	})

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"absent", "", "absent"},
		{"unparseable", "not json at all\n", "does not parse"},
		{"resolved nothing", "{\"baseline\":{\"ref\":\"v0.5.0\",\"commit\":\"\"},\"changed\":[]}\n", "names no baseline commit"},
		{"a bare object, which is what a hand-written stub looks like", "{}\n", "names no baseline commit"},
		// A tag is a mutable pointer and an abbreviation is a prefix. Either can
		// be written into a delta by hand or by a resolver that answered with
		// what it was given, and either means the comparison cannot say which
		// release it was against.
		{"a tag name where an identity belongs", "{\"baseline\":{\"ref\":\"v0.5.0\",\"commit\":\"v0.5.0\"},\"changed\":[]}\n", "not a full object name"},
		{"an abbreviated object name", "{\"baseline\":{\"ref\":\"v0.5.0\",\"commit\":\"3217a48\"},\"changed\":[]}\n", "not a full object name"},
		{"a baseline but no comparison", "{\"baseline\":{\"ref\":\"v0.5.0\",\"commit\":\"" + commit + "\"}}\n", "carries no `changed` list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.body != "" {
				gateWrite(t, root, gateDeltaFile, tc.body)
			}
			_, err := gateStage2ReadDelta(root)
			if err == nil {
				t.Fatal("a delta that compared nothing was accepted; it would hash into all thirteen keys and every verdict would rest on it")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want a mention of %q, got:\n%v", tc.want, err)
			}
			// And an unresolvable baseline is reported as ITSELF rather than as
			// an empty comparison, because the two send a reader to opposite
			// ends: one to the tags, one to the diff.
			if (strings.Contains(tc.want, "baseline commit") || strings.Contains(tc.want, "full object name")) && !errors.Is(err, errGateStage2UnresolvedBaseline) {
				t.Errorf("an unresolved baseline was not reported as one; got %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------
// the tests: the plan, which is the only path any of this is on
// ---------------------------------------------------------------------

// gateStage2PlanFixture is a small, complete, HONEST run: two surfaces, a
// method, a frame, two prompts, an inventory, a resolved baseline, a delta that
// describes this tree against that baseline, the rendered site text, and a run
// manifest that records every one of them.
//
// Small on purpose. The real-tree tests above exist to prove thirteen keys are
// computable over the actual repository; these exist to prove the run REFUSES,
// and each row needs a fixture it can break in exactly one place.
func gateStage2PlanFixture(t *testing.T) (root string, tracked []string) {
	t.Helper()
	root = t.TempDir()

	gateWrite(t, root, gateManifestFile, "surfaces:\n"+
		"  - name: readme\n    paths: [README.md]\n"+
		"  - name: roadmap\n    paths: [docs/ROADMAP.md]\n"+
		"out_of_scope:\n  - name: tests\n    paths: [\"**/*_test.go\"]\n")
	gateWrite(t, root, "README.md", "# DossierX\n\nthe front door\n")
	gateWrite(t, root, "docs/ROADMAP.md", "# Roadmap\n\nwhat is next\n")

	gateWrite(t, root, gateStage2MethodFile, "model: claude-opus-5\ntools:\n  - SurfaceFinding\n  - SurfaceVerdict\n")
	gateWrite(t, root, gateBundleFrameFile, "# Surface review — "+gateBundleSurfaceMarker+"\n\n"+
		"Report FAILED on any mismatch you can demonstrate from the material below.\n\n"+gateBundlePartsMarker+"\n")
	gateWrite(t, root, gateBundlePromptFile("readme"), "Read the README against surface.json.\n")
	gateWrite(t, root, gateBundlePromptFile("roadmap"), "Read the roadmap against surface.json.\n")

	gateWrite(t, root, gateSurfaceInventoryFile, "{\"counts\":{\"lint_rules\":28}}\n")
	gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX\"}\n")
	gateStage2WriteEvidence(t, root, gateStage2FixtureTree, "{\"counts\":{\"lint_rules\":27}}\n", "[\"counts\"]")

	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	gateStage2StampRun(t, root, gateStage2FixtureTree, declared)

	return root, []string{"README.md", "docs/ROADMAP.md"}
}

// TestGateStage2PlanRefusesEveryRunItCannotStandBehind drives the WHOLE
// precondition, not its parts.
//
// WHY THIS TEST HAD TO EXIST. Freshness and the delta reader were each tested
// thoroughly in isolation — and both could be DELETED from gateStage2Plan with
// the suite still green, because the only test that called the plan drove it
// over an honest fixture where neither could fire. A check that is well tested
// and never called is a check that does not run, and gateStage2Plan is the one
// place either of them gates anything. So every refusal below goes through the
// plan, and the assertions read the message rather than only the fact of an
// error: a row that reddens for some other reason would prove the wrong thing.
func TestGateStage2PlanRefusesEveryRunItCannotStandBehind(t *testing.T) {
	// The honest fixture plans. Every row below is therefore the guard rather
	// than a fixture that could never have worked.
	root, tracked := gateStage2PlanFixture(t)
	keys, err := gateStage2Plan(root, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("an honest run was refused, so every row below proves nothing: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("the fixture declares 2 surfaces and %d keys were computed", len(keys))
	}

	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, root string)
		want   string
	}{
		// ---- freshness, reached through the plan --------------------------
		{"the run produced nothing it can name", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateStage2RunFile))); err != nil {
				t.Fatalf("remove the manifest: %v", err)
			}
		}, "Every artifact the keys cover is a per-run file"},
		{"the evidence was produced against another tree", func(t *testing.T, root string) {
			declared, err := gateDeclaredSurfaces(root)
			if err != nil {
				t.Fatalf("declared: %v", err)
			}
			gateStage2StampRun(t, root, strings.Repeat("c", 40), declared)
		}, "produced against a different tree"},
		{"the run manifest names a tag rather than a resolved commit", func(t *testing.T, root string) {
			gateStage2RewriteManifest(t, root, gateStage2FixtureBaseline, "v0.5.0")
		}, "not a full object name"},
		{"the site text was replaced after the run recorded it", func(t *testing.T, root string) {
			gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX, from last release\"}\n")
		}, "not the file this run produced"},

		// ---- the delta's own state, reached through the plan ---------------
		// The delta the run says it produced is not there. Freshness reaches
		// this before the reader does, and that ordering is deliberate: an
		// artifact the manifest names and disk does not hold is a broken run
		// rather than a comparison that came out empty, and the human is sent to
		// the producer. The reader's own absent-delta branch is exercised in
		// TestGateStage2DistinguishesAnEmptyDeltaFromOneThatWasNeverComputed.
		{"the delta the run recorded is gone", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateDeltaFile))); err != nil {
				t.Fatalf("remove the delta: %v", err)
			}
		}, "is recorded as produced and cannot be read"},
		{"whatever is at that path is not a delta", func(t *testing.T, root string) {
			gateWrite(t, root, gateDeltaFile, "this is not json\n")
			gateStage2Restamp(t, root)
		}, "is not a delta"},
		{"a delta that resolved a baseline and compared nothing", func(t *testing.T, root string) {
			gateWrite(t, root, gateDeltaFile, "{\"tree\":\""+gateStage2FixtureTree+"\",\"baseline\":{\"ref\":\""+gateStage2FixtureRef+"\",\"commit\":\""+gateStage2FixtureBaseline+"\"}}\n")
			gateStage2Restamp(t, root)
		}, "carries no `changed` list"},

		// ---- the agreement between them, which is the whole failure --------
		//
		// This is the ordinary driver sequence: a gate FAILS, a fix lands, the
		// tree moves, the driver re-runs the captures and `record` but not
		// `delta`. Every artifact is honestly digested against the new tree and
		// the delta describes the old one.
		{"the delta describes the release before the fix", func(t *testing.T, root string) {
			gateStage2WriteEvidence(t, root, strings.Repeat("d", 40), "{\"counts\":{\"lint_rules\":27}}\n", "[\"counts\"]")
			gateStage2Restamp(t, root)
		}, "describes what moved in some other release"},
		{"the run and its own delta disagree about the baseline", func(t *testing.T, root string) {
			gateStage2RewriteManifest(t, root, gateStage2FixtureBaseline, strings.Repeat("e", 40))
		}, "disagree about which release is the past"},
		{"the delta and the run disagree about the baseline's name", func(t *testing.T, root string) {
			gateStage2RewriteManifest(t, root, gateStage2FixtureRef, "v0.4.9")
		}, "names baseline ref"},
		{"the baseline inventory was re-resolved and the delta was not recomputed", func(t *testing.T, root string) {
			// The bytes in every key move; the summary of the comparison does
			// not. Neither file is stale on its own account and the pair is
			// incoherent — which is what a re-resolved baseline with an
			// un-recomputed delta actually looks like.
			gateWrite(t, root, gateBaselineFile, "{\"counts\":{\"lint_rules\":25}}\n")
			gateStage2Restamp(t, root)
		}, "the keys would carry bytes the comparison never saw"},
		{"the delta does not say which baseline bytes it read", func(t *testing.T, root string) {
			gateWrite(t, root, gateDeltaFile, "{\"tree\":\""+gateStage2FixtureTree+"\",\"baseline\":{\"ref\":\""+gateStage2FixtureRef+"\",\"commit\":\""+gateStage2FixtureBaseline+"\"},\"changed\":[]}\n")
			gateStage2Restamp(t, root)
		}, "does not record the digest of the baseline inventory"},

		// ---- and the keys themselves --------------------------------------
		{"one surface's question is gone", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateBundlePromptFile("roadmap")))); err != nil {
				t.Fatalf("remove the prompt: %v", err)
			}
		}, "of 2 declared surfaces could not be fingerprinted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, tracked := gateStage2PlanFixture(t)
			tc.mutate(t, root)
			keys, err := gateStage2Plan(root, gateStage2FixtureTree, tracked)
			if err == nil {
				t.Fatal("the run was planned anyway; thirteen agents would be fanned out over evidence the run cannot account for, and every verdict they gave would be recorded against a key that looks perfectly current for this tree")
			}
			if keys != nil {
				t.Errorf("a partial key map came back alongside the error (%d entries); a caller that forgets the error reports on what it got", len(keys))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal must say what is wrong in terms a human can act on; want a mention of %q, got:\n%v", tc.want, err)
			}
		})
	}
}

// TestGateStage2AStaleDeltaIsRefusedEvenWhenEveryDIGESTAgrees is failure 5's
// end-to-end shape, driven through the REAL producer rather than through
// hand-written fixtures.
//
// It is the sequence a driver falls into without doing anything unusual: run
// `delta` for one tree, land a fix, then re-run the captures and `record` for
// the new tree but not `delta`. `record` re-digests whatever is on disk, so the
// stale delta is laundered into a manifest that is honest about every byte it
// names. Nothing is inconsistent except the one thing that matters.
func TestGateStage2AStaleDeltaIsRefusedEvenWhenEveryDIGESTAgrees(t *testing.T) {
	root, tracked := gateStage2PlanFixture(t)
	moved := strings.Repeat("d", 40)

	// The fix lands: surface.json moves, and so does the tree.
	gateWrite(t, root, gateSurfaceInventoryFile, "{\"counts\":{\"lint_rules\":29}}\n")
	gateStage2StampRunFor(t, root, moved)

	// Every digest in the manifest is honest about the file it names, and the
	// manifest is current for the tree being released.
	if _, err := gateStage2CheckFreshness(root, moved, []string{"readme", "roadmap"}); err != nil {
		t.Fatalf("the laundered run is supposed to look perfectly fresh; if it does not, this test is not exercising the failure it names: %v", err)
	}

	_, err := gateStage2Plan(root, moved, tracked)
	if err == nil {
		t.Fatal("thirteen agents would have been handed a delta computed against a different tree as the truth about what this release changed — " +
			"and on a docs-only follow-up, where surface.json does not move, every key would be identical to the previous run's and every PASS would carry forward too")
	}
	if !errors.Is(err, errGateStage2StaleDelta) {
		t.Errorf("a stale delta must be reported as itself, so a reader is sent to the producer rather than to the tags or the diff; got %v", err)
	}
}

// ---------------------------------------------------------------------
// the tests: the producer
// ---------------------------------------------------------------------

// TestGateStage2DeltaProducerResolvesOrFails exercises the REAL producer.
//
// Before it existed, gate/delta.json was a named component of all thirteen keys
// that nothing in the repository wrote — so the freshness half of the invariant
// could only ever be demonstrated against a stub of the lane's own invention.
func TestGateStage2DeltaProducerResolvesOrFails(t *testing.T) {
	commit := strings.Repeat("b", 40)
	tree := gateStage2FixtureTree

	fixture := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		gateWrite(t, root, gateManifestFile, "surfaces:\n  - name: readme\n    paths: [README.md]\n")
		gateWrite(t, root, gateSurfaceInventoryFile, "{\n  \"commands\": [\n    \"claim new\"\n  ],\n  \"counts\": {\n    \"lint_rules\": 28\n  }\n}\n")
		gateWrite(t, root, "baseline-source.json", "{\n  \"commands\": [\n    \"claim new\"\n  ],\n  \"counts\": {\n    \"lint_rules\": 28\n  }\n}\n")
		return root
	}

	t.Run("an unchanged tree produces an EMPTY delta against a resolved baseline", func(t *testing.T) {
		root := fixture(t)
		if _, err := gateStage2Harness(t, "delta", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"--baseline-file", filepath.Join(root, "baseline-source.json")); err != nil {
			t.Fatalf("the producer refused an unchanged tree: %v", err)
		}
		delta, err := gateStage2ReadDelta(root)
		if err != nil {
			t.Fatalf("read the produced delta: %v", err)
		}
		if len(*delta.Changed) != 0 {
			t.Errorf("an unchanged inventory produced a non-empty delta: %v", *delta.Changed)
		}
		if delta.Baseline.Commit != commit {
			t.Errorf("the delta records baseline %q, want %q", delta.Baseline.Commit, commit)
		}
		// The baseline's BYTES are written out, because a projection never
		// stands in for its source inside the key.
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateBaselineFile))); err != nil {
			t.Errorf("the producer did not write the resolved baseline's bytes: %v", err)
		}
	})

	t.Run("a moved inventory names the field that moved", func(t *testing.T) {
		root := fixture(t)
		gateWrite(t, root, gateSurfaceInventoryFile, "{\n  \"commands\": [\n    \"claim new\"\n  ],\n  \"counts\": {\n    \"lint_rules\": 29\n  }\n}\n")
		if _, err := gateStage2Harness(t, "delta", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"--baseline-file", filepath.Join(root, "baseline-source.json")); err != nil {
			t.Fatalf("the producer failed: %v", err)
		}
		delta, err := gateStage2ReadDelta(root)
		if err != nil {
			t.Fatalf("read the produced delta: %v", err)
		}
		if !gateEqualStrings(*delta.Changed, []string{"counts"}) {
			t.Errorf("the delta reports %v changed; a delta that says only \"the inventory moved\" tells thirteen agents nothing they could act on", *delta.Changed)
		}
	})

	// EVERY UNRESOLVED SHAPE, AGAINST BOTH MODES THAT TAKE ONE.
	//
	// The rows used to be `delta`-only and used to stop at three, all of which
	// the length check alone catches — so the character-class half of the guard
	// could be deleted and forty characters of anything at all was accepted with
	// nothing to notice. `record` had no rows whatsoever: its own copy of the
	// guard could be deleted entirely and the suite stayed green, which put a
	// tag name into the run manifest with only the reading side left to object.
	for _, commit := range []string{
		"",                      // nothing was resolved at all
		"v0.5.0",                // a tag: a mutable pointer, re-pointable by `git tag -f`
		"3217a48",               // an abbreviation: a prefix, not an identity
		strings.Repeat("z", 40), // the right LENGTH and not an object name
		strings.Repeat("g", 20) + "0123456789abcdef0123", // hex-looking and not hex
		"HEAD~1",                      // a revision expression, which resolves differently every day
		" " + strings.Repeat("b", 39), // whitespace where a digit belongs
		strings.Repeat("B", 40),       // upper case: git spells object names in lower
	} {
		name := commit
		if name == "" {
			name = "(empty)"
		}
		t.Run("delta refuses baseline "+name, func(t *testing.T) {
			root := fixture(t)
			if _, err := gateStage2Harness(t, "delta", "--root", root, "--tree", tree,
				"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
				"--baseline-file", filepath.Join(root, "baseline-source.json")); err == nil {
				t.Fatal("the producer accepted a baseline it had not resolved; an unresolvable baseline is a FAILED run, never an empty delta")
			}
			for _, rel := range []string{gateDeltaFile, gateBaselineFile} {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
					t.Errorf("%s was written for a baseline that was never resolved; downstream it is indistinguishable from a clean comparison", rel)
				}
			}
		})

		t.Run("record refuses baseline "+name, func(t *testing.T) {
			root := fixture(t)
			gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX\"}\n")
			if _, err := gateStage2Harness(t, "record", "--root", root, "--tree", tree,
				"--baseline-ref", "v0.5.0", "--baseline-commit", commit, gateSiteTextFile); err == nil {
				t.Fatal("a run manifest was written naming a baseline this run never resolved; every key computed under it would be attached to a release nobody can identify")
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateStage2RunFile))); err == nil {
				t.Error("a run manifest was written anyway")
			}
		})
	}

	// The producer's own account of the baseline BYTES. Without it the delta is
	// a summary and gate/baseline.json is a file, and nothing says they are
	// about each other.
	t.Run("the delta records the digest of the baseline it read", func(t *testing.T) {
		root := fixture(t)
		if _, err := gateStage2Harness(t, "delta", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"--baseline-file", filepath.Join(root, "baseline-source.json")); err != nil {
			t.Fatalf("the producer failed: %v", err)
		}
		delta, err := gateStage2ReadDelta(root)
		if err != nil {
			t.Fatalf("read the produced delta: %v", err)
		}
		digest, err := gateStage2FileDigest(root, gateBaselineFile)
		if err != nil {
			t.Fatalf("digest the produced baseline: %v", err)
		}
		if delta.Baseline.SHA256 != digest {
			t.Errorf("the delta records baseline digest %q and %s hashes %q; a comparison that cannot name its own source leaves thirteen keys carrying bytes nobody checked",
				delta.Baseline.SHA256, gateBaselineFile, digest)
		}
		if delta.Tree != tree {
			t.Errorf("the delta records tree %q, want %q", delta.Tree, tree)
		}
	})

	// THE LAUNDERING STEP, REFUSED AT THE POINT IT HAPPENS. `record` re-digests
	// whatever is on disk, so a delta left from before a fix is written into a
	// manifest that is honest about every byte it names. The reading side
	// refuses it; so does this side, because the driver sequence that produces
	// it — gate FAILS, fix lands, re-run the captures and `record` but not
	// `delta` — is ordinary rather than adversarial, and the earlier something
	// says so the less of the run is wasted.
	t.Run("record refuses to launder a delta computed over another tree", func(t *testing.T) {
		root := fixture(t)
		if _, err := gateStage2Harness(t, "delta", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"--baseline-file", filepath.Join(root, "baseline-source.json")); err != nil {
			t.Fatalf("the producer failed: %v", err)
		}
		// The fix lands and the tree moves; the delta is not recomputed.
		moved := strings.Repeat("d", 40)
		if _, err := gateStage2Harness(t, "record", "--root", root, "--tree", moved,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			gateBaselineFile, gateDeltaFile); err == nil {
			t.Fatal("a delta computed over another tree was recorded as this run's; every digest in that manifest is honest and the comparison under all thirteen keys is about a different release")
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateStage2RunFile))); err == nil {
			t.Error("a run manifest was written anyway")
		}

		// And the same delta recorded for the tree it was actually computed over
		// is accepted — otherwise the row above would pass on a `record` that
		// refuses everything.
		if _, err := gateStage2Harness(t, "record", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			gateBaselineFile, gateDeltaFile); err != nil {
			t.Fatalf("record refused an honest run: %v", err)
		}
	})

	t.Run("record refuses a delta computed against another baseline", func(t *testing.T) {
		root := fixture(t)
		if _, err := gateStage2Harness(t, "delta", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"--baseline-file", filepath.Join(root, "baseline-source.json")); err != nil {
			t.Fatalf("the producer failed: %v", err)
		}
		if _, err := gateStage2Harness(t, "record", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", strings.Repeat("e", 40),
			gateBaselineFile, gateDeltaFile); err == nil {
			t.Fatal("a delta computed against one baseline was recorded under a run that resolved another")
		}
	})

	t.Run("a baseline document that cannot be read is a FAILED run", func(t *testing.T) {
		root := fixture(t)
		if _, err := gateStage2Harness(t, "delta", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"--baseline-file", filepath.Join(root, "there-is-no-such-file.json")); err == nil {
			t.Fatal("the producer accepted a baseline document it could not read")
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateDeltaFile))); err == nil {
			t.Error("a delta was written with no baseline document to compare against")
		}
	})

	t.Run("the run manifest refuses to record an artifact that is not there", func(t *testing.T) {
		root := fixture(t)
		if _, err := gateStage2Harness(t, "record", "--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"gate/delta.json"); err == nil {
			t.Fatal("the producer recorded an artifact this run never produced")
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateStage2RunFile))); err == nil {
			t.Error("a run manifest was written naming an artifact that is not there")
		}
	})
}

// sha256Sum is a one-line helper kept beside its only caller.
func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
