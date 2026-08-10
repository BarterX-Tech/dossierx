// changelog_test.go reads CHANGELOG.md, which nothing in this repository read.
//
// WHY IT EXISTS. Four items in docs/RELEASING.md are about that file — there is
// an entry for the new version, it is dated, breaking and silent-behaviour
// changes are called out first, and the date is the day the tag carries — and
// all four were a maintainer's promise to look. Everything else in the release
// procedure grew a check; this one did not, so the file that tells a consumer
// what changed was the one artifact of a release that could ship saying
// anything at all, including nothing.
//
// That matters more here than the usual "changelogs drift" complaint. This
// project's whole thesis is that a consumer's own gate cannot see a silent
// behaviour change: v0.3.1's renderer expansion changed what already-locked
// claim bodies render as with no edit, no content-hash move and no ledger event,
// and v0.5.0's `mixed-cycle` lint fails a corpus that passed before it with
// nothing on the consumer's side to explain the change. For those, the CHANGELOG
// entry is not documentation of the release — it is the ONLY channel the release
// has. An entry that is missing, undated, or that names a version the rest of
// the tree disagrees with is a release that arrived unannounced.
//
// WHAT IS CHECKED, and against what:
//
//   - every entry heading is `## [<major>.<minor>.<patch>] - <YYYY-MM-DD>`, with
//     a date that is a real calendar day, and the entries run newest first. The
//     ordering is not tidiness: every other rule here says "the newest entry",
//     and the first heading is only the newest one if the file is ordered.
//   - the newest entry names the release the SITE calls current — the last
//     `releases[]` entry in site/src/content.ts, which is where every version
//     string the page renders comes from. Two files, one release: a CHANGELOG
//     that is a version behind the site is how a release ships announcing the
//     previous one's changes.
//   - BREAKING and SILENT items come before the entry's ordinary sections.
//   - the newest entry's date is the day its tag carries, checked against the
//     tag itself when the tag exists and bound to today when it does not.
//
// WHAT IS NOT, said plainly rather than left to be discovered. The callout rule
// is a rule about ORDER, not about classification: it can see an item that says
// BREAKING or SILENT and hold it above the ordinary sections, and it cannot see
// a silent behaviour change that was written up as an ordinary bullet, or one
// that was never written up at all. No test can — that judgement is what the
// two contract snapshots in docs/RELEASING.md are read for. As it stands, the
// current file's only marked callouts are BREAKING ones (v0.5.0, v0.4.0,
// v0.3.0); the SILENT half of the vocabulary is exercised by the fixtures below
// and by nothing in the file yet, which is a fact about the file rather than a
// gap in the rule.
package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	// clogFile is the logbook, relative to the repository root.
	clogFile = "CHANGELOG.md"

	// clogSiteFile is the release stamp: the file the deployed site derives
	// every version string on it from. It is read here for ONE value — which
	// release the page calls current — and never written.
	clogSiteFile = "site/src/content.ts"

	// clogSiteLandmark opens the array of releases, and clogSiteSelection is the
	// expression that picks the current one out of it. Both are required to
	// appear exactly once outside the file's comments, for different reasons:
	// the landmark is what this file reads, and the selection is what makes
	// reading the LAST entry the right answer. See clogSiteCurrentVersion.
	clogSiteLandmark  = "const releases: Release[] = ["
	clogSiteSelection = "releases[releases.length - 1]"

	// clogUnreleasedHeading is the one level-2 heading Keep a Changelog blesses
	// that is not a release. It is skipped rather than refused — but a release
	// cannot hide in it, because the newest DATED entry still has to name what
	// the site calls current.
	clogUnreleasedHeading = "## [Unreleased]"

	// clogDateLayout is ISO 8601, which is what Keep a Changelog specifies and
	// what every entry in the file uses. time.Parse with this layout rejects a
	// day that does not exist, so "2026-02-31" is a failure and not a date.
	clogDateLayout = "2006-01-02"
)

// clogHeadingRE is the ENTIRE grammar of an entry heading, anchored at both
// ends. A heading that does not match is reported, never skipped: "skip what you
// cannot parse" is how an undated entry — the exact defect the dated-entry item
// exists for — becomes invisible to the check written to catch it.
var clogHeadingRE = regexp.MustCompile(`^## \[(\d+)\.(\d+)\.(\d+)\] - (\d{4}-\d{2}-\d{2})$`)

// clogSectionRE is a level-3-or-deeper heading inside an entry: the entry's
// ordinary sections (`### Added`, `### Fixed`, …) and also the shape a callout
// sometimes takes (`### BREAKING — …`). Which of the two it is depends on the
// title, which is clogCalloutMarker's job.
var clogSectionRE = regexp.MustCompile(`^(#{3,6})\s+(.*)$`)

// clogVersionLiteralRE reads one `version: "…"` out of the site's releases array.
var clogVersionLiteralRE = regexp.MustCompile(`version:\s*"([^"]*)"`)

// clogCalloutMarkers is the CLOSED vocabulary of words that make an item a
// callout — a change a consumer's own gate cannot detect for them, which
// docs/RELEASING.md requires at the top of the entry rather than in a bullet
// halfway down.
//
// Two words, matched case-sensitively at the head of the item, because that is
// what distinguishes a callout from prose ABOUT one: v0.3.0's entry says "see
// the BREAKING section above" inside an ordinary bullet three hundred lines
// down, and a rule that read the word anywhere would call that a buried callout
// and be wrong about a correct file. SILENT covers the hyphenated and spaced
// spellings both ("SILENT-BEHAVIOUR", "SILENT RENDER CHANGES") because the
// marker ends at the first character that is not a letter.
var clogCalloutMarkers = []string{"BREAKING", "SILENT"}

// clogRepoRoot locates the repository root from THIS source file rather than
// from the process CWD, so everything here is unaffected by how `go test` is
// invoked. It is spelled out in this file instead of borrowing a neighbour's
// helper for the reason nested_module_coverage_test.go gives for the same
// choice: a check that stops compiling when a neighbouring file is split or
// renamed is a check that stops checking.
func clogRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate the test source, so this file cannot find the repository it is supposed to read")
	}
	return filepath.Dir(filepath.Dir(thisFile)) // <root>/tests/<file> -> <root>
}

// clogRead reads a file addressed relative to the repository root. An unreadable
// file is a failed check and not an absent one: the subject of every assertion
// below is a file, and a missing subject means nothing was examined.
func clogRead(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v\nThis is a FAILED check and not an empty one: with the file gone there is nothing to hold the release procedure's CHANGELOG items against, and a green run here would mean only that nothing was read", rel, err)
	}
	return string(b)
}

// ---------------------------------------------------------------------
// the logbook, parsed
// ---------------------------------------------------------------------

// clogEntry is one release entry: its heading, and the lines under it up to the
// next level-2 heading.
type clogEntry struct {
	version   string // "0.5.0" — as the heading spells it, with no leading `v`
	major     int
	minor     int
	patch     int
	dateText  string    // "2026-08-07"
	date      time.Time // the same day, parsed, so an impossible one cannot pass
	line      int       // 1-based line of the heading
	heading   string
	body      []string
	bodyStart int // 1-based line of body[0]
}

// clogParse reads a CHANGELOG into entries, collecting everything wrong with it
// rather than stopping at the first problem — a maintainer fixing a release
// entry wants the whole list, not one line of it per run.
//
// It returns problems as strings rather than calling t.Error so that the real
// file and every fixture below go through exactly this code. A rule that is
// only exercised against the one document that satisfies it has never been seen
// to fail, which is indistinguishable from a rule that cannot.
func clogParse(doc string) (entries []clogEntry, problems []string) {
	cur := -1

	for i, line := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(line, "## ") {
			if cur >= 0 {
				entries[cur].body = append(entries[cur].body, line)
			}
			continue
		}

		cur = -1 // every level-2 heading closes the entry above it
		if strings.TrimSpace(line) == clogUnreleasedHeading {
			continue
		}
		if !strings.HasPrefix(line, "## [") {
			problems = append(problems, fmt.Sprintf("line %d is a level-2 heading that is neither a release entry nor %q:\n\t%s\nEvery level-2 heading in this file is one release. A section that is neither leaves this file's reader guessing which lines belong to which release, and the callout rule below would judge them against the wrong entry",
				i+1, clogUnreleasedHeading, line))
			continue
		}

		m := clogHeadingRE.FindStringSubmatch(line)
		if m == nil {
			problems = append(problems, fmt.Sprintf("line %d claims to be a release entry and is not one this file can read:\n\t%s\nAn entry heading is exactly `## [<major>.<minor>.<patch>] - <YYYY-MM-DD>`. The two ways this goes wrong are both releases:\n"+
				"  - NO DATE. `## [0.6.0]` is the shape a release entry has while it is being written, and it is the shape it keeps if nobody comes back to it. docs/RELEASING.md requires the entry to be dated because the date is the release's only claim about WHEN, and an undated entry is one this file cannot hold against the tag at all.\n"+
				"  - A LEADING `v`. The site spells the version `v0.6.0` and this file spells it `0.6.0`; Keep a Changelog's heading carries no `v`, and the two spellings are compared with that difference known. Writing the tag's spelling here makes the heading unreadable rather than merely inconsistent",
				i+1, line))
			continue
		}

		day, err := time.Parse(clogDateLayout, m[4])
		if err != nil {
			problems = append(problems, fmt.Sprintf("line %d dates release %s.%s.%s %q, which is not a day that exists: %v.\nA date nobody can be at is not a weaker date than a real one, it is a different kind of thing — no tag can carry it, so the entry can never be checked against the release it describes",
				i+1, m[1], m[2], m[3], m[4], err))
			continue
		}

		// The three components are read as numbers because every ordering rule
		// below compares them as numbers — 0.10.0 is newer than 0.9.0 and no
		// string comparison says so.
		//
		// The regex has already established each one is a run of decimal
		// digits, so the only way this fails is a run too long to be an int.
		// That is still a heading this file cannot read, and it is refused for
		// the same reason the date above is: discarding the error would set the
		// component to Atoi's zero, and a heading silently read as 0.0.0 does
		// not fail anything — it sorts to the bottom, so the ordering rule and
		// the newest-entry rules would all quietly be about a different entry
		// than the reader is looking at.
		var (
			number = [3]int{}
			unread []string
		)
		for k, part := range []struct{ name, text string }{{"major", m[1]}, {"minor", m[2]}, {"patch", m[3]}} {
			n, err := strconv.Atoi(part.text)
			if err != nil {
				unread = append(unread, fmt.Sprintf("%s %q (%v)", part.name, part.text, err))
				continue
			}
			number[k] = n
		}
		if len(unread) > 0 {
			problems = append(problems, fmt.Sprintf("line %d numbers a release with a component this file cannot read as a number — %s:\n\t%s\nAn entry heading's three components are compared AS NUMBERS: that is what makes 0.10.0 newer than 0.9.0, and what every ordering and newest-entry rule below is built on. A component this file cannot turn into a number is a heading it cannot place in that order at all",
				i+1, strings.Join(unread, ", "), line))
			continue
		}

		major, minor, patch := number[0], number[1], number[2]
		entries = append(entries, clogEntry{
			version:   m[1] + "." + m[2] + "." + m[3],
			major:     major,
			minor:     minor,
			patch:     patch,
			dateText:  m[4],
			date:      day,
			line:      i + 1,
			heading:   line,
			bodyStart: i + 2,
		})
		cur = len(entries) - 1
	}

	if len(entries) == 0 {
		problems = append(problems, fmt.Sprintf("%s carries no release entry at all. Every release this project has ever cut is described in exactly one place, and there is nothing here to describe this one — a release with no entry ships with GoReleaser's generated notes, which are commit subjects, and docs/RELEASING.md says in those words that they are not a substitute",
			clogFile))
	}
	return entries, problems
}

// clogOrderProblems requires the entries to run newest first, strictly.
//
// This is what makes `entries[0]` mean "the newest entry" — every other rule in
// this file says those words, and in an unordered file they would be describing
// whichever entry happened to be typed at the top. Equal versions are refused
// too: two entries for one release are two accounts of it, and this file cannot
// say which one a reader is meant to believe.
func clogOrderProblems(entries []clogEntry) []string {
	var problems []string
	for i := 1; i < len(entries); i++ {
		prev, cur := entries[i-1], entries[i]
		if clogCompare(prev, cur) > 0 {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s lists %s (line %d) above %s (line %d), and this file is read newest-entry-first.\n"+
			"Every rule here — which release the site must agree with, which entry's date is held against the tag — is about THE NEWEST ENTRY, and it finds it by taking the first one. In this order the newest release is somewhere in the middle and none of those rules is looking at it",
			clogFile, cur.version, cur.line, prev.version, prev.line))
	}
	return problems
}

// clogCompare orders two entries by version, newest first.
func clogCompare(a, b clogEntry) int {
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] != pair[1] {
			if pair[0] > pair[1] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// ---------------------------------------------------------------------
// the newest entry against the site
// ---------------------------------------------------------------------

// clogSiteProblem holds the newest entry against the release the site calls
// current. The empty string means they agree.
//
// The `v` is stripped from the site's spelling and not added to the CHANGELOG's,
// because the two conventions are both deliberate: the tag and the site carry
// `v0.5.0`, Keep a Changelog's heading carries `0.5.0`, and the comparison has
// to know which side is which rather than accept either spelling anywhere.
func clogSiteProblem(newest clogEntry, siteVersion string) string {
	want := strings.TrimPrefix(siteVersion, "v")
	if newest.version == want {
		return ""
	}
	return fmt.Sprintf("%s's newest entry is %s (line %d), and %s says the current release is %s.\n"+
		"These are two halves of one release announcement and they name different releases. The site's entry is where the hero badge, the release history and the `dossierx version` transcript all get their version from, so whichever of the two is stale, a reader is being told two things:\n"+
		"  - if the CHANGELOG is behind, the release ships describing the PREVIOUS release's changes, and the one change this project cannot detect for a consumer — a silent behaviour change — is the one that goes unannounced.\n"+
		"  - if the SITE is behind, the release stamp does not name the tag, and .github/workflows/release.yml's gate job refuses to publish it at all.\n"+
		"Move whichever one was not moved",
		clogFile, newest.version, newest.line, clogSiteFile, siteVersion)
}

// clogSiteCurrentVersion reads which release the site calls current: the LAST
// entry in content.ts's `releases` array.
//
// WHY THE LAST ONE, and why that is asserted rather than assumed. The page
// selects its current release with `releases[releases.length - 1]`, so taking
// the last literal is only the right answer while that expression is what the
// file says. This file therefore requires the expression to be present, exactly
// once, outside the comments — not as an opinion about how the site should be
// written, but because a site that selected differently would leave this reader
// silently modelling the wrong entry. The full three-declaration model of that
// file (the selection, the timeline's latest index, and the derivation that
// strips the `v`) is held by TestSiteSelectsTheReleaseThisTreeModels in
// cmd/dossierx/gate_release_stamp_test.go; this is the one part of it this
// file's own answer depends on.
//
// Every way of failing to read it is a t.Fatal, in CLAUDE.md's words: an
// unreadable stamp means the release the site claims is current was never
// established, and a comparison against a value that was not read is not a
// comparison.
func clogSiteCurrentVersion(t *testing.T, root string) string {
	t.Helper()

	raw := clogRead(t, root, clogSiteFile)
	code := strings.Join(clogStripLineComments(raw), "\n")

	// A stripper self-check, not an assertion about the site: if the landmark
	// did not survive, the strip is what is wrong, and a maintainer sent to look
	// at a correct file is a maintainer who stops believing this check.
	if !strings.Contains(code, clogSiteLandmark) {
		t.Fatalf("stripping whole-line comments from %s removed %q, which is not a comment. This file's stripper has mis-read the source, so everything below would be running over a truncated file. Fix clogStripLineComments; the site is not what is wrong here",
			clogSiteFile, clogSiteLandmark)
	}
	if n := strings.Count(code, clogSiteLandmark); n != 1 {
		t.Fatalf("%s carries %d live declarations of %q. This file reads exactly one: with two, only one of them is the array the page renders and this file cannot say which, and the version it compared the CHANGELOG against would be a coin toss",
			clogSiteFile, n, clogSiteLandmark)
	}
	if n := strings.Count(code, clogSiteSelection); n != 1 {
		t.Fatalf("%s carries %d live occurrences of %q, and this file needs exactly one.\n"+
			"That expression is what makes the LAST entry in the releases array the release the page calls current, which is the entry this file reads and compares %s against. If the site now selects some other entry, this reader is not stale in a way anybody would notice — it would go on comparing the CHANGELOG against an entry the page does not render. Update this file to model whatever the site now does, or put the selection back",
			clogSiteFile, n, clogSiteSelection, clogFile)
	}

	var versions []string
	inBlock := false
	for _, line := range strings.Split(code, "\n") {
		switch {
		case strings.Contains(line, clogSiteLandmark):
			inBlock = true
		case inBlock && strings.HasPrefix(strings.TrimSpace(line), "];"):
			inBlock = false
		case inBlock:
			if m := clogVersionLiteralRE.FindStringSubmatch(line); m != nil {
				versions = append(versions, m[1])
			}
		}
	}

	if len(versions) == 0 {
		t.Fatalf("%s declares the releases array and no `version:` entry inside it outside the comments. The site names no release at all, so there is nothing for %s's newest entry to agree or disagree with",
			clogSiteFile, clogFile)
	}
	return versions[len(versions)-1]
}

// clogStripLineComments blanks whole-line `//` comments and whole-line `/* … */`
// blocks in TypeScript source, keeping the line count so that everything read
// afterwards still lines up with the file.
//
// It is LINE-BASED on purpose and that is a stated limit, not an oversight. A
// block comment opened part-way along a line of code would not be seen, so a
// declaration hidden that way would still be read as live. What it does close is
// the failure that has actually happened in this repository twice (see
// gate_release_stamp_test.go's corrections FIVE and SIX): a pinned line commented
// out with the wrong one written underneath it, which leaves the original text in
// the file for any check that searches the raw source. Whole-line comments are
// how that is spelled both times.
func clogStripLineComments(src string) []string {
	lines := strings.Split(src, "\n")
	out := make([]string, len(lines))
	inBlock := false
	for i, line := range lines {
		s := strings.TrimSpace(line)
		switch {
		case inBlock:
			if strings.Contains(s, "*/") {
				inBlock = false
			}
		case strings.HasPrefix(s, "/*"):
			if !strings.Contains(strings.TrimPrefix(s, "/*"), "*/") {
				inBlock = true
			}
		case strings.HasPrefix(s, "//"), strings.HasPrefix(s, "*"):
			// a comment line, or a continuation line of a doc block
		default:
			out[i] = line
		}
	}
	return out
}

// ---------------------------------------------------------------------
// callouts before ordinary entries
// ---------------------------------------------------------------------

// clogCalloutProblems requires every marked callout in an entry to stand above
// that entry's ordinary sections.
//
// WHAT "ORDINARY ENTRIES" MEANS HERE: the entry's `###`-or-deeper sections whose
// titles are not themselves callouts — `### Added`, `### Changed`, `### Fixed`.
// It deliberately does NOT mean "every bullet and paragraph", because a callout
// legitimately follows a sentence of context, and both shapes this file already
// contains are correct: v0.5.0 opens with a bold `**BREAKING: …**` paragraph
// before its first `### Added`, and v0.4.0 and v0.3.0 make BREAKING their first
// `###` section. What the rule refuses is the shape docs/RELEASING.md names in
// those words — a change a consumer's gate cannot detect for them, filed as "a
// bullet halfway down".
func clogCalloutProblems(entry clogEntry) []string {
	firstOrdinary, firstOrdinaryLine, title := -1, 0, ""
	var problems []string

	for i, line := range entry.body {
		marker := clogCalloutMarker(line)
		if m := clogSectionRE.FindStringSubmatch(strings.TrimSpace(line)); m != nil && marker == "" && firstOrdinary < 0 {
			firstOrdinary, firstOrdinaryLine, title = i, entry.bodyStart+i, m[2]
			continue
		}
		if marker == "" || firstOrdinary < 0 || i <= firstOrdinary {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s's %s entry buries a %s item at line %d, below the `%s` section that opens at line %d:\n\t%s\n"+
			"docs/RELEASING.md puts these first for one reason: a %s change is one a consumer's own gate CANNOT detect for them. `dossierx check` reports exactly what it reported before, no content hash moves, nothing flips to review_pending — the entry is the only notice they get. A reader who stops at the section headings has already missed it. Move it above `%s`, or make it the entry's first section",
			clogFile, entry.version, marker, entry.bodyStart+i, title, firstOrdinaryLine, strings.TrimSpace(line), marker, title))
	}
	return problems
}

// clogCalloutMarker returns the callout marker a line opens with, or "".
//
// A line is a callout when the marker is the FIRST thing it says, once the
// wrappers a marked-up document puts in front of a phrase are taken off: the
// `###` of a heading, a bullet's `-`, and bold's `**`. Anything after those has
// to begin with the marker word, ending at a character that is not a letter — so
// "SILENT-BEHAVIOUR" and "BREAKING:" are markers, "BREAKINGLY" is a word, and
// "see the BREAKING section above" is prose about a callout somewhere else.
func clogCalloutMarker(line string) string {
	s := strings.TrimSpace(line)
	if m := clogSectionRE.FindStringSubmatch(s); m != nil {
		s = m[2]
	}
	s = strings.TrimLeft(s, "-*+ \t")
	for _, marker := range clogCalloutMarkers {
		if !strings.HasPrefix(s, marker) {
			continue
		}
		rest := s[len(marker):]
		if rest == "" || !clogIsLetter(rest[0]) {
			return marker
		}
	}
	return ""
}

// clogIsLetter reports whether b is an ASCII letter, which is what decides
// whether a marker ended or is the start of a longer word.
func clogIsLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// ---------------------------------------------------------------------
// every rule over one document
// ---------------------------------------------------------------------

// clogAudit runs every rule that needs only the document and the site's version.
// The real CHANGELOG.md and every fixture in this file go through this one
// function, so a rule cannot be true of the fixtures and absent from the real
// read, or the other way round.
func clogAudit(doc, siteVersion string) []string {
	entries, problems := clogParse(doc)
	problems = append(problems, clogOrderProblems(entries)...)
	if len(entries) == 0 {
		return problems
	}
	if p := clogSiteProblem(entries[0], siteVersion); p != "" {
		problems = append(problems, p)
	}
	for _, entry := range entries {
		problems = append(problems, clogCalloutProblems(entry)...)
	}
	return problems
}

// TestChangelogHoldsTheReleaseItDescribes is every rule above, over the real
// file and the real release stamp.
func TestChangelogHoldsTheReleaseItDescribes(t *testing.T) {
	root := clogRepoRoot(t)
	site := clogSiteCurrentVersion(t, root)

	for _, problem := range clogAudit(clogRead(t, root, clogFile), site) {
		t.Errorf("%s\n", problem)
	}
}

// ---------------------------------------------------------------------
// the date the tag carries
// ---------------------------------------------------------------------

// clogTag is what a release tag can tell this file about its date. `exists` is
// the whole point of the type: the newest entry's tag is usually absent while
// the release is being prepared, and the rule that applies then is a different,
// weaker one that says so.
type clogTag struct {
	name   string
	exists bool
	ownDay string // the tag's day in the timezone whoever cut it was in
	utcDay string // the same instant's day in UTC
}

// clogDateProblem holds the newest entry's date against the day the tag carries.
// The empty string means it holds.
//
// TAG EXISTS: the two candidate days are the tagger's own and UTC's, and either
// is accepted. A calendar day is not a fact about an instant until a timezone is
// chosen, and the two plausible answers here are the day the person cutting the
// release was living in and the day the forge records — they differ for a few
// hours either side of midnight, and picking one would make a correct entry red
// on those days.
//
// TAG DOES NOT EXIST — the release in progress, which is the state this file is
// read in during a release. There is no tag, so the day the release will
// actually carry is UNKNOWN and this rule cannot check the thing the checklist
// item is about. What it can do is refuse a date that no tag cut from this tree
// could carry: today, give or take a day for whatever timezone the entry was
// written in. That catches the failure that actually happens — a date copied
// forward from the previous release, or one typed when the entry was drafted a
// week ago — and it says in its message that the real comparison happens once
// the tag exists.
func clogDateProblem(newest clogEntry, tag clogTag, today time.Time) string {
	if tag.exists {
		if newest.dateText == tag.ownDay || newest.dateText == tag.utcDay {
			return ""
		}
		return fmt.Sprintf("%s dates %s %s (line %d), and %s carries %s.\n"+
			"docs/RELEASING.md's rule is that the entry's date IS the date the tag carries — it is the release's only claim about when it happened, and it is read by people who cannot see the tag. The tag is the fact here and the entry is the claim, so the entry is what moves.\n"+
			"(Both timezone readings of the tag were accepted: %s where it was cut, %s in UTC.)",
			clogFile, newest.version, newest.dateText, newest.line, tag.name, tag.ownDay, tag.ownDay, tag.utcDay)
	}

	todayDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	delta := int(newest.date.Sub(todayDay).Hours() / 24)
	if delta >= -1 && delta <= 1 {
		return ""
	}

	when := "in the past"
	if delta > 0 {
		when = "in the future"
	}
	return fmt.Sprintf("%s dates %s %s (line %d), which is %d days %s. There is no %s tag yet, so this is the release in progress.\n"+
		"WHAT THIS CHECK CANNOT SEE, exactly: with no tag, the day the release will actually carry does not exist yet, and the checklist item this enforces — the entry's date is the tag's date — cannot be checked at all. What it can rule out is a date no tag cut from this tree could carry. Today is %s; a day either side is allowed, because the entry is written by a person in some timezone and the tag will be cut by a runner in UTC.\n"+
		"A date this far off is one of two things: copied forward from the previous release, or written when the entry was drafted and never moved. Re-date the entry on the day you tag it, and this rule becomes the real comparison against the tag the moment the tag exists",
		clogFile, newest.version, newest.dateText, newest.line, abs(delta), when, tag.name, todayDay.Format(clogDateLayout))
}

// abs is int absolute value, for the message above.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// clogGit runs one git command at the repository root, in UTC so that a date
// this file renders is the same date wherever it runs. A git that cannot answer
// is a t.Fatal: this rule's whole subject is what the repository's tags say, and
// an unanswered question is not a passed check.
func clogGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "TZ=UTC")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v\nThe newest entry's date is checked against the tag that release carries, and that means reading this repository's tags. A git that cannot be run here has not reported that the entry is fine; it has reported nothing",
			strings.Join(args, " "), err)
	}
	return string(out)
}

// clogTagFor resolves the tag a version's release carries, if it has been cut.
//
// THE TAGLESS CHECKOUT IS THE CASE THIS FUNCTION EXISTS FOR. "This tag does not
// exist" and "this checkout has no tags" look identical at the call site and
// mean opposite things: the first is a release being prepared, the second is
// actions/checkout at its default depth, where taking the release-in-progress
// branch would quietly hold every entry against today's date and pass. So the
// repository is asked whether it has ANY tags first, and a checkout with none
// fails in the same words render_across_releases_test.go uses for an
// unresolvable baseline — name the fix, stay red until it lands.
func clogTagFor(t *testing.T, root, version string) clogTag {
	t.Helper()
	name := "v" + version

	if strings.TrimSpace(clogGit(t, root, "for-each-ref", "--count=1", "--format=%(refname)", "refs/tags")) == "" {
		t.Fatalf("this checkout carries no tags at all, so it cannot be asked what date %s's release was tagged on.\n"+
			"This is a failure and not a skip. A clone with no tags is exactly what actions/checkout produces at its default depth, and taking the \"no tag yet\" branch here would hold the newest entry against TODAY and pass — the release-in-progress rule, applied to every released version, forever.\n"+
			"Fetch them:\n    git fetch --tags --force\nIn CI, check out with `fetch-depth: 0` — the shape .github/workflows/ci.yml already uses.",
			name)
	}

	out := strings.TrimSpace(clogGit(t, root,
		"for-each-ref",
		"--format=%(creatordate:format:%Y-%m-%d)\t%(creatordate:format-local:%Y-%m-%d)",
		"refs/tags/"+name))
	if out == "" {
		return clogTag{name: name}
	}

	days := strings.Split(out, "\t")
	if len(days) != 2 || days[0] == "" || days[1] == "" {
		t.Fatalf("git reported %s's date as %q, which this file cannot read as two days. It asked for the tag's own timezone and UTC; without both it cannot say whether the entry's date is the tag's",
			name, out)
	}
	return clogTag{name: name, exists: true, ownDay: days[0], utcDay: days[1]}
}

// TestChangelogNewestEntryCarriesTheDateItsTagDoes is the date rule against the
// real repository — the tag when there is one, today when the release has not
// been cut yet.
func TestChangelogNewestEntryCarriesTheDateItsTagDoes(t *testing.T) {
	root := clogRepoRoot(t)

	entries, problems := clogParse(clogRead(t, root, clogFile))
	if len(problems) > 0 {
		t.Fatalf("%s does not parse, so its newest entry cannot be held against any tag. TestChangelogHoldsTheReleaseItDescribes reports what is wrong with it:\n\t%s",
			clogFile, strings.Join(problems, "\n\t"))
	}

	newest := entries[0]
	if problem := clogDateProblem(newest, clogTagFor(t, root, newest.version), time.Now().UTC()); problem != "" {
		t.Error(problem)
	}
}

// ---------------------------------------------------------------------
// the fixtures: every rule, watched failing
// ---------------------------------------------------------------------

// clogFixtureSite is the release stamp every document fixture is judged against,
// so that a fixture disagreeing with the site is doing it on purpose.
const clogFixtureSite = "v0.6.0"

// clogGood is a correct entry: dated, newest first, agreeing with the site, and
// with its BREAKING callout above the ordinary sections. Every fixture below is
// this document with ONE thing wrong, which is what makes the failure it
// produces attributable to that one thing.
const clogGood = `# Changelog

## [0.6.0] - 2026-08-09

**BREAKING: the lint that nothing detects for you.** A corpus that passed before
this release exits 1 after it, with no edit on your side.

### Added — a thing

- an ordinary bullet

## [0.5.0] - 2026-08-07

### Fixed — another thing

- see the BREAKING section above, which is prose and not a callout
`

// TestChangelogRulesRefuseTheDocumentsTheyExistFor is the four failures the
// release procedure's CHANGELOG items are about, each constructed and each
// watched being refused — plus the four ways the parser has to stay strict for
// those four to be reachable at all.
//
// The `want` strings are what the maintainer reading the failure has to be told:
// which release, which line, and which of the two files disagrees. A rule that
// fires with a message nobody can act on has caught the defect and not reported
// it.
func TestChangelogRulesRefuseTheDocumentsTheyExistFor(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		site string
		want []string
	}{
		{
			name: "a missing entry",
			doc:  "# Changelog\n\nAll notable changes to this project will be documented in this file.\n",
			site: clogFixtureSite,
			want: []string{"carries no release entry at all", "not a substitute"},
		},
		{
			name: "an undated entry",
			doc:  strings.Replace(clogGood, "## [0.6.0] - 2026-08-09", "## [0.6.0]", 1),
			site: clogFixtureSite,
			want: []string{"claims to be a release entry", "NO DATE"},
		},
		{
			name: "a version disagreeing with the site",
			doc:  clogGood,
			site: "v0.7.0",
			want: []string{"newest entry is 0.6.0", "says the current release is v0.7.0", "refuses to publish it"},
		},
		{
			name: "a BREAKING item buried below ordinary items",
			doc: strings.Replace(clogGood,
				"**BREAKING: the lint that nothing detects for you.** A corpus that passed before\nthis release exits 1 after it, with no edit on your side.\n\n### Added — a thing\n\n- an ordinary bullet\n",
				"### Added — a thing\n\n- an ordinary bullet\n- **BREAKING: the lint that nothing detects for you.** A corpus that passed before this release exits 1 after it.\n",
				1),
			site: clogFixtureSite,
			want: []string{"buries a BREAKING item", "CANNOT detect for them", "Added — a thing"},
		},
		{
			name: "a SILENT-BEHAVIOUR item buried below ordinary items",
			doc: strings.Replace(clogGood,
				"- an ordinary bullet\n",
				"- an ordinary bullet\n- **SILENT-BEHAVIOUR: every viewer re-renders three times larger.** No content hash moves.\n",
				1),
			site: clogFixtureSite,
			want: []string{"buries a SILENT item"},
		},
		{
			name: "the tag's spelling in the heading",
			doc:  strings.Replace(clogGood, "## [0.6.0] -", "## [v0.6.0] -", 1),
			site: clogFixtureSite,
			want: []string{"A LEADING `v`"},
		},
		{
			// The heading matches the grammar — three runs of digits and a real
			// date — and one of the runs is longer than an int. Reading it as
			// Atoi's zero would file this release below every other entry in
			// the document, so the order rule and the newest-entry rules would
			// pass while describing a different release than the top of the
			// file shows.
			name: "a version component no number can hold",
			doc:  strings.Replace(clogGood, "## [0.6.0] - 2026-08-09", "## [99999999999999999999.0.0] - 2026-08-09", 1),
			site: clogFixtureSite,
			want: []string{"cannot read as a number", "major \"99999999999999999999\""},
		},
		{
			name: "a date no calendar has",
			doc:  strings.Replace(clogGood, "2026-08-09", "2026-02-31", 1),
			site: clogFixtureSite,
			want: []string{"not a day that exists"},
		},
		{
			name: "the newest entry is not the first one",
			doc: strings.Replace(
				strings.Replace(clogGood, "## [0.6.0] - 2026-08-09", "## [0.4.0] - 2026-07-01", 1),
				"## [0.5.0] - 2026-08-07", "## [0.6.0] - 2026-08-09", 1),
			site: clogFixtureSite,
			want: []string{"newest-entry-first"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems := strings.Join(clogAudit(tc.doc, tc.site), "\n")
			if problems == "" {
				t.Fatalf("this document was accepted, and it is the %s case.\nA rule that never refuses the document it was written for is indistinguishable from one that was deleted:\n%s", tc.name, tc.doc)
			}
			for _, want := range tc.want {
				if !strings.Contains(problems, want) {
					t.Errorf("the refusal never says %q, so the maintainer reading it is not told what this rule caught. It says:\n%s", want, problems)
				}
			}
		})
	}
}

// TestChangelogAcceptsACorrectDocument is the other half of the fixtures above,
// and it is not decoration. Eight documents being refused proves nothing on its
// own — a rule that refuses everything refuses them too, and it would be found
// only by whoever next tried to cut a release.
func TestChangelogAcceptsACorrectDocument(t *testing.T) {
	if problems := clogAudit(clogGood, clogFixtureSite); len(problems) > 0 {
		t.Errorf("a correct changelog was refused %d time(s). Every rule in this file is one a maintainer meets while cutting a release, and one that fires on a correct document is one they learn to work around:\n\t%s",
			len(problems), strings.Join(problems, "\n\t"))
	}
}

// TestChangelogDateRuleRefusesTheDatesItExistsFor is the date rule's own
// fixtures, held without a repository: the tag is supplied rather than resolved,
// so both branches — the tag that exists and the release still in progress — are
// exercised on every run, including the one that this tree cannot be in.
func TestChangelogDateRuleRefusesTheDatesItExistsFor(t *testing.T) {
	today := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	entry := func(day string) clogEntry {
		parsed, err := time.Parse(clogDateLayout, day)
		if err != nil {
			t.Fatalf("fixture date %q: %v", day, err)
		}
		return clogEntry{version: "0.6.0", dateText: day, date: parsed, line: 8}
	}
	tagged := func(own, utc string) clogTag {
		return clogTag{name: "v0.6.0", exists: true, ownDay: own, utcDay: utc}
	}

	for _, tc := range []struct {
		name  string
		entry clogEntry
		tag   clogTag
		want  string
	}{
		{"the tag's day, as the tagger saw it", entry("2026-08-09"), tagged("2026-08-09", "2026-08-08"), ""},
		{"the tag's day in UTC", entry("2026-08-08"), tagged("2026-08-09", "2026-08-08"), ""},
		{"a date the tag does not carry", entry("2026-08-01"), tagged("2026-08-09", "2026-08-09"), "the entry is the claim"},
		{"no tag yet, dated today", entry("2026-08-09"), clogTag{name: "v0.6.0"}, ""},
		{"no tag yet, a timezone either side", entry("2026-08-10"), clogTag{name: "v0.6.0"}, ""},
		{"no tag yet, copied forward from the last release", entry("2026-07-21"), clogTag{name: "v0.6.0"}, "WHAT THIS CHECK CANNOT SEE"},
		{"no tag yet, dated next week", entry("2026-08-20"), clogTag{name: "v0.6.0"}, "days in the future"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := clogDateProblem(tc.entry, tc.tag, today)
			switch {
			case tc.want == "" && got != "":
				t.Errorf("a correct date was refused:\n%s", got)
			case tc.want != "" && got == "":
				t.Errorf("%s was accepted. The date is the release's only claim about when it happened, and this is the rule that holds it to one", tc.name)
			case tc.want != "" && !strings.Contains(got, tc.want):
				t.Errorf("the refusal never says %q. It says:\n%s", tc.want, got)
			}
		})
	}
}
