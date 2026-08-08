// release_notes_predict_test.go covers release_notes_predict.go's predictor
// at three levels:
//
//   - TestPredictReleaseNotes_FixedFixture pins the pure algorithm (filter,
//     sort, group, render) against a hand-built commit list with a known
//     right answer, independent of this repository's own history so it never
//     drifts as that history grows.
//
//   - TestLoadReleaseNotesConfig_MatchesCommittedGoreleaserYAML pins the
//     parser against the REAL .goreleaser.yaml, including the "^Merge "
//     exclude this lane added — this is the mutation-tested guard for that
//     one-line config fix.
//
//   - TestPredictReleaseNotesForRange_MergeCommitExcluded runs the whole
//     pipeline, git invocation included, over a FRESH, HERMETICALLY BUILT
//     repository (a temp dir this test git-inits and commits into itself)
//     rather than this repository's own history, and confirms a --no-ff
//     merge commit's own subject is excluded, not silently swept into
//     "Other changes" the way it would be without the fix — proving the
//     config change actually closes the gap it claims to.
//
//     This test used to run the identical assertion against this
//     repository's real, immutable v0.4.1..v0.5.0 range (merge commit
//     eab3a63, "Merge pull request #32 — v0.5.0, a claims graph in the
//     viewer" — that range and commit are still what .goreleaser.yaml's own
//     changelog.filters.exclude comment cites as the motivating real-world
//     case). It was rebuilt hermetic because a test that reads the ambient
//     clone's TAG SET is not a test of this predictor: it is a test of
//     whatever checkout happened to produce the tree it is running in, and
//     that setting lives in another file, owned by another concern, free to
//     change without anyone reading this one. `git log v0.4.1..v0.5.0` exits
//     128 in a checkout that fetched no tags — a shallow clone, a
//     `--no-tags` mirror, a fork, a CI checkout at its default depth — and
//     that failure says nothing whatever about whether "^Merge " excludes a
//     merge commit.
//
//     The reason is deliberately stated WITHOUT reference to how ci.yml
//     checks out today, because an earlier version of this paragraph did the
//     opposite: it asserted, as the justification, that CI's checkout "is
//     depth-1 with no tags fetched, so `git log v0.4.1..v0.5.0` fails with
//     exit 128 on every one of ci.yml's six 'test' matrix cells". That
//     sentence stopped being true the moment the `test` job gained
//     `fetch-depth: 0` — tags are fetched on all six cells now — and a
//     rationale that a one-line edit to an unrelated file can falsify was
//     never the real one. The decision is unchanged and remains the better
//     one either way: a hermetic repo needs nothing from the ambient
//     checkout and proves the same claim regardless of clone depth, which is
//     also why this test would keep working if that `fetch-depth: 0` were
//     ever removed again.
//
//   - TestPredictReleaseNotesForRange_PublishedEqualAcrossMergeBoundary
//     builds on the same hermetic repo to prove PublishedEqual's contract:
//     predicting over the branch range (before the release PR's merge
//     lands) and over the merge-commit range (after) must NOT be
//     reflect.DeepEqual (Dropped differs by exactly the merge commit) but
//     MUST be PublishedEqual (Body and Groups — what actually
//     publishes — are identical) — this is the check G2 runs against G1's
//     recorded prediction.
//
//   - TestPublishedEqual_CatchesNewlyDroppedCommit and
//     TestCompareReleaseNotesPrediction pin the OTHER half of that same
//     contract: a docs:/chore: commit that lands between G1 and G2 (not the
//     merge commit — an ordinary user-visible change that happens to have a
//     dropped subject prefix) must NOT compare PublishedEqual, because that
//     is precisely the change a "Body only" comparison can never see (a
//     dropped commit's whole effect is its absence from Body).
//
//   - TestPredictReleaseNotes_GroupRegexMatchesSubjectOnly pins
//     PredictReleaseNotes's group-matching target against
//     goreleaser/goreleaser/v2@v2.17.1's real behavior (subject only, never
//     the sha-prefixed line) — see release_notes_predict_lib_test.go's
//     package doc, item 4, for the citation and for why this project's own
//     committed regexes never surfaced the earlier, wrong version of this.
//
// TestPredictReleaseNotesForRange_G1Capture is the reusable entry point
// itself, serving BOTH gate stages:
//
//	go test ./tests -run TestPredictReleaseNotesForRange_G1Capture -args \
//	  -release-notes-range=v0.5.0..HEAD \
//	  -release-notes-predict-out=/path/to/g1-prediction.json
//
// is how G1 records a prediction for the branch range, and
//
//	go test ./tests -run TestPredictReleaseNotesForRange_G1Capture -args \
//	  -release-notes-range=v0.5.0..<merge-sha> \
//	  -release-notes-predict-compare=/path/to/g1-prediction.json \
//	  -release-notes-predict-out=/path/to/g2-prediction.json
//
// is how G2 re-runs the SAME code over the merge-commit range, fails the
// test (not silently passes) if CompareReleaseNotesPrediction finds the two
// are not PublishedEqual, and records its own prediction for G3 to compare
// against the real published release body.
package tests

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var (
	releaseNotesRange          = flag.String("release-notes-range", "", "git revision range (e.g. v0.5.0..HEAD) to predict GoReleaser's changelog for; empty skips TestPredictReleaseNotesForRange_G1Capture's write")
	releaseNotesPredictOut     = flag.String("release-notes-predict-out", "", "write the -release-notes-range prediction here as JSON; requires -release-notes-range")
	releaseNotesPredictCompare = flag.String("release-notes-predict-compare", "", "path to a previously recorded ReleaseNotesPrediction JSON (G1's -release-notes-predict-out) to compare the fresh -release-notes-range prediction against; requires -release-notes-range. This is G2's check — the test fails if they are not PublishedEqual.")

	releaseNotesPredictedJSON = flag.String("release-notes-predicted-json", "", "path to a previously recorded ReleaseNotesPrediction JSON (G2's -release-notes-predict-out) to check against -release-notes-published-body; requires -release-notes-published-body. This is G3's check.")
	releaseNotesPublishedBody = flag.String("release-notes-published-body", "", "path to a file holding the REAL published GitHub release body (e.g. the output of `gh release view <tag> --json body -q .body`); requires -release-notes-predicted-json. This is G3's check — the test fails unless PublishedBodyMatches reports Matched.")
)

// ---------------------------------------------------------------------
// 1. the pure algorithm, pinned against a fixed fixture
// ---------------------------------------------------------------------

// fixtureReleaseNotesConfig mirrors this project's real .goreleaser.yaml
// changelog stanza (post-fix) byte for byte in SHAPE, so the fixed-fixture
// test below is exercising the same group/exclude rules a reader of
// .goreleaser.yaml sees, without depending on that file's exact bytes (that
// dependency is TestLoadReleaseNotesConfig_MatchesCommittedGoreleaserYAML's
// job, not this one's).
func fixtureReleaseNotesConfig() ReleaseNotesConfig {
	return ReleaseNotesConfig{
		Sort: "asc",
		Groups: []ReleaseNotesGroup{
			{Title: "Features", Regexp: `^.*?feat(\([[:word:]]+\))??!?:.+$`, Order: 0},
			{Title: "Bug fixes", Regexp: `^.*?fix(\([[:word:]]+\))??!?:.+$`, Order: 1},
			{Title: "Other changes", Regexp: "", Order: 999},
		},
		Filters: ReleaseNotesFilters{Exclude: []string{"^chore:", "^docs:", "^Merge "}},
	}
}

func TestPredictReleaseNotes_FixedFixture(t *testing.T) {
	// The Features group deliberately holds FOUR entries in an order that is
	// NOT already subject-ascending — "cli" before "breaking" before "zzz"
	// before "aaa" — the reverse of what changelog.sort: asc must produce
	// ("breaking", "aaa", "cli", "zzz": '!' < '(' in ASCII, then "aaa" <
	// "cli" < "zzz" lexicographically). A fixture whose surviving lines
	// already happen to land in ascending order (the previous version of
	// this fixture, two Features entries that were coincidentally already
	// sorted) passes identically whether the sort step runs or is deleted
	// outright — this shape is the one place that distinction is provable.
	lines := []string{
		"aaa1111 feat(cli): add widget export",
		"ggg7777 feat!: breaking change to config",
		"hhh8888 feat(zzz): last-alphabetically feature",
		"iii9999 feat(aaa): first-alphabetically feature",
		"bbb2222 fix(render): correct table overflow",
		"ccc3333 docs: update README",
		"ddd4444 chore: bump deps",
		"eee5555 Merge pull request #32 — v0.5.0",
		"fff6666 refactor(internal): tidy helper",
	}

	got, err := PredictReleaseNotes(lines, fixtureReleaseNotesConfig())
	if err != nil {
		t.Fatalf("PredictReleaseNotes: %v", err)
	}

	// Dropped: exactly the docs/chore/merge lines, each tagged with the
	// pattern that caught it.
	wantDropped := map[string]string{
		"update README":             "^docs:",
		"bump deps":                 "^chore:",
		"pull request #32 — v0.5.0": "^Merge ",
	}
	if len(got.Dropped) != len(wantDropped) {
		t.Fatalf("dropped: got %d entries, want %d: %+v", len(got.Dropped), len(wantDropped), got.Dropped)
	}
	for _, d := range got.Dropped {
		matched := false
		for suffix, pattern := range wantDropped {
			if strings.HasSuffix(d.Subject, suffix) {
				matched = true
				if d.ExcludedBy != pattern {
					t.Errorf("dropped %q: excluded_by = %q, want %q", d.Subject, d.ExcludedBy, pattern)
				}
			}
		}
		if !matched {
			t.Errorf("unexpected dropped commit: %+v", d)
		}
	}

	// Groups: asc-sorted by subject text across the WHOLE surviving set
	// before grouping ("feat!: ..." sorts before every "feat(...): ..." "
	// because '!' < '(' in ASCII; among the three "feat(...)" entries,
	// "aaa" < "cli" < "zzz" lexicographically), grouped in config order,
	// catch-all last regardless of its own position in that order. This
	// order is the OPPOSITE of the input lines' order above — proving the
	// sort step ran, not just that grouping did.
	if len(got.Groups) != 3 {
		t.Fatalf("groups: got %d, want 3 (Features, Bug fixes, Other changes): %+v", len(got.Groups), got.Groups)
	}
	wantGroups := []struct {
		title    string
		subjects []string
	}{
		{"Features", []string{
			"feat!: breaking change to config",
			"feat(aaa): first-alphabetically feature",
			"feat(cli): add widget export",
			"feat(zzz): last-alphabetically feature",
		}},
		{"Bug fixes", []string{"fix(render): correct table overflow"}},
		{"Other changes", []string{"refactor(internal): tidy helper"}},
	}
	for i, want := range wantGroups {
		g := got.Groups[i]
		if g.Title != want.title {
			t.Fatalf("group[%d].Title = %q, want %q", i, g.Title, want.title)
		}
		if len(g.Commits) != len(want.subjects) {
			t.Fatalf("group %q: got %d commits, want %d: %+v", g.Title, len(g.Commits), len(want.subjects), g.Commits)
		}
		for j, subj := range want.subjects {
			if g.Commits[j].Subject != subj {
				t.Errorf("group %q commit[%d] = %q, want %q", g.Title, j, g.Commits[j].Subject, subj)
			}
		}
	}

	wantBody := "## Changelog\n" +
		"### Features\n" +
		"* ggg7777 feat!: breaking change to config\n" +
		"* iii9999 feat(aaa): first-alphabetically feature\n" +
		"* aaa1111 feat(cli): add widget export\n" +
		"* hhh8888 feat(zzz): last-alphabetically feature\n" +
		"### Bug fixes\n" +
		"* bbb2222 fix(render): correct table overflow\n" +
		"### Other changes\n" +
		"* fff6666 refactor(internal): tidy helper\n"
	if got.Body != wantBody {
		t.Fatalf("Body mismatch.\ngot:\n%s\nwant:\n%s", got.Body, wantBody)
	}
}

// TestPredictReleaseNotes_GroupRegexMatchesSubjectOnly pins the fix to the
// finding this lane closes: goreleaser/goreleaser/v2@v2.17.1's
// formatChangelog matches a group's regexp against entry.Message — the
// SUBJECT ALONE, changelog.go:185's `re.MatchString(entry.Message)` — never
// against the raw "<sha> <subject>" git-log line. This project's own
// committed group regexes (`^.*?feat...`, `^.*?fix...`) never surface the
// distinction because a hex sha cannot spell "feat" or "fix" and the `^.*?`
// prefix happily eats the sha either way. A regexp anchored directly at the
// subject's own start, with no such prefix, does surface it: matched
// against the full line the sha pushes "feat:" away from offset 0 and the
// match silently fails.
func TestPredictReleaseNotes_GroupRegexMatchesSubjectOnly(t *testing.T) {
	cfg := ReleaseNotesConfig{
		Sort: "asc",
		Groups: []ReleaseNotesGroup{
			{Title: "Features", Regexp: `^feat:`, Order: 0},
			{Title: "Other changes", Order: 999},
		},
	}
	lines := []string{"deadbeefcafe feat: add widget export"}

	got, err := PredictReleaseNotes(lines, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotes: %v", err)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("groups: got %d, want 1 (the commit must land in Features, not fall through to a catch-all): %+v", len(got.Groups), got.Groups)
	}
	if got.Groups[0].Title != "Features" {
		t.Fatalf("the commit landed in %q, want %q — a subject-anchored regexp must match the subject, not the sha-prefixed line", got.Groups[0].Title, "Features")
	}
	if len(got.Groups[0].Commits) != 1 || got.Groups[0].Commits[0].Subject != "feat: add widget export" {
		t.Fatalf("Features group commits = %+v, want exactly the one feat: commit", got.Groups[0].Commits)
	}
}

// TestPredictReleaseNotes_GroupsSortedByOrderNotConfigListPosition is the
// regression test for the BLOCKING finding this lane fixes:
// PredictReleaseNotes's `sort.SliceStable(groups, ...)` re-sort by each
// group's own `order` field was previously deletable with zero test
// failures, because every fixture in this file — including
// fixtureReleaseNotesConfig and the real, committed .goreleaser.yaml — lists
// its groups in a config order that already equals ascending `order`
// (Features:0, Bug fixes:1, Other changes:999), so the sort step is a no-op
// on every input the suite fed it.
//
// GoReleaser's own formatChangelog does not require that coincidence: a
// group's `order` is an independent field from its position in the
// changelog.groups LIST, and goreleaser's own docs and schema allow them to
// disagree (nothing validates that config-list order matches `order`). This
// fixture deliberately lists Features (order 10) BEFORE Bug fixes (order 0)
// — the reverse of their own `order` values — so a predictor that renders
// groups in config-list order instead of `order`-ascending order produces a
// PUBLISHED body with "### Features" before "### Bug fixes", the wrong way
// around, while one that sorts by `order` correctly renders "### Bug fixes"
// first. Deleting the sort step (or a .goreleaser.yaml edit that reorders
// groups without updating `order`) is caught here and nowhere else in this
// suite — TestPredictReleaseNotes_FixedFixture's groups happen to already
// agree on both axes, so it cannot distinguish "sorted by order" from
// "emitted in config-list order".
func TestPredictReleaseNotes_GroupsSortedByOrderNotConfigListPosition(t *testing.T) {
	cfg := ReleaseNotesConfig{
		Sort: "asc",
		Groups: []ReleaseNotesGroup{
			// Listed Features-then-Bug-fixes, but `order` says the opposite.
			{Title: "Features", Regexp: `^feat:`, Order: 10},
			{Title: "Bug fixes", Regexp: `^fix:`, Order: 0},
		},
	}
	lines := []string{
		"aaa1111 feat: add widget export",
		"bbb2222 fix: correct table overflow",
	}

	got, err := PredictReleaseNotes(lines, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotes: %v", err)
	}

	if len(got.Groups) != 2 {
		t.Fatalf("groups: got %d, want 2: %+v", len(got.Groups), got.Groups)
	}
	// Order-ascending: Bug fixes (order 0) must render BEFORE Features
	// (order 10), even though Features was listed first in cfg.Groups.
	if got.Groups[0].Title != "Bug fixes" || got.Groups[1].Title != "Features" {
		t.Fatalf("groups = [%q, %q], want [\"Bug fixes\", \"Features\"] — groups must be rendered in ascending `order`, not in changelog.groups list position", got.Groups[0].Title, got.Groups[1].Title)
	}

	bugFixesAt := strings.Index(got.Body, "### Bug fixes")
	featuresAt := strings.Index(got.Body, "### Features")
	if bugFixesAt < 0 || featuresAt < 0 {
		t.Fatalf("expected both \"### Bug fixes\" and \"### Features\" headings in the body, got:\n%s", got.Body)
	}
	if bugFixesAt > featuresAt {
		t.Fatalf("\"### Bug fixes\" (order 0) must appear before \"### Features\" (order 10) in the rendered body, got:\n%s", got.Body)
	}
}

// An empty range (nothing landed) must still render "## Changelog" alone —
// GoReleaser's own formatChangelog always emits the header regardless of
// whether any group claimed anything.
func TestPredictReleaseNotes_EmptyRange(t *testing.T) {
	got, err := PredictReleaseNotes(nil, fixtureReleaseNotesConfig())
	if err != nil {
		t.Fatalf("PredictReleaseNotes: %v", err)
	}
	if got.Body != "## Changelog\n" {
		t.Fatalf("Body = %q, want %q", got.Body, "## Changelog\n")
	}
	if len(got.Groups) != 0 || len(got.Dropped) != 0 {
		t.Fatalf("expected no groups and no dropped commits, got groups=%+v dropped=%+v", got.Groups, got.Dropped)
	}
}

// ---------------------------------------------------------------------
// 1b. PublishedBodyMatches — G3's check against the REAL published body
// ---------------------------------------------------------------------
//
// The finding this section closes: the documented contract used to be
// "prediction.Body == published body", which holds for an ordinary release
// (v0.4.1's published body has "## Changelog" at byte 0) and silently breaks
// on a breaking one (v0.5.0's published body opens with hand-written
// breaking-change prose ahead of "## Changelog", per
// docs/RELEASING.md's checklist). These tests pin PublishedBodyMatches, the
// replacement, against exactly that shape.

// TestPublishedBodyMatches_OrdinaryRelease is the v0.4.1 shape: nothing
// before "## Changelog", nothing after but a single trailing newline —
// PublishedBodyMatches must accept the degenerate "no prefix" case, not just
// the breaking-release case with a prefix.
func TestPublishedBodyMatches_OrdinaryRelease(t *testing.T) {
	p := ReleaseNotesPrediction{Body: "## Changelog\n### Features\n* aaa1111 feat: add widget\n"}
	got := p.PublishedBodyMatches(p.Body)
	if !got.Matched {
		t.Fatalf("PublishedBodyMatches(p.Body).Matched = false, want true for an unmodified body with no hand-written prefix")
	}
	if !got.AnchorFound {
		t.Fatalf("PublishedBodyMatches(p.Body).AnchorFound = false, want true — the body itself starts with the anchor")
	}
}

// TestPublishedBodyMatches_ToleratesHandWrittenPrefix is the v0.5.0 shape:
// hand-written breaking-change prose ahead of the generated section. This is
// the exact scenario the false contract missed.
func TestPublishedBodyMatches_ToleratesHandWrittenPrefix(t *testing.T) {
	p := ReleaseNotesPrediction{Body: "## Changelog\n### Features\n* aaa1111 feat: add widget\n"}
	published := "Read this before upgrading.\n\nBREAKING — `dossierx check` fails on a mixed cycle.\n\n" + p.Body
	got := p.PublishedBodyMatches(published)
	if !got.Matched {
		t.Fatalf("PublishedBodyMatches did not tolerate a hand-written prefix ahead of \"## Changelog\":\npublished:\n%s", published)
	}
	if !got.AnchorFound {
		t.Fatalf("AnchorFound = false, want true — the anchor IS present, just not at byte 0")
	}
}

// TestPublishedBodyMatches_ToleratesTrailingWhitespace covers the OTHER
// difference the MAJOR finding's evidence called out: v0.5.0's published body
// ends with three trailing newlines against GoReleaser's own single trailing
// newline (plausibly introduced by a human re-saving the release notes
// through GitHub's own editor). Meaningless whitespace must not fail a
// prediction that is otherwise exactly right.
func TestPublishedBodyMatches_ToleratesTrailingWhitespace(t *testing.T) {
	p := ReleaseNotesPrediction{Body: "## Changelog\n### Features\n* aaa1111 feat: add widget\n"}
	published := "some preamble\n\n" + p.Body + "\n\n"
	if got := p.PublishedBodyMatches(published); !got.Matched {
		t.Fatalf("PublishedBodyMatches did not tolerate extra trailing whitespace after the generated section:\npublished:\n%q", published)
	}
}

// TestPublishedBodyMatches_CatchesRealDrift is the negative control: this
// must not be a check that a hand-written prefix makes vacuously true no
// matter what follows it. A generated section whose content actually differs
// from the prediction — here, a commit's subject was hand-edited in the
// published body, the exact kind of drift G3 exists to catch — must fail.
func TestPublishedBodyMatches_CatchesRealDrift(t *testing.T) {
	p := ReleaseNotesPrediction{Body: "## Changelog\n### Features\n* aaa1111 feat: add widget\n"}
	published := "preamble\n\n## Changelog\n### Features\n* aaa1111 feat: add a WIDGET (edited after tagging)\n"
	got := p.PublishedBodyMatches(published)
	if got.Matched {
		t.Fatalf("Matched = true for a generated section that does not match the prediction; drift must be caught, not waved through because a prefix preceded it")
	}
	if !got.AnchorFound {
		t.Fatalf("AnchorFound = false, want true — the anchor IS present; this is real drift, not a missing section")
	}
}

// TestPublishedBodyMatches_NoAnchorFails: a published body with no "##
// Changelog" line at all must NOT be Matched — CLAUDE.md's rule that a check
// which cannot run is a failure, not a silent pass, applies here exactly as
// it does to every other gate check in this project. It must also, distinctly,
// report AnchorFound: false — see TestPublishedBodyMatches_NoAnchorIsDistinctFromDrift
// for why that second field exists and is not redundant with Matched.
func TestPublishedBodyMatches_NoAnchorFails(t *testing.T) {
	p := ReleaseNotesPrediction{Body: "## Changelog\n### Features\n* aaa1111 feat: add widget\n"}
	got := p.PublishedBodyMatches("This release has no changelog section at all.\n")
	if got.Matched {
		t.Fatalf("Matched = true for a published body with no \"## Changelog\" line; want false")
	}
	if got.AnchorFound {
		t.Fatalf("AnchorFound = true for a published body with no \"## Changelog\" line; want false")
	}
}

// TestPublishedBodyMatches_NoAnchorIsDistinctFromDrift is the regression test
// for the finding this lane fixes: PublishedBodyMatches used to return a bare
// bool, so a published body with NO generated section at all (this project's
// OWN v0.2.0 and v0.3.0 release bodies — confirmed via `gh api
// repos/BarterX-Tech/dossierx/releases/tags/v0.2.0` and `...v0.3.0`, both
// entirely hand-written prose with zero "^## Changelog$" lines, even though
// .goreleaser.yaml carried the same changelog: stanza at both tags) compared
// identically-false to a published body whose generated section IS present
// but whose content actually disagrees with the prediction — real drift, the
// shape PublishedBodyMatches exists to catch. A human confirming a BLOCKING
// G3 finding could not tell "this release's process may be broken" apart
// from "this release's author replaced the section on purpose" from the
// verdict alone. AnchorFound is the fix: false for the former, true for the
// latter, with Matched false in both cases.
func TestPublishedBodyMatches_NoAnchorIsDistinctFromDrift(t *testing.T) {
	p := ReleaseNotesPrediction{Body: "## Changelog\n### Features\n* aaa1111 feat: add widget\n"}

	// The v0.2.0/v0.3.0 shape: no "## Changelog" line anywhere.
	replaced := p.PublishedBodyMatches("Hand-written release notes. No generated section here at all.\n")
	if replaced.Matched {
		t.Fatalf("replaced: Matched = true, want false")
	}
	if replaced.AnchorFound {
		t.Fatalf("replaced: AnchorFound = true, want false — there is no \"## Changelog\" line in this published body")
	}

	// Real drift: the anchor line IS present, but what follows it disagrees
	// with the prediction.
	drifted := p.PublishedBodyMatches("preamble\n\n## Changelog\n### Features\n* aaa1111 feat: add a WIDGET (edited)\n")
	if drifted.Matched {
		t.Fatalf("drifted: Matched = true, want false")
	}
	if !drifted.AnchorFound {
		t.Fatalf("drifted: AnchorFound = false, want true — the \"## Changelog\" line IS present; only its content differs")
	}
}

// TestPublishedBodyMatches_InlineMentionIsNotTheAnchor proves the anchor is
// line-exact, not a bare substring search: hand-written prose that merely
// MENTIONS "## Changelog" inside a sentence (not as its own line) must not be
// mistaken for the generated section's real start — if it were, everything
// from that inline mention onward, including the REAL generated section
// further down, would be compared as if it were Body, and a legitimate
// prediction would fail this check for a reason that has nothing to do with
// drift.
func TestPublishedBodyMatches_InlineMentionIsNotTheAnchor(t *testing.T) {
	p := ReleaseNotesPrediction{Body: "## Changelog\n### Features\n* aaa1111 feat: add widget\n"}
	published := "See the ## Changelog section below for details.\n\n" + p.Body
	if got := p.PublishedBodyMatches(published); !got.Matched {
		t.Fatalf("PublishedBodyMatches was fooled by an inline, non-line-exact mention of \"## Changelog\" in the hand-written prefix:\npublished:\n%s", published)
	}
}

// ---------------------------------------------------------------------
// 2. the parser, pinned against the real, committed .goreleaser.yaml
// ---------------------------------------------------------------------

func TestLoadReleaseNotesConfig_MatchesCommittedGoreleaserYAML(t *testing.T) {
	cfg, err := LoadReleaseNotesConfig(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("LoadReleaseNotesConfig: %v", err)
	}

	if cfg.Sort != "asc" {
		t.Errorf("changelog.sort = %q, want %q", cfg.Sort, "asc")
	}
	wantGroups := []ReleaseNotesGroup{
		{Title: "Features", Regexp: `^.*?feat(\([[:word:]]+\))??!?:.+$`, Order: 0},
		{Title: "Bug fixes", Regexp: `^.*?fix(\([[:word:]]+\))??!?:.+$`, Order: 1},
		{Title: "Other changes", Regexp: "", Order: 999},
	}
	if len(cfg.Groups) != len(wantGroups) {
		t.Fatalf("changelog.groups: got %d entries, want %d: %+v", len(cfg.Groups), len(wantGroups), cfg.Groups)
	}
	for i, want := range wantGroups {
		if cfg.Groups[i] != want {
			t.Errorf("changelog.groups[%d] = %+v, want %+v", i, cfg.Groups[i], want)
		}
	}

	// The fix this lane makes: without "^Merge " here, a --no-ff merge
	// commit's own subject survives filtering and is swept into "Other
	// changes" on every published release (see
	// TestPredictReleaseNotesForRange_MergeCommitExcluded below, which proves
	// it against a real git merge).
	wantExclude := []string{"^chore:", "^docs:", "^Merge "}
	if len(cfg.Filters.Exclude) != len(wantExclude) {
		t.Fatalf("changelog.filters.exclude: got %v, want %v", cfg.Filters.Exclude, wantExclude)
	}
	for i, want := range wantExclude {
		if cfg.Filters.Exclude[i] != want {
			t.Errorf("changelog.filters.exclude[%d] = %q, want %q", i, cfg.Filters.Exclude[i], want)
		}
	}
}

// TestLoadReleaseNotesConfig_RejectsUnmodeledChangelogKeys is the regression
// test for the MAJOR finding this lane fixes: a bare, non-strict
// yaml.Unmarshal silently drops any changelog.* key ReleaseNotesConfig does
// not name, so setting changelog.abbrev/filters.include/disable/format each
// changes what actually publishes while LoadReleaseNotesConfig kept reporting
// success and the predictor kept computing the OLD behavior. Each case below
// starts from a minimal, valid changelog stanza (mirroring the shape of the
// real .goreleaser.yaml) and adds exactly one unmodeled key; every one must
// now make LoadReleaseNotesConfig fail loudly rather than silently ignore it.
func TestLoadReleaseNotesConfig_RejectsUnmodeledChangelogKeys(t *testing.T) {
	// base has NO "filters:" block of its own — each case below supplies its
	// own complete "filters:" mapping (with "exclude" always present, so the
	// non-filters.include cases still exercise a config shaped like the real
	// one) inside extraLines rather than appended after a second, separate
	// "filters:" key. Two top-level "filters:" mappings in one YAML document
	// is a duplicate key, and yaml.v3's KnownFields(true) rejects duplicate
	// keys outright — an earlier version of this test appended
	// "filters:\n  include: ...\n" AFTER base's own "filters:\n  exclude:
	// ...\n" for exactly the filters.include case, so that case "failed" for
	// an accidental reason (a duplicate mapping key) rather than the reason
	// it claims to pin (filters.include specifically being rejected); a
	// mutation that disabled ONLY the abbrev/disable/format/use checks and
	// left filters.include's check disabled too was NOT caught, because the
	// duplicate-key error fired regardless of whether that check ran.
	base := "changelog:\n" +
		"  sort: asc\n" +
		"  groups:\n" +
		"    - title: Features\n" +
		"      regexp: '^feat:'\n" +
		"      order: 0\n"
	baseFilters := "  filters:\n    exclude:\n      - '^chore:'\n"
	// LoadReleaseNotesConfig now requires a top-level "release:" key too (see
	// goreleaserReleaseFull's doc comment). This has to be appended AFTER
	// each case's extraLines, not folded into base ahead of them: every
	// extraLines value below is 2-space-indented content meant to continue
	// the "changelog:" mapping (a sibling of sort/groups), and a top-level
	// "release:" key sitting between base and extraLines would make YAML
	// read those same 2-space-indented lines as children of "release:"
	// instead, breaking every case for a reason that has nothing to do with
	// what it claims to pin.
	releaseStanza := "release:\n" +
		"  github:\n" +
		"    owner: example\n" +
		"    name: example\n"

	// The base fixture (with the ordinary filters block) on its own must be
	// ACCEPTED — otherwise every case below "failing" would prove nothing
	// about the added key specifically.
	basePath := filepath.Join(t.TempDir(), "goreleaser.yaml")
	if err := os.WriteFile(basePath, []byte(base+baseFilters+releaseStanza), 0o644); err != nil {
		t.Fatalf("write base fixture: %v", err)
	}
	if _, err := LoadReleaseNotesConfig(basePath); err != nil {
		t.Fatalf("base fixture (no unmodeled keys) must load cleanly, got: %v", err)
	}

	cases := []struct {
		name       string
		extraLines string
	}{
		// goreleaser v2.17.1's changelog.go: len(filters.Include) > 0 skips
		// Exclude filtering entirely (an early return), so this predictor
		// applying Exclude anyway is exactly wrong once Include is set. Both
		// "include" and "exclude" live under the SAME "filters:" mapping —
		// see the doc comment above for why this must not be a second,
		// separate "filters:" block.
		{"filters.include", "  filters:\n    exclude:\n      - '^chore:'\n    include:\n      - '^feat'\n"},
		// abbrev shortens every published sha; the predictor always emits
		// the full 40 characters.
		{"abbrev", baseFilters + "  abbrev: 7\n"},
		// disable: true skips the changelog step; goreleaser publishes no
		// changelog section at all, but the predictor still computes one.
		{"disable", baseFilters + "  disable: true\n"},
		// format replaces "* <sha> <subject>" with a caller-supplied
		// template the predictor does not implement.
		{"format", baseFilters + "  format: '{{ .Message }}'\n"},
		// use switches away from the "git" log source this predictor reads.
		{"use", baseFilters + "  use: github\n"},
		// A key goreleaser v2 itself does not even recognize — the
		// KnownFields(true) decode must reject it, not just the five keys
		// this predictor names explicitly above.
		{"totally unknown key", baseFilters + "  mystery: 1\n"},
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), "goreleaser.yaml")
		if err := os.WriteFile(path, []byte(base+c.extraLines+releaseStanza), 0o644); err != nil {
			t.Fatalf("%s: write fixture: %v", c.name, err)
		}
		if _, err := LoadReleaseNotesConfig(path); err == nil {
			t.Errorf("%s: LoadReleaseNotesConfig accepted a config carrying changelog.%s; want an error, since this predictor's algorithm does not implement it and would silently mispredict", c.name, c.name)
		}
	}
}

// TestLoadReleaseNotesConfig_RejectsUnmodeledReleaseKeys is the regression
// test for the MAJOR finding this lane fixes: LoadReleaseNotesConfig's five
// "release:" stanza checks (Header, Footer, Disable, Mode, and the implicit
// KnownFields(true) rejection of any key goreleaserReleaseFull does not name)
// had zero test coverage. A regressed release.header guard in particular is
// dangerous in a way the other four are not: internal/pipe/release/body.go
// renders release.header BEFORE "## Changelog", and PublishedBodyMatches
// deliberately tolerates ANY hand-written prefix ahead of that anchor as an
// "expected hand-written prefix" (see that function's own doc comment) — so a
// templated header that this predictor's Body never accounts for is
// invisible to G3 by construction, not merely unmodeled. If
// LoadReleaseNotesConfig stopped rejecting it, the gate would read the
// published tag and report Matched:true over un-predicted templated prose.
//
// Mirrors TestLoadReleaseNotesConfig_RejectsUnmodeledChangelogKeys's shape: a
// minimal, valid "changelog:" stanza (accepted on its own, checked first so a
// case "failing" below proves something about release: specifically) plus one
// "release:" stanza per case that sets exactly one key this predictor's
// algorithm does not implement, or a key goreleaser v2 itself does not
// recognize at all.
func TestLoadReleaseNotesConfig_RejectsUnmodeledReleaseKeys(t *testing.T) {
	changelogStanza := "changelog:\n" +
		"  sort: asc\n" +
		"  groups:\n" +
		"    - title: Features\n" +
		"      regexp: '^feat:'\n" +
		"      order: 0\n" +
		"  filters:\n" +
		"    exclude:\n" +
		"      - '^chore:'\n"
	baseRelease := "release:\n" +
		"  github:\n" +
		"    owner: example\n" +
		"    name: example\n"

	// The base fixture (an ordinary release: stanza, at goreleaser's own
	// defaults for header/footer/disable/mode) must be ACCEPTED on its own —
	// otherwise every case below "failing" would prove nothing about the
	// added key specifically. This is also, incidentally, the shape of this
	// project's own committed .goreleaser.yaml release: stanza, which
	// TestLoadReleaseNotesConfig_MatchesCommittedGoreleaserYAML already
	// exercises against the real file; this fixture pins the same shape
	// in isolation.
	basePath := filepath.Join(t.TempDir(), "goreleaser.yaml")
	if err := os.WriteFile(basePath, []byte(changelogStanza+baseRelease), 0o644); err != nil {
		t.Fatalf("write base fixture: %v", err)
	}
	if _, err := LoadReleaseNotesConfig(basePath); err != nil {
		t.Fatalf("base fixture (no unmodeled release: keys) must load cleanly, got: %v", err)
	}

	cases := []struct {
		name         string
		releaseLines string
	}{
		// release.header lands BEFORE "## Changelog" in the published body —
		// see this test's own doc comment for why an un-caught regression
		// here is worse than the other four, not merely equivalent to them.
		{"header", baseRelease + "  header: 'Upgrade notes go here.'\n"},
		// release.footer lands AFTER the generated section; this predictor's
		// Body does not account for it either.
		{"footer", baseRelease + "  footer: 'Thanks for reading.'\n"},
		// release.disable: true skips the entire release pipe — no GitHub
		// release is created at all, so predicting a body for it is a wrong
		// answer dressed as a right one.
		{"disable", baseRelease + "  disable: true\n"},
		// release.mode away from the default "keep-existing" means a
		// pre-existing GitHub release at the tag gets merged with the
		// generated notes in a way this predictor's single "here is what
		// publishes" answer does not model.
		{"mode", baseRelease + "  mode: append\n"},
		// A key goreleaser v2 itself does not even recognize under this
		// predictor's narrowed goreleaserReleaseFull schema — the
		// KnownFields(true) decode must reject it, not just the four keys
		// named explicitly above.
		{"totally unknown key", baseRelease + "  mystery: 1\n"},
	}
	for _, c := range cases {
		path := filepath.Join(t.TempDir(), "goreleaser.yaml")
		if err := os.WriteFile(path, []byte(changelogStanza+c.releaseLines), 0o644); err != nil {
			t.Fatalf("%s: write fixture: %v", c.name, err)
		}
		if _, err := LoadReleaseNotesConfig(path); err == nil {
			t.Errorf("%s: LoadReleaseNotesConfig accepted a config carrying release.%s; want an error, since this predictor's algorithm does not implement it and would silently mispredict", c.name, c.name)
		}
	}
}

// ---------------------------------------------------------------------
// 3. the whole pipeline, git invocation included, over a hermetic repo
// ---------------------------------------------------------------------

// predictNotesGit runs one git command in dir with an isolated configuration
// (no developer/CI global config, fixed dates so nothing here is
// time-dependent) — the same isolation cmd/dossierx/check_staged_cli_test.go's
// stagedGit uses, reimplemented here because that helper lives in package
// main and is not importable from package tests.
func predictNotesGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE=2026-08-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-08-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// hermeticReleaseHistory is a from-scratch git repository this test builds
// and commits into itself, standing in for "this repository's real history"
// the way TestPredictReleaseNotesForRange_MergeCommitExcluded used to depend
// on directly. See that test's doc comment for why: a real, immutable git
// range (v0.4.1..v0.5.0) is only as resolvable as the ambient checkout that
// has to resolve it, and which tags that checkout holds is decided in another
// file by a concern that is not this one. Building the history here instead
// means every assertion below is checked against a REAL git invocation (the
// whole point — no rawLines fixture stands in for git) while depending on
// nothing about the checkout this test happens to run inside.
type hermeticReleaseHistory struct {
	root       string
	cfg        ReleaseNotesConfig
	base       string // the commit BEFORE any of the release's commits
	branchTip  string // last commit on the feature branch, before the merge — G1's view
	mergeSHA   string // the --no-ff merge commit itself — G2's view adds exactly this
	mergeShort string // "Merge pull request #99" — the subject prefix assertions match on
}

// buildHermeticReleaseHistory git-inits root, commits a base commit on main,
// branches, commits a feat/fix/docs trio on the branch, and merges it back
// with --no-ff — the same shape a DossierX release PR merge produces
// (confirmed against this repo's own eab3a63 merge commit via `git log
// --merges`, an ordinary GitHub "Merge pull request" subject).
//
// It uses the project's OWN .goreleaser.yaml (via LoadReleaseNotesConfig),
// not a synthetic config, so this is still checking the REAL committed
// exclude/group rules — only the commit history under test is hermetic, not
// the rules being applied to it.
func buildHermeticReleaseHistory(t *testing.T) hermeticReleaseHistory {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	cfg, err := LoadReleaseNotesConfig(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("LoadReleaseNotesConfig: %v", err)
	}

	root := t.TempDir()
	predictNotesGit(t, root, "init", "-q", "-b", "main")
	predictNotesGit(t, root, "config", "user.email", "fixture@example.invalid")
	predictNotesGit(t, root, "config", "user.name", "fixture")

	writeAndCommit := func(name, contents, subject string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		predictNotesGit(t, root, "add", "-A")
		predictNotesGit(t, root, "commit", "-q", "-m", subject)
	}

	writeAndCommit("README.md", "hermetic fixture\n", "chore: repo init")
	base := predictNotesGit(t, root, "rev-parse", "HEAD")

	predictNotesGit(t, root, "checkout", "-q", "-b", "feature")
	writeAndCommit("widget.go", "package widget\n", "feat(widget): add widget export")
	writeAndCommit("render.go", "package render\n", "fix(render): correct overflow")
	writeAndCommit("README.md", "hermetic fixture, documented\n", "docs: update README")
	branchTip := predictNotesGit(t, root, "rev-parse", "HEAD")

	predictNotesGit(t, root, "checkout", "-q", "main")
	mergeSubject := "Merge pull request #99 — hermetic release fixture"
	predictNotesGit(t, root, "merge", "--no-ff", "-q", "-m", mergeSubject, "feature")
	mergeSHA := predictNotesGit(t, root, "rev-parse", "HEAD")

	return hermeticReleaseHistory{
		root: root, cfg: cfg,
		base: base, branchTip: branchTip, mergeSHA: mergeSHA,
		mergeShort: "Merge pull request #99",
	}
}

// TestPredictReleaseNotesForRange_MergeCommitExcluded confirms a --no-ff
// merge commit's own subject is excluded (matched by "^Merge "), not
// silently swept into "Other changes" the way it would be without the fix —
// proving the config change actually closes the gap it claims to.
func TestPredictReleaseNotesForRange_MergeCommitExcluded(t *testing.T) {
	h := buildHermeticReleaseHistory(t)

	got, err := PredictReleaseNotesForRange(h.root, h.base+".."+h.mergeSHA, h.cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange: %v", err)
	}

	var mergeDropped *DroppedCommit
	for i := range got.Dropped {
		if strings.HasPrefix(got.Dropped[i].Subject, h.mergeShort) {
			mergeDropped = &got.Dropped[i]
			break
		}
	}
	if mergeDropped == nil {
		t.Fatalf("expected the merge commit's subject to be in Dropped (excluded_by %q); it was not found in: %+v", "^Merge ", got.Dropped)
	}
	if mergeDropped.ExcludedBy != "^Merge " {
		t.Errorf("merge commit excluded_by = %q, want %q", mergeDropped.ExcludedBy, "^Merge ")
	}
	// The full 40-character sha, not GoReleaser v1's 7-character
	// --abbrev-commit form — see release_notes_predict_lib_test.go's
	// "GROUND TRUTH IS GORELEASER v2" note. This assertion is NOT what pins
	// gitLogOneline's %H-vs-%h format choice, though an earlier version of
	// this comment claimed it was: under this test's own ambient git config
	// (no log.abbrevCommit set), git's own "auto" core.abbrev default already
	// yields the full 40 characters for a repo this small regardless of
	// whether gitLogOneline requests %H or %h/--pretty=oneline — verified by
	// mutating gitLogOneline to add "--abbrev-commit" (with the %H format
	// otherwise unchanged, since --abbrev-commit has no effect on %H at all)
	// and separately to "--pretty=oneline" outright: neither mutation fails
	// this test. TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig,
	// which explicitly sets log.abbrevCommit=true, is what actually pins the
	// %H choice — that same "--pretty=oneline" mutation fails four assertions
	// there. This assertion still earns its place: it pins that the sha
	// survives at all (non-empty, well-formed) through the exclude/merge
	// path, just not the abbreviation format specifically.
	if n := len(mergeDropped.SHA); n != 40 {
		t.Errorf("merge commit sha = %q (%d chars), want the full 40-character form", mergeDropped.SHA, n)
	}

	// The negative half of the same claim: it must not ALSO have landed in a
	// published group. A commit can be dropped or grouped, never both, but a
	// bug in the pool-narrowing logic could put it in both lists.
	for _, g := range got.Groups {
		for _, c := range g.Commits {
			if strings.HasPrefix(c.Subject, h.mergeShort) {
				t.Errorf("the merge commit also appears in published group %q; it must be dropped, not published", g.Title)
			}
			if n := len(c.SHA); n != 40 {
				t.Errorf("group %q commit %q: sha = %q (%d chars), want 40", g.Title, c.Subject, c.SHA, n)
			}
		}
	}
	if strings.Contains(got.Body, h.mergeShort) {
		t.Errorf("the merge commit's subject leaked into the predicted body:\n%s", got.Body)
	}
	// feat(widget) and fix(render) must both have published, in full-sha
	// form, proving the body isn't just empty.
	if !strings.Contains(got.Body, "feat(widget): add widget export") {
		t.Errorf("expected the feat commit in the predicted body:\n%s", got.Body)
	}
	if !strings.Contains(got.Body, "fix(render): correct overflow") {
		t.Errorf("expected the fix commit in the predicted body:\n%s", got.Body)
	}

	// And the config a client shipped before this lane's fix DID lack the
	// exclude — a straight sanity check that the scenario this test guards
	// against is real, not hypothetical.
	oldCfg := h.cfg
	oldCfg.Filters.Exclude = []string{"^chore:", "^docs:"}
	without, err := PredictReleaseNotesForRange(h.root, h.base+".."+h.mergeSHA, oldCfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange (pre-fix config): %v", err)
	}
	if !strings.Contains(without.Body, h.mergeShort) {
		t.Fatalf("expected the pre-fix config to leak the merge subject into the body (proving the fix is necessary), but it did not:\n%s", without.Body)
	}
}

// TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig is the regression
// test for the BLOCKING finding this lane fixes: gitLogOneline used to run
// `git log --pretty=oneline ...` with no `-c` guard, and --pretty=oneline's
// sha field is `%h`-shaped — its length tracks the SAME ambient
// log.abbrevCommit / core.abbrev config `%h` does, and only happened to print
// the full 40 characters because this repository's own git config never set
// it. A developer, CI runner or system-wide gitconfig that DOES set it would
// make G1 and G2 both compute the SAME wrong (abbreviated) shas — identical
// to each other, so PublishedEqual passes, and different from what
// GoReleaser's own %H-based invocation actually publishes.
//
// This sets log.abbrevCommit=true as a REPO-LOCAL config on the hermetic
// fixture (standing in for "a config gitLogOneline does not control",
// exactly as an ambient developer or CI config would be) and confirms the
// predicted shas are still the full 40 characters — proving gitLogOneline's
// own invocation (git.go's %H, not %h/oneline) is what determines the
// output, not whatever config happens to be active in the calling
// environment, the way GoReleaser's own -c-guarded invocation is immune too.
func TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig(t *testing.T) {
	h := buildHermeticReleaseHistory(t)
	predictNotesGit(t, h.root, "config", "log.abbrevCommit", "true")

	got, err := PredictReleaseNotesForRange(h.root, h.base+".."+h.mergeSHA, h.cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange: %v", err)
	}
	if len(got.Groups) == 0 {
		t.Fatalf("test setup: expected at least one published group, got none: %+v", got)
	}
	checked := 0
	for _, g := range got.Groups {
		for _, c := range g.Commits {
			checked++
			if n := len(c.SHA); n != 40 {
				t.Errorf("group %q commit %q: sha = %q (%d chars) under ambient log.abbrevCommit=true, want the full 40-character form", g.Title, c.Subject, c.SHA, n)
			}
		}
	}
	for _, d := range got.Dropped {
		checked++
		if n := len(d.SHA); n != 40 {
			t.Errorf("dropped commit %q: sha = %q (%d chars) under ambient log.abbrevCommit=true, want the full 40-character form", d.Subject, d.SHA, n)
		}
	}
	if checked == 0 {
		t.Fatalf("test setup: no commits were checked at all; the fixture produced neither Groups nor Dropped")
	}
}

// TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig_SignedCommits is
// what pins gitLogOneline's "-c log.showSignature=false" guard — see the
// package doc's item 1 and gitLogOneline's own doc comment for the full
// account of why it is load-bearing. Confirmed by hand ahead of writing this
// test (`git -c log.showSignature=true log --pretty=format:'%H %s' <range>`
// against a real ssh-signed commit) that the hazard is real: git prints
// `Good "git" signature for ...` on STDOUT, one line, immediately before the
// commit's own "<sha> <subject>" line — with no `-c` guard, splitCommitLine
// reads that banner line as its own malformed, sha-less entry.
//
// This builds a hermetic repo with a REAL ssh-signed commit (git's
// gpg.format=ssh, a throwaway ed25519 key generated into the fixture's own
// temp dir, an allowedSignersFile so `git log --show-signature`'s
// verification actually succeeds rather than erroring) and sets
// log.showSignature=true as REPO-LOCAL config on the fixture — standing in
// for an ambient developer/CI/system gitconfig gitLogOneline does not
// control, exactly as TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig
// does for log.abbrevCommit. It then confirms the predicted commit is
// exactly one Features entry with the correct 40-character sha and an
// unmangled subject — proving the guard, not just the %H format fix, is what
// keeps a signed commit's history readable.
func TestPredictReleaseNotesForRange_ImmuneToAmbientGitConfig_SignedCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not on PATH")
	}

	cfg, err := LoadReleaseNotesConfig(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("LoadReleaseNotesConfig: %v", err)
	}

	root := t.TempDir()
	predictNotesGit(t, root, "init", "-q", "-b", "main")
	predictNotesGit(t, root, "config", "user.email", "fixture@example.invalid")
	predictNotesGit(t, root, "config", "user.name", "fixture")

	// base commit, unsigned — the signing config below is set up afterward so
	// only the commit under test needs to carry a real signature.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hermetic fixture\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	predictNotesGit(t, root, "add", "-A")
	predictNotesGit(t, root, "commit", "-q", "-m", "docs: repo init")
	base := predictNotesGit(t, root, "rev-parse", "HEAD")

	// Generate a throwaway ssh signing key INTO the fixture's own temp dir
	// (never touches the real developer/CI signing key) and wire the repo to
	// sign with it, verify against it, and — the ambient config under test —
	// print the signature banner on every `git log` that doesn't override it.
	keyPath := filepath.Join(root, "signing_key")
	genCmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath, "-C", "fixture")
	if out, err := genCmd.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}
	pub, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		t.Fatalf("read %s.pub: %v", keyPath, err)
	}
	allowedSigners := filepath.Join(root, "allowed_signers")
	if err := os.WriteFile(allowedSigners, []byte("fixture@example.invalid "+string(pub)), 0o644); err != nil {
		t.Fatalf("write %s: %v", allowedSigners, err)
	}
	predictNotesGit(t, root, "config", "gpg.format", "ssh")
	predictNotesGit(t, root, "config", "user.signingkey", keyPath+".pub")
	predictNotesGit(t, root, "config", "commit.gpgsign", "true")
	predictNotesGit(t, root, "config", "gpg.ssh.allowedSignersFile", allowedSigners)
	predictNotesGit(t, root, "config", "log.showSignature", "true")

	if err := os.WriteFile(filepath.Join(root, "widget.go"), []byte("package widget\n"), 0o644); err != nil {
		t.Fatalf("write widget.go: %v", err)
	}
	predictNotesGit(t, root, "add", "-A")
	predictNotesGit(t, root, "commit", "-q", "-S", "-m", "feat(widget): add signed widget export")
	tip := predictNotesGit(t, root, "rev-parse", "HEAD")

	// Sanity check on the fixture itself: without the guard, this range's raw
	// git output really does carry the signature banner ahead of the commit
	// line — otherwise a passing assertion below would prove nothing about
	// the guard specifically.
	unguarded := predictNotesGit(t, root, "log", "--no-decorate", "--no-color", "--pretty=format:%H %s", base+".."+tip)
	if !strings.Contains(unguarded, "signature") {
		t.Fatalf("test setup: expected an unguarded `git log` over this range to print a signature banner (the hazard this test exists to catch), got:\n%s", unguarded)
	}

	got, err := PredictReleaseNotesForRange(root, base+".."+tip, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange: %v", err)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("groups: got %d, want exactly 1 (Features) — a signature banner read as its own entry would land here as an extra, malformed group or commit: %+v", len(got.Groups), got.Groups)
	}
	g := got.Groups[0]
	if g.Title != "Features" {
		t.Fatalf("group[0].Title = %q, want %q", g.Title, "Features")
	}
	if len(g.Commits) != 1 {
		t.Fatalf("Features commits: got %d, want exactly 1: %+v", len(g.Commits), g.Commits)
	}
	c := g.Commits[0]
	if c.Subject != "feat(widget): add signed widget export" {
		t.Errorf("commit subject = %q, want %q — a signature banner corrupts this into something like Subject=%q", c.Subject, "feat(widget): add signed widget export", "signature")
	}
	if n := len(c.SHA); n != 40 {
		t.Errorf("commit sha = %q (%d chars), want the full 40-character form", c.SHA, n)
	}
	if len(got.Dropped) != 0 {
		t.Errorf("dropped: got %+v, want none — nothing here should have matched an exclude filter, and a corrupted banner-derived entry must not silently land here either", got.Dropped)
	}
}

// TestPredictReleaseNotesForRange_PublishedEqualAcrossMergeBoundary is G2's
// check, exercised directly: G1 predicts over the branch range (the merge
// commit does not exist yet), G2 re-predicts over the merge-commit range
// (the merge commit exists, and is filtered into Dropped) — the two
// predictions must NOT be reflect.DeepEqual (Dropped differs, structurally,
// by exactly the merge commit) but MUST be PublishedEqual (the published
// Body and Groups are identical either way).
func TestPredictReleaseNotesForRange_PublishedEqualAcrossMergeBoundary(t *testing.T) {
	h := buildHermeticReleaseHistory(t)

	g1, err := PredictReleaseNotesForRange(h.root, h.base+".."+h.branchTip, h.cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange (G1, branch range): %v", err)
	}
	g2, err := PredictReleaseNotesForRange(h.root, h.base+".."+h.mergeSHA, h.cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange (G2, merge-commit range): %v", err)
	}

	if reflect.DeepEqual(g1, g2) {
		t.Fatalf("g1 and g2 are reflect.DeepEqual; expected Dropped to differ by the merge commit (g1.Dropped=%+v g2.Dropped=%+v) — if this now passes, either the merge commit stopped landing in the range or Dropped stopped recording it, and the scenario PublishedEqual exists for is no longer exercised", g1.Dropped, g2.Dropped)
	}
	if len(g2.Dropped) != len(g1.Dropped)+1 {
		t.Fatalf("g2.Dropped should hold exactly one more entry than g1.Dropped (the merge commit): g1=%d g2=%d", len(g1.Dropped), len(g2.Dropped))
	}
	if !g1.PublishedEqual(g2) {
		t.Fatalf("g1.PublishedEqual(g2) = false, want true (same Body/Groups on both sides of the merge boundary):\ng1.Body:\n%s\ng2.Body:\n%s", g1.Body, g2.Body)
	}
	if !g2.PublishedEqual(g1) {
		t.Fatalf("PublishedEqual is not symmetric: g2.PublishedEqual(g1) = false but g1.PublishedEqual(g2) = true")
	}

	// Negative control: PublishedEqual must not be a stub that always
	// returns true. A prediction over a range that is missing the fix
	// commit entirely has a different Body and must compare unequal.
	missingFix, err := PredictReleaseNotesForRange(h.root, h.base+".."+h.mergeSHA, h.cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange: %v", err)
	}
	missingFix.Body = strings.Replace(missingFix.Body, "fix(render): correct overflow", "fix(render): a different subject entirely", 1)
	if g1.PublishedEqual(missingFix) {
		t.Fatalf("PublishedEqual(missingFix) = true, want false — Body differs and must be caught")
	}
}

// TestPublishedEqual_CatchesNewlyDroppedCommit is the regression test for the
// finding this lane fixes: PublishedEqual used to exclude Dropped ENTIRELY,
// not just the one merge-commit entry that has to be exempted, so a
// "docs:"/"chore:"-prefixed commit landing between G1 and G2 — an ordinary
// user-visible change whose subject happens to start with a dropped prefix —
// vanished from the published release notes with PublishedEqual reporting no
// disagreement at all. Body is unaffected (a dropped commit never appears in
// Body by definition), which is exactly why Dropped has to be part of this
// check.
func TestPublishedEqual_CatchesNewlyDroppedCommit(t *testing.T) {
	cfg := fixtureReleaseNotesConfig()

	g1Lines := []string{
		"aaa1111 feat(cli): add widget export",
		"bbb2222 fix(render): correct table overflow",
	}
	g1, err := PredictReleaseNotes(g1Lines, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotes(g1): %v", err)
	}
	if len(g1.Dropped) != 0 {
		t.Fatalf("test setup: expected g1 to have dropped nothing, got %+v", g1.Dropped)
	}

	// g2 sees everything g1 saw, PLUS a docs: commit (a real, user-visible
	// change under a dropped prefix) and the merge commit that always lands
	// between a branch-range and a merge-commit-range prediction.
	g2Lines := append(append([]string{}, g1Lines...),
		"ccc3333 docs: the --strict flag now rejects empty claims",
		"eee5555 Merge pull request #40 — a later release",
	)
	g2, err := PredictReleaseNotes(g2Lines, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotes(g2): %v", err)
	}

	if g1.Body != g2.Body {
		t.Fatalf("test setup: expected identical published bodies (a dropped commit must not itself change Body), g1.Body=%q g2.Body=%q", g1.Body, g2.Body)
	}
	if g1.PublishedEqual(g2) {
		t.Fatalf("g1.PublishedEqual(g2) = true, want false — g2 dropped a docs: commit g1 never saw; a user-visible change silently missing from the release page must be caught, not waved through because Body alone did not move")
	}
}

// TestPublishedEqual_ComparesGroups pins PublishedEqual's Groups comparison —
// the `len(p.Groups) != len(other.Groups)` guard and the Title/Order/Commits
// loop — which until this test was ENTIRELY unpinned: deleting all of it left
// the whole package green, because every other PublishedEqual case in this file
// varies Body alongside Groups, and Body alone then carries the verdict.
//
// WHY THE PREDICTIONS ARE HAND-BUILT AND NOT PREDICTED. Body is DERIVED from
// Groups by formatChangelog ("### "+Title per group, "* "+SHA+" "+Subject per
// commit), so no input to PredictReleaseNotes can move Groups while holding
// Body fixed — running the real pipeline can only ever produce cases where
// Body already decides the answer, which is exactly the redundancy that hid
// this gap. Each case below therefore holds Body byte-identical by
// construction and varies ONE field of Groups, so a `false` verdict can have
// come from nothing but the branch it names.
//
// WHY THIS IS WORTH PINNING RATHER THAN DELETING. The redundancy is real but
// partial, and it is not a property of PublishedEqual — it is a property of
// today's config. `Order` is already a field Body cannot carry at all (it
// decides section sequence, and two group lists with swapped orders render the
// same "### " headings), and it is only safe today because G1 and G2 read
// order from the same .goreleaser.yaml, so they cannot normally disagree on
// it — that stops holding the moment .goreleaser.yaml's group orders are
// edited between the two stages. The rest of the redundancy stops holding the
// moment the predictor models a config key that decouples Body from Groups:
// changelog.abbrev shortens the sha in the rendered line, changelog.format
// replaces the line shape outright, and both are currently REJECTED by
// LoadReleaseNotesConfig rather than implemented — the day either is
// implemented, this comparison is the only thing left checking Groups, and it
// should not be discovering then that nothing ever tested it.
func TestPublishedEqual_ComparesGroups(t *testing.T) {
	// sharedBody is what every case on both sides carries, unchanged. Its
	// content is irrelevant to what is being pinned — only that it is IDENTICAL
	// across each pair, so Body's own `p.Body != other.Body` guard returns on
	// none of them and the Groups branch is the only thing that can speak.
	const sharedBody = "## Changelog\n### Features\n* aaa1111 feat(cli): add widget export\n### Bug fixes\n* bbb2222 fix(render): correct table overflow\n"

	features := PredictedGroup{
		Title:   "Features",
		Order:   0,
		Commits: []ReleaseNotesCommit{{SHA: "aaa1111", Subject: "feat(cli): add widget export"}},
	}
	bugfixes := PredictedGroup{
		Title:   "Bug fixes",
		Order:   1,
		Commits: []ReleaseNotesCommit{{SHA: "bbb2222", Subject: "fix(render): correct table overflow"}},
	}
	base := ReleaseNotesPrediction{Body: sharedBody, Groups: []PredictedGroup{features, bugfixes}}

	// withGroups clones base and swaps in a different Groups slice, leaving
	// Body and Dropped untouched.
	withGroups := func(groups ...PredictedGroup) ReleaseNotesPrediction {
		return ReleaseNotesPrediction{Body: sharedBody, Groups: groups}
	}

	cases := []struct {
		name  string
		other ReleaseNotesPrediction
		want  bool
		why   string
	}{
		{
			name:  "identical groups compare equal (positive control)",
			other: withGroups(features, bugfixes),
			want:  true,
			why:   "without this, a Groups comparison mutated to return false unconditionally would satisfy every negative case below while failing G2 on every real release",
		},
		{
			name:  "a missing group is caught (len guard)",
			other: withGroups(features),
			want:  false,
			why:   "one side records a section the other does not; the `len(p.Groups) != len(other.Groups)` guard is the only thing that sees it",
		},
		{
			name:  "an extra group is caught (len guard, other direction)",
			other: withGroups(features, bugfixes, PredictedGroup{Title: "Other changes", Order: 999}),
			want:  false,
			why:   "the len guard has to fire in both directions, not just when the receiver is longer",
		},
		{
			name:  "a renamed group title is caught",
			other: withGroups(features, PredictedGroup{Title: "Fixes", Order: bugfixes.Order, Commits: bugfixes.Commits}),
			want:  false,
			why:   "a.Title != b.Title is the branch; a section heading that changed between G1 and G2 is a different published document",
		},
		{
			name:  "a reordered group is caught (Order, the field Body cannot carry)",
			other: withGroups(features, PredictedGroup{Title: bugfixes.Title, Order: 7, Commits: bugfixes.Commits}),
			want:  false,
			why:   "Order decides which section publishes first and is invisible in Body; a.Order != b.Order is the ONLY assertion in the tree that can see .goreleaser.yaml's group orders being edited between G1 and G2",
		},
		{
			name: "a group that gained a commit is caught (commit-count branch)",
			other: withGroups(features, PredictedGroup{
				Title: bugfixes.Title, Order: bugfixes.Order,
				Commits: append(append([]ReleaseNotesCommit{}, bugfixes.Commits...),
					ReleaseNotesCommit{SHA: "ddd4444", Subject: "fix(lock): reject a stale ledger"}),
			}),
			want: false,
			why:  "len(a.Commits) != len(b.Commits) is the branch",
		},
		{
			name: "a commit whose subject was rewritten is caught (commit-element branch)",
			other: withGroups(features, PredictedGroup{
				Title: bugfixes.Title, Order: bugfixes.Order,
				Commits: []ReleaseNotesCommit{{SHA: "bbb2222", Subject: "fix(render): correct table OVERFLOW"}},
			}),
			want: false,
			why:  "a.Commits[j] != b.Commits[j] is the branch; same count, same position, different content",
		},
		{
			name: "a commit whose sha changed is caught (same subject, rebased)",
			other: withGroups(features, PredictedGroup{
				Title: bugfixes.Title, Order: bugfixes.Order,
				Commits: []ReleaseNotesCommit{{SHA: "999zzzz", Subject: bugfixes.Commits[0].Subject}},
			}),
			want: false,
			why:  "the commit struct compares BOTH fields; a subject-only comparison would let a rebased/amended sha through",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Guard the premise itself: if a case ever stopped holding Body
			// identical, a `false` verdict would prove nothing about Groups.
			if base.Body != tc.other.Body {
				t.Fatalf("test setup: this case must hold Body identical so ONLY the Groups branch can decide, got base.Body=%q other.Body=%q", base.Body, tc.other.Body)
			}
			if got := base.PublishedEqual(tc.other); got != tc.want {
				t.Fatalf("base.PublishedEqual(other) = %v, want %v — %s\nbase.Groups:  %+v\nother.Groups: %+v", got, tc.want, tc.why, base.Groups, tc.other.Groups)
			}
			// Symmetry: PublishedEqual is used in both directions across the
			// codebase (CompareReleaseNotesPrediction calls
			// recorded.PublishedEqual(fresh); the merge-boundary test asserts
			// both orders), so a branch that only fires when the receiver is the
			// longer/differing side is a real hole.
			if got := tc.other.PublishedEqual(base); got != tc.want {
				t.Fatalf("other.PublishedEqual(base) = %v, want %v (asymmetric with base.PublishedEqual(other)) — %s", got, tc.want, tc.why)
			}
		})
	}
}

// TestCompareReleaseNotesPrediction covers CompareReleaseNotesPrediction —
// the function that makes PublishedEqual actually reachable from a gate
// stage (see its doc comment: PublishedEqual on its own is only callable
// from within this package's own tests).
func TestCompareReleaseNotesPrediction(t *testing.T) {
	cfg := fixtureReleaseNotesConfig()
	g1Lines := []string{
		"aaa1111 feat(cli): add widget export",
		"bbb2222 fix(render): correct table overflow",
	}
	g1, err := PredictReleaseNotes(g1Lines, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotes(g1): %v", err)
	}

	recordedPath := filepath.Join(t.TempDir(), "g1-prediction.json")
	data, err := json.MarshalIndent(g1, "", "  ")
	if err != nil {
		t.Fatalf("marshal g1: %v", err)
	}
	if err := os.WriteFile(recordedPath, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", recordedPath, err)
	}

	// G2's ordinary case: nothing changed except the range is re-predicted.
	// Must pass.
	g2Same, err := PredictReleaseNotes(g1Lines, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotes(g2Same): %v", err)
	}
	if err := CompareReleaseNotesPrediction(g2Same, recordedPath); err != nil {
		t.Errorf("expected an identical re-prediction to compare equal, got: %v", err)
	}

	// A docs: commit landed in between: must fail loudly, not silently pass —
	// this is the same scenario TestPublishedEqual_CatchesNewlyDroppedCommit
	// exercises directly against PublishedEqual, checked here through the
	// entry point a gate stage actually calls.
	g2Changed, err := PredictReleaseNotes(append(append([]string{}, g1Lines...), "ccc3333 docs: the --strict flag now rejects empty claims"), cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotes(g2Changed): %v", err)
	}
	if err := CompareReleaseNotesPrediction(g2Changed, recordedPath); err == nil {
		t.Fatalf("expected CompareReleaseNotesPrediction to fail when a docs: commit landed between G1 and G2, got nil error")
	}

	// A missing recorded file — G1 never ran, or wrote somewhere else — must
	// fail loudly too. CLAUDE.md: "a skip is a failure, not a pass"; an
	// absent recording is not evidence the release notes are fine.
	if err := CompareReleaseNotesPrediction(g2Same, filepath.Join(t.TempDir(), "does-not-exist.json")); err == nil {
		t.Fatalf("expected CompareReleaseNotesPrediction to fail on a missing recorded file, got nil error")
	}

	// An unparsable recorded file must fail loudly for the same reason.
	badPath := filepath.Join(t.TempDir(), "not-json.json")
	if err := os.WriteFile(badPath, []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("write %s: %v", badPath, err)
	}
	if err := CompareReleaseNotesPrediction(g2Same, badPath); err == nil {
		t.Fatalf("expected CompareReleaseNotesPrediction to fail on an unparsable recorded file, got nil error")
	}
}

// ---------------------------------------------------------------------
// 4. the reusable G1/G2/G3 capture entry points
// ---------------------------------------------------------------------

// TestPredictReleaseNotesForRange_G1Capture is how a gate stage actually uses
// this predictor: invoked as
//
//	go test ./tests -run TestPredictReleaseNotesForRange_G1Capture \
//	  -args -release-notes-range=v0.5.0..HEAD -release-notes-predict-out=/path/to/prediction.json
//
// it writes a fresh ReleaseNotesPrediction for -release-notes-range to
// -release-notes-predict-out. G1 runs it once over the branch range; G2 runs
// the identical command with -release-notes-range set to the merge-commit
// range AND -release-notes-predict-compare set to G1's recorded JSON path —
// the test then FAILS (not "logs a mismatch") if CompareReleaseNotesPrediction
// finds the two are not PublishedEqual (NOT reflect.DeepEqual — see
// PublishedEqual's doc comment in release_notes_predict_lib_test.go for why
// raw equality is the wrong check here). G3 fetches the real published body
// and checks it with the last prediction's PublishedBodyMatches (NOT Body ==
// publishedBody — see ReleaseNotesPrediction's doc comment in
// release_notes_predict_lib_test.go for why a breaking release's hand-written
// prefix makes that the wrong check too). With none of the three
// flags set (the default `go test` invocation, and every CI run of this
// suite) this test does nothing beyond confirming the flags parse — the
// predictor's correctness is TestPredictReleaseNotesForRange_MergeCommitExcluded,
// TestPredictReleaseNotesForRange_PublishedEqualAcrossMergeBoundary,
// TestPublishedEqual_CatchesNewlyDroppedCommit and
// TestPredictReleaseNotes_FixedFixture's job, not this test's.
func TestPredictReleaseNotesForRange_G1Capture(t *testing.T) {
	// VALUE (*releaseNotesRange == "") can never tell "this flag was never
	// passed" apart from "this flag was passed with an empty value" (e.g. a
	// driver expanding an unset shell variable, `-release-notes-range=$RANGE`
	// with RANGE unset) — both produce the identical Go string "". PRESENCE
	// can: flag.CommandLine.Visit reports only flags actually set on the
	// command line, regardless of what they were set to. Everything below
	// keys off presence, not value, so a flag that was given but empty fails
	// loudly instead of being silently indistinguishable from "not given at
	// all". CLAUDE.md: "a skip is a failure, not a pass" / "indistinguishable
	// from a pass over zero assertions" (viewer-tests/harness_test.go:47).
	var rangeGiven, compareGiven, outGiven bool
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "release-notes-range":
			rangeGiven = true
		case "release-notes-predict-compare":
			compareGiven = true
		case "release-notes-predict-out":
			outGiven = true
		}
	})

	if !rangeGiven {
		// -release-notes-predict-compare/-release-notes-predict-out each
		// IMPLY -release-notes-range: a caller that set either one but left
		// range unset meant to run G1 or G2's actual check, not to no-op.
		// Only skip when NEITHER flag was given at all — the ordinary
		// `go test ./tests` invocation and every CI run of this suite.
		if compareGiven || outGiven {
			t.Fatalf("-release-notes-predict-compare (given=%v) or -release-notes-predict-out (given=%v) was given without -release-notes-range; both imply -release-notes-range and must not silently skip", compareGiven, outGiven)
		}
		t.Skip("no -release-notes-range given; this test is a capture entry point, not a correctness check (see TestPredictReleaseNotesForRange_MergeCommitExcluded for that)")
	}
	if *releaseNotesRange == "" {
		// -release-notes-range WAS passed on the command line but its value
		// is empty — the same unset-shell-variable hazard, on the very flag
		// that implies itself. An empty range is never a legitimate no-op:
		// treating it as one would mean the branch never intended to run G1
		// at all reads as a pass.
		t.Fatalf("-release-notes-range was given but is empty (e.g. a driver expanding an unset shell variable); a skip is a failure, not a pass")
	}

	root := repoRoot(t)
	cfg, err := LoadReleaseNotesConfig(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("LoadReleaseNotesConfig: %v", err)
	}
	prediction, err := PredictReleaseNotesForRange(root, *releaseNotesRange, cfg)
	if err != nil {
		t.Fatalf("PredictReleaseNotesForRange(%q): %v", *releaseNotesRange, err)
	}

	if compareGiven {
		if *releaseNotesPredictCompare == "" {
			// THE gap this whole rewrite closes: -release-notes-range was
			// given a real value (so the block above never fires) while
			// -release-notes-predict-compare was given an EMPTY one. Under
			// the old `!= ""` check this branch was simply never entered —
			// CompareReleaseNotesPrediction, the one call that is G2's
			// entire reason to exist, never ran, `go test` still exited 0,
			// and a driver reading that exit code would believe G2 had
			// confirmed G1's prediction when no comparison had happened at
			// all.
			t.Fatalf("-release-notes-predict-compare was given but is empty (e.g. a driver expanding an unset shell variable); G2's equality check against G1's recorded prediction must not silently not run")
		}
		if err := CompareReleaseNotesPrediction(prediction, *releaseNotesPredictCompare); err != nil {
			t.Fatal(err)
		}
		t.Logf("prediction for %s matches the recorded prediction at %s", *releaseNotesRange, *releaseNotesPredictCompare)
	}

	if !outGiven {
		t.Logf("prediction for %s (no -release-notes-predict-out given, not written):\n%s", *releaseNotesRange, prediction.Body)
		return
	}
	if *releaseNotesPredictOut == "" {
		t.Fatalf("-release-notes-predict-out was given but is empty (e.g. a driver expanding an unset shell variable); the prediction a later gate stage needs would silently not be written, while `go test` still exits 0")
	}
	data, err := json.MarshalIndent(prediction, "", "  ")
	if err != nil {
		t.Fatalf("marshal prediction: %v", err)
	}
	if err := os.WriteFile(*releaseNotesPredictOut, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", *releaseNotesPredictOut, err)
	}
	t.Logf("wrote prediction for %s to %s", *releaseNotesRange, *releaseNotesPredictOut)
}

// TestPredictReleaseNotesForRange_G1Capture_RequiresRangeWhenCompareOrOutGiven
// is the regression test for the first half of the BLOCKING finding this
// lane fixes: TestPredictReleaseNotesForRange_G1Capture used to check ONLY
// `*releaseNotesRange == ""` and skip (exit 0, "ok") without ever looking at
// -release-notes-predict-compare or -release-notes-predict-out — so a driver
// invoking `-release-notes-range=$RANGE` with RANGE unset (expanding to an
// empty string) silently skipped G2's equality check, the one this predictor
// exists to run, while `go test` still reported success. CLAUDE.md: "a skip
// is a failure, not a pass".
//
// TestPredictReleaseNotesForRange_G1Capture_RequiresNonEmptyValueWhenFlagGiven,
// immediately below, covers the SECOND half: the fix above only distinguished
// "flag absent" from "flag present" by checking the flag's own emptiness,
// which is exactly the ambiguity a `-flag=$VAR` with VAR unset produces —
// present-but-empty and absent are the same Go string. The worst case that
// gap left open was G2 itself: `-release-notes-range=<real>
// -release-notes-predict-compare=$RECORDED` with RECORDED unset used to run
// the whole prediction, silently skip CompareReleaseNotesPrediction (the one
// call G2 exists to make), and still exit 0 — a driver would read that as "G2
// confirmed G1's prediction" when no comparison had happened at all.
//
// The flag package's process-global *flag.String vars mean the fix inside
// TestPredictReleaseNotesForRange_G1Capture itself can only be exercised by
// actually invoking `go test -args ...` with a real flag set — a plain Go
// call from within this same test binary reads whatever flags THIS process
// happened to be started with, not an independently chosen combination. This
// spawns three real subprocess invocations, one per combination the finding
// names, and checks each one's actual exit code and output — the same
// invocation shape a gate driver uses, not a stand-in for it.
// runG1CaptureSubprocess invokes TestPredictReleaseNotesForRange_G1Capture the
// way a gate driver actually invokes it — a real `go test ./tests -run ... -args
// ...` subprocess against this repository's own checkout — and returns its
// combined output and exit code.
//
// It has to be a subprocess, not a direct Go call, for a reason that is easy to
// read past: the -release-notes-* flags are process-global *flag.String vars, so
// a same-process call observes whatever flags THIS test binary was started with
// (in practice: none), never an independently chosen combination. And the exit
// CODE is half the assertion — a driver reads exit status, not a Go error value,
// so a check that only inspected an in-process error would not be testing what
// the release pipeline consumes.
func runG1CaptureSubprocess(t *testing.T, args ...string) (combinedOutput string, exitCode int) {
	t.Helper()
	full := append([]string{"test", "./tests", "-run", "^TestPredictReleaseNotesForRange_G1Capture$", "-count=1", "-v", "-args"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("go %s: %v\n%s", strings.Join(full, " "), err, out)
		}
	}
	return string(out), code
}

func TestPredictReleaseNotesForRange_G1Capture_RequiresRangeWhenCompareOrOutGiven(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	t.Run("compare without range must fail, not skip", func(t *testing.T) {
		out, code := runG1CaptureSubprocess(t, "-release-notes-predict-compare="+filepath.Join(t.TempDir(), "recorded.json"))
		if code == 0 {
			t.Fatalf("expected a non-zero exit when -release-notes-predict-compare is given without -release-notes-range, got exit 0:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "imply -release-notes-range and must not silently skip") {
			t.Errorf("expected the fatal message naming the missing -release-notes-range, got:\n%s", out)
		}
	})

	t.Run("out without range must fail, not skip", func(t *testing.T) {
		out, code := runG1CaptureSubprocess(t, "-release-notes-predict-out="+filepath.Join(t.TempDir(), "prediction.json"))
		if code == 0 {
			t.Fatalf("expected a non-zero exit when -release-notes-predict-out is given without -release-notes-range, got exit 0:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
	})

	t.Run("neither flag given still skips cleanly", func(t *testing.T) {
		// Negative control: the fix must not turn the ordinary, flagless `go
		// test ./tests` invocation (every CI run of this suite) into a
		// failure — only the case where -release-notes-predict-compare or
		// -release-notes-predict-out was given without -release-notes-range.
		out, code := runG1CaptureSubprocess(t)
		if code != 0 {
			t.Fatalf("expected exit 0 when no flags are given at all, got exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to SKIP when no flags are given, got:\n%s", out)
		}
	})
}

// TestPredictReleaseNotesForRange_G1Capture_RequiresNonEmptyValueWhenFlagGiven
// is the regression test for the BLOCKING finding's actual worst case: a
// flag given on the command line but with an EMPTY value (a driver
// expanding an unset shell variable, `-release-notes-predict-compare=$VAR`
// with VAR unset) is not the same thing as that flag never having been
// passed at all, and the fix must fail loudly on the former while still
// treating the latter as a clean skip. `*flag == ""` alone cannot tell the
// two apart — only flag.CommandLine.Visit (presence) can — so this spawns
// real subprocesses the same way the sibling test above does, since a
// same-process call only ever observes the one flag set this test binary
// itself was started with.
func TestPredictReleaseNotesForRange_G1Capture_RequiresNonEmptyValueWhenFlagGiven(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	t.Run("range given empty must fail, not skip", func(t *testing.T) {
		out, code := runG1CaptureSubprocess(t, "-release-notes-range=")
		if code == 0 {
			t.Fatalf("expected a non-zero exit when -release-notes-range is given but empty, got exit 0:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "-release-notes-range was given but is empty") {
			t.Errorf("expected the fatal message naming the empty -release-notes-range, got:\n%s", out)
		}
	})

	t.Run("range real, compare given empty must fail, not silently skip the comparison (THE worst case)", func(t *testing.T) {
		out, code := runG1CaptureSubprocess(t, "-release-notes-range=HEAD..HEAD", "-release-notes-predict-compare=")
		if code == 0 {
			t.Fatalf("expected a non-zero exit when -release-notes-predict-compare is given but empty alongside a real range, got exit 0 — this is the exact BLOCKING scenario: G2's equality check silently not running while `go test` still passes:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "-release-notes-predict-compare was given but is empty") {
			t.Errorf("expected the fatal message naming the empty -release-notes-predict-compare, got:\n%s", out)
		}
	})

	t.Run("range real, out given empty must fail, not silently skip the write", func(t *testing.T) {
		out, code := runG1CaptureSubprocess(t, "-release-notes-range=HEAD..HEAD", "-release-notes-predict-out=")
		if code == 0 {
			t.Fatalf("expected a non-zero exit when -release-notes-predict-out is given but empty alongside a real range, got exit 0:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "-release-notes-predict-out was given but is empty") {
			t.Errorf("expected the fatal message naming the empty -release-notes-predict-out, got:\n%s", out)
		}
	})

	t.Run("range and out both real still succeeds and writes the file (positive control)", func(t *testing.T) {
		// Negative control for the fix above: it must not turn the ordinary,
		// legitimate G1 invocation (a real range, a real -predict-out path,
		// no -predict-compare) into a failure. Without this, a mutation that
		// made outGiven's empty-value check fire unconditionally (regardless
		// of *releaseNotesPredictOut's actual value) would pass every test
		// above and still break every real G1 run.
		outPath := filepath.Join(t.TempDir(), "prediction.json")
		out, code := runG1CaptureSubprocess(t, "-release-notes-range=HEAD..HEAD", "-release-notes-predict-out="+outPath)
		if code != 0 {
			t.Fatalf("expected exit 0 for a real range with a real -release-notes-predict-out, got exit %d:\n%s", code, out)
		}
		if _, err := os.Stat(outPath); err != nil {
			t.Fatalf("expected -release-notes-predict-out to be written to %s, got: %v\n%s", outPath, err, out)
		}
	})
}

// TestPredictReleaseNotesForRange_G1Capture_MismatchFailsTheGate is the
// end-to-end test for the one thing G2 exists to do: FAIL the release when the
// freshly computed prediction disagrees with the prediction G1 recorded.
//
// THE FINDING THIS CLOSES (MAJOR, and it was about this lane's own gate check).
// Until this test, that comparison had no end-to-end coverage anywhere in the
// tree. TestCompareReleaseNotesPrediction exercises the LIBRARY function in
// process, and the two _Requires* tests above exercise the flag plumbing — but
// nothing connected a disagreeing prediction to a non-zero EXIT CODE, which is
// the only thing a release driver actually reads. Verified before writing this,
// by mutating TestPredictReleaseNotesForRange_G1Capture's own
//
//	if err := CompareReleaseNotesPrediction(...); err != nil { t.Fatal(err) }
//
// into a `t.Logf` — the exact "downgrade the hard failure to a logged warning"
// change a future edit might make — and running `go test ./tests -count=1`:
// the whole package still reported `ok`. A gate whose failure path is untested
// is indistinguishable from a gate that cannot fail, which is CLAUDE.md's
// central rule ("a skipped check is indistinguishable from a pass over zero
// assertions") applied to the gate itself rather than to what it inspects.
//
// WHY "HEAD..HEAD". Every case below predicts over the EMPTY range HEAD..HEAD,
// whose fresh prediction is exactly `## Changelog\n` with no groups and no
// dropped commits, in ANY checkout — no tag, no history depth, no ambient git
// config is involved. That matters for the same reason
// TestPredictReleaseNotesForRange_MergeCommitExcluded was rebuilt hermetic: a
// case pinned to a real range (v0.5.0..HEAD) resolves only in a checkout that
// fetched the tag, and whether any given checkout did is decided somewhere this
// test does not read — so it would fail with git exit 128 for a reason that has
// nothing to do with what it claims to pin. The RECORDED side is then written
// by hand per case, which is what makes each disagreement precise: one case
// disagrees only in Body, one only in Dropped, and two agree — so a mutation
// that made the comparison always-fail is caught by the positive controls just
// as a mutation that made it never-fail is caught by the negative ones.
func TestPredictReleaseNotesForRange_G1Capture_MismatchFailsTheGate(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	// emptyRangeBody is what PredictReleaseNotesForRange returns for
	// HEAD..HEAD: formatChangelog always emits the "## Changelog" header even
	// when no group claimed anything (see TestPredictReleaseNotes_EmptyRange).
	const emptyRangeBody = "## Changelog\n"

	// recordAs writes a hand-built ReleaseNotesPrediction to a temp file in
	// exactly the shape G1's -release-notes-predict-out produces, and returns
	// its path — this is the "prediction G1 recorded" side of the comparison.
	recordAs := func(t *testing.T, p ReleaseNotesPrediction) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "g1-prediction.json")
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			t.Fatalf("marshal recorded prediction: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		return path
	}

	t.Run("a disagreeing Body must fail the gate, not warn", func(t *testing.T) {
		// G1 recorded a release with a feature in it; the fresh prediction has
		// nothing at all. This is the ordinary shape of the disagreement G2
		// exists to catch — the approved notes and the notes that would
		// actually publish are different documents.
		recorded := recordAs(t, ReleaseNotesPrediction{
			Body: "## Changelog\n### Features\n* aaa1111 feat(cli): add widget export\n",
			Groups: []PredictedGroup{{
				Title:   "Features",
				Order:   0,
				Commits: []ReleaseNotesCommit{{SHA: "aaa1111", Subject: "feat(cli): add widget export"}},
			}},
		})

		out, code := runG1CaptureSubprocess(t,
			"-release-notes-range=HEAD..HEAD",
			"-release-notes-predict-compare="+recorded)

		if code == 0 {
			t.Fatalf("expected a non-zero exit when the fresh prediction disagrees with G1's recording, got exit 0 — this is the entire reason G2 runs, and a driver reading this exit code would publish notes nobody approved:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "does not match the fresh prediction") {
			t.Errorf("expected CompareReleaseNotesPrediction's mismatch message, got:\n%s", out)
		}
		if strings.Contains(out, "matches the recorded prediction at") {
			t.Errorf("the success log line was printed for a mismatching prediction; the comparison took the passing branch:\n%s", out)
		}
	})

	t.Run("a Dropped-only disagreement must fail the gate", func(t *testing.T) {
		// The harder half, and the one a Body-only comparison can never see: a
		// docs:-prefixed commit — an ordinary user-visible change that happens
		// to carry a dropped subject prefix — is in G1's recording and not in
		// the fresh prediction. Body is byte-identical on both sides (a dropped
		// commit by definition never appears in Body), so ONLY PublishedEqual's
		// Dropped comparison can catch it. If this passes while the Body case
		// above fails, the gate is blind to exactly the hazard surfaces.yaml's
		// release-notes entry names.
		recorded := recordAs(t, ReleaseNotesPrediction{
			Body: emptyRangeBody,
			Dropped: []DroppedCommit{{
				ReleaseNotesCommit: ReleaseNotesCommit{
					SHA:     "ccc3333cccc3333cccc3333cccc3333cccc33333",
					Subject: "docs: the --strict flag now rejects empty claims",
				},
				ExcludedBy: "^docs:",
			}},
		})

		out, code := runG1CaptureSubprocess(t,
			"-release-notes-range=HEAD..HEAD",
			"-release-notes-predict-compare="+recorded)

		if code == 0 {
			t.Fatalf("expected a non-zero exit when the two predictions differ ONLY in Dropped, got exit 0 — a user-visible change silently missing from the published release page is invisible in Body by construction:\n%s", out)
		}
		if !strings.Contains(out, "docs: the --strict flag now rejects empty claims") {
			t.Errorf("expected the mismatch report to name the dropped commit that caused it, got:\n%s", out)
		}
	})

	t.Run("an identical recorded prediction still passes (positive control)", func(t *testing.T) {
		// Without this, a mutation that made CompareReleaseNotesPrediction
		// return an error unconditionally would satisfy both cases above while
		// failing every real release.
		recorded := recordAs(t, ReleaseNotesPrediction{Body: emptyRangeBody})

		out, code := runG1CaptureSubprocess(t,
			"-release-notes-range=HEAD..HEAD",
			"-release-notes-predict-compare="+recorded)

		if code != 0 {
			t.Fatalf("expected exit 0 when the recorded prediction matches the fresh one, got exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "matches the recorded prediction at") {
			t.Errorf("expected the success log line proving the comparison actually ran (rather than the test skipping past it), got:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the comparison to RUN, not skip, got:\n%s", out)
		}
	})

	t.Run("a recorded prediction differing only by the merge-commit drop still passes", func(t *testing.T) {
		// PublishedEqual's one deliberate exemption, exercised through the gate
		// entry point rather than only in process: the "^Merge "-excluded merge
		// commit exists in the merge-commit range and structurally cannot exist
		// in the branch range, so its presence on one side alone describes the
		// same published body. Were it compared like every other Dropped entry,
		// G2 would fail on EVERY release rather than on a wrong one — the
		// false-positive twin of the finding this test closes.
		recorded := recordAs(t, ReleaseNotesPrediction{
			Body: emptyRangeBody,
			Dropped: []DroppedCommit{{
				ReleaseNotesCommit: ReleaseNotesCommit{
					SHA:     "eee5555eeee5555eeee5555eeee5555eeee55555",
					Subject: "Merge pull request #99 — hermetic release fixture",
				},
				ExcludedBy: "^Merge ",
			}},
		})

		out, code := runG1CaptureSubprocess(t,
			"-release-notes-range=HEAD..HEAD",
			"-release-notes-predict-compare="+recorded)

		if code != 0 {
			t.Fatalf("expected exit 0 when the only difference is the \"^Merge \"-excluded merge commit, got exit %d — G2 would then fail every release, not just a wrong one:\n%s", code, out)
		}
		if !strings.Contains(out, "matches the recorded prediction at") {
			t.Errorf("expected the success log line proving the comparison ran, got:\n%s", out)
		}
	})
}

// TestReleaseNotesPublishedBodyCheck is G3's gate-callable entry point: the
// counterpart to TestPredictReleaseNotesForRange_G1Capture that runs after
// the tag exists and the release is actually published. Unlike G1/G2, G3
// makes no git invocation of its own and computes no fresh prediction — the
// release notes it needs to check ALREADY happened; it only reads G2's
// recorded prediction back off disk and the real published body a release
// workflow fetched separately (e.g. `gh release view <tag> --json body -q
// .body > published-body.txt`), then checks the two are consistent via
// PublishedBodyMatches — never `==` or PublishedEqual, see
// ReleaseNotesPrediction's doc comment in release_notes_predict_lib_test.go
// for why a breaking release's hand-written prefix makes those the wrong
// checks here.
//
//	go test ./tests -run TestReleaseNotesPublishedBodyCheck -args \
//	  -release-notes-predicted-json=/path/to/g2-prediction.json \
//	  -release-notes-published-body=/path/to/published-body.txt
//
// A missing/unreadable file on either side fails loudly (CLAUDE.md: "a skip
// is a failure, not a pass"), same as CompareReleaseNotesPrediction does for
// G2. A published body with no "## Changelog" anchor at all (AnchorFound:
// false) and one with the anchor but disagreeing content (AnchorFound: true,
// Matched: false — real drift) both fail this test, but with distinguishable
// messages, so a human reading a BLOCKING G3 finding can tell "the release
// process may not have run as expected" apart from "the release doesn't say
// what was predicted" — see PublishedBodyCheck's doc comment for the fuller
// account of why that distinction exists and is not derivable from Matched
// alone. With neither flag set (the default `go test` invocation and every
// CI run of this suite) this test does nothing beyond confirming the flags
// parse — PublishedBodyMatches's own correctness is pinned by the
// TestPublishedBodyMatches_* tests above, not this one.
//
// The "given together" check below keys off PRESENCE (flag.CommandLine.Visit),
// not VALUE (*flag == ""): a driver that passes both
// -release-notes-predicted-json= and -release-notes-published-body= with
// both values empty (e.g. two unset shell variables) has, from the flag
// package's point of view, given both flags — `*flag == ""` alone cannot
// distinguish that from neither flag having been passed at all, and the old
// `predictedJSON == "" && publishedBody == ""` skip condition treated the two
// identically, silently skipping the one check that confirms what actually
// published matches what was approved. See
// TestReleaseNotesPublishedBodyCheck_RequiresBothFlagsGivenWithNonEmptyValues
// for the regression test.
func TestReleaseNotesPublishedBodyCheck(t *testing.T) {
	var predictedGiven, publishedGiven bool
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "release-notes-predicted-json":
			predictedGiven = true
		case "release-notes-published-body":
			publishedGiven = true
		}
	})

	if !predictedGiven && !publishedGiven {
		t.Skip("no -release-notes-predicted-json/-release-notes-published-body given; this test is a capture entry point, not a correctness check (see the TestPublishedBodyMatches_* tests for that)")
	}
	if !predictedGiven || !publishedGiven {
		t.Fatalf("-release-notes-predicted-json and -release-notes-published-body must both be given together (predicted-json given=%v, published-body given=%v)", predictedGiven, publishedGiven)
	}
	if *releaseNotesPredictedJSON == "" || *releaseNotesPublishedBody == "" {
		// Both flags WERE passed, but at least one carries an empty value —
		// the unset-shell-variable hazard again, this time on the pair that
		// look, from `*flag == ""` alone, identical to "neither flag given at
		// all" and would otherwise hit the Skip above instead of failing.
		t.Fatalf("-release-notes-predicted-json and -release-notes-published-body were both given but at least one is empty (e.g. a driver expanding an unset shell variable); got predicted-json=%q published-body=%q", *releaseNotesPredictedJSON, *releaseNotesPublishedBody)
	}

	recorded, err := os.ReadFile(*releaseNotesPredictedJSON)
	if err != nil {
		t.Fatalf("read -release-notes-predicted-json %s: %v", *releaseNotesPredictedJSON, err)
	}
	var prediction ReleaseNotesPrediction
	if err := json.Unmarshal(recorded, &prediction); err != nil {
		t.Fatalf("parse -release-notes-predicted-json %s: %v", *releaseNotesPredictedJSON, err)
	}

	published, err := os.ReadFile(*releaseNotesPublishedBody)
	if err != nil {
		t.Fatalf("read -release-notes-published-body %s: %v", *releaseNotesPublishedBody, err)
	}

	check := prediction.PublishedBodyMatches(string(published))
	if check.Matched {
		t.Logf("published body at %s matches the recorded prediction at %s", *releaseNotesPublishedBody, *releaseNotesPredictedJSON)
		return
	}
	if !check.AnchorFound {
		t.Fatalf("published body at %s has no \"## Changelog\" anchor line at all — either the release process did not run as this project's own documented procedure describes, or a human deliberately replaced the generated section (this project's own v0.2.0/v0.3.0 releases did exactly that); confirm which with the human before treating this as drift. Recorded prediction at %s:\n%s", *releaseNotesPublishedBody, *releaseNotesPredictedJSON, prediction.Body)
	}
	t.Fatalf("published body at %s does not match the recorded prediction at %s — the generated section IS present but its content disagrees with what was approved.\nrecorded prediction body:\n%s\npublished body:\n%s", *releaseNotesPublishedBody, *releaseNotesPredictedJSON, prediction.Body, string(published))
}

// TestReleaseNotesPublishedBodyCheck_RequiresBothFlagsGivenWithNonEmptyValues
// is the regression test for the BLOCKING finding's G3 half: the WORST case
// there is a driver invoking G3 as
//
//	-release-notes-predicted-json=$G2_OUT -release-notes-published-body=$PUBLISHED
//
// with BOTH shell variables unset, expanding to
// `-release-notes-predicted-json= -release-notes-published-body=`. Both
// flags carry an empty value, so the OLD
// `predictedJSON == "" && publishedBody == ""` skip condition (true for
// both) could never tell that apart from the ordinary flagless `go test
// ./tests` invocation, and skipped cleanly — exit 0, no comparison of the
// real published release notes against what was approved ever ran. The fix
// keys off flag.CommandLine.Visit (presence) instead, so this spawns real
// subprocesses to exercise it, the same way the G1/G2 regression tests above
// do (a same-process call only observes the flags this test binary itself
// started with).
func TestReleaseNotesPublishedBodyCheck_RequiresBothFlagsGivenWithNonEmptyValues(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	t.Run("both flags given empty must fail, not silently skip (THE worst case)", func(t *testing.T) {
		out, code := runG3PublishedBodySubprocess(t, "-release-notes-predicted-json=", "-release-notes-published-body=")
		if code == 0 {
			t.Fatalf("expected a non-zero exit when both flags are given but empty, got exit 0 — this is the exact BLOCKING scenario: G3's published-body check silently not running while `go test` still passes:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "were both given but at least one is empty") {
			t.Errorf("expected the fatal message naming the empty flag pair, got:\n%s", out)
		}
	})

	t.Run("predicted-json given empty alone must fail (asymmetric, one side only)", func(t *testing.T) {
		out, code := runG3PublishedBodySubprocess(t, "-release-notes-predicted-json=")
		if code == 0 {
			t.Fatalf("expected a non-zero exit when only -release-notes-predicted-json is given (empty), got exit 0:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "must both be given together") {
			t.Errorf("expected the fatal message naming the missing pair, got:\n%s", out)
		}
	})

	t.Run("neither flag given still skips cleanly", func(t *testing.T) {
		// Negative control: the fix must not turn the ordinary, flagless `go
		// test ./tests` invocation (every CI run of this suite) into a
		// failure.
		out, code := runG3PublishedBodySubprocess(t)
		if code != 0 {
			t.Fatalf("expected exit 0 when no flags are given at all, got exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to SKIP when no flags are given, got:\n%s", out)
		}
	})

	t.Run("both flags given real values still runs the check (positive control)", func(t *testing.T) {
		// Negative control: a real, non-empty pair must still reach
		// PublishedBodyMatches. Uses a predicted-json whose "## Changelog"
		// anchor the published body deliberately omits, so the run is
		// expected to FAIL (AnchorFound: false) — that failure is the proof
		// the check actually executed rather than being skipped, which a
		// bug re-introducing the old `== "" && == ""` skip condition would
		// also produce as an exit-0 "ok" indistinguishable from this one
		// without the message assertion below.
		dir := t.TempDir()
		predictedPath := filepath.Join(dir, "g2-prediction.json")
		publishedPath := filepath.Join(dir, "published-body.txt")
		predicted := ReleaseNotesPrediction{Body: "## Changelog\n\n### Features\n- abc1234 feat: widget\n"}
		data, err := json.MarshalIndent(predicted, "", "  ")
		if err != nil {
			t.Fatalf("marshal fixture prediction: %v", err)
		}
		if err := os.WriteFile(predictedPath, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", predictedPath, err)
		}
		if err := os.WriteFile(publishedPath, []byte("no changelog anchor here\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", publishedPath, err)
		}
		out, code := runG3PublishedBodySubprocess(t, "-release-notes-predicted-json="+predictedPath, "-release-notes-published-body="+publishedPath)
		if code == 0 {
			t.Fatalf("expected a non-zero exit for a published body missing the \"## Changelog\" anchor (proving PublishedBodyMatches actually ran), got exit 0:\n%s", out)
		}
		if !strings.Contains(out, "has no \"## Changelog\" anchor line at all") {
			t.Errorf("expected the AnchorFound-false message, got:\n%s", out)
		}
	})
}

// runG3PublishedBodySubprocess invokes TestReleaseNotesPublishedBodyCheck the
// way a release workflow actually invokes it — a real `go test ./tests -run ...
// -args ...` subprocess against this repository's own checkout — and returns
// its combined output and exit code. It is the G3 twin of
// runG1CaptureSubprocess above and exists for the same two reasons: the
// -release-notes-* flags are process-global *flag.String vars, so a
// same-process call can only ever observe the flags THIS test binary was
// started with; and the exit CODE is half of every assertion here, because a
// release driver reads exit status, never a Go error value.
func runG3PublishedBodySubprocess(t *testing.T, args ...string) (combinedOutput string, exitCode int) {
	t.Helper()
	full := append([]string{"test", "./tests", "-run", "^TestReleaseNotesPublishedBodyCheck$", "-count=1", "-v", "-args"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("go %s: %v\n%s", strings.Join(full, " "), err, out)
		}
	}
	return string(out), code
}

// TestReleaseNotesPublishedBodyCheck_DriftFailsTheGate is the end-to-end test
// for the one thing G3 exists to do: FAIL the release when the body that
// ACTUALLY PUBLISHED disagrees with the prediction G2 recorded and a human
// approved.
//
// THE FINDING THIS CLOSES (MAJOR). This is the same defect class as the one
// closed one gate stage earlier by
// TestPredictReleaseNotesForRange_G1Capture_MismatchFailsTheGate — found again
// in this same file, against G3 instead of G2, because closing it for one stage
// did not close it for the next. Before this test, TWO independent mutations of
// TestReleaseNotesPublishedBodyCheck left the entire package green:
//
//	NEVER-FAILS: the real-drift `t.Fatalf("published body at %s does not match
//	the recorded prediction at %s ...")` replaced by `t.Logf` —
//	`go test ./tests -count=1` → ok. That is the single most alarming thing
//	G3 can see (the "## Changelog" anchor IS present and the published release
//	page disagrees with what was approved) and nothing in the tree connected it
//	to a non-zero exit code.
//
//	ALWAYS-FAILS: `if check.Matched {` → `if false && check.Matched {`, so G3
//	reports a mismatch on every release including a perfect one —
//	`go test ./tests -count=1` → ok. There was no positive control anywhere:
//	every G3 subtest reaching PublishedBodyMatches asserted a NON-ZERO exit
//	(the AnchorFound:false case), and the only exit-0 subtest skipped before
//	the check ran.
//
// So the two subtests below are deliberately a matched pair — one drift case
// that must exit NON-ZERO, one clean case that must exit ZERO — because either
// one alone leaves the opposite mutation invisible. That pairing is exactly
// what the G2 test does and exactly what this file previously did not do on
// the G3 side.
//
// TestPublishedBodyMatches_CatchesRealDrift already exercises the LIBRARY
// function in process. That is not this: it never reaches an exit code, which
// is the only signal a release driver reads — the identical in-process-only gap
// that made the G2 finding real.
//
// WHY THE PUBLISHED BODIES CARRY A HAND-WRITTEN PREFIX. Both cases put prose
// ahead of the "## Changelog" anchor, because that is this project's real
// shape on a breaking release (v0.5.0's published body opens with ~3,190
// characters of hand-written breaking-change prose). The clean case therefore
// also pins the contract that a prefix is EXPECTED and not drift — checked
// end to end through the gate entry point rather than only against
// PublishedBodyMatches directly — and the drift case proves the anchor logic
// still finds the boundary underneath one.
func TestReleaseNotesPublishedBodyCheck_DriftFailsTheGate(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	// predictedBody is the generated section as G2 recorded it: the exact
	// bytes PredictReleaseNotes produces for a two-commit release.
	const predictedBody = "## Changelog\n### Features\n* aaa1111 feat(cli): add widget export\n### Bug fixes\n* bbb2222 fix(render): correct table overflow\n"

	// handWrittenPrefix is the "breaking changes called out first" prose
	// docs/RELEASING.md requires, carried onto the published release page ahead
	// of the generated section. No predictor can generate it, and its presence
	// must not read as drift.
	const handWrittenPrefix = "This release changes the lock ledger format.\n\nRe-lock every claim before upgrading; `dossierx check` reports the old format as `mixed-cycle`.\n\n"

	// writeG3Inputs records a prediction and a published body to a temp dir in
	// exactly the shapes a release workflow hands G3 — a JSON file from G2's
	// -release-notes-predict-out, and a text file from
	// `gh release view <tag> --json body -q .body`.
	writeG3Inputs := func(t *testing.T, prediction ReleaseNotesPrediction, published string) (predictedPath, publishedPath string) {
		t.Helper()
		dir := t.TempDir()
		predictedPath = filepath.Join(dir, "g2-prediction.json")
		publishedPath = filepath.Join(dir, "published-body.txt")
		data, err := json.MarshalIndent(prediction, "", "  ")
		if err != nil {
			t.Fatalf("marshal recorded prediction: %v", err)
		}
		if err := os.WriteFile(predictedPath, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", predictedPath, err)
		}
		if err := os.WriteFile(publishedPath, []byte(published), 0o644); err != nil {
			t.Fatalf("write %s: %v", publishedPath, err)
		}
		return predictedPath, publishedPath
	}

	t.Run("real drift — anchor present, content disagrees — must fail the gate, not warn", func(t *testing.T) {
		// The published page has the generated section, but one commit subject
		// in it is not the subject that was approved (a subject hand-edited on
		// the release page after tagging, or a filter that behaved differently
		// than predicted). This is the single most alarming thing G3 can see.
		drifted := handWrittenPrefix + strings.Replace(predictedBody,
			"fix(render): correct table overflow",
			"fix(render): correct table overflow and drop the legacy renderer", 1)
		if !strings.Contains(drifted, "\n## Changelog\n") {
			t.Fatalf("test setup: the drifted body must still carry the anchor on its own line, otherwise this exercises the AnchorFound:false branch instead of drift:\n%s", drifted)
		}

		predictedPath, publishedPath := writeG3Inputs(t, ReleaseNotesPrediction{Body: predictedBody}, drifted)
		out, code := runG3PublishedBodySubprocess(t,
			"-release-notes-predicted-json="+predictedPath,
			"-release-notes-published-body="+publishedPath)

		if code == 0 {
			t.Fatalf("expected a non-zero exit when the published release body disagrees with the recorded prediction, got exit 0 — this is the entire reason G3 runs, and a driver reading this exit code would report a release as verified whose published notes nobody approved:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "does not match the recorded prediction at") {
			t.Errorf("expected the real-drift message, got:\n%s", out)
		}
		// The two failure branches must stay distinguishable: a human
		// confirming a BLOCKING G3 finding reads "confirm this replacement was
		// deliberate" very differently from "the release doesn't say what was
		// predicted", and only AnchorFound separates them.
		if strings.Contains(out, "has no \"## Changelog\" anchor line at all") {
			t.Errorf("drift was reported as a MISSING anchor; the two failure branches have collapsed and the human is told the wrong thing:\n%s", out)
		}
		if strings.Contains(out, "matches the recorded prediction at") {
			t.Errorf("the success log line was printed for a drifted body; the check took the passing branch:\n%s", out)
		}
	})

	t.Run("a published body that matches (under a hand-written prefix) passes (positive control)", func(t *testing.T) {
		// Without this, a mutation making G3 report a mismatch unconditionally
		// — `if false && check.Matched` — satisfies both the drift case above
		// and every pre-existing G3 subtest, while failing every real release.
		predictedPath, publishedPath := writeG3Inputs(t,
			ReleaseNotesPrediction{Body: predictedBody},
			handWrittenPrefix+predictedBody)

		out, code := runG3PublishedBodySubprocess(t,
			"-release-notes-predicted-json="+predictedPath,
			"-release-notes-published-body="+publishedPath)

		if code != 0 {
			t.Fatalf("expected exit 0 when the published body's generated section matches the recorded prediction (a hand-written prefix ahead of it is EXPECTED, not drift), got exit %d — G3 would then fail every release, not just a wrong one:\n%s", code, out)
		}
		if !strings.Contains(out, "matches the recorded prediction at") {
			t.Errorf("expected the success log line proving the comparison actually ran rather than the test skipping past it, got:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the check to RUN, not skip — an exit-0 SKIP is indistinguishable from a pass over zero assertions, got:\n%s", out)
		}
	})
}
