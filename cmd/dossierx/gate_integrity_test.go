// gate_integrity_test.go is the gate-integrity fingerprint: the digest of the
// gate's OWN definition, hashed into every surface's key.
//
// THE FAILURE THIS CLOSES. Every other input to a surface key is covered
// somewhere: the documents through the manifest resolution, the shared evidence
// by name, the prompts and the model through method_version, the framing and
// the captures through the bundle. The gate's own rules were covered nowhere.
// The verdict predicate (gateReceipt.evaluate, gateIsGreen), the carry-forward
// rule (gatePlanRerun), the bundle assembler, the freshness and delta checks,
// the harness that passes the tool grant — all of it lives in `*_test.go` files
// and a shell script, which is to say outside behaviour_fingerprint (non-test
// shipped sources only), outside every surface's documents (surfaces.yaml puts
// `**/*_test.go` and scripts/gate-stage2/ out of scope), and outside the method
// (prompts, model, tools). So weakening `evaluate()` so that a failed surface
// no longer blocks moved not one byte of any key: every surface carried its
// previous PASS forward, CI ran the weakened predicate, and the release went
// out green, gated by rules nobody re-read. Measured before this file existed:
// 0 of 13 keys moved on exactly that edit.
//
// THE REPAIR IS THE SAME ONE THE FRAME ALREADY HAS, for the same reason.
// Changing the frame prompt re-keys every surface because the question changed;
// changing what the gate IS — what blocks, what carries forward, what is
// assembled, what is fresh — changes the question too, one level up. So the
// gate's definition is hashed into every key, and any edit to it forces a full
// re-read: the previous verdicts were produced under rules that no longer
// exist, and a verdict is a function of the rules as much as of the evidence.
//
// WHAT IS IN THE SET, and where the boundary is. Four patterns, resolved
// against `git ls-files` (the tracked set every caller already carries):
//
//	cmd/dossierx/gate_*_test.go   — the gate's implementation: the fingerprint,
//	                                the verdict predicate, carry-forward, the
//	                                bundle assembler, freshness, the delta
//	                                checks, the fan-out, the ledger and
//	                                baseline freezes, the driver, this file.
//	cmd/dossierx/surface*_test.go — the inventory emitter and its meta-tests.
//	                                surface.json's BYTES are already shared
//	                                evidence, but bytes cover only what the
//	                                emitter says today: weakening a refusal, or
//	                                narrowing an extraction that happens to
//	                                emit identical output on this tree, moves
//	                                no byte of surface.json — and licenses the
//	                                inventory's future silence. The emitter is
//	                                part of what the gate IS.
//	scripts/gate-stage2/          — the harness: the tool grant's exclusive
//	                                allow-list, the delta and record steps, the
//	                                fan-out driver. reads: already hands its
//	                                bytes to the release-procedure agent, but a
//	                                borrow re-keys one surface; an edit to the
//	                                harness changes the machinery every verdict
//	                                rests on, so it must re-key all of them.
//	surfaces.yaml                 — the inventory of what the gate covers.
//	                                Most edits to it already move keys through
//	                                the document resolution, but not all:
//	                                deleting a whole surface shrinks the
//	                                fan-out while every remaining key stays
//	                                put, and the gate would go green over
//	                                twelve carried-forward PASSes with nobody
//	                                re-reading the coverage decision.
//
// WHAT IS DELIBERATELY OUT, and what covers each exclusion instead:
//
//	gate/method.yaml and gate/prompts/  — the QUESTION, already keyed by the
//	  routes built for it: the model id and tool list through method_version
//	  (an edit moves every key), each surface's prompt through its own
//	  method_version (an edit re-keys that surface, by design — re-running
//	  thirteen agents over one reworded question is the cost the per-surface
//	  route exists to avoid), and the frame through every bundle (an edit
//	  moves every key). Folding them in here would not widen coverage; it
//	  would only collapse the per-surface prompt route into a global one and
//	  make the double-coverage impossible to reason about. What this leaves
//	  uncovered is exactly the comment text of method.yaml — prose the parse
//	  ignores and no agent reads.
//	gate/*.json (baseline, delta, run, captures) — per-run EVIDENCE, not
//	  definition; already in the keys through the shared evidence and the
//	  bundles, and guarded by freshness. None of it is tracked, so the
//	  ls-files derivation cannot pick it up by accident.
//	tests/*_capture_test.go, viewer-tests/site_dom_test.go — the capture
//	  producers. Their OUTPUT is in the key of every surface that reads it,
//	  byte for byte, so a weakened producer produces moved bytes and a
//	  re-read. (A producer weakened so subtly that its output is
//	  byte-identical today is not caught — the same residue stated for the
//	  emitter above, accepted here because the capture freshness checks in
//	  scripts/gate-stage2/run.sh and gate_stage2_test.go are themselves in
//	  this set.)
//	tests/surfaces_manifest_test.go — repo-CI enforcement of the manifest's
//	  exactly-one rule. The gate's own reading of the manifest is
//	  gate_fingerprint_test.go's copy, which is in the set; the cross-check
//	  holding the two readings together (TestGateManifestPatternsAgreeWithTheTree)
//	  lives in a covered file.
//	docs/RELEASING.md — the procedure is a gated SURFACE with its own agent;
//	  editing it re-keys release-procedure through the documents component.
//
// WHAT THIS FINGERPRINT DOES NOT PROTECT, stated so nobody leans on it. It
// makes a weakening LOUD, not impossible. An edit to the rules re-keys every
// surface and forces a full re-read — under the edited rules. Anyone with push
// rights can still change what blocks and then re-run the gate governed by the
// changed predicate; the re-read guarantees the change costs a full fan-out
// and lands in the diff a human reviews, and guarantees nothing more. Only a
// forge-side control — review required on these paths, enforced outside this
// tree — closes that, and no test in this repository can stand in for it. Nor
// can this digest see the RUNTIME: whether the harness's requested tool grant
// was honoured, or what answers to the model id, is outside the tree, exactly
// as gate/method.yaml states about itself.
//
// Same shape as the rest of the gate files: test code, not a cobra command,
// not compiled into the shipped binary, outside behaviour_fingerprint — and,
// because its own name matches the first pattern above, inside the very digest
// it computes: editing this file re-keys every surface too, so the boundary
// itself cannot be redrawn quietly.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// the definition's file set
// ---------------------------------------------------------------------

// gateIntegrityPatterns is the gate's definition, as patterns over the tracked
// set — the same grammar surfaces.yaml documents and gateMatchPattern
// implements, so there is one reading of what a pattern means.
//
// PATTERNS, NOT A HAND LIST. A hand list is how five shipped skill bundles once
// left behaviour_fingerprint with every test green: the file that joins is the
// file nobody remembers to add. A gate_*_test.go that lands tomorrow is in this
// digest on the day it lands, with no second edit — and
// TestGateIntegrityCoversEveryTrackedGateFile re-derives the membership from
// `git ls-files` in a second spelling so that a narrowed pattern here cannot
// certify itself.
var gateIntegrityPatterns = []string{
	"cmd/dossierx/gate_*_test.go",
	"cmd/dossierx/surface*_test.go",
	"scripts/gate-stage2/",
	"surfaces.yaml",
}

// gateIntegrityFiles resolves the patterns against tracked, refusing any
// pattern that claims nothing.
//
// THE PER-PATTERN REFUSAL IS THE POINT. A pattern matching zero files is what a
// directory rename looks like from here — scripts/gate-stage2/ moves to
// scripts/gate2/ and the harness silently leaves the digest, which is the exact
// narrowing this whole mechanism exists to make loud. The digest helpers all
// err this way and say why: a value that hashes cleanly over a smaller set is
// stable, looks like a match, and carries verdicts forward across exactly the
// changes it exists to notice.
func gateIntegrityFiles(tracked []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, pattern := range gateIntegrityPatterns {
		matched := 0
		for _, file := range tracked {
			if !gateMatchPattern(pattern, file) {
				continue
			}
			matched++
			if !seen[file] {
				seen[file] = true
				out = append(out, file)
			}
		}
		if matched == 0 {
			return nil, fmt.Errorf("gate integrity: pattern %q claims no tracked file. The gate's definition has moved out from under its own fingerprint — a rename, most likely — and hashing what is left would carry every verdict forward across whatever now lives at the new path", pattern)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("gate integrity: the definition resolved to no files at all; a digest over nothing is a constant, and a constant carries every verdict forward forever")
	}
	// gateIntegrityFingerprint sorts again inside hashRepoFiles; sorting here
	// too keeps the list a canonical value for callers that only want the set.
	return out, nil
}

// gateIntegrityFingerprint is the digest of the gate's definition: sha256 over
// the resolved files' paths, lengths and bytes, domain-tagged so it can never
// be mistaken for a surface fingerprint or a method_version computed over
// overlapping bytes.
//
// It fails on any unreadable input rather than hashing what it could reach,
// for hashRepoFiles' own reason — the stream is hashRepoFiles' exactly, so the
// gate's definition and the machine contract are measured by the one hash
// function this repository has.
func gateIntegrityFingerprint(root string, tracked []string) (string, error) {
	files, err := gateIntegrityFiles(tracked)
	if err != nil {
		return "", err
	}
	inner, err := hashRepoFiles(root, files)
	if err != nil {
		return "", fmt.Errorf("gate integrity: %w", err)
	}
	h := sha256.New()
	fmt.Fprintf(h, "dossierx-gate-integrity\x00v1\x00%s", inner)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ---------------------------------------------------------------------
// test fixtures
// ---------------------------------------------------------------------

// gateIntegrityStandIns writes one stand-in file per integrity pattern into a
// synthetic root and returns their tracked paths.
//
// Every synthetic fixture that computes surface keys needs these, because the
// gate's definition is now an input to every key and an input that is missing
// is a refusal, never a shorter hash. surfaces.yaml is not written here — every
// fixture already writes its own manifest — but its path is in the returned
// list because the fixture's tracked set must claim it for the pattern to
// resolve.
func gateIntegrityStandIns(t *testing.T, root string) []string {
	t.Helper()
	gateWrite(t, root, "cmd/dossierx/gate_rules_test.go", "package main\n\n// a stand-in for the gate's own rules\n")
	gateWrite(t, root, "cmd/dossierx/surface_test.go", "package main\n\n// a stand-in for the inventory emitter\n")
	gateWrite(t, root, "scripts/gate-stage2/run.sh", "#!/usr/bin/env bash\n# a stand-in for the harness\n")
	return []string{
		"cmd/dossierx/gate_rules_test.go",
		"cmd/dossierx/surface_test.go",
		"scripts/gate-stage2/run.sh",
		gateManifestFile,
	}
}

// gateIntegrityOverlay is gateStage2Overlay with the two directories the
// headline test must EDIT — cmd/ and scripts/ — replaced by real copies, so a
// mutation touches the copy and never the repository. Everything else keeps
// the overlay's shape: top-level files copied, other directories symlinked.
func gateIntegrityOverlay(t *testing.T) (overlay, realRoot string) {
	t.Helper()
	overlay, realRoot = gateStage2Overlay(t)
	for _, dir := range []string{"cmd", "scripts"} {
		if err := os.Remove(filepath.Join(overlay, dir)); err != nil {
			t.Fatalf("unlink the overlay's %s: %v", dir, err)
		}
		gateIntegrityCopyTree(t, filepath.Join(realRoot, dir), filepath.Join(overlay, dir))
	}
	return overlay, realRoot
}

// gateIntegrityCopyTree deep-copies a tree of regular files. A symlink or any
// other shape is a refusal rather than a follow or a skip: neither cmd/ nor
// scripts/ holds one today, and a copy that silently flattened or dropped one
// would hand the assertions above a tree of a different shape than the one
// they claim to measure.
func gateIntegrityCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil {
			t.Fatalf("stat %s: %v", s, infoErr)
		}
		switch {
		case entry.IsDir():
			gateIntegrityCopyTree(t, s, d)
		case info.Mode().IsRegular():
			data, readErr := os.ReadFile(s)
			if readErr != nil {
				t.Fatalf("read %s: %v", s, readErr)
			}
			if writeErr := os.WriteFile(d, data, info.Mode().Perm()); writeErr != nil {
				t.Fatalf("write %s: %v", d, writeErr)
			}
		default:
			t.Fatalf("%s is %s, which this copy has no reading of; a flattened or dropped entry would change the shape of the tree under test", s, info.Mode().Type())
		}
	}
}

// gateIntegrityRewrite replaces exactly one occurrence of was in the overlay's
// copy of rel, failing if the string is not there exactly once — a mutation
// that lands zero times is a row asserting over an unedited tree, and one that
// lands twice is not the edit the row describes. It returns a restore func.
func gateIntegrityRewrite(t *testing.T, root, rel, was, now string) func() {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if got := strings.Count(string(body), was); got != 1 {
		t.Fatalf("%s holds %d occurrence(s) of %q, not one; the mutation would not be the edit this row describes", rel, got, was)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(body), was, now, 1)), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return func() {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatalf("restore %s: %v", rel, err)
		}
	}
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// TestGateIntegrityEditingTheGateMovesEverySurfaceKey is the assertion this
// file exists for, measured on the run path against the real manifest, the
// real documents and the real gate sources.
//
// Each row is a WEAKENING somebody could actually commit — not a synthetic
// byte flip — and each one, before this fingerprint existed, moved zero keys:
// every surface carried its previous PASS forward and the release went out
// green under rules nobody re-read. The requirement is total: a row that moved
// twelve of thirteen keys would leave one agent answering the old question
// under the new rules, which is the same staleness at surface granularity.
//
// The converse — that ordinary work still carries forward — is not asserted
// here because it already has an owner:
// TestGateStage2ComputesAKeyForEveryDeclaredSurfaceOfTheRealTree requires a
// one-document README edit to move exactly one key, and it runs against the
// same overlay machinery with the integrity component in every key. If this
// fingerprint ever started absorbing per-run or per-commit bytes, that test
// reddens, not this one.
func TestGateIntegrityEditingTheGateMovesEverySurfaceKey(t *testing.T) {
	overlay, realRoot := gateIntegrityOverlay(t)
	tracked := surfaceTrackedFiles(t, realRoot)

	declared, err := gateDeclaredSurfaces(overlay)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	gateStage2StampRun(t, overlay, gateStage2FixtureTree, declared)

	before, err := gateStage2Keys(overlay, tracked)
	if err != nil {
		t.Fatalf("key the real thirteen: %v", err)
	}

	for _, tc := range []struct {
		name string
		rel  string
		was  string
		now  string
	}{
		// The verdict predicate stops consulting the surface coverage: a
		// receipt with failed or missing surfaces evaluates PASS. This is the
		// demonstrated gap, replayed verbatim.
		{
			"the verdict predicate is weakened (gate_receipt_test.go, evaluate)",
			"cmd/dossierx/gate_receipt_test.go",
			"if err := gateIsGreen(declared, r.Surfaces, current); err != nil {",
			"if err := gateIsGreen(declared, r.Surfaces, current); err != nil && false {",
		},
		// Carry-forward stops comparing fingerprints: any previous verdict is
		// reused whatever the tree now says.
		{
			"the carry-forward rule is weakened (gate_fingerprint_test.go, gatePlanRerun)",
			"cmd/dossierx/gate_fingerprint_test.go",
			"case !ok, prev.Fingerprint != fingerprint:",
			"case !ok:",
		},
		// The harness drops the exclusive tool allow-list: the agents could
		// reach bytes the assembler never handed them, and every key would be
		// a key over an unknown input set.
		{
			"the harness drops the exclusive allow-list (scripts/gate-stage2/run.sh)",
			"scripts/gate-stage2/run.sh",
			"--model %s --allowed-tools %s",
			"--model %s",
		},
		// The emitter's walk loses the skills tree — the exact historical
		// failure behaviourRoots' own comment records. surface.json's BYTES do
		// not move on this edit until the next regeneration, which is why the
		// emitter is in this digest at all.
		{
			"the inventory emitter narrows (surface_test.go, behaviourRoots)",
			"cmd/dossierx/surface_test.go",
			`var behaviourRoots = []string{"cmd", "internal", "skills"}`,
			`var behaviourRoots = []string{"cmd", "internal"}`,
		},
		// A comment-only manifest edit: nothing about the resolution changes,
		// so before this fingerprint existed it moved nothing — yet the
		// manifest's comments are where every reads: entry justifies itself,
		// and the file is the gate's whole coverage claim.
		{
			"the coverage manifest is edited (surfaces.yaml, comment only)",
			"surfaces.yaml",
			"surfaces:",
			"# an edit to the gate's coverage claim\nsurfaces:",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := gateIntegrityRewrite(t, overlay, tc.rel, tc.was, tc.now)
			defer restore()

			after, keyErr := gateStage2Keys(overlay, tracked)
			if keyErr != nil {
				t.Fatalf("re-key after the edit: %v", keyErr)
			}
			var unmoved []string
			for _, surface := range declared {
				if after[surface] == before[surface] {
					unmoved = append(unmoved, surface)
				}
			}
			if len(unmoved) > 0 {
				t.Errorf("%s left %d of %d surface keys unmoved (%s); every one of them would carry forward a verdict produced under rules that no longer exist, and the release would go out green under a gate nobody re-read",
					tc.name, len(unmoved), len(declared), strings.Join(unmoved, ", "))
			}
		})
	}
}

// TestGateIntegrityFingerprintMovesWhenTheGateMoves is the synthetic half:
// each row moves exactly one member of the definition and requires the digest
// to move, including the row a hand list can never pass — a file that JOINS.
func TestGateIntegrityFingerprintMovesWhenTheGateMoves(t *testing.T) {
	fixture := func(t *testing.T) (root string, tracked []string) {
		t.Helper()
		root = t.TempDir()
		tracked = gateIntegrityStandIns(t, root)
		gateWrite(t, root, gateManifestFile, "surfaces:\n  - name: docs\n    paths: [docs/]\n")
		// A tracked file the patterns do NOT claim, for the boundary row.
		gateWrite(t, root, "cmd/dossierx/main.go", "package main\n")
		tracked = append(tracked, "cmd/dossierx/main.go")
		return root, tracked
	}

	root, tracked := fixture(t)
	base, err := gateIntegrityFingerprint(root, tracked)
	if err != nil {
		t.Fatalf("fingerprint the fixture: %v", err)
	}
	if again, err := gateIntegrityFingerprint(root, tracked); err != nil || again != base {
		t.Fatalf("two fingerprints of one unchanged definition disagree (%v): %s vs %s", err, base, again)
	}
	// Order independence: the tracked list arrives from git in sorted order,
	// but nothing about the digest may depend on that.
	if got, err := gateIntegrityFingerprint(root, gateReversedStrings(tracked)); err != nil || got != base {
		t.Fatalf("the tracked list's ORDER moved the fingerprint (%v); a reordering would re-run every surface for nothing", err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, root string, tracked *[]string)
	}{
		{"a gate rule file's bytes", func(t *testing.T, root string, _ *[]string) {
			gateWrite(t, root, "cmd/dossierx/gate_rules_test.go", "package main\n\n// the rules, weakened\n")
		}},
		// Same length, different bytes: the stream must carry contents, not
		// just names and sizes — a verdict predicate flipped from `!=` to `==`
		// is a same-length edit.
		{"a same-length edit to a gate rule file", func(t *testing.T, root string, _ *[]string) {
			const was = "package main\n\n// a stand-in for the gate's own rules\n"
			const now = "package main\n\n// a stand-in for the gate's OWN rules\n"
			if len(was) != len(now) {
				t.Fatalf("this row only means anything if the two bodies are the same length: %d vs %d", len(was), len(now))
			}
			gateWrite(t, root, "cmd/dossierx/gate_rules_test.go", now)
		}},
		{"the emitter's bytes", func(t *testing.T, root string, _ *[]string) {
			gateWrite(t, root, "cmd/dossierx/surface_test.go", "package main\n\n// the emitter, narrowed\n")
		}},
		{"the harness's bytes", func(t *testing.T, root string, _ *[]string) {
			gateWrite(t, root, "scripts/gate-stage2/run.sh", "#!/usr/bin/env bash\n# the harness, without its allow-list\n")
		}},
		{"the manifest's bytes", func(t *testing.T, root string, _ *[]string) {
			gateWrite(t, root, gateManifestFile, "surfaces:\n  # a comment\n  - name: docs\n    paths: [docs/]\n")
		}},
		// THE ROW A HAND LIST CANNOT PASS. A new gate_*_test.go — a new rule
		// file — joins the tree and the digest must move with no edit to any
		// list anywhere. This is the whole argument for patterns.
		{"a file joins the gate", func(t *testing.T, root string, tracked *[]string) {
			gateWrite(t, root, "cmd/dossierx/gate_newrule_test.go", "package main\n\n// a rule that did not exist yesterday\n")
			*tracked = append(*tracked, "cmd/dossierx/gate_newrule_test.go")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, tracked := fixture(t)
			before, err := gateIntegrityFingerprint(root, tracked)
			if err != nil {
				t.Fatalf("fingerprint: %v", err)
			}
			tc.mutate(t, root, &tracked)
			after, err := gateIntegrityFingerprint(root, tracked)
			if err != nil {
				t.Fatalf("fingerprint after the edit: %v", err)
			}
			if after == before {
				t.Fatalf("moving %s did not move the gate-integrity fingerprint (%s); a verdict computed under the old rules would be carried forward under the new ones", tc.name, before)
			}
		})
	}

	// And the boundary in the other direction: a shipped source is NOT the
	// gate's definition. It reaches every key already, through surface.json's
	// behaviour_fingerprint and the regeneration test that keeps that current;
	// hashing it here too would re-run all thirteen agents on every engine
	// edit and the carry-forward machinery would never fire again — the
	// tree-stamp defect, rebuilt one layer up.
	t.Run("a shipped source is not the gate", func(t *testing.T) {
		root, tracked := fixture(t)
		before, err := gateIntegrityFingerprint(root, tracked)
		if err != nil {
			t.Fatalf("fingerprint: %v", err)
		}
		gateWrite(t, root, "cmd/dossierx/main.go", "package main\n\n// the engine moved\n")
		after, err := gateIntegrityFingerprint(root, tracked)
		if err != nil {
			t.Fatalf("fingerprint after the edit: %v", err)
		}
		if after != before {
			t.Fatal("editing a shipped source moved the gate-integrity fingerprint; every engine edit would re-key every surface and the cache would never fire")
		}
	})
}

// TestGateIntegrityRefusesEmptyOrUnreadable pins the direction the derivation
// errs in: a definition that cannot be fully resolved or fully read is a
// refusal, never a digest over what was left.
func TestGateIntegrityRefusesEmptyOrUnreadable(t *testing.T) {
	t.Run("a pattern claiming nothing", func(t *testing.T) {
		root := t.TempDir()
		full := gateIntegrityStandIns(t, root)
		gateWrite(t, root, gateManifestFile, "surfaces:\n  - name: docs\n    paths: [docs/]\n")
		// Drop each pattern's matches from the tracked set in turn. This is
		// what a rename looks like: the files exist somewhere, and the pattern
		// no longer names them.
		for _, pattern := range gateIntegrityPatterns {
			var remaining []string
			for _, file := range full {
				if !gateMatchPattern(pattern, file) {
					remaining = append(remaining, file)
				}
			}
			if _, err := gateIntegrityFingerprint(root, remaining); err == nil {
				t.Errorf("a fingerprint was produced with pattern %q claiming nothing; the definition it stopped covering would change under a digest that still matches", pattern)
			} else if !strings.Contains(err.Error(), pattern) {
				t.Errorf("the refusal must name the pattern that went stale; got:\n%v", err)
			}
		}
	})

	t.Run("no tracked files at all", func(t *testing.T) {
		if _, err := gateIntegrityFingerprint(t.TempDir(), nil); err == nil {
			t.Error("a fingerprint was produced over an empty tracked set")
		}
	})

	t.Run("a member that cannot be read", func(t *testing.T) {
		root := t.TempDir()
		tracked := gateIntegrityStandIns(t, root)
		gateWrite(t, root, gateManifestFile, "surfaces:\n  - name: docs\n    paths: [docs/]\n")
		if err := os.Remove(filepath.Join(root, "scripts", "gate-stage2", "run.sh")); err != nil {
			t.Fatalf("remove the harness: %v", err)
		}
		if _, err := gateIntegrityFingerprint(root, tracked); err == nil {
			t.Error("a fingerprint was produced with a member of the definition unreadable; the digest would be stable, look like a match, and cover a harness nobody can read")
		}
	})
}

// TestGateIntegrityCoversEveryTrackedGateFile is the meta-test: the membership
// is re-derived from `git ls-files` in a SECOND SPELLING — raw prefix and
// suffix checks, sharing no code with gateMatchPattern — and the two readings
// must agree in both directions.
//
// The reason for the second spelling is surface_meta_test.go's: a cross-check
// that sources its scope from the thing it is checking narrows in the same
// edit and agrees. If a pattern above is ever read more narrowly than these
// raw checks — or widened until it swallows files that are not the gate — the
// disagreement is named file by file.
func TestGateIntegrityCoversEveryTrackedGateFile(t *testing.T) {
	root := surfaceRepoRoot(t)
	tracked := surfaceTrackedFiles(t, root)

	files, err := gateIntegrityFiles(tracked)
	if err != nil {
		t.Fatalf("resolve the gate's definition: %v", err)
	}
	inSet := map[string]bool{}
	for _, f := range files {
		inSet[f] = true
	}

	// The independent derivation. isGate is the raw spelling of the same four
	// classes; it is deliberately not gateMatchPattern.
	isGate := func(file string) bool {
		switch {
		case strings.HasPrefix(file, "cmd/dossierx/gate_") && strings.HasSuffix(file, "_test.go"):
			return true
		case strings.HasPrefix(file, "cmd/dossierx/surface") && strings.HasSuffix(file, "_test.go"):
			return true
		case strings.HasPrefix(file, "scripts/gate-stage2/"):
			return true
		case file == "surfaces.yaml":
			return true
		}
		return false
	}

	expected := 0
	for _, file := range tracked {
		if !isGate(file) {
			continue
		}
		expected++
		if !inSet[file] {
			t.Errorf("%s is a tracked gate file and is NOT in the gate-integrity fingerprint — an edit to it would move no key, and every surface would carry its verdict forward across a change to the gate's own rules", file)
		}
	}
	if expected == 0 {
		t.Fatal("the raw derivation found no gate files at all; this cross-check is comparing nothing against nothing")
	}
	for _, file := range files {
		if !isGate(file) {
			t.Errorf("%s is in the gate-integrity fingerprint and is not a gate file by the raw derivation; a widened pattern re-keys every surface over edits that are not the gate's", file)
		}
	}

	// The load-bearing members, by name. The two derivations above could agree
	// while both drifting off the files that matter — these are the homes of
	// the verdict predicate, the key itself, the run, the assembler, the
	// emitter, the harness and the coverage claim, and each must be present.
	for _, want := range []string{
		"cmd/dossierx/gate_receipt_test.go",
		"cmd/dossierx/gate_fingerprint_test.go",
		"cmd/dossierx/gate_stage2_test.go",
		"cmd/dossierx/gate_bundle_test.go",
		"cmd/dossierx/gate_integrity_test.go",
		"cmd/dossierx/surface_test.go",
		"cmd/dossierx/surface_meta_test.go",
		"scripts/gate-stage2/run.sh",
		"surfaces.yaml",
	} {
		if !inSet[want] {
			t.Errorf("%s is not in the gate-integrity fingerprint; the gate's definition is being hashed without one of its own load-bearing files", want)
		}
	}

	// And the exclusions hold: nothing under gate/ is in the set. The method
	// and the prompts reach the keys by their own routes (method_version and
	// the bundle), and the per-run evidence is keyed as evidence — folding any
	// of it in here would either double-cover the question in a way nobody can
	// reason about or put per-run bytes in a digest that must move only when
	// the gate's DEFINITION does.
	for _, file := range files {
		if strings.HasPrefix(file, "gate/") {
			t.Errorf("%s is in the gate-integrity fingerprint; gate/ holds the question and the per-run evidence, both already keyed by their own routes", file)
		}
	}
}
