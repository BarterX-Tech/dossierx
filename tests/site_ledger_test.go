package tests

// THE RELEASE STAMP THE SITE CARRIES, AND THE ONE READER OF IT.
//
// The release driver refuses to publish unless the TREE declares the release
// being tagged, and it establishes that from two statements which must agree:
// CHANGELOG.md's newest version heading, and the site's own ledger. This file
// is the whole of the second half — every test in this package that needs to
// know what release the site calls current comes through siteCurrentRelease,
// so the site's stamp cannot be read one way by one check and another way by
// another.
//
// WHAT MOVED, AND WHY THE READER IS DIFFERENT NOW. The ledger used to be a
// TypeScript array in site/src/content.ts, rendered by a React component, and
// "the current release" meant "the last element" — a position. That took three
// separate expressions in two files to state (the array's order, the page's
// `releases[releases.length - 1]` selection, and the timeline's own
// `releases.length - 1`), all of which had to agree, and a prepended entry
// silently demoted the new release while the page went on looking correct. The
// site is now static HTML with no build, and the entry says so about itself:
//
//	<article class="release" data-version="v0.5.1" data-current="true">
//
// The improvement is not the syntax. It is that ORDER NO LONGER CARRIES
// MEANING. A reordering of the page cannot change which release it names as
// current, because the claim is on the entry rather than in its position, and
// exactly one entry may carry it — nought is a page that names no current
// release, and two is a page that names two.
//
// EVERY FAILURE TO READ IT IS FATAL, in CLAUDE.md's terms. An unreadable stamp
// means the release the site claims is current was never established, and a
// comparison against a value that was not read is not a comparison. There is no
// path through this file that reports "could not check" and lets a caller
// proceed.
//
// COMMENTS ARE STRIPPED BEFORE ANYTHING IS READ, and that is not tidiness. The
// failure this repository has actually shipped twice is a pinned line commented
// out with a new one written underneath it (gate_release_stamp_test.go's
// corrections FIVE and SIX), which leaves the original text in the file for any
// check that searches raw source. releases.html carries a long comment that
// names v0.4.1 and a sha, precisely the kind of prose a raw scan would trip on.

import (
	"regexp"
	"strings"
	"testing"
)

// siteLedgerFile is the release stamp, relative to the repository root. It is
// read, never written.
const siteLedgerFile = "site/releases.html"

// siteArticleRE matches one opening <article> tag. The attributes inside it are
// read separately rather than pinned in a fixed order: an attribute reshuffle is
// a formatting change, and a reader that turned it into a failed release would
// be a reader nobody trusts.
var siteArticleRE = regexp.MustCompile(`(?s)<article\b[^>]*>`)

// siteVersionAttrRE and siteCurrentAttrRE read the two attributes that make an
// entry a release entry and one release entry the current one.
var (
	siteVersionAttrRE = regexp.MustCompile(`data-version="(v\d+\.\d+\.\d+)"`)
	siteCurrentAttrRE = regexp.MustCompile(`data-current="true"`)
)

// siteHTMLCommentRE is a whole HTML comment, including across lines.
var siteHTMLCommentRE = regexp.MustCompile(`(?s)<!--.*?-->`)

// siteCurrentRelease is the version the site calls current, as `vX.Y.Z`.
func siteCurrentRelease(t *testing.T) string {
	t.Helper()

	raw := readRepoFile(t, siteLedgerFile)
	code := siteStripComments(raw)

	// A stripper self-check, not an assertion about the site: if every entry
	// vanished, the strip is what is wrong, and a maintainer sent to look at a
	// correct page is a maintainer who stops believing this check.
	if !strings.Contains(code, "data-version=") {
		t.Fatalf("stripping HTML comments from %s removed every `data-version` attribute, which no comment contains. This file's stripper has mis-read the page, so everything below would run over a truncated document. Fix siteStripComments; the site is not what is wrong here", siteLedgerFile)
	}

	var versions []string
	var current []string
	for _, tag := range siteArticleRE.FindAllString(code, -1) {
		m := siteVersionAttrRE.FindStringSubmatch(tag)
		if m == nil {
			continue
		}
		versions = append(versions, m[1])
		if siteCurrentAttrRE.MatchString(tag) {
			current = append(current, m[1])
		}
	}

	if len(versions) == 0 {
		t.Fatalf("%s carries no `<article … data-version=\"vX.Y.Z\">` entry outside its comments. The site names no release at all, so there is nothing for %s's newest heading to agree or disagree with", siteLedgerFile, clogFile)
	}

	switch len(current) {
	case 1:
		return current[0]
	case 0:
		t.Fatalf("%s lists %d release entr(ies) and none of them carries `data-current=\"true\"`.\n\n"+
			"That attribute is the ONLY thing on the page that says which release is current — the order of the entries deliberately means nothing, so there is no fallback to read and this check will not invent one. The release driver compares this value against %s's newest heading before it will tag anything, so a page with no marked entry does not fail at release time, it refuses at release time.\n\n"+
			"Put `data-current=\"true\"` on the newest entry.", siteLedgerFile, len(versions), clogFile)
	default:
		t.Fatalf("%s marks %d entries `data-current=\"true\"` (%s). Exactly one release can be current.\n\n"+
			"This is what the previous ledger's failure looked like from the other side: a page naming two current releases renders perfectly and is wrong about the one fact it exists to state. Whichever entry this check picked would be a coin toss, so it picks none.\n\n"+
			"Move the attribute onto the newest entry and off every other.", siteLedgerFile, len(current), strings.Join(current, ", "))
	}
	return "" // unreachable: every case above is fatal
}

// siteStripComments blanks HTML comments while keeping the byte count, so that
// offsets into the result still line up with the file a maintainer opens.
//
// The limit, stated rather than left to be discovered: this is a regex over a
// text file and not an HTML parser, so a `<!--` inside an attribute value or a
// script would be read as a comment opener. Neither exists on this page and
// both would be strange things to add to it; what the strip DOES close is the
// failure that has happened here — live text left standing inside a comment for
// a raw scan to find.
func siteStripComments(src string) string {
	return siteHTMLCommentRE.ReplaceAllStringFunc(src, func(c string) string {
		return strings.Repeat(" ", len(c))
	})
}

// siteReleaseVersions is every version the ledger lists, in page order, for the
// checks that care about the set rather than the current one.
func siteReleaseVersions(t *testing.T) []string {
	t.Helper()

	code := siteStripComments(readRepoFile(t, siteLedgerFile))
	var versions []string
	for _, tag := range siteArticleRE.FindAllString(code, -1) {
		if m := siteVersionAttrRE.FindStringSubmatch(tag); m != nil {
			versions = append(versions, m[1])
		}
	}
	return versions
}

// TestSiteLedgerNamesExactlyOneCurrentRelease is siteCurrentRelease over the
// real page, so the stamp is checked on its own rather than only as a step
// inside whichever comparison happened to run first.
func TestSiteLedgerNamesExactlyOneCurrentRelease(t *testing.T) {
	current := siteCurrentRelease(t)
	versions := siteReleaseVersions(t)
	seen := map[string]int{}
	for _, v := range versions {
		seen[v]++
	}
	for v, n := range seen {
		if n > 1 {
			t.Errorf("%s lists %s %d times. A release appears once; a duplicated entry is a page that cannot be read as a ledger of what shipped", siteLedgerFile, v, n)
		}
	}

	if len(versions) > 0 && versions[0] != current {
		t.Errorf("%s marks %s current, but the FIRST entry on the page is %s.\n\n"+
			"Nothing breaks — the marker is what this repository reads, and it is read correctly. But the page presents itself newest-first, so a visitor takes the top entry as the current release and this one disagrees with the badge below it. Fix the order, or move the marker.",
			siteLedgerFile, current, versions[0])
	}
}

// TestSiteLedgerCarriesNoCommitSha refuses the field that was deleted for being
// unable to converge: writing the sha is itself a commit, so the value was stale
// the moment it landed — v0.4.1 shipped naming `5327923` while `refs/tags/v0.4.1`
// pointed at `206b4a4`. The old ledger's own type declaration was deleted with
// it, because a declared-but-unused field is how a deleted thing comes back
// quietly. Static HTML has no type declaration to delete, so the refusal has to
// be this: no entry may carry a sha-shaped attribute or a sha-shaped word.
func TestSiteLedgerCarriesNoCommitSha(t *testing.T) {
	code := siteStripComments(readRepoFile(t, siteLedgerFile))

	shaRE := regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
	for i, line := range strings.Split(code, "\n") {
		for _, hit := range shaRE.FindAllString(line, -1) {
			// A colour literal is hex and is not a sha; the ledger carries no
			// styling, but saying so costs one condition and a false red here
			// would be a release refused for a stylesheet.
			if strings.Contains(line, "#"+hit) {
				continue
			}
			t.Errorf("%s:%d carries %q, which reads as a commit sha.\n\n%s",
				siteLedgerFile, i+1, hit,
				"The ledger's `commit` field was deleted outright rather than fixed, because it could not be right: writing the sha is a commit, so the value is stale as it lands. Two releases shipped naming the wrong one. Nothing on this page may name a commit.")
		}
	}
}
