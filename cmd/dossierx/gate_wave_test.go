// gate_wave_test.go pins THE SNAG CHECK: two agents reading a fix wave's own
// diff before a full round pays thirteen agents to discover what that wave broke.
//
// WHY IT EXISTS. Every reading round of the v0.5.2 gate after the first opened by
// repairing the round before it — round two, "four were regressions introduced by
// round one's fixes"; round three, a section titled "MINE, FROM ROUND TWO"; round
// four, "three of these are high severity and all three are mine". The wave is
// written by an agent, and nothing read it until the next full round did.
//
// WHAT IS PINNED HERE. One property, and everything else in this file is a way of
// stating it:
//
//	A WAVE ANSWER IS NEVER A SURFACE ANSWER.
//
// The mode writes under gate/wave/, mints no run, and files nothing. Its reading
// is keyed to a RANGE; a surface answer is keyed to a tree, a run identifier and
// a surface fingerprint, none of which a wave has. That is what keeps a narrow
// read from standing where a full bundle read is required — the skipped check
// that reads as a pass, which is the failure this whole gate is written against.
//
// The second property is about the bundle: a reader is handed the diff AND the
// full text of every changed file, because a sentence is false or true against the
// paragraph around it, and a hunk hides that paragraph.
//
// Same shape as the rest of the gate: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const gateWaveBundleFile = "gate/wave/bundle.md"

// gateWaveRepo builds a real git repository with two commits: a base, and a wave
// that edits one file, adds another and deletes a third.
//
// A real repository rather than a fixture directory, because the mode's whole
// input is `git diff`, and a test that stubbed that would be pinning the shell's
// string handling instead of the reading it produces.
func gateWaveRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=gate", "GIT_AUTHOR_EMAIL=gate@example.invalid",
			"GIT_COMMITTER_NAME=gate", "GIT_COMMITTER_EMAIL=gate@example.invalid",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	// The declarations the harness reads before any mode runs. The wave prompt is
	// COPIED FROM THE REAL TREE rather than invented here: the question an agent is
	// asked is reviewed material, and a test that asked a different one would be
	// proving something about a string that ships nowhere.
	gateWrite(t, root, gateManifestFile, "surfaces:\n  - name: readme\n    paths:\n      - README.md\n")
	gateWrite(t, root, gateStage2MethodFile, "model: claude-opus-5\ntools:\n  - SurfaceFinding\n  - SurfaceVerdict\n")
	realPrompt, err := os.ReadFile(filepath.Join(surfaceRepoRoot(t), filepath.FromSlash("gate/prompts/_wave.md")))
	if err != nil {
		t.Fatalf("read the real wave prompt: %v", err)
	}
	gateWrite(t, root, "gate/prompts/_wave.md", string(realPrompt))

	gateWrite(t, root, "README.md", "# a project\n\nIt has nineteen commands.\n")
	gateWrite(t, root, "OLD.md", "this file is about to be deleted\n")
	run("init", "-q", "-b", "main")
	run("add", "-A")
	run("commit", "-q", "-m", "base")

	// The wave: an edit that changes a claim, a new file, and a deletion.
	gateWrite(t, root, "README.md", "# a project\n\nIt has twenty commands.\n")
	gateWrite(t, root, "NOTES.md", "a sentence the wave introduced\n")
	if err := os.Remove(filepath.Join(root, "OLD.md")); err != nil {
		t.Fatalf("remove OLD.md: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "the fix wave")

	return root
}

func TestGateWaveFilesNoAnswerAndMintsNothing(t *testing.T) {
	root := gateWaveRepo(t)

	out, err := gateStage2Harness(t, "wave", "--root", root, "--range", "HEAD~1..HEAD")
	if err != nil {
		t.Fatalf("the wave mode refused a real range: %v", err)
	}

	// The property this file exists for: nothing a later stage reads was written.
	for _, forbidden := range []string{"gate/answers", "gate/fanout.json", "gate/run.json"} {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(forbidden))); !os.IsNotExist(statErr) {
			t.Errorf("the wave mode created %s. A wave reading is keyed to a range, not to a tree — anything it leaves where stage 3 looks is a narrow read standing in for a full one", forbidden)
		}
	}

	lines := gateStage2Lines(out)
	if len(lines) != 2 {
		t.Fatalf("the wave printed %d invocation(s), want 2:\n%s", len(lines), out)
	}
	for i, line := range lines {
		if !strings.Contains(line, gateWaveBundleFile) {
			t.Errorf("invocation %d does not name the wave bundle: %s", i+1, line)
		}
		// The same exclusive allow-list every reading agent runs under. A wave
		// reader with a file tool could reach past its diff, and then its silence
		// would be a claim about material nobody chose to hand it.
		if !strings.Contains(line, "--allowed-tools SurfaceFinding,SurfaceVerdict") {
			t.Errorf("invocation %d does not request the exact declared grant as an exclusive allow-list: %s", i+1, line)
		}
	}
}

func TestGateWaveHandsOverWholeFilesAndNotOnlyHunks(t *testing.T) {
	root := gateWaveRepo(t)

	if _, err := gateStage2Harness(t, "wave", "--root", root, "--range", "HEAD~1..HEAD"); err != nil {
		t.Fatalf("wave: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateWaveBundleFile)))
	if err != nil {
		t.Fatalf("read the wave bundle: %v", err)
	}
	bundle := string(body)

	if strings.Contains(bundle, "<<RANGE>>") {
		t.Error("the bundle still carries the <<RANGE>> placeholder, so the reader is not told which change they are judging")
	}
	if !strings.Contains(bundle, "HEAD~1..HEAD") {
		t.Error("the bundle does not name the range it was assembled over")
	}

	// The diff.
	if !strings.Contains(bundle, "+It has twenty commands.") {
		t.Error("the bundle carries no diff of the changed claim, which is the change the reader is being asked about")
	}

	// The whole file, which is what lets a new sentence be judged against the
	// paragraph around it. The heading line is in the file and not in the hunk.
	if !strings.Contains(bundle, "# a project") {
		t.Error("the bundle carries the hunk but not the file it sits in; a sentence is false or true against the paragraph around it, and a hunk hides that paragraph")
	}
	if !strings.Contains(bundle, "a sentence the wave introduced") {
		t.Error("the bundle omits a file the wave ADDED, so the reader cannot judge new text at all")
	}

	// A deletion has no "after", and an empty block would read as a file that is
	// now empty rather than one that is gone.
	if !strings.Contains(bundle, "Deleted by this wave") {
		t.Error("the bundle does not say that OLD.md was deleted; an omission and a deletion are the same silence to a reader")
	}
}

func TestGateWaveRefusesEveryReadingItCannotStandBehind(t *testing.T) {
	root := gateWaveRepo(t)

	if code, stderr := gateSubjectRefusal(t, "wave", "--root", root); code != 1 || !strings.Contains(stderr, "--range is required") {
		t.Errorf("a wave with no range exited %d saying %q; it must refuse, because a wave with no range is not a smaller reading — there is nothing to read", code, strings.TrimSpace(stderr))
	}

	// A range that resolves to nothing. It is the shape a mistyped range takes,
	// and it must not read as "the wave was clean".
	if code, stderr := gateSubjectRefusal(t, "wave", "--root", root, "--range", "HEAD..HEAD"); code != 1 || !strings.Contains(stderr, "names no changed files") {
		t.Errorf("an empty range exited %d saying %q; a range that changed nothing must be refused rather than reported as a clean read", code, strings.TrimSpace(stderr))
	}

	if err := os.Remove(filepath.Join(root, filepath.FromSlash("gate/prompts/_wave.md"))); err != nil {
		t.Fatalf("remove the wave prompt: %v", err)
	}
	if code, stderr := gateSubjectRefusal(t, "wave", "--root", root, "--range", "HEAD~1..HEAD"); code != 2 || !strings.Contains(stderr, "no question to ask") {
		t.Errorf("a missing prompt exited %d saying %q; material assembled with no question is material nobody was asked about", code, strings.TrimSpace(stderr))
	}
}

// TestGateWavePromptCannotStandInForASurfaceReading pins the sentences in the
// real prompt that keep a clean wave read from being quoted as a passing surface.
//
// The prompt is the only place the boundary is stated to the entity that could
// cross it. A future edit that tightens the prose is welcome; one that drops the
// boundary is the edit this test exists to stop.
func TestGateWavePromptCannotStandInForASurfaceReading(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(surfaceRepoRoot(t), filepath.FromSlash("gate/prompts/_wave.md")))
	if err != nil {
		t.Fatalf("read the wave prompt: %v", err)
	}
	prompt := string(body)

	for _, want := range []string{
		"no regression found in this diff",
		"It does not\nmean any surface passes",
		"decides nothing about the release",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("gate/prompts/_wave.md no longer tells its reader %q.\n"+
				"Without that sentence a clean wave read is quotable as a surface verdict, which is precisely the narrow-read-standing-for-a-full-one failure this mode is built to avoid.", want)
		}
	}

	// The null answer has to be cheap and expected, or a reader under pressure to
	// look thorough manufactures a finding — and a manufactured finding costs a
	// fix wave, which costs another wave reading.
	if !strings.Contains(prompt, "Do not manufacture a finding") {
		t.Error("gate/prompts/_wave.md no longer says that finding nothing is a real answer, so its readers are pushed toward inventing one")
	}
}
