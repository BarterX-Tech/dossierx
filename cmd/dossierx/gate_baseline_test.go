// gate_baseline_test.go holds surface.baseline.json: the surface inventory of
// v0.5.0, and the only baseline the first gated release can be diffed against.
//
// WHAT THE ARTIFACT IS. v0.5.0 shipped before the surface emitter existed, so it
// carries no surface.json of its own (`git show v0.5.0:surface.json` fails, and
// that is a fact about the release, not about the clone). The baseline was
// therefore MANUFACTURED once, by the only method that answers "what would this
// inventory have said about v0.5.0": HEAD's emitter source — surface_test.go and
// surface_meta_test.go, copied in unchanged — built and run inside a detached
// v0.5.0 worktree. v0.5.0's tree is frozen, so the artifact has exactly one
// correct value, forever, and it can never legitimately be regenerated against
// anything else.
//
// THERE IS NO -regenerate-goldens PATH HERE, AND THAT IS THE POINT. Every other
// generated artifact in this repository is regenerated with that flag, and
// surface_test.go prints "regenerate it with: ..." as the one recovery a
// contributor ever sees. Applied to this file that habit is destructive: a
// maintainer who notices the baseline "looks stale" beside a much-changed
// surface.json and re-runs the emitter against the CURRENT tree produces an
// inventory of a tree nobody released, after which every delta reports as
// unchanged exactly the surfaces that changed. So the file is not regenerable by
// any command in this repository, and the failure message below names the v0.5.0
// tree rather than the current one.
//
// HOW ITS CORRECTNESS STAYS DECIDABLE. Today the honest artifact is BYTE-
// IDENTICAL to the committed surface.json — nothing but test files has changed
// under cmd/, internal/, skills/ or go.mod since v0.5.0, so HEAD's inventory and
// v0.5.0's coincide — and no check can tell identical files apart. What the
// checks below do instead is anchor the artifact to V0.5.0'S OWN BYTES rather
// than to whatever HEAD happens to say: every field that is a pure function of a
// checkout is recomputed from the v0.5.0 commit, read straight out of the object
// database. So `cp surface.json surface.baseline.json` passes today and stops
// passing the moment HEAD moves — which is the moment it matters — and a
// re-derivation against a newer tree fails on the same comparison.
//
// THE LINK-TIME NAMES ARE RECOMPUTED TOO, BY BUILDING v0.5.0. commands,
// root_flags, retired, lint_rules, skills and envelope do not fall out of a
// checkout: they are what the program IS once it is linked — newRootCmd()
// walked, lint.Registry read, live types reflected over. The only way to learn
// what v0.5.0's were is to build v0.5.0 and ask it, which means a nested `go
// test` inside `go test` that nothing else in this repository does. That was
// left undone once, and the hole was exactly the size of the six fields: a
// hand-edited name among them was indistinguishable from a derived one, because
// the fingerprints, the counts and the canonical-form check all stay green over
// a renamed lint rule. TestGateBaselineLinkTimeNamesAreTheFrozenTreesOwnBuild
// closes it — HEAD's emitter source is copied into the materialised v0.5.0 tree,
// built and run there, and all six fields are compared against what it reports.
//
// `counts` is the one field not recomputed, and it is arithmetic over lists that
// now are: TestGateBaselineCountsAgreeWithItsOwnLists holds all five numbers
// against the document's own lists.
// TestGateBaselineFieldsAreRecomputedOrNamedAsResidue is what keeps this
// paragraph honest: it fails if a field of surfaceDoc is in neither list.
//
// AND WHOSE BASELINE IT IS. This artifact is v0.5.0's and nothing else's.
// gateBaselineFor is the ONE answer in this package to "what is this release's
// baseline", and it chooses the bootstrap because the previous release IS
// v0.5.0's commit — never because some other way of getting a baseline returned
// an error. That distinction is the whole of failure scenario 2: in a shallow
// checkout `git show v0.6.0:surface.json` fails with the identical message an
// absent tag produces, so a resolver keyed on that failure would hand thirteen
// agents v0.5.0's inventory as the truth about v0.6.0 and produce a full,
// plausible, entirely wrong delta. A baseline that cannot be resolved is a
// FAILURE, in the same words CLAUDE.md uses for a missing browser — never a
// delta that happens to be empty.
//
// WHY IT IS TEST CODE. The same reason surface_test.go and gate_receipt_test.go
// are: nothing here is wired to a cobra verb, nothing is compiled into the
// shipped binary, and none of it moves surface.json's behaviour_fingerprint.
package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// the artifact's identity
// ---------------------------------------------------------------------

const (
	// gateBaselineBootstrapFile is the frozen artifact, relative to the repository root.
	// It is read as a document of the same shape as surface.json — same keys,
	// same order, same encoding — so a consumer needs no second reader.
	gateBaselineBootstrapFile = "surface.baseline.json"

	// gateBaselineRelease is the release this artifact is the inventory OF. It
	// is a label for humans and messages; it is NOT the identity — see
	// gateBaselineCommit.
	gateBaselineRelease = "v0.5.0"

	// gateBaselineCommit is the identity: the commit v0.5.0's annotated tag
	// object points at, and the tree every recomputation below reads.
	//
	// The COMMIT and not the tag name, because a tag name is a mutable pointer.
	// `git tag -f v0.5.0 <anything>` re-points it under any check that names
	// only the tag, and the annotated tag OBJECT's own sha (39581415…) is not
	// the tree anybody released either — which is why every read here goes
	// through this sha and why TestGateBaselineTagStillNamesTheFrozenCommit
	// exists to catch the pointer moving.
	gateBaselineCommit = "3217a48b4a123ea4b8b02f93fac6337b985eb7ce"

	// gateBaselinePrevReleaseTagEnv names the previous release outright. It is
	// deliberately the SAME knob tests/render_across_releases_test.go reads and
	// .github/workflows/ci.yml resolves: one variable names the baseline for
	// this repository, and a maintainer who is told to set it should not have to
	// discover that there are two.
	gateBaselinePrevReleaseTagEnv = "DOSSIERX_PREV_RELEASE_TAG"
)

// gateBaselineRecovery is what a maintainer does when the recomputations below
// disagree with the committed artifact. It is spelled as commands, and it names
// the V0.5.0 TREE in every one of them, because the reflex this file is written
// against is `-regenerate-goldens` against the current tree.
const gateBaselineRecovery = "surface.baseline.json is not regenerable from this tree, and running the emitter here would\n" +
	"replace v0.5.0's inventory with an inventory of a tree nobody released. If the artifact is\n" +
	"genuinely wrong, it is re-manufactured against the frozen tree and nowhere else:\n" +
	"\tgit worktree add --detach /tmp/dossierx-v0.5.0 " + gateBaselineCommit + "\n" +
	"\tcp cmd/dossierx/surface_test.go cmd/dossierx/surface_meta_test.go /tmp/dossierx-v0.5.0/cmd/dossierx/\n" +
	"\t(cd /tmp/dossierx-v0.5.0 && go test ./cmd/dossierx -run TestSurface)   # the extractions are complete there\n" +
	"\t(cd /tmp/dossierx-v0.5.0 && go test ./cmd/dossierx -run TestGenerateSurfaceJSON -regenerate-goldens)\n" +
	"\tcp /tmp/dossierx-v0.5.0/surface.json surface.baseline.json && git worktree remove /tmp/dossierx-v0.5.0\n" +
	"The worktree must live OUTSIDE this repository: scratchpad/land-lane.sh counts every untracked\n" +
	"path under the tree as a write-set violation."

// ---------------------------------------------------------------------
// which document is this release's baseline
// ---------------------------------------------------------------------

// gateBaselineKind is where a gate run's baseline inventory comes from.
type gateBaselineKind string

const (
	// gateBaselineFromBootstrap is the frozen artifact in this repository. It is
	// reachable for exactly one previous release and no other.
	gateBaselineFromBootstrap gateBaselineKind = "bootstrap"
	// gateBaselineFromTag is the previous release's own committed surface.json,
	// read out of the tag.
	gateBaselineFromTag gateBaselineKind = "tag"
)

// gateBaselineSource is the resolved answer to "what is this release's
// baseline": one kind, and one place to read it from.
type gateBaselineSource struct {
	Kind gateBaselineKind
	// Release is the previous release's tag name, for messages.
	Release string
	// Commit is the previous release's commit — the identity the choice was
	// made on.
	Commit string
	// Path is set for the bootstrap: a repo-relative file.
	Path string
	// Rev is set for a tag: a `git show`-able `<tag>:surface.json`.
	Rev string
}

// gateBaselineFor decides which document is the baseline for a release whose
// PREDECESSOR is the given tag and commit. It is the one answer in this package,
// and it is pure so that the branch that matters — "the previous release is not
// v0.5.0" — is reachable in a test today, years before this repository can
// produce that state on its own.
//
// IT IS KEYED ON IDENTITY, NEVER ON A FAILURE. The bootstrap is returned because
// the previous release IS the frozen commit. The shape this replaces — "when
// `git show <prev-tag>:surface.json` fails, use the bootstrap" — cannot work,
// because in a `--depth 1` checkout that command fails with `fatal: invalid
// object name 'v0.6.0'`, which is character for character what an absent tag
// says. A resolver that could not tell those apart would substitute v0.5.0's
// inventory for v0.6.0's and produce a delta spanning two releases: full,
// plausible, and wrong, handed to every surface agent as the truth about the
// past. Loud enough to look like work getting done is what makes it worse than
// an empty answer.
//
// An input it cannot decide on is errGateUncheckable — the package's existing
// sentinel for "the check could not be made" — and never a default.
func gateBaselineFor(release, commit string) (gateBaselineSource, error) {
	if release == "" {
		return gateBaselineSource{}, fmt.Errorf("%w: no previous release was named, so there is nothing to diff this one against. "+
			"A baseline that cannot be resolved is a failed gate, not a delta that happens to be empty", errGateUncheckable)
	}
	if !gateSHARE.MatchString(commit) {
		return gateBaselineSource{}, fmt.Errorf("%w: %q resolves to %q, which is not a full commit object name. "+
			"A tag NAME is a mutable pointer and is not sufficient grounds to hand a document to the surface agents as the truth about the past; "+
			"resolve it with `git rev-parse %s^{commit}` and fail if that cannot answer", errGateUncheckable, release, commit, release)
	}
	if commit == gateBaselineCommit {
		return gateBaselineSource{
			Kind:    gateBaselineFromBootstrap,
			Release: release,
			Commit:  commit,
			Path:    gateBaselineBootstrapFile,
		}, nil
	}
	return gateBaselineSource{
		Kind:    gateBaselineFromTag,
		Release: release,
		Commit:  commit,
		Rev:     release + ":" + surfaceFileName,
	}, nil
}

// gateBaselineResolveFault names the previous release the way
// tests/render_across_releases_test.go's resolveBaseline does, and for the
// reasons written out there: DOSSIERX_PREV_RELEASE_TAG names it outright, `git
// describe` finds the newest tag reachable from HEAD, and NEITHER MAY DECLINE.
// A checkout with no tags — which is what actions/checkout produces at its
// default depth — is the environment to fix, not the one to reassure.
//
// It returns its refusals as text rather than failing the test, for the reason
// gateBaselineAnchorFault does: this clone has tags and resolves them, so both
// refusals are unreachable here. Written inline they would be branches that had
// never executed — and both were exactly that until a fixture repository was
// pointed at them, which is when "the check could not run" stops being a claim
// and becomes something that has been watched happening.
func gateBaselineResolveFault(root string) (release, commit, fault string) {
	release = strings.TrimSpace(os.Getenv(gateBaselinePrevReleaseTagEnv))
	if release == "" {
		out, err := gateGit(root, "describe", "--tags", "--abbrev=0")
		if err != nil || out == "" {
			return "", "", fmt.Sprintf("no release tag is reachable from HEAD, so this release's BASELINE cannot be identified and\n"+
				"NOTHING WOULD BE COMPARED. This is a failure, not a skip: without a baseline the surface delta the\n"+
				"gate hands its agents is empty, and an empty delta reads exactly like a release that changed nothing.\n\n"+
				"Fetch the tags, or name the previous release outright:\n"+
				"    git fetch --tags --force\n"+
				"    %s=<previous release tag> go test ./cmd/dossierx -run TestGateBaseline\n\n"+
				"In CI, check out with `fetch-depth: 0` and resolve %s in a step that fails when it finds no tag —\n"+
				"the shape .github/workflows/ci.yml already uses.\n\n(%v)",
				gateBaselinePrevReleaseTagEnv, gateBaselinePrevReleaseTagEnv, err)
		}
		release = out
	}
	commit, err := gateResolve(root, release+"^{commit}")
	if err != nil {
		return release, "", fmt.Sprintf("%q does not resolve to a commit, so the baseline's identity cannot be established: %v\n"+
			"A tag name alone is not an identity — it is a pointer that `git tag -f` re-points — so the gate refuses\n"+
			"rather than trusting the name.", release, err)
	}
	return release, commit, ""
}

// gateBaselineResolve is gateBaselineResolveFault with the refusal turned into a
// failed test, which is what every caller in this repository wants.
func gateBaselineResolve(t *testing.T, root string) (release, commit string) {
	t.Helper()
	release, commit, fault := gateBaselineResolveFault(root)
	if fault != "" {
		t.Fatal(fault)
	}
	return release, commit
}

// gateBaselineReadableFault checks that the baseline a run resolved to is
// actually THERE, and says why not when it is not. "" means it can be read.
//
// This is the second half of failure scenario 2 and the half that is unreachable
// in this repository: the previous release is v0.5.0, so the tag arm — the one
// that must refuse rather than substitute the bootstrap when a release's own
// surface.json cannot be read — never executes here and will not until v0.6.0
// exists. Blinding it changed no test result, which is the same thing as not
// having it. A fixture repository reaches every arm instead.
func gateBaselineReadableFault(root string, src gateBaselineSource) string {
	switch src.Kind {
	case gateBaselineFromBootstrap:
		if _, err := os.Stat(filepath.Join(root, src.Path)); err != nil {
			return fmt.Sprintf("the previous release is %s, whose baseline is the committed %s — and it is not there: %v\n"+
				"%s carries no surface.json of its own, so there is no second way to get its inventory. A gate run\n"+
				"in this state has no past to diff against, which is a failed gate and not an empty delta.",
				gateBaselineRelease, src.Path, err, gateBaselineRelease)
		}
		return ""
	case gateBaselineFromTag:
		if _, err := gateGit(root, "show", src.Rev); err != nil {
			return fmt.Sprintf("the previous release is %s, whose baseline is `git show %s` — and that cannot be read: %v\n"+
				"This is where the frozen bootstrap must NOT be substituted: in a shallow checkout this failure is\n"+
				"character for character what an absent tag produces, so falling back on it would diff this release\n"+
				"against %s and report the difference as if it were this release's.\n"+
				"Fetch the history — `git fetch --tags --force --unshallow` — or check out with fetch-depth: 0.",
				src.Release, src.Rev, err, gateBaselineRelease)
		}
		return ""
	default:
		return fmt.Sprintf("the previous release %s resolved to the unknown baseline kind %q", src.Release, src.Kind)
	}
}

// ---------------------------------------------------------------------
// reading the artifact, and reading v0.5.0
// ---------------------------------------------------------------------

// gateBaselineDocument reads the committed artifact and decodes it into the SAME
// struct surface.json is emitted from. Decoding is strict: an unknown key is an
// error, because a document that is not a surface document cannot be diffed
// against one.
func gateBaselineDocument(t *testing.T, root string) (surfaceDoc, []byte) {
	t.Helper()
	raw := []byte(gateReadRepoFile(t, root, gateBaselineBootstrapFile))
	var doc surfaceDoc
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		t.Fatalf("%s does not decode as a surface document: %v\n"+
			"It is read by every consumer as the same shape surface.json has; a key surfaceDoc does not\n"+
			"declare would appear in the very first release delta as an addition no code change caused.\n%s",
			gateBaselineBootstrapFile, err, gateBaselineRecovery)
	}
	return doc, raw
}

// gateBaselineCheckout materialises the FROZEN COMMIT's tree into a temporary
// directory and returns its root.
//
// It reads the object database (`git archive`) rather than creating a worktree:
// a worktree is a write to the repository's git dir, and a test that leaves
// metadata behind when it is interrupted is not the read-only shape the rest of
// this package's gate helpers keep to. It extracts by COMMIT SHA and never by
// tag name, so re-pointing the tag cannot change what these checks read.
//
// A tree it cannot materialise is a FAILURE and never an empty directory: every
// comparison below would otherwise pass over nothing, which is the shape of
// green this whole file exists to prevent.
//
// The extraction itself lives in gateBaselineExtractTar, over an io.Reader, so
// that every refusal it makes can be reached by a forged stream. Written inline
// against `git archive` they could not be: v0.5.0's tree holds no symlink
// escape and no device node, so the branches that refuse them would ship having
// never once executed, which is the same thing as not knowing whether they
// work.
func gateBaselineCheckout(t *testing.T) string {
	t.Helper()
	root := surfaceRepoRoot(t)
	dir := t.TempDir()

	cmd := exec.Command("git", "archive", "--format=tar", gateBaselineCommit)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("git archive %s: %v", gateBaselineCommit, err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("git archive %s: %v", gateBaselineCommit, err)
	}

	_, extractErr := gateBaselineExtractTar(dir, stdout)
	// The stream is drained whatever happened: a reader that stops early leaves
	// `git archive` blocked on a full pipe, and Wait would never return.
	_, _ = io.Copy(io.Discard, stdout)

	// The archive command's own failure is reported FIRST and the extraction's
	// second, because a command that never produced a stream produces an empty
	// one, and "extracted no files" would be a true statement about the wrong
	// cause.
	if waitErr := cmd.Wait(); waitErr != nil {
		t.Fatalf("git archive %s failed: %v\n%s\n\n"+
			"The %s tree could not be materialised, so NOTHING WAS COMPARED against the frozen baseline.\n"+
			"This is a failure and not a skip. A shallow clone is the usual cause — fetch the history:\n"+
			"    git fetch --tags --force --unshallow\n"+
			"In CI, check out with `fetch-depth: 0`, which .github/workflows/ci.yml already does.",
			gateBaselineCommit, waitErr, stderr.String(), gateBaselineRelease)
	}
	if extractErr != nil {
		t.Fatalf("the %s tree could not be reproduced from its archive: %v\n\n"+
			"Nothing was compared against the frozen baseline, and this is a failure rather than a skip.",
			gateBaselineRelease, extractErr)
	}
	return dir
}

// gateBaselineExtractTar writes a tar stream's entries under dir and returns how
// many regular files it wrote. Every refusal is a returned error and none is a
// silent skip: a skipped entry means every fingerprint taken over the result is
// computed across a narrower tree than the release actually carried, and the
// disagreement is then reported as a corrupt baseline rather than as the
// extraction bug it is.
//
// It takes an io.Reader rather than reading `git archive` itself so that a test
// can hand it the streams v0.5.0's tree cannot produce — an entry that escapes
// the directory, a device node, an archive with no entries at all.
func gateBaselineExtractTar(dir string, r io.Reader) (int, error) {
	files := 0
	reader := tar.NewReader(r)
	for {
		header, readErr := reader.Next()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return files, fmt.Errorf("read the archive: %w", readErr)
		}
		// `git archive` opens the stream with a pax global header carrying the
		// commit id. It is archive metadata rather than a tree entry, and it is
		// the ONE thing skipped here — everything else the stream can hold is
		// either reproduced below or refused. It is also not COUNTED: a stream
		// carrying nothing but this header has materialised no tree at all, and
		// the count below is what notices.
		if header.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		target := filepath.Join(dir, filepath.FromSlash(header.Name))
		if !strings.HasPrefix(target, dir+string(os.PathSeparator)) {
			return files, fmt.Errorf("the archive names %q, which escapes the extraction directory", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if mkErr := os.MkdirAll(target, 0o755); mkErr != nil {
				return files, fmt.Errorf("extract %s: %w", header.Name, mkErr)
			}
		case tar.TypeReg:
			if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
				return files, fmt.Errorf("extract %s: %w", header.Name, mkErr)
			}
			body, readAllErr := io.ReadAll(reader)
			if readAllErr != nil {
				return files, fmt.Errorf("extract %s: %w", header.Name, readAllErr)
			}
			if writeErr := os.WriteFile(target, body, 0o644); writeErr != nil {
				return files, fmt.Errorf("extract %s: %w", header.Name, writeErr)
			}
			files++
		case tar.TypeSymlink:
			if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
				return files, fmt.Errorf("extract %s: %w", header.Name, mkErr)
			}
			if linkErr := os.Symlink(header.Linkname, target); linkErr != nil {
				return files, fmt.Errorf("extract the symlink %s: %w", header.Name, linkErr)
			}
		default:
			// Skipping an entry type would mean recomputing a fingerprint over a
			// narrower tree than the release actually carried, and reporting the
			// disagreement as a corrupt baseline.
			return files, fmt.Errorf("the archive carries %q as tar type %q, which this extraction does not reproduce; "+
				"a fingerprint taken over a tree missing it would be measuring something else",
				header.Name, string(header.Typeflag))
		}
	}
	if files == 0 {
		return 0, errors.New("the archive extracted no files, so every comparison taken over the result would pass over nothing")
	}
	return files, nil
}

// TestGateBaselineExtractionRefusesWhatItCannotReproduce reaches the refusals in
// gateBaselineExtractTar that v0.5.0's own tree cannot reach.
//
// That tree carries no symlink escape, no device node, no hardlink and no fifo,
// and `git archive` on a real commit never emits an entry-free stream. So those
// branches would ship having never executed once — and a branch nobody has run
// is a branch nobody knows the behaviour of. Each is exercised here on a forged
// stream instead, which is the only way to distinguish "it refuses" from "it
// silently drops the entry and every fingerprint below is taken over a tree the
// release did not have".
func TestGateBaselineExtractionRefusesWhatItCannotReproduce(t *testing.T) {
	// paxGlobal is the header `git archive` opens every real stream with.
	paxGlobal := &tar.Header{
		Typeflag:   tar.TypeXGlobalHeader,
		Name:       "pax_global_header",
		Format:     tar.FormatPAX,
		PAXRecords: map[string]string{"comment": gateBaselineCommit},
	}

	for _, c := range []struct {
		name    string
		entries []*tar.Header
		want    string
	}{
		{
			name:    "an entry-free stream",
			entries: nil,
			want:    "extracted no files",
		},
		{
			// The pax header is skipped, and this is what proves the skip does
			// not also count: were it counted, a stream that materialised
			// nothing would report one "file" and every recomputation would run
			// against an empty directory.
			name:    "a stream carrying only the pax global header",
			entries: []*tar.Header{paxGlobal},
			want:    "extracted no files",
		},
		{
			// A directory of directories is not a tree of bytes. Fingerprints
			// are taken over file contents, so this too would pass over nothing.
			name:    "a stream carrying only directories",
			entries: []*tar.Header{{Typeflag: tar.TypeDir, Name: "internal/", Mode: 0o755}},
			want:    "extracted no files",
		},
		{
			name:    "an entry that climbs out of the extraction directory",
			entries: []*tar.Header{{Typeflag: tar.TypeReg, Name: "../escaped.txt", Mode: 0o644, Size: 0}},
			want:    "escapes the extraction directory",
		},
		{
			name:    "an entry that climbs out through a subdirectory",
			entries: []*tar.Header{{Typeflag: tar.TypeReg, Name: "internal/../../escaped.txt", Mode: 0o644, Size: 0}},
			want:    "escapes the extraction directory",
		},
		{
			name:    "a fifo",
			entries: []*tar.Header{{Typeflag: tar.TypeFifo, Name: "internal/pipe", Mode: 0o644}},
			want:    "does not reproduce",
		},
		{
			name:    "a character device",
			entries: []*tar.Header{{Typeflag: tar.TypeChar, Name: "internal/tty", Mode: 0o644, Devmajor: 1, Devminor: 3}},
			want:    "does not reproduce",
		},
		{
			name:    "a hardlink",
			entries: []*tar.Header{{Typeflag: tar.TypeLink, Name: "internal/b.go", Linkname: "internal/a.go", Mode: 0o644}},
			want:    "does not reproduce",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			files, err := gateBaselineExtractTar(t.TempDir(), gateBaselineTarStream(t, c.entries, nil))
			if err == nil {
				t.Fatalf("the extraction accepted %s and reported %d file(s). Accepting it means the frozen tree is "+
					"reproduced WRONG and every fingerprint below is taken over something other than what %s shipped — "+
					"reported, when it finally disagrees, as a corrupt baseline.", c.name, files, gateBaselineRelease)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("extracting %s failed with %q, which does not name the reason (%q). The message is what a "+
					"maintainer acts on.", c.name, err, c.want)
			}
		})
	}

	// And the other half: a stream it CAN reproduce is reproduced, byte for
	// byte. Without this the refusals above would be satisfied by an extraction
	// that refused everything.
	t.Run("a stream it can reproduce", func(t *testing.T) {
		dir := t.TempDir()
		bodies := map[string]string{
			"internal/lint/lint.go": "package lint\n",
			"README.md":             "# dossierx\n",
		}
		files, err := gateBaselineExtractTar(dir, gateBaselineTarStream(t, []*tar.Header{
			paxGlobal,
			{Typeflag: tar.TypeDir, Name: "internal/", Mode: 0o755},
			{Typeflag: tar.TypeDir, Name: "internal/lint/", Mode: 0o755},
			{Typeflag: tar.TypeReg, Name: "internal/lint/lint.go", Mode: 0o644, Size: int64(len(bodies["internal/lint/lint.go"]))},
			{Typeflag: tar.TypeReg, Name: "README.md", Mode: 0o644, Size: int64(len(bodies["README.md"]))},
			{Typeflag: tar.TypeSymlink, Name: "internal/lint/alias.go", Linkname: "lint.go", Mode: 0o777},
		}, bodies))
		if err != nil {
			t.Fatalf("a stream of exactly what a git tree holds was refused: %v", err)
		}
		if files != 2 {
			t.Errorf("the extraction reported %d regular files over a stream carrying 2; the count is what the "+
				"anti-vacuity refusal above is made of, so a count that does not track the stream makes it meaningless", files)
		}
		for rel, want := range bodies {
			got, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
			if readErr != nil {
				t.Fatalf("read the extracted %s: %v", rel, readErr)
			}
			if string(got) != want {
				t.Errorf("the extracted %s reads %q, not the %q the stream carried; a fingerprint over this tree "+
					"measures the extraction rather than the release", rel, got, want)
			}
		}
		if link, readErr := os.Readlink(filepath.Join(dir, "internal", "lint", "alias.go")); readErr != nil || link != "lint.go" {
			t.Errorf("the symlink came out as %q (%v), not the link the stream carried", link, readErr)
		}
	})
}

// gateBaselineTarStream builds a tar stream in memory. bodies supplies the
// contents of regular files, keyed by header name.
func gateBaselineTarStream(t *testing.T, headers []*tar.Header, bodies map[string]string) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	for _, header := range headers {
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("write the %s header: %v", header.Name, err)
		}
		if body, ok := bodies[header.Name]; ok {
			if _, err := io.WriteString(writer, body); err != nil {
				t.Fatalf("write %s: %v", header.Name, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close the forged archive: %v", err)
	}
	return bytes.NewReader(buf.Bytes())
}

// gateBaselineSweptPins re-runs surfaceVersionPins' sweep over an EXTRACTED tree
// rather than a repository. It is `--no-index` because the extraction has no git
// dir, and `--no-exclude-standard` so a .gitignore rule in the old tree cannot
// hide a file that release actually tracked — the emitter's own sweep runs over
// tracked files and pays no attention to .gitignore, and the two must measure
// the same set.
//
// The exclusions are surfaceVersionPins' four, spelled here rather than shared,
// for the reason surfaceHTTPRoutes spells "HandleFunc" and "Handle" in two
// places: a shared list would be one edit away from narrowing both halves at
// once, and this half exists to disagree with the other.
// Every refusal is a returned error rather than a t.Fatalf, so that the empty
// answer — the one that would make the version_pins comparison pass over
// nothing — can be produced on purpose by a test and is not a branch taken on
// trust.
func gateBaselineSweptPins(dir string) ([]surfacePin, error) {
	cmd := exec.Command("git", "grep", "--no-index", "--no-exclude-standard", "-nE", surfacePinSweep, "--",
		".", ":!"+surfaceFileName, ":!CHANGELOG.md", ":!docs/RELEASING.md", ":!"+gateBaselineBootstrapFile)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// git grep exits 1 for "no matches", which is why this cannot be read as
		// a clean empty answer: a release that pins itself nowhere is not a state
		// this project has been in — README and the CI template both carry one —
		// so it is reported rather than compared against.
		return nil, fmt.Errorf("the tree at %s swept no version pins (or git grep could not run): %w", dir, err)
	}

	var pins []surfacePin
	seen := map[surfacePin]bool{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("unparseable git grep line %q", line)
		}
		matches := surfacePinRE.FindAllString(parts[2], -1)
		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: the sweep matched %q but no pin could be extracted from it; the extractor and the sweep disagree", parts[0], parts[2])
		}
		for _, match := range matches {
			version := surfacePinVersionRE.FindString(match)
			if version == "" {
				return nil, fmt.Errorf("%s: pin %q carries no version", parts[0], match)
			}
			pin := surfacePin{File: filepath.ToSlash(parts[0]), Pin: match, Version: version}
			if seen[pin] {
				continue
			}
			seen[pin] = true
			pins = append(pins, pin)
		}
	}
	if len(pins) == 0 {
		return nil, fmt.Errorf("the sweep of %s produced no pins", dir)
	}
	sort.Slice(pins, func(i, j int) bool {
		if pins[i].File != pins[j].File {
			return pins[i].File < pins[j].File
		}
		return pins[i].Pin < pins[j].Pin
	})
	return pins, nil
}

// TestGateBaselineSweepOfTheFrozenTreeRefusesAnAnswerItCannotStandBehind reaches
// gateBaselineSweptPins' refusals, which the frozen tree cannot reach: v0.5.0
// pins itself in four places and every one of them parses.
//
// The empty answer is the one that matters. version_pins is compared for exact
// equality against whatever this returns, so a sweep that quietly produced
// nothing would turn that comparison into a comparison of one empty list with
// another — green, and measuring nothing, over the exact field failure scenario
// 3 corrupts.
func TestGateBaselineSweepOfTheFrozenTreeRefusesAnAnswerItCannotStandBehind(t *testing.T) {
	write := func(t *testing.T, files map[string]string) string {
		t.Helper()
		dir := t.TempDir()
		for rel, body := range files {
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
				t.Fatalf("write %s: %v", rel, err)
			}
		}
		return dir
	}
	// Assembled rather than written out, for the reason
	// TestGateBaselineIsExcludedFromTheVersionPinSweep gives: a literal pin token
	// in this file would be swept as a live pin site of the repository itself.
	pin := "github.com/BarterX-Tech/dossierx/cmd/dossierx@" + gateBaselineRelease

	t.Run("a tree that pins nothing", func(t *testing.T) {
		_, err := gateBaselineSweptPins(write(t, map[string]string{"README.md": "# no pins here\n"}))
		if err == nil {
			t.Fatalf("a tree with no pin at all swept clean. An empty sweep compares equal to an empty version_pins " +
				"field, so the one field the release checklist can corrupt would be checked by an equality between two " +
				"empty lists")
		}
	})

	t.Run("a line the sweep matches but the extractor cannot parse", func(t *testing.T) {
		// The sweep expression is satisfied by the module name followed by an
		// at-sign and a "v"; the EXTRACTOR additionally wants three numbers. A
		// line that satisfies one and not the other must be reported, because
		// dropping it makes the recomputed pins a subset of the tree's real ones
		// and the comparison then reports the BASELINE as wrong.
		//
		// It is assembled from pieces, like every other pin-shaped string in this
		// file, and here the reason is sharper than usual: written out it would
		// be swept as a live pin site of THIS repository, and since no pin can be
		// extracted from it surfaceVersionPins would stop returning pins and
		// start returning an error — surface.json would no longer regenerate at
		// all.
		unparseable := "go install .../dossierx" + "@" + "vNEXT\n"
		_, err := gateBaselineSweptPins(write(t, map[string]string{"README.md": unparseable}))
		if err == nil {
			t.Fatal("a line the sweep matched and the extractor could not parse was dropped rather than reported")
		}
		if !strings.Contains(err.Error(), "no pin could be extracted") {
			t.Errorf("the unparseable line failed with %q, which does not say the extractor and the sweep disagree", err)
		}
	})

	t.Run("the frozen record is not swept as one of its own pin sites", func(t *testing.T) {
		pins, err := gateBaselineSweptPins(write(t, map[string]string{
			"README.md":               pin + "\n",
			surfaceFileName:           pin + "\n",
			"CHANGELOG.md":            pin + "\n",
			gateBaselineBootstrapFile: pin + "\n",
			"scripts-ci-dossierx.yml": pin + "\n",
		}))
		if err != nil {
			t.Fatalf("sweep a tree with pins in it: %v", err)
		}
		want := []string{"README.md", "scripts-ci-dossierx.yml"}
		var got []string
		for _, p := range pins {
			got = append(got, p.File)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("the sweep reported %v; it must report %v.\n"+
				"This half of the sweep spells its own exclusions rather than sharing surfaceVersionPins' list, so that\n"+
				"one edit cannot narrow both halves at once — and that only works if this half is checked separately.", got, want)
		}
	})
}

// ---------------------------------------------------------------------
// the artifact is v0.5.0's tree, not this one's
// ---------------------------------------------------------------------

// gateBaselineRecomputedFields and gateBaselineResidueFields partition
// surfaceDoc's keys into the ones recomputed below and the ones that are not.
// Together they are the coverage claim this file makes, and
// TestGateBaselineFieldsAreRecomputedOrNamedAsResidue is what stops the claim
// narrowing in silence when a field is added to the document.
var (
	// gateBaselineLinkTimeFields are the six that are not functions of a
	// checkout the way the ones above are: they come out of a BUILD of the
	// frozen tree. It is ONE list, feeding both the coverage claim here and the
	// comparisons in TestGateBaselineLinkTimeNamesAreTheFrozenTreesOwnBuild, in
	// that order — so a field dropped from the comparison cannot leave this
	// claim reading as though it were still covered, which is the only way the
	// hole these six had could quietly re-open.
	gateBaselineLinkTimeFields = []string{"commands", "root_flags", "retired", "lint_rules", "skills", "envelope"}

	gateBaselineRecomputedFields = append([]string{
		"behaviour_fingerprint",
		"error_codes",
		"http_routes",
		"markdown_constructs",
		"render_fingerprint",
		"version_pins",
	}, gateBaselineLinkTimeFields...)

	// gateBaselineResidueFields is what is left: one field, and it is arithmetic
	// rather than an extraction. Every number in `counts` counts a list this
	// file now recomputes, and TestGateBaselineCountsAgreeWithItsOwnLists holds
	// all five against the lists they count — so a count edited to match a
	// hand-edited list is caught by the list's own comparison instead.
	gateBaselineResidueFields = []string{
		"counts",
	}
)

// TestGateBaselineIsTheFrozenTreesInventory is the check the whole artifact
// hangs on: every field of surface.baseline.json that is a pure function of a
// checkout must be what HEAD's emitter computes over THE V0.5.0 COMMIT'S BYTES.
//
// The anchor is what matters. Today v0.5.0's shipped sources and HEAD's are the
// same bytes, so this passes over an honest bootstrap and over `cp surface.json
// surface.baseline.json` alike, and no check can separate two identical files.
// But it is reading the frozen commit out of the object database, not this
// working tree — so a copy of a LATER surface.json fails here, a re-derivation
// against a newer tree fails here, and a hand-edited fingerprint fails here.
// TestGateBaselineIsAnchoredToTheFrozenTreeAndNotToHEAD proves the anchor is
// real rather than incidental.
//
// Failure scenario this closes, concretely: someone broadens a rule in
// internal/lint so a corpus that passed `dossierx check` at v0.5.0 now fails —
// this project's canonical silent-behaviour example. That change moves NO name,
// NO count, and nothing in the render tree; behaviour_fingerprint is the only
// field in the document it touches. A baseline copied from surface.json after
// that change reads the new fingerprint, the first gated release's delta over
// internal/lint comes out empty, no SILENT-BEHAVIOUR is classified, and the
// release ships with no CHANGELOG line while other people's merge gates start
// refusing their locked corpora.
func TestGateBaselineIsTheFrozenTreesInventory(t *testing.T) {
	root := surfaceRepoRoot(t)
	doc, _ := gateBaselineDocument(t, root)
	frozen := gateBaselineCheckout(t)

	t.Run("render_fingerprint", func(t *testing.T) {
		files, err := renderFingerprintFiles(frozen)
		if err != nil {
			t.Fatalf("resolve %s's render fingerprint inputs: %v", gateBaselineRelease, err)
		}
		want, err := hashRepoFiles(frozen, files)
		if err != nil {
			t.Fatalf("hash %s's render fingerprint inputs: %v", gateBaselineRelease, err)
		}
		if doc.RenderFingerprint != want {
			t.Errorf("render_fingerprint is %s, but %s's %d render sources and templates hash to %s.\n"+
				"The committed baseline is not an inventory of the tree %s shipped.\n%s",
				doc.RenderFingerprint, gateBaselineRelease, len(files), want, gateBaselineRelease, gateBaselineRecovery)
		}
	})

	t.Run("behaviour_fingerprint", func(t *testing.T) {
		want, err := surfaceBehaviourFingerprint(frozen)
		if err != nil {
			t.Fatalf("compute %s's behaviour fingerprint: %v", gateBaselineRelease, err)
		}
		gateBaselineReport(t, gateBaselineFingerprintFaults(doc.BehaviourFingerprint, want))
	})

	t.Run("error_codes", func(t *testing.T) {
		want, err := surfaceErrorCodes(frozen)
		if err != nil {
			t.Fatalf("extract %s's error codes: %v", gateBaselineRelease, err)
		}
		gateBaselineReport(t, gateBaselineStringFieldFaults("error_codes", doc.ErrorCodes, want))
	})

	t.Run("http_routes", func(t *testing.T) {
		want, err := surfaceHTTPRoutes(frozen)
		if err != nil {
			t.Fatalf("extract %s's http routes: %v", gateBaselineRelease, err)
		}
		gateBaselineReport(t, gateBaselineStringFieldFaults("http_routes", doc.HTTPRoutes, want))
	})

	t.Run("markdown_constructs", func(t *testing.T) {
		want, err := surfaceMarkdownConstructs(frozen)
		if err != nil {
			t.Fatalf("extract %s's markdown constructs: %v", gateBaselineRelease, err)
		}
		gateBaselineReport(t, gateBaselineStringFieldFaults("markdown_constructs", doc.MarkdownConstructs, want))
	})

	// version_pins is the field failure scenario 3 corrupts. docs/RELEASING.md's
	// pin sweep will list this artifact as a site holding a stale pin from the
	// next release onward unless it is excluded, and a maintainer doing what the
	// checklist says rewrites v0.5.0's frozen record to say v0.6.0. Recomputing
	// it from the frozen tree makes that edit red instead of silent.
	t.Run("version_pins", func(t *testing.T) {
		want, err := gateBaselineSweptPins(frozen)
		if err != nil {
			t.Fatalf("sweep %s's version pins: %v\n"+
				"The sweep refusing is a failed comparison and not an empty one: an empty result compares equal to an\n"+
				"empty version_pins field, and this is the field the release checklist can corrupt.", gateBaselineRelease, err)
		}
		gateBaselineReport(t, gateBaselinePinFaults(doc.VersionPins, want))
	})
}

// ---------------------------------------------------------------------
// the link-time names: what v0.5.0 says when it is built
// ---------------------------------------------------------------------

// gateBaselineEmitterSources are the emitter files copied into the frozen tree,
// exactly the pair the recovery text tells a maintainer to copy. They are HEAD's
// — the whole definition of this artifact is "what HEAD's emitter says about
// v0.5.0's tree" — and they are copied rather than imported because v0.5.0
// predates both files and a package cannot import a test.
var gateBaselineEmitterSources = []string{"surface_test.go", "surface_meta_test.go"}

const (
	// gateBaselineLinkTimeShimFile is written into the frozen tree's
	// cmd/dossierx beside the copied emitter. It exists only inside a temporary
	// checkout and is never part of this repository's own package.
	gateBaselineLinkTimeShimFile = "gate_baseline_linktime_test.go"

	// gateBaselineLinkTimeTest is the shim's test name, matched exactly so the
	// nested run cannot drag in the frozen tree's own suite.
	gateBaselineLinkTimeTest = "TestGateBaselineEmitLinkTime"

	// gateBaselineLinkTimeOutEnv names the file the shim writes. It is passed
	// rather than defaulted so that a shim which never ran leaves no document to
	// read, instead of leaving a stale one that would be compared as if it were
	// this run's answer.
	gateBaselineLinkTimeOutEnv = "DOSSIERX_GATE_BASELINE_LINKTIME_OUT"
)

// gateBaselineLinkTimeShim is the program that answers "what were v0.5.0's
// link-time names". It runs INSIDE the frozen tree, linked against that
// release's own packages, and reports the six fields as a surface document —
// the same struct, marshalled the same way, so the comparison is between two
// documents of one shape rather than between a document and an ad-hoc dump.
//
// Every value in it comes from the copied emitter's own helpers.
// Re-implementing the extractions here would measure this file's idea of what a
// command tree is rather than the emitter's, and the baseline would then be
// checked against something no release ever produced.
const gateBaselineLinkTimeShim = `package main

import (
	"encoding/json"
	"os"
	"testing"
)

// Written by cmd/dossierx/gate_baseline_test.go into a materialised v0.5.0
// tree. It is not part of any committed package.
func ` + gateBaselineLinkTimeTest + `(t *testing.T) {
	out := os.Getenv("` + gateBaselineLinkTimeOutEnv + `")
	if out == "" {
		t.Fatal("` + gateBaselineLinkTimeOutEnv + ` is unset, so this run would report its answer nowhere")
	}
	root := surfaceRepoRoot(t)
	codes, err := surfaceErrorCodes(root)
	if err != nil {
		t.Fatalf("extract error codes: %v", err)
	}
	rootCmd := newRootCmd()
	commands, retired, _ := surfaceCommandTree(rootCmd)
	raw, err := json.MarshalIndent(surfaceDoc{
		Commands:  commands,
		RootFlags: surfaceCommandFlags(rootCmd),
		Retired:   retired,
		LintRules: surfaceLintRules(),
		Skills:    surfaceSkills(),
		Envelope:  surfaceEnvelopeContract(codes),
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal the link-time fields: %v", err)
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", out, err)
	}
}
`

// gateBaselineBuildLinkTime builds the frozen tree and returns what it says its
// own link-time surface is.
//
// Several tests here shell out to the toolchain (tests/cli_test.go and
// tests/portability_test.go build the binary); this is the only one that builds
// a tree OTHER than this one. It is here because there is no cheaper honest
// answer: these six fields are not in the checkout, they are in the linked
// program, and a check that cannot run is a failure rather than a pass — so the
// alternative to building was leaving them unchecked, which is what let a
// hand-edited name through.
//
// It runs in the ALREADY MATERIALISED tree (an extraction of the frozen commit
// under a temporary directory), never in a git worktree: a worktree is a write
// to the repository's git dir and leaves metadata behind when a test is
// interrupted. That is also why nothing here has to be cleaned up — the
// directory is the test's own and goes with it.
//
// Every way it can fail to produce an answer is a t.Fatalf naming what was not
// compared. There is no path through this function that returns an empty
// document and lets the comparisons below run over nothing.
func gateBaselineBuildLinkTime(t *testing.T, frozen string) surfaceDoc {
	t.Helper()

	root := surfaceRepoRoot(t)

	// The build has to happen in the FROZEN tree, and on this tree that is not
	// self-evident from its result: HEAD's link-time surface and v0.5.0's are
	// the same names today, so a build pointed at this working directory would
	// agree with the baseline exactly and the check would be the current CLI
	// compared with itself. The same premise the fingerprints are anchored by
	// settles it — v0.5.0 predates the emitter and carries no surface.json,
	// every later tree carries one — so it is asked here rather than assumed.
	head, err := gateResolve(root, "HEAD")
	if err != nil {
		t.Fatalf("resolve HEAD: %v", err)
	}
	_, frozenStatErr := os.Stat(filepath.Join(frozen, surfaceFileName))
	_, workingStatErr := os.Stat(filepath.Join(root, surfaceFileName))
	if fault := gateBaselineAnchorFault(head, frozenStatErr == nil, workingStatErr == nil); fault != "" {
		t.Fatalf("the link-time build would not be a build of %s: %s", gateBaselineRelease, fault)
	}

	pkg := filepath.Join(frozen, "cmd", "dossierx")
	for _, name := range gateBaselineEmitterSources {
		body, err := os.ReadFile(filepath.Join(root, "cmd", "dossierx", name))
		if err != nil {
			t.Fatalf("read HEAD's %s, which is the emitter this baseline is defined by: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(pkg, name), body, 0o644); err != nil {
			t.Fatalf("copy %s into the %s tree: %v", name, gateBaselineRelease, err)
		}
	}
	if err := os.WriteFile(filepath.Join(pkg, gateBaselineLinkTimeShimFile), []byte(gateBaselineLinkTimeShim), 0o644); err != nil {
		t.Fatalf("write the link-time shim into the %s tree: %v", gateBaselineRelease, err)
	}

	out := filepath.Join(t.TempDir(), "link-time.json")
	cmd := exec.Command("go", "test", "./cmd/dossierx", "-run", "^"+gateBaselineLinkTimeTest+"$", "-count=1")
	cmd.Dir = frozen
	cmd.Env = append(os.Environ(), gateBaselineLinkTimeOutEnv+"="+out)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s could not be BUILT AND RUN, so its commands, root_flags, retired, lint_rules, skills and\n"+
			"envelope were compared against nothing: %v\n%s\n"+
			"This is a failure and not a skip. Two causes are worth separating before reaching for the recovery:\n"+
			"the toolchain or the module cache cannot build that release offline, which is an environment to fix;\n"+
			"or HEAD's emitter source no longer COMPILES inside %s, which breaks the one method that can ever\n"+
			"re-manufacture this artifact and is a finding about the emitter rather than about the baseline.",
			gateBaselineRelease, err, combined, gateBaselineRelease)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the build of %s reported success and wrote no answer: %v\n"+
			"A missing document must not be read as a document that agrees.", gateBaselineRelease, err)
	}
	var built surfaceDoc
	if err := json.Unmarshal(raw, &built); err != nil {
		t.Fatalf("the link-time answer from %s does not decode as a surface document: %v", gateBaselineRelease, err)
	}
	return built
}

// TestGateBaselineLinkTimeNamesAreTheFrozenTreesOwnBuild is the check the six
// link-time fields had none of.
//
// What it closes, concretely: a name in commands, root_flags, retired,
// lint_rules, skills or envelope edited by hand — a lint rule renamed as it is
// broadened, a flag quietly dropped, an exit code moved — used to pass every
// check in this file. The fingerprints do not move, because they hash the source
// bytes of the FROZEN tree and the edit is in the document. counts does not
// move, because renaming an entry does not change how many there are. The
// canonical-form check does not move, because the file still round-trips. So the
// first gated release's delta over the CLI surface reads empty, and the release
// that renamed the rule ships with no line about it — the same shape as the
// silent lint broadening this artifact exists to make visible, one field over.
//
// The comparison is against a build of v0.5.0 and not against HEAD's own
// newRootCmd(). Today those agree, exactly as the fingerprints do; the point is
// that they stop agreeing the moment HEAD adds a command, and on that day this
// check must still be measuring what the release shipped.
func TestGateBaselineLinkTimeNamesAreTheFrozenTreesOwnBuild(t *testing.T) {
	root := surfaceRepoRoot(t)
	doc, _ := gateBaselineDocument(t, root)
	built := gateBaselineBuildLinkTime(t, gateBaselineCheckout(t))

	var compared []string
	for _, c := range []struct {
		field      string
		got, want  any
		recomputed int
	}{
		{"commands", doc.Commands, built.Commands, len(built.Commands)},
		{"root_flags", doc.RootFlags, built.RootFlags, len(built.RootFlags)},
		{"retired", doc.Retired, built.Retired, len(built.Retired)},
		{"lint_rules", doc.LintRules, built.LintRules, len(built.LintRules)},
		{"skills", doc.Skills, built.Skills, len(built.Skills)},
		// The envelope's own key set is what "the build reported an envelope at
		// all" means; a contract with no keys is the empty answer for this field.
		{"envelope", doc.Envelope, built.Envelope, len(built.Envelope.Keys)},
	} {
		compared = append(compared, c.field)
		t.Run(c.field, func(t *testing.T) {
			gateBaselineReport(t, gateBaselineBuiltFieldFaults(c.field,
				gateBaselineFieldJSON(t, c.field, c.got), gateBaselineFieldJSON(t, c.field, c.want), c.recomputed))
		})
	}
	if !reflect.DeepEqual(compared, gateBaselineLinkTimeFields) {
		t.Errorf("this test compares %v, and the document's coverage claim says the link-time fields are %v.\n"+
			"A field the claim calls recomputed and this loop does not compare is checked by nothing at all — which is\n"+
			"the state all six were in, and the state that let a renamed lint rule through.", compared, gateBaselineLinkTimeFields)
	}
}

// gateBaselineFieldJSON renders one field the way the document writes it, so the
// comparison and the diff below are over the bytes a reader would see.
func gateBaselineFieldJSON(t *testing.T, field string, value any) string {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s for comparison: %v", field, err)
	}
	return string(raw)
}

// ---------------------------------------------------------------------
// the comparisons, as functions of their inputs
// ---------------------------------------------------------------------
//
// Each comparison below returns its complaints rather than calling t.Errorf, and
// the reason is the anti-vacuity guard each one opens with. "The recomputation
// came out empty, so this comparison would pass over nothing" is the single most
// important line in the file — it is what stops an extractor that silently
// stopped working from turning every check here green — and written inline
// against the real v0.5.0 tree it could never execute, because v0.5.0 really
// does have 44 error codes and 22 behaviour packages. Written as a function of
// its inputs it can be handed the empty set on purpose, so the guard is a branch
// that has run rather than a branch that looks right.

// gateBaselineReport turns a comparison's complaints into test failures. Each is
// reported separately: they are separate accusations about the document, and a
// maintainer needs all of them, not the first.
//
// It takes an interface rather than *testing.T for one reason — so that
// TestGateBaselineReportedFaultsActuallyFailTheTest can watch it, and a version
// of this function that computed every fault and reported none cannot pass. That
// version would be invisible otherwise: on a correct artifact there are no
// faults to report, so the wiring between the comparisons and the test result is
// exercised by nothing on a green day.
type gateBaselineReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

func gateBaselineReport(t gateBaselineReporter, faults []string) {
	t.Helper()
	for _, fault := range faults {
		t.Errorf("%s\n%s", fault, gateBaselineRecovery)
	}
}

// gateBaselineFakeReporter records what gateBaselineReport did instead of failing.
type gateBaselineFakeReporter struct{ reported []string }

func (f *gateBaselineFakeReporter) Helper() {}

func (f *gateBaselineFakeReporter) Errorf(format string, args ...any) {
	f.reported = append(f.reported, fmt.Sprintf(format, args...))
}

// TestGateBaselineReportedFaultsActuallyFailTheTest holds the wiring between the
// comparisons and the verdict.
//
// Everything above computes a list of complaints; this is what turns them into a
// failure. On a correct artifact that list is always empty, so nothing else in
// this file would notice a report that computed the faults and dropped them —
// every check would print PASS over a baseline it had just found wrong, which is
// the "swallowed error read as a pass over zero assertions" this package already
// has words for.
func TestGateBaselineReportedFaultsActuallyFailTheTest(t *testing.T) {
	fake := &gateBaselineFakeReporter{}
	gateBaselineReport(fake, []string{"the first accusation", "the second accusation"})
	if len(fake.reported) != 2 {
		t.Fatalf("two faults were reported as %d failure(s): %v.\n"+
			"A comparison that finds the baseline wrong and reports nothing is worse than no comparison: the gate\n"+
			"prints a pass over a document it has already found to be an inventory of the wrong tree.", len(fake.reported), fake.reported)
	}
	for i, want := range []string{"the first accusation", "the second accusation"} {
		if !strings.Contains(fake.reported[i], want) {
			t.Errorf("fault %d was reported as %q, which does not carry the complaint %q", i, fake.reported[i], want)
		}
		if !strings.Contains(fake.reported[i], "not regenerable from this tree") {
			t.Errorf("fault %d was reported without the recovery. A maintainer who reads only the failure must not\n"+
				"reach for `-regenerate-goldens` against the current tree — that is failure scenario 4, and the\n"+
				"recovery text is the only place this repository says so at the moment it matters.", i)
		}
	}
	if fake := (&gateBaselineFakeReporter{}); func() bool {
		gateBaselineReport(fake, nil)
		return len(fake.reported) != 0
	}() {
		t.Errorf("a comparison that found nothing wrong reported %v", fake.reported)
	}
}

// gateBaselineStringFieldFaults compares one of the document's sorted name lists
// against what the frozen tree yields. The two directions are reported
// separately: a name the baseline is missing and a name it invents are different
// accusations about which tree it is an inventory of.
func gateBaselineStringFieldFaults(field string, got, want []string) []string {
	if len(want) == 0 {
		return []string{fmt.Sprintf("%s's %s came out of the frozen tree empty, so comparing the baseline against it would "+
			"pass over nothing. The extractor is not reading the materialised tree.", gateBaselineRelease, field)}
	}
	var faults []string
	inWant := map[string]bool{}
	for _, w := range want {
		inWant[w] = true
	}
	inGot := map[string]bool{}
	for _, g := range got {
		inGot[g] = true
	}
	for _, w := range want {
		if !inGot[w] {
			faults = append(faults, fmt.Sprintf("%s is missing %q, which %s shipped.", field, w, gateBaselineRelease))
		}
	}
	for _, g := range got {
		if !inWant[g] {
			faults = append(faults, fmt.Sprintf("%s carries %q, which %s did not ship; the baseline is an inventory of some other tree.",
				field, g, gateBaselineRelease))
		}
	}
	if len(faults) == 0 && !reflect.DeepEqual(got, want) {
		faults = append(faults, fmt.Sprintf("%s holds the right names in a different order than the emitter writes them (%v vs %v); "+
			"the document is sorted, so a re-ordering means it was edited by hand.", field, got, want))
	}
	return faults
}

// gateBaselineFingerprintFaults compares behaviour_fingerprint against the frozen
// tree's own bytes. This is the field failure scenario 1 turns on: a broadened
// lint rule moves nothing else in the document, so a package missing here is a
// package the first release delta cannot report a silent change in.
func gateBaselineFingerprintFaults(got, want map[string]string) []string {
	if len(want) == 0 {
		return []string{fmt.Sprintf("%s's behaviour fingerprint covers no packages, so this comparison would pass over nothing — "+
			"including over internal/lint, the package whose silent changes this whole field exists to catch.", gateBaselineRelease)}
	}
	var faults []string
	for _, pkg := range gateBaselineSortedKeys(want) {
		sum := want[pkg]
		have, ok := got[pkg]
		if !ok {
			faults = append(faults, fmt.Sprintf("behaviour_fingerprint has no entry for %s, which %s shipped. "+
				"A package missing from the baseline is a package the first release delta cannot report a silent change in.",
				pkg, gateBaselineRelease))
			continue
		}
		if have != sum {
			faults = append(faults, fmt.Sprintf("behaviour_fingerprint[%s] is %s, but %s's own bytes hash to %s.\n"+
				"This is the field a broadened lint rule moves and nothing else does, so a wrong value here is a\n"+
				"silent behaviour change the first gated release cannot see.", pkg, have, gateBaselineRelease, sum))
		}
	}
	for _, pkg := range gateBaselineSortedKeys(got) {
		if _, ok := want[pkg]; !ok {
			faults = append(faults, fmt.Sprintf("behaviour_fingerprint carries %s, which is not a package %s shipped. "+
				"The baseline is an inventory of some other tree.", pkg, gateBaselineRelease))
		}
	}
	return faults
}

// gateBaselinePinFaults compares version_pins against what the frozen tree
// itself pins. It is the machine half of failure scenario 3: every pin in the
// document is a historical fact about v0.5.0, correct precisely because it is
// old, and a maintainer sweeping it as a live site rewrites the only baseline the
// first gated release has.
func gateBaselinePinFaults(got, want []surfacePin) []string {
	if len(want) == 0 {
		return []string{fmt.Sprintf("%s's tree swept no version pins, so this comparison would pass over nothing. "+
			"A release that pins itself nowhere is not a state this project has been in.", gateBaselineRelease)}
	}
	if !reflect.DeepEqual(got, want) {
		return []string{fmt.Sprintf("version_pins is not what %s's own tree pins.\n  committed: %v\n  %s:  %v\n"+
			"Every pin in this document is a historical fact about %s and is correct precisely because it is\n"+
			"old. A pin here reading anything else means the frozen record was swept as if it were live.",
			gateBaselineRelease, got, gateBaselineRelease, want, gateBaselineRelease)}
	}
	return nil
}

// gateBaselineBuiltFieldFaults compares one link-time field of the document
// against what the build of the frozen tree reported for it. Both sides arrive
// as the JSON the document writes, so one function covers a list of names, a
// list of command records and the envelope contract alike.
//
// recomputed is how many entries the BUILD produced for the field. It is a
// separate argument rather than something read off want, because the empty
// answer is the one that matters and it has to be refusable without knowing
// which shape the field has: a shim that ran and reported nothing would compare
// equal to a baseline whose field was deleted, and a build that failed to link a
// command would silently narrow what the first release delta can see.
func gateBaselineBuiltFieldFaults(field, got, want string, recomputed int) []string {
	if recomputed == 0 {
		return []string{fmt.Sprintf("the build of %s reported no %s at all, so comparing the baseline against it would "+
			"pass over nothing. The shim is not reporting what that release linked.", gateBaselineRelease, field)}
	}
	if got != want {
		return []string{fmt.Sprintf("%s is not what %s reports when it is built.\n%s\n"+
			"This field is LINK-TIME: it is what the program IS once it is linked, so a name edited here moves no\n"+
			"fingerprint, no count and nothing else in the document. Every other check in this file passes over it.",
			field, gateBaselineRelease, surfaceLineDiff(got, want))}
	}
	return nil
}

func gateBaselineSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestGateBaselineComparisonsRefuseAnEmptyRecomputation is the mutation the
// frozen tree cannot supply.
//
// Every comparison above opens by refusing an empty right-hand side, and against
// the real v0.5.0 tree that branch never runs: the tree has 25 render sources, 22
// behaviour packages, 44 error codes, 14 routes, 129 markdown constructs and 4
// pins. So the guard that matters most — the one that catches an extractor which
// has quietly stopped reading the materialised tree, turning every check in this
// file into a comparison of two empty lists — would ship never having executed.
// Here each is handed the empty set on purpose.
//
// The other rows are the same functions' real work, and they are here for the
// same reason: a comparison that reports NOTHING when the document disagrees is
// the failure this whole file was written against, and it cannot be told from a
// correct one by watching it pass on a correct document.
func TestGateBaselineComparisonsRefuseAnEmptyRecomputation(t *testing.T) {
	// Each row asserts WHAT the complaint says and not merely that there was
	// one, and that is load-bearing rather than fussy. Drop the empty-input
	// guard from any of these functions and a complaint still comes out — the
	// baseline's 44 real error codes all become "names v0.5.0 did not ship" —
	// so a test that counted complaints would stay green over the exact bug it
	// was written for. Worse, that complaint accuses the ARTIFACT, and the
	// recovery this file prints beside it is "re-manufacture the baseline",
	// which bakes the broken extractor's empty answer into the frozen record
	// permanently. The empty case must name the RECOMPUTATION.
	assert := func(t *testing.T, faults []string, wantCount int, wantSaid string) {
		t.Helper()
		if len(faults) != wantCount {
			t.Fatalf("expected %d complaint(s) and got %d: %v", wantCount, len(faults), faults)
		}
		if wantSaid == "" {
			return
		}
		for _, fault := range faults {
			if strings.Contains(fault, wantSaid) {
				return
			}
		}
		t.Errorf("the complaint reads %v, which never says %q — so it points a maintainer at the wrong object", faults, wantSaid)
	}

	t.Run("a name list recomputed as empty", func(t *testing.T) {
		// Deliberately with a NON-empty document: an empty recomputation beside a
		// populated baseline is exactly the shape a broken extractor produces.
		assert(t, gateBaselineStringFieldFaults("error_codes", []string{"E_ONE", "E_TWO"}, nil),
			1, "came out of the frozen tree empty")
	})

	t.Run("a name list the baseline is missing an entry from", func(t *testing.T) {
		assert(t, gateBaselineStringFieldFaults("error_codes", []string{"E_ONE"}, []string{"E_ONE", "E_TWO"}),
			1, "is missing \"E_TWO\"")
	})

	t.Run("a name list the baseline invented an entry in", func(t *testing.T) {
		assert(t, gateBaselineStringFieldFaults("error_codes", []string{"E_ONE", "E_LATER"}, []string{"E_ONE"}),
			1, "carries \"E_LATER\"")
	})

	t.Run("a name list holding the right names in the wrong order", func(t *testing.T) {
		assert(t, gateBaselineStringFieldFaults("error_codes", []string{"E_TWO", "E_ONE"}, []string{"E_ONE", "E_TWO"}),
			1, "different order")
	})

	t.Run("a name list that agrees", func(t *testing.T) {
		assert(t, gateBaselineStringFieldFaults("error_codes", []string{"E_ONE", "E_TWO"}, []string{"E_ONE", "E_TWO"}), 0, "")
	})

	t.Run("a behaviour fingerprint recomputed as empty", func(t *testing.T) {
		// The shape a fingerprint walker pointed at an empty directory produces,
		// which would otherwise make the silent-behaviour check — the reason this
		// artifact exists at all — pass over every package.
		assert(t, gateBaselineFingerprintFaults(map[string]string{"internal/lint": "abc"}, nil),
			1, "covers no packages")
	})

	t.Run("a behaviour fingerprint whose entry moved", func(t *testing.T) {
		// Failure scenario 1 exactly: a broadened lint rule moves this and
		// nothing else in the document.
		assert(t, gateBaselineFingerprintFaults(map[string]string{"internal/lint": "old"}, map[string]string{"internal/lint": "new"}),
			1, "behaviour_fingerprint[internal/lint] is old")
	})

	t.Run("a behaviour fingerprint missing a package the release shipped", func(t *testing.T) {
		assert(t, gateBaselineFingerprintFaults(map[string]string{}, map[string]string{"internal/lint": "new"}),
			1, "no entry for internal/lint")
	})

	t.Run("a behaviour fingerprint carrying a package the release did not ship", func(t *testing.T) {
		assert(t, gateBaselineFingerprintFaults(
			map[string]string{"internal/lint": "same", "internal/later": "x"},
			map[string]string{"internal/lint": "same"}),
			1, "carries internal/later")
	})

	t.Run("a behaviour fingerprint that agrees", func(t *testing.T) {
		assert(t, gateBaselineFingerprintFaults(map[string]string{"internal/lint": "same"}, map[string]string{"internal/lint": "same"}), 0, "")
	})

	// The link-time comparison's own three cases. The empty one is the reason
	// the function takes a count at all: against the real frozen tree the build
	// always reports 19 commands and 28 lint rules, so a shim that ran and
	// reported nothing — the shape a failed extraction inside the old tree
	// produces — would otherwise be compared against a baseline whose field was
	// deleted and found to agree.
	t.Run("a link-time field the build reported nothing for", func(t *testing.T) {
		assert(t, gateBaselineBuiltFieldFaults("lint_rules", `["mixed-cycle"]`, `[]`, 0),
			1, "reported no lint_rules at all")
	})

	t.Run("a link-time name edited by hand", func(t *testing.T) {
		// A rule renamed as it is broadened: the one edit the fingerprints, the
		// counts and the canonical form all pass over.
		assert(t, gateBaselineBuiltFieldFaults("lint_rules", `["mixed-cycles"]`, `["mixed-cycle"]`, 1),
			1, "is not what "+gateBaselineRelease+" reports when it is built")
	})

	t.Run("a link-time field that agrees", func(t *testing.T) {
		assert(t, gateBaselineBuiltFieldFaults("lint_rules", `["mixed-cycle"]`, `["mixed-cycle"]`, 1), 0, "")
	})

	pin := surfacePin{File: "README.md", Pin: "dossierx@" + gateBaselineRelease, Version: gateBaselineRelease}

	t.Run("a pin sweep recomputed as empty", func(t *testing.T) {
		assert(t, gateBaselinePinFaults([]surfacePin{pin}, nil), 1, "swept no version pins")
	})

	t.Run("a pin the checklist moved", func(t *testing.T) {
		// The maintainer doing exactly what docs/RELEASING.md's sweep says.
		moved := surfacePin{File: "README.md", Pin: "dossierx" + "@" + "v0.6.0", Version: "v0.6.0"}
		assert(t, gateBaselinePinFaults([]surfacePin{moved}, []surfacePin{pin}), 1, "version_pins is not what")
	})

	t.Run("pins that agree", func(t *testing.T) {
		assert(t, gateBaselinePinFaults([]surfacePin{pin}, []surfacePin{pin}), 0, "")
	})
}

// TestGateBaselineIsAnchoredToTheFrozenTreeAndNotToHEAD is the anti-vacuity
// proof for the test above, and it is here because of exactly what makes this
// artifact hard.
//
// Today v0.5.0's shipped sources and HEAD's are the same bytes, so "the baseline
// matches what the emitter computes" is a sentence that stays true whether the
// recomputation reads the frozen commit or reads this working tree. Written the
// second way the check is worthless — it compares surface.json against itself
// and cannot fail — and nobody reading the suite could tell the two apart,
// because on this tree they print the same result. That is the exact shape an
// earlier attempt at these lanes shipped, so it is asserted rather than trusted.
//
// Two things are proved, and they are different claims.
//
// THE TREE READ IS v0.5.0'S. v0.5.0 predates the surface emitter and carries no
// surface.json; every tree from v0.5.1 onward carries one. So "the materialised
// tree has no surface.json in it, and this working tree does" separates the
// frozen checkout from any later one, permanently and without depending on which
// files happen to differ this month.
//
// THE COMPARISON RESPONDS TO BYTES. A fingerprint that is computed the same way
// no matter what the files say would satisfy every comparison above while
// measuring nothing. So one shipped source is changed in a COPY of the current
// tree — a copy, because these checks are read-only against the repository and a
// test that edited internal/lint to prove a point would be the read-only tool
// quietly changing the environment it was asked to observe — and the fingerprint
// is required to move.
func TestGateBaselineIsAnchoredToTheFrozenTreeAndNotToHEAD(t *testing.T) {
	root := surfaceRepoRoot(t)
	doc, _ := gateBaselineDocument(t, root)

	head, err := gateResolve(root, "HEAD")
	if err != nil {
		t.Fatalf("resolve HEAD: %v", err)
	}
	frozen := gateBaselineCheckout(t)
	_, frozenStatErr := os.Stat(filepath.Join(frozen, surfaceFileName))
	_, workingStatErr := os.Stat(filepath.Join(root, surfaceFileName))
	if fault := gateBaselineAnchorFault(head, frozenStatErr == nil, workingStatErr == nil); fault != "" {
		t.Fatal(fault)
	}

	const probe = "internal/lint"
	frozenSum, ok := doc.BehaviourFingerprint[probe]
	if !ok {
		t.Fatalf("the baseline carries no behaviour_fingerprint entry for %s, so this proof has nothing to measure against", probe)
	}
	fromFrozen, err := surfaceBehaviourFingerprint(frozen)
	if err != nil {
		t.Fatalf("compute %s's behaviour fingerprint: %v", gateBaselineRelease, err)
	}
	if fromFrozen[probe] != frozenSum {
		t.Fatalf("the frozen tree's %s fingerprint (%s) already disagrees with the baseline (%s); "+
			"TestGateBaselineIsTheFrozenTreesInventory reports why", probe, fromFrozen[probe], frozenSum)
	}

	mutated := gateBaselineCopyPackage(t, root, probe)
	fromMutated, err := surfaceBehaviourFingerprint(mutated)
	if err != nil {
		t.Fatalf("compute the mutated tree's behaviour fingerprint: %v", err)
	}
	if fromMutated[probe] == frozenSum {
		t.Fatalf("a shipped source under %s was changed and its behaviour fingerprint did not move (%s both times). "+
			"hashRepoFiles is not reading what it is supposed to read, and every fingerprint comparison in this file "+
			"is passing over content it cannot see", probe, frozenSum)
	}
}

// gateBaselineAnchorFault decides whether the anchor proof above can be MADE. It
// returns "" when the premise holds and the reason when it does not.
//
// It is a function of three facts rather than three inline ifs, and that is the
// only way it can be exercised: reaching the first branch means running the suite
// from the v0.5.0 commit, where this file does not exist; reaching the third
// means deleting the tracked surface.json, which reds a dozen unrelated tests
// and proves nothing about this one. Written as a function they are all reachable
// with three booleans, and a branch deleted from it turns
// TestGateBaselineAnchorPremiseIsCheckedRatherThanAssumed red.
//
// Each is a premise the proof RESTS on, not a nicety. If this checkout is
// v0.5.0, "the frozen tree" and "this tree" are the same tree and the proof
// distinguishes nothing. If the materialised tree carries a surface.json it is
// not v0.5.0 — that release predates the emitter — so the recomputations are
// reading some later tree, most likely this one, which makes every comparison in
// this file a comparison of the current inventory against itself. And if this
// working tree has no surface.json, then "the frozen tree lacks one and this tree
// has one" separated nothing and the first premise was met by accident.
func gateBaselineAnchorFault(head string, frozenCarriesSurfaceJSON, workingTreeCarriesSurfaceJSON bool) string {
	if head == gateBaselineCommit {
		return fmt.Sprintf("this checkout IS %s (%s), so \"the frozen tree\" and \"this tree\" are the same tree and the anchor "+
			"cannot be told apart here. Run the suite from a commit after the release; this is reported rather than "+
			"passed, because a proof that cannot be made is not a proof that succeeded",
			gateBaselineRelease, gateBaselineCommit)
	}
	if frozenCarriesSurfaceJSON {
		return fmt.Sprintf("the materialised tree carries %s, so it is NOT %s: that release shipped before the surface emitter "+
			"existed and carries none. The recomputations are reading some later tree — most likely this one — which "+
			"makes every one of them a comparison of the current inventory against itself",
			surfaceFileName, gateBaselineRelease)
	}
	if !workingTreeCarriesSurfaceJSON {
		return fmt.Sprintf("this working tree has no %s, so \"the frozen tree has none and this one does\" separated nothing "+
			"and the check above was satisfied by a property both trees share", surfaceFileName)
	}
	return ""
}

// TestGateBaselineAnchorPremiseIsCheckedRatherThanAssumed exercises the three
// premises of the anchor proof, each on the state this repository cannot be put
// into while the suite is running.
func TestGateBaselineAnchorPremiseIsCheckedRatherThanAssumed(t *testing.T) {
	const laterCommit = "1111111111111111111111111111111111111111"

	for _, c := range []struct {
		name                      string
		head                      string
		frozenHas, workingTreeHas bool
		wantFault                 string
	}{
		{
			name: "the suite is run from the frozen release itself", head: gateBaselineCommit,
			frozenHas: false, workingTreeHas: true,
			wantFault: "the same tree",
		},
		{
			name: "the materialised tree is a later one", head: laterCommit,
			frozenHas: true, workingTreeHas: true,
			wantFault: "is NOT " + gateBaselineRelease,
		},
		{
			name: "this working tree has no surface.json to separate it from the frozen one", head: laterCommit,
			frozenHas: false, workingTreeHas: false,
			wantFault: "separated nothing",
		},
		{
			name: "the premise holds", head: laterCommit,
			frozenHas: false, workingTreeHas: true,
			wantFault: "",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			fault := gateBaselineAnchorFault(c.head, c.frozenHas, c.workingTreeHas)
			switch {
			case c.wantFault == "" && fault != "":
				t.Errorf("a sound premise was reported as a fault: %s", fault)
			case c.wantFault != "" && fault == "":
				t.Errorf("the anchor proof was allowed to proceed when %s. Every comparison in this file would then be "+
					"the current inventory measured against itself, which passes and means nothing — the exact shape an "+
					"earlier attempt at these lanes shipped.", c.name)
			case c.wantFault != "" && !strings.Contains(fault, c.wantFault):
				t.Errorf("the fault reads %q, which does not tell a maintainer that %s", fault, c.wantFault)
			}
		})
	}
}

// gateBaselineCopyPackage copies the repository into a temporary directory,
// appends one byte-visible line to a package's first non-test source, and
// returns the copy's root. Only the trees behaviourPackageFiles walks are
// copied, because that is all the caller recomputes over.
func gateBaselineCopyPackage(t *testing.T, root, pkg string) string {
	t.Helper()
	dir := t.TempDir()
	for _, top := range behaviourRoots {
		if err := os.CopyFS(filepath.Join(dir, top), os.DirFS(filepath.Join(root, top))); err != nil {
			t.Fatalf("copy %s: %v", top, err)
		}
	}
	sources, err := goSourceFiles(filepath.Join(dir, filepath.FromSlash(pkg)))
	if err != nil {
		t.Fatalf("list %s's sources in the copy: %v", pkg, err)
	}
	if len(sources) == 0 {
		t.Fatalf("%s holds no non-test Go source in the copy", pkg)
	}
	body, err := os.ReadFile(sources[0])
	if err != nil {
		t.Fatalf("read %s: %v", sources[0], err)
	}
	if err := os.WriteFile(sources[0], append(body, "\n// a change to a shipped source\n"...), 0o644); err != nil {
		t.Fatalf("write %s: %v", sources[0], err)
	}
	return dir
}

// ---------------------------------------------------------------------
// the artifact's shape, and the residue
// ---------------------------------------------------------------------

// TestGateBaselineIsACanonicalSurfaceDocument holds the artifact to surface.json's
// exact encoding: the same key order, two-space indent, sorted map keys and a
// trailing newline, with no key surfaceDoc does not declare and none dropped.
//
// It is the check that settles what "carry its own identity" means for this
// file. The artifact is a PURE COPY of the document shape — no extra key naming
// the release it belongs to — because such a key would make the baseline
// structurally unlike every other surface inventory and would surface in the
// very first release delta as an added key no code change caused. Identity is
// carried by gateBaselineCommit and enforced by the resolver instead. This test
// is what stops somebody adding the key anyway, and it is also what catches a
// hand-edit that reflows or re-orders the file on its way past a reviewer.
func TestGateBaselineIsACanonicalSurfaceDocument(t *testing.T) {
	root := surfaceRepoRoot(t)
	doc, raw := gateBaselineDocument(t, root)

	remarshalled, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("re-marshal the baseline: %v", err)
	}
	remarshalled = append(remarshalled, '\n')
	if !bytes.Equal(raw, remarshalled) {
		t.Errorf("%s is not in the canonical form surfaceBytes writes.\n%s\n"+
			"A consumer reads this file with the same reader it reads surface.json with; a document that only\n"+
			"round-trips is one an equality diff will report churn over.\n%s",
			gateBaselineBootstrapFile, surfaceLineDiff(string(raw), string(remarshalled)), gateBaselineRecovery)
	}
}

// TestGateBaselineFieldsAreRecomputedOrNamedAsResidue keeps this file's coverage
// claim honest as the document grows.
//
// The package comment says which fields are recomputed from the frozen tree and
// which are link-time residue. That sentence is only worth anything if it cannot
// go quietly out of date — and the way it goes out of date is a later lane
// adding a field to surfaceDoc, which then belongs to neither list and is
// checked by nothing while the file still reads as if everything were covered.
// "Never narrow coverage silently" is the rule; this is its enforcement.
func TestGateBaselineFieldsAreRecomputedOrNamedAsResidue(t *testing.T) {
	emitted := jsonFieldNames(reflect.TypeOf(surfaceDoc{}))
	for _, fault := range gateBaselineCoverageFaults(gateBaselineRecomputedFields, gateBaselineResidueFields, emitted) {
		t.Error(fault)
	}
}

// gateBaselineCoverageFaults partitions the document's keys against this file's
// coverage claim and returns what does not line up.
//
// It is a function of its three lists so that every complaint it can make is
// reachable from a test. Two of them are not reachable any other way: a field
// NAMED in the claim that surfaceDoc no longer emits would have to be produced by
// deleting a field from the emitter, which reds TestGenerateSurfaceJSON and the
// meta-tests far more loudly first — so the mutation would be adjudicated by the
// wrong test and this branch would never be shown to work. Same for a field
// listed in both halves at once.
func gateBaselineCoverageFaults(recomputed, residue, emitted []string) []string {
	var faults []string
	declared := map[string]string{}
	for _, f := range recomputed {
		declared[f] = "recomputed from the frozen tree"
	}
	for _, f := range residue {
		if where, dup := declared[f]; dup {
			faults = append(faults, fmt.Sprintf("%q is listed as both %s and link-time residue; a field is one or the other, "+
				"and a field claimed as both is a field whose coverage nobody has actually decided", f, where))
		}
		declared[f] = "link-time residue"
	}

	if len(emitted) == 0 {
		return append(faults, "surfaceDoc marshals no keys, so this partition covers nothing and would report every claim as sound")
	}
	inDoc := map[string]bool{}
	for _, name := range emitted {
		inDoc[name] = true
		if _, ok := declared[name]; !ok {
			faults = append(faults, fmt.Sprintf("surface.json's %q is checked by nothing in the frozen baseline and is not named as residue either.\n"+
				"Add it to gateBaselineRecomputedFields with a comparison in TestGateBaselineIsTheFrozenTreesInventory\n"+
				"if it is a pure function of a checkout, or to gateBaselineResidueFields with the reason it is not.\n"+
				"A field in neither list is a field the first gated release's delta reports on with no baseline behind it.", name))
		}
	}
	for _, name := range gateBaselineSortedKeys(declared) {
		if !inDoc[name] {
			faults = append(faults, fmt.Sprintf("%q is named in this file's coverage claim but surfaceDoc no longer emits it; "+
				"the claim is stale, and a stale claim is how this file comes to read as if everything were covered", name))
		}
	}
	return faults
}

// TestGateBaselineCoveragePartitionCatchesEveryWayItCanGoWrong exercises the
// partition on inputs the real document cannot supply.
//
// The claim this file makes about its own coverage is only worth something if it
// cannot go quietly out of date, and the ways it goes out of date all arrive
// through a later lane editing surfaceDoc — which is to say, not while this test
// is being written. Each is supplied directly here instead.
func TestGateBaselineCoveragePartitionCatchesEveryWayItCanGoWrong(t *testing.T) {
	for _, c := range []struct {
		name                         string
		recomputed, residue, emitted []string
		wantFault                    string
	}{
		{
			name:       "a field added to the document and to neither list",
			recomputed: []string{"error_codes"}, residue: []string{"commands"},
			emitted:   []string{"commands", "error_codes", "new_thing"},
			wantFault: "checked by nothing",
		},
		{
			name:       "a field the claim names that the document no longer emits",
			recomputed: []string{"error_codes", "markdown_constructs"}, residue: []string{"commands"},
			emitted:   []string{"commands", "error_codes"},
			wantFault: "no longer emits it",
		},
		{
			name:       "a field claimed as recomputed AND as residue",
			recomputed: []string{"error_codes"}, residue: []string{"error_codes"},
			emitted:   []string{"error_codes"},
			wantFault: "one or the other",
		},
		{
			name:       "a document that marshals nothing",
			recomputed: []string{"error_codes"}, residue: []string{"commands"},
			emitted:   nil,
			wantFault: "covers nothing",
		},
		{
			name:       "a claim that covers the document exactly",
			recomputed: []string{"error_codes"}, residue: []string{"commands"},
			emitted:   []string{"commands", "error_codes"},
			wantFault: "",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			faults := gateBaselineCoverageFaults(c.recomputed, c.residue, c.emitted)
			if c.wantFault == "" {
				if len(faults) != 0 {
					t.Fatalf("a sound coverage claim was reported as faulty: %v", faults)
				}
				return
			}
			if len(faults) == 0 {
				t.Fatalf("%s was accepted. The partition is the only thing keeping this file's stated coverage true as "+
					"the document grows, and \"never narrow coverage silently\" is the rule it enforces.", c.name)
			}
			found := false
			for _, fault := range faults {
				if strings.Contains(fault, c.wantFault) {
					found = true
				}
			}
			if !found {
				t.Errorf("the faults reported for %s (%v) do not say %q", c.name, faults, c.wantFault)
			}
		})
	}
}

// TestGateBaselineCountsAgreeWithItsOwnLists holds the one field that is not
// recomputed from the frozen tree.
//
// The lists these numbers count ARE recomputed —
// TestGateBaselineLinkTimeNamesAreTheFrozenTreesOwnBuild builds v0.5.0 for the
// link-time six and TestGateBaselineIsTheFrozenTreesInventory reads the rest out
// of the frozen commit — so what is left for this test is the arithmetic
// between them: counts is taken
// over the FIELD it counts, never over the source that field was extracted
// from — surface_test.go says so and records the hole that taught it — so a
// dropped entry and its count must disagree.
//
// It is deliberately NOT a check against the numbers 7/19/28/44/14.
// surface_meta_test.go already pins those five against HEAD's document, and
// v0.5.0 happens to carry the same five, so a literal here would be green
// against a copy of HEAD's surface.json, green against a hand-written file, and
// red only on the day someone corrupts a count to a number no tree has ever
// produced. It would look like a guard on the baseline and be a guard on
// nothing.
func TestGateBaselineCountsAgreeWithItsOwnLists(t *testing.T) {
	root := surfaceRepoRoot(t)
	doc, _ := gateBaselineDocument(t, root)

	nouns := map[string]bool{}
	for _, cmd := range doc.Commands {
		if cmd.Path == "" {
			t.Errorf("commands carries a leaf with no path")
			continue
		}
		nouns[strings.Fields(cmd.Path)[0]] = true
	}

	for _, c := range []struct {
		key  string
		want int
		of   string
	}{
		{"commands", len(doc.Commands), "the commands list"},
		{"lint_rules", len(doc.LintRules), "the lint_rules list"},
		{"error_codes", len(doc.ErrorCodes), "the error_codes list"},
		{"http_routes", len(doc.HTTPRoutes), "the http_routes list"},
		{"nouns", len(nouns), "the distinct first segments of the command paths"},
	} {
		got, ok := doc.Counts[c.key]
		if !ok {
			t.Errorf("counts has no %q entry, so nothing holds %s to a number", c.key, c.of)
			continue
		}
		if got != c.want {
			t.Errorf("counts.%s reads %d beside %s of %d. The count and the list it counts come out of the same\n"+
				"document, so they disagree only if one of them was edited by hand.\n%s", c.key, got, c.of, c.want, gateBaselineRecovery)
		}
	}
	if len(doc.Counts) != 5 {
		t.Errorf("counts holds %d entries (%v); the five this test checks are the whole of it, and a sixth is a number nothing holds",
			len(doc.Counts), doc.Counts)
	}
}

// ---------------------------------------------------------------------
// whose baseline it is
// ---------------------------------------------------------------------

// TestGateBaselineTagStillNamesTheFrozenCommit guards the pointer.
//
// v0.5.0 is an annotated tag OBJECT, and a tag name is mutable: `git tag -f
// v0.5.0 <anything>` re-points it, and every check in this repository that names
// only the tag follows it. The artifact does not follow it — it is a function of
// one commit's tree and of nothing else — so if the two ever part company, the
// name the release procedure, the CHANGELOG and ci.yml all use has stopped
// meaning the tree this baseline describes, and the gate must say so rather than
// quietly diff against a different past.
func TestGateBaselineTagStillNamesTheFrozenCommit(t *testing.T) {
	root := surfaceRepoRoot(t)
	commit, err := gateResolve(root, gateBaselineRelease+"^{commit}")
	if err != nil {
		t.Fatalf("%s does not resolve to a commit in this clone: %v\n\n"+
			"surface.baseline.json is the inventory of that release and its identity cannot be confirmed without\n"+
			"the tag. This is a failure and not a skip — an unverified baseline handed to the surface agents is a\n"+
			"document about an unknown past. Fetch the tags:\n"+
			"    git fetch --tags --force\n"+
			"In CI, check out with `fetch-depth: 0`.", gateBaselineRelease, err)
	}
	if commit != gateBaselineCommit {
		t.Errorf("%s now points at %s, but surface.baseline.json is the inventory of %s.\n"+
			"The tag has been re-pointed. Everything that resolves the previous release BY NAME — the release\n"+
			"procedure, ci.yml's baseline step, tests/render_across_releases_test.go — is now reading a different\n"+
			"tree than this artifact describes, and the first gated release would be reviewed against a past that\n"+
			"never happened.",
			gateBaselineRelease, commit, gateBaselineCommit)
	}
}

// TestGateBaselineIsChosenByIdentityAndNeverByAFailure is failure scenario 2,
// written as a table because the branch that matters cannot occur in this
// repository yet.
//
// The scenario: v0.6.0 ships carrying its own surface.json. At v0.7.0 the gate
// runs in a checkout without tags. `git show v0.6.0:surface.json` fails, a
// resolver keyed on that failure substitutes the committed v0.5.0 bootstrap, and
// the run produces a full delta spanning two releases which thirteen agents read
// as the truth about v0.6.0. Being loud enough to look like work getting done is
// what makes it worse than an empty answer.
//
// The rule that prevents it is that the bootstrap is chosen because the previous
// release IS the frozen commit, and for no other reason. Every row below asks
// for a release this repository cannot produce today, which is the whole point:
// a resolver only ever exercised against the real repo would run one branch and
// look correct.
func TestGateBaselineIsChosenByIdentityAndNeverByAFailure(t *testing.T) {
	const otherCommit = "0000000000000000000000000000000000000000"

	t.Run("the frozen release resolves to the bootstrap", func(t *testing.T) {
		src, err := gateBaselineFor(gateBaselineRelease, gateBaselineCommit)
		if err != nil {
			t.Fatalf("gateBaselineFor(%s, %s): %v", gateBaselineRelease, gateBaselineCommit, err)
		}
		if src.Kind != gateBaselineFromBootstrap || src.Path != gateBaselineBootstrapFile {
			t.Errorf("the release the bootstrap was manufactured for resolved to %+v; nothing else can serve as %s's inventory, because %s carries no surface.json of its own",
				src, gateBaselineRelease, gateBaselineRelease)
		}
	})

	// A different NAME on the frozen commit still resolves to the bootstrap: the
	// tree is the identity. A different COMMIT never does, whatever it is called.
	t.Run("identity is the commit and not the name", func(t *testing.T) {
		src, err := gateBaselineFor("v0.5.0-rerolled", gateBaselineCommit)
		if err != nil || src.Kind != gateBaselineFromBootstrap {
			t.Errorf("the frozen commit under another tag name resolved to %+v (%v); the artifact is a function of the tree, not of what the tree is called", src, err)
		}
	})

	for _, release := range []string{"v0.6.0", "v0.7.0", "v1.0.0"} {
		t.Run("a later release never reaches the bootstrap: "+release, func(t *testing.T) {
			src, err := gateBaselineFor(release, otherCommit)
			if err != nil {
				t.Fatalf("gateBaselineFor(%s, %s): %v", release, otherCommit, err)
			}
			if src.Kind == gateBaselineFromBootstrap {
				t.Fatalf("%s resolved to the frozen %s bootstrap. That substitution is failure scenario 2: the delta spans two releases, "+
					"every surface agent is handed it as evidence, and the release is reviewed against a past that is not its own",
					release, gateBaselineRelease)
			}
			want := release + ":" + surfaceFileName
			if src.Kind != gateBaselineFromTag || src.Rev != want {
				t.Errorf("%s resolved to %+v; a release that carries its own inventory is read from %q", release, src, want)
			}
		})
	}

	// An unresolvable predecessor is a failure. It is NOT the bootstrap, and it
	// is NOT an empty delta: "we could not identify the past" must never reach
	// the report as "nothing changed".
	for _, c := range []struct {
		name, release, commit string
	}{
		{"no previous release at all", "", ""},
		// The row that reaches the empty-name refusal, and the only one that
		// can. Every other unresolvable row carries a commit the 40-hex check
		// refuses on the next line, so the refusal above it could be deleted
		// outright and this table stayed green — while a caller that had
		// resolved a commit and no name at all would be handed the frozen
		// bootstrap as its baseline, silently, because that commit IS the
		// frozen one.
		{"a commit with no release name against it", "", gateBaselineCommit},
		{"a tag that resolved to nothing", "v0.6.0", ""},
		{"a tag name offered in place of a commit", "v0.6.0", "v0.6.0"},
		{"an abbreviated object name", "v0.6.0", "0000000"},
	} {
		t.Run("unresolvable is a failure: "+c.name, func(t *testing.T) {
			src, err := gateBaselineFor(c.release, c.commit)
			if !errors.Is(err, errGateUncheckable) {
				t.Fatalf("gateBaselineFor(%q, %q) returned %+v, %v; an unresolvable baseline must be errGateUncheckable — "+
					"there is no result here that means \"we did not check\" and reads as \"it is fine\"", c.release, c.commit, src, err)
			}
			if src.Kind != "" {
				t.Errorf("gateBaselineFor(%q, %q) returned the source %+v alongside its error; a caller that ignores the error would diff against it",
					c.release, c.commit, src)
			}
		})
	}
}

// TestGateBaselineResolvesInThisRepository runs the resolver against the real
// clone, so the rule above is bound to something rather than only to a table.
//
// It does not assert that today's answer is the bootstrap. It asserts the
// property that has to hold at every release: whatever the resolver chose, the
// document is THERE AND READABLE, and the bootstrap was chosen only for the
// frozen commit. That is also this artifact's end of life, made mechanical
// rather than remembered — the day v0.6.0 carries its own surface.json the
// resolver stops reaching for the bootstrap on its own, and this test starts
// checking that v0.6.0's inventory is readable instead. The file itself is kept
// rather than deleted: v0.5.0 can never gain a surface.json, so deleting it
// would make the v0.5.0-to-v0.6.0 delta unreproducible forever.
func TestGateBaselineResolvesInThisRepository(t *testing.T) {
	root := surfaceRepoRoot(t)
	release, commit := gateBaselineResolve(t, root)

	src, err := gateBaselineFor(release, commit)
	if err != nil {
		t.Fatalf("this repository's previous release (%s at %s) resolves to no baseline: %v", release, commit, err)
	}

	if src.Kind == gateBaselineFromBootstrap && commit != gateBaselineCommit {
		t.Fatalf("the bootstrap was chosen for %s at %s, which is not the frozen commit %s", release, commit, gateBaselineCommit)
	}
	if fault := gateBaselineReadableFault(root, src); fault != "" {
		t.Fatal(fault)
	}
}

// TestGateBaselineResolutionRefusesRatherThanGuesses reaches, in a fixture
// repository, every refusal the resolution path makes and this clone cannot
// produce.
//
// All of them were unreachable, and unreachable in the way that matters: each
// could be deleted outright and the suite stayed green, because this repository
// has its tags, resolves them, holds the bootstrap file, and has v0.5.0 as its
// previous release. So the checks that carry the whole of "a baseline that
// cannot be resolved is a failed gate and never an empty delta" were checks
// nobody had ever seen fire. A tagless repository, a repository whose named
// release is not in it, and a tag carrying no inventory are three states this
// clone cannot be put into and a fixture can.
func TestGateBaselineResolutionRefusesRatherThanGuesses(t *testing.T) {
	// fixture is a repository with one commit, optionally tagged, carrying the
	// named files. It is the shallow-checkout shape when tag is empty: content,
	// history, and no tags at all.
	fixture := func(t *testing.T, tag string, files map[string]string) string {
		t.Helper()
		dir := t.TempDir()
		gateTestGit(t, dir, "init", "-q", "-b", "main")
		gateTestGit(t, dir, "config", "user.email", "fixture@example.invalid")
		gateTestGit(t, dir, "config", "user.name", "fixture")
		for rel, body := range files {
			if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
				t.Fatalf("write %s: %v", rel, err)
			}
		}
		gateTestGit(t, dir, "add", "-A")
		gateTestGit(t, dir, "commit", "-q", "-m", "fixture", "--allow-empty")
		if tag != "" {
			gateTestGit(t, dir, "tag", tag)
		}
		return dir
	}

	t.Run("a checkout with no tags is a failed gate", func(t *testing.T) {
		// actions/checkout at its default depth, which is how this repository
		// shipped the baseline failure once already.
		t.Setenv(gateBaselinePrevReleaseTagEnv, "")
		_, _, fault := gateBaselineResolveFault(fixture(t, "", map[string]string{"README.md": "# fixture\n"}))
		if fault == "" {
			t.Fatal("a checkout with no release tag resolved a baseline anyway. There is nothing to diff against in that " +
				"state, and an unidentified past reaching the report as an empty delta reads exactly like a release that changed nothing")
		}
		for _, want := range []string{"NOTHING WOULD BE COMPARED", "fetch-depth: 0"} {
			if !strings.Contains(fault, want) {
				t.Errorf("the refusal does not say %q, and it is the only thing a maintainer gets:\n%s", want, fault)
			}
		}
	})

	t.Run("a named release that is not in this checkout is a failed gate", func(t *testing.T) {
		// The tag NAME is not an identity: it has to resolve to a commit here,
		// or the gate has no tree to call the past.
		t.Setenv(gateBaselinePrevReleaseTagEnv, "v0.6.0")
		release, commit, fault := gateBaselineResolveFault(fixture(t, "", map[string]string{"README.md": "# fixture\n"}))
		if fault == "" {
			t.Fatalf("%q was accepted as this release's predecessor while resolving to %q; a name that names no tree is not an identity", release, commit)
		}
		if commit != "" {
			t.Errorf("the refusal came back with the commit %q beside it, which a caller that ignored the fault would diff against", commit)
		}
	})

	t.Run("a tagged checkout resolves to its tag and commit", func(t *testing.T) {
		// The other half: a resolver that refused everything would satisfy the
		// two rows above and identify no baseline ever.
		t.Setenv(gateBaselinePrevReleaseTagEnv, "")
		release, commit, fault := gateBaselineResolveFault(fixture(t, "v0.6.0", map[string]string{"README.md": "# fixture\n"}))
		if fault != "" {
			t.Fatalf("a repository whose newest tag is v0.6.0 resolved to nothing: %s", fault)
		}
		if release != "v0.6.0" || !gateSHARE.MatchString(commit) {
			t.Errorf("resolved to %q at %q; the tag is the name and the commit is the identity", release, commit)
		}
	})

	t.Run("a later release whose own inventory cannot be read is a failed gate", func(t *testing.T) {
		// Failure scenario 2, in the only place it can be staged: `git show
		// v0.6.0:surface.json` fails here for the same reason it fails in a
		// shallow clone, and the answer must be a refusal rather than v0.5.0's
		// bootstrap standing in for a release it is not the inventory of.
		dir := fixture(t, "v0.6.0", map[string]string{"README.md": "# fixture\n"})
		fault := gateBaselineReadableFault(dir, gateBaselineSource{
			Kind: gateBaselineFromTag, Release: "v0.6.0", Commit: strings.Repeat("a", 40), Rev: "v0.6.0:" + surfaceFileName,
		})
		if fault == "" {
			t.Fatal("a release whose own surface.json could not be read was reported as readable. Substituting the frozen " +
				"bootstrap there produces a delta spanning two releases — full, plausible, and handed to every surface " +
				"agent as the truth about a past that is not this release's")
		}
		if !strings.Contains(fault, "must NOT be substituted") {
			t.Errorf("the refusal does not say the bootstrap must not stand in for it:\n%s", fault)
		}
	})

	t.Run("a later release whose inventory is readable", func(t *testing.T) {
		dir := fixture(t, "v0.6.0", map[string]string{surfaceFileName: "{}\n"})
		if fault := gateBaselineReadableFault(dir, gateBaselineSource{
			Kind: gateBaselineFromTag, Release: "v0.6.0", Commit: strings.Repeat("a", 40), Rev: "v0.6.0:" + surfaceFileName,
		}); fault != "" {
			t.Errorf("a tag carrying its own inventory was reported unreadable: %s", fault)
		}
	})

	t.Run("the bootstrap missing from the tree is a failed gate", func(t *testing.T) {
		dir := fixture(t, "", map[string]string{"README.md": "# fixture\n"})
		src := gateBaselineSource{Kind: gateBaselineFromBootstrap, Release: gateBaselineRelease, Commit: gateBaselineCommit, Path: gateBaselineBootstrapFile}
		fault := gateBaselineReadableFault(dir, src)
		if fault == "" {
			t.Fatalf("%s was reported readable in a tree that does not have it. It is the only inventory %s will ever have — "+
				"that release carries no surface.json of its own — so its absence is a gate with no past to diff against",
				gateBaselineBootstrapFile, gateBaselineRelease)
		}
		if err := os.WriteFile(filepath.Join(dir, gateBaselineBootstrapFile), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write the fixture's baseline: %v", err)
		}
		if fault := gateBaselineReadableFault(dir, src); fault != "" {
			t.Errorf("the bootstrap was reported missing from a tree that has it: %s", fault)
		}
	})

	t.Run("a kind nothing knows how to read is a failed gate", func(t *testing.T) {
		if fault := gateBaselineReadableFault(t.TempDir(), gateBaselineSource{Kind: "invented", Release: "v0.6.0"}); fault == "" {
			t.Fatal("a baseline of an unknown kind was reported readable, so a source nothing can read would reach the delta as if it had been")
		}
	})
}

// ---------------------------------------------------------------------
// the frozen record must not be swept as a live pin site
// ---------------------------------------------------------------------

// TestGateBaselineIsExcludedFromTheVersionPinSweep is failure scenario 3, from
// the machine side.
//
// surface.baseline.json is a copy of a surface document, so it carries v0.5.0's
// four pin tokens in its own version_pins field. surfaceVersionPins excludes
// surface.json for exactly this reason — the field would otherwise find its own
// output — and that exclusion does not cover a copy of the output under another
// name. Without a fourth exclusion the LIVE contract grows four entries pointing
// at a frozen historical record, in the field whose job is to be this tree's
// current pin inventory.
//
// It is checked in a FIXTURE repository rather than against this one, and that
// is the whole reason the test is worth writing. Run against the real tree the
// obvious assertion — "surface.json's version_pins names no baseline file" — is
// green whether or not the exclusion exists, because `git grep` cannot see an
// untracked file and passes over the artifact entirely on the run that lands it.
// The fixture makes the file tracked and the sweep therefore able to reach it,
// so removing the exclusion turns this red.
func TestGateBaselineIsExcludedFromTheVersionPinSweep(t *testing.T) {
	fixture := t.TempDir()
	gateTestGit(t, fixture, "init", "-q", "-b", "main")
	gateTestGit(t, fixture, "config", "user.email", "fixture@example.invalid")
	gateTestGit(t, fixture, "config", "user.name", "fixture")

	// The token is ASSEMBLED rather than written out, and that is not style. A
	// literal pin in this file would be found by the very sweep under test:
	// surfaceVersionPins excludes no *_test.go, so surface.json's version_pins
	// would grow an entry pointing at a fixture and docs/RELEASING.md's checklist
	// would tell a maintainer to bump it at every release. The pollution would
	// not even surface here — `git grep` cannot see an untracked file, so it
	// would first appear in the regenerated contract during the NEXT lane's
	// landing, printed and committed under that lane's name.
	pinLine := "go install github.com/BarterX-Tech/dossierx/cmd/dossierx@" + gateBaselineRelease + "\n"
	for _, f := range []struct{ rel, body string }{
		// A live site, so the sweep is demonstrably working in this fixture. An
		// exclusion test whose sweep found nothing at all would be green for the
		// wrong reason.
		{"README.md", pinLine},
		// The frozen record, carrying the same token.
		{gateBaselineBootstrapFile, pinLine},
	} {
		if err := os.WriteFile(filepath.Join(fixture, f.rel), []byte(f.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", f.rel, err)
		}
	}
	gateTestGit(t, fixture, "add", "-A")
	gateTestGit(t, fixture, "commit", "-q", "-m", "fixture")

	pins, err := surfaceVersionPins(fixture)
	if err != nil {
		t.Fatalf("sweep the fixture: %v", err)
	}

	live := 0
	for _, pin := range pins {
		if pin.File == gateBaselineBootstrapFile {
			t.Errorf("the pin sweep reported %s as a live version-pin site (%+v).\n"+
				"That file is the frozen inventory of %s. Its pins are correct precisely because they are old —\n"+
				"the same reason CHANGELOG.md is excluded — and docs/RELEASING.md's checklist will tell a maintainer\n"+
				"to move every pin the sweep reports. Moving this one rewrites the only baseline the first gated\n"+
				"release has, and there is no second copy: %s carries no surface.json of its own.",
				gateBaselineBootstrapFile, pin, gateBaselineRelease, gateBaselineRelease)
		}
		if pin.File == "README.md" {
			live++
		}
	}
	if live == 0 {
		t.Fatalf("the sweep found no pin in the fixture's README.md, so it was not searching anything and the exclusion above proved nothing. Pins: %+v", pins)
	}
}

// gateBaselineSweepItemRE is docs/RELEASING.md's pin-sweep checklist item, taken
// as the slice between its own heading and the next one, for the reason
// gateAncestryItemRE gives: the file mentions version pins in several places and
// the question is what the item a maintainer READS tells them to do.
var gateBaselineSweepItemRE = regexp.MustCompile(`(?s)- \[ \] \*\*The version pins are moved\.\*\*.*?\n- \[ \] `)

// TestReleaseProcedureExcludesTheFrozenBaselineFromItsSweep is failure scenario
// 3 from the human side, which is the side it actually arrives on.
//
// The machine sweep and the checklist sweep are two different lists — the
// checklist excludes CHANGELOG.md and itself, surfaceVersionPins excludes those
// plus surface.json — and only the machine one is executed. So excluding the
// baseline in code and not in the checklist leaves the maintainer reading, at
// the next release, an instruction to move a pin that lives inside a record that
// must never move. The alternative outcome is no better: a permanent false alarm
// on a checklist whose entire value is that it is swept rather than remembered.
func TestReleaseProcedureExcludesTheFrozenBaselineFromItsSweep(t *testing.T) {
	root := surfaceRepoRoot(t)
	procedure := gateReadRepoFile(t, root, filepath.Join("docs", "RELEASING.md"))

	item := gateBaselineSweepItemRE.FindString(procedure)
	if item == "" {
		t.Fatal("docs/RELEASING.md no longer carries an item titled **The version pins are moved.**. " +
			"That item is the sweep as a maintainer actually performs it; surfaceVersionPins reports what the pins ARE " +
			"and moves none of them")
	}

	command := ""
	for _, line := range strings.Split(item, "\n") {
		if strings.Contains(line, "git grep") {
			command = line
			continue
		}
		if command != "" && strings.Contains(line, "':!") {
			command += "\n" + line
			break
		}
	}
	if command == "" {
		t.Fatalf("the pin-sweep item no longer carries a `git grep` command, so there is nothing for a maintainer to run.\nThe item reads:\n%s", item)
	}
	if !strings.Contains(command, "':!"+gateBaselineBootstrapFile+"'") {
		t.Errorf("docs/RELEASING.md's pin sweep does not exclude %s. The command reads:\n%s\n\n"+
			"That file is %s's frozen inventory and every pin in it is a historical fact. From the next release\n"+
			"onward this sweep reports it beside README.md and the CI template, and the maintainer doing exactly\n"+
			"what the checklist says rewrites the only baseline the first gated release has — silently, because a\n"+
			"pin bump looks like the most routine edit in the procedure. Excluding it in surfaceVersionPins alone\n"+
			"does not help: that function reports pins, and this checklist is what moves them.",
			gateBaselineBootstrapFile, command, gateBaselineRelease)
	}
}
