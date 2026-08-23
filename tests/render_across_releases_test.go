// render_across_releases_test.go is the detector for the one release class this
// project has actually shipped twice: ALREADY-LOCKED, BYTE-IDENTICAL CLAIMS
// RENDERING DIFFERENTLY.
//
// v0.4.0's silent change was table layout; v0.4.1's was the edges footer and the
// chips. In both cases every claim was locked, every claim file was byte for
// byte what it had been, the lock store was untouched, and `dossierx check`
// reported nothing — so a consumer's own merge gate could not detect it FOR
// them. The only place it could ever have been caught is here, by comparing what
// this project renders now against what it rendered at the last release.
//
// WHY IT CANNOT BE A PUSH-TIME CHECK. No individual push knows what the last
// release rendered; only a comparison against the release TAG does.
// tests/fixture_staleness_test.go is the push-time twin — it makes a commit that
// changes rendered output and does not regenerate the committed artifacts red —
// and it is precisely what makes this file's method sound: because the committed
// goldens and fixture viewers cannot be stale at any green commit, the artifacts
// AT THE TAG are what that release actually rendered. That is why the baseline
// needs the tag's SOURCE and not its surface.json (v0.5.0 carries none: the
// surface emitter landed after it).
//
// WHAT IT PRODUCES. Not a signal that something moved — THE DIFF. A silent
// change is only ever fixed by a CHANGELOG entry describing it, and that entry
// has to be written from the actual before/after. surface.json's
// `render_fingerprint` already answers "did rendered output move"; this answers
// "how". So the committed report carries a unified diff for EVERY artifact that
// moved — including the ones whose own inputs moved too, because a classifier
// that suppresses evidence is a classifier that can be disarmed by editing one
// unrelated byte — and a red build until the diff is recorded is the mechanism
// that stops a silent change shipping unannounced.
//
// WHERE IT RUNS. Every push and every pull request, in ci.yml's `test` job,
// which for this reason checks out with fetch-depth: 0 and resolves
// DOSSIERX_PREV_RELEASE_TAG in a step that fails when it finds no tag. It has no
// skip: a checkout with no tags cannot compare anything, and a check that cannot
// run is a failure here, never a quiet `ok`.
//
// WHAT IT IS NOT. It is not a verdict. Rendered output is supposed to change
// sometimes. The report is regenerated the same way every other golden here is,
// and the regeneration is the act of reading the diff:
//
//	go test ./tests -run TestRenderedOutputAcrossReleases -regenerate-goldens
//
// A RELEASE, NOT A COMMIT, IS WHAT RESETS IT. After a tag the previous release
// moves and the report empties out; regenerating it belongs in the release
// procedure alongside regenerating surface.json.
//
// COVERAGE, STATED PLAINLY. It compares every generated artifact committed under
// testdata/: the .golden.html of every markdown case, and every fixture
// project's viewer/index.html and .catalog.json. A rendering change to a
// construct no case covers is invisible to it — that is a gap in the corpora,
// not one this file can close, and adding a case closes it here automatically.
package tests

import (
	"flag"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// regenerateGoldens rewrites this package's committed snapshots instead of
// asserting against them. The name is the one internal/render/markdown and
// cmd/dossierx already register for their own goldens: one convention, now four
// artifacts. envelope_contract_golden_test.go reads the same flag.
var regenerateGoldens = flag.Bool("regenerate-goldens", false, "rewrite this package's committed snapshots instead of asserting the committed copies match")

const (
	// renderAcrossReleasesFile is the committed report, relative to the
	// repository root. It sits under testdata/ because it is a record of how
	// the rendered output moved, not a document a client reads.
	renderAcrossReleasesFile = "testdata/render-across-releases.golden.txt"

	// prevReleaseTagEnv names the baseline explicitly, for the same reason
	// .github/workflows/ci.yml resolves DOSSIERX_TEST_BROWSER explicitly rather
	// than letting the viewer suite probe: a resolver step that finds nothing
	// can say so in the job log, where a maintainer reads it, and name the
	// knob. The check itself no longer has a way to decline either way —
	// declared or probed, an unresolvable baseline is a failure.
	prevReleaseTagEnv = "DOSSIERX_PREV_RELEASE_TAG"

	// diffContext is the number of unchanged lines printed either side of a
	// hunk.
	diffContext = 3

	// maxEditDistance caps the diff search. A silent render change is a small
	// edit by construction — the claims did not move — so exceeding this means
	// the two artifacts are not variants of each other, and the report says so
	// in words rather than printing a hundred thousand lines nobody will read.
	maxEditDistance = 3000
)

// ---------------------------------------------------------------------
// git, and the baseline
// ---------------------------------------------------------------------

// gitOut runs git at the repository root and returns stdout. A git that fails
// is fatal, never a skip: every caller here is answering a question the gate
// cannot go green without.
func gitOut(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRootDir(t)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, errb.String())
	}
	return out.String()
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// resolveBaseline returns the previous release tag and the commit it points at.
//
// The two ways in are DOSSIERX_PREV_RELEASE_TAG, which names the baseline
// outright, and `git describe`, which finds the newest tag reachable from HEAD.
// Neither of them can decline to answer. An unresolvable baseline is a FAILURE,
// in the same words CLAUDE.md uses for a missing browser and an uninstalled
// tool: there is no result here that means "we did not check" and reads as "it
// is fine".
//
// This was a t.Skip until it was measured. A clone with no tags — which is
// exactly what actions/checkout produces at its default depth — took the skip,
// `go test` printed `ok`, and the detector for the release class this project
// actually ships ran in no automated context at all. The environment that
// cannot answer is not the environment that should be reassured; it is the one
// that has to be fixed, so the message names the fix and the build stays red
// until it lands. .github/workflows/ci.yml checks out with fetch-depth: 0 and
// resolves this variable explicitly, the same shape as the viewer job's
// DOSSIERX_TEST_BROWSER.
func resolveBaseline(t *testing.T) (tag, commit string) {
	t.Helper()
	declared := os.Getenv(prevReleaseTagEnv)
	if declared == "" {
		probe := exec.Command("git", "describe", "--tags", "--abbrev=0")
		probe.Dir = repoRootDir(t)
		out, err := probe.Output()
		if err != nil || strings.TrimSpace(string(out)) == "" {
			t.Fatalf("no release tag is reachable from HEAD, so the previous release's rendered output cannot\n"+
				"be read and NOTHING WAS COMPARED. This is a failure, not a skip: a checkout with no tags\n"+
				"(which is what actions/checkout does at its default depth) cannot run the one check that\n"+
				"detects an already-locked, byte-identical claim rendering differently.\n\n"+
				"Fetch the tags, or name the baseline outright:\n"+
				"    git fetch --tags --force\n"+
				"    %s=<previous release tag> go test ./tests -run TestRenderedOutputAcrossReleases\n\n"+
				"In CI, check out with `fetch-depth: 0` and resolve %s in a step that fails when it finds\n"+
				"no tag — the shape .github/workflows/ci.yml already uses for DOSSIERX_TEST_BROWSER.",
				prevReleaseTagEnv, prevReleaseTagEnv)
		}
		declared = strings.TrimSpace(string(out))
	}
	// ^{commit} so an annotated tag resolves to the commit and not to the tag
	// object, whose sha is not the tree anybody released.
	commit = strings.TrimSpace(gitOut(t, "rev-parse", declared+"^{commit}"))
	if commit == "" {
		t.Fatalf("%s names %q, which resolves to no commit", prevReleaseTagEnv, declared)
	}
	return declared, commit
}

// ---------------------------------------------------------------------
// what counts as rendered output, and what its inputs are
// ---------------------------------------------------------------------

// isRenderedArtifact reports whether a repository path is GENERATED rendered
// output rather than an input to it. The three shapes are the three golden
// corpora: the markdown cases' .golden.html (including the .claim/.comment
// variants under markdown-claim-body-cases), and each fixture project's
// viewer/index.html and .catalog.json.
func isRenderedArtifact(rel string) bool {
	base := path.Base(rel)
	switch {
	case strings.HasSuffix(base, ".golden.html"):
		return true
	case base == ".catalog.json":
		return true
	case base == "index.html" && path.Base(path.Dir(rel)) == "viewer":
		return true
	}
	return false
}

// isTimestamped reports whether an artifact legitimately differs run to run.
// Only the viewer carries generation stamps; fixture_staleness_test.go makes the
// same split and compares .catalog.json raw, "normalizing it would only be able
// to hide something".
func isTimestamped(rel string) bool {
	return path.Base(rel) == "index.html" && path.Base(path.Dir(rel)) == "viewer"
}

// fixtureDirOf returns the testdata/fixture-* directory a path sits under, or ""
// if it is not in a fixture project.
func fixtureDirOf(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) < 2 || parts[0] != "testdata" || !strings.HasPrefix(parts[1], "fixture-") {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// inputsOf returns every path whose content decides what artifact renders as.
//
// It is fatal, never lenient, when it cannot answer. An artifact whose inputs
// could not be found would be compared against an EMPTY input set, every input
// would trivially be "unchanged", and a change driven by an edited fixture would
// be reported as a silent render change — the report would be confidently wrong
// in the direction that costs a maintainer a day. A corpus shaped differently
// from the three that exist therefore stops the build until this function is
// taught about it.
func inputsOf(t *testing.T, artifact string, tracked map[string]bool) []string {
	t.Helper()

	if dir := fixtureDirOf(artifact); dir != "" {
		// A fixture project renders from everything in it: the claims, the
		// config, the lock store, the comment digest. Anything that is not
		// itself a generated artifact is an input.
		var inputs []string
		for p := range tracked {
			if strings.HasPrefix(p, dir+"/") && !isRenderedArtifact(p) {
				inputs = append(inputs, p)
			}
		}
		if len(inputs) == 0 {
			t.Fatalf("%s has no input files under %s; either the fixture moved or inputsOf is broken — both are failures, not a pass", artifact, dir)
		}
		sort.Strings(inputs)
		return inputs
	}

	// A markdown case: "<stem>.golden.html", or the two-variant
	// "<stem>.claim.golden.html" / "<stem>.comment.golden.html", both rendered
	// from the sibling "<stem>.yaml".
	stem := strings.TrimSuffix(artifact, ".golden.html")
	if stem == artifact {
		t.Fatalf("%s is classified as rendered output but matches no corpus shape inputsOf knows; teach it before the report can be trusted", artifact)
	}
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, ".claim"), ".comment")
	yaml := stem + ".yaml"
	if !tracked[yaml] {
		t.Fatalf("%s has no input %s at either revision; an artifact with no findable input would be reported as an unexplained render change", artifact, yaml)
	}
	return []string{yaml}
}

// ---------------------------------------------------------------------
// the comparison
// ---------------------------------------------------------------------

type renderMove struct {
	path          string
	changedInputs []string
	baseLines     int
	baseBytes     int
	headLines     int
	headBytes     int
	diff          string // populated for silent moves only; see the report header
	added         int
	removed       int
	tooLarge      bool
}

// renderComparison is ONE cross-release comparison, held apart from the two
// documents that render it.
//
// "What rendered differently since the last release" has exactly one
// implementation, and testdata/render-across-releases.golden.txt is its
// committed answer: it goes red when it drifts and it is read by a human before
// a release. A second implementation of the comparison would be the second
// release procedure CLAUDE.md forbids, and the two would diverge in silence
// because only one of them has a golden that goes red. So the comparison is a
// function, and this struct is its answer.
type renderComparison struct {
	// compared is every rendered artifact present at BOTH revisions. It is the
	// coverage witness: an empty artifacts list means something completely
	// different when compared is 147 than when it is 0.
	compared []string
	added    []string
	removed  []string
	// silent is the section that obliges a CHANGELOG line: rendered output
	// moved and not one byte of its inputs did.
	silent []renderMove
	// explained moved after its own inputs moved. It carries the SAME diff —
	// see the long comment in compareRenderedOutput for why suppressing it here
	// was reproduced as a real hole, not theorised.
	explained []renderMove
}

// compareRenderedOutput compares every rendered artifact under testdata/ at
// baselineCommit against the WORKING TREE, and classifies each move.
//
// baselineLabel is the human-readable name of what baselineCommit resolved from
// — a tag for the committed report, whatever the driver supplied for the gate
// capture. It appears only in messages; every question asked of git uses the
// commit.
//
// THE BASELINE IS A PARAMETER, NEVER RE-PROBED HERE. resolveBaseline is the
// human report's business: a maintainer running this by hand is entitled to have
// the tag found for them. A gate run is not, because the driver has already
// resolved one and passed an identity down, and a second resolver inside the
// same run is a second answer to "which release is this being compared against"
// that can disagree without either side being wrong on its own terms. Worse,
// once the release's own tag exists `git describe --tags --abbrev=0` names IT,
// and the comparison becomes the release against itself, reporting zero silent
// changes with perfect confidence.
// Deliberately NOT a t.Helper: the four refusals below are the load-bearing part
// of this function, and attributing them to the caller's line would point a
// reader at the report or at the capture instead of at the refusal that fired.
func compareRenderedOutput(t *testing.T, baselineLabel, commit string) renderComparison {
	tag := baselineLabel
	root := repoRootDir(t)

	// The head side is the WORKING TREE, not HEAD: the gate renders a verdict
	// against the tree it was pointed at, and a tree with an uncommitted
	// rendering change in it is exactly the tree a maintainer is about to tag.
	headPaths := gitLines(gitOut(t, "ls-files", "-z", "--", "testdata"))
	basePaths := gitLines(gitOut(t, "ls-tree", "-r", "-z", "--name-only", commit, "--", "testdata"))

	// An untracked generated artifact would be invisible to every git question
	// asked below, so it fails rather than shrinking the comparison in silence.
	for _, p := range gitLines(gitOut(t, "ls-files", "-z", "--others", "--exclude-standard", "--", "testdata")) {
		if isRenderedArtifact(p) {
			t.Fatalf("%s is rendered output and is not tracked, so it cannot be compared against the baseline; `git add` it", p)
		}
	}

	tracked := map[string]bool{}
	for _, p := range headPaths {
		tracked[p] = true
	}
	for _, p := range basePaths {
		tracked[p] = true
	}

	// One question to git for the whole tree: what differs between the baseline
	// commit and the working tree. Everything below is classification over this
	// set, which is why the comparison costs three git invocations and not one
	// per file.
	changed := map[string]bool{}
	for _, p := range gitLines(gitOut(t, "diff", "--name-only", "-z", commit, "--", "testdata")) {
		changed[p] = true
	}

	headArtifacts, baseArtifacts := map[string]bool{}, map[string]bool{}
	for _, p := range headPaths {
		if isRenderedArtifact(p) {
			headArtifacts[p] = true
		}
	}
	for _, p := range basePaths {
		if isRenderedArtifact(p) {
			baseArtifacts[p] = true
		}
	}
	if len(baseArtifacts) == 0 {
		t.Fatalf("the baseline %s (%s) carries no rendered artifact under testdata/; either the corpora moved or the classifier is broken — both are failures, not a pass", tag, commit)
	}

	var added, removed, compared []string
	for p := range headArtifacts {
		if baseArtifacts[p] {
			compared = append(compared, p)
		} else {
			added = append(added, p)
		}
	}
	for p := range baseArtifacts {
		if !headArtifacts[p] {
			removed = append(removed, p)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(compared)

	var silent, explained []renderMove
	for _, rel := range compared {
		if !changed[rel] {
			continue
		}
		baseText := gitOut(t, "show", commit+":"+rel)
		headBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s from the working tree: %v", rel, err)
		}
		headText := string(headBytes)

		cmpBase, cmpHead := baseText, headText
		if isTimestamped(rel) {
			cmpBase = normalizeGeneratedTimestamps(cmpBase)
			cmpHead = normalizeGeneratedTimestamps(cmpHead)
		}
		if cmpBase == cmpHead {
			// The only difference is a generation stamp. Reporting that would
			// bury the changes that matter under three lines per viewer.
			continue
		}

		move := renderMove{
			path:      rel,
			baseLines: countLines(baseText),
			baseBytes: len(baseText),
			headLines: countLines(headText),
			headBytes: len(headText),
		}
		for _, in := range inputsOf(t, rel, tracked) {
			if changed[in] {
				move.changedInputs = append(move.changedInputs, in)
			}
		}
		// THE DIFF IS PRODUCED FOR BOTH SECTIONS, and that is not symmetry for
		// its own sake — it is what stops the classification from being a
		// suppressor.
		//
		// A fixture project's inputs are "every tracked non-artifact file under
		// it": inputsOf cannot be finer, because nothing here knows which claim
		// fed which line of a ten-thousand-line viewer. So ONE edited byte
		// anywhere in a fixture — a claim's wording, its lock store, the
		// comment digest `dossierx check` rewrites on its own — reclassifies
		// that fixture's whole viewer as explained. When the explained branch
		// recorded only line and byte counts, that reclassification DELETED the
		// evidence: a chip class renamed from `comment-chip-count` to
		// `comment-chip-tally` is byte-length-preserving, so the report stayed
		// byte-identical and the test stayed green over a v0.4.1-class silent
		// change. Reproduced, not theorised.
		//
		// With the diff always present, the classification is a LABEL on
		// evidence that is there either way — it says how much of the diff you
		// should expect to be able to explain, and it never decides how much of
		// it you get to see.
		move.diff, move.added, move.removed, move.tooLarge = unifiedDiff(splitLines(cmpBase), splitLines(cmpHead))
		if len(move.changedInputs) > 0 {
			explained = append(explained, move)
			continue
		}
		silent = append(silent, move)
	}

	return renderComparison{compared: compared, added: added, removed: removed, silent: silent, explained: explained}
}

func TestRenderedOutputAcrossReleases(t *testing.T) {
	tag, commit := resolveBaseline(t)
	root := repoRootDir(t)

	cmp := compareRenderedOutput(t, tag, commit)

	report := renderAcrossReleasesReport(tag, commit, len(cmp.compared), cmp.added, cmp.removed, cmp.silent, cmp.explained)
	goldenPath := filepath.Join(root, filepath.FromSlash(renderAcrossReleasesFile))

	if *regenerateGoldens {
		if err := os.WriteFile(goldenPath, []byte(report), 0o644); err != nil {
			t.Fatalf("write %s: %v", renderAcrossReleasesFile, err)
		}
		t.Logf("regenerated %s against %s (%s): %d silent, %d input-driven", renderAcrossReleasesFile, tag, commit[:7], len(cmp.silent), len(cmp.explained))
		return
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read %s: %v\n\nRegenerate it with:\n  go test ./tests -run TestRenderedOutputAcrossReleases -regenerate-goldens", renderAcrossReleasesFile, err)
	}
	if string(wantBytes) == report {
		return
	}

	// A MOVED BASELINE IS NOT A RENDER CHANGE, and saying it is sends a
	// maintainer hunting a diff that does not exist. The first commit after a
	// tag reds this report for the ordinary reason that the previous release is
	// now a different release: everything below the baseline line usually
	// empties out at the same moment. That reads as a silent render change under
	// the general message, so it gets its own.
	if was, now := committedBaseline(string(wantBytes)), tag+" ("+commit+")"; was != "" && was != now {
		t.Fatalf(`%s was written against a different release than the one it is now being compared with.

  written against  %s
  comparing with   %s

Nothing is wrong with the rendered output — the PREVIOUS RELEASE moved, which is
what a tag does. Regenerating the report is a release step, in the same breath as
regenerating surface.json:

  go test ./tests -run TestRenderedOutputAcrossReleases -regenerate-goldens

If %s is set, check it names the release you meant.`,
			renderAcrossReleasesFile, was, now, prevReleaseTagEnv)
	}

	line, wantLine, gotLine := firstDifferingLine(string(wantBytes), report)
	t.Fatalf(`%s is out of date: rendered output has moved relative to %s in a way the committed report does not describe.

first difference at line %d
  committed: %s
  observed:  %s

This is the release class a consumer's own gate cannot detect for them: locked,
byte-identical claims that render differently. Regenerate the report, READ the
diff it now carries, and write the CHANGELOG entry from it:

  go test ./tests -run TestRenderedOutputAcrossReleases -regenerate-goldens`,
		renderAcrossReleasesFile, tag, line, truncateForDiff(wantLine), truncateForDiff(gotLine))
}

// committedBaseline reads the "baseline   <tag> (<sha>)" line out of a report
// generated earlier, so a report written against another release can be told
// apart from one describing a real change. The header's prose lines all start
// with "#", so only the record itself matches.
func committedBaseline(report string) string {
	for _, line := range strings.Split(report, "\n") {
		if rest, ok := strings.CutPrefix(line, "baseline "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// gitLines splits NUL-separated git output, dropping the trailing empty entry.
func gitLines(out string) []string {
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func countLines(s string) int { return len(splitLines(s)) }

// ---------------------------------------------------------------------
// the report
// ---------------------------------------------------------------------

const renderAcrossReleasesHeader = `# render-across-releases.golden.txt — how this project's rendered output has
# moved since the previous release.
#
# Generated by tests/render_across_releases_test.go. Regenerate with:
#
#     go test ./tests -run TestRenderedOutputAcrossReleases -regenerate-goldens
#
# WHAT IT IS FOR. v0.4.0 changed table layout and v0.4.1 changed the edges footer
# and the chips, with every claim locked and byte-identical, the lock store
# untouched and ` + "`dossierx check`" + ` reporting nothing. A consumer's own merge gate
# cannot detect that for them; only a comparison against the release tag can.
#
# SILENT RENDER CHANGES is the section that matters: an artifact whose inputs did
# not move by a byte and whose rendered output did. Every entry there needs a
# CHANGELOG line, and the diff printed under it is what that line is written from.
#
# EXPLAINED BY AN INPUT CHANGE carries THE SAME DIFF, and the inputs that moved.
# The two sections differ in what they oblige you to do, never in what they show
# you. A fixture project's inputs are every tracked non-artifact file under it —
# nothing here knows which claim fed which line of a ten-thousand-line viewer —
# so one edited byte anywhere in a fixture moves that whole fixture's viewer into
# this section. If the section could suppress the diff, that one byte would also
# hide a renamed CSS class in the same viewer, which is precisely the v0.4.1
# change. So: read this section's diffs, and satisfy yourself that every hunk in
# them is accounted for by the named inputs. A hunk that is not is a silent
# render change wearing an explanation.
#
# THE BASELINE LINE IS PART OF THE SNAPSHOT. It records which release this was
# compared against, so the comparison is auditable rather than implied. The
# baseline defaults to the newest tag reachable from HEAD; DOSSIERX_PREV_RELEASE_TAG
# overrides it, and an override that names a different release must be
# regenerated, because a report whose header says one release and whose diffs
# describe another would be worse than no report. Regenerating after a tag is a
# release step, in the same breath as regenerating surface.json.
#
# An empty report is the expected steady state immediately after a release.
`

func renderAcrossReleasesReport(tag, commit string, compared int, added, removed []string, silent, explained []renderMove) string {
	var b strings.Builder
	b.WriteString(renderAcrossReleasesHeader)
	b.WriteString("\n")
	b.WriteString("baseline   " + tag + " (" + commit + ")\n")
	b.WriteString("compared   " + itoa(compared) + " rendered artifacts present at both revisions\n")
	b.WriteString("added      " + itoa(len(added)) + "\n")
	b.WriteString("removed    " + itoa(len(removed)) + "\n")
	b.WriteString("silent     " + itoa(len(silent)) + " rendered differently from byte-identical inputs\n")
	b.WriteString("explained  " + itoa(len(explained)) + " rendered differently after their own inputs changed\n")

	if len(added)+len(removed)+len(silent)+len(explained) == 0 {
		b.WriteString("\nno rendered artifact has moved since the baseline.\n")
		return b.String()
	}

	section := func(title string, body func()) {
		b.WriteString("\n" + title + "\n" + strings.Repeat("-", len(title)) + "\n")
		body()
	}

	// One entry writer for both sections. They differ by which inputs moved and
	// by what that obliges a reader to do, never by how much of the change they
	// are shown.
	entry := func(m renderMove) {
		if m.tooLarge {
			b.WriteString("\n" + m.path + "  (rewritten)\n")
		} else {
			b.WriteString("\n" + m.path + "  (+" + itoa(m.added) + " -" + itoa(m.removed) + ")\n")
		}
		for i, in := range m.changedInputs {
			label := "    inputs changed  "
			if i > 0 {
				label = "                    "
			}
			b.WriteString(label + in + "\n")
		}
		b.WriteString("    size            " + itoa(m.baseLines) + " lines / " + itoa(m.baseBytes) + " bytes -> " +
			itoa(m.headLines) + " lines / " + itoa(m.headBytes) + " bytes\n")
		if m.tooLarge {
			b.WriteString("    more than " + itoa(maxEditDistance) + " lines differ, so these two are not variants of one\n" +
				"    document and a line diff would not be readable. NOTHING HERE IS NARROWED: the\n" +
				"    artifact IS reported, with its sizes, and it still rendered differently. Read it\n" +
				"    against the baseline directly:\n" +
				"        git diff " + tag + " -- " + m.path + "\n")
			return
		}
		b.WriteString(m.diff)
	}

	if len(silent) > 0 {
		section("SILENT RENDER CHANGES", func() {
			for _, m := range silent {
				entry(m)
			}
		})
	}

	if len(explained) > 0 {
		section("EXPLAINED BY AN INPUT CHANGE", func() {
			for _, m := range explained {
				entry(m)
			}
		})
	}

	if len(added) > 0 {
		section("ADDED SINCE THE BASELINE", func() {
			for _, p := range added {
				b.WriteString(p + "\n")
			}
		})
	}
	if len(removed) > 0 {
		section("REMOVED SINCE THE BASELINE", func() {
			for _, p := range removed {
				b.WriteString(p + "\n")
			}
		})
	}
	return b.String()
}

// ---------------------------------------------------------------------
// the diff itself
// ---------------------------------------------------------------------

// unifiedDiff produces a unified diff of two line slices, with the counts of
// added and removed lines. tooLarge is true when the edit distance exceeded
// maxEditDistance, in which case no diff is produced and the caller says so.
//
// The edit script is Myers' O(ND) algorithm rather than a full LCS table: the
// fixture viewers are ten thousand lines each, an N*M table over two of them is
// a hundred million cells, and the edits this file exists to print are small by
// construction — a footer, a chip, a table cell. O(ND) is linear in the size of
// the change, which is the shape of the problem.
//
// The shared head and tail are trimmed before the search, keeping diffContext
// lines of each so the first and last hunks still carry context. That is not an
// optimisation for its own sake: a silent render change is typically one edit
// repeated inside an otherwise identical document, and trimming is what keeps D
// — and therefore the trace's memory — proportional to the change rather than to
// the file.
func unifiedDiff(a, b []string) (diff string, added, removed int, tooLarge bool) {
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	head = maxInt(0, head-diffContext)
	tail = maxInt(0, tail-diffContext)

	ta, tb := a[head:len(a)-tail], b[head:len(b)-tail]
	trace, ok := myersTrace(ta, tb)
	if !ok {
		return "", 0, 0, true
	}
	ops := myersBacktrack(trace, ta, tb)
	for i := range ops {
		// Line numbers are relative to the trimmed slices; the reader needs
		// them relative to the documents.
		if ops[i].aLine != 0 {
			ops[i].aLine += head
		}
		if ops[i].bLine != 0 {
			ops[i].bLine += head
		}
		switch ops[i].kind {
		case opAdd:
			added++
		case opDel:
			removed++
		}
	}
	return renderHunks(ops), added, removed, false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type opKind int

const (
	opEqual opKind = iota
	opDel
	opAdd
)

type diffOp struct {
	kind opKind
	text string
	// aLine and bLine are 1-based line numbers in the two documents; the
	// unused one is zero.
	aLine, bLine int
}

// myersTrace walks the edit graph, keeping one snapshot of the furthest-reaching
// frontier per edit-distance d. Each snapshot holds only the diagonals reachable
// at that d (2d+1 of them) rather than the whole array, which is what keeps the
// trace quadratic in the SIZE OF THE CHANGE instead of in the size of the files.
func myersTrace(a, b []string) (trace [][]int, ok bool) {
	n, m := len(a), len(b)
	maxD := n + m
	if maxD > maxEditDistance {
		maxD = maxEditDistance
	}
	// v is indexed by diagonal k, offset so that k=-(n+m) lands at 0.
	offset := n + m
	v := make([]int, 2*(n+m)+1)
	for d := 0; d <= maxD; d++ {
		snapshot := make([]int, 2*d+1)
		copy(snapshot, v[offset-d:offset+d+1])
		trace = append(trace, snapshot)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[offset+k] = x
			if x >= n && y >= m {
				return trace, true
			}
		}
	}
	return nil, false
}

// myersBacktrack walks the trace backwards into an edit script in document
// order.
func myersBacktrack(trace [][]int, a, b []string) []diffOp {
	var rev []diffOp
	x, y := len(a), len(b)
	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d] // indexed by k+d; holds the frontier reached at d-1
		k := x - y
		var prevK int
		if k == -d || (k != d && v[k-1+d] < v[k+1+d]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[prevK+d]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			rev = append(rev, diffOp{opEqual, a[x-1], x, y})
			x--
			y--
		}
		if x > prevX {
			rev = append(rev, diffOp{opDel, a[x-1], x, 0})
			x--
		} else {
			rev = append(rev, diffOp{opAdd, b[y-1], 0, y})
			y--
		}
	}
	for x > 0 {
		rev = append(rev, diffOp{opEqual, a[x-1], x, y})
		x--
		y--
	}

	ops := make([]diffOp, 0, len(rev))
	for i := len(rev) - 1; i >= 0; i-- {
		ops = append(ops, rev[i])
	}
	return ops
}

// renderHunks turns an edit script into unified-diff hunks with diffContext
// lines of context, in the "@@ -a,b +c,d @@" shape every reader already knows.
func renderHunks(ops []diffOp) string {
	// Mark which ops are within diffContext of a change; runs of marked ops are
	// the hunks.
	keep := make([]bool, len(ops))
	for i, op := range ops {
		if op.kind == opEqual {
			continue
		}
		lo, hi := i-diffContext, i+diffContext
		if lo < 0 {
			lo = 0
		}
		if hi >= len(ops) {
			hi = len(ops) - 1
		}
		for j := lo; j <= hi; j++ {
			keep[j] = true
		}
	}

	var b strings.Builder
	for i := 0; i < len(ops); {
		if !keep[i] {
			i++
			continue
		}
		j := i
		for j < len(ops) && keep[j] {
			j++
		}
		writeHunk(&b, ops[i:j])
		i = j
	}
	return b.String()
}

func writeHunk(b *strings.Builder, ops []diffOp) {
	aStart, bStart, aCount, bCount := 0, 0, 0, 0
	for _, op := range ops {
		if op.aLine != 0 {
			if aStart == 0 {
				aStart = op.aLine
			}
			aCount++
		}
		if op.bLine != 0 {
			if bStart == 0 {
				bStart = op.bLine
			}
			bCount++
		}
	}
	b.WriteString("@@ -" + itoa(aStart) + "," + itoa(aCount) + " +" + itoa(bStart) + "," + itoa(bCount) + " @@\n")
	for _, op := range ops {
		switch op.kind {
		case opEqual:
			b.WriteString(" " + op.text + "\n")
		case opDel:
			b.WriteString("-" + op.text + "\n")
		case opAdd:
			b.WriteString("+" + op.text + "\n")
		}
	}
}
