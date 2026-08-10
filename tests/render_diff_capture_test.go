// render_diff_capture_test.go is THE PRODUCER of gate/render-diff.json — the
// cross-release render diff the stage-2 CHANGELOG agent is handed.
//
// WHY IT EXISTS. gate/render-diff.json is a named component of the `changelog`
// surface's key (cmd/dossierx/gate_stage2_test.go), gate/prompts/changelog.md
// instructs the agent to work from it ("For every artifact in the cross-release
// render diff, find the line describing what changed in rendered output"), and
// gateBundleAssemble hands it over verbatim under "a capture produced for this
// surface" (cmd/dossierx/gate_bundle_test.go) — and until this file existed
// NOTHING WROTE IT. The gate could therefore not go green on any release, and
// the only thing between that and a one-line workaround was that nobody had yet
// typed `printf '{}' > gate/render-diff.json`. Two bytes would have satisfied
// every check in the tree: the manifest is honest about the bytes, the digest
// matches, and the agent finds no artifact in the render diff and reports that
// nothing needs announcing — while a v0.4.1-class silent rendering change ships
// with a CHANGELOG that does not mention it.
//
// ONE COMPARISON, TWO RENDERINGS. It does not re-implement "what rendered
// differently since the last release". It calls compareRenderedOutput, the same
// function testdata/render-across-releases.golden.txt is produced from. A second
// implementation would be the second release procedure CLAUDE.md forbids, and
// the two would diverge in silence because only the committed one goes red.
//
// THE BASELINE AND THE TREE ARE SUPPLIED, NEVER PROBED. Every other piece of a
// run's release evidence is computed from an identity the driver hands down.
// This file therefore takes -render-diff-baseline-commit and -render-diff-tree
// and refuses anything that is not a full forty-digit object name; it never
// calls resolveBaseline. A producer that probed for itself would be a second
// answer to "which release is this being compared against" inside one run — and
// once the release's own tag exists, `git describe --tags --abbrev=0` names IT,
// so the capture would be the release compared against itself, reporting zero
// silent changes with perfect confidence.
//
// A CAPTURE IS A REFUSAL OR IT IS COMPLETE. Every way this can fail to compare
// writes nothing and fails: a flag given but empty, a baseline that is not an
// identity, an out path with no baseline, a comparison that found no artifact at
// the baseline. `{"artifacts": []}` with a coverage witness saying 147 were
// compared is a legitimate and expected answer for a release that changed no
// rendered output; it is a completely different statement from "nothing was
// compared", and the document says which on its face.
//
// HOW IT IS INVOKED. With -render-diff-out absent it produces nothing and
// asserts nothing, and it does that by RETURNING rather than by t.Skip: a skip
// in a job log is what a maintainer reads as "checked", and the correctness of
// the comparison is TestRenderedOutputAcrossReleases's job, which runs
// unconditionally.
//
//	go test ./tests -run TestRenderDiffCapture_G1Capture -args \
//	  -render-diff-out=gate/render-diff.json \
//	  -render-diff-baseline-commit=<40 hex> \
//	  -render-diff-tree=<40 hex>
package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	renderDiffCaptureOut            = flag.String("render-diff-out", "", "write the cross-release render diff (surfaces.yaml's `changelog` surface) to this path as render-diff.json")
	renderDiffCaptureBaselineCommit = flag.String("render-diff-baseline-commit", "", "the full 40-digit object name of the previous release's commit, as RESOLVED BY THE DRIVER; requires -render-diff-out")
	renderDiffCaptureTree           = flag.String("render-diff-tree", "", "the full 40-digit tree object name this run covers, as supplied by the driver; requires -render-diff-out")
)

// renderDiffCaptureObjectName is the same rule the producer and the reader of
// every other piece of this run's evidence apply: forty hexadecimal digits and
// nothing else. A TAG NAME is a mutable pointer that `git tag -f` re-points
// under anything naming only the tag, and an ABBREVIATION is a prefix that means
// a different object in a different clone. Both are answers that stop being true
// later, and the whole value of this capture is which release it was against.
var renderDiffCaptureObjectName = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ---------------------------------------------------------------------
// the document
// ---------------------------------------------------------------------

// renderDiffCaptureDoc is gate/render-diff.json.
//
// FIELD ORDER IS LOAD-BEARING, not cosmetic. scripts/gate-stage2/run.sh cross-
// checks this document's provenance at the moment it would be recorded, and it
// reads it with json_scalar — an awk one-liner that takes the FIRST `"<key>": "`
// on any line and exits, and which its own comment says is not a JSON parser and
// must not become one. A diff hunk is a JSON string that can perfectly well
// contain the literal text `"commit": "` (this repository's own shell and Go
// sources contain it). Putting tree and baseline before everything else is what
// makes the cheap reader correct rather than lucky.
type renderDiffCaptureDoc struct {
	// Tree is the tree identity the RUN covers, supplied by the driver and
	// recorded verbatim. See Head for why the two are not the same claim.
	Tree     string                    `json:"tree"`
	Baseline renderDiffCaptureBaseline `json:"baseline"`
	Head     renderDiffCaptureHead     `json:"head"`
	Coverage renderDiffCaptureCoverage `json:"coverage"`
	About    []string                  `json:"about"`
	Added    []string                  `json:"added"`
	Removed  []string                  `json:"removed"`
	// Artifacts carries THE DIFF, not a list of paths. surface.json's
	// render_fingerprint already answers "did rendered output move"; the entire
	// reason this document exists is that a CHANGELOG entry describing a silent
	// rendering change has to be written from the actual before-and-after, and
	// a filename cannot be written from.
	Artifacts []renderDiffCaptureArtifact `json:"artifacts"`
}

type renderDiffCaptureBaseline struct {
	Commit string `json:"commit"`
}

// renderDiffCaptureHead records WHICH TREE THE COMPARISON ACTUALLY READ.
//
// The head side of the comparison is the WORKING TREE and deliberately so: a
// tree with an uncommitted rendering change in it is exactly the tree a
// maintainer is about to tag, and comparing HEAD instead would blind the gate to
// precisely the change it exists to catch. But the run manifest anchors every
// key to a forty-hex tree object, so on a dirty checkout the recorded identity
// does not describe the bytes this capture was computed from.
//
// The document therefore says which of the two it did. DiffersFromHead is the
// `git status --porcelain` of the compared scope; empty means the working tree
// and HEAD agree there and the recorded Tree describes the compared bytes
// exactly. Refusing a dirty tree outright was the alternative and it is worse:
// it would refuse the very state the comparison was designed for.
type renderDiffCaptureHead struct {
	Side            string   `json:"side"`
	MatchesHead     bool     `json:"matches_head"`
	DiffersFromHead []string `json:"differs_from_head"`
	ComparedScope   string   `json:"compared_scope"`
}

// renderDiffCaptureCoverage is the witness that makes an empty Artifacts list
// readable. "Nothing rendered differently" and "nothing was compared" are
// opposite statements and an artifacts array cannot tell them apart on its own.
type renderDiffCaptureCoverage struct {
	Compared  int `json:"compared"`
	Added     int `json:"added"`
	Removed   int `json:"removed"`
	Silent    int `json:"silent"`
	Explained int `json:"explained"`
}

type renderDiffCaptureArtifact struct {
	Path string `json:"path"`
	// Classification is "silent" or "explained". It is a LABEL ON EVIDENCE that
	// is present either way — see compareRenderedOutput for the reproduced
	// hole that made suppressing the explained side's diff a mistake.
	Classification string   `json:"classification"`
	ChangedInputs  []string `json:"changed_inputs"`
	BaseLines      int      `json:"base_lines"`
	BaseBytes      int      `json:"base_bytes"`
	HeadLines      int      `json:"head_lines"`
	HeadBytes      int      `json:"head_bytes"`
	AddedLines     int      `json:"added_lines"`
	RemovedLines   int      `json:"removed_lines"`
	TooLarge       bool     `json:"too_large"`
	Diff           string   `json:"diff"`
}

const renderDiffCaptureScope = "testdata"

// renderDiffCaptureAbout is the document's account of itself, handed to the
// agent verbatim along with everything else. It is here because the one reading
// error this document can produce is fatal and silent: taking an empty
// `artifacts` list for "this release changed no rendered output" when in fact
// nothing was compared.
var renderDiffCaptureAbout = []string{
	"The cross-release render diff for this run: every artifact under testdata/ that renders",
	"differently at this tree than it did at the baseline commit above, with the diff.",
	"coverage.compared is how many rendered artifacts were present at BOTH revisions. An empty",
	"artifacts list with a non-zero coverage.compared means this release changed no rendered output.",
	"A run that could not compare writes no document at all, so an empty artifacts list is",
	"never a report of a comparison that did not happen.",
	"classification is silent (inputs byte-identical, rendered output moved: this needs a",
	"CHANGELOG line) or explained (its own inputs moved too). Both carry the same diff.",
}

// renderDiffCaptureDocument renders one comparison into the document. Pure: it
// asks nothing of git and reads nothing from disk, so the unit test below can
// build a comparison with real moves in it without touching the shared tree.
func renderDiffCaptureDocument(tree, commit string, drift []string, cmp renderComparison) renderDiffCaptureDoc {
	doc := renderDiffCaptureDoc{
		Tree:     tree,
		Baseline: renderDiffCaptureBaseline{Commit: commit},
		Head: renderDiffCaptureHead{
			Side:            "working-tree",
			MatchesHead:     len(drift) == 0,
			DiffersFromHead: append([]string{}, drift...),
			ComparedScope:   renderDiffCaptureScope,
		},
		Coverage: renderDiffCaptureCoverage{
			Compared:  len(cmp.compared),
			Added:     len(cmp.added),
			Removed:   len(cmp.removed),
			Silent:    len(cmp.silent),
			Explained: len(cmp.explained),
		},
		About:     renderDiffCaptureAbout,
		Added:     append([]string{}, cmp.added...),
		Removed:   append([]string{}, cmp.removed...),
		Artifacts: []renderDiffCaptureArtifact{},
	}
	for _, m := range cmp.silent {
		doc.Artifacts = append(doc.Artifacts, renderDiffCaptureEntry(m, "silent"))
	}
	for _, m := range cmp.explained {
		doc.Artifacts = append(doc.Artifacts, renderDiffCaptureEntry(m, "explained"))
	}
	return doc
}

// renderDiffCaptureMarshal is the ONE encoding of this document, and it turns
// HTML escaping OFF.
//
// encoding/json escapes `<`, `>` and `&` as <, > and & by
// default, for a browser-context hazard this file has nothing to do with. Every
// artifact this capture compares is HTML — the markdown cases' .golden.html and
// the fixture viewers' index.html — so under the default every angle bracket of
// every hunk reaches the CHANGELOG agent as a six-character escape. The document
// exists so that a human-readable before-and-after can be turned into a
// CHANGELOG line, and `<span class=\"comment-chip-count\">` is not one.
// This is a legibility rule, not a correctness one, which is exactly why it
// needs a test: nothing would ever go red.
func renderDiffCaptureMarshal(doc renderDiffCaptureDoc) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	// Encode appends the trailing newline itself.
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderDiffCaptureEntry(m renderMove, classification string) renderDiffCaptureArtifact {
	return renderDiffCaptureArtifact{
		Path:           m.path,
		Classification: classification,
		ChangedInputs:  append([]string{}, m.changedInputs...),
		BaseLines:      m.baseLines,
		BaseBytes:      m.baseBytes,
		HeadLines:      m.headLines,
		HeadBytes:      m.headBytes,
		AddedLines:     m.added,
		RemovedLines:   m.removed,
		TooLarge:       m.tooLarge,
		Diff:           m.diff,
	}
}

// renderDiffCaptureRefusal returns the reason this document must not be written,
// or "" if it may be. Kept apart from the flag handling so every refusal is
// exercisable in-process rather than only through a real gate run.
//
// It is the last gate before the rename, and it is deliberately about the
// DOCUMENT rather than about the flags: a capture that reaches disk having lost
// its provenance or its coverage witness is the one failure nothing downstream
// can detect, because every downstream check is about bytes and digests and both
// would be perfectly honest.
func renderDiffCaptureRefusal(doc renderDiffCaptureDoc) string {
	if !renderDiffCaptureObjectName.MatchString(doc.Tree) {
		return "the capture records tree " + renderDiffCaptureQuote(doc.Tree) + ", which is not a full 40-digit object name; a capture that cannot say which tree it covers cannot be checked against the run that records it"
	}
	if !renderDiffCaptureObjectName.MatchString(doc.Baseline.Commit) {
		return "the capture records baseline commit " + renderDiffCaptureQuote(doc.Baseline.Commit) + ", which is not a full 40-digit object name; a tag is a mutable pointer and an abbreviation is a prefix, and either can mean a different release tomorrow"
	}
	if doc.Coverage.Compared == 0 {
		return "the comparison covered 0 rendered artifacts present at both revisions, so an empty artifacts list would read as \"this release changed no rendered output\" when it means \"nothing was compared\"; this is a FAILED run, never a clean capture"
	}
	return ""
}

func renderDiffCaptureQuote(s string) string {
	if s == "" {
		return "(empty)"
	}
	return "\"" + s + "\""
}

// ---------------------------------------------------------------------
// the entry point
// ---------------------------------------------------------------------

// TestRenderDiffCapture_G1Capture is how a gate run produces
// gate/render-diff.json. See this file's header for the invocation.
//
// PRESENCE, NOT VALUE. Every flag decision below keys off
// flag.CommandLine.Visit rather than off the string being empty. A driver
// invoking `-render-diff-out=$OUT` with OUT unset expands to the flag being
// GIVEN AN EMPTY VALUE, which `== ""` cannot tell apart from the flag never
// having been passed — and under a value check that run writes nothing, exits 0,
// and the driver reads the exit code as a capture that was produced. This is the
// same gap tests/release_notes_predict_test.go and
// tests/skills_export_capture_test.go each closed the same way.
func TestRenderDiffCapture_G1Capture(t *testing.T) {
	var outGiven, baselineGiven, treeGiven bool
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "render-diff-out":
			outGiven = true
		case "render-diff-baseline-commit":
			baselineGiven = true
		case "render-diff-tree":
			treeGiven = true
		}
	})

	if !outGiven {
		// -render-diff-baseline-commit and -render-diff-tree each IMPLY
		// -render-diff-out: a caller that resolved a baseline and named a tree
		// meant to produce a capture, not to no-op.
		if baselineGiven || treeGiven {
			t.Fatalf("-render-diff-baseline-commit (given=%v) or -render-diff-tree (given=%v) was given without -render-diff-out; both imply it, and a run that resolves a baseline and then writes nothing is a capture the driver will believe it has", baselineGiven, treeGiven)
		}
		// The ordinary `go test ./tests` invocation, and every CI run of this
		// suite. It RETURNS rather than skipping: a SKIP line in a job log is
		// what a maintainer reads as "checked", and there is nothing here to
		// check — the correctness of the comparison belongs to
		// TestRenderedOutputAcrossReleases, which runs unconditionally.
		t.Logf("no -render-diff-out given; this test is the gate's capture entry point, not a correctness check (TestRenderedOutputAcrossReleases is that, and it has already run)")
		return
	}
	if *renderDiffCaptureOut == "" {
		t.Fatalf("-render-diff-out was given but is empty (e.g. a driver expanding an unset shell variable); gate/render-diff.json would silently not be written while `go test` still exits 0, and the CHANGELOG agent would be handed whatever was already at that path")
	}
	if !baselineGiven {
		t.Fatalf("-render-diff-out was given without -render-diff-baseline-commit; this capture never resolves a baseline for itself, because a second resolver inside one run is a second answer to which release the comparison was against — and once this release's own tag exists, probing names IT and the capture becomes the release compared against itself")
	}
	if !treeGiven {
		t.Fatalf("-render-diff-out was given without -render-diff-tree; a capture that does not say which tree it covers cannot be cross-checked against the run that records it, and would be carried forward under any tree at all")
	}
	if !renderDiffCaptureObjectName.MatchString(*renderDiffCaptureBaselineCommit) {
		t.Fatalf("-render-diff-baseline-commit is %s and must be a full 40-digit object name. A tag is a mutable pointer, an abbreviation is a prefix, and forty characters of something else is neither. An unresolvable baseline is a FAILED run; it is never an empty render diff.", renderDiffCaptureQuote(*renderDiffCaptureBaselineCommit))
	}
	if !renderDiffCaptureObjectName.MatchString(*renderDiffCaptureTree) {
		t.Fatalf("-render-diff-tree is %s and must be a full 40-digit object name, the same identity the run manifest anchors every key to", renderDiffCaptureQuote(*renderDiffCaptureTree))
	}

	commit := *renderDiffCaptureBaselineCommit

	// The comparison. Not re-implemented here: this is the function
	// testdata/render-across-releases.golden.txt is produced from, so the
	// document below and the committed report are two renderings of one
	// comparison rather than two comparisons that agree today.
	//
	// The baseline is passed in BOTH positions on purpose: the label is what
	// appears in this function's refusal messages, and the driver supplies an
	// identity rather than a tag name, so the identity is the most truthful
	// label available. resolveBaseline is not called and must not be.
	cmp := compareRenderedOutput(t, commit, commit)

	doc := renderDiffCaptureDocument(*renderDiffCaptureTree, commit, renderDiffCaptureWorkingTreeDrift(t, repoRootDir(t)), cmp)
	if reason := renderDiffCaptureRefusal(doc); reason != "" {
		t.Fatalf("refusing to write %s: %s", *renderDiffCaptureOut, reason)
	}

	data, err := renderDiffCaptureMarshal(doc)
	if err != nil {
		t.Fatalf("marshal the render diff: %v", err)
	}
	renderDiffCaptureWrite(t, *renderDiffCaptureOut, data)
	t.Logf("wrote the cross-release render diff to %s: compared %d, silent %d, explained %d, against %s",
		*renderDiffCaptureOut, doc.Coverage.Compared, doc.Coverage.Silent, doc.Coverage.Explained, commit)
}

// renderDiffCaptureWorkingTreeDrift is the `git status --porcelain` of the
// compared scope: what the working tree the comparison read has that HEAD does
// not. See renderDiffCaptureHead for why the document records it rather than
// refusing.
// It takes the directory to ask rather than asking the repository root
// directly, because the state it reports — a dirty testdata/ — cannot be
// manufactured in this repository without reddening every other suite running
// in the same checkout. The entry point passes the repository root; the test
// below passes a throwaway repository it dirties on purpose.
func renderDiffCaptureWorkingTreeDrift(t *testing.T, dir string) []string {
	var drift []string
	for _, line := range strings.Split(renderDiffCaptureGit(t, dir, "status", "--porcelain", "--", renderDiffCaptureScope), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		drift = append(drift, strings.TrimRight(line, "\r"))
	}
	return drift
}

// renderDiffCaptureGit is gitOut with the directory supplied rather than fixed
// to the repository root. A git that fails is fatal, never an empty answer: an
// empty answer here reads as "the working tree matches HEAD", which is the one
// thing this capture must never say without having checked.
func renderDiffCaptureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, errb.String())
	}
	return out.String()
}

// renderDiffCaptureWrite writes through a temp file in the destination directory
// and renames, so a failure partway through cannot leave a document that parses
// far enough to look like a run that compared less than it was asked to. This is
// the reasoning scripts/gate-stage2/run.sh already applies to gate/run.json ("a
// truncated manifest is worse than none"), one file over.
func renderDiffCaptureWrite(t *testing.T, dest string, data []byte) {
	t.Helper()
	dir := filepath.Dir(dest)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create the directory for %s: %v", dest, err)
	}
	f, err := os.CreateTemp(dir, ".render-diff-capture-*.json")
	if err != nil {
		t.Fatalf("create a temporary file beside %s: %v", dest, err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatalf("write %s: %v", tmp, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", tmp, err)
	}
	// CreateTemp makes the file 0600; the capture is read by the harness that
	// assembles the bundle, which may not be this process's user.
	if err := os.Chmod(tmp, 0o644); err != nil {
		t.Fatalf("chmod %s: %v", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		t.Fatalf("rename %s to %s: %v", tmp, dest, err)
	}
}

// ---------------------------------------------------------------------
// the document is the diff, not a path list
// ---------------------------------------------------------------------

// TestRenderDiffCaptureDocumentCarriesTheDiff pins the one property that decides
// whether this capture is worth producing at all.
//
// An agent handed `{"artifacts": ["testdata/fixture-basic/viewer/index.html"]}`
// can say something moved. It cannot write the CHANGELOG line, which is the only
// thing that fixes a silent rendering change — and "something moved" is already
// answered, for free, by surface.json's render_fingerprint. So the document has
// to carry the hunks, and it has to carry them in BOTH sections: W8b established
// by experiment that suppressing the explained side's diff let one edited byte
// anywhere in a fixture hide a byte-length-preserving CSS class rename in that
// fixture's ten-thousand-line viewer.
//
// It builds the comparison rather than observing one, on purpose. A real move
// exists only when the working tree differs from the last release, which is not
// true of a clean checkout — and dirtying testdata/ to manufacture one would
// redden the suite for anything else running in this tree at the same time.
func TestRenderDiffCaptureDocumentCarriesTheDiff(t *testing.T) {
	silentDiff := "@@ -12,3 +12,3 @@\n <footer>\n-<span class=\"comment-chip-count\">3</span>\n+<span class=\"comment-chip-tally\">3</span>\n"
	explainedDiff := "@@ -4,2 +4,3 @@\n <h1>Claims</h1>\n+<p>a new claim</p>\n"

	cmp := renderComparison{
		compared: []string{"a.golden.html", "b.golden.html", "c.golden.html"},
		added:    []string{"new.golden.html"},
		removed:  []string{"gone.golden.html"},
		silent: []renderMove{{
			path:      "testdata/fixture-basic/viewer/index.html",
			baseLines: 10000, baseBytes: 400000, headLines: 10000, headBytes: 400000,
			diff: silentDiff, added: 1, removed: 1,
		}},
		explained: []renderMove{{
			path:          "testdata/fixture-comments/viewer/index.html",
			changedInputs: []string{"testdata/fixture-comments/claims/api/x.yaml"},
			baseLines:     9000, baseBytes: 300000, headLines: 9001, headBytes: 300020,
			diff: explainedDiff, added: 1, removed: 0,
		}},
	}

	doc := renderDiffCaptureDocument(strings.Repeat("a", 40), strings.Repeat("b", 40), nil, cmp)
	if reason := renderDiffCaptureRefusal(doc); reason != "" {
		t.Fatalf("an honest comparison was refused: %s", reason)
	}

	if len(doc.Artifacts) != 2 {
		t.Fatalf("the document carries %d artifacts, want the silent one and the explained one", len(doc.Artifacts))
	}
	byPath := map[string]renderDiffCaptureArtifact{}
	for _, a := range doc.Artifacts {
		byPath[a.Path] = a
	}

	silent, ok := byPath["testdata/fixture-basic/viewer/index.html"]
	if !ok {
		t.Fatalf("the silent move is missing from the document: %+v", doc.Artifacts)
	}
	if silent.Classification != "silent" {
		t.Errorf("the silent move is classified %q; the CHANGELOG agent decides what needs announcing from this label", silent.Classification)
	}
	if silent.Diff != silentDiff {
		t.Errorf("the silent move's diff is %q, want the hunk the comparison produced.\nA capture that names the file and drops the hunk says no more than surface.json's render_fingerprint already says, and a CHANGELOG line cannot be written from it.", silent.Diff)
	}

	explained, ok := byPath["testdata/fixture-comments/viewer/index.html"]
	if !ok {
		t.Fatalf("the explained move is missing from the document: %+v", doc.Artifacts)
	}
	if explained.Classification != "explained" {
		t.Errorf("the explained move is classified %q", explained.Classification)
	}
	if explained.Diff != explainedDiff {
		t.Errorf("the explained move's diff is %q, want the hunk.\nThis section carries the SAME evidence as the silent one — one edited byte anywhere in a fixture reclassifies that fixture's whole viewer, and a section that suppressed the diff would hide a renamed CSS class behind it. Reproduced, not theorised (tests/render_across_releases_test.go).", explained.Diff)
	}
	if !renderDiffCaptureEqualStrings(explained.ChangedInputs, []string{"testdata/fixture-comments/claims/api/x.yaml"}) {
		t.Errorf("the explained move records changed inputs %v, want the input that moved; without it a reader cannot tell which hunks the explanation is supposed to account for", explained.ChangedInputs)
	}

	want := renderDiffCaptureCoverage{Compared: 3, Added: 1, Removed: 1, Silent: 1, Explained: 1}
	if doc.Coverage != want {
		t.Errorf("the coverage witness is %+v, want %+v", doc.Coverage, want)
	}
}

// TestRenderDiffCaptureProvenanceComesFirst is the direct check on the thing
// that makes the shell-side cross-check correct rather than lucky.
//
// scripts/gate-stage2/run.sh reads this document with json_scalar, which takes
// the FIRST `"<key>": "` on any line and exits — by design; its own comment says
// it is not a JSON parser and must not become one. So the document's guarantee
// to that reader can only ever be POSITIONAL: tree and baseline.commit come
// before everything else, and then no later key, nested field or hunk can be
// read in their place.
//
// It asserts the position and not merely "json_scalar happens to return the
// right value", because those are different claims. encoding/json escapes a
// quote inside a string as `\"`, so a hunk cannot forge the pattern TODAY — but
// that is a property of the writer, and this capture is a plain JSON file that
// a later lane, a hand-written stand-in, or a different encoder can produce
// without it. Ordering is the property the reader actually depends on, so
// ordering is what is pinned. A hunk built to look like provenance rides along
// as a second, weaker assertion.
func TestRenderDiffCaptureProvenanceComesFirst(t *testing.T) {
	tree, commit := strings.Repeat("a", 40), strings.Repeat("b", 40)
	poison := "@@ -1,1 +1,2 @@\n-  printf '  \"tree\": \"%s\",\\n' \"$TREE\"\n+  printf '  \"tree\": \"%s\",\\n' \"$OTHER\"\n+  printf '  \"commit\": \"deadbeef\"\\n'\n"

	doc := renderDiffCaptureDocument(tree, commit, nil, renderComparison{
		compared: []string{"x.golden.html"},
		silent:   []renderMove{{path: "x.golden.html", diff: poison, added: 2, removed: 1}},
	})
	data, err := renderDiffCaptureMarshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	keys := renderDiffCaptureTopLevelKeys(string(data))
	if len(keys) < 2 || keys[0] != "tree" || keys[1] != "baseline" {
		t.Fatalf("the document's keys are %v; run.sh reads it with an awk one-liner that takes the FIRST \"tree\" and the FIRST \"commit\" it sees on any line, so provenance must be the first two keys. With it anywhere else, the record-time cross-check compares this run against whatever a later key or a diff hunk happens to say, and a capture from another release records cleanly.", keys)
	}

	if got := renderDiffCaptureFirstScalar(string(data), "tree"); got != tree {
		t.Errorf("the first \"tree\" scalar in the document is %q, want the run's tree %q", got, tree)
	}
	if got := renderDiffCaptureFirstScalar(string(data), "commit"); got != commit {
		t.Errorf("the first \"commit\" scalar in the document is %q, want the baseline %q", got, commit)
	}
}

// TestRenderDiffCaptureHunksReachTheAgentAsHTML pins the legibility of the one
// thing this document exists to carry.
//
// EVERY artifact compared here is HTML — the markdown cases' .golden.html and
// the three fixture viewers' index.html — so every hunk is angle brackets almost
// all the way down. encoding/json's default escaping turns each one into
// a six-character `\u003c` / `\u003e` escape, and gateBundleAssemble hands the
// file to the CHANGELOG
// agent VERBATIM. A capture in that form still digests cleanly, still passes
// every provenance check, and still contains the whole diff; it is simply not
// something a release note can be written from, and nothing anywhere else would
// ever go red over it. That is precisely why it is asserted here.
func TestRenderDiffCaptureHunksReachTheAgentAsHTML(t *testing.T) {
	hunk := "@@ -12,3 +12,3 @@\n <footer>\n-<span class=\"comment-chip-count\">3 &amp; more</span>\n+<span class=\"comment-chip-tally\">3 &amp; more</span>\n"
	doc := renderDiffCaptureDocument(strings.Repeat("a", 40), strings.Repeat("b", 40), nil, renderComparison{
		compared: []string{"testdata/fixture-basic/viewer/index.html"},
		silent:   []renderMove{{path: "testdata/fixture-basic/viewer/index.html", diff: hunk, added: 1, removed: 1}},
	})
	data, err := renderDiffCaptureMarshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The JSON escapes themselves, spelled as literal backslash-u sequences.
	for _, escape := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(string(data), escape) {
			t.Errorf("the capture contains %s. The CHANGELOG agent is handed this document verbatim and every artifact compared is HTML, so under the default escaping the entire diff arrives as unicode escapes and no release note can be written from it.", escape)
		}
	}
	if !strings.Contains(string(data), `+<span class=\"comment-chip-tally\">3 &amp; more</span>`) {
		t.Errorf("the hunk did not survive into the document as readable HTML:\n%s", data)
	}
}

// renderDiffCaptureTopLevelKeys reads the top-level keys of a document produced
// by renderDiffCaptureMarshal with a two-space indent, in order. A
// top-level key is a line beginning with exactly two spaces and a quote — the
// same shape scripts/gate-stage2/run.sh's top_level_blocks already relies on for
// surface.json.
func renderDiffCaptureTopLevelKeys(doc string) []string {
	var keys []string
	for _, line := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(line, `  "`) {
			continue
		}
		rest := line[3:]
		i := strings.Index(rest, `"`)
		if i < 0 || !strings.HasPrefix(rest[i+1:], ":") {
			continue
		}
		keys = append(keys, rest[:i])
	}
	return keys
}

// renderDiffCaptureFirstScalar mirrors scripts/gate-stage2/run.sh's json_scalar
// exactly: the first line matching `"<key>"<space>*:<space>*"`, everything up to
// the next quote, then stop. It is a MIRROR rather than a JSON lookup on
// purpose — the property under test is what the shell reader sees, and a
// json.Unmarshal here would pass over a document json_scalar reads wrongly.
func renderDiffCaptureFirstScalar(doc, key string) string {
	pattern := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"[ \t]*:[ \t]*"`)
	for _, line := range strings.Split(doc, "\n") {
		loc := pattern.FindStringIndex(line)
		if loc == nil {
			continue
		}
		rest := line[loc[1]:]
		if i := strings.Index(rest, `"`); i >= 0 {
			return rest[:i]
		}
		return rest
	}
	return ""
}

func renderDiffCaptureEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------
// the capture says WHICH tree it read
// ---------------------------------------------------------------------

// TestRenderDiffCaptureRecordsWorkingTreeDrift pins the one claim in this
// document that no other check in the tree can make.
//
// The comparison's head side is the WORKING TREE, deliberately: a tree with an
// uncommitted rendering change in it is exactly the tree a maintainer is about
// to tag. The run manifest, by contrast, anchors every key to a forty-hex tree
// object. So on a dirty checkout the capture is recorded under an identity that
// does not describe the bytes it was computed from, and a later re-run over
// that same tree sha carries the verdict forward. head.matches_head and
// head.differs_from_head are the whole of the document's answer to which of the
// two it did — and both are computed rather than asserted anywhere else, so a
// producer that reported "clean" unconditionally would write a capture claiming
// to cover a tree nobody released, digest it honestly, and go green.
//
// It runs against a THROWAWAY repository it dirties on purpose. Dirtying this
// repository's testdata/ would redden every other suite running in the same
// checkout, and a fixture that only pretended to be dirty would pin the claim
// to the fixture rather than to `git status`.
func TestRenderDiffCaptureRecordsWorkingTreeDrift(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is not on PATH, so the capture's account of which tree it read cannot be exercised. A check that cannot run is a failure, not a pass.")
	}

	repo := t.TempDir()
	// REPO-RELATIVE, FORWARD SLASHES, ON EVERY PLATFORM — which is the one
	// spelling this document is written in. `git status --porcelain` names paths
	// that way on Windows as much as anywhere else, and the drift list the
	// capture records is that output verbatim, so this is the spelling the
	// recorded artifact carries and therefore the spelling to assert against.
	// Building it with filepath.Join instead made the expectation
	// `testdata\fixture-basic\viewer\index.html` on Windows and the assertion
	// below fail against a capture that was entirely correct.
	artifact := renderDiffCaptureScope + "/fixture-basic/viewer/index.html"
	write := func(rel, body string) {
		t.Helper()
		// The one place the name is allowed to change shape: touching the disk.
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("create the directory for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write(artifact, "<html><body><span class=\"comment-chip-count\">3</span></body></html>\n")
	write("README.md", "outside the compared scope\n")
	// The identity and the ignore file are supplied inline so the answer does
	// not depend on the machine's git configuration.
	renderDiffCaptureGit(t, repo, "-c", "init.defaultBranch=main", "init", "-q")
	renderDiffCaptureGit(t, repo, "-c", "core.excludesFile=/dev/null", "add", "-A")
	renderDiffCaptureGit(t, repo,
		"-c", "user.name=render diff capture", "-c", "user.email=capture@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "the released tree")

	// The Head block as the document would carry it, built from the real
	// helper rather than from a literal: what is under test is the pair, not
	// either half.
	head := func(t *testing.T) renderDiffCaptureHead {
		t.Helper()
		return renderDiffCaptureDocument(
			strings.Repeat("a", 40), strings.Repeat("b", 40),
			renderDiffCaptureWorkingTreeDrift(t, repo),
			renderComparison{compared: []string{"x.golden.html"}},
		).Head
	}

	t.Run("a clean checkout is reported as covering the tree it names", func(t *testing.T) {
		got := head(t)
		if !got.MatchesHead || len(got.DiffersFromHead) != 0 {
			t.Fatalf("a checkout with nothing uncommitted under %s reports matches_head=%v differs_from_head=%v; a capture that cries drift on a clean tree makes the honest signal unreadable, and every row below would then pass over an answer that is always \"dirty\"", renderDiffCaptureScope, got.MatchesHead, got.DiffersFromHead)
		}
		if got.Side != "working-tree" || got.ComparedScope != renderDiffCaptureScope {
			t.Errorf("the capture reports side %q over scope %q, want the working tree over %q; the drift list means nothing without saying what it was taken over", got.Side, got.ComparedScope, renderDiffCaptureScope)
		}
	})

	t.Run("an edit outside the compared scope is not drift", func(t *testing.T) {
		write("README.md", "outside the compared scope, edited\n")
		got := head(t)
		if !got.MatchesHead {
			t.Errorf("an uncommitted edit to README.md was reported as drift in the rendered output the comparison read: %v. The capture's tree claim is about %s, and a producer that reports the whole checkout marks every ordinary release dirty until the claim is ignored.", got.DiffersFromHead, renderDiffCaptureScope)
		}
	})

	t.Run("an uncommitted rendering change is reported truthfully", func(t *testing.T) {
		write(artifact, "<html><body><span class=\"comment-chip-tally\">3</span></body></html>\n")
		got := head(t)
		if got.MatchesHead {
			t.Fatalf("the capture claims matches_head=true while %s is uncommitted. That is the failure this field exists for: the comparison read bytes that are in no commit, the manifest records it under a forty-hex tree that does not describe them, and a later re-run over that tree sha carries the verdict forward for a tree nobody released.", artifact)
		}
		if len(got.DiffersFromHead) == 0 {
			t.Fatalf("matches_head=false with an empty differs_from_head; the reader is told the capture covers unreleased bytes and not which ones")
		}
		var named bool
		for _, line := range got.DiffersFromHead {
			if strings.Contains(line, artifact) {
				named = true
			}
		}
		if !named {
			t.Errorf("differs_from_head is %v and does not name %s, which is the artifact that moved", got.DiffersFromHead, artifact)
		}
	})
}

// ---------------------------------------------------------------------
// a capture is a refusal or it is complete
// ---------------------------------------------------------------------

// TestRenderDiffCaptureRefusesRatherThanNarrowing covers the last gate before
// the rename. Each row is a document that would be perfectly honest about its
// own bytes and would digest cleanly into the changelog surface's key, and each
// is one the run must not write.
func TestRenderDiffCaptureRefusesRatherThanNarrowing(t *testing.T) {
	hex := strings.Repeat("a", 40)
	base := func() renderDiffCaptureDoc {
		return renderDiffCaptureDocument(hex, strings.Repeat("b", 40), nil, renderComparison{
			compared: []string{"x.golden.html"},
		})
	}

	if reason := renderDiffCaptureRefusal(base()); reason != "" {
		t.Fatalf("the positive control was refused, so every row below would pass over a refusal that fires unconditionally: %s", reason)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*renderDiffCaptureDoc)
		want   string
	}{
		{"no tree at all", func(d *renderDiffCaptureDoc) { d.Tree = "" }, "not a full 40-digit object name"},
		{"a tree that is an abbreviation", func(d *renderDiffCaptureDoc) { d.Tree = "3217a48" }, "not a full 40-digit object name"},
		{"no baseline at all", func(d *renderDiffCaptureDoc) { d.Baseline.Commit = "" }, "not a full 40-digit object name"},
		{"a baseline that is a tag", func(d *renderDiffCaptureDoc) { d.Baseline.Commit = "v0.5.0" }, "not a full 40-digit object name"},
		{"a baseline in upper case", func(d *renderDiffCaptureDoc) { d.Baseline.Commit = strings.Repeat("B", 40) }, "not a full 40-digit object name"},
		{"a comparison that compared nothing", func(d *renderDiffCaptureDoc) { d.Coverage.Compared = 0 }, "nothing was compared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := base()
			tc.mutate(&doc)
			reason := renderDiffCaptureRefusal(doc)
			if reason == "" {
				t.Fatalf("the capture would have been written. Downstream it is indistinguishable from a clean comparison: the manifest is honest about the bytes, the digest matches, and the CHANGELOG agent finds no artifact and reports that nothing needs announcing.")
			}
			if !strings.Contains(reason, tc.want) {
				t.Errorf("the refusal reads %q, want it to name %q", reason, tc.want)
			}
		})
	}
}

// TestRenderDiffCapture_G1Capture_FlagContract exercises the flag handling by
// running the entry point in a subprocess, which is the only way to observe it:
// flag values are process-global and this binary only ever sees the flags it was
// started with.
func TestRenderDiffCapture_G1Capture_FlagContract(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("the go toolchain is not on PATH, so the flag contract of the gate's render-diff producer cannot be exercised. A check that cannot run is a failure, not a pass.")
	}
	root := repoRoot(t)

	run := func(t *testing.T, args ...string) (output string, code int) {
		t.Helper()
		full := append([]string{"test", "./tests", "-run", "^TestRenderDiffCapture_G1Capture$", "-count=1", "-v", "-args"}, args...)
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

	tree := strings.TrimSpace(gitOut(t, "rev-parse", "HEAD^{tree}"))
	commit := strings.TrimSpace(gitOut(t, "rev-parse", *renderDiffCaptureBaselineForTest+"^{commit}"))

	t.Run("no flags at all passes without writing and WITHOUT skipping", func(t *testing.T) {
		out, code := run(t)
		if code != 0 {
			t.Fatalf("the ordinary `go test ./tests` invocation must not fail, got exit %d:\n%s", code, out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("the capture entry point SKIPPED. A skip in a job log is what a maintainer reads as \"checked\"; with no -render-diff-out there is nothing to check and the test must simply pass:\n%s", out)
		}
	})

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			"-render-diff-out given but empty",
			[]string{"-render-diff-out="},
			"-render-diff-out was given but is empty",
		},
		{
			"a baseline given with no out path",
			[]string{"-render-diff-baseline-commit=" + commit},
			"was given without -render-diff-out",
		},
		{
			"an out path with no baseline",
			[]string{"-render-diff-out=OUT", "-render-diff-tree=" + tree},
			"without -render-diff-baseline-commit",
		},
		{
			"an out path with no tree",
			[]string{"-render-diff-out=OUT", "-render-diff-baseline-commit=" + commit},
			"without -render-diff-tree",
		},
		{
			"a baseline that is a tag rather than an identity",
			[]string{"-render-diff-out=OUT", "-render-diff-baseline-commit=v0.5.0", "-render-diff-tree=" + tree},
			"must be a full 40-digit object name",
		},
		{
			"a baseline that is an abbreviation",
			[]string{"-render-diff-out=OUT", "-render-diff-baseline-commit=" + commit[:7], "-render-diff-tree=" + tree},
			"must be a full 40-digit object name",
		},
		{
			"a tree that is not an identity",
			[]string{"-render-diff-out=OUT", "-render-diff-baseline-commit=" + commit, "-render-diff-tree=HEAD"},
			"must be a full 40-digit object name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outPath := filepath.Join(t.TempDir(), "render-diff.json")
			args := make([]string, len(tc.args))
			for i, a := range tc.args {
				args[i] = strings.Replace(a, "OUT", outPath, 1)
			}
			out, code := run(t, args...)
			if code == 0 {
				t.Fatalf("expected a refusal, got exit 0. The driver would read that as a capture produced for this run:\n%s", out)
			}
			if strings.Contains(out, "--- SKIP") {
				t.Errorf("expected a FAIL, not a SKIP:\n%s", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected the refusal to name %q, got:\n%s", tc.want, out)
			}
			if _, err := os.Stat(outPath); err == nil {
				t.Errorf("a capture was written for a run that refused; downstream it is indistinguishable from a comparison that succeeded")
			}
		})
	}

	t.Run("a real invocation writes a provenanced capture (positive control)", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "render-diff.json")
		out, code := run(t,
			"-render-diff-out="+outPath,
			"-render-diff-baseline-commit="+commit,
			"-render-diff-tree="+tree)
		if code != 0 {
			t.Fatalf("the producer refused an honest run, so every refusal above would pass over a check that fires unconditionally: exit %d\n%s", code, out)
		}
		raw, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("the capture was not written: %v\n%s", err, out)
		}
		var doc renderDiffCaptureDoc
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("the capture does not parse: %v\n%s", err, raw)
		}
		if doc.Tree != tree || doc.Baseline.Commit != commit {
			t.Errorf("the capture records tree %q baseline %q; the run supplied %q and %q", doc.Tree, doc.Baseline.Commit, tree, commit)
		}
		if doc.Coverage.Compared == 0 {
			t.Errorf("the capture reports 0 rendered artifacts compared; an empty artifacts list would then be unreadable")
		}
		if got := renderDiffCaptureFirstScalar(string(raw), "commit"); got != commit {
			t.Errorf("run.sh's json_scalar would read the baseline as %q, want %q", got, commit)
		}
	})
}

// ---------------------------------------------------------------------
// the provenance is re-checked at the moment the capture is recorded
// ---------------------------------------------------------------------

// TestRenderDiffCaptureProvenanceIsCrossCheckedAtRecordTime runs the REAL
// scripts/gate-stage2/run.sh, because a copy of it in a fixture would pin the
// guarantee to the fixture.
//
// WHAT IT IS FOR. Producing an honest capture is only half of the invariant. The
// other half is that a run cannot RECORD a capture belonging to another tree or
// another release, and the sequence that produces one is ordinary rather than
// adversarial: a gate FAILS on something unrelated, a fix lands, the tree moves,
// and the driver re-runs `record` but not the captures. Without this, the
// CHANGELOG agent is handed "the silent render changes since v0.4.1" beside a
// release delta computed against v0.5.0, and writes an entry describing two
// releases' worth of change as this one's.
//
// gate/delta.json has had this cross-check since W6 and gate/render-diff.json
// had none — the guard was scoped to one path by a literal string comparison.
// Every row below fails if that literal comes back.
func TestRenderDiffCaptureProvenanceIsCrossCheckedAtRecordTime(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Fatalf("bash is not on PATH, so the record-time cross-check on the render diff cannot be exercised. A check that cannot run is a failure, not a pass.")
	}
	script := filepath.Join(repoRoot(t), "scripts", "gate-stage2", "run.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("the stage-2 harness is not in the tree: %v", err)
	}

	tree := strings.Repeat("a", 40)
	commit := strings.Repeat("b", 40)

	honest := func(tree, commit string) string {
		doc := renderDiffCaptureDocument(tree, commit, nil, renderComparison{
			compared: []string{"x.golden.html"},
		})
		data, err := renderDiffCaptureMarshal(doc)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(data)
	}

	record := func(t *testing.T, body string) (output string, code int) {
		t.Helper()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "surfaces.yaml"), []byte("surfaces:\n  - name: changelog\n    paths: [CHANGELOG.md]\n"), 0o644); err != nil {
			t.Fatalf("write surfaces.yaml: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, "gate"), 0o755); err != nil {
			t.Fatalf("mkdir gate: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "gate", "render-diff.json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write the capture: %v", err)
		}
		cmd := exec.Command("bash", script, "record",
			"--root", root, "--tree", tree,
			"--baseline-ref", "v0.5.0", "--baseline-commit", commit,
			"gate/render-diff.json")
		out, err := cmd.CombinedOutput()
		code = 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				t.Fatalf("run.sh record: %v\n%s", err, out)
			}
		}
		if _, statErr := os.Stat(filepath.Join(root, "gate", "run.json")); statErr == nil && code != 0 {
			t.Errorf("run.sh refused and wrote gate/run.json anyway; a truncated or half-written manifest parses far enough to look like a run that recorded less than it produced")
		}
		if code == 0 {
			manifest, readErr := os.ReadFile(filepath.Join(root, "gate", "run.json"))
			if readErr != nil {
				t.Fatalf("run.sh exited 0 and wrote no manifest: %v", readErr)
			}
			if !strings.Contains(string(manifest), "gate/render-diff.json") {
				t.Errorf("the manifest does not name the capture:\n%s", manifest)
			}
		}
		return string(out), code
	}

	t.Run("an honest capture is recorded (positive control)", func(t *testing.T) {
		out, code := record(t, honest(tree, commit))
		if code != 0 {
			t.Fatalf("run.sh refused a capture that agrees with the run, so every row below would pass over a guard that refuses everything: exit %d\n%s", code, out)
		}
	})

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			// THE TWO-BYTE CAPTURE. `printf '{}' > gate/render-diff.json` is
			// the one-line workaround for a gate that has been refusing for ten
			// minutes, and before this guard it went green end to end: the
			// manifest is honest about the bytes, the digest matches, and the
			// CHANGELOG agent finds no artifact and reports that nothing needs
			// announcing.
			"the two-byte capture", "{}\n", "not a full 40-digit object name",
		},
		{
			"a capture from the previous release's tree",
			honest(strings.Repeat("d", 40), commit),
			"covers " + tree,
		},
		{
			"a capture taken against another baseline",
			honest(tree, strings.Repeat("e", 40)),
			"and this run resolved " + commit,
		},
		{
			"a capture whose baseline is a tag rather than an identity",
			"{\n  \"tree\": \"" + tree + "\",\n  \"baseline\": {\"commit\": \"v0.5.0\"}\n}\n",
			"not a full 40-digit object name",
		},
		{
			// The capture says nothing about its own coverage AND nothing about
			// its provenance. It is the shape gate_bundle_test.go's own fixture
			// writes, so it is not a hypothetical.
			"a capture that is only an empty artifacts list",
			"{\"artifacts\":[]}\n",
			"not a full 40-digit object name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, code := record(t, tc.body)
			if code == 0 {
				t.Fatalf("run.sh recorded a capture that does not belong to this run. Every digest in that manifest is honest and the comparison under the changelog surface's key is about a different release:\n%s", out)
			}
			if code != 3 {
				t.Errorf("run.sh exited %d; the baseline-could-not-be-resolved refusal is exit 3, which its own header documents as never being reported as an empty delta:\n%s", code, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected the refusal to name %q, got:\n%s", tc.want, out)
			}
		})
	}
}

// renderDiffCaptureBaselineForTest names the release the subprocess rows above
// resolve a real commit from. It is a flag rather than a literal so the same
// rows run in a clone where the newest release is a different tag; it is NOT the
// producer's baseline, which is always supplied by the driver.
var renderDiffCaptureBaselineForTest = flag.String("render-diff-test-baseline-ref", "v0.5.0", "the release tag TestRenderDiffCapture_G1Capture_FlagContract resolves a real commit from")
