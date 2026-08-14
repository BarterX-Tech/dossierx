// gate_deferred_wire_test.go pins THE WIRE: four documents — gate/.gitignore,
// CHANGELOG.md, docs/RELEASING.md and the priority design note — already claim
// that "the next release's round one reads gate/deferred.json as input".
// gateReadDeferred (gate_priority_test.go) can read the file back, but nothing
// outside its own tests ever called it. A claim four documents make and no code
// path acts on is exactly the defect class CLAUDE.md's override-record story
// warns about: this repository shipped one of those earlier this release.
//
// WHAT scripts/gate-stage2/run.sh's `fanout` mode now does about it, and what is
// pinned here:
//
//	A LEDGER NAMING A PREVIOUS RELEASE PRINTS A NOTICE — to stderr, naming the
//	    version that deferred and how many findings, so a maintainer starting
//	    round one is told rather than having to remember to ask.
//	A LEDGER NAMING THIS RELEASE IS SILENT — it is this run's own projection
//	    (or an earlier round of the same release's), not a previous release's
//	    backlog, and a notice about it would tell round one to triage findings
//	    it already knows about from a file it is about to overwrite again.
//	A LEDGER THAT CANNOT BE READ DIES RATHER THAN BEING TREATED AS EMPTY —
//	    CLAUDE.md's rule for this gate: a check that cannot run is a failure,
//	    not a pass, and a truncated or hand-broken ledger might carry findings
//	    nobody has triaged yet.
//	A PREVIOUS RELEASE'S LEDGER OF ZERO FINDINGS GETS ONE LINE, NOT THE NOTICE
//	    — there is nothing in it for round one to triage, and the triage
//	    instruction would print on every fan-out of every round until somebody
//	    deleted the file. The line says which release deferred nothing and to
//	    delete it, which is the whole of the action available.
//
// AND THE OTHER HALF OF THE SAME DEFECT, fixed in gate_priority_test.go rather
// than here: the notice used to end "leave it — the next recording overwrites it
// in full", and the only thing that recorded was the driver's D1, which does not
// run until the release is being published. So the recovery it offered at round
// one was to wait for something that happens after every round is over, and the
// notice fired again on every fan-out in between. TestGateDeferredProject now
// projects the ledger per round, and the notice names that invocation.
//
// THESE ROWS RUN AGAINST THE REAL REPOSITORY ROOT, not an overlay, and that is
// deliberate rather than a shortcut: TestGateFanoutProduce (gate_fanout_test.go)
// resolves its own root from the package's source location and ignores --root
// entirely, so a `fanout` run driven far enough to reach the producer always
// acts on THIS checkout no matter what --root named. gateFanoutStashRecord and
// gateFanoutRunHarness already exist for exactly that reason — this file reuses
// them rather than inventing a second way to drive the same script safely — and
// gateDeferredWireStash below does for gate/deferred.json what
// gateFanoutStashRecord does for gate/fanout.json. Every row here supplies a
// well-formed 40-hex tree that names no real object, so the producer's own
// identity check refuses BEFORE writing anything (gateFanoutProduce's own
// comment: "FROM HERE ON THIS CHECKOUT HOLDS NO FAN-OUT RECORD UNTIL THIS
// PRODUCTION WRITES ONE") — the same cheap, non-destructive shape
// TestGateFanoutHarnessRefusesWhatItCannotProduce already relies on.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gateDeferredWireVersionHeading is release_version's own regex, re-read in Go
// rather than assumed, so a fixture that claims "this release's version" is
// actually checked against the CHANGELOG rather than against a guess that goes
// stale the day the heading moves.
var gateDeferredWireVersionHeading = regexp.MustCompile(`(?m)^## \[([0-9]+\.[0-9]+\.[0-9]+)\]`)

// gateDeferredWireCurrentVersion is the version the script will actually compute
// for this checkout, so that a test asserting "this release's own ledger" and one
// asserting "a previous release's ledger" are both built against it rather than
// against a literal that drifts the day somebody cuts the next release.
//
// It goes through gateDeferredVersion, which is the function the PROJECTOR writes
// the ledger's version with (gate_priority_test.go). That is deliberate: the
// notice these rows are about fires exactly when the ledger's version differs from
// release_version's answer, so a test that computed the version its own third way
// could pass while the projector and the script disagreed.
func gateDeferredWireCurrentVersion(t *testing.T, root string) string {
	t.Helper()
	version, err := gateDeferredVersion(root)
	if err != nil {
		t.Fatalf("this checkout cannot name its own release, so release_version would read no version either: %v", err)
	}
	return version
}

// gateDeferredWireStash moves the real checkout's gate/deferred.json aside for
// the duration of one test and restores exactly what was there, or removes the
// file if there was nothing — gateFanoutStashRecord's own reasoning, applied to
// the other tracked file a `fanout` run can read. Without it, a row that writes
// a fixture ledger into the real gate/deferred.json would either clobber a real
// deferral this repository is carrying, or leave a fixture behind for the next
// git status to notice.
func gateDeferredWireStash(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(gateDeferredFile))
	switch stashed, err := os.ReadFile(path); {
	case err == nil:
		if err := os.Remove(path); err != nil {
			t.Fatalf("%s exists in this checkout and could not be moved aside: %v", gateDeferredFile, err)
		}
		t.Cleanup(func() {
			if err := os.WriteFile(path, stashed, 0o644); err != nil {
				t.Errorf("could not restore the %s this checkout had before this test ran: %v\nThe bytes were:\n%s", gateDeferredFile, err, stashed)
			}
		})
	case errors.Is(err, os.ErrNotExist):
		t.Cleanup(func() {
			if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
				t.Errorf("this test's fixture %s could not be cleaned up: %v", gateDeferredFile, rmErr)
			}
		})
	default:
		t.Fatalf("%s could not be read to stash it: %v", gateDeferredFile, err)
	}
}

// gateDeferredWireLedger builds n findings under the given version, in the
// shape gateWriteDeferred actually writes (a two-space-indented object, ending
// on its own closing brace) — real enough that deferred_notice's "does it end
// where a complete one does" check passes it.
func gateDeferredWireLedger(version string, n int) string {
	findings := make([]string, 0, n)
	for i := 0; i < n; i++ {
		findings = append(findings, fmt.Sprintf(`    {
      "surface": "surface-%d",
      "rule": "a rule",
      "consequence": "cosmetic",
      "failure_scenario": "a reader is mildly inconvenienced",
      "detail": "some detail",
      "priority": "P3"
    }`, i))
	}
	return fmt.Sprintf("{\n  \"version\": %q,\n  \"findings\": [\n%s\n  ]\n}\n", version, strings.Join(findings, ",\n"))
}

// TestGateDeferredWireFanoutNoticesAPreviousReleasesLedger is the positive
// control: a ledger left by a release other than this one produces a stderr
// notice naming that version and how many findings it carries.
func TestGateDeferredWireFanoutNoticesAPreviousReleasesLedger(t *testing.T) {
	root := surfaceRepoRoot(t)
	gateFanoutStashRecord(t, root)
	gateDeferredWireStash(t, root)

	current := gateDeferredWireCurrentVersion(t, root)
	previous := current + "-a-previous-release"
	if previous == current {
		t.Fatalf("the fixture version %q did not actually differ from the current one %q", previous, current)
	}

	path := filepath.Join(root, filepath.FromSlash(gateDeferredFile))
	if err := os.WriteFile(path, []byte(gateDeferredWireLedger(previous, 3)), 0o644); err != nil {
		t.Fatalf("write the fixture ledger: %v", err)
	}

	out, code := gateFanoutRunHarness(t, "", "fanout", "--root", root, "--tree", strings.Repeat("a", 40))

	// The producer refuses on the identity mismatch — see this file's header —
	// which is what proves the notice ran as part of a REAL `fanout` mode and
	// not some other codepath, while costing nothing but that one comparison.
	if code != 5 {
		t.Fatalf("fanout exited %d, want 5 (the producer's own refusal on an object name that names nothing):\n%s", code, out)
	}
	if !strings.Contains(out, previous) {
		t.Errorf("the notice does not name the deferring release %q:\n%s", previous, out)
	}
	if !strings.Contains(out, "deferred 3 finding") {
		t.Errorf("the notice does not carry the count of deferred findings (3):\n%s", out)
	}
	if !strings.Contains(out, gateDeferredFile) {
		t.Errorf("the notice does not name %s, so a maintainer reading it does not know which file to act on:\n%s", gateDeferredFile, out)
	}
	if !strings.Contains(out, "round-one") {
		t.Errorf("the notice does not say these findings are round one's input:\n%s", out)
	}
}

// TestGateDeferredWireFanoutSaysThereIsNothingToTriage is the empty-ledger case,
// which is a previous release that deferred nothing — `"findings": []`, the
// deliberate shape gateWriteDeferred emits so that "deferred nothing" and "never
// wrote the file" are different bytes.
//
// It gets ONE LINE and not the notice. There is nothing in the file for round one
// to triage, so printing the triage instruction would send a maintainer to read an
// empty list on every fan-out of every round until somebody deleted it — and a
// notice that is wrong most of the times it fires is one people learn to scroll
// past, which costs the notice its whole value on the round where it is right. The
// line still names the release and the file, because a stale tracked artifact is
// worth one sentence and deleting it is the only action available.
func TestGateDeferredWireFanoutSaysThereIsNothingToTriage(t *testing.T) {
	root := surfaceRepoRoot(t)
	gateFanoutStashRecord(t, root)
	gateDeferredWireStash(t, root)

	previous := gateDeferredWireCurrentVersion(t, root) + "-a-previous-release"
	path := filepath.Join(root, filepath.FromSlash(gateDeferredFile))
	if err := os.WriteFile(path, []byte(gateDeferredWireLedger(previous, 0)), 0o644); err != nil {
		t.Fatalf("write the fixture ledger: %v", err)
	}

	out, code := gateFanoutRunHarness(t, "", "fanout", "--root", root, "--tree", strings.Repeat("a", 40))

	if code != 5 {
		t.Fatalf("fanout exited %d, want 5 (the producer's own refusal on an object name that names nothing):\n%s", code, out)
	}
	if !strings.Contains(out, "deferred nothing") {
		t.Errorf("fanout did not say that the ledger it found defers nothing, so a maintainer either reads a triage instruction over an empty list or hears nothing about a stale tracked file:\n%s", out)
	}
	for _, want := range []string{previous, gateDeferredFile} {
		if !strings.Contains(out, want) {
			t.Errorf("the line does not name %q, so it cannot be acted on:\n%s", want, out)
		}
	}
	if strings.Contains(out, "round-one") {
		t.Errorf("fanout printed the triage instruction over a ledger with nothing in it; there is no finding to hand to any surface:\n%s", out)
	}
}

// TestGateDeferredWireFanoutIsSilentOnItsOwnVersion is the negative control: a
// ledger whose version equals THIS release's own is this run's own projection
// (or an earlier round's), and printing a notice about it would send round one
// to re-triage findings it already knows, from a file about to be overwritten
// again.
func TestGateDeferredWireFanoutIsSilentOnItsOwnVersion(t *testing.T) {
	root := surfaceRepoRoot(t)
	gateFanoutStashRecord(t, root)
	gateDeferredWireStash(t, root)

	current := gateDeferredWireCurrentVersion(t, root)
	path := filepath.Join(root, filepath.FromSlash(gateDeferredFile))
	if err := os.WriteFile(path, []byte(gateDeferredWireLedger(current, 1)), 0o644); err != nil {
		t.Fatalf("write the fixture ledger: %v", err)
	}

	out, code := gateFanoutRunHarness(t, "", "fanout", "--root", root, "--tree", strings.Repeat("a", 40))

	if code != 5 {
		t.Fatalf("fanout exited %d, want 5 (the producer's own refusal on an object name that names nothing):\n%s", code, out)
	}
	if strings.Contains(out, "round-one") || strings.Contains(out, "deferred") {
		t.Errorf("fanout spoke about %s even though its version (%s) is this release's own — a maintainer would be sent to re-triage findings this run already knows about, from a file it is about to overwrite:\n%s", gateDeferredFile, current, out)
	}
}

// TestGateDeferredWireFanoutRefusesAnUnreadableLedger is CLAUDE.md's rule for
// this gate, applied to the one file besides gate/subject.json that a `fanout`
// run reads back rather than only writes: a check that cannot run is a
// failure, not a pass. A ledger cut off mid-array might carry findings nobody
// has triaged, and reading that as "nothing deferred" would drop them exactly
// as silently as never having written the file at all.
func TestGateDeferredWireFanoutRefusesAnUnreadableLedger(t *testing.T) {
	root := surfaceRepoRoot(t)
	gateFanoutStashRecord(t, root)
	gateDeferredWireStash(t, root)

	// A version IS present and readable — this is not the missing-version case
	// below — but the findings array is cut off mid-object, the shape a process
	// that died half way through a hand-edit would leave.
	truncated := "{\n  \"version\": \"v0.0.1\",\n  \"findings\": [\n    {\n      \"surface\": \"readme\",\n"
	path := filepath.Join(root, filepath.FromSlash(gateDeferredFile))
	if err := os.WriteFile(path, []byte(truncated), 0o644); err != nil {
		t.Fatalf("write the fixture ledger: %v", err)
	}

	out, code := gateFanoutRunHarness(t, "", "fanout", "--root", root, "--tree", strings.Repeat("a", 40))

	if code != 2 {
		t.Fatalf("fanout exited %d, want 2 (an input could not be read); a truncated ledger passed as though nothing were deferred:\n%s", code, out)
	}
	if !strings.Contains(out, gateDeferredFile) {
		t.Errorf("the refusal does not name %s:\n%s", gateDeferredFile, out)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(gateFanoutFile))); !os.IsNotExist(statErr) {
		t.Error("a ledger the harness refused to read still let a fan-out get minted; the refusal must arrive before anything is written, the same rule subject_verify already follows")
	}

	t.Run("a version-less ledger refuses the same way", func(t *testing.T) {
		if err := os.WriteFile(path, []byte("not a json document at all\n"), 0o644); err != nil {
			t.Fatalf("write the fixture ledger: %v", err)
		}
		out, code := gateFanoutRunHarness(t, "", "fanout", "--root", root, "--tree", strings.Repeat("a", 40))
		if code != 2 {
			t.Fatalf("fanout exited %d, want 2:\n%s", code, out)
		}
		if !strings.Contains(out, gateDeferredFile) {
			t.Errorf("the refusal does not name %s:\n%s", gateDeferredFile, out)
		}
	})
}
