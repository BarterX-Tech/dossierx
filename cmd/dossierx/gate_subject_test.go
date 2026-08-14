// gate_subject_test.go pins THE SUBJECT FREEZE: the rule that a release may not
// grow the question its own rounds are counted over.
//
// WHY IT EXISTS. The v0.5.2 gate ran four reading rounds — 39, 31, 24, 18
// findings — and surfaces.yaml gained paths during rounds 1, 2 and 4. Round
// four's fix wave records that the widening "is what let round four adjudicate
// the retired-set question at all". The coverage was real and the timing was
// wrong: a curve measured over a subject that grows underneath it cannot
// converge, and worse, cannot be read at all. A round returning fewer findings
// might mean a better tree or a narrower question, and nothing on the record
// said which.
//
// WHAT IS PINNED HERE, and each of these is a way the freeze could be worth
// nothing:
//
//	IT REFUSES A MOVED MANIFEST — the whole point, and it names the frozen digest
//	    AND the one on disk, so a human sees what moved and what it moved to
//	    rather than being told that something did.
//	A THAW RECORDED IN surfaces_sha256 MUST CARRY A REASON. That is one of three
//	    ways to re-open a subject, and the only one this file can refuse. The
//	    other two are DELETING gate/subject.json and CHANGING its `version`, both
//	    of which mint a fresh freeze with no reason and are pinned OPEN below —
//	    the version path deliberately, because that is how the next release
//	    starts. What stands behind those two is not this test: it is that the
//	    file is tracked, so either edit is a line in a reviewed diff. Saying the
//	    reason requirement covers every re-opening would be the overclaim this
//	    gate exists to catch.
//	A NEW RELEASE IS NOT BOUND BY ITS PREDECESSOR'S FREEZE — inheriting the
//	    previous release's subject would be the same stale-evidence failure the
//	    rest of the gate refuses everywhere else.
//	A LATER ROUND DOES NOT REWRITE THE RECORD — a freeze a run can regenerate is
//	    not a freeze, and rewriting would erase a thaw's recorded reason.
//	THE FAN-OUT VERIFIES BEFORE IT MINTS — otherwise the refusal arrives after a
//	    run identifier exists, which is indistinguishable on disk from a fan-out
//	    that half happened.
//
// WHAT THE FREEZE IS NOT. It is not narrowing, and the distinction is
// load-bearing against CLAUDE.md's rule that coverage is never narrowed
// silently: coverage stays exactly where round one set it, nothing is sampled or
// dropped, and a gap found later is recorded as a finding against the next
// release rather than discarded. What is refused is GROWING the manifest between
// rounds of one release.
//
// Same shape as the rest of the gate: test code, not a cobra command, not
// compiled into the shipped binary, outside surface.json's behaviour_fingerprint.
package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	gateSubjectFile = "gate/subject.json"
	// The exit code the harness reserves for a subject that moved. It is its own
	// code rather than a general refusal because this failure has a recovery
	// nothing else shares — revert the manifest, or record a thaw.
	gateSubjectExitCode = 6
)

// gateSubjectRefusal runs one harness invocation expected to FAIL and returns the
// exit status with whatever the script wrote to stderr.
//
// A refusal whose reason nobody can read is half a refusal: every assertion below
// checks the status AND the sentence, because a script that exits 6 for the wrong
// reason passes a status-only test while telling the human nothing they can act
// on.
func gateSubjectRefusal(t *testing.T, args ...string) (code int, stderr string) {
	t.Helper()
	script := filepath.Join(surfaceRepoRoot(t), filepath.FromSlash(gateStage2HarnessFile))
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	out, err := cmd.Output()
	if err == nil {
		t.Fatalf("expected a refusal, got a clean exit with stdout %q", strings.TrimSpace(string(out)))
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("running the harness: %v", err)
	}
	return exitErr.ExitCode(), string(exitErr.Stderr)
}

// gateSubjectWrite puts a subject record into an overlay root.
func gateSubjectWrite(t *testing.T, root, version, frozen, current, reason string) {
	t.Helper()
	body := "{\n" +
		"  \"version\": \"" + version + "\",\n" +
		"  \"frozen_sha256\": \"" + frozen + "\",\n" +
		"  \"surfaces_sha256\": \"" + current + "\",\n" +
		"  \"frozen_at_run\": \"run-under-test\",\n" +
		"  \"thaw_reason\": \"" + reason + "\"\n" +
		"}\n"
	gateWrite(t, root, gateSubjectFile, body)
}

// gateSubjectMoveManifest appends a comment to the overlay's manifest. The
// overlay copies top-level files rather than symlinking them, so this cannot
// reach the real surfaces.yaml.
func gateSubjectMoveManifest(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "surfaces.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the overlay manifest: %v", err)
	}
	if err := os.WriteFile(path, append(body, []byte("\n# a surface declared mid-release\n")...), 0o644); err != nil {
		t.Fatalf("move the overlay manifest: %v", err)
	}
}

// gateSubjectFreeze mints a freeze in the overlay and returns the digest it
// recorded.
func gateSubjectFreeze(t *testing.T, root, run string) string {
	t.Helper()
	if _, err := gateStage2Harness(t, "subject", "--root", root, "--freeze", "--run", run); err != nil {
		t.Fatalf("freezing the subject: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(gateSubjectFile)))
	if err != nil {
		t.Fatalf("read the freeze that was just written: %v", err)
	}
	return gateSubjectField(t, string(body), "frozen_sha256")
}

// gateSubjectField reads one flat string field out of a subject record, the same
// way the harness's own json_scalar does.
func gateSubjectField(t *testing.T, body, key string) string {
	t.Helper()
	marker := "\"" + key + "\": \""
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("the subject record carries no %s field:\n%s", key, body)
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, "\"")
	if j < 0 {
		t.Fatalf("the %s field in the subject record is unterminated:\n%s", key, body)
	}
	return rest[:j]
}

func TestGateSubjectRefusesAManifestThatMovedMidRelease(t *testing.T) {
	overlay, _ := gateStage2Overlay(t)
	frozen := gateSubjectFreeze(t, overlay, "round-one")

	// The freeze holds while nothing moves.
	if _, err := gateStage2Harness(t, "subject", "--root", overlay); err != nil {
		t.Fatalf("the subject was verified against the manifest it was frozen over and refused: %v", err)
	}

	gateSubjectMoveManifest(t, overlay)

	code, stderr := gateSubjectRefusal(t, "subject", "--root", overlay)
	if code != gateSubjectExitCode {
		t.Errorf("a manifest that moved mid-release exited %d, want %d — the code is what tells this failure from a missing input, and the two have different recoveries", code, gateSubjectExitCode)
	}
	if !strings.Contains(stderr, frozen) {
		t.Errorf("the refusal does not name the digest the release froze (%s), so a human is told that something moved and not what:\n%s", frozen, stderr)
	}
	// BOTH digests, not one. At mint frozen_sha256 == surfaces_sha256, so a
	// single assertion on the frozen value is satisfied by either field — delete
	// the `on disk:` line from the refusal and a one-sided test stays green while
	// the human loses the half that says what the manifest moved TO.
	// The LABELLED line, not just the digest: the recovery sentence further down
	// also carries the new digest ("set surfaces_sha256 to <x>"), so an assertion
	// on the bare string stays green when the three-line comparison a human reads
	// is deleted. Measured — that is what the first version of this did.
	if moved := gateSubjectMovedDigest(t, overlay); !strings.Contains(stderr, "on disk:    "+moved) {
		t.Errorf("the refusal no longer shows `on disk:    %s` beside the frozen digest, so it says the manifest moved without showing what it moved to:\n%s", moved, stderr)
	}
	for _, want := range []string{"revert", "thaw_reason"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the refusal does not mention %q, so it names a problem with no way forward:\n%s", want, stderr)
		}
	}
}

func TestGateSubjectRefusesAThawWithNoReason(t *testing.T) {
	overlay, _ := gateStage2Overlay(t)
	frozen := gateSubjectFreeze(t, overlay, "round-one")

	gateSubjectMoveManifest(t, overlay)

	// The shape a maintainer produces when they accept the widening and do not
	// say why: the current digest is updated to match the moved manifest, and
	// frozen_sha256 — which is never edited — still records what round one asked.
	moved := gateSubjectMovedDigest(t, overlay)
	gateSubjectWrite(t, overlay, "v0.5.2", frozen, moved, "")

	code, stderr := gateSubjectRefusal(t, "subject", "--root", overlay)
	if code != gateSubjectExitCode {
		t.Errorf("a thaw with no reason exited %d, want %d", code, gateSubjectExitCode)
	}
	if !strings.Contains(stderr, "THAW with no reason") {
		t.Errorf("the refusal does not say the subject was re-opened without a reason:\n%s", stderr)
	}

	// The same edit WITH a reason is accepted. The mechanism has to have a way
	// through, or the next release under time pressure goes around it instead.
	gateSubjectWrite(t, overlay, "v0.5.2", frozen, moved, "ruled blocking: the changelog surface could not see retired.go")
	if _, err := gateStage2Harness(t, "subject", "--root", overlay); err != nil {
		t.Errorf("a thaw carrying a reason was refused, so the escape hatch does not open: %v", err)
	}
}

func TestGateSubjectDoesNotBindTheNextRelease(t *testing.T) {
	overlay, _ := gateStage2Overlay(t)

	// A freeze belonging to the PREVIOUS release, over a digest that matches
	// nothing in this tree. It must not refuse: this release has not frozen
	// anything yet, and inheriting a predecessor's subject is the stale-evidence
	// failure the rest of the gate refuses everywhere else.
	gateSubjectWrite(t, overlay, "v0.5.1", strings.Repeat("a", 64), strings.Repeat("a", 64), "")

	if _, err := gateStage2Harness(t, "subject", "--root", overlay); err != nil {
		t.Fatalf("a freeze from the previous release bound this one: %v", err)
	}

	// And the next fan-out re-mints it for the release actually being read.
	digest := gateSubjectFreeze(t, overlay, "round-one-of-this-release")
	body, err := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(gateSubjectFile)))
	if err != nil {
		t.Fatalf("read the re-minted freeze: %v", err)
	}
	if got := gateSubjectField(t, string(body), "version"); got != "v0.5.2" {
		t.Errorf("the re-minted freeze names %s; the CHANGELOG's newest heading is v0.5.2, and the freeze has to name the release it belongs to", got)
	}
	if digest == strings.Repeat("a", 64) {
		t.Error("the re-minted freeze kept the predecessor's digest rather than digesting this tree's manifest")
	}
}

func TestGateSubjectALaterRoundDoesNotRewriteTheRecord(t *testing.T) {
	overlay, _ := gateStage2Overlay(t)
	gateSubjectFreeze(t, overlay, "round-one")

	// A thaw the maintainer recorded, which a later round must not erase.
	body, err := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(gateSubjectFile)))
	if err != nil {
		t.Fatalf("read the freeze: %v", err)
	}
	frozen := gateSubjectField(t, string(body), "frozen_sha256")
	gateSubjectWrite(t, overlay, "v0.5.2", frozen, frozen, "ruled blocking in round three")

	if _, err := gateStage2Harness(t, "subject", "--root", overlay, "--freeze", "--run", "round-four"); err != nil {
		t.Fatalf("re-freezing during the same release refused: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(overlay, filepath.FromSlash(gateSubjectFile)))
	if err != nil {
		t.Fatalf("read the record after a later round: %v", err)
	}
	if got := gateSubjectField(t, string(after), "thaw_reason"); got != "ruled blocking in round three" {
		t.Errorf("a later round erased the recorded thaw reason (now %q). A record a run rewrites is a record that says whatever the last run felt like", got)
	}
	if got := gateSubjectField(t, string(after), "frozen_at_run"); got != "run-under-test" {
		t.Errorf("a later round re-stamped frozen_at_run to %q; the field names the fan-out that FIRST asked this release's question", got)
	}
}

func TestGateSubjectFanoutVerifiesBeforeItMintsAnything(t *testing.T) {
	overlay, _ := gateStage2Overlay(t)
	gateSubjectFreeze(t, overlay, "round-one")
	gateSubjectMoveManifest(t, overlay)

	code, stderr := gateSubjectRefusal(t, "fanout", "--root", overlay, "--tree", strings.Repeat("b", 40))
	if code != gateSubjectExitCode {
		t.Errorf("fanout over a moved subject exited %d, want %d:\n%s", code, gateSubjectExitCode, stderr)
	}
	// The status alone would pass against ANY exit-6 refusal, including one
	// raised for a reason that has nothing to do with the subject — which is
	// exactly the status-only test this file's own header warns about.
	if !strings.Contains(stderr, "moved during release") {
		t.Errorf("fanout exited %d but its reason is not the moved subject, so this test would pass against an unrelated refusal carrying the same code:\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(overlay, "gate", "fanout.json")); !os.IsNotExist(err) {
		t.Error("the fan-out minted a run before verifying the subject. A refusal that arrives after an identifier exists is indistinguishable on disk from a fan-out that half happened, and the answers filed under it would be answers to a question this release never agreed to ask")
	}
}

// TestGateSubjectTheTwoUnreasonedReopeningsArePinnedOpen states, as an
// assertion rather than as a sentence in a comment, that two ways of re-opening
// a subject carry no reason and are refused by nothing here.
//
// It exists because the header used to claim a thaw always costs a reason. It
// does not: only an edit to surfaces_sha256 does. Deleting the record or moving
// its version mints a fresh freeze silently. Both are legitimate — the version
// path is how the NEXT release starts, and the delete path is how a maintainer
// abandons a freeze — and both are visible in a reviewed diff because the file
// is tracked. Pinning them open means a future reader meets the real shape
// rather than the flattering one, and a change that closes either has to change
// this test on purpose.
func TestGateSubjectTheTwoUnreasonedReopeningsArePinnedOpen(t *testing.T) {
	t.Run("deleting the record mints a fresh freeze with no reason", func(t *testing.T) {
		overlay, _ := gateStage2Overlay(t)
		gateSubjectFreeze(t, overlay, "round-one")
		gateSubjectMoveManifest(t, overlay)
		if err := os.Remove(filepath.Join(overlay, filepath.FromSlash(gateSubjectFile))); err != nil {
			t.Fatalf("remove the freeze: %v", err)
		}
		if _, err := gateStage2Harness(t, "subject", "--root", overlay); err != nil {
			t.Fatalf("a deleted freeze was refused; if that is now the intent, this test is the record of the change: %v", err)
		}
		if got := gateSubjectFreeze(t, overlay, "round-two"); got != gateSubjectMovedDigest(t, overlay) {
			t.Errorf("the re-minted freeze does not cover the manifest as it now stands (%s)", got)
		}
	})

	t.Run("moving the version mints a fresh freeze with no reason", func(t *testing.T) {
		overlay, _ := gateStage2Overlay(t)
		frozen := gateSubjectFreeze(t, overlay, "round-one")
		gateSubjectMoveManifest(t, overlay)
		gateSubjectWrite(t, overlay, "v9.9.9", frozen, frozen, "")
		if _, err := gateStage2Harness(t, "subject", "--root", overlay); err != nil {
			t.Fatalf("a freeze naming another release bound this one, which is what TestGateSubjectDoesNotBindTheNextRelease says it must not: %v", err)
		}
	})
}

func TestGateSubjectRefusesAFreezeThatNamesNoRun(t *testing.T) {
	overlay, _ := gateStage2Overlay(t)

	code, stderr := gateSubjectRefusal(t, "subject", "--root", overlay, "--freeze")
	if code != 1 {
		t.Errorf("a freeze naming no run exited %d, want 1 (a usage error)", code)
	}
	if !strings.Contains(stderr, "--run is required") {
		t.Errorf("the refusal does not say what was missing:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(overlay, filepath.FromSlash(gateSubjectFile))); !os.IsNotExist(err) {
		t.Error("a refused freeze still wrote a subject record, so the release is now frozen to a question no run asked")
	}
}

// gateSubjectMovedDigest is the digest of the overlay's manifest as it stands
// now — used to build the record a maintainer would write when accepting a
// widening.
func gateSubjectMovedDigest(t *testing.T, root string) string {
	t.Helper()
	digest, err := gateStage2FileDigest(root, "surfaces.yaml")
	if err != nil {
		t.Fatalf("digest the moved manifest: %v", err)
	}
	return digest
}
