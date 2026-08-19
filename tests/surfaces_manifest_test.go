// surfaces_manifest_test.go makes an undeclared surface a build failure instead
// of a silent gap.
//
// The release gate reads this project's prose against its code, one read-only
// agent per client-facing surface. That is exactly as complete as the surface
// list, and the list used to live in a scope document — so a file that a client
// can observe could be added with nothing anywhere to notice it, and the only
// way to find the omission was to audit for it. Auditing for a missing thing is
// the most expensive kind of review and the least reliable.
//
// So: surfaces.yaml names every surface and, beside it, every path DECLARED out
// of scope with its reason, and this test asserts every tracked file is claimed
// by EXACTLY ONE entry. Add a file that matches neither and CI is red; add an
// out-of-scope pattern broad enough to swallow a real surface and CI is red
// naming both entries, because a coverage list that can be narrowed by accident
// is the same failure as a gate that samples.
//
// "Tracked" is `git ls-files`, deliberately: a file git does not carry cannot
// reach a client, and taking the working tree instead would fail on every
// scratch file a contributor has not cleaned up yet — a gate that fires on
// correct state is one people learn to switch off.
package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// surfacesManifestFile is the committed manifest, relative to the repository
// root.
const surfacesManifestFile = "surfaces.yaml"

// surfaceManifest is surfaces.yaml. The two halves are separate keys rather than
// one list with an "in scope" boolean because they are read by different
// people for different reasons: the surfaces are the gate's fan-out, and the
// exclusions are an argument that has to be re-read whenever somebody suspects
// the gate is missing something.
type surfaceManifest struct {
	Surfaces   []surfaceEntry `yaml:"surfaces"`
	OutOfScope []surfaceEntry `yaml:"out_of_scope"`
}

// surfaceEntry is one surface or one declared exclusion. What/Reach describe a
// surface; Reason justifies an exclusion. Not carves exceptions out of Paths.
type surfaceEntry struct {
	Name   string   `yaml:"name"`
	What   string   `yaml:"what"`
	Reach  string   `yaml:"reach"`
	Reason string   `yaml:"reason"`
	Paths  []string `yaml:"paths"`
	Not    []string `yaml:"not"`
	// Reads are documents this surface does NOT own, whose bytes its reviewing
	// agent needs in order to judge its own — README.md describing behaviour
	// FORMAT.md defines, a skill quoting an error message the CLI owns. They
	// take no part in ownership: `claims` below reads Paths and Not only, so a
	// reads: entry never makes this entry a second claimant and never disturbs
	// the exactly-one rule — and a file that is only ever borrowed and never
	// owned is still unclaimed and still reddens the coverage test. See
	// surfaces.yaml's header for what the list is for and the rules it answers
	// to.
	Reads []string `yaml:"reads"`
}

// TestEveryTrackedFileIsDeclaredASurfaceOrExcluded is the whole point of the
// manifest: no tracked file may be neither.
func TestEveryTrackedFileIsDeclaredASurfaceOrExcluded(t *testing.T) {
	root := repoRoot(t)
	manifest := loadSurfaceManifest(t, root)
	entries := allSurfaceEntries(manifest)

	var unclaimed, contested []string
	for _, file := range trackedFiles(t, root) {
		var owners []string
		for _, entry := range entries {
			if entry.claims(t, file) {
				owners = append(owners, entry.Name)
			}
		}
		switch len(owners) {
		case 0:
			unclaimed = append(unclaimed, file)
		case 1:
			// exactly one owner: the state this test exists to hold
		default:
			contested = append(contested, fmt.Sprintf("%s (claimed by %s)", file, strings.Join(owners, ", ")))
		}
	}

	if len(unclaimed) > 0 {
		t.Errorf("%d tracked file(s) are neither declared a client-facing surface nor explicitly excluded in %s.\n"+
			"Add each to a surface (the release gate will then review it) or to an out_of_scope entry WITH A REASON.\n  %s",
			len(unclaimed), surfacesManifestFile, strings.Join(unclaimed, "\n  "))
	}
	if len(contested) > 0 {
		t.Errorf("%d tracked file(s) are claimed by more than one entry in %s.\n"+
			"Exactly one entry must own each file: an out_of_scope pattern that also matches a surface's file is how coverage narrows without anyone deciding to narrow it.\n  %s",
			len(contested), surfacesManifestFile, strings.Join(contested, "\n  "))
	}
}

// TestSurfaceManifestEntriesAreWellFormed keeps the manifest readable as an
// argument rather than as a list of globs: every surface says what it is and how
// far a wrong statement travels, and every exclusion carries the reason it is
// out of scope. An exclusion with no reason is the thing this file is written to
// prevent, one indirection later.
func TestSurfaceManifestEntriesAreWellFormed(t *testing.T) {
	root := repoRoot(t)
	manifest := loadSurfaceManifest(t, root)

	if len(manifest.Surfaces) == 0 {
		t.Fatalf("%s declares no surfaces", surfacesManifestFile)
	}
	if len(manifest.OutOfScope) == 0 {
		t.Fatalf("%s declares no exclusions; every repository has files a client cannot observe, so an empty list means the argument was never made", surfacesManifestFile)
	}

	seen := map[string]bool{}
	for _, entry := range allSurfaceEntries(manifest) {
		if entry.Name == "" {
			t.Errorf("an entry with paths %v has no name", entry.Paths)
			continue
		}
		if seen[entry.Name] {
			t.Errorf("two entries are named %q", entry.Name)
		}
		seen[entry.Name] = true
		if len(entry.Paths) == 0 {
			t.Errorf("entry %q claims no paths", entry.Name)
		}
	}
	for _, entry := range manifest.Surfaces {
		if strings.TrimSpace(entry.What) == "" {
			t.Errorf("surface %q does not say what it is", entry.Name)
		}
		if strings.TrimSpace(entry.Reach) == "" {
			t.Errorf("surface %q does not say how far a wrong statement in it travels; that is what ranks the gate's fan-out", entry.Name)
		}
	}
	for _, entry := range manifest.OutOfScope {
		if strings.TrimSpace(entry.Reason) == "" {
			t.Errorf("out_of_scope entry %q carries no reason; an undeclared exclusion is the gap this manifest exists to close", entry.Name)
		}
	}
}

// TestReadsDoesNotCountTowardTheExactlyOneRule pins the scoping that makes
// `reads:` possible at all.
//
// A surface entry may carry a `reads:` list naming documents it does NOT own
// but whose bytes its reviewing agent needs — see surfaces.yaml's own header
// for why that exists. Ownership stays with `paths:`, and this file's central
// rule is that every tracked file is claimed by EXACTLY ONE entry. If the
// coverage walk ever started counting `reads:` as a claim, the first borrow to
// land would make its target doubly-claimed and redden the build immediately,
// and the obvious repair — deleting the reads: entry — throws the mechanism
// away. So the property is asserted directly rather than left to the coverage
// test to discover: a file named in some surface's reads: is claimed by
// whichever entry owns it, and by that entry alone.
//
// WHAT THIS DELIBERATELY DOES NOT LICENSE: borrowing is not a way to satisfy
// coverage. A file that appears only in reads: lists and in no entry's paths:
// is still unclaimed, and TestEveryTrackedFileIsDeclaredASurfaceOrExcluded
// still fails over it — the synthetic check at the bottom holds that reading of
// `claims` in place, because the real manifest cannot: every real borrow
// targets a file something else already owns, so the borrow-only case never
// arises there until the day it matters.
func TestReadsDoesNotCountTowardTheExactlyOneRule(t *testing.T) {
	root := repoRoot(t)
	manifest := loadSurfaceManifest(t, root)

	var referenced []string
	for _, entry := range manifest.Surfaces {
		referenced = append(referenced, entry.Reads...)
	}
	// NOT a skip. surfaces.yaml declares reads: entries today, so an empty set
	// here means this test's decode of the manifest has diverged from the file
	// — a renamed field, a moved key — and every assertion below would pass
	// over nothing, which is indistinguishable from a clean run and must never
	// be.
	if len(referenced) == 0 {
		t.Fatalf("%s declares no reads: entries as this test decodes it, but the manifest carries them; the decode has gone stale and the assertions below would pass over zero borrows", surfacesManifestFile)
	}

	for _, file := range referenced {
		var claimants []string
		for _, entry := range allSurfaceEntries(manifest) {
			if entry.claims(t, file) {
				claimants = append(claimants, entry.Name)
			}
		}
		if len(claimants) != 1 {
			t.Errorf("%q is named in a surface's reads: and is claimed by %d entries (%v); it must be claimed by exactly one.\n"+
				"reads: is not a claim — ownership is decided by paths: alone. If the coverage walk has started counting reads: as ownership, fix the walk rather than the manifest: deleting the reads: entry to make this green removes material an agent needs and re-opens the coverage gap it was declared to close.",
				file, len(claimants), claimants)
		}
	}

	// The synthetic half: an entry that BORROWS a file must not CLAIM it. This
	// is what keeps a borrow-only file unclaimed — and therefore still a red
	// build — no matter what the real manifest happens to borrow today.
	borrower := surfaceEntry{Name: "borrower", Paths: []string{"docs/OWNED.md"}, Reads: []string{"docs/BORROWED.md"}}
	if borrower.claims(t, "docs/BORROWED.md") {
		t.Error("an entry claims a file it only reads: borrowing has been folded into ownership, so a borrow-only file would read as covered while no agent is responsible for reviewing it — coverage narrowed with nobody deciding to narrow it")
	}
	if !borrower.claims(t, "docs/OWNED.md") {
		t.Error("the synthetic entry does not claim its own paths: file, so the assertion above proves nothing")
	}
}

// TestReadsEntriesAreExactTrackedPathsSomebodyElseOwns holds the rules a
// reads: entry answers to, at the manifest level.
//
// The gate enforces the same rules in gateSurfaceReferences
// (cmd/dossierx/gate_fingerprint_test.go), as refusals on the run path, which
// is what makes them checks rather than tests. They are asserted here as well
// because this file is where a person edits the manifest: a failure here names
// the entry and the rule in one place, instead of surfacing as a refused
// fan-out several steps into a release.
func TestReadsEntriesAreExactTrackedPathsSomebodyElseOwns(t *testing.T) {
	root := repoRoot(t)
	manifest := loadSurfaceManifest(t, root)
	tracked := trackedFiles(t, root)

	isTracked := make(map[string]bool, len(tracked))
	for _, file := range tracked {
		isTracked[file] = true
	}

	// Only surfaces may borrow. An out_of_scope entry has no reviewing agent,
	// so a reads: list on one is material handed to nobody — a stated need the
	// gate can never satisfy, sitting in the manifest looking satisfied.
	for _, entry := range manifest.OutOfScope {
		if len(entry.Reads) > 0 {
			t.Errorf("out_of_scope entry %q declares reads: %v; exclusions have no reviewing agent, so there is nobody to hand the borrowed bytes to", entry.Name, entry.Reads)
		}
	}

	for _, entry := range manifest.Surfaces {
		seen := map[string]bool{}
		for _, rel := range entry.Reads {
			switch {
			case strings.ContainsAny(rel, "*?") || strings.HasSuffix(rel, "/"):
				t.Errorf("surface %q reads %q, which is a pattern. reads: takes exact repository-relative paths: a surface borrowing another's material names what it borrowed, and a glob lets the borrowed set grow as the other surface does without anyone deciding — and makes the surface's key depend on directory contents", entry.Name, rel)
			case !isTracked[rel]:
				t.Errorf("surface %q reads %q, which is not a tracked file. If the file moved, move this entry with it — the gate refuses the whole fan-out over an unresolvable borrow rather than dropping it, because a dropped one leaves the agent reporting the coverage gap this list exists to close", entry.Name, rel)
			case seen[rel]:
				t.Errorf("surface %q reads %q twice", entry.Name, rel)
			case entry.claims(t, rel):
				t.Errorf("surface %q reads %q, which its own paths: already claim. reads: is for documents another surface owns — borrowing your own is either a stale entry or a paths: pattern that has grown, and those are different edits", entry.Name, rel)
			}
			seen[rel] = true
		}
	}
}

// loadSurfaceManifest reads and decodes surfaces.yaml. A missing or unreadable
// manifest is a failure, never a skip: with no manifest there is no coverage
// claim at all, and "we did not check" must not read as "it is fine".
func loadSurfaceManifest(t *testing.T, root string) surfaceManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, surfacesManifestFile))
	if err != nil {
		t.Fatalf("read %s: %v", surfacesManifestFile, err)
	}
	var manifest surfaceManifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse %s: %v", surfacesManifestFile, err)
	}
	return manifest
}

// allSurfaceEntries is both halves in one slice, which is how the coverage check
// reads them: a file is claimed by an entry, and whether that entry means "the
// gate reviews this" or "the gate deliberately does not" is the entry's own
// business.
func allSurfaceEntries(m surfaceManifest) []surfaceEntry {
	out := make([]surfaceEntry, 0, len(m.Surfaces)+len(m.OutOfScope))
	out = append(out, m.Surfaces...)
	out = append(out, m.OutOfScope...)
	return out
}

// claims reports whether this entry owns file: it matches one of the entry's
// paths and none of its exceptions.
func (e surfaceEntry) claims(t *testing.T, file string) bool {
	t.Helper()
	matched := false
	for _, pattern := range e.Paths {
		if matchSurfacePattern(t, pattern, file) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	for _, pattern := range e.Not {
		if matchSurfacePattern(t, pattern, file) {
			return false
		}
	}
	return true
}

// surfacePatternCache memoizes the compiled form of each pattern. The manifest
// holds a few dozen patterns and the tree a few hundred files, so compiling per
// comparison would be thousands of needless regexp builds.
var surfacePatternCache = map[string]*regexp.Regexp{}

// matchSurfacePattern implements the grammar surfaces.yaml documents in its own
// header: a trailing "/" claims a whole subtree, "**/" spans any number of
// directory segments, "*" spans one segment, and anything else is an exact path.
//
// It is written out rather than reached for from a library because the grammar
// is part of the manifest's contract — a reader has to be able to predict what a
// line claims without knowing which globbing package this test happens to
// import.
func matchSurfacePattern(t *testing.T, pattern, file string) bool {
	t.Helper()
	re, ok := surfacePatternCache[pattern]
	if !ok {
		var b strings.Builder
		b.WriteString("^")
		rest := pattern
		for rest != "" {
			switch {
			case strings.HasPrefix(rest, "**/"):
				b.WriteString(`(?:[^/]+/)*`)
				rest = rest[3:]
			case strings.HasPrefix(rest, "*"):
				b.WriteString(`[^/]*`)
				rest = rest[1:]
			default:
				next := strings.IndexAny(rest, "*")
				if next < 0 {
					next = len(rest)
				}
				b.WriteString(regexp.QuoteMeta(rest[:next]))
				rest = rest[next:]
			}
		}
		if strings.HasSuffix(pattern, "/") {
			b.WriteString(".+")
		}
		b.WriteString("$")
		compiled, err := regexp.Compile(b.String())
		if err != nil {
			t.Fatalf("pattern %q in %s does not compile: %v", pattern, surfacesManifestFile, err)
		}
		surfacePatternCache[pattern] = compiled
		re = compiled
	}
	return re.MatchString(file)
}

// trackedFiles is `git ls-files` — the set of files this repository actually
// carries. A failure to run git fails the test rather than emptying the set:
// an empty file list would make every assertion above pass over zero
// assertions, which is indistinguishable from a clean run and must never be.
func trackedFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var files []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			files = append(files, filepath.ToSlash(name))
		}
	}
	if len(files) == 0 {
		t.Fatal("git ls-files reported no tracked files; the coverage assertion would pass over nothing")
	}
	sort.Strings(files)
	return files
}
