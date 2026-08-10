// gate_bundle_test.go is the assembler: the thing that turns a surface's parts
// into the exact bytes one reading agent is handed, and the demonstrations that
// those bytes are what the cache key is taken over.
//
// WHY THE ASSEMBLER IS ITS OWN OBJECT. The landed key hashed a LIST OF PARTS —
// documents by content, three evidence files by content, prompt sources by
// content. That is a reconstruction of what the agent reads, and a
// reconstruction is not the thing. The text that wraps the parts is not in the
// list: the standing instruction ("report FAILED on any mismatch"), the section
// boundaries, the order, and the sentence telling the agent that a section it
// was NOT handed is still part of its surface. Soften any of it and all thirteen
// agents are asked a materially weaker question while no file the parts list
// names has moved — so no key moves, so every surface carries its previous
// verdict forward and the weakened question is never actually asked of anything.
// The gate's own standard drifts with no diff anyone has to review.
//
// So the digest is over the ASSEMBLED OUTPUT. That covers the frame, the
// assembly code, the ordering, and anything a future part list would forget,
// because there is nothing between the digest and the bytes.
//
// WHAT IT DELIBERATELY DOES NOT COVER, and why that is the point. The bundle
// names the documents it withheld and does NOT carry their bytes. `site`
// resolves to 47 files of TSX/TS source and the agent is handed the rendered DOM
// text; `binary-and-viewer` resolves to 106 files and 1.95 MB and the agent is
// handed the mechanical inventory. If the withheld bytes were folded in here,
// the bundle digest would cover them and gateSurfaceInputs.Documents would be a
// component that can never move on its own — a check that passes either way. The
// two halves of totality are kept as two components on purpose, and
// TestGateBundleWithheldBytesStayOutOfTheBundle is what holds them apart.
//
// Same shape as the rest of the gate files: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// where the framing lives
// ---------------------------------------------------------------------

// gateBundleFrameFile is the text the assembler wraps every surface's parts in:
// what the agent is being asked, the rules it answers under, and how it reports.
//
// IT IS DELIBERATELY NOT ONE OF gateMethod.Prompts. The method is the question
// put to THIS surface — the surface's own prompt, the model, the grant. The
// frame is part of the assembled bytes and reaches the key through the bundle
// digest and through nothing else. Listing it in both places would make the
// bundle component look load-bearing while the method component was doing the
// work, and the mutation that proves the bundle component real would pass with
// it deleted. TestGateBundleFrameReachesTheKeyOnlyThroughTheBundle asserts
// exactly that separation, so it cannot be quietly undone.
const gateBundleFrameFile = "gate/prompts/_frame.md"

// The two placeholders the frame must carry. Both are required: a frame with no
// parts marker wraps nothing, and a frame with no surface marker asks thirteen
// agents an unaddressed question — which is also thirteen identical bundles for
// every surface whose documents happen to be withheld.
const (
	gateBundleSurfaceMarker = "<<SURFACE>>"
	gateBundlePartsMarker   = "<<PARTS>>"
)

// gateBundlePromptFile is one surface's own question.
func gateBundlePromptFile(surface string) string {
	return "gate/prompts/" + surface + ".md"
}

// ---------------------------------------------------------------------
// the bundle
// ---------------------------------------------------------------------

// gateBundlePart is one titled section of the assembled text.
type gateBundlePart struct {
	Title string
	// Source is the repo-relative path the body came from, or "" for a section
	// the assembler synthesized. It is rendered into the output so the agent can
	// say which part of its material a finding rests on.
	Source string
	Body   []byte
}

// gateBundleSpec is what one surface's bundle is built from.
type gateBundleSpec struct {
	Surface string
	// Handed are the surface's documents whose BYTES go into the bundle.
	Handed []string
	// Withheld are the surface's documents that do not: the bundle names them
	// and says they still count. Handed and Withheld together must be the
	// surface's whole manifest-resolved document set — gateStage2BundleSpec is
	// where that totality is enforced, because only there is the resolved set in
	// hand.
	Withheld []string
	// Artifacts are the capture artifacts this surface alone reads.
	Artifacts []string
}

// errGateBundleIncomplete is an assembly the gate will not stand behind.
//
// It is a sentinel because the caller's correct response is never to hash what
// it got: a bundle missing a section is a smaller question, and a smaller
// question with a computable digest is a stale verdict waiting for a re-run.
var errGateBundleIncomplete = errors.New("the bundle could not be assembled completely")

// gateBundleAssemble builds the exact bytes handed to one surface's agent.
//
// Every refusal below is a refusal rather than a shorter bundle, for the reason
// method_version refuses an empty prompt set: a bundle assembled over less than
// it should be still hashes, still looks like a match, and still carries a
// verdict forward.
func gateBundleAssemble(root string, spec gateBundleSpec) ([]byte, error) {
	if strings.TrimSpace(spec.Surface) == "" {
		return nil, fmt.Errorf("%w: no surface name", errGateBundleIncomplete)
	}
	if len(spec.Handed) == 0 && len(spec.Withheld) == 0 {
		return nil, fmt.Errorf("%w: surface %q was assembled over no documents at all, neither handed over nor named as withheld", errGateBundleIncomplete, spec.Surface)
	}
	// A document on both lists is a bundle that hands over bytes while telling
	// the agent it did not. Left unrefused it also double-counts in the output,
	// which makes the "what was withheld" section unreadable.
	handed := map[string]bool{}
	for _, rel := range spec.Handed {
		handed[rel] = true
	}
	for _, rel := range spec.Withheld {
		if handed[rel] {
			return nil, fmt.Errorf("%w: surface %q both hands over and withholds %s", errGateBundleIncomplete, spec.Surface, rel)
		}
	}

	frame, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateBundleFrameFile)))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errGateBundleIncomplete, gateBundleFrameFile, err)
	}
	for _, marker := range []string{gateBundleSurfaceMarker, gateBundlePartsMarker} {
		if !bytes.Contains(frame, []byte(marker)) {
			return nil, fmt.Errorf("%w: %s carries no %s", errGateBundleIncomplete, gateBundleFrameFile, marker)
		}
	}
	// The frame is the standing instruction. A frame that is only its own
	// placeholders hands the agent the parts with no question attached, and
	// every part being present would make that look like a complete bundle.
	stripped := bytes.ReplaceAll(frame, []byte(gateBundleSurfaceMarker), nil)
	stripped = bytes.ReplaceAll(stripped, []byte(gateBundlePartsMarker), nil)
	if len(bytes.TrimSpace(stripped)) == 0 {
		return nil, fmt.Errorf("%w: %s is nothing but its own placeholders, so the agent would be handed the material with no question and no rules", errGateBundleIncomplete, gateBundleFrameFile)
	}

	var parts []gateBundlePart
	add := func(title, rel string) error {
		body, readErr := gateBundleReadDocument(root, rel)
		if readErr != nil {
			return fmt.Errorf("%w: surface %q: %s: %w", errGateBundleIncomplete, spec.Surface, title, readErr)
		}
		if len(bytes.TrimSpace(body)) == 0 {
			return fmt.Errorf("%w: surface %q: %s (%s) is empty; a section with nothing in it is a section the agent cannot read and a bundle that still hashes", errGateBundleIncomplete, spec.Surface, title, rel)
		}
		parts = append(parts, gateBundlePart{Title: title, Source: rel, Body: body})
		return nil
	}

	if err := add("the question", gateBundlePromptFile(spec.Surface)); err != nil {
		return nil, err
	}
	if err := add("the mechanical inventory", gateSurfaceInventoryFile); err != nil {
		return nil, err
	}
	if err := add("the baseline inventory", gateBaselineFile); err != nil {
		return nil, err
	}
	if err := add("the release delta", gateDeltaFile); err != nil {
		return nil, err
	}
	if err := add("the rendered site text", gateSiteTextFile); err != nil {
		return nil, err
	}
	for _, rel := range gateBundleSorted(spec.Artifacts) {
		if err := add("a capture produced for this surface", rel); err != nil {
			return nil, err
		}
	}
	for _, rel := range gateBundleSorted(spec.Handed) {
		if err := add("a document of this surface", rel); err != nil {
			return nil, err
		}
	}
	if len(spec.Withheld) > 0 {
		var b strings.Builder
		b.WriteString("These files are part of this surface and their bytes were NOT handed to you.\n")
		b.WriteString("They decide what the material above says. If your reading depends on one of\n")
		b.WriteString("them, report that you could not check it and name the file — do not assume it\n")
		b.WriteString("agrees with what you were given.\n\n")
		for _, rel := range gateBundleSorted(spec.Withheld) {
			b.WriteString("  " + rel + "\n")
		}
		parts = append(parts, gateBundlePart{Title: "documents of this surface that were not handed over", Body: []byte(b.String())})
	}

	var rendered bytes.Buffer
	for _, part := range parts {
		source := part.Source
		if source == "" {
			source = "assembled by the gate"
		}
		fmt.Fprintf(&rendered, "===== BEGIN %s — %s =====\n", part.Title, source)
		rendered.Write(part.Body)
		if !bytes.HasSuffix(part.Body, []byte("\n")) {
			rendered.WriteByte('\n')
		}
		fmt.Fprintf(&rendered, "===== END %s — %s =====\n\n", part.Title, source)
	}

	out := bytes.ReplaceAll(frame, []byte(gateBundleSurfaceMarker), []byte(spec.Surface))
	out = bytes.ReplaceAll(out, []byte(gateBundlePartsMarker), rendered.Bytes())
	return out, nil
}

// gateBundleReadDocument reads one document's bytes, including the one document
// shape os.ReadFile cannot: a symlink to a directory, which is what all five of
// `exported-skills`' documents are.
//
// The per-file headers are inside the returned bytes rather than added by the
// caller so that a bundle is never ambiguous about which of a linked bundle's
// pages a sentence came from.
func gateBundleReadDocument(root, rel string) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, statErr := os.Stat(path)
		if statErr != nil {
			return nil, fmt.Errorf("the link points at something that cannot be read: %w", statErr)
		}
		if resolved.IsDir() {
			reachable, walkErr := gateReachableFiles(path)
			if walkErr != nil {
				return nil, walkErr
			}
			if len(reachable) == 0 {
				return nil, fmt.Errorf("the link points at a tree holding no file")
			}
			var b bytes.Buffer
			for _, sub := range reachable {
				data, readErr := os.ReadFile(filepath.Join(path, filepath.FromSlash(sub)))
				if readErr != nil {
					return nil, readErr
				}
				fmt.Fprintf(&b, "--- %s/%s ---\n", rel, sub)
				b.Write(data)
				if !bytes.HasSuffix(data, []byte("\n")) {
					b.WriteByte('\n')
				}
			}
			return b.Bytes(), nil
		}
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("it is %s, which the gate has no reading of", info.Mode().Type())
	}
	return os.ReadFile(path)
}

// gateBundleDigest is the bundle's contribution to a surface's key, exposed so a
// test can say which component moved rather than only that the key did.
func gateBundleDigest(bundle []byte) string {
	sum := sha256.Sum256(bundle)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func gateBundleSorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------
// fixture
// ---------------------------------------------------------------------

// gateBundleFixture writes a synthetic tree holding everything one surface's
// bundle is built from, and returns the root and a spec over it. The `site`
// shape is used because it is the one with a real withheld set.
func gateBundleFixture(t *testing.T) (root string, spec gateBundleSpec) {
	t.Helper()
	root = t.TempDir()
	gateWrite(t, root, gateBundleFrameFile, "# Surface review — "+gateBundleSurfaceMarker+"\n\n"+
		"Report FAILED on any mismatch you can demonstrate from the material below.\n\n"+
		gateBundlePartsMarker+"\n")
	gateWrite(t, root, gateBundlePromptFile("site"), "Read the rendered site text against surface.json.\n")
	gateWrite(t, root, gateSurfaceInventoryFile, "{\"counts\":{\"lint_rules\":28}}\n")
	gateWrite(t, root, gateBaselineFile, "{\"counts\":{\"lint_rules\":27}}\n")
	gateWrite(t, root, gateDeltaFile, "{\"changed\":[\"lint_rules\"]}\n")
	gateWrite(t, root, gateSiteTextFile, "{\"/\":\"DossierX v9.9.9 — 28 lint rules\"}\n")
	gateWrite(t, root, "gate/render-diff.json", "{\"artifacts\":[]}\n")
	gateWrite(t, root, "site/src/content.ts", "export const latestVersion = \"v9.9.9\";\n")
	gateWrite(t, root, "site/src/nav.ts", "export const nav = [\"/docs\"];\n")
	gateWrite(t, root, "site/README.md", "how to build the site\n")
	// A declaration of its own, so the tests below can build their method the
	// way the RUN builds it — through gateStage2Method — rather than by writing
	// a gateMethod literal that agrees with the run only by coincidence.
	gateWrite(t, root, gateStage2MethodFile, "model: claude-opus-5\ntools:\n  - SurfaceFinding\n  - SurfaceVerdict\n")

	return root, gateBundleSpec{
		Surface:   "site",
		Handed:    []string{"site/README.md"},
		Withheld:  []string{"site/src/content.ts", "site/src/nav.ts"},
		Artifacts: []string{"gate/render-diff.json"},
	}
}

func gateBundleMust(t *testing.T, root string, spec gateBundleSpec) []byte {
	t.Helper()
	out, err := gateBundleAssemble(root, spec)
	if err != nil {
		t.Fatalf("assemble %s: %v", spec.Surface, err)
	}
	return out
}

// ---------------------------------------------------------------------
// the tests
// ---------------------------------------------------------------------

// TestGateBundleCarriesEveryPartAndNamesWhatItWithheld is the happy path, and it
// is asserted part by part rather than against a golden.
//
// A golden would go green on any assembly that reproduced it and would have to
// be regenerated by the same edit that softened the frame — which is the failure
// this file exists for.
func TestGateBundleCarriesEveryPartAndNamesWhatItWithheld(t *testing.T) {
	root, spec := gateBundleFixture(t)
	got := string(gateBundleMust(t, root, spec))

	for _, want := range []string{
		"# Surface review — site",
		"Report FAILED on any mismatch",
		"BEGIN the question — " + gateBundlePromptFile("site"),
		"BEGIN the mechanical inventory — " + gateSurfaceInventoryFile,
		"BEGIN the baseline inventory — " + gateBaselineFile,
		"BEGIN the release delta — " + gateDeltaFile,
		"BEGIN the rendered site text — " + gateSiteTextFile,
		"BEGIN a capture produced for this surface — gate/render-diff.json",
		"BEGIN a document of this surface — site/README.md",
		"BEGIN documents of this surface that were not handed over",
		"how to build the site",
		"28 lint rules",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the assembled bundle is missing %q", want)
		}
	}
	// The withheld documents are named, and only named.
	for _, rel := range spec.Withheld {
		if !strings.Contains(got, rel) {
			t.Errorf("the bundle does not name withheld document %s, so the agent cannot say its reading depended on one", rel)
		}
	}
	if strings.Contains(got, "export const latestVersion") {
		t.Error("the bundle carries the BYTES of a document it declared withheld; the two halves of the key's coverage have collapsed into one")
	}
	if strings.Contains(got, gateBundlePartsMarker) || strings.Contains(got, gateBundleSurfaceMarker) {
		t.Error("a placeholder survived into the assembled bundle")
	}
}

// TestGateBundleRefusesAnAssemblyItCannotStandBehind walks every way the
// assembly can come out smaller than it should. Each row would otherwise produce
// a perfectly hashable bundle over a question that is missing something.
func TestGateBundleRefusesAnAssemblyItCannotStandBehind(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, root string, spec *gateBundleSpec)
		want   string
	}{
		{"no surface name", func(_ *testing.T, _ string, spec *gateBundleSpec) {
			spec.Surface = ""
		}, "no surface name"},
		{"no documents on either list", func(_ *testing.T, _ string, spec *gateBundleSpec) {
			spec.Handed, spec.Withheld = nil, nil
		}, "no documents at all"},
		{"a document both handed over and withheld", func(_ *testing.T, _ string, spec *gateBundleSpec) {
			spec.Withheld = append(spec.Withheld, "site/README.md")
		}, "both hands over and withholds"},
		{"the frame is gone", func(t *testing.T, root string, _ *gateBundleSpec) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateBundleFrameFile))); err != nil {
				t.Fatalf("remove the frame: %v", err)
			}
		}, gateBundleFrameFile},
		{"the frame lost its parts marker", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, gateBundleFrameFile, "# Surface review — "+gateBundleSurfaceMarker+"\n\nReport FAILED on any mismatch.\n")
		}, gateBundlePartsMarker},
		{"the frame lost its surface marker", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, gateBundleFrameFile, "# Surface review\n\nReport FAILED on any mismatch.\n\n"+gateBundlePartsMarker+"\n")
		}, gateBundleSurfaceMarker},
		{"the frame is nothing but its placeholders", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, gateBundleFrameFile, gateBundleSurfaceMarker+"\n"+gateBundlePartsMarker+"\n")
		}, "nothing but its own placeholders"},
		{"the surface's question is gone", func(t *testing.T, root string, _ *gateBundleSpec) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateBundlePromptFile("site")))); err != nil {
				t.Fatalf("remove the prompt: %v", err)
			}
		}, "the question"},
		{"the release delta is gone", func(t *testing.T, root string, _ *gateBundleSpec) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateDeltaFile))); err != nil {
				t.Fatalf("remove the delta: %v", err)
			}
		}, "the release delta"},
		{"the resolved baseline is gone", func(t *testing.T, root string, _ *gateBundleSpec) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateBaselineFile))); err != nil {
				t.Fatalf("remove the baseline: %v", err)
			}
		}, "the baseline inventory"},
		{"this surface's capture is gone", func(t *testing.T, root string, _ *gateBundleSpec) {
			if err := os.Remove(filepath.Join(root, "gate", "render-diff.json")); err != nil {
				t.Fatalf("remove the capture: %v", err)
			}
		}, "a capture produced for this surface"},
		{"a part is present but empty", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, gateSiteTextFile, "   \n")
		}, "is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, spec := gateBundleFixture(t)
			tc.mutate(t, root, &spec)
			_, err := gateBundleAssemble(root, spec)
			if err == nil {
				t.Fatal("a bundle was assembled anyway; it would hash, look like a match, and carry a verdict forward over a question that is missing a part")
			}
			if !errors.Is(err, errGateBundleIncomplete) {
				t.Errorf("the refusal must be the incomplete-bundle sentinel, so no caller resolves it by hashing what it got; got %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal must name what is missing; want a mention of %q, got:\n%s", tc.want, err)
			}
		})
	}

	// The fixture is honest: untouched, it assembles. Every row above is the
	// guard rather than an assembler that refuses everything.
	root, spec := gateBundleFixture(t)
	if _, err := gateBundleAssemble(root, spec); err != nil {
		t.Fatalf("the untouched fixture would not assemble, so the rows above prove nothing: %v", err)
	}
}

// TestGateBundleDigestMovesWhenTheAssemblyMoves is the assertion that earns the
// component. Every row changes what the agent is ACTUALLY HANDED; a row that did
// not move the digest is a change to the question that no key would notice.
func TestGateBundleDigestMovesWhenTheAssemblyMoves(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, root string, spec *gateBundleSpec)
	}{
		// The named failure: no prompt file edited, no part file moved, all
		// thirteen agents asked a weaker question.
		{"the framing is softened", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, gateBundleFrameFile, "# Surface review — "+gateBundleSurfaceMarker+"\n\n"+
				"Note any mismatches you happen to see in the material below.\n\n"+
				gateBundlePartsMarker+"\n")
		}},
		{"the surface's own question is reworded", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, gateBundlePromptFile("site"), "Skim the rendered site text.\n")
		}},
		{"a section is dropped", func(_ *testing.T, _ string, spec *gateBundleSpec) {
			spec.Artifacts = nil
		}},
		{"a stale artifact is substituted", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, "gate/render-diff.json", "{\"artifacts\":[\"index.html\"]}\n")
		}},
		{"a document moves from withheld to handed over", func(_ *testing.T, _ string, spec *gateBundleSpec) {
			spec.Handed = append(spec.Handed, "site/src/nav.ts")
			spec.Withheld = []string{"site/src/content.ts"}
		}},
		{"a handed-over document's bytes change", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, "site/README.md", "how to build the site, revised\n")
		}},
		{"the shared evidence moves", func(t *testing.T, root string, _ *gateBundleSpec) {
			gateWrite(t, root, gateDeltaFile, "{\"changed\":[]}\n")
		}},
		{"a file joins the surface and is withheld", func(t *testing.T, root string, spec *gateBundleSpec) {
			gateWrite(t, root, "site/src/footer.ts", "export const footer = 1;\n")
			spec.Withheld = append(spec.Withheld, "site/src/footer.ts")
		}},
		{"the surface's name", func(t *testing.T, root string, spec *gateBundleSpec) {
			gateWrite(t, root, gateBundlePromptFile("readme"), "Read the rendered site text against surface.json.\n")
			spec.Surface = "readme"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, spec := gateBundleFixture(t)
			before := gateBundleDigest(gateBundleMust(t, root, spec))
			tc.mutate(t, root, &spec)
			after := gateBundleDigest(gateBundleMust(t, root, spec))
			if after == before {
				t.Fatalf("changing %s left the bundle byte-identical (%s); the agent is being asked a different question and every key in the system says nothing moved", tc.name, before)
			}
		})
	}
}

// TestGateBundleFrameReachesTheKeyOnlyThroughTheBundle is the anti-vacuity
// assertion for the component this file adds.
//
// The obvious way to catch a softened frame is to list the frame file among
// gateMethod.Prompts. Do that and the method digest moves, the key moves, and
// the bundle component in gateSurfaceFingerprint could be deleted with every
// test still green — a component that is present, looks load-bearing, and pins
// nothing. So the separation is asserted directly: editing the frame must leave
// method_version, the documents digest and the shared evidence digest all
// UNCHANGED, and must still move the surface's key.
//
// THE METHOD HERE IS THE RUN'S OWN, built by gateStage2Method rather than by a
// literal. A hand-written literal made this test prove the separation only for a
// value nothing in the run uses: adding the frame to gateStage2Method's prompt
// list left every assertion below green, so the bundle component could be made
// non-load-bearing on the exact path the gate takes. The separation has to be
// asserted where the run reads it.
func TestGateBundleFrameReachesTheKeyOnlyThroughTheBundle(t *testing.T) {
	root, spec := gateBundleFixture(t)
	gateWrite(t, root, "gate/prompts/site.md", "Read the rendered site text against surface.json.\n")

	method, err := gateStage2Method(root, spec.Surface)
	if err != nil {
		t.Fatalf("build the run's own method: %v", err)
	}
	in := gateSurfaceInputs{
		Surface:   spec.Surface,
		Documents: append(append([]string(nil), spec.Handed...), spec.Withheld...),
		Bundle:    gateBundleMust(t, root, spec),
		Method:    method,
	}

	methodBefore, err := in.Method.version(root)
	if err != nil {
		t.Fatalf("method version: %v", err)
	}
	documentsBefore, err := gateHashDocuments(root, in.Documents)
	if err != nil {
		t.Fatalf("hash documents: %v", err)
	}
	evidenceBefore, err := hashRepoFiles(root, gateSharedEvidence())
	if err != nil {
		t.Fatalf("hash evidence: %v", err)
	}
	keyBefore := gateMustFingerprint(t, root, in)

	gateWrite(t, root, gateBundleFrameFile, "# Surface review — "+gateBundleSurfaceMarker+"\n\n"+
		"Note any mismatches you happen to see in the material below.\n\n"+
		gateBundlePartsMarker+"\n")
	in.Bundle = gateBundleMust(t, root, spec)

	methodAfter, err := in.Method.version(root)
	if err != nil {
		t.Fatalf("method version after: %v", err)
	}
	documentsAfter, err := gateHashDocuments(root, in.Documents)
	if err != nil {
		t.Fatalf("hash documents after: %v", err)
	}
	evidenceAfter, err := hashRepoFiles(root, gateSharedEvidence())
	if err != nil {
		t.Fatalf("hash evidence after: %v", err)
	}

	if methodAfter != methodBefore {
		t.Errorf("the frame moved method_version, so it is being counted as one of the method's prompts. "+
			"The bundle component in gateSurfaceFingerprint could then be deleted with every test still green — "+
			"a component that pins nothing.\n before: %s\n  after: %s", methodBefore, methodAfter)
	}
	if documentsAfter != documentsBefore {
		t.Error("the frame moved the documents digest; the frame is not a document of any surface")
	}
	if evidenceAfter != evidenceBefore {
		t.Error("the frame moved the shared evidence digest; the frame is not evidence")
	}
	if got := gateMustFingerprint(t, root, in); got == keyBefore {
		t.Fatalf("softening the frame moved NOTHING in the key (%s). All thirteen agents would be asked a weaker question and every one of them would carry its previous verdict forward", keyBefore)
	}
}

// TestGateBundleWithheldBytesStayOutOfTheBundle holds the two halves of totality
// apart.
//
// The bundle names the documents it withheld; it must not carry them. If it did,
// gateSurfaceInputs.Documents would be a component that can never move while the
// bundle stays put — and the row in
// TestGateSurfaceFingerprintMovesWhenAnyInputMoves that defeats the site-href
// failure would be passing for the wrong reason, which is to say passing either
// way.
func TestGateBundleWithheldBytesStayOutOfTheBundle(t *testing.T) {
	root, spec := gateBundleFixture(t)
	before := gateBundleDigest(gateBundleMust(t, root, spec))

	// Exactly the shape of the release failure: an href re-pointed in
	// site/src/content.ts, which the rendered DOM extractor does not capture.
	gateWrite(t, root, "site/src/content.ts", "export const latestVersion = \"v9.9.9\";\nexport const gitHubURL = \"https://example.invalid/elsewhere\";\n")
	after := gateBundleDigest(gateBundleMust(t, root, spec))

	if after != before {
		t.Fatalf("editing a WITHHELD document moved the bundle, so the bundle is carrying bytes it told the agent it withheld.\n"+
			"That collapses the two halves of the key's coverage into one and makes the documents component untestable.\n before: %s\n  after: %s", before, after)
	}

	// And the key still moves, because the documents component is doing its job.
	method, err := gateStage2Method(root, spec.Surface)
	if err != nil {
		t.Fatalf("build the run's own method: %v", err)
	}
	in := gateSurfaceInputs{
		Surface:   spec.Surface,
		Documents: append(append([]string(nil), spec.Handed...), spec.Withheld...),
		Bundle:    gateBundleMust(t, root, spec),
		Method:    method,
	}
	withEdit := gateMustFingerprint(t, root, in)
	gateWrite(t, root, "site/src/content.ts", "export const latestVersion = \"v9.9.9\";\n")
	in.Bundle = gateBundleMust(t, root, spec)
	if withoutEdit := gateMustFingerprint(t, root, in); withoutEdit == withEdit {
		t.Fatal("an edit to a document the surface claims and the bundle never contained moved neither the bundle nor the key; that is the site-href release shipping with a carried-forward PASS")
	}
}

// TestGateBundleReadsASymlinkedDocument: `exported-skills` is five symlinks to
// directories, so the assembler has to be able to hand one over. os.ReadFile on
// one returns "is a directory".
func TestGateBundleReadsASymlinkedDocument(t *testing.T) {
	root, spec := gateBundleFixture(t)
	gateWrite(t, root, "skills/dossierx/SKILL.md", "the router bundle\n")
	gateWrite(t, root, "skills/dossierx/reference.md", "the error table\n")
	gateWrite(t, root, gateBundlePromptFile("exported-skills"), "Read the export against its source.\n")
	if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "skills", "dossierx"), filepath.Join(root, ".claude", "skills", "dossierx")); err != nil {
		t.Fatalf("this filesystem cannot create a symlink, so it cannot check out the tree the gate judges: %v", err)
	}

	spec.Surface = "exported-skills"
	spec.Handed = []string{".claude/skills/dossierx"}
	spec.Withheld = nil
	spec.Artifacts = nil

	got := string(gateBundleMust(t, root, spec))
	for _, want := range []string{
		"--- .claude/skills/dossierx/SKILL.md ---",
		"the router bundle",
		"--- .claude/skills/dossierx/reference.md ---",
		"the error table",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the bundle for a symlinked surface is missing %q; the agent would be reviewing an export it was never shown", want)
		}
	}
}
