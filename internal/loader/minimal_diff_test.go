package loader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Writing a claim used to marshal a fresh document from the struct, so every
// write re-emitted the whole file: key order came from model.Claim's field
// order and block-scalar indent from yaml.v3's default, and adding one comment
// landed as a 117-line diff in which exactly one key was new. These tests pin
// the mutate-in-place behaviour that replaced it — an untouched key keeps its
// AUTHORED bytes — for the four write shapes that matter: append a key, change
// one scalar (lock/unlock/reaudit/flag all reduce to this), preserve an
// authored key order the struct does not reproduce, and preserve a block
// scalar's indentation.

// authoredClaimYAML is deliberately NOT in model.Claim field order (governed_by
// sits between module and status, facet leads), so a whole-file re-emit is
// detectable rather than coincidentally identical, and its body is a 2-space
// block scalar like the authored corpus under testdata/.
const authoredClaimYAML = `facet: contract
id: widget.contract.overview
module: widget
governed_by:
  type: none
  reason: fixture claim, not backed by any real doctrine
status: draft
layout: card
body: |
  A widget is the smallest unit this fixture project documents.
  It is described in prose, not in rows.
`

func loadOne(t *testing.T, dir string) model.Claim {
	t.Helper()
	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("expected exactly 1 claim, got %d", len(claims))
	}
	return claims[0]
}

func readBack(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// deletedLines returns the lines of before that are absent from after — the
// programmatic form of `diff before after | grep -c '^<'`, which is the shape
// every one of T6's done-when bullets is phrased in.
func deletedLines(before, after string) []string {
	present := map[string]int{}
	for _, l := range strings.Split(after, "\n") {
		present[l]++
	}
	var gone []string
	for _, l := range strings.Split(before, "\n") {
		if present[l] > 0 {
			present[l]--
			continue
		}
		gone = append(gone, l)
	}
	return gone
}

// TestSaveClaim_AddingACommentIsAPureAppend is done-when 1 at the package
// level: the first comments: block is appended and NOTHING above it moves.
func TestSaveClaim_AddingACommentIsAPureAppend(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", authoredClaimYAML)
	c := loadOne(t, dir)

	c.Comments = []model.Comment{{
		ID:      "c-000001",
		Status:  model.CommentStatusOpen,
		Author:  model.CommentRoleHuman,
		Created: "2026-07-24T10:12:00Z",
		Body:    "does this cover the empty case?",
	}}
	if err := SaveClaim(c); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}

	after := readBack(t, c.SourcePath)
	if !strings.HasPrefix(after, authoredClaimYAML) {
		t.Fatalf("adding a comment rewrote bytes above the new key:\nwant prefix:\n%s\ngot:\n%s", authoredClaimYAML, after)
	}
	added := strings.TrimPrefix(after, authoredClaimYAML)
	if !strings.HasPrefix(added, "comments:\n") {
		t.Fatalf("the appended region must start at the comments: key, got:\n%s", added)
	}
}

// TestSaveClaim_StatusChangeTouchesOneLine is done-when 10 at the package
// level: lock, unlock, reaudit and flag all reduce to mutating one scalar on a
// claim loaded from disk, and none of them may re-emit the file.
func TestSaveClaim_StatusChangeTouchesOneLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", authoredClaimYAML)
	c := loadOne(t, dir)

	c.Status = model.StatusLocked
	if err := SaveClaim(c); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}

	after := readBack(t, c.SourcePath)
	gone := deletedLines(authoredClaimYAML, after)
	if len(gone) != 1 || gone[0] != "status: draft" {
		t.Fatalf("locking must delete exactly the old status line, deleted %q\ngot:\n%s", gone, after)
	}
	if !strings.Contains(after, "status: locked") {
		t.Fatalf("expected the new status, got:\n%s", after)
	}
	// The block scalar is the half a default-indent emitter would silently
	// re-indent, so check its region byte-for-byte rather than by line set.
	body := "body: |\n  A widget is the smallest unit this fixture project documents.\n  It is described in prose, not in rows.\n"
	if !strings.Contains(after, body) {
		t.Fatalf("the body block scalar was re-indented:\n%s", after)
	}
}

// TestSaveClaim_UnchangedClaimIsByteIdentical is the strongest form of the
// guarantee: a load/save with no mutation at all must be a no-op on the bytes,
// authored key order and all.
func TestSaveClaim_UnchangedClaimIsByteIdentical(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.yaml", authoredClaimYAML)
	c := loadOne(t, dir)

	if err := SaveClaim(c); err != nil {
		t.Fatalf("SaveClaim: %v", err)
	}
	if after := readBack(t, c.SourcePath); after != authoredClaimYAML {
		t.Fatalf("a no-op save rewrote the file:\nwant:\n%s\ngot:\n%s", authoredClaimYAML, after)
	}
}

// TestSaveClaimIfUnchanged_PreservesAuthoredStyle proves the optimistic-
// concurrency writer shares the mutate path rather than keeping its own
// whole-file emitter.
func TestSaveClaimIfUnchanged_PreservesAuthoredStyle(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.yaml", authoredClaimYAML)
	c := loadOne(t, dir)
	token, err := CaptureClaimFileToken(path)
	if err != nil {
		t.Fatalf("capture token: %v", err)
	}

	c.ReviewPending = true
	if err := SaveClaimIfUnchanged(c, token); err != nil {
		t.Fatalf("SaveClaimIfUnchanged: %v", err)
	}

	after := readBack(t, path)
	if !strings.HasPrefix(after, authoredClaimYAML) {
		t.Fatalf("SaveClaimIfUnchanged rewrote authored bytes:\nwant prefix:\n%s\ngot:\n%s", authoredClaimYAML, after)
	}
	if strings.TrimPrefix(after, authoredClaimYAML) != "review_pending: true\n" {
		t.Fatalf("expected review_pending appended alone, got:\n%s", after)
	}
}

// TestCommentBodyRoundTrips_AgreesWithBothWritePaths is done-when 8: the
// predicate the comments input boundary uses (comments.validateBody) and the
// `comment add --dry-run` body_is_storable precondition both call
// CommentBodyRoundTrips directly, so "the dry-run and the real write path give
// the same answer" is exactly this agreement — asserted here over EVERY
// bodyRoundTripCases() body against BOTH real writers, mutate mode included.
// TestCommentBodyRoundTrips_AgreesWithSaveGuard covers create mode; a
// mutate-mode writer that emitted at a different indent would pass that test
// and still disagree here, which is the regression this adds.
func TestCommentBodyRoundTrips_AgreesWithBothWritePaths(t *testing.T) {
	for _, tc := range bodyRoundTripCases() {
		t.Run(tc.name, func(t *testing.T) {
			pre := CommentBodyRoundTrips(tc.body)
			if pre != tc.want {
				t.Fatalf("CommentBodyRoundTrips(%q) = %v, want %v", tc.body, pre, tc.want)
			}

			dir := t.TempDir()
			path := filepath.Join(dir, "a.yaml")
			writeFile(t, dir, "a.yaml", authoredClaimYAML)
			seeded := readBack(t, path)

			c := brickClaim(path, tc.body)
			token, err := CaptureClaimFileToken(path)
			if err != nil {
				t.Fatalf("capture token: %v", err)
			}

			if err := SaveClaimIfUnchanged(c, token); (err == nil) != pre {
				t.Fatalf("pre-check/SaveClaimIfUnchanged DISAGREE for body=%q: pre=%v, err=%v", tc.body, pre, err)
			} else if err != nil {
				if !errors.Is(err, ErrClaimNotRoundTrippable) {
					t.Fatalf("SaveClaimIfUnchanged(body=%q): unexpected error kind: %v", tc.body, err)
				}
				if now := readBack(t, path); now != seeded {
					t.Fatalf("a refused SaveClaimIfUnchanged(body=%q) clobbered the file:\n%s", tc.body, now)
				}
			}

			// And again through SaveClaim, which now also sees an existing file
			// (the mutate branch), not the absent one the create-mode test uses.
			if err := SaveClaim(c); (err == nil) != pre {
				t.Fatalf("pre-check/SaveClaim(mutate) DISAGREE for body=%q: pre=%v, err=%v", tc.body, pre, err)
			} else if err != nil && !errors.Is(err, ErrClaimNotRoundTrippable) {
				t.Fatalf("SaveClaim(mutate, body=%q): unexpected error kind: %v", tc.body, err)
			}
			if _, err := LoadClaims(dir); err != nil {
				t.Fatalf("claims dir failed to load after writes with body=%q: %v", tc.body, err)
			}
		})
	}
}
