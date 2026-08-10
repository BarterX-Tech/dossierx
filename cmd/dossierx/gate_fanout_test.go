// gate_fanout_test.go is THE PRODUCER of a gate run: the file that mints the run
// identifier, assembles one bundle per declared surface, and writes the one
// document on disk that says a fan-out happened at all.
//
// WHY IT EXISTS. Stage 3 can already turn a fan-out's answers into a receipt.
// gateStage3Collect reads one answer per declared surface out of
// gate/answers/<surface>.json, holds each to THIS run's identifier and to stage
// 2's key for that surface, and refuses a record with a hole in it.
// gateStage3MintRun mints the identifier that attribution rests on, and
// gateStage3Answer.Run says why it is minted per RUN rather than derived from the
// tree: after a partial fan-out the tree is exactly what has not changed.
//
// NOTHING WROTE ANY OF IT. `gate/answers/`, `gateStage3MintRun` and
// `gateStage3Collect` appeared nowhere outside gate_stage3_test.go;
// scripts/gate-stage2/run.sh had six modes and not one of them minted a run,
// persisted one, or read an answer back; and gateBundleAssemble — the function
// that fixes the exact bytes an agent is handed, and whose digest every surface
// key is taken over — was reachable only from Go test code that assembled a
// bundle in memory and threw it away. `run.sh command --bundle P` demanded a path
// nothing in this repository could produce.
//
// So the gap is not a missing reader. THE RUN HAD NO NAME: an answer file on disk
// could be attributed to nothing, which is the same as saying every answer on
// disk was found rather than produced, and found on disk is not produced.
//
// ONE IMPLEMENTATION, IN GO, WRAPPED BY THE SHELL. Every rule a fan-out obeys
// already lives in this package: the fan-out is the manifest's
// (gateDeclaredSurfaces), a surface's documents are `git ls-files` resolved
// against the manifest's patterns (gateSurfaceDocuments), what is handed over and
// what is only named is gateStage2BundleSpec's answer, and the bytes are
// gateBundleAssemble's. A shell re-implementation of any of those would be a
// second answer to "what is this agent being asked", and the two would diverge in
// silence, because only the Go one is under test. scripts/gate-stage2/run.sh
// therefore calls this entry point and re-implements nothing.
//
// A FAN-OUT IS A REFUSAL OR IT IS WHOLE. A surface whose bundle cannot be
// assembled fails the whole production; it never shrinks the fan-out. Twelve
// bundles and a run identifier is a run that reports on twelve surfaces while
// surfaces.yaml declares thirteen, and stage 3 would only ever hear about the
// thirteenth if somebody read the error — the reading CLAUDE.md forbids.
//
// AND THE RECORD IS WRITTEN LAST. gate/fanout.json naming a run whose bundles
// were never written is a fan-out that looks producible and is not: the harness
// reads the record, execs thirteen agents against thirteen paths, and each of
// them is handed either nothing or a bundle assembled for some earlier tree.
//
// HOW IT IS INVOKED. With -fanout-out absent it produces nothing and asserts
// nothing, and it does that by RETURNING rather than by t.Skip: a SKIP line in a
// job log is what a maintainer reads as "checked", and there is nothing here to
// check — the correctness of the assembly belongs to gate_bundle_test.go, which
// runs unconditionally.
//
//	go test ./cmd/dossierx -run '^TestGateFanoutProduce$' -count=1 \
//	  -fanout-out -fanout-tree=<40 hex tree object name>
//
// Same shape as the rest of the gate files: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	// PRESENCE IS THE SWITCH, AND THE PATHS ARE NOT THE CALLER'S TO CHOOSE. A
	// fan-out writes one bundle per declared surface plus the record, and every
	// reader of those — the shell's `fanout` mode, stage 3, the driver — addresses
	// them by the constants below. A caller-supplied output directory would be a
	// second answer to "where does this run live", and the run that used it would
	// be invisible to every one of those readers while exiting 0.
	gateFanoutOut = flag.Bool("fanout-out", false, "produce this run's fan-out into gate/ of the checkout under test: one bundle per declared surface and the run record. Requires -fanout-tree")
	// The tree is SUPPLIED, never probed, for tests/render_diff_capture_test.go's
	// reason: every other piece of a run's evidence is computed from an identity
	// the driver hands down, and a producer that resolved its own would be a
	// second answer to "which tree is this run about" inside one run.
	gateFanoutTree = flag.String("fanout-tree", "", "the full 40-digit tree object name this fan-out covers, as supplied by the driver. Requires -fanout-out")
)

// ---------------------------------------------------------------------
// the record and the bundles
// ---------------------------------------------------------------------

// gateFanoutFile is the record: the whole of this run's identity on disk.
//
// It is per-run evidence with no committed form, so gate/.gitignore's
// ignore-everything rule already covers it and must not be widened for it.
const gateFanoutFile = "gate/fanout.json"

// gateFanoutBundleDir holds one assembled bundle per declared surface — the
// exact bytes each agent is handed, written down so that the harness can exec an
// agent against a file and so that a human reviewing a finding can read what the
// agent was actually asked.
const gateFanoutBundleDir = "gate/bundles"

// gateFanoutBundleFile is one surface's bundle, addressed by surface name for
// gateStage3AnswerFile's reason: the harness matches an answer to the bundle it
// came from by name, and a path that carried anything else would need a second
// index nobody writes.
func gateFanoutBundleFile(surface string) string {
	return gateFanoutBundleDir + "/" + surface + ".md"
}

// gateFanout is gate/fanout.json.
//
// TWO FIELDS, AND BOTH ARE PROVENANCE. It carries no list of surfaces, no count
// and no bundle digests, deliberately: the fan-out's shape is surfaces.yaml's
// answer and is re-read from the manifest by every consumer, so a copy of it here
// would be a second declaration that goes stale on the day a fourteenth surface
// is declared — and it would go stale in the direction that reads as green,
// because a reader trusting the copy would report thirteen surfaces covered.
type gateFanout struct {
	// Run is the identifier every answer of this fan-out must name. See
	// gateStage3Answer.Run: it is minted per run and never derived from the tree,
	// because two fan-outs over the same tree have to be distinguishable and
	// after a partial fan-out the tree is precisely what did not move.
	Run string `json:"run"`
	// Tree is the identity the run covers, supplied by the driver and recorded
	// verbatim, so that a record left behind by an earlier release can be told
	// apart from this one's rather than adopted by it.
	Tree string `json:"tree"`
}

// ---------------------------------------------------------------------
// producing a fan-out
// ---------------------------------------------------------------------

// gateFanoutProduce mints one run, writes a bundle for every surface the
// manifest declares, and records the run — or refuses and records nothing.
//
// IT IS SEPARATE FROM THE FLAG HANDLING, for renderDiffCaptureRefusal's reason:
// every refusal below is then exercisable in-process against a fixture rather
// than only through a real gate run against the real checkout, which is a tree no
// test may write into.
//
// checkoutTree is what `git rev-parse HEAD^{tree}` answers for root, resolved by
// the caller. It is a parameter rather than a call to git here so that the
// disagreement between "the tree the driver named" and "the tree this checkout is
// at" can be constructed by a test at all; the entry point below supplies the
// real one, and TestGateFanoutProduce_FlagContract is what holds that wiring in
// place.
//
// tracked is `git ls-files` for root — the same authority gateSurfaceDocuments
// resolves the manifest's patterns against everywhere else in this gate.
func gateFanoutProduce(root, tree, checkoutTree string, tracked []string) (gateFanout, error) {
	// THE IDENTITY FIRST, because it is the cheapest question and because
	// everything below is about a tree. Forty hexadecimal digits and nothing
	// else: a tag name is a mutable pointer that `git tag -f` re-points, and an
	// abbreviation is a prefix that means a different object in a different
	// clone. The reading-side rule is gateStage2ObjectName's, reused rather than
	// re-spelled — a third spelling of "this is an identity" in one package is a
	// third place for the rule to drift.
	if !gateStage2ObjectName.MatchString(tree) {
		return gateFanout{}, fmt.Errorf("the fan-out was asked to cover tree %s, which is not a full 40-digit object name. A run that cannot say which tree it fanned out over produces answers nothing can attach to a release",
			gateFanoutQuote(tree))
	}
	if !gateStage2ObjectName.MatchString(checkoutTree) {
		return gateFanout{}, fmt.Errorf("the tree of the checkout under %s reads as %s, which is not a full 40-digit object name; with no answer to what this checkout is at, nothing can say whether the bundles about to be assembled are the bytes of the tree being released",
			root, gateFanoutQuote(checkoutTree))
	}
	// THE BUNDLES ARE ASSEMBLED FROM THE WORKING TREE, so a run naming a tree the
	// checkout is not at hands thirteen agents the bytes of one release under the
	// identity of another — and every key, every carried verdict and the whole
	// receipt is then recorded against a tree nobody read. The tree is supplied
	// rather than probed (see the flag above); this is where the supplied answer
	// and the checkout are made to agree.
	if tree != checkoutTree {
		return gateFanout{}, fmt.Errorf("the fan-out was asked to cover tree %s and the checkout under %s is at tree %s. The bundles are assembled from the files in that checkout, so this run would hand every agent one release's bytes under another release's name; check out the tree being released, or pass the tree this checkout is actually at",
			tree, root, checkoutTree)
	}

	// FROM HERE ON THIS CHECKOUT HOLDS NO FAN-OUT RECORD UNTIL THIS PRODUCTION
	// WRITES ONE. Everything below can refuse, and a refusal that left the
	// previous run's record in place would leave the driver reading a run
	// identifier that names a fan-out nobody re-ran — with the previous run's
	// answers still beside it, matching it perfectly, and the production that was
	// supposed to replace them having refused. That is the exact sequence the
	// answer-directory refusal below exists to stop, arrived at from the other
	// side.
	//
	// It is removed AFTER the two questions above because those are about the
	// caller's arguments rather than about this checkout's state: a mistyped tree
	// is not an attempt to produce a fan-out here, and it must not cost a run
	// somebody already paid thirteen agents for.
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateFanoutFile))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return gateFanout{}, fmt.Errorf("the previous run's record at %s could not be removed, so a refusal below would leave a record naming a fan-out this production did not perform: %w", gateFanoutFile, err)
	}

	// THE FAN-OUT AND THE STAGE-2 EVIDENCE MUST COVER ONE TREE. gate/run.json is
	// what says the delta, the baseline and the captures in every bundle below
	// were produced by this run from this tree; without it the bundles are
	// assembled over whatever happened to be at those paths, which is the
	// found-on-disk input gateStage2CheckFreshness exists to refuse. Checking only
	// the tree here is deliberate — the digests are freshness's question and
	// gateStage2Plan asks it — but a manifest for another tree cannot be reconciled
	// at all, and assembling thirteen bundles first only delays the same refusal.
	run, err := gateFanoutReadRun(root)
	if err != nil {
		return gateFanout{}, err
	}
	if run.Tree != tree {
		return gateFanout{}, fmt.Errorf("%s records tree %q and this fan-out covers tree %q. The delta, the resolved baseline and the captures that go into every bundle below were produced against a different tree; re-produce this run's evidence (`run.sh delta`, the capture entry points, `run.sh record`) before fanning out",
			gateStage2RunFile, run.Tree, tree)
	}

	// AN ANSWER DIRECTORY THAT IS NOT EMPTY IS A PREVIOUS RUN'S ANSWERS. Minting a
	// new run over them leaves files that validate structurally — they parse, they
	// name a run, most of their fingerprints still match because most surfaces
	// genuinely did not move — and that were given by no agent this release.
	// gateStage3StrayAnswers catches the ones this run did not ask for, one
	// surface at a time and after the agents have been paid for; this catches all
	// of them before a single one is asked.
	if stray, strayErr := gateFanoutStrayAnswers(root); strayErr != nil {
		return gateFanout{}, strayErr
	} else if len(stray) > 0 {
		return gateFanout{}, fmt.Errorf("%s already holds %s. Those are a previous run's answers: this fan-out is about to mint a new identifier, and every one of those files would sit beside it looking like an answer somebody gave this release. Remove them and re-run the fan-out:\n\trm -r %s",
			gateStage3AnswerDir, strings.Join(stray, ", "), gateStage3AnswerDir)
	}

	// THE FAN-OUT IS THE MANIFEST'S, never a count and never a list written here.
	// A fourteenth surface declared in surfaces.yaml gets a bundle on the day it
	// is declared, with no edit to this file.
	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		return gateFanout{}, fmt.Errorf("the fan-out could not be read from %s, so this run cannot say which surfaces it covers: %w", gateManifestFile, err)
	}
	documents, err := gateSurfaceDocuments(root, tracked)
	if err != nil {
		return gateFanout{}, fmt.Errorf("the surfaces' documents could not be resolved against the tracked tree, so at least one agent would be asked about a document set nobody can state: %w", err)
	}

	// The context each surface borrows from other surfaces, resolved against the
	// same tracked set the documents were. It is resolved HERE, beside them and
	// before a single bundle is assembled, so that a `reads:` entry naming a file
	// that has moved refuses the whole fan-out rather than one surface's bundle:
	// a run that covered twelve of thirteen surfaces is the shape every refusal
	// in this producer exists to prevent.
	references, err := gateSurfaceReferences(root, tracked)
	if err != nil {
		return gateFanout{}, fmt.Errorf("the surfaces' borrowed context could not be resolved against the tracked tree, so at least one agent would be asked its question without material this manifest says it needs: %w", err)
	}

	// MINTED, NOT DERIVED. See gateStage3MintRun: a tree-derived identifier is
	// identical across the re-run that follows a partial fan-out, which is exactly
	// the case the identifier exists to catch.
	id, err := gateStage3MintRun()
	if err != nil {
		return gateFanout{}, err
	}

	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(gateFanoutBundleDir)), 0o755); err != nil {
		return gateFanout{}, fmt.Errorf("the bundle directory %s could not be created: %w", gateFanoutBundleDir, err)
	}
	for _, surface := range declared {
		spec, specErr := gateStage2BundleSpec(surface, documents[surface], references[surface])
		if specErr != nil {
			// A REFUSAL, NEVER A SHORTER FAN-OUT. The one shape this really guards
			// is gateStage2CheckExtractIsWhole's: a surface asked five questions
			// while holding the material for two answers what it could see, which
			// reads exactly like a pass.
			return gateFanout{}, fmt.Errorf("surface %q could not be given a bundle to be assembled from, so this fan-out would cover %d of the %d surfaces %s declares: %w",
				surface, len(declared)-1, len(declared), gateManifestFile, specErr)
		}
		bundle, assembleErr := gateBundleAssemble(root, spec)
		if assembleErr != nil {
			return gateFanout{}, fmt.Errorf("surface %q could not be assembled, so this fan-out would cover %d of the %d surfaces %s declares: %w",
				surface, len(declared)-1, len(declared), gateManifestFile, assembleErr)
		}
		rel := gateFanoutBundleFile(surface)
		// A bundle for a surface the manifest no longer declares is left where it
		// is rather than swept: every consumer of a fan-out walks the manifest, so
		// nothing reads it, and a producer that deleted files under gate/ on the
		// strength of a name pattern would be one manifest typo away from deleting
		// this run's own work.
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), bundle, 0o644); err != nil {
			return gateFanout{}, fmt.Errorf("surface %q assembled and %s could not be written, so the harness would exec an agent against a path holding nothing or holding an earlier run's question: %w", surface, rel, err)
		}
	}

	// LAST, AND ATOMICALLY. See the file header: a record naming a run whose
	// bundles were never written is a fan-out that looks producible and is not,
	// and a half-written record parses far enough to look like a run that covered
	// less than it did. This is the reasoning scripts/gate-stage2/run.sh already
	// applies to gate/run.json ("a truncated manifest is worse than none").
	doc := gateFanout{Run: id, Tree: tree}
	if err := gateFanoutWriteRecord(root, doc); err != nil {
		return gateFanout{}, err
	}
	return doc, nil
}

// gateFanoutReadRun reads gate/run.json far enough to ask which tree it covers.
//
// It reads it as gateStage2Run — the type stage 2 already declares for this
// document — rather than as a map with one key, so that a manifest this producer
// accepts and gateStage2CheckFreshness rejects cannot arise from two readers
// disagreeing about the document's shape.
func gateFanoutReadRun(root string) (gateStage2Run, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateStage2RunFile)))
	if err != nil {
		return gateStage2Run{}, fmt.Errorf("%s is not there, so this run has produced no evidence it can name: the delta, the baseline and the captures that go into every bundle would be whatever is at those paths, hand-written or left from the last release. Run `run.sh delta` and `run.sh record` for this tree first: %w",
			gateStage2RunFile, err)
	}
	var run gateStage2Run
	if err := json.Unmarshal(raw, &run); err != nil {
		return gateStage2Run{}, fmt.Errorf("%s does not parse, so whatever is at that path is not a run manifest and nothing attaches this fan-out to a tree: %w", gateStage2RunFile, err)
	}
	return run, nil
}

// gateFanoutStrayAnswers names every file sitting in the answer directory,
// whatever it is called.
//
// EVERY FILE, not every .json and not every declared surface's answer.
// gateStage3StrayAnswers is the narrower question — "which answers did this run
// not ask for" — and it is asked after the fan-out. This one is asked before a
// run is minted, when the correct answer for the whole directory is that it is
// empty, so anything at all in there is a leftover a human has to look at.
func gateFanoutStrayAnswers(root string) ([]string, error) {
	dir := filepath.Join(root, filepath.FromSlash(gateStage3AnswerDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s exists and cannot be read, so nothing can say whether this run is about to mint an identifier over a previous run's answers: %w", gateStage3AnswerDir, err)
	}
	var out []string
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	return out, nil
}

// gateFanoutWriteRecord writes the record through a temporary file in the
// destination directory and renames, so that a failure partway through cannot
// leave a document that parses far enough to look like a fan-out.
func gateFanoutWriteRecord(root string, doc gateFanout) error {
	dest := filepath.Join(root, filepath.FromSlash(gateFanoutFile))
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create the directory for %s: %w", gateFanoutFile, err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", gateFanoutFile, err)
	}
	data = append(data, '\n')

	f, err := os.CreateTemp(dir, ".fanout-*.json")
	if err != nil {
		return fmt.Errorf("create a temporary file beside %s: %w", gateFanoutFile, err)
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
	// CreateTemp makes the file 0600; the record is read by the harness that
	// collects the answers, which may not be this process's user.
	if err := os.Chmod(tmp, 0o644); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmp, gateFanoutFile, err)
	}
	return nil
}

func gateFanoutQuote(s string) string {
	if s == "" {
		return "(empty)"
	}
	return "\"" + s + "\""
}

// ---------------------------------------------------------------------
// reading a fan-out back
// ---------------------------------------------------------------------

// gateReadFanout is how everything downstream learns this run's identifier: the
// driver, before it collects a single answer, and any re-run deciding whether
// what is on disk belongs to the tree it is about to release.
//
// EVERY REFUSAL IS errGateStage2NotProduced, one sentinel for all four, because
// all four mean the same thing to the caller and take the same recovery: what is
// on disk was not produced by a fan-out over this tree, so fan out again. That is
// the same position gateStage2CheckFreshness takes about gate/run.json, which is
// the same kind of document — a per-run file with no committed form, which can be
// absent, half-written, hand-written, or left behind by the last release.
//
// The alternative — reading the answers first and treating an absent record as
// "nothing to attribute" — is the shape this whole stage exists to refuse: an
// answer set that cannot name the fan-out that produced it was FOUND on disk, and
// found on disk is not produced.
func gateReadFanout(root, tree string) (gateFanout, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateFanoutFile)))
	if err != nil {
		return gateFanout{}, fmt.Errorf("%w: %s is not there, so no fan-out was recorded for tree %s. Any answer under %s was produced by something nobody can name — a previous release's run, an interrupted one, or a hand-written file — and there is no reading of them that says an agent answered this release: %w",
			errGateStage2NotProduced, gateFanoutFile, gateFanoutQuote(tree), gateStage3AnswerDir, err)
	}
	var doc gateFanout
	if err := json.Unmarshal(raw, &doc); err != nil {
		return gateFanout{}, fmt.Errorf("%w: %s does not parse, so whatever is at that path is not a fan-out record and this run has no identifier to hold its answers to: %w",
			errGateStage2NotProduced, gateFanoutFile, err)
	}
	// THE IDENTIFIER IS CHECKED FOR SHAPE, not merely for presence. An empty or
	// truncated run field compares equal to the empty Run of every answer a
	// half-written producer left behind, so `answer.Run == fanout.Run` would hold
	// over two documents neither of which names anything.
	if !gateStage3RunID.MatchString(doc.Run) {
		return gateFanout{}, fmt.Errorf("%w: %s records run %s, which is not a minted run identifier. Every answer this run collects is attributed by matching that string, so a record that names no run would attribute answers to nothing while comparing equal to every answer that names nothing either",
			errGateStage2NotProduced, gateFanoutFile, gateFanoutQuote(doc.Run))
	}
	if !gateStage2ObjectName.MatchString(doc.Tree) {
		return gateFanout{}, fmt.Errorf("%w: %s records tree %s, which is not a full 40-digit object name. A record that cannot say which release it fanned out over cannot be told apart from one left behind by the last release, which is the only thing standing between this run's answers and a previous run's",
			errGateStage2NotProduced, gateFanoutFile, gateFanoutQuote(doc.Tree))
	}
	if doc.Tree != tree {
		return gateFanout{}, fmt.Errorf("%w: %s records a fan-out over tree %s and this run covers tree %s. The bundles those agents read were assembled from a different release's files, so their answers are about a release nobody is publishing; fan out again for this tree",
			errGateStage2NotProduced, gateFanoutFile, doc.Tree, tree)
	}
	return doc, nil
}

// ---------------------------------------------------------------------
// the entry point
// ---------------------------------------------------------------------

// TestGateFanoutProduce is how a gate run produces its fan-out. See this file's
// header for the invocation.
//
// PRESENCE, NOT VALUE. Both flag decisions key off flag.CommandLine.Visit rather
// than off the value, for the reason tests/render_diff_capture_test.go states and
// tests/release_notes_predict_test.go and tests/skills_export_capture_test.go
// each closed the same way: a driver expanding an unset shell variable gives a
// flag EMPTY, which a value check cannot tell apart from the flag never having
// been passed — and under a value check that run produces nothing, exits 0, and
// the driver reads the exit code as a fan-out that happened.
func TestGateFanoutProduce(t *testing.T) {
	var outGiven, treeGiven bool
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "fanout-out":
			outGiven = true
		case "fanout-tree":
			treeGiven = true
		}
	})

	if !outGiven {
		// -fanout-tree IMPLIES -fanout-out: a caller that named a tree meant to
		// produce a fan-out over it, not to no-op.
		if treeGiven {
			t.Fatalf("-fanout-tree was given without -fanout-out; it implies it, and a run that names a tree and then produces nothing is a fan-out the driver will believe it has")
		}
		// The ordinary `go test ./cmd/dossierx` invocation, and every CI run of
		// this package. It RETURNS rather than skipping: see the file header.
		t.Logf("no -fanout-out given; this test is the gate's fan-out producer, not a correctness check (gate_bundle_test.go and gate_stage2_test.go are that, and they have already run)")
		return
	}
	if !*gateFanoutOut {
		t.Fatalf("-fanout-out was given as false. This flag is the switch that produces the run, so a caller that passed it meant to produce one; nothing would be written while `go test` exits 0, and the driver would read that as a fan-out it can collect answers for")
	}
	if !treeGiven {
		t.Fatalf("-fanout-out was given without -fanout-tree; a fan-out that does not say which tree it covers mints an identifier that attaches its answers to no release, and every later reader would have to take on trust that the bundles were assembled from the tree being published")
	}

	root := surfaceRepoRoot(t)
	// THE CHECKOUT'S OWN TREE, resolved here and compared inside the producer. A
	// git that cannot answer is fatal rather than an empty string: an empty answer
	// would compare unequal to every supplied tree and the refusal would read as a
	// caller error, and a producer that skipped the comparison would assemble one
	// release's bytes under another release's name. A check that cannot run is a
	// failure, not a pass.
	checkoutTree, err := gateTreeSHA(root, "HEAD")
	if err != nil {
		t.Fatalf("the tree of the checkout under %s could not be resolved, so nothing can say whether the bundles this run would assemble are the bytes of the tree being released: %v", root, err)
	}

	doc, err := gateFanoutProduce(root, *gateFanoutTree, checkoutTree, surfaceTrackedFiles(t, root))
	if err != nil {
		t.Fatalf("refusing to produce a fan-out: %v", err)
	}
	t.Logf("minted run %s over tree %s: wrote %s/<surface>.md for every surface %s declares, and recorded the run in %s",
		doc.Run, doc.Tree, gateFanoutBundleDir, gateManifestFile, gateFanoutFile)
}

// ---------------------------------------------------------------------
// the fan-out is whole, or it is a refusal
// ---------------------------------------------------------------------

// gateFanoutFixture is a checkout the producer can really run against: the REAL
// manifest, the REAL documents and the REAL prompts, with this run's per-run
// evidence stubbed.
//
// It is gateStage2Overlay's tree because that is the only fixture in this
// repository over which thirteen bundles genuinely assemble — the real tree holds
// five symlinks to directories under `exported-skills` and 1.95 MB under
// `binary-and-viewer`, and a synthetic tree of regular files would let this whole
// file pass over a producer that cannot assemble the fan-out it exists to
// produce.
func gateFanoutFixture(t *testing.T) (root string, tracked []string) {
	t.Helper()
	overlay, realRoot := gateStage2Overlay(t)
	tracked = surfaceTrackedFiles(t, realRoot)
	declared, err := gateDeclaredSurfaces(overlay)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	gateStage2StampRun(t, overlay, gateStage2FixtureTree, declared)
	return overlay, tracked
}

// TestGateFanoutProducesABundleForEveryDeclaredSurface is the positive control
// and the coverage assertion in one, and the coverage half is asked of the
// MANIFEST rather than of what the producer reports.
//
// Iterating what was written and reporting on it is the shape that produces
// twelve bundles and states a run as though it covered thirteen — the same shape
// gateStage3Collect refuses one stage down, and the same one
// gateStage2RefusesToProceedWhenAnySurfaceKeyCannotBeComputed exists for.
func TestGateFanoutProducesABundleForEveryDeclaredSurface(t *testing.T) {
	root, tracked := gateFanoutFixture(t)

	doc, err := gateFanoutProduce(root, gateStage2FixtureTree, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the producer refused an honest run, so every refusal below would pass over a check that fires unconditionally:\n%v", err)
	}
	if !gateStage3RunID.MatchString(doc.Run) {
		t.Fatalf("the run identifier is %q, which gateStage3ValidateAnswer would refuse in every answer of this fan-out", doc.Run)
	}

	declared, err := gateDeclaredSurfaces(root)
	if err != nil {
		t.Fatalf("declared surfaces: %v", err)
	}
	if len(declared) < 2 {
		t.Fatalf("%s declares %d surface(s); the assertions below would be about almost nothing", gateManifestFile, len(declared))
	}
	for _, surface := range declared {
		rel := gateFanoutBundleFile(surface)
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil {
			t.Errorf("surface %q is declared in %s and this run wrote no bundle for it, so the harness would exec its agent against nothing and no answer for it could be produced: %v", surface, gateManifestFile, readErr)
			continue
		}
		// The bundle is the ASSEMBLED bytes, not a plan for assembling them: the
		// frame's placeholders are gone, this surface is named, and the standing
		// instruction every agent answers under is in the file.
		if strings.Contains(string(body), gateBundleSurfaceMarker) || strings.Contains(string(body), gateBundlePartsMarker) {
			t.Errorf("%s still carries a frame placeholder; the agent would be handed the question's template rather than its question", rel)
		}
		if !strings.Contains(string(body), "Surface review — "+surface) {
			t.Errorf("%s does not name surface %q, so thirteen agents would be reading an unaddressed question — which is also thirteen identical bundles wherever the documents are withheld", rel, surface)
		}
		if !strings.Contains(string(body), "BEGIN the question — "+gateBundlePromptFile(surface)) {
			t.Errorf("%s does not carry this surface's own question", rel)
		}
	}

	// The record is on disk, it reads back, and it says what was minted. This is
	// the handshake W12b-2's driver makes before it collects an answer.
	got, err := gateReadFanout(root, gateStage2FixtureTree)
	if err != nil {
		t.Fatalf("the record this run wrote is not one its own reader accepts: %v", err)
	}
	if got != doc {
		t.Errorf("the record reads back as %+v and the run produced %+v", got, doc)
	}
}

// TestGateFanoutMintsANewIdentifierEveryTime pins the property the whole
// attribution rests on, at the one moment it can be lost.
//
// gateStage3MintRun is random rather than tree-derived for a stated reason: two
// fan-outs over the same tree must be distinguishable, and after a partial
// fan-out the tree is precisely what has not changed. A producer that hashed the
// tree, or reused the identifier already in gate/fanout.json, would satisfy every
// other assertion in this file — the record parses, the identifier is well
// formed, thirteen bundles are written — while making gateStage3ValidateAnswer's
// run check accept every answer left behind by the previous run.
func TestGateFanoutMintsANewIdentifierEveryTime(t *testing.T) {
	root, tracked := gateFanoutFixture(t)

	first, err := gateFanoutProduce(root, gateStage2FixtureTree, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the first production refused: %v", err)
	}
	second, err := gateFanoutProduce(root, gateStage2FixtureTree, gateStage2FixtureTree, tracked)
	if err != nil {
		t.Fatalf("the second production refused: %v", err)
	}
	if second.Run == first.Run {
		t.Fatalf("two fan-outs over one unchanged tree minted the same identifier %s. Every answer the first run left on disk then names the second run exactly, and the check that tells a fresh answer from one nobody gave this release cannot fire", first.Run)
	}
}

// TestGateFanoutRefusesEveryRunItCannotStandBehind walks every state in which a
// fan-out must not be minted. Each row is a run that would otherwise produce a
// perfectly well-formed record, thirteen readable bundles, or both — and each is a
// run whose answers would be about something other than this release.
//
// wantRecord says what must be true of a record left over from a previous run
// after the refusal, and it is asserted because the boundary is a decision rather
// than an accident: an argument this producer will not act on leaves the checkout
// alone, and an attempt to produce a fan-out HERE invalidates the previous
// record before it can refuse — otherwise a refused production leaves the driver
// reading a run identifier that still matches the answers the refusal was about.
func TestGateFanoutRefusesEveryRunItCannotStandBehind(t *testing.T) {
	const stale = "{\n  \"run\": \"00000000000000000000000000000000\",\n  \"tree\": \"" + gateStage2FixtureTree + "\"\n}\n"

	for _, tc := range []struct {
		name       string
		mutate     func(t *testing.T, root string, tree, checkoutTree *string)
		want       string
		keepsStale bool
	}{
		{
			name:       "no tree at all",
			mutate:     func(_ *testing.T, _ string, tree, _ *string) { *tree = "" },
			want:       "not a full 40-digit object name",
			keepsStale: true,
		},
		{
			name:       "a tree that is an abbreviation",
			mutate:     func(_ *testing.T, _ string, tree, _ *string) { *tree = "3217a48" },
			want:       "not a full 40-digit object name",
			keepsStale: true,
		},
		{
			name:       "a tree in upper case, which names no git object",
			mutate:     func(_ *testing.T, _ string, tree, _ *string) { *tree = strings.Repeat("A", 40) },
			want:       "not a full 40-digit object name",
			keepsStale: true,
		},
		{
			name: "a tree this checkout is not at",
			// The ordinary sequence: the driver resolved a tree, a fix landed, and
			// the checkout moved under it. The bundles would be assembled from the
			// files on disk and recorded under the tree that was resolved.
			mutate:     func(_ *testing.T, _ string, _, checkoutTree *string) { *checkoutTree = strings.Repeat("c", 40) },
			want:       "is at tree",
			keepsStale: true,
		},
		{
			name: "the checkout's tree could not be resolved",
			// git failing is the caller's answer being empty. It must not read as
			// "the trees agree".
			mutate:     func(_ *testing.T, _ string, _, checkoutTree *string) { *checkoutTree = "" },
			want:       "not a full 40-digit object name",
			keepsStale: true,
		},
		{
			name: "no run manifest at all",
			mutate: func(t *testing.T, root string, _, _ *string) {
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(gateStage2RunFile))); err != nil {
					t.Fatalf("remove the run manifest: %v", err)
				}
			},
			want: "is not there",
		},
		{
			name: "a run manifest that does not parse",
			mutate: func(t *testing.T, root string, _, _ *string) {
				gateWrite(t, root, gateStage2RunFile, "{\n")
			},
			want: "does not parse",
		},
		{
			name: "a run manifest from the previous tree",
			mutate: func(t *testing.T, root string, _, _ *string) {
				declared, err := gateDeclaredSurfaces(root)
				if err != nil {
					t.Fatalf("declared surfaces: %v", err)
				}
				gateStage2StampRun(t, root, strings.Repeat("d", 40), declared)
			},
			want: "records tree",
		},
		{
			name: "answers left behind by a previous run",
			mutate: func(t *testing.T, root string, _, _ *string) {
				gateWrite(t, root, gateStage3AnswerFile("changelog"), "{\"run\":\"deadbeefdeadbeef\"}\n")
			},
			want: "Remove them and re-run the fan-out",
		},
		{
			name: "a file in the answer directory that is not even an answer",
			// Every file, not every .json: the correct state of this directory
			// before a fan-out is empty, so anything at all in it is something a
			// human has to look at.
			mutate: func(t *testing.T, root string, _, _ *string) {
				gateWrite(t, root, gateStage3AnswerDir+"/notes.txt", "half a paste from the last run\n")
			},
			want: "Remove them and re-run the fan-out",
		},
		{
			name: "the manifest is gone",
			// The fan-out IS the manifest. With no manifest there is no coverage
			// claim at all, and a producer that fanned out over what it happened to
			// find would be inventing one.
			mutate: func(t *testing.T, root string, _, _ *string) {
				if err := os.Remove(filepath.Join(root, gateManifestFile)); err != nil {
					t.Fatalf("remove the manifest: %v", err)
				}
			},
			want: "the fan-out could not be read from " + gateManifestFile,
		},
		{
			name: "a declared surface that owns no tracked file",
			// Its patterns went stale or the surface is gone. Either way an agent
			// would be asked to read a surface with no documents in it, and an agent
			// with nothing to read does not report FAILED — it reports what it could
			// see.
			mutate: func(t *testing.T, root string, _, _ *string) {
				gateFanoutEditManifest(t, root, "\nsurfaces:\n", "\nsurfaces:\n  - name: ghost\n    paths: [ghost/]\n")
			},
			want: "owns no tracked file",
		},
		{
			name: "a surface that would be asked a question it holds no material for",
			// gateStage2CheckExtractIsWhole's refusal, reached through the spec
			// rather than through the assembly: `binary-and-viewer` stops claiming
			// the command sources, so the class resolves to no handed file and the
			// prompt's first check is put to an agent holding surface.json, which
			// carries no Long text, no flag usage and no error string at all.
			mutate: func(t *testing.T, root string, _, _ *string) {
				gateFanoutEditManifest(t, root, "      - cmd/dossierx/\n      - internal/\n", "      - internal/\n")
			},
			want: "could not be given a bundle to be assembled from",
		},
		{
			name: "one surface's question is gone",
			// Twelve bundles stay perfectly assemblable, which is the whole shape
			// of the failure: the repair reached for under time pressure is to skip
			// the surface that will not assemble.
			mutate: func(t *testing.T, root string, _, _ *string) {
				gateFanoutReplacePromptsWithout(t, root, "roadmap.md")
			},
			want: "would cover 12 of the 13 surfaces",
		},
		{
			name: "the material one surface's own question is about is gone",
			// gateStage2CheckExtractIsWhole's refusal, reached through the producer:
			// `binary-and-viewer` is asked five questions and would hold the
			// material for two. An agent with no material does not report FAILED, it
			// reports what it could see.
			mutate: func(t *testing.T, root string, _, _ *string) {
				if err := os.Remove(filepath.Join(root, "gate", "render-diff.json")); err != nil {
					t.Fatalf("remove the render diff: %v", err)
				}
			},
			want: "could not be assembled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, tracked := gateFanoutFixture(t)
			gateWrite(t, root, gateFanoutFile, stale)
			tree, checkoutTree := gateStage2FixtureTree, gateStage2FixtureTree
			tc.mutate(t, root, &tree, &checkoutTree)

			_, err := gateFanoutProduce(root, tree, checkoutTree, tracked)
			if err == nil {
				t.Fatalf("the fan-out was produced. Thirteen agents would be paid to answer about a release nobody is publishing, and every answer would be attributed to a run this checkout cannot stand behind")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %q:\n%v", tc.want, err)
			}

			raw, statErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateFanoutFile)))
			switch {
			case tc.keepsStale && statErr != nil:
				t.Errorf("the refusal removed a record it never attempted to replace; the run it named cost thirteen agents, and %q is an argument this producer declined to act on rather than a production over this checkout", tc.name)
			case tc.keepsStale:
				if string(raw) != stale {
					t.Errorf("%s was rewritten by a run that refused:\n%s", gateFanoutFile, raw)
				}
			case statErr == nil:
				t.Errorf("a run that refused left %s naming a fan-out it did not perform:\n%s\nThe driver reads that identifier, finds the previous run's answers matching it exactly, and proceeds on a fan-out nobody re-ran", gateFanoutFile, raw)
			}
		})
	}
}

// gateFanoutEditManifest rewrites the fixture's copy of surfaces.yaml, and
// requires the text it is replacing to occur EXACTLY ONCE.
//
// The requirement is the point: a row built by a replacement that silently
// matched nothing is a row that runs against an untouched manifest and passes
// because the producer refused for some other reason, or fails for none. The
// fixture's manifest is a copy, so nothing here reaches the real tree.
func gateFanoutEditManifest(t *testing.T, root, before, after string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, gateManifestFile))
	if err != nil {
		t.Fatalf("read the fixture's %s: %v", gateManifestFile, err)
	}
	if got := strings.Count(string(raw), before); got != 1 {
		t.Fatalf("%s holds %d occurrence(s) of the text this row edits; the row would be about a manifest nobody changed", gateManifestFile, got)
	}
	gateWrite(t, root, gateManifestFile, strings.Replace(string(raw), before, after, 1))
}

// gateFanoutReplacePromptsWithout swaps the fixture's symlinked prompt directory
// for a real copy missing one file, so that exactly one surface stops being
// assemblable and nothing touches the real repository.
func gateFanoutReplacePromptsWithout(t *testing.T, root, drop string) {
	t.Helper()
	realRoot := surfaceRepoRoot(t)
	entries, err := os.ReadDir(filepath.Join(realRoot, "gate", "prompts"))
	if err != nil {
		t.Fatalf("read the prompts: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "gate", "prompts")); err != nil {
		t.Fatalf("unlink the fixture's prompts: %v", err)
	}
	dropped := 0
	for _, entry := range entries {
		if entry.Name() == drop {
			dropped++
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(realRoot, "gate", "prompts", entry.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Name(), readErr)
		}
		gateWrite(t, root, "gate/prompts/"+entry.Name(), string(data))
	}
	if dropped != 1 {
		t.Fatalf("meant to drop exactly one prompt named %s and dropped %d; the row below would be about a fan-out that is whole", drop, dropped)
	}
}

// TestGateFanoutWritesTheRecordLastOrNotAtAll is the ordering assertion.
//
// It is separate from the table above because the property is not "it refused" —
// it is that a production which dies partway through leaves NO record while
// leaving bundles behind. A record naming a run whose bundles were never written
// is a fan-out that looks producible and is not: the harness reads the record,
// execs an agent per declared surface, and hands each of them either nothing or
// an earlier run's question — and the answers come back well formed, naming the
// recorded run, matching this tree.
func TestGateFanoutWritesTheRecordLastOrNotAtAll(t *testing.T) {
	root, tracked := gateFanoutFixture(t)
	// `roadmap` sorts last of the thirteen, so twelve bundles are written before
	// the production fails. That is what makes this an ordering assertion rather
	// than a repeat of "it refused".
	gateFanoutReplacePromptsWithout(t, root, "roadmap.md")

	if _, err := gateFanoutProduce(root, gateStage2FixtureTree, gateStage2FixtureTree, tracked); err == nil {
		t.Fatal("a fan-out missing one surface's question was produced")
	}

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateFanoutFile))); err == nil {
		t.Errorf("%s was written for a run that could not assemble every bundle; the harness would fan out over a run identifier whose thirteenth surface has no question on disk", gateFanoutFile)
	}
	// And the bundles that WERE written are still there, which is why the record
	// is the seal rather than the bundles: nothing downstream reads a bundle
	// except through the record.
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateFanoutBundleFile("changelog")))); err != nil {
		t.Errorf("the production wrote no bundle at all before it refused, so this test is not exercising the ordering it is about: %v", err)
	}
	if _, err := gateReadFanout(root, gateStage2FixtureTree); err == nil {
		t.Error("the reader accepted a fan-out that was never completed")
	}
}

// ---------------------------------------------------------------------
// reading a fan-out back
// ---------------------------------------------------------------------

// TestGateReadFanoutRefusesARecordThatAttributesNothing covers the reader the
// driver calls before it collects a single answer. Every row is a file that
// exists, and all but two of them parse — which is the point: an unreadable
// record is the easy case, and these are the shapes an interrupted producer, a
// previous release, or a hand-written stand-in actually leave behind.
func TestGateReadFanoutRefusesARecordThatAttributesNothing(t *testing.T) {
	const run = "0123456789abcdef0123456789abcdef"
	tree := strings.Repeat("a", 40)
	honest := fmt.Sprintf("{\n  \"run\": %q,\n  \"tree\": %q\n}\n", run, tree)

	t.Run("an honest record is read (positive control)", func(t *testing.T) {
		root := t.TempDir()
		gateWrite(t, root, gateFanoutFile, honest)
		got, err := gateReadFanout(root, tree)
		if err != nil {
			t.Fatalf("a record that names this run over this tree was refused, so every row below would pass over a reader that refuses everything: %v", err)
		}
		if got.Run != run || got.Tree != tree {
			t.Errorf("the record read back as %+v, want run %s over tree %s", got, run, tree)
		}
	})

	for _, tc := range []struct {
		name string
		// body is "" for the row where the record is absent entirely.
		body  string
		write bool
		want  string
	}{
		{
			name:  "no record at all",
			write: false,
			want:  "is not there",
		},
		{
			name:  "a record that does not parse",
			body:  "{\n",
			write: true,
			want:  "does not parse",
		},
		{
			// The two-byte record. `printf '{}' > gate/fanout.json` is the one-line
			// workaround for a fan-out that has been refusing for ten minutes, and
			// it would otherwise hand the driver an empty run identifier that
			// compares equal to the empty Run of every half-written answer.
			name:  "a record that names neither a run nor a tree",
			body:  "{}\n",
			write: true,
			want:  "not a minted run identifier",
		},
		{
			name:  "a record whose run identifier is prose",
			body:  fmt.Sprintf("{\"run\": \"the second one\", \"tree\": %q}\n", tree),
			write: true,
			want:  "not a minted run identifier",
		},
		{
			name:  "a record whose tree is a tag rather than an identity",
			body:  fmt.Sprintf("{\"run\": %q, \"tree\": \"v0.5.0\"}\n", run),
			write: true,
			want:  "not a full 40-digit object name",
		},
		{
			name:  "a record left behind by the previous release",
			body:  fmt.Sprintf("{\"run\": %q, \"tree\": %q}\n", run, strings.Repeat("b", 40)),
			write: true,
			want:  "and this run covers tree",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.write {
				gateWrite(t, root, gateFanoutFile, tc.body)
			}
			_, err := gateReadFanout(root, tree)
			if err == nil {
				t.Fatalf("the record was accepted; the driver would collect answers under an identifier that attributes them to no fan-out over this tree")
			}
			if !errors.Is(err, errGateStage2NotProduced) {
				t.Errorf("the refusal is not the not-produced sentinel, so a caller cannot tell this apart from a run that failed for some other reason and cannot route the operator to `run.sh fanout`: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %q:\n%v", tc.want, err)
			}
		})
	}
}

// ---------------------------------------------------------------------
// the flag contract
// ---------------------------------------------------------------------

// TestGateFanoutProduce_FlagContract exercises the entry point by running it in a
// SUBPROCESS, which is the only way to observe it: flag values are process-global
// and this binary only ever sees the flags it was started with.
//
// Every row here refuses before anything is written, and that is a constraint on
// the rows rather than a coincidence — the entry point runs against the REAL
// checkout, which no test may write into. What the rows can therefore prove is
// exactly the wiring the in-process tests above cannot: that -fanout-out and
// -fanout-tree reach the producer, and that the tree the entry point compares
// against is the tree of the checkout it is running in rather than a value it
// invented. The production itself is exercised in-process, over a fixture built
// from the real manifest and the real documents.
func TestGateFanoutProduce_FlagContract(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("the go toolchain is not on PATH, so the flag contract of the gate's fan-out producer cannot be exercised. A check that cannot run is a failure, not a pass.")
	}
	root := surfaceRepoRoot(t)

	// THIS TEST IS HERMETIC IN gate/fanout.json, and it has to be, because it
	// drives the REAL producer against the REAL checkout while asserting that no
	// record exists afterwards.
	//
	// WHAT WENT WRONG, MEASURED. In a checkout where `make ci-evidence` has
	// already written a fan-out record — which is every checkout in the middle of
	// a release, the one moment this suite is most likely to be run — all five
	// refusal rows below failed, each reporting "a run that refused wrote
	// gate/fanout.json into the checkout under test" about a run that wrote
	// nothing at all. The file was somebody else's, already on disk, and the
	// assertion could not tell that from a record the run had just produced. Five
	// false findings that read exactly like true ones.
	//
	// WHAT DID NOT HAPPEN, stated because the opposite is the natural guess.
	// gateFanoutProduce does deliberately `os.Remove` the previous run's record —
	// deliberate and right, because a refusal that left the old record in place
	// would leave the driver reading a run identifier for a fan-out nobody
	// re-ran. Every row here refuses BEFORE that point, though: the tree
	// arguments are checked first, precisely so a mistyped tree does not cost a
	// run somebody already paid thirteen agents for. So this test never destroyed
	// a release's evidence, and the stash is not what stops it from doing so.
	//
	// The stash earns its place by making the assertion mean what it says. With
	// the file moved aside the baseline is genuinely "no record", so anything
	// found afterwards was written by the run under test — and the checkout is
	// left exactly as it was found, including for a future row that does reach
	// the removal.
	gateFanoutStashRecord(t, root)

	run := func(t *testing.T, args ...string) (output string, code int) {
		t.Helper()
		full := append([]string{"test", "./cmd/dossierx", "-run", "^TestGateFanoutProduce$", "-count=1", "-v"}, args...)
		cmd := exec.Command("go", full...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return string(out), exitErr.ExitCode()
			}
			t.Fatalf("go %s: %v\n%s", strings.Join(full, " "), err, out)
		}
		return string(out), 0
	}

	t.Run("no flags at all passes without producing and WITHOUT skipping", func(t *testing.T) {
		out, code := run(t)
		if code != 0 {
			t.Fatalf("the ordinary `go test ./cmd/dossierx` invocation must not fail, got exit %d:\n%s", code, out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("the producer SKIPPED. A skip in a job log is what a maintainer reads as \"checked\"; with no -fanout-out there is nothing to produce and the test must simply pass:\n%s", out)
		}
	})

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			"a tree named with no -fanout-out",
			[]string{"-fanout-tree=" + strings.Repeat("a", 40)},
			"was given without -fanout-out",
		},
		{
			"-fanout-out given as false",
			[]string{"-fanout-out=false", "-fanout-tree=" + strings.Repeat("a", 40)},
			"-fanout-out was given as false",
		},
		{
			"a production with no tree",
			[]string{"-fanout-out"},
			"without -fanout-tree",
		},
		{
			"a tree that is not an identity",
			[]string{"-fanout-out", "-fanout-tree=HEAD"},
			"not a full 40-digit object name",
		},
		{
			// The row that proves the entry point resolves the checkout's own tree
			// rather than believing the caller. Forty a's is a well-formed object
			// name that names nothing in any clone, so this refusal can only come
			// from a comparison against a tree that was actually read.
			"a well-formed tree that is not this checkout's",
			[]string{"-fanout-out", "-fanout-tree=" + strings.Repeat("a", 40)},
			"is at tree",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := run(t, tc.args...)
			if code == 0 {
				t.Fatalf("expected a refusal, got exit 0. The driver would read that as a fan-out produced for this run:\n%s", out)
			}
			if strings.Contains(out, "--- SKIP") {
				t.Errorf("expected a FAIL, not a SKIP:\n%s", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected the refusal to name %q, got:\n%s", tc.want, out)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateFanoutFile))); err == nil {
				t.Errorf("a run that refused wrote %s into the checkout under test", gateFanoutFile)
			}
		})
	}
}

// ---------------------------------------------------------------------
// the harness wraps this producer and re-implements none of it
// ---------------------------------------------------------------------

// gateFanoutRunHarness runs one mode of the REAL scripts/gate-stage2/run.sh and
// returns everything it said and the code it exited with.
//
// It is CombinedOutput rather than gateStage2Harness's stdout-only Output, and
// the difference is the whole point of the rows below: every refusal this mode
// can produce is written to stderr, and a runner that could not read stderr could
// only ever assert "it exited non-zero" — which usage() also does, and which a
// `fanout` mode that had been deleted would do too. This is the same shape
// tests/render_diff_capture_test.go uses to assert `record`'s refusals.
//
// path is the PATH the script runs under, so that "no go toolchain" can be
// constructed; "" inherits this process's.
func gateFanoutRunHarness(t *testing.T, path string, args ...string) (output string, code int) {
	t.Helper()
	script := filepath.Join(surfaceRepoRoot(t), filepath.FromSlash(gateStage2HarnessFile))
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("the stage-2 harness is not in the tree, so the fan-out has no invocation at all: %v", err)
	}
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	if path != "" {
		cmd.Env = append(os.Environ(), "PATH="+path)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("bash %s %s: %v\n%s", script, strings.Join(args, " "), err, out)
	}
	return string(out), 0
}

// gateFanoutPATHWithout builds a PATH holding the tools run.sh needs to reach the
// fan-out mode at all, and not the one named.
//
// It links the tools it finds rather than naming a directory, because "go is not
// installed" spelled as PATH=/usr/bin is a claim about the machine — true on the
// author's and false on a machine that installed the toolchain there — and this
// row is about the script's behaviour, not about where anybody's Go lives. A tool
// that cannot be found fails the test: a check that cannot run is a failure.
func gateFanoutPATHWithout(t *testing.T, missing string) string {
	t.Helper()
	dir := t.TempDir()
	// dirname is what run.sh resolves its own root with, before any mode runs;
	// awk is what it reads the manifest and the declaration with.
	for _, tool := range []string{"dirname", "awk", "grep", "paste"} {
		full, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("%s is not on PATH, so a PATH that holds everything but %q cannot be built and the refusal it exists to exercise cannot be reached: %v", tool, missing, err)
		}
		if err := os.Symlink(full, filepath.Join(dir, tool)); err != nil {
			t.Fatalf("link %s: %v", tool, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, missing)); err == nil {
		t.Fatalf("the PATH built to exclude %q holds it", missing)
	}
	return dir
}

// TestGateFanoutHarnessRefusesWhatItCannotProduce covers scripts/gate-stage2/run.sh's
// `fanout` mode, running the REAL script — a copy in a fixture would pin the
// guarantee to the fixture.
//
// WHAT THE ROWS CAN AND CANNOT PROVE. A full success writes a bundle per surface
// and a record into the checkout it is pointed at, and the only checkout this
// mode can be pointed at is a real one, so there is no positive control here that
// does not write into a tree other suites are reading — the production itself is
// exercised in-process, against a fixture built from the real manifest and the
// real documents. The last row stands in for the plumbing half: it passes every
// check the shell makes on its own, and what comes back is the GO PRODUCER's own
// refusal, quoting a tree the shell never looked at. A `fanout` mode that fell
// through to usage(), that had been deleted, or that refused everything before
// reaching the producer fails it — and since every row asserts its own message
// and its own exit code, no single always-taken refusal satisfies two of them.
// gateFanoutStashRecord moves this checkout's own fan-out record aside for the
// duration of one test and puts it back afterwards.
//
// WHY EVERY TEST THAT ASSERTS "NO RECORD WAS WRITTEN" NEEDS IT. These tests
// drive the real producer against the real checkout, and then assert that a
// refused run left no `gate/fanout.json` behind. That assertion is only about
// the run under test when the baseline is genuinely "no record". During a
// release — the one moment this suite is most likely to be run — the checkout
// holds a live record from the real fan-out, and every such row fails, each
// reporting that a run which wrote nothing at all had written a file. False
// findings that read exactly like true ones.
//
// WHAT DID NOT HAPPEN, stated because the opposite is the natural guess.
// gateFanoutProduce does deliberately `os.Remove` the previous run's record —
// deliberate and right, because a refusal that left the old record in place
// would leave the driver reading a run identifier for a fan-out nobody re-ran.
// Every refusal row checks its arguments BEFORE that point, precisely so a
// mistyped tree does not cost a run somebody already paid thirteen agents for.
// So these tests never destroyed a release's evidence, and the stash is not what
// stops them from doing so. It earns its place by making the assertion mean what
// it says, and by leaving the checkout as it was found — including for a future
// row that does reach the removal.
//
// IT IS A SHARED HELPER BECAUSE IT WAS ONCE TWO COPIES AND ONLY ONE WAS FIXED.
// v0.5.2 corrected TestGateFanoutProduce_FlagContract and left this file's other
// caller with the same defect, which the v0.5.2 gate run then hit for real. One
// implementation cannot rot on one side.
func gateFanoutStashRecord(t *testing.T, root string) {
	t.Helper()
	fanoutPath := filepath.Join(root, filepath.FromSlash(gateFanoutFile))
	switch stashed, err := os.ReadFile(fanoutPath); {
	case err == nil:
		if err := os.Remove(fanoutPath); err != nil {
			t.Fatalf("the checkout holds a fan-out record at %s and it could not be moved aside: %v\nThis test drives the real producer against this checkout, and its rows assert that no record exists afterwards. Run without stashing it, the rows that produce nothing would fail over somebody else's file and the rows that produce would DELETE it", gateFanoutFile, err)
		}
		t.Cleanup(func() {
			if err := os.WriteFile(fanoutPath, stashed, 0o644); err != nil {
				t.Errorf("the fan-out record stashed from %s could not be restored: %v\nThe checkout is now missing evidence it had before this test ran, and the release driver's D1 refuses a tree whose fan-out record is absent. The bytes were:\n%s", gateFanoutFile, err, stashed)
			}
		})
	case errors.Is(err, os.ErrNotExist):
		// The ordinary case outside a release. Nothing to stash, and the rows
		// start from the baseline they assume.
	default:
		t.Fatalf("the fan-out record at %s could not be read to stash it: %v\nIt is not absent and it is not readable, so this test can neither establish its baseline nor promise to leave the checkout as it found it", gateFanoutFile, err)
	}
}

func TestGateFanoutHarnessRefusesWhatItCannotProduce(t *testing.T) {
	root := surfaceRepoRoot(t)
	gateFanoutStashRecord(t, root)

	t.Run("no tree at all", func(t *testing.T) {
		out, code := gateFanoutRunHarness(t, "", "fanout")
		if code == 0 {
			t.Fatalf("the harness fanned out with no tree named; every answer of that run would attach to no release:\n%s", out)
		}
		if code != 1 {
			t.Errorf("the harness exited %d; a missing required option is a usage error, which this script documents as exit 1:\n%s", code, out)
		}
		if !strings.Contains(out, "fanout: --tree is required") {
			t.Errorf("the refusal does not name the missing tree, so this row cannot be told apart from the mode having been deleted and usage() taking the call:\n%s", out)
		}
	})

	t.Run("a checkout that does not carry the producer", func(t *testing.T) {
		other := t.TempDir()
		gateWrite(t, other, gateManifestFile, "surfaces:\n  - name: alpha\n    paths: [alpha/]\n")
		out, code := gateFanoutRunHarness(t, "", "fanout", "--root", other, "--tree", strings.Repeat("a", 40))
		if code == 0 {
			t.Fatalf("the harness fanned out over a checkout holding no producer; there is exactly one implementation of a fan-out and this mode must not stand in for it:\n%s", out)
		}
		if !strings.Contains(out, "carries no cmd/dossierx/gate_fanout_test.go") {
			t.Errorf("the refusal does not say what the checkout is missing:\n%s", out)
		}
	})

	t.Run("no go toolchain to run the producer with", func(t *testing.T) {
		out, code := gateFanoutRunHarness(t, gateFanoutPATHWithout(t, "go"),
			"fanout", "--root", root, "--tree", strings.Repeat("a", 40))
		if code == 0 {
			t.Fatalf("the harness fanned out with no toolchain to produce anything with:\n%s", out)
		}
		if !strings.Contains(out, "no go toolchain on PATH") {
			t.Errorf("the refusal does not name the missing toolchain. A mode that printed its invocations anyway would hand the harness one path per surface, every one of them holding nothing, and an answer set attributable to no run:\n%s", out)
		}
		if strings.Contains(out, gateFanoutBundleDir+"/") {
			t.Errorf("the harness printed invocations naming bundles nothing could have written:\n%s", out)
		}
	})

	t.Run("the producer's refusal is the harness's refusal", func(t *testing.T) {
		// A well-formed object name that names nothing in any clone: every check
		// the shell makes on its own passes, the Go producer runs, and its refusal
		// quotes the tree this checkout is really at — a value the shell never
		// reads and could not have invented.
		out, code := gateFanoutRunHarness(t, "", "fanout", "--root", root, "--tree", strings.Repeat("a", 40))
		if code == 0 {
			t.Fatalf("the harness printed a fan-out the producer refused:\n%s", out)
		}
		if code != 5 {
			t.Errorf("the harness exited %d; a fan-out that could not be produced is exit 5, which its own header documents as no run having been minted:\n%s", code, out)
		}
		if !strings.Contains(out, "the producer refused this run") {
			t.Errorf("the harness did not report that the producer refused:\n%s", out)
		}
		checkoutTree, err := gateTreeSHA(root, "HEAD")
		if err != nil {
			t.Fatalf("resolve this checkout's tree: %v", err)
		}
		if !strings.Contains(out, "is at tree "+checkoutTree) {
			t.Errorf("the producer's own refusal did not reach the caller. This row is the only thing that says the mode runs the producer at all rather than refusing on its own account, and it is the only reading a human gets of WHY a fan-out did not happen:\n%s", out)
		}
		if strings.Contains(out, gateFanoutBundleDir+"/") {
			t.Errorf("the harness printed invocations naming bundles that were never written:\n%s", out)
		}
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(gateFanoutFile))); statErr == nil {
			t.Errorf("a refused fan-out left %s in the checkout under test", gateFanoutFile)
		}
	})
}
