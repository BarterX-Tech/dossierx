package viewertests

// WHAT A DOM READ STRUCTURALLY CANNOT SEE.
//
// site_dom_test.go reads the thing the visitor reads, which is the right rule
// for output and blind to the invariants that decide whether that output stays
// true. A hand-typed "v0.5.0" and a derived one render the same bytes on the day
// they are written; they differ one release later, and by then the page is
// wrong and nothing in the DOM says so. Reading content.ts does not help either:
// a correct hard-coded string is not a falsehood yet.
//
// So the three assertions here are about the SOURCE, and each one pins a rule
// the site's own comments already state:
//
//   - content.ts's releases array is OLDEST-FIRST, because ReleaseTimeline
//     treats releases[len-1] as the current release (and marks it "latest").
//     A prepended entry silently demotes the new release and promotes the
//     previous one, on both the landing teaser and releases.html.
//   - exactly one entry carries tag: "Latest release", and it is the last one.
//     Two is a page that names two current releases; none is a page that names
//     the current release as nothing in particular.
//   - a version appears as a RENDERABLE literal in exactly one place — that
//     array — so the hero kicker, the hero badge list, the release-history intro
//     and the `dossierx version` example must all keep deriving it from
//     latestRelease. Four of them once carried their own copy and three went
//     stale. The one carve-out is the handful of sentences in content.ts that
//     narrate the project's past, and every one of them is DECLARED by name in
//     declaredHistoryLiterals; see there for why an open-ended carve-out for
//     that file reopened this hole from the other side.
//
// Each rule is a function that RETURNS its finding rather than failing in place,
// and TestSiteSourceRulesCatchTheirOwnDefects below runs all three over synthetic
// trees that contain the defect. That is not ceremony: this suite's whole value
// is that it goes red on a bad edit to site/, and the only honest way to show it
// does — without editing site/, which other lanes are holding — is to hand it a
// tree that is already bad and watch it complain.
//
// These live in viewer-tests rather than beside the engine's Go tests for the
// same reason the browser suite does: site/ is a TypeScript tree the root module
// has no business reading, and this is the module that already owns the site as
// a subject.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The releases array is delimited rather than parsed: a TypeScript parser in a
// Go test module is a dependency this repo will not take, and the two anchors
// below are the file's own top-level declaration and its closing bracket in
// column zero. Both are asserted to exist — a restructure that moves them makes
// this suite RED and demands a look, which is the correct outcome for a change
// to the one array every version string on the site is derived from.
//
// A LINE THIS REGEX CANNOT READ IS THE DANGEROUS CASE, because every rule below
// is defined relative to current() — the LAST version line that matched — and a
// missed line does not fail, it silently moves current() back one release. The
// rules then guard a version that can no longer go stale and wave through the
// one that can. Two things keep that shut.
//
// reReleaseEntry tolerates a trailing line comment. THAT TOLERANCE OUTLIVED ITS
// ORIGINAL REASON, which was that entries in this array carried inline comments
// on their `commit` field — a field that has since been deleted outright, so no
// entry carries a comment today and this comment used to assert, in the present
// tense, something the array had stopped doing. (The deletion is held by
// cmd/dossierx/gate_release_stamp_test.go, which refuses the field's data, its
// readers and its type declaration; `commit` is not coming back.)
//
// The tolerance is kept anyway, and on a different argument. It is not here to
// admit a shape the file has — it is here so that a shape the file is ALLOWED to
// have cannot turn this suite red for a formatting choice. A trailing `//` on a
// version line changes nothing a visitor reads, and a rule that went red on one
// would be a rule maintainers learn to work around.
//
// And loadReleasesArray cross-checks the number of version lines it read against
// the number of objects actually in the array (topLevelObjects), so any OTHER
// unreadable shape — a different quote style, a reformat onto two lines, a field
// renamed — is a red build rather than a quieter definition of "current". That
// cross-check is what makes the tolerance safe to keep: with it in place an
// unreadable line is an error, never a silent slide of current() back one
// release, so the tolerance can only ever prevent a false red.
var (
	reReleasesOpen  = regexp.MustCompile(`(?m)^const releases: Release\[\] = \[$`)
	reReleasesClose = regexp.MustCompile(`(?m)^\];$`)
	reReleaseEntry  = regexp.MustCompile(`(?m)^[ \t]*version: "(v\d+\.\d+\.\d+)",(?:[ \t]*//.*)?$`)
	reLatestTag     = regexp.MustCompile(`(?m)^\s*tag: "Latest release",$`)
	reVersionLit    = regexp.MustCompile(`v\d+\.\d+\.\d+`)
)

const latestReleaseTag = `tag: "Latest release"`

// releasesArray is content.ts plus the byte range of its releases declaration:
// everything the three rules below need, read once so a failure in the parse is
// reported once rather than three times in three different words.
type releasesArray struct {
	root     string // repository root this was read from
	path     string // absolute path to content.ts
	src      string // the whole file
	inert    []bool // src bytes inside a comment, string or template literal
	start    int    // first byte after `const releases: Release[] = [`
	end      int    // the `];` that closes it
	versions []string
}

// current is the release the site presents as the current one — the LAST entry,
// because that is the one ReleaseTimeline badges.
func (ra releasesArray) current() string { return ra.versions[len(ra.versions)-1] }

// siteContentPath is content.ts, the single source of the site's copy.
func siteContentPath(root string) string {
	return filepath.Join(root, "site", "src", "content.ts")
}

func loadReleasesArray(root string) (releasesArray, error) {
	ra := releasesArray{root: root, path: siteContentPath(root)}
	b, err := os.ReadFile(ra.path)
	if err != nil {
		return ra, fmt.Errorf("read %s: %w", ra.path, err)
	}
	ra.src = string(b)
	_, ra.inert = tsScan(ra.src)

	open := reReleasesOpen.FindStringIndex(ra.src)
	if open == nil {
		return ra, fmt.Errorf("%s no longer declares `const releases: Release[] = [` at the top level. "+
			"Every version string on the site is derived from that array, so this suite cannot "+
			"check anything until it can find it again", ra.path)
	}
	closing := reReleasesClose.FindStringIndex(ra.src[open[1]:])
	if closing == nil {
		return ra, fmt.Errorf("%s: found the releases array but no closing `];` in column zero after it", ra.path)
	}
	ra.start, ra.end = open[1], open[1]+closing[0]

	for _, m := range reReleaseEntry.FindAllStringSubmatch(ra.src[ra.start:ra.end], -1) {
		ra.versions = append(ra.versions, m[1])
	}
	// The line that stops a missed `version:` from redefining current(). See the
	// note on the regexes above: nothing else in this file would notice.
	if entries := topLevelObjects(ra.src, ra.inert, ra.start, ra.end); entries != len(ra.versions) {
		return ra, fmt.Errorf("%s: the releases array holds %d entry object(s) but only %d `version:` "+
			"line(s) could be parsed out of it (%v).\nEvery rule in this file is defined against the "+
			"LAST version parsed, so an entry this suite cannot read does not fail — it silently makes "+
			"the PREVIOUS release the one being guarded, and the current one stops being checked at all.\n"+
			"Restore the field to `version: \"vX.Y.Z\",` (a trailing // comment is fine) or teach "+
			"reReleaseEntry the new shape",
			ra.path, entries, len(ra.versions), ra.versions)
	}
	if len(ra.versions) < 2 {
		return ra, fmt.Errorf("only %d release entries parsed out of the releases array (%v). Ordering "+
			"cannot be asserted over fewer than two, so this would be a pass over nothing",
			len(ra.versions), ra.versions)
	}
	return ra, nil
}

// repoReleases is loadReleasesArray against the tree under test. A parse failure
// is fatal rather than reported, because every rule below is a pass over nothing
// without it.
func repoReleases(t *testing.T) releasesArray {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	ra, err := loadReleasesArray(root)
	if err != nil {
		t.Fatal(err)
	}
	return ra
}

// semver turns vMAJOR.MINOR.PATCH into three comparable numbers. The site's own
// history is entirely pre-1.0 three-part tags; anything else is a change worth
// failing on rather than parsing loosely.
func semver(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("release version %q is not vMAJOR.MINOR.PATCH", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("release version %q has a non-numeric component %q", v, p)
		}
		out[i] = n
	}
	return out, nil
}

func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// ---------------------------------------------------------------------
// rule 1 — the array's direction
// ---------------------------------------------------------------------

// checkOldestFirst pins the direction of the array against the consumer that
// gives it meaning: ReleaseTimeline.tsx computes `latestIndex = releases.length
// - 1`, badges that entry "latest", and slices the landing-page teaser off the
// END. Prepend a release instead of appending it and both pages announce the
// wrong one, with no type error and no visual clue on the day it happens.
func checkOldestFirst(ra releasesArray) error {
	for i := 1; i < len(ra.versions); i++ {
		prev, err := semver(ra.versions[i-1])
		if err != nil {
			return err
		}
		cur, err := semver(ra.versions[i])
		if err != nil {
			return err
		}
		if !less(prev, cur) {
			return fmt.Errorf("the releases array is not oldest-first: entry %d is %s and entry %d is %s.\n"+
				"ReleaseTimeline treats releases[len-1] as the CURRENT release, so a new entry is "+
				"APPENDED, never prepended.\nFull order read: %v",
				i-1, ra.versions[i-1], i, ra.versions[i], ra.versions)
		}
	}
	return nil
}

func TestReleasesAreOldestFirst(t *testing.T) {
	ra := repoReleases(t)
	if err := checkOldestFirst(ra); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d releases, oldest-first, current = %s", len(ra.versions), ra.current())
}

// ---------------------------------------------------------------------
// rule 2 — which entry says it is current
// ---------------------------------------------------------------------

// checkExactlyOneLatestTag pins the `tag` field that says, in the page's own
// words, which release is current. Two entries carrying it makes the site name
// two current releases; none makes it name none. It also has to sit on the LAST
// entry, because that is the one ReleaseTimeline independently badges "latest" —
// the two saying different things is the same defect wearing a different hat.
func checkExactlyOneLatestTag(ra releasesArray) error {
	block := ra.src[ra.start:ra.end]

	tags := reLatestTag.FindAllStringIndex(block, -1)
	if len(tags) != 1 {
		return fmt.Errorf("found %d entries carrying `%s` in the releases array, want exactly 1. "+
			"Two names two current releases; none names none", len(tags), latestReleaseTag)
	}

	entries := reReleaseEntry.FindAllStringIndex(block, -1)
	if len(entries) == 0 {
		return fmt.Errorf("no release entries parsed, so the position of `%s` means nothing", latestReleaseTag)
	}
	if lastEntry := entries[len(entries)-1][0]; tags[0][0] < lastEntry {
		return fmt.Errorf("`%s` sits on an entry that is not the last one. ReleaseTimeline badges "+
			"releases[len-1] as latest, so the tag and the badge would name different releases",
			latestReleaseTag)
	}
	return nil
}

func TestExactlyOneLatestReleaseTag(t *testing.T) {
	if err := checkExactlyOneLatestTag(repoReleases(t)); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------
// rule 3 — where a version is allowed to be written down
// ---------------------------------------------------------------------

// historyLiteral declares ONE version literal that content.ts renders outside
// the releases array, and says why that literal is history rather than a copy.
//
// phrase is the exact bytes of content.ts that carry the literal, and it must
// occur there exactly once. Keying on the sentence rather than on the number is
// the whole mechanism: the same version can be true history in one sentence and
// a stale copy in the next, and only the sentence tells them apart.
type historyLiteral struct {
	version string
	phrase  string // must contain version, and occur exactly once in content.ts
	why     string
}

// declaredHistoryLiterals is every renderable version literal that lives in
// content.ts outside the releases array. Eight sentences today, each naming what
// some past release did.
//
// WHY EACH ONE IS WRITTEN DOWN, rather than exempted by a rule.
//
// The site's copy has to be able to say "v0.3.0 deleted them" — statements no
// future release can falsify, and which a rule banning every `v\d+\.\d+\.\d+`
// under site/src would forbid the site from making. The first cut of this rule
// therefore exempted every NON-CURRENT literal in content.ts, on the reasoning
// that a version which is not the current one cannot be a copy of the current
// one.
//
// That reasoning is wrong, and wrong in the direction this whole file exists to
// close. Three of the four sites the site must DERIVE its version at — the hero
// badge, the `dossierx version` example and the release-history intro — live in
// content.ts, inside that exemption. Replace the interpolation in the version
// example with the literal it renders today and the page is not wrong yet; it
// becomes wrong at the next tag, at which point the literal is no longer the
// current version, the exemption swallows it, and the rule reports that every
// version on the site is derived. The check would be at its quietest precisely
// when the defect became real.
//
// So the carve-out is an INVENTORY instead. A literal in content.ts outside the
// array is allowed only where one of these entries says it is, which means a new
// one — pasted from an old transcript, or typed over an interpolation — is red
// on the day it lands, whichever release it names. Adding to this list is a
// deliberate act with a reason attached, the same shape surfaces.yaml uses to
// declare a file in or out of scope, and the current release can never be added
// to it at all: checkNoHardCodedVersions rejects that literal before it consults
// a single declaration.
//
// A reworded sentence also lands here as a failure. That is the intended cost:
// the prose around a version literal is the only evidence that the literal is
// history, so an edit to it is exactly the moment to re-read the claim.
var declaredHistoryLiterals = []historyLiteral{
	{
		version: "v0.3.0",
		phrase:  "v0.3.0 made the machine contract the product",
		why:     "the pitch section, narrating the release that made --format json the default",
	},
	{
		version: "v0.4.0",
		phrase:  "Since v0.4.0 it is also a DRIFT edge",
		why:     "the doctrine_facet field note, dating when governed_by started flagging drift",
	},
	{
		version: "v0.3.0",
		phrase:  "The lifecycle has not changed since v0.3.0",
		why:     "the lifecycle section, dating the release the current lifecycle settled in",
	},
	{
		version: "v0.4.0",
		phrase:  "A claim locked before v0.4.0 carries no governance baseline",
		why:     "the drift-reasons table, naming the release before which no baseline exists",
	},
	{
		version: "v0.3.0",
		phrase:  "v0.3.0's job was to stop the loop dead-ending",
		why:     "the review-loop section, crediting the release that made every card commentable",
	},
	{
		version: "v0.3.0",
		phrase:  "every other lock in v0.3.0, takes a",
		why:     "the build-order section, dating when --reason became mandatory on a lock",
	},
	{
		version: "v0.2.0",
		phrase:  "top-level commands as late as v0.2.0",
		why: "the migration table's row for lock/unlock/reaudit/flag, naming the last release that " +
			"carried them at the root. It says v0.2.0 rather than v0.3.0 deliberately: v0.3.0 is the " +
			"release that FOLDED them, and its own main.go registers none of the four, so `through " +
			"v0.3.0` — the first wording — was false. The row exists because the binary retires " +
			"sixteen commands and this table listed twelve",
	},
	{
		version: "v0.4.0",
		phrase:  "Removed outright in v0.4.0.",
		why:     "the removed-commands table, naming the release that deleted `migrate`",
	},
	{
		version: "v0.2.0",
		phrase:  "byte-for-byte identical to v0.2.0's output",
		why:     "the --format note, naming the release whose text output the golden fixtures pin",
	},
}

// historySpan is one declaration located in content.ts.
type historySpan struct {
	decl  historyLiteral
	start int
	end   int
	used  bool
}

// locateHistoryLiterals resolves every declaration against content.ts and
// returns the byte ranges the rule may not complain inside.
//
// Every failure here is an error rather than a finding, because a declaration
// that cannot be located is a hole in the rule and not a defect in the site: the
// literal it was written for is still in the file, and if this returned the
// declaration as merely absent the rule would go on and flag that literal, which
// is a true failure told as the wrong story.
func locateHistoryLiterals(ra releasesArray, declared []historyLiteral) ([]*historySpan, error) {
	known := map[string]bool{}
	for _, v := range ra.versions {
		known[v] = true
	}

	spans := make([]*historySpan, 0, len(declared))
	for _, d := range declared {
		switch {
		case !strings.Contains(d.phrase, d.version):
			return nil, fmt.Errorf("declared history literal %q does not appear in its own phrase %q, "+
				"so the declaration cannot say which literal it covers", d.version, d.phrase)
		case !known[d.version]:
			return nil, fmt.Errorf("declared history literal %q (%q) names a release the releases array "+
				"does not list (%v). The site cannot narrate a release it does not ship a history entry "+
				"for — either the entry was dropped or the sentence names the wrong version",
				d.version, d.phrase, ra.versions)
		}

		first := strings.Index(ra.src, d.phrase)
		if first < 0 {
			return nil, fmt.Errorf("declared history phrase %q is not in %s.\nIt is the evidence that the "+
				"%s beside it is history rather than a copy of the current version, so the declaration "+
				"cannot stand without it. If the sentence was reworded, re-read it and update the phrase; "+
				"if it was deleted, delete this declaration", d.phrase, ra.path, d.version)
		}
		if next := strings.Index(ra.src[first+1:], d.phrase); next >= 0 {
			return nil, fmt.Errorf("declared history phrase %q occurs more than once in %s. A declaration "+
				"covers one sentence; two means it would exempt a literal nobody read", d.phrase, ra.path)
		}
		if first < ra.end && first+len(d.phrase) > ra.start {
			return nil, fmt.Errorf("declared history phrase %q overlaps the releases array in %s, where "+
				"every literal is already allowed. A declaration there exempts nothing and hides that it "+
				"exempts nothing", d.phrase, ra.path)
		}
		spans = append(spans, &historySpan{decl: d, start: first, end: first + len(d.phrase)})
	}
	return spans, nil
}

// versionScan is one pass of rule 3 over site/src.
type versionScan struct {
	offenders []string // renderable literals in a place they are not allowed to be
	unused    []string // declarations that turned out to cover no literal at all
	scanned   int
}

// checkNoHardCodedVersions is the rule that catches the staleness before it
// exists. A version is a fact with an expiry date: written into a component
// today it is TRUE, and it becomes a published falsehood at the next tag with
// nothing to notice it — which is exactly how three of the site's four version
// sites went stale before they were rewritten to read latestRelease.
//
// A literal is an offender unless one of three things is true of it.
//
//   - IT IS IN A COMMENT. A comment is not client-facing prose: it reaches a
//     reader of the repository, never a visitor, and it cannot be the false
//     statement this gate exists to stop. content.ts explains in a doc comment
//     why release entries no longer carry a `commit` field, and does it by
//     naming the two releases that demonstrated the problem. The exemption is
//     computed by tsCommentMask, not by stripping `//` to end of line — see
//     there for why a URL in rendered text would otherwise blind this.
//   - IT IS INSIDE THE RELEASES ARRAY. That array is the one place a version is
//     supposed to be written down, and rules 1 and 2 are what guard it.
//   - IT IS DECLARED, one sentence at a time, in declaredHistoryLiterals — and
//     the current version is not declarable, whatever the list says. See the
//     note on that list for why the carve-out is an inventory and not a rule.
//
// Outside content.ts nothing is declarable at all: no other file under site/src
// carries a renderable version literal today, and none has any business doing
// so, since every one of them can read latestVersion.
func checkNoHardCodedVersions(ra releasesArray, declared []historyLiteral) (versionScan, error) {
	var scan versionScan
	current := ra.current()
	srcDir := filepath.Join(ra.root, "site", "src")

	spans, err := locateHistoryLiterals(ra, declared)
	if err != nil {
		return scan, err
	}

	walkErr := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(path); ext != ".ts" && ext != ".tsx" {
			return nil
		}
		body := ra.src
		if path != ra.path {
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			body = string(b)
		}
		scan.scanned++
		comments := tsCommentMask(body)
		for _, loc := range reVersionLit.FindAllStringIndex(body, -1) {
			if comments[loc[0]] {
				continue
			}
			lit := body[loc[0]:loc[1]]
			why := "no other file under site/src may write a version down; read latestVersion"
			if path == ra.path {
				if loc[0] >= ra.start && loc[0] < ra.end {
					continue
				}
				switch {
				case lit == current:
					why = "the CURRENT release, copied out of the releases array — correct today, " +
						"false at the next tag, and not declarable as history"
				default:
					if span := coveringSpan(spans, loc[0], loc[1], lit); span != nil {
						span.used = true
						continue
					}
					why = "an undeclared version literal: neither derived from latestRelease nor " +
						"listed in declaredHistoryLiterals as a sentence about a past release"
				}
			}
			rel, relErr := filepath.Rel(ra.root, path)
			if relErr != nil {
				rel = path
			}
			scan.offenders = append(scan.offenders, fmt.Sprintf("%s:%d (%s) — %s", rel,
				1+strings.Count(body[:loc[0]], "\n"), lit, why))
		}

		// THE OTHER SPELLING, and it is the one the rule above is shaped to miss.
		//
		// reVersionLit requires a leading `v`, so it reads the RELEASE's name and
		// is blind to the bare one. Until v0.5.2 `.goreleaser.yaml` stamped
		// `-X main.version={{.Version}}`, the tag with that `v` stripped, so the
		// site's `dossierx version` transcript depicted `0.5.0` where every other
		// version string on the page read `v0.5.0`.
		//
		// That gap was not theoretical. Replacing the transcript's interpolation
		// with the literal it renders today left this rule, the root suite and the
		// browser suite all green — the literal carried no `v` to match — and the
		// page would have gone on depicting THIS release's output from the next tag
		// onward. A rendered read cannot catch it either: a hand-typed string and a
		// derived one render the same bytes on the day they are written. Only the
		// source can, and only if it is looking for the right spelling.
		//
		// SINCE v0.5.2 THE STAMP IS `{{.Tag}}` AND NOTHING PRINTS THE BARE FORM,
		// which makes this rule stricter rather than obsolete. A bare literal used
		// to be the right characters in the wrong place — the binary's spelling,
		// hand-typed. Now it is a string no install path produces at all, so there
		// is no reading of the page on which it is correct.
		//
		// It is not exempted inside the releases array. That array declares tags,
		// and every one of them carries its `v`; a bare current version inside it is
		// a copy that happens to live in the same brackets.
		for _, loc := range binaryVersionLocs(body, strippedOf(current)) {
			if comments[loc[0]] {
				continue
			}
			rel, relErr := filepath.Rel(ra.root, path)
			if relErr != nil {
				rel = path
			}
			scan.offenders = append(scan.offenders, fmt.Sprintf("%s:%d (%s) — %s", rel,
				1+strings.Count(body[:loc[0]], "\n"), body[loc[0]:loc[1]],
				"the current release with its leading `v` stripped — a spelling NO install path prints. "+
					"Since v0.5.2 the archive stamps the tag as tagged and `go install` takes it verbatim from the "+
					"module proxy, so both print `v<x.y.z>`; a bare literal depicts output that does not exist, and "+
					"goes stale at the next tag on top of that. Derive it from latestVersion"))
		}
		return nil
	})
	if walkErr != nil {
		return scan, fmt.Errorf("walk %s: %w", srcDir, walkErr)
	}
	if scan.scanned == 0 {
		return scan, fmt.Errorf("no .ts/.tsx files found under %s, so this scan asserted nothing", srcDir)
	}
	for _, span := range spans {
		if !span.used {
			scan.unused = append(scan.unused, fmt.Sprintf("%s in %q", span.decl.version, span.decl.phrase))
		}
	}
	return scan, nil
}

// strippedOf is the tag with its leading `v` removed — THE SPELLING NOTHING IN
// THIS PROJECT PRODUCES, which is why the scan above hunts for it.
//
// It used to be named binaryOf, and it used to be the release transform: the
// version a release build stamped into the binary. `.goreleaser.yaml` stamped
// `{{.Version}}`, the tag with its `v` stripped, so the archive printed `0.5.1`
// while `go install …@v0.5.1` printed `v0.5.1` — the module proxy hands
// debug.ReadBuildInfo the tag verbatim and no ldflags reach that path at all.
// One release, two version strings, depending on how it was installed. That is
// issue #38, and v0.5.2 fixed it by moving the stamp to `{{.Tag}}`.
//
// So the transform is gone and the function survives with the opposite meaning:
// after the fix, a bare `0.5.2` in the site's source is not the binary's
// spelling of the current release, it is a spelling NO install path prints. It
// is still worth finding — a hand-typed one is still a literal that goes stale
// at the next tag — but the finding says something different now, and the
// offender message says it.
//
// The template that decides this is held in the root module by
// gateRequireReleaseTransform, which parses .goreleaser.yaml. This module's
// go.mod is chromedp and nothing else, so it cannot read YAML and does not
// pretend to.
func strippedOf(tag string) string { return strings.TrimPrefix(tag, "v") }

// binaryVersionLocs finds every occurrence of a bare version literal that is not
// part of a longer number and not the tail of the `v`-prefixed spelling.
//
// The boundary conditions are what keep this from being a substring search. A
// preceding `v` means the match is the tail of `v0.5.0`, which reVersionLit
// already judges and which would otherwise be reported twice with two different
// stories. A preceding or following digit or dot means the match is part of
// something else entirely — `127.0.0.1` contains `0.0.1`, and content.ts's own
// code samples are full of addresses.
func binaryVersionLocs(body, lit string) [][2]int {
	if lit == "" {
		return nil
	}
	var out [][2]int
	for at := 0; ; {
		i := strings.Index(body[at:], lit)
		if i < 0 {
			return out
		}
		start := at + i
		end := start + len(lit)
		at = start + 1

		if start > 0 {
			switch prev := body[start-1]; {
			case prev == 'v', prev == '.', prev >= '0' && prev <= '9':
				continue
			}
		}
		if end < len(body) {
			switch next := body[end]; {
			case next == '.', next >= '0' && next <= '9':
				continue
			}
		}
		out = append(out, [2]int{start, end})
	}
}

// coveringSpan returns the declaration covering the literal at [start,end), or
// nil. A span covers only literals of its OWN version: a sentence declared for
// "v0.3.0" does not license a "v0.5.0" that appeared inside it.
func coveringSpan(spans []*historySpan, start, end int, lit string) *historySpan {
	for _, s := range spans {
		if start >= s.start && end <= s.end && lit == s.decl.version {
			return s
		}
	}
	return nil
}

func TestNoHardCodedVersionsOutsideTheReleasesArray(t *testing.T) {
	ra := repoReleases(t)
	scan, err := checkNoHardCodedVersions(ra, declaredHistoryLiterals)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.offenders) > 0 {
		t.Fatalf("a version literal is hard-coded in %d renderable place(s) it is not allowed to be:\n  %s\n"+
			"Derive it from latestRelease (content.ts exports latestVersion for exactly this). The "+
			"CURRENT version is correct today and false at the next tag, with nothing rendering "+
			"differently in between; an OLDER one is either already false or a sentence of history, "+
			"and history is declared in declaredHistoryLiterals rather than assumed.\nCurrent release: %s.",
			len(scan.offenders), strings.Join(scan.offenders, "\n  "), ra.current())
	}
	// A declaration that covers nothing is a live exemption nobody is using, and
	// the next literal to land inside that sentence would inherit it silently.
	if len(scan.unused) > 0 {
		t.Fatalf("%d declared history literal(s) cover no renderable literal in %s: %v\n"+
			"The phrase was found — so the sentence is still there — but the version in it renders "+
			"nowhere, which today means it moved into a comment or into the releases array. Delete "+
			"the declaration rather than leaving an exemption standing over prose that does not need one",
			len(scan.unused), ra.path, scan.unused)
	}
	t.Logf("scanned %d TypeScript files under site/src; no renderable version literal outside the "+
		"releases array beyond the %d declared history sentence(s). Current: %s",
		scan.scanned, len(declaredHistoryLiterals), ra.current())
}

// ---------------------------------------------------------------------
// the source lexer, and the two masks the rules read it through
// ---------------------------------------------------------------------

// tsScan lexes a TypeScript/TSX source once and returns two masks over its
// bytes. COMMENT marks what lies inside a `//` or `/* */` comment. INERT marks
// that plus everything inside a string or template literal.
//
// The two callers want different answers, and both answers are right:
//
//   - Rule 3 reads COMMENT. A version literal inside a rendered string is
//     exactly the offender it is hunting, so strings must stay visible; only
//     comments, which no visitor can reach, are exempt.
//   - The structural counting below reads INERT. A `{` inside content.ts's
//     `code:` samples — embedded YAML, JSON envelopes, shell — is not an object
//     opening an entry, and this file is full of them.
//
// It is a lexer rather than a `//`-to-end-of-line strip for one reason that has
// a wrong answer in the dangerous direction: `https://…` is not a comment, and
// treating it as one would mask the REST OF THE LINE — so a hard-coded version
// in rendered text after a link would be waved through. This walks the states
// that decide the question (code, the three string forms, `${}` interpolation
// back into code, and the two comment forms) and, as a belt-and-braces guard for
// a URL that is not inside any string literal — JSX text is neither string nor
// code to a lexer this size — refuses to open a line comment on `://`.
//
// A false NEGATIVE here (a comment read as code) is a loud, fixable failure. A
// false POSITIVE (rendered text read as a comment) is a silent hole, which is
// why every ambiguous case above resolves towards "not a comment".
func tsScan(src string) (comment, inert []bool) {
	const (
		stCode = iota
		stLine
		stBlock
		stSingle
		stDouble
		stTemplate
	)

	comment = make([]bool, len(src))
	inert = make([]bool, len(src))
	mark := func(i int, isComment bool) {
		inert[i] = true
		if isComment {
			comment[i] = true
		}
	}

	state := stCode
	depth := 0     // `{` nesting while in code
	var tmpl []int // brace depth each open `${` interpolation returns at

	for i := 0; i < len(src); i++ {
		c := src[i]
		next := byte(0)
		if i+1 < len(src) {
			next = src[i+1]
		}

		switch state {
		case stCode:
			switch {
			case c == '/' && next == '/' && !(i > 0 && src[i-1] == ':'):
				state = stLine
				mark(i, true)
				mark(i+1, true)
				i++
			case c == '/' && next == '*':
				state = stBlock
				mark(i, true)
				mark(i+1, true)
				i++
			case c == '\'':
				state = stSingle
				mark(i, false)
			case c == '"':
				state = stDouble
				mark(i, false)
			case c == '`':
				state = stTemplate
				mark(i, false)
			case c == '{':
				depth++
			case c == '}':
				if depth > 0 {
					depth--
				}
				if len(tmpl) > 0 && depth == tmpl[len(tmpl)-1] {
					tmpl = tmpl[:len(tmpl)-1]
					state = stTemplate
				}
			}
		case stLine:
			if c == '\n' {
				state = stCode
			} else {
				mark(i, true)
			}
		case stBlock:
			mark(i, true)
			if c == '*' && next == '/' {
				mark(i+1, true)
				i++
				state = stCode
			}
		case stSingle, stDouble:
			mark(i, false)
			switch {
			case c == '\\':
				if i+1 < len(src) {
					mark(i+1, false)
				}
				i++
			case (state == stSingle && c == '\''), (state == stDouble && c == '"'):
				state = stCode
			case c == '\n':
				// Unterminated quote — almost certainly an apostrophe in JSX
				// text. Resync at the line end rather than swallowing the file.
				state = stCode
			}
		case stTemplate:
			mark(i, false)
			switch {
			case c == '\\':
				if i+1 < len(src) {
					mark(i+1, false)
				}
				i++
			case c == '`':
				state = stCode
			case c == '$' && next == '{':
				tmpl = append(tmpl, depth)
				depth++
				i++
				state = stCode
			}
		}
	}
	return comment, inert
}

// tsCommentMask is tsScan's first mask, named for the one rule that wants it.
func tsCommentMask(src string) []bool {
	comment, _ := tsScan(src)
	return comment
}

// topLevelObjects counts the entries of an object/array literal: the `{` that
// open an object at depth zero within src[start:end]. Every byte tsScan marked
// inert is skipped, because content.ts's release notes and embedded code samples
// carry more braces in prose than the file does in structure.
//
// It exists so that "how many entries did I parse" can be checked against "how
// many entries are there" — the difference between a shape this suite cannot
// read failing loudly and it quietly shrinking what the suite is guarding.
func topLevelObjects(src string, inert []bool, start, end int) int {
	depth, n := 0, 0
	for i := start; i < end; i++ {
		if inert[i] {
			continue
		}
		switch src[i] {
		case '{':
			if depth == 0 {
				n++
			}
			depth++
		case '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return n
}

// matchBracket returns the index of the bracket closing the one at open, or -1
// if the source ends first. Inert bytes are skipped for the same reason
// topLevelObjects skips them.
func matchBracket(src string, inert []bool, open int) int {
	depth := 0
	for i := open; i < len(src); i++ {
		if inert[i] {
			continue
		}
		switch src[i] {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// ---------------------------------------------------------------------
// the negative controls
// ---------------------------------------------------------------------

// fixtureRoot writes a synthetic repository root — just enough of site/src for
// the three rules to run over — and returns it.
func fixtureRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// fixtureContent renders a content.ts with the given release entries. Each entry
// is `version|tag`; an empty tag omits the field.
func fixtureContent(doc string, entries ...string) string {
	var b strings.Builder
	b.WriteString(doc)
	b.WriteString("const releases: Release[] = [\n")
	for _, e := range entries {
		version, tag, _ := strings.Cut(e, "|")
		b.WriteString("  {\n")
		fmt.Fprintf(&b, "    version: %q,\n", version)
		if tag != "" {
			fmt.Fprintf(&b, "    tag: %q,\n", tag)
		}
		b.WriteString("  },\n")
	}
	b.WriteString("];\n")
	return b.String()
}

// TestSiteSourceRulesCatchTheirOwnDefects is the mutation test, kept. Each rule
// above passes against site/ today, and a rule that passes against a BAD tree
// too would read as coverage while asserting nothing — so every case here is a
// tree carrying exactly the defect the rule names, plus the healthy twin that
// proves the rule is not simply always red.
//
// It runs over fixtures rather than by editing site/ because site/ is the tree
// under review, and because the other lanes of a gate run are editing it
// concurrently: a test that mutates the subject is a test that can lose someone
// else's work.
func TestSiteSourceRulesCatchTheirOwnDefects(t *testing.T) {
	const doc = "// A comment about v0.5.0, which is history and never rendered.\n"

	// The parse is checked before the rules are, because all three of them are
	// defined against current() and a parse that quietly loses the newest entry
	// re-points every one of them at a release that can no longer go stale.
	t.Run("a version line the parser cannot read fails instead of moving current()", func(t *testing.T) {
		const mangled = `const releases: Release[] = [
  {
    version: "v0.4.0",
  },
  {
    version: 'v0.5.0',
    tag: "Latest release",
  },
];
`
		root := fixtureRoot(t, map[string]string{"site/src/content.ts": mangled})
		ra, err := loadReleasesArray(root)
		if err == nil {
			t.Fatalf("an entry whose version line the regex cannot read was accepted, and current() "+
				"silently became %s — every rule here would then be guarding the PREVIOUS release "+
				"while the one being shipped went unchecked", ra.current())
		}
		t.Logf("unparseable entry rejected: %v", err)

		// The healthy twin. A trailing `//` on a version line is a formatting
		// choice, not a defect, and this is the case that must NOT be rejected.
		//
		// It used to be justified by pointing at the real array, whose entries
		// carried inline comments on their `commit` field. They no longer do —
		// that field was deleted, and no entry carries a comment today — so the
		// justification is now the one at the head of this file: with
		// topLevelObjects cross-checking the count, an unreadable line is already
		// a hard error, which leaves this tolerance able only to prevent a false
		// red and never to hide a real one.
		commented := fixtureRoot(t, map[string]string{
			"site/src/content.ts": `const releases: Release[] = [
  {
    version: "v0.4.0",
  },
  {
    version: "v0.5.0", // re-tagged after the notes fix
    tag: "Latest release",
  },
];
`,
		})
		ra, err = loadReleasesArray(commented)
		if err != nil {
			t.Fatalf("a trailing comment on the version line was rejected: %v", err)
		}
		if ra.current() != "v0.5.0" {
			t.Fatalf("current() is %s, want v0.5.0 — the newest entry was not read", ra.current())
		}
	})

	t.Run("oldest-first catches a prepended release", func(t *testing.T) {
		healthy := fixtureRoot(t, map[string]string{
			"site/src/content.ts": fixtureContent("", "v0.4.0", `v0.5.0|Latest release`),
		})
		ra, err := loadReleasesArray(healthy)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkOldestFirst(ra); err != nil {
			t.Fatalf("appended release rejected: %v", err)
		}

		prepended := fixtureRoot(t, map[string]string{
			"site/src/content.ts": fixtureContent("", `v0.5.0|Latest release`, "v0.4.0"),
		})
		ra, err = loadReleasesArray(prepended)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkOldestFirst(ra); err == nil {
			t.Fatal("a PREPENDED release was accepted; ReleaseTimeline would badge v0.4.0 as latest " +
				"and this rule would have said nothing")
		} else {
			t.Logf("prepend rejected: %v", err)
		}
	})

	t.Run("the latest-release tag must sit on exactly one entry, the last", func(t *testing.T) {
		cases := []struct {
			name    string
			entries []string
			wantErr bool
		}{
			{"one, on the last", []string{"v0.4.0", `v0.5.0|Latest release`}, false},
			{"two", []string{`v0.4.0|Latest release`, `v0.5.0|Latest release`}, true},
			{"none", []string{"v0.4.0", "v0.5.0"}, true},
			{"on an older entry", []string{`v0.4.0|Latest release`, "v0.5.0"}, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := fixtureRoot(t, map[string]string{
					"site/src/content.ts": fixtureContent("", tc.entries...),
				})
				ra, err := loadReleasesArray(root)
				if err != nil {
					t.Fatal(err)
				}
				err = checkExactlyOneLatestTag(ra)
				if tc.wantErr && err == nil {
					t.Fatalf("%s was accepted; the site would name %s current release(s)", tc.name, tc.name)
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("%s was rejected: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("a version literal is caught where it renders and allowed where it is declared", func(t *testing.T) {
		// The two sentences the "narrating an old release" cases below turn on,
		// and the declarations that license them. Everything else in this subtest
		// runs with NO declarations, which is the state a component is always in.
		const history = "export const note = \"v0.3.0 deleted ten verbs, and v0.4.0 removed migrate.\";\n"
		declaredHistory := []historyLiteral{
			{version: "v0.3.0", phrase: "v0.3.0 deleted ten verbs", why: "fixture history"},
			{version: "v0.4.0", phrase: "v0.4.0 removed migrate", why: "fixture history"},
		}

		cases := []struct {
			name        string
			component   string
			contentTail string
			declared    []historyLiteral
			want        int
			wantUnused  int
		}{
			{
				name:      "clean: derived from latestRelease",
				component: "export const Kicker = () => <p>{latestVersion} is out</p>;\n",
				want:      0,
			},
			{
				name:      "the current version, hard-coded in a component",
				component: "export const Kicker = () => <p>v0.5.0 is out</p>;\n",
				want:      1,
			},
			{
				name:      "the current version, hard-coded in a string literal",
				component: "export const example = \"dossierx version → v0.5.0\";\n",
				want:      1,
			},
			{
				// Stale on the day it is written, on a page whose current release
				// is v0.5.0 — and invisible to any rule that only ever looks for
				// the current version.
				name:      "a STALE version, hard-coded in a component",
				component: "export const Kicker = () => <p>v0.4.1 is out</p>;\n",
				want:      1,
			},
			{
				name:      "a line comment, which nobody renders",
				component: "// v0.5.0 chased its own sha through two commits.\nexport const x = 1;\n",
				want:      0,
			},
			{
				name:      "a doc comment, which nobody renders",
				component: "/**\n * The commit field went in v0.5.0 and came out again.\n */\nexport const x = 1;\n",
				want:      0,
			},
			{
				// The reason tsCommentMask is a lexer: strip `//` to end of line
				// and this line's tail disappears with the URL, taking a real
				// rendered version literal with it.
				name:      "after a URL, which does not open a comment",
				component: "export const L = () => <a href=\"https://x.dev/r\">https://x.dev/r v0.5.0</a>;\n",
				want:      1,
			},
			{
				// The carve-out, and the eight live sentences it protects: the
				// site is allowed to say what an old release did — once it has
				// said, in this list, which sentence says it.
				name:        "content.ts narrating an old release, declared",
				component:   "export const x = 1;\n",
				contentTail: history,
				declared:    declaredHistory,
				want:        0,
			},
			{
				// The same two literals with nothing declaring them. This is the
				// case an exemption keyed to "not the current version" waves
				// through, and it is the shape of every stale copy: a version
				// that is not current, sitting in content.ts, saying nothing
				// about the past.
				name:        "content.ts narrating an old release, undeclared",
				component:   "export const x = 1;\n",
				contentTail: history,
				want:        2,
			},
			{
				// The finding this rule was rewritten for, in the words it was
				// reported in: during v0.6.0 prep someone pastes an old
				// transcript over the interpolation in the `dossierx version`
				// example. The literal is not the current version, so a rule
				// keyed to current() reports a fully derived site while the site
				// ships a stale example.
				name:      "a stale transcript pasted over the version example",
				component: "export const x = 1;\n",
				contentTail: "export const versionExample = \"$ dossierx version --format text\\n" +
					"dossierx v0.4.0\\n  commit: 206b4a4\";\n",
				want: 1,
			},
			{
				// …and the edge of the carve-out: the CURRENT version in that
				// same prose is a copy that goes stale at the next tag, not
				// history, and it is not declarable however the list is written.
				name:        "content.ts naming the current release outside the array",
				component:   "export const x = 1;\n",
				contentTail: "export const kicker = \"v0.5.0 is out\";\n",
				declared: []historyLiteral{
					{version: "v0.5.0", phrase: "v0.5.0 is out", why: "an exemption that must not work"},
				},
				// Flagged anyway, and the declaration is reported as covering
				// nothing — which is the truth: the current version is refused
				// before a single span is consulted.
				want:       1,
				wantUnused: 1,
			},
			{
				// A declaration whose sentence is still there but whose version
				// no longer renders: the exemption is live over prose that does
				// not need it, and the next literal to land inside that sentence
				// would inherit it without anyone deciding to grant it.
				name:        "a declaration that covers nothing",
				component:   "export const x = 1;\n",
				contentTail: "// v0.3.0 deleted ten verbs, and nobody renders a comment.\n",
				declared: []historyLiteral{
					{version: "v0.3.0", phrase: "v0.3.0 deleted ten verbs", why: "fixture history"},
				},
				want:       0,
				wantUnused: 1,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := fixtureRoot(t, map[string]string{
					"site/src/content.ts": fixtureContent(doc, "v0.3.0", "v0.4.0", `v0.5.0|Latest release`) +
						tc.contentTail,
					"site/src/Component.tsx": tc.component,
				})
				ra, err := loadReleasesArray(root)
				if err != nil {
					t.Fatal(err)
				}
				scan, err := checkNoHardCodedVersions(ra, tc.declared)
				if err != nil {
					t.Fatal(err)
				}
				if scan.scanned != 2 {
					t.Fatalf("scanned %d files, want 2 — the fixture was not read", scan.scanned)
				}
				if len(scan.offenders) != tc.want {
					t.Fatalf("got %d offender(s) %v, want %d", len(scan.offenders), scan.offenders, tc.want)
				}
				if len(scan.unused) != tc.wantUnused {
					t.Fatalf("got %d unused declaration(s) %v, want %d",
						len(scan.unused), scan.unused, tc.wantUnused)
				}
			})
		}
	})

	// The inventory is only as good as its resolution against the file: a
	// declaration that cannot be located is an exemption whose evidence has gone,
	// and every one of these is an ERROR rather than a finding, because the
	// literal it was written for is still rendering and flagging that literal
	// would report the wrong defect.
	t.Run("a declaration that cannot be resolved fails rather than exempting anything", func(t *testing.T) {
		const tail = "export const note = \"v0.3.0 deleted ten verbs, and v0.3.0 is quoted twice.\";\n"
		cases := []struct {
			name string
			decl historyLiteral
		}{
			{
				name: "the phrase was reworded away",
				decl: historyLiteral{version: "v0.3.0", phrase: "v0.3.0 removed ten verbs", why: "stale"},
			},
			{
				name: "the phrase does not contain its own version",
				decl: historyLiteral{version: "v0.4.0", phrase: "deleted ten verbs", why: "ambiguous"},
			},
			{
				name: "the phrase matches two sentences",
				decl: historyLiteral{version: "v0.3.0", phrase: "v0.3.0 ", why: "too loose"},
			},
			{
				name: "the version is not one the site lists",
				decl: historyLiteral{version: "v9.9.9", phrase: "v9.9.9 never shipped", why: "not a release"},
			},
			{
				name: "the phrase sits inside the releases array",
				decl: historyLiteral{version: "v0.3.0", phrase: "version: \"v0.3.0\"", why: "exempts nothing"},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root := fixtureRoot(t, map[string]string{
					"site/src/content.ts": fixtureContent(doc, "v0.3.0", "v0.4.0", `v0.5.0|Latest release`) + tail,
				})
				ra, err := loadReleasesArray(root)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := checkNoHardCodedVersions(ra, []historyLiteral{tc.decl}); err == nil {
					t.Fatalf("%s was accepted, so an exemption stands over a sentence nobody can point at", tc.name)
				} else {
					t.Logf("rejected: %v", err)
				}
			})
		}
	})
}
