// gate_release_stamp_test.go holds the release stamp deleted, and holds the
// sentence that replaced it true.
//
// WHAT WAS DELETED. Each release entry in site/src/content.ts carried a `commit`
// field — the tagged release's short sha, hand-stamped onto the site after the
// tag existed. It could not converge, because writing the sha is itself a
// commit, so the value was stale the moment it landed: v0.4.1 shipped naming
// `5327923` while `refs/tags/v0.4.1` points at `206b4a4`, and v0.5.0 chased its
// own sha through two commits. It also disagreed with the binary by
// construction, since GoReleaser stamps `main.commit` from `{{.Commit}}` — forty
// characters against seven. The field is gone, along with the checklist step
// that wrote it and the `?? "(devel)"` fallback that rendered it.
//
// WHY IT NEEDS A TEST. A deletion pins nothing. The field was removed because it
// burned two releases, and the next person to notice the `dossierx version`
// transcript is missing a line will helpfully add it back — which is how it got
// there the first time. These assertions make that a red build instead of a
// third burned release.
//
// AND THE SENTENCES THAT REPLACED IT. docs/RELEASING.md used to claim that a
// `(devel)` fallback from `go install ...@vX.Y.Z` proved the ldflags had not
// applied. That was false in both directions — the proxy sets `info.Main.Version`
// to the tag, so an unstamped `go install` build prints the tag anyway, and this
// binary cannot print `(devel)` at all because resolveVersionInfo excludes that
// exact value.
//
// The first replacement for it was false in the same way, one step further
// along: it moved to the ARTIFACT step and said an unstamped archive reports
// `dev`. It does not. Go stamps `info.Main.Version` from the VCS tag, so a
// no-ldflags build of a clean checkout at a tag reports a version, the full sha
// and a real timestamp — every one of them plausible, so a check resting on the
// version envelope certifies failure as success. (What it reports is not
// identical to a stamped build's, which is correction ONE below; it is
// indistinguishable to a reader, which is the part that matters here.)
//
// What DOES tell them apart is the binary's own build settings, which record
// the link flags: `go version -m` prints a `build -ldflags=` line, or prints
// none at all for a build that got no flags. That is the check the procedure
// now carries, and TestLdflagsShowUpOnlyInTheBuildSettings is what keeps it
// true. `dev` survives in exactly one documented place — the last-resort
// fallback when there is no VCS metadata either — and
// TestBuildWithNoVCSAndNoLdflagsReportsDev pins that, without depending on
// whether the tree it runs in has a usable git checkout.
//
// TWO CORRECTIONS TO THE ABOVE, both made after this file's first version
// shipped a guard that could not see the defect it was written for. They are
// recorded here rather than quietly folded in, because each of them is a
// measurement that contradicts something this header used to assert.
//
// ONE — "every field identical to a correctly stamped build" is FALSE, and the
// paragraph above said it as measured fact. Built side by side on the v0.5.0
// tree, the published archive and a no-ldflags `go build` of the same commit
// report:
//
//	                 published archive        no ldflags
//	version          0.5.0                    v0.5.0
//	commit           3217a48…eb7ce            3217a48…eb7ce
//	date             2026-08-07T06:27:25Z     2026-08-07T06:17:22Z
//
// The commit is byte-identical, the dates are two real timestamps neither of
// which is identifiable as the wrong one from its value, and the VERSION
// DIFFERS — by a single leading `v`, because GoReleaser's `{{.Version}}` strips
// it where `info.Main.Version` keeps it. The load-bearing conclusion survives
// (no reading of the version envelope certifies the stamping; one character in
// the field nobody reads character by character is not a check, and it inverts
// the day `.goreleaser.yaml` moves to `{{.Tag}}`) but the statement it rested on
// did not, so docs/RELEASING.md now carries the measurement instead of the
// slogan, and gateLdflagsLies below refuses the slogan by name.
//
// TWO, THREE AND FOUR ARE A SINGLE HISTORY AND THE TEST THEY WERE ABOUT HAS LEFT
// THIS FILE. They are kept because each is a way a guard on this subject has
// actually been defeated, and the replacement has to be judged against all three.
//
// The test was TestSiteVersionTranscriptIsALiteralPrefixOfRealOutput, and it read
// the `example` transcript out of site/src/content.ts AS SOURCE TEXT. Correction
// TWO: it chose a constant, linked its binary with it and substituted the same
// constant into the transcript, so both sides read `v9.9.9` whatever the release
// build did and the one thing the site was wrong about — depicting
// `dossierx version v0.5.0` where the archive prints `dossierx version 0.5.0` —
// was structurally invisible. Correction THREE: the rewrite compared RESOLVED
// values, and a hand-typed literal resolves to itself, so replacing the
// interpolation with `0.5.0` left everything green. Correction FOUR: both
// operands rested on one unread model of `.goreleaser.yaml`, so a move to
// `{{.Tag}}` would have moved both sides together.
//
// Each fix was correct and none of them addressed the method. A transcript read
// out of source is a string this file has to interpret — which interpolations
// mean what, which declaration the bundler will actually evaluate, which lines
// are comments — and every one of those interpretations is a place to be wrong
// about a page nobody looked at. THE PAGE HAS ALREADY EVALUATED ITSELF. So the
// comparison now happens in viewer-tests/site_dom_test.go, against the rendered
// DOM of a real build: condition 8 reads the `version` card's session out of the
// browser and runs a binary linked the way a release links one. A comment
// renders nothing, a commented-out entry renders nothing, and a contradictory
// later declaration renders — and is what gets compared. There is deliberately
// no second copy of that check here; two readings of one claim can disagree, and
// the one that reads source would be the one that is wrong.
//
// WHAT STAYED, and why it could not move: TestSiteSelectsTheReleaseThisTreeModels
// below. A rendered read cannot see ReleaseTimeline's `latestIndex`, which
// decides which entry is badged "latest" — a timeline badging one release while
// every derived string names another renders perfectly on both halves. And
// correction THREE's hole is closed where such things can be closed, in
// viewer-tests/site_source_test.go's rule 3, which now refuses a renderable
// version literal in BOTH spellings: the tag's `v0.5.0` and the binary's `0.5.0`.
// That second spelling is the one nothing in this repository used to look for.
//
// FIVE — and this is correction FOUR's own defect, in the shape corrections TWO
// and THREE were about. gateRequireReleaseTransform read `.goreleaser.yaml` as
// ONE LONG STRING and asked whether it contained `-X main.version={{.Version}}`.
// A configuration file is not its text: that file carries a five-line prose
// comment naming the correct flag, so it contains the string whatever the build
// is told to do. Three mutations left every assertion here green — the flag
// swapped for the import-path spelling with the correct one added to the comment
// above it, the flag commented out in place, and a second
// `-X main.version={{.Tag}}` appended after the good one where the LAST
// assignment is the one the linker keeps. The checksum assertion had the same
// shape. So the configuration is PARSED now (gateGoreleaserConfig), the `-X`
// assignments are read as assignments, and a symbol stamped twice is a failure
// rather than a match on its first value. See that type's comment.
//
// SIX — the same lesson, one file over, unapplied in the guard written for the
// very next finding in the same pass. Correction FIVE parsed `.goreleaser.yaml`
// because a comment is a substring. The guard added alongside it to pin WHICH
// release the site calls current was a `strings.Contains` over the whole of
// site/src/content.ts, and it fell to a one-line mutation of exactly the shape
// FIVE describes: comment the pinned line out, write `releases[0]` beneath it,
// and the pinned text is still in the file. `go test ./...` green across the
// root module, viewer-tests green, and the deployed page renders the OLDEST
// release in the hero kicker, the hero badge, the release-history intro and the
// `dossierx version` transcript while ReleaseTimeline goes on badging the newest
// entry "latest". The same one-line mutation worked on the timeline's
// `latestIndex` and on `latestBinaryVersion` — the last of which reinstates
// `dossierx version v0.5.0`, the exact defect this file was opened for, with the
// guard written for it still reporting the model held. So the site's
// declarations are now read the way the configuration is: the file's COMMENTS
// ARE STRIPPED and the declaration must be the file's ONLY one. See
// gateSiteDeclarations.
//
// SEVEN — the import-path no-op was refused for ONE of the three symbols. The
// linker accepts `-X <import path>.commit=` and `-X <import path>.date=` and
// applies them to nothing exactly as it does for `.version`, and
// gateRequireReleaseTransform only ever named `.version`. Aiming both at the
// import path left every assertion here green: the binary falls back to
// `debug.ReadBuildInfo`, whose `vcs.revision` is a forty-character sha and whose
// `vcs.time` is an RFC 3339 timestamp, so the snapshot test's SHAPE checks are
// satisfied by the fallback itself — and the published binary then reports the
// commit's time where the release means the build's, with `go version -m`
// showing a perfectly good `-ldflags` line. That falsified this file's own claim
// that building the binary "catches the class": it caught the class for one
// symbol in three. Both halves are widened — the configuration is refused for
// all three symbols by name, and the snapshot binary's three reported fields are
// compared against the three values its own recorded link flags NAME.
//
// EIGHT — correction FIVE parsed the configuration and then read only two thirds
// of it. `builds` and `checksum` were asserted against; `archives` was not read at
// all, though it is the block that decides what the six downloads are CALLED. The
// matrix check counted platforms and the snapshot test stated six names, and
// nothing connected the two: a `name_template` edited to
// `{{ .ProjectName }}-{{ .Os }}-{{ .Arch }}`, or a deleted windows `format: zip`
// override, publishes six perfectly good archives under names docs/RELEASING.md
// does not tell anyone to ask for, with every count still correct. Quieter still
// is the `ids` wiring: an archive naming a build that does not exist packages
// NOTHING, and the release page still lists files either way. gateRequireArchiveNaming
// reads all three out of the parsed block.
//
// THE OVERRIDE THAT WAS RECORDED HERE IS WITHDRAWN, and both halves of it were
// wrong. This file previously carried a recorded override saying the specified
// `goreleaser release --snapshot --clean` dry run was not implemented, on two
// grounds. The measurement contradicts the first and the reasoning does not
// support the second.
//
//	"GoReleaser is not installed on this machine (`which goreleaser` finds
//	nothing)" — it is. `~/go/bin/goreleaser` is v2.17.1; `which` missed it
//	because $(go env GOPATH)/bin is not on the PATH `which` searched, which is
//	an observation about a shell and not about the machine. Run against this
//	tree it builds all six archives and `checksums.txt` in eleven seconds.
//
//	"installing it inside `go test ./...` would make the package's green depend
//	on a network fetch" — true, and it argues against FETCHING the tool, not
//	against RUNNING it. This repository had already made that distinction:
//	viewer-tests/site_dom_test.go requires a browser through
//	DOSSIERX_TEST_BROWSER, fails rather than skips when it is not named, fetches
//	nothing, and CI supplies the binary as a pinned job dependency. The gap was
//	never between "fetch it" and "do not check"; it was a third option that went
//	unconsidered.
//
// TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary is that third
// option, on the same contract under DOSSIERX_TEST_GORELEASER. It runs the
// release build and counts what came out, which is the only assertion in this
// tree that observes GoReleaser's behaviour rather than its inputs — and it is
// what catches the class of failure the configuration checks can only catch by
// name: an `-X` the linker accepts, records in the build settings, and applies
// to nothing.
//
// TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary is that third
// option, on the same contract under DOSSIERX_TEST_GORELEASER. It runs the
// release build and counts what came out, which is the only assertion in this
// tree that observes GoReleaser's behaviour rather than its inputs — and it is
// what catches the class of failure the configuration checks can only catch by
// name: an `-X` the linker accepts, records in the build settings, and applies
// to nothing.
//
// IT NO LONGER LIVES HERE, AND THAT IS THE CORRECTION TO THE PARAGRAPH THIS ONE
// REPLACES. The test is right and its position was wrong. Written into this
// package it made `go test ./...` in the root module FAIL on any machine that
// had not named a GoReleaser binary — which this header used to call "the
// intended shape", and then had to spend two paragraphs mitigating: the variable
// had to go into ci.yml's `test` job on all six matrix cells, running a
// cross-compiling release build six times for a result that does not depend on
// the host, and CONTRIBUTING.md was left describing a `go test -race ./...` dev
// loop that no longer worked.
//
// A strict check that makes the ordinary build unusable creates pressure to
// narrow a package selector, and narrowing a selector is the one repair this
// repository does not accept. The browser suite had already solved this and only
// half of it was copied: viewer-tests/ is a SEPARATE MODULE precisely so that a
// check with an external prerequisite can be strict without holding the engine's
// build hostage. So the test now sits in viewer-tests/site_toolchain_test.go, on
// the identical contract — it FAILS rather than skips when the tool is unnamed,
// it fetches nothing, and the CI job that runs that module supplies the binary
// the same way it supplies Chrome. The root suite is green on a clean machine and
// nothing about the check's strictness changed.
//
// What remains here is the INPUT side: this file reads what the release build is
// told to do.
//
// NINE — AND THE SUBJECT ITSELF WAS SUBSTITUTABLE. Every correction above is
// about reading `.goreleaser.yaml` correctly, and each one assumed the thing
// worth reading was `.goreleaser.yaml`. It is not, necessarily. GoReleaser
// resolves its configuration by trying SIX paths in order when no `--config` is
// given — `.config/goreleaser.yml`, `.config/goreleaser.yaml`, `.goreleaser.yml`,
// `.goreleaser.yaml`, `goreleaser.yml`, `goreleaser.yaml`
// (goreleaser/v2@v2.17.1, cmd/config.go's loadConfigCheck) — and
// `.github/workflows/release.yml` runs `goreleaser release --clean` with no
// `--config` at all. So four of those six paths SHADOW the file this gate spent
// eight corrections learning to parse, and two of them are inside a dot
// directory.
//
// Measured, with the pinned tool, in a scratch repository holding all three of
// `.config/goreleaser.yml`, `.goreleaser.yml` and `.goreleaser.yaml`:
// `goreleaser check` reports `checking path=.config/goreleaser.yml`; delete that
// and it reports `.goreleaser.yml`; delete that and it reports `.goreleaser.yaml`.
// Dropping a `.goreleaser.yml` into THIS repository — a copy of the real file
// with `{{.Version}}` changed to `{{.Tag}}` — left `go test ./...` green across
// the root module: the published binary would print `dossierx version v0.5.0`
// where the site depicts `0.5.0`, and every assertion below would have been made,
// correctly, about a file the release does not open.
//
// That is the class this file exists to remove, reached by moving the SUBJECT
// rather than by editing it, and no amount of careful parsing closes it: parsing
// the wrong document perfectly is still reading the wrong document. So the
// subject is no longer a constant. gateRequireGoreleaserConfigIsLoaded derives it
// from the workflow that publishes — the step that runs GoReleaser, its `--config`
// argument if it has one, and GoReleaser's own search order if it does not — and
// every read of the configuration in this file goes through gateLoadGoreleaser,
// which calls it first. A second candidate existing at all is a failure, because
// which one governs is then a question about a search order rather than about
// this repository.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// gateReleasesArrayRE isolates the `releases` array in content.ts, so the
// assertions below judge the release entries and not the prose elsewhere in the
// file (which discusses the deleted field at length, on purpose).
var gateReleasesArrayRE = regexp.MustCompile(`(?s)const releases: Release\[\] = \[.*?\n\];`)

// gateCommitKeyRE is a `commit:` object key. It is anchored to the start of a
// line so that the word "commit" in a highlight's prose — every release entry is
// full of them — is not mistaken for the field.
var gateCommitKeyRE = regexp.MustCompile(`(?m)^\s*commit:`)

// TestSiteReleaseEntriesCarryNoCommitStamp is the deletion, held.
func TestSiteReleaseEntriesCarryNoCommitStamp(t *testing.T) {
	root := surfaceRepoRoot(t)
	content := gateReadRepoFile(t, root, filepath.Join("site", "src", "content.ts"))

	array := gateReleasesArrayRE.FindString(content)
	if array == "" {
		t.Fatal("site/src/content.ts no longer declares a `const releases: Release[] = [ ... ];` array; this assertion has lost its subject")
	}
	if hits := gateCommitKeyRE.FindAllString(array, -1); len(hits) > 0 {
		t.Errorf("%d release entr(ies) in site/src/content.ts carry a `commit` field. "+
			"It was deleted because it cannot converge — writing the sha is itself a commit, so the value is stale the moment it lands — "+
			"and because the binary spells the same value as a forty-character sha. Delete it; do not fill it in.", len(hits))
	}
	if strings.Contains(content, `"(devel)"`) {
		t.Error("site/src/content.ts still carries a \"(devel)\" fallback. The binary cannot print that value — resolveVersionInfo excludes it — so depicting it would be depicting output no user can see")
	}
}

// gateCommitReadRE is a READ of the deleted field: `.commit` on some receiver.
// The receiver is CAPTURED rather than ignored, because one spelling has to be
// allowed through and it must be allowed by name — see gateCommitReadExempt.
// The word boundary keeps it off `.commitment`, and a declaration (`commit?:`)
// is not a read and does not match.
var gateCommitReadRE = regexp.MustCompile(`(\w+)\.commit\b`)

// gateCommitReadExempt is the one receiver that is not the site's field.
//
// `main.commit` is the Go LDFLAGS SYMBOL — the variable GoReleaser stamps with
// the full forty-character sha — and the prose explaining why the site's field
// was deleted cannot make its argument without naming it: the two disagreeing by
// construction, seven characters against forty, is half the reason the field is
// gone. Exempting it by name is what lets the explanation live next to the
// assertion instead of being softened to get past it.
//
// It is a receiver name and not a substring: `release.commit` is caught,
// `main.commit` is not, and nothing else is exempt.
func gateCommitReadExempt(receiver string) bool { return receiver == "main" }

// TestNoSiteFileReadsTheDeletedCommitField is the deletion held across the WHOLE
// site rather than in the one file the field was written in.
//
// The narrower assertion above reads content.ts, because that is where the
// entries live and where the transcript that rendered the field lived. But a
// deleted field comes back through whoever wants to render it, and that is a
// component, not the data file: the next person to want a sha on the release
// timeline writes `release.commit` in a `.tsx` file, discovers the entries do not
// carry one, and adds it back to content.ts to feed what they just wrote. The
// entry-level check fires only at the END of that sequence, after the reader
// exists and the field has a reason to.
//
// So the assertion is about READERS, over every tracked file the site ships. A
// field nothing reads is a field nobody has a reason to re-add.
func TestNoSiteFileReadsTheDeletedCommitField(t *testing.T) {
	root := surfaceRepoRoot(t)

	var scanned int
	for _, file := range surfaceTrackedFiles(t, root) {
		if !strings.HasPrefix(file, "site/src/") {
			continue
		}
		scanned++
		body := gateReadRepoFile(t, root, filepath.FromSlash(file))
		for _, hit := range gateCommitReadRE.FindAllStringSubmatch(body, -1) {
			if gateCommitReadExempt(hit[1]) {
				continue
			}
			t.Errorf("%s reads %s — the release entries' `commit` field, which was deleted for naming the wrong sha two releases running and which no entry carries. "+
				"Nothing on the site may read it: a reader is what gives the field a reason to come back, and it comes back as a seven-character sha the binary spells with forty", file, hit[0])
		}
	}
	// The scan has to have had something to scan. An empty file list would make
	// the loop above pass over zero comparisons, which is the shape of green this
	// gate exists to refuse.
	if scanned == 0 {
		t.Fatal("no tracked file under site/src/ was scanned; this assertion passed over nothing")
	}
}

// gateCommitDeclarationRE is a TypeScript property DECLARATION of the deleted
// field — `commit?: string` or `commit: string` — as it would appear on an
// interface. It is anchored to the start of a line so that prose about a commit,
// of which the release entries carry a great deal, cannot match.
var gateCommitDeclarationRE = regexp.MustCompile(`(?m)^\s*commit\??\s*:\s*string`)

// TestNoSiteTypeDeclaresTheDeletedCommitField is the third leg of the deletion,
// and it is here because the first two both passed while the field was still
// half alive.
//
// When the field was removed, three of its four parts went: the data on the
// entries, the transcript that read it, and the RELEASING.md step that wrote it.
// The DECLARATION stayed — `commit?: string` on ReleaseTimeline's `Release`
// interface, under a doc comment that still described it as feeding the
// `dossierx version` example, which by then it did not.
//
// Neither assertion next door could see it. TestSiteReleaseEntriesCarryNoCommitStamp
// reads the `releases` array, and an unused optional property puts nothing in
// the array. TestNoSiteFileReadsTheDeletedCommitField looks for READS, and a
// declaration is not a read. So the field survived in the one form that matters
// most for its return: optional means `commit: "abc1234"` on a new entry
// type-checks silently, which is precisely how the next person re-adds it — the
// compiler, the only thing that would have objected, had been told in advance
// not to.
//
// A deleted field's type declaration is not a leftover. It is a standing offer.
func TestNoSiteTypeDeclaresTheDeletedCommitField(t *testing.T) {
	root := surfaceRepoRoot(t)

	var scanned int
	for _, file := range surfaceTrackedFiles(t, root) {
		if !strings.HasPrefix(file, "site/src/") {
			continue
		}
		scanned++
		body := gateReadRepoFile(t, root, filepath.FromSlash(file))
		if hit := gateCommitDeclarationRE.FindString(body); hit != "" {
			t.Errorf("%s declares %q — the release entries' `commit` field, whose data, reader and release step were all deleted. "+
				"The declaration outliving them is what makes re-adding the field type-check: it is optional, so an entry carrying one compiles clean and the field is back before anything objects. Delete the declaration too",
				file, strings.TrimSpace(hit))
		}
	}
	if scanned == 0 {
		t.Fatal("no tracked file under site/src/ was scanned; this assertion passed over nothing")
	}
}

// gateSiteDeclaration is one TypeScript declaration this file MODELS and cannot
// evaluate, held against the source it models.
//
// There are three, and each is a duplicate on purpose: this test resolves the
// site's transcript without a TypeScript runtime, so it has to know which entry
// the page selects, which entry the timeline badges "latest", and how the site
// spells the string the BINARY prints. A duplicate that is not held is exactly
// how a guard stops describing its subject.
//
// HOW they are held is correction SIX, and it is the whole of this type. They
// used to be `strings.Contains` over the entire file, which is the shape
// correction FIVE dismantled one file over: a comment is a substring. Commenting
// the pinned line out and writing the wrong expression underneath it left the
// pinned text in the file and the whole suite green, on all three declarations
// independently — `releases[0]` for the current release, `0` for the timeline's
// latest index, and a `latestBinaryVersion` that stops stripping the `v` and so
// puts `dossierx version v0.5.0` back on the page.
//
// So the file's COMMENTS ARE STRIPPED before the search, which makes a
// commented-out original evidence of nothing, and the declaration must be the
// stripped file's ONLY one, which makes a second live declaration a failure
// rather than a match on whichever came first. Those two together are what a
// reader — and a bundler — actually act on.
type gateSiteDeclaration struct {
	rel      string         // repo-relative file the declaration must live in
	landmark string         // structure that must survive comment stripping; see gateRequireSiteDeclarations
	head     *regexp.Regexp // the declaration, from its `const` through its `;`
	want     string         // what it must say
	why      string         // what the deployed page does when it says something else
}

// gateSiteDeclarations are the three, and the two selection expressions are held
// TOGETHER because the contradiction needs both: either one moving alone is what
// makes the page disagree with itself, and a guard holding one of them cannot
// tell which moved.
var gateSiteDeclarations = []gateSiteDeclaration{
	{
		rel:      filepath.Join("site", "src", "content.ts"),
		landmark: "const releases: Release[] = [",
		head:     regexp.MustCompile(`(?:export\s+)?const\s+latestRelease\s*:[^;]*;`),
		want:     `export const latestRelease: Release = releases[releases.length - 1];`,
		why: "This test reads the LAST `version:` literal in the `releases` array because that is the entry the page renders. If the page selects a different one, every version string on it — the hero kicker, the hero badge, " +
			"the release-history intro and the `dossierx version` transcript — depicts THAT release while ReleaseTimeline goes on badging the last entry \"latest\", and this test would go on judging the transcript against this one",
	},
	{
		rel:      filepath.Join("site", "src", "components", "ReleaseTimeline.tsx"),
		landmark: "export function ReleaseTimeline(",
		head:     regexp.MustCompile(`(?:export\s+)?const\s+latestIndex\b[^;]*;`),
		want:     `const latestIndex = releases.length - 1;`,
		why:      "The timeline and content.ts must agree on which release is current. When they do not, the page badges one entry \"latest\" while every derived string on it names another — a page that contradicts itself, with each half correct about the wrong entry",
	},
	// A THIRD DECLARATION USED TO BE HELD HERE and is now REFUSED instead, by
	// TestSiteDeclaresNoSecondVersionSpelling below. It was
	//
	//   export const latestBinaryVersion: string = latestRelease.version.replace(/^v/, "");
	//
	// and it existed because the two install paths disagreed: the archive stamped
	// `{{.Version}}` and printed `0.5.1`, while `go install …@v0.5.1` took the tag
	// verbatim from the module proxy and printed `v0.5.1`. The site needed both
	// spellings to depict either honestly. v0.5.2 moved the stamp to `{{.Tag}}`,
	// so there is one spelling and a second constant would be a copy that can go
	// stale on its own.
}

// TestSiteSelectsTheReleaseThisTreeModels is the three declarations, held.
//
// It used to be a step INSIDE the transcript comparison, because that comparison
// modelled the site's selection and derivation in Go and could not be allowed to
// run against a stale model. The comparison has moved: the transcript is now read
// as RENDERED DOM in viewer-tests/site_dom_test.go, where nothing is modelled
// because the page has already evaluated itself.
//
// These three are what a rendered read STRUCTURALLY CANNOT SEE, which is why they
// stayed behind rather than moving with it. Two of them do show up in the DOM
// eventually — a page selecting `releases[0]` renders the oldest release's version
// into the transcript, and a `latestBinaryVersion` that stops stripping the `v`
// renders `dossierx version v<x.y.z>`, and the rendered check catches both. The
// THIRD does not: ReleaseTimeline's `latestIndex` decides which entry is badged
// "latest", and a timeline badging one entry while every derived string names
// another is a page that contradicts itself with both halves rendering perfectly.
// No read of the transcript can see that, so it is asserted here.
func TestSiteSelectsTheReleaseThisTreeModels(t *testing.T) {
	gateRequireSiteDeclarations(t, surfaceRepoRoot(t))
}

// TestSiteDeclaresNoSecondVersionSpelling refuses the constant that v0.5.2
// deleted, because deleting it is only half the fix.
//
// WHAT IT WAS. `latestBinaryVersion` was `latestVersion` with the leading `v`
// stripped, and it was CORRECT while it existed: `.goreleaser.yaml` stamped
// `-X main.version={{.Version}}`, so the published archive printed
// `dossierx version 0.5.1`, while `go install …@v0.5.1` applies no ldflags at
// all and falls back to `debug.ReadBuildInfo`, which the module proxy fills with
// the tag verbatim — `v0.5.1`. One release answered the question two ways
// depending on how it was installed, and the site kept two constants so it could
// depict whichever the reader would see.
//
// Issue #38 fixed the cause: the stamp is `{{.Tag}}` and both paths print the
// tag as tagged. The second constant is therefore not merely redundant, it is a
// derivation of a spelling nothing produces — and reintroducing one would put
// `dossierx version <x.y.z>` back into a transcript no install path matches,
// which is the defect this whole file was opened for, arrived at from the other
// direction.
//
// It is refused BY NAME rather than by scanning for a stripped literal, because
// the literal check already exists elsewhere and answers a different question:
// viewer-tests/site_source_test.go hunts a hand-typed bare version anywhere in
// the source, and this one refuses the reintroduction of the derivation that
// would generate one on every release without a literal ever being typed.
func TestSiteDeclaresNoSecondVersionSpelling(t *testing.T) {
	const rel = "site/src/content.ts"
	code := gateStripTSComments(gateReadRepoFile(t, surfaceRepoRoot(t), filepath.FromSlash(rel)))

	// Comments are stripped for the reason gateRequireSiteDeclarations strips
	// them, plus one specific to this test: content.ts now carries a block
	// comment explaining why the constant is gone, and that explanation has to be
	// able to NAME it.
	if !strings.Contains(code, "const releases: Release[] = [") {
		t.Fatalf("stripping comments from %s removed the releases array, which is not a comment. The lexer has mis-read the source — an unclosed literal, most likely — so the search below would run over a truncated file and pass over nothing", rel)
	}

	stripped := regexp.MustCompile(`(?:export\s+)?const\s+\w+\s*:[^;]*\.replace\(\s*/\^v/`)
	if found := stripped.FindAllString(code, -1); len(found) > 0 {
		t.Fatalf("%s declares a version constant derived by stripping the leading `v`:\n\t%s\n\n"+
			"There is one version spelling in this project and it is the tag as tagged. Since v0.5.2 `.goreleaser.yaml` stamps `-X main.version={{.Tag}}`, and `go install` takes the same string verbatim from the module proxy, so BOTH install paths print `v<x.y.z>` — a stripped derivation depicts output neither one produces.\n\n"+
			"This constant existed until v0.5.2 and was correct then, because the archive really did print the stripped form while `go install` printed the tag. If a release ever needs two spellings again, the cause is in .goreleaser.yaml, not here: fix it there, and gateRequireReleaseTransform will tell you which template moved.",
			rel, strings.Join(found, "\n\t"))
	}
}

// gateRequireSiteDeclarations holds every one of them.
//
// The landmark is a stripper self-check, not an assertion about the site. This
// file lexes TypeScript with a small state machine, and the failure mode of a
// small lexer is swallowing code it mistook for a literal — which would show up
// as "the declaration is missing" and send a maintainer to look at a file that
// is perfectly correct. If the landmark did not survive the strip, the strip is
// what is wrong, and it says so in those words.
func gateRequireSiteDeclarations(t *testing.T, root string) {
	t.Helper()

	for _, decl := range gateSiteDeclarations {
		code := gateStripTSComments(gateReadRepoFile(t, root, decl.rel))
		if !strings.Contains(code, decl.landmark) {
			t.Fatalf("stripping comments from %s removed %q, which is not a comment. This file's TypeScript lexer has mis-read the source — a literal it did not close, most likely — so the search below would be running over a truncated file. "+
				"Fix gateStripTSComments; the site is not what is wrong here", decl.rel, decl.landmark)
		}

		found := decl.head.FindAllString(code, -1)
		switch len(found) {
		case 1:
			// The only live declaration, which is what is required.
		case 0:
			t.Fatalf("%s declares nothing matching\n\t%s\noutside its comments. A commented-out declaration is not one: the page would not compile, and if it did it would not select what this file models. %s",
				decl.rel, decl.want, decl.why)
		default:
			t.Fatalf("%s carries %d live declarations of that name — %q. Only one of them is what the page evaluates, and this file cannot tell which; a correct declaration standing beside a wrong one is a wrong page that reads as a right one",
				decl.rel, len(found), found)
		}

		if got, want := gateCollapseSpace(found[0]), gateCollapseSpace(decl.want); got != want {
			t.Fatalf("%s declares\n\t%s\nwhere this file models\n\t%s\n%s", decl.rel, got, want, decl.why)
		}
	}
}

// gateStripTSComments removes `//` line comments and `/* */` block comments from
// TypeScript source, leaving string, template and regex literals alone.
//
// It is a lexer and not a regexp because the two things it must not confuse are
// both spelled with a slash: the `https://` inside a string is not a comment,
// and `/^v/` — the regex in `latestBinaryVersion`'s own derivation — is not one
// either. Newlines inside stripped comments are preserved so the shape of the
// remaining source is unchanged.
//
// Its failure mode is swallowing, never inventing: a literal it fails to close
// eats the rest of the file, which removes declarations and fails loudly, rather
// than exposing commented-out text and passing quietly. gateRequireSiteDeclarations
// names that case explicitly through its landmark.
func gateStripTSComments(src string) string {
	const (
		code = iota
		lineComment
		blockComment
		single
		double
		template
	)

	var out strings.Builder
	out.Grow(len(src))
	state := code

	for i := 0; i < len(src); i++ {
		c := src[i]
		var next byte
		if i+1 < len(src) {
			next = src[i+1]
		}

		switch state {
		case code:
			switch {
			case c == '/' && next == '/':
				state, i = lineComment, i+1
			case c == '/' && next == '*':
				state, i = blockComment, i+1
			case c == '\'':
				state = single
				out.WriteByte(c)
			case c == '"':
				state = double
				out.WriteByte(c)
			case c == '`':
				state = template
				out.WriteByte(c)
			default:
				out.WriteByte(c)
			}
		case lineComment:
			if c == '\n' {
				state = code
				out.WriteByte(c)
			}
		case blockComment:
			switch {
			case c == '*' && next == '/':
				state, i = code, i+1
			case c == '\n':
				out.WriteByte(c)
			}
		default: // inside a string, template or the middle of one
			out.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				out.WriteByte(src[i])
				continue
			}
			if (state == single && c == '\'') || (state == double && c == '"') || (state == template && c == '`') {
				state = code
			}
		}
	}
	return out.String()
}

// gateCollapseSpace reduces every run of whitespace to one space, so that a
// formatter reflowing a declaration onto two lines is not a failure. What the
// declaration SAYS is the assertion; how it is wrapped is not.
func gateCollapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// gateGoreleaserFile is the release build's configuration, relative to the
// repository root.
//
// It is this repository's INTENT and not this file's subject. What the subject
// IS gets derived from the publishing workflow by
// gateRequireGoreleaserConfigIsLoaded below, and the two are then required to be
// the same path — because they came apart once and nothing noticed. Renaming the
// configuration is a one-line edit here; putting a SECOND one beside it is a
// failure, and that is the difference this constant no longer has to carry alone.
const gateGoreleaserFile = ".goreleaser.yaml"

// gateGoreleaserCandidates is GoReleaser's own configuration search order, in
// GoReleaser's own order, for an invocation that passes no `--config`.
//
// Copied from the pinned tool rather than remembered: goreleaser/v2@v2.17.1,
// cmd/config.go, loadConfigCheck's `for _, f := range [6]string{...}`. The order
// is load-bearing and is not the obvious one — `.goreleaser.yml` is tried BEFORE
// `.goreleaser.yaml`, and both are preceded by two paths inside a `.config`
// directory. Verified against the tool itself in a scratch repository: with all
// three present, `goreleaser check` reports `path=.config/goreleaser.yml`.
//
// This list is a MODEL of another program's behaviour, which is the shape of
// thing this file has been wrong about before. Two things hold it: the rule
// below refuses to let more than one of these exist at all, so the order only
// has to be right about which single file is found; and viewer-tests'
// TestGoreleaserResolvesTheConfigurationThisGateReads asks the tool itself which
// path it loads, wherever the tool is available.
var gateGoreleaserCandidates = []string{
	".config/goreleaser.yml",
	".config/goreleaser.yaml",
	".goreleaser.yml",
	".goreleaser.yaml",
	"goreleaser.yml",
	"goreleaser.yaml",
}

// gateWorkflowDir holds every workflow this repository can run.
//
// IT IS A DIRECTORY AND NOT A FILENAME, which is correction NINE applied to
// itself. Deriving the subject from `.github/workflows/release.yml` fixes the
// attack that moved the CONFIGURATION and leaves the identical attack one level
// up: GitHub runs every workflow whose trigger matches, so a second file with
// `on: push: tags: ['v*']` publishes alongside the first, and a check that reads
// one named workflow would go on describing the release it was pointed at while
// another one shipped. Every workflow in this directory is parsed and every
// GoReleaser invocation in all of them is collected, so a second publisher is a
// failure rather than an unread file.
const gateWorkflowDir = ".github/workflows"

// gateGoreleaserAction and gateGoreleaserCommand are the two ways a workflow can
// run this tool: the published action, or the binary in a `run:` body. Both are
// looked for, so that swapping one for the other cannot move the release build
// out from under this check the way moving the configuration did.
//
// gateGoreleaserPublish is the subcommand that PUBLISHES. It is required because
// this repository runs GoReleaser for two other reasons — `check`, and a
// `--version` probe in ci.yml's viewer job — and counting those as release builds
// would make this check report three publishers in a tree that has one. What is
// being counted is the thing that ships artifacts, not the thing that runs the
// binary.
const (
	gateGoreleaserAction  = "goreleaser/goreleaser-action@"
	gateGoreleaserCommand = "goreleaser"
	gateGoreleaserPublish = "release"
)

// gateCommandName resolves the program a parsed command actually runs.
//
// POSIX lets a command be preceded by any number of `VAR=value` assignments, and
// a line that is NOTHING but assignments runs no program at all. Both matter
// here, and the second is not hypothetical: ci.yml's GoReleaser resolver opens
// with `g="$(go env GOPATH)/bin/goreleaser"`, whose single token ends in
// `/goreleaser`. Read as a command name that is an invocation of the release
// tool; read as what it is, it is a variable being set. The first version of this
// helper made that mistake and reported the repository as having three
// publishers.
//
// The returned name is the basename, so `/usr/local/bin/goreleaser` and
// `goreleaser` are the same program, and `goreleaser.exe` is too.
func gateCommandName(argv []string) (name string, args []string, ok bool) {
	for i, arg := range argv {
		if eq := strings.IndexByte(arg, '='); eq > 0 && gateIsIdentifier(arg[:eq]) {
			continue
		}
		name = arg
		if slash := strings.LastIndexAny(name, "/\\"); slash >= 0 {
			name = name[slash+1:]
		}
		return strings.TrimSuffix(name, ".exe"), argv[i+1:], true
	}
	return "", nil, false
}

// gateIsIdentifier reports whether s is a shell variable name, which is what
// tells `FOO=bar cmd` from `./cmd=weird`.
func gateIsIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// gatePublishes reports whether a GoReleaser argument list is a RELEASE.
//
// The subcommand is a bare token — `release --clean`, or `--config x release` —
// so bare tokens are what is read, with the one flag that takes a path skipped so
// a configuration file named `release` could not be mistaken for the verb.
func gatePublishes(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--config" || arg == "-f" {
			i++ // its value is a path, not a subcommand
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == gateGoreleaserPublish {
			return true
		}
	}
	return false
}

// gateWorkflowStep and gateWorkflow are the parts of a workflow this file reads.
// `with` is `any`-valued because a workflow's scalars are a mix of strings,
// integers and booleans and a stricter type would fail to parse a valid file.
//
// This is a SECOND workflow model in this repository — tests/ci_workflow_test.go
// has one — and the duplication is deliberate rather than overlooked. That file's
// subject is ci.yml and this one's is release.yml; they are different documents
// answering different questions, and the alternative to two small readers is an
// internal package that both import, which puts a non-test package in the engine's
// build for the sake of thirty lines.
type gateWorkflowStep struct {
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

type gateWorkflow struct {
	Jobs map[string]struct {
		Steps []gateWorkflowStep `yaml:"steps"`
	} `yaml:"jobs"`
}

// gateArgv splits a command line into arguments the way a runner would: runs of
// whitespace separate, and single or double quotes group. It is a PARSE of the
// one format a command line is, not a search for a word — `--config` inside a
// quoted string is one argument, and the word "--config" in a comment is not an
// argument at all because a comment never reaches this function.
//
// It is deliberately not a shell: no expansion, no operators, no escapes beyond
// quoting. A workflow's `args:` that needed any of those would be a release
// invocation nothing here could read, and gateReleaseConfigFlag says so rather
// than guessing.
func gateArgv(line string) []string {
	var (
		out   []string
		cur   strings.Builder
		open  bool
		quote byte
	)
	flush := func() {
		if open || cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			cur.WriteByte(c)
		case c == '\'' || c == '"':
			quote, open = c, true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
			open = false
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
}

// gateReleaseConfigFlag reads a GoReleaser argument list and returns the path its
// `--config`/`-f` names, if it names one.
//
// All four spellings the flag has, because a check that read one of them would be
// defeated by typing another: `--config X`, `--config=X`, `-f X`, `-fX`.
func gateReleaseConfigFlag(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-f":
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true // named with nothing after it; the caller reports it
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config="), true
		case strings.HasPrefix(arg, "-f") && len(arg) > 2 && !strings.HasPrefix(arg, "-f-"):
			return arg[2:], true
		}
	}
	return "", false
}

// gateReleaseInvocation returns the arguments the publishing workflow passes to
// GoReleaser, and the step it found them on.
//
// EXACTLY ONE STEP IN THE WHOLE WORKFLOW DIRECTORY MAY RUN IT. Two would be two
// release builds from two possibly different configurations, and this file could
// not say which one publishes — which is not a thing to settle by taking the
// first. Zero means the release is built by something this check cannot see, and
// that is reported as a failure rather than as nothing to check.
func gateReleaseInvocation(t *testing.T, root string) (args []string, where string) {
	t.Helper()

	dir := filepath.Join(root, filepath.FromSlash(gateWorkflowDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v\nThis file derives WHICH configuration the release build loads from the workflows in that directory. Without them there is nothing to derive it from, and the configuration this gate parses would be a filename it chose for itself",
			gateWorkflowDir, err)
	}

	type found struct {
		args  []string
		where string
	}
	var hits []found
	var read []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if ext := filepath.Ext(entry.Name()); ext != ".yml" && ext != ".yaml" {
			continue
		}
		rel := gateWorkflowDir + "/" + entry.Name()
		read = append(read, entry.Name())

		var wf gateWorkflow
		raw := gateReadRepoFile(t, root, filepath.FromSlash(rel))
		if unmarshalErr := yaml.Unmarshal([]byte(raw), &wf); unmarshalErr != nil {
			t.Fatalf("parse %s as YAML: %v\nEvery workflow here is read because any of them can publish a release. A workflow this file cannot read is a release path it cannot rule out, so it fails rather than skipping the file",
				rel, unmarshalErr)
		}
		if len(wf.Jobs) == 0 {
			t.Fatalf("%s declares no jobs. Either the top-level `jobs:` key moved or this file's model of a workflow no longer matches the document; either way this file would have read that workflow and found nothing in it, which is indistinguishable from a workflow that does nothing",
				rel)
		}

		names := make([]string, 0, len(wf.Jobs))
		for name := range wf.Jobs {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, job := range names {
			for _, step := range wf.Jobs[job].Steps {
				at := rel + " job " + job + ", step " + step.Name
				if strings.Contains(step.Uses, gateGoreleaserAction) {
					stepArgs := gateArgv(gateScalar(step.With["args"]))
					if !gatePublishes(stepArgs) {
						continue // the action installed, or ran `check`; it did not release
					}
					// The action's `workdir` changes the directory GoReleaser
					// runs in, and therefore the directory its six candidate
					// paths are resolved against. A workdir this file did not
					// read would make every path below relative to somewhere
					// else.
					if wd := strings.TrimSpace(gateScalar(step.With["workdir"])); wd != "" && wd != "." {
						t.Fatalf("%s runs GoReleaser with `workdir: %s`. Its configuration is then resolved inside that directory, and every path this file checks is relative to the repository root — so the file the release loads is not one this gate has read at all",
							at, wd)
					}
					hits = append(hits, found{args: stepArgs, where: at})
					continue
				}
				// A `run:` body that invokes the binary. Parsed as commands and
				// argv rather than searched for the word, so that
				// `echo goreleaser` is an echo, a commented-out invocation is
				// not a command, and `g="$(go env GOPATH)/bin/goreleaser"` is an
				// assignment.
				for _, argv := range gateCommands(step.Run) {
					name, rest, ok := gateCommandName(argv)
					if !ok || name != gateGoreleaserCommand || !gatePublishes(rest) {
						continue
					}
					hits = append(hits, found{args: rest, where: at})
				}
			}
		}
	}

	if len(read) == 0 {
		t.Fatalf("%s holds no workflow files at all. Either this repository no longer releases from CI — in which case nothing here describes how it releases — or the directory moved and this check has lost its subject", gateWorkflowDir)
	}

	switch len(hits) {
	case 1:
		return hits[0].args, hits[0].where
	case 0:
		t.Fatalf("no step in any of %s's %d workflow(s) runs a GoReleaser `%s` — none uses `%s` with that subcommand and no `run:` body invokes `%s %s`. Files read: %v.\n"+
			"Other GoReleaser subcommands do not count here on purpose: `check` validates a configuration and `--version` prints a banner, and neither ships an artifact — so a workflow that only runs those is a repository that publishes by some other means, which is exactly the thing this check exists to notice.\n"+
			"Everything in this file describes what the release build is told to do, and it learns WHICH file it is told that in from the workflows. With no invocation to read, the configuration it parses would be a filename this file chose for itself.\n"+
			"This is a FAILED check and not an empty one: a gate that cannot find the release build has not certified it",
			gateWorkflowDir, len(read), gateGoreleaserPublish, gateGoreleaserAction, gateGoreleaserCommand, gateGoreleaserPublish, read)
	default:
		var at []string
		for _, hit := range hits {
			at = append(at, hit.where)
		}
		t.Fatalf("%d steps across %s run GoReleaser (%v).\n"+
			"GitHub runs every workflow whose trigger matches, so two publishers are two releases — each able to name its own configuration — and this file cannot say which one a tag produces. Picking the first would be an answer nobody decided.\n"+
			"CLAUDE.md: there is exactly one description of how this project releases; there is exactly one thing that performs it too",
			len(hits), gateWorkflowDir, at)
	}
	return nil, ""
}

// gateCommands splits a shell body into the commands it runs, as argv lists.
//
// Lines are split on `&&`, `||`, `;` and `|`, comments are dropped, and each
// command is tokenised by gateArgv. This is a PARSE of a command line and not a
// search for a word, which is the distinction the whole file rests on: `echo
// "goreleaser release"` parses as an `echo` with one argument, and a `#`-prefixed
// line parses as nothing at all.
func gateCommands(body string) [][]string {
	var out [][]string
	for _, line := range strings.Split(body, "\n") {
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			// Only when the `#` starts a word; `foo#bar` is not a comment.
			if hash == 0 || line[hash-1] == ' ' || line[hash-1] == '\t' {
				line = line[:hash]
			}
		}
		for _, part := range gateSplitOperators(line) {
			if argv := gateArgv(part); len(argv) > 0 {
				out = append(out, argv)
			}
		}
	}
	return out
}

// gateSplitOperators cuts one line at the operators that end a command.
func gateSplitOperators(line string) []string {
	parts := []string{line}
	for _, op := range []string{"&&", "||", ";", "|"} {
		var next []string
		for _, part := range parts {
			next = append(next, strings.Split(part, op)...)
		}
		parts = next
	}
	return parts
}

// gateScalar renders a `with:` value as the string a runner would see. A
// workflow's scalars are a mix of strings, integers and booleans, and telling
// `'20'` from `20` would be asserting a quoting style.
func gateScalar(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// gateRequireGoreleaserConfigIsLoaded proves that the file this gate reads is the
// file the release build opens, and is called before every read of it.
//
// IT IS THE ANSWER TO CORRECTION NINE. Eight corrections taught this file to
// parse `.goreleaser.yaml` instead of searching it; none of them asked whether
// `.goreleaser.yaml` was the document GoReleaser would load. It is one of six
// candidates and the fourth in the order, so four other filenames take the
// release over while every assertion here goes on being made — correctly, in
// detail — about a file nothing opens.
//
// Two facts are established, in this order:
//
//   - WHAT THE PUBLISH PATH ASKS FOR. Read out of the publishing workflow's
//     GoReleaser step. If it names a `--config`, that path is the subject, full
//     stop; if it names none, GoReleaser's own six-path search order decides, and
//     the search is performed here the way the tool performs it.
//   - THAT NOTHING ELSE COULD HAVE BEEN CHOSEN. Every candidate path is stat'ed,
//     and a second one existing is a failure even when the first is correct.
//     Otherwise this check would be a model of a search order — the exact kind of
//     unread model that put `{{.Tag}}` in a shadowing file and left the suite
//     green.
func gateRequireGoreleaserConfigIsLoaded(t *testing.T, root string) {
	t.Helper()

	args, where := gateReleaseInvocation(t, root)

	if named, ok := gateReleaseConfigFlag(args); ok {
		if named == "" {
			t.Fatalf("%s passes a `--config` with nothing after it (args: %v). GoReleaser would refuse the invocation, and this file cannot say which configuration a release is built from", where, args)
		}
		clean := filepath.ToSlash(filepath.Clean(named))
		if clean != gateGoreleaserFile {
			t.Fatalf("%s builds the release from %q, where this gate reads %q.\n"+
				"Every assertion in this file — the ldflags, the six archives, the archive names, the checksum file — would then be made about a document the release does not open. Point this file at the configuration the release uses, or the release at this one; they may not be two files",
				where, clean, gateGoreleaserFile)
		}
		return
	}

	// No --config: GoReleaser searches, and so does this.
	var present []string
	for _, candidate := range gateGoreleaserCandidates {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate))); err == nil {
			present = append(present, candidate)
		}
	}

	switch {
	case len(present) == 0:
		t.Fatalf("%s runs GoReleaser with no `--config`, and NONE of its six candidate paths exists in this repository: %v.\n"+
			"GoReleaser would fall back to its built-in defaults — no ldflags, no archive names, no checksum file — and publish something nothing in this tree describes",
			where, gateGoreleaserCandidates)
	case len(present) > 1:
		t.Fatalf("%s runs GoReleaser with no `--config`, and %d of its candidate paths exist: %v.\n"+
			"GoReleaser tries them in the order %v and loads the FIRST, so %q is the file that would publish this release and the others are read by nobody — including by this gate, which reads %q.\n"+
			"That is not a preference about filenames. A copy of the configuration with one template changed, dropped in at a path earlier in that order, publishes a binary whose version disagrees with the site while every assertion in this file stays green, because each is true of the document it read.\n"+
			"There is exactly one release configuration in this repository: delete the others",
			where, len(present), present, gateGoreleaserCandidates, present[0], gateGoreleaserFile)
	case present[0] != gateGoreleaserFile:
		t.Fatalf("%s runs GoReleaser with no `--config`, so it loads %q — the first of its candidate paths that exists — where this gate reads %q.\n"+
			"Every assertion below would be made about a file the release does not open. Either move the configuration back to %q or change gateGoreleaserFile to name the one that governs; there is no arrangement in which they are different files",
			where, present[0], gateGoreleaserFile, gateGoreleaserFile)
	}
}

// TestReleaseBuildLoadsTheConfigurationThisGateReads is the rule on its own, so
// that the failure arrives as a sentence about which file publishes rather than
// buried inside an assertion about ldflags.
func TestReleaseBuildLoadsTheConfigurationThisGateReads(t *testing.T) {
	gateRequireGoreleaserConfigIsLoaded(t, surfaceRepoRoot(t))
}

// gateVersionLdflag is the stamping line gateReleaseBinaryVersion models, quoted
// verbatim, for failure messages that can say what the file should have said.
const gateVersionLdflag = `-X main.version={{.Tag}}`

// gateVersionSymbol is the linker symbol that stamping must target.
//
// It is `main.version` and not the full import path because the linker names a
// MAIN package's symbols `main.<name>` — `.goreleaser.yaml`'s own comment
// records this — and an `-X` aimed at
// `github.com/BarterX-Tech/dossierx/cmd/dossierx.version` is accepted by the
// linker, recorded in the build settings, and stamps nothing.
const gateVersionSymbol = "main.version"

// gateVersionTemplate is the value that symbol must be stamped with, and
// gateStrippedTemplate is the other spelling — named so a refusal can say which
// one it found rather than only that the expected one is missing. `{{.Tag}}`
// keeps the leading `v`; `{{.Version}}` strips it, and swapping the two is a
// one-word edit that inverts every spelling this file compares.
//
// THE PAIR WAS THE OTHER WAY ROUND UNTIL v0.5.2, and the reason it turned over
// is issue #38. The archive stamped `{{.Version}}` and printed `0.5.1`, while
// `go install …@v0.5.1` took its version from the module proxy, which hands
// `debug.ReadBuildInfo` the tag verbatim and printed `v0.5.1`. The same release
// reported two different versions depending on how it was installed, and the
// hazard was never cosmetic: a scripted
// `dossierx version --format json | jq -r .data.version` compared against a
// `vX.Y.Z` tag succeeded via one install path and failed via the other.
//
// `{{.Tag}}` is canonical because it is the form everything else already uses —
// the git tag, `go install`'s argument, the CHANGELOG heading, and every version
// string the site renders. Nothing in cmd/dossierx changed to achieve it:
// resolveVersionInfo's no-ldflags fallback already produced the tag verbatim, so
// the two paths now converge rather than being reconciled.
const (
	gateVersionTemplate  = `{{.Tag}}`
	gateStrippedTemplate = `{{.Version}}`
)

// gateReleaseStamps is EVERY symbol the release build stamps, with the template
// each must be stamped with.
//
// All three, and that is correction SEVEN. This table used to be one symbol:
// `main.version` was refused by name when it was aimed at the full import path,
// and `main.commit` and `main.date` were not looked at. The linker treats them
// identically — an `-X` aimed at
// `github.com/BarterX-Tech/dossierx/cmd/dossierx.commit` is accepted, recorded in
// the build settings, and stamps nothing — so aiming those two at the import
// path left every assertion in this file green. Nothing downstream could see it
// either: with the flags applied to nothing the binary falls back to
// `debug.ReadBuildInfo`, whose `vcs.revision` IS a forty-character sha and whose
// `vcs.time` IS an RFC 3339 timestamp, which is precisely what the snapshot
// test's shape checks ask for. The published binary would report the commit's
// time where the release means the build's.
//
// The templates are pinned too, not only the symbols. `{{.ShortCommit}}` is
// seven characters where docs/RELEASING.md rests half its argument for deleting
// the site's `commit` field on forty, and `{{.CommitDate}}` is the commit's time
// rather than the build's — each is a one-word edit that makes a sentence in the
// procedure false while the binary goes on reporting something well-formed.
var gateReleaseStamps = []struct{ symbol, template, stands string }{
	{"main.version", gateVersionTemplate, "the tag EXACTLY AS TAGGED, leading `v` included — the one form the archive, `go install`, the site and the CHANGELOG heading all agree on, and what every comparison in this file rests on"},
	{"main.commit", "{{.Commit}}", "the full forty-character sha — the width docs/RELEASING.md contrasts with the seven the deleted site field carried"},
	{"main.date", "{{.Date}}", "the BUILD's RFC 3339 timestamp, which is why the site's transcript may not depict a `date:` line beside a calendar day"},
}

// gateGoreleaserConfig is the part of `.goreleaser.yaml` this file rests on,
// PARSED rather than searched for.
//
// The parse is the whole point, and it is a correction. Every assertion here
// used to be a `strings.Contains` over the entire document, and a document is
// not a configuration: `.goreleaser.yaml` carries a five-line prose comment
// about `-X main.version` needing the `main.` prefix, so the file CONTAINS the
// correct stamping line no matter what the build is actually told to do. Three
// separate mutations left the old form green — the flag replaced with the
// import-path spelling while the correct spelling stayed in the comment above
// it; the flag commented out in place as `# - -X main.version={{.Version}}`;
// and a second `-X main.version={{.Tag}}` appended after the good one, where
// the LAST assignment is the one the linker keeps. All three ship a binary
// whose version disagrees with the site, and a substring search cannot see any
// of them, because a comment is a substring and so is a line that has been
// turned into one.
//
// This is the same lesson gateLdflagsItemRE below was written for on
// docs/RELEASING.md — assert against the structure a reader (or a linker) acts
// on, never against the file that happens to contain it.
type gateGoreleaserConfig struct {
	Builds []struct {
		ID      string   `yaml:"id"`
		GOOS    []string `yaml:"goos"`
		GOARCH  []string `yaml:"goarch"`
		Ldflags []string `yaml:"ldflags"`
	} `yaml:"builds"`
	Archives []struct {
		ID           string   `yaml:"id"`
		IDs          []string `yaml:"ids"`
		NameTemplate string   `yaml:"name_template"`
		// Both spellings, because GoReleaser v2 accepts both and the two say
		// the same thing: `format:` is the singular this file was written
		// against and `formats:` is the plural that replaced it (the singular
		// is deprecated and fails `goreleaser check` on the pinned v2.17.1).
		// A model that reads only one of them reports the OTHER one as an
		// override to the empty string — a configuration that is correct and
		// a gate that says it is wrong, which is the failure mode this whole
		// file exists to avoid in the other direction.
		FormatOverrides []struct {
			GOOS    string   `yaml:"goos"`
			Format  string   `yaml:"format"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
	Checksum struct {
		NameTemplate string `yaml:"name_template"`
	} `yaml:"checksum"`
}

// gateLoadGoreleaser parses the release configuration and returns it, refusing
// anything but exactly one build.
//
// One build is not a stylistic preference: every count in this file and the
// "all six archives" line in docs/RELEASING.md are the product of ONE build's
// goos and goarch lists. A second build entry multiplies the artifacts without
// changing either list, so the counts would go on being asserted correctly
// about a release that no longer matches them.
//
// IT PROVES THE FILE BEFORE IT PARSES IT. gateRequireGoreleaserConfigIsLoaded
// runs first, on every single read, rather than in a test of its own — because a
// test of its own would fail beside a dozen assertions that had already passed
// over the wrong document, and "the gate was green except for one test nobody
// wired to the others" is how correction FIVE's model went stale in the first
// place. There is no path to the bytes of this configuration that has not first
// established that a release opens them.
func gateLoadGoreleaser(t *testing.T, root string) gateGoreleaserConfig {
	t.Helper()
	gateRequireGoreleaserConfigIsLoaded(t, root)

	var config gateGoreleaserConfig
	if err := yaml.Unmarshal([]byte(gateReadRepoFile(t, root, gateGoreleaserFile)), &config); err != nil {
		t.Fatalf("parse %s: %v — every assertion about what the release builds reads this file as YAML, and it no longer parses", gateGoreleaserFile, err)
	}
	if len(config.Builds) != 1 {
		t.Fatalf("%s declares %d builds, not 1. Every artifact count here and the \"all six archives\" line in docs/RELEASING.md is one build's goos list times its goarch list; a second build changes what is published without touching either list",
			gateGoreleaserFile, len(config.Builds))
	}
	return config
}

// gateXAssignments collects the linker's `-X symbol=value` assignments out of a
// build's ldflags list, keyed by symbol, keeping EVERY value for a symbol
// rather than the first or the last.
//
// Keeping every value is the point. `-ldflags` is a command line, the linker
// applies the assignments in order, and the LAST one for a symbol is the one
// the binary carries — so a file that stamps `main.version` twice is not a file
// that stamps it correctly and then says something else. It is a file that
// stamps it with the second value, and a check that stops at the first match
// reports the value nobody gets.
//
// Both spellings of the flag are read: `-X sym=value` as one argument and `-X`
// followed by `sym=value` as two, because the linker accepts both and this file
// must not be a reason to prefer one.
func gateXAssignments(ldflags []string) map[string][]string {
	out := map[string][]string{}
	for _, item := range ldflags {
		fields := strings.Fields(item)
		for i := 0; i < len(fields); i++ {
			var assignment string
			switch {
			case fields[i] == "-X" && i+1 < len(fields):
				i++
				assignment = fields[i]
			case strings.HasPrefix(fields[i], "-X"):
				assignment = strings.TrimPrefix(fields[i], "-X")
			default:
				continue
			}
			symbol, value, ok := strings.Cut(assignment, "=")
			if !ok {
				continue
			}
			out[symbol] = append(out[symbol], value)
		}
	}
	return out
}

// gateRequireReleaseTransform holds gateReleaseBinaryVersion against the file
// that actually decides what the released binary prints.
//
// It is called before any comparison that rests on the transform, not merely
// asserted in a test of its own, because a stale model does not make the
// comparisons fail — it makes them succeed over the wrong values. A guard that
// notices its own model is wrong only in a separate test is still a guard that
// certified a release in between.
func gateRequireReleaseTransform(t *testing.T, root string) {
	t.Helper()
	assignments := gateXAssignments(gateLoadGoreleaser(t, root).Builds[0].Ldflags)

	for _, stamp := range gateReleaseStamps {
		// The import-path form first, by name, for THIS symbol. It is the
		// failure `.goreleaser.yaml` warns about in its own comment, the linker
		// accepts it silently, and it is reported here as what it is rather than
		// as "the expected line is missing" — the two send a maintainer to
		// different edits.
		//
		// Every symbol whose base name matches, not only `main.<name>`: the
		// spelling that no-ops is precisely the one that is NOT `main.`, so
		// anything ending in `.version`, `.commit` or `.date` that is not the
		// expected symbol is the defect rather than an unrelated flag.
		base := stamp.symbol[strings.LastIndex(stamp.symbol, ".")+1:]
		for symbol := range assignments {
			if symbol == stamp.symbol || !strings.HasSuffix(symbol, "."+base) {
				continue
			}
			t.Fatalf("%s stamps `-X %s=`, not `-X %s=`. The linker names a MAIN package's symbols `main.<name>`, so an `-X` aimed at the full import path is accepted, recorded in the build settings, and stamps NOTHING: "+
				"the released binary falls back to `debug.ReadBuildInfo` while `go version -m` still shows an `-ldflags` line. That is the failure this file's own configuration comment warns about, and the fallback it leaves behind is well-formed — "+
				"a forty-character `vcs.revision` and an RFC 3339 `vcs.time` — so no check on the SHAPE of what the binary reports can tell it from a stamped build",
				gateGoreleaserFile, symbol, stamp.symbol)
		}

		values := assignments[stamp.symbol]
		if len(values) == 0 {
			t.Fatalf("%s does not stamp `-X %s=` at all. It must carry %s. With the line gone the binary falls back to `debug.ReadBuildInfo`, which is well-formed and wrong: "+
				"for `main.version` it KEEPS the leading `v` — the opposite spelling from the one the site depicts — and for `main.date` it reports the commit's time rather than the build's",
				gateGoreleaserFile, stamp.symbol, stamp.stands)
		}
		if len(values) > 1 {
			t.Fatalf("%s stamps `-X %s=` %d times, with values %q. The linker applies them in order and the LAST one wins, so the binary carries %q whatever the earlier lines say — "+
				"a correct line followed by a wrong one is a wrong build that reads as a right one",
				gateGoreleaserFile, stamp.symbol, len(values), values, values[len(values)-1])
		}
		if values[0] == stamp.template {
			continue
		}
		if stamp.symbol == gateVersionSymbol && values[0] == gateStrippedTemplate {
			t.Fatalf("%s no longer stamps %q, and it names %s — the template that STRIPS the leading `v`. "+
				"That is the v0.5.2 fix run backwards, and it re-opens issue #38: the published archive would print `dossierx version <x.y.z>` while `go install …@v<x.y.z>` prints `v<x.y.z>`, because the module proxy hands debug.ReadBuildInfo the tag verbatim and no ldflags reach that path at all. "+
				"The same release then reports two different versions depending on how it was installed, and a scripted `dossierx version --format json | jq -r .data.version` compared against the tag succeeds one way and fails the other. "+
				"Nothing in cmd/dossierx can reconcile them: resolveVersionInfo's fallback IS the proxy's value. Put %s back, or move the site, gateReleaseBinaryVersion and this file's model together and accept the stripped form everywhere",
				gateGoreleaserFile, gateVersionLdflag, gateStrippedTemplate, gateVersionTemplate)
		}
		t.Fatalf("%s stamps `-X %s=%s`, where this file models %s. That symbol must carry %s; a third template makes it a guess, and every comparison here links its own binary with the guess",
			gateGoreleaserFile, stamp.symbol, values[0], stamp.template, stamp.stands)
	}
}

// gateGoreleaserArtifacts is what a release publishes: the product of the
// declared goos and goarch lists, plus the checksum file. Asserted as an EXACT
// set rather than as a subset, because docs/RELEASING.md's verification item
// says a release page lists "all six archives" and a seventh goos would make
// that sentence false with every containment check still green.
var (
	gateGoreleaserGOOS   = []string{"linux", "darwin", "windows"}
	gateGoreleaserGOARCH = []string{"amd64", "arm64"}
)

// gateChecksumFile is the checksum artifact docs/RELEASING.md's artifact step
// downloads alongside the archive.
const gateChecksumFile = "checksums.txt"

// gateArchiveNameTemplate is the template that DECIDES the six download names,
// and gateWindowsFormat is the one override that decides which of them is a zip.
//
// Both are read out of the parsed `archives` block rather than inferred from the
// names a build happened to produce. They are the input side of a sentence three
// other places state as fact: docs/RELEASING.md tells a maintainer to run
// `gh release download --pattern 'dossierx_<os>_<arch>*'`, the snapshot test in
// viewer-tests stats those six names, and `checksums.txt` is verified against
// them. A `name_template` edited to `{{ .ProjectName }}-{{ .Os }}` publishes six
// perfectly good archives under names nobody is told to ask for, and until this
// block was parsed nothing in the tree read it at all — the archive matrix was
// checked, the archive NAMING was not.
const (
	gateArchiveNameTemplate = "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
	gateWindowsGOOS         = "windows"
	gateWindowsFormat       = "zip"
)

// gateRequireArchiveNaming holds the `archives` block: one archive, fed by the
// one build, named by the template the procedure's download pattern spells, with
// windows and only windows overridden to a zip.
//
// The `ids` wiring is asserted rather than assumed, and it is the assertion with
// the quietest failure: an archive whose `ids` names a build that does not exist
// packages NOTHING, and an archive that names a second build packages a binary
// this file's one-build rule was written to exclude. Either way the release page
// still lists files, so no count of artifacts can tell the difference.
func gateRequireArchiveNaming(t *testing.T, config gateGoreleaserConfig) {
	t.Helper()

	if len(config.Archives) != 1 {
		t.Fatalf("%s declares %d archives, not 1. Every download name in docs/RELEASING.md and every name the snapshot test stats is one archive block's `name_template` applied to one build's goos/goarch pairs; "+
			"a second block publishes a second set of files under names nothing in this tree checks", gateGoreleaserFile, len(config.Archives))
	}
	archive := config.Archives[0]

	// The archive has to package the build this file just finished checking. A
	// build id nothing references is a stamped binary that is never shipped.
	buildID := config.Builds[0].ID
	if !gateSameSet(archive.IDs, []string{buildID}) {
		t.Errorf("%s's archive packages build ids %v, where the only build declared is %q. An `ids` entry naming no declared build packages nothing, and one naming a second build ships a binary whose ldflags this file never read — "+
			"in both cases the release page still lists files, so nothing downstream can tell", gateGoreleaserFile, archive.IDs, buildID)
	}

	// The names themselves. Compared with whitespace collapsed because the
	// template is written as a folded scalar and how it is wrapped is not a
	// decision anyone makes on purpose; what it SAYS is the assertion.
	if got := gateCollapseSpace(archive.NameTemplate); got != gateArchiveNameTemplate {
		t.Errorf("%s names its archives %q, where this tree models %q. docs/RELEASING.md tells a maintainer to download with `--pattern 'dossierx_<os>_<arch>*'` and the snapshot dry run stats the six names that template produces; "+
			"a renamed archive is six downloads nobody is told to ask for, published successfully", gateGoreleaserFile, got, gateArchiveNameTemplate)
	}

	// And the one override, as an exact set. A missing windows override ships
	// `dossierx_windows_amd64.tar.gz`, which is a name the procedure does not
	// give and which a Windows user has no ordinary tool for; a SECOND override
	// silently changes another platform's extension the same way.
	if len(archive.FormatOverrides) != 1 {
		t.Fatalf("%s declares %d archive format overrides, not 1. Exactly one platform is packaged differently from the rest — %s as a %s — and every extra override renames a download the procedure spells out",
			gateGoreleaserFile, len(archive.FormatOverrides), gateWindowsGOOS, gateWindowsFormat)
	}
	//
	// The format itself is resolved through gateArchivesOneFormat rather than
	// read off one field, so that the two legal spellings collapse to the one
	// value this tree models and an ambiguous or absent one is REFUSED instead
	// of silently compared as the empty string. There is no fallback: an
	// override that names no format at all overrides nothing, which is the
	// missing-override case one line above wearing a different shape.
	override := archive.FormatOverrides[0]
	format, err := gateArchivesOneFormat(fmt.Sprintf("the goos %q archive format override", override.GOOS), override.Format, override.Formats, "")
	if err != nil {
		t.Errorf("%s does not state one archive format for its goos %q override, so what this tree models — goos %q packaged as a %s — has nothing to be compared against: %v. The override decides an extension, so a config this check cannot read publishes a real archive under a name nothing asks for",
			gateGoreleaserFile, override.GOOS, gateWindowsGOOS, gateWindowsFormat, err)
		return
	}
	if override.GOOS != gateWindowsGOOS || format != gateWindowsFormat {
		t.Errorf("%s overrides the archive format for goos %q to %q, where this tree models goos %q to %q. The override decides an extension, so getting it wrong publishes a real archive under a name nothing asks for",
			gateGoreleaserFile, override.GOOS, format, gateWindowsGOOS, gateWindowsFormat)
	}
}

// TestGoreleaserConfigProducesTheArtifactsTheProcedureExpects reads the INPUTS
// the release build is given: the six os/arch combinations it publishes an
// archive for, and the checksum file the procedure downloads beside them.
//
// It is deliberately not the whole check, and it is no longer the only one:
// TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary below runs the tool
// and counts what it actually produced. This one runs everywhere with no
// external tool, and it fails with a message about the configuration rather
// than about a build, which is a different and more useful sentence when the
// matrix is what changed.
func TestGoreleaserConfigProducesTheArtifactsTheProcedureExpects(t *testing.T) {
	root := surfaceRepoRoot(t)
	config := gateLoadGoreleaser(t, root)
	build := config.Builds[0]

	if got := append([]string(nil), build.GOOS...); !gateSameSet(got, gateGoreleaserGOOS) {
		sort.Strings(got)
		t.Errorf("%s declares goos %v, not %v. The release publishes one archive per goos/goarch pair, so a dropped entry silently stops publishing a platform's download and an added one makes docs/RELEASING.md's \"all six archives\" false",
			gateGoreleaserFile, got, gateGoreleaserGOOS)
	}
	if got := append([]string(nil), build.GOARCH...); !gateSameSet(got, gateGoreleaserGOARCH) {
		sort.Strings(got)
		t.Errorf("%s declares goarch %v, not %v; the same argument as the goos list above", gateGoreleaserFile, got, gateGoreleaserGOARCH)
	}
	// The archive block: what the six artifacts are CALLED, and which of them is
	// a zip. The matrix above says how many there are; this says whether anyone
	// can find them.
	gateRequireArchiveNaming(t, config)
	// The checksum file the procedure's artifact item tells a maintainer to
	// verify the download against — read out of the `checksum` block, not found
	// somewhere in the document.
	if got := config.Checksum.NameTemplate; got != gateChecksumFile {
		t.Errorf("%s names its checksum file %q, not %q. docs/RELEASING.md's artifact step downloads the archive by name and the release publishes the checksums beside it; a renamed or removed checksum file leaves the download unverifiable",
			gateGoreleaserFile, got, gateChecksumFile)
	}
	// And the stamping line, through the same helper every comparison in this
	// file goes through — so this test fails for the same reason and with the
	// same message they would.
	gateRequireReleaseTransform(t, root)
}

// gateSameSet reports whether two string slices carry the same values, order
// and duplicates aside. Written out because the assertions above are about
// WHICH platforms are declared, and a YAML list's order is not a decision
// anyone makes on purpose.
func gateSameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	a := append([]string(nil), got...)
	b := append([]string(nil), want...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestReleaseProcedureHasNoCommitStampStep is the same deletion at the other
// end: the checklist item that wrote the field, and the false claim about
// `(devel)` that sat in the verification section, are both gone.
func TestReleaseProcedureHasNoCommitStampStep(t *testing.T) {
	root := surfaceRepoRoot(t)
	procedure := gateReadRepoFile(t, root, filepath.Join("docs", "RELEASING.md"))

	// The stamp step's own command. Naming the command rather than the prose
	// around it is what keeps this assertion from firing on the paragraph that
	// explains why the step is gone.
	if strings.Contains(procedure, "rev-parse --short vX.Y.Z^{commit}") {
		t.Error("docs/RELEASING.md still instructs stamping the release commit's short sha onto the site; the field it wrote to no longer exists")
	}
	if strings.Contains(procedure, "the ldflags did not\n      apply") || strings.Contains(procedure, "the ldflags did not apply") {
		t.Error("docs/RELEASING.md still claims a `go install` result proves whether the ldflags applied. It cannot: `go install ...@vX.Y.Z` builds from source with no ldflags at all, and prints the tag either way")
	}
}

// ---------------------------------------------------------------------
// the procedure may not name a check that does not exist
// ---------------------------------------------------------------------

// THE DEFECT THIS CLOSES, because it has now happened twice in one working tree.
// docs/RELEASING.md told a maintainer that two expressions in the site were
// "both now pinned by `TestSiteVersionTranscriptIsALiteralPrefixOfRealOutput`" —
// a test that had been deleted in the same change, in favour of a rendered read.
// The invariant was still held, by a differently-named test, so the sentence was
// TRUE and its evidence did not exist. That is the worst shape a procedure can
// take: a maintainer who goes to run the named check finds nothing, and the
// honest readings of that are "the check was removed" and "I typed it wrong",
// neither of which is what happened.
//
// The same document also said the snapshot dry run lives in `cmd/dossierx/`
// after it had moved to `viewer-tests/`, which is the same defect wearing a
// path instead of a name.
//
// Neither is caught by anything that reads prose for phrases, and a denylist of
// dead names would need an entry written the day each name dies — by the person
// who just deleted it, which is the person who did not. So both are checked
// structurally, against the Go DECLARATIONS in the tree: a `TestXxx` the
// procedure names must be declared somewhere, and where the procedure also
// names a Go location for it, it must be declared there.

// gateDocTestNameRE is a Go test identifier as it appears in prose. Anchored on
// a word boundary and requiring the capital after `Test`, so `Testing` and
// `Tested` are words and not claims.
var gateDocTestNameRE = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]*`)

// gateDocSpanRE is one inline code span. The ENTIRE span is captured and then
// filtered, rather than a path being searched for inside prose: `tests/` is a
// span and "the tests/ directory" is a sentence.
var gateDocSpanRE = regexp.MustCompile("`([^`\n]+)`")

// gateDeclaredTests maps every Go test function declared anywhere in this
// repository to the repo-relative file that declares it.
//
// PARSED, not grepped. `func TestFoo(` appears inside this very file's comments
// several times over, and a text search would accept a procedure that named a
// test which exists only in a paragraph explaining why it was deleted. go/parser
// sees declarations and nothing else: a name in a comment is a comment, a name in
// a string is a string, and a commented-out function is not declared.
//
// BOTH MODULES, because the nested one is where a check with an external
// prerequisite lives — the snapshot dry run among them — and a procedure that may
// only name tests from the root module is a procedure that cannot describe half
// its own gate.
func gateDeclaredTests(t *testing.T, root string) map[string]string {
	t.Helper()

	declared := map[string]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Dot directories hold .git and .claude — the latter can contain a
			// linked worktree, which is a second checkout of this repository
			// whose test declarations are that tree's, not this one's.
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			declared[fn.Name.Name] = filepath.ToSlash(rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the repository for test declarations: %v\nWithout the declarations there is nothing to hold the procedure's claims against, and every claim below would pass", err)
	}
	if len(declared) == 0 {
		t.Fatal("no Go test function was found anywhere in this repository, which cannot be true of this tree. The walk above has stopped seeing declarations, so the assertions below would pass over nothing")
	}
	return declared
}

// gateDocParagraphs splits a document into blank-line-separated paragraphs,
// which is the unit a checklist item's claim lives in.
func gateDocParagraphs(doc string) []string {
	var out, cur []string
	push := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(doc, "\n") {
		if strings.TrimSpace(line) == "" {
			push()
			continue
		}
		cur = append(cur, line)
	}
	push()
	return out
}

// gateGoLocations returns the code spans in a paragraph that name a place Go
// code can be declared: an existing directory, or an existing `.go` file.
//
// The existence check is what keeps this from firing on prose. A span has to
// resolve to something in this tree before it is read as a location claim, so
// `site/src/content.ts` in the same paragraph as a test name is a file that
// cannot declare a Go test and is ignored, while `cmd/dossierx/` is a claim.
func gateGoLocations(root, paragraph string) []string {
	var out []string
	for _, span := range gateDocSpanRE.FindAllStringSubmatch(paragraph, -1) {
		text := span[1]
		if strings.ContainsAny(text, " \t") || !strings.Contains(text, "/") || strings.Contains(text, "://") {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(text))
		info, err := os.Stat(filepath.Join(root, clean))
		if err != nil {
			continue
		}
		if info.IsDir() || strings.HasSuffix(clean, ".go") {
			out = append(out, filepath.ToSlash(clean))
		}
	}
	return out
}

// TestReleaseProcedureNamesOnlyChecksThatExist holds docs/RELEASING.md against
// the tree it describes.
//
// CLAUDE.md calls that file "the only description of how this project releases",
// and a maintainer runs it by hand. A named test that is not declared sends them
// looking for something that is not there; a named test declared somewhere other
// than where the procedure puts it sends them to the wrong directory and, when
// they find nothing, to the same wrong conclusion.
func TestReleaseProcedureNamesOnlyChecksThatExist(t *testing.T) {
	root := surfaceRepoRoot(t)
	procedure := gateReadRepoFile(t, root, filepath.Join("docs", "RELEASING.md"))
	declared := gateDeclaredTests(t, root)

	var named int
	for _, paragraph := range gateDocParagraphs(procedure) {
		for _, name := range gateDocTestNameRE.FindAllString(paragraph, -1) {
			named++
			where, ok := declared[name]
			if !ok {
				t.Errorf("docs/RELEASING.md names `%s`, which is declared nowhere in this repository.\n"+
					"A maintainer follows that file by hand, so a check it names is a check they are told to run: they will find nothing, and the two honest readings of that — the check was removed, or the name is a typo — are both wrong when the truth is that the invariant moved to a differently-named test.\n"+
					"Name the test that holds it now, or delete the claim; do not leave a sentence whose evidence does not exist.\nThe paragraph reads:\n%s",
					name, paragraph)
				continue
			}
			locations := gateGoLocations(root, paragraph)
			if len(locations) == 0 {
				continue
			}
			var inside bool
			for _, location := range locations {
				if where == location || strings.HasPrefix(where, location+"/") {
					inside = true
					break
				}
			}
			if !inside {
				t.Errorf("docs/RELEASING.md puts `%s` in %v, and it is declared in %s.\n"+
					"That is the same defect as naming a test that does not exist, wearing a path: a maintainer goes to the directory the procedure names, finds no such test, and concludes the check was deleted. This one moved.\nThe paragraph reads:\n%s",
					name, locations, where, paragraph)
			}
		}
	}
	if named == 0 {
		t.Fatal("docs/RELEASING.md names no Go test at all, which cannot be true of this procedure — it rests several of its items on named checks. This assertion has lost its subject and passed over nothing")
	}
	t.Logf("%d test name(s) in docs/RELEASING.md, all declared, over %d declarations in the tree", named, len(declared))
}

// gateLdflagsItemRE isolates the artifact checklist item, from its bolded title
// to the next item at the same level. Every assertion below is made against THAT
// SLICE rather than the whole document, which is the difference between "the
// file mentions the check somewhere" and "the item a maintainer reads tells them
// to make it". The first version of this test asserted against the whole file
// and survived deleting the command outright, because the surrounding prose
// discusses `go version -m` by name.
var gateLdflagsItemRE = regexp.MustCompile(`(?s)- \[ \] \*\*The ldflags reached the published binary\.\*\*.*?\n- \[ \] `)

// TestReleaseProcedureRestsTheLdflagsCheckOnTheBuildSettings pins WHICH check
// the procedure tells a maintainer to make, because the first two answers were
// both wrong and both wrong in the same way: they read the version envelope,
// which a no-ldflags build at the tag fills with a version, the full sha and a
// real timestamp. The third answer was wrong in a smaller way — it said the
// envelope came out IDENTICAL, and one field does not — which is why the
// denylist below carries all three.
func TestReleaseProcedureRestsTheLdflagsCheckOnTheBuildSettings(t *testing.T) {
	root := surfaceRepoRoot(t)
	procedure := gateReadRepoFile(t, root, filepath.Join("docs", "RELEASING.md"))

	item := gateLdflagsItemRE.FindString(procedure)
	if item == "" {
		t.Fatal("docs/RELEASING.md no longer carries an item titled **The ldflags reached the published binary.**; the only check that can tell a stamped archive from an unstamped one has left the procedure, and this assertion has lost its subject")
	}

	// The command itself, in the item's command block — not the same string
	// somewhere in the prose explaining it.
	if !strings.Contains(item, "\n      go version -m ./dossierx\n") {
		t.Errorf("the artifact item no longer tells a maintainer to RUN `go version -m ./dossierx`. Reading the version envelope cannot substitute: a no-ldflags build at the tag fills it with a version, the tagged commit's full sha and a real timestamp, "+
			"and the only field that differs from a stamped build's is the version's leading `v`. The item reads:\n%s", item)
	}
	// What the -ldflags line must be checked to SAY. Its presence alone passes
	// on the exact historical failure — an -X aimed at the full import path,
	// which is recorded in the build settings and silently stamps nothing.
	if !strings.Contains(item, "naming `-X main.version=`") {
		t.Errorf("the artifact item no longer says the `-ldflags` line must NAME `-X main.version=`. Checking only that some -ldflags line exists passes on the `main.` prefix bug .goreleaser.yaml warns about. The item reads:\n%s", item)
	}
	// And the other two symbols, because the no-op is PER SYMBOL. An item that
	// sends a maintainer to look for `-X main.version=` alone is satisfied by a
	// build whose commit and date were aimed at the import path — that binary
	// reports the sha and the COMMIT's timestamp out of `debug.ReadBuildInfo`,
	// both well-formed, and the maintainer certifies it.
	if !strings.Contains(item, "`-X main.commit=` and `-X main.date=`") {
		t.Errorf("the artifact item names only `-X main.version=`. The import-path no-op applies to each `-X` independently, so `main.commit` and `main.date` can be aimed at the import path while the version stamps correctly; "+
			"the binary then reports the commit's timestamp where the release means the build's, and every check the item tells a maintainer to make still passes. The item reads:\n%s", item)
	}
	// The claims that were false, spelled the way each was spelled.
	for _, lie := range gateLdflagsLies {
		if strings.Contains(procedure, lie.phrase) {
			t.Errorf("docs/RELEASING.md carries %q again. %s", lie.phrase, lie.why)
		}
	}
}

// gateLdflagsLies are the false statements docs/RELEASING.md has carried about
// how to tell a stamped archive from an unstamped one. Each was written in good
// faith, shipped, and then contradicted by a measurement.
//
// They are kept as a denylist because each was the CONCLUSION of the item, not a
// detail inside it: a reader who acts on any of them certifies a release over
// evidence that does not distinguish the two builds. A rewrite that reaches for
// one of these phrasings again is reaching for the reasoning behind it.
var gateLdflagsLies = []struct{ phrase, why string }{
	// The first two rested the verdict on the version envelope, which a
	// no-ldflags build at a tag fills with a version, the full sha and a real
	// timestamp.
	{"the one value that cannot be produced any other way",
		"A build with no ldflags at a tag reports a version, the full sha and a real timestamp, so no reading of the version envelope can certify the stamping."},
	{"**`version` is the signal.**",
		"The `-ldflags` build setting is the signal; the version envelope is filled either way."},
	// The third is the correction to the first two, and it overshot: it
	// generalised "the envelope cannot certify the stamping" into "the envelope
	// is identical", which is one field too far.
	{"every field identical to a correctly stamped build",
		"It is not every field. Measured side by side on the v0.5.0 tree, the published archive reports version 0.5.0 and a no-ldflags build of the same commit reports v0.5.0 — GoReleaser's `{{.Version}}` strips the leading `v` where `info.Main.Version` keeps it. " +
			"The commit is byte-identical and the two dates are both plausible, so the conclusion holds; the claim of identity does not, and a maintainer who reads it will not look at the one field that does differ."},
}

// TestLdflagsShowUpOnlyInTheBuildSettings is the code half of the artifact check
// docs/RELEASING.md now carries: `go version -m` records the link flags, and
// records nothing when there were none.
//
// It builds two binaries rather than reasoning about one, because the claim is
// about what the LINKER wrote into a build — something no in-process call can
// observe, since the test binary was linked by a different command line.
//
// Neither build depends on the tree having usable VCS metadata, which is what
// sank the assertion this replaces: that one asserted a plain `go build`
// reports "dev", which holds only where Go can find no repository. In an
// ordinary clone Go stamps `info.Main.Version` from the tag and the binary
// reports the tag; in CI's shallow, tagless checkout it reports a
// `v0.0.0-<date>-<sha>` pseudo-version. Both are correct behaviour and both
// made that test red.
func TestLdflagsShowUpOnlyInTheBuildSettings(t *testing.T) {
	root := surfaceRepoRoot(t)

	// The flags GoReleaser passes, in its shape: -s -w alongside the three -X
	// assignments. -s and -w are included deliberately — the procedure tells a
	// maintainer they do not hide the setting, and that is an assertion.
	stamped := gateBuildDossierx(t, root, "-ldflags",
		"-s -w -X main.version=v9.9.9 -X main.commit=c0ffee -X main.date=2026-01-01T00:00:00Z")
	settings := gateRun(t, "go", "version", "-m", stamped)

	if !strings.Contains(settings, "-ldflags=") {
		t.Fatalf("`go version -m` reports no `-ldflags` setting for a binary built WITH ldflags. The release procedure's artifact check reads exactly this line, and it has stopped being written:\n%s", settings)
	}
	if !strings.Contains(settings, "-X main.version=v9.9.9") {
		t.Errorf("`go version -m` does not report the `-X main.version=` assignment it was linked with. The procedure tells a maintainer to look for it, so a release could be certified against a line that no longer says which symbol was stamped:\n%s", settings)
	}
	// The flags having been RECORDED is not the same claim as their having
	// been APPLIED, and the procedure reads the envelope for the second. Both
	// halves are asserted so neither can rot alone.
	var envelope struct {
		Data versionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(gateRun(t, stamped, "version", "--format", "json")), &envelope); err != nil {
		t.Fatalf("decode the version envelope: %v", err)
	}
	if envelope.Data.Version != "v9.9.9" || envelope.Data.Commit != "c0ffee" || envelope.Data.Date != "2026-01-01T00:00:00Z" {
		t.Errorf("a binary linked with -X main.version/commit/date reports %+v; the -X assignments did not reach the variables they name. This is the `main.` prefix failure .goreleaser.yaml warns about, and it is the failure the artifact check exists to catch", envelope.Data)
	}

	// The negative. Without it the check above proves only that a line exists,
	// not that its absence means anything — and its absence is the whole
	// signal.
	plain := gateBuildDossierx(t, root, "-buildvcs=false")
	if s := gateRun(t, "go", "version", "-m", plain); strings.Contains(s, "-ldflags=") {
		t.Errorf("`go version -m` reports an `-ldflags` setting for a binary built with none. The artifact check reads the line's presence as proof of stamping, which would then be proof of nothing:\n%s", s)
	}
}

// TestBuildWithNoVCSAndNoLdflagsReportsDev pins the one thing `dev` still means:
// resolveVersionInfo's last resort, reached when there are no ldflags AND no
// build info to fall back on.
//
// -buildvcs=false is what makes this deterministic everywhere. With VCS
// stamping on, `info.Main.Version` is whatever the repository says, which
// differs between a tagged clone, a shallow CI checkout and a linked worktree —
// so the assertion would be about the checkout rather than about the code. With
// it off the module version is `(devel)`, which resolveVersionInfo excludes, and
// the fallback chain is the only thing left to observe.
func TestBuildWithNoVCSAndNoLdflagsReportsDev(t *testing.T) {
	root := surfaceRepoRoot(t)
	binary := gateBuildDossierx(t, root, "-buildvcs=false")

	var envelope struct {
		Data versionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(gateRun(t, binary, "version", "--format", "json")), &envelope); err != nil {
		t.Fatalf("decode the version envelope: %v", err)
	}

	switch envelope.Data.Version {
	case "dev":
		// The documented last resort.
	case "(devel)":
		t.Fatal("a build with no ldflags and no VCS info reports \"(devel)\" — resolveVersionInfo has stopped excluding debug.ReadBuildInfo's placeholder. " +
			"That value is not a version, and reporting it would put a string no release can carry into the one field an agent reads to identify the toolchain it was handed")
	default:
		t.Fatalf("a build with no ldflags and no VCS info reports %q; the documented last resort is \"dev\"", envelope.Data.Version)
	}
	if envelope.Data.Commit != "unknown" || envelope.Data.Date != "unknown" {
		t.Errorf("the same build reports commit=%q date=%q; with nothing stamped and no VCS settings to read, resolveVersionInfo documents \"unknown\" rather than a blank field", envelope.Data.Commit, envelope.Data.Date)
	}
}

// gateBuildDossierx builds ./cmd/dossierx into a fresh temporary path with the
// given extra build flags, and returns the binary. Each call gets its own
// TempDir so two builds in one test cannot overwrite each other.
func gateBuildDossierx(t *testing.T, root string, buildFlags ...string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "dossierx")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	args := append([]string{"build", "-o", binary}, buildFlags...)
	build := exec.Command("go", append(args, "./cmd/dossierx")...)
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(build.Args[1:], " "), err, out)
	}
	return binary
}

// gateRun executes a command and returns its stdout, failing on any error:
// every caller below reads the output as evidence, and an empty string from a
// command that did not run would be read as evidence of absence.
func gateRun(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		t.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return string(out)
}

// gateReadRepoFile reads one repo-relative file, failing rather than returning
// an empty string: every assertion above would pass over an empty document.
func gateReadRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if len(raw) == 0 {
		t.Fatalf("%s is empty", rel)
	}
	return string(raw)
}
